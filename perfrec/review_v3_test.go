package perfrec

import (
	"encoding/json"
	"testing"
)

// regression: class perf-v3-aggregate-boundary
func TestV3ReviewBoundaries(t *testing.T) {
	t.Setenv(EnvVar, "1")
	observed := func(kind ObservationKind) Record {
		s := BeginObservation(Probe{}, nil, v3Context("tool"), kind)
		ms := int64(5)
		if !s.ObserveRequest(RequestObservation{Class: RequestProvider, Origin: OriginDirect, Protocol: "graphql", Resource: "graphql", Outcome: "2xx", DurationMs: &ms, DurationCoverage: CoverageMeasured, Points: Points{Coverage: CoverageUnknown, Unit: UnitGitHubPoints}}) {
			t.Fatal("fixture refused")
		}
		r := s.Finish("tool", "query", "", 0, "")
		r.Install, _ = NewID()
		return r
	}
	t.Run("kind separation", func(t *testing.T) {
		summary, err := ReduceV3([]Record{observed(ObservationProcess), observed(ObservationProvider), observed(ObservationProvider)}, ProfileTool, "tool", V3Filter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(summary.Requests) != 2 {
			t.Fatal("requests merged across observation kinds")
		}
		raw, _ := json.Marshal(summary.Requests)
		var groups []map[string]any
		_ = json.Unmarshal(raw, &groups)
		kinds := map[any]bool{}
		for _, g := range groups {
			kinds[g["observationKind"]] = true
			if g["observationKind"] == string(ObservationProvider) && g["attempts"] != float64(2) {
				t.Fatal("same-kind attempts failed to merge")
			}
		}
		if !kinds[string(ObservationProcess)] || !kinds[string(ObservationProvider)] {
			t.Fatal("request kind not reported")
		}
	})
	for _, name := range []string{"no samples", "max exceeds sum", "sum exceeds samples"} {
		t.Run(name, func(t *testing.T) {
			r := observed(ObservationProvider)
			s := &r.V3.Requests[0]
			switch name {
			case "no samples":
				s.DurationCoverage = map[Coverage]int64{CoverageMissing: 1}
				s.DurationBins = [32]int64{}
			case "max exceeds sum":
				s.DurationMaxMs = 6
			case "sum exceeds samples":
				s.DurationSumMs = 11
			}
			if (&Writer{}).Write(t.TempDir(), r) == nil {
				t.Fatal("writer accepted impossible duration aggregate")
			}
		})
	}
	t.Run("empty work writer", func(t *testing.T) {
		r := v3Record(v3Context("workflow"))
		r.V3.Attribution.Coverage[FieldWork] = CoverageMeasured
		if (&Writer{}).Write(t.TempDir(), r) == nil {
			t.Fatal("writer accepted empty measured work")
		}
	})
	t.Run("work filter", func(t *testing.T) {
		if _, err := ReduceV3(nil, ProfileWorkflow, "tool", V3Filter{Work: &WorkRef{Kind: "arbitrary"}}); err == nil {
			t.Fatal("invalid work filter accepted")
		}
	})
	for raw, want := range map[string]Coverage{"[]": CoverageMissing, "null": CoverageInvalid} {
		t.Run("empty work "+raw, func(t *testing.T) {
			c := ResolveContext(ContextInput{Profile: Value{"workflow", true}, LookupEnv: func(k string) (string, bool) {
				if k == EnvVar {
					return "1", true
				}
				if k == "ASSAY_PERF_WORK" {
					return raw, true
				}
				return "", false
			}})
			if c.Data().Coverage[FieldWork] != want || len(c.Data().Work) != 0 {
				t.Fatal("empty work claimed measured coverage")
			}
		})
	}
}
