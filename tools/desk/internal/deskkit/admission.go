package deskkit

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// admission.go — deterministic agentic admission over hard facts and bounded
// probabilistic advice (GEA-09/10/11 of the admission-assurance
// amendment; normative text in spec/agentic-admission-v1.md).
//
// The evaluator answers ONE question — "how much of this owner-admitted category may an
// agent do on this subject, right now?" — with one of five closed dispositions, the
// reason codes that produced it, the operations it permits, and the risk-input verdict
// whose gate nodes it adds to the work's graph. It is a PURE function: no model call,
// no clock read (the caller passes Now), no environment read, no I/O. A provider's
// advice reaches it only as an already-recorded Prediction (decisionassessment.go).
//
// The safety properties, each pinned by a test in admission_test.go:
//
//   - Facts decide; advice only restricts. A disposition is computed from the owner's
//     category ceiling and the hard facts first. Advice is then folded in with
//     moreRestrictive, which is monotone toward `blocked` — a calibrated label of any
//     confidence can move the answer DOWN the order, never up
//     (TestAgenticAdmissionConfidenceCannotAuthorize).
//   - A failed hard check is never averaged away. Authority-class checks fail closed to
//     `blocked`; a failed readiness check caps at the policy's declared ceiling, and
//     ValidateAdmissionPolicy refuses any policy whose ceiling for a failed check is an
//     agent lane.
//   - Unknown readiness holds implementation. A could-not-check (missing, stale,
//     unknown-state or binding-changed) readiness fact permits at most `discovery-only`,
//     and only when a separately authorized discovery grant whose read scope covers the
//     subject is supplied. `discovery-only` is never the result without such a grant,
//     whichever path (unknown readiness, a fail ceiling, advice) reached it, and the
//     result carries the covering read scope.
//   - Bindings are never vacuous. An empty subject revision, schema digest or environment
//     digest on either side is `blocked`, so "empty equals empty" never reads as bound.
//   - Reason codes are built only from fixed literals, closed-vocabulary values and
//     grammar-checked owner tokens (reasonTokenRE); an assessed value outside a
//     vocabulary is never echoed, so the party being assessed cannot forge a reason.
//   - All-hard-pass is necessary, not sufficient. The owner's category ceiling and its
//     permitted-operation list still bound the result.
//   - Absent advice changes nothing. Ignored advice (absent, disabled, stale, malformed,
//     uncalibrated, out of scope) leaves the facts-only result standing — that result IS
//     the default, so advice never becomes a precondition that its presence then lifts.
//   - Mandatory gates are a UNION. MandatoryGates adds the gate nodes the disposition's
//     risk-input verdict names to the brief's own; it never subtracts one
//     (TestAgenticAdmissionRiskInputUnion).
//   - Human floors stand. merge / release / deploy are never a permitted operation, at
//     any disposition, and a category that lists one is refused at validation.
//   - No single score. The result carries a disposition and reasons — never a numeric
//     safety, suitability or compliance score (GEA-11 keeps AssayScore,
//     AgenticAssessment and ControlAssurance separate products).
//
// Activation is NOT part of this file. Nothing here is consulted by dispatch; binding the
// result at the dispatch boundary is a separate change, behind its own human gate.

// AdmissionDisposition is the closed admission vocabulary (GEA-10). These are admission
// dispositions, not replacements for graph risk classes or desk roles.
type AdmissionDisposition string

const (
	AdmitBoundedAgentWork AdmissionDisposition = "bounded-agent-work"
	AdmitSupervisedAgent  AdmissionDisposition = "supervised-agent"
	AdmitHumanLed         AdmissionDisposition = "human-led"
	AdmitDiscoveryOnly    AdmissionDisposition = "discovery-only"
	AdmitBlocked          AdmissionDisposition = "blocked"
)

// admissionOrder lists the dispositions from LEAST to MOST restrictive. discovery-only
// sits above human-led because it permits no implementation by anyone, only scoped reads.
var admissionOrder = []AdmissionDisposition{
	AdmitBoundedAgentWork,
	AdmitSupervisedAgent,
	AdmitHumanLed,
	AdmitDiscoveryOnly,
	AdmitBlocked,
}

// AdmissionDispositions returns the closed vocabulary, least restrictive first — the
// Vocabulary an AssessmentRequest for an admission advisory dimension must declare.
func AdmissionDispositions() []string {
	out := make([]string, len(admissionOrder))
	for i, d := range admissionOrder {
		out[i] = string(d)
	}
	return out
}

