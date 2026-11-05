package portfolio

import "math/big"

// Divergence states.
const (
	Aligned   = "aligned"
	Divergent = "divergent"
	Undefined = "undefined"
)

// Divergence compares one bucket's observed share with its policy target. It is advisory:
// it carries no effect, and neither the target nor any dispatch follows from it.
type Divergence struct {
	Bucket      string `json:"bucket"`
	Party       string `json:"party"`
	Measure     string `json:"measure"`
	TargetBasis int64  `json:"target-basis-points"`
	Observed    Share  `json:"observed"`
	State       string `json:"state"`
}

// LimitStatus reports a bucket's known subtotal against an already-authorized limit.
// "exceeded" needs only the known subtotal; "within" needs a complete measure.
type LimitStatus struct {
	Limit Limit    `json:"limit"`
	Value Quantity `json:"value"`
	State string   `json:"state"`
}

// Comparison is the advisory result of comparing a ledger with the policy it measured.
type Comparison struct {
	Policy         string        `json:"policy"`
	PolicyRevision string        `json:"policy-revision"`
	Divergences    []Divergence  `json:"divergences"`
	Limits         []LimitStatus `json:"limits"`
}

// Compare reports, for every bucket, how observed agent and human shares stand against the
// policy's independent targets, and each limit against its known subtotal. It returns
// values only: the policy is read, never rewritten, and nothing is dispatched.
// semantic: S-portfolio-measurement
func Compare(p Policy, l Ledger) Comparison {
	c := Comparison{Policy: p.ID, PolicyRevision: p.Revision, Divergences: []Divergence{}, Limits: []LimitStatus{}}
	for _, b := range p.Buckets {
		c.Divergences = append(c.Divergences,
			diverge(b.ID, "agent", b.AgentTarget, p.Tolerance, l.ShareOf(b.ID, true, b.AgentTarget.Measure)),
			diverge(b.ID, "human", b.HumanTarget, p.Tolerance, l.ShareOf(b.ID, false, b.HumanTarget.Measure)))
	}
	for _, lim := range p.Limits {
		t := l.Buckets[lim.Bucket]
		q, ok := t.Agent[lim.Measure]
		if !ok {
			q = t.Human[lim.Measure]
		}
		state := "unknown"
		switch {
		case q.Known > lim.Max:
			state = "exceeded"
		case q.Complete():
			state = "within"
		}
		c.Limits = append(c.Limits, LimitStatus{Limit: lim, Value: q, State: state})
	}
	return c
}

func diverge(bucket, party string, t Target, tolerance int64, observed Share) Divergence {
	d := Divergence{Bucket: bucket, Party: party, Measure: t.Measure, TargetBasis: t.BasisPoints, Observed: observed, State: Undefined}
	if !observed.Defined {
		return d
	}
	want := t.BasisPoints
	// |numerator*10000 - want*denominator| <= tolerance*denominator, exactly.
	den := big.NewInt(observed.Denominator)
	gap := new(big.Int).Mul(big.NewInt(observed.Numerator), big.NewInt(10000))
	gap.Sub(gap, new(big.Int).Mul(big.NewInt(want), den))
	gap.Abs(gap)
	if gap.Cmp(new(big.Int).Mul(big.NewInt(tolerance), den)) <= 0 {
		d.State = Aligned
	} else {
		d.State = Divergent
	}
	return d
}
