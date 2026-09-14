package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// gitLastTouch returns the commit time of the last commit touching relDir,
// falling back to the directory's mtime when untracked or git is unavailable.
func gitLastTouch(root, relDir string) time.Time {
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%ct", "--", relDir).Output()
	s := strings.TrimSpace(string(out))
	if err == nil && s != "" {
		if sec, perr := strconv.ParseInt(s, 10, 64); perr == nil {
			return time.Unix(sec, 0).UTC()
		}
	}
	if info, serr := os.Stat(filepath.Join(root, relDir)); serr == nil {
		return info.ModTime().UTC()
	}
	return time.Time{}
}

// gitCurrentSHA returns the current HEAD commit sha for root, or "" if
// unavailable (e.g. a non-git checkout — testdata fixtures copied to a plain
// tempdir have no .git, and that is not an error worth failing a run over).
func gitCurrentSHA(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitCommitTime returns the commit timestamp of HEAD as UTC, or the zero time
// when git is unavailable. Rendered timestamps in generated artifacts use this
// so the output is byte-deterministic for a given tree.
//
// %ct is the committer date (matching gitLastTouch above); it equals the author
// date for an ordinary commit and diverges only after a rebase or amend. Either
// is fixed for a given commit object, so determinism holds regardless.
//
// The zero time is returned for every "no HEAD to read" case — a non-git
// directory, a freshly-initialised repo with an unborn HEAD, or git missing
// from PATH — never a panic. Callers MUST treat the zero time as "unknown" and
// fall back to wall-clock time; they must not render it as an epoch date.
// A shallow clone (`--depth 1`) is fine: HEAD is present, so the timestamp is
// exact even though history behind it is not.
func gitCommitTime(root string) time.Time {
	out, err := exec.Command("git", "-C", root, "show", "-s", "--format=%ct", "HEAD").Output()
	s := strings.TrimSpace(string(out))
	if err != nil || s == "" {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

// defaultRemoteBranchTimeout bounds listRemoteBranches so an unreachable
// "origin" (offline host, no network in CI) fails fast instead of hanging
// --lint/--check.
//
// It was 3s, which is short enough to trip on an ordinary slow remote, a cold
// DNS cache or a busy runner — and every trip drops claim
// filtering. Raising it is a SECONDARY measure only: the primary fix is that a
// failed read is now reported instead of silently dropped (see ClaimSource).
// A too-short timeout only changes how OFTEN the degraded path is taken.
const defaultRemoteBranchTimeout = 10 * time.Second

// remoteTimeoutEnv overrides defaultRemoteBranchTimeout with a Go duration
// (e.g. "30s"). An unparseable or non-positive value falls back to the default
// — a typo'd env var must not silently shorten the window to zero.
const remoteTimeoutEnv = "STATUSGEN_REMOTE_TIMEOUT"

func remoteBranchTimeout() time.Duration {
	if v := strings.TrimSpace(os.Getenv(remoteTimeoutEnv)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultRemoteBranchTimeout
}

// listRemoteBranches enumerates short branch names on the "origin" remote via
// `git ls-remote --heads`. It returns a NON-NIL error on any failure or timeout,
// naming the cause: callers MUST treat that as "claims could not be determined"
// and say so out loud (resolveClaims / ClaimSource), never as "nothing is
// claimed" and never as "everything is claimed."
// A package-level var so tests can substitute a fake lister without a network call.
var listRemoteBranches = func(root string) (branches []string, err error) {
	timeout := remoteBranchTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-remote", "--heads", "origin")
	// Run git in its own process group and kill the WHOLE GROUP on timeout, so
	// orphaned remote helpers do not pile up. This is platform-specific: on unix it
	// sets a process group and kills it; on windows it cannot and degrades loudly.
	// See killWholeProcessGroup in procgroup_{unix,windows}.go.
	killWholeProcessGroup(cmd)
	// WaitDelay is what makes the deadline real. exec kills `git` when ctx
	// expires, but Wait still blocks on the stdout/stderr pipes — and git's
	// remote helper (git-remote-<scheme>, a GRANDCHILD) inherits and holds them
	// open, so a hung helper made the "fails fast" timeout wait indefinitely.
	// After the delay the pipes are closed and Wait returns.
	cmd.WaitDelay = time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if rerr := cmd.Run(); rerr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("`git ls-remote --heads origin` timed out after %s (raise with %s)", timeout, remoteTimeoutEnv)
		}
		if msg := firstLine(stderr.String()); msg != "" {
			return nil, fmt.Errorf("`git ls-remote --heads origin` failed: %s", msg)
		}
		return nil, fmt.Errorf("`git ls-remote --heads origin` failed: %v", rerr)
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		branches = append(branches, strings.TrimPrefix(fields[1], "refs/heads/"))
	}
	return branches, nil
}

// hasNoGitDir reports whether root has no .git entry at all — the case for a
// `git archive` export: there is no history, no remotes, nothing
// git can answer about this tree.
//
// This is deliberately distinct from "a real git checkout where origin/main
// happens to be unresolvable" (shallow clone, offline, no remote configured).
// Several checks (grandfatheredIDs/T9 among them) fail CLOSED in that second
// case on purpose, as an anti-forgery measure for an adversarial CI checkout —
// a hostile branch might otherwise manufacture "this ref can't be resolved" to
// win a bypass. A `git archive` export is not a checkout anything is merging
// into; there is no adversary to fail closed against, only an absence of data.
// Callers that would otherwise mis-fire on that absence (rather than simply
// having nothing to check) should test hasNoGitDir and skip outright.
func hasNoGitDir(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".git"))
	return os.IsNotExist(err)
}

