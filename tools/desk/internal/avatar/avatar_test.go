package avatar

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
)

func TestGenerateTeamNamesAndCount(t *testing.T) {
	set, err := Generate("example-org", TierTeam, Options{})
	if err != nil {
		t.Fatalf("team generate: %v", err)
	}
	if len(set) != 2 {
		t.Fatalf("team: want 2 tiles, got %d", len(set))
	}
	want := map[string]bool{"example-org-read": false, "example-org-act": false}
	for _, av := range set {
		if _, ok := want[av.App]; !ok {
			t.Errorf("unexpected app %q", av.App)
		}
		want[av.App] = true
		if len(av.SVG) == 0 {
			t.Errorf("%s: empty SVG", av.App)
		}
		if _, err := png.Decode(bytes.NewReader(av.PNGs[512])); err != nil {
			t.Errorf("%s: 512px PNG does not decode: %v", av.App, err)
		}
	}
	for app, seen := range want {
		if !seen {
			t.Errorf("missing tile %q", app)
		}
	}
}

func TestGenerateFamilyCount(t *testing.T) {
	set, err := Generate("example-org", TierFamily, Options{})
	if err != nil {
		t.Fatalf("family generate: %v", err)
	}
	if len(set) != 6 {
		t.Fatalf("family: want 6 tiles, got %d", len(set))
	}
	roles := map[string]bool{}
	for _, av := range set {
		roles[av.Role] = true
		if _, err := png.Decode(bytes.NewReader(av.PNGs[512])); err != nil {
			t.Errorf("%s: 512px PNG does not decode: %v", av.App, err)
		}
	}
	for _, want := range []string{"reviewer", "worker", "verifier", "desk", "issue-loop", "intake-loop"} {
		if !roles[want] {
			t.Errorf("family missing role %q", want)
		}
	}
}

func TestGenerateDeterministic(t *testing.T) {
	// Verify row 4 at the library level: two runs are byte-identical.
	for _, tier := range []Tier{TierTeam, TierFamily} {
		a, err := Generate("example-org", tier, Options{Sizes: []int{200, 512}})
		if err != nil {
			t.Fatalf("%s run a: %v", tier, err)
		}
		b, err := Generate("example-org", tier, Options{Sizes: []int{200, 512}})
		if err != nil {
			t.Fatalf("%s run b: %v", tier, err)
		}
		if len(a) != len(b) {
			t.Fatalf("%s: tile count differs", tier)
		}
		for i := range a {
			if a[i].App != b[i].App {
				t.Fatalf("%s: order differs at %d", tier, i)
			}
			if !bytes.Equal(a[i].SVG, b[i].SVG) {
				t.Errorf("%s/%s: SVG not deterministic", tier, a[i].App)
			}
			for _, sz := range []int{200, 512} {
				if !bytes.Equal(a[i].PNGs[sz], b[i].PNGs[sz]) {
					t.Errorf("%s/%s @%dpx: PNG not deterministic", tier, a[i].App, sz)
				}
			}
		}
	}
}

func TestGenerateRejectsInvalidOrg(t *testing.T) {
	// The org flows into output file stems, so a login-shaped value is required —
	// a path-traversal or separator-bearing org must be refused, never rendered.
	for _, bad := range []string{"../evil", "a/b", "-leading", "has space", "under_score", "dot.org", strings.Repeat("x", 40)} {
		if _, err := Generate(bad, TierTeam, Options{}); err == nil {
			t.Errorf("Generate(%q) should be rejected as a non-login org", bad)
		}
	}
	for _, ok := range []string{"example-org", "medici-finance", "a", "ACME", strings.Repeat("x", 39)} {
		if _, err := Generate(ok, TierTeam, Options{}); err != nil {
			t.Errorf("Generate(%q) should be accepted: %v", ok, err)
		}
	}
}

func TestGenerateRejectsBadSize(t *testing.T) {
	for _, sz := range []int{0, -1, MaxRenderSize + 1} {
		if _, err := Generate("example-org", TierTeam, Options{Sizes: []int{sz}}); err == nil {
			t.Errorf("Generate with size %d should be rejected", sz)
		}
	}
	if _, err := Generate("example-org", TierTeam, Options{Sizes: []int{MaxRenderSize}}); err != nil {
		t.Errorf("Generate at the max size %d should be accepted: %v", MaxRenderSize, err)
	}
}

func TestGenerateUnknownTier(t *testing.T) {
	if _, err := Generate("example-org", Tier("bogus"), Options{}); err == nil {
		t.Fatal("want error for unknown tier, got nil")
	}
}

func TestGenerateEmptyOrg(t *testing.T) {
	if _, err := Generate("  ", TierTeam, Options{}); err == nil {
		t.Fatal("want error for empty org, got nil")
	}
}

func TestInitialsOf(t *testing.T) {
	cases := map[string]string{
		"example-org":    "EO",
		"acme":           "A",
		"medici-finance": "MF",
		"hello world co": "HW",
		"one_two_three":  "OT",
		"":               "",
	}
	for in, want := range cases {
		if got := initialsOf(in); got != want {
			t.Errorf("initialsOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSeedHueDeterministic(t *testing.T) {
	if seedHue("example-org") != seedHue("example-org") {
		t.Fatal("seedHue not stable")
	}
	if seedHue("a") == seedHue("b") {
		t.Log("note: two logins share a hue; that is allowed, only determinism is required")
	}
}
