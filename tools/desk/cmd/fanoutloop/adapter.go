package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

// FanoutLoop is the worker-desk (batch-fanout) SECOND consumer of the deterministic drain engine
// (tools/desk/internal/loopengine), after verify-desk. Its entire purpose is CONTRACT VALIDATION:
// a contract with one consumer is an implementation detail, and batch-fanout is the hardest-fitting
// second consumer (arch doc §3 archetype A) —
//
//   - Land is a NEAR-NO-OP: the worker's draft PR is the durable artifact, so Land neither writes
//     Evidence nor flips a board cell (verify-desk's Land does both). It only records the handle in
//     the dispatch log and releases the dispatch claim once branch-as-claim has taken over.
//   - The pool is a STANDING N=8 with orphan-resume PRIORITY over fresh dispatch, where
//     verify-desk sizes its pool per queue and has no resume lane.
//   - TierPolicy is effort × exec-tier (TierCheap / TierSession), where verify-desk's is always
//     TierLocal or a human route. gate:human briefs DISPATCH NORMALLY here — the gate binds
//     approval, not implementation — so there is no TierHuman lane at all.
//
// If §4's frozen Loop interface survives this consumer UNCHANGED, the contract is real. It does:
// FanoutLoop implements loopengine.Loop with the six hooks and NOTHING added to the engine — the
// heterogeneity that did not fit a hook (out-of-repo serialization) is carried on the engine's
// existing WorkEvidence config seam, not a new hook. Verify row 4 (`git diff … internal/loopengine`
// empty) is that claim, mechanically checked.
//
// This is the REFERENCE / interim-mode build (arch doc §9.1): a Go conductor cannot call the
// harness Agent tool, so Dispatch EMITS the exact dispatch instruction and a Feeder feeds the
// structured Result back. The autonomous cutover — the standing fanout window actually booting this
// as its driver — is gate:human (BLOCKED-ON-IAN); the SKILL.md repoint is staged, not applied.
type FanoutLoop struct {
	Root      string // repo root the default board read scans (STATUS.md + brief files)
	TargetSHA string // merged-main SHA stamped onto every dispatched Item
	// RunnerID is intentionally EMPTY for batch-fanout: authorship differs per worker (each draft
	// PR is authored by the worker App, not this dispatcher), so the engine's author!=runner guard
	// is disabled and attribution is the adapter's own — exactly the case Config.RunnerID's doc
	// names. Set it only if a single-identity dispatch model is ever adopted.
	RunnerID string

	// Board is the Next-up source (statusgen-applied order). nil uses readNextUp(Root).
	Board func() ([]BoardRow, error)
	// Orphans is the orphan-PR resume source (open PRs owing worker action >4h, no live claim). nil
	// means NONE: the OFFLINE reference build issues no `gh` sweep, so the live orphan lane is wired
	// only at cutover. Tests inject fixtures here.
	Orphans func() ([]OrphanPR, error)
	// Rework is the `Awaiting implementer rework` board-section source (§Sources of work row 5) —
	// STATUS.md rows statusgen already flagged as needing implementer action on FEEDBACK, not a
	// fresh brief. nil uses readAwaitingRework(Root), the SAME origin/main-ref read the Next-up
	// board uses (statusMDContent). Tests inject fixtures here.
	Rework func() ([]BoardRow, error)
	// InFlight is the in-flight-claim source for the ADVISORY write-scope overlap warning:
	// the items already claimed for this root, carried as their derived
	// write-scopes. nil reads the root repo's local `refs/heads/dispatch/*` claims (offline). Tests
	// inject fixtures here. It is ADVISORY only — nothing dispatches or blocks on it.
	InFlight func() ([]loopengine.Item, error)

	// Represented is the ALREADY-REPRESENTED exclusion source: the set of brief IDs
	// (`<stream>/<NN>`, lower-cased) that already have an OPEN or MERGED PR, so a phantom fresh row
	// whose work is already in flight or merged is never offered for dispatch. nil = NONE: the
	// OFFLINE reference build issues no `gh` PR sweep (exactly like Orphans), so the exclusion is
	// wired only at the live cutover, where the source builds the set with
	// deskkit.RepresentedBriefSet over the repo's open+merged PRs — keyed on each PR's `Brief:`
	// trailer, NEVER a branch name (a branch spelled differently from the derived pattern is exactly
	// the phantom this exclusion exists to catch). Tests inject fixtures here. It applies ONLY to
	// fresh Next-up rows: orphan-resume and Awaiting-rework items act on an existing PR by design and
	// are never excluded by it.
	Represented func() (map[string]bool, error)

	// Emit is where interim-mode dispatch instructions are printed. nil = stdout.
	Emit io.Writer
	// Feeder obtains the structured Result for a dispatched item (interim mode: the operator feeds
	// the worker's draft-PR handle back). nil means no result path is wired — the autonomous cutover
	// is BLOCKED-ON-HUMAN, so Dispatch refuses rather than pretend to run.
	Feeder func(loopengine.Item, loopengine.Tier, string) (loopengine.Result, error)
	// DispatchSink makes a landed dispatch durable (record the handle + release the dispatch claim).
	// nil builds the real releasing sink from the forge resolver (see sink()). Tests inject here.
	DispatchSink Sink
	// DryRun selects the printing sink instead of the releasing one: no network write is made,
	// and every release is emitted as the line it WOULD have performed. It is an explicit
	// choice, never a fallback — see sink().
	DryRun bool
	// ResolveForge overrides how the real sink obtains the Forge for an item's target repo.
	// nil is production: deskkit.ForgeFor under the session's own App role. Tests inject here
	// so the release path is exercised without a live remote.
	ResolveForge forgeResolver

	mu sync.Mutex
	// inFlight / handled model the board's own claim-filtering for the reference build: a brief with
	// a live claim or an open PR is hidden from statusgen's Next-up, so once dispatched (in flight) or
	// handed off (handled), a static fixture board must stop re-offering it. The engine skips
	// inFlight itself; SelectQueue additionally drops handled so the queue drains.
	inFlight map[string]bool
	handled  map[string]bool
	// outOfRepo is the single-slot ledger for out-of-repo serialization: at most one brief declaring `out-of-repo
	// files:` may be in flight across ALL streams. Dispatch marks it; Land clears it; the engine's
	// WorkEvidence probe (workEvidence) refuses a second one at claim time.
	outOfRepo map[string]bool
}

