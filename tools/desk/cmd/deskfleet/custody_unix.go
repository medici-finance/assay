//go:build unix

package main

import "os"

// createRestricted creates a NEW file at path, mode 0600 in the create call itself (the
// process umask can only remove bits from that, never add them). O_EXCL refuses an existing
// path, so a pre-planted file or link is never written through.
func createRestricted(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}
