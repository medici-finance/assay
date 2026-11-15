package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/forgeread/mirror"
	"github.com/medici-finance/assay/forgeread/refresh"
	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
)

// LoadBootstrap reads only the explicit protected public bootstrap root. The
// caller must independently establish its qualified read-only mount/binding.
func LoadBootstrap(root *os.Root, max int) (Bootstrap, error) {
	var b Bootstrap
	if root == nil || max <= 0 || max > celllaunch.MaxRecordBytes {
		return b, ErrConfig
	}
	st, e := root.Lstat("bootstrap.json")
	if e != nil || !st.Mode().IsRegular() {
		return b, ErrHeld
	}
	f, e := root.Open("bootstrap.json")
	if e != nil {
		return b, ErrHeld
	}
	defer f.Close()
	opened, e := f.Stat()
	if e != nil || !os.SameFile(st, opened) {
		return b, ErrHeld
	}
	if readFrame(f, max, &b) != nil || !b.valid() {
		return Bootstrap{}, ErrHeld
	}
	return b, nil
}

type CurrentClient struct {
	config CurrentConfig
	clock  refresh.Clock
}

func NewCurrentClient(cfg CurrentConfig, wall refresh.Clock) (*CurrentClient, error) {
	if !clockPresent(wall) || !filepath.IsAbs(cfg.Path) || filepath.Clean(cfg.Path) != cfg.Path || !text(cfg.Path) || !cfg.Bootstrap.valid() || cfg.MaxFrameBytes <= 0 || cfg.MaxFrameBytes > celllaunch.MaxRecordBytes || cfg.IOTimeout <= 0 {
		return nil, ErrConfig
	}
	cfg.Bootstrap.PublicKey = bytes.Clone(cfg.Bootstrap.PublicKey)
	return &CurrentClient{config: cfg, clock: wall}, nil
}

type Verifier struct {
	config  VerifierConfig
	current *CurrentClient
	clock   refresh.Clock
}

func NewVerifier(cfg VerifierConfig, current *CurrentClient, wall refresh.Clock) (*Verifier, error) {
	if current == nil || !clockPresent(wall) || cfg.Schema != Schema || cfg.Profile != Profile || !text(cfg.Issuer) || !text(cfg.Audience) || !cfg.Bootstrap.valid() || !cfg.Policy.valid() || cfg.Policy.Cell != cfg.Bootstrap.Binding.Cell || cfg.MaxGrantLease <= 0 || cfg.ClockSkew < 0 || cfg.ClockSkew > cfg.MaxGrantLease || cfg.Launch.Validate() != nil || cfg.Launch.Kind != celllaunch.AuthorityCaller || cfg.Launch.Cell != cfg.Policy.Cell || cfg.Launch.Role != cfg.Policy.Role || cfg.Launch.Container.Engine != cfg.Bootstrap.Binding.Engine || cfg.Launch.Container.ID == cfg.Bootstrap.Binding.ContainerID || cfg.CurrentReadLease <= 0 || cfg.MaxRevocationBound < cfg.CurrentReadLease || cfg.MaxFrameBytes <= 0 || cfg.MaxFrameBytes > celllaunch.MaxRecordBytes || !same(cfg.Bootstrap, current.config.Bootstrap) || cfg.MaxFrameBytes != current.config.MaxFrameBytes {
		return nil, ErrConfig
	}
	cfg.Bootstrap.PublicKey = bytes.Clone(cfg.Bootstrap.PublicKey)
	return &Verifier{config: cfg, current: current, clock: wall}, nil
}

