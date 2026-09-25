package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// landedreconcile_test.go — #1339: `plan` must not offer a fresh row whose brief already has a PR.
// A MERGED PR means the row is landed-unreconciled (its board cell just never flipped) and must be
// listed, never dispatched; an OPEN PR means started work, routed to the resume lane; a could-not-
// check read HOLDS the fresh lane rather than offering rows on an unverified forge.

// TestPlan_MergedRowIsLandedUnreconciledNotDispatched is the FAIL-FIRST proof (mutations.json's
// #1339 entry reverts the merged→landed routing so this goes red): a `todo` Next-up row whose brief
// carries a MERGED PR must be ABSENT from the DISPATCH list and PRESENT under the LANDED-UNRECONCILED
// heading with the PR number.
func TestPlan_MergedRowIsLandedUnreconciledNotDispatched(t *testing.T) {
	setupDeskHome(t)

	rows := []BoardRow{briefRow("stale", "07", "M", "", "model", false)} // reads todo on the board...
	f := &FanoutLoop{
		Board:  func() ([]BoardRow, error) { return rows, nil },
		Rework: noRework,
		// ...but its brief already MERGED under PR #700 (keyed on the PR's Brief: trailer, at the source).
		Represented: func() (map[string]deskkit.RepresentedPR, error) {
			return map[string]deskkit.RepresentedPR{"stale/07": {Number: 700, Merged: true}}, nil
		},
		Emit: io.Discard,
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	s := out.String()

	if strings.Contains(s, "=== DISPATCH stale/07") {
		t.Errorf("a fresh row whose brief already MERGED was offered for DISPATCH — it is landed-unreconciled (#1339):\n%s", s)
	}
	if !strings.Contains(s, "LANDED-UNRECONCILED") {
		t.Errorf("the LANDED-UNRECONCILED heading is missing (#1339):\n%s", s)
	}
	if !strings.Contains(s, "stale/07 — merged PR #700") {
		t.Errorf("the merged row was not listed under landed-unreconciled with its PR number (#1339):\n%s", s)
	}
}

// TestPlan_OpenPRRowRoutedToResume: a `todo` row whose brief has an OPEN PR is not fresh dispatch —
// it is started work, routed to the existing resume lane (reused, not a second lane invented).
func TestPlan_OpenPRRowRoutedToResume(t *testing.T) {
	setupDeskHome(t)

	rows := []BoardRow{briefRow("wip", "03", "M", "", "model", false)}
	f := &FanoutLoop{
		Board:  func() ([]BoardRow, error) { return rows, nil },
		Rework: noRework,
		Represented: func() (map[string]deskkit.RepresentedPR, error) {
			return map[string]deskkit.RepresentedPR{"wip/03": {Number: 303, Merged: false}}, nil
		},
		Emit: io.Discard,
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	s := out.String()

	if strings.Contains(s, "=== DISPATCH wip/03") {
		t.Errorf("a fresh row whose brief has an OPEN PR was offered as fresh dispatch — it must resume (#1339):\n%s", s)
	}
	if !strings.Contains(s, "=== DISPATCH resume:pr-303") {
		t.Errorf("the OPEN-PR row was not routed to the resume lane (id resume:pr-303) (#1339):\n%s", s)
	}
	if !strings.Contains(s, "RESUME PR") || !strings.Contains(s, "#303") {
		t.Errorf("the resume dispatch instruction does not name the open PR #303 (#1339):\n%s", s)
	}
}

// TestPlan_CouldNotCheckHoldsFreshLane: when the represented read fails, the fresh lane is HELD (the
// row is NOT offered and a FRESH LANE HELD line states why), while the exempt lanes still flow — an
// orphan resume is still dispatched. Could-not-check is not no-PR-exists.
func TestPlan_CouldNotCheckHoldsFreshLane(t *testing.T) {
	setupDeskHome(t)

	rows := []BoardRow{briefRow("freshx", "01", "M", "", "model", false)}
	orphan := OrphanPR{Repo: "example-org/example-repo", Number: 9, ID: "resume:pr-9", Branch: "feat/x", Findings: "address review"}
	f := &FanoutLoop{
		Board:   func() ([]BoardRow, error) { return rows, nil },
		Rework:  noRework,
		Orphans: func() ([]OrphanPR, error) { return []OrphanPR{orphan}, nil },
		Represented: func() (map[string]deskkit.RepresentedPR, error) {
			return nil, fmt.Errorf("gh pr list --repo example-org/example-repo --state all failed: token expired")
		},
		Emit: io.Discard,
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	s := out.String()

	if !strings.Contains(s, "FRESH LANE HELD:") {
		t.Errorf("a could-not-check represented read must print FRESH LANE HELD (#1339):\n%s", s)
	}
	if strings.Contains(s, "=== DISPATCH freshx/01") {
		t.Errorf("a fresh row was offered on an unverified forge — could-not-check is not no-PR-exists (#1339):\n%s", s)
	}
	if !strings.Contains(s, "=== DISPATCH resume:pr-9") {
		t.Errorf("an orphan resume (exempt from the hold) was wrongly dropped when the fresh lane was held (#1339):\n%s", s)
	}
}

// TestRepresentedSourceFor_ReducesPRListOnce proves the transport reduction the command path wires:
// ONE recorded PR list is parsed and reduced to the brief→RepresentedPR map (open+merged only),
// keyed on the Brief: trailer, with no per-row call. It substitutes the package-level transport, so
// no live forge is touched.
func TestRepresentedSourceFor_ReducesPRListOnce(t *testing.T) {
	calls := 0
	orig := representedPRs
	representedPRs = func(repo string) ([]deskkit.PRRef, error) {
		calls++
		if repo != "example-org/example-repo" {
			t.Fatalf("unexpected repo %q", repo)
		}
		return []deskkit.PRRef{
			{Number: 700, State: "MERGED", Body: "Fixes it.\n\nBrief: stale/07\n"},
			{Number: 303, State: "OPEN", Body: "WIP.\n\nBrief: wip/03\n"},
			{Number: 12, State: "CLOSED", Body: "Abandoned.\n\nBrief: dead/02\n"}, // closed-unmerged: represents nothing
		}, nil
	}
	defer func() { representedPRs = orig }()

	src := representedSourceFor("example-org/example-repo", nil)
	m, err := src()
	if err != nil {
		t.Fatalf("representedSourceFor: %v", err)
	}
	if calls != 1 {
		t.Fatalf("the PR list must be read ONCE per plan run, not per row; got %d reads", calls)
	}
	if rp, ok := m["stale/07"]; !ok || rp.Number != 700 || !rp.Merged {
		t.Errorf("merged PR #700 for stale/07 not reduced correctly: %+v", m["stale/07"])
	}
	if rp, ok := m["wip/03"]; !ok || rp.Number != 303 || rp.Merged {
		t.Errorf("open PR #303 for wip/03 not reduced correctly: %+v", m["wip/03"])
	}
	if _, ok := m["dead/02"]; ok {
		t.Errorf("a CLOSED-unmerged PR must represent nothing, yet dead/02 is in the map: %+v", m)
	}
}
