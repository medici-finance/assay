package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitmaintenance"
)

// maintenance is an inspection unless --apply is explicit. The periodic prune
// owner calls the same engine with its independent common-directory cadence.
func runObjectMaintenance(o objectsOpts) (err error) {
	ac := &auditCtx{verb: "maintain"}
	defer func() { ac.finalize(err) }()
	// The one Cobra tree (cli.go) parsed the flags (durations included) and refused any
	// positional before this handler ran; the contradiction and bounds stay domain refusals.
	repo, apply, dry, asJSON := &o.repo, &o.apply, &o.dryRun, &o.json
	interval, timeout := &o.interval, &o.timeout
	if *apply && *dry {
		return deskkit.Refused("refused: maintenance takes no positional arguments; --apply and --dry-run are contradictory")
	}
	if *interval < time.Hour || *timeout < time.Second || *timeout > 5*time.Minute {
		return deskkit.Refused("refused: --min-interval must be at least 1h; --timeout must be between 1s and 5m")
	}
	dir := *repo
	if dir == "" {
		dir, err = getwd()
		if err != nil {
			return deskkit.Unverifiable("cannot resolve working directory", err)
		}
	}
	ac.repo = repoOrEmpty(dir)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	report, runErr := runMaintenance(ctx, dir, gitmaintenance.Options{
		DryRun: !*apply, MinInterval: *interval, Timeout: *timeout,
	})
	if *asJSON {
		if encodeErr := json.NewEncoder(os.Stdout).Encode(report); encodeErr != nil {
			return deskkit.Unverifiable("cannot write maintenance report", encodeErr)
		}
	} else {
		renderMaintenance(os.Stdout, report)
	}
	if runErr != nil {
		return deskkit.Unverifiable("offline repository maintenance could not complete", runErr)
	}
	ac.detail = maintenanceSummary(report)
	if report.DryRun || report.Skipped != "" {
		ac.successResult = deskkit.ResultNoop
	}
	return nil
}

var runMaintenance = gitmaintenance.Run

func maintenanceSummary(report gitmaintenance.Report) string {
	if report.Skipped != "" {
		return "maintenance held: " + report.Skipped
	}
	b := report.Before.Objects
	if report.After == nil && !report.DryRun {
		return "maintenance incomplete: no completed after-inventory"
	}
	if report.After == nil {
		return fmt.Sprintf("maintenance plan: refs=%d worktrees=%d packs=%d pack_bytes=%d loose=%d", report.Before.Refs, len(report.Before.Worktrees), b.Packs, b.PackBytes, b.Loose)
	}
	a := report.After.Objects
	return fmt.Sprintf("maintenance completed: packs=%d->%d pack_bytes=%d->%d loose=%d->%d", b.Packs, a.Packs, b.PackBytes, a.PackBytes, b.Loose, a.Loose)
}

func renderMaintenance(w io.Writer, report gitmaintenance.Report) {
	fmt.Fprintln(w, "deskwt "+maintenanceSummary(report))
	if report.Before.CommonDir == "" {
		return
	}
	fmt.Fprintf(w, "  common Git directory: %s\n  recovery: %s\n", report.Before.CommonDir, report.Before.Recovery)
	for _, task := range report.Tasks {
		fmt.Fprintf(w, "  %s: %s (%s)\n", task.Name, task.Status, task.Duration.Round(time.Millisecond))
	}
	for _, ref := range report.TemporaryRefs {
		fmt.Fprintf(w, "  temporary ref: %s %s — %s\n", ref.Ref, ref.State, ref.Reason)
	}
	if report.DryRun {
		for _, key := range sortedMaintenanceKeys(report.Before.RefNamespaces) {
			fmt.Fprintf(w, "  namespace: %s refs=%d\n", key, report.Before.RefNamespaces[key])
		}
		keys := make([]string, 0, len(report.Before.ReflogRecovery))
		for key := range report.Before.ReflogRecovery {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(w, "  recovery setting: %s=%s (unchanged)\n", key, report.Before.ReflogRecovery[key])
		}
		for _, holder := range report.Before.Worktrees {
			fmt.Fprintf(w, "  holder: %s head=%s branch=%s locked=%t prunable=%t\n", holder.Path, holder.HEAD, holder.Branch, holder.Locked, holder.Prunable)
		}
		for _, ref := range report.Before.DispatchRefs {
			fmt.Fprintf(w, "  claim held: %s (claim protocol owns release)\n", ref)
		}
		for _, alternate := range report.Before.Alternates {
			fmt.Fprintf(w, "  alternate: %s (all object IDs retained)\n", alternate)
		}
		for _, ref := range report.Before.TemporaryRefs {
			fmt.Fprintf(w, "  temporary ref: %s %s — %s\n", ref.Ref, ref.State, ref.Reason)
		}
	}
}

func sortedMaintenanceKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// Failure here is visible but does not turn a worktree cleanup or desk boot into
// an outage. Explicit maintenance --apply returns unverifiable on the same error.
func pruneMaintenance(ctx context.Context, dir string, opts pruneOpts, w io.Writer) {
	if opts.noMaintenance || opts.branchesOnly {
		return
	}
	report, err := runMaintenance(ctx, dir, gitmaintenance.Options{DryRun: opts.dryRun})
	if err != nil {
		fmt.Fprintf(w, "  maintenance could-not-check: %s\n", err)
		return
	}
	renderMaintenance(w, report)
}
