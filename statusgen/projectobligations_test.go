package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// Tests for the versioned source-obligation and applicability contract
// (projectobligations.go; spec/project-obligations-v1.md). The fixtures under
// testdata/projectobligations are synthetic: no licensed text, no adopter record.
// The expected digests in the fixture and in the golden test were computed by an
// independent implementation of the spec's canonical form
// (testdata/projectobligations/canonical_digest.py), never by the code under test.
//
// Test and helper names stay short: the pre-push secret scan reads a long
// unbroken alphanumeric run as a possible credential.

const (
	poFixtureDir = "testdata/projectobligations"
	poAsOf       = "2026-10-08"
	poDRa        = "docs/streams/decisions/DR-synth-applic-a.md"
	poDRb        = "docs/streams/decisions/DR-synth-applic-b.md"
)

func poTree() string { return filepath.Join(poFixtureDir, "tree") }

func poInput(t *testing.T) obligationInput {
	t.Helper()
	in, err := readObligationInput(filepath.Join(poFixtureDir, "obligations.json"))
	if err != nil {
		t.Fatalf("fixture unreadable: %v", err)
	}
	return in
}

// poGates is the pre-fetched corroboration state: each decision issue closed by the
// blessed login ("ada" under the fixture roster) and carrying the record's marker.
func poGates() decisionGateLinks {
	return decisionGateLinks{
		poDRa: {{Ref: "example-org/tracker#77", ClosedBy: "ada", Body: decisionGateMarker("DR-synth-applic-a")}},
		poDRb: {{Ref: "example-org/tracker#78", ClosedBy: "ada", Body: decisionGateMarker("DR-synth-applic-b")}},
	}
}

func poCtx(t *testing.T) obligationCtx {
	t.Helper()
	ctx, err := loadObligationCtx(poTree(), poGates())
	if err != nil {
		t.Fatalf("loadObligationCtx: %v", err)
	}
	return ctx
}

func poVerdictOf(t *testing.T, r obligationResult, mapping string) poVerdict {
	t.Helper()
	for _, v := range r.Verdicts {
		if v.Mapping == mapping {
			return v
		}
	}
	t.Fatalf("no verdict for %s in %+v", mapping, r.Verdicts)
	return poVerdict{}
}

func hasReason(v poVerdict, want string) bool {
	for _, r := range v.Reasons {
		if r == want || strings.HasPrefix(r, want+":") {
			return true
		}
	}
	return false
}

// poRebind makes the approval for APD-synth-001 internally consistent with the
// edited decision (a fresh subject digest in the input and in a copy of the
// decision record), so a test can isolate one fault from the digest check.
func poRebind(t *testing.T, in *obligationInput) obligationCtx {
	t.Helper()
	return poRebindAt(t, in, 0, poDRa, poGates())
}

