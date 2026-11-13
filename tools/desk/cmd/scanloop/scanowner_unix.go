//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
)

// scanCurrentUID is the invoking user's uid. It is a package var ONLY so a test can stand in for a
// directory another account owns; production keeps os.Getuid, and it is not wired to an env var or
// a flag.
var scanCurrentUID = os.Getuid

// checkScanWorktreeOwner refuses a retained scan worktree directory the invoking user does not own.
// The scan worktrees sit in a shared tmp base, where any local account can create a directory at a
// name that was vacated; fi is os.Lstat's result for that directory, never a followed link.
func checkScanWorktreeOwner(fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("the owner of the directory cannot be read")
	}
	if uid := scanCurrentUID(); int(st.Uid) != uid {
		return fmt.Errorf("the directory is owned by uid %d, not by the invoking user (uid %d)", st.Uid, uid)
	}
	return nil
}
