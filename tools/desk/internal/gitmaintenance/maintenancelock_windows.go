//go:build windows

package gitmaintenance

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
)

func openMaintenanceLock(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_ALWAYS, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open maintenance lock", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(h), path)
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err == nil && info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		err = fmt.Errorf("maintenance lock must not be a reparse point")
	}
	if err == nil {
		var stat os.FileInfo
		stat, err = f.Stat()
		if err == nil && !stat.Mode().IsRegular() {
			err = fmt.Errorf("maintenance lock must be a regular file")
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
