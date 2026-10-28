package contexthandoff

import (
	"fmt"
	"strings"
)

// Check refusal codes. They sit beside the statusgen/19 rule codes (an
// unbound obligation reuses RuleAmendUnbound) and name why a candidate
// amendment cannot land.
const (
	CheckPatchApply        = "patch-apply"        // patch bytes missing, base revision mismatch, or the patch does not apply exactly
	CheckHistoryRewrite    = "history-rewrite"    // the patch removes or rewrites populated Evidence or a dated source account
	CheckSilentRebind      = "silent-rebind"      // an active claim on the brief is neither acknowledged-adjusted nor cancelled
	CheckIncompatibleDelta = "incompatible-delta" // an approved rival changes the same aspect of the same brief differently
	CheckStaleReceipt      = "stale-receipt"      // the receipt no longer matches the snapshot: reconcile first
)

// Verdict outcomes.
const (
	OutcomePass   = "pass"
	OutcomeRefuse = "refuse"
	OutcomeHold   = "hold"
)

// Verdict is the check result for one amendment obligation.
type Verdict struct {
	Obligation string `json:"obligation"`
	Subject    string `json:"subject"`
	Outcome    string `json:"outcome"`
	Rule       string `json:"rule,omitempty"`
	Reason     string `json:"reason"`
}

// Report is a check run: whether the receipt is current against the
// snapshot, what the recheck found, and a verdict per obligation. Current is
// true only when the recheck neither reconciles nor holds: partial coverage
// is never confirmed current. A hold leaves Current false without making the
// receipt stale, so the outcome stays a hold, not a stale-receipt refusal.
type Report struct {
	Receipt  string    `json:"receipt"` // the checked handoff's canonical digest
	Current  bool      `json:"current"` // no reconcile and no hold: confirmed current
	Findings []Finding `json:"findings"`
	Verdicts []Verdict `json:"verdicts"`
	Outcome  string    `json:"outcome"`
}

// Check checks a handoff's amendment obligations against a snapshot. The
// manifest, when given, must bind to the handoff (CheckManifest) and supplies
// each obligation's binding; without one, an obligation's candidate patch is
// its binding. patches maps a patch digest to the patch bytes.
//
// For each obligation it refuses a patch that is unbound or does not apply
// exactly to its base revision, a patch that removes or rewrites protected
// history (populated Evidence, a dated source account), a change under an
// active claim that has no acknowledged adjustment or cancellation, a patch to
// a delivered brief (the obligation routes to an unfinished owner or a linked
// follow-on instead), and a delta an approved rival contradicts. An
// unapproved rival never blocks. An obligation whose brief is in an
// unavailable source is a hold, not a pass.
func Check(h *Handoff, m *Manifest, patches map[string][]byte, s *Snapshot) (*Report, error) {
	rc, err := Recheck(h, s)
	if err != nil {
		return nil, err
	}
	ix, err := newIndex(s)
	if err != nil {
		return nil, err
	}
	rep := &Report{Receipt: h.Digest(), Current: !rc.Reconcile() && !rc.Hold(), Findings: rc.Findings, Verdicts: []Verdict{}}
	if rep.Findings == nil {
		rep.Findings = []Finding{}
	}
	var manifestErr error
	entries := map[string]Entry{}
	if m != nil {
		manifestErr = CheckManifest(h, m)
		for _, e := range m.Entries {
			entries[e.Obligation] = e
		}
	}
	stale := rc.Reconcile()
	hd := h.Digest()
	for _, a := range h.Amendments {
		v := Verdict{Obligation: a.ID, Subject: a.Subject, Outcome: OutcomePass, Reason: "binds, applies and preserves history"}
		switch {
		case stale:
			v.Outcome, v.Rule, v.Reason = OutcomeRefuse, CheckStaleReceipt, "the receipt no longer matches the snapshot; recheck and reconcile before checking amendments"
		case manifestErr != nil:
			v.Outcome, v.Rule, v.Reason = OutcomeRefuse, RuleOf(manifestErr), manifestErr.Error()
		default:
			b := Binding{Patch: a.Candidate}
			if m != nil {
				b = entries[a.ID].Binding
			}
			v = ix.checkOne(v, a, b, h, hd, patches)
		}
		rep.Verdicts = append(rep.Verdicts, v)
	}
	rep.Outcome = OutcomePass
	for _, v := range rep.Verdicts {
		if v.Outcome == OutcomeRefuse {
			rep.Outcome = OutcomeRefuse
		} else if v.Outcome == OutcomeHold && rep.Outcome == OutcomePass {
			rep.Outcome = OutcomeHold
		}
	}
	if stale {
		rep.Outcome = OutcomeRefuse
	}
	if rep.Outcome == OutcomePass && rc.Hold() {
		rep.Outcome = OutcomeHold
	}
	return rep, nil
}

