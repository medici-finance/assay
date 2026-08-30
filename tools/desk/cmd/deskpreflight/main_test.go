package main

// Tests for the deskpreflight bundle.
//
// The suite is HERMETIC: every check's process edge AND its one filesystem edge
// are injected (see fakeTools), so no test shells a real grep/git/gofmt/go/
// statusgen/leak-sweep or reads a real tree. That is what lets `go test ./...`
// pass in any checkout — including a consumer tree with none of those tools
// installed. The skip-guarded real-tool cases at the end additionally exercise
// the REAL tools against the committed testdata trees and against a real
// throwaway git repository when they happen to be installed, but they never
// fail for their absence.
//
// The three load-bearing properties are each pinned here:
//   - every "tool missing" / "could not run" path is COULD-NOT-CHECK and exits
//     nonzero (a check that could not look is never a pass);
//   - the tool performs zero writes — asserted structurally by the fact that no
//     check is handed a writable edge at all (only lookPath, a read-capturing
//     output, and os.Lstat), and behaviourally by TestReadOnly_NoMutationEdge;
//     and
//   - the two work-tree checks see EXACTLY the `git ls-files` set and nothing
//     else, so `.git/` and ignored paths are structurally out of reach.

import (
	"crypto/rand"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// fakeTools builds an injected tools value with the default stat (every path a
// regular, zero-byte file). present decides lookPath; out is the per-invocation
// process result, so a test can answer "git rev-parse", "git ls-files" and "git
// status" (all named "git") differently by inspecting args.
func fakeTools(present map[string]bool, out func(dir, name string, args ...string) ([]byte, int, bool)) tools {
	return fakeToolsStat(present, out, nil)
}

// fakeToolsStat is fakeTools with the filesystem edge injected too, for the
// tests that care about a file's size or type.
func fakeToolsStat(present map[string]bool, out func(dir, name string, args ...string) ([]byte, int, bool), stat func(string) (os.FileInfo, error)) tools {
	if stat == nil {
		stat = statAllRegular
	}
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
		stat: stat,
	}
}

// fakeInfo is the minimal os.FileInfo the injected stat hands back.
type fakeInfo struct {
	name string
	size int64
	mode os.FileMode
}

func (f fakeInfo) Name() string       { return f.name }
func (f fakeInfo) Size() int64        { return f.size }
func (f fakeInfo) Mode() os.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return nil }

// statAllRegular is the default injected stat: every path is a regular,
// zero-byte file, so a test that does not care about size or type says nothing
// about them.
func statAllRegular(path string) (os.FileInfo, error) {
	return fakeInfo{name: filepath.Base(path)}, nil
}

// statSizes reports the named sizes, keyed by path SUFFIX so a test need not
// know the absolute root, and a regular zero-byte file for anything else.
func statSizes(sizes map[string]int64) func(string) (os.FileInfo, error) {
	return func(path string) (os.FileInfo, error) {
		slashed := filepath.ToSlash(path)
		for suffix, n := range sizes {
			if strings.HasSuffix(slashed, suffix) {
				return fakeInfo{name: filepath.Base(path), size: n}, nil
			}
		}
		return fakeInfo{name: filepath.Base(path)}, nil
	}
}

// nulJoin renders a path list exactly the way `git ls-files -z` writes it.
func nulJoin(paths ...string) []byte {
	if len(paths) == 0 {
		return nil
	}
	return []byte(strings.Join(paths, "\x00") + "\x00")
}

// lsFilesTools builds a tools value for the two work-tree checks: git present
// and answering `ls-files` with the given relative paths, grep present and
// answering grepOut/grepCode, and the default stat.
func lsFilesTools(paths []string, grepOut string, grepCode int) tools {
	return fakeTools(map[string]bool{"git": true, "grep": true},
		func(dir, name string, args ...string) ([]byte, int, bool) {
			if name == "git" {
				return nulJoin(paths...), 0, true
			}
			return []byte(grepOut), grepCode, true
		})
}

