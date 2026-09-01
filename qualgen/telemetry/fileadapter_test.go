package telemetry

import "testing"

func TestFileAdapter_MeasuredRecord(t *testing.T) {
	a := NewFileAdapter("testdata/sessions.jsonl")
	if err := a.OpenError(); err != nil {
		t.Fatalf("unexpected open error: %v", err)
	}
	key := PRKey{PRNumber: 42, MergeSHA: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", StreamTaskID: "quality/13"}
	m := a.Telemetry(key)
	if m.State != StateMeasured {
		t.Fatalf("expected measured, got %+v", m)
	}
	if m.Value.Retries != 7 || m.Value.ContextLength != 18000 || m.Value.ToolCallChurn != 3 ||
		m.Value.Interruptions != 1 || m.Value.Refusals != 0 {
		t.Fatalf("record did not decode as planted: %+v", m.Value)
	}
}

func TestFileAdapter_AllZeroRecordIsMeasuredZero(t *testing.T) {
	a := NewFileAdapter("testdata/sessions.jsonl")
	key := PRKey{PRNumber: 43, MergeSHA: "c0ffeec0ffeec0ffeec0ffeec0ffeec0ffeec0ff", StreamTaskID: "quality/13"}
	m := a.Telemetry(key)
	if m.State != StateMeasuredZero {
		t.Fatalf("expected a genuine all-zero session to be measured-zero, got %+v", m)
	}
}

func TestFileAdapter_IncompleteRecordIsCouldNotMeasure(t *testing.T) {
	a := NewFileAdapter("testdata/sessions.jsonl")
	key := PRKey{PRNumber: 44, MergeSHA: "badc0debadc0debadc0debadc0debadc0debadc0", StreamTaskID: "quality/13"}
	m := a.Telemetry(key)
	if m.State != StateCouldNotMeasure {
		t.Fatalf("expected a record missing required fields to be could-not-measure, never zero: got %+v", m)
	}
	if m.Reason == "" {
		t.Fatal("could-not-measure requires a non-empty reason")
	}
}

func TestFileAdapter_MissingKeyIsCouldNotMeasure(t *testing.T) {
	a := NewFileAdapter("testdata/sessions.jsonl")
	m := a.Telemetry(PRKey{PRNumber: 999, MergeSHA: "nope", StreamTaskID: "quality/13"})
	if m.State != StateCouldNotMeasure {
		t.Fatalf("expected a key with no record to be could-not-measure, never zero: got %+v", m)
	}
	if m.Reason == "" {
		t.Fatal("could-not-measure requires a non-empty reason")
	}
}

func TestFileAdapter_MissingFileIsCouldNotMeasure(t *testing.T) {
	a := NewFileAdapter("testdata/does-not-exist.jsonl")
	if a.OpenError() == nil {
		t.Fatal("expected an open error for a missing file")
	}
	m := a.Telemetry(PRKey{PRNumber: 1, MergeSHA: "x", StreamTaskID: "quality/13"})
	if m.State != StateCouldNotMeasure {
		t.Fatalf("expected a missing file to yield could-not-measure, never zero: got %+v", m)
	}
	if m.Reason == "" {
		t.Fatal("could-not-measure requires a non-empty reason")
	}
}

func TestFileAdapter_MalformedLineDoesNotAbortTheLoad(t *testing.T) {
	// Confirms one unparseable line doesn't take down the whole file: the
	// planted fixture's other, well-formed keys still resolve.
	a := NewFileAdapter("testdata/malformed.jsonl")
	if len(a.LoadErrors()) == 0 {
		t.Fatal("expected the malformed line to be reported via LoadErrors")
	}
	m := a.Telemetry(PRKey{PRNumber: 1, MergeSHA: "onlygoodline", StreamTaskID: "quality/13"})
	if m.State != StateMeasured {
		t.Fatalf("expected the well-formed line to still resolve despite the malformed one, got %+v", m)
	}
}

func TestFileAdapter_ImplementsTelemetrySource(t *testing.T) {
	var _ TelemetrySource = NewFileAdapter("testdata/sessions.jsonl")
}
