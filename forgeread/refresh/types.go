// Package refresh composes the mirror kernel with an injected shared refresh gate.
// It supplies no gate backend, credentials, provider walker or deployment.
package refresh

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
)

var (
	ErrHeld   = errors.New("refresh: shared coordination unavailable")
	ErrQuota  = errors.New("refresh: caller quota unavailable")
	ErrStale  = errors.New("refresh: owner generation no longer current")
	ErrPolicy = errors.New("refresh: invalid trusted policy")
)

// Limits are trusted composition input, never decoded from a caller request.
// The gate enforces these across coordinators; there are no local allowances.
type Limits struct {
	CallerFillsPerWindow, AdmissionFillsPerWindow                                int
	FillWindow, MinFreshness, MaxFreshness                                       time.Duration
	ActiveFills, QueuedCallers, AdmissionFills, AdmissionWaiters, FailureEntries int
	WaitTimeout, FillTimeout, Cooldown, FailureTTL                               time.Duration
	Refresh                                                                      mirror.RefreshPolicy
}

func (l Limits) valid() bool {
	return l.CallerFillsPerWindow > 0 && l.AdmissionFillsPerWindow >= l.CallerFillsPerWindow && l.FillWindow > 0 && l.MinFreshness > 0 && l.MaxFreshness >= l.MinFreshness && l.ActiveFills > 0 && l.QueuedCallers > 0 && l.AdmissionFills >= l.ActiveFills && l.AdmissionWaiters >= l.QueuedCallers && l.FailureEntries > 0 && l.WaitTimeout > 0 && l.FillTimeout > 0 && l.Cooldown > 0 && l.FailureTTL >= l.Cooldown && l.Refresh.Overlap >= 0 && l.Refresh.FullInventoryInterval > 0 && l.Refresh.FailureBackoff >= 0
}

// Caller is a trusted identity/policy snapshot. A caller-facing adapter must derive
// every field from current authentication/configuration, not request text.
type Caller struct {
	ID, AdmissionID, AccessEpoch, ConfigEpoch, PolicyClass, PolicyEpoch string
	Limits                                                              Limits
}

func (c Caller) valid(a mirror.Admission, q mirror.Query) bool {
	return !absent(a) && c.ID != "" && c.AdmissionID != "" && c.AdmissionID == a.ID() && c.AdmissionID == q.Scope.AdmissionID && c.AccessEpoch != "" && c.ConfigEpoch != "" && c.PolicyClass != "" && c.PolicyEpoch != "" && c.Limits.valid() && q.MaxAge >= c.Limits.MinFreshness && q.MaxAge <= c.Limits.MaxFreshness
}

type Query struct {
	mirror.Query
	RequireReviews bool
}

// FillKey qualifies the underlying source population, not a presentation projection.
// Caller identity is separate accounting, so compatible callers can share a fill.
type FillKey struct {
	Scope                                                        mirror.Scope
	SourceID, AccessEpoch, ConfigEpoch, PolicyClass, PolicyEpoch string
}
type Demand struct {
	Key      FillKey
	CallerID string
	Limits   Limits
	Now      time.Time
}
type Ticket struct {
	ID, Generation string
	Owner          bool
	Joined         bool
}
type State struct {
	Pending bool
	Reason  mirror.Reason
	RetryAt time.Time
}

// CallerAuthority revalidates current trusted caller, policy and epoch facts.
// It authenticates no remote request here and has no default implementation.
type CallerAuthority interface {
	Authorize(context.Context, Caller, mirror.Scope) error
}

type Completion struct {
	// AdmissionDenied is caller/admission refusal, never confirmed source loss.
	AdmissionDenied bool
	Authority       CallerAuthority
	Caller          Caller
	Query           Query
	Update          *mirror.ScanUpdate
	Failure         mirror.Reason
	Now             time.Time
}
type Result struct {
	mirror.Result
	Fulfilled bool
	Reason    mirror.Reason
	RetryAt   time.Time
}
type Clock interface{ Now() time.Time }

func absent(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return r.IsNil()
	}
	return false
}

func sufficient(r mirror.Result, q Query) bool {
	if !r.Available || r.Stale || r.Snapshot.Coverage.State != mirror.Complete {
		return false
	}
	if q.RequireReviews {
		for _, o := range r.Snapshot.Records {
			if o.Reviews.State != mirror.Complete {
				return false
			}
		}
	}
	return true
}
func present(r mirror.Result, q Query, state State) Result {
	ok := sufficient(r, q)
	if !ok {
		r.Available = false
		r.Stale = true
	}
	reason := r.Snapshot.Coverage.Reason
	if reason == "" && !ok {
		reason = state.Reason
		if reason == "" {
			reason = mirror.Unknown
		}
	}
	return Result{Result: r, Fulfilled: ok, Reason: reason, RetryAt: state.RetryAt}
}
func held(q Query, reason mirror.Reason) Result {
	if reason == "" {
		reason = mirror.Unknown
	}
	return Result{Result: mirror.Result{Snapshot: mirror.Snapshot{Scope: q.Scope, Coverage: mirror.Coverage{State: mirror.Unavailable, Reason: reason}}, Stale: true}, Reason: reason}
}

// Current admission is checked independently of gate ownership. Gate leases never
// authorize reading or returning observations.
func authorize(ctx context.Context, a mirror.Admission, scope mirror.Scope) error {
	if absent(a) || a.ID() == "" || a.ID() != scope.AdmissionID {
		return mirror.ErrPermission
	}
	if err := a.Authorize(ctx, scope.Repository); err != nil {
		return mirror.ErrPermission
	}
	return nil
}

// Observation counts only work at this composition seam. A scan-call is not an
// HTTP page, retry, provider point or a copied total from another process.
// Bytes remain unknown unless the source owner separately instruments them.
type Observation struct {
	Kind, Unit, Generation string
	Scope                  mirror.Scope
	Count                  int64
	Known                  bool
}

// Observer must honor context bounds. It is optional diagnostic metadata and
// never grants authorization or changes a pacing/brake decision.
type Observer interface {
	Observe(context.Context, Observation) error
}
