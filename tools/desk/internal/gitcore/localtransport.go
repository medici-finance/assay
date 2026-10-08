package gitcore

// localtransport.go — serve a local-path or file:// remote's FETCH side in-process.
//
// go-git's stock "file" transport is not in-process: it starts the git binary's
// git-upload-pack (and, when that is not on PATH, `git --exec-path`) as a child that inherits
// this process's WHOLE environment. That child honours git's environment-supplied
// configuration, so a caller who controls the environment of a desk verb could steer what it
// does — up to naming a program it runs — on any fetch or listing whose remote is a local
// path. The git-binary seams this package replaced built that child from an allowlisted
// environment; the stock file transport builds it from none.
//
// This file closes that for the upload-pack side (Fetch, List, FetchTree — every read-side
// transport verb) at the one place gitcore resolves a scheme: at package init it REPLACES
// go-git's "file" protocol with localTransport, whose upload-pack session is served from the
// local repository's own storage through go-git's embedded server — no child process, no
// environment, no PATH lookup. Every other scheme go-git ships (http/https, ssh, git) is a
// pure-Go client. TestFileFetchStartsNoChild observes the process (PATH stand-ins), not a
// tool seam, and TestFileProtocolIsInProcess pins the table entry.
//
// The RECEIVE side (Push into a local path) is deliberately unchanged: it still goes to
// go-git's stock file client, i.e. a git-receive-pack child. That child is what enforces the
// target's own hooks, its compare-and-swap of the old value and its refusal to move a
// checked-out branch — guarantees the embedded server does not provide — so replacing it is a
// design decision of its own, tracked separately, not a by-product of migrating fetch.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	gitconfig "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/plumbing/transport/file"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
)

func init() {
	client.InstallProtocol("file", localTransport{})
}

// localTransport is the in-process "file" protocol. It implements transport.Transport.
type localTransport struct{}

// NewUploadPackSession serves a fetch or a listing from the local repository at ep.Path.
func (localTransport) NewUploadPackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.UploadPackSession, error) {
	gitDir, err := localGitDir(ep.Path)
	if err != nil {
		return nil, err
	}
	st, _ := storageAt(gitDir, NewObjectCache())
	return server.NewClient(server.MapLoader{ep.String(): st}).NewUploadPackSession(ep, auth)
}

// NewReceivePackSession is go-git's stock file client, unchanged (see the file header).
func (localTransport) NewReceivePackSession(ep *transport.Endpoint, auth transport.AuthMethod) (transport.ReceivePackSession, error) {
	return file.DefaultClient.NewReceivePackSession(ep, auth)
}

// localGitDir resolves a local remote path to the repository admin directory to serve, the
// way git-upload-pack's own lookup does for the shapes desk origins take: a checkout (its
// .git directory, or a linked worktree's gitdir pointer) or a bare repository, at the path
// as given or with ".git" appended.
func localGitDir(p string) (string, error) {
	p = adjustLocalPathForWindows(p)
	for _, cand := range []string{p, p + ".git"} {
		if _, err := os.Stat(filepath.Join(cand, ".git")); err == nil {
			gitDir, _, rerr := resolveGitDir(cand)
			if rerr != nil {
				return "", rerr
			}
			return gitDir, nil
		}
		if isBareGitDir(cand) {
			return cand, nil
		}
	}
	return "", transport.ErrRepositoryNotFound
}

// isBareGitDir reports whether dir has a repository's own layout: a HEAD file and an objects
// directory (a linked worktree's admin dir also counts — its objects resolve through
// commondir, which storageAt follows).
func isBareGitDir(dir string) bool {
	head, err := os.Stat(filepath.Join(dir, "HEAD"))
	if err != nil || head.IsDir() {
		return false
	}
	if obj, err := os.Stat(filepath.Join(dir, "objects")); err == nil && obj.IsDir() {
		return true
	}
	_, err = os.Stat(filepath.Join(dir, "commondir"))
	return err == nil
}

// adjustLocalPathForWindows drops the leading slash a file: URL leaves before a drive letter,
// exactly as go-git's own file transport does.
func adjustLocalPathForWindows(p string) string {
	if runtime.GOOS != "windows" {
		return p
	}
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		return p[1:]
	}
	return p
}

// CheckedOutBranches returns the full ref names (refs/heads/...) of every branch checked out
// in any worktree of the repository containing dir — the main checkout (unless the repository
// is bare) and every linked worktree registered under the common directory — sorted. It is
// the set git itself refuses to fetch or push into (`refusing to fetch into branch ...
// checked out at ...`). A detached HEAD contributes nothing.
func CheckedOutBranches(dir string) ([]string, error) {
	top, err := Toplevel(dir)
	if err != nil {
		return nil, err
	}
	gitDir, _, err := resolveGitDir(top)
	if err != nil {
		return nil, fmt.Errorf("gitcore: checked-out branches %s: %w", dir, err)
	}
	set, err := checkedOutBranches(gitDir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name.String())
	}
	sort.Strings(out)
	return out, nil
}

// checkedOutBranches reads, from gitDir's common directory, the branch every worktree has
// checked out: <common>/HEAD for the main worktree when core.bare is not true, and
// <common>/worktrees/<name>/HEAD for each linked worktree. Any read error other than an
// absent worktrees directory is returned — a caller that cannot tell must not guess "none".
func checkedOutBranches(gitDir string) (map[plumbing.ReferenceName]bool, error) {
	common, err := commonDirOf(gitDir)
	if err != nil {
		return nil, fmt.Errorf("gitcore: read commondir of %s: %w", gitDir, err)
	}
	if common == "" {
		common = gitDir
	}
	set := map[plumbing.ReferenceName]bool{}
	add := func(headFile string) error {
		raw, rerr := os.ReadFile(headFile)
		if rerr != nil {
			return fmt.Errorf("gitcore: read %s: %w", headFile, rerr)
		}
		line := strings.TrimSpace(string(raw))
		if target, ok := strings.CutPrefix(line, "ref:"); ok {
			set[plumbing.ReferenceName(strings.TrimSpace(target))] = true
		}
		return nil
	}
	bare, err := isBareConfig(filepath.Join(common, "config"))
	if err != nil {
		return nil, err
	}
	if !bare {
		if err := add(filepath.Join(common, "HEAD")); err != nil {
			return nil, err
		}
	}
	ents, err := os.ReadDir(filepath.Join(common, "worktrees"))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("gitcore: list worktrees of %s: %w", common, err)
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		head := filepath.Join(common, "worktrees", e.Name(), "HEAD")
		if _, serr := os.Stat(head); os.IsNotExist(serr) {
			continue // a half-removed registration has no HEAD; git lists no branch for it either
		}
		if err := add(head); err != nil {
			return nil, err
		}
	}
	return set, nil
}

// isBareConfig reports whether the config file at path sets core.bare = true. A missing file
// reads as not bare (so the main HEAD is counted — the conservative side for a refusal).
func isBareConfig(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("gitcore: read %s: %w", path, err)
	}
	defer f.Close()
	raw := gitconfig.New()
	if err := gitconfig.NewDecoder(f).Decode(raw); err != nil {
		return false, fmt.Errorf("gitcore: parse %s: %w", path, err)
	}
	return strings.EqualFold(strings.TrimSpace(raw.Section("core").Option("bare")), "true"), nil
}
