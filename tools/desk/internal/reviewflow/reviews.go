package reviewflow

import "time"

// ReviewFact carries parsed policy facts; identity and marker parsing use deskkit's existing owners before entry.
type ReviewFact struct {
	Author, State, CommitID, SubmittedAt string
	Reviewer, ExternalPrereq             bool
	Security                             SecurityVerdict
}
type SecurityVerdict int

// SecurityPassMarker is the literal line an App security-review verdict must carry for a
// risk-classed PR to be flip-eligible (#216). This constant supplies classifier
// notes; operational marker parsing remains with deskkit.
const SecurityPassMarker = "Security-Review: pass"

const (
	SecurityNone SecurityVerdict = iota
	SecurityPass
	SecurityFail
)

func SameHead(a, b string) bool { return a != "" && b != "" && a == b }

type ReviewState struct {
	Ever         bool
	AtHead       bool
	Blocking     bool
	Approved     bool
	LastSHA      string
	SecurityPass bool
	ApprovedAt   time.Time
	// LastReviewAt is the submitted time of the EFFECTIVE standing verdict at head
	// (approved or blocking) — the baseline detectNonCommitResolution compares label /
	// body-edit timestamps against, so only a change that landed AFTER the last review
	// counts. Zero when there is no verdict at head or its timestamp was unparseable.
	LastReviewAt time.Time
	// SuspectNoOp (#37) is true when the effective verdict AT HEAD reads as
	// blocking specifically because an APPROVED review was SUPPRESSED: it immediately
	// followed a CHANGES_REQUESTED at the SAME commit, with no push in between. See the
	// reduction loop below for the mechanism and deskpost/ready.go's latestAppVerdict for
	// the KEEP IN SYNC twin that gates the actual mutating flip.
	SuspectNoOp bool
	// ExternalPrereqDeclared (brief 21) is true when the standing
	// CHANGES_REQUESTED that made the row SuspectNoOp DECLARES itself external-prereq-only
	// (deskkit.ExternalPrereqOnlyDeclared). Such a same-head APPROVE is NOT a suspected
	// forgery — it is a reviewer clearing a rejection whose only blockers were external
	// prerequisites — so the board must not fold it into SUSPECT-APPROVAL. The board is
	// ADVISORY and does not itself perform the independent prerequisite verification; it
	// surfaces the row as EXTERNAL-PREREQ-REVIEW and leaves the authoritative grant to the
	// ready gate (deskpost/ready.go), so the two surfaces agree on what the row IS.
	ExternalPrereqDeclared bool
	// BodyEditReverified is true when a row ReduceReviews marked SuspectNoOp was LIFTED by
	// the deskboard command's applyBodyEditReverification: every standing CR at head declared the documented body-edit
	// re-verification class and a later correctness APPROVE documents and verifies it against
	// the live PR body (deskkit.EvaluateBodyEditReverification — the SAME decision deskflip's
	// reviewer-approved gate runs). The row then reads approved at head and flows through the
	// ordinary CI / mergeability / security arms; this flag only annotates its note.
	BodyEditReverified bool
}

