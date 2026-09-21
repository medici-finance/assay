package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubHarnessesOnPath puts executable `claude` and `codex` stubs on PATH for the duration of a
// test. scrubbedComposeEnv resolves the harness binary on the PARENT PATH — that is the one
// parent-derived element a scrubbed launch keeps, named and pinned — and REFUSES when it cannot
// find one. Without this the tests below pass on a developer laptop (where a harness is
// installed) and die on a CI runner (where none is), which is a test that measures the machine
// rather than the code.
func stubHarnessesOnPath(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	for _, h := range []string{"claude", "codex"} {
		p := filepath.Join(bin, h)
		if err := os.WriteFile(p, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return bin
}

func scrubbedFixture(t *testing.T) *Cell {
	t.Helper()
	stubHarnessesOnPath(t)
	dir := t.TempDir()
	return &Cell{
		Env: envWith(map[string]string{
			"CELL_ROOTS": "example-org/example-repo=/tmp/r",
			"TERM":       "xterm-256color",
			"LANG":       "C",
			"HOME":       "/not/the/cell/home",
		}),
		Name:    "demo",
		Dir:     dir,
		Home:    filepath.Join(dir, "home"),
		Config:  filepath.Join(dir, "home", ".config", "assay"),
		Kind:    "scrubbed",
		Harness: "claude",
	}
}

// TestScrubbedEnvIsComposedNotInherited is the custody assertion behind the whole kind: the
// launch environment is built from the allowlist, and the launching shell contributes nothing
// that is not named in it.
func TestScrubbedEnvIsComposedNotInherited(t *testing.T) {
	c := scrubbedFixture(t)
	c.Env.Put("SSH_AUTH_SOCK", "/leaked/agent")
	c.Env.Put("GH_TOKEN", "leaked-token-value")
	c.Env.Put("AWS_SECRET_ACCESS_KEY", "leaked")
	env := c.scrubbedComposeEnv("the-desk", "claude", "demo-the-desk-20260920T000000Z")
	joined := strings.Join(env.Pairs, "\n")
	for _, leak := range []string{"SSH_AUTH_SOCK", "GH_TOKEN", "AWS_SECRET_ACCESS_KEY", "leaked"} {
		if strings.Contains(joined, leak) {
			t.Errorf("composed environment carries %q from the launching shell:\n%s", leak, joined)
		}
	}
	// Every key present must be one the allowlist names.
	for _, kv := range env.Pairs {
		k := kv[:strings.IndexByte(kv, '=')]
		if !valueIn(k, scrubbedEnvKeys) {
			t.Errorf("composed environment carries %q, which is not in the allowlist", k)
		}
	}
	// HOME is the CELL's, never the operator's.
	if got := envValue(env.Pairs, "HOME"); got != c.Home {
		t.Errorf("HOME = %q, want the cell home %q", got, c.Home)
	}
	// The cluster-isolation control is present.
	if got := envValue(env.Pairs, "KUBECONFIG"); got != "/dev/null" {
		t.Errorf("KUBECONFIG = %q, want /dev/null", got)
	}
}

func TestScrubbedHarnessNsExclusive(t *testing.T) {
	c := scrubbedFixture(t)
	claude := c.scrubbedComposeEnv("the-desk", "claude", "s")
	if envValue(claude.Pairs, "CLAUDE_CONFIG_DIR") == "" {
		t.Error("the claude arm must export CLAUDE_CONFIG_DIR")
	}
	if _, ok := envLookup(claude.Pairs, "CODEX_HOME"); ok {
		t.Error("the claude arm must NOT export CODEX_HOME")
	}
	codex := c.scrubbedComposeEnv("the-desk", "codex", "s")
	if envValue(codex.Pairs, "CODEX_HOME") == "" {
		t.Error("the codex arm must export CODEX_HOME")
	}
	if _, ok := envLookup(codex.Pairs, "CLAUDE_CONFIG_DIR"); ok {
		t.Error("the codex arm must NOT export CLAUDE_CONFIG_DIR")
	}
}

// TestScrubbedPromptSuggestionClaudeOnly is the scrubbed-launch half of assay#1436: a scrubbed
// desk/smoke session composes its whole environment through scrubbedComposeEnv (never inherits
// anything from the launching shell — TestScrubbedEnvIsComposedNotInherited above), so the
// prompt-suggestion default has to be an entry in the allowlist itself, not something set after
// the fact the way the non-scrubbed launch does it. Retains Codex behavior: the codex arm never
// carries this key at all, matching the claude-only knob it is.
func TestScrubbedPromptSuggestionClaudeOnly(t *testing.T) {
	c := scrubbedFixture(t)
	claude := c.scrubbedComposeEnv("the-desk", "claude", "s")
	if got := envValue(claude.Pairs, envClaudeCodePromptSuggestion); got != "false" {
		t.Errorf("the claude arm must export %s=false, got %q (pairs: %v)", envClaudeCodePromptSuggestion, got, claude.Pairs)
	}
	codex := c.scrubbedComposeEnv("the-desk", "codex", "s")
	if _, ok := envLookup(codex.Pairs, envClaudeCodePromptSuggestion); ok {
		t.Errorf("the codex arm must NOT export %s — Codex has no equivalent knob", envClaudeCodePromptSuggestion)
	}
}

func TestScrubbedPathKeepsShimPrefix(t *testing.T) {
	c := scrubbedFixture(t)
	c.Env.Put("CELL_PATH", "/only/this")
	c.Env.Put("DESK_TOOLS_BIN", "/opt/desk-tools/bin")
	env := c.scrubbedComposeEnv("the-desk", "claude", "s")
	p := envValue(env.Pairs, "PATH")
	if !strings.HasPrefix(p, filepath.Join(c.Dir, "shim")+":/opt/desk-tools/bin:") {
		t.Errorf("CELL_PATH overrode the shim prefix or the desk-tools bin: %q", p)
	}
	if !strings.HasSuffix(p, ":/only/this") {
		t.Errorf("CELL_PATH did not replace the trailing system part: %q", p)
	}
}

func TestScrubbedEnvSkipsKeysWithNoSource(t *testing.T) {
	c := scrubbedFixture(t)
	c.Env.Put("CELL_ROOTS", "")
	c.Env.Put("TERM", "")
	env := c.scrubbedComposeEnv("the-desk", "claude", "s")
	for _, k := range []string{"DESK_ROOTS", "TERM"} {
		if _, ok := envLookup(env.Pairs, k); ok {
			t.Errorf("%s has no source in this run and must be skipped, not exported empty", k)
		}
	}
}

func TestScrubbedSortedIsKeySorted(t *testing.T) {
	c := scrubbedFixture(t)
	env := c.scrubbedComposeEnv("the-desk", "claude", "s")
	for i := 1; i < len(env.Sorted); i++ {
		if env.Sorted[i-1] > env.Sorted[i] {
			t.Fatalf("plan env lines are not sorted: %q before %q", env.Sorted[i-1], env.Sorted[i])
		}
	}
	if len(env.Sorted) != len(env.Pairs) {
		t.Error("the sorted view must carry exactly the pairs the launch gets")
	}
}

func envLookup(pairs []string, k string) (string, bool) {
	for _, kv := range pairs {
		if strings.HasPrefix(kv, k+"=") {
			return strings.TrimPrefix(kv, k+"="), true
		}
	}
	return "", false
}

func envValue(pairs []string, k string) string {
	v, _ := envLookup(pairs, k)
	return v
}
