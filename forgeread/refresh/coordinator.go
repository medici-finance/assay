package refresh

import (
	"context"
	"errors"

	"github.com/medici-finance/assay/forgeread/mirror"
)

// Coordinator has no mutable cache or local ownership map. The lifetime is owned
// by the host; fill contexts are bounded independently of any one waiter's context.
type Coordinator struct {
	lifetime  context.Context
	store     mirror.Store
	source    mirror.Source
	gate      Gate
	clock     Clock
	sourceID  string
	authority CallerAuthority
	observer  Observer
}

func New(lifetime context.Context, store mirror.Store, source mirror.Source, gate Gate, clock Clock, sourceID string, authority CallerAuthority, observers ...Observer) (*Coordinator, error) {
	if len(observers) > 1 {
		return nil, ErrPolicy
	}
	if absent(lifetime) || absent(store) || absent(source) || absent(gate) || absent(clock) || sourceID == "" || absent(authority) {
		return nil, ErrHeld
	}
	c := &Coordinator{lifetime: lifetime, store: store, source: source, gate: gate, clock: clock, sourceID: sourceID, authority: authority}
	if len(observers) == 1 {
		c.observer = observers[0]
	}
	return c, nil
}

func (c *Coordinator) Read(ctx context.Context, a mirror.Admission, q Query, caller Caller) (result Result, err error) {
	if c == nil || absent(ctx) {
		return held(q, mirror.Unknown), ErrHeld
	}
	if q.RequireReviews && q.Scope.Kind != mirror.Changes {
		return held(q, mirror.Unknown), ErrPolicy
	}
	if !caller.valid(a, q.Query) {
		return held(q, mirror.Permission), ErrPolicy
	}
	readCtx, cancelRead := context.WithTimeout(ctx, caller.Limits.WaitTimeout)
	defer cancelRead()
	ctx = readCtx
	if err := c.authority.Authorize(ctx, caller, q.Scope); err != nil {
		return held(q, mirror.Permission), err
	}
	q.Now = c.clock.Now()
	r, err := c.queryScope(ctx, a, q)
	if err != nil {
		return held(q, mirror.Unknown), err
	}
	q.Now = c.clock.Now()
	r, err = mirror.EvaluateSnapshot(r.Snapshot, q.Query)
	if err != nil {
		return held(q, mirror.Unknown), err
	}
	if sufficient(r, q) {
		c.observe(ctx, "cache-hit", "query", q.Scope, "")
		return c.release(ctx, a, q, caller, r, State{}, nil)
	}
	baseline, err := c.store.Read(ctx, a, q.Scope)
	if err != nil {
		return held(q, mirror.Transport), err
	}
	if baseline.Scope != q.Scope {
		return held(q, mirror.Unknown), mirror.ErrInvalid
	}
	q.Now = c.clock.Now()
	plan, err := mirror.PlanRefresh(baseline, q.Now, caller.Limits.Refresh)
	if err != nil {
		return c.release(ctx, a, q, caller, r, State{}, err)
	}
	key := FillKey{Scope: q.Scope, SourceID: c.sourceID, AccessEpoch: caller.AccessEpoch, ConfigEpoch: caller.ConfigEpoch, PolicyClass: caller.PolicyClass, PolicyEpoch: caller.PolicyEpoch}
	ticket, err := c.gate.Acquire(ctx, Demand{Key: key, Requirements: Requirements{RequireReviews: q.RequireReviews}, CallerID: caller.ID, Limits: caller.Limits, Now: q.Now})
	if err != nil {
		out, releaseErr := c.release(ctx, a, q, caller, r, State{Reason: mirror.Unknown}, err)
		if releaseErr != nil {
			return out, releaseErr
		}
		out.Fulfilled = false
		out.Available = false
		out.Stale = true
		return out, err
	}
	// Cleanup gets its own finite context: a cancelled waiter cannot leak its debit.
	var outcome Outcome
	var staleWaitErr error
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(c.lifetime), caller.Limits.WaitTimeout)
		defer cancel()
		state, e := c.gate.Leave(cleanup, ticket, outcome)
		if e != nil {
			result = held(q, mirror.Unknown)
			if err == nil {
				err = e
			}
			return
		}
		if !result.Fulfilled && !state.RetryAt.IsZero() {
			result.RetryAt = state.RetryAt
		}
		// Leave performs shared-store I/O after the query. It cannot be the last
		// boundary before rows escape: current permission and deadline apply here too.
		result, err = c.release(ctx, a, q, caller, result.Result, State{Reason: result.Reason, RetryAt: result.RetryAt}, err)
		if staleWaitErr != nil && (!result.Fulfilled || err != nil) {
			result = held(q, result.Reason)
			err = errors.Join(staleWaitErr, err)
		}
	}()
	if ticket.ID == "" || ticket.Generation == "" {
		return held(q, mirror.Unknown), ErrHeld
	}
	if ticket.Joined {
		c.observe(ctx, "coalesced-wait", "waiter", q.Scope, ticket.Generation)
	}
	if ticket.Owner {
		fillCtx, cancel := context.WithTimeout(c.lifetime, caller.Limits.FillTimeout)
		go func() { defer cancel(); c.fill(fillCtx, ticket, a, q, plan, caller) }()
	}
	waitCtx, cancel := context.WithTimeout(ctx, caller.Limits.WaitTimeout)
	defer cancel()
	state, err := c.gate.Wait(waitCtx, ticket)
	if err != nil {
		if !onlyStaleWait(err) {
			return held(q, mirror.Transport), err
		}
		staleWaitErr = err
		// Rotation invalidates the old wait result, not a sufficient current
		// snapshot. Requery without source work and leave the old outcome unchecked.
		if e := c.authority.Authorize(ctx, caller, q.Scope); e != nil {
			return held(q, mirror.Permission), errors.Join(err, e)
		}
		r, e := c.queryScope(ctx, a, q)
		if e != nil {
			return held(q, mirror.Unknown), errors.Join(err, e)
		}
		result, e = c.release(ctx, a, q, caller, r, State{}, nil)
		if e != nil || !result.Fulfilled {
			return held(q, result.Reason), errors.Join(err, e)
		}
		return result, nil
	}
	if state.Pending {
		return held(q, mirror.Unknown), ErrHeld
	}
	q.Now = c.clock.Now()
	// Always requery under the current admission, including after joining a fill.
	if e := c.authority.Authorize(ctx, caller, q.Scope); e != nil {
		return held(q, mirror.Permission), e
	}
	r, err = c.queryScope(ctx, a, q)
	if err != nil {
		return held(q, mirror.Unknown), err
	}
	result, err = c.release(ctx, a, q, caller, r, state, nil)
	if err == nil && r.Snapshot.Version != "" && ctx.Err() == nil {
		outcome = Outcome{Checked: true, Fulfilled: result.Fulfilled, SnapshotVersion: r.Snapshot.Version, Reason: result.Reason, Now: c.clock.Now()}
	}
	return result, err
}

