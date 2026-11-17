package deskkit

import (
	"errors"
	"regexp"
	"strings"
)

// NormalizeForgeHost folds ASCII hostname letters to lower case. A non-ASCII
// byte is an error, never an alias for an ASCII forge. The empty hostname stays
// empty so each caller can apply its existing missing-host policy.
func NormalizeForgeHost(host string) (string, error) {
	if hasNonASCIIByte(host) {
		return "", errors.New("forge hostname must use ASCII")
	}
	return strings.ToLower(host), nil
}

// A bare git host can contain DNS label characters and underscores in local names.
var bareGitHost = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// IsBareGitHost reports whether host can be used as a git transport hostname
// without a port, user information, or URL delimiters.
func IsBareGitHost(host string) bool { return bareGitHost.MatchString(host) }

// IsSSHTransport reports whether a git remote URL is carried over SSH, by git's own rules: an
// explicit ssh-family scheme, or the scp-like `[user@]host:path` shorthand. It is the exported
// face of the rule CheckPushTransport refuses on.
func IsSSHTransport(u string) bool { return isSSHTransport(u) }

// IsWellKnownForgeHost reports whether host is one of the forge hosts the resolver maps
// unambiguously (github.com, gitlab.com). Case-insensitive; no suffix or prefix match.
func IsWellKnownForgeHost(host string) bool {
	h, err := NormalizeForgeHost(host)
	if err != nil {
		return false
	}
	_, ok := wellKnownForgeHosts[strings.TrimSpace(h)]
	return ok
}