func refuseV(v Verdict, rule, format string, a ...any) Verdict {
	v.Outcome, v.Rule, v.Reason = OutcomeRefuse, rule, fmt.Sprintf(format, a...)
	return v
}

// canon resolves a possibly elided brief identity against a declaring brief.
func canon(id, declaredBy string) string {
	decl, err := ResolveQualifiedID(declaredBy, nil)
	if err != nil {
		return id
	}
	q, err := ResolveQualifiedID(id, &decl)
	if err != nil {
		return id
	}
	return q.String()
}

func (ix *index) checkOne(v Verdict, a Amendment, bind Binding, h *Handoff, hd string, patches map[string][]byte) Verdict {
	a.Subject = canon(a.Subject, h.DeclaredBy)
	if ix.unavailable(a.Subject) {
		v.Outcome, v.Reason = OutcomeHold, "the brief's source is unavailable: could-not-check"
		return v
	}
	if a.Candidate != nil && bind.Patch != nil && *a.Candidate != *bind.Patch {
		return refuseV(v, RuleManifestMismatch, "the obligation's candidate patch and its manifest binding name different patches")
	}
	if r := ix.rivalConflict(a, hd); r != "" {
		return refuseV(v, CheckIncompatibleDelta, "%s", r)
	}
	if why := checkBinding(bind); why != "" {
		return refuseV(v, RuleAmendUnbound, "%s", why)
	}
	if bind.FollowUp != nil {
		f := ix.briefs[*bind.FollowUp]
		switch {
		case f == nil && ix.unavailable(*bind.FollowUp):
			v.Outcome, v.Reason = OutcomeHold, "the follow-up brief's source is unavailable: could-not-check"
		case f == nil:
			return refuseV(v, RuleAmendUnbound, "follow-up %s is not a brief in the snapshot", *bind.FollowUp)
		case !f.unfinished():
			return refuseV(v, RuleAmendUnbound, "follow-up %s is %s, not an unfinished brief that can take the obligation", f.ID, f.Lifecycle)
		default:
			how := ix.linked(a.Subject, f)
			if how == "" {
				return refuseV(v, RuleAmendUnbound, "follow-up %s is not linked to %s: it neither depends on it nor is unblocked by it, and owns no term; bind a linked follow-on or an unfinished owner", f.ID, a.Subject)
			}
			v.Reason = "routed to follow-up " + f.ID + " (" + how + "); " + a.Subject + " is left unchanged"
		}
		return v
	}
	b := ix.briefs[a.Subject]
	if b == nil {
		return refuseV(v, CheckPatchApply, "%s is not a brief in the snapshot, so no patch can apply to it", a.Subject)
	}
	if b.delivered() {
		to, how := ix.route(b)
		if to == "" {
			return refuseV(v, RuleAmendUnbound, "%s is delivered with populated Evidence; bind the obligation to a new follow-on brief instead of patching it", b.ID)
		}
		return refuseV(v, RuleAmendUnbound, "%s is delivered with populated Evidence; bind the obligation to %s (%s) instead of patching it", b.ID, to, how)
	}
	raw, ok := patches[bind.Patch.Digest]
	if !ok {
		return refuseV(v, CheckPatchApply, "no patch with digest %s was supplied", bind.Patch.Digest)
	}
	if !strings.HasPrefix(BlobRevision(b.Raw), bind.Patch.BaseRevision) {
		return refuseV(v, CheckPatchApply, "the patch is bound to base %s, but %s is now at %s", bind.Patch.BaseRevision, b.ID, BlobRevision(b.Raw))
	}
	fp, err := parsePatch(raw)
	if err != nil {
		return refuseV(v, CheckPatchApply, "%v", err)
	}
	if fp.newPath != b.Path {
		return refuseV(v, CheckPatchApply, "the patch changes %s, not %s", escapePath(fp.newPath), escapePath(b.Path))
	}
	res, err := fp.apply(b.Raw)
	if err != nil {
		return refuseV(v, CheckPatchApply, "%v", err)
	}
	if r := rewrites(b, res); r != "" {
		return refuseV(v, CheckHistoryRewrite, "%s", r)
	}
	if r := ix.silentRebind(a.Subject, h); r != "" {
		return refuseV(v, CheckSilentRebind, "%s", r)
	}
	return v
}

