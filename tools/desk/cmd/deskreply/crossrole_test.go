package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskreply posts as the worker App from any window (an intake or verify window replying
// on its own PR included), so a foreign loop passes desktoken's audited override and the
// worker's own loop passes none.
// regression: #6069
func TestReplyMintCrossRole(t *testing.T) {
	for _, tc := range []struct {
		loop     string
		override bool
	}{
		{"worker-desk", false}, {"intake-desk", true}, {"verify-desk", true},
	} {
		t.Run(tc.loop, func(t *testing.T) {
			work := newBaseFixture(t)
			calls := withEnv(t, work)
			t.Setenv("DESK_LOOP", tc.loop)
			body := bodyFileWith(t, "posted as the worker App")
			if rc := run([]string{"example-org/tracker", "7", "--body-file", body}); rc != deskkit.ExitOK {
				t.Fatalf("reply rc = %d, want 0", rc)
			}
			var mint []string
			for _, c := range *calls {
				if len(c) >= 2 && filepath.Base(c[0]) == "desktoken" && c[1] == "worker" {
					mint = c
				}
			}
			joined := strings.Join(mint, " ")
			if has := strings.Contains(joined, "--cross-role"); has != tc.override {
				t.Fatalf("loop %s: --cross-role present=%v, want %v: %q", tc.loop, has, tc.override, joined)
			}
			if tc.override && !strings.Contains(joined, "deskreply posts as the worker App by design; loop "+tc.loop) {
				t.Fatalf("override reason does not name the verb and loop: %q", joined)
			}
		})
	}
}
