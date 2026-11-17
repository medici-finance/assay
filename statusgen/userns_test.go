package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestStatusgenUserNSIdentity: the namespace gate passes only the identity mapping in both
// uid_map and gid_map; an absent file is the existing behavior only off Linux.
func TestStatusgenUserNSIdentity(t *testing.T) {
	id := "         0          0 4294967295\n"
	absent := func(string) ([]byte, error) { return nil, os.ErrNotExist }
	denied := func(string) ([]byte, error) { return nil, os.ErrPermission }
	uidOnly := func(p string) ([]byte, error) {
		if p == sgProcUIDMap {
			return []byte(id), nil
		}
		return nil, os.ErrNotExist
	}
	maps := func(u, g string) func(string) ([]byte, error) {
		return func(p string) ([]byte, error) {
			if p == sgProcUIDMap {
				return []byte(u), nil
			}
			if p == sgProcGIDMap {
				return []byte(g), nil
			}
			return nil, errors.New("unexpected read of " + p)
		}
	}
	for _, c := range []struct {
		name string
		read func(string) ([]byte, error)
		goos string
		ok   bool
	}{
		{"identity, linux", maps(id, id), "linux", true},
		{"identity, other unix", maps(id, id), "darwin", true},
		{"no proc files off linux", absent, "darwin", true},
		{"no proc files on linux", absent, "linux", false},
		{"gid_map absent on linux", uidOnly, "linux", false},
		{"unreadable off linux", denied, "darwin", false},
		{"uid_map remapped", maps("0 1000 1\n", id), "linux", false},
		{"gid_map remapped", maps(id, "0 1000 1\n"), "linux", false},
		{"remapped off linux", maps("0 1000 1\n", id), "freebsd", false},
		{"outside offset", maps("0 1 4294967295\n", id), "linux", false},
	} {
		err := sgUserNSIdentity(c.read, c.goos)
		if (err == nil) != c.ok || (err != nil && !strings.HasPrefix(err.Error(), "could-not-check: ")) {
			t.Errorf("%s: sgUserNSIdentity = %v, want ok=%v (a refusal is could-not-check)", c.name, err, c.ok)
		}
	}

}

// TestStatusgenOwnerInGated: the raw owner check, sgHotfixOwnedByIn, has exactly one caller,
// sgHotfixOwnedByNS in hotfixowner_unix.go, behind the namespace gate. A second caller
// anywhere would be a root-owner check that skips the gate.
func TestStatusgenOwnerInGated(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	call := regexp.MustCompile(`\bsgHotfixOwnedByIn\(`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		n := len(call.FindAllIndex(b, -1))
		want := 0
		if f == "hotfixowner_unix.go" {
			want = 2 // the definition and its one gated caller
		}
		if n != want {
			t.Errorf("%s names sgHotfixOwnedByIn %d times, want %d: a root-owner check must pass the user-namespace gate", f, n, want)
		}
	}
}

// sgOwnerReaders is every non-test file in this module that reads a file owner's uid, and
// why it may. A file owner is read as this process's user namespace maps it, so a check that
// trusts an owner as root, apart from the caller, belongs behind the namespace gate.
var sgOwnerReaders = map[string]string{
	"hotfixowner_unix.go": "the root-owner check, behind sgUserNSIdentity (TestStatusgenOwnerInGated)",
	"rosterowner_unix.go": "compares the owner with the caller's own uid, which no namespace remaps apart",
}

// regression: #6768
// TestStatusgenOwnerReadersKnown: any new file that reads a file owner's uid (a Stat_t and
// its Uid) fails here until it is listed in sgOwnerReaders with its reason, so a new
// root-owner check cannot land without the namespace gate being considered.
func TestStatusgenOwnerReadersKnown(t *testing.T) {
	uid := regexp.MustCompile(`\.Uid\b`)
	seen := map[string]bool{}
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if n := d.Name(); p != "." && (n == "testdata" || n == "vendor" || strings.HasPrefix(n, ".")) {
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
		seen[filepath.ToSlash(p)] = true
		if _, ok := sgOwnerReaders[filepath.ToSlash(p)]; !ok {
			t.Errorf("%s reads a file owner's uid and is not in sgOwnerReaders: a root-owner check must pass the user-namespace gate (sgUserNSIdentity); list it with its reason", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for f := range sgOwnerReaders {
		if !seen[f] {
			t.Errorf("sgOwnerReaders lists %s, which no longer reads a file owner: drop it", f)
		}
	}
}
