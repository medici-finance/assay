package deskkit

// externalprerequisite.go is the CANONICAL reader and decision for the same-head
// EXTERNAL-PREREQUISITE exemption: the narrow rule under which a standing
// CHANGES_REQUESTED whose ONLY blockers are explicitly recorded external prerequisites
// may clear at an UNCHANGED head, with no source commit, once every named prerequisite
// is independently verified as changed AFTER the rejection.
//
// It is the sibling of checkonlycr.go. Both answer the same shape of question — "may an
// APPROVED over a standing CHANGES_REQUESTED at an unchanged head be honoured, although
// nothing in the diff moved?" — and both answer it only from an EXPLICIT, TYPED
// declaration, never from prose. checkonlycr.go handles the case where the sole blocker
// was a required CHECK that turned green; this file handles the case where the sole
// blockers were EXTERNAL PREREQUISITES (an upstream PR that later merged, a recorded
// decision that later answered the question). The two exemptions are deliberately
// parallel so a reader who understands one understands the other, and so neither can be
// widened without the other's discipline being visible next to it.
//
// WHY A TYPED DECLARATION AND NOT PROSE. The exemption was authorised on the condition
// that "external-prerequisite-only" is established by the review's EXPLICIT SHAPE, never
// by heuristics over English. A reader that tried to infer "this CR names no code finding"
// from prose would be guessing, and guessing on a GRANT path is how a laundering hole
// opens: a CR carrying a real code finding and the words "waiting on upstream" would read
// as external-prereq-only. So the reviewer DECLARES it, on a fixed marker line AND in the
// typed finding block from brief 19 (reviewfinding.go), whose blocking findings must every
// one be BlockerExternalPrereq. A code/content blocking finding in the same block is
// "mixed", and mixed fails closed.
//
// FAIL-CLOSED IN EVERY DIRECTION. The decision (EvaluateExternalPrereqExemption) refuses
// unless every clause is positively established from FRESH, INDEPENDENT evidence supplied
// by the caller at the ready boundary — never from the citation's own say-so. A missing
// observation, an unreadable one, a wrong revision, a prerequisite that predates the
// rejection, a later revocation, a standing security failure, or any code/content finding
// all continue to block. "Could-not-check" is never rounded up to a clearance.
//
// THE GRANT MARKERS SKIP FENCED CODE, exactly as checkonlycr.go's do and for the same
// reason: a marker quoted inside a ``` fence is documentation of the format (this file's
// own doc comment, a skill, a review explaining the shape), and documentation is never a
// grant. This is the whole reason the readers below pass SkipFenced.

import (
	"regexp"
	"strings"
	"time"
)

// externalPrereqOnly matches the CR-side declaration: the reviewer states that the ONLY
// blockers in this CHANGES_REQUESTED are the external prerequisites enumerated in the
// review's typed finding block.
//
//	External-Prereq-Only: waiting on assay#1374 to merge
//
// The value is a human summary; its PRESENCE is the declaration, and the authoritative
// enumeration of conditions is the finding block (each a BlockerExternalPrereq blocking
// finding). The value may not be empty — a declaration that says nothing declares nothing.
var externalPrereqOnly = regexp.MustCompile(`(?i)^[ \t]*External-Prereq-Only:[ \t]*(\S.*?)[ \t\r]*$`)

// clearedPrereq matches the APPROVE-side citation: the reviewer names one satisfied
// prerequisite and the evidence that satisfied it.
//
//	Cleared-Prereq: epq-1 medici-finance/assay#1374 7071466f1dd952b9580edc5e98dbf0251fbba9b3
//
// The value is three whitespace-separated fields: the condition id (matching a finding id
// declared on the CR), the external OBJECT the prerequisite is about, and the SATISFYING
// REF the reviewer observed (a merge commit, a decision-event id). All three are required;
// a citation missing any of them cites nothing this reader can bind to fresh evidence.
var clearedPrereq = regexp.MustCompile(`(?i)^[ \t]*Cleared-Prereq:[ \t]*(\S.*?)[ \t\r]*$`)

