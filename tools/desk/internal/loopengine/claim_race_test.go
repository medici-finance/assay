package loopengine

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

// TestClaimRecordsBranch — the branch-liveness guard in claimIsStale reads
// claimRecord.Branch, but Claim never WROTE it, so the branch half of the stale-claim
// policy was dead code and every claim was reclaimable on age alone. Populating it is
// what makes the guard exist.
func TestClaimRecordsBranch(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{ClaimsDir: dir, StaleClaim: 120 * time.Minute, RunnerID: "runner-a", Branch: "fix/issue-216"}
	it := Item{ID: "branch/01"}

	ok, err := Claim(cfg, it)
	if err != nil || !ok {
		t.Fatalf("Claim: ok=%v err=%v", ok, err)
	}
	raw, err := os.ReadFile(claimPath(cfg, it))
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	var rec claimRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("parse claim: %v", err)
	}
	if rec.Branch != "fix/issue-216" {
		t.Fatalf("claim Branch = %q, want %q — the liveness guard has nothing to check", rec.Branch, "fix/issue-216")
	}
}

// TestClaimBranchGuardIsLiveEndToEnd — with Branch recorded and BranchActive reporting
// the branch still live, an AGED claim is no longer reclaimable. This is the behaviour
// the dead field made unreachable: before the fix the same scenario reclaimed, because
// the recorded branch was always empty.
func TestClaimBranchGuardIsLiveEndToEnd(t *testing.T) {
	dir := t.TempDir()
	it := Item{ID: "branch/02"}
	holder := Config{ClaimsDir: dir, StaleClaim: 30 * time.Minute, RunnerID: "holder", Branch: "feat/live"}
	if ok, err := Claim(holder, it); err != nil || !ok {
		t.Fatalf("seed claim: ok=%v err=%v", ok, err)
	}
	age(t, claimPath(holder, it), 90*time.Minute)

	taker := Config{
		ClaimsDir: dir, StaleClaim: 30 * time.Minute, RunnerID: "taker",
		BranchActive: func(b string) bool { return b == "feat/live" },
	}
	ok, err := Claim(taker, it)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if ok {
		t.Fatal("an aged claim whose branch is still LIVE was stolen — the guard did not fire")
	}

	// The same claim, with the branch now proven dead, IS reclaimable — the guard
	// protects live work, it does not park items forever.
	dead := taker
	dead.BranchActive = func(string) bool { return false }
	ok, err = Claim(dead, it)
	if err != nil || !ok {
		t.Fatalf("aged claim with a dead branch must be reclaimable: ok=%v err=%v", ok, err)
	}
}

// TestClaimConcurrentReclaimExactlyOneWinner — N reclaimers race for one stale claim.
// The old sequence (Remove then O_EXCL create) was not atomic: reclaimers could both
// pass the staleness check, and while the file was momentarily absent an unrelated
// FRESH claimant's create also succeeded. Two winners means the same item is dispatched
// twice.
func TestClaimConcurrentReclaimExactlyOneWinner(t *testing.T) {
	dir := t.TempDir()
	it := Item{ID: "concurrent/01"}
	seed := Config{ClaimsDir: dir, StaleClaim: 30 * time.Minute, RunnerID: "seeder"}
	if ok, err := Claim(seed, it); err != nil || !ok {
		t.Fatalf("seed claim: ok=%v err=%v", ok, err)
	}
	age(t, claimPath(seed, it), 90*time.Minute)

	const runners = 16
	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
		mu    sync.Mutex
		wins  []string
		errs  []error
	)
	for i := 0; i < runners; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cfg := Config{
				ClaimsDir: dir, StaleClaim: 30 * time.Minute,
				RunnerID: fmt.Sprintf("runner-%02d", id),
				Progress: io.Discard,
			}
			<-start
			ok, err := Claim(cfg, it)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if ok {
				wins = append(wins, cfg.RunnerID)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if len(errs) != 0 {
		t.Fatalf("concurrent Claim returned errors: %v", errs)
	}
	if len(wins) != 1 {
		t.Fatalf("winners = %d (%v), want exactly 1 — the item would be dispatched %d times", len(wins), wins, len(wins))
	}

	// The winner owns the file, and no lock file is left behind to park the item.
	raw, err := os.ReadFile(claimPath(seed, it))
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	var rec claimRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatalf("parse claim: %v", err)
	}
	if rec.Runner != wins[0] {
		t.Fatalf("claim file records %q but the winner was %q", rec.Runner, wins[0])
	}
	if _, err := os.Stat(claimPath(seed, it) + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("reclaim lock was not released: %v", err)
	}
}

// TestClaimFreshClaimantCannotSlipThroughReclaim — a reclaimer must never leave the
// claim file absent, because a fresh claimant's O_EXCL create would then succeed
// alongside it. Proven structurally: the file exists at every observable moment.
func TestClaimFreshClaimantCannotSlipThroughReclaim(t *testing.T) {
	dir := t.TempDir()
	it := Item{ID: "gap/01"}
	seed := Config{ClaimsDir: dir, StaleClaim: 30 * time.Minute, RunnerID: "seeder"}
	if ok, err := Claim(seed, it); err != nil || !ok {
		t.Fatalf("seed claim: ok=%v err=%v", ok, err)
	}
	path := claimPath(seed, it)
	age(t, path, 90*time.Minute)

	stop := make(chan struct{})
	gaps := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				select {
				case gaps <- struct{}{}:
				default:
				}
				return
			}
		}
	}()

	reclaimer := Config{ClaimsDir: dir, StaleClaim: 30 * time.Minute, RunnerID: "reclaimer", Progress: io.Discard}
	if ok, err := Claim(reclaimer, it); err != nil || !ok {
		t.Fatalf("reclaim: ok=%v err=%v", ok, err)
	}
	close(stop)
	select {
	case <-gaps:
		t.Fatal("the claim file vanished during reclaim — a fresh claimant could have created it")
	default:
	}
}

func age(t *testing.T, path string, by time.Duration) {
	t.Helper()
	old := time.Now().Add(-by)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("age %s: %v", path, err)
	}
}
