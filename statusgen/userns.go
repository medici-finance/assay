package main

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// sgProcGIDMap is the gid half of the mapping sgProcUIDMap (hotfixself.go) holds for uids.
const sgProcGIDMap = "/proc/self/gid_map"

// sgIdentityIDMap is the mapping the initial user namespace carries: every id maps to itself.
const sgIdentityIDMap = "0 0 4294967295"

// sgErrOwnerUnverified marks every refusal of the namespace gate: no file owner could be
// verified, which is could-not-check, never a finding that a file is not root-owned.
var sgErrOwnerUnverified = errors.New("could-not-check")

// sgOwnerReason is the refusal reason for an owner-check error: sgReasonOwnerUnverified when
// the namespace gate refused before any owner was read, else sgReasonNotRootOwned.
func sgOwnerReason(err error) string {
	if errors.Is(err, sgErrOwnerUnverified) {
		return sgReasonOwnerUnverified
	}
	return sgReasonNotRootOwned
}

// sgUserNSIdentity is the gate every root-ownership check passes before it trusts a file
// owner. A file owner is read as this process's user namespace maps it. In any namespace but
// the initial one, uid 0 can be the caller itself (`unshare --map-root-user` maps the caller's
// uid to 0), so a caller-owned file reads as root-owned. The check therefore holds only when
// both /proc/self/uid_map and /proc/self/gid_map show the identity mapping; anything else is
// could-not-check, never a pass and never an ownership verdict. On Linux a missing or
// unreadable file is could-not-check too: a kernel built without user namespaces has neither
// file, and there the gate refuses rather than guess. Off Linux (goos other than linux, the
// file absent) there is no such namespace and nil is returned.
func sgUserNSIdentity(read func(string) ([]byte, error), goos string) error {
	for _, p := range []string{sgProcUIDMap, sgProcGIDMap} {
		b, err := read(p)
		if err != nil {
			if goos != "linux" && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("%w: %s unreadable, so no file owner can be trusted: %v", sgErrOwnerUnverified, p, err)
		}
		if !sgIsIdentityIDMap(string(b)) {
			return fmt.Errorf("%w: %s is %q, not the initial user namespace's identity mapping %q, so a file owner read here may be a remapped caller",
				sgErrOwnerUnverified, p, strings.Join(strings.Fields(string(b)), " "), sgIdentityIDMap)
		}
	}
	return nil
}

// sgIsIdentityIDMap: exactly one extent, inside 0, outside 0, every one of the 2^32-1 ids.
func sgIsIdentityIDMap(s string) bool {
	return strings.Join(strings.Fields(s), " ") == sgIdentityIDMap
}
