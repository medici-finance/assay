package deskkit

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/gitcore"
)

// CheckoutGitRead binds a configured fetch destination to this session's role.
// The returned URL is the exact gitcore RemoteURLs value, not a replacement
// clone URL. Auth rides gitcore's in-memory HTTP channel, never a Git environment.
func CheckoutGitRead(root, expectedRepo, role string) (gitcore.ListOpts, error) {
	bound, _, err := SessionTokenRole("git read")
	if err != nil {
		return gitcore.ListOpts{}, err
	}
	if role != bound {
		return gitcore.ListOpts{}, Refused("git read role differs from the session's bound token role")
	}
	repo, err := gitcore.Open(root)
	if err != nil {
		return gitcore.ListOpts{}, Unverifiable("cannot open git read checkout", err)
	}
	urls, err := repo.RemoteURLs("origin")
	if err != nil {
		return gitcore.ListOpts{}, Unverifiable("cannot read git origin", err)
	}
	if len(urls) != 1 {
		return gitcore.ListOpts{}, Refused("git read requires exactly one origin fetch URL")
	}
	u, err := url.Parse(urls[0])
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return gitcore.ListOpts{}, Refused("git read requires a plain HTTPS origin, without userinfo, query or fragment")
	}
	slug := strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
	if !ValidRepoSlug(slug) || (expectedRepo != "" && slug != expectedRepo) || !IsAllowedRepo(slug) {
		return gitcore.ListOpts{}, Refused("git origin does not name the expected configured repository")
	}
	owner, name, _ := strings.Cut(slug, "/")
	target := ForgeRepo{Owner: owner, Name: name}
	res, err := ForgeKindForRepoRemote(target, urls[0])
	if err != nil {
		return gitcore.ListOpts{}, err
	}
	if err := assertEntryForgeAgrees(role, res.Kind); err != nil {
		return gitcore.ListOpts{}, err
	}
	base := ""
	if res.Kind == ForgeGitLab {
		base = gitlabAPIBaseOverride()
	}
	host, err := gitHostForKind(res.Kind, base, target)
	if err != nil {
		return gitcore.ListOpts{}, err
	}
	if err := validateGitHost(host, target); err != nil {
		return gitcore.ListOpts{}, err
	}
	expectedPort := "443"
	if res.Kind == ForgeGitLab {
		api, e := url.Parse(base)
		if e != nil || api.Scheme != "https" || api.User != nil {
			return gitcore.ListOpts{}, Refused("GitLab git read requires an HTTPS custody instance")
		}
		if api.Port() != "" {
			expectedPort = api.Port()
		}
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	if port != expectedPort {
		return gitcore.ListOpts{}, Refused("git read origin port differs from the credential's forge port")
	}
	if !strings.EqualFold(u.Hostname(), host) {
		return gitcore.ListOpts{}, Refused(fmt.Sprintf("git read origin host differs from the credential's forge host for %s", slug))
	}
	cred, err := ResolveRoleCredential(role, target, urls[0])
	if err != nil {
		return gitcore.ListOpts{}, err
	}
	user := gitcore.GitHubGitUsername
	if res.Kind == ForgeGitLab {
		user = gitcore.GitLabGitUsername
	}
	return gitcore.ListOpts{URL: urls[0], Auth: gitcore.BasicAuthAs(user, cred.Token)}, nil
}
