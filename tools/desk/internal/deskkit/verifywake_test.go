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
	"strings"
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

// regression: #2310
func TestWakeToolOnlyHolds(t *testing.T) {
	r := recRIC(map[string]string{"file:a": "rev1", "tool": "tool-v1"})
	got := r.EvaluateWake(stubReader{rev: map[string]string{"file:a": "rev1", "tool": "tool-v2"}}, time.Now())
	if got.State != WakeHold {
		t.Fatalf("state = %v (%q); want hold — a tool-only change does not move the blocker", got.State, got.Reason)
	}
	if !strings.Contains(got.Reason, "tool-only change (tool-v1 -> tool-v2); blocker unmoved") {
		t.Fatalf("reason %q does not name the tool-only change", got.Reason)
	}
}

// regression: #2310
func TestWakeToolAndFileFires(t *testing.T) {
	r := recRIC(map[string]string{"file:a": "rev1", "tool": "tool-v1"})
	got := r.EvaluateWake(stubReader{rev: map[string]string{"file:a": "rev2", "tool": "tool-v2"}}, time.Now())
	if got.State != WakeFire || strings.Join(got.Changed, ",") != "file:a,tool" {
		t.Fatalf("state = %v changed=%v; want fire listing file:a and tool", got.State, got.Changed)
	}
}

// regression: #2310
func TestWakeFileOnlyFires(t *testing.T) {
	r := recRIC(map[string]string{"file:a": "rev1", "tool": "tool-v1"})
	got := r.EvaluateWake(stubReader{rev: map[string]string{"file:a": "rev2", "tool": "tool-v1"}}, time.Now())
	if got.State != WakeFire || strings.Join(got.Changed, ",") != "file:a" {
		t.Fatalf("state = %v changed=%v; want fire listing file:a only", got.State, got.Changed)
	}
}

// regression: #2310
func TestWakeToolOnlyUnreadableCNC(t *testing.T) {
	r := recRIC(map[string]string{"file:a": "rev1", "tool": "tool-v1"})
	got := r.EvaluateWake(stubReader{rev: map[string]string{"tool": "tool-v2"},
		unread: map[string]bool{"file:a": true}}, time.Now())
	if got.State != WakeCouldNotCheck || strings.Join(got.Unreadable, ",") != "file:a" {
		t.Fatalf("state = %v unreadable=%v; want could-not-check naming file:a", got.State, got.Unreadable)
	}
}

// regression: #2310 — the planner's input read (Unchanged) applies the same tool-only rule.
func TestUnchangedToolOnly(t *testing.T) {
	r := recRIC(map[string]string{"file:a": "rev1", "tool": "tool-v1"})
	cases := []struct {
		name   string
		reader stubReader
		want   InputsState
		why    string
	}{
		{"tool only", stubReader{rev: map[string]string{"file:a": "rev1", "tool": "tool-v2"}},
			InputsUnchanged, "unchanged (tool-only change (tool-v1 -> tool-v2); blocker unmoved)"},
		{"tool and file", stubReader{rev: map[string]string{"file:a": "rev2", "tool": "tool-v2"}},
			InputsChanged, "changed a, tool"},
		{"file only", stubReader{rev: map[string]string{"file:a": "rev2", "tool": "tool-v1"}},
			InputsChanged, "changed a"},
		{"tool only, file unreadable", stubReader{rev: map[string]string{"tool": "tool-v2"},
			unread: map[string]bool{"file:a": true}}, InputsCouldNotCheck, "could-not-check unreadable input(s): a"},
		// The file "tool" has the label "tool" but is a file input: it must fire, not hold.
		{"file named tool only", stubReader{rev: map[string]string{"file:tool": "rev2", "tool": "tool-v1"}},
			InputsChanged, "changed tool"},
		// Both change: the shared label is shown as each key, never "tool, tool".
		{"file named tool and tool", stubReader{rev: map[string]string{"file:tool": "rev2", "tool": "tool-v2"}},
			InputsChanged, "changed file:tool, tool"},
	}
	fileNamedTool := recRIC(map[string]string{"file:tool": "rev1", "tool": "tool-v1"})
	for _, c := range cases {
		rec := r
		if strings.HasPrefix(c.name, "file named tool") {
			rec = fileNamedTool
		}
		st, why := Unchanged(rec, c.reader)
		if st != c.want || why != c.why {
			t.Errorf("%s: Unchanged = (%v, %q); want (%v, %q)", c.name, st, why, c.want, c.why)
		}
	}
}

// regression: #2310 — a declared file whose path is "tool" is a file input, never the tool
// input. Its label equals the tool key, so a label-based tool-only test would hold a real file
// change. Both comparators must fire on it.
func TestFileNamedToolFires(t *testing.T) {
	r := recRIC(map[string]string{"file:tool": "rev1", "tool": "tool-v1"})
	reader := stubReader{rev: map[string]string{"file:tool": "rev2", "tool": "tool-v1"}}
	if got := r.EvaluateWake(reader, time.Now()); got.State != WakeFire || strings.Join(got.Changed, ",") != "file:tool" {
		t.Errorf("EvaluateWake state = %v changed=%v (%q); want fire listing file:tool", got.State, got.Changed, got.Reason)
	}
	if st, why := Unchanged(r, reader); st != InputsChanged || why != "changed tool" {
		t.Errorf("Unchanged = (%v, %q); want (%v, %q)", st, why, InputsChanged, "changed tool")
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
