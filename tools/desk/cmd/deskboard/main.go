// deskboard — the desk's read-only cross-repo board (supersedes an earlier v1). It
// answers what the
// review/verify/next-job loops need to know across the fixed repo set WITHOUT any
// mutating GitHub call: open PRs, the per-PR ACTION classification, review states,
// the awaiting-verification queue, and reviewer diff/file reads.
//
// GET-only end to end: every gh invocation is a read, proven by the
// PATH-shim test. Fail-closed: any gh/API/parse error on ANY repo fails the
// whole run (exit 6, repo named) — never a partial board that reads as "nothing open".
//
// Exit codes (deskkit contract): 0 ok · 3 disabled · 5 refused (bad repo/args) ·
// 6 unverifiable (a read failed / precondition could not be positively verified).
//
// Banners (STALE, audit-age): the JSON output (default; the loops consume it)
// carries the drift/reset signal AS FIELDS in its header — the machine path writes
// NOTHING to stderr, so `deskboard prs | json-parser` stays clean. The --table (human)
// path additionally prints the two banners to stderr. This split is deliberate: an
// unconditional stderr banner would corrupt every JSON consumer.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

var usage = `deskboard — read-only cross-repo board (desk-tools)

usage:
  deskboard prs                     open PRs across the fixed repo set
  deskboard actions                 per-PR ACTION classification (+ tombstones, security gate)
  deskboard reviews <repo> <pr>     review states + which head each verdict landed at
  deskboard queue                   open verify-gate issues (awaiting-verification view)
  deskboard throughput              per-stage QUEUE DEPTH vs POOL SLOTS (dispatch / review /
                                    verify / intake), naming the bottleneck stage and the
                                    exact "deskroster set --role LOOP --width N" that widens
                                    it. Depths are derived from dispatch, actions and
                                    awaiting — nothing re-parses the board. SLOTS is the
                                    loop's resolved pool width (CAPACITY, not live
                                    occupancy). A stage whose depth could not be read is
                                    could-not-check and is EXCLUDED from bottleneck
                                    selection, never counted as an empty queue; the output
                                    always states how many of the four stages it read.
                                    Also accepted as --throughput.
  deskboard health                  default-branch ("is main red?") health per watched
                                    repo — three states: green / RED / COULD-NOT-CHECK
                                    (#295; also rides in the actions header + banner)
  deskboard dispatch                cross-repo DISPATCH queue — the work to START: briefs at
                                    todo/in-progress that are eligible and UNCLAIMED (no open
                                    origin-branch claim), merged from every configured root via
                                    the PINNED statusgen's --next-up (the SAME claim-filtered,
                                    capped Next-up selection STATUS.md shows). Honest about
                                    caps: reports the held-back decomposition (N by per-stream
                                    caps, M by span) and per-root claim-degradation, so an empty
                                    queue is distinguishable from a throttled one (#321).
                                    Aliases: todo, next, next-up.
  deskboard awaiting                cross-repo AWAITING-VERIFICATION queue — briefs at
                                    implemented/verified, merged from every configured root
                                    via the PINNED statusgen.
                                    NOT the dispatch queue: it contains no todo brief, and
                                    dispatching from it sends workers at finished work — use
                                    ` + "`dispatch`" + ` to find work to start (#321)
  deskboard nextup                  deprecated alias for ` + "`awaiting`" + ` — the name says
                                    dispatch queue, the rows are the verification backlog
                                    (the real dispatch queue is ` + "`deskboard dispatch`" + `)
  deskboard scope                   reconcile the watched repo set against the owners' repos
                                    with open PRs — names every repo the board does NOT read
  deskboard policydrift             compiled-in repo VISIBILITY vs the GitHub API
                                   ; exit 6 + loud stderr on any drift
  deskboard stalled                 stalled-draft detector + shepherd dispatch list:
                                    open drafts whose reviewer-App
                                    verdict at head is CHANGES_REQUESTED and whose author
                                    has not pushed or commented in --min-age-hours.
                                    Disposition is advisory (shepherd / close-candidate).
  deskboard diff <repo> <pr>        the PR diff (reviewer read)
  deskboard files <repo> <pr> [path]  changed files, or one file's contents (reviewer read)
  deskboard --version               source SHA / build time

flags:
  --table                   human-readable table to stdout + STALE/audit banners to stderr
                            (default output is JSON; banner signal rides in the JSON header).
                            NOTE: the stalled verb inverts this — its default IS the human table
                            (a discovery verb for a human/desk scan); pass --json for the
                            machine shape the dispatch consumer reads.
  --json                    emit JSON (stalled default is the human table; this flag
                            selects the JSON shape for stalled). No-op on other verbs,
                            which already default to JSON.
  --min-age-hours N         stalled: the stall window in hours (default 48).
  --merge-now-threshold D   MERGE-NOW approved-age decay threshold (default 20m)
  --unreviewed-threshold D  age past which an open PR with NO reviewer verdict is named
                            in an UNREVIEWED line (default 30m; #359 temporal axis).
                            A NEGLECT alarm, not the review trigger — a fresh PR is
                            NEEDS-REVIEW immediately, at any age; this line firing means
                            the cadenced sweep missed it.
  --delta                   (prs/actions/queue/nextup only) print only rows CHANGED since
                            the prior snapshot — new, removed, or field-changed. First run
                            or a corrupt/missing/schema-mismatched snapshot prints full
                            output with a label (never a silent empty diff). Implies the
                            text/table path (console discipline).
  --quiet                   (prs/actions/queue/nextup only) one summary line: counts per
                            state bucket + delta count vs snapshot + actionable count
                            (on actions: NEEDS-REVIEW + RE-REVIEW — the dispatch gate).
                            Composable with --delta (quiet line first, changed rows after).
                            Implies the text/table path.

env (nextup):
  DESK_ROOTS      override root PATHS: "<owner>/<repo>=<path>,..." (repos must stay
                  inside the fixed set below; the set itself is compiled in)
  STATUSGEN_BIN   statusgen binary to run (default: statusgen on PATH — install the
                  pinned release per tools/statusgen/README.md; the in-repo
                  tools/statusgen copy is FROZEN and is never run)

repo must be in the desk scope (compiled-in — org-default;
"medici-finance/*" means every current and future repo in that org):
  ` + strings.Join(deskkit.AllowedRepoScope(), "\n  ") + `
`

