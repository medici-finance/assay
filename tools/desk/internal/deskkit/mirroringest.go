package deskkit

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/forgeread/adapters"
	"github.com/medici-finance/assay/forgeread/mirror"
)

// mirrorIngestSource joins the existing admitted native read composition to the
// canonical observation port. It owns no credentials, storage or refresh loop.
type mirrorIngestSource struct {
	source      *MirrorReadSource
	admissionID string
	repository  mirror.RepositoryID
	now         func() time.Time
	sourceID    string
	authority   mirror.AttemptAuthority
}

// NewMirrorIngestSource accepts a previously admitted source and a trusted explicit
// observation clock. The bound admission ID is supplied alongside the reader by
// trusted credential composition, never by a request. This consistency binding
// does not independently prove credential-installation selection.
func NewMirrorIngestSource(source *MirrorReadSource, admissionID, sourceID string, now func() time.Time, authority mirror.AttemptAuthority) (mirror.Source, error) {
	if source == nil || source.forge == nil || admissionID == "" || strings.TrimSpace(admissionID) != admissionID || sourceID == "" || strings.TrimSpace(sourceID) != sourceID || now == nil || nilMirrorInput(authority) {
		return nil, Refused("mirror observation source needs an admitted reader, bound admission/source, explicit clock and attempt authority")
	}
	path := source.repo.Slug()
	if source.provider == ForgeGitHub {
		path = strings.ToLower(path)
	}
	repo := mirror.RepositoryID{Provider: mirror.Provider(source.provider), Host: source.instance, Path: path}
	if err := repo.Validate(); err != nil {
		return nil, err
	}
	return &mirrorIngestSource{source: source, admissionID: admissionID, repository: repo, now: now, sourceID: sourceID, authority: authority}, nil
}

func nilMirrorInput(a any) bool {
	if a == nil {
		return true
	}
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	}
	return false
}

