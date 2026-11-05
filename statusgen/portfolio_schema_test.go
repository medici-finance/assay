package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/statusgen/portfolio"
	"gopkg.in/yaml.v3"
)

// TestPortfolioSchemaParity holds schemas/portfolio-measurement-v1.json and the production
// decoder (statusgen/portfolio.Decode) in agreement over a corpus of shape-level
// documents, using the production conform validator rather than a second schema
// evaluator. Cross-field rules the schema cannot express (bucket exclusivity, source
// coverage, unit/quantity pairing, basis-point ceilings, interval order) are the
// decoder's alone; the corpus names one so the boundary is visible.
// regression: F-portfolio-measurement
func TestPortfolioSchemaParity(t *testing.T) {
	raw, err := os.ReadFile("../schemas/portfolio-measurement-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	schema, err := parseSchema(raw)
	if err != nil {
		t.Fatal(err)
	}
	var keywords func(map[string]any)
	keywords = func(node map[string]any) {
		for key, value := range node {
			if !schemaKeywords[key] {
				t.Fatalf("production conform does not support keyword %q", key)
			}
			switch key {
			case "properties":
				for _, child := range value.(map[string]any) {
					keywords(child.(map[string]any))
				}
			case "items":
				keywords(value.(map[string]any))
			}
		}
	}
	keywords(schema.raw)
	fixture, err := os.ReadFile("testdata/portfolio/snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(fixture)
	type sample struct {
		name, doc         string
		schemaOK, codecOK bool
	}
	corpus := []sample{
		{"fixture", doc, true, true},
		{"no limits", strings.Replace(doc, `"limits": [
      { "bucket": "features", "measure": "cost-micros", "max": 5000000, "authorization": "budget-approval-7" }
    ]`, `"limits": []`, 1), true, true},
		{"unknown member", strings.Replace(doc, `"tolerance-basis-points": 50,`, `"tolerance-basis-points": 50, "default-weight": 1000,`, 1), false, false},
		{"missing revision", strings.Replace(doc, `"revision": "p3",`, ``, 1), false, false},
		{"missing segments", strings.Replace(doc, `"segments": [`, `"other": [`, 1), false, false},
		{"other version", strings.Replace(doc, `portfolio-measurement/v1`, `portfolio-measurement/v2`, 1), false, false},
		{"proxy as target", strings.Replace(doc, `"measure": "active-seconds", "basis-points": 500`, `"measure": "elapsed-seconds-proxy", "basis-points": 500`, 1), false, false},
		{"unknown unit", strings.Replace(doc, `"unit": "minute",`, `"unit": "hour",`, 1), false, false},
		{"negative cost", strings.Replace(doc, `"cost-micros": 1200`, `"cost-micros": -1`, 1), false, false},
		{"local time", strings.Replace(doc, `2026-10-02T10:02:00Z`, `2026-10-02T10:02:00+01:00`, 1), false, false},
		{"no sources", strings.Replace(doc, `"sources": ["runner-ledger", "decision-log"],`, `"sources": [],`, 1), false, false},
		{"unqualified subject", strings.Replace(doc, `"repository": "example-org/product", "requirement": "REQ-feature-search" }],`, `"repository": "product", "requirement": "REQ-feature-search" }],`, 1), false, false},
		// Decoder-only: shape-valid, refused for its cross-field meaning.
		{"ambiguous bucket", strings.Replace(doc, `REQ-feature-search" }],`, `REQ-feature-search" }, { "repository": "example-org/product", "requirement": "REQ-reliability-core" }],`, 1), true, false},
	}
	for _, c := range corpus {
		if c.name != "fixture" && c.doc == doc {
			t.Fatalf("%s: control did not change the document", c.name)
		}
		var value map[string]any
		if err := yaml.Unmarshal([]byte(c.doc), &value); err != nil {
			t.Fatal(err)
		}
		problems := validateValue(schema, value, "")
		if got := len(problems) == 0; got != c.schemaOK {
			t.Fatalf("%s: schema accepted=%v, want %v: %v", c.name, got, c.schemaOK, problems)
		}
		_, err := portfolio.Decode([]byte(c.doc))
		if got := err == nil; got != c.codecOK {
			t.Fatalf("%s: decoder accepted=%v, want %v: %v", c.name, got, c.codecOK, err)
		}
	}
	// The decoder's ledger round-trips to JSON the schema-independent way a consumer reads it.
	s, err := portfolio.Decode(fixture)
	if err != nil {
		t.Fatal(err)
	}
	l, err := portfolio.Measure(s)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := json.Marshal(portfolio.Compare(s.Policy, l)); err != nil {
		t.Fatal(err)
	}
}
