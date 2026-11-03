// Package mirror defines admitted mirror observations and deterministic refresh
// plans. Storage, authenticated sources, clocks and scheduling belong to callers.
package mirror

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type Provider string

const (
	GitHub Provider = "github"
	GitLab Provider = "gitlab"
)

type RecordKind string

const (
	Issues  RecordKind = "issue"
	Changes RecordKind = "change"
)

// RepositoryID uses canonical provider identity. Trusted composition must canonicalize
// GitHub owner/repository case before constructing a Scope; aliases are not resolved here.
type RepositoryID struct {
	Provider   Provider
	Host, Path string
}
type Scope struct {
	Repository  RepositoryID
	Kind        RecordKind
	AdmissionID string
}

// Admission is supplied by trusted composition, never decoded from request data.
// Its ID binds a cell/credential installation; the ID alone grants no access.
type Admission interface {
	ID() string
	Authorize(context.Context, RepositoryID) error
}

type RecordID struct {
	Repository RepositoryID
	Kind       string
	Number     int64
}
type Actor struct{ ID, Login, Type string }
type GitHubReview struct {
	ID            string
	Author        Actor
	State, Commit string
	SubmittedAt   time.Time
}
type GitLabApproval struct {
	User       Actor
	Approved   bool
	ObservedAt time.Time
}
type ReviewObservations struct {
	State  CoverageState
	GitHub []GitHubReview
	GitLab []GitLabApproval
}
type Observation struct {
	ID                                              RecordID
	State, ClosureReason, HeadRevision, Title, Body string
	// BodyKnown distinguishes an omitted body from an explicitly empty body.
	BodyKnown bool
	// Nil collections are unknown; non-nil empty collections were observed empty.
	Labels                []string
	Author                Actor
	RequestedReviewers    []Actor
	UpdatedAt, ObservedAt time.Time
	Reviews               ReviewObservations
	// InInventory=false means absent from a complete inventory, never closed.
	InInventory bool
}

type CoverageState string

const (
	NeverRead   CoverageState = "never-read"
	Complete    CoverageState = "complete"
	Partial     CoverageState = "partial"
	Unavailable CoverageState = "unavailable"
)

type Reason string

const (
	Permission  Reason = "permission"
	Transport   Reason = "transport"
	RateLimited Reason = "rate-limited"
	Truncated   Reason = "truncated"
	Unknown     Reason = "unknown"
)

// Coverage.RetainedFrom is optional; zero means unknown. The durable Store must
// derive retained coverage under its selected policy, never from the oldest visible row.
type Coverage struct {
	// A conditional validator binds this exact scan request, separately from source revision.
	Validator                                                              string
	ValidatorSince                                                         time.Time
	ValidatorFullInventory                                                 bool
	State                                                                  CoverageState
	Reason                                                                 Reason
	LastAttempted, LastComplete, LastInventory, ObservedUpTo, RetainedFrom time.Time
	Revision                                                               string
}
type Snapshot struct {
	Scope    Scope
	Coverage Coverage
	Records  []Observation
	Version  string
}
type Result struct {
	Snapshot         Snapshot
	Stale, Available bool
}
type Query struct {
	Scope  Scope
	Now    time.Time
	MaxAge time.Duration
}
type RefreshPolicy struct{ Overlap, FullInventoryInterval, FailureBackoff time.Duration }
type ScanPlan struct {
	AttemptGrant    AttemptGrant
	Scope           Scope
	ExpectedVersion string
	// PriorObservedUpTo is the selected snapshot watermark, not new coverage.
	// PlanScan binds it independently to the same scope and expected version.
	PriorObservedUpTo time.Time
	FullInventory     bool
	Since, NotBefore  time.Time
	Validator         string
}
type ScanResult struct {
	Attempts                                    AttemptReceipt
	Validator                                   string
	Records                                     []Observation
	Complete, NotModified, ConditionalSupported bool
	ObservedUpTo                                time.Time
	Revision                                    string
	Failure                                     Reason
}
type ScanUpdate struct {
	Scope                                                  Scope
	ExpectedVersion                                        string
	Coverage                                               Coverage
	Upsert                                                 []Observation
	NotInInventory                                         []RecordID
	CompleteScan, FullInventory, RetentionDecisionRequired bool
}

