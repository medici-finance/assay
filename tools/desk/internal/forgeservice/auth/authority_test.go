package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
)

type fixtureWall struct {
	mu    sync.Mutex
	value time.Time
}

func (c *fixtureWall) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.value }
func (c *fixtureWall) move(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = c.value.Add(d)
}

type authorityFixture struct {
	controller     *Controller
	config         ProducerConfig
	resources      ProducerResources
	clock          *fixtureWall
	grant          SignedGrant
	verifier       *Verifier
	verifierConfig VerifierConfig
	client         *CurrentClient
	version        uint64
}

func fixtureConfig() (ProducerConfig, ProducerBinding) {
	limits := refresh.Limits{CallerFillsPerWindow: 2, AdmissionFillsPerWindow: 4, FillWindow: time.Minute, MinFreshness: time.Millisecond, MaxFreshness: time.Hour, ActiveFills: 1, QueuedCallers: 2, AdmissionFills: 2, AdmissionWaiters: 4, FailureEntries: 4, WaitTimeout: time.Second, FillTimeout: time.Second, Cooldown: time.Second, FailureTTL: time.Minute, Refresh: mirror.RefreshPolicy{FullInventoryInterval: time.Hour}}
	scope := GrantScope{Scope: mirror.Scope{Repository: mirror.RepositoryID{Provider: mirror.GitHub, Host: "github.com", Path: "fixture/repository"}, Kind: mirror.Issues, AdmissionID: "fixture-admission"}, Caller: refresh.Caller{ID: "fixture-caller", AdmissionID: "fixture-admission", AccessEpoch: "access-a", ConfigEpoch: "config-a", PolicyClass: "read-class", PolicyEpoch: "policy-a", Limits: limits}, Actor: "fixture-actor", Role: "worker", Cell: "fixture-cell", SourceID: "fixture-source", CompositionID: "fixture-composition"}
	launch := celllaunch.AuthorityLaunch{Schema: "cell-authority-launch-v1", LaunchID: "fixture-launch", Cell: scope.Cell, Role: scope.Role, Kind: celllaunch.AuthorityCaller, Container: celllaunch.ContainerIdentity{Endpoint: "unix:///fixture-owned-engine.sock", Engine: "fixture-engine", ID: strings.Repeat("c", 64), Cell: scope.Cell, Role: "caller", PolicyDigest: strings.Repeat("d", 64)}}
	cfg := ProducerConfig{Schema: Schema, Profile: Profile, Issuer: "fixture-issuer", Audience: "fixture-audience", MaxGrantLease: time.Minute, ClockSkew: time.Second, CurrentReadLease: time.Second, MaxRevocationBound: time.Second, IOTimeout: time.Second, MaxFrameBytes: 32 * 1024, MaxGrants: 8, MaxConnections: 4, Launches: []LaunchPolicy{{ID: launch.LaunchID, GrantVolume: "fixture-grant", Scope: scope, Launch: launch}}}
	binding := ProducerBinding{Engine: "fixture-engine", ContainerID: strings.Repeat("a", 64), PlanDigest: strings.Repeat("b", 64), Cell: scope.Cell, PrivateVolume: "fixture-control", CurrentVolume: "fixture-current", BootstrapVolume: "fixture-bootstrap"}
	return cfg, binding
}

func fixtureResources(t *testing.T, b ProducerBinding) ProducerResources {
	t.Helper()
	base, e := os.MkdirTemp("/tmp", "a57-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := os.RemoveAll(base); e != nil {
			t.Error(e)
		}
	})
	open := func(name string) *os.Root {
		p := filepath.Join(base, name)
		if e := os.Mkdir(p, 0700); e != nil {
			t.Fatal(e)
		}
		r, e := os.OpenRoot(p)
		if e != nil {
			t.Fatal(e)
		}
		t.Cleanup(func() { r.Close() })
		return r
	}
	return ProducerResources{Private: open("private"), Current: open("current"), Bootstrap: open("bootstrap"), Grants: map[string]*os.Root{"fixture-launch": open("grant")}, Binding: b}
}

