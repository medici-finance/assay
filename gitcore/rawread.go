package gitcore

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
)

// RawCommit returns the immutable commit object's original bytes. It deliberately
// bypasses object.Commit's signature decoder: provenance callers must judge the
// original headers, including duplicate authors and declared encodings. Replace
// refs, grafts, mailmaps and repository-locating environment are never consulted.
// Only a full canonical object id is accepted; revision selection stays with the
// caller. A failed read is uncertainty, never an invitation to another reader.
func (r *Repo) RawCommit(id string) ([]byte, error) {
	if len(id) != len(plumbing.ZeroHash.String()) || strings.ToLower(id) != id || plumbing.NewHash(id).String() != id {
		return nil, fmt.Errorf("gitcore: raw commit requires a canonical full object id")
	}
	obj, err := r.repo.Storer.EncodedObject(plumbing.CommitObject, plumbing.NewHash(id))
	if err != nil {
		return nil, fmt.Errorf("gitcore: raw commit %s: %w", id, err)
	}
	rd, err := obj.Reader()
	if err != nil {
		return nil, fmt.Errorf("gitcore: raw commit %s: %w", id, err)
	}
	data, readErr := io.ReadAll(rd)
	closeErr := rd.Close()
	if readErr != nil {
		return nil, fmt.Errorf("gitcore: raw commit %s: %w", id, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("gitcore: raw commit %s close: %w", id, closeErr)
	}
	return data, nil
}

// NativeObjectReadReason identifies an observed alternate-path syntax the pinned
// reader cannot represent. It is a pre-read capability check, never an error
// fallback: missing/unreadable objects still return their original uncertainty.
func (r *Repo) NativeObjectReadReason() string { return r.alternates.nativeReason }

// CommitTreeReadReason observes the object kind for an exact selector before
// the commit-only tree reader resolves it. It never expands short references or
// parses revision expressions. A successful non-commit observation permits the
// caller's original generic treeish renderer; an object/ref error stays an error.
func (r *Repo) CommitTreeReadReason(selector string) (string, error) {
	var hash plumbing.Hash
	if len(selector) == len(plumbing.ZeroHash.String()) && plumbing.NewHash(selector).String() == strings.ToLower(selector) {
		hash = plumbing.NewHash(selector)
	} else if selector == "HEAD" || strings.HasPrefix(selector, "refs/") {
		ref, err := r.repo.Reference(plumbing.ReferenceName(selector), true)
		if err != nil {
			return "", err
		}
		hash = ref.Hash()
	} else {
		return "", fmt.Errorf("gitcore: object kind requires an exact selector")
	}
	obj, err := r.repo.Storer.EncodedObject(plumbing.AnyObject, hash)
	if err != nil {
		return "", err
	}
	if obj.Type() != plumbing.CommitObject {
		return "selected object is not a commit; preserve native generic treeish rendering", nil
	}
	return "", nil
}

// NativeHistoryReadReason observes shallow metadata without parsing or
// normalizing it. The selected Shallow decoder loses empty-file and malformed
// line distinctions that native Git preserves. Any positively present entry,
// including a symlink, selects the caller's original native profile. Absence
// is checked afresh per operation; stat errors remain uncertainty.
func (r *Repo) NativeHistoryReadReason() (string, error) {
	common, err := CommonDir(r.dir)
	if err != nil {
		return "", err
	}
	_, err = os.Lstat(filepath.Join(common, "shallow"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return "shallow metadata is present; preserve native boundary grammar and history policy", nil
}
