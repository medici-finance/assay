//go:build windows

package deskkit

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

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
