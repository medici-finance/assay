//go:build darwin || linux

package main

import (
	"fmt"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"time"
)

var maintenanceAdmissions []*os.File

func releaseMaintenanceAdmissions() {
	for _, f := range maintenanceAdmissions {
		deskkit.UnlockFile(f)
		f.Close()
	}
	maintenanceAdmissions = nil
}
func holdMaintenanceAdmission(c *Cell) error {
	if maintenanceCommand == "cache" && len(verbWords) > 1 && verbWords[1] == "status" {
		return nil
	}
	if c.Env.Get("DRY_RUN") == "1" || maintenanceCommand == "set" || maintenanceCommand == "" || maintenanceCommand == "reset" || maintenanceCommand == "show" || maintenanceCommand == "status" || maintenanceCommand == "check" {
		return nil
	}
	path := filepath.Join(c.Dir, ".reset.lock")
	if _, err := os.Lstat(path); err == nil {
		if err = privateCommsFile(path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		f.Close()
		return fmt.Errorf("reset holds the cell admission lock")
	}
	maintenanceAdmissions = append(maintenanceAdmissions, f)
	return nil
}
func waitResetAdmission(c *Cell) (*os.File, error) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		f, err := acquireCellLock(c, ".reset.lock")
		if err == nil {
			return f, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("launches still hold cell admission: %w", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
