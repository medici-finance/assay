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
// Exit codes (deskkit contract): 0 ok · 3 disabled · 4 rate-limited (`actions` stopped
// at the --rate-floor; the message names the quota reset time) · 5 refused (bad repo/args) ·
// 6 unverifiable (a read failed / precondition could not be positively verified).
//
// Banners (STALE, audit-age): the JSON output (default; the loops consume it)
// carries the drift/reset signal AS FIELDS in its header, so stdout stays clean JSON.
// The --table (human) path additionally prints the two banners to stderr. `actions`
// also prints one `deskboard: cost …` line to stderr (#2421) — stderr, never stdout, so
// `deskboard actions | json-parser` still reads only the JSON, where the same numbers
// ride as Header.cost.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/perfrec"
	"github.com/medici-finance/assay/tools/desk/internal/cli"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/spf13/cobra"
)

// version is an optional bare-`vX.Y.Z` build stamp (`-ldflags -X main.version`).
// It feeds the brief-reading version gate (example-stream/06); left empty on a real
// release, where the namespaced ReleaseTag stamp supplies the version instead.
var version string

var perfVerbs = []string{"prs", "actions", "review-queue", "queue", "health", "awaiting", "nextup", "scope", "dispatch", "throughput", "policydrift", "stalled", "reviews", "diff", "files"}

func main() {
	args := os.Args[1:]
	selected := perfrec.ResolveContext(perfrec.ContextInput{})
	// Keep the registered performance wrapper at the process entrypoint, but call it
	// only after Cobra selected an execution (help and version never record samples).
	record := func(run func() int) int {
		return deskkit.PerfRunSelected("deskboard", perfVerbs, args, selected, func([]string) int { return run() })
	}
	os.Exit(executeRecorded(args, os.Stdout, os.Stderr, startup, record, func(c perfrec.Context) { selected = c }))
}

// startup retains the execution-only authority checks. Help and version never enter it.
func startup(stderr io.Writer) int {
	deskkit.SetToolClass(deskkit.ClassForTool(false))
	deskkit.EchoEffectiveConfig(stderr)
	if !deskkit.CheckVerbActivation(stderr) {
		return deskkit.ExitUnverifiable
	}
	return 0
}

func run(args []string, stdout, stderr io.Writer) int {
	return execute(args, stdout, stderr, nil)
}

// These seams let the contract tests observe the typed boundary without a forge or roster.
var boardGuard = deskkit.Guard
var boardHeader = buildHeader
var boardDispatch = dispatch
var boardLookupEnv = os.LookupEnv
var boardResolved func(*cli.Values)

type boardExit int

func (e boardExit) Error() string { return fmt.Sprintf("exit %d", e) }
func (e boardExit) ExitCode() int { return int(e) }
func boardCode(n int) error {
	if n == 0 {
		return nil
	}
	return boardExit(n)
}

type boardOptions struct {
	table, delta, quiet, json    bool
	out, tier, reviewObservation string
	minAge, floor                int
	merge, unreviewed            time.Duration
}

func versionLine() string {
	s, b := deskkit.Version()
	return fmt.Sprintf("deskboard sourceSHA=%s builtAt=%s releaseTag=%s", s, b, deskkit.ReleaseTagOrDev())
}

func boardBindings() []cli.Binding {
	return []cli.Binding{
		{Key: "table", Kind: cli.Bool, Flag: "table", Persistent: true, Default: false, Usage: "human table with STALE/audit banners on stderr"},
		{Key: "json", Kind: cli.Bool, Flag: "json", Persistent: true, Default: false, Usage: "JSON output (stalled otherwise defaults to a table)"},
		{Key: "delta", Kind: cli.Bool, Flag: "delta", Persistent: true, Default: false, Usage: "only changed rows (prs/actions/queue/nextup)"},
		{Key: "quiet", Kind: cli.Bool, Flag: "quiet", Persistent: true, Default: false, Usage: "summary line (prs/actions/queue/nextup)"},
		{Key: "review-observation", Kind: cli.String, Flag: "review-observation", Persistent: true, Usage: "throughput only: reuse a complete fresh review observation; no review recollection on hold"},
		{Key: "out", Kind: cli.String, Flag: "out", Persistent: true, Default: "", Usage: "write primary output to a caller-minted file"},
		{Key: "min-age-hours", Kind: cli.String, Flag: "min-age-hours", Persistent: true, Default: "48", Usage: "stalled draft window in positive hours"},
		{Key: "merge-now-threshold", Kind: cli.String, Flag: "merge-now-threshold", Persistent: true, Default: "20m", Usage: "MERGE-NOW approved-age decay duration"},
		{Key: "unreviewed-threshold", Kind: cli.String, Flag: "unreviewed-threshold", Persistent: true, Default: "30m", Usage: "UNREVIEWED neglect alarm duration"},
		{Key: "rate-floor", Kind: cli.String, Flag: "rate-floor", Persistent: true, Usage: "actions quota floor; 0 disables; flag overrides DESKBOARD_RATE_FLOOR (default 500)"},
		// Separate binding preserves the legacy trimmed, first-non-empty fallback.
		{Key: "rate-floor-env", Kind: cli.String, Env: []string{rateFloorEnv}},
		{Key: "tier", Kind: cli.String, Flag: "tier", Persistent: true, Default: "", Usage: "dispatch execution tier (any or strong)"},
		{Key: "throughput", Kind: cli.Bool, Flag: "throughput", Persistent: true, Hidden: true, Default: false, Usage: "legacy alias for the throughput verb"},
	}
}

