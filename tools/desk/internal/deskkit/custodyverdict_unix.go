//go:build unix

package deskkit

import (
	"fmt"
	"os"
)

// ClassifyCustodyOwnerOnly is the three-state twin of VerifyCustodyOwnerOnly for a caller
// that must tell "definitely not owner-only" from "could not tell" (see custodyverdict.go).
//
// On unix the decision is VerifyCustodyOwnerOnly's, unchanged: the mode bits are real, so a
// file that can be stat'd always yields a definite answer — 0600 is Verified, anything else
// is Refused. The only inconclusive case is a file whose metadata cannot be read at all, or
// a path that is not a regular file the check can reason about.
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
	if verr := VerifyCustodyOwnerOnly(path, fi); verr != nil {
		return CustodyVerdict{State: CustodyRefused, Err: verr}
	}
	return CustodyVerdict{State: CustodyVerified}
}
