package main

// doratiming.go — the DORA-timing historian.
//
// Two of the four DORA numbers — change_lead_time and time_to_restore — cannot
// be answered from a point-in-time API read: they need a faithful, reproducible
// SERIES, and history cannot be back-filled from nothing. This file records that
// series and reads it back.
//
// SUBSTRATE. A sibling append-only log, docs/streams/.dora-timing.jsonl, beside
// the brief-status historian's .history.jsonl (history.go). It is deliberately a
// SEPARATE file, not an extra record type inside .history.jsonl: that log has a
// fixed five-key brief-transition schema every register/Next-up reader parses,
// and mixing a foreign record type into it would break those readers. The two
// logs share the same discipline — single-writer, append-only, idempotent,
// format-stable — but never the same file.
//
//   - Single-writer. recordDoraTiming is called ONLY from run()'s mode=="record"
//     branch (main's regen CI), the same single-writer pass as recordHistory —
//     never from the PR-side --lint gate. A branch cannot forge a timing record;
//     the log only grows from what main's own CI observed.
//   - Append-only + idempotent. Each record carries a natural id: failed_run_id
//     for a restore episode, repo+pr for a lead time. A re-run that observes
//     nothing new leaves the file byte-identical; an already-recorded episode/PR
//     is never appended twice.
//   - Format-stable. Add a field (old readers ignore unknown keys); never rename
//     or repurpose an existing key.
//
// PORTABILITY. The recorder is repo-agnostic. It derives its target repo from
// runtime context ($GITHUB_REPOSITORY, else the checkout's own git remote, else
// gh's default) and reads that repo's own workflow_runs and merged PRs — never a
// hardcoded owner/repo and never a house-only workflow name. A bare adopter with
// statusgen in CI gets DORA-timing recording with zero house config. The one
// optional refinement, the workflow that defines red/green for a restore
// episode, defaults to the aggregate per-commit required-check state ("*") and
// fails open to that default; an adopter MAY narrow it to one workflow via
// STATUSGEN_DORA_WORKFLOW, but is never required to.
//
// FAIL-OPEN, NEVER FABRICATE. Every network read that fails records NOTHING and
// prints a could-not-check line; it never fails the record job and never invents
// an interval. A still-open (still-red) CI episode records nothing — an open
// episode is could-not-check, not a fabricated restore. The query emits an
// honest {state:"could-not-check", n:0} for an empty window, never a fabricated 0.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// doraTimingRelPath is the DORA-timing log's location relative to repo root —
// beside .history.jsonl under docs/streams/, git-tracked so it survives.
const doraTimingRelPath = "docs/streams/.dora-timing.jsonl"

// doraRecordLookbackDays bounds how far back a single record pass scans for
// merged PRs and CI runs. The pass is idempotent, so a generous window is free:
// already-recorded ids are skipped. Daily runs keep an open red episode well
// inside the window, so its true first-red is not scrolled off.
const doraRecordLookbackDays = 45

// doraRunPageCap bounds workflow_runs pagination (100/page). Three pages covers
// weeks of main-branch runs for a typical repo; the idempotent scan tolerates a
// truncated tail (older episodes are already recorded).
const doraRunPageCap = 3

// doraWorkflowEnv optionally narrows the required workflow whose main-branch
// conclusion defines red/green for a restore episode. Unset (the portable
// default) means "*": the aggregate per-commit required-check state.
const doraWorkflowEnv = "STATUSGEN_DORA_WORKFLOW"

// --- frozen record schema (contract §2) ------------------------------------

// restoreEpisode is one matched main red→green episode (feeds time_to_restore).
type restoreEpisode struct {
	Type           string `json:"type"` // always "restore_episode"
	Ts             string `json:"ts"`
	Workflow       string `json:"workflow"`
	FailedRunID    int64  `json:"failed_run_id"` // idempotency key
	FailedAt       string `json:"failed_at"`
	FailedSHA      string `json:"failed_sha"`
	RestoredRunID  int64  `json:"restored_run_id"`
	RestoredAt     string `json:"restored_at"`
	RestoredSHA    string `json:"restored_sha"`
	RestoreSeconds int64  `json:"restore_seconds"`
}

