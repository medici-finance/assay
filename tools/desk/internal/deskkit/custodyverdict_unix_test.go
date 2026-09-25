//go:build unix

package deskkit

import (
	"os"
	"path/filepath"
	"testing"
)

// TestClassifyCustodyOwnerOnlyPath pins the unix read-back on real files. The file the write
// path reads back is the regular file it has just renamed into place, so the read-back looks
// at the path itself (Lstat), never through a link: a link found there means the path was
// swapped after the rename, and it is refused rather than reported verified for whatever the
// link points at.
func TestClassifyCustodyOwnerOnlyPath(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, mode os.FileMode) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("tok"), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
		return p
	}
	owned := write("owned.token", 0o600)
	loose := write("loose.token", 0o644)
	link := filepath.Join(dir, "gitlab-worker.token")
	if err := os.Symlink(owned, link); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
	for _, tc := range []struct {
		name string
		path string
		want CustodyState
	}{
		{"a 0600 regular file is verified", owned, CustodyVerified},
		{"a 0644 regular file is refused", loose, CustodyRefused},
		{"a link to a 0600 file is refused, never judged through", link, CustodyRefused},
		{"a missing file is inconclusive", filepath.Join(dir, "absent.token"), CustodyInconclusive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyCustodyOwnerOnly(tc.path)
			if got.State != tc.want {
				t.Fatalf("state = %v, want %v (err=%v)", got.State, tc.want, got.Err)
			}
			if got.State != CustodyVerified && got.Err == nil {
				t.Fatalf("a %v verdict must carry its reason", got.State)
			}
		})
	}
}
