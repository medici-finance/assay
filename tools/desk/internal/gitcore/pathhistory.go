package gitcore

// pathhistory.go — one walk of a history that answers, for every commit it passes, "what did
// this path hold here, and what did it hold at each parent?".
//
// WHY ONE WALK. The same question can be put commit by commit through Log, ParentHashes,
// CommitMessage and TreeishID, but each of those resolves its revision afresh, and a
// resolve reads the repository's references every time. Over a history of a few thousand
// commits that is tens of thousands of reference reads for an answer the commit iterator
// already holds in memory. PathHistory decodes each commit once and each distinct tree once.
//
// IT IS A READ. Nothing is written, no remote is contacted, no process is started.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// PathCommit is one commit as PathHistory hands it to its visitor.
type PathCommit struct {
	// Hash is the commit's full object id.
	Hash string
	// Parents are the full ids of its parents, in the commit's own order.
	Parents []string
	// Message is the full commit message, CRLF normalised to LF as CommitMessage does.
	Message string
	// PathID is the object id the path has in this commit's tree — a blob id for a file, a
	// tree id for a directory — or "" when the path is absent there.
	PathID string
	// ParentPathIDs is the same for each parent, aligned with Parents.
	ParentPathIDs []string
}

// PathHistory walks the commits reachable from rev, newest first by committer time (the
// order `git log` prints), and calls visit for each. visit returns false to stop the walk.
//
// A commit, tree or parent that cannot be read — a shallow clone's boundary, a missing
// object — ends the walk with an error rather than being reported as "the path is absent":
// a caller comparing PathID against ParentPathIDs must never read a gap as a change.
func (r *Repo) PathHistory(rev, path string, visit func(PathCommit) bool) error {
	hash, err := r.Resolve(rev)
	if err != nil {
		return err
	}
	iter, err := r.repo.Log(&git.LogOptions{From: hash, Order: git.LogOrderCommitterTime})
	if err != nil {
		return fmt.Errorf("gitcore: path history %s: %w", rev, err)
	}
	defer iter.Close()

	// Keyed by TREE id: commits that share a tree share the answer.
	seen := map[plumbing.Hash]string{}
	at := func(c *object.Commit) (string, error) {
		if id, ok := seen[c.TreeHash]; ok {
			return id, nil
		}
		tree, err := c.Tree()
		if err != nil {
			return "", fmt.Errorf("gitcore: path history: tree of %s: %w", c.Hash, err)
		}
		id := ""
		entry, err := tree.FindEntry(path)
		switch {
		case err == nil:
			id = entry.Hash.String()
		case errors.Is(err, object.ErrEntryNotFound), errors.Is(err, object.ErrDirectoryNotFound):
			// Absent at this commit.
		default:
			return "", fmt.Errorf("gitcore: path history: %s at %s: %w", path, c.Hash, err)
		}
		seen[c.TreeHash] = id
		return id, nil
	}

	err = iter.ForEach(func(c *object.Commit) error {
		pc := PathCommit{
			Hash:          c.Hash.String(),
			Parents:       make([]string, len(c.ParentHashes)),
			Message:       strings.ReplaceAll(c.Message, "\r\n", "\n"),
			ParentPathIDs: make([]string, len(c.ParentHashes)),
		}
		var err error
		if pc.PathID, err = at(c); err != nil {
			return err
		}
		for i, ph := range c.ParentHashes {
			pc.Parents[i] = ph.String()
			parent, err := r.repo.CommitObject(ph)
			if err != nil {
				return fmt.Errorf("gitcore: path history: parent %s of %s: %w", ph, c.Hash, err)
			}
			if pc.ParentPathIDs[i], err = at(parent); err != nil {
				return err
			}
		}
		if !visit(pc) {
			return storer.ErrStop
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("gitcore: path history %s: %w", rev, err)
	}
	return nil
}
