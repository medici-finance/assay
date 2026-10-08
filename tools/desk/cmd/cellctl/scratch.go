package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/medici-finance/assay/tools/desk/internal/cellprocess"
	"github.com/medici-finance/assay/tools/desk/internal/cellscratch"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

// scratch is cell-scoped but task-granular: the long-lived role worktree remains
// with deskwt, while each bounded invocation receives its own TMPDIR and lease.
func cmdScratch(name, action string, v *cli.Values, command []string) {
	c := loadCell(name)
	defaults, policyErr := c.scratchPolicy()
	if policyErr != nil {
		die("scratch: %v", policyErr)
	}
	apply := v.Bool("apply")
	maxAge, maxBytes := defaults.MaxAge, defaults.MaxBytes
	if v.IsSet("max-age") {
		maxAge = v.Duration("max-age")
	}
	if v.IsSet("max-bytes") {
		maxBytes = int64(v.Int("max-bytes"))
	}
	task, session, source, revision := v.String("task"), v.String("session"), v.String("source"), v.String("revision")
	snapshot, snapshotBytes := v.Bool("snapshot"), int64(v.Int("snapshot-bytes"))
	inputs := v.Strings("input")
	id, receipt, resumable, legacy := v.String("id"), v.String("receipt"), v.Bool("resumable"), v.String("path")
	if action == "inventory" {
		if legacy == "" || len(command) > 0 {
			die("scratch inventory requires --path")
		}
		rows, err := cellscratch.Inventory(legacy)
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
	policy := cellscratch.Policy{MaxAge: maxAge, MaxBytes: maxBytes}
	switch action {
	case "sweep":
		if len(command) != 0 {
			die("scratch sweep takes no command")
		}
		report, err := s.Sweep(policy, apply)
		scratchJSON(report)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitWith(1)
		}
		if report.OverBudget {
			exitWith(2)
		}
	case "ack":
		if len(command) != 0 {
			die("scratch ack takes no command")
		}
		if err = s.Acknowledge(id, receipt, resumable); err != nil {
			die("scratch acknowledgment: %v", err)
		}
	case "run":
		if source == "" || len(command) == 0 {
			die("scratch run requires --source and -- command [args]")
		}
		if policy.MaxAge < 0 || policy.MaxBytes < 0 || snapshotBytes <= 0 {
			die("scratch budgets must be nonnegative; snapshot budget must be positive")
		}
		// Admit the working-tree root before resolving the immutable revision.
		abs, sha, err := cellscratch.SourceRevision(source, revision)
		if err != nil {
			die("scratch source: %v", err)
		}
		if _, err = s.Sweep(policy, true); err != nil {
			die("scratch startup recovery: %v", err)
		}
		r, err := s.Begin(session, task, sha)
		if err != nil {
			die("scratch begin: %v", err)
		}
		defer r.Close()
		ctx, stop := cellprocess.NotifyContext(context.Background())
		defer stop()
		code, uncertain := -1, false
		runErr := error(nil)
		if snapshot {
			runErr = r.Snapshot(ctx, abs, snapshotBytes)
		}
		if runErr == nil && len(inputs) > 0 {
			runErr = r.Inputs(abs, inputs, snapshotBytes)
		}
		tail := r.Tail()
		if runErr == nil {
			env := os.Environ()
			for k, v := range map[string]string{"TMPDIR": r.Work(), "TMP": r.Work(), "TEMP": r.Work(), "ASSAY_SCRATCH_ID": r.Record.ID, "ASSAY_SCRATCH_ROOT": root, "ASSAY_SCRATCH_CELL": c.Name, "CELLS_ROOT": filepath.Dir(c.Dir), "ASSAY_SOURCE_ROOT": abs, "ASSAY_SOURCE_REVISION": sha} {
				env = envSet(env, k, v)
			}
			fmt.Fprintf(os.Stderr, "[scratch] id=%s work=%s source=%s revision=%s; evidence pending\n", r.Record.ID, r.Work(), abs, sha)
			code, uncertain, runErr = cellprocess.RunObserved(ctx, command, env, r.Work(), io.MultiWriter(os.Stdout, tail), io.MultiWriter(os.Stderr, tail), r.Started)
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
		die("unknown scratch operation %q", action)
	}
}
func scratchJSON(v any) {
	if err := json.NewEncoder(os.Stdout).Encode(v); err != nil {
		die("scratch output: %v", err)
	}
}
