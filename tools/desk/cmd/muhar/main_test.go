package main

import (
	"runtime"
	"strings"
	"testing"
)

// The muhar OOM (a v0.22.0 release blocker): the release's `test (muhar-light)`
// leg OOMKilled at 4Gi (21 min) AND at 8Gi (40 min). Doubling the memory limit
// only doubled survival time — the signature of unbounded RSS growth, not a
// one-off spike. Each in-flight worker holds its OWN full copy of the module
// tree (Harness.Workspace / copyTree), so peak RSS scales with mutations-in-
// flight. release.yml invokes `muhar -j 0`, and `-j 0` used to map straight to
// runtime.NumCPU() (4 on the release pool → 4 tree-copies resident at once).
// resolveJobs now caps that auto-sized pool at maxAutoJobs, bounding peak RSS by
// construction whatever the core count. These tests pin that cap.

func TestResolveJobs_AutoIsCappedByMaxAutoJobs(t *testing.T) {
	// -j 0 on a box with more CPUs than the cap must NOT scale with the CPU
	// count — that unbounded "one per CPU" mapping is exactly what OOMKilled the
	// muhar-light leg. On the unfixed code (resolveJobs returning numCPU) this
	// fails for every numCPU > maxAutoJobs.
	for _, numCPU := range []int{3, 4, 8, 16, 64} {
		if got := resolveJobs(0, numCPU); got > maxAutoJobs {
			t.Fatalf("resolveJobs(0, %d) = %d — the auto-sized pool exceeds the memory cap maxAutoJobs=%d; peak RSS is unbounded again (the muhar OOM)", numCPU, got, maxAutoJobs)
		}
	}
	// With the cap in force a many-core box resolves to exactly the cap.
	if got := resolveJobs(0, 8); got != maxAutoJobs {
		t.Fatalf("resolveJobs(0, 8) = %d, want the cap %d", got, maxAutoJobs)
	}
}

func TestResolveJobs_FewerCPUsThanCapAreNotInflated(t *testing.T) {
	// The cap is a ceiling, not a floor: a single-CPU box still resolves to 1,
	// never up to 2. Bounding concurrency must never manufacture parallelism.
	if got := resolveJobs(0, 1); got != 1 {
		t.Fatalf("resolveJobs(0, 1) = %d, want 1 — the cap must not inflate a small box", got)
	}
}

func TestResolveJobs_ExplicitRequestPassesThrough(t *testing.T) {
	// An explicit -j N is the operator's own memory/time trade-off; the cap
	// governs only the -j 0 auto-size. (main() separately rejects N < 1.)
	for _, n := range []int{1, 2, 3, 8} {
		if got := resolveJobs(n, runtime.NumCPU()); got != n {
			t.Fatalf("resolveJobs(%d, _) = %d, want %d — an explicit -j must pass through unchanged", n, got, n)
		}
	}
	// A negative request passes straight through to main's `-j must be >= 0`
	// rejection; resolveJobs must not silently rewrite it into the cap.
	if got := resolveJobs(-1, 8); got != -1 {
		t.Fatalf("resolveJobs(-1, 8) = %d, want -1 — a bad request must reach main's validation, not be masked by the cap", got)
	}
}

// The per-run GOCACHE quarantine: a mutation sweep inside a long-lived cell
// must not inject its never-reused build artifacts into the cell's ambient
// cache. These tests pin the env the suite command runs with.

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
