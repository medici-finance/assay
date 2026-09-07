package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deps.go — the gateway's ONE construction site for PreCheckDeps. This is the
// site Verify rows 8/8a pin: a replayed assertion must be refused through THIS
// function's output, and no construction anywhere in cmd/commsgw may pass a nil
// ReplayGuard (identity.go: "a nil replay guard means an assertion can be
// replayed... a real gateway always supplies one"). Both socket.go's within-cell
// listener and a2a.go's cross-cell listener build their PreCheckDeps by calling
// this function — never by hand-assembling the struct a second time — so there
// is exactly one place a nil guard could ever slip in, and this file is it.

// defaultRateLimit / defaultRateWindow bound the per-sender pre-check rate
// stage when a caller does not override it. Generous for real desk traffic
// (desks message each other in bursts of a handful, not hundreds/minute), far
// below anything that would strain the gateway.
const (
	defaultRateLimit  = 120
	defaultRateWindow = time.Minute
)

// REPLAY WINDOW / CLOCK SKEW (#1951, owned HERE by this gateway, not by the
// earlier envelope/identity work). This gateway's assertion-verification
// stage (PreCheck -> comms.VerifyEnvelope -> comms.Verify) runs against TWO
// values that started as unexplained literals in internal/comms/identity.go
// and are documented at this, the gateway's own config surface, per #1951:
//
//   - comms.DefaultTTL = 2m — the signed assertion's validity window. Short by
//     design: an assertion binds one message, consumed once, so the window
//     only needs to cover mint-to-delivery, never a whole session. 2 minutes
//     is generous for an intra-house gateway hop (loopback or a house-network
//     A2A hop) and short enough that a captured, unused assertion is
//     worthless to a replayer within any realistic detection window.
//   - comms.DefaultSkew = 1m — how far AHEAD of this gateway's own clock a
//     mint may legitimately claim to be issued (Verify's not-yet-valid
//     check). 1 minute absorbs ordinary NTP drift between desk hosts without
//     opening a window wide enough to matter for a 2-minute-lifetime
//     assertion.
//
// NewPreCheckDeps passes comms.DefaultSkew explicitly (rather than leaving
// deps.Skew at its zero value for comms.Verify to default) so the value this
// gateway runs with is named at its own construction site, not only inside
// identity.go; Mint's own zero-ttl default already resolves to
// comms.DefaultTTL on the signing side, per identity.go.

// NewPreCheckDeps builds the real PreCheckDeps for a running gateway: the
// compiled lane ACL, the loaded trust store, a FRESH in-memory replay guard
// (never nil — see the file doc above), the claims directory under the
// gateway's queue root, and a rate limiter. trustStorePath is a JSON file
// mapping cell -> base64-encoded ed25519 public key.
func NewPreCheckDeps(cfg Config) (PreCheckDeps, error) {
	trust, err := LoadTrustStore(cfg.TrustStore)
	if err != nil {
		return PreCheckDeps{}, err
	}
	acl := comms.Compiled()
	return PreCheckDeps{
		Trust:       trust,
		Skew:        comms.DefaultSkew,
		ReplayGuard: comms.NewMemReplayGuard(), // ALWAYS non-nil; see file doc + TestReplayGuardWired.
		ACL:         &acl,
		ClaimConfig: deskkit.ClaimConfig{
			ClaimsDir: filepath.Join(cfg.QueueDir, "claims"),
		},
		Owner:       "commsgw@" + cfg.Cell,
		RateLimiter: NewRateLimiter(defaultRateLimit, defaultRateWindow),
	}, nil
}

// LoadTrustStore reads a JSON {cell: base64-pubkey} file into a
// comms.Ed25519TrustStore. It is parse-or-refuse: a malformed file, an entry
// that is not valid base64, or a decoded key of the wrong length is a load
// error naming the cell — never a partially-populated store returned alongside
// an error.
func LoadTrustStore(path string) (comms.Ed25519TrustStore, error) {
	if path == "" {
		return nil, deskkit.Refused("commsgw: trust store path is empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, deskkit.Unverifiable(fmt.Sprintf("commsgw: cannot read trust store %s", path), err)
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, deskkit.Refused(fmt.Sprintf("commsgw: trust store %s does not parse as JSON: %v", path, err))
	}
	out := make(comms.Ed25519TrustStore, len(m))
	for cell, enc := range m {
		if cell == "" {
			return nil, deskkit.Refused(fmt.Sprintf("commsgw: trust store %s: empty cell key", path))
		}
		raw, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			return nil, deskkit.Refused(fmt.Sprintf("commsgw: trust store %s: cell %q key is not valid base64: %v", path, cell, err))
		}
		if len(raw) != ed25519.PublicKeySize {
			return nil, deskkit.Refused(fmt.Sprintf("commsgw: trust store %s: cell %q key is %d bytes, want %d", path, cell, len(raw), ed25519.PublicKeySize))
		}
		out[cell] = ed25519.PublicKey(raw)
	}
	return out, nil
}
