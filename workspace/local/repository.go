// Package local is the local Git and filesystem adapter of the workspace contract: the effects
// the pure package describes, performed and re-verified against the real filesystem.
//
// This file provisions a cell's own repository stores. A store is an independent clone — its own
// Git common directory, configuration, refs and objects — made through a transport the caller
// injects. The adapter never clones on its own authority and never holds a credential: the
// transport is composed by the command from the existing role transport. What the adapter owns
// is the verification, after the transport ran and before anything is published, that the
// result is a store this cell alone owns: not a linked worktree of another checkout, not a
// directory whose `.git` resolves elsewhere, not a clone borrowing objects through alternates,
// not a copy carrying another checkout's configuration, hooks or worktree registrations, holding
// the requested commit, and physically outside every legacy checkout and cleanup root.
package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/workspace"
)

// Transport makes dest a clone of repo. dest does not exist when it is called; its parent does.
// The transport carries whatever identity and credential the command composed for it; it must
// not write the credential into the clone, and anything it leaves there is verified afterwards.
type Transport interface {
	Clone(ctx context.Context, repo workspace.RepoRef, dest string) error
}

// Legacy is the scope managed data must stay physically outside of (spec section 5.4): the
// checkouts the legacy keys name, and every legacy cleanup root. An entry may be a glob pattern
// (`/private/tmp/tracker-*`), matched against every ancestor of a resolved store path.
type Legacy struct {
	// Checkouts are repositories or worktrees selected by CELL_REPO and CELL_ROOTS. A store may
	// neither lie inside one, contain one, nor share its Git common directory.
	Checkouts []string
	// Cleanup are legacy cleanup roots: the cell's worktrees directory, cache and comms roots,
	// old sanctioned worktree prefixes. A store may neither lie inside nor contain one.
	Cleanup []string
}

// Stores provisions and observes a cell's repository stores.
type Stores struct {
	Transport Transport
	// Git is the git executable; empty means "git" on PATH.
	Git string
}

func refusal(reason workspace.Reason, field, format string, args ...any) error {
	return &workspace.Refusal{Reason: reason, Field: field, Msg: fmt.Sprintf(format, args...)}
}

// unknown is every failure that is not a typed refusal: the check could not be made.
func unknown(field string, err error) error {
	if _, ok := workspace.ReasonOf(err); ok {
		return err
	}
	return refusal(workspace.ReasonEvidenceUnavailable, field, "%v", err)
}

// Provision makes, or finds, the cell's store for one repository and returns it verified. An
// existing directory at the store path is observed, never overwritten: a verified store of the
// same repository and subject is returned as it is (provisioning is idempotent), anything else
// is foreign-state. A new clone is made in a staging directory beside the store and moved into
// place only after it verifies; a refused or failed clone leaves no store behind.
func (s Stores) Provision(ctx context.Context, cell workspace.CellRef, req workspace.StoreRequest, legacy Legacy) (workspace.StoreEntry, error) {
	plan, err := workspace.PlanStores(cell, []workspace.StoreRequest{req})
	if err != nil {
		return workspace.StoreEntry{}, err
	}
	req = plan[0]
	if s.Transport == nil {
		return workspace.StoreEntry{}, refusal(workspace.ReasonUnsupportedCapability, "transport", "no transport was composed for provisioning")
	}
	path := workspace.StorePath(cell, req.Repo)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return workspace.StoreEntry{}, unknown("store.path", err)
	}
	if err := checkPlacement(cell, req.Repo, legacy); err != nil {
		return workspace.StoreEntry{}, err
	}
	if _, err := os.Lstat(path); err == nil {
		e, err := s.Observe(ctx, cell, req, legacy)
		if err != nil {
			return workspace.StoreEntry{}, refusal(workspace.ReasonForeignState, "store.path", "an existing directory is not this repository's verified store: %v", err)
		}
		return e, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return workspace.StoreEntry{}, unknown("store.path", err)
	}
	staging, err := os.MkdirTemp(filepath.Dir(path), ".provision-")
	if err != nil {
		return workspace.StoreEntry{}, unknown("store.path", err)
	}
	// The staging directory is this call's own; removing it removes nothing the transport did
	// not put there (a symlink planted inside is removed, never followed).
	defer os.RemoveAll(staging)
	dest := filepath.Join(staging, workspace.SourceDir)
	if err := s.Transport.Clone(ctx, req.Repo, dest); err != nil {
		return workspace.StoreEntry{}, unknown("transport", err)
	}
	if _, err := s.verify(ctx, dest, req, legacy); err != nil {
		return workspace.StoreEntry{}, err
	}
	if err := os.Rename(dest, path); err != nil {
		return workspace.StoreEntry{}, unknown("store.path", err)
	}
	return s.Observe(ctx, cell, req, legacy)
}

