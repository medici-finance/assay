package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Output contract: --lint and --check ALWAYS end stdout with exactly one
// final verdict line ("LINT: PASS" / "LINT: FAIL <n> problem(s)",
// "CHECK: PASS" / "CHECK: DRIFT <n>"), so callers never have to interrogate
// the exit code in a separate shell. Formatting only — pass/fail semantics
// are asserted unchanged alongside each verdict.

// verdictRepo copies the goodrepo fixture into a temp dir.
func verdictRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("testdata/goodrepo")); err != nil {
		t.Fatal(err)
	}
	return root
}

// finalStdoutLine asserts stdout is non-empty, returns its last line, and
// fails the test if more than one verdict-shaped line appears.
func finalStdoutLine(t *testing.T, stdout string) string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		t.Fatalf("stdout has no final line: %q", stdout)
	}
	verdicts := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "LINT: ") || strings.HasPrefix(l, "CHECK: ") {
			verdicts++
		}
	}
	if verdicts != 1 {
		t.Errorf("stdout must contain exactly one verdict line, got %d:\n%s", verdicts, stdout)
	}
	return lines[len(lines)-1]
}

func TestLintVerdictLine(t *testing.T) {
	t.Run("pass", func(t *testing.T) {
		root := verdictRepo(t)
		var code int
		stdout := captureStdout(t, func() { code = run(root, "lint", nil, nil, "") })
		if code != 0 {
			t.Fatalf("lint on valid repo exited %d, want 0", code)
		}
		if got := finalStdoutLine(t, stdout); got != "LINT: PASS" {
			t.Errorf("final stdout line = %q, want %q", got, "LINT: PASS")
		}
	})

	t.Run("fail reports problem count", func(t *testing.T) {
		root := verdictRepo(t)
		readme := filepath.Join(root, "docs/streams/alpha/README.md")
		b, err := os.ReadFile(readme)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(readme, []byte(strings.Replace(string(b), "status: active", "status: bogus", 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		var code int
		stdout := captureStdout(t, func() { code = run(root, "lint", nil, nil, "") })
		if code != 1 {
			t.Fatalf("lint on broken repo exited %d, want 1", code)
		}
		got := finalStdoutLine(t, stdout)
		if !regexp.MustCompile(`^LINT: FAIL [1-9]\d* problem\(s\)$`).MatchString(got) {
			t.Errorf("final stdout line = %q, want LINT: FAIL <n> problem(s)", got)
		}
	})

	t.Run("fail on unreadable sources still ends with a verdict", func(t *testing.T) {
		var code int
		stdout := captureStdout(t, func() { code = run(t.TempDir(), "lint", nil, nil, "") })
		if code != 1 {
			t.Fatalf("lint on empty dir exited %d, want 1", code)
		}
		if got := finalStdoutLine(t, stdout); got != "LINT: FAIL 1 problem(s)" {
			t.Errorf("final stdout line = %q, want %q", got, "LINT: FAIL 1 problem(s)")
		}
	})

	t.Run("budget problems are counted in the verdict", func(t *testing.T) {
		root := verdictRepo(t)
		if err := os.WriteFile(filepath.Join(root, "WORDY.md"), []byte(strings.Repeat("word ", 50)), 0o644); err != nil {
			t.Fatal(err)
		}
		var code int
		stdout := captureStdout(t, func() { code = run(root, "lint", []string{"WORDY.md:10"}, nil, "") })
		if code != 1 {
			t.Fatalf("lint with blown budget exited %d, want 1", code)
		}
		if got := finalStdoutLine(t, stdout); got != "LINT: FAIL 1 problem(s)" {
			t.Errorf("final stdout line = %q, want %q", got, "LINT: FAIL 1 problem(s)")
		}
	})
}

func TestCheckVerdictLine(t *testing.T) {
	t.Run("pass", func(t *testing.T) {
		root := verdictRepo(t)
		if code := run(root, "write", nil, nil, ""); code != 0 {
			t.Fatalf("write run exited %d", code)
		}
		var code int
		stdout := captureStdout(t, func() { code = run(root, "check", nil, nil, "") })
		if code != 0 {
			t.Fatalf("check on fresh output exited %d, want 0", code)
		}
		if got := finalStdoutLine(t, stdout); got != "CHECK: PASS" {
			t.Errorf("final stdout line = %q, want %q", got, "CHECK: PASS")
		}
	})

	t.Run("drift reports count", func(t *testing.T) {
		root := verdictRepo(t)
		if code := run(root, "write", nil, nil, ""); code != 0 {
			t.Fatalf("write run exited %d", code)
		}
		if err := os.WriteFile(filepath.Join(root, "STATUS.md"), []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
		var code int
		stdout := captureStdout(t, func() { code = run(root, "check", nil, nil, "") })
		if code != 1 {
			t.Fatalf("check on stale output exited %d, want 1", code)
		}
		got := finalStdoutLine(t, stdout)
		if !regexp.MustCompile(`^CHECK: DRIFT [1-9]\d*$`).MatchString(got) {
			t.Errorf("final stdout line = %q, want CHECK: DRIFT <n>", got)
		}
	})
}

// Write and record modes are NOT part of the verdict contract — no LINT:/CHECK:
// line may leak into their stdout.
func TestNoVerdictOutsideGateModes(t *testing.T) {
	root := verdictRepo(t)
	stdout := captureStdout(t, func() {
		if code := run(root, "write", nil, nil, ""); code != 0 {
			t.Fatalf("write run exited %d", code)
		}
	})
	for _, forbidden := range []string{"LINT: ", "CHECK: "} {
		if strings.Contains(stdout, forbidden) {
			t.Errorf("write-mode stdout contains %q:\n%s", forbidden, stdout)
		}
	}
}

// The verdict is a pure formatting addition: for the same repo state, exit
// codes with and without a verdict-bearing mode agree (lint fail ⇔ 1,
// lint pass ⇔ 0). Guards against the contract ever becoming a gate change.
func TestVerdictMatchesExitCode(t *testing.T) {
	for _, broken := range []bool{false, true} {
		root := verdictRepo(t)
		if broken {
			readme := filepath.Join(root, "docs/streams/alpha/README.md")
			b, _ := os.ReadFile(readme)
			os.WriteFile(readme, []byte(strings.Replace(string(b), "status: active", "status: bogus", 1)), 0o644)
		}
		var code int
		stdout := captureStdout(t, func() { code = run(root, "lint", nil, nil, "") })
		last := finalStdoutLine(t, stdout)
		wantPass := code == 0
		if gotPass := last == "LINT: PASS"; gotPass != wantPass {
			t.Errorf("broken=%v: exit %d but verdict %q — verdict and exit code must agree", broken, code, last)
		}
	}
}
