package main

// The scheduled gc: every running `cellctl desk` supervisor, interactive or cadenced,
// runs `cellctl gc <cell> --role <role> --apply --scheduled` for its OWN desk on
// CELL_GC_INTERVAL.
//
// The supervisor EXECUTES the installed cellctl as a child, resolved afresh at every tick
// (gcToolPath: DESK_TOOLS_BIN, then PATH). It never calls the maintenance in-process, so a
// re-pinned install's maintenance code runs at the next tick without restarting a window.
//
// Desk-level, not cell-level: a desk may run alone (one desk per container, with no shared
// host filesystem and no sibling supervisor), so no desk depends on another to clean for it.
// Each pass covers what this desk's own filesystem holds. Where stores ARE shared (several
// desks on one host share the cell's scratch root, the managed Go cache and the repository
// that deskwt prune walks), mutual exclusion is the existing per-store lock each entry point
// already takes; the stores' own stamps (scratch maintenance.stamp, the prune singleton's
// TTL, the cache's recorded measurement) only save repeated work.
//
// The one stamp gc adds, run/gc/<scope>.stamp, is per desk (scope = the role): its lock keeps
// a manual `gc --role <role> --apply` and that role's scheduled pass from overlapping, and
// its finish time spaces the role's scheduled passes one interval apart. The recency half
// fails OPEN (a missing, damaged or future stamp is no stamp); the lock half fails CLOSED.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/cellrunner"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const (
	gcIntervalDefault = time.Hour
	gcStampSchema     = "cellctl-gc-stamp-v1"
	gcLogLimit        = 1 << 20
)

var (
	// gcIntervalFloor keeps a mistyped CELL_GC_INTERVAL from becoming a busy loop.
	gcIntervalFloor = time.Minute
	// gcFirstDelay is how long a supervisor waits after launch before its first check, so
	// a gc never competes with the launch's own admission and scratch transaction.
	gcFirstDelay = 10 * time.Minute
	// gcCheckEvery bounds how long a supervisor waits between checks of its stamp.
	gcCheckEvery = 10 * time.Minute
)

