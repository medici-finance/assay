package reviewtest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/medici-finance/assay/gitquiet/goquiet"
	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

func TestReviewBoundary(t *testing.T) {
	t.Run("pure-runtime", func(t *testing.T) {
		// Nothing in the pure path can obtain a provider, a subprocess or credentials.
		in, p := observationFixture()
		want := observation(t, in, p)
		t.Setenv("GH_TOKEN", "invalid-boundary-sentinel")
		t.Setenv("GITHUB_TOKEN", "invalid-boundary-sentinel")
		t.Setenv("HOME", filepath.Join(t.TempDir(), "absent"))
		t.Setenv("KUBECONFIG", os.DevNull)
		t.Setenv("PATH", t.TempDir())
		got, e := rf.PlanObservation(want, p)
		if e != nil || !reflect.DeepEqual(got.Result, want.Result) {
			t.Fatal("pure plan depended on environment", e)
		}
	})
	t.Run("dependency-isolation", func(t *testing.T) {
		assertPureImports(t)
	})
	t.Run("refusals", func(t *testing.T) {
		cases := []struct {
			name string
			kind rf.ErrorKind
			edit func(*rf.ObservationInputs, *rf.ObservationPolicy)
		}{
			{"missing-scope", rf.PartialError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { in.Requested = nil }},
			{"unadmitted-scope", rf.PartialError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) {
				r := in.Requested[0]
				r.Repository = "example-org/other"
				in.Requested = append(in.Requested, r)
			}},
			{"incomplete-as-empty", rf.PartialError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) {
				in.Changes = []rf.ObservedChange{}
				in.Population[0].Count = 0
				in.Population[0].Input.Complete = false
			}},
			{"missing-population", rf.PartialError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { in.Population = nil }},
			{"unknown-freshness", rf.StaleError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) {
				in.Changes[0].Inputs.Checks.Freshness = "unknown"
			}},
			{"stale-cached-input", rf.StaleError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) {
				in.Changes[0].Inputs.Body.ObservedAt = p.Oldest.Add(-time.Second)
			}},
			{"explicit-cutoff", rf.StaleError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { p.Oldest = in.Collection.End }},
			{"future-interval", rf.SchemaError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { in.Collection.End = p.Now.Add(time.Second) }},
			{"future-input", rf.SchemaError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) {
				in.Changes[0].Inputs.Checks.ObservedAt = p.Now
			}},
			{"unsupported-evaluator", rf.SchemaError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { in.Evaluator = "v999" }},
			{"policy-mismatch", rf.SchemaError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { in.Policy = "other-policy" }},
			{"no-policy-clock", rf.SchemaError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { p.Now = time.Time{} }},
			{"short-head", rf.SchemaError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { in.Changes[0].Identity.Head = "aaaa" }},
			{"missing-used-base", rf.SchemaError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { in.Changes[0].Identity.Base = "" }},
			{"scope-alias", rf.SchemaError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { in.Requested[0].Host = "GITHUB.COM" }},
			{"duplicate", rf.SchemaError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) {
				in.Changes = append(in.Changes, in.Changes[0])
				in.Population[0].Count = 2
			}},
			{"contradictory-review", rf.SchemaError, func(in *rf.ObservationInputs, p *rf.ObservationPolicy) { in.Changes[0].Facts.Reviews.Approved = true }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				in, p := observationFixture()
				tc.edit(&in, &p)
				_, e := rf.BuildObservation(in, p)
				expectInputError(t, e, tc.kind)
			})
		}
	})
	t.Run("closed-schema", func(t *testing.T) {
		in, p := observationFixture()
		o := observation(t, in, p)
		b, e := json.Marshal(o)
		if e != nil {
			t.Fatal(e)
		}
		var schema map[string]any
		raw, e := os.ReadFile(filepath.Join(moduleRoot(t), "..", "..", "schemas", "review-observation-v1.json"))
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(raw, &schema); e != nil {
			t.Fatal(e)
		}
		var doc any
		if e = json.Unmarshal(b, &doc); e != nil {
			t.Fatal(e)
		}
		if e = checkSchema(schema, schema, doc); e != nil {
			t.Fatal(e)
		}
		decoded, e := rf.DecodeObservation(b)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = rf.PlanObservation(decoded, p); e != nil {
			t.Fatal(e)
		}
		schemaShape(t, schema, reflect.TypeOf(o))
		mutations := []struct {
			name string
			edit func(map[string]any)
		}{
			{"schema", func(v map[string]any) { v["schema"] = "v2" }},
			{"missing-scope", func(v map[string]any) { delete(v["inputs"].(map[string]any), "requested") }},
			{"null-population", func(v map[string]any) { v["inputs"].(map[string]any)["changes"] = nil }},
			{"missing-false", func(v map[string]any) {
				c := v["inputs"].(map[string]any)["changes"].([]any)[0].(map[string]any)
				delete(c["facts"].(map[string]any), "riskClassed")
			}},
			{"unknown-field", func(v map[string]any) { v["authority"] = "ready" }},
		}
		for _, tc := range mutations {
			t.Run(tc.name, func(t *testing.T) {
				var v map[string]any
				_ = json.Unmarshal(b, &v)
				tc.edit(v)
				bad, _ := json.Marshal(v)
				_, e := rf.DecodeObservation(bad)
				expectInputError(t, e, rf.SchemaError)
				if checkSchema(schema, schema, v) == nil {
					t.Fatal("wire schema accepted decoder's negative fixture")
				}
			})
		}
		duplicate := strings.Replace(string(b), `"schema":"review-observation-v1"`, `"schema":"review-observation-v1","schema":"review-observation-v1"`, 1)
		_, e = rf.DecodeObservation([]byte(duplicate))
		expectInputError(t, e, rf.SchemaError)
		_, e = rf.DecodeObservation(append(b, []byte(" {}")...))
		expectInputError(t, e, rf.SchemaError)
	})
	t.Run("external-module-import", func(t *testing.T) {
		dir := t.TempDir()
		root := moduleRoot(t)
		gomod := "module example.invalid/review-consumer\n\ngo 1.27.0\n\nrequire github.com/medici-finance/assay/tools/desk v0.0.0\nreplace github.com/medici-finance/assay/tools/desk => " + strconv.Quote(root) + "\n"
		if e := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0600); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nimport _ \"github.com/medici-finance/assay/tools/desk/internal/reviewflow\"\nfunc main() {}\n"), 0600); e != nil {
			t.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
		out, e := goquiet.Cmd(cmd).CombinedOutput()
		if e == nil || !strings.Contains(string(out), "use of internal package github.com/medici-finance/assay/tools/desk/internal/reviewflow not allowed") {
			t.Fatalf("wanted Go internal-boundary refusal, got %v\n%s", e, out)
		}
	})
	t.Run("mutation-control", func(t *testing.T) {
		contractMutation(t, "internal/reviewflow/observation.go", "if !p.Complete {", "if false && !p.Complete {", "^TestReviewBoundary$/^refusals$/^incomplete-as-empty$", "want partial input error")
	})
}

