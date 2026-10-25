// Command verifyloop is the verify-desk REFERENCE CONSUMER of the deterministic drain engine
// (tools/desk/internal/loopengine). It exists to prove the engine
// contract end-to-end and to serve as the interim-mode debug surface — NOT (yet) to drive the
// standing verify window. The autonomous cutover (the verify desk actually booting this as its
// driver, writing Evidence + status straight to main) is gate: human — BLOCKED-ON-HUMAN.
//
// Subcommands:
//
//	verifyloop plan --root <repo> [--sha <targetSHA>] [--json] [--base-queue]
//	    Deterministic scheduler OUTPUT: read the Awaiting queue, compute each item's tier, and
//	    print the EXACT dispatch instruction the operator would execute (or the human-route
//	    note for a risk-flagged item). No agents are spawned, nothing is written — this is the
//	    "the Go engine owns all scheduler state and emits exact dispatches" surface (§9.1).
//
//	verifyloop --version
//
// The `run` (autonomous drive) path is deliberately NOT exposed here: a Go conductor cannot
// call the harness Agent tool, and wiring the live window is the human-gated cutover.
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/loopengine"
)

func main() {
	ctx := perfrec.ResolveContext(perfrec.ContextInput{})
	os.Exit(deskkit.PerfRunMeasured("verifyloop", verifyPerfVerbs, os.Args[1:], performanceCallback(ctx, startVerifyloop), verifyPerfPhases...))
}

// Share the production callback with offline tests. Startup is the existing
// activation boundary; it does not replace command or observer composition.
func performanceCallback(ctx perfrec.Context, startup func() bool) func([]string, *perfrec.Session) (int, perfrec.Context) {
	return func(args []string, session *perfrec.Session) (int, perfrec.Context) {
		if session != nil {
			session.SelectContext(ctx)
		}
		performance := verifyPerformance{session: session, context: ctx}
		stop := performance.phase("startup")
		active := startup()
		stop()
		if !active {
			return deskkit.ExitUnverifiable, ctx
		}
		return runMeasured(args, performance), ctx
	}
}

func startVerifyloop() bool {
	// This tool acts on the roster: config-home file only, never the CI environment.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	return deskkit.CheckVerbActivation(os.Stderr)
}

func run(args []string) int {
	return runMeasured(args, verifyPerformance{})
}

func runMeasured(args []string, performance verifyPerformance) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("verifyloop sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
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
	stopGuard := performance.phase("guard")
	err := deskkit.Guard()
	stopGuard()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	// `verifyloop --dry-run [--root .]` is shorthand for `verifyloop verdict --dry-run …`:
	// the deterministic runner's CI-testable surface (compose + sign + print, no filing).
	if args[0] == "--dry-run" || args[0] == "-dry-run" {
		deskkit.PerfCommand(verifyPerfVerbs, "verdict")
		return cmdVerdictMeasured(args, performance)
	}

	switch args[0] {
	case "verdict":
		// The deterministic runner: run check/check:ci rows, batch ~5 min, sign, print the
		// would-be verify-verdict issue body. Filing is BLOCKED-ON-HUMAN (autonomous cutover).
		return cmdVerdictMeasured(args[1:], performance)
	case "plan":
		// The OPERATING-ENVELOPE preflight runs at BOOT, before the queue is even
		// read. A red envelope is could-not-run for the WHOLE pass: one summary
		// line and exit — no queue read, no claim, no dispatch, and no issue filed
		// about the desk's own envelope.
		stop := performance.phase("preflight")
		err := preflightBoot(args[1:])
		stop()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return deskkit.ExitCodeOf(err)
		}
		// The boot just admitted each root's prerequisite record, so this pass binds every
		// run to one (desk-tools/50). A plan entered without the boot has no record to bind.
		return loopengine.ExitCodeOf(cmdPlanWorld(args[1:], performance, planWorldGate()))
	default:
		fmt.Fprintf(os.Stderr, "verifyloop: unknown subcommand %q\n\n%s", args[0], usage)
		return deskkit.ExitRefused
	}
}

