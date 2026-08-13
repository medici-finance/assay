package main

import (
	"strings"
	"testing"
)

// TestGhRefusesWithoutMintedToken hardens the #565 fix (mirroring #563/#562 for
// deskreply): on the --as-app path (requireWorkerAuth == true), gh() must refuse
// outright rather than silently falling through to whatever gh identity happens to be
// ambient/active in this shell if a future code path forgets to mint the worker token
// first. Tracing every gh() call site on the --as-app path shows they all run strictly
// after mintWorkerToken succeeds, so this is currently unreachable in production — but
// it makes that invariant load-bearing instead of implicit.
func TestGhRefusesWithoutMintedToken(t *testing.T) {
	oldToken, oldRequire := ghToken, requireWorkerAuth
	ghToken = ""
	requireWorkerAuth = true
	t.Cleanup(func() { ghToken, requireWorkerAuth = oldToken, oldRequire })

	_, err := gh(".", "pr", "view", "1")
	if err == nil {
		t.Fatal("gh() with no minted worker token succeeded on the --as-app path; it must refuse rather than fall back to the ambient gh identity/keyring")
	}
	if !strings.Contains(err.Error(), "minted worker token") {
		t.Fatalf("gh() error = %q, want it to name the missing minted token", err.Error())
	}
}

// TestGhAllowsAmbientIdentityWhenAsAppOff is the other half: deskpr, unlike deskreply,
// has a real, documented --as-app=false ambient-identity fallback (the example-org
// fallback). When the caller did not request worker-App auth (requireWorkerAuth ==
// false), an unset ghToken must NOT trip the fail-closed guard — otherwise
// --as-app=false would be broken outright on the very first gh call of a fresh process.
// Routed through the same fake-gh-on-PATH harness withEnv sets up for the rest of this
// package, so this actually exercises gh()/runCmd end to end rather than just the
// guard's boolean logic.
func TestGhAllowsAmbientIdentityWhenAsAppOff(t *testing.T) {
	withEnv(t, t.TempDir())

	oldToken, oldRequire := ghToken, requireWorkerAuth
	ghToken = ""
	requireWorkerAuth = false
	t.Cleanup(func() { ghToken, requireWorkerAuth = oldToken, oldRequire })

	if _, err := gh(".", "pr", "view", "1"); err != nil {
		t.Fatalf("gh() with no minted worker token failed on the --as-app=false path; it must fall back to the ambient gh identity: %v", err)
	}
}
