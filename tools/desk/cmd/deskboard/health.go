package main

// health.go — default-branch health per watched repo (#295).
//
// THE GAP THIS CLOSES. The board reads PR check state and nothing else, so a repo whose
// `main` is broken looks entirely normal: every PR's own check rollup is green, the desk
// reviews/approves/flips against it, and each merge stacks another commit onto a red
// branch. This can happen unobserved: `main` goes red, and a PR is reviewed, approved,
// flipped and merged against it with no signal anywhere on the board.
//
// THREE STATES, NEVER TWO. "main is green" and
// "main's health was never checked" are different facts and are reported as different
// states here. A probe that cannot see gets `unknown` with a reason — never `green`.
// The states:
//
//	green       ≥1 counted check succeeded at the assessed commit, none failing/pending
//	red         a counted check FAILED at the assessed commit
//	pending     checks still running; nothing failed yet (a known state, not an alarm)
//	no-ci       the repo runs no PR CI (deskkit policy) AND no check runs exist — there
//	            is nothing to assess, and saying so is not the same as saying "green"
//	no-commits  the default branch has no commits (empty repo) — nothing to assess
//	unknown     COULD-NOT-CHECK: a read failed, the check-run page cap was reached, or a
//	            CI-running repo produced no check runs anywhere in the lookback window
//
// WHAT `green` DOES AND DOES NOT CLAIM. `green` means "green at the assessed commit, for
// the jobs that actually ran there" — not "every job this repo expects is passing". The
// probe has no per-repo expected-check list, so a commit whose only run is a
// path-filtered job that passed reports green even if the job that last failed simply did
// not re-run on it. That gap is the known, already-tracked "expected-checks per repo"
// class (follow-up: #33; #33's deskboard-specific page-cap half is fixed here by the
// truncation → unknown path). Closing it needs an expected-check set per repo, which is
// that follow-up's job, not this probe's.
//
// NOISE BUDGET. A signal that fires constantly is trained away, which is the same
// outcome as not having it. Three deliberate dampers:
//
//  1. `cancelled` is NOT red. On a default branch, cancellation is overwhelmingly a
//     concurrency-group supersede by the next push, not a breakage. `skipped`/`neutral`
//     are ignored for the same reason PR-side ciState ignores them.
//  2. A head commit with zero check runs is NOT an alarm — path filters mean docs- and
//     status-only commits routinely carry none. The probe walks back up to bhLookback
//     commits to find the most recent commit that CI actually ran on, and reports which
//     commit it assessed and how far behind head that is.
//  3. Only red and unknown print a line each. Everything else collapses into one
//     always-printed summary line, which is what makes silence impossible to confuse
//     with health.
//
// FAIL-CLOSED, NARROWLY. board.go's fail-closed contract fails the WHOLE run (exit 6) when a PR
// read fails, because a partial PR list reads as "nothing open". Branch health is not
// like that: a failed probe does not make the PR rows wrong, and turning every transient
// API hiccup into a dead board is precisely the noise that gets a signal ignored. So a
// failed probe is a loud `unknown` row plus an `unknown` count in the header — visible,
// never silently folded into green — and the board still renders.
//
// SCOPE ANNOUNCEMENT. The summary names the repo set it covered and the count it could
// not assess, so this block can never report "no red branches" while quietly covering a
// subset (the general board-wide version of that property is #359).

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// bhLookback bounds how far back along the default branch the probe walks when the head
// commit carries no check runs. Five is enough to skip a run of docs/status commits
// without turning one board read into an unbounded crawl; a window that finds nothing is
// reported as unknown (with the window size), never assumed green.
const bhLookback = 5

// bhPageCap is the per_page value for the check-runs read. If total_count exceeds what
// came back, the probe reports could-not-check rather than judging a truncated list
// (three-state rule, sub-rule 2 — truncation is a lie).
const bhPageCap = 100

// branch-health states.
const (
	bhGreen     = "green"
	bhRed       = "red"
	bhPending   = "pending"
	bhNoCI      = "no-ci"
	bhNoCommits = "no-commits"
	bhUnknown   = "unknown"
)

// checkRun mirrors one element of the REST check-runs payload. Note the REST API returns
// LOWERCASE status/conclusion ("completed"/"failure") where the gh PR rollup returns
// uppercase — classifyCheckRuns normalises rather than trusting either casing.
type checkRun struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HeadSHA    string `json:"head_sha"`
}

type checkRunsResp struct {
	TotalCount int        `json:"total_count"`
	CheckRuns  []checkRun `json:"check_runs"`
}

