//go:build !darwin && !linux

package main

import "errors"

// oNoFollow has no portable equivalent off darwin and linux.
const oNoFollow = 0

// fileIdentity is unsupported here: pause still creates its flag but cannot record it, so resume
// leaves it in place and says so (lifting it is `resume --foreign`).
func fileIdentity(path string) (uint64, string, error) {
	return 0, "", errors.New("file identity is not supported on this platform")
}
