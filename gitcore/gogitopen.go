package gitcore

import "github.com/go-git/go-git/v5"

// OpenGoGit opens dir exactly as Open does and returns the go-git repository itself.
//
// It is for a module whose readers are written against go-git's own API (qualgen) and so
// cannot use Repo's helpers, but must still open the repository the way Open does:
// worktreeConfig tolerated, every other unsupported extension still refused, linked
// worktrees and alternates resolved. go-git's raw opener (git.PlainOpen*) refuses any
// repository carrying extensions.worktreeConfig (see extensionTolerantStorer), so a
// module that opens with it reads an ordinary checkout as unreadable.
func OpenGoGit(dir string) (*git.Repository, error) {
	r, err := Open(dir)
	if err != nil {
		return nil, err
	}
	return r.repo, nil
}
