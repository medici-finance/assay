package loopengine

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// twoInFlightWait bounds how long a dispatched drill item waits for a second item to be in
// flight beside it. It is far above any delay a loaded scheduler puts between two
// dispatches of one pool fill, so it is never the thing that decides a healthy run. It
// stays under runUntil's 5s deadline, so a pool that never runs two items at once fails
// with the concurrency message below rather than the generic no-exit message.
const twoInFlightWait = 3 * time.Second

// TestDrain is the drain drill against a FIXTURE queue (no live briefs). It proves,
// in one run: the pool runs more than one item at once, land-as-returned, the
// file-and-continue path, drain to empty, idle-poll, and stop-flag exit. Its Progress
// lines ("landed:" / "filed-and-continued:") are what Verify item 3 greps for (>=4). Run
// with -v to see them.
//
// The concurrency proof is a BARRIER, not a sample. Every drill item stays in flight until
// the pool has had two items in flight at the same moment, so a pool that can run two at
// once always shows it, however the scheduler orders the goroutines. An earlier version
// slept a few milliseconds per item and then asserted the observed peak was above 1; on a
// loaded machine the items finished one after another and the assertion failed although
// the pool was correct. The barrier wait is bounded (twoInFlightWait), so a pool that
// really does run items one at a time fails instead of hanging.
func TestDrain(t *testing.T) {
	deskDir := setupDeskHome(t, testLoopName)

	const poolN = 3
	var inFlight int32
	var maxConcurrent int32

	// twoInFlight closes the first time a dispatch finds another drill item still in
	// flight. barrierCtx bounds the wait for it; its Done channel releases every waiter at
	// once, so a serial pool costs one timeout, not one per item.
	twoInFlight := make(chan struct{})
	var twoInFlightOnce sync.Once
	barrierCtx, cancelBarrier := context.WithTimeout(context.Background(), twoInFlightWait)
	defer cancelBarrier()
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
		// Until the barrier opens no item can finish, so inFlight only counts up: reaching
		// 2 here means the engine dispatched this item while an earlier one was still out.
		if cur >= 2 {
			twoInFlightOnce.Do(func() { close(twoInFlight) })
		}
		h := &fakeHandle{item: it, done: make(chan Result, 1)}
		go func() {
			select {
			case <-twoInFlight:
			case <-barrierCtx.Done():
				// No second item arrived in time. Finish anyway so the drill drains and
				// the assertion below reports the serial pool, rather than hanging.
			}
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
	select {
	case <-twoInFlight:
	default:
		t.Fatalf("no two items were in flight at once within %s (PoolSize %d, max in flight %d) — the pool runs items one at a time",
			twoInFlightWait, poolN, atomic.LoadInt32(&maxConcurrent))
	}
	if got := atomic.LoadInt32(&maxConcurrent); got > poolN {
		t.Fatalf("max concurrency %d exceeded PoolSize %d", got, poolN)
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
