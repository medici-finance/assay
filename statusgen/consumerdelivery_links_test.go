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
		"aliases/view": "../web", "aliases/file": "../web/fixed.go", "aliases/chain": "view", "aliases/parent": "view/../web/fixed.go", "aliases/repeated": "view/../aliases/view/fixed.go",
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
func TestDeliveryAliasGlob(t *testing.T) {
	for _, tc := range []struct {
		name, route string
		partial     bool
		want        int
	}{
		{"tracked only", "*/fixed.go: fixed-here", false, 0},
		{"partial set", "*/fixed.go: fixed-here", true, 1},
		{"excluded but changed", "*/fixed.go: out-of-scope (handled by another delivery)", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _, _ := deliveryFixture(t, []string{tc.route})
			if err := os.Symlink("web", filepath.Join(root, "alt")); err != nil {
				t.Skipf("host cannot create symlink fixture: %v", err)
			}
			if tc.partial {
				mustWrite(t, filepath.Join(root, "api/fixed.go"), "untouched")
			}
			mustGitCommitAll(t, root, "add unchanged alias")
			base := mustGitOut(t, root, "rev-parse", "HEAD")
			mustWrite(t, filepath.Join(root, "web/fixed.go"), "delivery\n")
			mustGitCommitAll(t, root, "change tracked consumer")
			if got := runDeliveryConsumers(root, base, "alpha/01", mustGitOut(t, root, "rev-parse", "HEAD")); got != tc.want {
				t.Fatalf("delivery route got exit %d, want %d", got, tc.want)
			}
		})
	}
}

// regression: #5395
func TestSnapshotVirtualLinks(t *testing.T) {
	parent := t.TempDir()
	dest := filepath.Join(parent, "snapshot")
	if err := os.Mkdir(dest, 0o700); err != nil {
		t.Fatal(err)
	}
	canary := filepath.Join(parent, "outside")
	mustWrite(t, canary, "unchanged")
	entries := []evaluation.TreeEntry{
		{Mode: "100644", Path: "web/fixed.go"},
		{Mode: "120000", Path: "alt"},
		{Mode: "120000", Path: "nested/view"},
		{Mode: "100644", Path: "ALT/other.go"},
	}
	var archive bytes.Buffer
	w := tar.NewWriter(&archive)
	for _, h := range []*tar.Header{
		{Name: "web/fixed.go", Typeflag: tar.TypeReg, Size: 1, Mode: 0o644},
		{Name: "alt", Typeflag: tar.TypeSymlink, Linkname: "web"},
		{Name: "nested/view", Typeflag: tar.TypeSymlink, Linkname: "../web/fixed.go"},
		{Name: "ALT/other.go", Typeflag: tar.TypeReg, Size: 1, Mode: 0o644},
	} {
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if h.Size > 0 {
			if _, err := w.Write([]byte("x")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := newConsumerSnapshot(entries, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.extract(&archive, dest); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(dest, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			t.Errorf("snapshot materialized a link: %s", p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(canary)
	if err != nil || string(got) != "unchanged" {
		t.Fatalf("outside canary changed: %q %v", got, err)
	}
	siblings, err := os.ReadDir(parent)
	if err != nil || len(siblings) != 2 {
		t.Fatalf("unexpected outside entries: %v %v", siblings, err)
	}
}

// regression: #5395
func TestSnapshotSiteRoster(t *testing.T) {
	s, err := newConsumerSnapshot([]evaluation.TreeEntry{
		{Mode: "100644", Path: "web/fixed.go"},
		{Mode: "100644", Path: "api/fixed.go"},
		{Mode: "120000", Path: "alt"},
		{Mode: "120000", Path: "link.go"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ pattern, want string }{
		{"*/fixed.go", "api/fixed.go,web/fixed.go"},
		{"{web,api}/fixed.go", "api/fixed.go,web/fixed.go"},
		{"alt/fixed.go", ""},
		{"link.go", "link.go"},
		{"*", "alt,api,link.go,web"},
		{"web/", "web"},
		{"[/fixed.go", ""},
	} {
		got, err := s.resolveSitePath("", tc.pattern)
		if err != nil || strings.Join(got, ",") != tc.want {
			t.Errorf("%q: got %v, %v; want %q", tc.pattern, got, err, tc.want)
		}
	}
	if _, err := s.resolveSitePath("", "../outside"); err == nil {
		t.Fatal("escape accepted")
	}
}

// regression: #5395
func TestSnapshotBoardCase(t *testing.T) {
	dest := t.TempDir()
	mustWrite(t, filepath.Join(dest, "docs/streams/a/readme.md"), "x")
	s, err := newConsumerSnapshot([]evaluation.TreeEntry{{Mode: "100644", Path: "docs/streams/a/readme.md"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, lookupErr := os.Stat(filepath.Join(dest, "docs/streams/a/README.md"))
	t.Logf("case-folding lookup available: %t", lookupErr == nil)
	err = s.checkExtracted(dest)
	if lookupErr == nil && err == nil {
		t.Fatal("case-aliased board input accepted")
	}
	if os.IsNotExist(lookupErr) && err != nil {
		t.Fatalf("distinct board name refused: %v", err)
	}
}

// regression: #5395
func TestSnapshotRosterMismatch(t *testing.T) {
	dest := t.TempDir()
	mustWrite(t, filepath.Join(dest, "Dir", "one"), "a")
	s, err := newConsumerSnapshot([]evaluation.TreeEntry{
		{Mode: "100644", Path: "Dir/one"},
		{Mode: "100644", Path: "dir/two"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.checkExtracted(dest); err == nil {
		t.Fatal("incomplete filesystem roster accepted")
	}
	mustWrite(t, filepath.Join(dest, "dir", "two"), "b")
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	err = s.checkExtracted(dest)
	if len(entries) == 1 && err == nil {
		t.Fatal("folded filesystem directories accepted")
	}
	if len(entries) == 2 && err != nil {
		t.Fatalf("distinct directories refused: %v", err)
	}
}

// regression: #5395
func TestSnapshotResolveGuards(t *testing.T) {
	for _, tc := range []struct{ name, target, reason string }{
		{"root escape", "../web", "escapes snapshot"},
		{"absolute", "/web", "absolute or unsupported"},
		{"drive", "C:/web", "absolute or unsupported"},
		{"missing", "absent", "missing delivery link target"},
		{"file parent", "web/fixed.go/..", "non-directory"},
		{"cycle", "alias", "cyclic or excessive-depth"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := newConsumerSnapshot([]evaluation.TreeEntry{{Mode: "100644", Path: "web/fixed.go"}, {Mode: "120000", Path: "alias"}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			s.links["alias"] = tc.target
			if _, err := s.resolve("alias"); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("expected %q refusal, got %v", tc.reason, err)
			}
		})
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
		{"omitted link", []evaluation.TreeEntry{{Mode: "100644", Path: "safe"}, {Mode: "120000", Path: "alias"}}, []*tar.Header{regular("safe")}},
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
