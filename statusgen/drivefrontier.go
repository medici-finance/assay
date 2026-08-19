package main

import (
	"strings"
	"time"
)

// drivefrontier.go — methodology-metrics phase 2 (runsheet 2.4): the DRIVE
// LIFECYCLE + FRONTIER. Phase 1 (drives.go) loads an operator-declared campaign
// and adds its additive steer to the Next-up score. Phase 2 reads the SAME active
// DriveSet and answers two questions, deterministically and with NO side effects:
//
//   - FRONTIER — classify every drive item as
//       done | in-flight | ready | blocked-on(item|review|operator-act|needs-brief)
//     by resolving stream/brief items against the loaded board (status + typed
//     deps + the claim signal), exactly like Next-up eligibility. issue items are
//     external (cross-repo) and could-not-check from THIS tree, so they are
//     TRACKED — informational, never a stall.
//
//   - STATE — derive the drive's posture
//       ROLLING | WAITING-ON-OPERATOR | WAITING-ON-REVIEW | STUCK
//     from the frontier. The state is what the tracking issue and the @operator push
//     channel key on (driveissues.go).
//
// Safety invariants are unchanged from phase 1. This file reads only the loaded
// board and the ONE sanctioned wall-clock (`now`, at UTC-day granularity, for
// operator-act aging — driveDay, drives.go). It NEVER opens an issue, posts a
// comment, or feeds the wall clock into scoring; the drive term still comes from
// phase 1 alone. Nothing here touches the Next-up board or STATUS.md, so a
// no-manifest board stays byte-identical to the pre-drives baseline.

// Drive lifecycle states (brief-44 "Lifecycle"). The state is derived, never
// declared — an operator declares a campaign, and statusgen reads the board to
// say whether the fleet is rolling, waiting on the operator, waiting on review,
// or genuinely stuck.
const (
	driveStateRolling    = "ROLLING"             // dispatchable / in-flight work exists — the fleet self-organizes
	driveStateWaitingOp  = "WAITING-ON-OPERATOR" // the operator is THE bottleneck (the @operator push channel)
	driveStateWaitingRev = "WAITING-ON-REVIEW"   // progress is gated only on review / sign-off
	driveStateStuck      = "STUCK"               // blocked with nothing dispatchable and not on operator/review
)

// Frontier item states (brief-44 frontier grammar). blocked-on(needs-brief) is a
// dispatchable authoring gap, NEVER a stall — it is progress-enabling, so it
// counts toward ROLLING.
const (
	fsDone            = "done"
	fsInFlight        = "in-flight"
	fsReady           = "ready"
	fsBlockedItem     = "blocked-on(item)"
	fsBlockedReview   = "blocked-on(review)"
	fsBlockedOperator = "blocked-on(operator-act)"
	fsNeedsBrief      = "blocked-on(needs-brief)"
	fsTracked         = "tracked" // external owner/repo#N — could-not-check from this tree
)

// FrontierItem is one resolved drive-item row with its lifecycle classification.
type FrontierItem struct {
	Kind     string // brief | issue | operator-act | plan-gap
	Ref      string // "stream/NN" or "owner/repo#N"; "" for operator-act / plan-gap
	State    string // one of the fs* constants
	Unblocks string // operator-act: what it unblocks
	Since    string // operator-act: the YYYY-MM-DD its aging clock started
	AgeDays  int    // operator-act: whole UTC-days from Since to `now` (>=0)
}

// DriveStatus bundles a live drive with its computed frontier and derived state —
// the phase-2 view the tracking-issue emitter and (phase 4) the dashboard render.
type DriveStatus struct {
	Drive    Drive
	Frontier []FrontierItem
	State    string
}

// briefFrontierState classifies one brief for the frontier. It mirrors Next-up
// eligibility (eligibleBase/depIsSatisfied) so the frontier's "ready" agrees with
// what the board would actually offer, and layers the awaiting states on top:
// implemented ⇒ blocked-on(review) (a PR is up, awaiting a verdict), verified ⇒
// done (the review passed; the done-close is bookkeeping, not a fleet blocker).
func briefFrontierState(streams []*Stream, s *Stream, b Brief, claimed map[string]bool) string {
	id := s.Name + "/" + b.Num
	switch b.Status {
	case "done", "verified":
		return fsDone
	case "implemented":
		return fsBlockedReview
	case "in-progress":
		return fsInFlight
	case "blocked":
		return fsBlockedItem
	}
	// todo. A claim (open origin branch/PR) means the item is already in flight, so
	// it is NOT offered again — the same signal Next-up capping uses.
	if claimed[id] {
		return fsInFlight
	}
	if b.Schema == "placeholder-v1" {
		if b.Blocked != "" {
			return fsBlockedItem // awaiting an issue response
		}
		return fsReady
	}
	if b.Schema == "brief-v1" {
		for _, dep := range b.Depends {
			if !depIsSatisfied(streams, dep) {
				return fsBlockedItem
			}
		}
		return fsReady // empty / satisfied deps
	}
	// legacy (non-brief-v1): whole-wave gating in the same stream.
	for _, o := range s.Briefs {
		if o.Wave < b.Wave && o.Status != "done" && o.Status != "verified" {
			return fsBlockedItem
		}
	}
	return fsReady
}

// operatorActSince returns the date an operator-act's aging clock started: its own
// `since:` when set, else the drive's `starts` (a fresh drive's acts age from the
// day the campaign opened).
func operatorActSince(d Drive, it DriveItem) string {
	if s := strings.TrimSpace(it.Since); s != "" {
		return s
	}
	return d.Starts
}

