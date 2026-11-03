package cellscratch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// metadataPath applies the source exclusion to every path component.
func metadataPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		name, _, _ := strings.Cut(part, ":")
		if strings.EqualFold(strings.TrimRight(name, " ."), ".git") {
			return true
		}
	}
	return false
}

// descend walks from current through dirs, one literal directory at a time, holding
// each directory open once it is proved: a component must be a real directory (never a
// link, even one that stays inside the root) and keep its identity when opened. It owns
// current: it returns the last directory opened (current itself for no dirs) and
// closes every handle on failure. A later rename or link swap of any walked name cannot
// move the returned handle.
func descend(current *os.Root, dirs []string) (*os.Root, error) {
	for _, part := range dirs {
		before, err := current.Lstat(part)
		if err != nil {
			current.Close()
			return nil, err
		}
		if !before.IsDir() {
			current.Close()
			return nil, errNotRealDir
		}
		next, err := current.OpenRoot(part)
		current.Close()
		if err != nil {
			return nil, err
		}
		after, err := next.Stat(".")
		if err != nil || !os.SameFile(before, after) {
			next.Close()
			return nil, errors.New("directory identity changed while opening it")
		}
		current = next
	}
	return current, nil
}

// openInput holds directory handles throughout traversal. Every component must
// be literal and retain its identity when opened; the leaf must be regular.
func openInput(source *os.Root, path string) (*os.File, os.FileInfo, error) {
	current, err := source.OpenRoot(".")
	if err != nil {
		return nil, nil, err
	}
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	if current, err = descend(current, parts[:len(parts)-1]); err != nil {
		return nil, nil, err
	}
	defer current.Close()
	leaf := parts[len(parts)-1]
	before, err := current.Lstat(leaf)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, nil, errors.New("input must be a regular file")
	}
	file, err := current.Open(leaf)
	if err != nil {
		return nil, nil, err
	}
	after, err := file.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		file.Close()
		return nil, nil, errors.New("input file identity changed")
	}
	if err = singleInputLink(file); err != nil {
		file.Close()
		return nil, nil, err
	}
	return file, after, nil
}
