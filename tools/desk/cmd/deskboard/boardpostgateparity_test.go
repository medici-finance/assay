package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestBoardAndPostGateAgreeOnAuthorTrust — Verify row 3 (brief 17, #808): the
// board's PR classifier and `deskpost`'s trustGate answer the SAME question — "may
// this PR enter the review loop?" — and must give the SAME answer for the same
// author. This is the standing guard against the two predicates drifting apart
// again (the failure mode this whole brief exists to close): for a table covering
// every login class deskpost distinguishes (role App, mapped human, trusted shared
// account, unlisted account, empty login), the board's admission decision
// (whether the PR lands with an ACTION row rather than in EXTERNAL / UNBLESSED)
// must equal `deskkit.TrustedAuthorID(login, 0)` — the same call, with the same
// id==0 fallback, `deskpost`'s trustGate makes on a surface (the gh CLI PR list)
// that carries no numeric author id.
func TestBoardAndPostGateAgreeOnAuthorTrust(t *testing.T) {
	cases := []struct {
		name  string
		login string
	}{
		{"role App ([bot] rendering)", "app/assay-desk-app"},
		{"mapped human (ASSAY_HUMAN_LOGIN_MAP)", "ada"},
		{"trusted shared automation account", "shared-agent"},
		{"unlisted account", "some-fork-account"},
		{"empty login", ""},
	}

	for _, repo := range []string{pubRepoFixture, privRepoFixture} {
		for _, tc := range cases {
			t.Run(repo+"/"+tc.name, func(t *testing.T) {
				installFakeGH(t)
				t.Setenv("DESKBOARD_GH_PR_REPO", repo)
				t.Setenv("DESKBOARD_GH_PRLIST_JSON", publicAuthorGatePRJSON(tc.login))

				var out, errb bytes.Buffer
				if code := run([]string{"actions"}, &out, &errb); code != deskkit.ExitOK {
					t.Fatalf("run(actions) = exit %d, stderr=%s", code, errb.String())
				}
				var rep actionsReport
				if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
					t.Fatalf("parsing actions JSON: %v\n%s", err, out.String())
				}

				admitted := true
				for _, e := range rep.External {
					if e.Number == 7 && e.Author == tc.login {
						admitted = false
					}
				}

				want := deskkit.TrustedAuthorID(tc.login, 0)
				if admitted != want {
					t.Errorf("board admitted=%v for login %q on %s, deskpost trustGate (TrustedAuthorID(%q,0))=%v — the two predicates diverged",
						admitted, tc.login, repo, tc.login, want)
				}
			})
		}
	}
}
