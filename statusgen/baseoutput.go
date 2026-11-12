package main

// Base primary output is local sensitive evidence, opt-in and never printed.
// Files are exclusively created once; their full digest is bound into the Base
// witness cell. This is content integrity, not identity or acceptance authority.

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type baseOutputRecord struct{ name, digest string }
type baseOutputLog struct {
	dir  *os.Root
	rows map[string]baseOutputRecord
	next int
}

// openBaseLogParent rejects every symlink component and pins opened directories.
// Comparing the opened inode with Lstat also refuses a substitution during open.
func openBaseLogParent(path string, homes ...os.FileInfo) (*os.Root, error) {
	anchor := filepath.VolumeName(path) + string(filepath.Separator)
	current, err := os.OpenRoot(anchor)
	if err != nil {
		return nil, err
	}
	insideHome := func(info os.FileInfo) bool {
		for _, home := range homes {
			if os.SameFile(info, home) {
				return true
			}
		}
		return false
	}
	anchorInfo, err := current.Stat(".")
	if err != nil || insideHome(anchorInfo) {
		current.Close()
		return nil, fmt.Errorf("base-log must be outside the admitted and baseline history homes")
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, anchor), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		before, err := current.Lstat(part)
		if err != nil {
			current.Close()
			return nil, err
		}
		if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
			current.Close()
			return nil, fmt.Errorf("base-log parent must contain only real directories")
		}
		if insideHome(before) {
			current.Close()
			return nil, fmt.Errorf("base-log must be outside the admitted and baseline history homes")
		}
		next, err := current.OpenRoot(part)
		if err != nil {
			current.Close()
			return nil, err
		}
		after, err := next.Stat(".")
		current.Close()
		if err != nil || !os.SameFile(before, after) {
			next.Close()
			return nil, fmt.Errorf("base-log parent changed while opening")
		}
		current = next
	}
	return current, nil
}

func newBaseOutputLog(home, path string, otherHomes ...string) (*baseOutputLog, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, fmt.Errorf("base-log must be a clean absolute path without traversal")
	}
	var homes []os.FileInfo
	for _, candidate := range append([]string{home}, otherHomes...) {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			return nil, err
		}
		real, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve base-log home: %w", err)
		}
		if _, inside := relWithin(real, path); inside {
			return nil, fmt.Errorf("base-log must be outside the admitted and baseline history homes")
		}
		info, err := os.Stat(real)
		if err != nil {
			return nil, err
		}
		homes = append(homes, info)
	}
	parent, err := openBaseLogParent(filepath.Dir(path), homes...)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	name := filepath.Base(path)
	if err := parent.Mkdir(name, 0o700); err != nil {
		return nil, err
	}
	before, err := parent.Lstat(name)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("base-log directory changed after creation")
	}
	dir, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	info, err := dir.Stat(".")
	if err != nil || !os.SameFile(before, info) || !info.IsDir() || info.Mode().Perm() != 0o700 {
		dir.Close()
		return nil, fmt.Errorf("base-log directory is not private")
	}
	return &baseOutputLog{dir: dir, rows: map[string]baseOutputRecord{}}, nil
}

func (l *baseOutputLog) close() { _ = l.dir.Close() }

func (l *baseOutputLog) record(id string, output []byte) error {
	if _, exists := l.rows[id]; exists {
		return fmt.Errorf("duplicate base output row")
	}
	if l.next >= 999999 {
		return fmt.Errorf("too many base output rows")
	}
	l.next++
	name := fmt.Sprintf("row-%06d.log", l.next)
	f, err := l.dir.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = l.dir.Remove(name)
		return err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		_ = f.Close()
		_ = l.dir.Remove(name)
		return fmt.Errorf("base output file is not private and owner-readable")
	}
	_, writeErr := f.Write(output)
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil {
		_ = l.dir.Remove(name)
		return writeErr
	}
	if closeErr != nil {
		_ = l.dir.Remove(name)
		return closeErr
	}
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(output))
	l.rows[id] = baseOutputRecord{name: name, digest: digest}
	return nil
}
