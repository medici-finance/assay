package domain

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/evaluation"
	"github.com/medici-finance/assay/statusgen/internal/testschema"
)

func fixture(t *testing.T, name string) Record {
	t.Helper()
	b, e := os.ReadFile("../testdata/domain/" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	var r Record
	if e = json.Unmarshal(b, &r); e != nil {
		t.Fatal(e)
	}
	return r
}
func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	var r *Refusal
	if !errors.As(err, &r) || r.Code != code {
		t.Fatalf("got %v, want refusal %s", err, code)
	}
}

// frozenRepo answers only the observations in this fixture. All history beyond
// the exact tree is unavailable; a nil embedded port catches accidental new reads.
type frozenRepo struct {
	evaluation.Repo
	tree string
}

func (f frozenRepo) InRepository() bool { return false }
func (f frozenRepo) VerifyExpectAt(rev, path, row string) (string, bool) {
	if rev == f.tree && path == "docs/streams/build/brief-07.md" && row == "1" {
		return "exit 0", true
	}
	return "", false
}
func observations(r Record) Observations {
	o := Observations{Work: r.Work, Candidate: r.Candidate, Generation: r.Generation.Recorded,
		Eligibility: evaluation.EligibilityInput{Streams: []evaluation.Stream{{Name: "build", Briefs: []evaluation.Brief{{Num: "07", Status: "implemented"}}}}},
		Coverage:    evaluation.CoverageInput{Revision: strings.Repeat("9", 40), Briefs: []evaluation.BriefObservation{{ID: "build/07", BriefRel: "docs/streams/build/brief-07.md", VerifyRows: []evaluation.VerifyRow{{ID: "1", Command: "test -f src.txt", Expect: "exit 0"}}, Witnesses: map[string][]evaluation.Witness{"1": {{Command: "test -f src.txt", State: evaluation.StatePass, Tree: r.Candidate.Tree}}}}}},
		Repo:        frozenRepo{tree: r.Candidate.Tree},
	}
	if r.Acceptance != nil {
		o.AcceptanceVersion = r.Acceptance.Version
		o.AcceptanceDigest = r.Acceptance.Digest
	}
	return o
}

// regression: graph-execution/28
func TestDomainPathIsNotIdentity(t *testing.T) {
	a, e := NewWork("build:07", "example:widgets:other:02", "inst-example-07", "verify", 1)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewWork("widgets:build:07", "example:other:other:02", "inst-example-07", "verify", 1)
	if e != nil {
		t.Fatal(e)
	}
	if a != b || a.Brief != "example:widgets:build:07" {
		t.Fatalf("identity changed: %+v %+v", a, b)
	}
	for _, path := range []string{"old.md", "renamed.md"} {
		if e := ValidateLocations([]LocatedWork{{path, a.Brief}}); e != nil {
			t.Fatal(e)
		}
	}
	requireCode(t, ValidateLocations([]LocatedWork{{"old.md", a.Brief}, {"renamed.md", a.Brief}}), "duplicate-work")
	for _, bad := range []string{"docs/streams/build/brief-07.md", "build/07", "#7"} {
		_, e := NewWork(bad, a.Brief, a.Instance, "verify", 1)
		requireCode(t, e, "work-identity")
	}
	a.Attempt = "inst-other-07.verify.a1"
	requireCode(t, a.Validate(), "attempt-identity")
}

// regression: graph-execution/28
func TestDomainMissingFactHolds(t *testing.T) {
	for _, state := range []Completeness{Missing, CouldNotCheck} {
		r := fixture(t, "valid/candidate-a")
		r.Facts.Entries[1].State = state
		a, e := Assess(r, observations(r))
		if e != nil {
			t.Fatal(e)
		}
		if a.State != "held" || !reflect.DeepEqual(a.Holds, []FactHold{{"coverage", state}}) || a.Coverage != nil {
			t.Fatalf("incomplete facts passed: %+v", a)
		}
	}
	r := fixture(t, "valid/candidate-a")
	r.Facts.Entries = r.Facts.Entries[:1]
	a, e := Assess(r, observations(r))
	if e != nil || a.State != "held" || len(a.Holds) != 1 || a.Holds[0].State != Missing {
		t.Fatalf("omitted required fact: %+v %v", a, e)
	}
	r.Facts.Entries = append(r.Facts.Entries, Fact{"coverage", "future-state"})
	requireCode(t, r.Validate(), "fact-state")
}

