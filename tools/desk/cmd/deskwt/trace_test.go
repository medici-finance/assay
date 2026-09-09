package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// trace_test.go — deskwt's own failures carry git's words, its command line and its status.
//
// deskwt is the tool whose message an operator reads SECOND-hand more often than any other:
// deskdispatch shells out to it for every dispatch and forwards whatever it said. A diagnosis
// lost here is lost twice — once for whoever ran deskwt, and again for whoever ran the
// dispatch that wrapped it. What was missing was never git's stderr text (runGitStreams
// already appended it in parentheses) but the STRUCTURED detail a trace needs: the argv as
// executed and the child's exit status, neither of which survived onto the error, so
// DESK_TRACE would have had nothing to print for a deskwt failure at all.

// TestRunGitFailureCarriesStderrCommandAndExitStatus is the unit-level pin. It uses a real
// git process against a real fixture repo — the failure is git's own, not a synthetic one.
func TestRunGitFailureCarriesStderrCommandAndExitStatus(t *testing.T) {
	deskkit.ResetTrace()
	work := newRepo(t)
	withEnv(t, work)

	_, _, err := runGitStreams(work, "rev-parse", "--verify", "refs/heads/definitely-not-a-branch")
	if err == nil {
		t.Fatal("expected git to refuse an unknown ref")
	}

	var de *deskkit.DeskError
	if !errors.As(err, &de) {
		t.Fatalf("runGitStreams no longer produces a *DeskError: %T — the subprocess detail a "+
			"trace prints (command line, exit status, full stderr) has nowhere to live", err)
	}
	if de.ExitStatus == 0 {
		t.Errorf("git's exit status was not carried onto the error: %d", de.ExitStatus)
	}
	if !strings.Contains(de.Cmd, "rev-parse --verify refs/heads/definitely-not-a-branch") {
		t.Errorf("the command line as executed was not carried onto the error: %q", de.Cmd)
	}
	if !strings.Contains(err.Error(), "rev-parse") {
		t.Errorf("the message does not name the failing git verb:\n%s", err.Error())
	}
}

// TestWorktreeAddCollisionReachesTheOperatorWithGitsWords is the verb-level pin, on the
// commonest real deskwt failure there is: a branch of the target name already exists and
// carries unpushed work, so the reclaim declines and `add` refuses. The operator must be
// told WHICH branch and WHY, in one read.
func TestWorktreeAddCollisionReachesTheOperatorWithGitsWords(t *testing.T) {
	deskkit.ResetTrace()
	work := newRepo(t)
	withEnv(t, work)
	const br = "trace-collision"
	branchAhead(t, work, br)

	err := cmdAdd([]string{br, "--base", "refs/remotes/origin/main"})
	if err == nil {
		t.Fatal("add must not succeed onto a branch carrying unpushed work")
	}
	if !deskkit.IsRefused(err) {
		t.Errorf("a branch carrying unpushed work is a REFUSAL (5), got exit %d: %v",
			deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), br) {
		t.Errorf("the failing branch is not named in the message:\n%s", err.Error())
	}
}

// TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand pins both directions of the switch
// on a real deskwt failure.
func TestDeskwtTraceOffIsByteIdenticalAndOnCarriesTheCommand(t *testing.T) {
	deskkit.ResetTrace()
	t.Setenv("DESK_TRACE", "")
	work := newRepo(t)
	withEnv(t, work)

	_, _, err := runGitStreams(work, "rev-parse", "--verify", "refs/heads/definitely-not-a-branch")
	if err == nil {
		t.Fatal("expected git to refuse an unknown ref")
	}

	var off strings.Builder
	deskkit.ReportError(&off, err)
	if off.String() != err.Error()+"\n" {
		t.Errorf("trace-off output is not byte-identical to err.Error():\n got %q\nwant %q",
			off.String(), err.Error()+"\n")
	}
	if strings.Contains(off.String(), "desk-trace:") {
		t.Errorf("a trace block appeared with the switch off:\n%s", off.String())
	}

	deskkit.SetTrace(true)
	defer deskkit.ResetTrace()
	var on strings.Builder
	deskkit.ReportError(&on, err)
	for _, want := range []string{
		"desk-trace: failing command:",
		"rev-parse --verify refs/heads/definitely-not-a-branch",
		"desk-trace: child exit status:",
	} {
		if !strings.Contains(on.String(), want) {
			t.Errorf("DESK_TRACE output missing %q; got:\n%s", want, on.String())
		}
	}
}
