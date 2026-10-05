//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
)

func commsLaunchOwner(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("comms launch directory must belong to the invoking user")
	}
	return nil
}
