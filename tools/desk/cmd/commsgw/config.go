package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// config.go — commsgw's enablement surface.
//
// CONFIG-OFF DEFAULT. The gateway is the one chokepoint every inbound cell
// message crosses; standing it up by accident (a forgotten env var in a shell
// profile, a half-copied unit file) must never be possible. So every one of
// the keys below is REQUIRED, and absence of ANY ONE of them is a refusal to
// serve — there is no partially-enabled state, and no key defaults to a
// guessed value. This mirrors the house's recorded enablement rule ("per-cell
// topology `comms:` key, ASSAY_COMMS_* env, gateway deployment; all three
// required, any one absent = inert"): this binary's own slice of that
// contract is the ASSAY_COMMS_* env surface below.
const (
	// EnvEnable must be exactly "1". Every other key present but this one absent
	// (or anything other than "1") still refuses — the enable flag is not
	// inferred from "the rest of the config looks complete".
	EnvEnable = "ASSAY_COMMS_GATEWAY_ENABLE"
	// EnvCell names this gateway's own cell (the `cell` a locally-signed
	// assertion must assert, and the identity this process's own logs/audits
	// attribute to).
	EnvCell = "ASSAY_COMMS_CELL"
	// EnvQueueDir is the durable on-disk root for the accepted-queue and the
	// per-(cell,role) + held mailboxes (mailbox.go). commsloop (a SEPARATE
	// process/binary) reads the same directory, so gateway and drain agree on
	// disk rather than over an in-process channel.
	EnvQueueDir = "ASSAY_COMMS_QUEUE_DIR"
	// EnvSocket is the within-cell loopback Unix-domain socket path deskcomms's
	// client (cmd/deskcomms/gateway.go) dials. Within-cell sends traverse the
	// SAME precheck pipeline as cross-cell ones — one enforcement point.
	EnvSocket = "ASSAY_COMMS_SOCKET"
	// EnvListen is the network address (host:port) the cross-cell A2A server
	// listens on, e.g. ":8443".
	EnvListen = "ASSAY_COMMS_LISTEN"
	// EnvTLSCert / EnvTLSKey are this gateway's OWN mTLS server certificate and
	// key — the identity the transport presents to a peer gateway.
	EnvTLSCert = "ASSAY_COMMS_TLS_CERT"
	EnvTLSKey  = "ASSAY_COMMS_TLS_KEY"
	// EnvClientCA is the house-side trust store (a PEM CA bundle) commsgw uses to
	// verify-or-refuse an inbound peer gateway's client certificate — the mTLS
	// half of peer auth (identity.go's Assertion is the second, finer-grained
	// half, bound per-message rather than per-connection).
	EnvClientCA = "ASSAY_COMMS_CLIENT_CA"
	// EnvTrustStore is a JSON file mapping cell -> ed25519 public key, the
	// comms.TrustStore this gateway verifies signed assertions against. KEY
	// CUSTODY: this file holds only PUBLIC keys; the matching private signing
	// keys live per-role, outside this process, under the App-PEM custody rules
	// (identity.go's doc comment).
	EnvTrustStore = "ASSAY_COMMS_TRUST_STORE"
)

// requiredEnv is every key Config-off enablement gates on. Order is the order
// LoadConfig reports missing keys in, so the refusal message is stable.
var requiredEnv = []string{
	EnvEnable, EnvCell, EnvQueueDir, EnvSocket, EnvListen,
	EnvTLSCert, EnvTLSKey, EnvClientCA, EnvTrustStore,
}

// Config is commsgw's resolved, validated configuration. It is only ever
// returned alongside a nil error when every required key was present AND
// EnvEnable == "1" — there is no zero-value Config a caller could accidentally
// serve with.
type Config struct {
	Cell         string
	QueueDir     string
	Socket       string
	Listen       string
	TLSCertPath  string
	TLSKeyPath   string
	ClientCAPath string
	TrustStore   string
}

// LoadConfig reads and validates the ASSAY_COMMS_* enablement surface via
// getenv (os.Getenv in production; a test supplies a closed map so the absence
// of a key is never confused with an unset-but-inherited real environment
// variable). Any required key absent, OR EnvEnable != "1", is refused
// (deskkit.ExitRefused) naming every missing key — never a partial start.
func LoadConfig(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	var missing []string
	for _, k := range requiredEnv {
		if strings.TrimSpace(getenv(k)) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return Config{}, refusedf(
			"commsgw: inert — missing enable key(s) %s (config-off default: every ASSAY_COMMS_* "+
				"key is required, absent is refused, never a partial start)",
			strings.Join(missing, ", "))
	}
	if v := getenv(EnvEnable); v != "1" {
		return Config{}, refusedf(
			"commsgw: inert — %s=%q, want \"1\" (config-off default: an unrecognised enable "+
				"value is refused, never treated as \"enabled because everything else is set\")",
			EnvEnable, v)
	}
	return Config{
		Cell:         getenv(EnvCell),
		QueueDir:     getenv(EnvQueueDir),
		Socket:       getenv(EnvSocket),
		Listen:       getenv(EnvListen),
		TLSCertPath:  getenv(EnvTLSCert),
		TLSKeyPath:   getenv(EnvTLSKey),
		ClientCAPath: getenv(EnvClientCA),
		TrustStore:   getenv(EnvTrustStore),
	}, nil
}

func refusedf(format string, a ...any) error {
	return deskkit.Refused(fmt.Sprintf(format, a...))
}
