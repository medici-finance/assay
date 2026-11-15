package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/medici-finance/assay/gitcore"
	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// scanworktree.go — WHERE a scan worktree is cut, and what a coalesce does when the carrier branch
// is still checked out in one an earlier pass retained.
//
// Placement. A scan worktree used to be cut beside --root. That is outside both of deskwt's
// sanctioned prefixes, so a worktree the lane retained after a failure could not be removed through
// the sanctioned tooling at all (`deskwt remove` refused it, exit 5), and the skill's recovery step
// had no working route. The default is now the location `deskwt add` itself targets on the host, so
// deskwt's own prefix check (deskkit.InSanctionedWorktreePrefix) accepts every scan worktree this
// loop cuts. deskwt's prefix list is NOT widened for this: that list is the control that stops it
// removing an arbitrary directory, and the fix is to place the worktree inside it.
//
// Retained carrier worktree. See retainedCarrierWorktree.

// scanTmpBase is the POSIX parent of the scan worktrees: deskwt's `tracker-*` prefix parent. It is a
// package var ONLY so a test can point it at a temp dir; production keeps the compiled-in value, and
// it is deliberately not wired to an env var or a flag.
var scanTmpBase = deskkit.SanctionedWorktreeTmpBase

// scanGOOS is the host-OS seam. It selects WHICH sanctioned prefix the default sits under, the same
// choice deskwt's own worktreeTarget makes; it never decides whether a path is sanctioned.
var scanGOOS = runtime.GOOS

// scanWorktreeLeaf prefixes every scan worktree's directory name. It carries deskwt's `tracker-`
// leaf prefix, which a worktree directly under the tmp base needs to be sanctioned.
const scanWorktreeLeaf = deskkit.SanctionedWorktreeLeafPrefix + "intake-scan-"

// legacyScanWorktreeLeaf is the leaf a scan worktree carried before it moved under a sanctioned
// prefix. It is recognised ONLY so a coalesce can adopt a worktree an older pass retained; nothing
// is cut under it any more.
const legacyScanWorktreeLeaf = "intake-scan-"

// defaultScanWorktreeBase is the directory scan worktrees are cut under when --worktree-base is not
// given: the prefix `deskwt add` targets on this host.
//
//   - POSIX: the tmp base, so the worktree is `<tmp base>/<scanWorktreeLeaf><pass>`.
//   - Windows: `<repo-root>/.claude/worktrees`, deskwt's portable prefix (the tmp base is not a
//     usable absolute path there). That directory sits inside the MAIN checkout, so when --root IS
//     the main checkout the scan worktree would be nested in the checkout being scanned, which the
//     lane's isolation guard refuses. That case is refused HERE, up front and by name, rather than
//     falling back to a location deskwt cannot remove: run from a linked worktree of the repo, or
//     name a base with --worktree-base.
func defaultScanWorktreeBase(root string) (string, error) {
	if scanGOOS != "windows" {
		// The sanctioned tmp base is a fixed path. It exists on macOS but not on every POSIX host
		// (a stock Linux has no /private), and git would otherwise fail on every pass when it
		// creates the worktree. That case is refused up front, by name, never moved somewhere the
		// sanctioned tooling cannot remove.
		if fi, err := os.Stat(scanTmpBase); err != nil || !fi.IsDir() {
			return "", deskkit.Refused("scanloop: the sanctioned scan worktree base " + scanTmpBase +
				" is not a directory on this host, so no default scan worktree can be cut under it. " +
				"Pass --worktree-base <abs dir>")
		}
		return scanTmpBase, nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", deskkit.Unverifiable("scanloop: cannot resolve --root "+root, err)
	}
	common, err := gitcore.CommonDir(absRoot)
	if err != nil || common == "" {
		return "", deskkit.Unverifiable("scanloop: cannot resolve the git-common-dir of --root "+root+
			" to place the scan worktrees under <repo-root>/.claude/worktrees", err)
	}
	base := filepath.Join(filepath.Dir(common), ".claude", "worktrees")
	// The common dir comes back symlink-resolved, so the nesting test compares against the resolved
	// root too; comparing a resolved base with an unresolved root would miss the nesting.
	resolvedRoot := absRoot
	if r, rerr := filepath.EvalSymlinks(absRoot); rerr == nil {
		resolvedRoot = r
	}
	if nestedIn(base, absRoot) || nestedIn(base, resolvedRoot) {
		return "", deskkit.Refused("scanloop: --root " + absRoot + " is the main checkout, and the sanctioned scan " +
			"worktree prefix on this host (" + base + ") sits inside it, where the isolation guard refuses a scan " +
			"worktree. Run from a linked worktree of the repo, or pass --worktree-base <abs dir>")
	}
	return base, nil
}

