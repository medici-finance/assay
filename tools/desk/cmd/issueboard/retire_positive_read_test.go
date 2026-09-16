package main

// retire_positive_read_test.go — the #1032 regression: RETIRE means "this placeholder's issue
// is now CLOSED", so it must rest on a POSITIVE `state: closed` read for THAT issue number —
// never on the issue's ABSENCE from the open-issue listing. A listing that stopped early (a
// slow/partial page, a rate limit, a timeout) omits live issues, and a board that reads
// absence as closure retires every placeholder the listing left out: ~236 open issues flipped
// NONE→RETIRE between two sweeps on the incident this reproduces.

import (
	"bytes"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// retireRows returns the REPO#NUM cells of every RETIRE row rendered on the board.
func retireRows(board string) []string {
	var out []string
	for _, ln := range strings.Split(board, "\n") {
		f := strings.Fields(ln)
		if len(f) >= 2 && f[0] == actRetire {
			out = append(out, f[1])
		}
	}
	return out
}

// TestPartialListing_NeverRetires reproduces the incident shape: five open issues, each with
// a placeholder. Sweep 1 reads the full listing — five NONE rows. Sweep 2's listing comes back
// PARTIAL (only #10 of the five), while every omitted issue still positively reads OPEN. The
// unfixed board classifies the four omitted placeholders RETIRE off their absence alone; the
// fixed board must render ZERO RETIRE rows and refuse the sweep as could-not-check (exit 6),
// naming the repo and the issue whose open read proved the listing partial.
func TestPartialListing_NeverRetires(t *testing.T) {
	root := t.TempDir()
	full := []deskkit.IssueSummary{}
	for n := 10; n <= 14; n++ {
		full = append(full, deskkit.IssueSummary{Number: n, Title: "live issue " + strconv.Itoa(n), Author: deskkit.Account{Login: "shared-agent"}})
		writeFile(t, filepath.Join(root, issueLoopDir, "issue-"+strconv.Itoa(n)+".md"), placeholderFixture(homeRepo, "todo", ""))
	}

	// Sweep 1: the full listing — every placeholder's issue is open → NONE, nothing to retire.
	installForge(t, map[string]*repoFixture{homeRepo: {issues: full}})
	var out1, err1 bytes.Buffer
	if code := run([]string{"--root", root, "issues"}, &out1, &err1); code != 0 {
		t.Fatalf("sweep 1 (full listing) = exit %d, want 0; stderr=%s", code, err1.String())
	}
	if got := retireRows(out1.String()); len(got) != 0 {
		t.Fatalf("sweep 1 (full listing) rendered RETIRE rows %v for open issues:\n%s", got, out1.String())
	}
	if strings.Count(out1.String(), actNone) != 5 {
		t.Fatalf("sweep 1 (full listing) should render 5 NONE rows; got:\n%s", out1.String())
	}

	// Sweep 2: the SAME issues, but the listing stopped early — only #10 came back. Every
	// omitted issue still reads OPEN on a direct per-issue read.
	partial := map[string]*repoFixture{homeRepo: {
		issues:       full[:1],
		openUnlisted: map[int]string{11: "live issue 11", 12: "live issue 12", 13: "live issue 13", 14: "live issue 14"},
	}}
	calls := installForge(t, partial)
	var out2, err2 bytes.Buffer
	code := run([]string{"--root", root, "issues"}, &out2, &err2)

	if got := retireRows(out2.String()); len(got) != 0 {
		t.Errorf("#1032: a PARTIAL listing flipped %d still-open placeholders NONE→RETIRE %v — RETIRE must rest on a positive closed read, never on absence from the listing:\n%s", len(got), got, out2.String())
	}
	if code != 6 {
		t.Errorf("a partial listing must fail closed as could-not-check (exit 6), got exit %d; stdout:\n%s\nstderr: %s", code, out2.String(), err2.String())
	}
	if out2.Len() != 0 {
		t.Errorf("a refused sweep must emit no (partial) board on stdout; got:\n%s", out2.String())
	}
	if msg := err2.String(); !strings.Contains(msg, homeRepo) || !strings.Contains(msg, "partial") {
		t.Errorf("the refusal must name the repo %q and say the listing is partial; got: %s", homeRepo, msg)
	}
	sawPositiveRead := false
	for _, c := range *calls {
		if c.op == "GetIssue" && c.repo == homeRepo && c.num >= 11 && c.num <= 14 {
			sawPositiveRead = true
		}
	}
	if !sawPositiveRead {
		t.Errorf("the board must read the state of an issue absent from the listing before deciding anything about it; GetIssue was never called for #11..#14 (ops: %+v)", *calls)
	}
}

// TestUnreadableIssueState_NeverRetires: a placeholder's issue is absent from the listing AND
// its direct read FAILS (rate limit / 5xx / timeout). Its state could not be positively read,
// so it keeps its prior (un-retired) state and the sweep is could-not-check — never RETIRE.
func TestUnreadableIssueState_NeverRetires(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, issueLoopDir, "issue-20.md"), placeholderFixture(homeRepo, "todo", ""))
	installForge(t, map[string]*repoFixture{homeRepo: {
		issues: []deskkit.IssueSummary{{Number: 1, Title: "unrelated open", Author: deskkit.Account{Login: "shared-agent"}}},
		getErr: map[int]error{20: &deskkit.ForgeAPIError{Status: 503, Method: "GET", Path: "/repos/example-org/tracker/issues/20"}},
	}})
	var out, errb bytes.Buffer
	code := run([]string{"--root", root, "issues"}, &out, &errb)
	if got := retireRows(out.String()); len(got) != 0 {
		t.Errorf("#1032: an issue whose state could not be read was classified RETIRE %v:\n%s", got, out.String())
	}
	if code != 6 {
		t.Errorf("an unreadable issue state must be could-not-check (exit 6), got %d; stderr: %s", code, errb.String())
	}
	if msg := errb.String(); !strings.Contains(msg, homeRepo+"#20") {
		t.Errorf("the refusal must name the unreadable issue %s#20; got: %s", homeRepo, msg)
	}
	if out.Len() != 0 {
		t.Errorf("a refused sweep must emit no (partial) board on stdout; got:\n%s", out.String())
	}
}

