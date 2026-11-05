// Package fake is an in-memory runner Adapter with scriptable faults. It makes
// no provider call and starts no process; it exists so the conformance kit, and
// every consumer of the contract, can exercise start, observe, cancel and
// reconcile offline.
package fake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"sync"

	"github.com/medici-finance/assay/loopadmin/runner"
)

// Adapter is the fake. The zero value is not usable; use New.
type Adapter struct {
	mu   sync.Mutex
	caps runner.Capabilities
	runs map[string]*run

	loseNextAck   bool
	rejectNext    bool
	ignoreCancel  bool
	modelOverride string
	starts        int

	// The execution-profile extension: an adapter with no qualified profile
	// is an old adapter that does not know it.
	profiles        []runner.QualifiedProfile
	backendOverride *runner.Backend
	restored        *runner.Preservation
}

type run struct {
	req    runner.LaunchRequest
	state  runner.State
	usage  runner.Usage
	res    *runner.Result
	model  string
	report map[string]json.RawMessage
}

// New returns a fake declaring the given capabilities.
func New(caps runner.Capabilities) *Adapter {
	return &Adapter{caps: caps, runs: map[string]*run{}}
}

// FullCapabilities declares everything the contract defines.
func FullCapabilities() runner.Capabilities {
	return runner.Capabilities{
		Resume: true, CancelAck: true, Snapshot: true, LaunchDedupe: true,
		BudgetScopes: []runner.BudgetScope{runner.BudgetLaunch, runner.BudgetRequest},
		Telemetry:    runner.TelemetryComplete,
	}
}

// MinimalCapabilities declares only what every adapter must: a launch-scoped
// budget. No resume, no cancel acknowledgment, no telemetry, no dedupe.
func MinimalCapabilities() runner.Capabilities {
	return runner.Capabilities{BudgetScopes: []runner.BudgetScope{runner.BudgetLaunch}, Telemetry: runner.TelemetryNone}
}

// Capabilities implements runner.Adapter.
func (a *Adapter) Capabilities() runner.Capabilities {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.caps
}

// Starts reports how many launches the fake actually began.
func (a *Adapter) Starts() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.starts
}

// Start implements runner.Adapter.
func (a *Adapter) Start(_ context.Context, req runner.LaunchRequest) (runner.Receipt, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.rejectNext {
		a.rejectNext = false
		return runner.Receipt{}, errors.Join(runner.ErrDefiniteFailure, errors.New("fake: launch rejected"))
	}
	ref := req.Ref()
	key := ref.Caller + "/" + ref.ID
	if r, ok := a.runs[key]; ok && a.caps.LaunchDedupe {
		return runner.Receipt{Ref: ref, ActualModel: r.model, Extensions: r.report}, nil
	}
	report, err := a.admit(req)
	if err != nil {
		return runner.Receipt{}, errors.Join(runner.ErrDefiniteFailure, err)
	}
	model := req.Profile.Model
	if a.modelOverride != "" {
		model = a.modelOverride
	}
	a.runs[key] = &run{req: req, state: runner.StateRunning, model: model, report: report}
	a.starts++
	if a.loseNextAck {
		a.loseNextAck = false
		// The launch happened; only the acknowledgment is lost.
		return runner.Receipt{}, runner.ErrAckLost
	}
	return runner.Receipt{Ref: ref, ActualModel: model, Extensions: report}, nil
}

// admit is the adapter's own execution-profile check. A fake that knows the
// extension refuses an unqualified request itself, whatever the client did,
// and reports what it ran: the declared tuple, unless a backend substitution
// or a partial restoration is scripted.
func (a *Adapter) admit(req runner.LaunchRequest) (map[string]json.RawMessage, error) {
	_, carried := req.Extensions[runner.ExecutionProfileExt]
	if len(a.profiles) == 0 || (!carried && !slices.Contains(req.Require, runner.RequireExecutionProfile)) {
		return nil, nil // an old adapter ignores the extension; so does a launch without it
	}
	need, err := runner.AdmitExecutionProfile(req, a.profiles)
	if err != nil {
		return nil, err
	}
	rep := runner.ExecutionReport{Digest: need.Digest, Harness: need.Harness, Backend: need.Backend}
	if a.backendOverride != nil {
		rep.Backend = *a.backendOverride
	}
	if req.Resume != nil {
		for _, p := range a.profiles {
			if p.Digest == need.Digest {
				restored := p.Preservation.Resume
				rep.Restored = &restored
			}
		}
		if a.restored != nil {
			restored := *a.restored
			rep.Restored = &restored
		}
	}
	return runner.ReportExtension(rep), nil
}

