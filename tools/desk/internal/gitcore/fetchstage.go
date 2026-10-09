package gitcore

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// fetchstage.go — how Fetch decides what a fetch WRITES.
//
// go-git's own reference update cannot be trusted with that decision here, for two reasons
// review found at this layer:
//
//   - With tags off (which Fetch needs, so the refspec is the whole write set), go-git v5.19.2
//     returns from its reference update BEFORE turning a refused non-fast-forward update into
//     its "some refs were not updated" error: a non-forced fetch whose origin ref was rewritten
//     reports success while the local ref still names the superseded commit.
//   - Its prune removes every local ref under the refspec destination that the remote does not
//     advertise, symbolic refs included, so refs/remotes/origin/HEAD went on every prune. git
//     never prunes a symbolic ref.
//
// So go-git only TRANSFERS: every refspec is rewritten into a forced one whose destination is
// a private per-call namespace (stageNamespace/<nonce>/<spec index>/<real destination>), and
// gitcore then applies the staged values to the real destinations itself, under git's rules —
// prune runs BEFORE the updates and removes the directories it empties, so a stale ref whose
// name conflicts with a new one (branch X replaced by X/a, or the reverse) is cleared first;
// a non-forced update must fast-forward or it is refused and reported; a new ref whose name
// conflicts with an existing one is refused and reported; one destination that cannot be
// written never stops the others; a symbolic ref is never written through and never pruned —
// and drops the staging namespace on every path.

// stageNamespace is the ref namespace a Fetch stages the advertised values in. A refspec whose
// destination lies inside it is refused, so a caller can never write or prune a staged ref.
const stageNamespace = "refs/gitcore-fetch/"

// ErrRefsNotUpdated is wrapped by the error Fetch returns when one or more destinations could
// not be updated to the value the origin advertised (an update that is not a fast-forward
// without force, a destination that is a symbolic ref, a new name that conflicts with an
// existing ref, or a write that failed) or a stale ref could not be pruned. Every other
// destination was still applied, as `git fetch` does; the error names each one that was not.
var ErrRefsNotUpdated = errors.New("some refs were not updated")

// stagedSpec is one caller refspec and the forced staging refspec go-git fetches it as.
type stagedSpec struct {
	real   config.RefSpec
	prefix string // stage + index + "/": strip it from a staged ref name to get the real destination
	staged config.RefSpec
}

// stageRefSpecs rewrites specs into forced refspecs under a fresh staging namespace, and
// returns that namespace ("refs/gitcore-fetch/<nonce>/").
func stageRefSpecs(specs []config.RefSpec) (string, []stagedSpec, error) {
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", nil, fmt.Errorf("gitcore: fetch: staging nonce: %w", err)
	}
	stage := stageNamespace + hex.EncodeToString(nonce[:]) + "/"
	out := make([]stagedSpec, 0, len(specs))
	for i, rs := range specs {
		body := strings.TrimPrefix(rs.String(), "+")
		src, dst, ok := strings.Cut(body, ":")
		if !ok || src == "" || !strings.HasPrefix(dst, "refs/") || strings.HasPrefix(dst, stageNamespace) {
			return "", nil, fmt.Errorf("gitcore: fetch: refspec %q needs a full refs/ destination outside %s", rs.String(), stageNamespace)
		}
		prefix := stage + strconv.Itoa(i) + "/"
		staged := config.RefSpec("+" + src + ":" + prefix + dst)
		if err := staged.Validate(); err != nil {
			return "", nil, fmt.Errorf("gitcore: fetch: staging refspec for %q: %w", rs.String(), err)
		}
		out = append(out, stagedSpec{real: rs, prefix: prefix, staged: staged})
	}
	return stage, out, nil
}

// stagedUpdate is one advertised value, read back from the staging namespace.
type stagedUpdate struct {
	dst   plumbing.ReferenceName
	hash  plumbing.Hash
	force bool
}

