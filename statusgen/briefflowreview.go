package main

// --review-rework and --first-pass-yield (brief-07, metrics 4 and 5) both read
// one substrate: merged PRs in the window that are LINKED to a brief via the
// `Brief: <stream>/<NN>` trailer deskpr requires at open time
// (tools/desk/internal/deskkit/trailer.go), plus that PR's full reviews array.
//
// REUSE, not a fork:
//   - The reviews array itself: brief-07 facts are explicit — "review-rework
//     computes from the `gh pr view --json reviews` ARRAY (not the laundered
//     latest view)". autoflip.go's ghModelFlipSource.ReviewState already made
//     exactly this choice, for exactly this reason (its own comment: `gh pr
//     view --json reviews` collapses to one row per author, losing the
//     CHANGES_REQUESTED cycles a rework metric needs; the REST
//     `pulls/{n}/reviews` endpoint returns every submission). This file's
//     production source shells to the same REST endpoint and unmarshals into
//     corroborate.go's existing ghReview/ghAuthor types — no new review shape.
//   - The Brief: trailer: prlink.go already classifies a PR body's trailer
//     count (linked/unlinked/multi-linked) but does not extract the VALUE;
//     extractBriefTrailer below is the one place that does, reusing prlink.go's
//     fence-skipping regex so a trailer mentioned inside a code sample is not
//     read as a real link (same grammar prlink.go and deskkit.ParseTrailers
//     share).
//   - "no verify-fail" (first-pass-yield): verifyissues.go's lastVerifyVerdict
//     over the brief's Evidence body (already hydrated onto the Brief row by
//     loadHydratedStreams).
//   - "no finding names it" (first-pass-yield): drivecritical.go's
//     reviewerFindingCritical over the loaded Finding register — the same
//     linkage the reviewer-finding critical-path tier already uses.
//
// A PR with no resolvable Brief: trailer is EXCLUDED from both metrics (not
// silently zero-weighted) and counted in unlinked_excluded, so a reader can
// see how much of the merged-PR volume this window could not correlate to a
// brief — the desk's own `Brief: <stream>/<NN>` discipline (derived-board) is
// what makes the correlation possible at all; an adopter who does not carry
// that trailer convention will legitimately see 0 linked / N unlinked, which
// is could-not-check, never a fabricated pass rate.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// bfPRRecord is one merged PR's Brief:-trailer-resolvable metadata.
type bfPRRecord struct {
	Number   int
	Body     string
	MergedAt string
}

// bfPRSource is the network seam --review-rework and --first-pass-yield share:
// merged PRs in a window (with body, for the Brief: trailer) and one PR's full
// reviews array. The production impl shells to gh; tests inject a fake so
// neither metric's compute logic is ever exercised over the network.
type bfPRSource interface {
	MergedPRs(repo string, since time.Time) ([]bfPRRecord, error)
	Reviews(repo string, pr int) ([]ghReview, error)
}

// ghBFPRSource is the production bfPRSource.
type ghBFPRSource struct{}

