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
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const usage = `deskinbox — open issues across the configured repos carrying an
escalation-contract label (urgent, needs-decision, question, help wanted),
sorted urgency-then-age — plus the pipeline flow model (how is the system performing).

usage:
  deskinbox [owner/repo ...]              the terminal table — one row per item.
  deskinbox walk [--item K] [owner/repo ...]
                                           print ONE item in the five-part decision
                                           format (Header/Context/Options/Reply shape/
                                           Verification). Prints item 1 and exits.
  deskinbox html OUT.html [owner/repo ...]
                                           write the whole queue to OUT.html as cards in
                                           that same format, plus the Flow section.
  deskinbox flow [--root PATH ...] [--since YYYY-MM-DD]
                                           the pipeline flow model as a terminal table.
  deskinbox flow --html OUT.html [--root PATH ...] [--since YYYY-MM-DD]
                                           the same model as a self-contained inline-SVG
                                           stage-diagram page.
  deskinbox --version                     source SHA / build time.
  deskinbox -h | --help                   this text.

Repo resolution order (no repo args; table/walk/html): ./.assay/repos.txt, else the
current repo's origin remote.

Cell resolution order (flow): --root PATH (repeatable), else ./.assay/cells.txt, else
the current directory.

Exit codes: 0 every query/reader succeeded · 5 refused (bad arguments) · 6 unverifiable
(at least one read failed — the printed output, if any, is PARTIAL; in html mode a blind
Flow section alone does not redden the exit — see plugins/assay/commands/inbox.md).
`

func main() {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(os.Stderr)
	if !deskkit.CheckVerbActivation(os.Stderr) {
		os.Exit(deskkit.ExitUnverifiable)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, time.Now()))
}

func run(args []string, stdout, stderr io.Writer, now time.Time) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		s, b := deskkit.Version()
		fmt.Fprintf(stdout, "deskinbox sourceSHA=%s builtAt=%s releaseTag=%s\n", s, b, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(stdout, usage)
		return deskkit.ExitOK
	}

	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}

	mode := "table"
	walkItem := 1
	var repoArgs []string
	var htmlOut string

	i := 0
	if len(args) > 0 {
		switch args[0] {
		case "walk":
			mode = "walk"
			i = 1
		case "html":
			mode = "html"
			i = 1
			if i >= len(args) || (len(args[i]) > 0 && args[i][0] == '-' && args[i] != "-h" && args[i] != "--help") {
				fmt.Fprintln(stderr, "deskinbox: html needs an output path")
				return deskkit.ExitRefused
			}
			if i < len(args) && args[i] != "-h" && args[i] != "--help" {
				htmlOut = args[i]
				i++
			}
		case "flow":
			return runFlowCommand(stdout, stderr, args[1:], now)
		}
	}
	for ; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, usage)
			return deskkit.ExitOK
		case a == "--item":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinbox: --item needs a 1-based item number")
				return deskkit.ExitRefused
			}
			n, perr := parsePositiveInt(args[i+1])
			if perr != nil {
				fmt.Fprintf(stderr, "deskinbox: --item must be a positive integer (got %q)\n", args[i+1])
				return deskkit.ExitRefused
			}
			walkItem = n
			mode = "walk"
			i++
		case a == "--walk":
			mode = "walk"
		case len(a) > 0 && a[0] == '-':
			fmt.Fprintf(stderr, "deskinbox: unknown option %q\n", a)
			fmt.Fprint(stderr, usage)
			return deskkit.ExitRefused
		default:
			repoArgs = append(repoArgs, a)
		}
	}

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

// runFlowCommand parses `deskinbox flow`'s own flags (--root, repeatable; --since; --html)
// and dispatches to runFlow (flow.go). It is a separate parse from table/walk/html's because
// flow reads CELLS (a statusgen-root axis), never REPOS — mixing the two flag grammars in one
// loop is how a --root meant for flow would silently do nothing under table/walk, or vice
// versa.
func runFlowCommand(stdout, stderr io.Writer, args []string, now time.Time) int {
	var rootArgs []string
	var since, htmlOut string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-h" || a == "--help":
			fmt.Fprint(stdout, usage)
			return deskkit.ExitOK
		case a == "--root":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinbox: --root needs a path")
				return deskkit.ExitRefused
			}
			rootArgs = append(rootArgs, args[i+1])
			i++
		case a == "--since":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinbox: --since needs a YYYY-MM-DD date")
				return deskkit.ExitRefused
			}
			since = args[i+1]
			i++
		case a == "--html":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "deskinbox: --html needs an output path")
				return deskkit.ExitRefused
			}
			htmlOut = args[i+1]
			i++
		case len(a) > 0 && a[0] == '-':
			fmt.Fprintf(stderr, "deskinbox: unknown option %q\n", a)
			fmt.Fprint(stderr, usage)
			return deskkit.ExitRefused
		default:
			// A bare positional under `flow` names no repo or cell (flow resolves cells from
			// --root/./.assay/cells.txt/"." only) — accepted and ignored, matching the
			// oracle's own ARGS accumulator, which likewise never feeds `--flow`'s cell
			// resolution (assay-inbox.sh:265-292; testdata/spec.md records this divergence
			// point as intentional parity, not an oversight).
		}
	}
	return runFlow(stdout, stderr, rootArgs, since, htmlOut, func() string {
		return now.UTC().Format("2006-01-02T15:04:05Z")
	})
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
