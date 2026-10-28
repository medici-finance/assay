package domain

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/statusgen/evaluation"
	"github.com/medici-finance/assay/statusgen/internal/testschema"
)

func contextFixture(t *testing.T) EvaluationContext {
	t.Helper()
	b, e := os.ReadFile("../testdata/domain/context/valid.json")
	if e != nil {
		t.Fatal(e)
	}
	var c EvaluationContext
	if e = json.Unmarshal(b, &c); e != nil {
		t.Fatal(e)
	}
	return c
}

// regression: graph-execution/30
func TestContextInputs(t *testing.T) {
	c := contextFixture(t)
	wantBytes, e := os.ReadFile("../testdata/domain/context/fingerprint.txt")
	if e != nil {
		t.Fatal(e)
	}
	want := strings.TrimSpace(string(wantBytes))
	got, e := c.Fingerprint()
	if e != nil || got != want {
		t.Fatalf("fixed fingerprint: %s %v want %s", got, e, want)
	}
	changes := map[string]func(*EvaluationContext){
		"work": func(x *EvaluationContext) { x.Work.Attempt = x.Work.Node + ".a2" },
		"candidate": func(x *EvaluationContext) {
			x.Candidate.Tree = strings.Repeat("b", 40)
			x.Acceptance.Candidate = x.Candidate
		},
		"acceptance":        func(x *EvaluationContext) { x.Acceptance.Version++ },
		"acceptance-digest": func(x *EvaluationContext) { x.Acceptance.Digest = "sha256:" + strings.Repeat("b", 64) },
		"requirements":      func(x *EvaluationContext) { x.Acceptance.RequiredFacts = append(x.Acceptance.RequiredFacts, "review") },
		"source":            func(x *EvaluationContext) { x.Sources[0].Digest = "sha256:" + strings.Repeat("b", 64) },
		"fact":              func(x *EvaluationContext) { x.Facts[0].Digest = "sha256:" + strings.Repeat("b", 64) },
		"evaluator":         func(x *EvaluationContext) { x.Evaluator.Version = "statusgen-evaluation/next" },
		"evaluator-policy":  func(x *EvaluationContext) { x.Evaluator.PolicyDigest = strings.Repeat("b", 64) },
		"query":             func(x *EvaluationContext) { x.QueryDigest = "sha256:" + strings.Repeat("b", 64) },
		"options":           func(x *EvaluationContext) { x.OptionsDigest = "sha256:" + strings.Repeat("b", 64) },
		"policy":            func(x *EvaluationContext) { x.PolicyDigest = "sha256:" + strings.Repeat("b", 64) },
		"environment":       func(x *EvaluationContext) { x.EnvironmentDigest = "sha256:" + strings.Repeat("b", 64) },
		"epoch":             func(x *EvaluationContext) { x.AccessEpoch++ },
		"generation":        func(x *EvaluationContext) { x.Generation++ },
		"cutoff":            func(x *EvaluationContext) { x.ObservedCutoff = "2026-10-28T10:01:00Z" },
		"expiry":            func(x *EvaluationContext) { x.Bounds[0].ExpiresAt = "2026-10-28T10:05:00Z" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			x := contextFixture(t)
			change(&x)
			fp, e := x.Fingerprint()
			if e != nil || fp == got {
				t.Fatalf("input omitted: %s %v", fp, e)
			}
			if CheckApplicability(c, x, "2026-10-28T10:02:00Z").State != "reevaluate" {
				t.Fatal("changed context reused")
			}
		})
	}
	for name, change := range map[string]func(*EvaluationContext){
		"version":    func(x *EvaluationContext) { x.Schema = "future" },
		"capability": func(x *EvaluationContext) { x.Requires = []string{"future@1"} },
		"evaluator":  func(x *EvaluationContext) { x.Evaluator = evaluation.Identity{} },
		"membership": func(x *EvaluationContext) { x.Dependencies = nil },
		"coverage":   func(x *EvaluationContext) { x.Coverage = CouldNotCheck },
		"sources":    func(x *EvaluationContext) { x.Sources = nil },
		"facts":      func(x *EvaluationContext) { x.Facts = nil },
		"options":    func(x *EvaluationContext) { x.OptionsDigest = "" },
		"epoch":      func(x *EvaluationContext) { x.AccessEpoch = 0 },
		"bounds":     func(x *EvaluationContext) { x.Bounds = nil },
	} {
		t.Run("missing-"+name, func(t *testing.T) {
			x := contextFixture(t)
			change(&x)
			if _, e := x.Fingerprint(); e == nil {
				t.Fatal("incomplete context fingerprinted")
			}
			if CheckApplicability(c, x, "2026-10-28T10:02:00Z").State != "held" {
				t.Fatal("incomplete context reused")
			}
		})
	}
	// Set permutations are canonical and must not mutate the caller's slices.
	x := contextFixture(t)
	x.Dependencies[0], x.Dependencies[1] = x.Dependencies[1], x.Dependencies[0]
	x.Bounds[0], x.Bounds[1] = x.Bounds[1], x.Bounds[0]
	before, _ := json.Marshal(x)
	fp, e := x.Fingerprint()
	after, _ := json.Marshal(x)
	if e != nil || fp != got || string(before) != string(after) {
		t.Fatalf("canonical set order: %s %v", fp, e)
	}
	x = contextFixture(t)
	x.Generation = 9007199254740993
	a, _ := x.Fingerprint()
	x.Generation--
	b, _ := x.Fingerprint()
	if a == b {
		t.Fatal("integer precision lost")
	}
}