func ReduceReviews(reviews []ReviewFact, head string) ReviewState {
	var st ReviewState
	var decisive []ReviewFact
	lastSec := map[string]SecurityVerdict{}
	var secAuthors []string
	for _, r := range reviews {
		if !r.Reviewer {
			continue
		}
		// Security marker: reviews arrive in ascending submitted order, so the LAST
		// security verdict at head governs, per author (#216). A pass
		// retracted by a later fail at the same head is NOT green.
		if SameHead(r.CommitID, head) {
			if v := r.Security; v != SecurityNone {
				if _, seen := lastSec[r.Author]; !seen {
					secAuthors = append(secAuthors, r.Author)
				}
				lastSec[r.Author] = v
			}
		}
		if r.State == "APPROVED" || r.State == "CHANGES_REQUESTED" {
			decisive = append(decisive, r)
		}
	}
	st.SecurityPass = len(secAuthors) > 0
	for _, a := range secAuthors {
		if lastSec[a] != SecurityPass {
			st.SecurityPass = false
		}
	}
	if len(decisive) == 0 {
		return st
	}
	st.Ever = true

	// #37 ("Reviewer-App gate is forgeable — approve→flip→merge in 14s"): reduce the
	// decisive sequence the same way deskpost's latestAppVerdict does, rather than just
	// taking the chronologically-last review. "Un-forgeable because a PR author cannot
	// approve its own PR" (methodology/brief-17) only defends the AUTHOR account — GitHub's
	// self-approval block has no opinion on a third-party App re-posting a verdict, and any
	// session that can mint the reviewer App's token can post an APPROVED over a standing
	// CHANGES_REQUESTED at an unchanged head. An approval posted over a standing
	// rejection without an intervening commit must never become FLIP-eligible.
	// "An approval at an unchanged head cannot be a re-verification." Once a commit carries
	// a CHANGES_REQUESTED, only a NEW commit_id (a genuine push) produces a verdict this
	// reduction accepts as APPROVED; a same-commit APPROVED — including a retried one —
	// keeps reading as the standing CHANGES_REQUESTED, and is flagged suspectNoOp so the
	// board can say so LOUDLY instead of folding it into an ordinary blocking row.
	// KEEP IN SYNC with deskpost/ready.go's latestAppVerdict — with ONE DELIBERATE
	// DIVERGENCE, stated here so the sync claim stays honest (review NOTICE on the #37 PR):
	// latestAppVerdict filters its stream to the CORRECTNESS lane (classifyLane), whereas
	// `decisive` above admits any reviewer-bot APPROVED/CHANGES_REQUESTED, security lane
	// included. So a security pass posted at a head that already carries a standing
	// correctness CHANGES_REQUESTED reads as SUSPECT-APPROVAL on the board while the
	// mutating gate reads it as an ordinary blocked row. That divergence errs toward
	// FLAGGING on an advisory surface a human reads, never toward granting a flip, so it is
	// the safe direction — but it IS a divergence, not an exact mirror. Any change that
	// makes the board's reduction the basis of a mutating decision must close it first.
	// NOT a divergence (#3093): both reductions decide on reviews with STALE grants removed
	// — an APPROVED or security pass at head submitted before the head commit's committer
	// date, which the forge re-anchored from an older head. Without that, the new commit_id
	// below reads as a genuine push and a re-anchored APPROVED clears a standing CR. The
	// board's caller (classifyPRFrom) applies withoutStaleGrants; ready's applies its twin.
	var lastCommit string
	var commitBlocked bool
	var effState, effCommit, effSubmittedAt string
	var suspect bool
	var blockingExternal bool // the standing CR declared itself external-prereq-only
	for _, r := range decisive {
		if r.CommitID != lastCommit {
			lastCommit = r.CommitID
			commitBlocked = false
			blockingExternal = false
		}
		switch r.State {
		case "CHANGES_REQUESTED":
			commitBlocked = true
			effState, effCommit, effSubmittedAt, suspect = r.State, r.CommitID, r.SubmittedAt, false
			blockingExternal = r.ExternalPrereq
		case "APPROVED":
			if commitBlocked {
				// No push since the standing CHANGES_REQUESTED at this exact commit —
				// suppress the approval; commitBlocked stays true so a retried forgery
				// at the same commit is refused too.
				effState, effCommit, suspect = "CHANGES_REQUESTED", r.CommitID, true
				continue
			}
			effState, effCommit, effSubmittedAt, suspect = r.State, r.CommitID, r.SubmittedAt, false
			blockingExternal = false
		}
	}

	st.LastSHA = effCommit
	// #400 Q3: "at head" is a comparison of two shas, and it is only a statement about
	// the world when BOTH were actually read. A raw `==` made two ABSENCES compare
	// equal: an unread `headRefOid` plus a review carrying no `commit_id` used to
	// produce atHead=true, approved=true, and — with a green rollup — MERGE-NOW, the most
	// consequential verdict this board emits, off two fields nobody could read. Either
	// side empty is could-not-check, and could-not-check is NOT at head — the row falls
	// back to RE-REVIEW/MERGE-CURR, which is the fail-closed direction. sameHead applies
	// that guard on top of the #37 reduction's effCommit.
	st.AtHead = SameHead(effCommit, head)
	if st.AtHead {
		st.Blocking = effState == "CHANGES_REQUESTED"
		st.Approved = effState == "APPROVED"
		st.SuspectNoOp = suspect
		// A suspected no-op flip whose standing CR declared itself external-prereq-only is a
		// declared re-review, not a forgery — surface it distinctly (see the field doc).
		st.ExternalPrereqDeclared = suspect && blockingExternal
		if t, err := time.Parse(time.RFC3339, effSubmittedAt); err == nil {
			st.LastReviewAt = t
			if st.Approved {
				st.ApprovedAt = t
			}
		}
	}
	return st
}

// BodyEditReverificationNote annotates the canonical owner's resolved exemption;
// it does not evaluate or grant that exemption.
func BodyEditReverificationNote(reverified bool) string {
	if !reverified {
		return ""
	}
	return " — same-head APPROVE accepted under the documented body-edit re-verification class " +
		"(deskflip re-verifies it at the gate)"
}
