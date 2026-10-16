package avatar

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// teamProofOrgs are the org logins Verify row 2 names: two ordinary logins,
// the shortest legal login, and a 39-character one (the login grammar's max).
var teamProofOrgs = []string{"example", "acme", "a", "thirty-nine-character-org-login-example"}

// TestTeamThreeTileProof is apps-installer/10 Verify row 2: the three-tile team
// set (read, act, review) passes the 20 px proof through Generate — which runs
// the proof before returning anything — for every org login the row names.
//
// regression: F-apps-installer-10
func TestTeamThreeTileProof(t *testing.T) {
	if n := len(teamProofOrgs[3]); n != 39 {
		t.Fatalf("the long login is %d characters, want 39", n)
	}
	for _, org := range teamProofOrgs {
		set, err := Generate(org, TierTeam, Options{Sizes: []int{20}})
		if err != nil {
			t.Fatalf("org %q: three-tile team set should pass the 20 px proof: %v", org, err)
		}
		apps := make([]string, 0, len(set))
		for _, av := range set {
			apps = append(apps, av.App)
		}
		want := []string{org + "-read", org + "-act", org + "-review"}
		if strings.Join(apps, ",") != strings.Join(want, ",") {
			t.Errorf("org %q: tiles %v, want %v", org, apps, want)
		}
	}
}

// TestTeamProofEveryHue widens row 2 from four logins to the whole hue circle:
// the team hue is seedHue(org) — an integer in [0,360) — so a login per hue
// covers every team palette an adopter can get. Each set must pass the proof.
//
// regression: F-apps-installer-10
func TestTeamProofEveryHue(t *testing.T) {
	seen := map[float64]bool{}
	for i := 0; len(seen) < 360 && i < 1_000_000; i++ {
		org := fmt.Sprintf("o%d", i)
		h := seedHue(org)
		if seen[h] {
			continue
		}
		seen[h] = true
		specs, err := specsFor(org, TierTeam, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if len(specs) != 3 {
			t.Fatalf("org %q: want 3 team tiles, got %d", org, len(specs))
		}
		if err := runProof(specs); err != nil {
			t.Errorf("hue %v (org %q): %v", h, org, err)
		}
	}
	if len(seen) != 360 {
		t.Fatalf("covered %d of 360 hues — the sweep did not reach every team palette", len(seen))
	}
}

// TestTeamReadActUnchanged is Verify row 3: adding the review tile leaves the
// read and act tiles byte-identical. The goldens were written by deskavatar
// built from the mainline commit before the review tile existed
// (`deskavatar --org example --tier team`), so any drift in the two existing
// tiles — colour, glyph, geometry, identity field — is a byte difference here.
//
// regression: F-apps-installer-10
func TestTeamReadActUnchanged(t *testing.T) {
	set, err := Generate("example", TierTeam, Options{Sizes: []int{20}})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]byte{}
	for _, av := range set {
		got[av.App] = av.SVG
	}
	for _, role := range []string{"read", "act"} {
		app := "example-" + role
		want, err := os.ReadFile(filepath.Join("testdata", "golden", "team-"+app+".svg"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got[app], want) {
			t.Errorf("%s: SVG differs from the pre-change golden\n got: %s\nwant: %s", app, got[app], want)
		}
	}
}

// TestProofRejectsReviewLikeRead is Verify row 4: a review tile forced to be a
// copy of the read tile (same glyph, same colours) must make Generate fail with
// a *ProofError naming both Apps — the proof, not the palette choice, is what
// keeps an indistinguishable approver out of a PR timeline.
//
// The org is acme because its read star is bright enough to register as a
// silhouette (asserted below). For a hue whose read star stays under the
// silhouette luminance threshold, two identical tiles both have an EMPTY
// silhouette, silhouetteIoU returns 0, and the proof passes them; that gap in
// the metric is pre-existing and out of this test's reach — see the PR.
//
// regression: F-apps-installer-10
func TestProofRejectsReviewLikeRead(t *testing.T) {
	const org = "acme"
	specs, err := specsFor(org, TierTeam, Options{})
	if err != nil {
		t.Fatal(err)
	}
	read, err := proofTileFor(specs[0])
	if err != nil {
		t.Fatal(err)
	}
	marked := 0
	for _, on := range read.sil {
		if on {
			marked++
		}
	}
	if marked == 0 {
		t.Fatalf("org %q: read star has no silhouette at 20 px — this case no longer exercises a collision", org)
	}

	_, err = Generate(org, TierTeam, Options{Sizes: []int{20}, reviewLikeRead: true})
	if err == nil {
		t.Fatal("a review tile identical to read should fail the proof, got nil")
	}
	var pe *ProofError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProofError, got %T: %v", err, err)
	}
	named := false
	for _, c := range pe.Collisions {
		if strings.Contains(c, org+"-read") && strings.Contains(c, org+"-review") {
			named = true
		}
	}
	if !named {
		t.Errorf("expected the %s-read / %s-review pair to be named; collisions: %v", org, org, pe.Collisions)
	}
}
