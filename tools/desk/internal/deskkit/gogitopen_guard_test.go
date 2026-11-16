package deskkit

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// goGitRoot is the go-git package that holds PlainOpen, PlainOpenWithOptions and Open.
// Its repository opener refuses any config carrying extensions.worktreeConfig, which git
// sets wherever per-worktree config is in use (#6606). Production code in this module opens
// repositories through gitcore (Toplevel + Open), the one opener that tolerates that
// extension and still refuses any other unsupported one. Importing this package from
// non-test code is how a second raw opener would reappear, so it is the guarded shape.
const goGitRoot = "github.com/go-git/go-git/v5"

// rawGoGitAllowed names the only non-test files that may import goGitRoot, each with why.
// A new entry needs the same: a reader for which go-git's refusal is a deliberate fall-through
// to another reader, never a fail-closed result.
var rawGoGitAllowed = map[string]string{
	// originRemoteURL step 1: on a worktreeConfig or linked checkout go-git's refusal hands
	// the read to native git, whose insteadOf scopes TestClaimReaderCompatibility pins.
	"cmd/deskclaim-ref/gogit.go": "fall-through origin reader (#727)",
}

// rawGoGitImporters lists the non-test Go files under root that import goGitRoot.
func rawGoGitImporters(t *testing.T, root string) []string {
	t.Helper()
	var hits []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			if v, _ := strconv.Unquote(imp.Path.Value); v == goGitRoot {
				rel, _ := filepath.Rel(root, p)
				hits = append(hits, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hits
}

// regression: #6606
func TestNoRawGoGitOpener(t *testing.T) {
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(module, "go.mod")); err != nil {
		t.Fatalf("module root not found at %s: %v", module, err)
	}
	var hits []string
	for _, hit := range rawGoGitImporters(t, module) {
		if _, ok := rawGoGitAllowed[hit]; !ok {
			hits = append(hits, hit)
		}
	}
	if len(hits) != 0 {
		t.Fatalf("non-test code imports %s (raw repository opener; use gitcore.Toplevel + gitcore.Open): %v", goGitRoot, hits)
	}
}

// Positive control: the scanner must flag a planted importer under any alias, or a broken
// matcher would report the module clean.
func TestNoRawGoGitOpenerPlant(t *testing.T) {
	dir := t.TempDir()
	plants := map[string]string{
		"plain.go":   "package x\nimport \"" + goGitRoot + "\"\nvar _ = git.PlainOpen\n",
		"aliased.go": "package x\nimport gg \"" + goGitRoot + "\"\nvar _ = gg.PlainOpenWithOptions\n",
		"dot.go":     "package x\nimport . \"" + goGitRoot + "\"\nvar _ = Open\n",
		"ok_test.go": "package x\nimport \"" + goGitRoot + "\"\nvar _ = git.PlainInit\n",
		"sub.go":     "package x\nimport \"" + goGitRoot + "/plumbing\"\nvar _ plumbing.Hash\n",
	}
	for name, src := range plants {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := strings.Join(rawGoGitImporters(t, dir), ",")
	if got != "aliased.go,dot.go,plain.go" {
		t.Fatalf("scanner flagged %q, want the three planted non-test importers only", got)
	}
}
