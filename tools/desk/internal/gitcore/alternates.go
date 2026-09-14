package gitcore

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/chroot"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ErrUnsupportedAlternates reports that this repository borrows objects from an
// alternate object store (objects/info/alternates) that gitcore cannot resolve — so
// some objects git itself can read are invisible here. Callers MUST treat a read that
// wraps it as could-not-check, never as a clean or a dirty answer.
var ErrUnsupportedAlternates = errors.New("unsupported alternate object store")

// ErrObjectStoreIncomplete reports that an object the repository's own history
// references could not be read from this checkout's object store. It is the honest
// outcome for a read whose answer would otherwise be silently WRONG — see
// headTreeComplete, and go-git's tree-walk truncation documented there.
var ErrObjectStoreIncomplete = errors.New("object store incomplete")

// alternates is the resolved state of one repository's objects/info/alternates file:
// the filesystem go-git should resolve alternate object directories against, plus a
// human-readable diagnosis of any entry that could NOT be resolved. Both may be empty
// (no alternates file, or an empty one) — that is the ordinary case.
type alternates struct {
	// fs is the billy filesystem handed to go-git as filesystem.Options.AlternatesFS.
	// nil when there is nothing resolvable to point it at.
	fs billy.Filesystem
	// diagnosis names the entries that could not be used, for the could-not-check
	// message a caller surfaces. Empty when every entry resolved.
	diagnosis string
}

// readAlternates resolves the alternate object directories declared by objectsDir's
// own info/alternates file (objectsDir is <common .git>/objects).
//
// WHY THIS EXISTS. `git clone --shared` (and `git clone --reference`) does not copy
// objects; it writes the SOURCE repository's absolute objects path into
// objects/info/alternates and borrows from it. go-git honours that file — but only
// through the filesystem it is given, and gitcore hands it a billy chroot rooted at
// the repository's own .git (it must: that is what makes every other path go-git
// touches stay inside the repository). An alternate path outside that chroot cannot be
// stat'ed through it, so go-git's DotGit.Alternates fails, ObjectStorage.EncodedObject
// discards that failure (`if e == nil`), and every borrowed object simply reads as "not
// found". The fix is go-git's own escape hatch for exactly this: filesystem.Options.
// AlternatesFS, "the billy filesystem to be used for Git Alternates".
//
// THE BOUNDARY. That filesystem is NOT rooted at "/" — it is rooted at the nearest
// common ancestor of the directories THIS repository already declares it borrows from,
// so go-git's reach grows by exactly the subtree the repository itself names and not one
// directory more. Entries gitcore cannot resolve within that boundary (a relative entry,
// a C-quoted entry, a directory that is missing or is not a directory) are NOT resolved
// by guesswork: they are reported in diagnosis, and the reads that depend on them fail
// honestly through ErrObjectStoreIncomplete rather than returning a wrong answer.
func readAlternates(objectsDir string) alternates {
	f, err := os.Open(filepath.Join(objectsDir, "info", "alternates"))
	if err != nil {
		// No alternates file is the ordinary case; an unreadable one is reported as a
		// diagnosis rather than an error, since it only matters if an object is missing.
		if os.IsNotExist(err) {
			return alternates{}
		}
		return alternates{diagnosis: fmt.Sprintf("objects/info/alternates is unreadable: %v", err)}
	}
	defer f.Close()

	var resolved, problems []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		entry := strings.TrimSpace(scanner.Text())
		// Blank lines carry nothing. A '#' line is not a documented path form either;
		// git would treat it as a nonexistent directory and ignore it, so skipping it
		// here reaches the same outcome without manufacturing a false diagnosis.
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		if strings.HasPrefix(entry, `"`) {
			// git's parse_alt_odb_entry unquotes C-style quoted paths. Decoding that
			// escaping here would be guessing at a form we have no fixture for, so it
			// is declared unsupported instead of half-supported.
			problems = append(problems, fmt.Sprintf("C-quoted entry %s is not supported", entry))
			continue
		}
		if !filepath.IsAbs(entry) {
			// Git resolves a relative entry against objects/. go-git cannot: it treats a
			// relative entry as rooted at the AlternatesFS root, deliberately refusing to
			// cross its chroot boundary (see its own comment in DotGit.Alternates). We
			// therefore cannot make go-git read it, and we will not point the boundary
			// somewhere that makes it accidentally resolve to a different store.
			problems = append(problems, fmt.Sprintf("relative entry %q is not supported (only absolute alternate paths are)", entry))
			continue
		}
		clean := filepath.Clean(entry)
		fi, serr := os.Stat(clean)
		switch {
		case serr != nil:
			problems = append(problems, fmt.Sprintf("alternate object directory %q is unreadable: %v", clean, serr))
		case !fi.IsDir():
			problems = append(problems, fmt.Sprintf("alternate object directory %q is not a directory", clean))
		default:
			resolved = append(resolved, clean)
		}
	}
	if serr := scanner.Err(); serr != nil {
		problems = append(problems, fmt.Sprintf("objects/info/alternates could not be read to the end: %v", serr))
	}

	out := alternates{diagnosis: strings.Join(problems, "; ")}
	if len(resolved) == 0 {
		return out
	}
	root, rerr := alternatesRoot(resolved)
	if rerr != nil {
		out.diagnosis = strings.TrimPrefix(out.diagnosis+"; "+rerr.Error(), "; ")
		return out
	}
	// NOT osfs.New(root): osfs.New EvalSymlinks-resolves the directory it is rooted at,
	// and go-git makes an absolute alternates entry relative to that root by TEXT
	// (filepath.Rel(fs.Root(), entry)). On any system where the path holds a symlinked
	// prefix — macOS temp dirs are /var/… while the resolved form is /private/var/…, and
	// a symlinked home or checkout directory does the same — the resolved root is not a
	// textual prefix of the entry, Rel yields a "../.." path that cannot leave the chroot,
	// and every alternate silently fails to resolve again. chroot.New roots at the path as
	// given (the OS still resolves symlinks when the file is actually opened), which keeps
	// the root a prefix of the entries it was computed from. The type is the same
	// *chroot.ChrootHelper that go-git's DotGit.Alternates checks for.
	out.fs = chroot.New(osfs.Default, root)
	return out
}