// nestedIn reports whether path is dir itself or sits under it, lexically.
func nestedIn(path, dir string) bool {
	p, d := filepath.Clean(path), filepath.Clean(dir)
	return p == d || strings.HasPrefix(p, d+string(filepath.Separator))
}

// resolvedOrSelf returns p with symlinks evaluated, or p unchanged when it cannot be.
func resolvedOrSelf(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// isScanWorktree reports whether the worktree at held is one this loop cut. A current-leaf name
// qualifies wherever it sits (--worktree-base can put it anywhere). A legacy-leaf name qualifies
// ONLY at the location this loop used to cut it, beside --root: a clean, at-tip worktree that merely
// happens to carry the old leaf somewhere else on the host is not adopted (and so not removed at
// the end of the pass).
func isScanWorktree(held, root string) bool {
	leaf := filepath.Base(held)
	if strings.HasPrefix(leaf, scanWorktreeLeaf) {
		return true
	}
	if !strings.HasPrefix(leaf, legacyScanWorktreeLeaf) {
		return false
	}
	parent := filepath.Dir(filepath.Clean(held))
	oldBase := filepath.Dir(filepath.Clean(root))
	return parent == oldBase || resolvedOrSelf(parent) == resolvedOrSelf(oldBase)
}

// worktreeHoldingBranch returns the path of the registered worktree that has refs/heads/<branch>
// checked out, read from `git worktree list --porcelain`, or "" when none does. prunable reports
// whether git marked that entry `prunable`: its directory is gone and only the registration is left.
// The whole entry is read before it answers, because git prints the `prunable` line after `branch`.
func worktreeHoldingBranch(porcelain, branch string) (path string, prunable bool) {
	want := "branch refs/heads/" + branch
	cur, holds, stale := "", false, false
	for _, line := range strings.Split(porcelain+"\n", "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur, holds, stale = strings.TrimPrefix(line, "worktree "), false, false
		case line == "":
			if holds && cur != "" {
				return cur, stale
			}
			cur, holds, stale = "", false, false
		case line == want:
			holds = true
		case line == "prunable" || strings.HasPrefix(line, "prunable "):
			stale = true
		}
	}
	return "", false
}

// registrationFor returns the registration directory (<git-common-dir>/worktrees/<id>) whose gitdir
// file names the listed worktree path, read from the repository's own administrative files rather
// than from anything inside that path. It refuses (via refuse) unless exactly one registration names
// it and that registration has refs/heads/<branch> checked out.
//
// The leaf of the path is compared as written, never symlink-resolved: resolving it would follow a
// link planted at a vacated path straight to whatever it points at. Only the parent directory is
// resolved, because git may record either spelling of a symlinked tmp root.
func registrationFor(common, held, branch string, refuse func(string) error) (string, error) {
	admin := filepath.Join(common, "worktrees")
	entries, err := os.ReadDir(admin)
	if err != nil {
		return "", deskkit.Unverifiable("scan-carrier lane: cannot read the worktree registrations under "+admin, err)
	}
	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		reg := filepath.Join(admin, e.Name())
		b, err := os.ReadFile(filepath.Join(reg, "gitdir"))
		if err != nil {
			continue // a registration with no gitdir file names no path
		}
		p := strings.TrimSpace(string(b))
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(reg, p)
		}
		p = filepath.Clean(p)
		if filepath.Base(p) == ".git" {
			p = filepath.Dir(p)
		}
		if sameListedPath(p, held) {
			matches = append(matches, reg)
		}
	}
	if len(matches) != 1 {
		return "", refuse(fmt.Sprintf("%d registrations under %s name that path, so it cannot be proved to be the "+
			"registered worktree", len(matches), admin))
	}
	head, err := os.ReadFile(filepath.Join(matches[0], "HEAD"))
	if err != nil {
		return "", deskkit.Unverifiable("scan-carrier lane: cannot read the HEAD of the registration "+matches[0], err)
	}
	if strings.TrimSpace(string(head)) != "ref: refs/heads/"+branch {
		return "", refuse("the registration that names that path (" + matches[0] + ") does not hold " + branch +
			", so the path is not the registered worktree that does")
	}
	return matches[0], nil
}

