package main

// Self-improvement metric emitter (`statusgen --issues --self-improvement`).
//
// statusgen/02's --issues infra answers "is the front door healthy". This mode
// answers a narrower, higher-stakes question: of the issues that got RESOLVED,
// how many did the loops notice from their own experience, file, and fix on
// their own — versus how many needed a human to diagnose, direct, decide, or
// fix? That ratio (the autonomy / self-healing rate) is the clearest available
// signal of whether the system is getting better at fixing itself.
//
// Per RESOLVED (closed) issue, the classifier derives:
//
//   - SELF-HEALED (closed-loop autonomy) — ALL of: (a) agent-raised (the author
//     is an agent, OR the issue carries a raised-by:<desk> label); (b) fixed by
//     an agent-authored PR (the merged fixing PR's author is an agent);
//     (c) no human touch anywhere in the lifecycle.
//   - HUMAN-TOUCHED — any of: human-raised, a human comment, a needs-decision a
//     human answered, a human reopen/manual-close, or a human-authored fix.
//
// THE LOAD-BEARING CAVEAT: a human maintainer merges every PR in this house's
// flow; if "merged by a human" counted as a touch nothing would ever be
// self-healed. A touch is human diagnosis/direction/decision/intervention —
// never the standing merge gate. So when a fixing PR is found, this classifier
// checks only the fixing PR's AUTHOR — it never inspects who merged it, and a
// ClosedEvent actor is used as a "manual close" signal ONLY when no fixing PR
// was found at all (see classifySelfImprovement).
//
// Data source: statusgen/02's issue records (author, labels, state) plus one
// `gh api graphql` fetch per resolved issue for comment authors, reopen/close
// actors, and the merged fixing PR's author (closedByPullRequestsReferences).
// A per-issue gh failure degrades to a could-not-check note — that issue is
// excluded from the counts, never guessed into either bucket (C4: could-not-
// check is never a pass, and never a silent fail either).
//
// Like --issues/--dora this is a self-contained diagnostic sub-command: it
// never reads or writes STATUS.md.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// selfImprovementBanner is the diagnostic banner carried in text and JSON
// output. Contains DIAGNOSTIC (anti-Goodhart discipline) and, verbatim, the
// merge-gate-is-not-a-touch caveat so the number is never misread.
const selfImprovementBanner = "Self-improvement metrics are DIAGNOSTIC, per-project, for continuous " +
	"improvement — never a target, an individual scorecard, or a cross-team comparison (Goodhart's " +
	"law: a measure that becomes a target ceases to be a good measure). THE STANDING HUMAN MERGE " +
	"GATE IS NOT A TOUCH: every PR in this flow is merged by a human, but that merge approval is the " +
	"system's standing gate on all writes, orthogonal to who found and fixed the problem — only " +
	"diagnosis, direction, decision, reopening, manual closing, or authoring the fix counts as a " +
	"human touch."

// --- classification shapes (pure) ---------------------------

// selfImprovementDetail carries the per-issue timeline signals beyond
// issueMetricRecord that the classifier needs — fetched once per RESOLVED
// issue in production, injected as a fixture in tests.
type selfImprovementDetail struct {
	CommentAuthors    []string // every comment's author login, in order
	ReopenActors      []string // ReopenedEvent actor logins, in order
	ManualCloseActors []string // ClosedEvent actor login(s) — read ONLY when
	// FixingPRFound is false (see the merge-gate caveat above): when a fixing
	// PR was merged, the ClosedEvent actor is whoever merged it, which is
	// exactly the non-touch the caveat exists to exclude.
	FixingPRAuthor string // the merged fixing PR's author login
	FixingPRFound  bool   // true iff a merged closing PR reference was found
}

// selfImprovementTouch is the human-touch-by-type breakdown for one issue. An
// issue can land in more than one bucket (e.g. human-raised AND human-fixed).
type selfImprovementTouch struct {
	Raised           bool
	SteeredByComment bool
	Decided          bool
	Reopened         bool
	FixedByHuman     bool
}

func (t selfImprovementTouch) any() bool {
	return t.Raised || t.SteeredByComment || t.Decided || t.Reopened || t.FixedByHuman
}