func main() {
	// The roster class is an EXPLICIT declaration, never the zero value by accident
	// (a correctness review found SetToolClass had no caller anywhere
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
		fmt.Fprintf(stdout, "deskboard sourceSHA=%s builtAt=%s releaseTag=%s\n", s, b, deskkit.ReleaseTagOrDev())
		return deskkit.ExitOK
	}

	// kill switch FIRST, before any read. Guard audits result=disabled itself.
	if err := deskkit.Guard(); err != nil {
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}

	table := false
	delta := false
	quiet := false
	jsonOut := false
	minAgeHours := stalledMinAgeDefault
	mergeNowThreshold := 20 * time.Minute
	// #359: the UNREVIEWED line is a NEGLECT ALARM, never the review trigger — the
	// classifier surfaces a fresh PR as NEEDS-REVIEW immediately, at any age, and the
	// desk's cadenced `actions --delta --quiet` sweep is what dispatches it. The
	// original default was 2h, rationalised as "the desk reviews within minutes, so
	// this fires only on the PR nothing was ever going to pick up" — but while
	// --delta/--quiet existed only on verbs with no review state, the quiet loop had
	// no classified sweep, this alarm was the desk's first loud signal, and 2h became
	// the desk's de-facto review latency. 30m keeps it comfortably past a healthy
	// ~5-minute cadence (six missed sweeps) while making a dead trigger path loud in
	// half an hour instead of two.
	unreviewedThreshold := 30 * time.Minute
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--table":
			table = true
		case "--delta":
			delta = true
		case "--quiet":
			quiet = true
		case "--json":
			jsonOut = true
		case "--min-age-hours":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "deskboard: --min-age-hours requires a positive integer argument (e.g. 48)")
				return deskkit.ExitRefused
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "deskboard: --min-age-hours: invalid value "+strconv.Quote(args[i])+": must be a positive integer")
				return deskkit.ExitRefused
			}
			minAgeHours = n
		case "--merge-now-threshold":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "deskboard: --merge-now-threshold requires a duration argument (e.g. 20m)")
				return deskkit.ExitRefused
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				fmt.Fprintln(stderr, "deskboard: --merge-now-threshold: invalid duration "+strconv.Quote(args[i])+": "+err.Error())
				return deskkit.ExitRefused
			}
			mergeNowThreshold = d
		case "--unreviewed-threshold":
			i++
			if i >= len(args) {
				fmt.Fprintln(stderr, "deskboard: --unreviewed-threshold requires a duration argument (e.g. 2h)")
				return deskkit.ExitRefused
			}
			d, err := time.ParseDuration(args[i])
			if err != nil {
				fmt.Fprintln(stderr, "deskboard: --unreviewed-threshold: invalid duration "+strconv.Quote(args[i])+": "+err.Error())
				return deskkit.ExitRefused
			}
			unreviewedThreshold = d
		case "--throughput":
			// deskboard's modes are SUBCOMMANDS; `throughput` is the canonical spelling and
			// the one every other verb's shape matches. This alias exists because the verb
			// was specified as a flag, and a spelling that silently does nothing is worse
			// than either choice — it would look like a board with no bottleneck.
			pos = append(pos, "throughput")
		case "-h", "--help":
			fmt.Fprint(stdout, usage)
			return deskkit.ExitOK
		default:
			pos = append(pos, a)
		}
	}
	if len(pos) == 0 {
		fmt.Fprint(stderr, usage)
		return deskkit.ExitRefused
	}

	sub := pos[0]
	rest := pos[1:]
	hdr := buildHeader()

	rep, err := dispatch(sub, rest, hdr, mergeNowThreshold, unreviewedThreshold, minAgeHours)
	if err != nil {
		logRun(sub, resultFor(err), err.Error())
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}

	// --delta / --quiet: reshape stdout for console discipline.
	// These flags select the TEXT path — a JSON machine consumer does not pass them.
	// A subcommand not in deltaExtractors + either flag set = Refused (exit 5): never
	// silently ignore a flag a desk relied on for noise discipline.
	if delta || quiet {
		if _, supported := deltaExtractors[sub]; !supported {
			logRun(sub, deskkit.ResultRefused, "refused: --delta/--quiet not supported on "+sub)
			fmt.Fprintln(stderr, "deskboard: --delta/--quiet are supported on prs, actions, queue, and nextup only")
			return deskkit.ExitRefused
		}
		detailSuffix, applied := runDeltaQuiet(stdout, stderr, sub, rep, delta, quiet)
		if !applied {
			// Unreachable given the support check above — but if the two ever drift,
			// the safe direction is NOISY, not silent: print the full report rather
			// than exiting 0 having written nothing to stdout.
			fmt.Fprintf(stderr, "deskboard: WARNING --delta/--quiet could not be applied to %s; showing full output\n", sub)
			rep.render(stdout)
			detailSuffix = " delta=unavailable"
		}
		printBanners(stderr, hdr)
		logRun(sub, deskkit.ResultOK, rep.detail+detailSuffix)
		return deskkit.ExitOK
	}

	// Output mode. `stalled` inverts the deskboard default: its default is the human
	// table (a discovery verb for a human/desk scan), and --json selects the machine
	// shape. Every other verb defaults to JSON and selects the table with --table. Both
	// paths still carry the STALE/audit banners — in-band on JSON, on stderr
	// for tables — so neither mode can read as a clean board it did not verify.
	wantTable := table
	if sub == "stalled" {
		if table && jsonOut {
			// Contradictory output flags. Picking one silently gives the operator a shape
			// they did not ask for — refuse (exit 5) rather than guess.
			fmt.Fprintln(stderr, "deskboard: stalled: --table and --json are contradictory; pass one")
			return deskkit.ExitRefused
		}
		wantTable = !jsonOut
	}
	if wantTable {
		printBanners(stderr, hdr)
		rep.render(stdout)
	} else {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep.value); err != nil {
			logRun(sub, deskkit.ResultUnverifiable, "encode: "+err.Error())
			fmt.Fprintln(stderr, "deskboard: encode failed:", err)
			return deskkit.ExitUnverifiable
		}
	}

	logRun(sub, deskkit.ResultOK, rep.detail)
	return deskkit.ExitOK
}

