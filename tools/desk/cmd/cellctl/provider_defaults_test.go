package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func catalogFixture(t *testing.T) *policyFixture {
	t.Helper()
	f := newPolicyFixture(t, "2.1.278")
	path := filepath.Join(f.cellDir, "cell.env")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(raw), "CELL_MODEL_POLICY=model-policy.json\n", "")), 0600); err != nil {
		t.Fatal(err)
	}
	r := f.run(t, nil, "providers", "init")
	if r.code != 0 {
		t.Fatalf("init: %s", r.stderr)
	}
	return f
}

func writeCatalogJSON(t *testing.T, path string, value any) {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func mutateCatalog(t *testing.T, path string, fn func(map[string]any)) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	fn(m)
	writeCatalogJSON(t, path, m)
}

func TestProviderDefaultsInitNeverOverwrites(t *testing.T) {
	f := catalogFixture(t)
	path := filepath.Join(f.cellsRoot, "providers.json")
	before, _ := os.ReadFile(path)
	r := f.run(t, nil, "providers", "init")
	if r.code == 0 || !strings.Contains(r.stderr, "never overwritten") {
		t.Fatalf("second init: %+v", r)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("existing defaults were modified")
	}
}

func TestProviderDefaultsInheritanceAndCellExceptions(t *testing.T) {
	f := catalogFixture(t)
	r := f.dryRunDesk(t, "pr-review-desk")
	if r.code != 0 || !strings.Contains(r.stdout, "model=claude-opus-5-5[1m]") || !strings.Contains(r.stdout, "effort=high") {
		t.Fatalf("initial default: %+v", r)
	}
	// The cell overrides one desk's effort, and selects a different provider for its worker.
	writeCatalogJSON(t, filepath.Join(f.cellDir, "providers.json"), map[string]any{
		"roles":     map[string]string{"worker-desk": "glm"},
		"providers": map[string]any{"anthropic": map[string]any{"desks": map[string]any{"pr-review-desk": map[string]string{"effort": "xhigh"}}}},
	})
	mutateCatalog(t, filepath.Join(f.cellsRoot, "providers.json"), func(m map[string]any) {
		m["providers"].(map[string]any)["anthropic"].(map[string]any)["tiers"].(map[string]any)["strong"].(map[string]any)["model"] = "claude-opus-4-8-revised[1m]"
	})
	r = f.dryRunDesk(t, "pr-review-desk")
	if r.code != 0 || !strings.Contains(r.stdout, "model=claude-opus-4-8-revised[1m]") || !strings.Contains(r.stdout, "effort=xhigh") || !strings.Contains(r.stdout, " + ") {
		t.Fatalf("inherited model + local effort: %+v", r)
	}
	r = f.dryRunDesk(t, "verify-desk")
	if r.code != 0 || !strings.Contains(r.stdout, "model=claude-opus-4-8-revised[1m]") || !strings.Contains(r.stdout, "effort=high") {
		t.Fatalf("unmodified desk: %+v", r)
	}
	r = f.dryRunDesk(t, "worker-desk")
	if r.code != 0 || !strings.Contains(r.stdout, "model=glm-5.3-flash[1m]") || !strings.Contains(r.stdout, "provider=glm") {
		t.Fatalf("mixed providers: %+v", r)
	}
	r = f.dryRunDesk(t, "worker-desk", "--provider", "kimi")
	if r.code != 0 || !strings.Contains(r.stdout, "model=k3[1m]") {
		t.Fatalf("explicit provider wins: %+v", r)
	}
	r = f.run(t, nil, "show", "example")
	if r.code != 0 || !strings.Contains(r.stdout, "model pr-review-desk=claude-opus-4-8-revised[1m] provider=anthropic harness=claude effort=xhigh") {
		t.Fatalf("show disagrees: %+v", r)
	}
}

