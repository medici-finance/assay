package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// client.go — the session-context identity and the injected dependencies the verbs
// run against. It is deliberately the ONE place that reads WHO this session is: the
// verbs never take that from their arguments, so there is no argument a caller can
// pass to act as another cell or another desk. This mirrors the impersonation
// precedent (internal/deskkit/impersonation.go): a write path that cannot be told
// "you are someone else" cannot be tricked into being them.

// Session-context environment. These are the identity the session ESTABLISHED when
// it minted its token at boot — not per-invocation switches a caller flips. A
// desk's cell and desk-role are fixed for the life of its session; carrying them as
// context is what keeps them off the command line, where a self-claimed sender is
// the whole class of bug the gateway then has to catch a second time.
const (
	envCell    = "DESK_CELL"          // the local cell this session belongs to
	envDesk    = "DESK_ROLE"          // the desk-role this session minted its token as
	envGateway = "DESK_COMMS_GATEWAY" // the local gateway's loopback socket path
	envKey     = "DESK_COMMS_KEY"     // path to this desk-role's ed25519 signing seed (0600)
)

// deps is everything a verb needs, injected so a unit test drives the whole verb
// with no environment, no key file, and no socket. main.go builds the real deps
// (real identity, real signer, real gateway); a test builds a deps with fakes.
type deps struct {
	stdin  io.Reader
	stdout io.Writer
	now    func() time.Time

	// Session identity — established from context (envCell / envDesk), never from
	// an argument. cell is the local cell; self is this session's desk-role.
	cell string
	self string

	// signer mints the per-message assertion with this desk-role's custody-managed
	// key. A nil signer is could-not-mint: the verb refuses rather than sending an
	// unsigned message the gateway would reject anyway.
	signer  comms.Signer
	gateway Gateway

	// rateCheck gates one outbound send against the shared outward-write budget and
	// circuit breaker (deskkit/ratelimit.go). Injected so a test can pin a
	// budget/breaker refusal without seeding an audit history.
	rateCheck func(bucket string) error

	// newID / newNonce mint the per-message id and single-use nonce. Injected so a
	// test gets deterministic values.
	newID    func() string
	newNonce func() string
}

// resolveIdentity reads the session's cell and desk-role from context. Both empty
// or partially set is a refusal: a message with no established sender identity is
// not a low-privilege message, it is an unsendable one (the same posture
// comms.ParseEnvelope takes on an absent {from,to,verb} triple).
func resolveIdentity() (cell, self string, err error) {
	cell = strings.TrimSpace(os.Getenv(envCell))
	self = strings.TrimSpace(os.Getenv(envDesk))
	if cell == "" || self == "" {
		return "", "", deskkit.Refused(fmt.Sprintf(
			"refused: this session has no established identity — set %s and %s in the "+
				"session context (they are minted at boot, never passed per call). "+
				"An unsigned, unattributed message is not sendable.", envCell, envDesk))
	}
	return cell, self, nil
}

// loadSigner loads this desk-role's ed25519 signing key from the custody-managed
// file at $DESK_COMMS_KEY. The key is operator config under the App-PEM custody
// rules — mode 0600, never committed, never in the source tree — so this reads it
// from outside and refuses a world-readable or absent key rather than minting an
// assertion no verifier could trust. The file holds the ed25519 SEED (32 bytes) or
// full private key (64 bytes) as hex.
func loadSigner() (comms.Signer, error) {
	path := strings.TrimSpace(os.Getenv(envKey))
	if path == "" {
		return nil, deskkit.Unverifiable(fmt.Sprintf(
			"could-not-mint: no signing key configured — set %s to this desk-role's "+
				"custody-managed ed25519 key (mode 0600)", envKey), nil)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, deskkit.Unverifiable("could-not-mint: signing key is not readable", err)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return nil, deskkit.Refused(fmt.Sprintf(
			"refused: signing key %s is mode %o — a comms signing key must be 0600 "+
				"(custody rule), never group- or world-readable", path, fi.Mode().Perm()))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, deskkit.Unverifiable("could-not-mint: signing key is not readable", err)
	}
	key, err := parseEd25519Key(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, deskkit.Unverifiable("could-not-mint: signing key is malformed", err)
	}
	return comms.Ed25519Signer{Key: key}, nil
}

// parseEd25519Key decodes a hex-encoded ed25519 seed (32 bytes) or full private key
// (64 bytes) into a private key. Any other length is refused.
func parseEd25519Key(s string) (ed25519.PrivateKey, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("key is not valid hex: %w", err)
	}
	switch len(b) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(b), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(b), nil
	default:
		return nil, fmt.Errorf("key is %d bytes; want a %d-byte seed or %d-byte private key",
			len(b), ed25519.SeedSize, ed25519.PrivateKeySize)
	}
}

// randHex mints a random lowercase-hex token of n bytes, for message ids and
// nonces. crypto/rand so a nonce is unguessable and a collision is negligible.
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// rand.Read failing is catastrophic and never expected on the platforms the
		// desk runs on; fall back to a time-seeded value that is still unique enough
		// for an id, rather than panicking a CLI mid-send.
		return fmt.Sprintf("t%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// buildDeps assembles the real dependencies from the environment: session
// identity, a signer over the custody key, and the loopback socket gateway. It is
// the boundary between "read the world" and "run the verb", so the verbs stay pure.
func buildDeps(stdin io.Reader, stdout io.Writer) (*deps, error) {
	cell, self, err := resolveIdentity()
	if err != nil {
		return nil, err
	}
	return &deps{
		stdin:   stdin,
		stdout:  stdout,
		now:     time.Now,
		cell:    cell,
		self:    self,
		signer:  nil, // loaded lazily by runSend (send.go step 7) via loadSigner(), after every refusal check
		gateway: socketGateway{network: "unix", addr: strings.TrimSpace(os.Getenv(envGateway))},
		rateCheck: func(bucket string) error {
			// pr==0: a cell message carries no PR; the destination lane is the bucket.
			return deskkit.AllowWrite("deskcomms", bucket, 0)
		},
		newID:    func() string { return randHex(16) },
		newNonce: func() string { return randHex(16) },
	}, nil
}
