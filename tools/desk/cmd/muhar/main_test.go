package main

import (
	"runtime"
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