// boardWideVerbs are the subcommands that take NO repo argument and instead sweep the
// whole configured set (deskkit.AllowedRepos()). They are exactly the verbs for which
// "found nothing" is indistinguishable from "read nothing", so they are exactly the
// verbs that need the scope guard below.
//
// The per-repo verbs (reviews/diff/files) are deliberately absent: they name a repo, so
// parseRepoPR's IsAllowedRepo check already refuses at exit 5 when the set does not
// contain it — and an empty set contains nothing, so they already fail closed.
//
// `nextup` is absent too. It sweeps CONFIGURED ROOTS, not the repo set, and it already
// states its coverage in its own output (TestNextup_EmptyBoardSaysWhatItRead) — which is
// the precedent this guard generalises to the other five.
//
// `stalled` (#515) iterates deskkit.AllowedRepos() exactly like the original five
// (cmd/deskboard/stalled.go), so an empty set reads zero repos and "0 stalled" is the
// same false claim as the others' "found nothing" — it holds the guard's own
// precedent already (TestStalled_RepoListFailureFailsClosed: "'no stalled drafts in
// that repo' would be a claim nothing verified") and simply predates this map.
var boardWideVerbs = map[string]bool{
	"prs": true, "actions": true, "queue": true, "health": true, "policydrift": true,
	"stalled": true,
}

