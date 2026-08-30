package gitcore

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// --- Read-helper goldens ---------------------------------------------------
//
// These reuse brief 01's golden-harness names ("read-head", "diff-after-change") on
// purpose: the fixture construction is fully deterministic (fixed author/committer
// identity and dates), so a gitcore read helper that preserves outcome produces the
// BYTE-IDENTICAL golden JSON that the git-binary op it replaces already recorded in
// internal/gittest/testdata. That equality — not merely "gitcore's tests pass" — is
// the evidence a seam swap is outcome-preserving.

func TestGoldenResolveHead(t *testing.T) {
	gittest.Record(t, "read-head", func(f *gittest.Fixture) (string, error) {
		repo, err := Open(f.Dir)
		if err != nil {
			return "", err
		}
		h, err := repo.Resolve("HEAD")
		if err != nil {
			return "", err
		}
		return h.String(), nil
	})
}

func TestGoldenDiffAfterChange(t *testing.T) {
	gittest.Record(t, "diff-after-change", func(f *gittest.Fixture) (string, error) {
		f.CommitFile(t, "added.txt", "new line\n", "add a file")
		repo, err := Open(f.Dir)
		if err != nil {
			return "", err
		}
		names, err := repo.DiffNames("HEAD~1", "HEAD")
		if err != nil {
			return "", err
		}
		if len(names) != 1 {
			t.Fatalf("expected exactly one changed path, got %v", names)
		}
		return names[0], nil
	})
}

// --- Direct read-helper assertions (refs / objects / log / merge-base) -----
//
// These families have no brief-01 golden template to reuse, so they assert directly
// against the fixture's git-binary-reported truth rather than a JSON snapshot — same
// "outcome, not argv" discipline, applied inline.

func TestRefsMatchesForEachRef(t *testing.T) {
	f := gittest.NewFixture(t)
	f.CommitFile(t, "second.txt", "second\n", "second commit")

	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := repo.Refs()
	if err != nil {
		t.Fatal(err)
	}

	want, err := f.Git("rev-parse", "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	if got := refs["refs/heads/main"]; got != want {
		t.Fatalf("refs/heads/main: got %q want %q", got, want)
	}
}

func TestFileAtMatchesCatFile(t *testing.T) {
	f := gittest.NewFixture(t)

	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.FileAt("HEAD", "seed.txt")
	if err != nil {
		t.Fatal(err)
	}
	// FileAt returns raw blob content (including trailing newline); the fixture's
	// git-binary Outcome() trims via TrimSpace, so compare against the untrimmed seed.
	if want := "seed\n"; got != want {
		t.Fatalf("FileAt(seed.txt) = %q, want %q", got, want)
	}
}