func (f *FanoutLoop) Name() string { return "worker-desk" }

// SelectQueue is the deterministic board read. A fresh row addressed to THIS desk
// (`to:worker`) is a DIRECTED message and LEADS the whole queue;
// after the addressed lead it returns ORPHAN RESUMES, then
// AWAITING-IMPLEMENTER-REWORK rows (both outrank ordinary fresh dispatch — worker-desk SKILL.md §Sources
// of work rows 3 and 5: "resuming started work outranks a fresh brief"), then the Next-up rows
// in board order, each already priority/staleness/cap/dep-filtered by statusgen — so every fresh
// row it sees is already `todo` and unclaimed. It INCLUDES `issue-<NN>` placeholder rows: those
// ARE this loop's work (worker-desk dispatch spec, Procedure 2 — "INCLUDE issue-placeholders —
// `issue-<NN>` rows ARE yours to dispatch"). It drops only a DIFFERENT loop's dispatch token (a
// `review-request` token belongs to the review loop, not here) and anything already handed off.
// It adds NO scoring pass of its own — the order it returns is the order the boards agreed on.
func (f *FanoutLoop) SelectQueue() ([]loopengine.Item, error) {
	var items []loopengine.Item
	// addressed holds fresh rows carrying this desk's inbox label (`to:worker`). They are
	// a DIRECTED message to this desk, so they LEAD the whole queue — ahead of orphan
	// resumes and rework — by construction (`fanoutloop plan` emits
	// `to:<my role>` items first). Only fresh board rows can be addressed; orphan/rework
	// items act on an existing PR and carry no board labels.
	var addressed []loopengine.Item
	inbox := f.inboxRole()

	// 1. Orphan resumes — highest priority (drain started work before starting new).
	orphans, err := f.orphanSource()
	if err != nil {
		return nil, err
	}
	for _, o := range orphans {
		if f.isHandled(o.ID) {
			continue
		}
		items = append(items, o.toItem())
	}

	// 2. Awaiting-implementer-rework rows — second priority, ahead of fresh dispatch.
	rework, err := f.reworkSource()
	if err != nil {
		return nil, err
	}
	for _, r := range rework {
		if f.isHandled(r.ID()) {
			continue
		}
		items = append(items, r.toReworkItem(f.TargetSHA))
	}

	// 3. Fresh Next-up rows, board order preserved.
	rows, err := f.boardSource()
	if err != nil {
		return nil, err
	}
	// The already-represented exclusion set (brief IDs with an OPEN or MERGED PR). Applied to FRESH
	// rows ONLY — orphan-resume and rework items above act on an existing PR by design.
	represented, err := f.representedSource()
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if isForeignDispatchToken(r) {
			// A DIFFERENT loop's consumer (e.g. a `review-request` token owned by the review loop);
			// skipped so the two consumers never double-dispatch. NOTE: `issue-<NN>` placeholders are
			// NOT dropped here — they ARE this loop's work (Procedure 2) and flow through below.
			continue
		}
		if represented[strings.ToLower(r.Stream+"/"+r.Num)] {
			// A fresh row whose brief already has an OPEN or MERGED PR — a phantom the board has not
			// yet caught up with (the READMEs still read `todo`/`implemented` while the forge moved
			// on). Excluded so a worker is never spent re-deriving that the PR already exists. The
			// match is on the brief id (keyed against the PR's `Brief:` trailer at the source), never
			// on a branch name.
			continue
		}
		if f.isHandled(r.ID()) {
			continue
		}
		it := r.toItem(f.TargetSHA)
		if inbox != "" && addressedToRole(r, inbox) {
			// A directed message to THIS desk. Stamp the addressee (so the plan output and
			// any downstream reader can see the `to:` kind) and route it to the leading lane.
			it.Payload["to"] = inbox
			addressed = append(addressed, it)
			continue
		}
		items = append(items, it)
	}
	// Addressed items lead the whole queue; board order is preserved within each lane.
	return append(addressed, items...), nil
}

