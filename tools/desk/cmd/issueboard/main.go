// issueboard — the issue-loop desk's read-only cross-repo board (issue #703), the
// twin of tools/desk/cmd/deskboard/board.go. It answers what the issue-loop desk
// needs to know across the owned repo set WITHOUT any mutating GitHub call: one
// ACTION per open issue (ESCALATE / CREATE-PLACEHOLDER / RETIRE / AWAIT / NONE) plus
// one row per untriaged intake entry, flagging those past the 3-day threshold.
// ESCALATE is a computed CLASS, not a posted comment: a
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
// Exit codes (deskkit contract): 0 ok · 3 disabled · 5 refused (bad subcommand, flag or value) ·
// 6 unverifiable (a read failed / a local file could not be parsed).
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// helpLong is the root help's prose. The usage lines and flags are generated from the command
// tree (desktools-v2/45); this is what the tree cannot say on its own.
const helpLong = `issueboard — read-only issue + intake board (desk-tools, issue #703).

With no subcommand it renders the full board: the issue lane (one ACTION per open issue)
and the intake lane (untriaged entries, age-flagged).

desk inbox: an open issue labelled to:<role> (deskfile new --to <role>) is a durable
message TO a desk. It leads its addressee's sweep and is held out of un-briefed work. A
to:<role> item aged past --sla-days with NO comment from that role's App classifies
ESCALATE — an unread inbox becomes visible without the addressee's cooperation (a second,
independent layer over the addressee's own sweep). The human-response clock is unchanged.
Without --to, an issue carrying a to:<role> label renders ADDRESSED→<role> in its own
priority band and is EXCLUDED from the un-briefed (CREATE-PLACEHOLDER) work — it is the
addressee's item, not free work.

escalation: an open issue carrying a needs-decision or
question label ages against --sla-days, counted from the last HUMAN response (a bot
comment never resets the clock). Under the SLA it classifies AWAIT; past it, ESCALATE
— sorted to the top of the issue lane, age rendered in the row. The default (6) is the
market-scan "silent 6d" figure — desk-console-design.md §13.3.

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

retire: a placeholder whose issue is absent from the open-issue listing is RETIRE
only once a direct per-issue read positively says closed. A read that fails is
COULD-NOT-CHECK (exit 6, issue named), and an absent issue that reads OPEN proves the
listing came back partial — the whole sweep is then refused (exit 6) rather than
retiring the live placeholders the listing omitted (#1032).

Exit codes: 0 ok · 3 disabled · 5 refused (bad arguments) · 6 unverifiable.`

func main() {
	os.Exit(deskkit.PerfRun("issueboard", nil, os.Args[1:], func([]string) int {
		return execute(os.Args[1:], os.Stdout, os.Stderr, startup)
	}))
}

