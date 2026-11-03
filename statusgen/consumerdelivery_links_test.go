package main

import (
	"archive/tar"
	"bytes"
	"strings"

	"github.com/medici-finance/assay/statusgen/evaluation"
	"os"
	"path/filepath"
	"testing"
)

func linkedConsumerDelivery(t *testing.T, links map[string]string) (string, string, string) {
	t.Helper()
	root, _, _ := deliveryFixture(t, []string{"web/fixed.go: fixed-here"})
	for name, target := range links {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, p); err != nil {
			t.Skipf("host cannot create symlink fixture: %v", err)
		}
	}
	mustGitCommitAll(t, root, "links precede delivery")
	base := mustGitOut(t, root, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "second delivery\n")
	mustGitCommitAll(t, root, "regular consumer delivery")
	return root, base, mustGitOut(t, root, "rev-parse", "HEAD")
}

// regression: #5395
func TestDeliveryUnchangedContainedLinks(t *testing.T) {
	root, base, delivery := linkedConsumerDelivery(t, map[string]string{
		"aliases/view": "../web", "aliases/file": "../web/fixed.go", "aliases/chain": "view", "aliases/parent": "view/../web/fixed.go",
	})
	if got := runDeliveryConsumers(root, base, "alpha/01", delivery); got != 0 {
		t.Fatalf("unchanged contained links prevented immutable consumer evaluation: exit %d", got)
	}
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "dirty local payload\n")
	if got := runDeliveryConsumers(root, base, "alpha/01", delivery); got != 0 {
		t.Fatalf("working tree replaced the immutable snapshot: exit %d", got)
	}
}

// regression: #5395
func TestDeliveryUnsafeUnchangedLinks(t *testing.T) {
	cases := map[string]map[string]string{
		"relative escape": {"aliases/link": "../../outside"},
		"absolute escape": {"aliases/link": "/outside"},
		"missing target":  {"aliases/link": "../missing"},
		"cycle":           {"aliases/a": "b", "aliases/b": "a"},
		"self cycle":      {"aliases/a": "a"},
		"chain escape":    {"aliases/a": "b", "aliases/b": "../../outside"},
		// Lexical cleaning would turn pivot/../outside into an in-tree path.
		// Expanding pivot first instead exposes the actual escape.
		"parent after alias": {"aliases/pivot": "..", "aliases/link": "pivot/../outside"},
		"file parent":        {"aliases/link": "../web/fixed.go/../fixed.go"},
		"foreign separator":  {"aliases/link": `..\web`},
		"drive target":       {"aliases/link": "C:/outside"},
	}
	for name, links := range cases {
		t.Run(name, func(t *testing.T) {
			root, base, delivery := linkedConsumerDelivery(t, links)
			if got := runDeliveryConsumers(root, base, "alpha/01", delivery); got != 2 {
				t.Fatalf("unsafe link must remain could-not-check, got %d", got)
			}
		})
	}
}

func TestDeliveryChangedLinkStillRefuses(t *testing.T) {
	root, _, _ := linkedConsumerDelivery(t, map[string]string{"aliases/view": "../web"})
	base := mustGitOut(t, root, "rev-parse", "HEAD")
	p := filepath.Join(root, "aliases/view")
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../web/fixed.go", p); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "changed together with link\n")
	mustGitCommitAll(t, root, "change the link and its consumer")
	if got := runDeliveryConsumers(root, base, "alpha/01", mustGitOut(t, root, "rev-parse", "HEAD")); got != 2 {
		t.Fatalf("changed link was accepted: exit %d", got)
	}
}

func TestDeliveryBoardLinkStillRefuses(t *testing.T) {
	root, _, _ := deliveryFixture(t, []string{"web/fixed.go: fixed-here"})
	index := filepath.Join(root, "docs/streams/alpha/README.md")
	original := filepath.Join(root, "docs/original-index.md")
	if err := os.Rename(index, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../original-index.md", index); err != nil {
		t.Skipf("host cannot create symlink fixture: %v", err)
	}
	mustGitCommitAll(t, root, "board link precedes delivery")
	base := mustGitOut(t, root, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(root, "web/fixed.go"), "new delivery\n")
	mustGitCommitAll(t, root, "regular delivery")
	if got := runDeliveryConsumers(root, base, "alpha/01", mustGitOut(t, root, "rev-parse", "HEAD")); got != 2 {
		t.Fatalf("linked board input was accepted: exit %d", got)
	}
}

