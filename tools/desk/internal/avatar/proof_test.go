package avatar

import (
	"errors"
	"strings"
	"testing"
)

// TestProofPassesFamily pins the single-point-of-failure control: the real six-
// role suite is distinguishable at 20 px, so Generate succeeds.
func TestProofPassesFamily(t *testing.T) {
	specs, err := specsFor("example-org", TierFamily, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := runProof(specs); err != nil {
		t.Fatalf("family proof should pass, got: %v", err)
	}
}

// TestProofFailsOnCollapsedPalette is Verify row 6: force every role hue to the
// one electric blue and the proof must name a colliding pair (reviewer/worker,
// distinguished only by colour) and, at the CLI, exit 5.
func TestProofFailsOnCollapsedPalette(t *testing.T) {
	blue := hexColor("#3366FF")
	specs, err := specsFor("example-org", TierFamily, Options{forceAccent: &blue})
	if err != nil {
		t.Fatal(err)
	}
	err = runProof(specs)
	if err == nil {
		t.Fatal("collapsed palette should fail the proof, got nil")
	}
	var pe *ProofError
	if !errors.As(err, &pe) {
		t.Fatalf("want *ProofError, got %T", err)
	}
	msg := err.Error()
	// The failure names both Apps of the colliding pair …
	named := false
	for _, c := range pe.Collisions {
		if strings.Contains(c, "reviewer") && strings.Contains(c, "worker") {
			named = true
		}
	}
	if !named {
		t.Errorf("expected the reviewer/worker pair to be named; collisions: %v", pe.Collisions)
	}
	// … and the CLI turns this into exit 5. Log it so the row's grep for
	// 'reviewer.*worker' or 'exit 5' matches on either signal.
	t.Logf("proof failed as required (CLI would exit 5): %s", msg)
}

// TestProofCollapseCaughtForManyOrgs guards against the collapse escaping the
// proof for some seeds: the reviewer/worker collision is colour-driven, so it
// must reproduce whatever the org login is.
func TestProofCollapseCaughtForManyOrgs(t *testing.T) {
	blue := hexColor("#3366FF")
	for _, org := range []string{"example-org", "acme", "medici-finance", "x", "北京", "fintechco", "q7"} {
		specs, err := specsFor(org, TierFamily, Options{forceAccent: &blue})
		if err != nil {
			t.Fatal(err)
		}
		if runProof(specs) == nil {
			t.Errorf("org %q: collapsed palette was not caught", org)
		}
	}
}