// poRebindAt re-binds decision i to its own edited subject inside a copy of the
// fixture tree whose record dr carries the fresh digest, then loads the context
// with the given corroboration state.
func poRebindAt(t *testing.T, in *obligationInput, i int, dr string, gates decisionGateLinks) obligationCtx {
	t.Helper()
	root := t.TempDir()
	copyTree(t, poTree(), root)
	d := &in.Decisions[i]
	old := d.Acceptance.SubjectDigest
	d.Acceptance.SubjectDigest = poSubjectOf(t, *in, *d)
	p := filepath.Join(root, dr)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(string(raw), old, d.Acceptance.SubjectDigest, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, err := loadObligationCtx(root, gates)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

// poSubjectOf computes a decision's subject digest over the supplied mapping and
// source revisions it names.
func poSubjectOf(t *testing.T, in obligationInput, d poDecision) string {
	t.Helper()
	for _, m := range in.Mappings {
		if m.ID != d.MappingID || m.Revision != d.MappingRevision {
			continue
		}
		for _, s := range in.Sources {
			if s.key() == m.Source.key() {
				return subjectDigest(s, m, d)
			}
		}
	}
	t.Fatalf("decision %s names no supplied mapping/source", d.ID)
	return ""
}

// poAddDR writes a synthetic decision record carrying digest into a fixture copy.
func poAddDR(t *testing.T, root, id, digest string) {
	t.Helper()
	body := "---\nid: \"" + id + "\"\ndate: \"2026-10-06\"\ntitle: \"Synthetic: later decision\"\n" +
		"consequence: \"major\"\ndecided-by: \"human:<name>\"\nalternatives:\n  - \"Keep the earlier decision.\"\n" +
		"accepted:\n  - \"The later decision governs.\"\n---\n\nSynthetic decision record. It binds exactly one subject: " + digest + "\n"
	p := filepath.Join(root, "docs/streams/decisions", id+".md")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func poCtxAt(t *testing.T, root string, gates decisionGateLinks) obligationCtx {
	t.Helper()
	ctx, err := loadObligationCtx(root, gates)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func poCopyInput(in obligationInput) obligationInput {
	out := in
	out.Sources = append([]poSource(nil), in.Sources...)
	out.Mappings = append([]poMapping(nil), in.Mappings...)
	out.Decisions = append([]poDecision(nil), in.Decisions...)
	return out
}

const poDRc = "docs/streams/decisions/DR-synth-applic-c.md"

// poGatesC is poGates plus a corroborated issue for DR-synth-applic-c.
func poGatesC() decisionGateLinks {
	g := poGates()
	g[poDRc] = []decisionGateIssue{{Ref: "example-org/tracker#80", ClosedBy: "ada", Body: decisionGateMarker("DR-synth-applic-c")}}
	return g
}

// poSupInput returns the fixture plus an accepted not-applicable decision
// APD-synth-003 that supersedes APD-synth-001, with its own record and receipt.
func poSupInput(t *testing.T) (obligationInput, string) {
	t.Helper()
	in := poInput(t)
	d := in.Decisions[0]
	d.ID, d.Outcome, d.Reason = "APD-synth-003", "not-applicable", "The release process moved out of scope."
	d.DecidedAt, d.Supersedes = "2026-10-06", "APD-synth-001"
	acc := *d.Acceptance
	acc.DecisionID, acc.CorroborationRef = "DR-synth-applic-c", "example-org/tracker#80"
	acc.SubjectDigest = poSubjectOf(t, in, d)
	d.Acceptance = &acc
	in.Decisions = append(in.Decisions, d)
	root := t.TempDir()
	copyTree(t, poTree(), root)
	poAddDR(t, root, "DR-synth-applic-c", acc.SubjectDigest)
	return in, root
}

// ---------- A1: source permissions and stable revisions ----------

func TestAssuranceSourcePermissions(t *testing.T) {
	const sentinel = "SENTINEL-LICENSED-CLAUSE-TEXT"

	t.Run("fixture is clean and metadata-only where use is denied", func(t *testing.T) {
		in := poInput(t)
		if p := validateObligationInput(in); len(p) != 0 {
			t.Fatalf("fixture must validate clean: %v", p)
		}
		mi := modelInputs(in.Sources)
		if len(mi) != 2 {
			t.Fatalf("want 2 model sources, got %d", len(mi))
		}
		for _, m := range mi {
			if m.Trust != "untrusted-data" {
				t.Errorf("%s: source text must be labelled untrusted data, got %q", m.ID, m.Trust)
			}
			switch m.ID {
			case "SRC-synth-standard":
				if m.Mode != "full" || m.Text == "" {
					t.Errorf("a permitted source should be eligible, got %+v", m)
				}
			case "SRC-synth-licensed":
				if m.Mode != "metadata-only" || m.Text != "" {
					t.Errorf("a denied source must be metadata-only, got %+v", m)
				}
			}
		}
		raw, _ := json.Marshal(mi[0])
		if strings.Contains(string(raw), "synthetic://") {
			t.Errorf("locator must not appear in the model payload: %s", raw)
		}
	})

	t.Run("denied unknown or absent use never reaches the model", func(t *testing.T) {
		for _, storage := range []string{"permitted", "denied", "unknown", ""} {
			for _, ai := range []string{"permitted", "denied", "unknown", ""} {
				for _, access := range []string{"ACCESS-x", ""} {
					s := poSource{ID: "SRC-sentinel-test", Kind: "policy", Issuer: "i", Locator: "l", Revision: "1",
						Text: sentinel, Use: poUse{AccessRef: access, Storage: storage, AI: ai}}
					raw, _ := json.Marshal(modelInputs([]poSource{s}))
					got := strings.Contains(string(raw), sentinel)
					want := storage == "permitted" && ai == "permitted" && access != ""
					if got != want {
						t.Errorf("storage=%q ai=%q access=%q: text in payload = %v, want %v", storage, ai, access, got, want)
					}
				}
			}
		}
	})

	t.Run("revisions are stable and a uri alone is not a version", func(t *testing.T) {
		a, b := poInput(t), poInput(t)
		for i := range a.Sources {
			if a.Sources[i].key() != b.Sources[i].key() {
				t.Fatalf("source key not stable across loads: %s vs %s", a.Sources[i].key(), b.Sources[i].key())
			}
		}
		dup := poInput(t)
		same := dup.Sources[0]
		same.ContentDigest = poSum("different content")
		dup.Sources = append(dup.Sources, same)
		if !containsAll(validateObligationInput(dup), "duplicate source revision", "different content digest") {
			t.Errorf("same id+revision with other content must be refused as a mutated revision")
		}
		uri := poInput(t)
		uri.Sources[1].Revision = ""
		if !containsAll(validateObligationInput(uri), "a locator alone is not a content version") {
			t.Errorf("a source with only a locator must be refused")
		}
	})

	t.Run("stored text needs permission and a matching digest", func(t *testing.T) {
		in := poInput(t)
		in.Sources[1].Text = sentinel // storage denied
		if !containsAll(validateObligationInput(in), "keep licensed text out of the record") {
			t.Errorf("text under denied storage must be refused")
		}
		in = poInput(t)
		in.Sources[0].Text += " tampered"
		if !containsAll(validateObligationInput(in), "contentDigest does not match the supplied text") {
			t.Errorf("tampered text must not match its digest")
		}
		in = poInput(t)
		in.Sources[1].ContentDigest = poSum("x") // digest without storage permission
		if !containsAll(validateObligationInput(in), "metadata only") {
			t.Errorf("a digest where storage is not permitted must be refused")
		}
	})

	t.Run("unknown dates stay explicit", func(t *testing.T) {
		in := poInput(t)
		in.Sources[0].Effective = ""
		if !containsAll(validateObligationInput(in), `the explicit value "unknown"`) {
			t.Errorf("an empty date must be refused; unknown must be written out")
		}
	})
}

// ---------- A2: applicability acceptance at the authority boundary ----------

func TestAssuranceApplicability(t *testing.T) {
	t.Run("positive control accepts and keeps not-applicable listed", func(t *testing.T) {
		in, ctx := poInput(t), poCtx(t)
		res := resolveObligations(in, ctx, poAsOf)
		if v := poVerdictOf(t, res, "MAP-synth-001@1"); v.State != poAccepted || v.DecisionID != "APD-synth-001" {
			t.Fatalf("MAP-synth-001 should be accepted, got %+v", v)
		}
		na := poVerdictOf(t, res, "MAP-synth-002@1")
		if na.State != poNotApplicable || na.Reason == "" {
			t.Fatalf("not-applicable must stay in the inventory with its reason, got %+v", na)
		}
		if got := res.Accepted(); len(got) != 1 {
			t.Errorf("only applicable accepted mappings count as accepted, got %d", len(got))
		}
		if !res.Complete() {
			t.Errorf("every mapping decided: want complete, got problems %v verdicts %+v", res.Problems, res.Verdicts)
		}
		if (obligationResult{}).Complete() {
			t.Errorf("an empty source set must never demonstrate complete coverage")
		}
	})

	// Each case damages decision APD-synth-001 in a way the SHAPE validator cannot
	// see, and resolveObligations is called WITHOUT the validator ever running. The
	// boundary alone must refuse it.
	cases := []struct {
		name   string
		edit   func(in *obligationInput)
		state  string
		reason string
	}{
		{"forged reviewer identity", func(in *obligationInput) { in.Decisions[0].Reviewer = "mallory" }, poRejected, "reviewer-not-corroborated"},
		{"forged corroboration ref", func(in *obligationInput) { in.Decisions[0].Acceptance.CorroborationRef = "example-org/tracker#999" }, poRejected, "corroboration-ref-mismatch"},
		{"preparer approves own proposal", func(in *obligationInput) { in.Decisions[0].ProposedBy = "ada" }, poRejected, "self-approval"},
		{"wrong subject: scope widened", func(in *obligationInput) { in.Decisions[0].Scope = "every project" }, poRejected, "subject-digest-mismatch"},
		{"stale: mapping content edited in place", func(in *obligationInput) { in.Mappings[0].Paraphrase += " (edited)" }, poRejected, "subject-digest-mismatch"},
		{"stale: source revision moved", func(in *obligationInput) { in.Decisions[0].Source.Revision = "2029" }, poRejected, "stale-source-revision"},
		{"stale: profile revision moved", func(in *obligationInput) { in.Decisions[0].Profile.Revision = "0" }, poRejected, "stale-profile-revision"},
		{"disposition not approved", func(in *obligationInput) { in.Decisions[0].Acceptance.Disposition = "rejected" }, poRejected, "disposition-not-approved"},
		{"typed name is not a digest", func(in *obligationInput) { in.Decisions[0].Acceptance.SubjectDigest = "approved by ada" }, poRejected, "subject-digest-mismatch"},
		{"unknown decision record", func(in *obligationInput) { in.Decisions[0].Acceptance.DecisionID = "DR-no-such-record" }, poRejected, "decision-record-unknown"},
		{"replay: another subject's record", func(in *obligationInput) {
			in.Decisions[0].Acceptance.DecisionID = "DR-synth-applic-b"
			in.Decisions[0].Acceptance.CorroborationRef = "example-org/tracker#78"
		}, poRejected, "decision-record-not-subject-bound"},
		{"review is the decision itself", func(in *obligationInput) { in.Decisions[0].Acceptance.ReviewRef = "DR-synth-applic-a" }, poRejected, "review-not-independent"},
		{"review record unknown", func(in *obligationInput) { in.Decisions[0].Acceptance.ReviewRef = "DR-no-such-review" }, poRejected, "review-ref-unknown"},
		{"review missing stays unresolved", func(in *obligationInput) { in.Decisions[0].Acceptance.ReviewRef = "" }, poUnresolved, "review-ref-missing"},
		{"no acceptance link stays unresolved", func(in *obligationInput) { in.Decisions[0].Acceptance = nil }, poUnresolved, "no-acceptance-link"},
		{"proposed outcome unresolved", func(in *obligationInput) { in.Decisions[0].Outcome = "unresolved" }, poUnresolved, "outcome-unresolved"},
		{"not yet effective", func(in *obligationInput) { in.Decisions[0].EffectiveFrom = "2026-11-01" }, poUnresolved, "outside-effective-period"},
		{"period ended", func(in *obligationInput) { in.Decisions[0].EffectiveTo = "2026-10-02" }, poUnresolved, "outside-effective-period"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in, ctx := poInput(t), poCtx(t)
			c.edit(&in)
			// re-bind the approval to the edited subject so only the edit under test is wrong
			if c.name == "proposed outcome unresolved" || c.name == "not yet effective" || c.name == "period ended" {
				ctx = poRebind(t, &in)
			}
			res := resolveObligations(in, ctx, poAsOf) // candidate validator NOT run
			v := poVerdictOf(t, res, "MAP-synth-001@1")
			if v.State == poAccepted || v.State == poNotApplicable {
				t.Fatalf("a damaged decision was accepted: %+v", v)
			}
			if v.State != c.state || !hasReason(v, c.reason) {
				t.Errorf("want state %s with reason %s, got %+v", c.state, c.reason, v)
			}
			if res.Complete() || len(res.Accepted()) != 0 {
				t.Errorf("an unauthorized mapping must not complete the set or appear accepted")
			}
		})
	}

	t.Run("boundary catches what the shape validator passes", func(t *testing.T) {
		in, ctx := poInput(t), poCtx(t)
		in.Decisions[0].Reviewer = "mallory"
		if p := validateObligationInput(in); len(p) != 0 {
			t.Fatalf("a forged-but-well-formed reviewer is shape-valid; validator said %v", p)
		}
		src := in.Sources[0]
		got := acceptApplicability(in.Decisions[0], in.Mappings[0], src, ctx, poAsOf)
		if len(got) == 0 {
			t.Fatalf("acceptApplicability must refuse the forged reviewer on its own")
		}
	})

	t.Run("a decision bound to another mapping revision is refused directly", func(t *testing.T) {
		in, ctx := poInput(t), poCtx(t)
		d := in.Decisions[0]
		d.MappingRevision = 2
		got := acceptApplicability(d, in.Mappings[0], in.Sources[0], ctx, poAsOf)
		if !containsAll(got, "stale-mapping-revision") {
			t.Errorf("want stale-mapping-revision, got %v", got)
		}
	})

	t.Run("no trusted corroboration stays unresolved", func(t *testing.T) {
		in := poInput(t)
		ctx := poCtx(t)
		ctx.Receipts = nil // nothing was pre-fetched: the loader never fetches the forge
		v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poUnresolved || !hasReason(v, "no-trusted-corroboration") {
			t.Errorf("want unresolved/no-trusted-corroboration, got %+v", v)
		}
	})

	t.Run("a receipt from the wrong closer or open issue is not trusted", func(t *testing.T) {
		in := poInput(t)
		for name, gates := range map[string]decisionGateLinks{
			"closed by a non-blessed login": {poDRa: {{Ref: "example-org/tracker#77", ClosedBy: "someone-else", Body: decisionGateMarker("DR-synth-applic-a")}}},
			"issue still open":              {poDRa: {{Ref: "example-org/tracker#77", ClosedBy: "", Body: decisionGateMarker("DR-synth-applic-a")}}},
			"marker names another record":   {poDRa: {{Ref: "example-org/tracker#77", ClosedBy: "ada", Body: decisionGateMarker("DR-synth-applic-b")}}},
		} {
			ctx, err := loadObligationCtx(poTree(), gates)
			if err != nil {
				t.Fatal(err)
			}
			v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1")
			if v.State == poAccepted {
				t.Errorf("%s: accepted without a trusted receipt", name)
			}
		}
	})

	t.Run("a supplied receipt object is not corroboration", func(t *testing.T) {
		tmp := filepath.Join(t.TempDir(), "o.json")
		raw, _ := os.ReadFile(filepath.Join(poFixtureDir, "obligations.json"))
		forged := strings.Replace(string(raw), `"reviewRef": "DR-synth-review-a"`,
			`"reviewRef": "DR-synth-review-a", "receipt": {"trusted": true}`, 1)
		if err := os.WriteFile(tmp, []byte(forged), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readObligationInput(tmp); err == nil {
			t.Errorf("an input carrying a receipt object must be refused, not silently ignored")
		}
	})

	t.Run("a stale decision after a new mapping revision", func(t *testing.T) {
		in, ctx := poInput(t), poCtx(t)
		rev2 := in.Mappings[0]
		rev2.Revision, rev2.Supersedes = 2, "MAP-synth-001@1"
		rev2.Paraphrase = "Synthetic: revised wording."
		in.Mappings = append(in.Mappings, rev2)
		res := resolveObligations(in, ctx, poAsOf)
		if v := poVerdictOf(t, res, "MAP-synth-001@1"); v.State != poSuperseded {
			t.Errorf("revision 1 stays in the record as superseded history, got %+v", v)
		}
		v := poVerdictOf(t, res, "MAP-synth-001@2")
		if v.State != poUnresolved || !hasReason(v, "no-current-decision") || len(v.History) == 0 {
			t.Errorf("the decision on revision 1 must not carry to revision 2, got %+v", v)
		}
		if p := validateObligationInput(in); len(p) != 0 {
			t.Errorf("a correct supersession chain validates clean: %v", p)
		}
	})

	t.Run("conflicting and superseded decisions", func(t *testing.T) {
		in, ctx := poInput(t), poCtx(t)
		second := in.Decisions[0]
		second.ID, second.Outcome = "APD-synth-003", "not-applicable"
		in.Decisions = append(in.Decisions, second)
		if v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1"); v.State != poConflict {
			t.Errorf("two live decisions must conflict, not pick one: %+v", v)
		}
		// An UNACCEPTED proposal naming a predecessor retires nothing: the accepted
		// decision stays current and the mapping is in conflict, never resolved by the
		// input's own pointer.
		in.Decisions[2].Supersedes = "APD-synth-001"
		in.Decisions[2].Acceptance = nil
		v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poConflict || len(v.History) != 0 {
			t.Errorf("an unaccepted superseder must not retire the accepted decision: %+v", v)
		}
	})

	t.Run("supersession binds to the accepted decision", func(t *testing.T) {
		in, root := poSupInput(t)
		ctx := poCtxAt(t, root, poGatesC())
		res := resolveObligations(in, ctx, poAsOf)
		v := poVerdictOf(t, res, "MAP-synth-001@1")
		if v.State != poNotApplicable || v.DecisionID != "APD-synth-003" || !containsAll(v.History, "APD-synth-001 superseded") {
			t.Fatalf("an accepted superseder retires the earlier decision: %+v", v)
		}
		if !res.Complete() {
			t.Fatalf("positive control: the superseded set is complete, problems %v", res.Problems)
		}
		// Reverse the pointer in the input only (no record, receipt or digest change):
		// both decisions' subjects move, so neither is accepted and nothing is retired.
		rev := poCopyInput(in)
		rev.Decisions[0].Supersedes, rev.Decisions[2].Supersedes = "APD-synth-003", ""
		res = resolveObligations(rev, ctx, poAsOf)
		v = poVerdictOf(t, res, "MAP-synth-001@1")
		if v.State == poAccepted || v.State == poNotApplicable || res.Complete() {
			t.Errorf("a reversed supersedes pointer must not resolve the mapping: %+v", v)
		}
		// Dropping the pointer alone is also a different subject.
		drop := poCopyInput(in)
		drop.Decisions[2].Supersedes = ""
		if v := poVerdictOf(t, resolveObligations(drop, ctx, poAsOf), "MAP-synth-001@1"); v.State != poConflict {
			t.Errorf("a dropped supersedes pointer leaves two current decisions: %+v", v)
		}
	})

	t.Run("reason and decision time are part of the subject", func(t *testing.T) {
		for name, edit := range map[string]func(d *poDecision){
			"reason rewritten":    func(d *poDecision) { d.Reason = "A different recorded reason." },
			"decidedAt rewritten": func(d *poDecision) { d.DecidedAt = "2026-10-04" },
			"supersedes added":    func(d *poDecision) { d.Supersedes = "APD-synth-009" },
		} {
			in, ctx := poInput(t), poCtx(t)
			edit(&in.Decisions[1])
			v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-002@1")
			if v.State != poRejected || !hasReason(v, "subject-digest-mismatch") {
				t.Errorf("%s: an approved not-applicable decision edited after approval must be refused, got %+v", name, v)
			}
		}
	})

	t.Run("lineage: a dropped prior revision is a problem", func(t *testing.T) {
		in := poInput(t)
		in.Mappings[0].Revision = 2 // revision 1 absent
		in.Decisions[0].MappingRevision = 2
		if !containsAll(validateObligationInput(in), "a prior revision was dropped or skipped") {
			t.Errorf("revisions must run 1..n")
		}
	})

	t.Run("loader path", func(t *testing.T) {
		path := filepath.Join(poFixtureDir, "obligations.json")
		res, err := loadObligations(poTree(), path, poAsOf, poGates())
		if err != nil || !res.Complete() {
			t.Fatalf("fixture should load complete: err=%v problems=%v", err, res.Problems)
		}
		res, err = loadObligations(poTree(), path, poAsOf, nil)
		if err != nil || res.Complete() || len(res.Accepted()) != 0 {
			t.Errorf("with no corroboration state the loader must leave applicability unresolved")
		}
		// The loader runs the candidate validator: stored text on a source whose use
		// denies storage is refused there (spec section 9's first MUST), even though
		// the boundary's verdicts are unaffected by it.
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		srcs, _ := doc["sources"].([]any)
		lic, _ := srcs[len(srcs)-1].(map[string]any)
		if use, _ := lic["use"].(map[string]any); use == nil || use["storage"] != "denied" {
			t.Fatal("fixture: the last source must deny storage")
		}
		lic["text"] = "stored anyway"
		stored, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		tmp := filepath.Join(t.TempDir(), "o.json")
		if err := os.WriteFile(tmp, stored, 0o600); err != nil {
			t.Fatal(err)
		}
		res, err = loadObligations(poTree(), tmp, poAsOf, poGates())
		if err != nil || res.Complete() || !containsAll(res.Problems, "keep licensed text out of the record") {
			t.Errorf("the loader must report the validator's stored-text refusal: err=%v problems=%v", err, res.Problems)
		}
		if _, err := loadObligations(poTree(), "https://example.invalid/o.json", poAsOf, nil); err == nil {
			t.Errorf("the loader must refuse a remote source")
		}
	})
}

// ---------- A3: REQ dereference through the production parser ----------

func TestAssuranceReqDereference(t *testing.T) {
	t.Run("a real fixture REQ resolves", func(t *testing.T) {
		ctx := poCtx(t)
		entries, err := parseRequirementsDir(poTree())
		if err != nil || len(entries) != 1 || len(ctx.Reqs) != 1 {
			t.Fatalf("production parser should see the one fixture REQ: %v %d", err, len(entries))
		}
		v := poVerdictOf(t, resolveObligations(poInput(t), ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poAccepted {
			t.Errorf("a mapping to a real REQ should resolve, got %+v", v)
		}
	})

	t.Run("a dangling REQ fails and cannot complete", func(t *testing.T) {
		in := poInput(t)
		in.Mappings[0].Reqs = []string{"REQ-synthetic-controls", "REQ-no-such-requirement"}
		res := resolveObligations(in, poRebind(t, &in), poAsOf)
		v := poVerdictOf(t, res, "MAP-synth-001@1")
		if v.State != poRejected || !hasReason(v, "req-unknown") {
			t.Errorf("want rejected/req-unknown, got %+v", v)
		}
		if res.Complete() || len(res.Accepted()) != 0 {
			t.Errorf("no complete mapping may exist over a dangling REQ")
		}
	})

	t.Run("a withdrawn REQ is refused", func(t *testing.T) {
		root := t.TempDir()
		copyTree(t, poTree(), root)
		p := filepath.Join(root, "docs/streams/requirements/REQ-synthetic-controls.md")
		raw, _ := os.ReadFile(p)
		os.WriteFile(p, []byte(strings.Replace(string(raw), `status: "accepted"`, `status: "withdrawn"`, 1)), 0o600)
		ctx, err := loadObligationCtx(root, poGates())
		if err != nil {
			t.Fatal(err)
		}
		v := poVerdictOf(t, resolveObligations(poInput(t), ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poRejected || !hasReason(v, "req-withdrawn") {
			t.Errorf("want rejected/req-withdrawn, got %+v", v)
		}
	})

	t.Run("an absent register dereferences nothing", func(t *testing.T) {
		ctx, err := loadObligationCtx(t.TempDir(), nil)
		if err != nil {
			t.Fatal(err)
		}
		v := poVerdictOf(t, resolveObligations(poInput(t), ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poRejected {
			t.Errorf("an empty register cannot satisfy a REQ reference: %+v", v)
		}
	})

	t.Run("an unreadable register is an error not an empty one", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "docs", "streams"), 0o755); err != nil {
			t.Fatal(err)
		}
		// a regular file where the directory belongs: not "absent", not readable
		if err := os.WriteFile(filepath.Join(root, "docs", "streams", requirementsDirName), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadObligationCtx(root, nil); err == nil {
			t.Errorf("an unreadable requirements register must be could-not-check, not empty")
		}
	})

	t.Run("a cross-repo REQ is could-not-check, not accepted", func(t *testing.T) {
		in := poInput(t)
		in.Mappings[0].Reqs = []string{"other:REQ-synthetic-controls"}
		v := poVerdictOf(t, resolveObligations(in, poRebind(t, &in), poAsOf), "MAP-synth-001@1")
		if v.State == poAccepted || !hasReason(v, "req-cross-repo-unchecked") {
			t.Errorf("a ref the offline loader cannot read must hold the mapping: %+v", v)
		}
	})

	t.Run("a malformed REQ ref is refused by the shape validator", func(t *testing.T) {
		in := poInput(t)
		in.Mappings[0].Reqs = []string{"REQ-1"}
		if !containsAll(validateObligationInput(in), "is not a requirement reference") {
			t.Errorf("REQ-1 is not the slug form")
		}
	})

	t.Run("a malformed REQ ref is refused at the boundary", func(t *testing.T) {
		in := poInput(t)
		in.Mappings[0].Reqs = []string{"REQ-1"}
		v := poVerdictOf(t, resolveObligations(in, poCtx(t), poAsOf), "MAP-synth-001@1")
		if v.State != poRejected || !hasReason(v, "req-malformed") {
			t.Errorf("the boundary refuses a malformed REQ ref without the validator: %+v", v)
		}
	})

	t.Run("an unreadable decisions register is an error", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "docs", "streams"), 0o755); err != nil {
			t.Fatal(err)
		}
		// requirements absent (legitimately empty); a regular file where the
		// decisions directory belongs: not absent, not readable.
		if err := os.WriteFile(filepath.Join(root, "docs", "streams", decisionsDirName), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := loadObligationCtx(root, nil)
		if err == nil || !strings.Contains(err.Error(), "decisions register unreadable") {
			t.Errorf("an unreadable decisions register must be could-not-check, not empty: %v", err)
		}
	})

	t.Run("legacy requirements lint is unchanged", func(t *testing.T) {
		if p := requirementRegisterProblems(poTree()); len(p) != 0 {
			t.Errorf("the fixture REQ must lint clean: %v", p)
		}
	})
}

// ---------- digests: independent expected values ----------

func TestAssuranceSubjectDigest(t *testing.T) {
	in := poInput(t)
	// Expected values come from testdata/projectobligations/canonical_digest.py, an
	// independent implementation of spec section 3 written from the spec text (run by
	// hand, not by CI), never from this package.
	wantMap := "sha256:e719e1317c544348791c5e91a15e45d1c8cca9b789ffdbbd15cd6a7e21e4dce6"
	wantSubj := "sha256:d8a2ae0bdef090bab514d892330836b6b24792649b3d3ae80e88b1b1f28179e1"
	if got := mappingDigest(in.Mappings[0]); got != wantMap {
		t.Errorf("mapping digest = %s, want %s", got, wantMap)
	}
	if got := subjectDigest(in.Sources[0], in.Mappings[0], in.Decisions[0]); got != wantSubj {
		t.Errorf("subject digest = %s, want %s", got, wantSubj)
	}
	// The fixture decision has no end date, so the golden above cannot see an
	// effectiveTo dropped from the subject. Same helper, same fixture, with
	// decisions[0].effectiveTo = "2027-09-30".
	bounded := in.Decisions[0]
	bounded.EffectiveTo = "2027-09-30"
	wantBounded := "sha256:e043cc6eda86914dccd4cc5b6993e245eb24993765e284b2fd0667288df308a9"
	if got := subjectDigest(in.Sources[0], in.Mappings[0], bounded); got != wantBounded {
		t.Errorf("subject digest with effectiveTo = %s, want %s", got, wantBounded)
	}
	swapped := in.Mappings[0]
	swapped.Reqs = []string{"REQ-synthetic-controls", "REQ-aaaa-aaaa-aaaa"}
	a := mappingDigest(swapped)
	swapped.Reqs = []string{"REQ-aaaa-aaaa-aaaa", "REQ-synthetic-controls"}
	if a != mappingDigest(swapped) {
		t.Errorf("REQ order must not change a mapping's identity")
	}
}

// TestAssuranceDigestInjective: two different mappings never share a digest, even
// when a field holds the characters an unescaped join would split on.
func TestAssuranceDigestInjective(t *testing.T) {
	base := poInput(t).Mappings[0]
	pairs := map[string][2]func(m *poMapping){
		"newline moves clause/paraphrase boundary": {
			func(m *poMapping) { m.Clause, m.Paraphrase = "4.1", "first line\nsecond line" },
			func(m *poMapping) { m.Clause, m.Paraphrase = "4.1\nfirst line", "second line" },
		},
		"comma inside one control": {
			func(m *poMapping) { m.Controls = []string{"CTRL-a,CTRL-b"} },
			func(m *poMapping) { m.Controls = []string{"CTRL-a", "CTRL-b"} },
		},
		"context fields re-split": {
			func(m *poMapping) { m.Context = poContext{Entity: "a\nb", Activity: "c"} },
			func(m *poMapping) { m.Context = poContext{Entity: "a", Activity: "b\nc"} },
		},
		"at-sign inside id and revision": {
			func(m *poMapping) { m.Source = poRef{ID: "SRC-a@1", Revision: "2"} },
			func(m *poMapping) { m.Source = poRef{ID: "SRC-a", Revision: "1@2"} },
		},
	}
	for name, p := range pairs {
		x, y := base, base
		p[0](&x)
		p[1](&y)
		if mappingDigest(x) == mappingDigest(y) {
			t.Errorf("%s: two different mappings share a digest", name)
		}
	}

	// End to end: an approval recorded for one mapping is not accepted for its
	// re-split twin.
	in := poInput(t)
	in.Mappings[0].Clause, in.Mappings[0].Paraphrase = "4.1", "first line\nsecond line"
	ctx := poRebind(t, &in)
	if v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1"); v.State != poAccepted {
		t.Fatalf("positive control: the bound mapping is accepted, got %+v", v)
	}
	in.Mappings[0].Clause, in.Mappings[0].Paraphrase = "4.1\nfirst line", "second line"
	if v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1"); v.State != poRejected || !hasReason(v, "subject-digest-mismatch") {
		t.Errorf("an approval must not transfer to a re-split mapping, got %+v", v)
	}
}

// ---------- F3: the receipt is the seam's corroborating issue ----------

// TestAssuranceOneClosedByOwner: when a record links two issues and only the
// second is closed by the blessed login, the receipt names the second. A decision
// naming the non-blessed closer and its issue is never accepted.
func TestAssuranceOneClosedByOwner(t *testing.T) {
	gates := decisionGateLinks{
		poDRa: {
			{Ref: "example-org/tracker#77", ClosedBy: "mallory", Body: decisionGateMarker("DR-synth-applic-a")},
			{Ref: "example-org/tracker#79", ClosedBy: "ada", Body: decisionGateMarker("DR-synth-applic-a")},
		},
		poDRb: poGates()[poDRb],
	}
	in := poInput(t)
	in.Decisions[0].Acceptance.CorroborationRef = "example-org/tracker#79"
	ctx := poCtxAt(t, poTree(), gates)
	if v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1"); v.State != poAccepted {
		t.Errorf("the decision corroborated by the blessed closer must be accepted, got %+v", v)
	}
	forged := poInput(t)
	forged.Decisions[0].Reviewer = "mallory"
	forged.Decisions[0].Acceptance.CorroborationRef = "example-org/tracker#77"
	ctx = poRebindAt(t, &forged, 0, poDRa, gates)
	if v := poVerdictOf(t, resolveObligations(forged, ctx, poAsOf), "MAP-synth-001@1"); v.State == poAccepted {
		t.Errorf("a decision naming the non-blessed closer must not be accepted, got %+v", v)
	}
}

// TestAssuranceGateSeamOnly is the class guard for F3: outside
// decisiongateanchor.go, no production function may index, range over or
// otherwise read a decisionGateLinks parameter; it may only pass it on. Selecting
// the corroborating issue is the seam's job alone.
func TestAssuranceGateSeamOnly(t *testing.T) {
	// positive control: the matcher flags a planted second selection loop and
	// passes a pure pass-through.
	planted := `package main
func bad(gates decisionGateLinks) { for _, iss := range gates["f"] { _ = iss } }
func alsoBad(gates decisionGateLinks) { g := gates; _ = g }
func ok(gates decisionGateLinks) { use(gates) }`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "planted.go", planted, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := poGateReads(fset, f); len(got) != 2 || !strings.Contains(got[0], "bad") || !strings.Contains(got[1], "alsoBad") {
		t.Fatalf("matcher control: want bad and alsoBad flagged, got %v", got)
	}

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "decisiongateanchor.go" {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range poGateReads(fset, f) {
			t.Errorf("%s: reads decision-gate state directly; ask decisionGateCorroboratingIssue instead", r)
		}
	}
}

// poGateReads lists every use of a decisionGateLinks parameter that is not a
// direct call argument.
func poGateReads(fset *token.FileSet, f *ast.File) []string {
	var out []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		params := map[string]bool{}
		for _, fld := range fn.Type.Params.List {
			if id, ok := fld.Type.(*ast.Ident); ok && id.Name == "decisionGateLinks" {
				for _, n := range fld.Names {
					params[n.Name] = true
				}
			}
		}
		if len(params) == 0 {
			continue
		}
		args := map[*ast.Ident]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				for _, a := range c.Args {
					if id, ok := a.(*ast.Ident); ok {
						args[id] = true
					}
				}
			}
			return true
		})
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && params[id.Name] && !args[id] {
				out = append(out, fmt.Sprintf("%s %s", fset.Position(id.Pos()), fn.Name.Name))
			}
			return true
		})
	}
	return out
}

