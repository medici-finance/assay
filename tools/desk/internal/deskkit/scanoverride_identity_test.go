package deskkit

// The scan-override audit identity resolves to the ROSTER login, never to a bare
// machine account: the row exists so a human can answer for the bypass, and an OS
// account name that is not a roster identity answers nothing. These tests pin the
// resolution order, the distinguishable fallback, and — above all — that identity
// resolution has NO failure mode: a malformed map or an unconfigured roster degrades
// the label, never the audit row's existence (LogScanOverride is fatal on a row that
// does not land, so resolution must never be the reason one does not).

import (
	"os/user"
	"strings"
	"testing"
)

// TestResolveOverrideWho pins the pure resolution order over a Config literal:
// explicit OS-account map, then a human-name match, then an account that IS a
// roster login, then the marked fallback.
func TestResolveOverrideWho(t *testing.T) {
	cfg := Config{
		OSUserLogins: map[string]string{"laptop-acct": "ada"},
		HumanLogins:  map[string]string{"alex": "ada"},
		Humans:       map[string]int64{"ada": 2001, "shared-agent": 2002},
	}
	for _, tc := range []struct {
		name   string
		osUser string
		want   string
	}{
		{"explicit OS-account binding resolves to the roster login", "laptop-acct", "ada"},
		{"the OS-account lookup is case-insensitive", "Laptop-Acct", "ada"},
		{"an OS account named like a mapped human resolves through the human map", "alex", "ada"},
		{"an OS account that IS a trusted roster login renders its canonical spelling", "Shared-Agent", "shared-agent"},
		{"the explicit binding outranks a human-name match on the same account", "laptop-acct", "ada"},
		{"an unmapped account falls back MARKED, never as a bare login lookalike", "somebody", "os:somebody"},
		{"the fallback keeps the platform's own spelling for fidelity", "SomeBody", "os:SomeBody"},
		{"the platform saying nothing stays the long-standing unknown", "", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveOverrideWho(tc.osUser, cfg); got != tc.want {
				t.Errorf("resolveOverrideWho(%q) = %q, want %q", tc.osUser, got, tc.want)
			}
		})
	}

	// The explicit binding outranks EVERYTHING, including an account that is itself
	// a roster login — a machine account deliberately bound to a different
	// accountable identity must not be overridden by a name coincidence.
	clash := Config{
		OSUserLogins: map[string]string{"ada": "shared-agent"},
		Humans:       map[string]int64{"ada": 2001},
	}
	if got := resolveOverrideWho("ada", clash); got != "shared-agent" {
		t.Errorf("explicit binding lost to a Humans membership: got %q, want %q", got, "shared-agent")
	}
}

// TestResolveOverrideWhoNeverPanics: every degraded configuration state — nil maps
// throughout, an unconfigured (zero-value) Config — resolves to the fallback rather
// than panicking or answering "". The audit write depends on it.
func TestResolveOverrideWhoNeverPanics(t *testing.T) {
	for _, cfg := range []Config{
		{},                                    // unconfigured / refused roster
		{OSUserLogins: nil, HumanLogins: nil}, // explicit nils
		{OSUserLogins: map[string]string{"x": ""}}, // an empty login never resolves
		{Humans: map[string]int64{"someone": 0}},   // unrelated membership
	} {
		if got := resolveOverrideWho("absent-user", cfg); !strings.HasPrefix(got, "os:") && got != "unknown" {
			t.Errorf("degraded config %+v resolved %q — only the marked fallback or unknown may answer", cfg, got)
		}
	}
	if got := resolveOverrideWho("x", Config{OSUserLogins: map[string]string{"x": ""}}); got != "os:x" {
		t.Errorf("an entry with an EMPTY login resolved to %q — an empty identity is not an identity", got)
	}
}

// TestOverrideIdentityResolvesRosterLogin is the end-to-end proof through the REAL
// loader: a roster binding this machine's OS account to a roster login makes
// OverrideIdentity name the login, not the account.
func TestOverrideIdentityResolvesRosterLogin(t *testing.T) {
	u, err := user.Current()
	if err != nil || u.Username == "" || strings.ContainsAny(u.Username, ":,; \t\n\r") {
		t.Skipf("this platform's OS username (%q, %v) cannot be expressed in the map", u.Username, err)
	}
	t.Setenv("DESK_SESSION", "override-identity-test")

	r := goldenRoster()
	r[EnvOSUserLoginMap] = u.Username + ":ada"
	withRoster(t, r)

	if got, want := OverrideIdentity(), "ada/override-identity-test"; got != want {
		t.Errorf("OverrideIdentity() = %q, want %q — the audit row must name the roster login, not the OS account", got, want)
	}
}

