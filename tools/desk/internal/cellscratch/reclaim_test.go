package cellscratch

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// finishedOld plants ordinary work in a task, finishes it (code 0 leaves it pending
// an evidence handoff, any other code failed) and backdates it past the handoff
// grace but well inside the retention age.
func finishedOld(t *testing.T, s *Store, code int) *Run {
	t.Helper()
	r := scratchRun(t, s)
	plantFiles(t, r.Work(), "checkout-copy/main.go", "notes.txt", "gocache-x/aa/obj")
	if _, err := r.Tail().Write([]byte("last output")); err != nil {
		t.Fatal(err)
	}
	finish(t, r, code)
	backdate(t, s, r, HandoffGrace+time.Hour)
	return r
}

// A failed or reaped task that never got a receipt loses its whole work dir once
// quiet past the handoff grace, not only build-named output; its record and tail stay,
// and Git data inside work still keeps that entry.
//
// regression: #5560
func TestScratchFinishedWork(t *testing.T) {
	s := scratchStore(t)
	p := DefaultPolicy()
	failed, pending := finishedOld(t, s, 1), finishedOld(t, s, 0)
	plantFiles(t, failed.Work(), "repo/.git/HEAD", "repo/main.go")
	fresh := scratchRun(t, s)
	plantFiles(t, fresh.Work(), "notes.txt")
	finish(t, fresh, 1)
	got := sweep(t, s, p, true)
	rows := rowsByID(got)
	for _, r := range []*Run{failed, pending} {
		if row := rows[r.Record.ID]; row.Action != "reclaim-work" {
			t.Fatalf("finished task work kept: %+v", row)
		}
		for _, gone := range []string{"build", "notes.txt", "checkout-copy", "gocache-x"} {
			if exists(t, filepath.Join(r.Work(), gone)) {
				t.Fatal("finished task kept work entry", gone)
			}
		}
		for _, kept := range []string{"record.json", "diagnostic.txt"} {
			if !exists(t, filepath.Join(s.Path, r.Record.ID, kept)) {
				t.Fatal("finished task lost its record or tail", kept)
			}
		}
	}
	if !exists(t, filepath.Join(failed.Work(), "repo", "main.go")) {
		t.Fatal("Git-holding work entry reclaimed")
	}
	if !exists(t, filepath.Join(fresh.Work(), "notes.txt")) {
		t.Fatal("task inside its handoff grace reclaimed")
	}
	if got.Partial || got.Reclaimed == 0 {
		t.Fatalf("finished-task reclaim partial=%v reclaimed=%d", got.Partial, got.Reclaimed)
	}
}

// A task wider than the inventory bound is paged through, not given up at the bound:
// it is measured in full, reclaimed by the usual rules and the sweep is complete.
//
// regression: #5560
func TestScratchPagedInventory(t *testing.T) {
	s := scratchStore(t)
	wide := scratchRun(t, s)
	acknowledge(t, wide)
	finish(t, wide, 0)
	old := finishedOld(t, s, 1)
	var names []string
	for i := 0; i < 70; i++ {
		names = append(names, fmt.Sprintf("cache/%02d/obj", i))
	}
	plantFiles(t, wide.Work(), names...)
	plantFiles(t, old.Work(), names...)
	for i := 0; i < 70; i++ {
		plantFiles(t, old.Work(), fmt.Sprintf("top-%02d", i))
	}
	t.Setenv("ASSAY_MAINTENANCE_ENTRY_LIMIT", "32")
	rep, err := s.Sweep(DefaultPolicy(), true)
	if err != nil || rep.Partial {
		t.Fatalf("wide task aborted the inventory: %v %+v", err, rep)
	}
	rows := rowsByID(rep)
	if rows[wide.Record.ID].Action != "remove" || exists(t, filepath.Join(s.Path, wide.Record.ID)) {
		t.Fatalf("wide acknowledged task not removed: %+v", rows[wide.Record.ID])
	}
	if rows[old.Record.ID].Action != "reclaim-work" || exists(t, filepath.Join(old.Work(), "cache")) {
		t.Fatalf("wide finished task not reclaimed: %+v", rows[old.Record.ID])
	}
	// 70 cache files, 70 top-level files and the 3 planted by finishedOld, 4 bytes
	// each, plus the 1000-byte build.
	if want := int64(1000 + 143*4); rows[old.Record.ID].Reclaimed != want {
		t.Fatalf("wide task measured %d, want %d", rows[old.Record.ID].Reclaimed, want)
	}
}
