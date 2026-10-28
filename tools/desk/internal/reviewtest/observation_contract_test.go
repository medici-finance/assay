package reviewtest_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

func observationFixture() (rf.ObservationInputs, rf.ObservationPolicy) {
	now := time.Date(2026, 10, 17, 12, 1, 0, 0, time.UTC)
	repo := rf.Repository{Provider: "github", Host: "github.com", Repository: "example-org/tracker"}
	p := rf.InputProvenance{Source: "fixture-provider", Version: "fixture-v1", Revision: "set-1", ObservedAt: now.Add(-time.Minute), Freshness: "fresh", Complete: true}
	id := rf.ChangeIdentity{Repository: repo, Number: 1, Head: strings.Repeat("a", 40), Base: strings.Repeat("b", 40), BaseUsed: true}
	applies := rf.Applicability{Head: id.Head, Base: id.Base, Body: p.Revision, Reviews: p.Revision, Checks: p.Revision}
	change := rf.ObservedChange{Identity: id, Inputs: rf.ReviewInputs{Subject: p, Body: p, Reviews: p, Checks: p, Enrichment: p}, AppliesTo: applies, Score: 10,
		Facts: rf.ClassificationFacts{Change: rf.ChangeFact{Title: "fixture", MergeStateStatus: "CLEAN", Labels: []string{}, Checks: []rf.CheckFact{{Status: "COMPLETED", Conclusion: "SUCCESS"}}}, CIRequired: true},
		Lanes: []rf.LaneConclusion{{Name: "correctness", State: "missing", AppliesTo: applies}, {Name: "security", State: "missing", AppliesTo: applies}}}
	return rf.ObservationInputs{Evaluator: rf.ObservationEvaluator, Policy: "fixture-policy-v1", ReviewerDisplay: "fixture-reviewer", Requested: []rf.Repository{repo}, Admitted: []rf.Repository{repo}, Collection: rf.CollectionInterval{Start: now.Add(-2 * time.Minute), End: now.Add(-time.Minute)}, Population: []rf.PopulationCoverage{{Repository: repo, Input: p, Count: 1}}, Changes: []rf.ObservedChange{change}}, rf.ObservationPolicy{Now: now, Oldest: now.Add(-5 * time.Minute), Policy: "fixture-policy-v1"}
}
func approve(c *rf.ObservedChange) {
	c.Facts.Reviews = rf.ReduceReviews([]rf.ReviewFact{{Author: "fixture-reviewer", Reviewer: true, State: "APPROVED", CommitID: c.Identity.Head, SubmittedAt: "2026-10-17T11:00:00Z"}}, c.Identity.Head)
	c.Lanes[0].State = "approved"
}
func observation(t *testing.T, in rf.ObservationInputs, p rf.ObservationPolicy) rf.Observation {
	t.Helper()
	o, e := rf.BuildObservation(in, p)
	if e != nil {
		t.Fatal(e)
	}
	return o
}
func expectInputError(t *testing.T, err error, kind rf.ErrorKind) {
	t.Helper()
	var ie *rf.InputError
	if !errors.As(err, &ie) || ie.Kind != kind {
		t.Fatalf("want %s input error, got %v", kind, err)
	}
}

