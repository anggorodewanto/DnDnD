// sqlc_drift_check is the Phase 118c CI guard that fails when the working
// tree drifts from `sqlc generate`. It runs sqlc, then asks git whether
// any tracked file under the target directory changed. Drift means a prior
// migration shipped without re-running the generator and the committed
// query layer is stale.
//
// Flags:
//
//	-target string  directory whose generated files must stay clean
//	                (default "internal/refdata").
//
// Env:
//
//	SQLC_BIN  override the sqlc binary path (default "sqlc" via PATH).
//
// Exit codes: 0 on clean, 1 on drift, 2 on tooling/IO error.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		var exitErr *driftError
		if errors.As(err, &exitErr) {
			fmt.Fprintln(os.Stderr, exitErr.Error())
			os.Exit(exitErr.code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// driftError carries an explicit exit code so callers (main + tests) can
// distinguish "drift detected" (1) from "tooling failure" (2).
type driftError struct {
	code int
	msg  string
}

func (e *driftError) Error() string { return e.msg }

func run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("sqlc_drift_check", flag.ContinueOnError)
	target := fs.String("target", "internal/refdata", "directory whose sqlc-generated files must stay clean")
	if err := fs.Parse(args); err != nil {
		return err
	}

	sqlcBin := os.Getenv("SQLC_BIN")
	if sqlcBin == "" {
		sqlcBin = "sqlc"
	}

	if err := runSqlc(sqlcBin, stdout, stderr); err != nil {
		return fmt.Errorf("running %s generate: %w", sqlcBin, err)
	}

	dirty, err := dirtyPaths(*target)
	if err != nil {
		return fmt.Errorf("checking git status for %s: %w", *target, err)
	}
	if len(dirty) > 0 {
		fmt.Fprintf(stdout, "DIRTY: sqlc drift detected under %s:\n", *target)
		for _, p := range dirty {
			fmt.Fprintf(stdout, "  %s\n", p)
		}
		fmt.Fprintln(stdout, "Run `sqlc generate` and commit the result.")
		return &driftError{code: 1, msg: "sqlc drift detected"}
	}
	fmt.Fprintf(stdout, "OK: no sqlc drift under %s\n", *target)
	return nil
}

func runSqlc(bin string, stdout, stderr io.Writer) error {
	cmd := exec.Command(bin, "generate")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// dirtyPaths returns the list of paths under target that git reports as
// modified, deleted, added, or untracked. We use porcelain v1 because its
// format is documented as stable and trivial to parse: each line is
// "XY path" where XY are 2 status chars.
func dirtyPaths(target string) ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain", "--", target)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	body := strings.TrimRight(string(out), "\n")
	if body == "" {
		return nil, nil
	}
	lines := strings.Split(body, "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		paths = append(paths, strings.TrimSpace(line[3:]))
	}
	return paths, nil
}