func execute(args []string, stdout, stderr io.Writer, pre func(io.Writer) int) int {
	return executeRecorded(args, stdout, stderr, pre, nil)
}

func executeRecorded(args []string, stdout, stderr io.Writer, pre func(io.Writer) int, record func(func() int) int, sinks ...func(perfrec.Context)) int {
	// Additive offline discovery; bare version output and help stay unchanged.
	if len(args) == 2 && (args[0] == "--version" || args[0] == "version") && args[1] == "--json" {
		if err := json.NewEncoder(stdout).Encode(map[string]any{"version": versionLine(), "readerCapabilities": deskkit.ReaderCapabilities(deskkit.EffectiveToolVersion(version), "deskboard")}); err != nil {
			fmt.Fprintln(stderr, err)
			return deskkit.ExitUnverifiable
		}
		return 0
	}
	var sink func(perfrec.Context)
	if len(sinks) > 0 {
		sink = sinks[0]
	}
	build := func() *cobra.Command {
		root := cli.NewRoot("deskboard", "read-only cross-repo board")
		root.Long = "Read-only cross-repo board. Output is JSON except stalled (table by default).\nAll roster, custody, scope, kill-switch and admission checks apply to execution."
		set := cli.Declare(root, boardBindings()...)
		handler := func(sub string) func(*cobra.Command, []string) error {
			return func(_ *cobra.Command, rest []string) error {
				o, err := resolveBoard(set)
				if err != nil {
					fmt.Fprintln(stderr, err)
					return boardExit(deskkit.ExitRefused)
				}
				if o.reviewObservation != "" && sub != "throughput" {
					return deskkit.Refused("deskboard: --review-observation is supported only by throughput")
				}
				if o.out != "" && (o.delta || o.quiet) {
					fmt.Fprintln(stderr, "deskboard: --out is not supported together with --delta/--quiet")
					return boardExit(deskkit.ExitRefused)
				}
				if sub == "review-queue" && o.table {
					return deskkit.Refused("deskboard review-queue requires JSON output; --table cannot produce the complete artifact")
				}
				if sub == "stalled" && o.table && o.json {
					fmt.Fprintln(stderr, "deskboard: stalled: --table and --json are contradictory; pass one")
					return boardExit(deskkit.ExitRefused)
				}
				if o.delta || o.quiet {
					if _, ok := deltaExtractors[sub]; !ok {
						fmt.Fprintln(stderr, "deskboard: --delta/--quiet are supported on prs, actions, queue, and nextup only")
						return boardExit(deskkit.ExitRefused)
					}
				}
				if pre != nil {
					if n := pre(stderr); n != 0 {
						return boardExit(n)
					}
				}
				invoke := func() int {
					perfSub := sub
					switch sub {
					case "todo", "next", "next-up":
						perfSub = "dispatch"
					}
					deskkit.PerfCommand(perfVerbs, perfSub)
					return runBoard(sub, rest, args, o, stdout, stderr)
				}
				if record != nil {
					return boardCode(record(invoke))
				}
				return boardCode(invoke())
			}
		}
		root.Args = cobra.NoArgs
		root.RunE = func(c *cobra.Command, a []string) error {
			v, _ := c.Flags().GetBool("throughput")
			if !v {
				fmt.Fprintln(stderr, "deskboard: a subcommand is required (see --help)")
				return boardExit(deskkit.ExitRefused)
			}
			return handler("throughput")(c, a)
		}
		for _, v := range []struct {
			name, short string
			aliases     []string
		}{
			{"review-queue", "complete frozen review observation (offline planning bridge)", nil},
			{"prs", "open PRs across watched repos", nil}, {"actions", "per-PR ACTION classification", nil},
			{"queue", "open verify-gate issues", nil}, {"throughput", "queue depth versus pool slots", nil},
			{"health", "default-branch health per watched repo", nil},
			{"dispatch", "eligible unclaimed work to start", []string{"todo", "next", "next-up"}},
			{"awaiting", "awaiting-verification queue", nil}, {"nextup", "deprecated alias for awaiting", nil},
			{"scope", "reconcile watched repos against open PRs", nil}, {"policydrift", "repo visibility drift", nil},
			{"stalled", "stalled drafts and shepherd candidates", nil},
			{"reviews", "review states at each head", nil}, {"diff", "PR diff", nil}, {"files", "changed files or one file's content", nil},
		} {
			name := v.name
			cmd := &cobra.Command{Use: name, Short: v.short, Aliases: v.aliases, Args: cobra.NoArgs}
			if name == "reviews" || name == "diff" || name == "files" {
				cmd.Use += " <repo> <pr>"
				if name == "files" {
					cmd.Use += " [path]"
				}
				cmd.Args = func(c *cobra.Command, a []string) error {
					max := 2
					if name == "files" {
						max = 3
					}
					if err := cobra.RangeArgs(2, max)(c, a); err != nil {
						return err
					}
					n, err := strconv.Atoi(a[1])
					if err != nil || n <= 0 {
						return fmt.Errorf("pr %q is not a positive integer", a[1])
					}
					return nil
				}
			}
			cmd.RunE = func(c *cobra.Command, a []string) error { return handler(c.CalledAs())(c, a) }
			root.AddCommand(cmd)
		}
		root.AddCommand(&cobra.Command{Use: "version", Hidden: true, Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { fmt.Fprintln(stdout, versionLine()); return nil }})
		return root
	}
	return cli.Run(build, args, cli.Options{Performance: sink, IO: cli.IO{Out: stdout, Err: stderr}, Version: versionLine(), VersionTemplate: "{{.Version}}\n", UsageExit: deskkit.ExitRefused})
}

