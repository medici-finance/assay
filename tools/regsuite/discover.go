package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// discoverModules walks root and returns the repo-relative paths (forward
// slash separated; "." if go.mod sits at root itself) of every directory
// holding a go.mod. It skips any ".git" directory and any directory named
// "testdata" (and everything beneath it) — the same skip the brief's
// "Fixture hazard" fact requires, so a deliberately-broken fixture module
// used by this tool's own tests is never mistaken for a real module by this
// tool's own discovery (or by ci.yml's `git ls-files '*go.mod'` loop, which
// this mirrors).
func discoverModules(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var mods []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "go.mod" {
			return nil
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return err
		}
		mods = append(mods, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(mods)
	return mods, nil
}

// dirHasGoMod reports whether dir directly contains a go.mod file.
func dirHasGoMod(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil && !info.IsDir()
}

func dirReadable(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}
