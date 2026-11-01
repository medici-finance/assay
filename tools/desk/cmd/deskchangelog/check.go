package main

// The pull-request changelog gate, ported from tools/changelog/check.sh (now an adapter onto
// `deskchangelog check`). Every message, rule order and exit code is the script's.
//
// THE CONVENTION IT ENFORCES. A notable change records itself by ADDING one fragment file
// under changelog/ (changelog/<slug>.md); a genuinely non-notable PR carries the
// changelog:skip label instead. Hand-editing CHANGELOG.md's `## Unreleased` section is
// REFUSED: CHANGELOG.md is written only by the release workflow.
//
// Outcomes, decided from the diff and trusted PR metadata:
//   REFUSE — the PR adds a bullet under `## Unreleased` (rule 1, absolute, first), or ADDS a
//            fragment below the top level of changelog/ (rule 1b, the flat rule).
//   PASS   — the board-reconciliation exception holds (when --reconcile names its checker),
//            the changelog:skip label is present (rule 2), the PR adds/updates a top-level
//            fragment with a highlight bullet (rule 3), or a proxy fragment
//            changelog/pr-<PR_NUMBER>-<slug>.md with a bullet sits on the LIVE base-branch
//            tip (rule 4, the fork-PR path).
//   FAIL   — none of the above.
//
// It reads git history only; it contacts no network.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/medici-finance/assay/tools/desk/internal/cli"
)

var (
	nestedFragRe = regexp.MustCompile(`^changelog/.+/.+\.md$`)
	flatFragRe   = regexp.MustCompile(`^changelog/[^/]+\.md$`)
	prNumberRe   = regexp.MustCompile(`^[1-9][0-9]*$`)
	// highlightRe is grep -E '^[[:space:]]*-[[:space:]]+[^[:space:]]', per line.
	highlightRe = regexp.MustCompile(`(?m)^[ \t\v\f\r]*-[ \t\v\f\r]+[^ \t\v\f\r\n]`)
)

const (
	msgUnreleased = "::error title=Unreleased edit refused::This PR adds highlight line(s) under '## Unreleased' in CHANGELOG.md. That section is RETIRED — it was a standing merge-conflict source. Record the change as a fragment file instead: changelog/<slug>.md (one-to-few highlight lines; see changelog/README.md). CHANGELOG.md is written only by the release workflow, which aggregates the fragments at cut time."
	msgNested     = "::error title=nested changelog fragment refused::Changelog fragments must sit DIRECTLY under changelog/ (changelog/<slug>.md) — never in a subdirectory. The release aggregator reads only the top level, so a nested fragment would pass this gate and then be silently dropped from the release notes. Move it to the top level, e.g. git mv %s %s (a branch named feat/x still gets changelog/x.md: use the branch BASENAME, never the branch path). See changelog/README.md."
	msgSkip       = "::notice title=changelog:skip::PR carries the 'changelog:skip' label — a changelog fragment is not required for this PR."
	msgEmpty      = "::error title=empty changelog fragment::This PR adds/updates changelog fragment file(s) but none carries a highlight bullet. An empty, whitespace-only, or bullet-less fragment records nothing and is rejected — the gate is not satisfiable by 'touch changelog/x.md'. Put at least one '- …' highlight line in the fragment (see changelog/README.md), or label the PR 'changelog:skip' if the change is genuinely non-notable."
	msgDegraded   = "::notice title=proxy lookup degraded::Neither refs/remotes/origin/${BASE_REF} (BASE_REF, or its zero-config default GITHUB_BASE_REF) nor refs/remotes/origin/HEAD (not expected to resolve under actions/checkout) resolved, so the proxy-fragment lookup is running against the RECORDED base sha (%s) instead of the live base-branch tip. That sha is frozen at PR-open time, so a proxy fragment merged after this PR was opened will not be seen. Fix: ensure the workflow runs on pull_request (GITHUB_BASE_REF is then set automatically), or pass BASE_REF explicitly."
	msgProxyEmpty = "::error title=empty changelog fragment::The proxy fragment(s) for this PR on the base branch carry no highlight bullet. An empty, whitespace-only, or bullet-less fragment records nothing and is rejected — the gate is not satisfiable by 'touch changelog/x.md'. Put at least one '- …' highlight line in the proxy fragment (see changelog/README.md), or label the PR 'changelog:skip' if the change is genuinely non-notable."
	msgMissing    = "::error title=missing changelog fragment::This PR adds no changelog fragment and carries no 'changelog:skip' label. Fix: create changelog/%[1]s.md with at least one highlight bullet, e.g. printf '### Fixed\\n- <one line>\\n' > changelog/%[1]s.md then commit and push. The ### Added / ### Fixed / ### Changed bucket heading is optional; a bullet is not — an empty, whitespace-only, or bullet-less fragment is rejected. NEVER edit CHANGELOG.md (it is written only by the release workflow, which aggregates the fragments). The 'changelog:skip' waiver is a maintainer act, not one you self-apply. For a PR whose branch maintainers cannot commit to (a fork), a maintainer may instead land changelog/pr-<N>-<slug>.md on the base branch, then re-run this check by adding or removing a label — see changelog/README.md."
)

