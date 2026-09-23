package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// RESUME BASE — the start point a dispatch hands `deskwt add`.
//
// A resume dispatch (`--pr <N>`: an ALREADY-OPEN change whose branch exists on the forge)
// must cut its worktree from THAT BRANCH's remote tip, never from the mainline. Cutting
// from the mainline produced a worktree whose branch sat at main's tip while the change's
// own commits existed only on `refs/remotes/origin/<branch>` — a resuming agent that did
// not compare its HEAD against the change's reported head either lost the existing work or
// produced a diff that read as a full rewrite.
//
// A FRESH dispatch (no `--pr`) is unchanged: the mainline is its start point.

// deskwtAddArgv returns the recorded `deskwt add …` argv, joined, or "" when none ran.
func deskwtAddArgv(s *stub) string {
	for _, c := range s.calls {
		j := strings.Join(c, " ")
		if strings.Contains(j, "deskwt add") {
			return j
		}
	}
	return ""
}

// resumeDispatch runs one dispatch through the stub and returns the `deskwt add` argv it
// built. remoteBranchSHA != "" makes the branch's own remote-tracking ref resolve, which is
// what a real resume looks like; "" makes it unresolvable.
func resumeDispatch(t *testing.T, remoteBranchSHA string, args ...string) (*stub, string) {
	t.Helper()
	s := &stub{}
	_, root := s.install(t)
	plantScripts(t, root)
	s.replies = []reply{
		{match: "remote get-url origin", stdout: "git@github.com:medici-finance/assay.git"},
		{match: "rev-parse --verify --quiet refs/remotes/origin/feat/item-1", stdout: remoteBranchSHA},
		{match: "deskwt add", stdout: filepath.Join(t.TempDir(), "home")},
	}
	full := append([]string{"item-1", "--root", root,
		"--prompt-file", filepath.Join(t.TempDir(), "p.md")}, args...)
	_, stderr := runCapturingStderr(t, full)
	argv := deskwtAddArgv(s)
	if argv == "" {
		t.Fatalf("no `deskwt add` ran; stderr:\n%s", stderr)
	}
	return s, argv
}

// (a) A resume onto a branch that exists on the remote cuts from THAT branch's ref.
func TestResumeDispatchCutsFromTheChangesOwnRemoteBranch(t *testing.T) {
	const sha = "1111111111111111111111111111111111111111"
	s, argv := resumeDispatch(t, sha, "--pr", "42")
	if !strings.Contains(argv, "--base refs/remotes/origin/feat/item-1") {
		t.Errorf("a resume dispatch did not cut from the change's own remote branch:\nargv: %s", argv)
	}
	if strings.Contains(argv, "--base refs/remotes/origin/main") {
		t.Errorf("a resume dispatch cut from the MAINLINE — the worktree lands at main's tip, not the "+
			"change's head, so the resuming agent starts from the wrong commit:\nargv: %s", argv)
	}
	// The branch's remote tip is refreshed before it is read: a remote-tracking ref this
	// checkout never fetched is not the change's real tip either.
	if !s.ran("fetch") {
		t.Error("the resume did not refresh the change's branch before cutting from its remote-tracking ref")
	}
}

// (b) A resume whose branch does NOT exist on the remote falls back to the mainline rather
// than handing `deskwt` a ref that does not resolve.
func TestResumeWithNoRemoteBranchFallsBackToTheMainline(t *testing.T) {
	_, argv := resumeDispatch(t, "", "--pr", "42")
	if !strings.Contains(argv, "--base refs/remotes/origin/main") {
		t.Errorf("a resume with no remote branch did not fall back to the mainline:\nargv: %s", argv)
	}
}

// (d) REGRESSION GUARD: a fresh dispatch (no --pr) still cuts from the mainline, and does
// not probe or fetch any branch ref to decide it.
func TestFreshDispatchStillCutsFromTheMainline(t *testing.T) {
	const sha = "1111111111111111111111111111111111111111"
	s, argv := resumeDispatch(t, sha)
	if !strings.Contains(argv, "--base refs/remotes/origin/main") {
		t.Errorf("a fresh dispatch no longer cuts from the mainline:\nargv: %s", argv)
	}
	if s.ran("fetch") {
		t.Error("a fresh dispatch fetched a branch ref it has no use for")
	}
}

// A READ-ONLY lane (review, verifier) carries --pr as the change it is reading, not as a
// branch to resume: it checks the change's head out itself, as a detached HEAD, so its
// worktree start point stays the mainline.
func TestReadOnlyLanesWithAPRStillCutFromTheMainline(t *testing.T) {
	const sha = "1111111111111111111111111111111111111111"
	for _, kit := range []string{"review", "verifier"} {
		t.Run(kit, func(t *testing.T) {
			_, argv := resumeDispatch(t, sha, "--pr", "42", "--kit", kit)
			if !strings.Contains(argv, "--base refs/remotes/origin/main") {
				t.Errorf("the %s lane cut from a branch ref instead of the mainline:\nargv: %s", kit, argv)
			}
		})
	}
}