func launchBinding(t *testing.T, p LaunchPolicy) *LaunchBinding {
	t.Helper()
	permit, e := celllaunch.NewAuthorityPermit(p.Launch)
	if e != nil {
		t.Fatal(e)
	}
	return &LaunchBinding{ID: p.ID, Permit: permit, Observation: celllaunch.ContainerObservation{State: celllaunch.Owned, Scope: celllaunch.Scope{Cell: p.Launch.Cell, Role: string(p.Launch.Kind), LaunchID: p.ID, OS: "linux"}, Identity: p.Launch.Container}}
}

// This fixture exercises the real controller and AF_UNIX protocol. Its host
// roots are NOT independent role containment; only the opt-in profile test may
// qualify signer/control custody.
func newAuthorityFixture(t *testing.T, change func(*ProducerConfig)) *authorityFixture {
	t.Helper()
	cfg, b := fixtureConfig()
	if change != nil {
		change(&cfg)
	}
	r := fixtureResources(t, b)
	wall := &fixtureWall{value: time.Now().UTC()}
	c, e := NewController(cfg, r, wall)
	if e != nil {
		t.Fatal("controller", e)
	}
	t.Cleanup(func() { c.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	init, e := c.ApplyControl(ctx, ControlCommand{Operation: Initialize})
	if e != nil {
		t.Fatal("initialize", e)
	}
	registered, e := c.ApplyControl(ctx, ControlCommand{Operation: Register, ExpectedVersion: init.Version, Launch: launchBinding(t, cfg.Launches[0]), Lease: cfg.MaxGrantLease})
	if e != nil {
		t.Fatal("register", e)
	}
	done := make(chan error, 3)
	go func() { done <- c.ServeControl(ctx) }()
	go func() { done <- c.ServeCurrent(ctx, CurrentChannel{}) }()
	go func() { done <- c.ServeGrant(ctx, GrantChannel{LaunchID: cfg.Launches[0].ID}) }()
	t.Cleanup(func() {
		cancel()
		for range 3 {
			select {
			case e := <-done:
				if e != nil {
					t.Error("serve", e)
				}
			case <-time.After(2 * time.Second):
				t.Error("server did not stop")
			}
		}
	})
	for _, p := range []string{filepath.Join(r.Private.Name(), "control.sock"), filepath.Join(r.Current.Name(), "current.sock"), filepath.Join(r.Grants[cfg.Launches[0].ID].Name(), "grant.sock")} {
		deadline := time.Now().Add(2 * time.Second)
		for {
			if _, e := os.Lstat(p); e == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("listener unavailable")
			}
			time.Sleep(time.Millisecond)
		}
	}
	boot, e := LoadBootstrap(r.Bootstrap, cfg.MaxFrameBytes)
	if e != nil {
		t.Fatal("bootstrap", e)
	}
	client, e := NewCurrentClient(CurrentConfig{Path: filepath.Join(r.Current.Name(), "current.sock"), Bootstrap: boot, MaxFrameBytes: cfg.MaxFrameBytes, IOTimeout: cfg.IOTimeout}, wall)
	if e != nil {
		t.Fatal(e)
	}
	vc := VerifierConfig{Schema: Schema, Profile: Profile, Issuer: cfg.Issuer, Audience: cfg.Audience, Bootstrap: boot, Policy: cfg.Launches[0].Scope, Launch: cfg.Launches[0].Launch, MaxGrantLease: cfg.MaxGrantLease, ClockSkew: cfg.ClockSkew, CurrentReadLease: cfg.CurrentReadLease, MaxRevocationBound: cfg.MaxRevocationBound, MaxFrameBytes: cfg.MaxFrameBytes}
	v, e := NewVerifier(vc, client, wall)
	if e != nil {
		t.Fatal("verifier", e)
	}
	g, e := ReadGrant(ctx, filepath.Join(r.Grants[cfg.Launches[0].ID].Name(), "grant.sock"), cfg.MaxFrameBytes, cfg.IOTimeout)
	if e != nil {
		t.Fatal("grant channel", e)
	}
	return &authorityFixture{controller: c, config: cfg, resources: r, clock: wall, grant: g, verifier: v, verifierConfig: vc, client: client, version: registered.Version}
}

func assertHeld(t *testing.T, r VerifiedRead, e error) {
	t.Helper()
	if e == nil || r.owner != nil || r.lease != nil || r.Actor() != "" || r.Role() != "" || r.Caller().ID != "" {
		t.Fatal("refusal released trusted fields")
	}
}

func TestForgeLocalAuthorityClaims(t *testing.T) {
	f := newAuthorityFixture(t, nil)
	ctx := context.Background()
	scope := f.config.Launches[0].Scope.Scope
	for range 2 {
		r, e := f.verifier.Authorize(ctx, f.grant, scope)
		if e != nil {
			t.Fatal("real producer positive", e)
		}
		if r.Actor() != "fixture-actor" || r.Role() != "worker" || !same(r.Caller(), f.config.Launches[0].Scope.Caller) {
			t.Fatal("trusted fields changed")
		}
		if e = f.verifier.CheckRelease(ctx, r); e != nil {
			t.Fatal(e)
		}
		if f.verifier.CheckRelease(ctx, r) == nil {
			t.Fatal("receipt reused")
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*VerifierConfig)
	}{
		{"issuer", func(c *VerifierConfig) { c.Issuer = "other-issuer" }},
		{"audience", func(c *VerifierConfig) { c.Audience = "other-audience" }},
		{"actor", func(c *VerifierConfig) { c.Policy.Actor = "other-actor" }},
		{"role", func(c *VerifierConfig) { c.Policy.Role = "reviewer"; c.Launch.Role = "reviewer" }},
		{"source", func(c *VerifierConfig) { c.Policy.SourceID = "other-source" }},
		{"composition", func(c *VerifierConfig) { c.Policy.CompositionID = "other-composition" }},
		{"access-epoch", func(c *VerifierConfig) { c.Policy.Caller.AccessEpoch = "other-access" }},
		{"config-epoch", func(c *VerifierConfig) { c.Policy.Caller.ConfigEpoch = "other-config" }},
		{"policy-epoch", func(c *VerifierConfig) { c.Policy.Caller.PolicyEpoch = "other-policy" }},
		{"policy-class", func(c *VerifierConfig) { c.Policy.Caller.PolicyClass = "other-class" }},
		{"limits", func(c *VerifierConfig) { c.Policy.Caller.Limits.CallerFillsPerWindow++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := f.verifierConfig
			tc.change(&cfg)
			v, e := NewVerifier(cfg, f.client, f.clock)
			if e != nil {
				t.Fatal("independent valid policy", e)
			}
			r, e := v.Authorize(ctx, f.grant, scope)
			assertHeld(t, r, e)
		})
	}
	t.Run("constructor-cell", func(t *testing.T) {
		cfg := f.verifierConfig
		cfg.Policy.Cell = "foreign-cell"
		if v, e := NewVerifier(cfg, f.client, f.clock); v != nil || !errors.Is(e, ErrConfig) {
			t.Fatal("foreign cell configuration accepted")
		}
	})
	t.Run("verification-key", func(t *testing.T) {
		cfg := f.verifierConfig
		cfg.Bootstrap.PublicKey = bytes.Repeat([]byte{2}, 32)
		client, e := NewCurrentClient(CurrentConfig{Path: f.client.config.Path, Bootstrap: cfg.Bootstrap, MaxFrameBytes: cfg.MaxFrameBytes, IOTimeout: f.config.IOTimeout}, f.clock)
		if e != nil {
			t.Fatal(e)
		}
		v, e := NewVerifier(cfg, client, f.clock)
		if e != nil {
			t.Fatal(e)
		}
		r, e := v.Authorize(ctx, f.grant, scope)
		assertHeld(t, r, e)
	})
	for _, kind := range []string{"repository", "kind", "admission"} {
		t.Run("query-"+kind, func(t *testing.T) {
			q := scope
			switch kind {
			case "repository":
				q.Repository.Path = "fixture/other"
			case "kind":
				q.Kind = mirror.Changes
			case "admission":
				q.AdmissionID = "other-admission"
			}
			r, e := f.verifier.Authorize(ctx, f.grant, q)
			assertHeld(t, r, e)
		})
	}
	bad := SignedGrant{Payload: append([]byte(nil), f.grant.Payload...), Signature: append([]byte(nil), f.grant.Signature...)}
	bad.Signature[0] ^= 1
	r, e := f.verifier.Authorize(ctx, bad, scope)
	assertHeld(t, r, e)
	a, e := f.verifier.BindRefresh(f.grant)
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Authorize(ctx, f.config.Launches[0].Scope.Caller, scope); e != nil {
		t.Fatal(e)
	}
	caller := f.config.Launches[0].Scope.Caller
	caller.ID = "other-caller"
	if a.Authorize(ctx, caller, scope) == nil {
		t.Fatal("refresh caller selected identity")
	}
}

func TestForgeLocalAuthorityCurrent(t *testing.T) {
	t.Run("forged-proof-after-revoke", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		ctx := context.Background()
		claims, e := decodeClaims(f.grant, f.config.MaxFrameBytes)
		if e != nil {
			t.Fatal(e)
		}
		boot := f.verifierConfig.Bootstrap
		if !ed25519.Verify(boot.PublicKey, append([]byte("forge-local-grant-v1\n"), f.grant.Payload...), f.grant.Signature) {
			t.Fatal("fixture grant signature is not valid")
		}
		revoked, e := f.controller.ApplyControl(ctx, ControlCommand{Operation: Revoke, ExpectedVersion: f.version, GrantID: claims.GrantID})
		if e != nil {
			t.Fatal(e)
		}
		r, e := f.verifier.Authorize(ctx, f.grant, claims.Scope.Scope)
		assertHeld(t, r, e)
		_, otherKey, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		answered := make(chan bool, 1)
		v := currentFixtureProxy(t, f, func(q CurrentProbe) (SignedProof, error) {
			if !same(q.Grant, f.grant) || q.Instance != boot.Instance || len(q.Challenge) != 32 {
				t.Error("forged proof request did not bind original grant/challenge")
			}
			d, e := grantDigest(q.Grant, f.config.MaxFrameBytes)
			if e != nil {
				return SignedProof{}, e
			}
			payload, e := encode(CurrentProof{Schema: Schema, Instance: boot.Instance, GrantDigest: d, Challenge: q.Challenge, Claims: claims, Version: revoked.Version, Remaining: f.config.MaxGrantLease, CapturedAt: f.clock.Now()}, f.config.MaxFrameBytes)
			if e != nil {
				return SignedProof{}, e
			}
			sig := ed25519.Sign(otherKey, append([]byte("forge-local-current-v1\n"), payload...))
			answered <- !ed25519.Verify(boot.PublicKey, append([]byte("forge-local-current-v1\n"), payload...), sig)
			return SignedProof{Payload: payload, Signature: sig}, nil
		})
		r, e = v.Authorize(ctx, f.grant, claims.Scope.Scope)
		select {
		case invalid := <-answered:
			if !invalid {
				t.Fatal("forged proof unexpectedly signed by trusted key")
			}
		default:
			t.Fatal("forged current channel was not reached")
		}
		assertHeld(t, r, e)
	})
	t.Run("challenge-replay", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		original := bytes.Repeat([]byte{1}, 32)
		proof, e := f.client.probe(context.Background(), CurrentProbe{Schema: Schema, Instance: f.verifierConfig.Bootstrap.Instance, Challenge: original, Grant: f.grant})
		if e != nil {
			t.Fatal("actual signed current proof", e)
		}
		claims, e := decodeClaims(f.grant, f.config.MaxFrameBytes)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.controller.ApplyControl(context.Background(), ControlCommand{Operation: Revoke, ExpectedVersion: f.version, GrantID: claims.GrantID}); e != nil {
			t.Fatal(e)
		}
		v := currentFixtureProxy(t, f, func(q CurrentProbe) (SignedProof, error) {
			if bytes.Equal(q.Challenge, original) {
				t.Error("new request reused original challenge")
			}
			return proof, nil
		})
		r, e := v.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		assertHeld(t, r, e)
	})
	t.Run("delayed-current", func(t *testing.T) {
		f := newAuthorityFixture(t, func(c *ProducerConfig) {
			c.CurrentReadLease = 100 * time.Millisecond
			c.MaxRevocationBound = c.CurrentReadLease
		})
		v := currentFixtureProxy(t, f, func(q CurrentProbe) (SignedProof, error) {
			proof, e := f.controller.current(context.Background(), q)
			if e != nil {
				return proof, e
			}
			f.clock.move(-time.Second / 2)
			time.Sleep(150 * time.Millisecond)
			return proof, nil
		})
		r, e := v.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		assertHeld(t, r, e)
	})
	t.Run("during-current-expiry", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		v := currentFixtureProxy(t, f, func(q CurrentProbe) (SignedProof, error) {
			p, e := f.controller.current(context.Background(), q)
			f.clock.move(2 * time.Minute)
			return p, e
		})
		r, e := v.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		assertHeld(t, r, e)
	})
	t.Run("wall-release", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		r, e := f.verifier.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		if e != nil {
			t.Fatal(e)
		}
		f.clock.move(2 * time.Minute)
		if f.verifier.CheckRelease(context.Background(), r) == nil {
			t.Fatal("expired wall receipt released")
		}
	})
	t.Run("producer-monotonic-expiry", func(t *testing.T) {
		f := newAuthorityFixture(t, func(c *ProducerConfig) {
			c.MaxGrantLease = 500 * time.Millisecond
			c.ClockSkew = 100 * time.Millisecond
			c.CurrentReadLease = 100 * time.Millisecond
			c.MaxRevocationBound = c.CurrentReadLease
		})
		// The fixture wall remains frozen before ExpiresAt. Only the actual
		// producer's private elapsed deadline can expire this registration.
		time.Sleep(600 * time.Millisecond)
		r, e := f.verifier.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		assertHeld(t, r, e)
	})
	t.Run("unavailable-current", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		if e := f.controller.Close(); e != nil {
			t.Fatal(e)
		}
		r, e := f.verifier.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		assertHeld(t, r, e)
	})
	t.Run("revoke", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		claims, e := decodeClaims(f.grant, f.config.MaxFrameBytes)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.controller.ApplyControl(context.Background(), ControlCommand{Operation: Revoke, ExpectedVersion: f.version, GrantID: claims.GrantID}); e != nil {
			t.Fatal(e)
		}
		r, e := f.verifier.Authorize(context.Background(), f.grant, claims.Scope.Scope)
		assertHeld(t, r, e)
	})
	t.Run("epoch", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		next := f.config.Launches[0].Scope
		next.Caller.AccessEpoch = "access-b"
		if _, e := f.controller.ApplyControl(context.Background(), ControlCommand{Operation: Advance, ExpectedVersion: f.version, Launch: launchBinding(t, f.config.Launches[0]), Policy: &next}); e != nil {
			t.Fatal(e)
		}
		r, e := f.verifier.Authorize(context.Background(), f.grant, next.Scope)
		assertHeld(t, r, e)
	})
	t.Run("rotate", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		if _, e := f.controller.ApplyControl(context.Background(), ControlCommand{Operation: Rotate, ExpectedVersion: f.version}); e != nil {
			t.Fatal(e)
		}
		r, e := f.verifier.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		assertHeld(t, r, e)
	})
	t.Run("wall-expiry", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		f.clock.move(2 * time.Minute)
		r, e := f.verifier.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		assertHeld(t, r, e)
	})
	t.Run("cancel-release", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		ctx, cancel := context.WithCancel(context.Background())
		r, e := f.verifier.Authorize(ctx, f.grant, f.config.Launches[0].Scope.Scope)
		if e != nil {
			t.Fatal(e)
		}
		cancel()
		if f.verifier.CheckRelease(context.Background(), r) == nil {
			t.Fatal("new context revived canceled receipt")
		}
	})
	t.Run("monotonic-release", func(t *testing.T) {
		f := newAuthorityFixture(t, func(c *ProducerConfig) {
			c.CurrentReadLease = 100 * time.Millisecond
			c.MaxRevocationBound = c.CurrentReadLease
		})
		r, e := f.verifier.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		if e != nil {
			t.Fatal(e)
		}
		f.clock.move(-time.Second / 2)
		time.Sleep(150 * time.Millisecond)
		if f.verifier.CheckRelease(context.Background(), r) == nil {
			t.Fatal("wall rollback renewed local release lease")
		}
	})
	t.Run("gate-replacement", func(t *testing.T) {
		f := newAuthorityFixture(t, nil)
		if e := f.resources.Private.Rename("live.lock", "old.lock"); e != nil {
			t.Fatal(e)
		}
		fd, e := f.resources.Private.OpenFile("live.lock", os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
		if e != nil {
			t.Fatal(e)
		}
		fd.Close()
		r, e := f.verifier.Authorize(context.Background(), f.grant, f.config.Launches[0].Scope.Scope)
		assertHeld(t, r, e)
	})
}

