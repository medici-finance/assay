package perfrec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func v4Record(t *testing.T, profile string) Record {
	t.Helper()
	t.Setenv(EnvVar, "1")
	r := v3Record(v3Context(profile))
	r.Install, _ = NewID()
	r, e := ToV4(r)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func v4Wire(t *testing.T, r Record) map[string]any {
	t.Helper()
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	var m map[string]any
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	return m
}
func v4Decode(t *testing.T, m map[string]any) (Record, error) {
	t.Helper()
	b, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	return Decode(b)
}
func v4Extensions(m map[string]any) map[string]any { return m["extensions"].(map[string]any) }

// regression: #4360
func TestV4WireCompatibility(t *testing.T) {
	for _, profile := range []string{"tool", "workflow"} {
		t.Run(profile, func(t *testing.T) {
			r := v4Record(t, profile)
			m := v4Wire(t, r)
			if _, ok := m["v3"]; ok {
				t.Fatal("nested v3 leaked")
			}
			if _, ok := m["counters"]; ok {
				t.Fatal("extension duplicated in core")
			}
			ex := v4Extensions(m)
			want := 3
			if profile == "workflow" {
				want = 4
			}
			if len(ex) != want {
				t.Fatalf("extensions=%d", len(ex))
			}
			if _, ok := ex["attribution"]; ok != (profile == "workflow") {
				t.Fatal("profile linkage")
			}
			got, e := v4Decode(t, m)
			if e != nil {
				t.Fatal(e)
			}
			out, e := json.Marshal(got)
			if e != nil {
				t.Fatal(e)
			}
			before, _ := json.Marshal(r)
			if !bytes.Equal(before, out) {
				t.Fatal("typed roundtrip changed")
			}
			s, e := ReduceV4([]Record{got, got}, ProfileWorkflow, "tool", V3Filter{})
			if e != nil || s.Records != 1 || s.Duplicates != 1 || s.QueryNotApplicable != 1 {
				t.Fatalf("%+v %v", s, e)
			}
		})
	}
	// Existing raw fixtures must retain their exact historical codec interpretation.
	for _, schema := range []string{Schema, SchemaV2, SchemaV3} {
		b, e := os.ReadFile(filepath.Join("testdata", schema+".jsonl"))
		if e != nil {
			t.Fatal(e)
		}
		for _, line := range bytes.Split(bytes.TrimSpace(b), []byte("\n")) {
			r, e := Decode(line)
			if e != nil {
				t.Fatal(schema, e)
			}
			if r.Schema != schema || r.V4 != nil {
				t.Fatal("historical reinterpretation")
			}
		}
	}
}

// regression: #4360
func TestV4ExtensionsFailClosed(t *testing.T) {
	base := v4Record(t, "workflow")
	for _, facet := range []string{"process", "attribution", "requests", "query"} {
		for _, mode := range []string{"missing", "unsupported"} {
			t.Run(facet+mode, func(t *testing.T) {
				m := v4Wire(t, base)
				ex := v4Extensions(m)
				if mode == "missing" {
					delete(ex, facet)
				} else {
					ex[facet] = map[string]any{"version": 2, "data": map[string]any{"private-untrusted-name": "never echo this"}}
				}
				r, e := v4Decode(t, m)
				if e != nil {
					t.Fatal(e)
				}
				s, e := ReduceV4([]Record{r}, ProfileWorkflow, "tool", V3Filter{})
				if e != nil || s.Records != 1 {
					t.Fatal(s, e)
				}
				if mode == "unsupported" && s.UnsupportedExtensions != 1 {
					t.Fatal("unsupported coverage lost")
				}
				raw, _ := json.Marshal(r)
				if bytes.Contains(raw, []byte("private-untrusted-name")) {
					t.Fatal("raw unknown payload escaped")
				}
				if facet == "attribution" {
					r.Role = "verify"
					if _, e := json.Marshal(r); e != nil {
						t.Fatal("independent core lost")
					}
					kept, _, e := SelectV4([]Record{r}, ProfileWorkflow, V3Filter{Role: "verify"})
					if e != nil || len(kept) != 0 {
						t.Fatal("core role supplied unavailable attribution")
					}
				}
			})
		}
	}
	bad := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing core wall", func(m map[string]any) { delete(m, "wallMs") }},
		{"missing overlap", func(m map[string]any) {
			delete(v4Extensions(m)["process"].(map[string]any)["data"].(map[string]any), "phaseOverlap")
		}},
		{"core unknown", func(m map[string]any) { m["path"] = "private" }},
		{"core case", func(m map[string]any) { m["Tool"] = m["tool"]; delete(m, "tool") }},
		{"core case duplicate", func(m map[string]any) { m["Tool"] = m["tool"] }},
		{"known payload case duplicate", func(m map[string]any) {
			p := v4Extensions(m)["process"].(map[string]any)["data"].(map[string]any)
			p["PhaseOverlap"] = true
		}},
		{"null core", func(m map[string]any) { m["role"] = nil }},
		{"unknown major", func(m map[string]any) { m["schema"] = "perf-v5" }},
		{"invalid profile", func(m map[string]any) { m["profile"] = "future" }},
		{"null data", func(m map[string]any) { v4Extensions(m)["process"] = map[string]any{"version": 1, "data": nil} }},
		{"null version", func(m map[string]any) { v4Extensions(m)["process"].(map[string]any)["version"] = nil }},
		{"null query", func(m map[string]any) {
			v4Extensions(m)["query"].(map[string]any)["data"] = map[string]any{"query": nil}
		}},
		{"known payload case", func(m map[string]any) {
			p := v4Extensions(m)["process"].(map[string]any)["data"].(map[string]any)
			p["Phases"] = p["phases"]
			delete(p, "phases")
		}},
		{"known unknown member", func(m map[string]any) {
			v4Extensions(m)["requests"].(map[string]any)["data"].(map[string]any)["url"] = "private"
		}},
		{"missing coverage", func(m map[string]any) {
			delete(v4Extensions(m)["requests"].(map[string]any)["data"].(map[string]any), "requestCoverage")
		}},
		{"invalid coverage", func(m map[string]any) {
			v4Extensions(m)["requests"].(map[string]any)["data"].(map[string]any)["requestCoverage"] = "measurd"
		}},
		{"duplicate invocation", func(m map[string]any) {
			v4Extensions(m)["attribution"].(map[string]any)["data"].(map[string]any)["invocationId"] = m["invocationId"]
		}},
		{"tool linkage", func(m map[string]any) { m["profile"] = "tool" }},
		{"role disagreement", func(m map[string]any) {
			m["role"] = "verify"
			v4Extensions(m)["attribution"].(map[string]any)["data"].(map[string]any)["role"] = "review"
		}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			m := v4Wire(t, base)
			tc.mutate(m)
			_, e := v4Decode(t, m)
			if e == nil {
				t.Fatal("malformed known data accepted")
			}
			if strings.Contains(e.Error(), "private") {
				t.Fatal("private diagnostics")
			}
		})
	}
	// Duplicate keys must be rejected even inside discarded unknown extension data.
	raw, _ := json.Marshal(base)
	for _, b := range [][]byte{append(raw, []byte(" {}")...), bytes.Replace(raw, []byte(`"schema":"perf-v4"`), []byte(`"schema":"perf-v4","schema":"perf-v4"`), 1), bytes.Replace(raw, []byte(`"phaseOverlap":false`), []byte(`"phaseOverlap":false,"phaseOverlap":false`), 1), bytes.Replace(raw, []byte(`"extensions":{`), []byte(`"extensions":{"future":{"version":99,"data":{"x":1,"x":2}},`), 1)} {
		if _, e := Decode(b); e == nil {
			t.Fatal("ambiguous JSON accepted")
		}
	}
	// A late historical alias must not select the permissive old decoder before
	// the strict v4 validator sees the envelope (including escaped marker text).
	legacy := v3Record(v3Context("tool"))
	legacy.Schema = SchemaV2
	legacy.V3 = nil
	lraw, e := json.Marshal(legacy)
	if e != nil {
		t.Fatal(e)
	}
	for _, marker := range []string{`"schema":"perf-v4","Schema":"perf-v2"`, `"Schema":"perf-v4","schema":"perf-v2"`, `"schema":"perf-\u00764","Schema":"perf-v2"`, `"schema":"perf-v4","schema":"perf-v2"`} {
		b := bytes.Replace(lraw, []byte(`"schema":"perf-v2"`), []byte(marker), 1)
		if _, e := Decode(b); e == nil {
			t.Fatal("ambiguous discriminator downgraded v4 validation")
		}
	}
}

