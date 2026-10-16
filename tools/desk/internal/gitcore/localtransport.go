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
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	gitconfig "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/storer"
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
	s, err := server.NewClient(server.MapLoader{ep.String(): st}).NewUploadPackSession(ep, auth)
	if err != nil {
		return nil, err
	}
	return knownHavesSession{UploadPackSession: s, objects: st}, nil
}

// knownHavesSession drops, before the embedded server sees them, the client's "have" lines
// naming an object the served repository does not hold. The client offers every local tip —
// an unpushed commit included, the ordinary state of a working checkout — and go-git's
// embedded server walks the haves and fails the whole fetch ("object not found") on the first
// one it lacks, where git's upload-pack ignores a have it cannot resolve. Dropping an unknown
// have only means the pack may carry objects the client already holds; it never omits one the
// client needs, because the server still subtracts only what it can prove the client has.
type knownHavesSession struct {
	transport.UploadPackSession
	objects storer.EncodedObjectStorer
}

func (s knownHavesSession) UploadPack(ctx context.Context, req *packp.UploadPackRequest) (*packp.UploadPackResponse, error) {
	if req != nil && len(req.Haves) > 0 {
		kept := make([]plumbing.Hash, 0, len(req.Haves))
		for _, h := range req.Haves {
			if s.objects.HasEncodedObject(h) == nil {
				kept = append(kept, h)
			}
		}
		filtered := *req
		filtered.Haves = kept
		req = &filtered
	}
	return s.UploadPackSession.UploadPack(ctx, req)
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

// CheckedOutBranches returns the full ref names (refs/heads/...) of every branch in use by any
// worktree of the repository containing dir — the main checkout (unless the repository is
// bare) and every linked worktree registered under the common directory — sorted. It is the
// set git itself refuses to fetch into (`refusing to fetch into branch ... checked out at
// ...`; git's branch.c prepare_checked_out_branches), per worktree:
//
//   - the branch its HEAD names (a detached HEAD names none);
//   - the branch it is in the middle of rebasing (rebase-merge/head-name, or
//     rebase-apply/head-name when that directory is a rebase rather than `git am`) — HEAD is
//     detached for the whole rebase, and moving the branch under it breaks the rebase's finish;
//   - the branch it started bisecting from (BISECT_START, while BISECT_LOG exists) — HEAD is
//     detached on a midpoint, and `git bisect reset` returns to that branch;
//   - every branch an in-progress `rebase --update-refs` will rewrite (rebase-merge/update-refs).
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

// checkedOutBranches reads, from gitDir's common directory, the branches every worktree has
// in use (see CheckedOutBranches): from <common> for the main worktree when core.bare is not
// true, and from <common>/worktrees/<name> for each linked worktree. Any read error other than
// an absent file or directory is returned — a caller that cannot tell must not guess "none".
func checkedOutBranches(gitDir string) (map[plumbing.ReferenceName]bool, error) {
	common, err := commonDirOf(gitDir)
	if err != nil {
		return nil, fmt.Errorf("gitcore: read commondir of %s: %w", gitDir, err)
	}
	if common == "" {
		common = gitDir
	}
	set := map[plumbing.ReferenceName]bool{}
	bare, err := isBareConfig(filepath.Join(common, "config"))
	if err != nil {
		return nil, err
	}
	if !bare {
		if err := worktreeBranches(common, set); err != nil {
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
		admin := filepath.Join(common, "worktrees", e.Name())
		if _, serr := os.Stat(filepath.Join(admin, "HEAD")); os.IsNotExist(serr) {
			continue // a half-removed registration has no HEAD; git lists no branch for it either
		}
		if err := worktreeBranches(admin, set); err != nil {
			return nil, err
		}
	}
	return set, nil
}

// worktreeBranches adds to set the branches one worktree has in use, read from its admin
// directory (the common directory for the main worktree, worktrees/<name> for a linked one):
// HEAD's branch, the branch being rebased, the branch bisected from, and the branches an
// in-progress `rebase --update-refs` will rewrite. The file names and their shapes are git's
// (wt-status.c wt_status_check_rebase / wt_status_check_bisect, sequencer.c update-refs).
func worktreeBranches(admin string, set map[plumbing.ReferenceName]bool) error {
	head, err := readAdminFile(admin, "HEAD")
	if err != nil {
		return err
	}
	if target, ok := strings.CutPrefix(head, "ref:"); ok {
		set[plumbing.ReferenceName(strings.TrimSpace(target))] = true
	}

	// A rebase in progress. rebase-apply is also `git am`'s directory; only a rebase holds a
	// branch (git: rebase-apply/applying marks am).
	headName := ""
	switch {
	case isDir(filepath.Join(admin, "rebase-apply")):
		if !exists(filepath.Join(admin, "rebase-apply", "applying")) {
			if headName, err = readAdminFile(admin, filepath.Join("rebase-apply", "head-name")); err != nil {
				return err
			}
		}
	case isDir(filepath.Join(admin, "rebase-merge")):
		if headName, err = readAdminFile(admin, filepath.Join("rebase-merge", "head-name")); err != nil {
			return err
		}
	}
	if strings.HasPrefix(headName, "refs/heads/") { // "detached HEAD" for a detached rebase
		set[plumbing.ReferenceName(headName)] = true
	}

	// A bisect in progress: BISECT_START names the branch it started from (short form), or
	// the commit when it started detached.
	if exists(filepath.Join(admin, "BISECT_LOG")) {
		from, err := readAdminFile(admin, "BISECT_START")
		if err != nil {
			return err
		}
		switch {
		case from == "" || isHexObjectID(from):
		case strings.HasPrefix(from, "refs/heads/"):
			set[plumbing.ReferenceName(from)] = true
		case !strings.HasPrefix(from, "refs/"):
			set[plumbing.NewBranchReferenceName(from)] = true
		}
	}

	// rebase --update-refs: the file is (ref, old, new) line triples; every ref line counts.
	updateRefs, err := readAdminFile(admin, filepath.Join("rebase-merge", "update-refs"))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(updateRefs, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "refs/") {
			set[plumbing.ReferenceName(line)] = true
		}
	}
	return nil
}

// readAdminFile returns the trimmed content of rel under admin, or "" when it does not exist.
// Any other read error is returned.
func readAdminFile(admin, rel string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(admin, rel))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("gitcore: read %s: %w", filepath.Join(admin, rel), err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// isHexObjectID reports whether s is a full SHA-1 or SHA-256 object id.
func isHexObjectID(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
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
