//go:build linux

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// sgProcRootIno is the inode number of a procfs mount's root directory (PROC_ROOT_INO).
const sgProcRootIno = 1

// sgReadProc reads a file under /proc only when it can be proven to be that procfs file
// itself. A filesystem-type check alone is not enough: in a caller's own mount namespace
// another procfs file can be bind-mounted over /proc/self/uid_map and still report procfs.
// So the reader opens the procfs root, checks it is one (procfs type, root inode), and opens
// the path beneath it with openat2 refusing any mount crossing (RESOLVE_NO_XDEV), any magic
// link and any escape. A mount anywhere on the path, a kernel without openat2, or a path
// outside /proc is an error, which the namespace gate turns into could-not-check.
func sgReadProc(path string) ([]byte, error) { return sgReadProcIn("/proc", path) }

// sgReadProcIn is sgReadProc with the procfs root as a parameter, so a test can hand it a directory
// that is not a procfs root and see it refused. Production passes "/proc" only.
func sgReadProcIn(procRoot, path string) ([]byte, error) {
	rel, ok := strings.CutPrefix(path, "/proc/")
	if !ok || rel == "" {
		return nil, fmt.Errorf("%s is not on procfs (not under /proc)", path)
	}
	root, err := unix.Open(procRoot, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("%s unopenable: %v", procRoot, err)
	}
	defer unix.Close(root)
	var sfs unix.Statfs_t
	if err := unix.Fstatfs(root, &sfs); err != nil {
		return nil, fmt.Errorf("%s: filesystem type unreadable: %v", procRoot, err)
	}
	var st unix.Stat_t
	if err := unix.Fstat(root, &st); err != nil {
		return nil, fmt.Errorf("%s: unstattable: %v", procRoot, err)
	}
	if int64(sfs.Type) != unix.PROC_SUPER_MAGIC || st.Ino != sgProcRootIno {
		return nil, fmt.Errorf("%s is not a procfs root (filesystem type %#x, inode %d)", procRoot, sfs.Type, st.Ino)
	}
	fd, err := unix.Openat2(root, rel, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_CLOEXEC,
		Resolve: unix.RESOLVE_NO_XDEV | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_BENEATH,
	})
	if err != nil {
		if err == unix.EXDEV {
			return nil, fmt.Errorf("%s: a mount lies on the path, so it is not the kernel's own file", path)
		}
		return nil, fmt.Errorf("%s: no mount-free open (openat2): %v", path, err)
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	return io.ReadAll(f)
}
