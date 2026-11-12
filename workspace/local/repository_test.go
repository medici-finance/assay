package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/medici-finance/assay/workspace"
)

// The fixtures are disposable local repositories only: an origin, a host checkout cloned from
// it, and cells under t.TempDir(). No network, no forge, no credential.

func gitEnv() []string {
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

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// origin makes a bare origin with one commit and returns its path and that commit.
func origin(t *testing.T, base, name string) (string, string) {
	t.Helper()
	seed := filepath.Join(base, name+"-seed")
	run(t, base, "init", "-q", "-b", "main", seed)
	if err := os.WriteFile(filepath.Join(seed, "README"), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, seed, "add", "README")
	run(t, seed, "commit", "-q", "-m", "seed "+name)
	commit := run(t, seed, "rev-parse", "HEAD")
	bare := filepath.Join(base, name+".git")
	run(t, base, "clone", "-q", "--bare", seed, bare)
	return bare, commit
}

// cloneFrom is the fixture transport: an ordinary, non-local clone of a disposable origin.
type cloneFrom map[workspace.RepoRef]string

func (c cloneFrom) Clone(_ context.Context, repo workspace.RepoRef, dest string) error {
	src, ok := c[repo]
	if !ok {
		return errors.New("fixture transport: no origin for " + repo.Repository)
	}
	cmd := exec.Command("git", "clone", "-q", "--no-local", src, dest)
	cmd.Env = gitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(string(out))
	}
	return nil
}

type transportFunc func(ctx context.Context, repo workspace.RepoRef, dest string) error

func (f transportFunc) Clone(ctx context.Context, repo workspace.RepoRef, dest string) error {
	return f(ctx, repo, dest)
}

func newCell(t *testing.T, base, name, uuid string) workspace.CellRef {
	t.Helper()
	root := filepath.Join(base, name)
	if err := os.MkdirAll(filepath.Join(root, "worktrees"), 0o755); err != nil {
		t.Fatal(err)
	}
	return workspace.CellRef{UUID: uuid, Root: root}
}

// state is everything a provisioning in another cell must leave alone: the configuration
// bytes, every ref, the worktree registrations and HEAD.
func state(t *testing.T, dir string) string {
	t.Helper()
	cfg, err := os.ReadFile(filepath.Join(dir, ".git", "config"))
	if err != nil {
		t.Fatal(err)
	}
	return string(cfg) + "\n--refs\n" + run(t, dir, "for-each-ref", "--format=%(refname) %(objectname)") +
		"\n--worktrees\n" + run(t, dir, "worktree", "list", "--porcelain") + "\n--head\n" + run(t, dir, "rev-parse", "HEAD")
}

func wantRefusal(t *testing.T, what string, err error, reason workspace.Reason, field string) {
	t.Helper()
	var r *workspace.Refusal
	if !errors.As(err, &r) {
		t.Fatalf("%s: want %s at %s, got %v", what, reason, field, err)
	}
	if r.Reason != reason || r.Field != field {
		t.Fatalf("%s: want %s at %s, got %s at %s (%s)", what, reason, field, r.Reason, r.Field, r.Msg)
	}
}

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		out := filepath.Join(dst, rel)
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			l, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(l, out)
		case d.IsDir():
			return os.MkdirAll(out, info.Mode().Perm()|0o700)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, in); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	})
	if err != nil {
		t.Fatal(err)
	}
}

// linkIntoHost clones normally, then replaces one entry of the clone's Git directory with a
// symlink to the same entry of the host checkout: the store then reads, and writes, the host's.
func linkIntoHost(ctx context.Context, c cloneFrom, r workspace.RepoRef, d, host, entry string) error {
	if err := c.Clone(ctx, r, d); err != nil {
		return err
	}
	own := filepath.Join(d, ".git", entry)
	if err := os.RemoveAll(own); err != nil {
		return err
	}
	return os.Symlink(filepath.Join(host, ".git", entry), own)
}

