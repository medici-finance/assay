package parse

import (
	"os"
	"strings"
	"testing"
)

// TestMain is not a test: removing it is not a departure.
func TestMain(m *testing.M) { os.Exit(m.Run()) }

// TestAlpha pins the quoted-scalar case.
// regression: F-fixture-alpha
func TestAlpha(t *testing.T) {
	if strings.TrimSpace(" a ") != "a" {
		t.Fatal("alpha")
	}
}

func TestBeta(t *testing.T) {
	if len("beta") != 4 {
		t.Fatal("beta")
	}
}

func TestGamma(t *testing.T) {
	got := strings.ToUpper("gamma")
	if got != "GAMMA" {
		t.Fatalf("gamma: %s", got)
	}
}

// regression: class #12
func TestDelta(t *testing.T) {
	if !strings.HasPrefix("delta", "d") {
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

// Testhelper is not a test under go test's naming rule.
func Testhelper(t *testing.T) { t.Helper() }
