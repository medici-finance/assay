package deskkit

// prstate.go — the shared PR-state snapshot (#268).
//
// #268 names the structural cause behind #247 (merged vs closed load-bearing) and #236
// (stale:false when unassessable): PR-state reduction belongs in ONE deskkit snapshot,
// not re-derived (and re-drifted) in every deskboard verb. board.go already carried a
// command-local reducer (reduceReviews, with the `blocking = last.State ==
// "CHANGES_REQUESTED"` line #268 quotes).
//
// This package is the CANONICAL HOME and stalled is its FIRST CONSUMER —
// which is a smaller claim than "the reducers are consolidated", and the smaller claim is
// the true one. board.go's reduceReviews is untouched and still live: it carries
// security-marker and approved-age concerns this reduction deliberately does not, and
// migrating it is a separate change against #268. Until then TWO reducers exist, and
// #268's whole premise is that parallel reducers drift — so the migration is outstanding
// work, not a finished job. New consumers must build on this one rather than add a third.
//
// The shape is exactly what #268 specifies: head SHA, draft, open/merged/closed, the
// reduced reviewer-App verdict, and the CI verdict. Consumers read what they need; a
// field left unset reads as its zero value, and every zero value is the "unknown" kind
// on purpose — "not stated" must never read as a clean state (the #247/#236 fail-open
// shape this snapshot exists to end).

import "strings"

// PRState is the shared reduction of one PR's GitHub state. A board verb builds it from
// its raw gh reads and consults the fields its loop needs; nothing re-walks the raw
// reviews/checks to re-derive them.
type PRState struct {
	HeadSHA    string     // the PR's current head ref oid
	Draft      bool       // isDraft
	State      PRKind     // Open / Merged / Closed
	AppVerdict AppVerdict // reduced reviewer-App decisive verdict AT HEAD
	CIVerdict  CIVerdict  // reduced status-check rollup
}

// PRKind is a PR's open/merged/closed state. The zero value is PRUnknown: "not stated"
// must never read as "open" — that is the #247 shape (a merged PR read as open).
type PRKind string

const (
	PRUnknown PRKind = "unknown"
	PROpen    PRKind = "open"
	PRMerged  PRKind = "merged"
	PRClosed  PRKind = "closed"
)

// ParsePRKind reduces GitHub's PR state string ("OPEN" / "CLOSED" / "MERGED", in any
// case) to a PRKind. Anything else — including the empty string — is PRUnknown, NEVER a
// guessed PROpen: #247 is the bug where merged-vs-closed stopped being load-bearing, and
// the only safe reading of a state nobody stated is "not stated". Callers that compare a
// PR head against a branch must branch on PRUnknown explicitly (label it), not fall
// through to the open path.
func ParsePRKind(s string) PRKind {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "OPEN":
		return PROpen
	case "MERGED":
		return PRMerged
	case "CLOSED":
		return PRClosed
	default:
		return PRUnknown
	}
}

// AppVerdict is the reduced reviewer-App verdict AT THE CURRENT HEAD — the one fact
// every board consumer of App reviews needs: did the reviewer approve, request changes,
// or neither, at the code the PR is on right now. The reduction mirrors board.go's
// reduceReviews "at head" semantics for the decisive-verb path; board.go's extra
// concerns (security markers, approved-age decay for the FLIP gate) are board-specific
// and stay there until #268 migrates the whole reducer.
type AppVerdict string

const (
	// AppVerdictNone — the App has posted no APPROVED/CHANGES_REQUESTED review.
	AppVerdictNone AppVerdict = "none"
	// AppVerdictChangesRequested — the App's latest decisive review AT HEAD requested
	// changes. This is the "blocking" signal stalled hunts for.
	AppVerdictChangesRequested AppVerdict = "changes_requested"
	// AppVerdictApproved — the App's latest decisive review AT HEAD approved.
	AppVerdictApproved AppVerdict = "approved"
	// AppVerdictStale — the App has a decisive review, but it landed at a DIFFERENT head
	// (the PR advanced past it). Distinct from None: a verdict exists, just not at this
	// code, so "is there a review?" and "is there a review AT HEAD?" stay separable.
	AppVerdictStale AppVerdict = "stale"
)

// IsBlockingAtHead reports whether the verdict is CHANGES_REQUESTED at the current head
// — the stalled signal. Convenience for consumers that do not switch on the kind.
func (v AppVerdict) IsBlockingAtHead() bool { return v == AppVerdictChangesRequested }

// CIVerdict reduces a PR's status-check rollup to one verdict.
type CIVerdict string

const (
	CIVerdictUnknown CIVerdict = "unknown"
	CIVerdictPass    CIVerdict = "pass"
	CIVerdictPending CIVerdict = "pending"
	CIVerdictFail    CIVerdict = "fail"
)

// ---------------------------------------------------------------------------
// actor identity — the ONE place two renderings of one GitHub App are folded
// ---------------------------------------------------------------------------

// GitHub renders a single App identity two different ways, and which one you get is a
// property of the ENDPOINT, not of the App:
//
//	gh pr list --json author          →  "app/<slug>"
//	REST (comments, reviews, commits) →  "<slug>[bot]"
//
// A consumer that reads an author from the gh CLI and then compares it against a REST
// payload's login is comparing two strings that can never be equal, so the comparison
// silently answers "different actor" for EVERY App — and the watched set is largely
// App-authored. Every deskkit comparison of two GitHub logins goes through SameActor so
// that fold exists once, not once per caller (the #268 rule applied to identity).
//
// Neither fold can capture a human: a GitHub login may contain neither "/" nor "[", so
// only an App-rendered string can carry either affix.

