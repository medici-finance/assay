//go:build darwin

package main

import (
	"io/fs"
	"syscall"
	"time"
)

// snapNoFollow opens a snapshot directory for chmod without following a final link.
const snapNoFollow = syscall.O_NOFOLLOW

// changeTimeKnown: this platform gives the snapshot prune an inode change time.
const changeTimeKnown = true

// changeTime is the inode change time of st, or the zero time when it cannot be read.
func changeTime(st fs.FileInfo) time.Time {
	if s, ok := st.Sys().(*syscall.Stat_t); ok {
		return time.Unix(s.Ctimespec.Sec, s.Ctimespec.Nsec)
	}
	return time.Time{}
}
