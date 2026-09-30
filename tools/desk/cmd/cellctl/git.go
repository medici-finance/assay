package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (c *Cell) git(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// gitlabCredArgs are the `-c credential.helper=…` flags that answer git's HTTPS auth for
// CELL_REPO's own fetch transport on a gitlab cell. The FIRST `credential.helper=` clears any OS
// keychain helper that would otherwise shadow this one; the inline function reads the
// hand-provisioned deskd read token from its FILE and never a token in the URL, and is never
// persisted into the operator's git config. Shared verbatim between the boot fetch and the
// check-time ls-remote probe so the two never drift apart.
func (c *Cell) gitlabCredArgs() []string {
	return []string{
		"-c", "credential.helper=",
		"-c", `credential.helper=!f(){ echo username=oauth2; printf "password=%s\n" "$(cat "$DESKD_GITLAB_TOKEN_FILE")"; }; f`,
	}
}

// gitlabFetchReachable is the check-time probe for the gitlab arm's fetch transport, proving a
// broken/missing token is caught at `cellctl check` rather than as a boot hang at an interactive
// Username prompt. `ls-remote` never mutates CELL_REPO.
func (c *Cell) gitlabFetchReachable() bool {
	args := append(c.gitlabCredArgs(), "-C", c.Repo, "ls-remote", "--exit-code", "origin", "main")
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0",
		"DESKD_GITLAB_TOKEN_FILE="+c.Env.Get("DESKD_GITLAB_TOKEN_FILE"))
	return cmd.Run() == nil
}

// fetchMainUnderLock serialises the shared fetch: several windows boot at once against ONE .git,
// and concurrent fetches race on the same ref lock. The lock lives in the COMMON git dir, so a
// CELL_REPO that is itself a linked worktree (where .git is a file) still gets a lock rather
// than a 60s wait. A fetch that lost the ref-lock race still wrote FETCH_HEAD, and FETCH_HEAD is
// all that is used — so a non-zero exit is a notice, not a stop.
//
// FETCH_HEAD is removed before the fetch, so the one read after it is always THIS fetch's. git
// truncates FETCH_HEAD itself before it contacts the remote, so a fetch that fails on auth or
// connect already leaves nothing to read. The removal is for the case git's own truncate cannot
// cover: a FETCH_HEAD git cannot open for writing (read-only file), where the fetch fails and the
// PREVIOUS boot's sha is left in place. And when the file cannot be removed either, the boot is
// refused — reading it would be that previous boot's main.
func (c *Cell) fetchMainUnderLock(role string) string {
	gitdir, err := gitOut(c.Repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil || gitdir == "" {
		gitdir = filepath.Join(c.Repo, ".git")
	}
	lock := filepath.Join(gitdir, "cellctl-fetch.lock")
	held := false
	for i := 0; i < 60; i++ {
		if os.Mkdir(lock, 0o700) == nil {
			held = true
			break
		}
		time.Sleep(time.Second)
	}
	if !held {
		// Pre-existing behaviour, kept: a lock still held after 60s is taken to be a dead boot's.
		// This boot proceeds, and the release below removes that lock. If the holder is in fact
		// alive, removing FETCH_HEAD under it makes ITS read fail — a refusal, never a stale boot.
		fmt.Fprintf(os.Stderr, "NOTICE: fetch lock %s still held after 60s — proceeding without it\n", lock)
	}
	// die() panics and main recovers, so this release runs on the refusals below too — a refused
	// boot must not hand the next one a 60s wait.
	defer os.Remove(lock)
	if fh, err := gitOut(c.Repo, "rev-parse", "--path-format=absolute", "--git-path", "FETCH_HEAD"); err == nil && fh != "" {
		if err := os.Remove(fh); err != nil && !errors.Is(err, fs.ErrNotExist) {
			die("desk: cannot remove %s before the fetch — refusing to boot: a fetch that cannot rewrite it would leave a previous boot's main to read. "+
				"Make it removable (check its permissions and its directory's), then re-run: cellctl desk %s %s", fh, c.Name, role)
		}
	}
	var cmd *exec.Cmd
	if c.Forge == "gitlab" {
		args := append(c.gitlabCredArgs(), "-C", c.Repo, "fetch", "--no-tags", "origin", "main")
		cmd = exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0",
			"DESKD_GITLAB_TOKEN_FILE="+c.Env.Get("DESKD_GITLAB_TOKEN_FILE"))
	} else {
		cmd = exec.Command("git", "-C", c.Repo, "fetch", "--no-tags", "origin", "main")
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "NOTICE: fetch returned non-zero — checking whether it wrote FETCH_HEAD")
	}
	sha, err := gitOut(c.Repo, "rev-parse", "--verify", "-q", "FETCH_HEAD")
	if err != nil || sha == "" {
		die("%s", fetchFailedMsg(c.Repo, c.Name, role))
	}
	return sha
}