// Observe re-verifies the cell's store for one repository where it stands and returns its entry.
func (s Stores) Observe(ctx context.Context, cell workspace.CellRef, req workspace.StoreRequest, legacy Legacy) (workspace.StoreEntry, error) {
	if err := workspace.ValidateCell(cell); err != nil {
		return workspace.StoreEntry{}, err
	}
	if err := checkPlacement(cell, req.Repo, legacy); err != nil {
		return workspace.StoreEntry{}, err
	}
	path := workspace.StorePath(cell, req.Repo)
	real, err := s.verify(ctx, path, req, legacy)
	if err != nil {
		return workspace.StoreEntry{}, err
	}
	return workspace.StoreEntry{
		Repo:      req.Repo,
		RepoKey:   workspace.RepoKey(req.Repo),
		Path:      real,
		CommonDir: filepath.Join(real, ".git"),
		Subject:   req.Subject,
	}, nil
}

// checkPlacement refuses a store directory that does not physically sit where the layout puts
// it: the repository's directory must resolve to <resolved root>/repos/<key>, so a symlinked
// `repos/` (or key directory) pointing into another cell, the legacy worktrees directory or a
// cleanup root is refused before any clone is made.
func checkPlacement(cell workspace.CellRef, repo workspace.RepoRef, legacy Legacy) error {
	root, err := filepath.EvalSymlinks(cell.Root)
	if err != nil {
		return unknown("cell.root", err)
	}
	dir := filepath.Dir(workspace.StorePath(cell, repo))
	got, err := filepath.EvalSymlinks(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // nothing placed yet; Provision creates it, and verify resolves it again
		}
		return unknown("store.path", err)
	}
	if want := filepath.Join(root, workspace.ReposDir, workspace.RepoKey(repo)); got != want {
		return refusal(workspace.ReasonIdentityMismatch, "store.path", "the store directory resolves outside its cell's repos directory")
	}
	return checkLegacy(filepath.Join(got, workspace.SourceDir), legacy)
}

// verify checks that dir is an independent store of req and returns its resolved path.
func (s Stores) verify(ctx context.Context, dir string, req workspace.StoreRequest, legacy Legacy) (string, error) {
	fi, err := os.Lstat(dir)
	if err != nil {
		return "", unknown("store.path", err)
	}
	if !fi.IsDir() {
		return "", refusal(workspace.ReasonForeignState, "store.path", "the store is not a directory")
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", unknown("store.path", err)
	}
	if err := checkLegacy(real, legacy); err != nil {
		return "", err
	}

	// A linked worktree carries a `.git` file pointing at another checkout's administration.
	dotgit := filepath.Join(dir, ".git")
	gi, err := os.Lstat(dotgit)
	if errors.Is(err, fs.ErrNotExist) {
		return "", refusal(workspace.ReasonForeignState, "store.git", "the store has no .git directory")
	} else if err != nil {
		return "", unknown("store.git", err)
	}
	if gi.Mode().IsRegular() {
		return "", refusal(workspace.ReasonIdentityMismatch, "store.git", "the store is a linked worktree of another checkout")
	}

	// Its own common directory: what Git resolves must be this directory's own .git, a real
	// directory, never a symlink to (or a commondir pointer into) another checkout's.
	gitDir, commonDir, err := s.gitDirs(ctx, dir)
	if err != nil {
		return "", err
	}
	own := filepath.Join(real, ".git")
	if !gi.IsDir() || gitDir != own || commonDir != own {
		return "", refusal(workspace.ReasonIdentityMismatch, "store.common-dir", "the store's Git common directory is not its own .git")
	}
	if _, err := os.Lstat(filepath.Join(own, "commondir")); err == nil {
		return "", refusal(workspace.ReasonIdentityMismatch, "store.common-dir", "the store's Git directory points at another common directory")
	}
	for _, other := range legacy.Checkouts {
		if shared, err := s.sharesCommonDir(ctx, own, other); err != nil {
			return "", err
		} else if shared {
			return "", refusal(workspace.ReasonIdentityMismatch, "store.common-dir", "the store shares a Git common directory with a legacy checkout")
		}
	}

	// No object borrowing: alternates make the store depend on, and read through, another one.
	for _, name := range []string{"alternates", "http-alternates"} {
		if _, err := os.Lstat(filepath.Join(own, "objects", "info", name)); err == nil {
			return "", refusal(workspace.ReasonIdentityMismatch, "store.alternates", "the store borrows objects from another repository")
		} else if !errors.Is(err, fs.ErrNotExist) {
			return "", unknown("store.alternates", err)
		}
	}

	// Nothing inherited: a recursive copy of another checkout carries its configuration,
	// worktree registrations and hooks; a fresh clone has none of them.
	if err := s.checkConfig(ctx, own); err != nil {
		return "", err
	}
	if ents, err := os.ReadDir(filepath.Join(own, "worktrees")); err == nil && len(ents) > 0 {
		return "", refusal(workspace.ReasonForeignState, "store.worktrees", "the store carries worktree registrations it did not make")
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", unknown("store.worktrees", err)
	}
	if err := checkHooks(filepath.Join(own, "hooks")); err != nil {
		return "", err
	}

	// The right history: the requested commit must be present. A transport that cloned some
	// other repository cannot produce it.
	if _, err := s.git(ctx, dir, "cat-file", "-e", req.Subject+"^{commit}"); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", refusal(workspace.ReasonIdentityMismatch, "store.subject", "the store does not hold the requested commit")
		}
		return "", unknown("store.subject", err)
	}
	return real, nil
}

