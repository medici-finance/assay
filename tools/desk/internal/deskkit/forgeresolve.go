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

// HostOfRemote is the exported wrapper over hostOfRemote: it extracts the host from a git
// remote URL (scheme://host/... or the scp-like user@host:path), lower-cased, defending
// against the scp-like `user@a@evil.test` shape url.Parse would silently swallow. A caller
// outside this package that must scope a per-host config key to the origin's host uses this
// rather than re-deriving the parse (cmd/deskwt's role credential helper, #1309 item 7 fix).
func HostOfRemote(raw string) (string, error) { return hostOfRemote(raw) }

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

// resolveForgeKind implements resolution steps a/b/c of the contract above, reading the
// origin host from the CURRENT WORKING DIRECTORY's checkout (originRemoteHost). A caller
// that has the target checkout somewhere OTHER than its CWD — deskdispatch resolves the
// forge of a repo it is not sitting in — supplies the host itself via
// resolveForgeKindWithHost / ForgeKindForRepoRemote rather than routing through here.
func resolveForgeKind(repo ForgeRepo) (ForgeResolution, error) {
	key := strings.ToLower(strings.TrimSpace(repo.Slug()))
	if kind, ok := EffectiveConfig().RepoForges[key]; ok {
		// The roster answered — the remote-host fallback must NOT even be consulted
		// (TestForgeForResolvesFromRepoConfig proves it by panicking if originRemoteHost is
		// reached), so return here rather than reading git on a question already answered.
		return ForgeResolution{Repo: repo, Kind: ForgeKind(kind), Source: "repo-config:" + EnvRepoForges}, nil
	}
	host, herr := originRemoteHost()
	if herr != nil {
		host = ""
	}
	return resolveForgeKindWithHost(repo, host)
}

// resolveForgeKindWithHost is resolveForgeKind's core with the origin host supplied by the
// caller rather than read from the CWD checkout. It answers WHICH FORGE SOFTWARE — roster
// first (ASSAY_REPO_FORGES), then, only when the roster is silent, the given host mapped
// through the unambiguous well-known table. host may be "" (only the roster can then
// answer). It carries NO instance-host requirement — unlike ForgeKindFromSlugAndHost, which
// must also decide WHERE the instance lives — because "which forge?" is all a caller
// picking a forge-shaped literal needs. Unresolvable is Unverifiable (could-not-check),
// naming the repo and the configuration that would resolve it: never a default, never a guess.
func resolveForgeKindWithHost(repo ForgeRepo, host string) (ForgeResolution, error) {
	key := strings.ToLower(strings.TrimSpace(repo.Slug()))
	cfg := EffectiveConfig()
	if kind, ok := cfg.RepoForges[key]; ok {
		return ForgeResolution{Repo: repo, Kind: ForgeKind(kind), Source: "repo-config:" + EnvRepoForges}, nil
	}
	if h := strings.ToLower(strings.TrimSpace(host)); h != "" {
		if kind, ok := wellKnownForgeHosts[h]; ok {
			return ForgeResolution{Repo: repo, Kind: kind, Source: "remote-host:" + h}, nil
		}
	}
	return ForgeResolution{}, Unverifiable(fmt.Sprintf(
		"cannot resolve which forge serves %s: no %s entry names it, and the origin remote's host is "+
			"absent, unreadable, or does not map unambiguously to a known forge (%s). Configure "+
			"%s=%s=github (or =gitlab) in the roster — never a flag or environment variable, which this "+
			"resolver deliberately does not read.",
		repo.Slug(), EnvRepoForges, knownForgeHostsList(), EnvRepoForges, repo.Slug()), nil)
}

