package cellscratch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// sourceGitCmd discovers the named checkout, never a repository selected by an
// inherited Git environment. It only reads local repository state.
func sourceGitCmd(ctx context.Context, source string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", source}, args...)...)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(entry), "GIT_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	return cmd
}

// sourceWorktree identifies the working tree containing a directory. Git's
// repository discovery rejects metadata directories independently of their name.
func sourceWorktree(source string) (string, error) {
	out, err := sourceGitCmd(context.Background(), source, "rev-parse", "--is-inside-work-tree", "--is-bare-repository").Output()
	if err != nil || string(out) != "true\nfalse\n" {
		return "", errors.New("scratch source must be in a non-bare Git working tree")
	}
	out, err = sourceGitCmd(context.Background(), source, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(strings.TrimSuffix(string(out), "\n"))
}

func sourceRoot(source string) (string, error) {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	source, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	if metadataPath(source) {
		return "", errors.New("source must be outside repository metadata")
	}
	root, err := sourceWorktree(source)
	if err != nil {
		return "", err
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !os.SameFile(sourceInfo, rootInfo) {
		return "", errors.New("scratch source must be the Git working-tree root")
	}
	return source, nil
}

// SourceRevision admits a working-tree root before resolving its immutable
// revision. The same admission is repeated by both materialization entry points.
func SourceRevision(source, revision string) (string, string, error) {
	source, err := sourceRoot(source)
	if err != nil {
		return "", "", err
	}
	out, err := sourceGitCmd(context.Background(), source, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}").Output()
	if err != nil {
		return "", "", err
	}
	return source, strings.TrimSpace(string(out)), nil
}
