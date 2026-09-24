package main

import (
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// withRepresentedPRs wires a recorded PR-list transport for one test and restores the nil default
// (offline reference build — no forge read) after. The stream slugs below are synthetic
// (example-*): a shipped test file naming a real withheld stream would publish a map to it.
func withRepresentedPRs(t *testing.T, fn func(repo string) ([]deskkit.PRRef, error)) {
	t.Helper()
	old := listRepresentedPRs
	listRepresentedPRs = fn
	t.Cleanup(func() { listRepresentedPRs = old })
}

func TestBriefIDFromItem(t *testing.T) {
	cases := []struct{ in, want string }{
		{"example-a/00", "example-a/00"},                      // slash plan key
		{"assay--example-a--00", "example-a/00"},              // claim key, <repo>--<stream>--<NN>
		{"at--example-two-part--08", "example-two-part/08"},   // stream carrying a single dash survives
		{"medici-finance/assay:example-a/00", "example-a/00"}, // repo-qualified plan key
		{"item-1", ""},      // a bare item names no brief
		{"", ""},            // empty
		{"just-a-name", ""}, // no slash, no --
	}
	for _, c := range cases {
		if got := briefIDFromItem(c.in); got != c.want {
			t.Errorf("briefIDFromItem(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestPhantomCheckRefusesItemWhosePRExistsUnderANonMatchingBranch is the fail-first proof of the
// deskdispatch half of the fix: the live PR sits on a branch this verb's derived name would never
// match, so a branch-name existence check misses it. Keyed on the PR's `Brief:` trailer instead, the
// phantom is caught and the dispatch is refused before the claim.
func TestPhantomCheckRefusesItemWhosePRExistsUnderANonMatchingBranch(t *testing.T) {
	withRepresentedPRs(t, func(repo string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{
			{Number: 373, State: "OPEN", Body: "does the work\n\nBrief: example-a/00"},
		}, nil
	})
	_, err := phantomCheck(dispatchOpts{item: "assay--example-a--00", kit: "worker", pr: 0}, allowedRepo)
	if err == nil {
		t.Fatal("phantomCheck accepted an item whose PR exists under a non-matching branch name")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("phantom is a REFUSAL (exit 5), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "#373") || !strings.Contains(err.Error(), "Brief: example-a/00") {
		t.Errorf("the refusal must name the PR and the trailer it matched on: %v", err)
	}
}

// A MERGED PR represents its brief too — the ~1-in-6-already-merged reality an open-only check leaks.
func TestPhantomCheckExcludesOnAMergedPR(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 372, State: "MERGED", Body: "Brief: example-b/08"}}, nil
	})
	if _, err := phantomCheck(dispatchOpts{item: "assay--example-b--08", kit: "worker"}, allowedRepo); err == nil {
		t.Fatal("a MERGED PR must exclude its brief — the row is done, not dispatchable")
	}
}

// A CLOSED-unmerged PR does NOT represent its brief: the work was abandoned, so the row is
// dispatchable again — no refusal, and so no "resume"/"delivered" wording at all. This is the
// CLOSED half of assay#1654's wording matrix: proving the closed-unmerged case stays silent is
// what makes the open/merged wording split below exhaustive.
func TestPhantomCheckIgnoresAClosedUnmergedPR(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 401, State: "CLOSED", Body: "Brief: example-a/00"}}, nil
	})
	if _, err := phantomCheck(dispatchOpts{item: "assay--example-a--00", kit: "worker"}, allowedRepo); err != nil {
		t.Fatalf("a CLOSED-unmerged PR must NOT block a fresh dispatch (the work was abandoned): %v", err)
	}
}

// TestPhantomCheckOpenPRMessageKeepsResumeWording is the OPEN half of assay#1654's wording
// matrix: an OPEN matched PR keeps today's "resume it" advice unchanged — only the MERGED
// message's wording changes.
func TestPhantomCheckOpenPRMessageKeepsResumeWording(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 373, State: "OPEN", Body: "Brief: example-a/00"}}, nil
	})
	_, err := phantomCheck(dispatchOpts{item: "assay--example-a--00", kit: "worker"}, allowedRepo)
	if err == nil {
		t.Fatal("an OPEN representing PR must refuse a fresh dispatch")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Resume the PR (--pr 373)") {
		t.Errorf("an OPEN match must keep the resume-it wording: %v", err)
	}
	if strings.Contains(msg, "DELIVERED") || strings.Contains(msg, "issue's key") {
		t.Errorf("an OPEN match must not carry the merged/delivered wording: %v", err)
	}
}

