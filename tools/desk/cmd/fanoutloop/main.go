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
//	fanoutloop plan --root <repo> [--sha <targetSHA>]
//	    Deterministic scheduler OUTPUT: read the Next-up board (orphan resumes first, then rows in
//	    board order — issue-<NN> placeholders INCLUDED, only a different loop's `review-request`
//	    dispatch tokens skipped), compute each item's tier, and print the
//	    EXACT dispatch instruction the operator would execute as an Agent call. No agents are spawned,
//	    nothing is written, and no network is touched — this is the "the Go engine owns all scheduler
//	    state and emits exact dispatches" surface (§9.1).
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
	// Declare the roster class EXPLICITLY (never ClassWrite-by-omission): this tool ACTS on the
	// dispatch queue, so ciEligible=false — config-home file only, never the environment.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective trust/authority configuration to stderr before doing anything (P3): a
	// control surface that lives in settings rather than a diff is visible only at RUN time.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
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
		return loopengine.ExitCodeOf(cmdPlan(args[1:]))
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
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("bad flags: " + err.Error())
	}

	f := &FanoutLoop{Root: *root, TargetSHA: *sha}
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
	fmt.Fprintf(out, "worker-desk plan: %d item(s) to dispatch (orphan resumes first, then Next-up in board order)\n", len(items))
	fmt.Fprintln(out, classLine(items))
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
// "resume" (an orphan-PR resume item), "rework" (an Awaiting-implementer-rework row), or
// "fresh" (everything else — the Next-up board's ordinary rows). It is a pure read of the kind
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
  fanoutloop plan --root <repo> [--sha <targetSHA>]
  fanoutloop --version

'plan' prints the deterministic scheduler output: the dispatch queue (orphan resumes first, then
Awaiting-implementer-rework rows, then the Next-up board in board order — issue-<NN> placeholders
INCLUDED, only a different loop's review-request dispatch tokens skipped), each item's tier, and the
exact dispatch instruction. It spawns nothing, writes nothing, and touches no network. The autonomous
drive / live-window cutover is gate:human — BLOCKED-ON-IAN.

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
