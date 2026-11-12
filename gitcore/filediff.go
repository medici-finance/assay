package gitcore

import (
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/filemode"
	fdiff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// FileDiff returns a three-context-line content patch for one regular file in two
// commit trees. Missing objects, non-files and mode changes return an error.
// Other changed files are not read.
func (r *Repo) FileDiff(from, to, path string) (string, error) {
	oldTree, err := r.treeAt(from)
	if err != nil {
		return "", err
	}
	newTree, err := r.treeAt(to)
	if err != nil {
		return "", err
	}
	oldEntry, err := oldTree.FindEntry(path)
	if err != nil {
		return "", err
	}
	newEntry, err := newTree.FindEntry(path)
	if err != nil {
		return "", err
	}
	if oldEntry.Mode != newEntry.Mode || (oldEntry.Mode != filemode.Regular && oldEntry.Mode != filemode.Executable) {
		return "", fmt.Errorf("gitcore: content patch requires an unchanged regular-file mode for %s", path)
	}
	change := object.Change{
		From: object.ChangeEntry{Name: path, Tree: oldTree, TreeEntry: *oldEntry},
		To:   object.ChangeEntry{Name: path, Tree: newTree, TreeEntry: *newEntry},
	}
	patch, err := change.Patch()
	if err != nil {
		return "", err
	}
	var out strings.Builder
	if err := fdiff.NewUnifiedEncoder(&out, 3).Encode(patch); err != nil {
		return "", err
	}
	return out.String(), nil
}
