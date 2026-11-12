package deskkit

// writeverbloop_test.go — a write verb's in-process credential read keeps the loop-role
// check the forked desktoken applied (#6069): a window takes only its own loop's role.
// deskreply and deskevidence post as a fixed App by design, so for them a refusal is
// admitted and recorded as an audited override; any other verb stays refused.

import (
	"strings"
	"testing"
)

// regression: #6069
func TestWriteVerbCredLoopRole(t *testing.T) {
	for _, tc := range []struct {
		verb, role, loop string
		refused          bool
		audited          bool
	}{
		{"deskreply", "worker", "worker-desk", false, false},
		{"deskreply", "worker", "intake-desk", false, true},
		{"deskreply", "worker", "verify-desk", false, true},
		{"deskevidence", "verifier", "verify-desk", false, false},
		{"deskevidence", "verifier", "the-desk", false, true},
		{"deskpr", "worker", "pr-review-desk", true, false},
		{"deskclose", "worker", "the-desk", true, false},
		{"deskreply", "verifier", "the-desk", true, false},
		{"deskreply", "worker", "no-such-loop", true, false},
	} {
		t.Run(tc.verb+"/"+tc.loop, func(t *testing.T) {
			setup(t)
			repo := ForgeRepo{Owner: "example-org", Name: "tracker"}
			rosterWithForge(t, repo.Slug(), "github")
			minterDetector(t, plantAppToken(t, "app-cred-stub"))
			t.Setenv(loopEnv, tc.loop)

			cred, err := WriteVerbRoleCredential(tc.verb, tc.role, repo.Slug())
			if tc.refused {
				if err == nil || cred.Token != "" {
					t.Fatalf("%s as loop %s took the %s credential (%v); want a refusal", tc.verb, tc.loop, tc.role, err)
				}
				return
			}
			if err != nil || cred.Token != "app-cred-stub" {
				t.Fatalf("%s as loop %s: err=%v; want the credential", tc.verb, tc.loop, err)
			}
			entries, lerr := LoadEntries()
			if lerr != nil {
				t.Fatal(lerr)
			}
			var rows int
			for _, e := range entries {
				if e.Tool == "desktoken" && strings.Contains(e.Detail, "cross-role override: loop="+tc.loop) &&
					strings.Contains(e.Detail, "role="+tc.role) &&
					strings.Contains(e.Detail, tc.verb+" posts as the "+tc.role+" App by design") {
					rows++
				}
			}
			want := 0
			if tc.audited {
				want = 1
			}
			if rows != want {
				t.Fatalf("%d override audit rows, want %d: %+v", rows, want, entries)
			}
		})
	}
}
