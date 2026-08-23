package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// dispatchqueue.go — the `--next-up` JSON emitter (#321).
//
// `--gate-scores` emits the awaiting-VERIFICATION backlog (implemented/verified
// only, by construction — see gateScores). That is exactly the population a
// dispatcher must NOT hand out. This emitter answers the other question: what work
// can a dispatcher START right now?
//
// It runs the SAME nextUp() selection the STATUS.md board uses — briefs at
// `todo`/`in-progress`, eligible, UNCLAIMED (no open-branch claim), per-stream +
// span-of-control capped, drive-steered — and emits the resulting DISPATCH queue as
// JSON together with the held-back decomposition, so a consumer (deskboard
// dispatch) can render the cross-repo queue without re-deriving selection. Reusing
// nextUp() rather than re-implementing it is the whole point: any change to the
// selection (e.g. a change to claim resolution) flows through here automatically.
//
// STATUS.md-free, exactly like --gate-scores: it never reads or writes the
// generated board. Unlike --gate-scores it DOES resolve claims — a dispatch queue
// that still lists a brief already claimed by an open branch is not a dispatch
// queue — and it reports whether that read succeeded in `claimsKnown`, so a caller
// can refuse to dispatch from an unfiltered superset.

// dispatchRow is one dispatchable pick: a todo/in-progress brief that is eligible
// and unclaimed, after per-stream and span-of-control capping.
type dispatchRow struct {
	Brief        string `json:"brief"` // "<stream>/<NN>"
	Stream       string `json:"stream"`
	Status       string `json:"status"` // todo | in-progress
	Score        int    `json:"score"`  // Total(): base score + any active-drive term
	BlockedCount int    `json:"blockedCount"`
	Repo         string `json:"repo,omitempty"`
	CriticalArm  string `json:"criticalArm,omitempty"` // set iff an active drive marks it critical
	DriveSlug    string `json:"driveSlug,omitempty"`   // the drive that supplied the steer, if any
}

// dispatchView is the whole --next-up payload: the dispatchable rows PLUS the
// held-back decomposition, so an empty `rows` is distinguishable from a throttled
// one. Every count mirrors a field the real Next-up board renders (nextup.go), so
// the emitter and STATUS.md agree on WHY a brief is not shown.
type dispatchView struct {
	Repo              string        `json:"repo,omitempty"`
	Rows              []dispatchRow `json:"rows"`
	Eligible          int           `json:"eligible"`        // total eligible before capping
	Shown             int           `json:"shown"`           // len(Rows)
	Span              int           `json:"span"`            // span-of-control cap applied
	HeldByStreamCap   int           `json:"heldByStreamCap"` // held back by per-stream caps
	HeldBySpan        int           `json:"heldBySpan"`      // held back by the span cap
	HeldByDriveCap    int           `json:"heldByDriveCap"`  // held back by the drive anti-starvation floor
	ClaimsKnown       bool          `json:"claimsKnown"`     // false => rows are an UNFILTERED superset
	ClaimsReason      string        `json:"claimsReason,omitempty"`
	SerializedUnknown []string      `json:"serializedUnknown,omitempty"`
	MeasuresGated     []string      `json:"measuresGated,omitempty"`
	MeasuresUnknown   []string      `json:"measuresUnknown,omitempty"`
}

