//go:build unix

package deskkit

import (
	"fmt"
	"os"
	"syscall"
)

// checkFileOwner enforces that the roster config is owned by the user running the
// tool — a co-located unprivileged process must not be able to plant or edit the
// roster without already holding the user's own write access. Unix variant: reads
// the owning uid from the stat result.
//
// These are deskkit's OWN error strings, moved verbatim from checkOwnerPerms; they
// are deliberately NOT converged with statusgen's cross-tree copy. The
// group/world-writable MODE check that precedes the extraction stays in
// rosterconfig.go and runs on both platforms.
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
