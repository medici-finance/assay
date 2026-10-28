package perfrec

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"math"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var writeV4Schema = flag.String("write-v4-schema", "", "explicit output path for the generated v4 reference schema")

// schemaType takes member spellings/requiredness from the same reference types
// as the exact-key reader. Cross-field coverage arithmetic is additionally
// enforced by the historical canonical validators, not claimed by JSON Schema.
func schemaType(t reflect.Type, vocab schemaVocabulary) map[string]any {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	s := map[string]any{}
	switch t.Kind() {
	case reflect.Struct:
		props := map[string]any{}
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if tag == "-" {
				continue
			}
			if tag == "" {
				tag = f.Name
			}
			props[tag] = schemaType(f.Type, vocab)
			if !strings.Contains(f.Tag.Get("json"), ",omitempty") {
				required = append(required, tag)
			}
		}
		s = map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
	case reflect.Map:
		s = map[string]any{"type": "object", "additionalProperties": schemaType(t.Elem(), vocab)}
	case reflect.Slice, reflect.Array:
		s = map[string]any{"type": "array", "items": schemaType(t.Elem(), vocab)}
		if t.Kind() == reflect.Array {
			s["minItems"] = t.Len()
			s["maxItems"] = t.Len()
		}
	case reflect.String:
		s["type"] = "string"
	case reflect.Bool:
		s["type"] = "boolean"
	default:
		s["type"] = "integer"
		s["minimum"] = 0
		s["maximum"] = int64(9223372036854775807)
	}
	constrainV4Schema(t, s, vocab)
	return s
}
func v4ReferenceSchema() []byte {
	vocab := loadSchemaVocabulary()
	s := schemaType(reflect.TypeOf(v4Core{}), vocab)
	s["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	s["$id"] = "https://github.com/medici-finance/assay/schemas/perf-v4.json"
	s["title"] = "Local performance observation v4"
	s["$comment"] = "Exact known members derive from canonical perfrec reference structs. The codec additionally enforces duplicate rejection, maximum nesting 32, JSONL bytes smaller than 1 MiB including newline, work-reference relationships, identity privacy, UTF-8 byte limits and all cross-field coverage/aggregate invariants. Registered v1 member sets, fixed vocabularies, rosters, patterns and array bounds are structural constraints; they do not replace canonical Go validation. Unknown optional data is discarded, never a producer API."
	p := s["properties"].(map[string]any)
	p["schema"] = map[string]any{"const": SchemaV4}
	p["profile"] = map[string]any{"enum": []string{"tool", "workflow"}}
	p["observationKind"] = map[string]any{"enum": []ObservationKind{ObservationProcess, ObservationClient, ObservationHandler, ObservationProvider}}
	p["role"] = map[string]any{"enum": append([]string{RoleNone}, vocab.cases("roleValid", "s")...)}
	p["boundary"] = map[string]any{"enum": []string{BoundaryProcess, BoundaryCaller}}
	p["cpu"] = map[string]any{"enum": []string{CPUMeasured, CPUMissing, CPUUnsupported, CPUFailed}}
	p["result"] = map[string]any{"enum": []string{ResultOK, ResultDisabled, ResultRefused, ResultUnverifiable}}
	p["exit"] = map[string]any{"type": "integer"}
	for _, k := range []string{"invocationId", "install"} {
		p[k] = map[string]any{"type": "string", "pattern": InstallIDPattern.String()}
	}
	for _, k := range []string{"tool", "verb"} {
		p[k] = map[string]any{"type": "string", "pattern": labelPattern.String(), "maxLength": 64}
	}
	p["sourceSHA"] = map[string]any{"type": "string", "pattern": "^([0-9a-f]{7,40}|unpinned)$"}
	p["ts"] = map[string]any{"type": "string", "format": "date-time"}
	generic := func() map[string]any {
		return map[string]any{"type": "object", "required": []string{"version", "data"}, "additionalProperties": false, "properties": map[string]any{"version": map[string]any{"type": "integer", "minimum": 1, "maximum": MaxV4ExtensionVersion}, "data": map[string]any{"type": "object"}}}
	}
	props := map[string]any{}
	for name, t := range map[string]reflect.Type{"process": reflect.TypeOf(v4Process{}), "attribution": reflect.TypeOf(V4Attribution{}), "requests": reflect.TypeOf(V4Requests{}), "query": reflect.TypeOf(v4Query{})} {
		known := generic()
		known["properties"].(map[string]any)["version"] = map[string]any{"const": 1}
		known["properties"].(map[string]any)["data"] = schemaType(t, vocab)
		future := generic()
		future["properties"].(map[string]any)["version"] = map[string]any{"type": "integer", "minimum": 2, "maximum": MaxV4ExtensionVersion}
		props[name] = map[string]any{"oneOf": []any{known, future}}
	}
	p["extensions"] = map[string]any{"type": "object", "maxProperties": MaxV4Extensions, "propertyNames": map[string]any{"pattern": extensionName.String()}, "properties": props, "additionalProperties": generic()}
	b, _ := json.MarshalIndent(s, "", "  ")
	return append(b, '\n')
}

// regression: #4360
func TestV4MachineSchema(t *testing.T) {
	want := v4ReferenceSchema()
	if *writeV4Schema != "" {
		if e := os.WriteFile(*writeV4Schema, want, 0644); e != nil {
			t.Fatal(e)
		}
	}
	got, e := os.ReadFile("../schemas/perf-v4.json")
	if e != nil || !bytes.Equal(got, want) {
		t.Fatal("v4 reference types/schema drift; regenerate through explicit write-v4-schema", e)
	}
}

// The generator reads the canonical validator's string cases, rather than owning
// another vocabulary. Unresolvable/non-string cases fail generation visibly.
type schemaVocabulary struct {
	files     []*ast.File
	constants map[string]ast.Expr
}

func loadSchemaVocabulary() schemaVocabulary {
	v := schemaVocabulary{constants: map[string]ast.Expr{}}
	for _, name := range []string{"context.go", "v3.go", "schema.go", "perfrec.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			panic(err)
		}
		v.files = append(v.files, f)
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, sp := range g.Specs {
				s := sp.(*ast.ValueSpec)
				if len(s.Names) == len(s.Values) {
					for i, n := range s.Names {
						v.constants[n.Name] = s.Values[i]
					}
				}
			}
		}
	}
	return v
}

