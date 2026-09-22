package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

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
	// RepairObligations is the durable REPAIR-OBLIGATION source (example-stream/17): the
	// reconciled, CURRENT obligation per immutable key across the configured roots' append-only
	// repair-obligations.jsonl sidecars. It shares the rework lane (kind=kindRework) so a repair
	// obligation inherits the rework reservation floor and orphan-behind priority. nil uses the
	// default cross-root reader (defaultRepairObligations). Tests inject fixtures here. Only the
	// ASSIGNABLE obligations (actionable, unresolved, awaiting-assignment or dead-lease) become
	// items — a live repairing claim, a resolved obligation and a waiting-external obligation are
	// withheld by deskkit's Assignable rule, never dispatched.
	RepairObligations func() ([]deskkit.RepairObligation, error)
	// Now is the clock the obligation lease horizon is evaluated against. nil uses time.Now. Tests
	// inject a fixed clock so a dead-lease reassignment is deterministic.
	Now func() time.Time
	// InFlight is the in-flight-claim source for the ADVISORY write-scope overlap warning:
	// the items already claimed for this root, carried as their derived
	// write-scopes. nil reads the root repo's local `refs/heads/dispatch/*` claims (offline). Tests
	// inject fixtures here. It is ADVISORY only — nothing dispatches or blocks on it.
	InFlight func() ([]loopengine.Item, error)

	// Represented is the ALREADY-REPRESENTED reconciliation source: brief IDs (`<stream>/<NN>`,
	// lower-cased) that already have an OPEN or MERGED PR, each carried as its PR number + merged flag
	// (deskkit.RepresentedPR) so the fresh lane can ROUTE by state rather than merely drop. nil =
	// NONE: the OFFLINE reference build reconciles against no PRs, so every fresh row is offered
	// exactly as before; the reconciliation activates only when a source is wired (cmdPlan wires the
	// live one; tests inject fixtures). The source builds the map with deskkit.RepresentedBriefPRs
	// over the repo's open+merged PRs — keyed on each PR's `Brief:` trailer, NEVER a branch name (a
	// branch spelled differently from the derived pattern is exactly the phantom this reconciliation
	// exists to catch).
	//
	// It governs FRESH Next-up rows AND `### Awaiting implementer rework` STATUS.md rows
	// (#1028/at#2026: a rework row whose deliverable already merged is a phantom this lane used to
	// skip). In the FRESH lane the three outcomes are distinct (#1339): a MERGED PR → the row is
	// landed-unreconciled and recorded for the plan's own heading, NEVER dispatched; an OPEN PR → the
	// row is routed to the RESUME lane (started work outranks fresh); no PR → fresh dispatch. In the
	// rework lane a represented row (either state) is dropped, as before. Two lanes stay EXEMPT because
	// a representing PR is expected, not a phantom: ORPHAN resumes (the open PR they act on) and
	// DURABLE repair obligations (which rework a MERGED original via a fresh follow-up branch, §row 5b).
	//
	// A source ERROR is could-not-check, NEVER rounded to "no PR exists": the fresh and rework lanes
	// (the two that consult it) are HELD, so a phantom can never slip through on a failed read, while
	// the exempt lanes still flow. SelectQueue records the hold reason in freshHeld for the plan output.
	Represented func() (map[string]deskkit.RepresentedPR, error)

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

	// landedUnreconciled and freshHeld are PLAN DIAGNOSTICS the fresh-lane reconciliation writes and
	// renderPlan reads (#1339). They are not queue state: the engine ignores them. landedUnreconciled
	// lists every fresh row whose brief already MERGED — printed under the plan's own heading with the
	// PR number, never dispatched. freshHeld, when non-empty, is the could-not-check reason the fresh
	// (and rework) lanes were HELD for this run. Both are reset at the top of each SelectQueue call,
	// under mu, so a re-poll never reports a stale run's diagnostics.
	landedUnreconciled []landedRow
	freshHeld          string
}

