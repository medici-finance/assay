package main

// Sweep-local read reuse. No state survives an interval tick, no ref name is cached
// as an immutable fact, and none of these inventories grants removal authority.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

type ancestryQuery struct{ ancestor, descendant string }

var sweepIsAncestor = func(repo *gitcore.Repo, ancestor, descendant string) (bool, error) {
	return repo.IsAncestorCached(ancestor, descendant)
}

var sweepRefsContaining = func(repo *gitcore.Repo, head string) ([]string, error) {
	return repo.RefsContaining(head, "refs/remotes/")
}

// isAncestor resolves the candidate's refs afresh, then reuses immutable hash-bound
// answers. Mainline queries use the ordinary sweep's existing ancestor set. Other
// queries share gitcore's reachability index on ONE Repo for the common object store,
// rather than constructing an index for every candidate worktree.
func (sc *sweepCtx) isAncestor(rt, ancestor, descendant string) (bool, error) {
	repo, err := sc.open(rt)
	if err != nil {
		return false, err
	}
	a, err := repo.Resolve(ancestor)
	if err != nil {
		return false, err
	}
	d, err := repo.Resolve(descendant)
	if err != nil {
		return false, err
	}
	query := ancestryQuery{a.String(), d.String()}
	if answer, ok := sc.ancestry[query]; ok {
		return answer, nil
	}
	var answer bool
	if query.descendant == sc.mainTip {
		if sc.mainAncestors == nil {
			return false, deskkit.Unverifiable("the sweep's mainline history could not be read", sc.ancestorErr)
		}
		// A missing/non-commit HEAD must remain uncertainty, even when membership
		// would otherwise return false without opening that object.
		if valid, err := repo.CommitVerifyQuiet(query.ancestor); err != nil || !valid {
			return false, deskkit.Unverifiable("cannot read the ancestry query's HEAD commit", err)
		}
		_, answer = sc.mainAncestors[query.ancestor]
	} else {
		history, err := sc.open(sc.historyDir)
		if err != nil {
			return false, err
		}
		answer, err = sweepIsAncestor(history, query.ancestor, query.descendant)
		if err != nil {
			return false, err
		}
	}
	sc.ancestry[query] = answer
	return answer, nil
}

type worktreeAdminIndex struct {
	paths     map[string]string
	ambiguous map[string]bool
	err       error
}

// These are the actual inventory operations, counted by the scale regressions.
var readAdminRegistry = os.ReadDir
var readAdminPointer = os.ReadFile

func loadWorktreeAdminIndex(commonDir string) *worktreeAdminIndex {
	index := &worktreeAdminIndex{paths: map[string]string{}, ambiguous: map[string]bool{}}
	if commonDir == "" {
		index.err = fmt.Errorf("the shared git-common-dir is not resolved")
		return index
	}
	entries, err := readAdminRegistry(filepath.Join(commonDir, "worktrees"))
	if err != nil {
		index.err = err
		return index
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		admin := filepath.Join(commonDir, "worktrees", entry.Name())
		raw, err := readAdminPointer(filepath.Join(admin, "gitdir"))
		pointer := strings.TrimSpace(string(raw))
		if err != nil || !filepath.IsAbs(pointer) || filepath.Base(pointer) != ".git" {
			// An unreadable pointer could hide a second registration for ANY
			// path. A partial inventory therefore cannot certify unique identity.
			if err == nil {
				err = fmt.Errorf("gitdir must name an absolute .git path")
			}
			index.err = fmt.Errorf("cannot inventory worktree admin pointer %s: %w", admin, err)
			continue
		}
		path := resolvePath(filepath.Dir(pointer))
		if _, exists := index.paths[path]; exists {
			index.ambiguous[path] = true
		}
		index.paths[path] = admin
	}
	return index
}

func (index *worktreeAdminIndex) lookup(rt string) (string, error) {
	if index.err != nil {
		return "", deskkit.Unverifiable("cannot read the complete worktree admin registry", index.err)
	}
	if index.ambiguous[rt] {
		return "", deskkit.Unverifiable("multiple admin entries point at "+rt, nil)
	}
	admin, ok := index.paths[rt]
	if !ok {
		return "", deskkit.Unverifiable("no admin entry points at "+rt, nil)
	}
	// Reuse discovery, never a mutable pointer's old value. A moved/removed
	// registration is held rather than applying another worktree's age or HEAD.
	raw, err := readAdminPointer(filepath.Join(admin, "gitdir"))
	pointer := strings.TrimSpace(string(raw))
	if err != nil || !filepath.IsAbs(pointer) || filepath.Base(pointer) != ".git" || resolvePath(filepath.Dir(pointer)) != rt {
		return "", deskkit.Unverifiable("worktree admin identity changed after inventory", err)
	}
	return admin, nil
}

func (sc *sweepCtx) adminDirFor(guard *pathGuard, rt string) (string, error) {
	if guard.commonDir != sc.commonDir {
		return "", deskkit.Unverifiable("admin registry belongs to a different common Git store", nil)
	}
	if sc.admins == nil {
		sc.admins = loadWorktreeAdminIndex(sc.commonDir)
	}
	return sc.admins.lookup(rt)
}

func (sc *sweepCtx) adminEntryAge(guard *pathGuard, rt string, now time.Time) (time.Duration, error) {
	admin, err := sc.adminDirFor(guard, rt)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(admin)
	if err != nil {
		return 0, deskkit.Unverifiable("cannot stat the worktree's admin entry", err)
	}
	return now.Sub(info.ModTime()), nil
}
