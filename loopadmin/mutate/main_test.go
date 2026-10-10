package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestMutateConfinesFile: a map entry can only name a file inside the scratch
// copy of the module.
func TestMutateConfinesFile(t *testing.T) {
	dir := t.TempDir()
	for _, bad := range []string{"", "/etc/hosts", "../outside.go", "runner/../../outside.go", ".."} {
		if got, err := confine(dir, bad); err == nil {
			t.Errorf("%q must be refused, resolved to %q", bad, got)
		}
	}
	// Positive control: a module-relative path resolves inside the copy.
	got, err := confine(dir, "runner/client.go")
	if err != nil || got != filepath.Join(dir, "runner", "client.go") {
		t.Fatalf("module path: %q %v", got, err)
	}
}

// The per-run GOCACHE quarantine: a mutation sweep inside a long-lived cell
// must not inject its never-reused build artifacts into the cell's ambient
// cache. These tests pin the env the per-mutant `go test` runs with.

func TestEnvWithGOCACHE_OverridesInherited(t *testing.T) {
	// An inherited GOCACHE must be REPLACED, not duplicated: if the ambient
	// value survived anywhere in the child env the quarantine would leak on
	// exactly the long-lived cells it exists to protect.
	env := []string{"PATH=/bin", "GOCACHE=/ambient/cache", "GOMODCACHE=/ambient/mod"}
	got := envWithGOCACHE(env, "/tmp/throwaway")
	seen := 0
	for _, e := range got {
		if e == "GOCACHE=/ambient/cache" {
			t.Fatalf("inherited GOCACHE survived in child env %q — the run would poison the ambient cache", got)
		}
		if e == "GOCACHE=/tmp/throwaway" {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("want exactly one GOCACHE=/tmp/throwaway entry, got %d in %q", seen, got)
	}
	// The caller's slice must not be rewritten in place.
	if env[1] != "GOCACHE=/ambient/cache" {
		t.Fatalf("envWithGOCACHE mutated its input: %q", env)
	}
}

func TestEnvWithGOCACHE_LeavesGOMODCACHEAlone(t *testing.T) {
	// Module downloads are immutable and shared-safe; pinning GOMODCACHE into
	// the throwaway dir would re-download the world per run for no gain.
	env := []string{"GOMODCACHE=/ambient/mod"}
	got := envWithGOCACHE(env, "/tmp/throwaway")
	found := false
	for _, e := range got {
		if strings.HasPrefix(e, "GOMODCACHE=") {
			if e != "GOMODCACHE=/ambient/mod" {
				t.Fatalf("GOMODCACHE was rewritten in child env %q — module cache must stay shared", got)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("GOMODCACHE dropped from child env %q", got)
	}
}