// branchHealthRow is one repo's default-branch verdict.
type branchHealthRow struct {
	Repo string `json:"repo"`
	// State is one of the bh* constants. green/red/pending are checked results;
	// no-ci/no-commits are "nothing to check, and here is why"; unknown is
	// COULD-NOT-CHECK and must never be read as green.
	State string `json:"state"`
	// Commit is the sha the verdict was computed at ("" when nothing was assessed).
	Commit string `json:"commit,omitempty"`
	// BehindHead is how many commits back from the branch head Commit sits. >0 means the
	// head itself carried no check runs and the probe fell back to the last commit CI
	// actually ran on.
	BehindHead int `json:"behindHead"`
	// Failing names the failed check runs (red rows only).
	Failing []string `json:"failing,omitempty"`
	// Reason is human-readable and always populated.
	Reason string `json:"reason"`
}

// branchHealthReport is the whole block: per-repo rows, counts, and the scope it covers.
// Scope is carried explicitly so a reader can never mistake "no red branches in the four
// repos I looked at" for "no red branches".
type branchHealthReport struct {
	Scope        []string          `json:"scope"`
	Lookback     int               `json:"lookback"`
	Green        int               `json:"green"`
	Red          int               `json:"red"`
	Pending      int               `json:"pending"`
	NotAssessed  int               `json:"notAssessed"` // no-ci + no-commits
	Unknown      int               `json:"unknown"`     // could-not-check
	RedRepos     []string          `json:"redRepos,omitempty"`
	UnknownRepos []string          `json:"unknownRepos,omitempty"`
	Repos        []branchHealthRow `json:"repos"`
}

// fetchRecentCommits returns up to bhLookback commit shas from the head of repo's
// DEFAULT branch, newest first. Omitting ?sha= makes the API use the default branch, so
// this costs one call and needs no separate "what is the default branch" read.
// An empty repository is a distinct, KNOWN answer (no-commits), not a read failure.
func fetchRecentCommits(repo string) (shas []string, empty bool, err error) {
	out, gerr := ghRun("api", fmt.Sprintf("repos/%s/commits?per_page=%d", repo, bhLookback))
	if gerr != nil {
		if strings.Contains(strings.ToLower(gerr.Error()), "repository is empty") {
			return nil, true, nil
		}
		return nil, false, gerr
	}
	var commits []struct {
		SHA string `json:"sha"`
	}
	if jerr := json.Unmarshal(out, &commits); jerr != nil {
		return nil, false, fmt.Errorf("unparseable commits payload: %v", jerr)
	}
	for _, c := range commits {
		if c.SHA != "" {
			shas = append(shas, c.SHA)
		}
	}
	return shas, len(shas) == 0, nil
}

// fetchCheckRuns returns the check runs for one commit. truncated is true when the API
// reported more runs than it returned — the caller must then report could-not-check
// rather than judge a partial list.
func fetchCheckRuns(repo, sha string) (runs []checkRun, truncated bool, err error) {
	out, gerr := ghRun("api", fmt.Sprintf("repos/%s/commits/%s/check-runs?per_page=%d", repo, sha, bhPageCap))
	if gerr != nil {
		return nil, false, gerr
	}
	var resp checkRunsResp
	if jerr := json.Unmarshal(out, &resp); jerr != nil {
		return nil, false, fmt.Errorf("unparseable check-runs payload: %v", jerr)
	}
	return resp.CheckRuns, resp.TotalCount > len(resp.CheckRuns), nil
}

// classifyCheckRuns reduces a commit's check runs to a verdict.
//
// counted is the number of runs that carried usable evidence — a run that is skipped,
// neutral or cancelled carries none. counted == 0 means "CI did not really run here",
// which is the caller's cue to walk further back, NOT a green verdict.
//
// cancelled is deliberately not red: on a default branch it almost always means the run
// was superseded by the next push (concurrency group), and treating that as a breakage
// would fire the alarm on ordinary back-to-back merges.
func classifyCheckRuns(runs []checkRun) (state string, failing []string, counted int) {
	pending, passed := 0, 0
	for _, r := range runs {
		if !strings.EqualFold(r.Status, "completed") {
			pending++
			counted++
			continue
		}
		switch strings.ToUpper(r.Conclusion) {
		case "SUCCESS":
			passed++
			counted++
		case "SKIPPED", "NEUTRAL", "CANCELLED":
			// no evidence either way — see the doc comment
		default: // FAILURE, TIMED_OUT, STARTUP_FAILURE, ACTION_REQUIRED, STALE
			failing = append(failing, r.Name)
			counted++
		}
	}
	switch {
	case counted == 0:
		return "", nil, 0
	case len(failing) > 0:
		return bhRed, failing, counted
	case pending > 0:
		return bhPending, nil, counted
	case passed > 0:
		return bhGreen, nil, counted
	default:
		return "", nil, 0
	}
}

