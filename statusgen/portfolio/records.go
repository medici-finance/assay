// Package portfolio holds the portfolio-measurement-v1 records: a versioned allocation
// policy and the read-only agent/human capacity ledger measured against it.
//
// The package is pure. It reads no file, clock, network or environment, writes nothing and
// dispatches nothing: every input is a supplied value and every output is a returned value.
// Adapters that read snapshots from disk or collect runner reports live outside it. It ships
// no default weight, share, tolerance or limit; a measure the inputs do not support is
// reported unknown, never filled in.
package portfolio

// Schema is the only wire version this package evaluates.
const Schema = "portfolio-measurement/v1"

// Agent measures. Weighted active time is active seconds times the policy's per-model weight
// in thousandths. The elapsed-time proxy is a separate measure: it never joins active time
// and no target may name it.
const (
	ActiveSeconds   = "active-seconds"
	WeightedActive  = "weighted-active-milliseconds"
	CostMicros      = "cost-micros"
	ElapsedProxy    = "elapsed-seconds-proxy"
	Decisions       = "decisions"
	DecisionMinutes = "decision-minutes"
)

// Human decision units. A decision counted in "decision" units contributes only to the
// decisions measure; one declared in "minute" units contributes only to decision-minutes.
// Counts are not time.
const (
	UnitDecision = "decision"
	UnitMinute   = "minute"
)

var (
	agentMeasures = []string{ActiveSeconds, WeightedActive, CostMicros}
	humanMeasures = []string{Decisions, DecisionMinutes}
)

// Snapshot is one portfolio-measurement-v1 document: the policy in force and the
// measurements reported for its scope and window. Reconciliations are explicit choices
// between conflicting segment reports.
type Snapshot struct {
	Schema          string           `json:"schema"`
	Policy          Policy           `json:"policy"`
	Segments        []Segment        `json:"segments"`
	Decisions       []Decision       `json:"decisions"`
	Reconciliations []Reconciliation `json:"reconciliations,omitempty"`
}

// RequiredKeys names the members a document must carry, so an absent list is refused
// rather than read as a proven empty one.
func (Snapshot) RequiredKeys() []string {
	return []string{"schema", "policy", "segments", "decisions"}
}

// Policy is one revision of an allocation policy. Agent and human targets are independent:
// neither is derived from the other, and neither is derived from consumption.
type Policy struct {
	ID       string       `json:"id"`
	Revision string       `json:"revision"`
	Scope    string       `json:"scope"`
	Window   Interval     `json:"window"`
	Sources  []string     `json:"sources"`
	Weights  WeightPolicy `json:"weights"`
	// Tolerance is the explicit band, in basis points, within which an observed share is
	// aligned with its target. There is no default.
	Tolerance int64    `json:"tolerance-basis-points"`
	Buckets   []Bucket `json:"buckets"`
	Limits    []Limit  `json:"limits,omitempty"`
}

func (Policy) RequiredKeys() []string {
	return []string{"id", "revision", "scope", "window", "sources", "weights", "tolerance-basis-points", "buckets"}
}

// WeightPolicy is the explicit per-model weight table. A model it does not list has an
// unknown weight; nothing is assumed.
type WeightPolicy struct {
	ID       string        `json:"id"`
	Revision string        `json:"revision"`
	Models   []ModelWeight `json:"models"`
}

func (WeightPolicy) RequiredKeys() []string { return []string{"id", "revision", "models"} }

// ModelWeight is one model's weight in thousandths (1000 = 1.0).
type ModelWeight struct {
	Model string `json:"model"`
	Milli int64  `json:"milli"`
}

func (ModelWeight) RequiredKeys() []string { return []string{"model", "milli"} }

// Bucket groups qualified REQ subjects. A subject belongs to at most one bucket.
type Bucket struct {
	ID          string    `json:"id"`
	Members     []Subject `json:"members"`
	AgentTarget Target    `json:"agent-target"`
	HumanTarget Target    `json:"human-target"`
}

func (Bucket) RequiredKeys() []string {
	return []string{"id", "members", "agent-target", "human-target"}
}

// Target is a desired share of one measure, in basis points of the scope's total.
type Target struct {
	Measure     string `json:"measure"`
	BasisPoints int64  `json:"basis-points"`
}

func (Target) RequiredKeys() []string { return []string{"measure", "basis-points"} }

// Limit is an already-authorized ceiling on one bucket's measure. It is reported against,
// never enforced here.
type Limit struct {
	Bucket        string `json:"bucket"`
	Measure       string `json:"measure"`
	Max           int64  `json:"max"`
	Authorization string `json:"authorization"`
}

func (Limit) RequiredKeys() []string { return []string{"bucket", "measure", "max", "authorization"} }

// Subject is a repository-qualified REQ reference.
type Subject struct {
	Repository  string `json:"repository"`
	Requirement string `json:"requirement"`
}

func (Subject) RequiredKeys() []string { return []string{"repository", "requirement"} }

// Interval is a half-open UTC interval [Start, End) in whole seconds.
type Interval struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

func (Interval) RequiredKeys() []string { return []string{"start", "end"} }

// Invocation is the runner's stable invocation identity. A retry is a new invocation.
type Invocation struct {
	Caller string `json:"caller"`
	ID     string `json:"id"`
}

func (Invocation) RequiredKeys() []string { return []string{"caller", "id"} }

// Profile is the launch profile a segment ran under: its id, the revision of that profile
// and the model it pinned. The pinned model is not the actual model.
type Profile struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Model    string `json:"model"`
}

func (Profile) RequiredKeys() []string { return []string{"id", "revision", "model"} }

// Segment is one reported span of agent work. (Invocation, ID) is its stable identity and
// Revision the revision of that report. An absent Active, ActualModel or Cost is unknown.
type Segment struct {
	Invocation  Invocation `json:"invocation"`
	ID          string     `json:"id"`
	Revision    string     `json:"revision"`
	Profile     Profile    `json:"profile"`
	ActualModel string     `json:"actual-model,omitempty"`
	Source      string     `json:"source"`
	Subject     *Subject   `json:"subject,omitempty"`
	Active      *Interval  `json:"active,omitempty"`
	Elapsed     *int64     `json:"elapsed-seconds-proxy,omitempty"`
	Cost        *int64     `json:"cost-micros,omitempty"`
}

func (Segment) RequiredKeys() []string {
	return []string{"invocation", "id", "revision", "profile", "source"}
}

// Decision is one human decision record. ID is its stable dedupe identity. A "decision"
// unit carries no quantity (it counts one); a "minute" unit carries its declared minutes.
type Decision struct {
	ID       string   `json:"id"`
	Unit     string   `json:"unit"`
	Quantity *int64   `json:"quantity,omitempty"`
	Window   Interval `json:"window"`
	Primary  *Subject `json:"primary-subject,omitempty"`
	Source   string   `json:"source"`
}

func (Decision) RequiredKeys() []string { return []string{"id", "unit", "window", "source"} }

// SegmentRef names one revision of one segment of an invocation.
type SegmentRef struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}

func (SegmentRef) RequiredKeys() []string { return []string{"id", "revision"} }

// Reconciliation is an explicit choice, for one invocation, of which conflicting segment
// reports count. Reports of that invocation it does not accept are excluded and listed.
type Reconciliation struct {
	Invocation Invocation   `json:"invocation"`
	Accept     []SegmentRef `json:"accept"`
	Reason     string       `json:"reason"`
	Source     string       `json:"source"`
}

func (Reconciliation) RequiredKeys() []string {
	return []string{"invocation", "accept", "reason", "source"}
}
