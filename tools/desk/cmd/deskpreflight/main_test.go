package main

// Tests for the deskpreflight bundle.
//
// The suite is HERMETIC: every check's process edge is injected (see fakeTools),
// so no test shells a real grep/find/git/gofmt/go/statusgen/leak-sweep. That is
// what lets `go test ./...` pass in any checkout — including a consumer tree with
// none of those tools installed. The skip-guarded Test*Fixture cases at the end
// additionally exercise the REAL tools against the committed testdata trees when
// they happen to be installed, but they never fail for their absence.
//
// The two load-bearing properties are each pinned here:
//   - every "tool missing" / "could not run" path is COULD-NOT-CHECK and exits
//     nonzero (a check that could not look is never a pass); and
//   - the tool performs zero writes — asserted structurally by the fact that no
//     check is handed a writable edge at all (only lookPath + a read-capturing
//     output), and behaviourally by TestReadOnly_NoMutationEdge.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeTools builds an injected tools value. present decides lookPath; out is the
// per-invocation process result, so a test can answer "git rev-parse" and "git
// status" (both name "git") differently by inspecting args.
func fakeTools(present map[string]bool, out func(dir, name string, args ...string) ([]byte, int, bool)) tools {
	return tools{
		lookPath: func(n string) (string, error) {
			if present[n] {
				return "/fake/bin/" + n, nil
			}
			return "", fmt.Errorf("%s: not found in $PATH", n)
		},
		output: func(dir, name string, args ...string) ([]byte, int, bool) {
			if out == nil {
				return nil, 0, true
			}
			return out(dir, name, args...)
		},
	}
}

// allClean is the process behaviour of a spotless tree: grep finds nothing
// (exit 1), find lists nothing (exit 0), git reports no touched files, statusgen
// lints clean (exit 0). leak-sweep is left absent.
func allClean(dir, name string, args ...string) ([]byte, int, bool) {
	switch name {
	case "grep":
		return nil, 1, true // no conflict markers
	case "find":
		return nil, 0, true // nothing junk/oversize
	case "git":
		if len(args) > 0 && args[0] == "rev-parse" {
			return []byte("/some/repo\n"), 0, true
		}
		return nil, 0, true // git status: empty → no touched Go files
	case "statusgen":
		return []byte("LINT: PASS\n"), 0, true
	}
	return nil, 0, true
}

var cleanPresent = map[string]bool{"grep": true, "find": true, "git": true, "statusgen": true}

func TestRun_AllClean_Passes(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--root", t.TempDir()}, &stdout, &stderr, fakeTools(cleanPresent, allClean))
	if code != deskkit.ExitOK {
		t.Fatalf("clean tree exit = %d, want %d\n%s", code, deskkit.ExitOK, stdout.String())
	}
	if !strings.Contains(stdout.String(), "PREFLIGHT: PASS") {
		t.Fatalf("clean tree did not print PREFLIGHT: PASS:\n%s", stdout.String())
	}
}

// TestRun_AllToolsMissing_CouldNotCheck is the hermetic twin of Verify item 4:
// with nothing on PATH, no check can look, so the whole preflight is
// could-not-check and exits nonzero. leak-sweep, being conditional, is simply
// omitted — it must not turn into a could-not-check.
func TestRun_AllToolsMissing_CouldNotCheck(t *testing.T) {
	var stdout, stderr strings.Builder
	code := run([]string{"--root", t.TempDir()}, &stdout, &stderr,
		fakeTools(map[string]bool{}, func(string, string, ...string) ([]byte, int, bool) {
			t.Fatal("output must never be called when no tool is on PATH")
			return nil, 0, false
		}))
	if code == deskkit.ExitOK {
		t.Fatalf("no tools present must NOT exit 0:\n%s", stdout.String())
	}
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("exit = %d, want %d (could-not-check):\n%s", code, deskkit.ExitUnverifiable, stdout.String())
	}
	if !strings.Contains(stdout.String(), "could-not-check") {
		t.Fatalf("output must contain could-not-check:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), nameLeakSweep) {
		t.Fatalf("absent leak-sweep must be omitted, not reported:\n%s", stdout.String())
	}
}