// assessRepoBranch probes one repo's default branch. It never returns an error: an
// unreadable repo is a first-class `unknown` ROW, so one flaky read degrades exactly one
// line of the board instead of erasing the whole thing (see the fail-closed note above).
func assessRepoBranch(repo string) branchHealthRow {
	row := branchHealthRow{Repo: repo}
	shas, empty, err := fetchRecentCommits(repo)
	if err != nil {
		row.State = bhUnknown
		row.Reason = "COULD-NOT-CHECK: cannot read default-branch commits — " + oneLine(err.Error())
		return row
	}
	if empty {
		row.State = bhNoCommits
		row.Reason = "default branch has no commits (empty repo) — nothing to assess"
		return row
	}
	for i, sha := range shas {
		runs, truncated, ferr := fetchCheckRuns(repo, sha)
		if ferr != nil {
			row.State = bhUnknown
			row.Commit = sha
			row.BehindHead = i
			row.Reason = "COULD-NOT-CHECK: cannot read check runs at " + short(sha) + " — " + oneLine(ferr.Error())
			return row
		}
		if truncated {
			row.State = bhUnknown
			row.Commit = sha
			row.BehindHead = i
			row.Reason = fmt.Sprintf("COULD-NOT-CHECK: check-run list truncated at the per_page=%d cap for %s — "+
				"a failing run may be on an unread page", bhPageCap, short(sha))
			return row
		}
		state, failing, counted := classifyCheckRuns(runs)
		if counted == 0 {
			continue // no CI evidence at this commit — walk back (path filters, docs commits)
		}
		row.State = state
		row.Commit = sha
		row.BehindHead = i
		row.Failing = failing
		switch state {
		case bhRed:
			row.Reason = fmt.Sprintf("default branch is RED at %s%s — failing: %s",
				short(sha), behindNote(i), strings.Join(failing, ", "))
		case bhPending:
			row.Reason = fmt.Sprintf("checks still running at %s%s — no failure yet", short(sha), behindNote(i))
		default:
			row.Reason = fmt.Sprintf("checks green at %s%s", short(sha), behindNote(i))
		}
		return row
	}
	// Nothing in the lookback window carried CI evidence. Which state that is depends on
	// whether this repo is supposed to run CI at all.
	if deskkit.CIRequired(repo) {
		row.State = bhUnknown
		row.Reason = fmt.Sprintf("COULD-NOT-CHECK: no check runs on any of the last %d default-branch commits, "+
			"though this repo runs CI — main health is UNKNOWN, not green", len(shas))
		return row
	}
	row.State = bhNoCI
	row.Reason = fmt.Sprintf("no check runs on the last %d default-branch commits and the repo runs no PR CI "+
		"(deskkit policy) — nothing to assess, which is not the same as green", len(shas))
	return row
}

// behindNote renders the "how stale is this verdict" suffix.
func behindNote(behind int) string {
	if behind == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d commit(s) behind head — head carries no check runs)", behind)
}

// oneLine flattens a multi-line error for single-line board output.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// assessBranchHealth probes every watched repo and tallies the result. Scope is recorded
// from the same deskkit.AllowedRepos() the rest of the board iterates, so the block
// cannot claim coverage it does not have.
func assessBranchHealth() branchHealthReport {
	repos := deskkit.AllowedRepos()
	rep := branchHealthReport{Scope: repos, Lookback: bhLookback, Repos: make([]branchHealthRow, 0, len(repos))}
	// Bounded-concurrency probe (sweep.go). assessRepoBranch never returns an error — an
	// unreadable repo is a first-class `unknown` ROW, not a run failure — so the nil error
	// here is by construction, and the tally below stays identical to the serial version
	// because rep.Repos is re-sorted by rank afterward (finish order cannot leak).
	rows, _ := sweepRepos(repos, sweepConcurrency, func(repo string) (branchHealthRow, error) {
		return assessRepoBranch(repo), nil
	})
	for _, row := range rows {
		rep.Repos = append(rep.Repos, row)
		switch row.State {
		case bhGreen:
			rep.Green++
		case bhRed:
			rep.Red++
			rep.RedRepos = append(rep.RedRepos, row.Repo)
		case bhPending:
			rep.Pending++
		case bhNoCI, bhNoCommits:
			rep.NotAssessed++
		default:
			rep.Unknown++
			rep.UnknownRepos = append(rep.UnknownRepos, row.Repo)
		}
	}
	sort.SliceStable(rep.Repos, func(i, j int) bool {
		// Red first, then unknown, then everything else alphabetically — the board's top
		// line is the one that outranks a merge.
		return branchStateRank(rep.Repos[i].State) < branchStateRank(rep.Repos[j].State) ||
			(branchStateRank(rep.Repos[i].State) == branchStateRank(rep.Repos[j].State) &&
				rep.Repos[i].Repo < rep.Repos[j].Repo)
	})
	return rep
}

