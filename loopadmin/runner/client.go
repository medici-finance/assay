package runner

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Client is the caller API over one Adapter: start, observe, cancel, reconcile
// and result acceptance, with every check the contract owns applied on the way.
// It keeps invocation state in memory only; durable launch journaling belongs
// to the supervisor, which wraps this contract. A Client serializes its own
// calls.
//
// An error an adapter or a fence returns reaches the caller only inside the
// error chain, behind one of the contract's sentinels (errors.Is matches both),
// never in the returned error's text: an adapter error may carry request,
// result or credential content, and a caller that logs refusals must not log
// it. The Client also keeps deep copies of what it records and returns copies,
// so no one sharing a slice, map or pointer with it can rewrite its state.
type Client struct {
	adapter Adapter
	caps    Capabilities
	fence   Fence

	mu   sync.Mutex
	inv  map[string]*invocation
	held map[string]string // authority key -> invocation key that still holds it
}

type invocation struct {
	req       LaunchRequest
	state     State
	usage     Usage
	obs       *Observation
	cancelled bool // a cancel was acknowledged without error
	fellBack  bool // an adapter ran a model other than the pinned one
}

// NewClient wraps an adapter. The fence reads the caller's current authority
// generation; a nil fence is a programming error and refuses every acceptance.
func NewClient(a Adapter, f Fence) *Client {
	return &Client{
		adapter: a, caps: a.Capabilities().clone(), fence: f,
		inv: map[string]*invocation{}, held: map[string]string{},
	}
}

// Accepted is a result that passed the contract: the invocation finished, its
// generation is current, the model and tools match the pinned profile and the
// result is well formed. It is NOT the caller's decision to take the work: the
// caller still rechecks its own authority before it acts on this.
type Accepted struct {
	Ref         Ref
	Authority   Authority
	Result      Result
	Usage       Usage
	UsageKnown  bool
	ActualModel string
}

// Start records the launch intent, then asks the adapter to start. A definite
// adapter failure frees the authority; any other error leaves the invocation
// unknown and refuses all further launches under that authority until it is
// reconciled.
func (c *Client) Start(ctx context.Context, req LaunchRequest) (Ref, State, error) {
	req = req.clone() // checked, recorded and sent as this copy only
	if err := req.Validate(); err != nil {
		return Ref{}, "", err
	}
	if err := req.CheckCapabilities(c.caps); err != nil {
		return Ref{}, "", err
	}
	ref := req.Ref()
	c.mu.Lock()
	defer c.mu.Unlock()
	if prev, ok := c.inv[ref.key()]; ok {
		if !reflect.DeepEqual(prev.req, req) {
			return Ref{}, "", ErrIdentityReuse
		}
		if prev.state == StateUnknown {
			return ref, prev.state, ErrReconcileRequired
		}
		return ref, prev.state, nil
	}
	if holder, ok := c.held[req.Authority.Key]; ok && c.inv[holder].state.holds() {
		return Ref{}, "", fmt.Errorf("%w: another invocation still holds this authority", ErrReconcileRequired)
	}
	// Intent first: the invocation exists as unknown before the adapter is
	// asked, so a crash or a lost acknowledgment can never leave a launch the
	// client does not know about.
	in := &invocation{req: req, state: StateUnknown}
	c.inv[ref.key()] = in
	c.held[req.Authority.Key] = ref.key()
	rec, aerr := c.adapter.Start(ctx, req.clone())
	if aerr != nil {
		if errors.Is(aerr, ErrDefiniteFailure) {
			in.state = StateFailed
			return ref, in.state, &opaque{sentinel: ErrDefiniteFailure, cause: aerr}
		}
		return ref, in.state, &opaque{sentinel: ErrReconcileRequired, cause: aerr}
	}
	in.state = StateRunning
	// A receipt that names no model is not yet a fallback: the model is
	// unknown, and acceptance refuses a result whose model stays unreported.
	if rec.ActualModel != "" && rec.ActualModel != req.Profile.Model {
		in.fellBack = true
		return ref, in.state, ErrModelFallback
	}
	return ref, in.state, nil
}

// Observe asks the adapter for the invocation's state and records it. An
// adapter error leaves the recorded state as it was: not being able to look is
// not an outcome.
func (c *Client) Observe(ctx context.Context, ref Ref) (Observation, error) {
	return c.look(ctx, ref, c.adapter.Observe)
}

// Reconcile looks the invocation up by identity. An absent record settles the
// launch as failed only when the adapter declares launch-dedupe; otherwise the
// invocation stays unknown and held.
func (c *Client) Reconcile(ctx context.Context, ref Ref) (Observation, error) {
	return c.look(ctx, ref, c.adapter.Reconcile)
}

