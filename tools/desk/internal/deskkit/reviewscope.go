package deskkit

// reviewscope.go — the review scope contract: first-pass inventory + impact-based
// blocking boundary (example-stream/20).
//
// THE PROBLEM. An incremental reviewer search keeps discovering old instances of the same
// false claim after each worker fix. Clause 8 of the review kit sweeps the WHOLE diff (and,
// where cheap, the repository) on RE-review, but never mandates a complete FIRST-pass
// inventory and never bounds which of the hits may HOLD the pull request. So a small change
// acquires unbounded cleanup scope: every newly-noticed sibling sentence becomes a fresh
// blocker in its own round, and a five-line fix spends several review rounds discovering
// prose that was already in the first tree.
//
// THE MODEL, in two halves.
//
//   1. FIRST PASS. The first review of a false-claim class INVENTORIES its related
//      occurrences before the verdict — the changed surface, the brief's required
//      deliverables, and references to the affected entity — and RECORDS the search, its
//      scope, its exclusions and the input revision. An incomplete search is reported
//      incomplete and NEVER certified clean (the three-state instrument rule, applied to
//      discovery): a search that did not look has cleared nothing.
//
//   2. BLOCKING BOUNDARY. Repository search is DISCOVERY, not authority to make every hit a
//      merge blocker. A blocking finding must name a concrete failure and its SCOPE BASIS —
//      one of: changed behaviour, an explicit acceptance obligation, a material PR-body /
//      Verify claim, or a demonstrated safety consequence of THIS change. Unrelated
//      pre-existing prose routes to a linked follow-up. "Untouched" does not automatically
//      mean irrelevant (a required operator-state table is a deliverable even when the
//      worker omitted it from the diff); conversely, sharing a directory or a substring is
//      insufficient scope.
//
// CLASS CONTINUITY. All occurrences of one proposition share ONE claim class. A late /
// missed sibling occurrence retains that class and its existing round count — it is review
// coverage failure, not a fresh class, and it does not reset or re-open the counter. A
// previously non-blocking occurrence cannot become blocking merely because another file was
// edited: a promotion requires CHANGED IMPACT or NEW EVIDENCE, explicitly recorded. This is
// the property that distinguishes genuine changed evidence from a bypass of a standing
// review rejection.
//
// WHAT THIS DELIBERATELY DOES NOT TOUCH. The numeric three-round cap and the independent
// security review are UNCHANGED. This model narrows what counts as a NEW blocker; it does
// not alter the round-cap mechanics (that ledger is a separate concern) or remove any
// verdict lane.
//
// WHERE IT RUNS. The review boundary is enforced by the dispatched REVIEWER reading the kit
// clause and by the review-desk reader — this package is the EXECUTABLE SPECIFICATION of
// that contract: the four bases below are pinned to the kit's machine-checkable block by
// TestReviewScopeKitMatchesModel, and the case corpus (docs) is scored against these rules
// by the independent verifier. Nothing here changes a predicate that admits, authorises or
// writes.

import "strings"

// ScopeBasis is the concrete-failure basis a blocking review finding must name. The zero
// value ("") is "no basis named" — a finding with no basis cannot hold the pull request.
type ScopeBasis string

const (
	// BasisChangedBehaviour — the change alters observable behaviour and the finding is a
	// defect in that changed behaviour.
	BasisChangedBehaviour ScopeBasis = "changed-behaviour"
	// BasisAcceptanceObligation — the finding is an explicit acceptance deliverable this
	// change owes, blocking even when omitted from the diff (a required operator-state table).
	BasisAcceptanceObligation ScopeBasis = "acceptance-obligation"
	// BasisMaterialClaim — the finding contradicts a material PR-body or Verify-table claim
	// of this change.
	BasisMaterialClaim ScopeBasis = "material-claim"
	// BasisSafetyConsequence — the finding is a demonstrated safety consequence of this
	// change, including outside the edited lines.
	BasisSafetyConsequence ScopeBasis = "safety-consequence"
)

// ScopeBasisDoc pairs a basis with the one-line description the kit's machine-checkable
// block carries. TestReviewScopeKitMatchesModel holds the kit block to this list, so a kit
// that names a basis the model does not carry (or omits one it does) fails.
type ScopeBasisDoc struct {
	Basis ScopeBasis
	When  string
}

