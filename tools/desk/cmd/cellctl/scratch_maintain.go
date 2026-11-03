package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellscratch"
)

// scratchSweepDelay is how long a supervisor waits after its launch before its first
// periodic scratch sweep, so recursive maintenance never competes with admission.
var scratchSweepDelay = 10 * time.Minute

const (
	scratchSweepIntervalDefault = 6 * time.Hour
	scratchMaintenanceLogLimit  = 1 << 20
)

// scratchSweepIntervalFloor keeps a mistyped CELL_SCRATCH_SWEEP_INTERVAL from turning
// maintenance into a busy loop. Tests lower it to observe several sweeps.
var scratchSweepIntervalFloor = time.Minute

// scratchSweepInterval reads CELL_SCRATCH_SWEEP_INTERVAL from the existing cell.env:
// a duration of at least one minute, or 0 to turn periodic maintenance off.
func (c *Cell) scratchSweepInterval() (time.Duration, error) {
	if c.Env == nil {
		return scratchSweepIntervalDefault, nil
	}
	raw := c.Env.Get("CELL_SCRATCH_SWEEP_INTERVAL")
	if raw == "" {
		return scratchSweepIntervalDefault, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value < 0 || value > 0 && value < scratchSweepIntervalFloor {
		return 0, fmt.Errorf("CELL_SCRATCH_SWEEP_INTERVAL must be 0 (off) or a duration of at least %s", scratchSweepIntervalFloor)
	}
	return value, nil
}

// startScratchMaintenance is the only scheduler of scratch maintenance. It runs beside
// a supervised child, never on the launch path: the first sweep waits
// scratchSweepDelay, later ones follow the interval, and the stamp under the
// maintenance lease coalesces sibling supervisors onto one finished sweep per
// interval. After a sweep it ran, this supervisor waits a whole interval even when
// that sweep failed; a sibling checking sooner runs the retry once the store's
// retry delay has passed. stop
// cancels and waits, so no sweep outlives or follows the child it was started for.
// Failures are logged and recorded; they never stop the desk.
func (c *Cell) startScratchMaintenance(parent context.Context, role string, log io.Writer) (stop func()) {
	interval, err := c.scratchSweepInterval()
	if err != nil || interval == 0 {
		return func() {}
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		wait := scratchSweepDelay
		for {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			wait = min(scratchSweepDelay, interval)
			if ran, _ := c.deferredScratchSweep(ctx, role, interval, log); ran {
				wait = interval
			}
		}
	}()
	return func() { cancel(); <-done }
}

// deferredScratchSweep runs one periodic sweep on its own store handle, so the
// launch's admission handle never walks history. The maintenance handle bounds its
// lock waits and writes no lock-wait lines to a terminal the child may own.
func (c *Cell) deferredScratchSweep(ctx context.Context, role string, interval time.Duration, log io.Writer) (ran bool, err error) {
	began := time.Now()
	var rep cellscratch.Report
	defer func() {
		if value := recover(); value != nil {
			err = fmt.Errorf("scratch maintenance panic: %v", value)
		}
		if ran || err != nil {
			c.recordLaunchStep("scratch-maintenance", time.Since(began), err)
			c.logScratchMaintenance(role, rep, ran, err, log)
		}
	}()
	p, err := c.scratchPolicy()
	if err != nil {
		return false, err
	}
	s, err := cellscratch.OpenMaintenance(ctx, filepath.Join(c.Dir, "run", "scratch"))
	if err != nil {
		return false, err
	}
	s.Label(c.Name, role)
	rep, ran, err = s.SweepIfDue(p, interval, true)
	return ran, errors.Join(err, s.Close())
}

// logScratchMaintenance appends one summary line to run/scratch-maintenance.log, beside
// (not inside) the scratch root, and copies it to log when the supervisor owns its
// stream. An interactive terminal belongs to the child, so it gets nothing.
func (c *Cell) logScratchMaintenance(role string, rep cellscratch.Report, ran bool, err error, log io.Writer) {
	actions := map[string]int{}
	for _, e := range rep.Entries {
		actions[e.Action]++
	}
	line := struct {
		Time       string         `json:"time"`
		Role       string         `json:"role"`
		Ran        bool           `json:"ran"`
		Bytes      int64          `json:"bytes"`
		Reclaimed  int64          `json:"reclaimed"`
		Partial    bool           `json:"partial"`
		OverBudget bool           `json:"over_budget"`
		Actions    map[string]int `json:"actions"`
		Error      string         `json:"error,omitempty"`
	}{time.Now().UTC().Format(time.RFC3339), role, ran, rep.Bytes, rep.Reclaimed, rep.Partial, rep.OverBudget, actions, ""}
	if err != nil {
		line.Error = err.Error()
	}
	b, _ := json.Marshal(line)
	b = append(b, '\n')
	path := filepath.Join(c.Dir, "run", "scratch-maintenance.log")
	if st, statErr := os.Stat(path); statErr == nil && st.Size() > scratchMaintenanceLogLimit {
		_ = os.Rename(path, path+".1") // one previous generation bounds the log
	}
	if f, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); openErr == nil {
		_, _ = f.Write(b)
		_ = f.Close()
	}
	if log != nil {
		_, _ = fmt.Fprintf(log, "scratch-maintenance %s", b)
	}
}
