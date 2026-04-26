// Package scripts contains helper scripts that run as part of CI. The
// sqlc_drift_check binary detects when committed sqlc-generated files
// drift from what `sqlc generate` would produce, so a missed regen never
// merges silently. See Phase 118c.
package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runDriftCheck builds and runs the sqlc_drift_check binary against the
// given target directory. The binary is built in a temp dir and executed
// with the working directory set to the test's temp git repo. The harness
// passes a fake `sqlc` and `git` location via the SQLC_BIN and PATH env vars
// so we can simulate clean / dirty trees without running the real sqlc.
func runDriftCheck(t *testing.T, workdir, sqlcBin, target string) (string, error) {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	binPath := filepath.Join(t.TempDir(), "sqlc_drift_check")
	build := exec.Command("go", "build", "-o", binPath, "./sqlc_drift_check")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build sqlc_drift_check: %v\n%s", err, out)
	}
	args := []string{"-target", target}
	cmd := exec.Command(binPath, args...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "SQLC_BIN="+sqlcBin)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// initRepo creates a fresh git repo in a temp dir with a single committed
// file under the given target directory. Returns the workdir path.
func initRepo(t *testing.T, target, initialContent string) string {
	t.Helper()
	dir := t.TempDir()
	mustRun(t, dir, "git", "init", "-q")
	mustRun(t, dir, "git", "config", "user.email", "t@t")
	mustRun(t, dir, "git", "config", "user.name", "t")
	if err := os.MkdirAll(filepath.Join(dir, target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, target, "queries.sql.go"), []byte(initialContent), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	mustRun(t, dir, "git", "add", ".")
	mustRun(t, dir, "git", "commit", "-q", "-m", "init")
	return dir
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
}

// fakeSqlc writes a small Go binary that, when invoked, replaces the
// committed file with the supplied "regenerated" content. This simulates
// `sqlc generate`. If newContent matches the existing content, the tree
// stays clean.
func fakeSqlc(t *testing.T, target, newContent string) string {
	t.Helper()
	dir := t.TempDir()
	src := `package main
import (
  "os"
  "path/filepath"
)
func main() {
  wd, _ := os.Getwd()
  path := filepath.Join(wd, ` + "`" + target + "`" + `, "queries.sql.go")
  _ = os.WriteFile(path, []byte(` + "`" + newContent + "`" + `), 0o600)
}
`
	srcPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcPath, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	binPath := filepath.Join(dir, "sqlc")
	build := exec.Command("go", "build", "-o", binPath, srcPath)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake sqlc: %v\n%s", err, out)
	}
	return binPath
}

func TestSqlcDriftCheck_PassesWhenClean(t *testing.T) {
	const target = "internal/refdata"
	const content = "// generated\npackage refdata\n"
	workdir := initRepo(t, target, content)
	// Fake sqlc writes the same content back — tree stays clean.
	sqlc := fakeSqlc(t, target, content)
	out, err := runDriftCheck(t, workdir, sqlc, target)
	if err != nil {
		t.Fatalf("expected clean pass, got err %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected OK marker, got: %s", out)
	}
}

func TestSqlcDriftCheck_FailsWhenDirty(t *testing.T) {
	const target = "internal/refdata"
	const oldContent = "// generated v1\npackage refdata\n"
	const newContent = "// generated v2\npackage refdata\n"
	workdir := initRepo(t, target, oldContent)
	// Fake sqlc rewrites the file — tree becomes dirty.
	sqlc := fakeSqlc(t, target, newContent)
	out, err := runDriftCheck(t, workdir, sqlc, target)
	if err == nil {
		t.Fatalf("expected non-zero exit on drift, got nil\noutput: %s", out)
	}
	if !strings.Contains(out, "drift") && !strings.Contains(out, "DIRTY") {
		t.Errorf("expected drift marker in output, got: %s", out)
	}
}

func TestSqlcDriftCheck_FailsWhenSqlcMissing(t *testing.T) {
	const target = "internal/refdata"
	workdir := initRepo(t, target, "// generated\npackage refdata\n")
	out, err := runDriftCheck(t, workdir, "/nonexistent/path/to/sqlc", target)
	if err == nil {
		t.Fatalf("expected non-zero exit when sqlc binary missing, got nil\noutput: %s", out)
	}
}