// --- conflict-markers -------------------------------------------------------

func TestConflictMarkers_NamesFile(t *testing.T) {
	c := runConflictMarkers("/root", fakeTools(map[string]bool{"grep": true},
		func(string, string, ...string) ([]byte, int, bool) {
			return []byte("/root/pkg/thing.go:12:<<<<<<< HEAD\n/root/pkg/thing.go:14:=======\n"), 0, true
		}))
	if c.state != deskkit.CheckedFailed {
		t.Fatalf("state = %v, want checked-failed", c.state)
	}
	if !strings.Contains(c.detail, "thing.go") {
		t.Fatalf("detail must name the file, got %q", c.detail)
	}
	// The file is named ONCE even though grep matched two lines in it.
	if strings.Count(c.detail, "thing.go") != 1 {
		t.Fatalf("file should be listed once, got %q", c.detail)
	}
}

func TestConflictMarkers_NoMatchIsClean(t *testing.T) {
	c := runConflictMarkers("/root", fakeTools(map[string]bool{"grep": true},
		func(string, string, ...string) ([]byte, int, bool) { return nil, 1, true }))
	if c.state != deskkit.CheckedClean {
		t.Fatalf("grep exit 1 (no match) must be clean, got %v (%s)", c.state, c.detail)
	}
}

func TestConflictMarkers_GrepMissingIsCouldNotCheck(t *testing.T) {
	c := runConflictMarkers("/root", fakeTools(map[string]bool{}, nil))
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("missing grep must be could-not-check, got %v", c.state)
	}
}

func TestConflictMarkers_GrepErrorIsCouldNotCheck(t *testing.T) {
	c := runConflictMarkers("/root", fakeTools(map[string]bool{"grep": true},
		func(string, string, ...string) ([]byte, int, bool) { return []byte("grep: /root: I/O error"), 2, true }))
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("grep exit 2 (error) must be could-not-check, got %v (%s)", c.state, c.detail)
	}
}

// --- junk / oversize --------------------------------------------------------

func TestJunkOversize_FlagsHits(t *testing.T) {
	c := runJunkOversize("/root", fakeTools(map[string]bool{"find": true},
		func(string, string, ...string) ([]byte, int, bool) {
			return []byte("/root/a.orig\n/root/big.bin\n"), 0, true
		}))
	if c.state != deskkit.CheckedFailed {
		t.Fatalf("state = %v, want checked-failed (%s)", c.state, c.detail)
	}
	if !strings.Contains(c.detail, "a.orig") || !strings.Contains(c.detail, "big.bin") {
		t.Fatalf("detail must name the hits, got %q", c.detail)
	}
}

func TestJunkOversize_NoneIsClean(t *testing.T) {
	c := runJunkOversize("/root", fakeTools(map[string]bool{"find": true},
		func(string, string, ...string) ([]byte, int, bool) { return nil, 0, true }))
	if c.state != deskkit.CheckedClean {
		t.Fatalf("empty find output must be clean, got %v", c.state)
	}
}

func TestJunkOversize_FindMissingIsCouldNotCheck(t *testing.T) {
	c := runJunkOversize("/root", fakeTools(map[string]bool{}, nil))
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("missing find must be could-not-check, got %v", c.state)
	}
}

// --- gofmt + go vet ---------------------------------------------------------