func TestReviewContractFlow(t *testing.T) {
	t.Run("classifier-oracle", func(t *testing.T) {
		// Expected decisions are authored here, not calculated from the classifier.
		cases := []struct {
			name, action, disposition string
			edit                      func(*rf.ObservedChange)
		}{
			{"unreviewed", "NEEDS-REVIEW", "DISPATCH", func(c *rf.ObservedChange) {}},
			{"approved", "MERGE-NOW", "SURFACE", approve},
			{"draft", "FLIP", "FLIP-VERB", func(c *rf.ObservedChange) {
				approve(c)
				c.Facts.Change.IsDraft = true
				c.Facts.Change.MergeStateStatus = "UNKNOWN"
			}},
			{"security-missing", "SECURITY-REVIEW-REQUIRED", "SURFACE", func(c *rf.ObservedChange) { approve(c); c.Facts.RiskClassed = true }},
			{"blocked", "BLOCKED", "WAIT", func(c *rf.ObservedChange) {
				c.Facts.Reviews = rf.ReduceReviews([]rf.ReviewFact{{Author: "reviewer", Reviewer: true, State: "CHANGES_REQUESTED", CommitID: c.Identity.Head}}, c.Identity.Head)
				c.Lanes[0].State = "blocked"
			}},
			{"pending", "WAIT-CI", "WAIT", func(c *rf.ObservedChange) {
				approve(c)
				c.Facts.Change.IsDraft = true
				c.Facts.Change.Checks = []rf.CheckFact{{Status: "IN_PROGRESS"}}
			}},
			{"red", "CI-RED", "SURFACE", func(c *rf.ObservedChange) {
				approve(c)
				c.Facts.Change.IsDraft = true
				c.Facts.Change.Checks = []rf.CheckFact{{Status: "COMPLETED", Conclusion: "FAILURE"}}
			}},
			{"empty-checks", "CI-UNVERIFIED", "SURFACE", func(c *rf.ObservedChange) { approve(c); c.Facts.Change.Checks = []rf.CheckFact{} }},
			{"unknown-check", "CI-UNKNOWN", "SURFACE", func(c *rf.ObservedChange) {
				approve(c)
				c.Facts.Change.Checks = []rf.CheckFact{{Status: "COMPLETED"}}
			}},
			{"benign-advance", "MERGE-CURR", "NO-OP", func(c *rf.ObservedChange) {
				c.Facts.Reviews = rf.ReduceReviews([]rf.ReviewFact{{Reviewer: true, State: "APPROVED", CommitID: strings.Repeat("c", 40)}}, c.Identity.Head)
				c.Lanes[0].State = "stale"
			}},
			{"own-change", "RE-REVIEW", "DISPATCH", func(c *rf.ObservedChange) {
				c.Facts.Reviews = rf.ReduceReviews([]rf.ReviewFact{{Reviewer: true, State: "APPROVED", CommitID: strings.Repeat("c", 40)}}, c.Identity.Head)
				c.Facts.OwnFilesChanged = true
				c.Lanes[0].State = "stale"
			}},
			{"body-human-gate", "HUMAN-GATE", "SURFACE", func(c *rf.ObservedChange) { approve(c); c.Facts.Change.Body = "Gate: human" }},
			{"body-reverified", "MERGE-NOW", "SURFACE", func(c *rf.ObservedChange) { approve(c); c.Facts.Reviews.BodyEditReverified = true }},
			{"same-head-block", "SUSPECT-APPROVAL", "SURFACE", func(c *rf.ObservedChange) {
				c.Facts.Reviews = rf.ReduceReviews([]rf.ReviewFact{{Reviewer: true, State: "CHANGES_REQUESTED", CommitID: c.Identity.Head}, {Reviewer: true, State: "APPROVED", CommitID: c.Identity.Head}}, c.Identity.Head)
				c.Lanes[0].State = "blocked"
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				in, p := observationFixture()
				tc.edit(&in.Changes[0])
				o := observation(t, in, p)
				plan, e := rf.PlanObservation(o, p)
				if e != nil {
					t.Fatal(e)
				}
				row := plan.Result.Rows[0]
				if tc.name == "body-reverified" && !strings.Contains(row.Note, " — same-head APPROVE accepted under the documented body-edit re-verification class (deskflip re-verifies it at the gate)") {
					t.Fatal("body-edit re-verification note missing")
				}
				if row.Action != tc.action || row.Disposition != tc.disposition {
					t.Fatalf("decision %s/%s; want %s/%s", row.Action, row.Disposition, tc.action, tc.disposition)
				}
				if (tc.disposition == "WAIT" || tc.disposition == "SURFACE" || tc.disposition == "NO-OP") && (len(row.Holds) != 1 || len(plan.Verbs) != 0) {
					t.Fatal("non-verb row/hold disappeared or gained an effect")
				}
			})
		}
	})
	t.Run("lanes", func(t *testing.T) {
		in, p := observationFixture()
		c := &in.Changes[0]
		approve(c)
		c.Facts.RiskClassed = true
		o := observation(t, in, p)
		if !o.Result.Rows[0].Lanes[0].Satisfied || o.Result.Rows[0].Lanes[1].Satisfied {
			t.Fatal("correctness approval leaked into security lane")
		}
		// The preserved advisory reducer can aggregate a security approval. It does
		// not supply an independent correctness approval to the observation.
		c.Lanes[0].State = "blocked"
		c.Lanes[1].State = "approved"
		c.Facts.Reviews.SecurityPass = true
		o = observation(t, in, p)
		if o.Result.Rows[0].Lanes[0].Satisfied || !o.Result.Rows[0].Lanes[1].Satisfied {
			t.Fatal("aggregate approval replaced independent lane conclusions")
		}
		o.Result.Rows[0].Lanes[0].Satisfied = true
		_, e := rf.PlanObservation(o, p)
		expectInputError(t, e, rf.SchemaError)
		c.Facts.Reviews.SecurityPass = false
		_, e = rf.BuildObservation(in, p)
		expectInputError(t, e, rf.SchemaError)
	})
	t.Run("board-row-parity", func(t *testing.T) {
		// A planned Row must equal the Row ReadBoard builds from the same decision,
		// so a later consumer cannot read a risk-classed change as "no security lane".
		for _, risk := range []bool{false, true} {
			in, p := observationFixture()
			in.Changes[0].Facts.RiskClassed = risk
			o := observation(t, in, p)
			plan, e := rf.PlanObservation(o, p)
			if e != nil {
				t.Fatal(e)
			}
			if len(plan.Verbs) != 1 {
				t.Fatalf("want one planned verb, got %d", len(plan.Verbs))
			}
			r := o.Result.Rows[0]
			actions, _ := json.Marshal(rf.ActionsReport{AsOf: p.Now.Format(time.RFC3339), Rows: []rf.ActionRow{{Repo: r.Identity.Repository.Repository, Number: r.Identity.Number, Title: r.Title, Action: r.Action, RiskClassed: risk, Score: r.Score}}})
			prs, _ := json.Marshal(rf.PRsReport{Rows: []rf.PRRow{{Repo: r.Identity.Repository.Repository, Number: r.Identity.Number, HeadSHA: r.Identity.Head}}})
			b, e := rf.ReadBoard(actions, prs)
			if e != nil {
				t.Fatal(e)
			}
			got, want := plan.Verbs[0].Row, b.Rows[0]
			// ReadBoard is the single-forge legacy wire: Provider/Host are empty there.
			got.Provider, got.Host = "", ""
			if got.RiskClassed != risk || !reflect.DeepEqual(got, want) {
				t.Fatalf("observation plan row %+v differs from ReadBoard row %+v (riskClassed want %v)", got, want, risk)
			}
		}
	})
	t.Run("applicability", func(t *testing.T) {
		for _, name := range []string{"head", "base", "body", "reviews", "checks", "lane-head", "lane-body", "lane-checks"} {
			t.Run(name, func(t *testing.T) {
				in, p := observationFixture()
				c := &in.Changes[0]
				switch name {
				case "head":
					c.Identity.Head = strings.Repeat("d", 40)
				case "base":
					c.Identity.Base = strings.Repeat("e", 40)
				case "body":
					c.Inputs.Body.Revision = "body-2"
				case "reviews":
					c.Inputs.Reviews.Revision = "reviews-2"
				case "checks":
					c.Inputs.Checks.Revision = "run-2"
				case "lane-head":
					c.Lanes[0].AppliesTo.Head = strings.Repeat("f", 40)
				case "lane-body":
					c.Lanes[0].AppliesTo.Body = "old-body"
				case "lane-checks":
					c.Lanes[1].AppliesTo.Checks = "old-run"
				}
				_, e := rf.BuildObservation(in, p)
				expectInputError(t, e, rf.StaleError)
			})
		}
	})
	t.Run("population-order-and-scope", func(t *testing.T) {
		in, p := observationFixture()
		base := in.Changes[0]
		in.Changes = nil
		for i := 0; i < 151; i++ {
			c := base
			c.Identity.Number = i + 1
			c.Score = i
			in.Changes = append(in.Changes, c)
		}
		in.Population[0].Count = 151
		o := observation(t, in, p)
		plan, e := rf.PlanObservation(o, p)
		if e != nil {
			t.Fatal(e)
		}
		if len(plan.Result.Rows) != 151 || plan.Result.Depths.Total != 151 || plan.Result.Depths.Actions["NEEDS-REVIEW"] != 151 || plan.Result.Depths.Dispositions["DISPATCH"] != 151 || len(plan.Verbs) != 151 || plan.Idle.State != rf.IdleNo {
			t.Fatal("display cap or empty result replaced full population")
		}
		if plan.Result.Rows[0].Identity.Number != 151 || plan.Result.Rows[150].Rank != 151 || !plan.Result.Depths.Cutoff.Equal(in.Collection.End) {
			t.Fatal("rank/order/cutoff lost")
		}
		in, p = observationFixture()
		other := in.Admitted[0]
		other.Host = "forge.example"
		in.Requested = append(in.Requested, other)
		in.Admitted = append(in.Admitted, other)
		pop := in.Population[0]
		pop.Repository = other
		in.Population = append(in.Population, pop)
		c := in.Changes[0]
		c.Identity.Repository = other
		in.Changes = append(in.Changes, c)
		plan, e = rf.PlanObservation(observation(t, in, p), p)
		if e != nil {
			t.Fatal(e)
		}
		if len(plan.Verbs) != 2 || plan.Verbs[0].Key == plan.Verbs[1].Key {
			t.Fatal("qualified hosts coalesced")
		}
		in.Changes[1].Identity.Repository.Host = in.Changes[0].Identity.Repository.Host
		in.Admitted[1].Host = in.Admitted[0].Host
		in.Requested[1].Host = in.Requested[0].Host
		in.Population[1].Repository.Host = in.Population[0].Repository.Host
		in.Changes[1].Identity.Repository.Provider = "gitlab"
		in.Admitted[1].Provider = "gitlab"
		in.Requested[1].Provider = "gitlab"
		in.Population[1].Repository.Provider = "gitlab"
		plan, e = rf.PlanObservation(observation(t, in, p), p)
		if e != nil || len(plan.Verbs) != 2 {
			t.Fatal("qualified providers coalesced", e)
		}
	})
	t.Run("idle-and-contradictions", func(t *testing.T) {
		in, p := observationFixture()
		in.Changes = []rf.ObservedChange{}
		in.Population[0].Count = 0
		o := observation(t, in, p)
		plan, e := rf.PlanObservation(o, p)
		if e != nil || plan.Idle.State != rf.IdleYes {
			t.Fatal("measured empty not idle", e)
		}
		o.Inputs.Population[0].Input.Complete = false
		plan, e = rf.PlanObservation(o, p)
		expectInputError(t, e, rf.PartialError)
		if plan.Idle.State == rf.IdleYes {
			t.Fatal("partial became idle")
		}
		in, p = observationFixture()
		o = observation(t, in, p)
		o.Result.Depths.Total = 0
		_, e = rf.PlanObservation(o, p)
		expectInputError(t, e, rf.SchemaError)
		o = observation(t, in, p)
		o.Result.Rows[0].Action = "MERGE-NOW"
		_, e = rf.PlanObservation(o, p)
		expectInputError(t, e, rf.SchemaError)
		p.Oldest = p.Now.Add(-30 * time.Second)
		_, e = rf.PlanObservation(observation(t, in, func() rf.ObservationPolicy { _, orig := observationFixture(); return orig }()), p)
		expectInputError(t, e, rf.StaleError)
	})
	t.Run("legacy-command-oracle", func(t *testing.T) {
		var cases []oracleCase
		fixture(t, "oracle.json", &cases)
		bin := build(t)
		for _, c := range cases {
			if c.Tool != "deskboard" && c.Tool != "reviewloop" {
				continue
			}
			t.Run(c.Name, func(t *testing.T) {
				got, _ := probe(t, bin, c.Tool, c.Request, t.TempDir())
				if e := compare(got, c.Want); e != nil {
					t.Fatal(e)
				}
			})
		}
	})
	t.Run("mutation-controls", func(t *testing.T) {
		contractMutation(t, "internal/reviewflow/coalesce.go", "Provider: r.Provider, Host: r.Host, Repo: r.Repo", "Provider: \"\", Host: r.Host, Repo: r.Repo", "^TestReviewContractFlow$/^population-order-and-scope$", "qualified providers coalesced")
		contractMutation(t, "internal/reviewflow/observation.go", "Body: v.Inputs.Body.Revision", "Body: v.AppliesTo.Body", "^TestReviewContractFlow$/^applicability$/^body$", "want stale input error")
		contractMutation(t, "internal/reviewflow/idlegate.go", "if v.NeedsReview > 0 || v.ReReview > 0 {", "if false && (v.NeedsReview > 0 || v.ReReview > 0) {", "^TestReviewContractFlow$/^population-order-and-scope$", "display cap or empty result")
	})
}

