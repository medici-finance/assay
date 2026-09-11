// Command deskfile is the filing-gate desk tool.
//
// It makes the filing-discipline ruling binding: dedupe BEFORE filing, attach
// instances to class issues, and budget filings. An issue that should have been a comment
// on a class issue never gets minted.
//
// Repo scope comes from deskkit.IsAllowedRepo — deskfile introduces NO new repo list. The
// filing identity is the session-role credential the forge resolver hands back
// (deskkit.ResolveForge: an App installation token on GitHub, the role PAT on GitLab);
// deskfile gates WHETHER and WHERE, not WHO.
//
// Exit codes (deskkit contract): 0 success/noop · 3 disabled ·
// 4 rate-limited · 5 refused · 6 unverifiable. See deskkit/exitcodes.go.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskfile — filing gate: mandatory dedupe, class-issue attach, per-pass budget.

USAGE:
  deskfile new    -R <owner/repo> --title <t> --body-file <f> [--label ...] [--raised-by <role>]
                  [--to <role>] [--force-new --reason <r>]
  deskfile attach -R <owner/repo> --to <N> --body-file <f>
  deskfile check  -R <owner/repo> --title <t>
  deskfile --version

new    — file a new issue. Runs a dedupe search against the repo's OPEN issues first: if a
         candidate scores at/above the match threshold it REFUSES (exit 5) and prints the
         exact ` + "`attach`" + ` command for the top candidate. A search API failure fails CLOSED
         (exit 6) — minting a possibly-duplicate issue is the expensive direction. The
         escape hatch is --force-new --reason <r>, which bypasses the search and is
         audit-logged. A session may file at most 3 new issues per repo per rolling 24h
         (exit 4 over); attach comments are never budgeted. The budget is charged to the
         FILING agent's own session tag ($DESK_SESSION ahead of the harness session id a
         dispatched agent inherits), so each agent in a fan-out has its own 3.

         --raised-by <role> stamps the provenance label raised-by:<role> so the by-desk
         issue metric can attribute the filing. The role vocabulary is DERIVED from the
         roster's role-bindings (the role= prefixes on ASSAY_TRUSTED_BOT_SLUGS) — an
         unbound role is REFUSED (exit 5) with the bound set named. Everything else about
         the stamp degrades rather than blocks: omit the flag, or have the label not exist
         on the repo, or fail to check whether it exists, and the issue is still filed —
         UNSTAMPED, with a NOTICE, and its provenance reads as UNKNOWN. Unknown is the
         absence of an answer, never "human-raised". Each outcome is distinguished on the
         audit line (raised-by=<role> | UNSTAMPED:not-requested | UNSTAMPED:label-missing |
         UNSTAMPED:could-not-check). The raised-by:* labels are not forge defaults and
         deskfile does not create them; the NOTICE prints the one-off create command for
         the repo's forge (gh label create on GitHub, glab label create on GitLab).

         --to <role> ADDRESSES the issue to a desk: it stamps the label to:<role> so that
         desk's own sweep (fanoutloop / issueboard) leads with the issue, turning a typed
         "tell the-desk…" relay into a durable, forge-visible message. It takes the SAME
         role vocabulary as --raised-by (one resolver, two flags) — an unbound role is
         REFUSED (exit 5) — and degrades the same way when the to:<role> label does not yet
         exist on the repo (filed UNADDRESSED, a NOTICE prints the one-off forge-selected
         label create). Omitting --to is the normal case and is SILENT. The audit line records
         to=<role> or to=UNADDRESSED:<reason>. CAUTION: on the new subcommand, --to takes a
         ROLE; on the attach subcommand (below), --to takes an issue NUMBER — same token,
         two meanings by subcommand.

attach — post an observation as a comment on issue N (a class issue or duplicate target).
         Never budgeted. Refuses (exit 5) if N is CLOSED, with reopen-or-new guidance.

check  — dry-run dedupe: prints candidates and exits 0/5 the same as ` + "`new`" + ` would, but
         writes nothing. The verb skills embed in authoring loops.

The body is read from --body-file only (no stdin/inline), capped at 16 KiB, and secret-
scanned; there is no override flag. <owner/repo> must be in the desk-tools repo set
(deskkit.allowedRepos — deskfile adds no list of its own).

FORGE: every verb runs through the typed forge backend the resolver picks for the repo
(the roster's ASSAY_REPO_FORGES binding, else an unambiguous origin host) — GitHub AND
GitLab are both served; nothing shells gh or glab. A repo whose forge cannot be resolved
is a could-not-check REFUSAL (exit 6) naming the configuration that would resolve it,
never an assumed GitHub. Do not substitute a bare glab/gh call on any forge: it bypasses
this tool's dedupe/stamp/budget gates.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.

DIAGNOSTICS: DESK_TRACE=1 (or a global --trace, any position) prints the full cause
chain, every child process with its command line, exit status and elapsed time, and the
failing child's stderr in full. Credentials are redacted. With it off, output is
unchanged. See tools/desk/README.md, "Diagnostics — DESK_TRACE".`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (a correctness review found: SetToolClass had no caller anywhere,
	// so "ClassWrite is the safe default" was true only by luck — and
	// TestEveryRosterReadingMainDeclaresClassAndEchoes now fails any main that reads
	// the roster without one). deskfile ACTS on the roster (files issues, posts
	// comments), so ciEligible=false: it reads the config-home file and never the
	// environment, in CI as well as locally — matching deskpr/deskboard.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// P3: echo the effective roster once per run. A control surface that lives in
	// settings rather than in a diff is visible only at RUN time; without the echo a
	// NARROWING is invisible.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	// The global diagnostic switch, taken BEFORE any verb dispatch so `--trace` works on
	// every deskfile verb and is invisible to each one's own FlagSet.
	args = deskkit.TakeTraceFlag(args)

	// --version / help are pure reads: no kill-switch gate, no audit line.
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskfile sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	// The kill-switch check is the FIRST action of the tool. Guard writes its own
	// result=disabled audit line and maps to exit 3.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Outward verbs present a LOOP IDENTITY. The kill switch's per-loop halt is
	// `STOP.<loop>`, matched against $DESK_LOOP; with the variable unset nothing matches,
	// so a stop flag a human is holding never fires and this verb keeps writing while the
	// operator believes it has been halted. The boot verb has checked this since it was
	// written — an outward verb run OUTSIDE a booted window did not, which is the gap.
	if err := deskkit.RequireLoopIdentity("deskfile"); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Running from source (go run / unstamped) is a drift risk — say so loudly.
	deskkit.WarnIfUnpinned(os.Stderr)

	verb := args[0]
	rest := args[1:]
	var err error
	switch verb {
	case "new":
		err = cmdNew(rest)
	case "attach":
		err = cmdAttach(rest)
	case "check":
		err = cmdCheck(rest)
	default:
		err = deskkit.Refused("refused: unknown verb " + verb + " (want one of: new, attach, check)")
	}
	// The shared exit path. With DESK_TRACE off this is byte-identical to the
	// fmt.Fprintln(os.Stderr, err.Error()) it replaces.
	deskkit.ReportError(os.Stderr, err)
	return deskkit.ExitCodeOf(err)
}
