//go:build !darwin && !linux

package main

import (
	"io/fs"
	"time"
)

// changeTimeKnown is false here: this build reads no inode change time, and modification time
// alone cannot show that a timestamp-preserving copy just populated a snapshot. The snapshot
// prune therefore keeps every snapshot on this platform and says so (planSnapsIn,
// reportVerifySnaps).
const changeTimeKnown = false

// changeTime has no source on this platform.
func changeTime(fs.FileInfo) time.Time { return time.Time{} }
