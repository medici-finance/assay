package main

import (
	"os"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/cellcache"
)

// fixtureProcessEnv keeps subprocess fixtures independent from an operator's
// managed-cache lease. Fixture-specific values are applied after the ambient
// environment has been copied.
func fixtureProcessEnv(extra ...string) []string {
	env := envUnset(append([]string(nil), os.Environ()...), cellcache.EnvPolicy)
	for _, kv := range extra {
		key, value, ok := strings.Cut(kv, "=")
		if ok {
			env = envSet(env, key, value)
		}
	}
	return env
}

// regression: #2781
func TestFixtureNoCachePolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	policy := cellcache.Policy{Root: root, Domain: strings.Repeat("a", 64), Budget: 4096}
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, kv := range policy.Env() {
		key, value, _ := strings.Cut(kv, "=")
		if key == cellcache.EnvPolicy {
			t.Setenv(cellcache.EnvPolicy, value)
		}
	}

	env := fixtureProcessEnv("CELL_FIXTURE=child")
	got, err := cellcache.FromEnv(env)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("fixture inherited managed cache policy for %q; child could acquire its lease", got.Root)
	}
	if !containsEnv(env, "CELL_FIXTURE", "child") {
		t.Fatal("fixture-specific environment was not applied")
	}
}

func containsEnv(env []string, key, value string) bool {
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k == key && v == value {
			return true
		}
	}
	return false
}
