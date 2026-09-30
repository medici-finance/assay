package main

// query.go — the ONE ordering every rendering (table today; walk/html in the follow-up)
// reads from, ported from the oracle's per-label `gh issue list` fan-out + its
// `unique_by`/rank/`sort_by` jq pipeline (assay-inbox.sh:333-386).
//
// MECHANISM DIVERGES FROM THE ORACLE; THE ORDERING DOES NOT. The oracle issues one
// `gh issue list --label <L>` call per label per repo (4 calls/repo) and unions the
// results. This reads each repo's OPEN issues ONCE via the resolved forge's
// ListOpenIssues (the frozen op cmd/issueboard already consumes) and filters to the same
// four labels client-side — fewer calls, and the same union: an issue is included iff it
// carries at least one of LABELS, exactly as the per-label fan-out would include it. The
// one observable difference is the truncation THRESHOLD: the oracle caps each label query
// at --limit (default 500) and warns per label; ListOpenIssues caps the repo's WHOLE open
// list at 10,000 (forgeMaxIssuePages*forgeIssuePerPage) and refuses (could-not-check)
// past it — a repo would need more open issues than that, escalation-labelled or not,
// before the two mechanisms could disagree. testdata/spec.md records this as a deliberate,
// bounded divergence.

import (
	"sort"
	"strconv"
	"strings"
)

// labels is the escalation-contract label set, rank-ordered most urgent first — LABELS in
// the oracle. Keep in exact sync with the escalation contract; do not add/reorder without
// re-reading it (assay-inbox.sh:49-59).
var labels = []string{"urgent", "needs-decision", "question", "help wanted"}

// item is one row of the sorted queue — the oracle's $TMP_SORTED entry.
type item struct {
	Repo      string
	Number    int
	Title     string
	URL       string
	CreatedAt string // RFC3339, as the forge reported it
	Labels    []string
	Rank      int // lower = more urgent; 99 = carries none of `labels` (never emitted here)
}

// rankOf returns the lowest index into `labels` any of names carries, or 99 when none
// match — the oracle's `$rankorder | index($n) | min // 99`.
func rankOf(names []string) int {
	best := 99
	for _, n := range names {
		for i, l := range labels {
			if n == l && i < best {
				best = i
			}
		}
	}
	return best
}

// hasEscalationLabel reports whether names carries at least one of `labels`.
func hasEscalationLabel(names []string) bool {
	return rankOf(names) != 99
}

// queryFailure is one repo's failed read, surfaced on stderr exactly as the oracle's
// "QUERY FAILED" line names the repo and the underlying error — never silently dropped
// from the count that reddens the exit code.
type queryFailure struct {
	Repo string
	Err  error
}

// fetchQueue reads every repo's open issues, filters to the escalation label set, dedupes
// by repo+number (unique_by), and sorts urgency-then-age (rank, then oldest createdAt
// first within the same rank) — the oracle's ONE ordering every renderer reads from
// (assay-inbox.sh:364-386). A repo whose read failed is counted in `failures` and excluded
// from the queue, never silently treated as empty.
func fetchQueue(repos []string) (sorted []item, failures []queryFailure) {
	for _, repo := range repos {
		if strings.TrimSpace(repo) == "" {
			continue
		}
		f, fr, ferr := forgeFor(repo)
		if ferr != nil {
			failures = append(failures, queryFailure{Repo: repo, Err: ferr})
			continue
		}
		issues, lerr := f.ListOpenIssues(fr)
		if lerr != nil {
			failures = append(failures, queryFailure{Repo: repo, Err: lerr})
			continue
		}
		for _, is := range issues {
			if !hasEscalationLabel(is.Labels) {
				continue
			}
			sorted = append(sorted, item{
				Repo:      repo,
				Number:    is.Number,
				Title:     is.Title,
				URL:       is.URL,
				CreatedAt: is.CreatedAt,
				Labels:    append([]string(nil), is.Labels...),
				Rank:      rankOf(is.Labels),
			})
		}
	}

	// unique_by(.repo + "#" + .number): ListOpenIssues already reads one repo's issues
	// once, so within a repo there is nothing to dedupe — this guards a caller that passed
	// the same repo twice on the command line.
	seen := make(map[string]bool, len(sorted))
	deduped := sorted[:0]
	for _, it := range sorted {
		key := it.Repo + "#" + strconv.Itoa(it.Number)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, it)
	}
	sorted = deduped

	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Rank != sorted[j].Rank {
			return sorted[i].Rank < sorted[j].Rank
		}
		return sorted[i].CreatedAt < sorted[j].CreatedAt
	})
	return sorted, failures
}