// ForgeKindForRepoRemote resolves WHICH forge serves repo — the kind and provenance — from
// the roster first (ASSAY_REPO_FORGES) and, only when the roster is silent, the host parsed
// from the given origin remote URL, mapped through the unambiguous well-known host table. It
// constructs NO backend, reads NO token, and shells to NO git: the origin URL is supplied by
// the caller, which read it from the target checkout it already has in hand (deskdispatch
// resolves the forge of the repo its --root names, not the one it is sitting in). An empty or
// unparseable URL is not itself an error here — only the roster can then answer — so a
// roster-configured repo resolves with no readable remote at all.
//
// This is the "which forge?" read a caller makes to pick a forge-shaped literal, the review
// kit's head-fetch refspec being the case it exists for (GitHub refs/pull/<N>/head vs GitLab
// refs/merge-requests/<iid>/head, #773). It answers which forge SOFTWARE, not which instance,
// so it does not carry ForgeKindFromSlugAndHost's instance-host requirement. Unresolvable is
// Unverifiable (could-not-check), naming the repo and the configuration that would resolve it.
func ForgeKindForRepoRemote(repo ForgeRepo, originRemoteURL string) (ForgeResolution, error) {
	host := ""
	if h, err := hostOfRemote(originRemoteURL); err == nil {
		host = h
	}
	return resolveForgeKindWithHost(repo, host)
}

// ForgeKindFromSlugAndHost resolves WHICH forge serves a repo WITHOUT reading git, minting a
// token, or constructing a backend. It is the token-free variant a caller that speaks its own
// git transport needs (cmd/deskclaim-ref, which pushes/reads dispatch-claim refs in-process via
// gitcore and never mints an App token): it must decide github-vs-gitlab to pick the git-basic
// username, but must NOT drag in the custody/mint machinery ResolveForge/ForgeFor carry.
//
// Resolution mirrors resolveForgeKind's steps a/b, with the origin host supplied by the CALLER
// (read in-process from its own checkout) rather than shelled from git here:
//
//	a. the repo's configured forge — ASSAY_REPO_FORGES in the roster;
//	b. failing that, the given host mapped through the unambiguous well-known table
//	   (github.com/gitlab.com); host may be "" (only step a can then answer);
//	c. failing that, Unverifiable — never a default, never a guess.
//
// It returns the resolved forge kind AND the git host to speak to: the caller's origin host,
// which the caller read in-process from its own checkout. The ForgeKind is a RETURN, never a
// parameter — resolution is never a caller's choice (TestForgeForRejectsCallerSuppliedForge).
//
// When the caller supplies NO host, this NEVER guesses one. A roster entry (or the well-known
// host table) answers WHICH FORGE SOFTWARE, never WHICH INSTANCE — GitLab and GitHub are both
// self-hosted-capable, so "gitlab" is not "gitlab.com" and "github" is not "github.com".
// Defaulting the host to the canonical SaaS instance is how a self-hosted adopter's credential
// ends up presented to a public host, with the auth failure swallowed downstream as a bare
// "unverifiable" (#727). An empty host is therefore could-not-check (Unverifiable, exit 6),
// naming the repo — never a SaaS default. The caller's job is to hand a readable origin host
// (see cmd/deskclaim-ref's originRemoteURL, which reads it through something that understands
// the worktreeConfig extension go-git does not).
func ForgeKindFromSlugAndHost(slug, host string) (ForgeKind, string, error) {
	key := strings.ToLower(strings.TrimSpace(slug))
	h := strings.ToLower(strings.TrimSpace(host))
	resolve := func(kind ForgeKind) (ForgeKind, string, error) {
		if h != "" {
			return kind, h, nil
		}
		// No origin host to go on. Refuse rather than default to the canonical SaaS host: the
		// roster named the forge SOFTWARE, not the INSTANCE, so any host we substitute here is a
		// guess about WHERE — and on a self-hosted adopter that guess silently points the
		// adopter's credential at gitlab.com/github.com (#727). Fail closed, naming the repo
		// and the forge so an operator can see the resolution stopped for lack of a host.
		return "", "", Unverifiable(fmt.Sprintf(
			"resolved forge %q for %s, but no instance host is known: the origin remote was unreadable "+
				"(e.g. go-git cannot read a worktreeConfig checkout) and %s names the forge software, not "+
				"the instance. Make the origin remote readable, or supply the host — never a SaaS default.",
			kind, slug, EnvRepoForges), nil)
	}
	cfg := EffectiveConfig()
	if kind, ok := cfg.RepoForges[key]; ok {
		return resolve(ForgeKind(kind))
	}
	if h != "" {
		if kind, ok := wellKnownForgeHosts[h]; ok {
			return resolve(kind)
		}
	}
	return "", "", Unverifiable(fmt.Sprintf(
		"cannot resolve which forge serves %s: no %s entry names it, and the origin host %q does not map "+
			"unambiguously to a known forge (%s). Configure %s=%s=github (or =gitlab) in the roster.",
		slug, EnvRepoForges, host, knownForgeHostsList(), EnvRepoForges, slug), nil)
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
//
// custody RESOLVES NOTHING: it trusts the kind it is handed. So it is a confined link of the
// GitHub App mint chain (roletokenguard_test.go), reachable only from the two functions that
// resolve the forge and check it against the role's roster entry before calling it —
// ResolveForge and ForgeGitEndpointFor. A new caller has to argue its way onto that list.
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

// githubCustody is custody's GitHub branch. It resolves no forge either — it is an unexported
// pass-through into githubAppRoleToken — so the class guard confines it to its one caller,
// custody, whose own callers are confined in turn.
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
	tok, path, rerr := githubAppRoleToken(role, repo.Slug())
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
	tok, _, err := gitlabRoleTokenFile(role)
	if err != nil {
		return "", "", err
	}
	return tok, gitlabAPIBaseOverride(), nil
}

