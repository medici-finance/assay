package deskkit

import (
	"strings"
	"testing"
)

// TestFirstIncompleteFieldAgreesWithComplete holds Complete (the yes/no gate) and
// FirstIncompleteField (the name of the first failing field) together: the helper is empty
// exactly when Complete is true, over every predicate.
func TestFirstIncompleteFieldAgreesWithComplete(t *testing.T) {
	full := func(mut func(*WakeReceipt)) WakeReceipt {
		r := WakeReceipt{
			Schema: SchemaWakeV1, ID: "vw1-0123456789abcdef", BlockerKind: BlockerImplementation,
			WakePredicate: WakeExplicitRecheck, RecheckReason: "r",
		}
		if mut != nil {
			mut(&r)
		}
		return r
	}
	cases := []struct {
		name string
		r    WakeReceipt
		want string
	}{
		{"complete", full(nil), ""},
		{"legacy", full(func(r *WakeReceipt) { r.Schema = "" }), "wake_schema"},
		{"no id", full(func(r *WakeReceipt) { r.ID = " " }), "receipt_id"},
		{"unknown kind", full(func(r *WakeReceipt) { r.BlockerKind = "mood" }), "blocker_kind"},
		{"no predicate", full(func(r *WakeReceipt) { r.WakePredicate = "" }), "wake_predicate"},
		{"no recheck reason", full(func(r *WakeReceipt) { r.RecheckReason = "" }), "recheck_reason"},
		{"no blocker ref", full(func(r *WakeReceipt) { r.WakePredicate = WakeReferencedActionDone }), "blocker_ref"},
		{"no deadline", full(func(r *WakeReceipt) { r.WakePredicate = WakeDeadlineReached }), "deadline"},
		{"no inputs", full(func(r *WakeReceipt) { r.WakePredicate = WakeRelevantInputChanged }), "inputs"},
	}
	for _, c := range cases {
		if got := c.r.FirstIncompleteField(); got != c.want {
			t.Errorf("%s: FirstIncompleteField = %q, want %q", c.name, got, c.want)
		}
		if complete := c.r.Complete(); complete != (c.want == "") {
			t.Errorf("%s: Complete = %v but FirstIncompleteField = %q", c.name, complete, c.want)
		}
	}
}

func TestCheckRecordKeys(t *testing.T) {
	ok := []string{
		`{"brief":"a/1","ts":"t"}`,
		`{"brief":"a/1","prs":[{"repo":"o/n","number":1},{"repo":"o/m","number":2}]}`,
	}
	for _, raw := range ok {
		if err := CheckRecordKeys([]byte(raw)); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
	bad := map[string]string{
		`{"ts":"1","ts":"2"}`:                                 "ts",
		`{"brief":"a/1","Brief":"b/2"}`:                       "rief",
		`{"prs":[],"PRS":[]}`:                                 "rs",
		`{"dispatch_ref":"x","Dispatch_Ref":"x"}`:             "ispatch_",
		`{"prs":[{"repo":"o/n","Repo":"o/m"}]}`:               "epo",
		`{"row_results":[{"row":1,"row":2,"result":"pass"}]}`: "row",
		`{"x":{"a":1,"A":2}}`:                                 "a",
	}
	for raw, frag := range bad {
		err := CheckRecordKeys([]byte(raw))
		if err == nil {
			t.Errorf("%s: accepted", raw)
			continue
		}
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("%s: refusal %q does not name the key (%q)", raw, err, frag)
		}
	}
}