// Scan independently admits each invocation before using the already-bound
// source. A caller refusal or HTTP status alone is never retention authority.
func (s *mirrorIngestSource) Scan(ctx context.Context, a mirror.Admission, scope mirror.Scope, plan mirror.ScanPlan) (outcome mirror.ScanResult, failure error) {
	var attempt *mirrorAttemptState
	defer func() {
		held := outcome.Attempts.Held
		outcome.Attempts = mirror.AttemptReceipt{Grant: plan.AttemptGrant, HTTP: mirror.WorkCount{Known: true}, DecodedBytes: mirror.WorkCount{Known: true}, Retry: mirror.RetryEvidence{Knowledge: mirror.RetryNoConstraint}, Held: held}
		if attempt != nil {
			outcome.Attempts = attempt.snapshot()
		}
		if errors.Is(failure, mirror.ErrAttemptHeld) && outcome.Attempts.Held == "" {
			outcome.Attempts.Held = mirror.AttemptUnknownHeld
		}
	}()
	if ctx == nil {
		return mirror.ScanResult{}, fmt.Errorf("%w: missing scan context", mirror.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return mirror.ScanResult{}, err
	}
	if s == nil || s.source == nil || s.now == nil {
		return mirror.ScanResult{}, fmt.Errorf("%w: unconfigured observation source", mirror.ErrInvalid)
	}
	if err := scope.Validate(); err != nil {
		return mirror.ScanResult{}, err
	}
	if scope.Repository != s.repository || scope.AdmissionID != s.admissionID || nilMirrorInput(a) || a.ID() != s.admissionID {
		return mirror.ScanResult{}, mirror.ErrPermission
	}
	if err := a.Authorize(ctx, scope.Repository); err != nil {
		return mirror.ScanResult{}, mirror.ErrPermission
	}
	now := s.now()
	if now.IsZero() || plan.Scope != scope || (plan.FullInventory && !plan.Since.IsZero()) || plan.Since.After(now) || plan.NotBefore.After(now) || plan.PriorObservedUpTo.After(now) || (!plan.PriorObservedUpTo.IsZero() && plan.ExpectedVersion == "") {
		return mirror.ScanResult{}, fmt.Errorf("%w: incompatible scan plan or clock", mirror.ErrInvalid)
	}
	if plan.AttemptGrant.Validate(now) != nil || plan.AttemptGrant.Scope != scope || plan.AttemptGrant.SourceID != s.sourceID || s.authority == nil {
		return mirror.ScanResult{Attempts: mirror.AttemptReceipt{Held: mirror.AttemptUnknownHeld}}, &mirror.AttemptError{Hold: mirror.AttemptUnknownHeld}
	}
	var setupErr error
	attempt, setupErr = newMirrorAttempt(ctx, s, a, scope, plan)
	if setupErr != nil {
		return mirror.ScanResult{}, setupErr
	}
	ctx = context.WithValue(ctx, mirrorAttemptKey{}, attempt)
	kind := MirrorIssues
	if scope.Kind == mirror.Changes {
		kind = MirrorChanges
	}
	r, err := s.source.Read(ctx, kind, adapters.UpdateQuery{Since: plan.Since})
	if err != nil {
		if errors.Is(err, mirror.ErrAttemptHeld) || errors.Is(err, mirror.ErrPermission) {
			return mirror.ScanResult{}, err
		}
		// Do not turn an unqualified 403/404, body cap or transport failure into
		// Permission. The retention owner must independently qualify access loss.
		failure := mirror.Transport
		if IsForgeRateLimited(err) {
			failure = mirror.RateLimited
		}
		return mirror.ScanResult{Failure: failure}, err
	}
	if err := ctx.Err(); err != nil {
		return mirror.ScanResult{Failure: mirror.Transport}, err
	}
	// Current admission must still hold when observations escape the source;
	// revocation is a caller refusal, never source failure/retention authority.
	if scope.AdmissionID != s.admissionID || a.ID() != s.admissionID {
		return mirror.ScanResult{}, mirror.ErrPermission
	}
	if err := a.Authorize(ctx, scope.Repository); err != nil {
		return mirror.ScanResult{}, mirror.ErrPermission
	}
	completed := s.now()
	if completed.IsZero() || completed.Before(now) {
		return mirror.ScanResult{Failure: mirror.Unknown}, fmt.Errorf("%w: observation clock regressed", mirror.ErrInvalid)
	}
	dataAt := attempt.snapshot().ObservedAt
	if dataAt.IsZero() || dataAt.Before(now) || dataAt.After(completed) {
		return mirror.ScanResult{Failure: mirror.Unknown}, fmt.Errorf("%w: native data completion time is unavailable", mirror.ErrInvalid)
	}
	now = dataAt
	validRepo := r.Repository == s.source.repo.Slug()
	if s.source.provider == ForgeGitHub {
		validRepo = strings.EqualFold(r.Repository, s.source.repo.Slug())
	}
	if r.Instance != s.repository.Host || !validRepo || r.Provider != string(s.source.provider) {
		return mirror.ScanResult{Failure: mirror.Unknown}, fmt.Errorf("%w: incompatible source inventory", mirror.ErrInvalid)
	}
	out := mirror.ScanResult{Complete: !r.Incomplete}
	if r.Incomplete {
		out.Failure = mirror.Truncated
	}
	seen := map[mirror.RecordID]bool{}
	add := func(v adapters.IssueUpdate, kind string, head string, reviewers []adapters.UpdateActor) error {
		matches := v.Repository == s.source.repo.Slug()
		if s.source.provider == ForgeGitHub {
			matches = strings.EqualFold(v.Repository, s.source.repo.Slug())
		}
		if v.Provider != string(s.source.provider) || !matches || v.Number <= 0 || v.UpdatedAt.IsZero() || v.UpdatedAt.After(now) {
			return fmt.Errorf("%w: incompatible source observation", mirror.ErrInvalid)
		}
		id := mirror.RecordID{Repository: s.repository, Kind: kind, Number: int64(v.Number)}
		if seen[id] {
			return fmt.Errorf("%w: duplicate source observation", mirror.ErrInvalid)
		}
		seen[id] = true
		body := ""
		if v.BodyKnown {
			body = v.Body
		}
		row := mirror.Observation{ID: id, State: v.State, ClosureReason: v.ClosureReason, HeadRevision: head,
			Title: v.Title, Body: body, BodyKnown: v.BodyKnown, Labels: slices.Clone(v.Labels),
			Author: mirrorUpdateActor(v.Author), UpdatedAt: v.UpdatedAt, ObservedAt: now,
			Reviews: mirror.ReviewObservations{State: mirror.NeverRead}}
		if reviewers != nil {
			row.RequestedReviewers = make([]mirror.Actor, len(reviewers))
			for i, actor := range reviewers {
				row.RequestedReviewers[i] = mirrorUpdateActor(actor)
			}
		}
		out.Records = append(out.Records, row)
		if v.UpdatedAt.After(out.ObservedUpTo) {
			out.ObservedUpTo = v.UpdatedAt
		}
		return nil
	}
	if kind == MirrorIssues {
		for _, v := range r.Issues {
			if err := add(v, "issue", "", nil); err != nil {
				return mirror.ScanResult{Failure: mirror.Unknown}, err
			}
		}
	} else {
		want := "pull-request"
		if s.source.provider == ForgeGitLab {
			want = "merge-request"
		}
		for _, v := range r.Changes {
			if v.Kind != want {
				return mirror.ScanResult{Failure: mirror.Unknown}, fmt.Errorf("%w: incompatible change kind", mirror.ErrInvalid)
			}
			if err := add(v.IssueUpdate, want, v.HeadSHA, v.RequestedReviewers); err != nil {
				return mirror.ScanResult{Failure: mirror.Unknown}, err
			}
		}
	}
	// Returned rows supply new provider stamps; the carried baseline retains only
	// an already-observed maximum. Never substitute the observation clock.
	if r.Incomplete {
		out.ObservedUpTo = time.Time{}
	} else if plan.PriorObservedUpTo.After(out.ObservedUpTo) {
		// The carried value retains a previously observed maximum. Completeness
		// is earned by this successful walk; this never substitutes for a walk.
		out.ObservedUpTo = plan.PriorObservedUpTo
	}
	return out, nil
}

func mirrorUpdateActor(a adapters.UpdateActor) mirror.Actor {
	id := ""
	if a.ID != 0 {
		id = strconv.FormatInt(a.ID, 10)
	}
	return mirror.Actor{ID: id, Login: a.Login, Type: a.Type}
}

var _ mirror.Source = (*mirrorIngestSource)(nil)
