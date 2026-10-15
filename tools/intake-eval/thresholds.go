package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Floors are the per-exit minimums. A nil floor is not checked.
type Floors struct {
	Precision *float64 `json:"precision,omitempty"`
	F1        *float64 `json:"f1,omitempty"`
}

// Thresholds is the verdict config (--thresholds). Unknown keys are rejected so
// a misspelt floor cannot silently disable a check.
type Thresholds struct {
	// Advisory reports the verdict but never fails the process.
	Advisory bool `json:"advisory"`
	// Unmeasured decides a checked metric that is undefined for this data:
	// "fail" (default) or "report".
	Unmeasured string `json:"unmeasured,omitempty"`
	// RegressionDelta bounds how far any exit's F1 may drop below its baseline.
	RegressionDelta *float64 `json:"regression_delta,omitempty"`
	// Baseline is a per-exit F1 baseline; --baseline <report.json> replaces it.
	Baseline map[string]*float64 `json:"baseline,omitempty"`
	// Exits holds per-exit precision and F1 floors.
	Exits map[string]Floors `json:"exits"`
}

const (
	unmeasuredFail   = "fail"
	unmeasuredReport = "report"
)

func inUnit(v *float64) bool { return v == nil || (*v >= 0 && *v <= 1) }

func decodeStrict(path string, v any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func loadThresholds(path string) (*Thresholds, error) {
	var t Thresholds
	if err := decodeStrict(path, &t); err != nil {
		return nil, fmt.Errorf("thresholds: %w", err)
	}
	switch t.Unmeasured {
	case "":
		t.Unmeasured = unmeasuredFail
	case unmeasuredFail, unmeasuredReport:
	default:
		return nil, fmt.Errorf("thresholds: unmeasured must be %q or %q, got %q", unmeasuredFail, unmeasuredReport, t.Unmeasured)
	}
	for e, f := range t.Exits {
		if !validExit(e) {
			return nil, fmt.Errorf("thresholds: exits names %q, not one of the five exits", e)
		}
		if !inUnit(f.Precision) || !inUnit(f.F1) {
			return nil, fmt.Errorf("thresholds: exit %q has a floor outside [0,1]", e)
		}
	}
	if t.RegressionDelta != nil && (*t.RegressionDelta < 0 || *t.RegressionDelta > 1) {
		return nil, errors.New("thresholds: regression_delta must be within [0,1]")
	}
	if err := checkBaseline(t.Baseline); err != nil {
		return nil, fmt.Errorf("thresholds: %w", err)
	}
	return &t, nil
}

func checkBaseline(b map[string]*float64) error {
	for e, v := range b {
		if !validExit(e) {
			return fmt.Errorf("baseline names %q, not one of the five exits", e)
		}
		if !inUnit(v) {
			return fmt.Errorf("baseline F1 for %q is outside [0,1]", e)
		}
	}
	return nil
}

// loadBaselineReport reads the per-exit F1 values out of a previous --json report.
func loadBaselineReport(path string) (map[string]*float64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("baseline: %w", err)
	}
	var r struct {
		Exits map[string]struct {
			F1 *float64 `json:"f1"`
		} `json:"exits"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("baseline %s: not a --json report: %w", path, err)
	}
	if len(r.Exits) == 0 {
		return nil, fmt.Errorf("baseline %s: no exits in report", path)
	}
	b := map[string]*float64{}
	for e, x := range r.Exits {
		b[e] = x.F1
	}
	if err := checkBaseline(b); err != nil {
		return nil, fmt.Errorf("baseline %s: %w", path, err)
	}
	return b, nil
}

// Check is one evaluated threshold.
type Check struct {
	Exit     string   `json:"exit"`
	Metric   string   `json:"metric"` // precision | f1
	Rule     string   `json:"rule"`   // floor | regression
	Floor    float64  `json:"floor"`
	Baseline *float64 `json:"baseline,omitempty"` // regression checks only
	Value    *float64 `json:"value"`
	Status   string   `json:"status"` // pass | fail | unmeasured
}

// Verdict is the threshold outcome.
type Verdict struct {
	Result     string  `json:"result"` // PASS | FAIL
	Advisory   bool    `json:"advisory"`
	Unmeasured string  `json:"unmeasured"`
	Checks     []Check `json:"checks"`
	Failures   int     `json:"failures"`
}

const eps = 1e-9

// evaluate applies the thresholds to a score. An unmeasured value counts as a
// failure under the default "fail" policy and is reported only under "report".
func evaluate(t *Thresholds, s *Score) (*Verdict, error) {
	if t.RegressionDelta != nil && len(t.Baseline) == 0 {
		return nil, errors.New("thresholds set regression_delta but no baseline was supplied (thresholds baseline or --baseline)")
	}
	if t.RegressionDelta == nil && len(t.Baseline) > 0 {
		return nil, errors.New("a baseline was supplied but thresholds set no regression_delta")
	}
	v := &Verdict{Result: "PASS", Advisory: t.Advisory, Unmeasured: t.Unmeasured, Checks: []Check{}}
	add := func(c Check) {
		switch {
		case c.Value == nil:
			c.Status = "unmeasured"
			if t.Unmeasured == unmeasuredFail {
				v.Failures++
			}
		case *c.Value+eps < c.Floor:
			c.Status = "fail"
			v.Failures++
		default:
			c.Status = "pass"
		}
		v.Checks = append(v.Checks, c)
	}
	for _, e := range exits {
		x := s.Exits[e]
		if f, ok := t.Exits[e]; ok {
			if f.Precision != nil {
				add(Check{Exit: e, Metric: "precision", Rule: "floor", Floor: *f.Precision, Value: x.Precision})
			}
			if f.F1 != nil {
				add(Check{Exit: e, Metric: "f1", Rule: "floor", Floor: *f.F1, Value: x.F1})
			}
		}
		if t.RegressionDelta != nil {
			// A baseline exit that was itself unmeasured has nothing to regress from.
			if b := t.Baseline[e]; b != nil {
				floor := *b - *t.RegressionDelta
				if floor < 0 {
					floor = 0
				}
				add(Check{Exit: e, Metric: "f1", Rule: "regression", Floor: floor, Baseline: b, Value: x.F1})
			}
		}
	}
	if v.Failures > 0 {
		v.Result = "FAIL"
	}
	return v, nil
}
