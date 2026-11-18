package main

import (
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/readercaps"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// printedProgressArgs is the argument vector of the progress command deskdispatch prints
// for a prepared run: `progress "<key>" --repo <repo> --branch - --token-file <f>`.
func printedProgressArgs(key, branchForm string) []string {
	args := []string{"progress", key, "--repo", "medici-finance/assay", "--owner", "verifier-1"}
	return append(args, strings.Fields(branchForm)...)
}

// regression: #6896
func TestPrintedBranchlessProgressRuns(t *testing.T) {
	for _, form := range []string{"--branch -", "--branch=-"} {
		t.Run(form, func(t *testing.T) {
			f := newStore()
			f.seedClaim("assay--stream--08", "verifier-1", "claimed", "", time.Minute)
			rc, _, se := harnessRun(t, f, printedProgressArgs("assay--stream--08", form)...)
			if rc != exitOK {
				t.Fatalf("%q: rc = %d, want 0: %s", form, rc, se.String())
			}
			msg := f.claims["assay--stream--08"].msg
			if !strings.Contains(msg, "state=dispatched") || !strings.Contains(msg, "branch=-") {
				t.Fatalf("%q: claim not advanced branchless: %s", form, msg)
			}
		})
	}
}

// A verifier claim key can carry the work-reference marker "--b-" while being a plain
// physical key. The tracking repo publishes no owning root for itself, so the work-source
// read refuses; progress must still advance a claim that holds no negotiated subject.
// regression: #6896
func TestProgressWithoutPublishedOwningRoot(t *testing.T) {
	const key = "assay-console--stream--b-08"
	old := loadClaimWork
	t.Cleanup(func() { loadClaimWork = old })
	loadClaimWork = func(string) (deskkit.WorkCatalog, []readercaps.Observation, bool, error) {
		return deskkit.WorkCatalog{}, nil, false, deskkit.NoPublishedOwningRoot()
	}
	f := newStore()
	f.seedClaim(key, "verifier-1", "claimed", "", time.Minute)
	rc, _, se := harnessRun(t, f, printedProgressArgs(key, "--branch -")...)
	if rc != exitOK {
		t.Fatalf("progress rc = %d, want 0: %s", rc, se.String())
	}
	if msg := f.claims[key].msg; !strings.Contains(msg, "state=dispatched") {
		t.Fatalf("claim not advanced: %s", msg)
	}

	// Only progress relaxes: acquire still needs the owning root.
	if rc, _, _ := harnessRun(t, newStore(), "acquire", key, "--repo", "medici-finance/assay", "--owner", "verifier-1"); rc == exitOK {
		t.Fatal("acquire proceeded without a published owning root")
	}

	// A claim holding a negotiated subject is never advanced by the legacy writer.
	catalog, peers := claimWorkFixture(t)
	sel, err := catalog.Select("stream:b-08@2")
	if err != nil {
		t.Fatal(err)
	}
	g := newStore()
	msg, _ := deskkit.EncodeClaimSubject(claimTestMessage(sel.ClaimKey, "verifier-1", "claimed", "", ""), sel.Subject)
	g.claims[sel.ClaimKey] = fakeClaim{sha: g.nextSHA(), msg: msg, date: g.now.Format(time.RFC3339)}
	_ = peers
	before := g.claims[sel.ClaimKey]
	loadClaimWork = func(string) (deskkit.WorkCatalog, []readercaps.Observation, bool, error) {
		return deskkit.WorkCatalog{}, nil, false, deskkit.NoPublishedOwningRoot()
	}
	if rc, _, _ := harnessRun(t, g, printedProgressArgs(sel.ClaimKey, "--branch -")...); rc == exitOK || g.claims[sel.ClaimKey] != before {
		t.Fatal("legacy progress advanced a subject-bearing claim")
	}
}

// A stale or unknown flag stays refused: the = form is accepted only for the claim flags.
// regression: #6896
func TestEqualsFormRejectsUnknownFlags(t *testing.T) {
	f := newStore()
	f.seedClaim("assay--stream--08", "verifier-1", "claimed", "", time.Minute)
	if rc, _, _ := harnessRun(t, f, "progress", "assay--stream--08", "--repo", "medici-finance/assay", "--bogus=1", "--branch", "-"); rc != exitRefused {
		t.Fatalf("unknown --bogus=1 rc = %d, want %d", rc, exitRefused)
	}
}
