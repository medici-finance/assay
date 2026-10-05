package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/cellprocess"
	"github.com/medici-finance/assay/tools/desk/internal/cellscratch"
)

type scratchInputs []string

func (i *scratchInputs) String() string     { return strings.Join(*i, ",") }
func (i *scratchInputs) Set(v string) error { *i = append(*i, v); return nil }

// scratch is cell-scoped but task-granular: the long-lived role worktree remains
// with deskwt, while each bounded invocation receives its own TMPDIR and lease.
func cmdScratch(name string, args []string) {
	if len(args) == 0 {
		die("scratch requires run, ack, sweep, or inventory")
	}
	c := loadCell(name)
	defaults, policyErr := c.scratchPolicy()
	if policyErr != nil {
		die("scratch: %v", policyErr)
	}
	f := flag.NewFlagSet("scratch "+args[0], flag.ContinueOnError)
	apply := f.Bool("apply", false, "apply cleanup; default is dry-run")
	maxAge := f.Duration("max-age", defaults.MaxAge, "diagnostic retention age")
	maxBytes := f.Int64("max-bytes", defaults.MaxBytes, "retained diagnostic byte budget")
	task := f.String("task", "", "task identity")
	session := f.String("session", os.Getenv("DESK_SESSION"), "session identity")
	source := f.String("source", "", "source Git checkout (required for run)")
	revision := f.String("revision", "HEAD", "source revision")
	snapshot := f.Bool("snapshot", false, "materialize all tracked files, never working-directory copies")
	snapshotBytes := f.Int64("snapshot-bytes", 256*1024*1024, "snapshot plus declared input byte limit")
	var inputs scratchInputs
	f.Var(&inputs, "input", "explicit extra source-relative file needed by this task; repeatable")
	id := f.String("id", os.Getenv("ASSAY_SCRATCH_ID"), "owned task id")
	receipt := f.String("receipt", "", "canonical evidence destination, after verified handoff")
	resumable := f.Bool("resumable", false, "retain for resumption (inactive task only)")
	legacy := f.String("path", "", "legacy root for read-only inventory")
	if err := f.Parse(args[1:]); err != nil {
		exitWith(2)
	}
	if args[0] == "inventory" {
		if *legacy == "" || len(f.Args()) > 0 {
			die("scratch inventory requires --path")
		}
		rows, err := cellscratch.Inventory(*legacy)
		scratchJSON(rows)
		if err != nil {
			die("scratch inventory: %v", err)
		}
		return
	}
	root := filepath.Join(c.Dir, "run", "scratch")
	s, err := cellscratch.Open(root)
	if err != nil {
		die("scratch: %v", err)
	}
	defer s.Close()
	policy := cellscratch.Policy{MaxAge: *maxAge, MaxBytes: *maxBytes}
	switch args[0] {
	case "sweep":
		if len(f.Args()) != 0 {
			die("scratch sweep takes no command")
		}
		report, err := s.Sweep(policy, *apply)
		scratchJSON(report)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitWith(1)
		}
		if report.OverBudget {
			exitWith(2)
		}
	case "ack":
		if len(f.Args()) != 0 {
			die("scratch ack takes no command")
		}
		if err = s.Acknowledge(*id, *receipt, *resumable); err != nil {
			die("scratch acknowledgment: %v", err)
		}
	case "run":
		if *source == "" || len(f.Args()) == 0 {
			die("scratch run requires --source and -- command [args]")
		}
		if policy.MaxAge < 0 || policy.MaxBytes < 0 || *snapshotBytes <= 0 {
			die("scratch budgets must be nonnegative; snapshot budget must be positive")
		}
		abs, err := filepath.Abs(*source)
		if err != nil {
			die("scratch source: %v", err)
		}
		// Resolve once. Snapshot reads that immutable object even if the source branch moves.
		cmd := exec.Command("git", "-C", abs, "rev-parse", "--verify", "--end-of-options", *revision+"^{commit}")
		out, err := cmd.Output()
		if err != nil {
			die("scratch source revision: %v", err)
		}
		sha := strings.TrimSpace(string(out))
		if _, err = s.Sweep(policy, true); err != nil {
			die("scratch startup recovery: %v", err)
		}
		r, err := s.Begin(*session, *task, sha)
		if err != nil {
			die("scratch begin: %v", err)
		}
		defer r.Close()
		ctx, stop := cellprocess.NotifyContext(context.Background())
		defer stop()
		code, uncertain := -1, false
		runErr := error(nil)
		if *snapshot {
			runErr = r.Snapshot(ctx, abs, *snapshotBytes)
		}
		if runErr == nil && len(inputs) > 0 {
			runErr = r.Inputs(abs, inputs, *snapshotBytes)
		}
		tail := r.Tail()
		if runErr == nil {
			env := os.Environ()
			for k, v := range map[string]string{"TMPDIR": r.Work(), "TMP": r.Work(), "TEMP": r.Work(), "ASSAY_SCRATCH_ID": r.Record.ID, "ASSAY_SCRATCH_ROOT": root, "ASSAY_SCRATCH_CELL": c.Name, "CELLS_ROOT": filepath.Dir(c.Dir), "ASSAY_SOURCE_ROOT": abs, "ASSAY_SOURCE_REVISION": sha} {
				env = envSet(env, k, v)
			}
			fmt.Fprintf(os.Stderr, "[scratch] id=%s work=%s source=%s revision=%s; evidence pending\n", r.Record.ID, r.Work(), abs, sha)
			code, uncertain, runErr = cellprocess.RunObserved(ctx, f.Args(), env, r.Work(), io.MultiWriter(os.Stdout, tail), io.MultiWriter(os.Stderr, tail), r.Started)
		}
		if runErr != nil {
			fmt.Fprintln(os.Stderr, "scratch task:", runErr)
		}
		if tail.Err() != nil {
			uncertain = true
		}
		if err = r.Finish(code, !uncertain); err != nil {
			die("scratch finish: %v", err)
		}
		// Release before sweep; all destruction still flows through the same owner.
		if err = r.Close(); err != nil {
			die("scratch release: %v", err)
		}
		report, sweepErr := s.Sweep(policy, true)
		scratchJSON(report)
		if code < 0 {
			code = 1
		}
		if code == 0 && (runErr != nil || sweepErr != nil || report.OverBudget) {
			code = 1
		}
		if sweepErr != nil {
			fmt.Fprintln(os.Stderr, "scratch cleanup:", sweepErr)
		}
		exitWith(code)
	default:
		die("unknown scratch operation %q", args[0])
	}
}
func scratchJSON(v any) {
	if err := json.NewEncoder(os.Stdout).Encode(v); err != nil {
		die("scratch output: %v", err)
	}
}
