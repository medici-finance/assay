package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/workspace"
	"github.com/medici-finance/assay/workspace/local"
)

// Disposable local origins only: no forge, no network, no credential.

func wsGitEnv() []string {
	env := []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
		"GIT_COMMITTER_NAME=fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid"}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			env = append(env, kv)
		}
	}
	return env
}

func wsGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = wsGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func wsOrigin(t *testing.T, base, name string) (string, string) {
	t.Helper()
	seed := filepath.Join(base, name+"-seed")
	wsGit(t, base, "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wsGit(t, seed, "add", "README")
	wsGit(t, seed, "commit", "-q", "-m", "seed "+name)
	bare := filepath.Join(base, name+".git")
	wsGit(t, base, "clone", "-q", "--bare", seed, bare)
	return bare, wsGit(t, seed, "rev-parse", "HEAD")
}

// wsTransport clones a disposable origin per repository; fail names repositories whose clone
// fails, the way a forge outage or a revoked grant would.
type wsTransport struct {
	origins map[workspace.RepoRef]string
	fail    map[workspace.RepoRef]bool
	calls   int
}

func (w *wsTransport) Clone(_ context.Context, repo workspace.RepoRef, dest string) error {
	w.calls++
	if w.fail[repo] {
		return errors.New("transport unavailable")
	}
	src, ok := w.origins[repo]
	if !ok {
		return errors.New("no origin")
	}
	cmd := exec.Command("git", "clone", "-q", "--no-local", src, dest)
	cmd.Env = wsGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(string(out))
	}
	return nil
}

func wsRef(t *testing.T, forge, host, spec string) workspace.RepoRef {
	t.Helper()
	r, err := workspaceRepoRef(forge, host, spec)
	if err != nil {
		t.Fatalf("repo ref %s %s %s: %v", forge, host, spec, err)
	}
	return r
}