func branchStateRank(state string) int {
	switch state {
	case bhRed:
		return 0
	case bhUnknown:
		return 1
	case bhPending:
		return 2
	case bhGreen:
		return 3
	default:
		return 4
	}
}

// redRepoSet returns the set of repos whose default branch is RED, for the per-row
// annotation on `actions`.
func (r branchHealthReport) redRepoSet() map[string]bool {
	set := make(map[string]bool, len(r.RedRepos))
	for _, repo := range r.RedRepos {
		set[repo] = true
	}
	return set
}

// renderAlarms writes the lines that must appear ABOVE everything else on the board
// (#295: a red main outranks a merge). Red and unknown get a line each; every other
// state collapses into the single always-printed summary — so the block is one line on a
// healthy day and cannot be silent on an unhealthy one.
func (r branchHealthReport) renderAlarms(w io.Writer) {
	for _, row := range r.Repos {
		switch row.State {
		case bhRed:
			fmt.Fprintf(w, "MAIN-RED: %s %s — do not stack merges onto it; a fix for main outranks every merge on this repo\n",
				shortRepo(row.Repo), row.Reason)
		case bhUnknown:
			fmt.Fprintf(w, "MAIN-UNKNOWN: %s %s\n", shortRepo(row.Repo), row.Reason)
		}
	}
	fmt.Fprintln(w, r.summaryLine())
}

// summaryLine is the always-printed scope + tally line.
func (r branchHealthReport) summaryLine() string {
	return fmt.Sprintf("main-health: %d green · %d RED · %d pending · %d not-assessed · %d COULD-NOT-CHECK "+
		"— scope: %d watched repo(s), lookback %d commit(s)",
		r.Green, r.Red, r.Pending, r.NotAssessed, r.Unknown, len(r.Scope), r.Lookback)
}

// ---- health subcommand ----

// healthReport carries the probe in the SAME place `actions` does — the embedded
// Header's `mainHealth`. It deliberately does not repeat the report under a second key:
// two keys with identical contents make a consumer choose, and a consumer that chooses
// the one a future change stops filling reads absence as health (#377 review, N2).
type healthReport struct {
	Header
}

func cmdHealth(hdr Header) (*Report, error) {
	bh := assessBranchHealth()
	hdr.MainHealth = &bh
	hdr.Scope = boardScope() // #359: a sweeping verb states its coverage
	rep := healthReport{Header: hdr}
	return &Report{value: rep, detail: bh.auditDetail(), render: func(w io.Writer) {
		fmt.Fprintf(w, "asOf %s\n", hdr.AsOf)
		bh.renderAlarms(w)
		fmt.Fprintf(w, "%-13s %-11s %-10s %-7s %s\n", "REPO", "STATE", "COMMIT", "BEHIND", "REASON")
		for _, row := range bh.Repos {
			fmt.Fprintf(w, "%-13s %-11s %-10s %-7d %s\n",
				shortRepo(row.Repo), strings.ToUpper(row.State), short(row.Commit), row.BehindHead, row.Reason)
		}
		renderScopeLine(w, rep.Header.Scope)
	}}, nil
}

// auditDetail records the verdict in the audit line, so a later run (or a human reading
// the audit) can see that the probe RAN and what it saw — including how many repos it
// could not assess.
func (r branchHealthReport) auditDetail() string {
	d := fmt.Sprintf("mainHealth green=%d red=%d pending=%d notAssessed=%d unknown=%d scope=%d",
		r.Green, r.Red, r.Pending, r.NotAssessed, r.Unknown, len(r.Scope))
	if len(r.RedRepos) > 0 {
		d += " red:" + strings.Join(r.RedRepos, ",")
	}
	if len(r.UnknownRepos) > 0 {
		d += " unknown:" + strings.Join(r.UnknownRepos, ",")
	}
	return d
}