// rank is the disposition's position in admissionOrder, or -1 when it is not a member.
func (d AdmissionDisposition) rank() int {
	for i, o := range admissionOrder {
		if o == d {
			return i
		}
	}
	return -1
}

// Valid reports whether d is a member of the closed vocabulary.
func (d AdmissionDisposition) Valid() bool { return d.rank() >= 0 }

// moreRestrictive returns whichever of a and b is further toward `blocked`. A value
// outside the vocabulary is treated as `blocked` — an unknown disposition never widens.
func moreRestrictive(a, b AdmissionDisposition) AdmissionDisposition {
	ra, rb := a.rank(), b.rank()
	if ra < 0 || rb < 0 {
		return AdmitBlocked
	}
	if rb > ra {
		return b
	}
	return a
}

// dispositionRiskInput is the EXPLICIT mapping from an admission disposition to the
// workflow pattern's risk-input verdict whose gate nodes the disposition adds. It is
// consumed only through MandatoryGates, which unions it with the brief's own verdict.
var dispositionRiskInput = map[AdmissionDisposition]string{
	AdmitBoundedAgentWork: "standard",
	AdmitSupervisedAgent:  "elevated",
	AdmitHumanLed:         "human",
	AdmitDiscoveryOnly:    "human",
	AdmitBlocked:          "human",
}

// DispositionRiskInput returns the risk-input verdict a disposition maps to. A value
// outside the vocabulary maps to "human", the most gated verdict.
func DispositionRiskInput(d AdmissionDisposition) string {
	if v, ok := dispositionRiskInput[d]; ok {
		return v
	}
	return "human"
}

// HardCheck names one deterministic readiness fact (GEA-09). Category opt-in, the
// remaining GEA-09 hard check, is not a supplied fact: it is derived from the policy's
// own category record, so an assessment cannot assert its own admission.
type HardCheck string

const (
	HardAuthority            HardCheck = "authority"
	HardDataHandling         HardCheck = "data-handling"
	HardAcceptance           HardCheck = "acceptance-availability"
	HardExecutorCapability   HardCheck = "executor-capability"
	HardIndependentVerifier  HardCheck = "independent-verifier"
	HardBudget               HardCheck = "budget"
	HardEffectRecoverability HardCheck = "effect-recoverability"
)

// authorityChecks fail CLOSED to `blocked` on fail or could-not-check: without authority
// or data-handling permission not even discovery may run.
var authorityChecks = []HardCheck{HardAuthority, HardDataHandling}

// readinessChecks gate implementation. A failure caps at the policy's declared ceiling;
// a could-not-check holds implementation (at most separately authorized discovery).
var readinessChecks = []HardCheck{
	HardAcceptance,
	HardExecutorCapability,
	HardIndependentVerifier,
	HardBudget,
	HardEffectRecoverability,
}

// FactState is the three-state result of a hard check. could-not-check is reported as
// itself: never rounded up to pass, never rounded down to a fail it did not observe.
type FactState string

const (
	FactPass          FactState = "pass"
	FactFail          FactState = "fail"
	FactCouldNotCheck FactState = "could-not-check"
)

// HardFact is one observed deterministic fact about the subject.
type HardFact struct {
	Check      HardCheck `json:"check"`
	State      FactState `json:"state"`
	Evidence   []string  `json:"evidence,omitempty"`
	ObservedAt time.Time `json:"observedAt"`
}

// AdvisoryDimension names one probabilistic assessment dimension (GEA-09).
type AdvisoryDimension string

const (
	AdviseAmbiguity            AdvisoryDimension = "ambiguity"
	AdviseVerificationAdequacy AdvisoryDimension = "verification-adequacy"
	AdviseTaskModelFit         AdvisoryDimension = "task-model-fit"
	AdviseSemanticRisk         AdvisoryDimension = "semantic-risk"
)

var advisoryDimensions = map[AdvisoryDimension]bool{
	AdviseAmbiguity:            true,
	AdviseVerificationAdequacy: true,
	AdviseTaskModelFit:         true,
	AdviseSemanticRisk:         true,
}

// AdvisoryAssessment is one RECORDED piece of probabilistic advice: the request it
// answered, the Prediction envelope (decisionassessment.go), the subject scope it was
// produced for, and when. The evaluator never calls a provider; it reads this record.
type AdvisoryAssessment struct {
	Dimension  AdvisoryDimension `json:"dimension"`
	Scope      string            `json:"scope"`
	Request    AssessmentRequest `json:"request"`
	Prediction Prediction        `json:"prediction"`
	ObservedAt time.Time         `json:"observedAt"`
}