// regression: graph-execution/30
func TestContextExpiry(t *testing.T) {
	c := contextFixture(t)
	for _, kind := range []string{"lease", "access", "evaluation"} {
		t.Run(kind, func(t *testing.T) {
			x := contextFixture(t)
			x.Bounds = []ValidityBound{{"lease", "2026-10-28T11:00:00Z"}, {"access", "2026-10-28T11:00:00Z"}}
			if kind == "evaluation" {
				x.Bounds = append(x.Bounds, ValidityBound{kind, "2026-10-28T10:10:00Z"})
			} else {
				for i := range x.Bounds {
					if x.Bounds[i].Kind == kind {
						x.Bounds[i].ExpiresAt = "2026-10-28T10:10:00Z"
					}
				}
			}
			for _, tt := range []struct{ at, want string }{{"2026-10-28T10:09:59Z", "reusable"}, {"2026-10-28T10:10:00Z", "reevaluate"}, {"2026-10-28T10:10:01Z", "reevaluate"}} {
				if got := CheckApplicability(x, x, tt.at); got.State != tt.want {
					t.Fatalf("%s: %+v", tt.at, got)
				}
			}
		})
	}
	for _, now := range []string{"", "unavailable", "2026-10-28T09:59:59Z"} {
		if CheckApplicability(c, c, now).State != "held" {
			t.Fatal("unavailable or past time reused")
		}
	}
}

// regression: graph-execution/30
func TestContextMembership(t *testing.T) {
	c := contextFixture(t)
	for _, kind := range []string{"removed", "discovered", "value"} {
		t.Run(kind, func(t *testing.T) {
			x := contextFixture(t)
			switch kind {
			case "removed":
				x.Dependencies = x.Dependencies[:1]
			case "discovered":
				x.Dependencies[1].Present = true
			case "value":
				x.Dependencies[0].Digest = "sha256:" + strings.Repeat("b", 64)
			}
			x.DependencyDigest, _ = DependencySetDigest(x.Dependencies)
			if CheckApplicability(c, x, "2026-10-28T10:02:00Z").State != "reevaluate" {
				t.Fatal("membership change reused")
			}
		})
	}
	for _, coverage := range []Completeness{Missing, CouldNotCheck} {
		x := contextFixture(t)
		x.Coverage = coverage
		if CheckApplicability(x, x, "2026-10-28T10:02:00Z").State != "held" {
			t.Fatal("incomplete coverage selectively reused")
		}
	}
	x := contextFixture(t)
	x.Dependencies = append(x.Dependencies, x.Dependencies[0])
	if _, e := x.Fingerprint(); e == nil {
		t.Fatal("duplicate accepted")
	}
}

