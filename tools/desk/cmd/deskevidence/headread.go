package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/gitcore"
)

// landingHead reads the same HEAD commit the decision-gate base and branch
// comparison previously obtained through separate rev-parse processes. It is a
// fresh read per operation, not a source/authority cache. The existing gitcore
// capability probes select the original native operation before any object read;
// an unreadable repository or object is never retried through a second reader.
func landingHead(root string) (string, error) {
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		// rev-parse neither invokes a pager nor initializes a repository, so
		// these settings cannot change this read's discovery/object/output.
		presentationOnly := strings.EqualFold(key, "GIT_PAGER") || strings.EqualFold(key, "GIT_TEMPLATE_DIR")
		if strings.HasPrefix(strings.ToUpper(key), "GIT_") && !presentationOnly {
			return nativeLandingHead(root)
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	top, err := gitcore.Toplevel(abs)
	if err != nil {
		return "", err
	}
	why, err := gitcore.NativeFilesystemReadReason(abs, top)
	if err != nil {
		return "", err
	}
	if why != "" {
		return nativeLandingHead(root)
	}
	format, err := gitcore.ObjectFormat(top)
	if err != nil {
		return "", err
	}
	if format != gitcore.BuildObjectFormat() {
		return nativeLandingHead(root)
	}
	why, err = gitcore.NativeRepositoryReadReason(top)
	if err != nil {
		return "", err
	}
	if why != "" {
		return nativeLandingHead(root)
	}
	r, err := gitcore.Open(top)
	if err != nil {
		return "", err
	}
	if r.NativeObjectReadReason() != "" {
		return nativeLandingHead(root)
	}
	for _, probe := range []func() (string, error){r.NativeDiscoveryReason, r.NativeHistoryReadReason, r.NativeReplacementReadReason} {
		why, err = probe()
		if err != nil {
			return "", err
		}
		if why != "" {
			return nativeLandingHead(root)
		}
	}
	why, err = r.CommitTreeReadReason("HEAD")
	if err != nil {
		return "", err
	}
	if why != "" {
		return nativeLandingHead(root)
	}
	head, err := r.HeadID()
	if err != nil {
		return "", err
	}
	if _, err := r.RawCommit(head); err != nil {
		return "", err
	}
	return head, nil
}

func nativeLandingHead(root string) (string, error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	cmd.Dir = os.TempDir()
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
