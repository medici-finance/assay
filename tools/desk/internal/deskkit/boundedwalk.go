package deskkit

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
)

// MaintenanceEntries bounds retained accounting/candidate state. Exceeding it
// makes accounting incomplete; it must never authorize admission or reclamation.
const MaintenanceEntries = 262144
const maintenanceDepth = 64

var ErrScanLimit = errors.New("maintenance inventory limit exceeded; accounting incomplete")

// ReadDirBounded never asks the filesystem to materialize an entire directory.
// It returns no partial listing on overflow, so callers cannot act on a prefix.
func ReadDirBounded(f fs.ReadDirFile, limit int) ([]fs.DirEntry, error) {
	var out []fs.DirEntry
	for {
		batch, err := f.ReadDir(1)
		if len(batch) > 0 {
			if len(out) >= limit {
				return nil, ErrScanLimit
			}
			out = append(out, batch...)
		}
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

// BoundedWalkDir streams one entry at a time and keeps at most 64 directory
// descriptors. It preserves WalkDir's no-follow traversal and callback checks;
// callers retain responsibility for custody and identity validation.
func BoundedWalkDir(fsys fs.FS, name string, limit int, fn fs.WalkDirFunc) error {
	return walkDir(fsys, name, max(limit, 0), 0, nil, fn)
}

// PagedWalkDir streams the whole tree in pages of page entries, with no ceiling on
// the entry count: it retains nothing per entry, so a tree wider than one page is
// measured in full instead of being given up at the bound. next runs after every
// page and may stop the walk with an error (a cancelled maintenance context). The
// fixed directory depth still applies and returns ErrScanLimit.
func PagedWalkDir(fsys fs.FS, name string, page int, next func() error, fn fs.WalkDirFunc) error {
	if page < 1 {
		return errors.New("paged walk needs a positive page size")
	}
	return walkDir(fsys, name, -1, page, next, fn)
}

// walkDir is the shared streaming core: limit < 0 means no entry ceiling.
func walkDir(fsys fs.FS, name string, limit, page int, next func() error, fn fs.WalkDirFunc) error {
	remaining, seen := limit, 0
	var visit func(string, fs.DirEntry, int) error
	visit = func(name string, d fs.DirEntry, depth int) error {
		if remaining == 0 || depth >= maintenanceDepth {
			return ErrScanLimit
		}
		if remaining > 0 {
			remaining--
		}
		if seen++; page > 0 && seen%page == 0 && next != nil {
			if err := next(); err != nil {
				return err
			}
		}
		if err := fn(name, d, nil); err != nil {
			if err == fs.SkipDir && d.IsDir() {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			return nil
		}
		f, err := fsys.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		before, beforeErr := d.Info()
		after, afterErr := f.Stat()
		if beforeErr != nil || afterErr != nil || !os.SameFile(before, after) {
			return fmt.Errorf("inventory directory identity changed: %s", name)
		}
		dir, ok := f.(fs.ReadDirFile)
		if !ok {
			return fmt.Errorf("inventory directory cannot stream: %s", name)
		}
		for {
			batch, err := dir.ReadDir(1)
			for _, child := range batch {
				if e := visit(path.Join(name, child.Name()), child, depth+1); e != nil {
					return e
				}
			}
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
		}
	}
	st, err := fs.Stat(fsys, name)
	if err != nil {
		return fn(name, nil, err)
	}
	err = visit(name, fs.FileInfoToDirEntry(st), 0)
	if err == fs.SkipAll {
		return nil
	}
	return err
}

// MaintenanceEntryLimit is an explicit maintenance-only recovery budget. It is
// not persisted as launch policy, and has no unlimited or invalid-value fallback.
func MaintenanceEntryLimit() (int, error) {
	raw := os.Getenv("ASSAY_MAINTENANCE_ENTRY_LIMIT")
	if raw == "" {
		return MaintenanceEntries, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 1048576 {
		return 0, errors.New("ASSAY_MAINTENANCE_ENTRY_LIMIT must be 1..1048576")
	}
	return n, nil
}
