//go:build windows

package gitmaintenance

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

func openMaintenanceRegular(path string) (*os.File, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	volume := filepath.VolumeName(abs)
	// Reject device/named-pipe and UNC namespaces before opening a handle.
	if len(volume) != 2 || volume[1] != ':' {
		return nil, fmt.Errorf("maintenance state requires a drive-rooted path")
	}
	name, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open maintenance state", Path: abs, Err: err}
	}
	f := os.NewFile(uintptr(h), abs)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		_ = f.Close()
		return nil, err
	}
	typeID, err := windows.GetFileType(h)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if typeID != windows.FILE_TYPE_DISK || info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 {
		_ = f.Close()
		return nil, fmt.Errorf("maintenance state is not a plain regular file")
	}
	return f, nil
}
