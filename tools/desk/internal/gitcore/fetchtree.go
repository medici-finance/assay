package gitcore

// fetchtree.go — fetch a remote's branches into MEMORY and write one commit's tree to a
// directory, for a caller that needs a remote tree on disk but must never have a repository
// (or a credential) there. deskadvisory is that caller: it runs check tools over a fork's
// tree, and what that tree is fetched with — a short-lived installation token — must not
// land in a .git/config, a credential file or an askpass script beside it.
//
// There is no work tree checkout in the `git checkout` sense: the tree is MATERIALISED, and
// materialisation is deliberately narrower than a checkout, because the tree is attacker-shaped
// input and the consumer runs tools over it:
//
//   - only regular and executable files are written. A symlink entry is skipped (a link out of
//     the tree is the way a tool reading it escapes it) and so is a submodule (a gitlink, which
//     a checkout leaves as an empty directory and which names a commit in a repository this
//     fetch never contacts);
//   - an entry whose path is not local to the destination (absolute, "..", empty element), or
//     which has a ".git" element in any letter case (a case-insensitive filesystem would make
//     it the repository's own metadata), refuses the whole write rather than being skipped:
//     a tree that carries one is hostile, not merely unusual. These checks are a SECOND
//     layer: the pinned go-git tree walker already refuses such entries while decoding, so
//     no fixture reaches them today; they stay so the guarantee does not rest on the library;
//   - files are created exclusively (O_EXCL), so nothing already at the path — a planted
//     symlink included — is followed.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

// TreeOpts configures FetchTree.
type TreeOpts struct {
	URL      string               // the full remote URL, caller-built; the only place Auth is sent
	RefSpecs []string             // what to fetch, e.g. "+refs/heads/*:refs/heads/*"
	Auth     transport.AuthMethod // scoped to this call, held in memory only
	Commit   string               // the 40-hex commit whose tree is written; must be reachable from RefSpecs
}

// TreeResult reports what a materialisation did and did not write.
type TreeResult struct {
	Files   int // regular and executable files written
	Skipped int // symlinks and submodules left unwritten
}

// FetchTree fetches RefSpecs from URL into an in-memory repository and writes the tree of
// Commit under dest (which must exist). Nothing is written to disk but the tree's files; no
// .git directory, config or credential file is created, and no git binary is spawned.
func FetchTree(opts TreeOpts, dest string) (TreeResult, error) {
	var res TreeResult
	if !isFullHex(opts.Commit) {
		return res, fmt.Errorf("gitcore: fetch-tree: %q is not a full 40-hex commit id", opts.Commit)
	}
	specs, err := buildRefSpecs(opts.RefSpecs, false)
	if err != nil {
		return res, err
	}
	store := memory.NewStorage()
	repo, err := git.Init(store, nil)
	if err != nil {
		return res, fmt.Errorf("gitcore: fetch-tree: init in-memory repository: %w", err)
	}
	remote := git.NewRemote(store, &config.RemoteConfig{
		Name: transientRemoteName,
		URLs: []string{opts.URL},
	})
	if ferr := remote.Fetch(&git.FetchOptions{RefSpecs: specs, Auth: opts.Auth, Force: true}); ferr != nil && ferr != git.NoErrAlreadyUpToDate {
		return res, fmt.Errorf("gitcore: fetch-tree: fetch: %w", ferr)
	}
	commit, err := repo.CommitObject(plumbing.NewHash(opts.Commit))
	if err != nil {
		return res, fmt.Errorf("gitcore: fetch-tree: commit %s is not in what was fetched: %w", opts.Commit, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return res, fmt.Errorf("gitcore: fetch-tree: tree of %s: %w", opts.Commit, err)
	}
	return materialise(tree, dest)
}

func isFullHex(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// materialise writes tree's regular files under dest. See the file header for the rules.
func materialise(tree *object.Tree, dest string) (TreeResult, error) {
	var res TreeResult
	walker := object.NewTreeWalker(tree, true, nil)
	defer walker.Close()
	for {
		name, entry, err := walker.Next()
		if err == io.EOF {
			return res, nil
		}
		if err != nil {
			return res, fmt.Errorf("gitcore: materialise: walk tree: %w", err)
		}
		switch entry.Mode {
		case filemode.Dir:
			continue
		case filemode.Symlink, filemode.Submodule:
			res.Skipped++
			continue
		case filemode.Regular, filemode.Deprecated, filemode.Executable:
		default:
			res.Skipped++
			continue
		}
		if !filepath.IsLocal(name) {
			return res, fmt.Errorf("gitcore: materialise: refusing tree entry %q: not a path inside the destination", name)
		}
		for _, el := range strings.Split(name, "/") {
			if strings.EqualFold(el, ".git") || el == "" {
				return res, fmt.Errorf("gitcore: materialise: refusing tree entry %q: it has a .git or empty path element", name)
			}
		}
		file, err := tree.TreeEntryFile(&entry)
		if err != nil {
			return res, fmt.Errorf("gitcore: materialise: read %s: %w", name, err)
		}
		if err := writeTreeFile(dest, name, file, entry.Mode == filemode.Executable); err != nil {
			return res, err
		}
		res.Files++
	}
}

func writeTreeFile(dest, name string, file *object.File, exec bool) error {
	target := filepath.Join(dest, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("gitcore: materialise: mkdir for %s: %w", name, err)
	}
	perm := os.FileMode(0o644)
	if exec {
		perm = 0o755
	}
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("gitcore: materialise: create %s: %w", name, err)
	}
	rc, err := file.Reader()
	if err != nil {
		out.Close()
		return fmt.Errorf("gitcore: materialise: read %s: %w", name, err)
	}
	_, cerr := io.Copy(out, rc)
	rc.Close()
	if clerr := out.Close(); cerr == nil {
		cerr = clerr
	}
	if cerr != nil {
		return fmt.Errorf("gitcore: materialise: write %s: %w", name, cerr)
	}
	return nil
}
