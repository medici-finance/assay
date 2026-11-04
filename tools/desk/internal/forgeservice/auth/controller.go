package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/medici-finance/assay/comms/contract"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

type registration struct {
	claims  GrantClaims
	grant   SignedGrant
	until   time.Time // private monotonic deadline; never reconstructed from disk
	revoked bool
}

type channelKey struct {
	root *os.Root
	name string
}

// Controller is a live operator-owned registry. It never imports a signing key
// or resumes authority from a saved record. Query roles receive no Controller.
type Controller struct {
	mu                  sync.Mutex
	config              ProducerConfig
	resources           ProducerResources
	clock               refresh.Clock
	gate                *deskkit.HeldLock
	gateInfo            os.FileInfo
	rootInfo            []os.FileInfo
	roots               []*os.Root
	bootstrap           Bootstrap
	signer              contract.Ed25519Signer
	version, generation uint64
	lastOperation       string
	ready, closed       bool
	entries             map[string]*registration
	byLaunch            map[string]string
	policies            map[string]LaunchPolicy
	dataSlots           chan struct{}
	controlSlots        chan struct{}
	channels            map[channelKey]bool
}

func NewController(cfg ProducerConfig, r ProducerResources, wall refresh.Clock) (*Controller, error) {
	if CheckConfig(cfg) != nil || !clockPresent(wall) || !r.Binding.valid() || r.Private == nil || r.Current == nil || r.Bootstrap == nil || len(r.Grants) != len(cfg.Launches) {
		return nil, ErrConfig
	}
	// Detach trusted configuration so later caller mutation cannot change policy.
	b, err := encode(cfg, cfg.MaxFrameBytes)
	if err != nil {
		return nil, ErrConfig
	}
	var owned ProducerConfig
	if json.Unmarshal(b, &owned) != nil {
		return nil, ErrConfig
	}
	policies := make(map[string]LaunchPolicy, len(owned.Launches))
	grantRoots := make(map[string]*os.Root, len(r.Grants))
	roots := []*os.Root{r.Private, r.Current, r.Bootstrap}
	seenVolumes := map[string]bool{r.Binding.PrivateVolume: true, r.Binding.CurrentVolume: true, r.Binding.BootstrapVolume: true}
	for _, p := range owned.Launches {
		if p.Scope.Cell != r.Binding.Cell || p.Launch.Container.Engine != r.Binding.Engine || p.Launch.Container.ID == r.Binding.ContainerID || seenVolumes[p.GrantVolume] || r.Grants[p.ID] == nil {
			return nil, ErrConfig
		}

		seenVolumes[p.GrantVolume] = true
		policies[p.ID] = p
		grantRoots[p.ID] = r.Grants[p.ID]
		roots = append(roots, r.Grants[p.ID])
	}
	if checkFrames(owned, r.Binding, policies) != nil {
		return nil, ErrConfig
	}
	// All shape/policy validation above precedes filesystem, key and listener I/O.
	infos := make([]os.FileInfo, 0, len(roots))
	for _, root := range roots {
		if !filepath.IsAbs(root.Name()) {
			return nil, ErrConfig
		}
		st, e := root.Stat(".")
		if e != nil || !st.IsDir() {
			return nil, ErrHeld
		}
		at, e := os.Stat(root.Name())
		if e != nil || !os.SameFile(st, at) {
			return nil, ErrHeld
		}
		for _, prior := range infos {
			if os.SameFile(st, prior) {
				return nil, ErrConfig
			}
		}
		infos = append(infos, st)
	}
	if st, e := r.Private.Lstat("live.lock"); e == nil && (!st.Mode().IsRegular() || st.Mode().Perm() != 0600) {
		return nil, ErrHeld
	} else if e != nil && !os.IsNotExist(e) {
		return nil, ErrHeld
	}
	f, err := r.Private.OpenFile("live.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, ErrHeld
	}
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Mode().Perm() != 0600 {
		f.Close()
		return nil, ErrHeld
	}
	h, err := deskkit.AcquireHeld(r.Private, "live.lock", f, deskkit.LockSpec{Name: "local authority producer", Cell: r.Binding.Cell, Role: "operator", Wait: 0, EndStale: false})
	if err != nil {
		return nil, ErrHeld
	}
	r.Grants = grantRoots
	c := &Controller{config: owned, resources: r, clock: wall, gate: h, gateInfo: st, rootInfo: infos, roots: roots, entries: map[string]*registration{}, byLaunch: map[string]string{}, policies: policies, dataSlots: make(chan struct{}, owned.MaxConnections), controlSlots: make(chan struct{}, 1)}
	if !c.liveGate() {
		c.Close()
		return nil, ErrHeld
	}
	return c, nil
}

