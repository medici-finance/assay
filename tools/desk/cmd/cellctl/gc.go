package main

// `cellctl gc <cell> [--role <role>]` — a read-only report over a desk's reclaim targets.
//
// Bare gc is the dry run spec/cell-op-v1.md §5 calls read-only. It runs each existing,
// already-guarded entry point in its dry form and prints one JSON line per target:
//
//   - scratch:   the explicit sweep (sweepScratch) without --apply;
//   - go-cache:  the cache's recorded measurement, read through cellcache.RecordedReport
//     (the package's bounded metadata reader); nothing is walked and no lock is taken;
//   - worktrees: the installed `deskwt prune --dry-run --no-cache` for each CELL_ROOTS
//     checkout, run as the role's loop when --role is given.
//
// A target that fails or could not check is reported and the next one still runs; the exit
// code is 1 when any did. gc adds no delete logic of its own.
//
// The state-changing forms §5 names, `gc --run` and `gc --apply <plan-id>`, are registered
// and annotated cellop:state-changing, so they are refused off a terminal and in a desk
// session, and on a terminal they go through the one apply path (cellopApply) and its typed
// confirm. Apply itself is HELD: the driver's ruling on #7280 holds gc's apply until
// cell-op-v1 specifies a non-interactive maintenance class, so every action of a gc plan
// refuses at run time and nothing is reclaimed through gc.

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

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
	"github.com/medici-finance/assay/tools/desk/internal/cellop"
	"github.com/medici-finance/assay/tools/desk/internal/cellrunner"
	"github.com/medici-finance/assay/tools/desk/internal/cellscratch"
)