// ScopeBases enumerates the four bases in canonical order. This is the single source of
// truth the kit block and the case corpus are checked against.
func ScopeBases() []ScopeBasisDoc {
	return []ScopeBasisDoc{
		{BasisChangedBehaviour, "the change alters observable behaviour and the finding is a defect in it"},
		{BasisAcceptanceObligation, "the finding is an explicit acceptance deliverable this change owes, even if omitted from the diff"},
		{BasisMaterialClaim, "the finding contradicts a material PR-body or Verify-table claim of this change"},
		{BasisSafetyConsequence, "the finding is a demonstrated safety consequence of this change, including outside the edited lines"},
	}
}

// validBasis reports whether b is one of the four enumerated bases.
func validBasis(b ScopeBasis) bool {
	for _, d := range ScopeBases() {
		if d.Basis == b {
			return true
		}
	}
	return false
}

// ReviewFinding is one occurrence a reviewer is weighing, reduced to the properties the
// scope boundary judges. It carries no free text the decision depends on: the decision is a
// pure function of these flags, so a case in the corpus and a test express the same input.
type ReviewFinding struct {
	// ClassID groups this occurrence with every other occurrence of the same proposition.
	ClassID string
	// Location is the file:line the occurrence sits at, for the verdict — not judged.
	Location string
	// Basis is the scope basis the reviewer named, or "" when none was named.
	Basis ScopeBasis
	// UnrelatedPreexisting records the reviewer's determination that the occurrence is
	// pre-existing prose unrelated to this change. Meaningful only when no basis is named.
	UnrelatedPreexisting bool
	// OutsideEditedLines is true when the occurrence sits outside the diff's edited lines.
	// It NEVER downgrades a basis-backed finding — "untouched" is not "irrelevant".
	OutsideEditedLines bool
	// PreviouslyNonBlocking records that this occurrence was judged non-blocking in an
	// earlier round of the same review.
	PreviouslyNonBlocking bool
	// ChangedImpact / NewEvidence are the ONLY two things that let a previously non-blocking
	// occurrence be promoted to blocking. Another file being edited is neither.
	ChangedImpact bool
	NewEvidence   bool
}

// ScopeDisposition is where the boundary routes a finding.
type ScopeDisposition string

const (
	// DispBlocker — the finding holds the pull request.
	DispBlocker ScopeDisposition = "blocker"
	// DispFollowUp — the finding is real but out of this change's blocking scope; it goes to
	// a linked follow-up rather than holding the pull request.
	DispFollowUp ScopeDisposition = "follow-up"
	// DispCouldNotCheck — the finding names something the boundary cannot certify (an
	// unrecognised basis); it is reported as itself, never rounded to a blocker or a pass.
	DispCouldNotCheck ScopeDisposition = "could-not-check"
)

// ClassifyFinding routes one finding through the impact-based blocking boundary and returns
// a one-line reason. It is a pure function of the finding's properties, with no I/O.
//
// The order is load-bearing:
//   - The no-silent-promotion guard runs FIRST: a previously non-blocking occurrence with
//     neither changed impact nor new evidence is held to follow-up whatever else it names,
//     so a re-edit elsewhere can never quietly promote it.
//   - A finding with no scope basis is a follow-up: discovery is not blocking authority, and
//     co-location (same directory or substring) is not a basis.
//   - A finding naming an unrecognised basis is could-not-check, never a blocker.
//   - A finding naming a valid basis blocks — regardless of whether it sits outside the
//     edited lines, because a required deliverable or a safety consequence is in scope
//     wherever it lives.
func ClassifyFinding(f ReviewFinding) (ScopeDisposition, string) {
	if f.PreviouslyNonBlocking && !f.ChangedImpact && !f.NewEvidence {
		return DispFollowUp, "previously non-blocking occurrence — another file being edited is not changed " +
			"impact or new evidence; no silent promotion"
	}
	if strings.TrimSpace(string(f.Basis)) == "" {
		if f.UnrelatedPreexisting {
			return DispFollowUp, "unrelated pre-existing prose — route to a linked follow-up, not a blocker"
		}
		return DispFollowUp, "no scope basis named — repository search is discovery, and co-location " +
			"(same directory or substring) is insufficient scope"
	}
	if !validBasis(f.Basis) {
		return DispCouldNotCheck, "unrecognised scope basis " + string(f.Basis) + " — cannot certify as a blocker"
	}
	return DispBlocker, "blocker: names a concrete failure with scope basis " + string(f.Basis)
}

