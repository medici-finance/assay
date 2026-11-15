package deskkit

import (
	"encoding/json"
	"strings"
	"testing"
)

// mrWire builds a MergeRequirementsRequest response. runs are rollup nodes; classic and
// ruleset name the base branch's required contexts.
func mrWire(t *testing.T, mergeable string, review any, classic, ruleset []string, runs ...map[string]any) []byte {
	t.Helper()
	rsc := []map[string]any{}
	for _, c := range ruleset {
		rsc = append(rsc, map[string]any{"context": c})
	}
	nodes := []any{map[string]any{"type": "PULL_REQUEST", "parameters": nil}}
	if len(ruleset) > 0 {
		nodes = append(nodes, map[string]any{"type": "REQUIRED_STATUS_CHECKS",
			"parameters": map[string]any{"requiredStatusChecks": rsc}})
	}
	var bpr any
	if classic != nil {
		bpr = map[string]any{"requiredStatusCheckContexts": classic}
	}
	if runs == nil {
		runs = []map[string]any{}
	}
	b, err := json.Marshal(map[string]any{"data": map[string]any{"repository": map[string]any{"pullRequest": map[string]any{
		"mergeable": mergeable, "reviewDecision": review, "headRefOid": "abc123",
		"baseRef": map[string]any{"branchProtectionRule": bpr,
			"rules": map[string]any{"totalCount": len(nodes), "nodes": nodes}},
		"commits": map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{"oid": "abc123",
			"statusCheckRollup": map[string]any{"contexts": map[string]any{"totalCount": len(runs), "nodes": runs}}}}}},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func checkRun(name, status, conclusion, started string, required bool) map[string]any {
	return map[string]any{"__typename": "CheckRun", "name": name, "status": status, "conclusion": conclusion,
		"startedAt": started, "isRequired": required}
}

// The decode reads every requirement source: the ruleset and classic contexts, the rollup's
// isRequired flags, and the latest run per name (#6508).
//
// regression: #6508
func TestDecodeMergeReqs(t *testing.T) {
	raw := mrWire(t, "MERGEABLE", "APPROVED", []string{"lint"}, []string{"build-test", "changelog"},
		checkRun("changelog", "COMPLETED", "CANCELLED", "2026-01-01T00:00:00Z", true),
		checkRun("changelog", "COMPLETED", "SUCCESS", "2026-01-01T00:05:00Z", true),
		checkRun("lint", "IN_PROGRESS", "", "2026-01-01T00:01:00Z", false),
		checkRun("leak-sweep", "COMPLETED", "SUCCESS", "2026-01-01T00:02:00Z", true),
		checkRun("optional", "COMPLETED", "FAILURE", "2026-01-01T00:02:00Z", false),
		map[string]any{"__typename": "StatusContext", "context": "ci/legacy", "state": "PENDING",
			"createdAt": "2026-01-01T00:00:00Z", "isRequired": true})
	m, err := DecodeMergeRequirements(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range m.Required {
		got[c.Name] = c.State
	}
	want := map[string]string{"lint": "IN_PROGRESS", "build-test": "ABSENT", "changelog": "SUCCESS",
		"leak-sweep": "SUCCESS", "ci/legacy": "PENDING"}
	if len(got) != len(want) {
		t.Fatalf("required = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("required[%s] = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
	if !m.Complete || m.Mergeable != "MERGEABLE" || m.ReviewDecision != "APPROVED" || m.HeadSHA != "abc123" {
		t.Errorf("read = %+v", m)
	}
	un := strings.Join(m.Unmet(), "\n")
	for _, w := range []string{"lint is IN_PROGRESS", "build-test has not reported", "ci/legacy is PENDING"} {
		if !strings.Contains(un, w) {
			t.Errorf("unmet = %q, want %q", un, w)
		}
	}
	if strings.Contains(un, "optional") || strings.Contains(un, "changelog") {
		t.Errorf("unmet = %q names a met or optional check", un)
	}
}

// A null review decision (no review required) with every requirement met has nothing unmet.
func TestDecodeMergeReqsAllMet(t *testing.T) {
	m, err := DecodeMergeRequirements(mrWire(t, "MERGEABLE", nil, nil, []string{"build-test"},
		checkRun("build-test", "COMPLETED", "SUCCESS", "2026-01-01T00:00:00Z", true)))
	if err != nil {
		t.Fatal(err)
	}
	if u := m.Unmet(); len(u) != 0 {
		t.Errorf("unmet = %q, want none", u)
	}
}

// Anything the read cannot show is never a pass: an error, a missing PR, a stale head, or a
// truncated connection.
//
// regression: #6508
func TestDecodeMergeReqsFailClosed(t *testing.T) {
	for name, raw := range map[string]string{
		"graphql error": `{"errors":[{"message":"Resource not accessible by integration"}]}`,
		"no pr":         `{"data":{"repository":{"pullRequest":null}}}`,
		"stale head": `{"data":{"repository":{"pullRequest":{"mergeable":"MERGEABLE","headRefOid":"abc123",` +
			`"commits":{"nodes":[{"commit":{"oid":"0ld"}}]}}}}}`,
		"garbage": `not json`,
	} {
		if m, err := DecodeMergeRequirements([]byte(raw)); err == nil {
			t.Errorf("%s: read %+v, want an error", name, m)
		}
	}
	m, err := DecodeMergeRequirements([]byte(`{"data":{"repository":{"pullRequest":{"mergeable":"MERGEABLE",` +
		`"headRefOid":"abc123","baseRef":null,"commits":{"nodes":[{"commit":{"oid":"abc123",` +
		`"statusCheckRollup":{"contexts":{"totalCount":300,"nodes":[]}}}}]}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Complete || len(m.Unmet()) == 0 {
		t.Errorf("a null base ref and a truncated rollup read Complete=%v unmet=%q, want incomplete and unmet", m.Complete, m.Unmet())
	}
}
