package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for the versioned source-obligation and applicability contract
// (projectobligations.go; spec/project-obligations-v1.md). The fixtures under
// testdata/projectobligations are synthetic: no licensed text, no adopter record.
// The expected digests in the fixture and in the golden test were computed by an
// independent implementation (a separate script over the canonical subject text in
// the spec), never by the code under test.
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
	root := t.TempDir()
	copyTree(t, poTree(), root)
	d := &in.Decisions[0]
	old := d.Acceptance.SubjectDigest
	d.Acceptance.SubjectDigest = subjectDigest(in.Sources[0], in.Mappings[0], *d)
	p := filepath.Join(root, poDRa)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(strings.Replace(string(raw), old, d.Acceptance.SubjectDigest, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, err := loadObligationCtx(root, poGates())
	if err != nil {
		t.Fatal(err)
	}
	return ctx
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
		in.Decisions[2].Supersedes = "APD-synth-001"
		in.Decisions[2].Acceptance = nil
		v := poVerdictOf(t, resolveObligations(in, ctx, poAsOf), "MAP-synth-001@1")
		if v.State != poUnresolved || len(v.History) == 0 {
			t.Errorf("a superseded accepted decision is history, the new proposal is unresolved: %+v", v)
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

	t.Run("legacy requirements lint is unchanged", func(t *testing.T) {
		if p := requirementRegisterProblems(poTree()); len(p) != 0 {
			t.Errorf("the fixture REQ must lint clean: %v", p)
		}
	})
}

// ---------- digests: independent expected values ----------

func TestAssuranceSubjectDigest(t *testing.T) {
	in := poInput(t)
	// Expected values were computed by a separate script over the canonical text
	// documented in spec/project-obligations-v1.md, not by this package.
	wantMap := "sha256:235922bdf69914b106f062ace9c5081bcf7d69f5f61c1dd349ad63fa1bae6e56"
	wantSubj := "sha256:e58ed0d80cc74d6d2e7a4d82a1b855ca132a49fad089a709b19de353c9658fa2"
	if got := mappingDigest(in.Mappings[0]); got != wantMap {
		t.Errorf("mapping digest = %s, want %s", got, wantMap)
	}
	if got := subjectDigest(in.Sources[0], in.Mappings[0], in.Decisions[0]); got != wantSubj {
		t.Errorf("subject digest = %s, want %s", got, wantSubj)
	}
	swapped := in.Mappings[0]
	swapped.Reqs = []string{"REQ-synthetic-controls", "REQ-aaaa-aaaa-aaaa"}
	a := mappingDigest(swapped)
	swapped.Reqs = []string{"REQ-aaaa-aaaa-aaaa", "REQ-synthetic-controls"}
	if a != mappingDigest(swapped) {
		t.Errorf("REQ order must not change a mapping's identity")
	}
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
