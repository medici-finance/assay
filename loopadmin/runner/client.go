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
		adapter: a, caps: a.Capabilities(), fence: f,
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
		return Ref{}, "", fmt.Errorf("%w: %s still holds authority %q", ErrReconcileRequired, c.inv[holder].req.ID, req.Authority.Key)
	}
	// Intent first: the invocation exists as unknown before the adapter is
	// asked, so a crash or a lost acknowledgment can never leave a launch the
	// client does not know about.
	in := &invocation{req: req, state: StateUnknown}
	c.inv[ref.key()] = in
	c.held[req.Authority.Key] = ref.key()
	rec, err := c.adapter.Start(ctx, req)
	if err != nil {
		if errors.Is(err, ErrDefiniteFailure) {
			in.state = StateFailed
			return ref, in.state, err
		}
		return ref, in.state, fmt.Errorf("%w: %v", ErrReconcileRequired, err)
	}
	in.state = StateRunning
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
	obs, err := q(ctx, ref)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		return Observation{}, fmt.Errorf("%w: could not check: %v", ErrReconcileRequired, err)
	}
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
	in.state = obs.State
	if in.cancelled && obs.State == StateRunning {
		in.state = StateCancelRequested
	}
	if c.caps.Telemetry == TelemetryNone {
		obs.Usage = Usage{}
	}
	in.usage = obs.Usage
	kept := obs
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
func (c *Client) Cancel(ctx context.Context, ref Ref) (CancelAck, error) {
	in, err := c.get(ref)
	if err != nil {
		return CancelAck{}, err
	}
	ack, err := c.adapter.Cancel(ctx, ref)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		return CancelAck{}, err
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
	c.mu.Unlock()
	if state != StateFinished || obs == nil || obs.Result == nil {
		return Accepted{}, fmt.Errorf("%w: state is %s", ErrNotFinished, state)
	}
	if c.fence == nil {
		return Accepted{}, ErrFenceUnavailable
	}
	cur, err := c.fence.CurrentGeneration(ctx, req.Authority.Key)
	if err != nil {
		return Accepted{}, fmt.Errorf("%w: %v", ErrFenceUnavailable, err)
	}
	if cur != req.Authority.Generation {
		return Accepted{}, fmt.Errorf("%w: attempt ran under %d, current is %d", ErrFenced, req.Authority.Generation, cur)
	}
	if fellBack {
		return Accepted{}, ErrModelFallback
	}
	res := *obs.Result
	if err := res.Validate(); err != nil {
		return Accepted{}, err
	}
	if res.Generation != req.Authority.Generation {
		return Accepted{}, fmt.Errorf("%w: result echoes generation %d, attempt ran under %d", ErrFenced, res.Generation, req.Authority.Generation)
	}
	for _, t := range res.ToolRequests {
		if !contains(req.Profile.Tools, t.Name) {
			return Accepted{}, fmt.Errorf("%w: %q", ErrUnauthorizedTool, t.Name)
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

func contains(list []string, s string) bool {
	for _, l := range list {
		if l == s {
			return true
		}
	}
	return false
}
