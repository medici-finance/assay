package deskkit

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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

// ownerReaders is every non-test file in the desk-tools module that reads a file owner's
// uid, and why it may. A file owner is read as this process's user namespace maps it, so a
// check that trusts an owner as root, apart from the caller, belongs behind the namespace
// gate (userNSIdentity). Paths are relative to the module root.
var ownerReaders = map[string]string{
	"internal/deskkit/hotfixowner_unix.go":  "the root-owner check, behind userNSIdentity",
	"internal/deskkit/rosterowner_unix.go":  "compares the owner with the caller's own uid",
	"internal/cellcache/platform_unix.go":   "caller's own uid, or root for an ancestor beside it: grants nothing the caller lacks",
	"internal/commstransport/owner_unix.go": "compares the owner with the caller's own uid",
	"cmd/cellctl/comms_launch_unix.go":      "compares the owner with the caller's own uid",
	"cmd/cellctl/rolesecret_unix.go":        "returns the owner for a caller's-own-uid comparison",
	"cmd/deskverdict/signopen_unix.go":      "compares the owner with the caller's own uid",
	"cmd/scanloop/scanowner_unix.go":        "compares the owner with the caller's own uid",
}

// regression: #6768
// TestHotfixOwnerReadersKnown: any new file in the module that reads a file owner's uid (a
// Stat_t and its Uid) fails here until it is listed in ownerReaders with its reason, so a
// new root-owner check cannot land without the namespace gate being considered.
func TestHotfixOwnerReadersKnown(t *testing.T) {
	root := filepath.Join("..", "..")
	uid := regexp.MustCompile(`\.Uid\b`)
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); p != root && (n == "testdata" || n == "vendor" || strings.HasPrefix(n, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !strings.Contains(string(b), "Stat_t") || !uid.Match(b) {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		seen[rel] = true
		if _, ok := ownerReaders[rel]; !ok {
			t.Errorf("%s reads a file owner's uid and is not in ownerReaders: a root-owner check must pass the user-namespace gate (userNSIdentity); list it with its reason", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for f := range ownerReaders {
		if !seen[f] {
			t.Errorf("ownerReaders lists %s, which no longer reads a file owner: drop it", f)
		}
	}
}