// startup is what every real run does first, after its command line parsed and its values
// passed their checks, and never for help or version (desktools-v2/45: help reads nothing).
//
// The roster class is an EXPLICIT declaration, never the zero value by accident
// (correctness review finding: SetToolClass had no caller anywhere
// in the tree, so every tool was ClassWrite only because nothing ever set it).
// This tool ACTS on the roster, so ciEligible=false: config-home file only, never
// the environment, in CI as well as locally.
// Every run echoes the EFFECTIVE trust/authority
// configuration to stderr before it does anything. The roster, the allowed-repo
// set and the risk-path additions live in repository settings or a config-home
// file, not in a diff — so the RUN is the only place a change to them becomes
// visible, and a NARROWING has to be as visible as a widening. Logins and paths
// only; never a token or a credential path.
func startup(stderr io.Writer) int {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(stderr)
	if !deskkit.CheckVerbActivation(stderr) {
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

// run is execute without the process startup: the in-package tests' entry, as before.
func run(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr, nil)
}

// guard is the kill switch, a variable so the contract tests can count it.
var guard = deskkit.Guard

// dispatchFn is the typed handler the command tree calls; the contract tests replace it to
// observe the values that reach it.
var dispatchFn = dispatch

// onResolve, when set, sees every invocation's resolved settings (the contract tests' window on
// which source won).
var onResolve func(*cli.Values)

// exitErr is a handler's exit code carried as an error, so cli.Run keeps it.
type exitErr int

func (e exitErr) Error() string { return "exit " + strconv.Itoa(int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

func codeErr(code int) error {
	if code == deskkit.ExitOK {
		return nil
	}
	return exitErr(code)
}

// versionLine is what --version and the legacy `version` word print.
func versionLine() string {
	s, b := deskkit.Version()
	return fmt.Sprintf("issueboard sourceSHA=%s builtAt=%s releaseTag=%s", s, b, deskkit.ReleaseTagOrDev())
}

// bindings are every setting issueboard declares: flags and defaults only, all persistent so
// they work before or after the lane word, as the hand-rolled loop read them anywhere. The tool
// has no configuration file and reads no environment variable for them.
func bindings() []cli.Binding {
	return []cli.Binding{
		{Key: "root", Kind: cli.String, Flag: "root", Persistent: true, Default: ".",
			Usage: "repo root `path` to read docs/streams/{issue-loop,intake} from"},
		{Key: "sla-days", Kind: cli.String, Flag: "sla-days", Persistent: true, Default: strconv.Itoa(escalateSLADays),
			Usage: "decision-owed silence threshold of `N` days before ESCALATE"},
		{Key: "to", Kind: cli.String, Flag: "to", Persistent: true,
			Usage: "(issues lane only) restrict to the desk-inbox items addressed to `role` (to:<role>)"},
	}
}

// execute runs one invocation against a fresh command tree. pre is the process startup (nil in
// the in-package tests, as run never did it).
func execute(args []string, stdout, stderr io.Writer, pre func(io.Writer) int) int {
	build := func() *cobra.Command {
		root := cli.NewRoot("issueboard", "read-only issue + intake board")
		root.Long = helpLong
		set := cli.Declare(root, bindings()...)
		lane := func(sub string) func(*cobra.Command, []string) error {
			return func(*cobra.Command, []string) error {
				return codeErr(runLane(sub, set, stdout, stderr, pre))
			}
		}
		// An unknown lane word is a refusal before anything runs, never a guessed default.
		root.Args = func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("refused: unknown subcommand %s (see --help)", strconv.Quote(args[0]))
			}
			return nil
		}
		root.RunE = lane("board")
		for _, l := range []struct{ name, short string }{
			{"board", "full board: issue lane + intake lane (same as no subcommand)"},
			{"issues", "issue lane only (one ACTION per open issue); --to <role> narrows it to a desk inbox"},
			{"intake", "intake lane only (untriaged entries, age-flagged)"},
		} {
			// Words after the lane were ignored by the hand-rolled loop; they still are.
			root.AddCommand(&cobra.Command{Use: l.name, Short: l.short, Args: cobra.ArbitraryArgs, RunE: lane(l.name)})
		}
		root.AddCommand(&cobra.Command{Use: "version", Short: "print the version", Hidden: true, Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				fmt.Fprintln(stdout, versionLine())
				return nil
			}})
		return root
	}
	return cli.Run(build, args, cli.Options{
		IO:              cli.IO{Out: stdout, Err: stderr},
		Version:         versionLine(),
		VersionTemplate: "{{.Version}}\n",
		// A parse error keeps the legacy refusal code (5), not the adapter's default.
		UsageExit: deskkit.ExitRefused,
	})
}

// runLane resolves and checks the settings, then runs the process startup, the kill switch and
// the lane, in that order: a bad value is refused before anything is read or echoed.
func runLane(sub string, set *cli.Set, stdout, stderr io.Writer, pre func(io.Writer) int) int {
	v, err := set.Resolve(cli.Inputs{})
	if err != nil {
		fmt.Fprintln(stderr, "refused:", err)
		return deskkit.ExitRefused
	}
	if onResolve != nil {
		onResolve(v)
	}
	raw := v.String("sla-days")
	slaDays, perr := strconv.Atoi(raw)
	if perr != nil || slaDays < 0 {
		fmt.Fprintln(stderr, "refused: --sla-days must be a non-negative integer, got "+strconv.Quote(raw))
		return deskkit.ExitRefused
	}
	toFilter := v.String("to")
	// --to is the desk-inbox filter and is meaningful only on the `issues` lane. Passing
	// it to another subcommand is a caller error with a fix in hand, refused rather than
	// silently ignored (a filter that quietly does nothing reads as an empty inbox).
	if toFilter != "" && sub != "issues" {
		fmt.Fprintln(stderr, "refused: --to <role> filters the `issues` lane only; it has no meaning for `"+sub+"`")
		return deskkit.ExitRefused
	}
	if pre != nil {
		if code := pre(stderr); code != deskkit.ExitOK {
			return code
		}
	}
	// Kill switch FIRST, before any read.
	if err := guard(); err != nil {
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}

	now := time.Now().UTC()
	rep, err := dispatchFn(sub, v.String("root"), now, slaDays, toFilter)
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
func dispatch(sub, root string, now time.Time, slaDays int, toFilter string) (*Report, error) {
	switch sub {
	case "board":
		return cmdBoard(root, now, slaDays)
	case "issues":
		return cmdIssues(root, now, slaDays, toFilter)
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
