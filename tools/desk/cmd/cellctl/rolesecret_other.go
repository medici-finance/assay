//go:build !darwin && !linux

package main

import "errors"

// fileOwnerUID cannot be read here, so every role-secret binding refuses on this platform.
func fileOwnerUID(path string) (int, error) {
	return 0, errors.New("file ownership is not supported on this platform")
}

// probeACL cannot be answered here, so every role-secret binding refuses on this platform.
func probeACL(path string) (bool, error) {
	return false, errors.New("access-control lists cannot be probed on this platform")
}
