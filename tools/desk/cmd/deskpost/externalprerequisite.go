package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// externalprerequisite.go is the READY-GATE half of the same-head external-prerequisite
// exemption (brief 21). The reader, the markers and the fail-closed decision
// live in deskkit (externalprerequisite.go); this file supplies the two things the
// decision cannot get for itself, at the READY BOUNDARY where the interface contract says
// the re-validation must happen:
//
//   - the REVIEWER-authored records only (the standing CR and the citing APPROVEs at head),
//     filtered from the review timeline by the authenticated reviewer-bot login so a
//     worker's prose can never author a clearance; and
//   - a FRESH, INDEPENDENT observation of each declared prerequisite, fetched now rather
//     than trusted from the reviewer's citation, so a stale cached external pass cannot
//     satisfy the gate.
//
// It is consulted ONLY on the branch where the ordinary rule would refuse: an APPROVED
// sitting over a standing CHANGES_REQUESTED at an unchanged head (ready.go's noOpApproval).
// It removes that block when — and only when — the CR declared itself external-prereq-only
// and every prerequisite independently verifies; it never manufactures an APPROVED, so the
// governing-verdict reduction still has to find one.

// observePrereq is the SEAM the ready gate calls to gather fresh evidence for one declared
// prerequisite. It is a package var so tests can inject precise offline observations
// (the same override pattern forgeAPIBase uses); production reads the forge.
var observePrereq = productionObservePrereq

// externalPrereqOutcome is clearedByExternalPrereq's result: whether the CR declared the
// exemption at all (declared) and, if so, the full decision.
type externalPrereqOutcome struct {
	declared bool
	decision deskkit.ExternalPrereqDecision
}

// clearedByExternalPrereq evaluates the same-head external-prerequisite exemption for the
// standing CHANGES_REQUESTED at head. It returns declared=false when no reviewer CR at
// head carries the External-Prereq-Only declaration (the caller then keeps its ordinary
// refusal, unchanged). When declared, decision.Cleared says whether the block clears.
//
// securityFail is the caller's already-computed standing security verdict — passed in
// rather than re-derived so the two gates cannot disagree about the same retraction.
func clearedByExternalPrereq(reviews []reviewInfo, head string, securityFail bool) externalPrereqOutcome {
	// The standing CR at head that DECLARES the exemption. Reviews arrive in ascending
	// submitted order, so the last declaring CR at head is the standing one.
	var cr *reviewInfo
	for i := range reviews {
		r := &reviews[i]
		if !isReviewerBot(r.User.Login) || r.CommitID != head || r.State != "CHANGES_REQUESTED" {
			continue
		}
		if deskkit.ExternalPrereqOnlyDeclared(r.Body) {
			cr = r
		}
	}
	if cr == nil {
		return externalPrereqOutcome{declared: false}
	}

	crAt, _ := time.Parse(time.RFC3339, strings.TrimSpace(cr.SubmittedAt))

	// The citing APPROVEs: same reviewer, same head, submitted AFTER the CR, correctness
	// lane only — a security-marked body must never act in the correctness lane, and
	// clearing a correctness block is acting in it.
	var approveBodies []string
	for _, r := range reviews {
		if !isReviewerBot(r.User.Login) || r.CommitID != head || r.State != "APPROVED" {
			continue
		}
		if classifyLane(r.Body) != laneCorrectness {
			continue
		}
		// An unreadable or not-after APPROVE cannot be the clearing one. (A zero crAt makes
		// every real APPROVE "after" it, but the decision separately refuses a zero CR time
		// on its freshness clause, so an unreadable CR time still fails closed there.)
		at, err := time.Parse(time.RFC3339, strings.TrimSpace(r.SubmittedAt))
		if err != nil || !at.After(crAt) {
			continue
		}
		approveBodies = append(approveBodies, r.Body)
	}

	// Fresh, independent observation of each declared prerequisite.
	decl := deskkit.ParsePrereqDeclaration(cr.Body)
	obs := make(map[string]deskkit.PrereqObservation, len(decl.Conditions))
	for _, cond := range decl.Conditions {
		o := observePrereq(cond)
		o.ConditionID = cond.ID
		obs[cond.ID] = o
	}

	in := deskkit.ExternalPrereqInput{
		CRBody:             cr.Body,
		CRSubmittedAt:      crAt,
		CRHead:             head,
		ApproveBodies:      approveBodies,
		ApproveHead:        head,
		Observations:       obs,
		SecurityFail:       securityFail,
		OpenContentDefects: hasOpenContentDefect(reviews),
	}
	return externalPrereqOutcome{declared: true, decision: deskkit.EvaluateExternalPrereqExemption(in)}
}

// hasOpenContentDefect derives the review ledger (brief 19's contract) from the reviewer's
// review records and reports whether any blocking code/content finding is still open. This
// is the "mixed findings still block" guard's second half: the CR's own block is checked
// by ParsePrereqDeclaration, and this catches a content blocker raised on a DIFFERENT
// reviewer record. It fails closed — reports an open defect — whenever it cannot read the
// ledger positively, because an unreadable ledger can never certify the content lane clean:
//
//   - a malformed block, or one that breaks the reviewer write gate's rules
//     (deskkit.FindingBlockV1.Validate: an unknown lane word, the key separator in an id or
//     class, ...) — a review the App posted outside deskpost is read by the same rules
//     deskpost would have refused it under;
//   - a record whose lane could not be established (reviewLane returned "") — a lane-less
//     reviewer record would take its block's lane, which is how one body could speak for a
//     lane it is not;
//   - any record the fold itself reports Blind (a block naming another lane, a stale-head
//     resolution, ...) — ambiguity resolves toward blocking.
//
// A could-not-check entry is never retired: the fold reads the PR's whole reviewer history,
// so one entry anywhere in it — a resolution once recorded against a stale head and later
// repeated at the current head, or a historical block a newer write rule would refuse —
// withholds the exemption on that PR from then on, and the PR clears only through the
// ordinary review path. That is the accepted cost of failing closed.
func hasOpenContentDefect(reviews []reviewInfo) bool {
	return contentDefectBy(reviews, reviewLane)
}

