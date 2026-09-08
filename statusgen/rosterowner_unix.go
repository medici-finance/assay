//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
)

// checkFileOwner enforces the unix form of the sshd rule: the roster config must
// not be group- or world-writable (mode bits), and must be OWNED by the user
// running the tool (uid). A file another account owns, or one a co-located
// unprivileged process can write, must not be trusted to name the accounts these
// tools trust.
//
// The mode-bit check lived in scanCheckOwnerPerms until the Windows-ACL port: it
// moved HERE because os.FileMode's permission bits are synthetic on Windows (a
// normal file reads 0666), so a shared mode check fired spuriously there. Both
// halves — mode and uid — are unix's guarantee and both stay on the unix path;
// the Windows variant enforces the same intent through owner SID + DACL. The two
// error strings below are verbatim from the block scanCheckOwnerPerms used to run.
func checkFileOwner(path string, fi os.FileInfo) error {
	if mode := fi.Mode().Perm(); mode&0o022 != 0 {
		kind := "file"
		fix := "0600"
		if fi.IsDir() {
			kind = "directory"
			fix = "0700"
		}
		return fmt.Errorf("roster config %s %s is group- or world-writable (mode %04o): "+
			"anything that can write it can name the accounts this tool trusts. "+
			"Fix with `chmod %s %s`", kind, path, mode, fix, path)
	}
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
