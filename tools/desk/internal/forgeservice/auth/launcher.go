package auth

import (
	"context"
	"sync"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcontainer"
	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
)

type GrantSlot struct {
	LaunchID string
	Slot     int
}

// ProducerStart is passed only through the protected launcher exec path. It
// contains no private key and cannot initialize or register authority by itself.
type ProducerStart struct {
	Schema  string
	Config  ProducerConfig
	Binding ProducerBinding
	Grants  []GrantSlot
}
type ControlCall struct {
	Schema        string
	MaxFrameBytes int
	IOTimeout     time.Duration
	Command       *OperatorCommand
}

type LauncherRoute struct {
	Runtime *cellcontainer.AuthorityRuntime
	Handle  *cellcontainer.AuthorityHandle
	Scope   GrantScope
}
type pendingControl struct {
	previous uint64
	digest   string
}

// Launcher is the protected operator writer. No instance is passed to a query
// consumer; only a verifier and read-only bootstrap/current view reach that role.
type Launcher struct {
	mu       sync.Mutex
	producer *cellcontainer.AuthorityRuntime
	handle   *cellcontainer.AuthorityHandle
	config   ProducerConfig
	start    ProducerStart
	routes   map[string]LauncherRoute
	pending  *pendingControl
}

func NewLauncher(ctx context.Context, producer *cellcontainer.AuthorityRuntime, handle *cellcontainer.AuthorityHandle, cfg ProducerConfig, routes []LauncherRoute) (*Launcher, error) {
	if ctx == nil || ctx.Err() != nil || producer == nil || handle == nil || len(cfg.Launches) != 0 || len(routes) == 0 {
		return nil, ErrConfig
	}
	p := producer.Configuration()
	intent := handle.Intent()
	if p.Kind != celllaunch.AuthorityProducer || intent.Kind != celllaunch.AuthorityProducer || intent.Cell != p.Cell || intent.Validate() != nil || len(routes) != len(p.Volumes.Grants) {
		return nil, ErrConfig
	}
	volumes := map[string]cellcontainer.AuthorityGrantVolume{}
	for _, g := range p.Volumes.Grants {
		volumes[g.LaunchID] = g
	}
	ownedRoutes := map[string]LauncherRoute{}
	var slots []GrantSlot
	for _, route := range routes {
		if route.Runtime == nil || route.Handle == nil {
			return nil, ErrConfig
		}
		c := route.Runtime.Configuration()
		launch := route.Handle.Intent()
		g, ok := volumes[launch.LaunchID]
		if !ok || ownedRoutes[launch.LaunchID].Handle != nil || c.Kind != celllaunch.AuthorityCaller || launch.Kind != celllaunch.AuthorityCaller || c.Host != p.Host || c.Engine != p.Engine || c.Owner != p.Owner || c.Cell != p.Cell || c.Image != p.Image || c.Platform != p.Platform || !same(c.Volumes, p.Volumes) || route.Scope.Cell != c.Cell || route.Scope.Role != c.Role {
			return nil, ErrConfig
		}
		cfg.Launches = append(cfg.Launches, LaunchPolicy{ID: launch.LaunchID, GrantVolume: g.Volume, Scope: route.Scope, Launch: launch})
		slots = append(slots, GrantSlot{LaunchID: launch.LaunchID, Slot: g.Slot})
		ownedRoutes[launch.LaunchID] = route
	}
	if CheckConfig(cfg) != nil {
		return nil, ErrConfig
	}
	start := ProducerStart{Schema: Schema, Config: cfg, Binding: ProducerBinding{Engine: intent.Container.Engine, ContainerID: intent.Container.ID, PlanDigest: intent.Container.PolicyDigest, Cell: intent.Cell, PrivateVolume: p.Volumes.Private, CurrentVolume: p.Volumes.Current, BootstrapVolume: p.Volumes.Bootstrap}, Grants: slots}
	data, e := encode(start, cfg.MaxFrameBytes)
	if e != nil {
		return nil, ErrConfig
	}
	// Detach the operator input before retaining any field.
	var owned ProducerStart
	if canonicalDecode(data, cfg.MaxFrameBytes, &owned) != nil {
		return nil, ErrConfig
	}
	policies := map[string]LaunchPolicy{}
	for _, x := range owned.Config.Launches {
		policies[x.ID] = x
	}
	if checkFrames(owned.Config, owned.Binding, policies) != nil {
		return nil, ErrConfig
	}
	// These are actual lower-engine observations, after pure policy validation.
	o, e := producer.Observe(ctx, handle)
	if e != nil || intent.Match(o) != nil {
		return nil, ErrHeld
	}
	for _, r := range ownedRoutes {
		o, e := r.Runtime.Observe(ctx, r.Handle)
		if e != nil || r.Handle.Intent().Match(o) != nil {
			return nil, ErrHeld
		}
	}
	return &Launcher{producer: producer, handle: handle, config: owned.Config, start: owned, routes: ownedRoutes}, nil
}

func (l *Launcher) Run(ctx context.Context) error {
	if l == nil || ctx == nil || ctx.Err() != nil {
		return ErrHeld
	}
	for _, r := range l.routes {
		o, e := r.Runtime.Observe(ctx, r.Handle)
		if e != nil || r.Handle.Intent().Match(o) != nil {
			return ErrHeld
		}
	}
	b, e := encode(l.start, l.config.MaxFrameBytes)
	if e != nil {
		return ErrConfig
	}
	return l.producer.RunProducer(ctx, l.handle, b)
}