// ExternalPrereqOnlyDeclared reports whether a CHANGES_REQUESTED body carries a
// well-formed External-Prereq-Only declaration line. It is the shared CLASSIFIER — the
// board (deskboard) and the review planner (reviewloop) call it to recognise the declared
// state without themselves performing the independent verification that only the ready
// gate does, so all three surfaces AGREE on what a declared external-prereq re-review is
// and none mislabels it as a suspected forgery.
//
// It answers only "did the reviewer DECLARE the exemption", never "is the exemption
// GRANTED" — the grant is EvaluateExternalPrereqExemption's, and it needs fresh evidence
// the classifier deliberately does not touch.
func ExternalPrereqOnlyDeclared(body string) bool {
	return SoleVerdictMarkerValue(body, externalPrereqOnly, SkipFenced) != ""
}

// PrereqClearance is one parsed `Cleared-Prereq:` citation from an APPROVE body.
type PrereqClearance struct {
	ConditionID   string
	Object        string
	SatisfyingRef string
	Raw           string // the whole value, for a diagnostic on a malformed citation
}

// ParsePrereqClearances returns every `Cleared-Prereq:` citation an APPROVE body carries,
// under the same fence/emphasis handling the other grant markers use. A citation whose
// value does not split into the three required fields is returned with only Raw set, so
// the decision can name it in the refusal rather than silently drop it.
func ParsePrereqClearances(body string) []PrereqClearance {
	vals := VerdictMarkerValues(body, clearedPrereq, SkipFenced)
	out := make([]PrereqClearance, 0, len(vals))
	for _, v := range vals {
		fields := strings.Fields(v)
		if len(fields) < 3 {
			out = append(out, PrereqClearance{Raw: v})
			continue
		}
		out = append(out, PrereqClearance{
			ConditionID:   fields[0],
			Object:        fields[1],
			SatisfyingRef: fields[2],
			Raw:           v,
		})
	}
	return out
}

// PrereqCondition is one external prerequisite the CR declares as a blocker, read from a
// BlockerExternalPrereq blocking finding in the CR's typed finding block.
type PrereqCondition struct {
	ID     string // the finding id — matched against a Cleared-Prereq citation's condition id
	Object string // the external object (the finding's SharedRepair), e.g. "medici-finance/assay#1374"
	Detail string // the resolution condition, for a diagnostic
}

// PrereqDeclaration is a CR body parsed for the external-prerequisite exemption.
type PrereqDeclaration struct {
	// Declared is true when the External-Prereq-Only marker line is present.
	Declared bool
	// Summary is the declaration line's human value.
	Summary string
	// Conditions is every declared external-prerequisite blocker, from the typed block.
	Conditions []PrereqCondition
	// HasContentBlocker is true when the SAME block carries a blocking code/content
	// finding — a "mixed" CR, which can never be external-prereq-only.
	HasContentBlocker bool
	// Err is non-nil when a finding block is present but malformed/wrong-schema. An
	// unreadable declaration is could-not-check, and could-not-check never clears.
	Err error
}

// ParsePrereqDeclaration reads a CHANGES_REQUESTED body's external-prerequisite
// declaration: the marker line plus the typed finding block that enumerates the
// prerequisites. Legacy free text (a marker with no typed block) yields Declared=true but
// no Conditions, which the decision refuses — the enumeration must be typed.
func ParsePrereqDeclaration(crBody string) PrereqDeclaration {
	d := PrereqDeclaration{}
	d.Summary = SoleVerdictMarkerValue(crBody, externalPrereqOnly, SkipFenced)
	d.Declared = d.Summary != ""

	block, present, err := ParseFindingBlock(crBody)
	if err != nil {
		d.Err = err
		return d
	}
	if !present {
		return d
	}
	for _, f := range block.Findings {
		if f.Severity != SeverityBlocking {
			continue
		}
		switch f.Blocker {
		case BlockerCodeContent:
			d.HasContentBlocker = true
		case BlockerExternalPrereq:
			d.Conditions = append(d.Conditions, PrereqCondition{
				ID:     strings.TrimSpace(f.ID),
				Object: strings.TrimSpace(f.SharedRepair),
				Detail: strings.TrimSpace(f.Resolution),
			})
		}
	}
	return d
}

