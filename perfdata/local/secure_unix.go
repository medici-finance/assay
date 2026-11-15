//go:build unix

package local

import (
	"fmt"
	"os"
	"syscall"
)

// checkOwner refuses a directory owned by another user or open to group or other users.
func checkOwner(p string, fi os.FileInfo) error {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Geteuid() {
		return unsafeEntry(p, fmt.Sprintf("owned by uid %d, not this user", st.Uid))
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		return unsafeEntry(p, fmt.Sprintf("mode %04o allows group or other access; want 0700", perm))
	}
	return nil
}

// checkStateDir refuses a state directory in which another user could replace the
// store directory: one owned by someone other than this user or the superuser, or
// writable by group or other users without the sticky bit.
func checkStateDir(p string, fi os.FileInfo) error {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Geteuid() && st.Uid != 0 {
		return unsafeEntry(p, fmt.Sprintf("state directory owned by uid %d, not this user", st.Uid))
	}
	if perm := fi.Mode().Perm(); perm&0o022 != 0 && fi.Mode()&os.ModeSticky == 0 {
		return unsafeEntry(p, fmt.Sprintf("state directory mode %04o is writable by group or other users", perm))
	}
	return nil
}
