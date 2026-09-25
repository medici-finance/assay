package main

// inbound.go — `deskmonitor inbound`, the Go port of plugins/assay/scripts/inbound-monitor.sh.
//
// The durable, stateful "did anything new arrive?" poll over the open ISSUES of a repo set, carrying
// the three properties a hand-rolled poll keeps losing (each one a way a desk monitor SILENTLY GOES
// BLIND):
//
//	A. EXPLICIT IDENTITY — identity.go.
//	B. PER-SOURCE STATE WITH RETENTION. State is one file per repo, and liveness is judged per
//	   repo: a read that FAILS, returns ZERO where the repo had issues, comes back AT the --limit
//	   (a moving window, not ground truth), or COLLAPSES below the retain floor of its previous
//	   count RETAINS the previous baseline and prints `MONITOR-DEGRADED: <slug> …`. Because the
//	   untrusted read's baseline is retained, the next good cycle diffs against the real baseline
//	   and the outage is absorbed — zero phantom INBOUND events.
//	C. BURST CAP. More than INBOUND_MONITOR_BURST_CAP new keys for one repo in one cycle collapse to
//	   one `INBOUND-BURST: <slug> N over <cap> — listing suppressed` line.
//
// stdout is the contract scanloop's ParseMonitorOutput reads, and it is byte-identical to the
// script's on the same forge answers (parity_test.go). The ONE text that differs by construction is
// the diagnostic inside a failed read's parentheses: the script quotes gh's stderr as `(gh: …)`,
// this verb quotes the forge client's error as `(forge: …)`. The parser treats everything after
// `MONITOR-DEGRADED:` as opaque text, so the difference is invisible to it.
//
// Read-only: one issue-list read per repo per cycle through the resolved forge; writes only its own
// per-repo state files.

import (
	"fmt"
	"io"
)

const inboundUsage = `Usage: deskmonitor inbound [--token-file OWNER=PATH ...] [owner/repo ...]

Stateful open-issue monitor across the given repos (or ./.assay/repos.txt, or the
current repo's origin remote). Meant to be re-run on a cadence behind the harness
Monitor tool or armed by scanloop. On the first run it SEEDS (prints
MONITOR-ARMED: <total>); on every run after that it prints one
INBOUND: <slug>#<num> <updatedAt> per newly seen or updated issue.

It never silently goes blind:
  · it never polls as an inherited token (GH_TOKEN/GITHUB_TOKEN are unset): a
    repo whose owner was given --token-file is read as that file's token, every
    other repo as this session's App token (resolved from $DESK_LOOP);
  · a repo whose read fails, or which returns zero when it previously had issues,
    RETAINS its previous state and prints MONITOR-DEGRADED: <slug> ...;
  · a read that comes back AT the --limit is TRUNCATED, so it is treated as
    could-not-check: retain + go loud;
  · a read that collapses below the retain floor of its previous count is a
    partial read: retain + go loud, so the recovery cycle absorbs it;
  · a burst of more than the cap new items for one repo collapses to a single
    INBOUND-BURST: <slug> N over <cap> — listing suppressed line.

Options:
  --token-file OWNER=PATH     read OWNER's repos as the installation token held in
                              PATH (an owner-only file, read and never copied).
                              Repeatable, one per owner. An unusable file is a
                              precondition failure (exit 1), never a fallback.

Environment:
  INBOUND_MONITOR_STATE_DIR    per-repo state (default <temp dir>/assay-inbound-monitor).
  INBOUND_MONITOR_LIMIT        per-repo read cap (default 500).
  INBOUND_MONITOR_BURST_CAP    new-items-per-repo-per-cycle listing cap (default 25).
  INBOUND_MONITOR_RETAIN_FLOOR percent-of-previous below which a read is partial
                               (retain + degrade); 0 disables (default 50).
  ASSAY_MONITOR_PACE_SECONDS   seconds slept between repo reads (default 2).

Exit codes:
  0  every repo polled cleanly (armed, quiet, or emitted INBOUND lines)
  1  precondition failure — no repos resolvable, unusable state dir, bad knob,
     malformed or unusable --token-file
  2  at least one repo went DEGRADED (read failed, truncated, collapsed or
     rate-limited) — state RETAINED
`

