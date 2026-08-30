package main

// Issue metrics emitter (`statusgen --issues`).
//
// GitHub issues are a first-class part of the work model (the issue-loop desk
// raises and works them), but the board otherwise measures PRs and briefs, not
// issues — so the front door's health is invisible: how many are open, how long
// they have been sitting, who raised them, and whether our own agents are
// finding the work or an outside human is reporting a problem.
//
// This mode emits four metric groups as a SYSTEM (never a single "issues"
// number that conflates them):
//   - Standard: open/closed counts, close rate, time-to-close median+p90, by label.
//   - By TYPE and SEVERITY: process-STATES (verify-gate/live-verify/needs-decision)
//     counted as their own class and EXCLUDED from the "bug" totals; DEFECTS
//     (`bug`); severity within defects (critical/high broken out from normal);
//     everything else per label. Mixing states into the bug lump hides the signal
//     and distorts --dora's CFR (it should use the defect count, not raw `bug`).
//   - Age / sitting-time (OPEN issues): age buckets, oldest N, and a stale-issue
//     alarm mirroring issue-loop/07's intake-debt alarm.
//   - Internal-vs-external and agent-vs-human author classification, plus the
//     by-raising-desk cut over the `raised-by:<desk>` label.
//
// Data source: `gh issue list --state all --json number,state,createdAt,closedAt,
// author,labels,title` over the owned-repo set (scanRepos — the same reader the
// issue scanner uses), generalising dora.go's bug-issue fetch. Like --dora it is
// a self-contained diagnostic sub-command: it never reads or writes STATUS.md.
//
// Offline discipline: --issues needs `gh`; a per-repo gh failure degrades to a
// could-not-check note rather than aborting, and the stale-issue alarm that rides
// the --lint gate is gh-guarded (skipped, never a hard failure, when gh is
// absent) so the offline lint never gains a hard network dependency.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// issueBanner is the diagnostic banner carried in --issues text output and the
// JSON `note` field — the same anti-gaming discipline as --dora. Contains the
// word DIAGNOSTIC (Verify item 2).
const issueBanner = "Issue metrics are DIAGNOSTIC, per-project, for continuous improvement — " +
	"never a target, an individual scorecard, or a cross-team comparison (Goodhart's law: a measure " +
	"that becomes a target ceases to be a good measure)."

// defaultStaleIssueDays is the age past which an OPEN issue trips the stale-issue
// alarm. 7 days = the same rot the intake-debt alarm catches, applied to issues.
const defaultStaleIssueDays = 7

// staleIssueDaysCfg is the effective stale-issue threshold the --lint gate's
// stale-issue NOTICE reads. Set from --stale-issue-days before any run, mirroring
// standingAgeDays. The --issues emitter takes the flag value directly instead.
var staleIssueDaysCfg = defaultStaleIssueDays

// defaultOldestIssueN is how many of the oldest open issues the report names.
const defaultOldestIssueN = 5

// raisedByPrefix is the label namespace the by-desk cut reads. brief-29 makes the
// filing desks stamp it; until then every issue reads as `unattributed`.
const raisedByPrefix = "raised-by:"

// unattributedDesk is the by-desk bucket for an issue carrying no raised-by:<desk>
// label — graceful degradation that is itself the signal (like intake "untagged").
const unattributedDesk = "unattributed"

// Age-bucket keys, in display order. Age is now-createdAt over OPEN issues only.
var issueAgeBucketOrder = []string{"<1d", "1-3d", "3-7d", ">7d"}

// severityLabels are the labels that, on a defect, raise it to the critical/high
// class — its own count AND its own age, because a critical sitting 3 days is a
// different alarm than a normal bug sitting 3 days.
var issueSeverityLabels = map[string]bool{
	"critical":      true,
	"high-priority": true,
	"high":          true,
}

// severityTitleWords are the escalation-vocabulary words in an issue TITLE that
// also raise a defect to critical/high (matched case-insensitively as whole
// upper-case tokens, the convention the desks use).
var issueSeverityTitleWords = []string{"URGENT", "BLOCKER"}