// goFmtEnv builds an env for the go-fmt-vet check around a real temp dir holding
// one real .go file (touchedGoFiles stats the path, so it must exist), with git
// reporting that file as touched. out lets each test decide gofmt/go behaviour.
func goFmtEnv(t *testing.T, present map[string]bool, gofmtOut string, gofmtCode int, gofmtRan bool, vetCode int, vetRan bool) (string, tools) {
	t.Helper()
	root := t.TempDir()
	goPath := filepath.Join(root, "x.go")
	if err := os.WriteFile(goPath, []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := func(dir, name string, args ...string) ([]byte, int, bool) {
		switch name {
		case "git":
			if len(args) > 0 && args[0] == "rev-parse" {
				return []byte(root + "\n"), 0, true
			}
			return []byte(" M x.go\n"), 0, true // touched
		case "gofmt":
			return []byte(gofmtOut), gofmtCode, gofmtRan
		case "go":
			return []byte("vet output"), vetCode, vetRan
		}
		return nil, 0, true
	}
	return root, fakeTools(present, out)
}

func TestGoFmtVet_UnformattedIsFailed(t *testing.T) {
	root, tl := goFmtEnv(t, map[string]bool{"git": true, "gofmt": true, "go": true},
		filepath.Join("x.go")+"\n", 0, true, 0, true)
	c := runGoFmtVet(root, tl)
	if c.state != deskkit.CheckedFailed {
		t.Fatalf("state = %v, want checked-failed (%s)", c.state, c.detail)
	}
	if !strings.Contains(c.detail, "not formatted") {
		t.Fatalf("detail should mention formatting, got %q", c.detail)
	}
}

func TestGoFmtVet_VetFailureIsFailed(t *testing.T) {
	root, tl := goFmtEnv(t, map[string]bool{"git": true, "gofmt": true, "go": true},
		"", 0, true, 1, true) // gofmt clean, vet exits 1
	c := runGoFmtVet(root, tl)
	if c.state != deskkit.CheckedFailed {
		t.Fatalf("state = %v, want checked-failed (%s)", c.state, c.detail)
	}
	if !strings.Contains(c.detail, "vet") {
		t.Fatalf("detail should mention vet, got %q", c.detail)
	}
}

func TestGoFmtVet_CleanWhenFormattedAndVetPasses(t *testing.T) {
	root, tl := goFmtEnv(t, map[string]bool{"git": true, "gofmt": true, "go": true},
		"", 0, true, 0, true)
	c := runGoFmtVet(root, tl)
	if c.state != deskkit.CheckedClean {
		t.Fatalf("state = %v, want checked-clean (%s)", c.state, c.detail)
	}
}

func TestGoFmtVet_GitMissingIsCouldNotCheck(t *testing.T) {
	c := runGoFmtVet(t.TempDir(), fakeTools(map[string]bool{}, nil))
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("missing git must be could-not-check, got %v (%s)", c.state, c.detail)
	}
}

// A missing gofmt while there ARE touched Go files is could-not-check — the
// check we intended to run could not run. (Contrast leak-sweep's "where
// present", which is conditional by design.)
func TestGoFmtVet_GofmtMissingWithTouchedGoIsCouldNotCheck(t *testing.T) {
	root, tl := goFmtEnv(t, map[string]bool{"git": true}, "", 0, true, 0, true) // gofmt/go absent
	c := runGoFmtVet(root, tl)
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("missing gofmt with touched Go must be could-not-check, got %v (%s)", c.state, c.detail)
	}
}

func TestGoFmtVet_NoTouchedGoIsClean(t *testing.T) {
	root := t.TempDir()
	tl := fakeTools(map[string]bool{"git": true}, func(dir, name string, args ...string) ([]byte, int, bool) {
		if len(args) > 0 && args[0] == "rev-parse" {
			return []byte(root + "\n"), 0, true
		}
		return []byte(" M README.md\n"), 0, true // touched, but not Go
	})
	c := runGoFmtVet(root, tl)
	if c.state != deskkit.CheckedClean {
		t.Fatalf("no touched Go files must be clean, got %v (%s)", c.state, c.detail)
	}
}

// --- statusgen --------------------------------------------------------------

