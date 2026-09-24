//go:build windows

package deskkit

import (
	"fmt"
	"os"
)

// ClassifyCustodyOwnerOnly is the three-state twin of VerifyCustodyOwnerOnly for a caller
// that must tell "definitely not owner-only" from "could not tell" (see custodyverdict.go).
//
// It reads the file's owner and DACL through the SAME windowsFileACLModel adapter the
// read-side check uses and hands the model to classifyCustodyModel, whose decision is
// evaluateCustodyACL's. An access list that cannot be read at all — the "this filesystem
// cannot report it" case — is Inconclusive.
func ClassifyCustodyOwnerOnly(path string) CustodyVerdict {
	// Lstat, not Stat: the path read back is the regular file the write path has just
	// renamed into place, so a link found there was swapped in after the rename. It is
	// refused, never judged through (the custody-link rule of custodylink.go).
	fi, err := os.Lstat(path)
	if err != nil {
		return CustodyVerdict{State: CustodyInconclusive,
			Err: fmt.Errorf("cannot read the metadata of custody file %s: %w", path, err)}
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return CustodyVerdict{State: CustodyRefused,
			Err: fmt.Errorf("custody path %s is a symbolic link, not the regular file this run wrote", path)}
	}
	if !fi.Mode().IsRegular() {
		return CustodyVerdict{State: CustodyRefused,
			Err: fmt.Errorf("custody path %s is not a regular file", path)}
	}
	model, err := windowsFileACLModel(path)
	if err != nil {
		return CustodyVerdict{State: CustodyInconclusive, Err: err}
	}
	return classifyCustodyModel(path, model)
}