// checkBindings are the gate's inputs: the environment the workflow sets, and the path of
// the board-reconciliation checker.
func checkBindings() []cli.Binding {
	env := func(key, name, usage string) cli.Binding {
		return cli.Binding{Key: key, Kind: cli.String, Env: []string{name}, Usage: usage}
	}
	return []cli.Binding{
		env("skip", "SKIP", `"true" when the PR carries the changelog:skip label`),
		env("base-sha", "BASE_SHA", "the PR base commit as recorded at open (required)"),
		env("head-sha", "HEAD_SHA", "the PR head commit (required)"),
		env("base-ref", "BASE_REF", "the PR base branch name (explicit override)"),
		env("github-base-ref", "GITHUB_BASE_REF", "the base branch name Actions sets on pull_request runs"),
		env("head-ref", "HEAD_REF", "the PR head branch name (names the suggested fragment)"),
		env("pr-number", "PR_NUMBER", "the PR number (enables the proxy path)"),
		env("changelog-agg", "CHANGELOG_AGG", "an external unreleased-bullets parser (default: this command)"),
		{Key: "reconcile", Kind: cli.String, Flag: "reconcile",
			Usage: "board-reconciliation checker `file`, run before the skip label; exit 0 passes"},
	}
}

// gitRun runs git in the working directory; stdout is returned whatever the exit status.
// stderr goes to errw (nil discards it).
func gitRun(errw io.Writer, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	var out strings.Builder
	cmd.Stdout = &out
	if errw != nil {
		cmd.Stderr = errw
	}
	err := cmd.Run()
	return out.String(), err
}

// nonEmptyLines splits command output into its non-empty lines.
func nonEmptyLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

// diffPaths is `git diff ... | awk '{print $2}'` filtered by re.
func diffPaths(stderr io.Writer, re *regexp.Regexp, args ...string) []string {
	out, _ := gitRun(stderr, args...)
	var paths []string
	for _, ln := range strings.Split(out, "\n") {
		f := strings.FieldsFunc(ln, func(r rune) bool { return r == ' ' || r == '\t' })
		if len(f) >= 2 && re.MatchString(f[1]) {
			paths = append(paths, f[1])
		}
	}
	return paths
}

// hasHighlight reports whether any of files, read at tree, carries a highlight bullet.
func hasHighlight(tree string, files []string) bool {
	for _, f := range files {
		blob, _ := gitRun(nil, "show", tree+":"+f)
		if highlightRe.MatchString(blob) {
			return true
		}
	}
	return false
}

type gate struct {
	stdout, stderr io.Writer
	agg            string
}

// unreleasedAt is the `## Unreleased` bullet entries at sha's CHANGELOG.md (none when the
// file is absent there). With an external parser it runs `<agg> unreleased-bullets <tmp>`.
func (g gate) unreleasedAt(sha string) []string {
	blob, err := gitRun(nil, "show", sha+":CHANGELOG.md")
	if err != nil {
		return nil
	}
	if g.agg == "" {
		if !utf8.ValidString(blob) {
			fmt.Fprintf(g.stderr, "deskchangelog: CHANGELOG.md at %s: %v\n", sha, errUndecodable)
			return nil
		}
		return unreleasedEntries(splitLines(blob))
	}
	tmp, err := os.CreateTemp("", "cl-")
	if err != nil {
		return nil
	}
	defer os.Remove(tmp.Name())
	_, werr := tmp.WriteString(blob)
	if cerr := tmp.Close(); werr != nil || cerr != nil {
		return nil
	}
	cmd := exec.Command("python3", g.agg, "unreleased-bullets", tmp.Name())
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, g.stderr
	_ = cmd.Run()
	return nonEmptyLines(out.String())
}

