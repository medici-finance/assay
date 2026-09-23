package main

// Pure calibration logic for reviewer-calibration (verify-integrity/10).
//
// The reviewer App's APPROVED verdict gates the ready-flip and the model flip,
// and it has never been calibrated against an independent judge. This file holds
// the three load-bearing, side-effect-free pieces the calibration rests on, so
// each is unit-testable without a forge or a live model:
//
//  1. SeededSample — a deterministic, uniform sample of the month's APPROVED PRs.
//     Same seed → same ten PRs, so a calibration run is reproducible and its
//     sample auditable; a recorded seed is the difference between "we sampled"
//     and "we can prove which ten we sampled".
//
//  2. CheckIndependence — the SPOF. If the re-reviewer is the same model VENDOR
//     as the reviewer App, agreement measures two instances of one model, not
//     the gate against an independent judge. The whole exercise is worthless
//     without this check, so it is a refusal, not a warning, and it is a pure
//     function of (reviewer vendor, re-reviewer identity).
//
//  3. Report.Render — the agreement metric as an explicit numerator/denominator
//     FRACTION, never a bare percentage. A "90%" hides its denominator, and a
//     9/10 sample and a 900/1000 sample carry very different weight; the brief's
//     Verify row 4 pins the fraction shape for exactly this reason.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// DefaultSampleSize is the monthly sample the brief fixes: ten PRs APPROVED by
// the reviewer App in the prior calendar month.
const DefaultSampleSize = 10

// normalizeVendor folds a vendor label to its comparison form. Vendor names are
// compared case- and whitespace-insensitively so "Anthropic", "anthropic " and
// "anthropic" are one vendor — the roster and a --vendor flag are typed by
// different hands.
func normalizeVendor(v string) string { return strings.ToLower(strings.TrimSpace(v)) }

// ReReviewer identifies who (or what) performs the independent re-review of the
// sampled PRs. Exactly one of Vendor / Human is set: a model re-reviewer carries
// a Vendor, a human checklist carries a Human login. A human is independent of
// any model vendor by construction, which is why the two live in one type — the
// SPOF is "different vendor OR a human", and both satisfy it.
type ReReviewer struct {
	Vendor string // model vendor, e.g. "deepseek" | "openai"; empty for a human
	Human  string // human login; empty for a model re-reviewer
}

// isHuman reports whether this re-reviewer is a human checklist.
func (rr ReReviewer) isHuman() bool { return strings.TrimSpace(rr.Human) != "" }

// Identity is the re-reviewer identity recorded in the report — never a bare
// vendor that could be mistaken for the reviewer role's own binding.
func (rr ReReviewer) Identity() string {
	if rr.isHuman() {
		return "human:" + strings.TrimSpace(rr.Human)
	}
	return "vendor:" + normalizeVendor(rr.Vendor)
}

// CheckIndependence enforces the calibration SPOF: the re-review must come from a
// DIFFERENT model vendor than the reviewer role's App, or from a human. It
// returns a refusal error when independence cannot be positively established, so
// a caller can map it to ExitRefused. This is deliberately fail-closed: an
// unknown reviewer vendor is a refusal, not a silent pass, because a sample whose
// independence was never proven is exactly the same failure the whole brief
// exists to prevent.
func CheckIndependence(reviewerVendor string, rr ReReviewer) error {
	// A human checklist is independent of every model vendor by construction.
	if rr.isHuman() {
		return nil
	}
	reviewer := normalizeVendor(reviewerVendor)
	reReviewer := normalizeVendor(rr.Vendor)
	if reviewer == "" {
		return deskkit.Refused(fmt.Sprintf(
			"reviewer role vendor unknown — set %s in roster.env; a re-review cannot be proven independent of a vendor that is not recorded",
			deskkit.EnvReviewerVendor))
	}
	if reReviewer == "" {
		return deskkit.Refused(
			"re-reviewer vendor unset — pass --vendor <name> for a model re-review, or --human <login> for a human checklist")
	}
	if reviewer == reReviewer {
		return deskkit.Refused(fmt.Sprintf(
			"re-reviewer vendor %q equals the reviewer role's vendor (%s=%q): a same-vendor re-review measures agreement between two instances of ONE model, not against an independent judge (calibration SPOF) — pick a different vendor or a human login",
			rr.Vendor, deskkit.EnvReviewerVendor, reviewerVendor))
	}
	return nil
}