// regression: graph-execution/28
func TestDomainAcceptanceBindsCandidate(t *testing.T) {
	r := fixture(t, "valid/candidate-a")
	a, e := Assess(r, observations(r))
	if e != nil || a.State != "supported" {
		t.Fatalf("positive binding: %+v %v", a, e)
	}
	// Both snapshots can have the same branch; the record cannot express that
	// mutable locator as a substitute for any member of the frozen subject.
	for _, field := range []string{"repository", "base-commit", "base-tree", "commit", "tree", "manifest"} {
		t.Run(field, func(t *testing.T) {
			b := r
			switch field {
			case "repository":
				b.Candidate.Repository = "example-org/other"
			case "base-commit":
				b.Candidate.BaseCommit = strings.Repeat("a", 40)
			case "base-tree":
				b.Candidate.BaseTree = strings.Repeat("a", 40)
			case "commit":
				b.Candidate.Commit = strings.Repeat("a", 40)
			case "tree":
				b.Candidate.Tree = strings.Repeat("a", 40)
			case "manifest":
				b.Candidate.ManifestDigest = "sha256:" + strings.Repeat("a", 64)
			}
			_, e := Assess(b, observations(b))
			requireCode(t, e, "candidate-mismatch")
		})
	}
	o := observations(r)
	o.AcceptanceVersion++
	_, e = Assess(r, o)
	requireCode(t, e, "observation-acceptance")
	o = observations(r)
	o.Work.Brief = "example:other:build:07"
	_, e = Assess(r, o)
	requireCode(t, e, "observation-subject")
}

// regression: graph-execution/28
func TestDomainExecutionIsNotAcceptance(t *testing.T) {
	r := fixture(t, "valid/execution-only")
	for _, state := range []string{"not-requested", "retired"} {
		r.Progress.Retirement = state
		a, e := Assess(r, observations(r))
		if e != nil || a.State != "held" || r.Progress.Acceptance != "unaccepted" {
			t.Fatalf("execution or retirement granted acceptance: %+v %v", a, e)
		}
	}
	r.Progress.Acceptance = "accepted"
	requireCode(t, r.Validate(), "acceptance-missing")
}

// regression: graph-execution/28
func TestDomainReevaluationGeneration(t *testing.T) {
	r := fixture(t, "valid/candidate-a")
	a, e := Assess(r, observations(r))
	if e != nil {
		t.Fatal(e)
	}
	if e := a.CheckCurrent(r); e != nil {
		t.Fatal(e)
	}
	r.Generation.Requested++
	_, e = Assess(r, observations(r))
	requireCode(t, e, "stale-generation")
	r.Generation.Recorded++
	requireCode(t, a.CheckCurrent(r), "stale-generation")
	o := observations(r)
	o.Generation--
	_, e = Assess(r, o)
	requireCode(t, e, "stale-generation")
}

