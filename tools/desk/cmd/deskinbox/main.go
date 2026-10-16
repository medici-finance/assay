// deskinbox — the Go port of assay-inbox.sh, the `assay:inbox` skill's
// engine: open issues across the configured repos carrying an escalation-contract label
// (urgent / needs-decision / question / help wanted), sorted urgency-then-age — PLUS the
// pipeline flow model (`flow`), a different question ("how is the system performing")
// derived from other desk binaries rather than a forge read.
//
// ALL FIVE of the oracle's renderings are ported here, byte-parity tested against the
// oracle's own jq programs (format_parity_test.go, flow_parity_test.go, html_parity_test.go):
//
//	(none)          the terminal table — one row per item.
//	walk            ONE item in the five-part decision format (Header/Context/Options/Reply
//	                shape/Verification) — the `ask-decision` skill's entry point.
//	html OUT.html   the whole queue as self-contained HTML cards in that same format, plus
//	                the Flow section.
//	flow            the pipeline flow model as a terminal table.
//	flow --html O   the same model as a self-contained inline-SVG stage-diagram page.
//
// No shell-outs to a forge: every issue/detail read reaches it through deskkit.ForgeFor
// (forge.go) or a small package-local GitHub REST reader for comment bodies (detail.go).
// `flow`'s readers are the ONE exception (flow.go's file header): they run OTHER DESK
// BINARIES (statusgen, deskboard), not a forge, so every exec.Command site in this package
// names one of those two (Verify row 5) — everything else forks no subprocess.
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
const helpLong = `deskinbox — open issues across the configured repos carrying an
escalation-contract label (urgent, needs-decision, question, help wanted),
sorted urgency-then-age — plus the pipeline flow model (how is the system performing).

  deskinbox [owner/repo ...]               the terminal table — one row per item.
  deskinbox walk [--item K] [owner/repo ...]
                                           print ONE item in the five-part decision
                                           format (Header/Context/Options/Reply shape/
                                           Verification). Prints item 1 and exits.
  deskinbox html OUT.html [owner/repo ...] write the whole queue to OUT.html as cards in
                                           that same format, plus the Flow section.
  deskinbox flow [--root PATH ...] [--since YYYY-MM-DD]
                                           the pipeline flow model as a terminal table.
  deskinbox flow --html OUT.html [--root PATH ...] [--since YYYY-MM-DD]
                                           the same model as a self-contained inline-SVG
                                           stage-diagram page.

--item K (or --walk) selects walk from the table or html forms too.

Repo resolution order (no repo args; table/walk/html): ./.assay/repos.txt, else the
current repo's origin remote.

Cell resolution order (flow): --root PATH (repeatable), else ./.assay/cells.txt, else
the current directory.

Exit codes: 0 every query/reader succeeded · 5 refused (bad arguments) · 6 unverifiable
(at least one read failed — the printed output, if any, is PARTIAL; in html mode a blind
Flow section alone does not redden the exit — see plugins/assay/commands/inbox.md).`

func main() {
	os.Exit(deskkit.PerfRun("deskinbox", nil, os.Args[1:], func([]string) int {
		return execute(os.Args[1:], os.Stdout, os.Stderr, time.Now(), startup)
	}))
}

// startup is what every real run does first, after its command line parsed and its values
// passed their checks, and never for help or version (desktools-v2/45: help reads nothing).
func startup(stderr io.Writer) int {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(stderr)
	if !deskkit.CheckVerbActivation(stderr) {
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}

// run is execute without the process startup: the in-package tests' entry, as before.
func run(args []string, stdout, stderr io.Writer, now time.Time) int {
	return execute(args, stdout, stderr, now, nil)
}

// guard is the kill switch, a variable so the contract tests can count it.
var guard = deskkit.Guard

// queueFn and flowFn are the typed handlers the command tree calls; the contract tests replace
// them to observe the values that reach them.
var (
	queueFn = runQueue
	flowFn  = runFlow
)

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
	return fmt.Sprintf("deskinbox sourceSHA=%s builtAt=%s releaseTag=%s", s, b, deskkit.ReleaseTagOrDev())
}