func TestWorkspaceRootMapAtomic(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()

	// Identity: case and remote aliases fold to one repository; owner and forge host do not.
	tools := wsRef(t, "github", "github.com", "acme/tools")
	for _, alias := range []string{"Acme/Tools", "https://github.com/acme/tools.git", "git@github.com:ACME/tools.git"} {
		if got := wsRef(t, "github", "github.com", alias); got != tools {
			t.Fatalf("alias %s resolves to %+v, want %+v", alias, got, tools)
		}
	}
	otherOwner := wsRef(t, "github", "github.com", "other/tools")
	otherForge := wsRef(t, "gitlab", "gitlab.example", "acme/tools")
	keys := map[string]bool{workspace.RepoKey(tools): true, workspace.RepoKey(otherOwner): true, workspace.RepoKey(otherForge): true}
	if len(keys) != 3 {
		t.Fatalf("acme/tools, other/tools and a second forge's acme/tools share a store key: %v", keys)
	}
	if _, err := workspaceRepoRef("bitbucket", "bitbucket.org", "acme/tools"); err == nil {
		t.Fatal("a forge with no identity rule was given a repository identity")
	}

	// The cell: two legacy checkouts in its stream-root map, and its legacy cell.env.
	cellDir := filepath.Join(base, "cells", "house")
	legacyA := filepath.Join(base, "checkouts", "tools")
	legacyB := filepath.Join(base, "checkouts", "docs")
	for _, d := range []string{filepath.Join(cellDir, "worktrees"), legacyA, legacyB} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cellEnv := filepath.Join(cellDir, "cell.env")
	envBytes := []byte("CELL_REPO=" + legacyA + "\nCELL_ROOTS=acme/tools=" + legacyA + ",acme/docs=" + legacyB + "\n")
	if err := os.WriteFile(cellEnv, envBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Env{vals: map[string]string{}, set: map[string]bool{}, src: map[string]string{}}
	e.Put("CELL_REPO", legacyA)
	e.Put("CELL_ROOTS", "acme/tools="+legacyA+",acme/docs="+legacyB)
	legacy := workspaceLegacyScope(e, cellDir)
	scope := workspaceStoreScope{
		Cell:     workspace.CellRef{UUID: "00000000-0000-4000-8000-000000000090", Root: cellDir},
		Registry: filepath.Join(base, "registry"),
		Legacy:   legacy,
	}

	toolsSrc, toolsAt := wsOrigin(t, base, "tools")
	docsSrc, docsAt := wsOrigin(t, base, "docs")
	docs := wsRef(t, "github", "github.com", "Acme/Docs")
	tr := &wsTransport{origins: map[workspace.RepoRef]string{tools: toolsSrc, docs: docsSrc}, fail: map[workspace.RepoRef]bool{docs: true}}
	stores := local.Stores{Transport: tr}
	reqs := []workspace.StoreRequest{{Repo: tools, Subject: toolsAt}, {Repo: docs, Subject: docsAt},
		{Repo: wsRef(t, "github", "github.com", "https://github.com/ACME/tools"), Subject: toolsAt}}

	noMapping := func(what string) {
		t.Helper()
		if _, err := os.Lstat(workspace.MappingPath(scope.Cell)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s: a mapping was published (%v)", what, err)
		}
	}

	// One clone fails: per-repository outcomes, and no mapping at all.
	_, outcomes, err := publishWorkspaceStores(ctx, stores, scope, reqs, 0)
	if err == nil {
		t.Fatal("a failed clone still published")
	}
	if len(outcomes) != 2 {
		t.Fatalf("aliases were not folded to one store: %d outcomes", len(outcomes))
	}
	for _, o := range outcomes {
		if failed := o.Err != nil; failed != (o.Repo == docs) {
			t.Fatalf("outcome for %s: %v", o.Repo.Repository, o.Err)
		}
		if o.Repo == docs {
			wsWantReason(t, o.Err, workspace.ReasonEvidenceUnavailable)
		}
	}
	noMapping("failed second clone")

	// A wrong repository behind the transport: refused, still nothing published.
	tr.fail = nil
	tr.origins[docs] = toolsSrc
	if _, _, err := publishWorkspaceStores(ctx, stores, scope, reqs, 0); err == nil {
		t.Fatal("a store holding the wrong repository was published")
	}
	noMapping("wrong repository")

	// Retry once the transport is right: generation 1, every store, no change to cell.env.
	tr.origins[docs] = docsSrc
	m, _, err := publishWorkspaceStores(ctx, stores, scope, reqs, 0)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if m.Generation != 1 || len(m.Stores) != 2 {
		t.Fatalf("published generation %d with %d stores", m.Generation, len(m.Stores))
	}
	b, err := os.ReadFile(workspace.MappingPath(scope.Cell))
	if err != nil {
		t.Fatal(err)
	}
	if back, err := workspace.DecodeStoreMapping(b); err != nil || back.Generation != 1 {
		t.Fatalf("published mapping does not decode: %v", err)
	}
	if got, _ := os.ReadFile(cellEnv); !bytes.Equal(got, envBytes) {
		t.Fatalf("publication changed cell.env:\n%s", got)
	}
	if bytes.Contains(b, []byte("CELL_REPO")) || bytes.Contains(b, []byte("CELL_ROOTS")) {
		t.Fatal("the mapping is exported under a legacy key")
	}

	// Every published store is physically disjoint from every legacy root and checkout, by
	// resolved path and by Git common directory.
	for _, s := range m.Stores {
		for _, l := range append(append([]string(nil), legacy.Checkouts...), legacy.Cleanup...) {
			if strings.ContainsAny(l, "*?[") {
				if ok, _ := filepath.Match(l, s.Path); ok {
					t.Fatalf("store %s matches legacy pattern %s", s.Path, l)
				}
				continue
			}
			r, err := filepath.EvalSymlinks(l)
			if err != nil {
				continue
			}
			if s.Path == r || strings.HasPrefix(s.Path, r+string(filepath.Separator)) || strings.HasPrefix(r, s.Path+string(filepath.Separator)) {
				t.Fatalf("store %s overlaps legacy root %s", s.Path, r)
			}
		}
		if wsGit(t, s.Path, "rev-parse", "--path-format=absolute", "--git-common-dir") != s.CommonDir {
			t.Fatalf("store %s does not resolve to its own common dir", s.Path)
		}
	}

	// A stale generation is refused and leaves generation 1 in place.
	if _, _, err := publishWorkspaceStores(ctx, stores, scope, reqs, 0); err == nil {
		t.Fatal("a stale generation was published")
	} else {
		wsWantReason(t, err, workspace.ReasonStaleGeneration)
	}
	if m2, _, err := publishWorkspaceStores(ctx, stores, scope, reqs, 1); err != nil || m2.Generation != 2 {
		t.Fatalf("next generation: %+v, %v", m2.Generation, err)
	}

	// A legacy checkout that resolves to a store: the next publication is refused whole.
	link := filepath.Join(base, "checkouts", "aliased")
	if err := os.Symlink(m.Stores[0].Path, link); err != nil {
		t.Fatal(err)
	}
	e.Put("CELL_ROOTS", e.Get("CELL_ROOTS")+",acme/aliased="+link)
	scope.Legacy = workspaceLegacyScope(e, cellDir)
	before, _ := os.ReadFile(workspace.MappingPath(scope.Cell))
	if _, _, err := publishWorkspaceStores(ctx, stores, scope, reqs, 2); err == nil {
		t.Fatal("a store inside a legacy checkout was republished")
	} else {
		wsWantReason(t, err, workspace.ReasonIdentityMismatch)
	}
	if after, _ := os.ReadFile(workspace.MappingPath(scope.Cell)); !bytes.Equal(before, after) {
		t.Fatal("a refused publication changed the published mapping")
	}
}

func wsWantReason(t *testing.T, err error, want workspace.Reason) {
	t.Helper()
	var r *workspace.Refusal
	if !errors.As(err, &r) || r.Reason != want {
		t.Fatalf("want %s, got %v", want, err)
	}
}
