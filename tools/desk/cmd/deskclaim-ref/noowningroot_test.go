package main

import (
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// A tracking-repo claim key can carry the work-reference marker "--b-" while being a plain
// physical key. The tracking repo publishes no owning root for itself, so the work-source
// read refuses; progress must still advance a claim that holds no negotiated subject.
// regression: #6896
func TestProgressWithoutPublishedOwningRoot(t *testing.T) {
	const key = "tracker--stream--b-08"
	old := loadClaimWork
	t.Cleanup(func() { loadClaimWork = old })
	noRoot := func(string) (deskkit.WorkCatalog, []readercaps.Observation, bool, error) {
		return deskkit.WorkCatalog{}, nil, false, deskkit.NoPublishedOwningRoot()
	}
	loadClaimWork = noRoot

	f := newStore()
	f.seedClaim(key, "verifier-1", "claimed", "", time.Minute)
	rc, _, se := harnessRun(t, f, "progress", key, "--repo", placeholderRepo, "--owner", "verifier-1", "--branch", "-")
	if rc != exitOK {
		t.Fatalf("progress rc = %d, want 0: %s", rc, se.String())
	}
	if msg := f.claims[key].msg; fieldOf(msg, "state") != "dispatched" || fieldOf(msg, "branch") != "-" {
		t.Fatalf("claim not advanced: %s", msg)
	}

	// Only progress relaxes: acquire still needs the owning root.
	if rc, _, _ := harnessRun(t, newStore(), "acquire", key, "--repo", placeholderRepo, "--owner", "verifier-1"); rc == exitOK {
		t.Fatal("acquire proceeded without a published owning root")
	}

	// A claim holding a negotiated subject is never advanced by the legacy writer.
	catalog, _ := claimWorkFixture(t)
	sel, err := catalog.Select("stream:b-08@2")
	if err != nil {
		t.Fatal(err)
	}
	g := newStore()
	msg, _ := deskkit.EncodeClaimSubject(claimTestMessage(sel.ClaimKey, "verifier-1", "claimed", "", ""), sel.Subject)
	g.claims[sel.ClaimKey] = fakeClaim{sha: g.nextSHA(), msg: msg, date: g.now.Format(time.RFC3339)}
	before := g.claims[sel.ClaimKey]
	if rc, _, _ := harnessRun(t, g, "progress", sel.ClaimKey, "--repo", placeholderRepo, "--owner", "verifier-1", "--branch", "-"); rc == exitOK || g.claims[sel.ClaimKey] != before {
		t.Fatal("legacy progress advanced a subject-bearing claim")
	}
}
