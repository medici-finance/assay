//go:build linux

package main

import (
	"os"
	"syscall"
	"time"
)

// statCtime reads the change time, inode and device from a linux stat.
func statCtime(fi os.FileInfo) (ctime time.Time, ino, dev uint64, ok bool) {
	st, isStat := fi.Sys().(*syscall.Stat_t)
	if !isStat {
		return time.Time{}, 0, 0, false
	}
	return time.Unix(int64(st.Ctim.Sec), int64(st.Ctim.Nsec)), st.Ino, uint64(st.Dev), true
}