func (v schemaVocabulary) stringValue(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		x, exists := v.constants[id.Name]
		if !exists {
			panic("unknown schema constant: " + id.Name)
		}
		return v.stringValue(x)
	}
	if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		x, err := strconv.Unquote(lit.Value)
		if err == nil {
			return x
		}
	}
	panic("schema vocabulary is not a string constant")
}

func schemaExpr(e ast.Expr) string {
	var b bytes.Buffer
	if err := format.Node(&b, token.NewFileSet(), e); err != nil {
		panic(err)
	}
	return b.String()
}

func (v schemaVocabulary) function(name string) *ast.FuncDecl {
	for _, f := range v.files {
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name {
				return fn
			}
		}
	}
	panic("missing canonical validator: " + name)
}

func (v schemaVocabulary) cases(fn, selector string) []string {
	var values []string
	ast.Inspect(v.function(fn), func(n ast.Node) bool {
		s, ok := n.(*ast.SwitchStmt)
		if !ok || s.Tag == nil || schemaExpr(s.Tag) != selector {
			return true
		}
		for _, c := range s.Body.List {
			for _, e := range c.(*ast.CaseClause).List {
				values = append(values, v.stringValue(e))
			}
		}
		return false
	})
	if len(values) == 0 {
		panic("missing schema vocabulary: " + fn + ":" + selector)
	}
	return values
}

func (v schemaVocabulary) comparisons(fn, selector string) []string {
	var values []string
	ast.Inspect(v.function(fn), func(n ast.Node) bool {
		b, ok := n.(*ast.BinaryExpr)
		if ok && b.Op == token.NEQ && schemaExpr(b.X) == selector {
			value := v.stringValue(b.Y)
			if value != "" {
				values = append(values, value)
			}
		}
		return true
	})
	if len(values) == 0 {
		panic("missing schema comparisons: " + fn + ":" + selector)
	}
	return values
}

