package loopengine

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// setupDeskHome points $HOME at a fresh temp dir so deskkit.Guard() reads a clean
// desk-tools dir (no ambient DISABLED/STOP flag can leak in from the real ~/.claude),
// neutralises the ambient kill-switch env, and pins DESK_LOOP to loopName so per-loop
// STOP.<loop> flags are honoured deterministically. Returns the desk-tools dir path so a
// test can plant flag files (STOP, STOP.<loop>, DISABLED).
func setupDeskHome(t *testing.T, loopName string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DESK_TOOLS_DISABLED", "") // "" is not "1" => not armed
	t.Setenv("CLAUDE_SESSION_ID", "loopengine-test")
	t.Setenv("DESK_LOOP", loopName)
	return filepath.Join(home, ".config", "assay")
}

// fakeHandle is an in-flight dispatched worker whose Done() channel the test (or the
// default dispatch) feeds a structured Result on — the interim-mode "operator feeds the
// result back" made deterministic.
type fakeHandle struct {
	item Item
	done chan Result
}

func (h *fakeHandle) Done() <-chan Result { return h.done }
func (h *fakeHandle) Item() Item          { return h.item }

// fakeLoop is a controllable Loop for engine tests. remaining is the queue SelectQueue
// returns; the default Land removes the landed item from remaining, so the queue drains to
// empty as results land. All hooks are overridable via the *Fn fields.
type fakeLoop struct {
	name string

	mu         sync.Mutex
	remaining  []Item
	dispatched []string
	landed     []Result
	onIdleN    int

	selectErr  error
	tierFn     func(Item) (Tier, error)
	dispatchFn func(*fakeLoop, Item, Tier) (Handle, error)
	landFn     func(*fakeLoop, Result) error
	onIdleFn   func(*fakeLoop) error
}

func (l *fakeLoop) Name() string { return l.name }

func (l *fakeLoop) SelectQueue() ([]Item, error) {
	if l.selectErr != nil {
		return nil, l.selectErr
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Item, len(l.remaining))
	copy(out, l.remaining)
	return out, nil
}

func (l *fakeLoop) TierPolicy(it Item) (Tier, error) {
	if l.tierFn != nil {
		return l.tierFn(it)
	}
	return TierLocal, nil
}

func (l *fakeLoop) Dispatch(it Item, tier Tier) (Handle, error) {
	l.mu.Lock()
	l.dispatched = append(l.dispatched, it.ID)
	l.mu.Unlock()
	if l.dispatchFn != nil {
		return l.dispatchFn(l, it, tier)
	}
	// Default: complete shortly with a PASS, one Evidence row.
	h := &fakeHandle{item: it, done: make(chan Result, 1)}
	go func() {
		time.Sleep(2 * time.Millisecond)
		h.done <- Result{
			Item:     it,
			Verdict:  VerdictPass,
			RunnerID: "local:test-runner",
			Rows:     []EvidenceRow{{Command: "go test ./...", Exit: 0, Output: "ok"}},
		}
	}()
	return h, nil
}

func (l *fakeLoop) Land(r Result) error {
	if l.landFn != nil {
		return l.landFn(l, r)
	}
	l.recordLandAndDrain(r)
	return nil
}

// recordLandAndDrain records the landed result and removes the item from remaining so the
// queue drains. Used by the default Land and by custom landFns that still want to drain.
func (l *fakeLoop) recordLandAndDrain(r Result) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.landed = append(l.landed, r)
	l.removeLocked(r.Item.ID)
}

func (l *fakeLoop) removeLocked(id string) {
	out := l.remaining[:0]
	for _, it := range l.remaining {
		if it.ID != id {
			out = append(out, it)
		}
	}
	l.remaining = out
}

func (l *fakeLoop) OnIdle() error {
	l.mu.Lock()
	l.onIdleN++
	l.mu.Unlock()
	if l.onIdleFn != nil {
		return l.onIdleFn(l)
	}
	return nil
}

func (l *fakeLoop) landedIDs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	ids := make([]string, 0, len(l.landed))
	for _, r := range l.landed {
		ids = append(ids, r.Item.ID)
	}
	return ids
}

func (l *fakeLoop) dispatchedIDs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.dispatched))
	copy(out, l.dispatched)
	return out
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