func (c *Coordinator) fill(ctx context.Context, t Ticket, a mirror.Admission, q Query, plan mirror.ScanPlan, caller Caller) {
	complete := Completion{Query: q, Now: c.clock.Now(), Authority: c.authority, Caller: caller}
	finish := func() {
		// The gate bounds the complete call too; it never grants a retry on uncertainty.
		done, cancel := context.WithTimeout(context.WithoutCancel(c.lifetime), caller.Limits.FillTimeout)
		defer cancel()
		_ = c.gate.Complete(done, t, c.store, a, complete)
	}
	if err := c.authority.Authorize(ctx, caller, q.Scope); err != nil {
		complete.Failure = mirror.Permission
		complete.AdmissionDenied = true
		finish()
		return
	}
	if err := authorize(ctx, a, q.Scope); err != nil {
		complete.Failure = mirror.Permission
		complete.AdmissionDenied = true
		finish()
		return
	}
	q.Now = c.clock.Now()
	current, err := c.queryScope(ctx, a, q)
	if err != nil {
		complete.Failure = mirror.Unknown
		finish()
		return
	}
	q.Now = c.clock.Now()
	current, err = mirror.EvaluateSnapshot(current.Snapshot, q.Query)
	if err != nil {
		complete.Failure = mirror.Unknown
		finish()
		return
	}
	if sufficient(current, q) {
		complete.Query = q
		complete.Now = q.Now
		finish()
		return
	}
	baseline, err := c.store.Read(ctx, a, q.Scope)
	if err != nil || baseline.Scope != q.Scope {
		complete.Failure = mirror.Unknown
		finish()
		return
	}
	// An owner acquired after another commit must plan from that actual snapshot.
	q.Now = c.clock.Now()
	plan, err = mirror.PlanRefresh(baseline, q.Now, caller.Limits.Refresh)
	if err != nil {
		complete.Failure = mirror.Unknown
		finish()
		return
	}
	// Admitted composition has zero kernel backoff; Gate owns retry eligibility.
	if err := c.authority.Authorize(ctx, caller, q.Scope); err != nil {
		complete.AdmissionDenied = true
		complete.Failure = mirror.Permission
		finish()
		return
	}
	if err := authorize(ctx, a, q.Scope); err != nil {
		complete.AdmissionDenied = true
		complete.Failure = mirror.Permission
		finish()
		return
	}
	scan, scanErr := c.source.Scan(ctx, a, q.Scope, plan)
	complete.Now = c.clock.Now()
	complete.Query = q
	complete.Query.Now = complete.Now
	c.observe(ctx, "source-attempt", "scan-call", q.Scope, t.Generation)
	if scanErr == nil {
		scanErr = ctx.Err()
	}
	if errors.Is(scanErr, mirror.ErrPermission) {
		complete.AdmissionDenied = true
		complete.Failure = mirror.Permission
		finish()
		return
	}
	if scanErr != nil && scan.Failure == "" {
		scan.Failure = mirror.Transport
	}
	if scan.Failure != "" {
		scan.Complete = false
		scan.NotModified = false
		scan.Records = nil
	}
	qualifiedPopulation := scan.Complete || scan.NotModified
	if q.RequireReviews {
		if scan.NotModified {
			for _, o := range baseline.Records {
				if o.Reviews.State != mirror.Complete {
					scan.Complete = false
					scan.NotModified = false
					break
				}
			}
		}
		for _, o := range scan.Records {
			if o.Reviews.State != mirror.Complete {
				scan.Complete = false
				scan.NotModified = false
				break
			}
		}
	}
	update, err := mirror.PlanScan(baseline, plan, scan, complete.Now)
	if err != nil {
		complete.Failure = mirror.Unknown
		finish()
		return
	}
	complete.Update = &update
	if !scan.Complete && !qualifiedPopulation {
		complete.Failure = scan.Failure
		if complete.Failure == "" {
			complete.Failure = mirror.Truncated
		}
	}
	finish()
}

