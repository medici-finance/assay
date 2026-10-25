package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// witnessadapter_test.go — the real-git adapter behind the v2 reader
// (coverage_witness.go): a failed read is never an empty answer, and a
// replace ref never changes what a listing returns.

// regression: #4409 — a listing the repository could not answer is "could
// not read", never an empty set (an empty replace-ref set would clear the
// replace check; an empty tree would read every input as absent).
func TestWitnessAdapterFailureIsNotEmpty(t *testing.T) {
	r := coverageRepo{root: filepath.Join(t.TempDir(), "no-such-repo")}
	if tree, ok := r.ListTree("HEAD"); ok || len(tree) != 0 {
		t.Errorf("ListTree outside a repository = %v, %v; want not ok", tree, ok)
	}
	if set, ok := r.ReplacedObjects(); ok || len(set) != 0 {
		t.Errorf("ReplacedObjects outside a repository = %v, %v; want not ok", set, ok)
	}
}

// regression: #4409 — the witness reads ignore replacement objects: a
// refs/replace/ ref over a tree changes neither the tree listing nor the
// replaced-object listing's report that it exists.
func TestWitnessAdapterIgnoresReplaceRefs(t *testing.T) {
	root := t.TempDir()
	gv2Git(t, root, "init", "-q", "-b", "main")
	gv2Git(t, root, "config", "commit.gpgsign", "false")
	gv2Write(t, root, "a.txt", "one\n")
	gv2Git(t, root, "add", "-A")
	gv2Git(t, root, "commit", "-q", "-m", "one")
	first := gv2Git(t, root, "rev-parse", "HEAD")
	tree1 := gv2Git(t, root, "rev-parse", "HEAD^{tree}")
	gv2Write(t, root, "a.txt", "two\n")
	gv2Write(t, root, "b.txt", "extra\n")
	gv2Git(t, root, "add", "-A")
	gv2Git(t, root, "commit", "-q", "-m", "two")
	tree2 := gv2Git(t, root, "rev-parse", "HEAD^{tree}")

	r := coverageRepo{root: root}
	before, ok := r.ListTree(first)
	if !ok || len(before) != 1 || before[0].Path != "a.txt" {
		t.Fatalf("ListTree(first) = %v, %v", before, ok)
	}
	gv2Git(t, root, "replace", tree1, tree2)
	if got := gv2Git(t, root, "ls-tree", "--name-only", first); !strings.Contains(got, "b.txt") {
		t.Skipf("this git does not apply the replace ref to a plain ls-tree (%q); nothing to guard", got)
	}
	after, ok := r.ListTree(first)
	if !ok || len(after) != len(before) || after[0] != before[0] {
		t.Errorf("ListTree after a replace ref = %v, %v; want %v", after, ok, before)
	}
	set, ok := r.ReplacedObjects()
	if !ok || !set[tree1] {
		t.Errorf("ReplacedObjects = %v, %v; want %s listed", set, ok, tree1)
	}
}
