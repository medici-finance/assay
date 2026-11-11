//go:build pirunner

package pirunner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/medici-finance/assay/loopadmin/runner"
)

// ErrOwned: another launch owner holds the state directory. Pi's SQLite
// storage has no cross-process locking, so exclusivity is this owner's job.
var ErrOwned = errors.New("pirunner: state directory has another launch owner")

// Step is one scripted tool call the faux model makes before it waits for the
// test owner's result.
type Step struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	CallID string         `json:"callId"`
}

// Counters are the bridge's observable events, for attempts and launches.
type Counters struct {
	mu sync.Mutex
	Counts
}

// Counts is a point-in-time copy of the counters.
type Counts struct {
	ModelCalls  int
	BeforeTool  int
	ToolExec    map[string]int
	Submits     int
	ChildPIDs   []int
	ToolHeld    map[string]int
	BridgeStart int
}

// Snapshot copies the counters.
func (c *Counters) Snapshot() Counts {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := Counts{ModelCalls: c.ModelCalls, BeforeTool: c.BeforeTool, Submits: c.Submits, BridgeStart: c.BridgeStart, ToolExec: map[string]int{}, ToolHeld: map[string]int{}}
	for k, v := range c.ToolExec {
		out.ToolExec[k] = v
	}
	for k, v := range c.ToolHeld {
		out.ToolHeld[k] = v
	}
	out.ChildPIDs = append(out.ChildPIDs, c.ChildPIDs...)
	return out
}

// Host is the launch owner: it holds an exclusive lock on the state
// directory for its lifetime and runs at most one bridge child at a time.
type Host struct {
	dir, node, script string
	lock              *os.File
	Ledger            *Ledger
	Counters          *Counters

	mu     sync.Mutex
	b      *bridge
	runGen map[string]uint64 // invocation key -> generation it was launched under
	faults msg
}

// OpenHost takes ownership of dir and starts the bridge child.
func OpenHost(dir, node, script string) (*Host, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "owner.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("%w: %v", ErrOwned, err)
	}
	h := &Host{dir: dir, node: node, script: script, lock: f, Ledger: NewLedger(), Counters: &Counters{Counts: Counts{ToolExec: map[string]int{}, ToolHeld: map[string]int{}}}, runGen: map[string]uint64{}, faults: msg{}}
	if err := h.spawn(); err != nil {
		h.release()
		return nil, err
	}
	return h, nil
}

func (h *Host) spawn() error {
	b, err := startBridge(h.node, h.script, h.dir, h.callback, h.event)
	if err != nil {
		return err
	}
	h.b = b
	h.Counters.mu.Lock()
	h.Counters.BridgeStart++
	h.Counters.mu.Unlock()
	if len(h.faults) > 0 {
		if _, err := b.call(context.Background(), "faults", h.faults); err != nil {
			return err
		}
	}
	return nil
}

func (h *Host) bridge() *bridge {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.b
}

// Kill SIGKILLs the current child and waits until it has been reaped. No
// successor is started; Restart does that.
func (h *Host) Kill() error {
	return h.bridge().kill()
}

// Restart kills the current child, confirms it is gone, then starts a new
// one over the same state directory under the same held lock.
func (h *Host) Restart() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.b.kill(); err != nil {
		return err
	}
	return h.spawn()
}

