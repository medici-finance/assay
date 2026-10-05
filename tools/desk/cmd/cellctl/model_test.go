package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

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

// TestTheDeskFloorSuffixedOpus50 is the-desk floor's half of the suffixed-ID coverage: a date or
// other tail on the Opus 5.0 id must not be read as a minor version (`claude-opus-5-20260101` is
// Opus 5.0, not Opus 5.20260101), so the floor refuses it like `claude-opus-5`, while a dated id
// AT or above the floor still passes. It shares opus50Suffixed / opus5xAllowed (policy_test.go)
// with the ban-set test, so both sites are held to one list of spellings.
func TestTheDeskFloorSuffixedOpus50(t *testing.T) {
	for _, m := range opus50Suffixed {
		if strings.Contains(strings.ToLower(m), "claude-opus") && !isOpusPin(m) {
			t.Errorf("isOpusPin(%q) = false, want true (a suffixed Opus 5.0 id)", m)
		}
	}
	for _, m := range []string{"claude-opus-5-5-20260101", "claude-opus-5-6-20260101", "claude-opus-6-20270101", "claude-opus-5-10"} {
		if isOpusPin(m) {
			t.Errorf("isOpusPin(%q) = true, want false (at or above the 5.5 floor)", m)
		}
	}
	assertDies(t, "dated opus-5 for the-desk", func() { refuseOpusForTheDesk("claude-opus-5-20260101") })
}

// opusVersionLitRe spots a string literal that spells an opus VERSION (`opus-5`, `opus5`,
// `opus.5`, a `*opus-5` glob …): the shape of a hand-rolled, fixed-spelling opus tier match.
var opusVersionLitRe = regexp.MustCompile(`(?i)opus[-.]?[0-9]`)

// opusVersionLits returns file:line for every string literal in src that spells an opus version.
func opusVersionLits(t *testing.T, name string, src any) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	var hits []string
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && opusVersionLitRe.MatchString(lit.Value) {
			hits = append(hits, fmt.Sprintf("%s: %s", fset.Position(lit.Pos()), lit.Value))
		}
		return true
	})
	return hits
}

// TestOpusTierParsedInOnePlace is the CLASS guard for fixed-spelling opus tier matching: a
// literal such as `*opus-5` matches only the spellings it names, so a suffixed id (a date, a
// provider tail) slips past it. Every opus-tier decision in this package must go through the one
// version parser, opusVersion (model.go), so no non-test source here may carry an opus-version
// string literal; the allow-list is empty. The planted fixture is the positive control — a guard
// whose matcher stopped matching fails here instead of reporting clean.
func TestOpusTierParsedInOnePlace(t *testing.T) {
	planted := "package x\n\nvar ban = []string{\"*opus-5\"}\n"
	if hits := opusVersionLits(t, "planted.go", planted); len(hits) != 1 {
		t.Fatalf("positive control: want 1 hit on the planted literal, got %v", hits)
	}
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("could-not-check: no package sources found (%v)", err)
	}
	scanned := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		scanned++
		for _, h := range opusVersionLits(t, f, nil) {
			t.Errorf("opus-version literal outside opusVersion (match the tier via opusVersion instead): %s", h)
		}
	}
	if scanned == 0 {
		t.Fatal("could-not-check: no non-test sources scanned")
	}
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
