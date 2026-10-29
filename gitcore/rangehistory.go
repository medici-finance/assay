package gitcore

import (
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing"
	"os"
	"path/filepath"
	"strings"
)

// RangeCommit is an immutable history fact, without log formatting or identity
// rewriting. Parents retain object order; Message comes from the commit object.
type RangeCommit struct {
	ID                  string
	Parents             []string
	Message             string
	NativeMessageReason string
}

// CommitRange returns commits reachable from tip and not reachable from base.
// Endpoints must be full object IDs. It reads only go-git storage and never starts
// a process. The result is a set, not a promise about Git's display order. Every
// traversed parent must be readable, including excluded ancestry: a storage gap
// is uncertainty, never a confidently smaller set. Close the Repo at run end.
func (r *Repo) CommitRange(base, tip string) ([]RangeCommit, error) {
	if !plumbing.IsHash(base) || !plumbing.IsHash(tip) {
		return nil, fmt.Errorf("gitcore: range requires full object IDs")
	}
	excluded, err := r.rangeReachable(base)
	if err != nil {
		return nil, err
	}
	included, err := r.rangeReachable(tip)
	if err != nil {
		return nil, err
	}
	var out []RangeCommit
	for _, row := range included {
		if _, omit := excluded[row.ID]; !omit {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *Repo) rangeReachable(tip string) (map[string]RangeCommit, error) {
	seen := map[string]RangeCommit{}
	pending := []string{tip}
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if _, ok := seen[id]; ok {
			continue
		}
		row, err := r.rangeCommit(id)
		if err != nil {
			return nil, fmt.Errorf("gitcore: range ancestry %s: %w", id, err)
		}
		seen[id] = row
		pending = append(pending, row.Parents...)
	}
	return seen, nil
}

func (r *Repo) rangeCommit(id string) (RangeCommit, error) {
	r.graphMu.Lock()
	defer r.graphMu.Unlock()
	if row, ok := r.graphCommits[id]; ok {
		row.Parents = append([]string(nil), row.Parents...)
		return row, nil
	}
	c, err := r.exactCommit(id)
	if err != nil {
		return RangeCommit{}, err
	}
	row := RangeCommit{ID: id, Message: c.Message}
	raw, err := r.RawCommit(id)
	if err != nil {
		return RangeCommit{}, err
	}
	header, _, _ := strings.Cut(string(raw), "\n\n")
	for _, line := range strings.Split(header, "\n") {
		if strings.HasPrefix(line, "encoding ") {
			row.NativeMessageReason = "declared message encoding requires native log rendering"
		}
	}

	for _, parent := range c.ParentHashes {
		row.Parents = append(row.Parents, parent.String())
	}
	if r.graphCommits == nil {
		r.graphCommits = map[string]RangeCommit{}
	}
	r.graphCommits[id] = row
	row.Parents = append([]string(nil), row.Parents...)
	return row, nil
}

// NativeAbbreviationReason detects reference names that can shadow a displayed
// hexadecimal abbreviation. It only classifies the native name profile; it does
// not resolve an alias, nor select a retry after an object read failed.
func (r *Repo) NativeAbbreviationReason(token string) (string, error) {
	if len(token) < 4 || len(token) > len(plumbing.ZeroHash.String()) {
		return "outside hexadecimal object abbreviation scope", nil
	}
	for _, c := range token {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return "outside hexadecimal object abbreviation scope", nil
		}
	}
	gitDir, _, err := resolveGitDir(r.dir)
	if err != nil {
		return "", err
	}
	if _, err := os.Lstat(filepath.Join(gitDir, token)); err == nil {
		return "native pseudo-ref shadows object abbreviation", nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	for _, name := range []string{"refs/" + token, "refs/tags/" + token, "refs/heads/" + token, "refs/remotes/" + token, "refs/remotes/" + token + "/HEAD"} {
		_, err := r.repo.Reference(plumbing.ReferenceName(name), false)
		if err == nil {
			return "native revision name shadows object abbreviation", nil
		}
		if !errors.Is(err, plumbing.ErrReferenceNotFound) {
			return "", err
		}
	}
	return "", nil
}