// NormalizeActorLogin folds a GitHub login to a comparable (name, isApp) pair. The
// isApp flag is carried rather than discarded: a human may legitimately be named
// `foo` while an App is named `app/foo` / `foo[bot]`, and collapsing both to "foo"
// would let a stranger's comment read as the App author's.
func NormalizeActorLogin(login string) (name string, isApp bool) {
	s := strings.ToLower(strings.TrimSpace(login))
	if rest, ok := strings.CutPrefix(s, "app/"); ok {
		return rest, true
	}
	if rest, ok := strings.CutSuffix(s, "[bot]"); ok {
		return rest, true
	}
	return s, false
}

// SameActor reports whether two GitHub login strings name the same identity, across the
// gh-CLI and REST renderings of an App. Two EMPTY logins are never the same actor:
// "GitHub stated no login" is missing data, and matching it against another absence
// would resolve an unknown into a claimed identity (the #236 shape).
func SameActor(a, b string) bool {
	an, aApp := NormalizeActorLogin(a)
	bn, bApp := NormalizeActorLogin(b)
	return an != "" && an == bn && aApp == bApp
}

// AppReview is the deskkit-level review a caller reduces to an AppVerdict. It is the
// subset of a GitHub review the reduction needs, so deskkit does not import any
// command-local review struct and the reducer stays pure — testable with no gh.
type AppReview struct {
	AuthorLogin string
	State       string // APPROVED / CHANGES_REQUESTED / COMMENTED / ...
	CommitID    string
	SubmittedAt string // RFC3339
}

// CICheck is the deskkit-level check a caller reduces to a CIVerdict.
type CICheck struct {
	Status     string // QUEUED / IN_PROGRESS / COMPLETED
	Conclusion string // SUCCESS / FAILURE / SKIPPED / NEUTRAL / ...
}

// ReduceAppVerdict reduces the reviewer App's reviews to the decisive verdict AT HEAD.
// reviews are assumed chronological (GitHub REST order). Only the App identified by
// appBotLogin counts — the desk's reviewer App is the unforgeable voice; human reviews
// do not govern the loop. The identity test is SameActor, not string equality, so a
// caller may name the App in either rendering. Semantics, matching board.go's
// reduceReviews at-head path:
//
//   - no decisive (APPROVED/CHANGES_REQUESTED) App review → None
//   - latest decisive review's CommitID != head → Stale (a verdict exists, not at head)
//   - latest decisive at head is CHANGES_REQUESTED → ChangesRequested
//   - latest decisive at head is APPROVED → Approved
//
// An empty head ("") forces Stale for any decisive review: a verdict's "at head" status
// is unverifiable when the head itself is unknown, and "I don't know the head" must not
// collapse to "the verdict is current".
func ReduceAppVerdict(appBotLogin string, reviews []AppReview, head string) AppVerdict {
	var decisive []AppReview
	for _, r := range reviews {
		if !SameActor(r.AuthorLogin, appBotLogin) {
			continue
		}
		if r.State == "APPROVED" || r.State == "CHANGES_REQUESTED" {
			decisive = append(decisive, r)
		}
	}
	if len(decisive) == 0 {
		return AppVerdictNone
	}
	last := decisive[len(decisive)-1]
	if head == "" || last.CommitID != head {
		return AppVerdictStale
	}
	switch last.State {
	case "CHANGES_REQUESTED":
		return AppVerdictChangesRequested
	case "APPROVED":
		return AppVerdictApproved
	}
	return AppVerdictNone
}

// LastAppDecisiveReview returns the latest decisive App review (chronological) and
// whether one exists. Callers that need its timestamp — stalled reports "last review
// date" — read it here rather than re-walking the list (the ONE reduction rule).
func LastAppDecisiveReview(appBotLogin string, reviews []AppReview) (AppReview, bool) {
	var last AppReview
	found := false
	for _, r := range reviews {
		if !SameActor(r.AuthorLogin, appBotLogin) {
			continue
		}
		if r.State == "APPROVED" || r.State == "CHANGES_REQUESTED" {
			last = r
			found = true
		}
	}
	return last, found
}

// ReduceCIVerdict reduces a status-check rollup to one verdict. SKIPPED/NEUTRAL
// conclusions are ignored (path-filtered / neutral), matching board.go's ciState.
// ciRequired mirrors CIRequired: when false, an empty rollup is vacuously green (the
// repo has no CI to wait for) rather than unknown — an empty rollup on a CI-LESS repo
// is the full answer, not a missing one. On a CI-REQUIRED repo an empty rollup is
// Unknown: no checks have reported, and "no checks" must never read as "green".
func ReduceCIVerdict(checks []CICheck, ciRequired bool) CIVerdict {
	var pass, pending, fail int
	for _, c := range checks {
		switch {
		case c.Status != "COMPLETED":
			pending++
		case c.Conclusion == "SUCCESS":
			pass++
		case c.Conclusion == "SKIPPED" || c.Conclusion == "NEUTRAL":
			// ignore path-filtered / neutral
		default: // FAILURE, CANCELLED, TIMED_OUT, ACTION_REQUIRED, STARTUP_FAILURE
			fail++
		}
	}
	switch {
	case fail > 0:
		return CIVerdictFail
	case pending > 0:
		return CIVerdictPending
	case pass > 0:
		return CIVerdictPass
	case !ciRequired:
		return CIVerdictPass // vacuously green — there are no checks to fail
	default:
		return CIVerdictUnknown // CI-required repo, no checks reported yet
	}
}
