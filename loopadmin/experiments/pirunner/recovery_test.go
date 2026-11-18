//go:build pirunner

package pirunner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/medici-finance/assay/loopadmin/runner"
	"github.com/medici-finance/assay/loopadmin/runner/conformance"
)

func pilot(t *testing.T) *Adapter { return &Adapter{H: newHost(t)} }

func start(t *testing.T, a *Adapter, id string, steps ...Step) runner.LaunchRequest {
	t.Helper()
	req := pilotReq("desk-worker", id, 1)
	a.NextScript(steps...)
	if _, err := a.Start(ctxT(t), req); err != nil {
		t.Fatalf("start %s: %v", id, err)
	}
	return req
}

func observe(t *testing.T, a *Adapter, req runner.LaunchRequest, reconcile bool) runner.Observation {
	t.Helper()
	f := a.Observe
	if reconcile {
		f = a.Reconcile
	}
	obs, err := f(ctxT(t), req.Ref())
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	return obs
}

func wantState(t *testing.T, what string, got, want runner.State) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: state %q, want %q", what, got, want)
	}
}

func step(tool, call string, args map[string]any) Step {
	if args == nil {
		args = map[string]any{}
	}
	return Step{Tool: tool, CallID: call, Args: args}
}

func entries(t *testing.T, a *Adapter, req runner.LaunchRequest) []map[string]any {
	t.Helper()
	r, err := a.Raw(ctxT(t), "entries", req.Ref())
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, e := range r["entries"].([]any) {
		out = append(out, e.(map[string]any))
	}
	return out
}

func toolResults(es []map[string]any, tool string) (ok, failed int, texts []string) {
	for _, e := range es {
		if e["kind"] != "pi.tool-result" || e["tool"] != tool {
			continue
		}
		texts = append(texts, fmt.Sprint(e["text"]))
		if e["isError"] == true {
			failed++
		} else {
			ok++
		}
	}
	return ok, failed, texts
}

// TestPiPilotRestart kills the real Node process mid-run and reopens the same
// SQLite store: the durable submission survives, the identity mapping settles
// both crash windows of a start, and only one owner may hold the directory.
// regression: #6208
func TestPiPilotRestart(t *testing.T) {
	t.Run("KillMidTool", func(t *testing.T) {
		a := pilot(t)
		req := start(t, a, "kill-mid-tool", step("slow_tool", "c1", map[string]any{"step": "s1"}))
		waitFor(t, "slow tool to run", held(a.H, "slow_tool"))
		before := observe(t, a, req, false)
		if err := a.H.Restart(); err != nil {
			t.Fatal(err)
		}
		obs := observe(t, a, req, true)
		wantState(t, "after restart", obs.State, runner.StateRunning)
		a.Complete(req.Ref(), conformance.GoodResult(req), conformance.FullUsage())
		obs = observe(t, a, req, false)
		wantState(t, "after complete", obs.State, runner.StateFinished)
		if obs.Result == nil || obs.Result.ID != req.ID {
			t.Fatalf("result lost across restart: %+v", obs.Result)
		}
		c := a.H.Counters.Snapshot()
		if c.Submits != 1 || c.BridgeStart != 2 {
			t.Fatalf("submits %d bridges %d; want one launch across two processes", c.Submits, c.BridgeStart)
		}
		if c.ToolExec["slow_tool"] != 1 {
			t.Fatalf("unsafe slow_tool ran %d times; an interrupted unsafe tool must not rerun", c.ToolExec["slow_tool"])
		}
		_, failed, _ := toolResults(entries(t, a, req), "slow_tool")
		if failed != 1 {
			t.Fatalf("interrupted unsafe tool should settle as one error result, got %d", failed)
		}
		_ = before
	})

	t.Run("CrashAfterIntent", func(t *testing.T) {
		a := pilot(t)
		req := pilotReq("desk-worker", "crash-intent", 1)
		a.CrashNextStart("after-intent")
		_, err := a.Start(ctxT(t), req)
		if !errors.Is(err, runner.ErrAckLost) {
			t.Fatalf("crashed start: %v, want ErrAckLost", err)
		}
		if err := a.H.Restart(); err != nil {
			t.Fatal(err)
		}
		wantState(t, "reconcile", observe(t, a, req, true).State, runner.StateAbsent)
		_, err = a.Start(ctxT(t), req)
		if !errors.Is(err, runner.ErrDefiniteFailure) {
			t.Fatalf("abandoned identity relaunched: %v", err)
		}
		if n := a.H.Counters.Snapshot().Submits; n != 0 {
			t.Fatalf("abandoned start reached Pi: %d", n)
		}
	})

	t.Run("CrashAfterSubmit", func(t *testing.T) {
		a := pilot(t)
		req := pilotReq("desk-worker", "crash-submit", 1)
		a.CrashNextStart("after-submit")
		if _, err := a.Start(ctxT(t), req); !errors.Is(err, runner.ErrAckLost) {
			t.Fatalf("crashed start: %v, want ErrAckLost", err)
		}
		if err := a.H.Restart(); err != nil {
			t.Fatal(err)
		}
		wantState(t, "reconcile", observe(t, a, req, true).State, runner.StateRunning)
		rec, err := a.Start(ctxT(t), req)
		if err != nil || rec.Ref != req.Ref() {
			t.Fatalf("dedupe after crash: %+v %v", rec, err)
		}
		a.Complete(req.Ref(), conformance.GoodResult(req), conformance.FullUsage())
		wantState(t, "complete", observe(t, a, req, false).State, runner.StateFinished)
		if n := a.H.Counters.Snapshot().Submits; n != 1 {
			t.Fatalf("submissions %d, want exactly one launch", n)
		}
	})

	t.Run("OneOwner", func(t *testing.T) {
		a := pilot(t)
		node, script := bridgeScript(t)
		if _, err := OpenHost(a.H.dir, node, script); !errors.Is(err, ErrOwned) {
			t.Fatalf("second owner admitted: %v", err)
		}
	})

	t.Run("Corpus", func(t *testing.T) {
		for _, n := range []int{100, 1000} {
			m := measure(t, n)
			line, _ := json.Marshal(m)
			t.Logf("pirunner-measure %s", line)
			if m.ToolResults != n {
				t.Fatalf("corpus %d: %d tool results", n, m.ToolResults)
			}
		}
	})
}

