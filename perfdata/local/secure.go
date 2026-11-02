package local

// secure.go — the store trusts no entry by name. The state directory is checked once
// (this user's or the superuser's, not writable by group or other users without the
// sticky bit); the store directory and its three subdirectories are then each checked
// with Lstat (a real directory, this user's, no group or other access) and bound to a
// directory handle (os.Root). Every later file operation is resolved relative to those
// handles, so renaming or replacing any directory on the path after the check cannot
// redirect a read or a write: the handles keep pointing at the directories that were
// checked, and no name resolves outside them. Every file is opened through openRegular,
// which refuses a symbolic link or any non-regular file before and after the open.
// Nothing here repairs an unsafe entry: it refuses with ErrUnsafe.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func unsafeEntry(p, why string) error { return fmt.Errorf("%w: %s: %s", ErrUnsafe, p, why) }

// subdirs are the store's three subdirectories.
var subdirs = []string{"segments", "run", "distributions"}

// tree is the store directory and its subdirectories, each bound to a handle after it
// was checked. A nil handle is a subdirectory the reader found missing.
type tree struct {
	top, segs, run, dist *os.Root
}

func (t *tree) close() {
	for _, r := range []*os.Root{t.segs, t.run, t.dist, t.top} {
		if r != nil {
			r.Close()
		}
	}
}

// sub is the handle of one subdirectory.
func (t *tree) sub(name string) **os.Root {
	switch name {
	case "segments":
		return &t.segs
	case "run":
		return &t.run
	}
	return &t.dist
}

// openTree checks stateDir and binds the store under it. With create it makes the
// store directories that are missing (0700); without it a missing store returns an
// os.ErrNotExist error and a missing subdirectory is left nil.
func openTree(stateDir string, create bool) (*tree, error) {
	sr, err := os.OpenRoot(stateDir)
	if err != nil {
		return nil, err
	}
	defer sr.Close()
	fi, err := sr.Stat(".")
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, unsafeEntry(stateDir, "not a directory")
	}
	if err := checkStateDir(stateDir, fi); err != nil {
		return nil, err
	}
	top, err := bindDir(sr, DirName, create)
	if err != nil {
		return nil, err
	}
	t := &tree{top: top}
	for _, d := range subdirs {
		r, err := bindDir(top, d, create)
		if errors.Is(err, os.ErrNotExist) && !create {
			continue
		}
		if err != nil {
			t.close()
			return nil, err
		}
		*t.sub(d) = r
	}
	return t, nil
}

// bindDir checks the directory name under parent (creating it first when asked) and
// returns a handle on exactly the directory that was checked.
func bindDir(parent *os.Root, name string, create bool) (*os.Root, error) {
	if create {
		if err := parent.Mkdir(name, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	fi, err := checkDir(parent, name)
	if err != nil {
		return nil, err
	}
	r, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	if got, err := r.Stat("."); err != nil || !os.SameFile(fi, got) {
		r.Close()
		return nil, unsafeEntry(display(parent, name), "replaced while it was opened")
	}
	return r, nil
}

func display(r *os.Root, name string) string { return filepath.Join(r.Name(), name) }

// checkDir refuses a store directory that is a symbolic link, is not a directory,
// belongs to another user or is open to group or other users (where the platform has
// owners and mode bits).
func checkDir(parent *os.Root, name string) (os.FileInfo, error) {
	fi, err := parent.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() { // Lstat reports a link as a link, never as its target
		return nil, unsafeEntry(display(parent, name), "not a directory ("+fi.Mode().Type().String()+")")
	}
	return fi, checkOwner(display(parent, name), fi)
}

// readDir lists a bound directory, sorted by name; a nil handle lists nothing.
func readDir(r *os.Root) ([]os.DirEntry, error) {
	if r == nil {
		return nil, nil
	}
	f, err := r.Open(".")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ents, err := f.ReadDir(-1)
	sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })
	return ents, err
}

// checkTree refuses any entry of the store that is not what the layout says it is: the
// three subdirectories must be directories and everything else, at the top and inside
// them, must be a regular file.
func checkTree(t *tree) error {
	ents, err := readDir(t.top)
	if err != nil {
		return err
	}
	for _, e := range ents {
		switch e.Name() {
		case "segments", "run", "distributions":
			if _, err := checkDir(t.top, e.Name()); err != nil {
				return err
			}
		default:
			if !e.Type().IsRegular() {
				return unsafeEntry(display(t.top, e.Name()), "not a regular file ("+e.Type().String()+")")
			}
		}
	}
	for _, d := range subdirs {
		r := *t.sub(d)
		sub, err := readDir(r)
		if err != nil {
			return err
		}
		for _, f := range sub {
			if !f.Type().IsRegular() {
				return unsafeEntry(display(r, f.Name()), "not a regular file ("+f.Type().String()+")")
			}
		}
	}
	return nil
}

// openRegular opens name under r only if it is a regular file and not a symbolic link.
// The Lstat before the open keeps a FIFO or device from blocking the open; the checks
// after it refuse a file that is not regular or is not the entry the name holds now
// (a link swapped in between the Lstat and the open, which os.Root resolves only
// within r).
func openRegular(r *os.Root, name string, flag int, perm os.FileMode) (*os.File, error) {
	if r == nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
	}
	fi, err := r.Lstat(name)
	switch {
	case err == nil && !fi.Mode().IsRegular():
		return nil, unsafeEntry(display(r, name), "not a regular file ("+fi.Mode().Type().String()+")")
	case err != nil && !(errors.Is(err, os.ErrNotExist) && flag&os.O_CREATE != 0):
		return nil, err
	}
	f, err := r.OpenFile(name, flag, perm)
	if err != nil {
		if fi, lerr := r.Lstat(name); lerr == nil && !fi.Mode().IsRegular() {
			return nil, unsafeEntry(display(r, name), "not a regular file ("+fi.Mode().Type().String()+")")
		}
		return nil, err
	}
	got, err := f.Stat()
	if err == nil && !got.Mode().IsRegular() {
		err = unsafeEntry(display(r, name), "not a regular file ("+got.Mode().Type().String()+")")
	}
	if err == nil {
		if now, lerr := r.Lstat(name); lerr != nil || !os.SameFile(got, now) {
			err = unsafeEntry(display(r, name), "replaced while it was opened")
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// readRegular reads a whole regular file through openRegular.
func readRegular(r *os.Root, name string) ([]byte, error) {
	f, err := openRegular(r, name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// regularSize is the size of name under r when it is a regular file; ok=false otherwise.
func regularSize(r *os.Root, name string) (int64, bool) {
	if r == nil {
		return 0, false
	}
	fi, err := r.Lstat(name)
	if err != nil || !fi.Mode().IsRegular() {
		return 0, false
	}
	return fi.Size(), true
}
