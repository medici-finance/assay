package gitcore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
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
type rangeNode struct {
	record  RangeCommit
	parents []int
}

func (r *Repo) CommitRange(base, tip string) ([]RangeCommit, error) {
	if !plumbing.IsHash(base) || !plumbing.IsHash(tip) {
		return nil, fmt.Errorf("gitcore: range requires full object IDs")
	}
	// One indexed immutable graph per Repo. Query-local membership is a compact
	// bitmap, not a rebuilt string-keyed map of decoded records and parent slices.
	r.graphMu.Lock()
	defer r.graphMu.Unlock()
	baseIndex, err := r.loadRangeNode(base, map[string]bool{})
	if err != nil {
		return nil, err
	}
	tipIndex, err := r.loadRangeNode(tip, map[string]bool{})
	if err != nil {
		return nil, err
	}
	excluded := make([]bool, len(r.graphNodes))
	pending := []int{baseIndex}
	for len(pending) > 0 {
		index := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if excluded[index] {
			continue
		}
		excluded[index] = true
		pending = append(pending, r.graphNodes[index].parents...)
	}
	visited := make([]bool, len(r.graphNodes))
	pending = append(pending, tipIndex)
	var out []RangeCommit
	for len(pending) > 0 {
		index := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if visited[index] {
			continue
		}
		visited[index] = true
		if !excluded[index] {
			row := r.graphNodes[index].record
			row.Parents = append([]string(nil), row.Parents...)
			out = append(out, row)
		}
		// Even excluded nodes are already fully validated by loadRangeNode. Their
		// parents are all excluded too, so there is no need to visit that subgraph.
		if !excluded[index] {
			pending = append(pending, r.graphNodes[index].parents...)
		}
	}
	return out, nil
}

// The caller holds graphMu. Publish an index only after every parent was read:
// failed or cyclic ancestry is retryable uncertainty, never a cached small set.
func (r *Repo) loadRangeNode(id string, visiting map[string]bool) (int, error) {
	if index, ok := r.graphIDs[id]; ok {
		return index, nil
	}
	if visiting[id] {
		return 0, fmt.Errorf("gitcore: cyclic range ancestry at %s", id)
	}
	visiting[id] = true
	defer delete(visiting, id)
	row, err := r.readRangeCommit(id)
	if err != nil {
		return 0, fmt.Errorf("gitcore: range commit %s: %w", id, err)
	}
	var parents []int
	for _, parent := range row.Parents {
		index, err := r.loadRangeNode(parent, visiting)
		if err != nil {
			return 0, err
		}
		parents = append(parents, index)
	}
	index := len(r.graphNodes)
	r.graphNodes = append(r.graphNodes, rangeNode{record: row, parents: parents})
	if r.graphIDs == nil {
		r.graphIDs = map[string]int{}
	}
	r.graphIDs[id] = index
	return index, nil
}

func (r *Repo) readRangeCommit(id string) (RangeCommit, error) {
	c, err := r.exactCommit(id)
	if err != nil {
		return RangeCommit{}, err
	}
	row := RangeCommit{ID: id, Message: c.Message}
	if strings.ContainsRune(c.Message, 0) {
		row.NativeMessageReason = "binary message requires native log rendering"
	}
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
	common, err := CommonDir(r.dir)
	if err != nil {
		return "", err
	}
	for _, dir := range []string{gitDir, common} {
		if _, err := os.Lstat(filepath.Join(dir, token)); err == nil {
			return "native pseudo-ref shadows object abbreviation", nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
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