// Occurrence is one hit the first-pass inventory recorded.
type Occurrence struct {
	// ClassID is the claim class this hit belongs to.
	ClassID string
	// Location is the file:line of the hit.
	Location string
	// InScope records whether the hit is an in-scope occurrence of the class (as opposed to
	// an incidental substring match the reviewer read and excluded).
	InScope bool
}

// FirstPassInventory is the packet a reviewer records on the first review of a false-claim
// class, before the verdict. Its provenance fields make the search reproducible; Complete
// records whether the search covered its declared scope.
type FirstPassInventory struct {
	// Search is the command or description of the search performed.
	Search string
	// Scope is the surface searched — the changed surface, the brief's required
	// deliverables, and references to the affected entity.
	Scope string
	// Exclusions is what was excluded from the search and why. "none" is a legitimate value,
	// but it must be stated: an unstated exclusion set is not the same as no exclusions.
	Exclusions string
	// Revision is the input revision the search ran against.
	Revision string
	// Complete records whether the search covered the declared scope. A false value is the
	// three-state "could-not-check": the search did not finish its own scope.
	Complete bool
	// Occurrences are every hit the search recorded, in-scope and excluded alike.
	Occurrences []Occurrence
}

// Recorded reports whether the packet records the four required provenance fields. A packet
// missing any of them is not a first-pass inventory, whatever else it contains.
func (inv FirstPassInventory) Recorded() (bool, string) {
	var missing []string
	if strings.TrimSpace(inv.Search) == "" {
		missing = append(missing, "search")
	}
	if strings.TrimSpace(inv.Scope) == "" {
		missing = append(missing, "scope")
	}
	if strings.TrimSpace(inv.Exclusions) == "" {
		missing = append(missing, "exclusions")
	}
	if strings.TrimSpace(inv.Revision) == "" {
		missing = append(missing, "revision")
	}
	if len(missing) > 0 {
		return false, "first-pass packet missing " + strings.Join(missing, ", ")
	}
	return true, "first-pass packet records search, scope, exclusions and revision"
}

// InScopeOccurrences returns every in-scope occurrence the packet named — the set the
// verdict must report together, so naming one and leaving the next round to find another is
// not possible from a complete packet.
func (inv FirstPassInventory) InScopeOccurrences() []Occurrence {
	out := make([]Occurrence, 0, len(inv.Occurrences))
	for _, o := range inv.Occurrences {
		if o.InScope {
			out = append(out, o)
		}
	}
	return out
}

// CertifyClean reports whether the packet may certify the class clean. An incomplete search
// NEVER certifies clean; an unrecorded packet never certifies clean; only a complete,
// recorded search with no in-scope occurrence does.
func (inv FirstPassInventory) CertifyClean() (bool, string) {
	if ok, why := inv.Recorded(); !ok {
		return false, why + " — cannot certify clean"
	}
	if !inv.Complete {
		return false, "search reported incomplete — reported incomplete, never certified clean"
	}
	if n := len(inv.InScopeOccurrences()); n > 0 {
		return false, "complete search found in-scope occurrences — not clean"
	}
	return true, "complete first-pass search recorded, no in-scope occurrence — clean"
}

// ClaimClass is one proposition and the occurrences and round count attached to it.
type ClaimClass struct {
	// ID is the claim class's stable identity.
	ID string
	// RoundCount is the existing number of full verdict-fix-re-review rounds on this class.
	RoundCount int
	// Occurrences are the occurrences grouped under this class.
	Occurrences []Occurrence
}

// AbsorbSibling folds a newly-discovered sibling occurrence of the SAME proposition into
// this class. The class ID and the round count are UNCHANGED, and the sibling is stamped
// with this class's ID: a missed occurrence is review coverage failure, not a fresh class,
// and it does not reset or re-open the round counter. The receiver is returned by value, so
// the caller's original class is not mutated.
func (c ClaimClass) AbsorbSibling(occ Occurrence) ClaimClass {
	occ.ClassID = c.ID
	out := c
	out.Occurrences = append(append([]Occurrence(nil), c.Occurrences...), occ)
	return out
}
