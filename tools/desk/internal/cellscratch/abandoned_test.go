package cellscratch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// plantBuild writes the layout an abandoned agent session leaves behind: compiler
// and test-binary output beside ordinary work and a captured output tail.
func plantBuild(t *testing.T, r *Run) {
	t.Helper()
	for name, size := range map[string]int{"go-build1/b001/x.a": 1000, "cellctl-bin-2/cellctl": 500, "deskpins-bin-3/deskpins": 700} {
		path := filepath.Join(r.Work(), name)
		must(t, os.MkdirAll(filepath.Dir(path), 0700))
		must(t, os.WriteFile(path, []byte(strings.Repeat("b", size)), 0600))
	}
	if _, err := r.Tail().Write([]byte("last output")); err != nil {
		t.Fatal(err)
	}
}

// backdate moves a task's recorded activity into the past, as a session abandoned days ago.
func backdate(t *testing.T, s *Store, r *Run, d time.Duration) {
	t.Helper()
	rec, err := s.record(r.Record.ID)
	must(t, err)
	rec.Created = rec.Created.Add(-d)
	if !rec.Finished.IsZero() {
		rec.Finished = rec.Finished.Add(-d)
	}
	rec.Reaped = true
	must(t, s.save(rec))
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// regression: #5353
func TestScratchAbandonedBuild(t *testing.T) {
	s := scratchStore(t)
	p := DefaultPolicy()
	old := scratchRun(t, s)
	plantBuild(t, old)
	finish(t, old, 1)
	backdate(t, s, old, p.MaxAge+time.Hour)
	fresh := scratchRun(t, s)
	plantBuild(t, fresh)
	finish(t, fresh, 1)
	live := scratchRun(t, s)
	plantBuild(t, live)
	backdate(t, s, live, p.MaxAge+time.Hour)
	running := scratchRun(t, s)
	plantBuild(t, running)
	must(t, running.Finish(1, false))
	must(t, running.Close())
	backdate(t, s, running, p.MaxAge+time.Hour)
	rec, err := s.record(running.Record.ID)
	must(t, err)
	rec.Reaped = false
	must(t, s.save(rec))
	before, err := s.record(old.Record.ID)
	must(t, err)

	dry := sweep(t, s, p, false)
	rows := map[string]Entry{}
	for _, e := range dry.Entries {
		rows[e.ID] = e
	}
	if rows[old.Record.ID].Action != "reclaim-build" || !exists(t, filepath.Join(old.Work(), "go-build1")) {
		t.Fatalf("dry run did not plan or did delete abandoned build output: %+v", dry)
	}
	got := sweep(t, s, p, true)
	if got.Reclaimed != 1500 || got.Partial {
		t.Fatalf("abandoned build reclaim=%d partial=%v: %+v", got.Reclaimed, got.Partial, got)
	}
	for _, gone := range []string{"go-build1", "cellctl-bin-2"} {
		if exists(t, filepath.Join(old.Work(), gone)) {
			t.Fatal("abandoned build output retained", gone)
		}
	}
	for _, kept := range []string{filepath.Join(old.Work(), "build"), filepath.Join(old.Work(), "deskpins-bin-3"), filepath.Join(s.Path, old.Record.ID, "record.json"), filepath.Join(s.Path, old.Record.ID, "diagnostic.txt")} {
		if !exists(t, kept) {
			t.Fatal("abandoned task lost more than build output", kept)
		}
	}
	after, err := s.record(old.Record.ID)
	if err != nil || after != before {
		t.Fatalf("abandoned record rewritten: %+v %v", after, err)
	}
	for name, r := range map[string]*Run{"fresh": fresh, "live": live, "unreaped": running} {
		if !exists(t, filepath.Join(r.Work(), "go-build1")) || !exists(t, filepath.Join(r.Work(), "cellctl-bin-2")) {
			t.Fatal("build output reclaimed from a protected task:", name)
		}
	}
	if again := sweep(t, s, p, true); again.Reclaimed != 0 {
		t.Fatal("second sweep reclaimed again", again)
	}
}

// regression: #5353
func TestScratchSweepIfDue(t *testing.T) {
	s := scratchStore(t)
	p := DefaultPolicy()
	if _, ran, err := s.SweepIfDue(p, 0, true); ran || err != nil {
		t.Fatal("zero interval swept", ran, err)
	}
	if _, ran, err := s.SweepIfDue(p, time.Hour, true); !ran || err != nil {
		t.Fatal("first periodic sweep skipped", ran, err)
	}
	if _, ran, err := s.SweepIfDue(p, time.Hour, true); ran || err != nil {
		t.Fatal("sibling supervisor swept again inside the interval", ran, err)
	}
	if _, ran, err := s.SweepIfDue(p, time.Nanosecond, true); !ran || err != nil {
		t.Fatal("elapsed interval skipped", ran, err)
	}
	must(t, os.WriteFile(filepath.Join(s.Path, maintenanceStampName), []byte("damaged"), 0600))
	if _, ran, err := s.SweepIfDue(p, time.Hour, true); !ran || err != nil {
		t.Fatal("damaged stamp blocked maintenance", ran, err)
	}
	f, err := os.OpenFile(filepath.Join(s.Path, maintenanceLockName), os.O_CREATE|os.O_RDWR, 0600)
	must(t, err)
	defer f.Close()
	must(t, deskkit.TryLockExclusive(f))
	must(t, os.Remove(filepath.Join(s.Path, maintenanceStampName)))
	if _, ran, err := s.SweepIfDue(p, time.Hour, true); ran || err != nil {
		t.Fatal("busy maintenance lease was not a quiet skip", ran, err)
	}
	must(t, f.Close())
	if rep := sweep(t, s, p, false); len(rep.Entries) != 0 {
		t.Fatal("stamp entered the inventory", rep)
	}
}

// regression: #5353
func TestScratchSweepStops(t *testing.T) {
	s := scratchStore(t)
	r := scratchRun(t, s)
	acknowledge(t, r)
	finish(t, r, 0)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Bind(ctx)
	rep, ran, err := s.SweepIfDue(DefaultPolicy(), time.Hour, true)
	if !ran || err == nil || !rep.Partial || rep.Reclaimed != 0 || !exists(t, r.Work()) {
		t.Fatalf("stopped sweep reclaimed: ran=%v err=%v %+v", ran, err, rep)
	}
}