// gitPathLastAuthorIdentity returns a stable identity key (the author email,
// lowercased) for the MOST RECENT commit that touched relPath under root — for
// a brief file, the commit that most recently added or edited its Evidence /
// flipped its status. ok is false when git is unavailable, root has no .git, or
// the path has no commit history (an untracked, just-added file): callers MUST
// degrade LOUDLY on !ok and never treat it as a pass.
//
// The author email is the identity key rather than the display name because a
// name is trivially re-typed while the email is what the commit is attributed
// to; distinct bot/human identities carry distinct emails
// (`…+assay-worker-app[bot]@users.noreply.github.com` vs the verifier App's).
// This is a committer-identity signal for the attribution cross-check, NOT a
// timestamp; see attribution.go's identity cross-check for how it is used and
// why it is a NOTICE, not a hard gate (file-level git attribution is
// best-effort — the same shared-identity caveat attributionProblems documents).
func gitPathLastAuthorIdentity(root, relPath string) (identity string, ok bool) {
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%ae", "--", relPath).Output()
	if err != nil {
		return "", false
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "", false
	}
	return strings.ToLower(s), true
}

// gitPathFirstAuthorIdentity returns the identity key (author email,
// lowercased) of the FIRST commit that introduced relPath — for a brief file,
// its authoring commit. Same ok semantics as gitPathLastAuthorIdentity: false
// when git is unavailable or the path has no history, and callers degrade
// loudly rather than pass. The oldest commit is read via `--reverse` and the
// first non-empty line taken, so a file that landed in one commit and a file
// with a long edit history both resolve to whoever first committed it.
func gitPathFirstAuthorIdentity(root, relPath string) (identity string, ok bool) {
	out, err := exec.Command("git", "-C", root, "log", "--reverse", "--format=%ae", "--", relPath).Output()
	if err != nil {
		return "", false
	}
	for _, ln := range strings.Split(string(out), "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			return strings.ToLower(t), true
		}
	}
	return "", false
}

// firstLine returns the first non-empty line of s, trimmed — git's stderr is
// often multi-line and only the first line names the cause.
func firstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			return t
		}
	}
	return ""
}

// --- the one-walk authorship index ----------------------------------------------------------

// pathAuthors is the first and last author identity of one path, both already lowercased.
type pathAuthors struct {
	first string // the identity of the commit that INTRODUCED the path
	last  string // the identity of the most recent commit that touched it
}

// buildPathAuthorIndex answers, in ONE git invocation, what
// gitPathFirstAuthorIdentity + gitPathLastAuthorIdentity answer in TWO per path.
//
// WHY THIS EXISTS. The attribution cross-check ran that pair for every verified/done brief,
// serially: on a tree of 165 briefs one --lint made 144 `git log` and 62 `git blame`
// invocations, and the subprocess overhead — not the work — dominated the wall clock. The same
// history read as one `git log --name-only` over the streams directory takes 0.03 s and answers
// every path at once.
//
// SEMANTICS, deliberately identical to the pair it replaces. `git log` walks newest-first, so a
// path's FIRST sighting in the output is its most recent commit (last) and its LAST sighting is
// its introducing commit (first) — the same two commits `-1` and `--reverse` select. The same
// default history simplification applies, because the pathspec is a directory containing the
// same files rather than a different query. Identity is the author email lowercased, exactly as
// the pair returns it.
//
// ok is false when git is unavailable or the walk fails. A path ABSENT from a successfully-built
// index is NOT an error here: the caller falls back to the per-path read for it, so a path this
// walk cannot account for costs two subprocesses rather than a lost signal. That fallback is the
// reason this optimisation cannot change a check's ANSWER, only its cost.
func buildPathAuthorIndex(root, subdir string) (map[string]pathAuthors, bool) {
	// core.quotePath=false keeps a non-ASCII path spelled as itself rather than C-quoted, so an
	// index key always compares equal to the caller's own relative path.
	out, err := exec.Command("git", "-C", root, "-c", "core.quotePath=false",
		"log", "--format=%x01%ae", "--name-only", "--", subdir).Output()
	if err != nil {
		return nil, false
	}
	idx := make(map[string]pathAuthors, 512)
	cur := ""
	for _, ln := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(ln, "\x01") {
			cur = strings.ToLower(strings.TrimSpace(ln[1:]))
			continue
		}
		p := strings.TrimSpace(ln)
		if p == "" || cur == "" {
			continue
		}
		e, seen := idx[p]
		if !seen {
			e.last = cur // newest-first: the first sighting is the most recent commit
		}
		e.first = cur // overwritten on every sighting; the final one is the oldest
		idx[p] = e
	}
	return idx, true
}

// lookupPathAuthors resolves one path through the index, falling back to the per-path reads when
// the index is absent or does not account for it. Returning the two values TOGETHER is what lets
// the caller keep its existing all-or-nothing degradation: a path for which either half is
// unavailable degrades LOUDLY, exactly as before.
func lookupPathAuthors(idx map[string]pathAuthors, root, relPath string) (first, last string, ok bool) {
	if idx != nil {
		if e, hit := idx[relPath]; hit && e.first != "" && e.last != "" {
			return e.first, e.last, true
		}
	}
	f, fok := gitPathFirstAuthorIdentity(root, relPath)
	l, lok := gitPathLastAuthorIdentity(root, relPath)
	if !fok || !lok {
		return "", "", false
	}
	return f, l, true
}
