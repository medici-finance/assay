package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// examplePolicyPath is the same fixture tools/cellctl/tests/model-policy.test.py loads
// (EXAMPLE = CELLCTL.parent / 'examples/model-policy.json'), reached the same way
// usage_test.go reaches the shell oracle: relative to this package's directory.
const examplePolicyPath = "../../../cellctl/examples/model-policy.json"

func readExamplePolicy(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(examplePolicyPath)
	if err != nil {
		t.Skipf("example policy not readable from this checkout (%v)", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("example policy is not valid JSON: %v", err)
	}
	return m
}

// writeMutatedPolicy re-marshals a mutated copy of the example policy into a fresh temp file and
// returns its path — the pattern every negative test below uses to reach one specific refusal
// without hand-writing a whole policy body.
func writeMutatedPolicy(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	m := readExamplePolicy(t)
	if mutate != nil {
		mutate(m)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("re-marshal mutated policy: %v", err)
	}
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func loadExamplePolicy(t *testing.T) *ModelPolicy {
	t.Helper()
	readExamplePolicy(t) // triggers the skip when the checkout does not carry the fixture
	m, err := loadModelPolicy(examplePolicyAbsPath(t))
	if err != nil {
		t.Fatalf("loading the example policy should succeed: %v", err)
	}
	return m
}

func examplePolicyAbsPath(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs(examplePolicyPath)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestLoadModelPolicyExampleValid(t *testing.T) {
	m := loadExamplePolicy(t)
	if m.Schema != 1 {
		t.Errorf("schema = %d, want 1", m.Schema)
	}
	if len(m.Providers) != 4 {
		t.Errorf("providers = %d, want 4 (anthropic, glm, kimi, codex)", len(m.Providers))
	}
	if len(m.Roles) != 5 {
		t.Errorf("roles = %d, want 5", len(m.Roles))
	}
	if len(m.SHA256) != 64 {
		t.Errorf("sha256 = %q, want a 64-hex-char digest", m.SHA256)
	}
}

func TestLoadModelPolicyMissingFileRefuses(t *testing.T) {
	_, err := loadModelPolicy(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil {
		t.Fatal("a missing policy file must refuse, not silently pass")
	}
}

func TestLoadModelPolicyRejectsUnknownTopLevelField(t *testing.T) {
	path := writeMutatedPolicy(t, func(m map[string]any) { m["extra"] = true })
	if _, err := loadModelPolicy(path); err == nil {
		t.Fatal("an unknown top-level policy field must refuse")
	}
}

func TestLoadModelPolicyRejectsMissingEffortField(t *testing.T) {
	path := writeMutatedPolicy(t, func(m map[string]any) {
		tier := m["providers"].(map[string]any)["anthropic"].(map[string]any)["tiers"].(map[string]any)["strong"].(map[string]any)
		delete(tier, "effort")
	})
	if _, err := loadModelPolicy(path); err == nil {
		t.Fatal("a tier missing its required effort field must refuse")
	}
}

// TestLoadModelPolicyRejectsExtraRoleField ports model-policy.test.py's
// test_unknown_effort_field_refuses: a role assignment naming provider+tier+effort (effort does
// not belong on a role — it belongs on the tier) is an unknown-key refusal, not a silent ignore.
func TestLoadModelPolicyRejectsExtraRoleField(t *testing.T) {
	path := writeMutatedPolicy(t, func(m map[string]any) {
		m["roles"].(map[string]any)["pr-review-desk"].(map[string]any)["effort"] = "max"
	})
	if _, err := loadModelPolicy(path); err == nil {
		t.Fatal("a role assignment with an extra field must refuse")
	}
}

func TestLoadModelPolicyRejectsUnknownRoleName(t *testing.T) {
	path := writeMutatedPolicy(t, func(m map[string]any) {
		roles := m["roles"].(map[string]any)
		roles["typo-desk"] = map[string]any{"provider": "anthropic", "tier": "mid"}
	})
	if _, err := loadModelPolicy(path); err == nil {
		t.Fatal("a role name outside the fixed five must refuse")
	}
}

// TestLoadModelPolicyDenyCannotBeRemoved ports test_ban_cannot_be_removed_by_omitting_deny: the
// Opus-5 prohibition applies even when `deny` is emptied out.
func TestLoadModelPolicyDenyCannotBeRemoved(t *testing.T) {
	path := writeMutatedPolicy(t, func(m map[string]any) {
		m["deny"] = []any{}
		m["providers"].(map[string]any)["anthropic"].(map[string]any)["tiers"].(map[string]any)["strong"].(map[string]any)["model"] = "claude-opus-5"
	})
	if _, err := loadModelPolicy(path); err == nil {
		t.Fatal("an Opus-5 tier target must refuse even with an empty deny list")
	}
}

// TestLoadModelPolicyAllowsOpusFiveFiveTierTarget is the positive twin of DenyCannotBeRemoved: the
// built-in Opus-5.0 prohibition must NOT over-reach to Opus 5.5, which is a valid top tier. Pinned
// as the-desk's top tier it both loads and resolves WITHOUT the coordinator's non-Opus refusal —
// whereas an older opus tier (Opus 4.8) in the same slot is still refused for the-desk.
func TestLoadModelPolicyAllowsOpusFiveFiveTierTarget(t *testing.T) {
	setTopModel := func(model string) func(map[string]any) {
		return func(m map[string]any) {
			m["deny"] = []any{}
			m["providers"].(map[string]any)["anthropic"].(map[string]any)["tiers"].(map[string]any)["top"].(map[string]any)["model"] = model
		}
	}
	m, err := loadModelPolicy(writeMutatedPolicy(t, setTopModel("claude-opus-5-5")))
	if err != nil {
		t.Fatalf("Opus 5.5 is a valid tier target and must load even with deny=[]: %v", err)
	}
	res, err := m.Resolve("the-desk", "", "", "")
	if err != nil {
		t.Fatalf("the-desk must accept Opus 5.5 as its top tier: %v", err)
	}
	if res.Model != "claude-opus-5-5" {
		t.Errorf("the-desk top tier = %q, want claude-opus-5-5", res.Model)
	}
	// The carve-out is Opus 5.5 ONLY: an older opus tier as the-desk's top is still refused.
	m2, err := loadModelPolicy(writeMutatedPolicy(t, setTopModel("claude-opus-4-8[1m]")))
	if err != nil {
		t.Fatalf("Opus 4.8 is not denied and must load: %v", err)
	}
	if _, err := m2.Resolve("the-desk", "", "", ""); err == nil {
		t.Error("the-desk must still refuse a non-5.5 opus top tier (Opus 4.8)")
	}
}

// TestLoadModelPolicyValidatesUnusedProvider ports test_invalid_unused_provider_still_fails: the
// whole file is validated, including a provider no role currently routes to.
func TestLoadModelPolicyValidatesUnusedProvider(t *testing.T) {
	path := writeMutatedPolicy(t, func(m map[string]any) {
		m["providers"].(map[string]any)["kimi"].(map[string]any)["tiers"].(map[string]any)["fast"].(map[string]any)["model"] = "opus"
	})
	if _, err := loadModelPolicy(path); err == nil {
		t.Fatal("a floating alias in an unused provider's tier must still refuse")
	}
}

func TestLoadModelPolicyUnsupportedEfforts(t *testing.T) {
	cases := []struct{ provider, effort string }{
		{"glm", "medium"}, {"kimi", "xhigh"}, {"codex", "max"},
	}
	for _, c := range cases {
		path := writeMutatedPolicy(t, func(m map[string]any) {
			spec := m["providers"].(map[string]any)[c.provider].(map[string]any)["tiers"].(map[string]any)["mid"].(map[string]any)
			spec["effort"] = c.effort
			spec["supported_efforts"] = append(spec["supported_efforts"].([]any), c.effort)
		})
		if _, err := loadModelPolicy(path); err == nil {
			t.Errorf("%s/%s: an effort outside that harness/provider's allowed set must refuse", c.provider, c.effort)
		}
	}
}

func TestResolveRoleResolution(t *testing.T) {
	m := loadExamplePolicy(t)
	res, err := m.Resolve("pr-review-desk", "", "", "")
	if err != nil {
		t.Fatalf("resolve pr-review-desk: %v", err)
	}
	if res.Model != "claude-opus-4-8[1m]" || res.Effort != "high" || res.Provider != "anthropic" {
		t.Errorf("pr-review-desk resolved %+v", res)
	}
	if res, err := m.Resolve("worker-desk", "", "", ""); err != nil || res.Provider != "glm" {
		t.Errorf("worker-desk provider = %+v, err=%v", res, err)
	}
	res, err = m.Resolve("intake-desk", "", "", "")
	if err != nil || res.Harness != "codex" || res.Effort != "medium" {
		t.Errorf("intake-desk = %+v, err=%v", res, err)
	}
}

// TestResolveDirectAndAliasRequestsDeniesOpus5Variants ports test_direct_and_alias_requests.
func TestResolveDirectAndAliasRequestsDeniesOpus5Variants(t *testing.T) {
	m := loadExamplePolicy(t)
	res, err := m.Resolve("pr-review-desk", "", "opus", "")
	if err != nil || res.Model != "claude-opus-4-8[1m]" {
		t.Fatalf("alias 'opus' = %+v, err=%v", res, err)
	}
	for _, v := range []string{"claude-opus-5", "Opus5", "claude-opus-5[1m]", "gateway/claude-opus-5", "latest", "opusplan"} {
		if _, err := m.Resolve("pr-review-desk", "", v, ""); err == nil {
			t.Errorf("requested=%q must refuse, resolved instead", v)
		}
	}
	// Deny anchoring: Opus 5.0 stays banned (canonical id, its `[1m]` / gateway spellings, the
	// no-hyphen `Opus5`, and the explicit `-5-0` / `-5.0` spellings of the same tier), while
	// Opus 5.5 — a valid top tier — is NOT banned, because `opus-5-5` does not end in `opus-5`.
	for _, v := range []string{
		"claude-opus-5", "Opus5", "claude-opus-5[1m]", "gateway/claude-opus-5",
		"claude-opus-5-0", "claude-opus-5.0",
	} {
		if !policyDenied(v, m.Banned) {
			t.Errorf("policyDenied(%q) = false, want true (Opus 5.0 is banned)", v)
		}
	}
	for _, v := range []string{
		"claude-opus-5-5", "claude-opus-5-5[1m]", "gateway/claude-opus-5-5",
		"claude-fable-5-1", "claude-sonnet-5", "claude-opus-4-8[1m]",
	} {
		if policyDenied(v, m.Banned) {
			t.Errorf("policyDenied(%q) = true, want false (not the Opus 5.0 tier)", v)
		}
	}
}

func TestResolveUnknownRoleAndProvider(t *testing.T) {
	m := loadExamplePolicy(t)
	if _, err := m.Resolve("typo", "", "", ""); err == nil {
		t.Error("an unknown role must refuse")
	}
	if _, err := m.Resolve("pr-review-desk", "typo", "", ""); err == nil {
		t.Error("an unknown provider override must refuse")
	}
	if _, err := m.Resolve("pr-review-desk", "glm", "", "codex"); err == nil {
		t.Error("a provider/harness override mismatch must refuse")
	}
}

func TestResolveExplicitOverrideRoutesProviderAndTier(t *testing.T) {
	m := loadExamplePolicy(t)
	res, err := m.Resolve("pr-review-desk", "kimi", "opus", "")
	if err != nil || res.Model != "k3[1m]" || res.Effort != "high" {
		t.Fatalf("provider override = %+v, err=%v", res, err)
	}
	res, err = m.Resolve("pr-review-desk", "", "", "codex")
	if err != nil || res.Provider != "codex" || res.Harness != "codex" {
		t.Fatalf("harness override = %+v, err=%v", res, err)
	}
}

// TestResolveDigestChangesWithEffort ports test_digest_changes_with_effort: the sha256 is of the
// raw file bytes, so any edit — including one that changes nothing about role routing — changes
// the digest `show`/dry-run print.
func TestResolveDigestChangesWithEffort(t *testing.T) {
	before := loadExamplePolicy(t).SHA256
	path := writeMutatedPolicy(t, func(m map[string]any) {
		m["providers"].(map[string]any)["anthropic"].(map[string]any)["tiers"].(map[string]any)["strong"].(map[string]any)["effort"] = "xhigh"
	})
	after, err := loadModelPolicy(path)
	if err != nil {
		t.Fatalf("mutated policy should still load: %v", err)
	}
	if before == after.SHA256 {
		t.Error("changing the policy body must change its sha256")
	}
}

// TestResolveChildMappingEffortPropagation ports test_child_mapping_allowlist_and_hooks' table —
// against ResolveChild directly rather than the oracle's PreToolUse hook subprocess, which this
// port does not implement (see policy.go's header comment and the PR body).
func TestResolveChildMappingEffortPropagation(t *testing.T) {
	m := loadExamplePolicy(t)
	res, err := m.Resolve("pr-review-desk", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Effort != "high" {
		t.Fatalf("parent effort = %q, want high", res.Effort)
	}
	cases := []struct {
		requested string
		wantErr   bool
		wantModel string
	}{
		{"opus", false, "claude-opus-4-8[1m]"},
		{"inherit", false, res.Model},
		{"", false, res.Model},
		{"claude-opus-5", true, ""},
		{"unknown-model", true, ""},
	}
	for _, c := range cases {
		model, effort, err := res.ResolveChild(c.requested)
		if c.wantErr {
			if err == nil {
				t.Errorf("child request %q must refuse, got model=%q effort=%q", c.requested, model, effort)
			}
			continue
		}
		if err != nil {
			t.Errorf("child request %q: %v", c.requested, err)
			continue
		}
		if model != c.wantModel || effort != res.Effort {
			t.Errorf("child request %q = (%q,%q), want (%q,%q)", c.requested, model, effort, c.wantModel, res.Effort)
		}
	}
}

// TestResolveChildRefusesUnsupportedInheritedEffort is the negative half of effort propagation:
// a child tier whose supported_efforts does not include the PARENT's effort is refused rather
// than silently dropping to a different effort.
func TestResolveChildRefusesUnsupportedInheritedEffort(t *testing.T) {
	path := writeMutatedPolicy(t, func(m map[string]any) {
		spec := m["providers"].(map[string]any)["anthropic"].(map[string]any)["tiers"].(map[string]any)["mid"].(map[string]any)
		spec["supported_efforts"] = []any{"low"}
		spec["effort"] = "low"
	})
	m, err := loadModelPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	res, err := m.Resolve("pr-review-desk", "", "", "") // strong tier, effort=high
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := res.ResolveChild("sonnet"); err == nil {
		t.Error("a child tier that does not support the parent's effort must refuse")
	}
}

func TestClaudeEnvAndCodexArgsPropagateEffort(t *testing.T) {
	m := loadExamplePolicy(t)
	res, err := m.Resolve("pr-review-desk", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.ClaudeEnv["CLAUDE_CODE_EFFORT_LEVEL"] != "high" {
		t.Errorf("CLAUDE_CODE_EFFORT_LEVEL = %q", res.ClaudeEnv["CLAUDE_CODE_EFFORT_LEVEL"])
	}
	if res.ClaudeEnv["ANTHROPIC_MODEL"] != res.Model || res.ClaudeEnv["CLAUDE_CODE_SUBAGENT_MODEL"] != res.Model {
		t.Errorf("ANTHROPIC_MODEL/CLAUDE_CODE_SUBAGENT_MODEL should mirror the resolved model, got %+v", res.ClaudeEnv)
	}
	if res.ClaudeEnv["ANTHROPIC_BASE_URL"] != "https://api.anthropic.com" {
		t.Errorf("ANTHROPIC_BASE_URL = %q, want the native anthropic endpoint", res.ClaudeEnv["ANTHROPIC_BASE_URL"])
	}

	intake, err := m.Resolve("intake-desk", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, a := range intake.CodexArgs {
		joined += a + " "
	}
	for _, want := range []string{`model_provider="openai"`, `model_reasoning_effort="medium"`, `agents.default_subagent_model="gpt-5.6-terra"`, `agents.default_subagent_reasoning_effort="medium"`} {
		if !contains(intake.CodexArgs, want) {
			t.Errorf("codex args %v missing %q", intake.CodexArgs, want)
		}
	}
	_ = joined
}

// TestCheckClaudeVersionOutput exercises the pure parsing/comparison half directly — no exec,
// no PATH, no binary literally named "claude" required. checkClaudeMinVersion's own argv[0] is
// the package constant claudeBinary precisely so the forge-CLI-shellout ban can resolve it; this
// test covers the logic behind it without threading a variable binary path through that call.
func TestCheckClaudeVersionOutput(t *testing.T) {
	if err := checkClaudeVersionOutput("2.1.251 (stub)"); err != nil {
		t.Errorf("exactly the floor must pass: %v", err)
	}
	if err := checkClaudeVersionOutput("2.1.278 (stub)"); err != nil {
		t.Errorf("above the floor must pass: %v", err)
	}
	if err := checkClaudeVersionOutput("2.1.200 (stub)"); err == nil {
		t.Error("below the floor must refuse")
	}
	if err := checkClaudeVersionOutput("1.9.999 (stub)"); err == nil {
		t.Error("an old major version must refuse")
	}
	if err := checkClaudeVersionOutput("not a version string"); err == nil {
		t.Error("unparseable --version output must refuse")
	}
}

// TestCheckClaudeMinVersionExecPath is the exec half, at the built-binary boundary: a stub
// literally named "claude" (matching the claudeBinary constant) on PATH, in both directions.
func TestCheckClaudeMinVersionExecPath(t *testing.T) {
	writeStubOnPath := func(t *testing.T, version string) {
		t.Helper()
		dir := t.TempDir()
		script := "#!/bin/sh\necho '" + version + " (stub)'\n"
		if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
	}
	t.Run("above floor passes", func(t *testing.T) {
		writeStubOnPath(t, "2.1.278")
		if err := checkClaudeMinVersion(); err != nil {
			t.Errorf("above the floor must pass: %v", err)
		}
	})
	t.Run("below floor refuses", func(t *testing.T) {
		writeStubOnPath(t, "2.1.200")
		if err := checkClaudeMinVersion(); err == nil {
			t.Error("below the floor must refuse")
		}
	})
	t.Run("no claude on PATH refuses", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if err := checkClaudeMinVersion(); err == nil {
			t.Error("a harness binary that cannot even report --version must refuse")
		}
	})
}