// dispatch routes a subcommand to its handler, validating repo args against the fixed
// set — a bad repo or arity is a Refused (exit 5), never a guessed default.
func dispatch(sub string, rest []string, hdr Header, mergeNowThreshold, unreviewedThreshold time.Duration, minAgeHours int) (*Report, error) {
	// #489: a board-wide verb with an empty repo set reads NOTHING, and before this
	// guard each one reported that as a clean result at exit 0 — `policydrift`, the
	// drift GATE, returned "no drift" without checking anything; `health` printed
	// "0 green · 0 RED · scope: 0 watched repo(s)" while owning an unused
	// COULD-NOT-CHECK state; and `actions` announced every PR a prior run had seen as
	// "MERGED — drop from your list".
	//
	// The guard is here rather than in each handler on purpose: this is the one place
	// every board-wide verb provably passes through, so a new one cannot be added
	// without a decision about which side of this line it sits on.
	if boardWideVerbs[sub] {
		if err := deskkit.BoardScopeError(); err != nil {
			return nil, err
		}
	}
	switch sub {
	case "prs":
		return cmdPRs(hdr)
	case "actions":
		return cmdActions(&hdr, mergeNowThreshold, unreviewedThreshold)
	case "queue":
		return cmdQueue(hdr)
	case "health":
		return cmdHealth(hdr)
	case "awaiting", "nextup":
		// #321: ONE population, honestly named. `nextup` stays as an alias so no
		// consumer breaks, and every report says which population it is.
		return cmdAwaiting(hdr, sub)
	case "scope":
		return cmdScope(hdr)
	case "dispatch", "todo", "next", "next-up":
		// #321: the DISPATCH queue (todo/in-progress + unclaimed) is a different
		// population from `awaiting` (implemented/verified). It is served by the
		// pinned statusgen's `--next-up` emitter — the SAME claim-filtered, capped
		// Next-up selection the STATUS.md board shows — merged cross-root here. The
		// verb is deliberately distinct from `awaiting`: dispatching from `awaiting`
		// sends workers at finished work.
		return cmdDispatch(hdr, sub)
	case "throughput":
		return cmdThroughput(hdr, mergeNowThreshold, unreviewedThreshold)
	case "policydrift":
		return cmdPolicyDrift(hdr)
	case "stalled":
		return cmdStalled(hdr, minAgeHours)
	case "reviews", "diff", "files":
		repo, num, path, err := parseRepoPR(sub, rest)
		if err != nil {
			return nil, err
		}
		switch sub {
		case "reviews":
			return cmdReviews(hdr, repo, num)
		case "diff":
			return cmdDiff(hdr, repo, num)
		default:
			return cmdFiles(hdr, repo, num, path)
		}
	default:
		return nil, deskkit.Refused("refused: unknown subcommand " + strconv.Quote(sub) + " (see --help)")
	}
}

