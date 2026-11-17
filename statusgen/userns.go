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

// sgUserNSIdentity is the gate every root-ownership check passes before it trusts a file
// owner. A file owner is read as this process's user namespace maps it. In any namespace but
// the initial one, uid 0 can be the caller itself (`unshare --map-root-user` maps the caller's
// uid to 0), so a caller-owned file reads as root-owned. The check therefore holds only when
// both /proc/self/uid_map and /proc/self/gid_map show the identity mapping; anything else is
// could-not-check, never a pass and never an ownership verdict. On Linux both files always
// exist, so a missing or unreadable one is could-not-check too. On a platform without them
// (goos other than linux, the file absent) there is no such namespace and nil is returned.
func sgUserNSIdentity(read func(string) ([]byte, error), goos string) error {
	for _, p := range []string{sgProcUIDMap, sgProcGIDMap} {
		b, err := read(p)
		if err != nil {
			if goos != "linux" && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("could-not-check: %s unreadable, so no file owner can be trusted: %v", p, err)
		}
		if !sgIsIdentityIDMap(string(b)) {
			return fmt.Errorf("could-not-check: %s is %q, not the initial user namespace's identity mapping %q, so a file owner read here may be a remapped caller",
				p, strings.Join(strings.Fields(string(b)), " "), sgIdentityIDMap)
		}
	}
	return nil
}

// sgIsIdentityIDMap: exactly one extent, inside 0, outside 0, every one of the 2^32-1 ids.
func sgIsIdentityIDMap(s string) bool {
	return strings.Join(strings.Fields(s), " ") == sgIdentityIDMap
}