// AgenticAssessment is the suitability record for one subject (GEA-09, GEA-11): the
// deterministic facts and the probabilistic advice, kept in separate fields, bound to
// the subject revision, schema and environment they were observed against.
type AgenticAssessment struct {
	Subject           string               `json:"subject"`
	SubjectRevision   string               `json:"subjectRevision"`
	SchemaDigest      string               `json:"schemaDigest"`
	EnvironmentDigest string               `json:"environmentDigest"`
	Category          string               `json:"category"`
	Operations        []string             `json:"operations"`
	Facts             []HardFact           `json:"facts"`
	Advice            []AdvisoryAssessment `json:"advice,omitempty"`
	AssessedAt        time.Time            `json:"assessedAt"`
}

// CategoryAdmission is the OWNER's admission of a category of work: who owns it, the
// most permissive disposition the owner admits (Ceiling), the agent operations the
// category permits, and whether it has been revoked. Advice can only restrict within it.
type CategoryAdmission struct {
	Owner               string               `json:"owner"`
	Ceiling             AdmissionDisposition `json:"ceiling"`
	PermittedOperations []string             `json:"permittedOperations"`
	Revoked             bool                 `json:"revoked,omitempty"`
	RevokedReason       string               `json:"revokedReason,omitempty"`
}

// AdmissionException admits ONE subject outside the policy's declared applicability, with
// a named owner and an expiry. It extends applicability only: it never waives a hard check,
// never lifts a category ceiling, and never removes a gate.
type AdmissionException struct {
	Subject string    `json:"subject"`
	Owner   string    `json:"owner"`
	Reason  string    `json:"reason"`
	Expires time.Time `json:"expires"`
}

// AdmissionPolicy is the deterministic policy record. Its VALUES are an owner's ruling;
// this package ships the shape and the validation, and fixtures use example values only.
type AdmissionPolicy struct {
	Version       string                       `json:"version"`
	Owner         string                       `json:"owner"`
	Applicability []string                     `json:"applicability"`
	Exceptions    []AdmissionException         `json:"exceptions,omitempty"`
	Categories    map[string]CategoryAdmission `json:"categories"`
	// ReadinessFailCeiling caps the disposition when a readiness check FAILS. Every
	// readiness check MUST be mapped, and only to human-led or more restrictive.
	ReadinessFailCeiling map[HardCheck]AdmissionDisposition `json:"readinessFailCeiling"`
	// MaxFactAge / MaxAdviceAge bound freshness; an older observation is could-not-check
	// (a fact) or ignored (advice).
	MaxFactAge   Duration `json:"maxFactAge"`
	MaxAdviceAge Duration `json:"maxAdviceAge"`
}

// Duration is a time.Duration that reads and writes as a Go duration string ("72h") in
// JSON, so a policy file states its freshness bounds legibly.
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler.
func (d *Duration) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	v, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("admission: duration %q: %w", s, err)
	}
	*d = Duration(v)
	return nil
}

// MarshalJSON implements json.Marshaler.
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}

// DiscoveryScope is a SEPARATE authorization for discovery-only work: who granted it and
// which read scope it covers. Absent (nil) means discovery is not authorized. A grant
// authorizes discovery of a subject only when its owner is a reason-code token and at
// least one read-scope entry covers the subject (scopeCovers); a blank entry voids it.
type DiscoveryScope struct {
	Owner     string   `json:"owner"`
	ReadScope []string `json:"readScope"`
}

// AdmissionContext is the evaluation-time state: the clock, the CURRENT subject
// revision, schema and environment (compared against what the assessment recorded), the
// stop flag, the advice kill switch and any separately authorized discovery grant.
type AdmissionContext struct {
	Now               time.Time       `json:"now"`
	SubjectRevision   string          `json:"subjectRevision"`
	SchemaDigest      string          `json:"schemaDigest"`
	EnvironmentDigest string          `json:"environmentDigest"`
	StopFlag          bool            `json:"stopFlag,omitempty"`
	AdviceDisabled    bool            `json:"adviceDisabled,omitempty"`
	Discovery         *DiscoveryScope `json:"discovery,omitempty"`
}

// AdmissionResult is the policy's deterministic output for one subject.
type AdmissionResult struct {
	// Subject is the assessed subject when it passed the subject grammar, else empty.
	Subject             string               `json:"subject"`
	PolicyVersion       string               `json:"policyVersion"`
	Disposition         AdmissionDisposition `json:"disposition"`
	Reasons             []string             `json:"reasons"`
	PermittedOperations []string             `json:"permittedOperations"`
	HumanFloors         []string             `json:"humanFloors"`
	RiskInput           string               `json:"riskInput"`
	AdviceApplied       []string             `json:"adviceApplied,omitempty"`
	// DiscoveryScope is set only on a `discovery-only` result: the grant's read-scope
	// entries that cover the subject, so a consumer can enforce the scope afterwards.
	DiscoveryScope []string `json:"discoveryScope,omitempty"`
}