// selfImprovementVerdict is the classification of one resolved issue.
// Classified is false when the issue is neither self-healed nor human-touched
// (e.g. closed with no linked fixing PR and no human signal at all — a
// duplicate/wontfix an agent closed outright): reported for transparency,
// excluded from the rate exactly as the brief's formula specifies.
type selfImprovementVerdict struct {
	SelfHealed bool
	Touch      selfImprovementTouch
	Classified bool
}

// classifySelfImprovement is the pure classifier — no exec, no clock read — so
// the whole rule, including the merge-gate caveat, is exercised offline over
// fixtures.
func classifySelfImprovement(rec issueMetricRecord, det selfImprovementDetail, cfg issueMetricConfig) selfImprovementVerdict {
	authorAgent, _ := classifyIssueAuthor(rec.Author, cfg)
	agentRaised := authorAgent || raisedByDesk(rec.Labels) != unattributedDesk

	var touch selfImprovementTouch
	// Human-raised: the issue's own author account is a human — independent of
	// any raised-by:<desk> label an agent might also stamp on it.
	touch.Raised = !authorAgent

	needsDecision := hasLabel(rec.Labels, decisionLabel)
	humanCommented := false
	for _, login := range det.CommentAuthors {
		if agent, _ := classifyIssueAuthor(login, cfg); !agent {
			humanCommented = true
			break
		}
	}
	if humanCommented {
		if needsDecision {
			touch.Decided = true
		} else {
			touch.SteeredByComment = true
		}
	}

	for _, login := range det.ReopenActors {
		if agent, _ := classifyIssueAuthor(login, cfg); !agent {
			touch.Reopened = true
			break
		}
	}
	if !touch.Reopened && !det.FixingPRFound {
		// Manual-close signal reads ONLY when no fixing PR was found — see the
		// merge-gate caveat on selfImprovementDetail.ManualCloseActors.
		for _, login := range det.ManualCloseActors {
			if agent, _ := classifyIssueAuthor(login, cfg); !agent {
				touch.Reopened = true
				break
			}
		}
	}

	agentFixed := false
	if det.FixingPRFound {
		fixerAgent, _ := classifyIssueAuthor(det.FixingPRAuthor, cfg)
		agentFixed = fixerAgent
		touch.FixedByHuman = !fixerAgent
	}

	humanTouched := touch.any()
	selfHealed := agentRaised && agentFixed && !humanTouched

	return selfImprovementVerdict{
		SelfHealed: selfHealed,
		Touch:      touch,
		Classified: selfHealed || humanTouched,
	}
}

// --- report shapes (JSON) ------------------------------------

// SelfImprovementTouchCounts is the human-touched bucket broken out by touch
// TYPE, so it is clear WHERE the human was needed. An issue may contribute to
// more than one field.
type SelfImprovementTouchCounts struct {
	Raised           int `json:"raised"`
	SteeredByComment int `json:"steeredByComment"`
	Decided          int `json:"decided"`
	Reopened         int `json:"reopened"`
	FixedByHuman     int `json:"fixedByHuman"`
}

// SelfImprovementReport is the full emitted system.
type SelfImprovementReport struct {
	Note                string                     `json:"note"`
	Generated           string                     `json:"generated"`
	Repos               []string                   `json:"repos"`
	Resolved            int                        `json:"resolved"` // closed issues actually classified
	SelfHealed          int                        `json:"selfHealed"`
	HumanTouched        int                        `json:"humanTouched"`
	SelfImprovementRate string                     `json:"selfImprovementRate"`
	Unclassified        int                        `json:"unclassified"` // neither bucket (see selfImprovementVerdict.Classified)
	HumanTouchedByType  SelfImprovementTouchCounts `json:"humanTouchedByType"`
	CouldNotCheck       []string                   `json:"couldNotCheck,omitempty"`
}

// issueKey correlates a base issue record with its fetched detail across a
// possibly multi-repo run (repo label + number; label is unique enough for a
// single run's internal correlation, it never needs to be a real slug).
type issueKey struct {
	Repo   string
	Number int
}

