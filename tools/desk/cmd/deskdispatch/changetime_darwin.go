//go:build darwin

package main

import (
	"io/fs"
	"syscall"
	"time"
)

// changeTime is the inode change time of st, or the zero time when the platform has none.
func changeTime(st fs.FileInfo) time.Time {
	if s, ok := st.Sys().(*syscall.Stat_t); ok {
		return time.Unix(s.Ctimespec.Sec, s.Ctimespec.Nsec)
	}
	return time.Time{}
}