// TestOverrideIdentityFallbackIsMarked pins the contract for an UNMAPPED account: a
// row still lands, and its identity cannot be mistaken for a resolved roster login.
func TestOverrideIdentityFallbackIsMarked(t *testing.T) {
	u, err := user.Current()
	if err != nil || u.Username == "" {
		t.Skipf("no OS username on this platform: %v", err)
	}
	t.Setenv("DESK_SESSION", "override-identity-test")

	// The golden roster carries no OS-account binding. Unless this machine's account
	// name collides with a fixture identity, the identity must be the marked fallback.
	withRoster(t, goldenRoster())
	want := "os:" + u.Username
	switch strings.ToLower(u.Username) {
	case "ada", "shared-agent":
		want = strings.ToLower(u.Username)
	}
	if got := OverrideIdentity(); got != want+"/override-identity-test" {
		t.Errorf("OverrideIdentity() = %q, want %q", got, want+"/override-identity-test")
	}
}

// TestOSUserLoginMapMalformedDegradesNotRefuses: the map is an EXTENSION key — a
// malformed value is recorded against that one key and the map reads as EMPTY, the
// roster itself still loads, and identity resolution still produces a (marked,
// fallback) row. The whole-roster refusal is reserved for keys that decide who acts.
func TestOSUserLoginMapMalformedDegradesNotRefuses(t *testing.T) {
	extName, ok := ExtKeyName(EnvOSUserLoginMap)
	if !ok {
		t.Fatal("ASSAY_OS_USER_LOGIN_MAP is not catalogued as an extension key")
	}

	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"no separator", "laptop-acct"},
		{"empty OS account", ":ada"},
		{"empty login", "laptop-acct:"},
		{"a bot-shaped login", "laptop-acct:some-bot"},
		{"an OS account mapped twice", "laptop-acct:ada,laptop-acct:shared-agent"},
		{"one bad entry poisons the whole map, never a partial one", "laptop-acct:ada,broken"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := goldenRoster()
			r[EnvOSUserLoginMap] = tc.raw
			withRoster(t, r)

			cfg := EffectiveConfig()
			if !cfg.Configured() {
				t.Fatalf("%s=%q collapsed the WHOLE roster — an audit-label key degrades its own feature, never the configuration", EnvOSUserLoginMap, tc.raw)
			}
			if len(cfg.OSUserLogins) != 0 {
				t.Errorf("%s=%q produced a partial map %v — a half-applied attribution table is worse than none", EnvOSUserLoginMap, tc.raw, cfg.OSUserLogins)
			}
			res, ok := cfg.Ext[extName]
			if !ok || res.Status != ExtInvalid {
				t.Errorf("%s=%q recorded Ext %v — the malformed value must be visible against its own key", EnvOSUserLoginMap, tc.raw, res)
			}
			// The audit row still has an identity: resolution fell back, it did not block.
			if id := OverrideIdentity(); id == "" || !strings.HasSuffix(id, "/"+SessionTag()) {
				t.Errorf("OverrideIdentity() = %q under a malformed map — the audit row must always carry an identity", id)
			}
		})
	}
}

// TestOSUserLoginMapParsesAndEchoes: a well-formed map loads lowercased, survives
// case-insensitively, and appears in the run echo — a misattributing edit to an
// accountability table must never be invisible.
func TestOSUserLoginMapParsesAndEchoes(t *testing.T) {
	r := goldenRoster()
	r[EnvOSUserLoginMap] = "Laptop-Acct:ADA, desktop-acct:shared-agent"
	withRoster(t, r)

	cfg := EffectiveConfig()
	if !cfg.Configured() {
		t.Fatalf("a well-formed %s refused the roster", EnvOSUserLoginMap)
	}
	if got := cfg.OSUserLogins["laptop-acct"]; got != "ada" {
		t.Errorf("OSUserLogins[laptop-acct] = %q, want %q (keys and values normalise to lower case)", got, "ada")
	}
	if got := cfg.OSUserLogins["desktop-acct"]; got != "shared-agent" {
		t.Errorf("OSUserLogins[desktop-acct] = %q, want %q", got, "shared-agent")
	}
	if res := cfg.Ext["os-user-login-map"]; res.Status != ExtOK {
		t.Errorf("a well-formed map recorded Ext %v, want OK", res)
	}

	var echo strings.Builder
	EchoEffectiveConfig(&echo)
	if !strings.Contains(echo.String(), EnvOSUserLoginMap+"=desktop-acct=shared-agent,laptop-acct=ada") {
		t.Errorf("the run echo does not carry the sorted %s surface:\n%s", EnvOSUserLoginMap, echo.String())
	}
}