func stringSchema(values []string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}
func patternSchema(pattern string, empty bool) map[string]any {
	if empty {
		pattern = "^$|" + pattern
	}
	return map[string]any{"type": "string", "pattern": pattern}
}
func labelSchema(empty bool) map[string]any {
	s := patternSchema(labelPattern.String(), empty)
	s["maxLength"] = 64
	return s
}
func rosterMap(keys []string, value map[string]any, required bool) map[string]any {
	s := map[string]any{"type": "object", "propertyNames": stringSchema(keys), "additionalProperties": value, "maxProperties": len(keys)}
	if required {
		s["required"] = keys
	}
	return s
}
func counterNames() []string {
	var names []string
	for _, c := range counterRosterArr {
		names = append(names, string(c))
	}
	return names
}
func contextNames() []string {
	var names []string
	for _, c := range contextFields {
		names = append(names, string(c))
	}
	return names
}

func constrainV4Schema(t reflect.Type, s map[string]any, v schemaVocabulary) {
	p, _ := s["properties"].(map[string]any)
	switch t {
	case reflect.TypeOf(v4Process{}):
		p["phases"].(map[string]any)["maxItems"] = MaxPhases + 1
		p["counters"] = rosterMap(counterNames(), schemaType(reflect.TypeOf(int64(0)), v), false)
		p["unavailable"] = map[string]any{"type": "array", "items": stringSchema(counterNames()), "maxItems": len(counterRosterArr), "uniqueItems": true}
	case reflect.TypeOf(PhaseStat{}):
		p["name"] = labelSchema(false)
		p["n"].(map[string]any)["minimum"] = 1
	case reflect.TypeOf(V4Attribution{}):
		for _, k := range []string{"sessionId", "agentId", "parentAgentId", "parentInvocationId"} {
			p[k] = patternSchema(InstallIDPattern.String(), true)
		}
		p["role"] = stringSchema(append([]string{""}, v.cases("roleValid", "s")...))
		p["deskRole"] = stringSchema(append([]string{""}, v.cases("deskRoleValid", "s")...))
		p["work"].(map[string]any)["maxItems"] = MaxWork
		p["coverage"] = rosterMap(contextNames(), stringSchema(v.cases("validAttribution", "v")), true)
	case reflect.TypeOf(WorkRef{}):
		p["kind"] = stringSchema(v.cases("validWork", "w.Kind"))
		p["forge"] = stringSchema(append([]string{""}, v.comparisons("validWork", "w.Forge")...))
		// Empty optional members are accepted by the codec. Relationships between
		// kind/forge/issue/brief and primary/duplicate work remain Go validation.
		p["repo"] = patternSchema(repoRE.String(), false)
		p["repo"].(map[string]any)["not"] = map[string]any{"pattern": `\.\.`}
		p["instance"] = patternSchema(hostRE.String(), true)
		p["instance"].(map[string]any)["not"] = map[string]any{"pattern": `\.\.`}
		p["brief"] = patternSchema(briefRE.String(), true)
		p["briefUuid"] = patternSchema(uuidRE.String(), true)
		p["issue"].(map[string]any)["maximum"] = int64(2147483647)
	case reflect.TypeOf(V4Requests{}):
		p["requests"].(map[string]any)["maxItems"] = MaxRequestGroups
		p["requestCoverage"] = stringSchema(v.cases("validV3", "v.RequestCoverage"))
	case reflect.TypeOf(RequestStats{}):
		for _, k := range []string{"class", "origin", "protocol", "resource", "outcome"} {
			selector := "r." + strings.ToUpper(k[:1]) + k[1:]
			p[k] = stringSchema(v.cases("validRequest", selector))
		}
		p["attempts"].(map[string]any)["minimum"] = 1
		p["durationCoverage"] = rosterMap(v.cases("validV3", "c"), schemaType(reflect.TypeOf(int64(0)), v), false)
	case reflect.TypeOf(Points{}):
		p["coverage"] = stringSchema(v.cases("validPoints", "p.Coverage"))
		p["unit"] = stringSchema(v.cases("validPoints", "p.Unit"))
		p["method"], p["version"] = labelSchema(true), labelSchema(true)
	case reflect.TypeOf(QueryObservation{}):
		for _, k := range []string{"operation", "providerCoverage", "source", "coalescing", "result"} {
			selector := "q." + strings.ToUpper(k[:1]) + k[1:]
			p[k] = stringSchema(v.cases("validQuery", selector))
		}
		for _, k := range []string{"diagnosticId", "parentDiagnosticId"} {
			p[k] = patternSchema(InstallIDPattern.String(), true)
		}
	}
}

