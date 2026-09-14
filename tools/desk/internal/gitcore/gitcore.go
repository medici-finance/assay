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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	fdiff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
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

// Open opens the repository rooted at dir (an exact repo/worktree root — like
// `git.PlainOpen`, it does not search upward; use Toplevel first if dir might be a
// subdirectory). It never shells out and never consults a credential helper — go-git
// reads only the repository's own on-disk objects/refs.
//
// This does NOT call git.PlainOpen. It reproduces PlainOpen's own construction
// sequence (resolve the .git entry, build a filesystem storage over it, open) so it
// can route the storer through extensionTolerantStorer — go-git v5.19.2's
// verifyExtensions lowercases an extension's NAME before checking it against its own
// mixed-case allowlist (see extensionTolerantStorer), so PlainOpen hard-refuses to open
// ANY repository carrying `extensions.worktreeConfig = true` even though go-git's own
// table lists that extension as supported. This house's roleinit.go/workpad.go set
// exactly that extension on every linked worktree it provisions (to scope bot identity
// and the workpad id via `git config --worktree`), so without this, Open would refuse
// almost every real worktree here.
func Open(dir string) (*Repo, error) {
	gitDir, worktreeDir, err := resolveGitDir(dir)
	if err != nil {
		return nil, fmt.Errorf("gitcore: open %s: %w", dir, err)
	}
	// A linked worktree's gitDir (.git/worktrees/<name>) holds only HEAD/index/the
	// worktree-scoped config — refs, objects, packed-refs and the SHARED config live
	// one level up, in the main checkout's common .git. dotgit.RepositoryFilesystem
	// routes each path to the right one of the two, exactly as
	// `git.PlainOpenWithOptions(..., EnableDotGitCommonDir: true)` does internally
	// (unexported there, so reproduced here rather than reused).
	var repoFS billy.Filesystem = osfs.New(gitDir)
	if commonDir, cerr := commonDirOf(gitDir); cerr == nil && commonDir != "" && commonDir != gitDir {
		repoFS = dotgit.NewRepositoryFilesystem(osfs.New(gitDir), osfs.New(commonDir))
	}
	st := filesystem.NewStorage(repoFS, cache.NewObjectLRUDefault())
	r, err := git.Open(extensionTolerantStorer{st}, osfs.New(worktreeDir))
	if err != nil {
		return nil, fmt.Errorf("gitcore: open %s: %w", dir, err)
	}
	return &Repo{repo: r, dir: dir}, nil
}

// commonDirOf reads gitDir's own "commondir" pointer file (present only for a linked
// worktree's admin directory), resolving a relative path against gitDir. Returns ""
// with no error when the file is absent — gitDir IS its own common dir.
func commonDirOf(gitDir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	common := strings.TrimSpace(string(raw))
	if !filepath.IsAbs(common) {
		common = filepath.Join(gitDir, common)
	}
	return filepath.Clean(common), nil
}

// resolveGitDir returns the repository's own admin directory and its worktree root for
// the repository whose .git entry is directly inside dir (no upward search — see
// Toplevel for that). For a normal checkout gitDir is dir/.git; for a linked worktree
// it is the resolved target of the "gitdir: <path>" pointer file (the per-worktree
// admin directory — CommonDir resolves one step further, to the shared .git).
func resolveGitDir(dir string) (gitDir, worktreeDir string, err error) {
	dotGit := filepath.Join(dir, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return dotGit, dir, nil
	}
	gd, err := readGitdirPointer(dotGit, dir)
	if err != nil {
		return "", "", err
	}
	return gd, dir, nil
}

// readGitdirPointer reads a linked-worktree ".git" FILE (as opposed to a directory)
// and returns the absolute path its "gitdir: <path>" line points at, resolving a
// relative path against base.
func readGitdirPointer(pointerFile, base string) (string, error) {
	raw, err := os.ReadFile(pointerFile)
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(raw))
	const prefix = "gitdir: "
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("unrecognised .git file %q", line)
	}
	gd := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(base, gd)
	}
	return filepath.Clean(gd), nil
}

