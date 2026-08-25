package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMeasureThreeStateRoundTrip pins the frozen three-state contract: each of
// measured / measured-zero / could-not-measure round-trips through JSONL as a
// DISTINCT state string, and could-not-measure requires a non-empty reason. This
// is the invariant every later brief's numbers depend on.
func TestMeasureThreeStateRoundTrip(t *testing.T) {
	t.Run("measured carries its value and no reason", func(t *testing.T) {
		m := Measured(42)
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(raw), `"state":"measured"`) {
			t.Errorf("expected state measured, got %s", raw)
		}
		if !strings.Contains(string(raw), `"value":42`) {
			t.Errorf("expected value 42, got %s", raw)
		}
		if strings.Contains(string(raw), `"reason"`) {
			t.Errorf("measured must not carry a reason, got %s", raw)
		}
		var back Measure[int]
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if back.State != StateMeasured || back.Value != 42 {
			t.Errorf("round-trip lost data: %+v", back)
		}
	})

	t.Run("measured-zero is distinct from measured and from could-not-measure", func(t *testing.T) {
		m := MeasuredZero[int]()
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(raw), `"state":"measured-zero"`) {
			t.Errorf("expected state measured-zero, got %s", raw)
		}
		var back Measure[int]
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if back.State != StateMeasuredZero {
			t.Errorf("measured-zero did not round-trip: %+v", back)
		}
		// The three states must be three distinct strings.
		if StateMeasured == StateMeasuredZero || StateMeasuredZero == StateCouldNotMeasure || StateMeasured == StateCouldNotMeasure {
			t.Fatal("the three states must be distinct")
		}
	})

	t.Run("could-not-measure round-trips with its reason", func(t *testing.T) {
		m := CouldNotMeasure[int]("blame failed on unreadable blob")
		raw, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if !strings.Contains(string(raw), `"state":"could-not-measure"`) {
			t.Errorf("expected state could-not-measure, got %s", raw)
		}
		if !strings.Contains(string(raw), `"reason":"blame failed on unreadable blob"`) {
			t.Errorf("expected the reason, got %s", raw)
		}
		var back Measure[int]
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if back.State != StateCouldNotMeasure || back.Reason == "" {
			t.Errorf("could-not-measure lost its reason: %+v", back)
		}
	})

	t.Run("could-not-measure with an empty reason is rejected", func(t *testing.T) {
		// Constructing then marshalling an empty-reason could-not-measure must
		// fail — the invariant is enforced on the wire, not just by convention.
		bad := Measure[int]{State: StateCouldNotMeasure, Reason: "   "}
		if _, err := json.Marshal(bad); err == nil {
			t.Fatal("expected marshal to reject an empty-reason could-not-measure")
		}
		// And a hand-edited artifact line with the same defect fails to decode.
		if err := json.Unmarshal([]byte(`{"state":"could-not-measure"}`), &Measure[int]{}); err == nil {
			t.Fatal("expected unmarshal to reject a could-not-measure with no reason")
		}
	})

	t.Run("a measured measure carrying a reason is rejected", func(t *testing.T) {
		bad := Measure[int]{State: StateMeasured, Value: 1, Reason: "should not be here"}
		if _, err := json.Marshal(bad); err == nil {
			t.Fatal("expected marshal to reject a measured measure that carries a reason")
		}
	})

	t.Run("an unknown state is rejected on decode", func(t *testing.T) {
		if err := json.Unmarshal([]byte(`{"state":"guessed","value":1}`), &Measure[int]{}); err == nil {
			t.Fatal("expected unmarshal to reject an unknown state string")
		}
	})

	t.Run("round-trips inside a JSONL line as a nested field", func(t *testing.T) {
		// The way the diff table actually carries it: a struct field of type
		// Measure[[]Hunk]. All three states must survive a full line round-trip.
		type row struct {
			Lines Measure[[]Hunk] `json:"lines"`
		}
		for _, in := range []row{
			{Lines: Measured([]Hunk{{NewLines: 1, Lines: []LineChange{{Op: OpAdd, Content: "x"}}}})},
			{Lines: MeasuredZero[[]Hunk]()},
			{Lines: CouldNotMeasure[[]Hunk]("binary blob")},
		} {
			raw, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("marshal %v: %v", in.Lines.State, err)
			}
			var back row
			if err := json.Unmarshal(raw, &back); err != nil {
				t.Fatalf("unmarshal %s: %v", raw, err)
			}
			if back.Lines.State != in.Lines.State {
				t.Errorf("state changed across round-trip: %q -> %q", in.Lines.State, back.Lines.State)
			}
		}
	})
}
