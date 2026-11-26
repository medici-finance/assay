package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// qgcQualified is the command-priority config the library read path accepts:
// the declared verifier env's GIT_CONFIG_COUNT=0, an admitted row's grep
// pins and core.quotePath=false alone, spelled literally so the test does not
// read the set it checks.
var qgcQualified = []struct {
	name string
	env  [][2]string
}{
	{"count-zero", [][2]string{{"GIT_CONFIG_COUNT", "0"}}},
	{"admitted-pins", [][2]string{
		{"GIT_CONFIG_COUNT", "2"},
		{"GIT_CONFIG_KEY_0", "grep.patternType"}, {"GIT_CONFIG_VALUE_0", "default"},
		{"GIT_CONFIG_KEY_1", "grep.extendedRegexp"}, {"GIT_CONFIG_VALUE_1", "false"},
	}},
	{"quotepath", [][2]string{{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", "core.quotePath"}, {"GIT_CONFIG_VALUE_0", "false"}}},
}

// qgcNative is near misses that must still select the native profile.
var qgcNative = []struct {
	name string
	env  [][2]string
}{
	{"count-empty", [][2]string{{"GIT_CONFIG_COUNT", ""}}},
	{"one-pin", [][2]string{{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", "grep.patternType"}, {"GIT_CONFIG_VALUE_0", "default"}}},
	{"pin-changed", [][2]string{
		{"GIT_CONFIG_COUNT", "2"},
		{"GIT_CONFIG_KEY_0", "grep.patternType"}, {"GIT_CONFIG_VALUE_0", "fixed"},
		{"GIT_CONFIG_KEY_1", "grep.extendedRegexp"}, {"GIT_CONFIG_VALUE_1", "false"},
	}},
	{"core-key", [][2]string{{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", "core.worktree"}, {"GIT_CONFIG_VALUE_0", "/"}}},
	{"quotepath-true", [][2]string{{"GIT_CONFIG_COUNT", "1"}, {"GIT_CONFIG_KEY_0", "core.quotePath"}, {"GIT_CONFIG_VALUE_0", "true"}}},
	{"pins-reordered", [][2]string{
		{"GIT_CONFIG_COUNT", "2"},
		{"GIT_CONFIG_KEY_0", "grep.extendedRegexp"}, {"GIT_CONFIG_VALUE_0", "false"},
		{"GIT_CONFIG_KEY_1", "grep.patternType"}, {"GIT_CONFIG_VALUE_1", "default"},
	}},
}

func qgcSetenv(t *testing.T, env [][2]string) {
	t.Helper()
	ffHermeticGitEnv(t)
	for _, kv := range env {
		t.Setenv(kv[0], kv[1])
	}
}

// TestGitCoreQualifiedConfig: the verifier's own git config env keeps a read
// on the library path, and every other command-priority config still sends it
// to the native profile.
// regression: #7007
func TestGitCoreQualifiedConfig(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.invalid")
	runGit(t, root, "commit", "--allow-empty", "-qm", "c")
	top, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range qgcQualified {
		t.Run(tc.name, func(t *testing.T) {
			qgcSetenv(t, tc.env)
			if read := gitCoreFor(root); read.err != nil || read.native != "" {
				t.Fatalf("qualified config declined: native=%q err=%v", read.native, read.err)
			}
			if vars := nativeGitEnvSet(); slices.Contains(vars, "GIT_CONFIG_COUNT") {
				t.Errorf("qualified count named as a native selector: %v", vars)
			}
			for name, fn := range map[string]func(string) (string, error){"provenanceRepoTop": provenanceRepoTop, "repoTopLevel": repoTopLevel} {
				if got, err := fn(root); err != nil || got != top {
					t.Errorf("%s = %q, %v; want %q", name, got, err, top)
				}
			}
		})
	}
	for _, tc := range qgcNative {
		t.Run(tc.name, func(t *testing.T) {
			qgcSetenv(t, tc.env)
			if read := gitCoreFor(root); read.native == "" {
				t.Fatal("unqualified command-priority config kept on the library path")
			}
			if vars := nativeGitEnvSet(); !slices.Contains(vars, "GIT_CONFIG_COUNT") {
				t.Errorf("unqualified count not named: %v", vars)
			}
		})
	}
	// A qualified count never excuses GIT_CONFIG_PARAMETERS beside it.
	for _, tc := range qgcQualified {
		t.Run(tc.name+"-parameters", func(t *testing.T) {
			qgcSetenv(t, tc.env)
			t.Setenv("GIT_CONFIG_PARAMETERS", "'core.worktree'='/'")
			if read := gitCoreFor(root); read.native == "" {
				t.Fatal("GIT_CONFIG_PARAMETERS kept on the library path beside a qualified count")
			}
			if vars := nativeGitEnvSet(); !slices.Contains(vars, "GIT_CONFIG_PARAMETERS") || slices.Contains(vars, "GIT_CONFIG_COUNT") {
				t.Errorf("native selectors = %v, want GIT_CONFIG_PARAMETERS only", vars)
			}
		})
	}
}

// TestFailFirstQualifiedEnv: --delivering-commit binds under the
// declared verifier env and under an admitted row's env (#6851's host lane).
// regression: #7007
func TestFailFirstQualifiedEnv(t *testing.T) {
	withBase(t, true)
	root, _, prep, deliv, _, _, briefPath := ffDeliveredRepo(t, false)
	for _, tc := range qgcQualified {
		t.Run(tc.name, func(t *testing.T) {
			qgcSetenv(t, tc.env)
			if base, got, err := failFirstBaseRev(root, "", deliv, briefPath); err != nil || base != prep || got != deliv {
				t.Errorf("base=%s delivering=%s err=%v, want %s %s", base, got, err, prep, deliv)
			}
		})
	}
}

// TestVerifyflipQualifiedEnv: verifyflip's outcome-record read flips under the
// verifier's git config env (desk-tools/28 row 4's -run Verdict witness).
// regression: #7007
func TestVerifyflipQualifiedEnv(t *testing.T) {
	for _, tc := range qgcQualified {
		t.Run(tc.name, func(t *testing.T) {
			qgcSetenv(t, tc.env)
			o := vlOpts()
			o.evidence = "### Non-implementer run — VERIFY: BLOCKED — 2026-08-10\n\n" + vlOld(vfRow1, vlRow2) +
				"\n\n" + vlLatestRun()
			vlFlips(t, o)
		})
	}
}

// TestConfigCountOneOwner: no statusgen source outside tests spells
// GIT_CONFIG_COUNT except configCountVar's declaration. A reader that tests
// the count itself (presence, emptiness or "0") is the defect class #7007
// fixes: it declines the verifier's own git config env that gitcore
// qualifies, so it must ask configCountNative instead.
// regression: #7007
func TestConfigCountOneOwner(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	const decl = `const configCountVar = "GIT_CONFIG_COUNT"`
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, `"GIT_CONFIG_COUNT"`) && strings.TrimSpace(line) != decl {
				t.Errorf("%s:%d spells GIT_CONFIG_COUNT; read it through configCountNative", f, i+1)
			}
		}
	}
}
