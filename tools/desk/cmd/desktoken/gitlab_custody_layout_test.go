package main

// gitlab_custody_layout_test.go — the custody LAYOUT a rotation has to preserve (#1112).
//
// The documented GitLab layout (docs/adopting-assay-gitlab.md §2) provisions each role's PAT
// as <prefix>-<role>-bot.token and LINKS gitlab-<role>.token at it, because the provisioning
// script names files after the service account while the desk verbs resolve
// gitlab-<role>.token. A rotation that renamed over the LINK PATH replaced the link with a
// regular file and left the provisioned file holding the invalidated token — so re-running the
// documented link step (it is written to be idempotent) re-pointed custody at a dead
// credential, and the very next API read 401'd with nothing in the rotate path having failed.
//
// These tests pin the layout contract: the link survives a rotation, and no pre-rotation value
// is left readable anywhere the custody path can resolve to.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// linkedCustody provisions the DOCUMENTED layout for one role: the provisioned bot-token file
// holding tok, plus a relative symlink at gitlab-<role>.token pointing at it (exactly the
// `ln -s "<prefix>-$r-bot.token" "gitlab-$r.token"` the adopter runbook prescribes). It returns
// the custody path (the link) and the provisioned target path.
func linkedCustody(t *testing.T, homeDir, role, tok string) (custody, target string) {
	t.Helper()
	custody = gitlabTokenPath(homeDir, role)
	dir := filepath.Dir(custody)
	target = filepath.Join(dir, "example-"+role+"-bot.token")
	writeTokenCache(t, target, tok)
	if err := os.Symlink(filepath.Base(target), custody); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	return custody, target
}

// TestGitLabRotatePreservesCustodySymlink is the layout guard: after a successful rotation the
// custody path is STILL a symlink, and the file it points at holds the rotated token 0600.
//
// Fail-first: with the rotation renaming over the link path, the custody path is a regular file
// after the rotate and this fails on the "custody path is no longer a symlink" assertion.
func TestGitLabRotatePreservesCustodySymlink(t *testing.T) {
	homeDir := setupTest(t)
	custody, target := linkedCustody(t, homeDir, "worker", glOldWorker)

	valid := glOldWorker
	srv, calls := makeRotateServer(t, &valid, glNewWorker, "2124-01-08T00:00:00Z")
	defer srv.Close()
	pointHTTPClientAt(t, srv)

	rc, stdout, stderr := runCap(t, []string{"--forge", "gitlab", "worker"})
	if rc != deskkit.ExitOK {
		t.Fatalf("rotate rc = %d, want 0; stderr: %s", rc, stderr)
	}
	if *calls != 1 {
		t.Fatalf("rotate endpoint hit %d times, want 1", *calls)
	}
	assertNoTokenLeak(t, stdout+stderr)

	fi, err := os.Lstat(custody)
	if err != nil {
		t.Fatalf("lstat custody: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("custody path %s is no longer a symlink after the rotation (mode %s) — "+
			"the rotation replaced the provisioned layout with a regular file", custody, fi.Mode())
	}

	// The link must resolve to the rotated value, and the target must still be owner-only.
	got, err := os.ReadFile(custody)
	if err != nil {
		t.Fatalf("read custody through the link: %v", err)
	}
	if string(got) != glNewWorker {
		t.Fatalf("custody path resolves to %q, want the rotated token", string(got))
	}
	tfi, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat link target: %v", err)
	}
	if tfi.Mode().Perm() != 0o600 {
		t.Fatalf("link target mode = %o, want 0600", tfi.Mode().Perm())
	}
}

