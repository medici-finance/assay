package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// phantom_window_test.go — the phantom check past an INCOMPLETE represented-PR window (#5853).
// The window is the real seam reduction (Forge.ListChanges → deskkit.RepresentedPRRefs) over a
// recorded change list whose Incomplete flag is set and which omits the brief, so the only way the
// check can see the brief's delivery is the per-brief trailer lookup.

// windowTransport is liveRepresentedPRs minus ResolveForge (which needs credentials): the shipped
// representedWindow over a fake forge, so the window's completeness reaches the check exactly as live.
func windowTransport(f deskkit.Forge) func(string) ([]deskkit.PRRef, bool, error) {
	return func(repo string) ([]deskkit.PRRef, bool, error) {
		fr, err := forgeRepoOf(repo)
		if err != nil {
			return nil, false, err
		}
		return representedWindow(f, repo, fr)
	}
}

// recordedWindow is a window of unrelated changes; it never represents example-a/00.
func recordedWindow(incomplete bool) fakeForge {
	return fakeForge{cl: &deskkit.ChangeList{Incomplete: incomplete, Changes: []deskkit.ChangeRef{
		{Number: 900, State: "MERGED", Body: "Brief: example-other/01"},
		{Number: 901, State: "OPEN", Body: "Brief: example-other/02"},
	}}}
}

// lookupCalls wires a recorded trailer lookup and returns the list of ids it was asked for.
func lookupCalls(t *testing.T, fn func(repo, id string) ([]deskkit.PRRef, error)) *[]string {
	t.Helper()
	var calls []string
	old := lookupBriefChanges
	lookupBriefChanges = func(repo, id string) ([]deskkit.PRRef, error) {
		calls = append(calls, id)
		return fn(repo, id)
	}
	t.Cleanup(func() { lookupBriefChanges = old })
	return &calls
}

var freshItem = dispatchOpts{item: "assay--example-a--00", kit: "worker"}

// regression: #5853
func TestWindowMissMergedRefuses(t *testing.T) {
	withRepresentedWindow(t, windowTransport(recordedWindow(true)))
	calls := lookupCalls(t, func(string, string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{
			{Number: 12, State: "MERGED", Body: "Delivers it.\n\nBrief: example-a/00\n"},
		}, nil
	})
	_, err := phantomCheck(freshItem, allowedRepo)
	if err == nil {
		t.Fatal("a brief delivered by a merged PR outside the incomplete window was dispatched as new work")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("a delivered brief is a REFUSAL (exit 5), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
	if !strings.Contains(err.Error(), "#12") || !strings.Contains(err.Error(), "DELIVERED") {
		t.Errorf("the refusal must name the merged PR the lookup found: %v", err)
	}
	if len(*calls) != 1 || (*calls)[0] != "example-a/00" {
		t.Errorf("lookup calls = %q, want exactly one for example-a/00", *calls)
	}
}

// regression: #5853
func TestWindowMissLookupErrHolds(t *testing.T) {
	withRepresentedWindow(t, windowTransport(recordedWindow(true)))
	lookupCalls(t, func(string, string) ([]deskkit.PRRef, error) {
		return nil, errors.New("search is incomplete")
	})
	_, err := phantomCheck(freshItem, allowedRepo)
	if err == nil {
		t.Fatal("a failed trailer lookup past an incomplete window was rounded to not-found")
	}
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable || !strings.Contains(err.Error(), "could-not-check") {
		t.Fatalf("a failed lookup is could-not-check (exit 6), got exit %d: %v", deskkit.ExitCodeOf(err), err)
	}
}

// regression: #5853
func TestWindowMissNoLookupHolds(t *testing.T) {
	withRepresentedWindow(t, windowTransport(recordedWindow(true)))
	old := lookupBriefChanges
	lookupBriefChanges = nil
	t.Cleanup(func() { lookupBriefChanges = old })
	_, err := phantomCheck(freshItem, allowedRepo)
	if deskkit.ExitCodeOf(err) != deskkit.ExitUnverifiable {
		t.Fatalf("no lookup past an incomplete window must HOLD as could-not-check, got exit %d: %v",
			deskkit.ExitCodeOf(err), err)
	}
}

// regression: #5853
func TestWindowMissNewRowDispatches(t *testing.T) {
	withRepresentedWindow(t, windowTransport(recordedWindow(true)))
	calls := lookupCalls(t, func(string, string) ([]deskkit.PRRef, error) {
		// A closed-unmerged PR and a body that only mentions the phrase deliver nothing.
		return []deskkit.PRRef{
			{Number: 13, State: "CLOSED", Body: "Brief: example-a/00"},
			{Number: 14, State: "MERGED", Body: "see the Brief: example-a/00 thread"},
		}, nil
	})
	if _, err := phantomCheck(freshItem, allowedRepo); err != nil {
		t.Fatalf("a brief the complete lookup finds undelivered must dispatch: %v", err)
	}
	if len(*calls) != 1 {
		t.Errorf("lookup calls = %q, want one", *calls)
	}
}

// regression: #5853
func TestCompleteWindowNoLookup(t *testing.T) {
	withRepresentedWindow(t, windowTransport(recordedWindow(false)))
	calls := lookupCalls(t, func(string, string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 12, State: "MERGED", Body: "Brief: example-a/00"}}, nil
	})
	if _, err := phantomCheck(freshItem, allowedRepo); err != nil {
		t.Fatalf("a new row against a complete window must dispatch: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("a complete window made %d extra lookup(s): %q", len(*calls), *calls)
	}
}

// regression: #5853
func TestWindowHitNoLookup(t *testing.T) {
	f := recordedWindow(true)
	f.cl.Changes = append(f.cl.Changes, deskkit.ChangeRef{Number: 700, State: "MERGED", Body: "Brief: example-a/00"})
	withRepresentedWindow(t, windowTransport(f))
	calls := lookupCalls(t, func(string, string) ([]deskkit.PRRef, error) {
		t.Error("the window already represents the brief; no lookup is owed")
		return nil, nil
	})
	if _, err := phantomCheck(freshItem, allowedRepo); err == nil || !strings.Contains(err.Error(), "#700") {
		t.Fatalf("the window's own merged PR must refuse: %v", err)
	}
	if len(*calls) != 0 {
		t.Errorf("lookup calls = %q, want none", *calls)
	}
}

// regression: #5853
func TestWindowMissCrossRepoLink(t *testing.T) {
	withRepresentedWindow(t, windowTransport(recordedWindow(true)))
	calls := lookupCalls(t, func(_, id string) ([]deskkit.PRRef, error) {
		if id != "example-home:example-a/00" {
			return nil, nil
		}
		return []deskkit.PRRef{{Number: 15, State: "MERGED",
			Body: "Delivers it.\n\nCross-repo-brief: example-home:example-a/00\n"}}, nil
	})
	dl := deliverable{crossRepoBrief: true, homeAlias: "example-home"}
	_, err := phantomCheckFor(freshItem, allowedRepo, dl)
	if err == nil || !strings.Contains(err.Error(), "#15") {
		t.Fatalf("a cross-repo delivery outside the incomplete window must refuse: %v", err)
	}
	if strings.Join(*calls, ",") != "example-a/00,example-home:example-a/00" {
		t.Errorf("lookup calls = %q, want the brief id then the cross-repo link", *calls)
	}
}