// Input bounds. The evaluator refuses (blocked, `input-oversized` / `subject-invalid`)
// an assessment beyond them rather than evaluating it; the schema states the same bounds.
const (
	admissionMaxSubject    = 256
	admissionMaxOperations = 16
	admissionMaxFacts      = 32
	admissionMaxAdvice     = 16
	admissionMaxPatternLen = 1 << 20
)

// reasonTokenRE is the grammar of every non-literal value a reason code may carry (an
// exception or discovery-grant owner). It excludes whitespace, control characters and
// the `;` that PolicyResult joins reason codes with, so an echoed value cannot add a
// line or a reason code of its own.
var reasonTokenRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@/-]{0,63}$`)

// subjectValid reports whether a subject is non-empty, bounded and free of control
// characters and whitespace.
func subjectValid(s string) bool {
	if s == "" || len(s) > admissionMaxSubject {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// scopeCovers reports whether a scope entry (an applicability prefix or a discovery read
// scope) covers the subject. It is boundary-aware: the subject equals the entry, or
// extends it past a `#` or `/` boundary — so `example-org/widgets` covers
// `example-org/widgets#1` and never `example-org/widgets-evil#1`.
func scopeCovers(scope, subject string) bool {
	if strings.TrimSpace(scope) == "" || !strings.HasPrefix(subject, scope) {
		return false
	}
	if subject == scope {
		return true
	}
	if last := scope[len(scope)-1]; last == '#' || last == '/' {
		return true
	}
	next := subject[len(scope)]
	return next == '#' || next == '/'
}

// discoveryGrant returns the grant's read-scope entries that cover the subject, or the
// reason code that the grant does not authorize discovery of it.
func discoveryGrant(g *DiscoveryScope, subject string) ([]string, string) {
	if g == nil {
		return nil, "discovery-not-authorized"
	}
	if !reasonTokenRE.MatchString(g.Owner) || len(g.ReadScope) == 0 {
		return nil, "discovery-grant-invalid"
	}
	var covered []string
	for _, s := range g.ReadScope {
		if strings.TrimSpace(s) == "" {
			return nil, "discovery-grant-invalid"
		}
		if scopeCovers(s, subject) {
			covered = append(covered, s)
		}
	}
	if len(covered) == 0 {
		return nil, "discovery-out-of-scope"
	}
	sort.Strings(covered)
	return covered, ""
}

// agentOperations is the vocabulary an owner may list in a category's permitted
// operations: `read`, plus the effect kinds the workflow-pattern spec's role table names.
var agentOperations = map[string]bool{
	"read":            true,
	"push":            true,
	"pr-open":         true,
	"comment":         true,
	"review":          true,
	"evidence-commit": true,
	"file-issue":      true,
	"dispatch":        true,
}

// humanFloorOperations are never permitted to an agent at any disposition.
var humanFloorOperations = []string{"merge", "release", "deploy"}

func isHumanFloor(op string) bool {
	for _, f := range humanFloorOperations {
		if op == f {
			return true
		}
	}
	return false
}

