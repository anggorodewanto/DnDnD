// Package scripts contains helper scripts that run as part of CI. The
// coverage_check binary is exercised by these tests so we don't need to
// invoke it from a shell.
package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runCoverageCheck builds and runs the coverage_check binary against a given
// profile and threshold. Returns combined stdout/stderr and the exit error.
func runCoverageCheck(t *testing.T, profile string, threshold string, perPkg string, exclude string) (string, error) {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	binPath := filepath.Join(t.TempDir(), "coverage_check")
	build := exec.Command("go", "build", "-o", binPath, "./coverage_check")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build coverage_check: %v\n%s", err, out)
	}
	args := []string{"-profile", profile, "-min-overall", threshold}
	if perPkg != "" {
		args = append(args, "-min-per-package", perPkg)
	}
	if exclude != "" {
		args = append(args, "-exclude", exclude)
	}
	cmd := exec.Command(binPath, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func writeProfile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "profile.out")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return path
}

func TestCoverageCheck_PassesAboveThreshold(t *testing.T) {
	body := "mode: set\n" +
		"github.com/example/foo/a.go:1.1,2.1 1 1\n" +
		"github.com/example/foo/a.go:2.1,3.1 1 1\n"
	profile := writeProfile(t, body)
	out, err := runCoverageCheck(t, profile, "50", "", "")
	if err != nil {
		t.Fatalf("expected pass, got error %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected OK in output, got: %s", out)
	}
}

func TestCoverageCheck_FailsBelowThreshold(t *testing.T) {
	body := "mode: set\n" +
		"github.com/example/foo/a.go:1.1,2.1 1 0\n" +
		"github.com/example/foo/a.go:2.1,3.1 1 0\n"
	profile := writeProfile(t, body)
	out, err := runCoverageCheck(t, profile, "50", "", "")
	if err == nil {
		t.Fatalf("expected non-zero exit for sub-threshold, got nil; output: %s", out)
	}
	if !strings.Contains(out, "below") {
		t.Errorf("expected 'below' in output, got: %s", out)
	}
}

func TestCoverageCheck_ExcludesMatchingPaths(t *testing.T) {
	// File a.go is fully uncovered; b.go is fully covered. With a.go excluded,
	// overall jumps from 50% to 100%.
	body := "mode: set\n" +
		"github.com/example/foo/a.go:1.1,2.1 1 0\n" +
		"github.com/example/foo/b.go:1.1,2.1 1 1\n"
	profile := writeProfile(t, body)
	// Without exclusion: 50% — should fail at 90% threshold.
	if _, err := runCoverageCheck(t, profile, "90", "", ""); err == nil {
		t.Fatal("expected fail without exclude")
	}
	// With exclusion: 100% — passes at 90%.
	if out, err := runCoverageCheck(t, profile, "90", "", "a\\.go$"); err != nil {
		t.Fatalf("expected pass with exclude, got %v\n%s", err, out)
	}
}

func TestCoverageCheck_PerPackageEnforced(t *testing.T) {
	// Package foo: 100% covered. Package bar: 0% covered. Per-pkg threshold
	// 50% should fail.
	body := "mode: set\n" +
		"github.com/example/foo/a.go:1.1,2.1 1 1\n" +
		"github.com/example/bar/b.go:1.1,2.1 1 0\n"
	profile := writeProfile(t, body)
	out, err := runCoverageCheck(t, profile, "0", "50", "")
	if err == nil {
		t.Fatalf("expected per-package failure, got pass\noutput: %s", out)
	}
	if !strings.Contains(out, "bar") {
		t.Errorf("expected failing pkg name in output, got: %s", out)
	}
}
