package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Source-tree guard (#1947). `go test` runs this package's binary with the package
// source directory as its working directory, so any test that writes through a
// relative path — a Loop built with an empty Root, a helper that forgot
// t.TempDir() — lands its files in the source tree and leaves `git status` dirty.
// TestMain snapshots the directory before m.Run and fails the run if any path
// appears that was not there before, naming each one. It covers every test in the
// package, not just the one that tripped it.
//
// The guard only reports; it never deletes, because a path that was not there
// before the run is not necessarily one a test created.

// sourceTreeFiles lists every path under dir, relative and slash-separated.
func sourceTreeFiles(dir string) (map[string]bool, error) {
	seen := map[string]bool{}
	err := filepath.WalkDir(dir, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(dir, p)
		if rerr != nil {
			return rerr
		}
		if rel != "." {
			seen[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	return seen, err
}

// strayPaths returns, sorted, the paths present in after but not in before.
func strayPaths(before, after map[string]bool) []string {
	var out []string
	for p := range after {
		if !before[p] {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// TestStrayPathsGuardSeesPlant is the guard's positive control: a file planted
// between two snapshots must be reported, and an untouched tree must not be.
func TestStrayPathsGuardSeesPlant(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kept.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := sourceTreeFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	same, err := sourceTreeFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := strayPaths(before, same); len(got) != 0 {
		t.Fatalf("untouched tree reported strays: %v", got)
	}
	if err := os.MkdirAll(filepath.Join(dir, "mailbox", "cell-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mailbox", "cell-a", "x.json"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := sourceTreeFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := strayPaths(before, after)
	want := []string{"mailbox", "mailbox/cell-a", "mailbox/cell-a/x.json"}
	if len(got) != len(want) {
		t.Fatalf("strays = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("strays = %v, want %v", got, want)
		}
	}
}
