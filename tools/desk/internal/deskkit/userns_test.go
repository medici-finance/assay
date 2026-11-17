package deskkit

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// TestHotfixUserNSIdentity: the namespace gate passes only the identity mapping in both
// uid_map and gid_map; an absent file is the existing behavior only off Linux.
func TestHotfixUserNSIdentity(t *testing.T) {
	id := "         0          0 4294967295\n"
	absent := func(string) ([]byte, error) { return nil, os.ErrNotExist }
	denied := func(string) ([]byte, error) { return nil, os.ErrPermission }
	for _, c := range []struct {
		name string
		read func(string) ([]byte, error)
		goos string
		ok   bool
	}{
		{"identity, linux", fixIDMaps(id, id), "linux", true},
		{"identity, other unix", fixIDMaps(id, id), "darwin", true},
		{"no proc files off linux", absent, "darwin", true},
		{"no proc files on linux", absent, "linux", false},
		{"gid_map absent on linux", func(p string) ([]byte, error) {
			if p == procUIDMap {
				return []byte(id), nil
			}
			return nil, os.ErrNotExist
		}, "linux", false},
		{"unreadable off linux", denied, "darwin", false},
		{"uid_map remapped", fixIDMaps("0 1000 1\n", id), "linux", false},
		{"gid_map remapped", fixIDMaps(id, "0 1000 1\n"), "linux", false},
		{"outside offset", fixIDMaps("0 1 4294967295\n", id), "linux", false},
		{"other read error", func(string) ([]byte, error) { return nil, errors.New("boom") }, "darwin", false},
	} {
		err := userNSIdentity(c.read, c.goos)
		if (err == nil) != c.ok || (err != nil && !strings.HasPrefix(err.Error(), "could-not-check: ")) {
			t.Errorf("%s: userNSIdentity = %v, want ok=%v (a refusal is could-not-check)", c.name, err, c.ok)
		}
	}
}

// fixIDMaps is a /proc reader serving the given uid_map and gid_map; every other path is
// absent.
func fixIDMaps(uidMap, gidMap string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		switch p {
		case procUIDMap:
			return []byte(uidMap), nil
		case procGIDMap:
			return []byte(gidMap), nil
		}
		return nil, os.ErrNotExist
	}
}
