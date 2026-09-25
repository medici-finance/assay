//go:build unix

package main

import "golang.org/x/sys/unix"

// setUmask lets the permissive-create fixture produce a genuinely group/world-readable file
// whatever the test runner's umask is.
func setUmask(m int) int { return unix.Umask(m) }