func (c *Controller) liveGate() bool {
	if c == nil || c.closed || c.gate == nil {
		return false
	}
	for i, r := range c.roots {
		st, e := r.Stat(".")
		if e != nil || !os.SameFile(st, c.rootInfo[i]) {
			return false
		}
		at, e := os.Stat(r.Name())
		if e != nil || !os.SameFile(at, st) {
			return false
		}
	}
	at, e := c.resources.Private.Lstat("live.lock")
	if e != nil || !at.Mode().IsRegular() || !os.SameFile(at, c.gateInfo) {
		return false
	}
	fd, e := c.gate.File().Stat()
	return e == nil && os.SameFile(at, fd)
}

func freshID() (string, error) {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", ErrHeld
	}
	return hex.EncodeToString(b[:]), nil
}

func (c *Controller) receipt() ControlReceipt {
	b, _ := json.Marshal(struct {
		Version, Generation uint64
		Ready               bool
		Instance            string
	}{c.version, c.generation, c.ready, c.bootstrap.Instance})
	return ControlReceipt{Version: c.version, Ready: c.ready, TupleDigest: digest(b), OperationDigest: c.lastOperation}
}

// ControlState is an operator read-back only. It returns no grant or signing key.
func (c *Controller) ControlState(ctx context.Context) (ControlReceipt, error) {
	if ctx == nil {
		return ControlReceipt{}, ErrHeld
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil || !c.liveGate() {
		return ControlReceipt{}, ErrHeld
	}
	return c.receipt(), nil
}

func (c *Controller) ApplyControl(ctx context.Context, cmd ControlCommand) (ControlReceipt, error) {
	if ctx == nil {
		return ControlReceipt{}, ErrHeld
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil || !c.liveGate() {
		return ControlReceipt{}, ErrHeld
	}
	if cmd.ExpectedVersion != c.version {
		return ControlReceipt{}, ErrConflict
	}
	switch cmd.Operation {
	case Initialize, Rotate:
		if cmd.Launch != nil || cmd.GrantID != "" || cmd.Lease != 0 || cmd.Policy != nil || (cmd.Operation == Initialize && c.version != 0) || (cmd.Operation == Rotate && c.version == 0) {
			return ControlReceipt{}, ErrConfig
		}
		c.ready = false
		pub, key, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			return ControlReceipt{}, ErrHeld
		}
		instance, e := freshID()
		if e != nil {
			return ControlReceipt{}, e
		}
		incarnation, e := freshID()
		if e != nil {
			return ControlReceipt{}, e
		}
		boot := Bootstrap{Schema: Schema, Binding: c.resources.Binding, Instance: instance, Incarnation: incarnation, KeyRef: digest(pub), PublicKey: pub}
		data, e := encode(boot, c.config.MaxFrameBytes)
		if e != nil {
			return ControlReceipt{}, e
		}
		name := "bootstrap-" + instance + ".tmp"
		f, e := c.resources.Bootstrap.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return ControlReceipt{}, ErrHeld
		}
		_, we := f.Write(data)
		se := f.Sync()
		ce := f.Close()
		if we != nil || se != nil || ce != nil {
			c.resources.Bootstrap.Remove(name)
			return ControlReceipt{}, ErrHeld
		}
		if c.resources.Bootstrap.Rename(name, "bootstrap.json") != nil {
			c.resources.Bootstrap.Remove(name)
			return ControlReceipt{}, ErrHeld
		}
		if e = c.publishVerifierConfigs(boot); e != nil {
			return ControlReceipt{}, e
		}
		clear(c.signer.Key)
		c.signer = contract.Ed25519Signer{Key: key}
		c.bootstrap = boot
		c.generation++
		c.version++
		c.entries = map[string]*registration{}
		c.byLaunch = map[string]string{}
		c.ready = true
	case Register:
		if !c.ready || cmd.Launch == nil || cmd.GrantID != "" || cmd.Policy != nil || cmd.Lease <= 0 || cmd.Lease > c.config.MaxGrantLease || len(c.entries) >= c.config.MaxGrants {
			return ControlReceipt{}, ErrConfig
		}
		p, ok := c.policies[cmd.Launch.ID]
		if !ok || (cmd.Launch.Permit.Check(p.Launch) != nil || p.Launch.Match(cmd.Launch.Observation) != nil) {
			return ControlReceipt{}, ErrHeld
		}
		if id := c.byLaunch[p.ID]; id != "" && !c.entries[id].revoked {
			return ControlReceipt{}, ErrConflict
		}
		began := time.Now()
		now := c.clock.Now().UTC()
		if now.IsZero() || ctx.Err() != nil {
			return ControlReceipt{}, ErrHeld
		}
		id, e := freshID()
		if e != nil {
			return ControlReceipt{}, e
		}
		claims := GrantClaims{Schema: Schema, Operation: Operation, Issuer: c.config.Issuer, Audience: c.config.Audience, Binding: c.resources.Binding, Instance: c.bootstrap.Instance, Incarnation: c.bootstrap.Incarnation, KeyRef: c.bootstrap.KeyRef, GrantID: id, LaunchID: p.ID, LaunchDigest: launchDigest(p.Launch), Generation: c.generation, Scope: p.Scope, IssuedAt: now, ExpiresAt: now.Add(cmd.Lease)}
		payload, e := encode(claims, c.config.MaxFrameBytes)
		if e != nil {
			return ControlReceipt{}, e
		}
		sig, e := c.signer.Sign(append([]byte("forge-local-grant-v1\n"), payload...))
		if e != nil {
			return ControlReceipt{}, ErrHeld
		}
		g := SignedGrant{Payload: payload, Signature: sig}
		if _, e = EncodeGrant(g, c.config.MaxFrameBytes); e != nil {
			return ControlReceipt{}, e
		}
		if ctx.Err() != nil || !c.liveGate() || !time.Now().Before(began.Add(cmd.Lease)) {
			return ControlReceipt{}, ErrHeld
		}
		c.entries[id] = &registration{claims: claims, grant: g, until: began.Add(cmd.Lease)}
		c.byLaunch[p.ID] = id
		c.version++
	case Revoke:
		if !c.ready || cmd.Launch != nil || cmd.Policy != nil || cmd.Lease != 0 || !hexID(cmd.GrantID) {
			return ControlReceipt{}, ErrConfig
		}
		r, ok := c.entries[cmd.GrantID]
		if !ok {
			return ControlReceipt{}, ErrHeld
		}
		r.revoked = true
		c.version++
	case Advance:
		if !c.ready || cmd.Launch == nil || cmd.Policy == nil || cmd.GrantID != "" || cmd.Lease != 0 || !cmd.Policy.valid() {
			return ControlReceipt{}, ErrConfig
		}
		p, ok := c.policies[cmd.Launch.ID]
		if !ok || (cmd.Launch.Permit.Check(p.Launch) != nil || p.Launch.Match(cmd.Launch.Observation) != nil) {
			return ControlReceipt{}, ErrHeld
		}
		next := *cmd.Policy
		stable := next
		stable.Caller = p.Scope.Caller
		if !same(stable, p.Scope) || next.Caller.ID != p.Scope.Caller.ID || next.Caller.AdmissionID != p.Scope.Caller.AdmissionID {
			return ControlReceipt{}, ErrConfig
		}
		if id := c.byLaunch[p.ID]; id != "" {
			c.entries[id].revoked = true
		}
		p.Scope = next
		c.policies[p.ID] = p
		c.version++
		if e := c.publishVerifierConfigs(c.bootstrap); e != nil {
			c.ready = false
			return ControlReceipt{}, ErrHeld
		}
	case Stop:
		if cmd.Launch != nil || cmd.Policy != nil || cmd.GrantID != "" || cmd.Lease != 0 {
			return ControlReceipt{}, ErrConfig
		}
		c.ready = false
		c.version++
	default:
		return ControlReceipt{}, ErrConfig
	}
	c.lastOperation = controlCommandDigest(cmd)
	return c.receipt(), nil
}