// headTreeComplete reports whether every TREE object reachable from HEAD's commit tree
// can actually be read from this checkout's object store, returning an error wrapping
// ErrObjectStoreIncomplete when one cannot.
//
// WHY A STATUS READ NEEDS THIS. go-git's Worktree.Status diffs the HEAD tree against the
// index, and its tree walk does not fail when a subtree object is missing: object.
// TreeWalker.Next turns a failed GetTree into io.EOF (`if err != nil { err = io.EOF }`),
// which every caller reads as "no more entries". The walk therefore TRUNCATES silently,
// and Status reports a tree that is merely unreadable as a tree that is merely smaller.
// Both directions of that are wrong, and both matter here:
//
//   - Index entries whose HEAD-side entry was truncated away read as staged ADDITIONS —
//     the false "staged-but-uncommitted changes" refusal on a clean shared clone.
//   - A genuinely staged DELETION of a path that was truncated away disappears from the
//     diff entirely, so a dirty index could read as clean. A staged-change check that
//     can return a false clean is worth more than the cost of this walk.
//
// Cost: trees only, blobs are never fetched and each distinct tree is read once, through
// the same object cache Status itself uses. An unborn HEAD (no commit yet) is complete by
// definition — there is no tree to read.
func (r *Repo) headTreeComplete() error {
	ref, err := r.repo.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return nil
		}
		return fmt.Errorf("resolve HEAD: %w", err)
	}
	commit, err := r.repo.CommitObject(ref.Hash())
	if err != nil {
		return r.incompleteErr(fmt.Sprintf("HEAD commit %s", ref.Hash()), err)
	}
	root, err := commit.Tree()
	if err != nil {
		return r.incompleteErr(fmt.Sprintf("tree of HEAD commit %s", ref.Hash()), err)
	}

	seen := map[plumbing.Hash]struct{}{root.Hash: {}}
	stack := []*object.Tree{root}
	for len(stack) > 0 {
		t := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, e := range t.Entries {
			// Only subtrees are walked. A blob is never read here (Status compares blob
			// hashes from the tree entry, it does not inflate them), and a gitlink
			// (filemode.Submodule) points at a commit in ANOTHER repository, whose
			// absence is not this object store being incomplete.
			if e.Mode != filemode.Dir {
				continue
			}
			if _, dup := seen[e.Hash]; dup {
				continue
			}
			seen[e.Hash] = struct{}{}
			sub, serr := object.GetTree(r.repo.Storer, e.Hash)
			if serr != nil {
				return r.incompleteErr(fmt.Sprintf("tree object %s (%s)", e.Hash, e.Name), serr)
			}
			stack = append(stack, sub)
		}
	}
	return nil
}

