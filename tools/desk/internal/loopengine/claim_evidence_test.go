package loopengine

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// The corpus these tests encode (measured 2026-08-13, on this repo's stream boards):
// a dispatcher acquired claims for two briefs whose board rows read "todo" and dispatched
// workers; BOTH briefs already had open PRs (sample/01 and sample/04),
// and the claim call returned rc=0 for both, because a claim records only "this
// dispatcher is working on it". Across the same boards, 24 of 147 "todo" rows already had
// a MERGED PR. Claim's contract therefore has to consult evidence outside the claims dir,
// and has to say which evidence it consulted.

// TestClaimRefusesItemWithOpenPR is the positive control for the dedupe check: an item
// that ALREADY has an open PR is offered, and Claim must refuse. This is the exact live
// failure above. MUTATION TARGET (see mutations.json): removing the taken-branch from
// checkWorkEvidence must redden this test.
func TestClaimRefusesItemWithOpenPR(t *testing.T) {
	dir := t.TempDir()
	var log bytes.Buffer
	cfg := Config{
		ClaimsDir:  dir,
		StaleClaim: 120 * time.Minute,
		RunnerID:   "dispatcher-a",
		Progress:   &log,
		WorkEvidence: func(it Item) (bool, string, error) {
			if it.ID == "sample/01" {
				return true, "an open PR (2026-08-12) already implements it", nil
			}
			return false, "", nil
		},
	}
	it := Item{ID: "sample/01"}

	ok, err := Claim(cfg, it)
	if err != nil {
		t.Fatalf("Claim errored: %v; want a clean refusal (false, nil)", err)
	}
	if ok {
		t.Fatal("Claim ACQUIRED an item that already has an open PR — this is the 2026-08-13 double-dispatch reopening")
	}
	// The refusal must not have written a claim: a refused item is not half-claimed.
	if _, serr := os.Stat(claimPath(cfg, it)); !os.IsNotExist(serr) {
		t.Fatalf("a refused claim left a claim file behind (stat err=%v)", serr)
	}
	// The refusal must NAME what it deduplicated against — a silent no-op is the failure
	// this replaced.
	if got := log.String(); !strings.Contains(got, "DEDUP sample/01") || !strings.Contains(got, "open PR") {
		t.Fatalf("refusal did not name its evidence; Progress=%q", got)
	}

	// Control: an item the SAME probe reports free still claims, so the test above is
	// measuring the check and not a Claim that refuses everything.
	if ok, err := Claim(cfg, Item{ID: "sample/09"}); err != nil || !ok {
		t.Fatalf("an item with no work evidence failed to claim: ok=%v err=%v", ok, err)
	}
}

// TestClaimFailsClosedWhenEvidenceUnreachable is the three-state instrument invariant
// (docs/three-state-instrument-rule.md) at the claim boundary: an evidence source that
// cannot be reached is COULD-NOT-CHECK, never a confident "acquired". A dedupe that fails
// open silently is the defect, not the fix.
func TestClaimFailsClosedWhenEvidenceUnreachable(t *testing.T) {
	dir := t.TempDir()
	var log bytes.Buffer
	cfg := Config{
		ClaimsDir: dir,
		RunnerID:  "dispatcher-a",
		Progress:  &log,
		WorkEvidence: func(Item) (bool, string, error) {
			return false, "", errors.New("gh: API unreachable")
		},
	}
	it := Item{ID: "sample/10"}

	ok, err := Claim(cfg, it)
	if ok {
		t.Fatal("Claim returned acquired=true when the evidence source was unreachable — fail-open dedupe")
	}
	if err == nil {
		t.Fatal("an unreachable evidence check returned no error — the caller cannot tell could-not-check from checked-clean")
	}
	if got := deskkit.ExitCodeOf(err); got != deskkit.ExitUnverifiable {
		t.Fatalf("could-not-check mapped to exit %d, want %d (unverifiable)", got, deskkit.ExitUnverifiable)
	}
	if _, serr := os.Stat(claimPath(cfg, it)); !os.IsNotExist(serr) {
		t.Fatalf("a could-not-check claim wrote a claim file anyway (stat err=%v)", serr)
	}
	if got := log.String(); !strings.Contains(got, "COULD-NOT-CHECK") {
		t.Fatalf("could-not-check was not stated in Claim's own output; Progress=%q", got)
	}
}

// TestClaimAnnouncesItsBoundWhenNoProbe is the "no silent caps" rule: Claim MAY be
// configured without an evidence probe, but it must then say, in its own output, that the
// only thing it consulted was the claims dir — so a caller cannot read "acquired" as a
// wider guarantee than the one that was actually checked.
func TestClaimAnnouncesItsBoundWhenNoProbe(t *testing.T) {
	dir := t.TempDir()
	var log bytes.Buffer
	cfg := Config{ClaimsDir: dir, RunnerID: "dispatcher-a", Progress: &log}

	if ok, err := Claim(cfg, Item{ID: "sample/10"}); err != nil || !ok {
		t.Fatalf("claim with no probe: ok=%v err=%v; want ok=true (the bound is announced, not enforced)", ok, err)
	}
	got := log.String()
	if !strings.Contains(got, "BOUND sample/10") {
		t.Fatalf("Claim did not announce its narrowed evidence bound; Progress=%q", got)
	}
	if !strings.Contains(got, "claims dir ONLY") {
		t.Fatalf("the announced bound did not say WHAT was consulted; Progress=%q", got)
	}
}

// TestClaimEvidenceRunsBeforeTheLock pins the ordering documented in doc.go: the probe is
// consulted first, so a refused item never takes the directory-wide flock and a network
// call is never made while that lock is held.
func TestClaimEvidenceRunsBeforeTheLock(t *testing.T) {
	dir := t.TempDir()
	var order []string
	cfg := Config{
		ClaimsDir: dir,
		RunnerID:  "dispatcher-a",
		Progress:  new(bytes.Buffer),
		WorkEvidence: func(Item) (bool, string, error) {
			order = append(order, "probe")
			// The claims dir must be untouched at probe time — nothing has locked yet.
			if entries, err := os.ReadDir(dir); err == nil && len(entries) != 0 {
				t.Errorf("claims dir already has %d entries at probe time — the lock ran first", len(entries))
			}
			return true, "a merged PR already implements it", nil
		},
	}
	if ok, err := Claim(cfg, Item{ID: "sample/42"}); ok || err != nil {
		t.Fatalf("Claim: ok=%v err=%v; want refusal", ok, err)
	}
	if len(order) != 1 || order[0] != "probe" {
		t.Fatalf("probe call order = %v; want exactly one probe call", order)
	}
}