// semantic: S-forge-local-authority
func (v *Verifier) Authorize(ctx context.Context, g SignedGrant, scope mirror.Scope) (VerifiedRead, error) {
	var zero VerifiedRead
	if v == nil || ctx == nil || ctx.Err() != nil || scope.Validate() != nil {
		return zero, ErrHeld
	}
	// This actual stdlib time retains its monotonic component for this request.
	began := time.Now()
	wallStart := v.clock.Now()
	claims, e := decodeClaims(g, v.config.MaxFrameBytes)
	if e != nil {
		return zero, ErrHeld
	}
	b := v.config.Bootstrap
	if !ed25519.Verify(b.PublicKey, append([]byte("forge-local-grant-v1\n"), g.Payload...), g.Signature) || claims.Issuer != v.config.Issuer || claims.Audience != v.config.Audience || !same(claims.Binding, b.Binding) || claims.Instance != b.Instance || claims.Incarnation != b.Incarnation || claims.KeyRef != b.KeyRef || claims.LaunchID != v.config.Launch.LaunchID || claims.LaunchDigest != launchDigest(v.config.Launch) || claims.ExpiresAt.Sub(claims.IssuedAt) > v.config.MaxGrantLease || !same(claims.Scope, v.config.Policy) || claims.Scope.Scope != scope || wallStart.IsZero() || !wallStart.Before(claims.ExpiresAt) || wallStart.Add(v.config.ClockSkew).Before(claims.IssuedAt) {
		return zero, ErrHeld
	}
	challenge := make([]byte, 32)
	if _, e = rand.Read(challenge); e != nil {
		return zero, ErrHeld
	}
	proof, e := v.current.probe(ctx, CurrentProbe{Schema: Schema, Instance: b.Instance, Challenge: challenge, Grant: g})
	if e != nil {
		return zero, ErrHeld
	}
	if !ed25519.Verify(b.PublicKey, append([]byte("forge-local-current-v1\n"), proof.Payload...), proof.Signature) {
		return zero, ErrHeld
	}
	var p CurrentProof
	if canonicalDecode(proof.Payload, v.config.MaxFrameBytes, &p) != nil {
		return zero, ErrHeld
	}
	d, e := grantDigest(g, v.config.MaxFrameBytes)
	if e != nil {
		return zero, ErrHeld
	}
	if p.Schema != Schema || p.Instance != b.Instance || p.GrantDigest != d || !bytes.Equal(p.Challenge, challenge) || !same(p.Claims, claims) || p.Version == 0 || p.Remaining <= 0 || p.Remaining > v.config.MaxGrantLease || p.CapturedAt.IsZero() {
		return zero, ErrHeld
	}
	// Anchor every bound at REQUEST START, never proof arrival. The producer's
	// signed remaining monotonic window may only shorten the wall/lease window.
	until := began.Add(v.config.CurrentReadLease)
	for _, candidate := range []time.Time{began.Add(claims.ExpiresAt.Sub(wallStart)), began.Add(p.Remaining)} {
		if candidate.Before(until) {
			until = candidate
		}
	}
	if deadline, ok := ctx.Deadline(); ok && deadline.Before(until) {
		until = deadline
	}
	now := v.clock.Now()
	if ctx.Err() != nil || !now.Before(claims.ExpiresAt) || now.Add(v.config.ClockSkew).Before(p.CapturedAt) || !time.Now().Before(until) {
		return zero, ErrHeld
	}
	return VerifiedRead{owner: v, policy: claims.Scope, lease: &readLease{original: ctx, until: until, expires: claims.ExpiresAt}}, nil
}

// CheckRelease is the final local check after encoding/callbacks/other I/O.
// A receipt permits exactly one release and cannot renew itself or fetch state.
func (v *Verifier) CheckRelease(ctx context.Context, r VerifiedRead) error {
	if v == nil || r.owner != v || r.lease == nil || ctx == nil || r.lease.original == nil {
		return ErrHeld
	}
	now := v.clock.Now()
	if ctx.Err() != nil || r.lease.original.Err() != nil || now.IsZero() || !now.Before(r.lease.expires) || !time.Now().Before(r.lease.until) {
		return ErrHeld
	}
	if !r.lease.used.CompareAndSwap(false, true) {
		return ErrHeld
	}
	return nil
}

type refreshAuthority struct {
	verifier *Verifier
	grant    SignedGrant
}

func (v *Verifier) BindRefresh(g SignedGrant) (refresh.CallerAuthority, error) {
	if v == nil {
		return nil, ErrConfig
	}
	// Detach bytes before any request can mutate its authentication envelope.
	b, e := EncodeGrant(g, v.config.MaxFrameBytes)
	if e != nil {
		return nil, ErrHeld
	}
	owned, e := DecodeGrant(b, v.config.MaxFrameBytes)
	if e != nil {
		return nil, ErrHeld
	}
	return &refreshAuthority{verifier: v, grant: owned}, nil
}
func (a *refreshAuthority) Authorize(ctx context.Context, caller refresh.Caller, scope mirror.Scope) error {
	r, e := a.verifier.Authorize(ctx, a.grant, scope)
	if e != nil {
		return ErrHeld
	}
	if !same(caller, r.policy.Caller) {
		return ErrHeld
	}
	return a.verifier.CheckRelease(ctx, r)
}
