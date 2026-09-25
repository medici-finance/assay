package deskkit

import "strings"

// transporthost.go — two read-only predicates a caller OUTSIDE this package needs to build a
// worktree's App transport (`deskwt add --role`, #861) without re-deriving either rule:
//
//   - IsSSHTransport: git's own "is this remote URL carried over SSH" rule — the one the
//     push-transport gate (pushtransport.go) refuses on. One definition, so the writer that
//     replaces an SSH transport and the gate that refuses one can never disagree about what
//     counts as SSH.
//   - IsWellKnownForgeHost: the unambiguous forge-host table the forge resolver maps a remote
//     host through (forgeresolve.go, resolution step b).

// IsSSHTransport reports whether a git remote URL is carried over SSH, by git's own rules: an
// explicit ssh-family scheme, or the scp-like `[user@]host:path` shorthand. It is the exported
// face of the rule CheckPushTransport refuses on.
func IsSSHTransport(u string) bool { return isSSHTransport(u) }

// IsWellKnownForgeHost reports whether host is one of the forge hosts the resolver maps
// unambiguously (github.com, gitlab.com). Case-insensitive; no suffix or prefix match.
func IsWellKnownForgeHost(host string) bool {
	_, ok := wellKnownForgeHosts[strings.ToLower(strings.TrimSpace(host))]
	return ok
}
