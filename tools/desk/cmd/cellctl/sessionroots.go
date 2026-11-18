package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/gitcore"
)

// Per-session verifier roots.
//
// The verifier's queue read refuses a stream root whose HEAD is not that root's fetched
// refs/remotes/origin/main (deskkit VerifierAssociationPass.Snapshot). A shared operator checkout
// sits on a branch the operator moves, and any other session's fetch moves origin/main in the
// shared object database, so the shared root is stale again minutes after anyone repaired it.
//
// A house verify-desk window therefore never reads the shared checkouts. At boot it cuts one
// detached worktree per CELL_ROOTS entry, under the cell, at that root's freshly fetched
// origin/main, and exports DESK_ROOTS pointing at those trees (names unchanged). Before every
// cadence pass the trees are fetched and moved to origin/main again. The shared checkouts are
// read (to fetch into their database) and never moved. CELL_VERIFY_SHARED_ROOTS=1 opts out.

const (
	verifyRootsRole   = "verify-desk"
	verifyRootsDir    = "verify-desk-roots"
	verifyRootsOptOut = "CELL_VERIFY_SHARED_ROOTS"
)

// sessionRoot is one CELL_ROOTS entry and the per-session tree that stands in for it. WT is
// empty when the entry is not a git checkout and DESK_ROOTS keeps its shared path.
type sessionRoot struct {
	Name, Src, WT string
}

// wantsSessionRoots: a house cell's verify-desk window, unless the cell opted out.
func (c *Cell) wantsSessionRoots(role string) bool {
	return c.Kind == "house" && role == verifyRootsRole && c.Env.Get(verifyRootsOptOut) != "1"
}

// sessionRootPath is where an entry's per-session tree lives: one directory per root, named
// from `<owner>/<repo>` with the slash spelled `--`.
func (c *Cell) sessionRootPath(name string) string {
	return filepath.Join(c.Dir, "worktrees", verifyRootsDir, strings.ReplaceAll(name, "/", "--"))
}

// sessionRootsPlan maps CELL_ROOTS to the per-session trees without touching disk — what a
// dry run prints.
func (c *Cell) sessionRootsPlan(roots string) string {
	var out []string
	for _, e := range rootEntries(roots) {
		out = append(out, e[0]+"="+c.sessionRootPath(e[0]))
	}
	return strings.Join(out, ",")
}

// joinSessionRoots renders DESK_ROOTS from the prepared entries: the per-session tree where one
// exists, else the shared path.
func joinSessionRoots(rs []sessionRoot) string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		p := r.Src
		if r.WT != "" {
			p = r.WT
		}
		out = append(out, r.Name+"="+p)
	}
	return strings.Join(out, ",")
}

// prepareSessionRoots cuts or refreshes every entry's tree and returns the DESK_ROOTS value the
// window gets. A root that cannot be brought to origin/main stops the boot: a verify window on a
// stale root is the refusal this exists to remove, so it is never launched silently.
func (c *Cell) prepareSessionRoots(roots string) string {
	var rs []sessionRoot
	for _, e := range rootEntries(roots) {
		r := sessionRoot{Name: e[0], Src: e[1]}
		if !isRootCheckout(r.Src) {
			fmt.Fprintf(os.Stderr, "[roots] %s: %s is not a git checkout — DESK_ROOTS keeps the shared path\n", r.Name, r.Src)
			rs = append(rs, r)
			continue
		}
		r.WT = c.sessionRootPath(r.Name)
		sha, err := refreshSessionRoot(r)
		if err != nil {
			die("desk: verify root %s: %v. Fix it, or set %s=1 in cell.env to read the shared checkouts", r.Name, err, verifyRootsOptOut)
		}
		// Locked so a prune never takes it from under the window; a re-lock on reboot refuses
		// quietly, which is the same state.
		_ = exec.Command("git", "-C", r.Src, "worktree", "lock", "--reason", "cellctl "+c.Name+"/"+verifyRootsRole+" session root", r.WT).Run()
		fmt.Printf("[roots] %s: session tree %s @ %s (origin/main, locked)\n", r.Name, r.WT, short8(sha))
		rs = append(rs, r)
	}
	c.sessionRoots = rs
	return joinSessionRoots(rs)
}

// refreshSessionRootsPass moves every per-session tree to the root's freshly fetched
// origin/main. It runs before each cadence pass; the first failure is returned.
func (c *Cell) refreshSessionRootsPass() error {
	for _, r := range c.sessionRoots {
		if r.WT == "" {
			continue
		}
		if _, err := refreshSessionRoot(r); err != nil {
			return fmt.Errorf("verify root %s: %w", r.Name, err)
		}
	}
	return nil
}

