//go:build !unix

package cellscratch

import "os"

// removeEmptyDir removes name from the open directory parent. Off Unix there is no
// directory-only removal relative to a handle, so a file renamed onto name after the
// directory was emptied is removed in its place.
func removeEmptyDir(parent *os.Root, name string) error {
	return parent.Remove(name)
}
