package deskkit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func fixtureAggregateScript(lines []string) string {
	var script []string
	inRun := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "run: |" {
			inRun = true
			continue
		}
		if inRun {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "          ") {
				break
			}
			script = append(script, strings.TrimPrefix(line, "          "))
		}
	}
	return strings.Join(script, "\n")
}

func fixtureAggregateFaults(t *testing.T, raw string) []string {
	t.Helper()
	var faults []string
	job, found := ciWorkflowJobLines(t, raw, ciWorkflow, "build-test")
	if !found {
		return []string{"required build-test aggregate absent"}
	}
	body := strings.Join(job, "\n")
	if !strings.Contains(body, "needs: [build-test-shard, forge-local-authority, fixture-environment]") {
		faults = append(faults, "fixture job is not a required aggregate dependency")
	}
	if !strings.Contains(body, "if: ${{ always() }}") {
		faults = append(faults, "aggregate does not run after failed/skipped dependencies")
	}
	if !strings.Contains(body, "FIXTURES: ${{ needs.fixture-environment.result }}") {
		faults = append(faults, "aggregate does not consume actual fixture job result")
	}
	lane, found := ciWorkflowJobLines(t, raw, ciWorkflow, "fixture-environment")
	if !found {
		return append(faults, "fixture lane absent")
	}
	laneText := strings.Join(lane, "\n")
	if strings.Contains(laneText, "if:") || strings.Contains(laneText, "continue-on-error:") || strings.Contains(laneText, "needs: scope") {
		faults = append(faults, "fixture lane can omit or ignore required execution")
	}
	for _, required := range []string{"bash tools/test-fixture-environment.sh", "bash tools/test-fixture-environment_test.sh", "go list -mod=readonly -deps -test ./...", "statusgen/fetch-baseline.sh"} {
		if !strings.Contains(laneText, required) {
			faults = append(faults, "fixture lane missing "+required)
		}
	}
	return faults
}

func TestFixtureEnvironmentCIAggregate(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("../../../..", ciWorkflow))
	if err != nil {
		t.Fatal(err)
	}
	raw := string(content)
	if faults := fixtureAggregateFaults(t, raw); len(faults) != 0 {
		t.Fatal(strings.Join(faults, "; "))
	}
	for name, mutant := range map[string]string{
		"missing lane":       strings.Replace(raw, "  fixture-environment:\n", "  removed-fixture-lane:\n", 1),
		"missing dependency": strings.Replace(raw, "needs: [build-test-shard, forge-local-authority, fixture-environment]", "needs: [build-test-shard, forge-local-authority]", 1),
		"ignored result":     strings.Replace(raw, "FIXTURES: ${{ needs.fixture-environment.result }}", "FIXTURES: success", 1),
		"optional lane":      strings.Replace(raw, "  fixture-environment:\n", "  fixture-environment:\n    if: false\n", 1),
	} {
		t.Run(name, func(t *testing.T) {
			if faults := fixtureAggregateFaults(t, mutant); len(faults) == 0 {
				t.Fatal("CI wiring mutation was accepted")
			}
		})
	}
	job, _ := ciWorkflowJobLines(t, raw, ciWorkflow, "build-test")
	script := fixtureAggregateScript(job)
	if script == "" {
		t.Fatal("aggregate script absent")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Log("shell execution unavailable; structural assertions above still ran")
		return
	}
	run := func(script, state string) bool {
		cmd := exec.Command(bash, "-c", script)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "SHARDS=success", "AUTHORITY=success", "FIXTURES=" + state}
		return cmd.Run() == nil
	}
	if !run(script, "success") {
		t.Fatal("healthy required lane cannot pass")
	}
	for _, state := range []string{"failure", "skipped", "cancelled", ""} {
		if run(script, state) {
			t.Fatalf("aggregate accepted fixture conclusion %q", state)
		}
	}
	mutant := strings.Replace(script, `[ "$FIXTURES" = "success" ]`, `true`, 1)
	if mutant == script || !run(mutant, "failure") {
		t.Fatal("failure-state positive control did not expose the removed assertion")
	}
}

func TestFixtureEnvironmentCIRunner(t *testing.T) {
	root := filepath.Clean("../../../..")
	runner, err := os.ReadFile(filepath.Join(root, "tools/test-fixture-environment.sh"))
	if err != nil {
		t.Fatal(err)
	}
	// Couple the launcher's injected Git profile to the actual admission owner,
	// rather than perpetuating an older issue report's marker values. The
	// caller global config is deliberately stronger hostile input: it names a
	// disposable victim instead of pretending that caller configuration is safe.
	owner, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "statusgen/verifieradmission.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	profileValues := 0
	for _, decl := range owner.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "admittedRowEnv" {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			lit, ok := node.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err == nil && strings.HasPrefix(value, "GIT_") && strings.Contains(value, "=") {
				profileValues++
				if !strings.Contains(string(runner), strconv.Quote(value)) {
					t.Errorf("hostile launcher drifted from admitted Git profile: %s", value)
				}
			}
			return true
		})
	}
	if profileValues == 0 {
		t.Fatal("actual admitted Git profile was not inspected")
	}
	for _, required := range []string{"fixture_run_module statusgen", "fixture_run_module tools/desk", "test -json -count=1 -timeout=20m ./...", "GIT_CONFIG_COUNT=2", "GIT_NO_REPLACE_OBJECTS=1", "CI=true", `"PATH=$fixture_scratch/bin:/usr/bin:/bin:/usr/sbin:/sbin"`, `env -i "${fixture_env[@]}"`, "fixture_unchanged"} {
		if !strings.Contains(string(runner), required) {
			t.Errorf("full-suite launcher missing %q", required)
		}
	}
	policy, err := os.ReadFile(filepath.Join(root, "docs/test-policy.md"))
	if err != nil || !strings.Contains(string(policy), "## Fixture caller environments") {
		t.Fatal("fixture execution policy absent", err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Log("shell controls unavailable on this platform")
		return
	}
	cmd := exec.Command(bash, "tools/test-fixture-environment_test.sh")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("launcher controls: %v: %s", err, out)
	}
	for _, control := range []string{"healthy", "git-global", "git-repo", "installed-tool", "suite-failure", "absent-execution", "explicit credential presence and absence", "physical owned scratch paths", "explicit compatible interpreters and CI tool strictness", "public entry executes both complete modules after failure"} {
		if !strings.Contains(string(out), fmt.Sprintf("PASS: %s", control)) {
			t.Errorf("missing actual launcher control %s", control)
		}
	}
}
