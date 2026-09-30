package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// remoteurl_callers_test.go — the CLASS guard for the defect #1617 fixes in effectiveOriginURL.
//
// The defect class: an allow/deny gate decides a remote's identity from go-git's
// (*gitcore.Repo).RemoteURL — which reads only the repository's own config file — while the git
// verb the gate protects resolves the remote ITSELF (worktree- and global-scope values, empty-value
// list resets, insteadOf / pushInsteadOf rewrites, pushurl). The two reads can disagree, so the
// gate passes an allowed repo while git connects to another one.
//
// The per-site tests in origin_resolution_test.go prove the ONE site this PR fixes. This test is
// the independent layer: a source-tree scan that enumerates every RemoteURL caller in the desk
// tree and fails on any caller not on the allow-list below. A new caller has to be looked at and
// ruled on here (and its reason written) before it can land; a listed caller that disappears has
// to be removed, so the list cannot go stale and keep a fixed site looking open.

// remoteURLAllowList maps each file (relative to tools/desk) that calls RemoteURL to the number of
// calls it makes and WHY each is on the list. "OPEN class site" marks a gate that git then
// contradicts, still to be fixed; remove it here in the change that fixes it.
//
// #1623 fixed and removed deskpr (preflight now gates on `git remote get-url --all origin`, and
// create/update gate the push on `git remote get-url --push --all origin`) and deskmerge
// (resolveRepoRoot gates the fetch on git's resolution; the merge gates its push destinations in
// the scratch worktree the push leaves from). It also assessed the sites below and the one site
// this scan cannot see:
//
//   - deskclaim-ref (cmd/deskclaim-ref/gogit.go, originRemoteURL — go-git's Remote().Config(),
//     not a RemoteURL call, so the scan does not count it): NOT a gate git contradicts. The
//     origin read supplies only the HOST hint and the default slug; the tool then dials
//     `https://<host>/<owner>/<name>.git` itself, built from deskkit.ForgeKindFromSlugAndHost,
//     through go-git's own transport. No git verb resolves origin after the read, so there is no
//     second resolution to disagree with it.
var remoteURLAllowList = map[string]struct {
	calls  int
	reason string
}{
	"cmd/deskwt/deskwt.go": {1, "OPEN class site (assessed #1623, fix owned by the deskwt-side " +
		"work, not this change): currentRepo gates IsAllowedRepo, and role-init then runs " +
		"`git fetch --no-tags origin main`, which resolves origin itself (worktree/global scope, " +
		"insteadOf)."},
	"cmd/deskreply/deskreply.go": {1, "assessed #1623 — NOT a gate git contradicts: preflight " +
		"gates IsAllowedRepo for a forge-API reply, and no git verb that resolves origin runs " +
		"after it (the reply goes through the forge API, never git transport)."},
	"cmd/deskpushguard/main.go": {1, "fallback only when the pre-push hook received no URL " +
		"argument; the normal path takes the URL git itself hands the hook."},
	"internal/deskkit/remoterepo.go": {1, "OriginRepoSlug reads the RAW configured value on " +
		"purpose, so the identity is not insteadOf-expanded (see its doc comment)."},
	"internal/deskkit/preflight.go": {2, "deriveRepoSlug only labels the mint's owner; the landing " +
		"probe only checks the remote exists, and its `git push --dry-run` resolves the remote itself."},
}

// remoteURLCallers walks root's cmd/ and internal/ trees (non-test Go files, testdata skipped,
// the defining gitcore package excluded) and counts the `.RemoteURL(` method calls per file,
// keyed relative to root with forward slashes. It returns the counts and the number of files it
// parsed, so a caller can tell "found none" from "looked at nothing".
func remoteURLCallers(root string) (map[string]int, int, error) {
	hits := map[string]int{}
	scanned := 0
	fset := token.NewFileSet()
	for _, top := range []string{"cmd", "internal"} {
		base := filepath.Join(root, top)
		if _, err := os.Stat(base); err != nil {
			return nil, 0, err
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if d.Name() == "testdata" || rel == "internal/gitcore" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			af, perr := parser.ParseFile(fset, path, nil, 0) // no ParseComments: only code decides
			if perr != nil {
				return perr
			}
			scanned++
			ast.Inspect(af, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "RemoteURL" {
					hits[rel]++
				}
				return true
			})
			return nil
		})
		if err != nil {
			return nil, 0, err
		}
	}
	return hits, scanned, nil
}

func TestRemoteURLCallers_AllowListed(t *testing.T) {
	root := filepath.Join("..", "..") // tools/desk, from cmd/deskgit
	if _, err := os.Stat(filepath.Join(root, "internal", "gitcore", "gitcore.go")); err != nil {
		t.Fatalf("cannot find the desk tree root at %s: %v", root, err)
	}
	hits, scanned, err := remoteURLCallers(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if scanned == 0 {
		t.Fatal("scanned 0 source files — the enumeration proved nothing")
	}

	var problems []string
	for rel, n := range hits {
		want, listed := remoteURLAllowList[rel]
		switch {
		case !listed:
			problems = append(problems, rel+": a NEW (*gitcore.Repo).RemoteURL caller. RemoteURL reads "+
				"only the repository config file; if this read gates a git verb, gate on what git "+
				"itself resolves instead (see effectiveOriginURL), or add the file to "+
				"remoteURLAllowList with the reason it is not such a gate")
		case n != want.calls:
			problems = append(problems, rel+": "+strconv.Itoa(n)+" RemoteURL call(s), allow-list says "+
				strconv.Itoa(want.calls)+" — rule on the change and update the entry")
		}
	}
	for rel := range remoteURLAllowList {
		if hits[rel] == 0 {
			problems = append(problems, rel+": on the allow-list but no longer calls RemoteURL — "+
				"remove the stale entry")
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("RemoteURL caller allow-list is out of step with the tree:\n  %s",
			strings.Join(problems, "\n  "))
	}
}

// The scanner itself is not vacuous: a planted caller in a synthetic tree is found, a test file
// and a testdata file are not, and a call spelled through a different receiver still counts.
func TestRemoteURLCallers_DetectsPlantedCaller(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("cmd/planted/planted.go", "package main\n\nfunc gate(r interface{ RemoteURL(string) (string, error) }) {\n\tu, _ := r.RemoteURL(\"origin\")\n\t_ = u\n}\n")
	write("cmd/planted/planted_test.go", "package main\n\nfunc x(r interface{ RemoteURL(string) (string, error) }) { r.RemoteURL(\"origin\") }\n")
	write("cmd/planted/testdata/fixture.go", "package fixture\n\nfunc y(r interface{ RemoteURL(string) (string, error) }) { r.RemoteURL(\"origin\") }\n")
	write("internal/other/other.go", "package other\n\nfunc z(repo interface{ RemoteURL(string) (string, error) }) { repo.RemoteURL(\"upstream\") }\n")
	write("internal/gitcore/gitcore.go", "package gitcore\n\nfunc w(r interface{ RemoteURL(string) (string, error) }) { r.RemoteURL(\"origin\") }\n")

	hits, scanned, err := remoteURLCallers(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	want := map[string]int{"cmd/planted/planted.go": 1, "internal/other/other.go": 1}
	if scanned != 2 {
		t.Fatalf("scanned %d files, want 2 (test files, testdata and gitcore excluded)", scanned)
	}
	if len(hits) != len(want) {
		t.Fatalf("hits = %v, want %v", hits, want)
	}
	for rel, n := range want {
		if hits[rel] != n {
			t.Fatalf("hits = %v, want %v", hits, want)
		}
	}
}
