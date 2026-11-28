package gitcore

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
)

// OpenGoGit opens dir the way Open does and returns the go-git repository itself.
//
// It is for a module whose readers are written against go-git's own API (qualgen) and so
// cannot use Repo's helpers, but must still open the repository the way Open does:
// worktreeConfig tolerated, every other unsupported extension still refused, linked
// worktrees and alternates resolved. go-git's raw opener (git.PlainOpen*) refuses any
// repository carrying extensions.worktreeConfig (see extensionTolerantStorer), so a
// module that opens with it reads an ordinary checkout as unreadable.
//
// Unlike Open, it also opens a bare repository given its own path (dir has no .git
// entry but is itself a git directory), as git.PlainOpen does, so a caller moving off
// the raw opener keeps reading bare repositories. The bare path uses the same storage,
// so the same extension handling applies there. The bare-layout test is localtransport.go's
// isBareGitDir, the one the local transport already serves bare origins by.
func OpenGoGit(dir string) (*git.Repository, error) {
	if _, err := os.Lstat(filepath.Join(dir, ".git")); os.IsNotExist(err) && isBareGitDir(dir) {
		st, _ := storageAt(dir, NewObjectCache())
		r, err := git.Open(st, nil)
		if err != nil {
			return nil, fmt.Errorf("gitcore: open %s: %w", dir, err)
		}
		return r, nil
	}
	r, err := Open(dir)
	if err != nil {
		return nil, err
	}
	return r.repo, nil
}
