// Package gitcore is the shared in-process git layer for the desktools-go-git
// migration (brief 02). It wraps go-git so every migrated desk tool executes git
// operations as library calls inside its own process — no external `git` binary, no
// credential helper, no `insteadOf` substitution, no hooks, no ambient PATH lookup.
//
// The package holds two families of surface:
//
//   - Read helpers (Open / Resolve / Refs / FileAt / Files / DiffNames / Log /
//     MergeBase / IsAncestor) — plain wrappers over go-git's plumbing, verified against
//     the brief-01 golden harness so a migrated caller's OUTCOME (not argv) matches the
//     git-binary seam it replaces.
//   - Transport verbs (Fetch / Push / List) — each takes an explicit URL and an
//     explicit Auth for that call only. There is no remote alias, no `insteadOf`
//     layer, no credential helper, no askpass, and no GIT_* environment: a caller
//     mints a repo-scoped token (see auth.go) and is structurally unable to send it
//     anywhere but the URL it built itself, from a roster-validated slug.
//
// This layer introduces NO behaviour change and swaps NO caller's seam — that is
// briefs 03-07. It stands up the package and proves it against fixtures.
package gitcore

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

// transientRemoteName is the Name every ephemeral *git.Remote gitcore constructs for a
// transport op carries. It is never persisted to the repo's config — Fetch/Push build
// a fresh, unstored *git.Remote per call so the URL is exactly the one the caller
// passed, never one resolved from a configured alias. go-git's FetchOptions/
// PushOptions default RemoteName to "origin" when empty and then assert it equals the
// Remote's own Name, so this constant must track that default.
const transientRemoteName = "origin"

// Repo is an opened git repository, read and written in-process via go-git.
type Repo struct {
	repo *git.Repository
	dir  string
}

// Open opens the repository rooted at dir. It never shells out and never consults a
// credential helper — go-git reads only the repository's own on-disk objects/refs.
func Open(dir string) (*Repo, error) {
	r, err := git.PlainOpen(dir)
	if err != nil {
		return nil, fmt.Errorf("gitcore: open %s: %w", dir, err)
	}
	return &Repo{repo: r, dir: dir}, nil
}

// Resolve resolves rev (a ref name, a short or long SHA, or a revision expression
// such as "HEAD~1") to its full object hash, matching `git rev-parse <rev>`.
func (r *Repo) Resolve(rev string) (plumbing.Hash, error) {
	h, err := r.repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("gitcore: resolve %q: %w", rev, err)
	}
	return *h, nil
}

// Refs returns every hash reference in the repository as refname -> resolved SHA
// (hex), matching `git for-each-ref --format=%(refname) %(objectname)`. Symbolic
// references (HEAD) are omitted, matching for-each-ref's own default behaviour.
func (r *Repo) Refs() (map[string]string, error) {
	iter, err := r.repo.References()
	if err != nil {
		return nil, fmt.Errorf("gitcore: refs: %w", err)
	}
	defer iter.Close()
	out := map[string]string{}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}
		out[string(ref.Name())] = ref.Hash().String()
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("gitcore: refs: %w", err)
	}
	return out, nil
}

// treeAt resolves rev to its commit's tree.
func (r *Repo) treeAt(rev string) (*object.Tree, error) {
	hash, err := r.Resolve(rev)
	if err != nil {
		return nil, err
	}
	commit, err := r.repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("gitcore: commit %s: %w", hash, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("gitcore: tree at %s: %w", hash, err)
	}
	return tree, nil
}

// FileAt returns the content of path in the tree rev resolves to, matching
// `git cat-file blob <rev>:<path>`.
func (r *Repo) FileAt(rev, path string) (string, error) {
	tree, err := r.treeAt(rev)
	if err != nil {
		return "", err
	}
	f, err := tree.File(path)
	if err != nil {
		return "", fmt.Errorf("gitcore: file %s at %s: %w", path, rev, err)
	}
	content, err := f.Contents()
	if err != nil {
		return "", fmt.Errorf("gitcore: read %s at %s: %w", path, rev, err)
	}
	return content, nil
}