// allClean is the process behaviour of a spotless tree: git lists two ordinary
// work-tree files and reports nothing touched, grep finds no markers (exit 1),
// statusgen lints clean (exit 0). leak-sweep is left absent.
func allClean(dir, name string, args ...string) ([]byte, int, bool) {
	switch name {
	case "grep":
		return nil, 1, true // no conflict markers
	case "git":
		if len(args) > 0 {
			switch args[0] {
			case "rev-parse":
				return []byte("/some/repo\n"), 0, true
			case "ls-files":
				return nulJoin("README.md", "pkg/thing.go"), 0, true
			}
		}
		return nil, 0, true // git status: empty → no touched Go files
	case "statusgen":
		return []byte("LINT: PASS\n"), 0, true
	}
	return nil, 0, true
}

var cleanPresent = map[string]bool{"grep": true, "git": true, "statusgen": true}

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
	c := runConflictMarkers("/root", lsFilesTools([]string{"pkg/thing.go"},
		"pkg/thing.go:12:<<<<<<< HEAD\npkg/thing.go:14:=======\n", 0))
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
	// A failed line still ships the shape the check cannot see.
	if !strings.Contains(c.detail, conflictMarkerExclusion) {
		t.Fatalf("failed detail must print the declared exclusion, got %q", c.detail)
	}
}

func TestConflictMarkers_NoMatchIsClean(t *testing.T) {
	c := runConflictMarkers("/root", lsFilesTools([]string{"pkg/thing.go"}, "", 1))
	if c.state != deskkit.CheckedClean {
		t.Fatalf("grep exit 1 (no match) must be clean, got %v (%s)", c.state, c.detail)
	}
	if !strings.Contains(c.detail, conflictMarkerExclusion) {
		t.Fatalf("clean detail must print the declared exclusion, got %q", c.detail)
	}
}

func TestConflictMarkers_GrepMissingIsCouldNotCheck(t *testing.T) {
	c := runConflictMarkers("/root", fakeTools(map[string]bool{"git": true},
		func(string, string, ...string) ([]byte, int, bool) {
			return nulJoin("pkg/thing.go"), 0, true
		}))
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("missing grep must be could-not-check, got %v (%s)", c.state, c.detail)
	}
}

func TestConflictMarkers_GrepErrorIsCouldNotCheck(t *testing.T) {
	c := runConflictMarkers("/root", lsFilesTools([]string{"pkg/thing.go"},
		"grep: pkg/thing.go: I/O error", 2))
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("grep exit 2 (error) must be could-not-check, got %v (%s)", c.state, c.detail)
	}
}

// --- the work-tree scan input (issue #200) ----------------------------------

// TestConflictMarkers_GitMissingIsCouldNotCheck: without git there is no
// working-tree file set, so the check cannot look — and a check that could not
// look is never a pass, even though "scan nothing" would trivially find nothing.
func TestConflictMarkers_GitMissingIsCouldNotCheck(t *testing.T) {
	c := runConflictMarkers("/root", fakeTools(map[string]bool{"grep": true}, nil))
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("missing git must be could-not-check, got %v (%s)", c.state, c.detail)
	}
}

func TestJunkOversize_GitMissingIsCouldNotCheck(t *testing.T) {
	c := runJunkOversize("/root", fakeTools(map[string]bool{}, nil))
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("missing git must be could-not-check, got %v (%s)", c.state, c.detail)
	}
}

// A root that is not a git work tree (ls-files exits nonzero) is could-not-check
// for BOTH checks, with a detail that says so.
func TestWorkTreeChecks_NotAGitWorkTreeIsCouldNotCheck(t *testing.T) {
	notARepo := func(string, string, ...string) ([]byte, int, bool) {
		return []byte("fatal: not a git repository (or any of the parent directories): .git"), 128, true
	}
	for _, tc := range []struct {
		name string
		got  check
	}{
		{nameConflictMarkers, runConflictMarkers("/root", fakeTools(map[string]bool{"git": true, "grep": true}, notARepo))},
		{nameJunkOversize, runJunkOversize("/root", fakeTools(map[string]bool{"git": true}, notARepo))},
	} {
		if tc.got.state != deskkit.CouldNotCheck {
			t.Fatalf("%s: state = %v, want could-not-check (%s)", tc.name, tc.got.state, tc.got.detail)
		}
		if !strings.Contains(tc.got.detail, "ls-files") {
			t.Fatalf("%s: detail should name the failed enumeration, got %q", tc.name, tc.got.detail)
		}
	}
}