// regression: F-store-symlinked-git-internals
func TestCellRepositoryIsolation(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	repo := workspace.RepoRef{Forge: "forge.example", Repository: "acme/tools"}
	src, commit := origin(t, base, "tools")
	other, _ := origin(t, base, "other")

	// The host checkout: the operator's own clone, with the configuration a real one carries.
	host := filepath.Join(base, "host")
	run(t, base, "clone", "-q", src, host)
	run(t, host, "config", "user.name", "host operator")
	run(t, host, "config", "credential.helper", "host-helper")

	cellA := newCell(t, base, "cell-a", "00000000-0000-4000-8000-000000000010")
	cellB := newCell(t, base, "cell-b", "00000000-0000-4000-8000-000000000020")
	legacy := Legacy{Checkouts: []string{host}, Cleanup: []string{filepath.Join(cellA.Root, "worktrees"), filepath.Join(cellB.Root, "worktrees")}}
	req := workspace.StoreRequest{Repo: repo, Subject: commit}
	s := Stores{Transport: cloneFrom{repo: src}}

	hostBefore := state(t, host)
	a, err := s.Provision(ctx, cellA, req, legacy)
	if err != nil {
		t.Fatalf("provision A: %v", err)
	}
	b, err := s.Provision(ctx, cellB, req, legacy)
	if err != nil {
		t.Fatalf("provision B: %v", err)
	}

	// Two stores of one repository: each its own common directory, at the layout's path.
	for _, e := range []struct {
		cell  workspace.CellRef
		entry workspace.StoreEntry
	}{{cellA, a}, {cellB, b}} {
		want, _ := filepath.EvalSymlinks(filepath.Dir(workspace.StorePath(e.cell, repo)))
		if e.entry.Path != filepath.Join(want, workspace.SourceDir) || e.entry.CommonDir != filepath.Join(e.entry.Path, ".git") {
			t.Fatalf("store of %s at %s / %s, want under %s", e.cell.UUID, e.entry.Path, e.entry.CommonDir, want)
		}
		if run(t, e.entry.Path, "rev-parse", "--path-format=absolute", "--git-common-dir") != e.entry.CommonDir {
			t.Fatalf("git does not resolve %s to its own common dir", e.entry.Path)
		}
	}
	ai, _ := os.Stat(a.CommonDir)
	bi, _ := os.Stat(b.CommonDir)
	hi, _ := os.Stat(filepath.Join(host, ".git"))
	if os.SameFile(ai, bi) || os.SameFile(ai, hi) || os.SameFile(bi, hi) {
		t.Fatal("two stores, or a store and the host, share one Git common directory")
	}
	if hostBefore != state(t, host) {
		t.Fatal("provisioning a cell store changed the host checkout")
	}

	// Change B's configuration and refs; A and the host do not move.
	aBefore := state(t, a.Path)
	run(t, b.Path, "config", "user.name", "cell b")
	run(t, b.Path, "config", "remote.origin.url", other)
	run(t, b.Path, "update-ref", "refs/heads/b-only", "HEAD")
	run(t, b.Path, "tag", "b-tag")
	run(t, b.Path, "checkout", "-q", "-b", "b-work")
	if got := state(t, a.Path); got != aBefore {
		t.Fatalf("changing cell B's store changed cell A's:\nbefore:\n%s\nafter:\n%s", aBefore, got)
	}
	if hostBefore != state(t, host) {
		t.Fatal("changing cell B's store changed the host checkout")
	}
	if _, err := s.Observe(ctx, cellA, req, legacy); err != nil {
		t.Fatalf("A no longer verifies after B changed: %v", err)
	}
	// Re-observation sees B's drift: the planted identity is configuration a clone never writes.
	_, err = s.Observe(ctx, cellB, req, legacy)
	wantRefusal(t, "observe drifted B", err, workspace.ReasonForeignState, "store.config")

	// Provisioning is idempotent: A again returns A, without calling the transport.
	calls := 0
	counting := Stores{Transport: transportFunc(func(ctx context.Context, r workspace.RepoRef, d string) error {
		calls++
		return cloneFrom{repo: src}.Clone(ctx, r, d)
	})}
	if again, err := counting.Provision(ctx, cellA, req, legacy); err != nil || again != a || calls != 0 {
		t.Fatalf("re-provision A: %+v, %v, %d transport calls", again, err, calls)
	}

	// Substitutes a transport might hand back. Each is refused for its own reason, and leaves no
	// store at the store path.
	subs := []struct {
		name   string
		clone  transportFunc
		reason workspace.Reason
		field  string
	}{
		{"shared common dir: .git links to cell A's", func(_ context.Context, _ workspace.RepoRef, d string) error {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return err
			}
			return os.Symlink(a.CommonDir, filepath.Join(d, ".git"))
		}, workspace.ReasonIdentityMismatch, "store.common-dir"},
		{"alternates: a shared clone of the host", func(_ context.Context, _ workspace.RepoRef, d string) error {
			cmd := exec.Command("git", "clone", "-q", "--shared", host, d)
			cmd.Env = gitEnv()
			return cmd.Run()
		}, workspace.ReasonIdentityMismatch, "store.alternates"},
		{"recursive copy of the host checkout", func(_ context.Context, _ workspace.RepoRef, d string) error {
			copyTree(t, host, d)
			return nil
		}, workspace.ReasonForeignState, "store.config"},
		{"credential in the origin URL", func(ctx context.Context, r workspace.RepoRef, d string) error {
			if err := (cloneFrom{repo: src}).Clone(ctx, r, d); err != nil {
				return err
			}
			cmd := exec.Command("git", "-C", d, "remote", "set-url", "origin", "https://x-access-token:$FIXTURE_PW@forge.example/acme/tools.git")
			cmd.Env = gitEnv()
			return cmd.Run()
		}, workspace.ReasonForeignState, "store.config"},
		{"unparseable origin URL (cannot be shown credential-free)", func(ctx context.Context, r workspace.RepoRef, d string) error {
			if err := (cloneFrom{repo: src}).Clone(ctx, r, d); err != nil {
				return err
			}
			cmd := exec.Command("git", "-C", d, "remote", "set-url", "origin", "https://us%zzer:$FIXTURE_PW@forge.example/acme/tools.git")
			cmd.Env = gitEnv()
			return cmd.Run()
		}, workspace.ReasonForeignState, "store.config"},
		{"commondir pointer into the host's common directory", func(ctx context.Context, r workspace.RepoRef, d string) error {
			if err := (cloneFrom{repo: src}).Clone(ctx, r, d); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(d, ".git", "commondir"), []byte(filepath.Join(host, ".git")+"\n"), 0o644)
		}, workspace.ReasonIdentityMismatch, "store.common-dir"},
		{"objects/ is a symlink into the host checkout", func(ctx context.Context, r workspace.RepoRef, d string) error {
			return linkIntoHost(ctx, cloneFrom{repo: src}, r, d, host, "objects")
		}, workspace.ReasonIdentityMismatch, "store.common-dir"},
		{"config is a symlink to the host checkout's config", func(ctx context.Context, r workspace.RepoRef, d string) error {
			return linkIntoHost(ctx, cloneFrom{repo: src}, r, d, host, "config")
		}, workspace.ReasonIdentityMismatch, "store.common-dir"},
		{"wrong repository", func(ctx context.Context, r workspace.RepoRef, d string) error {
			return cloneFrom{r: other}.Clone(ctx, r, d)
		}, workspace.ReasonIdentityMismatch, "store.subject"},
		{"transport failure", func(context.Context, workspace.RepoRef, string) error {
			return errors.New("connection refused")
		}, workspace.ReasonEvidenceUnavailable, "transport"},
		// Last: it registers a worktree in the host's administration (the transport's doing).
		{"linked worktree of the host", func(_ context.Context, _ workspace.RepoRef, d string) error {
			cmd := exec.Command("git", "-C", host, "worktree", "add", "-q", "--detach", d)
			cmd.Env = gitEnv()
			return cmd.Run()
		}, workspace.ReasonIdentityMismatch, "store.git"},
	}
	for i, sub := range subs {
		cell := newCell(t, base, "sub-"+string(rune('a'+i)), fmt.Sprintf("00000000-0000-4000-8000-0000000030%02d", i))
		lg := legacy
		lg.Cleanup = append(append([]string(nil), legacy.Cleanup...), filepath.Join(cell.Root, "worktrees"))
		_, err := Stores{Transport: sub.clone}.Provision(ctx, cell, req, lg)
		wantRefusal(t, sub.name, err, sub.reason, sub.field)
		if _, err := os.Lstat(workspace.StorePath(cell, repo)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("%s: a refused clone left a store behind (%v)", sub.name, err)
		}
		if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(workspace.StorePath(cell, repo)), ".provision-*")); len(leftovers) > 0 {
			t.Fatalf("%s: staging left behind: %v", sub.name, leftovers)
		}
		if i < len(subs)-1 && hostBefore != state(t, host) {
			t.Fatalf("%s: the refused provisioning changed the host checkout", sub.name)
		}
	}

	// Placement: a store that would resolve into legacy scope is refused before any clone.
	calls = 0
	never := Stores{Transport: transportFunc(func(context.Context, workspace.RepoRef, string) error { calls++; return nil })}
	cellL := newCell(t, base, "cell-l", "00000000-0000-4000-8000-000000000040")
	linked := filepath.Join(base, "legacy-root-link")
	if err := os.Symlink(workspace.StorePath(cellL, repo), linked); err != nil {
		t.Fatal(err)
	}
	_, err = never.Provision(ctx, cellL, req, Legacy{Checkouts: []string{linked}})
	wantRefusal(t, "legacy checkout resolving to the store", err, workspace.ReasonIdentityMismatch, "store.path")
	cellR := newCell(t, base, "cell-r", "00000000-0000-4000-8000-000000000050")
	if err := os.MkdirAll(filepath.Join(cellR.Root, "worktrees", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(cellR.Root, "worktrees", "x"), filepath.Join(cellR.Root, workspace.ReposDir)); err != nil {
		t.Fatal(err)
	}
	_, err = never.Provision(ctx, cellR, req, Legacy{Cleanup: []string{filepath.Join(cellR.Root, "worktrees")}})
	wantRefusal(t, "repos directory linked into the legacy worktrees", err, workspace.ReasonIdentityMismatch, "store.path")
	// A refused placement makes nothing in the place the link points at, even when the key
	// directory does not exist yet.
	if entries, err := os.ReadDir(filepath.Join(cellR.Root, "worktrees", "x")); err != nil || len(entries) != 0 {
		t.Fatalf("a refused placement wrote into the link target: %v, %v", entries, err)
	}
	// An observation that could not be made keeps its own kind: with no usable git, an existing
	// store is evidence-unavailable, not foreign state.
	_, err = Stores{Transport: cloneFrom{repo: src}, Git: filepath.Join(base, "no-such-git")}.Provision(ctx, cellA, req, legacy)
	wantRefusal(t, "existing store, git unavailable", err, workspace.ReasonEvidenceUnavailable, "store.git")
	tracker := newCell(t, filepath.Join(base, "scratch-old"), "cell", "00000000-0000-4000-8000-000000000060")
	_, err = never.Provision(ctx, tracker, req, Legacy{Cleanup: []string{filepath.Join(base, "scratch-*")}})
	wantRefusal(t, "cell under an old sanctioned prefix", err, workspace.ReasonIdentityMismatch, "store.path")
	if calls != 0 {
		t.Fatalf("a placement refusal reached the transport %d times", calls)
	}
}

