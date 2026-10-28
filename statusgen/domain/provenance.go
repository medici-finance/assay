package domain

import (
	"reflect"
	"regexp"
)

// ProvenanceSchema is evidence-provenance-v1: who requested, represented,
// executed and is accountable for one piece of evidence, and which assurance
// dimensions a trusted adapter corroborated. It is a pure record. Nothing here
// authenticates an actor, verifies a signature, mints a grant, waives a duty or
// activates a consumer; the trusted adapters and the existing identity owner do
// the observing, and this package only compares what they normalized.
const ProvenanceSchema = "evidence-provenance-v1"

// Closed vocabularies. Order matters only for proofClasses, which is a ladder.
var (
	actorKinds          = []string{"human", "provider-app", "agent-session", "service"}
	proofClasses        = []string{"self-report", "authenticated", "attested", "signed", "custody-qualified"}
	corroborationStates = []string{"corroborated", "claimed", "missing", "withheld", "could-not-check"}
	// dutyStates is ordered by severity; a result only ever moves rightwards.
	dutyStates = []string{"satisfied", "held", "unsatisfied"}
	// principalRE is an opaque, pseudonymous principal handle: a kind prefix and
	// an id. Names, emails and provider account numbers are not required here;
	// redaction stays with the existing access policy.
	principalRE = regexp.MustCompile(`^[a-z][a-z0-9-]*:[a-z0-9][a-z0-9._-]{0,127}$`)
)

const corroborated = "corroborated"

// Actor is an identity a trusted adapter authenticated or observed. A
// provider-app acting for a human remains a provider-app act.
type Actor struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

// ContractBinding names the accepted definition the evidence was produced for.
type ContractBinding struct {
	Version uint64 `json:"version"`
	Digest  string `json:"digest"`
}

// Capture records who captured the evidence and from which source kind.
type Capture struct {
	Actor      Actor  `json:"actor"`
	Source     string `json:"source"`
	ObservedAt string `json:"observed-at"`
}

// Environment identifies the execution environment manifest. It makes evidence
// attributable and comparable; it does not prove the run is reproducible.
type Environment struct {
	ManifestDigest string `json:"manifest-digest"`
}

// Corroboration keeps each assurance dimension separate. Only "corroborated"
// counts; a claimed signature or issuer label is not verified proof. Duty
// independence is deliberately absent: it is computed by EvaluateDuty from
// lineage, never declared.
type Corroboration struct {
	Attribution string `json:"attribution"`
	Lineage     string `json:"lineage"`
	Execution   string `json:"execution"`
	Signature   string `json:"signature"`
	Custody     string `json:"custody"`
}

// Provenance describes one evidence item. Responsible plus Delegation is the
// accountable lineage. Sponsor, Issuer and Key are recorded for accountability
// and audit but are never part of the independence comparison.
type Provenance struct {
	Schema        string          `json:"schema"`
	Requires      []string        `json:"requires"`
	Work          WorkIdentity    `json:"work"`
	Candidate     Candidate       `json:"candidate"`
	Contract      ContractBinding `json:"contract"`
	Duty          string          `json:"duty"`
	Evidence      string          `json:"evidence"`
	Requester     Actor           `json:"requester"`
	Represented   Actor           `json:"represented"`
	Executor      Actor           `json:"executor"`
	Responsible   string          `json:"responsible"`
	Delegation    []string        `json:"delegation"`
	Sponsor       string          `json:"sponsor"`
	Issuer        string          `json:"issuer"`
	Key           string          `json:"key"`
	Capture       Capture         `json:"capture"`
	Environment   Environment     `json:"environment"`
	DeclaredProof string          `json:"declared-proof"`
	Corroboration Corroboration   `json:"corroboration"`
}

func rank(set []string, v string) int {
	for i, s := range set {
		if s == v {
			return i
		}
	}
	return -1
}

func principal(s string) bool { return principalRE.MatchString(s) }

func (a Actor) validate() error {
	if !principal(a.ID) {
		return refuse("actor-shape")
	}
	if rank(actorKinds, a.Kind) < 0 {
		return unknown("unsupported-actor-kind")
	}
	return nil
}

