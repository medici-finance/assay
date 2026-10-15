package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEvidenceHistoryChecksBoundedAndOrdered(t *testing.T) {
	const count = evidenceHistoryWorkers*3 + 1
	entered := make(chan int, count)
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var active, peak atomic.Int32
	done := make(chan []evidenceHistoryResult, 1)
	go func() {
		done <- evidenceHistoryChecks(count, func(i int) (bool, bool) {
			n := active.Add(1)
			for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
			}
			entered <- i
			<-release
			active.Add(-1)
			return i%2 == 0, i%3 != 0
		})
	}()
	for i := 0; i < evidenceHistoryWorkers; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("history reads are still serial or workers failed to start")
		}
	}
	if active.Load() != evidenceHistoryWorkers || peak.Load() > evidenceHistoryWorkers {
		t.Fatalf("worker bound violated: active=%d peak=%d", active.Load(), peak.Load())
	}
	unblock()
	results := <-done
	for i, r := range results {
		if r.touchedByOther != (i%2 == 0) || r.known != (i%3 != 0) {
			t.Errorf("index %d changed verdict/unknown handling: %+v", i, r)
		}
	}
	if peak.Load() > evidenceHistoryWorkers {
		t.Fatalf("later work exceeded the worker bound: %d", peak.Load())
	}
	if got := evidenceHistoryChecks(0, func(int) (bool, bool) {
		t.Fatal("empty work called the history reader")
		return false, false
	}); len(got) != 0 {
		t.Fatalf("empty work returned %v", got)
	}
}