func sortedSet(xs []string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		if x != "" {
			m[x] = true
		}
	}
	return m
}

func (g gate) run(v *cli.Values) int {
	baseSHA, headSHA := v.String("base-sha"), v.String("head-sha")
	for _, req := range []struct{ name, val string }{{"BASE_SHA", baseSHA}, {"HEAD_SHA", headSHA}} {
		if req.val == "" {
			fmt.Fprintf(g.stderr, "%s: %s is required\n", req.name, req.name)
			return 1
		}
	}
	baseRef := v.String("base-ref")
	if baseRef == "" {
		baseRef = v.String("github-base-ref")
	}
	slug := "<slug>"
	if h := v.String("head-ref"); h != "" {
		slug = path.Base(h)
	}
	out := g.stdout

	// 1) DEPRECATION GUARD: any `## Unreleased` bullet present at head but not at base.
	base := sortedSet(g.unreleasedAt(baseSHA))
	var added []string
	for e := range sortedSet(g.unreleasedAt(headSHA)) {
		if !base[e] {
			added = append(added, e)
		}
	}
	if len(added) > 0 {
		sort.Strings(added)
		fmt.Fprintln(out, msgUnreleased)
		fmt.Fprintln(out, "Refused — added under '## Unreleased':")
		fmt.Fprintln(out, strings.Join(added, "\n"))
		return 1
	}

	// 1b) FLAT-FRAGMENT GUARD: the PR may not ADD a fragment below changelog/'s top level.
	rng := baseSHA + "..." + headSHA
	if nested := diffPaths(g.stderr, nestedFragRe, "diff", "--no-renames", "--name-status", "--diff-filter=A", rng, "--", "changelog/"); len(nested) > 0 {
		hint := "changelog/" + slug + ".md"
		if slug == "<slug>" {
			hint = "changelog/" + path.Base(nested[0])
		}
		fmt.Fprintf(out, msgNested+"\n", nested[0], hint)
		fmt.Fprintln(out, "Refused — nested fragment path(s) (flat rule: changelog/<slug>.md):")
		fmt.Fprintln(out, strings.Join(nested, "\n"))
		return 1
	}

	// Board reconciliation records existing delivery state, not a release highlight. Its
	// checker reads the runner's event metadata and the immutable diff itself; isolated
	// Python keeps the PR checkout and PYTHONPATH out of module lookup.
	if rec := v.String("reconcile"); rec != "" {
		cmd := exec.Command("python3", "-I", "-c", `import runpy, sys; sys.exit(runpy.run_path(sys.argv[1])["check"]())`, rec)
		cmd.Stdout, cmd.Stderr = out, g.stderr
		if cmd.Run() == nil {
			return 0
		}
	}

	// 2) SKIP label.
	if v.String("skip") == "true" {
		fmt.Fprintln(out, msgSkip)
		fmt.Fprintln(out, "SKIP: changelog:skip label present — fragment requirement waived for this PR.")
		return 0
	}

	// 3) A top-level fragment added/updated, carrying a highlight bullet read from HEAD.
	var frag []string
	for _, f := range diffPaths(g.stderr, flatFragRe, "diff", "--name-status", "--diff-filter=AM", rng, "--", "changelog/") {
		if f != "changelog/README.md" {
			frag = append(frag, f)
		}
	}
	if len(frag) > 0 {
		if hasHighlight(headSHA, frag) {
			fmt.Fprintln(out, "PASS: this PR adds/updates changelog fragment(s) with content:")
			fmt.Fprintln(out, strings.Join(frag, "\n"))
			return 0
		}
		fmt.Fprintln(out, msgEmpty)
		fmt.Fprintf(out, "fragment file(s) with no highlight bullet:\n%s\n", strings.Join(frag, "\n"))
		return 1
	}

	// 4) PROXY FRAGMENT on the LIVE base-branch tip (never BASE_SHA, which is frozen at
	// PR-open time and so cannot contain a proxy landed afterwards).
	if pr := v.String("pr-number"); pr != "" && prNumberRe.MatchString(pr) {
		var tree string
		if _, err := gitRun(nil, "rev-parse", "--verify", "-q", "refs/remotes/origin/"+baseRef+"^{commit}"); baseRef != "" && err == nil {
			tree = "refs/remotes/origin/" + baseRef
		} else if _, err := gitRun(nil, "rev-parse", "--verify", "-q", "refs/remotes/origin/HEAD^{commit}"); err == nil {
			tree = "refs/remotes/origin/HEAD"
		} else {
			tree = baseSHA
			fmt.Fprintf(out, msgDegraded+"\n", baseSHA)
			fmt.Fprintf(out, "NOTICE: proxy lookup degraded — using recorded BASE_SHA %s, not the live base-branch tip.\n", baseSHA)
		}
		short, err := gitRun(nil, "rev-parse", "--short", tree)
		if err != nil {
			short += tree
		}
		short = strings.TrimRight(short, "\n")

		// DEFENSE IN DEPTH: PR_NUMBER is interpolated into a pattern below; re-assert that it
		// carries no newline right where it is used.
		if strings.Contains(pr, "\n") {
			fmt.Fprintln(g.stderr, "::error title=invalid PR_NUMBER::PR_NUMBER contains a newline; refusing the proxy-fragment lookup.")
			return 1
		}
		proxyRe := regexp.MustCompile(`^changelog/pr-` + regexp.QuoteMeta(pr) + `-[A-Za-z0-9._-]+\.md$`)
		listing, _ := gitRun(nil, "ls-tree", "-r", "--name-only", tree, "--", "changelog/")
		var proxy []string
		for _, ln := range strings.Split(listing, "\n") {
			if proxyRe.MatchString(ln) {
				proxy = append(proxy, ln)
			}
		}
		if len(proxy) > 0 {
			if hasHighlight(tree, proxy) {
				fmt.Fprintf(out, "PASS: proxy fragment(s) for PR #%s found on the base branch tip %s:\n", pr, short)
				fmt.Fprintln(out, strings.Join(proxy, "\n"))
				return 0
			}
			fmt.Fprintln(out, msgProxyEmpty)
			fmt.Fprintf(out, "proxy fragment(s) with no highlight bullet:\n%s\n", strings.Join(proxy, "\n"))
			return 1
		}
	}

	fmt.Fprintf(out, msgMissing+"\n", slug)
	fmt.Fprintf(out, "MISSING: no changelog/<slug>.md fragment between base %s and head %s — suggested path: changelog/%s.md\n", baseSHA, headSHA, slug)
	return 1
}

func checkCommand(stdout, stderr io.Writer) *cobra.Command {
	c := &cobra.Command{
		Use:   "check",
		Short: "the pull-request changelog gate",
		Long: `The pull-request changelog gate: refuses a new '## Unreleased' bullet or a nested
fragment, then passes on the board-reconciliation exception (--reconcile), the
changelog:skip label, an added/updated top-level fragment with a highlight bullet,
or a proxy fragment changelog/pr-<PR_NUMBER>-<slug>.md on the live base-branch tip.
Run from the repository checkout. Reads git history only.

Environment: SKIP, BASE_SHA (required), HEAD_SHA (required), BASE_REF, falling
back to GITHUB_BASE_REF, HEAD_REF, PR_NUMBER, CHANGELOG_AGG.

Exit codes: 0 pass · 1 refused, missing, or a required input unset · 2 usage.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usageErr(cmd)
			}
			return nil
		},
	}
	set := cli.Declare(c, checkBindings()...)
	c.RunE = func(_ *cobra.Command, _ []string) error {
		v, err := resolve(set)
		if err != nil {
			return err
		}
		return codeErr(gate{stdout: stdout, stderr: stderr, agg: v.String("changelog-agg")}.run(v))
	}
	return c
}
