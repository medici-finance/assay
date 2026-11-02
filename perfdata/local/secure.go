package local

// secure.go — the store trusts no entry by name. Every directory is checked with Lstat
// (a real directory, this user's, no group or other access) and every file is opened
// through openRegular, which refuses a symbolic link or any non-regular file before and
// after the open. Nothing here repairs an unsafe entry: it refuses with ErrUnsafe.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func unsafeEntry(p, why string) error { return fmt.Errorf("%w: %s: %s", ErrUnsafe, p, why) }

// ensureDir creates d with mode 0700 when it is missing, then checks it.
func ensureDir(d string) error {
	if err := os.Mkdir(d, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return checkDir(d)
}

// checkDir refuses a store directory that is a symbolic link, is not a directory,
// belongs to another user or is open to group or other users (where the platform has
// owners and mode bits).
func checkDir(d string) error {
	fi, err := os.Lstat(d)
	if err != nil {
		return err
	}
	if !fi.IsDir() { // Lstat reports a link as a link, never as its target
		return unsafeEntry(d, "not a directory ("+fi.Mode().Type().String()+")")
	}
	return checkOwner(d, fi)
}

// checkTree refuses any entry of the store that is not what the layout says it is: the
// three subdirectories must be directories and everything else, at the top and inside
// them, must be a regular file.
func checkTree(root string) error {
	ents, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, e := range ents {
		switch e.Name() {
		case "segments", "run", "distributions":
			p := filepath.Join(root, e.Name())
			if err := checkDir(p); err != nil {
				return err
			}
			sub, err := os.ReadDir(p)
			if err != nil {
				return err
			}
			for _, f := range sub {
				if !f.Type().IsRegular() {
					return unsafeEntry(filepath.Join(p, f.Name()), "not a regular file ("+f.Type().String()+")")
				}
			}
		default:
			if !e.Type().IsRegular() {
				return unsafeEntry(filepath.Join(root, e.Name()), "not a regular file ("+e.Type().String()+")")
			}
		}
	}
	return nil
}

// openRegular opens p without following a final symbolic link and only if it is a
// regular file. The Lstat before the open keeps a FIFO or device from blocking the open;
// the Stat after it closes the window between the two.
func openRegular(p string, flag int, perm os.FileMode) (*os.File, error) {
	fi, err := os.Lstat(p)
	switch {
	case err == nil && !fi.Mode().IsRegular():
		return nil, unsafeEntry(p, "not a regular file ("+fi.Mode().Type().String()+")")
	case err != nil && !(errors.Is(err, os.ErrNotExist) && flag&os.O_CREATE != 0):
		return nil, err
	}
	f, err := os.OpenFile(p, flag|noFollow, perm)
	if err != nil {
		if fi, lerr := os.Lstat(p); lerr == nil && !fi.Mode().IsRegular() {
			return nil, unsafeEntry(p, "not a regular file ("+fi.Mode().Type().String()+")")
		}
		return nil, err
	}
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		f.Close()
		if err != nil {
			return nil, err
		}
		return nil, unsafeEntry(p, "not a regular file ("+fi.Mode().Type().String()+")")
	}
	return f, nil
}

// readRegular reads a whole regular file through openRegular.
func readRegular(p string) ([]byte, error) {
	f, err := openRegular(p, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// regularSize is the size of p when it is a regular file; ok=false otherwise.
func regularSize(p string) (int64, bool) {
	fi, err := os.Lstat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return 0, false
	}
	return fi.Size(), true
}
