//go:build !darwin && !linux

package main

import (
	"io/fs"
	"time"
)

// changeTime has no portable source here: the snapshot prune then judges by modification time
// alone.
func changeTime(fs.FileInfo) time.Time { return time.Time{} }
