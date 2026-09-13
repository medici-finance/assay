package deskkit

// provenance.go — the contributor provenance probe.
//
// WHY. When a pull request arrives from an identity the roster does not know, the only
// instrument a maintainer has is whatever they happen to notice in the thirty seconds before
// they decide whether to look. Every signal here is already public metadata about the
// SUBMISSION and its timing, never about the person, and this file gathers it and states it
// as facts. See docs/contributor-provenance.md for the published description of what this
// probe measures and why, and the design record it was authored against.
//
// THE BOUND ON THIS CLAIM, STATED. This file renders no score, no rating and no verdict —
// there is no aggregate field anywhere on [SignalResult] or [ProvenanceInput] a caller could
// sum, average or otherwise combine into a figure, so a caller cannot add one without
// changing the type. [Overall] folds many three-state signals into one three-state card exit,
// which is a fold over states (clean/flagged/could-not-check), never a fold over values.
//
// PURE OVER INJECTED READERS. Every signal here is a function of [ProvenanceInput], which the
// caller populates from a live forge reader or from a fixture; this file makes no network call
// and never fetches, checks out, builds or executes anything. A nil field on ProvenanceInput
// means the signal it feeds could not be read — it is rendered as could-not-check, never as a
// real zero.
//
// EXCLUDED, BY DESIGN AND BY TEST. Profile text, avatar, follower or following counts, named
// employer, geography, account name shape, and anything else that describes the PERSON rather
// than the submission never appears in [signalRegistry] — see TestProvenanceExcludedSignals in
// provenance_test.go, which fails the build the day one of those categories is added here.

import (
	"fmt"
	"strings"
	"time"
)

// ProvenanceState is the three-state exit of one signal, or of a whole card.
type ProvenanceState int

const (
	// StateClean means the signal was measured and fell outside its notable band, or (for
	// a whole card) that every signal was measured and none did.
	StateClean ProvenanceState = iota
	// StateFlagged means the signal was measured and fell inside its notable band, or (for
	// a whole card) that at least one signal did, with no unreadable signal alongside it.
	StateFlagged
	// StateCouldNotCheck means the signal could not be measured at all, or (for a whole
	// card) that at least one signal could not be. This state DOMINATES: a card carrying
	// even one could-not-check signal is never rendered clean or flagged — a partially
	// blind card is not a clean one, and it is not a confidently flagged one either.
	StateCouldNotCheck
)

// String renders the state the way the card and the CLI both spell it — "could-not-check" as
// one hyphenated word, matching the brief's own vocabulary and every Verify row that greps it.
func (s ProvenanceState) String() string {
	switch s {
	case StateClean:
		return "clean"
	case StateFlagged:
		return "flagged"
	case StateCouldNotCheck:
		return "could-not-check"
	default:
		return "unknown"
	}
}

// ExitCode maps a card's overall [ProvenanceState] to the process exit code
// cmd/deskprovenance uses: 0 clean, 1 flagged, 6 could-not-check.
func (s ProvenanceState) ExitCode() int {
	switch s {
	case StateClean:
		return 0
	case StateFlagged:
		return 1
	default:
		return 6
	}
}

// ProvenanceInput is the injected raw metadata this file measures. Every field is nil-able:
// a nil field means the signal(s) it feeds could not be read, never that the answer is zero.
// Both a live forge reader and a JSON fixture (cmd/deskprovenance's --fixture) produce this
// same shape, and [Gather] cannot tell which one it was handed — that is the point.
type ProvenanceInput struct {
	// AccountCreatedAt and FirstActivityAt feed the account-age-at-first-activity signal.
	AccountCreatedAt *time.Time `json:"account_created_at,omitempty"`
	FirstActivityAt  *time.Time `json:"first_activity_at,omitempty"`

	// ForkCreatedAt and PROpenedAt feed the fork-to-pull-request signal.
	ForkCreatedAt *time.Time `json:"fork_created_at,omitempty"`
	PROpenedAt    *time.Time `json:"pr_opened_at,omitempty"`

	// CrossRepoPRCount24h feeds the cross-repository burst signal.
	CrossRepoPRCount24h *int `json:"cross_repo_pr_count_24h,omitempty"`

	// PriorMergedCount and PriorClosedCount feed the merged/closed ratio signal.
	PriorMergedCount *int `json:"prior_merged_count,omitempty"`
	PriorClosedCount *int `json:"prior_closed_count,omitempty"`

	// ThisBody and RecentBodies feed the body-similarity signal. RecentBodies nil means
	// unreadable; an empty-but-non-nil slice means readable and genuinely empty (no prior
	// bodies to compare against, which is not notable).
	ThisBody     *string   `json:"this_body,omitempty"`
	RecentBodies *[]string `json:"recent_bodies,omitempty"`

	// CommitsSigned feeds the commit-signature signal.
	CommitsSigned *bool `json:"commits_signed,omitempty"`

	// ChangedPaths feeds the continuous-integration/lockfile/install-script/container-path
	// signal. Nil means unreadable; an empty-but-non-nil slice means readable and genuinely
	// empty.
	ChangedPaths *[]string `json:"changed_paths,omitempty"`
}

