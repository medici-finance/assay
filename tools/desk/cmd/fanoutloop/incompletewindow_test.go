package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// incompletewindow_test.go — #5853: plan's represented read is a bounded window of the repo's most
// recently updated open+merged PRs. When the window is INCOMPLETE, a queued brief whose delivering
// PR is older than the window must be looked up by its `Brief:` trailer, never offered as IMPLEMENT
// on absence alone. Each test wires the same representedSourceFor + candidateBriefIDs closure
// cmdPlan wires (planWithRows) and renders through renderPlan. Stream slugs are synthetic.

// withWindow wires a recorded window (prs, incomplete) and a recorded per-brief lookup for one test,
// restoring the nil defaults. It returns a pointer to the lookup's call count.
func withWindow(t *testing.T, prs []deskkit.PRRef, incomplete bool, lookup func(repo, id string) ([]deskkit.PRRef, error)) *int {
	t.Helper()
	oldPRs, oldFiles, oldLookup := representedPRs, representedPRFiles, briefChangeLookup
	calls := new(int)
	representedPRs = func(string) ([]deskkit.PRRef, bool, error) { return prs, incomplete, nil }
	representedPRFiles = nil
	briefChangeLookup = func(repo, id string) ([]deskkit.PRRef, error) {
		*calls++
		return lookup(repo, id)
	}
	t.Cleanup(func() { representedPRs, representedPRFiles, briefChangeLookup = oldPRs, oldFiles, oldLookup })
	return calls
}

// recentWindow is a window that holds only an unrelated, recently updated PR.
var recentWindow = []deskkit.PRRef{{Number: 9001, State: "OPEN", Body: "Recent.\n\nBrief: example-other/02\n"}}

func renderWindowPlan(t *testing.T, f *FanoutLoop) string {
	t.Helper()
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	return out.String()
}

// regression: #5853
func TestPlan_WindowMissLookupLands(t *testing.T) {
	setupDeskHome(t)
	withWindow(t, recentWindow, true, func(_, id string) ([]deskkit.PRRef, error) {
		if id != "example-port/11" {
			return nil, nil
		}
		return []deskkit.PRRef{{Number: 4100, State: "MERGED", Body: "Delivers it.\n\nBrief: example-port/11\n"}}, nil
	})
	s := renderWindowPlan(t, planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "model", false)}))
	if strings.Contains(s, "=== DISPATCH example-port/11") {
		t.Errorf("a todo row delivered outside the incomplete window was offered as IMPLEMENT (#5853):\n%s", s)
	}
	if !strings.Contains(s, "LANDED-UNRECONCILED") || !strings.Contains(s, "example-port/11 — merged PR #4100") {
		t.Errorf("the lookup's merged PR was not listed under LANDED-UNRECONCILED (#5853):\n%s", s)
	}
}

// regression: #5853
func TestPlan_LookupErrorUnchecked(t *testing.T) {
	setupDeskHome(t)
	withWindow(t, recentWindow, true, func(string, string) ([]deskkit.PRRef, error) {
		return nil, errors.New("search response incomplete")
	})
	s := renderWindowPlan(t, planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "model", false)}))
	if strings.Contains(s, "=== DISPATCH example-port/11") {
		t.Errorf("a failed lookup was rounded to no-PR and the row offered as IMPLEMENT (#5853):\n%s", s)
	}
	if !strings.Contains(s, "=== COULD-NOT-CHECK: 1 row(s)") || !strings.Contains(s, "example-port/11 — ") || !strings.Contains(s, "search response incomplete") {
		t.Errorf("the row is not listed under COULD-NOT-CHECK with the lookup's cause (#5853):\n%s", s)
	}
	if strings.Contains(s, "LANDED-UNRECONCILED") {
		t.Errorf("a could-not-check row was reported as landed (#5853):\n%s", s)
	}
}

// regression: #5853
func TestPlan_CompleteWindowNoLookup(t *testing.T) {
	setupDeskHome(t)
	calls := withWindow(t, recentWindow, false, func(string, string) ([]deskkit.PRRef, error) {
		return []deskkit.PRRef{{Number: 4100, State: "MERGED", Body: "Brief: example-port/11\n"}}, nil
	})
	s := renderWindowPlan(t, planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "model", false)}))
	if *calls != 0 {
		t.Errorf("a complete window still ran %d per-brief lookup(s) (#5853)", *calls)
	}
	if !strings.Contains(s, "=== DISPATCH example-port/11") {
		t.Errorf("a new todo row with no PR in a complete window was not dispatched (#5853):\n%s", s)
	}
}

// regression: #5853
func TestPlan_HeldLandedNamesKey(t *testing.T) {
	setupDeskHome(t)
	root := t.TempDir()
	dir := filepath.Join(root, "docs", "streams", "example-port")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	brief := "---\nschema: brief-v3\nauthored: \"2026-10-01 by example-author\"\ngate: human\n---\n# Brief 11\n"
	if err := os.WriteFile(filepath.Join(dir, "brief-11-x.md"), []byte(brief), 0o644); err != nil {
		t.Fatal(err)
	}
	withWindow(t, []deskkit.PRRef{{Number: 4100, State: "MERGED", Body: "Brief: example-port/11\n"}}, false,
		func(string, string) ([]deskkit.PRRef, error) { return nil, nil })
	f := planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "human", false)})
	f.Root = root
	s := renderWindowPlan(t, f)
	for _, key := range []string{"`design:`", "`decision-issue:`", "`ruling:`"} {
		if !strings.Contains(s, key) {
			t.Errorf("the lint-held landed row does not name the missing %s (#5853):\n%s", key, s)
		}
	}
	if strings.Contains(s, "reconcile --backfill --apply") {
		t.Errorf("a lint-held landed row was still sent to the reconcile (#5853):\n%s", s)
	}
}

