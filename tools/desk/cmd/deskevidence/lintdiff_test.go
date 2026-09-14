package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// lintdiff_test.go — unit coverage for lintDiffAt's own staging/diff/restore mechanics and
// for statusgenLintAt's real subprocess plumbing. deskevidence_test.go's TestLintDiff*
// functions exercise cmdEvidence's INTEGRATION with the lintDiffFn seam (stubbed at the top
// level); this file goes one level down, against a real t.TempDir() root — never the source
// tree — with statusgenLintFn stubbed so no test here needs a real statusgen on PATH either.

// stubStatusgenLint installs a sequenced statusgenLintFn: the first call returns before,
// the second returns after, and any further call is a test failure (lintDiffAt calls it
// exactly twice — once before staging, once after).
func stubStatusgenLint(t *testing.T, before, after []string) {
	t.Helper()
	old := statusgenLintFn
	calls := 0
	statusgenLintFn = func(root string) ([]string, error) {
		calls++
		switch calls {
		case 1:
			return before, nil
		case 2:
			return after, nil
		default:
			t.Fatalf("statusgenLintFn called a 3rd time (root=%q) — lintDiffAt must call it exactly twice", root)
			return nil, nil
		}
	}
	t.Cleanup(func() { statusgenLintFn = old })
}

// TestLintDiffAtIntroducedOnly: a problem present in BOTH before and after is pre-existing
// (never counted as introduced); a problem present only in after is diff-introduced.
func TestLintDiffAtIntroducedOnly(t *testing.T) {
	preexisting := "PROBLEM: docs/streams/other/brief.md: unrelated pre-existing red"
	introducedOne := "PROBLEM: docs/streams/x/brief.md: backticked path \"../sibling/x\" does not exist — " +
		"for a sibling-repo file, prefix it ../<repo>/../sibling/x"
	stubStatusgenLint(t, []string{preexisting}, []string{preexisting, introducedOne})

	root := t.TempDir()
	introduced, err := lintDiffAt(root, "docs/streams/x/brief.md", []byte("new content\n"))
	if err != nil {
		t.Fatalf("lintDiffAt: %v", err)
	}
	if len(introduced) != 1 || introduced[0] != introducedOne {
		t.Fatalf("introduced = %v, want exactly [%q] (the pre-existing problem must be excluded)", introduced, introducedOne)
	}
}

// TestLintDiffAtCleanTreeIntroducesNothing: identical before/after sets introduce nothing —
// the ordinary "this landing is clean" case, including when statusgen reports problems
// elsewhere in the tree that this landing did not touch.
func TestLintDiffAtCleanTreeIntroducesNothing(t *testing.T) {
	same := []string{"PROBLEM: docs/streams/other/brief.md: unrelated pre-existing red"}
	stubStatusgenLint(t, same, same)

	root := t.TempDir()
	introduced, err := lintDiffAt(root, "docs/streams/x/brief.md", []byte("new content\n"))
	if err != nil {
		t.Fatalf("lintDiffAt: %v", err)
	}
	if len(introduced) != 0 {
		t.Fatalf("introduced = %v, want none", introduced)
	}
}

// TestLintDiffAtRestoresOriginalFile: the local stage is reverted to the ORIGINAL content
// after lintDiffAt returns, on a CLEAN run — the real commit rides the Contents API, never
// this local write, so the landing worktree must come back exactly as it was found.
func TestLintDiffAtRestoresOriginalFile(t *testing.T) {
	stubStatusgenLint(t, nil, nil)

	root := t.TempDir()
	rel := "docs/streams/x/brief.md"
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	if _, err := lintDiffAt(root, rel, []byte("staged content that must not survive\n")); err != nil {
		t.Fatalf("lintDiffAt: %v", err)
	}

	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read after lintDiffAt: %v", err)
	}
	if string(got) != "original\n" {
		t.Fatalf("file after lintDiffAt = %q, want the ORIGINAL content restored, got the staged content leaked out", got)
	}
}

