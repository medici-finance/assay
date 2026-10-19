package cellop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellop/schemacheck"
)

// The validator lives in schemacheck so cmd/cellctl's tests check the documents `up --json` prints
// against the same schema.

var repoRoot = filepath.Join("..", "..", "..", "..")

type schemaValidator struct{ v *schemacheck.Validator }

func loadSchema(t *testing.T) *schemaValidator {
	t.Helper()
	v, err := schemacheck.Load(filepath.Join(repoRoot, "schemas", "cell-op-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &schemaValidator{v: v}
}

func decodeDoc(raw []byte) (any, error) { return schemacheck.Decode(raw) }

func (sv *schemaValidator) validate(doc any) []string { return sv.v.Validate(doc) }

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

// A contract example that drifts from the schema, or a plan example whose digest no longer recomputes, is caught (spec/cell-op-v1.md).
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

// A validator that accepts anything would make TestContractExamplesValidate vacuous (spec/cell-op-v1.md).
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
