package reviewtest_test

// declaration_hold_test.go — the review-observation half of #2690. A row whose producer
// recorded a declaration hold must classify BLOCKED through BuildObservation and survive the
// JSON round trip (schema, strict decode, PlanObservation recompute) with no effect planned.
// Absence of the fact is the older v1 profile and keeps its old classification.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rf "github.com/medici-finance/assay/tools/desk/internal/reviewflow"
)

const dhReason = "a reviewer Undeclared-desk-decision line stands in the correctness lane"

// regression: #2690
func TestObservationDeclarationHold(t *testing.T) {
	var schema map[string]any
	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "..", "..", "schemas", "review-observation-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	for _, draft := range []bool{false, true} {
		for _, hold := range []string{"", dhReason} {
			in, policy := observationFixture()
			c := &in.Changes[0]
			approve(c)
			c.Facts.Change.IsDraft = draft
			c.Facts.DeclarationHold = hold
			o := observation(t, in, policy)
			got := o.Result.Rows[0]
			if hold == "" {
				if got.Action != "MERGE-NOW" && got.Action != "FLIP" {
					t.Fatalf("draft=%v control: action %s, want FLIP or MERGE-NOW", draft, got.Action)
				}
			} else if got.Action != "BLOCKED" || !strings.Contains(got.Note, "declaration hold") || !strings.Contains(got.Note, hold) {
				t.Fatalf("draft=%v held row: action %s (%s), want BLOCKED naming the hold", draft, got.Action, got.Note)
			}
			data, err := json.Marshal(o)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte(`"declarationHold"`)) != (hold != "") {
				t.Fatalf("draft=%v hold=%q: declarationHold presence on the wire is wrong", draft, hold)
			}
			var doc any
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			if err := checkSchema(schema, schema, doc); err != nil {
				t.Fatal(err)
			}
			decoded, err := rf.DecodeObservation(data)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := rf.PlanObservation(decoded, policy)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Result.Rows[0].Action != got.Action {
				t.Fatalf("draft=%v: recomputed %s, built %s", draft, plan.Result.Rows[0].Action, got.Action)
			}
			if hold != "" {
				if len(plan.Verbs) != 0 {
					t.Fatalf("draft=%v: a held row acquired an effect: %+v", draft, plan.Verbs)
				}
				nullData := bytes.Replace(data, []byte(`"declarationHold":`+mustJSON(t, hold)), []byte(`"declarationHold":null`), 1)
				if bytes.Equal(nullData, data) {
					t.Fatal("declarationHold absent from the encoded observation")
				}
				if _, err := rf.DecodeObservation(nullData); err == nil {
					t.Fatal("null declarationHold accepted")
				}
			}
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