// extensionTolerantStorer works around a go-git v5.19.2 bug: verifyExtensions
// lowercases an extension's declared NAME before checking it against
// extensionsValidForV0, whose keys are mixed-case ("worktreeConfig") — so the
// lowercased "worktreeconfig" never matches and a genuinely go-git-supported extension
// is refused as unsupported. Scoped to exactly that one known-safe, git-native
// extension name (go-git's own table already lists it as valid for
// repositoryformatversion 0); every other extension still reaches verifyExtensions
// unfiltered and a genuinely unknown/unsupported one still refuses, as intended.
type extensionTolerantStorer struct {
	storage.Storer
}

func (s extensionTolerantStorer) Config() (*config.Config, error) {
	cfg, err := s.Storer.Config()
	if err != nil || cfg == nil || cfg.Raw == nil || !cfg.Raw.HasSection("extensions") {
		return cfg, err
	}
	sect := cfg.Raw.Section("extensions")
	kept := sect.Options[:0:0]
	for _, opt := range sect.Options {
		if strings.EqualFold(opt.Key, "worktreeConfig") {
			continue
		}
		kept = append(kept, opt)
	}
	sect.Options = kept
	return cfg, nil
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

// renameDetectOptions are the DiffTree options DiffNames runs under. go-git's
// tree.Diff performs NO rename detection, so under it a rename surfaces as an
// unrelated delete+add pair and DiffNames would report BOTH paths — while the
// `git diff --name-only` seam it replaces detects renames by default (git's
// diff.renames has defaulted to true since 2.9) and reports only the new path.
// That divergence is exactly the kind of silent outcome change this package
// exists to prevent, so rename detection is turned on here, at git's own
// similarity threshold (`-M` defaults to 50%) rather than go-git's 60, so the
// pair git calls a rename is the pair gitcore calls a rename.
var renameDetectOptions = &object.DiffTreeOptions{
	DetectRenames:    true,
	RenameScore:      50,
	RenameLimit:      0,
	OnlyExactRenames: false,
}

// DiffNames returns the sorted set of paths that differ between from and to (each a
// revision expression), matching `git diff --name-only <from> <to>` — including its
// rename detection: a detected rename contributes ONLY its new path, exactly as
// git's own --name-only does for a detected rename pair.
func (r *Repo) DiffNames(from, to string) ([]string, error) {
	fromTree, err := r.treeAt(from)
	if err != nil {
		return nil, err
	}
	toTree, err := r.treeAt(to)
	if err != nil {
		return nil, err
	}
	changes, err := object.DiffTreeWithOptions(context.Background(), fromTree, toTree, renameDetectOptions)
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
		// A detected rename is ONE change carrying both endpoints (From.Name !=
		// To.Name, both non-empty). git --name-only prints only the new path for
		// it, so only To.Name is added. Every other shape — add (From.Name ""),
		// delete (To.Name ""), modify (the two names equal) — has one distinct
		// path, and adding both endpoints yields exactly that one path.
		if c.From.Name != "" && c.To.Name != "" && c.From.Name != c.To.Name {
			add(c.To.Name)
			continue
		}
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

// --- Read helpers added for brief 03 (migrate read/plumbing verbs) ---------
//
// These extend the brief-02 surface above with the remaining read families brief 03's
// callers need: worktree/common-dir discovery, branch/upstream resolution, staged-changes
// and tracked-status checks, ahead-counts, a tree-ish object id lookup, local branch
// enumeration, and a full unified diff. Every one is READ-ONLY and mirrors an existing
// git-binary seam being migrated — see this migration stream's brief-03 spec.
//
// NOT added here, deliberately: a generic `config --get`/`config --list` reader. go-git's
// Repository.Config() reads ONLY the repository's own local .git/config — it does not merge
// extensions.worktreeConfig's config.worktree file the way real git does — and this house's
// own worktrees set user.name/user.email (and other keys) AT THE WORKTREE SCOPE
// specifically so each linked worktree carries its own commit identity without touching the
// shared .git/config (see roleinit_test.go, workpad.go). A gitcore-based config reader would
// silently return the WRONG (or empty) value for exactly that case, so `config --get
// user.email` and `config --list -z` stay on the git binary — see brief-03's PR body for the
// full note. remote.origin.url is the one config key this package DOES read (via RemoteURL,
// below): remotes are never worktree-scoped in this house's actual usage.

// Toplevel returns the working tree root containing dir, matching
// `git rev-parse --show-toplevel`: it walks UP from dir to find the nearest ancestor
// holding a ".git" entry, so it works from any subdirectory of a worktree.
//
// This does not open the repository at all (no git.PlainOpenWithOptions, so the
// extensionTolerantStorer workaround Open needs does not apply here — there is
// nothing to work around: this function never calls verifyExtensions).
func Toplevel(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("gitcore: toplevel %s: %w", dir, err)
	}
	cur := abs
	for {
		if _, statErr := os.Stat(filepath.Join(cur, ".git")); statErr == nil {
			return resolveSymlinksBestEffort(cur), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", fmt.Errorf("gitcore: toplevel %s: not a git repository (or any parent up to /)", dir)
		}
		cur = parent
	}
}

// CommonDir returns the shared .git directory for the repository containing dir, matching
// `git rev-parse --path-format=absolute --git-common-dir`. For a linked worktree this is
// the MAIN checkout's .git — not the per-worktree .git/worktrees/<name> admin directory —
// resolved the same way git itself does: read the .git file's `gitdir: <path>` pointer,
// then that directory's own `commondir` file if one is present.
func CommonDir(dir string) (string, error) {
	top, err := Toplevel(dir)
	if err != nil {
		return "", err
	}
	gitDir, _, err := resolveGitDir(top)
	if err != nil {
		return "", fmt.Errorf("gitcore: common-dir %s: %w", dir, err)
	}
	// The per-worktree gitdir carries a "commondir" file pointing back at the shared
	// .git — resolve it exactly as git's gitrepository-layout doc specifies. Its absence
	// means gitDir IS the common dir (a non-worktree, non-bare layout).
	common, err := commonDirOf(gitDir)
	if err != nil {
		return "", fmt.Errorf("gitcore: common-dir %s: %w", dir, err)
	}
	if common == "" {
		return resolveSymlinksBestEffort(filepath.Clean(gitDir)), nil
	}
	return resolveSymlinksBestEffort(common), nil
}

// resolveSymlinksBestEffort mirrors git's own `--path-format=absolute`, which always
// prints a REALPATH-resolved path: on a host where the temp/checkout root itself is a
// symlink (e.g. macOS's /var -> /private/var), the raw "gitdir:"/"commondir" pointer
// file content can carry either spelling depending on which one was current-working-
// directory at worktree-creation time, so this normalises before returning. A resolve
// failure (a dangling or permission-denied path) falls back to the unresolved, already-
// cleaned path rather than erroring — CommonDir's callers all use the result for
// display/comparison, never as a filesystem handle this needs to open.
func resolveSymlinksBestEffort(p string) string {
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		return resolved
	}
	return p
}

