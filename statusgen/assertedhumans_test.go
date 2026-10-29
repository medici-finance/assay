package main

import (
	"strings"
	"testing"
	"time"
)

// statusgen's half of the pinned human-name assertion. The two config readers must agree
// on every verdict; the shared vector (testdata/roster_coupling.json) binds the common
// rows, and these pin the statusgen-only runtime arm (the R-7 direct-author predicate,
// which is handed the forge's own actor kind).

const scanAssertKey = "ASSAY_ASSERTED_HUMANS"

func scanAssertBase(extra map[string]string) map[string]string {
	v := map[string]string{
		scanEnvBlessLogin:      "ops-bot:5005",
		scanEnvTrustedLogins:   "ops-bot:5005",
		scanEnvTrustedBotSlugs: "reviewer=example-reviewer-app:3003",
		scanEnvAllowedRepos:    "example-org/tracked:ci:private",
		scanEnvRepoForges:      "example-org/tracked=github",
		scanAssertKey:          "ops-bot:5005",
	}
	for k, val := range extra {
		v[k] = val
	}
	return v
}

// regression: #4817
func TestScanAssertPinnedLoads(t *testing.T) {
	c := scanParseConfig(scanClassReadOnly, "test", scanAssertBase(nil))
	if !c.Configured() {
		t.Fatalf("a pinned assertion was refused: %q", c.Problems)
	}
	if c.Bless.Login != "ops-bot" || c.Bless.ID != 5005 {
		t.Fatalf("bless = %+v", c.Bless)
	}
	echo := strings.Join(c.EffectiveConfigLines(), "\n")
	if !strings.Contains(echo, "assay-config: "+scanAssertKey+"=ops-bot:5005 (name check waived for ") {
		t.Fatalf("assertion use not echoed:\n%s", echo)
	}
}

// regression: #4817
func TestScanAssertWrongIDRefused(t *testing.T) {
	for _, extra := range []map[string]string{
		{scanAssertKey: "ops-bot:5006"},
		{scanAssertKey: "ops-bot"},
		{scanAssertKey: "relay-bot:5005"},
		{scanAssertKey: "ops[bot]:5005"},
		{scanAssertKey: ""},
	} {
		c := scanParseConfig(scanClassReadOnly, "test", scanAssertBase(extra))
		if c.Configured() {
			t.Errorf("loaded with %v — want refused", extra)
		}
	}
}

// regression: #4817
func TestScanAssertDroppedEcho(t *testing.T) {
	c := scanParseConfig(scanClassReadOnly, "test", scanAssertBase(map[string]string{scanAssertKey: ""}))
	if c.Configured() {
		t.Fatal("a -bot bless with no assertion loaded")
	}
	echo := strings.Join(c.EffectiveConfigLines(), "\n")
	for _, want := range []string{
		"assay-config: DROPPED " + scanEnvAllowedRepos + " — not applied: discarded with the refused roster",
		"assay-config: DROPPED " + scanEnvBlessLogin + " — refused: its own value is named in a REFUSED line",
	} {
		if !strings.Contains(echo, want) {
			t.Errorf("echo missing %q:\n%s", want, echo)
		}
	}
}

// regression: #4817
func TestScanAssertForgeKindWins(t *testing.T) {
	scanWithRoster(t, scanAssertBase(nil))
	if !authorizedByIdentity("ops-bot", 5005, "User") {
		t.Fatal("the asserted bless human (forge kind User) is not an authorized author")
	}
	if authorizedByIdentity("ops-bot", 5005, "Bot") {
		t.Fatal("a forge-marked bot was authorized through the name assertion")
	}
	if authorizedByIdentity("ops-bot", 5006, "User") {
		t.Fatal("the asserted login with another id was authorized")
	}
}

// regression: #4817
func TestScanAssertEventBotWins(t *testing.T) {
	scanWithRoster(t, scanAssertBase(nil))
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	blessed := func(bot bool) bool {
		ok, err := evalBlessingEvents(time.Time{}, []blessEvent{{login: "ops-bot", id: 5005, forgeBot: bot, created: at}}, true)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if !blessed(false) {
		t.Fatal("the asserted bless human could not bless")
	}
	if blessed(true) {
		t.Fatal("a forge-marked bot blessed through the name assertion")
	}
}
