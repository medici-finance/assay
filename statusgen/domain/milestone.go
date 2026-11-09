package domain

// Milestone receipt composition belongs to S-verification-domain. These records
// are distinct from brief WorkIdentity and do not allocate workflow instances.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/statusgen/workref"
)

const MilestoneReceiptSchema = "milestone-receipt-v1"
const MilestoneDecisionSchema = "milestone-decision-v1"
const MilestoneAuthoritySchema = "milestone-authority-v1"

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var criterionRE = regexp.MustCompile(`^C[0-9]{2,}$`)

// EvidencePointer is immutable, root-relative and exact. Anchor identifies a
// Verify row, witness or provenance record; a digest alone is not authority.
type EvidencePointer struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	Path       string `json:"path"`
	Digest     string `json:"digest"`
	Anchor     string `json:"anchor"`
}

func (p EvidencePointer) Validate() error {
	if !repoRE.MatchString(p.Repository) || !objectRE.MatchString(p.Commit) || !digestRE.MatchString(p.Digest) || p.Path == "" || p.Path == "." || path.IsAbs(p.Path) || path.Clean(p.Path) != p.Path || strings.HasPrefix(p.Path, "../") || strings.ContainsAny(p.Path, "\\\x00") {
		return refuse("milestone-pointer")
	}
	return nil
}
func ContentDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

type MilestoneSubject struct {
	Candidates    []Candidate `json:"candidates"`
	Artifacts     []string    `json:"artifacts"`
	Environment   string      `json:"environment"`
	Configuration string      `json:"configuration"`
	Interface     string      `json:"interface"`
	Profile       string      `json:"profile"`
}

func (s MilestoneSubject) validate() error {
	if s.Candidates == nil || s.Artifacts == nil || len(s.Candidates)+len(s.Artifacts) == 0 || !digestRE.MatchString(s.Environment) || !digestRE.MatchString(s.Configuration) || s.Interface == "" || s.Profile == "" {
		return refuse("milestone-subject")
	}
	seen := map[Candidate]bool{}
	for _, c := range s.Candidates {
		if e := c.Validate(); e != nil {
			return e
		}
		if seen[c] {
			return refuse("milestone-subject-duplicate")
		}
		seen[c] = true
	}
	for i, a := range s.Artifacts {
		if !digestRE.MatchString(a) || slices.Contains(s.Artifacts[:i], a) {
			return refuse("milestone-artifact")
		}
	}
	return nil
}

type MilestoneContributor struct {
	Brief    string            `json:"brief"`
	UUID     string            `json:"uuid"`
	Revision uint64            `json:"revision"`
	Source   EvidencePointer   `json:"source"`
	Evidence []EvidencePointer `json:"evidence"`
}
type MilestonePrerequisite struct {
	Milestone   string          `json:"milestone"`
	Manifest    string          `json:"manifest"`
	Decision    DecisionPointer `json:"decision"`
	Environment string          `json:"environment"`
	Interface   string          `json:"interface"`
	Profile     string          `json:"profile"`
}
type MilestoneCriterionResult struct {
	ID         string            `json:"id"`
	Result     string            `json:"result"`
	ObservedAt string            `json:"observed-at"`
	Verifier   Actor             `json:"verifier"`
	Provenance EvidencePointer   `json:"provenance"`
	Evidence   []EvidencePointer `json:"evidence"`
}
type MilestoneManifest struct {
	Schema        string                     `json:"schema"`
	Milestone     string                     `json:"milestone"`
	Contract      EvidencePointer            `json:"contract"`
	Subject       MilestoneSubject           `json:"subject"`
	Contributors  []MilestoneContributor     `json:"contributors"`
	Prerequisites []MilestonePrerequisite    `json:"prerequisites"`
	Criteria      []MilestoneCriterionResult `json:"criteria"`
	Authority     EvidencePointer            `json:"authority"`
	ObservedAt    string                     `json:"observed-at"`
	Evaluator     string                     `json:"evaluator"`
}