// The proxy replays, delays or forges a negative proof. It is an adversarial
// fault-injection transport, never a substitute source/signing authority.
func currentFixtureProxy(t *testing.T, f *authorityFixture, reply func(CurrentProbe) (SignedProof, error)) *Verifier {
	t.Helper()
	path := filepath.Join(f.resources.Current.Name(), "fault.sock")
	l, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		t.Fatal(e)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, e := l.AcceptUnix()
		if e != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		var q wireRequest
		if readFrame(conn, f.config.MaxFrameBytes, &q) != nil || q.Probe == nil {
			return
		}
		p, e := reply(*q.Probe)
		if e != nil {
			return
		}
		b, e := encode(wireResponse{Schema: Schema, Proof: &p}, f.config.MaxFrameBytes)
		if e != nil {
			return
		}
		conn.Write(b)
	}()
	t.Cleanup(func() {
		l.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("fault transport did not stop")
		}
	})
	client, e := NewCurrentClient(CurrentConfig{Path: path, Bootstrap: f.verifierConfig.Bootstrap, MaxFrameBytes: f.config.MaxFrameBytes, IOTimeout: time.Second}, f.clock)
	if e != nil {
		t.Fatal(e)
	}
	v, e := NewVerifier(f.verifierConfig, client, f.clock)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func TestForgeLocalAuthorityLaunch(t *testing.T) {
	cfg, b := fixtureConfig()
	r := fixtureResources(t, b)
	wall := &fixtureWall{value: time.Now().UTC()}
	for _, name := range []string{"policy", "issuer", "profile", "lease", "frame"} {
		t.Run(name, func(t *testing.T) {
			copy := cfg
			copy.Launches = append([]LaunchPolicy(nil), cfg.Launches...)
			switch name {
			case "policy":
				copy.Launches[0].Scope.Caller.Limits.Refresh.FailureBackoff = time.Second
			case "issuer":
				copy.Issuer = ""
			case "profile":
				copy.Profile = "unknown"
			case "lease":
				copy.CurrentReadLease = 0
			case "frame":
				copy.MaxFrameBytes = 1
			}
			c, e := NewController(copy, r, wall)
			if c != nil {
				c.Close()
			}
			if !errors.Is(e, ErrConfig) {
				t.Fatal("malformed configuration accepted", e)
			}
			if _, e = r.Private.Lstat("live.lock"); !os.IsNotExist(e) {
				t.Fatal("configuration touched operator gate")
			}
			if _, e = r.Current.Lstat("current.sock"); !os.IsNotExist(e) {
				t.Fatal("configuration opened listener")
			}
		})
	}
	f := newAuthorityFixture(t, nil)
	claims, e := decodeClaims(f.grant, f.config.MaxFrameBytes)
	if e != nil {
		t.Fatal(e)
	}
	revoked, e := f.controller.ApplyControl(context.Background(), ControlCommand{Operation: Revoke, ExpectedVersion: f.version, GrantID: claims.GrantID})
	if e != nil {
		t.Fatal(e)
	}
	f.version = revoked.Version
	binding := launchBinding(t, f.config.Launches[0])
	binding.Observation.Identity.ID = strings.Repeat("f", 64)
	if _, e := f.controller.ApplyControl(context.Background(), ControlCommand{Operation: Register, ExpectedVersion: f.version, Launch: binding, Lease: time.Second}); e == nil {
		t.Fatal("foreign resource observation accepted")
	}
	for _, state := range []celllaunch.ObservationState{celllaunch.Missing, celllaunch.Foreign, celllaunch.Unknown} {
		binding := launchBinding(t, f.config.Launches[0])
		binding.Observation.State = state
		if _, e := f.controller.ApplyControl(context.Background(), ControlCommand{Operation: Register, ExpectedVersion: f.version, Launch: binding, Lease: time.Second}); e == nil {
			t.Fatal("non-owned observation accepted")
		}
	}
	if _, e := f.controller.ApplyControl(context.Background(), ControlCommand{Operation: Rotate, ExpectedVersion: f.version - 1}); !errors.Is(e, ErrConflict) {
		t.Fatal("control version not fenced")
	}
}

