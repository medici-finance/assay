package deskkit

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
)

func sampleRequest() AssessmentRequest {
	return AssessmentRequest{
		Subject:      "example-org/agents#1@rev-1",
		InputDigest:  "sha256:input-a",
		SchemaDigest: "sha256:schema-a",
		Vocabulary:   []string{"REBASELINE", "REGRESSION", "RETRY", "ESCALATE"},
	}
}

func samplePrediction(req AssessmentRequest) Prediction {
	return Prediction{
		Subject:      req.Subject,
		InputDigest:  req.InputDigest,
		SchemaDigest: req.SchemaDigest,
		LabelProbabilities: map[string]float64{
			"REGRESSION": 0.7,
			"RETRY":      0.2,
			"ESCALATE":   0.1,
		},
		ProviderVersion:   "laya-0.1.0",
		CalibratorVersion: "cal-2026-09-01",
		RequestedBackend:  "cpu",
		ActualBackend:     "cpu",
		Budget:            BudgetUsage{Limit: 10, Used: 1},
		EvidenceRefs:      []string{"evidence://run-42"},
	}
}

// TestDecisionAssessment — the happy path: a well-formed request/prediction pair
// validates cleanly and projects to the highest-probability calibrated label.
func TestDecisionAssessment(t *testing.T) {
	req := sampleRequest()
	pred := samplePrediction(req)

	if err := ValidatePrediction(req, pred); err != nil {
		t.Fatalf("ValidatePrediction: unexpected error on a well-formed prediction: %v", err)
	}

	advice, ok := pred.ToAdvice()
	if !ok {
		t.Fatalf("ToAdvice: ok=false on a well-formed calibrated prediction")
	}
	if advice.Answer != "REGRESSION" {
		t.Fatalf("advice.Answer = %q, want REGRESSION (highest probability 0.7)", advice.Answer)
	}
	if !strings.Contains(advice.Justification, "REGRESSION") {
		t.Fatalf("justification %q does not name the chosen label", advice.Justification)
	}

	// An explicit abstention and a shadow-only prediction both project to "no advice".
	abst := ConservativePrediction(req)
	if err := ValidatePrediction(req, abst); err != nil {
		t.Fatalf("ValidatePrediction on ConservativePrediction: %v", err)
	}
	if _, ok := abst.ToAdvice(); ok {
		t.Fatalf("ToAdvice: ok=true on an abstained prediction")
	}

	shadowOnly := Prediction{
		Subject: req.Subject, InputDigest: req.InputDigest, SchemaDigest: req.SchemaDigest,
		ShadowLabels: []string{"RETRY"},
	}
	if err := ValidatePrediction(req, shadowOnly); err != nil {
		t.Fatalf("ValidatePrediction on a shadow-only prediction: %v", err)
	}
	if _, ok := shadowOnly.ToAdvice(); ok {
		t.Fatalf("ToAdvice: ok=true on a shadow-only (uncalibrated) prediction")
	}
}

// TestDecisionAssessmentMalformedDefaults — every rejection class GEA-04/07 names is
// caught by ValidatePrediction, one mutation of the well-formed prediction at a time. Each
// subtest is the "fail-first" proof for its own guard: starting from samplePrediction
// (which TestDecisionAssessment already proves passes clean), exactly one field is
// mutated away from well-formed, and the mutation MUST be caught.
func TestDecisionAssessmentMalformedDefaults(t *testing.T) {
	req := sampleRequest()

	cases := []struct {
		name   string
		mutate func(Prediction) Prediction
	}{
		{
			name: "unknown label",
			mutate: func(p Prediction) Prediction {
				p.LabelProbabilities = map[string]float64{"NOT-IN-VOCAB": 1.0}
				return p
			},
		},
		{
			name: "unknown shadow label",
			mutate: func(p Prediction) Prediction {
				p.LabelProbabilities = nil
				p.ShadowLabels = []string{"NOT-IN-VOCAB"}
				return p
			},
		},
		{
			name: "NaN probability",
			mutate: func(p Prediction) Prediction {
				p.LabelProbabilities = map[string]float64{"REGRESSION": math.NaN(), "RETRY": 1.0}
				return p
			},
		},
		{
			name: "infinite probability",
			mutate: func(p Prediction) Prediction {
				p.LabelProbabilities = map[string]float64{"REGRESSION": math.Inf(1)}
				return p
			},
		},
		{
			name: "out-of-range probability",
			mutate: func(p Prediction) Prediction {
				p.LabelProbabilities = map[string]float64{"REGRESSION": 1.5, "RETRY": -0.5}
				return p
			},
		},
		{
			name: "invalid normalization beyond declared tolerance",
			mutate: func(p Prediction) Prediction {
				p.LabelProbabilities = map[string]float64{"REGRESSION": 0.5, "RETRY": 0.2} // sums to 0.7
				return p
			},
		},
		{
			name: "mismatched subject",
			mutate: func(p Prediction) Prediction {
				p.Subject = "example-org/agents#999@rev-1"
				return p
			},
		},
		{
			name: "stale input digest",
			mutate: func(p Prediction) Prediction {
				p.InputDigest = "sha256:input-STALE"
				return p
			},
		},
		{
			name: "mismatched schema digest",
			mutate: func(p Prediction) Prediction {
				p.SchemaDigest = "sha256:schema-STALE"
				return p
			},
		},
		{
			name: "uncalibrated label carries a synthesized probability",
			mutate: func(p Prediction) Prediction {
				p.ShadowLabels = []string{"REGRESSION"} // REGRESSION is already calibrated above
				return p
			},
		},
		{
			name: "abstained prediction still carries probabilities",
			mutate: func(p Prediction) Prediction {
				p.Abstained = true
				return p
			},
		},
		{
			name: "self-reported budget overrun",
			mutate: func(p Prediction) Prediction {
				p.Budget = BudgetUsage{Limit: 1, Used: 2}
				return p
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pred := tc.mutate(samplePrediction(req))
			if err := ValidatePrediction(req, pred); err == nil {
				t.Fatalf("ValidatePrediction accepted a malformed prediction (%s)", tc.name)
			} else if !IsRefused(err) {
				t.Fatalf("error = %v, want a Refused (exit 5) for %s", err, tc.name)
			}
		})
	}

	// Inapplicable calibration only fires when the request DECLARES a required version.
	t.Run("inapplicable calibration", func(t *testing.T) {
		strictReq := req
		strictReq.RequiredCalibratorVersion = "cal-2026-10-01"
		pred := samplePrediction(req) // carries cal-2026-09-01
		if err := ValidatePrediction(strictReq, pred); err == nil {
			t.Fatalf("ValidatePrediction accepted a calibrator-version mismatch the request declared")
		} else if !IsRefused(err) {
			t.Fatalf("error = %v, want a Refused (exit 5)", err)
		}
		// The SAME prediction is fine when the request declares no requirement.
		if err := ValidatePrediction(req, pred); err != nil {
			t.Fatalf("ValidatePrediction rejected a prediction the (undeclared) request never constrained: %v", err)
		}
	})
}

