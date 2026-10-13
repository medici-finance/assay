//go:build linux

package deskkit

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// procStart returns pid's start time in clock ticks since boot (/proc/<pid>/stat
// field 22), which distinguishes a reused pid from the recorded process.
func procStart(pid int) (string, error) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if errors.Is(err, os.ErrNotExist) {
		return "", errProcessGone
	}
	if err != nil {
		return "", err
	}
	s := string(b)
	i := strings.LastIndexByte(s, ')')
	if i < 0 {
		return "", errors.New("unparseable /proc stat")
	}
	fields := strings.Fields(s[i+1:])
	if len(fields) < 20 {
		return "", errors.New("short /proc stat")
	}
	if fields[0] == "Z" || fields[0] == "X" {
		return "", errProcessGone // a zombie holds no descriptors
	}
	return fields[19], nil
}

func procCommand(pid int) (string, error) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/comm")
	if errors.Is(err, os.ErrNotExist) {
		return "", errProcessGone
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// procHoldsFile reports whether pid has f's file open, by identity, via
// /proc/<pid>/fd. Unreadable descriptor tables are could-not-check, never "no".
func procHoldsFile(pid int, f *os.File) (bool, error) {
	want, err := f.Stat()
	if err != nil {
		return false, err
	}
	dir := "/proc/" + strconv.Itoa(pid) + "/fd"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("%w: %v", errCouldNotCheck, err)
	}
	for _, e := range entries {
		if st, err := os.Stat(dir + "/" + e.Name()); err == nil && os.SameFile(st, want) {
			return true, nil
		}
	}
	return false, nil
}

func procTerminate(pid int) error { return syscall.Kill(pid, syscall.SIGTERM) }
