package deskkit

// tokenidentity.go — issue 1631: WHO does an inherited credential act as?
//
// A desk verb that mints its own role token used to treat any GH_TOKEN in its environment as the
// operator's deliberate override and skip the mint. A cell shim that exported the operator's
// ambient `gh` login as GH_TOKEN therefore made the verb act as the human, with nothing in the
// verb able to tell the difference. The shim no longer does that, but the verb must not depend on
// every launcher getting it right: an inherited token is honoured only once it is shown to BE the
// dispatching role's App. This file is the check — one forge read plus a roster comparison.

import (
	"net/http"
	"strings"
)

// TokenIdentity is the forge account a credential acts as.
type TokenIdentity struct {
	// Login is the account's login as the forge renders it: `<slug>[bot]` for a GitHub App
	// installation token, the user's login for a personal credential.
	Login string
	// ID is the account's numeric USER id (a bot USER id for an App token); 0 when the forge
	// named none.
	ID int64
}

// GitHubTokenIdentity asks GitHub which account token acts as, with ONE GraphQL `viewer` read.
// GraphQL answers for an App installation token too (`<slug>[bot]` plus the bot USER id), where
// REST GET /user refuses an installation token outright — so this one read tells a role App's
// token from a human login and from ANOTHER role's App alike, which a probe like
// GET /installation/repositories (true for every App's token) cannot.
//
// Any failure — transport, a non-2xx such as 401 Bad credentials, a GraphQL error, an answer with
// no login — is returned as an error. An identity that could not be read is never an identity.
func GitHubTokenIdentity(token string) (TokenIdentity, error) {
	return (&GitHubForge{Token: token}).viewerIdentity()
}

// viewerIdentity is GitHubTokenIdentity's read, on the forge's own authenticated client (so a
// test points it at an httptest server through BaseURL, and the token is bound explicitly — an
// empty one is refused by restClient, never resolved from an ambient gh login).
func (g *GitHubForge) viewerIdentity() (TokenIdentity, error) {
	in := map[string]any{"query": `query{viewer{login databaseId}}`}
	var out struct {
		Data struct {
			Viewer struct {
				Login      string `json:"login"`
				DatabaseID int64  `json:"databaseId"`
			} `json:"viewer"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := g.doJSON(http.MethodPost, "/graphql", in, &out); err != nil {
		return TokenIdentity{}, err
	}
	if len(out.Errors) > 0 {
		return TokenIdentity{}, Unverifiable("viewer GraphQL error: "+StripControl(out.Errors[0].Message), nil)
	}
	login := strings.TrimSpace(out.Data.Viewer.Login)
	if login == "" {
		return TokenIdentity{}, Unverifiable("the viewer GraphQL answer named no login — an absent identity is not an identity", nil)
	}
	return TokenIdentity{Login: login, ID: out.Data.Viewer.DatabaseID}, nil
}

// ActsAsRole reports whether this identity IS role's App as the roster binds it: the login is one
// of the role's accepted renderings, and — when the roster pins the bot USER id — the ids agree
// too, so a look-alike account cannot pass on its login alone. bound is false when the roster
// gives role no bot identity; the caller then cannot check at all, and match is false.
func (id TokenIdentity) ActsAsRole(role string) (match, bound bool) {
	ident, ok := EffectiveConfig().RoleBotIdentity(role)
	if !ok {
		return false, false
	}
	got := strings.ToLower(strings.TrimSpace(id.Login))
	if got == "" {
		return false, true
	}
	for _, l := range ident.AcceptedLogins() {
		if got == strings.ToLower(l) {
			return ident.ID == 0 || id.ID == ident.ID, true
		}
	}
	return false, true
}