// currentAuthority models the separate admission boundary, deliberately taking
// independently supplied facts instead of trusting the applicability result.
func currentAuthority(authorized bool, grantExpiry, now string) bool {
	expiry, e := time.Parse(time.RFC3339, grantExpiry)
	if e != nil {
		return false
	}
	at, e := time.Parse(time.RFC3339, now)
	return e == nil && authorized && at.Before(expiry)
}

// regression: graph-execution/30
func TestContextAdmissionFlow(t *testing.T) {
	c := contextFixture(t)
	r := fixture(t, "valid/candidate-a")
	if c.Work != r.Work || c.Candidate != r.Candidate || !reflect.DeepEqual(c.Acceptance, *r.Acceptance) {
		t.Fatal("fixture subject mismatch")
	}
	cached, e := Assess(r, observations(r))
	if e != nil || cached.State != "supported" {
		t.Fatalf("assessment: %+v %v", cached, e)
	}
	fresh, e := Assess(r, observations(r))
	if e != nil || !reflect.DeepEqual(cached, fresh) {
		t.Fatal("fresh differs")
	}
	if CheckApplicability(c, c, "2026-10-28T10:02:00Z").State != "reusable" || !currentAuthority(true, "2026-10-28T10:10:00Z", "2026-10-28T10:02:00Z") {
		t.Fatal("authorized immutable hit held")
	}
	if CheckApplicability(c, c, "2026-10-28T10:10:00Z").State != "reevaluate" {
		t.Fatal("expired runtime result reused")
	}
	// Bypass the upper context validator entirely: cached support still grants
	// no effect when the independent lower grant is expired or unavailable.
	for _, grant := range []string{"2026-10-28T10:10:00Z", ""} {
		if currentAuthority(true, grant, "2026-10-28T10:10:00Z") {
			t.Fatal("cache hit became authority")
		}
	}
	if currentAuthority(false, "2026-10-28T11:00:00Z", "2026-10-28T10:02:00Z") {
		t.Fatal("revocation ignored")
	}
	moved := r
	moved.Candidate.Tree = strings.Repeat("b", 40)
	binding := *r.Acceptance
	binding.Candidate = moved.Candidate
	moved.Acceptance = &binding
	if cached.CheckCurrent(moved) == nil {
		t.Fatal("cached PASS retargeted")
	}
	movedObservations := observations(moved)
	movedObservations.Coverage = observations(r).Coverage
	movedObservations.Repo = observations(r).Repo
	next, e := Assess(moved, movedObservations)
	if e != nil || next.State != "held" || next.Coverage.Released {
		t.Fatalf("old witness released new revision: %+v %v", next, e)
	}
}

// regression: graph-execution/30
func TestContextSchema(t *testing.T) {
	s := testschema.Load(t, "../../schemas/evaluation-context-v1.json")
	b, e := os.ReadFile("../testdata/domain/context/valid.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Accepts(b); e != nil {
		t.Fatal(e)
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(b, &fields); e != nil {
		t.Fatal(e)
	}
	spec, e := os.ReadFile("../../spec/evaluation-context-v1.md")
	if e != nil {
		t.Fatal(e)
	}
	for key := range fields {
		if !strings.Contains(string(spec), "`"+key+"`") {
			t.Errorf("spec lacks %s", key)
		}
		var reduced map[string]json.RawMessage
		_ = json.Unmarshal(b, &reduced)
		delete(reduced, key)
		raw, _ := json.Marshal(reduced)
		if s.Accepts(raw) == nil {
			t.Errorf("mandatory field omitted: %s", key)
		}
	}
	required := s.Root["required"].([]any)
	if len(required) != len(fields) || reflect.TypeOf(EvaluationContext{}).NumField() != len(fields) {
		t.Fatal("wire field inventory drift")
	}
	for _, name := range []string{"unknown-field", "missing-evaluator", "bad-digest", "unknown-version"} {
		raw, e := os.ReadFile("../testdata/domain/context/" + name + ".json")
		if e != nil {
			t.Fatal(e)
		}
		if s.Accepts(raw) == nil {
			t.Errorf("negative fixture passed: %s", name)
		}
	}
	c := contextFixture(t)
	if e = c.Validate(); e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(c)
	for _, secret := range []string{"token", "credential", "password", "bearer"} {
		if strings.Contains(strings.ToLower(string(encoded)), secret) {
			t.Fatal("secret payload serialized")
		}
	}
}