// operatorActAgeDays is the whole-UTC-day age of an operator-act at `now`. Both
// ends truncate to UTC midnight (driveDay) — the same one-board-day granularity as
// the drive window (drives.go), so two regenerations on the same day agree. An
// unparseable date ages to 0 rather than panicking (the loader already validated
// a present `since:`; `starts` was validated in phase 1).
func operatorActAgeDays(d Drive, it DriveItem, now time.Time) int {
	t, err := time.Parse("2006-01-02", operatorActSince(d, it))
	if err != nil {
		return 0
	}
	days := int(driveDay(now).Sub(driveDay(t)).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return days
}

// driveFrontier resolves every item of one drive into a FrontierItem. A stream
// item expands to one row per brief in the stream (the honest, granular frontier);
// a brief item resolves the single brief; an issue item is tracked (external);
// operator-act and plan-gap rows carry through with their aging / needs-brief
// classification. The loader (phase 1) already validated that stream/brief refs
// resolve, so a miss here is a defensively-skipped row, never a panic.
func driveFrontier(d Drive, streams []*Stream, claimed map[string]bool, now time.Time) []FrontierItem {
	byName := map[string]*Stream{}
	for _, s := range streams {
		byName[s.Name] = s
	}
	var out []FrontierItem
	for _, it := range d.Items {
		switch it.Kind {
		case "stream":
			s := byName[it.Ref]
			if s == nil {
				continue
			}
			for _, b := range s.Briefs {
				out = append(out, FrontierItem{
					Kind:  "brief",
					Ref:   s.Name + "/" + b.Num,
					State: briefFrontierState(streams, s, b, claimed),
				})
			}
		case "brief":
			parts := strings.SplitN(it.Ref, "/", 2)
			if len(parts) != 2 {
				continue
			}
			s := byName[parts[0]]
			if s == nil {
				continue
			}
			for _, b := range s.Briefs {
				if b.Num == parts[1] {
					out = append(out, FrontierItem{
						Kind:  "brief",
						Ref:   it.Ref,
						State: briefFrontierState(streams, s, b, claimed),
					})
				}
			}
		case "issue":
			out = append(out, FrontierItem{Kind: "issue", Ref: it.Ref, State: fsTracked})
		case "operator-act":
			out = append(out, FrontierItem{
				Kind:     "operator-act",
				State:    fsBlockedOperator,
				Unblocks: it.Unblocks,
				Since:    operatorActSince(d, it),
				AgeDays:  operatorActAgeDays(d, it, now),
			})
		case "plan-gap":
			out = append(out, FrontierItem{Kind: "plan-gap", State: fsNeedsBrief})
		}
	}
	return out
}

// driveState derives the drive's posture from its frontier (brief-44 "Lifecycle").
//
// ROLLING wins whenever ANY item is dispatchable or in flight — even if other
// items are operator-blocked. This is the "classify the blocker and take
// other-stream work" rule: the operator is only pinged when they are THE
// bottleneck (nothing else can progress), so WAITING-ON-OPERATOR fires only once
// the ready/in-flight set is empty. Precedence below ROLLING: operator > review >
// stuck. A frontier with no blockers and no ready work (all done, or issue-tracked
// only, or empty) is ROLLING — it is not waiting on anyone.
func driveState(frontier []FrontierItem) string {
	var ready, operator, review, blockedItem bool
	for _, f := range frontier {
		switch f.State {
		case fsReady, fsInFlight, fsNeedsBrief:
			ready = true
		case fsBlockedOperator:
			operator = true
		case fsBlockedReview:
			review = true
		case fsBlockedItem:
			blockedItem = true
		}
	}
	switch {
	case ready:
		return driveStateRolling
	case operator:
		return driveStateWaitingOp
	case review:
		return driveStateWaitingRev
	case blockedItem:
		return driveStateStuck
	default:
		return driveStateRolling
	}
}

// driveStatuses computes the frontier + state for every active drive in the set,
// in the loader's deterministic slug order. Empty when no drive is active.
func driveStatuses(ds DriveSet, streams []*Stream, claimed map[string]bool, now time.Time) []DriveStatus {
	var out []DriveStatus
	for _, d := range ds.Active {
		fr := driveFrontier(d, streams, claimed, now)
		out = append(out, DriveStatus{Drive: d, Frontier: fr, State: driveState(fr)})
	}
	return out
}

// progress counts done frontier BRIEF items over the total resolvable brief items
// (issue/operator-act/plan-gap rows are excluded — they have no board-derived done
// signal). total is 0 when a drive names only external/operator work.
func (st DriveStatus) progress() (done, total int) {
	for _, f := range st.Frontier {
		if f.Kind != "brief" {
			continue
		}
		total++
		if f.State == fsDone {
			done++
		}
	}
	return done, total
}

// operatorActs returns the operator-act frontier rows (the WAITING-ON-YOU slice),
// preserving manifest order.
func (st DriveStatus) operatorActs() []FrontierItem {
	var acts []FrontierItem
	for _, f := range st.Frontier {
		if f.Kind == "operator-act" {
			acts = append(acts, f)
		}
	}
	return acts
}

// oldestActDays is the age of the longest-waiting operator-act, 0 when there are
// none. It drives the 24h→72h re-ping escalation (driveissues.go).
func (st DriveStatus) oldestActDays() int {
	oldest := 0
	for _, f := range st.operatorActs() {
		if f.AgeDays > oldest {
			oldest = f.AgeDays
		}
	}
	return oldest
}
