//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// createJunction makes link a directory junction to target — the directory link a Windows host
// can create without SeCreateSymbolicLinkPrivilege. link must not exist: os.Mkdir refuses an
// existing entry, so a junction never replaces or writes through anything. If the reparse point
// cannot be set, the empty directory this call made is removed again (os.Remove of an empty
// directory — nothing else is touched).
func createJunction(target, link string) error {
	data, err := junctionReparseData(target)
	if err != nil {
		return err
	}
	if err := os.Mkdir(link, 0o755); err != nil {
		return err
	}
	p, err := windows.UTF16PtrFromString(link)
	if err != nil {
		_ = os.Remove(link)
		return err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		_ = os.Remove(link)
		return fmt.Errorf("open %s: %w", link, err)
	}
	var n uint32
	ioErr := windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &data[0], uint32(len(data)), nil, 0, &n, nil)
	_ = windows.CloseHandle(h)
	if ioErr != nil {
		_ = os.Remove(link)
		return fmt.Errorf("set junction %s -> %s: %w", link, target, ioErr)
	}
	return nil
}
