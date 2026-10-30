package gitcore

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/revlist"
	"github.com/medici-finance/assay/gitcore/gittest"
)

func TestHistoryPackNoNative(t *testing.T) {
	f := gittest.NewFixture(t)
	base, err := f.Git("rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	head := f.CommitFile(t, "delivered.txt", "delivered\n", "delivery")
	if _, err := f.Git("config", "credential.helper", "host-config-canary"); err != nil {
		t.Fatal(err)
	}
	// No native binary or helper is available during either library operation.
	t.Setenv("PATH", t.TempDir())
	var pack bytes.Buffer
	if err := WriteHistoryPack(f.Dir, head, &pack); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "imported")
	if err := ReadHistoryPack(dest, head, bytes.NewReader(pack.Bytes())); err != nil {
		t.Fatal(err)
	}
	r, err := Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r.Resolve("HEAD"); err != nil || got.String() != head {
		t.Fatalf("source binding: %v %v", got, err)
	}
	if got, err := r.FileAt(base, "seed.txt"); err != nil || got != "seed\n" {
		t.Fatalf("baseline history incomplete: %q %v", got, err)
	}
	config, err := os.ReadFile(filepath.Join(dest, ".git", "config"))
	if err != nil || bytes.Contains(config, []byte("host-config-canary")) {
		t.Fatalf("host configuration crossed the transport: %s %v", config, err)
	}
}

func TestHistoryPackRejects(t *testing.T) {
	f := gittest.NewFixture(t)
	head := f.CommitFile(t, "new.txt", "new\n", "head")
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var partial bytes.Buffer
	// A valid Git pack containing the source commit, but not its trees, blobs
	// or parents, must not be mistaken for complete admitted history.
	if _, err := packfile.NewEncoder(&partial, r.repo.Storer, false).Encode([]plumbing.Hash{plumbing.NewHash(head)}, 10); err != nil {
		t.Fatal(err)
	}
	if err := ReadHistoryPack(filepath.Join(t.TempDir(), "partial"), head, bytes.NewReader(partial.Bytes())); err == nil {
		t.Fatal("accepted a source commit with missing reachable objects")
	}
	var pack bytes.Buffer
	if err := WriteHistoryPack(f.Dir, head, &pack); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"corrupt": []byte("not a pack"), "truncated": pack.Bytes()[:pack.Len()/2]} {
		if err := ReadHistoryPack(filepath.Join(t.TempDir(), name), head, bytes.NewReader(data)); err == nil {
			t.Fatalf("accepted %s history", name)
		}
	}
	if err := ReadHistoryPack(filepath.Join(t.TempDir(), "wrong"), strings.Repeat("b", 40), bytes.NewReader(pack.Bytes())); err == nil {
		t.Fatal("accepted history without the admitted source")
	}
	if err := os.WriteFile(filepath.Join(f.Dir, ".git", "shallow"), []byte(head+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteHistoryPack(f.Dir, head, &bytes.Buffer{}); err == nil {
		t.Fatal("exported shallow history")
	}
}

func TestHistoryMissingBlob(t *testing.T) {
	f := gittest.NewFixture(t)
	head := f.CommitFile(t, "delivered.txt", "delivery\n", "delivery")
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := r.repo.CommitObject(plumbing.NewHash(head))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := commit.Tree()
	if err != nil {
		t.Fatal(err)
	}
	file, err := tree.File("seed.txt")
	if err != nil {
		t.Fatal(err)
	}
	objects, err := revlist.Objects(r.repo.Storer, []plumbing.Hash{commit.Hash}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var kept []plumbing.Hash
	for _, hash := range objects {
		if hash != file.Hash {
			kept = append(kept, hash)
		}
	}
	if len(kept) != len(objects)-1 {
		t.Fatal("fixture did not omit exactly one reachable blob")
	}
	var pack bytes.Buffer
	if _, err := packfile.NewEncoder(&pack, r.repo.Storer, false).Encode(kept, 0); err != nil {
		t.Fatal(err)
	}
	if err := ReadHistoryPack(filepath.Join(t.TempDir(), "missing-blob"), head, bytes.NewReader(pack.Bytes())); err == nil {
		t.Fatal("accepted a pack missing only a reachable blob, with all commits and trees present")
	}
}

func TestHistorySourceGuards(t *testing.T) {
	f := gittest.NewFixture(t)
	base, err := f.Git("rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	head := f.CommitFile(t, "new.txt", "new\n", "head")
	if err := WriteHistoryPack(f.Dir, base, &bytes.Buffer{}); err == nil {
		t.Fatal("export accepted a source different from HEAD")
	}
	r, err := Open(f.Dir)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := r.repo.CommitObject(plumbing.NewHash(head))
	if err != nil {
		t.Fatal(err)
	}
	tree, err := commit.Tree()
	if err != nil {
		t.Fatal(err)
	}
	file, err := tree.File("new.txt")
	if err != nil {
		t.Fatal(err)
	}
	var pack bytes.Buffer
	if _, err := packfile.NewEncoder(&pack, r.repo.Storer, false).Encode([]plumbing.Hash{file.Hash}, 0); err != nil {
		t.Fatal(err)
	}
	if err := ReadHistoryPack(filepath.Join(t.TempDir(), "blob-source"), file.Hash.String(), bytes.NewReader(pack.Bytes())); err == nil {
		t.Fatal("import accepted a blob as its source commit")
	}
}