func TestProviderDefaultsLegacyPolicyWins(t *testing.T) {
	f := catalogFixture(t)
	if err := os.WriteFile(filepath.Join(f.cellsRoot, "providers.json"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	r := f.run(t, nil, "set", "example", "CELL_MODEL_POLICY=model-policy.json")
	if r.code != 0 {
		t.Fatal(r.stderr)
	}
	r = f.dryRunDesk(t, "pr-review-desk")
	if r.code != 0 || !strings.Contains(r.stdout, "model=claude-opus-4-8[1m]") {
		t.Fatalf("legacy policy: %+v", r)
	}
}

func TestProviderDefaultsRefuseInvalidOverrides(t *testing.T) {
	for _, tc := range []struct{ name, patch, want string }{
		{"unknown field", `{"providers":{"anthropic":{"desks":{"pr-review-desk":{"efort":"high"}}}}}`, "unknown field"},
		{"unsupported effort", `{"providers":{"kimi":{"desks":{"worker-desk":{"effort":"medium"}}}}}`, "unsupported desk effort"},
		{"banned model", `{"providers":{"anthropic":{"tiers":{"strong":{"model":"claude-opus-5"}}}}}`, "denied model"},
		{"unknown role", `{"roles":{"work-desk":"glm"}}`, "unknown role"},
		{"null", `{"providers":{"anthropic":null}}`, "cannot be null"},
		{"schema", `{"schema":2}`, "unsupported schema"},
		{"floating alias", `{"providers":{"glm":{"tiers":{"mid":{"model":"sonnet"}}}}}`, "floating aliases"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := catalogFixture(t)
			if err := os.WriteFile(filepath.Join(f.cellDir, "providers.json"), []byte(tc.patch), 0600); err != nil {
				t.Fatal(err)
			}
			r := f.dryRunDesk(t, "worker-desk")
			if r.code == 0 || !strings.Contains(r.stderr, tc.want) {
				t.Fatalf("wanted %s: %+v", tc.want, r)
			}
		})
	}
}

func TestProviderDefaultsCannotRemoveSharedDeny(t *testing.T) {
	f := catalogFixture(t)
	mutateCatalog(t, filepath.Join(f.cellsRoot, "providers.json"), func(m map[string]any) { m["deny"] = []string{"*opus-5*", "forbidden-model"} })
	writeCatalogJSON(t, filepath.Join(f.cellDir, "providers.json"), map[string]any{"deny": []string{}, "providers": map[string]any{"anthropic": map[string]any{"tiers": map[string]any{"strong": map[string]string{"model": "forbidden-model"}}}}})
	r := f.dryRunDesk(t, "pr-review-desk")
	if r.code == 0 || !strings.Contains(r.stderr, "denied model") {
		t.Fatalf("shared deny lost: %+v", r)
	}
}

func TestProviderDefaultsMissingExplicitFileRefuses(t *testing.T) {
	f := catalogFixture(t)
	r := f.run(t, nil, "set", "example", "CELL_PROVIDER_DEFAULTS=missing.json")
	if r.code != 0 {
		t.Fatal(r.stderr)
	}
	r = f.dryRunDesk(t, "worker-desk")
	if r.code == 0 || !strings.Contains(r.stderr, "cannot read provider defaults") {
		t.Fatalf("missing explicit defaults: %+v", r)
	}
}

// Exercise the actual built-binary launch. Git fetch/worktree operations use only a
// temporary repository with a filesystem origin; the harness prints selected env keys
// instead of contacting any model endpoint. No tokens or host git helpers are used.
func (f *policyFixture) prepareLocalLaunch(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(f.repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", f.repoDir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+filepath.Join(f.cellDir, "absent-gitconfig"), "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %v %s", args, err, out)
		}
	}
	git("init", "-b", "main")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "fixture")
	git("remote", "add", "origin", f.repoDir)
	script := `#!/bin/sh
case "$1" in
 --version) echo '2.1.278 (stub)'; exit 0;;
 plugin) exit 0;;
esac
printf 'ARGV=%s\n' "$*"
printf 'ANTHROPIC_MODEL=%s\n' "${ANTHROPIC_MODEL-}"
printf 'FABLE=%s\n' "${ANTHROPIC_DEFAULT_FABLE_MODEL-}"
printf 'OPUS=%s\n' "${ANTHROPIC_DEFAULT_OPUS_MODEL-}"
printf 'SONNET=%s\n' "${ANTHROPIC_DEFAULT_SONNET_MODEL-}"
printf 'HAIKU=%s\n' "${ANTHROPIC_DEFAULT_HAIKU_MODEL-}"
printf 'EFFORT=%s\n' "${CLAUDE_CODE_EFFORT_LEVEL-}"
printf 'PROMPT_SUGGESTION=%s\n' "${CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION-}"
printf 'CHILD=%s\n' "${CLAUDE_CODE_SUBAGENT_MODEL-}"
printf 'BASE_URL=%s\n' "${ANTHROPIC_BASE_URL-}"
`
	if err := os.WriteFile(filepath.Join(f.binDir, "claude"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
}

func TestProviderDefaultsBuiltBinaryLaunch(t *testing.T) {
	for _, tc := range []struct{ provider, role, model, fable, opus, sonnet, haiku, effort, base string }{
		{"anthropic", "pr-review-desk", "claude-opus-5-5[1m]", "claude-fable-5-1", "claude-opus-5-5[1m]", "claude-sonnet-5", "claude-sonnet-5", "high", "https://api.anthropic.com"},
		{"glm", "worker-desk", "glm-5.3-flash[1m]", "glm-5.3[1m]", "glm-5.3[1m]", "glm-5.3-flash[1m]", "glm-5.3-flash[1m]", "high", "https://api.z.ai/api/anthropic"},
		{"kimi", "verify-desk", "k3[1m]", "k3[1m]", "k3[1m]", "k3[1m]", "k3[1m]", "high", "https://api.kimi.com/coding"},
	} {
		t.Run(tc.provider, func(t *testing.T) {
			f := catalogFixture(t)
			f.prepareLocalLaunch(t)
			r := f.run(t, []string{"CELLCTL_DESKWT=0", "ANTHROPIC_DEFAULT_OPUS_MODEL=stale-alias", "CLAUDE_CODE_EFFORT_LEVEL=low", "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=true"}, "desk", "example", tc.role, "--provider", tc.provider)
			if r.code != 0 {
				t.Fatalf("launch: %+v", r)
			}
			for key, value := range map[string]string{"ANTHROPIC_MODEL": tc.model, "FABLE": tc.fable, "OPUS": tc.opus, "SONNET": tc.sonnet, "HAIKU": tc.haiku, "EFFORT": tc.effort, "CHILD": tc.model, "BASE_URL": tc.base, "PROMPT_SUGGESTION": "false"} {
				if !strings.Contains(r.stdout, key+"="+value+"\n") {
					t.Errorf("missing %s=%s in %s", key, value, r.stdout)
				}
			}
			if !strings.Contains(r.stdout, "ARGV=--effort "+tc.effort+" ") {
				t.Errorf("effort not in argv: %s", r.stdout)
			}
		})
	}
}

func TestProviderDefaultsSharedAcrossCells(t *testing.T) {
	f := catalogFixture(t)
	other := filepath.Join(f.cellsRoot, "other")
	if err := os.MkdirAll(other, 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(f.cellDir, "cell.env"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "cell.env"), []byte(strings.ReplaceAll(string(raw), "CELL=example", "CELL=other")), 0600); err != nil {
		t.Fatal(err)
	}
	writeCatalogJSON(t, filepath.Join(f.cellDir, "providers.json"), map[string]any{"providers": map[string]any{"anthropic": map[string]any{"tiers": map[string]any{"strong": map[string]string{"model": "claude-opus-4-8-cell[1m]"}}}}})
	mutateCatalog(t, filepath.Join(f.cellsRoot, "providers.json"), func(m map[string]any) {
		m["providers"].(map[string]any)["anthropic"].(map[string]any)["tiers"].(map[string]any)["strong"].(map[string]any)["model"] = "claude-opus-4-8-shared[1m]"
	})
	for cell, model := range map[string]string{"example": "claude-opus-4-8-cell[1m]", "other": "claude-opus-4-8-shared[1m]"} {
		r := f.run(t, nil, "show", cell)
		if r.code != 0 || !strings.Contains(r.stdout, "model pr-review-desk="+model) {
			t.Fatalf("%s: %+v", cell, r)
		}
	}
	if err := os.Remove(filepath.Join(f.cellDir, "providers.json")); err != nil {
		t.Fatal(err)
	}
	r := f.dryRunDesk(t, "pr-review-desk")
	if r.code != 0 || !strings.Contains(r.stdout, "model=claude-opus-4-8-shared[1m]") {
		t.Fatalf("inheritance not restored: %+v", r)
	}
}

func TestProviderDefaultsCustomPaths(t *testing.T) {
	f := catalogFixture(t)
	if err := os.Rename(filepath.Join(f.cellsRoot, "providers.json"), filepath.Join(f.cellsRoot, "team.json")); err != nil {
		t.Fatal(err)
	}
	writeCatalogJSON(t, filepath.Join(f.cellDir, "local.json"), map[string]any{"default_provider": "kimi"})
	r := f.run(t, nil, "set", "example", "CELL_PROVIDER_DEFAULTS=team.json", "CELL_PROVIDER_OVERRIDES=local.json")
	if r.code != 0 {
		t.Fatal(r.stderr)
	}
	r = f.dryRunDesk(t, "worker-desk")
	if r.code != 0 || !strings.Contains(r.stdout, "model=k3[1m]") || !strings.Contains(r.stdout, "team.json + "+filepath.Join(f.cellDir, "local.json")) {
		t.Fatalf("custom relative paths: %+v", r)
	}
}

func TestProviderDefaultsBadSharedFileCannotBeMasked(t *testing.T) {
	f := catalogFixture(t)
	mutateCatalog(t, filepath.Join(f.cellsRoot, "providers.json"), func(m map[string]any) { m["schema"] = 2 })
	writeCatalogJSON(t, filepath.Join(f.cellDir, "providers.json"), map[string]any{"schema": 1})
	r := f.dryRunDesk(t, "worker-desk")
	if r.code == 0 || !strings.Contains(r.stderr, "unsupported schema") {
		t.Fatalf("bad base masked: %+v", r)
	}
	if err := os.Remove(filepath.Join(f.cellsRoot, "providers.json")); err != nil {
		t.Fatal(err)
	}
	r = f.dryRunDesk(t, "worker-desk")
	if r.code == 0 || !strings.Contains(r.stderr, "cannot read provider defaults") {
		t.Fatalf("orphan override ignored: %+v", r)
	}
}

func TestProviderDefaultsCodexLaunch(t *testing.T) {
	f := catalogFixture(t)
	f.prepareLocalLaunch(t)
	if err := os.WriteFile(filepath.Join(f.binDir, "codex"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	writeCatalogJSON(t, filepath.Join(f.cellDir, "providers.json"), map[string]any{"roles": map[string]string{"worker-desk": "codex"}})
	r := f.run(t, []string{"CELLCTL_DESKWT=0"}, "desk", "example", "worker-desk")
	if r.code != 0 {
		t.Fatalf("codex launch: %+v", r)
	}
	for _, want := range []string{"-m\ngpt-5.6-terra\n", "model_reasoning_effort=\"medium\"", "agents.default_subagent_model=\"gpt-5.6-terra\"", "agents.default_subagent_reasoning_effort=\"medium\""} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("missing %s in %s", want, r.stdout)
		}
	}
}

func TestProviderDefaultsCheckAndUpUseEffectiveRoles(t *testing.T) {
	f := catalogFixture(t)
	for _, name := range []string{"tmux", "orca", "deskroster", "codex"} {
		if err := os.WriteFile(filepath.Join(f.binDir, name), []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeCatalogJSON(t, filepath.Join(f.cellDir, "providers.json"), map[string]any{"roles": map[string]string{"worker-desk": "glm", "intake-desk": "codex"}})
	r := f.run(t, []string{"CELL_COCKPIT=tmux"}, "check", "example")
	// The fixture intentionally lacks the other house provisioning preconditions.
	for _, want := range []string{"role worker-desk: provider=glm harness=claude model=glm-5.3-flash[1m] effort=high", "provider glm: token environment variable ZAI_API_KEY is set", "codex on PATH"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("check missing %s: %+v", want, r)
		}
	}
	if strings.Contains(r.stdout, "CELL_PROVIDER unset") {
		t.Errorf("check reports obsolete legacy provider: %s", r.stdout)
	}
	r = f.run(t, []string{"DRY_RUN=1"}, "up", "example", "--cockpit", "tmux", "--no-attach")
	if r.code != 0 || !strings.Contains(r.stdout, "role=intake-desk provider=codex model=gpt-5.6-terra effort=medium") {
		t.Fatalf("up ignored policy: %+v", r)
	}
	r = f.run(t, []string{"DRY_RUN=1"}, "up", "example", "--cockpit", "tmux", "--model", "claude-opus-5")
	if r.code == 0 || !strings.Contains(r.stderr, "denied model") || strings.Contains(r.stdout, "[dry-run] cell=") {
		t.Fatalf("up accepted denied model: %+v", r)
	}
}

func TestProviderDefaultsRejectScheduledLaunch(t *testing.T) {
	f := catalogFixture(t)
	if err := os.WriteFile(filepath.Join(f.binDir, "orca"), []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	r := f.run(t, []string{"DRY_RUN=1"}, "up", "example", "--cockpit", "orca", "--automate", "hourly")
	if r.code == 0 || !strings.Contains(r.stderr, "cannot propagate model policy") || strings.Contains(r.stdout, "automations create") {
		t.Fatalf("scheduled launch bypassed policy: %+v", r)
	}
}