// fetchFailedMsg is the refusal when the boot fetch wrote no FETCH_HEAD. It names the usual
// cause and the way out, because git's own output reads like a transient error: a credential
// helper answering with a dead token (an expired App token left in the checkout's shared config,
// say). The inspect command prints helper values verbatim, and an inline helper can carry that
// token — hence the warning not to paste it anywhere public.
func fetchFailedMsg(repo, cell, role string) string {
	return fmt.Sprintf("desk: fetch of origin main in %s failed and wrote no FETCH_HEAD — refusing to boot on a stale main. "+
		"An authentication refusal is the usual cause. See which credential helper answers for origin: "+
		"git -C '%s' config --show-origin --get-regexp '^credential\\.' "+
		"(its output can contain a token — do not paste it into a PR or issue). "+
		"Re-mint or replace the dead credential, or remove the stale helper entry, then re-run: cellctl desk %s %s",
		repo, repo, cell, role)
}

// cellctlGeneratedFiles names the single-writer files main's CI regenerates (the board and the
// findings view). On a boot merge they are taken from origin/main OUTRIGHT — never hand-merged,
// never left conflicted — because a role worktree's copy is at best a local regen main has
// already superseded.
func (c *Cell) cellctlGeneratedFiles() []string {
	return strings.Fields(c.Env.GetOr("CELLCTL_GENERATED_FILES", "STATUS.md docs/streams/FINDINGS.md"))
}

// mergeRoleWorktree brings an EXISTING role worktree up to the fetched origin/main (#1157). A
// real merge — fast-forward when the tree carries nothing of its own, a two-parent merge commit
// when it does — never a rebase, and never `--ff-only`-then-give-up: a role worktree that ever
// carried a local commit can never fast-forward again, so every later boot was stale and silent.
//
// On a conflict the generated single-writer files are taken from main; ANY other conflict STOPS
// the boot — merge aborted, tree left exactly as it was, the paths named, nothing launched.
func (c *Cell) mergeRoleWorktree(wt, sha string) {
	if exec.Command("git", "-C", wt, "merge-base", "--is-ancestor", sha, "HEAD").Run() == nil {
		fmt.Printf("[worktree] %s already contains origin/main %s\n", wt, short8(sha))
		return
	}
	before, _ := gitOut(wt, "rev-parse", "HEAD")
	mergeCmd := exec.Command("git", "-C", wt, "merge", "--no-edit",
		"-m", "merge origin/main "+short8(sha)+" at cellctl desk boot", sha)
	mergeOut, mergeErr := mergeCmd.CombinedOutput()
	if mergeErr == nil {
		if head, _ := gitOut(wt, "rev-parse", "HEAD"); head == sha {
			fmt.Printf("[worktree] fast-forwarded %s to origin/main %s\n", wt, short8(sha))
		} else {
			fmt.Printf("[worktree] merged origin/main %s into %s (two-parent merge; local commits kept)\n", short8(sha), wt)
		}
		return
	}
	conflicts, _ := gitOut(wt, "diff", "--name-only", "--diff-filter=U")
	if strings.TrimSpace(conflicts) == "" {
		// Not a conflict: git refused the merge outright (a dirty tree the merge would
		// overwrite, no commit identity, …). Nothing is half-done, but the tree IS behind main
		// — STOP, quoting git, rather than launching a desk on it.
		_ = exec.Command("git", "-C", wt, "merge", "--abort").Run()
		die("desk: %s is behind origin/main %s and could not merge it — %s— boot STOPPED (nothing launched). Fix that, then re-run; or by hand: git -C %s merge %s (merge, never rebase)",
			wt, short8(sha), lastLines(string(mergeOut), 3), wt, sha)
	}
	var remaining []string
	for _, p := range strings.Split(conflicts, "\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if valueIn(p, c.cellctlGeneratedFiles()) {
			// Main's side: `--theirs` in a merge is the branch being merged in. A path main
			// deleted has no "theirs" to check out — dropping it is main's side too.
			if exec.Command("git", "-C", wt, "checkout", "--theirs", "--", p).Run() == nil {
				_ = exec.Command("git", "-C", wt, "add", "--", p).Run()
			} else {
				_ = exec.Command("git", "-C", wt, "rm", "-q", "--cached", "--", p).Run()
				_ = os.Remove(filepath.Join(wt, p))
			}
			fmt.Printf("[worktree] %s taken from origin/main (generated, single-writer — never hand-merged)\n", p)
			continue
		}
		remaining = append(remaining, p)
	}
	if len(remaining) > 0 {
		if exec.Command("git", "-C", wt, "merge", "--abort").Run() != nil {
			_ = exec.Command("git", "-C", wt, "reset", "-q", "--hard", before).Run()
		}
		die("desk: %s conflicts with origin/main %s on: %s — boot STOPPED (merge aborted, tree left at %s, nothing launched). Resolve by hand: git -C %s merge %s (merge, never rebase; take main's side for generated files), then re-run cellctl desk",
			wt, short8(sha), strings.Join(remaining, " "), short8(before), wt, sha)
	}
	if exec.Command("git", "-C", wt, "commit", "-q", "--no-edit").Run() != nil {
		die("desk: %s could not commit the boot merge of origin/main %s after taking the generated files from main — boot STOPPED (nothing launched). Inspect: git -C %s status", wt, short8(sha), wt)
	}
	fmt.Printf("[worktree] merged origin/main %s into %s (two-parent merge; generated files taken from main)\n", short8(sha), wt)
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " ") + " "
}

