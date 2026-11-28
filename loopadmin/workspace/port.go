package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"

	"github.com/medici-finance/assay/loopadmin/runner"
)

// Presence is whether a workspace holds files not yet published.
type Presence string

const (
	PresenceNone    Presence = "none"
	PresencePresent Presence = "present"
	PresenceUnknown Presence = "unknown"
)

// Effective is what an observer reports a workspace actually is.
type Effective struct {
	Repositories []Repository
	Environment  Environment
	Writable     []string
	Policy       Policy
}

// Resources allocates and inspects preparations and workspaces. It never
// deletes: release delegates to Retirement.
type Resources interface {
	// Preparation returns the shared preparation for a cell and environment,
	// building it if absent. Instances of one cell share it.
	Preparation(ctx context.Context, cell string, env Environment) (string, error)
	// Workspace returns the writable workspace of a binding over a
	// preparation, allocating it if absent; the same binding always names
	// the same workspace.
	Workspace(ctx context.Context, binding, preparation string) (string, error)
	// Apply sets a workspace's writable paths and policy references.
	Apply(ctx context.Context, workspace string, writable []string, policy Policy) error
	// Inspect reports a workspace's effective environment, writable paths
	// and policy (not its repositories: Git reports those). known is false
	// when the allocation cannot be found or read.
	Inspect(ctx context.Context, workspace string) (eff Effective, known bool, err error)
	// RestoreFiles copies a source workspace's files into a new one.
	RestoreFiles(ctx context.Context, from, to string) error
}

// Git reads and moves repository revisions in a workspace.
type Git interface {
	Revisions(ctx context.Context, workspace string) ([]Repository, error)
	Checkout(ctx context.Context, workspace string, repos []Repository) error
	Unpublished(ctx context.Context, workspace string) (Presence, error)
}

// Authority confirms that an owner still holds the instance at this
// generation. A nil error is the only confirmation.
type Authority interface {
	Current(ctx context.Context, owner Owner, instance Instance) error
}

// Store is caller-owned receipt storage keyed by binding.
type Store interface {
	Get(ctx context.Context, binding string) (Receipt, bool, error)
	Put(ctx context.Context, binding string, r Receipt) error
}

// Retirement is the existing retirement owner. Release delegates to it and
// never removes anything itself.
type Retirement interface {
	Retire(ctx context.Context, r Receipt) error
}

// Port prepares, reconciles and releases instance workspaces.
type Port struct {
	Resources  Resources
	Git        Git
	Authority  Authority
	Store      Store
	Retirement Retirement
	// Continuity is what this environment preserves across a restoration.
	Continuity runner.Preservation
	// Validator and Observer name the provenance a receipt records.
	Validator string
	Observer  string
}

// HoldReason is why the port held instead of acting.
type HoldReason string

const (
	HoldNoReceipt         HoldReason = "no receipt for the binding"
	HoldExpired           HoldReason = "an expired allocation never authorizes deletion"
	HoldUnknownAllocation HoldReason = "the allocation is unknown"
	HoldUnpublished       HoldReason = "the workspace holds unpublished or unknown files"
	HoldNoSource          HoldReason = "the restoration source has no ready receipt"
)

// Hold is the typed refusal to act on an uncertain or file-bearing
// allocation. errors.Is(h, ErrHold) holds.
type Hold struct{ Reason HoldReason }

func (h *Hold) Error() string        { return ErrHold.Error() + ": " + string(h.Reason) }
func (h *Hold) Is(target error) bool { return target == ErrHold }

// opaque carries an adapter error behind a sentinel whose text alone is shown.
type opaque struct{ sentinel, cause error }

func (o opaque) Error() string   { return o.sentinel.Error() }
func (o opaque) Unwrap() []error { return []error{o.sentinel, o.cause} }

// BindingKey is the readiness binding of one instance: cell, role, instance
// and generation. Two instances never share one, and a successor generation
// gets its own.
func BindingKey(i Instance) string {
	return i.Cell + "/" + i.Role + "/" + i.ID + "#" + strconv.FormatUint(i.Generation, 10)
}

// Release hands a clean, observed allocation to the retirement owner,
// whatever outcome its receipt records: a partial or not-ready workspace that
// is observable and holds no unpublished files is retired the same way. A
// missing receipt, an expired allocation, an unknown allocation and
// unpublished or unknown files each hold. Release checks no authority: the
// retirement owner fences which caller may retire which instance.
func (p *Port) Release(ctx context.Context, inst Instance, expired bool) error {
	r, found, err := p.Store.Get(ctx, BindingKey(inst))
	switch {
	case err != nil:
		return opaque{ErrAdapter, err}
	case !found:
		return &Hold{HoldNoReceipt}
	case r.Inputs.Instance != inst:
		return fmt.Errorf("%w: receipt.inputs.instance", ErrMismatch)
	case expired:
		return &Hold{HoldExpired}
	case r.Allocation.Workspace == "":
		return &Hold{HoldUnknownAllocation}
	}
	if _, known, err := p.Resources.Inspect(ctx, r.Allocation.Workspace); err != nil || !known {
		return &Hold{HoldUnknownAllocation}
	}
	if pres, err := p.Git.Unpublished(ctx, r.Allocation.Workspace); err != nil || pres != PresenceNone {
		return &Hold{HoldUnpublished}
	}
	if err := p.Retirement.Retire(ctx, r); err != nil {
		return opaque{ErrAdapter, err}
	}
	return nil
}

