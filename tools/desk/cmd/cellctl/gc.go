package main

// `cellctl gc <cell> [--role <role>]` — one maintenance pass over a desk's reclaim targets.
//
// gc adds no delete logic of its own. Each target is an existing, already-guarded entry
// point, called as its own caller calls it:
//
//   - scratch:   the explicit sweep (sweepScratch), or, for a --scheduled pass, the
//     supervisor's periodic sweep (deferredScratchSweep) under the scratch root's own
//     maintenance lease and stamp, so it never sweeps twice per CELL_SCRATCH_SWEEP_INTERVAL;
//   - go-cache:  the cadence supervisor's between-pass maintenance (cadenceCacheMaintenance),
//     or, in a dry run, its recorded measurement;
//   - worktrees: the installed `deskwt prune --no-cache` for each CELL_ROOTS checkout, as the
//     role's loop when --role is given.
//
// Each target covers what this desk's filesystem holds, and takes the existing lock of the
// store it touches; gc adds no cell-wide lock (gc_schedule.go says why). One JSON line per
// target goes to stdout. A target that fails or could not check is reported and the next
// one still runs; the exit code is 1 when any did.
//
// Plain gc is a dry run. --apply acts, under the scope's own lock (run/gc/<role>.stamp).
// --scheduled is the supervisor's form (gc_schedule.go): it also skips, exit 0, when that
// stamp records a pass finished less than CELL_GC_INTERVAL ago.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/cellrunner"
	"github.com/medici-finance/assay/tools/desk/internal/cellscratch"
)