// ValidateAdmissionPolicy refuses a policy that could let confidence or a failed check
// open a lane. It returns the FIRST violation as a Refused *DeskError.
func ValidateAdmissionPolicy(p AdmissionPolicy) error {
	if strings.TrimSpace(p.Version) == "" {
		return Refused("admission: policy has no version")
	}
	if strings.TrimSpace(p.Owner) == "" {
		return Refused("admission: policy has no owner")
	}
	if len(p.Applicability) == 0 {
		return Refused("admission: policy declares no applicability")
	}
	for _, a := range p.Applicability {
		if strings.TrimSpace(a) == "" {
			return Refused("admission: policy applicability carries an empty entry (would match every subject)")
		}
	}
	if len(p.Categories) == 0 {
		return Refused("admission: policy admits no category")
	}
	names := make([]string, 0, len(p.Categories))
	for n := range p.Categories {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := p.Categories[n]
		if strings.TrimSpace(c.Owner) == "" {
			return Refused(fmt.Sprintf("admission: category %q has no owner", n))
		}
		if !c.Ceiling.Valid() {
			return Refused(fmt.Sprintf("admission: category %q ceiling %q is not an admission disposition", n, c.Ceiling))
		}
		for _, op := range c.PermittedOperations {
			if isHumanFloor(op) {
				return Refused(fmt.Sprintf("admission: category %q permits %q — a human-floor operation is never an agent operation", n, op))
			}
			if !agentOperations[op] {
				return Refused(fmt.Sprintf("admission: category %q permits unknown operation %q", n, op))
			}
		}
	}
	for _, chk := range readinessChecks {
		ceil, ok := p.ReadinessFailCeiling[chk]
		if !ok {
			return Refused(fmt.Sprintf("admission: readiness check %q has no fail ceiling", chk))
		}
		if !ceil.Valid() || ceil.rank() < AdmitHumanLed.rank() {
			return Refused(fmt.Sprintf(
				"admission: readiness check %q fail ceiling %q is an agent lane — a failed hard check must cap at human-led or more restrictive",
				chk, ceil))
		}
	}
	for chk := range p.ReadinessFailCeiling {
		if !isReadinessCheck(chk) {
			return Refused(fmt.Sprintf("admission: fail ceiling names %q, which is not a readiness check", chk))
		}
	}
	if p.MaxFactAge <= 0 {
		return Refused("admission: policy maxFactAge must be positive")
	}
	if p.MaxAdviceAge <= 0 {
		return Refused("admission: policy maxAdviceAge must be positive")
	}
	for i, e := range p.Exceptions {
		if strings.TrimSpace(e.Subject) == "" || strings.TrimSpace(e.Owner) == "" || e.Expires.IsZero() {
			return Refused(fmt.Sprintf("admission: exception %d must name a subject, an owner and an expiry", i))
		}
		if !reasonTokenRE.MatchString(e.Owner) {
			return Refused(fmt.Sprintf("admission: exception %d owner is not a reason-code token (letters, digits and . _ @ / -, at most 64)", i))
		}
	}
	return nil
}

func isReadinessCheck(c HardCheck) bool {
	for _, r := range readinessChecks {
		if r == c {
			return true
		}
	}
	return false
}

// admission is the evaluator's running state: the current cap and the reasons for it.
type admission struct {
	cap     AdmissionDisposition
	reasons []string
}

func (a *admission) restrict(d AdmissionDisposition, reason string) {
	a.cap = moreRestrictive(a.cap, d)
	a.reasons = append(a.reasons, reason)
}