// Prepare makes the instance's workspace ready for req, or says why not.
//
// The request is validated and the owner confirmed current before anything
// else. A ready receipt whose recorded inputs equal req (policy included) is
// reused, re-observed; any other prior receipt is rebound: a partial or
// unknown one is reconciled first, and files are never checked out over
// while unpublished or unknown files are present. Every return path leaves
// the store holding the receipt as far as it got. A receipt returned with an
// error never reads ready: the caller cannot rely on what was stored.
func (p *Port) Prepare(ctx context.Context, req Request) (Receipt, error) {
	r, err := p.prepare(ctx, req)
	if err != nil && r.Outcome == OutcomeReady {
		r.Outcome = OutcomeUnknown
	}
	return r, err
}

func (p *Port) prepare(ctx context.Context, req Request) (Receipt, error) {
	if err := req.Validate(); err != nil {
		return Receipt{}, err
	}
	if err := p.Authority.Current(ctx, req.Owner, req.Instance); err != nil {
		return Receipt{}, opaque{ErrUnauthorized, err}
	}
	if req.Restore != nil {
		if err := runner.CheckPreservation(p.Continuity, req.Restore.Needs); err != nil {
			return Receipt{}, err
		}
	}
	key := BindingKey(req.Instance)
	prior, found, err := p.Store.Get(ctx, key)
	if err != nil {
		return Receipt{}, opaque{ErrAdapter, err}
	}
	if found && prior.Inputs.Instance != req.Instance {
		return Receipt{}, fmt.Errorf("%w: receipt.inputs.instance", ErrMismatch)
	}
	if found && reusable(prior, req) {
		return p.observe(ctx, key, prior)
	}

	r := Receipt{Version: Version, Inputs: req, Outcome: OutcomeUnknown,
		Validation: Validation{Validator: p.Validator, Observer: p.Observer}}
	if found {
		r.Allocation = prior.Allocation
		// A recorded restoration carries forward only while the request asks
		// for no other one; a changed source or changed needs restores again.
		if req.Restore == nil || sameRestore(prior.Restored, *req.Restore) {
			r.Restored = prior.Restored
		}
	}
	// The intent is durable before the first adapter call, so a crash
	// between here and the next Put leaves an unknown receipt to reconcile.
	if err := p.put(ctx, key, r); err != nil {
		return Receipt{}, err
	}
	prep, err := p.Resources.Preparation(ctx, req.Instance.Cell, req.Environment)
	if err != nil {
		return r, opaque{ErrAdapter, err}
	}
	ws, err := p.Resources.Workspace(ctx, key, prep)
	if err != nil {
		return r, opaque{ErrAdapter, err}
	}
	if r.Allocation.Workspace != "" && r.Allocation.Workspace != ws {
		return r, &Hold{HoldUnknownAllocation}
	}
	r.Allocation = Allocation{Preparation: prep, Workspace: ws}
	r.Outcome = OutcomePartial
	if err := p.put(ctx, key, r); err != nil {
		return r, err
	}
	if found && prior.Outcome != OutcomeReady {
		// Reconcile before retry: the earlier attempt's allocation must be
		// observable before anything is done to it.
		if _, known, err := p.Resources.Inspect(ctx, ws); err != nil || !known {
			return r, &Hold{HoldUnknownAllocation}
		}
	}
	if req.Restore != nil && r.Restored == nil {
		src, ok, err := p.Store.Get(ctx, BindingKey(req.Restore.Source))
		if err != nil || !ok || src.Outcome != OutcomeReady || src.Inputs.Instance != req.Restore.Source {
			return r, &Hold{HoldNoSource}
		}
		if req.Restore.Needs.Files {
			// The workspace may hold files whatever the store says of it (no
			// receipt, or an intent receipt with no allocation), so the copy
			// goes through the same guard as a checkout.
			if err := p.overwrite(ctx, ws, func() error {
				return p.Resources.RestoreFiles(ctx, src.Allocation.Workspace, ws)
			}); err != nil {
				return r, err
			}
		}
		r.Restored = &Restored{Source: req.Restore.Source, Preserved: req.Restore.Needs}
		if err := p.put(ctx, key, r); err != nil {
			return r, err
		}
	}
	actual, err := p.Git.Revisions(ctx, ws)
	if err != nil {
		return r, opaque{ErrAdapter, err}
	}
	if !sameRepos(actual, req.Repositories) {
		if err := p.overwrite(ctx, ws, func() error {
			return p.Git.Checkout(ctx, ws, req.Repositories)
		}); err != nil {
			return r, err
		}
	}
	if err := p.Resources.Apply(ctx, ws, req.Writable, req.Policy); err != nil {
		return r, opaque{ErrAdapter, err}
	}
	return p.observe(ctx, key, r)
}

