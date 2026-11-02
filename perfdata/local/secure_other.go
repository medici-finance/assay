//go:build !unix

package local

import "os"

// Without O_NOFOLLOW the Lstat checks in openRegular are the only link guard.
const noFollow = 0

// Ownership and mode bits are not modeled here (Windows uses ACLs); the link and
// file-kind checks still apply.
func checkOwner(string, os.FileInfo) error { return nil }