// isRootCheckout: the path is the top of a git work tree — not merely a directory somewhere
// inside one, whose fetch would land in an unrelated repository.
func isRootCheckout(p string) bool {
	top, err := gitcore.Toplevel(p)
	return err == nil && samePath(top, p)
}

// refreshSessionRoot fetches the root's main into its remote-tracking ref (an explicit refspec,
// so a narrowed fetch configuration cannot leave the ref behind), then creates the detached tree
// at it or moves the existing one. An existing tree is moved only when that loses nothing: a
// tree with tracked changes, or a HEAD that is not an ancestor of origin/main, is refused.
func refreshSessionRoot(r sessionRoot) (string, error) {
	fetch := exec.Command("git", "-C", r.Src, "fetch", "--no-tags", "--quiet", "origin", "+refs/heads/main:refs/remotes/origin/main")
	fetch.Stderr = os.Stderr
	fetch.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if err := fetch.Run(); err != nil {
		return "", fmt.Errorf("fetch of origin main in %s failed: %v", r.Src, err)
	}
	sha, err := remoteMainCommit(r.Src)
	if err != nil || sha == "" {
		return "", fmt.Errorf("%s has no refs/remotes/origin/main after the fetch", r.Src)
	}
	if _, err := os.Stat(r.WT); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(r.WT), 0o755); err != nil {
			return "", err
		}
		add := exec.Command("git", "-C", r.Src, "worktree", "add", "--quiet", "--detach", r.WT, sha)
		if out, err := add.CombinedOutput(); err != nil {
			return "", fmt.Errorf("could not create %s: %v: %s", r.WT, err, strings.TrimSpace(string(out)))
		}
		return sha, nil
	}
	if err := sameRepository(r.Src, r.WT); err != nil {
		return "", err
	}
	wt, err := gitcore.Open(r.WT)
	if err != nil {
		return "", fmt.Errorf("cannot open %s: %v", r.WT, err)
	}
	if st, err := wt.DirtyTrackedPorcelain(); err != nil || st != "" {
		return "", fmt.Errorf("%s has uncommitted changes (or its status is unreadable) — left as is", r.WT)
	}
	head, err := wt.HeadID()
	if err != nil {
		return "", fmt.Errorf("cannot read HEAD of %s: %v", r.WT, err)
	}
	if head == sha {
		return sha, nil
	}
	if ok, err := wt.IsAncestor(head, sha); err != nil || !ok {
		return "", fmt.Errorf("%s is at %s, which origin/main does not contain — left as is", r.WT, short8(head))
	}
	if out, err := exec.Command("git", "-C", r.WT, "checkout", "--quiet", "--detach", sha).CombinedOutput(); err != nil {
		return "", fmt.Errorf("could not move %s to %s: %v: %s", r.WT, short8(sha), err, strings.TrimSpace(string(out)))
	}
	return sha, nil
}

// sameRepository refuses a path at the tree's location that is not a worktree of the root's
// own repository — a stray directory is never adopted or moved.
func sameRepository(src, wt string) error {
	a, err1 := gitcore.CommonDir(src)
	b, err2 := gitcore.CommonDir(wt)
	top, err3 := gitcore.Toplevel(wt)
	if err1 != nil || err2 != nil || err3 != nil || a == "" || a != b || !samePath(top, wt) {
		return fmt.Errorf("%s exists and is not a worktree of %s — move it aside", wt, src)
	}
	return nil
}

// remoteMainCommit is `git rev-parse --verify -q refs/remotes/origin/main^{commit}`, read
// in-process: the full ref name, so a local branch of that name never shadows it.
func remoteMainCommit(dir string) (string, error) {
	repo, err := gitcore.Open(dir)
	if err != nil {
		return "", err
	}
	const ref = "refs/remotes/origin/main"
	if ok, _ := repo.CommitVerifyQuiet(ref); !ok {
		return "", fmt.Errorf("%s does not name a commit", ref)
	}
	h, err := repo.Resolve(ref)
	if err != nil {
		return "", err
	}
	return h.String(), nil
}

func samePath(a, b string) bool {
	ea, err1 := filepath.EvalSymlinks(a)
	eb, err2 := filepath.EvalSymlinks(b)
	return err1 == nil && err2 == nil && ea == eb
}
