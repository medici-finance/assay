//go:build !darwin && !linux

package main

import (
	"os"
	"time"
)

// statCtime off darwin and linux: no change time is read, so ok is false and
// readFileMemo / readDirMemo read from disk on every call, as before the memo.
func statCtime(os.FileInfo) (ctime time.Time, ino, dev uint64, ok bool) {
	return time.Time{}, 0, 0, false
}
