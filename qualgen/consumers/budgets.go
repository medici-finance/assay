package consumers

import "fmt"

// budgets.go implements per-stream quality error-budgets in an ALARM posture
// (spec §9.6). A budget is a ceiling on a stream's churn and on its
// defect-inducing changes per window; a BREACH raises an alarm record, not a
// dashboard line — the whole point is that a breach is acted on, not merely
// displayed.
//
// The load-bearing invariant: a budget is armable only after the stream has ≥2
// MEASURED windows (spec §9.6). Before that the budget status is
// could-not-measure — NOT a budget armed at zero. Arming at zero would raise a
// false alarm on the very first window; three-state honesty (spec §3.2) reports
// "not enough history to arm" as itself.

// MinWindowsToArm is the minimum number of measured windows a stream needs
// before a budget can be armed and enforced (spec §9.6).
const MinWindowsToArm = 2

// WindowSample is one measured window's readings for a stream. Both counts are
// the genuine measured values for that window; a window with no data is simply
// not in the slice (its absence is what keeps the count honest, rather than a
// zero-filled placeholder).
type WindowSample struct {
	// Window is the window label (e.g. an ISO week), for the record only.
	Window string
	// ChurnLines is the stream's churn (added+deleted lines) in the window.
	ChurnLines int
	// DefectInducing is the count of defect-inducing changes attributed to the
	// stream in the window (the M2/M3 defect-inducing family).
	DefectInducing int
}

// StreamMeasurement is a stream's measured history — the windows a budget is
// evaluated against.
type StreamMeasurement struct {
	Stream  string
	Windows []WindowSample
}

// Budget is a per-stream ceiling. A zero ceiling means "no budget set for this
// dimension" and that dimension is not evaluated; a positive ceiling is
// enforced once the stream is armable.
type Budget struct {
	Stream string
	// MaxChurn is the per-window churn ceiling; 0 means unset (not evaluated).
	MaxChurn int
	// MaxDefectInducing is the per-window defect-inducing ceiling; 0 means unset.
	MaxDefectInducing int
}

// Alarm is a single budget breach — the actionable record a breach produces
// (spec §9.6: an alarm, not a dashboard line).
type Alarm struct {
	Stream string `json:"stream"`
	// Kind is "churn" or "defect-inducing".
	Kind string `json:"kind"`
	// Window is the breaching window's label.
	Window string `json:"window"`
	// Budget and Observed are the ceiling and the measured value that breached it.
	Budget   int    `json:"budget"`
	Observed int    `json:"observed"`
	Message  string `json:"message"`
}

// BudgetStatus is the evaluation of one stream's budget. Armed is three-state:
// could-not-measure when the stream has <2 measured windows (never armed at
// zero), measured-true once armed. The breach flags are meaningful only when
// Armed is measured-true; before arming they too are could-not-measure.
type BudgetStatus struct {
	Stream string `json:"stream"`
	// Windows is the number of measured windows seen (for the record and audit).
	Windows int `json:"windows"`
	// Armed reports whether the budget is armed and enforced. It is
	// could-not-measure with a reason until MinWindowsToArm windows exist.
	Armed Measure[bool] `json:"armed"`
	// ChurnBreach / DefectBreach are measured-true/false once armed; each is
	// could-not-measure while unarmed OR when its ceiling is unset (0).
	ChurnBreach  Measure[bool] `json:"churn_breach"`
	DefectBreach Measure[bool] `json:"defect_breach"`
	// Alarms is one entry per breached dimension in the evaluated (latest)
	// window. Empty when armed and within budget, and always empty while unarmed.
	Alarms []Alarm `json:"alarms,omitempty"`
}

// EvaluateBudget evaluates a stream's budget against its measured history.
//
// With fewer than MinWindowsToArm measured windows the budget is REFUSED —
// Armed is could-not-measure, the breach flags are could-not-measure, and no
// alarm is raised (spec §9.6). With enough history the budget is armed and the
// LATEST window is checked against each set ceiling; a breach appends an Alarm.
func EvaluateBudget(b Budget, m StreamMeasurement) BudgetStatus {
	n := len(m.Windows)
	st := BudgetStatus{Stream: b.Stream, Windows: n}

	if n < MinWindowsToArm {
		reason := fmt.Sprintf(
			"stream %q has %d measured window(s); a budget is armable only after %d (spec §9.6) — not armed at zero",
			b.Stream, n, MinWindowsToArm,
		)
		st.Armed = CouldNotMeasure[bool](reason)
		st.ChurnBreach = CouldNotMeasure[bool](reason)
		st.DefectBreach = CouldNotMeasure[bool](reason)
		return st
	}

	st.Armed = Measured(true)
	latest := m.Windows[n-1]

	st.ChurnBreach = evalDimension(&st, b.Stream, "churn", latest.Window, b.MaxChurn, latest.ChurnLines)
	st.DefectBreach = evalDimension(&st, b.Stream, "defect-inducing", latest.Window, b.MaxDefectInducing, latest.DefectInducing)

	return st
}

// evalDimension evaluates one budgeted dimension for the latest window. An unset
// ceiling (0) is could-not-measure (nothing to enforce). A set ceiling yields a
// measured true/false breach flag and, on a breach, appends an Alarm to st.
func evalDimension(st *BudgetStatus, stream, kind, window string, ceiling, observed int) Measure[bool] {
	if ceiling <= 0 {
		return CouldNotMeasure[bool](fmt.Sprintf("no %s ceiling set for stream %q", kind, stream))
	}
	breached := observed > ceiling
	if breached {
		st.Alarms = append(st.Alarms, Alarm{
			Stream:   stream,
			Kind:     kind,
			Window:   window,
			Budget:   ceiling,
			Observed: observed,
			Message: fmt.Sprintf(
				"ALARM: stream %q %s budget breached in window %q — observed %d > budget %d",
				stream, kind, window, observed, ceiling,
			),
		})
	}
	return Measured(breached)
}