// Measurement characterises the experiment at a fixed corpus; it is not a
// capacity or cost claim.
type Measurement struct {
	ToolResults  int     `json:"tool_results"`
	Entries      int     `json:"entries"`
	InputBytes   int     `json:"input_bytes"`
	StoreBytes   int64   `json:"store_bytes"`
	LogBytes     int64   `json:"log_bytes"`
	ModelCalls   int     `json:"model_calls"`
	RunSeconds   float64 `json:"run_seconds"`
	ReopenSecond float64 `json:"reopen_seconds"`
	PeakRSS      int64   `json:"peak_rss_bytes"`
	RSSCoverage  string  `json:"rss_coverage"`
}

func measure(t *testing.T, n int) Measurement {
	a := pilot(t)
	steps := make([]Step, n)
	for i := range steps {
		steps[i] = step("safe_probe", fmt.Sprintf("p%d", i), map[string]any{"step": fmt.Sprintf("p%d", i)})
	}
	in, _ := json.Marshal(steps)
	t0 := time.Now()
	req := start(t, a, fmt.Sprintf("corpus-%d", n), steps...)
	a.Complete(req.Ref(), conformance.GoodResult(req), conformance.FullUsage())
	run := time.Since(t0)
	wantState(t, "corpus", observe(t, a, req, false).State, runner.StateFinished)
	st, err := a.Raw(ctxT(t), "stats", req.Ref())
	if err != nil {
		t.Fatal(err)
	}
	ok, _, _ := toolResults(entries(t, a, req), "safe_probe")
	if err := a.H.Restart(); err != nil {
		t.Fatal(err)
	}
	t1 := time.Now()
	wantState(t, "reopen", observe(t, a, req, true).State, runner.StateFinished)
	reopen := time.Since(t1)
	var store, wal int64
	files, _ := filepath.Glob(filepath.Join(a.H.dir, "inv", Key(req.Caller, req.ID)+".sqlite*"))
	for _, f := range files {
		if fi, err := os.Stat(f); err == nil {
			if strings.HasSuffix(f, "-wal") {
				wal += fi.Size()
			} else {
				store += fi.Size()
			}
		}
	}
	rss, _ := st["rss"].(float64)
	ents, _ := st["entries"].(float64)
	return Measurement{
		ToolResults: ok, Entries: int(ents), InputBytes: len(in), StoreBytes: store, LogBytes: wal,
		ModelCalls: a.H.Counters.Snapshot().ModelCalls, RunSeconds: run.Seconds(), ReopenSecond: reopen.Seconds(),
		PeakRSS: int64(rss), RSSCoverage: "bridge RSS sampled after the run; not a continuous peak",
	}
}

