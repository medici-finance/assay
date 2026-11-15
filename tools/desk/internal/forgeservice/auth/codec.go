package auth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/medici-finance/assay/tools/desk/internal/celllaunch"
)

// canonicalDecode is the one closed protocol decoder. Re-encoding the fixed
// typed value enforces required members and rejects duplicates, aliases and
// alternative encodings as well as unknown fields. It does not classify policy.
func canonicalDecode(b []byte, n int, dst any) error {
	if len(b) == 0 || len(b) > n || !utf8.Valid(b) {
		return ErrHeld
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(dst) != nil || d.Decode(new(any)) != io.EOF {
		return ErrHeld
	}
	want, err := json.Marshal(dst)
	if err != nil || !bytes.Equal(want, b) {
		return ErrHeld
	}
	return nil
}

func encode(v any, max int) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 || len(b) > max {
		return nil, ErrHeld
	}
	return b, nil
}

func digest(b []byte) string { x := sha256.Sum256(b); return hex.EncodeToString(x[:]) }
func grantDigest(g SignedGrant, max int) (string, error) {
	b, err := encode(g, max)
	if err != nil {
		return "", err
	}
	return digest(b), nil
}

// EncodeGrant and DecodeGrant carry only the closed authentication envelope.
// Decoding grants no trust; Verifier must check it against the protected bootstrap.
func EncodeGrant(g SignedGrant, max int) ([]byte, error) {
	if max <= 0 || max > 128*1024 || len(g.Payload) == 0 || len(g.Payload) > max || len(g.Signature) != ed25519.SignatureSize {
		return nil, ErrHeld
	}
	return encode(g, max)
}
func DecodeGrant(b []byte, max int) (SignedGrant, error) {
	var g SignedGrant
	if max <= 0 || max > 128*1024 || canonicalDecode(b, max, &g) != nil || len(g.Payload) == 0 || len(g.Signature) != ed25519.SignatureSize {
		return SignedGrant{}, ErrHeld
	}
	g.Payload = bytes.Clone(g.Payload)
	g.Signature = bytes.Clone(g.Signature)
	return g, nil
}

func decodeClaims(g SignedGrant, max int) (GrantClaims, error) {
	var c GrantClaims
	if len(g.Signature) != ed25519.SignatureSize || canonicalDecode(g.Payload, max, &c) != nil || c.Schema != Schema || c.Operation != Operation || !text(c.Issuer) || !text(c.Audience) || !c.Binding.valid() || !hexID(c.Instance) || !hexID(c.Incarnation) || !hexID(c.KeyRef) || !hexID(c.GrantID) || !name(c.LaunchID) || !hexID(c.LaunchDigest) || c.Generation == 0 || !c.Scope.valid() || c.Scope.Cell != c.Binding.Cell || c.IssuedAt.IsZero() || c.ExpiresAt.IsZero() || !c.ExpiresAt.After(c.IssuedAt) {
		return GrantClaims{}, ErrHeld
	}
	return c, nil
}

type wireRequest struct {
	Schema, Operation string
	Probe             *CurrentProbe
	Control           *OperatorCommand
	Readback          bool
}
type wireResponse struct {
	Schema, Error string
	Grant         *SignedGrant
	Proof         *SignedProof
	Receipt       *ControlReceipt
}

func readFrame(r io.Reader, max int, dst any) error {
	b, err := io.ReadAll(io.LimitReader(r, int64(max)+1))
	if err != nil {
		return ErrHeld
	}
	return canonicalDecode(b, max, dst)
}

func launchDigest(s celllaunch.AuthorityLaunch) string { b, _ := json.Marshal(s); return digest(b) }

// checkFrames validates complete closed envelope shapes without creating a key
// or opening resources. These placeholders are never signed or authorized.
func checkFrames(cfg ProducerConfig, binding ProducerBinding, policies map[string]LaunchPolicy) error {
	id := strings.Repeat("0", 64)
	boot := Bootstrap{Schema: Schema, Binding: binding, Instance: id, Incarnation: id, KeyRef: id, PublicKey: make([]byte, ed25519.PublicKeySize)}
	prototype := Controller{config: cfg, policies: policies}
	if _, e := encode(prototype.verifierConfigs(boot), cfg.MaxFrameBytes); e != nil {
		return ErrConfig
	}
	stamp := time.Date(9000, 1, 1, 1, 1, 1, 999999999, time.UTC)
	for _, p := range policies {
		claims := GrantClaims{Schema: Schema, Operation: Operation, Issuer: cfg.Issuer, Audience: cfg.Audience, Binding: binding, Instance: id, Incarnation: id, KeyRef: id, GrantID: id, LaunchID: p.ID, LaunchDigest: id, Generation: math.MaxUint64, Scope: p.Scope, IssuedAt: stamp, ExpiresAt: stamp.Add(cfg.MaxGrantLease)}
		payload, e := encode(claims, cfg.MaxFrameBytes)
		if e != nil {
			return ErrConfig
		}
		g := SignedGrant{Payload: payload, Signature: make([]byte, ed25519.SignatureSize)}
		proof, e := encode(CurrentProof{Schema: Schema, Instance: id, GrantDigest: id, Challenge: make([]byte, 32), Claims: claims, Version: math.MaxUint64, Remaining: cfg.MaxGrantLease, CapturedAt: stamp}, cfg.MaxFrameBytes)
		if e != nil {
			return ErrConfig
		}
		signed := SignedProof{Payload: proof, Signature: make([]byte, ed25519.SignatureSize)}
		for _, value := range []any{wireRequest{Schema: Schema, Operation: "current", Probe: &CurrentProbe{Schema: Schema, Instance: id, Challenge: make([]byte, 32), Grant: g}}, wireResponse{Schema: Schema, Grant: &g}, wireResponse{Schema: Schema, Proof: &signed}} {
			if _, e := encode(value, cfg.MaxFrameBytes); e != nil {
				return ErrConfig
			}
		}
	}
	return nil
}