// prLeadTime is one merged PR's commit→merge lead time (feeds change_lead_time).
type prLeadTime struct {
	Type          string `json:"type"` // always "pr_lead_time"
	Ts            string `json:"ts"`
	Repo          string `json:"repo"` // with pr, the idempotency key
	PR            int    `json:"pr"`
	MergedSHA     string `json:"merged_sha"`
	FirstCommitAt string `json:"first_commit_at,omitempty"`
	OpenedAt      string `json:"opened_at"`
	MergedAt      string `json:"merged_at"`
	Anchor        string `json:"anchor"` // "first_commit" | "opened"
	LeadSeconds   int64  `json:"lead_seconds"`
}

// doraTimingRecord is the read-side view: the union of keys the idempotency
// scan and the --dora-timing query read from any line, regardless of type.
// Unknown keys are ignored (format stability), so a future added field never
// breaks this reader.
type doraTimingRecord struct {
	Type           string `json:"type"`
	FailedRunID    int64  `json:"failed_run_id"`
	RestoredAt     string `json:"restored_at"`
	RestoreSeconds int64  `json:"restore_seconds"`
	Repo           string `json:"repo"`
	PR             int    `json:"pr"`
	MergedAt       string `json:"merged_at"`
	LeadSeconds    int64  `json:"lead_seconds"`
}

// --- the CI source seam (tests substitute a fake) --------------------------

