package main

import (
	"strings"
	"testing"
)

// statusgen's twin of deskkit's loginmapspace_test.go: a name:login entry whose name half
// carries a space is refused on its own, with a NOTICE naming it, in BOTH name:login maps
// (ASSAY_HUMAN_LOGIN_MAP and ASSAY_FORMER_HUMAN_LOGIN_MAP) — it no longer refuses the whole
// roster, and it yields no mapping. The assertions read only Configured, the two maps and
// the echo, so the test compiles, and runs red, on a tree without the fix.

func scanSpacedBase(key, raw string) map[string]string {
	return map[string]string{
		scanEnvBlessLogin:      "grace:1001",
		scanEnvTrustedLogins:   "grace:1001",
		scanEnvTrustedBotSlugs: "reviewer=example-reviewer-app:3003",
		scanEnvAllowedRepos:    "example-org/tracked:ci:private",
		scanEnvRepoForges:      "example-org/tracked=github",
		key:                    raw,
	}
}

// regression: #4978
func TestScanLoginMapSpacedName(t *testing.T) {
	for _, key := range []string{scanEnvHumanLoginMap, scanEnvFormerHumanLoginMap} {
		c := scanParseConfig(scanClassReadOnly, "test",
			scanSpacedBase(key, "grace:grace, Ada Lovelace:ada-l, linus:linus"))
		if !c.Configured() {
			t.Errorf("%s: one spaced-name entry refused the whole roster: %q", key, c.Problems)
			continue
		}
		m := c.HumanLogins
		if key == scanEnvFormerHumanLoginMap {
			m = c.FormerHumanLogins
		}
		if m["grace"] != "grace" || m["linus"] != "linus" || len(m) != 2 {
			t.Errorf("%s: want exactly the two well-formed entries, got %v", key, m)
		}
		if c.Bless.Login != "grace" {
			t.Errorf("%s: other keys dropped: bless=%+v", key, c.Bless)
		}
		echo := strings.Join(c.EffectiveConfigLines(), "\n")
		if !strings.Contains(echo, "NOTICE") || !strings.Contains(echo, `"Ada Lovelace:ada-l"`) {
			t.Errorf("%s: the refused entry is not named in a NOTICE line:\n%s", key, echo)
		}
	}
}

// regression: #4978
func TestScanLoginMapWellFormed(t *testing.T) {
	for _, key := range []string{scanEnvHumanLoginMap, scanEnvFormerHumanLoginMap} {
		for _, raw := range []string{"grace:grace,linus:linus", "grace:grace linus:linus", "grace:grace;\tlinus:linus"} {
			c := scanParseConfig(scanClassReadOnly, "test", scanSpacedBase(key, raw))
			if !c.Configured() {
				t.Errorf("%s=%q refused: %q", key, raw, c.Problems)
				continue
			}
			m := c.HumanLogins
			if key == scanEnvFormerHumanLoginMap {
				m = c.FormerHumanLogins
			}
			if len(m) != 2 || m["grace"] != "grace" || m["linus"] != "linus" {
				t.Errorf("%s=%q parsed to %v", key, raw, m)
			}
			if echo := strings.Join(c.EffectiveConfigLines(), "\n"); strings.Contains(echo, "NOTICE") {
				t.Errorf("%s=%q echoes a NOTICE:\n%s", key, raw, echo)
			}
		}
		for _, raw := range []string{"grace:grace, Ada Lovelace:ops-bot", "grace:grace, lovelace",
			"grace:grace, ada :ada-l", "grace:grace ada :ada-l", "grace:grace, ada\t:ada-l",
			"grace:grace, Ada Lovelace :ada-l"} {
			if c := scanParseConfig(scanClassReadOnly, "test", scanSpacedBase(key, raw)); c.Configured() {
				t.Errorf("%s=%q configured; want the whole roster refused", key, raw)
			}
		}
	}
}

// Twin of deskkit's TestLoginMapNoSilentMapping, for BOTH name:login maps: no shape that
// SplitList's split left inert may become a live mapping, and a non-ASCII space inside a
// name is refused with a NOTICE, never mapped.
//
// regression: #4978
func TestScanLoginMapNoSilentMap(t *testing.T) {
	for _, key := range []string{scanEnvHumanLoginMap, scanEnvFormerHumanLoginMap} {
		pick := func(c scanConfig) map[string]string {
			if key == scanEnvFormerHumanLoginMap {
				return c.FormerHumanLogins
			}
			return c.HumanLogins
		}
		for _, raw := range []string{
			"grace:grace bob:bob", "grace:grace bob:bob", "grace:grace\vbob:bob",
			"grace:grace, ada :ada-l", "grace:grace, ada\t:ada-l", "grace:grace ada :ada-l",
		} {
			for n, l := range pick(scanParseConfig(scanClassReadOnly, "test", scanSpacedBase(key, raw))) {
				if n == "bob" || n == "ada" || l == "bob" || l == "ada-l" {
					t.Errorf("%s=%q: mapped %q -> %q; that shape must not map", key, raw, n, l)
				}
			}
		}
		for _, raw := range []string{"grace:grace, Ada Lovelace:ada-l", "grace:grace, ada :ada-l"} {
			c := scanParseConfig(scanClassReadOnly, "test", scanSpacedBase(key, raw))
			if m := pick(c); !c.Configured() || len(m) != 1 || m["grace"] != "grace" {
				t.Errorf("%s=%q: want only grace mapped, configured; got %v %q", key, raw, m, c.Problems)
			}
			if echo := strings.Join(c.EffectiveConfigLines(), "\n"); !strings.Contains(echo, "NOTICE") {
				t.Errorf("%s=%q: the refused entry is not named in a NOTICE line:\n%s", key, raw, echo)
			}
		}
	}
}
