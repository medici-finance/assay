package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// gitRepo builds a throwaway repo with a base commit and a scan commit on top, so the
// derivation runs against a REAL `git diff`, not a fixture string. The fixture-string
// cases live in deskkit; this file's job is the git plumbing and the exit codes.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("init", "-q", "-b", "main")
	write(deskkit.ScanDir+"/README.md", "# issue-loop\n")
	write(deskkit.ScanDir+"/issue-800-old.md", "---\nstatus: todo\n---\n")
	run("add", "-A")
	run("commit", "-qm", "base")
	run("branch", "scan-base")

	// The scan commit: two created, one retired by a status flip.
	write(deskkit.ScanDir+"/issue-901-new.md", "---\nstatus: todo\n---\n")
	write(deskkit.ScanDir+"/issue-902-new.md", "---\nstatus: todo\n---\n")
	write(deskkit.ScanDir+"/issue-800-old.md", "---\nstatus: done\n---\n")
	write(deskkit.ScanDir+"/README.md", "# issue-loop\nboard row edited\n")
	run("add", "-A")
	run("commit", "-qm", "scan")
	return dir
}

func inDir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func TestEmitDerivesFromTheRealDiff(t *testing.T) {
	inDir(t, gitRepo(t))

	out := captureStdout(t, func() {
		if err := cmdEmit([]string{"--base", "scan-base", "--date", "2026-08-13", "--format", "title"}, os.Stdout); err != nil {
			t.Fatalf("emit: %v", err)
		}
	})
	want := "chore(issue-loop): scan 2026-08-13 — 2 created, 1 retired\n"
	if out != want {
		t.Fatalf("emit --format title = %q, want %q", out, want)
	}

	body := captureStdout(t, func() {
		if err := cmdEmit([]string{"--base", "scan-base", "--date", "2026-08-13", "--format", "body"}, os.Stdout); err != nil {
			t.Fatalf("emit: %v", err)
		}
	})
	for _, w := range []string{deskkit.ScanBodyMarker, "**created:** 2", "**retired:** 1",
		"issue-901-new.md", "issue-800-old.md", "#685"} {
		if !strings.Contains(body, w) {
			t.Errorf("emitted body missing %q:\n%s", w, body)
		}
	}
	if strings.Contains(body, "README.md") {
		t.Error("the stream README is the board, not a placeholder — it must never be counted")
	}
}

// TestRenameIntoDoneIsCountedAsRetired is brief 03's named rename-detection parity
// golden for deskscanbody: `derive` now takes its diff from gitcore.Repo.Diff (go-git,
// in-process) rather than shelling out to `git diff -M`, and go-git's tree.Diff alone
// does NOT detect renames — only gitcore's own rename-detection options (mirrored from
// DiffNames) make a `git mv` into <dir>/done/ surface as a "rename to" line rather than
// an unrelated delete+add pair. ParseScanDiff's retired-by-archival path
// (isRename && scanArchived(curNew) && !scanArchived(curOld)) only fires on that "rename
// to" line, so this is genuinely fail-capable: drop rename detection from the diff this
// tool consumes and the retired count silently goes to zero instead of counting the
// archival.
func TestRenameIntoDoneIsCountedAsRetired(t *testing.T) {
	dir := gitRepo(t)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid",
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// A body long enough that the rename is detected by similarity, not only by exact-
	// content match — the same discipline gitcore's own rename golden uses, so this
	// exercises the real detector rather than an exact-match shortcut.
	body := "alpha\nbravo\ncharlie\ndelta\necho\nfoxtrot\ngolf\nhotel\nstatus: todo\n"
	if err := os.WriteFile(filepath.Join(dir, deskkit.ScanDir, "issue-950-movable.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-qm", "add the file that will be archived")
	run("branch", "-f", "scan-base")

	if err := os.MkdirAll(filepath.Join(dir, deskkit.ScanDir, "done"), 0o755); err != nil {
		t.Fatal(err)
	}
	run("mv", deskkit.ScanDir+"/issue-950-movable.md", deskkit.ScanDir+"/done/issue-950-movable.md")
	run("commit", "-qm", "archive the file into done/")

	inDir(t, dir)
	wantOut, err := exec.Command("git", "-C", dir, "diff", "--name-only", "-M", "scan-base", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wantOut), "done/issue-950-movable.md") ||
		strings.Count(strings.TrimSpace(string(wantOut)), "\n")+1 != 1 {
		t.Fatalf("COULD-NOT-CHECK: git itself did not report this fixture as a single detected rename: %q", wantOut)
	}

	out := captureStdout(t, func() {
		if err := cmdEmit([]string{"--base", "scan-base", "--date", "2026-08-13", "--format", "title"}, os.Stdout); err != nil {
			t.Fatalf("emit: %v", err)
		}
	})
	want := "chore(issue-loop): scan 2026-08-13 — 0 created, 1 retired\n"
	if out != want {
		t.Fatalf("emit --format title = %q, want %q (the archival-by-rename must be counted as 1 retired)", out, want)
	}
}