// workflowRun is the slice of a GitHub Actions run this recorder reads.
type workflowRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	HeadSHA    string `json:"head_sha"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	UpdatedAt  string `json:"updated_at"`
}

// doraMergedPR is the slice of a merged pull request this recorder reads.
type doraMergedPR struct {
	Number    int    `json:"number"`
	MergedSHA string `json:"merged_sha"`
	CreatedAt string `json:"created_at"`
	MergedAt  string `json:"merged_at"`
}

// doraTimingSource is the network seam: main-branch workflow runs, merged PRs,
// and a PR's earliest-commit authored time. Read-only in every direction — it
// lists and reads, creating and mutating nothing. The production impl shells to
// gh; tests substitute a fake with canned data and no network.
type doraTimingSource interface {
	MainWorkflowRuns(repo string) ([]workflowRun, error)
	MergedPRs(repo string, since time.Time) ([]doraMergedPR, error)
	// FirstCommitAt returns the authored time of the PR's earliest commit ahead
	// of base (ok=false when the commit list is unavailable — the caller then
	// falls back to the PR's opened_at anchor).
	FirstCommitAt(repo string, pr int) (t time.Time, ok bool)
}

// --- episode matching (the correctness core) -------------------------------

// ciHead is one main commit's settled required-check state: green or red, the
// instant it settled, and the run id that decides it. Built from workflow_runs
// by headsFromRuns; consumed by matchEpisodes.
type ciHead struct {
	SHA   string
	Red   bool // true = failure, false = success
	At    time.Time
	RunID int64
}

// matchedEpisode is one closed red→green interval (pre-schema; the caller stamps
// workflow + ts).
type matchedEpisode struct {
	FailedRunID   int64
	FailedAt      time.Time
	FailedSHA     string
	RestoredRunID int64
	RestoredAt    time.Time
	RestoredSHA   string
}

// isRedConclusion reports whether a completed run's conclusion counts as red
// (main went down). success is green; failure/timed_out/startup_failure are red;
// everything else (cancelled/skipped/neutral/action_required/stale/null) is
// non-decisive and does not define a transition on its own.
func isRedConclusion(c string) bool {
	switch c {
	case "failure", "timed_out", "startup_failure":
		return true
	}
	return false
}

func isGreenConclusion(c string) bool { return c == "success" }

// headsFromRuns collapses raw workflow_runs into one settled ciHead per main
// commit, ordered by settle time. workflowFilter=="*" is the aggregate
// per-commit required-check state; any other value narrows to that single
// workflow (matched by display name or workflow-file basename).
//
// Re-runs are handled correctly: for each (workflow, sha) only the LATEST run
// counts, so a red check re-run green leaves that workflow green on that commit.
// A commit is red iff any of its workflows' latest run is red; green iff it has
// at least one settled workflow and none are red. A commit with only
// non-decisive conclusions is dropped (neither a clear red nor green).
func headsFromRuns(runs []workflowRun, workflowFilter string) []ciHead {
	// latest[sha][workflow] = the most-recently-updated run for that pair.
	latest := map[string]map[string]workflowRun{}
	for _, r := range runs {
		if r.Status != "completed" {
			continue
		}
		if !matchesWorkflow(r, workflowFilter) {
			continue
		}
		wf := r.Name
		if wf == "" {
			wf = filepath.Base(r.Path)
		}
		byWF := latest[r.HeadSHA]
		if byWF == nil {
			byWF = map[string]workflowRun{}
			latest[r.HeadSHA] = byWF
		}
		cur, ok := byWF[wf]
		if !ok || runUpdated(r).After(runUpdated(cur)) {
			byWF[wf] = r
		}
	}

	var heads []ciHead
	for sha, byWF := range latest {
		var (
			anyRed, anyGreen bool
			redAt, greenAt   time.Time
			redRun, greenRun int64
			haveRed          bool
		)
		for _, r := range byWF {
			at := runUpdated(r)
			switch {
			case isRedConclusion(r.Conclusion):
				anyRed = true
				if !haveRed || at.Before(redAt) { // earliest red = when it went down
					redAt, redRun, haveRed = at, r.ID, true
				}
			case isGreenConclusion(r.Conclusion):
				anyGreen = true
				if at.After(greenAt) { // latest green = when it fully settled
					greenAt, greenRun = at, r.ID
				}
			}
		}
		switch {
		case anyRed:
			heads = append(heads, ciHead{SHA: sha, Red: true, At: redAt, RunID: redRun})
		case anyGreen:
			heads = append(heads, ciHead{SHA: sha, Red: false, At: greenAt, RunID: greenRun})
		}
	}
	sort.Slice(heads, func(i, j int) bool {
		if heads[i].At.Equal(heads[j].At) {
			return heads[i].SHA < heads[j].SHA // stable tiebreak
		}
		return heads[i].At.Before(heads[j].At)
	})
	return heads
}

func matchesWorkflow(r workflowRun, filter string) bool {
	if filter == "" || filter == "*" {
		return true
	}
	if r.Name == filter {
		return true
	}
	return filepath.Base(r.Path) == filter
}

func runUpdated(r workflowRun) time.Time {
	t, err := time.Parse(time.RFC3339, r.UpdatedAt)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// matchEpisodes walks settled heads in time order and returns each CLOSED
// red→green episode. This is the correctness core (exec-tier strong):
//
//   - An episode OPENS on success|null → failure and stays anchored at that
//     FIRST red. Consecutive reds do NOT re-open it — measuring a later red to
//     green would under-report the interval forever.
//   - An episode CLOSES on the next failure → success.
//   - A still-red tail (an open episode with no closing green yet) records
//     NOTHING — an open episode is could-not-check, never a fabricated interval.
func matchEpisodes(heads []ciHead) []matchedEpisode {
	var out []matchedEpisode
	var open *ciHead
	for i := range heads {
		h := heads[i]
		if h.Red {
			if open == nil { // success|null -> failure: open at the first red
				open = &heads[i]
			}
			// consecutive red: stay anchored at the first red (do nothing)
			continue
		}
		// green
		if open != nil { // failure -> success: close
			out = append(out, matchedEpisode{
				FailedRunID:   open.RunID,
				FailedAt:      open.At,
				FailedSHA:     open.SHA,
				RestoredRunID: h.RunID,
				RestoredAt:    h.At,
				RestoredSHA:   h.SHA,
			})
			open = nil
		}
	}
	// open != nil here => still-red tail => record nothing.
	return out
}

// --- lead-time computation -------------------------------------------------

// computeLeadTime builds one pr_lead_time record. It prefers first_commit_at
// (the authored time of the PR's earliest commit); it falls back to opened_at
// only when the commit list is unavailable, and records which anchor it used so
// a reader can see it. merged_at is the terminal "deploy" instant under
// merges-as-deploys. ok=false when neither a commit time nor a parseable
// opened_at is available (nothing recorded — never a fabricated interval).
func computeLeadTime(repo string, pr doraMergedPR, firstCommitAt time.Time, haveFirst bool, now time.Time) (prLeadTime, bool) {
	mergedAt, err := time.Parse(time.RFC3339, pr.MergedAt)
	if err != nil {
		return prLeadTime{}, false
	}
	mergedAt = mergedAt.UTC()

	var (
		start     time.Time
		anchor    string
		firstStr  string
		haveStart bool
	)
	if haveFirst {
		start, anchor, haveStart = firstCommitAt.UTC(), "first_commit", true
		firstStr = firstCommitAt.UTC().Format(time.RFC3339)
	} else if openedAt, oerr := time.Parse(time.RFC3339, pr.CreatedAt); oerr == nil {
		start, anchor, haveStart = openedAt.UTC(), "opened", true
	}
	if !haveStart {
		return prLeadTime{}, false
	}
	lead := int64(mergedAt.Sub(start).Seconds())
	if lead < 0 {
		lead = 0 // clock skew guard — never a negative interval
	}
	return prLeadTime{
		Type:          "pr_lead_time",
		Ts:            now.UTC().Format(time.RFC3339),
		Repo:          repo,
		PR:            pr.Number,
		MergedSHA:     pr.MergedSHA,
		FirstCommitAt: firstStr,
		OpenedAt:      pr.CreatedAt,
		MergedAt:      pr.MergedAt,
		Anchor:        anchor,
		LeadSeconds:   lead,
	}, true
}

// --- idempotency + append --------------------------------------------------

// loadDoraTimingRecords reads the append-only log. A missing file is not an
// error (no timing recorded yet). A malformed line is a hard error — the log is
// a machine format.
func loadDoraTimingRecords(path string) ([]doraTimingRecord, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []doraTimingRecord
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r doraTimingRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("%s: malformed dora-timing line: %w", path, err)
		}
		out = append(out, r)
	}
	return out, nil
}

// recordedKeys returns the already-recorded idempotency keys: the set of
// restore-episode failed_run_ids and the set of "repo\x00pr" lead-time keys.
func recordedKeys(recs []doraTimingRecord) (episodes map[int64]bool, leads map[string]bool) {
	episodes = map[int64]bool{}
	leads = map[string]bool{}
	for _, r := range recs {
		switch r.Type {
		case "restore_episode":
			if r.FailedRunID != 0 {
				episodes[r.FailedRunID] = true
			}
		case "pr_lead_time":
			leads[leadKey(r.Repo, r.PR)] = true
		}
	}
	return episodes, leads
}

func leadKey(repo string, pr int) string { return repo + "\x00" + fmt.Sprint(pr) }

// appendDoraTiming appends already-marshalled JSONL lines to the log, creating
// it (and its parent dir) if absent. An empty slice is a true no-op: the file is
// not opened or its mtime bumped — a pass that observed nothing new leaves it
// byte-identical.
func appendDoraTiming(path string, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, l := range lines {
		if _, err := f.WriteString(l + "\n"); err != nil {
			return err
		}
	}
	return nil
}

// marshalRecord renders one record as a compact JSON line (HTML escaping off, to
// match history.go's encoder and keep shas/paths byte-faithful).
func marshalRecord(v any) (string, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// --- the recorder entry point ----------------------------------------------

// recordDoraTiming is the single-writer append path for the DORA-timing log,
// called only from run()'s mode=="record" branch (never --lint). It is
// best-effort and fail-open: a repo it cannot resolve, or any gh read that
// fails, records NOTHING and prints a could-not-check line — it never fails the
// record job and never fabricates an interval. Returns the number of records
// appended (0 on a clean idempotent no-op or a could-not-check).
func recordDoraTiming(root string, src doraTimingSource, now time.Time) int {
	repo := doraTargetRepo(root)
	if repo == "" {
		fmt.Println("dora-timing: could-not-check — no target repo ($GITHUB_REPOSITORY / git remote / gh default all empty); nothing recorded")
		return 0
	}
	workflow := strings.TrimSpace(os.Getenv(doraWorkflowEnv))
	if workflow == "" {
		workflow = "*" // portable default: aggregate per-commit required-check state
	}

	path := filepath.Join(root, filepath.FromSlash(doraTimingRelPath))
	existing, err := loadDoraTimingRecords(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: dora-timing:", err)
		return 0
	}
	seenEpisodes, seenLeads := recordedKeys(existing)
	since := now.AddDate(0, 0, -doraRecordLookbackDays)

	var lines []string

	// --- restore episodes (time_to_restore) ---
	if runs, rerr := src.MainWorkflowRuns(repo); rerr != nil {
		fmt.Printf("dora-timing: could-not-check restore episodes for %s — %v; none recorded\n", repo, rerr)
	} else {
		for _, ep := range matchEpisodes(headsFromRuns(runs, workflow)) {
			if seenEpisodes[ep.FailedRunID] {
				continue // idempotent: already recorded
			}
			secs := int64(ep.RestoredAt.Sub(ep.FailedAt).Seconds())
			if secs < 0 {
				continue // clock skew — never a negative interval
			}
			rec := restoreEpisode{
				Type:           "restore_episode",
				Ts:             now.UTC().Format(time.RFC3339),
				Workflow:       workflow,
				FailedRunID:    ep.FailedRunID,
				FailedAt:       ep.FailedAt.UTC().Format(time.RFC3339),
				FailedSHA:      ep.FailedSHA,
				RestoredRunID:  ep.RestoredRunID,
				RestoredAt:     ep.RestoredAt.UTC().Format(time.RFC3339),
				RestoredSHA:    ep.RestoredSHA,
				RestoreSeconds: secs,
			}
			if l, merr := marshalRecord(rec); merr == nil {
				lines = append(lines, l)
				seenEpisodes[ep.FailedRunID] = true
			}
		}
	}

	// --- PR lead times (change_lead_time) ---
	if prs, perr := src.MergedPRs(repo, since); perr != nil {
		fmt.Printf("dora-timing: could-not-check lead times for %s — %v; none recorded\n", repo, perr)
	} else {
		for _, pr := range prs {
			if seenLeads[leadKey(repo, pr.Number)] {
				continue // idempotent
			}
			first, ok := src.FirstCommitAt(repo, pr.Number)
			rec, built := computeLeadTime(repo, pr, first, ok, now)
			if !built {
				continue
			}
			if l, merr := marshalRecord(rec); merr == nil {
				lines = append(lines, l)
				seenLeads[leadKey(repo, pr.Number)] = true
			}
		}
	}

	if err := appendDoraTiming(path, lines); err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: dora-timing:", err)
		return 0
	}
	if len(lines) == 0 {
		fmt.Println("dora-timing: no new episodes or lead times — nothing appended")
		return 0
	}
	fmt.Printf("dora-timing: appended %d record(s) to %s\n", len(lines), path)
	return len(lines)
}

// doraTargetRepo resolves the target repo, repo-agnostically:
//  1. $GITHUB_REPOSITORY (set in every GitHub Actions job)
//  2. the checkout's own git remote origin
//  3. gh's default repo for the checkout
//
// Never a hardcoded owner/repo — the recorder runs on any adopter repo.
func doraTargetRepo(root string) string {
	if r := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY")); r != "" {
		return r
	}
	if out, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output(); err == nil {
		if r := ownerRepoFromURL(strings.TrimSpace(string(out))); r != "" {
			return r
		}
	}
	if out, err := exec.Command("gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// ownerRepoFromURL extracts "owner/repo" from an origin URL (SSH or HTTPS,
// tolerating the :443 host form the house uses).
func ownerRepoFromURL(url string) string {
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "https://") {
		rest := strings.TrimPrefix(url, "https://")
		_, after, ok := strings.Cut(rest, "/") // drop host[:port]
		if ok {
			return after
		}
		return ""
	}
	if strings.Contains(url, "@") && strings.Contains(url, ":") {
		_, after, _ := strings.Cut(url, ":")
		return after
	}
	return ""
}

// --- the production gh source ----------------------------------------------

// ghDoraTimingSource is the production seam: read-only gh api reads against the
// target repo's own workflow runs and merged PRs. It creates and mutates
// nothing.
type ghDoraTimingSource struct{}

func (ghDoraTimingSource) MainWorkflowRuns(repo string) ([]workflowRun, error) {
	var all []workflowRun
	for page := 1; page <= doraRunPageCap; page++ {
		out, err := exec.Command("gh", "api",
			fmt.Sprintf("repos/%s/actions/runs?branch=main&status=completed&per_page=100&page=%d", repo, page),
		).Output()
		if err != nil {
			return nil, fmt.Errorf("gh api actions/runs page %d: %w", page, err)
		}
		var resp struct {
			WorkflowRuns []workflowRun `json:"workflow_runs"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			return nil, fmt.Errorf("unmarshal actions/runs: %w", err)
		}
		if len(resp.WorkflowRuns) == 0 {
			break
		}
		all = append(all, resp.WorkflowRuns...)
		if len(resp.WorkflowRuns) < 100 {
			break
		}
	}
	return all, nil
}

