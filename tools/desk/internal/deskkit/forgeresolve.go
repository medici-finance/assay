package deskkit

// forgeresolve.go — ForgeFor, the ONE function in this tree allowed to construct a Forge
// backend (GitHubForge or GitLabForge). Two complete backends have existed since the
// forge-abstraction stream landed and nothing in the fleet could obtain one: no
// constructor, no resolver, no config key. This file is that missing answer.
//
// THE CONTRACT, stated once so every later brief inherits it rather than re-deriving it:
//
//  1. RESOLUTION. The forge that serves a repo is never a caller's choice. ForgeFor takes
//     no forge parameter, reads no forge-selecting flag or environment variable, and no
//     other exported symbol in this package accepts one. Resolution order:
//
//     a. the repo's configured forge — ASSAY_REPO_FORGES in the roster (rosterconfig.go);
//     b. failing that, the origin remote's host, mapped to a forge ONLY when the mapping
//     is unambiguous (github.com -> github, gitlab.com -> gitlab; anything else,
//     including an unreachable or absent remote, is NOT a match);
//     c. failing that, Unverifiable — could-not-check, naming the repo and the
//     configuration that would resolve it. Never a default, never a guess.
//
//  2. CUSTODY. ForgeFor obtains the resolved role's already-minted token from the
//     existing custody path for that forge and hands it to the backend it constructs.
//     It never mints a credential itself (GitHub: it either calls the installed
//     GitHubCustodyMinter hook — see below — or shells to the existing `desktoken`
//     mint-or-reuse path via RoleTokenForRepo; GitLab: it only READS the file a prior
//     `desktoken --forge gitlab <role>` rotation produced — it never rotates). It never
//     falls back to an ambient CLI credential. A missing or insecurely-permissioned
//     custody file is Refused (exit 5) naming the remedy: this is a deployment
//     precondition an operator has to fix, not a transient could-not-check.
//
//  3. REFUSAL. When the resolved forge cannot serve an operation, the backend returns
//     Unverifiable (exit 6) naming the forge, the operation and the gap — never a silent
//     success, never the OTHER forge's behavior, never a raw request against anything.
//     GitLabForge.DeleteRef (forge_gitlab.go) is the reference shape: it refuses outside
//     the one namespace GitLab Community Edition actually maps, and says so by name. Every
//     later brief that adds an operation with a partial per-forge mapping follows that
//     shape, not a zero-value return.
//
// SINGLE-POINT-OF-FAILURE. This file is the one place a backend is constructed, so the
// control that matters is that no OTHER construction site can exist. It is backed by a
// second, independent layer that fails for a different reason in a different component:
// the `forge-surface-control.yml` CI job (forgeban), whose no-passthrough shape check and
// permit-register ratchet already refuse a new forge-CLI call site or an exported raw
// request method. TestForgeSingleConstructionSite (forgeresolve_test.go) is the layer
// that catches a stray `GitHubForge{}`/`GitLabForge{}` literal; the CI job catches a
// different way of going around this resolver entirely (a shell-out, a passthrough).

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ForgeKind names one of the two forges this tree knows how to construct a backend for.
type ForgeKind string

const (
	ForgeGitHub ForgeKind = "github"
	ForgeGitLab ForgeKind = "gitlab"
)

// ForgeResolution records HOW a repo's forge was determined, so a caller can report the
// provenance rather than silently trusting the answer.
type ForgeResolution struct {
	Repo ForgeRepo
	Kind ForgeKind
	// Source names the resolution step that answered: "repo-config" (ASSAY_REPO_FORGES)
	// or "remote-host:<host>" (the unambiguous host mapping).
	Source string
}

// wellKnownForgeHosts is the UNAMBIGUOUS remote-host mapping (resolution step b). It is
// deliberately a short, exact-match table: a self-hosted GitLab or GitHub Enterprise
// instance has no host literal this tree can compile in, so it is NOT ambiguous by
// omission — it simply does not match, and resolution falls through to the roster
// configuration requirement (step c refuses rather than guessing).
var wellKnownForgeHosts = map[string]ForgeKind{
	"github.com": ForgeGitHub,
	"gitlab.com": ForgeGitLab,
}

