package main

import (
	"encoding/json"
	"testing"
)

// fixtureRunFields are the run-metadata and join-key fields a verify-wake-v1 record must carry
// for a delivery that has no PR.
var fixtureRunFields = map[string]any{
	"prs":                []any{},
	"prs_unknown_reason": "no-pr-delivery",
	"verifier_tier":      "strong",
	"started":            "2000-01-01T00:00:00Z",
	"finished":           "2000-01-01T00:00:00Z",
	"rows_passed":        0,
	"rows_total":         1,
}

// withRunFields adds only the run-metadata and join-key fields to a legacy-shape line, for a test
// that lets the real receipt derivation supply the rest of the verify-wake-v1 receipt.
func withRunFields(t testing.TB, line string) string {
	t.Helper()
	return addFixtureFields(t, line, false)
}

// completeFail turns a legacy-shape non-pass fixture line into a complete verify-wake-v1 record
// by adding every field the writer now requires and that the line does not already carry. A key
// the line already holds is left alone, so a test can still pin one field to a specific value.
func completeFail(t testing.TB, line string) string {
	t.Helper()
	return addFixtureFields(t, line, true)
}

func addFixtureFields(t testing.TB, line string, receipt bool) string {
	t.Helper()
	m := map[string]any{}
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("fixture %q is not a JSON object: %v", line, err)
	}
	add := func(k string, v any) {
		if _, ok := m[k]; !ok {
			m[k] = v
		}
	}
	if receipt {
		add("wake_schema", "verify-wake-v1")
		add("receipt_id", "fixture-receipt")
		add("blocker_kind", "implementation")
		add("wake_predicate", "explicit-recheck-with-reason")
		add("recheck_reason", "fixture")
	}
	for k, v := range fixtureRunFields {
		add(k, v)
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