// EvaluateAgenticAdmission is the deterministic admission policy. It is total and fail-closed:
// an invalid policy, a stop flag, an inapplicable subject or a missing category returns
// `blocked` with the reason, never an error a caller could mistake for "no opinion".
func EvaluateAgenticAdmission(p AdmissionPolicy, a AgenticAssessment, c AdmissionContext) AdmissionResult {
	res := AdmissionResult{
		PolicyVersion: p.Version,
		HumanFloors:   append([]string(nil), humanFloorOperations...),
	}
	st := &admission{cap: AdmitBoundedAgentWork}
	finish := func() AdmissionResult {
		// The record names the subject only when it passed the subject grammar, on every
		// return including the ones that precede the subject check: a refused subject is
		// never echoed into the decision record.
		if subjectValid(a.Subject) {
			res.Subject = a.Subject
		}
		res.Disposition = st.cap
		res.Reasons = st.reasons
		res.RiskInput = DispositionRiskInput(st.cap)
		res.PermittedOperations = permittedFor(st.cap, a.Operations)
		return res
	}

	if err := ValidateAdmissionPolicy(p); err != nil {
		st.restrict(AdmitBlocked, "policy-invalid")
		return finish()
	}
	if c.StopFlag {
		st.restrict(AdmitBlocked, "stop-flag")
		return finish()
	}
	if strings.TrimSpace(a.Subject) == "" {
		st.restrict(AdmitBlocked, "subject-missing")
		return finish()
	}
	if !subjectValid(a.Subject) {
		st.restrict(AdmitBlocked, "subject-invalid")
		return finish()
	}
	if len(a.Operations) > admissionMaxOperations || len(a.Facts) > admissionMaxFacts ||
		len(a.Advice) > admissionMaxAdvice {
		st.restrict(AdmitBlocked, "input-oversized")
		return finish()
	}
	if c.Now.IsZero() {
		st.restrict(AdmitBlocked, "clock-missing")
		return finish()
	}
	// A binding that was never computed cannot be compared: empty on both sides would
	// otherwise read as "unchanged" and bind every fact to nothing.
	for _, b := range []struct{ name, assessed, current string }{
		{"subject-revision", a.SubjectRevision, c.SubjectRevision},
		{"schema-digest", a.SchemaDigest, c.SchemaDigest},
		{"environment-digest", a.EnvironmentDigest, c.EnvironmentDigest},
	} {
		if strings.TrimSpace(b.assessed) == "" || strings.TrimSpace(b.current) == "" {
			st.restrict(AdmitBlocked, "binding-missing:"+b.name)
		}
	}
	if st.cap == AdmitBlocked {
		return finish()
	}

	// Applicability, then the owner-named, expiring exception.
	if !applies(p.Applicability, a.Subject) {
		ex, found := exceptionFor(p.Exceptions, a.Subject)
		switch {
		case !found:
			st.restrict(AdmitBlocked, "not-applicable")
			return finish()
		case !c.Now.Before(ex.Expires):
			st.restrict(AdmitBlocked, "exception-expired:"+ex.Owner)
			return finish()
		default:
			st.reasons = append(st.reasons, "exception-applied:"+ex.Owner)
		}
	}

	// Category opt-in — derived from the owner's record, never asserted by the assessment.
	cat, ok := p.Categories[a.Category]
	if !ok {
		st.restrict(AdmitBlocked, "category-absent")
		return finish()
	}
	if cat.Revoked {
		st.restrict(AdmitBlocked, "category-revoked")
		return finish()
	}
	st.restrict(cat.Ceiling, "category-ceiling:"+string(cat.Ceiling))

	// Binding: facts and advice observed against another revision, schema or environment
	// cannot speak for the current one.
	bindingChanged := false
	if a.SubjectRevision != c.SubjectRevision {
		bindingChanged = true
		st.reasons = append(st.reasons, "subject-changed")
	}
	if a.SchemaDigest != c.SchemaDigest {
		bindingChanged = true
		st.reasons = append(st.reasons, "schema-changed")
	}
	if a.EnvironmentDigest != c.EnvironmentDigest {
		bindingChanged = true
		st.reasons = append(st.reasons, "environment-changed")
	}

	facts := map[HardCheck]HardFact{}
	dup := map[HardCheck]bool{}
	for _, f := range a.Facts {
		if _, seen := facts[f.Check]; seen {
			dup[f.Check] = true
		}
		facts[f.Check] = f
	}
	state := func(chk HardCheck) (FactState, string) {
		f, ok := facts[chk]
		switch {
		case !ok:
			return FactCouldNotCheck, "hard-missing:" + string(chk)
		case dup[chk]:
			return FactCouldNotCheck, "hard-conflicting:" + string(chk)
		case bindingChanged:
			return FactCouldNotCheck, "hard-unbound:" + string(chk)
		case f.ObservedAt.IsZero() || f.ObservedAt.After(c.Now) ||
			c.Now.Sub(f.ObservedAt) > time.Duration(p.MaxFactAge):
			return FactCouldNotCheck, "hard-stale:" + string(chk)
		}
		switch f.State {
		case FactPass:
			return FactPass, ""
		case FactFail:
			return FactFail, "hard-fail:" + string(chk)
		default:
			return FactCouldNotCheck, "hard-unknown:" + string(chk)
		}
	}

	for _, chk := range authorityChecks {
		if s, why := state(chk); s != FactPass {
			st.restrict(AdmitBlocked, why)
		}
	}
	readinessUnknown := false
	for _, chk := range readinessChecks {
		s, why := state(chk)
		switch s {
		case FactFail:
			st.restrict(p.ReadinessFailCeiling[chk], why)
		case FactCouldNotCheck:
			readinessUnknown = true
			st.reasons = append(st.reasons, why)
		}
	}
	// The discovery grant is read once: whichever path reaches discovery-only below, it
	// stands only on a grant whose read scope covers this subject.
	discoveryScope, discoveryWhy := discoveryGrant(c.Discovery, a.Subject)
	discoveryNoted := false
	if readinessUnknown {
		if discoveryWhy == "" {
			st.restrict(AdmitDiscoveryOnly, "discovery-scope:"+c.Discovery.Owner)
			discoveryNoted = true
		} else {
			st.restrict(AdmitBlocked, discoveryWhy)
		}
	}

	// Operations: all-hard-pass is necessary, not sufficient. An operation outside the
	// vocabulary is reported by a fixed code — the assessed string is never echoed.
	if len(a.Operations) == 0 {
		st.restrict(AdmitBlocked, "operations-undeclared")
	}
	permitted := map[string]bool{}
	for _, op := range cat.PermittedOperations {
		permitted[op] = true
	}
	for _, op := range a.Operations {
		switch {
		case isHumanFloor(op):
			st.restrict(AdmitHumanLed, "human-floor:"+op)
		case !agentOperations[op]:
			st.restrict(AdmitHumanLed, "operation-unknown")
		case !permitted[op]:
			st.restrict(AdmitHumanLed, "operation-not-permitted:"+op)
		}
	}

	// Advice: only restricts, only when applicable. Absent, disabled, stale, malformed or
	// uncalibrated advice is ignored with a reason, and the facts-only result above IS the
	// default — so the result with any advice is never less restrictive than without it.
	for _, adv := range a.Advice {
		label, why := applicableAdvice(p, a, c, adv, bindingChanged)
		if why != "" {
			if !advisoryDimensions[adv.Dimension] {
				// Never echo a dimension outside the vocabulary.
				st.reasons = append(st.reasons, "advice-ignored:"+why)
			} else {
				st.reasons = append(st.reasons, "advice-ignored:"+string(adv.Dimension)+":"+why)
			}
			continue
		}
		res.AdviceApplied = append(res.AdviceApplied, string(adv.Dimension)+":"+string(label))
		if moreRestrictive(st.cap, label) != st.cap {
			st.restrict(label, "advice-restricted:"+string(adv.Dimension)+":"+string(label))
		}
	}

	// discovery-only means "scoped reads under a separate grant". A fail ceiling or an
	// advice label can reach it too, so it is re-checked here against the grant: with no
	// grant covering the subject the result is blocked, never a grant-less discovery-only.
	if st.cap == AdmitDiscoveryOnly {
		if discoveryWhy != "" {
			st.restrict(AdmitBlocked, discoveryWhy)
		} else {
			if !discoveryNoted {
				st.reasons = append(st.reasons, "discovery-scope:"+c.Discovery.Owner)
			}
			res.DiscoveryScope = discoveryScope
		}
	}

	return finish()
}

