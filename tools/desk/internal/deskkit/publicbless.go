package deskkit

// publicbless.go — the standing per-repo authorization for public-repo writes.
//
// PublicRepoGate's default posture on a public/internal repo is a PER-WRITE
// human +1 on the associated issue/PR (repovis.go). That is the right default,
// but it makes fully autonomous operation on a deliberately opted-in public
// repo impossible: `deskpr create` has no issue surface yet (issueNumber 0)
// and fails closed, and every review needs a fresh reaction.
//
// The standing bless is the human's way to opt NAMED repos out of the per-write
// +1: a local sentinel file listing exact `owner/name` repos, one per line.
// A repo on the list passes the gate with a stderr NOTICE; every other repo is
// gated exactly as before. Because PublicRepoGate is the one choke point every
// write-capable verb calls, a bless covers create, review, ready, reply,
// evidence and release writes on the named repo — scope the list accordingly.
//
// The mechanics mirror the writeguard shared-checkout sentinel
// (cmd/writeguard: ~/.config/assay/writeguard-shared-ok), the established
// human-only standing-claim idiom:
//
//   - HUMAN-ONLY: no desk tool writes this file, ever — not to create it, not
//     to append to it. It is created out-of-band, by a person:
//
//       mkdir -p ~/.config/assay
//       printf 'owner/name\n' >> ~/.config/assay/public-app-ok
//
//   - FAIL CLOSED: a missing, unreadable, or malformed file blesses NOTHING.
//     Every anomaly answers "not blessed" and the per-write +1 gate applies
//     exactly as if the file did not exist. Unreadable is never "blessed".
//
//   - PER-REPO, NEVER GLOBAL: the unit of authorization is one exact
//     `owner/name`. There is no wildcard form and no "skip the gate" switch;
//     a line that is not exactly one owner and one name never matches
//     anything. This deliberately diverges from the writeguard sentinel,
//     whose EMPTY file claims any checkout: an empty bless file blesses zero
//     repos.
//
//   - VISIBLE: every skip is announced on stderr (NOTICE) naming the repo and
//     the sentinel, so the relaxation is on the audit trail, never silent.
//
// Two further deliberate divergences from the writeguard idiom:
//
//   - No env-token alternative. The writeguard hook reads its OWN process
//     env, which a per-call tool shell cannot reach — there the env token is
//     human-shell-only by construction. The desk verbs run IN the calling
//     shell and inherit its environment, so an env bless here would be
//     claimable inline by the very automation the gate stands in front of.
//     The file, created out-of-band, is the only claim surface.
//
//   - No expiry. The writeguard claim is session-scoped, so it expires with a
//     TTL; this bless is a STANDING authorization recorded by a human ruling,
//     and it stands until the human removes the line (or the file). Revoke by
//     deleting the line.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// PublicBlessSentinelName is the file name of the standing-bless sentinel
// within the config directory (~/.config/assay by default).
const PublicBlessSentinelName = "public-app-ok"

// publicBlessNoticeW is where the bless-skip NOTICE is written. Package
// variable so tests can capture it; production always leaves it at os.Stderr.
var publicBlessNoticeW io.Writer = os.Stderr

// publicBlessSentinelPath returns the sentinel file path:
// $XDG_CONFIG_HOME/assay/public-app-ok when XDG_CONFIG_HOME is set, else
// ~/.config/assay/public-app-ok — the same resolution the writeguard
// sentinel uses. Error means "no resolvable path", which callers treat as
// not blessed.
func publicBlessSentinelPath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "assay", PublicBlessSentinelName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("no home directory")
	}
	return filepath.Join(home, ".config", "assay", PublicBlessSentinelName), nil
}

// publicRepoBlessed reports whether owner/repo carries a standing bless, and
// the sentinel path consulted (for the NOTICE). Every failure mode — no
// resolvable path, missing file, a directory where the file should be, an
// unreadable file — answers false: the gate then applies normally. The match
// is exact `owner/name`, case-insensitive, whitespace-trimmed; blank lines
// and `#` comments are skipped; any other malformed line (no slash, extra
// slashes, embedded whitespace, a wildcard) simply never matches, so a
// corrupt file can only ever UNDER-bless, never over-bless.
func publicRepoBlessed(owner, repo string) (bool, string) {
	path, err := publicBlessSentinelPath()
	if err != nil {
		return false, ""
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return false, path
	}
	// A group- or world-WRITABLE sentinel is not a human's standing
	// authorization — any local process could have appended to it. Same
	// posture as the roster loader in this package: refuse (bless nothing)
	// rather than trust a file other principals can edit.
	if fi.Mode().Perm()&0o022 != 0 {
		return false, path
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return false, path
	}
	target := strings.ToLower(strings.TrimSpace(owner)) + "/" + strings.ToLower(strings.TrimSpace(repo))
	if !wellFormedRepoLine(target) {
		return false, path // an empty owner or name must never match anything
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !wellFormedRepoLine(line) {
			continue // malformed lines bless nothing
		}
		if line == target {
			return true, path
		}
	}
	return false, path
}

// wellFormedRepoLine reports whether s is exactly `owner/name`: one slash,
// both halves non-empty, no whitespace, no wildcard characters. This is a
// positive shape check, not a sanitizer — anything that fails it blesses
// nothing.
func wellFormedRepoLine(s string) bool {
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	return !strings.ContainsAny(s, "*?[ \t")
}