// TestCheckCanFail is the required proof-it-can-fail, run end to end through the verb:
// the positive control is #627's own stale title over this branch's real diff.
func TestCheckCanFail(t *testing.T) {
	dir := gitRepo(t)
	inDir(t, dir)

	stale := filepath.Join(t.TempDir(), "stale.md")
	if err := os.WriteFile(stale, []byte("chore(issue-loop): scan 2026-08-11 — 29 created, 29 retired\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := cmdCheck([]string{"--base", "scan-base", "--text-file", stale}, os.Stdout)
	if err == nil {
		t.Fatal("a stale count must FAIL check — a gate that cannot fire is not a gate")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want exit 5 refused, got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "29 created") || !strings.Contains(err.Error(), "2 created") {
		t.Errorf("the refusal must state both the asserted and the derived counts; got: %v", err)
	}
}

func TestCheckPassesOnTheEmittedTitle(t *testing.T) {
	inDir(t, gitRepo(t))

	title := captureStdout(t, func() {
		if err := cmdEmit([]string{"--base", "scan-base", "--date", "2026-08-13", "--format", "title"}, os.Stdout); err != nil {
			t.Fatal(err)
		}
	})
	p := filepath.Join(t.TempDir(), "title.md")
	if err := os.WriteFile(p, []byte(title), 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := cmdCheck([]string{"--base", "scan-base", "--text-file", p}, os.Stdout); err != nil {
			t.Fatalf("the emitted title must pass its own gate: %v", err)
		}
	})
	if !strings.Contains(out, "checked-clean") {
		t.Errorf("a passing check reports checked-clean; got %q", out)
	}
}

// TestGitFailureIsCouldNotCheck — an unresolvable base must never derive 0/0 and call it
// clean. Zero created / zero retired is a real answer only when git actually answered.
func TestGitFailureIsCouldNotCheck(t *testing.T) {
	inDir(t, gitRepo(t))
	err := cmdEmit([]string{"--base", "no-such-ref", "--format", "title"}, os.Stdout)
	if err == nil {
		t.Fatal("an unresolvable base must fail, not emit 0 created / 0 retired")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("want exit 6 unverifiable, got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "could-not-check") {
		t.Errorf("the failure must be spelled could-not-check; got: %v", err)
	}
}

func TestCheckUnreadableTextFileIsCouldNotCheck(t *testing.T) {
	inDir(t, gitRepo(t))
	err := cmdCheck([]string{"--base", "scan-base", "--text-file", filepath.Join(t.TempDir(), "absent.md")}, os.Stdout)
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("a gate that cannot read its input must not pass it; got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
}

func TestRunRefusesUnknownSubcommandAndNoArgs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DESK_TOOLS_DISABLED", "")
	if got := run([]string{"emitt"}); got != deskkit.ExitRefused {
		t.Errorf("unknown subcommand must refuse (5), got %d", got)
	}
	if got := run([]string{}); got != deskkit.ExitRefused {
		t.Errorf("no args must refuse (5), got %d", got)
	}
	if got := run([]string{"--help"}); got != deskkit.ExitOK {
		t.Errorf("--help must exit 0, got %d", got)
	}
}

func TestEmitRefusesUnknownFormat(t *testing.T) {
	inDir(t, gitRepo(t))
	err := cmdEmit([]string{"--base", "scan-base", "--format", "yaml"}, os.Stdout)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("want exit 5, got %d (%v)", deskkit.ExitCodeOf(err), err)
	}
}

// captureStdout swaps os.Stdout for a pipe while fn runs. The verbs take an *os.File so
// production writes straight to the real stdout with no buffering layer between the
// emitter and the pipe the scan flow reads.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		done <- sb.String()
	}()
	fn()
	w.Close()
	os.Stdout = old
	return <-done
}