func assertPureImports(t *testing.T) {
	t.Helper()
	allowed := map[string]bool{"bytes": true, "encoding/json": true, "fmt": true, "io": true, "reflect": true, "regexp": true, "sort": true, "strconv": true, "strings": true, "time": true}
	files, e := filepath.Glob(filepath.Join(moduleRoot(t), "internal", "reviewflow", "*.go"))
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, e := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if e != nil {
			t.Fatal(e)
		}
		for _, imp := range f.Imports {
			name, e := strconv.Unquote(imp.Path.Value)
			if e != nil || !allowed[name] {
				t.Fatalf("pure reviewflow imports effect/credential dependency %s in %s", name, path)
			}
		}
	}
}

// The published schema uses this deliberately small JSON Schema subset. Unknown
// keywords fail this qualification helper instead of silently weakening it.
func checkSchema(root, s map[string]any, v any) error {
	if ref, ok := s["$ref"].(string); ok {
		target, ok := root["$defs"].(map[string]any)[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
		if !ok {
			return fmt.Errorf("unknown ref %s", ref)
		}
		return checkSchema(root, target, v)
	}
	for k := range s {
		switch k {
		case "$schema", "$id", "$defs", "title", "description", "type", "properties", "required", "additionalProperties", "items", "const", "enum", "pattern", "format", "minimum", "minLength", "minItems", "maxItems":
		default:
			return fmt.Errorf("unsupported schema keyword %s", k)
		}
	}
	if want, ok := s["const"]; ok && !reflect.DeepEqual(want, v) {
		return fmt.Errorf("const mismatch")
	}
	if choices, ok := s["enum"].([]any); ok {
		found := false
		for _, c := range choices {
			found = found || reflect.DeepEqual(c, v)
		}
		if !found {
			return fmt.Errorf("enum mismatch")
		}
	}
	switch s["type"] {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("expected object")
		}
		if req, ok := s["required"].([]any); ok {
			for _, key := range req {
				if _, ok := obj[key.(string)]; !ok {
					return fmt.Errorf("missing %s", key)
				}
			}
		}
		props, _ := s["properties"].(map[string]any)
		for key, item := range obj {
			if child, ok := props[key].(map[string]any); ok {
				if e := checkSchema(root, child, item); e != nil {
					return fmt.Errorf("%s: %w", key, e)
				}
			} else {
				switch extra := s["additionalProperties"].(type) {
				case bool:
					if !extra {
						return fmt.Errorf("unknown field %s", key)
					}
				case map[string]any:
					if e := checkSchema(root, extra, item); e != nil {
						return e
					}
				}
			}
		}
	case "array":
		items, ok := v.([]any)
		if !ok {
			return fmt.Errorf("expected array")
		}
		if n, ok := s["minItems"].(float64); ok && len(items) < int(n) {
			return fmt.Errorf("too few items")
		}
		if n, ok := s["maxItems"].(float64); ok && len(items) > int(n) {
			return fmt.Errorf("too many items")
		}
		for _, item := range items {
			if e := checkSchema(root, s["items"].(map[string]any), item); e != nil {
				return e
			}
		}
	case "string":
		str, ok := v.(string)
		if !ok {
			return fmt.Errorf("expected string")
		}
		if n, ok := s["minLength"].(float64); ok && len(str) < int(n) {
			return fmt.Errorf("short string")
		}
		if pattern, ok := s["pattern"].(string); ok {
			matched, e := regexp.MatchString(pattern, str)
			if e != nil || !matched {
				return fmt.Errorf("pattern mismatch")
			}
		}
		if s["format"] == "date-time" {
			if _, e := time.Parse(time.RFC3339Nano, str); e != nil {
				return e
			}
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("expected bool")
		}
	case "integer":
		n, ok := v.(float64)
		if !ok || n != float64(int(n)) {
			return fmt.Errorf("expected integer")
		}
		if minimum, ok := s["minimum"].(float64); ok && n < minimum {
			return fmt.Errorf("below minimum")
		}
	default:
		return fmt.Errorf("unsupported schema type %v", s["type"])
	}
	return nil
}
func schemaShape(t *testing.T, root map[string]any, typ reflect.Type) {
	t.Helper()
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		if typ == reflect.TypeOf(time.Time{}) || seen[typ] {
			return
		}
		seen[typ] = true
		switch typ.Kind() {
		case reflect.Struct:
			defs := root["$defs"].(map[string]any)
			def, ok := defs[typ.Name()].(map[string]any)
			if !ok {
				t.Fatalf("schema missing Go type %s", typ)
			}
			props := def["properties"].(map[string]any)
			required := def["required"].([]any)
			if len(props) != typ.NumField() || len(required) != typ.NumField() || def["additionalProperties"] != false {
				t.Fatalf("schema differs from closed Go type %s", typ)
			}
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				key := strings.Split(f.Tag.Get("json"), ",")[0]
				if key == "" {
					key = f.Name
				}
				if _, ok := props[key]; !ok {
					t.Fatalf("schema missing field %s.%s", typ, key)
				}
				walk(f.Type)
			}
		case reflect.Slice, reflect.Map:
			walk(typ.Elem())
		}
	}
	walk(typ)
}
