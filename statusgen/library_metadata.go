package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/gitcore"
)

func literalMetadataPath(read gitCoreRead, rel string) (string, bool) {
	for _, key := range []string{"GIT_LITERAL_PATHSPECS", "GIT_GLOB_PATHSPECS", "GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS"} {
		if _, ok := os.LookupEnv(key); ok {
			return "", false
		}
	}
	if rel == "" || filepath.IsAbs(rel) || strings.ContainsAny(rel, "\\:*?[") {
		return "", false
	}
	p := path.Clean(read.prefix + filepath.ToSlash(rel))
	if p == ".." || strings.HasPrefix(p, "../") {
		return "", false
	}
	return p, true
}

// nativeLogOutputKeys select the bytes a native log prints for an already
// chosen commit (signature output, re-encoded messages). nativeLastPathKeys add
// log.follow, which changes which commit a single-path log selects. Every
// library replacement of a native log consults one of these lists.
var (
	nativeLogOutputKeys = []string{"log.showsignature", "i18n.logoutputencoding", "i18n.commitencoding"}
	nativeLastPathKeys  = append([]string{"log.follow"}, nativeLogOutputKeys...)
)

// logOutputConfigured reads a native config listing for any output key.
func logOutputConfigured(config []byte) bool {
	lower := strings.ToLower(string(config))
	for _, key := range nativeLogOutputKeys {
		if strings.Contains(lower, key) {
			return true
		}
	}
	return false
}

// lastPathConfigNative reports whether effective Git configuration (every
// scope and include, as Git itself resolves it) sets a key that changes a
// single-path log. Presence alone routes native; values are never re-parsed.
// A definite answer is kept per root for the run; a failed observation is
// native and not kept.
func lastPathConfigNative(root string) bool {
	if _, set := os.LookupEnv("GIT_CONFIG"); set {
		return true // honoured by git config only, so the observation would differ from log
	}
	s := activeGitReads
	if s != nil {
		s.coreMu.Lock()
		native, ok := s.logConfig[root]
		s.coreMu.Unlock()
		if ok {
			return native
		}
	}
	pattern := "^(" + strings.ReplaceAll(strings.Join(nativeLastPathKeys, "|"), ".", "\\.") + ")$"
	err := exec.Command("git", "-C", root, "config", "--get-regexp", pattern).Run()
	var exit *exec.ExitError
	native := true
	switch {
	case err == nil:
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		native = false
	default:
		return true
	}
	if s != nil {
		s.coreMu.Lock()
		if s.logConfig == nil {
			s.logConfig = map[string]bool{}
		}
		s.logConfig[root] = native
		s.coreMu.Unlock()
	}
	return native
}

// lastPathCommit is the one library owner of a single-path "log -1". A nil
// row with native false is a definite empty history.
func lastPathCommit(root, rel string) (row *gitcore.PathMetadata, native bool, err error) {
	read := ordinaryGitCore(root)
	if read.err != nil {
		return nil, false, read.err
	}
	p, literal := literalMetadataPath(read, rel)
	if read.native != "" || !literal {
		return nil, true, nil
	}
	if lastPathConfigNative(root) {
		return nil, true, nil
	}
	row, err = read.repo.LastPathCommit("HEAD", p)
	if err != nil {
		return nil, false, err
	}
	if row != nil && row.NativeReason != "" {
		return nil, true, nil
	}
	return row, false, nil
}

// gitLastPathTimeOutput preserves the existing native framing, including an
// empty successful result for no history. Read failure never starts a fallback.
func gitLastPathTimeOutput(root, rel string) ([]byte, error) {
	row, native, err := lastPathCommit(root, rel)
	if err != nil {
		return nil, err
	}
	if native {
		return exec.Command("git", "-C", root, "log", "-1", "--format=%ct", "--", rel).Output()
	}
	if row == nil {
		return nil, nil
	}
	return []byte(fmt.Sprintf("%d\n", row.CommitterTime.Unix())), nil
}

// staleFailReferenceOutput re-observes refs on each query. Native config bytes
// remain native; only the reference observation moves into the shared owner.
func staleFailReferenceOutput(root string, native func(...string) ([]byte, error)) ([]byte, error) {
	read := referenceGitCore(root)
	if read.err != nil {
		return nil, read.err
	}
	if read.native != "" {
		return native("for-each-ref", "--format=%(refname)%00%(objectname)%00%(symref)")
	}
	return referenceSnapshotOutput(read.repo)
}
func referenceSnapshotOutput(repo *gitcore.Repo) ([]byte, error) {
	rows, err := repo.ReferenceSnapshot()
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&b, "%s%c%s%c%s\n", row.Name, 0, row.ObjectID, 0, row.SymbolicTarget)
	}
	return []byte(b.String()), nil
}

// Only display formatting of the selected commit remains native: core.abbrev
// and repository-wide collision rules are not replaced by slicing a full object
// name, and the strict ISO date suffix for a zero offset depends on the Git
// version ("Z" from 2.45, "+00:00" before), so no Go layout reproduces it.
func gitLastPathHeartbeatOutput(root string) ([]byte, error) {
	row, native, err := lastPathCommit(root, "STATUS.md")
	if err != nil {
		return nil, err
	}
	selector := []string{"--", "STATUS.md"}
	if !native {
		if row == nil {
			return nil, nil
		}
		selector = []string{row.ID, "--"}
	}
	return exec.Command("git", append([]string{"-C", root, "log", "-1", "--format=%h %cI"}, selector...)...).Output()
}
