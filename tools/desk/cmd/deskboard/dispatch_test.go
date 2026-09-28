package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func dispatchHdrFixture() Header { return Header{AsOf: "2026-08-23T00:00:00Z"} }

// TestMergeDispatch_MergesAttributesAndSums: rows from two roots merge into one
// score-sorted queue; a view with an empty repo (assay's own tree carries no
// repo: frontmatter) inherits its configured repo; the held-back decomposition
// sums across roots; and a degraded root is collected. The population is
// `dispatch` (todo/in-progress), NEVER the awaiting-verification statuses.
func TestMergeDispatch_MergesAttributesAndSums(t *testing.T) {
	resolved := []deskkit.RootConfig{
		{Repo: "medici-finance/a", Path: "/roots/a"},
		{Repo: "medici-finance/b", Path: "/roots/b"},
	}
	views := []dispatchView{
		{
			Repo: "", // no repo: frontmatter → inherits configured "medici-finance/a"
			Rows: []statusgenDispRow{
				{Brief: "s1/01", Stream: "s1", Status: "todo", Score: 1000},
			},
			Eligible: 3, Shown: 1, HeldByStreamCap: 2, ClaimsKnown: true,
		},
		{
			Repo: "medici-finance/b",
			Rows: []statusgenDispRow{
				{Brief: "s2/02", Stream: "s2", Status: "in-progress", Score: 3000},
			},
			Eligible: 2, Shown: 1, HeldBySpan: 1, ClaimsKnown: false, ClaimsReason: "origin unreachable",
		},
	}
	rep, err := mergeDispatch(dispatchHdrFixture(), "dispatch", resolved, views,
		"statusgen/v0.9.0", "medici-finance/a", "statusgen/v0.9.0")
	if err != nil {
		t.Fatalf("mergeDispatch: %v", err)
	}
	if len(rep.Rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rep.Rows))
	}
	// Score-sorted: s2/02 (3000) before s1/01 (1000).
	if rep.Rows[0].Brief != "s2/02" || rep.Rows[1].Brief != "s1/01" {
		t.Errorf("rows not score-sorted: %+v", rep.Rows)
	}
	// Empty-repo view inherited the configured repo.
	if rep.Rows[1].Repo != "medici-finance/a" {
		t.Errorf("s1/01 repo = %q, want inherited medici-finance/a", rep.Rows[1].Repo)
	}
	if rep.Rows[0].Repo != "medici-finance/b" {
		t.Errorf("s2/02 repo = %q, want medici-finance/b", rep.Rows[0].Repo)
	}
	// Held-back decomposition sums across roots.
	if rep.Eligible != 5 || rep.Shown != 2 || rep.HeldByStreamCap != 2 || rep.HeldBySpan != 1 {
		t.Errorf("held-back sums wrong: eligible=%d shown=%d streamCap=%d span=%d",
			rep.Eligible, rep.Shown, rep.HeldByStreamCap, rep.HeldBySpan)
	}
	// Degraded root collected (its rows are an unfiltered superset).
	if len(rep.ClaimsDegraded) != 1 || rep.ClaimsDegraded[0].Repo != "medici-finance/b" {
		t.Fatalf("claims-degraded not collected: %+v", rep.ClaimsDegraded)
	}
	if rep.ClaimsDegraded[0].Reason != "origin unreachable" {
		t.Errorf("degraded reason = %q", rep.ClaimsDegraded[0].Reason)
	}
	// Population distinguishes dispatch from awaiting.
	if rep.Population != populationDispatch {
		t.Errorf("population = %q, want %q", rep.Population, populationDispatch)
	}
	for _, s := range rep.PopulationStatuses {
		if s == "implemented" || s == "verified" {
			t.Errorf("dispatch population must not include awaiting statuses: %v", rep.PopulationStatuses)
		}
	}
}

