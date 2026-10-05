//go:build unix

package deskkit

import (
	"encoding/json"
	"golang.org/x/sys/unix"
	"os"
	"testing"
	"time"
)

func TestBeaconBoundaryRejectsFIFOWithoutBlocking(t *testing.T) {
	for _, lock := range []bool{false, true} {
		name := "beacon"
		if lock {
			name = "lock"
		}
		t.Run(name, func(t *testing.T) {
			beaconStoreFixture(t)
			path := beaconStoreSeed(t, "boundary", `{}`)
			if lock {
				path += ".lock"
			} else if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := unix.Mkfifo(path, 0600); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			called := false
			go func() {
				if !lock {
					if _, err := ReadRosterBeacon("boundary"); err == nil {
						done <- nil
						return
					}
				}
				_, err := MutateRosterBeacon("boundary", func(map[string]json.RawMessage) (BeaconAction, error) { called = true; return BeaconWrite, nil })
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil || called {
					t.Fatalf("FIFO accepted: %t %v", called, err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("FIFO open blocked")
			}
		})
	}
}

func beaconSymlinkUnavailable(error) bool { return false }