// computeSelfImprovementMetrics builds the full report from CLOSED records
// whose detail was fetched successfully, plus config. Pure — no exec, no
// clock read.
func computeSelfImprovementMetrics(records []issueMetricRecord, details map[issueKey]selfImprovementDetail, cfg issueMetricConfig) SelfImprovementReport {
	rep := SelfImprovementReport{Note: selfImprovementBanner, Generated: cfg.Now.Format(time.RFC3339)}
	for _, rec := range records {
		if !strings.EqualFold(rec.State, "closed") {
			continue // per-RESOLVED (closed) issue only
		}
		det := details[issueKey{rec.Repo, rec.Number}]
		v := classifySelfImprovement(rec, det, cfg)
		rep.Resolved++
		switch {
		case v.SelfHealed:
			rep.SelfHealed++
		case v.Touch.any():
			rep.HumanTouched++
			if v.Touch.Raised {
				rep.HumanTouchedByType.Raised++
			}
			if v.Touch.SteeredByComment {
				rep.HumanTouchedByType.SteeredByComment++
			}
			if v.Touch.Decided {
				rep.HumanTouchedByType.Decided++
			}
			if v.Touch.Reopened {
				rep.HumanTouchedByType.Reopened++
			}
			if v.Touch.FixedByHuman {
				rep.HumanTouchedByType.FixedByHuman++
			}
		default:
			rep.Unclassified++
		}
	}
	denom := rep.SelfHealed + rep.HumanTouched
	if denom > 0 {
		rep.SelfImprovementRate = fmt.Sprintf("%.2f", float64(rep.SelfHealed)/float64(denom))
	} else {
		rep.SelfImprovementRate = "n/a"
	}
	return rep
}

// --- rendering -------------------------------------------------

func renderSelfImprovementText(rep SelfImprovementReport) string {
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	w("Self-improvement metric")
	if len(rep.Repos) > 0 {
		w(" — %s", strings.Join(rep.Repos, ", "))
	}
	w("\n%s\n\n", selfImprovementBanner)

	w("Resolved (closed) issues examined: %d\n\n", rep.Resolved)
	w("self-healed (agent-raised + agent-fixed + no human touch): %d\n", rep.SelfHealed)
	w("human-touched (human diagnosis/direction/decision/fix at any point): %d\n", rep.HumanTouched)
	w("self_improvement_rate: %s   (self-healed / (self-healed + human-touched))\n", rep.SelfImprovementRate)
	if rep.Unclassified > 0 {
		w("unclassified (neither bucket — e.g. closed with no linked fixing PR and no human signal): %d\n", rep.Unclassified)
	}

	w("\nHuman-touched by type (an issue may land in more than one bucket)\n")
	w("  raised: %d\n", rep.HumanTouchedByType.Raised)
	w("  steered-by-comment: %d\n", rep.HumanTouchedByType.SteeredByComment)
	w("  decided (needs-decision answered): %d\n", rep.HumanTouchedByType.Decided)
	w("  reopened / manual-close: %d\n", rep.HumanTouchedByType.Reopened)
	w("  fixed-by-human: %d\n", rep.HumanTouchedByType.FixedByHuman)

	if len(rep.CouldNotCheck) > 0 {
		w("\nCOULD-NOT-CHECK (per-issue gh failure — these issues are excluded from the counts above):\n")
		for _, c := range rep.CouldNotCheck {
			w("  %s\n", c)
		}
	}
	return b.String()
}

// --- time series (--issues --self-improvement --series) --------

// selfImprovementSeriesPoint is one weekly bucket of the self-improvement rate
// over issues CLOSED in that bucket — "is autonomy rising?".
type selfImprovementSeriesPoint struct {
	Period              string `json:"period"` // ISO-week Monday, YYYY-MM-DD
	SelfHealed          int    `json:"selfHealed"`
	HumanTouched        int    `json:"humanTouched"`
	SelfImprovementRate string `json:"selfImprovementRate"`
}