// Qualify makes the fake an adapter that knows the execution-profile
// extension and declares these qualified profiles.
func (a *Adapter) Qualify(profiles ...runner.QualifiedProfile) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.profiles = append(a.profiles, profiles...)
	for _, e := range a.caps.Extensions {
		if e == runner.ExecutionProfileExt {
			return
		}
	}
	a.caps.Extensions = append(a.caps.Extensions, runner.ExecutionProfileExt)
}

// QualifiedProfiles implements runner.ProfileDeclarer. An old fake declares
// none.
func (a *Adapter) QualifiedProfiles() []runner.QualifiedProfile {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]runner.QualifiedProfile(nil), a.profiles...)
}

// SubstituteBackend makes later runs execute on, and report, another backend
// than the qualified one.
func (a *Adapter) SubstituteBackend(b runner.Backend) {
	a.mu.Lock()
	a.backendOverride = &b
	a.mu.Unlock()
}

// ObserveRestored makes later resumed runs report this restoration instead of
// the declared one: a profile can declare more than a run restores.
func (a *Adapter) ObserveRestored(p runner.Preservation) {
	a.mu.Lock()
	a.restored = &p
	a.mu.Unlock()
}

// Observe implements runner.Adapter.
func (a *Adapter) Observe(_ context.Context, ref runner.Ref) (runner.Observation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, ok := a.runs[ref.Caller+"/"+ref.ID]
	if !ok {
		return runner.Observation{State: runner.StateAbsent}, nil
	}
	return r.observation(), nil
}

// Reconcile implements runner.Adapter.
func (a *Adapter) Reconcile(ctx context.Context, ref runner.Ref) (runner.Observation, error) {
	return a.Observe(ctx, ref)
}

// Cancel implements runner.Adapter. Unless IgnoreCancel was set, the run stops.
func (a *Adapter) Cancel(_ context.Context, ref runner.Ref) (runner.CancelAck, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, ok := a.runs[ref.Caller+"/"+ref.ID]
	if !ok {
		return runner.CancelAck{}, errors.New("fake: no such run")
	}
	if !a.ignoreCancel && r.state == runner.StateRunning {
		r.state = runner.StateStopped
	}
	return runner.CancelAck{Acknowledged: true}, nil
}

func (r *run) observation() runner.Observation {
	return runner.Observation{State: r.state, ActualModel: r.model, Usage: r.usage, Result: r.res, Extensions: r.report}
}

// Complete drives a run to a finished observation with the given result and
// usage.
func (a *Adapter) Complete(ref runner.Ref, res runner.Result, usage runner.Usage) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if r, ok := a.runs[ref.Caller+"/"+ref.ID]; ok {
		r.res, r.usage, r.state = &res, usage, runner.StateFinished
	}
}

// LoseNextLaunchAck makes the next Start begin the run but report a lost
// acknowledgment.
func (a *Adapter) LoseNextLaunchAck() { a.mu.Lock(); a.loseNextAck = true; a.mu.Unlock() }

// RejectNextLaunch makes the next Start fail definitely, starting nothing.
func (a *Adapter) RejectNextLaunch() { a.mu.Lock(); a.rejectNext = true; a.mu.Unlock() }

// IgnoreCancel makes Cancel acknowledge without stopping the run.
func (a *Adapter) IgnoreCancel() { a.mu.Lock(); a.ignoreCancel = true; a.mu.Unlock() }

// RunModelInstead makes later runs report this model instead of the pinned one.
func (a *Adapter) RunModelInstead(model string) { a.mu.Lock(); a.modelOverride = model; a.mu.Unlock() }

// Digest returns a sha256 hex digest, for building well-formed artifacts.
func Digest(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
