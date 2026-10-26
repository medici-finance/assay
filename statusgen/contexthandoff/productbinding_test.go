package contexthandoff

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

// The product journey binds two alpha briefs to one product outcome of
// repo-a's requirement register. The architecture target of the base journey
// is untouched: product targets are a separate snapshot input.
const (
	productOutcome = "example:repo-a:REQ-nightly-export"
	productRival   = "example:repo-a:REQ-ledger-archive"
)

func productTarget(outcome, rev, approval string) map[string]any {
	return map[string]any{"outcome": outcome, "revision": rev, "approval": approval, "reference": "spec/ledger-export-v2.md"}
}

// productJourney is the base journey with alpha/03 and alpha/04 bound to
// revision r1 of the product outcome, and the given product targets.
func productJourney(t *testing.T, targets ...map[string]any) fstest.MapFS {
	t.Helper()
	m := loadJourney(t)
	bindBrief(t, m, fileA03, "REQ-nightly-export", "r1")
	bindBrief(t, m, fileA04, "REQ-nightly-export", "r1")
	setProductTargets(t, m, targets...)
	return m
}

func bindBrief(t *testing.T, m fstest.MapFS, file, outcome, rev string) {
	t.Helper()
	keys := "outcome: " + outcome + "\n"
	if rev != "" {
		keys += "outcome-revision: " + rev + "\n"
	}
	replaceIn(t, m, file, "\n---\n", "\n"+keys+"---\n")
}

func setProductTargets(t *testing.T, m fstest.MapFS, targets ...map[string]any) {
	t.Helper()
	editSnapshot(t, m, func(v map[string]any) {
		pr, _ := v["product"].(map[string]any)
		if pr == nil {
			pr = map[string]any{"relationships": []any{}}
		}
		list := []any{}
		for _, tg := range targets {
			list = append(list, tg)
		}
		pr["targets"] = list
		v["product"] = pr
	})
}

func assessmentOf(h *Handoff, subject, outcome, relation string) *ProductAssessment {
	if h.Product == nil {
		return nil
	}
	for i, a := range h.Product.Assessments {
		if a.Subject == subject && deref(a.Outcome) == outcome && a.Relation == relation {
			return &h.Product.Assessments[i]
		}
	}
	return nil
}

func bindingOf(h *Handoff, subject string) *ProductBinding {
	if h.Product == nil {
		return nil
	}
	for i, b := range h.Product.Bindings {
		if b.Subject == subject {
			return &h.Product.Bindings[i]
		}
	}
	return nil
}

func wantAssessment(t *testing.T, h *Handoff, subject, outcome, relation, result, coverage string) *ProductAssessment {
	t.Helper()
	a := assessmentOf(h, subject, outcome, relation)
	if a == nil {
		t.Fatalf("no %s product assessment of %s for %q in %+v", relation, subject, outcome, h.Product)
	}
	if a.Result != result || a.Coverage != coverage {
		t.Errorf("%s %s: result %s (%s), want %s (%s): %s", subject, outcome, a.Result, a.Coverage, result, coverage, a.Reason)
	}
	return a
}