// gitlabRoleTokenFile resolves the role's already-provisioned GitLab PAT custody file: it
// returns the token value AND the 0600 file path it was read from, having verified the file's
// mode. It is the shared core behind gitlabCustody (which pairs the token with the API base a
// GitLabForge dials) and the exported GitLabRoleToken (which a caller that hands the raw token
// to a git-basic-auth child — not a GitLabForge — needs the PATH for). ForgeFor never rotates
// or mints a GitLab credential; both callers only READ an already-provisioned custody file.
func gitlabRoleTokenFile(role string) (token, path string, err error) {
	if strings.TrimSpace(role) == "" {
		return "", "", Refused("no App role named for the GitLab custody lookup — a token cannot be " +
			"obtained for an identity this process cannot name")
	}
	name := gitlabTokenFileName(role)
	p, searched, found := FindConfigFile(name)
	if !found {
		return "", "", Refused(fmt.Sprintf(
			"gitlab token file not found: no %s on the App-credential search path. Searched: %s. "+
				"Provision the role's PAT there (0600) via a group owner, or set %s to the directory "+
				"that has it. ForgeFor never rotates or mints a GitLab credential on your behalf — it "+
				"only reads an already-provisioned custody file.",
			name, strings.Join(searched, ", "), EnvConfigHome))
	}
	if verr := verifyCustodyFileMode(p); verr != nil {
		return "", "", verr
	}
	b, rerr := os.ReadFile(p)
	if rerr != nil {
		return "", "", Refused(fmt.Sprintf("cannot read gitlab token file at %s: %v", p, rerr))
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", "", Refused(fmt.Sprintf("the gitlab token file at %s is empty", p))
	}
	return tok, p, nil
}

// GitLabRoleToken returns the role's already-provisioned GitLab PAT and the custody file path
// it was read from. It is the GitLab twin of the GitHub App minter (RoleTokenForRepo) for the
// callers that hand the raw token to a NON-GitLabForge child that speaks its own transport —
// deskdispatch's claim child (issue 1203), which authenticates git-over-HTTPS to the dispatch
// refs and, on a GitLab-served repo, must run under the same role PAT custody deskpost/deskflip
// use rather than a GitHub App installation token there is no App to mint. It reads only an
// already-provisioned custody file (0600 on the App-credential search path); it never mints or
// rotates. A missing/loose/empty file is Refused (exit 5) — a precondition an operator fixes.
func GitLabRoleToken(role string) (token, path string, err error) {
	return gitlabRoleTokenFile(role)
}

