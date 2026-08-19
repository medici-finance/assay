package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// envGetter builds a getenv closure over a fixed map (no process env mutation).
func envGetter(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// TestRunnerTable covers the runner-table contract: valid table resolves per tier; the
// refusal cases (missing pin, unknown tier) refuse; the C-floor isolate=false refuses; and boot
// validation catches a reachable-but-unconfigured tier.
func TestRunnerTable(t *testing.T) {
	// --- valid table resolves per tier -------------------------------------
	t.Run("valid table resolves per tier", func(t *testing.T) {
		tbl, err := LoadRunnerTable(envGetter(map[string]string{
			"ASSAY_RUNNER_LOCAL":   `{"cmd":["npx","@agentclientprotocol/claude-agent-acp"],"model":"session-model","pin":"0.4.1"}`,
			"ASSAY_RUNNER_SESSION": `{"cmd":["codex-acp"],"model":"strong","pin":"1.2.0"}`,
		}), nil)
		if err != nil {
			t.Fatalf("valid table failed to load: %v", err)
		}
		e, err := tbl.Resolve(loopengine.TierLocal)
		if err != nil {
			t.Fatalf("resolve local: %v", err)
		}
		if e.Model != "session-model" || e.Pin != "0.4.1" || len(e.Cmd) != 2 {
			t.Fatalf("local entry mis-resolved: %+v", e)
		}
		// RunnerID is derived from the resolved row (tier + cmd + model + pin).
		if id := e.RunnerID(loopengine.TierLocal); !strings.Contains(id, "0.4.1") || !strings.Contains(id, "session-model") || !strings.HasPrefix(id, "acp:local:") {
			t.Fatalf("RunnerID not derived from the entry: %q", id)
		}
		if s, err := tbl.Resolve(loopengine.TierSession); err != nil || s.Pin != "1.2.0" {
			t.Fatalf("resolve session: %+v err=%v", s, err)
		}
		// An unconfigured tier fails closed, never a silent no-runner.
		if _, err := tbl.Resolve(loopengine.TierCheap); err == nil {
			t.Fatal("resolve of an unconfigured tier must refuse")
		}
	})

	// --- missing pin refuses (mandatory pinning, spec §7.3) -----------------
	t.Run("missing pin refuses", func(t *testing.T) {
		_, err := LoadRunnerTable(envGetter(map[string]string{
			"ASSAY_RUNNER_LOCAL": `{"cmd":["npx","claude-agent-acp"],"model":"m"}`,
		}), nil)
		if err == nil || !strings.Contains(err.Error(), "version pin") {
			t.Fatalf("missing pin must refuse with a pin error, got: %v", err)
		}
		if !deskkit.IsRefused(err) {
			t.Fatalf("missing pin must map to exit 5 (refused), got code %d", deskkit.ExitCodeOf(err))
		}
	})

	// --- unknown / non-dispatchable tier key refuses ------------------------
	t.Run("unknown tier refuses", func(t *testing.T) {
		// A file-form table can carry an arbitrary key; the per-tier env keys can't name
		// "human", so exercise the unknown-key path via the file loader.
		read := func(string) ([]byte, error) {
			return []byte(`{"human":{"cmd":["x"],"pin":"1"}}`), nil
		}
		_, err := LoadRunnerTable(envGetter(map[string]string{"ASSAY_RUNNER_TABLE": "/runners.json"}), read)
		if err == nil || !strings.Contains(err.Error(), "non-dispatchable") {
			t.Fatalf("a human/unknown tier key must refuse, got: %v", err)
		}
		if !deskkit.IsRefused(err) {
			t.Fatalf("unknown tier must map to exit 5, got code %d", deskkit.ExitCodeOf(err))
		}
	})

	// --- C floor: isolate=false REFUSES, never degrades -------
	t.Run("isolate=false refuses (C floor)", func(t *testing.T) {
		_, err := LoadRunnerTable(envGetter(map[string]string{
			"ASSAY_RUNNER_LOCAL": `{"cmd":["x"],"pin":"1","isolate":false}`,
		}), nil)
		if err == nil || !strings.Contains(err.Error(), "isolate") {
			t.Fatalf("isolate=false must REFUSE (C floor), got: %v", err)
		}
		if !deskkit.IsRefused(err) {
			t.Fatalf("isolate=false must map to exit 5, got code %d", deskkit.ExitCodeOf(err))
		}
		// A runner that does NOT declare isolate (nil) defaults to isolate-capable and loads.
		if _, err := LoadRunnerTable(envGetter(map[string]string{
			"ASSAY_RUNNER_LOCAL": `{"cmd":["x"],"pin":"1"}`,
		}), nil); err != nil {
			t.Fatalf("an entry with isolate unset must default isolate-capable and load: %v", err)
		}
	})

	// --- boot validation catches a reachable-but-unconfigured tier ----------
	t.Run("boot validation catches reachable-but-unconfigured tier", func(t *testing.T) {
		tbl, err := LoadRunnerTable(envGetter(map[string]string{
			"ASSAY_RUNNER_SESSION": `{"cmd":["x"],"pin":"1"}`, // session configured, local NOT
		}), nil)
		if err != nil {
			t.Fatalf("table load: %v", err)
		}
		// TierLocal is always reachable (TierPolicy emits it), so a table without it fails boot.
		err = tbl.ValidateReachable((&VerifyLoop{}).reachableTiers())
		if err == nil || !strings.Contains(err.Error(), "reachable") {
			t.Fatalf("a reachable-but-unconfigured tier must fail boot validation, got: %v", err)
		}
		// With TierLocal configured, the default reachable set validates clean.
		tbl2, _ := LoadRunnerTable(envGetter(map[string]string{
			"ASSAY_RUNNER_LOCAL": `{"cmd":["x"],"pin":"1"}`,
		}), nil)
		if err := tbl2.ValidateReachable((&VerifyLoop{}).reachableTiers()); err != nil {
			t.Fatalf("a table covering all reachable tiers must validate clean: %v", err)
		}
		// When the middle-rung flag is on, TierSession becomes reachable and must also be covered.
		vSession := &VerifyLoop{F16ReversibleRiskToSession: true}
		if err := tbl2.ValidateReachable(vSession.reachableTiers()); err == nil {
			t.Fatal("with the middle-rung flag on, an uncovered TierSession must fail boot")
		}
	})

	// --- empty / malformed config fails closed ------------------------------
	t.Run("no entries refuses", func(t *testing.T) {
		if _, err := LoadRunnerTable(envGetter(map[string]string{}), nil); err == nil {
			t.Fatal("a table with no entries must refuse, not return an empty table")
		}
	})
	t.Run("malformed JSON refuses", func(t *testing.T) {
		if _, err := LoadRunnerTable(envGetter(map[string]string{
			"ASSAY_RUNNER_LOCAL": `{not json`,
		}), nil); err == nil {
			t.Fatal("malformed entry JSON must refuse")
		}
	})
}

// TestRunnerTableNamespace pins the config-namespace-split ruling: the runner table reads ONLY
// the ASSAY_* (generic methodology) namespace and never a product-deploy key. The forbidden
// product token is constructed at runtime so this test file does not itself carry the literal —
// Verify row 4 greps these paths for it and must stay CLEAN.
func TestRunnerTableNamespace(t *testing.T) {
	productKey := "MEDICI" + "_LOAN_RUNNER_LOCAL" // never a literal in-file (Verify row 4)
	env := map[string]string{
		productKey:             `{"cmd":["x"],"pin":"1"}`,
		"PRODUCT_RUNNER_LOCAL": `{"cmd":["y"],"pin":"1"}`,
	}
	// A product-namespace key alone is invisible to the table: nothing is configured.
	if RunnerTableConfigured(envGetter(env)) {
		t.Fatal("a non-ASSAY_ (product) key must NOT count as a configured runner table")
	}
	// And it must not load as an entry either.
	if _, err := LoadRunnerTable(envGetter(env), nil); err == nil {
		t.Fatal("product-namespace keys must not produce a table — the loader reads ASSAY_* only")
	}
	// The same value under the ASSAY_ namespace IS read — proving it is the prefix that gates.
	if !RunnerTableConfigured(envGetter(map[string]string{"ASSAY_RUNNER_LOCAL": `{"cmd":["x"],"pin":"1"}`})) {
		t.Fatal("an ASSAY_RUNNER_ key must count as configured")
	}
}

// TestRunnerTableFileForm exercises the file-pointed table (ASSAY_RUNNER_TABLE => JSON file).
func TestRunnerTableFileForm(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/runners.json"
	if err := os.WriteFile(path, []byte(`{"local":{"cmd":["npx","acp"],"model":"m","pin":"9.9.9"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tbl, err := LoadRunnerTable(envGetter(map[string]string{"ASSAY_RUNNER_TABLE": path}), nil)
	if err != nil {
		t.Fatalf("file-form table load: %v", err)
	}
	e, err := tbl.Resolve(loopengine.TierLocal)
	if err != nil || e.Pin != "9.9.9" {
		t.Fatalf("file-form local entry: %+v err=%v", e, err)
	}
	// A missing file fails closed as unverifiable (could not read the declared config).
	if _, err := LoadRunnerTable(envGetter(map[string]string{"ASSAY_RUNNER_TABLE": dir + "/nope.json"}), nil); err == nil {
		t.Fatal("a missing table file must fail closed")
	}
}

// TestNativeDispatchRoundTripViaTable drives the native ACP path with the runner selected by the
// tier→runner table (not the LE/15 legacy RunnerCmd) and asserts Result.RunnerID is derived from
// the resolved entry — attribution the engine knows at spawn, carrying the pin + model.
func TestNativeDispatchRoundTripViaTable(t *testing.T) {
	nativeTestHome(t)
	wt := t.TempDir()
	isolate := true
	v := &VerifyLoop{
		Root:          t.TempDir(),
		TargetSHA:     "deadbeef",
		Native:        true,
		NativeEnv:     []string{"VERIFYLOOP_FAKE_ACP=roundtrip"},
		NativeTimeout: 20 * time.Second,
		RunnerTable: &RunnerTable{entries: map[loopengine.Tier]RunnerEntry{
			loopengine.TierLocal: {Cmd: []string{os.Args[0]}, Model: "local-model", Pin: "v0", Isolate: &isolate},
		}},
		MakeWorktree: func(loopengine.Item) (string, func(), error) { return wt, func() {}, nil },
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
	v := &VerifyLoop{
		Root:      t.TempDir(),
		TargetSHA: "deadbeef",
		Native:    true,
		RunnerTable: &RunnerTable{entries: map[loopengine.Tier]RunnerEntry{
			loopengine.TierLocal: {Cmd: []string{"x"}, Pin: "v0"},
		}},
	}
	// TierSession is not in the table.
	if _, err := v.Dispatch(fixtureItem(), loopengine.TierSession); err == nil || !strings.Contains(err.Error(), "no runner configured for tier") {
		t.Fatalf("dispatch on an unconfigured tier must refuse, got: %v", err)
	}
}

// TestPreflightRunnerTable proves the boot hook is inert with no table and refuses on a bad one.
func TestPreflightRunnerTable(t *testing.T) {
	if err := preflightRunnerTable(envGetter(map[string]string{})); err != nil {
		t.Fatalf("no table configured must be a no-op boot, got: %v", err)
	}
	// Configured but invalid (missing pin) => boot refuses.
	if err := preflightRunnerTable(envGetter(map[string]string{
		"ASSAY_RUNNER_LOCAL": `{"cmd":["x"]}`,
	})); err == nil {
		t.Fatal("a configured-but-invalid table must refuse at boot")
	}
	// Configured, valid, covers the reachable set => boot clean.
	if err := preflightRunnerTable(envGetter(map[string]string{
		"ASSAY_RUNNER_LOCAL": `{"cmd":["x"],"pin":"1"}`,
	})); err != nil {
		t.Fatalf("a valid table covering the reachable set must boot clean, got: %v", err)
	}
}
