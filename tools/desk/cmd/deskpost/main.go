// Command deskpost posts the reviewer App's verdict, plain comments, and the
// draft→ready flip AS the configured reviewer App — the unforgeable
// review-gate identity. It centralises the App-token mint and encodes the review-
// gate policy in code:
//
//   - reviews are posted BY THE APP so a PR author cannot self-approve;
//   - the TRUST GATE (deskkit/trust.go) refuses every verb (exit 5, audited) on a PR
//     authored outside the trusted desk identities unless the blessing authority's comment blesses it;
//   - a review must be head-pinned (--head, the FULL SHA — an abbreviated one is a
//     form error, not a head mismatch: see headFormError) — a verdict never lands on
//     unreviewed code;
//   - the ready flip happens ONLY when the App has APPROVED at the CURRENT head, CI is
//     green, and (for risk-classed PRs) a Security-Review: pass verdict exists at head;
//   - the security verdict is its own verb and its own REVIEW (`security-review`), because
//     the flip gate reads reviews and never comments — a comment-shaped pass is invisible
//     to it (#513 / #438);
//   - ready = "ready for HUMAN review"; merge is always the owner's.
//
// The tool has NO other verbs — no merge, close, un-ready, edit, or label.
// Every invocation runs the deskkit kill switch first, audits exactly one line,
// and fails closed on anything it cannot positively verify.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (an earlier correctness review found SetToolClass had no caller anywhere
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
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	deskkit.WarnIfUnpinned(stderr)

	if len(argv) == 0 {
		usage()
		return 2
	}

	switch argv[0] {
	case "version", "--version", "-v":
		s, b := deskkit.Version()
		fmt.Fprintf(stdout, "deskpost sourceSHA=%s builtAt=%s releaseTag=%s\n", s, b, deskkit.ReleaseTagOrDev())
		return 0
	case "help", "-h", "--help":
		usage()
		return 0
	}

	// Kill switch FIRST — before parsing args or touching the network. A disabled
	// suite exits 3 after Guard audits result=disabled.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, "deskpost: "+err.Error())
		return deskkit.ExitCodeOf(err)
	}

	// Outward verbs present a LOOP IDENTITY. The kill switch's per-loop halt is
	// `STOP.<loop>`, matched against $DESK_LOOP; with the variable unset nothing matches,
	// so a stop flag a human is holding never fires and this verb keeps writing while the
	// operator believes it has been halted. The boot verb has checked this since it was
	// written — an outward verb run OUTSIDE a booted window did not, which is the gap.
	if err := deskkit.RequireLoopIdentity("deskpost"); err != nil {
		fmt.Fprintln(stderr, "deskpost: "+err.Error())
		return deskkit.ExitCodeOf(err)
	}

	switch argv[0] {
	case "review":
		return cmdReview(argv)
	case "security-review":
		return cmdSecurityReview(argv)
	case "comment":
		return cmdComment(argv)
	case "ready":
		return cmdReady(argv)
	default:
		fmt.Fprintln(stderr, "deskpost: unknown subcommand "+argv[0])
		usage()
		return 2
	}
}

func cmdReview(argv []string) int {
	a, code, ok := parseVerdictArgs("review", "approve|request-changes", argv)
	if !ok {
		return code
	}
	code = runReview(a.owner, a.name, a.pr, a.verdict, a.head, a.body, argv, a.opts)
	if code == deskkit.ExitRateLimited {
		fmt.Fprint(stderr, fallbackRecipe(a.owner, a.name, a.pr, a.verdict, a.head, a.bodyFile))
	}
	return code
}

// cmdSecurityReview parses the security verdict verb (#513 /
// #438). Identical argument shape to `review` — the DIFFERENCE is entirely in what gets
// submitted (runSecurityReview), which is why the parsing is shared rather than copied.
//
// The exit-4 fallback recipe is deliberately NOT printed here. The recipe tells the caller
// to re-post through a raw `gh pr review --approve|--request-changes`, and neither is this
// verb's shape: a clean pass is a COMMENT-event review, so `--approve` would land the very
// artifact this fix exists to stop the desk from having to post. The correct move on exit 4
// is `--wait`, which the general rate-limit message already names.
func cmdSecurityReview(argv []string) int {
	a, code, ok := parseVerdictArgs("security-review", "pass|fail", argv)
	if !ok {
		return code
	}
	return runSecurityReview(a.owner, a.name, a.pr, a.verdict, a.head, a.body, argv, a.opts)
}

