package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// deskevidence lands Evidence as the verifier App from whichever window dispatched the
// verifier, so under a foreign loop the in-process credential read admits the audited
// cross-role override (#6069) and under the verifier's own loop it records none.
// regression: #6069
func TestVerifierMintCrossRole(t *testing.T) {
	for _, tc := range []struct {
		loop     string
		override bool
	}{
		{"verify-desk", false}, {"", false}, {"the-desk", true}, {"worker-desk", true},
	} {
		t.Run(tc.loop, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			plantFixtureRoster(t, home)
			t.Setenv("DESK_TOOLS_DISABLED", "")
			t.Setenv("CLAUDE_SESSION_ID", "test")
			t.Setenv("DESK_LOOP", tc.loop)

			cred := filepath.Join(t.TempDir(), "verifier-cred")
			if err := os.WriteFile(cred, []byte("fake-verifier-cred\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			restore := deskkit.SetRoleTokenMinter(func(role, owner string) (string, string, error) {
				return cred, "", nil
			})
			t.Cleanup(restore)

			if err := mintVerifierToken("example-org/tracker"); err != nil {
				t.Fatalf("mintVerifierToken as loop %q: %v", tc.loop, err)
			}
			entries, err := deskkit.LoadEntries()
			if err != nil {
				t.Fatal(err)
			}
			var rows int
			for _, e := range entries {
				if e.Tool == "desktoken" && strings.Contains(e.Detail, "deskevidence posts as the verifier App by design; loop "+tc.loop) {
					rows++
				}
			}
			if (rows > 0) != tc.override {
				t.Fatalf("loop %q: %d override audit rows, want override=%v", tc.loop, rows, tc.override)
			}
		})
	}
}