// SignalResult is one signal's measurement. Exactly one of Detail (a measured fact) or Reason
// (why it could not be measured) is meaningful, selected by State. There is no field here a
// caller could sum, average, or otherwise fold into a score.
type SignalResult struct {
	// Name is the signal's stable identifier, e.g. "account-age-at-first-activity".
	Name string
	// State is the signal's own three-state result.
	State ProvenanceState
	// Detail is the measured fact, in words, when State is StateClean or StateFlagged.
	Detail string
	// Explanation is the signal's fixed, ordinary/innocent explanation — shown on the card
	// only when this signal is notable, so the reader gets the benign reading alongside the
	// measurement rather than having to supply it themselves.
	Explanation string
	// Reason is populated only when State is StateCouldNotCheck: why the signal could not
	// be read.
	Reason string
}

// signalSpec is one entry in [signalRegistry]: a name, a gatherer, a description of its
// notable band (for documentation and for the excluded-signal test), and its one-line
// ordinary explanation.
type signalSpec struct {
	// Name is the signal's stable identifier.
	Name string
	// NotableBand describes, in words, what makes this signal notable. It is documentation
	// and test fodder, never rendered on the card itself (the card renders Detail, which is
	// computed per-input by Gather, not this fixed description).
	NotableBand string
	// Explanation is the signal's fixed ordinary/innocent explanation.
	Explanation string
	// Gather measures the signal from in, returning its state, a human-legible detail (when
	// measured) and a could-not-check reason (when not).
	Gather func(in ProvenanceInput) (state ProvenanceState, detail string, reason string)
}

// signalRegistry is the FIXED, ordered list of every signal this probe measures. Order here
// is render order: Gather and Card both walk this slice in this order, never re-sorted by
// anything data-dependent, so the same input always renders the same card byte-for-byte.
//
// Every entry measures something about the SUBMISSION and its timing. None measures the
// person — see TestProvenanceExcludedSignals, which asserts that directly against a denylist
// so a signal added later without reading this comment still fails the build.
var signalRegistry = []signalSpec{
	{
		Name:        "account-age-at-first-activity",
		NotableBand: "the account's first observed activity fell within 24h of the account being created",
		Explanation: "a new account is how everybody starts; every established contributor's account was new once",
		Gather:      gatherAccountAge,
	},
	{
		Name:        "fork-to-pull-request-elapsed",
		NotableBand: "fewer than 5 minutes elapsed between the fork being created and the pull request being opened",
		Explanation: "a fast fork-to-pull-request gap is what a prepared patch, queued and ready to send, looks like",
		Gather:      gatherForkToPR,
	},
	{
		Name:        "cross-repository-burst",
		NotableBand: "5 or more pull requests opened by this author across repositories in the preceding 24h",
		Explanation: "a batch of pull requests across repositories in a short window is what a dependency-bump or template sweep looks like, and most such sweeps are genuine maintenance",
		Gather:      gatherCrossRepoBurst,
	},
	{
		Name:        "prior-merged-closed-ratio",
		NotableBand: "at least 3 prior submissions on record, fewer than 20% of them merged",
		Explanation: "a low merged ratio can simply mean the author's earlier proposals did not fit this particular project, not that the author acted in bad faith",
		Gather:      gatherMergedClosedRatio,
	},
	{
		Name:        "body-shape-similarity",
		NotableBand: "this submission's description is near-identical in shape to another of the author's recent submissions",
		Explanation: "a repeated or templated description is what an author fixing the same class of issue across several places looks like, which most maintainers welcome",
		Gather:      gatherBodySimilarity,
	},
	{
		Name:        "commit-signature",
		NotableBand: "the commits carried by this submission are not signed",
		Explanation: "most contributors do not sign their commits; this is the ordinary case, not a marker of automation",
		Gather:      gatherCommitSignature,
	},
	{
		Name:        "build-and-dependency-paths-touched",
		NotableBand: "the diff touches continuous-integration configuration, a dependency lockfile, an install script or a container definition",
		Explanation: "touching build or dependency configuration is very often exactly the intended, legitimate change",
		Gather:      gatherRiskyPaths,
	},
}

