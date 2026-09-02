package main

// runnertable_wiring_test.go exercises the cmd/verifyloop consumption of the extracted
// internal/runnertable package: VerifyLoop.Dispatch resolving a native runner through a
// *runnertable.RunnerTable, and the preflightRunnerTable boot hook. The generic table
// contract (load/resolve/refusal rules) is covered by internal/runnertable's own tests;
// this file covers only the verifyloop-specific wiring (Dispatch, reachableTiers, boot).

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
	"github.com/medici-finance/assay/tools/desk/internal/runnertable"
)

// wiringEnvGetter builds a getenv closure over a fixed map (no process env mutation) — the
// same pattern internal/runnertable's own tests use, kept local since env-var-backed test
// tables are the package's public construction path (no exported struct-literal surface).
func wiringEnvGetter(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestNativeDispatchRoundTripViaTable drives the native ACP path with the runner selected by the
// tier→runner table (not the LE/15 legacy RunnerCmd) and asserts Result.RunnerID is derived from
// the resolved entry — attribution the engine knows at spawn, carrying the pin + model.
func TestNativeDispatchRoundTripViaTable(t *testing.T) {
	nativeTestHome(t)
	wt := t.TempDir()
	tbl, err := runnertable.LoadRunnerTable(wiringEnvGetter(map[string]string{
		"ASSAY_RUNNER_LOCAL": `{"cmd":["` + os.Args[0] + `"],"model":"local-model","pin":"v0"}`,
	}), nil)
	if err != nil {
		t.Fatalf("build runner table: %v", err)
	}
	v := &VerifyLoop{
		Root:          t.TempDir(),
		TargetSHA:     "deadbeef",
		Native:        true,
		NativeEnv:     []string{"VERIFYLOOP_FAKE_ACP=roundtrip"},
		NativeTimeout: 20 * time.Second,
		RunnerTable:   tbl,
		MakeWorktree:  func(loopengine.Item) (string, func(), error) { return wt, func() {}, nil },
	}
	h, err := v.Dispatch(fixtureItem(), loopengine.TierLocal)
	if err != nil {
		t.Fatalf("native Dispatch via table: %v", err)
	}
	r := awaitResult(t, h)
	if r.Verdict != loopengine.VerdictPass {
		t.Fatalf("verdict = %q, want PASS", r.Verdict)
	}
	if !strings.HasPrefix(r.RunnerID, "acp:local:") || !strings.Contains(r.RunnerID, "@v0") || !strings.Contains(r.RunnerID, "#local-model") {
		t.Fatalf("Result.RunnerID not derived from the table entry (want tier+cmd+pin+model): %q", r.RunnerID)
	}
}

// TestNativeDispatchRefusesUnconfiguredTierViaTable: a table that does not configure the
// dispatched tier refuses synchronously — never a silent no-runner dispatch.
func TestNativeDispatchRefusesUnconfiguredTierViaTable(t *testing.T) {
	nativeTestHome(t)
	tbl, err := runnertable.LoadRunnerTable(wiringEnvGetter(map[string]string{
		"ASSAY_RUNNER_LOCAL": `{"cmd":["x"],"pin":"v0"}`,
	}), nil)
	if err != nil {
		t.Fatalf("build runner table: %v", err)
	}
	v := &VerifyLoop{
		Root:        t.TempDir(),
		TargetSHA:   "deadbeef",
		Native:      true,
		RunnerTable: tbl,
	}
	// TierSession is not in the table.
	if _, err := v.Dispatch(fixtureItem(), loopengine.TierSession); err == nil || !strings.Contains(err.Error(), "no runner configured for tier") {
		t.Fatalf("dispatch on an unconfigured tier must refuse, got: %v", err)
	}
}

// TestPreflightRunnerTable proves the boot hook is inert with no table and refuses on a bad one.
func TestPreflightRunnerTable(t *testing.T) {
	if err := preflightRunnerTable(wiringEnvGetter(map[string]string{})); err != nil {
		t.Fatalf("no table configured must be a no-op boot, got: %v", err)
	}
	// Configured but invalid (missing pin) => boot refuses.
	if err := preflightRunnerTable(wiringEnvGetter(map[string]string{
		"ASSAY_RUNNER_LOCAL": `{"cmd":["x"]}`,
	})); err == nil {
		t.Fatal("a configured-but-invalid table must refuse at boot")
	}
	// Configured, valid, covers the reachable set => boot clean.
	if err := preflightRunnerTable(wiringEnvGetter(map[string]string{
		"ASSAY_RUNNER_LOCAL": `{"cmd":["x"],"pin":"1"}`,
	})); err != nil {
		t.Fatalf("a valid table covering the reachable set must boot clean, got: %v", err)
	}
}