func (m MilestoneManifest) Validate() error {
	if m.Schema != MilestoneReceiptSchema {
		return unknown("milestone-schema")
	}
	if _, e := workref.Parse(m.Milestone, workref.TypedMilestone); e != nil {
		return refuse("milestone-identity")
	}
	if e := m.Subject.validate(); e != nil {
		return e
	}
	if m.Contributors == nil || m.Prerequisites == nil || m.Criteria == nil || len(m.Criteria) == 0 || !timestamp(m.ObservedAt) || m.Evaluator == "" {
		return refuse("milestone-manifest")
	}
	pointers := []EvidencePointer{m.Contract, m.Authority}
	seen := map[string]bool{}
	for _, c := range m.Contributors {
		r, e := workref.Parse(c.Brief, workref.TypedBrief)
		if e != nil || r.Cell == "" || r.Repo == "" || r.Revision != 0 || seen[c.Brief] || !uuidRE.MatchString(c.UUID) || c.Revision == 0 || len(c.Evidence) == 0 {
			return refuse("milestone-contributor")
		}
		seen[c.Brief] = true
		pointers = append(pointers, c.Source)
		pointers = append(pointers, c.Evidence...)
	}
	seen = map[string]bool{}
	for _, p := range m.Prerequisites {
		if _, e := workref.Parse(p.Milestone, workref.TypedMilestone); e != nil {
			return refuse("milestone-prerequisite")
		}
		if seen[p.Milestone] || !digestRE.MatchString(p.Manifest) || p.Decision.Validate() != nil || p.Environment == "" || p.Interface == "" || p.Profile == "" {
			return refuse("milestone-prerequisite")
		}
		seen[p.Milestone] = true
	}
	seen = map[string]bool{}
	for _, c := range m.Criteria {
		if !criterionRE.MatchString(c.ID) || seen[c.ID] || !timestamp(c.ObservedAt) || c.Verifier.validate() != nil || len(c.Evidence) == 0 || c.Provenance.Anchor == "" {
			return refuse("milestone-criterion")
		}
		seen[c.ID] = true
		if c.Result != "pass" && c.Result != "fail" && c.Result != "could-not-check" {
			return unknown("milestone-result")
		}
		pointers = append(pointers, c.Provenance)
		pointers = append(pointers, c.Evidence...)
	}
	for _, p := range pointers {
		if e := p.Validate(); e != nil {
			return e
		}
	}
	return nil
}

// Canonical includes no decision pointer, so the later human comment can name
// this digest without a cycle. Array order is significant, object keys sorted.
func (m MilestoneManifest) Canonical() ([]byte, error) {
	if e := m.Validate(); e != nil {
		return nil, e
	}
	raw, e := json.Marshal(m)
	if e != nil {
		return nil, e
	}
	var v any
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	if e = d.Decode(&v); e != nil {
		return nil, e
	}
	return json.Marshal(v)
}
func (m MilestoneManifest) Digest() (string, error) {
	b, e := m.Canonical()
	if e != nil {
		return "", e
	}
	return ContentDigest(b), nil
}

// Definition comes from the protected contract snapshot, never the producer's
// manifest. Contributor pins may omit a revision, but observations may not.
type MilestoneDefinition struct {
	Milestone             string
	Source                EvidencePointer
	Authority             EvidencePointer
	Contributors          []workref.Result
	Prerequisites         []string
	Criteria              []string
	CoverageKnown         bool
	CriterionContributors map[string][]string
}
type MilestoneAuthority struct {
	Schema           string          `json:"schema"`
	Source           EvidencePointer `json:"-"`
	Milestone        string          `json:"milestone"`
	DecisionIssue    int             `json:"decision-issue"`
	Humans           []string        `json:"humans"`
	Duty             DutyPolicy      `json:"duty"`
	SeparateAcceptor bool            `json:"separate-acceptor"`
}
type DecisionPointer struct {
	Repository string `json:"repository"`
	Issue      int    `json:"issue"`
	Comment    int64  `json:"comment"`
	Digest     string `json:"digest"`
}

