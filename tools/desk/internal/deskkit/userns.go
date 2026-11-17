package deskkit

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// The kernel files that say how this process's user namespace maps ids to its parent's.
const (
	procUIDMap = "/proc/self/uid_map"
	procGIDMap = "/proc/self/gid_map"
)

// identityIDMap is the mapping the initial user namespace carries: every id maps to itself.
const identityIDMap = "0 0 4294967295"

// userNSIdentity is the gate every hotfix owner check passes before it trusts a file owner. A
// file owner is read as this process's user namespace maps it. In any namespace but the
// initial one, uid 0 can be the caller itself (`unshare --map-root-user` maps the caller's uid
// to 0), so a caller-owned file reads as root-owned. The check therefore holds only when both
// /proc/self/uid_map and /proc/self/gid_map show the identity mapping; anything else is
// could-not-check, never a pass and never an ownership verdict. On Linux both files always
// exist, so a missing or unreadable one is could-not-check too. On a platform without them
// (goos other than linux, the file absent) there is no such namespace and nil is returned.
func userNSIdentity(read func(string) ([]byte, error), goos string) error {
	for _, p := range []string{procUIDMap, procGIDMap} {
		b, err := read(p)
		if err != nil {
			if goos != "linux" && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("could-not-check: %s unreadable, so no file owner can be trusted: %v", p, err)
		}
		if !isIdentityIDMap(string(b)) {
			return fmt.Errorf("could-not-check: %s is %q, not the initial user namespace's identity mapping %q, so a file owner read here may be a remapped caller",
				p, strings.Join(strings.Fields(string(b)), " "), identityIDMap)
		}
	}
	return nil
}

// isIdentityIDMap: exactly one extent, inside 0, outside 0, every one of the 2^32-1 ids.
func isIdentityIDMap(s string) bool {
	return strings.Join(strings.Fields(s), " ") == identityIDMap
}
