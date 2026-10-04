//go:build windows

package deskkit

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"testing"
)

func TestBeaconBoundaryWindowsLockRetainsInode(t *testing.T) {
	beaconStoreFixture(t)
	path := beaconStoreSeed(t, "boundary", `{}`) + ".lock"
	f, err := openRosterBeaconFile(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := os.Remove(path); err == nil {
		t.Fatal("open lock file was removable")
	}
	if err := os.Rename(path, path+".moved"); err == nil {
		t.Fatal("open lock file was movable")
	}
	contender, err := openRosterBeaconFile(path, true)
	if err != nil {
		t.Fatalf("lock contenders must still open the shared file: %v", err)
	}
	contender.Close()
}

func TestBeaconBoundaryWindowsReaderAllowsReplacement(t *testing.T) {
	beaconStoreFixture(t)
	path := beaconStoreSeed(t, "boundary", `{"old":true}`)
	reader, err := openRosterBeaconFile(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := writeRosterBeacon(path, []byte(`{"new":true}`)); err != nil {
		t.Fatalf("snapshot reader blocked replacement: %v", err)
	}
}

func beaconSymlinkUnavailable(err error) bool {
	return errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD) || errors.Is(err, windows.ERROR_NOT_SUPPORTED)
}