// originRemoteHost reads the "origin" remote's host from the current working directory's
// git checkout. It is a package var so a test can replace it without depending on the
// ambient git state of whatever checkout happens to run the suite.
var originRemoteHost = func() (string, error) {
	out, err := gitOut(".", "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	return hostOfRemote(strings.TrimSpace(out))
}

// hostOfRemote extracts the host component from a git remote URL in either an
// scheme://host/... form or an scp-like user@host:path form.
func hostOfRemote(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty remote URL")
	}
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			return "", fmt.Errorf("cannot parse remote URL %q", raw)
		}
		return strings.ToLower(u.Hostname()), nil
	}
	at := strings.Index(raw, "@")
	colon := strings.Index(raw, ":")
	if at >= 0 && colon > at {
		return strings.ToLower(raw[at+1 : colon]), nil
	}
	return "", fmt.Errorf("cannot determine a host from remote URL %q", raw)
}

// knownForgeHostsList renders wellKnownForgeHosts for a refusal message, sorted so the
// message is deterministic.
func knownForgeHostsList() string {
	hosts := make([]string, 0, len(wellKnownForgeHosts))
	for h := range wellKnownForgeHosts {
		hosts = append(hosts, h)
	}
	// Two entries today; a fixed short list does not need sort.Strings's import cost to
	// stay deterministic — but if that ever changes, sort here rather than depending on
	// map iteration order leaking into a user-facing message.
	if len(hosts) == 2 && hosts[0] == "gitlab.com" {
		hosts[0], hosts[1] = hosts[1], hosts[0]
	}
	return strings.Join(hosts, ", ")
}

// resolveForgeKind implements resolution steps a/b/c of the contract above.
func resolveForgeKind(repo ForgeRepo) (ForgeResolution, error) {
	key := strings.ToLower(strings.TrimSpace(repo.Slug()))
	cfg := EffectiveConfig()
	if kind, ok := cfg.RepoForges[key]; ok {
		return ForgeResolution{Repo: repo, Kind: ForgeKind(kind), Source: "repo-config:" + EnvRepoForges}, nil
	}
	if host, herr := originRemoteHost(); herr == nil {
		if kind, ok := wellKnownForgeHosts[strings.ToLower(host)]; ok {
			return ForgeResolution{Repo: repo, Kind: kind, Source: "remote-host:" + host}, nil
		}
	}
	return ForgeResolution{}, Unverifiable(fmt.Sprintf(
		"cannot resolve which forge serves %s: no %s entry names it, and the origin remote's host is "+
			"absent, unreadable, or does not map unambiguously to a known forge (%s). Configure "+
			"%s=%s=github (or =gitlab) in the roster — never a flag or environment variable, which this "+
			"resolver deliberately does not read.",
		repo.Slug(), EnvRepoForges, knownForgeHostsList(), EnvRepoForges, repo.Slug()), nil)
}

// --- Custody: obtaining the resolved role's already-minted token ------------------------

// GitHubCustodyMinterFunc mints (or reuses a cached) GitHub App installation token for
// role against repo, and reports the base URL the caller's OWN transport is bound to (""
// meaning the real GitHubAPIBase). It is read AT CALL TIME, not cached, so a per-test
// override of the caller's own base-URL variable still reaches the Forge this produces.
type GitHubCustodyMinterFunc func(role string, repo ForgeRepo) (token, baseURL string, err error)

// githubCustodyMinter is the installable seam ForgeFor's GitHub branch calls instead of
// its default (RoleTokenForRepo, which shells to the `desktoken` binary's mint-or-reuse
// path). A caller that already mints its OWN GitHub App installation tokens in-process —
// deskpost, extracted from the live behavior this package's golden corpus pins — installs
// its existing, tested minter here (cmd/deskpost/github.go's init) instead of this
// package growing a SECOND JWT-signing implementation, and instead of deskpost losing the
// test harness its ~30 test files already depend on. Unset (every other caller) resolves
// through RoleTokenForRepo.
var githubCustodyMinter GitHubCustodyMinterFunc

