package main

import (
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #2202
// regression: #3146
func TestCommentCallerRole(t *testing.T) {
	for _, loop := range deskkit.KnownLoopNames() {
		canonical, known := deskkit.CanonicalLoopName(loop)
		if !known {
			t.Fatalf("known loop %q has no canonical identity", loop)
		}
		role, hasRole := deskkit.TokenRoleForLoop(canonical)
		if hasRole && role == "reviewer" {
			continue // The existing comment success tests cover the reviewer role.
		}
		for _, kind := range []string{"issue", "mr", "auto"} {
			t.Run(loop+"/"+kind, func(t *testing.T) {
				f, errBuf := setupFake(t)
				t.Setenv("DESK_LOOP", loop)
				f.issueNums[21] = kind == "issue"
				bf := writeBody(t, "comment.md", "An observation from the caller.")
				args := commentArgs(exampleRepo, "21", bf)
				if kind != "auto" {
					args = append(args, "--kind", kind)
				}
				code := run(args)
				if code != deskkit.ExitRefused || f.postedCmt != 0 || len(f.hits) != 0 {
					t.Fatalf("caller %s: exit=%d comments=%d network=%d; want refused before mint/read/write",
						loop, code, f.postedCmt, len(f.hits))
				}
				for _, want := range []string{"reviewer", "deskfile attach", "deskreply", "own PR"} {
					if !strings.Contains(errBuf.String(), want) {
						t.Errorf("missing guidance %q: %s", want, errBuf.String())
					}
				}
				if e := lastAudit(t); e.Result != deskkit.ResultRefused || e.BodyDigest == "" {
					t.Errorf("refusal audit = %+v", e)
				}
			})
		}
	}
}

// A planted second caller invokes the shared comment path without the CLI's loop
// check. Admission must be owned by that path, not merely by command dispatch.
// regression: #2202
// regression: #3146
func TestCommentRoleBypass(t *testing.T) {
	for _, tc := range []struct {
		loop string
		code int
	}{
		{"", deskkit.ExitRefused},
		{"unknown-loop", deskkit.ExitUnverifiable},
		{"worker-desk", deskkit.ExitRefused},
	} {
		t.Run(tc.loop, func(t *testing.T) {
			f, errBuf := setupFake(t)
			t.Setenv("DESK_LOOP", tc.loop)
			code := runComment("example-org", "tracker", 1, "", nil,
				[]byte("A direct caller's observation."), []string{"comment"}, postOpts{})
			if code != tc.code || f.postedCmt != 0 || len(f.hits) != 0 {
				t.Fatalf("direct caller: exit=%d comments=%d network=%d; want exit=%d and zero contact",
					code, f.postedCmt, len(f.hits), tc.code)
			}
			if !strings.Contains(errBuf.String(), "DESK_LOOP") || !strings.Contains(errBuf.String(), "worker") {
				t.Fatalf("missing role diagnostic: %s", errBuf.String())
			}
		})
	}
}
