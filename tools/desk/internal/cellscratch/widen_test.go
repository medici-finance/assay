package cellscratch

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// abandonedRuns plants build output in n abandoned tasks, the first one oldest, so a
// sweep handles them in that order.
func abandonedRuns(t *testing.T, s *Store, n int) []*Run {
	t.Helper()
	var runs []*Run
	for i := 0; i < n; i++ {
		r := scratchRun(t, s)
		plantBuild(t, r)
		finish(t, r, 1)
		backdate(t, s, r, DefaultPolicy().MaxAge+time.Duration(n-i)*time.Hour)
		runs = append(runs, r)
	}
	return runs
}

func plantFiles(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(dir, name)
		must(t, os.MkdirAll(filepath.Dir(path), 0700))
		must(t, os.WriteFile(path, []byte("kept"), 0600))
	}
}

func rowsByID(rep Report) map[string]Entry {
	rows := map[string]Entry{}
	for _, e := range rep.Entries {
		rows[e.ID] = e
	}
	return rows
}

// Build output beside a Git worktree is reclaimable; the worktree and any build-named
// entry holding Git data stay.
//
// regression: #5353
func TestScratchBuildGitEntry(t *testing.T) {
	s := scratchStore(t)
	r := abandonedRuns(t, s, 1)[0]
	plantFiles(t, r.Work(), "checkout/.git/HEAD", "checkout/main.go", "repo-bin-4/src/.git", "repo-bin-4/out")
	got := sweep(t, s, DefaultPolicy(), true)
	if got.Reclaimed != 2200 || got.Partial {
		t.Fatalf("build output beside a worktree reclaim=%d partial=%v: %+v", got.Reclaimed, got.Partial, got)
	}
	for _, gone := range []string{"go-build1", "cellctl-bin-2", "deskpins-bin-3"} {
		if exists(t, filepath.Join(r.Work(), gone)) {
			t.Fatal("build output beside a worktree retained", gone)
		}
	}
	for _, kept := range []string{"checkout/.git/HEAD", "checkout/main.go", "repo-bin-4/src/.git", "repo-bin-4/out", "build"} {
		if !exists(t, filepath.Join(r.Work(), kept)) {
			t.Fatal("Git-holding entry reclaimed", kept)
		}
	}
}

// An entry that gains Git data, or cannot be inspected, after planning is caught by
// the per-entry recheck right before removal, and its whole task is kept.
func TestScratchBuildRecheck(t *testing.T) {
	for _, kind := range []string{"git", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			s := scratchStore(t)
			runs := abandonedRuns(t, s, 2)
			later := runs[1]
			entry := filepath.Join(later.Work(), "cellctl-bin-2")
			original, once := s.remove, false
			s.remove = func(path string, seen pins) error {
				if !once {
					once = true
					locked := filepath.Join(entry, "locked")
					must(t, os.Mkdir(locked, 0700))
					if kind == "git" {
						plantFiles(t, locked, ".git/HEAD")
					} else {
						must(t, os.Chmod(locked, 0))
						t.Cleanup(func() { _ = os.Chmod(locked, 0700) })
					}
				}
				return original(path, seen)
			}
			got, _ := s.Sweep(DefaultPolicy(), true)
			row := rowsByID(got)[later.Record.ID]
			if row.Action != "keep" || row.Reason != "Git retention or unknown tree at deletion" {
				t.Fatal("changed entry not caught before removal", row)
			}
			for _, kept := range []string{"cellctl-bin-2/cellctl", "go-build1"} {
				if !exists(t, filepath.Join(later.Work(), kept)) {
					t.Fatal("task lost build output after its recheck failed", kept)
				}
			}
			if kind == "unreadable" && !got.Partial {
				t.Fatal("uninspectable entry reported as a clean sweep", got)
			}
		})
	}
}

// A path component swapped for a link after the deletion check, while the sweep lock
// is released, must not redirect the removal into a live sibling's files.
//
// regression: #5353
func TestScratchRemoveSwapRace(t *testing.T) {
	for _, kind := range []string{"build", "compact"} {
		t.Run(kind, func(t *testing.T) {
			s := scratchStore(t)
			live := scratchRun(t, s)
			plantBuild(t, live)
			var victim *Run
			if kind == "build" {
				victim = abandonedRuns(t, s, 1)[0]
			} else {
				victim = scratchRun(t, s)
				acknowledge(t, victim)
				finish(t, victim, 1)
			}
			original, once := s.remove, false
			s.remove = func(path string, seen pins) error {
				if !once {
					once = true
					if kind == "build" {
						must(t, os.Rename(victim.Work(), victim.Work()+".moved"))
						linkWork(t, victim, live)
					} else {
						task := filepath.Join(s.Path, victim.Record.ID)
						must(t, os.Rename(task, task+".moved"))
						must(t, os.Symlink(live.Record.ID, task))
					}
				}
				return original(path, seen)
			}
			got, err := s.Sweep(DefaultPolicy(), true)
			for _, kept := range []string{"build", "go-build1", "cellctl-bin-2", "deskpins-bin-3"} {
				if !exists(t, filepath.Join(live.Work(), kept)) {
					t.Fatal("removal redirected into a live task through a swapped link", kept)
				}
			}
			if err == nil || !got.Partial {
				t.Fatalf("refused removal reported as clean: err=%v %+v", err, got)
			}
		})
	}
}

