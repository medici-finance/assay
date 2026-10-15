package cellop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A test-local validator for the JSON Schema subset schemas/cell-op-v1.json uses. The module
// carries no JSON Schema library, and this package does not add a dependency for one test; the
// subset is small, and the validator REFUSES any keyword it does not implement, so a schema edit
// that reaches for an unsupported keyword fails loudly instead of being silently ignored.

var repoRoot = filepath.Join("..", "..", "..", "..")

// annotations are keywords that carry no constraint.
var annotations = map[string]bool{"$schema": true, "$id": true, "title": true, "description": true, "$defs": true}

type schemaValidator struct {
	root map[string]any
}

func loadSchema(t *testing.T) *schemaValidator {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot, "schemas", "cell-op-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("schema does not parse: %v", err)
	}
	return &schemaValidator{root: root}
}

func decodeDoc(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing data")
	}
	return v, nil
}

// validate returns every violation of the document against the schema's root.
func (sv *schemaValidator) validate(doc any) []string {
	var errs []string
	sv.check(sv.root, doc, "$", &errs)
	return errs
}

func (sv *schemaValidator) resolve(ref string) map[string]any {
	const prefix = "#/$defs/"
	if !strings.HasPrefix(ref, prefix) {
		panic("unsupported $ref " + ref)
	}
	defs, _ := sv.root["$defs"].(map[string]any)
	s, ok := defs[strings.TrimPrefix(ref, prefix)].(map[string]any)
	if !ok {
		panic("dangling $ref " + ref)
	}
	return s
}

func typeOf(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		if _, err := t.Int64(); err == nil {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return fmt.Sprintf("%T", v)
}

func typeMatches(want string, v any) bool {
	got := typeOf(v)
	return got == want || (want == "number" && got == "integer")
}

func (sv *schemaValidator) check(s map[string]any, v any, path string, errs *[]string) {
	fail := func(format string, args ...any) {
		*errs = append(*errs, path+": "+fmt.Sprintf(format, args...))
	}
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		kw := s[k]
		switch k {
		case "$ref":
			sv.check(sv.resolve(kw.(string)), v, path, errs)
		case "type":
			var types []string
			switch tw := kw.(type) {
			case string:
				types = []string{tw}
			case []any:
				for _, x := range tw {
					types = append(types, x.(string))
				}
			}
			ok := false
			for _, ty := range types {
				ok = ok || typeMatches(ty, v)
			}
			if !ok {
				fail("type %s, want %v", typeOf(v), types)
			}
		case "const":
			if !reflect.DeepEqual(kw, plain(v)) {
				fail("value %v, want const %v", v, kw)
			}
		case "enum":
			found := false
			for _, e := range kw.([]any) {
				found = found || reflect.DeepEqual(e, plain(v))
			}
			if !found {
				fail("value %v not in enum", v)
			}
		case "pattern":
			if str, ok := v.(string); ok && !regexp.MustCompile(kw.(string)).MatchString(str) {
				fail("%q does not match %s", str, kw)
			}
		case "minLength":
			if str, ok := v.(string); ok && float64(len([]rune(str))) < kw.(float64) {
				fail("shorter than %v", kw)
			}
		case "minItems":
			if arr, ok := v.([]any); ok && float64(len(arr)) < kw.(float64) {
				fail("fewer than %v items", kw)
			}
		case "required":
			if obj, ok := v.(map[string]any); ok {
				for _, r := range kw.([]any) {
					if _, has := obj[r.(string)]; !has {
						fail("missing required member %q", r)
					}
				}
			}
		case "properties":
			if obj, ok := v.(map[string]any); ok {
				props := kw.(map[string]any)
				for name, sub := range props {
					if mv, has := obj[name]; has {
						sv.check(sub.(map[string]any), mv, path+"."+name, errs)
					}
				}
			}
		case "additionalProperties":
			obj, ok := v.(map[string]any)
			if !ok {
				break
			}
			props, _ := s["properties"].(map[string]any)
			for name, mv := range obj {
				if _, declared := props[name]; declared {
					continue
				}
				switch ap := kw.(type) {
				case bool:
					if !ap {
						fail("member %q is not allowed", name)
					}
				case map[string]any:
					sv.check(ap, mv, path+"."+name, errs)
				}
			}
		case "items":
			if arr, ok := v.([]any); ok {
				for i, e := range arr {
					sv.check(kw.(map[string]any), e, fmt.Sprintf("%s[%d]", path, i), errs)
				}
			}
		case "oneOf":
			matched := 0
			for _, sub := range kw.([]any) {
				var subErrs []string
				sv.check(sub.(map[string]any), v, path, &subErrs)
				if len(subErrs) == 0 {
					matched++
				}
			}
			if matched != 1 {
				fail("matches %d of oneOf's schemas, want exactly 1", matched)
			}
		default:
			if !annotations[k] {
				panic("schema uses keyword " + k + ", which the test validator does not implement")
			}
		}
	}
}

