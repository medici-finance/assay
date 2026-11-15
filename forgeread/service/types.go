// Package service defines the bounded forge-service-v1 transport contract.
// Values are observations and requests, never authentication or effect authority.
package service

import (
	"errors"
	"time"

	"github.com/medici-finance/assay/forgeread"
	"github.com/medici-finance/assay/forgeread/mirror"
)

const Schema = "forge-service-v1"

// Protocol ceilings bound a single message. Trusted composition can impose lower
// limits; these ceilings grant no provider, retention or execution allowance.
const (
	MaxMessageBytes = 1 << 20
	MaxContentBytes = 256 << 10
	MaxItems        = 256
	MaxTextBytes    = 4096
)

var (
	ErrSchema   = errors.New("forge service: unsupported schema or operation")
	ErrInvalid  = errors.New("forge service: invalid envelope")
	ErrScope    = errors.New("forge service: scope mismatch")
	ErrConflict = errors.New("forge service: operation payload conflict")
)

type Operation string

const (
	MirrorQuery       Operation = "mirror-query"
	FileAt            Operation = "file-at"
	Diff              Operation = "diff"
	MergeBase         Operation = "merge-base"
	CreateIssue       Operation = "create-issue"
	CreateComment     Operation = "create-change-comment"
	LookupOperation   Operation = "lookup-operation"
	ReadObservation   Operation = "read-observation"
	ReviewObservation Operation = "review-observation"
	IssueProjection   Operation = "issue-projection"
)

// Scope records the trusted binding expected by a caller. Servers independently
// establish current authentication, admission and epochs; these fields grant none.
type Scope struct {
	Repositories     []mirror.RepositoryID `json:"repositories"`
	AdmissionID      string                `json:"admissionId"`
	AccessEpoch      string                `json:"accessEpoch"`
	CallerID         string                `json:"callerId"`
	RepresentedActor string                `json:"representedActor"`
}

// Token is opaque progress within exactly one scope. It is neither a SQL sequence
// nor authorization. Its bytes support equality, not a total provider ordering.
type Token struct {
	Scope      Scope               `json:"scope"`
	Repository mirror.RepositoryID `json:"repository"`
	Kind       mirror.RecordKind   `json:"kind"`
	Value      string              `json:"value"`
}

type MirrorRequest struct {
	Kind     mirror.RecordKind `json:"kind"`
	MaxAgeMS int64             `json:"maxAgeMs"`
	Minimum  *Token            `json:"minimum"`
}
type SourceRequest struct {
	Revision string `json:"revision"`
	Other    string `json:"other"`
	Path     string `json:"path"`
}

// OperationID comes from the canonical R2/R4 effect owner. The wire neither
// generates a work namespace nor decides whether an unknown effect can replay.
type OperationID struct {
	Key           string `json:"key"`
	PayloadDigest string `json:"payloadDigest"`
}
type EffectRequest struct {
	ID     OperationID `json:"id"`
	Number int64       `json:"number"`
	Title  string      `json:"title"`
	Body   string      `json:"body"`
}

// DiagnosticLink is separately admitted at both transport ends, never a role,
// token or raw work/session locator. Clients do not propagate automatic headers.
type DiagnosticLink struct {
	Parent string `json:"parent"`
	Link   string `json:"link"`
}
type Request struct {
	Schema     string              `json:"schema"`
	ID         string              `json:"id"`
	Scope      Scope               `json:"scope"`
	Repository mirror.RepositoryID `json:"repository"`
	Operation  Operation           `json:"operation"`
	Mirror     *MirrorRequest      `json:"mirror"`
	Source     *SourceRequest      `json:"source"`
	Effect     *EffectRequest      `json:"effect"`
	Lookup     *OperationID        `json:"lookup"`
	Minimum    *Token              `json:"minimum"`
	Diagnostic *DiagnosticLink     `json:"diagnostic"`
}

type State string

const (
	Older             State = "older"
	Confirmed         State = "confirmed"
	Newer             State = "newer"
	ProjectionPending State = "projection-pending"
	Unavailable       State = "unavailable"
	Permission        State = "permission"
)

type Provenance struct {
	Source     string    `json:"source"`
	Version    string    `json:"version"`
	Revision   string    `json:"revision"`
	ObservedAt time.Time `json:"observedAt"`
}

// CachedObservation carries original owned-snapshot coverage and descriptive age.
// It grants no freshness, permission or retry authority and contains no rows.
type CachedObservation struct {
	Coverage    mirror.Coverage `json:"coverage"`
	EvaluatedAt time.Time       `json:"evaluatedAt"`
	AgeMS       *int64          `json:"ageMs"`
}
type Metadata struct {
	State       State              `json:"state"`
	Reason      mirror.Reason      `json:"reason"`
	Complete    *bool              `json:"complete"`
	Freshness   string             `json:"freshness"`
	RetryAt     *time.Time         `json:"retryAt"`
	Cached      *CachedObservation `json:"cached"`
	Source      *Provenance        `json:"source"`
	Observation *Token             `json:"observation"`
	Executor    *mirror.Actor      `json:"executor"`
}

type ProviderOutcome string
type ProjectionState string

const (
	Succeeded    ProviderOutcome = "succeeded"
	Failed       ProviderOutcome = "failed"
	Unknown      ProviderOutcome = "unknown"
	Current      ProjectionState = "current"
	Pending      ProjectionState = "pending"
	NoProjection ProjectionState = "unavailable"
)

type EffectResult struct {
	ID         OperationID     `json:"id"`
	Outcome    ProviderOutcome `json:"outcome"`
	Projection ProjectionState `json:"projection"`
	Object     *ObjectRef      `json:"object"`
	Provenance *Provenance     `json:"provenance"`
}

// ObjectRef preserves the provider's confirmed identifier and the comment's
// parent. A comment identifier is never relabeled an issue or change number.
type ObjectRef struct {
	Repository mirror.RepositoryID `json:"repository"`
	Kind       string              `json:"kind"`
	ProviderID string              `json:"providerId"`
	Number     *int64              `json:"number"`
	Parent     *int64              `json:"parent"`
}
type DiffEntry struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Before string `json:"before"`
	After  string `json:"after"`
}
type SourceResult struct {
	Revision string      `json:"revision"`
	Other    string      `json:"other"`
	Path     string      `json:"path"`
	Content  []byte      `json:"content"`
	Entries  []DiffEntry `json:"entries"`
	Base     string      `json:"base"`
}

// Measurement keeps process-local logical calls, transport attempts and server
// handlers distinct. Unknown counts remain nil; recipients never sum them as calls.
type Measurement struct {
	Boundary string `json:"boundary"`
	Unit     string `json:"unit"`
	Count    *int64 `json:"count"`
}
type Response struct {
	Schema       string                    `json:"schema"`
	ID           string                    `json:"id"`
	Scope        Scope                     `json:"scope"`
	Repository   mirror.RepositoryID       `json:"repository"`
	Operation    Operation                 `json:"operation"`
	Metadata     Metadata                  `json:"metadata"`
	Mirror       *mirror.Snapshot          `json:"mirror"`
	Issues       *forgeread.IssuesEnvelope `json:"issues"`
	Source       *SourceResult             `json:"source"`
	Effect       *EffectResult             `json:"effect"`
	Review       *ReviewEnvelope           `json:"review"`
	Projection   *IssueProjectionEnvelope  `json:"projection"`
	Conflict     bool                      `json:"conflict"`
	Measurements []Measurement             `json:"measurements"`
	Diagnostic   *DiagnosticLink           `json:"diagnostic"`
}