func TestForgeLocalAuthorityClosedCodec(t *testing.T) {
	f := newAuthorityFixture(t, nil)
	b, e := EncodeGrant(f.grant, f.config.MaxFrameBytes)
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range [][]byte{append(append([]byte(nil), b...), ' '), []byte(`{"Payload":null,"Signature":null}`), []byte(`{"Payload":"","Payload":"","Signature":""}`)} {
		if _, e := DecodeGrant(bad, f.config.MaxFrameBytes); e == nil {
			t.Fatal("noncanonical envelope accepted")
		}
	}
	var c GrantClaims
	if json.Unmarshal(f.grant.Payload, &c) != nil {
		t.Fatal("fixture claims")
	}
	c.Operation = "effect-create"
	payload, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := decodeClaims(SignedGrant{Payload: payload, Signature: f.grant.Signature}, f.config.MaxFrameBytes); e == nil {
		t.Fatal("effect operation accepted")
	}
}

func TestLocalAuthorityPrivateControlProtocol(t *testing.T) {
	f := newAuthorityFixture(t, nil)
	ctx := context.Background()
	path := filepath.Join(f.resources.Private.Name(), "control.sock")
	state, e := RequestControl(ctx, path, f.config.MaxFrameBytes, f.config.IOTimeout, nil)
	if e != nil || !state.Ready || state.Version != f.version {
		t.Fatal("private readback", e)
	}
	currentPath := filepath.Join(f.resources.Current.Name(), "current.sock")
	if _, e = RequestControl(ctx, currentPath, f.config.MaxFrameBytes, f.config.IOTimeout, nil); e == nil {
		t.Fatal("current channel exposed control")
	}
	grantPath := filepath.Join(f.resources.Grants[f.config.Launches[0].ID].Name(), "grant.sock")
	if _, e = RequestControl(ctx, grantPath, f.config.MaxFrameBytes, f.config.IOTimeout, nil); e == nil {
		t.Fatal("grant channel exposed control")
	}
	after, e := RequestControl(ctx, path, f.config.MaxFrameBytes, f.config.IOTimeout, nil)
	if e != nil || !same(state, after) {
		t.Fatal("data channel modified authority")
	}
	if _, e = RequestControl(ctx, path, f.config.MaxFrameBytes, f.config.IOTimeout, &OperatorCommand{Operation: "sign", ExpectedVersion: state.Version}); e == nil {
		t.Fatal("generic sign operation exposed")
	}
	claims, e := decodeClaims(f.grant, f.config.MaxFrameBytes)
	if e != nil {
		t.Fatal(e)
	}
	result, e := RequestControl(ctx, path, f.config.MaxFrameBytes, f.config.IOTimeout, &OperatorCommand{Operation: Revoke, ExpectedVersion: state.Version, GrantID: claims.GrantID})
	if e != nil || result.Version != state.Version+1 {
		t.Fatal("operator revoke", e)
	}
	r, e := f.verifier.Authorize(ctx, f.grant, claims.Scope.Scope)
	assertHeld(t, r, e)
}

