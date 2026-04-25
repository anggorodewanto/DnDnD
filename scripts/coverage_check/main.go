// coverage_check parses a Go coverage profile and enforces a minimum overall
// coverage percentage and (optionally) a per-package floor. Files matching
// the -exclude regex are removed from the profile before computing
// percentages, which lets us exclude generated code (sqlc queries) and
// thin main-package wiring from the threshold without polluting the result.
//
// Exit codes: 0 on pass; 1 on threshold miss or parse error; 2 on flag
// parsing problems.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("coverage_check", flag.ContinueOnError)
	profile := fs.String("profile", "coverage.out", "path to Go coverage profile")
	minOverall := fs.Float64("min-overall", 90, "minimum overall coverage percent")
	minPerPkg := fs.Float64("min-per-package", 0, "minimum per-package coverage (0 = disabled)")
	exclude := fs.String("exclude", "", "regex of file paths to exclude from the profile")
	if err := fs.Parse(args); err != nil {
		return err
	}

	body, err := os.ReadFile(*profile)
	if err != nil {
		return fmt.Errorf("read profile: %w", err)
	}

	excludeRE, err := compileExclude(*exclude)
	if err != nil {
		return err
	}

	stats, err := parseProfile(string(body), excludeRE)
	if err != nil {
		return err
	}

	overall := stats.overallPercent()
	fmt.Fprintf(out, "Overall coverage (post-exclusion): %.2f%% (%d/%d statements)\n",
		overall, stats.coveredStmts, stats.totalStmts)

	failed := false
	if overall < *minOverall {
		fmt.Fprintf(out, "FAIL: overall %.2f%% is below threshold %.2f%%\n", overall, *minOverall)
		failed = true
	}

	if *minPerPkg > 0 {
		fmt.Fprintf(out, "Per-package coverage:\n")
		pkgs := stats.sortedPackages()
		for _, pkg := range pkgs {
			pct := stats.packagePercent(pkg)
			marker := "  "
			if pct < *minPerPkg {
				marker = "X "
				fmt.Fprintf(out, "%s%-65s %.2f%% — below %.2f%%\n", marker, pkg, pct, *minPerPkg)
				failed = true
				continue
			}
			fmt.Fprintf(out, "%s%-65s %.2f%%\n", marker, pkg, pct)
		}
	}

	if failed {
		return fmt.Errorf("coverage thresholds not met")
	}
	fmt.Fprintln(out, "OK: coverage thresholds met")
	return nil
}

func compileExclude(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile -exclude: %w", err)
	}
	return re, nil
}

type fileStmts struct {
	covered int64
	total   int64
}

type profileStats struct {
	files        map[string]*fileStmts
	coveredStmts int64
	totalStmts   int64
}

func parseProfile(body string, exclude *regexp.Regexp) (*profileStats, error) {
	stats := &profileStats{files: map[string]*fileStmts{}}
	for i, line := range strings.Split(body, "\n") {
		if i == 0 || line == "" {
			continue
		}
		// Format: filename:line.col,line.col numStatements count
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("malformed profile line %d: %q", i+1, line)
		}
		filename := strings.SplitN(fields[0], ":", 2)[0]
		if exclude != nil && exclude.MatchString(filename) {
			continue
		}
		numStmts, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad numStatements on line %d: %w", i+1, err)
		}
		count, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad count on line %d: %w", i+1, err)
		}
		fs, ok := stats.files[filename]
		if !ok {
			fs = &fileStmts{}
			stats.files[filename] = fs
		}
		fs.total += numStmts
		stats.totalStmts += numStmts
		if count > 0 {
			fs.covered += numStmts
			stats.coveredStmts += numStmts
		}
	}
	return stats, nil
}

func (s *profileStats) overallPercent() float64 {
	if s.totalStmts == 0 {
		return 100
	}
	return 100 * float64(s.coveredStmts) / float64(s.totalStmts)
}

// packagePercent groups by directory (everything before the last '/').
func (s *profileStats) packagePercent(pkg string) float64 {
	var covered, total int64
	for file, st := range s.files {
		if filePackage(file) == pkg {
			covered += st.covered
			total += st.total
		}
	}
	if total == 0 {
		return 100
	}
	return 100 * float64(covered) / float64(total)
}

func (s *profileStats) sortedPackages() []string {
	seen := map[string]bool{}
	for file := range s.files {
		seen[filePackage(file)] = true
	}
	out := make([]string, 0, len(seen))
	for pkg := range seen {
		out = append(out, pkg)
	}
	sort.Strings(out)
	return out
}

func filePackage(file string) string {
	idx := strings.LastIndex(file, "/")
	if idx < 0 {
		return file
	}
	return file[:idx]
}