// A real directory or file renamed onto a planned path after the deletion check, while
// the sweep lock is released, is not the one the sweep planned: the removal fails on the
// pinned identity instead of deleting the live task's files that now sit there. A
// planned entry moved under a fresh directory of its parent's name is refused too: the
// removal walks every pinned directory on open handles, not the joined path.
//
// regression: #5353
func TestScratchRemoveRenameRace(t *testing.T) {
	for _, kind := range []string{"task", "work", "entry", "parent"} {
		t.Run(kind, func(t *testing.T) {
			s := scratchStore(t)
			live := scratchRun(t, s)
			plantBuild(t, live)
			var victim *Run
			if kind == "task" {
				victim = scratchRun(t, s)
				acknowledge(t, victim)
				finish(t, victim, 1)
			} else {
				victim = abandonedRuns(t, s, 1)[0]
			}
			// Where the live files sit once renamed onto the victim's planned path.
			moved := live.Work()
			original, once := s.remove, false
			s.remove = func(path string, seen pins) error {
				if !once {
					once = true
					switch kind {
					case "task":
						task, other := filepath.Join(s.Path, victim.Record.ID), filepath.Join(s.Path, live.Record.ID)
						must(t, os.Rename(task, task+".moved"))
						must(t, os.Rename(other, task))
						moved = filepath.Join(task, "work")
					case "work":
						must(t, os.Rename(victim.Work(), victim.Work()+".moved"))
						must(t, os.Rename(live.Work(), victim.Work()))
						moved = victim.Work()
					case "parent":
						dir, leaf := filepath.Split(filepath.FromSlash(path))
						dir = filepath.Join(s.Path, dir)
						must(t, os.Rename(dir, dir+".moved"))
						must(t, os.Mkdir(dir, 0o700))
						must(t, os.Rename(filepath.Join(dir+".moved", leaf), filepath.Join(dir, leaf)))
						moved = filepath.Join(dir, leaf)
					default:
						entry := filepath.Join(victim.Work(), "cellctl-bin-2")
						must(t, os.Rename(entry, entry+".moved"))
						must(t, os.Rename(filepath.Join(live.Work(), "build"), entry))
					}
				}
				return original(path, seen)
			}
			got, err := s.Sweep(DefaultPolicy(), true)
			kept := []string{"build", "go-build1/b001/x.a", "cellctl-bin-2/cellctl", "deskpins-bin-3/deskpins"}
			switch kind {
			case "entry":
				kept, moved = []string{"cellctl-bin-2"}, victim.Work()
			case "parent":
				kept = []string{""}
			}
			for _, name := range kept {
				if !exists(t, filepath.Join(moved, name)) {
					t.Fatal("removal followed a renamed directory into a live task's files", name)
				}
			}
			if err == nil || !got.Partial {
				t.Fatalf("refused removal reported as clean: err=%v %+v", err, got)
			}
		})
	}
}

// A live task's work renamed onto a later candidate's work before that candidate's
// deletion check is caught by the pinned identities under the sweep lock: the task is
// kept with a reason, nothing of the live task goes, and the sweep reports partial.
//
// regression: #5353
func TestScratchRenameAtDeletion(t *testing.T) {
	s := scratchStore(t)
	live := scratchRun(t, s)
	plantBuild(t, live)
	runs := abandonedRuns(t, s, 2)
	older, renamed := runs[0], runs[1]
	original, once := s.remove, false
	s.remove = func(path string, seen pins) error {
		if !once {
			once = true
			must(t, os.Rename(renamed.Work(), renamed.Work()+".moved"))
			must(t, os.Rename(live.Work(), renamed.Work()))
		}
		return original(path, seen)
	}
	got, err := s.Sweep(DefaultPolicy(), true)
	rows := rowsByID(got)
	if rows[older.Record.ID].Action != "reclaim-build" || exists(t, filepath.Join(older.Work(), "go-build1")) {
		t.Fatal("unchanged abandoned task was not reclaimed", rows[older.Record.ID])
	}
	if row := rows[renamed.Record.ID]; row.Action != "keep" || row.Reason != "workspace changed since the sweep planned it" {
		t.Fatal("workspace renamed into place was not caught at deletion", row)
	}
	for _, kept := range []string{"build", "go-build1/b001/x.a", "cellctl-bin-2/cellctl", "deskpins-bin-3/deskpins"} {
		if !exists(t, filepath.Join(renamed.Work(), kept)) {
			t.Fatal("live task lost files renamed onto a planned workspace", kept)
		}
	}
	if err == nil || !got.Partial {
		t.Fatalf("refused deletion reported as clean: err=%v %+v", err, got)
	}
}

