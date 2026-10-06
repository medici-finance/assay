//go:build windows

package cellscratch

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
)

func singleInputLink(file *os.File) error {
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(windows.Handle(file.Fd()), &info); err != nil {
		return err
	}
	if info.NumberOfLinks != 1 {
		return errors.New("input must have one filesystem link")
	}
	return nil
}
