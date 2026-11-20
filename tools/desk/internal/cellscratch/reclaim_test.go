package cellscratch

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// old's work holds 75 top-level entries: build, the 3 finishedOld plants, cache
	// and 70 top-level files. One sweep plans and reclaims one batch of 32.
	row := rows[old.Record.ID]
	if row.Action != "reclaim-work" || !strings.Contains(row.Reason, "bounded batch") {
		t.Fatalf("wide finished task not reclaimed in a bounded batch: %+v", row)
	}
	if left := workWidth(t, old); left != 75-32 {
		t.Fatalf("one sweep retained %d candidates, want the bound 32", 75-left)
	}
	reclaimed := row.Reclaimed
	for i := 0; i < 5 && workWidth(t, old) > 0; i++ {
		rep, err = s.Sweep(DefaultPolicy(), true)
		if err != nil || rep.Partial {
			t.Fatalf("later batch: %v %+v", err, rep)
		}
		reclaimed += rowsByID(rep)[old.Record.ID].Reclaimed
	}
	if left := workWidth(t, old); left != 0 {
		t.Fatalf("wide finished task never fully reclaimed: %d entries left", left)
	}
	// 70 cache files, 70 top-level files and the 3 planted by finishedOld, 4 bytes
	// each, plus the 1000-byte build: every byte is measured once across the batches.
	if want := int64(1000 + 143*4); reclaimed != want {
		t.Fatalf("wide task measured %d, want %d", reclaimed, want)
	}
}

// workWidth counts the entries directly under a task's work dir.
func workWidth(t *testing.T, r *Run) int {
	t.Helper()
	entries, err := os.ReadDir(r.Work())
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

// Removal empties a directory a bounded number of pages at a time: a directory
// wider than the bound fails that removal, partial and retried, instead of the
// loop running for as long as something refills it. Later sweeps finish the job.
//
// regression: #5560
func TestScratchRemovalPageBound(t *testing.T) {
	s := scratchStore(t)
	old := finishedOld(t, s, 1)
	var names []string
	for i := 0; i < 70; i++ {
		names = append(names, fmt.Sprintf("cache/%02d", i))
	}
	plantFiles(t, old.Work(), names...)
	t.Setenv("ASSAY_MAINTENANCE_ENTRY_LIMIT", "32")
	saved := RemovalPages
	RemovalPages = 1
	t.Cleanup(func() { RemovalPages = saved })
	rep, err := s.Sweep(DefaultPolicy(), true)
	if row := rowsByID(rep)[old.Record.ID]; !errors.Is(err, errRefilled) || !rep.Partial || row.Action != "error" {
		t.Fatalf("removal ran past its page bound: %v %+v", err, row)
	}
	cache, err := os.ReadDir(filepath.Join(old.Work(), "cache"))
	if err != nil || len(cache) != 70-32 {
		t.Fatalf("bounded removal emptied %d entries, want one page of 32: %v", 70-len(cache), err)
	}
	for i := 0; i < 5 && exists(t, filepath.Join(old.Work(), "cache")); i++ {
		if _, err := s.Sweep(DefaultPolicy(), true); err != nil && !errors.Is(err, errRefilled) {
			t.Fatal(err)
		}
	}
	if exists(t, filepath.Join(old.Work(), "cache")) {
		t.Fatal("later sweeps did not finish the bounded removal")
	}
}

// A finished task's work dir that becomes a link to a live task's work after
// planning is caught again right before removal, on the reclaim-work path too.
//
// regression: #5560
func TestScratchFinishedLinkAtDelete(t *testing.T) {
	s := scratchStore(t)
	p := DefaultPolicy()
	live := scratchRun(t, s)
	plantFiles(t, live.Work(), "notes.txt", "checkout-copy/main.go")
	older, swapped := finishedOld(t, s, 1), finishedOld(t, s, 0)
	backdate(t, s, older, time.Hour)
	original, once := s.remove, false
	s.remove = func(path string, seen pins) error {
		if !once {
			once = true
			must(t, os.Rename(swapped.Work(), swapped.Work()+".moved"))
			linkWork(t, swapped, live)
		}
		return original(path, seen)
	}
	rows := rowsByID(sweep(t, s, p, true))
	if rows[older.Record.ID].Action != "reclaim-work" || exists(t, filepath.Join(older.Work(), "notes.txt")) {
		t.Fatal("unchanged finished task was not reclaimed", rows[older.Record.ID])
	}
	if row := rows[swapped.Record.ID]; row.Action != "keep" || row.Reason != "workspace is not a real directory at deletion" {
		t.Fatal("finished workspace swapped to a link was not caught at deletion", row)
	}
	for _, kept := range []string{"build", "notes.txt", "checkout-copy/main.go"} {
		if !exists(t, filepath.Join(live.Work(), kept)) {
			t.Fatal("live task lost work through a swapped link", kept)
		}
	}
}