// TestConflictMarkers_DeclinesTestdataFixture is issue #200's first half: the
// tool's own committed fixture carries real markers by design, so a tree whose
// ONLY marker file is under testdata/ is clean — and grep is never even asked,
// because there is nothing left to scan.
func TestConflictMarkers_DeclinesTestdataFixture(t *testing.T) {
	grepCalled := false
	c := runConflictMarkers("/root", fakeTools(map[string]bool{"git": true, "grep": true},
		func(dir, name string, args ...string) ([]byte, int, bool) {
			if name == "git" {
				return nulJoin("tools/desk/cmd/deskpreflight/testdata/markers/conflict.txt"), 0, true
			}
			grepCalled = true
			return []byte("conflict.txt:4:<<<<<<< HEAD\n"), 0, true
		}))
	if c.state != deskkit.CheckedClean {
		t.Fatalf("a testdata-only tree must be clean, got %v (%s)", c.state, c.detail)
	}
	if grepCalled {
		t.Fatal("the declined fixture must never reach grep")
	}
	if !strings.Contains(c.detail, conflictMarkerExclusion) {
		t.Fatalf("clean detail must print the declared exclusion, got %q", c.detail)
	}
}

// TestWorkTreeChecks_ScanExactlyTheLsFilesSet is issue #200's second half made
// structural. `git ls-files` cannot emit a path under `.git/`, so the checks
// cannot see one — provided the scan input is EXACTLY the ls-files set. This
// asserts that identity for both checks: every path handed to grep, and every
// path handed to stat, comes from the listing (minus the declared exclusion for
// conflict-markers), and nothing else does.
func TestWorkTreeChecks_ScanExactlyTheLsFilesSet(t *testing.T) {
	listed := []string{"README.md", "docs/notes.md", "pkg/testdata/golden.txt"}
	// The shapes the old filesystem walk reported, none of which git can list.
	unlistable := []string{".git/objects/pack/pack-abc.pack", ".git/lost-found/other/deadbeef", "dist/bundle.js"}

	var grepped, statted, grepFlags []string
	tl := fakeToolsStat(map[string]bool{"git": true, "grep": true},
		func(dir, name string, args ...string) ([]byte, int, bool) {
			if name == "git" {
				return nulJoin(listed...), 0, true
			}
			for i, a := range args {
				if a == "--" {
					grepFlags = append(grepFlags, args[:i]...)
					grepped = append(grepped, args[i+1:]...)
					break
				}
			}
			return nil, 1, true
		},
		func(path string) (os.FileInfo, error) {
			statted = append(statted, path)
			return fakeInfo{name: filepath.Base(path)}, nil
		})

	if c := runConflictMarkers("/root", tl); c.state != deskkit.CheckedClean {
		t.Fatalf("conflict-markers: %v (%s)", c.state, c.detail)
	}
	// grep sees the listing minus the declared exclusion — no more, no less.
	want := []string{"README.md", "docs/notes.md"}
	if strings.Join(grepped, "|") != strings.Join(want, "|") {
		t.Fatalf("grep scanned %v, want exactly %v", grepped, want)
	}
	// -H is load-bearing now that grep is handed an explicit file list: without
	// it a single-file batch prints "12:<<<<<<<" with no path, and the reported
	// hit would name a line number instead of a file.
	if !strings.Contains(strings.Join(grepFlags, " "), "-H") {
		t.Fatalf("grep must be invoked with -H so every match names its file, got %v", grepFlags)
	}

	statted = nil
	if c := runJunkOversize("/root", tl); c.state != deskkit.CheckedClean {
		t.Fatalf("junk-oversize: %v (%s)", c.state, c.detail)
	}
	var wantStat []string
	for _, p := range listed {
		wantStat = append(wantStat, filepath.Join("/root", p))
	}
	if strings.Join(statted, "|") != strings.Join(wantStat, "|") {
		t.Fatalf("junk-oversize stat'ed %v, want exactly %v", statted, wantStat)
	}
	for _, u := range unlistable {
		for _, seen := range append(append([]string{}, grepped...), statted...) {
			if strings.Contains(seen, u) {
				t.Fatalf("a path git cannot list reached the scan: %q", seen)
			}
		}
	}
}

