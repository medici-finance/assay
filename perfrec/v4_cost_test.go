package perfrec

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
)

// regression: #4360
func TestV4ReaderCost(t *testing.T) {
	t.Setenv(EnvVar, "1")
	const records = 200
	var old, current [][]byte
	for i := 0; i < records; i++ {
		r := v3Record(v3Context("tool"))
		r.Install = fmt.Sprintf("%032x", i+1)
		r.V3.Attribution.InvocationID = fmt.Sprintf("%032x", i+1001)
		r.TS = "2026-01-01T00:00:00Z"
		r.WallMs = 42
		b, e := json.Marshal(r)
		if e != nil {
			t.Fatal(e)
		}
		old = append(old, b)
		r, e = ToV4(r)
		if e != nil {
			t.Fatal(e)
		}
		b, e = json.Marshal(r)
		if e != nil {
			t.Fatal(e)
		}
		current = append(current, b)
	}
	type result struct {
		Schema      string  `json:"schema"`
		Records     int     `json:"records"`
		Bytes       int     `json:"bytes"`
		SHA256      string  `json:"sha256"`
		FirstMs     float64 `json:"firstInProcessMs"`
		WarmP50Ms   float64 `json:"warmP50Ms"`
		WarmP95Ms   float64 `json:"warmP95Ms"`
		OwnCPUMs    int64   `json:"ownCpuMs"`
		CPUCoverage string  `json:"cpuCoverage"`
	}
	var out []result
	cohorts := []struct {
		name  string
		lines [][]byte
	}{{SchemaV3, old}, {SchemaV4, current}}
	if os.Getenv("PERFREC_TEST_V4_FIRST") == "1" {
		cohorts[0], cohorts[1] = cohorts[1], cohorts[0]
	}
	for _, cohort := range cohorts {
		s := result{Schema: cohort.name, Records: records, CPUCoverage: CPUUnsupported}
		h := sha256.New()
		for _, b := range cohort.lines {
			s.Bytes += len(b) + 1
			h.Write(b)
			h.Write([]byte{'\n'})
		}
		s.SHA256 = fmt.Sprintf("%x", h.Sum(nil))
		probe := SystemProbe()
		var before int64
		var cpuOK bool
		if probe.CPU != nil {
			before, cpuOK = probe.CPU()
		}
		var warm []float64
		for sample := 0; sample < 16; sample++ {
			start := time.Now()
			rs := make([]Record, 0, records)
			for _, b := range cohort.lines {
				r, e := Decode(b)
				if e != nil {
					t.Fatal(e)
				}
				rs = append(rs, r)
			}
			var count int
			var wall int64
			if cohort.name == SchemaV3 {
				s, e := ReduceV3(rs, ProfileTool, "tool", V3Filter{})
				if e != nil {
					t.Fatal(e)
				}
				count = s.Records
				wall = s.Groups[0].SumOwnProcessWallMs
			} else {
				s, e := ReduceV4(rs, ProfileTool, "tool", V3Filter{})
				if e != nil {
					t.Fatal(e)
				}
				count = s.Records
				wall = s.Groups[0].SumOwnProcessWallMs
			}
			if count != records || wall != records*42 {
				t.Fatal("measurement cohort lost facts")
			}
			ms := float64(time.Since(start).Nanoseconds()) / 1e6
			if sample == 0 {
				s.FirstMs = ms
			} else {
				warm = append(warm, ms)
			}
		}
		if cpuOK {
			if after, ok := probe.CPU(); ok && after >= before {
				s.OwnCPUMs = after - before
				s.CPUCoverage = CPUMeasured
			}
		}
		sort.Float64s(warm)
		s.WarmP50Ms = warm[len(warm)/2]
		s.WarmP95Ms = warm[len(warm)-1]
		out = append(out, s)
	}
	b, _ := json.Marshal(out)
	t.Logf("CODEC_COST %s; fixture read+reduce only, first-in-process versus warm; not filesystem-cold or whole-tool savings", b)
}