func (c *Client) look(ctx context.Context, ref Ref, q func(context.Context, Ref) (Observation, error)) (Observation, error) {
	in, err := c.get(ref)
	if err != nil {
		return Observation{}, err
	}
	obs, aerr := q(ctx, ref)
	c.mu.Lock()
	defer c.mu.Unlock()
	if aerr != nil {
		return Observation{}, &opaque{sentinel: ErrReconcileRequired, cause: aerr}
	}
	obs = obs.clone() // the adapter keeps no handle on what is recorded
	if obs.State == StateAbsent {
		if c.caps.LaunchDedupe && in.state == StateUnknown {
			in.state = StateFailed
		}
		return obs, nil
	}
	switch obs.State {
	case StateUnknown, StateRunning, StateFinished, StateFailed, StateCancelRequested, StateStopped:
	default:
		// A state the contract does not define is not an outcome.
		obs.State = StateUnknown
	}
	if in.state.terminal() {
		// Terminal states are absorbing. A failed or stopped attempt reported
		// running again, or a finished one reported with another result, would
		// put a second holder back on an authority already handed to a
		// replacement, or swap an accepted result. Refuse the report and keep
		// the recorded state and observation.
		if obs.State != in.state || (in.state == StateFinished && !sameOutcome(in.obs, obs)) {
			return Observation{}, ErrStateRegression
		}
		if in.obs != nil {
			return in.obs.clone(), nil
		}
		return obs, nil
	}
	in.state = obs.State
	if in.cancelled && obs.State == StateRunning {
		in.state = StateCancelRequested
	}
	if c.caps.Telemetry == TelemetryNone {
		obs.Usage = Usage{}
	}
	obs.Usage = obs.Usage.readings()
	kept := obs.clone() // the caller gets obs, which shares nothing with kept
	in.usage = kept.Usage
	in.obs = &kept
	if obs.ActualModel != "" && obs.ActualModel != in.req.Profile.Model {
		in.fellBack = true
		return obs, ErrModelFallback
	}
	return obs, nil
}

// Cancel requests a stop. The acknowledgment is reported only when the adapter
// declares cancel-ack, and either way the invocation is CancelRequested, never
// stopped: only an observation of a stopped invocation confirms a stop.
//
// An adapter error leaves the recorded state unchanged and returns
// ErrCancelFailed: the request may not have been taken.
func (c *Client) Cancel(ctx context.Context, ref Ref) (CancelAck, error) {
	in, err := c.get(ref)
	if err != nil {
		return CancelAck{}, err
	}
	ack, aerr := c.adapter.Cancel(ctx, ref)
	c.mu.Lock()
	defer c.mu.Unlock()
	if aerr != nil {
		return CancelAck{}, &opaque{sentinel: ErrCancelFailed, cause: aerr}
	}
	in.cancelled = true
	if in.state == StateRunning {
		in.state = StateCancelRequested
	}
	return CancelAck{Acknowledged: ack.Acknowledged && c.caps.CancelAck}, nil
}

// State returns the recorded state.
func (c *Client) State(ref Ref) (State, error) {
	in, err := c.get(ref)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return in.state, nil
}

func (c *Client) get(ref Ref) (*invocation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	in, ok := c.inv[ref.key()]
	if !ok || in.req.Authority.Generation != ref.Generation {
		return nil, ErrUnknownInvocation
	}
	return in, nil
}

// AcceptResult applies the contract's acceptance checks to a finished
// invocation. The generation fence comes first: an attempt whose generation is
// no longer current is refused however well formed its result is.
func (c *Client) AcceptResult(ctx context.Context, ref Ref) (Accepted, error) {
	in, err := c.get(ref)
	if err != nil {
		return Accepted{}, err
	}
	c.mu.Lock()
	state, obs, req, fellBack := in.state, in.obs, in.req, in.fellBack
	if obs != nil {
		kept := obs.clone() // the Accepted below shares nothing with the record
		obs = &kept
	}
	c.mu.Unlock()
	if state != StateFinished || obs == nil || obs.Result == nil {
		return Accepted{}, ErrNotFinished
	}
	if c.fence == nil {
		return Accepted{}, ErrFenceUnavailable
	}
	cur, ferr := c.fence.CurrentGeneration(ctx, req.Authority.Key)
	if ferr != nil {
		return Accepted{}, &opaque{sentinel: ErrFenceUnavailable, cause: ferr}
	}
	if cur != req.Authority.Generation {
		return Accepted{}, fmt.Errorf("%w: attempt ran under %d, current is %d", ErrFenced, req.Authority.Generation, cur)
	}
	if fellBack {
		return Accepted{}, ErrModelFallback
	}
	// Unknown stays unknown: a model nobody reported is not the pinned model.
	if obs.ActualModel == "" {
		return Accepted{}, ErrModelUnreported
	}
	if obs.ActualModel != req.Profile.Model {
		return Accepted{}, ErrModelFallback
	}
	res := *obs.Result
	if err := res.Validate(); err != nil {
		return Accepted{}, err
	}
	if res.Generation != req.Authority.Generation {
		return Accepted{}, fmt.Errorf("%w: result echoes generation %d, attempt ran under %d", ErrFenced, res.Generation, req.Authority.Generation)
	}
	if res.Caller != ref.Caller || res.ID != ref.ID {
		return Accepted{}, ErrResultIdentity
	}
	for i, t := range res.ToolRequests {
		if !contains(req.Profile.Tools, t.Name) {
			return Accepted{}, fmt.Errorf("%w: tool request %d", ErrUnauthorizedTool, i)
		}
	}
	return Accepted{
		Ref: ref, Authority: req.Authority, Result: res,
		Usage: obs.Usage, UsageKnown: obs.Usage.Complete(), ActualModel: obs.ActualModel,
	}, nil
}

// Usage totals the latest usage of every invocation made under a caller
// namespace. A figure unknown for any invocation is unknown in the total.
func (c *Client) Usage(caller string) Usage {
	c.mu.Lock()
	defer c.mu.Unlock()
	total := Usage{InputTokens: Int64(0), OutputTokens: Int64(0), CostMicros: Int64(0)}
	for _, in := range c.inv {
		if in.req.Caller == caller {
			total = total.Add(in.usage)
		}
	}
	return total
}

// sameOutcome reports whether a later finished observation repeats the
// recorded one: the same result and the same reported model.
func sameOutcome(kept *Observation, obs Observation) bool {
	return kept != nil && kept.ActualModel == obs.ActualModel && reflect.DeepEqual(kept.Result, obs.Result)
}

func contains(list []string, s string) bool {
	for _, l := range list {
		if l == s {
			return true
		}
	}
	return false
}
