// Command deskpushguard is a git pre-push hook that refuses a push to a branch
// whose PR is MERGED or CLOSED. It reads the pre-push hook's stdin ref lines, queries
// gh for each branch's PR state, and exits non-zero only on positive MERGED/CLOSED
// confirmation. Fail-OPEN: any ambiguity or error allows the push. This inverts the
// desk-tools fail-closed convention.
//
// It ALSO refuses a push that introduces a new register entry (docs/streams/findings/ or
// docs/streams/intake/) whose `id:` frontmatter collides with an entry already present on
// some OTHER open (not-yet-merged) remote branch — see registerid.go. This is #72's second
// unique failure shape (tracker#566 reused `id: F-39`): two branches, each individually cut
// correctly from origin/main, can independently pick the same register id; the collision is
// invisible on either branch alone (each adds exactly one new, locally-unique-looking entry)
// and today only reds main's CI after the second one merges. A separate guard covers the
// OTHER failure shape (a fake single-parent "merge" commit) but does not reach this one — it
// checks commit ancestry, not cross-branch content collisions, and a branch pushed this
// carries no foreign commit at all.
//
// It does NOT call deskkit.Guard() — the guard must run even when the desk-tools
// kill-switch is armed, because a stopped desk still must not orphan commits.
//
// Hook contract:
//
//	git invokes .githooks/pre-push <remote-name> <remote-url> (core.hooksPath)
//	stdin: <local-ref> <local-sha> <remote-ref> <remote-sha> (one per line)
//
// Environment:
//
//	DESKPUSHGUARD_OFF=1        skip the check (with stderr warning)
//	DESKPUSHGUARD_FAKE_STATE=X  fake the gh response (testing only)
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// execCommand is the seam for testing — production binds exec.Command.
var execCommand = exec.Command

