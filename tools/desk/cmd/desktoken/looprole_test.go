package main

// looprole_test.go — role separation for the credential (#6069): a window whose $DESK_LOOP
// acts as one role is refused another loop's role, unless an audited --cross-role reason
// is given. Every case makes the role genuinely mintable first, so a refusal here is the
// loop check firing and not a missing key.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestMain clears the developer's own $DESK_LOOP: every pre-existing test here mints a role
// with no loop in mind, and a desk window running them must not turn them into refusals.
// A test that needs a loop sets it with t.Setenv.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("DESK_LOOP")
	os.Exit(m.Run())
}

// plantMintable makes role mint against a fake access_tokens endpoint with no cache, and
// returns the POST paths that endpoint saw.
func plantMintable(t *testing.T, home, role string) *[]string {
	t.Helper()
	prefix := roleEnvPrefix(role)
	t.Setenv(prefix+"_APP_ID", "12345")
	t.Setenv(prefix+"_INSTALL_ID", "100000006")
	writeFileMode(t, filepath.Join(home, ".config", "assay", appNameFor(role)+".pem"), makePEM(t), 0o600)
	srv, _, paths := makeTokenServer(t, "fixture-not-a-credential", "2124-01-01T00:00:00Z")
	t.Cleanup(srv.Close)
	old := httpClient
	httpClient = &http.Client{Transport: &rewriteTransport{orig: srv.URL}}
	t.Cleanup(func() { httpClient = old })
	return paths
}

// loopBound returns the roles some loop acts as, and each loop's role.
func loopBound() (roles []string, byLoop map[string]string) {
	byLoop = deskkit.LoopTokenRoles()
	seen := map[string]bool{}
	for _, r := range byLoop {
		if !seen[r] {
			seen[r] = true
			roles = append(roles, r)
		}
	}
	return roles, byLoop
}

// regression: #6069
func TestCrossRoleMintRefused(t *testing.T) {
	roles, byLoop := loopBound()
	for loop, own := range byLoop {
		for _, role := range roles {
			if role == own {
				continue
			}
			t.Run(loop+"/"+role, func(t *testing.T) {
				home := setupTest(t)
				posts := plantMintable(t, home, role)
				t.Setenv("DESK_LOOP", loop)
				rc, stdout, stderr := runCap(t, []string{role, "--repo", "example-org/tracker"})
				if rc != deskkit.ExitRefused || len(*posts) != 0 || stdout != "" {
					t.Fatalf("loop %s minting %s: rc=%d posts=%d stdout=%q; want refused before any mint\n%s",
						loop, role, rc, len(*posts), stdout, stderr)
				}
				for _, want := range []string{"#6069", loop, "--cross-role"} {
					if !strings.Contains(stderr, want) {
						t.Errorf("refusal missing %q: %s", want, stderr)
					}
				}
				es := auditEntries(t)
				if len(es) == 0 || es[len(es)-1].Result != deskkit.ResultRefused {
					t.Errorf("refusal must leave a refused audit row; got %+v", es)
				}
			})
		}
	}
}

// A retired loop name resolves to its successor, for the admission as for the stop flag.
// regression: #6069
func TestRetiredLoopRoleChecked(t *testing.T) {
	home := setupTest(t)
	plantMintable(t, home, "reviewer")
	t.Setenv("DESK_LOOP", "batch-fanout")
	if rc, _, stderr := runCap(t, []string{"reviewer"}); rc != deskkit.ExitRefused {
		t.Fatalf("batch-fanout minting reviewer: rc=%d, want refused\n%s", rc, stderr)
	}
}

// regression: #6069
func TestCrossRoleGitLabRefused(t *testing.T) {
	setupTest(t)
	t.Setenv("DESK_LOOP", "worker-desk")
	rc, _, stderr := runCap(t, []string{"--forge", "gitlab", "reviewer"})
	if rc != deskkit.ExitRefused || !strings.Contains(stderr, "#6069") {
		t.Fatalf("GitLab cross-role rotate: rc=%d; want the loop refusal before custody\n%s", rc, stderr)
	}
}

// A blank reason is no override: the refusal stands.
func TestCrossRoleBlankReason(t *testing.T) {
	home := setupTest(t)
	posts := plantMintable(t, home, "reviewer")
	t.Setenv("DESK_LOOP", "the-desk")
	rc, _, _ := runCap(t, []string{"reviewer", "--cross-role", "   "})
	if rc != deskkit.ExitRefused || len(*posts) != 0 {
		t.Fatalf("blank --cross-role: rc=%d posts=%d; want refused", rc, len(*posts))
	}
}

func TestOwnLoopRoleMints(t *testing.T) {
	_, byLoop := loopBound()
	byLoop["batch-fanout"] = "worker"
	byLoop["issue-loop"] = "issue-loop"
	for loop, own := range byLoop {
		t.Run(loop, func(t *testing.T) {
			home := setupTest(t)
			posts := plantMintable(t, home, own)
			t.Setenv("DESK_LOOP", loop)
			rc, stdout, stderr := runCap(t, []string{own, "--repo", "example-org/tracker"})
			if rc != deskkit.ExitOK || len(*posts) != 1 || !strings.Contains(stdout, own+"-token-") {
				t.Fatalf("loop %s minting its own %s: rc=%d posts=%d stdout=%q\n%s", loop, own, rc, len(*posts), stdout, stderr)
			}
		})
	}
}

// Roles no loop acts as are taken by name from any window, and an unset loop is not a
// desk window, so neither is checked.
func TestByNameRolesUnchecked(t *testing.T) {
	for _, tc := range []struct{ loop, role string }{
		{"worker-desk", "cell-issues"}, {"the-desk", "release-runner"},
		{"verify-desk", "auditor"}, {"intake-desk", "intake-loop"},
		{"", "reviewer"}, {"", "desk"},
	} {
		t.Run(tc.loop+"/"+tc.role, func(t *testing.T) {
			home := setupTest(t)
			posts := plantMintable(t, home, tc.role)
			t.Setenv("DESK_LOOP", tc.loop)
			rc, _, stderr := runCap(t, []string{tc.role})
			if rc != deskkit.ExitOK || len(*posts) != 1 {
				t.Fatalf("loop %q minting %s: rc=%d posts=%d\n%s", tc.loop, tc.role, rc, len(*posts), stderr)
			}
		})
	}
}

// The override admits the mint and is recorded on every use, a cache reuse included —
// the one row #1035 otherwise suppresses.
func TestCrossRoleOverrideAudited(t *testing.T) {
	home := setupTest(t)
	posts := plantMintable(t, home, "reviewer")
	t.Setenv("DESK_LOOP", "the-desk")
	args := []string{"reviewer", "--repo", "example-org/tracker", "--cross-role", "fixture reason"}
	for i := 1; i <= 2; i++ {
		rc, _, stderr := runCap(t, args)
		if rc != deskkit.ExitOK {
			t.Fatalf("override mint %d: rc=%d\n%s", i, rc, stderr)
		}
		var rows int
		for _, e := range auditEntries(t) {
			if e.Tool == "desktoken" && e.Result == deskkit.ResultOK &&
				strings.Contains(e.Detail, "cross-role override: loop=the-desk acts-as=desk role=reviewer") &&
				strings.Contains(e.Detail, `reason="fixture reason"`) {
				rows++
			}
		}
		if rows != i {
			t.Fatalf("after override %d: %d override rows, want %d: %+v", i, rows, i, auditEntries(t))
		}
	}
	if len(*posts) != 1 {
		t.Fatalf("second override should reuse the cache; posts=%d", len(*posts))
	}
}
