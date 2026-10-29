package main

import (
	"fmt"
	"os"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// addTarget is the directory this invocation exclusively reserved. Keeping its root
// open pins its identity even if the path is removed and its inode could be reused.
// Git accepts an empty existing directory; competing deskwt invocations cannot reserve it.
type addTarget struct {
	path string
	root *os.Root
}

func reserveAddTarget(target string) (*addTarget, error) {
	if err := os.Mkdir(target, 0o700); err != nil {
		if os.IsExist(err) {
			return nil, deskkit.Refused("refused: target already exists (never clobbered): " + target)
		}
		return nil, deskkit.Unverifiable("cannot reserve worktree target "+target, err)
	}
	root, err := os.OpenRoot(target)
	if err != nil {
		return nil, deskkit.Unverifiable("cannot retain worktree target identity; inspect "+target, err)
	}
	return &addTarget{path: target, root: root}, nil
}

// rollbackFailedAdd covers every failure after reservation, including git, identity,
// transport and hooks. Ownership never depends on git's localized stderr. Missing
// targets need no deletion; foreign, replaced or symlink targets are left for inspection.
func rollbackFailedAdd(guard *pathGuard, dir, target string, owned *addTarget) string {
	if owned == nil {
		return ""
	}
	fail := func(err error) string {
		return "; rollback of " + target + " stopped (" + err.Error() + "); target retained for inspection"
	}
	if owned.path != target {
		return fail(fmt.Errorf("target was not reserved by this add"))
	}
	fi, err := os.Lstat(target) // unresolved: never follow a substituted leaf symlink
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		return fail(err)
	}
	original, err := owned.root.Stat(".")
	if err != nil {
		return fail(err)
	}
	if !fi.IsDir() || !os.SameFile(original, fi) {
		return fail(fmt.Errorf("reserved directory was replaced"))
	}
	rt, err := guard.check(target) // the sanctioned-prefix/identity gate still binds cleanup
	if err != nil {
		return fail(err)
	}
	current, err := os.Lstat(rt)
	if err != nil || !current.IsDir() || !os.SameFile(original, current) {
		return fail(fmt.Errorf("reserved directory no longer matches the checked target"))
	}
	if err := removeWorktreeDir(guard, dir, rt); err != nil {
		return fail(err)
	}
	return "; worktree rolled back: " + target + " (ROLLED BACK; this add reserved it)"
}