// inboundConfig is one inbound cycle's resolved inputs.
type inboundConfig struct {
	stateDir    string
	limit       int
	burstCap    int
	retainFloor int
	pace        int
}

// fetchIssueKeys reads one repo's open issues and renders them as the keyset the state file holds
// — "<slug>#<num> <updatedAt>" — plus the RAW count the forge returned. It is a package var so the
// property tests can drive a cycle without a backend; production reads through forgeFor.
var fetchIssueKeys = func(repo string) ([]string, error) {
	f, fr, err := forgeFor(repo)
	if err != nil {
		return nil, err
	}
	issues, err := f.ListOpenIssues(fr)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(issues))
	for _, is := range issues {
		ts := is.UpdatedAt
		if ts == "" {
			ts = "null" // jq's rendering of an absent updatedAt — the script's key for the same answer
		}
		keys = append(keys, fmt.Sprintf("%s#%d %s", repo, is.Number, ts))
	}
	return keys, nil
}

func runInbound(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		fmt.Fprint(stdout, inboundUsage)
		return 0
	}
	fail := func(err error) int {
		fmt.Fprintf(stderr, "deskmonitor inbound: %s\n", diagnostic(err))
		return 1
	}

	var cfg inboundConfig
	var err error
	if cfg.limit, err = knob("INBOUND_MONITOR_LIMIT", "LIMIT", 500); err != nil {
		return fail(err)
	}
	if cfg.burstCap, err = knob("INBOUND_MONITOR_BURST_CAP", "BURST_CAP", 25); err != nil {
		return fail(err)
	}
	if cfg.retainFloor, err = knob("INBOUND_MONITOR_RETAIN_FLOOR", "RETAIN_FLOOR", 50); err != nil {
		return fail(err)
	}
	if cfg.pace, err = knob("ASSAY_MONITOR_PACE_SECONDS", "PACE", 2); err != nil {
		return fail(err)
	}

	files, repoArgs, err := splitTokenFiles(args)
	if err != nil {
		return fail(err)
	}
	tokens, err := readTokenFiles(files)
	if err != nil {
		return fail(err)
	}
	explicitTokens = tokens

	repos := resolveRepos(repoArgs)
	if len(repos) == 0 {
		return fail(precondition("no repos to query (no repo args, no ./.assay/repos.txt, and no git origin remote found)"))
	}
	if err := validateRepos(repos); err != nil {
		return fail(err)
	}
	cfg.stateDir = stateDirFrom("INBOUND_MONITOR_STATE_DIR", "assay-inbound-monitor")
	if err := prepareStateDir(cfg.stateDir); err != nil {
		return fail(err)
	}

	degraded, err := inboundCycle(cfg, repos, stdout)
	if err != nil {
		return fail(err)
	}
	if degraded {
		return 2
	}
	return 0
}