// TestLintDiffAtRestoresOriginalFileOnRefusal: the same restore-on-return guarantee holds
// even when the run refuses (an introduced PROBLEM) — a deferred restore, not a conditional
// one.
func TestLintDiffAtRestoresOriginalFileOnRefusal(t *testing.T) {
	stubStatusgenLint(t, nil, []string{"PROBLEM: docs/streams/x/brief.md: something new and red"})

	root := t.TempDir()
	rel := "docs/streams/x/brief.md"
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte("original\n"), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	introduced, err := lintDiffAt(root, rel, []byte("staged content that must not survive\n"))
	if err != nil {
		t.Fatalf("lintDiffAt: %v", err)
	}
	if len(introduced) != 1 {
		t.Fatalf("introduced = %v, want exactly 1", introduced)
	}

	got, rerr := os.ReadFile(abs)
	if rerr != nil {
		t.Fatalf("read after lintDiffAt: %v", rerr)
	}
	if string(got) != "original\n" {
		t.Fatalf("file after a REFUSED lintDiffAt = %q, want the ORIGINAL content restored", got)
	}
}

// TestLintDiffAtRemovesNewlyCreatedFile: when the target did NOT exist before the stage
// (a brand-new Evidence file), lintDiffAt must remove it again on return — the pre-landing
// state had no file there at all, so "restore" means "delete", not "leave it".
func TestLintDiffAtRemovesNewlyCreatedFile(t *testing.T) {
	stubStatusgenLint(t, nil, nil)

	root := t.TempDir()
	rel := "docs/streams/x/new-brief.md"

	if _, err := lintDiffAt(root, rel, []byte("brand new content\n")); err != nil {
		t.Fatalf("lintDiffAt: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(statErr) {
		t.Fatalf("file at %s still exists after lintDiffAt for a target that did not exist before staging (err=%v)",
			rel, statErr)
	}
}

// TestLintDiffAtBeforeLintErrorPropagatesWithoutStaging: if the FIRST (pre-landing)
// statusgenLintFn call errors, lintDiffAt must return that error without ever touching the
// filesystem — there is nothing to stage if the baseline itself could not be established.
func TestLintDiffAtBeforeLintErrorPropagatesWithoutStaging(t *testing.T) {
	old := statusgenLintFn
	wantErr := deskkit.Unverifiable("statusgen is not on PATH", nil)
	statusgenLintFn = func(string) ([]string, error) { return nil, wantErr }
	t.Cleanup(func() { statusgenLintFn = old })

	root := t.TempDir()
	rel := "docs/streams/x/brief.md"
	_, err := lintDiffAt(root, rel, []byte("content\n"))
	if err != wantErr {
		t.Fatalf("lintDiffAt error = %v, want the before-lint error propagated unchanged", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(statErr) {
		t.Fatal("lintDiffAt staged a file despite the before-lint failing")
	}
}

// --- statusgenLintAt: the one real subprocess call ---

// fakeStatusgenOnPath writes an executable shell script named "statusgen" into a fresh
// directory and prepends it to PATH for the duration of the test — a real subprocess,
// without needing the actual statusgen binary present. Skipped on Windows: the shell-script
// shape does not apply there, and this package's other tests already run cross-platform.
func fakeStatusgenOnPath(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake statusgen does not apply on windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "statusgen")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("write fake statusgen: %v", err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
}

// TestStatusgenLintAtNotOnPathIsUnverifiable: statusgen absent from PATH is could-not-check
// (Unverifiable), never a silent "no problems" pass — C-4 of the three-state instrument
// rule.
func TestStatusgenLintAtNotOnPathIsUnverifiable(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // a PATH with nothing on it
	_, err := statusgenLintAt(t.TempDir())
	if err == nil {
		t.Fatal("statusgenLintAt with no statusgen on PATH returned nil error — want could-not-check")
	}
	var de *deskkit.DeskError
	if !asDeskError(err, &de) || de.Code != deskkit.ExitUnverifiable {
		t.Fatalf("error = %v, want an Unverifiable DeskError (exit %d)", err, deskkit.ExitUnverifiable)
	}
}

// TestStatusgenLintAtParsesProblemLines: a real subprocess exiting nonzero with PROBLEM:
// lines on stdout is the normal "problems found" outcome, not a runner failure — its
// PROBLEM lines come back verbatim and non-PROBLEM output is dropped.
func TestStatusgenLintAtParsesProblemLines(t *testing.T) {
	fakeStatusgenOnPath(t, `echo "statusgen: config echoed here"
echo "PROBLEM: docs/streams/x/brief.md: first problem"
echo "NOTICE: some notice, not a problem"
echo "PROBLEM: docs/streams/y/brief.md: second problem"
exit 1`)

	got, err := statusgenLintAt(t.TempDir())
	if err != nil {
		t.Fatalf("statusgenLintAt: %v", err)
	}
	want := []string{
		"PROBLEM: docs/streams/x/brief.md: first problem",
		"PROBLEM: docs/streams/y/brief.md: second problem",
	}
	if len(got) != len(want) {
		t.Fatalf("problems = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("problems[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestStatusgenLintAtCleanExitNoProblems: exit 0 with no PROBLEM: lines reports clean.
func TestStatusgenLintAtCleanExitNoProblems(t *testing.T) {
	fakeStatusgenOnPath(t, `echo "LINT: PASS"
exit 0`)

	got, err := statusgenLintAt(t.TempDir())
	if err != nil {
		t.Fatalf("statusgenLintAt: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("problems = %v, want none", got)
	}
}

// TestStatusgenLintAtPassesRootAndNeutralCwd: statusgen is invoked with --root <the exact
// dir given> and from a NEUTRAL cwd (os.TempDir()) — never the calling process's own cwd —
// mirroring deskpreflight's runStatusgenLint (cmd/deskpreflight/main.go).
func TestStatusgenLintAtPassesRootAndNeutralCwd(t *testing.T) {
	target := t.TempDir()

	// pwd -P inside the script resolves through any symlink (macOS's /tmp -> /private/tmp),
	// so compare against the same resolution rather than os.TempDir()'s raw string.
	wantCwd, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks(os.TempDir()): %v", err)
	}

	// statusgenLintAt only returns PROBLEM: lines, so capture the script's own report through
	// a temp file instead of trying to read it back from the return value.
	report := filepath.Join(t.TempDir(), "report.txt")
	fakeStatusgenOnPath(t, `echo "ARGS: $@" > `+report+`
echo "CWD: $(pwd -P)" >> `+report+`
exit 0`)

	if _, err := statusgenLintAt(target); err != nil {
		t.Fatalf("statusgenLintAt: %v", err)
	}
	out, rerr := os.ReadFile(report)
	if rerr != nil {
		t.Fatalf("read report: %v", rerr)
	}
	gotWantRoot, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("EvalSymlinks(target): %v", err)
	}
	if !strings.Contains(string(out), "ARGS: --root "+gotWantRoot+" --lint") &&
		!strings.Contains(string(out), "ARGS: --root "+target+" --lint") {
		t.Fatalf("report missing the expected --root/--lint args: %s", out)
	}
	if !strings.Contains(string(out), "CWD: "+wantCwd) {
		t.Fatalf("report shows the wrong cwd, want the neutral os.TempDir(): %s", out)
	}
}

// asDeskError is errors.As without importing "errors" twice across this file's tests —
// small local helper matching how the rest of this package checks typed exit codes.
func asDeskError(err error, target **deskkit.DeskError) bool {
	de, ok := err.(*deskkit.DeskError)
	if !ok {
		return false
	}
	*target = de
	return true
}