// A tracked path that is no longer a regular file on disk — deleted in the work
// tree but not yet staged, or a submodule gitlink — is dropped before grep sees
// it. Handing it to grep would make grep exit 2, turning an ordinary
// mid-refactor tree into a blocked gate.
func TestConflictMarkers_MissingWorkTreeFileIsSkipped(t *testing.T) {
	for _, tc := range []struct {
		name string
		stat func(string) (os.FileInfo, error)
	}{
		{"deleted in the work tree", func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }},
		{"a gitlink stats as a directory", func(p string) (os.FileInfo, error) {
			return fakeInfo{name: filepath.Base(p), mode: os.ModeDir}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			grepCalled := false
			c := runConflictMarkers("/root", fakeToolsStat(map[string]bool{"git": true, "grep": true},
				func(dir, name string, args ...string) ([]byte, int, bool) {
					if name == "git" {
						return nulJoin("gone"), 0, true
					}
					grepCalled = true
					return nil, 2, true
				}, tc.stat))
			if c.state != deskkit.CheckedClean {
				t.Fatalf("must not block the check, got %v (%s)", c.state, c.detail)
			}
			if grepCalled {
				t.Fatal("a path that is not a regular file must never reach grep")
			}
		})
	}
}

func TestUnderDeclinedPath(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"tools/desk/cmd/deskpreflight/testdata/markers/conflict.txt", true},
		{"testdata/x.txt", true},
		{"a/testdata/b/c/d.txt", true},
		{"README.md", false},
		{"pkg/testdata", false},    // a FILE named testdata is still scanned
		{"pkg/testdatax/a", false}, // only the exact segment counts
		{"pkg/mytestdata/a", false},
	} {
		if got := underDeclinedPath(tc.path); got != tc.want {
			t.Fatalf("underDeclinedPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestBatchPaths_SplitsWithoutLosingAPath(t *testing.T) {
	paths := []string{"aaaa", "bbbb", "cccc", "dddd"}
	batches := batchPaths(paths, 10) // ~2 paths per batch
	if len(batches) < 2 {
		t.Fatalf("expected the budget to force a split, got %v", batches)
	}
	var flat []string
	for _, b := range batches {
		flat = append(flat, b...)
	}
	if strings.Join(flat, "|") != strings.Join(paths, "|") {
		t.Fatalf("batching lost or reordered paths: %v", flat)
	}
	// A single path longer than the budget still gets scanned.
	long := strings.Repeat("x", 64)
	if got := batchPaths([]string{long}, 10); len(got) != 1 || got[0][0] != long {
		t.Fatalf("an over-budget path must still be scanned, got %v", got)
	}
}

// --- junk / oversize --------------------------------------------------------

func TestJunkOversize_FlagsHits(t *testing.T) {
	c := runJunkOversize("/root", fakeToolsStat(map[string]bool{"git": true},
		func(string, string, ...string) ([]byte, int, bool) {
			return nulJoin("a.orig", "big.bin", "ok.txt"), 0, true
		},
		statSizes(map[string]int64{"big.bin": oversizeBytes + 1})))
	if c.state != deskkit.CheckedFailed {
		t.Fatalf("state = %v, want checked-failed (%s)", c.state, c.detail)
	}
	if !strings.Contains(c.detail, "a.orig") || !strings.Contains(c.detail, "big.bin") {
		t.Fatalf("detail must name the hits, got %q", c.detail)
	}
	if strings.Contains(c.detail, "ok.txt") {
		t.Fatalf("an ordinary file must not be reported, got %q", c.detail)
	}
}

func TestJunkOversize_NoneIsClean(t *testing.T) {
	c := runJunkOversize("/root", fakeTools(map[string]bool{"git": true},
		func(string, string, ...string) ([]byte, int, bool) {
			return nulJoin("README.md"), 0, true
		}))
	if c.state != deskkit.CheckedClean {
		t.Fatalf("an ordinary work tree must be clean, got %v (%s)", c.state, c.detail)
	}
}

// The threshold is STRICTLY greater, exactly as `find -size +Nc` was: a file of
// precisely oversizeBytes is not a hit, and one byte more is.
func TestJunkOversize_ThresholdIsStrict(t *testing.T) {
	at := runJunkOversize("/root", fakeToolsStat(map[string]bool{"git": true},
		func(string, string, ...string) ([]byte, int, bool) { return nulJoin("edge.bin"), 0, true },
		statSizes(map[string]int64{"edge.bin": oversizeBytes})))
	if at.state != deskkit.CheckedClean {
		t.Fatalf("a file of exactly oversizeBytes must be clean, got %v (%s)", at.state, at.detail)
	}
	over := runJunkOversize("/root", fakeToolsStat(map[string]bool{"git": true},
		func(string, string, ...string) ([]byte, int, bool) { return nulJoin("edge.bin"), 0, true },
		statSizes(map[string]int64{"edge.bin": oversizeBytes + 1})))
	if over.state != deskkit.CheckedFailed {
		t.Fatalf("one byte over the threshold must be failed, got %v (%s)", over.state, over.detail)
	}
}

// The junk-name list is matched against the BASE name, and the whole list is
// still live — a rename of the check's mechanism must not quietly drop a shape.
func TestJunkOversize_EveryJunkNameStillMatches(t *testing.T) {
	for _, base := range []string{"a.orig", "a.rej", "a.swp", "a~", ".DS_Store", "Thumbs.db"} {
		if !isJunkName(base) {
			t.Fatalf("%q must be matched by the junk-name list", base)
		}
	}
	for _, base := range []string{"main.go", "orig.txt", "README.md", "swap.c"} {
		if isJunkName(base) {
			t.Fatalf("%q must NOT be matched by the junk-name list", base)
		}
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
//
// Each fixture is copied into a throwaway GIT REPOSITORY rather than being
// scanned where it sits. --root now has to name a git work tree — that is the
// whole fix — and the fixture directories are only inside one when the package
// happens to be tested from a checkout. Copying makes the case say what it
// means, and keeps it green wherever the package is built from: an exported
// tarball, a container build context, a mutation harness's scratch copy.
func TestRealFixtures(t *testing.T) {
	requireRealTools(t, "grep", "git", "statusgen")

	t.Run("clean", func(t *testing.T) {
		dir := realRepoFrom(t, "testdata/clean")
		var b strings.Builder
		code := run([]string{"--root", dir}, &b, &b, realTools())
		if code != deskkit.ExitOK || !strings.Contains(b.String(), "PREFLIGHT: PASS") {
			t.Fatalf("clean fixture: exit=%d\n%s", code, b.String())
		}
	})

	// The markers fixture is scanned, not declined: the exclusion is matched on
	// the path RELATIVE to --root, and here conflict.txt sits at the root. A
	// fixture is only invisible to the check when --root is above its testdata/
	// directory, which is exactly the case issue #200 reported.
	t.Run("markers", func(t *testing.T) {
		dir := realRepoFrom(t, "testdata/markers")
		var b strings.Builder
		code := run([]string{"--root", dir}, &b, &b, realTools())
		if code == deskkit.ExitOK {
			t.Fatalf("markers fixture must refuse:\n%s", b.String())
		}
		if !strings.Contains(b.String(), "conflict.txt") {
			t.Fatalf("markers fixture output must name conflict.txt:\n%s", b.String())
		}
	})
}

// TestRun_NonGitRootIsCouldNotCheck: pointed at a directory that is not a git
// work tree, the two work-tree checks cannot enumerate anything, so they report
// could-not-check and the run exits nonzero. "No files to scan" is not a pass.
func TestRun_NonGitRootIsCouldNotCheck(t *testing.T) {
	requireRealTools(t, "git")
	dir := t.TempDir()
	// A TMPDIR that happened to sit inside a checkout would make this vacuous.
	probe := exec.Command("git", "rev-parse", "--show-toplevel")
	probe.Dir = dir
	if err := probe.Run(); err == nil {
		t.Skip("could-not-check: the temp dir is itself inside a git work tree")
	}
	c := runJunkOversize(dir, realTools())
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("a non-repo root must be could-not-check, got %v (%s)", c.state, c.detail)
	}
	c = runConflictMarkers(dir, realTools())
	if c.state != deskkit.CouldNotCheck {
		t.Fatalf("a non-repo root must be could-not-check, got %v (%s)", c.state, c.detail)
	}
}

// --- skip-guarded real-tool tests against a REAL git repository -------------
//
// These are the regression tests for issue #200. Each builds a throwaway git
// repository, runs the REAL checks over it, and asserts a property the hermetic
// suite can only approximate: what `git ls-files` actually emits, and what
// actually lands under `.git/`. They skip — never fail — when the underlying
// tools are absent.

// markerBody carries a real conflict hunk. It is ASSEMBLED rather than written
// out so this source file never itself contains a seven-character marker run at
// line start — the shape every marker scanner, this one included, looks for.
var markerBody = strings.Repeat("<", 7) + " HEAD\nours\n" +
	strings.Repeat("=", 7) + "\ntheirs\n" +
	strings.Repeat(">", 7) + " feature-branch\n"

func requireRealTools(t *testing.T, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := exec.LookPath(n); err != nil {
			t.Skipf("real-repo test needs %s on PATH: %v", n, err)
		}
	}
}

// gitIn runs a real git command in dir, with an inline identity so the test does
// not depend on the machine's git config.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.name=deskpreflight test",
		"-c", "user.email=deskpreflight@example.invalid",
		"-c", "commit.gpgsign=false",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// realRepoFrom copies a committed fixture into a temp dir and makes it a real
// git repository with one commit.
func realRepoFrom(t *testing.T, fixture string) string {
	t.Helper()
	dir := t.TempDir()
	copyTree(t, fixture, dir)
	gitIn(t, dir, "init", "-q", "-b", "main")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "seed")
	return dir
}

// realRepo seeds from the committed CLEAN fixture precisely because the suite
// already proves that tree reaches PREFLIGHT: PASS, so anything a case below
// reports is the thing that case introduced.
func realRepo(t *testing.T) string {
	t.Helper()
	return realRepoFrom(t, "testdata/clean")
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	}); err != nil {
		t.Fatal(err)
	}
}