// landedRow is one fresh Next-up row whose brief already has a MERGED PR: its board cell simply never
// reconciled after the merge (#1339). It is a plan DIAGNOSTIC, never a dispatch item.
type landedRow struct {
	briefID string // `<stream>/<NN>`
	pr      int    // the merged PR's number
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
	// Reset the plan diagnostics up front so a re-poll never reports a stale run's landed/held state.
	f.mu.Lock()
	f.landedUnreconciled = nil
	f.freshHeld = ""
	f.mu.Unlock()

	// Each lane is assembled into its own slice so the final concatenation states the priority order
	// explicitly: addressed (a directed `to:worker` message) leads, then RESUME (orphan resumes plus
	// fresh rows found to have an OPEN PR — started work outranks fresh), then rework, then repair,
	// then fresh dispatch in board order.
	var addressed, orphanItems, openResumeItems, reworkItems, repairItems, freshItems []loopengine.Item
	inbox := f.inboxRole()

	// The already-represented reconciliation (brief IDs with an OPEN or MERGED PR, each with its PR
	// number + merged flag, keyed on each PR's `Brief:` trailer — NEVER a branch name, so a branch
	// spelled `feat/<repo>--<stream>--<NN>` instead of the derived `feat/<stream>-<NN>` cannot hide a
	// phantom, at#2026). nil source = the OFFLINE reference default (no PR read) — the reconciliation
	// is inert and every row is offered exactly as before.
	//
	// A source ERROR is could-not-check, NOT no-PR-exists (#1339): the fresh and rework lanes — the
	// two that consult the map — are HELD (a phantom must never slip through a failed read), the reason
	// is recorded for the plan output, and the exempt lanes (orphan resumes, repair obligations) still
	// flow. It is NOT returned as an error, so the whole plan does not fail on it.
	represented, repErr := f.representedSource()
	held := repErr != nil
	if held {
		f.mu.Lock()
		f.freshHeld = fmt.Sprintf("could not read the deliverable repo's open+merged PRs (%v) — "+
			"the fresh and rework lanes are HELD this run; could-not-check is not no-PR-exists, so a "+
			"phantom row is never offered on an unread forge", repErr)
		f.mu.Unlock()
	}

	// 1. Orphan resumes — highest priority (drain started work before starting new). NEVER subject to
	// the represented reconciliation: an orphan resume IS an open PR by construction, so the PR that
	// represents it is exactly the one it acts on. Their PR numbers are recorded so a fresh row later
	// found to have that same OPEN PR is not surfaced twice (once as an orphan, once as a resume).
	orphans, err := f.orphanSource()
	if err != nil {
		return nil, err
	}
	orphanPRs := map[int]bool{}
	for _, o := range orphans {
		orphanPRs[o.Number] = true
		if f.isHandled(o.ID) {
			continue
		}
		orphanItems = append(orphanItems, o.toItem())
	}

	// 2. Awaiting-implementer-rework STATUS.md rows — ahead of fresh dispatch. These ARE subject to the
	// reconciliation (#1028/at#2026): a `### Awaiting implementer rework` row whose brief already has an
	// OPEN or MERGED PR is a phantom this lane used to skip. Held on could-not-check. The DURABLE repair
	// obligations (step 2b) stay EXEMPT: a repair obligation legitimately reworks a MERGED original via a
	// fresh follow-up branch (§Sources of work row 5b), so a representing PR is expected there.
	rework, err := f.reworkSource()
	if err != nil {
		return nil, err
	}
	if !held {
		for _, r := range rework {
			if f.isHandled(r.ID()) {
				continue
			}
			if _, ok := represented[strings.ToLower(r.Stream+"/"+r.Num)]; ok {
				continue
			}
			reworkItems = append(reworkItems, r.toReworkItem(f.TargetSHA))
		}
	}

	// 2b. Durable REPAIR OBLIGATIONS (example-stream/17) — the OTHER rework source, EXEMPT from the
	// reconciliation and from the could-not-check hold (a repair obligation reworks a MERGED original by
	// design). Same priority as the rework rows above: resuming owed repair outranks a fresh brief. Each
	// item's ID is the IMMUTABLE obligation key, so a replacement worker resumes the SAME obligation.
	repairs, err := f.repairSource()
	if err != nil {
		return nil, err
	}
	for _, it := range repairs {
		if f.isHandled(it.ID) {
			continue
		}
		repairItems = append(repairItems, it)
	}

	// 3. Fresh Next-up rows, board order preserved. On could-not-check the whole fresh lane is HELD.
	// Otherwise each row is ROUTED by its representing PR's state (#1339): a MERGED PR → the row is
	// landed-unreconciled (its board cell just never flipped after the merge) and is recorded for the
	// plan's own heading, never dispatched; an OPEN PR → the row is started work, routed to the RESUME
	// lane (deduped against an orphan already covering that PR); no PR → fresh dispatch. The match is on
	// the brief id keyed against the PR's `Brief:` trailer, never a branch name.
	rows, err := f.boardSource()
	if err != nil {
		return nil, err
	}
	if !held {
		for _, r := range rows {
			if isForeignDispatchToken(r) {
				// A DIFFERENT loop's consumer (e.g. a `review-request` token owned by the review loop);
				// skipped so the two consumers never double-dispatch. NOTE: `issue-<NN>` placeholders are
				// NOT dropped here — they ARE this loop's work (Procedure 2) and flow through below.
				continue
			}
			if f.isHandled(r.ID()) {
				continue
			}
			if rp, ok := represented[strings.ToLower(r.Stream+"/"+r.Num)]; ok {
				if rp.Merged {
					// Landed-unreconciled: the deliverable merged, the board row just never reconciled.
					// A plan diagnostic, never a dispatch — recorded for renderPlan's own heading.
					f.mu.Lock()
					f.landedUnreconciled = append(f.landedUnreconciled, landedRow{briefID: r.Stream + "/" + r.Num, pr: rp.Number})
					f.mu.Unlock()
					continue
				}
				// An OPEN PR: started work. Route to the resume lane rather than fresh dispatch, unless
				// an orphan already covers that PR (dedupe — do not surface the same PR twice).
				if !orphanPRs[rp.Number] && !f.isHandled(openResumeID(rp.Number)) {
					openResumeItems = append(openResumeItems, openResumeItem(r, rp.Number))
				}
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
			freshItems = append(freshItems, it)
		}
	}

	// Concatenate in priority order: addressed, resume (orphans then open-PR resumes), rework, repair,
	// fresh. Board order is preserved within each lane.
	out := addressed
	out = append(out, orphanItems...)
	out = append(out, openResumeItems...)
	out = append(out, reworkItems...)
	out = append(out, repairItems...)
	out = append(out, freshItems...)
	return out, nil
}

// landedUnreconciledRows returns the fresh rows the last SelectQueue found already MERGED (a plan
// diagnostic, never dispatched). renderPlan reads it after SelectQueue; it is copied under mu so the
// caller never races a concurrent re-poll.
func (f *FanoutLoop) landedUnreconciledRows() []landedRow {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]landedRow(nil), f.landedUnreconciled...)
}

