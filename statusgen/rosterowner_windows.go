//go:build windows

package main

import (
	"fmt"
	"os"
	"sync"
)

// checkFileOwner is the windows variant of the roster owner check.
//
// DEGRADATION — stated out loud, never a silent success. Windows has no POSIX uid
// to compare against (ownership there is an ACL question that needs an entirely
// different check), so the uid-equality enforcement the unix variant performs
// cannot run here. Rather than quietly return nil — the exact failure mode this
// split exists to prevent — it prints one NOTICE line naming the path and saying
// the owner check is skipped, so the operator knows the roster's trust on windows
// rests on the group/world-writable MODE check in scanCheckOwnerPerms alone (that
// check runs on both platforms).
//
// Closing this gap on windows would mean an ACL owner check (GetSecurityInfo /
// GetNamedSecurityInfo comparing the file owner SID to the process token's user
// SID); that is out of scope for this compile-target split.
func checkFileOwner(path string, _ os.FileInfo) error {
	rosterOwnerSkipNotice(path)
	return nil
}

// rosterOwnerNoticed tracks paths already NOTICE'd, so the line prints once per
// path even when scanCheckOwnerPerms is called hot.
var rosterOwnerNoticed sync.Map

func rosterOwnerSkipNotice(path string) {
	if _, seen := rosterOwnerNoticed.LoadOrStore(path, struct{}{}); seen {
		return
	}
	fmt.Fprintf(os.Stderr, "NOTICE: roster owner check skipped for %s — windows has no uid "+
		"to compare (ownership is an ACL question needing a different check); the roster's "+
		"trust rests on the group/world-writable mode check alone\n", path)
}