// inboundCycle is one poll over repos. It returns whether any repo went DEGRADED; an error is only
// a local state-file write failure (the baseline could not be recorded), which is a precondition
// failure, never a silent skip.
func inboundCycle(cfg inboundConfig, repos []string, out io.Writer) (bool, error) {
	// An ARM run only if NOT ONE resolved repo already has state. A repo added to an armed set
	// seeds silently — never a re-flood on expansion.
	armedRun := true
	for _, r := range repos {
		if hasState(cfg.stateDir, r) {
			armedRun = false
			break
		}
	}

	degraded := false
	armedTotal := 0
	reads := 0
	for i, repo := range repos {
		// PACING: between consecutive reads — never before the first.
		if reads > 0 && cfg.pace > 0 {
			sleepFn(secondsOf(cfg.pace))
		}
		keys, rerr := fetchIssueKeys(repo)
		reads++

		// STOP-ON-LIMIT: this repo and every one after it are skipped (baselines retained) and the
		// cycle ends without another read.
		if rerr != nil && isRateLimited(rerr) {
			degraded = true
			for _, r := range repos[i:] {
				fmt.Fprintf(out, "MONITOR-DEGRADED: %s rate-limited, skipped\n", r)
			}
			break
		}

		readOK := rerr == nil
		var cur []string
		curN, atLimit := 0, false
		if readOK {
			// The forge client reads the WHOLE open set; the script's `gh --limit` keeps only the
			// newest LIMIT. A set at or past the limit is reported exactly as the script sees it —
			// LIMIT rows, TRUNCATED — so the fail-closed rule and its line are the same.
			cur = sortC(keys)
			curN = len(cur)
			if curN >= cfg.limit {
				atLimit = true
				curN = cfg.limit
			}
		}

		sf := statePath(cfg.stateDir, repo)
		if !hasState(cfg.stateDir, repo) {
			// SEED — first sight of this repo: record silently, never emit INBOUND.
			switch {
			case readOK && atLimit:
				degraded = true
				fmt.Fprintf(out, "MONITOR-DEGRADED: %s seed returned %d == --limit %d — results TRUNCATED, no baseline established; raise INBOUND_MONITOR_LIMIT and retry\n",
					repo, curN, cfg.limit)
			case readOK:
				if err := writeState(cfg.stateDir, repo, cur); err != nil {
					return degraded, precondition("cannot write state file '%s': %v", sf, err)
				}
				armedTotal += curN
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
			fmt.Fprintf(out, "MONITOR-DEGRADED: %s read FAILED (forge: %s) — keeping its previous %d issue(s)\n",
				repo, diagnostic(rerr), prevN)
			continue
		case curN == 0 && prevN > 0:
			// Went empty on a "successful" read — a repo is never allowed to go empty silently.
			degraded = true
			fmt.Fprintf(out, "MONITOR-DEGRADED: %s returned 0 (had %d) — keeping its previous %d issue(s)\n",
				repo, prevN, prevN)
			continue
		case atLimit:
			degraded = true
			fmt.Fprintf(out, "MONITOR-DEGRADED: %s returned %d == --limit %d — results TRUNCATED, treating as could-not-check; keeping its previous %d issue(s)\n",
				repo, curN, cfg.limit, prevN)
			continue
		case cfg.retainFloor > 0 && prevN > 0 && curN*100 < prevN*cfg.retainFloor:
			// PARTIAL read — non-empty, but collapsed under the floor of the prior count.
			degraded = true
			fmt.Fprintf(out, "MONITOR-DEGRADED: %s returned %d, under %d%% of its previous %d — treating as a partial read; keeping its previous %d issue(s)\n",
				repo, curN, cfg.retainFloor, prevN, prevN)
			continue
		}

		// Healthy read. New = current keys absent from the retained baseline: a brand-new issue,
		// or an existing one whose updatedAt moved (a new comment).
		base := make(map[string]bool, prevN)
		for _, l := range prev {
			base[l] = true
		}
		var fresh []string
		for _, k := range cur {
			if !base[k] {
				fresh = append(fresh, k)
			}
		}
		switch {
		case len(fresh) > cfg.burstCap:
			fmt.Fprintf(out, "INBOUND-BURST: %s %d over %d — listing suppressed\n", repo, len(fresh), cfg.burstCap)
		default:
			for _, k := range fresh {
				fmt.Fprintf(out, "INBOUND: %s\n", k)
			}
		}
		if err := writeState(cfg.stateDir, repo, cur); err != nil {
			return degraded, precondition("cannot write state file '%s': %v", sf, err)
		}
	}

	if armedRun && !degraded {
		fmt.Fprintf(out, "MONITOR-ARMED: %d\n", armedTotal)
	}
	return degraded, nil
}