// issueMetricRecord is the slice of one issue the metrics need. It is the pure
// input to computeIssueMetrics — assembled from gh in production, from a fixture
// in tests, so all the math is exercised offline with no exec/network.
type issueMetricRecord struct {
	Number    int
	Repo      string
	OwnerRepo string // explicit "owner/repo" slug — never the "(ambient)" display
	// label Repo carries; statusgen/03's per-issue gh api graphql fetch needs a
	// literal owner/name pair (see resolvedOwnerRepo).
	Title     string
	State     string // "OPEN" | "CLOSED" (gh renders upper-case)
	CreatedAt time.Time
	ClosedAt  time.Time // zero when open / unset
	HasClosed bool      // true when ClosedAt is a real timestamp
	Author    string    // login; gh renders Apps as "app/<slug>" or "<name>[bot]"
	Labels    []string
}

// issueMetricConfig is the pure classification configuration. Assembling it from
// the roster (assembleIssueConfig) keeps computeIssueMetrics free of any env /
// roster read, so the tests drive it directly.
type issueMetricConfig struct {
	Now        time.Time
	StaleDays  int
	OldestN    int
	OrgAccount string          // the owner org login (author == org ⇒ agent)
	TeamLogins map[string]bool // lower-cased internal/team logins (roster + --team-logins)
}

// --- report shapes (JSON) ----------------------------------

// TTCStats is the time-to-close distribution over CLOSED issues.
type TTCStats struct {
	N      int    `json:"n"`
	Median string `json:"median"` // human-readable, or "n/a" when n==0
	P90    string `json:"p90"`
}

// IssueTypeCounts is the type partition: process-states, defects, and everything
// else per label. Each issue lands in exactly one class (states win over defects
// win over other), so a verify-gate issue is NEVER counted as a bug.
type IssueTypeCounts struct {
	States  int            `json:"states"`  // carries a system-state label
	Defects int            `json:"defects"` // carries `bug`, no system-state label
	Other   map[string]int `json:"other"`   // remaining issues, tallied per label
}

// DefectCounts breaks the defect total into severity classes.
type DefectCounts struct {
	Total    int `json:"total"`
	Critical int `json:"critical"` // ALSO critical/high, or URGENT/BLOCKER title
	Normal   int `json:"normal"`
}

// OldestIssue names one of the oldest OPEN issues.
type OldestIssue struct {
	Number  int    `json:"number"`
	Repo    string `json:"repo"`
	AgeDays int    `json:"ageDays"`
	Age     string `json:"age"`
	Title   string `json:"title"`
}

// StaleSummary is the stale-issue alarm state (OPEN issues past StaleDays).
type StaleSummary struct {
	Days         int    `json:"days"`
	Open         int    `json:"open"`
	Over         int    `json:"over"`
	OldestNumber int    `json:"oldestNumber"`
	OldestRepo   string `json:"oldestRepo"`
	OldestAge    string `json:"oldestAge"`
}

// IssueReport is the full emitted system.
type IssueReport struct {
	Note          string          `json:"note"`
	Generated     string          `json:"generated"`
	Repos         []string        `json:"repos"`
	Open          int             `json:"open"`
	Closed        int             `json:"closed"`
	Total         int             `json:"total"`
	CloseRate     string          `json:"closeRate"`
	TimeToClose   TTCStats        `json:"timeToClose"`
	ByLabel       map[string]int  `json:"byLabel"`
	ByType        IssueTypeCounts `json:"byType"`
	Defects       DefectCounts    `json:"defects"`
	AgeBuckets    map[string]int  `json:"ageBuckets"`
	Oldest        []OldestIssue   `json:"oldest"`
	Stale         StaleSummary    `json:"stale"`
	Agent         int             `json:"agent"`
	Human         int             `json:"human"`
	Internal      int             `json:"internal"`
	External      int             `json:"external"`
	ByDesk        map[string]int  `json:"byDesk"`
	CouldNotCheck []string        `json:"couldNotCheck,omitempty"`
}

// --- classification helpers (pure) -------------------------

// isBotLogin reports whether a login is an automation identity: the `[bot]`
// suffix or gh's `app/<slug>` rendering. Case-insensitive.
func isBotLogin(login string) bool {
	l := strings.ToLower(strings.TrimSpace(login))
	return strings.HasSuffix(l, "[bot]") || strings.HasPrefix(l, "app/")
}

