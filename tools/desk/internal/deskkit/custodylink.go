package deskkit

// custodylink.go — how a token-custody path is stat'd before its mode is checked and its
// bytes are read as a credential (follow-up to #1573).
//
// os.Stat follows a symlink, so a custody check built on it validated whatever a link AT the
// custody path pointed to — a 0600 regular file anywhere passed — and the read that followed
// presented that file's bytes as the role's credential. LstatCustody looks at the custody path
// itself first:
//
//   - CustodyNoLinks: any symlink is refused. The GitHub App token cache is written by
//     desktoken alone and has no link layout, so a link there was planted.
//   - CustodySameDirLink: one link is allowed — to a file in the SAME directory as the custody
//     path. That is the documented GitLab layout (docs/adopting-assay-gitlab.md §2 links
//     gitlab-<role>.token at <prefix>-<role>-bot.token beside it). A same-directory target is
//     a file the custody directory's owner already controls; a link that resolves anywhere
//     else, or does not resolve, is refused.
//
// A refusal is a *CustodyLinkError, never a not-exist error: a caller that treats "missing"
// as "go and write a fresh one" must not write THROUGH a dangling link.

import (
	"fmt"
	"os"
	"path/filepath"
)

// CustodyLinkPolicy says which symlinks, if any, a custody path may be.
type CustodyLinkPolicy int

const (
	// CustodyNoLinks refuses any symlink at the custody path.
	CustodyNoLinks CustodyLinkPolicy = iota
	// CustodySameDirLink accepts a link whose fully resolved target sits in the custody
	// path's own (resolved) directory, and refuses every other link.
	CustodySameDirLink
)

// CustodyLinkError is the refusal LstatCustody returns for a symlinked custody path.
type CustodyLinkError struct {
	Path   string
	Reason string
}

func (e *CustodyLinkError) Error() string {
	return fmt.Sprintf("custody path %s is a symlink %s; token custody must be a 0600 regular file "+
		"— re-provision it", e.Path, e.Reason)
}

// LstatCustody returns the FileInfo a custody check should judge for path WITHOUT blindly
// following a link there. A regular (non-link) entry returns its own Lstat. A link is refused
// with a *CustodyLinkError unless policy is CustodySameDirLink and the link resolves to a file
// in the custody path's own directory, in which case the RESOLVED target's FileInfo is returned
// so the caller's regular-file and owner-only checks run on the file actually read.
//
// Any other error is the raw Lstat error, so os.IsNotExist keeps meaning "nothing is there".
func LstatCustody(path string, policy CustodyLinkPolicy) (os.FileInfo, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return fi, nil
	}
	if policy != CustodySameDirLink {
		return nil, &CustodyLinkError{Path: path, Reason: "(this custody file has no link layout; replace the link with the file itself)"}
	}
	resolved, rerr := filepath.EvalSymlinks(path)
	if rerr != nil {
		return nil, &CustodyLinkError{Path: path, Reason: fmt.Sprintf("that does not resolve (%v)", rerr)}
	}
	dir, derr := filepath.EvalSymlinks(filepath.Dir(path))
	if derr != nil {
		return nil, &CustodyLinkError{Path: path, Reason: fmt.Sprintf("whose directory does not resolve (%v)", derr)}
	}
	if filepath.Dir(resolved) != dir {
		return nil, &CustodyLinkError{Path: path, Reason: fmt.Sprintf(
			"to %s, outside its own directory %s (only a link to a file in the same directory is accepted)",
			resolved, dir)}
	}
	return os.Lstat(resolved)
}