// parseRepoPR validates <repo> <pr> [path] and enforces the fixed repo set.
func parseRepoPR(sub string, rest []string) (repo string, num int, path string, err error) {
	if len(rest) < 2 {
		return "", 0, "", deskkit.Refused("refused: " + sub + " needs <repo> <pr>")
	}
	repo = rest[0]
	if !deskkit.IsAllowedRepo(repo) {
		return "", 0, "", deskkit.Refused("refused: repo " + strconv.Quote(repo) +
			" is outside the fixed set")
	}
	n, perr := strconv.Atoi(rest[1])
	if perr != nil || n <= 0 {
		return "", 0, "", deskkit.Refused("refused: pr " + strconv.Quote(rest[1]) + " is not a positive integer")
	}
	num = n
	if sub == "files" && len(rest) >= 3 {
		path = rest[2]
	}
	return repo, num, path, nil
}

// buildHeader stamps asOf (#209), the in-band drift/reset banner signal,
// and active loop stop-flags.
func buildHeader() Header {
	h := Header{AsOf: time.Now().UTC().Format(time.RFC3339)}
	h.StaleState, h.Stale, h.StaleDetail = staleState()
	h.AuditFirstTS, h.AuditReset = auditAgeState()
	h.StopFlags = deskkit.ActiveStopFlags()
	return h
}

// Stale states (#236). `stale` alone could not tell "checked, fresh" from "could not
// check", and reported the second as the first — a fail-open on a drift detector, on
// the field the operator guidance names as authoritative.
const (
	staleStateInSync        = "in-sync"        // checked: installed tree == origin/main
	staleStateDrift         = "drift"          // checked: they differ
	staleStateNotApplicable = "not-applicable" // unpinned build: there is no install to drift
	staleStateUnknown       = "unknown"        // COULD-NOT-CHECK: git unavailable / refs missing
)

// staleState reports installed-vs-origin drift as a three-state verdict plus the
// authoritative boolean.
//
// The boolean now fails CLOSED: an `unknown` (the check could not run) reports
// stale=true, because the one answer a drift detector must never give is "fresh" when
// it did not look. `not-applicable` is the separate, honest case — an unpinned `go run`
// has no installed binary that COULD have drifted, which is a statement about the
// world, not a failed measurement, so it reports stale=false and says so by name. The
// two are kept apart rather than both folded into "not assessable", which is what made
// the old detail line contradict its own boolean.
//
// isPinned and gitTree are vars for the same reason searchOpenPRs and ghRun already are:
// the two MEASURING outcomes (drift / in-sync) are the ones a pinned install actually
// runs, and with the real git call hard-wired neither could be driven from a test — the
// drift branch could be mutated back to "in-sync, stale=false" (i.e. #236 itself, on the
// live path) with the whole suite still green. Seams, not behaviour: production still
// calls deskkit.IsPinned and the real git.
var (
	isPinned = deskkit.IsPinned
	gitTree  = gitTreeReal
)