// TestGitLabRotateLeavesNoPreRotationTokenBehindTheLink reproduces the reported symptom: a read
// that follows a successful rotation presents the PRE-rotation token.
//
// The sequence is the adopter's, not a contrivance: provision the linked layout, rotate, then
// re-run the runbook's own link step (written to be idempotent, and re-run by any re-issue or
// re-provisioning pass). Before the fix the rotation had replaced the link with a regular file
// and the provisioned target still held the invalidated token, so re-linking pointed custody
// straight back at the dead credential and this test reads the OLD token after a rotation the
// audit trail records as "ok" — the intermittent first-read 401.
//
// Fail-first: asserts the custody path reads the ROTATED token after the re-link; against the
// unfixed rotate path it reads glOldWorker.
func TestGitLabRotateLeavesNoPreRotationTokenBehindTheLink(t *testing.T) {
	homeDir := setupTest(t)
	custody, target := linkedCustody(t, homeDir, "worker", glOldWorker)

	valid := glOldWorker
	srv, _ := makeRotateServer(t, &valid, glNewWorker, "2124-01-08T00:00:00Z")
	defer srv.Close()
	pointHTTPClientAt(t, srv)

	if rc, _, stderr := runCap(t, []string{"--forge", "gitlab", "worker"}); rc != deskkit.ExitOK {
		t.Fatalf("rotate rc = %d, want 0; stderr: %s", rc, stderr)
	}

	// Re-run the runbook's idempotent link step (`ln -sf <bot-token> gitlab-<role>.token`),
	// then take the first read a verb would take.
	if err := os.Remove(custody); err != nil {
		t.Fatalf("remove custody for re-link: %v", err)
	}
	if err := os.Symlink(filepath.Base(target), custody); err != nil {
		t.Fatalf("re-link custody: %v", err)
	}
	got, err := os.ReadFile(custody)
	if err != nil {
		t.Fatalf("read custody after re-link: %v", err)
	}
	if string(got) != glNewWorker {
		t.Fatalf("the first read after the rotation presents the PRE-ROTATION token "+
			"(read %q, want the rotated value) — a rotation must leave no stale value reachable "+
			"through the custody path", redactToken(string(got)))
	}

	// The provisioned file the layout links at must itself hold the live credential. If it
	// still holds the pre-rotation value, every path that reaches it — including a re-link —
	// serves a token the rotation already invalidated.
	if tgot, err := os.ReadFile(target); err != nil {
		t.Fatalf("read link target: %v", err)
	} else if string(tgot) == glOldWorker {
		t.Fatalf("the provisioned custody file %s still holds the PRE-ROTATION token after a "+
			"successful rotation — the rotation wrote past the link instead of through it", target)
	}
}

// redactToken keeps a failure message readable without printing a token-shaped value: the
// assertions in this file compare values, so the message only needs to say WHICH of the two
// fixtures came back, never the value itself.
func redactToken(v string) string {
	switch v {
	case glOldWorker:
		return "<pre-rotation token>"
	case glNewWorker:
		return "<rotated token>"
	case "":
		return "<empty>"
	default:
		return "<unrecognised value>"
	}
}

// TestGitLabCustodyWriteTargetResolvesLinks pins the resolver the rotation uses to decide WHERE
// its bytes land: a regular file is its own target and reports unlinked; a symlink resolves to
// the file it points at and reports linked, so the caller knows to assert the link survived.
func TestGitLabCustodyWriteTargetResolvesLinks(t *testing.T) {
	dir := t.TempDir()

	plain := filepath.Join(dir, "gitlab-plain.token")
	writeTokenCache(t, plain, glOldWorker)
	tgt, linked, err := gitlabCustodyWriteTarget(plain)
	if err != nil {
		t.Fatalf("regular-file custody: %v", err)
	}
	if linked {
		t.Fatal("a regular-file custody must not be reported as linked")
	}
	if tgt != plain {
		t.Fatalf("regular-file custody target = %q, want %q", tgt, plain)
	}

	real := filepath.Join(dir, "example-worker-bot.token")
	writeTokenCache(t, real, glOldWorker)
	link := filepath.Join(dir, "gitlab-worker.token")
	if err := os.Symlink(filepath.Base(real), link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}
	tgt, linked, err = gitlabCustodyWriteTarget(link)
	if err != nil {
		t.Fatalf("symlink custody: %v", err)
	}
	if !linked {
		t.Fatal("a symlink custody must be reported as linked")
	}
	// EvalSymlinks also resolves the temp dir itself, so compare resolved forms.
	wantReal, rerr := filepath.EvalSymlinks(real)
	if rerr != nil {
		t.Fatalf("resolve want: %v", rerr)
	}
	if tgt != wantReal {
		t.Fatalf("symlink custody target = %q, want %q", tgt, wantReal)
	}
}
