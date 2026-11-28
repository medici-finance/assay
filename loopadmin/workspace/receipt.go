// Package workspace is the prepared-environment/v1 contract
// (spec/prepared-environment-v1.md): the receipt that records what a
// workspace was prepared from and what was observed in it, and a
// prepare/reconcile/release port over injected resource, Git, authority,
// store and retirement adapters.
//
// The package runs no subprocess, opens no network connection, keeps no store
// and embeds no Git engine or scheduler. Every effect goes through an adapter
// the caller supplies. Readiness is decided here, by one validator, against
// what the adapters observe: an adapter (the fakes included) reports state and
// never decides that a workspace is ready.
package workspace

import (
	"errors"
	"fmt"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/loopadmin/runner"
)

// Version is the receipt contract version.
const Version = "prepared-environment/v1"

var (
	// ErrInvalid: a request or receipt does not satisfy the contract's shape.
	ErrInvalid = errors.New("prepared-environment: invalid request or receipt")
	// ErrNotReady: the effective configuration does not match every
	// mandatory input; the receipt records which.
	ErrNotReady = errors.New("prepared-environment: effective configuration does not match")
	// ErrMismatch: a receipt was presented for an invocation, owner,
	// instance or allocation it does not record.
	ErrMismatch = errors.New("prepared-environment: receipt does not belong to this binding")
	// ErrUnauthorized: the authority adapter did not confirm the owner and
	// instance as current. Nothing is prepared or reused.
	ErrUnauthorized = errors.New("prepared-environment: owner or instance is not current")
	// ErrAdapter: an adapter failed; the receipt keeps the outcome it had
	// reached (unknown or partial) for the next attempt to reconcile. The
	// returned error's text is the sentinel's alone, but it unwraps to the
	// adapter's cause for errors.Is and errors.As: callers log only Error(),
	// never an unwrapped cause, which may carry adapter text.
	ErrAdapter = errors.New("prepared-environment: adapter failed")
	// ErrHold: the port refused an action that could lose files or act on an
	// uncertain allocation. A *Hold carries the reason.
	ErrHold = errors.New("prepared-environment: hold")
)

// Invocation is the caller's invocation identity (runner request caller+id).
type Invocation struct {
	Caller string `json:"caller"`
	ID     string `json:"id"`
}

// Owner is the authority the preparation is held under: the runner request's
// authority key and generation, the owner incarnation, and who validated it.
type Owner struct {
	Key         string `json:"key"`
	Generation  uint64 `json:"generation"`
	Incarnation string `json:"incarnation"`
	ValidatedBy string `json:"validated_by"`
}

// Instance names one desk instance: its cell, role, instance id and
// generation. Two instances of one role are two instances; a successor is a
// new generation, so it gets its own binding.
type Instance struct {
	Cell       string `json:"cell"`
	Role       string `json:"role"`
	ID         string `json:"id"`
	Generation uint64 `json:"generation"`
}

// Repository is one repository at an exact revision.
type Repository struct {
	Name     string `json:"name"`
	Revision string `json:"revision"`
}

// Component is a tool or skill at an exact version.
type Component struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Recipe is the preparation recipe.
type Recipe struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Digest  string `json:"digest"`
}

// Environment is everything a cell preparation is built from. Instances of
// one cell with an equal environment may share a preparation.
type Environment struct {
	Recipe        Recipe      `json:"recipe"`
	Tools         []Component `json:"tools"`
	Skills        []Component `json:"skills"`
	HarnessBundle string      `json:"harness_bundle"`
	Image         string      `json:"image,omitempty"`
}

// Policy holds references to the tool and network policies, never their
// content.
type Policy struct {
	Tool    string `json:"tool"`
	Network string `json:"network"`
}

// Restore asks for a new instance workspace restored from a source
// instance's, preserving the named continuity dimensions.
type Restore struct {
	Source Instance                `json:"source"`
	Needs  runner.PreservationNeed `json:"needs"`
}

// Request is every mandatory input of one preparation. Candidate and
// Acceptance are opaque references the port records and never interprets.
type Request struct {
	Invocation   Invocation   `json:"invocation"`
	Owner        Owner        `json:"owner"`
	Instance     Instance     `json:"instance"`
	Repositories []Repository `json:"repositories"`
	Environment  Environment  `json:"environment"`
	Writable     []string     `json:"writable"`
	Policy       Policy       `json:"policy"`
	Candidate    string       `json:"candidate,omitempty"`
	Acceptance   string       `json:"acceptance,omitempty"`
	Restore      *Restore     `json:"restore,omitempty"`
}

// Allocation names the shared cell preparation and the instance's own
// writable workspace. Both are empty in a receipt written before any adapter
// was called.
type Allocation struct {
	Preparation string `json:"preparation,omitempty"`
	Workspace   string `json:"workspace,omitempty"`
}

// Outcome is what the receipt records about the preparation.
type Outcome string

