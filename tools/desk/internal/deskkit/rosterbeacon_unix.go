//go:build unix

package deskkit

import "os"

func replaceRosterBeacon(from, to string) error {
	return os.Rename(from, to)
}
