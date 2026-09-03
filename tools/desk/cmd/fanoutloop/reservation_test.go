package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

// --- example-stream/05: per-class concurrency reservation in `plan` --------------------------
//
// These two tests are Verify rows 2 and 3 of example-stream/05's brief: the floor applies
// ONLY when a reserved-class item is waiting (row 2), and never idles a slot when nothing
// reserved is waiting (row 3) — the same "resume/rework outranks fresh, but a reservation is
// never a way to leave a slot empty" property the worker-desk SKILL states in prose and this
// mechanism now enforces in code.

// TestPlanReservationCapsFreshWhenResumeWaits proves the floor: with an orphan resume waiting
// and worker-desk's shipped default reservation (resume=2, rework=0) in force, the printed
// `classes:` line states fresh capped at width(8) - 2 = 6 — even though SelectQueue itself still
// renders every fresh row (the cap is advisory, stated for a pool sizing off this plan, not an
// item this call drops).
func TestPlanReservationCapsFreshWhenResumeWaits(t *testing.T) {
	setupDeskHome(t)

	var fresh []BoardRow
	for i := 1; i <= 5; i++ {
		fresh = append(fresh, briefRow("fresh", fmt.Sprintf("%02d", i), "M", "", "model", false))
	}
	orphan := OrphanPR{Repo: "medici-finance/assay", Number: 1, ID: "resume:pr-1", Branch: "feat/x"}

	f := &FanoutLoop{
		Board:   func() ([]BoardRow, error) { return fresh, nil },
		Orphans: func() ([]OrphanPR, error) { return []OrphanPR{orphan}, nil },
		Rework:  noRework,
		Emit:    io.Discard,
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	s := out.String()

	want := "classes: resume=1 rework=0 fresh=5 (fresh capped at 6 by reservation)"
	if !strings.Contains(s, want) {
		t.Fatalf("missing/wrong classes line; want it to contain %q, got:\n%s", want, s)
	}
	// The cap is ADVISORY: every fresh row still renders (nothing here drops a queue row).
	for _, id := range []string{"fresh/01", "fresh/02", "fresh/03", "fresh/04", "fresh/05"} {
		if !strings.Contains(s, "=== DISPATCH "+id) {
			t.Errorf("the reservation cap dropped a queue row (%s) — it must be advisory only:\n%s", id, s)
		}
	}
}

// TestPlanReservationNeverIdlesASlot proves the inverse: with NOTHING in the resume or rework
// class waiting, the reservation must not idle a slot — the floor does not apply, and the
// printed line says so explicitly rather than silently reporting a cap of 0.
func TestPlanReservationNeverIdlesASlot(t *testing.T) {
	setupDeskHome(t)

	fresh := []BoardRow{briefRow("fresh", "01", "M", "", "model", false)}
	f := &FanoutLoop{
		Board:  func() ([]BoardRow, error) { return fresh, nil },
		Rework: noRework, // no Orphans either: nothing reserved-class is waiting
		Emit:   io.Discard,
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	s := out.String()

	want := "classes: resume=0 rework=0 fresh=1 (no reservation applied: no reserved-class item waiting)"
	if !strings.Contains(s, want) {
		t.Fatalf("missing/wrong classes line; want it to contain %q, got:\n%s", want, s)
	}
	if strings.Contains(s, "capped at") {
		t.Fatalf("a reservation with nothing reserved-class waiting must never state a cap:\n%s", s)
	}
}

// TestPlanReservationClassifiesReworkRows proves the third class: an Awaiting-implementer-rework
// row (kindRework) counts into the `rework=` bucket, not `fresh=`, and — worker-desk's shipped
// default reservation being rework=0 — waiting rework alone applies no floor (the default only
// protects resume; a house that raises rework's reservation gets the same floor behaviour for
// free, exercised by the deskkit-level tests instead of re-derived here).
func TestPlanReservationClassifiesReworkRows(t *testing.T) {
	setupDeskHome(t)

	fresh := []BoardRow{briefRow("fresh", "01", "M", "", "model", false)}
	rework := []BoardRow{briefRow("stuck", "02", "M", "", "model", false)}
	f := &FanoutLoop{
		Board:  func() ([]BoardRow, error) { return fresh, nil },
		Rework: func() ([]BoardRow, error) { return rework, nil },
		Emit:   io.Discard,
	}
	var out bytes.Buffer
	if err := renderPlan(f, &out); err != nil {
		t.Fatalf("renderPlan: %v", err)
	}
	s := out.String()

	if !strings.Contains(s, "classes: resume=0 rework=1 fresh=1") {
		t.Fatalf("rework row not classified into rework=1:\n%s", s)
	}
	if !strings.Contains(s, "=== DISPATCH stuck/02") {
		t.Fatalf("a rework row must still be dispatched (it outranks fresh, never dropped):\n%s", s)
	}
}
