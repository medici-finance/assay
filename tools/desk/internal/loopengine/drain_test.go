package loopengine

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestDrain is the drain drill against a FIXTURE queue (no live briefs). It proves,
// in one run: sustain-at-N concurrency, land-as-returned, the file-and-continue path, drain
// to empty, idle-poll, and stop-flag exit. Its Progress lines ("landed:" / "filed-and-
// continued:") are what Verify item 3 greps for (>=4). Run with -v to see them.
func TestDrain(t *testing.T) {
	deskDir := setupDeskHome(t, testLoopName)

	const poolN = 3
	var inFlight int32
	var maxConcurrent int32
	verdicts := map[string]string{
		"drill-ok-1":  VerdictPass,
		"drill-ok-2":  VerdictPass,
		"drill-ok-3":  VerdictPass,
		"drill-fail":  VerdictFail, // deliberate FAIL — still LANDS its FAIL Evidence
		"drill-stuck": VerdictPass, // deliberate un-landable — Land returns error => filed-and-continued
	}

	loop := &fakeLoop{name: "drilltest"}
	for id := range verdicts {
		loop.remaining = append(loop.remaining, Item{ID: id, BriefPath: "docs/streams/fixture/brief-" + id + ".md"})
	}

	loop.dispatchFn = func(l *fakeLoop, it Item, tier Tier) (Handle, error) {
		cur := atomic.AddInt32(&inFlight, 1)
		for {
			m := atomic.LoadInt32(&maxConcurrent)
			if cur <= m || atomic.CompareAndSwapInt32(&maxConcurrent, m, cur) {
				break
			}
		}
		h := &fakeHandle{item: it, done: make(chan Result, 1)}
		go func() {
			time.Sleep(8 * time.Millisecond) // simulate real verifier work so the pool fills
			atomic.AddInt32(&inFlight, -1)
			h.done <- Result{
				Item:     it,
				Verdict:  verdicts[it.ID],
				RunnerID: "local:drill-verifier",
				Rows:     []EvidenceRow{{Command: "go test ./...", Exit: boolExit(verdicts[it.ID]), Output: "fixture output"}},
			}
		}()
		return h, nil
	}

	var mu sync.Mutex
	filed := 0
	loop.landFn = func(l *fakeLoop, r Result) error {
		if r.Item.ID == "drill-stuck" {
			mu.Lock()
			filed++
			mu.Unlock()
			// Simulate an item whose durable landing cannot succeed even after retry.
			l.mu.Lock()
			l.removeLocked(r.Item.ID) // engine parks it; drop from queue so drain finishes
			l.mu.Unlock()
			return fmt.Errorf("push race unresolved after retries")
		}
		l.recordLandAndDrain(r)
		return nil
	}

	cfg := Config{
		PoolSize:   poolN,
		IdlePoll:   3 * time.Millisecond,
		ClaimsDir:  t.TempDir(),
		StaleClaim: time.Hour,
		Progress:   os.Stdout, // Verify item 3 greps this
	}

	err := runUntil(t, cfg, loop, deskDir, func() bool {
		// stop once the four landable items landed and the stuck one was filed
		mu.Lock()
		f := filed
		mu.Unlock()
		return len(loop.landedIDs()) == 4 && f >= 1
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := len(loop.landedIDs()); got != 4 {
		t.Fatalf("landed %d; want 4 (3 PASS + 1 FAIL, all land their Evidence)", got)
	}
	if maxConcurrent < 2 {
		t.Fatalf("max concurrency observed %d; the pool never sustained >1 in flight", maxConcurrent)
	}
	if maxConcurrent > poolN {
		t.Fatalf("max concurrency %d exceeded PoolSize %d", maxConcurrent, poolN)
	}
	mu.Lock()
	defer mu.Unlock()
	if filed == 0 {
		t.Fatal("the un-landable item was never filed-and-continued")
	}
}

func boolExit(verdict string) int {
	if verdict == VerdictPass {
		return 0
	}
	return 1
}