// applicableAdvice returns the advised disposition, or a non-empty reason it is ignored.
func applicableAdvice(p AdmissionPolicy, a AgenticAssessment, c AdmissionContext, adv AdvisoryAssessment, bindingChanged bool) (AdmissionDisposition, string) {
	switch {
	case c.AdviceDisabled:
		return "", "disabled"
	case !advisoryDimensions[adv.Dimension]:
		return "", "unknown-dimension"
	case bindingChanged:
		return "", "binding-changed"
	case adv.Scope != a.Subject || adv.Request.Subject != a.Subject:
		return "", "out-of-scope"
	}
	for _, v := range adv.Request.Vocabulary {
		if !AdmissionDisposition(v).Valid() {
			return "", "vocabulary"
		}
	}
	if err := ValidatePrediction(adv.Request, adv.Prediction); err != nil {
		return "", "malformed"
	}
	if adv.ObservedAt.IsZero() || adv.ObservedAt.After(c.Now) ||
		c.Now.Sub(adv.ObservedAt) > time.Duration(p.MaxAdviceAge) {
		return "", "stale"
	}
	advice, ok := adv.Prediction.ToAdvice()
	if !ok {
		return "", "uncalibrated"
	}
	d := AdmissionDisposition(advice.Answer)
	if !d.Valid() {
		return "", "vocabulary"
	}
	return d, ""
}

func applies(prefixes []string, subject string) bool {
	for _, pre := range prefixes {
		if scopeCovers(pre, subject) {
			return true
		}
	}
	return false
}

func exceptionFor(exs []AdmissionException, subject string) (AdmissionException, bool) {
	for _, e := range exs {
		if e.Subject == subject {
			return e, true
		}
	}
	return AdmissionException{}, false
}

// permittedFor derives the agent operations a disposition permits. Requested operations
// that reached an agent lane are, by construction, all category-permitted and none a
// human floor; `read` is always included in a non-blocked lane.
func permittedFor(d AdmissionDisposition, requested []string) []string {
	switch d {
	case AdmitBoundedAgentWork, AdmitSupervisedAgent:
		set := map[string]bool{"read": true}
		for _, op := range requested {
			if !isHumanFloor(op) && agentOperations[op] {
				set[op] = true
			}
		}
		out := make([]string, 0, len(set))
		for op := range set {
			out = append(out, op)
		}
		sort.Strings(out)
		return out
	case AdmitHumanLed, AdmitDiscoveryOnly:
		return []string{"read"}
	default:
		return []string{}
	}
}