func (c Corroboration) dimensions() []string {
	return []string{c.Attribution, c.Lineage, c.Execution, c.Signature, c.Custody}
}

// Validate checks shape and cross-field rules. It cannot tell whether any value
// is true; Corroborate binds a claim to the trusted selector's record.
func (p Provenance) Validate() error {
	if p.Schema != ProvenanceSchema {
		return unknown("unsupported-schema")
	}
	if p.Requires == nil {
		return refuse("requires-missing")
	}
	for _, v := range p.Requires {
		if v != "provenance@1" {
			return unknown("unsupported-capability")
		}
	}
	for _, err := range []error{p.Work.Validate(), p.Candidate.Validate()} {
		if err != nil {
			return err
		}
	}
	if p.Contract.Version == 0 || !digestRE.MatchString(p.Contract.Digest) {
		return refuse("contract-shape")
	}
	if !nodeRE.MatchString(p.Duty) || !digestRE.MatchString(p.Evidence) {
		return refuse("evidence-shape")
	}
	for _, a := range []Actor{p.Requester, p.Represented, p.Executor, p.Capture.Actor} {
		if err := a.validate(); err != nil {
			return err
		}
	}
	if !principal(p.Responsible) || !principal(p.Sponsor) || !principal(p.Issuer) || !principal(p.Key) {
		return refuse("principal-shape")
	}
	if p.Delegation == nil {
		return refuse("lineage-shape")
	}
	seen := map[string]bool{p.Responsible: true}
	for _, d := range p.Delegation {
		if !principal(d) || seen[d] {
			return refuse("lineage-shape")
		}
		seen[d] = true
	}
	if !nodeRE.MatchString(p.Capture.Source) || !timestamp(p.Capture.ObservedAt) {
		return refuse("capture-shape")
	}
	if !digestRE.MatchString(p.Environment.ManifestDigest) {
		return refuse("environment-shape")
	}
	if rank(proofClasses, p.DeclaredProof) < 0 {
		return unknown("unsupported-proof-class")
	}
	for _, s := range p.Corroboration.dimensions() {
		if rank(corroborationStates, s) < 0 {
			return unknown("unsupported-corroboration")
		}
	}
	return nil
}

// EffectiveProof is the highest ladder rung whose dimension and every rung
// below it are corroborated. The declared label never raises it.
func (p Provenance) EffectiveProof() string {
	c := p.Corroboration
	ladder := []string{c.Attribution, c.Execution, c.Signature, c.Custody}
	n := 0
	for n < len(ladder) && ladder[n] == corroborated {
		n++
	}
	return proofClasses[n]
}

// lineage is the accountable set: the responsible principal and every
// delegating principal. Issuer, key and sponsor are excluded by construction.
func (p Provenance) lineage() map[string]bool {
	set := map[string]bool{p.Responsible: true}
	for _, principal := range p.Delegation {
		set[principal] = true
	}
	return set
}

// sharesLineage reports whether two evidence items have any accountable
// principal in common. Distinct keys for one principal still share it.
func sharesLineage(a, b Provenance) bool {
	la := a.lineage()
	for k := range b.lineage() {
		if la[k] {
			return true
		}
	}
	return false
}

// Selector is the trusted-composition port: the only source of corroborated
// provenance. Production adapters live outside this package; none is installed.
type Selector interface {
	Select(work WorkIdentity, candidate Candidate, duty, evidence string) (Provenance, bool)
}

// Corroborate replaces a caller-supplied claim with the selector's record. A
// claim that differs in any field (a model-chosen principal, delegation or
// proof label) is refused; an unknown item is could-not-check, never a pass.
func Corroborate(claim Provenance, sel Selector) (Provenance, error) {
	if sel == nil {
		return Provenance{}, unknown("provenance-uncorroborated")
	}
	trusted, ok := sel.Select(claim.Work, claim.Candidate, claim.Duty, claim.Evidence)
	if !ok {
		return Provenance{}, unknown("provenance-uncorroborated")
	}
	if err := trusted.Validate(); err != nil {
		return Provenance{}, err
	}
	if !reflect.DeepEqual(trusted, claim) {
		return Provenance{}, refuse("provenance-not-selected")
	}
	return trusted, nil
}

