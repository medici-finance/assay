package main

import (
	"errors"
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

// TestStatusgenOwnerInGated: the raw owner check, sgHotfixOwnedByIn, has exactly two callers,
// both in hotfixowner_unix.go: sgHotfixOwnedByNS, behind the namespace gate, and
// sgHotfixOwnedByRoot, the network-off helper's check, which reads its namespace's mapping
// itself. A third caller anywhere would be a root-owner check that skips the gate.
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
			want = 3 // the definition and its two gated callers
		}
		if n != want {
			t.Errorf("%s names sgHotfixOwnedByIn %d times, want %d: a root-owner check must pass the user-namespace gate", f, n, want)
		}
	}
}