// regression: #4360
func TestV4CodecBounds(t *testing.T) {
	base := v4Record(t, "tool")
	for _, version := range []any{0, -1, 2147483648, 1.5, "1", nil} {
		m := v4Wire(t, base)
		v4Extensions(m)["future"] = map[string]any{"version": version, "data": map[string]any{}}
		if _, e := v4Decode(t, m); e == nil {
			t.Fatalf("version %v accepted", version)
		}
	}
	for _, name := range []string{"Future", "x_y", strings.Repeat("x", 33), ""} {
		m := v4Wire(t, base)
		v4Extensions(m)[name] = map[string]any{"version": 1, "data": map[string]any{}}
		if _, e := v4Decode(t, m); e == nil {
			t.Fatal("name accepted")
		}
	}
	m := v4Wire(t, base)
	ex := v4Extensions(m)
	for i := len(ex); i < MaxV4Extensions; i++ {
		ex[fmt.Sprintf("future-%d", i)] = map[string]any{"version": MaxV4ExtensionVersion, "data": map[string]any{}}
	}
	if _, e := v4Decode(t, m); e != nil {
		t.Fatal("legal extension bound", e)
	}
	ex["ninth"] = map[string]any{"version": 1, "data": map[string]any{}}
	if _, e := v4Decode(t, m); e == nil {
		t.Fatal("extension overflow")
	}
	for _, depth := range []int{MaxV4Depth, MaxV4Depth + 1} {
		raw := []byte(strings.Repeat(`{"a":`, depth) + `0` + strings.Repeat("}", depth))
		_, e := boundedJSON(raw)
		if (e == nil) != (depth == MaxV4Depth) {
			t.Fatalf("depth %d: %v", depth, e)
		}
	}
	// Pad a discarded payload to the exact newline-inclusive ceiling.
	m = v4Wire(t, base)
	v4Extensions(m)["future"] = map[string]any{"version": 1, "data": map[string]any{"padding": ""}}
	b, _ := json.Marshal(m)
	for _, size := range []int{MaxV4LineBytes - 1, MaxV4LineBytes} {
		p := size - 1 - len(b)
		v4Extensions(m)["future"].(map[string]any)["data"].(map[string]any)["padding"] = strings.Repeat("x", p)
		raw, _ := json.Marshal(m)
		_, e := Decode(raw)
		if (e == nil) != (size < MaxV4LineBytes) {
			t.Fatalf("line bytes %d: %v", len(raw)+1, e)
		}
	}
	// Saturate all typed facet rosters and numeric widths without truncation.
	r := v4Record(t, "workflow")
	r.Phases = nil
	for i := 0; i < MaxPhases+1; i++ {
		r.Phases = append(r.Phases, PhaseStat{fmt.Sprintf("phase-%d", i), math.MaxInt64, math.MaxInt64})
	}
	req := &V4Requests{Requests: []RequestStats{}, RequestCoverage: CoverageMeasured}
	for i := 0; i < MaxRequestGroups; i++ {
		n := int64(math.MaxInt64)
		q := RequestStats{Class: RequestProvider, Origin: OriginDirect, Protocol: "graphql", Resource: "graphql", Outcome: "2xx", Attempts: n, DurationCoverage: map[Coverage]int64{CoverageMeasured: n}, DurationSumMs: n, DurationMaxMs: 1, Bytes: n, BytesSamples: n, Points: Points{Coverage: CoverageEstimated, Unit: UnitGitHubPoints, Value: &n, Method: fmt.Sprintf("method-%d", i), Version: "v1"}, PointSamples: n}
		q.DurationBins[1] = n
		req.Requests = append(req.Requests, q)
	}
	r.V4.Requests = req
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	if len(b)+1 >= MaxV4LineBytes {
		t.Fatal("typed maximum did not fit")
	}
	got, e := Decode(b)
	if e != nil || len(got.V4.Requests.Requests) != MaxRequestGroups || len(got.Phases) != MaxPhases+1 {
		t.Fatal("typed data truncated", e)
	}
}

