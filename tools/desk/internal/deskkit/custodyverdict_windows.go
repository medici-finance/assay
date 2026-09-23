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
	fi, err := os.Stat(path)
	if err != nil {
		return CustodyVerdict{State: CustodyInconclusive,
			Err: fmt.Errorf("cannot read the metadata of custody file %s: %w", path, err)}
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