// --- Role credential by forge: the raw-token / token-PATH read ---------------------------
//
// ForgeFor hands a backend it constructs its credential. Some callers need the credential
// ITSELF instead — a git credential helper that reads a token FILE (deskwt role-init), a
// child process handed --token-file (deskdispatch's claim child), a credential helper (deskgit --as),
// a poller (scanloop). Before #1573 each of those reached the GitHub App minter
// (RoleTokenForOwner / RoleTokenForRepo) directly. Those names are forge-neutral but the
// minter is GitHub-only, so a caller that picked it up on a GitLab-served repo asked for a
// GitHub App that does not exist — #676 at deskboot, then #1573 at role-init, each fixed at
// its own call site. The entry points below are the forge-aware answer, and
// githubAppRoleToken is the ONE function in this package that reaches the GitHub minter.
// TestGitHubMinterReachedOnlyFromForgeArms (roletokenguard_test.go) walks tools/desk and
// fails on any other reference outside its reviewed allow-list, so the next caller cannot
// repeat the mistake quietly.

// unresolvedRoleCredentialSource is the provenance recorded when neither the roster nor the
// origin host names the forge. With NO origin URL in hand, such a repo takes the GitHub App
// path: that is the historical behaviour of the roster-only callers (the GitHubCustodyMinter
// hooks, the GitHub-API pollers), so a GitHub adopter with no forge configuration is
// unaffected. With an origin URL in hand it is REFUSED instead (githubAppOriginHost). It is a
// DEFAULT only for the credential read — ForgeFor itself still refuses an unresolved forge.
const unresolvedRoleCredentialSource = "unresolved: no " + EnvRepoForges + " entry or known origin host; the GitHub App path"

// githubAppHost is the ONE host a GitHub App installation token authenticates to. The minter
// (desktoken) mints against GitHubAPIBase, so the token is a github.com credential and nothing
// else: a self-hosted GitHub or GitLab instance cannot accept it, and any other host that is
// offered it has simply been handed a live github.com secret.
const githubAppHost = "github.com"

// githubAppOriginHost is the host binding on the GitHub App arm. A caller that holds the
// target's origin URL is about to hand the token to a transport bound to THAT origin's host —
// role-init scopes its credential helper to it, `deskgit --as` hands it to its helper — so
// the arm is taken only when the parsed origin host is EXACTLY githubAppHost: lower-cased by
// the parser, and nothing else forgiven (no suffix or prefix match, no trailing dot, no
// userinfo trick — url.Hostname already discards the userinfo and the port, and a port does
// not change which host receives the credential). An origin that does not parse to a host is
// refused the same way. This holds whether the roster named the forge "github" (the roster
// names forge SOFTWARE, never the instance) or nothing resolved at all.
//
// An EMPTY originURL is a roster-only caller whose transport is not bound to an origin (a
// GitHubCustodyMinter hook talks to GitHubAPIBase itself); it keeps the historical path.
func githubAppOriginHost(res ForgeResolution, originURL string) error {
	raw := strings.TrimSpace(originURL)
	if raw == "" {
		return nil
	}
	host, err := hostOfRemote(raw)
	if err == nil && host == githubAppHost {
		return nil
	}
	// Never quote the raw URL: an https origin can carry userinfo credentials. The parsed host
	// is safe to name; an unparseable origin is named only as such.
	origin := "an origin remote that does not parse to a host"
	if err == nil {
		origin = fmt.Sprintf("origin host %q", host)
	}
	return Refused(fmt.Sprintf(
		"refused: the GitHub App installation token for %s authenticates only to %s, but the target "+
			"checkout has %s (forge %s, %s) — no token is minted or read, and none is offered to that "+
			"host. A GitLab-served repo is mapped in the roster (%s=%s=gitlab); a GitHub-served repo "+
			"needs an origin on %s.",
		res.Repo.Slug(), githubAppHost, origin, res.Kind, res.Source, EnvRepoForges, res.Repo.Slug(),
		githubAppHost))
}