// writeIncompressible writes n bytes of random data, so the git object made from
// it stays above the size threshold after zlib.
func writeIncompressible(t *testing.T, path string, n int) {
	t.Helper()
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

// hasOversizeUnderGitDir reports whether the repository's own storage holds a
// file bigger than the threshold — the precondition the first case depends on.
// Asserting it turns "the check passed" into "the check passed WITH the trap set".
func hasOversizeUnderGitDir(t *testing.T, dir string) bool {
	t.Helper()
	found := false
	_ = filepath.WalkDir(filepath.Join(dir, ".git"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // an unreadable corner is simply not evidence
		}
		if fi, err := d.Info(); err == nil && fi.Size() > oversizeBytes {
			found = true
		}
		return nil
	})
	return found
}

// TestRealRepo_GitStorageIsOutOfScope: a repository whose OBJECT STORE holds a
// blob past the size threshold is clean. Git's own storage is not something a
// worker is about to commit, and reporting it made deskpreflight unable to reach
// exit 0 in any repository with more than 5 MiB of history.
func TestRealRepo_GitStorageIsOutOfScope(t *testing.T) {
	requireRealTools(t, "grep", "git", "statusgen")
	dir := realRepo(t)

	writeIncompressible(t, filepath.Join(dir, "big.bin"), 6*1024*1024)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "add a large blob")
	// Drop it from the work tree AND the index; the object survives in .git.
	gitIn(t, dir, "rm", "-q", "big.bin")
	gitIn(t, dir, "commit", "-q", "-m", "drop the large blob")

	if !hasOversizeUnderGitDir(t, dir) {
		t.Skip("could-not-check: no object over the threshold materialised under .git")
	}

	var b strings.Builder
	code := run([]string{"--root", dir}, &b, &b, realTools())
	if !strings.Contains(b.String(), nameJunkOversize+": checked-clean") {
		t.Fatalf("git's own storage must be out of scope:\n%s", b.String())
	}
	if code != deskkit.ExitOK || !strings.Contains(b.String(), "PREFLIGHT: PASS") {
		t.Fatalf("a real repo with a large object must pass: exit=%d\n%s", code, b.String())
	}
}

