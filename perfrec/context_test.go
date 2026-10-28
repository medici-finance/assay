package perfrec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func v3Context(profile string) Context {
	return ResolveContext(ContextInput{Profile: Value{profile, true}, LookupEnv: func(k string) (string, bool) {
		if k == EnvVar {
			return "1", true
		}
		return "", false
	}})
}
func v3Record(c Context) Record {
	p := Probe{}
	return BeginSelected(p, p.Mark(), nil, c).Finish("tool", "query", "worker-desk", 0, "")
}

// regression: class perf-v3-consent
func TestPerfV3ContextAndConsent(t *testing.T) {
	t.Setenv(EnvVar, "1")
	t.Setenv("ASSAY_PERF_PROFILE", "workflow")
	id, _ := NewID()
	t.Setenv("ASSAY_PERF_AGENT_ID", id)
	c := ResolveContext(ContextInput{AgentID: Value{"", true}})
	if c.Data().AgentID != "" || c.Data().Coverage[FieldAgent] != CoverageMissing {
		t.Fatal("empty did not clear")
	}
	c = ResolveContext(ContextInput{SessionID: Value{"hostile input", true}})
	if c.Data().SessionID != "" || c.Data().Coverage[FieldSession] != CoverageInvalid {
		t.Fatal("invalid context retained")
	}
	d := c.Data()
	d.Coverage[FieldSession] = CoverageMeasured
	if c.Data().Coverage[FieldSession] != CoverageInvalid {
		t.Fatal("mutable coverage")
	}
	raw := `{"kind":"issue","forge":"github","instance":"github.com","repo":"example/one","issue":7,"primary":true}`
	other := `{"kind":"issue","forge":"github","instance":"github.com","repo":"example/two","issue":7}`
	c = ResolveContext(ContextInput{Work: []string{raw, other}, WorkPresent: true})
	if len(c.Data().Work) != 2 {
		t.Fatal("qualified issues conflated")
	}
	d = c.Data()
	d.Work[0].Repo = "changed/subject"
	if c.Data().Work[0].Repo != "example/one" {
		t.Fatal("mutable work")
	}
	for _, profile := range []string{"tool", "workflow"} {
		r := v3Record(v3Context(profile))
		b, e := json.Marshal(r)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = Decode(b); e != nil {
			t.Fatal(e)
		}
		if profile == "tool" {
			r.V3.Attribution.AgentID = id
			dir := t.TempDir()
			w := Writer{}
			if w.Write(dir, r) == nil {
				t.Fatal("writer allowed tool identity bypass")
			}
			if _, e = os.Stat(Dir(dir)); !os.IsNotExist(e) {
				t.Fatal("invalid write created directory")
			}
			var wire map[string]any
			_ = json.Unmarshal(b, &wire)
			v := wire["v3"].(map[string]any)
			v["attribution"].(map[string]any)["agentId"] = id
			planted, _ := json.Marshal(wire)
			if _, e = Decode(planted); e == nil {
				t.Fatal("decoder allowed tool identity bypass")
			}
		}
	}
	bad := v3Record(v3Context("tool"))
	bad.Tool = "raw/title must not pass"
	if (&Writer{}).Write(t.TempDir(), bad) == nil {
		t.Fatal("writer retained unbounded tool label")
	}
	t.Setenv("ASSAY_PERF_PROFILE", "")
	if ResolveContext(ContextInput{}).Profile() != "" {
		t.Fatal("profile inferred")
	}
	for _, name := range []string{"perf-v1.jsonl", "perf-v2.jsonl", "perf-v3.jsonl"} {
		b, e := os.ReadFile(filepath.Join("testdata", name))
		if e != nil {
			t.Fatal(e)
		}
		for _, line := range bytes.Split(bytes.TrimSpace(b), []byte("\n")) {
			r, e := Decode(line)
			if e != nil {
				t.Fatal(e)
			}
			got, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			var before, after any
			_ = json.Unmarshal(line, &before)
			_ = json.Unmarshal(got, &after)
			bb, _ := json.Marshal(before)
			aa, _ := json.Marshal(after)
			if !bytes.Equal(bb, aa) {
				t.Fatal("fixture meanings changed")
			}
		}
	}
	for _, profile := range []string{"", "off", "bogus"} {
		if ResolveContext(ContextInput{Profile: Value{profile, true}}).Profile() != "" {
			t.Fatal("invalid profile enabled")
		}
	}
	t.Setenv(EnvVar, "0")
	direct := v3Record(v3Context("tool"))
	sink := t.TempDir()
	if (&Writer{}).Write(sink, direct) == nil {
		t.Fatal("direct v3 writer bypassed opt-out")
	}
	if _, e := os.Stat(Dir(sink)); !os.IsNotExist(e) {
		t.Fatal("off direct writer created state")
	}
	t.Setenv(EnvVar, "0")
	if ResolveContext(ContextInput{Profile: Value{"workflow", true}}).Profile() != "" {
		t.Fatal("profile bypassed opt-in")
	}
}

// regression: #3353
func TestWriterResolvedConsent(t *testing.T) {
	v3 := v3Record(v3Context("tool"))
	for _, value := range []string{"", "0", "true", "1", " 1"} {
		t.Run("v3 env="+value, func(t *testing.T) {
			t.Setenv(EnvVar, value)
			for _, enabled := range []bool{false, true} {
				sink := t.TempDir()
				err := (&Writer{}).WriteEnabled(enabled, sink, v3)
				want := enabled && value == "1"
				if (err == nil) != want {
					t.Fatalf("resolved=%v env=%q: write error %v, want admitted=%v", enabled, value, err, want)
				}
				if !want {
					if _, err := os.Stat(Dir(sink)); !os.IsNotExist(err) {
						t.Fatalf("refused v3 write created state: %v", err)
					}
				}
			}
		})
	}
	for _, schema := range []string{Schema, SchemaV2} {
		r, err := Decode(fixtureLines(t, schema+".jsonl")[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, ambient := range []string{"0", "1"} {
			t.Setenv(EnvVar, ambient)
			for _, enabled := range []bool{false, true} {
				sink := t.TempDir()
				err := (&Writer{}).WriteEnabled(enabled, sink, r)
				if (err == nil) != enabled {
					t.Fatalf("%s resolved=%v ambient=%q: %v", schema, enabled, ambient, err)
				}
				if !enabled {
					if _, err := os.Stat(Dir(sink)); !os.IsNotExist(err) {
						t.Fatalf("resolved opt-out created state: %v", err)
					}
				}
			}
		}
	}
}
