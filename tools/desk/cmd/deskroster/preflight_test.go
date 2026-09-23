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

// greenPreflightProbes is an all-green, fully-INJECTED probe set for role verifier under
// fullRosterForPreflight — every edge is stubbed so the suite is hermetic (no mint, no git
// remote, no `gh`). Each ambient-identity sub-case below darkens exactly the one probe it
// tests, so a red CheckAmbientID is attributable to that probe and nothing else.
func greenPreflightProbes() deskkit.PreflightProbes {
	return deskkit.PreflightProbes{
		ColdMint:          func(string, string) (string, error) { return "/tmp/fake-token", nil },
		ResolveForgeKind:  func(string) deskkit.ForgeKind { return deskkit.ForgeGitHub },
		GitLabColdCustody: func(string) (string, error) { return "/tmp/fake-gitlab-token", nil },
		GrantedScopes: func(string, string) (map[string]string, error) {
			return map[string]string{"pull_requests": "write", "issues": "write", "contents": "write"}, nil
		},
		WriteTransport: func(deskkit.Landing) (deskkit.ProbeVerdict, string, error) {
			return deskkit.ProbePermitted, "up to date", nil
		},
		CommitEmail:    func(string) (string, error) { return "300000005+assay-verifier-app[bot]@users.noreply.github.com", nil },
		AppIDFor:       func(string) (string, error) { return "400000005", nil },
		QueuedSiblings: func(string) ([]deskkit.SiblingReq, error) { return nil, nil },
		DirExists:      func(string) (bool, error) { return true, nil },
		// Ambient identity: the blessing human, with a helper that resolves to the App token.
		AmbientLogin:         func() (string, error) { return "ada", nil },
		CredHelperMatchesApp: func(deskkit.Landing, string) (bool, string, error) { return true, "app-token helper", nil },
	}
}

func ambientIDCheck(t *testing.T, p deskkit.PreflightProbes) deskkit.Check {
	t.Helper()
	rep := deskkit.PreflightRequest{Role: "verifier", Root: t.TempDir(), Probes: p}.Run()
	for _, c := range rep.Checks {
		if c.Name == deskkit.CheckAmbientID {
			return c
		}
	}
	t.Fatalf("the report carries no %s check", deskkit.CheckAmbientID)
	return deskkit.Check{}
}

// TestPreflightAmbientIdentity is Verify row 4, the negative-path row for the remaining
// credfence layer: the ambient `gh` login the tools would fall through to must be the blessing
// human (not a bot slug, not a non-blessing login), and the origin credential helper must
// resolve to the minted App token. It exercises the deskroster preflight's own check via the
// injectable deskkit API — the check lives in deskkit (where preflight.go is), and the verb is
// a thin wrapper, so testing it through the injectable API is testing exactly what the
// `deskroster preflight` verb runs.
//
// FAIL-FIRST: on the pre-brief code there is no ambient-identity check at all, so ambientIDCheck
// t.Fatalf's ("no ambient-identity check") on the unfixed tree; the fix is the whole check.
func TestPreflightAmbientIdentity(t *testing.T) {
	installRosterEnv(t, fullRosterForPreflight) // blessing login ada; assay-verifier-app is a bot slug

	// bot login → red.
	p := greenPreflightProbes()
	p.AmbientLogin = func() (string, error) { return "assay-verifier-app[bot]", nil }
	if c := ambientIDCheck(t, p); c.State != deskkit.CheckedFailed {
		t.Fatalf("bot ambient login = %s, want checked-failed (%s)", c.State, c.Detail)
	}

	// non-blessing human → red.
	p = greenPreflightProbes()
	p.AmbientLogin = func() (string, error) { return "mallory", nil }
	if c := ambientIDCheck(t, p); c.State != deskkit.CheckedFailed {
		t.Fatalf("non-blessing ambient login = %s, want checked-failed (%s)", c.State, c.Detail)
	}

	// blessing human + matching helper → green.
	if c := ambientIDCheck(t, greenPreflightProbes()); c.State != deskkit.CheckedClean {
		t.Fatalf("blessing ambient login + matching helper = %s, want checked-clean (%s)", c.State, c.Detail)
	}

	// helper mismatch → red (independent of the identity half).
	p = greenPreflightProbes()
	p.CredHelperMatchesApp = func(deskkit.Landing, string) (bool, string, error) {
		return false, "osxkeychain, not the App token", nil
	}
	if c := ambientIDCheck(t, p); c.State != deskkit.CheckedFailed {
		t.Fatalf("helper mismatch = %s, want checked-failed (%s)", c.State, c.Detail)
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
