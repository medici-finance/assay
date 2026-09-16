// Command desklabel sets or clears ONE label on an issue or change, through the resolved
// forge, under the calling session's own App role — and refuses every label that role does
// not own.
//
// THE GAP IT CLOSES. Every label-carrying write in the desk today is bundled into a bigger
// verb's own fixed set: deskflip's `authorization-needed` → `approval-needed` swap, deskclose
// superseded's `superseded?` proposal, deskdisposition set's `disposition:*` record. None of
// them is a general "set or clear one label" verb, and that is deliberate — a general label
// write is a general forgery surface. But the gap is real: `deskclose superseded --dispute`
// posts the verdict and applies `needs-decision`, and never removes the `superseded?` label
// the proposal applied, so an item a reviewer has just ruled NOT settled still carries the
// exact marker a sweep filters on as "awaiting a verdict". Clearing it had no verb; only a
// raw, unscoped label write reached it. This is that verb.
//
// THE CONTROL. A closed vocabulary table (vocabulary.go) says which role owns which label:
// the shared escalation vocabulary (`question`, `help wanted`, `needs-decision`) is any
// role's; `superseded?` and the `disposition:*` family are the WORKER's (they are
// worker-authored findings); `authorization-needed` / `approval-needed` are the REVIEWER's
// (deskflip's queue-state pair); `human-decided` is NOBODY's — it asserts a recorded human
// act, and a role self-applying it is the forgery class the two-role superseded lane exists
// to prevent. Anything not in the table is refused (exit 5). The check runs BEFORE any forge
// read or write, and the acting role is read from the SESSION (DESK_LOOP → App role via
// deskkit.SessionTokenRole), never from a flag: a `--as <role>` flag would be a claim; the
// session's minted role is a fact. The table is a Go literal with no runtime extension —
// no flag, environment variable or config file widens it.
//
// THE TARGET KIND. GitHub numbers issues and pull requests in one sequence and labels both
// through one endpoint; GitLab numbers issues and merge requests in SEPARATE sequences with
// separate endpoints. The verb learns which kind a number names from the seam's own typed
// read: `GetIssue` (op 2) resolves it and REFUSES a GitLab both-resolve, and `--kind
// issue|mr` routes that case through `GetIssueTyped` (op 38) — the seam's own answer to the
// ambiguity, not a second kind-resolution mechanism. The write is then `ApplyLabels` with
// the stated target, whose own idempotency (present label = no-op, absent removal = no-op)
// this verb inherits and reports.
//
// IDENTITY: the session-role App installation token (DESK_LOOP-selected) minted via
// desktoken and handed to the resolved deskkit.Forge under that App's custody — the
// deskclose precedent, verbatim. There is no ambient fallback and no worker default: an
// unresolvable role is a refusal, because the vocabulary is keyed on it.
//
// Exit codes (deskkit contract): 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused ·
// 6 unverifiable.
package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `desklabel — set or clear ONE label on an issue or change, as the session's own App role.

USAGE:
  desklabel add <owner/repo> <number> <label> [--kind issue|mr] [--dry-run]
  desklabel rm  <owner/repo> <number> <label> [--kind issue|mr] [--dry-run]
  desklabel vocabulary
  desklabel --version

add    — ensures <label> is on the target. Already present: no-op (exit 0), no write.
rm     — takes <label> off the target. Already absent: no-op (exit 0), no write.

VOCABULARY (closed; the acting role is read from the session, never from a flag):
  shared — any role may set or clear:   question · help wanted · needs-decision
  worker-owned:                          superseded? · disposition:superseded ·
                                         disposition:resolved-elsewhere · disposition:needs-rebase
  reviewer-owned:                        authorization-needed · approval-needed
  refused for EVERY role:                human-decided (it records a human act)
  anything else:                         refused — no desklabel vocabulary entry

--kind   states whether <number> is an issue or a change (mr; pr is an alias). Needed only
         where a bare number is ambiguous: a GitLab project carrying BOTH issue #N and
         merge request !N. On a one-sequence forge the stated kind is validated, never
         used to route.
--dry-run validates the ownership check and reads the target's kind and labels, then
         stops before the write, printing what would happen.

Exit: 0 ok/noop · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable.`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident. This
	// tool ACTS on the roster (it gates the repo it writes to), so it is ciEligible=false:
	// it reads the config-home file and never the environment, in CI as well as locally —
	// matching deskclose/deskfile.
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
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("desklabel sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
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

	sub, rest := args[0], args[1:]
	var err error
	switch sub {
	case "add":
		err = cmdAdd(rest, os.Stdout)
	case "rm":
		err = cmdRm(rest, os.Stdout)
	case "vocabulary":
		printVocabulary(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "desklabel: unknown subcommand %q\n\n%s\n", sub, usage)
		return deskkit.ExitRefused
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "desklabel:", err.Error())
	}
	return deskkit.ExitCodeOf(err)
}