func (ghDoraTimingSource) MergedPRs(repo string, since time.Time) ([]doraMergedPR, error) {
	// Closed PRs against main, newest-updated first; keep the merged ones whose
	// merge instant is within the window.
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/pulls?state=closed&base=main&sort=updated&direction=desc&per_page=100", repo),
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api pulls: %w", err)
	}
	var raw []struct {
		Number         int    `json:"number"`
		CreatedAt      string `json:"created_at"`
		MergedAt       string `json:"merged_at"`
		MergeCommitSHA string `json:"merge_commit_sha"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal pulls: %w", err)
	}
	var prs []doraMergedPR
	for _, p := range raw {
		if p.MergedAt == "" {
			continue // closed unmerged — not a change
		}
		mt, perr := time.Parse(time.RFC3339, p.MergedAt)
		if perr != nil || mt.Before(since) {
			continue
		}
		prs = append(prs, doraMergedPR{
			Number:    p.Number,
			MergedSHA: p.MergeCommitSHA,
			CreatedAt: p.CreatedAt,
			MergedAt:  p.MergedAt,
		})
	}
	return prs, nil
}

func (ghDoraTimingSource) FirstCommitAt(repo string, pr int) (time.Time, bool) {
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/pulls/%d/commits?per_page=100", repo, pr),
	).Output()
	if err != nil {
		return time.Time{}, false
	}
	var commits []struct {
		Commit struct {
			Author struct {
				Date string `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := json.Unmarshal(out, &commits); err != nil || len(commits) == 0 {
		return time.Time{}, false
	}
	var earliest time.Time
	found := false
	for _, c := range commits {
		t, perr := time.Parse(time.RFC3339, c.Commit.Author.Date)
		if perr != nil {
			continue
		}
		if !found || t.Before(earliest) {
			earliest, found = t.UTC(), true
		}
	}
	return earliest, found
}

