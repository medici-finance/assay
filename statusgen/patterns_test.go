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

// ---- graph-execution/08: the signal-triggered pattern ----

// lintSignalFixture lints one file under testdata/patterns/signal-triggered/.
func lintSignalFixture(t *testing.T, name string) (patternState, []string) {
	t.Helper()
	return lintPatternFile(filepath.Join("testdata", "patterns", "signal-triggered", name), mustParseEmbeddedPatternSchema(t))
}

// requireSignalRed asserts a mutated copy fails, naming rule and, when given,
// the node. The valid copy is linted alongside as the control: a mutation is
// only evidence when its unmutated twin is clean.
func requireSignalRed(t *testing.T, name, rule, node string) []string {
	t.Helper()
	if state, fs := lintSignalFixture(t, "valid.yaml"); state != patternStateClean {
		t.Fatalf("control valid.yaml is not clean: state=%d %v", state, fs)
	}
	state, fs := lintSignalFixture(t, name)
	if state != patternStateFailed {
		t.Fatalf("%s: want checked-failed, got state=%d %v", name, state, fs)
	}
	for _, f := range fs {
		if strings.Contains(f, "["+rule+"]") && (node == "" || strings.Contains(f, "node \""+node+"\"")) {
			return fs
		}
	}
	t.Fatalf("%s: no finding names [%s] for node %q:\n%s", name, rule, node, strings.Join(fs, "\n"))
	return nil
}

