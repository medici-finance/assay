package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// --- Verify row 7: InertWithoutConfig ---------------------------------------

func TestInertWithoutConfig(t *testing.T) {
	empty := func(string) string { return "" }

	t.Run("LoadConfig refuses with nothing set", func(t *testing.T) {
		_, err := LoadConfig(empty)
		if err == nil {
			t.Fatalf("want a refusal with no ASSAY_COMMS_* keys set")
		}
		if !deskkit.IsRefused(err) {
			t.Fatalf("want ExitRefused, got %v", err)
		}
	})

	t.Run("run() refuses to serve with nothing set", func(t *testing.T) {
		code := run(empty)
		if code != deskkit.ExitRefused {
			t.Fatalf("run() = %d, want %d (ExitRefused) — the gateway must never start serving without every enable key", code, deskkit.ExitRefused)
		}
	})

	t.Run("enable key present but not \"1\" still refuses", func(t *testing.T) {
		env := map[string]string{
			EnvEnable: "true", // NOT "1" — must not be treated as enabled
			EnvCell:   "cell-a", EnvQueueDir: "/tmp/q", EnvSocket: "/tmp/s",
			EnvListen: ":0", EnvTLSCert: "/tmp/c", EnvTLSKey: "/tmp/k",
			EnvClientCA: "/tmp/ca", EnvTrustStore: "/tmp/t",
		}
		_, err := LoadConfig(func(k string) string { return env[k] })
		if !deskkit.IsRefused(err) {
			t.Fatalf("want ExitRefused for %s=%q, got %v", EnvEnable, env[EnvEnable], err)
		}
	})

	t.Run("every key present and enable=1 loads cleanly", func(t *testing.T) {
		env := map[string]string{
			EnvEnable: "1", EnvCell: "cell-a", EnvQueueDir: "/tmp/q", EnvSocket: "/tmp/s",
			EnvListen: ":0", EnvTLSCert: "/tmp/c", EnvTLSKey: "/tmp/k",
			EnvClientCA: "/tmp/ca", EnvTrustStore: "/tmp/t",
		}
		cfg, err := LoadConfig(func(k string) string { return env[k] })
		if err != nil {
			t.Fatalf("fully-configured LoadConfig should succeed, got %v", err)
		}
		if cfg.Cell != "cell-a" {
			t.Fatalf("cfg.Cell = %q, want cell-a", cfg.Cell)
		}
	})
}

// --- InertWithoutAllKeys ----------------------------------------------------
//
// The house enablement contract (config.go's own doc comment) is THREE
// independent off-switches: a
// topology `comms:` key, ASSAY_COMMS_* env, and the gateway actually
// deployed — any ONE absent leaves the cell inert. This binary's own slice of
// that contract is the ASSAY_COMMS_* env surface (config.go's requiredEnv).
// TestInertWithoutConfig above proves the ALL-ABSENT and the
// enable-flag-wrong-value cases; this test proves the NEGATIVE-PATH case that
// matters most for an independent-off-switches design — removing exactly ONE
// otherwise-fully-configured key still refuses — for every key individually,
// not just the all-or-nothing extremes.
func TestInertWithoutAllKeys(t *testing.T) {
	full := func() map[string]string {
		return map[string]string{
			EnvEnable: "1", EnvCell: "cell-a", EnvQueueDir: "/tmp/q", EnvSocket: "/tmp/s",
			EnvListen: ":0", EnvTLSCert: "/tmp/c", EnvTLSKey: "/tmp/k",
			EnvClientCA: "/tmp/ca", EnvTrustStore: "/tmp/t",
		}
	}

	for _, missing := range requiredEnv {
		t.Run("missing "+missing, func(t *testing.T) {
			env := full()
			delete(env, missing)
			getenv := func(k string) string { return env[k] }

			_, err := LoadConfig(getenv)
			if err == nil {
				t.Fatalf("LoadConfig should refuse with %s absent (independent off-switch — every other key "+
					"present must never be treated as \"close enough\")", missing)
			}
			if !deskkit.IsRefused(err) {
				t.Fatalf("LoadConfig with %s absent: want ExitRefused, got %v", missing, err)
			}

			code := run(getenv)
			if code != deskkit.ExitRefused {
				t.Fatalf("run() with %s absent = %d, want %d (ExitRefused) — the gateway must never start "+
					"serving with any one enable key missing", missing, code, deskkit.ExitRefused)
			}
		})
	}

	t.Run("every key present refuses none of them (positive control)", func(t *testing.T) {
		env := full()
		if _, err := LoadConfig(func(k string) string { return env[k] }); err != nil {
			t.Fatalf("POSITIVE CONTROL FAILED: a fully-configured env should not refuse, got %v\n"+
				"  Without this, the per-key removals above could all be passing vacuously — e.g. if "+
				"requiredEnv or `full()` had drifted out of sync and every removal already refused before "+
				"the delete.", err)
		}
	})
}