func TestStatusgen_CleanExit0(t *testing.T) {
	c := runStatusgenLint("/abs", fakeTools(map[string]bool{"statusgen": true},
		func(string, string, ...string) ([]byte, int, bool) { return []byte("LINT: PASS\n"), 0, true }))
	if c.state != deskkit.CheckedClean {
		t.Fatalf("statusgen exit 0 must be clean, got %v (%s)", c.state, c.detail)
	}
}

func TestStatusgen_NonzeroIsFailed(t *testing.T) {
	c := runStatusgenLint("/abs", fakeTools(map[string]bool{"statusgen": true},
		func(string, string, ...string) ([]byte, int, bool) {
			return []byte("assay-config: noise\nPROBLEM: bad board\nLINT: FAIL 1 problem(s)\n"), 1, true
		}))
	if c.state != deskkit.CheckedFailed {
		t.Fatalf("statusgen nonzero must be failed, got %v", c.state)
	}
	if !strings.Contains(c.detail, "PROBLEM: bad board") {
		t.Fatalf("detail should surface the PROBLEM line, got %q", c.detail)
	}
}

func TestStatusgen_MissingIsCouldNotCheck(t *testing.T) {
	c := runStatusgenLint("/abs", fakeTools(map[string]bool{}, nil))
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("missing statusgen must be could-not-check, got %v", c.state)
	}
}

// statusgen runs from a NEUTRAL cwd (os.TempDir), passing the ABSOLUTE root, so
// it scans exactly --root and never the worker's cwd.
func TestStatusgen_RunsFromNeutralCwdWithAbsRoot(t *testing.T) {
	var gotDir, gotRoot string
	c := runStatusgenLint("/abs/root", fakeTools(map[string]bool{"statusgen": true},
		func(dir, name string, args ...string) ([]byte, int, bool) {
			gotDir = dir
			for i, a := range args {
				if a == "--root" && i+1 < len(args) {
					gotRoot = args[i+1]
				}
			}
			return nil, 0, true
		}))
	if c.state != deskkit.CheckedClean {
		t.Fatalf("unexpected state %v", c.state)
	}
	if gotDir != os.TempDir() {
		t.Fatalf("statusgen cwd = %q, want neutral %q", gotDir, os.TempDir())
	}
	if gotRoot != "/abs/root" {
		t.Fatalf("statusgen --root = %q, want the absolute root", gotRoot)
	}
}

// --- leak-sweep (where present) ---------------------------------------------

func TestLeakSweep_AbsentIsOmitted(t *testing.T) {
	_, present := runLeakSweep("/abs", fakeTools(map[string]bool{}, nil))
	if present {
		t.Fatal("absent leak-sweep must report present=false (omitted, not could-not-check)")
	}
}

func TestLeakSweep_PresentCleanAndFailed(t *testing.T) {
	c, present := runLeakSweep("/abs", fakeTools(map[string]bool{"leaksweep": true},
		func(string, string, ...string) ([]byte, int, bool) { return nil, 0, true }))
	if !present || c.state != deskkit.CheckedClean {
		t.Fatalf("present clean leak-sweep: present=%v state=%v", present, c.state)
	}
	c, present = runLeakSweep("/abs", fakeTools(map[string]bool{"leaksweep": true},
		func(string, string, ...string) ([]byte, int, bool) { return []byte("LEAK: token X"), 1, true }))
	if !present || c.state != deskkit.CheckedFailed {
		t.Fatalf("present failing leak-sweep: present=%v state=%v", present, c.state)
	}
}

func TestLeakSweep_PresentButUnrunnableIsCouldNotCheck(t *testing.T) {
	c, present := runLeakSweep("/abs", fakeTools(map[string]bool{"leaksweep": true},
		func(string, string, ...string) ([]byte, int, bool) { return nil, -1, false }))
	if !present || c.state != deskkit.CouldNotCheck {
		t.Fatalf("installed-but-unrunnable leak-sweep must be could-not-check: present=%v state=%v", present, c.state)
	}
}

// --- report precedence + summary --------------------------------------------

