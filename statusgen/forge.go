package main

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
)

// forgeKind classifies the hosting forge behind a tracking root's `origin`
// remote. statusgen's data model is forge-agnostic, but two surfaces are not:
// the CI half `init` scaffolds, and the dead-claim decay pass, which reads
// PR/MR state through `gh` (a GitHub-only client). Both must know which forge
// they are on so they can scaffold the matching CI file and say honestly whether
// a GitHub-only pass applies here at all (#349).
type forgeKind int

const (
	// forgeUnknown is the honest default when the remote cannot be read or its
	// host matches neither known forge (a self-hosted host that names neither
	// "github" nor "gitlab", or no remote at all). It is NOT rounded to GitHub:
	// callers keep their pre-forge behaviour on unknown (init scaffolds the
	// historical GitHub half; decay still attempts `gh` and degrades loudly),
	// because "could not tell" is not "confirmed not GitHub".
	forgeUnknown forgeKind = iota
	forgeGitHub
	forgeGitLab
)

func (f forgeKind) String() string {
	switch f {
	case forgeGitHub:
		return "github"
	case forgeGitLab:
		return "gitlab"
	default:
		return "unknown"
	}
}

// parseForgeFlag maps a `--forge` flag value to a forgeKind, rejecting anything
// but the two forges statusgen can scaffold. An empty string means "not given"
// and resolves to forgeUnknown so detection takes over.
func parseForgeFlag(s string) (forgeKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return forgeUnknown, nil
	case "github":
		return forgeGitHub, nil
	case "gitlab":
		return forgeGitLab, nil
	default:
		return forgeUnknown, fmt.Errorf("unknown --forge %q: expected github or gitlab", s)
	}
}

// remoteHost extracts the lowercased host from a git remote URL, handling both
// the scp-like SSH form (`git@host:owner/repo.git`) and the URL forms
// (`https://host/…`, `ssh://git@host/…`). Returns "" when no host is discernible.
func remoteHost(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// scp-like syntax has no scheme and uses `host:path` (a colon before any
	// slash). Distinguish it from `ssh://…` by the absence of `://`.
	if !strings.Contains(s, "://") {
		if at := strings.LastIndex(s, "@"); at >= 0 {
			s = s[at+1:]
		}
		if colon := strings.Index(s, ":"); colon >= 0 {
			return strings.ToLower(s[:colon])
		}
		return strings.ToLower(s)
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// classifyForgeHost maps a remote HOST to a forgeKind. It matches on the host's
// dotted labels, never a substring of the whole URL — so a GitHub repo literally
// named "gitlab-migration" is not misread as GitLab. A host whose labels include
// "github" (github.com, github.example.com, a GHE host that carries the word) is
// GitHub; likewise "gitlab"; anything else is unknown.
func classifyForgeHost(host string) forgeKind {
	if host == "" {
		return forgeUnknown
	}
	for _, label := range strings.Split(host, ".") {
		switch label {
		case "github":
			return forgeGitHub
		case "gitlab":
			return forgeGitLab
		}
	}
	return forgeUnknown
}

// classifyForgeURL is the pure URL→forge classifier, split out so it can be
// tested without a git checkout.
func classifyForgeURL(raw string) forgeKind {
	return classifyForgeHost(remoteHost(raw))
}

// remoteOriginURL returns the `origin` remote URL for the repo rooted at root.
// A package-level var so tests substitute a fake without a git checkout, matching
// the listRemoteBranches / listMergedClosedBranches injection pattern.
var remoteOriginURL = func(root string) (string, error) {
	out, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// detectForge classifies the forge behind root's `origin` remote. It returns
// forgeUnknown (never an error) when the remote cannot be read — a tracking root
// with no origin, a non-git directory, or git absent — so callers get a
// three-state answer (github / gitlab / unknown) and never a hard failure over a
// missing remote.
func detectForge(root string) forgeKind {
	raw, err := remoteOriginURL(root)
	if err != nil {
		return forgeUnknown
	}
	return classifyForgeURL(raw)
}