func staleState() (state string, stale bool, detail string) {
	if !isPinned() {
		return staleStateNotApplicable, false,
			"unpinned build (go run) — there is no installed binary to drift; install via `sudo make desk-install`"
	}
	src, _ := deskkit.Version()
	installedTree, err1 := gitTree(src)
	// FULLY-QUALIFIED remote-tracking ref, not the bare short name `origin/main`
	// (#885): a stray local `refs/heads/origin/main` decoy would otherwise shadow
	// the real remote tip and make the drift check compare against a stale tree.
	originTree, err2 := gitTree("refs/remotes/origin/main")
	if err1 != nil || err2 != nil || installedTree == "" || originTree == "" {
		return staleStateUnknown, true,
			"COULD-NOT-CHECK drift (git unavailable or refs missing) — reported as STALE because " +
				"an unverifiable drift check is not evidence of freshness (#236); re-run where " +
				"`git rev-parse origin/main:tools/desk` resolves"
	}
	if installedTree != originTree {
		return staleStateDrift, true,
			"installed sourceSHA " + src + " tools/desk tree differs from origin/main — reinstall (sudo make desk-install)"
	}
	return staleStateInSync, false, "in sync with origin/main"
}

// gitTreeReal returns the git tree object id of tools/desk at a ref/sha (read-only).
func gitTreeReal(ref string) (string, error) {
	out, err := exec.Command("git", "rev-parse", ref+":tools/desk").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// auditAgeState returns the audit file's first-ts and whether it looks recently reset
// Empty history is bootstrap (not suspicious). A first-ts younger than the reset
// window flags a possible history wipe. Best-effort: an unreadable audit file yields
// no signal here (the outward-write flow surfaces true corruption as exit 6).
func auditAgeState() (firstTS string, reset bool) {
	ts, err := deskkit.FirstTS()
	if err != nil || ts == "" {
		return "", false
	}
	t, perr := time.Parse(time.RFC3339, ts)
	if perr != nil {
		return ts, false
	}
	return ts, time.Since(t) < time.Hour
}

// printBanners writes the two human banners to stderr (--table path only).
func printBanners(w io.Writer, h Header) {
	switch h.StaleState {
	case staleStateUnknown:
		// Named distinctly from a measured drift: the operator's action differs
		// (make the check runnable vs reinstall), and conflating them is how a
		// could-not-check becomes invisible.
		fmt.Fprintln(w, "STALE-UNKNOWN: "+h.StaleDetail)
	case staleStateDrift:
		fmt.Fprintln(w, "STALE: "+h.StaleDetail)
	default:
		fmt.Fprintln(w, "desk-tools: "+h.StaleDetail)
	}
	if h.AuditFirstTS == "" {
		fmt.Fprintln(w, "audit: no history yet (bootstrap)")
	} else if h.AuditReset {
		fmt.Fprintln(w, "audit: WARNING first-ts "+h.AuditFirstTS+" is <1h old — possible history reset")
	} else {
		fmt.Fprintln(w, "audit: first-ts "+h.AuditFirstTS)
	}
	// Stop flags banner: any active STOP flag or stale HEARTBEAT is surfaced
	// on every board read so a silently-stopped loop is not discovered by absence.
	if len(h.StopFlags) > 0 {
		for _, f := range h.StopFlags {
			tag := "STOP"
			if f.Stale {
				tag = "STALE"
			}
			fmt.Fprintf(w, "WARNING: %s flag active — %s: %s\n", tag, f.Name, f.Reason)
		}
	}
}

// logRun appends the mandatory per-run audit line. Board runs (prs/actions) carry
// the open-PR set in Detail so a later run can compute #209 tombstones; other verbs pass
// their message. A failure to write the audit line is surfaced loudly, not swallowed.
func logRun(verb, result, detail string) {
	if err := deskkit.Log(deskkit.Entry{Tool: "deskboard", Verb: verb, Result: result, Detail: detail}); err != nil {
		fmt.Fprintln(os.Stderr, "deskboard: WARNING could not write audit line:", err)
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