// regression: F-local-authority-registration
func TestForgeLocalAuthorityRegistrationBudget(t *testing.T) {
	for _, expired := range []bool{false, true} {
		name := "live"
		if expired {
			name = "expired"
		}
		t.Run(name, func(t *testing.T) {
			f := newAuthorityFixture(t, nil)
			if expired {
				f.clock.move(2 * f.config.MaxGrantLease)
			}
			ctx := context.Background()
			cmd := ControlCommand{Operation: Register, ExpectedVersion: f.version, Launch: launchBinding(t, f.config.Launches[0]), Lease: f.config.MaxGrantLease}
			if _, e := f.controller.ApplyControl(ctx, cmd); e == nil || e.Error() != "local authority: registration requires explicit revoke" {
				t.Fatalf("existing registration refusal class: %v", e)
			}
			state, e := f.controller.ControlState(ctx)
			if e != nil || state.Version != f.version {
				t.Fatal("refusal changed control version")
			}
			claims, e := decodeClaims(f.grant, f.config.MaxFrameBytes)
			if e != nil {
				t.Fatal(e)
			}
			r, e := f.controller.ApplyControl(ctx, ControlCommand{Operation: Revoke, ExpectedVersion: state.Version, GrantID: claims.GrantID})
			if e != nil {
				t.Fatal(e)
			}
			cmd.ExpectedVersion = r.Version
			if _, e = f.controller.ApplyControl(ctx, cmd); e != nil {
				t.Fatal("explicit revoke did not permit registration", e)
			}
			if expired {
				f.clock.move(-2 * f.config.MaxGrantLease)
			}
			got, e := f.verifier.Authorize(ctx, f.grant, claims.Scope.Scope)
			assertHeld(t, got, e)
		})
	}
	t.Run("lifetime-capacity", func(t *testing.T) {
		f := newAuthorityFixture(t, func(c *ProducerConfig) { c.MaxGrants = 2 })
		ctx := context.Background()
		old := f.grant
		version := f.version
		for range 2 {
			claims, e := decodeClaims(old, f.config.MaxFrameBytes)
			if e != nil {
				t.Fatal(e)
			}
			r, e := f.controller.ApplyControl(ctx, ControlCommand{Operation: Revoke, ExpectedVersion: version, GrantID: claims.GrantID})
			if e != nil {
				t.Fatal(e)
			}
			version = r.Version
			if len(f.controller.entries) == 2 {
				break
			}
			r, e = f.controller.ApplyControl(ctx, ControlCommand{Operation: Register, ExpectedVersion: version, Launch: launchBinding(t, f.config.Launches[0]), Lease: f.config.MaxGrantLease})
			if e != nil {
				t.Fatal(e)
			}
			version = r.Version
			old, e = f.controller.grant(ctx, f.config.Launches[0].ID)
			if e != nil {
				t.Fatal(e)
			}
		}
		f.clock.move(2 * f.config.MaxGrantLease)
		cmd := ControlCommand{Operation: Register, ExpectedVersion: version, Launch: launchBinding(t, f.config.Launches[0]), Lease: f.config.MaxGrantLease}
		if _, e := f.controller.ApplyControl(ctx, cmd); e == nil || e.Error() != "local authority: registration capacity exhausted" {
			t.Fatalf("capacity refusal class: %v", e)
		}
		if len(f.controller.entries) != 2 || f.controller.version != version {
			t.Fatal("capacity refusal evicted history or changed version")
		}
		r, e := f.controller.ApplyControl(ctx, ControlCommand{Operation: Rotate, ExpectedVersion: version})
		if e != nil {
			t.Fatal(e)
		}
		if len(f.controller.entries) != 0 || f.controller.bootstrap.Instance == f.verifierConfig.Bootstrap.Instance || bytes.Equal(f.controller.bootstrap.PublicKey, f.verifierConfig.Bootstrap.PublicKey) {
			t.Fatal("rotation did not fence history with new instance/key")
		}
		cmd.ExpectedVersion = r.Version
		if _, e = f.controller.ApplyControl(ctx, cmd); e != nil {
			t.Fatal("rotation did not restore capacity", e)
		}
		f.clock.move(-2 * f.config.MaxGrantLease)
		got, e := f.verifier.Authorize(ctx, f.grant, f.config.Launches[0].Scope.Scope)
		assertHeld(t, got, e)
	})
}