// plain turns a decoded number into the float64 a schema's own (non-UseNumber) decode holds, so
// const and enum compare like with like.
func plain(v any) any {
	if n, ok := v.(json.Number); ok {
		f, _ := n.Float64()
		return f
	}
	return v
}

// jsonBlocks are the bodies of every ```json fence in a Markdown file.
func jsonBlocks(t *testing.T, rel string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		t.Fatal(err)
	}
	var blocks []string
	var cur []string
	in := false
	for _, line := range strings.Split(string(raw), "\n") {
		switch {
		case !in && strings.TrimSpace(line) == "```json":
			in, cur = true, nil
		case in && strings.TrimSpace(line) == "```":
			in = false
			blocks = append(blocks, strings.Join(cur, "\n"))
		case in:
			cur = append(cur, line)
		}
	}
	if in {
		t.Fatalf("%s: unterminated ```json fence", rel)
	}
	return blocks
}

// A contract example that drifts from the schema, or a plan example whose digest no longer recomputes, is caught (desk-containers/15).
// regression: #2890
func TestContractExamplesValidate(t *testing.T) {
	sv := loadSchema(t)
	kinds := map[string]int{}
	for _, rel := range []string{"spec/cell-op-v1.md", "docs/cellctl-machine-seam.md"} {
		blocks := jsonBlocks(t, rel)
		if len(blocks) == 0 {
			t.Fatalf("%s carries no ```json example", rel)
		}
		for i, b := range blocks {
			doc, err := decodeDoc([]byte(b))
			if err != nil {
				t.Fatalf("%s example %d does not parse: %v", rel, i+1, err)
			}
			if errs := sv.validate(doc); len(errs) > 0 {
				t.Errorf("%s example %d fails the schema:\n  %s", rel, i+1, strings.Join(errs, "\n  "))
			}
			kind, _ := doc.(map[string]any)["kind"].(string)
			kinds[kind]++
			if kind == KindPlan {
				if _, err := DecodePlan([]byte(b)); err != nil {
					t.Errorf("%s example %d: %v", rel, i+1, err)
				}
			}
		}
	}
	if kinds[KindPlan] == 0 || kinds[KindResult] == 0 {
		t.Fatalf("the examples must show both a plan and a result, got %v", kinds)
	}
}

// A validator that accepts anything would make TestContractExamplesValidate vacuous (desk-containers/15).
// regression: #2890
func TestSchemaRejectsBrokenDocs(t *testing.T) {
	sv := loadSchema(t)
	p := testPlan(t, "example", "running")
	r := NewResult("drain", "example", p.ExpiresAt())
	r.Finish(OK, "", "", p.ExpiresAt())
	for name, mutate := range map[string]func(m map[string]any){
		"missing next":        func(m map[string]any) { delete(m, "next") },
		"extra member":        func(m map[string]any) { m["approved_by"] = "someone" },
		"unknown outcome":     func(m map[string]any) { m["outcome"] = "done" },
		"unknown reason":      func(m map[string]any) { m["reason"] = "operator-said-so" },
		"zero for unmeasured": func(m map[string]any) { m["started"] = 0 },
		"bad plan id":         func(m map[string]any) { m["plan_id"] = "../x" },
	} {
		raw, _ := json.Marshal(r)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		mutate(m)
		raw, _ = json.Marshal(m)
		doc, _ := decodeDoc(raw)
		if errs := sv.validate(doc); len(errs) == 0 {
			t.Errorf("result with %s validated", name)
		}
	}
	for name, mutate := range map[string]func(m map[string]any){
		"unsorted-looking cells": func(m map[string]any) { m["cells"] = []any{} },
		"short confirm code":     func(m map[string]any) { m["confirm_code"] = "abc" },
		"approval member":        func(m map[string]any) { m["approval"] = map[string]any{} },
		"null children":          func(m map[string]any) { m["children"] = nil },
	} {
		raw, _ := json.Marshal(p)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		mutate(m)
		raw, _ = json.Marshal(m)
		doc, _ := decodeDoc(raw)
		if errs := sv.validate(doc); len(errs) == 0 {
			t.Errorf("plan with %s validated", name)
		}
	}
	for _, ok := range []any{p, r} {
		raw, _ := json.Marshal(ok)
		doc, _ := decodeDoc(raw)
		if errs := sv.validate(doc); len(errs) > 0 {
			t.Errorf("a document the package emits fails the schema: %v", errs)
		}
	}
}
