//go:build linux

package main

import (
	"io/fs"
	"syscall"
	"time"
)

// changeTimeKnown: this platform gives the snapshot prune an inode change time.
const changeTimeKnown = true

// changeTime is the inode change time of st, or the zero time when it cannot be read.
func changeTime(st fs.FileInfo) time.Time {
	if s, ok := st.Sys().(*syscall.Stat_t); ok {
		return time.Unix(int64(s.Ctim.Sec), int64(s.Ctim.Nsec))
	}
	return time.Time{}
}
