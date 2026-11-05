package gitcore

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// FileAtBounded reads only a regular blob at an exact caller-selected revision.
// A large blob is reported before decompression; no filesystem path is followed.
func (r *Repo) FileAtBounded(rev, path string, limit int64) (string, bool, error) {
	commit, e := r.exactCommit(rev)
	if e != nil {
		return "", false, e
	}
	tree, e := commit.Tree()
	if e != nil {
		return "", false, e
	}
	entry, e := tree.FindEntry(path)
	if e != nil {
		return "", false, e
	}
	if entry.Mode != filemode.Regular && entry.Mode != filemode.Executable {
		return "", false, fmt.Errorf("not a regular blob")
	}
	size, e := tree.Size(path)
	if e != nil {
		return "", false, e
	}
	if limit < 0 || size > limit {
		return "", true, nil
	}
	f, e := tree.File(path)
	if e != nil {
		return "", false, e
	}
	rd, e := f.Reader()
	if e != nil {
		return "", false, e
	}
	return readBoundedBlob(rd, limit)
}

// readBoundedBlob preserves budget skips and primary read errors. A sole close
// failure cannot certify an otherwise successful local blob read.
func readBoundedBlob(rd io.ReadCloser, limit int64) (content string, oversized bool, resultErr error) {
	defer func() {
		if closeErr := rd.Close(); closeErr != nil && resultErr == nil && !oversized {
			resultErr = fmt.Errorf("gitcore: bounded blob close: %w", closeErr)
		}
	}()
	b, err := io.ReadAll(io.LimitReader(rd, limit+1))
	if int64(len(b)) > limit {
		return "", true, nil
	}
	return string(b), false, err
}

// DiffSymmetricBounded checks changed blob headers before rename detection or
// patch construction can inflate their contents. Over-budget inputs are omitted;
// no unbounded provider fallback is implied by this result.
func (r *Repo) DiffSymmetricBounded(a, b string, contextLines int, budget int64) (string, bool, error) {
	ac, e := r.exactCommit(a)
	if e != nil {
		return "", false, e
	}
	bc, e := r.exactCommit(b)
	if e != nil {
		return "", false, e
	}
	bases, e := ac.MergeBase(bc)
	if e != nil {
		return "", false, e
	}
	if len(bases) == 0 {
		return "", false, fmt.Errorf("no common ancestor")
	}
	base := bases[0]
	from, e := base.Tree()
	if e != nil {
		return "", false, e
	}
	to, e := bc.Tree()
	if e != nil {
		return "", false, e
	}
	changes, e := object.DiffTreeWithOptions(context.Background(), from, to, &object.DiffTreeOptions{DetectRenames: false})
	if e != nil {
		return "", false, e
	}
	seen := map[plumbing.Hash]bool{}
	var total int64
	for _, c := range changes {
		for _, v := range []object.ChangeEntry{c.From, c.To} {
			if v.Tree == nil || v.TreeEntry.Hash.IsZero() || seen[v.TreeEntry.Hash] {
				continue
			}
			if v.TreeEntry.Mode != filemode.Regular && v.TreeEntry.Mode != filemode.Executable {
				return "", true, nil
			}
			seen[v.TreeEntry.Hash] = true
			n, e := r.repo.Storer.EncodedObjectSize(v.TreeEntry.Hash)
			if e != nil {
				return "", false, e
			}
			if n < 0 || n > budget-total {
				return "", true, nil
			}
			total += n
		}
	}
	raw, e := diffTrees(from, to, contextLines, base.Hash.String(), bc.Hash.String())
	return raw, false, e
}

// exactCommit is the immutable object boundary for preparation source reads.
// Revision expressions are deliberately unsupported by these bounded APIs.
func (r *Repo) exactCommit(id string) (*object.Commit, error) {
	if len(id) != 40 || strings.ToLower(id) != id || plumbing.NewHash(id).String() != id {
		return nil, fmt.Errorf("a canonical full object id is required")
	}
	return r.repo.CommitObject(plumbing.NewHash(id))
}
