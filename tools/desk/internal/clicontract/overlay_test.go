package clicontract

import (
	"io/fs"
	"sort"
	"strings"
	"testing/fstest"
	"time"
)

// overlayFS lays planted files over a base tree, so a mutation can add a command or
// rewrite a brief without touching the checkout. Directories that exist only in the
// overlay are synthesized; ReadDir merges both layers.
type overlayFS struct {
	base  fs.FS
	files fstest.MapFS
}

func newOverlay(base fs.FS, files map[string]string) *overlayFS {
	m := fstest.MapFS{}
	for p, s := range files {
		m[p] = &fstest.MapFile{Data: []byte(s), Mode: 0o644, ModTime: time.Unix(0, 0)}
	}
	return &overlayFS{base: base, files: m}
}

func (o *overlayFS) Open(name string) (fs.File, error) {
	if _, ok := o.files[name]; ok {
		return o.files.Open(name)
	}
	f, err := o.base.Open(name)
	if err == nil {
		return f, nil
	}
	if o.isOverlayDir(name) {
		return o.files.Open(name)
	}
	return nil, err
}

func (o *overlayFS) ReadFile(name string) ([]byte, error) {
	if f, ok := o.files[name]; ok {
		return f.Data, nil
	}
	return fs.ReadFile(o.base, name)
}

func (o *overlayFS) isOverlayDir(name string) bool {
	for p := range o.files {
		if strings.HasPrefix(p, name+"/") {
			return true
		}
	}
	return false
}

func (o *overlayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	seen := map[string]fs.DirEntry{}
	baseEntries, baseErr := fs.ReadDir(o.base, name)
	for _, e := range baseEntries {
		seen[e.Name()] = e
	}
	overEntries, overErr := fs.ReadDir(o.files, name)
	for _, e := range overEntries {
		seen[e.Name()] = e
	}
	if baseErr != nil && overErr != nil {
		return nil, baseErr
	}
	out := make([]fs.DirEntry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

var _ fs.ReadDirFS = (*overlayFS)(nil)
var _ fs.ReadFileFS = (*overlayFS)(nil)
