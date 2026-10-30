package gitcore

import (
	"fmt"
	"io"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/revlist"
)

// WriteHistoryPack exports only objects reachable from the exact current source.
// It copies no repository config, refs, hooks, alternates or credentials and
// starts no native process. The caller owns the writer and publication policy.
func WriteHistoryPack(home, source string, out io.Writer) error {
	if !isFullHex(source) {
		return fmt.Errorf("gitcore: history export requires a full source commit")
	}
	r, err := Open(home)
	if err != nil {
		return err
	}
	if closer, ok := r.repo.Storer.(io.Closer); ok {
		defer closer.Close()
	}
	shallow, err := r.repo.Storer.Shallow()
	if err != nil || len(shallow) != 0 {
		return fmt.Errorf("gitcore: history export requires complete, non-shallow history")
	}
	head, err := r.repo.Head()
	if err != nil || head.Hash().String() != source {
		return fmt.Errorf("gitcore: history export HEAD is not the admitted source")
	}
	objects, err := revlist.Objects(r.repo.Storer, []plumbing.Hash{head.Hash()}, nil)
	if err != nil {
		return fmt.Errorf("gitcore: incomplete source history: %w", err)
	}
	_, err = packfile.NewEncoder(out, r.repo.Storer, false).Encode(objects, 10)
	return err
}

// ReadHistoryPack imports object data into a new disposable repository. Its tip
// is supplied only by the already admitted source, never a ref in the pack.
// A full reachability walk must succeed before HEAD or the baseline anchor is
// installed. Native Git, network, config and credential resolution are unused.
func ReadHistoryPack(dest, source string, in io.Reader) error {
	if !isFullHex(source) {
		return fmt.Errorf("gitcore: history import requires a full source commit")
	}
	r, err := git.PlainInit(dest, false)
	if err != nil {
		return err
	}
	if closer, ok := r.Storer.(io.Closer); ok {
		defer closer.Close()
	}
	if err := packfile.UpdateObjectStorage(r.Storer, in); err != nil {
		return fmt.Errorf("gitcore: invalid history pack: %w", err)
	}
	hash := plumbing.NewHash(source)
	if _, err := r.CommitObject(hash); err != nil {
		return fmt.Errorf("gitcore: history pack does not contain the admitted source: %w", err)
	}
	if _, err := revlist.Objects(r.Storer, []plumbing.Hash{hash}, nil); err != nil {
		return fmt.Errorf("gitcore: incomplete source history pack: %w", err)
	}
	for _, ref := range []*plumbing.Reference{
		plumbing.NewHashReference("refs/heads/verification-source", hash),
		plumbing.NewHashReference("refs/remotes/origin/main", hash),
		plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/verification-source"),
	} {
		if err := r.Storer.SetReference(ref); err != nil {
			return err
		}
	}
	return nil
}
