package trust

import (
	"strings"
	"testing"
)

// regression: #4817
func TestNameShapeBot(t *testing.T) {
	for login, want := range map[string]bool{
		"ops-bot": true, "Ops-App": true, " ops-bot ": true,
		"ops[bot]": false, "app/ops": false, "app/ops-bot": false, "ops-bot[bot]": false,
		"ops": false, "robot": false,
	} {
		if got := NameShapeBot(login); got != want {
			t.Errorf("NameShapeBot(%q) = %t, want %t", login, got, want)
		}
	}
}

// regression: #4817
func TestWaivesPinnedPairOnly(t *testing.T) {
	a, probs := ParseHumanAssertions("ops-bot:5005", nil)
	if len(probs) != 0 {
		t.Fatalf("problems: %q", probs)
	}
	if !a.Waives("ops-bot", 5005) || !a.Waives("@OPS-BOT", 5005) {
		t.Fatal("the pinned pair is not waived")
	}
	for _, c := range []struct {
		login string
		id    int64
	}{{"ops-bot", 0}, {"ops-bot", 5006}, {"relay-bot", 5005}, {"ops-bot[bot]", 5005}} {
		if a.Waives(c.login, c.id) {
			t.Errorf("Waives(%q, %d) = true — only the exact pinned pair may pass", c.login, c.id)
		}
	}
	var none HumanAssertions
	if none.Waives("ops-bot", 5005) {
		t.Fatal("an empty set waived a login")
	}
}

// regression: #4817
func TestParseAssertRefusals(t *testing.T) {
	for raw, want := range map[string]string{
		"ops-bot":                     "is not a pinned login:id pair",
		"ops-bot:-1":                  "is not a pinned login:id pair",
		"ops[bot]:5005":               "forge-structural",
		"app/ops-bot:5005":            "forge-structural",
		"ops:5005":                    "asserts nothing",
		"bot-ops-bot:7":               "registers as an App/bot account",
		"ops-bot:5005,ops-bot:5005":   "asserted more than once",
		"ops-bot:5005,relay-app:5005": "one forge id is one account",
	} {
		_, probs := ParseHumanAssertions(raw, map[string]bool{"bot-ops-bot": true})
		if len(probs) == 0 || !strings.Contains(strings.Join(probs, "\n"), want) {
			t.Errorf("%q: problems %q do not name %q", raw, probs, want)
		}
		for _, p := range probs {
			if !strings.HasPrefix(p, HumanAssertionsKey+": ") {
				t.Errorf("problem %q does not start with the key", p)
			}
		}
	}
}

// regression: #4817
func TestAssertionOverruled(t *testing.T) {
	a := HumanAssertions{"ops-bot": 5005}
	if !AssertionOverruled(a, "Ops-Bot", true) {
		t.Fatal("a forge-marked bot presenting an asserted login was not overruled")
	}
	if AssertionOverruled(a, "ops-bot", false) || AssertionOverruled(a, "other", true) || AssertionOverruled(nil, "ops-bot", true) {
		t.Fatal("overruled outside its one case")
	}
	for kind, want := range map[string]bool{"Bot": true, "GitLabBot": true, "bot": true, "User": false, "": false} {
		if ForgeMarkedBot(kind) != want {
			t.Errorf("ForgeMarkedBot(%q) != %t", kind, want)
		}
	}
}

// regression: #4817
func TestDroppedKeysNamesDiscards(t *testing.T) {
	vals := map[string]string{
		"ASSAY_TRUSTED_LOGINS": "ops-bot:5005", "ASSAY_ALLOWED_REPOS": "o/r", "ASSAY_RISK_CALLOUT": " ",
		"OTHER_KEY": "x",
	}
	if DroppedKeys(vals, nil) != nil {
		t.Fatal("a configured roster reported dropped keys")
	}
	got := strings.Join(DroppedKeys(vals, []string{`ASSAY_TRUSTED_LOGINS lists "ops-bot", which renders as a bot`}), "\n")
	for _, want := range []string{
		"ASSAY_ALLOWED_REPOS — not applied: discarded with the refused roster",
		"ASSAY_TRUSTED_LOGINS — refused: its own value is named in a REFUSED line",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "ASSAY_RISK_CALLOUT") || strings.Contains(got, "OTHER_KEY") {
		t.Errorf("an empty or non-ASSAY_ key was reported:\n%s", got)
	}
}
