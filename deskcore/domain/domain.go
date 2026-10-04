// Package domain is the leaf of deskcore: the identifiers, revisions, facts, requests, results
// and receipts every other package speaks in.
//
// It imports only the standard library and performs no effect. Nothing here reads the process
// environment, the filesystem, the network or a clock; a caller that needs "now" passes it in.
// The archtest package's tests check this: no process, network or effectful package along any
// import chain, no direct import of os, syscall or the other effect-capable packages it lists,
// no environment read, and no clock read. A change that breaks one of those fails `go test` in
// deskcore/archtest. CI does not run that suite yet (medici-finance/assay#2208), so until it
// does the check holds only where the suite is run.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Revision names one immutable state of an object: a commit SHA, a change head, an ETag or a
// cursor. It is opaque to deskcore; only equality matters.
type Revision string

// Identifiers. Each is opaque and compared for equality only; their distinct types stop one
// kind of identifier being passed where another is expected.
type (
	// RequestID is the stable identity of one consequential request. A retry reuses it, which
	// is what lets an executor reconcile an earlier attempt instead of repeating its effect.
	RequestID string
	// InstanceID names one run of a work graph.
	InstanceID string
	// NodeID names one node (a work item) within a graph.
	NodeID string
	// AttemptID names one attempt at a node. An attempt may own a workspace; the workspace is
	// a resource of the attempt, never the node's identity.
	AttemptID string
	// OperationID names one outward effect an executor performed or tried to perform.
	OperationID string
	// FactID names one fact inside a fact bundle; it is unique within that bundle.
	FactID string
)

var (
	digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	roleRe   = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
)

// Digest is a content digest in the form "sha256:<64 lowercase hex>".
type Digest string

// DigestOf returns the sha256 Digest of b.
func DigestOf(b []byte) Digest {
	sum := sha256.Sum256(b)
	return Digest("sha256:" + hex.EncodeToString(sum[:]))
}

// Validate reports whether d is a well-formed sha256 digest.
func (d Digest) Validate() error {
	if !digestRe.MatchString(string(d)) {
		return fmt.Errorf("digest %q: want sha256:<64 lowercase hex>", string(d))
	}
	return nil
}

// Role names a credential-holding role (for example a worker or a reviewer). Role names are
// lowercase words joined by single hyphens. A role is configuration, not a fixed set: which
// roles an installation runs is its own choice.
type Role string

// Validate reports whether r is a well-formed role name.
func (r Role) Validate() error {
	if len(r) > 63 || !roleRe.MatchString(string(r)) {
		return fmt.Errorf("role %q: want lowercase words joined by single hyphens, at most 63 characters", string(r))
	}
	return nil
}

// Target is the object a request acts on, bound by immutable identity rather than by name:
// repositories and projects can be renamed, their numeric identities cannot.
type Target struct {
	ForgeInstanceID string `json:"forge_instance_id"`
	RepositoryID    string `json:"repository_id"`
	ObjectKind      string `json:"object_kind"`
	ObjectID        string `json:"object_id"`
}

// Validate reports whether every field of t is set.
func (t Target) Validate() error {
	var errs []error
	for _, f := range []struct{ name, v string }{
		{"forge_instance_id", t.ForgeInstanceID},
		{"repository_id", t.RepositoryID},
		{"object_kind", t.ObjectKind},
		{"object_id", t.ObjectID},
	} {
		if f.v == "" {
			errs = append(errs, fmt.Errorf("target: %s is empty", f.name))
		}
	}
	return errors.Join(errs...)
}

// Expected holds the revisions a request was prepared against. An executor rechecks them
// before the effect; a mismatch is a conflict, never a silent overwrite.
type Expected struct {
	ChangeHead     Revision `json:"change_head,omitempty"`
	BaseRevision   Revision `json:"base_revision,omitempty"`
	ArtifactDigest Digest   `json:"artifact_digest,omitempty"`
}

// Binding ties a request to the reviewed pattern, acceptance contract and policy it was
// evaluated under, and to the lease generation that admitted it.
type Binding struct {
	PatternDigest    Digest `json:"pattern_digest,omitempty"`
	AcceptanceDigest Digest `json:"acceptance_digest,omitempty"`
	PolicyDigest     Digest `json:"policy_digest"`
	LeaseGeneration  uint64 `json:"lease_generation"`
}

// RequestSchemaV1 is the schema version a Request carries.
const RequestSchemaV1 = 1

// Request is one consequential write request. The actor is deliberately absent: an executor
// fills the actor from its authenticated transport and never trusts a requester to name it.
type Request struct {
	SchemaVersion int        `json:"schema_version"`
	RequestID     RequestID  `json:"request_id"`
	InstanceID    InstanceID `json:"instance_id"`
	NodeID        NodeID     `json:"node_id"`
	AttemptID     AttemptID  `json:"attempt_id"`
	Operation     string     `json:"operation"`
	Target        Target     `json:"target"`
	Expected      Expected   `json:"expected"`
	Binding       Binding    `json:"binding"`
	// PayloadDigest identifies the operation-specific payload or immutable artifact.
	PayloadDigest Digest `json:"payload_digest"`
}

