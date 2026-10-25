//go:build unix

package deskkit

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// HotfixOwnedBy returns the owner check for owner uid: path (symlinks resolved) must be owned
// by uid and not group- or world-writable, and every parent directory must be owned by uid
// or root and not group- or world-writable unless sticky. Production passes uid 0
// (HotfixRootOwned); only a test seam passes the test user's uid.
func HotfixOwnedBy(uid uint32) HotfixOwnerCheck {
	return func(path string) error {
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("%s: %v", path, err)
		}
		fi, err := os.Lstat(real)
		if err != nil {
			return fmt.Errorf("%s: %v", real, err)
		}
		st, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("%s: owner unreadable", real)
		}
		if st.Uid != uid {
			return fmt.Errorf("%s is owned by uid %d, not %d", real, st.Uid, uid)
		}
		if fi.Mode().Perm()&0o022 != 0 {
			return fmt.Errorf("%s is group- or world-writable (%04o)", real, fi.Mode().Perm())
		}
		for dir := filepath.Dir(real); ; dir = filepath.Dir(dir) {
			di, err := os.Lstat(dir)
			if err != nil {
				return fmt.Errorf("%s: %v", dir, err)
			}
			dst, ok := di.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("%s: owner unreadable", dir)
			}
			if dst.Uid != uid && dst.Uid != 0 {
				return fmt.Errorf("parent %s is owned by uid %d, not %d or root", dir, dst.Uid, uid)
			}
			if di.Mode().Perm()&0o022 != 0 && di.Mode()&os.ModeSticky == 0 {
				return fmt.Errorf("parent %s is group- or world-writable (%04o)", dir, di.Mode().Perm())
			}
			if parent := filepath.Dir(dir); parent == dir {
				return nil
			}
		}
	}
}