// sameListedPath reports whether a (from a registration's gitdir file) and b (from the listing) name
// the same worktree path: equal when cleaned, or the same leaf under the same resolved parent.
func sameListedPath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if a == b {
		return true
	}
	return filepath.Base(a) == filepath.Base(b) && resolvedOrSelf(filepath.Dir(a)) == resolvedOrSelf(filepath.Dir(b))
}

// assertRegisteredWorktree proves that the directory now at the listed path held IS the worktree
// git has registered there, before anything runs in it. The retained worktree's directory can
// vanish while its registration stays behind (the tmp base is cleared), and the vacated name in the
// shared tmp base can then be re-created by another local account — as a symlink to another
// worktree of the repo, or as a directory holding its own .git. Status and HEAD read IN that
// directory would look right in both cases, so they prove nothing about which directory it is.
//
// The proof, in this order, so no git command runs in the directory until it is known to be a real
// directory this user owns:
//
//   - the registration naming the path is found from the repository's administrative files;
//   - os.Lstat(held) is a real directory, not a symlink, and on POSIX owned by the invoking user;
//   - `git rev-parse --absolute-git-dir` run in it resolves to that registration's own directory —
//     the "points back" test git's own `worktree remove` applies.
//
// Any mismatch is a refusal, and nothing is changed.
func assertRegisteredWorktree(capture func(dir, name string, args ...string) (string, error), req LaneRequest,
	held string, refuse func(string) error) error {
	common, err := gitcore.CommonDir(req.Root)
	if err != nil || common == "" {
		return deskkit.Unverifiable("scan-carrier lane: cannot resolve the git-common-dir of --root "+req.Root+
			" to find the registration of the retained worktree "+held, err)
	}
	reg, err := registrationFor(common, held, req.Branch, refuse)
	if err != nil {
		return err
	}
	fi, err := os.Lstat(held)
	switch {
	case os.IsNotExist(err):
		return refuse("its directory is missing (git lists the registration prunable), so it is never reused")
	case err != nil:
		return deskkit.Unverifiable("scan-carrier lane: cannot lstat the retained worktree "+held, err)
	case fi.Mode()&os.ModeSymlink != 0:
		return refuse("the path is a symlink, not the registered worktree's own directory")
	case !fi.IsDir():
		return refuse("the path is not a directory, so it is not the registered worktree")
	}
	if err := checkScanWorktreeOwner(fi); err != nil {
		return refuse(err.Error() + ", so it is not the registered worktree this loop cut")
	}
	gitDir, err := capture(held, "git", "rev-parse", "--absolute-git-dir")
	if err != nil {
		return deskkit.Unverifiable("scan-carrier lane: cannot read the git dir of the retained worktree "+held, err)
	}
	got := resolvedOrSelf(filepath.Clean(strings.TrimSpace(gitDir)))
	if want := resolvedOrSelf(reg); got != want {
		return refuse("it does not point back to its registration: git resolves its git dir to " + got +
			", not the registration's own " + want + ", so it is not the registered worktree")
	}
	return nil
}

