package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// TestExitOf_OneExitLands is the positive control.
func TestExitOf_OneExitLands(t *testing.T) {
	got, err := ExitOf(ExitPlaceholder, "")
	if err != nil || got != ExitPlaceholder {
		t.Fatalf("ExitOf = %s, %v", got, err)
	}
	got, err = ExitOf("", ExitBug)
	if err != nil || got != ExitBug {
		t.Fatalf("ExitOf = %s, %v", got, err)
	}
}

// TestExitOf_NoExitIsARefusal — an inbound item that lands with no exit is the front door leaking,
// and the whole desk exists to hold that property.
func TestExitOf_NoExitIsARefusal(t *testing.T) {
	_, err := ExitOf("", "")
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if !strings.Contains(err.Error(), "front door") {
		t.Fatalf("the refusal does not say what was lost: %v", err)
	}
}

// TestExitOf_EmittedButUnroutedIsARefusal — emitting an item for a model tier and hearing nothing
// back is not an exit. It is the same leak wearing a busier face.
func TestExitOf_EmittedButUnroutedIsARefusal(t *testing.T) {
	_, err := ExitOf(ExitUnrouted, "")
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if !strings.Contains(err.Error(), "UNROUTED") {
		t.Fatalf("the refusal does not name the unrouted state: %v", err)
	}
}

// TestExitOf_TwoDisagreeingExitsIsARefusal_NotAPrecedenceRule — a silent winner would let the two
// sources drift apart forever, and the first anyone would learn of it is a downstream owner acting
// on an exit nobody chose.
func TestExitOf_TwoDisagreeingExitsIsARefusal_NotAPrecedenceRule(t *testing.T) {
	_, err := ExitOf(ExitPlaceholder, ExitBug)
	if deskkit.ExitCodeOf(err) != deskkit.ExitRefused {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

// TestExitOf_AgreeingSourcesAreFine — the same exit from both sides is one exit, not two.
func TestExitOf_AgreeingSourcesAreFine(t *testing.T) {
	got, err := ExitOf(ExitPlaceholder, ExitPlaceholder)
	if err != nil || got != ExitPlaceholder {
		t.Fatalf("ExitOf = %s, %v", got, err)
	}
}

// TestExitSet_IsClosed — a value outside the five is never a tracked exit, however plausible.
func TestExitSet_IsClosed(t *testing.T) {
	if Exit("wontfix").Tracked() {
		t.Fatal("an unknown exit reported itself tracked")
	}
	if len(trackedExits) != 5 {
		t.Fatalf("the tracked-exit set has %d members, want the declared five", len(trackedExits))
	}
	if ExitUnrouted.Tracked() {
		t.Fatal("the not-yet-routed marker reported itself as a tracked exit")
	}
}

func TestLedger_RefusesASecondDifferentExit(t *testing.T) {
	l := NewExitLedger()
	if err := l.Record(ExitRecord{ItemID: "x#1", Exit: ExitPlaceholder}); err != nil {
		t.Fatal(err)
	}
	// Re-landing the SAME exit is idempotent — a retried land must not be an error.
	if err := l.Record(ExitRecord{ItemID: "x#1", Exit: ExitPlaceholder}); err != nil {
		t.Fatalf("an idempotent re-land was refused: %v", err)
	}
	if err := l.Record(ExitRecord{ItemID: "x#1", Exit: ExitBug}); err == nil {
		t.Fatal("an item was recorded as leaving by two different exits")
	}
}

// TestLedger_LeakCheckNamesWhatDidNotLeave — the check runs against the QUEUED set, so an item that
// was admitted and then quietly dropped is caught.
func TestLedger_LeakCheckNamesWhatDidNotLeave(t *testing.T) {
	l := NewExitLedger()
	_ = l.Record(ExitRecord{ItemID: "x#1", Exit: ExitPlaceholder})
	leaked := l.Unexited([]string{"x#1", "x#2", "x#3"})
	if len(leaked) != 2 || leaked[0] != "x#2" || leaked[1] != "x#3" {
		t.Fatalf("leaked = %v, want x#2 and x#3", leaked)
	}
	if got := l.Unexited([]string{"x#1"}); len(got) != 0 {
		t.Fatalf("a fully drained queue reported %v", got)
	}
}

func TestLedger_CountsByExit(t *testing.T) {
	l := NewExitLedger()
	_ = l.Record(ExitRecord{ItemID: "a#1", Exit: ExitPlaceholder})
	_ = l.Record(ExitRecord{ItemID: "a#2", Exit: ExitPlaceholder})
	_ = l.Record(ExitRecord{ItemID: "a#3", Exit: ExitNeedsDecision})
	c := l.CountByExit()
	if c[ExitPlaceholder] != 2 || c[ExitNeedsDecision] != 1 {
		t.Fatalf("counts = %v", c)
	}
	if got := len(l.Records()); got != 3 {
		t.Fatalf("records = %d, want 3 in land order", got)
	}
}

func TestResultExit_ReadsTheRoutingDecisionOffTheFrozenResult(t *testing.T) {
	r := loopengine.Result{Item: loopengine.Item{ID: "a#1", Payload: map[string]string{"exit": " bug "}}}
	if got := resultExit(r); got != ExitBug {
		t.Fatalf("resultExit = %q, want %q", got, ExitBug)
	}
	if got := resultExit(loopengine.Result{}); got != "" {
		t.Fatalf("a result with no payload produced %q", got)
	}
}

func TestRepoOfItemID(t *testing.T) {
	if got := repoOfItemID("example-org/tracker#42"); got != "example-org/tracker" {
		t.Fatalf("repoOfItemID = %q", got)
	}
	if got := repoOfItemID("nonsense"); got != "" {
		t.Fatalf("repoOfItemID = %q, want empty for an unparsable key", got)
	}
}

// TestFiledExits_MatchTheLaneThatFilesThem — the labels a filed exit carries are part of the exit,
// not of the call site.
func TestFiledExits_MatchTheLaneThatFilesThem(t *testing.T) {
	if !ExitBug.Filed() || !ExitNeedsDecision.Filed() {
		t.Fatal("a filed exit reported itself unfiled")
	}
	if ExitPlaceholder.Filed() || ExitFinding.Filed() || ExitRejectedWatching.Filed() {
		t.Fatal("an exit the filing lane does not produce reported itself filed")
	}
	if got := ExitBug.Labels(); len(got) != 1 || got[0] != "bug" {
		t.Fatalf("labels = %v", got)
	}
	if got := ExitPlaceholder.Labels(); got != nil {
		t.Fatalf("labels = %v, want none", got)
	}
}
