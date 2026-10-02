package deskkit

// FAIL-FIRST: internal/deskkit/admission-mutations.json (run with
// `go run ./cmd/muhar -spec internal/deskkit/admission-mutations.json` from tools/desk).
// Each mutant is a way confidence, a missing fact or an assessed string could open a
// lane, drop a gate or reach the decision record. Every one is caught by this file's
// TestAgenticAdmission tests.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

const admissionFixtureDir = "testdata/admission"

// admissionCase is one fixture under testdata/admission: an assessment, the evaluation
// context, an optional inline policy (absent = policy.json) and the expected outcome.
type admissionCase struct {
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	BriefRiskVerdict string            `json:"briefRiskVerdict"`
	Policy           *AdmissionPolicy  `json:"policy,omitempty"`
	Assessment       AgenticAssessment `json:"assessment"`
	Context          AdmissionContext  `json:"context"`
	Expect           struct {
		Disposition         AdmissionDisposition `json:"disposition"`
		Reasons             []string             `json:"reasons"`
		PermittedOperations []string             `json:"permittedOperations"`
		DiscoveryScope      []string             `json:"discoveryScope"`
	} `json:"expect"`
}

func loadAdmissionPolicy(t *testing.T) AdmissionPolicy {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(admissionFixtureDir, "policy.json"))
	if err != nil {
		t.Fatalf("read policy fixture: %v", err)
	}
	var p AdmissionPolicy
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatalf("parse policy fixture: %v", err)
	}
	return p
}

func loadAdmissionCases(t *testing.T) []admissionCase {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(admissionFixtureDir, "*.json"))
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}
	var cases []admissionCase
	for _, p := range paths {
		if filepath.Base(p) == "policy.json" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		var c admissionCase
		dec := json.NewDecoder(strings.NewReader(string(b)))
		dec.DisallowUnknownFields() // a misspelt fixture key is a red run, not a dropped field
		if err := dec.Decode(&c); err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		if c.Name+".json" != filepath.Base(p) {
			t.Fatalf("%s: fixture name %q does not match its file name", p, c.Name)
		}
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		t.Fatal("no admission fixtures found — the fixture directory is wrong")
	}
	return cases
}

func (c admissionCase) policy(base AdmissionPolicy) AdmissionPolicy {
	if c.Policy != nil {
		return *c.Policy
	}
	return base
}

// requiredAdmissionFixtures are the negative paths the brief names. Deleting one is a
// red run here, not a silently thinner fixture set.
var requiredAdmissionFixtures = []string{
	"high-confidence-unsafe-advice",
	"missing-verifier",
	"revoked-category",
	"schema-changed",
	"environment-changed",
	"expired-exception",
	"no-data-authority",
	"all-hard-pass-policy-disallowed",
	// The fail-closed rules of spec §3.1 and §4, one fixture each.
	"subject-changed",
	"fact-future",
	"fact-zero-time",
	"fact-duplicated",
	"not-applicable",
	"applicability-prefix-boundary",
	"operations-undeclared",
	// discovery-only stands only on a grant covering the subject (spec §2, §4 step 9).
	"discovery-out-of-scope",
	"readiness-fail-no-grant",
	"readiness-fail-discovery-granted",
	"advice-discovery-no-grant",
}

// TestAgenticAdmissionFixtures runs every fixture through the production evaluator and
// checks its disposition, its reason codes (as a required subset, in order-insensitive
// form) and its permitted operations, plus the floors every result carries.
func TestAgenticAdmissionFixtures(t *testing.T) {
	base := loadAdmissionPolicy(t)
	if err := ValidateAdmissionPolicy(base); err != nil {
		t.Fatalf("fixture policy is invalid: %v", err)
	}
	cases := loadAdmissionCases(t)
	have := map[string]bool{}
	for _, c := range cases {
		have[c.Name] = true
	}
	for _, n := range requiredAdmissionFixtures {
		if !have[n] {
			t.Errorf("required admission fixture %q is missing from %s", n, admissionFixtureDir)
		}
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			res := EvaluateAgenticAdmission(c.policy(base), c.Assessment, c.Context)
			if res.Disposition != c.Expect.Disposition {
				t.Fatalf("disposition = %q, want %q (reasons %v)", res.Disposition, c.Expect.Disposition, res.Reasons)
			}
			got := map[string]bool{}
			for _, r := range res.Reasons {
				got[r] = true
			}
			for _, want := range c.Expect.Reasons {
				if !got[want] {
					t.Errorf("reasons %v lack %q", res.Reasons, want)
				}
			}
			if c.Expect.PermittedOperations != nil {
				gotOps := append([]string{}, res.PermittedOperations...)
				sort.Strings(gotOps)
				if strings.Join(gotOps, ",") != strings.Join(c.Expect.PermittedOperations, ",") {
					t.Errorf("permitted operations = %v, want %v", gotOps, c.Expect.PermittedOperations)
				}
			}
			if c.Expect.DiscoveryScope != nil && strings.Join(res.DiscoveryScope, ",") != strings.Join(c.Expect.DiscoveryScope, ",") {
				t.Errorf("discovery scope = %v, want %v", res.DiscoveryScope, c.Expect.DiscoveryScope)
			}
			assertAdmissionFloors(t, res)
		})
	}
}

