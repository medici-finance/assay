//go:build linux

package main

import (
	"io/fs"
	"syscall"
	"time"
)

// changeTime is the inode change time of st, or the zero time when the platform has none.
func changeTime(st fs.FileInfo) time.Time {
	if s, ok := st.Sys().(*syscall.Stat_t); ok {
		return time.Unix(int64(s.Ctim.Sec), int64(s.Ctim.Nsec))
	}
	return time.Time{}
}