// riskyPathMarkers are substrings of a changed path that put it in the build/dependency-path
// band. This is a fixed, small set, deliberately: it names the FILE CLASS, never a specific
// project's layout.
var riskyPathMarkers = []string{
	".github/workflows/",
	"dockerfile",
	"docker-compose",
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"go.sum",
	"go.mod",
	"gemfile.lock",
	"requirements.txt",
	"poetry.lock",
	"install.sh",
	"makefile",
}

// Gather runs every registered signal over in, in the fixed [signalRegistry] order, and
// returns their results. It never computes or returns an aggregate.
func Gather(in ProvenanceInput) []SignalResult {
	out := make([]SignalResult, 0, len(signalRegistry))
	for _, spec := range signalRegistry {
		state, detail, reason := spec.Gather(in)
		out = append(out, SignalResult{
			Name:        spec.Name,
			State:       state,
			Detail:      detail,
			Explanation: spec.Explanation,
			Reason:      reason,
		})
	}
	return out
}

// Overall folds a set of signal results into the card's own three-state exit.
// could-not-check DOMINATES: a card carrying even one unreadable signal is never rendered
// clean, and it is never rendered as a plain flagged with no caveat either — the reader is
// told the card is partial. Otherwise: flagged if any signal is notable, else clean.
func Overall(results []SignalResult) ProvenanceState {
	sawCouldNotCheck := false
	sawFlagged := false
	for _, r := range results {
		switch r.State {
		case StateCouldNotCheck:
			sawCouldNotCheck = true
		case StateFlagged:
			sawFlagged = true
		}
	}
	if sawCouldNotCheck {
		return StateCouldNotCheck
	}
	if sawFlagged {
		return StateFlagged
	}
	return StateClean
}

// abstentionText is the card's closing statement. It renders on EVERY card, clean, flagged or
// could-not-check alike — never only on the happy path, which is the exact defect a flagged
// card with no caveat would be.
const abstentionText = "This card describes the shape of this submission; it is not a judgement of the change, and the change is reviewed on its merits."

// Card renders results as the neutral, deterministic comment body this probe posts. The same
// results always render the same bytes: no timestamp, no run identifier, nothing that would
// make two runs over identical input differ.
func Card(results []SignalResult) string {
	overall := Overall(results)
	var b strings.Builder
	b.WriteString("## Contributor provenance probe\n\n")
	fmt.Fprintf(&b, "State: %s\n\n", overall)
	b.WriteString("Facts observed about this submission's shape and timing — never about the person:\n\n")
	for _, r := range results {
		switch r.State {
		case StateCouldNotCheck:
			fmt.Fprintf(&b, "- **%s** — could-not-check: %s\n", r.Name, r.Reason)
		case StateFlagged:
			fmt.Fprintf(&b, "- **%s** — %s. Ordinary explanation: %s\n", r.Name, r.Detail, r.Explanation)
		default:
			fmt.Fprintf(&b, "- %s — %s\n", r.Name, r.Detail)
		}
	}
	b.WriteString("\n")
	b.WriteString(abstentionText)
	b.WriteString("\n")
	return b.String()
}

// roundDuration renders d in the single largest whole unit that fits, for a human-legible
// Detail string. Negative durations render as their absolute value: a clock skew between two
// recorded timestamps is a could-not-check-worthy oddity, not a sign to render "-3m".
func roundDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func gatherAccountAge(in ProvenanceInput) (ProvenanceState, string, string) {
	if in.AccountCreatedAt == nil || in.FirstActivityAt == nil {
		return StateCouldNotCheck, "", "the account's creation time or its first observed activity time was not available"
	}
	gap := in.FirstActivityAt.Sub(*in.AccountCreatedAt)
	detail := fmt.Sprintf("the account's first observed activity came %s after it was created", roundDuration(gap))
	if gap >= 0 && gap < 24*time.Hour {
		return StateFlagged, detail, ""
	}
	return StateClean, detail, ""
}

