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
	r, err := mirror.QueryScope(ctx, c.store, a, q.Query)
	if err != nil {
		return held(q, mirror.Unknown), err
	}
	if sufficient(r, q) {
		out, e := c.release(ctx, a, q, caller, r, State{}, nil)
		if e == nil && out.Fulfilled {
			c.observe(ctx, "cache-hit", "query", q.Scope, "")
		}
		return out, e
	}
	baseline, err := c.store.Read(ctx, a, q.Scope)
	if err != nil {
		return held(q, mirror.Transport), err
	}
	if baseline.Scope != q.Scope {
		return held(q, mirror.Unknown), mirror.ErrInvalid
	}
	plan, err := mirror.PlanRefresh(baseline, q.Now, caller.Limits.Refresh)
	if err != nil {
		return c.release(ctx, a, q, caller, r, State{}, err)
	}
	key := FillKey{Scope: q.Scope, SourceID: c.sourceID, AccessEpoch: caller.AccessEpoch, ConfigEpoch: caller.ConfigEpoch, PolicyClass: caller.PolicyClass, PolicyEpoch: caller.PolicyEpoch}
	ticket, err := c.gate.Acquire(ctx, Demand{Key: key, CallerID: caller.ID, Limits: caller.Limits, Now: q.Now})
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
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(c.lifetime), caller.Limits.WaitTimeout)
		defer cancel()
		if e := c.gate.Leave(cleanup, ticket); e != nil {
			result = held(q, mirror.Unknown)
			if err == nil {
				err = e
			}
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
		return held(q, mirror.Transport), err
	}
	if state.Pending {
		return held(q, mirror.Unknown), ErrHeld
	}
	q.Now = c.clock.Now()
	// Always requery under the current admission, including after joining a fill.
	if e := c.authority.Authorize(ctx, caller, q.Scope); e != nil {
		return held(q, mirror.Permission), e
	}
	r, err = mirror.QueryScope(ctx, c.store, a, q.Query)
	if err != nil {
		return held(q, mirror.Unknown), err
	}
	return c.release(ctx, a, q, caller, r, state, nil)
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
	current, err := mirror.QueryScope(ctx, c.store, a, q.Query)
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
	plan, err = mirror.PlanRefresh(baseline, q.Now, caller.Limits.Refresh)
	if err != nil {
		complete.Failure = mirror.Unknown
		finish()
		return
	}
	if q.Now.Before(plan.NotBefore) {
		complete.Failure = mirror.Unknown
		finish()
		return
	}
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
	c.observe(ctx, "source-attempt", "scan-call", q.Scope, t.Generation)
	scan, scanErr := c.source.Scan(ctx, a, q.Scope, plan)
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
	complete.Now = c.clock.Now()
	complete.Query = q
	complete.Query.Now = complete.Now
	update, err := mirror.PlanScan(baseline, plan, scan, complete.Now)
	if err != nil {
		complete.Failure = mirror.Unknown
		finish()
		return
	}
	complete.Update = &update
	if !scan.Complete {
		complete.Failure = scan.Failure
		if complete.Failure == "" {
			complete.Failure = mirror.Truncated
		}
	}
	finish()
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
	return present(r, q, state), cause
}

func (c *Coordinator) observe(ctx context.Context, kind, unit string, scope mirror.Scope, generation string) {
	if !absent(c.observer) {
		_ = c.observer.Observe(ctx, Observation{Kind: kind, Unit: unit, Scope: scope, Generation: generation, Count: 1, Known: true})
	}
}