// freshHoldReason returns the reason the last SelectQueue HELD the fresh lane (a could-not-check
// represented read), or "" if it did not. renderPlan reads it after SelectQueue.
func (f *FanoutLoop) freshHoldReason() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.freshHeld
}

// openResumeID is the resume claim key for a fresh row found to have an OPEN PR — the same
// `resume:pr-<N>` shape the orphan-resume source uses, so the two lanes share one id namespace and a
// PR covered by both dedupes to one item.
func openResumeID(pr int) string { return fmt.Sprintf("resume:pr-%d", pr) }

// openResumeItem turns a fresh Next-up row whose brief already has an OPEN PR into a RESUME item
// (kindOrphan) — reusing the existing resume lane rather than inventing a second. The branch is left
// unresolved (the reconciliation reads number+state, not the head branch); the resuming worker
// checks the PR out by number. The findings line records WHY this is a resume, not fresh dispatch.
func openResumeItem(r BoardRow, pr int) loopengine.Item {
	o := OrphanPR{
		Repo:   r.Repo,
		Number: pr,
		ID:     openResumeID(pr),
		Findings: fmt.Sprintf(
			"brief %s/%s already has an OPEN PR (#%d) — resume it rather than fresh-dispatch (the board row lagged the forge, #1339)",
			r.Stream, r.Num, pr),
	}
	return o.toItem()
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

// repairSource returns the ASSIGNABLE repair obligations as rework items, evaluated against this
// loop's clock and the repair lease horizon. It reads the reconciled obligations from
// RepairObligations (default: the cross-root sidecar reader) and maps each assignable one to a
// rework item. The roots are needed both to read the sidecars AND to resolve each obligation's brief
// frontmatter for tiering, so the default reader and the item mapper share one roots list.
func (f *FanoutLoop) repairSource() ([]loopengine.Item, error) {
	roots := f.repairRoots()
	var obligs []deskkit.RepairObligation
	var err error
	if f.RepairObligations != nil {
		obligs, err = f.RepairObligations()
	} else {
		obligs, err = collectRepairObligations(roots)
	}
	if err != nil {
		return nil, err
	}
	return assignableRepairItems(obligs, roots, f.TargetSHA, f.clock(), repairLeaseTTL), nil
}

// repairRoots is the roots list the default repair reader and the brief-frontmatter resolver use.
// It prefers the multi-repo configured roots (example-stream/17: "across configured roots"); a
// malformed roots configuration degrades to this loop's single reference Root rather than wedging
// the whole plan — the cross-root read is an enhancement over the single-root reference build, not a
// new hard precondition on it.
func (f *FanoutLoop) repairRoots() []deskkit.RootConfig {
	if roots, err := deskkit.ConfiguredRoots(); err == nil && len(roots) > 0 {
		return roots
	}
	return []deskkit.RootConfig{{Path: f.Root}}
}

// clock returns this loop's time source (default time.Now), used to evaluate a repair obligation's
// lease horizon deterministically under test.
func (f *FanoutLoop) clock() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

// representedSource returns the already-represented brief map (open/merged PRs, each with its PR
// number + merged flag). nil Represented means NONE — the OFFLINE reference default reconciles against
// no PRs, so every fresh board row is offered exactly as before; the reconciliation activates only
// when a source is wired (cmdPlan wires the live one; tests inject fixtures).
func (f *FanoutLoop) representedSource() (map[string]deskkit.RepresentedPR, error) {
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
