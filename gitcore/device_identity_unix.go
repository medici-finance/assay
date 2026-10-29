//go:build unix

package gitcore

import (
	"os"
	"syscall"
)

func platformDeviceIdentity(info os.FileInfo) (uint64, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(stat.Dev), true
}