// classifyIssueAuthor returns the two author axes for one login. They answer
// different questions: agent-vs-human ("are our loops finding the work?") and
// internal-vs-external ("is anyone outside filing?").
//
//   - agent  = a bot identity, or the owner org account.
//   - internal = agent, or a login in the team set (roster trusted logins +
//     --team-logins). external is every other login — how the first real
//     user-reported issue becomes visible when it arrives.
func classifyIssueAuthor(login string, cfg issueMetricConfig) (agent, internal bool) {
	l := strings.ToLower(strings.TrimSpace(login))
	agent = isBotLogin(login) || (cfg.OrgAccount != "" && l == strings.ToLower(cfg.OrgAccount))
	internal = agent || cfg.TeamLogins[l]
	return agent, internal
}

// hasSystemStateLabel reports whether any label is a process-STATE label
// (verify-gate/live-verify/needs-decision/…). Those are work-state issues, not
// defects, and are counted as their own class.
func hasSystemStateLabel(labels []string) bool {
	set := scanExcludedLabelSet() // topologySystemStateLabels, lower-cased
	for _, l := range labels {
		if set[strings.ToLower(strings.TrimSpace(l))] {
			return true
		}
	}
	return false
}

// hasLabel reports a case-insensitive label membership.
func hasLabel(labels []string, want string) bool {
	want = strings.ToLower(want)
	for _, l := range labels {
		if strings.ToLower(strings.TrimSpace(l)) == want {
			return true
		}
	}
	return false
}

// isCriticalDefect reports whether a defect is critical/high severity: a
// severity label, or an escalation word in the title.
func isCriticalDefect(rec issueMetricRecord) bool {
	for _, l := range rec.Labels {
		if issueSeverityLabels[strings.ToLower(strings.TrimSpace(l))] {
			return true
		}
	}
	up := strings.ToUpper(rec.Title)
	for _, w := range issueSeverityTitleWords {
		if strings.Contains(up, w) {
			return true
		}
	}
	return false
}

// raisedByDesk returns the desk from an issue's raised-by:<desk> label, or
// unattributedDesk when none is present.
func raisedByDesk(labels []string) string {
	for _, l := range labels {
		ll := strings.ToLower(strings.TrimSpace(l))
		if strings.HasPrefix(ll, raisedByPrefix) {
			desk := strings.TrimSpace(ll[len(raisedByPrefix):])
			if desk != "" {
				return desk
			}
		}
	}
	return unattributedDesk
}

// ageBucket returns the age-distribution bucket for a duration.
func ageBucket(age time.Duration) string {
	days := age.Hours() / 24
	switch {
	case days < 1:
		return "<1d"
	case days < 3:
		return "1-3d"
	case days < 7:
		return "3-7d"
	default:
		return ">7d"
	}
}

// --- computation (pure) ------------------------------------

