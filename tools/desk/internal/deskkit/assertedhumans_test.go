package deskkit

import (
	"strings"
	"testing"
)

// The pinned human-name assertion (ASSAY_ASSERTED_HUMANS) and the dropped-keys echo.
//
// A login that ends in -bot or -app trips the loader's name-shape check. The assertion
// is the ONLY way such a login loads as a human, and it is bound to one exact login:id
// pair. Every other refusal stays. The key is spelled as a literal here on purpose: the
// tests compile on a tree that does not yet know the key, so they can be run red first.

const assertKey = "ASSAY_ASSERTED_HUMANS"

// assertBase is a minimal configured roster with a valid repo, a forge binding and one
// App, so a refusal visibly discards keys that were themselves well-formed.
func assertBase(extra map[string]string) map[string]string {
	v := map[string]string{
		EnvBlessLogin:      "ada:1001",
		EnvTrustedLogins:   "ada:1001",
		EnvTrustedBotSlugs: "reviewer=example-reviewer-app:3003",
		EnvAllowedRepos:    "example-org/tracked:ci:private",
		EnvRepoForges:      "example-org/tracked=github",
	}
	for k, val := range extra {
		v[k] = val
	}
	return v
}

func assertParse(vals map[string]string) Config {
	return parseConfig(ClassWrite, "test", vals)
}

func hasProblem(c Config, want string) bool {
	for _, p := range c.Problems {
		if strings.Contains(p, want) {
			return true
		}
	}
	return false
}

// regression: #4817
func TestAssertNoneStillRefused(t *testing.T) {
	c := assertParse(assertBase(map[string]string{EnvTrustedLogins: "ada:1001,ops-bot:5005"}))
	if c.Configured() {
		t.Fatal("a -bot login with no assertion loaded — the name-shape control was lost")
	}
	if !hasProblem(c, "renders as a bot/App account") {
		t.Fatalf("refusal lost its reason: %q", c.Problems)
	}
}

// regression: #4817
func TestAssertPinnedLoads(t *testing.T) {
	c := assertParse(assertBase(map[string]string{
		EnvTrustedLogins: "ada:1001,ops-bot:5005",
		assertKey:        "ops-bot:5005",
	}))
	if !c.Configured() {
		t.Fatalf("a pinned assertion matching the trusted entry was refused: %q", c.Problems)
	}
	if got := c.Humans["ops-bot"]; got != 5005 {
		t.Fatalf("asserted human not pinned: Humans[ops-bot]=%d", got)
	}
	if !c.Logins["ops-bot"] {
		t.Fatal("asserted human not in the login set")
	}
}

// regression: #4817
func TestAssertBlessPinnedLoads(t *testing.T) {
	c := assertParse(assertBase(map[string]string{
		EnvBlessLogin:    "ops-bot:5005",
		EnvTrustedLogins: "ops-bot:5005",
		assertKey:        "ops-bot:5005",
	}))
	if !c.Configured() {
		t.Fatalf("a pinned assertion for the bless login was refused: %q", c.Problems)
	}
	if c.Bless.Login != "ops-bot" || c.Bless.ID != 5005 {
		t.Fatalf("bless identity = %+v", c.Bless)
	}
}

// regression: #4817
func TestAssertCaseVariantsLoad(t *testing.T) {
	c := assertParse(assertBase(map[string]string{
		EnvTrustedLogins: "ada:1001,Ops-Bot:5005",
		assertKey:        "@OPS-BOT:5005",
	}))
	if !c.Configured() {
		t.Fatalf("a case/@ variant of the SAME pair was refused: %q", c.Problems)
	}
	if c.Humans["ops-bot"] != 5005 {
		t.Fatalf("case variant not normalised: %v", c.Humans)
	}
}

// regression: #4817
func TestAssertRefusals(t *testing.T) {
	cases := []struct {
		why   string
		extra map[string]string
		want  string
	}{
		{"same login, another id in the trusted entry",
			map[string]string{EnvTrustedLogins: "ada:1001,ops-bot:5006", assertKey: "ops-bot:5005"},
			"no pinned " + assertKey + " entry for that exact login:id"},
		{"bare trusted entry: the waiver needs the pinned pair",
			map[string]string{EnvTrustedLogins: "ada:1001,ops-bot", assertKey: "ops-bot:5005"},
			"no pinned " + assertKey + " entry for that exact login:id"},
		{"bless with another id",
			map[string]string{EnvBlessLogin: "ops-bot:5006", EnvTrustedLogins: "ops-bot:5006", assertKey: "ops-bot:5005"},
			"no pinned " + assertKey + " entry for that exact login:id"},
		{"assertion for a different login",
			map[string]string{EnvTrustedLogins: "ada:1001,ops-bot:5005", assertKey: "relay-bot:5005"},
			"no pinned " + assertKey + " entry for that exact login:id"},
		{"bare login assertion",
			map[string]string{EnvTrustedLogins: "ada:1001,ops-bot:5005", assertKey: "ops-bot"},
			"is not a pinned login:id pair"},
		{"non-numeric id",
			map[string]string{EnvTrustedLogins: "ada:1001,ops-bot:5005", assertKey: "ops-bot:abc"},
			"is not a pinned login:id pair"},
		{"zero id",
			map[string]string{EnvTrustedLogins: "ada:1001,ops-bot:5005", assertKey: "ops-bot:0"},
			"is not a pinned login:id pair"},
		{"forge-structural [bot] rendering",
			map[string]string{assertKey: "ops[bot]:5005"},
			"forge-structural"},
		{"forge-structural app/ rendering",
			map[string]string{assertKey: "app/ops:5005"},
			"forge-structural"},
		{"duplicate assertion, same pair",
			map[string]string{EnvTrustedLogins: "ada:1001,ops-bot:5005", assertKey: "ops-bot:5005,OPS-BOT:5005"},
			"asserted more than once"},
		{"one id asserted for two logins",
			map[string]string{EnvTrustedLogins: "ada:1001,ops-bot:5005,relay-bot:5005", assertKey: "ops-bot:5005,relay-bot:5005"},
			"one forge id is one account"},
		{"a login the name check never refused",
			map[string]string{assertKey: "plain:5005"},
			"asserts nothing"},
		{"a login registered as a bot account",
			map[string]string{EnvTrustedBotSlugs: "reviewer=example-reviewer-app:3003,worker=gitlab:ops-bot:5005", assertKey: "ops-bot:5005"},
			"registers as an App/bot account"},
		{"an assertion no entry uses",
			map[string]string{assertKey: "ops-bot:5005"},
			"is asserted but no"},
	}
	for _, tc := range cases {
		c := assertParse(assertBase(tc.extra))
		if c.Configured() {
			t.Errorf("%s: loaded — want refused", tc.why)
			continue
		}
		if !hasProblem(c, tc.want) {
			t.Errorf("%s: problems %q do not name %q", tc.why, c.Problems, tc.want)
		}
	}
}

