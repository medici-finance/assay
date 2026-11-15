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
	for _, key := range headReadEnv {
		// Presence, even an empty value, is native Git's own selection.
		if _, present := os.LookupEnv(key); present {
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

// headReadEnv names the Git environment variables native `rev-parse HEAD`
// honors and the library reader does not: repository and object location,
// discovery bounds, injected or relocated configuration, and history rewriting.
// Any one present selects the native read. Everything else, such as GIT_EDITOR,
// GIT_AUTHOR_NAME, GIT_PAGER or GIT_TRACE, cannot change which commit this read
// returns, so it stays on the library path. The same explicit-list approach is
// statusgen's (gitCoreFor, historyRepoEnv).
var headReadEnv = []string{
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_COMMON_DIR",
	"GIT_INDEX_FILE",
	"GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_QUARANTINE_PATH",
	"GIT_NAMESPACE",
	"GIT_CEILING_DIRECTORIES",
	"GIT_DISCOVERY_ACROSS_FILESYSTEM",
	"GIT_CONFIG",
	"GIT_CONFIG_COUNT",
	"GIT_CONFIG_PARAMETERS",
	"GIT_CONFIG_GLOBAL",
	"GIT_CONFIG_SYSTEM",
	"GIT_CONFIG_NOSYSTEM",
	"GIT_SHALLOW_FILE",
	"GIT_GRAFT_FILE",
	"GIT_REPLACE_REF_BASE",
	"GIT_NO_REPLACE_OBJECTS",
}

func nativeLandingHead(root string) (string, error) {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--verify", "--quiet", "HEAD^{commit}")
	cmd.Dir = os.TempDir()
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