// computeSelfImprovementSeries buckets classified (self-healed or human-
// touched) resolved issues by ISO week of ClosedAt. A week with no closes
// still renders with rate "n/a" (a zero week is data, not a gap).
func computeSelfImprovementSeries(records []issueMetricRecord, details map[issueKey]selfImprovementDetail, cfg issueMetricConfig) []selfImprovementSeriesPoint {
	type agg struct{ healed, touched int }
	buckets := map[string]*agg{}
	var earliest, latest time.Time

	for _, rec := range records {
		if !strings.EqualFold(rec.State, "closed") || !rec.HasClosed || rec.ClosedAt.IsZero() {
			continue
		}
		v := classifySelfImprovement(rec, details[issueKey{rec.Repo, rec.Number}], cfg)
		if !v.Classified {
			continue // unclassified issues don't inform the trend
		}
		key := bucketStart(rec.ClosedAt, "weekly").Format("2006-01-02")
		a, ok := buckets[key]
		if !ok {
			a = &agg{}
			buckets[key] = a
		}
		if v.SelfHealed {
			a.healed++
		} else {
			a.touched++
		}
		if earliest.IsZero() || rec.ClosedAt.Before(earliest) {
			earliest = rec.ClosedAt
		}
		if rec.ClosedAt.After(latest) {
			latest = rec.ClosedAt
		}
	}
	if earliest.IsZero() {
		return nil
	}

	start := bucketStart(earliest, "weekly")
	end := bucketStart(latest, "weekly")
	var points []selfImprovementSeriesPoint
	for b := start; !b.After(end); b = periodStep(b, "weekly") {
		key := b.Format("2006-01-02")
		p := selfImprovementSeriesPoint{Period: key, SelfImprovementRate: "n/a"}
		if a, ok := buckets[key]; ok {
			p.SelfHealed = a.healed
			p.HumanTouched = a.touched
			if denom := a.healed + a.touched; denom > 0 {
				p.SelfImprovementRate = fmt.Sprintf("%.2f", float64(a.healed)/float64(denom))
			}
		}
		points = append(points, p)
	}
	return points
}

// renderSelfImprovementSeriesText renders the weekly series. It takes couldNotCheck
// for the same reason the aggregate and both JSON modes carry it: an issue whose
// detail fetch failed is excluded from every bucket, so a reader who is not told
// how many issues were dropped cannot tell a genuine low week from a blind one.
// A renderer that silently omits could-not-check reports a clean number it did not
// earn (C4) — this is the one mode that used to do that.
func renderSelfImprovementSeriesText(points []selfImprovementSeriesPoint, couldNotCheck []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Self-improvement metric — weekly time series\n%s\n\n", selfImprovementBanner)
	fmt.Fprintf(&b, "  %-12s %10s %12s %6s\n", "week", "self-healed", "human-touched", "rate")
	for _, p := range points {
		fmt.Fprintf(&b, "  %-12s %10d %12d %6s\n", p.Period, p.SelfHealed, p.HumanTouched, p.SelfImprovementRate)
	}
	if len(couldNotCheck) > 0 {
		fmt.Fprintf(&b, "\nCOULD-NOT-CHECK (%d per-issue gh failure(s) — these issues are excluded from every bucket above):\n", len(couldNotCheck))
		for _, c := range couldNotCheck {
			fmt.Fprintf(&b, "  %s\n", c)
		}
	}
	return b.String()
}

// --- gh fetch (production) --------------------------------------

// selfImprovementDetailFetcher is the per-issue detail reader. The production
// implementation shells out to `gh api graphql`; tests inject a fixture so the
// classifier and its wiring are exercised offline.
type selfImprovementDetailFetcher func(ownerRepo string, number int) (selfImprovementDetail, error)

// selfImprovementDetailQuery fetches, for one issue: every comment's author,
// ReopenedEvent/ClosedEvent actors (bounded first:100 — an overflowing thread
// degrades to a could-not-check on parse rather than walking a cursor), and
// the merged closing PR's author via closedByPullRequestsReferences. Actor
// __typename is carried so a Bot actor is re-suffixed "[bot]" the same way
// ghIssueMetricLister and the trust query already do (GraphQL renders a Bot's
// bare slug, unlike REST's "<name>[bot]").
const selfImprovementDetailQuery = `query($owner:String!,$name:String!,$number:Int!){` +
	`repository(owner:$owner,name:$name){issue(number:$number){` +
	`comments(first:100){pageInfo{hasNextPage} nodes{author{login __typename}}} ` +
	`timelineItems(first:100,itemTypes:[REOPENED_EVENT,CLOSED_EVENT]){nodes{__typename ` +
	`...on ReopenedEvent{actor{login __typename}} ...on ClosedEvent{actor{login __typename}}}} ` +
	`closedByPullRequestsReferences(first:10,includeClosedPrs:true){nodes{merged author{login __typename}}}` +
	`}}}`