func TestFilesMatchesLsTree(t *testing.T) {
	f := gittest.NewFixture(t)
	if err := os.MkdirAll(filepath.Join(f.Dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.CommitFile(t, "sub/nested.txt", "nested\n", "add nested file")

	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Files("HEAD")
	if err != nil {
		t.Fatal(err)
	}

	wantOut, err := f.Git("ls-tree", "-r", "--name-only", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"seed.txt", "sub/nested.txt"}
	if wantOut == "" {
		want = nil
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Files(HEAD) = %v, want %v", got, want)
	}
}

func TestLogMatchesRevList(t *testing.T) {
	f := gittest.NewFixture(t)
	f.CommitFile(t, "second.txt", "second\n", "second commit")

	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Log("HEAD")
	if err != nil {
		t.Fatal(err)
	}

	wantOut, err := f.Git("rev-list", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{}
	for _, line := range splitNonEmpty(wantOut) {
		want = append(want, line)
	}
	if len(got) != len(want) {
		t.Fatalf("Log(HEAD) length = %d, want %d (%v vs %v)", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("Log(HEAD)[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMergeBaseAndIsAncestor(t *testing.T) {
	f := gittest.NewFixture(t)
	base, err := f.Git("rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	f.CommitFile(t, "second.txt", "second\n", "second commit")
	tip, err := f.Git("rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	repo, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}

	gotBase, err := repo.MergeBase(base, tip)
	if err != nil {
		t.Fatal(err)
	}
	if gotBase != base {
		t.Fatalf("MergeBase = %q, want %q", gotBase, base)
	}

	isAnc, err := repo.IsAncestor(base, tip)
	if err != nil {
		t.Fatal(err)
	}
	if !isAnc {
		t.Fatal("IsAncestor(base, tip) = false, want true")
	}

	isAnc, err = repo.IsAncestor(tip, base)
	if err != nil {
		t.Fatal(err)
	}
	if isAnc {
		t.Fatal("IsAncestor(tip, base) = true, want false")
	}
}

func splitNonEmpty(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}

// --- Transport verbs: Fetch / Push / List -----------------------------------
//
// go-git treats a plain local filesystem path as a local endpoint and serves it
// in-process (no git binary spawned, no network) — see go-git's
// transport/internal/common url.IsLocalEndpoint. That makes these fixtures a fully
// OFFLINE stand-in for a real remote: same in-process transport code path go-git uses
// for a real HTTPS remote, zero live infrastructure.

func TestFetchOutcomeMatchesServer(t *testing.T) {
	server := gittest.NewFixture(t)
	server.CommitFile(t, "second.txt", "second\n", "second commit")

	dest := &gittest.Fixture{Dir: t.TempDir()}
	if _, err := dest.Git("init", "-q", "-b", "main"); err != nil {
		t.Fatalf("init dest: %v", err)
	}

	repo, err := Open(dest.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Fetch(FetchOpts{
		URL:      server.Dir,
		RefSpecs: []string{"refs/heads/main:refs/heads/main"},
	}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// Point dest's HEAD at the fetched branch so Outcome's `ls-tree HEAD` resolves —
	// Fetch (matching `git fetch`) updates refs/objects, not the local HEAD/worktree.
	if _, err := dest.Git("checkout", "-q", "main"); err != nil {
		t.Fatalf("checkout fetched main: %v", err)
	}

	got := dest.Outcome("", nil)
	want := server.Outcome("", nil)
	if got.Refs["refs/heads/main"] != want.Refs["refs/heads/main"] {
		t.Fatalf("fetched HEAD = %v, want %v", got.Refs, want.Refs)
	}
	if !reflect.DeepEqual(got.Files, want.Files) {
		t.Fatalf("fetched files = %v, want %v", got.Files, want.Files)
	}
}

func TestPushOutcomeMatchesLocal(t *testing.T) {
	local := gittest.NewFixture(t)
	local.CommitFile(t, "second.txt", "second\n", "second commit")

	remoteDir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", "--bare", "-b", "main", remoteDir).CombinedOutput(); err != nil {
		t.Fatalf("init bare remote: %v: %s", err, out)
	}

	repo, err := Open(local.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Push(PushOpts{
		URL:      remoteDir,
		RefSpecs: []string{"refs/heads/main:refs/heads/main"},
	}); err != nil {
		t.Fatalf("Push: %v", err)
	}

	remote := &gittest.Fixture{Dir: remoteDir}
	got := remote.Outcome("", nil)
	want := local.Outcome("", nil)
	if got.Refs["refs/heads/main"] != want.Refs["refs/heads/main"] {
		t.Fatalf("pushed HEAD = %v, want %v", got.Refs, want.Refs)
	}
	if !reflect.DeepEqual(got.Files, want.Files) {
		t.Fatalf("pushed files = %v, want %v", got.Files, want.Files)
	}
}

func TestListMatchesForEachRef(t *testing.T) {
	server := gittest.NewFixture(t)

	refs, err := List(ListOpts{URL: server.Dir})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want, err := server.Git("rev-parse", "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for _, ref := range refs {
		if ref.Name().String() == "refs/heads/main" {
			got = ref.Hash().String()
		}
	}
	if got != want {
		t.Fatalf("List refs/heads/main = %q, want %q", got, want)
	}
}
