// Package workspace is the shared contract for cell-owned workspaces: the identity, request
// idempotency, lifecycle generation and typed outcomes that every caller which creates, attaches
// to or removes a checkout speaks in (spec/cell-workspaces-v1.md, schemas/cell-workspace-v1.json).
//
// The package is pure. It decodes and validates documents, derives identities and digests, and
// checks lifecycle transitions; it performs no Git, filesystem, process, network or clock effect.
// Effects plug in through the ports at the end of this file, which an allocator implements with
// its local Git/filesystem adapter and the caller's existing process, scratch and transport
// owners. The module imports only the standard library, so both Go consumers (the desk tools and
// statusgen) can require it, and it never imports a consumer's internal packages or any
// credential code: an allocation grants no authority.
package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Schema is the one schema version this package reads and writes. After wire integrity is
// checked, a document naming any other version, or none, is refused with ReasonUnknownSchema
// before semantic checks run.
const Schema = "cell-workspace/v1"

// Operation names what a request asks for. Version 1 defines allocation; the other lifecycle
// operations (attach, renew, finish, reconcile, inventory, retirement, adoption, transfer) are
// added by the steps that implement them, and a reader that does not know an operation refuses it.
type Operation string

const OpAllocate Operation = "allocate"

// Operations is the closed set of operations this version accepts.
func Operations() []Operation { return []Operation{OpAllocate} }

// Kind is what a workspace is for. Path and branch name are attributes, never the identity.
type Kind string

const (
	KindDesk   Kind = "desk"   // a role session's home
	KindWorker Kind = "worker" // a branch-capable task attempt
	KindReview Kind = "review" // a frozen review subject
	KindVerify Kind = "verify" // a frozen verification subject
	KindScan   Kind = "scan"   // a scan or reconcile attempt
	KindCheck  Kind = "check"  // a short-lived detached check
)

// Kinds is the closed set of workspace kinds.
func Kinds() []Kind {
	return []Kind{KindDesk, KindWorker, KindReview, KindVerify, KindScan, KindCheck}
}

// State is a workspace's lifecycle state. Refusal, conflict and unknown evidence are operation
// outcomes, never states.
type State string

const (
	StateAllocating State = "allocating" // durable intent reserved before any Git effect
	StateReady      State = "ready"      // Git and identity checks done; eligible for one attach
	StateActive     State = "active"     // one session holds a validated process lease
	StateRetained   State = "retained"   // no writer; work, evidence or uncertainty keeps it
	StateRetiring   State = "retiring"   // exact generation and preservation proof reserved
	StateRetired    State = "retired"    // absence independently verified; tombstone remains
)

// States is the closed set of lifecycle states.
func States() []State {
	return []State{StateAllocating, StateReady, StateActive, StateRetained, StateRetiring, StateRetired}
}

var transitions = map[State][]State{
	StateAllocating: {StateReady, StateRetained, StateRetiring},
	StateReady:      {StateActive, StateRetained, StateRetiring},
	StateActive:     {StateRetained},
	StateRetained:   {StateActive, StateRetiring},
	StateRetiring:   {StateRetired, StateRetained},
	StateRetired:    nil,
}

// CanTransition reports whether the lifecycle allows from -> to.
func CanTransition(from, to State) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Mode is how the checkout holds its subject.
type Mode string

const (
	ModeBranch   Mode = "branch"   // a named branch the holder may commit to
	ModeDetached Mode = "detached" // the exact commit, frozen
)

// Modes is the closed set of checkout modes.
func Modes() []Mode { return []Mode{ModeBranch, ModeDetached} }

// Provenance says whether the allocator created the workspace or adopted an existing one.
type Provenance string

const (
	ProvenanceManaged Provenance = "managed"
	ProvenanceLegacy  Provenance = "legacy"
)

// Provenances is the closed set of provenances.
func Provenances() []Provenance { return []Provenance{ProvenanceManaged, ProvenanceLegacy} }

// RefOwner names the existing owner a reference points into. The workspace record stores
// references only; it is never a second claim, process, scratch or evidence store.
type RefOwner string

const (
	OwnerTask      RefOwner = "task"
	OwnerClaim     RefOwner = "claim"
	OwnerProcess   RefOwner = "process"
	OwnerScratch   RefOwner = "scratch"
	OwnerEvidence  RefOwner = "evidence"
	OwnerRetention RefOwner = "retention"
)

// RefOwners is the closed set of reference owners.
func RefOwners() []RefOwner {
	return []RefOwner{OwnerTask, OwnerClaim, OwnerProcess, OwnerScratch, OwnerEvidence, OwnerRetention}
}

// OutcomeKind is the three-way answer of an operation plus success.
type OutcomeKind string