// ghSelfImprovementDetailFetcher is the production selfImprovementDetailFetcher.
// A gh failure or a GraphQL error (including, should the GitHub schema not
// carry closedByPullRequestsReferences on some API version, an "unknown field"
// error) is returned to the caller, which degrades that ONE issue to a
// could-not-check rather than aborting the run or guessing a classification.
func ghSelfImprovementDetailFetcher(ownerRepo string, number int) (selfImprovementDetail, error) {
	owner, name, ok := strings.Cut(ownerRepo, "/")
	if !ok || owner == "" || name == "" {
		return selfImprovementDetail{}, fmt.Errorf("bad repo %q", ownerRepo)
	}
	out, err := exec.Command("gh", "api", "graphql",
		"-f", "query="+selfImprovementDetailQuery,
		"-f", "owner="+owner, "-f", "name="+name, "-F", "number="+strconv.Itoa(number)).Output()
	if err != nil {
		detail := ""
		if ee, ok := err.(*exec.ExitError); ok {
			detail = strings.TrimSpace(string(ee.Stderr))
		}
		return selfImprovementDetail{}, fmt.Errorf("gh api graphql self-improvement detail %s#%d: %v %s", ownerRepo, number, err, detail)
	}
	return parseSelfImprovementDetail(out)
}

