//go:build unix

package deskkit

import (
	"fmt"
	"os"
)

// VerifyCustodyOwnerOnly enforces, on unix, that a token custody file is mode 0600
// exactly — owner read/write and NOTHING for group or world. This is the POSIX half
// of the custody permission guarantee; the Windows half (custodyowner_windows.go)
// enforces the same owner-only intent through the file's ACL via evaluateCustodyACL,
// because os.FileMode's permission bits are synthetic on Windows (a normal file
// reads 0666) and the 0600 test there rejects a correctly ACL-locked file — the
// exact #667 failure. Behind the same OS boundary as the roster owner check
// (rosterowner_{unix,windows}.go), so the unix contract is unchanged: this is the
// same `fi.Mode().Perm() != 0o600` test the GitLab custody paths ran inline before.
//
// fi must be the os.Stat of path. On unix it carries the mode bits this check reads;
// on Windows it is unused and the ACL is read from path instead.
func VerifyCustodyOwnerOnly(path string, fi os.FileInfo) error {
	if perm := fi.Mode().Perm(); perm != 0o600 {
		return fmt.Errorf("custody token file at %s has permissions %04o; must be 0600 — run: chmod 600 %s",
			path, perm, path)
	}
	return nil
}