// RoleCredential is a role's credential as selected by the forge serving a repo. Token is a
// secret: a caller hands it to its transport and never logs it. Path is the custody file it
// was read from — the thing a credential helper or a --token-file hand-off carries instead of
// the value.
type RoleCredential struct {
	Resolution ForgeResolution
	Token      string
	Path       string
}

// roleCredentialForge answers which forge's custody serves repo: ASSAY_REPO_FORGES first, then
// the host of originURL through the unambiguous well-known table (ForgeKindForRepoRemote). An
// empty originURL consults the roster only — no git read, no process fork, so a caller on a
// hot path (a GitHubCustodyMinter hook reached once per repo per read) pays a map lookup.
//
// On the GitHub arm (resolved or unresolved) it also applies githubAppOriginHost, so BOTH entry
// points below refuse a non-github.com origin from this one place, before either can reach the
// minter. The error is that refusal; the resolution is returned alongside it for the caller's
// record.
func roleCredentialForge(repo ForgeRepo, originURL string) (ForgeResolution, error) {
	res, err := ForgeKindForRepoRemote(repo, originURL)
	if err != nil {
		res = ForgeResolution{Repo: repo, Kind: ForgeGitHub, Source: unresolvedRoleCredentialSource}
	}
	if res.Kind == ForgeGitHub {
		if herr := githubAppOriginHost(res, originURL); herr != nil {
			return res, herr
		}
	}
	return res, nil
}

// ResolveRoleCredential selects the role's credential by the forge that serves repo (#1573),
// resolving the forge BEFORE any token is touched:
//
//   - GitLab: the role's already-provisioned `gitlab-<role>.token` custody file, read through
//     GitLabRoleToken. Never minted, never rotated, no App ID required. A missing, empty or
//     loosely-permissioned file is Refused (exit 5) naming it — it never falls through to the
//     GitHub App minter or to any ambient credential.
//   - GitHub: the GitHub App installation token, minted or reused (githubAppRoleToken) — but
//     only when originURL is "" or its host is exactly github.com (githubAppOriginHost). A
//     forge neither the roster nor originURL resolves takes this arm only with no originURL;
//     with one, it is Refused (exit 5) naming ASSAY_REPO_FORGES.
//
// originURL is the TARGET checkout's origin remote when the caller has one (it may be "":
// the roster alone then answers). The returned Resolution records which forge answered and
// how, so a caller can say so without resolving twice.
func ResolveRoleCredential(role string, repo ForgeRepo, originURL string) (RoleCredential, error) {
	res, err := roleCredentialForge(repo, originURL)
	cred := RoleCredential{Resolution: res}
	if err != nil {
		return cred, err
	}
	switch res.Kind {
	case ForgeGitLab:
		cred.Token, cred.Path, err = GitLabRoleToken(role)
	case ForgeGitHub:
		cred.Token, cred.Path, err = githubAppRoleToken(role, repo.Slug())
	default:
		err = Unverifiable(fmt.Sprintf("no role credential custody known for forge %q serving %s (%s)",
			res.Kind, repo.Slug(), res.Source), nil)
	}
	return cred, err
}