func TestDeliverySubmoduleStillRefuses(t *testing.T) {
	root, _, _ := deliveryFixture(t, []string{"web/fixed.go: fixed-here"})
	base := mustGitOut(t, root, "rev-parse", "HEAD")
	runGit(t, root, "update-index", "--add", "--cacheinfo", "160000,"+base+",module")
	runGit(t, root, "commit", "-qm", "submodule delivery")
	if got := runDeliveryConsumers(root, base, "alpha/01", mustGitOut(t, root, "rev-parse", "HEAD")); got != 2 {
		t.Fatalf("submodule was accepted: exit %d", got)
	}
}

func TestConsumerArchiveRefusals(t *testing.T) {
	regular := func(name string) *tar.Header {
		return &tar.Header{Name: name, Typeflag: tar.TypeReg, Size: 1, Mode: 0o644}
	}
	cases := []struct {
		name    string
		modes   []evaluation.TreeEntry
		headers []*tar.Header
	}{
		{"traversal", []evaluation.TreeEntry{{Mode: "100644", Path: "safe"}}, []*tar.Header{regular("../outside")}},
		{"absolute", []evaluation.TreeEntry{{Mode: "100644", Path: "safe"}}, []*tar.Header{regular("/outside")}},
		{"untracked", []evaluation.TreeEntry{{Mode: "100644", Path: "safe"}}, []*tar.Header{regular("extra")}},
		{"duplicate", []evaluation.TreeEntry{{Mode: "100644", Path: "safe"}}, []*tar.Header{regular("safe"), regular("safe")}},
		{"hardlink", []evaluation.TreeEntry{{Mode: "100644", Path: "safe"}}, []*tar.Header{{Name: "safe", Typeflag: tar.TypeLink, Linkname: "../outside"}}},
		{"wrong type", []evaluation.TreeEntry{{Mode: "100644", Path: "safe"}}, []*tar.Header{{Name: "safe", Typeflag: tar.TypeSymlink, Linkname: "../outside"}}},
		{"escaping link", []evaluation.TreeEntry{{Mode: "120000", Path: "alias"}}, []*tar.Header{{Name: "alias", Typeflag: tar.TypeSymlink, Linkname: "../outside"}}},
		{"omitted", []evaluation.TreeEntry{{Mode: "100644", Path: "safe"}, {Mode: "100644", Path: "missing"}}, []*tar.Header{regular("safe")}},
		{"metadata path", []evaluation.TreeEntry{{Mode: "100644", Path: ".git/config"}}, []*tar.Header{regular(".git/config")}},
		{"link parent", []evaluation.TreeEntry{{Mode: "120000", Path: "alias"}, {Mode: "100644", Path: "alias/file"}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			dest := filepath.Join(parent, "snapshot")
			if err := os.Mkdir(dest, 0o700); err != nil {
				t.Fatal(err)
			}
			canary := filepath.Join(parent, "outside")
			mustWrite(t, canary, "unchanged")
			var archive bytes.Buffer
			writer := tar.NewWriter(&archive)
			for _, h := range tc.headers {
				if err := writer.WriteHeader(h); err != nil {
					t.Fatal(err)
				}
				if h.Size != 0 {
					if _, err := writer.Write([]byte("x")); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			snapshot, err := newConsumerSnapshot(tc.modes, nil)
			if err == nil {
				err = snapshot.extract(bytes.NewReader(archive.Bytes()), dest)
			}
			if err == nil {
				t.Fatal("unsafe or incomplete archive was accepted")
			}
			if data, err := os.ReadFile(canary); err != nil || string(data) != "unchanged" {
				t.Fatalf("outside canary changed: %q %v", data, err)
			}
			if err := filepath.WalkDir(dest, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.Type()&os.ModeSymlink != 0 {
					t.Fatalf("unsafe archive created a link: %s", p)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestConsumerSnapshotBoardAndChangedLinkPreflight(t *testing.T) {
	for _, p := range []string{"docs", "docs/streams", "docs/archive", "docs/streams/alpha/README.md", "DOCS/streams/alpha/README.md"} {
		if _, err := newConsumerSnapshot([]evaluation.TreeEntry{{Path: p, Mode: "120000"}}, nil); err == nil {
			t.Fatalf("board link accepted: %s", p)
		}
	}
	if _, err := newConsumerSnapshot([]evaluation.TreeEntry{{Path: "alias", Mode: "120000"}}, map[string]bool{"alias": true}); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("changed link was not refused before extraction: %v", err)
	}
}
