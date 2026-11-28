// Package fake holds in-memory prepared-environment adapters with scriptable
// state. They report what they hold and never decide readiness: the port's
// own validator (workspace.Match) does, for the fakes exactly as for a
// production adapter.
package fake

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"sync"

	"github.com/medici-finance/assay/loopadmin/workspace"
)

// ErrFault is what a scripted fault returns.
var ErrFault = errors.New("fake: scripted fault")

// ErrRevisions is what a scripted Revisions fault returns, distinct from
// ErrFault so a test can tell which adapter's failure a caller received.
var ErrRevisions = errors.New("fake: scripted revisions fault")

// Env is the fake resources, Git, authority, store and retirement in one
// value, so a test can script all of them. Use New.
type Env struct {
	mu sync.Mutex

	preps      map[string]string // cell+environment -> preparation
	prepEnv    map[string]workspace.Environment
	workspaces map[string]string // binding -> workspace
	effective  map[string]*workspace.Effective
	revisions  map[string][]workspace.Repository
	unpub      map[string]workspace.Presence
	forgotten  map[string]bool
	receipts   map[string]workspace.Receipt
	revoked    map[string]bool

	// Moved, when set, is the revision a checkout of that repository lands
	// on instead of the one asked for (a ref that moved under the caller).
	Moved map[string]string
	// FailWorkspace makes the next Workspace call fail after allocating.
	FailWorkspace bool
	// RetireUnknown makes Retire report an unknown outcome.
	RetireUnknown bool
	// FailPut makes every Put fail and store nothing.
	FailPut bool
	// FailRevisions makes every Revisions call fail.
	FailRevisions bool

	Applies, Checkouts, Restores int
	Retired                      []string
}

// New returns an empty fake environment.
func New() *Env {
	return &Env{
		preps: map[string]string{}, prepEnv: map[string]workspace.Environment{},
		workspaces: map[string]string{}, effective: map[string]*workspace.Effective{},
		revisions: map[string][]workspace.Repository{}, unpub: map[string]workspace.Presence{},
		forgotten: map[string]bool{}, receipts: map[string]workspace.Receipt{},
		revoked: map[string]bool{}, Moved: map[string]string{},
	}
}

// Port wires a port over this environment, with the continuity it declares.
func (e *Env) Port(c workspace.Port) *workspace.Port {
	c.Resources, c.Git, c.Authority, c.Store, c.Retirement = e, e, e, e, e
	if c.Validator == "" {
		c.Validator = "workspace.Match"
	}
	if c.Observer == "" {
		c.Observer = "fake"
	}
	return &c
}

// Preparation implements workspace.Resources.
func (e *Env) Preparation(_ context.Context, cell string, env workspace.Environment) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	b, _ := json.Marshal(env)
	k := cell + "\x00" + string(b)
	if id, ok := e.preps[k]; ok {
		return id, nil
	}
	id := "prep-" + strconv.Itoa(len(e.preps)+1)
	e.preps[k], e.prepEnv[id] = id, env
	return id, nil
}

// Workspace implements workspace.Resources.
func (e *Env) Workspace(_ context.Context, binding, prep string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ws, ok := e.workspaces[binding]
	if !ok {
		ws = "ws-" + strconv.Itoa(len(e.workspaces)+1)
		e.workspaces[binding] = ws
		// A deep copy: drifting one workspace never edits the preparation,
		// another workspace or the caller's request.
		var env workspace.Environment
		b, _ := json.Marshal(e.prepEnv[prep])
		_ = json.Unmarshal(b, &env)
		e.effective[ws] = &workspace.Effective{Environment: env}
		e.unpub[ws] = workspace.PresenceNone
	}
	if e.FailWorkspace {
		e.FailWorkspace = false
		return "", ErrFault
	}
	return ws, nil
}

// Apply implements workspace.Resources.
func (e *Env) Apply(_ context.Context, ws string, writable []string, p workspace.Policy) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Applies++
	if eff := e.effective[ws]; eff != nil {
		eff.Writable, eff.Policy = slices.Clone(writable), p
	}
	return nil
}

// Inspect implements workspace.Resources.
func (e *Env) Inspect(_ context.Context, ws string) (workspace.Effective, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	eff, ok := e.effective[ws]
	if !ok || e.forgotten[ws] {
		return workspace.Effective{}, false, nil
	}
	return *eff, true, nil
}

// RestoreFiles implements workspace.Resources.
func (e *Env) RestoreFiles(_ context.Context, from, to string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Restores++
	e.revisions[to] = slices.Clone(e.revisions[from])
	e.unpub[to] = e.unpub[from]
	return nil
}

// Revisions implements workspace.Git.
func (e *Env) Revisions(_ context.Context, ws string) ([]workspace.Repository, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.FailRevisions {
		return nil, ErrRevisions
	}
	return slices.Clone(e.revisions[ws]), nil
}

// Checkout implements workspace.Git.
func (e *Env) Checkout(_ context.Context, ws string, repos []workspace.Repository) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Checkouts++
	var got []workspace.Repository
	for _, r := range repos {
		if m, ok := e.Moved[r.Name]; ok {
			r.Revision = m
		}
		got = append(got, r)
	}
	e.revisions[ws] = got
	return nil
}

// Unpublished implements workspace.Git.
func (e *Env) Unpublished(_ context.Context, ws string) (workspace.Presence, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if p, ok := e.unpub[ws]; ok {
		return p, nil
	}
	return workspace.PresenceUnknown, nil
}

// Current implements workspace.Authority.
func (e *Env) Current(_ context.Context, o workspace.Owner, i workspace.Instance) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.revoked[o.Key] || e.revoked[workspace.BindingKey(i)] {
		return ErrFault
	}
	return nil
}

// Get implements workspace.Store.
func (e *Env) Get(_ context.Context, binding string) (workspace.Receipt, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.receipts[binding]
	return r, ok, nil
}

// Put implements workspace.Store. It stores a round trip through JSON, so a
// receipt is recovered as a caller's store would recover it.
func (e *Env) Put(_ context.Context, binding string, r workspace.Receipt) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	var back workspace.Receipt
	if err := json.Unmarshal(b, &back); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.FailPut {
		return ErrFault
	}
	e.receipts[binding] = back
	return nil
}

// Retire implements workspace.Retirement: it records the hand-off.
func (e *Env) Retire(_ context.Context, r workspace.Receipt) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.RetireUnknown {
		return ErrFault
	}
	e.Retired = append(e.Retired, r.Allocation.Workspace)
	return nil
}

// Script hooks.

// Revoke makes the authority refuse an owner key or instance binding.
func (e *Env) Revoke(keyOrBinding string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.revoked[keyOrBinding] = true
}

// SetUnpublished sets a workspace's unpublished-file presence.
func (e *Env) SetUnpublished(ws string, p workspace.Presence) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.unpub[ws] = p
}

// Forget makes a workspace's allocation unknown to Inspect.
func (e *Env) Forget(ws string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.forgotten[ws] = true
}

// Drift edits a workspace's effective configuration in place.
func (e *Env) Drift(ws string, f func(*workspace.Effective)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if eff := e.effective[ws]; eff != nil {
		f(eff)
	}
}

// SetReceipt stores a receipt as-is, as a caller's store might hold it.
func (e *Env) SetReceipt(binding string, r workspace.Receipt) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.receipts[binding] = r
}

// Receipt returns the stored receipt of a binding.
func (e *Env) Receipt(binding string) (workspace.Receipt, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.receipts[binding]
	return r, ok
}
