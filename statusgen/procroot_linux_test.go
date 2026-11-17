//go:build linux

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// sgFixIno1Root is the root directory of a mounted filesystem other than procfs (its root
// inode is 1, as procfs's is), or "" where this host shows none of the usual candidates.
func sgFixIno1Root() string {
	for _, d := range []string{"/sys", "/sys/fs/cgroup", "/dev", "/dev/shm", "/run"} {
		var st unix.Stat_t
		var sfs unix.Statfs_t
		if unix.Stat(d, &st) != nil || unix.Statfs(d, &sfs) != nil {
			continue
		}
		if st.Ino == sgProcRootIno && int64(sfs.Type) != unix.PROC_SUPER_MAGIC {
			return d
		}
	}
	return ""
}

// regression: #6781
// TestStatusgenProcRootChecked: the /proc reader trusts its root only when that root is a
// procfs root, by filesystem type and by inode. A mount-crossing check below the root cannot
// see a tmpfs mounted at /proc itself, so a root carrying planted identity maps, a procfs
// directory that is not the root, and another filesystem's root are each refused, and the
// namespace gate reads all three as could-not-check.
func TestStatusgenProcRootChecked(t *testing.T) {
	if _, err := sgReadProcIn("/proc", sgProcUIDMap); err != nil {
		t.Fatalf("control: %s unreadable under /proc: %v", sgProcUIDMap, err)
	}
	planted := t.TempDir()
	for _, f := range []string{"uid_map", "gid_map"} {
		sgFixWrite(t, filepath.Join(planted, "self", f), sgIdentityIDMap+"\n", 0o644)
	}
	roots := map[string]string{
		"not procfs, identity maps planted": planted,
		"procfs, not its root":              "/proc/self",
	}
	if d := sgFixIno1Root(); d != "" {
		roots["inode 1, not procfs"] = d
	} else {
		t.Log("no non-procfs root with inode 1 here: the filesystem-type half is pinned only by the planted root")
	}
	for name, root := range roots {
		b, err := sgReadProcIn(root, sgProcUIDMap)
		if err == nil || !strings.Contains(err.Error(), root+" is not a procfs root") {
			t.Errorf("%s (%s): read = %q, %v; want a not-a-procfs-root refusal", name, root, b, err)
		}
		read := func(p string) ([]byte, error) { return sgReadProcIn(root, p) }
		if err := sgUserNSIdentity(read, "linux"); err == nil || !strings.HasPrefix(err.Error(), "could-not-check: ") {
			t.Errorf("%s (%s): gate = %v, want could-not-check", name, root, err)
		}
	}
}