// assertAdmissionFloors checks the properties every admission result carries.
func assertAdmissionFloors(t *testing.T, res AdmissionResult) {
	t.Helper()
	if !res.Disposition.Valid() {
		t.Fatalf("disposition %q is outside the closed vocabulary", res.Disposition)
	}
	if len(res.Reasons) == 0 {
		t.Errorf("result for %q carries no reason codes", res.Subject)
	}
	// The record's subject is either one that passed the subject grammar or empty: a
	// refused subject is never echoed (spec §7).
	if res.Subject != "" && !subjectValid(res.Subject) {
		t.Errorf("result echoes a subject outside the subject grammar (%d bytes)", len(res.Subject))
	}
	floors := strings.Join(res.HumanFloors, ",")
	for _, f := range []string{"merge", "release", "deploy"} {
		if !strings.Contains(floors, f) {
			t.Errorf("human floors %v lack %q", res.HumanFloors, f)
		}
	}
	for _, op := range res.PermittedOperations {
		if isHumanFloor(op) {
			t.Errorf("permitted operations %v include human-floor %q", res.PermittedOperations, op)
		}
	}
	if res.Disposition == AdmitBlocked && len(res.PermittedOperations) != 0 {
		t.Errorf("blocked result permits %v", res.PermittedOperations)
	}
	if res.RiskInput != DispositionRiskInput(res.Disposition) {
		t.Errorf("risk input %q does not match the mapping for %q", res.RiskInput, res.Disposition)
	}
	// discovery-only always carries the covering grant scope; nothing else carries one.
	if (res.Disposition == AdmitDiscoveryOnly) != (len(res.DiscoveryScope) > 0) {
		t.Errorf("disposition %q with discovery scope %v", res.Disposition, res.DiscoveryScope)
	}
	// Class guard over every reason-producing site: each code is a literal plus
	// grammar-checked tokens, so no assessed value can add a separator, a line or a code.
	for _, r := range res.Reasons {
		if !admissionReasonRE.MatchString(r) {
			t.Errorf("reason %q is outside the reason-code grammar (all reasons %q)", r, res.Reasons)
		}
	}
}

// admissionReasonRE is the reason-code grammar: a lowercase literal, then zero or more
// `:`-separated tokens of reasonTokenRE's alphabet.
var admissionReasonRE = regexp.MustCompile(`^[a-z][a-z0-9-]*(:[A-Za-z0-9][A-Za-z0-9._@/-]{0,63})*$`)

