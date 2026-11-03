package adapters

import (
	"context"
	"fmt"
	"time"
)

// UpdateQuery is a fixed incremental read. A zero Since requests the bounded full
// inventory; a nonzero bound is inclusive. It never selects an endpoint or a host.
type UpdateQuery struct {
	Since time.Time
}

// UpdateActor preserves the provider's identity without inferring an account kind
// when the endpoint does not supply one.
type UpdateActor struct {
	Login string
	ID    int64
	Type  string
}

// IssueUpdate is the issue-list observation, not a complete thread, trust verdict
// or policy decision. Provider and Repository describe the admitted source route.
type IssueUpdate struct {
	Provider      string
	Repository    string
	Number        int
	Title         string
	Body          string
	BodyKnown     bool
	State         string
	ClosureReason string
	Author        UpdateActor
	// Nil collections mean the endpoint omitted them; non-nil empty means the
	// provider explicitly reported an empty collection.
	Labels    []string
	CreatedAt time.Time
	UpdatedAt time.Time
	ClosedAt  time.Time
}

// ChangeUpdate is a provider change-list observation. Missing review, check and
// comment data remain unknown; these records never claim those collections complete.
type ChangeUpdate struct {
	IssueUpdate
	Kind               string
	HeadSHA            string
	BaseSHA            string
	HeadBranch         string
	BaseBranch         string
	SourceRepository   string
	SourceProjectID    int64
	RequestedReviewers []UpdateActor
	MergedAt           time.Time
}

// UpdateCoverage describes only this endpoint's bounded walk. Incomplete records
// are diagnostic observations, never a complete inventory or evidence of absence.
// Conditional support is explicitly false in this full-read baseline; validators
// and 304 reuse belong to the later mirror ingestion integration.
type UpdateCoverage struct {
	Incomplete           bool
	PageCount            int
	Provider             string
	Repository           string
	Since                time.Time
	ServerSideSince      bool
	ObservedUpTo         time.Time
	ConditionalSupported bool
}

type IssueUpdates struct {
	UpdateCoverage
	Records []IssueUpdate
}

type ChangeUpdates struct {
	UpdateCoverage
	Records []ChangeUpdate
}

// ValidateUpdateRead rejects canceled or invalid requests before access is used.
func ValidateUpdateRead(ctx context.Context, repo string, q UpdateQuery) error {
	if ctx == nil {
		return fmt.Errorf("incremental read requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if repo == "" {
		return fmt.Errorf("incremental read requires a repository")
	}
	if !q.Since.IsZero() && (q.Since.Year() < 1 || q.Since.Year() > 9999) {
		return fmt.Errorf("incremental read has an unrepresentable time bound")
	}
	return nil
}

// UpdateTimestamp keeps malformed or missing update stamps from advancing a
// consumer's observation watermark. Optional timestamps may remain zero.
func UpdateTimestamp(value string, required bool) (time.Time, error) {
	if value == "" && !required {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("incremental read has an invalid timestamp: %w", err)
	}
	return t.UTC(), nil
}
