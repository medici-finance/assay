package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestHeldByStreamTopRanks pins the decomposition helper: streams rank by held
// count (descending), ties break by name (deterministic board), the top holder
// is named, the total sums every stream, and `max` bounds the rendered fragments
// without changing the total.
func TestHeldByStreamTopRanks(t *testing.T) {
	detail := map[string]int{
		"issue-loop": 650,
		"beta":       3,
		"alpha":      3, // ties with beta on count → alpha sorts first by name
		"gamma":      1,
	}
	frags, top, total := heldByStreamTop(detail, 2)
	if top != "issue-loop" {
		t.Errorf("top = %q, want issue-loop", top)
	}
	if total != 657 {
		t.Errorf("total = %d, want 657 (sum across ALL streams, not just the shown ones)", total)
	}
	want := []string{"issue-loop (650)", "alpha (3)"}
	if !reflect.DeepEqual(frags, want) {
		t.Errorf("frags = %v, want %v (ranked by count desc, then name)", frags, want)
	}
}

// TestOverflowNamesTopStreamHolder is the honest-output half: when a stream sits
// at its dispatch cap and holds its backlog back, the board must NAME the stream
// and the top holder — an unqualified "N held back" reads as an empty board when
// in fact one capped stream is sitting on the whole queue.
func TestOverflowNamesTopStreamHolder(t *testing.T) {
	var briefs []Brief
	for i := 1; i <= 8; i++ {
		briefs = append(briefs, Brief{Num: fmt.Sprintf("%02d", i), Title: "B", Status: "todo"})
	}
	s := mkStream("issue-loop", "active", "P0", briefs...)

	// Four claims consume perStreamCap=4 → cap 0 → all eight eligible briefs are
	// held by the per-stream cap. (KnownClaims: the read succeeded, so the board is
	// filtered, not degraded.)
	claimed := map[string]bool{
		"issue-loop/91": true,
		"issue-loop/92": true,
		"issue-loop/93": true,
		"issue-loop/94": true,
	}
	nu := nextUp([]*Stream{s}, KnownClaims(claimed), nil)
	if nu.HeldByStreamCap != 8 {
		t.Fatalf("HeldByStreamCap = %d, want 8", nu.HeldByStreamCap)
	}
	if nu.HeldByStreamDetail["issue-loop"] != 8 {
		t.Fatalf("HeldByStreamDetail[issue-loop] = %d, want 8", nu.HeldByStreamDetail["issue-loop"])
	}

	sec := nextUpSection(t, emit([]*Stream{s}, nil, nu, nil, nil, IntakeAlarmResult{}, nil, ""))
	if !strings.Contains(sec, "Held by per-stream caps") {
		t.Errorf("board does not decompose the held-back count by stream; got:\n%s", sec)
	}
	if !strings.Contains(sec, "top: issue-loop") {
		t.Errorf("board does not name the top holder; got:\n%s", sec)
	}
	if !strings.Contains(sec, "issue-loop (8)") {
		t.Errorf("board does not state how many issue-loop holds back; got:\n%s", sec)
	}
	if !strings.Contains(sec, "capped here, not drained") {
		t.Errorf("board does not distinguish capped from drained; got:\n%s", sec)
	}
}

// TestNoStreamHoldbackNoDecomposition pins the other side: a board where no
// per-stream cap fired must not print the decomposition line at all, or the
// signal becomes noise.
func TestNoStreamHoldbackNoDecomposition(t *testing.T) {
	s := mkStream("issue-loop", "active", "P0",
		Brief{Num: "01", Title: "One", Status: "todo"},
	)
	nu := nextUp([]*Stream{s}, KnownClaims(nil), nil)
	if nu.HeldByStreamCap != 0 {
		t.Fatalf("HeldByStreamCap = %d, want 0", nu.HeldByStreamCap)
	}
	sec := nextUpSection(t, emit([]*Stream{s}, nil, nu, nil, nil, IntakeAlarmResult{}, nil, ""))
	if strings.Contains(sec, "Held by per-stream caps") {
		t.Errorf("decomposition line rendered with nothing held by a per-stream cap; got:\n%s", sec)
	}
}