// TestAgenticAdmissionPolicyValidation proves the policy validator refuses every shape
// that would let a failed check or a confident model open a lane, and that the evaluator
// fails closed on such a policy rather than evaluating it.
func TestAgenticAdmissionPolicyValidation(t *testing.T) {
	base := loadAdmissionPolicy(t)
	clone := func() AdmissionPolicy {
		b, _ := json.Marshal(base)
		var p AdmissionPolicy
		if err := json.Unmarshal(b, &p); err != nil {
			t.Fatalf("clone policy: %v", err)
		}
		return p
	}
	cases := []struct {
		name   string
		mutate func(*AdmissionPolicy)
		want   string
	}{
		{"failed check maps to an agent lane", func(p *AdmissionPolicy) {
			p.ReadinessFailCeiling[HardIndependentVerifier] = AdmitSupervisedAgent
		}, "agent lane"},
		{"failed check has no ceiling", func(p *AdmissionPolicy) {
			delete(p.ReadinessFailCeiling, HardBudget)
		}, "no fail ceiling"},
		{"ceiling for an authority check", func(p *AdmissionPolicy) {
			p.ReadinessFailCeiling[HardAuthority] = AdmitHumanLed
		}, "not a readiness check"},
		{"category permits merge", func(p *AdmissionPolicy) {
			c := p.Categories["docs-change"]
			c.PermittedOperations = append(c.PermittedOperations, "merge")
			p.Categories["docs-change"] = c
		}, "human-floor"},
		{"category permits an unknown operation", func(p *AdmissionPolicy) {
			c := p.Categories["docs-change"]
			c.PermittedOperations = append(c.PermittedOperations, "approve")
			p.Categories["docs-change"] = c
		}, "unknown operation"},
		{"category ceiling outside the vocabulary", func(p *AdmissionPolicy) {
			c := p.Categories["docs-change"]
			c.Ceiling = "autonomous"
			p.Categories["docs-change"] = c
		}, "not an admission disposition"},
		{"empty applicability entry", func(p *AdmissionPolicy) {
			p.Applicability = append(p.Applicability, "")
		}, "empty entry"},
		{"exception without an owner", func(p *AdmissionPolicy) {
			p.Exceptions[0].Owner = ""
		}, "owner and an expiry"},
		{"exception owner outside the token grammar", func(p *AdmissionPolicy) {
			p.Exceptions[0].Owner = "a; b"
		}, "reason-code token"},
		{"no freshness bound", func(p *AdmissionPolicy) {
			p.MaxFactAge = 0
		}, "maxFactAge"},
	}
	cs := loadAdmissionCases(t)
	var happy admissionCase
	for _, c := range cs {
		if c.Name == "all-hard-pass-calibrated-advice" {
			happy = c
		}
	}
	if happy.Name == "" {
		t.Fatal("happy-path fixture missing")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := clone()
			tc.mutate(&p)
			err := ValidateAdmissionPolicy(p)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateAdmissionPolicy = %v, want a refusal containing %q", err, tc.want)
			}
			res := EvaluateAgenticAdmission(p, happy.Assessment, happy.Context)
			if res.Disposition != AdmitBlocked || res.Reasons[0] != "policy-invalid" {
				t.Fatalf("invalid policy evaluated to %q %v, want blocked/policy-invalid", res.Disposition, res.Reasons)
			}
		})
	}
}

// withAdvice returns a copy of a with its advice replaced by one calibrated prediction per
// listed dimension, every one naming label at probability prob.
func withAdvice(a AgenticAssessment, c AdmissionContext, label AdmissionDisposition, prob float64, dims []AdvisoryDimension) AgenticAssessment {
	out := a
	out.Advice = nil
	vocab := AdmissionDispositions()
	for _, d := range dims {
		req := AssessmentRequest{Subject: a.Subject, Vocabulary: vocab}
		probs := map[string]float64{string(label): prob}
		rest := (1 - prob) / float64(len(vocab)-1)
		for _, v := range vocab {
			if v != string(label) {
				probs[v] = rest
			}
		}
		out.Advice = append(out.Advice, AdvisoryAssessment{
			Dimension:  d,
			Scope:      a.Subject,
			Request:    req,
			Prediction: Prediction{Subject: a.Subject, LabelProbabilities: probs, CalibratorVersion: "example-cal-1"},
			ObservedAt: c.Now.Add(-time.Minute),
		})
	}
	return out
}