// verdictArgs is one parsed invocation of a verdict verb.
type verdictArgs struct {
	owner, name string
	pr          int
	verdict     string
	head        string
	bodyFile    string
	body        []byte
	opts        postOpts
}

// parseVerdictArgs parses the argument shape both verdict verbs share:
//
//	deskpost <verb> <owner/repo> <pr> --verdict <values> --head <full-40-char-sha> --body-file F
//
// It returns ok=false with the exit code to use. --head is REQUIRED and must be the FULL
// 40-char SHA on both verbs — an abbreviated one is a form error, not a head mismatch
// (#214): the two call for opposite responses, and a verdict that lands on a commit the
// reviewer did not read is the failure the flag exists to prevent (#197).
func parseVerdictArgs(verb, verdictValues string, argv []string) (verdictArgs, int, bool) {
	var a verdictArgs
	rest := argv[1:]
	if len(rest) < 2 {
		fmt.Fprintf(stderr, "usage: deskpost %s <owner/repo> <pr> --verdict %s --head <full-40-char-sha> --body-file F\n",
			verb, verdictValues)
		return a, 2, false
	}
	owner, name, ok := splitRepo(rest[0])
	if !ok {
		fmt.Fprintln(stderr, "deskpost: repo must be owner/name, got "+rest[0])
		return a, 2, false
	}
	pr, ok := parsePR(rest[1])
	if !ok {
		fmt.Fprintln(stderr, "deskpost: pr must be a positive integer, got "+rest[1])
		return a, 2, false
	}
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	verdict := fs.String("verdict", "", verdictValues)
	head := fs.String("head", "", "the reviewed head SHA — FULL 40-char lowercase hex, not abbreviated (required)")
	bodyFile := fs.String("body-file", "", "path to the review body file (required)")
	raw := addPostFlags(fs)
	if err := fs.Parse(rest[2:]); err != nil {
		return a, 2, false
	}
	opts, ok := raw.resolve()
	if !ok {
		return a, 2, false
	}
	if *verdict == "" || *head == "" || *bodyFile == "" {
		fmt.Fprintf(stderr, "deskpost %s: --verdict, --head and --body-file are all required\n", verb)
		return a, 2, false
	}
	if !isFullSHA(*head) {
		fmt.Fprintf(stderr, "deskpost %s: %s\n", verb, headFormError(*head))
		return a, 2, false
	}
	body, err := os.ReadFile(*bodyFile)
	if err != nil {
		fmt.Fprintf(stderr, "deskpost %s: cannot read --body-file: %s\n", verb, err.Error())
		return a, 2, false
	}
	return verdictArgs{owner: owner, name: name, pr: pr, verdict: *verdict, head: *head,
		bodyFile: *bodyFile, body: body, opts: opts}, 0, true
}

// fallbackRecipe is printed when a review is rate-limited (exit 4) — the one exit whose
// documented remedy is "run the raw command manually".
//
// WHY THE TOOL EMITS IT (#197). `deskpost review --head` exists so a verdict
// cannot be misattributed to a commit it did not assess. The sanctioned fallback,
// `gh pr review`, has NO head-pinning flag: it attaches to whatever the head is AT POST
// TIME. Observed on #195 — exit 4, re-posted via the install token, and a
// CHANGES_REQUESTED written against `151ebe99` landed on `2bbf529c`, a commit that had
// already fixed two of its three findings. Both exit 0; nothing in the output signals it.
// A stale CHANGES_REQUESTED blocks a flip on closed findings; a stale APPROVED certifies
// a commit nobody reviewed.
//
// Under a saturated budget the fallback is the COMMON path, not the rare one, so the
// protection is off more often than on. The head assertion below is what puts it back,
// and it belongs in the tool's own output rather than only in skill prose: the skill lives
// in another repo, and an agent that hits exit 4 reads stderr, not the skill.
func fallbackRecipe(owner, name string, pr int, verdict, head, bodyFile string) string {
	return fmt.Sprintf(`
deskpost review: RATE-LIMITED — the verdict was NOT posted.

If you fall back to the raw command, note that `+"`gh pr review`"+` has NO head-pinning flag
(#197): it attaches to whatever the head is at post time, so a verdict formed
against %s can silently land on a commit nobody reviewed. Re-assert the head and ABORT on
mismatch — do not post a stale verdict:

  cur=$(gh api repos/%s/%s/pulls/%d --jq .head.sha)
  [ "$cur" = "%s" ] || { echo "HEAD MOVED ($cur) — re-review at the new head; DO NOT post"; exit 1; }
  GH_TOKEN=... gh pr review %d -R %s/%s --%s --body-file %s

Preferred: sleep the stated retry-after and attempt ONCE more through deskpost, which
pins the head itself.
`, short(head), owner, name, pr, head, pr, owner, name, verdict, bodyFile)
}