// Close stops the child and releases the directory.
func (h *Host) Close() error {
	h.mu.Lock()
	err := h.b.close()
	h.mu.Unlock()
	for _, pid := range h.Counters.Snapshot().ChildPIDs {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	h.release()
	return err
}

func (h *Host) release() {
	_ = syscall.Flock(int(h.lock.Fd()), syscall.LOCK_UN)
	h.lock.Close()
}

// Stderr returns what the current child wrote to stderr.
func (h *Host) Stderr() string { return h.bridge().stderr.String() }

func (h *Host) call(ctx context.Context, op string, fields msg) (msg, error) {
	return h.bridge().call(ctx, op, fields)
}

func (h *Host) setFault(name string, v any) error {
	h.mu.Lock()
	h.faults[name] = v
	b := h.b
	h.mu.Unlock()
	_, err := b.call(context.Background(), "faults", msg{name: v})
	return err
}

func (h *Host) callback(op string, m msg) msg {
	key := fmt.Sprint(m["key"])
	h.mu.Lock()
	gen := h.runGen[key]
	h.mu.Unlock()
	switch op {
	case "authorize", "exec-check":
		return h.Ledger.check(op, key, gen)
	case "effect":
		return h.Ledger.effect(key, gen, fmt.Sprint(m["action"]), fmt.Sprint(m["target"]))
	}
	return msg{"status": "unknown-callback"}
}

func (h *Host) event(name string, m msg) {
	c := h.Counters
	c.mu.Lock()
	defer c.mu.Unlock()
	switch name {
	case "model-call":
		c.ModelCalls++
	case "before-tool":
		c.BeforeTool++
	case "tool-exec":
		c.ToolExec[fmt.Sprint(m["tool"])]++
	case "tool-held":
		c.ToolHeld[fmt.Sprint(m["tool"])]++
	case "submit":
		c.Submits++
	case "child":
		if pid, ok := m["pid"].(float64); ok && pid > 0 {
			c.ChildPIDs = append(c.ChildPIDs, int(pid))
		}
	}
}

// Adapter is the experimental runner.Adapter over the Host's bridge. It also
// implements conformance.Control and conformance.ModelSwitcher, driving the
// faults through the Node/Pi path rather than a Go substitute.
type Adapter struct {
	H *Host

	mu       sync.Mutex
	loseAck  bool
	reject   bool
	script   []Step
	scripted bool
	crash    string
}

// Capabilities declares only what the experiment shows: launch dedupe by
// durable identity mapping, the launch budget scope, partial telemetry (usage
// is reported only for finished runs). No resume, no cancel acknowledgment, no
// snapshot and no qualified execution profile.
func (a *Adapter) Capabilities() runner.Capabilities {
	return runner.Capabilities{
		LaunchDedupe: true,
		BudgetScopes: []runner.BudgetScope{runner.BudgetLaunch},
		Telemetry:    runner.TelemetryPartial,
	}
}

// Key is the invocation's stable, filesystem-safe identity.
func Key(caller, id string) string {
	h := sha256.Sum256([]byte(caller + "\x00" + id))
	return hex.EncodeToString(h[:12])
}

// NextScript sets the tool calls the next launched run makes.
func (a *Adapter) NextScript(steps ...Step) {
	a.mu.Lock()
	a.script, a.scripted = steps, true
	a.mu.Unlock()
}

// Start launches one invocation. The bridge writes the identity mapping
// durably before submitting and again before acknowledging, so a lost
// acknowledgment leaves a run Reconcile can find.
func (a *Adapter) Start(ctx context.Context, req runner.LaunchRequest) (runner.Receipt, error) {
	a.mu.Lock()
	reject, lose, script, crash := a.reject, a.loseAck, a.script, a.crash
	a.reject, a.loseAck, a.script, a.scripted, a.crash = false, false, nil, false, ""
	a.mu.Unlock()
	if script == nil {
		script = []Step{}
	}
	ref := req.Ref()
	key := Key(ref.Caller, ref.ID)
	desc, err := json.Marshal(req)
	if err != nil {
		return runner.Receipt{}, errors.Join(runner.ErrDefiniteFailure, err)
	}
	sum := sha256.Sum256(desc)
	a.H.mu.Lock()
	if _, ok := a.H.runGen[key]; !ok {
		a.H.runGen[key] = ref.Generation
	}
	a.H.mu.Unlock()
	a.H.Ledger.Grant(key, ref.Generation)
	r, err := a.H.call(ctx, "start", msg{
		"key": key, "caller": ref.Caller, "invId": ref.ID, "generation": ref.Generation,
		"descriptor": hex.EncodeToString(sum[:]), "model": req.Profile.Model, "script": script,
		"input": "run " + key, "reject": reject, "crash": crash,
	})
	if err != nil {
		return runner.Receipt{}, errors.Join(runner.ErrAckLost, err)
	}
	if ok, _ := r["ok"].(bool); !ok {
		if def, _ := r["definite"].(bool); def {
			return runner.Receipt{}, errors.Join(runner.ErrDefiniteFailure, fmt.Errorf("pirunner: %v", r["error"]))
		}
		return runner.Receipt{}, errors.Join(runner.ErrAckLost, fmt.Errorf("pirunner: %v", r["error"]))
	}
	if lose {
		return runner.Receipt{}, runner.ErrAckLost
	}
	return runner.Receipt{Ref: ref, SessionID: "pi:" + key + ":" + fmt.Sprint(r["sessionId"]), ActualModel: fmt.Sprint(r["actualModel"])}, nil
}

// Observe attaches read-only: it does not start Pi's scheduler.
func (a *Adapter) Observe(ctx context.Context, ref runner.Ref) (runner.Observation, error) {
	return a.observe(ctx, "observe", ref)
}

// Reconcile reopens and resumes the invocation's store after a restart.
func (a *Adapter) Reconcile(ctx context.Context, ref runner.Ref) (runner.Observation, error) {
	return a.observe(ctx, "reconcile", ref)
}

// Raw returns the bridge's own view of an invocation, for tests.
func (a *Adapter) Raw(ctx context.Context, op string, ref runner.Ref) (msg, error) {
	return a.H.call(ctx, op, msg{"key": Key(ref.Caller, ref.ID)})
}

func (a *Adapter) observe(ctx context.Context, op string, ref runner.Ref) (runner.Observation, error) {
	r, err := a.Raw(ctx, op, ref)
	if err != nil {
		return runner.Observation{State: runner.StateUnknown}, err
	}
	if ok, _ := r["ok"].(bool); !ok {
		return runner.Observation{State: runner.StateUnknown}, fmt.Errorf("pirunner: %s: %v", op, r["error"])
	}
	obs := runner.Observation{State: runner.State(fmt.Sprint(r["state"]))}
	if m, ok := r["model"].(string); ok {
		obs.ActualModel = m
	}
	if obs.State != runner.StateFinished {
		return obs, nil
	}
	var answer struct {
		Result *runner.Result `json:"result"`
		Usage  runner.Usage   `json:"usage"`
	}
	text, _ := r["result"].(string)
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		// A finished run whose answer is not the scripted shape: report it,
		// with no result, rather than inventing one.
		return obs, nil
	}
	obs.Result, obs.Usage = answer.Result, answer.Usage
	return obs, nil
}

