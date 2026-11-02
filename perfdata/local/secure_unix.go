//go:build unix

package local

import (
	"fmt"
	"os"
	"syscall"
)

// noFollow makes an open fail on a final symbolic link instead of following it.
const noFollow = syscall.O_NOFOLLOW

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
