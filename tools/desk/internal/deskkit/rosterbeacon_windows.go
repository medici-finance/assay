//go:build windows

package deskkit

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// Open the reparse point itself and reject it by handle before reading/locking.
// Delete sharing keeps snapshot readers compatible with atomic publication.
func openRosterBeaconFile(path string, lock bool) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	access, disposition := uint32(windows.GENERIC_READ), uint32(windows.OPEN_EXISTING)
	if lock {
		access |= windows.GENERIC_WRITE
		disposition = windows.OPEN_ALWAYS
	}
	sharing := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE)
	if !lock {
		sharing |= windows.FILE_SHARE_DELETE
	}
	h, err := windows.CreateFile(name, access, sharing, nil, disposition, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open roster file", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(h), path)
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err == nil && info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		err = fmt.Errorf("roster file must not be a reparse point")
	}
	if err == nil {
		var stat os.FileInfo
		stat, err = f.Stat()
		if err == nil && !stat.Mode().IsRegular() {
			err = fmt.Errorf("roster file must be regular")
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

const rosterBeaconReplaceWait = 2 * time.Second

func replaceRosterBeacon(from, to string) error {
	source, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	dest, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	// Readers and antivirus software can hold handles that temporarily deny delete
	// sharing. Retry replacement, never remove/truncate the old beacon to get past
	// those handles: a failed publication must leave the previous JSON intact.
	deadline := time.Now().Add(rosterBeaconReplaceWait)
	for {
		err = windows.MoveFileEx(source, dest, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
		if err == nil {
			return nil
		}
		if (!errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_ACCESS_DENIED)) || !time.Now().Before(deadline) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}
