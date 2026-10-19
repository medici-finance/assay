package productintent

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func fixture(t *testing.T) ([]byte, Document) {
	t.Helper()
	data, err := os.ReadFile("../testdata/productintent/record.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return data, d
}
func marshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func evidenceFor(r *Record) Evidence {
	e := Evidence{Authorities: []string{"product-council"}}
	for _, d := range r.Directions {
		e.Decisions = append(e.Decisions, Decision{d.Decision, Revision{r.Subject, "direction", d.Revision}, "product-council", "approved", "synthetic-decision", true})
	}
	for _, target := range r.Targets {
		e.Decisions = append(e.Decisions, Decision{target.Decision, Revision{r.Subject, "target", target.Revision}, "product-council", "approved", "synthetic-decision", true})
	}
	return e
}

// regression: F-product-intent
func TestProductIntentApprovalAndUnknown(t *testing.T) {
	_, d := fixture(t)
	r := d.Record()
	if got := Resolve(d, Evidence{}, "accepted"); got.Targets[0].Approval != "unknown" || len(got.Targets[1].Observed) != 0 {
		t.Fatalf("self claim manufactured evidence: %+v", got)
	}
	e := evidenceFor(r)
	if Resolve(d, e, "").Targets[0].Approval != "unknown" {
		t.Fatal("unknown requirement status approved")
	}
	got := Resolve(d, e, "accepted")
	if got.Targets[0].Approval != "approved" || got.Targets[1].Approval != "approved" || got.Targets[0].Direction.Revision != "d1" || got.Targets[1].Direction.Revision != "d2" {
		t.Fatalf("exact decision bindings lost: %+v", got)
	}
	cases := map[string]func(*Evidence){
		"unavailable":           func(e *Evidence) { e.Decisions[2].Available = false },
		"wrong-revision":        func(e *Evidence) { e.Decisions[2].Subject.Revision = "t2" },
		"wrong-repo":            func(e *Evidence) { e.Decisions[2].Subject.Subject.Repository = "other/product" },
		"wrong-kind":            func(e *Evidence) { e.Decisions[2].Subject.Kind = "direction" },
		"wrong-source":          func(e *Evidence) { e.Decisions[2].Source = "" },
		"unauthorized":          func(e *Evidence) { e.Authorities = nil },
		"unavailable-direction": func(e *Evidence) { e.Decisions[0].Available = false },
		// One decision reference binds exactly one subject: an entry citing the
		// same reference for another binding is a conflicting vote, not a skip.
		"shared-reference": func(e *Evidence) {
			other := e.Decisions[2]
			other.Subject.Revision = "t2"
			e.Decisions = append(e.Decisions, other)
		},
		"conflict": func(e *Evidence) {
			other := e.Decisions[2]
			other.Verdict = "rejected"
			e.Decisions = append(e.Decisions, other)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := evidenceFor(r)
			mutate(&e)
			got := Resolve(d, e, "accepted")
			if got.Targets[0].Approval != "unknown" || got.Targets[0].State != "unknown" {
				t.Fatalf("bad evidence approved: %+v", got.Targets[0])
			}
		})
	}
	e = evidenceFor(r)
	e.Decisions[2].Verdict = "rejected"
	if Resolve(d, e, "accepted").Targets[0].Approval != "rejected" {
		t.Fatal("rejection lost")
	}
	r.Targets[0].Approved = nil
	withoutClaim, err := Decode(marshal(t, r))
	if err != nil {
		t.Fatal(err)
	}
	if Resolve(withoutClaim, evidenceFor(r), "accepted").Targets[0].Approval != "approved" {
		t.Fatal("claim boolean required for source-backed decision")
	}
	for _, status := range []string{"active", "withdrawn"} {
		r.Status = status
		doc, err := Decode(marshal(t, r))
		if err != nil {
			t.Fatal(err)
		}
		got := Resolve(doc, evidenceFor(r), "withdrawn")
		if got.State != "withdrawn" || got.Targets[0].State != "withdrawn" || len(got.Targets) != 2 || len(got.Targets[0].Observed) != 1 {
			t.Fatalf("tombstone lost history: %+v", got)
		}
	}
	future, err := Decode([]byte(`{"schema":"product-intent-v2","mandatory":true,"future":{"value":42}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := Resolve(future, e, "accepted"); got.State != "unsupported" || !got.Mandatory || len(got.Targets) != 0 || !strings.Contains(string(future.Raw()), "future") {
		t.Fatalf("future invented support: %+v", got)
	}
	for _, data := range []string{`{"schema":"product-intent-v1","schema":"product-intent-v2"}`, `null`, `{} {}`, `{"schema":1}`} {
		if _, err := Decode([]byte(data)); err == nil {
			t.Fatalf("malformed envelope accepted: %s", data)
		}
	}
	// Mutating a returned copy cannot overwrite retained direction/evidence history.
	copy := d.Record()
	copy.Directions[0].Rationale = "overwritten"
	copy.Observed[0].Source = "overwritten"
	*copy.Targets[0].Approved = false
	if d.Record().Directions[0].Rationale == "overwritten" || d.Record().Observed[0].Source == "overwritten" || !*d.Record().Targets[0].Approved {
		t.Fatal("record accessor aliases stored snapshot")
	}
	raw := d.Raw()
	raw[0] = 'x'
	if d.Raw()[0] != '{' {
		t.Fatal("raw accessor aliases stored snapshot")
	}
	original := Resolve(d, evidenceFor(d.Record()), "accepted")
	copyResult := original
	copyResult.Targets[0].Direction.Rationale = "changed"
	if reflect.DeepEqual(Resolve(d, evidenceFor(d.Record()), "accepted"), copyResult) {
		t.Fatal("resolve mutates retained snapshot")
	}
}

// regression: F-product-intent
func TestProductHistory(t *testing.T) {
	_, d := fixture(t)
	after := d.Record()
	before := d.Record()
	before.Directions = before.Directions[:1]
	before.Targets = before.Targets[:1]
	if err := CheckHistory(*before, *after); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Record){
		"direction":       func(r *Record) { r.Targets[0].DirectionRevision = "d2" },
		"evidence":        func(r *Record) { r.Observed[0].Source = "replacement" },
		"prior-direction": func(r *Record) { r.Directions[0].Rationale = "replacement" },
		"remove-target":   func(r *Record) { r.Targets = r.Targets[1:] },
		"identity":        func(r *Record) { r.Subject.Repository = "other/product" },
		"reuse-revision":  func(r *Record) { r.Targets[1].Revision = r.Targets[0].Revision },
	} {
		t.Run(name, func(t *testing.T) {
			r := d.Record()
			mutate(r)
			if CheckHistory(*before, *r) == nil {
				t.Fatal("history mutation accepted")
			}
		})
	}
	before = d.Record()
	before.Status = "withdrawn"
	after = d.Record()
	if CheckHistory(*before, *after) == nil {
		t.Fatal("withdrawn identity resurrected")
	}
	after.Status = "withdrawn"
	after.Owner = "new-owner"
	if err := CheckHistory(*before, *after); err != nil {
		t.Fatal(err)
	}
}

// regression: F-product-intent
func TestProductIntentSchemaParity(t *testing.T) {
	data, _ := fixture(t)
	var base map[string]any
	if err := json.Unmarshal(data, &base); err != nil {
		t.Fatal(err)
	}
	type sample struct {
		Name  string
		Data  json.RawMessage
		Valid bool
	}
	corpus := []sample{{"valid", data, true}}
	add := func(name string, valid bool, mutate func(map[string]any)) {
		var value map[string]any
		_ = json.Unmarshal(data, &value)
		mutate(value)
		corpus = append(corpus, sample{name, marshal(t, value), valid})
	}
	add("withdrawn", true, func(v map[string]any) { v["status"] = "withdrawn" })
	add("missing-revision", false, func(v map[string]any) { delete(v["targets"].([]any)[1].(map[string]any), "revision") })
	add("wrong-operational-kind", false, func(v map[string]any) {
		v["operational-references"].([]any)[0].(map[string]any)["kind"] = "workflow-pattern"
	})
	add("unknown-field", false, func(v map[string]any) { v["policy"] = "approve" })
	add("case-folded-schema", false, func(v map[string]any) { v["Schema"] = v["schema"]; delete(v, "schema") })
	add("case-folded-target", false, func(v map[string]any) {
		target := v["targets"].([]any)[0].(map[string]any)
		target["Revision"] = target["revision"]
		delete(target, "revision")
	})
	add("case-alias", false, func(v map[string]any) { v["Schema"] = v["schema"] })
	add("future-version", false, func(v map[string]any) { v["schema"] = "product-intent-v2"; v["mandatory"] = true })
	add("wrong-claim-type", false, func(v map[string]any) { v["targets"].([]any)[0].(map[string]any)["approved"] = "true" })
	add("blank-owner", false, func(v map[string]any) { v["owner"] = " \t\n" })
	add("null-optional", false, func(v map[string]any) { v["observed"] = nil })
	add("empty-targets", false, func(v map[string]any) { v["targets"] = []any{} })
	add("wrong-subject", false, func(v map[string]any) { v["subject"].(map[string]any)["requirement"] = "#42" })
	add("missing-direction", false, func(v map[string]any) { delete(v["targets"].([]any)[0].(map[string]any), "direction-revision") })
	add("wrong-assessment-kind", false, func(v map[string]any) {
		v["observed"].([]any)[0].(map[string]any)["assessment"].(map[string]any)["kind"] = "decision"
	})
	// Every required shape field is removed independently, and null cannot coerce
	// to a zero value. These cases use the same frozen corpus in both consumers.
	for _, field := range []string{"schema", "subject", "kind", "owner", "status", "directions", "targets"} {
		f := field
		add("missing-"+f, false, func(v map[string]any) { delete(v, f) })
		add("null-"+f, false, func(v map[string]any) { v[f] = nil })
	}
	for _, section := range []string{"directions", "targets", "observed"} {
		fields := base[section].([]any)[0].(map[string]any)
		for field := range fields {
			if field == "approved" {
				continue
			}
			section, field := section, field
			add("missing-"+section+"-"+field, false, func(v map[string]any) { delete(v[section].([]any)[0].(map[string]any), field) })
		}
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "corpus.json")
	output := filepath.Join(dir, "results.json")
	if err := os.WriteFile(input, marshal(t, corpus), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "-count=1", "-timeout=45s", "-run", "^TestProductSchemaBridge$", ".")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "GOWORK=off", "PRODUCT_INTENT_CORPUS="+input, "PRODUCT_INTENT_RESULTS="+output)
	if log, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual conform schema validator bridge failed: %v\n%s", err, log)
	}
	results, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var accepted []bool
	if err := json.Unmarshal(results, &accepted); err != nil {
		t.Fatal(err)
	}
	if len(accepted) != len(corpus) {
		t.Fatalf("schema validated %d/%d cases", len(accepted), len(corpus))
	}
	for i, c := range corpus {
		t.Run(c.Name, func(t *testing.T) {
			doc, err := Decode(c.Data)
			parsed := err == nil && doc.Record() != nil
			if parsed != c.Valid || accepted[i] != c.Valid {
				t.Errorf("valid=%v parser=%v (%v) actual-schema=%v", c.Valid, parsed, err, accepted[i])
			}
		})
	}
	t.Logf("actual conform schema and parser agree on %d cases", len(corpus))
}

// pureImports is the closed allow-list of standard-library packages the pure
// package may import. Anything else fails closed: io/ioutil, io/fs, os and its
// subpackages, syscall, unsafe, plugin, runtime/debug, network and third-party.
var pureImports = map[string]bool{
	"bytes": true, "encoding/json": true, "fmt": true, "io": true,
	"reflect": true, "regexp": true, "strings": true, "unicode/utf8": true,
}

func boundaryImports(source []byte) error {
	f, err := parser.ParseFile(token.NewFileSet(), "fixture.go", source, parser.ImportsOnly)
	if err != nil {
		return err
	}
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return err
		}
		if !pureImports[path] {
			return &ErrorBoundary{path}
		}
	}
	return nil
}

type ErrorBoundary struct{ path string }

func (e *ErrorBoundary) Error() string { return "non-pure productintent import: " + e.path }

// regression: F-product-intent
func TestProductBoundary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := boundaryImports(data); err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count == 0 {
		t.Fatal("no production files checked")
	}
	if err := boundaryImports([]byte("package fixture\nimport \"fmt\"")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"os", "net/http", "os/exec", "path/filepath", "example.invalid/database", "io/ioutil", "io/fs", "os/user", "os/signal", "plugin", "unsafe", "runtime/debug", "syscall"} {
		if boundaryImports([]byte("package fixture\nimport "+strconv.Quote(path))) == nil {
			t.Fatalf("planted sibling import escaped: %s", path)
		}
	}
}

// regression: F-unsupported-envelope-null
func TestProductUnsupportedNull(t *testing.T) {
	for _, data := range []string{
		`{"schema":"product-intent-v2","mandatory":false,"future":null}`,
		`{"schema":"product-intent-v2","mandatory":true,"future":{"nested":[null,{"x":null}]}}`,
	} {
		d, err := Decode([]byte(data))
		if err != nil {
			t.Fatalf("unsupported payload evaluated: %s: %v", data, err)
		}
		if got := Resolve(d, Evidence{}, "accepted"); got.State != "unsupported" || d.Record() != nil || string(d.Raw()) != data {
			t.Fatalf("unsupported envelope not retained: %s: %+v", data, got)
		}
	}
	// Well-formedness still binds every version.
	for _, data := range []string{
		`null`,
		`{"schema":"product-intent-v2","future":{"a":1,"a":2}}`,
		`{"schema":"product-intent-v2","future":null} {}`,
		`{"schema":"product-intent-v2","mandatory":null}`,
		`{"schema":null,"future":null}`,
	} {
		if _, err := Decode([]byte(data)); err == nil {
			t.Fatalf("malformed envelope accepted: %s", data)
		}
	}
	// A supported record still refuses a nested null as such.
	if _, err := Decode([]byte(`{"schema":"product-intent-v1","observed":null}`)); err == nil || !strings.Contains(err.Error(), "null") {
		t.Fatalf("supported nested null not refused as null: %v", err)
	}
}