// InsideWorkTree reports whether the repository has a working tree (is non-bare),
// matching the outcome every caller of `git rev-parse --is-inside-work-tree` actually
// checks for (they test the call's success, not its printed "true"/"false").
func (r *Repo) InsideWorkTree() bool {
	_, err := r.repo.Worktree()
	return err == nil
}

// AbbrevRefHEAD returns the current branch's short name, matching
// `git rev-parse --abbrev-ref HEAD` — including that call's own behaviour of returning
// the literal string "HEAD" (never an error) when HEAD is detached.
func (r *Repo) AbbrevRefHEAD() (string, error) {
	head, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("gitcore: abbrev-ref HEAD: %w", err)
	}
	if head.Name() == plumbing.HEAD {
		return "HEAD", nil
	}
	return head.Name().Short(), nil
}

// SymbolicRefShortHEAD returns the current branch's short name, matching
// `git symbolic-ref --short HEAD` — including that call's own behaviour of ERRORING
// when HEAD is detached (unlike AbbrevRefHEAD, which reports "HEAD" instead).
func (r *Repo) SymbolicRefShortHEAD() (string, error) {
	head, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("gitcore: symbolic-ref --short HEAD: %w", err)
	}
	if head.Name() == plumbing.HEAD {
		return "", fmt.Errorf("gitcore: symbolic-ref --short HEAD: HEAD is detached")
	}
	return head.Name().Short(), nil
}

