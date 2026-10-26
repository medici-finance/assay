package contexthandoff

import (
	"errors"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// product.go is product target binding (statusgen/23): which approved product
// target revision each impacted brief was selected against, how that binding
// compares with the target that is current now, and the authorized
// continue/adapt/stop dispositions recorded against a mismatch.
//
// The product target is a different input from the architecture target. A
// change to one never moves the other: the architecture baseline
// ($.target) comes only from the snapshot's target, and the product targets
// come only from the snapshot's product section and the briefs' own outcome
// and outcome-revision keys. A product assessment is a projection: it never
// changes a lifecycle, a disposition, the next act or the recheck outcome,
// and a stale product binding is never a veto.

// Product approval states. Only an approved product target can be current; an
// unapproved one is a rival that cannot stale approved work, and an
// unavailable one is an approval this run could not read.
const (
	ProductApproved    = "approved"
	ProductUnapproved  = "unapproved"
	ProductUnavailable = "unavailable"
)

// Product binding states of one brief.
const (
	BindingBound         = "bound"           // outcome and outcome-revision both declared
	BindingUnbound       = "unbound"         // no outcome, or an outcome with no revision (every legacy brief)
	BindingNone          = "none"            // outcome: none, a decision that the brief moves no outcome
	BindingCouldNotCheck = "could-not-check" // the brief, its frontmatter or its outcome could not be read
)

// Product assessment results and coverage. Only "covered" coverage can carry
// an aligned or mismatch result; every gap is "unknown", never aligned.
const (
	ResultAligned       = "aligned"
	ResultMismatch      = "mismatch"
	ResultUnknown       = "unknown"
	ResultNotApplicable = "not-applicable"

	CoverageCovered             = "covered"
	CoverageUnbound             = "unbound"
	CoverageUndiscovered        = "undiscovered"
	CoverageApprovalUnavailable = "approval-unavailable"
	CoverageTargetUnknown       = "target-unknown"
	CoverageCouldNotCheck       = "could-not-check"

	RelationPrimary   = "primary"   // the outcome the brief's own binding names
	RelationSecondary = "secondary" // an outcome the snapshot declares the work relates to
)

// Disposition decisions an authorized role records against a product
// mismatch. None of them changes a lifecycle or cancels work by itself.
const (
	DecisionContinue = "continue"
	DecisionAdapt    = "adapt"
	DecisionStop     = "stop"
)

// Refusals of Dispose. They are not record-shape rules (a decoded record never
// carries them), so they are not in the context-handoff-v1 rule list.
const (
	RuleStaleDisposition = "stale-disposition" // the receipt or the current target moved since the disposition was prepared
	RuleDispositionScope = "disposition-scope" // the disposition names nothing it may decide, or comes from the wrong role
)

// ProductAssessed is the scope line a receipt with a product section carries
// in place of ArchitectureOnly.
const ProductAssessed = "architecture and product-binding coverage: product alignment is a projection over each brief's declared outcome-revision; an unbound or undiscovered binding is unknown, never aligned"

const productLinePrefix = "product: "

var (
	// outcomeRefRe is a fully qualified requirement reference,
	// <cell>:<repo>:REQ-<slug>; the slug grammar is the requirement
	// register's.
	outcomeRefRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*:REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	inRepoReqRe  = regexp.MustCompile(`^REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
	aliasReqRe   = regexp.MustCompile(`^[a-z0-9]+:REQ-[a-z0-9][a-z0-9-]{8,18}[a-z0-9]$`)
)

// Product is the context-handoff-v3 product-binding section.
type Product struct {
	Targets      []ProductTarget      `json:"targets"`
	Bindings     []ProductBinding     `json:"bindings"`
	Assessments  []ProductAssessment  `json:"assessments"`
	Dispositions []ProductDisposition `json:"dispositions"`
}

// ProductTarget is one product target revision the snapshot supplies, in the
// snapshot's order: the last approved revision of an outcome is its current
// target.
type ProductTarget struct {
	Outcome   string `json:"outcome"`
	Revision  string `json:"revision"`
	Approval  string `json:"approval"`
	Reference string `json:"reference"`
}

// ProductBinding is what one brief declares. Selected is the revision the work
// was first selected against; a recheck carries it forward unchanged, so a
// later rebinding never rewrites the original selection.
type ProductBinding struct {
	Subject  string  `json:"subject"`
	State    string  `json:"state"`
	Outcome  *string `json:"outcome"`
	Declared *string `json:"declared"`
	Selected *string `json:"selected"`
	Locator  string  `json:"locator"`
	Reason   string  `json:"reason"`
}

// ProductAssessment is the exact comparison of one binding (or one declared
// secondary relationship) with the current approved target.
type ProductAssessment struct {
	Subject  string  `json:"subject"`
	Outcome  *string `json:"outcome"`
	Relation string  `json:"relation"`
	Bound    *string `json:"bound"`
	Current  *string `json:"current"`
	Result   string  `json:"result"`
	Coverage string  `json:"coverage"`
	Reason   string  `json:"reason"`
}

// ProductDisposition is one authorized decision on a product mismatch. The list
// is append-only: a later disposition supersedes an earlier one for the same
// subject and outcome, and the earlier one stays as history. Basis is the
// digest of the receipt the decision was made on; ExpectedRevision is the
// current target it was made against.
type ProductDisposition struct {
	Subject          string `json:"subject"`
	Outcome          string `json:"outcome"`
	ExpectedRevision string `json:"expected_revision"`
	Decision         string `json:"decision"`
	Rationale        string `json:"rationale"`
	DecidedBy        string `json:"decided_by"`
	Basis            string `json:"basis"`
}

// SnapshotProduct is the optional product input of a snapshot.
type SnapshotProduct struct {
	Targets       []ProductTarget
	Relationships []ProductRelationship
}

// ProductRelationship declares that work relates to an outcome other than (or
// besides) the one its own binding names.
type ProductRelationship struct {
	Subject string
	Outcome string
}

// ---- decoding ---------------------------------------------------------------

func (r *reader) product(v any, p string) *Product {
	m := r.object(v, p, "targets", "bindings", "assessments", "dispositions")
	pr := &Product{}
	items, paths := r.array(m, p, "targets", 0)
	pr.Targets = make([]ProductTarget, 0, len(items))
	for i, it := range items {
		tm := r.object(it, paths[i], "outcome", "revision", "approval", "reference")
		pr.Targets = append(pr.Targets, ProductTarget{
			Outcome:   r.str(tm, paths[i], "outcome", outcomeRefRe, "qualified requirement reference (<cell>:<repo>:REQ-<slug>)"),
			Revision:  r.str(tm, paths[i], "revision", nameRe, "outcome revision name"),
			Approval:  r.enum(tm, paths[i], "approval", ProductApproved, ProductUnapproved, ProductUnavailable),
			Reference: r.str(tm, paths[i], "reference", locatorRe, "locator"),
		})
	}
	items, paths = r.array(m, p, "bindings", 0)
	pr.Bindings = make([]ProductBinding, 0, len(items))
	for i, it := range items {
		bm := r.object(it, paths[i], "subject", "state", "outcome", "declared", "selected", "locator", "reason")
		pr.Bindings = append(pr.Bindings, ProductBinding{
			Subject:  r.qid(bm, paths[i], "subject"),
			State:    r.enum(bm, paths[i], "state", BindingBound, BindingUnbound, BindingNone, BindingCouldNotCheck),
			Outcome:  r.optStr(bm, paths[i], "outcome", outcomeRefRe, "qualified requirement reference (<cell>:<repo>:REQ-<slug>)"),
			Declared: r.optStr(bm, paths[i], "declared", nameRe, "outcome revision name"),
			Selected: r.optStr(bm, paths[i], "selected", nameRe, "outcome revision name"),
			Locator:  r.str(bm, paths[i], "locator", locatorRe, "locator"),
			Reason:   r.str(bm, paths[i], "reason", textRe, "non-blank text"),
		})
	}
	items, paths = r.array(m, p, "assessments", 0)
	pr.Assessments = make([]ProductAssessment, 0, len(items))
	for i, it := range items {
		am := r.object(it, paths[i], "subject", "outcome", "relation", "bound", "current", "result", "coverage", "reason")
		pr.Assessments = append(pr.Assessments, ProductAssessment{
			Subject:  r.qid(am, paths[i], "subject"),
			Outcome:  r.optStr(am, paths[i], "outcome", outcomeRefRe, "qualified requirement reference (<cell>:<repo>:REQ-<slug>)"),
			Relation: r.enum(am, paths[i], "relation", RelationPrimary, RelationSecondary),
			Bound:    r.optStr(am, paths[i], "bound", nameRe, "outcome revision name"),
			Current:  r.optStr(am, paths[i], "current", nameRe, "outcome revision name"),
			Result:   r.enum(am, paths[i], "result", ResultAligned, ResultMismatch, ResultUnknown, ResultNotApplicable),
			Coverage: r.enum(am, paths[i], "coverage", CoverageCovered, CoverageUnbound, CoverageUndiscovered, CoverageApprovalUnavailable, CoverageTargetUnknown, CoverageCouldNotCheck),
			Reason:   r.str(am, paths[i], "reason", textRe, "non-blank text"),
		})
	}
	items, paths = r.array(m, p, "dispositions", 0)
	pr.Dispositions = make([]ProductDisposition, 0, len(items))
	for i, it := range items {
		pr.Dispositions = append(pr.Dispositions, r.productDisposition(it, paths[i]))
	}
	return pr
}

func (r *reader) productDisposition(v any, p string) ProductDisposition {
	dm := r.object(v, p, "subject", "outcome", "expected_revision", "decision", "rationale", "decided_by", "basis")
	return ProductDisposition{
		Subject:          r.qid(dm, p, "subject"),
		Outcome:          r.str(dm, p, "outcome", outcomeRefRe, "qualified requirement reference (<cell>:<repo>:REQ-<slug>)"),
		ExpectedRevision: r.str(dm, p, "expected_revision", nameRe, "outcome revision name"),
		Decision:         r.enum(dm, p, "decision", DecisionContinue, DecisionAdapt, DecisionStop),
		Rationale:        r.str(dm, p, "rationale", textRe, "non-blank text"),
		DecidedBy:        r.str(dm, p, "decided_by", nameRe, "role name"),
		Basis:            r.str(dm, p, "basis", digestRe, "digest (sha256:<64 hex>)"),
	}
}

// checkProduct applies the product section's cross-field rules: unique
// identities, a binding that says "bound" names its outcome and revision, and
// an assessment that says aligned or mismatch is a covered exact comparison.
// An absent binding is never aligned.
func checkProduct(pr *Product) error {
	if pr == nil {
		return nil
	}
	const p = "$.product"
	targets := map[string]bool{}
	for i, t := range pr.Targets {
		k := t.Outcome + "@" + t.Revision
		if targets[k] {
			return refuse(RuleDuplicateID, idx(p+".targets", i), "product target %s is listed twice", k)
		}
		targets[k] = true
	}
	bindings := map[string]ProductBinding{}
	for i, b := range pr.Bindings {
		at := idx(p+".bindings", i)
		if _, dup := bindings[b.Subject]; dup {
			return refuse(RuleDuplicateID, at+".subject", "%s has more than one product binding", b.Subject)
		}
		bindings[b.Subject] = b
		switch b.State {
		case BindingBound:
			if b.Outcome == nil || b.Declared == nil || b.Selected == nil {
				return refuse(RuleCoverage, at, "a bound binding names its outcome, its declared revision and its original selection")
			}
		case BindingNone:
			if b.Outcome != nil || b.Declared != nil {
				return refuse(RuleCoverage, at, "outcome none binds no outcome and no revision")
			}
		case BindingUnbound:
			if b.Declared != nil {
				return refuse(RuleCoverage, at, "an unbound binding declares no revision")
			}
		}
	}
	seen := map[string]bool{}
	for i, a := range pr.Assessments {
		at := idx(p+".assessments", i)
		k := a.Subject + "\x00" + deref(a.Outcome) + "\x00" + a.Relation
		if seen[k] {
			return refuse(RuleDuplicateID, at, "%s is assessed twice for the same outcome and relation", a.Subject)
		}
		seen[k] = true
		switch a.Result {
		case ResultAligned, ResultMismatch:
			if a.Coverage != CoverageCovered || a.Outcome == nil || a.Bound == nil || a.Current == nil {
				return refuse(RuleCoverage, at+".result", "%s is an exact comparison: it needs covered coverage, an outcome, a bound revision and a current revision", a.Result)
			}
			if (*a.Bound == *a.Current) != (a.Result == ResultAligned) {
				return refuse(RuleCoverage, at+".result", "bound %s and current %s do not give %s", *a.Bound, *a.Current, a.Result)
			}
			if b, ok := bindings[a.Subject]; a.Relation == RelationPrimary && (!ok || b.State != BindingBound || !reflect.DeepEqual(b.Outcome, a.Outcome) || !reflect.DeepEqual(b.Declared, a.Bound)) {
				return refuse(RuleCoverage, at+".result", "%s has no bound binding declaring %s: an absent binding is never %s", a.Subject, *a.Bound, a.Result)
			}
		case ResultNotApplicable:
			if b, ok := bindings[a.Subject]; a.Coverage != CoverageCovered || a.Outcome != nil || !ok || b.State != BindingNone {
				return refuse(RuleCoverage, at+".result", "not-applicable is the covered result of a binding of outcome none")
			}
		case ResultUnknown:
			if a.Coverage == CoverageCovered {
				return refuse(RuleCoverage, at+".coverage", "an unknown result states its coverage gap")
			}
		}
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ptr(s string) *string { return &s }

// ---- snapshot ---------------------------------------------------------------

func (sp *SnapshotProduct) validate(decl QID) error {
	seen := map[string]bool{}
	for i, t := range sp.Targets {
		switch {
		case !outcomeRefRe.MatchString(t.Outcome):
			return snapshotErr("product target %d: outcome %q is not <cell>:<repo>:REQ-<slug>", i, t.Outcome)
		case !nameRe.MatchString(t.Revision):
			return snapshotErr("product target %d: revision %q is not a revision name", i, t.Revision)
		case t.Approval != ProductApproved && t.Approval != ProductUnapproved && t.Approval != ProductUnavailable:
			return snapshotErr("product target %d: approval %q is not approved, unapproved or unavailable", i, t.Approval)
		case !locatorRe.MatchString(t.Reference):
			return snapshotErr("product target %d: reference is a non-empty locator", i)
		case seen[t.Outcome+"@"+t.Revision]:
			return snapshotErr("product target %s@%s is listed twice", t.Outcome, t.Revision)
		}
		seen[t.Outcome+"@"+t.Revision] = true
	}
	for i := range sp.Relationships {
		rel := &sp.Relationships[i]
		q, err := ResolveQualifiedID(rel.Subject, &decl)
		if err != nil {
			return snapshotErr("product relationship %d subject: %v", i, err)
		}
		rel.Subject = q.String()
		if !outcomeRefRe.MatchString(rel.Outcome) {
			return snapshotErr("product relationship %d: outcome %q is not <cell>:<repo>:REQ-<slug>", i, rel.Outcome)
		}
	}
	return nil
}

// ---- discovery --------------------------------------------------------------

// currentTarget is the outcome's current approved product target: the last
// approved revision in snapshot order. An unavailable approval after it (or
// with none approved) means the current target cannot be known.
func (ix *index) currentTarget(outcome string) (cur *string, coverage string) {
	unavailable := false
	for _, t := range ix.s.Product.Targets {
		if t.Outcome != outcome {
			continue
		}
		switch t.Approval {
		case ProductApproved:
			cur, unavailable = ptr(t.Revision), false
		case ProductUnavailable:
			unavailable = true
		}
	}
	switch {
	case unavailable:
		return cur, CoverageApprovalUnavailable
	case cur == nil:
		return nil, CoverageTargetUnknown
	}
	return cur, CoverageCovered
}

// binding reads one subject's product binding from its brief.
func (ix *index) binding(subject string) ProductBinding {
	b := ix.briefs[subject]
	if b == nil {
		why := "no brief with this identity is enumerated in the snapshot: could-not-check"
		if ix.unavailable(subject) {
			why = "its source is unavailable: could-not-check"
		}
		return ProductBinding{Subject: subject, State: BindingCouldNotCheck, Locator: subject, Reason: why}
	}
	pb := ProductBinding{Subject: subject, Locator: b.locator("")}
	q, _ := ResolveQualifiedID(subject, nil)
	switch {
	case !b.FrontmatterOK:
		pb.State, pb.Reason = BindingCouldNotCheck, "the frontmatter could not be read: could-not-check"
		return pb
	case b.Outcome == nil:
		pb.State, pb.Reason = BindingUnbound, "no outcome declared: product alignment unchecked, not aligned"
		return pb
	}
	raw, ok := b.Outcome.(string)
	raw = strings.TrimSpace(raw)
	switch {
	case !ok:
		pb.State, pb.Reason = BindingCouldNotCheck, "outcome is not a string: could-not-check"
		return pb
	case raw == "none":
		pb.State, pb.Reason = BindingNone, "outcome none: the brief moves no product outcome"
		return pb
	case inRepoReqRe.MatchString(raw):
		pb.Outcome = ptr(q.Cell + ":" + q.Repo + ":" + raw)
	case outcomeRefRe.MatchString(raw):
		pb.Outcome = ptr(raw)
	case aliasReqRe.MatchString(raw):
		pb.State, pb.Reason = BindingCouldNotCheck, "outcome names another repository's register through an alias the snapshot cannot resolve: could-not-check"
		return pb
	default:
		pb.State, pb.Reason = BindingCouldNotCheck, "outcome is not a requirement reference: could-not-check"
		return pb
	}
	if b.OutcomeRevision == nil {
		pb.State, pb.Reason = BindingUnbound, "outcome declared without an outcome-revision: unbound, not aligned"
		return pb
	}
	rev, ok := b.OutcomeRevision.(string)
	if !ok || !nameRe.MatchString(rev) {
		pb.State, pb.Reason = BindingCouldNotCheck, "outcome-revision is not a revision name: could-not-check"
		return pb
	}
	pb.State, pb.Declared, pb.Selected = BindingBound, ptr(rev), ptr(rev)
	pb.Reason = "declared by the brief's outcome and outcome-revision keys"
	return pb
}

// assess compares one binding with its outcome's current approved target.
func (ix *index) assess(pb ProductBinding) ProductAssessment {
	a := ProductAssessment{Subject: pb.Subject, Outcome: pb.Outcome, Relation: RelationPrimary, Bound: pb.Declared}
	switch pb.State {
	case BindingNone:
		a.Result, a.Coverage, a.Reason = ResultNotApplicable, CoverageCovered, "outcome none"
		return a
	case BindingCouldNotCheck:
		a.Result, a.Coverage, a.Reason = ResultUnknown, CoverageCouldNotCheck, pb.Reason
		return a
	}
	if pb.Outcome != nil {
		a.Current, a.Coverage = ix.currentTarget(*pb.Outcome)
	}
	switch {
	case pb.State == BindingUnbound:
		a.Result, a.Coverage, a.Reason = ResultUnknown, CoverageUnbound, "no product binding: unchecked, not aligned"
	case a.Coverage == CoverageApprovalUnavailable:
		a.Result, a.Reason = ResultUnknown, "a product approval for this outcome is unavailable to this run: could-not-check"
	case a.Coverage == CoverageTargetUnknown:
		a.Result, a.Reason = ResultUnknown, "the snapshot supplies no approved product target for this outcome"
	case *a.Bound == *a.Current:
		a.Result, a.Reason = ResultAligned, "bound to the current approved product target"
	default:
		a.Result, a.Reason = ResultMismatch, "bound to "+*a.Bound+"; the current approved product target is "+*a.Current
	}
	return a
}

// product builds the receipt's product section, or nil when the snapshot
// supplies no product input (the receipt then stays v1/v2, architecture-only).
func (ix *index) product(h *Handoff) *Product {
	if ix.s.Product == nil {
		return nil
	}
	subjects := map[string]bool{ix.decl.String(): true}
	for _, im := range h.Impacts {
		subjects[im.Subject] = true
	}
	for _, rel := range ix.s.Product.Relationships {
		subjects[rel.Subject] = true
	}
	pr := &Product{Targets: append([]ProductTarget{}, ix.s.Product.Targets...), Bindings: []ProductBinding{}, Assessments: []ProductAssessment{}, Dispositions: []ProductDisposition{}}
	bound := map[string]ProductBinding{}
	for _, id := range sortedKeys(subjects) {
		pb := ix.binding(id)
		bound[id] = pb
		pr.Bindings = append(pr.Bindings, pb)
		pr.Assessments = append(pr.Assessments, ix.assess(pb))
	}
	for _, rel := range ix.s.Product.Relationships {
		if o := bound[rel.Subject].Outcome; o != nil && *o == rel.Outcome {
			continue // the primary binding already covers it
		}
		cur, _ := ix.currentTarget(rel.Outcome)
		pr.Assessments = append(pr.Assessments, ProductAssessment{
			Subject: rel.Subject, Outcome: ptr(rel.Outcome), Relation: RelationSecondary, Current: cur,
			Result: ResultUnknown, Coverage: CoverageUndiscovered,
			Reason: "a declared secondary relationship the work's own binding does not name: undiscovered, not aligned",
		})
	}
	sortAssessments(pr.Assessments)
	return pr
}

func sortAssessments(as []ProductAssessment) {
	sort.SliceStable(as, func(i, j int) bool {
		if as[i].Subject != as[j].Subject {
			return as[i].Subject < as[j].Subject
		}
		if as[i].Relation != as[j].Relation {
			return as[i].Relation < as[j].Relation
		}
		return deref(as[i].Outcome) < deref(as[j].Outcome)
	})
}

// effective is the disposition in force for an assessment: the latest one for
// its subject and outcome made against the revision that is current now. A
// disposition made against an older target is history, not in force.
func (pr *Product) effective(a ProductAssessment) *ProductDisposition {
	if a.Outcome == nil || a.Current == nil {
		return nil
	}
	for i := len(pr.Dispositions) - 1; i >= 0; i-- {
		d := pr.Dispositions[i]
		if d.Subject == a.Subject && d.Outcome == *a.Outcome && d.ExpectedRevision == *a.Current {
			return &pr.Dispositions[i]
		}
	}
	return nil
}

// productLines are the readiness prerequisites a product section implies: a
// mismatch with no disposition in force, a recorded decision, and every
// coverage gap. They inform the deciding role; they gate nothing.
func productLines(pr *Product) []string {
	var out []string
	for _, a := range pr.Assessments {
		o := deref(a.Outcome)
		if o == "" {
			o = "(no outcome)"
		}
		switch a.Result {
		case ResultMismatch:
			if d := pr.effective(a); d != nil {
				out = append(out, productLinePrefix+d.Decision+": "+a.Subject+" "+o+" bound "+*a.Bound+", current "+*a.Current+" (decided by "+d.DecidedBy+")")
			} else {
				out = append(out, productLinePrefix+"mismatch: "+a.Subject+" "+o+" bound "+*a.Bound+", current "+*a.Current+": needs a continue, adapt or stop disposition")
			}
		case ResultUnknown:
			out = append(out, productLinePrefix+"unknown: "+a.Subject+" "+o+" ("+a.Relation+"): "+a.Coverage)
		}
	}
	return out
}

// withProductScope replaces the scope line and the product lines of a
// prerequisite list with the ones pr implies.
func withProductScope(pre []string, pr *Product) []string {
	out := []string{ArchitectureOnly}
	if pr != nil {
		out = []string{ProductAssessed}
		out = append(out, productLines(pr)...)
	}
	for _, l := range pre {
		if l == ArchitectureOnly || l == ProductAssessed || strings.HasPrefix(l, productLinePrefix) {
			continue
		}
		out = append(out, l)
	}
	return out
}

// ---- recheck ----------------------------------------------------------------

// FindingAssess is a product-binding change: the deciding role assesses it. It
// never forces reconciliation and never holds, because the architecture
// baseline does not depend on it.
const FindingAssess = "assess"

func (c *comparison) compareProduct() {
	fresh, prior := c.fresh.Product, c.prior.Product
	if fresh == nil {
		return
	}
	if prior == nil {
		prior = &Product{}
	}
	pt, ft := map[string]ProductTarget{}, map[string]ProductTarget{}
	for _, t := range prior.Targets {
		pt[t.Outcome+"@"+t.Revision] = t
	}
	for _, t := range fresh.Targets {
		ft[t.Outcome+"@"+t.Revision] = t
	}
	for _, k := range unionKeys(pt, ft) {
		p, inP := pt[k]
		f, inF := ft[k]
		if inP && inF && p == f {
			continue
		}
		approval := f.Approval
		if !inF {
			approval = p.Approval
		}
		switch approval {
		case ProductUnapproved:
			c.add("product-target:"+k, FindingIgnored, "an unapproved rival product target cannot stale approved work")
		case ProductUnavailable:
			c.add("product-target:"+k, FindingAssess, "the product approval is unavailable to this run: could-not-check; the architecture baseline is unchanged")
		default:
			c.add("product-target:"+k, FindingAssess, "the approved product target changed; the architecture baseline is unchanged: assess each bound work item")
		}
	}
	pa := map[string]ProductAssessment{}
	for _, a := range prior.Assessments {
		pa[a.Subject+"\x00"+deref(a.Outcome)+"\x00"+a.Relation] = a
	}
	for _, a := range fresh.Assessments {
		p, ok := pa[a.Subject+"\x00"+deref(a.Outcome)+"\x00"+a.Relation]
		if ok && p.Result == a.Result && p.Coverage == a.Coverage && reflect.DeepEqual(p.Bound, a.Bound) && reflect.DeepEqual(p.Current, a.Current) {
			continue
		}
		o := deref(a.Outcome)
		if o == "" {
			o = "(no outcome)"
		}
		c.add("product:"+a.Subject+":"+o, FindingAssess, a.Result+" ("+a.Coverage+"): "+a.Reason)
	}
}

// mergeProduct carries the prior original selection and every prior
// disposition onto the fresh product section. The selection is preserved for
// as long as the binding names the same outcome; dispositions are history and
// are never dropped.
func (c *comparison) mergeProduct() *Product {
	fresh := c.fresh.Product
	if fresh == nil {
		return nil
	}
	out := *fresh
	out.Bindings = append([]ProductBinding{}, fresh.Bindings...)
	out.Dispositions = []ProductDisposition{}
	if prior := c.prior.Product; prior != nil {
		pb := map[string]ProductBinding{}
		for _, b := range prior.Bindings {
			pb[b.Subject] = b
		}
		for i, b := range out.Bindings {
			p, ok := pb[b.Subject]
			if ok && b.State == BindingBound && p.Selected != nil && reflect.DeepEqual(p.Outcome, b.Outcome) {
				out.Bindings[i].Selected = p.Selected
			}
		}
		out.Dispositions = append(out.Dispositions, prior.Dispositions...)
	}
	return &out
}

// ---- dispositions -----------------------------------------------------------

// Dispose records one continue/adapt/stop decision on a product binding and
// returns the receipt with it appended. It refuses a decision prepared on a
// different receipt (a concurrent change: Basis is not this receipt's digest),
// one made against a target revision that is no longer current (a stale
// rebinding), one from any role but the receipt's deciding role, and one on an
// assessment with nothing to decide. It changes no lifecycle, no impact
// disposition and no next act: a stop is a recorded decision, not a
// cancellation.
func Dispose(h *Handoff, d ProductDisposition) (*Handoff, error) {
	const p = "$.product.dispositions"
	if h.Product == nil {
		return nil, refuse(RuleDispositionScope, "$.product", "the receipt carries no product binding to decide")
	}
	if want := h.Digest(); d.Basis != want {
		return nil, refuse(RuleStaleDisposition, p+".basis", "the disposition was prepared on receipt %s, but the receipt is now %s: re-read it and decide again", d.Basis, want)
	}
	q, err := ResolveQualifiedID(d.Subject, nil)
	if err != nil {
		return nil, refuse(RuleQualifiedID, p+".subject", "%v", err)
	}
	d.Subject = q.String()
	var target *ProductAssessment
	for i, a := range h.Product.Assessments {
		if a.Subject == d.Subject && deref(a.Outcome) == d.Outcome {
			target = &h.Product.Assessments[i]
			break
		}
	}
	switch {
	case target == nil:
		return nil, refuse(RuleDispositionScope, p+".subject", "%s has no product assessment for %s", d.Subject, d.Outcome)
	case target.Current == nil:
		return nil, refuse(RuleDispositionScope, p+".expected_revision", "%s has no current approved product target (%s): nothing to decide against", d.Outcome, target.Coverage)
	case target.Result != ResultMismatch && target.Coverage != CoverageUnbound && target.Coverage != CoverageUndiscovered:
		return nil, refuse(RuleDispositionScope, p+".decision", "%s is %s (%s) for %s: there is no drift to decide", d.Subject, target.Result, target.Coverage, d.Outcome)
	case d.ExpectedRevision != *target.Current:
		return nil, refuse(RuleStaleDisposition, p+".expected_revision", "the disposition expects product target %s, but the current approved target is %s: a stale rebinding", d.ExpectedRevision, *target.Current)
	case d.DecidedBy != h.Readiness.DecidingRole:
		return nil, refuse(RuleDispositionScope, p+".decided_by", "%q is not the receipt's deciding role %q", d.DecidedBy, h.Readiness.DecidingRole)
	}
	c, err := DecodeHandoff(canonical(h))
	if err != nil {
		return nil, err
	}
	c.Product.Dispositions = append(c.Product.Dispositions, d)
	c.Readiness.Prerequisites = withProductScope(c.Readiness.Prerequisites, c.Product)
	out, err := roundTrip(c)
	var refusal *Error
	if errors.As(err, &refusal) {
		return nil, refusal // a malformed decision, rationale or role name
	}
	return out, err
}