// TestPiPilotEffects routes consequential synthetic writes through the
// separate ledger with stable logical identity, and loses a reply mid-flight.
// regression: #6208
func TestPiPilotEffects(t *testing.T) {
	t.Run("LostReply", func(t *testing.T) {
		a := pilot(t)
		key := Key("desk-worker", "lost-reply")
		id := EffectID(key, "merge", "pr-1")
		a.H.Ledger.LoseReplyFor(id)
		killed := make(chan struct{})
		a.H.Ledger.LostReply = func(string) { _ = a.H.Kill(); close(killed) }
		req := start(t, a, "lost-reply", step("effect_write", "e1", map[string]any{"action": "merge", "target": "pr-1"}))
		<-killed
		if err := a.H.Restart(); err != nil {
			t.Fatal(err)
		}
		wantState(t, "reconcile", observe(t, a, req, true).State, runner.StateRunning)
		a.Complete(req.Ref(), conformance.GoodResult(req), conformance.FullUsage())
		wantState(t, "complete", observe(t, a, req, false).State, runner.StateFinished)
		if got := a.H.Ledger.Applied(id); got != 1 {
			t.Fatalf("effect applied %d times across a lost reply, want 1", got)
		}
		if got := a.H.Counters.Snapshot().ToolExec["effect_write"]; got != 1 {
			t.Fatalf("unsafe effect tool executed %d times, want 1 (no blind replay)", got)
		}
		_, failed, texts := toolResults(entries(t, a, req), "effect_write")
		if failed != 1 {
			t.Fatalf("the tool outcome after a lost reply must stay an error/unknown, got %d errors %q", failed, texts)
		}
	})

	t.Run("StableIdentity", func(t *testing.T) {
		a := pilot(t)
		args := map[string]any{"action": "merge", "target": "pr-2"}
		req := start(t, a, "stable", step("effect_write", "e1", args), step("effect_write", "e2", args))
		a.Complete(req.Ref(), conformance.GoodResult(req), conformance.FullUsage())
		id := EffectID(Key(req.Caller, req.ID), "merge", "pr-2")
		if got := a.H.Ledger.Applied(id); got != 1 {
			t.Fatalf("one logical effect applied %d times", got)
		}
		if a.H.Ledger.Duplicates != 1 || a.H.Ledger.Attempts != 2 {
			t.Fatalf("attempts %d duplicates %d", a.H.Ledger.Attempts, a.H.Ledger.Duplicates)
		}
	})

	t.Run("OutcomesSeparate", func(t *testing.T) {
		// A run whose tool failed still finishes as a session; acceptance of the
		// work is the client's separate decision on the result.
		a := pilot(t)
		key := Key("desk-worker", "separate")
		a.H.Ledger.SetBudget(key, 0)
		req := start(t, a, "separate", step("effect_write", "e1", map[string]any{"action": "merge", "target": "pr-3"}))
		res := conformance.GoodResult(req)
		res.Outcome = runner.OutcomeFailure
		a.Complete(req.Ref(), res, conformance.FullUsage())
		obs := observe(t, a, req, false)
		wantState(t, "session", obs.State, runner.StateFinished)
		if obs.Result == nil || obs.Result.Outcome != runner.OutcomeFailure {
			t.Fatalf("work outcome not separate from session outcome: %+v", obs.Result)
		}
		if _, failed, _ := toolResults(entries(t, a, req), "effect_write"); failed != 1 {
			t.Fatal("refused effect not reported as a failed tool outcome")
		}
	})
}