// queryScope chooses evaluation time after the guarded read. A concurrent commit
// may be newer than the read's start; actual future timestamps still fail the
// canonical evaluator. Source observation/watermark timestamps are never changed.
func (c *Coordinator) queryScope(ctx context.Context, a mirror.Admission, q Query) (mirror.Result, error) {
	q.Now = c.clock.Now()
	snapshot, reason, err := mirror.ReadScopeSnapshot(ctx, c.store, a, q.Query)
	if reason != "" {
		return held(q, reason).Result, err
	}
	if err != nil {
		return mirror.Result{}, err
	}
	q.Now = c.clock.Now()
	return mirror.EvaluateSnapshot(snapshot, q.Query)
}

// release is the single return boundary for snapshots. Checks before a blocking
// Store.Read do not establish current permission when that read finishes.
func (c *Coordinator) release(ctx context.Context, a mirror.Admission, q Query, caller Caller, r mirror.Result, state State, cause error) (Result, error) {
	if err := c.authority.Authorize(ctx, caller, q.Scope); err != nil {
		return held(q, mirror.Permission), errors.Join(cause, err)
	}
	if err := authorize(ctx, a, q.Scope); err != nil {
		return held(q, mirror.Permission), errors.Join(cause, err)
	}
	if err := ctx.Err(); err != nil {
		return held(q, mirror.Transport), errors.Join(cause, err)
	}
	q.Now = c.clock.Now()
	if err := ctx.Err(); err != nil {
		return held(q, mirror.Transport), errors.Join(cause, err)
	}
	evaluated, err := mirror.EvaluateSnapshot(r.Snapshot, q.Query)
	if err != nil {
		return held(q, mirror.Unknown), errors.Join(cause, err)
	}
	return present(evaluated, q, state), cause
}

func (c *Coordinator) observe(ctx context.Context, kind, unit string, scope mirror.Scope, generation string) {
	if !absent(c.observer) {
		_ = c.observer.Observe(ctx, Observation{Kind: kind, Unit: unit, Scope: scope, Generation: generation, Count: 1, Known: true})
	}
}

// onlyStaleWait accepts the typed generation refusal through ordinary wrapping.
// Aggregate failures can also contain a held/quota/deadline refusal and must not
// be promoted to a successful query merely because one cause is ErrStale.
func onlyStaleWait(err error) bool {
	for err != nil {
		if err == ErrStale {
			return true
		}
		wrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = wrapped.Unwrap()
	}
	return false
}
