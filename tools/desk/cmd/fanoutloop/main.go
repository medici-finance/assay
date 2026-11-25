// Command fanoutloop is the worker-desk (batch-fanout) REFERENCE CONSUMER of the deterministic
// drain engine (tools/desk/internal/loopengine) — the SECOND consumer after verify-desk, whose
// entire job is to VALIDATE that the engine contract generalizes to a
// consumer that fits it hardest (Land a near-no-op, a standing N=8 pool with orphan-resume priority,
// tiering by effort × exec-tier). It exists to prove the contract end-to-end and to serve as the
// interim-mode debug surface — NOT (yet) to drive the standing fanout window. The autonomous cutover
// (the worker desk actually booting this as its driver) is gate:human — BLOCKED-ON-IAN.
//
// Subcommands:
//
//	fanoutloop plan --root <repo> [--sha <targetSHA>] [--repo <owner/name>]
//	    Deterministic scheduler OUTPUT: read the Next-up board (orphan resumes first, then rows in
//	    board order — issue-<NN> placeholders INCLUDED, only a different loop's `review-request`
//	    dispatch tokens skipped), reconcile each fresh row against the repo's open+merged PRs, compute
//	    each item's tier, and print the EXACT dispatch instruction the operator would execute as an
//	    Agent call. No agents are spawned and nothing is written. The already-represented reconciliation
//	    (#1339) routes a fresh row whose brief already has an OPEN PR to RESUME and one already MERGED to
//	    the landed-unreconciled listing, never fresh dispatch; a could-not-check read HOLDS the fresh
//	    lane rather than offering rows on an unverified forge. That reconciliation's PR-list transport is
//	    DEFERRED like the orphan sweep — the closed forge surface ships no forge-CLI call and the typed
//	    open+merged-changes op is the cutover work — so the shipped offline build performs no forge read
//	    and offers rows as before until the transport is wired. This is the "the Go engine owns all
//	    scheduler state and emits exact dispatches" surface (§9.1).
//
//	fanoutloop --version
//
// The `run` (autonomous drive) path is deliberately NOT exposed here: a Go conductor cannot call the
// harness Agent tool, and wiring the live window is the human-gated cutover.
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func main() {
	os.Exit(deskkit.PerfRun("fanoutloop", nil, os.Args[1:], func([]string) int {
		// Declare the roster class EXPLICITLY (never ClassWrite-by-omission): this tool ACTS on the
		// dispatch queue, so ciEligible=false — config-home file only, never the environment.
		deskkit.SetToolClass(deskkit.ClassForTool(false))
		// Echo the effective trust/authority configuration to stderr before doing anything (P3): a
		// control surface that lives in settings rather than a diff is visible only at RUN time.
		deskkit.EchoEffectiveConfig(os.Stderr)
		if !deskkit.CheckVerbActivation(os.Stderr) {
			return deskkit.ExitUnverifiable
		}
		// Wire `plan`'s already-represented PR-list transport LIVE for the shipped binary
		// (represented_live.go). Assigned here, in main(), not at package init: tests call
		// cmdPlan/run directly and keep the nil default (plan offers rows as before unless a test
		// wires its own recorded transport), while the real binary reconciles against the repo's
		// open+merged changes read through the typed Forge seam.
		representedPRs = liveRepresentedPRs
		// The per-brief lookup past an incomplete window (#5853), wired beside it.
		briefChangeLookup = liveBriefChangeLookup
		// The briefs-authoring exemption's file transport (#1339), wired beside it for the same reason.
		representedPRFiles = liveRepresentedPRFiles
		representedPRBatchFiles = liveRepresentedPRBatchFiles
		return run(os.Args[1:])
	}))
}

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("fanoutloop sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// Guard (kill switch + stop flags) is the FIRST action, before any board read.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	switch args[0] {
	case "plan":
		// cmdPlan RETURNS its refusal (renderPlan wraps it, e.g. "cannot read the Next-up queue")
		// and prints none of it; the exit code alone never says why (#5029). Print it here, once.
		err := cmdPlan(args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		return loopengine.ExitCodeOf(err)
	default:
		fmt.Fprintf(os.Stderr, "fanoutloop: unknown subcommand %q\n\n%s", args[0], usage)
		return deskkit.ExitRefused
	}
}

