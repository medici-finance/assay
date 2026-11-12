package deskkit

import (
	"slices"
	"strings"
	"testing"
)

// TestGateRefusalNamesGatedVerbs pins the scope sentence of the public-repo gate's refusal.
// The gate runs only in the commands PublicRepoGateVerbs lists, so the refusal must name
// those commands, name a tracker write it does not cover, and never claim to cover every
// write verb (the pre-#5436 wording, false for deskfile).
//
// regression: #5436
func TestGateRefusalNamesGatedVerbs(t *testing.T) {
	installRoster(t, internalGateRoster)

	cases := []struct{ live, repo string }{
		{"public", "bare"},       // live public, no visibility tag
		{"public", "inner"},      // live public, tagged :internal
		{"internal", "privrepo"}, // live internal, tagged :private (#5514: :public now passes)
		{"internal", "not-in-set"},
	}
	for _, c := range cases {
		err := PublicRepoGate(&stubRepoInfoFetcher{visibility: c.live}, "example-org", c.repo)
		if !IsRefused(err) {
			t.Fatalf("%s live %s: got %v, want Refused/exit 5", c.repo, c.live, err)
		}
		msg := err.Error()
		if strings.Contains(msg, "every write verb") {
			t.Fatalf("%s live %s: refusal still claims to cover every write verb: %s", c.repo, c.live, msg)
		}
		want := "covers the desk commands that run this gate (deskpr, deskpost, deskreply, deskevidence, deskrelease); " +
			"tracker writes such as deskfile do not run it; merge remains the human's."
		if !strings.Contains(msg, want) {
			t.Fatalf("%s live %s: refusal lacks the gated-commands sentence\n got: %s\nwant substring: %s", c.repo, c.live, msg, want)
		}
	}

	if slices.Contains(PublicRepoGateVerbs, publicRepoGateUngated) {
		t.Fatalf("%s is named as ungated but is listed in PublicRepoGateVerbs %v", publicRepoGateUngated, PublicRepoGateVerbs)
	}
}
