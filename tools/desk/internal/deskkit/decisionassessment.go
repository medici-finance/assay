package deskkit

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"
)

// decisionassessment.go — the typed advice / policy-result envelope (graph-execution/10,
// GEA-04/07/09-11). `Decide` (decide.go) already carries a bounded, fail-closed
// vocabulary-consult primitive; this file adds the richer, CALIBRATED envelope a
// probabilistic provider (Laya or any other) fills in, and keeps it strictly separate
// from the DETERMINISTIC policy record that acts on it.
//
// Three products, never merged:
//
//   - AssessmentRequest — what is being assessed: subject identity, the exact input and
//     schema it was built against (as digests, never raw content) and the closed label
//     vocabulary a provider must answer within.
//   - Prediction — the probabilistic ADVICE: a label distribution (or an explicit
//     abstention), which labels the calibrator actually covers, provider/calibrator
//     identity, the actual backend used, budget usage and evidence references. It is
//     never itself an authorization — GEA-07: "confidence and action fields are
//     annotations, never a calibrated probability of safe execution".
//   - PolicyResult — a SEPARATE deterministic record. A policy result is never derived by
//     averaging or thresholding a Prediction inside this package; it is a distinct typed
//     value a caller's own policy code produces (GEA-11: preserve AssayScore,
//     AgenticAssessment and ControlAssurance as separate products, never one score).
//
// Existing `Advice`/`Decide` callers are preserved through an EXPLICIT projection
// (Prediction.ToAdvice, PredictionAdvisor) rather than a change to Decide itself: a
// provider that upgrades to the typed envelope plugs into the same bounded Consult flow
// — kill switch, budget, timeout, journal, fail-closed default — with zero change to that
// machinery. A malformed Prediction is never projected into an Advice; it is reported as
// an error so Decide's existing invalid/error path takes over.

// AssessmentRequest is the fixed envelope a Prediction answers against: a stable subject
// identity, the exact input and schema the request was built from (as digests — the
// content itself is never carried here, matching Consult.Context's digest-only
// journalling), and the closed set of labels a provider may name.
//
// RequiredCalibratorVersion is optional (empty = no calibration-version check declared):
// when set, a Prediction whose CalibratorVersion differs is inapplicable calibration
// (GEA-08: "a model/precision/schema/domain change invalidates calibration unless
// equivalence is proved") and is refused rather than silently accepted.
type AssessmentRequest struct {
	Subject                   string
	InputDigest               string
	SchemaDigest              string
	Vocabulary                []string
	RequiredCalibratorVersion string
}

// BudgetUsage is the provider's OWN report of how much of its declared budget a Prediction
// consumed. It is independent of, and in addition to, the caller-side Consult.Budget that
// already bounds how often the valve is consulted.
type BudgetUsage struct {
	Limit int
	Used  int
}

// PredictionNormalizationTolerance is the DECLARED rounding tolerance a Prediction's
// calibrated label probabilities must sum to 1 within (unless the prediction abstains,
// which carries no calibrated probabilities at all). Declared here, once, rather than
// left implicit in each caller's own epsilon.
const PredictionNormalizationTolerance = 1e-6

// Prediction is the probabilistic advice envelope. LabelProbabilities holds ONLY the
// labels the provider's calibrator actually covers; ShadowLabels names labels the
// provider produced without calibration support — GEA-07/08: "uncalibrated labels may be
// shadow-only; do not synthesize confidence". A label MUST NOT appear in both.
//
// Abstained is an EXPLICIT "no usable prediction" — distinct from a low-confidence guess.
// An abstained Prediction carries no calibrated probabilities.
type Prediction struct {
	Subject      string
	InputDigest  string
	SchemaDigest string

	LabelProbabilities map[string]float64
	ShadowLabels       []string
	Abstained          bool

	ProviderVersion   string
	CalibratorVersion string
	RequestedBackend  string
	ActualBackend     string

	Budget       BudgetUsage
	EvidenceRefs []string
	GeneratedAt  time.Time
}

// PolicyResult is the SEPARATE deterministic record: a policy's own disposition over a
// subject, never a probability and never derived inside this package from a Prediction's
// numbers. GEA-11 keeps this a distinct product from the probabilistic assessment above.
type PolicyResult struct {
	Subject       string
	InputDigest   string
	Decision      string
	PolicyVersion string
	Reason        string
	GeneratedAt   time.Time
}

