//go:build darwin || linux

package main

import (
	"fmt"
	"golang.org/x/sys/unix"
)

func availableDiskBytes(path string) (uint64, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, err
	}
	if stat.Bsize <= 0 {
		return 0, fmt.Errorf("filesystem block size unavailable")
	}
	blocks, size := uint64(stat.Bavail), uint64(stat.Bsize)
	if blocks > ^uint64(0)/size {
		return 0, fmt.Errorf("filesystem available bytes overflow")
	}
	return blocks * size, nil
}