// contentDefectBy is hasOpenContentDefect with the lane reader passed in, so the empty-lane
// fail-closed can be exercised even though reviewLane never returns an empty lane today.
func contentDefectBy(reviews []reviewInfo, laneOf func(string) string) bool {
	var recs []deskkit.ForgeRecord
	for i, r := range reviews {
		if !isReviewerBot(r.User.Login) {
			continue
		}
		block, present, err := deskkit.ParseFindingBlock(r.Body)
		if err != nil {
			return true // could-not-read a block that claims to be typed → fail closed
		}
		if !present {
			continue
		}
		if block.Validate(deskkit.RoleReviewer) != nil {
			return true // a block the write gate would refuse → fail closed
		}
		lane := laneOf(r.Body)
		if lane == "" {
			return true // a lane-less record would take its block's lane → fail closed
		}
		recs = append(recs, deskkit.ForgeRecord{
			Seq:   i,
			Kind:  deskkit.RecordReview,
			Role:  deskkit.RoleReviewer,
			Actor: r.User.Login,
			Head:  r.CommitID,
			Lane:  lane,
			Block: block,
		})
	}
	if len(recs) == 0 {
		return false
	}
	l := deskkit.DeriveLedger(recs)
	return len(l.Blind) > 0 || len(l.ContentDefects()) > 0
}

// reviewLane names the ledger lane a review body was written in. The two lanes number their
// findings independently, so the ledger must key them apart. It reads the body through
// classifyLane — the package's one definition of which lane a review body speaks in — and
// adds nothing of its own:
//
//   - laneSecurity → the security lane; laneCorrectness → the correctness lane.
//   - laneBoth (one body claiming both verdicts) → deskkit.LaneAmbiguous, the reserved
//     record lane no block can name. The fold keys such a record's findings under that lane
//     and ignores whatever lane its block states (reporting a stated one as could-not-check,
//     which hasOpenContentDefect treats as blocking), so the body can still raise an open
//     finding but can never resolve either lane's finding — ambiguity resolves toward
//     blocking, as it does for the verdict itself.
//
// It never returns an empty lane: a lane-less reviewer record takes its block's lane (and
// hasOpenContentDefect fails closed on one, should a later edit introduce it).
func reviewLane(body string) string {
	switch classifyLane(body) {
	case laneSecurity:
		return deskkit.LaneSecurity.Name
	case laneBoth:
		return deskkit.LaneAmbiguous
	default:
		return deskkit.LaneCorrectness.Name
	}
}

// prRefPattern matches a referenced-PR object of the form "<owner>/<repo>#<n>", the one
// external-object shape the production observer can read fresh evidence for.
var prRefPattern = regexp.MustCompile(`^([A-Za-z0-9._-]+)/([A-Za-z0-9._-]+)#([0-9]+)$`)

// productionObservePrereq reads fresh evidence for one declared prerequisite. The one
// object kind it can verify against the forge is a referenced PULL REQUEST merged at a
// cited commit: it reads the PR fresh, and reports the merge commit and merge time as the
// satisfying event. Everything else — a decision-event id, a cross-forge object, an object
// it cannot parse or read — is reported Readable=false, i.e. COULD-NOT-CHECK, which the
// decision blocks on. It NEVER contacts a cluster or production endpoint; a forge read of
// a public PR is the only query, and it is bounded to the referenced object.
//
// It deliberately does not attempt revocation detection for a merged PR: GitHub keeps
// merged_at set once a PR merges, so a not-merged read reports NO satisfying event (which
// blocks as "wrong revision"), never a false clearance. A test-injected observer supplies
// the Revoked signal for the prerequisites this reader cannot re-derive.
func productionObservePrereq(cond deskkit.PrereqCondition) deskkit.PrereqObservation {
	obs := deskkit.PrereqObservation{ConditionID: cond.ID, Object: cond.Object}
	m := prRefPattern.FindStringSubmatch(strings.TrimSpace(cond.Object))
	if m == nil {
		return obs // unparseable object → could-not-check
	}
	owner, name := m[1], m[2]
	num, err := strconv.Atoi(m[3])
	if err != nil {
		return obs
	}
	be, err := newPostBackend(owner, name)
	if err != nil {
		return obs
	}
	pr, err := be.getPR(num)
	if err != nil {
		return obs // unreadable source → could-not-check
	}
	obs.Readable = true
	if strings.TrimSpace(pr.MergedAt) == "" || strings.TrimSpace(pr.MergeCommitSHA) == "" {
		return obs // read, but no satisfying merge event yet
	}
	obs.SatisfyingRef = strings.TrimSpace(pr.MergeCommitSHA)
	if t, perr := time.Parse(time.RFC3339, strings.TrimSpace(pr.MergedAt)); perr == nil {
		obs.ObservedAt = t
	}
	return obs
}