// ValidatePrediction checks p against the envelope req declares. It returns the FIRST
// violation found (a *DeskError from Refused, exit 5 — malformed data, the same
// construction-time-refusal shape NewQuestion already uses), covering every rejection
// GEA-04/07 names:
//
//   - mismatched subject
//   - stale or wrong-subject input (the prediction's input/schema digest does not match
//     the request's current one)
//   - an unknown label (outside the declared vocabulary), in either LabelProbabilities or
//     ShadowLabels
//   - a NaN/Inf or out-of-[0,1] probability
//   - a label that is both calibrated and shadow-only (a synthesized confidence for a
//     label the calibrator does not cover)
//   - an abstained prediction that still carries calibrated probabilities
//   - calibrated probabilities that do not sum to 1 within PredictionNormalizationTolerance
//   - inapplicable calibration (CalibratorVersion mismatch, when req declares one)
//   - a self-reported budget overrun
//
// A nil error means p is well-formed against req — never a claim that its NUMBERS are
// accurate; calibration quality is graph-execution/12's concern, not this function's.
func ValidatePrediction(req AssessmentRequest, p Prediction) error {
	if p.Subject == "" || p.Subject != req.Subject {
		return Refused(fmt.Sprintf(
			"decisionassessment: prediction subject %q does not match request subject %q", p.Subject, req.Subject))
	}
	if req.InputDigest != "" && p.InputDigest != req.InputDigest {
		return Refused(fmt.Sprintf(
			"decisionassessment: prediction input digest %q does not match the request's current input digest %q — stale or wrong input",
			p.InputDigest, req.InputDigest))
	}
	if req.SchemaDigest != "" && p.SchemaDigest != req.SchemaDigest {
		return Refused(fmt.Sprintf(
			"decisionassessment: prediction schema digest %q does not match the request's schema digest %q",
			p.SchemaDigest, req.SchemaDigest))
	}

	vocab := make(map[string]bool, len(req.Vocabulary))
	for _, v := range req.Vocabulary {
		vocab[v] = true
	}

	calibrated := make(map[string]bool, len(p.LabelProbabilities))
	labels := make([]string, 0, len(p.LabelProbabilities))
	for label := range p.LabelProbabilities {
		labels = append(labels, label)
	}
	sort.Strings(labels) // deterministic error message on multiple violations
	for _, label := range labels {
		prob := p.LabelProbabilities[label]
		if !vocab[label] {
			return Refused(fmt.Sprintf(
				"decisionassessment: prediction carries unknown label %q outside the declared vocabulary", label))
		}
		if math.IsNaN(prob) || math.IsInf(prob, 0) {
			return Refused(fmt.Sprintf(
				"decisionassessment: prediction probability for label %q is NaN/Inf", label))
		}
		if prob < 0 || prob > 1 {
			return Refused(fmt.Sprintf(
				"decisionassessment: prediction probability for label %q is out of [0,1]: %v", label, prob))
		}
		calibrated[label] = true
	}

	shadow := append([]string(nil), p.ShadowLabels...)
	sort.Strings(shadow)
	for _, label := range shadow {
		if !vocab[label] {
			return Refused(fmt.Sprintf(
				"decisionassessment: shadow label %q is outside the declared vocabulary", label))
		}
		if calibrated[label] {
			return Refused(fmt.Sprintf(
				"decisionassessment: label %q is both calibrated and shadow-only — an uncalibrated label must never also carry a synthesized probability",
				label))
		}
	}

	if p.Abstained {
		if len(p.LabelProbabilities) != 0 {
			return Refused("decisionassessment: an abstained prediction must carry no calibrated label probabilities")
		}
	} else if len(p.LabelProbabilities) > 0 {
		var sum float64
		for _, prob := range p.LabelProbabilities {
			sum += prob
		}
		if math.Abs(sum-1) > PredictionNormalizationTolerance {
			return Refused(fmt.Sprintf(
				"decisionassessment: calibrated label probabilities sum to %v, want 1 within tolerance %v",
				sum, PredictionNormalizationTolerance))
		}
	}

	if req.RequiredCalibratorVersion != "" && len(p.LabelProbabilities) > 0 &&
		p.CalibratorVersion != req.RequiredCalibratorVersion {
		return Refused(fmt.Sprintf(
			"decisionassessment: prediction calibrator version %q is inapplicable — request requires %q",
			p.CalibratorVersion, req.RequiredCalibratorVersion))
	}

	if p.Budget.Limit > 0 && p.Budget.Used > p.Budget.Limit {
		return Refused(fmt.Sprintf(
			"decisionassessment: prediction reports budget used %d exceeding its own declared limit %d",
			p.Budget.Used, p.Budget.Limit))
	}

	return nil
}