// TestPhantomCheckMergedPRMessageSaysDeliveredWithIssueKey is assay#1654's fail-first proof for
// the MERGED half: "Resume the PR (--pr N)" is wrong advice for a merged PR — there is nothing to
// resume. The refusal must say the brief is already delivered by the merged PR, and that any
// follow-up fix is dispatched under a fresh ISSUE key (`<repo>--issue-<N>`, trailer `Issue:
// #<N>`), never re-dispatched under the brief key. The refusal itself (exit 5, still refused) is
// unchanged — only the wording.
func TestPhantomCheckMergedPRMessageSaysDeliveredWithIssueKey(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 1334, State: "MERGED", Body: "Brief: windows-port/00"}}, nil
	})
	_, err := phantomCheck(dispatchOpts{item: "assay--windows-port--00", kit: "worker"}, allowedRepo)
	if err == nil {
		t.Fatal("a MERGED representing PR must still refuse a fresh dispatch — behaviour is unchanged")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("a MERGED phantom is a REFUSAL (exit 5), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	msg := err.Error()
	if strings.Contains(msg, "Resume the PR") {
		t.Errorf("a MERGED PR cannot be resumed — the message must not say so: %v", err)
	}
	if !strings.Contains(msg, "DELIVERED by "+allowedRepo+"#1334 (MERGED)") {
		t.Errorf("the message must say the brief is already delivered by the merged PR: %v", err)
	}
	if !strings.Contains(msg, "assay--issue-<N>") || !strings.Contains(msg, "Issue: #<N>") {
		t.Errorf("the message must point a follow-up fix at a fresh issue key, not the brief key: %v", err)
	}
}

// A PR list the transport could not read is UNVERIFIABLE, never rounded to "no PR exists".
func TestPhantomCheckUnreadablePRListIsUnverifiable(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return nil, os.ErrDeadlineExceeded
	})
	_, err := phantomCheck(dispatchOpts{item: "assay--example-a--00", kit: "worker"}, allowedRepo)
	if err == nil {
		t.Fatal("an unreadable PR list must not pass as clear")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("could-not-read is UNVERIFIABLE (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}

// The check applies ONLY to a fresh worker dispatch. A review/verifier dispatch and a --pr resume
// act on an existing PR by design, so a PR existing for them is not a phantom.
func TestPhantomCheckSkipsNonFreshDispatch(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		t.Fatal("the transport must not even be read for a non-fresh dispatch")
		return nil, nil
	})
	for _, o := range []dispatchOpts{
		{item: "assay--example-a--00", kit: "review", pr: 0},
		{item: "assay--example-a--00", kit: "verifier", pr: 0},
		{item: "assay--example-a--00", kit: "worker", pr: 373}, // a --pr resume
	} {
		if _, err := phantomCheck(o, allowedRepo); err != nil {
			t.Errorf("phantomCheck must be a no-op for %+v: %v", o, err)
		}
	}
}

// With no transport wired (nil listRepresentedPRs — the offline reference build), the check is inert
// and never reads or refuses.
func TestPhantomCheckNoTransportIsInert(t *testing.T) {
	old := listRepresentedPRs
	listRepresentedPRs = nil
	t.Cleanup(func() { listRepresentedPRs = old })
	if _, err := phantomCheck(dispatchOpts{item: "assay--example-a--00", kit: "worker"}, allowedRepo); err != nil {
		t.Fatalf("an unwired phantom check must be a no-op: %v", err)
	}
}

// A brief that does NOT appear in the represented set dispatches normally.
func TestPhantomCheckAllowsAnUnrepresentedBrief(t *testing.T) {
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 500, State: "OPEN", Body: "Brief: example-other/03"}}, nil
	})
	if _, err := phantomCheck(dispatchOpts{item: "assay--example-a--00", kit: "worker"}, allowedRepo); err != nil {
		t.Fatalf("a brief with no open/merged PR must dispatch: %v", err)
	}
}

// TestPhantomIsCaughtBeforeTheClaim is the end-to-end proof: a full worker dispatch of a phantom
// item is REFUSED and the durable claim is never acquired — nothing is spent chasing it.
func TestPhantomIsCaughtBeforeTheClaim(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = happyReplies("/private/tmp/worker-home")
	withRepresentedPRs(t, func(string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 377, State: "OPEN", Body: "Brief: example-c/14"}}, nil
	})

	rc := run([]string{"assay--example-c--14", "--root", root, "--repo", allowedRepo, "--kit", "worker"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("a phantom dispatch must be REFUSED (exit 5), got %d", rc)
	}
	if s.ran("dispatch-claim.sh acquire") {
		t.Error("the durable claim was acquired for a phantom item — the refusal must precede the claim")
	}
	if s.ran("deskwt add") {
		t.Error("a worktree was cut for a phantom item")
	}
}