// MergedPRs mirrors ghDoraTimingSource.MergedPRs (doratiming.go) — same
// endpoint, same closed+base=main+client-side-since-filter shape — but also
// reads `body`, which that source has no need of.
func (ghBFPRSource) MergedPRs(repo string, since time.Time) ([]bfPRRecord, error) {
	out, err := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/pulls?state=closed&base=main&sort=updated&direction=desc&per_page=100", repo),
	).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api pulls: %w", err)
	}
	var raw []struct {
		Number   int    `json:"number"`
		Body     string `json:"body"`
		MergedAt string `json:"merged_at"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal pulls: %w", err)
	}
	var prs []bfPRRecord
	for _, p := range raw {
		if p.MergedAt == "" {
			continue // closed unmerged
		}
		mt, perr := time.Parse(time.RFC3339, p.MergedAt)
		if perr != nil || mt.Before(since) {
			continue
		}
		prs = append(prs, bfPRRecord{Number: p.Number, Body: p.Body, MergedAt: p.MergedAt})
	}
	return prs, nil
}

// Reviews reads the REST reviews-endpoint array (autoflip.go's ghRESTReview
// shape, reused as-is) — the un-laundered array brief-07's facts call for.
func (ghBFPRSource) Reviews(repo string, pr int) ([]ghReview, error) {
	out, err := exec.Command("gh", "api", "--paginate",
		fmt.Sprintf("repos/%s/pulls/%d/reviews", repo, pr)).Output()
	if err != nil {
		return nil, fmt.Errorf("gh api pulls/%d/reviews: %w", pr, err)
	}
	var rest []ghRESTReview
	if err := json.Unmarshal(out, &rest); err != nil {
		return nil, fmt.Errorf("unmarshal PR %d reviews: %w", pr, err)
	}
	reviews := make([]ghReview, 0, len(rest))
	for _, r := range rest {
		reviews = append(reviews, ghReview{Author: ghAuthor{Login: r.User.Login}, State: r.State})
	}
	return reviews, nil
}

// bfBriefTrailerRe matches a `Brief: <id>` trailer line (deskkit's trailer
// grammar); the captured id is trimmed of trailing punctuation a prose
// sentence might leave attached.
var bfBriefTrailerRe = regexp.MustCompile(`(?m)^[ \t]*Brief:[ \t]*(\S+)[ \t]*$`)

// extractBriefTrailer returns the first Brief: trailer VALUE outside a fenced
// code block — prlink.go's rePRLinkFence reused verbatim so a trailer shown as
// a documentation example is not read as a real link.
func extractBriefTrailer(body string) (string, bool) {
	inFence := false
	for _, raw := range strings.Split(body, "\n") {
		if rePRLinkFence.MatchString(raw) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := bfBriefTrailerRe.FindStringSubmatch(raw); m != nil {
			return strings.TrimRight(m[1], ".,;:"), true
		}
	}
	return "", false
}

// resolvePRsByBrief maps each brief id to the LATEST (by merged_at) merged PR
// in the window naming it. A brief with more than one linked PR in the window
// (a rare re-open/rework case) is credited to its most recent merge — the one
// closest to when the brief actually left the pipeline.
func resolvePRsByBrief(prs []bfPRRecord) (byBrief map[string]bfPRRecord, unlinked int) {
	byBrief = map[string]bfPRRecord{}
	for _, p := range prs {
		id, ok := extractBriefTrailer(p.Body)
		if !ok {
			unlinked++
			continue
		}
		if cur, seen := byBrief[id]; !seen || p.MergedAt > cur.MergedAt {
			byBrief[id] = p
		}
	}
	return byBrief, unlinked
}

// countChangesRequested counts CHANGES_REQUESTED entries in the full reviews
// array — each one a rework round (brief-07 Task item 5).
func countChangesRequested(reviews []ghReview) int {
	n := 0
	for _, r := range reviews {
		if r.State == "CHANGES_REQUESTED" {
			n++
		}
	}
	return n
}

// --- metric 5: review-rework rounds -----------------------------------------

type bfReviewReworkReport struct {
	Generated        string           `json:"generated"`
	Window           doraTimingWindow `json:"window"`
	State            string           `json:"state"`
	N                int              `json:"n"` // linked merged PRs examined
	UnlinkedExcluded int              `json:"unlinked_excluded"`
	MeanRounds       float64          `json:"mean_rounds,omitempty"`
	Distribution     map[string]int   `json:"rounds_distribution"` // "0","1","2","3+"
}

// reworkBucket labels a CHANGES_REQUESTED count for the distribution.
func reworkBucket(n int) string {
	switch {
	case n <= 0:
		return "0"
	case n == 1:
		return "1"
	case n == 2:
		return "2"
	default:
		return "3+"
	}
}

// computeReviewRework builds the CHANGES_REQUESTED-rounds distribution over
// every brief-linked merged PR in the window. reviewsErr records per-PR read
// failures (best-effort: one unreadable PR does not fail the whole report; it
// is simply excluded, same as an unlinked one).
func computeReviewRework(prsByBrief map[string]bfPRRecord, unlinked int, reviewsFn func(pr int) ([]ghReview, error)) bfReviewReworkReport {
	rep := bfReviewReworkReport{
		UnlinkedExcluded: unlinked,
		Distribution:     map[string]int{"0": 0, "1": 0, "2": 0, "3+": 0},
	}
	var total int
	for _, pr := range prsByBrief {
		reviews, err := reviewsFn(pr.Number)
		if err != nil {
			continue // could-not-read this one PR — excluded, not fabricated
		}
		n := countChangesRequested(reviews)
		rep.Distribution[reworkBucket(n)]++
		rep.N++
		total += n
	}
	if rep.N == 0 {
		rep.State = "could-not-check"
		return rep
	}
	rep.State = "ok"
	rep.MeanRounds = round1(float64(total) / float64(rep.N))
	return rep
}

func runReviewRework(root, since, until string, asJSON bool, src bfPRSource) int {
	now := nowFunc()
	sinceT, untilT, err := resolveBFWindow(since, until, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	rep := bfReviewReworkReport{Generated: now.UTC().Format(time.RFC3339), Window: bfWindowJSON(sinceT, untilT), Distribution: map[string]int{}}
	repo := doraTargetRepo(root)
	if repo == "" {
		rep.State = "could-not-check"
		return finishReviewRework(rep, asJSON)
	}
	prs, perr := src.MergedPRs(repo, sinceT)
	if perr != nil {
		rep.State = "could-not-check"
		return finishReviewRework(rep, asJSON)
	}
	byBrief, unlinked := resolvePRsByBrief(prs)
	computed := computeReviewRework(byBrief, unlinked, func(pr int) ([]ghReview, error) { return src.Reviews(repo, pr) })
	computed.Generated = rep.Generated
	computed.Window = rep.Window
	return finishReviewRework(computed, asJSON)
}

func finishReviewRework(rep bfReviewReworkReport, asJSON bool) int {
	if asJSON {
		return printBFJSON(rep)
	}
	if rep.State == "could-not-check" {
		fmt.Printf("review-rework -- %s ... %s: could-not-check (unlinked=%d)\n", rep.Window.Since, rep.Window.Until, rep.UnlinkedExcluded)
		return 0
	}
	fmt.Printf("review-rework -- %s ... %s: n=%d mean=%.1f dist=%v (unlinked=%d)\n",
		rep.Window.Since, rep.Window.Until, rep.N, rep.MeanRounds, rep.Distribution, rep.UnlinkedExcluded)
	return 0
}

// --- metric 4: first-pass yield ----------------------------------------------

type bfFirstPassReport struct {
	Generated        string           `json:"generated"`
	Window           doraTimingWindow `json:"window"`
	State            string           `json:"state"`
	N                int              `json:"n"` // done briefs with a resolvable linked PR
	FirstPass        int              `json:"first_pass"`
	Yield            float64          `json:"yield,omitempty"`
	UnlinkedExcluded int              `json:"unlinked_excluded"`
}

// computeFirstPassYield implements brief-07 Task item 4's compound
// definition: "brief merged with 0 CHANGES_REQUESTED ∧ no verify-fail ∧ no
// finding names it". doneBriefIDs are the historian's to:"done" brief ids in
// the window; evidenceByID/findings are the offline sources for the latter two
// legs (loadHydratedStreams' Brief.Evidence + the Finding register).
func computeFirstPassYield(
	doneBriefIDs []string,
	prsByBrief map[string]bfPRRecord,
	unlinked int,
	reviewsFn func(pr int) ([]ghReview, error),
	evidenceByID map[string]string,
	findings []Finding,
) bfFirstPassReport {
	rep := bfFirstPassReport{UnlinkedExcluded: unlinked}
	for _, id := range doneBriefIDs {
		pr, ok := prsByBrief[id]
		if !ok {
			continue // no resolvable PR — excluded (already counted in unlinked via the PR side)
		}
		reviews, err := reviewsFn(pr.Number)
		if err != nil {
			continue // could-not-read this PR's reviews — excluded, not fabricated
		}
		stream, num, cutOK := strings.Cut(id, "/")
		if !cutOK {
			continue
		}
		changesReq := countChangesRequested(reviews)
		verifyFail := lastVerifyVerdict(evidenceByID[id]) == verdictFail
		findingNamed := reviewerFindingCritical(findings, stream, num)
		rep.N++
		if changesReq == 0 && !verifyFail && !findingNamed {
			rep.FirstPass++
		}
	}
	if rep.N == 0 {
		rep.State = "could-not-check"
		return rep
	}
	rep.State = "ok"
	rep.Yield = round1(float64(rep.FirstPass) / float64(rep.N) * 100)
	return rep
}

func runFirstPassYield(root, since, until string, asJSON bool, src bfPRSource) int {
	now := nowFunc()
	sinceT, untilT, err := resolveBFWindow(since, until, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	rep := bfFirstPassReport{Generated: now.UTC().Format(time.RFC3339), Window: bfWindowJSON(sinceT, untilT)}

	streams, findings, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen: first-pass-yield:", err)
		return 1
	}
	history, herr := LoadHistory(historyAbsPath(root))
	if herr != nil {
		fmt.Fprintln(os.Stderr, "statusgen: first-pass-yield:", herr)
		return 1
	}
	evidenceByID := map[string]string{}
	for _, s := range streams {
		for _, b := range s.Briefs {
			evidenceByID[s.Name+"/"+b.Num] = b.Evidence
		}
	}
	var doneIDs []string
	for _, e := range history {
		if e.To == "done" && inWindow(e.Ts, sinceT, untilT) {
			doneIDs = append(doneIDs, e.Brief)
		}
	}

	repo := doraTargetRepo(root)
	if repo == "" {
		rep.State = "could-not-check"
		return finishFirstPass(rep, asJSON)
	}
	prs, perr := src.MergedPRs(repo, sinceT)
	if perr != nil {
		rep.State = "could-not-check"
		return finishFirstPass(rep, asJSON)
	}
	byBrief, unlinked := resolvePRsByBrief(prs)
	computed := computeFirstPassYield(doneIDs, byBrief, unlinked, func(pr int) ([]ghReview, error) { return src.Reviews(repo, pr) }, evidenceByID, findings)
	computed.Generated = rep.Generated
	computed.Window = rep.Window
	return finishFirstPass(computed, asJSON)
}

func finishFirstPass(rep bfFirstPassReport, asJSON bool) int {
	if asJSON {
		return printBFJSON(rep)
	}
	if rep.State == "could-not-check" {
		fmt.Printf("first-pass yield -- %s ... %s: could-not-check (unlinked=%d)\n", rep.Window.Since, rep.Window.Until, rep.UnlinkedExcluded)
		return 0
	}
	fmt.Printf("first-pass yield -- %s ... %s: %.1f%% (%d/%d, unlinked=%d)\n",
		rep.Window.Since, rep.Window.Until, rep.Yield, rep.FirstPass, rep.N, rep.UnlinkedExcluded)
	return 0
}