// regression: #4817
func TestAssertOtherKeysStillRefuse(t *testing.T) {
	// The waiver covers ASSAY_TRUSTED_LOGINS and ASSAY_BLESS_LOGIN only. The mapped
	// accountable-human set and the stamp allowance keep their own refusal.
	for _, extra := range []map[string]string{
		{EnvTrustedLogins: "ada:1001,ops-bot:5005", assertKey: "ops-bot:5005", EnvHumanLoginMap: "ops:ops-bot"},
		{EnvTrustedLogins: "ada:1001,ops-bot:5005", assertKey: "ops-bot:5005", EnvStampTrustedLogins: "ops-bot:5005"},
	} {
		if c := assertParse(assertBase(extra)); c.Configured() {
			t.Errorf("an assertion widened another key: %v", extra)
		}
	}
}

// regression: #4817
func TestAssertEchoed(t *testing.T) {
	c := assertParse(assertBase(map[string]string{
		EnvBlessLogin:    "ops-bot:5005",
		EnvTrustedLogins: "ops-bot:5005",
		assertKey:        "ops-bot:5005",
	}))
	echo := strings.Join(c.EffectiveConfigLines(), "\n")
	want := "assay-config: " + assertKey + "=ops-bot:5005 (name check waived for " +
		EnvBlessLogin + "," + EnvTrustedLogins + ")"
	if !strings.Contains(echo, want) {
		t.Fatalf("assertion use not in the echo (and so not in its digest):\nwant line %q\n%s", want, echo)
	}
	if line := compactConfigLine(c, ConfigDigest(c.EffectiveConfigLines())); !strings.Contains(line, " asserted=1 ") {
		t.Fatalf("compact echo does not count the assertion: %q", line)
	}
	// A roster without the key echoes exactly as before: no line, so no digest change.
	plain := strings.Join(assertParse(assertBase(nil)).EffectiveConfigLines(), "\n")
	if strings.Contains(plain, assertKey) {
		t.Fatalf("a roster without the key echoes it:\n%s", plain)
	}
}

// regression: #4817
func TestAssertDroppedEcho(t *testing.T) {
	c := assertParse(assertBase(map[string]string{EnvTrustedLogins: "ada:1001,ops-bot:5005"}))
	if c.Configured() || len(c.Repos) != 0 {
		t.Fatal("a refused roster partially loaded")
	}
	echo := strings.Join(c.EffectiveConfigLines(), "\n")
	for _, want := range []string{
		"assay-config: DROPPED " + EnvAllowedRepos + " — not applied: discarded with the refused roster",
		"assay-config: DROPPED " + EnvRepoForges + " — not applied: discarded with the refused roster",
		"assay-config: DROPPED " + EnvTrustedLogins + " — refused: its own value is named in a REFUSED line",
	} {
		if !strings.Contains(echo, want) {
			t.Errorf("echo missing %q:\n%s", want, echo)
		}
	}
	if strings.Contains(echo, "assay-config: DROPPED "+EnvRiskCallout) {
		t.Error("an absent key was reported dropped")
	}
}

// regression: #4817
func TestAssertForgeBotRefused(t *testing.T) {
	withRoster(t, assertBase(map[string]string{
		EnvBlessLogin:    "ops-bot:5005",
		EnvTrustedLogins: "ops-bot:5005",
		assertKey:        "ops-bot:5005",
	}))
	note := func(bot bool) *glGQLNoteable {
		return &glGQLNoteable{Comments: glGQLNoteConn{Nodes: []glGQLNote{{
			Body:      "bless",
			CreatedAt: "2026-10-01T00:00:00Z",
			Author:    &glGQLActor{ID: "gid://gitlab/User/5005", Username: "ops-bot", Bot: bot},
		}}}}
	}
	blessed := func(bot bool) bool {
		gi, _, err := gitlabTrustItem(note(bot))
		if err != nil {
			t.Fatal(err)
		}
		body, events, _, err := collectEvents(gi, false)
		if err != nil {
			t.Fatal(err)
		}
		return Blessed(body, events)
	}
	if !blessed(false) {
		t.Fatal("the asserted human (forge bot=false) could not bless")
	}
	if blessed(true) {
		t.Fatal("a forge-marked bot blessed through the name assertion")
	}
	if !TrustedAuthorID("ops-bot", 5005) || TrustedAuthorID("ops-bot", 5006) {
		t.Fatal("asserted pair is not id-bound")
	}
}