func gatherForkToPR(in ProvenanceInput) (ProvenanceState, string, string) {
	if in.ForkCreatedAt == nil || in.PROpenedAt == nil {
		return StateCouldNotCheck, "", "the fork's creation time or the pull request's open time was not available"
	}
	gap := in.PROpenedAt.Sub(*in.ForkCreatedAt)
	detail := fmt.Sprintf("%s elapsed between the fork being created and this pull request being opened", roundDuration(gap))
	if gap >= 0 && gap < 5*time.Minute {
		return StateFlagged, detail, ""
	}
	return StateClean, detail, ""
}

func gatherCrossRepoBurst(in ProvenanceInput) (ProvenanceState, string, string) {
	if in.CrossRepoPRCount24h == nil {
		return StateCouldNotCheck, "", "the count of pull requests this author opened across repositories in the preceding 24h was not available"
	}
	n := *in.CrossRepoPRCount24h
	detail := fmt.Sprintf("this author opened %d pull request(s) across repositories in the preceding 24h", n)
	if n >= 5 {
		return StateFlagged, detail, ""
	}
	return StateClean, detail, ""
}

func gatherMergedClosedRatio(in ProvenanceInput) (ProvenanceState, string, string) {
	if in.PriorMergedCount == nil || in.PriorClosedCount == nil {
		return StateCouldNotCheck, "", "the author's prior merged and closed submission counts were not available"
	}
	merged, closed := *in.PriorMergedCount, *in.PriorClosedCount
	total := merged + closed
	if total == 0 {
		return StateClean, "this author has no prior submissions on record to compute a merged/closed ratio from", ""
	}
	ratio := float64(merged) / float64(total)
	detail := fmt.Sprintf("%d of this author's %d prior submissions on record were merged (%.0f%%)", merged, total, ratio*100)
	if total >= 3 && ratio < 0.2 {
		return StateFlagged, detail, ""
	}
	return StateClean, detail, ""
}

func gatherBodySimilarity(in ProvenanceInput) (ProvenanceState, string, string) {
	if in.ThisBody == nil || in.RecentBodies == nil {
		return StateCouldNotCheck, "", "this submission's description or the author's other recent descriptions were not available"
	}
	this := normalizeForSimilarity(*in.ThisBody)
	if this != "" {
		for _, other := range *in.RecentBodies {
			if normalizeForSimilarity(other) == this {
				return StateFlagged, "this submission's description is near-identical in shape to another of the author's recent submissions", ""
			}
		}
	}
	return StateClean, "this submission's description does not match the shape of the author's other recent submissions", ""
}

// normalizeForSimilarity folds two descriptions onto the same shape when they differ only by
// whitespace or by the literal digits in a templated placeholder (e.g. an issue number), which
// is the shape a repeated or bulk-templated body actually takes.
func normalizeForSimilarity(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune('#')
		case r == ' ' || r == '\n' || r == '\t' || r == '\r':
			// collapsed, not kept
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func gatherCommitSignature(in ProvenanceInput) (ProvenanceState, string, string) {
	if in.CommitsSigned == nil {
		return StateCouldNotCheck, "", "whether this submission's commits carry a signature was not available"
	}
	if *in.CommitsSigned {
		return StateClean, "this submission's commits carry a signature", ""
	}
	return StateFlagged, "this submission's commits do not carry a signature", ""
}

func gatherRiskyPaths(in ProvenanceInput) (ProvenanceState, string, string) {
	if in.ChangedPaths == nil {
		return StateCouldNotCheck, "", "this submission's changed paths were not available"
	}
	var matched []string
	for _, p := range *in.ChangedPaths {
		lower := strings.ToLower(p)
		for _, marker := range riskyPathMarkers {
			if strings.Contains(lower, marker) {
				matched = append(matched, p)
				break
			}
		}
	}
	if len(matched) > 0 {
		return StateFlagged, fmt.Sprintf(
			"this diff touches continuous-integration configuration, a dependency lockfile, an install script or a container definition (%s)",
			strings.Join(matched, ", "),
		), ""
	}
	return StateClean, "this diff does not touch continuous-integration configuration, dependency lockfiles, install scripts or container definitions", ""
}