// reusable: a ready receipt is reused only for the same invocation, the
// same current policy and the same every other input. Anything else rebinds.
func reusable(prior Receipt, req Request) bool {
	if prior.Outcome != OutcomeReady {
		return false
	}
	if prior.Inputs.Policy != req.Policy {
		return false
	}
	a, b := prior.Inputs, req
	a.Policy, b.Policy = Policy{}, Policy{}
	return reflect.DeepEqual(a, b)
}

// observe compares what the adapters report with every mandatory input and
// records the outcome. Ready only when everything matches.
func (p *Port) observe(ctx context.Context, key string, r Receipt) (Receipt, error) {
	eff, known, err := p.Resources.Inspect(ctx, r.Allocation.Workspace)
	if err != nil || !known {
		r.Outcome = OutcomeUnknown
		// A store that cannot record the unknown outcome may still hold an
		// earlier ready one: that is an adapter failure, not only a hold.
		if err := p.put(ctx, key, r); err != nil {
			return r, err
		}
		return r, &Hold{HoldUnknownAllocation}
	}
	if eff.Repositories, err = p.Git.Revisions(ctx, r.Allocation.Workspace); err != nil {
		r.Outcome = OutcomeUnknown
		if perr := p.put(ctx, key, r); perr != nil {
			return r, perr
		}
		return r, opaque{ErrAdapter, err}
	}
	r.Validation = Validation{Validator: p.Validator, Observer: p.Observer, Checks: Match(r.Inputs, eff)}
	r.Outcome = OutcomeReady
	for _, c := range r.Validation.Checks {
		if c.Result != ResultMatch {
			r.Outcome = OutcomeNotReady
		}
	}
	if err := p.put(ctx, key, r); err != nil {
		return r, err
	}
	if r.Outcome != OutcomeReady {
		return r, ErrNotReady
	}
	return r, nil
}

// put is the one way the port stores a receipt: only one that survives the
// same strict decode a caller admits it through (ErrInvalid otherwise, and
// nothing is stored), and a store failure is ErrAdapter.
func (p *Port) put(ctx context.Context, key string, r Receipt) error {
	if err := roundTrips(r); err != nil {
		return err
	}
	if err := p.Store.Put(ctx, key, r); err != nil {
		return opaque{ErrAdapter, err}
	}
	return nil
}

// overwrite is the one way the port replaces a workspace's files (a checkout
// or a restoration): only when Git reports none unpublished, whatever the
// store records of the workspace. Present, unknown or unreadable holds.
func (p *Port) overwrite(ctx context.Context, ws string, effect func() error) error {
	if pres, err := p.Git.Unpublished(ctx, ws); err != nil || pres != PresenceNone {
		return &Hold{HoldUnpublished}
	}
	if err := effect(); err != nil {
		return opaque{ErrAdapter, err}
	}
	return nil
}

// roundTrips validates r by encoding it and decoding it strictly, so every
// receipt the port stores, and every receipt Admit accepts, is one Decode
// accepts.
func roundTrips(r Receipt) error {
	b, err := json.Marshal(r)
	if err != nil {
		return invalid("receipt", "does not encode")
	}
	_, err = Decode(b)
	return err
}

// sameRestore: a recorded restoration is the one req asks for.
func sameRestore(done *Restored, req Restore) bool {
	return done != nil && done.Source == req.Source && done.Preserved == req.Needs
}

// Match is the one validator: it compares each mandatory input of req with
// its effective value.
func Match(req Request, eff Effective) []Check {
	var out []Check
	check := func(in string, ok bool) {
		res := ResultMismatch
		if ok {
			res = ResultMatch
		}
		out = append(out, Check{Input: in, Result: res})
	}
	e, have := req.Environment, eff.Environment
	check("repositories", sameRepos(req.Repositories, eff.Repositories))
	check("recipe", e.Recipe == have.Recipe)
	check("tools", sameComponents(e.Tools, have.Tools))
	check("skills", sameComponents(e.Skills, have.Skills))
	check("harness_bundle", e.HarnessBundle == have.HarnessBundle)
	check("writable", sameSet(req.Writable, eff.Writable))
	check("policy", req.Policy == eff.Policy)
	if e.Image != "" {
		check("image", e.Image == have.Image)
	}
	return out
}

func sameRepos(a, b []Repository) bool {
	return sameSet(reposKeys(a), reposKeys(b))
}

func reposKeys(rs []Repository) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Name+"@"+r.Revision)
	}
	return out
}

func sameComponents(a, b []Component) bool {
	key := func(cs []Component) []string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Name+"@"+c.Version)
		}
		return out
	}
	return sameSet(key(a), key(b))
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
