package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// trace_test.go — the swallowed-stderr regression, pinned per step.
//
// THE DEFECT, stated once. Every desk tool opens its stderr with the effective-config echo
// (`assay-config: …`). A step report built with firstLine(r.stderr) therefore prints the
// CONFIG ECHO and never the tool's own message, so the operator reads
//
//	step claim-acquire: the claim on <key> could not be established (assay-config: …)
//
// and has to re-run the child by hand to find out what actually happened. Every case below
// plants the real two-line shape — echo first, diagnosis second — and asserts that the
// diagnosis reaches the message. A single-line stub could not tell the two apart, which is
// why the shared harness emits both.

// realShape is the stderr a desk tool actually produces: preamble, then its own message.
func realShape(diagnosis string) string {
	return "assay-config: roster=/x/roster.env allowed=3 repos\n" +
		"desk-tools WARNING: running UNPINNED build\n" +
		diagnosis
}

// captureStderr runs fn with os.Stderr redirected and returns what was written. It is the
// end-to-end read of the shared exit path: what an operator's terminal would show.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		buf := make([]byte, 4096)
		for {
			n, rerr := r.Read(buf)
			if n > 0 {
				b.Write(buf[:n])
			}
			if rerr != nil {
				break
			}
		}
		done <- b.String()
	}()
	fn()
	os.Stderr = old
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// TestClaimAcquireFailureNamesTheClaimToolsOwnMessage — the exit-6 path the driver observed:
// "the claim on <key> could not be established" with nothing but the config echo after it.
func TestClaimAcquireFailureNamesTheClaimToolsOwnMessage(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "dispatch-claim.sh acquire", code: deskkit.ExitUnverifiable,
			stderr: realShape("refs/dispatch: could not read the claim ref (network unreachable)")},
	}

	err := cmdDispatch([]string{"example-stream--07", "--root", root, "--kit", "worker",
		"--tier", "strong", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil {
		t.Fatal("a failed claim acquire must not dispatch")
	}
	msg := err.Error()
	if !strings.Contains(msg, "could not read the claim ref (network unreachable)") {
		t.Errorf("the claim tool's own message was swallowed; operator sees only:\n%s", msg)
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Errorf("exit code changed: got %d want %d", deskkit.ExitCodeOf(err), deskkit.ExitUnverifiable)
	}
	// The child's full stderr and command line must be carried for DESK_TRACE, not just the
	// one line that made it into the message.
	var de *deskkit.DeskError
	if !asDeskError(err, &de) {
		t.Fatalf("not a *DeskError: %T", err)
	}
}

// TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine — the step the driver named
// first. The message already forwarded deskwt's words; what it never carried was the command
// line and the child's exit status a trace needs.
func TestWorktreeCreateFailureCarriesDeskwtStderrAndTheCommandLine(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "deskwt add", code: deskkit.ExitRefused,
			stderr: realShape("deskwt: refused: branch wd/example-stream--07 is checked out in worktree /private/tmp/tracker-other")},
	}

	err := cmdDispatch([]string{"example-stream--07", "--root", root, "--kit", "worker",
		"--tier", "strong", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil {
		t.Fatal("a failed worktree create must not dispatch")
	}
	if !strings.Contains(err.Error(), "checked out in worktree /private/tmp/tracker-other") {
		t.Errorf("deskwt's own message did not reach the operator:\n%s", err.Error())
	}
	if !deskkit.IsRefused(err) {
		t.Errorf("deskwt's refusal (exit 5) was flattened; got exit %d", deskkit.ExitCodeOf(err))
	}

	// Under DESK_TRACE the exact `deskwt add` argv and the child's exit status must appear.
	deskkit.SetTrace(true)
	defer deskkit.ResetTrace()
	out := captureStderr(t, func() { deskkit.ReportError(os.Stderr, err) })
	for _, want := range []string{
		"desk-trace: failing command:",
		"deskwt add",
		"child exit status: 5",
		"checked out in worktree /private/tmp/tracker-other",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("DESK_TRACE output missing %q; got:\n%s", want, out)
		}
	}
}

// TestResolveRepoFailureNamesGitsOwnMessage — the origin-URL read, another firstLine site.
func TestResolveRepoFailureNamesGitsOwnMessage(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", code: 128,
			stderr: realShape("error: No such remote 'origin'")},
	}
	err := cmdDispatch([]string{"example-stream--07", "--root", root, "--kit", "worker",
		"--tier", "strong", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil {
		t.Fatal("an unresolvable repo must not dispatch")
	}
	if !strings.Contains(err.Error(), "No such remote 'origin'") {
		t.Errorf("git's own message was swallowed:\n%s", err.Error())
	}
}

// TestGitOutFailureCarriesGitStderr — worktree.go's gitOut discarded the child's stderr
// ENTIRELY (it returned the bare *exec.ExitError, i.e. "exit status 128"), which is the
// hardest shape to bisect: there is nothing in the message to search for.
func TestGitOutFailureCarriesGitStderr(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	s.replies = []reply{
		{match: "rev-parse", code: 128, stderr: "fatal: not a git repository (or any parent up to /)"},
	}
	_, err := gitOut(root, "rev-parse", "--show-toplevel")
	if err == nil {
		t.Fatal("expected the stubbed failure")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("gitOut dropped git's stderr entirely; message is only:\n%s", err.Error())
	}
}

// TestTraceIsOffByDefaultAndOutputIsUnchanged — the no-regression floor at the verb level.
func TestTraceIsOffByDefaultAndOutputIsUnchanged(t *testing.T) {
	deskkit.ResetTrace()
	t.Setenv("DESK_TRACE", "")
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "dispatch-claim.sh acquire", code: deskkit.ExitUnverifiable,
			stderr: realShape("refs/dispatch: unreadable")},
	}
	err := cmdDispatch([]string{"example-stream--07", "--root", root, "--kit", "worker",
		"--tier", "strong", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if err == nil {
		t.Fatal("expected a failure")
	}
	out := captureStderr(t, func() { deskkit.ReportError(os.Stderr, err) })
	if out != err.Error()+"\n" {
		t.Errorf("with DESK_TRACE unset the exit line is not byte-identical to err.Error():\n got %q\nwant %q",
			out, err.Error()+"\n")
	}
	if strings.Contains(out, "desk-trace:") {
		t.Errorf("a trace block appeared with the switch off:\n%s", out)
	}
}

// asDeskError is errors.As specialised, kept local so the test file's intent reads without
// an import that exists for one call.
func asDeskError(err error, target **deskkit.DeskError) bool {
	return errors.As(err, target)
}