// computeIssueMetrics builds the full report from the records + config. Pure:
// no exec, no clock read (cfg.Now is the clock) — the whole surface is tested
// offline over fixtures.
func computeIssueMetrics(records []issueMetricRecord, cfg issueMetricConfig) IssueReport {
	if cfg.OldestN <= 0 {
		cfg.OldestN = defaultOldestIssueN
	}
	if cfg.StaleDays <= 0 {
		cfg.StaleDays = defaultStaleIssueDays
	}
	rep := IssueReport{
		Note:       issueBanner,
		Generated:  cfg.Now.Format(time.RFC3339),
		ByLabel:    map[string]int{},
		AgeBuckets: map[string]int{},
		ByDesk:     map[string]int{},
		ByType:     IssueTypeCounts{Other: map[string]int{}},
		Stale:      StaleSummary{Days: cfg.StaleDays},
	}
	// Seed the age buckets so every bucket renders even at zero (an absent
	// bucket and a zero bucket are the same fact, shown the same way).
	for _, b := range issueAgeBucketOrder {
		rep.AgeBuckets[b] = 0
	}

	var closeDurations []time.Duration
	var openIssues []issueMetricRecord

	for _, rec := range records {
		rep.Total++
		// Standard by-label breakdown counts every label occurrence across all
		// issues (distinct from the type partition below).
		for _, l := range rec.Labels {
			if n := strings.TrimSpace(l); n != "" {
				rep.ByLabel[n]++
			}
		}
		// By-desk cut.
		rep.ByDesk[raisedByDesk(rec.Labels)]++
		// Author axes.
		if agent, internal := classifyIssueAuthor(rec.Author, cfg); agent {
			rep.Agent++
			if internal {
				rep.Internal++
			} else {
				rep.External++
			}
		} else {
			rep.Human++
			if internal {
				rep.Internal++
			} else {
				rep.External++
			}
		}
		// Type partition: states win over defects win over other, so a
		// verify-gate issue is never counted as a bug.
		switch {
		case hasSystemStateLabel(rec.Labels):
			rep.ByType.States++
		case hasLabel(rec.Labels, "bug"):
			rep.ByType.Defects++
			rep.Defects.Total++
			if isCriticalDefect(rec) {
				rep.Defects.Critical++
			} else {
				rep.Defects.Normal++
			}
		default:
			if len(rec.Labels) == 0 {
				rep.ByType.Other["unlabeled"]++
			}
			for _, l := range rec.Labels {
				if n := strings.TrimSpace(l); n != "" {
					rep.ByType.Other[n]++
				}
			}
		}
		// Open/closed split + time-to-close.
		if strings.EqualFold(rec.State, "closed") {
			rep.Closed++
			if rec.HasClosed && !rec.CreatedAt.IsZero() && !rec.ClosedAt.Before(rec.CreatedAt) {
				closeDurations = append(closeDurations, rec.ClosedAt.Sub(rec.CreatedAt))
			}
		} else {
			rep.Open++
			openIssues = append(openIssues, rec)
		}
	}

	// Close rate = closed / total.
	if rep.Total > 0 {
		rep.CloseRate = fmt.Sprintf("%.2f", float64(rep.Closed)/float64(rep.Total))
	} else {
		rep.CloseRate = "n/a"
	}

	// Time-to-close median + p90.
	rep.TimeToClose = TTCStats{N: len(closeDurations), Median: "n/a", P90: "n/a"}
	if len(closeDurations) > 0 {
		rep.TimeToClose.Median = humanDur(medianDur(closeDurations))
		rep.TimeToClose.P90 = humanDur(p90Duration(closeDurations))
	}

	// Age / sitting-time over OPEN issues.
	type aged struct {
		rec  issueMetricRecord
		age  time.Duration
		days int
	}
	var ageds []aged
	for _, rec := range openIssues {
		if rec.CreatedAt.IsZero() {
			continue // no credible age — never bucket or age-alarm it
		}
		age := cfg.Now.Sub(rec.CreatedAt)
		if age < 0 {
			age = 0
		}
		days := int(age.Hours() / 24)
		rep.AgeBuckets[ageBucket(age)]++
		ageds = append(ageds, aged{rec, age, days})
		if days > cfg.StaleDays {
			rep.Stale.Over++
		}
	}
	rep.Stale.Open = rep.Open
	// Oldest N (descending age); also feeds the stale oldest fields.
	sort.SliceStable(ageds, func(i, j int) bool { return ageds[i].age > ageds[j].age })
	for i, a := range ageds {
		if i >= cfg.OldestN {
			break
		}
		rep.Oldest = append(rep.Oldest, OldestIssue{
			Number:  a.rec.Number,
			Repo:    a.rec.Repo,
			AgeDays: a.days,
			Age:     humanDur(a.age),
			Title:   a.rec.Title,
		})
	}
	if len(ageds) > 0 {
		o := ageds[0]
		rep.Stale.OldestNumber = o.rec.Number
		rep.Stale.OldestRepo = o.rec.Repo
		rep.Stale.OldestAge = humanDur(o.age)
	}
	return rep
}

// --- rendering ---------------------------------------------