// schemaAccepts is a bounded test interpreter of the emitted structural
// keywords, not a replacement codec or a general JSON Schema implementation.
// Unsupported generated keywords fail the test instead of silently passing.
func schemaAccepts(s map[string]any, x any) bool {
	for k := range s {
		switch k {
		case "$schema", "$id", "$comment", "title", "type", "const", "enum", "pattern", "format", "minimum", "maximum", "maxLength", "required", "properties", "additionalProperties", "propertyNames", "minProperties", "maxProperties", "items", "minItems", "maxItems", "uniqueItems", "oneOf", "not":
		default:
			panic("unsupported schema test keyword: " + k)
		}
	}
	if c, ok := s["const"]; ok && !reflect.DeepEqual(x, c) {
		return false
	}
	if e, ok := s["enum"].([]any); ok {
		found := false
		for _, v := range e {
			found = found || reflect.DeepEqual(v, x)
		}
		if !found {
			return false
		}
	}
	if branches, ok := s["oneOf"].([]any); ok {
		matches := 0
		for _, b := range branches {
			if schemaAccepts(b.(map[string]any), x) {
				matches++
			}
		}
		if matches != 1 {
			return false
		}
	}
	if n, ok := s["not"].(map[string]any); ok && schemaAccepts(n, x) {
		return false
	}
	switch s["type"] {
	case "object":
		if _, ok := x.(map[string]any); !ok {
			return false
		}
	case "array":
		if _, ok := x.([]any); !ok {
			return false
		}
	case "string":
		if _, ok := x.(string); !ok {
			return false
		}
	case "boolean":
		if _, ok := x.(bool); !ok {
			return false
		}
	case "integer":
		if n, ok := x.(float64); !ok || math.Trunc(n) != n {
			return false
		}
	}
	if str, ok := x.(string); ok {
		if p, ok := s["pattern"].(string); ok && !regexp.MustCompile(p).MatchString(str) {
			return false
		}
		if n, ok := s["maxLength"].(float64); ok && len([]rune(str)) > int(n) {
			return false
		}
		if s["format"] == "date-time" {
			if _, err := time.Parse(time.RFC3339, str); err != nil {
				return false
			}
		}
	}
	if n, ok := x.(float64); ok {
		if min, ok := s["minimum"].(float64); ok && n < min {
			return false
		}
		if max, ok := s["maximum"].(float64); ok && n > max {
			return false
		}
	}
	if a, ok := x.([]any); ok {
		if min, ok := s["minItems"].(float64); ok && len(a) < int(min) {
			return false
		}
		if max, ok := s["maxItems"].(float64); ok && len(a) > int(max) {
			return false
		}
		for i, v := range a {
			if item, ok := s["items"].(map[string]any); ok && !schemaAccepts(item, v) {
				return false
			}
			if s["uniqueItems"] == true {
				for _, prev := range a[:i] {
					if reflect.DeepEqual(v, prev) {
						return false
					}
				}
			}
		}
	}
	if o, ok := x.(map[string]any); ok {
		if min, ok := s["minProperties"].(float64); ok && len(o) < int(min) {
			return false
		}
		if max, ok := s["maxProperties"].(float64); ok && len(o) > int(max) {
			return false
		}
		for _, k := range schemaList(s["required"]) {
			if _, ok := o[k.(string)]; !ok {
				return false
			}
		}
		props, _ := s["properties"].(map[string]any)
		for k, value := range o {
			if names, ok := s["propertyNames"].(map[string]any); ok && !schemaAccepts(names, k) {
				return false
			}
			if p, ok := props[k].(map[string]any); ok {
				if !schemaAccepts(p, value) {
					return false
				}
			} else {
				switch additional := s["additionalProperties"].(type) {
				case bool:
					if !additional {
						return false
					}
				case map[string]any:
					if !schemaAccepts(additional, value) {
						return false
					}
				}
			}
		}
	}
	return true
}
func schemaList(x any) []any { a, _ := x.([]any); return a }
func machineSchema(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile("../schemas/perf-v4.json")
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err = json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	return s
}
func facetSchema(s map[string]any, name string) map[string]any {
	return s["properties"].(map[string]any)["extensions"].(map[string]any)["properties"].(map[string]any)[name].(map[string]any)["oneOf"].([]any)[0].(map[string]any)["properties"].(map[string]any)["data"].(map[string]any)
}
func facetProperties(s map[string]any, name string) map[string]any {
	return facetSchema(s, name)["properties"].(map[string]any)
}
func facetData(m map[string]any, name string) map[string]any {
	return v4Extensions(m)[name].(map[string]any)["data"].(map[string]any)
}

