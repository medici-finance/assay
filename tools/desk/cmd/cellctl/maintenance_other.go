//go:build !darwin && !linux

package main

import (
	"fmt"
	"os"
)

func releaseMaintenanceAdmissions()        {}
func holdMaintenanceAdmission(*Cell) error { return nil }
func waitResetAdmission(*Cell) (*os.File, error) {
	return nil, fmt.Errorf("reset unsupported on this platform")
}