const (
	OutcomeOK       OutcomeKind = "ok"
	OutcomeRefused  OutcomeKind = "refused"  // the request is not acceptable as sent
	OutcomeConflict OutcomeKind = "conflict" // the request met state it must not overwrite
	OutcomeUnknown  OutcomeKind = "unknown"  // could not check; nothing was decided
)

// OutcomeKinds is the closed set of outcome kinds.
func OutcomeKinds() []OutcomeKind {
	return []OutcomeKind{OutcomeOK, OutcomeRefused, OutcomeConflict, OutcomeUnknown}
}

// Reason names why an operation did not succeed.
type Reason string

const (
	ReasonUnknownSchema         Reason = "unknown-schema"
	ReasonMalformed             Reason = "malformed"
	ReasonInvalid               Reason = "invalid"
	ReasonUnsupportedCapability Reason = "unsupported-capability"
	ReasonPayloadMismatch       Reason = "payload-mismatch"
	ReasonIdentityMismatch      Reason = "identity-mismatch"
	ReasonIllegalTransition     Reason = "illegal-transition"
	ReasonStaleGeneration       Reason = "stale-generation"
	ReasonForeignState          Reason = "foreign-state"
	ReasonBusy                  Reason = "busy"
	ReasonEvidenceUnavailable   Reason = "evidence-unavailable"
)

var reasonKind = map[Reason]OutcomeKind{
	ReasonUnknownSchema:         OutcomeRefused,
	ReasonMalformed:             OutcomeRefused,
	ReasonInvalid:               OutcomeRefused,
	ReasonUnsupportedCapability: OutcomeRefused,
	ReasonPayloadMismatch:       OutcomeRefused,
	ReasonIdentityMismatch:      OutcomeRefused,
	ReasonIllegalTransition:     OutcomeRefused,
	ReasonStaleGeneration:       OutcomeConflict,
	ReasonForeignState:          OutcomeConflict,
	ReasonBusy:                  OutcomeUnknown,
	ReasonEvidenceUnavailable:   OutcomeUnknown,
}