// inboxRole is the desk role whose `to:<role>` inbox this loop leads with — the App role
// bound to the worker-desk loop (deskkit.TokenRoleForLoop), i.e. `worker`. An unbound loop
// name yields "" and no addressed lane, rather than a guessed role.
func (f *FanoutLoop) inboxRole() string {
	role, ok := deskkit.TokenRoleForLoop(f.Name())
	if !ok {
		return ""
	}
	return role
}

// addressedToRole reports whether a board row carries the `to:<role>` desk-inbox label
// naming exactly this role (deskkit.AddressedToOf's Stamped state — a conflicting pair is
// malformed and is NOT treated as addressed). Rows from a label-less board source (the
// default STATUS.md reader) carry no labels and are never addressed, exactly as
// isForeignDispatchToken degrades.
func addressedToRole(r BoardRow, role string) bool {
	addr, state := deskkit.AddressedToOf(r.Labels)
	return state == deskkit.RaisedByStamped && strings.EqualFold(addr, role)
}

// TierPolicy is in tier.go.

// Dispatch renders the batch worker prompt (essentials verbatim from the skill), emits the exact
// interim-mode instruction, and returns a Handle fed by Feeder. It marks the out-of-repo ledger the
// instant it dispatches — BEFORE returning — so the next item's WorkEvidence probe, in the SAME
// fillPool pass, sees the single slot held.
func (f *FanoutLoop) Dispatch(it loopengine.Item, tier loopengine.Tier) (loopengine.Handle, error) {
	prompt := renderDispatchPrompt(it, tier)
	if err := assertNoSharedCheckout(prompt); err != nil {
		return nil, err
	}
	fmt.Fprintf(f.emit(), "\n=== DISPATCH %s (tier=%s) ===\n%s\n=== END DISPATCH ===\n", it.ID, tier, prompt)

	if f.Feeder == nil {
		return nil, fmt.Errorf(
			"interim dispatch for %s: no Agent binding and no result feeder wired — the autonomous cutover is BLOCKED-ON-HUMAN (arch doc §9.1)", it.ID)
	}

	f.mu.Lock()
	if f.inFlight == nil {
		f.inFlight = map[string]bool{}
	}
	f.inFlight[it.ID] = true
	if isOutOfRepo(it) {
		if f.outOfRepo == nil {
			f.outOfRepo = map[string]bool{}
		}
		f.outOfRepo[it.ID] = true
	}
	f.mu.Unlock()

	done := make(chan loopengine.Result, 1)
	go func() {
		r, err := f.Feeder(it, tier, prompt)
		if err != nil {
			r = loopengine.Result{Item: it, Verdict: loopengine.VerdictBlocked, RunnerID: f.RunnerID}
		}
		if r.Item.ID == "" {
			r.Item = it
		}
		done <- r
	}()
	return &handle{item: it, done: done}, nil
}

// Land is a NEAR-NO-OP by design (the whole "hardest-fitting consumer" point). The worker's draft
// PR is the durable artifact, so there is no Evidence write and no status flip. Land records the
// handle in the dispatch log and releases the dispatch claim now that branch-as-claim (the worker's
// first push) has taken over, and frees the out-of-repo single slot if this brief held it.
func (f *FanoutLoop) Land(r loopengine.Result) error {
	f.mu.Lock()
	delete(f.inFlight, r.Item.ID)
	delete(f.outOfRepo, r.Item.ID)
	if f.handled == nil {
		f.handled = map[string]bool{}
	}
	f.handled[r.Item.ID] = true
	f.mu.Unlock()

	s, serr := f.sink()
	if serr != nil {
		return serr
	}
	if err := s.RecordDispatch(r); err != nil {
		return err
	}
	return s.ReleaseDispatchClaim(r.Item)
}

