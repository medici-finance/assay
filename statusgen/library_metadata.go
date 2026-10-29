package main

import (
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

// gitLastPathTimeOutput preserves the existing native framing, including an
// empty successful result for no history. Read failure never starts a fallback.
func gitLastPathTimeOutput(root, rel string) ([]byte, error) {
	native := func() ([]byte, error) {
		return exec.Command("git", "-C", root, "log", "-1", "--format=%ct", "--", rel).Output()
	}
	read := ordinaryGitCore(root)
	if read.err != nil {
		return nil, read.err
	}
	p, literal := literalMetadataPath(read, rel)
	if read.native != "" || !literal {
		return native()
	}
	row, err := read.repo.LastPathCommit("HEAD", p)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	if row.NativeReason != "" {
		return native()
	}
	return []byte(fmt.Sprintf("%d\n", row.CommitterTime.Unix())), nil
}

// staleFailReferenceOutput re-observes refs on each query. Native config bytes
// remain native; only the reference observation moves into the shared owner.
func staleFailReferenceOutput(root string, native func(...string) ([]byte, error)) ([]byte, error) {
	read := ordinaryGitCore(root)
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

// Only abbreviation formatting remains native: core.abbrev and repository-wide
// collision rules are not replaced by slicing a full object name.
func gitLastPathHeartbeatOutput(root string) ([]byte, error) {
	native := func() ([]byte, error) {
		return exec.Command("git", "-C", root, "log", "-1", "--format=%h %cI", "--", "STATUS.md").Output()
	}
	read := ordinaryGitCore(root)
	if read.err != nil {
		return nil, read.err
	}
	if read.native != "" {
		return native()
	}
	row, err := read.repo.LastPathCommit("HEAD", read.prefix+"STATUS.md")
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	if row.NativeReason != "" {
		return native()
	}
	short, err := exec.Command("git", "-C", root, "rev-parse", "--short", row.ID).Output()
	if err != nil {
		return nil, err
	}
	return []byte(strings.TrimSpace(string(short)) + " " + row.CommitterTime.Format("2006-01-02T15:04:05-07:00") + "\n"), nil
}