// AttemptGrant contains opaque identity and claims independently bound by the
// canonical authority. Copying it grants neither access nor budget; trusted
// refresh composition supplies it, never query text.
type AttemptGrant struct {
	ID, SourceID, Generation, CallerID                 string
	AccessEpoch, ConfigEpoch, PolicyClass, PolicyEpoch string
	Scope                                              Scope
	MaxHTTPAttempts                                    int64
	ExpiresAt                                          time.Time
}

func (g AttemptGrant) Validate(now time.Time) error {
	if err := g.Scope.Validate(); err != nil {
		return err
	}
	if g.ID == "" || g.SourceID == "" || g.Generation == "" || g.CallerID == "" || g.AccessEpoch == "" || g.ConfigEpoch == "" || g.PolicyClass == "" || g.PolicyEpoch == "" || g.MaxHTTPAttempts <= 0 || now.IsZero() || !now.Before(g.ExpiresAt) {
		return invalid("missing, expired or unbounded native attempt grant")
	}
	return nil
}

// A reservation is one canonical BeforeAttempt result, not an HTTP send. An
// uncertain call cannot be closed or refunded from an aggregate receipt.
type AttemptReservation struct {
	ID    string
	Grant AttemptGrant
}

// Value is the observed count/prefix. Known=false never qualifies it as a
// complete total, and is distinct from an observed zero.
type WorkCount struct {
	Value int64
	Known bool
}

type RetryKnowledge string

const (
	RetryUnknown      RetryKnowledge = "unknown"
	RetryNotBefore    RetryKnowledge = "not-before"
	RetryNoConstraint RetryKnowledge = "no-observed-constraint"
	RetryNoResponse   RetryKnowledge = "qualified-no-response"
)

// Unknown knowledge retains any prior positive maximum floor. NoConstraint is
// a fully covered observation, never a universal provider allowance.
type RetryEvidence struct {
	Knowledge RetryKnowledge
	NotBefore time.Time
	// A qualified relative hint remains separate from the source clock's
	// absolute floor. The canonical authority also anchors it to its own
	// observation time, so process clock skew cannot shorten the delay.
	Delay      time.Duration
	DelayKnown bool
}

type AttemptHold string

const (
	AttemptCallerHeld    AttemptHold = "caller"
	AttemptAdmissionHeld AttemptHold = "admission"
	AttemptSourceHeld    AttemptHold = "source"
	AttemptUnknownHeld   AttemptHold = "unknown"
)

func (h AttemptHold) Valid() bool {
	return h == AttemptCallerHeld || h == AttemptAdmissionHeld || h == AttemptSourceHeld || h == AttemptUnknownHeld
}

var ErrAttemptHeld = errors.New("mirror: native attempt accounting held")

// AttemptError is a bounded accounting classification, never a provider error
// or confirmed source permission loss.
type AttemptError struct{ Hold AttemptHold }

func (e *AttemptError) Error() string { return ErrAttemptHeld.Error() }
func (e *AttemptError) Unwrap() error { return ErrAttemptHeld }

// Safe evidence for exactly one reservation. Status zero is unqualified, not
// success. DecodedBytes counts consumed JSON (including a rejected prefix), not
// wire bytes or a claimed total body length.
type AttemptObservation struct {
	HTTP, DecodedBytes WorkCount
	Status             int
	ResponseComplete   bool
	Retry              RetryEvidence
	ObservedAt         time.Time
}

type AttemptAuthority interface {
	BeforeAttempt(context.Context, AttemptGrant) (AttemptReservation, error)
	ObserveAttempt(context.Context, AttemptReservation, AttemptObservation) error
}

// This invocation's own work only. The aggregate never closes/refunds individual
// reservations or invents missing reports.
type AttemptReceipt struct {
	Grant              AttemptGrant
	HTTP, DecodedBytes WorkCount
	Retry              RetryEvidence
	Held               AttemptHold
}

// Source is a future composition seam, not a source implementation or permission
// to access a network. Its implementation independently admits the supplied scope.
type Source interface {
	Scan(context.Context, Admission, Scope, ScanPlan) (ScanResult, error)
}