// SetGitHubCustodyMinter installs fn as the GitHub custody step ForgeFor's resolver calls.
// Passing nil restores the default (RoleTokenForRepo). See githubCustodyMinter's doc for
// why this seam exists rather than a second mint implementation.
func SetGitHubCustodyMinter(fn GitHubCustodyMinterFunc) {
	githubCustodyMinter = fn
}

// custody obtains the token (and, where relevant, a base-URL override) for kind/role/repo
// from the existing custody path. It never mints for GitLab (rotation is a deliberate,
// infrequent operator action — see gitlabCustody) and never falls back to an ambient
// credential for either forge.
func custody(kind ForgeKind, role string, repo ForgeRepo) (token, baseURL string, err error) {
	switch kind {
	case ForgeGitHub:
		return githubCustody(role, repo)
	case ForgeGitLab:
		return gitlabCustody(role)
	default:
		return "", "", Unverifiable(fmt.Sprintf("no custody path known for forge %q", kind), nil)
	}
}

func githubCustody(role string, repo ForgeRepo) (string, string, error) {
	if strings.TrimSpace(role) == "" {
		return "", "", Refused("no App role named for the GitHub custody lookup — a token cannot be " +
			"obtained for an identity this process cannot name")
	}
	if githubCustodyMinter != nil {
		tok, base, merr := githubCustodyMinter(role, repo)
		if merr != nil {
			return "", "", Refused(fmt.Sprintf(
				"the installed GitHub custody minter refused for role %s on %s: %s",
				role, repo.Slug(), merr.Error()))
		}
		if strings.TrimSpace(tok) == "" {
			return "", "", Refused(fmt.Sprintf(
				"the installed GitHub custody minter returned an empty token for role %s on %s",
				role, repo.Slug()))
		}
		return tok, base, nil
	}
	tok, path, rerr := RoleTokenForRepo(role, repo.Slug())
	if rerr != nil {
		return "", "", Refused(fmt.Sprintf(
			"cannot obtain the %s GitHub App installation token for %s: %s — provision it "+
				"(docs/github-apps-setup.md) or repair the mint path; ForgeFor never falls back to an "+
				"ambient gh-CLI identity", role, repo.Slug(), rerr.Error()))
	}
	if verr := verifyCustodyFileMode(path); verr != nil {
		return "", "", verr
	}
	return tok, "", nil
}

// gitlabTokenFileName is the per-role custody file name, matching the shape
// cmd/desktoken/gitlab.go's rotation path already writes: gitlab-<role>.token, resolved
// across the same App-credential search path (ConfigHomeDirs).
func gitlabTokenFileName(role string) string { return "gitlab-" + role + ".token" }

func gitlabCustody(role string) (string, string, error) {
	if strings.TrimSpace(role) == "" {
		return "", "", Refused("no App role named for the GitLab custody lookup — a token cannot be " +
			"obtained for an identity this process cannot name")
	}
	name := gitlabTokenFileName(role)
	path, searched, found := FindConfigFile(name)
	if !found {
		return "", "", Refused(fmt.Sprintf(
			"gitlab token file not found: no %s on the App-credential search path. Searched: %s. "+
				"Provision the role's PAT there (0600) via a group owner, or set %s to the directory "+
				"that has it. ForgeFor never rotates or mints a GitLab credential on your behalf — it "+
				"only reads an already-provisioned custody file.",
			name, strings.Join(searched, ", "), EnvConfigHome))
	}
	if verr := verifyCustodyFileMode(path); verr != nil {
		return "", "", verr
	}
	b, rerr := os.ReadFile(path)
	if rerr != nil {
		return "", "", Refused(fmt.Sprintf("cannot read gitlab token file at %s: %v", path, rerr))
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", "", Refused(fmt.Sprintf("the gitlab token file at %s is empty", path))
	}
	return tok, gitlabAPIBaseOverride(), nil
}

