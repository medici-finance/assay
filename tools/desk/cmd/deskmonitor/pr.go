package main

// pr.go — `deskmonitor pr`, the Go port of plugins/assay/scripts/pr-monitor.sh.
//
// The review desk's "did any open PR change?" poll: the PR-side twin of `deskmonitor inbound`, with
// the same repo resolution, per-repo state files, seed-silently rule, truncation guard and
// MONITOR-ARMED / MONITOR-DEGRADED vocabulary, plus PACING (the property one 16-repo tight-loop
// poller taught: ASSAY_MONITOR_PACE_SECONDS between reads, ASSAY_MONITOR_MAX_REPOS_PER_CYCLE with a
// cursor carried in the state dir, and stop-on-rate-limit).
//
// Unlike the issue monitor there is NO zero-where-it-had-some guard: an open-PR queue draining to
// zero is a normal steady state for a review desk, and a PR leaving the set is a `closed` PR-EVENT.
//
// ONE DELIBERATE DIVERGENCE FROM THE ORACLE. pr-monitor.sh diffs with an awk `NR==FNR` idiom, and
// when a repo's baseline file is EMPTY (seeded while it had no open PRs) that idiom reads the
// CURRENT set as the baseline: every newly opened PR is then reported as `closed` and none as
// `opened`. This verb reports them as `opened`. parity_test.go pins the divergence explicitly (the
// oracle's wrong lines and this verb's right ones), so it is a stated difference, not a drift.
//
// The oracle also prints `closed` events in awk's hash order, which is unspecified; this verb
// prints them in ascending PR-number order.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const prUsage = `Usage: deskmonitor pr [owner/repo ...]

