//go:build !darwin && !linux

package main

import (
	"io/fs"
	"time"
)

// changeTimeKnown is false here: this build reads no inode change time, and modification time
// alone cannot show that a timestamp-preserving copy just populated a snapshot. The snapshot
// prune therefore keeps every snapshot on this platform and says so (planVerifySnaps).
const changeTimeKnown = false

// snapNoFollow is inert here: the prune removes nothing on this platform.
const snapNoFollow = 0

// changeTime has no source on this platform.
func changeTime(fs.FileInfo) time.Time { return time.Time{} }