// worktreeViaDeskwt lets `deskwt role-init` create the role worktree when the installed
// desk-tools ship one that supports the role, so cellctl and the desk skills agree on the
// worktree's name. cellctl's own worktree path stays the fallback, and CELLCTL_DESKWT=0 forces
// it.
func (c *Cell) worktreeViaDeskwt(role string) (string, bool) {
	if c.Env.GetOr("CELLCTL_DESKWT", "1") != "1" {
		return "", false
	}
	if _, err := exec.LookPath("deskwt"); err != nil {
		return "", false
	}
	if exec.Command("deskwt", "role-init", "--help").Run() != nil {
		return "", false
	}
	cmd := exec.Command("deskwt", "role-init", role)
	cmd.Dir = c.Repo
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		return "", false
	}
	if _, serr := os.Stat(filepath.Join(last, ".git")); serr != nil {
		return "", false
	}
	return last, true
}

// deskdUp is the cell deskd's health probe, bounded so a hung daemon cannot hang the launcher.
func (c *Cell) deskdUp() bool {
	client := &http.Client{
		Timeout:   3 * time.Second,
		Transport: &http.Transport{DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext},
	}
	resp, err := client.Get("http://" + c.DeskdAddr + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}

// rootsState reports "<branch> <upstream> <ahead> <behind> <dirty>" for one stream-root path, or
// ok=false when the root cannot be compared: not a checkout, detached HEAD, or no upstream. It
// NEVER fetches — it compares the local branch against the remote-tracking ref as it already
// stands, which is exactly the drift that presents as a permanently stale board. A stale
// remote-tracking ref means this under-reports, never over-reports.
type rootState struct {
	Branch, Upstream string
	Ahead, Behind    string
	Dirty            string
}

func rootsState(p string) (rootState, bool) {
	var st rootState
	if _, err := os.Lstat(filepath.Join(p, ".git")); err != nil {
		return st, false
	}
	b, err := gitOut(p, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || b == "" {
		return st, false
	}
	up, err := gitOut(p, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	if err != nil || up == "" {
		return st, false
	}
	ahead, err := gitOut(p, "rev-list", "--count", up+"..HEAD")
	if err != nil {
		ahead = "0"
	}
	behind, err := gitOut(p, "rev-list", "--count", "HEAD.."+up)
	if err != nil {
		behind = "0"
	}
	porcelain, _ := gitOut(p, "status", "--porcelain")
	dirty := "0"
	if strings.TrimSpace(porcelain) != "" {
		dirty = fmt.Sprintf("%d", len(strings.Split(strings.TrimRight(porcelain, "\n"), "\n")))
	}
	st = rootState{Branch: b, Upstream: up, Ahead: ahead, Behind: behind, Dirty: dirty}
	return st, true
}

// ffRoots fast-forwards every stream root that can move WITHOUT a decision: on a branch, that
// branch has an upstream, the tree is clean, and it is 0 ahead. Anything else is left alone and
// NAMED — a root the operator is working in is never moved under them.
func (c *Cell) ffRoots() {
	for _, e := range rootEntries(c.Env.Get("CELL_ROOTS")) {
		name, p := e[0], e[1]
		st, ok := rootsState(p)
		if !ok {
			fmt.Fprintf(os.Stderr, "[roots] %s: not a comparable checkout (missing, detached HEAD, or no upstream) — left as is\n", name)
			continue
		}
		// Refresh the remote-tracking ref first; without it "behind" only reflects the last fetch.
		if i := strings.IndexByte(st.Upstream, '/'); i > 0 {
			_ = exec.Command("git", "-C", p, "fetch", "--no-tags", "--quiet", st.Upstream[:i], st.Upstream[i+1:]).Run()
		}
		st, ok = rootsState(p)
		if !ok {
			continue
		}
		if st.Dirty != "0" {
			fmt.Fprintf(os.Stderr, "[roots] %s: %s uncommitted change(s) — left as is\n", name, st.Dirty)
			continue
		}
		if st.Ahead != "0" {
			fmt.Fprintf(os.Stderr, "[roots] %s: %s is %s ahead of %s — left as is (merge it yourself)\n", name, st.Branch, st.Ahead, st.Upstream)
			continue
		}
		if st.Behind == "0" {
			fmt.Printf("[roots] %s: %s already current with %s\n", name, st.Branch, st.Upstream)
			continue
		}
		if exec.Command("git", "-C", p, "merge", "--ff-only", st.Upstream).Run() == nil {
			head, _ := gitOut(p, "rev-parse", "--short", "HEAD")
			fmt.Printf("[roots] %s: %s fast-forwarded %s commit(s) to %s\n", name, st.Branch, st.Behind, head)
		} else {
			fmt.Fprintf(os.Stderr, "[roots] %s: %s did not fast-forward — left as is\n", name, st.Branch)
		}
	}
}