// ConservativePrediction returns the explicit, always-valid abstention envelope for req:
// no calibrated label carries a probability, Abstained is true. Use it as the typed-
// envelope equivalent of Decide's fail-closed default — the value a caller falls back to
// on no-advisor, disabled valve, timeout, spent budget, or a malformed upstream Prediction
// — never a guessed label.
func ConservativePrediction(req AssessmentRequest) Prediction {
	return Prediction{
		Subject:      req.Subject,
		InputDigest:  req.InputDigest,
		SchemaDigest: req.SchemaDigest,
		Abstained:    true,
	}
}

// ToAdvice is the explicit projection that preserves existing Advice/Decide callers: it
// reports the highest-probability CALIBRATED label (ties broken by label name, for a
// deterministic answer) as an Advice, or ok=false when there is no calibrated answer to
// project (an explicit abstention, or a prediction carrying only shadow labels). Decide's
// own vocabulary check treats an empty/unreturned Answer as OutcomeInvalid and falls back
// to the pre-declared default — so "no calibrated answer" degrades exactly like any other
// malformed advice, never as a synthesized guess.
func (p Prediction) ToAdvice() (Advice, bool) {
	if p.Abstained || len(p.LabelProbabilities) == 0 {
		return Advice{}, false
	}
	labels := make([]string, 0, len(p.LabelProbabilities))
	for label := range p.LabelProbabilities {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	best := labels[0]
	bestProb := p.LabelProbabilities[best]
	for _, label := range labels[1:] {
		if prob := p.LabelProbabilities[label]; prob > bestProb {
			best, bestProb = label, prob
		}
	}
	just := fmt.Sprintf("assessment: label=%s p=%.4f provider=%s calibrator=%s backend=%s",
		best, bestProb, p.ProviderVersion, p.CalibratorVersion, p.ActualBackend)
	return Advice{Answer: best, Justification: StripControl(truncate(just, 280))}, true
}

// PredictionAdvisor adapts a richer probabilistic Predict func to the existing bounded
// Advisor contract Decide already enforces (vocabulary membership, timeout, budget, kill
// switch, journal). It is the "explicit projection" GEA-04 requires: a provider upgrades
// to the typed assessment envelope with ZERO change to Decide, Consult or any existing
// Advisor caller.
//
// Predict's returned Prediction is validated against Request before it is ever projected
// into an Advice. A malformed Prediction (unknown label, NaN/Inf or out-of-range
// probability, bad normalization, mismatched subject/digest, an uncalibrated label
// carrying a probability, or inapplicable calibration) is returned as an error, not as an
// Advice — Decide's existing OutcomeError path takes over and the pre-declared default is
// used. An explicit abstention (or a calibration-free prediction) returns ok=false from
// ToAdvice, which Decide's vocabulary check resolves to OutcomeInvalid, again falling to
// the default. Either way, a no-tools, no-write-credential, read-only provider can never
// produce anything other than a vocabulary member or a fall to the default.
type PredictionAdvisor struct {
	Request AssessmentRequest
	Predict func(ctx context.Context, c Consultation) (Prediction, error)
}

// Advise implements Advisor.
func (a PredictionAdvisor) Advise(ctx context.Context, c Consultation) (Advice, error) {
	if a.Predict == nil {
		return Advice{}, Unverifiable("decisionassessment: PredictionAdvisor has no Predict func wired", nil)
	}
	pred, err := a.Predict(ctx, c)
	if err != nil {
		return Advice{}, err
	}
	if verr := ValidatePrediction(a.Request, pred); verr != nil {
		return Advice{}, verr
	}
	advice, ok := pred.ToAdvice()
	if !ok {
		// Explicit abstention or calibration-free prediction: no answer to project.
		// Returning a nil error with a zero Advice lets Decide's own vocabulary check
		// (Answer not a member) resolve this to OutcomeInvalid -> default, rather than
		// this adapter inventing its own error class for "no opinion".
		return Advice{}, nil
	}
	return advice, nil
}