// regression: #4360
func TestV4MachineSchemaConformance(t *testing.T) {
	s := machineSchema(t)
	base := schemaConformanceRecord(t)
	// Real producer output, including request histograms, work attribution and
	// query fields, must satisfy the artifact independently of generation bytes.
	if !schemaAccepts(s, v4Wire(t, base)) {
		t.Fatal("schema rejects canonical producer")
	}
	mutations := []struct {
		name   string
		change func(map[string]any)
	}{
		{"counter roster", func(m map[string]any) { facetData(m, "process")["counters"].(map[string]any)["foreign"] = float64(0) }},
		{"unavailable roster", func(m map[string]any) { facetData(m, "process")["unavailable"] = []any{"foreign"} }},
		{"unavailable duplicate", func(m map[string]any) { facetData(m, "process")["unavailable"] = []any{"http", "http"} }},
		{"coverage roster", func(m map[string]any) {
			c := facetData(m, "attribution")["coverage"].(map[string]any)
			delete(c, "sessionId")
			c["foreign"] = "missing"
		}},
		{"coverage missing", func(m map[string]any) { delete(facetData(m, "attribution")["coverage"].(map[string]any), "sessionId") }},
		{"coverage enum", func(m map[string]any) {
			facetData(m, "attribution")["coverage"].(map[string]any)["sessionId"] = "estimated"
		}},
		{"private identity", func(m map[string]any) { facetData(m, "attribution")["sessionId"] = "native-session-path" }},
		{"desk role", func(m map[string]any) { facetData(m, "attribution")["deskRole"] = "owner" }},
		{"work repo", func(m map[string]any) {
			facetData(m, "attribution")["work"] = []any{map[string]any{"kind": "issue", "repo": "../private", "issue": float64(1)}}
		}},
		{"work kind", func(m map[string]any) {
			facetData(m, "attribution")["work"] = []any{map[string]any{"kind": "path", "repo": "example/project"}}
		}},
		{"work forge", func(m map[string]any) {
			facetData(m, "attribution")["work"] = []any{map[string]any{"kind": "issue", "repo": "example/project", "forge": "foreign"}}
		}},
		{"work brief", func(m map[string]any) {
			facetData(m, "attribution")["work"] = []any{map[string]any{"kind": "brief", "repo": "example/project", "brief": "../private"}}
		}},
		{"work issue bound", func(m map[string]any) {
			facetData(m, "attribution")["work"] = []any{map[string]any{"kind": "issue", "repo": "example/project", "issue": float64(2147483648)}}
		}},
		{"work cap", func(m map[string]any) {
			a := []any{}
			for i := 0; i < 9; i++ {
				a = append(a, map[string]any{"kind": "issue", "forge": "github", "instance": "github.com", "repo": "example/project", "issue": float64(i + 1)})
			}
			facetData(m, "attribution")["work"] = a
		}},
		{"phase cap", func(m map[string]any) {
			a := []any{}
			for i := 0; i < 18; i++ {
				a = append(a, map[string]any{"name": fmt.Sprintf("phase-%d", i), "n": float64(1), "ms": float64(0)})
			}
			facetData(m, "process")["phases"] = a
		}},
		{"phase name", func(m map[string]any) {
			facetData(m, "process")["phases"] = []any{map[string]any{"name": "/private/path", "n": float64(1), "ms": float64(0)}}
		}},
		{"request coverage", func(m map[string]any) { facetData(m, "requests")["requestCoverage"] = "estimated" }},

		{"request class", func(m map[string]any) { schemaRequest(m)["class"] = "foreign" }},
		{"request origin", func(m map[string]any) { schemaRequest(m)["origin"] = "foreign" }},
		{"request protocol", func(m map[string]any) { schemaRequest(m)["protocol"] = "ftp" }},
		{"request resource", func(m map[string]any) { schemaRequest(m)["resource"] = "raw-path" }},
		{"request outcome", func(m map[string]any) { schemaRequest(m)["outcome"] = "success" }},
		{"duration coverage", func(m map[string]any) { schemaRequest(m)["durationCoverage"] = map[string]any{"estimated": float64(1)} }},
		{"histogram short", func(m map[string]any) {
			schemaRequest(m)["durationBins"] = schemaRequest(m)["durationBins"].([]any)[:31]
		}},
		{"histogram long", func(m map[string]any) {
			a := schemaRequest(m)["durationBins"].([]any)
			schemaRequest(m)["durationBins"] = append(a, float64(0))
		}},
		{"points coverage", func(m map[string]any) { schemaRequest(m)["points"].(map[string]any)["coverage"] = "invalid" }},
		{"points unit", func(m map[string]any) { schemaRequest(m)["points"].(map[string]any)["unit"] = "quota" }},
		{"request cap", schemaRequestOverflow},
		{"query operation", func(m map[string]any) { schemaQuery(m)["operation"] = "foreign" }},
		{"query coverage", func(m map[string]any) { schemaQuery(m)["providerCoverage"] = "estimated" }},
		{"query source", func(m map[string]any) { schemaQuery(m)["source"] = "foreign" }},
		{"query coalescing", func(m map[string]any) { schemaQuery(m)["coalescing"] = "foreign" }},
		{"query result", func(m map[string]any) { schemaQuery(m)["result"] = "foreign" }},
		{"query diagnostic identity", func(m map[string]any) { schemaQuery(m)["diagnosticId"] = "private-session-path" }},
		{"closed v1", func(m map[string]any) { facetData(m, "process")["foreign"] = true }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			m := v4Wire(t, base)
			tc.change(m)
			if _, err := v4Decode(t, m); err == nil {
				t.Fatal("negative corpus did not fail canonical codec")
			}
			if schemaAccepts(s, m) {
				t.Fatal("schema accepted invalid facet")
			}
		})
	}
	// Unknown optional facets and future registered versions intentionally allow
	// arbitrary object members: structural closure applies only to registered v1.
	for _, name := range []string{"future-observation", "process", "attribution", "requests", "query"} {
		m := v4Wire(t, base)
		version := float64(1)
		if name != "future-observation" {
			version = 2
		}
		v4Extensions(m)[name] = map[string]any{"version": version, "data": map[string]any{"future": []any{true, map[string]any{"arbitrary": "object"}}}}
		if _, err := v4Decode(t, m); err != nil || !schemaAccepts(s, m) {
			t.Fatalf("future facet %s overconstrained: %v", name, err)
		}
	}
}