// SymbolicRefTarget returns the FULL ref name a symbolic reference points at, matching
// `git symbolic-ref <name>` (unlike SymbolicRefShortHEAD's --short form) — e.g.
// SymbolicRefTarget("refs/remotes/origin/HEAD") returns "refs/remotes/origin/main".
// Errors when name does not exist or is not itself a symbolic reference, matching
// git's own refusal.
func (r *Repo) SymbolicRefTarget(name string) (string, error) {
	ref, err := r.repo.Reference(plumbing.ReferenceName(name), false)
	if err != nil {
		return "", fmt.Errorf("gitcore: symbolic-ref %s: %w", name, err)
	}
	if ref.Type() != plumbing.SymbolicReference {
		return "", fmt.Errorf("gitcore: symbolic-ref %s: not a symbolic reference", name)
	}
	return string(ref.Target()), nil
}

// UpstreamRef returns the fully-qualified remote-tracking ref name of the current
// branch's upstream (its `@{u}`), matching `git rev-parse --symbolic-full-name @{u}` —
// e.g. "refs/remotes/origin/main". It resolves branch.<name>.remote +
// branch.<name>.merge through the remote's OWN configured fetch refspecs (config.RefSpec.
// Dst), rather than assuming the default `refs/remotes/<remote>/*` layout, so a
// non-default fetch refspec resolves the same ref git would use. Errors — matching git's
// own refusal — when HEAD is detached, the branch has no configured upstream, or the
// remote's fetch refspecs do not cover it.
func (r *Repo) UpstreamRef() (string, error) {
	head, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("gitcore: upstream: %w", err)
	}
	if head.Name() == plumbing.HEAD || !head.Name().IsBranch() {
		return "", fmt.Errorf("gitcore: upstream: HEAD is detached")
	}
	branchName := head.Name().Short()
	cfg, err := r.repo.Config()
	if err != nil {
		return "", fmt.Errorf("gitcore: upstream: %w", err)
	}
	bc, ok := cfg.Branches[branchName]
	if !ok || bc.Remote == "" || bc.Merge == "" {
		return "", fmt.Errorf("gitcore: upstream: no upstream configured for %s", branchName)
	}
	rc, ok := cfg.Remotes[bc.Remote]
	if !ok {
		return "", fmt.Errorf("gitcore: upstream: remote %s not configured", bc.Remote)
	}
	for _, rs := range rc.Fetch {
		if rs.Match(bc.Merge) {
			return string(rs.Dst(bc.Merge)), nil
		}
	}
	return "", fmt.Errorf("gitcore: upstream: no fetch refspec of remote %s maps %s", bc.Remote, bc.Merge)
}