// linked names how a follow-up brief is tied to the subject whose obligation
// it takes: a linked follow-on (it depends on the subject, or the subject
// unblocks it) or a declared owner. These are the routes ix.route offers; ""
// means the follow-up is not linked and cannot take the obligation.
func (ix *index) linked(subject string, f *brief) string {
	if contains(f.Depends, subject) {
		return "linked follow-on"
	}
	if b := ix.briefs[subject]; b != nil && contains(b.Unblocks, f.ID) {
		return "linked follow-on"
	}
	for _, e := range ix.s.Edges {
		if e.Edge == "owns" && e.Subject == f.ID {
			return "unfinished owner"
		}
	}
	return ""
}

// rewrites names the first protected region a patch removes a line from or
// inserts into. Adding before or after a region is not a rewrite.
func rewrites(b *brief, res *applied) string {
	for i, r := range protectedRegions(b.Raw) {
		for _, d := range res.deleted {
			if d >= r.start && d < r.end {
				return fmt.Sprintf("the patch removes or rewrites line %d of protected %s history (region %d); history is corrected by a new record, never edited", d+1, r.kind, i+1)
			}
		}
		for _, at := range res.inserted {
			if at > r.start && at < r.end {
				return fmt.Sprintf("the patch inserts inside protected %s history (region %d) before line %d; append after it instead", r.kind, i+1, at+1)
			}
		}
	}
	return ""
}

// silentRebind names an active claim on the subject that the handoff neither
// adjusts with the owner's acknowledgement nor cancels.
func (ix *index) silentRebind(subject string, h *Handoff) string {
	for _, c := range ix.s.Claims {
		if c.Subject != subject {
			continue
		}
		ok := false
		for _, w := range h.ActiveWork {
			if canon(w.Subject, h.DeclaredBy) == c.Subject && w.Claim == c.Claim &&
				((w.Treatment == "adjust" && w.Acknowledged) || w.Treatment == "cancel") {
				ok = true
			}
		}
		if !ok {
			return fmt.Sprintf("claim %s by %s is active at brief revision %s; record an acknowledged adjustment or a cancellation before changing the brief under it", c.Claim, c.Owner, c.BriefRevision)
		}
	}
	return ""
}

// rivalConflict names an approved rival proposal that changes the same aspect
// of the same brief to something else. Two such deltas can merge cleanly as
// text and still contradict each other; only an approved rival counts.
func (ix *index) rivalConflict(a Amendment, self string) string {
	for _, p := range ix.s.Proposals {
		r := p.Handoff
		if r.Target.State != TargetApproved || r.Digest() == self {
			continue
		}
		for _, ra := range r.Amendments {
			if canon(ra.Subject, r.DeclaredBy) != a.Subject {
				continue
			}
			for _, c := range a.Changes {
				for _, rc := range ra.Changes {
					if c.Aspect == rc.Aspect && c.After != rc.After {
						return fmt.Sprintf("approved proposal %s changes the %s of %s differently (obligation %s); approved deltas must be reconciled, not merged", escapePath(p.Locator), c.Aspect, a.Subject, ra.ID)
					}
				}
			}
		}
	}
	return ""
}
