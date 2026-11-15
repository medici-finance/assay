package main

import (
	"os/exec"
	"strings"
	"testing"
)

// deskevidence lands Evidence as the verifier App from whichever window dispatched the
// verifier, so a foreign loop passes desktoken's audited override and the verifier's own
// loop passes none. The child is stubbed to fail: only the argv is under test.
// regression: #6069
func TestVerifierMintCrossRole(t *testing.T) {
	for _, tc := range []struct {
		loop     string
		override bool
	}{
		{"verify-desk", false}, {"", false}, {"the-desk", true}, {"worker-desk", true},
	} {
		t.Run(tc.loop, func(t *testing.T) {
			t.Setenv("DESK_LOOP", tc.loop)
			var got []string
			old := execCommand
			execCommand = func(name string, args ...string) *exec.Cmd {
				got = append([]string{name}, args...)
				return exec.Command("false")
			}
			t.Cleanup(func() { execCommand = old })
			_ = mintVerifierToken("example-org/tracker")
			joined := strings.Join(got, " ")
			if !strings.HasPrefix(joined, "desktoken verifier --repo example-org/tracker") {
				t.Fatalf("argv = %q", joined)
			}
			has := strings.Contains(joined, "--cross-role")
			if has != tc.override {
				t.Fatalf("loop %q: --cross-role present=%v, want %v: %q", tc.loop, has, tc.override, joined)
			}
			if tc.override && !strings.Contains(joined, "deskevidence posts as the verifier App by design; loop "+tc.loop) {
				t.Fatalf("override reason does not name the verb and loop: %q", joined)
			}
		})
	}
}