func TestCellRootBinding(t *testing.T) {
	base := t.TempDir()
	registry := filepath.Join(base, "registry")
	cell := newCell(t, base, "cell", "00000000-0000-4000-8000-000000000070")
	if err := BindCell(registry, cell); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if err := BindCell(registry, cell); err != nil {
		t.Fatalf("re-bind the same root: %v", err)
	}
	// The same UUID at a second root — a copy of the cell directory — is refused.
	copied := filepath.Join(base, "copied")
	copyTree(t, cell.Root, copied)
	err := BindCell(registry, workspace.CellRef{UUID: cell.UUID, Root: copied})
	wantRefusal(t, "copied cell", err, workspace.ReasonForeignState, "cell.root")
	// A second UUID at an already bound root is refused.
	err = BindCell(registry, workspace.CellRef{UUID: "00000000-0000-4000-8000-000000000080", Root: cell.Root})
	wantRefusal(t, "second identity at one root", err, workspace.ReasonIdentityMismatch, "cell.uuid")
	// An entry is linked into place whole and leaves no temporary file behind; an entry that
	// is empty (an interrupted write by an older writer) is an unavailable observation, never
	// a refusal of the root as foreign.
	if leftovers, _ := filepath.Glob(filepath.Join(registry, ".bind-*")); len(leftovers) > 0 {
		t.Fatalf("bind left temporary files: %v", leftovers)
	}
	wedged := newCell(t, base, "wedged", "00000000-0000-4000-8000-000000000090")
	if err := os.WriteFile(filepath.Join(registry, wedged.UUID), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	wantRefusal(t, "empty registry entry", BindCell(registry, wedged), workspace.ReasonEvidenceUnavailable, "registry")
	b, err := os.ReadFile(filepath.Join(registry, cell.UUID))
	if err != nil {
		t.Fatal(err)
	}
	if want, _ := filepath.EvalSymlinks(cell.Root); !bytes.Equal(bytes.TrimSpace(b), []byte(want)) {
		t.Fatalf("registry binds %q, want %q", b, want)
	}
}