// applyStaged applies the fetched values as `git fetch` does: first (when prune is set) it
// drops each local ref a refspec covers that the origin no longer advertises — never a
// symbolic ref — removing the directories that leaves empty; then it writes every staged
// value to its real destination under git's rules. It returns an error wrapping
// ErrRefsNotUpdated naming every destination it refused or could not write and every stale
// ref it could not prune; everything else is still applied.
//
// The order is git's (prune before update) and it matters: a stale tracking ref whose NAME
// conflicts with a new one — upstream replaced branch X with X/a, or X/a with X — cannot
// coexist with it on disk, so updating first would refuse the new name and leave the stale
// one in place on every run.
func (r *Repo) applyStaged(specs []stagedSpec, prune bool) error {
	st := r.repo.Storer
	iter, err := st.IterReferences()
	if err != nil {
		return fmt.Errorf("gitcore: fetch: list refs: %w", err)
	}
	var updates []stagedUpdate
	advertised := map[plumbing.ReferenceName]bool{}
	var local []*plumbing.Reference
	names := newRefNameSet()
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		names.add(name)
		if !strings.HasPrefix(name, stageNamespace) {
			local = append(local, ref)
			return nil
		}
		for _, s := range specs {
			if rest, ok := strings.CutPrefix(name, s.prefix); ok {
				if ref.Type() != plumbing.HashReference {
					return fmt.Errorf("gitcore: fetch: staged ref %s is not a hash reference", name)
				}
				dst := plumbing.ReferenceName(rest)
				updates = append(updates, stagedUpdate{dst: dst, hash: ref.Hash(), force: s.real.IsForceUpdate()})
				advertised[dst] = true
				break
			}
		}
		return nil
	})
	iter.Close()
	if err != nil {
		return err
	}
	sort.SliceStable(updates, func(i, j int) bool { return updates[i].dst < updates[j].dst })

	refsDir, _ := r.commonRefsDir() // "" when unresolvable: no directory is then touched
	var refused []string

	if prune {
		for _, ref := range local {
			// git never prunes a symbolic ref (refs/remotes/origin/HEAD names the origin's
			// default branch; no origin advertises a branch called HEAD, so it would go every time).
			if ref.Type() != plumbing.HashReference || advertised[ref.Name()] {
				continue
			}
			for _, s := range specs {
				if s.real.Reverse().Match(ref.Name()) {
					if err := st.RemoveReference(ref.Name()); err != nil {
						refused = append(refused, ref.Name().String()+" (could not be pruned: "+err.Error()+")")
						break
					}
					names.remove(ref.Name().String())
					removeEmptyParents(refsDir, ref.Name().String())
					break
				}
			}
		}
	}

	for _, u := range updates {
		old, rerr := st.Reference(u.dst)
		if errors.Is(rerr, plumbing.ErrReferenceNotFound) {
			old, rerr = nil, nil
		}
		if rerr != nil {
			refused = append(refused, u.dst.String()+" (could not be read: "+rerr.Error()+")")
			continue
		}
		if old != nil {
			if old.Type() != plumbing.HashReference {
				refused = append(refused, u.dst.String()+" (a symbolic ref; not written through)")
				continue
			}
			if old.Hash() == u.hash {
				continue
			}
			if !u.force {
				// git refuses to move an existing tag without force, fast-forward or not.
				if strings.HasPrefix(u.dst.String(), "refs/tags/") {
					refused = append(refused, u.dst.String()+" (would clobber an existing tag)")
					continue
				}
				if ok, why := r.fastForwards(old.Hash(), u.hash); !ok {
					refused = append(refused, u.dst.String()+" ("+why+")")
					continue
				}
			}
		} else {
			// A new name must not be a directory of an existing ref, nor have one as a directory:
			// git refuses that pair, and so does a loose ref store.
			if conflict := names.conflict(u.dst.String()); conflict != "" {
				refused = append(refused, u.dst.String()+" (cannot create it: "+conflict+" exists)")
				continue
			}
			// As git does, an empty directory left where the new ref goes is cleared first.
			removeEmptyTree(refsDir, u.dst.String())
		}
		if err := st.CheckAndSetReference(plumbing.NewHashReference(u.dst, u.hash), old); err != nil {
			refused = append(refused, u.dst.String()+" (could not be written: "+err.Error()+")")
			continue
		}
		names.add(u.dst.String())
	}

	if len(refused) > 0 {
		return fmt.Errorf("gitcore: fetch: %w: %s", ErrRefsNotUpdated, strings.Join(refused, ", "))
	}
	return nil
}

// refNameSet is the set of ref names in the repository, kept current as applyStaged prunes
// and writes, with the count of names under each directory prefix — so it can tell, as git
// does, when a new name and an existing one cannot both exist (one is a directory of the
// other), whether the existing one is loose or packed.
type refNameSet struct {
	names map[string]bool
	under map[string]int // "refs/remotes/origin/feat" -> how many names lie below it
}

func newRefNameSet() *refNameSet {
	return &refNameSet{names: map[string]bool{}, under: map[string]int{}}
}

func (s *refNameSet) add(name string) {
	if s.names[name] {
		return
	}
	s.names[name] = true
	for i := 0; i < len(name); i++ {
		if name[i] == '/' {
			s.under[name[:i]]++
		}
	}
}