// recordLookups wires a window-incomplete plan whose per-brief lookup records the ids it is asked
// for, in call order, and answers each through answer.
func recordLookups(t *testing.T, answer func(n int, id string) ([]deskkit.PRRef, error)) *[]string {
	t.Helper()
	asked := &[]string{}
	withWindow(t, recentWindow, true, func(_, id string) ([]deskkit.PRRef, error) {
		*asked = append(*asked, id)
		return answer(len(*asked), id)
	})
	return asked
}

// regression: #5853
func TestPlan_LookupPriorityOrder(t *testing.T) {
	setupDeskHome(t)
	asked := recordLookups(t, func(int, string) ([]deskkit.PRRef, error) { return nil, nil })
	f := planWithRows([]BoardRow{
		briefRow("example-zeta", "01", "M", "", "model", false),
		briefRow("example-alpha", "02", "M", "", "model", false),
	})
	f.Rework = func() ([]BoardRow, error) {
		return []BoardRow{briefRow("example-mid", "03", "M", "", "model", false)}, nil
	}
	renderWindowPlan(t, f)
	want := []string{"example-mid/03", "example-zeta/01", "example-alpha/02"}
	if strings.Join(*asked, ",") != strings.Join(want, ",") {
		t.Errorf("lookups ran in %v, want queue priority order %v (rework, then Next-up in board order) (#5853)", *asked, want)
	}
}

// regression: #5853
func TestPlan_RateLimitStopsLookups(t *testing.T) {
	setupDeskHome(t)
	asked := recordLookups(t, func(n int, _ string) ([]deskkit.PRRef, error) {
		if n == 2 {
			return nil, &deskkit.ForgeAPIError{Status: 429, Method: "GET", Path: "search/issues"}
		}
		return nil, nil
	})
	s := renderWindowPlan(t, planWithRows([]BoardRow{
		briefRow("example-port", "14", "M", "", "model", false),
		briefRow("example-port", "11", "M", "", "model", false),
		briefRow("example-port", "13", "M", "", "model", false),
		briefRow("example-port", "12", "M", "", "model", false),
	}))
	if want := "example-port/14,example-port/11"; strings.Join(*asked, ",") != want {
		t.Errorf("lookups after the rate limit: asked %v, want exactly %s then stop (#5853)", *asked, want)
	}
	if !strings.Contains(s, "=== DISPATCH example-port/14") {
		t.Errorf("the row checked before the rate limit (no PR) was not dispatched (#5853):\n%s", s)
	}
	for _, id := range []string{"11", "13", "12"} {
		if strings.Contains(s, "=== DISPATCH example-port/"+id) {
			t.Errorf("example-port/%s was dispatched although its lookup was rate-limited or never run (#5853):\n%s", id, s)
		}
	}
	if !strings.Contains(s, "=== COULD-NOT-CHECK: 3 row(s)") {
		t.Errorf("the rate-limited row and the two never looked up are not all COULD-NOT-CHECK (#5853):\n%s", s)
	}
	for _, id := range []string{"13", "12"} {
		if !strings.Contains(s, "example-port/"+id+" — the PR list is a bounded window and the `Brief:` lookup was not run") {
			t.Errorf("example-port/%s is not listed as never looked up after the rate limit (#5853):\n%s", id, s)
		}
	}
	head := strings.Index(s, "COULD-NOT-CHECK: 3 row(s) NOT dispatched")
	if head < 0 || head > strings.Index(s, "=== DISPATCH") {
		t.Errorf("the COULD-NOT-CHECK count is not stated in the plan header (#5853):\n%s", s)
	}
}

// regression: #5853
func TestPlan_NilLookupUnchecked(t *testing.T) {
	setupDeskHome(t)
	withWindow(t, recentWindow, true, nil)
	briefChangeLookup = nil
	s := renderWindowPlan(t, planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "model", false)}))
	if strings.Contains(s, "=== DISPATCH example-port/11") {
		t.Errorf("an incomplete window with no lookup wired offered the row as IMPLEMENT (#5853):\n%s", s)
	}
	if !strings.Contains(s, "example-port/11 — the PR list is a bounded window and no per-brief lookup is wired") {
		t.Errorf("the row is not COULD-NOT-CHECK for the unwired lookup (#5853):\n%s", s)
	}
}

// regression: #5853
func TestPlan_LookupNewestHitWins(t *testing.T) {
	setupDeskHome(t)
	withWindow(t, recentWindow, true, func(string, string) ([]deskkit.PRRef, error) {
		// Oldest-created first, as the search returns them.
		return []deskkit.PRRef{
			{Number: 100, State: "MERGED", Body: "First cut.\n\nBrief: example-port/11\n"},
			{Number: 200, State: "OPEN", Body: "Follow-up.\n\nBrief: example-port/11\n"},
		}, nil
	})
	s := renderWindowPlan(t, planWithRows([]BoardRow{briefRow("example-port", "11", "M", "", "model", false)}))
	if !strings.Contains(s, "=== DISPATCH resume:pr-200") || strings.Contains(s, "merged PR #100") {
		t.Errorf("the old merged hit outranked the newer open follow-up (#5853):\n%s", s)
	}
}