// The cases below drive the CHECK under test directly rather than the whole
// bundle. They still use realTools() — real git, real grep, a real repository —
// but they do not pay for statusgen, which dominates a full run() and has
// nothing to say about the scan scope these cases are about. The end-to-end
// exit-0 path is covered once, above.

// The check must still catch the thing it exists for: a large file actually
// sitting in the work tree, untracked, is exactly what a worker is about to add.
func TestRealRepo_UntrackedOversizeIsStillFlagged(t *testing.T) {
	requireRealTools(t, "git")
	dir := realRepo(t)
	writeIncompressible(t, filepath.Join(dir, "stray.bin"), 6*1024*1024)

	c := runJunkOversize(dir, realTools())
	if c.state != deskkit.CheckedFailed {
		t.Fatalf("an untracked oversize file must be flagged, got %v (%s)", c.state, c.detail)
	}
	if !strings.Contains(c.detail, "stray.bin") {
		t.Fatalf("detail must name the oversize file, got %q", c.detail)
	}
}

// Build output .gitignore excludes is not the worker's diff, so it is not the
// gate's business — this is what stops a sibling worktree's dist/ from failing
// an unrelated PR.
func TestRealRepo_IgnoredOversizeIsNotFlagged(t *testing.T) {
	requireRealTools(t, "git")
	dir := realRepo(t)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("dist/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeIncompressible(t, filepath.Join(dir, "dist", "bundle.bin"), 6*1024*1024)
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "ignore build output")

	c := runJunkOversize(dir, realTools())
	if c.state != deskkit.CheckedClean {
		t.Fatalf("gitignored output must be out of scope, got %v (%s)", c.state, c.detail)
	}
}

