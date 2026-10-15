package contexthandoff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	fixtureRoot         = "../testdata/contexthandoff"
	handoffSchemaPath   = "../../schemas/context-handoff-v1.json"
	manifestSchemaPath  = "../../schemas/amendment-manifest-v1.json"
	specPath            = "../../spec/context-handoff-v1.md"
	pairedHandoffName   = "approved-amend-and-split"
	pairedHandoffPath   = fixtureRoot + "/handoff/valid/" + pairedHandoffName + ".json"
	pairedManifestPath  = fixtureRoot + "/manifest/valid/patch-and-follow-up.json"
	draftHandoffPath    = fixtureRoot + "/handoff/valid/draft-unresolved.json"
	fixtureLabelValid   = "valid"
	fixtureLabelInvalid = "invalid"
	fixtureLabelPkgOnly = "package-only"
)

// The seven handoff records of the future-state model. The schema titles its
// record definitions with exactly these names and the spec names each one.
var handoffRecords = []string{
	"Target and intent",
	"Discovery receipt",
	"Impact obligation",
	"Amendment obligation",
	"Preserved history",
	"Active-work binding",
	"Decisions and readiness",
}

// packageOnlyRules are the refusals that rest, at least in part, on a rule no
// JSON schema can state (spec §"What the schema cannot express"). A schema
// that accepts what the decoder refuses is drift unless the refusal is one of
// these.
var packageOnlyRules = map[string]bool{
	RuleDuplicateKey:     true,
	RuleApproval:         true,
	RuleCoverage:         true,
	RuleDuplicateID:      true,
	RuleImpactUnbound:    true,
	RuleReadiness:        true,
	RuleNotApproved:      true,
	RuleManifestMismatch: true,
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func pairedHandoff(t *testing.T) *Handoff {
	t.Helper()
	h, err := DecodeHandoff(readFile(t, pairedHandoffPath))
	if err != nil {
		t.Fatalf("paired handoff fixture refused: %v", err)
	}
	return h
}

// decodeKind runs the production decoder for a fixture kind. A manifest is
// decoded and then bound to the paired handoff, which is what a consumer does.
func decodeKind(t *testing.T, kind string, data []byte) error {
	t.Helper()
	switch kind {
	case "handoff":
		_, err := DecodeHandoff(data)
		return err
	case "manifest":
		m, err := DecodeManifest(data)
		if err != nil {
			return err
		}
		return CheckManifest(pairedHandoff(t), m)
	}
	t.Fatalf("unknown fixture kind %q", kind)
	return nil
}

// regression: statusgen/19 Verify row 6
func TestContextHandoffSchemaFixtures(t *testing.T) {
	schemas := map[string]*testSchema{
		"handoff":  loadSchema(t, handoffSchemaPath),
		"manifest": loadSchema(t, manifestSchemaPath),
	}
	spec := string(readFile(t, specPath))

	// spec -> schema: every schema definition and every record is named in
	// the spec, and the record titles are exactly the seven records.
	var titles []string
	for kind, s := range schemas {
		defs, _ := s.root["$defs"].(map[string]any)
		for name, def := range defs {
			if !strings.Contains(spec, "`"+name+"`") {
				t.Errorf("%s schema definition `%s` is not named in the spec", kind, name)
			}
			if title, ok := def.(map[string]any)["title"].(string); ok {
				titles = append(titles, title)
				if !strings.Contains(spec, title) {
					t.Errorf("%s schema record %q is not named in the spec", kind, title)
				}
			}
		}
		props, _ := s.root["properties"].(map[string]any)
		for name := range props {
			if !strings.Contains(spec, "`"+name+"`") {
				t.Errorf("%s schema top-level field `%s` is not named in the spec", kind, name)
			}
		}
	}
	sort.Strings(titles)
	want := append([]string(nil), handoffRecords...)
	sort.Strings(want)
	if fmt.Sprint(titles) != fmt.Sprint(want) {
		t.Errorf("schema record titles = %q, want exactly the seven handoff records %q", titles, want)
	}
	for _, rec := range handoffRecords {
		if !strings.Contains(spec, rec) {
			t.Errorf("spec does not name the %q record", rec)
		}
	}
	for _, rule := range allRules {
		if !strings.Contains(spec, "`"+rule+"`") {
			t.Errorf("refusal rule `%s` is not listed in the spec", rule)
		}
	}

	// schema <-> package: each labelled fixture goes through both.
	counts := map[string]int{}
	for _, kind := range []string{"handoff", "manifest"} {
		for _, label := range []string{fixtureLabelValid, fixtureLabelInvalid, fixtureLabelPkgOnly} {
			files, err := filepath.Glob(filepath.Join(fixtureRoot, kind, label, "*.json"))
			if err != nil {
				t.Fatal(err)
			}
			if len(files) == 0 {
				t.Errorf("no %s/%s fixtures", kind, label)
			}
			for _, f := range files {
				counts[kind+"/"+label]++
				checkFixture(t, schemas[kind], kind, label, f)
			}
		}
	}
	t.Logf("fixtures: %v", counts)

	// Drift sweep: from every valid fixture, delete each object key and null
	// each value, at every depth. The decoder must never accept what the
	// schema refuses, and the schema may accept what the decoder refuses only
	// for a package-only rule. Dropping a required field from the schema alone
	// makes the schema accept a deletion the decoder refuses as missing-field.
	variants := 0
	for _, kind := range []string{"handoff", "manifest"} {
		files, _ := filepath.Glob(filepath.Join(fixtureRoot, kind, fixtureLabelValid, "*.json"))
		for _, f := range files {
			tree := parseTree(t, readFile(t, f))
			for _, v := range mutations(tree) {
				variants++
				data, err := json.Marshal(v.tree)
				if err != nil {
					t.Fatal(err)
				}
				decErr := decodeKind(t, kind, data)
				schemaErr := schemas[kind].Accepts(data)
				switch {
				case decErr == nil && schemaErr != nil:
					t.Errorf("%s: %s: decoder accepts, schema refuses: %v", filepath.Base(f), v.desc, schemaErr)
				case decErr != nil && schemaErr == nil && !packageOnlyRules[RuleOf(decErr)]:
					t.Errorf("%s: %s: schema accepts, decoder refuses with %v (not a package-only rule; schema drift)", filepath.Base(f), v.desc, decErr)
				}
			}
		}
	}
	t.Logf("drift variants checked: %d", variants)
}

var fixtureRuleRe = regexp.MustCompile(`^([a-z-]+)--[a-z0-9-]+\.json$`)

func checkFixture(t *testing.T, schema *testSchema, kind, label, path string) {
	t.Helper()
	name := filepath.Base(path)
	data := readFile(t, path)
	decErr := decodeKind(t, kind, data)
	schemaErr := schema.Accepts(data)
	if label == fixtureLabelValid {
		if decErr != nil {
			t.Errorf("%s/%s/%s: decoder refuses a valid fixture: %v", kind, label, name, decErr)
		}
		if schemaErr != nil {
			t.Errorf("%s/%s/%s: schema refuses a valid fixture: %v", kind, label, name, schemaErr)
		}
		return
	}
	m := fixtureRuleRe.FindStringSubmatch(name)
	if m == nil {
		t.Errorf("%s/%s/%s: a negative fixture is named <rule>--<description>.json", kind, label, name)
		return
	}
	if got := RuleOf(decErr); got != m[1] {
		t.Errorf("%s/%s/%s: decoder rule = %q (%v), want %q", kind, label, name, got, decErr, m[1])
	}
	switch label {
	case fixtureLabelInvalid:
		if schemaErr == nil {
			t.Errorf("%s/%s/%s: schema accepts a fixture both layers must refuse", kind, label, name)
		}
	case fixtureLabelPkgOnly:
		if schemaErr != nil {
			t.Errorf("%s/%s/%s: schema refuses a package-only fixture (%v); move it to invalid/", kind, label, name, schemaErr)
		}
		if !packageOnlyRules[m[1]] {
			t.Errorf("%s/%s/%s: rule %q is not a package-only rule", kind, label, name, m[1])
		}
	}
}

func parseTree(t *testing.T, data []byte) any {
	t.Helper()
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

type mutation struct {
	desc string
	tree any
}

// mutations returns, for every object key at every depth, a copy with that
// key deleted and a copy with its value set to null, and for every array
// element a copy with the element set to null.
func mutations(tree any) []mutation {
	var out []mutation
	var walk func(path []any, v any)
	walk = func(path []any, v any) {
		switch t := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				p := append(append([]any(nil), path...), k)
				out = append(out,
					mutation{"delete " + pathString(p), edit(tree, p, nil, true)},
					mutation{"null " + pathString(p), edit(tree, p, nil, false)})
				walk(p, t[k])
			}
		case []any:
			for i, e := range t {
				p := append(append([]any(nil), path...), i)
				out = append(out, mutation{"null " + pathString(p), edit(tree, p, nil, false)})
				walk(p, e)
			}
		}
	}
	walk(nil, tree)
	return out
}

func pathString(p []any) string {
	var b strings.Builder
	b.WriteString("$")
	for _, s := range p {
		switch s := s.(type) {
		case string:
			b.WriteString("." + s)
		case int:
			fmt.Fprintf(&b, "[%d]", s)
		}
	}
	return b.String()
}

// edit returns a deep copy of tree with the value at path replaced (or, for
// del, the object key removed).
func edit(tree any, path []any, val any, del bool) any {
	c := deepCopy(tree)
	if len(path) == 0 {
		return val
	}
	parent := c
	for _, s := range path[:len(path)-1] {
		switch s := s.(type) {
		case string:
			parent = parent.(map[string]any)[s]
		case int:
			parent = parent.([]any)[s]
		}
	}
	switch last := path[len(path)-1].(type) {
	case string:
		if del {
			delete(parent.(map[string]any), last)
		} else {
			parent.(map[string]any)[last] = val
		}
	case int:
		parent.([]any)[last] = val
	}
	return c
}

func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, e := range t {
			m[k] = deepCopy(e)
		}
		return m
	case []any:
		a := make([]any, len(t))
		for i, e := range t {
			a[i] = deepCopy(e)
		}
		return a
	}
	return v
}