func cmdPlan(args []string) error {
	// `plan --help` prints the command usage (which documents the advisory WRITE-OVERLAP
	// output) rather than the bare flag defaults, so the surface is discoverable from the
	// subcommand itself, not only the top-level help.
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			fmt.Fprint(os.Stdout, usage)
			return nil
		}
	}
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	root := fs.String("root", ".", "repo root to scan for the Next-up queue")
	sha := fs.String("sha", "", "merged-main target SHA workers branch from")
	repo := fs.String("repo", "", "owner/name to reconcile fresh rows against open+merged PRs (#1339); "+
		"defaults to the repo the configured roots map --root to, else the checkout's origin remote")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("bad flags: " + err.Error())
	}

	// DryRun is set EXPLICITLY here rather than left to a default: `plan` writes nothing, and every
	// release the sink would perform is emitted as the line it WOULD run instead.
	f := &FanoutLoop{Root: *root, TargetSHA: *sha, DryRun: true}

	// Wire the already-represented reconciliation (#1339) when the PR-list transport is live. It reads
	// the resolved repo's open+merged PRs ONCE per run and reduces them in memory, so `plan` routes a
	// row whose brief already has an OPEN PR to resume and one already MERGED to landed-unreconciled,
	// never fresh dispatch. When the repo cannot be resolved, the source is a could-not-check that HOLDS
	// the fresh lane (renderPlan prints why) rather than offering rows on an unverified forge. When the
	// transport is NOT wired (representedPRs nil — the shipped offline build; the live typed forge op is
	// the deferred cutover, see represented.go), Represented is left unset and every row is offered as
	// before.
	if representedPRs != nil {
		if resolved, rerr := resolveRepoForPlan(*root, *repo); rerr == nil {
			index := &deskkit.IssuePRIndex{List: representedPRs, Lookup: briefChangeLookup}
			f.issueIndex, f.issueBoardRepo = index, resolved
			f.Represented = representedSourceFor(resolved, f.candidateBriefIDs, f.declaredPartials, index)
		} else {
			heldErr := rerr
			f.Represented = func() (map[string]deskkit.RepresentedPR, error) { return nil, heldErr }
		}
	}
	return renderPlan(f, os.Stdout)
}

