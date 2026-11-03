package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitmaintenance"
)

func maintenanceRecorder(t *testing.T, failure error) *[]gitmaintenance.Options {
	t.Helper()
	calls := &[]gitmaintenance.Options{}
	old := runMaintenance
	runMaintenance = func(_ context.Context, _ string, opts gitmaintenance.Options) (gitmaintenance.Report, error) {
		*calls = append(*calls, opts)
		return gitmaintenance.Report{DryRun: opts.DryRun, Skipped: "fixture coordinator already completed"}, failure
	}
	t.Cleanup(func() { runMaintenance = old })
	return calls
}

// regression: #2513
func TestMaintenanceRequiresExplicitApply(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	calls := maintenanceRecorder(t, nil)
	for _, args := range [][]string{{"maintain", "objects"}, {"maintain", "objects", "--dry-run"}} {
		if code := run(args); code != deskkit.ExitOK {
			t.Fatalf("%v: exit %d", args, code)
		}
	}
	if len(*calls) != 2 || !(*calls)[0].DryRun || !(*calls)[1].DryRun {
		t.Fatalf("inspection unexpectedly requested mutation: %+v", *calls)
	}
	if code := run([]string{"maintain", "objects", "--apply", "--min-interval", "48h", "--timeout", "1m"}); code != deskkit.ExitOK {
		t.Fatalf("apply: exit %d", code)
	}
	got := (*calls)[2]
	if got.DryRun || got.MinInterval != 48*time.Hour || got.Timeout != time.Minute {
		t.Fatalf("apply did not forward bounded options: %+v", got)
	}
}

// regression: #2513
func TestMaintenanceInvalidOptionsNeverReachEngine(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	calls := maintenanceRecorder(t, nil)
	for _, tail := range [][]string{
		{"--apply", "--dry-run"}, {"--min-interval", "0"}, {"--min-interval", "-1h"},
		{"--min-interval", "1ns"}, {"--timeout", "0"}, {"--timeout", "500ms"}, {"--timeout", "6m"},
	} {
		if code := run(append([]string{"maintain", "objects"}, tail...)); code != deskkit.ExitRefused {
			t.Fatalf("%v: exit %d, want refused", tail, code)
		}
	}
	// An undeclared flag or a stray word is a malformed command line: the usage code.
	for _, tail := range [][]string{{"--prefetch"}, {"--force"}, {"gc"}, {"--timeout", "soon"}} {
		if code := run(append([]string{"maintain", "objects"}, tail...)); code != 2 {
			t.Fatalf("%v: exit %d, want 2 (usage)", tail, code)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("invalid options reached maintenance: %+v", *calls)
	}
}

// regression: #2513
func TestMaintenanceFailureIsUnverifiableAndPruneReportsIt(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	maintenanceRecorder(t, errors.New("unreadable common-directory state"))
	code, output := runCapErr(t, []string{"maintain", "objects", "--apply"})
	if code != deskkit.ExitUnverifiable || !strings.Contains(output, "unreadable common-directory state") {
		t.Fatalf("explicit failure was not reported: code=%d %s", code, output)
	}
	var report bytes.Buffer
	pruneMaintenance(context.Background(), work, pruneOpts{}, &report)
	if !strings.Contains(report.String(), "maintenance could-not-check: unreadable common-directory state") {
		t.Fatalf("boot would conceal maintenance failure: %s", report.String())
	}
}

// regression: #2513
func TestPruneMaintenanceCadenceIsIndependentAndBranchesOnlyStaysNarrow(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)
	calls := maintenanceRecorder(t, nil)
	// A lifecycle sweep that completed recently still gives the independent
	// common-directory coordinator a chance to inspect its own daily cadence.
	writeStamp(t, stampPathFor(t, work), pruneStamp{Schema: pruneStampSchema, PID: 4242,
		StartedAt:  time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339),
		FinishedAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)})
	if code, _ := runCapErr(t, []string{"prune"}); code != deskkit.ExitOK {
		t.Fatalf("prune: %d", code)
	}
	if len(*calls) != 1 || (*calls)[0].DryRun {
		t.Fatalf("daily coordinator was skipped with lifecycle cadence: %+v", *calls)
	}
	for _, flag := range []string{"--no-maintenance", "--branches-only"} {
		if code, _ := runCapErr(t, []string{"prune", flag}); code != deskkit.ExitOK {
			t.Fatalf("prune %s: %d", flag, code)
		}
	}
	if len(*calls) != 1 {
		t.Fatalf("explicit narrow sweeps launched maintenance: %+v", *calls)
	}
	var plan bytes.Buffer
	pruneMaintenance(context.Background(), work, pruneOpts{dryRun: true}, &plan)
	if len(*calls) != 2 || !(*calls)[1].DryRun {
		t.Fatalf("prune dry-run requested mutation: %+v", *calls)
	}
}

// regression: #2513
func TestMaintenanceIncompleteReport(t *testing.T) {
	r := gitmaintenance.Report{Tasks: []gitmaintenance.Task{{Name: "pack-refs", Status: "failed"}}}
	if got := maintenanceSummary(r); !strings.Contains(got, "maintenance incomplete") {
		t.Fatalf("failed apply described as a plan: %s", got)
	}
}
