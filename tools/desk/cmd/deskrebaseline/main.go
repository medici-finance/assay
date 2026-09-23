// Command deskrebaseline turns a PROVABLY-INTACT-BUT-STALE `## Verify` row into a one-row
// re-baseline PR, instead of the Nth duplicate "stale Verify" issue.
//
// A third of recorded verify runs fail on the same shape: a row pins a path, a count, or a
// tool idiom the tree moved out from under it, so the table fails AS WRITTEN while the work
// it checks is intact. The desk correctly refuses to flip and files another issue, and the
// brief re-fails every drain. deskrebaseline classifies the failing row against a NARROW
// safe set and, only when it can PROVE the work intact from git history, opens a draft PR
// that re-baselines that one row. Everything it cannot prove intact it REFUSES — the verb
// files nothing green on its own authority; the re-baseline is a PR, reviewed by the
// reviewer App and merged by a human (brief verify-integrity/05).
//
// The safe/refusal boundary is the single point of failure: a loose "safe" launders a real
// regression into a green. It therefore fails CLOSED (classify.go) — a row reaches the safe
// set only by a positive proof, and everything else is filed. In this first cut the ONE safe
// class the shipped verb produces is safe:rename (a single rename hop to an existing file).
// safe:count (a pure-additions count drift) and safe:idiom (a tool idiom retired by a
// recorded ruling) are NOT YET IMPLEMENTED: the classifier carries their arms, but
// gatherRowFacts (facts.go) does not populate their facts, so such a row falls through to
// refused:unclassified and is filed as today (see docs/rebaseline.md).
//
// USAGE:
//
//	deskrebaseline <brief> --row K [--root DIR] [--repo owner/name] [--open]
//	deskrebaseline --version
//
// <brief>   a brief file path, or a `<stream>/<NN>` id resolved under --root first, then
//           under the configured stream root for --repo.
// --row K   the 1-based row number in the brief's `## Verify` table to classify.
// --root    the checkout root the row's paths and commands resolve against
//           (default: `git rev-parse --show-toplevel` from the working directory).
// --repo    owner/name, used only as the fallback to resolve a `<stream>/<NN>` brief id under its
//           configured stream root (default: derived from --root's origin remote).
// --open    push the one-row `rebaseline/<stream>-<NN>-row-<K>` branch and open a DRAFT PR
//           via `deskpr create` (as the loop identity — the verifier App under
//           DESK_LOOP=verify-desk). WITHOUT --open the verb is DRY-RUN: it prints the
//           classification, the git evidence and the plan, creates no branch, and exits 0.
//           Before any mutation --open refuses unless the brief resolves inside --root,
//           HEAD is the fetched refs/remotes/origin/main, and the checkout is clean
//           (open.go openPreflight). The commit names only the brief's path.
//
// EXIT CODES (deskkit contract):
//
//	dry-run (no --open) — always exit 0: a classification report is not a refusal. Read the
//	    printed `verdict:` line; a refused:* verdict means "file the issue as today".
//	--open on a safe:* verdict — 0 when the PR opened (deskpr's own exit otherwise).
//	--open on a refused:* verdict — 5 (refused): no branch, no PR; file the issue.
//	other — 5 refused (bad flags / no such row), 6 unverifiable (brief unreadable).
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var (
	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr
)

const usage = `deskrebaseline — re-baseline a provably-intact-but-stale Verify row via a one-row PR.

USAGE:
  deskrebaseline <brief> --row K [--root DIR] [--repo owner/name] [--open]
  deskrebaseline --version

<brief>   brief file path, or a <stream>/<NN> id (resolved under --root, then --repo's root).
--row K   1-based row number in the brief's ## Verify table.
--root    checkout root the row resolves against (default: git toplevel of the cwd).
--repo    owner/name for <stream>/<NN> resolution (default: derived from origin).
--open    push rebaseline/<stream>-<NN>-row-K and open a DRAFT PR via deskpr create
          (verifier App under DESK_LOOP=verify-desk). Without --open: DRY-RUN — print the
          classification and git evidence, create no branch, exit 0. --open refuses
          before any write unless the brief is inside --root, HEAD is the fetched
          refs/remotes/origin/main, and the checkout is clean.

The verb never merges and never lands on main; the re-baseline PR is reviewed and merged
by a human. It fails CLOSED: a row is re-baselined only when git PROVES the work intact.`

func main() { os.Exit(run(os.Args[1:])) }