func TestReport_CouldNotCheckOutranksFailed(t *testing.T) {
	var b strings.Builder
	code := report([]check{
		failedCheck("a", "broke"),
		couldNotCheck("b", "blind"),
	}, &b)
	if code != deskkit.ExitUnverifiable {
		t.Fatalf("could-not-check present must exit %d, got %d", deskkit.ExitUnverifiable, code)
	}
	if !strings.Contains(b.String(), "PREFLIGHT: COULD-NOT-CHECK 1") {
		t.Fatalf("headline should be COULD-NOT-CHECK 1:\n%s", b.String())
	}
}

func TestReport_FailedOnlyExitsRefused(t *testing.T) {
	var b strings.Builder
	code := report([]check{clean("a", "ok"), failedCheck("b", "broke")}, &b)
	if code != deskkit.ExitRefused {
		t.Fatalf("failed-only must exit %d, got %d", deskkit.ExitRefused, code)
	}
	if !strings.Contains(b.String(), "PREFLIGHT: FAIL 1") {
		t.Fatalf("headline should be FAIL 1:\n%s", b.String())
	}
}

func TestReport_EmptyIsNeverPass(t *testing.T) {
	var b strings.Builder
	if code := report(nil, &b); code == deskkit.ExitOK {
		t.Fatalf("a preflight that ran no checks must not pass:\n%s", b.String())
	}
}

// --- read-only structural guard ---------------------------------------------

// TestReadOnly_NoMutationEdge pins that the injected surface is read-only by
// construction: the tools value exposes only lookPath + output, and output is
// only ever consulted for its captured bytes/exit — there is no field a check
// could write the tree through. This is the P-Control property made mechanical.
func TestReadOnly_NoMutationEdge(t *testing.T) {
	calls := 0
	tl := fakeTools(cleanPresent, func(dir, name string, args ...string) ([]byte, int, bool) {
		calls++
		// No check may pass a mutating verb to any tool.
		joined := name + " " + strings.Join(args, " ")
		for _, banned := range []string{"-w", "-delete", "-exec", "--fix", "commit", "add ", "push", "-i "} {
			if strings.Contains(joined, banned) {
				t.Fatalf("check invoked a mutating verb: %q", joined)
			}
		}
		return allClean(dir, name, args...)
	})
	var b strings.Builder
	if code := run([]string{"--root", t.TempDir()}, &b, &b, tl); code != deskkit.ExitOK {
		t.Fatalf("clean run unexpectedly failed: %d\n%s", code, b.String())
	}
	if calls == 0 {
		t.Fatal("expected the checks to consult the process edge")
	}
}

// --- skip-guarded real-tool smoke tests against the committed fixtures -------

// TestRealFixtures runs the REAL checks against the committed testdata trees,
// but only when the underlying tools are installed — never failing for their
// absence (that is what keeps `go test ./...` green in a bare checkout). It is
// the in-suite echo of Verify items 2 and 3.
func TestRealFixtures(t *testing.T) {
	for _, tool := range []string{"grep", "find", "git", "statusgen"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("real-tool smoke test needs %s on PATH: %v", tool, err)
		}
	}

	t.Run("clean", func(t *testing.T) {
		var b strings.Builder
		code := run([]string{"--root", "testdata/clean"}, &b, &b, realTools())
		if code != deskkit.ExitOK || !strings.Contains(b.String(), "PREFLIGHT: PASS") {
			t.Fatalf("clean fixture: exit=%d\n%s", code, b.String())
		}
	})

	t.Run("markers", func(t *testing.T) {
		var b strings.Builder
		code := run([]string{"--root", "testdata/markers"}, &b, &b, realTools())
		if code == deskkit.ExitOK {
			t.Fatalf("markers fixture must refuse:\n%s", b.String())
		}
		if !strings.Contains(b.String(), "conflict.txt") {
			t.Fatalf("markers fixture output must name conflict.txt:\n%s", b.String())
		}
	})
}
