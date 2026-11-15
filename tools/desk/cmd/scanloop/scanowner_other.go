//go:build !unix

package main

import "os"

// checkScanWorktreeOwner has no POSIX uid to compare off unix. The default scan worktree base there
// is <repo-root>/.claude/worktrees, not a shared tmp base, and the points-back check in
// assertRegisteredWorktree still applies.
func checkScanWorktreeOwner(os.FileInfo) error { return nil }
