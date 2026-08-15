// issueboard — the issue-loop desk's read-only cross-repo board (issue #703), the
// twin of tools/desk/cmd/deskboard/board.go. It answers what the issue-loop desk
// needs to know across the owned repo set WITHOUT any mutating GitHub call: one
// ACTION per open issue (ESCALATE / CREATE-PLACEHOLDER / RETIRE / AWAIT / NONE) plus
// one row per untriaged intake entry, flagging those past the 3-day threshold.
// ESCALATE (brief loop-engine/13) is a computed CLASS, not a posted comment: a
// decision-owed issue (needs-decision/question label) whose last human response is
// older than --sla-days sorts to the top of the lane with its age rendered — acting
// on it (a re-ping, raising to the human) stays the desk's judgment, never this tool's.
//
// GET-only end to end: every gh invocation is a read (`issue list`, `issue view`,
// comments GET), proven by the PATH-shim test. A gh/parse error on any owned repo
// fails the whole run (exit 6, repo named) — never a partial board.
//
// Trust gate (deskkit/trust.go): an open issue authored outside the compiled-in
// trusted set (the configured humans and desk Apps) with no blessing comment is
// QUARANTINED — listed under EXTERNAL / UNBLESSED, counted, never given an ACTION.
// One comment from the blessing authority admits it. Comment authors are fetched only for untrusted-author
// issues, keeping API growth bounded.
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 5 refused (bad subcommand) ·
// 6 unverifiable (a read failed / a local file could not be parsed).
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var usage = `issueboard — read-only issue + intake board (desk-tools, issue #703)

usage:
  issueboard              full board: issue lane + intake lane (default)
  issueboard board        same as no subcommand
  issueboard issues       issue lane only (one ACTION per open issue)
  issueboard intake       intake lane only (untriaged entries, age-flagged)
  issueboard --version    source SHA / build time

flags:
  --root <path>      repo root to read docs/streams/{issue-loop,intake} from (default ".")
  --sla-days <N>     decision-owed silence threshold before ESCALATE (default 6, the
                      market-scan "silent 6d" figure — desk-console-design.md §13.3)

escalation (brief loop-engine/13): an open issue carrying a needs-decision or
question label ages against --sla-days, counted from the last HUMAN response (a bot
comment never resets the clock). Under the SLA it classifies AWAIT; past it, ESCALATE
— sorted to the top of the issue lane, age rendered in the row.

owned repos (the intake SCAN scope): the issue lane READS the owned-repo set
resolved at runtime from the ASSAY_SCAN_REPOS roster key (deskkit.ScanRepos) — the
desk-side half of the owned-repo roster, externalised out of source so it cannot
drift from statusgen's --scan-issues scanner, which reads the IDENTICAL key. It is a
DISTINCT set from the write boundary (ASSAY_ALLOWED_REPOS): the scan scope may cover
repos the desk is the front door for that are not write targets, and the desk still
POSTS only where deskpost/deskpr/deskreply gate independently on
deskkit.IsAllowedRepo. The set is configured in CI (the repository/organization
Actions variable) or the config-home roster file, never compiled in — run the tool
to see the effective value echoed to stderr. An UNSET or EMPTY ASSAY_SCAN_REPOS is
refused LOUDLY (exit 6, COULD-NOT-CHECK): an empty sweep is never reported as a
clean, empty board (#777). And a repo the token cannot read fails the WHOLE board
with exit 6 (unverifiable), so the board is never silently partial — unlike
statusgen --scan-issues, which NOTICEs and skips.

trust gate: issues authored outside the configured trusted set (humans + desk Apps)
with no blessing comment are quarantined under EXTERNAL / UNBLESSED — visible,
never actionable. A comment from the configured blessing authority admits it.
`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (correctness review finding: SetToolClass had no caller anywhere
	// in the tree, so every tool was ClassWrite only because nothing ever set it).
	// This tool ACTS on the roster, so ciEligible=false: config-home file only, never
	// the environment, in CI as well as locally.
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	// Every run echoes the EFFECTIVE trust/authority
	// configuration to stderr before it does anything. The roster, the allowed-repo
	// set and the risk-path additions live in repository settings or a config-home
	// file, not in a diff — so the RUN is the only place a change to them becomes
	// visible, and a NARROWING has to be as visible as a widening. Logins and paths
	// only; never a token or a credential path.
	deskkit.EchoEffectiveConfig(os.Stderr)
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	// --version short-circuits before Guard (pure introspection, no side effects).
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		s, b := deskkit.Version()
		fmt.Fprintf(stdout, "issueboard sourceSHA=%s builtAt=%s releaseTag=%s\n", s, b, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}

	// Kill switch FIRST, before any read.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}

	root := "."
	slaDays := escalateSLADays
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--root":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "refused: --root needs a path")
				return deskkit.ExitRefused
			}
			root = args[i+1]
			i++
		case a == "--sla-days":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "refused: --sla-days needs a number")
				return deskkit.ExitRefused
			}
			n, perr := strconv.Atoi(args[i+1])
			if perr != nil || n < 0 {
				fmt.Fprintln(stderr, "refused: --sla-days must be a non-negative integer, got "+strconv.Quote(args[i+1]))
				return deskkit.ExitRefused
			}
			slaDays = n
			i++
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, usage)
			return deskkit.ExitOK
		default:
			pos = append(pos, a)
		}
	}

	sub := "board"
	if len(pos) > 0 {
		sub = pos[0]
	}

	now := time.Now().UTC()
	rep, err := dispatch(sub, root, now, slaDays)
	if err != nil {
		logRun(sub, resultFor(err), err.Error())
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}

	rep.render(stdout)
	logRun(sub, deskkit.ResultOK, rep.detail)
	return deskkit.ExitOK
}

// dispatch routes a subcommand to its handler. An unknown subcommand is a Refused
// (exit 5), never a guessed default.
func dispatch(sub, root string, now time.Time, slaDays int) (*Report, error) {
	switch sub {
	case "board":
		return cmdBoard(root, now, slaDays)
	case "issues":
		return cmdIssues(root, now, slaDays)
	case "intake":
		return cmdIntake(root, now)
	default:
		return nil, deskkit.Refused("refused: unknown subcommand " + strconv.Quote(sub) + " (see --help)")
	}
}

// logRun appends the mandatory per-run audit line.
func logRun(verb, result, detail string) {
	if err := deskkit.Log(deskkit.Entry{Tool: "issueboard", Verb: verb, Result: result, Detail: detail}); err != nil {
		fmt.Fprintln(os.Stderr, "issueboard: WARNING could not write audit line:", err)
	}
}

// resultFor maps a typed error to its audit result string.
func resultFor(err error) string {
	switch {
	case deskkit.IsDisabled(err):
		return deskkit.ResultDisabled
	case deskkit.IsRateLimited(err):
		return deskkit.ResultRateLimited
	case deskkit.IsRefused(err):
		return deskkit.ResultRefused
	default:
		return deskkit.ResultUnverifiable
	}
}