// gitlabAPIBaseOverride reads GITLAB_API_BASE at call time (never cached), the same
// deployment-supplied self-hosted-instance override cmd/desktoken/gitlab.go's rotation
// path reads. Empty means GitLabForge's own default (GitLabAPIBase, gitlab.com) — unlike
// the rotation path, ForgeFor does not REQUIRE this: rotation transmits a live PAT and a
// wrong default would misdirect a credential, but reading through an already-resolved
// GitLabForge to gitlab.com when nothing else was configured is the backend's own shipped
// default behavior, not a new risk this resolver introduces.
func gitlabAPIBaseOverride() string {
	return strings.TrimSpace(os.Getenv("GITLAB_API_BASE"))
}

// verifyCustodyFileMode enforces that a custody file is a 0600 regular file — the same
// rule cmd/desktoken/gitlab.go's rotation path already enforces before it will rotate a
// PAT, applied here uniformly to every file-backed custody read. A missing, non-regular,
// or loosely-permissioned file is Refused (exit 5): the deployment has not provisioned
// this correctly, which is a precondition an operator fixes, not a could-not-check.
func verifyCustodyFileMode(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return Refused(fmt.Sprintf("cannot stat custody token file at %s: %v — re-provision it", path, err))
	}
	if !fi.Mode().IsRegular() {
		return Refused(fmt.Sprintf(
			"custody token at %s is not a regular file (mode %s); token custody requires a 0600 "+
				"regular file — re-provision it", path, fi.Mode()))
	}
	if fi.Mode().Perm() != 0o600 {
		return Refused(fmt.Sprintf(
			"custody token file at %s has permissions %o; must be 0600 — run: chmod 600 %s",
			path, fi.Mode().Perm(), path))
	}
	return nil
}

// --- The resolver -------------------------------------------------------------------

// ForgeFor is the ONLY function in this tree that constructs a GitHubForge or a
// GitLabForge — see the file header for the full resolution/custody/refusal contract.
//
// role is the App role (e.g. "reviewer", "worker") whose custody the caller acts under; it
// is NOT a forge selector — the ONLY thing role affects is WHICH cached/minted credential
// is read for whichever forge resolution already determined. There is no parameter, flag,
// or environment variable anywhere in this package by which a caller supplies the forge
// itself (TestForgeForRejectsCallerSuppliedForge).
func ForgeFor(repo ForgeRepo, role string) (Forge, error) {
	res, err := resolveForgeKind(repo)
	if err != nil {
		return nil, err
	}
	// Forge agreement is ENFORCED, not assumed: a role whose EXPLICITLY forge-qualified
	// roster entry names a forge other than the one this repo resolves to is refused here,
	// BEFORE any credential is read — a mismatched entry must never be handed a backend
	// (the forge-qualified-identity brief, assertEntryForgeAgrees). An inferred-github entry is exempt (the
	// human-gated backward-compatibility rule).
	if aerr := assertEntryForgeAgrees(role, res.Kind); aerr != nil {
		return nil, aerr
	}
	tok, base, cerr := custody(res.Kind, role, repo)
	if cerr != nil {
		return nil, cerr
	}
	switch res.Kind {
	case ForgeGitHub:
		return &GitHubForge{Token: tok, BaseURL: base}, nil
	case ForgeGitLab:
		return &GitLabForge{Token: tok, BaseURL: base}, nil
	default:
		// Unreachable given resolveForgeKind only ever returns a value from
		// wellKnownForgeHosts or cfg.RepoForges (both constrained to the two known kinds
		// at parse/lookup time) — kept as Unverifiable, never a panic, because a resolver
		// that cannot name a backend must fail closed even on a case it believes is
		// impossible.
		return nil, Unverifiable(fmt.Sprintf(
			"forge resolution for %s returned an unrecognised kind %q", repo.Slug(), res.Kind), nil)
	}
}
