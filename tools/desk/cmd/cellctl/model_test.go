package main

import "testing"

func envWith(kv map[string]string) *Env {
	e := &Env{vals: map[string]string{}, set: map[string]bool{}}
	for k, v := range kv {
		e.Put(k, v)
	}
	return e
}

func TestResolveRoleModelPrecedence(t *testing.T) {
	c := &Cell{Env: envWith(map[string]string{
		"DESK_MODEL_the_desk":   "fable",
		"DESK_MODEL_DEFAULT":    "sonnet",
		"CODEX_MODEL_default":   "",
		"TIER_MODEL_TOP_CODEX":  "gpt-5.6-terra",
		"TIER_MODEL_MID_CODEX":  "gpt-5.6-terra",
		"TIER_MODEL_TOP_CLAUDE": "fable",
		"TIER_MODEL_MID_CLAUDE": "sonnet",
	})}
	// (1) the harness's own per-role pin
	if rm := c.resolveRoleModel("the-desk", "claude"); rm.Model != "fable" || rm.Src != "DESK_MODEL_the_desk" {
		t.Errorf("per-role pin: %+v", rm)
	}
	// (2) the harness's own default
	if rm := c.resolveRoleModel("worker-desk", "claude"); rm.Model != "sonnet" || rm.Src != "DESK_MODEL_DEFAULT" {
		t.Errorf("harness default: %+v", rm)
	}
	// (3) the tier map — and NEVER the other harness's namespace
	rm := c.resolveRoleModel("the-desk", "codex")
	if rm.Model != "gpt-5.6-terra" || rm.Src != "tier:top (TIER_MODEL_TOP_CODEX)" {
		t.Errorf("tier fallback: %+v", rm)
	}
	// total failure names every place looked
	bare := &Cell{Env: envWith(map[string]string{})}
	if rm := bare.resolveRoleModel("worker-desk", "codex"); rm.OK {
		t.Errorf("expected no resolution, got %+v", rm)
	} else if rm.Src == "" {
		t.Error("an unresolved model must name what was checked")
	}
}

func TestOpusRefusalBindsTheDeskOnly(t *testing.T) {
	// Refused for the-desk: the bare `opus` strong-tier alias (no version, so below the floor),
	// and every opus tier BELOW the 5.5 floor — Opus 4.8 and Opus 5.0 (`claude-opus-5`, its
	// `[1m]` variant and the explicit `-5-0` / `-5.0` spellings).
	for _, m := range []string{
		"opus", "Opus", "OPUS",
		"claude-opus-4-8", "claude-opus-4-8[1m]",
		"claude-opus-5", "CLAUDE-OPUS-5", "claude-opus-5[1m]", "claude-opus-5-0", "claude-opus-5.0",
	} {
		if !isOpusPin(m) {
			t.Errorf("isOpusPin(%q) = false, want true (an opus tier below the 5.5 floor)", m)
		}
	}
	// Allowed: non-opus tiers, AND every opus tier AT OR ABOVE the 5.5 floor. The-desk's opus gate
	// is a VERSION FLOOR (>= 5.5), not a fixed 5.5-only allowlist, so 5.6, 6.0 and a future 9.0
	// auto-qualify without a code edit (case-insensitive, `[1m]`-insensitive, `-`/`.`-insensitive).
	for _, m := range []string{
		"fable", "sonnet", "haiku", "gpt-5.6-terra", "opusculum",
		"claude-opus-5-5", "Claude-Opus-5-5", "claude-opus-5-5[1m]", "claude-opus-5.5",
		"claude-opus-5-6", "claude-opus-6-0", "claude-opus-6", "claude-opus-9", "CLAUDE-OPUS-9",
	} {
		if isOpusPin(m) {
			t.Errorf("isOpusPin(%q) = true, want false (an opus tier at or above the 5.5 floor)", m)
		}
	}
	assertDies(t, "opus for the-desk", func() { refuseOpusForTheDesk("opus") })
	assertDies(t, "opus-5 for the-desk", func() { refuseOpusForTheDesk("claude-opus-5") })
	assertDies(t, "opus-4-8 for the-desk", func() { refuseOpusForTheDesk("claude-opus-4-8") })
	refuseOpusForTheDesk("fable")               // must not refuse
	refuseOpusForTheDesk("claude-opus-5-5")     // Opus 5.5 is at the floor — must not refuse
	refuseOpusForTheDesk("claude-opus-5-5[1m]") // the [1m] variant too
	refuseOpusForTheDesk("claude-opus-5-6")     // above the floor — must not refuse
	refuseOpusForTheDesk("claude-opus-6-0")     // above the floor — must not refuse
}

func TestRoleTier(t *testing.T) {
	if roleTier("the-desk") != "top" {
		t.Error("the-desk resolves at the TOP tier")
	}
	for _, r := range []string{"worker-desk", "verify-desk", "intake-desk", "pr-review-desk"} {
		if roleTier(r) != "mid" {
			t.Errorf("%s resolves at MID", r)
		}
	}
}

func TestProviderPresetsCarryNoCredential(t *testing.T) {
	c := &Cell{Env: envWith(map[string]string{})}
	for _, name := range []string{"kimi", "glm"} {
		base, src := c.providerValue(name, "BASE_URL")
		if base == "" || src != "preset" {
			t.Errorf("%s BASE_URL = %q (%s), want a preset value", name, base, src)
		}
		tok, src := c.providerValue(name, "TOKEN_ENV")
		if tok == "" || src != "preset" {
			t.Errorf("%s TOKEN_ENV = %q (%s), want a preset value", name, tok, src)
		}
		// The preset names an env VAR, never a token. A value that looked like a credential
		// here would be a leak in the source tree.
		if len(tok) > 0 && tok != "KIMI_API_KEY" && tok != "ZAI_API_KEY" {
			t.Errorf("%s TOKEN_ENV = %q, want the NAME of an env var", name, tok)
		}
	}
	// A cell.env line overrides its preset piecewise.
	c = &Cell{Env: envWith(map[string]string{"CELL_PROVIDER_GLM_MODEL": "mine"})}
	if v, src := c.providerValue("glm", "MODEL"); v != "mine" || src != "cell.env" {
		t.Errorf("cell.env override = %q (%s)", v, src)
	}
	if v, src := c.providerValue("glm", "BASE_URL"); src != "preset" || v == "" {
		t.Errorf("unoverridden key should still resolve from the preset, got %q (%s)", v, src)
	}
}

func TestProviderVarNaming(t *testing.T) {
	if got := providerVar("z-ai", "BASE_URL"); got != "CELL_PROVIDER_Z_AI_BASE_URL" {
		t.Errorf("providerVar = %q", got)
	}
}