// Files lists every regular file path in the tree rev resolves to, matching
// `git ls-tree -r --name-only <rev>`.
func (r *Repo) Files(rev string) ([]string, error) {
	tree, err := r.treeAt(rev)
	if err != nil {
		return nil, err
	}
	walker := object.NewTreeWalker(tree, true, nil)
	defer walker.Close()
	var out []string
	for {
		name, entry, err := walker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gitcore: walk tree at %s: %w", rev, err)
		}
		if entry.Mode.IsFile() {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// DiffNames returns the sorted set of paths that differ between from and to (each a
// revision expression), matching `git diff --name-only <from> <to>` (with rename
// detection: a rename contributes both its old and new path, as git's own
// --name-only does for a detected rename pair).
func (r *Repo) DiffNames(from, to string) ([]string, error) {
	fromTree, err := r.treeAt(from)
	if err != nil {
		return nil, err
	}
	toTree, err := r.treeAt(to)
	if err != nil {
		return nil, err
	}
	changes, err := fromTree.Diff(toTree)
	if err != nil {
		return nil, fmt.Errorf("gitcore: diff %s..%s: %w", from, to, err)
	}
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for _, c := range changes {
		add(c.From.Name)
		add(c.To.Name)
	}
	sort.Strings(out)
	return out, nil
}

// Log returns the commit hashes reachable from rev, newest first, matching
// `git rev-list <rev>` / `git log --format=%H <rev>`.
func (r *Repo) Log(rev string) ([]string, error) {
	hash, err := r.Resolve(rev)
	if err != nil {
		return nil, err
	}
	iter, err := r.repo.Log(&git.LogOptions{From: hash})
	if err != nil {
		return nil, fmt.Errorf("gitcore: log %s: %w", rev, err)
	}
	defer iter.Close()
	var out []string
	err = iter.ForEach(func(c *object.Commit) error {
		out = append(out, c.Hash.String())
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("gitcore: log %s: %w", rev, err)
	}
	return out, nil
}

// MergeBase returns the best common ancestor of a and b, matching
// `git merge-base <a> <b>`.
func (r *Repo) MergeBase(a, b string) (string, error) {
	aHash, err := r.Resolve(a)
	if err != nil {
		return "", err
	}
	bHash, err := r.Resolve(b)
	if err != nil {
		return "", err
	}
	aCommit, err := r.repo.CommitObject(aHash)
	if err != nil {
		return "", fmt.Errorf("gitcore: commit %s: %w", aHash, err)
	}
	bCommit, err := r.repo.CommitObject(bHash)
	if err != nil {
		return "", fmt.Errorf("gitcore: commit %s: %w", bHash, err)
	}
	bases, err := aCommit.MergeBase(bCommit)
	if err != nil {
		return "", fmt.Errorf("gitcore: merge-base %s %s: %w", a, b, err)
	}
	if len(bases) == 0 {
		return "", fmt.Errorf("gitcore: merge-base %s %s: no common ancestor", a, b)
	}
	return bases[0].Hash.String(), nil
}

// IsAncestor reports whether ancestor is reachable from descendant (i.e. is an
// ancestor of, or equal to, descendant), matching
// `git merge-base --is-ancestor <ancestor> <descendant>`.
func (r *Repo) IsAncestor(ancestor, descendant string) (bool, error) {
	aHash, err := r.Resolve(ancestor)
	if err != nil {
		return false, err
	}
	dHash, err := r.Resolve(descendant)
	if err != nil {
		return false, err
	}
	aCommit, err := r.repo.CommitObject(aHash)
	if err != nil {
		return false, fmt.Errorf("gitcore: commit %s: %w", aHash, err)
	}
	dCommit, err := r.repo.CommitObject(dHash)
	if err != nil {
		return false, fmt.Errorf("gitcore: commit %s: %w", dHash, err)
	}
	ok, err := aCommit.IsAncestor(dCommit)
	if err != nil {
		return false, fmt.Errorf("gitcore: is-ancestor %s %s: %w", ancestor, descendant, err)
	}
	return ok, nil
}

// buildRefSpecs turns caller-supplied literal refspec strings into validated
// config.RefSpec values. force, when true, is a TYPE-LEVEL property: it prepends the
// force marker ("+") to every spec that does not already carry one, so "no force
// possible" is something the caller must ask for explicitly per call rather than
// something argv discipline has to remember to omit.
func buildRefSpecs(specs []string, force bool) ([]config.RefSpec, error) {
	out := make([]config.RefSpec, 0, len(specs))
	for _, s := range specs {
		if force && !strings.HasPrefix(s, "+") {
			s = "+" + s
		}
		rs := config.RefSpec(s)
		if err := rs.Validate(); err != nil {
			return nil, fmt.Errorf("gitcore: invalid refspec %q: %w", s, err)
		}
		out = append(out, rs)
	}
	return out, nil
}

// FetchOpts configures an in-process Fetch. URL is the full remote URL — built by the
// caller from a roster-validated slug, never resolved from a configured remote alias
// or an `insteadOf` substitution, because none exists here. Auth is scoped to exactly
// this call: gitcore has no mechanism to send it anywhere but URL.
type FetchOpts struct {
	URL      string
	RefSpecs []string
	Auth     transport.AuthMethod
	Force    bool
	Prune    bool
}

// Fetch fetches RefSpecs from URL into the repo, entirely in-process: no external git
// binary is spawned, no credential helper or askpass is consulted, no hook runs.
// Returns nil on success, including when the remote was already up to date.
func (r *Repo) Fetch(opts FetchOpts) error {
	specs, err := buildRefSpecs(opts.RefSpecs, opts.Force)
	if err != nil {
		return err
	}
	remote := git.NewRemote(r.repo.Storer, &config.RemoteConfig{
		Name: transientRemoteName,
		URLs: []string{opts.URL},
	})
	err = remote.Fetch(&git.FetchOptions{
		RefSpecs: specs,
		Auth:     opts.Auth,
		Force:    opts.Force,
		Prune:    opts.Prune,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("gitcore: fetch: %w", err)
	}
	return nil
}

// PushOpts configures an in-process Push. Same shape and same containment guarantee
// as FetchOpts: URL is caller-built, Auth is scoped to this call only.
type PushOpts struct {
	URL      string
	RefSpecs []string
	Auth     transport.AuthMethod
	Force    bool
}

// Push pushes RefSpecs from the repo to URL, entirely in-process. Force is off unless
// set — a non-fast-forward update is refused by the protocol, not by an argv
// convention the caller has to remember. Returns nil on success, including when the
// remote was already up to date.
func (r *Repo) Push(opts PushOpts) error {
	specs, err := buildRefSpecs(opts.RefSpecs, opts.Force)
	if err != nil {
		return err
	}
	remote := git.NewRemote(r.repo.Storer, &config.RemoteConfig{
		Name: transientRemoteName,
		URLs: []string{opts.URL},
	})
	err = remote.Push(&git.PushOptions{
		RefSpecs: specs,
		Auth:     opts.Auth,
		Force:    opts.Force,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("gitcore: push: %w", err)
	}
	return nil
}

// ListOpts configures an in-process remote List (`git ls-remote` / the transport
// probe deskkit's preflight used to run separately). It needs no local repository.
type ListOpts struct {
	URL  string
	Auth transport.AuthMethod
}

// List lists every reference advertised by the remote at URL, entirely in-process,
// with no local repository required — the effective URL IS the one the caller passed:
// there is no `insteadOf` layer for it to be substituted through.
func List(opts ListOpts) ([]*plumbing.Reference, error) {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: transientRemoteName,
		URLs: []string{opts.URL},
	})
	refs, err := remote.List(&git.ListOptions{Auth: opts.Auth})
	if err != nil {
		return nil, fmt.Errorf("gitcore: list: %w", err)
	}
	return refs, nil
}
