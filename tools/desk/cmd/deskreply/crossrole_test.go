package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskreply posts as the worker App from any window (an intake or verify window replying
// on its own PR included), so under a foreign loop the in-process credential read admits the
// audited cross-role override (#6069) and under the worker's own loop it records none.
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
			withEnv(t, work)
			t.Setenv("DESK_LOOP", tc.loop)
			body := bodyFileWith(t, "posted as the worker App")
			if rc := run([]string{"example-org/tracker", "7", "--body-file", body}); rc != deskkit.ExitOK {
				t.Fatalf("reply rc = %d, want 0", rc)
			}
			entries, err := deskkit.LoadEntries()
			if err != nil {
				t.Fatal(err)
			}
			var rows int
			for _, e := range entries {
				if e.Tool == "desktoken" && strings.Contains(e.Detail, "deskreply posts as the worker App by design; loop "+tc.loop) {
					rows++
				}
			}
			if (rows > 0) != tc.override {
				t.Fatalf("loop %s: %d override audit rows, want override=%v", tc.loop, rows, tc.override)
			}
		})
	}
}
