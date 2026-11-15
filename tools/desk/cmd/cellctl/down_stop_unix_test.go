//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A launch removes the one STOP body `cellctl down` writes and nothing else (#6116).

// regression: #6116
func TestAdmitRoleClearsOnlyDownStop(t *testing.T) {
	cases := map[string]func(t *testing.T, stop string){
		"operator stop": func(t *testing.T, stop string) {
			must(t, os.WriteFile(stop, []byte("operator requested stop\n"), 0600))
		},
		"empty marker": func(t *testing.T, stop string) {
			must(t, os.WriteFile(stop, nil, 0600))
		},
		"down body plus more": func(t *testing.T, stop string) {
			must(t, os.WriteFile(stop, []byte(downStopReason+"and the operator's note\n"), 0600))
		},
		"down body without newline": func(t *testing.T, stop string) {
			must(t, os.WriteFile(stop, []byte("cellctl down"), 0600))
		},
		"symlink to a down body": func(t *testing.T, stop string) {
			target := filepath.Join(t.TempDir(), "elsewhere")
			must(t, os.WriteFile(target, []byte(downStopReason), 0600))
			must(t, os.Symlink(target, stop))
		},
		"directory": func(t *testing.T, stop string) {
			must(t, os.Mkdir(stop, 0700))
		},
	}
	for name, plant := range cases {
		t.Run(name, func(t *testing.T) {
			c := roundTripCell(t)
			dir := c.cadenceDir(roundTripRole)
			must(t, os.MkdirAll(dir, 0700))
			stop := filepath.Join(dir, "STOP")
			plant(t, stop)
			before, err := os.Lstat(stop)
			must(t, err)
			lease, err := c.admitRole(roundTripRole, false)
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			after, err := os.Lstat(stop)
			if err != nil || !os.SameFile(before, after) || before.Size() != after.Size() {
				t.Fatalf("a launch removed or changed a stop down did not write: %v", err)
			}
			if err := c.cadenceGuard(roundTripRole); !errors.Is(err, errStoppedByFlag) {
				t.Fatalf("the kept stop no longer stops the role: %v", err)
			}
		})
	}
	t.Run("down stop", func(t *testing.T) {
		c := roundTripCell(t)
		dir := c.cadenceDir(roundTripRole)
		must(t, os.MkdirAll(dir, 0700))
		must(t, writeDownStop(dir))
		lease, err := c.admitRole(roundTripRole, false)
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Close()
		if _, err := os.Lstat(filepath.Join(dir, "STOP")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("the launch kept the stop down wrote: %v", err)
		}
		if err := c.cadenceGuard(roundTripRole); err != nil {
			t.Fatalf("the role is still stopped: %v", err)
		}
	})
}

// The planted second instance of the class: down's stop request written over an operator's stop
// would turn it into one the next launch lifts.
//
// regression: #6116
func TestDownStopKeepsOperatorStop(t *testing.T) {
	dir := t.TempDir()
	stop := filepath.Join(dir, "STOP")
	const operator = "operator requested stop\n"
	must(t, os.WriteFile(stop, []byte(operator), 0600))
	must(t, writeDownStop(dir))
	if b, err := os.ReadFile(stop); err != nil || string(b) != operator {
		t.Fatalf("down overwrote the operator's stop: %q %v", b, err)
	}
	if removed, err := clearDownStop(dir); err != nil || removed {
		t.Fatalf("the next launch lifted the operator's stop: removed=%v %v", removed, err)
	}
	if b, err := os.ReadFile(stop); err != nil || string(b) != operator {
		t.Fatalf("the operator's stop changed: %q %v", b, err)
	}
}

// The cell-wide flags are never down's to lift, whatever their body: the safety #6116 keeps.
func TestAdmitRoleKeepsCellFlags(t *testing.T) {
	for _, name := range []string{"STOP", "STOP." + roundTripRole, "DISABLED"} {
		t.Run(name, func(t *testing.T) {
			c := roundTripCell(t)
			flag := filepath.Join(c.Config, name)
			must(t, os.WriteFile(flag, []byte(downStopReason), 0600))
			lease, err := c.admitRole(roundTripRole, false)
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			if _, err := os.Lstat(flag); err != nil {
				t.Fatalf("a launch removed the cell flag %s: %v", name, err)
			}
		})
	}
}
