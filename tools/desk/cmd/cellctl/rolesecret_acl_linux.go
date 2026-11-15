//go:build linux

package main

import (
	"errors"

	"golang.org/x/sys/unix"
)

// probeACL reports whether path (not followed) carries a POSIX or NFSv4 access-control list, or
// a default list on a directory. Any list at all counts, even one equal to the mode bits: the
// check accepts owner-only mode bits as the only grant. A read that fails for any other reason
// is an error, which refuses the binding.
func probeACL(path string) (bool, error) {
	for _, attr := range []string{"system.posix_acl_access", "system.posix_acl_default", "system.nfs4_acl"} {
		n, err := unix.Lgetxattr(path, attr, nil)
		if err != nil {
			if errNoACLAttr(err) {
				continue
			}
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

// errNoACLAttr reports whether an extended-attribute read failed only because the attribute is
// absent or the filesystem keeps no such attribute: either way there is no list to grant read.
func errNoACLAttr(err error) bool {
	return errors.Is(err, unix.ENODATA) || errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP)
}