func (s *refNameSet) remove(name string) {
	if !s.names[name] {
		return
	}
	delete(s.names, name)
	for i := 0; i < len(name); i++ {
		if name[i] == '/' {
			s.under[name[:i]]--
		}
	}
}

// conflict returns the existing ref (or "<name>/…" for refs below it) that a new ref called
// name cannot coexist with, or "" when there is none.
func (s *refNameSet) conflict(name string) string {
	for i := 0; i < len(name); i++ {
		if name[i] == '/' && s.names[name[:i]] {
			return name[:i]
		}
	}
	if s.under[name] > 0 {
		return name + "/…"
	}
	return ""
}

// looseRefPath is where ref name's loose file lives under refsDir, or "" when refsDir is
// unknown or name is not a plain refs/ name (no empty, "." or ".." component).
func looseRefPath(refsDir, name string) string {
	rel, ok := strings.CutPrefix(name, "refs/")
	if refsDir == "" || !ok || rel == "" {
		return ""
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			return ""
		}
	}
	return filepath.Join(refsDir, filepath.FromSlash(rel))
}

// removeEmptyParents removes the directories a pruned ref's removal left empty, deepest first,
// as git does — never refs/ or refs/<kind>/ themselves. Without it a pruned loose X/a leaves a
// directory X that a ref named X can never be written over.
func removeEmptyParents(refsDir, name string) {
	path := looseRefPath(refsDir, name)
	if path == "" {
		return
	}
	keep := len(strings.Split(strings.TrimPrefix(name, "refs/"), "/"))
	for dir := filepath.Dir(path); keep > 2; keep-- {
		if os.Remove(dir) != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// removeEmptyTree clears a directory tree that holds no files from where a new loose ref is
// about to be written, as git does; a tree holding any file is left alone (the write then fails
// and is reported).
func removeEmptyTree(refsDir, name string) {
	path := looseRefPath(refsDir, name)
	if path == "" {
		return
	}
	if fi, err := os.Lstat(path); err != nil || !fi.IsDir() {
		return
	}
	removeIfEmptyTree(path)
}

func removeIfEmptyTree(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() || !removeIfEmptyTree(filepath.Join(dir, e.Name())) {
			return false
		}
	}
	return os.Remove(dir) == nil
}

// fastForwards reports whether moving a ref from old to new is a fast-forward (old is an
// ancestor of new). Anything it cannot establish is NOT a fast-forward, with the reason.
func (r *Repo) fastForwards(old, new plumbing.Hash) (bool, string) {
	oc, err := r.repo.CommitObject(old)
	if err != nil {
		return false, "cannot read the local commit " + old.String() + ": " + err.Error()
	}
	nc, err := r.repo.CommitObject(new)
	if err != nil {
		return false, "cannot read the fetched commit " + new.String() + ": " + err.Error()
	}
	ok, err := oc.IsAncestor(nc)
	if err != nil {
		return false, "cannot tell whether it fast-forwards: " + err.Error()
	}
	if !ok {
		return false, "non-fast-forward: the origin's commit " + new.String()[:12] + " does not descend from the local " + old.String()[:12]
	}
	return true, ""
}

// dropStage removes every ref under stage. The common case is loose ref files only, removed
// with their directory in one pass; a staged ref a concurrent `git pack-refs` packed meanwhile
// is then removed through the storer, which rewrites packed-refs without it.
func (r *Repo) dropStage(stage string) error {
	if refsDir, err := r.commonRefsDir(); err == nil {
		// Only this call's own nonce directory: the shared parent stays, so a concurrent
		// Fetch creating its own namespace under it is never raced.
		_ = os.RemoveAll(filepath.Join(refsDir, filepath.FromSlash(strings.TrimPrefix(stage, "refs/"))))
	}
	st := r.repo.Storer
	iter, err := st.IterReferences()
	if err != nil {
		return fmt.Errorf("gitcore: fetch: list refs to drop the staging namespace: %w", err)
	}
	var left []plumbing.ReferenceName
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		if strings.HasPrefix(ref.Name().String(), stage) {
			left = append(left, ref.Name())
		}
		return nil
	})
	iter.Close()
	for _, name := range left {
		if err := st.RemoveReference(name); err != nil {
			return fmt.Errorf("gitcore: fetch: drop staged ref %s: %w", name, err)
		}
	}
	return nil
}

// commonRefsDir is the refs/ directory of this repository's common git directory (where a
// linked worktree's refs live too).
func (r *Repo) commonRefsDir() (string, error) {
	gitDir, _, err := resolveGitDir(r.dir)
	if err != nil {
		return "", err
	}
	if common, cerr := commonDirOf(gitDir); cerr == nil && common != "" {
		gitDir = common
	}
	return filepath.Join(gitDir, "refs"), nil
}