// gcLine is one target's report line. Bytes is what the target measured in its scope and
// Reclaimed what it removed; nil (JSON null) means the target does not report that figure.
type gcLine struct {
	Target    string `json:"target"`
	Outcome   string `json:"outcome"` // planned | applied | skipped | failed | could-not-check
	Bytes     *int64 `json:"bytes"`
	Reclaimed *int64 `json:"reclaimed"`
	Items     *int   `json:"items,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

func (l gcLine) failed() bool { return l.Outcome == "failed" || l.Outcome == "could-not-check" }

// gcTarget is one reclaim target; run never panics past runGC (a panic is that target's failure).
type gcTarget struct {
	name string
	run  func(ctx context.Context) []gcLine
}

// runGC runs every target in order and returns their lines. One target's failure, error or
// panic never stops the next.
func runGC(ctx context.Context, targets []gcTarget) []gcLine {
	var lines []gcLine
	for _, t := range targets {
		lines = append(lines, runGCTarget(ctx, t)...)
	}
	return lines
}

func runGCTarget(ctx context.Context, t gcTarget) (lines []gcLine) {
	defer func() {
		if v := recover(); v != nil {
			if code, ok := v.(exitCode); ok {
				v = fmt.Sprintf("exit %d", code.code)
			}
			lines = []gcLine{{Target: t.name, Outcome: "failed", Detail: fmt.Sprintf("target stopped: %v", v)}}
		}
	}()
	if err := ctx.Err(); err != nil {
		return []gcLine{{Target: t.name, Outcome: "skipped", Detail: "gc stopped: " + err.Error()}}
	}
	return t.run(ctx)
}

func gcOutcome(apply bool) string {
	if apply {
		return "applied"
	}
	return "planned"
}

func i64(v int64) *int64 { return &v }

// gcScratchTarget is the scratch sweep. A scheduled pass uses the periodic form, which
// skips while the scratch root's own maintenance stamp is younger than
// CELL_SCRATCH_SWEEP_INTERVAL (0 turns it off), exactly as the in-process timer did.
func (c *Cell) gcScratchTarget(scope string, apply, scheduled bool) gcTarget {
	return gcTarget{name: "scratch", run: func(ctx context.Context) []gcLine {
		line := gcLine{Target: "scratch"}
		var rep cellscratch.Report
		var err error
		if scheduled {
			interval, ierr := c.scratchSweepInterval()
			if ierr != nil {
				return []gcLine{{Target: "scratch", Outcome: "could-not-check", Detail: ierr.Error()}}
			}
			if interval == 0 {
				return []gcLine{{Target: "scratch", Outcome: "skipped", Detail: "CELL_SCRATCH_SWEEP_INTERVAL=0"}}
			}
			var ran bool
			rep, ran, err = c.deferredScratchSweep(ctx, scope, interval, nil)
			if err == nil && !ran {
				return []gcLine{{Target: "scratch", Outcome: "skipped", Detail: "swept less than CELL_SCRATCH_SWEEP_INTERVAL ago, or a sibling holds the maintenance lease"}}
			}
		} else {
			p, perr := c.scratchPolicy()
			if perr != nil {
				return []gcLine{{Target: "scratch", Outcome: "could-not-check", Detail: perr.Error()}}
			}
			s, oerr := cellscratch.Open(filepath.Join(c.Dir, "run", "scratch"))
			if oerr != nil {
				return []gcLine{{Target: "scratch", Outcome: "could-not-check", Detail: oerr.Error()}}
			}
			s.Label(c.Name, scope)
			s.Bind(ctx)
			rep, err = sweepScratch(s, p, apply)
			err = errors.Join(err, s.Close())
		}
		items := 0
		for _, e := range rep.Entries {
			if e.Action != "keep" {
				items++
			}
		}
		line.Bytes, line.Reclaimed, line.Items = i64(rep.Bytes), i64(rep.Reclaimed), &items
		line.Outcome = gcOutcome(apply)
		switch {
		case err != nil:
			line.Outcome, line.Detail = "failed", err.Error()
		case rep.Partial:
			line.Outcome, line.Detail = "could-not-check", "partial accounting"
		case rep.OverBudget:
			line.Detail = "over budget after the sweep"
		}
		return []gcLine{line}
	}}
}

// gcCacheTarget is Go cache maintenance: in a dry run, the recorded measurement; else the
// between-pass maintenance a cadence supervisor runs. That maintenance clears cheaply,
// without a walk, while the recorded measurement is fresh and under budget, so a pass
// right after a cadence supervisor's own does not walk the cache again.
func (c *Cell) gcCacheTarget(apply bool) gcTarget {
	return gcTarget{name: "go-cache", run: func(ctx context.Context) []gcLine {
		p, err := c.cachePolicy()
		if err != nil {
			return []gcLine{{Target: "go-cache", Outcome: "could-not-check", Detail: err.Error()}}
		}
		if p == nil {
			return []gcLine{{Target: "go-cache", Outcome: "skipped", Detail: "CELL_GO_CACHE is off"}}
		}
		var r cellcache.Report
		if apply {
			var rp *cellcache.Report
			rp, err = c.cadenceCacheMaintenance(ctx)
			if rp != nil {
				r = *rp
			}
		} else {
			// The dry run reads the cache's recorded measurement (report.json) and walks
			// nothing; launches and dispatches keep the cache lock to themselves.
			r, err = recordedCacheReport(*p)
		}
		line := gcLine{Target: "go-cache", Outcome: gcOutcome(apply), Bytes: r.Before.LogicalBytes, Reclaimed: i64(r.Reclaimed), Detail: r.Outcome}
		if r.After.LogicalBytes != nil && apply {
			line.Bytes = r.After.LogicalBytes
		}
		if err != nil {
			line.Outcome, line.Detail = "failed", err.Error()
		}
		return []gcLine{line}
	}}
}

// recordedCacheReport reads the last measurement through cellcache's bounded reader. A
// missing report means the cache has not been measured yet.
func recordedCacheReport(p cellcache.Policy) (cellcache.Report, error) {
	r, err := cellcache.RecordedReport(p)
	if errors.Is(err, fs.ErrNotExist) {
		return cellcache.Report{Outcome: "not measured yet"}, nil
	}
	return r, err
}

// gcToolPath resolves an installed desk tool: the cell's DESK_TOOLS_BIN directory first
// (the pinned install path), then PATH. Only an absolute, executable path is returned.
func (c *Cell) gcToolPath(name string) (string, error) {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(deskToolsBin(c.Env), name)
	if filepath.IsAbs(p) && isExecFile(p) {
		return p, nil
	}
	var found string
	var err error
	switch name {
	case "deskwt", "deskwt.exe":
		found, err = exec.LookPath("deskwt")
	case "cellctl", "cellctl.exe":
		found, err = exec.LookPath("cellctl")
	default:
		err = fmt.Errorf("not a gc tool")
	}
	if err != nil {
		return "", fmt.Errorf("no installed %s in %s or on PATH", name, deskToolsBin(c.Env))
	}
	if !filepath.IsAbs(found) {
		return "", fmt.Errorf("%s resolves to a relative path on PATH", name)
	}
	return found, nil
}

// gcRunTool runs one installed tool to completion and returns its combined output. A test
// replaces it to observe the argv without a real deskwt.
var gcRunTool = func(ctx context.Context, argv, env []string, dir string) (string, int, error) {
	w := &tailWriter{limit: 64 * 1024}
	code, uncertain, err := cellrunner.Run(ctx, argv, env, dir, w, w)
	if err == nil && uncertain {
		err = errors.New("child cleanup uncertain")
	}
	return w.String(), code, err
}

// gcWorktreeTargets runs `deskwt prune` once per stream root. --no-cache leaves Go cache
// pressure to the go-cache target; every other prune gate, its singleton included, is
// deskwt's own and unchanged.
func (c *Cell) gcWorktreeTargets(role string, apply bool) []gcTarget {
	var roots [][2]string
	seen := map[string]bool{}
	for _, e := range rootEntries(c.Env.Get("CELL_ROOTS")) {
		if e[1] == "" || seen[e[1]] {
			continue
		}
		seen[e[1]] = true
		roots = append(roots, e)
	}
	if len(roots) == 0 {
		return []gcTarget{{name: "worktrees", run: func(context.Context) []gcLine {
			return []gcLine{{Target: "worktrees", Outcome: "skipped", Detail: "CELL_ROOTS names no checkout"}}
		}}}
	}
	var targets []gcTarget
	for _, root := range roots {
		name, path := "worktrees:"+root[0], root[1]
		targets = append(targets, gcTarget{name: name, run: func(ctx context.Context) []gcLine {
			bin, err := c.gcToolPath("deskwt")
			if err != nil {
				return []gcLine{{Target: name, Outcome: "could-not-check", Detail: err.Error()}}
			}
			argv := []string{bin, "prune", "--repo", path, "--no-cache"}
			if !apply {
				argv = append(argv, "--dry-run")
			}
			// The same helper identity every other deskwt call of cellctl gets.
			env := c.deskHelperEnv(envUnset(os.Environ(), cellcache.EnvPolicy), role, "")
			out, code, err := gcRunTool(ctx, argv, env, path)
			line := gcLine{Target: name, Outcome: gcOutcome(apply), Detail: lastLine([]byte(out))}
			if err != nil || code != 0 {
				line.Outcome = "failed"
				line.Detail = strings.TrimSpace(fmt.Sprintf("exit %d %v %s", code, errOrEmpty(err), lastLine([]byte(out))))
			}
			return []gcLine{line}
		}})
	}
	return targets
}

func errOrEmpty(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// gcTargets is every target, in report order. role is "" for a pass with no role.
func (c *Cell) gcTargets(role string, apply, scheduled bool) []gcTarget {
	return append([]gcTarget{c.gcScratchTarget(gcScope(role), apply, scheduled), c.gcCacheTarget(apply)}, c.gcWorktreeTargets(role, apply)...)
}

func gcScope(role string) string {
	if role == "" {
		return "cell"
	}
	return role
}

// cmdGC is the verb. It returns the exit code: 0 when every target was planned, applied or
// skipped, 1 when any failed or could not check, or when the scope's lock was not available.
func cmdGC(c *Cell, role string, apply, scheduled bool) int {
	if scheduled && !apply {
		die("gc: --scheduled requires --apply")
	}
	if role != "" && !valueIn(role, knownRoles) {
		die("gc: unknown role %q", role)
	}
	emit := func(l gcLine) {
		b, _ := json.Marshal(l)
		fmt.Println(string(b))
	}
	ctx, stop := cellrunner.NotifyContext(context.Background())
	defer stop()
	var lock *gcLock
	if apply {
		interval := time.Duration(0)
		if scheduled {
			var err error
			if interval, err = c.gcInterval(); err != nil {
				die("gc: %v", err)
			}
			if interval == 0 {
				emit(gcLine{Target: "gc", Outcome: "skipped", Detail: "CELL_GC_INTERVAL=0"})
				return 0
			}
		}
		var outcome gcLockOutcome
		var detail string
		lock, outcome, detail = acquireGCLock(c.gcStampPath(gcScope(role)), interval, time.Now())
		switch outcome {
		case gcLockHeld, gcLockRecent:
			emit(gcLine{Target: "gc", Outcome: "skipped", Detail: detail})
			if scheduled {
				return 0
			}
			return 1
		case gcLockUnavailable:
			emit(gcLine{Target: "gc", Outcome: "could-not-check", Detail: detail})
			return 1
		}
	}
	lines := runGC(ctx, c.gcTargets(role, apply, scheduled))
	code := 0
	for _, l := range lines {
		emit(l)
		if l.failed() {
			code = 1
		}
	}
	if lock != nil {
		lock.release(code == 0, time.Now())
	}
	return code
}