func (p DecisionPointer) Validate() error {
	if !repoRE.MatchString(p.Repository) || p.Issue < 1 || p.Comment < 1 || !digestRE.MatchString(p.Digest) {
		return refuse("milestone-decision-pointer")
	}
	return nil
}

// DecisionPayload is the entire authenticated comment, not a prose match.
type MilestoneDecisionPayload struct {
	Schema      string           `json:"schema"`
	Milestone   string           `json:"milestone"`
	Contract    EvidencePointer  `json:"contract"`
	Manifest    string           `json:"manifest"`
	Disposition string           `json:"disposition"`
	Replaces    *DecisionPointer `json:"replaces,omitempty"`
}
type MilestoneDecision struct {
	Pointer       DecisionPointer
	Payload       MilestoneDecisionPayload
	Actor         Actor
	Authenticated bool
	// Lineage is selected by the existing trusted identity owner, never inferred
	// from the login, issuer, key or manifest. nil means could-not-check.
	Lineage []string
	// Unavailable means identity/content could not be observed, rather than an
	// observed unauthorized comment. The latter is quarantined, never a conflict.
	Unavailable bool
}
type MilestoneBriefObservation struct {
	Contributor  MilestoneContributor
	Status       string
	Record       Record
	Observations Observations
	Claims       []Provenance
	Selector     Selector
}
type MilestoneEvidenceObservation struct {
	Pointer EvidencePointer
	State   Completeness
}
type MilestonePrerequisiteObservation struct {
	Receipt    MilestonePrerequisite
	Historical bool
	State      string
}
type MilestoneInputs struct {
	Definition         MilestoneDefinition
	Policy             MilestoneAuthority
	Registry           workref.Registry
	Owner              workref.Context
	ContractProtected  Completeness
	AuthorityProtected Completeness
	Briefs             []MilestoneBriefObservation
	Prerequisites      []MilestonePrerequisiteObservation
	Decisions          []MilestoneDecision
	DecisionsComplete  bool
	Evidence           []MilestoneEvidenceObservation
	CurrentContract    EvidencePointer
	CurrentMilestone   string
	CurrentSubject     MilestoneSubject
	ObservedAt         string
	ValidUntil         string // required explicit applicability bound; no default TTL
	Evaluator          string
	Requirements       readercaps.Requirements
	Reader             readercaps.Observation
}
type MilestoneHistory struct {
	Manifest   string
	Decision   DecisionPointer
	Revoked    bool
	Superseded bool
}
type MilestoneAssessment struct {
	History               []MilestoneHistory
	State                 string
	AcceptanceOutstanding bool
	Blockers              []string
	Unknowns              []string
	Quarantined           []DecisionPointer
}

func (a *MilestoneAssessment) problem(e error) {
	if r, ok := e.(*Refusal); ok && r.State == "could-not-check" {
		a.Unknowns = append(a.Unknowns, r.Code)
	} else {
		a.Blockers = append(a.Blockers, e.Error())
	}
}