// SeededSample returns n PR numbers sampled uniformly WITHOUT replacement from
// candidates, deterministically for a given seed. The same seed over the same
// candidate set yields the same set (so a run is reproducible from its recorded
// seed); a different seed generally yields a different set. Output is sorted
// ascending so two runs with the same seed are byte-identical regardless of
// candidate input order.
//
// The sampler runs its own splitmix64 PRNG rather than math/rand, so the mapping
// from (seed, candidates) to sample is pinned by THIS code and cannot shift under
// a standard-library rand change — a recorded seed must reproduce its sample for
// as long as the report claims it does.
func SeededSample(candidates []int, seed int64, n int) ([]int, error) {
	if n <= 0 {
		return nil, deskkit.Refused(fmt.Sprintf("sample size must be positive, got %d", n))
	}
	// Canonicalise: dedupe + sort, so the starting order is a pure function of the
	// SET of candidates, never the order they arrived in.
	seen := map[int]struct{}{}
	pool := make([]int, 0, len(candidates))
	for _, c := range candidates {
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		pool = append(pool, c)
	}
	sort.Ints(pool)
	if len(pool) < n {
		return nil, deskkit.Refused(fmt.Sprintf(
			"only %d distinct APPROVED PRs in the window, need %d for a calibration sample", len(pool), n))
	}
	// Partial Fisher–Yates over the first n slots, indices drawn from a pinned
	// PRNG seeded by `seed`.
	state := uint64(seed) + 0x9e3779b97f4a7c15
	next := func() uint64 {
		state += 0x9e3779b97f4a7c15
		z := state
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		return z ^ (z >> 31)
	}
	for i := 0; i < n; i++ {
		span := uint64(len(pool) - i)
		j := i + int(next()%span)
		pool[i], pool[j] = pool[j], pool[i]
	}
	out := append([]int(nil), pool[:n]...)
	sort.Ints(out)
	return out, nil
}

// PRVerdict is one sampled PR's re-review outcome.
type PRVerdict struct {
	PR int `json:"pr"`
	// ReReviewerApproves is true when the independent re-reviewer would ALSO have
	// approved the PR — i.e. it agrees with the reviewer App's APPROVED verdict.
	ReReviewerApproves bool `json:"reReviewerApproves"`
	// MissedFindings lists, for a disagreement, the findings the re-reviewer would
	// have raised that the App's APPROVED verdict missed.
	MissedFindings []string `json:"missedFindings,omitempty"`
}

// Report is one month's calibration result, ready to render to the monthly
// report file.
type Report struct {
	Month          string      `json:"month"` // "YYYY-MM"
	Seed           int64       `json:"seed"`
	ReviewerVendor string      `json:"reviewerVendor"`
	ReReviewer     ReReviewer  `json:"reReviewer"`
	Sample         []int       `json:"sample"`
	Verdicts       []PRVerdict `json:"verdicts"`
}

// Agreement returns the numerator and denominator of the agreement fraction:
// (PRs the re-reviewer would also have approved) / (PRs re-reviewed).
func (r Report) Agreement() (agree, total int) {
	for _, v := range r.Verdicts {
		total++
		if v.ReReviewerApproves {
			agree++
		}
	}
	return agree, total
}

// Render produces the monthly report markdown. It refuses to render a report
// whose independence was not established (the SPOF is enforced at render time as
// well as at sample time, so a report file can never exist without it) and a
// report with no re-review verdicts (an empty denominator is a could-not-check,
// never an agreement of 0/0).
//
// The agreement metric is emitted ONLY as an explicit `agreement: <n>/<d>`
// fraction — never a bare percentage (brief Verify row 4).
func (r Report) Render() (string, error) {
	if err := CheckIndependence(r.ReviewerVendor, r.ReReviewer); err != nil {
		return "", err
	}
	agree, total := r.Agreement()
	if total == 0 {
		return "", deskkit.Refused(
			"no re-review verdicts — agreement is a could-not-check, never 0/0; run the re-review before rendering the report")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Reviewer calibration — %s\n\n", r.Month)
	fmt.Fprintf(&b, "reviewer-vendor: %s\n", normalizeVendor(r.ReviewerVendor))
	fmt.Fprintf(&b, "re-reviewer: %s\n", r.ReReviewer.Identity())
	fmt.Fprintf(&b, "seed: %d\n", r.Seed)
	fmt.Fprintf(&b, "sample-size: %d\n", total)
	// The one metric, as a fraction. Numerator and denominator both present; no
	// bare percentage anywhere in the file.
	fmt.Fprintf(&b, "agreement: %d/%d\n\n", agree, total)

	fmt.Fprintf(&b, "## Sample\n\n")
	sample := append([]int(nil), r.Sample...)
	sort.Ints(sample)
	for _, pr := range sample {
		fmt.Fprintf(&b, "- #%d\n", pr)
	}

	fmt.Fprintf(&b, "\n## Disagreements\n\n")
	any := false
	for _, v := range r.Verdicts {
		if v.ReReviewerApproves {
			continue
		}
		any = true
		fmt.Fprintf(&b, "- #%d — re-reviewer would NOT have approved\n", v.PR)
		for _, f := range v.MissedFindings {
			fmt.Fprintf(&b, "  - missed: %s\n", f)
		}
	}
	if !any {
		fmt.Fprintf(&b, "- none — the re-reviewer agreed with every sampled APPROVED verdict\n")
	}
	return b.String(), nil
}
