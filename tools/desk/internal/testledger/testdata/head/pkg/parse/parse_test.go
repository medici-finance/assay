package parse

import (
	"strings"
	"testing"
)

// TestGammaRenamed is TestGamma under a new name; only this comment and the
// layout changed, so the body hash still matches.
func TestGammaRenamed(t *testing.T) {
	got := strings.ToUpper("gamma")

	if got != "GAMMA" {
		t.Fatalf("gamma: %s", got) // same statement, new comment
	}
}

// regression: class #12
func TestDeltaRewritten(t *testing.T) {
	if !strings.HasSuffix("delta", "a") {
		t.Fatal("delta")
	}
}

func TestEpsilon(t *testing.T) {
	if strings.Count("epsilon", "e") != 1 {
		t.Fatal("epsilon")
	}
}

func TestZeta(t *testing.T) {
	if strings.Repeat("z", 2) != "zz" {
		t.Fatal("zeta")
	}
}

func TestEta(t *testing.T) {
	if strings.Index("eta", "t") != 1 {
		t.Fatal("eta")
	}
}