// allowedConfig is everything a fresh clone writes into its own configuration. Any other key —
// a credential helper, an include, a hooks path, an identity, another remote — was inherited or
// planted, and refuses the store.
func allowedConfig(name string) bool {
	switch name {
	case "core.repositoryformatversion", "core.filemode", "core.bare", "core.logallrefupdates",
		"core.ignorecase", "core.precomposeunicode", "core.symlinks", "extensions.objectformat",
		"remote.origin.url", "remote.origin.fetch":
		return true
	}
	if strings.HasPrefix(name, "branch.") {
		return strings.HasSuffix(name, ".remote") || strings.HasSuffix(name, ".merge")
	}
	return false
}

func (s Stores) checkConfig(ctx context.Context, own string) error {
	if _, err := os.Lstat(filepath.Join(own, "config.worktree")); err == nil {
		return refusal(workspace.ReasonForeignState, "store.config", "the store carries per-worktree configuration")
	}
	out, err := s.git(ctx, "", "config", "--file", filepath.Join(own, "config"), "--list", "--name-only")
	if err != nil {
		return unknown("store.config", err)
	}
	for _, name := range strings.Fields(string(out)) {
		if !allowedConfig(strings.ToLower(name)) {
			return refusal(workspace.ReasonForeignState, "store.config", "the store's configuration carries a key a fresh clone does not write")
		}
	}
	raw, err := s.git(ctx, "", "config", "--file", filepath.Join(own, "config"), "--get-all", "remote.origin.url")
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return refusal(workspace.ReasonForeignState, "store.config", "the store has no origin remote")
		}
		return unknown("store.config", err)
	}
	for _, u := range strings.Fields(string(raw)) {
		if hasCredential(u) {
			return refusal(workspace.ReasonForeignState, "store.config", "the store's origin URL carries a credential")
		}
	}
	return nil
}

// hasCredential reports a URL with userinfo on an http(s) remote, where the user part is a
// credential (a token or password), as opposed to an ssh login name.
func hasCredential(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return false
	}
	if _, ok := u.User.Password(); ok {
		return true
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func checkHooks(dir string) error {
	ents, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return unknown("store.hooks", err)
	}
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".sample") {
			return refusal(workspace.ReasonForeignState, "store.hooks", "the store carries an active hook it did not make")
		}
	}
	return nil
}

// checkLegacy refuses a resolved store path inside, or containing, any legacy scope entry.
func checkLegacy(real string, legacy Legacy) error {
	for _, set := range [][]string{legacy.Checkouts, legacy.Cleanup} {
		for _, p := range set {
			if strings.ContainsAny(p, "*?[") {
				for d := real; ; d = filepath.Dir(d) {
					if ok, _ := filepath.Match(p, d); ok {
						return refusal(workspace.ReasonIdentityMismatch, "store.path", "the store lies under a legacy cleanup prefix")
					}
					if filepath.Dir(d) == d {
						break
					}
				}
				continue
			}
			r, err := resolve(p)
			if err != nil {
				return unknown("legacy", err)
			}
			if within(real, r) || within(r, real) {
				return refusal(workspace.ReasonIdentityMismatch, "store.path", "the store overlaps a legacy checkout or cleanup root")
			}
		}
	}
	return nil
}

