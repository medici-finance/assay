// Package productintent owns the optional REQUIREMENTS product-intent-v1 extension.
// It evaluates caller-supplied snapshots only. It grants no decision or effect authority.
package productintent

import "encoding/json"

const Schema = "product-intent-v1"

type Subject struct {
	Repository  string `json:"repository"`
	Requirement string `json:"requirement"`
}

// Reference pins an external record's kind, repository, identity and revision.
// Operational definitions and assessment contents remain with their own owners.
type Reference struct {
	Repository string `json:"repository"`
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Revision   string `json:"revision"`
}

type Direction struct {
	Revision  string    `json:"revision"`
	Rationale string    `json:"rationale"`
	Owner     string    `json:"owner"`
	Decision  Reference `json:"decision"`
}

type Target struct {
	Revision          string    `json:"revision"`
	DirectionRevision string    `json:"direction-revision"`
	Condition         string    `json:"condition"`
	Measure           string    `json:"measure"`
	Unit              string    `json:"unit"`
	Window            string    `json:"window"`
	Confidence        string    `json:"confidence"`
	Basis             string    `json:"basis"`
	Decision          Reference `json:"decision"`
	// Approved is retained as an author's claim, never consumed as authority.
	Approved *bool `json:"approved,omitempty"`
}

type Observation struct {
	TargetRevision string    `json:"target-revision"`
	Assessment     Reference `json:"assessment"`
	Source         string    `json:"source"`
	AttributedTo   string    `json:"attributed-to"`
	Date           string    `json:"date"`
	Window         string    `json:"window"`
	Confidence     string    `json:"confidence"`
	Limitations    string    `json:"limitations"`
}

type Record struct {
	Schema                string        `json:"schema"`
	Mandatory             bool          `json:"mandatory,omitempty"`
	Subject               Subject       `json:"subject"`
	Kind                  string        `json:"kind"`
	Owner                 string        `json:"owner"`
	Status                string        `json:"status"`
	Directions            []Direction   `json:"directions"`
	Targets               []Target      `json:"targets"`
	Observed              []Observation `json:"observed,omitempty"`
	OperationalReferences []Reference   `json:"operational-references,omitempty"`
}

// Document retains bytes even for an unsupported version. Accessors return copies.
type Document struct {
	raw       json.RawMessage
	record    *Record
	version   string
	mandatory bool
}

func (d Document) Raw() []byte     { return append([]byte(nil), d.raw...) }
func (d Document) Version() string { return d.version }
func (d Document) Mandatory() bool { return d.mandatory }
func (d Document) Record() *Record {
	if d.record == nil {
		return nil
	}
	data, _ := json.Marshal(d.record)
	var r Record
	_ = json.Unmarshal(data, &r)
	return &r
}

// Revision identifies the immutable selection, independently of a name or file.
type Revision struct {
	Subject  Subject
	Kind     string // direction or target
	Revision string
}

// Decision is an already verified source snapshot supplied by an adapter. The
// adapter owns source authenticity, freshness, visibility and authority policy.
// A boolean on the product record cannot manufacture any of these facts.
type Decision struct {
	Reference Reference
	Subject   Revision
	Authority string
	Verdict   string // approved or rejected
	Source    string
	Available bool
}
type Evidence struct {
	Authorities []string
	Decisions   []Decision
}

type ResolvedTarget struct {
	Identity  Revision
	Target    Target
	Direction Direction
	Approval  string // approved, rejected or unknown
	State     string // known, unknown or withdrawn; known is a binding, not observed benefit
	Observed  []Observation
}
type Resolution struct {
	State     string // unbound, unsupported, known or withdrawn
	Version   string
	Mandatory bool
	Targets   []ResolvedTarget
}