// --- the query (contract §3) -----------------------------------------------

// doraTimingWindow is the [since, until) bound of a query.
type doraTimingWindow struct {
	Since string `json:"since"`
	Until string `json:"until"`
}

// doraTimingMetric is one metric's aggregate. n≥1 renders {unit,p50,p90,n};
// n==0 renders {state:"could-not-check", n:0} — an honest empty, never a
// fabricated 0. The pointer/omitempty split produces exactly those two shapes.
type doraTimingMetric struct {
	Unit  string   `json:"unit,omitempty"`
	P50   *float64 `json:"p50,omitempty"`
	P90   *float64 `json:"p90,omitempty"`
	State string   `json:"state,omitempty"`
	N     int      `json:"n"`
}

type doraTimingReport struct {
	Generated      string           `json:"generated"`
	Window         doraTimingWindow `json:"window"`
	ChangeLeadTime doraTimingMetric `json:"change_lead_time"`
	TimeToRestore  doraTimingMetric `json:"time_to_restore"`
}

// aggregateSeconds turns a list of whole-second intervals into a metric: p50/p90
// in hours (one decimal) when n≥1, else the honest could-not-check.
func aggregateSeconds(secs []int64) doraTimingMetric {
	n := len(secs)
	if n == 0 {
		return doraTimingMetric{State: "could-not-check", N: 0}
	}
	p50 := pctlHours(secs, 0.5)
	p90 := pctlHours(secs, 0.9)
	return doraTimingMetric{Unit: "hours", P50: &p50, P90: &p90, N: n}
}