// TestPiPilotAuthority proves current authority is asked at execution time,
// including on a replay that skips beforeTool.
// regression: #6208
func TestPiPilotAuthority(t *testing.T) {
	t.Run("ReplayRechecks", func(t *testing.T) {
		a := pilot(t)
		req := start(t, a, "replay", step("safe_probe", "p1", map[string]any{"step": "s1", "hold": true}))
		waitFor(t, "probe held", held(a.H, "safe_probe"))
		a.H.Ledger.Supersede(Key(req.Caller, req.ID), 2)
		if err := a.H.Restart(); err != nil {
			t.Fatal(err)
		}
		observe(t, a, req, true)
		// The bridge emits tool-exec only after the exec-check is answered,
		// so the replay's verdict is recorded once this count is reached.
		waitFor(t, "safe probe replay", func() bool { return a.H.Counters.Snapshot().ToolExec["safe_probe"] >= 2 })
		// The session keeps running after the refused replay, and later model
		// turns may call beforeTool again, so read the count the replay itself
		// saw rather than the live total.
		if seen := a.H.Counters.Snapshot().ExecBeforeTool["safe_probe"][1]; seen != 1 {
			t.Fatalf("beforeTool ran %d times; replay was expected to skip it", seen)
		}
		if !refused(a.H.Ledger, "exec-check", "stale-generation") {
			t.Fatalf("replayed safe tool was not refused at execution: %v", refusals(a.H.Ledger))
		}
	})

	t.Run("LedgerAlone", func(t *testing.T) {
		a := pilot(t)
		a.DisableUpperHook()
		key := Key("desk-worker", "ledger-alone")
		a.H.Ledger.Pause(key)
		req := start(t, a, "ledger-alone", step("effect_write", "e1", map[string]any{"action": "merge", "target": "pr-4"}))
		a.Complete(req.Ref(), conformance.GoodResult(req), conformance.FullUsage())
		if a.H.Ledger.Applied(EffectID(key, "merge", "pr-4")) != 0 || !refused(a.H.Ledger, "effect", "paused") {
			t.Fatalf("paused effect applied without the upper hook: %v", refusals(a.H.Ledger))
		}
	})

	t.Run("UpperHookBlocks", func(t *testing.T) {
		a := pilot(t)
		key := Key("desk-worker", "upper")
		a.H.Ledger.Pause(key)
		req := start(t, a, "upper", step("effect_write", "e1", map[string]any{"action": "merge", "target": "pr-5"}))
		a.Complete(req.Ref(), conformance.GoodResult(req), conformance.FullUsage())
		if a.H.Counters.Snapshot().ToolExec["effect_write"] != 0 || !refused(a.H.Ledger, "authorize", "paused") {
			t.Fatal("beforeTool did not block a paused effect")
		}
	})
}

// refusals copies the ledger's refusals under its lock.
func refusals(l *Ledger) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.Refusals...)
}

func refused(l *Ledger, kind, why string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, r := range l.Refusals {
		if strings.HasPrefix(r, kind+":") && strings.HasSuffix(r, ":"+why) {
			return true
		}
	}
	return false
}

