package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/contexthandoff"
)

const (
	ctxProductOutcome = "example:repo-a:REQ-nightly-export"
	ctxBriefA04       = "sources/repo-a/docs/streams/alpha/brief-04-export-reader.md"
	ctxBriefA03       = "sources/repo-a/docs/streams/alpha/brief-03-export-layout-v2.md"
	ctxIDA04          = "example:repo-a:alpha:04"
)

// ctxEditFile replaces old with new, once, in a snapshot file on disk.
func ctxEditFile(t *testing.T, dir, rel, old, new string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), old) {
		t.Fatalf("%s does not contain %q", rel, old)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(string(b), old, new, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// ctxProductTargets sets the snapshot's product targets for the outcome, in
// order, each approved.
func ctxProductTargets(t *testing.T, dir string, revs ...string) {
	t.Helper()
	p := filepath.Join(dir, "snapshot.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	targets := []any{}
	for _, r := range revs {
		targets = append(targets, map[string]any{"outcome": ctxProductOutcome, "revision": r, "approval": "approved", "reference": "spec/ledger-export-v2.md"})
	}
	v["product"] = map[string]any{"targets": targets, "relationships": []any{}}
	out, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(p, out, 0o600); err != nil {
		t.Fatal(err)
	}
}

func ctxDecodeReceipt(t *testing.T, raw string) *contexthandoff.Handoff {
	t.Helper()
	h, err := contexthandoff.DecodeHandoff([]byte(raw))
	if err != nil {
		t.Fatalf("not a context handoff: %v\n%s", err, raw)
	}
	return h
}

func ctxProductOf(t *testing.T, h *contexthandoff.Handoff, subject string) (*contexthandoff.ProductBinding, *contexthandoff.ProductAssessment) {
	t.Helper()
	if h.Product == nil {
		t.Fatalf("receipt %s carries no product section", h.Digest())
	}
	var b *contexthandoff.ProductBinding
	var a *contexthandoff.ProductAssessment
	for i := range h.Product.Bindings {
		if h.Product.Bindings[i].Subject == subject {
			b = &h.Product.Bindings[i]
		}
	}
	for i := range h.Product.Assessments {
		x := &h.Product.Assessments[i]
		if x.Subject == subject && x.Relation == contexthandoff.RelationPrimary {
			a = x
		}
	}
	if b == nil || a == nil {
		t.Fatalf("no product binding or assessment of %s", subject)
	}
	return b, a
}

func ctxStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// regression: statusgen/23 Verify row 3 — a rebinding that replaces the
// original selection revision loses the record of what the work was first
// selected against; a disposition against a target that moved must refuse.
func TestProductBindingCheckpointFlow(t *testing.T) {
	bin := buildStatusgenBinary(t)
	dir := ctxCopyJourney(t)
	for _, f := range []string{ctxBriefA03, ctxBriefA04} {
		ctxEditFile(t, dir, f, "\n---\n", "\noutcome: REQ-nightly-export\noutcome-revision: r1\n---\n")
	}
	ctxProductTargets(t, dir, "r1")

	// The real brief reader accepts the binding keys on a brief-v1 brief and
	// flags a revision that binds no outcome or is not a revision name.
	reqRoot := requirementRoot(t, validRequirementFixture())
	st := costBrief(t, reqRoot, "2026-09-24", "outcome: REQ-evidence-visible\noutcome-revision: r1\n")
	bf, found, err := parseBriefFile(filepath.Join(st.Dir, "brief-01.md"))
	if err != nil || !found || ctxStr(bf.OutcomeRevision) != "r1" {
		t.Fatalf("brief reader: %+v found %t err %v, want outcome-revision r1", bf, found, err)
	}
	if problems, _ := checkBriefFiles([]*Stream{st}, []*Stream{st}); mentioning(problems, "outcome") != 0 {
		t.Errorf("a bound brief is flagged:\n%s", joined(problems))
	}
	for _, bad := range []string{"outcome: none\noutcome-revision: r1\n", "outcome-revision: r1\n", "outcome: REQ-evidence-visible\noutcome-revision: R1\n"} {
		st := costBrief(t, requirementRoot(t, validRequirementFixture()), "2026-09-24", bad)
		if problems, _ := checkBriefFiles([]*Stream{st}, []*Stream{st}); mentioning(problems, "outcome-revision") != 1 {
			t.Errorf("%q must be one outcome-revision PROBLEM; got:\n%s", bad, joined(problems))
		}
	}
	st = costBrief(t, t.TempDir(), "2026-09-24", "outcome: REQ-evidence-visible\noutcome-revision: [r1]\n")
	if _, _, err := parseBriefFile(filepath.Join(st.Dir, "brief-01.md")); err == nil {
		t.Error("a list outcome-revision must be a parse error")
	}

	// 1. Discover: alpha/04 is selected against r1, which is current.
	h1, raw1 := ctxDiscoverReceipt(t, bin, dir)
	if h1.Schema != contexthandoff.HandoffSchemaV3 {
		t.Fatalf("schema %s, want %s", h1.Schema, contexthandoff.HandoffSchemaV3)
	}
	if b, a := ctxProductOf(t, h1, ctxIDA04); ctxStr(b.Selected) != "r1" || a.Result != contexthandoff.ResultAligned {
		t.Errorf("first receipt: binding %+v, assessment %+v, want selected r1 and aligned", b, a)
	}
	text, _, code := ctxRun(t, bin, "discover", "--snapshot", dir)
	if code != 0 || !strings.Contains(text, "scope: "+contexthandoff.ProductAssessed) || !strings.Contains(text, "product primary "+ctxIDA04+" "+ctxProductOutcome+": aligned (covered)") {
		t.Errorf("discover text: exit %d\n%s", code, text)
	}

	// 2. The approved product target moves to r2: recheck assesses, never holds.
	ctxProductTargets(t, dir, "r1", "r2")
	out, errOut, code := ctxRun(t, bin, "recheck", "--snapshot", dir, ctxWriteTemp(t, "r1.json", raw1), "--json")
	var rc struct {
		Outcome  string                   `json:"outcome"`
		Receipt  json.RawMessage          `json:"receipt"`
		Findings []contexthandoff.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &rc); err != nil || code != 0 || rc.Outcome != "current" {
		t.Fatalf("recheck after a product target change: exit %d outcome %q err %v\n%s%s", code, rc.Outcome, err, out, errOut)
	}
	assessed := false
	for _, f := range rc.Findings {
		assessed = assessed || f.Class == contexthandoff.FindingAssess && f.Scope == "product:"+ctxIDA04+":"+ctxProductOutcome
	}
	if !assessed {
		t.Errorf("findings %+v, want alpha/04's binding assessed", rc.Findings)
	}
	h2 := ctxDecodeReceipt(t, string(rc.Receipt))
	if _, a := ctxProductOf(t, h2, ctxIDA04); a.Result != contexthandoff.ResultMismatch || ctxStr(a.Current) != "r2" {
		t.Errorf("rechecked assessment %+v, want a mismatch against current r2", a)
	}
	r2 := ctxWriteTemp(t, "r2.json", h2.Canonical())

	// 3. A disposition against the old target revision is a stale rebinding.
	dispose := func(receipt, expected, basis string) (string, string, int) {
		return ctxRun(t, bin, "dispose", receipt, "--subject", ctxIDA04, "--outcome", ctxProductOutcome,
			"--decision", "adapt", "--expected-revision", expected, "--rationale", "the reader adopts the r2 export fields",
			"--by", h2.Readiness.DecidingRole, "--basis", basis, "--json")
	}
	if _, errOut, code := dispose(r2, "r1", h2.Digest()); code != 1 || !strings.Contains(errOut, contexthandoff.RuleStaleDisposition) {
		t.Errorf("disposition against the superseded target: exit %d, want 1 %s\n%s", code, contexthandoff.RuleStaleDisposition, errOut)
	}
	if _, errOut, code := dispose(r2, "r2", h1.Digest()); code != 1 || !strings.Contains(errOut, contexthandoff.RuleStaleDisposition) {
		t.Errorf("disposition prepared on another receipt: exit %d, want 1 %s\n%s", code, contexthandoff.RuleStaleDisposition, errOut)
	}
	out, errOut, code = dispose(r2, "r2", h2.Digest())
	if code != 0 {
		t.Fatalf("dispose: exit %d\n%s", code, errOut)
	}
	h3 := ctxDecodeReceipt(t, out)
	if n := len(h3.Product.Dispositions); n != 1 || h3.Product.Dispositions[0].Decision != contexthandoff.DecisionAdapt {
		t.Fatalf("dispositions %+v, want the one adapt decision", h3.Product.Dispositions)
	}
	if h3.Readiness.NextAct != h2.Readiness.NextAct || len(h3.Impacts) != len(h2.Impacts) {
		t.Errorf("a disposition changed the next act or the impacts: %s / %d", h3.Readiness.NextAct, len(h3.Impacts))
	}

	// 4. The brief rebinds to r2: the original selection and the decision are
	// retained as history.
	ctxEditFile(t, dir, ctxBriefA04, "outcome-revision: r1", "outcome-revision: r2")
	out, errOut, _ = ctxRun(t, bin, "recheck", "--snapshot", dir, ctxWriteTemp(t, "r3.json", h3.Canonical()), "--json")
	if err := json.Unmarshal([]byte(out), &rc); err != nil {
		t.Fatalf("recheck after rebinding: %v\n%s%s", err, out, errOut)
	}
	h4 := ctxDecodeReceipt(t, string(rc.Receipt))
	b, a := ctxProductOf(t, h4, ctxIDA04)
	if ctxStr(b.Declared) != "r2" || ctxStr(b.Selected) != "r1" {
		t.Errorf("rebound binding declared %s selected %s, want declared r2 with the original selection r1 retained", ctxStr(b.Declared), ctxStr(b.Selected))
	}
	if a.Result != contexthandoff.ResultAligned {
		t.Errorf("rebound assessment %+v, want aligned", a)
	}
	if len(h4.Product.Dispositions) != 1 || h4.Product.Dispositions[0].ExpectedRevision != "r2" {
		t.Errorf("dispositions %+v, want the adapt decision kept as history", h4.Product.Dispositions)
	}
	if again := ctxDecodeReceipt(t, string(h4.Canonical())); again.Digest() != h4.Digest() {
		t.Error("the rebound receipt does not round-trip")
	}
}
