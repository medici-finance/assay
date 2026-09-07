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