// Store must independently admit every Read and Apply, enforce ExpectedVersion,
// and atomically commit rows with a complete scan's watermark. An implementation
// must refuse incomplete watermark advancement even with the planner bypassed.
// A persisted Permission hold survives until a CompleteScan with Complete coverage
// and FullInventory, under current matching admission, advances both LastComplete
// and LastInventory strictly beyond the held snapshot's LastAttempted.
type Store interface {
	Read(context.Context, Admission, Scope) (Snapshot, error)
	Apply(context.Context, Admission, ScanUpdate) (Snapshot, error)
}

var (
	ErrPermission = errors.New("mirror: permission unavailable")
	ErrConflict   = errors.New("mirror: snapshot version changed")
	ErrInvalid    = errors.New("mirror: invalid observation")
)

func invalid(why string) error { return fmt.Errorf("%w: %s", ErrInvalid, why) }
func (r RepositoryID) Validate() error {
	if r.Provider != GitHub && r.Provider != GitLab {
		return invalid("unknown provider")
	}
	if r.Host == "" || strings.ContainsAny(r.Host, "/:@?#\\ \t\r\n") || strings.ToLower(r.Host) != r.Host {
		return invalid("noncanonical host")
	}
	parts := strings.Split(r.Path, "/")
	if len(parts) < 2 || (r.Provider == GitHub && len(parts) != 2) {
		return invalid("repository path")
	}
	for _, p := range parts {
		if p == "" || p == "." || p == ".." || strings.ContainsAny(p, ":?#\\ \t\r\n") {
			return invalid("repository path segment")
		}
	}
	return nil
}
func (s Scope) Validate() error {
	if err := s.Repository.Validate(); err != nil {
		return err
	}
	if (s.Kind != Issues && s.Kind != Changes) || strings.TrimSpace(s.AdmissionID) == "" || s.AdmissionID != strings.TrimSpace(s.AdmissionID) {
		return invalid("scope kind or admission")
	}
	return nil
}
func validReason(r Reason) bool {
	return r == Permission || r == Transport || r == RateLimited || r == Truncated || r == Unknown
}
func validState(s CoverageState) bool {
	return s == NeverRead || s == Complete || s == Partial || s == Unavailable
}
func cloneRecords(in []Observation) []Observation {
	out := make([]Observation, len(in))
	copy(out, in)
	for i := range out {
		out[i].Labels = slices.Clone(in[i].Labels)
		out[i].RequestedReviewers = slices.Clone(in[i].RequestedReviewers)
		out[i].Reviews.GitHub = slices.Clone(in[i].Reviews.GitHub)
		out[i].Reviews.GitLab = slices.Clone(in[i].Reviews.GitLab)
	}
	return out
}

// CloneSnapshot detaches collections at a storage/consumer boundary.
func CloneSnapshot(s Snapshot) Snapshot { s.Records = cloneRecords(s.Records); return s }
func validateRecords(scope Scope, records []Observation, now time.Time) error {
	seen := map[RecordID]bool{}
	for _, r := range records {
		kindOK := scope.Kind == Issues && r.ID.Kind == "issue" || scope.Kind == Changes && (scope.Repository.Provider == GitHub && r.ID.Kind == "pull-request" || scope.Repository.Provider == GitLab && r.ID.Kind == "merge-request")
		if r.ID.Repository != scope.Repository || !kindOK || r.ID.Number <= 0 || seen[r.ID] {
			return invalid("foreign, invalid or duplicate record")
		}
		seen[r.ID] = true
		if (!r.ObservedAt.IsZero() && r.ObservedAt.After(now)) || (!r.UpdatedAt.IsZero() && r.UpdatedAt.After(now)) {
			return invalid("future record timestamp")
		}
		if r.Reviews.State != "" && !validState(r.Reviews.State) {
			return invalid("review coverage")
		}
		if scope.Repository.Provider == GitHub && len(r.Reviews.GitLab) > 0 || scope.Repository.Provider == GitLab && len(r.Reviews.GitHub) > 0 {
			return invalid("foreign review provenance")
		}
		if scope.Kind == Issues && (len(r.Reviews.GitHub) > 0 || len(r.Reviews.GitLab) > 0) {
			return invalid("reviews on issue")
		}
	}
	return nil
}