// retainedCarrierWorktree answers, for a coalesce, whether the carrier branch is already checked out
// in a worktree an earlier pass retained, and if so whether this pass may carry on IN it.
//
// The defect: a pass that fails after its commit keeps its scan worktree with the carrier branch
// checked out. The next coalesce's `git worktree add <new> -B <carrier> origin/<carrier>` then dies
// on "already used by worktree", so every pass in the window exits 6 and lands nothing.
//
// It returns "" when no worktree holds the branch (cut a fresh one, as before), the retained path
// when it is safe to reuse, and a refusal otherwise. Reuse needs ALL of:
//
//   - the worktree is a scan worktree by name (this loop's, not one a person checked out; a legacy
//     name only at the old location beside --root);
//   - it passes the same isolation guard a fresh scan worktree does;
//   - git does not list it `prunable`, and the directory at the path IS the registered worktree
//     (assertRegisteredWorktree), proved before any git command runs in it;
//   - `git status --porcelain` in it is empty — nothing modified, staged or untracked;
//   - its HEAD is exactly the fetched remote carrier tip, the commit `-B` would have reset to.
//
// A worktree that fails any of them is left EXACTLY as it is and the coalesce refuses, as it did
// before: nothing here removes, resets, stashes or forces anything. A read that cannot be taken is
// could-not-check, never "nothing holds the branch".
func retainedCarrierWorktree(capture func(dir, name string, args ...string) (string, error), req LaneRequest) (string, error) {
	listing, err := capture(req.Root, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return "", deskkit.Unverifiable("scan-carrier lane: cannot read `git worktree list` to check whether "+
			req.Branch+" is already checked out", err)
	}
	held, prunable := worktreeHoldingBranch(listing, req.Branch)
	if held == "" {
		return "", nil
	}
	refuse := func(why string) error {
		return deskkit.Refused("scan-carrier lane: " + req.Branch + " is already checked out in the worktree " + held +
			", and " + why + ". It is left exactly as it is: inspect it, then remove it through the project's " +
			"sanctioned worktree tooling (never with --force) and the next pass will coalesce")
	}
	if !isScanWorktree(held, req.Root) {
		return "", refuse("that worktree is not a scan worktree this loop cut, so it is not reused")
	}
	// git lists a worktree by its symlink-resolved path, so the guard runs on the resolved pair as
	// well as the raw one: a lexical miss between the two spellings must never let a worktree nested
	// in the target checkout through.
	if err := assertIsolatedScanRoot(req.Root, held); err != nil {
		return "", refuse("it fails the scan worktree isolation guard (" + err.Error() + ")")
	}
	if err := assertIsolatedScanRoot(resolvedOrSelf(req.Root), resolvedOrSelf(held)); err != nil {
		return "", refuse("it fails the scan worktree isolation guard (" + err.Error() + ")")
	}
	if prunable {
		return "", refuse("its directory is missing (git lists the registration prunable), so it is never reused")
	}
	if err := assertRegisteredWorktree(capture, req, held, refuse); err != nil {
		return "", err
	}
	status, err := capture(held, "git", "status", "--porcelain")
	if err != nil {
		return "", deskkit.Unverifiable("scan-carrier lane: cannot read the status of the retained worktree "+held, err)
	}
	if strings.TrimSpace(status) != "" {
		return "", refuse("it has uncommitted or untracked changes")
	}
	head, err := capture(held, "git", "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", deskkit.Unverifiable("scan-carrier lane: cannot read HEAD of the retained worktree "+held, err)
	}
	tip, err := capture(req.Root, "git", "rev-parse", "--verify", "refs/remotes/origin/"+req.Open.Branch)
	if err != nil {
		return "", deskkit.Unverifiable("scan-carrier lane: cannot read the remote carrier tip origin/"+req.Open.Branch, err)
	}
	head, tip = strings.TrimSpace(head), strings.TrimSpace(tip)
	if head == "" || head != tip {
		return "", refuse("its HEAD " + head + " is not the remote carrier tip " + tip)
	}
	return held, nil
}
