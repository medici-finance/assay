//go:build unix

package cellscratch

import (
	"errors"
	"os"
	"syscall"
)

func singleInputLink(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink != 1 {
		return errors.New("input must have one filesystem link")
	}
	return nil
}
