package main

import (
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gitversion"
)

// requireGitListReset skips a test that drives the role App transport (`deskwt add --role`,
// `deskwt role-init`) on a git older than 2.46.
//
// The transport resets the inherited multi-valued remote.origin.url / remote.origin.pushurl
// lists with a worktree-scoped EMPTY entry, and only git 2.46 or later treats that entry as a
// list reset (transport.go). On an older git the read-back still sees the inherited URL, so
// every role-transport add is refused — a success-path test then fails, and a refusal-path
// test passes for the wrong reason. Neither proves anything there, so the test is skipped
// with the reason named; on git 2.46 or later it runs in full.
func requireGitListReset(t *testing.T) {
	t.Helper()
	gitversion.RequireGit(t, 2, 46,
		"the empty-entry reset of the remote.origin.url/pushurl lists that the role App transport writes")
}