func (l *Launcher) read(ctx context.Context) (ControlReceipt, error) {
	b, e := encode(ControlCall{Schema: Schema, MaxFrameBytes: l.config.MaxFrameBytes, IOTimeout: l.config.IOTimeout}, l.config.MaxFrameBytes)
	if e != nil {
		return ControlReceipt{}, ErrConfig
	}
	out, e := l.producer.Control(ctx, l.handle, b)
	if e != nil {
		return ControlReceipt{}, ErrHeld
	}
	var r ControlReceipt
	if canonicalDecode(out, l.config.MaxFrameBytes, &r) != nil || !hexID(r.TupleDigest) {
		return ControlReceipt{}, ErrHeld
	}
	return r, nil
}

func (l *Launcher) ReadControl(ctx context.Context) (ControlReceipt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, e := l.read(ctx)
	if e != nil {
		return ControlReceipt{}, e
	}
	if l.pending != nil && r.Version == l.pending.previous+1 && r.OperationDigest == l.pending.digest {
		l.pending = nil
	}
	return r, nil
}

func (l *Launcher) mutate(ctx context.Context, x OperatorCommand) (ControlReceipt, error) {
	if l.pending != nil {
		return ControlReceipt{}, ErrHeld
	}
	state, e := l.read(ctx)
	if e != nil {
		return ControlReceipt{}, e
	}
	if x.Operation == Initialize && state.Version != 0 {
		return ControlReceipt{}, ErrConflict
	}
	x.ExpectedVersion = state.Version
	b, e := encode(ControlCall{Schema: Schema, MaxFrameBytes: l.config.MaxFrameBytes, IOTimeout: l.config.IOTimeout, Command: &x}, l.config.MaxFrameBytes)
	if e != nil {
		return ControlReceipt{}, ErrConfig
	}
	l.pending = &pendingControl{previous: state.Version, digest: operationDigest(x)}
	out, e := l.producer.Control(ctx, l.handle, b)
	if e != nil {
		return ControlReceipt{}, ErrHeld
	}
	var result ControlReceipt
	if canonicalDecode(out, l.config.MaxFrameBytes, &result) != nil || result.Version != state.Version+1 || result.OperationDigest != l.pending.digest || !hexID(result.TupleDigest) {
		return ControlReceipt{}, ErrHeld
	}
	l.pending = nil
	return result, nil
}

func (l *Launcher) Initialize(ctx context.Context) (ControlReceipt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.mutate(ctx, OperatorCommand{Operation: Initialize})
}
func (l *Launcher) Rotate(ctx context.Context) (ControlReceipt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.mutate(ctx, OperatorCommand{Operation: Rotate})
}
func (l *Launcher) Stop(ctx context.Context) (ControlReceipt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.mutate(ctx, OperatorCommand{Operation: Stop})
}
func (l *Launcher) Revoke(ctx context.Context, id string) (ControlReceipt, error) {
	if !hexID(id) {
		return ControlReceipt{}, ErrConfig
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.mutate(ctx, OperatorCommand{Operation: Revoke, GrantID: id})
}
func (l *Launcher) Register(ctx context.Context, id string, lease time.Duration) (ControlReceipt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, ok := l.routes[id]
	if !ok || lease <= 0 || lease > l.config.MaxGrantLease {
		return ControlReceipt{}, ErrConfig
	}
	o, e := r.Runtime.Observe(ctx, r.Handle)
	if e != nil || r.Handle.Intent().Match(o) != nil {
		return ControlReceipt{}, ErrHeld
	}
	return l.mutate(ctx, OperatorCommand{Operation: Register, LaunchID: id, Observation: &o, Lease: lease})
}
func (l *Launcher) Advance(ctx context.Context, id string, policy GrantScope) (ControlReceipt, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r, ok := l.routes[id]
	if !ok || !policy.valid() {
		return ControlReceipt{}, ErrConfig
	}
	o, e := r.Runtime.Observe(ctx, r.Handle)
	if e != nil || r.Handle.Intent().Match(o) != nil {
		return ControlReceipt{}, ErrHeld
	}
	return l.mutate(ctx, OperatorCommand{Operation: Advance, LaunchID: id, Observation: &o, Policy: &policy})
}

// EncodeProducerStart is a bounded operator transport helper, not a mint API.
func EncodeProducerStart(s ProducerStart) ([]byte, error) { return encode(s, s.Config.MaxFrameBytes) }

// DecodeProducerStart validates policy and fixed slot bindings before the helper
// opens any protected root. The actual producer creates a fresh empty registry.
func DecodeProducerStart(b []byte) (ProducerStart, error) {
	var s ProducerStart
	if canonicalDecode(b, celllaunch.MaxRecordBytes, &s) != nil || s.Schema != Schema || CheckConfig(s.Config) != nil || !s.Binding.valid() || len(s.Grants) != len(s.Config.Launches) {
		return ProducerStart{}, ErrConfig
	}
	seen := map[string]bool{}
	slots := map[int]bool{}
	for _, g := range s.Grants {
		if !name(g.LaunchID) || g.Slot < 0 || g.Slot >= cellcontainer.AuthorityGrantSlots || seen[g.LaunchID] || slots[g.Slot] {
			return ProducerStart{}, ErrConfig
		}
		seen[g.LaunchID] = true
		slots[g.Slot] = true
	}
	for _, p := range s.Config.Launches {
		if !seen[p.ID] {
			return ProducerStart{}, ErrConfig
		}
	}
	return s, nil
}

// DecodeControlCall validates the fixed protected-launcher envelope before the
// helper attempts its private socket. It is not used by either data channel.
func DecodeControlCall(b []byte) (ControlCall, error) {
	var c ControlCall
	if canonicalDecode(b, celllaunch.MaxRecordBytes, &c) != nil || c.Schema != Schema || c.MaxFrameBytes <= 0 || c.MaxFrameBytes > celllaunch.MaxRecordBytes || c.IOTimeout <= 0 {
		return ControlCall{}, ErrConfig
	}
	return c, nil
}
