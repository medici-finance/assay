package main

// worktreerole_test.go — deskdispatch's worktree-create step hands `deskwt add` the DISPATCHED
// agent's role (#861), so deskwt replaces the transport the worktree would inherit from the
// shared checkout (an SSH origin, an operator's pushurl sentinel) with that role App's own
// worktree-scoped https transport and credential helper.
//
// FAIL-FIRST: before the change step 2 ran `deskwt add <name> --branch|--detach … --base …`
// with no --role, so both assertions below failed (no `--role` on either argv).

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

func TestWorktreeCreatePassesTheDispatchedRole(t *testing.T) {
	for _, tc := range []struct {
		kit, want string
	}{
		{"", "--role worker"},
		{"verifier", "--role verifier"},
	} {
		t.Run("kit="+tc.kit, func(t *testing.T) {
			s := &stub{}
			_, root := s.install(t)
			plantScripts(t, root)
			s.replies = happyReplies("/private/tmp/role-home")
			args := []string{"example-stream/07", "--root", root, "--prompt-file", filepath.Join(t.TempDir(), "p.md")}
			if tc.kit != "" {
				args = append(args, "--kit", tc.kit)
			}
			if rc := run(args); rc != deskkit.ExitOK {
				t.Fatalf("dispatch rc = %d, want 0", rc)
			}
			found := false
			for _, c := range s.deskwtCalls() {
				if strings.Contains(c, "deskwt add ") && strings.HasSuffix(strings.TrimSpace(c), tc.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("`deskwt add` did not carry %q; deskwt calls: %v", tc.want, s.deskwtCalls())
			}
		})
	}
}
