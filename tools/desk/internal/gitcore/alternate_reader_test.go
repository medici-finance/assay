package gitcore

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
)

type indexOpenCounter struct {
	billy.Filesystem
	opens int
}

func (f *indexOpenCounter) Open(path string) (billy.File, error) {
	if strings.HasSuffix(path, ".idx") {
		f.opens++
	}
	return f.Filesystem.Open(path)
}

// Assert work, not elapsed time: many distinct borrowed objects and repeated
// reads must decode one pack index once, rather than once per lookup. The
// native git oracle checks contents, and existing shared-clone tests cover
// staged changes and missing stores.
func TestPackedAlternateIndexReadOnce(t *testing.T) {
	src, clone := sharedClone(t)
	for i := 0; i < 12; i++ {
		src.CommitFile(t, "seed.txt", strings.Repeat("x", i+1)+"\n", "grow history")
	}
	gitIn(t, src.Dir, "repack", "-a", "-d")
	repo, err := Open(clone)
	if err != nil {
		t.Fatal(err)
	}
	store := repo.repo.Storer.(extensionTolerantStorer).Storer.(alternateStorer)
	count := &indexOpenCounter{Filesystem: osfs.New(string(filepath.Separator))}
	altRoot := repo.alternates.fs.Root()
	store.reader.dir = dotgit.NewWithOptions(osfs.New(filepath.Join(clone, ".git")), dotgit.Options{
		AlternatesFS: chroot.New(count, altRoot),
	})
	lines := strings.Split(strings.TrimSpace(mustGitOut(t, src.Dir, "rev-list", "--objects", "main")), "\n")
	for round := 0; round < 2; round++ {
		for _, line := range lines {
			h := plumbing.NewHash(strings.Fields(line)[0])
			obj, err := repo.repo.Storer.EncodedObject(plumbing.AnyObject, h)
			if err != nil {
				t.Fatalf("read %s: %v", h, err)
			}
			wantType := strings.TrimSpace(mustGitOut(t, src.Dir, "cat-file", "-t", h.String()))
			if obj.Hash() != h || obj.Type().String() != wantType {
				t.Fatalf("read %s = %s (%q), want type %q; equal hash=%v", h, obj.Hash(), obj.Type().String(), wantType, obj.Hash() == h)
			}
			if round == 0 {
				reader, err := obj.Reader()
				if err != nil {
					t.Fatal(err)
				}
				// Hash/type alone would miss a broken payload read.
				data, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					t.Fatal(err)
				}
				want := mustGitOut(t, src.Dir, "cat-file", wantType, h.String())
				if string(data) != want {
					t.Fatalf("content differs for %s", h)
				}
			}
		}
	}
	if count.opens != 1 {
		t.Fatalf("pack index opened %d times for %d object reads, want 1", count.opens, 2*len(lines))
	}
	_, err = repo.repo.Storer.EncodedObject(plumbing.CommitObject, plumbing.NewHash(strings.TrimSpace(mustGitOut(t, src.Dir, "rev-parse", "main:seed.txt"))))
	if !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Fatalf("wrong-type read = %v, want object not found", err)
	}
	_, err = repo.repo.Storer.EncodedObject(plumbing.AnyObject, plumbing.NewHash(strings.Repeat("f", 40)))
	if !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Fatalf("missing read = %v, want object not found", err)
	}
	if count.opens != 1 {
		t.Fatalf("missing/wrong-type reads reopened index: %d", count.opens)
	}
}

func TestPushPackedSharedClone(t *testing.T) {
	src, clone := sharedClone(t)
	gitIn(t, src.Dir, "repack", "-a", "-d")
	remote := filepath.Join(t.TempDir(), "remote.git")
	gitIn(t, "", "init", "--bare", "-q", remote)
	repo, err := Open(clone)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Push(PushOpts{URL: remote, RefSpecs: []string{"refs/heads/main:refs/heads/main"}}); err != nil {
		t.Fatal(err)
	}
	want := mustGitOut(t, clone, "rev-parse", "HEAD")
	if got := mustGitOut(t, remote, "rev-parse", "main"); got != want {
		t.Fatalf("pushed %s, want %s", got, want)
	}
	gitIn(t, remote, "fsck", "--full", "--no-dangling")
	if got := mustGitOut(t, remote, "show", "main:nested/deep/a.txt"); got != "a\n" {
		t.Fatalf("borrowed blob = %q", got)
	}
	// A failed alternate read must not produce an incomplete successful push.
	os.Remove(filepath.Join(remote, "refs", "heads", "main"))
	withAlternatesHidden(t, clone, func() {
		reopened, err := Open(clone)
		if err != nil {
			t.Fatal(err)
		}
		if err := reopened.Push(PushOpts{URL: remote, RefSpecs: []string{"refs/heads/main:refs/heads/main"}}); err == nil {
			t.Fatal("push succeeded with missing borrowed objects")
		}
	})
}