// Reasons is the closed set of reasons, sorted.
func Reasons() []Reason {
	out := make([]Reason, 0, len(reasonKind))
	for r := range reasonKind {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// KindOf returns the outcome kind a reason always carries.
func KindOf(r Reason) (OutcomeKind, bool) {
	k, ok := reasonKind[r]
	return k, ok
}

// Refusal is the typed error every check returns. It names the rule and the field path, never
// the content that failed it.
type Refusal struct {
	Reason Reason
	Field  string
	Msg    string
}

func (r *Refusal) Error() string {
	s := "workspace: " + string(r.Reason)
	if r.Field != "" {
		s += " at " + r.Field
	}
	if r.Msg != "" {
		s += ": " + r.Msg
	}
	return s
}

// ReasonOf extracts the reason of a Refusal anywhere in err's chain.
func ReasonOf(err error) (Reason, bool) {
	var r *Refusal
	if errors.As(err, &r) {
		return r.Reason, true
	}
	return "", false
}

func refuse(reason Reason, field, format string, args ...any) error {
	return &Refusal{Reason: reason, Field: field, Msg: fmt.Sprintf(format, args...)}
}

// CellRef identifies an admitted cell: its durable UUID and the root it is admitted at.
type CellRef struct {
	UUID string `json:"uuid"`
	Root string `json:"root"`
}

// RepoRef is a canonical repository identity: the forge instance and the repository path on it,
// as the admitted topology names them. It never carries a URL, a user or a credential.
type RepoRef struct {
	Forge      string `json:"forge"`
	Repository string `json:"repository"`
}

// SessionRef names the role session a request runs under.
type SessionRef struct {
	Role    string `json:"role"`
	Session string `json:"session"`
	Harness string `json:"harness,omitempty"`
}

// Ref is a reference into an existing owner (a task or claim, a process lease, a scratch run,
// evidence, a retention artifact).
type Ref struct {
	Owner RefOwner `json:"owner"`
	ID    string   `json:"id"`
}

// Subject is the exact commit a checkout holds, pinned before any checkout is created.
type Subject struct {
	Commit string `json:"commit"`
	Mode   Mode   `json:"mode"`
	Branch string `json:"branch,omitempty"`
}

// Request asks for one operation. (cell, repo, kind, task-or-session, attempt, request id) is its
// idempotency key; the payload digest binds the rest.
type Request struct {
	Schema    string      `json:"schema"`
	RequestID string      `json:"request_id"`
	Operation Operation   `json:"operation"`
	Cell      CellRef     `json:"cell"`
	Repo      RepoRef     `json:"repo"`
	Kind      Kind        `json:"kind"`
	Session   *SessionRef `json:"session,omitempty"`
	Task      *Ref        `json:"task,omitempty"`
	AttemptID string      `json:"attempt_id"`
	Subject   Subject     `json:"subject"`
	Require   []string    `json:"require,omitempty"`
}

// Path is where a workspace lives: the platform-native logical path a caller is told, and the
// resolved physical path the effect boundary verified.
type Path struct {
	Logical  string `json:"logical"`
	Physical string `json:"physical"`
}

// GitIdentity is the Git administration identity verified at effect time.
type GitIdentity struct {
	CommonDir string `json:"common_dir"`
	AdminDir  string `json:"admin_dir"`
}

// Record is the durable state of one workspace.
type Record struct {
	Schema         string       `json:"schema"`
	AllocationID   string       `json:"allocation_id"`
	RequestID      string       `json:"request_id"`
	PayloadDigest  string       `json:"payload_digest"`
	Cell           CellRef      `json:"cell"`
	Repo           RepoRef      `json:"repo"`
	RepoKey        string       `json:"repo_key"`
	Kind           Kind         `json:"kind"`
	Session        *SessionRef  `json:"session,omitempty"`
	Task           *Ref         `json:"task,omitempty"`
	AttemptID      string       `json:"attempt_id"`
	Subject        Subject      `json:"subject"`
	Path           Path         `json:"path"`
	Git            *GitIdentity `json:"git,omitempty"`
	State          State        `json:"state"`
	Generation     uint64       `json:"generation"`
	Provenance     Provenance   `json:"provenance"`
	Lease          *Ref         `json:"lease,omitempty"`
	Refs           []Ref        `json:"refs,omitempty"`
	Reason         string       `json:"reason,omitempty"`
	CreatedAt      string       `json:"created_at"`
	TransitionedAt string       `json:"transitioned_at"`
}

// Outcome is an operation's typed answer.
type Outcome struct {
	Kind   OutcomeKind `json:"kind"`
	Reason Reason      `json:"reason,omitempty"`
	Field  string      `json:"field,omitempty"`
}

// Result answers a Request. Only an ok result carries a record, so no failure can hand a caller
// a path to work in.
type Result struct {
	Schema    string  `json:"schema"`
	RequestID string  `json:"request_id"`
	Outcome   Outcome `json:"outcome"`
	Record    *Record `json:"record,omitempty"`
}

var (
	uuidRe       = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	commitRe     = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	digestRe     = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	idRe         = regexp.MustCompile(`^[!-~]{1,256}$`)
	wordRe       = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	forgeRe      = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?(?::[0-9]{1,5})?$`)
	repositoryRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*(?:/[A-Za-z0-9_][A-Za-z0-9._-]*)+$`)
	branchRe     = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._/-]*$`)
	capabilityRe = regexp.MustCompile(`^[a-z][a-z0-9.-]*(?:/v[0-9]+)?$`)
	repoKeyRe    = regexp.MustCompile(`^[a-z0-9-]{1,32}-[0-9a-f]{16}$`)
	absPathRe    = regexp.MustCompile(absPathPattern)
)

// absPathPattern is the whole-path rule of spec section 3, written once so that the schema's
// path patterns can carry the identical string (TestWorkspaceSchemaCoverage compares them): a
// POSIX root not followed by a second separator, or a drive root; no network (UNC) root, so no
// path begins with two separators; no `..` segment between either separator; no control
// character. A segment is any run of permitted characters other than `..` (the first one also
// not empty, which is what keeps a second separator off the root).
//
// The control characters are the characters themselves, not regexp escapes: RE2 and ECMA-262
// both read a literal character in a class as itself, and the JSON schema then carries them as
// ordinary JSON string escapes (backslash, u, four hex digits), so its pattern decodes to this
// exact string.
const (
	pathCtl        = "\x00-\x1f\x7f"
	pathChar       = `[^` + pathCtl + `/\\]`
	pathCharNoDot  = `[^` + pathCtl + `/\\.]`
	pathSegment    = `(\.?` + pathCharNoDot + `|\.\.` + pathChar + `)` + pathChar + `*`
	absPathPattern = `^(/|[A-Za-z]:[\\/])((` + pathSegment + `|\.)([\\/](` + pathSegment + `|\.?))*)?$`
)

// checkSchema is the version gate every validator and wire-valid decoder runs first: a document of
// any other version, or of none, is refused as unknown, never read as this one.
func checkSchema(s string) error {
	if s != Schema {
		return refuse(ReasonUnknownSchema, "schema", "this reader understands %s only", Schema)
	}
	return nil
}

func checkAbs(field, p string) error {
	if p == "" || len(p) > 4096 || !utf8.ValidString(p) || !absPathRe.MatchString(p) {
		return refuse(ReasonInvalid, field, "want an absolute local path without a .. segment or control character")
	}
	return nil
}

func checkID(field, v string) error {
	if !idRe.MatchString(v) {
		return refuse(ReasonInvalid, field, "want 1-256 visible ASCII characters")
	}
	return nil
}

func (c CellRef) validate(at string) error {
	if !uuidRe.MatchString(c.UUID) {
		return refuse(ReasonInvalid, at+".uuid", "want a lowercase RFC 9562 UUID")
	}
	return checkAbs(at+".root", c.Root)
}

func (r RepoRef) validate(at string) error {
	if !forgeRe.MatchString(r.Forge) {
		return refuse(ReasonInvalid, at+".forge", "want a lowercase forge host")
	}
	if !repositoryRe.MatchString(r.Repository) || strings.HasSuffix(r.Repository, ".git") || strings.Contains(r.Repository, "..") {
		return refuse(ReasonInvalid, at+".repository", "want a canonical owner/name path")
	}
	return nil
}

func (s *SessionRef) validate(at string) error {
	if !wordRe.MatchString(s.Role) || len(s.Role) > 63 {
		return refuse(ReasonInvalid, at+".role", "want lowercase words joined by hyphens")
	}
	if err := checkID(at+".session", s.Session); err != nil {
		return err
	}
	if s.Harness != "" && !wordRe.MatchString(s.Harness) {
		return refuse(ReasonInvalid, at+".harness", "want lowercase words joined by hyphens")
	}
	return nil
}

func (r Ref) validate(at string, allowed ...RefOwner) error {
	ok := false
	for _, o := range allowed {
		if r.Owner == o {
			ok = true
		}
	}
	if !ok {
		return refuse(ReasonInvalid, at+".owner", "owner %q not allowed here", r.Owner)
	}
	return checkID(at+".id", r.ID)
}

func (s Subject) validate(at string, kind Kind) error {
	if !commitRe.MatchString(s.Commit) {
		return refuse(ReasonInvalid, at+".commit", "want a full lowercase commit id")
	}
	switch s.Mode {
	case ModeBranch:
		switch kind {
		case KindReview, KindVerify, KindCheck:
			return refuse(ReasonInvalid, at+".mode", "a %s workspace holds a detached subject", kind)
		}
		b := s.Branch
		if !branchRe.MatchString(b) || strings.Contains(b, "..") || strings.Contains(b, "//") ||
			strings.HasSuffix(b, "/") || strings.HasSuffix(b, ".lock") || strings.HasSuffix(b, ".") {
			return refuse(ReasonInvalid, at+".branch", "want a valid branch name")
		}
	case ModeDetached:
		if s.Branch != "" {
			return refuse(ReasonInvalid, at+".branch", "a detached subject names no branch")
		}
	default:
		return refuse(ReasonInvalid, at+".mode", "unknown mode %q", s.Mode)
	}
	return nil
}

func validKind(k Kind) bool {
	for _, x := range Kinds() {
		if k == x {
			return true
		}
	}
	return false
}

// validateOwnerScope holds the task-or-session rule: a desk home belongs to a role session and
// names no task; every other kind is a task attempt and names its task or claim.
func validateOwnerScope(kind Kind, session *SessionRef, task *Ref) error {
	if !validKind(kind) {
		return refuse(ReasonInvalid, "kind", "unknown kind %q", kind)
	}
	if session != nil {
		if err := session.validate("session"); err != nil {
			return err
		}
	}
	if kind == KindDesk {
		if session == nil {
			return refuse(ReasonInvalid, "session", "a desk workspace needs its role session")
		}
		if task != nil {
			return refuse(ReasonInvalid, "task", "a desk workspace names no task")
		}
		return nil
	}
	if task == nil {
		return refuse(ReasonInvalid, "task", "a %s workspace needs its task or claim reference", kind)
	}
	return task.validate("task", OwnerTask, OwnerClaim)
}

// Validate checks a request against every rule of the contract.
func (q Request) Validate() error {
	if err := checkSchema(q.Schema); err != nil {
		return err
	}
	if err := checkID("request_id", q.RequestID); err != nil {
		return err
	}
	if q.Operation != OpAllocate {
		return refuse(ReasonUnsupportedCapability, "operation", "operation %q is not defined by %s", q.Operation, Schema)
	}
	if err := q.Cell.validate("cell"); err != nil {
		return err
	}
	if err := q.Repo.validate("repo"); err != nil {
		return err
	}
	if err := validateOwnerScope(q.Kind, q.Session, q.Task); err != nil {
		return err
	}
	if err := checkID("attempt_id", q.AttemptID); err != nil {
		return err
	}
	if err := q.Subject.validate("subject", q.Kind); err != nil {
		return err
	}
	seen := map[string]bool{}
	for i, c := range q.Require {
		f := fmt.Sprintf("require[%d]", i)
		if !capabilityRe.MatchString(c) {
			return refuse(ReasonInvalid, f, "want a capability name")
		}
		if seen[c] {
			return refuse(ReasonInvalid, f, "capability listed twice")
		}
		seen[c] = true
	}
	return nil
}

// Admit refuses a request that requires a capability the serving implementation does not
// declare. It never downgrades the request.
func Admit(q Request, supported []string) error {
	have := map[string]bool{}
	for _, s := range supported {
		have[s] = true
	}
	for i, c := range q.Require {
		if !have[c] {
			return refuse(ReasonUnsupportedCapability, fmt.Sprintf("require[%d]", i), "capability %q is not supported", c)
		}
	}
	return nil
}

// Key is the idempotency key of an allocation.
type Key struct {
	Cell      string `json:"cell"`
	RepoKey   string `json:"repo_key"`
	Kind      Kind   `json:"kind"`
	Scope     string `json:"scope"`
	AttemptID string `json:"attempt_id"`
	RequestID string `json:"request_id"`
}

func scopeOf(kind Kind, session *SessionRef, task *Ref) string {
	if kind == KindDesk && session != nil {
		return "session:" + session.Session
	}
	if task != nil {
		return string(task.Owner) + ":" + task.ID
	}
	return ""
}

// Key returns the request's idempotency key.
func (q Request) Key() Key {
	return Key{Cell: q.Cell.UUID, RepoKey: RepoKey(q.Repo), Kind: q.Kind,
		Scope: scopeOf(q.Kind, q.Session, q.Task), AttemptID: q.AttemptID, RequestID: q.RequestID}
}

// Key returns the idempotency key the record was allocated under.
func (r Record) Key() Key {
	return Key{Cell: r.Cell.UUID, RepoKey: r.RepoKey, Kind: r.Kind,
		Scope: scopeOf(r.Kind, r.Session, r.Task), AttemptID: r.AttemptID, RequestID: r.RequestID}
}

// Digest is the payload digest of a request: sha256 over its canonical encoding (spec section
// 4.2). Two requests with equal digests are the same request, however their documents were
// spaced or their strings escaped.
func (q Request) Digest() string {
	b, err := canonical(q)
	if err != nil {
		panic(err) // unreachable: every field is a string, slice of strings or struct of them
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// canonical is the canonical encoding of spec section 4.2: members in schema (struct field)
// order, absent optional members omitted, no insignificant whitespace and nothing after the
// value, and strings escaped only where JSON requires it. json.Marshal is not that encoding: it
// also escapes <, > and & (HTML safety) and U+2028/U+2029, which the identifiers and paths of
// this contract may carry, so a digest over its output could not be reproduced from the spec.
func canonical(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encode terminates the value with exactly one newline, which is not part of the encoding.
	b := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	// The encoder escapes U+2028 and U+2029 even with HTML escaping off; write them as UTF-8.
	// Every backslash in the encoder's output opens an escape inside a string, so walking the
	// escapes pairwise never mistakes an escaped backslash followed by "u2028" for one.
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' || i+1 >= len(b) {
			out = append(out, b[i])
			continue
		}
		if b[i+1] == 'u' && i+5 < len(b) {
			switch string(b[i+2 : i+6]) {
			case "2028":
				out, i = utf8.AppendRune(out, 0x2028), i+5
				continue
			case "2029":
				out, i = utf8.AppendRune(out, 0x2029), i+5
				continue
			}
		}
		out = append(out, b[i], b[i+1])
		i++
	}
	return out, nil
}

// Replay decides a request that arrives with the key of an existing record: the same payload
// is the same allocation (nil: return the record), a changed payload is refused.
func Replay(prior Record, q Request) error {
	if prior.Key() != q.Key() {
		return refuse(ReasonInvalid, "", "replay of a request under a different key")
	}
	if prior.PayloadDigest != q.Digest() {
		return refuse(ReasonPayloadMismatch, "", "the key was first used with a different payload")
	}
	return nil
}

// RepoKey derives the collision-resistant directory key of a repository: its lowercased base
// name for readability, then 16 hex digits of sha256(forge NUL repository) for identity, so two
// forges or namespaces with the same base name never share a key.
func RepoKey(r RepoRef) string {
	base := r.Repository[strings.LastIndex(r.Repository, "/")+1:]
	var b strings.Builder
	for _, c := range strings.ToLower(base) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
		} else {
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if len(name) > 32 {
		name = strings.TrimRight(name[:32], "-")
	}
	if name == "" {
		name = "repo"
	}
	sum := sha256.Sum256([]byte(r.Forge + "\x00" + r.Repository))
	return name + "-" + hex.EncodeToString(sum[:])[:16]
}

func checkTime(field, v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return t, refuse(ReasonInvalid, field, "want an RFC 3339 time")
	}
	return t, nil
}

// Validate checks a record against every rule of the contract.
func (r Record) Validate() error {
	if err := checkSchema(r.Schema); err != nil {
		return err
	}
	if !uuidRe.MatchString(r.AllocationID) {
		return refuse(ReasonInvalid, "allocation_id", "want a lowercase RFC 9562 UUID")
	}
	if err := checkID("request_id", r.RequestID); err != nil {
		return err
	}
	if !digestRe.MatchString(r.PayloadDigest) {
		return refuse(ReasonInvalid, "payload_digest", "want sha256:<64 lowercase hex>")
	}
	if err := r.Cell.validate("cell"); err != nil {
		return err
	}
	if err := r.Repo.validate("repo"); err != nil {
		return err
	}
	if !repoKeyRe.MatchString(r.RepoKey) || r.RepoKey != RepoKey(r.Repo) {
		return refuse(ReasonIdentityMismatch, "repo_key", "does not derive from repo")
	}
	if err := validateOwnerScope(r.Kind, r.Session, r.Task); err != nil {
		return err
	}
	if err := checkID("attempt_id", r.AttemptID); err != nil {
		return err
	}
	if err := r.Subject.validate("subject", r.Kind); err != nil {
		return err
	}
	if err := checkAbs("path.logical", r.Path.Logical); err != nil {
		return err
	}
	if err := checkAbs("path.physical", r.Path.Physical); err != nil {
		return err
	}
	known := false
	for _, s := range States() {
		known = known || r.State == s
	}
	if !known {
		return refuse(ReasonInvalid, "state", "unknown state %q", r.State)
	}
	if r.Git == nil && r.State != StateAllocating {
		return refuse(ReasonInvalid, "git", "a %s workspace carries its verified Git identity", r.State)
	}
	if r.Git != nil {
		if err := checkAbs("git.common_dir", r.Git.CommonDir); err != nil {
			return err
		}
		if err := checkAbs("git.admin_dir", r.Git.AdminDir); err != nil {
			return err
		}
	}
	if r.Generation < 1 {
		return refuse(ReasonInvalid, "generation", "want at least 1")
	}
	switch r.Provenance {
	case ProvenanceManaged:
	case ProvenanceLegacy:
		if r.State == StateAllocating || r.State == StateReady {
			return refuse(ReasonInvalid, "provenance", "a legacy workspace was never allocated here")
		}
	default:
		return refuse(ReasonInvalid, "provenance", "unknown provenance %q", r.Provenance)
	}
	if r.Lease != nil {
		if err := r.Lease.validate("lease", OwnerProcess); err != nil {
			return err
		}
	} else if r.State == StateActive {
		return refuse(ReasonInvalid, "lease", "an active workspace is held by a process lease")
	}
	seen := map[Ref]bool{}
	for i, ref := range r.Refs {
		f := fmt.Sprintf("refs[%d]", i)
		if err := ref.validate(f, OwnerScratch, OwnerEvidence, OwnerRetention); err != nil {
			return err
		}
		if seen[ref] {
			return refuse(ReasonInvalid, f, "reference listed twice")
		}
		seen[ref] = true
	}
	if r.State == StateRetained && r.Reason == "" {
		return refuse(ReasonInvalid, "reason", "retained work names why it is retained")
	}
	if r.Reason != "" && !wordRe.MatchString(r.Reason) {
		return refuse(ReasonInvalid, "reason", "want lowercase words joined by hyphens")
	}
	created, err := checkTime("created_at", r.CreatedAt)
	if err != nil {
		return err
	}
	moved, err := checkTime("transitioned_at", r.TransitionedAt)
	if err != nil {
		return err
	}
	if moved.Before(created) {
		return refuse(ReasonInvalid, "transitioned_at", "earlier than created_at")
	}
	return nil
}

// Advance moves a record to state to, under a compare-and-set on its generation: expected must
// be the generation the caller read. The result has the next generation. The caller validates
// the result (for example that an active record carries its lease) before publishing it.
func Advance(r Record, expected uint64, to State, at time.Time) (Record, error) {
	if r.Generation != expected {
		return r, refuse(ReasonStaleGeneration, "generation", "record is at %d, caller read %d", r.Generation, expected)
	}
	if !CanTransition(r.State, to) {
		return r, refuse(ReasonIllegalTransition, "state", "%s cannot become %s", r.State, to)
	}
	r.State = to
	r.Generation++
	r.TransitionedAt = at.UTC().Format(time.RFC3339)
	return r, nil
}

// Validate checks a result: an ok result carries a valid record and no reason; any other
// outcome carries the reason its kind is defined by, and no record.
func (s Result) Validate() error {
	if err := checkSchema(s.Schema); err != nil {
		return err
	}
	if err := checkID("request_id", s.RequestID); err != nil {
		return err
	}
	if s.Outcome.Field != "" && len(s.Outcome.Field) > 256 {
		return refuse(ReasonInvalid, "outcome.field", "too long")
	}
	if s.Outcome.Kind == OutcomeOK {
		if s.Outcome.Reason != "" || s.Outcome.Field != "" {
			return refuse(ReasonInvalid, "outcome", "an ok outcome carries no reason")
		}
		if s.Record == nil {
			return refuse(ReasonInvalid, "record", "an ok outcome carries the record it observed")
		}
		return s.Record.Validate()
	}
	want, ok := KindOf(s.Outcome.Reason)
	if !ok {
		return refuse(ReasonInvalid, "outcome.reason", "unknown reason %q", s.Outcome.Reason)
	}
	if want != s.Outcome.Kind {
		return refuse(ReasonInvalid, "outcome.kind", "reason %s is a %s outcome", s.Outcome.Reason, want)
	}
	if s.Record != nil {
		return refuse(ReasonInvalid, "record", "a %s outcome hands out no workspace", s.Outcome.Kind)
	}
	return nil
}

// ResultOf maps an error from this package (or an adapter's Refusal) to the result a caller
// returns; any other error is unknown, never a refusal or success.
func ResultOf(requestID string, err error) Result {
	var r *Refusal
	if errors.As(err, &r) {
		if k, ok := KindOf(r.Reason); ok {
			return Result{Schema: Schema, RequestID: requestID, Outcome: Outcome{Kind: k, Reason: r.Reason, Field: r.Field}}
		}
	}
	return Result{Schema: Schema, RequestID: requestID, Outcome: Outcome{Kind: OutcomeUnknown, Reason: ReasonEvidenceUnavailable}}
}

// DecodeRequest strictly decodes and validates a request document.
func DecodeRequest(data []byte) (Request, error) {
	var q Request
	if err := decode(data, &q); err != nil {
		return Request{}, err
	}
	return q, q.Validate()
}

// DecodeRecord strictly decodes and validates a record document.
func DecodeRecord(data []byte) (Record, error) {
	var r Record
	if err := decode(data, &r); err != nil {
		return Record{}, err
	}
	return r, r.Validate()
}

// DecodeResult strictly decodes and validates a result document.
func DecodeResult(data []byte) (Result, error) {
	var s Result
	if err := decode(data, &s); err != nil {
		return Result{}, err
	}
	return s, s.Validate()
}

// checkWire prevents encoding/json's documented replacement of invalid UTF-8 and
// unpaired UTF-16 escapes with U+FFFD. json.Valid checks syntax without decoding
// string tokens; this pass then checks the original bytes, including object keys.
// All three public decoders pass here before even the schema is decoded.
func checkWire(data []byte) error {
	if !utf8.Valid(data) || !json.Valid(data) {
		return refuse(ReasonMalformed, "", "want a UTF-8 JSON document")
	}
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' {
			continue
		}
		// Valid JSON guarantees a complete escape here. Consume escaped backslashes
		// as a pair, so the literal text \\ud800 is never treated as a surrogate.
		if data[i+1] != 'u' {
			i++
			continue
		}
		code, err := strconv.ParseUint(string(data[i+2:i+6]), 16, 16)
		if err != nil {
			return refuse(ReasonMalformed, "", "invalid Unicode escape")
		}
		i += 5
		if code < 0xd800 || code > 0xdfff {
			continue
		}
		if code > 0xdbff || i+6 >= len(data) || data[i+1] != '\\' || data[i+2] != 'u' {
			return refuse(ReasonMalformed, "", "unpaired Unicode surrogate")
		}
		low, err := strconv.ParseUint(string(data[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return refuse(ReasonMalformed, "", "unpaired Unicode surrogate")
		}
		i += 6
	}
	return nil
}

// decode reads one document under the strict rules: after wire integrity, the schema is checked
// first, so a newer version is refused as unknown rather than as malformed; then no repeated
// key at any depth, no
// key outside the defined fields, no key differing from a field name only in case, no null or
// empty optional field, numbers in canonical form, and nothing after the value. The last group is
// one check: the document must equal its own re-encoding.
func decode(data []byte, v any) error {
	if err := checkWire(data); err != nil {
		return err
	}
	var head map[string]json.RawMessage
	if err := json.Unmarshal(data, &head); err != nil {
		return refuse(ReasonMalformed, "", "not a JSON object")
	}
	var schema string
	if raw, ok := head["schema"]; ok && json.Unmarshal(raw, &schema) != nil {
		return refuse(ReasonUnknownSchema, "schema", "not a schema name")
	}
	if err := checkSchema(schema); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	in, err := walk(dec, "$")
	if err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return refuse(ReasonMalformed, "", "trailing data after the document")
	}
	strict := json.NewDecoder(bytes.NewReader(data))
	strict.DisallowUnknownFields()
	if err := strict.Decode(v); err != nil {
		return refuse(ReasonMalformed, "", "%v", err)
	}
	again, err := canonical(v)
	if err != nil {
		return refuse(ReasonMalformed, "", "%v", err)
	}
	back := json.NewDecoder(bytes.NewReader(again))
	back.UseNumber()
	out, err := walk(back, "$")
	if err != nil {
		return err
	}
	if p := firstDiff(in, out, "$"); p != "" {
		return refuse(ReasonMalformed, p, "not canonical: a key differs in case, a field is null or empty, or a number is not in canonical form")
	}
	return nil
}

// walk reads one JSON value, refusing a repeated object key at any depth.
func walk(dec *json.Decoder, at string) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, refuse(ReasonMalformed, at, "invalid JSON")
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch d {
	case '{':
		obj := map[string]any{}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, refuse(ReasonMalformed, at, "invalid JSON")
			}
			k, _ := kt.(string)
			if _, dup := obj[k]; dup {
				return nil, refuse(ReasonMalformed, at+"."+k, "key repeated")
			}
			if obj[k], err = walk(dec, at+"."+k); err != nil {
				return nil, err
			}
		}
		_, err = dec.Token()
		return obj, err
	case '[':
		var arr []any
		for i := 0; dec.More(); i++ {
			e, err := walk(dec, fmt.Sprintf("%s[%d]", at, i))
			if err != nil {
				return nil, err
			}
			arr = append(arr, e)
		}
		_, err = dec.Token()
		return arr, err
	}
	return nil, refuse(ReasonMalformed, at, "unexpected delimiter")
}

func firstDiff(a, b any, at string) string {
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if aok && bok {
		keys := make([]string, 0, len(am)+len(bm))
		for k := range am {
			keys = append(keys, k)
		}
		for k := range bm {
			if _, ok := am[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			_, ina := am[k]
			_, inb := bm[k]
			if ina != inb {
				return at + "." + k
			}
			if p := firstDiff(am[k], bm[k], at+"."+k); p != "" {
				return p
			}
		}
		return ""
	}
	if !reflect.DeepEqual(a, b) {
		return at
	}
	return ""
}

// The ports below are how effects plug in. An allocator implements Effects with its local
// Git/filesystem adapter and composes the caller's existing owners for Journal storage, process
// leases and scratch snapshots. Each method reports what it observed; an error that is not a
// Refusal is unknown, never success. None of these carries or mints a credential.

// Presence is a three-state observation of whether a workspace exists.
type Presence string

const (
	Present Presence = "present"
	Absent  Presence = "absent"
	Unsure  Presence = "unknown"
)

// Observation is what the effect boundary saw, independent of what the record says.
type Observation struct {
	Presence Presence
	Path     Path
	Git      *GitIdentity
	Commit   string
}

// Effects performs and observes local Git/filesystem effects for one record at one generation.
// Create and Remove re-verify path, common-dir and admin identity at effect time and refuse
// with ReasonIdentityMismatch or ReasonForeignState rather than overwrite.
type Effects interface {
	Create(ctx context.Context, r Record) (Observation, error)
	Observe(ctx context.Context, r Record) (Observation, error)
	Remove(ctx context.Context, r Record) (Observation, error)
}

// Journal stores records under a per-(cell, repository) exclusive lock. Lock returns
// ReasonBusy when it cannot acquire within its bound; Put refuses with ReasonStaleGeneration
// when the stored generation is not expected.
type Journal interface {
	Lock(ctx context.Context, cell, repoKey string) (unlock func() error, err error)
	Get(ctx context.Context, k Key) (r Record, found bool, err error)
	Put(ctx context.Context, r Record, expected uint64) error
}

// Liveness is a three-state answer about a process lease.
type Liveness string

const (
	Alive           Liveness = "alive"
	Ended           Liveness = "ended"
	LivenessUnknown Liveness = "unknown"
)

// Leases asks the existing process owner about a lease reference. Unknown retains the workspace.
type Leases interface {
	Liveness(ctx context.Context, lease Ref) (Liveness, error)
}

// Snapshots asks the existing scratch owner for a metadata-free snapshot of an exact commit,
// for a check that needs immutable files rather than a Git checkout.
type Snapshots interface {
	Snapshot(ctx context.Context, cell CellRef, repo RepoRef, commit string) (Ref, error)
}