// regression: statusgen/23 Verify row 1 — one revision field for both the
// architecture and the product target lets a product change move the
// architecture baseline, or the architecture revision pose as the product one.
func TestProductBindingIndependentTargets(t *testing.T) {
	m := productJourney(t, productTarget(productOutcome, "r1", ProductApproved))
	prior := discover(t, snap(t, m, Limits{}))
	if prior.Schema != HandoffSchemaV3 || prior.Product == nil {
		t.Fatalf("a snapshot with product input gives schema %s, product %v; want %s with a product section", prior.Schema, prior.Product, HandoffSchemaV3)
	}
	a := wantAssessment(t, prior, idA04, productOutcome, RelationPrimary, ResultAligned, CoverageCovered)
	if deref(a.Bound) != "r1" || deref(a.Current) != "r1" {
		t.Errorf("alpha/04 compares bound %s with current %s, want r1 with r1", deref(a.Bound), deref(a.Current))
	}
	if !contains(prior.Readiness.Prerequisites, ProductAssessed) || contains(prior.Readiness.Prerequisites, ArchitectureOnly) {
		t.Errorf("prerequisites %v, want the product-assessed scope line in place of the architecture-only one", prior.Readiness.Prerequisites)
	}

	// The approved product target moves to r2; the architecture target does not.
	setProductTargets(t, m, productTarget(productOutcome, "r1", ProductApproved), productTarget(productOutcome, "r2", ProductApproved))
	s := snap(t, m, Limits{})
	fresh := discover(t, s)
	if !reflect.DeepEqual(fresh.Target, prior.Target) {
		t.Errorf("a product target change moved the architecture baseline: %+v, was %+v", fresh.Target, prior.Target)
	}
	a = wantAssessment(t, fresh, idA04, productOutcome, RelationPrimary, ResultMismatch, CoverageCovered)
	if deref(a.Bound) != "r1" || deref(a.Current) != "r2" {
		t.Errorf("alpha/04 compares bound %s with current %s, want r1 with r2", deref(a.Bound), deref(a.Current))
	}
	if im := impactOf(fresh, idA04); im == nil || !reflect.DeepEqual(*im, *impactOf(prior, idA04)) {
		t.Errorf("a product mismatch changed the architecture impact of alpha/04: %+v", im)
	}

	rc, err := Recheck(prior, s)
	if err != nil {
		t.Fatal(err)
	}
	if rc.Reconcile() || rc.Hold() {
		t.Errorf("a product target change forced architecture reconciliation: %+v", rc.Findings)
	}
	if f := findingOf(rc.Findings, "product-target:"+productOutcome+"@r2"); f == nil || f.Class != FindingAssess {
		t.Errorf("findings %+v, want the new approved product target assessed", rc.Findings)
	}
	if f := findingOf(rc.Findings, "product:"+idA04+":"+productOutcome); f == nil || f.Class != FindingAssess {
		t.Errorf("findings %+v, want alpha/04's binding assessed", rc.Findings)
	}
	if !reflect.DeepEqual(rc.Receipt.Target, prior.Target) || rc.Receipt.Readiness.NextAct != prior.Readiness.NextAct {
		t.Errorf("recheck moved the architecture baseline or next act: %+v / %s", rc.Receipt.Target, rc.Receipt.Readiness.NextAct)
	}

	// The brief rebinds to r2: the original selection stays r1.
	replaceIn(t, m, fileA04, "outcome-revision: r1", "outcome-revision: r2")
	rc2, err := Recheck(rc.Receipt, snap(t, m, Limits{}))
	if err != nil {
		t.Fatal(err)
	}
	b := bindingOf(rc2.Receipt, idA04)
	if b == nil || deref(b.Declared) != "r2" || deref(b.Selected) != "r1" {
		t.Errorf("rebound alpha/04 binding %+v, want declared r2 and the original selection r1", b)
	}
	wantAssessment(t, rc2.Receipt, idA04, productOutcome, RelationPrimary, ResultAligned, CoverageCovered)
	if !reflect.DeepEqual(rc2.Receipt.Target, prior.Target) {
		t.Errorf("rebinding moved the architecture baseline: %+v", rc2.Receipt.Target)
	}
}