// gcInterval reads CELL_GC_INTERVAL: a duration of at least one minute, or 0 for off.
func (c *Cell) gcInterval() (time.Duration, error) {
	if c.Env == nil {
		return gcIntervalDefault, nil
	}
	raw := c.Env.Get("CELL_GC_INTERVAL")
	if raw == "" {
		return gcIntervalDefault, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value < 0 || value > 0 && value < gcIntervalFloor {
		return 0, fmt.Errorf("CELL_GC_INTERVAL must be 0 (off) or a duration of at least %s", gcIntervalFloor)
	}
	return value, nil
}

// gcStampPath is the desk's own stamp; scope is the role, or "cell" for a pass with no role.
func (c *Cell) gcStampPath(scope string) string {
	return filepath.Join(c.Dir, "run", "gc", scope+".stamp")
}

type gcStamp struct {
	Schema     string `json:"schema"`
	PID        int    `json:"pid"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Clean      bool   `json:"clean"`
}

// readGCStamp returns the stamp at path, or nil for anything unusable (fail open).
func readGCStamp(path string) *gcStamp {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return nil
	}
	var st gcStamp
	if json.Unmarshal(b, &st) != nil || st.Schema != gcStampSchema {
		return nil
	}
	return &st
}

// finishedWithin reports whether the stamp records a pass that finished less than ttl ago.
// A missing, unparseable or future finish time is not within.
func (st *gcStamp) finishedWithin(ttl time.Duration, now time.Time) (bool, time.Duration) {
	if st == nil || ttl <= 0 || st.FinishedAt == "" {
		return false, 0
	}
	fin, err := time.Parse(time.RFC3339Nano, st.FinishedAt)
	if err != nil {
		return false, 0
	}
	age := now.Sub(fin)
	if age < 0 || age >= ttl {
		return false, 0
	}
	return true, age
}

type gcLock struct {
	file    *os.File
	started time.Time
}

type gcLockOutcome int

const (
	gcLockAcquired gcLockOutcome = iota
	gcLockHeld
	gcLockRecent
	gcLockUnavailable
)

// acquireGCLock takes the cell's gc lock. ttl > 0 adds the recency check (scheduled form).
func acquireGCLock(path string, ttl time.Duration, now time.Time) (*gcLock, gcLockOutcome, string) {
	if err := gcCustody(path); err != nil {
		return nil, gcLockUnavailable, "gc lock unavailable: " + err.Error()
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, gcLockUnavailable, "gc lock unavailable: " + err.Error()
	}
	if err := deskkit.TryLockExclusive(f); err != nil {
		_ = f.Close()
		if st := readGCStamp(path); st != nil {
			return nil, gcLockHeld, fmt.Sprintf("another gc of this scope holds its lock (pid %d, started %s)", st.PID, st.StartedAt)
		}
		return nil, gcLockHeld, "another gc of this scope holds its lock"
	}
	if within, age := readGCStamp(path).finishedWithin(ttl, now); within {
		_ = deskkit.UnlockFile(f)
		_ = f.Close()
		return nil, gcLockRecent, fmt.Sprintf("last gc finished %s ago (under the %s interval)", age.Round(time.Second), ttl)
	}
	l := &gcLock{file: f, started: now}
	l.write(gcStamp{Schema: gcStampSchema, PID: os.Getpid(), StartedAt: now.UTC().Format(time.RFC3339Nano)})
	return l, gcLockAcquired, ""
}

// write replaces the stamp's contents, best effort: a stamp that cannot be written costs
// a later pass its debounce, never this pass its run.
func (l *gcLock) write(st gcStamp) {
	b, err := json.Marshal(st)
	if err != nil {
		return
	}
	if _, err = l.file.Seek(0, 0); err != nil || l.file.Truncate(0) != nil {
		return
	}
	_, _ = l.file.Write(append(b, '\n'))
	_ = l.file.Sync()
}

// release records the finish and drops the lock. A pass with a failed target still records
// its finish: the failure is retried at the next interval, never by every supervisor at once.
func (l *gcLock) release(clean bool, now time.Time) {
	l.write(gcStamp{Schema: gcStampSchema, PID: os.Getpid(), StartedAt: l.started.UTC().Format(time.RFC3339Nano),
		FinishedAt: now.UTC().Format(time.RFC3339Nano), Clean: clean})
	_ = deskkit.UnlockFile(l.file)
	_ = l.file.Close()
}

// startGCSchedule runs beside a supervised child, like the scratch timer it replaces: the
// first check waits gcFirstDelay, later ones every min(gcCheckEvery, interval). stop
// cancels and waits, so no gc is started by, or outlives, a stopping supervisor.
func (c *Cell) startGCSchedule(parent context.Context, role string, interval time.Duration, log io.Writer) (stop func()) {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		wait := gcFirstDelay
		for {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			wait = min(gcCheckEvery, interval)
			c.scheduledGCTick(ctx, role, interval, log)
		}
	}()
	return func() { cancel(); <-done }
}

// scheduledGCTick starts one child gc for this desk unless the supervisor is stopping (its
// context is done, or a stop flag covers its role) or the role's stamp shows its last pass
// is fresh.
func (c *Cell) scheduledGCTick(ctx context.Context, role string, interval time.Duration, log io.Writer) (started bool) {
	if ctx.Err() != nil || c.cadenceGuard(role) != nil {
		return false
	}
	if within, _ := readGCStamp(c.gcStampPath(role)).finishedWithin(interval, time.Now()); within {
		return false
	}
	began := time.Now()
	record := struct {
		Time    string `json:"time"`
		Role    string `json:"role"`
		Cellctl string `json:"cellctl,omitempty"`
		Exit    int    `json:"exit"`
		Seconds int64  `json:"seconds"`
		Error   string `json:"error,omitempty"`
	}{Time: began.UTC().Format(time.RFC3339), Role: role, Exit: -1}
	out := c.openGCLog(role)
	defer func() {
		record.Seconds = int64(time.Since(began).Seconds())
		b, _ := json.Marshal(record)
		line := fmt.Sprintf("gc-schedule %s\n", b)
		_, _ = io.WriteString(out, line)
		if log != nil {
			_, _ = io.WriteString(log, line)
		}
		if f, ok := out.(*os.File); ok {
			_ = f.Close()
		}
	}()
	bin, err := c.gcToolPath("cellctl")
	if err != nil {
		record.Error = "could-not-check: " + err.Error()
		return false
	}
	record.Cellctl = bin
	cellsRoot, name := c.reenter()
	env := envSet(envUnset(os.Environ(), cellcache.EnvPolicy), "CELLS_ROOT", cellsRoot)
	var stdout io.Writer = out
	if log != nil {
		stdout = io.MultiWriter(out, log)
	}
	code, uncertain, runErr := cellrunner.Run(ctx, []string{bin, "gc", name, "--role", role, "--apply", "--scheduled"}, env, c.Dir, stdout, out)
	record.Exit = code
	if runErr != nil {
		record.Error = runErr.Error()
	} else if uncertain {
		record.Error = "child cleanup uncertain"
	}
	return true
}

// gcCustody prepares a gc stamp or log path: its directory (run/gc) is a private real
// directory, and an existing file is an owner-only regular file reached through no link,
// the custody the cell's admission lock already has (privateCommsFile).
func gcCustody(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := privateCommsDir(dir); err != nil {
		return fmt.Errorf("gc directory custody: %w", err)
	}
	if _, err := os.Lstat(path); err == nil {
		if err := privateCommsFile(path); err != nil {
			return fmt.Errorf("gc file custody: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

// gcLogPath is the desk's own log; one log per role keeps two desks sharing a cell
// directory from rotating the same file.
func (c *Cell) gcLogPath(role string) string { return filepath.Join(c.Dir, "run", "gc", role+".log") }

// openGCLog opens the desk's gc log for append, keeping one previous generation past 1 MiB.
// A log that cannot be opened, or fails its custody check, discards: logging never stops
// maintenance.
func (c *Cell) openGCLog(role string) io.Writer {
	path := c.gcLogPath(role)
	if gcCustody(path) != nil {
		return io.Discard
	}
	if st, err := os.Lstat(path); err == nil && st.Size() > gcLogLimit {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return io.Discard
	}
	return f
}
