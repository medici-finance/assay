package consumers

import "testing"

func win(label string, churn, defect int) WindowSample {
	return WindowSample{Window: label, ChurnLines: churn, DefectInducing: defect}
}

// TestBudgets_RefusesUnderTwoWindows is Verify #4: a budget over a stream with
// fewer than two measured windows is refused as could-not-measure — NEVER armed
// at zero.
func TestBudgets_RefusesUnderTwoWindows(t *testing.T) {
	b := Budget{Stream: "quality", MaxChurn: 100, MaxDefectInducing: 2}

	// Zero windows.
	st0 := EvaluateBudget(b, StreamMeasurement{Stream: "quality"})
	if st0.Armed.State != StateCouldNotMeasure {
		t.Fatalf("0 windows: Armed must be could-not-measure, got %q", st0.Armed.State)
	}
	if st0.Armed.State == StateMeasuredZero || (st0.Armed.State == StateMeasured && !st0.Armed.Value) {
		t.Fatalf("0 windows must not be armed at zero")
	}
	if len(st0.Alarms) != 0 {
		t.Fatalf("unarmed budget must raise no alarm, got %d", len(st0.Alarms))
	}

	// One window: still under the arming threshold.
	st1 := EvaluateBudget(b, StreamMeasurement{Stream: "quality", Windows: []WindowSample{win("w1", 9999, 99)}})
	if st1.Armed.State != StateCouldNotMeasure {
		t.Fatalf("1 window: Armed must be could-not-measure, got %q", st1.Armed.State)
	}
	if len(st1.Alarms) != 0 {
		t.Fatalf("1 window breaching the ceiling must NOT alarm (budget unarmed), got %d", len(st1.Alarms))
	}
	if st1.ChurnBreach.State != StateCouldNotMeasure {
		t.Fatalf("1 window: ChurnBreach must be could-not-measure, got %q", st1.ChurnBreach.State)
	}
	if st1.Armed.Reason == "" {
		t.Fatalf("could-not-measure Armed must carry a reason")
	}
}

// TestBudgets_ArmsAtTwoWindowsAndAlarmsOnBreach: with two windows the budget
// arms and a breach in the latest window raises an ALARM record (not a
// dashboard line).
func TestBudgets_ArmsAtTwoWindowsAndAlarmsOnBreach(t *testing.T) {
	b := Budget{Stream: "quality", MaxChurn: 100, MaxDefectInducing: 2}
	m := StreamMeasurement{Stream: "quality", Windows: []WindowSample{
		win("w1", 50, 1),
		win("w2", 150, 5), // both dimensions breach
	}}
	st := EvaluateBudget(b, m)
	if st.Armed.State != StateMeasured || !st.Armed.Value {
		t.Fatalf("2 windows must arm, got state=%q value=%v", st.Armed.State, st.Armed.Value)
	}
	if st.ChurnBreach.State != StateMeasured || !st.ChurnBreach.Value {
		t.Fatalf("churn should breach: %+v", st.ChurnBreach)
	}
	if st.DefectBreach.State != StateMeasured || !st.DefectBreach.Value {
		t.Fatalf("defect should breach: %+v", st.DefectBreach)
	}
	if len(st.Alarms) != 2 {
		t.Fatalf("want 2 alarms (churn + defect), got %d: %+v", len(st.Alarms), st.Alarms)
	}
	for _, a := range st.Alarms {
		if a.Window != "w2" {
			t.Fatalf("alarm should be on the latest window w2, got %q", a.Window)
		}
		if a.Observed <= a.Budget {
			t.Fatalf("alarm observed %d must exceed budget %d", a.Observed, a.Budget)
		}
	}
}

// TestBudgets_ArmedWithinBudgetNoAlarm: armed and within budget raises nothing.
func TestBudgets_ArmedWithinBudgetNoAlarm(t *testing.T) {
	b := Budget{Stream: "quality", MaxChurn: 100, MaxDefectInducing: 2}
	m := StreamMeasurement{Stream: "quality", Windows: []WindowSample{
		win("w1", 10, 0),
		win("w2", 20, 1),
	}}
	st := EvaluateBudget(b, m)
	if st.Armed.State != StateMeasured || !st.Armed.Value {
		t.Fatalf("must be armed")
	}
	if st.ChurnBreach.Value || st.DefectBreach.Value {
		t.Fatalf("within budget must not breach")
	}
	if len(st.Alarms) != 0 {
		t.Fatalf("within budget must raise no alarm, got %d", len(st.Alarms))
	}
}

// TestBudgets_UnsetCeilingIsCouldNotMeasure: a dimension with no ceiling set (0)
// is could-not-measure once armed — nothing to enforce, not a false pass.
func TestBudgets_UnsetCeilingIsCouldNotMeasure(t *testing.T) {
	b := Budget{Stream: "quality", MaxChurn: 100} // no defect ceiling
	m := StreamMeasurement{Stream: "quality", Windows: []WindowSample{
		win("w1", 10, 100),
		win("w2", 20, 100),
	}}
	st := EvaluateBudget(b, m)
	if st.DefectBreach.State != StateCouldNotMeasure {
		t.Fatalf("unset defect ceiling must be could-not-measure, got %q", st.DefectBreach.State)
	}
	if st.ChurnBreach.State != StateMeasured {
		t.Fatalf("set churn ceiling must be measured, got %q", st.ChurnBreach.State)
	}
}