// GitHubRoleToken is the GitHub App installation token for role on repo (an "owner/name"
// slug, or a bare owner), with RoleTokenForRepo's shape — for a caller whose TRANSPORT only
// speaks GitHub's API: a GitHubCustodyMinter hook, a GitHub-API poller. It resolves the forge
// from the roster and REFUSES (exit 5) when repo is configured to another forge, instead of
// minting a GitHub App token for a repo GitHub does not serve — or handing that forge's
// credential to a GitHub transport. With no origin URL in hand, an unresolved forge takes the
// GitHub path (unresolvedRoleCredentialSource). A caller whose transport is bound to an origin
// remote's host uses GitHubRoleTokenForRemote.
func GitHubRoleToken(role, repo string) (token, path string, err error) {
	return GitHubRoleTokenForRemote(role, repo, "")
}

// GitHubRoleTokenForRemote is GitHubRoleToken for a caller that also holds the target's origin
// remote URL — a host-scoped `x-access-token` credential helper — so the token is
// handed back only when that origin's host is exactly github.com (githubAppOriginHost): a repo
// the roster is silent on whose origin maps to another forge, or to no known forge, or does not
// parse, is refused before any mint, and so is a roster "github" entry on another host.
func GitHubRoleTokenForRemote(role, repo, originURL string) (token, path string, err error) {
	if err := githubTransportArm(repo, originURL); err != nil {
		return "", "", err
	}
	return githubAppRoleToken(role, repo)
}

// GitHubRoleTokenForDestinations is GitHubRoleTokenForRemote for a transport that CONNECTS to a
// list of URLs rather than to one origin: `git push origin` sends to every remote.origin.pushurl
// value (or every url value, pushInsteadOf applied), and every URL git uses is rewritten by
// url.<base>.insteadOf from every config scope. The caller hands in exactly the URLs git itself
// resolved (`git remote get-url [--push] --all origin`), and EVERY one of them must pass the
// same binding GitHubRoleTokenForRemote applies to its one origin — resolved to GitHub, host
// exactly github.com — before anything is minted or read. The token is minted once, after the
// whole list has passed. Two further refusals are specific to a list of connection targets:
//
//   - an empty list, or an empty entry, is refused: GitHubRoleTokenForRemote reads an empty
//     origin as a roster-only caller whose transport binds no host, but a transport that is
//     about to connect somewhere has no such case — an unknown destination is not a pass;
//   - a cleartext `http://` destination is refused even on github.com: a credential answered to
//     a 401 challenge over cleartext reaches anyone on the path. (deskgit's helper also declines
//     every non-https request; this refusal is the layer before it.)
func GitHubRoleTokenForDestinations(role, repo string, destinations []string) (token, path string, err error) {
	if len(destinations) == 0 {
		return "", "", Refused(fmt.Sprintf(
			"refused: no transport destination was resolved for %s — no GitHub App token is minted for a "+
				"connection whose target is unknown", repo))
	}
	for i, dest := range destinations {
		d := strings.TrimSpace(dest)
		if d == "" {
			return "", "", Refused(fmt.Sprintf(
				"refused: transport destination %d of %d for %s is empty — no GitHub App token is minted for a "+
					"connection whose target is unknown", i+1, len(destinations), repo))
		}
		if strings.HasPrefix(strings.ToLower(d), "http://") {
			return "", "", Refused(fmt.Sprintf(
				"refused: transport destination %d of %d for %s is cleartext http:// — the GitHub App token is "+
					"offered only over an authenticated channel to %s (use https://)", i+1, len(destinations), repo,
				githubAppHost))
		}
		if err := githubTransportArm(repo, d); err != nil {
			return "", "", err
		}
	}
	return githubAppRoleToken(role, repo)
}