// TestPositiveClosedRead_StillRetires pins the unchanged semantics: a placeholder whose
// issue positively reads CLOSED is RETIRE, with the title from that same read, exit 0.
func TestPositiveClosedRead_StillRetires(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, issueLoopDir, "issue-3.md"), placeholderFixture(homeRepo, "todo", ""))
	calls := installForge(t, map[string]*repoFixture{homeRepo: {
		issues: []deskkit.IssueSummary{{Number: 1, Title: "open one", Author: deskkit.Account{Login: "shared-agent"}}},
		titles: map[int]string{3: "genuinely closed"},
	}})
	var out, errb bytes.Buffer
	if code := run([]string{"--root", root, "issues"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d, want 0; stderr=%s", code, errb.String())
	}
	if got := retireRows(out.String()); len(got) != 1 || got[0] != "tracker#3" {
		t.Errorf("a positively-closed placeholder issue must be the ONE RETIRE row; got %v:\n%s", got, out.String())
	}
	if !strings.Contains(out.String(), "genuinely closed") {
		t.Errorf("the RETIRE row must carry the title from the positive read; got:\n%s", out.String())
	}
	reads := 0
	for _, c := range *calls {
		if c.op == "GetIssue" && c.num == 3 {
			reads++
		}
	}
	if reads != 1 {
		t.Errorf("RETIRE must rest on exactly ONE positive read of #3; got %d GetIssue reads", reads)
	}
}

// TestDonePlaceholder_NoStateRead: an already-retired placeholder (status: done) classifies
// NONE whatever its issue's state, so the board spends no per-issue read on it — and a
// forge that cannot answer for it cannot fail the sweep.
func TestDonePlaceholder_NoStateRead(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, issueLoopDir, "issue-30.md"), placeholderFixture(homeRepo, "done", ""))
	calls := installForge(t, map[string]*repoFixture{homeRepo: {
		getErr: map[int]error{30: errors.New("must not be read")},
	}})
	var out, errb bytes.Buffer
	if code := run([]string{"--root", root, "issues"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d, want 0; stderr=%s", code, errb.String())
	}
	if got := retireRows(out.String()); len(got) != 0 {
		t.Errorf("a done placeholder is never RETIRE; got %v", got)
	}
	if !strings.Contains(out.String(), actNone+" ") || !strings.Contains(out.String(), "tracker#30") {
		t.Errorf("a done placeholder must render as a NONE row; got:\n%s", out.String())
	}
	for _, c := range *calls {
		if c.op == "GetIssue" && c.num == 30 {
			t.Errorf("no per-issue read should be spent on an already-retired placeholder; ops: %+v", *calls)
		}
	}
}