func TestParseJoinInput(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	pr := `{"repo":"o/n","number":3,"merge_sha":"` + sha + `"}`
	good := []string{
		`{"brief":"a/1"}`,
		`{"prs":[` + pr + `]}`,
		`{"prs":[],"prs_unknown_reason":"no-pr-delivery"}`,
		`{"verifier_tier":"any","started":"2026-10-06T14:00:00Z","finished":"2026-10-06T14:10:00Z","ts":"2026-10-06T14:20:00Z","rows_passed":1,"rows_total":2,"row_results":[{"row":1,"result":"pass"},{"row":2,"result":"could-not-run"}]}`,
		`{"dispatch_ref":"assay--x--1@20261006T141502Z.3fa9c01b7d2e"}`,
	}
	for _, raw := range good {
		if _, err := ParseJoinInput([]byte(raw)); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
	bad := map[string]string{
		`{"prs":[]}`: "prs_unknown_reason",
		`{"prs":[],"prs_unknown_reason":"because"}`:                                            "prs_unknown_reason",
		`{"prs":[` + pr + `],"prs_unknown_reason":"no-pr-delivery"}`:                           "prs_unknown_reason",
		`{"prs":[{"repo":"x","number":3,"merge_sha":"` + sha + `"}]}`:                          "repo",
		`{"prs":[{"repo":"o/n","number":0,"merge_sha":"` + sha + `"}]}`:                        "number",
		`{"prs":[{"repo":"o/n","number":-1,"merge_sha":"` + sha + `"}]}`:                       "number",
		`{"prs":[{"repo":"o/n","number":1.5,"merge_sha":"` + sha + `"}]}`:                      "number",
		`{"prs":[{"repo":"o/n","number":3,"merge_sha":"ABC"}]}`:                                "merge_sha",
		`{"prs":[{"withheld":"` + strings.Repeat("a", 64) + `"}]}`:                             "withheld",
		`{"dispatch_ref_sha256":"` + strings.Repeat("a", 64) + `"}`:                            "dispatch_ref_sha256",
		`{"dispatch_ref":"nonce-less"}`:                                                        "dispatch_ref",
		`{"verifier_tier":"gpt-6"}`:                                                            "verifier_tier",
		`{"started":"2026-10-06T14:11:00Z","finished":"2026-10-06T14:10:00Z"}`:                 "started",
		`{"rows_total":1,"row_results":[{"row":1,"result":"pass"},{"row":2,"result":"pass"}]}`: "row_results",
		`{"rows_total":1,"rows_passed":0,"row_results":[{"row":1,"result":"pass"}]}`:           "row_results",
		`{"rows_total":1,"row_results":[{"row":1,"result":"green"}]}`:                          "result",
	}
	// The entry cap is applied before any entry is parsed: an oversized list of malformed
	// entries is refused for its size, not for its first bad entry.
	oversized := `{"prs":[` + strings.TrimSuffix(strings.Repeat(`{"repo":"x"},`, 65), ",") + `]}`
	bad[oversized] = "more than 64"
	for raw, frag := range bad {
		_, err := ParseJoinInput([]byte(raw))
		if err == nil {
			t.Errorf("%s: accepted", raw)
			continue
		}
		if !strings.Contains(err.Error(), frag) {
			t.Errorf("%s: refusal %q does not name %q", raw, err, frag)
		}
	}
}

func TestCheckWakeV1RunFields(t *testing.T) {
	full := `{"ts":"2026-10-06T14:20:00Z","prs":[],"prs_unknown_reason":"no-pr-delivery","verifier_tier":"strong","started":"2026-10-06T14:00:00Z","finished":"2026-10-06T14:10:00Z"}`
	in, err := ParseJoinInput([]byte(full))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckWakeV1RunFields(in); err != nil {
		t.Errorf("complete run fields refused: %v", err)
	}
	for _, drop := range []string{`"verifier_tier":"strong",`, `"started":"2026-10-06T14:00:00Z",`, `,"finished":"2026-10-06T14:10:00Z"`, `"prs":[],"prs_unknown_reason":"no-pr-delivery",`} {
		raw := strings.Replace(full, drop, "", 1)
		in, err := ParseJoinInput([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if CheckWakeV1RunFields(in) == nil {
			t.Errorf("%s: a wake-v1 record missing %s was accepted", raw, drop)
		}
	}
}