// Compile a real production-source overlay and require the designated behavioral
// assertion to fail. Compilation errors and unrelated failures are not kills.
func contractMutation(t *testing.T, relative, old, replacement, run, diagnostic string, packages ...string) {
	t.Helper()
	root := moduleRoot(t)
	source := filepath.Join(root, relative)
	b, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Count(string(b), old) != 1 {
		t.Fatalf("mutation anchor %q must be unique", old)
	}
	dir := t.TempDir()
	mutant := filepath.Join(dir, "mutant.go")
	if e = os.WriteFile(mutant, []byte(strings.Replace(string(b), old, replacement, 1)), 0600); e != nil {
		t.Fatal(e)
	}
	overlay, _ := json.Marshal(map[string]any{"Replace": map[string]string{source: mutant}})
	path := filepath.Join(dir, "overlay.json")
	if e = os.WriteFile(path, overlay, 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pkg := "./internal/reviewtest"
	if len(packages) > 0 {
		pkg = packages[0]
	}
	cmd := exec.CommandContext(ctx, "go", "test", "-overlay", path, "-count=1", "-timeout=60s", "-v", "-run", run, pkg)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off", "GOFLAGS=-overlay="+path, "ASSAY_REVIEW_CONTRACT_MUTANT=1")
	out, e := goquiet.Cmd(cmd).CombinedOutput()
	if e == nil || !strings.Contains(string(out), diagnostic) || !strings.Contains(string(out), "--- FAIL: "+strings.Split(strings.TrimPrefix(run, "^"), "$")[0]) {
		t.Fatalf("mutation did not fail designated assertion: %v\n%s", e, out)
	}
	t.Logf("killed production mutation: %s (%s)", relative, diagnostic)
}

func TestReviewContractRoundTrip(t *testing.T) {
	in, p := observationFixture()
	o := observation(t, in, p)
	data, e := json.Marshal(o)
	if e != nil {
		t.Fatal(e)
	}
	decoded, e := rf.DecodeObservation(data)
	if e != nil {
		t.Fatal(e)
	}
	plan, e := rf.PlanObservation(decoded, p)
	if e != nil || !reflect.DeepEqual(plan.Result, o.Result) {
		t.Fatal("round trip changed result", e)
	}

}