// Validate reports every structural defect in q. It checks shape only; whether the request is
// admissible is a policy decision made elsewhere.
func (q Request) Validate() error {
	var errs []error
	if q.SchemaVersion != RequestSchemaV1 {
		errs = append(errs, fmt.Errorf("request: schema_version %d is not %d", q.SchemaVersion, RequestSchemaV1))
	}
	for _, f := range []struct{ name, v string }{
		{"request_id", string(q.RequestID)},
		{"instance_id", string(q.InstanceID)},
		{"node_id", string(q.NodeID)},
		{"attempt_id", string(q.AttemptID)},
		{"operation", q.Operation},
	} {
		if f.v == "" {
			errs = append(errs, fmt.Errorf("request: %s is empty", f.name))
		}
	}
	if err := q.Target.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("request: %w", err))
	}
	if q.Expected.ArtifactDigest != "" {
		if err := q.Expected.ArtifactDigest.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("request: expected.artifact_digest: %w", err))
		}
	}
	for _, d := range []struct {
		name     string
		v        Digest
		optional bool
	}{
		{"binding.pattern_digest", q.Binding.PatternDigest, true},
		{"binding.acceptance_digest", q.Binding.AcceptanceDigest, true},
		{"binding.policy_digest", q.Binding.PolicyDigest, false},
		{"payload_digest", q.PayloadDigest, false},
	} {
		if d.v == "" && d.optional {
			continue
		}
		if err := d.v.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("request: %s: %w", d.name, err))
		}
	}
	if q.Binding.LeaseGeneration == 0 {
		errs = append(errs, errors.New("request: binding.lease_generation is 0; a request is admitted under a live lease generation"))
	}
	return errors.Join(errs...)
}

// Receipt is the authoritative record of an applied effect.
type Receipt struct {
	OperationID OperationID `json:"operation_id"`
	RequestID   RequestID   `json:"request_id"`
	Target      Target      `json:"target"`
	// Revision is the object's revision after the effect.
	Revision Revision  `json:"revision"`
	At       time.Time `json:"at"`
}

// OutcomeKind is one of exactly six outcomes a request can have.
type OutcomeKind string

// The six outcome kinds. A timeout after a request was transmitted is OutcomeUnknown: it is
// not evidence that the effect failed, and it must be reconciled before any retry.
const (
	Applied         OutcomeKind = "applied"
	Denied          OutcomeKind = "denied"
	Conflict        OutcomeKind = "conflict"
	PendingExternal OutcomeKind = "pending_external"
	CouldNotCheck   OutcomeKind = "could_not_check"
	OutcomeUnknown  OutcomeKind = "outcome_unknown"
)

// OutcomeKinds returns the six outcome kinds in a fixed order.
func OutcomeKinds() []OutcomeKind {
	return []OutcomeKind{Applied, Denied, Conflict, PendingExternal, CouldNotCheck, OutcomeUnknown}
}

// ParseOutcomeKind returns the OutcomeKind named s, or an error for any other string. There is
// no default: an unrecognised outcome is refused, never mapped to a nearby one.
func ParseOutcomeKind(s string) (OutcomeKind, error) {
	for _, k := range OutcomeKinds() {
		if string(k) == s {
			return k, nil
		}
	}
	return "", fmt.Errorf("outcome kind %q is not one of applied, denied, conflict, pending_external, could_not_check, outcome_unknown", s)
}

// UnmarshalJSON refuses an unrecognised outcome kind.
func (k *OutcomeKind) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := ParseOutcomeKind(s)
	if err != nil {
		return err
	}
	*k = v
	return nil
}

// Result is the outcome of one request. Each kind requires the field that makes it
// actionable: a receipt for applied, a reason for denied, the current revision for a conflict,
// an external reference for pending, a cause for could-not-check, and the operation identity
// to reconcile for an unknown outcome.
type Result struct {
	Kind            OutcomeKind `json:"kind"`
	Receipt         *Receipt    `json:"receipt,omitempty"`
	Reason          string      `json:"reason,omitempty"`
	Remedy          string      `json:"remedy,omitempty"`
	CurrentRevision Revision    `json:"current_revision,omitempty"`
	Reference       string      `json:"reference,omitempty"`
	Cause           string      `json:"cause,omitempty"`
	OperationID     OperationID `json:"operation_id,omitempty"`
}

// Validate reports whether r carries the field its kind requires.
func (r Result) Validate() error {
	switch r.Kind {
	case Applied:
		if r.Receipt == nil || r.Receipt.OperationID == "" {
			return errors.New("result applied: a receipt with an operation_id is required")
		}
	case Denied:
		if r.Reason == "" {
			return errors.New("result denied: a reason is required")
		}
	case Conflict:
		if r.CurrentRevision == "" {
			return errors.New("result conflict: the current revision is required")
		}
	case PendingExternal:
		if r.Reference == "" {
			return errors.New("result pending_external: an external reference is required")
		}
	case CouldNotCheck:
		if r.Cause == "" {
			return errors.New("result could_not_check: a cause is required")
		}
	case OutcomeUnknown:
		if r.OperationID == "" {
			return errors.New("result outcome_unknown: the operation_id to reconcile is required")
		}
	default:
		return fmt.Errorf("result: unknown kind %q", string(r.Kind))
	}
	return nil
}

// Fact is one observation collected from one source. Value is the observation's payload; its
// shape is defined by Kind and is opaque to deskcore.
type Fact struct {
	ID         FactID          `json:"id"`
	Source     string          `json:"source"`
	Kind       string          `json:"kind"`
	Subject    string          `json:"subject"`
	Revision   Revision        `json:"revision,omitempty"`
	ObservedAt time.Time       `json:"observed_at"`
	Value      json.RawMessage `json:"value,omitempty"`
}

// RequiredKeys lists the keys a fact's JSON object must carry (see DecodeStrict).
func (Fact) RequiredKeys() []string {
	return []string{"id", "source", "kind", "subject", "observed_at"}
}
