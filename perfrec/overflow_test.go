package perfrec

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
)

func overflowRequest(n int64) RequestStats {
	s := RequestStats{Class: RequestProvider, Origin: OriginDirect, Protocol: "graphql", Resource: "graphql", Outcome: "2xx", Attempts: n, PointSamples: n, DurationCoverage: map[Coverage]int64{CoverageMeasured: n}, Points: Points{Coverage: CoverageMeasured, Unit: UnitGitHubPoints, Value: new(int64)}}
	s.DurationBins[0] = n
	return s
}

func overflowRecord(t *testing.T, set func(*Record, int64), n int64) Record {
	t.Helper()
	t.Setenv(EnvVar, "1")
	r := v3Record(v3Context("tool"))
	r.Install, _ = NewID()
	r.WallMs = 0
	set(&r, n)
	if _, e := json.Marshal(r); e != nil {
		t.Fatal("individual input must remain valid:", e)
	}
	return r
}

// regression: #4360
func TestOverflowAggregatesFailClosed(t *testing.T) {
	cases := []struct {
		name string
		set  func(*Record, int64)
	}{
		{"wall", func(r *Record, n int64) { r.WallMs = n }},
		{"cpu", func(r *Record, n int64) { r.CPU, r.CPUMs = CPUMeasured, &n }},
		{"request-counts-bins-coverage", func(r *Record, n int64) { r.V3.Requests = []RequestStats{overflowRequest(n)} }},
		{"request-duration", func(r *Record, n int64) {
			s := overflowRequest(1)
			s.DurationSumMs, s.DurationMaxMs = n, n
			s.DurationBins[0], s.DurationBins[31] = 0, 1
			r.V3.Requests = []RequestStats{s}
		}},
		{"request-bytes", func(r *Record, n int64) {
			s := overflowRequest(1)
			s.Bytes, s.BytesSamples = n, 1
			r.V3.Requests = []RequestStats{s}
		}},
		{"request-points", func(r *Record, n int64) {
			s := overflowRequest(1)
			s.Points.Value = &n
			r.V3.Requests = []RequestStats{s}
		}},
		{"request-drops", func(r *Record, n int64) { r.V3.RequestDrops, r.V3.RequestCoverage = n, CoverageFailed }},
		{"logical-query", func(r *Record, n int64) {
			r.V3.Kind, r.CPU, r.CPUMs, r.Boundary = ObservationClient, CPUUnsupported, nil, BoundaryCaller
			r.V3.Query = &QueryObservation{Operation: "query", LogicalCalls: n, ProviderCoverage: CoverageMissing, Source: "unavailable", Coalescing: "none", Result: "ok"}
		}},
		{"service-transports", func(r *Record, n int64) {
			r.V3.Kind, r.CPU, r.CPUMs, r.Boundary = ObservationClient, CPUUnsupported, nil, BoundaryCaller
			r.V3.Query = &QueryObservation{Operation: "query", TransportAttempts: n, ProviderCoverage: CoverageMissing, Source: "unavailable", Coalescing: "none", Result: "ok"}
		}},
		{"handler-upstream", func(r *Record, n int64) {
			r.V3.Kind, r.CPU, r.CPUMs, r.Boundary = ObservationHandler, CPUUnsupported, nil, BoundaryCaller
			r.V3.Query = &QueryObservation{Operation: "query", ProviderAttempts: &n, ProviderCoverage: CoverageMeasured, Source: "miss", Coalescing: "leader", Result: "ok"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, schema := range []string{SchemaV3, SchemaV4} {
				t.Run(schema, func(t *testing.T) {
					for _, first := range []int64{math.MaxInt64 - 1, math.MaxInt64} {
						records := []Record{overflowRecord(t, tc.set, first), overflowRecord(t, tc.set, 1)}
						if schema == SchemaV4 {
							for i := range records {
								var e error
								records[i], e = ToV4(records[i])
								if e != nil {
									t.Fatal(e)
								}
							}
							s, e := ReduceV4(records, ProfileTool, "tool", V3Filter{})
							if first == math.MaxInt64 {
								if !errors.Is(e, errAggregateOverflow) || !reflect.DeepEqual(s, V4Summary{}) {
									t.Fatalf("overflow exposed summary: %+v / %v", s, e)
								}
							} else if e != nil || s.Records != 2 {
								t.Fatalf("exact boundary rejected: %+v / %v", s, e)
							}
						} else {
							s, e := ReduceV3(records, ProfileTool, "tool", V3Filter{})
							if first == math.MaxInt64 {
								if !errors.Is(e, errAggregateOverflow) || !reflect.DeepEqual(s, V3Summary{}) {
									t.Fatalf("overflow exposed summary: %+v / %v", s, e)
								}
							} else if e != nil || s.Records != 2 {
								t.Fatalf("exact boundary rejected: %+v / %v", s, e)
							}
						}
					}
				})
			}
		})
	}
}

// regression: #4360
func TestOverflowPerRecordCoverageRejected(t *testing.T) {
	for _, kind := range []string{"coverage", "bins"} {
		t.Run(kind, func(t *testing.T) {
			r := overflowRecord(t, func(r *Record, _ int64) { r.V3.Requests = []RequestStats{overflowRequest(1)} }, 0)
			s := &r.V3.Requests[0]
			if kind == "coverage" {
				// Nonnegative mathematical sum is 2^64+1; unchecked int64 wraps to Attempts=1.
				s.DurationCoverage = map[Coverage]int64{CoverageMeasured: 1, CoverageMissing: math.MaxInt64, CoverageUnsupported: math.MaxInt64, CoverageFailed: 2}
			} else {
				s.DurationBins[0], s.DurationBins[1], s.DurationBins[2] = math.MaxInt64, math.MaxInt64, 3
			}
			if e := validV3(r); !errors.Is(e, errAggregateOverflow) {
				t.Fatalf("wrapped %s accepted: %v", kind, e)
			}
			if _, e := json.Marshal(r); e == nil {
				t.Fatal("writer accepted malformed aggregate")
			}
			if _, e := ToV4(r); e == nil {
				t.Fatal("v4 adapter accepted malformed aggregate")
			}
		})
	}
}

// regression: #4360
func TestOverflowExactTotalsPreserved(t *testing.T) {
	for _, schema := range []string{SchemaV3, SchemaV4} {
		t.Run(schema, func(t *testing.T) {
			set := func(r *Record, n int64) {
				r.WallMs, r.CPU, r.CPUMs = n, CPUMeasured, &n
				s := overflowRequest(n)
				s.DurationSumMs, s.DurationMaxMs, s.Bytes, s.BytesSamples, s.Points.Value = n, 1, n, n, &n
				s.DurationBins[0], s.DurationBins[1] = 0, n
				r.V3.Requests = []RequestStats{s}
			}
			records := []Record{overflowRecord(t, set, math.MaxInt64-1), overflowRecord(t, set, 1)}
			var s V3Summary
			var e error
			if schema == SchemaV4 {
				for i := range records {
					records[i], e = ToV4(records[i])
					if e != nil {
						t.Fatal(e)
					}
				}
				v, err := ReduceV4(records, ProfileTool, "tool", V3Filter{})
				s, e = v.V3Summary, err
			} else {
				s, e = ReduceV3(records, ProfileTool, "tool", V3Filter{})
			}
			if e != nil || s.Records != 2 || len(s.Groups) != 1 || len(s.Requests) != 1 {
				t.Fatalf("%+v / %v", s, e)
			}
			g, q := s.Groups[0], s.Requests[0]
			for _, n := range []int64{g.SumOwnProcessWallMs, g.SumOwnCPUMs, q.Attempts, q.PointSamples, q.DurationSumMs, q.DurationBins[1], q.DurationCoverage[CoverageMeasured], q.Bytes, q.BytesSamples, *q.Points.Value} {
				if n != math.MaxInt64 {
					t.Fatalf("exact aggregate changed to %d", n)
				}
			}
		})
	}
}