func resolveBoard(set *cli.Set) (boardOptions, error) {
	v, err := set.Resolve(cli.Inputs{LookupEnv: boardLookupEnv})
	if err != nil {
		return boardOptions{}, err
	}
	if boardResolved != nil {
		boardResolved(v)
	}
	o := boardOptions{table: v.Bool("table"), json: v.Bool("json"), delta: v.Bool("delta"), quiet: v.Bool("quiet"), out: v.String("out"), tier: v.String("tier"), reviewObservation: v.String("review-observation")}
	o.minAge, err = strconv.Atoi(v.String("min-age-hours"))
	if err != nil || o.minAge <= 0 {
		return o, deskkit.Refused("deskboard: --min-age-hours requires a positive integer")
	}
	o.merge, err = time.ParseDuration(v.String("merge-now-threshold"))
	if err != nil {
		return o, deskkit.Refused("deskboard: --merge-now-threshold: invalid duration")
	}
	o.unreviewed, err = time.ParseDuration(v.String("unreviewed-threshold"))
	if err != nil {
		return o, deskkit.Refused("deskboard: --unreviewed-threshold: invalid duration")
	}
	raw, source := v.String("rate-floor"), "--rate-floor"
	if !v.IsSet("rate-floor") {
		raw, source = strings.TrimSpace(v.String("rate-floor-env")), rateFloorEnv
		if raw == "" {
			raw = strconv.Itoa(defaultRateFloor)
		}
	}
	o.floor, err = strconv.Atoi(raw)
	if err != nil || o.floor < 0 {
		return o, deskkit.Refused("deskboard: " + source + ": must be a non-negative integer")
	}
	if v.Source("tier") == cli.Flag {
		var ok bool
		o.tier, ok = deskkit.CanonicalDispatchTier(o.tier)
		if !ok {
			return o, deskkit.Refused("deskboard dispatch: --tier is outside the tier vocabulary")
		}
	}
	return o, nil
}

