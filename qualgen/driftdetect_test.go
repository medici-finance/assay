package main

import (
	"testing"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// openHeadTree opens repoDir and returns HEAD's SHA and tree, for
// driftdetect unit tests that resolve targets against real git state — not
// mocks — the same dereference discipline mine_test.go uses.
func openHeadTree(t *testing.T, repoDir string) (sha string, tree *object.Tree) {
	t.Helper()
	r, err := git.PlainOpen(repoDir)
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}
	head, err := r.Head()
	if err != nil {
		t.Fatalf("resolve HEAD: %v", err)
	}
	c, err := r.CommitObject(head.Hash())
	if err != nil {
		t.Fatalf("resolve HEAD commit: %v", err)
	}
	tr, err := c.Tree()
	if err != nil {
		t.Fatalf("resolve HEAD tree: %v", err)
	}
	return head.Hash().String(), tr
}

// TestDriftDetectFilePath_InSyncAndDrifted is part of Verify row 2's `-run
// Drift` filter: a live path resolves in-sync, a dead path resolves drifted,
// dereferenced against a real git tree (not a mock).
func TestDriftDetectFilePath_InSyncAndDrifted(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "real.go", "package lib\n")
	gitCmd(t, dir, "add", "real.go")
	gitCmd(t, dir, "commit", "-q", "-m", "add real.go")

	sha, tree := openHeadTree(t, dir)
	ctx := Context{Tree: tree, CommitSHA: sha, SourcePath: "doc.md"}

	live := driftdetect(ctx, Target{Kind: TargetFilePath, Value: "real.go"})
	if live.Verdict != VerdictInSync {
		t.Fatalf("real.go: got verdict %q, want in-sync", live.Verdict)
	}
	if live.Reason != "" {
		t.Errorf("in-sync verdict carries a reason %q, want empty", live.Reason)
	}

	dead := driftdetect(ctx, Target{Kind: TargetFilePath, Value: "ghost.go"})
	if dead.Verdict != VerdictDrifted {
		t.Fatalf("ghost.go: got verdict %q, want drifted", dead.Verdict)
	}
	if dead.Reason == "" {
		t.Error("a drifted verdict must carry a reason")
	}
}

// TestDriftDetectSymbol_ExcludesSourceSelfMention proves a symbol only
// mentioned inside the SOURCE doc itself (never defined elsewhere) resolves
// drifted — the doc's own mention of its dead symbol must never count as the
// evidence keeping it alive.
func TestDriftDetectSymbol_ExcludesSourceSelfMention(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "doc.md", "mentions `GhostFunc()` but never defines it\n")
	writeFile(t, dir, "lib.go", "package lib\n\nfunc RealFunc() {}\n")
	gitCmd(t, dir, "add", "doc.md", "lib.go")
	gitCmd(t, dir, "commit", "-q", "-m", "add doc and lib")

	sha, tree := openHeadTree(t, dir)
	ctx := Context{Tree: tree, CommitSHA: sha, SourcePath: "doc.md"}

	dead := driftdetect(ctx, Target{Kind: TargetSymbol, Value: "GhostFunc()"})
	if dead.Verdict != VerdictDrifted {
		t.Fatalf("GhostFunc: got verdict %q, want drifted (self-mention only)", dead.Verdict)
	}

	live := driftdetect(ctx, Target{Kind: TargetSymbol, Value: "RealFunc()"})
	if live.Verdict != VerdictInSync {
		t.Fatalf("RealFunc: got verdict %q, want in-sync", live.Verdict)
	}
}

// TestDriftDetectUnregisteredKind_CouldNotCheck proves an unknown target kind
// is could-not-check, never silently dropped or mis-resolved as either
// in-sync or drifted (Verify row 6: the shared resolution path is the ONLY
// path — an unregistered kind fails loud, not silent).
func TestDriftDetectUnregisteredKind_CouldNotCheck(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	gitCmd(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "doc.md", "placeholder\n")
	gitCmd(t, dir, "add", "doc.md")
	gitCmd(t, dir, "commit", "-q", "-m", "add doc")

	sha, tree := openHeadTree(t, dir)
	ctx := Context{Tree: tree, CommitSHA: sha, SourcePath: "doc.md"}

	res := driftdetect(ctx, Target{Kind: TargetKind("unknown-kind"), Value: "whatever"})
	if res.Verdict != VerdictCouldNotCheck {
		t.Fatalf("unregistered kind: got verdict %q, want could-not-check", res.Verdict)
	}
	if res.Reason == "" {
		t.Error("a could-not-check verdict must carry a reason")
	}
}