type prInfo struct {
	State  string `json:"state"`
	Number int    `json:"number"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stderr))
}

func run(args []string, stdin io.Reader, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-version") {
		sha, built := deskkit.Version()
		fmt.Printf("deskpushguard sourceSHA=%s builtAt=%s\n", sha, built)
		return deskkit.ExitOK
	}
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Fprintf(stderr, "deskpushguard — git pre-push hook: refuse push to MERGED/CLOSED PR branch\n")
		return deskkit.ExitOK
	}

	// Running from source (unstamped) is a drift risk.
	deskkit.WarnIfUnpinned(stderr)

	// Read remote URL from args. git invokes: pre-push <remote-name> <remote-url>
	remoteURL := ""
	if len(args) >= 2 {
		remoteURL = args[1]
	}

	// Parse stdin refs.
	sc := bufio.NewScanner(stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)

	var refs []refLine
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		ref, ok := parseRef(line)
		if !ok {
			continue
		}
		refs = append(refs, ref)
	}

	if len(refs) == 0 {
		return deskkit.ExitOK
	}

	// DESKPUSHGUARD_OFF escape hatch.
	if os.Getenv("DESKPUSHGUARD_OFF") == "1" {
		for _, ref := range refs {
			fmt.Fprintf(stderr, "deskpushguard: DESKPUSHGUARD_OFF=1 — skipping guard for branch %s\n", ref.branch)
		}
		auditSkip(stderr, refs)
		return deskkit.ExitOK
	}

	// Register-id collision check: mechanical, local-only (no gh/network
	// call — only already-fetched remote-tracking branches are consulted, matching this
	// tool's fail-open-on-ambiguity contract) detection of a new register entry whose `id:`
	// collides with one already claimed by another in-flight branch. Runs regardless of
	// whether the remote/repo below is derivable, since it needs neither.
	var collisions []registerIDCollision
	for _, ref := range refs {
		cs, cerr := checkRegisterIDCollisions("", ref.branch, ref.localSHA)
		if cerr != nil {
			// checkRegisterIDCollisions currently never returns a non-nil error (it fails
			// open internally); kept so a future stricter variant has somewhere to report.
			fmt.Fprintf(stderr, "deskpushguard: cannot check %s for register-id collisions: %v — allowing (fail-open)\n", ref.branch, cerr)
			continue
		}
		collisions = append(collisions, cs...)
	}

	// Derive repo from remote URL. Fail-OPEN: if we can't parse the remote, skip the
	// MERGED/CLOSED PR-state check (it needs gh+repo), but still enforce any register-id
	// collision already found above.
	var blocked []blockedRef
	repo, err := deriveRepo(remoteURL)
	if err != nil {
		fmt.Fprintf(stderr, "deskpushguard: cannot derive repo from remote %q: %v — skipping PR-state check (fail-open)\n", remoteURL, err)
	} else {
		// Check each ref.
		for _, ref := range refs {
			info, err := fetchPR(repo, ref.branch)
			if err != nil {
				// Fail-OPEN: any gh/network error allows the push.
				fmt.Fprintf(stderr, "deskpushguard: cannot query PR for %s: %v — allowing push (fail-open)\n", ref.branch, err)
				continue
			}
			if info == nil {
				// No PR found for this branch — allow.
				continue
			}
			if info.State == "MERGED" || info.State == "CLOSED" {
				blocked = append(blocked, blockedRef{branch: ref.branch, pr: info.Number, state: info.State})
			}
		}
	}

	if len(blocked) > 0 || len(collisions) > 0 {
		for _, b := range blocked {
			msg := fmt.Sprintf("refusing: PR #%d for %s is %s — a merged/closed PR is DONE; open a NEW branch for follow-up.",
				b.pr, b.branch, b.state)
			fmt.Fprintln(stderr, msg)
		}
		for _, c := range collisions {
			fmt.Fprintf(stderr, "refusing: %s: register entry %s claims id %q, already claimed by %s on %s — rename/re-derive the id before pushing.\n",
				c.branch, c.ownPath, c.id, c.sourcePath, c.sourceBranch)
		}
		if len(blocked) > 0 {
			auditBlock(stderr, blocked)
		}
		if len(collisions) > 0 {
			auditRegisterCollision(stderr, collisions)
		}
		return deskkit.ExitRefused
	}

	return deskkit.ExitOK
}

type refLine struct {
	branch   string
	localSHA string
}

type blockedRef struct {
	branch string
	pr     int
	state  string
}

func parseRef(line string) (refLine, bool) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return refLine{}, false
	}
	local := parts[0]
	branch := filepath.Base(local)
	if branch == "." || branch == "/" || branch == "" {
		return refLine{}, false
	}
	localSHA := ""
	if len(parts) >= 2 {
		localSHA = parts[1]
	}
	return refLine{branch: branch, localSHA: localSHA}, true
}

// deriveRepo extracts "owner/repo" from the origin remote URL.
// Handles HTTPS, git@, and ssh:// URLs.
func deriveRepo(remoteURL string) (string, error) {
	// Try to get repo from `git remote get-url <name>` or parse directly.
	// The URL comes from git's hook invocation.

	// If empty, derive from origin remote.
	if remoteURL == "" {
		cmd := execCommand("git", "remote", "get-url", "origin")
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("cannot get origin URL: %w", err)
		}
		remoteURL = strings.TrimSpace(string(out))
	}

	return parseRepo(remoteURL)
}

// parseRepo extracts owner/repo from a GitHub URL.
func parseRepo(url string) (string, error) {
	// git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@") {
		// git@github.com:owner/repo.git
		parts := strings.SplitN(url, ":", 2)
		if len(parts) == 2 {
			path := strings.TrimSuffix(parts[1], ".git")
			return path, nil
		}
	}
	// ssh://git@github.com/owner/repo.git
	if strings.HasPrefix(url, "ssh://") {
		rest := strings.TrimPrefix(url, "ssh://")
		if idx := strings.Index(rest, "@"); idx >= 0 {
			rest = rest[idx+1:]
		}
		if idx := strings.Index(rest, "/"); idx >= 0 {
			path := strings.TrimSuffix(rest[idx+1:], ".git")
			return path, nil
		}
	}
	// https://github.com/owner/repo.git (and variants)
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		rest := url
		if idx := strings.Index(rest, "://"); idx >= 0 {
			rest = rest[idx+3:]
		}
		// Remove host part
		if idx := strings.Index(rest, "/"); idx >= 0 {
			path := strings.TrimSuffix(rest[idx+1:], ".git")
			return path, nil
		}
	}
	return "", fmt.Errorf("cannot parse repo from URL: %s", url)
}

// fetchPR returns the PR info for a branch, or nil if no PR exists.
// Returns an error only on gh/network failures (fail-open).
func fetchPR(repo, branch string) (*prInfo, error) {
	// Testing seam: DESKPUSHGUARD_FAKE_STATE overrides gh invocation.
	if fake := os.Getenv("DESKPUSHGUARD_FAKE_STATE"); fake != "" {
		if fake == "NONE" {
			return nil, nil
		}
		return &prInfo{State: fake, Number: 42}, nil
	}

	cmd := execCommand("gh", "pr", "view", branch, "--repo", repo, "--json", "state,number")
	var stdout, stderrBuf strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	if err != nil {
		// "no pull requests found for branch" → exit code 1, not an error for our purposes.
		out := strings.TrimSpace(stdout.String())
		errOut := strings.TrimSpace(stderrBuf.String())
		if strings.Contains(errOut, "no pull requests found") || strings.Contains(out, "no pull requests found") {
			return nil, nil
		}
		return nil, fmt.Errorf("gh pr view: %w (stderr: %s)", err, errOut)
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return nil, nil
	}

	var info prInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		// Can't parse — fail open.
		return nil, nil
	}
	return &info, nil
}

// auditSkip logs an audit entry recording that the guard was skipped via DESKPUSHGUARD_OFF.
func auditSkip(stderr io.Writer, refs []refLine) {
	// Build a branch list for the audit detail.
	branches := make([]string, len(refs))
	for i, r := range refs {
		branches[i] = r.branch
	}
	detail := fmt.Sprintf("override skip: branches=%s", strings.Join(branches, ","))
	if err := deskkit.Log(deskkit.Entry{
		Tool:   "deskpushguard",
		Verb:   "pre-push",
		Result: deskkit.ResultOK,
		Detail: detail,
	}); err != nil {
		fmt.Fprintf(stderr, "deskpushguard: audit log error: %v\n", err)
	}
}

// auditBlock logs an audit entry recording which branches were blocked.
func auditBlock(stderr io.Writer, blocked []blockedRef) {
	parts := make([]string, len(blocked))
	for i, b := range blocked {
		parts[i] = fmt.Sprintf("%s=#%d/%s", b.branch, b.pr, b.state)
	}
	detail := fmt.Sprintf("refused push to merged/closed PR branch: %s", strings.Join(parts, ", "))
	if err := deskkit.Log(deskkit.Entry{
		Tool:   "deskpushguard",
		Verb:   "pre-push",
		Result: deskkit.ResultRefused,
		Detail: detail,
	}); err != nil {
		fmt.Fprintf(stderr, "deskpushguard: audit log error: %v\n", err)
	}
}

// auditRegisterCollision logs an audit entry recording which branches were blocked for
// introducing a register entry whose id collides with one already claimed by another
// in-flight branch.
func auditRegisterCollision(stderr io.Writer, collisions []registerIDCollision) {
	parts := make([]string, len(collisions))
	for i, c := range collisions {
		parts[i] = fmt.Sprintf("%s: id %q (%s) also claimed by %s on %s", c.branch, c.id, c.ownPath, c.sourcePath, c.sourceBranch)
	}
	detail := fmt.Sprintf("refused push introducing colliding register id: %s", strings.Join(parts, ", "))
	if err := deskkit.Log(deskkit.Entry{
		Tool:   "deskpushguard",
		Verb:   "pre-push",
		Result: deskkit.ResultRefused,
		Detail: detail,
	}); err != nil {
		fmt.Fprintf(stderr, "deskpushguard: audit log error: %v\n", err)
	}
}
