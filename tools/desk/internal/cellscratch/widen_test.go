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
			s.remove = func(path string) error {
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
				return original(path)
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
			s.remove = func(path string) error {
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
				return original(path)
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
	s.remove = func(string) error { return errors.New("injected deletion failure") }
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
