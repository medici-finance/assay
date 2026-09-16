package deskkit

// forgegit.go — the endpoint + credential a caller's OWN git transport (internal/gitcore's
// in-process List / FetchTagPayload) speaks to for a repo, read from the same custody ForgeFor
// reads. It exists because two production readers (cmd/desksupervise's live claim enumeration
// and loopengine's BranchMoved liveness probe) each listed refs ANONYMOUSLY against a
// hardcoded github.com — an anonymous listing 404s on every private repo ("authentication
// required: Repository not found"), which made the per-run stop / blocked-timeout /
// dead-claim-reclaim instrument could-not-check on every private board root (#1197). One
// builder, two callers, no third copy of the URL/username/custody dance.
//
// WHERE THE CREDENTIAL IS DIALED — the #1197 security review's blocker. The host in the URL is
// the one a custody credential (a GitHub App installation token, or the role's GitLab PAT) is
// presented to as an HTTP Basic password. It is derived from the RESOLVED FORGE KIND's OWN
// canonical instance — github.com for GitHub, the GITLAB_API_BASE host custody returns for
// GitLab — NEVER from the origin remote of the checkout the caller happens to sit in or was
// pointed at. The repo being listed is independent of that checkout (a desk in one checkout
// sweeping another repo's claims): its origin host names an UNRELATED repo's forge, and binding
// the credential to that host is how a GitHub App token reaches a GitLab host and vice versa
// (the #1206 crossing shape, via the host). The forge KIND likewise comes from the roster
// (ASSAY_REPO_FORGES), which maps slug→forge, not from that origin. Nothing here reads git.
//
// What it does NOT do: default a SaaS host for a forge whose instance is unknown (#727 — a
// GitLab repo with no configured instance is could-not-check, never gitlab.com), fall back to
// an ambient credential, or mint for GitLab.

