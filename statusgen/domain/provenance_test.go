package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/internal/testschema"
)

func provFixture(t *testing.T, name string) Provenance {
	t.Helper()
	b, e := os.ReadFile("../testdata/domain/provenance/" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	var p Provenance
	if e = json.Unmarshal(b, &p); e != nil {
		t.Fatal(e)
	}
	return p
}

// currentAssessment runs the real /28 flow so every duty check binds to a
// current assessment of candidate A, never a hand-built one.
func currentAssessment(t *testing.T) (Record, Assessment) {
	t.Helper()
	r := fixture(t, "valid/candidate-a")
	a, e := Assess(r, observations(r))
	if e != nil {
		t.Fatal(e)
	}
	return r, a
}

func review(min string) DutyPolicy {
	return DutyPolicy{Duty: "review", Against: []string{"author"}, MinProof: min}
}

func evalDuty(t *testing.T, policy DutyPolicy, names ...string) DutyResult {
	t.Helper()
	r, a := currentAssessment(t)
	var evidence []Provenance
	for _, n := range names {
		evidence = append(evidence, provFixture(t, n))
	}
	out, e := EvaluateDuty(r, a, policy, evidence)
	if e != nil {
		t.Fatal(e)
	}
	return out
}

func wantDuty(t *testing.T, got DutyResult, state string, reasons ...string) {
	t.Helper()
	if reasons == nil {
		reasons = []string{}
	}
	if got.State != state || !reflect.DeepEqual(got.Reasons, reasons) {
		t.Fatalf("got %s %v, want %s %v", got.State, got.Reasons, state, reasons)
	}
}

// regression: graph-execution/31
func TestDutyPrincipals(t *testing.T) {
	author := provFixture(t, "valid/author")
	cases := []struct {
		name, state string
		reasons     []string
	}{
		// Distinct principals behind one issuing infrastructure are independent.
		{"valid/review-independent", "satisfied", nil},
		// The reviewer agent's delegation includes the author's principal.
		{"valid/review-overlap", "unsatisfied", []string{"principal-overlap"}},
		// One principal, two keys: still one principal.
		{"valid/review-same-key", "unsatisfied", []string{"principal-overlap"}},
	}
	for _, c := range cases {
		t.Run(filepath.Base(c.name), func(t *testing.T) {
			rev := provFixture(t, c.name)
			if rev.Issuer != author.Issuer || rev.Key == author.Key {
				t.Fatal("fixture must share the issuer and differ in key")
			}
			wantDuty(t, evalDuty(t, review("attested"), "valid/author", c.name), c.state, c.reasons...)
		})
	}
}

// regression: graph-execution/31
func TestDutySponsor(t *testing.T) {
	author := provFixture(t, "valid/author")
	sponsor := provFixture(t, "valid/review-sponsor")
	if sponsor.Sponsor != author.Responsible {
		t.Fatal("fixture must make the author the reviewer's sponsor")
	}
	// Sponsorship is accountability, not delegation: no overlap.
	wantDuty(t, evalDuty(t, review("attested"), "valid/author", "valid/review-sponsor"), "satisfied")

	// The only human authors and also sits in the reviewer agents' delegation.
	sole := provFixture(t, "valid/review-sole-human")
	if author.Executor.Kind != "human" || sole.Executor.Kind == "human" || sole.Delegation[0] != author.Responsible {
		t.Fatal("fixture must model a sole human delegating the reviewer")
	}
	wantDuty(t, evalDuty(t, review("attested"), "valid/author", "valid/review-sole-human"), "unsatisfied", "principal-overlap")

	// An overlap is never softened by another, weaker hold.
	weak := review("signed")
	wantDuty(t, evalDuty(t, weak, "valid/author", "valid/review-sole-human"), "unsatisfied", "proof-uncorroborated", "principal-overlap")
}

// regression: graph-execution/31
func TestProofClass(t *testing.T) {
	author := DutyPolicy{Duty: "author", Against: []string{}, MinProof: "attested"}
	wantDuty(t, evalDuty(t, author, "valid/author"), "satisfied")

	// A declared signature that was only claimed proves attested, not signed.
	forged := provFixture(t, "valid/author-forged")
	if forged.DeclaredProof != "signed" || forged.EffectiveProof() != "attested" {
		t.Fatalf("effective proof %s", forged.EffectiveProof())
	}
	signed := DutyPolicy{Duty: "author", Against: []string{}, MinProof: "signed"}
	wantDuty(t, evalDuty(t, signed, "valid/author-forged"), "held", "proof-uncorroborated", "declared-proof-unsupported")
	// Even below the policy minimum, an inflated label is reported.
	wantDuty(t, evalDuty(t, author, "valid/author-forged"), "held", "declared-proof-unsupported")

	// Self-report with nothing corroborated (one dimension withheld).
	self := provFixture(t, "valid/author-self-report")
	if self.EffectiveProof() != "self-report" {
		t.Fatalf("effective proof %s", self.EffectiveProof())
	}
	auth := DutyPolicy{Duty: "author", Against: []string{}, MinProof: "authenticated"}
	wantDuty(t, evalDuty(t, auth, "valid/author-self-report"), "held", "proof-uncorroborated")

	// A provider App acting for a person is not a human act.
	human := DutyPolicy{Duty: "author", Against: []string{}, HumanAct: true, MinProof: "authenticated"}
	wantDuty(t, evalDuty(t, human, "valid/author"), "satisfied")
	wantDuty(t, evalDuty(t, human, "valid/author-bot-for-human"), "unsatisfied", "not-human-act")

	// Missing corroboration and missing evidence hold; they never pass.
	wantDuty(t, evalDuty(t, review("attested"), "valid/author", "valid/review-uncorroborated"), "held", "lineage-uncorroborated")
	wantDuty(t, evalDuty(t, review("attested"), "valid/author"), "held", "duty-evidence-missing")
}

// trustedSource is a synthetic trusted-composition selector. It answers only
// from records it was built with, keyed by subject, duty and evidence digest.
type trustedSource map[string]Provenance

func sourceKey(w WorkIdentity, c Candidate, duty, evidence string) string {
	return w.Attempt + "|" + c.Commit + "|" + duty + "|" + evidence
}
func (s trustedSource) Select(w WorkIdentity, c Candidate, duty, evidence string) (Provenance, bool) {
	p, ok := s[sourceKey(w, c, duty, evidence)]
	return p, ok
}
func newSource(items ...Provenance) trustedSource {
	s := trustedSource{}
	for _, p := range items {
		s[sourceKey(p.Work, p.Candidate, p.Duty, p.Evidence)] = p
	}
	return s
}

// regression: graph-execution/31
func TestDutyEvidenceFlow(t *testing.T) {
	r, a := currentAssessment(t)
	author, overlap := provFixture(t, "valid/author"), provFixture(t, "valid/review-overlap")
	other := provFixture(t, "valid/author-other-candidate")
	src := newSource(author, overlap, other)

	// Upper validation bypassed: the selector alone must reject a claim that
	// hides the author from the reviewer's lineage or raises its proof label.
	hidden := provFixture(t, "valid/review-overlap")
	hidden.Responsible, hidden.Delegation = "principal:bob", []string{}
	_, e := Corroborate(hidden, src)
	requireCode(t, e, "provenance-not-selected")
	raised := provFixture(t, "valid/author")
	raised.DeclaredProof = "custody-qualified"
	_, e = Corroborate(raised, src)
	requireCode(t, e, "provenance-not-selected")
	unknownItem := provFixture(t, "valid/review-independent")
	_, e = Corroborate(unknownItem, src)
	requireCode(t, e, "provenance-uncorroborated")
	_, e = Corroborate(author, nil)
	requireCode(t, e, "provenance-uncorroborated")

	// The selector's own records flow into the real duty check.
	var trusted []Provenance
	for _, claim := range []Provenance{author, overlap} {
		p, e := Corroborate(claim, src)
		if e != nil {
			t.Fatal(e)
		}
		trusted = append(trusted, p)
	}
	out, e := EvaluateDuty(r, a, review("attested"), trusted)
	if e != nil {
		t.Fatal(e)
	}
	wantDuty(t, out, "unsatisfied", "principal-overlap")

	// Corroborated evidence for another candidate is refused, not ignored.
	p, e := Corroborate(other, src)
	if e != nil {
		t.Fatal(e)
	}
	_, e = EvaluateDuty(r, a, review("attested"), []Provenance{p, overlap})
	requireCode(t, e, "provenance-subject")
	_, e = EvaluateDuty(r, a, review("attested"), []Provenance{author, author})
	requireCode(t, e, "duty-ambiguous")

	// A stale assessment cannot be reused for duty evaluation.
	stale := a
	stale.Generation++
	_, e = EvaluateDuty(r, stale, review("attested"), trusted)
	requireCode(t, e, "stale-generation")
}

func jsonFields(v any) []string {
	typ := reflect.TypeOf(v)
	var out []string
	for i := 0; i < typ.NumField(); i++ {
		out = append(out, typ.Field(i).Tag.Get("json"))
	}
	return out
}

func keys(node map[string]any, field string) []string {
	var out []string
	switch v := node[field].(type) {
	case map[string]any:
		for k := range v {
			out = append(out, k)
		}
	case []any:
		for _, k := range v {
			out = append(out, k.(string))
		}
	}
	return out
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

// regression: graph-execution/31
func TestProvenanceSchema(t *testing.T) {
	s := testschema.Load(t, "../../schemas/evidence-provenance-v1.json")
	defs := s.Root["$defs"].(map[string]any)
	spec, e := os.ReadFile("../../spec/evidence-provenance-v1.md")
	if e != nil {
		t.Fatal(e)
	}
	for name := range defs {
		if !strings.Contains(string(spec), "`"+name+"`") {
			t.Errorf("spec lacks %s", name)
		}
	}
	// Work and candidate are /28's definitions, not a second identity scheme.
	base := testschema.Load(t, "../../schemas/verification-domain-v1.json").Root["$defs"].(map[string]any)
	for _, name := range []string{"work-identity", "candidate"} {
		if !reflect.DeepEqual(defs[name], base[name]) {
			t.Fatalf("%s drifts from verification-domain-v1", name)
		}
	}
	if defs["principal"].(map[string]any)["pattern"] != principalRE.String() {
		t.Fatal("principal grammar drift")
	}
	// Every Go field is a schema property and every property is mandatory.
	types := map[string]any{"": Provenance{}, "actor": Actor{}, "contract-binding": ContractBinding{}, "capture": Capture{}, "environment": Environment{}, "corroboration": Corroboration{}}
	for name, v := range types {
		node := s.Root
		if name != "" {
			node = defs[name].(map[string]any)
		}
		fields := jsonFields(v)
		if !sameSet(fields, keys(node, "properties")) || !sameSet(fields, keys(node, "required")) {
			t.Fatalf("%s schema fields drift", name)
		}
	}
	for name, want := range map[string][]string{"actor-kind": actorKinds, "proof-class": proofClasses, "corroboration-state": corroborationStates, "duty-state": dutyStates} {
		if got := keys(defs[name].(map[string]any), "enum"); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s vocabulary drift: %v", name, got)
		}
	}
	files, e := filepath.Glob("../testdata/domain/provenance/*/*.json")
	if e != nil || len(files) < 20 {
		t.Fatalf("fixtures not found: %v", e)
	}
	seen := map[string]bool{}
	for _, file := range files {
		kind := filepath.Base(filepath.Dir(file))
		seen[kind+"/"+strings.TrimSuffix(filepath.Base(file), ".json")] = true
		t.Run(kind+"/"+filepath.Base(file), func(t *testing.T) {
			data, e := os.ReadFile(file)
			if e != nil {
				t.Fatal(e)
			}
			if e := s.Accepts(data); (e != nil) != (kind == "invalid") {
				t.Fatalf("schema verdict: %v, want invalid %v", e, kind == "invalid")
			}
			if kind == "invalid" {
				return
			}
			var p Provenance
			if e := json.Unmarshal(data, &p); e != nil {
				t.Fatal(e)
			}
			if e := p.Validate(); (e != nil) != (kind == "semantic-invalid") {
				t.Fatalf("semantic verdict: %v, want invalid %v", e, kind == "semantic-invalid")
			}
		})
	}
	// Mandatory provenance and omitted-proof states each have a negative case.
	for _, name := range []string{"invalid/missing-environment", "invalid/missing-capture-actor", "invalid/omitted-signature", "invalid/unknown-state", "invalid/waiver-label", "invalid/declared-independence", "semantic-invalid/self-delegation", "semantic-invalid/bad-calendar"} {
		if !seen[name] {
			t.Errorf("missing fixture %s", name)
		}
	}
	e = provFixture(t, "semantic-invalid/unknown-capability").Validate()
	requireCode(t, e, "unsupported-capability")
	if e.(*Refusal).State != "could-not-check" {
		t.Fatal("unknown capability must be could-not-check")
	}
}
