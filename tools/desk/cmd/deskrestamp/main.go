// Command deskrestamp re-applies a change's existing dispatched-model/dispatched-tier
// stamp under a bound DISPATCHER identity — the first-class re-stamp path (#336).
//
// THE GAP IT CLOSES. The model-capability floor derives dispatch authority from WHO holds
// the standing application of a PR's dispatched-* labels: a bound dispatcher slug, or a
// login of the roster-configured ASSAY_STAMP_TRUSTED_LOGINS allowance. A stamp whose
// standing application is anything else — a foreign login, or a present label the timeline
// read cannot attribute — reads Indeterminate and every authority-bearing write on the PR
// is refused. The only repair an append-only label timeline offers is to REMOVE the
// unreadable labels and re-apply the same (model, tier) pair under the dispatcher, and
// until now that repair was buried inside the dispatch ceremony's stamp step, reachable
// only by re-running a whole dispatch. A migration over an existing backlog needs it as a
// verb of its own, with no claim, no worktree, and no prompt.
//
// CONTENT IS PRESERVED, NEVER INVENTED. The verb reads the stamp the PR ALREADY carries
// and re-applies exactly that (model, tier) pair. An unreadable or incomplete stamp —
// conflicting labels, a malformed half, only one of the two labels — is REFUSED: this verb
// is the repair for "the right stamp under the wrong actor", not for "the wrong stamp",
// which is the dispatch ceremony's own re-stamp step's territory (it validates an explicit
// --model/--tier). A re-stamp that could change what a stamp SAYS would let this verb
// launder content, and it cannot.
//
// THE PROVENANCE GATE (#336; SEC-1b round 3 — the driver's ruling on PR #1727, comment
// https://github.com/medici-finance/assay/pull/1727#issuecomment-5860170351,
// 2026-09-27T21:54:19Z, quoted): "deskrestamp may vouch only for dispatched-* labels
// applied by the driver's own login (the roster bless login) before
// 2026-09-27T00:00:00Z (the #336 legacy backlog); every other applier is refused."
// Preserving a stamp's content is not the same as vouching for whoever applied it. This
// verb re-attests a label ONLY when its standing application is BOTH the roster's own
// blessing authority (deskkit.IsRestampDriverLogin — never any other trusted login, and
// never the broader ASSAY_TRUSTED_LOGINS or ASSAY_STAMP_TRUSTED_LOGINS sets) AND
// timestamped strictly before deskkit.RestampDriverCutoff, checked PER LABEL so a
// half-swap timeline cannot slip through (SEC-1a) — an App, a bot, any other login
// (trusted or not), a post-cutoff application, or a present label the timeline cannot
// attribute to anyone at all is REFUSED before any write, never repaired here.
// Re-attesting an unvouched applier's content under the dispatcher would be laundering,
// not repair; that case is the dispatch ceremony's territory, which validates a fresh
// --model/--tier rather than re-stamping what is already there.
//
// WHO MAY RUN IT. The acting role is read from the SESSION (DESK_LOOP → App role via
// deskkit.SessionTokenRole, never a flag) and must be one of deskkit.DispatcherRoles() —
// the whole point is that the label's new standing application belongs to an identity the
// floor accepts, and a role outside that set would only replace one unreadable stamp with
// another. The labels are written under that role's own minted credential, never the
// ambient one.
//
// THE RECORD. The ruling this serves (#336) requires the re-stamp actor AND the original
// actor to be recorded on the PR. The verb therefore posts one comment per actual
// re-stamp naming the removed labels' previous applier(s) and the dispatcher identity the
// pair now stands under — the audit trail the append-only timeline itself cannot render
// (an `unlabeled` event does not say WHY).
//
// Exit codes (deskkit contract): 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused ·
// 6 unverifiable.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskrestamp — re-apply a change's existing dispatched-* stamp under the dispatcher's identity.

USAGE:
  deskrestamp <owner/repo> <pr> [--dry-run]
  deskrestamp --version

The PR must already carry a COMPLETE, READABLE stamp (one dispatched-model:<slug> label and
one dispatched-tier:<strong|any> label). deskrestamp removes whichever of those labels the
floor cannot attribute to an accepted authority — a foreign applier, or a present label the
timeline read cannot attribute at all — and re-applies the SAME pair under the session's
dispatcher App, so the floor reads the stamp as that App's attestation. It then posts one
comment recording both the original applier(s) and the re-stamp actor.

This repair is available only per label, and only when that label's standing application is
BOTH the roster's own blessing authority (the driver's own login, read dynamically — never
any other trusted login) AND applied strictly before 2026-09-27T00:00:00Z (the #336 legacy
backlog; see REFUSED below).

REFUSED (exit 5): an unreadable/incomplete stamp (re-dispatch validates a fresh --model/--tier
instead — this verb preserves content, it never invents it); a session role that is not a
bound dispatcher; a repo outside the desk-tools set; a standing foreign applier that is an
App, a bot, any login other than the roster's own blessing authority (trusted or not), a
blessing-authority application AT OR AFTER the cutoff, or a present label the timeline cannot
attribute to anyone at all — re-attesting that content under the dispatcher would be
laundering, not repair, so the dispatch ceremony (a fresh --model/--tier) is the sanctioned
path for those cases instead.
NOOP (exit 0): the pair already stands under an accepted label authority — no label provenance
to repair. This verb does not check claim liveness or establish current review authority;
it cannot renew a released dispatch. Review verdicts still check the live review claim family.

--dry-run reads the PR and its label timeline, prints what would be removed and re-applied,
         and writes nothing.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident. This
	// tool ACTS on the roster (the repo gate, the dispatcher-role check, the stamp-authority
	// allowance), so it is ciEligible=false: it reads the config-home file and never the
	// environment, in CI as well as locally — matching desklabel/deskclose.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Echo the effective roster once per run: a control surface that lives in settings
	// rather than in a diff is visible only at RUN time.
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	args = deskkit.TakeTraceFlag(args)
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskrestamp sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill switch is checked FIRST, before the verb's payload is parsed.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}
	deskkit.WarnIfUnpinned(os.Stderr)

	err := cmdReStamp(args, os.Stdout)
	if err != nil {
		deskkit.ReportError(os.Stderr, err)
	}
	return deskkit.ExitCodeOf(err)
}