// The retry delay saturates at the interval: no failure count or interval, however
// large, overflows the doubling into a zero, negative or spinning delay, and a retry
// setting that is not positive falls back to the whole interval.
//
// regression: #5353
func TestScratchRetryDelayBounds(t *testing.T) {
	done := make(chan string, 1)
	go func() {
		defer close(done)
		for _, c := range []struct {
			n              int
			interval, want time.Duration
		}{
			{1, time.Hour, MaintenanceRetry},
			{3, time.Hour, 4 * MaintenanceRetry},
			{9, time.Hour, time.Hour},
			{2, MaintenanceRetry / 2, MaintenanceRetry / 2},
			{64, math.MaxInt64, math.MaxInt64},
			{200, math.MaxInt64 / 3, math.MaxInt64 / 3},
			{math.MaxInt, math.MaxInt64, math.MaxInt64},
		} {
			if got := retryDelay(c.n, c.interval); got != c.want {
				done <- fmt.Sprintf("retryDelay(%d, %d) = %d, want %d", c.n, c.interval, got, c.want)
				return
			}
		}
	}()
	select {
	case msg := <-done:
		if msg != "" {
			t.Fatal(msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("retryDelay did not return for a huge failure count and interval")
	}
	saved := MaintenanceRetry
	t.Cleanup(func() { MaintenanceRetry = saved })
	MaintenanceRetry = 0
	if got := retryDelay(3, time.Hour); got != time.Hour {
		t.Fatal("a retry setting that is not positive shortened the delay", got)
	}
}

// backdateAttempt moves the recorded failed periodic sweep d into the past.
func backdateAttempt(t *testing.T, s *Store, d time.Duration) {
	t.Helper()
	path := filepath.Join(s.Path, "maintenance.stamp.attempt")
	b, err := os.ReadFile(path)
	must(t, err)
	stamp, count, _ := strings.Cut(string(b), " ")
	at, err := time.Parse(time.RFC3339Nano, stamp)
	must(t, err)
	must(t, os.WriteFile(path, []byte(at.Add(-d).Format(time.RFC3339Nano)+" "+count), 0600))
}

// Sibling supervisors coalesce onto one retry after a failed periodic sweep, and the
// retry delay doubles from ten minutes up to the interval on each further failure.
//
// regression: #5353
func TestScratchRetryBackoff(t *testing.T) {
	s := scratchStore(t)
	r := scratchRun(t, s)
	acknowledge(t, r)
	finish(t, r, 0)
	original := s.remove
	s.remove = func(string, pins) error { return errors.New("injected deletion failure") }
	const retry, interval = 10 * time.Minute, time.Hour
	try := func(want bool, note string) {
		t.Helper()
		_, ran, err := s.SweepIfDue(DefaultPolicy(), interval, true)
		if ran != want || ran && err == nil || !ran && err != nil {
			t.Fatalf("%s: ran=%v err=%v", note, ran, err)
		}
	}
	try(true, "first periodic sweep")
	try(false, "sibling retried at once after a failed sweep")
	for i, wait := range []time.Duration{retry, 2 * retry, 4 * retry, interval, interval} {
		backdateAttempt(t, s, wait-time.Minute)
		try(false, fmt.Sprintf("failure %d retried before its %s delay", i+1, wait))
		backdateAttempt(t, s, 2*time.Minute)
		try(true, fmt.Sprintf("failure %d not retried after its %s delay", i+1, wait))
	}
	attempt := filepath.Join(s.Path, "maintenance.stamp.attempt")
	must(t, os.WriteFile(attempt, []byte("damaged"), 0600))
	try(true, "damaged attempt record blocked maintenance")
	s.remove = original
	backdateAttempt(t, s, interval)
	if _, ran, err := s.SweepIfDue(DefaultPolicy(), interval, true); !ran || err != nil {
		t.Fatal("retry after the delay did not finish", ran, err)
	}
	if exists(t, attempt) || !exists(t, filepath.Join(s.Path, maintenanceStampName)) {
		t.Fatal("finished sweep did not replace the failed attempt with the stamp")
	}
}
