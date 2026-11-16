//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
)

// rootIdentity names a landing root by its directory's device and inode, so every spelling of
// one directory — including a re-cased path on a case-insensitive filesystem — maps to one
// preparation lock. A root that cannot be stat'ed keeps its resolved path (admission refuses
// such a root before anything stages into it).
func rootIdentity(abs string) string {
	info, err := os.Stat(abs)
	if err != nil {
		return "path:" + abs
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "path:" + abs
	}
	return fmt.Sprintf("dev:%d:ino:%d", uint64(st.Dev), uint64(st.Ino))
}