// postFlagVals holds the raw --dry-run / --wait values until fs.Parse has run.
type postFlagVals struct {
	dryRun *bool
	wait   *string
}

// addPostFlags registers the two cross-verb modifiers on a verb's FlagSet. They are
// registered identically on every mutating verb so a caller never has to remember which
// verb accepts which — an unknown flag is a usage error (exit 2), not a silent ignore.
func addPostFlags(fs *flag.FlagSet) postFlagVals {
	return postFlagVals{
		dryRun: fs.Bool("dry-run", false,
			"run every check and STOP before the write; exit 0, audited dryrun, charges neither meter"),
		wait: fs.String("wait", "",
			"on a rate-limit refusal (exit 4), wait up to this long in-tool for the budget to free (e.g. 20m) "+
				"instead of returning exit 4; default: do not wait"),
	}
}

// maxWait bounds --wait. The budget window is a ROLLING HOUR, so the longest legitimate
// wait is "just over an hour" — the case where the budget was spent in one burst and the
// oldest charged entry has an almost-full hour left in the window (deskkit reports
// retry-after 3601s for exactly that). 90m leaves headroom for that worst case while still
// refusing a wait that cannot be about this limiter: a tool that can block a session for a
// day is worse than the refusal it is avoiding.
const maxWait = 90 * time.Minute

func (v postFlagVals) resolve() (postOpts, bool) {
	var o postOpts
	o.dryRun = *v.dryRun
	if *v.wait != "" {
		d, err := time.ParseDuration(*v.wait)
		if err != nil {
			fmt.Fprintf(stderr, "deskpost: --wait %q is not a duration (e.g. 90s, 20m): %v\n", *v.wait, err)
			return o, false
		}
		if d < 0 || d > maxWait {
			fmt.Fprintf(stderr, "deskpost: --wait %s is out of range — it must be between 0 and %s "+
				"(the write budget is a ROLLING HOUR, so a longer wait is not waiting for this limiter)\n", d, maxWait)
			return o, false
		}
		o.wait = d
	}
	if o.dryRun && o.wait > 0 {
		// A rehearsal that blocks for twenty minutes is not a rehearsal. Refusing the
		// combination is clearer than silently ignoring one of the two flags.
		fmt.Fprintln(stderr, "deskpost: --dry-run and --wait are mutually exclusive — a dry run makes no "+
			"write, so there is nothing to wait for the budget to permit")
		return o, false
	}
	return o, true
}

// cmdComment takes a bare <number>, which may name EITHER a pull request or an issue —
// GitHub numbers both from one per-repo sequence (#296). The kind is resolved
// against the API before anything is posted (resolveTarget); there is deliberately no
// --issue / --pr flag, because a caller-declared kind is a second source of truth that can
// disagree with the remote, and the remote is the one that decides where the comment lands.
func cmdComment(argv []string) int {
	rest := argv[1:]
	if len(rest) < 2 {
		fmt.Fprintln(stderr, "usage: deskpost comment <owner/repo> <number> --body-file F   (<number> = a PR or an issue)")
		return 2
	}
	owner, name, ok := splitRepo(rest[0])
	if !ok {
		fmt.Fprintln(stderr, "deskpost: repo must be owner/name, got "+rest[0])
		return 2
	}
	pr, ok := parsePR(rest[1])
	if !ok {
		fmt.Fprintln(stderr, "deskpost: number must be a positive integer, got "+rest[1])
		return 2
	}
	fs := flag.NewFlagSet("comment", flag.ContinueOnError)
	fs.SetOutput(stderr)
	bodyFile := fs.String("body-file", "", "path to the comment body file (required)")
	// --head is OPTIONAL here and REQUIRED on the verdict verbs, and the asymmetry is not
	// laziness: `comment` also targets ISSUES, which have no head at all (#296), and most
	// desk comments are not written against a specific commit.
	//
	// When it IS given, it is enforced exactly as `review --head` is (#513,
	// second defect). Without it the tool stamps whichever head is live at POST time into
	// the audit ledger and the idempotency key — on #505 the reviewer examined
	// f9c9ca94 and the recorded head was d7b2d1d3, because a merge landed mid-review. That
	// one was a clean keep-current merge so the body's conclusions still held, but the tool
	// guaranteed nothing: it would have stamped the new head identically had the push been
	// real work, and a head-pinned claim recorded against a commit nobody read is the same
	// defect #197 closed on the review path.
	head := fs.String("head", "", "assert the PR head the comment was written against — FULL 40-char "+
		"lowercase hex; refuses if the head has moved (optional; not valid on an issue)")
	raw := addPostFlags(fs)
	if err := fs.Parse(rest[2:]); err != nil {
		return 2
	}
	opts, ok := raw.resolve()
	if !ok {
		return 2
	}
	if *bodyFile == "" {
		fmt.Fprintln(stderr, "deskpost comment: --body-file is required")
		return 2
	}
	if *head != "" && !isFullSHA(*head) {
		fmt.Fprintln(stderr, "deskpost comment: "+headFormError(*head))
		return 2
	}
	body, err := os.ReadFile(*bodyFile)
	if err != nil {
		fmt.Fprintln(stderr, "deskpost comment: cannot read --body-file: "+err.Error())
		return 2
	}
	return runComment(owner, name, pr, *head, body, argv, opts)
}