import (
	"fmt"
	"net/url"
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
// (ASSAY_REPO_FORGES), obtains the role's credential from the SAME custody path ForgeFor uses
// (an installed GitHub custody minter or the desktoken mint-or-reuse; the provisioned 0600 PAT
// file for GitLab), and builds the HTTPS git URL on the RESOLVED KIND's canonical instance host
// (see the file header for why the host is never taken from a checkout's origin). The
// credential is paired with the forge's git-basic username (gitcore.GitHubGitUsername /
// gitcore.GitLabGitUsername).
//
// Fail-closed at every step, naming the repo: a slug that is not owner/name; a forge the roster
// does not name (could-not-check, never guessed from an unrelated origin); a role whose
// forge-qualified identity disagrees with the resolved forge; a credential custody cannot
// produce; a forge kind whose instance host is unknown (GitLab with no GITLAB_API_BASE); or a
// host that is not a bare hostname. Every one is an error the caller surfaces as could-not-check;
// none yields a usable ListOpts, and none presents a credential to a host derived from an
// untrusted or unrelated source.
func ForgeGitEndpointFor(repoSlug, role string) (ForgeGitEndpoint, error) {
	owner, name, ok := strings.Cut(strings.TrimSpace(repoSlug), "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return ForgeGitEndpoint{}, Unverifiable(fmt.Sprintf(
			"repo %q is not an owner/name slug — no git endpoint can be built for it", repoSlug), nil)
	}
	repo := ForgeRepo{Owner: owner, Name: name}

	// WHICH forge — from the roster ONLY (host "" ⇒ ASSAY_REPO_FORGES answers, else refuse).
	// The checkout's origin is deliberately NOT consulted: it names an unrelated repo's forge
	// in the cross-repo sweep, and a mis-resolution there would pick the wrong credential.
	res, err := resolveForgeKindWithHost(repo, "")
	if err != nil {
		return ForgeGitEndpoint{}, err
	}
	kind := res.Kind

	// Identity agreement (kept intact from the reviewer-approved surface): a role whose
	// forge-qualified roster entry names a different forge is refused before any credential read.
	if aerr := assertEntryForgeAgrees(role, kind); aerr != nil {
		return ForgeGitEndpoint{}, aerr
	}

	token, baseURL, cerr := custody(kind, role, repo)
	if cerr != nil {
		return ForgeGitEndpoint{}, cerr
	}

	host, herr := gitHostForKind(kind, baseURL, repo)
	if herr != nil {
		return ForgeGitEndpoint{}, herr
	}
	if verr := validateGitHost(host, repo); verr != nil {
		return ForgeGitEndpoint{}, verr
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

// gitHostForKind derives the git-over-HTTPS host for the resolved forge kind from the kind's
// OWN canonical instance — the custody base URL when custody names one, else the kind's SaaS
// host for GitHub only. It never reads a checkout's origin, so the credential is only ever
// presented to the forge that issued it.
//
//   - GitHub: the git host is the API base's host for a GitHub Enterprise install (custody's
//     minter supplies the base), else canonical github.com. api.github.com is the SaaS API
//     host; its git host is github.com, so it maps there rather than to the literal api host.
//   - GitLab: the git host is the GITLAB_API_BASE host custody returns. There is NO SaaS
//     default: defaulting an unconfigured instance to gitlab.com would present a self-hosted
//     adopter's PAT to the public host (the #727 crossing). Unconfigured is could-not-check.
func gitHostForKind(kind ForgeKind, custodyBaseURL string, repo ForgeRepo) (string, error) {
	switch kind {
	case ForgeGitHub:
		switch h := hostOfBaseURL(custodyBaseURL); {
		case h == "" || h == "github.com" || h == "api.github.com":
			return "github.com", nil
		default:
			// A GitHub Enterprise instance: its REST base is https://<host>/api/v3 and its git
			// host is <host>; api.github.com already returned github.com above.
			return strings.TrimPrefix(h, "api."), nil
		}
	case ForgeGitLab:
		h := hostOfBaseURL(custodyBaseURL)
		if h == "" {
			return "", Unverifiable(fmt.Sprintf(
				"cannot list %s: its forge resolves to GitLab but no instance host is configured "+
					"(GITLAB_API_BASE is unset) — refusing to default to gitlab.com and present the "+
					"role's PAT to a host the repo may not live on (#727). Set GITLAB_API_BASE to the "+
					"instance root.", repo.Slug()), nil)
		}
		return h, nil
	default:
		return "", Unverifiable(fmt.Sprintf(
			"no git host is known for forge %q serving %s", kind, repo.Slug()), nil)
	}
}

// hostOfBaseURL parses the host out of an API base URL (scheme://host[:port][/path]); it lowercases
// the result and returns "" for an empty base. A base with no scheme is returned verbatim
// (lowercased) so validateGitHost — not this parser — is the one gate that rejects a malformed
// value; url.Parse would silently swallow an scp-like `user@a@evil.test` into a clean hostname,
// which is exactly the authority-injection validateGitHost must still catch.
func hostOfBaseURL(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	if !strings.Contains(base, "://") {
		return strings.ToLower(base)
	}
	u, err := url.Parse(base)
	if err != nil || u.Hostname() == "" {
		return strings.ToLower(strings.TrimPrefix(base, "https://"))
	}
	return strings.ToLower(u.Hostname())
}

// validateGitHost rejects a host that is not a bare hostname — anything carrying a userinfo
// `@`, a path/scheme `/` or `\`, a port/scheme `:`, a query/fragment `?`/`#`, or whitespace —
// BEFORE it is interpolated into the URL authority. Without it an scp-like origin such as
// `git@a.example@evil.test:o/n` would yield authority `evil.test` and present the credential to
// it (#1197 security review S2). The kind-canonical derivation above already keeps an unrelated
// checkout's origin out of the host, so this is defense in depth on the remaining
// operator-configured input (GITLAB_API_BASE) and any future host source.
func validateGitHost(host string, repo ForgeRepo) error {
	if strings.TrimSpace(host) == "" {
		return Unverifiable(fmt.Sprintf("no git host resolved for %s", repo.Slug()), nil)
	}
	if strings.ContainsAny(host, "@/\\:?# \t\r\n") {
		return Unverifiable(fmt.Sprintf(
			"refusing to dial %q for %s: a git host must be a bare hostname, not a URL authority "+
				"(no @, /, \\, :, ?, # or whitespace) — a credential must never be presented to a host "+
				"parsed out of an untrusted or malformed source (#1197 security review S2)",
			host, repo.Slug()), nil)
	}
	return nil
}