// renderPlan writes the deterministic plan output (queue rows + the advisory WRITE-OVERLAP
// warnings) to out. Split from cmdPlan so a test can drive it with injected Board / InFlight
// sources and capture the output. The advisory-warning contract is asserted here: the queue is
// rendered in full FIRST and no overlap ever gates, reorders, or drops a row.
func renderPlan(f *FanoutLoop, out io.Writer) error {
	items, err := f.SelectQueue()
	if err != nil {
		return deskkit.Unverifiable("cannot read the Next-up queue", err)
	}
	fmt.Fprintf(out, "worker-desk plan: %d item(s) to dispatch (addressed to:worker items first, then orphan resumes, then Next-up in board order)\n", len(items))
	fmt.Fprintln(out, classLine(items))
	// The fresh lane HELD on a could-not-check represented read (#1339): stated up front so a reader
	// never mistakes an empty fresh lane for "nothing to do" when it is actually "could not verify".
	if held := f.freshHoldReason(); held != "" {
		fmt.Fprintf(out, "FRESH LANE HELD: %s\n", held)
	}
	landed, unchecked := splitUnchecked(f.landedUnreconciledRows())
	// The COULD-NOT-CHECK count is stated up front too (#5853): a plan with few items and many
	// unchecked rows must not read as idle at a glance. The rows themselves are listed below.
	if len(unchecked) > 0 {
		fmt.Fprintf(out, "COULD-NOT-CHECK: %d row(s) NOT dispatched — representing PR unknown (listed below)\n", len(unchecked))
	}
	for _, hold := range f.issuePRHoldRows() {
		if _, err := fmt.Fprintf(out, "ISSUE-PR HOLD %s\n", hold); err != nil {
			return err
		}
	}
	for _, it := range items {
		tier, terr := f.TierPolicy(it)
		if terr != nil {
			fmt.Fprintf(out, "\n-- %s: tier error: %v\n", it.ID, terr)
			continue
		}
		prompt := renderDispatchPrompt(it, tier)
		if err := assertNoSharedCheckout(prompt); err != nil {
			return deskkit.Refused(err.Error())
		}
		fmt.Fprintf(out, "\n=== DISPATCH %s (tier=%s) ===\n%s\n", it.ID, tier, prompt)
	}

	// LANDED-UNRECONCILED (#1339): fresh Next-up rows whose brief already has a MERGED PR. Their board
	// cell simply never reconciled after the merge, so they read `todo` while the work is done — a
	// worker dispatched to one returns nothing. They are listed here with the merged PR number, under
	// their own heading, and are DELIBERATELY absent from the DISPATCH list above. The cell is
	// reconciled by `statusgen reconcile --backfill --apply` (a separate lane), never by this planner.
	// A row whose reconcile flip statusgen's lint would HOLD names the missing frontmatter key instead
	// (#5853): suggesting the reconcile there only sends the reader into a red lint.
	if len(landed) > 0 {
		fmt.Fprintf(out, "\n=== LANDED-UNRECONCILED: %d row(s) — merged, board cell not yet reconciled; NOT dispatched (#1339) ===\n", len(landed))
		for _, l := range landed {
			fmt.Fprintf(out, "  %s — merged PR #%d (%s)\n", l.briefID, l.pr, landedNextStep(f.Root, l.briefID))
		}
	}
	// COULD-NOT-CHECK (#5853): rows whose representing PR could not be determined — the PR list was a
	// bounded window and the per-brief `Brief:` lookup failed. Never dispatched, never called landed.
	if len(unchecked) > 0 {
		fmt.Fprintf(out, "\n=== COULD-NOT-CHECK: %d row(s) — representing PR unknown; NOT dispatched (#5853) ===\n", len(unchecked))
		for _, l := range unchecked {
			fmt.Fprintf(out, "  %s — %s\n", l.briefID, l.unchecked)
		}
	}

	// AWAITING REWORK (#5853): `### Awaiting implementer rework` rows at implemented/verified. A
	// verification already FAILED each one with a cited blocker, so the owner is a worker and the owed
	// work is that blocker — never a re-run of the delivered brief as IMPLEMENT. Each row carries the
	// section's own Next act (`fix, cite <ref>`) so the blocker is not lost. DELIBERATELY absent from
	// DISPATCH above.
	if rework := f.awaitingReworkRows(); len(rework) > 0 {
		fmt.Fprintf(out, "\n=== AWAITING REWORK, owner worker: %d row(s) — verification failed with a cited blocker; NOT dispatched as IMPLEMENT (#5853) ===\n", len(rework))
		for _, r := range rework {
			act := r.NextAct
			if act == "" || act == "—" {
				act = "could-not-check (the board row names no next act)"
			}
			fmt.Fprintf(out, "  %s — status %s; next act: %s\n", r.ID(), r.Status, act)
		}
	}

	// WITHHELD BY STATUS (#5853): every other brief row whose own Status is known and is not
	// `todo`/`in-progress`. implemented/verified rows are delivered and await verification; a `done`
	// row owes nothing; any other status (blocked, malformed) is held at that status. None is
	// dispatched as IMPLEMENT, and each line says which of those it is.
	if withheld := f.withheldRows(); len(withheld) > 0 {
		fmt.Fprintf(out, "\n=== WITHHELD BY STATUS: %d row(s) — status is not todo/in-progress; NOT dispatched as IMPLEMENT (#5853) ===\n", len(withheld))
		for _, r := range withheld {
			fmt.Fprintf(out, "  %s — status %s (%s)\n", r.ID(), r.Status, withheldReason(r.Status))
		}
	}

	// ADVISORY write-scope overlap warnings, AFTER the queue rows.
	// These are coordination HINTS, not locks: nothing above was gated, delayed, or skipped on
	// account of an overlap — every eligible item was planned exactly as before. Disjoint
	// scopes print nothing; a candidate whose scopes cannot be derived is named
	// `could-not-derive` (three-state honest), never silently treated as clear.
	inflight, ierr := f.inFlightSource()
	if ierr != nil {
		// Advisory: an unreadable claim universe never fails the plan. Note it on stderr and
		// fall through with no in-flight items — the candidates' own could-not-derive lines
		// still print (they do not depend on the in-flight read), honoring three-state honesty.
		fmt.Fprintf(os.Stderr, "fanoutloop: WARNING: could not read in-flight dispatch claims for the write-scope overlap check (%v) — overlap warnings omitted\n", ierr)
		inflight = nil
	}
	for _, w := range loopengine.WriteOverlapWarnings(items, inflight) {
		fmt.Fprintln(out, w)
	}
	return nil
}

// classOf returns the per-class RESERVATION bucket an item belongs to (example-stream/05):
// "resume" (an orphan-PR resume item), "rework" (a durable repair obligation, or an
// Awaiting-implementer-rework row whose Status could not be read — a known-status rework row is
// never queued, #5853), or "fresh" (everything else — the Next-up board's ordinary rows). It is a pure read of the kind
// tag SelectQueue already stamps onto Payload — no new classification pass, no new state.
func classOf(it loopengine.Item) string {
	switch it.Payload["kind"] {
	case kindOrphan:
		return "resume"
	case kindRework:
		return "rework"
	default:
		return "fresh"
	}
}

