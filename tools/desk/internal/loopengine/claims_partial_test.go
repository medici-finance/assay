package loopengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2972
func TestReclaimPartialClaims(t *testing.T) {
	cfg := Config{ClaimsDir: t.TempDir(), RunnerID: "reader", Branch: "feat/test"}
	plantClaim(t, cfg, "good", "reader", "feat/test")
	bad := filepath.Join(cfg.ClaimsDir, "bad.claim")
	if err := os.WriteFile(bad, []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	if ok, err := reclaimForLiveness(cfg, "good"); err != nil || !ok {
		t.Fatalf("readable claim could not be reclaimed: %v %v", ok, err)
	}
	if ok, err := reclaimForLiveness(cfg, "bad"); ok || !deskkit.IsUnverifiable(err) {
		t.Fatalf("blind target mistaken for absent: %v %v", ok, err)
	}
	data, err := os.ReadFile(bad)
	if err != nil || string(data) != `{broken` {
		t.Fatalf("bad claim changed: %q %v", data, err)
	}
}

// regression: #2972
func TestRecoverPartialClaims(t *testing.T) {
	cfg, loop, _ := recoverFixture(t, "reader", "feat/test", "restart")
	var progress strings.Builder
	cfg.Progress = &progress
	plantClaim(t, cfg, "good", "reader", "feat/test")
	journalPrior(t, "prior", journalLand, "good", VerdictPass)
	bad := filepath.Join(cfg.ClaimsDir, "bad.claim")
	if err := os.WriteFile(bad, []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	Recover(cfg, loop)
	if claimExists(t, cfg, "good") {
		t.Fatal("readable completed claim was not released")
	}
	if !strings.Contains(progress.String(), "bad.claim") {
		t.Fatalf("recovery hid unreadable claim: %q", progress.String())
	}
	data, err := os.ReadFile(bad)
	if err != nil || string(data) != `{broken` {
		t.Fatalf("bad claim changed: %q %v", data, err)
	}
}