// PrereqObservation is a FRESH, INDEPENDENT observation of one external prerequisite,
// gathered by the caller AT THE READY BOUNDARY and never trusted from the reviewer's
// citation. It binds evidence to the object, the condition, the satisfying ref and the
// observation time, exactly as the interface contract requires. Readable=false is the
// could-not-check state — a source the reader could not read — and it BLOCKS.
type PrereqObservation struct {
	ConditionID   string
	Object        string    // the object the observation is about
	SatisfyingRef string    // the observed satisfying commit/decision-event
	ObservedAt    time.Time // when the satisfying event occurred (a merge time, a decision time)
	Revoked       bool      // a later revocation (a reopened PR, a retracted decision)
	Readable      bool      // false ⇒ the source data could not be read (could-not-check)
}

// ExternalPrereqInput is everything EvaluateExternalPrereqExemption needs. The caller
// (the ready gate) is responsible for supplying only REVIEWER-authored bodies (the CR and
// the citing APPROVEs), fetching the Observations fresh, and deriving OpenContentDefects
// from the durable finding ledger — the decision reasons over what it is given and never
// reaches for the forge itself.
type ExternalPrereqInput struct {
	CRBody             string
	CRSubmittedAt      time.Time
	CRHead             string
	ApproveBodies      []string // reviewer, same head, submitted AFTER the CR, correctness lane
	ApproveHead        string
	Observations       map[string]PrereqObservation // fresh, keyed by condition id
	SecurityFail       bool                         // a standing Security-Review: fail
	OpenContentDefects bool                         // any open code/content blocker in the ledger
}

// ExternalPrereqDecision is the result. Declared drives the board/planner classification
// (a declared-but-not-yet-granted re-review is not a forgery); Cleared drives the grant.
// Reason is always populated, so a refusal names the clause it failed on.
type ExternalPrereqDecision struct {
	Cleared    bool
	Declared   bool
	Reason     string
	Conditions []string // the condition ids evaluated, in order
}

