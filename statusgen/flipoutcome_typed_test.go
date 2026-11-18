package main

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// flipoutcome_typed_test.go — the two record reads of verifyflip's outcome
// cross-check run on the typed reader. Each test builds a repository where
// the native command the read replaced, and the typed read, could disagree,
// and requires the same answer from both.

// regression: #6734
func TestOutcomeStatusParity(t *testing.T) {
	installStatusgenFixture(t)
	root := t.TempDir()
	mail := vfMailVerifier
	vfGit(t, root, mail, "init", "-q")
	for _, rel := range []string{
		"docs/streams/verify-outcomes/vf/a.json",
		"docs/streams/verify-outcomes/vf/gone.json",
		"docs/streams/verify-outcomes/zz/b.json",
		"docs/streams/verify-outcomes-old.jsonl",
		"docs/streams/other.md",
	} {
		vfWriteAt(t, root, rel, []byte("x\n"))
	}
	vfWriteAt(t, root, "docs/streams/verify-outcomes/vf/.gitignore", []byte("*.tmp\n"))
	vfCommit(t, root, mail)

	// In scope: a modified, a deleted and an untracked record under the
	// stream directory; a modified legacy log; an untracked file the glob
	// matches only because its wildcard crosses '/'.
	vfWriteAt(t, root, "docs/streams/verify-outcomes/vf/a.json", []byte("y\n"))
	if err := os.Remove(filepath.Join(root, "docs/streams/verify-outcomes/vf/gone.json")); err != nil {
		t.Fatal(err)
	}
	vfWriteAt(t, root, "docs/streams/verify-outcomes/vf/new/deep.json", []byte("x\n"))
	vfWriteAt(t, root, "docs/streams/verify-outcomes-old.jsonl", []byte("y\n"))
	vfWriteAt(t, root, "docs/streams/verify-outcomes2/sub/x.jsonl", []byte("x\n"))
	// Out of scope: another stream's records, a near-miss name, an ignored
	// file under the stream directory, a change elsewhere.
	vfWriteAt(t, root, "docs/streams/verify-outcomes/zz/b.json", []byte("y\n"))
	vfWriteAt(t, root, "docs/streams/verify-outcomes/vfx/c.json", []byte("x\n"))
	vfWriteAt(t, root, "docs/streams/verify-outcomes.json", []byte("x\n"))
	vfWriteAt(t, root, "docs/streams/verify-outcomes/vf/scratch.tmp", []byte("x\n"))
	vfWriteAt(t, root, "docs/streams/other.md", []byte("y\n"))

	out, err := historyGit(root, "status", "--porcelain=v1", "--untracked-files=all", "--",
		path.Join(outcomeRecordsDir, "vf"), path.Join("docs/streams", verifyOutcomesGlob)).Output()
	if err != nil {
		t.Fatal(err)
	}
	var native []string
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if l != "" {
			native = append(native, l)
		}
	}
	sort.Strings(native)

	read, err := flipOutcomeRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	status, err := read.repo.DirtyPorcelain()
	if err != nil {
		t.Fatal(err)
	}
	typed := outcomeStatusLines(status, read.prefix, "vf")
	sort.Strings(typed)
	if len(native) != 5 || strings.Join(native, "\n") != strings.Join(typed, "\n") {
		t.Fatalf("status differs from native (want 5 lines):\nnative:\n%s\ntyped:\n%s", strings.Join(native, "\n"), strings.Join(typed, "\n"))
	}

	// A root below the top: the pathspecs are root-relative, the lines are
	// top-relative.
	if got := outcomeStatusLines("?? sub/docs/streams/verify-outcomes/vf/a.json\n?? docs/streams/verify-outcomes/vf/b.json", "sub/", "vf"); len(got) != 1 || !strings.HasPrefix(got[0], "?? sub/") {
		t.Fatalf("prefix not applied: %q", got)
	}
}

// regression: #6734
func TestOutcomeAddersParity(t *testing.T) {
	installStatusgenFixture(t)
	root := t.TempDir()
	mail := vfMailVerifier
	g := func(args ...string) { t.Helper(); vfGit(t, root, mail, args...) }
	g("init", "-q")
	g("commit", "-q", "--allow-empty", "-m", "root")
	// a.json arrives on a side branch and is merged; b.json is added on
	// main, deleted and added again; c.json is added then only modified.
	g("checkout", "-q", "-b", "side")
	vfWriteAt(t, root, "r/a.json", []byte("a\n"))
	vfCommit(t, root, mail)
	g("checkout", "-q", "main")
	vfWriteAt(t, root, "r/b.json", []byte("b\n"))
	vfWriteAt(t, root, "r/c.json", []byte("c\n"))
	vfCommit(t, root, mail)
	g("merge", "-q", "--no-ff", "side", "-m", "merge side")
	g("rm", "-q", "r/b.json")
	g("commit", "-q", "-m", "drop b")
	vfWriteAt(t, root, "r/b.json", []byte("b2\n"))
	vfWriteAt(t, root, "r/c.json", []byte("c2\n"))
	vfCommit(t, root, mail)

	read, err := flipOutcomeRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"r/a.json": 1, "r/b.json": 2, "r/c.json": 1, "r/none.json": 0}
	for rel, n := range want {
		out, err := historyGit(root, "log", "--full-history", "--diff-filter=A", "--format=%H", "HEAD", "--", filepath.FromSlash(rel)).Output()
		if err != nil {
			t.Fatal(err)
		}
		native := strings.Fields(string(out))
		typed, err := outcomeAddingCommits(read, rel)
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(native)
		sort.Strings(typed)
		if len(native) != n || strings.Join(native, " ") != strings.Join(typed, " ") {
			t.Fatalf("%s: adding commits differ from native (want %d): native %v typed %v", rel, n, native, typed)
		}
	}
}