// regression: statusgen/23 Verify row 2 — treating an absent binding as
// aligned reports unchecked work as current, and lets a rival stale it.
func TestProductBindingApprovalCoverage(t *testing.T) {
	m := productJourney(t,
		productTarget(productOutcome, "r1", ProductApproved),
		productTarget(productOutcome, "r2", ProductUnapproved),
		productTarget(productRival, "r1", ProductApproved),
	)
	// alpha/01 is a legacy brief: no outcome at all. alpha/02 names an
	// outcome but no revision. alpha/04 also relates to a second outcome its
	// own binding does not name.
	bindBrief(t, m, fileA02, "REQ-nightly-export", "")
	editSnapshot(t, m, func(v map[string]any) {
		pr := v["product"].(map[string]any)
		pr["relationships"] = []any{
			map[string]any{"subject": "alpha:04", "outcome": productRival},
		}
	})
	prior := discover(t, snap(t, m, Limits{}))

	// An unapproved rival r2 does not stale approved work bound to r1.
	wantAssessment(t, prior, idA04, productOutcome, RelationPrimary, ResultAligned, CoverageCovered)
	// A missing binding is unknown, never aligned.
	for _, id := range []string{idA01, idA02} {
		if b := bindingOf(prior, id); b == nil || b.State != BindingUnbound {
			t.Errorf("%s binding %+v, want unbound", id, b)
		}
	}
	wantAssessment(t, prior, idA02, productOutcome, RelationPrimary, ResultUnknown, CoverageUnbound)
	wantAssessment(t, prior, idA01, "", RelationPrimary, ResultUnknown, CoverageUnbound)
	for _, a := range prior.Product.Assessments {
		if b := bindingOf(prior, a.Subject); b != nil && b.State == BindingUnbound && a.Result == ResultAligned {
			t.Errorf("unbound %s reported aligned", a.Subject)
		}
	}
	// A secondary relationship the binding does not name is undiscovered.
	wantAssessment(t, prior, idA04, productRival, RelationSecondary, ResultUnknown, CoverageUndiscovered)
	if !hasLine(prior.Readiness.Prerequisites, "product: unknown: "+idA04+" "+productRival+" (secondary): undiscovered") {
		t.Errorf("prerequisites %v, want the undiscovered secondary relationship named", prior.Readiness.Prerequisites)
	}

	// A recheck that only adds another unapproved rival ignores it.
	setProductTargets(t, m,
		productTarget(productOutcome, "r1", ProductApproved),
		productTarget(productOutcome, "r2", ProductUnapproved),
		productTarget(productOutcome, "r3", ProductUnapproved),
		productTarget(productRival, "r1", ProductApproved),
	)
	rc, err := Recheck(prior, snap(t, m, Limits{}))
	if err != nil {
		t.Fatal(err)
	}
	if f := findingOf(rc.Findings, "product-target:"+productOutcome+"@r3"); f == nil || f.Class != FindingIgnored {
		t.Errorf("findings %+v, want the unapproved rival ignored", rc.Findings)
	}
	if len(rc.Findings) != 1 {
		t.Errorf("findings %+v, want only the ignored rival", rc.Findings)
	}

	// An approval this run cannot read is could-not-check, never aligned.
	setProductTargets(t, m,
		productTarget(productOutcome, "r1", ProductApproved),
		productTarget(productOutcome, "r4", ProductUnavailable),
		productTarget(productRival, "r1", ProductApproved),
	)
	gap := discover(t, snap(t, m, Limits{}))
	wantAssessment(t, gap, idA04, productOutcome, RelationPrimary, ResultUnknown, CoverageApprovalUnavailable)

	// No product target for the outcome at all: target-unknown.
	setProductTargets(t, m, productTarget(productRival, "r1", ProductApproved))
	wantAssessment(t, discover(t, snap(t, m, Limits{})), idA04, productOutcome, RelationPrimary, ResultUnknown, CoverageTargetUnknown)

	// The record contract refuses an aligned result over an unbound binding.
	bad := reshapeRaw(t, prior, func(v map[string]any) {
		for _, x := range v["product"].(map[string]any)["assessments"].([]any) {
			am := x.(map[string]any)
			if am["coverage"] == CoverageUnbound {
				am["result"], am["coverage"], am["bound"], am["current"] = ResultAligned, CoverageCovered, "r1", "r1"
				return
			}
		}
		t.Fatal("fixture error: no unbound assessment")
	})
	if _, err := DecodeHandoff(bad); RuleOf(err) != RuleCoverage {
		t.Errorf("an aligned result over an unbound binding: %v, want %s", err, RuleCoverage)
	}
	unk := reshapeRaw(t, prior, func(v map[string]any) {
		for _, x := range v["product"].(map[string]any)["assessments"].([]any) {
			am := x.(map[string]any)
			if am["coverage"] == CoverageUnbound {
				am["coverage"] = CoverageCovered
				return
			}
		}
	})
	if _, err := DecodeHandoff(unk); RuleOf(err) != RuleCoverage {
		t.Errorf("an unknown result claiming covered coverage: %v, want %s", err, RuleCoverage)
	}

	// Legacy receipts stay architecture-only and unbound by default.
	legacy := discover(t, snap(t, loadJourney(t), Limits{}))
	if legacy.Product != nil || legacy.Schema == HandoffSchemaV3 || !contains(legacy.Readiness.Prerequisites, ArchitectureOnly) {
		t.Errorf("a snapshot with no product input gave schema %s, product %v", legacy.Schema, legacy.Product)
	}
}