// PolicyResult projects the admission result onto the deterministic PolicyResult record
// (decisionassessment.go): Decision is the disposition, Reason the ordered reason codes.
//
// Subject is projected only when it passes the subject grammar, so a hand-built result
// carrying an invalid subject cannot put it into the record either.
func (r AdmissionResult) PolicyResult(inputDigest string, at time.Time) PolicyResult {
	subject := ""
	if subjectValid(r.Subject) {
		subject = r.Subject
	}
	return PolicyResult{
		Subject:       subject,
		InputDigest:   inputDigest,
		Decision:      string(r.Disposition),
		PolicyVersion: r.PolicyVersion,
		Reason:        strings.Join(r.Reasons, "; "),
		GeneratedAt:   at,
	}
}

// riskVerdicts are the four risk-input verdicts a workflow pattern MUST map.
var riskVerdicts = []string{"low", "standard", "elevated", "human"}

// PatternRiskInput is a workflow pattern's risk-input table: verdict -> gate node ids.
type PatternRiskInput struct {
	Pattern string
	Version int
	Gates   map[string][]string
}

// LoadPatternRiskInput reads a workflow-pattern-v1 file's risk-input table. It refuses a
// file that does not map all four verdicts, or whose gate ids name no node — the
// admission seam's own check, independent of the pattern lint that guards the same file.
func LoadPatternRiskInput(path string) (PatternRiskInput, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return PatternRiskInput{}, Unverifiable("admission: cannot read workflow pattern "+path, err)
	}
	if fi.Size() > admissionMaxPatternLen {
		return PatternRiskInput{}, Refused(fmt.Sprintf("admission: workflow pattern %s is %d bytes, over the %d-byte bound", path, fi.Size(), admissionMaxPatternLen))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return PatternRiskInput{}, Unverifiable("admission: cannot read workflow pattern "+path, err)
	}
	var doc struct {
		Schema    string              `yaml:"schema"`
		Pattern   string              `yaml:"pattern"`
		Version   int                 `yaml:"version"`
		RiskInput map[string][]string `yaml:"risk-input"`
		Nodes     []struct {
			ID string `yaml:"id"`
		} `yaml:"nodes"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return PatternRiskInput{}, Refused(fmt.Sprintf("admission: workflow pattern %s is not valid YAML: %v", path, err))
	}
	if doc.Schema != "workflow-pattern-v1" {
		return PatternRiskInput{}, Refused(fmt.Sprintf("admission: %s is not a workflow-pattern-v1 file (schema %q)", path, doc.Schema))
	}
	nodes := map[string]bool{}
	for _, n := range doc.Nodes {
		nodes[n.ID] = true
	}
	for _, v := range riskVerdicts {
		gates, ok := doc.RiskInput[v]
		if !ok {
			return PatternRiskInput{}, Refused(fmt.Sprintf("admission: %s risk-input does not map verdict %q", path, v))
		}
		for _, g := range gates {
			if !nodes[g] {
				return PatternRiskInput{}, Refused(fmt.Sprintf("admission: %s risk-input %q names gate %q, which is not a node", path, v, g))
			}
		}
	}
	return PatternRiskInput{Pattern: doc.Pattern, Version: doc.Version, Gates: doc.RiskInput}, nil
}

// MandatoryGates returns the gate node ids the work must clear: the UNION of the gates
// the brief's own risk verdict requires and the gates the admission disposition's mapped
// verdict requires. It never subtracts a gate — a high score cannot delete a required
// node. An unmapped verdict on EITHER side is a refusal (the caller holds), never an empty
// contribution — the guarantee is the function's, not only LoadPatternRiskInput's.
// Gates are returned for every disposition, `blocked` included: the caller checks the
// disposition first, and a blocked result proceeds to no gate at all.
func MandatoryGates(ri PatternRiskInput, briefVerdict string, d AdmissionDisposition) ([]string, error) {
	base, ok := ri.Gates[briefVerdict]
	if !ok {
		return nil, Refused(fmt.Sprintf("admission: brief risk verdict %q is not mapped by pattern %q", briefVerdict, ri.Pattern))
	}
	dv := DispositionRiskInput(d)
	extra, ok := ri.Gates[dv]
	if !ok {
		return nil, Refused(fmt.Sprintf("admission: disposition %q verdict %q is not mapped by pattern %q", d, dv, ri.Pattern))
	}
	set := map[string]bool{}
	for _, g := range base {
		set[g] = true
	}
	for _, g := range extra {
		set[g] = true
	}
	out := make([]string, 0, len(set))
	for g := range set {
		out = append(out, g)
	}
	sort.Strings(out)
	return out, nil
}
