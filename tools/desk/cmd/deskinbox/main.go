// deskinbox — the Go port of assay-inbox.sh, the `assay:inbox` skill's
// engine: open issues across the configured repos carrying an escalation-contract label
// (urgent / needs-decision / question / help wanted), sorted urgency-then-age.
//
// THIS PORT'S SCOPE (split from the authoring brief — see testdata/spec.md for the full
// contract and the reason for the split). Two of the oracle's five renderings are
// implemented here, byte-parity tested against the oracle's own jq program:
//
//	(none)   the terminal table — one row per item.
//	walk     ONE item in the five-part decision format (Header/Context/Options/Reply
//	         shape/Verification) — the `ask-decision` skill's entry point.
//
// --html, --flow and --flow --html are NOT yet ported (a follow-up brief); passing them
// here is refused with a message naming the oracle as the fallback, never a silent
// no-op or a guessed rendering.
//
// No shell-outs: every read reaches the forge through deskkit.ForgeFor (forge.go) or a
// small package-local GitHub REST reader for comment bodies (detail.go) — this package
// forks no subprocess and shells to no external interpreter (Verify row 4).
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
sorted urgency-then-age.

usage:
  deskinbox [owner/repo ...]              the terminal table — one row per item.
  deskinbox walk [--item K] [owner/repo ...]
                                           print ONE item in the five-part decision
                                           format (Header/Context/Options/Reply shape/
                                           Verification). Prints item 1 and exits.
  deskinbox --version                     source SHA / build time.
  deskinbox -h | --help                   this text.

Repo resolution order (no repo args): ./.assay/repos.txt, else the current repo's
origin remote.

NOT YET PORTED in this Go verb (a follow-up brief covers it): --html and --flow.
Use the bash oracle for those: bash plugins/assay/scripts/assay-inbox.sh --html OUT.html
/ --flow.

Exit codes: 0 every query succeeded · 5 refused (bad arguments) · 6 unverifiable
(at least one repo's read failed — the printed output, if any, is PARTIAL).
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

	i := 0
	if len(args) > 0 && args[0] == "walk" {
		mode = "walk"
		i = 1
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
		case a == "--html" || a == "--flow" || a == "--root" || a == "--since":
			fmt.Fprintf(stderr,
				"deskinbox: %s is not yet ported (a follow-up brief covers it) — use "+
					"`bash plugins/assay/scripts/assay-inbox.sh %s ...` instead\n", a, a)
			return deskkit.ExitRefused
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
