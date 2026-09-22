package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// TestAddWithoutRoleClearsIdentitySoItNeverInherits pins the no-inherit floor (#1490): a
// `deskwt add` with NO --role must leave the new worktree with an EMPTY worktree-scoped
// commit identity, shadowing the shared checkout's, so a commit there fails closed rather
// than silently committing under the identity the shared checkout happened to carry.
//
// FAIL-FIRST: before the fix `deskwt add` set nothing, so the worktree INHERITED the shared
// user.name ("Test", set by newRepo) and the two assertions below both flipped — the
// effective user.name read back "Test", and the commit SUCCEEDED under it. That inherited
// identity is exactly the misattribution this closes.
func TestAddWithoutRoleClearsIdentitySoItNeverInherits(t *testing.T) {
	work := newRepo(t) // sets shared user.name=Test, user.email=t@e.st
	withEnv(t, work)

	if rc := run([]string{"add", "noinherit"}); rc != deskkit.ExitOK {
		t.Fatalf("add rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-noinherit")

	// The EFFECTIVE identity (worktree scope shadowing shared) is empty — the shared "Test"
	// is masked, not inherited. `--get` returns an empty value at rc 0 because the key is
	// DEFINED (empty) at worktree scope; an --unset would instead fall through to "Test".
	if got := mustGit(t, target, "config", "--get", "user.name"); got != "" {
		t.Fatalf("effective user.name in a no-role worktree = %q, want empty (the shared identity must be shadowed, not inherited)", got)
	}
	if got := mustGit(t, target, "config", "--get", "user.email"); got != "" {
		t.Fatalf("effective user.email in a no-role worktree = %q, want empty", got)
	}

	// And a commit there FAILS closed rather than landing under an inherited identity.
	writeFile(t, filepath.Join(target, "x.txt"), "x\n")
	mustGit(t, target, "add", "x.txt")
	cmd := exec.Command("git", "-c", "commit.gpgsign=false", "commit", "-m", "should fail closed")
	cmd.Dir = target
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("a commit in a cleared-identity worktree SUCCEEDED; the no-inherit floor must make it fail closed. git said:\n%s", out)
	}
}

// TestAddWithRoleStampsThatRolesIdentity pins the other arm: `deskwt add --role verifier`
// stamps the verifier App's own commit identity into the new worktree, worktree-scoped, so a
// hand-created worktree carries the right identity instead of the cleared floor. The role is
// accepted in either vocabulary (token role or loop name), folded like role-init.
//
// FAIL-FIRST: before the fix `--role` did not exist, so the flag was refused (exit 5) and no
// identity was ever stamped.
func TestAddWithRoleStampsThatRolesIdentity(t *testing.T) {
	work := newRepo(t)
	withEnv(t, work)

	if rc := run([]string{"add", "withrole", "--role", "verifier"}); rc != deskkit.ExitOK {
		t.Fatalf("add --role verifier rc = %d, want 0", rc)
	}
	target := filepath.Join(tmpBaseDir, "tracker-withrole")
	if got := mustGit(t, target, "config", "--worktree", "--get", "user.email"); got != verifierBotEmail {
		t.Fatalf("worktree-scoped user.email = %q, want the verifier binding %q", got, verifierBotEmail)
	}
	if got := mustGit(t, target, "config", "--worktree", "--get", "user.name"); got != "assay-verifier-app[bot]" {
		t.Fatalf("worktree-scoped user.name = %q, want %q", got, "assay-verifier-app[bot]")
	}
	// The loop-name spelling folds to the same role.
	if rc := run([]string{"add", "withloop", "--role", "verify-desk"}); rc != deskkit.ExitOK {
		t.Fatalf("add --role verify-desk (loop spelling) rc = %d, want 0", rc)
	}
	if got := mustGit(t, filepath.Join(tmpBaseDir, "tracker-withloop"), "config", "--worktree", "--get", "user.email"); got != verifierBotEmail {
		t.Fatalf("loop-spelled --role stamped user.email = %q, want %q", got, verifierBotEmail)
	}
}

// TestAddWithUnboundRoleRefusesBeforeCreating proves the fail-closed refusal: a --role whose
// roster entry has no derivable commit identity is refused (exit 5) BEFORE any worktree is
// created, naming the roster key — never a worktree left inheriting an unrelated identity.
func TestAddWithUnboundRoleRefusesBeforeCreating(t *testing.T) {
	work := newRepo(t)
	calls := withEnv(t, work)

	// Re-plant a roster that binds every role EXCEPT verifier, so its identity is unbound.
	unbound := strings.ReplaceAll(fixtureRoster, "verifier=assay-verifier-app:300000005,", "")
	home := os.Getenv("HOME")
	if err := os.WriteFile(filepath.Join(home, ".config", "assay", "roster.env"), []byte(unbound), 0o600); err != nil {
		t.Fatalf("re-plant roster: %v", err)
	}
	deskkit.ReloadConfig()

	rc, stderr := runCapErr(t, []string{"add", "willrefuse", "--role", "verifier"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("add --role verifier with an unbound identity rc = %d, want 5 (refused); stderr: %s", rc, stderr)
	}
	if !contains(stderr, "ASSAY_TRUSTED_BOT_SLUGS") {
		t.Fatalf("refusal must name the roster key to pin the identity; stderr: %s", stderr)
	}
	if hasWorktreeVerb(*calls, "add") {
		t.Fatalf("a worktree was created despite the fail-closed refusal; git calls: %v", gitCalls(*calls))
	}
}