// pctlHours returns the q-quantile of secs, in hours rounded to one decimal.
// Same index convention as the roadmap's p90Duration/medianDur (nearest-rank).
func pctlHours(secs []int64, q float64) float64 {
	sorted := make([]int64, len(secs))
	copy(sorted, secs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)) * q)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	hours := float64(sorted[idx]) / 3600.0
	return float64(int64(hours*10+0.5)) / 10 // round to one decimal
}

// computeDoraTiming aggregates records whose TERMINAL instant (restored_at for a
// restore episode, merged_at for a lead time) falls in [since, until).
func computeDoraTiming(recs []doraTimingRecord, since, until, now time.Time) doraTimingReport {
	var restoreSecs, leadSecs []int64
	for _, r := range recs {
		switch r.Type {
		case "restore_episode":
			if inWindow(r.RestoredAt, since, until) {
				restoreSecs = append(restoreSecs, r.RestoreSeconds)
			}
		case "pr_lead_time":
			if inWindow(r.MergedAt, since, until) {
				leadSecs = append(leadSecs, r.LeadSeconds)
			}
		}
	}
	return doraTimingReport{
		Generated:      now.UTC().Format(time.RFC3339),
		Window:         doraTimingWindow{Since: since.UTC().Format(time.RFC3339), Until: until.UTC().Format(time.RFC3339)},
		ChangeLeadTime: aggregateSeconds(leadSecs),
		TimeToRestore:  aggregateSeconds(restoreSecs),
	}
}

