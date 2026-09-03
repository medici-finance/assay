//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
)

// checkFileOwner enforces that the roster config is owned by the user running the
// tool — a file another account owns must not be trusted to name the accounts
// these tools trust. Unix variant: reads the owning uid from the stat result.
//
// This is today's behaviour and today's two error strings, verbatim from the block
// it replaced in scanCheckOwnerPerms. The group/world-writable MODE check that
// precedes the extraction stays in rosterconfig.go and runs on both platforms.
func checkFileOwner(path string, fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot determine the owner of %s — refusing to read a roster "+
			"whose ownership cannot be established", path)
	}
	if uid := os.Getuid(); int(st.Uid) != uid {
		return fmt.Errorf("roster config %s is owned by uid %d, not by the invoking user (uid %d) — "+
			"refusing to take the trusted-identity list from a file this user does not own",
			path, st.Uid, uid)
	}
	return nil
}
