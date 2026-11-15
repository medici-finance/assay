package deskkit

import (
	"strings"
	"testing"
)

// The unknown-ASSAY_-key behaviour (#1261): a key in the ASSAY_ namespace
// that this version does not recognise records a NOTICE naming it and does NOT refuse the
// roster. Both sides are pinned here: the NOTICE (and that the roster loads), and that every
// refusal a RECOGNISED key's bad value or a missing trust surface produces still fires.

const unknownExampleKey = "ASSAY_UNKNOWN_EXAMPLE_KEY"

// regression: #1261
func TestUnknownAssayKeyNotice(t *testing.T) {
	quietEchoEnv(t)
	vals := goldenRoster()
	vals[unknownExampleKey] = "1"
	withRoster(t, vals)
	c := EffectiveConfig()

	if !c.Configured() {
		t.Fatalf("an unknown ASSAY_ key refused the roster; it must be a NOTICE only.\n  problems: %v",
			c.Problems)
	}
	if len(c.Problems) != 0 {
		t.Fatalf("an unknown ASSAY_ key recorded problems: %v", c.Problems)
	}
	var named int
	for _, n := range c.Notices {
		if strings.HasPrefix(n, unknownExampleKey+": unknown key in the ASSAY_ namespace") {
			named++
		}
	}
	if named != 1 {
		t.Fatalf("want exactly one NOTICE naming %s, got %d: %v", unknownExampleKey, named, c.Notices)
	}
	for _, k := range c.UnknownKeys {
		if k == unknownExampleKey {
			t.Fatalf("%s was echoed as a co-tenant key; an ASSAY_ key must be a NOTICE", k)
		}
	}
	// The roster's own trust still loaded in full.
	if !TrustedAuthor("ada") {
		t.Fatal("the trusted login did not load alongside the unknown key")
	}

	out := echoOnce(t)
	if !isFullEcho(out) {
		t.Fatalf("an unknown-key NOTICE must force the full echo:\n%s", out)
	}
	if !strings.Contains(out, "assay-config: NOTICE — "+unknownExampleKey+": unknown key") {
		t.Fatalf("the echo carries no NOTICE line naming %s:\n%s", unknownExampleKey, out)
	}
	if strings.Contains(out, "REFUSED") {
		t.Fatalf("the echo carries a REFUSED line for an unknown key alone:\n%s", out)
	}
}

// regression: #1261
func TestUnknownKeyKnownRefusals(t *testing.T) {
	for _, tc := range []struct {
		name     string
		edit     func(map[string]string)
		problem  string // "" = no Problems line, but the trust surface is still unconfigured
		trustSet bool
	}{
		{"bless login without an id", func(v map[string]string) { v[EnvBlessLogin] = "ada" },
			"carries no numeric id", true},
		{"malformed allowed-repos entry", func(v map[string]string) { v[EnvAllowedRepos] = "ex*mple-org/*" },
			"may not itself contain '*'", true},
		{"unsupported schema version", func(v map[string]string) { v[EnvRosterSchema] = "999" },
			"schema version", true},
		{"misspelt bless-login key", func(v map[string]string) {
			v["ASSAY_BLESS_LOGN"] = v[EnvBlessLogin]
			delete(v, EnvBlessLogin)
		}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vals := goldenRoster()
			vals[unknownExampleKey] = "1"
			tc.edit(vals)
			withRoster(t, vals)
			c := EffectiveConfig()
			if c.Configured() {
				t.Fatalf("the roster loaded as configured; the known-key refusal must still fire.\n"+
					"  problems: %v\n  notices: %v", c.Problems, c.Notices)
			}
			if tc.problem != "" && !strings.Contains(strings.Join(c.Problems, "\n"), tc.problem) {
				t.Fatalf("the refusal does not name %q:\n  problems: %v", tc.problem, c.Problems)
			}
			if !tc.trustSet && TrustedAuthor("ada") {
				t.Fatal("a misspelt trust key left a login trusted; unset trust must stay closed")
			}
			if !strings.Contains(strings.Join(c.Notices, "\n"), unknownExampleKey+": unknown key") {
				t.Fatalf("the unknown key's NOTICE was lost on a refused roster: %v", c.Notices)
			}
		})
	}

	// A misspelt trusted-logins key: the bless login is still trusted (it always is), but the
	// login the misspelt key meant to add stays untrusted — the unset surface fails closed.
	vals := goldenRoster()
	vals["ASSAY_TRUSTED_LOGINZ"] = vals[EnvTrustedLogins]
	delete(vals, EnvTrustedLogins)
	withRoster(t, vals)
	if TrustedAuthor("shared-agent") {
		t.Fatal("a misspelt trusted-logins key trusted its login; the key must not be applied")
	}
	if !strings.Contains(strings.Join(EffectiveConfig().Notices, "\n"), "ASSAY_TRUSTED_LOGINZ: unknown key") {
		t.Fatal("the misspelt trusted-logins key drew no NOTICE")
	}
}
