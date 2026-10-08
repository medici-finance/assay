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

// openInput holds directory handles throughout traversal. Every component must
// be literal and retain its identity when opened; the leaf must be regular.
func openInput(source *os.Root, path string) (*os.File, os.FileInfo, error) {
	current, err := source.OpenRoot(".")
	if err != nil {
		return nil, nil, err
	}
	defer func() { current.Close() }()
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	for _, part := range parts[:len(parts)-1] {
		before, err := current.Lstat(part)
		if err != nil {
			return nil, nil, err
		}
		if !before.IsDir() {
			return nil, nil, errors.New("input parents must be literal directories")
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			return nil, nil, err
		}
		after, err := next.Stat(".")
		if err != nil || !os.SameFile(before, after) {
			next.Close()
			return nil, nil, errors.New("input directory identity changed")
		}
		current.Close()
		current = next
	}
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