// resolve is the physical path p names, following symlinks, including a dangling one: a legacy
// entry that is a link to where a store is about to be made resolves to that place, so it is
// refused before the clone rather than discovered after it. A missing tail resolves to its
// spelling under its resolved existing ancestor.
func resolve(p string) (string, error) {
	p = filepath.Clean(p)
	for hops := 0; hops < 40; hops++ {
		r, err := filepath.EvalSymlinks(p)
		if err == nil {
			return r, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if t, lerr := os.Readlink(p); lerr == nil {
			if !filepath.IsAbs(t) {
				t = filepath.Join(filepath.Dir(p), t)
			}
			p = filepath.Clean(t)
			continue
		}
		parent := filepath.Dir(p)
		if parent == p {
			return p, nil
		}
		rp, err := resolve(parent)
		if err != nil {
			return "", err
		}
		return filepath.Join(rp, filepath.Base(p)), nil
	}
	return "", fmt.Errorf("too many symbolic links resolving a legacy path")
}

func within(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// sharesCommonDir reports whether the checkout at other resolves to the same Git common
// directory (by file identity) as own. A path that is not there or not a checkout shares none.
func (s Stores) sharesCommonDir(ctx context.Context, own, other string) (bool, error) {
	if _, err := os.Stat(other); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	_, common, err := s.gitDirs(ctx, other)
	if err != nil {
		if r, ok := workspace.ReasonOf(err); ok && r == workspace.ReasonForeignState {
			return false, nil
		}
		return false, err
	}
	a, err := os.Stat(own)
	if err != nil {
		return false, unknown("store.common-dir", err)
	}
	b, err := os.Stat(common)
	if err != nil {
		return false, unknown("legacy", err)
	}
	return os.SameFile(a, b), nil
}

// gitDirs is Git's own resolution of dir's Git directory and common directory, both resolved.
// It never searches above dir: a directory that is not itself a checkout is foreign-state.
func (s Stores) gitDirs(ctx context.Context, dir string) (string, string, error) {
	out, err := s.git(ctx, dir, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", "", refusal(workspace.ReasonForeignState, "store.git", "not a Git checkout")
		}
		return "", "", unknown("store.git", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		return "", "", refusal(workspace.ReasonEvidenceUnavailable, "store.git", "unexpected rev-parse output")
	}
	var res [2]string
	for i, l := range lines {
		r, err := filepath.EvalSymlinks(l)
		if err != nil {
			return "", "", unknown("store.git", err)
		}
		res[i] = r
	}
	return res[0], res[1], nil
}

// git runs one read-only git command with a closed environment: no system or global
// configuration, no inherited repository selection, and no discovery above dir.
func (s Stores) git(ctx context.Context, dir string, args ...string) ([]byte, error) {
	bin := s.Git
	if bin == "" {
		bin = "git"
	}
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	env := []string{"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") && !strings.HasPrefix(kv, "LC_ALL=") {
			env = append(env, kv)
		}
	}
	if dir != "" {
		env = append(env, "GIT_CEILING_DIRECTORIES="+filepath.Dir(filepath.Clean(dir)))
	}
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	return cmd.Output()
}

// CellMarker is the file, under a cell's control directory, naming the UUID the root is bound to.
const CellMarker = "cell"

// BindCell binds a cell UUID to exactly one active root, and the root to exactly one UUID.
// registry is a machine-level directory holding one file per bound UUID, naming its resolved
// root; the cell's own control directory holds a marker naming its UUID. A root already bound
// to another UUID is an identity mismatch. A UUID already active at another root — a copied or
// moved cell directory, or a second cell claiming the same identity — is foreign-state: the
// binding is never silently moved.
func BindCell(registry string, cell workspace.CellRef) error {
	if err := workspace.ValidateCell(cell); err != nil {
		return err
	}
	root, err := filepath.EvalSymlinks(cell.Root)
	if err != nil {
		return unknown("cell.root", err)
	}
	marker := filepath.Join(root, filepath.FromSlash(workspace.ControlDir), CellMarker)
	if b, err := os.ReadFile(marker); err == nil {
		if strings.TrimSpace(string(b)) != cell.UUID {
			return refusal(workspace.ReasonIdentityMismatch, "cell.uuid", "the root is bound to another cell")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return unknown("cell.root", err)
	}
	if err := os.MkdirAll(registry, 0o700); err != nil {
		return unknown("registry", err)
	}
	entry := filepath.Join(registry, cell.UUID)
	f, err := os.OpenFile(entry, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	switch {
	case err == nil:
		_, werr := f.WriteString(root + "\n")
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			return unknown("registry", werr)
		}
	case errors.Is(err, fs.ErrExist):
		b, rerr := os.ReadFile(entry)
		if rerr != nil {
			return unknown("registry", rerr)
		}
		if strings.TrimSpace(string(b)) != root {
			return refusal(workspace.ReasonForeignState, "cell.root", "the cell is already active at another root")
		}
	default:
		return unknown("registry", err)
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		return unknown("cell.root", err)
	}
	if err := os.WriteFile(marker, []byte(cell.UUID+"\n"), 0o600); err != nil {
		return unknown("cell.root", err)
	}
	return nil
}