// TestDecisionAssessmentDecideJournal — the flow row: production Decide/NewQuestion code,
// through PredictionAdvisor, proves malformed and unjournallable advice both resolve to
// the pre-declared default and are journalled correctly, and a well-formed prediction is
// advised and journalled as such. This calls REAL Question.Decide, not a hand-built
// expected record.
func TestDecisionAssessmentDecideJournal(t *testing.T) {
	t.Setenv(decideDisabledEnv, "")
	req := sampleRequest()
	q, err := NewQuestion("Classify this verify FAIL", req.Vocabulary, "ESCALATE")
	if err != nil {
		t.Fatalf("NewQuestion: %v", err)
	}

	t.Run("well-formed prediction is advised and journalled", func(t *testing.T) {
		j := &recordingJournal{}
		advisor := PredictionAdvisor{
			Request: req,
			Predict: func(ctx context.Context, c Consultation) (Prediction, error) {
				return samplePrediction(req), nil
			},
		}
		got, err := q.Decide(context.Background(), Consult{Item: req.Subject, Advisor: advisor, Journal: j})
		if err != nil {
			t.Fatalf("Decide error: %v", err)
		}
		if got != "REGRESSION" {
			t.Fatalf("Decide answer = %q, want REGRESSION", got)
		}
		rec, ok := j.last()
		if !ok {
			t.Fatalf("no journal record written")
		}
		if rec.Outcome != OutcomeAdvised {
			t.Fatalf("outcome = %q, want %q", rec.Outcome, OutcomeAdvised)
		}
		if rec.Answer != "REGRESSION" {
			t.Fatalf("journalled answer = %q, want REGRESSION", rec.Answer)
		}
	})

	t.Run("malformed prediction falls to the default, never advised", func(t *testing.T) {
		j := &recordingJournal{}
		advisor := PredictionAdvisor{
			Request: req,
			Predict: func(ctx context.Context, c Consultation) (Prediction, error) {
				bad := samplePrediction(req)
				bad.LabelProbabilities = map[string]float64{"NOT-IN-VOCAB": 1.0} // unknown label
				return bad, nil
			},
		}
		got, err := q.Decide(context.Background(), Consult{Item: req.Subject, Advisor: advisor, Journal: j})
		if err != nil {
			t.Fatalf("Decide error: %v", err)
		}
		if got != q.Default() {
			t.Fatalf("Decide answer = %q, want default %q for a malformed prediction", got, q.Default())
		}
		rec, ok := j.last()
		if !ok {
			t.Fatalf("no journal record written")
		}
		if rec.Outcome != OutcomeError {
			t.Fatalf("outcome = %q, want %q (malformed prediction surfaces as an advisor error)", rec.Outcome, OutcomeError)
		}
		if rec.Answer != q.Default() {
			t.Fatalf("journalled answer = %q, want default", rec.Answer)
		}
	})

	t.Run("abstained prediction resolves to the default via the vocabulary check", func(t *testing.T) {
		j := &recordingJournal{}
		advisor := PredictionAdvisor{
			Request: req,
			Predict: func(ctx context.Context, c Consultation) (Prediction, error) {
				return ConservativePrediction(req), nil
			},
		}
		got, err := q.Decide(context.Background(), Consult{Item: req.Subject, Advisor: advisor, Journal: j})
		if err != nil {
			t.Fatalf("Decide error: %v", err)
		}
		if got != q.Default() {
			t.Fatalf("Decide answer = %q, want default %q for an abstention", got, q.Default())
		}
		rec, ok := j.last()
		if !ok {
			t.Fatalf("no journal record written")
		}
		if rec.Outcome != OutcomeInvalid {
			t.Fatalf("outcome = %q, want %q (abstention has no vocabulary member to advise)", rec.Outcome, OutcomeInvalid)
		}
	})

	t.Run("advised answer discarded when it cannot be journalled", func(t *testing.T) {
		j := &recordingJournal{fail: errors.New("disk full")}
		advisor := PredictionAdvisor{
			Request: req,
			Predict: func(ctx context.Context, c Consultation) (Prediction, error) {
				return samplePrediction(req), nil
			},
		}
		got, err := q.Decide(context.Background(), Consult{Item: req.Subject, Advisor: advisor, Journal: j})
		if err != nil {
			t.Fatalf("Decide error: %v", err)
		}
		if got != q.Default() {
			t.Fatalf("Decide answer = %q, want default %q when advice cannot be journalled", got, q.Default())
		}
	})
}
