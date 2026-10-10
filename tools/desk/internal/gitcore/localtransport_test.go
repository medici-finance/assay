package gitcore

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	gitproto "github.com/go-git/go-git/v5/plumbing/transport/git"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"

	"github.com/medici-finance/assay/tools/desk/internal/gittest"
)

// A fetch, a listing and a tree fetch from a local origin — as a bare path, as a checkout
// path and as a file:// URL — start no process at all, under an environment that carries
// git configuration a child would honour. Removing the init() that installs localTransport
// puts go-git's exec-based file client back and this goes red on the first stand-in start.
func TestFileFetchStartsNoChild(t *testing.T) {
	origin := gittest.NewFixture(t)
	sha := origin.CommitFile(t, "a.txt", "a\n", "a")
	bare := filepath.Join(t.TempDir(), "origin.git")
	if out, err := exec.Command("git", "clone", "-q", "--bare", origin.Dir, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone --bare: %v: %s", err, out)
	}
	work := gittest.NewFixture(t)

	logPath := gittest.StandInLocalTransport(t, true)
	gittest.HostileGitEnv(t)

	for _, url := range []string{bare, origin.Dir, "file://" + bare} {
		repo, err := Open(work.Dir)
		if err != nil {
			t.Fatal(err)
		}
		ferr := repo.Fetch(FetchOpts{URL: url, RefSpecs: []string{"+refs/heads/*:refs/remotes/o/*"}, Force: true})
		gittest.AssertNoChild(t, logPath, "Fetch "+url)
		if ferr != nil {
			t.Fatalf("Fetch(%s): %v", url, ferr)
		}

		refs, lerr := List(ListOpts{URL: url})
		gittest.AssertNoChild(t, logPath, "List "+url)
		if lerr != nil || len(refs) == 0 {
			t.Fatalf("List(%s) = %d refs, %v", url, len(refs), lerr)
		}

		_, terr := FetchTree(TreeOpts{URL: url, RefSpecs: fetchTreeSpecs, Commit: sha}, t.TempDir())
		gittest.AssertNoChild(t, logPath, "FetchTree "+url)
		if terr != nil {
			t.Fatalf("FetchTree(%s): %v", url, terr)
		}
	}
	repo, _ := Open(work.Dir)
	if got, err := repo.Resolve("refs/remotes/o/main"); err != nil || got.String() != sha {
		t.Fatalf("fetched refs/remotes/o/main = %v, %v; want %s", got, err, sha)
	}
}

// The protocol table every gitcore transport verb resolves a scheme through holds, for each
// scheme, a client that starts no process: the in-process local transport for file, and
// go-git's pure-Go clients for the network schemes. A scheme (re)installed with go-git's
// exec-based file client — or any client not on this list — fails here, naming the scheme.
func TestFileProtocolIsInProcess(t *testing.T) {
	inProcess := map[transport.Transport]bool{
		localTransport{}:       true,
		http.DefaultClient:     true,
		ssh.DefaultClient:      true,
		gitproto.DefaultClient: true,
	}
	for scheme, tr := range client.Protocols {
		if !inProcess[tr] {
			t.Errorf("scheme %q is served by %T, which is not a known in-process client", scheme, tr)
		}
	}
	if _, ok := client.Protocols["file"].(localTransport); !ok {
		t.Errorf("scheme \"file\" is served by %T, want the in-process localTransport", client.Protocols["file"])
	}
}

func TestLocalGitDirShapes(t *testing.T) {
	origin := gittest.NewFixture(t)
	bare := filepath.Join(t.TempDir(), "o.git")
	if out, err := exec.Command("git", "clone", "-q", "--bare", origin.Dir, bare).CombinedOutput(); err != nil {
		t.Fatalf("clone --bare: %v: %s", err, out)
	}
	cases := map[string]string{
		origin.Dir:                       filepath.Join(origin.Dir, ".git"),
		bare:                             bare,
		strings.TrimSuffix(bare, ".git"): bare,
	}
	for in, want := range cases {
		got, err := localGitDir(in)
		if err != nil || filepath.Clean(got) != filepath.Clean(want) {
			t.Errorf("localGitDir(%s) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := localGitDir(filepath.Join(t.TempDir(), "absent")); err != transport.ErrRepositoryNotFound {
		t.Errorf("localGitDir(absent) err = %v, want ErrRepositoryNotFound", err)
	}
}

// CheckedOutBranches sees the branch of EVERY worktree — the main checkout and each linked
// one — from any of them, and nothing for a detached HEAD.
func TestCheckedOutBranchesAllWorktrees(t *testing.T) {
	main := gittest.NewFixture(t)
	for _, args := range [][]string{{"branch", "side"}, {"branch", "other"}} {
		if _, err := main.Git(args...); err != nil {
			t.Fatal(err)
		}
	}
	linked := filepath.Join(t.TempDir(), "linked")
	if _, err := main.Git("worktree", "add", "-q", linked, "side"); err != nil {
		t.Fatal(err)
	}
	detached := filepath.Join(t.TempDir(), "detached")
	if _, err := main.Git("worktree", "add", "-q", "--detach", detached, "other"); err != nil {
		t.Fatal(err)
	}
	mainHead, _ := main.Git("symbolic-ref", "HEAD")
	want := []string{strings.TrimSpace(mainHead), "refs/heads/side"}
	if want[0] > want[1] {
		want[0], want[1] = want[1], want[0]
	}
	for _, from := range []string{main.Dir, linked, detached} {
		got, err := CheckedOutBranches(from)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("CheckedOutBranches(from %s) = %v, %v; want %v", filepath.Base(from), got, err, want)
		}
	}
}