func schemaConformanceRecord(t *testing.T) Record {
	t.Helper()
	t.Setenv(EnvVar, "1")
	id := strings.Repeat("0", 31) + "1"
	c := ResolveContext(ContextInput{Profile: Value{"workflow", true}, SessionID: Value{id, true}, Role: Value{"verify", true}, WorkPresent: true, Work: []string{`{"kind":"issue","forge":"github","instance":"github.com","repo":"example/project","issue":1}`}, LookupEnv: func(k string) (string, bool) {
		if k == EnvVar {
			return "1", true
		}
		return "", false
	}})
	p := Probe{}
	s := BeginObservation(p, nil, c, ObservationClient)
	ms := int64(1)
	if !s.ObserveRequest(RequestObservation{Class: RequestProvider, Origin: OriginDirect, Protocol: "rest", Resource: "core", Outcome: "2xx", DurationMs: &ms, DurationCoverage: CoverageMeasured, Points: Points{Coverage: CoverageUnknown, Unit: UnitNone}}) {
		t.Fatal("request fixture refused")
	}
	if !s.SetQuery(QueryObservation{Operation: "query", LogicalCalls: 1, TransportAttempts: 1, ProviderCoverage: CoverageUnsupported, Source: "bypass", Coalescing: "none", Result: "ok"}) {
		t.Fatal("query fixture refused")
	}
	r := s.Finish("tool", "query", "verify-desk", 0, "")
	r.Install = id
	out, err := ToV4(r)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func schemaRequest(m map[string]any) map[string]any {
	return facetData(m, "requests")["requests"].([]any)[0].(map[string]any)
}
func schemaQuery(m map[string]any) map[string]any {
	return facetData(m, "query")["query"].(map[string]any)
}

// regression: #4360
func TestV4MachineSchemaDoctoredConstraints(t *testing.T) {
	base := schemaConformanceRecord(t)
	tests := []struct {
		name   string
		wire   func(map[string]any)
		doctor func(map[string]any)
	}{
		{"counter roster", func(m map[string]any) { facetData(m, "process")["counters"].(map[string]any)["foreign"] = float64(0) }, func(s map[string]any) {
			delete(facetProperties(s, "process")["counters"].(map[string]any), "propertyNames")
		}},
		{"enum", func(m map[string]any) { schemaRequest(m)["outcome"] = "foreign" }, func(s map[string]any) {
			delete(facetProperties(s, "requests")["requests"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["outcome"].(map[string]any), "enum")
		}},
		{"coverage required", func(m map[string]any) { delete(facetData(m, "attribution")["coverage"].(map[string]any), "sessionId") }, func(s map[string]any) {
			delete(facetProperties(s, "attribution")["coverage"].(map[string]any), "required")
		}},
		{"phase cap", func(m map[string]any) {
			a := []any{}
			for i := 0; i < 18; i++ {
				a = append(a, map[string]any{"name": fmt.Sprintf("phase-%d", i), "n": float64(1), "ms": float64(0)})
			}
			facetData(m, "process")["phases"] = a
		}, func(s map[string]any) { delete(facetProperties(s, "process")["phases"].(map[string]any), "maxItems") }},
		{"work cap", func(m map[string]any) {
			a := []any{}
			for i := 0; i < 9; i++ {
				a = append(a, map[string]any{"kind": "issue", "forge": "github", "instance": "github.com", "repo": "example/project", "issue": float64(i + 1)})
			}
			facetData(m, "attribution")["work"] = a
		}, func(s map[string]any) { delete(facetProperties(s, "attribution")["work"].(map[string]any), "maxItems") }},
		{"request cap", func(m map[string]any) {
			a := []any{}
			for i := 0; i < 65; i++ {
				a = append(a, schemaRequest(m))
			}
			facetData(m, "requests")["requests"] = a
		}, func(s map[string]any) {
			delete(facetProperties(s, "requests")["requests"].(map[string]any), "maxItems")
		}},
		{"histogram minimum", func(m map[string]any) {
			schemaRequest(m)["durationBins"] = schemaRequest(m)["durationBins"].([]any)[:31]
		}, func(s map[string]any) {
			delete(facetProperties(s, "requests")["requests"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["durationBins"].(map[string]any), "minItems")
		}},
		{"histogram cap", func(m map[string]any) {
			a := schemaRequest(m)["durationBins"].([]any)
			schemaRequest(m)["durationBins"] = append(a, float64(0))
		}, func(s map[string]any) {
			delete(facetProperties(s, "requests")["requests"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["durationBins"].(map[string]any), "maxItems")
		}},
		{"identity pattern", func(m map[string]any) { facetData(m, "attribution")["sessionId"] = "private-session-path" }, func(s map[string]any) {
			delete(facetProperties(s, "attribution")["sessionId"].(map[string]any), "pattern")
		}},
		{"registered shape", func(m map[string]any) { facetData(m, "process")["foreign"] = true }, func(s map[string]any) { facetSchema(s, "process")["additionalProperties"] = true }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := v4Wire(t, base)
			tc.wire(m)
			if _, err := v4Decode(t, m); err == nil {
				t.Fatal("negative control is not invalid in canonical codec")
			}
			s := machineSchema(t)
			if schemaAccepts(s, m) {
				t.Fatal("undamaged schema missed negative control")
			}
			tc.doctor(s)
			if !schemaAccepts(s, m) {
				t.Fatal("doctored constraint was not exercised by negative control")
			}
		})
	}
	// The request-cap sample contains distinct, otherwise valid groups. Its
	// immediate in-budget neighbor must pass both validators.
	within := v4Wire(t, base)
	schemaRequestOverflow(within)
	facetData(within, "requests")["requests"] = facetData(within, "requests")["requests"].([]any)[:64]
	if _, err := v4Decode(t, within); err != nil || !schemaAccepts(machineSchema(t), within) {
		t.Fatalf("in-budget request groups invalid: %v", err)
	}
	// The converse control catches accidentally closing unknown optional data.
	m := v4Wire(t, base)
	v4Extensions(m)["future-observation"] = map[string]any{"version": float64(1), "data": map[string]any{"new-member": true}}
	s := machineSchema(t)
	if !schemaAccepts(s, m) {
		t.Fatal("unknown optional data must remain structurally permissive")
	}
	s["properties"].(map[string]any)["extensions"].(map[string]any)["additionalProperties"].(map[string]any)["properties"].(map[string]any)["data"].(map[string]any)["additionalProperties"] = false
	if schemaAccepts(s, m) {
		t.Fatal("negative control did not detect closing unknown optional data")
	}
}

// regression: #4360
func TestV4MachineSchemaSourceVocabulary(t *testing.T) {
	v, s, base := loadSchemaVocabulary(), machineSchema(t), schemaConformanceRecord(t)
	for _, field := range []string{"class", "origin", "protocol", "resource", "outcome"} {
		for _, value := range v.cases("validRequest", "r."+strings.ToUpper(field[:1])+field[1:]) {
			m := v4Wire(t, base)
			schemaRequest(m)[field] = value
			if _, err := v4Decode(t, m); err != nil || !schemaAccepts(s, m) {
				t.Fatalf("canonical request %s=%s rejected: %v", field, value, err)
			}
		}
	}
	for _, field := range []string{"operation", "source", "coalescing", "result"} {
		for _, value := range v.cases("validQuery", "q."+strings.ToUpper(field[:1])+field[1:]) {
			m := v4Wire(t, base)
			schemaQuery(m)[field] = value
			if _, err := v4Decode(t, m); err != nil || !schemaAccepts(s, m) {
				t.Fatalf("canonical query %s=%s rejected: %v", field, value, err)
			}
		}
	}
	for _, value := range v.cases("validPoints", "p.Coverage") {
		m := v4Wire(t, base)
		r := schemaRequest(m)
		p := r["points"].(map[string]any)
		p["coverage"] = value
		if value == string(CoverageMeasured) || value == string(CoverageEstimated) {
			r["protocol"], r["resource"] = "graphql", "graphql"
			p["unit"], p["value"] = string(UnitGitHubPoints), float64(0)
			if value == string(CoverageEstimated) {
				p["method"], p["version"] = "estimate", "v-one"
			}
		}
		if _, err := v4Decode(t, m); err != nil || !schemaAccepts(s, m) {
			t.Fatalf("canonical points coverage %s rejected: %v", value, err)
		}
	}
	for _, field := range []string{"role", "deskRole"} {
		fn := "roleValid"
		if field == "deskRole" {
			fn = "deskRoleValid"
		}
		for _, value := range v.cases(fn, "s") {
			m := v4Wire(t, base)
			a := facetData(m, "attribution")
			a[field] = value
			a["coverage"].(map[string]any)[field] = "measured"
			if field == "role" {
				m["role"] = value
			}
			if _, err := v4Decode(t, m); err != nil || !schemaAccepts(s, m) {
				t.Fatalf("canonical attribution %s=%s rejected: %v", field, value, err)
			}
		}
	}
}

func schemaRequestOverflow(m map[string]any) {
	v := loadSchemaVocabulary()
	origins, outcomes, protocols := v.cases("validRequest", "r.Origin"), v.cases("validRequest", "r.Outcome"), v.cases("validRequest", "r.Protocol")
	one := schemaRequest(m)
	a := []any{}
	for i := 0; i < 65; i++ {
		group := map[string]any{}
		for k, value := range one {
			group[k] = value
		}
		group["origin"] = origins[i%len(origins)]
		group["outcome"] = outcomes[(i/len(origins))%len(outcomes)]
		group["protocol"] = protocols[(i/(len(origins)*len(outcomes)))%len(protocols)]
		a = append(a, group)
	}
	facetData(m, "requests")["requests"] = a
}