// parseSelfImprovementDetail parses a selfImprovementDetailQuery response.
func parseSelfImprovementDetail(raw []byte) (selfImprovementDetail, error) {
	type actor struct {
		Login    string `json:"login"`
		Typename string `json:"__typename"`
	}
	var env struct {
		Data struct {
			Repository struct {
				Issue *struct {
					Comments struct {
						PageInfo struct {
							HasNextPage bool `json:"hasNextPage"`
						} `json:"pageInfo"`
						Nodes []struct {
							Author *actor `json:"author"`
						} `json:"nodes"`
					} `json:"comments"`
					TimelineItems struct {
						Nodes []struct {
							Typename string `json:"__typename"`
							Actor    *actor `json:"actor"`
						} `json:"nodes"`
					} `json:"timelineItems"`
					ClosedByPullRequestsReferences struct {
						Nodes []struct {
							Merged bool   `json:"merged"`
							Author *actor `json:"author"`
						} `json:"nodes"`
					} `json:"closedByPullRequestsReferences"`
				} `json:"issue"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return selfImprovementDetail{}, fmt.Errorf("parsing self-improvement detail response: %w", err)
	}
	if len(env.Errors) > 0 {
		return selfImprovementDetail{}, fmt.Errorf("self-improvement detail GraphQL error: %s", env.Errors[0].Message)
	}
	iss := env.Data.Repository.Issue
	if iss == nil {
		return selfImprovementDetail{}, fmt.Errorf("self-improvement detail query returned no issue")
	}
	if iss.Comments.PageInfo.HasNextPage {
		return selfImprovementDetail{}, fmt.Errorf("self-improvement detail: >100 comments — could-not-check rather than a truncated read")
	}
	loginOf := func(a *actor) (string, bool) {
		if a == nil {
			return "", false
		}
		login := a.Login
		if a.Typename == "Bot" {
			login += "[bot]"
		}
		return login, true
	}

	var det selfImprovementDetail
	for _, n := range iss.Comments.Nodes {
		if login, ok := loginOf(n.Author); ok {
			det.CommentAuthors = append(det.CommentAuthors, login)
		}
	}
	for _, n := range iss.TimelineItems.Nodes {
		login, ok := loginOf(n.Actor)
		if !ok {
			continue
		}
		switch n.Typename {
		case "ReopenedEvent":
			det.ReopenActors = append(det.ReopenActors, login)
		case "ClosedEvent":
			det.ManualCloseActors = append(det.ManualCloseActors, login)
		}
	}
	for _, n := range iss.ClosedByPullRequestsReferences.Nodes {
		if !n.Merged {
			continue
		}
		if login, ok := loginOf(n.Author); ok {
			det.FixingPRAuthor = login
			det.FixingPRFound = true
			break // the first merged fixing PR is the fix
		}
	}
	return det, nil
}

// --- entrypoint ---------------------------------------------------

// selfImprovementDetailLister is the injection point production code and
// tests share — mirrors issueMetricLister's fixture-injection pattern.
var selfImprovementDetailLister selfImprovementDetailFetcher = ghSelfImprovementDetailFetcher

// gatherSelfImprovementDetails fetches the per-issue detail for every CLOSED
// record, degrading a per-issue gh failure to a could-not-check note. Returns
// only the records whose detail was actually fetched — a failed fetch is
// EXCLUDED from classification entirely rather than classified against a
// zero-value detail, so a could-not-check is never silently read as "no
// activity found" (C4).
func gatherSelfImprovementDetails(records []issueMetricRecord, fetch selfImprovementDetailFetcher) (classifiable []issueMetricRecord, details map[issueKey]selfImprovementDetail, couldNotCheck []string) {
	details = map[issueKey]selfImprovementDetail{}
	for _, rec := range records {
		if !strings.EqualFold(rec.State, "closed") {
			continue
		}
		if rec.OwnerRepo == "" {
			couldNotCheck = append(couldNotCheck, fmt.Sprintf("#%d (%s): no resolvable owner/repo for the detail fetch", rec.Number, rec.Repo))
			continue
		}
		det, err := fetch(rec.OwnerRepo, rec.Number)
		if err != nil {
			couldNotCheck = append(couldNotCheck, fmt.Sprintf("#%d (%s): %v", rec.Number, rec.Repo, err))
			continue
		}
		details[issueKey{rec.Repo, rec.Number}] = det
		classifiable = append(classifiable, rec)
	}
	return classifiable, details, couldNotCheck
}

// runSelfImprovement is the --issues --self-improvement entrypoint.
// Self-contained diagnostic sub-command — never reads or writes STATUS.md,
// same discipline as --issues/--dora.
func runSelfImprovement(root string, asJSON, series bool, staleDays int, teamLoginsCSV string) int {
	records, repos, listCouldNotCheck := gatherIssueRecords(ghIssueMetricLister)
	cfg := assembleIssueConfig(staleDays, teamLoginsCSV)
	classifiable, details, detailCouldNotCheck := gatherSelfImprovementDetails(records, selfImprovementDetailLister)
	couldNotCheck := append(append([]string{}, listCouldNotCheck...), detailCouldNotCheck...)

	if series {
		points := computeSelfImprovementSeries(classifiable, details, cfg)
		if asJSON {
			enc, err := json.MarshalIndent(struct {
				Note          string                       `json:"note"`
				Points        []selfImprovementSeriesPoint `json:"points"`
				CouldNotCheck []string                     `json:"couldNotCheck,omitempty"`
			}{Note: selfImprovementBanner, Points: points, CouldNotCheck: couldNotCheck}, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, "statusgen:", err)
				return 1
			}
			fmt.Println(string(enc))
			return 0
		}
		if len(points) == 0 {
			// An empty series with could-not-check entries is not "no resolved issues" —
			// it may be "every issue was unreadable", so the list is printed here too.
			fmt.Printf("Self-improvement metric — weekly time series\n%s\n\nno resolved issues found\n", selfImprovementBanner)
			if len(couldNotCheck) > 0 {
				fmt.Printf("\nCOULD-NOT-CHECK (%d per-issue gh failure(s) — these issues are excluded from every bucket above):\n", len(couldNotCheck))
				for _, c := range couldNotCheck {
					fmt.Printf("  %s\n", c)
				}
			}
			return 0
		}
		fmt.Print(renderSelfImprovementSeriesText(points, couldNotCheck))
		return 0
	}

	rep := computeSelfImprovementMetrics(classifiable, details, cfg)
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
	fmt.Print(renderSelfImprovementText(rep))
	return 0
}