// TestMergeDispatch_RepoMismatchFailsClosed: a root configured for X whose view
// declares a different repo Y is a fail-closed misattribution — never a silent
// re-home (the failure the cross-repo board exists to eliminate).
func TestMergeDispatch_RepoMismatchFailsClosed(t *testing.T) {
	resolved := []deskkit.RootConfig{{Repo: "medici-finance/a", Path: "/roots/a"}}
	views := []dispatchView{{Repo: "medici-finance/evil", ClaimsKnown: true}}
	_, err := mergeDispatch(dispatchHdrFixture(), "dispatch", resolved, views, "t", "medici-finance/a", "t")
	if err == nil {
		t.Fatal("expected fail-closed on repo mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "not configured under") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestMergeDispatch_EmptyIsHonest: an all-empty board reports 0 shown / 0 eligible
// with no degraded roots — a genuinely drained queue, distinguishable in render
// from a throttled one via the held-back line.
func TestMergeDispatch_EmptyIsHonest(t *testing.T) {
	resolved := []deskkit.RootConfig{{Repo: "medici-finance/a", Path: "/roots/a"}}
	views := []dispatchView{{Repo: "medici-finance/a", ClaimsKnown: true}}
	rep, err := mergeDispatch(dispatchHdrFixture(), "dispatch", resolved, views, "t", "medici-finance/a", "t")
	if err != nil {
		t.Fatalf("mergeDispatch: %v", err)
	}
	if rep.Shown != 0 || rep.Eligible != 0 {
		t.Errorf("empty board: shown=%d eligible=%d, want 0/0", rep.Shown, rep.Eligible)
	}
	if len(rep.ClaimsDegraded) != 0 {
		t.Errorf("empty board should have no degraded roots: %+v", rep.ClaimsDegraded)
	}
}

// TestDispatchHeldBackLine_EmptyVsThrottled: requirement 3 — a genuinely drained
// queue and one held back by caps produce DIFFERENT lines, so "0 shown" is never a
// bare "drained".
func TestDispatchHeldBackLine_EmptyVsThrottled(t *testing.T) {
	if got := dispatchHeldBackLine(&dispatchReport{}); got != "0 held back" {
		t.Errorf("drained line = %q, want %q", got, "0 held back")
	}
	throttled := dispatchHeldBackLine(&dispatchReport{HeldByStreamCap: 3, HeldBySpan: 2, HeldByDriveCap: 1})
	for _, want := range []string{"3 by per-stream caps", "2 by the span-of-control cap", "1 by the drive anti-starvation floor"} {
		if !strings.Contains(throttled, want) {
			t.Errorf("throttled line %q missing %q", throttled, want)
		}
	}
}

// TestMergeDispatch_DriveWorkerFloorAcrossRoots: the drive worker-pool floor is one
// pool shared by every root, so the merge re-applies statusgen's reported
// driveWorkerCap against the SUM of in-flight drive work. Two roots each within
// their own headroom must still be cut to the shared headroom; non-drive rows are
// never withheld; a root whose in-flight count is unknown withholds every drive
// row; and a root set with no active drive is untouched.
func TestMergeDispatch_DriveWorkerFloorAcrossRoots(t *testing.T) {
	resolved := []deskkit.RootConfig{
		{Repo: "medici-finance/a", Path: "/roots/a"},
		{Repo: "medici-finance/b", Path: "/roots/b"},
	}
	driveRows := func(prefix string, n, base int) []statusgenDispRow {
		var rows []statusgenDispRow
		for i := 0; i < n; i++ {
			rows = append(rows, statusgenDispRow{Brief: prefix + "/0" + string(rune('1'+i)), Stream: prefix,
				Status: "todo", Score: base - i, DriveSlug: "d"})
		}
		return rows
	}
	build := func(unknownB string, capA, capB int) []dispatchView {
		a := driveRows("da", 3, 5000)
		a = append(a, statusgenDispRow{Brief: "fa/01", Stream: "fa", Status: "todo", Score: 100})
		b := driveRows("db", 3, 4000)
		return []dispatchView{
			{Rows: a, Eligible: 4, Shown: 4, ClaimsKnown: true, DriveWorkerCap: capA, DriveInFlight: 2},
			{Repo: "medici-finance/b", Rows: b, Eligible: 3, Shown: 3, ClaimsKnown: true,
				DriveWorkerCap: capB, DriveInFlight: 1, DriveWorkerUnknown: unknownB},
		}
	}
	count := func(rep *dispatchReport) (drive, other int) {
		for _, r := range rep.Rows {
			if r.DriveSlug != "" {
				drive++
			} else {
				other++
			}
		}
		return
	}

	t.Run("shared-headroom", func(t *testing.T) {
		rep, err := mergeDispatch(dispatchHdrFixture(), "dispatch", resolved, build("", 6, 6), "t", "medici-finance/a", "t")
		if err != nil {
			t.Fatal(err)
		}
		// cap 6 − (2+1) in flight = 3 drive rows across BOTH roots, highest score first.
		drive, other := count(rep)
		if drive != 3 || other != 1 {
			t.Fatalf("want 3 drive rows (shared headroom) + 1 non-drive, got drive=%d other=%d: %+v", drive, other, rep.Rows)
		}
		if rep.HeldByDriveWorkerCap != 3 || rep.DriveWorkerCap != 6 || rep.DriveInFlight != 3 {
			t.Fatalf("aggregate floor accounting wrong: held=%d cap=%d inFlight=%d", rep.HeldByDriveWorkerCap, rep.DriveWorkerCap, rep.DriveInFlight)
		}
		for _, r := range rep.Rows {
			if r.DriveSlug != "" && !strings.HasPrefix(r.Brief, "da/") {
				t.Fatalf("the headroom must go to the highest-scored drive rows (root a), got %s", r.Brief)
			}
		}
		if rep.Shown != len(rep.Rows) {
			t.Fatalf("shown %d must equal len(rows) %d", rep.Shown, len(rep.Rows))
		}
		if line := dispatchHeldBackLine(rep); !strings.Contains(line, "3 by the drive worker floor") {
			t.Fatalf("held-back line must name the drive worker floor: %q", line)
		}
	})

	t.Run("unknown-in-flight-withholds-every-drive-row", func(t *testing.T) {
		rep, err := mergeDispatch(dispatchHdrFixture(), "dispatch", resolved, build("claims unread", 6, 6), "t", "medici-finance/a", "t")
		if err != nil {
			t.Fatal(err)
		}
		drive, other := count(rep)
		if drive != 0 || other != 1 || len(rep.DriveWorkerUnknown) != 1 {
			t.Fatalf("unknown in-flight must withhold every drive row and name the root: drive=%d other=%d unknown=%+v", drive, other, rep.DriveWorkerUnknown)
		}
	})

	t.Run("no-active-drive-is-untouched", func(t *testing.T) {
		rep, err := mergeDispatch(dispatchHdrFixture(), "dispatch", resolved, build("", 0, 0), "t", "medici-finance/a", "t")
		if err != nil {
			t.Fatal(err)
		}
		if len(rep.Rows) != 7 || rep.HeldByDriveWorkerCap != 0 {
			t.Fatalf("with no root reporting a drive the queue is unchanged: rows=%d held=%d", len(rep.Rows), rep.HeldByDriveWorkerCap)
		}
	})
}