// renderIssuesText renders the human table. It always names the process-STATES
// class (verify-gate/…), the DEFECT count, and the CRITICAL severity as distinct
// classes (Verify item 4b), and the by-desk cut (unattributed until mm/29).
func renderIssuesText(rep IssueReport) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	w("Issue metrics")
	if len(rep.Repos) > 0 {
		w(" — %s", strings.Join(rep.Repos, ", "))
	}
	w("\n%s\n\n", issueBanner)

	w("Standard\n")
	w("  open: %d   closed: %d   total: %d   close-rate: %s\n", rep.Open, rep.Closed, rep.Total, rep.CloseRate)
	w("  time-to-close: median %s   p90 %s   (n=%d closed)\n", rep.TimeToClose.Median, rep.TimeToClose.P90, rep.TimeToClose.N)

	w("\nBy type & severity\n")
	w("  process-states (verify-gate/live-verify/needs-decision, NOT bugs): %d\n", rep.ByType.States)
	w("  defects (bug): %d   of which critical/high: %d   normal: %d\n", rep.Defects.Total, rep.Defects.Critical, rep.Defects.Normal)
	if len(rep.ByType.Other) > 0 {
		w("  other:\n")
		for _, k := range sortedKeys(rep.ByType.Other) {
			w("    %s: %d\n", k, rep.ByType.Other[k])
		}
	}

	w("\nBy label\n")
	if len(rep.ByLabel) == 0 {
		w("  (none)\n")
	}
	for _, k := range sortedKeys(rep.ByLabel) {
		w("  %s: %d\n", k, rep.ByLabel[k])
	}

	w("\nAge / sitting-time (open)\n")
	for _, bk := range issueAgeBucketOrder {
		w("  %-5s %d\n", bk, rep.AgeBuckets[bk])
	}
	if len(rep.Oldest) > 0 {
		w("  oldest:\n")
		for _, o := range rep.Oldest {
			w("    #%d (%s) %s — %s\n", o.Number, o.Repo, o.Age, o.Title)
		}
	}
	w("  stale-issue alarm: %d open, %d over %dd", rep.Stale.Open, rep.Stale.Over, rep.Stale.Days)
	if rep.Stale.Over > 0 {
		w(", oldest #%d at %s", rep.Stale.OldestNumber, rep.Stale.OldestAge)
	}
	w("\n")

	w("\nInternal vs external / agent vs human\n")
	w("  agent: %d   human: %d\n", rep.Agent, rep.Human)
	w("  internal: %d   external: %d\n", rep.Internal, rep.External)

	w("\nBy raising desk (raised-by:<desk>; unattributed until mm/29)\n")
	if len(rep.ByDesk) == 0 {
		w("  (no issues)\n")
	}
	for _, k := range sortedKeys(rep.ByDesk) {
		w("  %s: %d\n", k, rep.ByDesk[k])
	}

	if len(rep.CouldNotCheck) > 0 {
		w("\nCOULD-NOT-CHECK (per-repo gh failure — these repos are not in the counts above):\n")
		for _, c := range rep.CouldNotCheck {
			w("  %s\n", c)
		}
	}
	return b.String()
}

// (sortedKeys is shared with shardcheck.go — a map[string]int → ascending keys.)

// --- time series (--issues --series) -----------------------

// issueSeriesPoint is one weekly bucket of issue activity.
type issueSeriesPoint struct {
	Period string `json:"period"` // ISO-week Monday, YYYY-MM-DD
	Opened int    `json:"opened"` // issues createdAt in the bucket
	Closed int    `json:"closed"` // issues closedAt in the bucket
}

// computeIssueSeries buckets issues by ISO week of createdAt (opened) and
// closedAt (closed). Buckets run from the earliest createdAt to now; a week with
// no activity still renders (a zero week is data, not a gap).
func computeIssueSeries(records []issueMetricRecord, now time.Time) []issueSeriesPoint {
	if len(records) == 0 {
		return nil
	}
	earliest := now
	for _, rec := range records {
		if !rec.CreatedAt.IsZero() && rec.CreatedAt.Before(earliest) {
			earliest = rec.CreatedAt
		}
	}
	start := bucketStart(earliest, "weekly")
	end := bucketStart(now, "weekly")
	opened := map[string]int{}
	closed := map[string]int{}
	for _, rec := range records {
		if !rec.CreatedAt.IsZero() {
			opened[bucketStart(rec.CreatedAt, "weekly").Format("2006-01-02")]++
		}
		if rec.HasClosed && !rec.ClosedAt.IsZero() {
			closed[bucketStart(rec.ClosedAt, "weekly").Format("2006-01-02")]++
		}
	}
	var points []issueSeriesPoint
	for b := start; !b.After(end); b = periodStep(b, "weekly") {
		key := b.Format("2006-01-02")
		points = append(points, issueSeriesPoint{Period: key, Opened: opened[key], Closed: closed[key]})
	}
	return points
}

