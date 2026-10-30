package deskkit

import (
	"strings"
	"testing"
)

// ASSAY_HUMAN_LOGIN_MAP entries whose NAME half carries a space (a display name, which is
// what a GitLab account shows). The list splitter used to shred "Ada Lovelace:ada-l" into
// the colon-less word "Ada" — which refused the WHOLE roster — plus the fragment
// "Lovelace:ada-l", a mapping nobody wrote. Now that ONE entry is refused with a NOTICE
// naming it, it yields no mapping, and the rest of the roster stays configured. The
// assertions read only the public surface (Configured, HumanLogins, the echo) so the tests
// compile, and run red, on a tree without the fix.

const spacedEntry = "Ada Lovelace:ada-l"

func spacedMapBase(humanMap string) map[string]string {
	return map[string]string{
		EnvBlessLogin:      "grace:1001",
		EnvTrustedLogins:   "grace:1001,ada-l:1002",
		EnvTrustedBotSlugs: "reviewer=example-reviewer-app:3003",
		EnvAllowedRepos:    "example-org/tracked:ci:private",
		EnvRepoForges:      "example-org/tracked=github",
		EnvHumanLoginMap:   humanMap,
	}
}

// regression: #4978
func TestLoginMapSpacedNameOneEntry(t *testing.T) {
	c := parseConfig(ClassWrite, "test", spacedMapBase("grace:grace, "+spacedEntry+", linus:linus"))
	if !c.Configured() {
		t.Fatalf("one spaced-name entry refused the whole roster: %q", c.Problems)
	}
	if got := c.HumanLogins["grace"]; got != "grace" {
		t.Errorf("well-formed entry grace lost: HumanLogins = %v", c.HumanLogins)
	}
	if got := c.HumanLogins["linus"]; got != "linus" {
		t.Errorf("well-formed entry linus lost: HumanLogins = %v", c.HumanLogins)
	}
	for _, name := range []string{"ada lovelace", "ada", "lovelace", "ada-lovelace"} {
		if l, ok := c.HumanLogins[name]; ok {
			t.Errorf("spaced entry produced a mapping %q -> %q; want none", name, l)
		}
	}
	for n, l := range c.HumanLogins {
		if l == "ada-l" {
			t.Errorf("login ada-l is mapped from %q; the spaced entry must give that human NO mapping", n)
		}
	}
	if c.Bless.Login != "grace" || len(c.Repos) != 1 {
		t.Errorf("other keys dropped: bless=%+v repos=%v", c.Bless, c.Repos)
	}
	echo := strings.Join(c.EffectiveConfigLines(), "\n")
	if !strings.Contains(echo, "NOTICE") || !strings.Contains(echo, `"`+spacedEntry+`"`) {
		t.Errorf("the refused entry is not named in a NOTICE line:\n%s", echo)
	}
}

// Well-formed entries parse exactly as before, whatever separator the operator used —
// including a bare space between two entries.
//
// regression: #4978
func TestLoginMapWellFormedUnchanged(t *testing.T) {
	for _, raw := range []string{
		"grace:grace,linus:linus",
		"grace:grace linus:linus",
		"grace:grace;linus:linus",
		"grace:grace\nlinus:linus",
		" grace:grace ,\tlinus:linus ",
	} {
		c := parseConfig(ClassWrite, "test", spacedMapBase(raw))
		if !c.Configured() {
			t.Errorf("%q refused: %q", raw, c.Problems)
			continue
		}
		if len(c.HumanLogins) != 2 || c.HumanLogins["grace"] != "grace" || c.HumanLogins["linus"] != "linus" {
			t.Errorf("%q parsed to %v", raw, c.HumanLogins)
		}
		if echo := strings.Join(c.EffectiveConfigLines(), "\n"); strings.Contains(echo, "NOTICE") {
			t.Errorf("%q echoes a NOTICE:\n%s", raw, echo)
		}
	}
}

// The single-entry degrade never reaches the whole-roster refusals: an unparseable entry
// and a bot-shaped login still refuse the configuration, spaced name or not.
//
// regression: #4978
func TestLoginMapSpacedBotRefuses(t *testing.T) {
	for _, raw := range []string{
		"grace:grace, Ada Lovelace:ops-bot",
		"grace:grace, lovelace",
	} {
		c := parseConfig(ClassWrite, "test", spacedMapBase(raw))
		if c.Configured() {
			t.Errorf("%q: configured; want the whole roster refused", raw)
		}
	}
}

// spacedPrincipalRoster: the blessing authority's ONLY map entry has a spaced name, so
// that human has no neutral name. A public-target write needs the neutral name and must
// still refuse; a private-target write needs only the login and now resolves, because the
// roster is no longer refused wholesale.
const spacedPrincipalRoster = `ASSAY_BLESS_LOGIN=a-private-handle:2001
ASSAY_TRUSTED_LOGINS=a-private-handle:2001
ASSAY_HUMAN_LOGIN_MAP="Ada Lovelace:a-private-handle, grace:grace"
ASSAY_TRUSTED_BOT_SLUGS=worker=example-worker-app:300000006
ASSAY_ALLOWED_REPOS=example-org/pub:ci:public,example-org/one:ci:private
`

// regression: #4978
func TestLoginMapSpacedTrailerRefuses(t *testing.T) {
	plantRoster(t, spacedPrincipalRoster)
	t.Setenv("DESK_SESSION", "no-such-session-"+t.Name())

	out, err := AppendOnBehalfOf([]byte("The comment body."), "", publicFixtureRepo)
	if err == nil {
		t.Fatalf("AppendOnBehalfOf(%q) = %q; want a refusal — the human's only map entry was refused, "+
			"so there is no neutral name to stamp", publicFixtureRepo, string(out))
	}
	if !strings.Contains(err.Error(), "carries no name:login entry for that login") {
		t.Fatalf("AppendOnBehalfOf(%q) refused for the wrong reason: %v; want the no-neutral-name refusal",
			publicFixtureRepo, err)
	}
	if strings.Contains(string(out), "human:ada") {
		t.Fatalf("a refused spaced name reached the trailer: %q", string(out))
	}

	out, err = AppendOnBehalfOf([]byte("The comment body."), "", privateFixtureRepo)
	if err != nil {
		t.Fatalf("AppendOnBehalfOf(%q) = %v; want the login form — one refused map entry must not "+
			"refuse the rest of the roster", privateFixtureRepo, err)
	}
	if want := "On-behalf-of: human:" + fixtureLogin + " mode:unattended\n"; !strings.HasSuffix(string(out), want) {
		t.Fatalf("AppendOnBehalfOf(%q) = %q, want it to end with %q", privateFixtureRepo, string(out), want)
	}
}

// A NOTICE is never compacted away: a roster carrying a refused entry prints the full echo
// block on every run, the way a REFUSED or unrecognised-keys line does, so the refused entry
// stays visible after its digest has been seen once.
//
// regression: #4978
func TestLoginMapNoticeEchoFull(t *testing.T) {
	quietEchoEnv(t)
	vals := goldenRoster()
	vals[EnvHumanLoginMap] = "ada:ada," + spacedEntry
	withRoster(t, vals)
	for run := 1; run <= 2; run++ {
		out := echoOnce(t)
		if !isFullEcho(out) || !strings.Contains(out, "assay-config: NOTICE") {
			t.Fatalf("run %d: want the full block carrying the NOTICE line, got:\n%s", run, out)
		}
	}
}
