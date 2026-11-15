package deskkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// regression: #5704
func TestWorkSourceSnapshot(t *testing.T) {
	root := t.TempDir()
	repo, err := git.PlainInit(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{"https://github.com/owner/repo.git"}}); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"docs/streams/graph-repos.yaml":        "schema: graph-repos-v1\ncell: cell\nrepos:\n  repo: {cell: cell, repo: owner/repo}\n",
		"docs/streams/stream/brief-08-work.md": "---\nbrief: cell:repo:stream:08\nid: 11cf440e-4e8e-4e51-98d6-211210dbefab\nversion: 2\nschema: brief-v2\n---\n# Work\n",
	}
	tree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		file := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := tree.Add(name); err != nil {
			t.Fatal(err)
		}
	}
	commit, err := tree.Commit("fixture", &git.CommitOptions{Author: &object.Signature{Name: "Fixture", Email: "fixture@example.invalid", When: time.Unix(1000, 0)}})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := LocalWorkCatalog(root, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	selected, err := catalog.Select("stream:b-08@2")
	if err != nil {
		t.Fatal(err)
	}
	if selected.Subject.Commit != commit.String() || selected.ClaimKey != "repo--stream--08" {
		t.Fatal("source not pinned")
	}
	// Host adapter must read immutable source, not silently substitute working bytes.
	if err := os.WriteFile(filepath.Join(root, "docs/streams/stream/brief-08-work.md"), []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err = LocalWorkCatalog(root, "owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = catalog.Select("stream:b-08@2"); err != nil {
		t.Fatal(err)
	}
	// Actual writer gate resolves the source and then reports unqualified rollout.
	if err = CheckLocalWorkTrailer([]byte("Brief: stream:b-08@2"), root); err == nil || !strings.Contains(err.Error(), "complete participating") {
		t.Fatalf("unqualified operation activated or resolver not reached: %v", err)
	}
}
