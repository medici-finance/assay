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
