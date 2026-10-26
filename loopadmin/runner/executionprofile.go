package runner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
)

// The execution-profile/v1 extension (spec/loop-admin-execution-profile-v1.md,
// schemas/loop-admin-execution-profile-v1.json) rides on v1's ext/require
// negotiation. A request that needs a qualified harness, model and backend
// tuple, or a stated preservation promise, carries a ProfileRequirement under
// ext["execution-profile/v1"] and lists "ext:execution-profile/v1" in require,
// so an adapter that does not know the extension refuses the request instead
// of running it on whatever it has. The legacy resume and snapshot booleans
// keep their v1 meanings and imply nothing here.

// ExecutionProfileExt is the extension's name: the ext key its value travels
// under, and, prefixed with "ext:", the require entry that makes it mandatory.
const ExecutionProfileExt = "execution-profile/v1"

// RequireExecutionProfile is the require entry that makes the extension
// mandatory.
const RequireExecutionProfile = "ext:" + ExecutionProfileExt

// Support is a tri-state declaration. Unknown (could not check) and
// unsupported (checked, and it does not) stay distinct: both refuse a
// mandatory need, with different sentinels, and neither is ever read as the
// other or as supported.
type Support string

const (
	SupportYes     Support = "supported"
	SupportNo      Support = "unsupported"
	SupportUnknown Support = "unknown"
)

// Coverage is a profile's telemetry coverage. It adds unknown to v1's
// none/partial/complete; a requirement asks for partial or complete.
type Coverage string

const (
	CoverageNone     Coverage = "none"
	CoveragePartial  Coverage = "partial"
	CoverageComplete Coverage = "complete"
	CoverageUnknown  Coverage = "unknown"
)

// Preservation states, dimension by dimension, whether that part of a session
// survives an operation. Each dimension is declared on its own: files
// surviving says nothing about the conversation, and a restored conversation
// says nothing about process memory or descendant processes.
type Preservation struct {
	Files               Support `json:"files"`
	Conversation        Support `json:"conversation"`
	ProcessMemory       Support `json:"process_memory"`
	Descendants         Support `json:"descendants"`
	PendingInteractions Support `json:"pending_interactions"`
}

// Operations keys preservation by the lifecycle operation, in the cell
// operator vocabulary: resume (paused -> running, and v1's continuation of a
// pinned session) and restart (drain -> stop -> start).
type Operations struct {
	Resume  Preservation `json:"resume"`
	Restart Preservation `json:"restart"`
}

// Harness binds the harness implementation and version to the immutable
// digest and provenance reference of the harness-component bundle it runs.
type Harness struct {
	Implementation   string `json:"implementation"`
	Version          string `json:"version"`
	BundleDigest     string `json:"bundle_digest"`
	BundleProvenance string `json:"bundle_provenance"`
}

// ModelBinding is the model profile and the model it pins. It is a REQUESTED
// identity: what the provider actually ran is only ever the observation's
// reported model.
type ModelBinding struct {
	Profile string `json:"profile"`
	Model   string `json:"model"`
}

// Backend is the qualified execution backend and platform profile.
type Backend struct {
	Profile  string `json:"profile"`
	Platform string `json:"platform"`
}

// Qualification references the evidence a profile was qualified on and the
// limits that qualification holds within (versions, platforms). The references
// are opaque to the contract; the caller validates them.
type Qualification struct {
	Evidence []string `json:"evidence"`
	Limits   []string `json:"limits"`
}

// QualifiedProfile is what an adapter declares it is qualified to run: the
// harness, model and backend tuple, the runner profile and roles it serves,
// what each operation preserves, cancellation confirmation, budget scopes,
// telemetry coverage and the qualification it rests on. Digest is
// ProfileDigest of everything else, so a declaration whose content drifted
// under an old digest is refused, never matched.
type QualifiedProfile struct {
	Digest        string        `json:"digest"`
	RunnerProfile string        `json:"runner_profile"`
	Roles         []string      `json:"roles"`
	Harness       Harness       `json:"harness"`
	Model         ModelBinding  `json:"model"`
	Backend       Backend       `json:"backend"`
	Preservation  Operations    `json:"preservation"`
	Cancellation  Support       `json:"cancellation"`
	BudgetScopes  []BudgetScope `json:"budget_scopes"`
	Telemetry     Coverage      `json:"telemetry"`
	Qualification Qualification `json:"qualification"`
}