// classLine is the per-class concurrency-reservation summary line (example-stream/05): it
// counts each item SelectQueue already returned into its class, then states whether the
// resume/rework floor applies THIS tick — floor = the loop's stored/default reservation summed
// over exactly the classes that have at least one item waiting, never idling a slot for a class
// with nothing in the queue. It is advisory, like the write-scope overlap warnings below it:
// nothing here drops, reorders, or gates a row `plan` already rendered — it states the ceiling a
// pool sizing itself off this ID should use.
func classLine(items []loopengine.Item) string {
	var resumeN, reworkN, freshN int
	for _, it := range items {
		switch classOf(it) {
		case "resume":
			resumeN++
		case "rework":
			reworkN++
		default:
			freshN++
		}
	}

	width, _, werr := deskkit.ResolvedWidth("worker-desk")
	reserve, _, rerr := deskkit.ResolvedReserve("worker-desk")
	if werr != nil || rerr != nil {
		err := werr
		if err == nil {
			err = rerr
		}
		return fmt.Sprintf("classes: resume=%d rework=%d fresh=%d (could-not-check reservation: %v)",
			resumeN, reworkN, freshN, err)
	}

	floor := 0
	if resumeN > 0 {
		floor += reserve["resume"]
	}
	if reworkN > 0 {
		floor += reserve["rework"]
	}
	if floor > 0 {
		return fmt.Sprintf("classes: resume=%d rework=%d fresh=%d (fresh capped at %d by reservation)",
			resumeN, reworkN, freshN, width-floor)
	}
	return fmt.Sprintf("classes: resume=%d rework=%d fresh=%d (no reservation applied: no reserved-class item waiting)",
		resumeN, reworkN, freshN)
}

const usage = `fanoutloop — worker-desk (batch-fanout) reference consumer of the drain engine.

USAGE:
  fanoutloop plan --root <repo> [--sha <targetSHA>] [--repo <owner/name>]
  fanoutloop --version

'plan' prints the deterministic scheduler output: the dispatch queue (a to:worker desk-inbox item —
a directed message to this desk — leads, then orphan resumes, then the rework class (durable repair
obligations, and an Awaiting-implementer-rework row only when its status could not be read), then
the Next-up board in board order — issue-<NN> placeholders INCLUDED, only a different loop's
review-request dispatch tokens skipped), each item's tier, and the exact dispatch instruction. It
spawns nothing and writes nothing. No brief row whose own status is known and is not todo/in-progress
is offered as IMPLEMENT (#5853): an Awaiting-implementer-rework row is listed under 'AWAITING REWORK,
owner worker' with its cited blocker (the section's 'fix, cite <ref>' next act), and any other such
row under 'WITHHELD BY STATUS' with what it waits on. The already-represented reconciliation (#1339) routes a fresh row
whose brief already has a MERGED PR to the LANDED-UNRECONCILED listing (its board cell just never
reconciled) and never dispatches it, routes one with an OPEN PR to the resume lane, and dispatches
only unrepresented rows. A PR that only AUTHORED the queued brief (a docs-only PR adding its brief
file) is not its delivery and represents nothing; an Authors: trailer never represents a brief; a could-not-check read HOLDS the fresh lane (a 'FRESH LANE HELD:' line states
why) rather than offering rows on an unverified forge. --repo names the repo to reconcile against; it
defaults to the repo the configured roots map --root to, else the checkout's origin remote. That
reconciliation's PR-list transport is DEFERRED (the closed forge surface ships no forge-CLI call; the
typed open+merged-changes op is the cutover work), so the shipped offline build performs no forge read
and offers rows as before until it is wired. The autonomous drive / live-window cutover is gate:human
— BLOCKED-ON-IAN.

Right after the item count, 'plan' prints a 'classes: resume=<n> rework=<n> fresh=<n> (...)' line: the
per-class concurrency RESERVATION (example-stream/05) worker-desk's width carries alongside its pool
size (deskroster width --role worker-desk --reserve resume=N,rework=M) — a floor of slots held for
resume/rework items so a full pool of fresh briefs cannot crowd them out. The line states whether the
floor applied this tick ('fresh capped at <k> by reservation') or did not ('no reservation applied: no
reserved-class item waiting') — a reservation never idles a slot when nothing reserved is waiting.

After the queue rows, 'plan' prints ADVISORY write-scope overlap warnings:
a 'WRITE-OVERLAP: <candidate> ~ <in-flight> on <prefix>' line whenever a candidate brief's write
scopes (derived from its Context 'files:' list) share a path prefix with an item already holding an
in-flight dispatch claim for the same root. These are COORDINATION HINTS, NOT LOCKS — nothing is
blocked, delayed, or skipped on account of an overlap. A brief whose scopes cannot be derived is
named 'could-not-derive' (never silently treated as clear); disjoint scopes print nothing.

Exit: 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner.
`