func (c *Controller) grant(ctx context.Context, id string) (SignedGrant, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil || !c.liveGate() || !c.ready {
		return SignedGrant{}, ErrHeld
	}
	r := c.entries[c.byLaunch[id]]
	if r == nil || r.revoked {
		return SignedGrant{}, ErrHeld
	}
	now := c.clock.Now()
	if ctx.Err() != nil || !c.liveGate() || !now.Before(r.claims.ExpiresAt) || !time.Now().Before(r.until) {
		return SignedGrant{}, ErrHeld
	}
	return SignedGrant{Payload: append([]byte(nil), r.grant.Payload...), Signature: append([]byte(nil), r.grant.Signature...)}, nil
}

func (c *Controller) current(ctx context.Context, p CurrentProbe) (SignedProof, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil || !c.liveGate() || !c.ready || p.Schema != Schema || p.Instance != c.bootstrap.Instance || len(p.Challenge) != 32 {
		return SignedProof{}, ErrHeld
	}
	claims, e := decodeClaims(p.Grant, c.config.MaxFrameBytes)
	if e != nil {
		return SignedProof{}, ErrHeld
	}
	if !ed25519.Verify(c.bootstrap.PublicKey, append([]byte("forge-local-grant-v1\n"), p.Grant.Payload...), p.Grant.Signature) {
		return SignedProof{}, ErrHeld
	}
	r := c.entries[claims.GrantID]
	if r == nil || r.revoked || !same(claims, r.claims) || !same(c.policies[claims.LaunchID].Scope, claims.Scope) || claims.Instance != c.bootstrap.Instance || claims.Generation != c.generation {
		return SignedProof{}, ErrHeld
	}
	now := c.clock.Now().UTC()
	remaining := time.Until(r.until)
	if ctx.Err() != nil || remaining <= 0 || !c.liveGate() || !now.Before(claims.ExpiresAt) || now.Add(c.config.ClockSkew).Before(claims.IssuedAt) {
		return SignedProof{}, ErrHeld
	}
	d, e := grantDigest(p.Grant, c.config.MaxFrameBytes)
	if e != nil {
		return SignedProof{}, e
	}
	proof := CurrentProof{Schema: Schema, Instance: c.bootstrap.Instance, GrantDigest: d, Challenge: append([]byte(nil), p.Challenge...), Claims: claims, Version: c.version, Remaining: remaining, CapturedAt: now}
	b, e := encode(proof, c.config.MaxFrameBytes)
	if e != nil {
		return SignedProof{}, e
	}
	sig, e := c.signer.Sign(append([]byte("forge-local-current-v1\n"), b...))
	if e != nil {
		return SignedProof{}, ErrHeld
	}
	return SignedProof{Payload: b, Signature: sig}, nil
}

func (c *Controller) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	owned := c.liveGate()
	c.closed = true
	c.ready = false
	clear(c.signer.Key)
	if c.gate == nil {
		return nil
	}
	if !owned {
		return c.gate.File().Close()
	} // do not remove a replacement owner's record
	return c.gate.Release()
}