// TestRealRepo_TestdataDeclinedButRealMarkersAreNot pins both halves of the
// exclusion against real grep: a fixture under testdata/ is declined (and the
// output says so), while the same bytes one directory up still refuse.
func TestRealRepo_TestdataDeclinedButRealMarkersAreNot(t *testing.T) {
	requireRealTools(t, "git", "grep")
	dir := realRepo(t)

	if err := os.MkdirAll(filepath.Join(dir, "pkg", "testdata"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "testdata", "conflict.txt"), []byte(markerBody), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "add a marker fixture")

	c := runConflictMarkers(dir, realTools())
	if c.state != deskkit.CheckedClean {
		t.Fatalf("a committed testdata fixture must not fail the check, got %v (%s)", c.state, c.detail)
	}
	if !strings.Contains(c.detail, conflictMarkerExclusion) {
		t.Fatalf("the clean detail must print the declared exclusion, got %q", c.detail)
	}

	// The same bytes outside testdata/ are a real unresolved merge.
	if err := os.WriteFile(filepath.Join(dir, "pkg", "real.txt"), []byte(markerBody), 0o644); err != nil {
		t.Fatal(err)
	}
	c = runConflictMarkers(dir, realTools())
	if c.state != deskkit.CheckedFailed {
		t.Fatalf("a real conflict marker must still refuse, got %v (%s)", c.state, c.detail)
	}
	if !strings.Contains(c.detail, "pkg/real.txt") {
		t.Fatalf("detail must name the file carrying markers, got %q", c.detail)
	}
	if strings.Contains(c.detail, "conflict.txt") {
		t.Fatalf("the declined fixture must not be named as a hit, got %q", c.detail)
	}
}