// runNextUp loads streams, runs the SAME nextUp() selection the STATUS.md board
// uses, and emits the DISPATCH queue (todo/in-progress, unclaimed, eligible) as
// JSON with its held-back decomposition. STATUS.md-free.
//
// It mirrors main()'s board wiring so this queue is faithful to the one STATUS.md
// renders: findings feed the critical-tier reviewer arm, the per-stream git touch
// and the historian drive the staleness clock, and the drive manifest steers
// ranking — all through the same package vars nextUp() reads.
func runNextUp(root string) int {
	streams, findings, err := loadHydratedStreams(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	applyFindings(streams, findings)
	// Set explicitly every run so a prior invocation's value can never leak in;
	// nil is the inert default (the reviewer-finding critical arm only fires when a
	// drive is active).
	activeFindings = findings
	for _, s := range streams {
		rel, _ := filepath.Rel(root, s.Dir)
		s.LastTouch = gitLastTouch(root, rel)
	}
	claimed, claimSource := resolveClaims(root, streams)
	// Fail-closed opt-in (parity with the STATUS.md path): a caller that must never
	// dispatch from an unfiltered board sets --require-claims and gets exit 1 instead
	// of a degraded superset.
	if !claimSource.Known && requireClaims {
		fmt.Fprintln(os.Stderr, "statusgen: --next-up: claim filtering could not be established and --require-claims is set: "+claimSource.reason())
		return 1
	}
	briefTouch := map[string]time.Time{}
	if entries, err := LoadHistory(filepath.Join(root, filepath.FromSlash(historyRelPath))); err == nil {
		briefTouch = LastTransitionTime(entries)
	}
	activeDriveSet = loadDrives(root, streams, nowFunc())
	nu := nextUp(streams, ClaimView{Claimed: claimed, Source: claimSource}, briefTouch)

	// The root's declared repo, carried on the view AND every row so a cross-repo
	// aggregator (deskboard dispatch) can attribute a brief from the DATA rather than
	// from whichever root it happened to invoke. A malformed/conflicting declaration
	// is a hard PROBLEM in --lint; here it just yields "" and the field is omitted.
	repo, _ := rootRepo(streams)
	view := buildDispatchView(nu, streams, repo, claimSource)
	out, err := json.Marshal(view)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statusgen:", err)
		return 1
	}
	fmt.Println(string(out))
	return 0
}

// buildDispatchView maps a computed NextUp into the JSON emitter's payload: the
// dispatchable picks plus the held-back decomposition. Pure over its inputs (no
// file/git IO), so the selection surfaced by --next-up — todo/in-progress and
// unclaimed shown, implemented/verified/claimed absent — is unit-testable without a
// filesystem. runNextUp does the IO and hands the result here.
func buildDispatchView(nu NextUp, streams []*Stream, repo string, claimSource ClaimSource) dispatchView {
	// blockedCount per SHOWN pick: the reverse-dep graph is the same machinery
	// nextUp/gateScores use; recompute for the shown set (cheap) so each row carries
	// how many open briefs it unblocks.
	rev, status := buildRevDeps(streams)
	view := dispatchView{
		Repo:              repo,
		Rows:              make([]dispatchRow, 0, len(nu.Picks)),
		Eligible:          nu.Eligible,
		Shown:             len(nu.Picks),
		Span:              nu.Span,
		HeldByStreamCap:   nu.HeldByStreamCap,
		HeldBySpan:        nu.HeldBySpan(),
		HeldByDriveCap:    nu.HeldByDriveCap,
		ClaimsKnown:       claimSource.Known,
		ClaimsReason:      claimReasonWhenDegraded(claimSource),
		SerializedUnknown: nu.SerializedUnknown,
		MeasuresGated:     nu.MeasuresGated,
		MeasuresUnknown:   nu.MeasuresUnknown,
	}
	for _, p := range nu.Picks {
		id := p.Stream.Name + "/" + p.Brief.Num
		view.Rows = append(view.Rows, dispatchRow{
			Brief:        id,
			Stream:       p.Stream.Name,
			Status:       p.Brief.Status,
			Score:        p.Total(),
			BlockedCount: blockedCount(rev, status, id),
			Repo:         repo,
			CriticalArm:  p.CriticalArm,
			DriveSlug:    p.DriveSlug,
		})
	}
	return view
}

// claimReasonWhenDegraded returns the human reason ONLY when the claim read did not
// run, so a `claimsKnown:true` view carries no spurious reason string.
func claimReasonWhenDegraded(c ClaimSource) string {
	if c.Known {
		return ""
	}
	return c.reason()
}
