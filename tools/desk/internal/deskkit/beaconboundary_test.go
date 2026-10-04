package deskkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBeaconBoundaryRejectsSymlinks(t *testing.T) {
	for _, lock := range []bool{false, true} {
		for _, dangling := range []bool{false, true} {
			name := "beacon"
			if lock {
				name = "lock"
			}
			if dangling {
				name += "-dangling"
			}
			t.Run(name, func(t *testing.T) {
				beaconStoreFixture(t)
				path := beaconStoreSeed(t, "boundary", `{"keep":true}`)
				target := filepath.Join(t.TempDir(), "target")
				const seed = `{"foreign":"preserve"}`
				if !dangling {
					if err := os.WriteFile(target, []byte(seed), 0600); err != nil {
						t.Fatal(err)
					}
				}
				leaf := path
				if lock {
					leaf += ".lock"
				} else if err := os.Remove(leaf); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, leaf); err != nil {
					if beaconSymlinkUnavailable(err) {
						t.Skipf("symlink privilege unavailable: %v", err)
					}
					t.Fatal(err)
				}
				if !lock {
					if obj, err := ReadRosterBeacon("boundary"); err == nil || obj != nil {
						t.Fatalf("read followed symlink: %s %v", obj, err)
					}
				}
				called := false
				_, err := MutateRosterBeacon("boundary", func(map[string]json.RawMessage) (BeaconAction, error) { called = true; return BeaconWrite, nil })
				if err == nil || called {
					t.Fatalf("mutation entered callback through symlink: called=%t err=%v", called, err)
				}
				data, err := os.ReadFile(target)
				if dangling {
					if !os.IsNotExist(err) {
						t.Fatalf("dangling target was created: %q %v", data, err)
					}
				} else if err != nil || string(data) != seed {
					t.Fatalf("target changed: %q %v", data, err)
				}
				info, err := os.Lstat(leaf)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("symlink replaced: %v %v", info, err)
				}
			})
		}
	}
}

// Runs on Windows even when creating symlinks requires unavailable privileges.
func TestBeaconBoundaryRejectsDirectories(t *testing.T) {
	for _, lock := range []bool{false, true} {
		name := "beacon"
		if lock {
			name = "lock"
		}
		t.Run(name, func(t *testing.T) {
			beaconStoreFixture(t)
			path := beaconStoreSeed(t, "boundary", `{"keep":true}`)
			if lock {
				path += ".lock"
			} else if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			if !lock {
				if _, err := ReadRosterBeacon("boundary"); err == nil {
					t.Fatal("read accepted directory")
				}
			}
			called := false
			_, err := MutateRosterBeacon("boundary", func(map[string]json.RawMessage) (BeaconAction, error) { called = true; return BeaconWrite, nil })
			if err == nil || called {
				t.Fatalf("mutation accepted directory: %t %v", called, err)
			}
			info, err := os.Stat(path)
			if err != nil || !info.IsDir() {
				t.Fatalf("directory changed: %v %v", info, err)
			}
		})
	}
}
