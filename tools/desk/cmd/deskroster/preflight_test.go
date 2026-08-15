package main

// The CLI contract for `deskroster preflight`.
//
// The load-bearing property is the one a CONSUMER depends on: `deskroster
// preflight --help` exits 0, and an unknown subcommand exits 5. That pair is how
// a skill's boot section, or CI, answers "does this build of deskroster have the
// preflight verb?" in one call with no output parsing — and it is why the verb
// had to be registered rather than left to the default branch, which refuses.

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestPreflightHelpExitsZero — the capability probe. Before the verb existed
// this argv fell through to the unknown-subcommand branch and exited 5.
func TestPreflightHelpExitsZero(t *testing.T) {
	installRosterEnv(t, fullRosterForPreflight)
	for _, argv := range [][]string{
		{"preflight", "--help"},
		{"preflight", "-h"},
		{"preflight", "help"},
	} {
		if got := run(argv); got != deskkit.ExitOK {
			t.Errorf("deskroster %s exited %d, want 0", strings.Join(argv, " "), got)
		}
	}
}

// TestPreflightUnknownSubcommandStillExitsFive — the other half of the pair. If
// an unknown verb ALSO exited 0 the probe above would prove nothing.
func TestPreflightUnknownSubcommandStillExitsFive(t *testing.T) {
	installRosterEnv(t, fullRosterForPreflight)
	if got := run([]string{"preflight-typo", "--help"}); got != deskkit.ExitRefused {
		t.Fatalf("an unknown subcommand exited %d, want 5 — the --help probe would be meaningless", got)
	}
}

// TestPreflightRequiresARole — the envelope is checked FOR a role. Defaulting to
// one would let a session pass a preflight for an identity it is not using.
func TestPreflightRequiresARole(t *testing.T) {
	installRosterEnv(t, fullRosterForPreflight)
	if got := run([]string{"preflight"}); got != deskkit.ExitRefused {
		t.Fatalf("preflight with no --role exited %d, want 5 (refused)", got)
	}
}

// fullRosterForPreflight is a usable roster (the CLI calls EchoEffectiveConfig on
// every run, and the guard path reads the config home).
const fullRosterForPreflight = `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001,shared-agent:2002
ASSAY_TRUSTED_BOT_SLUGS=verifier=assay-verifier-app:300000005,reviewer=assay-reviewer-app:300000004
ASSAY_ALLOWED_REPOS=medici-finance/assay:ci:private
ASSAY_HUMAN_LOGIN_MAP=alex:ada
`