// githubTransportArm answers whether a GitHub-only transport bound to originURL may carry the
// GitHub App token for repo: the forge resolves to GitHub and roleCredentialForge's host binding
// holds. It is the one check both GitHub transport entry points apply.
func githubTransportArm(repo, originURL string) error {
	owner, name, _ := strings.Cut(strings.TrimSpace(repo), "/")
	res, rerr := roleCredentialForge(ForgeRepo{Owner: owner, Name: name}, originURL)
	if rerr != nil {
		return rerr
	}
	if res.Kind != ForgeGitHub {
		return Refused(fmt.Sprintf(
			"refused: %s is served by %s (%s), not GitHub — no GitHub App token is minted for it, and this "+
				"caller's transport speaks only GitHub. Resolve the credential with the forge-aware resolver "+
				"(deskkit.ResolveRoleCredential) instead.", repo, res.Kind, res.Source))
	}
	return nil
}

// githubAppRoleToken is the GitHub arm: the ONE place in this package that reaches the GitHub
// App minter. Its callers are CONFINED, not just documented: the class guard
// (roletokenguard_test.go) treats this name as a minter too and allow-lists exactly four
// callers. Three have resolved the forge to GitHub and bound the host before calling it —
// ResolveRoleCredential, GitHubRoleTokenForRemote and GitHubRoleTokenForDestinations. The
// fourth, githubCustody, resolves nothing itself: it is custody's GitHub branch, and custody is
// in turn reached only from the two functions that resolve the forge and check it against the
// roster before calling it (ResolveForge, ForgeGitEndpointFor). The guard confines custody and
// githubCustody too, so that chain cannot be entered from anywhere else. Any other caller, in
// this package or through an alias, fails the guard.
func githubAppRoleToken(role, repo string) (token, path string, err error) {
	return RoleTokenForRepo(role, repo)
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

// verifyCustodyFileMode enforces that a custody file is a regular file locked to its
// owner — the same rule cmd/desktoken/gitlab.go's rotation path already enforces before
// it will rotate a PAT, applied here uniformly to every file-backed custody read. The
// owner-only test is behind the OS boundary (VerifyCustodyOwnerOnly): a POSIX 0600 test
// on unix, and the owner-only NTFS ACL evaluation on Windows, where os.FileMode's
// permission bits are synthetic (#667). A missing, non-regular, or loosely-permissioned
// file is Refused (exit 5): the deployment has not provisioned this correctly, which is a
// precondition an operator fixes, not a could-not-check.
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
	if err := VerifyCustodyOwnerOnly(path, fi); err != nil {
		return Refused(err.Error())
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
	f, _, err := ResolveForge(repo, role)
	return f, err
}

// ResolveForge is ForgeFor plus the PROVENANCE of the answer. A caller that has to REPORT
// which forge it acted against — a refusal naming the forge, an audit line, a could-not-check
// that says which backend could not serve an operation — needs the ForgeResolution, and
// re-deriving it by calling resolveForgeKind a second time would let the two answers drift
// (the roster is read at call time, so a second read is a second question).
//
// It is the same function as ForgeFor in every other respect, including that there is no
// parameter, flag or environment variable by which a caller supplies the forge.
func ResolveForge(repo ForgeRepo, role string) (Forge, ForgeResolution, error) {
	res, err := resolveForgeKind(repo)
	if err != nil {
		return nil, ForgeResolution{}, err
	}
	// Forge agreement is ENFORCED, not assumed: a role whose EXPLICITLY forge-qualified
	// roster entry names a forge other than the one this repo resolves to is refused here,
	// BEFORE any credential is read — a mismatched entry must never be handed a backend
	// (the forge-qualified-identity brief, assertEntryForgeAgrees). An inferred-github entry is exempt (the
	// human-gated backward-compatibility rule).
	if aerr := assertEntryForgeAgrees(role, res.Kind); aerr != nil {
		return nil, ForgeResolution{}, aerr
	}
	tok, base, cerr := custody(res.Kind, role, repo)
	if cerr != nil {
		return nil, ForgeResolution{}, cerr
	}
	switch res.Kind {
	case ForgeGitHub:
		return &GitHubForge{Token: tok, BaseURL: base}, res, nil
	case ForgeGitLab:
		return &GitLabForge{Token: tok, BaseURL: base}, res, nil
	default:
		// Unreachable given resolveForgeKind only ever returns a value from
		// wellKnownForgeHosts or cfg.RepoForges (both constrained to the two known kinds
		// at parse/lookup time) — kept as Unverifiable, never a panic, because a resolver
		// that cannot name a backend must fail closed even on a case it believes is
		// impossible.
		return nil, ForgeResolution{}, Unverifiable(fmt.Sprintf(
			"forge resolution for %s returned an unrecognised kind %q", repo.Slug(), res.Kind), nil)
	}
}

// ForgeKindFor resolves WHICH forge serves repo — the kind and the provenance of the
// answer — WITHOUT obtaining a credential or constructing a backend. It is the read a
// caller makes when it needs to KNOW the forge (to branch behaviour, or to refuse a
// forge it cannot serve) but is NOT about to act through the backend, so it must not pay
// the custody cost ResolveForge does.
//
// The distinction is load-bearing for a caller that files under an AMBIENT credential and
// mints no token of its own — deskfile is the case this exists for: routing it through
// ResolveForge/ForgeFor would drag in App-token custody and change WHO the filing is
// authored as, which deskfile deliberately never does. ForgeKindFor answers "which forge?"
// with the SAME resolution ResolveForge uses (steps a/b/c of the forgeresolve contract at
// the head of this file) and stops there — it reads the roster/remote, never a token file.
//
// The refusal shape is identical too: an unresolvable forge is the same Unverifiable
// (could-not-check), naming the repo and the configuration that would resolve it, so a
// caller can tell "known to be some other forge" (a definite ForgeResolution it can refuse
// on) from "could not determine the forge at all" (the error — which such a caller reads as
// could-not-check, never as a licence to assume one).
func ForgeKindFor(repo ForgeRepo) (ForgeResolution, error) {
	return resolveForgeKind(repo)
}

// ReadyFlip performs the draft→ready transition on an ALREADY-READ change, and is the one
// place the flip's node-id rule is enforced.
//
// THE RULE: the opaque id comes from the change the backend itself returned, and from
// nowhere else. MarkReadyForReview takes an id whose ENCODING is the backend's own — a
// GraphQL global id on GitHub, a `gitlab:<owner>/<name>!<iid>` coordinate on GitLab — and a
// caller that composed one by string-building would be constructing a value whose format the
// forge, not the caller, defines. That is not a style preference: the GitLab encoding names a
// PROJECT and an IID, so a locally built id addresses whatever project the builder guessed at,
// and it would keep working on the happy path (where the guess matches) right up until it did
// not. Taking *PullRequest rather than a string is what makes the rule checkable by the
// compiler: the only way to obtain one is to have read the change.
//
// A change carrying NO opaque id is a backend that cannot serve this operation for it. That
// is could-not-check (exit 6), naming the forge and the operation, and NOTHING is written —
// not a fallback to some other transition, not a raw request, not a guess at an id.
func ReadyFlip(res ForgeResolution, f Forge, pr *PullRequest) error {
	if f == nil {
		return Unverifiable("refusing to flip a change ready with no resolved forge backend", nil)
	}
	if pr == nil {
		return Unverifiable(fmt.Sprintf(
			"could-not-check: the %s backend was asked to perform MarkReadyForReview without a change to "+
				"perform it on — the opaque id is read from the change, never composed", res.Kind), nil)
	}
	if strings.TrimSpace(pr.NodeID) == "" {
		return Unverifiable(fmt.Sprintf(
			"could-not-check: the resolved %s backend serves no opaque change id for %s#%d, so the "+
				"MarkReadyForReview operation cannot be performed against it. The id is READ from the change "+
				"(GetPullRequest), never composed locally, so there is nothing to fall back to — and nothing "+
				"was written.", res.Kind, res.Repo.Slug(), pr.Number), nil)
	}
	return f.MarkReadyForReview(pr.NodeID)
}
