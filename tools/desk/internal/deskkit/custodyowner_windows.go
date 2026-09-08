//go:build windows

package deskkit

import "os"

// VerifyCustodyOwnerOnly is the Windows half of the custody permission guarantee.
// os.FileMode's permission bits are synthetic here — a normal file reads 0666 — so
// the unix 0600 test (custodyowner_unix.go) rejects a token file that is correctly
// locked down by an owner-only NTFS ACL (#667). Instead of the mode bits, it reads
// the file's real owner SID and DACL through the same windowsFileACLModel adapter
// the roster owner check uses (rosterowner_windows.go) and hands them to
// evaluateCustodyACL, which accepts an owner-only ACL and refuses any foreign
// write-capable principal or any permission it cannot establish.
//
// fi is unused on Windows: ownership here is an ACL question read from path, not a
// mode-bit question. The signature matches the unix variant so the callers are
// platform-agnostic.
func VerifyCustodyOwnerOnly(path string, _ os.FileInfo) error {
	model, err := windowsFileACLModel(path)
	if err != nil {
		return err
	}
	return evaluateCustodyACL(path, model)
}
