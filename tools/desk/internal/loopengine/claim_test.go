package loopengine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestClaim_NoclobberCollision(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{ClaimsDir: dir, StaleClaim: 120 * time.Minute, RunnerID: "runner-a"}
	it := Item{ID: "loop-engine/01"}

	ok, err := Claim(cfg, it)
	if err != nil || !ok {
		t.Fatalf("first claim: ok=%v err=%v; want ok=true err=nil", ok, err)
	}
	// A second claim of the same item is a collision (someone else owns it) — no clobber.
	ok, err = Claim(cfg, it)
	if err != nil {
		t.Fatalf("second claim errored: %v", err)
	}
	if ok {
		t.Fatal("second claim of a LIVE claim returned ok=true — double-dispatch is possible")
	}
	// The item ID has a slash — it must resolve to a single segment inside the claims dir.
	if _, err := os.Stat(filepath.Join(dir, "loop-engine_01.claim")); err != nil {
		t.Fatalf("claim file not at sanitized path: %v", err)
	}
}

func TestClaim_StaleReclaimedOnlyWhenOldAndNoBranch(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{ClaimsDir: dir, StaleClaim: 30 * time.Minute, RunnerID: "runner-a"}
	it := Item{ID: "issue-841"}

	if ok, err := Claim(cfg, it); err != nil || !ok {
		t.Fatalf("seed claim: ok=%v err=%v", ok, err)
	}
	path := claimPath(cfg, it)

	// Still fresh => not stale => collision, not reclaimed.
	if ok, _ := Claim(cfg, it); ok {
		t.Fatal("fresh claim reclaimed as stale")
	}

	// Age the claim past StaleClaim (no branch recorded) => reclaimable.
	old := time.Now().Add(-90 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if ok, err := Claim(cfg, it); err != nil || !ok {
		t.Fatalf("stale (old, no branch) not reclaimed: ok=%v err=%v", ok, err)
	}
}

func TestClaim_StaleWithLiveBranchNotStolen(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		ClaimsDir:    dir,
		StaleClaim:   30 * time.Minute,
		RunnerID:     "runner-a",
		BranchActive: func(string) bool { return true }, // branch always live
	}
	it := Item{ID: "brief-x"}
	// Hand-write a claim with a branch recorded, aged past the threshold.
	path := claimPath(cfg, it)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"itemID":"brief-x","runner":"other","branch":"feat/x","claimed":"2020-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-90 * time.Minute)
	_ = os.Chtimes(path, old, old)

	// Old, but the recorded branch is provably live => must NOT be stolen.
	if ok, err := Claim(cfg, it); err != nil || ok {
		t.Fatalf("claim with live branch was stolen: ok=%v err=%v", ok, err)
	}
}

func TestClaim_UnreadableExistingFailsClosed(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{ClaimsDir: dir, StaleClaim: time.Minute, RunnerID: "r"}
	it := Item{ID: "z"}
	path := claimPath(cfg, it)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Directory-in-place-of-file: Stat succeeds, but treat any anomaly conservatively.
	// Use an aged unreadable file to exercise the read-error fail-closed path.
	if err := os.WriteFile(path, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	_ = os.Chtimes(path, old, old)
	if os.Geteuid() == 0 {
		t.Skip("running as root: 0000 perms do not block reads")
	}
	ok, err := Claim(cfg, it)
	if ok {
		t.Fatal("unreadable existing claim reclaimed — must fail closed, not steal")
	}
	if err == nil {
		t.Fatal("unreadable existing claim returned nil error — must surface, fail closed")
	}
}

func TestCheckAuthorRunner_RefusesSelf(t *testing.T) {
	it := Item{ID: "hardening/07", BriefPath: "docs/streams/x/brief-07.md", Implementer: "glm-worker-3"}
	err := CheckAuthorRunner(it, "glm-worker-3")
	if err == nil {
		t.Fatal("author==runner not refused")
	}
	var are *AuthorIsRunnerError
	if !errors.As(err, &are) {
		t.Fatalf("wrong error type: %T", err)
	}
	if got := ExitCodeOf(err); got != ExitAuthorIsRunner {
		t.Fatalf("exit code = %d; want distinct %d", got, ExitAuthorIsRunner)
	}
	// Distinct from every deskkit code.
	for _, c := range []int{deskkit.ExitDisabled, deskkit.ExitRateLimited, deskkit.ExitRefused, deskkit.ExitUnverifiable} {
		if ExitAuthorIsRunner == c {
			t.Fatalf("ExitAuthorIsRunner %d collides with deskkit code %d", ExitAuthorIsRunner, c)
		}
	}
}

func TestCheckAuthorRunner_AllowsDifferentAndEmpty(t *testing.T) {
	base := Item{ID: "a", Implementer: "author-1"}
	if err := CheckAuthorRunner(base, "verifier-2"); err != nil {
		t.Fatalf("author != runner wrongly refused: %v", err)
	}
	if err := CheckAuthorRunner(Item{ID: "a"}, "runner"); err != nil {
		t.Fatalf("empty implementer disables guard, got: %v", err)
	}
	if err := CheckAuthorRunner(base, ""); err != nil {
		t.Fatalf("empty runnerID disables guard, got: %v", err)
	}
}

func TestExitCodeOf_DelegatesToDeskkit(t *testing.T) {
	if ExitCodeOf(nil) != deskkit.ExitOK {
		t.Fatal("nil should map to ExitOK")
	}
	if ExitCodeOf(deskkit.Refused("x")) != deskkit.ExitRefused {
		t.Fatal("deskkit refusal not delegated")
	}
	if ExitCodeOf(errors.New("boom")) != deskkit.ExitUnverifiable {
		t.Fatal("unexpected error must fail closed to ExitUnverifiable")
	}
}