// AheadCount returns the number of commits reachable from head that are not reachable
// from base, matching `git rev-list --count <base>..<head>` for the common
// fast-forward-descendant case every caller uses this for (head is a descendant of
// base). It is computed as the count of head's ancestors walked before reaching one
// that is also an ancestor of base, which is exactly rev-list's two-dot count when that
// precondition holds.
func (r *Repo) AheadCount(base, head string) (int, error) {
	baseHash, err := r.Resolve(base)
	if err != nil {
		return 0, err
	}
	headHash, err := r.Resolve(head)
	if err != nil {
		return 0, err
	}
	baseAncestors := map[plumbing.Hash]bool{}
	iter, err := r.repo.Log(&git.LogOptions{From: baseHash})
	if err != nil {
		return 0, fmt.Errorf("gitcore: ahead-count: %w", err)
	}
	err = iter.ForEach(func(c *object.Commit) error {
		baseAncestors[c.Hash] = true
		return nil
	})
	iter.Close()
	if err != nil {
		return 0, fmt.Errorf("gitcore: ahead-count: %w", err)
	}
	headIter, err := r.repo.Log(&git.LogOptions{From: headHash})
	if err != nil {
		return 0, fmt.Errorf("gitcore: ahead-count: %w", err)
	}
	defer headIter.Close()
	count := 0
	err = headIter.ForEach(func(c *object.Commit) error {
		if baseAncestors[c.Hash] {
			return storer.ErrStop
		}
		count++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("gitcore: ahead-count: %w", err)
	}
	return count, nil
}

// RemoteURL returns the URL configured for the named remote, matching
// `git config --get remote.<name>.url` / `git ls-remote --get-url <name>` — both read
// the SAME value; the brief's own facts note that `ls-remote --get-url` collapses to
// this read with no `insteadOf` layer to expand.
func (r *Repo) RemoteURL(name string) (string, error) {
	remote, err := r.repo.Remote(name)
	if err != nil {
		return "", fmt.Errorf("gitcore: remote-url %s: %w", name, err)
	}
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return "", fmt.Errorf("gitcore: remote-url %s: no URL configured", name)
	}
	return urls[0], nil
}

// CommitVerifyQuiet reports whether rev resolves to a commit, matching
// `git rev-parse --verify --quiet <rev>^{commit}` — including --quiet's own behaviour of
// reporting false rather than erroring when rev does not resolve.
func (r *Repo) CommitVerifyQuiet(rev string) (bool, error) {
	hash, err := r.Resolve(rev)
	if err != nil {
		return false, nil
	}
	if _, err := r.repo.CommitObject(hash); err != nil {
		return false, nil
	}
	return true, nil
}

// HasStagedChanges reports whether the index holds content not yet committed, matching
// `git diff --cached --quiet`'s exit code (1 = staged changes exist, 0 = none).
func (r *Repo) HasStagedChanges() (bool, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("gitcore: staged-changes: %w", err)
	}
	st, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("gitcore: staged-changes: %w", err)
	}
	for _, fs := range st {
		// Untracked means "not in the index at all" — not staged, and therefore not a
		// difference between the index and HEAD. `git diff --cached --quiet` never
		// trips on an untracked file; only Added/Modified/Deleted/Renamed/Copied/
		// UpdatedButUnmerged in the STAGING column represent a real staged change.
		if fs.Staging != git.Unmodified && fs.Staging != git.Untracked {
			return true, nil
		}
	}
	return false, nil
}