func cmdPlan(args []string) error {
	return cmdPlanMeasured(args, verifyPerformance{})
}

// planWorldGate builds the booted plan's prerequisite gate. It is a package var ONLY as
// the test seam beside preflightRoot: a stubbed envelope admits no record to gate on.
var planWorldGate = newWorldGate

func cmdPlanMeasured(args []string, performance verifyPerformance) error {
	return cmdPlanWorld(args, performance, nil)
}

func cmdPlanWorld(args []string, performance verifyPerformance, world *worldGate) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	root := fs.String("root", ".", "repo root to scan for the Awaiting queue")
	sha := fs.String("sha", "", "merged-main target SHA verifiers run against")
	runner := fs.String("runner", "", "this session's runner identity (author!=runner guard)")
	noForge := fs.Bool("no-forge", false, "read neither blockers nor pending Evidence: unread observations are could-not-check")
	asJSON := fs.Bool("json", false, "emit the plan as one JSON document on stdout (diagnostics go to stderr)")
	baseQueue := fs.Bool("base-queue", false, "compose statusgen's canonical queue classification under the overlays (it can only demote)")
	if err := fs.Parse(args); err != nil {
		return deskkit.Refused("bad flags: " + err.Error())
	}

	v := &VerifyLoop{performance: performance, Root: *root, TargetSHA: *sha, RunnerID: *runner, RootRepo: deskkit.RepoSlugForDir(*root),
		World: world}
	if !*noForge {
		v.Issues = issueSourceFn()
		if source, ok := v.Issues.(*forgeIssueSource); ok {
			source.performance = performance
			source.observer = deskkit.RequestObserverFor(performance.context, performance.session, perfrec.OriginDirect)
		}
	} else {
		v.Issues = offlineScheduling{}
	}

	// MULTI-ROOT: DESK_ROOTS set and no explicit --root. The plan spans every configured
	// stream root — the same deskkit.ConfiguredRoots map deskboard reads — with a PER-ROOT
	// envelope preflight: a RED (or unreadable) sibling is reported and skipped, never a
	// whole-pass abort, so a desk booted on one root still sees the others' queues.
	//
	// The plan is built as ONE value (plan.go) and rendered from it, as text or as JSON.
	doc, err := buildPlan(v, planOpts{root: *root, sha: *sha, runner: *runner,
		multi: multiRootPlan(args), baseQueue: *baseQueue})
	if err != nil {
		return err
	}
	stopRender := performance.phase("render")
	defer stopRender()
	if *asJSON {
		// stdout carries the JSON document and nothing else; the human-readable summary of
		// what the plan could not cover goes to stderr.
		if err := renderPlanJSON(doc); err != nil {
			return deskkit.Unverifiable("cannot render the plan as JSON", err)
		}
		for _, g := range doc.Gaps {
			fmt.Fprintln(os.Stderr, "verifyloop plan: "+g)
		}
	} else {
		renderPlanText(doc)
	}
	if doc.Skipped > 0 {
		// The plan printed is complete for every root that was read; the exit code says the
		// queue it shows is NOT the whole queue. Could-not-check is never rounded up to green.
		return deskkit.Unverifiable(fmt.Sprintf(
			"%d configured root(s) skipped (preflight red or unreadable) — this plan is not the whole queue", doc.Skipped), nil)
	}
	if !doc.Complete {
		// A healthy root whose canonical queue was partial or unsupported (--base-queue) is a
		// named gap: the affected items were demoted to could-not-check, but a scripted caller
		// keying on the exit code must not read this plan as the whole queue either.
		return deskkit.Unverifiable(fmt.Sprintf(
			"the plan is incomplete (%d gap(s): %s) — this plan is not the whole queue",
			len(doc.Gaps), strings.Join(doc.Gaps, "; ")), nil)
	}
	return nil
}