func hasLine(lines []string, prefix string) bool {
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

// reshapeRaw edits the canonical JSON of h without re-validating it.
func reshapeRaw(t *testing.T, h *Handoff, f func(map[string]any)) []byte {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(h.Canonical(), &v); err != nil {
		t.Fatal(err)
	}
	f(v)
	return marshal(t, v)
}

// TestProductV3SchemaParity holds the v3 schema and the decoder in agreement
// over every structural mutation of a product-bearing receipt, and keeps the
// product section out of v1 and v2.
func TestProductV3SchemaParity(t *testing.T) {
	m := productJourney(t, productTarget(productOutcome, "r1", ProductApproved), productTarget(productOutcome, "r2", ProductApproved))
	h := discover(t, snap(t, m, Limits{}))
	h, err := Dispose(h, ProductDisposition{
		Subject: idA04, Outcome: productOutcome, ExpectedRevision: "r2", Decision: DecisionContinue,
		Rationale: "the reader is unaffected by r2", DecidedBy: "brief-author", Basis: h.Digest(),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := h.Canonical()
	schema := loadSchema(t, "../../schemas/context-handoff-v3.json")
	if err := schema.Accepts(raw); err != nil {
		t.Fatalf("v3 schema refuses a discovered receipt: %v", err)
	}
	for _, v := range mutations(parseTree(t, raw)) {
		data, _ := json.Marshal(v.tree)
		_, err := DecodeHandoff(data)
		schemaErr := schema.Accepts(data)
		if err == nil && schemaErr != nil {
			t.Errorf("%s: decoder accepts, schema refuses: %v", v.desc, schemaErr)
		}
		if err != nil && schemaErr == nil && !packageOnlyRules[RuleOf(err)] {
			t.Errorf("%s: decoder/schema drift: %v", v.desc, err)
		}
	}
	for _, old := range []string{HandoffSchema, HandoffSchemaV2} {
		data := bytes.Replace(raw, []byte(`"`+HandoffSchemaV3+`"`), []byte(`"`+old+`"`), 1)
		if _, err := DecodeHandoff(data); err == nil {
			t.Errorf("%s accepted a product section", old)
		}
	}
	var v map[string]any
	_ = json.Unmarshal(raw, &v)
	delete(v, "product")
	if _, err := DecodeHandoff(marshal(t, v)); err == nil {
		t.Error("v3 accepted a receipt with no product section")
	}
	if err := schema.Accepts(marshal(t, v)); err == nil {
		t.Error("v3 schema accepted a receipt with no product section")
	}
}

// productGapReceipt is a receipt whose alpha/04 also relates to the rival
// outcome, an undiscovered secondary assessment with a current approved
// target, and the receipt after a continue decision on that gap.
func productGapReceipt(t *testing.T) (*Handoff, *Handoff, fstest.MapFS) {
	t.Helper()
	m := productJourney(t, productTarget(productOutcome, "r1", ProductApproved), productTarget(productRival, "r1", ProductApproved))
	editSnapshot(t, m, func(v map[string]any) {
		v["product"].(map[string]any)["relationships"] = []any{map[string]any{"subject": "alpha:04", "outcome": productRival}}
	})
	h := discover(t, snap(t, m, Limits{}))
	out, err := Dispose(h, ProductDisposition{Subject: idA04, Outcome: productRival, ExpectedRevision: "r1",
		Decision: DecisionContinue, Rationale: "out of scope for this change", DecidedBy: h.Readiness.DecidingRole, Basis: h.Digest()})
	if err != nil {
		t.Fatal(err)
	}
	return h, out, m
}

// A decision Dispose accepts on an unbound or undiscovered gap is projected as
// a prerequisite line naming the decision and the gap, in place of the bare
// gap line, and survives a recheck.
func TestProductGapDecisionProjected(t *testing.T) {
	_, out, m := productGapReceipt(t)
	want := "product: continue: " + idA04 + " " + productRival + " (secondary, undiscovered) current r1 (decided by " + out.Readiness.DecidingRole + ")"
	if !contains(out.Readiness.Prerequisites, want) {
		t.Errorf("prerequisites %v, want the recorded gap decision %q", out.Readiness.Prerequisites, want)
	}
	if hasLine(out.Readiness.Prerequisites, "product: unknown: "+idA04+" "+productRival) {
		t.Errorf("prerequisites %v still show the decided gap as undecided", out.Readiness.Prerequisites)
	}
	rc, err := Recheck(out, snap(t, m, Limits{}))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(rc.Receipt.Readiness.Prerequisites, want) {
		t.Errorf("recheck prerequisites %v, want the carried gap decision %q", rc.Receipt.Readiness.Prerequisites, want)
	}
	// Once the outcome's current target moves on, the decision is history and
	// the gap is undecided again.
	setProductTargets(t, m, productTarget(productOutcome, "r1", ProductApproved), productTarget(productRival, "r1", ProductApproved), productTarget(productRival, "r2", ProductApproved))
	rc, err = Recheck(out, snap(t, m, Limits{}))
	if err != nil {
		t.Fatal(err)
	}
	if hasLine(rc.Receipt.Readiness.Prerequisites, "product: continue: ") || !hasLine(rc.Receipt.Readiness.Prerequisites, "product: unknown: "+idA04+" "+productRival+" (secondary): undiscovered") {
		t.Errorf("prerequisites %v, want the superseded gap decision out of force", rc.Receipt.Readiness.Prerequisites)
	}
}

// linesFor are the product prerequisite lines that name subject and outcome.
func linesFor(lines []string, subject, outcome string) []string {
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, "product: ") && strings.Contains(l, " "+subject+" "+outcome+" ") {
			out = append(out, l)
		}
	}
	return out
}

// A disposition stays in force on an aligned or approval-unavailable
// assessment, held in the product section, but no prerequisite line names the
// decision: an aligned assessment prints no line and an approval-unavailable
// one prints only its plain gap line.
func TestProductInForceDecisionUnprojected(t *testing.T) {
	adapted := func(t *testing.T) (*Handoff, fstest.MapFS) {
		t.Helper()
		m := productJourney(t, productTarget(productOutcome, "r1", ProductApproved), productTarget(productOutcome, "r2", ProductApproved))
		h := discover(t, snap(t, m, Limits{}))
		out, err := Dispose(h, ProductDisposition{Subject: idA04, Outcome: productOutcome, ExpectedRevision: "r2",
			Decision: DecisionAdapt, Rationale: "adapt to r2", DecidedBy: h.Readiness.DecidingRole, Basis: h.Digest()})
		if err != nil {
			t.Fatal(err)
		}
		return out, m
	}
	check := func(t *testing.T, rc *RecheckResult, result, coverage string, want []string) {
		t.Helper()
		a := wantAssessment(t, rc.Receipt, idA04, productOutcome, RelationPrimary, result, coverage)
		if deref(a.Current) != "r2" {
			t.Errorf("current %q, want r2", deref(a.Current))
		}
		d := rc.Receipt.Product.effective(*a)
		if d == nil || d.Decision != DecisionAdapt || d.ExpectedRevision != "r2" {
			t.Errorf("disposition in force %+v, want the adapt at r2 carried and in force", d)
		}
		if got := linesFor(rc.Receipt.Readiness.Prerequisites, idA04, productOutcome); !reflect.DeepEqual(got, want) {
			t.Errorf("lines for %s %s: %q, want exactly %q", idA04, productOutcome, got, want)
		}
	}

	t.Run("aligned", func(t *testing.T) {
		out, m := adapted(t)
		replaceIn(t, m, fileA04, "outcome-revision: r1\n", "outcome-revision: r2\n")
		rc, err := Recheck(out, snap(t, m, Limits{}))
		if err != nil {
			t.Fatal(err)
		}
		check(t, rc, ResultAligned, CoverageCovered, nil)
	})
	t.Run("approval-unavailable", func(t *testing.T) {
		out, m := adapted(t)
		setProductTargets(t, m, productTarget(productOutcome, "r1", ProductApproved), productTarget(productOutcome, "r2", ProductApproved), productTarget(productOutcome, "r3", ProductUnavailable))
		rc, err := Recheck(out, snap(t, m, Limits{}))
		if err != nil {
			t.Fatal(err)
		}
		check(t, rc, ResultUnknown, CoverageApprovalUnavailable, []string{"product: unknown: " + idA04 + " " + productOutcome + " (primary): approval-unavailable"})
	})
}

// A decision covers every state of its subject and outcome at the revision it
// names: a continue recorded on an unbound gap stays in force, and is shown,
// once the brief binds to another revision and the assessment is a mismatch.
func TestProductDecisionCarriesAcrossStates(t *testing.T) {
	m := productJourney(t, productTarget(productOutcome, "r1", ProductApproved))
	replaceIn(t, m, fileA04, "outcome-revision: r1\n", "")
	h := discover(t, snap(t, m, Limits{}))
	wantAssessment(t, h, idA04, productOutcome, RelationPrimary, ResultUnknown, CoverageUnbound)
	out, err := Dispose(h, ProductDisposition{Subject: idA04, Outcome: productOutcome, ExpectedRevision: "r1",
		Decision: DecisionContinue, Rationale: "unbound is fine for now", DecidedBy: h.Readiness.DecidingRole, Basis: h.Digest()})
	if err != nil {
		t.Fatal(err)
	}
	replaceIn(t, m, fileA04, "outcome: REQ-nightly-export\n", "outcome: REQ-nightly-export\noutcome-revision: r0\n")
	rc, err := Recheck(out, snap(t, m, Limits{}))
	if err != nil {
		t.Fatal(err)
	}
	wantAssessment(t, rc.Receipt, idA04, productOutcome, RelationPrimary, ResultMismatch, CoverageCovered)
	want := []string{"product: continue: " + idA04 + " " + productOutcome + " bound r0, current r1 (decided by " + rc.Receipt.Readiness.DecidingRole + ")"}
	if got := linesFor(rc.Receipt.Readiness.Prerequisites, idA04, productOutcome); !reflect.DeepEqual(got, want) {
		t.Errorf("lines for %s %s: %q, want the gap decision carried onto the mismatch %q", idA04, productOutcome, got, want)
	}
}

// A snapshot that lists one product relationship twice, in either spelling of
// the subject, is refused as a snapshot error.
func TestProductSnapshotRelationshipListedTwice(t *testing.T) {
	m := productJourney(t, productTarget(productOutcome, "r1", ProductApproved), productTarget(productRival, "r1", ProductApproved))
	editSnapshot(t, m, func(v map[string]any) {
		v["product"].(map[string]any)["relationships"] = []any{
			map[string]any{"subject": "alpha:04", "outcome": productRival},
			map[string]any{"subject": idA04, "outcome": productRival},
		}
	})
	_, err := LoadSnapshot(m, Limits{})
	if err == nil || !strings.HasPrefix(err.Error(), "snapshot: ") || !strings.Contains(err.Error(), "listed twice") {
		t.Errorf("a relationship listed twice: %v, want a snapshot error naming the repeat", err)
	}
}

// A disposition carried in a receipt is checked again on decode and on recheck:
// one not recorded by the receipt's deciding role is refused, and a recheck
// whose snapshot changes the deciding role does not carry dispositions across.
func TestProductCarriedDispositionRole(t *testing.T) {
	_, out, m := productGapReceipt(t)
	other := reshapeRaw(t, out, func(v map[string]any) {
		d := v["product"].(map[string]any)["dispositions"].([]any)[0].(map[string]any)
		d["decided_by"] = "another-role"
	})
	if _, err := DecodeHandoff(other); RuleOf(err) != RuleDispositionScope {
		t.Errorf("a carried disposition from another role decodes: %v, want %s", err, RuleDispositionScope)
	}

	prior := *out
	pr := *out.Product
	pr.Dispositions = append([]ProductDisposition{}, out.Product.Dispositions...)
	pr.Dispositions[0].DecidedBy = "another-role"
	prior.Product = &pr
	if _, err := Recheck(&prior, snap(t, m, Limits{})); RuleOf(err) != RuleDispositionScope {
		t.Errorf("recheck of a prior with a disposition from another role: %v, want %s", err, RuleDispositionScope)
	}

	editSnapshot(t, m, func(v map[string]any) { v["deciding_role"] = "another-role" })
	if _, err := Recheck(out, snap(t, m, Limits{})); err == nil || !strings.Contains(err.Error(), "deciding role") {
		t.Errorf("recheck under a changed deciding role: %v, want a snapshot refusal naming the deciding role", err)
	}
}

// An assessment's current revision is the last approved target of its outcome
// in the receipt's own targets list, on decode and on recheck.
func TestProductAssessmentCurrentIsListedTarget(t *testing.T) {
	m := productJourney(t, productTarget(productOutcome, "r1", ProductApproved), productTarget(productOutcome, "r2", ProductUnapproved))
	h := discover(t, snap(t, m, Limits{}))
	edit := func(f func(am map[string]any)) []byte {
		return reshapeRaw(t, h, func(v map[string]any) {
			for _, x := range v["product"].(map[string]any)["assessments"].([]any) {
				am := x.(map[string]any)
				if am["subject"] == idA04 && am["outcome"] == productOutcome {
					f(am)
					return
				}
			}
			t.Fatal("fixture error: no alpha/04 assessment")
		})
	}
	for name, raw := range map[string][]byte{
		"unapproved": edit(func(am map[string]any) { am["current"], am["result"] = "r2", ResultMismatch }),
		"unlisted":   edit(func(am map[string]any) { am["current"], am["result"] = "r9", ResultMismatch }),
	} {
		if _, err := DecodeHandoff(raw); RuleOf(err) != RuleCoverage {
			t.Errorf("an assessment current at an %s revision decodes: %v, want %s", name, err, RuleCoverage)
		}
	}

	prior := *h
	pr := *h.Product
	pr.Assessments = append([]ProductAssessment{}, h.Product.Assessments...)
	for i := range pr.Assessments {
		if pr.Assessments[i].Subject == idA03 && deref(pr.Assessments[i].Outcome) == productOutcome {
			pr.Assessments[i].Current, pr.Assessments[i].Result = ptr("r9"), ResultMismatch
		}
	}
	prior.Product = &pr
	if _, err := Recheck(&prior, snap(t, m, Limits{})); RuleOf(err) != RuleCoverage {
		t.Errorf("recheck of a prior with an unlisted current revision: %v, want %s", err, RuleCoverage)
	}
}
