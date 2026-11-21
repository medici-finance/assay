//go:build !unix

package main

import "os"

// openStateFile opens p after an Lstat finds a regular file there, and
// confirms with os.SameFile that the open reached that same file. No FIFO can
// be created at a filesystem name here, and the open has no O_NOFOLLOW, so the
// Lstat plus SameFile pair is what refuses a link at the name.
func openStateFile(p string) (*os.File, error) {
	fi, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errNotRegularStateFile
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	if held, err := f.Stat(); err != nil || !os.SameFile(fi, held) {
		f.Close()
		return nil, errStateFileChanged
	}
	return f, nil
}
