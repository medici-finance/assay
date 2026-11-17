//go:build !linux

package deskkit

import "os"

// readProcFile off Linux is a plain read: there is no procfs, and userNSIdentity treats the
// absent file as the existing behavior.
func readProcFile(path string) ([]byte, error) { return os.ReadFile(path) }
