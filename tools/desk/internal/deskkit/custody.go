package deskkit

// custody.go — the file-identity custody check shared by cellctl (the role-context declaration
// and every file it names) and deskdispatch (a role's dispatch-instruction file). Moved from
// cellctl's rolecontext.go unchanged in behaviour (desk-containers/30); neither binary carries a
// copy.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// CustodyStat is os.Stat; a test replaces it to make one path impossible to examine.
var CustodyStat = os.Stat

// cellMemoryRoot is the cell subdirectory role memory lives below; a role session writes it.
const cellMemoryRoot = "memory"

// WritablePlace is one directory a role session writes, held by file identity: what it is
// called in a refusal, and the directory its name leads to once every link is followed.
type WritablePlace struct {
	Label string
	Info  os.FileInfo
}

// CustodyPlace is an extra directory a caller adds to the cell's own places: its label in a
// refusal and its path.
type CustodyPlace struct {
	Label string
	Dir   string
}

// SessionWritablePlaces lists the directories of a cell that a role session writes:
// <cell-dir>/worktrees and <cell-dir>/memory, and the directory every entry of worktrees/
// leads to, then every extra place the caller adds. The launcher links worktrees/<role> to a tree
// elsewhere when the worktree tool makes the tree, and that tree is the session's working tree
// wherever it lives. A name with nothing behind it is passed over. Any other failure is
// returned: a place that cannot be examined cannot be ruled out.
func SessionWritablePlaces(cellDir string, extra ...CustodyPlace) ([]WritablePlace, error) {
	var places []WritablePlace
	add := func(label, dir string) error {
		info, err := CustodyStat(dir)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		places = append(places, WritablePlace{label, info})
		return nil
	}
	worktrees := filepath.Join(cellDir, "worktrees")
	entries, err := os.ReadDir(worktrees)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	// Role worktrees first, so a refusal names the tree and not only the directory above it.
	for _, e := range entries {
		dir := filepath.Join(worktrees, e.Name())
		if err := add("the role worktree "+dir, dir); err != nil {
			return nil, err
		}
	}
	for _, dir := range []string{worktrees, filepath.Join(cellDir, cellMemoryRoot)} {
		if err := add(dir, dir); err != nil {
			return nil, err
		}
	}
	for _, p := range extra {
		if err := add(p.Label, p.Dir); err != nil {
			return nil, err
		}
	}
	return places, nil
}

// SessionWritablePlace names the place a role session writes that path lies in, or "" when it
// lies in none. A file there could be rewritten by one session and decide the next launch, so
// the declaration and every file it names are refused when they resolve into one.
//
// The question is settled by file identity, never by spelling. The path is resolved, then it
// and each directory above it is compared (os.SameFile) with every place. A path spelling
// proves nothing here: a linked role worktree resolves outside the cell, and on a volume that
// ignores letter case WORKTREES/ and worktrees/ are one directory under two names.
func SessionWritablePlace(cellDir, path string, extra ...CustodyPlace) (string, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if real, err = filepath.Abs(real); err != nil {
		return "", err
	}
	places, err := SessionWritablePlaces(cellDir, extra...)
	if err != nil {
		return "", err
	}
	for at := real; ; at = filepath.Dir(at) {
		info, err := CustodyStat(at)
		if err != nil {
			return "", err
		}
		for _, pl := range places {
			if os.SameFile(info, pl.Info) {
				return pl.Label, nil
			}
		}
		if filepath.Dir(at) == at {
			return "", nil
		}
	}
}

// RefuseSessionWritable is SessionWritablePlace as a refusal; path names the file in the error.
// It fails closed: when the file or a place cannot be examined, the file is refused.
func RefuseSessionWritable(cellDir, path string, extra ...CustodyPlace) error {
	place, err := SessionWritablePlace(cellDir, path, extra...)
	if err != nil {
		return fmt.Errorf("%s: cannot tell whether a role session can write it: %v", path, err)
	}
	if place != "" {
		return fmt.Errorf("%s resolves into %s, which a role session can write; keep the declaration and every file it names outside the role worktrees and the memory directories", path, place)
	}
	return nil
}
