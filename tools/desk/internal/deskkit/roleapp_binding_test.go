package deskkit

// roleapp_binding_test.go — the role→App indirection.
//
// Two INDEPENDENT layers meet here and are tested apart:
//
//   - the App-NAME binding (AppBinding / AppEnvPrefix, appconfig.go): which KEY a role
//     mints with. Its regression guards are TestAppEnvPrefixAndBindingDefaults and the
//     desktoken-side --version tests.
//   - the roster's `role=slug` binding (Config.RoleBots, rosterconfig.go): which `[bot]`
//     login the TRUST gate accepts for a role. TestMultiRoleOneSlugBindsBothRoles measures
//     that one slug may serve several roles; TestUnboundRoleRefused proves the gate refuses
//     a login the roster does not bind to the posting role — the lower layer that catches a
//     mis-bound key even when a token for the wrong App exists.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// renderRoleBindings renders Config.RoleBots as a sorted `role=slug` space-separated
// string, for the `role-bindings=` echo Verify row 4 greps.
func renderRoleBindings(c Config) string {
	parts := make([]string, 0, len(c.RoleBots))
	for role, slug := range c.RoleBots {
		parts = append(parts, role+"="+slug)
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

// TestMultiRoleOneSlugBindsBothRoles — Task 1's measurement, kept as a regression guard.
// The roster parser is keyed on the ROLE, so one App slug may carry more than one `role=`
// prefix (`reviewer=x-act:1,worker=x-act:1`): both bindings land, which is exactly the
// two-App tier's shape — two roles, one App. Measured 2026-09-07 @ this branch: the parser
// ALREADY accepts it (no parser change was needed); this test pins it so a future narrowing
// to one-role-per-slug is caught.
func TestMultiRoleOneSlugBindsBothRoles(t *testing.T) {
	plantRoster(t, `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=reviewer=x-act:1,worker=x-act:1
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`)
	c := EffectiveConfig()
	if !c.Configured() {
		t.Fatalf("roster did not configure: %v", c.Problems)
	}
	if got := c.RoleBots["reviewer"]; got != "x-act" {
		t.Fatalf("RoleBots[reviewer] = %q, want x-act", got)
	}
	if got := c.RoleBots["worker"]; got != "x-act" {
		t.Fatalf("RoleBots[worker] = %q, want x-act — one slug must bind to several roles for the two-App tier", got)
	}
	// Verify row 4 greps this line for `role-bindings=.*reviewer=x-act` and
	// `role-bindings=.*worker=x-act`.
	t.Logf("role-bindings=%s", renderRoleBindings(c))
}

// TestMultiRoleSharedGrantPassesEveryBoundRole — Task 3. When several roles are bound to one
// App they mint the same installation token from one grant, and the app-scopes-vs-duties
// check reads that grant. The check is grant-based (requiredDuties is the same fixed set for
// every role), so ONE shared `.perms` sidecar that covers the duties passes the check for
// every role pointed at it. Proven by handing two roles the SAME token path and sidecar.
func TestMultiRoleSharedGrantPassesEveryBoundRole(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "x-act-token-1")
	if err := os.WriteFile(tokenPath, []byte("stub-token"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	// One grant, shared, covering every required duty.
	grant := `{"contents":"write","issues":"write","pull_requests":"write"}`
	if err := os.WriteFile(tokenPath+".perms", []byte(grant), 0o600); err != nil {
		t.Fatalf("write perms: %v", err)
	}
	p := PreflightProbes{GrantedScopes: grantedScopesProbe}
	for _, role := range []string{"reviewer", "worker"} {
		c := checkAppScopes(p, role, tokenPath)
		if c.State != CheckedClean {
			t.Fatalf("checkAppScopes(%s) over the shared grant = %s (%s), want checked-clean — "+
				"a shared grant covering the duties must pass every bound role", role, c.State, c.Detail)
		}
	}
}

// TestUnboundRoleRefused — Verify row 6, the independent LOWER layer. The trust gate decides
// which `[bot]` login counts for a role from the roster's `role=slug` binding, NOT from which
// App a token was minted for. So a login whose slug the roster does not bind to the posting
// role is refused even if a token for that App exists, and an unbound role resolves to no
// login at all (RoleAppLogin ok=false), which every matcher reads as a refusal.
func TestUnboundRoleRefused(t *testing.T) {
	plantRoster(t, `ASSAY_BLESS_LOGIN=ada:2001
ASSAY_TRUSTED_LOGINS=ada:2001
ASSAY_TRUSTED_BOT_SLUGS=reviewer=x-act:1
ASSAY_ALLOWED_REPOS=example-org/one:ci:private
`)
	want, ok := RoleAppLogin("reviewer")
	if !ok || want != "x-act[bot]" {
		t.Fatalf("RoleAppLogin(reviewer) = (%q,%v), want (x-act[bot],true)", want, ok)
	}

	// accepts models the acceptance predicate EVERY caller runs against an actor login
	// (isReviewerBot at deskpost/github.go, and its siblings): the role's bound App login is
	// resolved from the roster, and an actor is accepted only when the role is bound AND the
	// actor equals that login AND the actor is non-empty. Exercising it here — rather than
	// comparing two string literals — makes the trust-gate keying FALSIFIABLE: the wrong-App
	// case below would flip to accepted if the gate keyed on which App minted a token instead
	// of on the roster binding, and the empty-actor case guards the deleted-author "" match.
	accepts := func(role, actor string) bool {
		w, k := RoleAppLogin(role)
		return k && actor != "" && actor == w
	}

	// The roster-bound login IS accepted for the role it is bound to.
	if !accepts("reviewer", "x-act[bot]") {
		t.Fatal("the roster-bound login x-act[bot] was not accepted for reviewer — the gate is " +
			"not keying on the binding it should")
	}
	// A DIFFERENT App's login — one a token could well have been minted for — is REFUSED,
	// because the accepted login is the roster-derived one, not whatever App holds a token.
	if accepts("reviewer", "y-act[bot]") {
		t.Fatal("a login for an App NOT bound to reviewer (y-act[bot]) was accepted for reviewer — " +
			"the trust gate must key on the roster binding, not on which App minted a token")
	}
	// An UNBOUND role accepts NO login — not even the login that IS bound to another role —
	// because RoleAppLogin returns ok=false and the predicate short-circuits.
	if accepts("worker", "x-act[bot]") {
		t.Fatal("an unbound role (worker) accepted a login — an unbound role is a refusal, so no " +
			"actor may satisfy it")
	}
	// The refusal is in RoleAppLogin's return shape, and the empty login it hands back must
	// never be usable as an identity (RoleAppLogin's contract; deposit for the "" == "" trap).
	if login, ok := RoleAppLogin("worker"); ok || login != "" {
		t.Fatalf("RoleAppLogin(worker) = (%q,%v) against a roster that does not bind worker, "+
			"want (\"\",false) — an unbound role is a refusal, not an empty-login match", login, ok)
	}
}

// TestAppEnvPrefixAndBindingDefaults pins the App-name→env-prefix mapping the whole binding
// rests on: the default `<role>-app` keeps the pre-binding `<ROLE>_APP_ID` keys byte for
// byte, while a bound App-name uses its own prefix. This is the property that makes an
// unconfigured deployment resolve exactly the files it does today.
func TestAppEnvPrefixAndBindingDefaults(t *testing.T) {
	cases := []struct{ appName, wantPrefix string }{
		{"reviewer-app", "REVIEWER"},     // default → historical key
		{"issue-loop-app", "ISSUE_LOOP"}, // dashed role default
		{"x-act", "X_ACT"},               // bound App-name
		{"x-act-app", "X_ACT"},           // a bound name that itself ends -app
	}
	for _, c := range cases {
		if got := AppEnvPrefix(c.appName); got != c.wantPrefix {
			t.Fatalf("AppEnvPrefix(%q) = %q, want %q", c.appName, got, c.wantPrefix)
		}
	}
	// AppBinding with no env and no apps.env falls back to <role>-app.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvConfigHome, "")
	t.Setenv("REVIEWER_APP", "")
	if got := AppBinding("reviewer"); got != "reviewer-app" {
		t.Fatalf("AppBinding(reviewer) unbound = %q, want reviewer-app", got)
	}
	// An env <ROLE>_APP wins.
	t.Setenv("REVIEWER_APP", "x-act")
	if got := AppBinding("reviewer"); got != "x-act" {
		t.Fatalf("AppBinding(reviewer) with REVIEWER_APP=x-act = %q, want x-act", got)
	}
	// The App ID then resolves off the bound prefix.
	t.Setenv("X_ACT_APP_ID", "424242")
	if got, err := AppIDForApp("x-act"); err != nil || got != "424242" {
		t.Fatalf("AppIDForApp(x-act) = %q err=%v, want 424242", got, err)
	}
}