// DirtyTrackedPorcelain returns the porcelain-style status of TRACKED changes only,
// matching `git status --porcelain --untracked-files=no` — an empty string means clean.
// Entries are sorted by path (git's own porcelain output is already path-sorted; go-git's
// Status is a map, so this sorts explicitly to match).
func (r *Repo) DirtyTrackedPorcelain() (string, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("gitcore: status: %w", err)
	}
	st, err := wt.Status()
	if err != nil {
		return "", fmt.Errorf("gitcore: status: %w", err)
	}
	type row struct{ path, line string }
	var rows []row
	for path, fs := range st {
		if fs.Staging == git.Untracked && fs.Worktree == git.Untracked {
			continue // --untracked-files=no
		}
		disp := path
		if fs.Staging == git.Renamed && fs.Extra != "" {
			disp = fs.Extra + " -> " + path
		}
		rows = append(rows, row{path: path, line: fmt.Sprintf("%c%c %s", fs.Staging, fs.Worktree, disp)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].path < rows[j].path })
	var b strings.Builder
	for _, rr := range rows {
		b.WriteString(rr.line)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// Diff returns the unified diff between the trees at from and to (each a revision
// expression), matching `git diff <from> <to>` at the given context-line count —
// full patch text, headers included, with the same rename detection DiffNames uses.
func (r *Repo) Diff(from, to string, contextLines int) (string, error) {
	fromTree, err := r.treeAt(from)
	if err != nil {
		return "", err
	}
	toTree, err := r.treeAt(to)
	if err != nil {
		return "", err
	}
	changes, err := object.DiffTreeWithOptions(context.Background(), fromTree, toTree, renameDetectOptions)
	if err != nil {
		return "", fmt.Errorf("gitcore: diff %s..%s: %w", from, to, err)
	}
	patch, err := changes.PatchContext(context.Background())
	if err != nil {
		return "", fmt.Errorf("gitcore: diff %s..%s: patch: %w", from, to, err)
	}
	var buf strings.Builder
	enc := fdiff.NewUnifiedEncoder(&buf, contextLines)
	if err := enc.Encode(patch); err != nil {
		return "", fmt.Errorf("gitcore: diff %s..%s: encode: %w", from, to, err)
	}
	return buf.String(), nil
}

// DiffSymmetric returns the unified diff git's three-dot form produces, matching
// `git diff <a>...<b>`: the diff between b and the merge-base of a and b (NOT a
// straight two-dot diff between a and b).
func (r *Repo) DiffSymmetric(a, b string, contextLines int) (string, error) {
	base, err := r.MergeBase(a, b)
	if err != nil {
		return "", err
	}
	return r.Diff(base, b, contextLines)
}

// TreeishID returns the object id of path within the tree rev resolves to, matching
// `git rev-parse <rev>:<path>` — a tree id for a directory, a blob id for a file.
func (r *Repo) TreeishID(rev, path string) (string, error) {
	tree, err := r.treeAt(rev)
	if err != nil {
		return "", err
	}
	entry, err := tree.FindEntry(path)
	if err != nil {
		return "", fmt.Errorf("gitcore: treeish %s:%s: %w", rev, path, err)
	}
	return entry.Hash.String(), nil
}

// LocalBranchNames returns every local branch's short name, matching
// `git for-each-ref --format=%(refname:strip=2) refs/heads/`.
func (r *Repo) LocalBranchNames() ([]string, error) {
	refs, err := r.Refs()
	if err != nil {
		return nil, err
	}
	var out []string
	const prefix = "refs/heads/"
	for name := range refs {
		if strings.HasPrefix(name, prefix) {
			out = append(out, strings.TrimPrefix(name, prefix))
		}
	}
	sort.Strings(out)
	return out, nil
}

// RefsContaining returns every ref under refsPrefix whose tip has commit as an ancestor
// (or is equal to it), matching
// `git for-each-ref --contains=<commit> --format=%(refname) <refsPrefix>`.
//
// This is built on Refs, which (like go-git's underlying reference iteration) does not
// surface a SYMBOLIC ref such as refs/remotes/<name>/HEAD — real git's for-each-ref does
// list it. That is harmless for every current caller: such a ref is always an alias for
// another hash ref under the same prefix (e.g. refs/remotes/origin/HEAD mirrors
// refs/remotes/origin/main), which Refs does return, so a commit reachable via the alias
// is always ALSO reachable via the ref it mirrors. This helper must not be reused for a
// check that needs to see the alias ref itself (see ambiguousbase.go's refCandidates,
// deliberately NOT migrated in this brief for exactly that reason).
func (r *Repo) RefsContaining(commit, refsPrefix string) ([]string, error) {
	refs, err := r.Refs()
	if err != nil {
		return nil, err
	}
	var out []string
	for name, hash := range refs {
		if !strings.HasPrefix(name, refsPrefix) {
			continue
		}
		ok, err := r.IsAncestor(commit, hash)
		if err != nil {
			continue // an unresolvable tip is skipped, matching for-each-ref's own behaviour
		}
		if ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// --- Read helpers added for brief 04 (migrate deskpushguard detection reads) -----------
//
// deskpushguard is a security-DETECTION control (foreign-commit / merge-masquerade /
// register-id-collision checks), so its seam swap needs two per-commit fields no existing
// helper exposes: a commit's subject line and its parent hashes. Both are plain
// object.Commit field reads, verified against real git in gitcore_test.go.

// CommitSubject returns the subject line of rev's commit message, matching
// `git log -1 --format=%s <rev>`: the first line of the raw commit message. A CRLF-authored
// message's trailing "\r" is trimmed so it reads identically to an LF one, matching git's
// own line-ending normalisation of commit message content.
func (r *Repo) CommitSubject(rev string) (string, error) {
	hash, err := r.Resolve(rev)
	if err != nil {
		return "", err
	}
	commit, err := r.repo.CommitObject(hash)
	if err != nil {
		return "", fmt.Errorf("gitcore: commit %s: %w", hash, err)
	}
	subject, _, _ := strings.Cut(commit.Message, "\n")
	return strings.TrimRight(subject, "\r"), nil
}

// ParentHashes returns the full hex object ids of rev's parent commits, in the commit's own
// parent order, matching `git log -1 --format=%P <rev>` split on whitespace (an empty slice
// for a root commit, exactly as %P prints an empty string for one).
func (r *Repo) ParentHashes(rev string) ([]string, error) {
	hash, err := r.Resolve(rev)
	if err != nil {
		return nil, err
	}
	commit, err := r.repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("gitcore: commit %s: %w", hash, err)
	}
	out := make([]string, len(commit.ParentHashes))
	for i, p := range commit.ParentHashes {
		out[i] = p.String()
	}
	return out, nil
}

// ChangeStatus is one path's git diff --name-status status code paired with the path it
// applies to (the NEW path, for a detected rename).
type ChangeStatus struct {
	Status string // "A" (added), "M" (modified), "D" (deleted), or "R" (renamed)
	Path   string
}

// DiffNameStatus returns the per-path change status between from and to (each a revision
// expression), matching `git diff --name-status <from> <to>` — including rename detection
// at the same threshold DiffNames uses, so a detected rename is reported as a single "R"
// entry naming the new path exactly as git's own --name-status does. This package runs no
// copy detection, matching git diff's own default (`-C` is off unless requested), so a copy
// surfaces as a plain "A" of the new path.
func (r *Repo) DiffNameStatus(from, to string) ([]ChangeStatus, error) {
	fromTree, err := r.treeAt(from)
	if err != nil {
		return nil, err
	}
	toTree, err := r.treeAt(to)
	if err != nil {
		return nil, err
	}
	changes, err := object.DiffTreeWithOptions(context.Background(), fromTree, toTree, renameDetectOptions)
	if err != nil {
		return nil, fmt.Errorf("gitcore: diff %s..%s: %w", from, to, err)
	}
	var out []ChangeStatus
	for _, c := range changes {
		switch {
		case c.From.Name != "" && c.To.Name != "" && c.From.Name != c.To.Name:
			out = append(out, ChangeStatus{Status: "R", Path: c.To.Name})
		case c.From.Name == "":
			out = append(out, ChangeStatus{Status: "A", Path: c.To.Name})
		case c.To.Name == "":
			out = append(out, ChangeStatus{Status: "D", Path: c.From.Name})
		default:
			out = append(out, ChangeStatus{Status: "M", Path: c.To.Name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