const (
	// OutcomeUnknown: written before the first adapter call, or after an
	// observation could not be made. Reconciled before any retry.
	OutcomeUnknown Outcome = "unknown"
	// OutcomePartial: the workspace was allocated and preparation did not
	// finish. Its files are preserved.
	OutcomePartial Outcome = "partial"
	// OutcomeNotReady: observed, and some mandatory input did not match.
	OutcomeNotReady Outcome = "not-ready"
	// OutcomeReady: observed, and every mandatory input matched.
	OutcomeReady Outcome = "ready"
)

// Result is one validation check's result.
type Result string

const (
	ResultMatch    Result = "match"
	ResultMismatch Result = "mismatch"
)

// Check is one mandatory input compared with its effective value.
type Check struct {
	Input  string `json:"input"`
	Result Result `json:"result"`
}

// Validation is the provenance of the outcome: which validator compared
// which observer's report, and each check.
type Validation struct {
	Validator string  `json:"validator"`
	Observer  string  `json:"observer"`
	Checks    []Check `json:"checks,omitempty"`
}

// Artifact is a reference to something preparation produced.
type Artifact struct {
	Name   string `json:"name"`
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

// Restored records a restoration: the source subject and the dimensions
// carried over.
type Restored struct {
	Source    Instance                `json:"source"`
	Preserved runner.PreservationNeed `json:"preserved"`
}

// Receipt is the prepared-environment/v1 record.
type Receipt struct {
	Version    string     `json:"version"`
	Inputs     Request    `json:"inputs"`
	Allocation Allocation `json:"allocation"`
	Outcome    Outcome    `json:"outcome"`
	Validation Validation `json:"validation"`
	Artifacts  []Artifact `json:"artifacts,omitempty"`
	Restored   *Restored  `json:"restored,omitempty"`
}

var (
	revision = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	digest   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// invalid formats a refusal from a constant path and a constant reason.
func invalid(at, why string) error { return fmt.Errorf("%w: %s %s", ErrInvalid, at, why) }

// Validate checks a request's shape and that no field carries credential
// material. Refusals name the field, never its content.
func (r Request) Validate() error {
	if p := credentialPath(reflect.ValueOf(r), "request"); p != "" {
		return fmt.Errorf("%w: %s", runner.ErrCredential, p)
	}
	return r.shape("request")
}

func (r Request) shape(at string) error {
	switch {
	case r.Invocation.Caller == "" || r.Invocation.ID == "":
		return invalid(at+".invocation", "needs caller and id")
	case r.Owner.Key == "" || r.Owner.Incarnation == "" || r.Owner.ValidatedBy == "":
		return invalid(at+".owner", "needs key, incarnation and validated_by")
	case r.Owner.Generation < 1:
		return invalid(at+".owner.generation", "must be positive")
	}
	if err := r.Instance.shape(at + ".instance"); err != nil {
		return err
	}
	if len(r.Repositories) == 0 {
		return invalid(at+".repositories", "is empty")
	}
	seen := map[string]bool{}
	for i, repo := range r.Repositories {
		ip := at + ".repositories[" + strconv.Itoa(i) + "]"
		if repo.Name == "" || seen[repo.Name] {
			return invalid(ip+".name", "is empty or repeated")
		}
		seen[repo.Name] = true
		if !revision.MatchString(repo.Revision) {
			return invalid(ip+".revision", "is not an exact revision")
		}
	}
	e := r.Environment
	switch {
	case e.Recipe.ID == "" || e.Recipe.Version == "":
		return invalid(at+".environment.recipe", "needs id and version")
	case !digest.MatchString(e.Recipe.Digest):
		return invalid(at+".environment.recipe.digest", "is not a sha256 digest")
	case !digest.MatchString(e.HarnessBundle):
		return invalid(at+".environment.harness_bundle", "is not a sha256 digest")
	case e.Image != "" && !digest.MatchString(e.Image):
		return invalid(at+".environment.image", "is not a sha256 digest")
	case e.Tools == nil:
		return invalid(at+".environment.tools", "needs a tools list")
	case e.Skills == nil:
		return invalid(at+".environment.skills", "needs a skills list")
	}
	for _, list := range [][]Component{e.Tools, e.Skills} {
		for _, c := range list {
			if c.Name == "" || c.Version == "" {
				return invalid(at+".environment", "has a tool or skill without name and version")
			}
		}
	}
	// A nil list would encode as null, which Decode refuses: every required
	// list is refused nil here, so no receipt the port stores fails to decode.
	if r.Writable == nil {
		return invalid(at+".writable", "needs a writable list")
	}
	for _, w := range r.Writable {
		// A backslash is refused outright: path is slash-only, so "..\x"
		// would otherwise pass as clean and relative.
		if w == "" || strings.Contains(w, `\`) || path.IsAbs(w) || path.Clean(w) != w || w == ".." || strings.HasPrefix(w, "../") {
			return invalid(at+".writable", "has a path that is not clean and relative")
		}
	}
	if r.Policy.Tool == "" || r.Policy.Network == "" {
		return invalid(at+".policy", "needs tool and network references")
	}
	if r.Restore != nil {
		if err := r.Restore.Source.shape(at + ".restore.source"); err != nil {
			return err
		}
		if r.Restore.Source == r.Instance {
			return invalid(at+".restore.source", "is the instance itself")
		}
	}
	return nil
}

func (i Instance) shape(at string) error {
	if i.Cell == "" || i.Role == "" || i.ID == "" {
		return invalid(at, "needs cell, role and id")
	}
	if i.Generation < 1 {
		return invalid(at+".generation", "must be positive")
	}
	return nil
}

// mandatory lists the inputs a ready receipt must record as matched.
func (r Request) mandatory() []string {
	in := []string{"repositories", "recipe", "tools", "skills", "harness_bundle", "writable", "policy"}
	if r.Environment.Image != "" {
		in = append(in, "image")
	}
	return in
}

// Validate checks a receipt's shape, that it carries no credential material,
// and that a ready receipt records an allocation and a match for every
// mandatory input.
func (r Receipt) Validate() error {
	if p := credentialPath(reflect.ValueOf(r), "receipt"); p != "" {
		return fmt.Errorf("%w: %s", runner.ErrCredential, p)
	}
	if r.Version != Version {
		return invalid("receipt.version", "is not "+Version)
	}
	if err := r.Inputs.shape("receipt.inputs"); err != nil {
		return err
	}
	if r.Validation.Validator == "" || r.Validation.Observer == "" {
		return invalid("receipt.validation", "needs validator and observer")
	}
	for _, c := range r.Validation.Checks {
		if c.Input == "" || (c.Result != ResultMatch && c.Result != ResultMismatch) {
			return invalid("receipt.validation.checks", "has an undefined check")
		}
	}
	for _, a := range r.Artifacts {
		if a.Name == "" || a.Ref == "" || !digest.MatchString(a.Digest) {
			return invalid("receipt.artifacts", "has an artifact without name, ref and sha256 digest")
		}
	}
	if r.Restored != nil {
		if err := r.Restored.Source.shape("receipt.restored.source"); err != nil {
			return err
		}
	}
	switch r.Outcome {
	case OutcomeUnknown, OutcomePartial, OutcomeNotReady:
		return nil
	case OutcomeReady:
	default:
		return invalid("receipt.outcome", "is not a defined outcome")
	}
	if r.Allocation.Preparation == "" || r.Allocation.Workspace == "" {
		return invalid("receipt.allocation", "is incomplete on a ready receipt")
	}
	for _, in := range r.Inputs.mandatory() {
		if !slices.Contains(r.Validation.Checks, Check{Input: in, Result: ResultMatch}) {
			return invalid("receipt.validation.checks", "lacks a match for a mandatory input on a ready receipt")
		}
	}
	return nil
}

// Decode strictly decodes and validates a receipt: exact keys, no repeated
// key, no null for a defined field, nothing trailing.
func Decode(b []byte) (Receipt, error) {
	var r Receipt
	if err := runner.DecodeStrict(b, &r); err != nil {
		return Receipt{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := r.Validate(); err != nil {
		return Receipt{}, err
	}
	return r, nil
}

// Admit accepts a receipt only as a ready record of this invocation, owner,
// instance and workspace. A receipt for another instance (a swapped
// readiness), another invocation or another allocation is refused.
func (r Receipt) Admit(inv Invocation, owner Owner, inst Instance, workspace string) error {
	if err := r.Validate(); err != nil {
		return err
	}
	switch {
	case r.Outcome != OutcomeReady:
		return fmt.Errorf("%w: receipt.outcome", ErrNotReady)
	case r.Inputs.Invocation != inv:
		return fmt.Errorf("%w: receipt.inputs.invocation", ErrMismatch)
	case r.Inputs.Owner != owner:
		return fmt.Errorf("%w: receipt.inputs.owner", ErrMismatch)
	case r.Inputs.Instance != inst:
		return fmt.Errorf("%w: receipt.inputs.instance", ErrMismatch)
	case r.Allocation.Workspace != workspace:
		return fmt.Errorf("%w: receipt.allocation.workspace", ErrMismatch)
	}
	return nil
}

// credentialPath walks every string of a request or receipt, at every depth,
// and returns the json path of the first one that looks like a credential, or
// "". The path is built from json tags and indexes, never from content.
func credentialPath(v reflect.Value, at string) string {
	switch v.Kind() {
	case reflect.String:
		if runner.LooksLikeCredential(v.String()) {
			return at
		}
	case reflect.Pointer:
		if !v.IsNil() {
			return credentialPath(v.Elem(), at)
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if p := credentialPath(v.Index(i), at+"["+strconv.Itoa(i)+"]"); p != "" {
				return p
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			tag, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("json"), ",")
			if p := credentialPath(v.Field(i), at+"."+tag); p != "" {
				return p
			}
		}
	}
	return ""
}