// TestSignalTriggeredValidLintsClean — the valid instance, and the shipped file it
// copies, lint clean; the fixture is a byte copy plus a one-line header.
func TestSignalTriggeredValidLintsClean(t *testing.T) {
	shipped, err := os.ReadFile(filepath.Join("..", "spec", "workflow-patterns", "signal-triggered-v1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "patterns", "signal-triggered", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, body, _ := strings.Cut(string(fixture), "\n"); body != string(shipped) {
		t.Error("testdata/patterns/signal-triggered/valid.yaml has drifted from spec/workflow-patterns/signal-triggered-v1.yaml")
	}
	if state, fs := lintSignalFixture(t, "valid.yaml"); state != patternStateClean {
		t.Errorf("want clean, got state=%d %v", state, fs)
	}
}

// TestSignalTriggeredGuardEditIsRefused — Verify row 2, mutated copy (a).
func TestSignalTriggeredGuardEditIsRefused(t *testing.T) {
	requireSignalRed(t, "a-guard-config-edit.yaml", rulePatternEffectExceedsRole, "contain")

	// Containment is scoped to the signal node: the same steps on any other node,
	// or an unknown step on the signal node, are refused by the same rule.
	base := patternNode{ID: "contain", Kind: "effect", Role: "worker", Signal: &patternSignal{}, Containment: []string{"rollback"}}
	if fs := checkContainmentSteps("x.yaml", "contain", base); len(fs) != 0 {
		t.Fatalf("scoped containment refused: %v", fs)
	}
	for name, n := range map[string]patternNode{
		"no signal":    {ID: "n", Kind: "effect", Role: "worker", Containment: []string{"rollback"}},
		"reviewer":     {ID: "n", Kind: "effect", Role: "reviewer", Signal: &patternSignal{}, Containment: []string{"stop"}},
		"not effect":   {ID: "n", Kind: "check", Role: "worker", Signal: &patternSignal{}, Containment: []string{"stop"}},
		"unknown step": {ID: "n", Kind: "effect", Role: "worker", Signal: &patternSignal{}, Containment: []string{"guard-config-edit"}},
	} {
		if !anyContains(checkContainmentSteps("x.yaml", n.ID, n), rulePatternEffectExceedsRole) {
			t.Errorf("%s: containment accepted outside its scope: %+v", name, n)
		}
	}
	// A containment step written as an ordinary effect is not in any role's table.
	for role := range patternRoleEffectPermissions {
		for step := range patternContainmentSteps {
			if patternRoleEffectPermissions[role][step] {
				t.Errorf("role %s holds containment step %s as an ordinary effect", role, step)
			}
		}
	}
}

// TestSignalTriggeredFixtureMandatory — Verify row 3, mutated copy (b).
func TestSignalTriggeredFixtureMandatory(t *testing.T) {
	requireSignalRed(t, "b-fixture-not-mandatory.yaml", rulePatternFixtureOptional, "root-cause")
}

// TestSignalTriggeredNoMergeEffect — Verify row 4, mutated copy (c).
func TestSignalTriggeredNoMergeEffect(t *testing.T) {
	fs := requireSignalRed(t, "c-propose-merges.yaml", rulePatternEffectExceedsRole, "propose")
	if !anyContains(fs, rulePatternSchemaViolation) {
		t.Errorf("merge is not in the closed effect-kind enum, want a schema finding too:\n%s", strings.Join(fs, "\n"))
	}
}

// TestSignalTriggeredGateRedAdmission — Verify row 9, mutated copy (d), plus a
// push to a ref other than main.
func TestSignalTriggeredGateRedAdmission(t *testing.T) {
	requireSignalRed(t, "d-admits-pull-request.yaml", rulePatternGateRedAdmission, "")
	for _, a := range []patternAdmit{
		{Trigger: "push", Ref: "feature"}, {Trigger: "push"}, {Trigger: "schedule", Ref: "feature"},
		{Trigger: "pull_request_target"}, {Trigger: "workflow_dispatch", Ref: "main"},
	} {
		if !anyContains(checkGateRedAdmission("x.yaml", &patternSignal{Admission: []patternAdmit{a}}), rulePatternGateRedAdmission) {
			t.Errorf("admitted %+v", a)
		}
	}
	if !anyContains(checkGateRedAdmission("x.yaml", &patternSignal{}), rulePatternGateRedAdmission) {
		t.Error("an empty gate-red admission list was accepted")
	}
}

// TestSignalTriggeredRemedySetClosed — Verify row 10, mutated copy (e) (a
// non-canonical path that cleans to .github/workflows/ci.yml), plus a remedy
// outside the set, a case-folded gate path, an escaping path and a refused-paths
// list that drops the floor.
func TestSignalTriggeredRemedySetClosed(t *testing.T) {
	requireSignalRed(t, "e-remedy-edits-workflows.yaml", rulePatternRemedySetOpen, "propose")
	pat := patternDoc{NodeByID: map[string]patternNode{
		"triage": {ID: "triage", Kind: "decision", Role: "human", Default: "hold"},
	}}
	good := patternRemedies{Refused: []string{".github/", ".claude/"}, Otherwise: "triage"}
	for name, r := range map[string]patternRemedies{
		"kind":   {Set: []patternRemedy{{Kind: "workflow-edit"}}},
		"case":   {Set: []patternRemedy{{Kind: "gofmt", Path: ".GitHub/Workflows/ci.yml"}}},
		"escape": {Set: []patternRemedy{{Kind: "gofmt", Path: "../other/x.go"}}},
		"abs":    {Set: []patternRemedy{{Kind: "gofmt", Path: "/etc/x"}}},
		"floor":  {Set: []patternRemedy{{Kind: "gofmt"}}, Refused: []string{".github/workflows/"}},
		"route":  {Set: []patternRemedy{{Kind: "gofmt"}}, Otherwise: "nowhere"},
	} {
		if r.Refused == nil {
			r.Refused = good.Refused
		}
		if r.Otherwise == "" {
			r.Otherwise = good.Otherwise
		}
		n := patternNode{ID: "propose", Remedies: &r}
		fs := checkRemedies("x.yaml", n, pat)
		if !anyContains(fs, rulePatternRemedySetOpen) && !anyContains(fs, rulePatternJudgementDefault) {
			t.Errorf("%s: accepted %+v", name, r)
		}
	}
	ok := good
	ok.Set = []patternRemedy{{Kind: "gofmt", Path: "statusgen/x.go"}}
	if fs := checkRemedies("x.yaml", patternNode{ID: "propose", Remedies: &ok}, pat); len(fs) != 0 {
		t.Errorf("an in-set remedy on an ordinary path was refused: %v", fs)
	}
}

// TestSignalTriggeredJudgementCarriesDefault — Verify row 11, mutated copy (f).
func TestSignalTriggeredJudgementCarriesDefault(t *testing.T) {
	requireSignalRed(t, "f-judgement-no-default.yaml", rulePatternJudgementDefault, "")
	pat := patternDoc{NodeByID: map[string]patternNode{
		"blank": {ID: "blank", Kind: "decision", Role: "human", Default: "  "},
		"desk":  {ID: "desk", Kind: "decision", Role: "desk", Default: "hold"},
	}}
	for _, target := range []string{"blank", "desk", "absent"} {
		if !anyContains(checkJudgementNode("x.yaml", "route", target, pat), rulePatternJudgementDefault) {
			t.Errorf("judgement node %q accepted", target)
		}
	}
}

// TestSignalTriggeredWithheldNeverMechanical — Verify row 12, mutated copy (g),
// plus a withheld gate given a reconstructed file.
func TestSignalTriggeredWithheldNeverMechanical(t *testing.T) {
	requireSignalRed(t, "g-withheld-mechanical.yaml", rulePatternWithheldClass, "confirm")
	raw, err := os.ReadFile(filepath.Join("testdata", "patterns", "signal-triggered", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(raw), "{gate: leak-sweep, class: judgement, file: null}", "{gate: leak-sweep, class: judgement, file: main.go}", 1)
	if mutated == string(raw) {
		t.Fatal("valid.yaml no longer carries the withheld entry this test mutates")
	}
	p := filepath.Join(t.TempDir(), "withheld-file.yaml")
	if err := os.WriteFile(p, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, fs := lintPatternFile(p, mustParseEmbeddedPatternSchema(t)); !anyContains(fs, rulePatternWithheldClass) {
		t.Errorf("a withheld gate with a reconstructed file was accepted: %v", fs)
	}
}

// TestSignalTriggeredEnumsMatchLint binds the schema's closed sets to the lint's
// own copies: the effect-kind enum is the union of §7's role table, and the
// containment enum is the containment taxonomy.
func TestSignalTriggeredEnumsMatchLint(t *testing.T) {
	schema := mustParseEmbeddedPatternSchema(t)
	node := schema.raw["properties"].(map[string]any)["nodes"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	enum := func(m map[string]any) []string {
		var out []string
		for _, v := range m["enum"].([]any) {
			out = append(out, v.(string))
		}
		sort.Strings(out)
		return out
	}
	effKind := node["effects"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["kind"].(map[string]any)
	steps := node["containment"].(map[string]any)["properties"].(map[string]any)["steps"].(map[string]any)["items"].(map[string]any)
	union := map[string]bool{}
	for _, kinds := range patternRoleEffectPermissions {
		for k := range kinds {
			union[k] = true
		}
	}
	keys := func(m map[string]bool) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	if got, want := strings.Join(enum(effKind), ","), strings.Join(keys(union), ","); got != want {
		t.Errorf("effects[].kind enum %s != role-table union %s", got, want)
	}
	if got, want := strings.Join(enum(steps), ","), strings.Join(keys(patternContainmentSteps), ","); got != want {
		t.Errorf("containment.steps enum %s != patternContainmentSteps %s", got, want)
	}
}
