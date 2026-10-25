package gitcore

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage"
)

type denyBlobLoad struct {
	storage.Storer
	reads int
}

func (s *denyBlobLoad) EncodedObject(typ plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	if typ == plumbing.BlobObject {
		s.reads++
		return nil, fmt.Errorf("blob inflated before header preflight")
	}
	return s.Storer.EncodedObject(typ, h)
}
func boundedFixture(t *testing.T, withLink bool) (*Repo, string, string) {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) string {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
		b, e := c.CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		return strings.TrimSpace(string(b))
	}
	git("init", "-q")
	if e := os.WriteFile(filepath.Join(dir, "a"), []byte("small"), 0600); e != nil {
		t.Fatal(e)
	}
	git("add", "a")
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "base")
	base := git("rev-parse", "HEAD")
	if e := os.WriteFile(filepath.Join(dir, "a"), []byte(strings.Repeat("x", 4096)), 0600); e != nil {
		t.Fatal(e)
	}
	if withLink {
		if e := os.Symlink("a", filepath.Join(dir, "link")); e != nil {
			t.Skip("symlink unavailable", e)
		}
	}
	git("add", "a")
	if withLink {
		git("add", "link")
	}
	git("-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "large")
	head := git("rev-parse", "HEAD")
	r, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	return r, base, head
}
func TestFileAtBoundedPreflight(t *testing.T) {
	for _, path := range []string{"a", "link"} {
		t.Run(path, func(t *testing.T) {
			r, _, head := boundedFixture(t, true)
			guard := &denyBlobLoad{Storer: r.repo.Storer}
			r.repo.Storer = guard
			_, large, e := r.FileAtBounded(head, path, 16)
			if path == "a" && (e != nil || !large || guard.reads != 0) {
				t.Fatalf("over-cap loose blob inflated before refusal: large=%v reads=%d err=%v", large, guard.reads, e)
			}
			if path == "link" && (e == nil || guard.reads != 0) {
				t.Fatal("non-regular blob loaded before mode refusal", guard.reads, e)
			}
		})
	}
}
func TestDiffBoundedPreflight(t *testing.T) {
	r, base, head := boundedFixture(t, false)
	guard := &denyBlobLoad{Storer: r.repo.Storer}
	r.repo.Storer = guard
	if _, large, e := r.DiffSymmetricBounded(base, head, 3, 16); e != nil || !large || guard.reads != 0 {
		t.Fatalf("diff inflated blobs before budget refusal: large=%v reads=%d err=%v", large, guard.reads, e)
	}
}

func TestPreparationExactObjects(t *testing.T) {
	r, base, head := boundedFixture(t, false)
	absent := strings.Repeat("e", 40)
	if _, e := r.repo.CommitObject(plumbing.NewHash(absent)); e == nil {
		t.Fatal("fixture absent object exists")
	}
	// A named reference is not an immutable object observation.
	if e := r.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/"+absent), plumbing.NewHash(head))); e != nil {
		t.Fatal(e)
	}
	if _, _, e := r.FileAtBounded(absent, "a", 8192); e == nil {
		t.Fatal("missing exact file object accepted a reference substitute")
	}
	if _, _, e := r.DiffSymmetricBounded(base, absent, 3, 8192); e == nil {
		t.Fatal("missing exact diff head accepted a reference substitute")
	}
	if _, _, e := r.DiffSymmetricBounded(absent, head, 3, 8192); e == nil {
		t.Fatal("missing exact diff base accepted a reference substitute")
	}
	if raw, large, e := r.FileAtBounded(head, "a", 8192); e != nil || large || len(raw) != 4096 {
		t.Fatal("present exact object failed", large, e)
	}
}
