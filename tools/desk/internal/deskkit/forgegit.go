package deskkit

// forgegit.go — the endpoint + credential a caller's OWN git transport (internal/gitcore's
// in-process List / FetchTagPayload) speaks to for a repo, resolved the same way
// cmd/deskclaim-ref's newForgeStore resolves them and read from the same custody ForgeFor
// reads. It exists because two production readers (cmd/desksupervise's live claim
// enumeration and loopengine's BranchMoved liveness probe) each listed refs ANONYMOUSLY
// against a hardcoded github.com — an anonymous listing 404s on every private repo
// ("authentication required: Repository not found"), which made the per-run stop /
// blocked-timeout / dead-claim-reclaim instrument could-not-check on every private board
// root (#1197). One builder, two callers, no third copy of the URL/username/custody dance.
//
// What it does NOT do: read git for the origin host (the caller supplies it, exactly as
// ForgeKindFromSlugAndHost demands — see OriginRemoteHost for the reader), default a SaaS
// host when none is known (#727), fall back to an ambient credential, or mint for GitLab.

import (
	"fmt"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// ForgeGitEndpoint is what a gitcore caller needs to list or fetch from repo as role: the
// resolved forge kind, the instance host the URL was built from, and the ListOpts (URL +
// git-basic Auth) scoped to that one call. Auth is never nil on a successful return — a
// caller that could not obtain a credential gets an error, never an anonymous endpoint.
type ForgeGitEndpoint struct {
	Kind ForgeKind
	Host string
	Opts gitcore.ListOpts
}

// ForgeGitEndpointFor resolves the forge serving repoSlug ("owner/name") from the roster
// (ASSAY_REPO_FORGES) or originHost (the well-known host table), builds the HTTPS git URL on
// originHost, and pairs the role's already-minted/provisioned token with the forge's
// git-basic username (gitcore.GitHubGitUsername / gitcore.GitLabGitUsername) — the shape
// cmd/deskclaim-ref's newForgeStore uses, with custody read through the SAME path ForgeFor
// uses (an installed GitHub custody minter or the desktoken mint-or-reuse; the provisioned
// 0600 PAT file for GitLab).
//
// Fail-closed at every step, naming the repo: a slug that is not owner/name, a forge the
// resolver cannot name, an EMPTY originHost (never a SaaS default — the roster names the
// forge software, not the instance, #727), a role whose forge-qualified identity disagrees
// with the resolved forge, or a credential custody cannot produce. Every one is an error
// the caller surfaces as could-not-check; none yields a usable ListOpts.
func ForgeGitEndpointFor(repoSlug, role, originHost string) (ForgeGitEndpoint, error) {
	owner, name, ok := strings.Cut(strings.TrimSpace(repoSlug), "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return ForgeGitEndpoint{}, Unverifiable(fmt.Sprintf(
			"repo %q is not an owner/name slug — no git endpoint can be built for it", repoSlug), nil)
	}
	repo := ForgeRepo{Owner: owner, Name: name}
	kind, host, err := ForgeKindFromSlugAndHost(repo.Slug(), originHost)
	if err != nil {
		return ForgeGitEndpoint{}, err
	}
	if host == "" {
		// ForgeKindFromSlugAndHost refuses an empty host itself; this is the belt to that
		// brace, so a future change to the resolver cannot hand this builder a hostless answer.
		return ForgeGitEndpoint{}, Unverifiable(fmt.Sprintf(
			"resolved forge %q for %s but no instance host is known — refusing to default one", kind, repo.Slug()), nil)
	}
	if aerr := assertEntryForgeAgrees(role, kind); aerr != nil {
		return ForgeGitEndpoint{}, aerr
	}
	token, _, cerr := custody(kind, role, repo)
	if cerr != nil {
		return ForgeGitEndpoint{}, cerr
	}
	username := gitcore.GitHubGitUsername
	if kind == ForgeGitLab {
		username = gitcore.GitLabGitUsername
	}
	return ForgeGitEndpoint{
		Kind: kind,
		Host: host,
		Opts: gitcore.ListOpts{
			URL:  "https://" + host + "/" + owner + "/" + name + ".git",
			Auth: gitcore.BasicAuthAs(username, token),
		},
	}, nil
}

// OriginRemoteHost reads the host of dir's `origin` remote, for a caller that hands it to
// ForgeGitEndpointFor / ForgeKindFromSlugAndHost. It reads in-process first (gitcore.Open,
// which resolves a linked worktree's shared config through its common dir) and falls back
// to the git binary (`git -C dir remote get-url origin` — git, never a forge CLI, and no
// network) for a checkout go-git cannot open, the same two-step cmd/deskclaim-ref's
// originRemoteURL takes. An unreadable origin is an error the caller treats as "no host
// known": the resolver then refuses rather than guessing an instance (#727).
func OriginRemoteHost(dir string) (string, error) {
	if repo, err := gitcore.Open(dir); err == nil {
		if raw, rerr := repo.RemoteURL("origin"); rerr == nil {
			return hostOfRemote(raw)
		}
	}
	out, err := gitOut(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("cannot read the origin remote of %s: %w", dir, err)
	}
	return hostOfRemote(strings.TrimSpace(out))
}
