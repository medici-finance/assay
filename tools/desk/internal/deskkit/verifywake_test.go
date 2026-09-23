package deskkit

// verifywake_test.go — unit coverage for the pure wake-receipt evaluator (example-stream/16).
//
// FAIL-FIRST: these pin the evaluator's four states directly. A mutation that makes
// EvaluateWake return WakeHold whenever it is unsure (the exact "fabricate an unchanged claim"
// defect the brief warns against) reddens TestEvaluateWake_UnreadableIsCouldNotCheck and
// TestEvaluateWake_LegacyAndIncompleteAreUnclassified; a mutation that skips the per-input
// comparison reddens TestEvaluateWake_RelevantChangeFires. See the PR's ## Fail-first section.

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

type stubReader struct {
	rev    map[string]string
	unread map[string]bool
	done   map[string]bool
}

func (s stubReader) Revision(in string) (string, bool) {
	if s.unread[in] {
		return "", false
	}
	r, ok := s.rev[in]
	return r, ok
}
func (s stubReader) ActionCompleted(ref string) (bool, bool) { d, ok := s.done[ref]; return d, ok }

func recRIC(inputs map[string]string) WakeReceipt {
	return NewWakeReceipt("id", "o/r", "s/01", "verifier", "verify-fail", "sha", nil, inputs,
		"tool-v1", BlockerImplementation, "", WakeRelevantInputChanged, "", "", "ts")
}

func TestEvaluateWake_UnchangedHolds(t *testing.T) {
	r := recRIC(map[string]string{"file:a": "rev1", "tool": "tool-v1"})
	got := r.EvaluateWake(stubReader{rev: map[string]string{"file:a": "rev1", "tool": "tool-v1"}}, time.Now())
	if got.State != WakeHold {
		t.Fatalf("state = %v (%q); want hold", got.State, got.Reason)
	}
}

func TestEvaluateWake_RelevantChangeFires(t *testing.T) {
	r := recRIC(map[string]string{"file:a": "rev1"})
	got := r.EvaluateWake(stubReader{rev: map[string]string{"file:a": "rev2"}}, time.Now())
	if got.State != WakeFire || len(got.Changed) != 1 || got.Changed[0] != "file:a" {
		t.Fatalf("state = %v changed=%v; want fire naming file:a", got.State, got.Changed)
	}
}

func TestEvaluateWake_UnreadableIsCouldNotCheck(t *testing.T) {
	r := recRIC(map[string]string{"file:a": "rev1"})
	got := r.EvaluateWake(stubReader{unread: map[string]bool{"file:a": true}}, time.Now())
	if got.State != WakeCouldNotCheck {
		t.Fatalf("state = %v; want could-not-check — an unreadable input must never round up to unchanged", got.State)
	}
}

func TestEvaluateWake_ActionAndDeadline(t *testing.T) {
	// referenced-action-completed
	act := NewWakeReceipt("id", "o/r", "s/01", "v", "blocked", "sha", nil, nil, "tool-v1",
		BlockerHumanAction, "o/r#9", WakeReferencedActionDone, "", "", "ts")
	if got := act.EvaluateWake(stubReader{done: map[string]bool{"o/r#9": true}}, time.Now()); got.State != WakeFire {
		t.Fatalf("completed action state = %v; want fire", got.State)
	}
	if got := act.EvaluateWake(stubReader{done: map[string]bool{"o/r#9": false}}, time.Now()); got.State != WakeHold {
		t.Fatalf("incomplete action state = %v; want hold", got.State)
	}
	if got := act.EvaluateWake(stubReader{}, time.Now()); got.State != WakeCouldNotCheck {
		t.Fatalf("unobservable action state = %v; want could-not-check", got.State)
	}
	// declared-deadline-reached
	dl := NewWakeReceipt("id", "o/r", "s/01", "v", "blocked", "sha", nil, nil, "tool-v1",
		BlockerEnvironment, "", WakeDeadlineReached, "2026-09-20", "", "ts")
	before := time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)
	after := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	if got := dl.EvaluateWake(stubReader{}, before); got.State != WakeHold {
		t.Fatalf("pre-deadline state = %v; want hold", got.State)
	}
	if got := dl.EvaluateWake(stubReader{}, after); got.State != WakeFire {
		t.Fatalf("post-deadline state = %v; want fire", got.State)
	}
}

func TestEvaluateWake_LegacyAndIncompleteAreUnclassified(t *testing.T) {
	legacy := WakeReceipt{Brief: "s/01", Outcome: "verify-fail"} // no schema/wake fields
	if got := legacy.EvaluateWake(stubReader{}, time.Now()); got.State != WakeUnclassified {
		t.Fatalf("legacy state = %v; want unclassified", got.State)
	}
	// complete schema but empty input scope for relevant-input-changed → cannot establish unchangedness
	incomplete := recRIC(nil)
	if incomplete.Complete() {
		t.Fatalf("empty-scope relevant-input-changed receipt must not be Complete")
	}
	if got := incomplete.EvaluateWake(stubReader{}, time.Now()); got.State != WakeUnclassified {
		t.Fatalf("incomplete state = %v; want unclassified (no fabricated unchanged claim)", got.State)
	}
	// unknown blocker kind / predicate → not complete
	bad := recRIC(map[string]string{"file:a": "rev1"})
	bad.BlockerKind = "made-up"
	if bad.Complete() {
		t.Fatalf("unknown blocker kind must not be Complete")
	}
}

func TestRootRevisionReader_OfflineAndContained(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	rr := NewRootRevisionReader(dir, "tool-v1")
	h1, ok := rr.Revision("file:a.txt")
	if !ok || h1 == "" {
		t.Fatalf("readable file must return a revision")
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2, _ := rr.Revision("file:a.txt")
	if h1 == h2 {
		t.Fatalf("content change must change the revision hash")
	}
	if v, ok := rr.Revision("tool"); !ok || v != "tool-v1" {
		t.Fatalf("tool revision = %q ok=%v; want tool-v1", v, ok)
	}
	if _, ok := rr.Revision("file:../escape"); ok {
		t.Fatalf("a path climbing out of root must be could-not-check, never read")
	}
	if _, ok := rr.Revision("file:missing"); ok {
		t.Fatalf("a missing file must be could-not-check")
	}
	if done, ok := rr.ActionCompleted("o/r#1"); ok || done {
		t.Fatalf("offline reader must never observe an action (could-not-check)")
	}
}
