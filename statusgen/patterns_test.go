package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// patterns_test.go — graph-execution/02. Covers the schema-parity gate, the two
// shipped patterns validating clean, and the four MUST-rule fixtures each
// failing for the reason they are named for (never a different rule, and never
// silently clean).

// TestPatternsRoleEffectPermissionsCoversTopologyRoles binds
// patternRoleEffectPermissions to the compiled topology derivation
// (topologyvalues.go's topologyAppRoles, itself bound to topology.yaml by
// TestTopologyValuesMatchSource): the pattern lint's role→effect-kind table
// MUST cover exactly the role set the topology declares, no more and no fewer,
// so a role added to or removed from topology.yaml is a red test here rather
// than a silently stale permission table.
func TestPatternsRoleEffectPermissionsCoversTopologyRoles(t *testing.T) {
	got := make([]string, 0, len(patternRoleEffectPermissions))
	for role := range patternRoleEffectPermissions {
		got = append(got, role)
	}
	want := append([]string(nil), topologyAppRoles...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("patternRoleEffectPermissions covers roles %v, topologyAppRoles (from topology.yaml) says %v — "+
			"the pattern lint's permission table has drifted from the compiled topology derivation", got, want)
	}
}

// TestPatternsEmbeddedSchemaMatchesCommitted pins the embedded copy byte-identical
// to the canonical repo-root artifact, the same parity gate
// TestEmbeddedSchemaMatchesCommitted holds for the brief schemas (conform.go's
// embed cannot cross the module boundary, so schemas/workflow-pattern-v1.json is
// mirrored under statusgen/schemas/).
func TestPatternsEmbeddedSchemaMatchesCommitted(t *testing.T) {
	canonical, err := os.ReadFile(filepath.Join("..", "schemas", "workflow-pattern-v1.json"))
	if err != nil {
		t.Fatalf("reading canonical repo-root schema: %v", err)
	}
	if !bytes.Equal(canonical, embeddedPatternSchemaBytes()) {
		t.Error("statusgen/schemas/workflow-pattern-v1.json is not byte-identical to the canonical schemas/workflow-pattern-v1.json — re-copy the canonical artifact")
	}
}

// TestPatternsGoodFixturesLintClean is the positive control: both shipped
// patterns, copied as testdata fixtures, MUST lint clean — no schema violation
// and no MUST-rule violation.
func TestPatternsGoodFixturesLintClean(t *testing.T) {
	schema := mustParseEmbeddedPatternSchema(t)
	for _, name := range []string{"good-implementation-v1.yaml", "good-research-v1.yaml"} {
		path := filepath.Join("testdata", "patterns", name)
		state, violations := lintPatternFile(path, schema)
		if state != patternStateClean {
			t.Errorf("%s: want clean, got state=%d violations=%v", name, state, violations)
		}
	}
}

// TestPatternsShippedPatternsLintClean runs the full `statusgen patterns --lint`
// path (not just lintPatternFile) against the real committed
// spec/workflow-patterns/ directory, mirroring Verify row 2
// (`statusgen patterns --lint --root .`).
func TestPatternsShippedPatternsLintClean(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runPatterns([]string{"--lint", "--root", ".."}, &stdout, &stderr)
	if rc != patternsExitClean {
		t.Errorf("want exit 0, got %d\nstdout:\n%s\nstderr:\n%s", rc, stdout.String(), stderr.String())
	}
}

// TestPatternsEffectExceedsRoleIsProblem — Verify row 3. The fixture's
// `reviewer`-role `review` node declares a `push` effect; `push` is not among
// `reviewer`'s permitted effect kinds. The lint MUST exit 1, naming
// pattern-effect-exceeds-role, and MUST NOT report the fixture as clean.
func TestPatternsEffectExceedsRoleIsProblem(t *testing.T) {
	schema := mustParseEmbeddedPatternSchema(t)
	path := filepath.Join("testdata", "patterns", "bad-effect-exceeds-role.yaml")
	state, violations := lintPatternFile(path, schema)
	if state != patternStateFailed {
		t.Fatalf("want checked-failed, got state=%d violations=%v", state, violations)
	}
	if !anyContains(violations, rulePatternEffectExceedsRole) {
		t.Errorf("violations do not name %q:\n%s", rulePatternEffectExceedsRole, strings.Join(violations, "\n"))
	}
}

