package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitbatch_test.go — forge-neutral/18 Verify row 12's correctness half. The count half is
// measured on a real tree with a git PATH shim (the PR records it); these tests pin that the
// fewer processes give the SAME answers the per-read processes gave, case by case.

// TestGitShowObjectMatchesExec: every read through a session — blob, empty blob, file without
// a trailing newline, binary bytes, a tree, a missing path, a bad revision, a repeated read —
// returns exactly what `git show <rev>:<path>` returns, error-ness included.
func TestGitShowObjectMatchesExec(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.com")
	files := map[string][]byte{
		"a.md":          []byte("# a\nline\n"),
		"empty.md":      {},
		"nonl.md":       []byte("no trailing newline"),
		"bin.dat":       {0, 1, 2, '\n', 0xff, 0},
		"dir/inner.md":  []byte("inner\n"),
		"sp ace/x y.md": []byte("spaced\n"),
	}
	for p, b := range files {
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "one")
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("# a\nchanged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "commit", "-am", "two")

	cases := [][2]string{
		{"HEAD", "a.md"}, {"HEAD~1", "a.md"}, {"HEAD", "empty.md"}, {"HEAD", "nonl.md"},
		{"HEAD", "bin.dat"}, {"HEAD", "dir/inner.md"}, {"HEAD", "dir"}, {"HEAD", "sp ace/x y.md"},
		{"HEAD", "missing.md"}, {"no-such-rev", "a.md"}, {"HEAD", "a.md"},
	}
	end := beginGitReadSession()
	defer end()
	for _, c := range cases {
		want, wantErr := exec.Command("git", "-C", root, "show", c[0]+":"+c[1]).Output()
		got, gotErr := gitShowObject(root, c[0], c[1])
		if (wantErr != nil) != (gotErr != nil) || !bytes.Equal(want, got) {
			t.Errorf("%s:%s: session read (%q, err=%v) != git show (%q, err=%v)",
				c[0], c[1], got, gotErr, want, wantErr)
		}
	}
	// A memoised answer is a copy: a caller mutating it cannot corrupt the next read.
	b1, _ := gitShowObject(root, "HEAD", "a.md")
	b1[0] = 'X'
	if b2, _ := gitShowObject(root, "HEAD", "a.md"); b2[0] != '#' {
		t.Error("memoised blob was shared with a caller, not copied")
	}
	for _, args := range [][2]string{{"HEAD", "HEAD~1"}, {"HEAD~1", "HEAD"}, {"HEAD", "nope"}} {
		want, wantErr := exec.Command("git", "-C", root, "merge-base", args[0], args[1]).Output()
		for i := 0; i < 2; i++ { // second call is the memo
			got, gotErr := gitMergeBaseOut(root, args[0], args[1])
			if (wantErr != nil) != (gotErr != nil) || !bytes.Equal(want, got) {
				t.Errorf("merge-base %v (call %d): (%q, %v) != (%q, %v)", args, i, got, gotErr, want, wantErr)
			}
		}
	}
}

// TestGitReadSessionEndsWithRun: the memo never outlives its session — a read after the
// session closes sees the tree as it is now, not the answer cached before HEAD moved.
func TestGitReadSessionEndsWithRun(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.com")
	if err := os.WriteFile(filepath.Join(root, "f.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "v1")
	end := beginGitReadSession()
	if b, _ := gitShowObject(root, "HEAD", "f.md"); string(b) != "v1\n" {
		t.Fatalf("got %q", b)
	}
	end()
	if activeGitReads != nil {
		t.Fatal("session still active after its closer ran")
	}
	if err := os.WriteFile(filepath.Join(root, "f.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "commit", "-am", "v2")
	end = beginGitReadSession()
	defer end()
	if b, _ := gitShowObject(root, "HEAD", "f.md"); string(b) != "v2\n" {
		t.Fatalf("a new session served a previous session's answer: %q", b)
	}
}

// TestDeclAgeBatchMatchesPickaxe: the one-`git log -p` index returns, for every name, the same
// introducing commit time per-name `git log -S` does — across add, remove, re-add, a move that
// leaves the count unchanged, a count change on one line, a name that is a prefix of another,
// a merge, and a name that never existed.
func TestDeclAgeBatchMatchesPickaxe(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root, "T", "t@example.com")
	file := filepath.Join(root, "statusgen", "main.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	when := 1700000000
	commit := func(body, msg string) {
		t.Helper()
		if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		when += 86400
		date := fmt.Sprintf("GIT_COMMITTER_DATE=%d +0000", when)
		runGit(t, root, "add", "-A")
		runGitEnv(t, root, []string{date, fmt.Sprintf("GIT_AUTHOR_DATE=%d +0000", when)}, "commit", "-m", msg)
	}
	commit("a := flag.Bool(\"alpha\", false, \"\")\nb := flag.Bool(\"beta\", false, \"\")\n", "add alpha beta")
	commit("b := flag.Bool(\"beta\", false, \"\")\n", "remove alpha")
	commit("b := flag.Bool(\"beta\", false, \"\")\na := flag.Bool(\"alpha\", false, \"\")\n", "re-add alpha")
	commit("a := flag.Bool(\"alpha\", false, \"\")\nb := flag.Bool(\"beta\", false, \"\")\n", "move lines, counts unchanged")
	commit("a := flag.Bool(\"alpha\", false, \"\")\nb := flag.Bool(\"beta\", false, \"\")\nd := \"delta\" + \"delta\"\n", "delta twice")
	commit("a := flag.Bool(\"alpha\", false, \"\")\nb := flag.Bool(\"beta\", false, \"\")\nd := \"delta\"\n", "delta once")
	runGit(t, root, "checkout", "-b", "side")
	commit("a := flag.Bool(\"alpha\", false, \"\")\nb := flag.Bool(\"beta\", false, \"\")\nd := \"delta\"\ng := flag.Bool(\"gamma\", false, \"\")\n", "side: gamma")
	runGit(t, root, "checkout", "-")
	commit("z := flag.Bool(\"zeta\", false, \"\")\na := flag.Bool(\"alpha\", false, \"\")\nb := flag.Bool(\"beta\", false, \"\")\nd := \"delta\"\n", "main: zeta")
	runGitEnv(t, root, []string{fmt.Sprintf("GIT_COMMITTER_DATE=%d +0000", when+86400)}, "merge", "--no-edit", "side")

	b := newBatchedDeclAge()
	if idx := readDeclPatchIndex(root, filepath.Join("statusgen", "main.go")); idx == nil {
		t.Fatal("the batch index fell back on a plain text history — the test would compare exec with exec")
	}
	for _, name := range []string{"alpha", "beta", "delta", "gamma", "zeta", "alph", "never-declared"} {
		wantUnix, wantOK := gitDeclIntroUnix(root, file, name)
		gotUnix, gotOK := b.introUnix(root, file, name)
		if wantUnix != gotUnix || wantOK != gotOK {
			t.Errorf("%q: batch (%d, %v) != git log -S (%d, %v)", name, gotUnix, gotOK, wantUnix, wantOK)
		}
	}
}

// TestDeclPatchParseFailsClosed: a patch the parser does not read with certainty yields no
// index, so every name goes back to per-name `git log -S` — never a different answer.
func TestDeclPatchParseFailsClosed(t *testing.T) {
	for name, log := range map[string]string{
		"binary": declCommitMark + "1\n\ndiff --git a/x b/x\nBinary files a/x and b/x differ\n",
		"rename": declCommitMark + "1\n\ndiff --git a/x b/y\nsimilarity index 90%\nrename from x\nrename to y\n",
		"bad ts": declCommitMark + "soon\n",
		"cc":     declCommitMark + "1\n\ndiff --cc x\n",
	} {
		if parseDeclPatchLog(log) != nil {
			t.Errorf("%s: parser returned an index for a patch it cannot read with certainty", name)
		}
	}
}