// inWindow reports whether an RFC3339 instant falls in [since, until). An
// unparseable instant is out (never silently counted).
func inWindow(ts string, since, until time.Time) bool {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return false
	}
	t = t.UTC()
	return !t.Before(since) && t.Before(until)
}

// runDoraTiming is the `statusgen --root . --dora-timing --json` entry point: a
// self-contained, STATUS.md-free, offline query over the recorded substrate.
// --since / --until (YYYY-MM-DD, until exclusive) bound the window; both default
// to the standard reporting window.
func runDoraTiming(root, since, until string, asJSON bool) int {
	now := nowFunc()
	untilT := now
	if until != "" {
		t, err := parseSinceDate(until)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --until must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		untilT = t
	}
	var sinceT time.Time
	if since != "" {
		t, err := parseSinceDate(since)
		if err != nil {
			fmt.Fprintf(os.Stderr, "statusgen: --since must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		sinceT = t
	} else {
		sinceT = untilT.AddDate(0, 0, -defaultDoraWindowDays)
	}
	if sinceT.After(untilT) {
		fmt.Fprintln(os.Stderr, "statusgen: --since is after --until")
		return 1
	}

	recs, err := loadDoraTimingRecords(filepath.Join(root, filepath.FromSlash(doraTimingRelPath)))
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: dora-timing:", err)
		return 1
	}
	rep := computeDoraTiming(recs, sinceT, untilT, now)

	if asJSON {
		enc, merr := json.MarshalIndent(rep, "", "  ")
		if merr != nil {
			fmt.Fprintln(os.Stderr, "statusgen: dora-timing:", merr)
			return 1
		}
		fmt.Println(string(enc))
		return 0
	}
	// Text summary (JSON is the fixed contract form; text is a convenience).
	fmt.Printf("DORA timing -- %s ... %s\n", rep.Window.Since, rep.Window.Until)
	fmt.Printf("  change_lead_time: %s\n", renderTimingMetric(rep.ChangeLeadTime))
	fmt.Printf("  time_to_restore:  %s\n", renderTimingMetric(rep.TimeToRestore))
	return 0
}

func renderTimingMetric(m doraTimingMetric) string {
	if m.State != "" || m.P50 == nil {
		return fmt.Sprintf("could-not-check (n=%d)", m.N)
	}
	return fmt.Sprintf("p50=%.1fh p90=%.1fh (n=%d)", *m.P50, *m.P90, m.N)
}
