package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// patterns_test.go — graph-execution/02. Covers the schema-parity gate, the two
// shipped patterns validating clean, and the five MUST-rule fixtures each
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
	for _, name := range []string{"good-implementation-v1.yaml", "good-research-v1.yaml", "good-signal-example.yaml"} {
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

// TestPatternsEffectTargetNotOwnedIsProblem — the `bad-effect-target-not-owned.yaml`
// fixture: a non-`effect`-kind node declares an effect whose `target` is not
// among its own `outputs`. The lint MUST exit 1, naming
// pattern-effect-target-not-owned — and only that rule, never
// pattern-effect-exceeds-role (the fixture's effect kind IS permitted for its
// role; only the target-ownership check is meant to fire).
func TestPatternsEffectTargetNotOwnedIsProblem(t *testing.T) {
	schema := mustParseEmbeddedPatternSchema(t)
	path := filepath.Join("testdata", "patterns", "bad-effect-target-not-owned.yaml")
	state, violations := lintPatternFile(path, schema)
	if state != patternStateFailed {
		t.Fatalf("want checked-failed, got state=%d violations=%v", state, violations)
	}
	if !anyContains(violations, rulePatternEffectTargetUnowned) {
		t.Errorf("violations do not name %q:\n%s", rulePatternEffectTargetUnowned, strings.Join(violations, "\n"))
	}
	if anyContains(violations, rulePatternEffectExceedsRole) {
		t.Errorf("fixture unexpectedly also tripped %q — it should trip only %q:\n%s",
			rulePatternEffectExceedsRole, rulePatternEffectTargetUnowned, strings.Join(violations, "\n"))
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

// regression: #1933
func TestPatternsHumanFloorOutput(t *testing.T) {
	for _, name := range []string{"bad-merge-behind-dispatch.yaml", "bad-floor-effect-target.yaml"} {
		t.Run(name, func(t *testing.T) {
			state, fs := lintPatternFile(filepath.Join("testdata", "patterns", name), mustParseEmbeddedPatternSchema(t))
			if state != patternStateFailed || !anyContains(fs, "pattern-human-floor-output") {
				t.Fatalf("want human-floor refusal, got state=%d findings=%v", state, fs)
			}
			if anyContains(fs, rulePatternEffectExceedsRole) {
				t.Fatalf("wrong permission-table refusal: %v", fs)
			}
		})
	}
	// A second site: all three floor artifacts, on an unrelated node, with
	// output-only and effect-target-only faults. The latter owns no floor output.
	for _, artifact := range []string{"merge-commit", "release", "deployment"} {
		for _, targetOnly := range []bool{false, true} {
			n := patternNode{ID: "second-site", Kind: "effect", Role: "worker"}
			if targetOnly {
				n.Outputs = []string{"request"}
				n.Effects = []patternEffect{{Kind: "comment", Target: artifact}}
			} else {
				n.Outputs = []string{artifact}
			}
			p := patternDoc{Nodes: []patternNode{n}}
			if !anyContains(checkPatternMustRules("second-site.yaml", p), "pattern-human-floor-output") {
				t.Errorf("accepted second site %s targetOnly=%v", artifact, targetOnly)
			}
		}
	}
}

// regression: #1933
func TestPatternsHumanRoleDecision(t *testing.T) {
	state, fs := lintPatternFile(filepath.Join("testdata", "patterns", "bad-human-role-not-decision.yaml"), mustParseEmbeddedPatternSchema(t))
	if state != patternStateFailed || !anyContains(fs, "pattern-human-role-not-decision") {
		t.Errorf("missing human role refusal: %v", fs)
	}
	// Call the lint directly: its refusal must survive a loosened schema.
	for _, n := range []patternNode{
		{ID: "human-effect", Role: "human", Kind: "effect"},
		{ID: "human-decision", Role: "human", Kind: "decision", Effects: []patternEffect{{Kind: "comment", Target: "note"}}},
	} {
		if !anyContains(checkPatternMustRules("shape.yaml", patternDoc{Nodes: []patternNode{n}}), "pattern-human-role-not-decision") {
			t.Errorf("accepted illegal human node: %+v", n)
		}
	}
}

// regression: #1933
func TestHumanSchemaConditions(t *testing.T) {
	schema, err := parseSchema([]byte(`{"if":{"properties":{"role":{"const":"human"}},"required":["role"]},"then":{"properties":{"kind":{"const":"decision"},"effects":{"maxItems":0}},"not":{"required":["actor"]}},"else":{"required":["actor"]}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		node map[string]any
		fail bool
	}{
		{"human", map[string]any{"role": "human", "kind": "decision", "effects": []any{}}, false},
		{"worker", map[string]any{"role": "worker", "actor": map[string]any{}}, false},
		{"kind", map[string]any{"role": "human", "kind": "effect"}, true},
		{"effects", map[string]any{"role": "human", "kind": "decision", "effects": []any{"dispatch"}}, true},
		{"actor", map[string]any{"role": "human", "kind": "decision", "actor": map[string]any{}}, true},
		{"missing", map[string]any{"role": "worker"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := validateValue(schema, tc.node, "")
			if (len(fs) > 0) != tc.fail {
				t.Fatalf("findings=%v want fail=%v", fs, tc.fail)
			}
		})
	}
}

// Signal-pattern rules (graph-execution/34, spec/workflow-pattern-v1.md §4.3).
// Each test feeds its named bad fixtures and asserts the rule tag fires, and
// that the good signal fixture does not report that tag.

// assertSignalRule lints each fixture and requires it to fail naming rule, and
// requires good-signal-example.yaml not to name it.
func assertSignalRule(t *testing.T, rule string, fixtures ...string) {
	t.Helper()
	schema := mustParseEmbeddedPatternSchema(t)
	tag := "[" + rule + "]"
	for _, name := range fixtures {
		path := filepath.Join("testdata", "patterns", name+".yaml")
		state, violations := lintPatternFile(path, schema)
		if state != patternStateFailed {
			t.Errorf("%s: want checked-failed, got state=%d violations=%v", name, state, violations)
			continue
		}
		if !anyContains(violations, tag) {
			t.Errorf("%s: violations do not name %s:\n%s", name, tag, strings.Join(violations, "\n"))
		}
	}
	good := filepath.Join("testdata", "patterns", "good-signal-example.yaml")
	if _, violations := lintPatternFile(good, schema); anyContains(violations, tag) {
		t.Errorf("good-signal-example.yaml names %s:\n%s", tag, strings.Join(violations, "\n"))
	}
}

// lintSignalDoc runs the MUST rules over an inline YAML document.
func lintSignalDoc(t *testing.T, doc string) []string {
	t.Helper()
	var data map[string]any
	if err := yaml.Unmarshal([]byte(doc), &data); err != nil {
		t.Fatalf("inline doc: %v", err)
	}
	pat, err := parsePatternDoc(data)
	if err != nil {
		t.Fatalf("parse inline doc: %v", err)
	}
	return checkPatternMustRules("inline.yaml", pat)
}

// regression: #4506
func TestPatternsSignalInput(t *testing.T) {
	assertSignalRule(t, rulePatternSignalInput,
		"bad-signal-input-not-entry", "bad-mechanical-no-culprit", "bad-exit-unreachable",
		"bad-class-unlisted", "bad-disconnected-effect")

	// The schema no longer requires `artifact`, so an input item naming neither
	// key (or both) is the lint's to refuse.
	good, err := os.ReadFile(filepath.Join("testdata", "patterns", "good-signal-example.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for label, item := range map[string]string{
		"neither": "inputs: [{revision: brief.version}]",
		"both":    `inputs: [{artifact: signal-record, signal: {source: "example-org/example-app#ci-main", liveness-check: "x", classes: [example-alert, example-gate-red]}}]`,
	} {
		doc := strings.Replace(string(good), "inputs: [{artifact: signal-record}]", item, 1)
		if doc == string(good) {
			t.Fatalf("%s: anchor not found in the good fixture", label)
		}
		if v := lintSignalDoc(t, doc); !anyContains(v, "["+rulePatternSignalInput+"]") {
			t.Errorf("%s: input item naming %s of artifact/signal not refused: %v", label, label, v)
		}
	}
}

// regression: #4506
func TestPatternsReplayMandatory(t *testing.T) {
	assertSignalRule(t, rulePatternReplayMandatory, "bad-replay-not-mandatory")
}

// regression: #4506
func TestPatternsSignalAdmission(t *testing.T) {
	assertSignalRule(t, rulePatternAdmissionOpen,
		"bad-admission-pull-request", "bad-exit-effect-no-admission", "bad-judgement-class-no-admission",
		"bad-artifact-effect-no-admission", "bad-disconnected-effect")
}

// regression: #4506
func TestPatternsRemedySetClosed(t *testing.T) {
	assertSignalRule(t, rulePatternRemedySetOpen,
		"bad-remedy-open", "bad-refused-paths-dropped", "bad-class-no-floor",
		"bad-artifact-effect-no-floor", "bad-disconnected-effect")
}

// regression: #4506
func TestPatternsJudgementDefault(t *testing.T) {
	assertSignalRule(t, rulePatternJudgementDefault, "bad-judgement-no-default")
}

// regression: #4506
func TestPatternsCulpritRecord(t *testing.T) {
	assertSignalRule(t, rulePatternCulpritRecord, "bad-withheld-mechanical")
}

// regression: #4506
func TestPatternsDecisionOptions(t *testing.T) {
	assertSignalRule(t, rulePatternDecisionOptions, "bad-options-on-effect")
}

// regression: #4506
func TestPatternsContainmentSet(t *testing.T) {
	assertSignalRule(t, rulePatternContainmentSet, "bad-containment-open", "bad-containment-default")
}
