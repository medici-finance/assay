//go:build !linux

package main

import "os"

// sgReadProc off Linux is a plain read: there is no procfs, and sgUserNSIdentity treats the
// absent file as the existing behavior.
func sgReadProc(path string) ([]byte, error) { return os.ReadFile(path) }
