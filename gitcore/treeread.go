package gitcore

import (
	"fmt"
	"io"

	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// TreeEntry preserves all entry modes, including symlinks and gitlinks.
type TreeEntry struct{ Path, Mode, Hash string }

// TreeEntries lists a revision's entries in Git tree order. Complete-tree
// validation uses the same owner as the existing status preflight, because the
// upstream TreeWalker otherwise silently truncates on a missing subtree.
func (r *Repo) TreeEntries(rev string, recursive, includeTrees bool) ([]TreeEntry, error) {
	tree, err := r.treeAt(rev)
	if err != nil {
		return nil, err
	}
	if err := r.treeComplete(tree); err != nil {
		return nil, err
	}
	walker := object.NewTreeWalker(tree, recursive, nil)
	// This API is inspection only: returned names never become filesystem or
	// archive destinations. Use the upstream inspection mode so native-valid
	// control-byte names remain visible; materialization owners keep validation.
	walker.SkipPathValidation()
	defer walker.Close()
	var entries []TreeEntry
	for {
		name, e, err := walker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if e.Mode == filemode.Dir && !includeTrees {
			continue
		}
		entries = append(entries, TreeEntry{Path: name, Mode: fmt.Sprintf("%06o", e.Mode), Hash: e.Hash.String()})
	}
	return entries, nil
}
