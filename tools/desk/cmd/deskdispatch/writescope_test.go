package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// writeBriefFile writes a brief with the given Context files block under
// <root>/docs/streams/<stream>/brief-<num>-x.md and returns its path.
func writeBriefFile(t *testing.T, root, stream, num, filesBlock string) string {
	t.Helper()
	dir := filepath.Join(root, "docs", "streams", stream)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "brief-"+num+"-x.md")
	body := "---\nbrief: " + stream + "/" + num + "\ngate: model\n---\n\n## Context\n\n" + filesBlock + "\n## Task\ndo\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func gitFixture(t *testing.T, root string) {
	t.Helper()
	run := func(args ...string) {
		full := append([]string{"-C", root}, args...)
		cmd := exec.Command("git", full...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")
	run("-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init")
}

// TestEchoWriteOverlap_WarnsAdvisory proves the deskdispatch echo emits a WRITE-OVERLAP line
// when the item's --brief scopes overlap an in-flight refs/dispatch claim's scopes, and nothing
// when they are disjoint — the advisory hint, offline (local refs only).
func TestEchoWriteOverlap_WarnsAdvisory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	candBrief := writeBriefFile(t, root, "alpha", "01", "files:\n- `internal/loopengine/engine.go`\n")
	writeBriefFile(t, root, "beta", "02", "files:\n- `internal/loopengine/`\n")
	gitFixture(t, root)
	gitRun(t, root, "update-ref", "refs/dispatch/repo--beta--02", "HEAD")

	var buf bytes.Buffer
	echoWriteOverlap(&buf, dispatchOpts{item: "alpha/01", brief: candBrief, root: root})
	want := "WRITE-OVERLAP: alpha/01 ~ beta/02 on internal/loopengine/"
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("expected overlap echo %q, got:\n%s", want, buf.String())
	}

	// Disjoint --brief: no echo.
	disjoint := writeBriefFile(t, root, "gamma", "03", "files:\n- `docs/site/`\n")
	var buf2 bytes.Buffer
	echoWriteOverlap(&buf2, dispatchOpts{item: "gamma/03", brief: disjoint, root: root})
	if strings.Contains(buf2.String(), "WRITE-OVERLAP") {
		t.Fatalf("disjoint item must not echo an overlap:\n%s", buf2.String())
	}

	// A missing/underivable --brief is silent at dispatch time (no spurious could-not-derive).
	var buf3 bytes.Buffer
	echoWriteOverlap(&buf3, dispatchOpts{item: "x/09", brief: "", root: root})
	if strings.TrimSpace(buf3.String()) != "" {
		t.Fatalf("no --brief must echo nothing, got:\n%s", buf3.String())
	}
}

func gitRun(t *testing.T, root string, args ...string) {
	t.Helper()
	full := append([]string{"-C", root}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

// TestDispatch_OverlapWarnsAndProceeds is Verify row 5: a dispatch of the overlapping fixture
// item completes the claim (exit 0) — the overlap is advisory, warn-and-PROCEED, never a block.
func TestDispatch_OverlapWarnsAndProceeds(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	s := &stub{}
	home := t.TempDir()
	t.Setenv("HOME", home)
	plantFixtureRoster(t, home)
	t.Setenv("DESK_TOOLS_DISABLED", "")
	t.Setenv("DESK_SESSION", "deskdispatch-test")
	t.Setenv("CLAUDE_SESSION_ID", "deskdispatch-test")

	// root is a REAL git repo (InFlightClaimScopes reads its local refs directly), carrying the
	// consumer scripts, the candidate brief, and an in-flight refs/dispatch claim it overlaps.
	root := t.TempDir()
	plantScripts(t, root)
	candBrief := writeBriefFile(t, root, "alpha", "01", "files:\n- `internal/loopengine/engine.go`\n")
	writeBriefFile(t, root, "beta", "02", "files:\n- `internal/loopengine/`\n")
	gitFixture(t, root)
	gitRun(t, root, "update-ref", "refs/dispatch/repo--beta--02", "HEAD")

	// Mock the child processes: claim acquire succeeds (exit 0), worktree create returns a path.
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		joined := name + " " + strings.Join(args, " ")
		s.calls = append(s.calls, append([]string{name}, args...))
		if strings.Contains(joined, "deskwt add") {
			return exec.Command("/bin/sh", "-c", "echo /private/tmp/worker-home")
		}
		return exec.Command("/bin/sh", "-c", "exit 0") // claim acquire, roster, etc.
	}
	t.Cleanup(func() { execCommand = old })

	// Capture stderr so the advisory warning is observable.
	rOut, wOut, _ := os.Pipe()
	oldErr := os.Stderr
	os.Stderr = wOut

	promptFile := filepath.Join(t.TempDir(), "prompt.md")
	rc := run([]string{"alpha/01", "--repo", allowedRepo, "--root", root,
		"--brief", candBrief, "--prompt-file", promptFile, "--quiet"})

	wOut.Close()
	os.Stderr = oldErr
	var errBuf bytes.Buffer
	_, _ = errBuf.ReadFrom(rOut)

	if rc != deskkit.ExitOK {
		t.Fatalf("dispatch over an overlap must PROCEED (advisory), rc=%d want 0\nstderr:\n%s", rc, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "WRITE-OVERLAP: alpha/01 ~ beta/02 on internal/loopengine/") {
		t.Fatalf("advisory overlap warning missing from stderr:\n%s", errBuf.String())
	}
	if !s.ran("dispatch-claim.sh acquire") {
		t.Fatalf("the claim was not attempted — the echo must PRECEDE and not replace the claim; calls=%v", s.calls)
	}
}
