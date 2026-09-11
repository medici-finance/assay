package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// sshPushURL is a shaped, non-resolving SSH remote. It never has to work: the gate refuses
// before git is ever asked to reach it.
const sshPushURL = "ssh://git@example.invalid/example-org/tracker.git"

// TestCreateSSHPushRemoteRefuses is #861's create half. The fixture's fetch url is
// untouched — only the PUSH url is SSH — so this also proves the gate reads the push url
// rather than `remote.origin.url`.
//
// FAIL-FIRST, observed via internal/deskkit/pushtransport-mutations.json's "deskpr create
// no longer runs the push-transport gate" entry. With the gate call removed, this case runs
// the whole create path and deskpr ATTEMPTS the SSH push for real:
//
//	git push failed: git push -u origin feature/test-branch: exit status 128
//	  (ssh: connect to host … port 22: Operation timed out)
//	--- FAIL: TestCreateSSHPushRemoteRefuses
//	    create over an SSH push remote rc = 6, want 5 (refused)
//
// That rc=6 is the fault itself, one layer short of succeeding: on a machine whose agent
// DOES hold a key for that host, the push lands and the forge records the human.
func TestCreateSSHPushRemoteRefuses(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", sshPushURL)
	calls := withEnv(t, work) // sets DESK_LOOP=worker-desk: this session acts as the worker App
	stderr := withStderrCapture(t)

	rc := run([]string{"create", "--title", "x", "--body-min", "y\nBrief: fixture/01"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("create over an SSH push remote rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	assertNoPushNoCreate(t, *calls)
	if !anyCall(gitCalls(*calls), "config", "--list", "-z") {
		t.Fatalf("the gate's config read never happened, so rc=5 came from some OTHER refusal: %v", gitCalls(*calls))
	}
	_ = stderr
}

// TestUpdateSSHPushRemoteRefuses is the update half. update pushes too, so it is gated on
// the same terms — and it must refuse BEFORE the token mint, so a mis-configured checkout
// costs no round trip.
func TestUpdateSSHPushRemoteRefuses(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", sshPushURL)
	calls := withEnv(t, work)

	rc := run([]string{"update"})
	if rc != deskkit.ExitRefused {
		t.Fatalf("update over an SSH push remote rc = %d, want %d (refused)", rc, deskkit.ExitRefused)
	}
	assertNoPushNoCreate(t, *calls)
	for _, c := range *calls {
		if len(c) > 0 && filepath.Base(c[0]) == "desktoken" {
			t.Fatalf("the transport refusal ran AFTER a token mint; it must refuse first: %v", *calls)
		}
	}
}

// TestSSHFetchHTTPSPushStillCreates is the other side of the contract, and the
// one a too-broad gate would break: fetch over SSH is ALLOWED. Only the push transport is
// gated, so an SSH fetch url with an https push override is a normal, successful create.
func TestSSHFetchHTTPSPushStillCreates(t *testing.T) {
	work := newBaseFixture(t)
	bare := mustGit(t, work, "remote", "get-url", "--push", "origin") // the offline file:// bare
	mustGit(t, work, "remote", "set-url", "origin", "git@example.invalid:example-org/tracker.git")
	mustGit(t, work, "remote", "set-url", "--push", "origin", bare)
	calls := withEnv(t, work)

	rc := run([]string{"create", "--title", "x", "--body-min", "y\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("SSH FETCH url with a non-SSH push url rc = %d, want 0 — only the push transport is gated", rc)
	}
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("expected the push to proceed; git calls: %v", gitCalls(*calls))
	}
}

// TestCreateHTTPSNoAppHelperNotices: https is the sanctioned transport, so
// this must NOT refuse — but an https push answered by nothing but the machine's ambient
// credential is the same ambient-identity shape one layer along, so it says so.
func TestCreateHTTPSNoAppHelperNotices(t *testing.T) {
	work := newBaseFixture(t)
	// An https push url that still routes to the offline bare: `insteadOf` rewrites it at
	// transport time, so the gate sees https and the push stays local and offline.
	bare := mustGit(t, work, "remote", "get-url", "--push", "origin")
	mustGit(t, work, "remote", "set-url", "--push", "origin", "https://example.com/example-org/tracker.git")
	mustGit(t, work, "config", "url."+bare+".insteadOf", "https://example.com/example-org/tracker.git")
	mustGit(t, work, "config", "credential.helper", "osxkeychain") // ambient, not the App's
	calls := withEnv(t, work)
	stderr := withStderrCapture(t)

	rc := run([]string{"create", "--title", "x", "--body-min", "y\nBrief: fixture/01"})
	if rc != deskkit.ExitOK {
		t.Fatalf("https push url rc = %d, want 0 — the missing App helper is a NOTICE, never a refusal", rc)
	}
	got := stderr.String()
	if !strings.Contains(got, "NOTICE") || !strings.Contains(got, "osxkeychain") {
		t.Fatalf("expected a NOTICE naming the ambient helper; stderr:\n%s", got)
	}
	if !anyCall(gitCalls(*calls), "push", "-u", "origin", "feature/test-branch") {
		t.Fatalf("a NOTICE must not stop the push; git calls: %v", gitCalls(*calls))
	}
}

// TestEditIsNotPushTransportGated pins the gate's scope: `edit` replaces a PR body and
// pushes NOTHING, so an SSH remote is none of its business. A gate wired into the shared
// preflight (where it would have been one line cheaper) would refuse here, and a worker with
// a legitimately SSH-remoted checkout would lose the body-correction verb for no custody
// reason at all.
//
// The assertion is on the gate's own git read rather than on the exit code: `edit` has its
// own refusals (no open PR for the branch, in this fixture), so an exit code alone cannot
// tell "refused for transport" from "refused for something else". The `git config --list -z`
// argv is the gate's unique fingerprint, and the recorder sees every argv this package
// builds.
func TestEditIsNotPushTransportGated(t *testing.T) {
	work := newBaseFixture(t)
	mustGit(t, work, "remote", "set-url", "--push", "origin", sshPushURL)
	calls := withEnv(t, work)

	body := filepath.Join(t.TempDir(), "body.md")
	writeFile(t, body, "replacement body\n\nBrief: fixture/01\n")
	run([]string{"edit", "--body-file", body})

	if anyCall(gitCalls(*calls), "config", "--list", "-z") {
		t.Fatalf("edit ran the push-transport gate, but it pushes nothing: %v", gitCalls(*calls))
	}
}
