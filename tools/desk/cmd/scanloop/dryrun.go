package main

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// dryrun.go — a --dry-run poll never touches the real baselines.
//
// A poll ADVANCES the poller's per-repo baselines: that is how the next poll knows what is new. A
// `run --dry-run` that polled the REAL state dir therefore consumed the very inbound delta it
// printed, and the next real `run` saw nothing, cut no scan carrier, and dropped the whole
// placeholder delta without a word. The flag's name promises that nothing is touched, so the flag
// has to keep that promise rather than a help paragraph warning that it does not.
//
// So a dry-run poll runs against a THROWAWAY COPY of the state dir. The preview stays live — the
// poller starts from the real baselines and reports the real delta — and the copy is removed when
// the pass ends, leaving the real baselines byte-identical.
//
// The copy fails CLOSED. If it cannot be made faithfully, the pass is refused (exit 5) before the
// poller runs. It never falls back to the real dir: a fallback is exactly the mutation this file
// exists to prevent, and a preview that silently became a real poll is worse than no preview.

// dryRunStatePrefix names the throwaway copies so a leftover one is recognisable on disk.
const dryRunStatePrefix = "scanloop-dryrun-state-"

// dryRunTempBase is where throwaway copies are made. It is a seam so a test can point it at a dir
// it controls; production uses the system temp dir.
var dryRunTempBase = func() string { return os.TempDir() }

// dryRunStateCopy copies the real state dir into a fresh throwaway dir and returns that dir and
// the function that removes it.
//
// A real dir that does not exist yet (never armed) yields an EMPTY copy: the dry-run poll then
// seeds the copy, and the real dir stays un-armed, as it was. Any other failure to read or copy is
// a refusal, and the partial copy is removed before returning.
//
// Only directories and regular files are copied. Anything else is refused rather than copied as-is.
// A symlinked baseline copied as a symlink would let the poller write THROUGH it to the real file.
func dryRunStateCopy(real string) (string, func() error, error) {
	refuse := func(why string, err error) error {
		msg := "scanloop run --dry-run: cannot make a throwaway copy of the poller's state dir " + real +
			" (" + why + "). A dry-run polls a copy so the real per-repo baselines are not advanced; " +
			"it never falls back to the real dir. Fix the state dir, or preview a captured poll with " +
			"--offline --inbound <file>"
		if err != nil {
			msg += ": " + err.Error()
		}
		return deskkit.Refused(msg)
	}

	realAbs, err := filepath.Abs(real)
	if err != nil {
		return "", nil, refuse("the path does not resolve", err)
	}

	tmp, err := os.MkdirTemp(dryRunTempBase(), dryRunStatePrefix)
	if err != nil {
		return "", nil, refuse("no throwaway dir could be created", err)
	}
	cleanup := func() error { return os.RemoveAll(tmp) }
	fail := func(why string, err error) (string, func() error, error) {
		_ = cleanup()
		return "", nil, refuse(why, err)
	}

	tmpAbs, err := filepath.Abs(tmp)
	if err != nil {
		return fail("the throwaway dir does not resolve", err)
	}
	// A copy made INSIDE the tree it copies would copy itself, and removing it would be a write
	// inside the real state dir.
	if rel, rerr := filepath.Rel(realAbs, tmpAbs); rerr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fail("the throwaway dir "+tmpAbs+" would sit inside the state dir", nil)
	}

	info, err := os.Lstat(realAbs)
	switch {
	case os.IsNotExist(err):
		// Never armed. The copy starts empty, and the dry-run poll seeds the copy, not the real dir.
		return tmp, cleanup, nil
	case err != nil:
		return fail("the state dir cannot be read", err)
	case !info.IsDir():
		return fail("the state dir is not a directory", nil)
	}

	werr := filepath.WalkDir(realAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(realAbs, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(tmp, rel)
		fi, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			if rel == "." {
				return nil // the throwaway root itself, already created owner-only
			}
			// Owner rwx is added so the poller can write the copy and cleanup can remove it.
			return os.Mkdir(dst, fi.Mode().Perm()|0o700)
		case fi.Mode().IsRegular():
			if err := copyStateFile(path, dst, fi); err != nil {
				return err
			}
			return nil
		default:
			return &fs.PathError{Op: "copy", Path: path,
				Err: errNotCopyable(fi.Mode().Type().String())}
		}
	})
	if werr != nil {
		return fail("an entry could not be copied", werr)
	}
	return tmp, cleanup, nil
}

// copyStateFile copies one baseline: bytes, permission bits, and modification time. The poller
// reads the content, but the mtime is kept so the copy is indistinguishable from the real file.
func copyStateFile(src, dst string, fi fs.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fi.Mode().Perm()|0o200)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chtimes(dst, fi.ModTime(), fi.ModTime())
}

type errNotCopyable string

func (e errNotCopyable) Error() string {
	return "not a regular file or directory (" + string(e) + "), so it cannot be copied without the poller being able to write through it"
}