// Cancel records the request durably and asks Pi to abort. Pi's abort is not
// an acknowledgment that every process the run started has stopped, so the
// adapter never claims one.
func (a *Adapter) Cancel(ctx context.Context, ref runner.Ref) (runner.CancelAck, error) {
	r, err := a.H.call(ctx, "cancel", msg{"key": Key(ref.Caller, ref.ID)})
	if err != nil {
		return runner.CancelAck{}, err
	}
	if ok, _ := r["ok"].(bool); !ok {
		return runner.CancelAck{}, fmt.Errorf("pirunner: cancel: %v", r["error"])
	}
	return runner.CancelAck{Acknowledged: false}, nil
}

// Complete supplies the run's scripted result and usage and waits until Pi
// has settled the submission.
func (a *Adapter) Complete(ref runner.Ref, res runner.Result, usage runner.Usage) {
	_, _ = a.H.call(context.Background(), "complete", msg{"key": Key(ref.Caller, ref.ID), "result": res, "usage": usage})
}

// CrashNextStart makes the bridge kill itself during the next start, at
// "after-intent" (mapping written, nothing submitted) or "after-submit"
// (submitted, mapping not yet finished, no acknowledgment).
func (a *Adapter) CrashNextStart(point string) { a.mu.Lock(); a.crash = point; a.mu.Unlock() }

// LoseNextLaunchAck: the next Start runs in Pi but its receipt is dropped.
func (a *Adapter) LoseNextLaunchAck() { a.mu.Lock(); a.loseAck = true; a.mu.Unlock() }

// RejectNextLaunch: the bridge refuses the next start before writing anything.
func (a *Adapter) RejectNextLaunch() { a.mu.Lock(); a.reject = true; a.mu.Unlock() }

// IgnoreCancel: the bridge records cancels but does not abort.
func (a *Adapter) IgnoreCancel() { _ = a.H.setFault("ignoreCancel", true) }

// RunModelInstead: the bridge configures model for later launches.
func (a *Adapter) RunModelInstead(model string) { _ = a.H.setFault("modelOverride", model) }

// DisableUpperHook removes the beforeTool authorization, so only the
// execution-time recheck guards tools.
func (a *Adapter) DisableUpperHook() { _ = a.H.setFault("noUpperHook", true) }

// Release opens a held tool's gate.
func (a *Adapter) Release(ctx context.Context, ref runner.Ref, step string) error {
	_, err := a.H.call(ctx, "release", msg{"key": Key(ref.Caller, ref.ID), "step": step})
	return err
}
