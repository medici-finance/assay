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