// OnIdle refreshes for the next poll. In the reference build it re-scans nothing beyond the next
// SelectQueue (the per-cycle orphan + board sweep IS SelectQueue's two sources); the live wiring
// re-fetches origin/main and regenerates the boards here. The engine calls OnIdle only when the
// pool is empty, and the orphan sweep is part of every SelectQueue anyway, so orphan-resume
// priority holds on both the idle path and the per-cycle refill path (brief facts).
func (f *FanoutLoop) OnIdle() error { return nil }

// workEvidence is the engine's WorkEvidence probe (Config.WorkEvidence, consulted inside Claim
// BEFORE the flock). It is the contract-preserving home for the out-of-repo
// serialization: a brief declaring `out-of-repo files:` is "taken" whenever a DIFFERENT such brief
// is already in flight, so Claim returns (false, nil) and the engine skips it — no new Loop hook,
// no engine change. It is a typed read of the in-flight ledger, replacing the SKILL's prose rule.
func (f *FanoutLoop) workEvidence(it loopengine.Item) (taken bool, why string, err error) {
	if !isOutOfRepo(it) {
		return false, "", nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for id := range f.outOfRepo {
		if id != it.ID {
			return true, fmt.Sprintf(
				"out-of-repo serialization: %s holds the single out-of-repo slot in flight — at most one brief declaring `out-of-repo files:` dispatches at a time across ALL streams; retry once it lands",
				id), nil
		}
	}
	return false, "", nil
}

func (f *FanoutLoop) isHandled(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.handled[id]
}

func (f *FanoutLoop) boardSource() ([]BoardRow, error) {
	if f.Board != nil {
		return f.Board()
	}
	return readNextUp(f.Root, f.TargetSHA)
}

func (f *FanoutLoop) orphanSource() ([]OrphanPR, error) {
	if f.Orphans != nil {
		return f.Orphans()
	}
	return nil, nil // OFFLINE reference build: no `gh` orphan sweep; wired at cutover
}

func (f *FanoutLoop) reworkSource() ([]BoardRow, error) {
	if f.Rework != nil {
		return f.Rework()
	}
	return readAwaitingRework(f.Root)
}

// representedSource returns the already-represented brief-id set (open/merged PRs). nil Represented
// means NONE — the OFFLINE reference build reconciles against no PRs, so every fresh board row is
// offered exactly as before; the exclusion activates only when the live cutover wires a source.
func (f *FanoutLoop) representedSource() (map[string]bool, error) {
	if f.Represented != nil {
		return f.Represented()
	}
	return nil, nil
}

func (f *FanoutLoop) emit() io.Writer {
	if f.Emit != nil {
		return f.Emit
	}
	return os.Stdout
}

// sink returns the Sink a landing acts through.
//
// The DEFAULT is the real, claim-releasing sink, constructed from the forge resolver — this is
// where the file-level "wired only at cutover, on the owner's call" deferral is discharged. It
// can no longer yield a sink holding a nil forge: the resolver is obtained here and refused at
// construction if absent (newForgeDispatchSink), and the forge for the item's target repo is
// resolved through deskkit.ForgeFor at release time.
//
// DryRun selects the printing sink INSTEAD, explicitly. That is the shape the safety property
// needs: a driver that must not touch the network says so, rather than a misconfigured
// deployment silently falling back to a sink that leaks a claim per landing.
func (f *FanoutLoop) sink() (Sink, error) {
	if f.DispatchSink != nil {
		return f.DispatchSink, nil
	}
	if f.DryRun {
		return dryRunSink{out: f.emit()}, nil
	}
	return newForgeDispatchSink(f.emit(), f.forgeResolver())
}

// forgeResolver is the resolver the real sink is built from: production's deskkit.ForgeFor
// wiring unless a test injected one.
func (f *FanoutLoop) forgeResolver() forgeResolver {
	if f.ResolveForge != nil {
		return f.ResolveForge
	}
	return productionForgeResolver
}

// handle is the interim in-flight tracker: Done() fires when Feeder returns the structured Result.
// Under the native-primitive upgrade it would wrap a real child-worker completion; the engine
// treats both identically.
type handle struct {
	item loopengine.Item
	done chan loopengine.Result
}

func (h *handle) Done() <-chan loopengine.Result { return h.done }
func (h *handle) Item() loopengine.Item          { return h.item }

// compile-time assertion: FanoutLoop is a valid drain-engine consumer — the whole point of the
// brief. If the frozen Loop interface ever needed a new hook to fit batch-fanout, THIS line would
// stop compiling, which is the contract-erosion tripwire.
var _ loopengine.Loop = (*FanoutLoop)(nil)