// bucketMember is one non-dispatchable queue item: its ID plus the per-item reason detail
// (the blocked-until condition, the lane name, or the repair pipeline reference; empty for
// awaiting-human, whose why is identical for every member).
type bucketMember struct {
	ID     string
	Reason string
}

// evidenceOnlyMarker is the literal appended to every awaiting-human member line whose brief
// carries a risk answer yes: it states, on the same line as the reason, that Evidence-gathering
// is permitted but a status flip is not — so the ROUTE-HUMAN permission and its limit are read
// together. Land enforces the limit structurally (no flip for a risk-flagged / irreversible item).
const evidenceOnlyMarker = "Evidence-only (never flip-eligible)"

// bucketHeading is the human-facing heading name for a disposition. It equals the stable slug
// (disposition.String(), which tests and reason lookups key on) EXCEPT for awaiting-human, where
// the routing destination ROUTE-HUMAN is made visible in the plan output so a reader can tell a
// held risk-flagged brief from a dispatchable one at a glance.
func bucketHeading(d disposition) string {
	if d == dispAwaitingHuman {
		return d.String() + " / ROUTE-HUMAN"
	}
	return d.String()
}

// printBuckets renders the deferred section and each non-empty bucket, each headed by a count
// and its one-line "why it waits" note. The order is deterministic (deferred, then the three
// buckets) so the output is stable across runs.
func printBuckets(dispatchable int, bucketed map[disposition][]bucketMember) {
	total := 0
	for _, m := range bucketed {
		total += len(m)
	}
	fmt.Printf("\nverify-desk plan: %d dispatchable, %d deferred/bucketed (not offline-convertible this run)\n",
		dispatchable, total)
	for _, disp := range []disposition{dispCouldNotCheck, dispDeferred, dispWaitReceipt, dispAwaitingMerge, dispAwaitingHuman, dispStuckFlip, dispAwaitingOnlineLane, dispInRepair, dispBaseHeld} {
		members := bucketed[disp]
		if len(members) == 0 {
			continue
		}
		fmt.Printf("\n-- %s (%d): %s\n", bucketHeading(disp), len(members), disp.whyItWaits())
		for _, m := range members {
			switch {
			case disp == dispWaitReceipt:
				// verify-reset/03: one WAIT line per held brief, naming its blocker ref.
				fmt.Printf("   WAIT %s — %s\n", m.ID, m.Reason)
			case disp == dispAwaitingHuman && m.Reason != "":
				// A risk-flagged item: its risk reason plus the Evidence-only marker, so the
				// permission (may gather Evidence) and the limit (never flips) sit on one line.
				fmt.Printf("   %s — %s — %s\n", m.ID, m.Reason, evidenceOnlyMarker)
			case m.Reason != "":
				fmt.Printf("   %s — %s\n", m.ID, m.Reason)
			default:
				fmt.Printf("   %s\n", m.ID)
			}
		}
	}
	// verify-reset/03: the hold-table summary — held on an open blocker, dispatchable, and held
	// because an input or the blocker could not be read.
	fmt.Printf("\nverify-desk plan: wait=%d dispatchable=%d could-not-check=%d\n",
		len(bucketed[dispWaitReceipt])+len(bucketed[dispAwaitingMerge]), dispatchable, len(bucketed[dispCouldNotCheck]))
}

