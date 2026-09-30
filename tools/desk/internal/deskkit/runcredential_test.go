package deskkit

import (
	"strings"
	"testing"
)

// runcredential_test.go — ResolveRunCredential's three outcomes (forge-neutral brief 14), which the
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

// TestRunCredentialsInvalidRefusesAll — ASSAY_RUN_CREDENTIALS is a TRUST key: it decides which
// credential a desk write runs as, so a malformed entry refuses the WHOLE roster, exactly as a
// malformed ASSAY_TRUSTED_LOGINS does. It is never recorded as a per-key extension result, and
// no binding survives — the well-formed sibling included.
//
// It also proves the LOWER layer holds with the UPPER one bypassed: the upper layer is the verb's
// activation gate (deskrun's CheckVerbActivation refuses on an unconfigured roster). This test
// calls ResolveRunCredential directly, never through that gate, and the resolver must still
// refuse — could-not-check naming the key and the refused roster — rather than resolve anything.
func TestRunCredentialsInvalidRefusesAll(t *testing.T) {
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
		if cfg.Configured() {
			t.Errorf("%q: the roster still loaded — a malformed trust key must refuse the WHOLE configuration", bad)
		}
		named := false
		for _, p := range cfg.Problems {
			if strings.Contains(p, EnvRunCredentials) {
				named = true
			}
		}
		if !named {
			t.Errorf("%q: no roster problem names %s: %v", bad, EnvRunCredentials, cfg.Problems)
		}
		if ext, ok := cfg.Ext["run-credentials"]; ok {
			t.Errorf("%q: recorded as an extension result %+v — a trust key has no per-key outcome", bad, ext)
		}
		if len(cfg.RunCredentials) != 0 {
			t.Errorf("%q: a refused roster still carries bindings %v", bad, cfg.RunCredentials)
		}
		_, err := ResolveRunCredential(ForgeRepo{Owner: "example-org", Name: "good"})
		if ExitCodeOf(err) != ExitUnverifiable {
			t.Errorf("%q: the well-formed sibling resolved with the gate bypassed (err=%v)", bad, err)
			continue
		}
		if msg := err.Error(); !strings.Contains(msg, EnvRunCredentials) || !strings.Contains(msg, "roster is refused") {
			t.Errorf("%q: the refusal must name %s and the refused roster: %v", bad, EnvRunCredentials, err)
		}
	}
}

// TestRunCredentialsIsTrustKey — the classification itself: ExtKeyName reports ASSAY_RUN_CREDENTIALS
// as NOT an extension key (ok=false, the answer it gives every trust key), and a well-formed roster
// carrying it records no `assay.roster.ext.run-credentials` outcome.
func TestRunCredentialsIsTrustKey(t *testing.T) {
	if name, ok := ExtKeyName(EnvRunCredentials); ok {
		t.Fatalf("ExtKeyName(%s) = %q, ok=true — it chooses which credential acts, so it is a trust key",
			EnvRunCredentials, name)
	}
	runCredRoster(t, "example-org/auto=release-runner", "")
	cfg := EffectiveConfig()
	if !cfg.Configured() {
		t.Fatalf("a well-formed %s refused the roster: %v", EnvRunCredentials, cfg.Problems)
	}
	if ext, ok := cfg.Ext["run-credentials"]; ok {
		t.Fatalf("cfg.Ext[run-credentials] = %+v — a trust key has no extension outcome", ext)
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
