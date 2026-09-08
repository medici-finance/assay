package main

import (
	"strings"
	"testing"
)

// addressed_test.go — the desk-inbox lead: a fresh board row carrying
// this desk's `to:worker` label is a DIRECTED message and leads the whole dispatch queue,
// ahead of the ordinary board rows (and ahead of orphan resumes / rework).

// TestPlanLeadsWithAddressed is Verify row 5b: a fixture with one to:worker item and three
// plain board rows emits the addressed item FIRST.
func TestPlanLeadsWithAddressed(t *testing.T) {
	rows := []BoardRow{
		briefRow("alpha", "01", "M", "", "model", false),
		briefRow("beta", "02", "M", "", "model", false),
		{Stream: "inbox", Num: "07", Title: "addressed to the worker desk",
			BriefPath: "docs/streams/inbox/brief-07-x.md", Labels: []string{"to:worker"}},
		briefRow("gamma", "03", "M", "", "model", false),
	}
	loop := &FanoutLoop{Board: func() ([]BoardRow, error) { return rows, nil }, Rework: noRework, TargetSHA: "sha"}

	items, err := loop.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}
	if items[0].ID != "inbox/07" {
		var ids []string
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		t.Fatalf("addressed item did not lead the queue; order = %v", ids)
	}
	if items[0].Payload["to"] != "worker" {
		t.Errorf("the leading addressed item is not stamped with its addressee: %v", items[0].Payload)
	}
	// The three plain rows keep their board order after the addressed lead.
	for i, want := range []string{"inbox/07", "alpha/01", "beta/02", "gamma/03"} {
		if items[i].ID != want {
			t.Errorf("item %d = %q, want %q (addressed leads, board order preserved after)", i, items[i].ID, want)
		}
	}
}

// TestAddressedLeadsAheadOfOrphans — the directed message outranks even an orphan-PR
// resume: a `to:worker` board row leads a queue that also holds an orphan resume.
func TestAddressedLeadsAheadOfOrphans(t *testing.T) {
	rows := []BoardRow{
		{Stream: "inbox", Num: "07", Title: "addressed", BriefPath: "docs/streams/inbox/brief-07-x.md", Labels: []string{"to:worker"}},
	}
	orphans := func() ([]OrphanPR, error) {
		return []OrphanPR{{Repo: "example-org/tracker", Number: 5, ID: "orphan-5", Branch: "b"}}, nil
	}
	loop := &FanoutLoop{
		Board:     func() ([]BoardRow, error) { return rows, nil },
		Orphans:   orphans,
		Rework:    noRework,
		TargetSHA: "sha",
	}
	items, err := loop.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	if len(items) != 2 || items[0].ID != "inbox/07" {
		var ids []string
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		t.Fatalf("addressed item must lead ahead of the orphan resume; order = %v", ids)
	}
}

// TestUnaddressedRowsUnaffected — a board with no inbox labels is returned in the ordinary
// order (orphans/rework/fresh), i.e. the addressed lane is inert when nothing is addressed.
func TestUnaddressedRowsUnaffected(t *testing.T) {
	rows := []BoardRow{
		briefRow("alpha", "01", "M", "", "model", false),
		{Stream: "beta", Num: "02", Title: "addressed to another desk",
			BriefPath: "docs/streams/beta/brief-02-x.md", Labels: []string{"to:reviewer"}},
	}
	loop := &FanoutLoop{Board: func() ([]BoardRow, error) { return rows, nil }, Rework: noRework, TargetSHA: "sha"}
	items, err := loop.SelectQueue()
	if err != nil {
		t.Fatalf("SelectQueue: %v", err)
	}
	// A to:reviewer row is not THIS desk's inbox, so it is an ordinary fresh row (kept, not
	// led, not dropped) — board order preserved.
	if len(items) != 2 || items[0].ID != "alpha/01" || items[1].ID != "beta/02" {
		var ids []string
		for _, it := range items {
			ids = append(ids, it.ID)
		}
		t.Fatalf("a to:<other-role> row must not lead this desk's queue; order = %v", ids)
	}
	if strings.TrimSpace(items[1].Payload["to"]) != "" {
		t.Errorf("a to:<other-role> row must not be stamped as this desk's inbox item: %v", items[1].Payload)
	}
}
