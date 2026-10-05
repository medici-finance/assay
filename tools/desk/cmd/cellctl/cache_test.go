//go:build darwin || linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheLaunchEnvironment(t *testing.T) {
	c := codexEnvironmentCell(t)
	if err := os.MkdirAll(c.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	c.Env.Put("CELL_GO_CACHE", "on")
	c.Env.Put("CELL_GO_CACHE_MIN_FREE", "0")
	env := c.cacheEnv([]string{"GOCACHE=/foreign", "GOMODCACHE=/foreign-mod", "GH_TOKEN=secret"})
	values, err := c.codexCommandEnvironment(env)
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.cachePolicy()
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range p.Env() {
		k, v, _ := strings.Cut(kv, "=")
		if values[k] != v {
			t.Fatalf("Codex loses %s across HOME change", k)
		}
	}
	if _, ok := values["GH_TOKEN"]; ok {
		t.Fatal("ambient secret copied")
	}
	ce := c.scrubbedComposeEnv("worker-desk", "go", "fixture")
	for _, kv := range p.Env() {
		found := false
		for _, got := range ce.Pairs {
			if got == kv {
				found = true
			}
		}
		if !found {
			t.Fatal("scrubbed launch lost", kv)
		}
	}
	if _, err = os.Stat(filepath.Join(p.Root, "owner")); !os.IsNotExist(err) {
		t.Fatal("composition mutated cache")
	}
	c.Env.Put("CELL_GO_CACHE", "off")
	if strings.Contains(strings.Join(c.cacheEnv(env), "\n"), "ASSAY_GO_CACHE_POLICY=") {
		t.Fatal("foreign ambient policy survived opt-out")
	}
}

func TestCacheBootHeldBeforeFetch(t *testing.T) {
	c := codexEnvironmentCell(t)
	c.Repo = t.TempDir() // intentionally not a Git repository; never a real remote
	if err := os.Mkdir(filepath.Join(c.Repo, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(c.Dir, 0700); err != nil {
		t.Fatal(err)
	}
	c.Env.Put("CELL_GO_CACHE", "on")
	c.Env.Put("CELL_GO_CACHE_MIN_FREE", "18446744073709551615")
	defer func() {
		r := recover()
		code, ok := r.(exitCode)
		if !ok || code.code != 6 {
			t.Fatalf("boot bypassed prefetch storage hold: %v", r)
		}
	}()
	// No repo or harness is configured: touching either first is a different
	// failure. The storage gate must exit 6 before those operations are possible.
	c.deskLaunch("worker-desk", "", "", "", "", "", "", "", Provider{}, "", false, nil, nil, cockpitResolution{})
	t.Fatal("boot continued despite unavailable storage floor")
}