// AssessMilestone consumes only trusted normalized observations. Authentication,
// selection and protected-branch proofs remain adapter responsibilities. No
// observation, supported result or historical receipt authorizes an effect.
// semantic: S-verification-domain
func AssessMilestone(m MilestoneManifest, in MilestoneInputs) MilestoneAssessment {
	out := MilestoneAssessment{History: []MilestoneHistory{}, State: "unknown", AcceptanceOutstanding: true, Blockers: []string{}, Unknowns: []string{}, Quarantined: []DecisionPointer{}}
	if e := m.Validate(); e != nil {
		out.problem(e)
		return out
	}
	digest, _ := m.Digest()
	def := in.Definition
	resolved, e := workref.Resolve(m.Milestone, &in.Owner, in.Registry, workref.TypedMilestone)
	if e != nil || resolved.Identity.String()+"@"+strconv.FormatUint(resolved.Revision, 10) != def.Milestone || m.Milestone != def.Milestone {
		out.Blockers = append(out.Blockers, "contract-identity")
		return out
	}
	if m.Authority.Repository != m.Contract.Repository || m.Contract != def.Source || m.Authority != def.Authority || m.Authority != in.Policy.Source {
		out.Blockers = append(out.Blockers, "contract-source")
		return out
	}
	// Independently enforced below host admission; unmerged sources cannot set policy.
	for name, state := range map[string]Completeness{"contract": in.ContractProtected, "authority": in.AuthorityProtected} {
		if state != Present {
			if state == Missing {
				out.Blockers = append(out.Blockers, name+"-unmerged")
			} else {
				out.Unknowns = append(out.Unknowns, name+"-protected-source-unknown")
			}
		}
	}
	if !def.CoverageKnown {
		out.Unknowns = append(out.Unknowns, "contract-coverage")
	}
	if in.Policy.Schema != MilestoneAuthoritySchema || in.Policy.Milestone != def.Milestone || in.Policy.DecisionIssue < 1 || len(in.Policy.Humans) == 0 || !in.Policy.SeparateAcceptor {
		out.Unknowns = append(out.Unknowns, "authority-policy")
	}
	for _, human := range in.Policy.Humans {
		if !principal(human) || !strings.HasPrefix(human, "human:") {
			out.Blockers = append(out.Blockers, "authority-human")
		}
	}
	if len(m.Contributors) != len(def.Contributors) || len(m.Prerequisites) != len(def.Prerequisites) || len(m.Criteria) != len(def.Criteria) {
		out.Blockers = append(out.Blockers, "contract-set")
	}
	lineages := map[string]bool{}
	selected := map[EvidencePointer]map[string]bool{}
	for _, pin := range def.Contributors {
		idx := slices.IndexFunc(m.Contributors, func(c MilestoneContributor) bool { return c.Brief == pin.Identity.String() })
		if idx < 0 {
			out.Blockers = append(out.Blockers, "contributor-missing")
			continue
		}
		c := m.Contributors[idx]
		if pin.Revision != 0 && pin.Revision != c.Revision {
			out.Blockers = append(out.Blockers, "contributor-revision")
		}
		idx = slices.IndexFunc(in.Briefs, func(o MilestoneBriefObservation) bool { return o.Contributor.Brief == c.Brief })
		if idx < 0 {
			out.Unknowns = append(out.Unknowns, "contributor-unavailable")
			continue
		}
		o := in.Briefs[idx]
		// Compare every selected pointer as well as revision: never substitute latest evidence.
		if !reflect.DeepEqual(o.Contributor, c) {
			out.Blockers = append(out.Blockers, "contributor-binding")
			continue
		}
		if o.Status != "done" {
			if o.Status == "" {
				out.Unknowns = append(out.Unknowns, "contributor-status")
			} else {
				out.Blockers = append(out.Blockers, "work-remaining")
			}
		}
		if o.Record.Work.Brief != pin.Compatibility || !slices.Contains(m.Subject.Candidates, o.Record.Candidate) {
			out.Blockers = append(out.Blockers, "brief-subject")
			continue
		}
		assessment, err := Assess(o.Record, o.Observations)
		if err != nil {
			out.problem(err)
			continue
		}
		if assessment.State != "supported" {
			out.Unknowns = append(out.Unknowns, "brief-evidence-support")
		}
		duty, err := EvaluateDuty(o.Record, assessment, in.Policy.Duty, o.Selector, o.Claims)
		if err != nil {
			out.problem(err)
			continue
		}
		if duty.State == "held" {
			out.Unknowns = append(out.Unknowns, duty.Reasons...)
		} else if duty.State != "satisfied" {
			out.Blockers = append(out.Blockers, duty.Reasons...)
		}
		for _, claim := range o.Claims {
			p, err := Corroborate(claim, o.Selector)
			if err != nil {
				out.problem(err)
				continue
			}
			for l := range p.lineage() {
				lineages[l] = true
			}
			if p.Duty == in.Policy.Duty.Duty {
				for _, criterion := range m.Criteria {
					if p.Evidence == criterion.Provenance.Digest && p.Executor == criterion.Verifier && p.Environment.ManifestDigest == m.Subject.Environment && p.Capture.ObservedAt == criterion.ObservedAt && slices.Contains(c.Evidence, criterion.Provenance) {
						if selected[criterion.Provenance] == nil {
							selected[criterion.Provenance] = map[string]bool{}
						}
						selected[criterion.Provenance][c.Brief] = true
					}
				}
			}
		}
	}
	for _, id := range def.Criteria {
		idx := slices.IndexFunc(m.Criteria, func(c MilestoneCriterionResult) bool { return c.ID == id })
		if idx < 0 {
			out.Blockers = append(out.Blockers, "criterion-missing")
			continue
		}
		c := m.Criteria[idx]
		if c.Result == "fail" {
			out.Blockers = append(out.Blockers, "criterion-failed")
		} else if c.Result != "pass" {
			out.Unknowns = append(out.Unknowns, "criterion-unknown")
		}
		mapped := false
		for _, brief := range def.CriterionContributors[c.ID] {
			if selected[c.Provenance][brief] {
				mapped = true
			}
		}
		if !mapped || !slices.Contains(c.Evidence, c.Provenance) {
			out.Unknowns = append(out.Unknowns, "criterion-provenance")
		}
		observed, _ := time.Parse(time.RFC3339, c.ObservedAt)
		accepted, _ := time.Parse(time.RFC3339, m.ObservedAt)
		if observed.After(accepted) {
			out.Blockers = append(out.Blockers, "criterion-after-receipt")
		}
	}
	for _, ref := range def.Prerequisites {
		idx := slices.IndexFunc(m.Prerequisites, func(p MilestonePrerequisite) bool { return p.Milestone == ref })
		if idx < 0 {
			out.Blockers = append(out.Blockers, "prerequisite-missing")
			continue
		}
		p := m.Prerequisites[idx]
		if p.Environment != m.Subject.Environment || p.Interface != m.Subject.Interface || p.Profile != m.Subject.Profile {
			out.Blockers = append(out.Blockers, "prerequisite-profile")
		}
		idx = slices.IndexFunc(in.Prerequisites, func(o MilestonePrerequisiteObservation) bool { return o.Receipt == p })
		if idx < 0 || !in.Prerequisites[idx].Historical {
			out.Unknowns = append(out.Unknowns, "prerequisite-receipt")
		}
	}
	// Historical qualification precedes current applicability. Later unavailability,
	// expiry, revocation or a different subject never rewrites an accepted receipt.
	historicalOK := len(out.Blockers) == 0 && len(out.Unknowns) == 0
	var accepted []MilestoneDecision
	var successors []MilestoneDecision
	for _, d := range in.Decisions {
		authorized := d.Actor.Kind == "human" && slices.Contains(in.Policy.Humans, d.Actor.ID)
		if d.Authenticated && !authorized {
			out.Quarantined = append(out.Quarantined, d.Pointer)
			continue
		}
		if d.Unavailable {
			out.Unknowns = append(out.Unknowns, "decision-unavailable")
			continue
		}
		if !d.Authenticated || !authorized {
			out.Quarantined = append(out.Quarantined, d.Pointer)
			continue
		}
		if len(d.Lineage) == 0 {
			out.Unknowns = append(out.Unknowns, "decision-lineage")
			continue
		}
		overlap := false
		for _, p := range d.Lineage {
			if !principal(p) {
				out.Unknowns = append(out.Unknowns, "decision-lineage")
				overlap = true
			}
			if lineages[p] {
				overlap = true
			}
		}
		if overlap {
			out.Quarantined = append(out.Quarantined, d.Pointer)
			continue
		}
		p := d.Payload
		if d.Pointer.Validate() != nil || d.Pointer.Repository != m.Contract.Repository || d.Pointer.Issue != in.Policy.DecisionIssue || p.Schema != MilestoneDecisionSchema {
			out.Unknowns = append(out.Unknowns, "decision-binding")
			continue
		}
		if p.Milestone != m.Milestone || p.Contract != m.Contract || p.Manifest != digest {
			continue
		}
		switch p.Disposition {
		case "accept":
			if p.Replaces == nil {
				accepted = append(accepted, d)
			} else {
				successors = append(successors, d)
			}
		case "revoke", "supersede":
			if p.Replaces != nil {
				successors = append(successors, d)
			} else {
				out.Unknowns = append(out.Unknowns, "decision-target")
			}
		case "reject", "hold": // An actor corroboration pass alone is not acceptance.
		default:
			out.Unknowns = append(out.Unknowns, "decision-disposition")
		}
	}
	if historicalOK {
		for _, d := range accepted {
			out.History = append(out.History, MilestoneHistory{Manifest: digest, Decision: d.Pointer})
		}
	}
	if len(out.History) > 0 {
		out.AcceptanceOutstanding = false
	}
	if len(accepted) > 1 {
		out.Unknowns = append(out.Unknowns, "conflicting-acceptances")
	}
	for i := range out.History {
		var replacements []MilestoneDecision
		for _, d := range successors {
			if *d.Payload.Replaces == out.History[i].Decision {
				replacements = append(replacements, d)
			}
		}
		if len(replacements) > 1 {
			out.Unknowns = append(out.Unknowns, "conflicting-successors")
		}
		for _, d := range replacements {
			if d.Payload.Disposition == "revoke" {
				out.History[i].Revoked = true
				out.Blockers = append(out.Blockers, "receipt-revoked")
			} else {
				out.History[i].Superseded = true
				out.Blockers = append(out.Blockers, "receipt-superseded")
			}
		}
	}
	if !in.DecisionsComplete {
		out.Unknowns = append(out.Unknowns, "decision-coverage")
	}
	if caps := readercaps.Assess(in.Requirements, in.Reader); caps.State != readercaps.Supported {
		out.Unknowns = append(out.Unknowns, "receipt-capability")
	}
	if in.CurrentMilestone != m.Milestone || in.CurrentContract != m.Contract || !reflect.DeepEqual(in.CurrentSubject, m.Subject) {
		out.Unknowns = append(out.Unknowns, "current-subject")
	}
	now, err1 := time.Parse(time.RFC3339, in.ObservedAt)
	until, err2 := time.Parse(time.RFC3339, in.ValidUntil)
	at, _ := time.Parse(time.RFC3339, m.ObservedAt)
	if err1 != nil || err2 != nil || now.Before(at) || until.Before(at) || now.After(until) || in.Evaluator != m.Evaluator {
		out.Unknowns = append(out.Unknowns, "current-applicability")
	}
	for _, p := range m.Prerequisites {
		idx := slices.IndexFunc(in.Prerequisites, func(o MilestonePrerequisiteObservation) bool { return o.Receipt == p })
		if idx < 0 || in.Prerequisites[idx].State == "unknown" {
			out.Unknowns = append(out.Unknowns, "prerequisite-current")
		} else if in.Prerequisites[idx].State != "supported" {
			out.Blockers = append(out.Blockers, "prerequisite-unavailable")
		}
	}
	for _, p := range MilestoneEvidencePointers(m) {
		idx := slices.IndexFunc(in.Evidence, func(o MilestoneEvidenceObservation) bool { return o.Pointer == p })
		if idx < 0 || in.Evidence[idx].State != Present {
			out.Unknowns = append(out.Unknowns, "evidence-unavailable")
		}
	}
	if len(out.Blockers) > 0 {
		out.State = "blocked"
	} else if len(out.Unknowns) == 0 && len(out.History) > 0 {
		out.State = "supported"
	}
	return out
}
func MilestoneEvidencePointers(m MilestoneManifest) []EvidencePointer {
	out := []EvidencePointer{m.Contract, m.Authority}
	for _, c := range m.Contributors {
		out = append(out, c.Source)
		out = append(out, c.Evidence...)
	}
	for _, c := range m.Criteria {
		out = append(out, c.Provenance)
		out = append(out, c.Evidence...)
	}
	return out
}