// TestPatternsReviewSameRoleIsProblem — Verify row 4. The fixture's `review`
// node (evidence kind: review) has the same role as `implement`, which produced
// its input. The lint MUST exit 1, naming pattern-review-same-role — the
// implementer<->reviewer separation is machine-checked in the pattern.
func TestPatternsReviewSameRoleIsProblem(t *testing.T) {
	schema := mustParseEmbeddedPatternSchema(t)
	path := filepath.Join("testdata", "patterns", "bad-review-same-role.yaml")
	state, violations := lintPatternFile(path, schema)
	if state != patternStateFailed {
		t.Fatalf("want checked-failed, got state=%d violations=%v", state, violations)
	}
	if !anyContains(violations, rulePatternReviewSameRole) {
		t.Errorf("violations do not name %q:\n%s", rulePatternReviewSameRole, strings.Join(violations, "\n"))
	}
}

// TestPatternsJoinNotCheckIsProblem — the `bad-join-not-check.yaml` fixture:
// `join` names a `decision`-kind node. The lint MUST exit 1, naming
// pattern-join-not-check.
func TestPatternsJoinNotCheckIsProblem(t *testing.T) {
	schema := mustParseEmbeddedPatternSchema(t)
	path := filepath.Join("testdata", "patterns", "bad-join-not-check.yaml")
	state, violations := lintPatternFile(path, schema)
	if state != patternStateFailed {
		t.Fatalf("want checked-failed, got state=%d violations=%v", state, violations)
	}
	if !anyContains(violations, rulePatternJoinNotCheck) {
		t.Errorf("violations do not name %q:\n%s", rulePatternJoinNotCheck, strings.Join(violations, "\n"))
	}
}

// TestPatternsRiskInputMissingVerdictIsProblem — the
// `bad-risk-input-missing-verdict.yaml` fixture: `risk-input` omits `human`. The
// lint MUST exit 1, naming pattern-risk-input-missing-verdict.
func TestPatternsRiskInputMissingVerdictIsProblem(t *testing.T) {
	schema := mustParseEmbeddedPatternSchema(t)
	path := filepath.Join("testdata", "patterns", "bad-risk-input-missing-verdict.yaml")
	state, violations := lintPatternFile(path, schema)
	if state != patternStateFailed {
		t.Fatalf("want checked-failed, got state=%d violations=%v", state, violations)
	}
	if !anyContains(violations, rulePatternRiskInputMissing) {
		t.Errorf("violations do not name %q:\n%s", rulePatternRiskInputMissing, strings.Join(violations, "\n"))
	}
}

// TestPatternsLintReportsCouldNotCheckOnMissingRoot is the three-state control: a
// --root with no spec/workflow-patterns directory is could-not-check, never a
// silent clean pass.
func TestPatternsLintReportsCouldNotCheckOnMissingRoot(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	rc := runPatterns([]string{"--lint", "--root", dir}, &stdout, &stderr)
	if rc != patternsExitCouldNot {
		t.Errorf("want exit %d (could-not-check), got %d\nstdout:\n%s", patternsExitCouldNot, rc, stdout.String())
	}
}

// TestPatternsNoVerbIsUsageRefusal proves `statusgen patterns` with no verb
// refuses rather than silently doing nothing.
func TestPatternsNoVerbIsUsageRefusal(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runPatterns(nil, &stdout, &stderr)
	if rc != patternsExitUsageError {
		t.Errorf("want exit %d (usage error), got %d", patternsExitUsageError, rc)
	}
	if stderr.Len() == 0 {
		t.Error("no verb given: want a refusal message on stderr, got none")
	}
}

func mustParseEmbeddedPatternSchema(t *testing.T) *schemaNode {
	t.Helper()
	schema, err := parseSchema(embeddedPatternSchemaBytes())
	if err != nil {
		t.Fatalf("parsing embedded pattern schema: %v", err)
	}
	return schema
}
