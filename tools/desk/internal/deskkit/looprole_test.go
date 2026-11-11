package deskkit

import (
	"strings"
	"testing"
)

// regression: #6069
func TestRequireLoopRoleTable(t *testing.T) {
	for _, tc := range []struct {
		loop, role string
		code       int // 0 = admitted
	}{
		{"the-desk", "desk", 0},
		{"pr-review-desk", "reviewer", 0},
		{"batch-fanout", "worker", 0},
		{"issue-loop", "issue-loop", 0},
		{"", "reviewer", 0},
		{"the-desk", "cell-issues", 0},
		{"worker-desk", "release-runner", 0},
		{"the-desk", "reviewer", ExitRefused},
		{"worker-desk", "reviewer", ExitRefused},
		{"pr-review-desk", "worker", ExitRefused},
		{"intake-desk", "verifier", ExitRefused},
		{"batch-fanout", "desk", ExitRefused},
		{"no-such-loop", "reviewer", ExitUnverifiable},
	} {
		t.Run(tc.loop+"/"+tc.role, func(t *testing.T) {
			t.Setenv(loopEnv, tc.loop)
			_, _, err := RequireLoopRole("test", tc.role)
			got := 0
			if err != nil {
				got = ExitCodeOf(err)
			}
			if got != tc.code {
				t.Fatalf("loop %q role %q: exit %d, want %d (%v)", tc.loop, tc.role, got, tc.code, err)
			}
			reason := CrossRoleReason("verb", tc.role)
			if (reason != "") != (tc.code == ExitRefused) {
				t.Fatalf("CrossRoleReason = %q for exit %d; only a refusal earns an override", reason, tc.code)
			}
			if err != nil && tc.code == ExitRefused && !strings.Contains(err.Error(), "--cross-role") {
				t.Fatalf("refusal names no remedy: %v", err)
			}
		})
	}
}

// CrossRoleReasonFor names an override exactly when the named loop's own check would refuse,
// without reading this process's $DESK_LOOP.
func TestCrossRoleReasonForLoop(t *testing.T) {
	t.Setenv(loopEnv, "pr-review-desk") // must not leak into the answer
	for _, tc := range []struct {
		loop, role string
		want       bool
	}{
		{"worker-desk", "desk", true},
		{"the-desk", "verifier", true},
		{"batch-fanout", "desk", true},
		{"worker-desk", "worker", false},
		{"the-desk", "desk", false},
		{"", "desk", false},
		{"no-such-loop", "desk", false},
		{"worker-desk", "cell-issues", false},
	} {
		got := CrossRoleReasonFor(tc.loop, "printed command", tc.role)
		if (got != "") != tc.want {
			t.Errorf("loop %q role %q: reason %q, want override=%v", tc.loop, tc.role, got, tc.want)
		}
	}
	if got := CrossRoleReasonFor("batch-fanout", "v", "desk"); !strings.Contains(got, "loop worker-desk acts as worker") {
		t.Errorf("retired loop not canonicalised in the reason: %q", got)
	}
}