// queueBindings are the settings of the table, walk and html forms; flowBindings are flow's.
// Flags and defaults only: deskinbox has no configuration file and reads no environment
// variable for them. The two grammars stay apart, as the hand-rolled loops kept them: a --root
// meant for flow is refused under table/walk/html, and --item under flow.
func queueBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "item", Kind: cli.String, Flag: "item", Usage: "walk the `K`th item (1-based); selects walk"},
		{Key: "walk", Kind: cli.Bool, Flag: "walk", Usage: "walk mode (item 1 unless --item)"},
	}
}

func flowBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "root", Kind: cli.StringArray, Flag: "root", Usage: "a cell root to read (`PATH`, repeatable)"},
		{Key: "since", Kind: cli.String, Flag: "since", Usage: "count from this `YYYY-MM-DD` date"},
		{Key: "html", Kind: cli.String, Flag: "html", Usage: "write the model as a self-contained inline-SVG page to `OUT.html`"},
	}
}

// execute runs one invocation against a fresh command tree. pre is the process startup (nil in
// the in-package tests, as run never did it).
func execute(args []string, stdout, stderr io.Writer, now time.Time, pre func(io.Writer) int) int {
	build := func() *cobra.Command {
		root := cli.NewRoot("deskinbox [owner/repo ...]", "the escalation-label inbox and the pipeline flow model")
		root.Long = helpLong
		root.Args = cobra.ArbitraryArgs
		rootSet := cli.Declare(root, queueBindings()...)
		root.RunE = func(_ *cobra.Command, args []string) error {
			return codeErr(queueCommand("table", "", args, rootSet, stdout, stderr, now, pre))
		}

		walk := &cobra.Command{Use: "walk [--item K] [owner/repo ...]", Short: "print ONE item in the five-part decision format",
			Args: cobra.ArbitraryArgs}
		walkSet := cli.Declare(walk, queueBindings()...)
		walk.RunE = func(_ *cobra.Command, args []string) error {
			return codeErr(queueCommand("walk", "", args, walkSet, stdout, stderr, now, pre))
		}

		html := &cobra.Command{Use: "html OUT.html [owner/repo ...]", Short: "write the whole queue to OUT.html as decision cards plus the Flow section",
			Args: func(_ *cobra.Command, args []string) error {
				if len(args) == 0 {
					return fmt.Errorf("html needs an output path")
				}
				return nil
			}}
		htmlSet := cli.Declare(html, queueBindings()...)
		html.RunE = func(_ *cobra.Command, args []string) error {
			return codeErr(queueCommand("html", args[0], args[1:], htmlSet, stdout, stderr, now, pre))
		}

		// A bare word under flow names no repo or cell (flow resolves cells from
		// --root/./.assay/cells.txt/"." only) — accepted and ignored, matching the
		// oracle's own ARGS accumulator, which likewise never feeds --flow's cell
		// resolution (assay-inbox.sh:265-292; testdata/spec.md records this divergence
		// point as intentional parity, not an oversight).
		flow := &cobra.Command{Use: "flow [--root PATH ...] [--since YYYY-MM-DD] [--html OUT.html]",
			Short: "the pipeline flow model (a terminal table, or --html a stage-diagram page)", Args: cobra.ArbitraryArgs}
		flowSet := cli.Declare(flow, flowBindings()...)
		flow.RunE = func(*cobra.Command, []string) error {
			return codeErr(flowCommand(flowSet, stdout, stderr, now, pre))
		}

		version := &cobra.Command{Use: "version", Short: "print the version", Hidden: true, Args: cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				fmt.Fprintln(stdout, versionLine())
				return nil
			}}
		root.AddCommand(walk, html, flow, version)
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

// begin runs the process startup and the kill switch, after the values passed their checks.
func begin(stderr io.Writer, pre func(io.Writer) int) int {
	if pre != nil {
		if code := pre(stderr); code != deskkit.ExitOK {
			return code
		}
	}
	if err := guard(); err != nil {
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}
	return deskkit.ExitOK
}

// queueCommand settles the table/walk/html mode from the parsed values: --item and --walk
// select walk from any of the three forms, as the hand-rolled loop did.
func queueCommand(mode, htmlOut string, repoArgs []string, set *cli.Set, stdout, stderr io.Writer, now time.Time, pre func(io.Writer) int) int {
	v, err := set.Resolve(cli.Inputs{})
	if err != nil {
		fmt.Fprintln(stderr, "deskinbox:", err)
		return deskkit.ExitRefused
	}
	if onResolve != nil {
		onResolve(v)
	}
	walkItem := 1
	if v.IsSet("item") {
		raw := v.String("item")
		n, perr := parsePositiveInt(raw)
		if perr != nil {
			fmt.Fprintf(stderr, "deskinbox: --item must be a positive integer (got %q)\n", raw)
			return deskkit.ExitRefused
		}
		walkItem, mode = n, "walk"
	}
	if v.Bool("walk") {
		mode = "walk"
	}
	if code := begin(stderr, pre); code != deskkit.ExitOK {
		return code
	}
	return queueFn(stdout, stderr, mode, htmlOut, repoArgs, walkItem, now)
}

// flowCommand passes flow's values to runFlow (flow.go). flow reads CELLS (a statusgen-root
// axis), never REPOS.
func flowCommand(set *cli.Set, stdout, stderr io.Writer, now time.Time, pre func(io.Writer) int) int {
	v, err := set.Resolve(cli.Inputs{})
	if err != nil {
		fmt.Fprintln(stderr, "deskinbox:", err)
		return deskkit.ExitRefused
	}
	if onResolve != nil {
		onResolve(v)
	}
	since := v.String("since")
	if since != "" && !flowSinceRe.MatchString(since) {
		fmt.Fprintf(stderr, "deskinbox: --since must be YYYY-MM-DD (got %q)\n", since)
		return deskkit.ExitRefused
	}
	if code := begin(stderr, pre); code != deskkit.ExitOK {
		return code
	}
	return flowFn(stdout, stderr, v.Strings("root"), since, v.String("html"), func() string {
		return now.UTC().Format("2006-01-02T15:04:05Z")
	})
}

// runQueue resolves the repo set and runs the table, walk or html rendering.
func runQueue(stdout, stderr io.Writer, mode, htmlOut string, repoArgs []string, walkItem int, now time.Time) int {
	cwd, cerr := os.Getwd()
	if cerr != nil {
		fmt.Fprintln(stderr, deskkit.Unverifiable("cannot resolve the current directory", cerr))
		return deskkit.ExitUnverifiable
	}
	repos, rerr := resolveRepos(repoArgs, cwd)
	if rerr != nil {
		fmt.Fprintln(stderr, rerr)
		return deskkit.ExitCodeOf(rerr)
	}
	if len(repos) == 0 {
		fmt.Fprintln(stderr, "deskinbox: no repos to query (no repo args, no ./.assay/repos.txt, and no git origin remote found)")
		return deskkit.ExitRefused
	}

	switch mode {
	case "walk":
		return runWalk(stdout, stderr, repos, walkItem, now)
	case "html":
		return runHTML(stdout, stderr, htmlOut, repos, now)
	default:
		return runTable(stdout, stderr, repos, now)
	}
}

func parsePositiveInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a digit: %q", s)
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return n, nil
}

func runTable(stdout, stderr io.Writer, repos []string, now time.Time) int {
	if err := validateRepos(repos); err != nil {
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}
	items, failures := fetchQueue(repos)
	for _, f := range failures {
		fmt.Fprintf(stderr, "deskinbox: QUERY FAILED for %s: %v\n", f.Repo, f.Err)
	}
	renderTable(stdout, items, now)
	fmt.Fprintln(stdout, summaryText(len(items), len(repos), failures))
	if len(failures) > 0 {
		return deskkit.ExitUnverifiable
	}
	return deskkit.ExitOK
}
