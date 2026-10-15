package contexthandoff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A deliberately small JSON Schema (draft 2020-12) evaluator for the two
// schemas this package owns. It implements a CLOSED keyword set; a schema that
// uses any other keyword fails the test rather than being half-checked, so the
// schema cannot silently grow a constraint this evaluator ignores. The module
// carries no JSON Schema dependency and this test adds none.
var schemaKeywords = map[string]bool{
	"$schema": true, "$id": true, "title": true, "description": true,
	"type": true, "const": true, "enum": true,
	"required": true, "properties": true, "additionalProperties": true,
	"items": true, "minItems": true, "maxItems": true,
	"pattern": true, "$ref": true, "$defs": true,
	"not": true, "oneOf": true, "allOf": true, "if": true, "then": true, "else": true,
}

type testSchema struct {
	root     map[string]any
	patterns map[string]*regexp.Regexp
}

func loadSchema(t *testing.T, path string) *testSchema {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var root map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&root); err != nil {
		t.Fatalf("parse schema %s: %v", path, err)
	}
	s := &testSchema{root: root, patterns: map[string]*regexp.Regexp{}}
	if err := s.checkKeywords(root, "#"); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return s
}

func (s *testSchema) checkKeywords(node any, at string) error {
	m, ok := node.(map[string]any)
	if !ok {
		if _, isBool := node.(bool); isBool {
			return nil
		}
		return fmt.Errorf("%s: schema node is %T, not an object", at, node)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !schemaKeywords[k] {
			return fmt.Errorf("%s: keyword %q is outside the evaluator's closed set", at, k)
		}
		v := m[k]
		switch k {
		case "properties", "$defs":
			sub, _ := v.(map[string]any)
			names := make([]string, 0, len(sub))
			for n := range sub {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				if err := s.checkKeywords(sub[n], at+"/"+k+"/"+n); err != nil {
					return err
				}
			}
		case "items", "additionalProperties", "if", "then", "else", "not":
			if err := s.checkKeywords(v, at+"/"+k); err != nil {
				return err
			}
		case "oneOf", "allOf":
			arr, _ := v.([]any)
			for i, e := range arr {
				if err := s.checkKeywords(e, fmt.Sprintf("%s/%s/%d", at, k, i)); err != nil {
					return err
				}
			}
		case "pattern":
			p, _ := v.(string)
			re, err := regexp.Compile(p)
			if err != nil {
				return fmt.Errorf("%s: pattern %q: %v", at, p, err)
			}
			s.patterns[p] = re
		case "$ref":
			if _, err := s.resolve(v); err != nil {
				return fmt.Errorf("%s: %v", at, err)
			}
		}
	}
	return nil
}

func (s *testSchema) resolve(ref any) (any, error) {
	r, _ := ref.(string)
	const prefix = "#/$defs/"
	if !strings.HasPrefix(r, prefix) {
		return nil, fmt.Errorf("$ref %q is not a local #/$defs/ reference", r)
	}
	defs, _ := s.root["$defs"].(map[string]any)
	n, ok := defs[strings.TrimPrefix(r, prefix)]
	if !ok {
		return nil, fmt.Errorf("$ref %q does not resolve", r)
	}
	return n, nil
}

// Accepts parses data the way a generic JSON consumer does (duplicate keys
// last-wins, no trailing data) and validates it against the schema.
func (s *testSchema) Accepts(data []byte) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("not JSON: %v", err)
	}
	if dec.More() {
		return fmt.Errorf("not JSON: trailing data")
	}
	return s.validate(v, s.root, "$")
}

func schemaKind(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case json.Number:
		return "number"
	}
	return "unknown"
}

func (s *testSchema) validate(v any, node any, at string) error {
	if b, ok := node.(bool); ok {
		if !b {
			return fmt.Errorf("%s: not allowed", at)
		}
		return nil
	}
	n := node.(map[string]any)
	if ref, ok := n["$ref"]; ok {
		target, err := s.resolve(ref)
		if err != nil {
			return err
		}
		if err := s.validate(v, target, at); err != nil {
			return err
		}
	}
	if ty, ok := n["type"]; ok {
		var want []string
		switch t := ty.(type) {
		case string:
			want = []string{t}
		case []any:
			for _, e := range t {
				want = append(want, e.(string))
			}
		}
		got := schemaKind(v)
		match := false
		for _, w := range want {
			match = match || w == got
		}
		if !match {
			return fmt.Errorf("%s: type %s, want %v", at, got, want)
		}
	}
	if c, ok := n["const"]; ok && !reflect.DeepEqual(c, v) {
		return fmt.Errorf("%s: %v is not the constant %v", at, v, c)
	}
	if e, ok := n["enum"]; ok {
		found := false
		for _, a := range e.([]any) {
			found = found || reflect.DeepEqual(a, v)
		}
		if !found {
			return fmt.Errorf("%s: %v is not in the enum", at, v)
		}
	}
	if obj, isObj := v.(map[string]any); isObj {
		if req, ok := n["required"]; ok {
			for _, k := range req.([]any) {
				if _, present := obj[k.(string)]; !present {
					return fmt.Errorf("%s: required %q is absent", at, k)
				}
			}
		}
		props, _ := n["properties"].(map[string]any)
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if ps, ok := props[k]; ok {
				if err := s.validate(obj[k], ps, at+"."+k); err != nil {
					return err
				}
			} else if ap, ok := n["additionalProperties"]; ok {
				if err := s.validate(obj[k], ap, at+"."+k); err != nil {
					return err
				}
			}
		}
	}
	if arr, isArr := v.([]any); isArr {
		if mi, ok := n["minItems"]; ok {
			if lim, _ := mi.(json.Number).Int64(); int64(len(arr)) < lim {
				return fmt.Errorf("%s: %d item(s), want at least %d", at, len(arr), lim)
			}
		}
		if ma, ok := n["maxItems"]; ok {
			if lim, _ := ma.(json.Number).Int64(); int64(len(arr)) > lim {
				return fmt.Errorf("%s: %d item(s), want at most %d", at, len(arr), lim)
			}
		}
		if it, ok := n["items"]; ok {
			for i, e := range arr {
				if err := s.validate(e, it, fmt.Sprintf("%s[%d]", at, i)); err != nil {
					return err
				}
			}
		}
	}
	if str, isStr := v.(string); isStr {
		if p, ok := n["pattern"]; ok && !s.patterns[p.(string)].MatchString(str) {
			return fmt.Errorf("%s: %q does not match %s", at, str, p)
		}
	}
	if neg, ok := n["not"]; ok && s.validate(v, neg, at) == nil {
		return fmt.Errorf("%s: matches forbidden schema", at)
	}
	if all, ok := n["allOf"]; ok {
		for _, sub := range all.([]any) {
			if err := s.validate(v, sub, at); err != nil {
				return err
			}
		}
	}
	if one, ok := n["oneOf"]; ok {
		passed := 0
		for _, sub := range one.([]any) {
			if s.validate(v, sub, at) == nil {
				passed++
			}
		}
		if passed != 1 {
			return fmt.Errorf("%s: matches %d oneOf branches, want exactly 1", at, passed)
		}
	}
	if cond, ok := n["if"]; ok {
		branch := "else"
		if s.validate(v, cond, at) == nil {
			branch = "then"
		}
		if b, ok := n[branch]; ok {
			if err := s.validate(v, b, at); err != nil {
				return err
			}
		}
	}
	return nil
}
