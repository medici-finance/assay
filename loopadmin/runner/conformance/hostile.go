package conformance

import (
	"context"
	"sync"

	"github.com/medici-finance/assay/loopadmin/runner"
)

// Hostile wraps the adapter under test and overrides what it reports, so a
// case can drive the faulty or hostile adapter the contract must survive,
// whatever the adapter under test would itself do. Everything it does not
// override is forwarded unchanged.
type Hostile struct {
	runner.Adapter

	mu           sync.Mutex
	startErr     error
	startForward bool
	observeErr   error
	blankModel   bool
	reports      map[string]runner.Observation
}

// NewHostile wraps an adapter.
func NewHostile(a runner.Adapter) *Hostile {
	return &Hostile{Adapter: a, reports: map[string]runner.Observation{}}
}

func hostileKey(ref runner.Ref) string { return ref.Caller + "\x00" + ref.ID }

// FailNextStart makes the next Start return err. With forward the wrapped
// adapter is asked first, so the launch really happens and only the answer
// lies; without it nothing is launched.
func (h *Hostile) FailNextStart(err error, forward bool) {
	h.mu.Lock()
	h.startErr, h.startForward = err, forward
	h.mu.Unlock()
}

// FailNextObserve makes the next Observe or Reconcile return err.
func (h *Hostile) FailNextObserve(err error) {
	h.mu.Lock()
	h.observeErr = err
	h.mu.Unlock()
}

// BlankModel makes every receipt and observation omit the model.
func (h *Hostile) BlankModel() {
	h.mu.Lock()
	h.blankModel = true
	h.mu.Unlock()
}

// Report makes every later Observe and Reconcile of ref return obs.
func (h *Hostile) Report(ref runner.Ref, obs runner.Observation) {
	h.mu.Lock()
	h.reports[hostileKey(ref)] = obs
	h.mu.Unlock()
}

// Start implements runner.Adapter.
func (h *Hostile) Start(ctx context.Context, req runner.LaunchRequest) (runner.Receipt, error) {
	h.mu.Lock()
	err, forward, blank := h.startErr, h.startForward, h.blankModel
	h.startErr = nil
	h.mu.Unlock()
	if err != nil {
		if forward {
			_, _ = h.Adapter.Start(ctx, req)
		}
		return runner.Receipt{}, err
	}
	rec, err := h.Adapter.Start(ctx, req)
	if blank {
		rec.ActualModel = ""
	}
	return rec, err
}

// Observe implements runner.Adapter.
func (h *Hostile) Observe(ctx context.Context, ref runner.Ref) (runner.Observation, error) {
	return h.look(ctx, ref, h.Adapter.Observe)
}

// Reconcile implements runner.Adapter.
func (h *Hostile) Reconcile(ctx context.Context, ref runner.Ref) (runner.Observation, error) {
	return h.look(ctx, ref, h.Adapter.Reconcile)
}

func (h *Hostile) look(ctx context.Context, ref runner.Ref, q func(context.Context, runner.Ref) (runner.Observation, error)) (runner.Observation, error) {
	h.mu.Lock()
	err, blank := h.observeErr, h.blankModel
	h.observeErr = nil
	forced, ok := h.reports[hostileKey(ref)]
	h.mu.Unlock()
	if err != nil {
		return runner.Observation{}, err
	}
	if ok {
		return forced, nil
	}
	obs, err := q(ctx, ref)
	if blank {
		obs.ActualModel = ""
	}
	return obs, err
}
