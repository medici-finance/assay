package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCellEnvShellForms(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cell.env")
	body := "" +
		"# a comment\n" +
		"CELL=demo\n" +
		"ROLES=\"the-desk worker-desk\"\n" +
		"CELL_ROOTS='example-org/example-repo=/tmp/r'\n" +
		"export CELL_HARNESS=codex\n" +
		"EMPTY=\n" +
		"  INDENTED=yes\n" +
		"not an assignment\n" +
		"DERIVED=$CELL-cell\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Env{vals: map[string]string{}, set: map[string]bool{}}
	if err := parseCellEnv(e, p); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"CELL":         "demo",
		"ROLES":        "the-desk worker-desk",
		"CELL_ROOTS":   "example-org/example-repo=/tmp/r",
		"CELL_HARNESS": "codex",
		"INDENTED":     "yes",
		"DERIVED":      "demo-cell",
	} {
		if got := e.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	// The `-` vs `:-` distinction the tier map depends on: set-to-empty is SET.
	if !e.IsSet("EMPTY") || e.Get("EMPTY") != "" {
		t.Errorf("EMPTY: set=%v value=%q, want set with an empty value", e.IsSet("EMPTY"), e.Get("EMPTY"))
	}
	if e.IsSet("not an assignment") {
		t.Error("a non-assignment line was parsed as a key")
	}
	if got := e.GetOrSet("EMPTY", "fallback"); got != "" {
		t.Errorf("GetOrSet on a set-but-empty key = %q, want \"\" (the ${K-default} form)", got)
	}
	if got := e.GetOr("EMPTY", "fallback"); got != "fallback" {
		t.Errorf("GetOr on a set-but-empty key = %q, want the fallback (the ${K:-default} form)", got)
	}
}

func TestRootsValid(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"example-org/example-repo=/abs/path", true},
		{"a/b=/p,c/d=/q", true},
		{"", true},
		{"example-org/example-repo=relative", false},
		{"noslash=/abs", false},
		{"a/b/c=/abs", false},
		{"a/b", false},
	} {
		if got := rootsValid(tc.in); got != tc.want {
			t.Errorf("rootsValid(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestPrescanKindOverrideRefusesUnknown(t *testing.T) {
	if got := prescanKindOverride([]string{"--model", "x", "--kind", "house"}); got != "house" {
		t.Errorf("prescanKindOverride = %q, want house", got)
	}
	if got := prescanKindOverride([]string{"--model", "x"}); got != "" {
		t.Errorf("prescanKindOverride with no --kind = %q, want empty", got)
	}
	assertDies(t, "unknown kind", func() { prescanKindOverride([]string{"--kind", "nope"}) })
	assertDies(t, "missing value", func() { prescanKindOverride([]string{"--kind"}) })
}

// assertDies runs fn and fails unless it raised the package's exit panic. die() writes to
// stderr, which the test binary shows only on failure, so a refusal's text stays visible where
// it matters without polluting a passing run.
func assertDies(t *testing.T, what string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%s: expected a refusal, got none", what)
			return
		}
		if _, ok := r.(exitCode); !ok {
			panic(r)
		}
	}()
	fn()
}