// incompleteErr builds the could-not-check error for an unreadable object, naming the
// object, the underlying cause, and — when the repository borrows objects — what the
// alternate object store did or did not resolve to, which is the usual reason.
func (r *Repo) incompleteErr(what string, cause error) error {
	if r.alternates.diagnosis != "" {
		// Wrapped, not just printed: a caller can errors.Is this to ErrUnsupportedAlternates
		// and say "this checkout's alternate object store is the problem, repack or re-clone".
		return fmt.Errorf("%w: cannot read %s in %s: %v (%w: %s)",
			ErrObjectStoreIncomplete, what, r.dir, cause, ErrUnsupportedAlternates, r.alternates.diagnosis)
	}
	if r.alternates.fs != nil {
		return fmt.Errorf("%w: cannot read %s in %s: %v (this checkout borrows objects from an alternate object store, which no longer holds it)",
			ErrObjectStoreIncomplete, what, r.dir, cause)
	}
	return fmt.Errorf("%w: cannot read %s in %s: %v", ErrObjectStoreIncomplete, what, r.dir, cause)
}

// alternatesRoot returns the directory the alternates filesystem is rooted at: the
// nearest common ancestor of every resolved alternate path, and never one of those
// paths itself.
//
// The "never one of those paths itself" clause is load-bearing. go-git chroots at
// filepath.Dir(<path relative to the root>) and then treats THAT as a .git directory,
// joining "objects" onto it — so if the root were the alternate objects directory, the
// relative path would be "." and go-git would look for <objects>/objects. Handing it the
// parent keeps the relative path at least one segment long.
func alternatesRoot(paths []string) (string, error) {
	root := paths[0]
	for _, p := range paths[1:] {
		root = commonAncestor(root, p)
		if root == "" {
			return "", fmt.Errorf("alternate object directories %q and %q share no common parent directory", paths[0], p)
		}
	}
	for _, p := range paths {
		if root == p {
			root = filepath.Dir(root)
			break
		}
	}
	if root == "" || root == string(filepath.Separator) || root == filepath.VolumeName(root) {
		// A root of "/" would hand go-git the whole filesystem. Refuse rather than widen:
		// alternates that share no closer ancestor than the filesystem root are rare
		// enough that could-not-check is the better answer.
		return "", errors.New("alternate object directories share no common parent below the filesystem root")
	}
	return root, nil
}

// commonAncestor returns the longest directory prefix a and b share, or "" when they
// share none (different volumes on Windows, say). Both inputs are absolute and clean.
func commonAncestor(a, b string) string {
	as := strings.Split(a, string(filepath.Separator))
	bs := strings.Split(b, string(filepath.Separator))
	n := len(as)
	if len(bs) < n {
		n = len(bs)
	}
	i := 0
	for i < n && as[i] == bs[i] {
		i++
	}
	if i == 0 {
		return ""
	}
	joined := strings.Join(as[:i], string(filepath.Separator))
	if joined == "" {
		// The shared prefix is only the leading empty segment of an absolute path,
		// i.e. the filesystem root itself.
		return string(filepath.Separator)
	}
	return joined
}