// ---------- S1: digest and receipt come from ONE file-bound record ----------

// poShadowDR writes a second decisions file, name, whose frontmatter claims id and
// whose body carries digest. The real record is left untouched.
func poShadowDR(t *testing.T, root, name, id, digest string) {
	t.Helper()
	body := "---\nid: \"" + id + "\"\ndate: \"2026-10-07\"\ntitle: \"Synthetic: shadow\"\n" +
		"consequence: \"major\"\ndecided-by: \"human:<name>\"\nalternatives:\n  - \"None.\"\n" +
		"accepted:\n  - \"None.\"\n---\n\nShadow record: " + digest + "\n"
	if err := os.WriteFile(filepath.Join(root, "docs/streams/decisions", name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// poFlipped is the fixture with APD-synth-001 flipped to not-applicable and its
// input digest recomputed. No decision record carries the new digest.
func poFlipped(t *testing.T) obligationInput {
	t.Helper()
	in := poInput(t)
	d := &in.Decisions[0]
	d.Outcome, d.Reason = "not-applicable", "Synthetic: never approved."
	acc := *d.Acceptance
	acc.SubjectDigest = poSubjectOf(t, in, *d)
	d.Acceptance = &acc
	return in
}

// TestAssuranceRecordIdentity: the record whose body must carry the subject digest
// is the same file the corroboration receipt was minted from. A second file that
// claims a corroborated record's id never supplies the digest, whatever its name
// or where it sorts; the id is held as ambiguous instead of one file being picked.
func TestAssuranceRecordIdentity(t *testing.T) {
	for _, name := range []string{"notes.md", "DR-synth-zzzzz-x.md", "DR-synth-aaaaa-x.md"} {
		t.Run("shadow file "+name, func(t *testing.T) {
			in := poFlipped(t)
			root := t.TempDir()
			copyTree(t, poTree(), root)
			poShadowDR(t, root, name, "DR-synth-applic-a", in.Decisions[0].Acceptance.SubjectDigest)
			v := poVerdictOf(t, resolveObligations(in, poCtxAt(t, root, poGates()), poAsOf), "MAP-synth-001@1")
			if v.State != poUnresolved || !hasReason(v, "decision-record-ambiguous") {
				t.Errorf("a second file claiming the id must hold the decision, never supply its digest: %+v", v)
			}
		})
	}

	t.Run("approved subject held while its id is ambiguous", func(t *testing.T) {
		root := t.TempDir()
		copyTree(t, poTree(), root)
		poShadowDR(t, root, "notes.md", "DR-synth-applic-a", "no digest here")
		v := poVerdictOf(t, resolveObligations(poInput(t), poCtxAt(t, root, poGates()), poAsOf), "MAP-synth-001@1")
		if v.State != poUnresolved || !hasReason(v, "decision-record-ambiguous") {
			t.Errorf("an ambiguous record id is held, not resolved by picking a file: %+v", v)
		}
	})

	t.Run("boundary refuses a body from another file", func(t *testing.T) {
		// A hand-built context bypasses the loader's admission rule: the receipt is
		// the real record's, the body comes from a different file claiming its id.
		in := poFlipped(t)
		ctx := poCtx(t)
		rec := ctx.Decisions["DR-synth-applic-a"]
		rec.File, rec.Body = "notes.md", "Shadow record: "+in.Decisions[0].Acceptance.SubjectDigest
		ctx.Decisions["DR-synth-applic-a"] = rec
		v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poRejected || !hasReason(v, "decision-record-not-file-bound") {
			t.Errorf("a body from a file that is not the record's own must be refused: %+v", v)
		}
	})

	t.Run("boundary refuses another record filed under the id", func(t *testing.T) {
		// DR-synth-applic-b is its own file, but it is not DR-synth-applic-a.
		in := poFlipped(t)
		ctx := poCtx(t)
		rec := ctx.Decisions["DR-synth-applic-b"]
		rec.Body = "Other record: " + in.Decisions[0].Acceptance.SubjectDigest
		ctx.Decisions["DR-synth-applic-a"] = rec
		v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poRejected || !hasReason(v, "decision-record-not-file-bound") {
			t.Errorf("a record filed under another id must be refused: %+v", v)
		}
	})

	t.Run("boundary refuses a receipt minted from another file", func(t *testing.T) {
		in, ctx := poInput(t), poCtx(t)
		rc := ctx.Receipts["DR-synth-applic-a"]
		rc.file = "DR-synth-other.md"
		ctx.Receipts["DR-synth-applic-a"] = rc
		v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poRejected || !hasReason(v, "receipt-record-mismatch") {
			t.Errorf("a receipt from another file must not corroborate this record: %+v", v)
		}
	})

	t.Run("an ambiguous review reference is held", func(t *testing.T) {
		root := t.TempDir()
		copyTree(t, poTree(), root)
		poShadowDR(t, root, "notes.md", "DR-synth-review-a", "review shadow")
		v := poVerdictOf(t, resolveObligations(poInput(t), poCtxAt(t, root, poGates()), poAsOf), "MAP-synth-001@1")
		if v.State != poUnresolved || !hasReason(v, "review-ref-ambiguous") {
			t.Errorf("a review id two files claim is held, not resolved: %+v", v)
		}
	})

	t.Run("admission is order independent", func(t *testing.T) {
		a := decisionEntry{ID: "DR-synth-xone", File: "DR-synth-xone.md"}
		b := decisionEntry{ID: "DR-synth-xone", File: "aa.md"}
		for _, decs := range [][]decisionEntry{{a, b}, {b, a}} {
			adm, amb := admitDecisionRecords(decs)
			if _, ok := adm["DR-synth-xone"]; ok || len(amb["DR-synth-xone"]) != 2 {
				t.Errorf("order %v: want held as ambiguous, got admitted=%v ambiguous=%v", decs, adm, amb)
			}
		}
		adm, amb := admitDecisionRecords([]decisionEntry{a, {ID: "DR-synth-xtwo", File: "notes.md"}})
		if _, ok := adm["DR-synth-xone"]; !ok || len(amb) != 0 {
			t.Errorf("control: a lone file-bound record is admitted, got %v %v", adm, amb)
		}
		if _, ok := adm["DR-synth-xtwo"]; ok {
			t.Errorf("a record that is not its own file is never admitted")
		}
	})
}

// ---------- F6: one test per refusal path the mutation spec disarms ----------

func TestAssuranceRefusals(t *testing.T) {
	// boundary: each edit is re-bound so only the guard under test is wrong.
	boundary := []struct {
		name, reason string
		i            int
		edit         func(d *poDecision)
	}{
		{"decision dated after as-of", "decision-in-future", 0, func(d *poDecision) { d.DecidedAt = "2026-10-09" }},
		{"not-applicable without reason", "not-applicable-needs-reason", 1, func(d *poDecision) { d.Reason = " " }},
		{"unknown outcome", "unknown-outcome", 0, func(d *poDecision) { d.Outcome = "maybe" }},
		{"malformed date", "bad-date", 0, func(d *poDecision) { d.DecidedAt = "2026-13-45" }},
		{"empty scope", "scope-missing", 0, func(d *poDecision) { d.Scope = " " }},
		{"missing proposer", "actor-missing", 0, func(d *poDecision) { d.ProposedBy = "" }},
	}
	for _, c := range boundary {
		t.Run(c.name, func(t *testing.T) {
			in := poInput(t)
			c.edit(&in.Decisions[c.i])
			dr := []string{poDRa, poDRb}[c.i]
			ctx := poRebindAt(t, &in, c.i, dr, poGates())
			mp := in.Decisions[c.i].MappingID + "@1"
			v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), mp)
			if v.State != poRejected || !hasReason(v, c.reason) {
				t.Errorf("want rejected/%s, got %+v", c.reason, v)
			}
		})
	}

	t.Run("unknown source rejects the mapping", func(t *testing.T) {
		in, ctx := poInput(t), poCtx(t)
		in.Sources = in.Sources[1:]
		v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poRejected || !hasReason(v, "source-unknown") {
			t.Errorf("want rejected/source-unknown, got %+v", v)
		}
	})

	t.Run("duplicate source revision is a boundary problem", func(t *testing.T) {
		in, ctx := poInput(t), poCtx(t)
		dup := in.Sources[0]
		dup.ContentDigest = poSum("other")
		in.Sources = append(in.Sources, dup)
		res := resolveObligations(in, ctx, poAsOf)
		if res.Complete() || !containsAll(res.Problems, "duplicate source revision") {
			t.Errorf("a duplicate source revision must hold the set, problems %v", res.Problems)
		}
	})

	t.Run("record whose id is not its file name gets no receipt", func(t *testing.T) {
		root := t.TempDir()
		copyTree(t, poTree(), root)
		old := filepath.Join(root, poDRa)
		moved := "docs/streams/decisions/DR-synth-other.md"
		if err := os.Rename(old, filepath.Join(root, moved)); err != nil {
			t.Fatal(err)
		}
		gates := poGates()
		gates[moved] = []decisionGateIssue{{Ref: "example-org/tracker#77", ClosedBy: "ada", Body: decisionGateMarker("DR-synth-other")}}
		ctx := poCtxAt(t, root, gates)
		v := poVerdictOf(t, resolveObligations(poInput(t), ctx, poAsOf), "MAP-synth-001@1")
		if v.State == poAccepted || !hasReason(v, "no-trusted-corroboration") || !hasReason(v, "decision-record-unknown") {
			t.Errorf("a record that is not its own file is not admitted and gets no receipt, got %+v", v)
		}
	})

	t.Run("complete needs no candidate problems", func(t *testing.T) {
		r := obligationResult{Verdicts: []poVerdict{{Mapping: "MAP-x@1", State: poAccepted}}}
		if !r.Complete() {
			t.Fatalf("positive control: one accepted mapping is complete")
		}
		r.Problems = []string{"schema mismatch"}
		if r.Complete() {
			t.Errorf("a candidate problem must hold completeness")
		}
	})

	t.Run("cross-repo hold kept with no current decision", func(t *testing.T) {
		in, ctx := poInput(t), poCtx(t)
		in.Mappings[0].Reqs = []string{"other:REQ-synthetic-controls"}
		in.Decisions = in.Decisions[1:]
		v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poUnresolved || !hasReason(v, "no-current-decision") || !hasReason(v, "req-cross-repo-unchecked") {
			t.Errorf("want unresolved with both reasons, got %+v", v)
		}
	})

	t.Run("a scheme-shaped path is refused before reading", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("a ':' directory name is not portable")
		}
		dir := filepath.Join(t.TempDir(), "a:")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(poFixtureDir, "obligations.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "b.json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := readObligationInput(dir + "/b.json"); err != nil {
			t.Fatalf("control: the plain path reads: %v", err)
		}
		// dir+"//b.json" contains "://" yet names the same readable file.
		if _, err := readObligationInput(dir + "//b.json"); err == nil || !strings.Contains(err.Error(), "only a local") {
			t.Errorf("a path containing :// must be refused by the guard, got %v", err)
		}
	})

	validator := []struct {
		name, want string
		edit       func(in *obligationInput)
	}{
		{"paraphrase over the bound", "paraphrase exceeds", func(in *obligationInput) {
			in.Mappings[0].Paraphrase = strings.Repeat("x", poMaxParaphrase+1)
		}},
		{"mapping supersedes chain", "supersedes", func(in *obligationInput) {
			in.Mappings[0].Supersedes = "MAP-synth-001@0"
		}},
		{"source predecessor form", "must be <same id>@<revision>", func(in *obligationInput) {
			in.Sources[0].Predecessor = in.Sources[1].key()
		}},
		{"not-applicable reason", "not-applicable needs a reason", func(in *obligationInput) {
			in.Decisions[1].Reason = ""
		}},
		{"decision on an unsupplied mapping", "is not a supplied mapping revision", func(in *obligationInput) {
			in.Decisions[0].MappingRevision = 7
		}},
	}
	for _, c := range validator {
		t.Run("validator: "+c.name, func(t *testing.T) {
			in := poInput(t)
			c.edit(&in)
			if !containsAll(validateObligationInput(in), c.want) {
				t.Errorf("want a problem containing %q, got %v", c.want, validateObligationInput(in))
			}
		})
	}

	t.Run("validator: paraphrase at the bound is clean", func(t *testing.T) {
		in := poInput(t)
		in.Mappings[0].Paraphrase = strings.Repeat("x", poMaxParaphrase)
		if p := validateObligationInput(in); len(p) != 0 {
			t.Errorf("a paraphrase of exactly the bound is allowed: %v", p)
		}
	})
}