// EvaluateExternalPrereqExemption decides whether a standing external-prerequisite-only
// CHANGES_REQUESTED clears at the unchanged head. It is PURE: the same inputs always yield
// the same decision, which is the restart- and agent-replacement-survival guarantee —
// re-deriving from the same durable records and the same fresh observations yields the
// same verdict. It authorises nothing on its own; the caller still requires an APPROVED to
// GOVERN at head after the block is cleared (the exemption removes a block, it never
// manufactures a verdict).
//
// Every clause is checked in the fail-closed direction. The order puts the cheap,
// declaration-shape refusals first (they need no evidence) and the per-condition evidence
// checks last.
func EvaluateExternalPrereqExemption(in ExternalPrereqInput) ExternalPrereqDecision {
	dec := ExternalPrereqDecision{}
	d := ParsePrereqDeclaration(in.CRBody)
	dec.Declared = d.Declared

	// (1) No declaration: the ordinary unchanged-head rule stands, with no diagnosis —
	// this is the overwhelmingly common CR with real findings.
	if !d.Declared {
		dec.Reason = "no `External-Prereq-Only:` declaration on the standing CHANGES_REQUESTED — the unchanged-head rule stands"
		return dec
	}
	// (2) A malformed typed block is could-not-check, never a clearance.
	if d.Err != nil {
		dec.Reason = "the CR's typed finding block is unreadable (" + d.Err.Error() +
			") — an unreadable declaration is could-not-check, never a clearance"
		return dec
	}
	// (3) Mixed: any code/content blocker — in this CR or standing in the ledger — blocks.
	if d.HasContentBlocker {
		dec.Reason = "the CR declares External-Prereq-Only but its finding block also carries a blocking " +
			"code/content finding — a mixed rejection is never external-prerequisite-only"
		return dec
	}
	if in.OpenContentDefects {
		dec.Reason = "a blocking code/content finding still stands in the review ledger — mixed findings block; " +
			"only an external-prerequisite-only rejection may clear at an unchanged head"
		return dec
	}
	// (4) A declaration with no typed enumeration cannot qualify.
	if len(d.Conditions) == 0 {
		dec.Reason = "the CR declares External-Prereq-Only but enumerates no external-prerequisite blocker in a " +
			"typed finding block — legacy free text cannot qualify"
		return dec
	}
	// (5) A standing security retraction blocks regardless of external prerequisites.
	if in.SecurityFail {
		dec.Reason = "a standing `Security-Review: fail` is unsatisfied — a security retraction blocks the clearance " +
			"whatever the external prerequisites say"
		return dec
	}
	// (6) The exemption is SAME-HEAD: the CR and the clearing APPROVE must be one unchanged
	// head. An empty head on either side is could-not-check.
	if in.CRHead == "" || in.ApproveHead == "" || in.CRHead != in.ApproveHead {
		dec.Reason = "the same-head exemption requires the CR and the clearing APPROVE at one unchanged head " +
			"(CR head " + short12(in.CRHead) + ", approve head " + short12(in.ApproveHead) + ")"
		return dec
	}

	// Gather the citations from the reviewer's APPROVE bodies.
	cited := map[string]PrereqClearance{}
	for _, b := range in.ApproveBodies {
		for _, c := range ParsePrereqClearances(b) {
			if c.ConditionID != "" {
				cited[c.ConditionID] = c
			}
		}
	}

	// (7) Every declared condition must be cited AND independently observed as changed
	// AFTER the rejection, at the right object and revision, without revocation.
	for _, cond := range d.Conditions {
		dec.Conditions = append(dec.Conditions, cond.ID)

		c, ok := cited[cond.ID]
		if !ok {
			dec.Reason = "condition " + cond.ID + " is declared but no APPROVE at this head cites `Cleared-Prereq: " +
				cond.ID + " …` — an uncited prerequisite is not cleared"
			return dec
		}
		if c.SatisfyingRef == "" || c.Object == "" {
			dec.Reason = "the Cleared-Prereq citation for " + cond.ID + " is malformed (" + c.Raw +
				") — it must name the object and the satisfying ref"
			return dec
		}
		obs, ok := in.Observations[cond.ID]
		if !ok || !obs.Readable {
			dec.Reason = "no fresh independent evidence could be read for " + cond.ID + " (" + cond.Object +
				") — could-not-check never clears a standing rejection"
			return dec
		}
		if !sameObjectRef(cond.Object, obs.Object) || !sameObjectRef(cond.Object, c.Object) {
			dec.Reason = "the evidence for " + cond.ID + " names object " + obs.Object + " / cited " + c.Object +
				", not the declared prerequisite " + cond.Object + " — an unrelated object cannot clear it"
			return dec
		}
		if obs.Revoked {
			dec.Reason = "the prerequisite " + cond.ID + " (" + cond.Object +
				") was later revoked — a revocation re-blocks the rejection"
			return dec
		}
		if obs.SatisfyingRef == "" || obs.SatisfyingRef != c.SatisfyingRef {
			dec.Reason = "the cited satisfying ref for " + cond.ID + " (" + c.SatisfyingRef +
				") does not match the independently observed " + obs.SatisfyingRef + " — wrong revision"
			return dec
		}
		if in.CRSubmittedAt.IsZero() || !obs.ObservedAt.After(in.CRSubmittedAt) {
			dec.Reason = "the prerequisite " + cond.ID + " was satisfied at " + fmtTime(obs.ObservedAt) +
				", not AFTER the rejection at " + fmtTime(in.CRSubmittedAt) +
				" — a fact the reviewer already had when they blocked cannot re-verify"
			return dec
		}
	}

	dec.Cleared = true
	dec.Reason = "every declared external prerequisite independently verified as changed after the rejection, at the unchanged head"
	return dec
}

// sameObjectRef compares two external object references. Owner/repo casing is
// insignificant on the forge, so the compare is case-insensitive after trimming; an empty
// side never matches (could-not-check is not equality).
func sameObjectRef(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(a, b)
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "(no time)"
	}
	return t.UTC().Format(time.RFC3339)
}
