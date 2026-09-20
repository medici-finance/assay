package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// This file exercises the BUILT cellctl binary end to end (assay#1390's ask: "Go tests that
// exercise the BUILT binary"), the same package path and no extra ldflags the release workflow
// uses (.github/workflows/release.yml "Build and package desk-tools binaries": `go build … -o
// <out> ./cmd/cellctl`, run from the tools/desk module root) — so what these tests run against is
// what a release ships, not a separately-compiled copy of the logic policy_test.go already
// exercises in-process.
//
// Every fixture cell is a "house" kind: DRY_RUN=1 never fetches, never opens a real worktree and
// never contacts a forge or a model endpoint, so no live infrastructure is touched (C3). A stub
// `claude` on PATH answers only `--version`, which is as far as a dry run's policy preflight
// reaches into the harness.

var (
	buildOnce   sync.Once
	builtBinary string
	buildErr    error
)

func cellctlBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			buildErr = err
			return
		}
		moduleRoot := filepath.Join(wd, "..", "..")
		tmp, err := os.MkdirTemp("", "cellctl-bin-")
		if err != nil {
			buildErr = err
			return
		}
		out := filepath.Join(tmp, "cellctl")
		cmd := exec.Command("go", "build", "-o", out, "./cmd/cellctl")
		cmd.Dir = moduleRoot
		if b, err := cmd.CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("go build ./cmd/cellctl (from %s): %v\n%s", moduleRoot, err, b)
			return
		}
		builtBinary = out
	})
	if buildErr != nil {
		t.Fatalf("building the cellctl binary under test: %v", buildErr)
	}
	return builtBinary
}

// policyFixture is one house cell, complete enough for `set`/`show`/`desk --model … DRY_RUN=1`
// but never for a real (non-dry-run) launch — that needs a real git checkout, worktree creation
// and desk-verb shims this suite does not stand up (see the PR body's "not covered" section).
type policyFixture struct {
	cellsRoot string
	cellDir   string
	cfgDir    string
	binDir    string
	repoDir   string
	policy    string // absolute path to the cell's own policy.json
}

