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
// and today only reds main's CI after the second one merges. This check is independent of
// the foreign-commit/merge-masquerade check below — it looks at cross-branch content
// collisions, not commit ancestry, and a branch that trips it carries no foreign commit at
// all.
//
// It ALSO refuses a push carrying a foreign commit (a commit dragged in from a sibling
// branch/PR because the worktree was cut from the wrong base) or a single-parent commit
// masquerading as a merge — see foreigncommit.go. This is the mechanical form of the
// worker-desk skill's manual "Pre-PR self-check" (#22, #72): the 2026-07-30
// recurrence on #22 showed a worker briefed against this in writing do it anyway, so the
// check now runs on every push instead of depending on a worker remembering a manual step.
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
		fmt.Printf("deskpushguard sourceSHA=%s builtAt=%s releaseTag=%s\n", sha, built, deskkit.ReleaseTagOrDev())
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

	// Foreign-commit / merge-masquerade check (#22, #72): mechanical form of
	// the worker-desk skill's manual "Pre-PR self-check". Local-only (no gh/network call),
	// so it runs regardless of whether the remote/repo above was derivable.
	var laundered []foreignRefFinding
	var masqueraded []masqueradeRefFinding
	var strayBased []strayBaseRefFinding
	var unchecked []uncheckedRef
	for _, ref := range refs {
		found, cerr := checkForeignCommits("", ref.branch, ref.localSHA)
		if cerr != nil {
			// checkForeignCommits currently never returns a non-nil error (it reports
			// could-not-check inline); kept so a future stricter variant has somewhere to report.
			fmt.Fprintf(stderr, "deskpushguard: cannot check %s for foreign commits: %v — allowing (fail-open)\n", ref.branch, cerr)
			continue
		}
		for _, f := range found.foreign {
			laundered = append(laundered, foreignRefFinding{branch: ref.branch, commit: f})
		}
		for _, m := range found.masquerades {
			masqueraded = append(masqueraded, masqueradeRefFinding{branch: ref.branch, commit: m})
		}
		for _, s := range found.strayBases {
			strayBased = append(strayBased, strayBaseRefFinding{branch: ref.branch, base: s})
		}
		for _, r := range found.indeterminate {
			unchecked = append(unchecked, uncheckedRef{branch: ref.branch, reason: r})
		}
	}

	// COULD-NOT-CHECK is announced BEFORE any verdict, and whether or not the push is refused.
	// Silence used to mean two different things — "looked, found nothing" and "could not
	// look" — and only one of them is safe. Per brief-10 a client-side hook does not wedge a
	// push on indeterminacy, so this warns and audits rather than refusing; what it must never
	// do is let an unverifiable base pass as a verified one.
	if len(unchecked) > 0 {
		for _, u := range unchecked {
			fmt.Fprintf(stderr, "deskpushguard: COULD-NOT-CHECK the base of %s: %s. This is NOT "+
				"a clean bill of health — the base was never verified. Allowing the push "+
				"(fail-open, brief-10); re-check by hand with `git log refs/remotes/origin/main..HEAD`.\n",
				u.branch, u.reason)
		}
		auditUnchecked(stderr, unchecked)
	}

	if len(blocked) > 0 || len(laundered) > 0 || len(masqueraded) > 0 || len(strayBased) > 0 || len(collisions) > 0 {
		for _, b := range blocked {
			msg := fmt.Sprintf("refusing: PR #%d for %s is %s — a merged/closed PR is DONE; open a NEW branch for follow-up.",
				b.pr, b.branch, b.state)
			fmt.Fprintln(stderr, msg)
		}
		for _, l := range laundered {
			// The remediation is spelled refs/remotes/origin/main deliberately: the bare
			// `origin/main` form this message used to suggest is exactly what produces the
			// stray-base cut below when a refs/heads/origin/main exists.
			fmt.Fprintf(stderr, "refusing: %s: commit %s (%q) is also reachable from %s — foreign commit dragged in from a sibling branch, not main. Re-cut with `git worktree add <path> refs/remotes/origin/main --detach` (#22).\n",
				l.branch, shortSHA(l.commit.sha), l.commit.subject, l.commit.sourceBranch)
		}
		for _, s := range strayBased {
			fmt.Fprintf(stderr, "refusing: %s: branch point %s is the tip of the STRAY LOCAL branch refs/heads/origin/main, not refs/remotes/origin/main (%s)%s — this ref was cut from a stale shadow of main. `git worktree add <path> origin/main --detach` picks refs/heads/ over refs/remotes/ and only warns. Re-cut with `git worktree add <path> refs/remotes/origin/main --detach` (#22).\n",
				s.branch, shortSHA(s.base.strayTip), shortSHA(s.base.trueBase), behindPhrase(s.base.behind))
		}
		for _, m := range masqueraded {
			fmt.Fprintf(stderr, "refusing: %s: commit %s (%q) claims to be a merge but has fewer than two parents — fake rebase masquerade (#72).\n",
				m.branch, shortSHA(m.commit.sha), m.commit.subject)
		}
		for _, c := range collisions {
			fmt.Fprintf(stderr, "refusing: %s: register entry %s claims id %q, already claimed by %s on %s — rename/re-derive the id before pushing.\n",
				c.branch, c.ownPath, c.id, c.sourcePath, c.sourceBranch)
		}
		if len(blocked) > 0 {
			auditBlock(stderr, blocked)
		}
		if len(laundered) > 0 || len(masqueraded) > 0 || len(strayBased) > 0 {
			auditForeign(stderr, laundered, masqueraded, strayBased)
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

// foreignRefFinding pairs a laundered commit with the ref it was found on.
type foreignRefFinding struct {
	branch string
	commit foreignCommit
}

// strayBaseRefFinding pairs a stray-base cut with the ref it was found on.
type strayBaseRefFinding struct {
	branch string
	base   strayBase
}

// uncheckedRef pairs a could-not-check reason with the ref whose base could not be verified.
type uncheckedRef struct {
	branch string
	reason string
}

// behindPhrase renders the commit-count clause of the stray-base diagnostic, omitting it when
// the count could not be taken (behind < 0) rather than printing a misleading "0 commits".
func behindPhrase(behind int) string {
	if behind < 0 {
		return ""
	}
	return fmt.Sprintf(", leaving it %d commit(s) behind main", behind)
}

// masqueradeRefFinding pairs a single-parent merge-masquerade commit with the ref it was
// found on.
type masqueradeRefFinding struct {
	branch string
	commit mergeMasquerade
}

func parseRef(line string) (refLine, bool) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return refLine{}, false
	}
	local := parts[0]
	var branch string
	if strings.HasPrefix(local, "refs/heads/") {
		// Keep the FULL slashed branch name (e.g. fix/issue-108). filepath.Base
		// collapses it to the last segment (issue-108), so fetchPR looks up a
		// branch that does not exist, finds no PR, and the guard fails OPEN on
		// the house slash-branch convention — a silent no-op (#267).
		branch = strings.TrimPrefix(local, "refs/heads/")
	} else {
		// Tags and other refs keep their leaf name (e.g. refs/tags/v1.0 -> v1.0).
		branch = filepath.Base(local)
	}
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

// auditForeign logs an audit entry recording which branches were blocked for carrying a
// foreign (laundered) commit, a single-parent merge masquerade, or a stray-base cut
// (#22, #72).
func auditForeign(stderr io.Writer, laundered []foreignRefFinding, masqueraded []masqueradeRefFinding, strayBased []strayBaseRefFinding) {
	parts := make([]string, 0, len(laundered)+len(masqueraded)+len(strayBased))
	for _, l := range laundered {
		parts = append(parts, fmt.Sprintf("%s: foreign-commit %s from %s", l.branch, shortSHA(l.commit.sha), l.commit.sourceBranch))
	}
	for _, m := range masqueraded {
		parts = append(parts, fmt.Sprintf("%s: merge-masquerade %s", m.branch, shortSHA(m.commit.sha)))
	}
	for _, s := range strayBased {
		parts = append(parts, fmt.Sprintf("%s: stray-base cut from refs/heads/origin/main %s", s.branch, shortSHA(s.base.strayTip)))
	}
	detail := fmt.Sprintf("refused push carrying foreign/laundered commits: %s", strings.Join(parts, ", "))
	if err := deskkit.Log(deskkit.Entry{
		Tool:   "deskpushguard",
		Verb:   "pre-push",
		Result: deskkit.ResultRefused,
		Detail: detail,
	}); err != nil {
		fmt.Fprintf(stderr, "deskpushguard: audit log error: %v\n", err)
	}
}

// auditUnchecked records the refs whose base could NOT be verified.
//
// Result class is ResultUnwritten (#448), not ResultRefused and not ResultOK. ResultRefused is
// a POSITIVE compiled-in determination ("this is disallowed") and nothing was determined here;
// ResultOK would assert a clean bill of health the tool does not have. ResultUnwritten is
// exactly this epistemic state — "a local determination came back short, and no outward write
// was attempted by this tool" — and it is non-charging, which is right for a check that never
// touches a remote. The push itself is still allowed (brief-10 fail-open, exit 0); what the
// audit line guarantees is that reading the log back later distinguishes a push that went out
// UNVERIFIED from one that was checked and found clean.
func auditUnchecked(stderr io.Writer, unchecked []uncheckedRef) {
	parts := make([]string, len(unchecked))
	for i, u := range unchecked {
		parts[i] = fmt.Sprintf("%s: %s", u.branch, u.reason)
	}
	detail := fmt.Sprintf("base NOT verified (could-not-check), push allowed fail-open: %s", strings.Join(parts, "; "))
	if err := deskkit.Log(deskkit.Entry{
		Tool:   "deskpushguard",
		Verb:   "pre-push",
		Result: deskkit.ResultUnwritten,
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