func renderIssueSeriesText(points []issueSeriesPoint) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Issue metrics — weekly time series\n%s\n\n", issueBanner)
	fmt.Fprintf(&b, "  %-12s %8s %8s\n", "week", "opened", "closed")
	for _, p := range points {
		fmt.Fprintf(&b, "  %-12s %8d %8d\n", p.Period, p.Opened, p.Closed)
	}
	return b.String()
}

// --- gh fetch (production) ---------------------------------

// issueMetricListerFn is the issue reader --issues uses. The production
// implementation shells out to gh; tests inject a fixture so the mode is
// exercised offline.
type issueMetricLister func(repo string) ([]issueMetricRecord, error)

// ghIssueMetricLister lists ALL issues (state all) of a repo with the extended
// field set the metrics need. An empty repo means the ambient repo (no --repo),
// matching --dora's ambient-repo behaviour. A gh failure is an error the caller
// degrades to a per-repo could-not-check note.
var ghIssueMetricLister issueMetricLister = func(repo string) ([]issueMetricRecord, error) {
	args := []string{"issue", "list", "--state", "all", "--limit", "1000",
		"--json", "number,state,createdAt,closedAt,author,labels,title"}
	if repo != "" {
		args = append(args[:2:2], append([]string{"--repo", repo}, args[2:]...)...)
	}
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return nil, fmt.Errorf("gh issue list%s: %v %s", repoArgSuffix(repo), err, detail)
	}
	var raw []struct {
		Number    int        `json:"number"`
		Title     string     `json:"title"`
		State     string     `json:"state"`
		CreatedAt time.Time  `json:"createdAt"`
		ClosedAt  *time.Time `json:"closedAt"`
		Author    struct {
			Login string `json:"login"`
		} `json:"author"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing gh output%s: %w", repoArgSuffix(repo), err)
	}
	recs := make([]issueMetricRecord, 0, len(raw))
	for _, r := range raw {
		rec := issueMetricRecord{
			Number:    r.Number,
			Repo:      repoLabel(repo),
			OwnerRepo: resolvedOwnerRepo(repo),
			Title:     r.Title,
			State:     r.State,
			CreatedAt: r.CreatedAt.UTC(),
			Author:    r.Author.Login,
		}
		if r.ClosedAt != nil && !r.ClosedAt.IsZero() {
			rec.ClosedAt = r.ClosedAt.UTC()
			rec.HasClosed = true
		}
		for _, l := range r.Labels {
			if n := strings.TrimSpace(l.Name); n != "" {
				rec.Labels = append(rec.Labels, n)
			}
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

func repoArgSuffix(repo string) string {
	if repo == "" {
		return ""
	}
	return " --repo " + repo
}

// repoLabel names a repo for display: its own slug, or "(ambient)" for the empty
// (no --repo) fetch.
func repoLabel(repo string) string {
	if repo == "" {
		return "(ambient)"
	}
	return repo
}

// resolvedOwnerRepo returns the explicit "owner/repo" slug for API calls that
// cannot lean on gh's ambient-repo resolution the way `gh issue list` can — the
// self-improvement detail fetch (`gh api graphql`) takes literal owner/name
// arguments, not a --repo flag. Falls back to the scanner's configured home
// repo for the ambient (no --repo) case; empty when even that is unconfigured,
// which the caller degrades to a could-not-check per issue rather than guessing.
func resolvedOwnerRepo(repo string) string {
	if repo != "" {
		return repo
	}
	return scanHomeRepo()
}

// reposForIssues is the repo set --issues reads. It is the scanner's owned-repo
// roster (scanRepos); when that is unconfigured it falls back to the single
// ambient repo, so the mode still reports the current repo's issues offline-of-
// roster, exactly as --dora does.
func reposForIssues() []string {
	repos := scanRepos()
	if len(repos) == 0 {
		return []string{""}
	}
	return repos
}

// assembleIssueConfig builds the classification config from the flags + the
// roster. The team/internal set is the roster's trusted logins (the same current-
// authors allowlist the scanner trusts) UNION any --team-logins, and the org
// account is the owner of the home repo — both sourced, never a guessed literal.
func assembleIssueConfig(staleDays int, teamLoginsCSV string) issueMetricConfig {
	cfg := issueMetricConfig{
		Now:        nowFunc(),
		StaleDays:  staleDays,
		OldestN:    defaultOldestIssueN,
		TeamLogins: map[string]bool{},
	}
	// Owner org from the home repo slug (owner/repo).
	if home := scanHomeRepo(); home != "" {
		if i := strings.Index(home, "/"); i > 0 {
			cfg.OrgAccount = home[:i]
		}
	}
	// Roster trusted logins are the current-authors allowlist.
	for login := range scanEffectiveConfig().Logins {
		cfg.TeamLogins[strings.ToLower(login)] = true
	}
	// --team-logins augments that set.
	for _, l := range strings.Split(teamLoginsCSV, ",") {
		if t := strings.ToLower(strings.TrimSpace(l)); t != "" {
			cfg.TeamLogins[t] = true
		}
	}
	return cfg
}

// gatherIssueRecords lists issues across the repo set, degrading a per-repo gh
// failure to a could-not-check note rather than aborting the whole run.
func gatherIssueRecords(list issueMetricLister) (records []issueMetricRecord, repos []string, couldNotCheck []string) {
	for _, r := range reposForIssues() {
		repos = append(repos, repoLabel(r))
		recs, err := list(r)
		if err != nil {
			couldNotCheck = append(couldNotCheck, err.Error())
			continue
		}
		records = append(records, recs...)
	}
	return records, repos, couldNotCheck
}

// runIssues is the --issues entrypoint. Self-contained diagnostic sub-command —
// never reads or writes STATUS.md, same discipline as --dora.
func runIssues(root string, asJSON, series bool, staleDays int, teamLoginsCSV string) int {
	records, repos, couldNotCheck := gatherIssueRecords(ghIssueMetricLister)

	if series {
		points := computeIssueSeries(records, nowFunc())
		if asJSON {
			enc, err := json.MarshalIndent(struct {
				Note   string             `json:"note"`
				Points []issueSeriesPoint `json:"points"`
			}{Note: issueBanner, Points: points}, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, "statusgen:", err)
				return 1
			}
			fmt.Println(string(enc))
			return 0
		}
		if len(points) == 0 {
			fmt.Printf("Issue metrics — weekly time series\n%s\n\nno issues found\n", issueBanner)
			return 0
		}
		fmt.Print(renderIssueSeriesText(points))
		return 0
	}

	rep := computeIssueMetrics(records, assembleIssueConfig(staleDays, teamLoginsCSV))
	rep.Repos = repos
	rep.CouldNotCheck = couldNotCheck
	if asJSON {
		enc, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, "statusgen:", err)
			return 1
		}
		fmt.Println(string(enc))
		return 0
	}
	fmt.Print(renderIssuesText(rep))
	return 0
}

// --- stale-issue alarm (the --lint gate line) --------------

// openIssueDebtNotice is the gh-guarded stale-issue alarm that rides the --lint
// gate, mirroring issue-loop/07's intake-debt alarm applied to issues. It is a
// NOTICE only (never a hard problem) and is gh-GUARDED: absent gh, or any gh
// failure, degrades to "" (skipped) so the offline --lint gate never gains a hard
// network dependency. Returns the one board line
// `issue debt: N open, K over <days>d, oldest #<n> at <age>` when at least one
// open issue is over the threshold, else "".
func openIssueDebtNotice(staleDays int) string {
	if staleDays <= 0 {
		staleDays = defaultStaleIssueDays
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return "" // skipped (no gh) — offline discipline
	}
	var records []issueMetricRecord
	for _, r := range reposForIssues() {
		recs, err := ghIssueMetricLister(r)
		if err != nil {
			return "" // gh present but failed → skip, never fail the lint
		}
		records = append(records, recs...)
	}
	rep := computeIssueMetrics(records, issueMetricConfig{Now: nowFunc(), StaleDays: staleDays, OldestN: 1})
	if rep.Stale.Over == 0 {
		return ""
	}
	return fmt.Sprintf("issue debt: %d open, %d over %dd, oldest #%d at %s",
		rep.Stale.Open, rep.Stale.Over, rep.Stale.Days, rep.Stale.OldestNumber, rep.Stale.OldestAge)
}