// TestPiPilotStopAndObserve: an abort request is not a stop while anything it
// started still runs, an ignored cancel stays running, and Observe attaches
// without resuming work.
// regression: #6208
func TestPiPilotStopAndObserve(t *testing.T) {
	t.Run("AbortHoldsForChild", func(t *testing.T) {
		a := pilot(t)
		req := start(t, a, "child", step("spawn_child", "k1", map[string]any{"step": "k1"}), step("slow_tool", "s2", map[string]any{"step": "s2"}))
		waitFor(t, "slow tool", held(a.H, "slow_tool"))
		ack, err := a.Cancel(ctxT(t), req.Ref())
		if err != nil || ack.Acknowledged {
			t.Fatalf("cancel ack %+v %v; abort must not claim acknowledgment", ack, err)
		}
		wantState(t, "child alive", observe(t, a, req, false).State, runner.StateRunning)
		for _, pid := range a.H.Counters.Snapshot().ChildPIDs {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
		waitFor(t, "stopped", func() bool { return observe(t, a, req, false).State == runner.StateStopped })
	})

	t.Run("IgnoredCancel", func(t *testing.T) {
		a := pilot(t)
		a.IgnoreCancel()
		req := start(t, a, "ignored", step("slow_tool", "s1", map[string]any{"step": "s1"}))
		waitFor(t, "slow tool", held(a.H, "slow_tool"))
		if _, err := a.Cancel(ctxT(t), req.Ref()); err != nil {
			t.Fatal(err)
		}
		wantState(t, "ignored cancel", observe(t, a, req, false).State, runner.StateRunning)
	})

	t.Run("ObserveReadOnly", func(t *testing.T) {
		a := pilot(t)
		req := start(t, a, "readonly", step("slow_tool", "s1", map[string]any{"step": "s1"}))
		waitFor(t, "slow tool", held(a.H, "slow_tool"))
		if err := a.H.Restart(); err != nil {
			t.Fatal(err)
		}
		calls := a.H.Counters.Snapshot().ModelCalls
		wantState(t, "observe", observe(t, a, req, false).State, runner.StateRunning)
		time.Sleep(300 * time.Millisecond)
		if c := a.H.Counters.Snapshot(); c.ModelCalls != calls || c.ToolExec["slow_tool"] != 1 {
			t.Fatalf("read-only observe resumed work: model calls %d->%d, tool runs %d", calls, c.ModelCalls, c.ToolExec["slow_tool"])
		}
	})
}

// TestPiPilotAccounting: usage is reported once, unknown cost stays unknown
// across a restart, pause survives a restart, and unsupported continuity or
// capability requirements refuse before launch.
// regression: #6208
func TestPiPilotAccounting(t *testing.T) {
	t.Run("PartialCostUnknown", func(t *testing.T) {
		a := pilot(t)
		req := start(t, a, "partial", step("slow_tool", "s1", map[string]any{"step": "s1"}))
		waitFor(t, "slow tool", held(a.H, "slow_tool"))
		if err := a.H.Restart(); err != nil {
			t.Fatal(err)
		}
		observe(t, a, req, true)
		partial := runner.Usage{InputTokens: runner.Int64(30), OutputTokens: runner.Int64(7)}
		a.Complete(req.Ref(), conformance.GoodResult(req), partial)
		obs := observe(t, a, req, false)
		if obs.Usage.CostMicros != nil {
			t.Fatalf("missing cost became %d", *obs.Usage.CostMicros)
		}
		if obs.Usage.InputTokens == nil || *obs.Usage.InputTokens != 30 {
			t.Fatalf("input tokens %v after a restart, want 30 counted once", obs.Usage.InputTokens)
		}
		u, err := a.Raw(ctxT(t), "usage", req.Ref())
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("pi-harness-usage %v (faux estimates; Pi has no unknown-cost value)", u["usage"])
	})

	t.Run("RunningUsageUnknown", func(t *testing.T) {
		a := pilot(t)
		req := start(t, a, "running-usage", step("slow_tool", "s1", map[string]any{"step": "s1"}))
		waitFor(t, "slow tool", held(a.H, "slow_tool"))
		u := observe(t, a, req, false).Usage
		if u.InputTokens != nil || u.OutputTokens != nil || u.CostMicros != nil {
			t.Fatalf("running usage reported as known: %+v", u)
		}
	})

	t.Run("PauseSurvivesRestart", func(t *testing.T) {
		a := pilot(t)
		key := Key("desk-worker", "pause-restart")
		req := start(t, a, "pause-restart", step("safe_probe", "p1", map[string]any{"step": "s1", "hold": true}), step("effect_write", "e1", map[string]any{"action": "merge", "target": "pr-6"}))
		waitFor(t, "probe held", held(a.H, "safe_probe"))
		a.H.Ledger.Pause(key)
		if err := a.H.Restart(); err != nil {
			t.Fatal(err)
		}
		observe(t, a, req, true)
		a.Complete(req.Ref(), conformance.GoodResult(req), conformance.FullUsage())
		if a.H.Ledger.Applied(EffectID(key, "merge", "pr-6")) != 0 {
			t.Fatal("pause lost across restart")
		}
	})

	t.Run("ContinuityRefused", func(t *testing.T) {
		a := pilot(t)
		c := runner.NewClient(a, conformance.NewClaims())
		for name, mut := range map[string]func(*runner.LaunchRequest){
			"resume":      func(r *runner.LaunchRequest) { r.Require = []string{string(runner.CapResume)} },
			"snapshot":    func(r *runner.LaunchRequest) { r.Require = []string{string(runner.CapSnapshot)} },
			"unknown-cap": func(r *runner.LaunchRequest) { r.Require = []string{"preserve-process"} },
			"profile":     func(r *runner.LaunchRequest) { r.Require = []string{runner.RequireExecutionProfile} },
		} {
			req := pilotReq("desk-worker", "cont-"+name, 1)
			mut(&req)
			if _, _, err := c.Start(ctxT(t), req); err == nil {
				t.Errorf("%s: launched without the capability", name)
			}
		}
		if n := a.H.Counters.Snapshot().Submits; n != 0 {
			t.Fatalf("refused requirement reached Pi: %d", n)
		}
	})
}