func newPolicyFixture(t *testing.T, claudeVersion string) *policyFixture {
	t.Helper()
	root := t.TempDir()
	f := &policyFixture{
		cellsRoot: filepath.Join(root, "cells"),
		cfgDir:    filepath.Join(root, "claude-config"),
		binDir:    filepath.Join(root, "bin"),
		repoDir:   filepath.Join(root, "repo"),
	}
	f.cellDir = filepath.Join(f.cellsRoot, "example")
	for _, d := range []string{f.cellDir, filepath.Join(f.cellDir, "home", ".config", "assay"), f.cfgDir, f.binDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.cellDir, "home", ".config", "assay", "roster.env"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(examplePolicyPath)
	if err != nil {
		t.Skipf("example policy not readable from this checkout (%v)", err)
	}
	f.policy = filepath.Join(f.cellDir, "model-policy.json")
	if err := os.WriteFile(f.policy, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	env := "CELL=example\nCELL_KIND=house\n" +
		"CELL_ROOTS=example-org/example-repo=" + f.repoDir + "\n" +
		"CELL_REPO=" + f.repoDir + "\n" +
		"CELL_MODEL_POLICY=model-policy.json\n"
	if err := os.WriteFile(filepath.Join(f.cellDir, "cell.env"), []byte(env), 0o644); err != nil {
		t.Fatal(err)
	}

	if claudeVersion != "" {
		script := "#!/bin/sh\ncase \"$1\" in\n--version) echo '" + claudeVersion + " (stub)'; exit 0;;\nesac\necho \"[stub claude] $*\"\nexit 0\n"
		if err := os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// rewritePolicy overlays a fresh policy body (built the same way policy_test.go's
// writeMutatedPolicy does) onto this fixture's cell — used by the negative-path tests that need
// one specific schema violation.
func (f *policyFixture) rewritePolicy(t *testing.T, mutate func(map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile(examplePolicyPath)
	if err != nil {
		t.Skipf("example policy not readable from this checkout (%v)", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	mutate(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.policy, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

type runResult struct {
	stdout, stderr string
	code           int
}

func (f *policyFixture) run(t *testing.T, extraEnv []string, args ...string) runResult {
	t.Helper()
	cmd := exec.Command(cellctlBinary(t), args...)
	base := []string{
		"CELLS_ROOT=" + f.cellsRoot,
		"CLAUDE_CONFIG_DIR=" + f.cfgDir,
		"PATH=" + f.binDir + ":" + os.Getenv("PATH"),
		"HOME=" + f.cellDir,
		"KUBECONFIG=/dev/null",
		// glm/kimi credential presets still resolve through the EXISTING provider machinery
		// under a policy (policy.go's header comment) — a fixture routing worker-desk to glm
		// needs its token env present, exactly like model-policy.test.py's fixture does.
		"ZAI_API_KEY=fixture-zai",
		"KIMI_API_KEY=fixture-kimi",
	}
	cmd.Env = append(base, extraEnv...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running cellctl %v: %v", args, err)
		}
	}
	return runResult{out.String(), errb.String(), code}
}

func (f *policyFixture) dryRunDesk(t *testing.T, role string, args ...string) runResult {
	t.Helper()
	return f.run(t, []string{"DRY_RUN=1"}, append([]string{"desk", "example", role}, args...)...)
}

// TestBinarySetAcceptsModelPolicyKey is the direct fix for the issue's own repro:
// `cellctl set <cell> CELL_MODEL_POLICY=<file>` used to refuse with "not a known cell.env key".
func TestBinarySetAcceptsModelPolicyKey(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	// Start from a cell.env that does NOT yet carry the key, exactly the issue's repro shape.
	if err := os.WriteFile(filepath.Join(f.cellDir, "cell.env"),
		[]byte("CELL=example\nCELL_KIND=house\nCELL_ROOTS=example-org/example-repo="+f.repoDir+"\nCELL_REPO="+f.repoDir+"\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	r := f.run(t, nil, "set", "example", "CELL_MODEL_POLICY="+f.policy)
	if r.code != 0 {
		t.Fatalf("set CELL_MODEL_POLICY should be accepted without --force, got exit %d: %s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "CELL_MODEL_POLICY") {
		t.Errorf("set output does not confirm the key: %s", r.stdout)
	}
}

// TestBinarySetShowRoundTrip: `set` persists the key, `show` displays it plus every role's
// resolved provider/model/effort/sha256 — the "set/show round-trip" the assignment asks for.
func TestBinarySetShowRoundTrip(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	if r := f.run(t, nil, "set", "example", "CELL_MODEL_POLICY="+f.policy); r.code != 0 {
		t.Fatalf("set: %s", r.stderr)
	}
	r := f.run(t, nil, "show", "example")
	if r.code != 0 {
		t.Fatalf("show: exit %d: %s", r.code, r.stderr)
	}
	if !strings.Contains(r.stdout, "[show] policy="+f.policy) {
		t.Errorf("show does not print the policy path: %s", r.stdout)
	}
	want := []string{
		"model pr-review-desk=claude-opus-4-8[1m] provider=anthropic harness=claude effort=high",
		"model worker-desk=glm-5.3-flash[1m] provider=glm harness=claude effort=high",
		"model intake-desk=gpt-5.6-terra provider=codex harness=codex effort=medium",
	}
	for _, w := range want {
		if !strings.Contains(r.stdout, w) {
			t.Errorf("show output missing %q\nfull output:\n%s", w, r.stdout)
		}
	}
}

// TestBinaryDryRunResolvesEveryRole ports test_desk_show_and_up_agree_on_mixed_providers' table
// (the `up` half is not ported — see the PR body) against `DRY_RUN=1 cellctl desk`.
func TestBinaryDryRunResolvesEveryRole(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	cases := []struct{ role, model, provider, harness, effort string }{
		{"the-desk", "claude-fable-5-1", "anthropic", "claude", "high"},
		{"pr-review-desk", "claude-opus-4-8[1m]", "anthropic", "claude", "high"},
		{"verify-desk", "claude-opus-4-8[1m]", "anthropic", "claude", "high"},
		{"worker-desk", "glm-5.3-flash[1m]", "glm", "claude", "high"},
		{"intake-desk", "gpt-5.6-terra", "codex", "codex", "medium"},
	}
	for _, c := range cases {
		r := f.dryRunDesk(t, c.role)
		if r.code != 0 {
			t.Errorf("%s: dry-run desk refused: %s%s", c.role, r.stdout, r.stderr)
			continue
		}
		for _, want := range []string{
			"role=" + c.role, "model=" + c.model, "provider=" + c.provider, "harness=" + c.harness, "effort=" + c.effort,
		} {
			if !strings.Contains(r.stdout, want) {
				t.Errorf("%s: dry-run output missing %q\n%s", c.role, want, r.stdout)
			}
		}
		if !strings.Contains(r.stdout, "policy:"+f.policy+"@") {
			t.Errorf("%s: dry-run output does not carry the policy file's sha256: %s", c.role, r.stdout)
		}
	}
}

// TestBinaryOpusFiveDeniedOnOverride is the "deny list refuses a matching model instead of a
// silent fallback" requirement: an explicit --model claude-opus-5 must refuse the whole launch
// with a non-zero exit, never fall back to the role's own tier.
func TestBinaryOpusFiveDeniedOnOverride(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	r := f.dryRunDesk(t, "pr-review-desk", "--model", "claude-opus-5")
	if r.code == 0 {
		t.Fatalf("an Opus-5 override must refuse, got exit 0: %s", r.stdout)
	}
	if !strings.Contains(r.stderr, "model-policy") {
		t.Errorf("refusal should name the model-policy layer: %s", r.stderr)
	}
}

// TestBinaryOpusFiveDeniedEvenWithEmptyDenyList: the ban survives an operator clearing `deny`.
func TestBinaryOpusFiveDeniedEvenWithEmptyDenyList(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.rewritePolicy(t, func(m map[string]any) {
		m["deny"] = []any{}
		m["providers"].(map[string]any)["anthropic"].(map[string]any)["tiers"].(map[string]any)["strong"].(map[string]any)["model"] = "claude-opus-5"
	})
	r := f.dryRunDesk(t, "pr-review-desk")
	if r.code == 0 {
		t.Fatalf("Opus-5 must be refused even with deny=[], got exit 0: %s", r.stdout)
	}
}

func TestBinaryNegativeMissingPolicyFile(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	if err := os.Remove(f.policy); err != nil {
		t.Fatal(err)
	}
	r := f.run(t, nil, "show", "example")
	if r.code == 0 {
		t.Fatalf("show against a missing policy file must refuse, got exit 0: %s", r.stdout)
	}
}

func TestBinaryNegativeUnknownRoleInPolicy(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.rewritePolicy(t, func(m map[string]any) {
		m["roles"].(map[string]any)["typo-desk"] = map[string]any{"provider": "anthropic", "tier": "mid"}
	})
	r := f.run(t, nil, "show", "example")
	if r.code == 0 {
		t.Fatalf("a policy naming an unknown role must refuse, got exit 0: %s", r.stdout)
	}
}

func TestBinaryNegativeUnknownTopLevelKey(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.rewritePolicy(t, func(m map[string]any) { m["unknown_field"] = true })
	r := f.run(t, nil, "show", "example")
	if r.code == 0 {
		t.Fatalf("an unknown top-level policy key must refuse, got exit 0: %s", r.stdout)
	}
}

func TestBinaryNegativeEffortOmitted(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	f.rewritePolicy(t, func(m map[string]any) {
		tier := m["providers"].(map[string]any)["anthropic"].(map[string]any)["tiers"].(map[string]any)["strong"].(map[string]any)
		delete(tier, "effort")
	})
	r := f.dryRunDesk(t, "pr-review-desk")
	if r.code == 0 {
		t.Fatalf("a tier with no effort field must refuse, got exit 0: %s", r.stdout)
	}
}

// TestBinaryNegativeHarnessBelowMinimum ports test_old_harness_refuses_before_launch: a stubbed
// Claude Code below docs/cellctl-model-policy.md's 2.1.251 floor refuses BEFORE any launch,
// dry-run included.
func TestBinaryNegativeHarnessBelowMinimum(t *testing.T) {
	f := newPolicyFixture(t, "2.1.200")
	r := f.dryRunDesk(t, "pr-review-desk")
	if r.code == 0 {
		t.Fatalf("a below-minimum Claude Code must refuse, got exit 0: %s", r.stdout)
	}
	if !strings.Contains(r.stderr, "2.1.251") {
		t.Errorf("refusal should name the version floor: %s", r.stderr)
	}
}

// TestBinaryCodexRoleSkipsClaudeVersionCheck: intake-desk resolves to the codex harness, so no
// claude binary needs to exist at all — the version floor is Claude-Code-specific.
func TestBinaryCodexRoleSkipsClaudeVersionCheck(t *testing.T) {
	f := newPolicyFixture(t, "") // no claude stub written at all
	r := f.dryRunDesk(t, "intake-desk")
	if r.code != 0 {
		t.Fatalf("a codex-harness role must not need a claude binary: %s%s", r.stdout, r.stderr)
	}
}

// TestBinarySetWithPolicyIsAmbiguous: --set combined with an active policy refuses rather than
// persisting a pin the policy would ignore on the next boot.
func TestBinarySetWithPolicyIsAmbiguous(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	r := f.run(t, []string{"DRY_RUN=1"}, "desk", "example", "pr-review-desk", "--model", "opus", "--set")
	if r.code == 0 {
		t.Fatalf("--set with a model policy active must refuse, got exit 0: %s", r.stdout)
	}
}

// TestBinaryContainerKindRefusesPolicy: a container/scrubbed cell cannot apply a policy at all
// yet (docs/cellctl-model-policy.md "Harness adapters and child agents").
func TestBinaryContainerKindRefusesPolicy(t *testing.T) {
	f := newPolicyFixture(t, "2.1.278")
	launcher := filepath.Join(f.binDir, "fake-launcher")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := []string{"DRY_RUN=1", "CELL_CONTAINER_LAUNCHER=" + launcher}
	r := f.run(t, env, "desk", "example", "the-desk", "--kind", "container")
	if r.code == 0 {
		t.Fatalf("a container cell must refuse an active model policy, got exit 0: %s", r.stdout)
	}
	if !strings.Contains(r.stderr, "house or k8s cell") {
		t.Errorf("refusal should name the house/k8s requirement: %s", r.stderr)
	}
}
