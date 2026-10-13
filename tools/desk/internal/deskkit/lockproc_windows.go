//go:build windows

package deskkit

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

const stillActive = 259

func openProc(pid int) (windows.Handle, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return 0, errProcessGone
	}
	if err != nil {
		return 0, err
	}
	var code uint32
	if err = windows.GetExitCodeProcess(h, &code); err != nil {
		windows.CloseHandle(h)
		return 0, err
	}
	if code != stillActive {
		windows.CloseHandle(h)
		return 0, errProcessGone
	}
	return h, nil
}

// procStart returns pid's creation time, which distinguishes a reused pid.
func procStart(pid int) (string, error) {
	h, err := openProc(pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	var created, exited, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", created.Nanoseconds()), nil
}

func procCommand(pid int) (string, error) {
	h, err := openProc(pid)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err = windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", err
	}
	return commandName(windows.UTF16ToString(buf[:n])), nil
}

// procHoldsFile cannot be answered without enumerating system handles, so a
// windows holder is never more than "unverified" and never eligible for recovery.
func procHoldsFile(pid int, f *os.File) (bool, error) {
	return false, fmt.Errorf("%w: windows exposes no per-process open-file listing to this tool", errCouldNotCheck)
}

// procTerminate is unsupported: a windows holder never reaches eligibility.
func procTerminate(pid int) error {
	return fmt.Errorf("%w: ending a lock holder is not supported on windows", errCouldNotCheck)
}