// regression: graph-execution/28
func TestDomainEvaluationSeam(t *testing.T) {
	r := fixture(t, "valid/candidate-a")
	o := observations(r)
	a, e := Assess(r, o)
	if e != nil {
		t.Fatal(e)
	}
	if a.State != "supported" || a.Evaluator != evaluation.CurrentIdentity() || a.Coverage.Claims[0].Result != evaluation.ResultPass || a.Eligibility.Verdict != evaluation.VerdictEligible {
		t.Fatalf("real evaluator seam: %+v", a)
	}
	if o.Coverage.Revision == r.Candidate.Tree {
		t.Fatal("Assess mutated caller's revision")
	}
	o.Coverage.Briefs[0].Witnesses["1"][0].Tree = strings.Repeat("8", 40)
	a, e = Assess(r, o)
	if e != nil || a.State != "held" || a.Coverage.Claims[0].Result != evaluation.ResultWrongRevision {
		t.Fatalf("foreign witness released: %+v %v", a, e)
	}
	o = observations(r)
	o.Eligibility.Streams[0].Briefs[0].Gates = []evaluation.Edge{{Ref: "build/08", Type: "input"}}
	a, e = Assess(r, o)
	if e != nil || a.State != "held" || a.Eligibility.Verdict != evaluation.VerdictHeld || !a.Coverage.Released {
		t.Fatalf("eligibility bypassed: %+v %v", a, e)
	}
	o = observations(r)
	o.Coverage.Briefs = nil
	_, e = Assess(r, o)
	requireCode(t, e, "coverage-subject")
	o = observations(r)
	o.Eligibility.Streams = nil
	_, e = Assess(r, o)
	requireCode(t, e, "eligibility-subject")
}

// regression: graph-execution/28
func TestDomainStageSeparation(t *testing.T) {
	r := fixture(t, "valid/candidate-a")
	r.Progress.Acceptance = "held"
	r.Progress.Holds = []Hold{{"human", "decision-owner", "recorded-act", r.Progress.ObservedAt}}
	variants := []Record{r, fixture(t, "valid/accepted-unpublished"), fixture(t, "valid/retirement-held")}
	want := [][4]string{{"succeeded", "held", "not-requested", "not-requested"}, {"succeeded", "accepted", "pending", "not-requested"}, {"succeeded", "accepted", "confirmed", "held"}}
	for i, r := range variants {
		b, e := r.Canonical()
		if e != nil {
			t.Fatal(e)
		}
		var got Record
		if e := json.Unmarshal(b, &got); e != nil {
			t.Fatal(e)
		}
		p := got.Progress
		if [4]string{p.Execution, p.Acceptance, p.Publication, p.Retirement} != want[i] {
			t.Fatalf("stage conflation: %+v", p)
		}
	}
	r = fixture(t, "valid/accepted-unpublished")
	r.Progress.Stage = "complete"
	requireCode(t, r.Validate(), "completion-premature")
	r = fixture(t, "valid/candidate-a")
	r.Progress.Components.Decide = "future-state"
	e := r.Validate()
	requireCode(t, e, "unsupported-component")
	var ref *Refusal
	if !errors.As(e, &ref) || ref.State != "could-not-check" {
		t.Fatalf("unknown state not scoped uncertainty: %v", e)
	}
}

// regression: graph-execution/28
func TestDomainPureImports(t *testing.T) {
	out, e := exec.Command("go", "list", "-f", "{{join .Imports `\n`}}", ".").CombinedOutput()
	if e != nil {
		t.Fatalf("go list: %s %v", out, e)
	}
	banned := map[string]bool{"os": true, "os/exec": true, "net": true, "net/http": true, "syscall": true, "io/ioutil": true, "path/filepath": true}
	for _, p := range strings.Fields(string(out)) {
		if banned[p] {
			t.Fatalf("domain effect import: %s", p)
		}
	}
}

// regression: graph-execution/28
func TestDomainSchemaFixtures(t *testing.T) {
	s := testschema.Load(t, "../../schemas/verification-domain-v1.json")
	spec, e := os.ReadFile("../../spec/verification-domain-v1.md")
	if e != nil {
		t.Fatal(e)
	}
	for name := range s.Root["$defs"].(map[string]any) {
		if !strings.Contains(string(spec), "`"+name+"`") {
			t.Errorf("spec lacks %s", name)
		}
	}
	files, e := filepath.Glob("../testdata/domain/*/*.json")
	if e != nil || len(files) < 15 {
		t.Fatalf("fixtures not found: %v", e)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, e := os.ReadFile(file)
			if e != nil {
				t.Fatal(e)
			}
			invalid := filepath.Base(filepath.Dir(file)) == "invalid"
			e = s.Accepts(data)
			if (e != nil) != invalid {
				t.Fatalf("schema verdict: %v, want invalid %v", e, invalid)
			}
			if invalid {
				return
			}
			var r Record
			if e := json.Unmarshal(data, &r); e != nil {
				t.Fatal(e)
			}
			semantic := filepath.Base(filepath.Dir(file)) == "semantic-invalid"
			if e := r.Validate(); (e != nil) != semantic {
				t.Fatalf("semantic verdict: %v, want invalid %v", e, semantic)
			}
			if semantic {
				return
			}
			if filepath.Base(filepath.Dir(file)) == "historical" {
				requireCode(t, r.Generation.CheckCurrent(), "stale-generation")
			}
			b, e := r.Canonical()
			if e != nil {
				t.Fatal(e)
			}
			if e := s.Accepts(b); e != nil {
				t.Fatal(e)
			}
			var round Record
			if e := json.Unmarshal(b, &round); e != nil {
				t.Fatal(e)
			}
			b2, e := round.Canonical()
			if e != nil || string(b) != string(b2) {
				t.Fatalf("unstable canonical: %v", e)
			}
		})
	}
}

