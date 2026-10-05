package cellscratch

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Inputs opts specific generated/untracked files into the snapshot budget. It never
// traverses a directory or follows an input symlink outside the source checkout.
func (r *Run) Inputs(source string, paths []string, maxBytes int64) error {
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	if metadataPath(source) {
		return errors.New("source must be outside repository metadata")
	}
	src, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := r.Store.root.OpenRoot(r.Record.ID + "/work")
	if err != nil {
		return err
	}
	defer dst.Close()
	used, _, err := r.Store.measure(r.Record.ID + "/work")
	if err != nil {
		return err
	}
	for _, p := range paths {
		if !filepath.IsLocal(p) || strings.Contains(p, "\\") {
			return errors.New("input must be source-relative")
		}
		if metadataPath(p) {
			return errors.New("Git metadata is not a scratch input")
		}
		// A declared input cannot point back into this owner's output.
		full, err := filepath.EvalSymlinks(filepath.Join(source, p))
		if err != nil {
			return err
		}
		rootPath, err := filepath.EvalSymlinks(r.Store.Path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(rootPath, full)
		if err != nil {
			return err
		}
		if filepath.IsLocal(rel) || rel == "." {
			return errors.New("scratch output cannot be a scratch input")
		}
		in, st, err := openInput(src, p)
		if err != nil {
			return err
		}
		if st.Size() > maxBytes-used {
			in.Close()
			return fmt.Errorf("input %s exceeds snapshot budget", p)
		}
		if err = dst.MkdirAll(filepath.Dir(p), 0700); err != nil {
			in.Close()
			return err
		}
		out, err := dst.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			in.Close()
			return err
		}
		n, copyErr := io.Copy(out, io.LimitReader(in, maxBytes-used+1))
		err = errors.Join(copyErr, in.Close(), out.Close())
		if err != nil {
			return err
		}
		used += n
		if used > maxBytes {
			return errors.New("input grew beyond snapshot budget")
		}
	}
	return nil
}
