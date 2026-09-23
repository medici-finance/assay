package deskkit

import (
	"strings"
	"testing"
)

// runcredential_test.go — ResolveRunCredential's three outcomes (forge-neutral/14), which the
// design requires never to be conflated: unbound is a configuration gap (could-not-check),
// human-bound is a deliberate state (Refused), release-runner proceeds.

func runCredRoster(t *testing.T, creds, extraBots string) {
	t.Helper()
	r := goldenRoster()
	if creds != "" {
		r[EnvRunCredentials] = creds
	}
	if extraBots != "" {
		r[EnvTrustedBotSlugs] += "," + extraBots
	}
	withRoster(t, r)
}

func TestResolveRunCredentialOutcomes(t *testing.T) {
	runCredRoster(t, "example-org/auto=release-runner+environment,example-org/manual=human:ada",
		"release-runner=example-release-runner-app:300000009")

	cred, err := ResolveRunCredential(ForgeRepo{Owner: "example-org", Name: "auto"})
	if err != nil || cred.Role != ReleaseRunnerRole || cred.GateShape != GateShapeEnvironment {
		t.Fatalf("release-runner binding: cred=%+v err=%v, want the role with the environment shape", cred, err)
	}

	cred, err = ResolveRunCredential(ForgeRepo{Owner: "Example-Org", Name: "Manual"})
	if ExitCodeOf(err) != ExitRefused {
		t.Fatalf("human binding: err=%v (exit %d), want Refused (exit %d)", err, ExitCodeOf(err), ExitRefused)
	}
	if cred.Human != "ada" || !strings.Contains(err.Error(), "human:ada") || !strings.Contains(strings.ToLower(err.Error()), "example-org/manual") {
		t.Fatalf("human binding refusal must name the human and the repo: cred=%+v err=%v", cred, err)
	}

	_, err = ResolveRunCredential(ForgeRepo{Owner: "example-org", Name: "unbound"})
	if ExitCodeOf(err) != ExitUnverifiable || !strings.Contains(err.Error(), EnvRunCredentials) {
		t.Fatalf("unbound repo: err=%v (exit %d), want could-not-check naming %s", err, ExitCodeOf(err), EnvRunCredentials)
	}
}

// TestRunCredentialsInvalidResetsEveryBinding — a malformed entry marks the key invalid and
// EVERY binding (including the well-formed ones) reads as unbound: never a partial binding set.
func TestRunCredentialsInvalidResetsEveryBinding(t *testing.T) {
	for _, bad := range []string{
		"auto=release-runner",                                        // bare basename
		"example-org/*=release-runner",                               // pattern
		"example-org/auto=worker",                                    // a desk role is not a run credential
		"example-org/auto=human:",                                    // no human named
		"example-org/auto=human:<name>",                              // the unsubstituted placeholder
		"example-org/auto=release-runner+sideways",                   // unknown gate shape
		"example-org/auto=release-runner,example-org/auto=human:ada", // bound twice
	} {
		runCredRoster(t, "example-org/good=release-runner,"+bad, "")
		cfg := EffectiveConfig()
		if !cfg.Configured() {
			t.Errorf("%q refused the WHOLE roster — an extension key's bad value must only empty its own bindings", bad)
		}
		if ext := cfg.Ext["run-credentials"]; ext.Status != ExtInvalid {
			t.Errorf("%q: Ext status %q, want %q", bad, ext.Status, ExtInvalid)
		}
		_, err := ResolveRunCredential(ForgeRepo{Owner: "example-org", Name: "good"})
		if ExitCodeOf(err) != ExitUnverifiable {
			t.Errorf("%q: the well-formed sibling still resolved (err=%v) — a partial binding set", bad, err)
		}
	}
}

// TestResolveRunCredentialRefusesRepurposedApp — the release credential is never a desk role's
// App: a release-runner binding sharing another role's App slug is refused.
func TestResolveRunCredentialRefusesRepurposedApp(t *testing.T) {
	runCredRoster(t, "example-org/auto=release-runner", "release-runner=assay-worker-app:300000006")
	_, err := ResolveRunCredential(ForgeRepo{Owner: "example-org", Name: "auto"})
	if ExitCodeOf(err) != ExitRefused || !strings.Contains(err.Error(), "worker") {
		t.Fatalf("a release-runner bound to the worker App resolved: err=%v, want Refused naming the worker role", err)
	}
}