const usage = `verifyloop — verify-desk reference consumer of the drain engine.

USAGE:
  verifyloop plan    [--root <repo>] [--sha <targetSHA>] [--runner <id>] [--no-forge]
                     [--json] [--base-queue]
  verifyloop verdict --root <repo> [--dry-run] [--window 5m] [--runner <id>] [--pem <path>]
                     [--unsigned-out <file>]
  verifyloop --dry-run [--root <repo>]        # shorthand for 'verdict --dry-run'
  verifyloop --version

'plan' prints the deterministic scheduler output: the Awaiting queue, each item's tier, and
the exact dispatch instruction (or the human-route note). It spawns nothing and writes nothing.
The item keys it prints (<stream>/<NN>) resolve to their file, frontmatter and board row with
'statusgen brief <key>'.

'plan' is MULTI-ROOT when DESK_ROOTS is set and no --root is given: it iterates every
configured stream root (the same <owner>/<repo>=<path> map deskboard reads), runs the envelope
preflight PER ROOT (a red or unreadable sibling is reported and skipped, never a whole-pass
abort; the exit is then 6 because the plan is not the whole queue), and names the root on every
item (<owner>/<repo>:<stream>/<NN>, plus root=<path> on the DISPATCH header). An explicit --root
narrows to that one root; with DESK_ROOTS unset the plan is the single --root read (default .).

'plan' is built as ONE plan value and rendered from it. --json prints that value as a single
JSON document on stdout and nothing else (the text lines, and any note about what the plan could
not cover, never reach stdout in that mode; gaps go to stderr). The document names every root with
an explicit state (healthy, skipped on a red preflight, unreadable), every item with its kind,
disposition, tier, reason and authority (flip, evidence-only or none), the counts, and whether the
plan is complete. A skipped or unreadable root is a named gap and exit 6, never a zero; under
--base-queue a healthy root whose canonical queue is partial or unsupported is likewise a named
gap and exit 6 (complete=false never exits 0).

'plan --base-queue' composes statusgen's CANONICAL queue classification (the queueClass fields on
'statusgen --gate-scores' rows, read with STATUSGEN_BIN or statusgen on PATH) under verifyloop's own
overlays. PRECEDENCE: the overlays decide first (could-not-check, blocked-until, human gate or
risk, stuck-flip, wake receipt, in-repair, online lane) and a held item stays held; the canonical
class is applied last and can only DEMOTE an item the overlays would dispatch. A DISPATCH item must
be judgment-dispatch, else it is held as queue-held (runnable-offline, continuation, human-action,
paused, parked) with the class, owner and next act on its line; an unreadable class, a row the
canonical queue does not have, or a statusgen that predates the contract is could-not-check, never
dispatch. A DISPATCH-FOR-EVIDENCE item stays Evidence-only: human-action and judgment-dispatch keep
it, the other classes demote it, and it never gains flip authority. Without --base-queue the plan
is exactly the overlay plan.

'plan' FAILS SAFE on risk: any brief with gate:human OR any risk answer yes (irreversible
first) is bucketed under awaiting-human / ROUTE-HUMAN, never DISPATCH. A model MAY gather
Evidence for such a brief — its member line says 'Evidence-only (never flip-eligible)' — but
never flips it; the human's merge of the checkpoint PR is the flip. When such a brief's Evidence
is still EMPTY (and nothing else withholds an offline run) it is emitted as a second dispatchable
class, DISPATCH-FOR-EVIDENCE, after the DISPATCH set: Evidence rows + the outcome sidecar row
only, flip never. A human-gated brief whose Evidence is already gathered stays awaiting-human.

'plan' HOLDS a verify-fail/blocked brief until its blocker moves (docs/verify-wake.md): a
complete receipt whose declared inputs are unchanged and whose blocker issue reads open is a
WAIT line naming the ref and the next actor, never a DISPATCH. Changed inputs, a closed blocker
or a newer explicit-recheck receipt wake it; an unreadable input or blocker read is
could-not-check, never "unchanged" or "closed". The blocker is read through the forge as the
verifier App, only in a repository inside the configured set (one outside it is could-not-check,
never read), and each repository is named on stderr before its first read. A bare #N resolves
in the record's repo, else the root's origin repository. Current-source Evidence in an open,
trusted verifier PR appears as awaiting-merge #N. Its exact attestation and existing wake
contract govern freshness; pending Evidence grants no lifecycle authority. --no-forge reads
neither blockers nor pending Evidence, so the queue is could-not-check. The last line is
'verify-desk plan: wait=<n> dispatchable=<n> could-not-check=<n>'.

'verdict' is the DETERMINISTIC runner: it runs each brief's check/check:ci Verify rows locally
(exit code = verdict), batches results over the flush window into ONE signed verdict-v1
payload, and prints the would-be verifier-App issue body. --dry-run composes + signs + prints
without filing (the CI-testable surface). A missing verifier PEM is a loud envelope error and
nothing is signed. Filing the issue is the autonomous cutover — gate: human, BLOCKED-ON-HUMAN.

'verdict --unsigned-out <file>' is the keyless half of a split run, for rows that execute in a
fence (a sealed container) that holds no key: it runs the same rows, composes ONE canonical
verdict-v1 payload over all of them, writes it to <file> (which must not exist) UNSIGNED and
prints one line carrying its sha256 and nothing else to bind to. This mode resolves, opens and
reads no key: not --pem (refused with this flag, as are --dry-run and --window), not
VERIFIER_PEM, not any config-home verifier-app.pem. It never prints a fenced verdict body. Any
non-zero exit leaves no file of its own and prints no digest; an empty queue writes no file.
The signature happens afterwards on the key-holding host, under this host contract:
  H1  Before starting the fence, the host writes its own dispatch record outside the fence:
      repo, head sha and dispatch time (whole seconds, RFC3339, no fraction). It runs the
      composer with explicit --repo and --sha from that record.
  H2  It signs only after the composer exited 0 with exactly one sha256=<64 lowercase hex>
      value on stdout, and only once the fence has fully exited with no row process left
      alive; it adds that fence exit time (whole seconds) to its dispatch record. Any other
      exit, no sha256= value or more than one means do not sign, whatever file is present.
  H3  It signs with deskverdict sign --payload <file> --expect-sha256 <digest from H2>
      --expect-repo <repo> --expect-head <sha> --not-before <dispatch time>
      --not-after <fence exit time>, every value but the digest taken from its own dispatch
      record and never from anything the fence printed or wrote; it passes all five flags and
      stops on any non-zero exit.
  H4  It takes the signed body from sign's stdout, never from a .out file in a directory the
      fence could write.
  H5  It signs only a payload in a directory owned by the signing user that the composing uid
      cannot write (every directory above it equally out of reach), and the composing uid is
      not the signing user. After the fence has fully exited it moves the payload entry there
      by rename, never by copy; if the rename fails it does not sign. On unix the signer checks
      the immediate parent; on other platforms this contract is the only control.
  H6  The fence's composer needs a roster and no key. Like every desk binary it runs the
      activation check before it dispatches any subcommand, so in a checkout with no usable
      roster it exits 6 ('could-not-check: assay/desk-tools inactive — assay.roster.trust
      unset or malformed'), writes no payload and prints no digest, which H2 reads as do not
      sign. The roster is read from $HOME/.config/assay/roster.env only (ASSAY_CONFIG_HOME does
      not move it): the host gives the composing uid a HOME of its own inside the fence whose
      .config/assay directory holds a copy of the roster and nothing else (ASSAY_BLESS_LOGIN and
      ASSAY_TRUSTED_LOGINS suffice), the file mode 0600, the file and its directory both owned
      by that uid and neither writable by group or other. That directory is also a place the verifier key is looked
      up, so the host never mounts its own config home, never lets verifier-app.pem exist in
      this HOME or in ASSAY_CONFIG_HOME, and leaves VERIFIER_PEM unset in the fence. On an
      activation exit 6 it fixes the fence's HOME; it does not mount the host's config home.
      Running the composer from outside any checkout skips the check, but this contract does
      not rely on that.
Within the dispatched scope a hostile row can still invent results. The binding is checked only
when the host signs: the host refuses a payload whose digest, repo, head or ts does not match its
own record. It does not limit what the entries claim, and nothing after signing checks head or ts
again (statusgen's transcriber checks only the repo), so a forged entry in a signed payload is
consumed for any brief it names in that repo.

Exit: 0 ok · 3 disabled · 5 refused · 6 unverifiable · 7 author==runner.
`