// gcLine is one target's report line. Bytes is what the target measured in its scope and
// Reclaimed what it would reclaim or did; nil (JSON null) means the target reports no figure.
type gcLine struct {
	Target    string `json:"target"`
	Outcome   string `json:"outcome"` // planned | skipped | failed | could-not-check
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

func i64(v int64) *int64 { return &v }

// gcScratchTarget is the scratch sweep's dry run.
func (c *Cell) gcScratchTarget(scope string) gcTarget {
	return gcTarget{name: "scratch", run: func(ctx context.Context) []gcLine {
		p, err := c.scratchPolicy()
		if err != nil {
			return []gcLine{{Target: "scratch", Outcome: "could-not-check", Detail: err.Error()}}
		}
		s, err := cellscratch.Open(filepath.Join(c.Dir, "run", "scratch"))
		if err != nil {
			return []gcLine{{Target: "scratch", Outcome: "could-not-check", Detail: err.Error()}}
		}
		s.Label(c.Name, scope)
		s.Bind(ctx)
		rep, err := sweepScratch(s, p, false)
		err = errors.Join(err, s.Close())
		items := 0
		for _, e := range rep.Entries {
			if e.Action != "keep" {
				items++
			}
		}
		line := gcLine{Target: "scratch", Outcome: "planned", Bytes: i64(rep.Bytes), Reclaimed: i64(rep.Reclaimed), Items: &items}
		switch {
		case err != nil:
			line.Outcome, line.Detail = "failed", err.Error()
		case rep.Partial:
			line.Outcome, line.Detail = "could-not-check", "partial accounting"
		}
		return []gcLine{line}
	}}
}

// gcCacheTarget reports the Go cache's recorded measurement.
func (c *Cell) gcCacheTarget() gcTarget {
	return gcTarget{name: "go-cache", run: func(context.Context) []gcLine {
		p, err := c.cachePolicy()
		if err != nil {
			return []gcLine{{Target: "go-cache", Outcome: "could-not-check", Detail: err.Error()}}
		}
		if p == nil {
			return []gcLine{{Target: "go-cache", Outcome: "skipped", Detail: "CELL_GO_CACHE is off"}}
		}
		r, err := recordedCacheReport(*p)
		if err != nil {
			return []gcLine{{Target: "go-cache", Outcome: "could-not-check", Detail: err.Error()}}
		}
		bytes := r.After.LogicalBytes
		if bytes == nil {
			bytes = r.Before.LogicalBytes
		}
		return []gcLine{{Target: "go-cache", Outcome: "planned", Bytes: bytes, Reclaimed: i64(r.Reclaimed), Detail: r.Outcome}}
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

// gcDeskwtPath resolves the installed deskwt: the cell's DESK_TOOLS_BIN directory first
// (the pinned install path), then PATH. Only an absolute, executable path is returned.
func (c *Cell) gcDeskwtPath() (string, error) {
	name := "deskwt"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if p := filepath.Join(deskToolsBin(c.Env), name); filepath.IsAbs(p) && isExecFile(p) {
		return p, nil
	}
	found, err := exec.LookPath("deskwt")
	if err != nil {
		return "", fmt.Errorf("no installed deskwt in %s or on PATH", deskToolsBin(c.Env))
	}
	if !filepath.IsAbs(found) {
		return "", errors.New("deskwt resolves to a relative path on PATH")
	}
	return found, nil
}

// gcRunTool runs one installed tool to completion and returns the tail of its output. A
// test replaces it to observe the argv without a real deskwt.
var gcRunTool = func(ctx context.Context, argv, env []string, dir string) (string, int, error) {
	w := &tailWriter{limit: 64 * 1024}
	code, uncertain, err := cellrunner.Run(ctx, argv, env, dir, w, w)
	if err == nil && uncertain {
		err = errors.New("child cleanup uncertain")
	}
	return w.String(), code, err
}

// gcRoots is each distinct CELL_ROOTS checkout.
func (c *Cell) gcRoots() [][2]string {
	var roots [][2]string
	seen := map[string]bool{}
	for _, e := range rootEntries(c.Env.Get("CELL_ROOTS")) {
		if e[1] == "" || seen[e[1]] {
			continue
		}
		seen[e[1]] = true
		roots = append(roots, e)
	}
	return roots
}

// gcWorktreeTargets runs `deskwt prune --dry-run` once per stream root. --no-cache leaves Go
// cache pressure to the go-cache target; every other prune gate is deskwt's own.
func (c *Cell) gcWorktreeTargets(role string) []gcTarget {
	roots := c.gcRoots()
	if len(roots) == 0 {
		return []gcTarget{{name: "worktrees", run: func(context.Context) []gcLine {
			return []gcLine{{Target: "worktrees", Outcome: "skipped", Detail: "CELL_ROOTS names no checkout"}}
		}}}
	}
	var targets []gcTarget
	for _, root := range roots {
		name, path := "worktrees:"+root[0], root[1]
		targets = append(targets, gcTarget{name: name, run: func(ctx context.Context) []gcLine {
			bin, err := c.gcDeskwtPath()
			if err != nil {
				return []gcLine{{Target: name, Outcome: "could-not-check", Detail: err.Error()}}
			}
			argv := []string{bin, "prune", "--repo", path, "--no-cache", "--dry-run"}
			// The same helper identity every other deskwt call of cellctl gets.
			env := c.deskHelperEnv(envUnset(os.Environ(), cellcache.EnvPolicy), role, "")
			out, code, err := gcRunTool(ctx, argv, env, path)
			line := gcLine{Target: name, Outcome: "planned", Detail: lastLine([]byte(out))}
			if err != nil || code != 0 {
				line.Outcome = "failed"
				line.Detail = strings.TrimSpace(fmt.Sprintf("exit %d %s %s", code, errOrEmpty(err), lastLine([]byte(out))))
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
func (c *Cell) gcTargets(role string) []gcTarget {
	scope := role
	if scope == "" {
		scope = "cell"
	}
	return append([]gcTarget{c.gcScratchTarget(scope), c.gcCacheTarget()}, c.gcWorktreeTargets(role)...)
}

// cmdGC is the dry run. It returns the exit code: 0 when every target was planned or
// skipped, 1 when any failed or could not check.
func cmdGC(c *Cell, role string) int {
	if role != "" && !valueIn(role, knownRoles) {
		die("gc: unknown role %q", role)
	}
	ctx, stop := cellrunner.NotifyContext(context.Background())
	defer stop()
	code := 0
	for _, l := range runGC(ctx, c.gcTargets(role)) {
		b, _ := json.Marshal(l)
		fmt.Println(string(b))
		if l.failed() {
			code = 1
		}
	}
	return code
}

// gcApplyHeld is the run-time answer of every gc plan action: apply is held by the ruling
// on #7280. It is the only Run gc gives the apply path, so a confirmed apply reclaims nothing.
const gcApplyHeld = "gc apply is held until cell-op-v1 specifies a non-interactive maintenance class (ruling on #7280); nothing was reclaimed"

// gcHeldPrecondition is what every gc action records and re-measures: nothing gc applies can
// change between plan and apply, because nothing is applied.
var gcHeldPrecondition = cellop.Precondition{Key: "gc/apply", Value: "held"}

// gcVerb is gc's cell operation: its actions name the targets, and running one refuses.
var gcVerb = cellopVerb{
	Verb: "gc",
	Measure: func(*Cell, cellop.Action) ([]cellop.Precondition, error) {
		return []cellop.Precondition{gcHeldPrecondition}, nil
	},
	Run: func(*Cell, cellop.Action) (string, error) { return "", errors.New(gcApplyHeld) },
}

// gcActions is the plan of a gc: one held action per target, built without running any.
func (c *Cell) gcActions() []cellop.Action {
	ids := []string{"scratch", "go-cache"}
	for _, r := range c.gcRoots() {
		ids = append(ids, "worktrees:"+r[0])
	}
	var actions []cellop.Action
	for _, id := range ids {
		actions = append(actions, cellop.Action{ID: id, Cell: c.Name,
			Summary:       "gc " + id + " (apply held: ruling on #7280)",
			Preconditions: []cellop.Precondition{gcHeldPrecondition}})
	}
	return actions
}

// gcRun is the verb's handler: --plan writes a plan, --apply <id> and --run go through the
// one apply path (whose actions refuse, held), and the bare form is the dry-run report.
func gcRun(cmd *cobra.Command, c *Cell, role string) {
	if role != "" && !valueIn(role, knownRoles) {
		die("gc: unknown role %q", role)
	}
	plan, apply, run := cmd.Flags().Changed("plan"), cmd.Flags().Changed("apply"), cmd.Flags().Changed("run")
	switch {
	case plan && (apply || run):
		die("gc: --plan is exclusive with --apply and --run")
	case apply && run:
		die("gc: --apply and --run are exclusive")
	case plan:
		cellopPlan(cmd, c, "gc", c.gcActions())
	case apply:
		cellopApply(cmd, c, gcVerb, cmd.Flags().Lookup("apply").Value.String())
	case run:
		p := cellopNewPlan(c, "gc", c.gcActions())
		cellopApply(cmd, c, gcVerb, p.ID)
	default:
		if code := cmdGC(c, role); code != 0 {
			exitWith(code)
		}
	}
}
