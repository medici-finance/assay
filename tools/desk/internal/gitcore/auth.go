package gitcore

import (
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// appUsername is the literal GitHub App username go-git's BasicAuth carries when the
// password is an App installation token. GitHub's own git-over-HTTPS convention
// accepts any non-empty username paired with a token password; "x-access-token" is
// the literal this house's other in-process REST callers (desktoken, deskpost,
// deskevidence, deskrelease) already use, so gitcore matches it rather than
// introducing a second convention.
const appUsername = "x-access-token"

// BasicAuth builds the in-process HTTP credential for a GitHub App installation
// token, handed directly to go-git's transport as a Go value.
//
// The token flows: desktoken -> this Go value -> the "Authorization" header go-git's
// http transport sets on the request it sends to url. It is held in memory for the
// lifetime of the caller's process only:
//
//   - never written to a file — there is no credential-helper layer to write it to;
//   - never placed in an environment variable — gitcore spawns no child process;
//   - never embedded in a URL — Fetch/Push/List take URL and Auth as separate fields,
//     and BasicAuth never renders Password into its Username, its error text, or any
//     string a caller could log; only the request's Authorization header carries it;
//   - never written to a log line by this package — BasicAuth.String() (used by
//     go-git's own logging/debug paths) masks Password, printing only
//     "http-basic-auth - x-access-token:*******".
//
// Each gitcore transport call takes its OWN Auth value (see FetchOpts/PushOpts/
// ListOpts), so a caller mints a token scoped to one op's URL and is structurally
// unable to send it anywhere else.
func BasicAuth(token string) *githttp.BasicAuth {
	return &githttp.BasicAuth{
		Username: appUsername,
		Password: token,
	}
}

// GitHubGitUsername / GitLabGitUsername are the git-over-HTTPS basic-auth usernames each forge
// pairs with a token password. GitHub accepts any non-empty username with a token (the house
// convention is "x-access-token", = appUsername); GitLab REQUIRES the literal "oauth2" when the
// password is a personal/OAuth token. A caller that speaks to whichever forge a repo resolves
// to (cmd/deskclaim-ref) picks the right one here rather than hardcoding GitHub's.
const (
	GitHubGitUsername = appUsername
	GitLabGitUsername = "oauth2"
)

// BasicAuthAs is BasicAuth with an explicit username, for a forge (GitLab) whose git transport
// does not accept the default. Same in-memory-only, never-rendered-into-a-string guarantees as
// BasicAuth — only the request's Authorization header ever carries the token.
func BasicAuthAs(username, token string) *githttp.BasicAuth {
	return &githttp.BasicAuth{
		Username: username,
		Password: token,
	}
}