func run(args []string) int {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Fprintf(stdout, "deskrebaseline sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, usage)
		if len(args) == 0 {
			return deskkit.ExitRefused
		}
		return deskkit.ExitOK
	}

	deskkit.WarnIfUnpinned(stderr)

	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "deskrebaseline: %v\n\n%s\n", err, usage)
		return deskkit.ExitRefused
	}

	root := opts.root
	if root == "" {
		if tl, ok := gitOutput(".", "rev-parse", "--show-toplevel"); ok {
			root = tl
		} else {
			fmt.Fprintln(stderr, "deskrebaseline: --root not given and the working directory is not a git checkout")
			return deskkit.ExitRefused
		}
	}
	repo := opts.repo
	if repo == "" {
		repo = deriveRepo(root)
	}

	lb, err := loadBrief(repo, root, opts.brief)
	if err != nil {
		fmt.Fprintf(stderr, "deskrebaseline: %v\n", err)
		return deskkit.ExitUnverifiable
	}
	rowK, ok := lb.row(opts.row)
	if !ok {
		fmt.Fprintf(stderr, "deskrebaseline: brief %s has no Verify row %d\n", lb.Path, opts.row)
		return deskkit.ExitRefused
	}

	facts := gatherRowFacts(root, rowK, lb.RiskBearing, lb.RiskReason)
	cls := Classify(facts)

	printPlan(stdout, lb, rowK, facts, cls, opts)

	if !opts.open {
		// DRY-RUN: a classification report, never a refusal. Exit 0; no branch created.
		return deskkit.ExitOK
	}

	if !cls.Verdict.IsSafe() {
		fmt.Fprintf(stderr, "deskrebaseline: %s — not re-baselineable; file the issue as today (%s)\n", cls.Verdict, cls.Reason)
		return deskkit.ExitRefused
	}
	return openRebaselinePR(root, repo, lb, rowK, facts, cls)
}

type options struct {
	brief string
	row   int
	root  string
	repo  string
	open  bool
}

func parseArgs(args []string) (options, error) {
	var o options
	haveRow := false
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--open":
			o.open = true
		case a == "--row":
			i++
			if i >= len(args) {
				return o, fmt.Errorf("--row needs a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return o, fmt.Errorf("--row must be a positive integer, got %q", args[i])
			}
			o.row, haveRow = n, true
		case strings.HasPrefix(a, "--row="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--row="))
			if err != nil || n < 1 {
				return o, fmt.Errorf("--row must be a positive integer")
			}
			o.row, haveRow = n, true
		case a == "--root":
			i++
			if i >= len(args) {
				return o, fmt.Errorf("--root needs a value")
			}
			o.root = args[i]
		case strings.HasPrefix(a, "--root="):
			o.root = strings.TrimPrefix(a, "--root=")
		case a == "--repo":
			i++
			if i >= len(args) {
				return o, fmt.Errorf("--repo needs a value")
			}
			o.repo = args[i]
		case strings.HasPrefix(a, "--repo="):
			o.repo = strings.TrimPrefix(a, "--repo=")
		case strings.HasPrefix(a, "-"):
			return o, fmt.Errorf("unknown flag %q", a)
		default:
			if o.brief != "" {
				return o, fmt.Errorf("unexpected extra argument %q (one <brief> only)", a)
			}
			o.brief = a
		}
		i++
	}
	if o.brief == "" {
		return o, fmt.Errorf("a <brief> (path or <stream>/<NN>) is required")
	}
	if !haveRow {
		return o, fmt.Errorf("--row K is required")
	}
	return o, nil
}

// deriveRepo reads owner/name from the checkout's origin remote, offline. "" when it cannot.
func deriveRepo(root string) string {
	url, ok := gitOutput(root, "-C", root, "remote", "get-url", "origin")
	if !ok || url == "" {
		return ""
	}
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")
	if i := strings.Index(url, "github.com"); i >= 0 {
		rest := url[i+len("github.com"):]
		rest = strings.TrimLeft(rest, ":/")
		if strings.Count(rest, "/") == 1 {
			return rest
		}
	}
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return ""
}

// printPlan prints the classification, the intactness evidence and the re-baseline plan —
// the dry-run output, and the record a --open run echoes before it acts.
func printPlan(w io.Writer, lb loadedBrief, row verifyRow, f RowFacts, cls Classification, opts options) {
	fmt.Fprintf(w, "brief: %s\n", lb.Path)
	fmt.Fprintf(w, "row: %d (class %s)\n", row.Num, row.Class)
	fmt.Fprintf(w, "command: %s\n", row.Command)
	if row.Expect != "" {
		fmt.Fprintf(w, "expect: %s\n", row.Expect)
	}
	fmt.Fprintf(w, "verdict: %s\n", cls.Verdict)
	fmt.Fprintf(w, "reason: %s\n", cls.Reason)
	if f.PathRef != "" {
		fmt.Fprintf(w, "evidence: pinned-path=%s exists=%t", f.PathRef, f.PathExists)
		if f.RenameHop != "" {
			fmt.Fprintf(w, " rename-hop=%s", f.RenameHop)
		}
		fmt.Fprintln(w)
	}
	if f.CommandProbed {
		fmt.Fprintf(w, "evidence: command-probed ran=%t result-differs=%t\n", f.CommandRan, f.RCDiffers)
	}
	if cls.Verdict.IsSafe() {
		branch := rebaselineBranch(lb.Path, row.Num)
		if opts.open {
			fmt.Fprintf(w, "plan: open draft PR on branch %s (one row re-baselined)\n", branch)
		} else {
			fmt.Fprintf(w, "plan: --open would push branch %s and open a draft PR (dry-run: no branch created)\n", branch)
		}
	} else {
		fmt.Fprintf(w, "plan: file the issue as today with reason %q\n", cls.Verdict)
	}
}