func runBoard(sub string, rest, args []string, o boardOptions, stdout, stderr io.Writer) int {
	if err := boardGuard(); err != nil {
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}
	if code := deskkit.RefuseIfTreeV2BelowV1(deskkit.RootsFromArgs(args), deskkit.EffectiveToolVersion(version), "deskboard", stderr); code != 0 {
		return code
	}
	table, delta, quiet, jsonOut := o.table, o.delta, o.quiet, o.json
	minAgeHours, mergeNowThreshold, unreviewedThreshold, outPath := o.minAge, o.merge, o.unreviewed, o.out
	rateFloor = o.floor
	lastSweepCost = nil
	hdr := boardHeader()

	var rep *Report
	var err error
	if sub == "throughput" && o.reviewObservation != "" {
		rep, err = cmdThroughput(hdr, mergeNowThreshold, unreviewedThreshold, o.reviewObservation)
	} else {
		rep, err = boardDispatch(sub, rest, hdr, mergeNowThreshold, unreviewedThreshold, minAgeHours, o.tier)
	}
	// #2421: one read-cost line per actions sweep, on success and on refusal alike.
	if sub == "actions" && lastSweepCost != nil {
		fmt.Fprintln(stderr, lastSweepCost.line())
	}
	if err != nil {
		logRun(sub, resultFor(err), err.Error())
		fmt.Fprintln(stderr, err)
		return deskkit.ExitCodeOf(err)
	}

	// --out is not composable with --delta/--quiet: those already select their own
	// (text) output path and write a summary/changed-rows shape, not the primary
	// report --out exists to redirect. Refuse rather than silently ignore --out.
	if outPath != "" && (delta || quiet) {
		fmt.Fprintln(stderr, "deskboard: --out is not supported together with --delta/--quiet")
		return deskkit.ExitRefused
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
	if sub == "review-queue" && outPath != "" {
		if err := writeReviewObservation(outPath, rep.value); err != nil {
			logRun(sub, deskkit.ResultUnverifiable, "--out: "+err.Error())
			fmt.Fprintln(stderr, "deskboard review-queue: cannot save artifact:", err)
			return deskkit.ExitUnverifiable
		}
		fmt.Fprintln(stdout, rep.detail)
		logRun(sub, deskkit.ResultOK, rep.detail)
		return deskkit.ExitOK
	}
	// --out redirects the PRIMARY output (table or JSON) to a caller-minted scratch
	// file instead of stdout — a portable substitute for a shell `>` redirect
	// (desk-shell.md §Scratch files) that does not depend on the invoking shell.
	// Banners (stderr) and the audit line are unaffected either way.
	out := stdout
	if outPath != "" {
		f, ferr := os.Create(outPath)
		if ferr != nil {
			logRun(sub, deskkit.ResultUnverifiable, "--out: "+ferr.Error())
			fmt.Fprintln(stderr, "deskboard: --out: cannot create", outPath+":", ferr)
			return deskkit.ExitUnverifiable
		}
		defer f.Close()
		out = f
	}
	if wantTable {
		printBanners(stderr, hdr)
		rep.render(out)
	} else {
		enc := json.NewEncoder(out)
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
	"review-queue": true, "prs": true, "actions": true, "queue": true, "health": true, "policydrift": true,
	"stalled": true,
}

// dispatch routes a subcommand to its handler, validating repo args against the fixed
// set — a bad repo or arity is a Refused (exit 5), never a guessed default.
func dispatch(sub string, rest []string, hdr Header, mergeNowThreshold, unreviewedThreshold time.Duration, minAgeHours int, tier string) (*Report, error) {
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
	case "review-queue":
		return cmdReviewQueue()
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
		// CLI validation is not authority: direct handler callers retain the tier boundary.
		if tier != "" {
			canonical, ok := deskkit.CanonicalDispatchTier(tier)
			if !ok {
				return nil, deskkit.Refused("deskboard dispatch: --tier is outside the tier vocabulary")
			}
			tier = canonical
		}
		return cmdDispatch(hdr, sub, tier)
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
	staleStateInSync        = "in-sync"        // checked: running binary == the version this checkout pins/tracks
	staleStateDrift         = "drift"          // checked: they differ
	staleStateNotApplicable = "not-applicable" // unpinned build: there is no install to drift
	staleStateUnknown       = "unknown"        // COULD-NOT-CHECK: no pin found AND no in-tree ref
	// staleStateHotfix: checked — the running binary is a valid local hotfix over this
	// checkout's pin (its base == the pin, unexpired). Not stale, but never silent: the
	// header names it on every board read.
	staleStateHotfix = "hotfix"
)

// hotfixNow is the clock the hotfix arm reads expiry against; a seam for tests only.
var hotfixNow = time.Now

// hotfixStaleState classifies a running hotfix tag against the checkout's pin: base == pin
// and unexpired is `hotfix` (stale=false, a HOTFIX: header); anything else is drift naming
// the hotfix, which leaves the board BLIND exactly as any other drift does.
func hotfixStaleState(running, base, pinTag, pinArtifact, pinFile string) (string, bool, string) {
	expires, err := time.Parse(time.RFC3339, deskkit.HotfixExpires)
	if err != nil {
		return staleStateDrift, true, "desk-tools hotfix " + running + " carries no readable expiry (" +
			deskkit.HotfixExpires + ") — run `cellctl hotfix status`; the pin is " + pinTag + " in " + pinFile
	}
	exp := expires.UTC().Format(time.RFC3339)
	if !hotfixNow().Before(expires) {
		return staleStateDrift, true, "desk-tools hotfix " + running + " expired " + exp +
			" — regenerate the shims (`cellctl up`) to fall back to pin " + pinTag + " (" + pinFile + ")"
	}
	if normalizeTag(base) != normalizeTag(pinTag) {
		return staleStateDrift, true, "desk-tools hotfix " + running + " outlived its pin: built over " + base +
			", " + pinFile + " pins " + pinArtifact + " " + pinTag + " — regenerate the shims (`cellctl up`)"
	}
	return staleStateHotfix, false, "HOTFIX: desk-tools " + running + " over pin " + pinTag + " (expires " + exp + ")"
}

// staleState reports installed-vs-tracked drift as a three-state verdict plus the
// authoritative boolean.
//
// The boolean fails CLOSED: an `unknown` (the check could not run) reports
// stale=true, because the one answer a drift detector must never give is "fresh" when
// it did not look. `not-applicable` is the separate, honest case — an unpinned `go run`
// has no installed binary that COULD have drifted, which is a statement about the
// world, not a failed measurement, so it reports stale=false and says so by name. The
// two are kept apart rather than both folded into "not assessable", which is what made
// the old detail line contradict its own boolean.
//
// #185 — the PRIMARY drift source is the consumer's `.assay-versions` pin, not the
// in-tree `tools/desk` git ref. Once the desk tools moved out of the consumer's tree
// to their release home, `git rev-parse origin/main:tools/desk` stopped resolving in
// any consumer checkout, so the old in-tree-only check went permanently
// STALE-UNKNOWN there — it could never distinguish "the binary is behind the source"
// from "the source is not in this tree". The running binary embeds its own
// `releaseTag`; comparing that against the `desk-tools` tag the consumer pins in its
// own `.assay-versions` resolves WHERE the binary actually runs. The in-tree ref is
// kept as a FALLBACK for the source-repo case (assay's own tree, where tools/desk
// still exists), and only a run that finds NEITHER source reports could-not-check.
//
// isPinned, gitTree and deskToolsPin are vars for the same reason searchOpenPRs and
// forgeFor already are: the MEASURING outcomes (drift / in-sync) are the ones a pinned
// install actually runs, and with the real git/filesystem calls hard-wired neither
// could be driven from a test — the drift branch could be mutated back to "in-sync,
// stale=false" (i.e. #236 itself, on the live path) with the whole suite still green.
// Seams, not behaviour: production still calls deskkit.IsPinned, the real git, and the
// real `.assay-versions` walk.
var (
	isPinned           = deskkit.IsPinned
	gitTree            = gitTreeReal
	deskToolsPin       = deskToolsPinReal
	deskToolsSourcePin = deskToolsSourcePinReal
	originPin          = originPinReal
)

func staleState() (state string, stale bool, detail string) {
	if !isPinned() {
		return staleStateNotApplicable, false,
			"unpinned build (go run) — there is no installed binary to drift; install via `sudo make desk-install`"
	}

	// PRIMARY (#185): the running binary's release tag vs the desk-tools tag this
	// checkout pins in its own `.assay-versions`. This is the check that resolves in
	// a consumer, where the desk tools no longer live in the tree.
	if pinRoot, pinTag, pinArtifact, found := deskToolsPin(); found {
		running := deskkit.ReleaseTag
		if running != "" {
			pinFile := filepath.Join(pinRoot, deskkit.AssayVersionsFile)
			if base, ok := deskkit.HotfixBase(running); ok {
				return hotfixStaleState(running, base, pinTag, pinArtifact, pinFile)
			}
			if normalizeTag(running) == normalizeTag(pinTag) {
				return staleStateInSync, false,
					"in sync with " + pinFile + " (" + pinArtifact + " " + pinTag + ")"
			}
			top, originTag, _, originFound := originPin(pinRoot, pinArtifact)
			return staleStateDrift, true, driftDetail(driftSides{
				kind:        "releaseTag",
				artifact:    pinArtifact,
				installed:   running,
				worktree:    pinTag,
				origin:      originTag,
				originFound: originFound,
				pinFile:     pinFile,
				top:         top,
				same:        func(a, b string) bool { return normalizeTag(a) == normalizeTag(b) },
			})
		}
		// Pinned by sourceSHA/builtAt but carrying no releaseTag stamp (an older
		// stamped binary): the tag comparison cannot run. Fall through to the
		// channel-D source pin, then the in-tree ref, then could-not-check.
	}

	// CHANNEL-D (#776): a GitLab / native-Windows adopter pins the desk-tools
	// SOURCE line — `desk-tools-source <tag> <40-hex-commit>` — the same shape
	// `desksourceguard` reads, rather than a `desk-tools` release line, and its
	// consumer checkout carries no in-tree `tools/desk` ref either. That line IS
	// a real pin, so treating it as absent (which sent every such sweep to
	// could-not-check, and pinned reviewloop's idle gate there) was #185's
	// fallback landing short. Bind the running binary's stamped sourceSHA to the
	// pinned commit exactly as desksourceguard's third agreement does: the stamp
	// is a short SHA, so the full pinned commit must have it as a prefix. We are
	// inside isPinned(), so deskkit.SourceSHA is non-empty here.
	if pinRoot, pinCommit, found := deskToolsSourcePin(); found {
		if src := deskkit.SourceSHA; src != "" && isFullCommitSHA(pinCommit) {
			pinFile := filepath.Join(pinRoot, deskkit.AssayVersionsFile)
			if strings.HasPrefix(pinCommit, src) {
				return staleStateInSync, false,
					"in sync with " + pinFile + " (desk-tools-source " + pinCommit + ")"
			}
			top, originTag, originSHA, originFound := originPin(pinRoot, "desk-tools-source")
			originCommit := originSHA
			if !isFullCommitSHA(originCommit) && isFullCommitSHA(originTag) {
				originCommit = originTag // the channel-D `<commit> channel-D` column layout
			}
			return staleStateDrift, true, driftDetail(driftSides{
				kind:        "sourceSHA",
				artifact:    "desk-tools-source",
				installed:   src,
				worktree:    pinCommit,
				origin:      originCommit,
				originFound: originFound && isFullCommitSHA(originCommit),
				pinFile:     pinFile,
				top:         top,
				// The installed stamp is a SHORT sha; a pin holds the full commit.
				same: func(installed, pin string) bool { return installed != "" && strings.HasPrefix(pin, installed) },
			})
		}
		// A source line whose commit is not a full 40-hex SHA cannot bind the
		// stamp; do not invent a verdict from it. Fall through to the in-tree ref,
		// then could-not-check — malformed-pin detection is `deskpins --check`'s job.
	}

	// FALLBACK: the in-tree `tools/desk` git ref — the source-repo case, where the
	// desk tools still live in the tree. Only meaningful when origin/main's
	// tools/desk path resolves; a consumer checkout falls straight through to
	// could-not-check.
	src, _ := deskkit.Version()
	installedTree, err1 := gitTree(src)
	// FULLY-QUALIFIED remote-tracking ref, not the bare short name `origin/main`
	// (#885): a stray local `refs/heads/origin/main` decoy would otherwise shadow
	// the real remote tip and make the drift check compare against a stale tree.
	originTree, err2 := gitTree("refs/remotes/origin/main")
	if err1 != nil || err2 != nil || installedTree == "" || originTree == "" {
		return staleStateUnknown, true,
			"COULD-NOT-CHECK drift — reported as STALE because an unverifiable drift check is not " +
				"evidence of freshness (#236). Neither source resolved: no readable `desk-tools` pin in a " +
				"`.assay-versions` at or above the working directory, and the in-tree " +
				"`git rev-parse refs/remotes/origin/main:tools/desk` ref is missing. Re-run where the " +
				"consumer's `.assay-versions` pins desk-tools, or from the source tree where tools/desk exists"
	}
	if installedTree != originTree {
		return staleStateDrift, true,
			"installed sourceSHA " + src + " tools/desk tree differs from origin/main — reinstall (sudo make desk-install)"
	}
	return staleStateInSync, false, "in sync with origin/main"
}

// driftSides is one measured drift, all three sides named: the release the running
// binary carries, the pin in THIS worktree's `.assay-versions`, and the pin on
// origin/main. `same` is the equality the kind uses (tag normalisation for a release
// tag, short-sha prefix for a source commit); originFound=false is origin/main's side
// reported as could-not-check, never as agreement with either.
type driftSides struct {
	kind, artifact      string
	installed, worktree string
	origin              string
	originFound         bool
	pinFile, top        string
	same                func(installed, pin string) bool
}

// driftDetail renders a drift verdict that says WHICH side is stale (#1157).
//
// Two sides are never enough. "installed vs this worktree's pin" is a true statement
// about a role worktree three days behind main and about a binary nobody reinstalled,
// and the old message answered both with `sudo make desk-install` — which, in the live
// case, told the operator to reinstall the CURRENT side while the stale side (the
// worktree) stayed stale and the loop refused every flip for a day. So the message names
// all three, and the shim reinstall is recommended ONLY when the installed release is the
// one behind main's pin. A worktree behind main gets the merge line; a three-way
// disagreement gets the merge FIRST, then the reinstall; an unreadable origin/main is
// reported as could-not-check with no reinstall recommended, because a recommendation
// with no evidence for which side it repairs is the message this replaces.
func driftDetail(d driftSides) string {
	var b strings.Builder
	fmt.Fprintf(&b, "installed desk-tools %s %s; this worktree's %s pin %s (%s); origin/main's %s pin ",
		d.kind, d.installed, d.artifact, d.worktree, d.pinFile, d.artifact)
	merge := "git -C " + d.top + " merge refs/remotes/origin/main"
	if d.top == "" {
		merge = "git merge refs/remotes/origin/main"
	}
	switch {
	case !d.originFound:
		fmt.Fprintf(&b, "could-not-check (no readable refs/remotes/origin/main:%s) — which side is stale cannot "+
			"be established from two sides: `git fetch origin` and re-run; if this worktree is then behind, "+
			"`%s` (merge, never rebase)", deskkit.AssayVersionsFile, merge)
	case d.same(d.installed, d.origin):
		fmt.Fprintf(&b, "%s — the installed release IS main's pin, so this WORKTREE is the stale side (behind "+
			"origin/main). Bring it current: `%s` (merge, never rebase). Do not reinstall.", d.origin, merge)
	case normalizeTag(d.worktree) == normalizeTag(d.origin):
		fmt.Fprintf(&b, "%s — this worktree carries main's pin, so the INSTALLED release is the stale side: "+
			"reinstall the pinned release (sudo make desk-install)", d.origin)
	default:
		fmt.Fprintf(&b, "%s — all three differ: the installed release matches neither pin. Bring this worktree "+
			"current first, `%s` (merge, never rebase), then reinstall to main's pin (sudo make desk-install)",
			d.origin, merge)
	}
	return b.String()
}

// originPinReal reads artifact's pin line from `refs/remotes/origin/main:.assay-versions`
// in the repo pinRoot lives in, so a drift verdict can name main's side alongside the
// worktree's. Fully-qualified ref, never the bare `origin/main` (#885: a stray local
// branch of that name would answer instead). found=false for anything that cannot be
// read — no repo, no ref, no file at that ref, no line — which the caller reports as
// could-not-check, never as a verdict. top is the worktree's toplevel (for the merge
// line the caller prints), or pinRoot when there is no repo to ask.
func originPinReal(pinRoot, artifact string) (top, tag, sha string, found bool) {
	top, err := gitcore.Toplevel(pinRoot)
	if err != nil {
		return pinRoot, "", "", false
	}
	repo, err := gitcore.Open(top)
	if err != nil {
		return top, "", "", false
	}
	rel, err := filepath.Rel(top, pinRoot)
	if err != nil {
		return top, "", "", false
	}
	path := deskkit.AssayVersionsFile
	if rel != "." {
		path = filepath.ToSlash(filepath.Join(rel, deskkit.AssayVersionsFile))
	}
	raw, err := repo.FileAt("refs/remotes/origin/main", path)
	if err != nil {
		return top, "", "", false
	}
	tag, sha, ok, perr := deskkit.ArtifactPinFrom([]byte(raw), "refs/remotes/origin/main:"+path, artifact)
	if perr != nil || !ok {
		return top, "", "", false
	}
	return top, tag, sha, true
}

// normalizeTag strips an optional `<component>/` prefix so the two legitimate pin-tag
// shapes compare equal: the running binary's releaseTag stamp and a consumer pin may
// each be written as the plain `vX.Y.Z` the public release home cuts OR the legacy
// `desk-tools/vX.Y.Z` per-tool form (see deskkit/pins.go artifactTagPattern). Comparing
// the raw strings would report a spurious drift between `v0.18.0` and
// `desk-tools/v0.18.0`, which name the same release.
func normalizeTag(tag string) string {
	if i := strings.LastIndex(tag, "/"); i >= 0 {
		return tag[i+1:]
	}
	return tag
}

// deskToolsPinReal locates the consumer's `.assay-versions` — walking up from the
// working directory — and returns the `desk-tools` pin tag it carries. found=false
// means either no `.assay-versions` was located at or above the working directory (the
// source-repo case: assay's own tree never carries one) OR the nearest one has no
// readable `desk-tools` line; both fall through to staleState's in-tree fallback rather
// than being reported as a failure here, per #185's stated fallback order (fall back to
// the in-tree ref "when it exists", could-not-check only when neither source resolves).
// Malformed-pin detection is `deskpins --check`'s job, not the drift banner's.
//
// THREE ARTIFACT NAMES, TRIED IN ORDER, NONE MATCHED LOOSELY. The pin contract admits three
// legitimate spellings of the same pin: the bare `desk-tools` line, the per-platform
// `desk-tools-<os>-<arch>` line `docs/distribution.md` tells an adopter to write and that
// `deskinstall` resolves by `runtime.GOOS + "-" + runtime.GOARCH`, and the
// `desk-tools-<os>-<arch>.tar.gz` line the installer writes under the tarball's full asset
// name (#4842; deskkit.DeskToolsPlatformArtifacts holds the per-platform list). Only the bare one was ever
// looked up, so a consumer whose pin file carries ONLY the per-platform line fell through
// here (found=false), through the channel-D arm, through the in-tree fallback its checkout
// does not have, and landed on staleState's could-not-check arm — which reports stale=true by
// design, because an unverifiable drift check is not evidence of freshness. The verdict was
// right about what it observed; the observation was the defect.
//
// The fix is a FURTHER LOOKUP WITH AN EXACT NAME per spelling, never a looser match. ArtifactPin selects by
// a trailing-space prefix, and pins.go states plainly that this is why `desk-tools ` must
// never match `desk-tools-linux-amd64 ` — that disambiguation is a control, not an
// inconvenience, and relaxing it would collapse the two lines into one match everywhere in
// the suite. So this asks for each name in full, and the BARE line still wins when more than
// one is present (then the suffixless platform line): an existing consumer's verdict cannot
// change under it.
//
// The matched artifact name is returned so the banner can say which line the verdict came
// from — two sources that can disagree must be distinguishable in the message that reports
// one of them.
func deskToolsPinReal() (root, tag, artifact string, found bool) {
	dir := nearestPinRoot()
	if t, name, ok := deskkit.DeskToolsReleasePin(dir, deskPlatform); ok {
		return dir, t, name, true
	}
	return "", "", "", false // no pin file, or no usable desk-tools line of any shape
}

// deskToolsArtifact is the bare artifact name, tried first so a consumer that already
// carries it keeps exactly today's verdict. The lookup itself is deskkit's, shared with
// deskboot's first-sweep check (#2986), so the boot and the board read one pin the same way.
var deskToolsArtifact = deskkit.DeskToolsArtifact

// deskPlatform is the host's `<os>-<arch>` token. It is a var for the same reason isPinned
// and gitTree are: the per-platform fallback has to be exercisable for a platform the test
// host is not, or the only arm anyone ever runs is the one that happens to match.
// Production reads runtime.
var deskPlatform = runtime.GOOS + "-" + runtime.GOARCH

// deskToolsPlatformArtifact spells the per-platform pin line's artifact name. It returns ""
// when the platform token is empty, so the caller skips the lookup rather than asking for
// `desk-tools-` and matching a line that merely starts that way.
func deskToolsPlatformArtifact() string { return deskkit.DeskToolsPlatformArtifact(deskPlatform) }

// deskToolsSourcePinReal is the channel-D sibling of deskToolsPinReal (#776): it
// returns the COMMIT sha of a `desk-tools-source` line in the nearest
// `.assay-versions`, the pin a GitLab / native-Windows adopter writes in place of
// a `desk-tools` release line. found=false means no pin file, or one with no
// readable `desk-tools-source` line — both fall through to staleState's in-tree
// fallback, never a failure here (malformed-pin detection is `deskpins --check`'s
// job, matching deskToolsPinReal). The trailing-space prefix match inside
// ArtifactPin keeps `desk-tools-source ` from matching `desk-tools-source-notes`.
//
// TWO LEGITIMATE COLUMN LAYOUTS (#795 §3). A `desk-tools-source` line can carry
// its 40-hex commit in EITHER column:
//
//   - `desk-tools-source <tag> <40-hex-commit>` — commit in field 3 (the shape
//     the #776 tests and CheckPins' `-source` rule assume);
//   - `desk-tools-source <40-hex-commit> channel-D` — commit in field 2, with a
//     literal `channel-D` channel marker in field 3, the shape a real adopter's
//     `.assay-versions` (and `desksourceguard`) actually writes.
//
// That interpretation is deskkit.SourcePin's and is not restated here (#1122): it
// is the ONE reader of a source line, shared with the statusgen pin the board's
// Next-up selection resolves, so the two can never drift into disagreeing about
// what the same line says. Field 3 is preferred when it is a full commit (no
// regression for the #776 shape), then field 2. When NEITHER column holds a 40-hex
// commit, field 3 is returned unchanged so staleState's own isFullCommitSHA guard
// falls the run through to the in-tree ref / could-not-check exactly as before —
// a malformed pin never manufactures a verdict here.
func deskToolsSourcePinReal() (root, commit string, found bool) {
	dir := nearestPinRoot()
	if dir == "" {
		return "", "", false
	}
	ref, hit, err := deskkit.SourcePin(dir, deskToolsArtifact)
	if err != nil || !hit {
		return "", "", false // pin file present but no usable desk-tools-source line
	}
	if ref.Commit != "" {
		return dir, ref.Commit, true
	}
	return dir, ref.Field3, true
}

// nearestPinRoot walks up from the working directory and returns the first
// directory carrying a `.assay-versions`, or "" if none exists at or above it
// (the source-repo case: assay's own tree never carries one). It stops at the
// FIRST pin file — a nearer file with no matching line is not skipped in favour
// of a farther one, matching #185's stated "nearest `.assay-versions`" order.
func nearestPinRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return deskkit.NearestPinRoot(dir)
}

// isFullCommitSHA reports whether s is a full 40-hex git commit id — the shape a
// `desk-tools-source` pin's digest column must hold to bind the running binary's
// stamped (short) sourceSHA. Anything shorter or non-hex cannot anchor a commit,
// so staleState treats it as unreadable rather than manufacturing a drift verdict
// from it. Kept local (not deskkit's unexported commitPattern) so main.go needs
// no regexp dependency for a fixed-shape check.
func isFullCommitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// gitTreeReal returns the git tree object id of tools/desk at a ref/sha (read-only).
func gitTreeReal(ref string) (string, error) {
	// Toplevel-based, not a plain Open("."): the original `git rev-parse <ref>:path`
	// (no `-C`) resolves from any subdirectory of the repo via git's own upward
	// search, and gitcore.Open requires an exact repo root.
	top, err := gitcore.Toplevel(".")
	if err != nil {
		return "", err
	}
	repo, err := gitcore.Open(top)
	if err != nil {
		return "", err
	}
	return repo.TreeishID(ref, "tools/desk")
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
	case staleStateHotfix:
		fmt.Fprintln(w, h.StaleDetail) // the detail opens with `HOTFIX:`
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
	// Cell-scoped flags (STOP.cell.<cell>[.<loop>]) ride the same banner; an INERT one
	// (unknown loop, malformed or unregistered cell) is tagged so: the first two halt nothing;
	// the unregistered-cell reason names the root looked in and says it may still be live.
	if len(h.StopFlags) > 0 {
		for _, f := range h.StopFlags {
			tag := "STOP"
			switch {
			case f.Stale:
				tag = "STALE"
			case f.Inert:
				tag = "INERT STOP"
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