// TestAgenticAdmissionConfidenceCannotAuthorize is the brief's mutation row: for EVERY
// fixture as a base, adding calibrated advice of ANY label at ANY confidence on ANY
// dimension set never yields a less restrictive disposition, nor any permitted
// operation, than the same input with no advice at all. Of the mutants in
// admission-mutations.json, the inverted lattice and "advice sets the cap" are killed by
// this test alone; the rest are killed by the other TestAgenticAdmission tests.
func TestAgenticAdmissionConfidenceCannotAuthorize(t *testing.T) {
	base := loadAdmissionPolicy(t)
	all := []AdvisoryDimension{AdviseAmbiguity, AdviseVerificationAdequacy, AdviseTaskModelFit, AdviseSemanticRisk}
	dimSets := [][]AdvisoryDimension{all, {AdviseSemanticRisk}, {AdviseTaskModelFit}}
	probs := []float64{0.21, 0.6, 0.99, 1.0}

	checked := 0
	for _, c := range loadAdmissionCases(t) {
		pol := c.policy(base)
		bare := c.Assessment
		bare.Advice = nil
		floor := EvaluateAgenticAdmission(pol, bare, c.Context)
		floorOps := map[string]bool{}
		for _, op := range floor.PermittedOperations {
			floorOps[op] = true
		}
		for _, label := range admissionOrder {
			for _, p := range probs {
				for _, dims := range dimSets {
					res := EvaluateAgenticAdmission(pol, withAdvice(c.Assessment, c.Context, label, p, dims), c.Context)
					checked++
					if res.Disposition.rank() < floor.Disposition.rank() {
						t.Fatalf("%s: advice %s@%.2f on %v widened %q to %q (reasons %v)",
							c.Name, label, p, dims, floor.Disposition, res.Disposition, res.Reasons)
					}
					for _, op := range res.PermittedOperations {
						if !floorOps[op] {
							t.Fatalf("%s: advice %s@%.2f on %v granted operation %q the facts-only result did not",
								c.Name, label, p, dims, op)
						}
					}
					// Exactness: applied advice restricts to exactly min(facts-only, label);
					// ignored advice (disabled, binding changed, ...) changes nothing. A
					// discovery-only label with no grant covering the subject is blocked:
					// advice alone never yields a grant-less discovery-only.
					want := floor.Disposition
					if len(res.AdviceApplied) > 0 {
						want = moreRestrictive(floor.Disposition, label)
						if _, why := discoveryGrant(c.Context.Discovery, c.Assessment.Subject); want == AdmitDiscoveryOnly && why != "" {
							want = AdmitBlocked
						}
					}
					if res.Disposition != want {
						t.Fatalf("%s: advice %s@%.2f on %v gave %q, want %q (facts-only %q restricted by the advice)",
							c.Name, label, p, dims, res.Disposition, want, floor.Disposition)
					}
					assertAdmissionFloors(t, res)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no combinations evaluated")
	}

	// The named high-confidence-unsafe case, stated directly: p=1.0 bounded advice on
	// every dimension against a failed hard check is still human-led.
	for _, c := range loadAdmissionCases(t) {
		if c.Name != "high-confidence-unsafe-advice" {
			continue
		}
		res := EvaluateAgenticAdmission(base, withAdvice(c.Assessment, c.Context, AdmitBoundedAgentWork, 1.0, all), c.Context)
		if res.Disposition != AdmitHumanLed {
			t.Fatalf("p=1.0 bounded advice over a failed hard check gave %q, want human-led", res.Disposition)
		}
	}
}

const repoPatternDir = "../../../../spec/workflow-patterns"

// TestAgenticAdmissionRiskInputUnion is the brief's flow row: facts + recorded advice go
// through the production evaluator, its disposition crosses the seam into the SHIPPED
// workflow patterns' risk-input tables via LoadPatternRiskInput, and MandatoryGates
// returns the union — never fewer gates than the brief's own risk verdict requires.
func TestAgenticAdmissionRiskInputUnion(t *testing.T) {
	base := loadAdmissionPolicy(t)
	shipped := []string{"implementation-v1.yaml", "research-v1.yaml"}
	verdicts := []string{"low", "standard", "elevated", "human"}

	for _, f := range shipped {
		ri, err := LoadPatternRiskInput(filepath.Join(repoPatternDir, f))
		if err != nil {
			t.Fatalf("LoadPatternRiskInput(%s): %v", f, err)
		}
		// Every brief verdict x every disposition: a superset of both inputs.
		for _, v := range verdicts {
			for _, d := range admissionOrder {
				gates, err := MandatoryGates(ri, v, d)
				if err != nil {
					t.Fatalf("%s: MandatoryGates(%s, %s): %v", f, v, d, err)
				}
				assertSuperset(t, f+"/"+v+"/"+string(d)+" brief", gates, ri.Gates[v])
				assertSuperset(t, f+"/"+v+"/"+string(d)+" disposition", gates, ri.Gates[DispositionRiskInput(d)])
			}
		}
		// Every fixture, end to end through the evaluator.
		for _, c := range loadAdmissionCases(t) {
			res := EvaluateAgenticAdmission(c.policy(base), c.Assessment, c.Context)
			gates, err := MandatoryGates(ri, c.BriefRiskVerdict, res.Disposition)
			if err != nil {
				t.Fatalf("%s/%s: %v", f, c.Name, err)
			}
			assertSuperset(t, f+"/"+c.Name, gates, ri.Gates[c.BriefRiskVerdict])
			pr := res.PolicyResult("sha256:example-input", c.Context.Now)
			if pr.Decision != string(res.Disposition) || pr.PolicyVersion != base.Version && c.Policy == nil {
				t.Fatalf("%s: PolicyResult projection %+v does not carry the disposition/version", c.Name, pr)
			}
		}
	}

	// A high score cannot delete a required node: a human-verdict brief admitted to
	// bounded work by p=1.0 advice still carries every gate the human verdict names.
	impl, err := LoadPatternRiskInput(filepath.Join(repoPatternDir, "implementation-v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range loadAdmissionCases(t) {
		if c.Name != "all-hard-pass-calibrated-advice" {
			continue
		}
		all := []AdvisoryDimension{AdviseAmbiguity, AdviseVerificationAdequacy, AdviseTaskModelFit, AdviseSemanticRisk}
		res := EvaluateAgenticAdmission(base, withAdvice(c.Assessment, c.Context, AdmitBoundedAgentWork, 1.0, all), c.Context)
		if res.Disposition != AdmitBoundedAgentWork {
			t.Fatalf("setup: want bounded-agent-work, got %q %v", res.Disposition, res.Reasons)
		}
		gates, err := MandatoryGates(impl, "human", res.Disposition)
		if err != nil {
			t.Fatal(err)
		}
		assertSuperset(t, "human brief admitted to bounded work", gates, impl.Gates["human"])
	}

	// Union, not "take the stricter verdict": with non-nested gate lists the result must
	// hold both verdicts' gates.
	nn, err := LoadPatternRiskInput(filepath.Join(admissionFixtureDir, "patterns", "nonnested-v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	gates, err := MandatoryGates(nn, "human", AdmitBoundedAgentWork)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(gates, ",") != "gate-human,gate-standard" {
		t.Fatalf("non-nested union = %v, want [gate-human gate-standard]", gates)
	}
	gates, err = MandatoryGates(nn, "low", AdmitSupervisedAgent)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(gates, ",") != "gate-elevated,gate-low" {
		t.Fatalf("non-nested union = %v, want [gate-elevated gate-low]", gates)
	}

	// Lower boundary, independent of the pattern lint: the seam itself refuses a table it
	// cannot read whole, and an unmapped brief verdict holds rather than yielding no gates.
	for _, bad := range []string{"missing-verdict-v1.yaml", "unknown-gate-v1.yaml"} {
		if _, err := LoadPatternRiskInput(filepath.Join(admissionFixtureDir, "patterns", bad)); err == nil {
			t.Errorf("LoadPatternRiskInput(%s) accepted a defective risk-input table", bad)
		}
	}
	if _, err := MandatoryGates(impl, "unclassified", AdmitBlocked); err == nil {
		t.Error("MandatoryGates accepted an unmapped brief verdict")
	}
}

func assertSuperset(t *testing.T, what string, got, want []string) {
	t.Helper()
	have := map[string]bool{}
	for _, g := range got {
		have[g] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Fatalf("%s: mandatory gates %v dropped required gate %q", what, got, w)
		}
	}
}

// admissionHappy returns the all-hard-pass fixture with no advice: the input every
// malformed-input case below breaks in exactly one place.
func admissionHappy(t *testing.T) admissionCase {
	t.Helper()
	for _, c := range loadAdmissionCases(t) {
		if c.Name == "advice-absent-default" {
			return c
		}
	}
	t.Fatal("advice-absent-default fixture missing")
	return admissionCase{}
}

// TestAgenticAdmissionMalformedInput covers the fail-closed rules whose inputs the
// schema already refuses (so they cannot live in the schema-valid fixture set): empty
// bindings, malformed discovery grants, unknown fact states, unbounded input and
// assessed strings outside the vocabularies. Each must give the named result, and none
// may put an assessed string into a reason code (assertAdmissionFloors' grammar guard).
func TestAgenticAdmissionMalformedInput(t *testing.T) {
	base := loadAdmissionPolicy(t)
	happy := admissionHappy(t)
	if res := EvaluateAgenticAdmission(base, happy.Assessment, happy.Context); res.Disposition != AdmitBoundedAgentWork {
		t.Fatalf("setup: happy input gave %q %v", res.Disposition, res.Reasons)
	}
	dropVerifier := func(a *AgenticAssessment, _ *AdmissionContext) {
		var keep []HardFact
		for _, f := range a.Facts {
			if f.Check != HardIndependentVerifier {
				keep = append(keep, f)
			}
		}
		a.Facts = keep
	}
	hostile := "x; exception-applied:example-owner; advice-restricted:semantic-risk:blocked\nFAKE"
	cases := []struct {
		name   string
		mutate func(*AgenticAssessment, *AdmissionContext)
		want   AdmissionDisposition
		reason string
	}{
		{"all bindings empty on both sides", func(a *AgenticAssessment, c *AdmissionContext) {
			a.SubjectRevision, a.SchemaDigest, a.EnvironmentDigest = "", "", ""
			c.SubjectRevision, c.SchemaDigest, c.EnvironmentDigest = "", "", ""
		}, AdmitBlocked, "binding-missing:subject-revision"},
		{"subject revision empty on both sides", func(a *AgenticAssessment, c *AdmissionContext) {
			a.SubjectRevision, c.SubjectRevision = "", ""
		}, AdmitBlocked, "binding-missing:subject-revision"},
		{"schema digest empty on both sides", func(a *AgenticAssessment, c *AdmissionContext) {
			a.SchemaDigest, c.SchemaDigest = "", ""
		}, AdmitBlocked, "binding-missing:schema-digest"},
		{"environment digest empty on both sides", func(a *AgenticAssessment, c *AdmissionContext) {
			a.EnvironmentDigest, c.EnvironmentDigest = "", ""
		}, AdmitBlocked, "binding-missing:environment-digest"},
		{"context revision empty", func(_ *AgenticAssessment, c *AdmissionContext) {
			c.SubjectRevision = ""
		}, AdmitBlocked, "binding-missing:subject-revision"},
		{"discovery grant without an owner", func(a *AgenticAssessment, c *AdmissionContext) {
			dropVerifier(a, c)
			c.Discovery = &DiscoveryScope{ReadScope: []string{"example-org/widgets"}}
		}, AdmitBlocked, "discovery-grant-invalid"},
		{"discovery grant without a read scope", func(a *AgenticAssessment, c *AdmissionContext) {
			dropVerifier(a, c)
			c.Discovery = &DiscoveryScope{Owner: "example-owner"}
		}, AdmitBlocked, "discovery-grant-invalid"},
		{"discovery grant with a blank scope entry", func(a *AgenticAssessment, c *AdmissionContext) {
			dropVerifier(a, c)
			c.Discovery = &DiscoveryScope{Owner: "example-owner", ReadScope: []string{"example-org/widgets", " "}}
		}, AdmitBlocked, "discovery-grant-invalid"},
		{"discovery grant owner outside the token grammar", func(a *AgenticAssessment, c *AdmissionContext) {
			dropVerifier(a, c)
			c.Discovery = &DiscoveryScope{Owner: "owner; forged-code", ReadScope: []string{"example-org/widgets"}}
		}, AdmitBlocked, "discovery-grant-invalid"},
		{"authority fact state outside the vocabulary", func(a *AgenticAssessment, _ *AdmissionContext) {
			a.Facts[0].State = "passed"
		}, AdmitBlocked, "hard-unknown:authority"},
		{"readiness fact state outside the vocabulary", func(a *AgenticAssessment, _ *AdmissionContext) {
			for i := range a.Facts {
				if a.Facts[i].Check == HardBudget {
					a.Facts[i].State = "ok"
				}
			}
		}, AdmitBlocked, "hard-unknown:budget"},
		{"operation outside the vocabulary is not echoed", func(a *AgenticAssessment, _ *AdmissionContext) {
			a.Operations = append(a.Operations, hostile)
		}, AdmitHumanLed, "operation-unknown"},
		{"advice dimension outside the vocabulary is not echoed", func(a *AgenticAssessment, c *AdmissionContext) {
			*a = withAdvice(*a, *c, AdmitBlocked, 0.99, []AdvisoryDimension{AdvisoryDimension(hostile)})
		}, AdmitBoundedAgentWork, "advice-ignored:unknown-dimension"},
		{"subject with a newline", func(a *AgenticAssessment, _ *AdmissionContext) {
			a.Subject = "example-org/widgets#12\nFAKE"
		}, AdmitBlocked, "subject-invalid"},
		{"subject with a space", func(a *AgenticAssessment, _ *AdmissionContext) {
			a.Subject = "example-org/widgets #12"
		}, AdmitBlocked, "subject-invalid"},
		{"subject over the length bound", func(a *AgenticAssessment, _ *AdmissionContext) {
			a.Subject = "example-org/widgets#" + strings.Repeat("9", admissionMaxSubject)
		}, AdmitBlocked, "subject-invalid"},
		{"clock missing", func(_ *AgenticAssessment, c *AdmissionContext) {
			c.Now = time.Time{}
		}, AdmitBlocked, "clock-missing"},
		{"category absent from the policy", func(a *AgenticAssessment, _ *AdmissionContext) {
			a.Category = "example-unlisted-category"
		}, AdmitBlocked, "category-absent"},
		{"advice scoped to another subject is ignored", func(a *AgenticAssessment, c *AdmissionContext) {
			*a = withAdvice(*a, *c, AdmitBlocked, 0.99, []AdvisoryDimension{AdviseSemanticRisk})
			a.Advice[0].Scope = "example-org/widgets#13"
		}, AdmitBoundedAgentWork, "advice-ignored:semantic-risk:out-of-scope"},
		{"advice requested for another subject is ignored", func(a *AgenticAssessment, c *AdmissionContext) {
			*a = withAdvice(*a, *c, AdmitBlocked, 0.99, []AdvisoryDimension{AdviseSemanticRisk})
			a.Advice[0].Request.Subject = "example-org/widgets#13"
			a.Advice[0].Prediction.Subject = "example-org/widgets#13"
		}, AdmitBoundedAgentWork, "advice-ignored:semantic-risk:out-of-scope"},
		{"too many operations", func(a *AgenticAssessment, _ *AdmissionContext) {
			for len(a.Operations) <= admissionMaxOperations {
				a.Operations = append(a.Operations, "read")
			}
		}, AdmitBlocked, "input-oversized"},
		{"too many facts", func(a *AgenticAssessment, _ *AdmissionContext) {
			for len(a.Facts) <= admissionMaxFacts {
				a.Facts = append(a.Facts, a.Facts[0])
			}
		}, AdmitBlocked, "input-oversized"},
		{"too much advice", func(a *AgenticAssessment, c *AdmissionContext) {
			dims := make([]AdvisoryDimension, admissionMaxAdvice+1)
			for i := range dims {
				dims[i] = AdviseSemanticRisk
			}
			*a = withAdvice(*a, *c, AdmitBlocked, 0.99, dims)
		}, AdmitBlocked, "input-oversized"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(happy)
			var c admissionCase
			if err := json.Unmarshal(b, &c); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&c.Assessment, &c.Context)
			res := EvaluateAgenticAdmission(base, c.Assessment, c.Context)
			if res.Disposition != tc.want {
				t.Fatalf("disposition = %q, want %q (reasons %q)", res.Disposition, tc.want, res.Reasons)
			}
			found := false
			for _, r := range res.Reasons {
				found = found || r == tc.reason
			}
			if !found {
				t.Errorf("reasons %q lack %q", res.Reasons, tc.reason)
			}
			if pr := res.PolicyResult("sha256:example-input", c.Context.Now); strings.ContainsAny(pr.Reason, "\n\r") ||
				strings.Count(pr.Reason, "; ") != len(res.Reasons)-1 {
				t.Errorf("PolicyResult reason %q carries a forged separator or line", pr.Reason)
			} else if pr.Subject != res.Subject {
				t.Errorf("PolicyResult subject (%d bytes) differs from the result's (%d bytes)", len(pr.Subject), len(res.Subject))
			}
			assertAdmissionFloors(t, res)
		})
	}
}

// TestAgenticAdmissionScopeBoundary pins the boundary-aware scope match shared by
// applicability and the discovery read scope.
func TestAgenticAdmissionScopeBoundary(t *testing.T) {
	cases := []struct {
		scope, subject string
		want           bool
	}{
		{"example-org/widgets", "example-org/widgets", true},
		{"example-org/widgets", "example-org/widgets#1", true},
		{"example-org/widgets", "example-org/widgets/sub#1", true},
		{"example-org/widgets#", "example-org/widgets#1", true},
		{"example-org/widgets#1", "example-org/widgets#1", true},
		{"example-org/widgets", "example-org/widgets-evil#1", false},
		{"example-org/widgets#1", "example-org/widgets#12", false},
		{"example-org/unrelated#999", "example-org/widgets#12", false},
		{"", "example-org/widgets#12", false},
		{" ", " example", false},
	}
	for _, c := range cases {
		if got := scopeCovers(c.scope, c.subject); got != c.want {
			t.Errorf("scopeCovers(%q, %q) = %v, want %v", c.scope, c.subject, got, c.want)
		}
	}
}

// TestAgenticAdmissionGateSeamBounds pins the two refusals the gate seam owns itself: an
// unmapped DISPOSITION verdict holds (as an unmapped brief verdict does), and an
// over-long pattern file is refused before it is read whole.
func TestAgenticAdmissionGateSeamBounds(t *testing.T) {
	partial := PatternRiskInput{Pattern: "partial", Gates: map[string][]string{"low": {"gate-low"}}}
	if gates, err := MandatoryGates(partial, "low", AdmitBlocked); err == nil {
		t.Fatalf("MandatoryGates with an unmapped disposition verdict = %v, nil; want a refusal", gates)
	}
	big := filepath.Join(t.TempDir(), "big-v1.yaml")
	if err := os.WriteFile(big, []byte("schema: workflow-pattern-v1\n"+strings.Repeat("#", admissionMaxPatternLen)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPatternRiskInput(big); err == nil || !strings.Contains(err.Error(), "bound") {
		t.Fatalf("LoadPatternRiskInput(oversized) = %v, want a size refusal", err)
	}
}

// TestAgenticAdmissionSubjectNotEchoed pins spec §7's subject rule on every early return:
// a subject outside the grammar never reaches AdmissionResult.Subject or the projected
// PolicyResult, including on the policy-invalid and stop-flag returns that precede the
// subject check, while a valid subject is carried through (the positive control).
func TestAgenticAdmissionSubjectNotEchoed(t *testing.T) {
	base := loadAdmissionPolicy(t)
	happy := admissionHappy(t)
	hostile := "example-org/widgets#12\nFAKE; forged" + strings.Repeat("x", 1<<20)
	badPolicy := base
	badPolicy.MaxFactAge = 0
	cases := []struct {
		name   string
		policy AdmissionPolicy
		stop   bool
		reason string
	}{
		{"subject check", base, false, "subject-invalid"},
		{"policy-invalid return", badPolicy, false, "policy-invalid"},
		{"stop-flag return", base, true, "stop-flag"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, c := happy.Assessment, happy.Context
			a.Subject = hostile
			c.StopFlag = tc.stop
			res := EvaluateAgenticAdmission(tc.policy, a, c)
			if res.Disposition != AdmitBlocked || len(res.Reasons) == 0 || res.Reasons[0] != tc.reason {
				t.Fatalf("got %q %v, want blocked/%s", res.Disposition, res.Reasons, tc.reason)
			}
			if res.Subject != "" {
				t.Errorf("result echoes the refused subject (%d bytes)", len(res.Subject))
			}
			if pr := res.PolicyResult("sha256:example-input", c.Now); pr.Subject != "" {
				t.Errorf("PolicyResult echoes the refused subject (%d bytes)", len(pr.Subject))
			}
			assertAdmissionFloors(t, res)
		})
	}
	if pr := (AdmissionResult{Subject: hostile}).PolicyResult("sha256:example-input", happy.Context.Now); pr.Subject != "" {
		t.Errorf("PolicyResult projects an invalid subject from a hand-built result (%d bytes)", len(pr.Subject))
	}
	res := EvaluateAgenticAdmission(base, happy.Assessment, happy.Context)
	if res.Subject != happy.Assessment.Subject || res.PolicyResult("sha256:example-input", happy.Context.Now).Subject != res.Subject {
		t.Fatalf("valid subject %q not carried through (result %q)", happy.Assessment.Subject, res.Subject)
	}
}