// regression: #4360
func TestV4IdentitySelection(t *testing.T) {
	r := v4Record(t, "workflow")
	conflict := r
	conflict.Tool = "another"
	kept, s, e := SelectV4([]Record{r, r, conflict}, ProfileWorkflow, V3Filter{})
	if e != nil || len(kept) != 0 || s.Duplicates != 1 || s.Conflicts != 1 {
		t.Fatal(s, e)
	}
	m := v4Wire(t, r)
	v4Extensions(m)["future"] = map[string]any{"version": 1, "data": map[string]any{"x": 1}}
	a, e := v4Decode(t, m)
	if e != nil {
		t.Fatal(e)
	}
	v4Extensions(m)["future"].(map[string]any)["data"] = map[string]any{"x": 2}
	b, e := v4Decode(t, m)
	if e != nil {
		t.Fatal(e)
	}
	kept, s, e = SelectV4([]Record{a, b}, ProfileWorkflow, V3Filter{})
	if e != nil || len(kept) != 0 || s.Conflicts != 1 {
		t.Fatal("discarded-facet conflict hidden", s, e)
	}
}

// regression: #4360
func TestV4WriterConsentIsolation(t *testing.T) {
	r := v4Record(t, "tool")
	linked := r
	cov := map[ContextField]Coverage{}
	for _, k := range contextFields {
		cov[k] = CoverageUnsupported
	}
	linked.V4 = &V4Data{Profile: ProfileTool, Kind: ObservationProcess, InvocationID: r.V4.InvocationID, ProcessAvailable: true, Attribution: &V4Attribution{Coverage: cov}}
	if _, e := json.Marshal(linked); e == nil {
		t.Fatal("tool profile emitted even empty workflow linkage")
	}
	dir := filepath.Join(t.TempDir(), "off")
	t.Setenv(EnvVar, "")
	if (&Writer{}).WriteEnabled(true, dir, r) == nil {
		t.Fatal("resolved consent bypassed exact opt-in")
	}
	if _, e := os.Stat(dir); !os.IsNotExist(e) {
		t.Fatal("disabled writer touched disk")
	}
	t.Setenv(EnvVar, "1")
	if e := (&Writer{}).Write(dir, r); e != nil {
		t.Fatal(e)
	}
	entries, e := os.ReadDir(Dir(dir))
	if e != nil || len(entries) != 2 {
		t.Fatal(entries, e)
	}
	for _, f := range entries {
		info, e := f.Info()
		if e != nil || info.Mode().Perm() != 0600 {
			t.Fatal("permissions", e)
		}
		if strings.HasSuffix(f.Name(), ".jsonl") {
			b, _ := os.ReadFile(filepath.Join(Dir(dir), f.Name()))
			if bytes.Count(b, []byte("\n")) != 1 {
				t.Fatal("dual write")
			}
			if _, e = Decode(bytes.TrimSpace(b)); e != nil {
				t.Fatal(e)
			}
		}
	}
}

// regression: #4360
func TestV4PaddedDecodeLineBound(t *testing.T) {
	raw, e := json.Marshal(v4Record(t, "tool"))
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range []int{MaxV4LineBytes - 2, MaxV4LineBytes - 1} {
		padded := append(append([]byte{}, raw...), bytes.Repeat([]byte(" "), n-len(raw))...)
		_, e := Decode(padded)
		if (e == nil) != (n+1 < MaxV4LineBytes) {
			t.Fatalf("padded input bytes=%d: %v", n, e)
		}
	}
}