func cmdReady(argv []string) int {
	rest := argv[1:]
	if len(rest) < 2 {
		fmt.Fprintln(stderr, "usage: deskpost ready <owner/repo> <pr>")
		return 2
	}
	owner, name, ok := splitRepo(rest[0])
	if !ok {
		fmt.Fprintln(stderr, "deskpost: repo must be owner/name, got "+rest[0])
		return 2
	}
	pr, ok := parsePR(rest[1])
	if !ok {
		fmt.Fprintln(stderr, "deskpost: pr must be a positive integer, got "+rest[1])
		return 2
	}
	fs := flag.NewFlagSet("ready", flag.ContinueOnError)
	fs.SetOutput(stderr)
	raw := addPostFlags(fs)
	if err := fs.Parse(rest[2:]); err != nil {
		return 2
	}
	opts, ok := raw.resolve()
	if !ok {
		return 2
	}
	return runReady(owner, name, pr, argv, opts)
}

func splitRepo(s string) (owner, name string, ok bool) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parsePR(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

func usage() {
	fmt.Fprint(stderr, `deskpost — post the reviewer App's verdict / comment / ready-flip

usage:
  deskpost review          <owner/repo> <pr>     --verdict approve|request-changes --head <full-40-char-sha> --body-file F
  deskpost security-review <owner/repo> <pr>     --verdict pass|fail              --head <full-40-char-sha> --body-file F
  deskpost comment         <owner/repo> <number> --body-file F [--head <full-40-char-sha>]
  deskpost ready           <owner/repo> <pr>
  deskpost version

the two verdict verbs (a risk-classed PR needs BOTH at the same head):
  review           the CORRECTNESS verdict. Body carries 'Verdict: approve|request-changes'.
                   Submitted as APPROVE / REQUEST_CHANGES.
  security-review  the SECURITY verdict. Body carries
                   'Security-Review: pass|fail'. A PASS is submitted as a COMMENT-event
                   review — state COMMENTED, so `+"`ready`"+`'s gate (e) can read it while the
                   board's review state is NOT flipped to APPROVED over live correctness
                   findings. A FAIL is submitted as REQUEST_CHANGES. Do NOT post a clean
                   security pass as a plain comment: `+"`ready`"+` reads reviews, never
                   comments, so a comment-shaped pass leaves a risk-classed PR unflippable
                   (#513 / #438).

targets:
  comment        <number> may be a PULL REQUEST or an ISSUE — GitHub numbers both from one
                 per-repo sequence, so the number names exactly one object and deskpost
                 resolves which kind it is against the API before posting (no flag, no
                 guess). Both kinds get the same body checks, trust gate, budget, audit
                 line and idempotency. --head is optional and PR-only: when given, the
                 comment is refused if the head has moved since it was written.
  review, security-review, ready
                 pull requests ONLY. Given an issue number they refuse (exit 5) and name
                 `+"`comment`"+` — they never report it as unverifiable (exit 6).

modifiers (every mutating verb; both default off):
  --dry-run       run every check, then STOP before the write. Exit 0, audited 'dryrun' —
                  a result class NEITHER meter counts, so rehearsing a body until it passes
                  costs no charged attempt. It waives nothing: same checks, same order.
  --wait <dur>    on a rate-limit refusal (exit 4), wait in-tool up to <dur> (max 90m) for the
                  budget to free, instead of returning 4. Use this rather than falling back
                  to a raw 'gh pr review' — that command has NO --head flag, so a verdict
                  written against one commit lands on whatever the head is when it runs.

exit codes: 0 ok/noop/dryrun · 3 disabled · 4 rate-limited · 5 refused · 6 unverifiable · 2 usage
`)
}
