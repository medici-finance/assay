package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

const verifierWorktreeEmail = "300000005+assay-verifier-app[bot]@users.noreply.github.com"

// (ddGit — real-git test helper — is shared from worktreedryrun_test.go.)

// TestVerifierDispatchStampsWorktreeIdentityNotTheSharedCheckouts is the headline #1490
// regression: a verifier dispatched from a shared checkout whose config carries an UNRELATED
// (desk) commit identity must land in a worktree whose OWN commit identity is the verifier's,
// not the desk identity it would otherwise inherit — the identity statusgen verifyrun would
// otherwise stamp into every Evidence witness Runner cell.
//
// It runs a REAL dispatch through the fake claim/forge seams, but passes `git config` through
// to REAL git against a REAL worktree, so the stamp actually writes and the assertion reads
// the worktree's true committer identity.
//
// FAIL-FIRST: before the fix the worktree-create step stamped nothing, so the worktree
// inherited the shared checkout's `assay-desk-app[bot]` identity and this assertion read that
// desk email back instead of the verifier's — a red run demonstrable by stashing the fix.
func TestVerifierDispatchStampsWorktreeIdentityNotTheSharedCheckouts(t *testing.T) {
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)

	// A REAL shared checkout carrying an UNRELATED (dispatcher/desk) identity in its shared
	// config — the bug's precondition: a worktree cut from it inherits this.
	shared := t.TempDir()
	ddGit(t, "", "init", "-b", "main", shared)
	ddGit(t, shared, "config", "user.name", "assay-desk-app[bot]")
	ddGit(t, shared, "config", "user.email", "300000001+assay-desk-app[bot]@users.noreply.github.com")
	ddGit(t, shared, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(shared, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ddGit(t, shared, "add", "README.md")
	ddGit(t, shared, "commit", "-m", "init")

	// A REAL detached worktree — the verifier's home shape — cut from that shared checkout,
	// so its default (inherited) identity is the desk one above.
	wt := filepath.Join(t.TempDir(), "verify-home")
	ddGit(t, shared, "worktree", "add", "--detach", wt, "HEAD")

	// Stub: pass `git config` through to REAL git (so the stamp writes and is readable), return
	// the real worktree for `deskwt add`, and canned-reply everything else.
	old := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		joined := name + " " + strings.Join(args, " ")
		s.calls = append(s.calls, append([]string{name}, args...))
		if name == "git" && len(args) > 0 && args[0] == "config" {
			return exec.Command("git", args...)
		}
		for _, r := range happyReplies(wt) {
			if strings.Contains(joined, r.match) {
				return exec.Command("/bin/sh", "-c", "cat <<'STUBEOF'\n"+r.stdout+"\nSTUBEOF")
			}
		}
		return exec.Command("/bin/sh", "-c", "exit 0")
	}
	t.Cleanup(func() { execCommand = old })

	rc := run([]string{"verdict-lane--05", "--root", root, "--kit", "verifier",
		"--tier", "strong", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})
	if rc != deskkit.ExitOK {
		t.Fatalf("verifier dispatch rc = %d, want 0", rc)
	}

	// The worktree's OWN (worktree-scoped) identity is the verifier's.
	if got := ddGit(t, wt, "config", "--worktree", "--get", "user.email"); got != verifierWorktreeEmail {
		t.Fatalf("worktree-scoped user.email = %q, want the verifier binding %q (not the inherited desk identity)", got, verifierWorktreeEmail)
	}
	if got := ddGit(t, wt, "config", "--worktree", "--get", "user.name"); got != "assay-verifier-app[bot]" {
		t.Fatalf("worktree-scoped user.name = %q, want %q", got, "assay-verifier-app[bot]")
	}
	// The EFFECTIVE identity (worktree scope shadowing the shared config) is the verifier's —
	// the inherited desk identity is gone.
	if eff := ddGit(t, wt, "config", "--get", "user.email"); eff != verifierWorktreeEmail {
		t.Fatalf("effective user.email in the dispatched worktree = %q, want %q — the shared desk identity leaked through", eff, verifierWorktreeEmail)
	}
}

// TestVerifierKitUnboundIdentityRefusedPreClaim proves the pre-claim refusal (#1490): a
// --kit whose role has no roster commit identity is refused (exit 5) naming the kit, the role
// and the roster key, BEFORE any durable state — never a worktree left to inherit an
// unrelated identity.
func TestVerifierKitUnboundIdentityRefusedPreClaim(t *testing.T) {
	s := &stub{}
	home, root := s.install(t)
	plantScripts(t, root)

	// Re-plant a roster that binds every role EXCEPT verifier.
	unbound := strings.ReplaceAll(fixtureRoster, "verifier=assay-verifier-app:300000005,", "")
	if err := os.WriteFile(filepath.Join(home, ".config", "assay", "roster.env"), []byte(unbound), 0o600); err != nil {
		t.Fatalf("re-plant roster: %v", err)
	}
	deskkit.ReloadConfig()

	s.replies = happyReplies("/private/tmp/worker-home")
	rc, stderr := runCapturingStderr(t, []string{"verdict-lane--05", "--root", root,
		"--kit", "verifier", "--tier", "strong", "--prompt-file", filepath.Join(t.TempDir(), "p.md")})

	if rc != deskkit.ExitRefused {
		t.Fatalf("unbound verifier identity rc = %d, want 5 (refused); stderr:\n%s", rc, stderr)
	}
	for _, want := range []string{"verifier", "ASSAY_TRUSTED_BOT_SLUGS"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("refusal must name %q (kit/role + roster key); stderr:\n%s", want, stderr)
		}
	}
	// Pre-claim: nothing durable was created.
	if s.ran("dispatch-claim.sh acquire") {
		t.Fatal("a claim was acquired despite the pre-claim identity refusal — it wedges the item")
	}
	if s.ran("deskwt add") {
		t.Fatal("a worktree was created despite the pre-claim identity refusal")
	}
}
