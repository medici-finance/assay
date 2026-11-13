package main

import (
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
// checked out, read from `git worktree list --porcelain`, or "" when none does.
func worktreeHoldingBranch(porcelain, branch string) string {
	want := "branch refs/heads/" + branch
	cur := ""
	for _, line := range strings.Split(porcelain, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = strings.TrimPrefix(line, "worktree ")
		case line == "":
			cur = ""
		case line == want && cur != "":
			return cur
		}
	}
	return ""
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
	held := worktreeHoldingBranch(listing, req.Branch)
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