Stateful open-PR monitor across the given repos (or ./.assay/repos.txt, or the
current repo's origin remote). Meant to be re-run on a cadence behind the harness
Monitor tool. On the first run it SEEDS each repo silently (prints
MONITOR-ARMED: <n> repos (pace <p>s, cap <c>)); on every run after that it
prints one line per change:

  PR-EVENT: <slug>#<num> <kind> <old> -> <new>
    kind ∈ opened | pushed | draft-flip | state | merge-state | closed

It never silently goes blind:
  · it never polls as an inherited token (GH_TOKEN/GITHUB_TOKEN are unset); it
    reads as this session's App token (resolved from $DESK_LOOP);
  · a repo whose read fails RETAINS its previous state and prints
    MONITOR-DEGRADED: <slug> ...;
  · a read that comes back AT the --limit is TRUNCATED: retain + go loud.

It never floods the forge:
  · ASSAY_MONITOR_PACE_SECONDS is slept between consecutive repo reads;
  · ASSAY_MONITOR_MAX_REPOS_PER_CYCLE caps a cycle and carries a cursor forward;
  · a rate-limit answer ends the cycle without further reads, marking every
    remaining repo MONITOR-DEGRADED: <slug> rate-limited, skipped.

Environment:
  PR_MONITOR_STATE_DIR               per-repo state (default <temp dir>/assay-pr-monitor).
  PR_MONITOR_LIMIT                   per-repo read cap (default 100).
  ASSAY_MONITOR_PACE_SECONDS         seconds slept between repo reads (default 2).
  ASSAY_MONITOR_MAX_REPOS_PER_CYCLE  repos read per cycle, 0 = all (default 0).

Exit codes:
  0  every repo read cleanly (armed, quiet, or emitted PR-EVENT lines)
  1  precondition failure — no repos resolvable, unusable state dir, bad knob
  2  at least one repo went DEGRADED (read failed, truncated, or rate-limited) — state RETAINED
`

// prCursorFile carries the next cycle's starting index when a per-cycle cap is in force.
const prCursorFile = ".cursor"

type prConfig struct {
	stateDir string
	limit    int
	pace     int
	maxRepos int
}

// prSnapshot is one repo's open-PR read: the rows in the order the forge returned them (newest
// first), and the forge's own page cap when the read came back AT it (0 = not truncated by the
// forge). The page cap matters only when it sits BELOW the configured limit.
type prSnapshot struct {
	rows       []prRow
	forgeCapAt int
}

type prRow struct {
	number int
	line   string // "<num> <head-sha> <draft|ready> <state> <mergeState>" — the state-file line
}

// fetchPRRows reads one repo's open PRs through the resolved forge. A package var so the property
// tests can drive a cycle without a backend.
var fetchPRRows = func(repo string) (prSnapshot, error) {
	f, fr, err := forgeFor(repo)
	if err != nil {
		return prSnapshot{}, err
	}
	oc, err := f.ListOpenChanges(fr)
	if err != nil {
		return prSnapshot{}, err
	}
	snap := prSnapshot{}
	if oc.TruncatedAtCap {
		snap.forgeCapAt = oc.Cap
	}
	for _, c := range oc.Changes {
		draft := "ready"
		if c.Draft {
			draft = "draft"
		}
		snap.rows = append(snap.rows, prRow{number: c.Number,
			line: fmt.Sprintf("%d %s %s %s %s", c.Number, orNull(c.HeadSHA), draft, orNull(c.State), c.MergeStateStatus)})
	}
	return snap, nil
}

// orNull is jq's rendering of an absent string field, so a key matches the script's for the same answer.
func orNull(s string) string {
	if s == "" {
		return "null"
	}
	return s
}

func runPR(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(stdout, prUsage)
		return 0
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "deskmonitor pr: %s\n", diagnostic(err))
		return 1
	}
	var cfg prConfig
	var err error
	if cfg.limit, err = knob("PR_MONITOR_LIMIT", "LIMIT", 100); err != nil {
		return fail(err)
	}
	if cfg.pace, err = knob("ASSAY_MONITOR_PACE_SECONDS", "PACE", 2); err != nil {
		return fail(err)
	}
	if cfg.maxRepos, err = knob("ASSAY_MONITOR_MAX_REPOS_PER_CYCLE", "MAX_REPOS", 0); err != nil {
		return fail(err)
	}
	explicitTokens = map[string]string{}
	repos := resolveRepos(args)
	if len(repos) == 0 {
		return fail(precondition("no repos to query (no repo args, no ./.assay/repos.txt, and no git origin remote found)"))
	}
	if err := validateRepos(repos); err != nil {
		return fail(err)
	}
	cfg.stateDir = stateDirFrom("PR_MONITOR_STATE_DIR", "assay-pr-monitor")
	if err := prepareStateDir(cfg.stateDir); err != nil {
		return fail(err)
	}
	degraded, err := prCycle(cfg, repos, stdout)
	if err != nil {
		return fail(err)
	}
	if degraded {
		return 2
	}
	return 0
}

var cursorRe = regexp.MustCompile(`^[0-9]+$`)

func prCycle(cfg prConfig, repos []string, out io.Writer) (bool, error) {
	n := len(repos)
	cursorPath := filepath.Join(cfg.stateDir, prCursorFile)

	// This cycle's slice: the cap + the carried cursor. A fresh, out-of-range or missing cursor
	// restarts at 0.
	start := 0
	if cfg.maxRepos > 0 {
		if b, err := os.ReadFile(cursorPath); err == nil {
			first, _, _ := strings.Cut(string(b), "\n")
			if cursorRe.MatchString(first) {
				if c, cerr := strconv.Atoi(first); cerr == nil {
					start = c
				}
			}
		}
		if start >= n {
			start = 0
		}
	}
	var proc []string
	nextCursor := 0
	if cfg.maxRepos == 0 {
		proc = repos
	} else {
		i := start
		for taken := 0; taken < cfg.maxRepos && i < n; taken++ {
			proc = append(proc, repos[i])
			i++
		}
		// No wrap within a cycle: the next one resumes where this stopped (0 at the end).
		nextCursor = i
		if nextCursor >= n {
			nextCursor = 0
		}
	}

	armedRun := true
	for _, r := range proc {
		if hasState(cfg.stateDir, r) {
			armedRun = false
			break
		}
	}

	degraded := false
	armedTotal := 0
	reads := 0
	for idx, repo := range proc {
		if reads > 0 && cfg.pace > 0 {
			sleepFn(secondsOf(cfg.pace))
		}
		snap, rerr := fetchPRRows(repo)
		reads++

		if rerr != nil && isRateLimited(rerr) {
			degraded = true
			for _, r := range proc[idx:] {
				fmt.Fprintf(out, "MONITOR-DEGRADED: %s rate-limited, skipped\n", r)
			}
			// Next cycle resumes at the tripped repo so the skipped tail is retried.
			if cfg.maxRepos > 0 {
				nextCursor = start + idx
				if nextCursor >= n {
					nextCursor = 0
				}
			}
			break
		}

		readOK := rerr == nil
		var cur []prRow
		curN, atLimit, forgeCapped := 0, false, false
		if readOK {
			rows := snap.rows
			if len(rows) > cfg.limit {
				rows = rows[:cfg.limit] // gh --limit keeps the newest LIMIT; the forge returns newest first
			}
			cur = append([]prRow(nil), rows...)
			sort.SliceStable(cur, func(a, b int) bool { return cur[a].number < cur[b].number })
			curN = len(cur)
			atLimit = curN >= cfg.limit
			// A forge page cap BELOW the configured limit truncates the read where gh would not
			// have: the same moving window, so the same fail-closed treatment.
			forgeCapped = !atLimit && snap.forgeCapAt > 0 && snap.forgeCapAt < cfg.limit
		}
		lines := make([]string, len(cur))
		for i, r := range cur {
			lines[i] = r.line
		}

		sf := statePath(cfg.stateDir, repo)
		if !hasState(cfg.stateDir, repo) {
			switch {
			case readOK && atLimit:
				degraded = true
				fmt.Fprintf(out, "MONITOR-DEGRADED: %s seed returned %d == --limit %d — results TRUNCATED, no baseline established; raise PR_MONITOR_LIMIT and retry\n",
					repo, curN, cfg.limit)
			case readOK && forgeCapped:
				degraded = true
				fmt.Fprintf(out, "MONITOR-DEGRADED: %s seed returned %d == the forge page cap %d (below --limit %d) — results TRUNCATED, no baseline established; lower PR_MONITOR_LIMIT to at most %d\n",
					repo, curN, snap.forgeCapAt, cfg.limit, snap.forgeCapAt)
			case readOK:
				if err := writeState(cfg.stateDir, repo, lines); err != nil {
					return degraded, precondition("cannot write state file '%s': %v", sf, err)
				}
				armedTotal++
			default:
				degraded = true
				fmt.Fprintf(out, "MONITOR-DEGRADED: %s seed read FAILED (forge: %s) — no baseline established, will retry next cycle\n",
					repo, diagnostic(rerr))
			}
			continue
		}

		prev, perr := readStateLines(sf)
		if perr != nil {
			return degraded, precondition("cannot read state file '%s': %v", sf, perr)
		}
		prevN := len(prev)
		switch {
		case !readOK:
			degraded = true
			fmt.Fprintf(out, "MONITOR-DEGRADED: %s read FAILED (forge: %s) — keeping its previous %d PR(s)\n",
				repo, diagnostic(rerr), prevN)
			continue
		case atLimit:
			degraded = true
			fmt.Fprintf(out, "MONITOR-DEGRADED: %s returned %d == --limit %d — results TRUNCATED, treating as could-not-check; keeping its previous %d PR(s)\n",
				repo, curN, cfg.limit, prevN)
			continue
		case forgeCapped:
			degraded = true
			fmt.Fprintf(out, "MONITOR-DEGRADED: %s returned %d == the forge page cap %d (below --limit %d) — results TRUNCATED, treating as could-not-check; keeping its previous %d PR(s)\n",
				repo, curN, snap.forgeCapAt, cfg.limit, prevN)
			continue
		}

		for _, ev := range prEvents(repo, prev, lines) {
			fmt.Fprintln(out, ev)
		}
		if err := writeState(cfg.stateDir, repo, lines); err != nil {
			return degraded, precondition("cannot write state file '%s': %v", sf, err)
		}
	}

	if cfg.maxRepos > 0 {
		if err := os.WriteFile(cursorPath, []byte(strconv.Itoa(nextCursor)+"\n"), 0o644); err != nil {
			return degraded, precondition("cannot write cursor file '%s': %v", cursorPath, err)
		}
	}
	if armedRun && !degraded {
		capDesc := strconv.Itoa(cfg.maxRepos)
		if cfg.maxRepos == 0 {
			capDesc = "all"
		}
		fmt.Fprintf(out, "MONITOR-ARMED: %d repos (pace %ds, cap %s)\n", armedTotal, cfg.pace, capDesc)
	}
	return degraded, nil
}

// prFields splits a state line the way awk does (runs of blanks), padded to five fields so a line
// whose trailing merge-state is empty still reads as five.
func prFields(line string) [5]string {
	var f [5]string
	copy(f[:], strings.Fields(line))
	return f
}

// prEvents diffs the current rows against the retained baseline, per PR number, and emits one line
// per CHANGED dimension — a push and a draft-flip on the same PR in one cycle are two lines, never
// collapsed. Order: the current rows in ascending number (opened, or pushed / draft-flip / state /
// merge-state), then every baseline PR absent from the current set as `closed`, ascending.
func prEvents(repo string, baseline, current []string) []string {
	base := map[string][5]string{}
	var baseOrder []string
	for _, l := range baseline {
		f := prFields(l)
		if _, dup := base[f[0]]; !dup {
			baseOrder = append(baseOrder, f[0])
		}
		base[f[0]] = f
	}
	seen := map[string]bool{}
	var out []string
	for _, l := range current {
		c := prFields(l)
		num := c[0]
		seen[num] = true
		b, ok := base[num]
		if !ok {
			out = append(out, fmt.Sprintf("PR-EVENT: %s#%s opened - -> %s", repo, num, c[1]))
			continue
		}
		if b[1] != c[1] {
			out = append(out, fmt.Sprintf("PR-EVENT: %s#%s pushed %s -> %s", repo, num, b[1], c[1]))
		}
		if b[2] != c[2] {
			out = append(out, fmt.Sprintf("PR-EVENT: %s#%s draft-flip %s -> %s", repo, num, b[2], c[2]))
		}
		if b[3] != c[3] {
			out = append(out, fmt.Sprintf("PR-EVENT: %s#%s state %s -> %s", repo, num, b[3], c[3]))
		}
		if b[4] != c[4] {
			out = append(out, fmt.Sprintf("PR-EVENT: %s#%s merge-state %s -> %s", repo, num, b[4], c[4]))
		}
	}
	sort.SliceStable(baseOrder, func(i, j int) bool { return numLess(baseOrder[i], baseOrder[j]) })
	for _, num := range baseOrder {
		if !seen[num] {
			out = append(out, fmt.Sprintf("PR-EVENT: %s#%s closed %s -> -", repo, num, base[num][1]))
		}
	}
	return out
}

func numLess(a, b string) bool {
	ai, aerr := strconv.Atoi(a)
	bi, berr := strconv.Atoi(b)
	if aerr == nil && berr == nil {
		return ai < bi
	}
	return a < b
}
