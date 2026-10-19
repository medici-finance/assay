// Package schemacheck is a validator for the JSON Schema subset schemas/cell-op-v1.json uses. The
// module carries no JSON Schema library, and this repo does not add a dependency for one check; the
// subset is small, and the validator REFUSES any keyword it does not implement, so a schema edit
// that reaches for an unsupported keyword fails loudly instead of being silently ignored. Only
// tests import it.
package schemacheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

// annotations are keywords that carry no constraint.
var annotations = map[string]bool{"$schema": true, "$id": true, "title": true, "description": true, "$defs": true}

// Validator checks documents against one loaded schema.
type Validator struct {
	root map[string]any
}

// Load reads the schema at path.
func Load(path string) (*Validator, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("schema does not parse: %v", err)
	}
	return &Validator{root: root}, nil
}

// Decode parses one JSON document, keeping numbers exact.
func Decode(raw []byte) (any, error) {
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

// Validate returns every violation of the document against the schema's root.
func (sv *Validator) Validate(doc any) []string {
	var errs []string
	sv.check(sv.root, doc, "$", &errs)
	return errs
}

func (sv *Validator) resolve(ref string) map[string]any {
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

func (sv *Validator) check(s map[string]any, v any, path string, errs *[]string) {
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
