package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKnownCellEnvKeyFamilies(t *testing.T) {
	for _, k := range []string{
		"CELL_KIND", "CELL_ROOTS", "DESK_MODEL_DEFAULT", "CODEX_MODEL_default",
		"DESK_MODEL_the_desk", "CODEX_MODEL_worker_desk",
		"CELL_PROVIDER_ZAI_BASE_URL", "CELL_PROVIDER_ZAI_TOKEN_ENV", "CELL_PROVIDER_ZAI_MODEL",
		"TIER_MODEL_TOP_CODEX",
	} {
		if !knownCellEnvKey(k) {
			t.Errorf("knownCellEnvKey(%q) = false, want true", k)
		}
	}
	// A typo'd role/provider name must NOT scaffold a variable nothing ever reads.
	for _, k := range []string{"DESK_MODEL_woker_desk", "CODEX_MODEL_nope", "NOT_A_REAL_KEY", "CELL_PROVIDER__BASE_URL"} {
		if knownCellEnvKey(k) {
			t.Errorf("knownCellEnvKey(%q) = true, want false", k)
		}
	}
}

func TestValidateEnvKeyRules(t *testing.T) {
	assertDies(t, "bad key shape", func() { validateEnvKey("1BAD", "x", true) })
	assertDies(t, "unknown key without --force", func() { validateEnvKey("NOT_A_REAL_KEY", "x", false) })
	validateEnvKey("NOT_A_REAL_KEY", "x", true) // --force widens the KEY allowlist
	// …but never the VALUE rules: the Opus refusal and the value sets are not bypassable.
	assertDies(t, "opus pin", func() { validateEnvKey("DESK_MODEL_the_desk", "opus", true) })
	assertDies(t, "opus-5 pin", func() { validateEnvKey("DESK_MODEL_the_desk", "claude-opus-5", true) })
	// …but Opus 5.5 is a valid top tier, so pinning the-desk to it is accepted (not an opus pin).
	validateEnvKey("DESK_MODEL_the_desk", "claude-opus-5-5", true)
	validateEnvKey("DESK_MODEL_the_desk", "claude-opus-5-5[1m]", true)
	assertDies(t, "bad harness", func() { validateEnvKey("CELL_HARNESS", "bogus", true) })
	assertDies(t, "bad kind", func() { validateEnvKey("CELL_KIND", "bogus", true) })
	assertDies(t, "bad cockpit", func() { validateEnvKey("CELL_COCKPIT", "bogus", true) })
}

func TestSetEnvKeyRewritesInPlaceAndAppends(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cell.env")
	body := "# header comment\nCELL=demo\nDESK_MODEL_DEFAULT=sonnet\n# trailing comment\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() { setEnvKey(p, "DESK_MODEL_DEFAULT", "haiku", false) })
	if !strings.Contains(out, "[set] DESK_MODEL_DEFAULT: sonnet -> haiku") {
		t.Errorf("before/after line missing: %q", out)
	}
	got, _ := os.ReadFile(p)
	want := "# header comment\nCELL=demo\nDESK_MODEL_DEFAULT=haiku\n# trailing comment\n"
	if string(got) != want {
		t.Errorf("in-place rewrite changed more than the one line:\ngot:\n%s\nwant:\n%s", got, want)
	}
	out = captureStdout(t, func() { setEnvKey(p, "CELL_COCKPIT", "tmux", false) })
	if !strings.Contains(out, "[set] CELL_COCKPIT: <unset> -> tmux") {
		t.Errorf("append line missing: %q", out)
	}
	got, _ = os.ReadFile(p)
	if !strings.HasSuffix(string(got), "CELL_COCKPIT=tmux\n") {
		t.Errorf("appended key is not at the end:\n%s", got)
	}
}

func TestApplyKVsValidatesBeforeWrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cell.env")
	body := "CELL=demo\nDESK_MODEL_DEFAULT=sonnet\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	e := envWith(map[string]string{})
	// A refusal on pair 2 of 2 must leave cell.env exactly as it found it — no backup, no edit.
	assertDies(t, "second pair invalid", func() {
		applyEnvKVs(e, p, false, []string{"DESK_MODEL_DEFAULT=haiku", "DESK_MODEL_the_desk=opus"})
	})
	got, _ := os.ReadFile(p)
	if string(got) != body {
		t.Errorf("cell.env was modified despite a refusal:\n%s", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, en := range entries {
		if strings.Contains(en.Name(), ".bak-") {
			t.Errorf("a backup was written despite a refusal: %s", en.Name())
		}
	}
}

func TestKindChangeNeedsPrecondition(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cell.env")
	if err := os.WriteFile(p, []byte("CELL=demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	assertDies(t, "house without CELL_ROOTS", func() { validateKindChange(p, "house", []string{"CELL_KIND=house"}) })
	// …but the same call that supplies it passes.
	validateKindChange(p, "house", []string{"CELL_KIND=house", "CELL_ROOTS=a/b=/tmp"})
	// k8s has no extra precondition.
	validateKindChange(p, "k8s", []string{"CELL_KIND=k8s"})
}

func TestEnvFileValueReadsTheLastActiveLine(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cell.env")
	if err := os.WriteFile(p, []byte("#CELL_HARNESS=commented\nCELL_HARNESS=claude\nCELL_HARNESS=codex\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if v, ok := envFileValue(p, "CELL_HARNESS"); !ok || v != "codex" {
		t.Errorf("envFileValue = %q,%v — want the LAST active line", v, ok)
	}
	if activeHarnessOf(p) != "codex" {
		t.Error("activeHarnessOf must read the last active line")
	}
	empty := filepath.Join(dir, "empty.env")
	_ = os.WriteFile(empty, []byte("CELL=demo\n"), 0o600)
	if activeHarnessOf(empty) != "claude" {
		t.Error("a cell.env with no CELL_HARNESS line defaults to claude")
	}
}

// TestSetExpandsVariablesAsBootDoes: a `$VAR` in a defaults-file line, in a cell.env line or in
// a pair `set` is about to write means to `set` what it will mean to the launch that reads the
// file — the launching shell's value. loadCell is the reference; `set` validating a path it
// expanded against nothing would judge a different path than the one a cell boots with.
func TestSetExpandsVariablesAsBootDoes(t *testing.T) {
	root := defaultsRoot(t)
	for _, k := range []string{"CELLCTL_TEST_FROM_DEFAULTS", "CELLCTL_TEST_BRACED", "CELLCTL_TEST_FROM_CELL", "CELLCTL_TEST_CHAINED", "CELLCTL_TEST_PAIR"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("CELLCTL_TEST_PROBE", "/example/base")
	writeDefaults(t, root, "CELLCTL_TEST_FROM_DEFAULTS=$CELLCTL_TEST_PROBE/defaults\nCELLCTL_TEST_BRACED=\"${CELLCTL_TEST_PROBE}/braced\"\n")
	envfile := writeCell(t, root, "demo", demoCell+
		"CELLCTL_TEST_FROM_CELL=$CELLCTL_TEST_PROBE/cell\nCELLCTL_TEST_CHAINED=$CELLCTL_TEST_FROM_DEFAULTS/$CELLCTL_TEST_FROM_CELL\n")
	want := map[string]string{
		"CELLCTL_TEST_FROM_DEFAULTS": "/example/base/defaults",
		"CELLCTL_TEST_BRACED":        "/example/base/braced",
		"CELLCTL_TEST_FROM_CELL":     "/example/base/cell",
		"CELLCTL_TEST_CHAINED":       "/example/base/defaults//example/base/cell",
	}
	boot := loadCell("demo")
	seen := effectiveCellEnv(envfile, []string{"CELLCTL_TEST_PAIR=$CELLCTL_TEST_PROBE/pair"})
	for k, w := range want {
		if got := boot.Env.Get(k); got != w {
			t.Fatalf("fixture: the cell boots with %s=%q, want %q", k, got, w)
		}
		if got := seen.Get(k); got != w {
			t.Errorf("set reads %s=%q, the cell boots with %q", k, got, w)
		}
	}
	if got, w := seen.Get("CELLCTL_TEST_PAIR"), "/example/base/pair"; got != w {
		t.Errorf("set reads the pair it is about to write as %q; the line will boot as %q", got, w)
	}
	// The shell is read to expand a reference and for nothing else: a variable only the shell
	// sets is not part of what `set` judges.
	if seen.IsSet("CELLCTL_TEST_PROBE") {
		t.Errorf("a variable only the launching shell sets became part of set's view of the cell")
	}
}
