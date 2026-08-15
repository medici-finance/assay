package main

// accuracy_test.go — MEASURING the classifier instead of asserting it.
//
// The brief's `why` cites "~20–25% of messages were pure relays" from a hand count.
// This classifier will disagree with a hand count sometimes, and a tool that reports a
// ratio without reporting how well it agrees with a human is asking to be believed
// rather than checked. So the score is MEASURED here, against a hand-labelled corpus,
// printed on every run, and floored — and the failure DIRECTION is asserted, not just
// the magnitude.
//
// THE DIRECTION MATTERS MORE THAN THE NUMBER. The floors below are set so that the
// classifier's false NEGATIVES (relays it calls substantive) outnumber its false
// POSITIVES. That makes the emitted relay ratio a FLOOR: the operator is doing at
// least this much plumbing, probably more. For a diagnostic nobody may use as a
// scorecard, erring toward flattering the subject is the safe direction — the opposite
// error would manufacture evidence against a person.

import (
	"encoding/json"
	"os"
	"testing"
)

type labelled struct {
	Session string `json:"session"`
	Text    string `json:"text"`
	Expect  string `json:"expect"`
}

// relayFloor / precisionFloor are the scores this classifier must not fall below.
// They are set just under the measured values so a REGRESSION fails while an
// improvement does not — the point is to catch the ruler getting worse, not to freeze
// it. Measured values are printed by the test; the README quotes them.
const (
	accuracyFloor  = 0.85
	precisionFloor = 0.88
	recallFloor    = 0.85
)

func loadCorpus(t *testing.T) []labelled {
	t.Helper()
	raw, err := os.ReadFile("testdata/labelled/corpus.json")
	if err != nil {
		t.Fatalf("cannot read the labelled corpus: %v", err)
	}
	var out []labelled
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("labelled corpus is not valid JSON: %v", err)
	}
	if len(out) < 40 {
		t.Fatalf("labelled corpus has only %d entries — too few to measure anything", len(out))
	}
	return out
}

func TestClassifierAccuracyAgainstLabelledCorpus(t *testing.T) {
	corpus := loadCorpus(t)
	c := NewClassifier()

	var tp, fp, tn, fn int
	for _, e := range corpus {
		got := c.Classify(e.Session, e.Text)
		predictedRelay := got.Class == ClassRelay
		actualRelay := e.Expect == "relay"
		switch {
		case predictedRelay && actualRelay:
			tp++
		case predictedRelay && !actualRelay:
			fp++
			t.Logf("false positive (called relay, labelled substantive): %q", e.Text)
		case !predictedRelay && actualRelay:
			fn++
			t.Logf("false negative (called substantive, labelled relay): %q", e.Text)
		default:
			tn++
		}
	}

	n := float64(len(corpus))
	accuracy := float64(tp+tn) / n
	precision := float64(tp) / float64(tp+fp)
	recall := float64(tp) / float64(tp+fn)

	t.Logf("classifier=%s corpus=%d accuracy=%.4f relay-precision=%.4f relay-recall=%.4f (tp=%d fp=%d tn=%d fn=%d)",
		ClassifierVersion, len(corpus), accuracy, precision, recall, tp, fp, tn, fn)

	if accuracy < accuracyFloor {
		t.Errorf("accuracy %.4f below floor %.2f", accuracy, accuracyFloor)
	}
	if precision < precisionFloor {
		t.Errorf("relay precision %.4f below floor %.2f", precision, precisionFloor)
	}
	if recall < recallFloor {
		t.Errorf("relay recall %.4f below floor %.2f", recall, recallFloor)
	}

	// THE DIRECTION ASSERTION. More missed relays than invented ones ⇒ the emitted
	// ratio under-states the operator's plumbing load, i.e. it is a floor. If this
	// ever inverts, the tool has started manufacturing relays and the README's claim
	// ("the ratio is a floor") is no longer true — which is a documentation bug as
	// much as a code one, so the message says both.
	if fn <= fp {
		t.Errorf("failure direction inverted: fn=%d fp=%d. The classifier now OVER-counts relays, "+
			"so the emitted ratio is no longer a floor. Either restore the conservative bias or "+
			"change the README's stated direction — they must agree", fn, fp)
	}
}

// TestClassifierVersionIsPinnedToItsScore is the anti-drift pin. The version string
// is only worth emitting if it actually changes when the rules change; this test
// records the version the floors above were measured against, so a rule change that
// moves the score without bumping the version fails here.
func TestClassifierVersionIsPinnedToItsScore(t *testing.T) {
	const measuredAgainst = "opmetrics-relay/1"
	if ClassifierVersion != measuredAgainst {
		t.Fatalf("ClassifierVersion is %q but the accuracy floors in this file were measured "+
			"against %q. Re-measure against the labelled corpus, update the floors AND the README "+
			"table, then update this constant — in the same commit", ClassifierVersion, measuredAgainst)
	}
}