// PreservationNeed names the dimensions a launch cannot do without for one
// operation. An absent dimension is not needed; a needed one must be declared
// supported.
type PreservationNeed struct {
	Files               bool `json:"files,omitempty"`
	Conversation        bool `json:"conversation,omitempty"`
	ProcessMemory       bool `json:"process_memory,omitempty"`
	Descendants         bool `json:"descendants,omitempty"`
	PendingInteractions bool `json:"pending_interactions,omitempty"`
}

// Needs is what the launch requires of the qualified profile beyond the tuple
// binding. The budget scope needed is the request's own budget.scope.
type Needs struct {
	Resume             *PreservationNeed `json:"resume,omitempty"`
	Restart            *PreservationNeed `json:"restart,omitempty"`
	CancelConfirmation bool              `json:"cancel_confirmation,omitempty"`
	Telemetry          Telemetry         `json:"telemetry,omitempty"`
}

// ProfileRequirement is the extension's request value: the qualified profile
// the caller pinned, by digest and by the identities it binds, and what the
// launch needs of it.
type ProfileRequirement struct {
	Digest  string       `json:"digest"`
	Harness Harness      `json:"harness"`
	Model   ModelBinding `json:"model"`
	Backend Backend      `json:"backend"`
	Needs   Needs        `json:"needs"`
}

// ExecutionReport is the extension's adapter value, under a receipt's or an
// observation's ext: the identities the adapter actually observed running, and
// for a resumed launch what it observed restored. It is adapter-reported and so
// untrusted; it grants no scheduling, provider-write or acceptance authority.
// The actual model is the observation's own actual_model.
type ExecutionReport struct {
	Digest   string        `json:"digest"`
	Harness  Harness       `json:"harness"`
	Backend  Backend       `json:"backend"`
	Restored *Preservation `json:"restored,omitempty"`
}

// ProfileDeclarer is implemented by an adapter that knows the extension. An
// adapter that does not implement it declares no qualified profile, so a
// request requiring one is refused before Start.
type ProfileDeclarer interface {
	QualifiedProfiles() []QualifiedProfile
}

var profileDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// ProfileDigest is the digest a qualified profile's Digest must carry: sha256
// over the profile's compact JSON encoding (struct field order, as the spec lists
// the members) with Digest itself empty.
func ProfileDigest(p QualifiedProfile) string {
	p.Digest = ""
	b, _ := json.Marshal(p) // a struct of strings, slices and nested structs always marshals
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// dimensionNames is the dimension table every preservation check walks, in
// dims order, so no dimension is ever checked through another.
var dimensionNames = []fieldPath{"files", "conversation", "process_memory", "descendants", "pending_interactions"}

func (p Preservation) dims() []Support {
	return []Support{p.Files, p.Conversation, p.ProcessMemory, p.Descendants, p.PendingInteractions}
}

func (n PreservationNeed) dims() []bool {
	return []bool{n.Files, n.Conversation, n.ProcessMemory, n.Descendants, n.PendingInteractions}
}

func definedSupport(s Support) bool {
	return s == SupportYes || s == SupportNo || s == SupportUnknown
}

// supports refuses a needed facility that is not declared supported: unknown
// with ErrCapabilityUnknown, anything else with ErrUnsupportedMandatory.
func supports(have Support, what fieldPath) error {
	switch have {
	case SupportYes:
		return nil
	case SupportUnknown:
		return fmt.Errorf("%w: %s", ErrCapabilityUnknown, what)
	}
	return fmt.Errorf("%w: %s is not supported", ErrUnsupportedMandatory, what)
}

// checkPreservation checks each needed dimension against its own declaration.
func checkPreservation(have Preservation, need *PreservationNeed, op fieldPath) error {
	if need == nil {
		return nil
	}
	h, w := have.dims(), need.dims()
	for i := range w {
		if !w[i] {
			continue
		}
		if err := supports(h[i], op+"."+dimensionNames[i]); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks a declared profile's shape and that its digest is the digest
// of its content.
func (p QualifiedProfile) Validate() error {
	if !profileDigest.MatchString(p.Digest) {
		return fmt.Errorf("%w: profile digest is not a sha256 digest", ErrInvalidRequest)
	}
	if blank(p.RunnerProfile) || len(p.Roles) == 0 {
		return fmt.Errorf("%w: a qualified profile names its runner profile and roles", ErrInvalidRequest)
	}
	for _, r := range p.Roles {
		if blank(r) {
			return fmt.Errorf("%w: a qualified profile role is blank", ErrInvalidRequest)
		}
	}
	if err := checkTuple(p.Harness, p.Model, p.Backend); err != nil {
		return err
	}
	for _, d := range append(p.Preservation.Resume.dims(), append(p.Preservation.Restart.dims(), p.Cancellation)...) {
		if !definedSupport(d) {
			return fmt.Errorf("%w: a support declaration is not supported, unsupported or unknown", ErrInvalidRequest)
		}
	}
	for _, s := range p.BudgetScopes {
		if s != BudgetLaunch && s != BudgetRequest {
			return fmt.Errorf("%w: a profile budget scope is not a defined scope", ErrInvalidRequest)
		}
	}
	switch p.Telemetry {
	case CoverageNone, CoveragePartial, CoverageComplete, CoverageUnknown:
	default:
		return fmt.Errorf("%w: profile telemetry is not a defined coverage", ErrInvalidRequest)
	}
	if len(p.Qualification.Evidence) == 0 {
		return fmt.Errorf("%w: a qualified profile references its qualification evidence", ErrInvalidRequest)
	}
	for _, e := range append(cloneSlice(p.Qualification.Evidence), p.Qualification.Limits...) {
		if blank(e) {
			return fmt.Errorf("%w: a qualification reference is blank", ErrInvalidRequest)
		}
	}
	if p.Digest != ProfileDigest(p) {
		return fmt.Errorf("%w: the declared digest is not the digest of the declared profile", ErrProfileMismatch)
	}
	return nil
}

func checkTuple(h Harness, m ModelBinding, b Backend) error {
	for name, v := range map[fieldPath]string{
		"harness.implementation": h.Implementation, "harness.version": h.Version,
		"harness.bundle_provenance": h.BundleProvenance, "model.profile": m.Profile,
		"model.model": m.Model, "backend.profile": b.Profile, "backend.platform": b.Platform,
	} {
		if blank(v) {
			return fmt.Errorf("%w: %s is required", ErrInvalidRequest, name)
		}
	}
	if !profileDigest.MatchString(h.BundleDigest) {
		return fmt.Errorf("%w: harness.bundle_digest is not a sha256 digest", ErrInvalidRequest)
	}
	return nil
}

// Validate checks a requirement's shape.
func (r ProfileRequirement) Validate() error {
	if !profileDigest.MatchString(r.Digest) {
		return fmt.Errorf("%w: digest is not a sha256 digest", ErrInvalidRequest)
	}
	if err := checkTuple(r.Harness, r.Model, r.Backend); err != nil {
		return err
	}
	switch r.Needs.Telemetry {
	case "", TelemetryPartial, TelemetryComplete:
	default:
		return fmt.Errorf("%w: needs.telemetry is partial or complete", ErrInvalidRequest)
	}
	return nil
}

// decodeExtension decodes one extension value as strictly as a request: exact
// keys, no repeated key, no null for a defined field, nothing trailing.
func decodeExtension(raw json.RawMessage, into any, path fieldPath) error {
	if err := scanStrict(raw); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	if err := checkKeys(raw, reflect.TypeOf(into), path); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidRequest, decodeError(err))
	}
	return nil
}

// DecodeProfileRequirement strictly decodes and validates an extension value.
func DecodeProfileRequirement(raw []byte) (ProfileRequirement, error) {
	var r ProfileRequirement
	if err := decodeExtension(raw, &r, "ext.execution-profile/v1"); err != nil {
		return ProfileRequirement{}, err
	}
	if err := r.Validate(); err != nil {
		return ProfileRequirement{}, err
	}
	return r, nil
}

// DecodeQualifiedProfile strictly decodes and validates a profile declaration.
func DecodeQualifiedProfile(raw []byte) (QualifiedProfile, error) {
	var p QualifiedProfile
	if err := decodeExtension(raw, &p, "profile"); err != nil {
		return QualifiedProfile{}, err
	}
	if err := p.Validate(); err != nil {
		return QualifiedProfile{}, err
	}
	return p, nil
}

// ExecutionProfile returns the request's extension value, decoded and
// validated, and whether the request carries one.
func (r LaunchRequest) ExecutionProfile() (ProfileRequirement, bool, error) {
	raw, ok := r.Extensions[ExecutionProfileExt]
	if !ok {
		return ProfileRequirement{}, false, nil
	}
	p, err := DecodeProfileRequirement(raw)
	return p, true, err
}

func (r LaunchRequest) requires(name string) bool {
	for _, n := range r.Require {
		if n == name {
			return true
		}
	}
	return false
}

// governed reports whether the extension applies to this request on an adapter
// with these capabilities: it is required, or it is carried and the adapter
// understands it (an adapter that understands an extension honours it, so it
// is checked whenever it is present). Carried to an adapter that does not
// declare it, an unrequired value is a v1 optional extension and is ignored.
func (r LaunchRequest) governed(c Capabilities) bool {
	_, carried := r.Extensions[ExecutionProfileExt]
	return r.requires(RequireExecutionProfile) || (carried && c.hasExtension(ExecutionProfileExt))
}

// AdmitExecutionProfile is the admission check of the extension: the request's
// pinned qualified profile must be one the adapter declares, the declaration
// must be internally consistent, every identity it binds must be the
// request's, and every need must be declared supported. The Client runs it
// before Start; an adapter that knows the extension runs it again inside its
// own Start, so a client that skipped it still launches nothing unqualified.
func AdmitExecutionProfile(req LaunchRequest, declared []QualifiedProfile) (ProfileRequirement, error) {
	raw, ok := req.Extensions[ExecutionProfileExt]
	if !ok {
		return ProfileRequirement{}, fmt.Errorf("%w: require names the execution profile but ext carries none", ErrInvalidRequest)
	}
	need, err := DecodeProfileRequirement(raw)
	if err != nil {
		return ProfileRequirement{}, err
	}
	var p *QualifiedProfile
	for i := range declared {
		if declared[i].Digest == need.Digest {
			p = &declared[i]
			break
		}
	}
	if p == nil {
		return ProfileRequirement{}, fmt.Errorf("%w: the adapter declares no qualified profile with the pinned digest", ErrUnsupportedMandatory)
	}
	if err := p.Validate(); err != nil {
		return ProfileRequirement{}, err
	}
	if p.Harness != need.Harness || p.Model != need.Model || p.Backend != need.Backend {
		return ProfileRequirement{}, fmt.Errorf("%w: the pinned harness, model or backend is not the qualified one", ErrProfileMismatch)
	}
	if p.RunnerProfile != req.Profile.ID || p.Model.Model != req.Profile.Model {
		return ProfileRequirement{}, fmt.Errorf("%w: the runner profile or pinned model is not the qualified one", ErrProfileMismatch)
	}
	if !contains(p.Roles, req.Packet.Role) {
		return ProfileRequirement{}, fmt.Errorf("%w: the role is not one the profile is qualified for", ErrProfileMismatch)
	}
	if !contains(budgetNames(p.BudgetScopes), string(req.Budget.Scope)) {
		return ProfileRequirement{}, fmt.Errorf("%w: the qualified profile does not enforce the budget's scope", ErrUnsupportedMandatory)
	}
	if need.Needs.CancelConfirmation {
		if err := supports(p.Cancellation, "needs.cancel_confirmation"); err != nil {
			return ProfileRequirement{}, err
		}
	}
	if err := coverageMeets(p.Telemetry, need.Needs.Telemetry); err != nil {
		return ProfileRequirement{}, err
	}
	if err := checkPreservation(p.Preservation.Resume, need.Needs.Resume, "needs.resume"); err != nil {
		return ProfileRequirement{}, err
	}
	if err := checkPreservation(p.Preservation.Restart, need.Needs.Restart, "needs.restart"); err != nil {
		return ProfileRequirement{}, err
	}
	return need, nil
}

func budgetNames(s []BudgetScope) []string {
	out := make([]string, len(s))
	for i, v := range s {
		out[i] = string(v)
	}
	return out
}

func coverageMeets(have Coverage, need Telemetry) error {
	switch need {
	case "":
		return nil
	case TelemetryPartial:
		if have == CoveragePartial || have == CoverageComplete {
			return nil
		}
	case TelemetryComplete:
		if have == CoverageComplete {
			return nil
		}
	}
	if have == CoverageUnknown {
		return fmt.Errorf("%w: needs.telemetry", ErrCapabilityUnknown)
	}
	return fmt.Errorf("%w: needs.telemetry is not covered", ErrUnsupportedMandatory)
}

// checkExecutionProfile is the client's admission step: nothing for a request
// the extension does not govern, AdmitExecutionProfile for one it does.
func (r LaunchRequest) checkExecutionProfile(c Capabilities, declared []QualifiedProfile) (*ProfileRequirement, error) {
	if !r.governed(c) {
		return nil, nil
	}
	need, err := AdmitExecutionProfile(r, declared)
	if err != nil {
		return nil, err
	}
	return &need, nil
}

// ReportExtension encodes a report as the ext map of a receipt or observation.
func ReportExtension(rep ExecutionReport) map[string]json.RawMessage {
	b, _ := json.Marshal(rep) // strings and nested structs always marshal
	return map[string]json.RawMessage{ExecutionProfileExt: b}
}

// executionReport reads the report an adapter put under ext. It reports false
// when there is none; a report that does not decode, or that carries credential
// material, is malformed.
func executionReport(ext map[string]json.RawMessage) (ExecutionReport, bool, error) {
	raw, ok := ext[ExecutionProfileExt]
	if !ok {
		return ExecutionReport{}, false, nil
	}
	if extensionCarriesCredential(ExecutionProfileExt, raw) {
		return ExecutionReport{}, true, fmt.Errorf("%w: in the execution report", ErrCredential)
	}
	var rep ExecutionReport
	if err := decodeExtension(raw, &rep, "ext.execution-profile/v1"); err != nil {
		return ExecutionReport{}, true, fmt.Errorf("%w: the execution report does not decode", ErrMalformedResult)
	}
	return rep, true, nil
}

// substituted reports whether an observed identity differs from the pinned one.
func (rep ExecutionReport) substituted(need ProfileRequirement) bool {
	return rep.Digest != need.Digest || rep.Harness != need.Harness || rep.Backend != need.Backend
}

// checkReport is the acceptance check of a governed invocation's report: it
// must be present, name the pinned tuple, and, for a resumed launch whose
// requirement names what must survive, show those dimensions restored.
// Restoring files is not restoring the conversation or the processes.
func checkReport(req LaunchRequest, need ProfileRequirement, ext map[string]json.RawMessage) (*ExecutionReport, error) {
	rep, ok, err := executionReport(ext)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrProfileUnreported
	}
	if rep.substituted(need) {
		return nil, fmt.Errorf("%w: the observed harness, backend or profile is not the pinned one", ErrProfileMismatch)
	}
	if req.Resume != nil && need.Needs.Resume != nil {
		if rep.Restored == nil {
			return nil, fmt.Errorf("%w: what the resume restored was not reported", ErrCapabilityUnknown)
		}
		if err := checkPreservation(*rep.Restored, need.Needs.Resume, "restored"); err != nil {
			return nil, err
		}
	}
	return &rep, nil
}
