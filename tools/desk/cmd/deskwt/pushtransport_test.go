package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// sshPushURL is a shaped, non-resolving SSH remote. It never has to work: the gate refuses
// before git is asked to reach it, and `add` does no network at all.
const sshPushURL = "ssh://git@example.invalid/example-org/tracker.git"

// TestAddSSHPushRemoteRefuses is #861's deskwt half. A worktree INHERITS the source
// checkout's remote, so an SSH push url here is an SSH push url in every worktree cut from
// it — and a bot session pushing over SSH goes out under a human's key while its commits
// read as the App's. Refusing at CREATE time costs nothing; refusing at push time costs the
// work already in the tree.
//
// FAIL-FIRST (the committed mutation entry): internal/deskkit/pushtransport-mutations.json
// carries "deskwt add no longer runs the push-transport gate", which removes the
// pushTransportGate call from cmdAdd. With it applied this test goes green-path — the
// worktree is created off an SSH-remoted checkout — and fails here.
func TestAddSSHPushRemoteRefuses(t *testing.T) {
	work := newRepo(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", sshPushURL)
	withEnv(t, work)
	t.Setenv("DESK_LOOP", "worker-desk") // this session acts as the worker App

	rc, stderr := runCapErr(t, []string{"add", "sshremote"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("add on an SSH push remote rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	for _, frag := range []string{"SSH transport", "worker App", "remote set-url --push origin"} {
		if !strings.Contains(stderr, frag) {
			t.Errorf("refusal does not carry %q:\n%s", frag, stderr)
		}
	}
	if _, serr := os.Lstat(filepath.Join(tmpBaseDir, "tracker-sshremote")); !os.IsNotExist(serr) {
		t.Errorf("a worktree was created on the refusal path (stat err = %v)", serr)
	}
}

// TestAddSSHRemoteWithoutBotIdentityStillAdds is the boundary the gate must not cross. A
// human at a terminal has no $DESK_LOOP, pushes under their own key, and an SSH remote is
// exactly what that key is for. Gating them would make the tool unusable for the case it
// was never about.
func TestAddSSHRemoteWithoutBotIdentityStillAdds(t *testing.T) {
	work := newRepo(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", sshPushURL)
	withEnv(t, work)
	t.Setenv("DESK_LOOP", "")

	rc, stderr := runCapErr(t, []string{"add", "humanwt"})
	if rc != deskkit.ExitOK {
		t.Fatalf("add with no loop identity rc = %d, want 0 (the gate is inert); stderr:\n%s", rc, stderr)
	}
	if strings.Contains(stderr, "SSH transport") || strings.Contains(stderr, "NOTICE $DESK_LOOP") {
		t.Errorf("the gate spoke for a session that claims no App identity:\n%s", stderr)
	}
}

// TestAddSSHFetchURLWithHTTPSPushURLAdds: fetch over SSH stays allowed — only the push
// transport is gated. The fixture's local bare origin stands in for the https push url's
// destination; what matters is that the SSH value sits on `remote.origin.url` and the gate
// reads `pushurl` instead.
func TestAddSSHFetchURLWithHTTPSPushURLAdds(t *testing.T) {
	work := newRepo(t)
	bare := originBare(t, work)
	mustGit(t, work, "remote", "set-url", "origin", "git@example.invalid:example-org/tracker.git")
	mustGit(t, work, "remote", "set-url", "--push", "origin", bare)
	withEnv(t, work)
	t.Setenv("DESK_LOOP", "worker-desk")

	rc, stderr := runCapErr(t, []string{"add", "fetchssh"})
	if rc != deskkit.ExitOK {
		t.Fatalf("SSH FETCH url with a non-SSH push url rc = %d, want 0 — only the push transport is gated; stderr:\n%s", rc, stderr)
	}
}