// regression: graph-execution/28
func TestDomainSchemaSurface(t *testing.T) {
	s := testschema.Load(t, "../../schemas/verification-domain-v1.json")
	defs := s.Root["$defs"].(map[string]any)
	types := map[string]any{"": Record{}, "work-identity": WorkIdentity{}, "candidate": Candidate{}, "acceptance-binding": AcceptanceBinding{}, "fact-set": FactSet{}, "fact": Fact{}, "reevaluation-generation": Generation{}, "progress": Progress{}, "components": Components{}, "hold": Hold{}}
	for name, v := range types {
		node := s.Root
		if name != "" {
			node = defs[name].(map[string]any)
		}
		typ := reflect.TypeOf(v)
		var fields, required []string
		for i := 0; i < typ.NumField(); i++ {
			tag := typ.Field(i).Tag.Get("json")
			parts := strings.Split(tag, ",")
			fields = append(fields, parts[0])
			if len(parts) == 1 {
				required = append(required, parts[0])
			}
		}
		var props, req []string
		for k := range node["properties"].(map[string]any) {
			props = append(props, k)
		}
		for _, k := range node["required"].([]any) {
			req = append(req, k.(string))
		}
		for _, x := range [][]string{fields, required, props, req} {
			sort.Strings(x)
		}
		if !reflect.DeepEqual(fields, props) || !reflect.DeepEqual(required, req) {
			t.Fatalf("%s schema fields drift", name)
		}
	}
	for k, want := range states {
		name := k + "-state"
		if k == "hold" {
			name = "hold-kind"
		}
		var got []string
		for _, v := range defs[name].(map[string]any)["enum"].([]any) {
			got = append(got, v.(string))
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s states drift", k)
		}
	}
	// The instance shape is consumed from /09, not a second identifier scheme.
	base := testschema.Load(t, "../../schemas/workflow-instance-v1.json")
	props := base.Root["properties"].(map[string]any)
	inst := props["instance"].(map[string]any)["properties"].(map[string]any)["id"].(map[string]any)
	if inst["pattern"] != instanceRE.String() {
		t.Fatal("instance identity grammar drift")
	}
}

// regression: graph-execution/28
func TestDomainCanonical(t *testing.T) {
	r := fixture(t, "valid/candidate-a")
	b, e := r.Canonical()
	if e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(string(b), `{"acceptance":{"candidate":{"base-commit":`) || strings.Contains(string(b), "\n") {
		t.Fatalf("canonical key ordering changed: %s", b)
	}
	var round Record
	pretty, e := json.MarshalIndent(r, "", "   ")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(pretty, &round); e != nil {
		t.Fatal(e)
	}
	b2, e := round.Canonical()
	if e != nil || string(b) != string(b2) {
		t.Fatal("formatting changed canonical record")
	}
	d, e := r.Digest()
	if e != nil || !strings.HasPrefix(d, "sha256:") || len(d) != 71 {
		t.Fatalf("digest: %s %v", d, e)
	}
	r.Candidate.Commit = "short"
	if _, e := r.Canonical(); e == nil {
		t.Fatal("invalid record exported")
	}
}
