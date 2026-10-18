package contexthandoff

// Schema identifiers. A record whose "schema" value is not exactly one of these
// is refused; a new version is a new identifier, never a silent shape change.
const (
	HandoffSchema   = "context-handoff-v1"
	HandoffSchemaV2 = "context-handoff-v2"
	ManifestSchema  = "amendment-manifest-v1"
)

// Target states. A draft or proposed target is a distinct state from an
// approved one, and only an approved target carries an approval.
const (
	TargetDraft    = "draft"
	TargetProposed = "proposed"
	TargetApproved = "approved"
)

// Handoff is a context-handoff-v1 record: the seven handoff records of the
// future-state model, bound to the brief that declares them.
type Handoff struct {
	Schema     string       `json:"schema"`
	DeclaredBy string       `json:"declared_by"`
	Target     Target       `json:"target"`
	Discovery  Discovery    `json:"discovery"`
	Impacts    []Impact     `json:"impacts"`
	Amendments []Amendment  `json:"amendments"`
	History    []History    `json:"history"`
	ActiveWork []ActiveWork `json:"active_work"`
	Readiness  Readiness    `json:"readiness"`
}

// Target is the "target and intent" record.
type Target struct {
	State            string           `json:"state"`
	Revision         string           `json:"revision"`
	Approval         *Approval        `json:"approval"`
	Intent           string           `json:"intent"`
	Contracts        []ContractChange `json:"contracts"`
	Transition       *string          `json:"transition"`
	AcceptanceSource string           `json:"acceptance_source"`
}

// Approval records who approved which revision. Revision is mandatory: an
// approval with no recorded revision is refused.
type Approval struct {
	Revision  string `json:"revision"`
	Reference string `json:"reference"`
}

// ContractChange is one proposed contract revision. From is null for a new
// contract.
type ContractChange struct {
	Name string  `json:"name"`
	From *string `json:"from"`
	To   string  `json:"to"`
}

// Discovery is the "discovery receipt" record.
type Discovery struct {
	Sources         []Source `json:"sources"`
	Queries         []Query  `json:"queries"`
	DependencySlice []string `json:"dependency_slice"`
	Coverage        Coverage `json:"coverage"`
}

// Source is one member of the authorized source universe.
type Source struct {
	ID         string  `json:"id"`
	Revision   *string `json:"revision"`
	Membership string  `json:"membership"`
	Reason     string  `json:"reason"`
}

// Query is one discovery rule run: its version, parameters and results. An
// empty Results is a negative result, recorded as such.
type Query struct {
	Rule       string            `json:"rule"`
	Version    string            `json:"version"`
	Parameters map[string]string `json:"parameters"`
	Results    []string          `json:"results"`
	Truncated  bool              `json:"truncated"`
}

// Coverage states what was enumerated and what was unavailable. It is
// mandatory: a missing coverage is refused, never defaulted to complete.
type Coverage struct {
	State       string        `json:"state"`
	Enumerated  []string      `json:"enumerated"`
	Unavailable []Unavailable `json:"unavailable"`
}

// Unavailable is one included source that could not be read, with why.
type Unavailable struct {
	Source string `json:"source"`
	Reason string `json:"reason"`
}

// Impact is the "impact obligation" record for one existing plan identity.
type Impact struct {
	Subject      string       `json:"subject"`
	Lifecycle    string       `json:"lifecycle"`
	Evidence     *string      `json:"evidence"`
	Relationship Relationship `json:"relationship"`
	Disposition  string       `json:"disposition"`
	Reason       string       `json:"reason"`
}

// Relationship is how the subject was found: declared, mechanically observed
// or inferred, by which edge, at which locator.
type Relationship struct {
	Basis   string `json:"basis"`
	Edge    string `json:"edge"`
	Locator string `json:"locator"`
}

// Amendment is the "amendment obligation" record: what must change in one
// brief. Candidate is the produced patch, or null before one exists; the
// manifest is where a binding becomes mandatory.
type Amendment struct {
	ID         string   `json:"id"`
	Subject    string   `json:"subject"`
	Action     string   `json:"action"`
	Changes    []Change `json:"changes"`
	ReplacedBy []string `json:"replaced_by"`
	Candidate  *Patch   `json:"candidate"`
}

// Change is one before/after delta of an amendment.
type Change struct {
	Aspect string `json:"aspect"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// Patch binds a candidate patch to the revision it was made against.
type Patch struct {
	BaseRevision string `json:"base_revision"`
	Digest       string `json:"digest"`
}

// History is the "preserved history" record for one protected artifact.
type History struct {
	Kind       string      `json:"kind"`
	Subject    string      `json:"subject"`
	Locator    string      `json:"locator"`
	Digest     string      `json:"digest"`
	Correction *Correction `json:"correction"`
}

// Correction is a proposed historical correction; the original record stays.
type Correction struct {
	Rationale         string `json:"rationale"`
	Source            string `json:"source"`
	Review            string `json:"review"`
	ReplacementDigest string `json:"replacement_digest"`
}

// ActiveWork is the "active-work binding" record: an observed claim, not a
// board row.
type ActiveWork struct {
	Subject       string `json:"subject"`
	Claim         string `json:"claim"`
	BriefRevision string `json:"brief_revision"`
	ObservedAt    string `json:"observed_at"`
	Owner         string `json:"owner"`
	Checkpoint    string `json:"checkpoint"`
	Treatment     string `json:"treatment"`
	Acknowledged  bool   `json:"acknowledged"`
}

// Readiness is the "decisions and readiness" record.
type Readiness struct {
	Unresolved    []string `json:"unresolved"`
	DecidingRole  string   `json:"deciding_role"`
	NextAct       string   `json:"next_act"`
	Prerequisites []string `json:"prerequisites"`
	Facts         Facts    `json:"facts"`
}

// Facts keeps three facts apart that are easy to conflate.
type Facts struct {
	SourceAvailable     bool `json:"source_available"`
	ProposalApproved    bool `json:"proposal_approved"`
	DeploymentQualified bool `json:"deployment_qualified"`
}

// Manifest is an amendment-manifest-v1 record: one entry per amendment
// obligation of the handoff whose canonical digest it names.
type Manifest struct {
	Schema     string  `json:"schema"`
	DeclaredBy string  `json:"declared_by"`
	Handoff    string  `json:"handoff"`
	Entries    []Entry `json:"entries"`
}

// Entry binds one amendment obligation to its owning brief and delivery.
type Entry struct {
	Obligation      string   `json:"obligation"`
	Brief           string   `json:"brief"`
	Action          string   `json:"action"`
	DeliveryPaths   []string `json:"delivery_paths"`
	ContractVersion *string  `json:"contract_version"`
	Depends         []string `json:"depends"`
	TestCommands    []string `json:"test_commands"`
	ActiveWork      string   `json:"active_work"`
	Effort          string   `json:"effort"`
	Binding         Binding  `json:"binding"`
}

// Binding is exactly one of a bound patch or an explicit follow-up brief.
type Binding struct {
	Patch    *Patch  `json:"patch,omitempty"`
	FollowUp *string `json:"follow_up,omitempty"`
}