// DutyPolicy is the caller's selected existing duty policy, normalized. This
// package defines no policy of its own and has no exception or waiver input.
type DutyPolicy struct {
	Duty     string   `json:"duty"`
	Against  []string `json:"against"`
	HumanAct bool     `json:"human-act"`
	MinProof string   `json:"min-proof"`
}

// DutyResult is three-state: unsatisfied is an observed conflict, held is a
// missing or uncorroborated fact. Neither is waived here.
type DutyResult struct {
	Duty    string   `json:"duty"`
	State   string   `json:"state"`
	Proof   string   `json:"proof"`
	Reasons []string `json:"reasons"`
}

func (d *DutyResult) note(state, reason string) {
	if rank(d.Reasons, reason) < 0 {
		d.Reasons = append(d.Reasons, reason)
	}
	if rank(dutyStates, state) > rank(dutyStates, d.State) {
		d.State = state
	}
}

// EvaluateDuty applies one duty policy to evidence bound to the record's
// current assessment. Every claim is passed through Corroborate against sel
// before any field of it is read, so a claim the selector did not select is
// refused here and a nil selector is could-not-check. The selector itself is
// the caller's trusted composition; this function cannot tell a trusted
// selector from one that echoes its input. It does not consult
// Assessment.State: duty provenance and evidence support are separate claims.
// The result grants nothing; consumers keep their own effect and role controls.
// semantic: S-verification-domain
func EvaluateDuty(r Record, a Assessment, policy DutyPolicy, sel Selector, claims []Provenance) (DutyResult, error) {
	if err := a.CheckCurrent(r); err != nil {
		return DutyResult{}, err
	}
	if sel == nil {
		return DutyResult{}, unknown("provenance-uncorroborated")
	}
	if !nodeRE.MatchString(policy.Duty) || rank(proofClasses, policy.MinProof) < 0 || policy.Against == nil {
		return DutyResult{}, refuse("duty-policy")
	}
	for _, duty := range policy.Against {
		if !nodeRE.MatchString(duty) || duty == policy.Duty {
			return DutyResult{}, refuse("duty-policy")
		}
	}
	byDuty := map[string]Provenance{}
	for _, claim := range claims {
		p, err := Corroborate(claim, sel)
		if err != nil {
			return DutyResult{}, err
		}
		// Each duty acts at its own node, so node and attempt may differ; the
		// work, instance, exact candidate and accepted definition may not.
		if p.Work.Brief != r.Work.Brief || p.Work.Instance != r.Work.Instance || p.Candidate != r.Candidate ||
			p.Contract.Version != r.Acceptance.Version || p.Contract.Digest != r.Acceptance.Digest {
			return DutyResult{}, refuse("provenance-subject")
		}
		if _, dup := byDuty[p.Duty]; dup {
			return DutyResult{}, refuse("duty-ambiguous")
		}
		byDuty[p.Duty] = p
	}
	out := DutyResult{Duty: policy.Duty, State: "satisfied", Proof: proofClasses[0], Reasons: []string{}}
	p, ok := byDuty[policy.Duty]
	if !ok {
		out.note("held", "duty-evidence-missing")
		return out, nil
	}
	out.Proof = p.EffectiveProof()
	if rank(proofClasses, out.Proof) < rank(proofClasses, policy.MinProof) {
		out.note("held", "proof-uncorroborated")
	}
	if rank(proofClasses, p.DeclaredProof) > rank(proofClasses, out.Proof) {
		out.note("held", "declared-proof-unsupported")
	}
	if policy.HumanAct {
		if p.Executor.Kind != "human" || p.Requester != p.Executor || p.Represented != p.Executor {
			out.note("unsatisfied", "not-human-act")
		} else if p.Corroboration.Attribution != corroborated {
			out.note("held", "attribution-uncorroborated")
		}
	}
	for _, duty := range policy.Against {
		q, ok := byDuty[duty]
		if !ok {
			out.note("held", "duty-evidence-missing")
			continue
		}
		if p.Corroboration.Lineage != corroborated || q.Corroboration.Lineage != corroborated {
			out.note("held", "lineage-uncorroborated")
			continue
		}
		if sharesLineage(p, q) {
			out.note("unsatisfied", "principal-overlap")
		}
	}
	return out, nil
}
