package main

import (
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// regression: #3232
func TestCreateRemoteCheck(t *testing.T) {
	for _, state := range []string{"published", "new role", "new foreign", "unreadable", "divergent", "custody refusal", "budget exhausted", "conflicting modes"} {
		t.Run(state, func(t *testing.T) {
			work, calls, tip := evidenceFixture(t)
			bare := filepath.Join(filepath.Dir(work), "origin.git")
			switch state {
			case "budget exhausted":
				seedDeskprAudit(t, deskkit.RateLimitPerPRPerHour, createAuditShape)
			case "custody refusal":
				forgeForFn = func(string) (deskkit.Forge, deskkit.ForgeRepo, error) {
					return nil, deskkit.ForgeRepo{}, deskkit.Refused("fixture custody unavailable")
				}

			case "new role":
				addCommitAs(t, work, "Assay Worker", prWorkerEmail, "new.txt")
			case "new foreign":
				addCommitAs(t, work, "Other App", prIssueLoopEmail, "foreign.txt")
				mustGit(t, work, "update-ref", "refs/remotes/origin/feature/test-branch", "HEAD")
			case "unreadable":
				mustGit(t, work, "remote", "set-url", "--push", "origin", "file://"+filepath.Join(t.TempDir(), "missing.git"))
			case "divergent":
				mustGit(t, work, "checkout", "-b", "side", tip)
				addCommitAs(t, work, "Assay Worker", prWorkerEmail, "side.txt")
				mustGit(t, work, "push", "origin", "HEAD:refs/heads/feature/test-branch")
				mustGit(t, work, "checkout", "feature/test-branch")
			}
			before := remoteRef(t, bare, "refs/heads/feature/test-branch")
			args := []string{"create", "--title", "check published Evidence", "--body-min", "Issue: #3232\nCheck existing Evidence.", "--check-remote"}
			if state == "conflicting modes" {
				args = append(args, "--check")
			}
			rc := run(args)
			want := deskkit.ExitOK
			if state == "new foreign" || state == "divergent" || state == "custody refusal" || state == "conflicting modes" {
				want = deskkit.ExitRefused
			}
			if state == "unreadable" {
				want = deskkit.ExitUnverifiable
			}
			if rc != want {
				t.Fatalf("remote check rc=%d want %d", rc, want)
			}
			if anyPush(*calls) || curForge.created != nil {
				t.Fatal("preflight caused an outward write")
			}
			if remoteRef(t, bare, "refs/heads/feature/test-branch") != before {
				t.Fatal("preflight moved destination")
			}
		})
	}
}
