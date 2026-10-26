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
