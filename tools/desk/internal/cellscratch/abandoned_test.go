package cellscratch

import (
	"context"
	"errors"
	"io"
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
	for _, name := range []string{"go-builder/main.go", "go-build-notes", "cellctl-bin-/x", "-bin-x/y", "tool-bin/x"} {
		path := filepath.Join(old.Work(), name)
		must(t, os.MkdirAll(filepath.Dir(path), 0700))
		must(t, os.WriteFile(path, []byte("ordinary work"), 0600))
	}
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
	if got.Reclaimed != 2200 || got.Partial {
		t.Fatalf("abandoned build reclaim=%d partial=%v: %+v", got.Reclaimed, got.Partial, got)
	}
	for _, gone := range []string{"go-build1", "cellctl-bin-2", "deskpins-bin-3"} {
		if exists(t, filepath.Join(old.Work(), gone)) {
			t.Fatal("abandoned build output retained", gone)
		}
	}
	for _, kept := range []string{filepath.Join(old.Work(), "build"), filepath.Join(old.Work(), "go-builder"), filepath.Join(old.Work(), "go-build-notes"), filepath.Join(old.Work(), "cellctl-bin-"), filepath.Join(old.Work(), "-bin-x"), filepath.Join(old.Work(), "tool-bin"), filepath.Join(s.Path, old.Record.ID, "record.json"), filepath.Join(s.Path, old.Record.ID, "diagnostic.txt")} {
		if !exists(t, kept) {
			t.Fatal("abandoned task lost more than build output", kept)
		}
	}
	after, err := s.record(old.Record.ID)
	if err != nil || after != before {
		t.Fatalf("abandoned record rewritten: %+v %v", after, err)
	}
	for name, r := range map[string]*Run{"fresh": fresh, "live": live, "unreaped": running} {
		for _, kept := range []string{"go-build1", "cellctl-bin-2", "deskpins-bin-3"} {
			if !exists(t, filepath.Join(r.Work(), kept)) {
				t.Fatal("build output reclaimed from a protected task:", name, kept)
			}
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
	if exists(t, filepath.Join(s.Path, maintenanceStampName)) {
		t.Fatal("stopped sweep stamped the interval as done")
	}
	if exists(t, filepath.Join(s.Path, maintenanceStampName+".attempt")) {
		t.Fatal("stopped sweep recorded a failed attempt that delays the next supervisor")
	}
}

// Only a finished sweep stamps the interval, and only after its work: a partial pass
// must not hold sibling supervisors off for a whole interval, only for the retry delay.
//
// regression: #5353
func TestScratchSweepIfDueStampsCompletion(t *testing.T) {
	s := scratchStore(t)
	r := scratchRun(t, s)
	acknowledge(t, r)
	finish(t, r, 0)
	stamp := filepath.Join(s.Path, maintenanceStampName)
	original := s.remove
	s.remove = func(string, pins) error { return errors.New("injected deletion failure") }
	rep, ran, err := s.SweepIfDue(DefaultPolicy(), time.Hour, true)
	if !ran || err == nil || !rep.Partial {
		t.Fatalf("injected failure did not leave a partial sweep: ran=%v err=%v %+v", ran, err, rep)
	}
	if exists(t, stamp) {
		t.Fatal("partial sweep stamped the interval as done")
	}
	if _, ran, err = s.SweepIfDue(DefaultPolicy(), time.Hour, true); ran || err != nil {
		t.Fatal("sibling retried at once instead of after the retry delay", ran, err)
	}
	backdateAttempt(t, s, 11*time.Minute)
	stampedEarly := false
	s.remove = func(path string, seen pins) error {
		stampedEarly = stampedEarly || exists(t, stamp)
		return original(path, seen)
	}
	rep, ran, err = s.SweepIfDue(DefaultPolicy(), time.Hour, true)
	if !ran || err != nil || rep.Partial || rep.Reclaimed == 0 {
		t.Fatalf("sibling retry after a partial sweep did not run: ran=%v err=%v %+v", ran, err, rep)
	}
	if stampedEarly || !exists(t, stamp) || exists(t, stamp+".attempt") {
		t.Fatalf("stamp not written (or failed attempt kept) after the finished sweep: early=%v", stampedEarly)
	}
	if _, ran, err = s.SweepIfDue(DefaultPolicy(), time.Hour, true); ran || err != nil {
		t.Fatal("finished sweep did not hold siblings off", ran, err)
	}
}

// The supervisor's maintenance handle gives up on a busy sweep lock well before a
// launch would, and never writes the lock-wait line to a terminal the child may own.
//
// regression: #5353
func TestScratchOpenMaintenanceQuiet(t *testing.T) {
	s := scratchStore(t)
	held, err := s.sweepLock()
	must(t, err)
	defer func(launch, maintenance time.Duration) {
		SweepLockWait, MaintenanceLockWait = launch, maintenance
	}(SweepLockWait, MaintenanceLockWait)
	SweepLockWait, MaintenanceLockWait = 5*time.Second, 200*time.Millisecond
	began := time.Now()
	if o, err := OpenMaintenance(context.Background(), s.Path); !errors.Is(err, deskkit.ErrLockBusy) {
		if err == nil {
			o.Close()
		}
		t.Fatal("busy sweep lock not reported to periodic maintenance", err)
	}
	if waited := time.Since(began); waited > 2*time.Second {
		t.Fatal("periodic maintenance waited like a launch", waited)
	}
	MaintenanceLockWait = 5 * time.Second
	read, write, err := os.Pipe()
	must(t, err)
	stderr := os.Stderr
	os.Stderr = write
	defer func() { os.Stderr = stderr }()
	time.AfterFunc(1200*time.Millisecond, func() { _ = held.Release() })
	o, err := OpenMaintenance(context.Background(), s.Path)
	os.Stderr = stderr
	must(t, write.Close())
	out, readErr := io.ReadAll(read)
	must(t, readErr)
	must(t, err)
	must(t, o.Close())
	if strings.Contains(string(out), "deskkit: lock") {
		t.Fatalf("lock-wait line written to stderr: %q", out)
	}
}

// linkWork replaces a task's work with a link to a sibling's work. os.Root follows a
// link that stays inside the scratch root, so only an explicit Lstat sees it.
func linkWork(t *testing.T, r, sibling *Run) {
	t.Helper()
	must(t, os.Symlink(filepath.Join("..", sibling.Record.ID, "work"), r.Work()))
}

// An abandoned task whose work is a link to a live task's work must neither plan nor
// delete the live task's build output.
//
// regression: #5353
func TestScratchAbandonedBuildLink(t *testing.T) {
	s := scratchStore(t)
	p := DefaultPolicy()
	live := scratchRun(t, s)
	plantBuild(t, live)
	linked := scratchRun(t, s)
	finish(t, linked, 1)
	backdate(t, s, linked, p.MaxAge+time.Hour)
	must(t, os.RemoveAll(linked.Work()))
	linkWork(t, linked, live)
	for _, apply := range []bool{false, true} {
		got := sweep(t, s, p, apply)
		for _, e := range got.Entries {
			if e.ID == linked.Record.ID && (e.Action != "keep" || e.Reason != "workspace is not a real directory; retained") {
				t.Fatalf("linked workspace planned for reclaim (apply=%v): %+v", apply, e)
			}
		}
		if got.Reclaimed != 0 {
			t.Fatal("reclaimed through a linked workspace", got)
		}
	}
	for _, kept := range []string{"go-build1", "cellctl-bin-2", "deskpins-bin-3"} {
		if !exists(t, filepath.Join(live.Work(), kept)) {
			t.Fatal("live task lost build output through a sibling's link", kept)
		}
	}
}

// A workspace that becomes a link after planning is caught again right before removal.
//
// regression: #5353
func TestScratchAbandonedBuildLinkAtDeletion(t *testing.T) {
	s := scratchStore(t)
	p := DefaultPolicy()
	live := scratchRun(t, s)
	plantBuild(t, live)
	older, swapped := scratchRun(t, s), scratchRun(t, s)
	for _, r := range []*Run{older, swapped} {
		plantBuild(t, r)
		finish(t, r, 1)
	}
	backdate(t, s, older, p.MaxAge+2*time.Hour)
	backdate(t, s, swapped, p.MaxAge+time.Hour)
	original, once := s.remove, false
	s.remove = func(path string, seen pins) error {
		if !once {
			once = true
			must(t, os.Rename(swapped.Work(), swapped.Work()+".moved"))
			linkWork(t, swapped, live)
		}
		return original(path, seen)
	}
	got := sweep(t, s, p, true)
	rows := map[string]Entry{}
	for _, e := range got.Entries {
		rows[e.ID] = e
	}
	if rows[older.Record.ID].Action != "reclaim-build" || exists(t, filepath.Join(older.Work(), "go-build1")) {
		t.Fatal("unchanged abandoned task was not reclaimed", rows[older.Record.ID])
	}
	if row := rows[swapped.Record.ID]; row.Action != "keep" || row.Reason != "workspace is not a real directory at deletion" {
		t.Fatal("workspace swapped to a link was not caught at deletion", row)
	}
	for _, kept := range []string{"go-build1", "cellctl-bin-2", "deskpins-bin-3"} {
		if !exists(t, filepath.Join(live.Work(), kept)) {
			t.Fatal("live task lost build output through a swapped link", kept)
		}
	}
}