// TestAssuranceReasonsMutated keeps spec section 9's coverage sentence checkable:
// every reason token the authority boundary can return is named, in [brackets], by
// at least one entry of the committed mutation spec. A new reason with no mutation
// fails here.
func TestAssuranceReasonsMutated(t *testing.T) {
	src, err := os.ReadFile("projectobligations.go")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("projectobligations-mutations.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Mutations []struct{ Name string } `json:"mutations"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	named := map[string]bool{}
	tag := regexp.MustCompile(`\[([a-z:-]+)\]`)
	for _, m := range spec.Mutations {
		for _, g := range tag.FindAllStringSubmatch(m.Name, -1) {
			named[g[1]] = true
		}
	}
	reasons := poReasonTokens(string(src))
	// positive control: the extractor sees reasons from each emitting form.
	for _, want := range []string{"no-acceptance-link", "self-approval", "req-malformed", "conflicting-decisions", "no-current-decision"} {
		if !reasons[want] {
			t.Fatalf("extractor control: %q not found in %v", want, reasons)
		}
	}
	for r := range reasons {
		if !named[r] {
			t.Errorf("reason %q has no [%s] entry in projectobligations-mutations.json", r, r)
		}
	}
}

// poReasonTokens extracts the reason tokens the boundary emits: add("x"),
// []string{"x"}, and the "x:"+ prefixed forms.
func poReasonTokens(src string) map[string]bool {
	out := map[string]bool{}
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`add\("([a-z]+(?:-[a-z]+)+)"\)`),
		regexp.MustCompile(`\[\]string\{"([a-z]+(?:-[a-z]+)+)"`),
		regexp.MustCompile(`"([a-z]+(?:-[a-z]+)+):"\s*\+`),
	} {
		for _, g := range re.FindAllStringSubmatch(src, -1) {
			out[g[1]] = true
		}
	}
	return out
}

// copyTree copies a small fixture tree (files only, no symlinks).
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.Walk(from, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		dst := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, raw, 0o600)
	})
	if err != nil {
		t.Fatal(err)
	}
}
