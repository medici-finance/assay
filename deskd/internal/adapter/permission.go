package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// This file is the deskd-side collaborator-permission query that decides a
// viewer's capability tier (the capability model task 2, Verify 6). The load-bearing
// property: it is made with DESKD'S OWN read identity — the per-org read token
// routed exactly as the escalation poll is — NEVER the viewer's
// token. A viewer cannot influence their own tier, because the credential that
// answers "what permission does this login hold?" is one the viewer does not
// control.
//
// It reuses the read-only transport, so this query can never become a write:
// the same structural guarantee the escalation reader has.

// PermissionRouter answers collaborator-permission queries, one read client per
// org. It satisfies identity.PermissionQuerier structurally (no import either
// way — the signatures match and deskd wires them), so the identity package
// stays free of any GitHub or token dependency.
//
// It holds the [TokenSource], never a token: every query resolves the owner's
// CURRENT token through [CurrentToken] (the authorization contract). A token the sidecar
// rotates under --tokens-dir is used on the very next query with no deskd
// restart, and a token file that disappears fails that query closed rather than
// reusing the last token seen.
type PermissionRouter struct {
	apiBase string
	source  TokenSource
	owners  []string // the configured owners; any other owner is never resolved
	http    *http.Client
}

// NewPermissionRouter builds the router over the SAME [TokenSource] the
// escalation poller's [ReloadingRouter] re-resolves every cycle, restricted to
// the configured owners. An owner outside owners, or one the source has no
// token for at query time, yields ok=false (fail closed to anonymous) — never
// a cross-org read with another org's credential.
func NewPermissionRouter(apiBase string, source TokenSource, owners []string, base http.RoundTripper) *PermissionRouter {
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}
	return &PermissionRouter{
		apiBase: strings.TrimSuffix(apiBase, "/"),
		source:  source,
		owners:  append([]string(nil), owners...),
		http:    &http.Client{Transport: NewReadOnlyTransport(base), Timeout: 30 * time.Second},
	}
}

// Permission returns the GitHub coarse permission ("none"/"read"/"triage"/
// "write"/"maintain"/"admin") the username holds on owner/name, queried with
// the org's own read token.
//
// ok=false is the fail-closed signal for EVERY inability to answer: no token
// resolvable for the owner at query time, a non-200 (404 = the read token
// cannot see the repo or the user; 403 = rate limited), a decode failure, or
// a network error. The
// caller (identity.Deriver) maps ok=false to the anonymous tier and does not
// cache it, so a transient failure is retried rather than pinned.
func (p *PermissionRouter) Permission(ctx context.Context, owner, name, username string) (string, bool) {
	tok, ok := CurrentToken(p.source, p.owners, owner)
	if !ok {
		return "", false // owner not routed, or no token right now — never read with another org's token
	}
	url := fmt.Sprintf("%s/repos/%s/%s/collaborators/%s/permission", p.apiBase, owner, name, username)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := p.http.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}
	if resp.StatusCode != http.StatusOK {
		// 404 for a user with no access, 403 rate-limited, etc. Fail closed.
		return "", false
	}
	var body struct {
		Permission string `json:"permission"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", false
	}
	if body.Permission == "" {
		return "", false
	}
	return body.Permission, true
}

// RepoPrivate reports whether owner/name is a PRIVATE repo, queried with the
// org's own read token. It is the input to the private-repo gate (the capability model
// task 6): a cell containing any private repo login-gates its whole surface,
// and an `allow_anonymous_read` opt-in is valid only when EVERY repo verifies
// public here.
//
// ok=false is fail-closed: an owner with no token, a 404 (the read token
// cannot see it — indistinguishable from private), a 403, a decode or network
// error all return ok=false, and the caller MUST treat an unverifiable repo as
// NOT-known-public — i.e. as private for gate purposes. A repo that cannot be
// proven public is never served anonymously.
func (p *PermissionRouter) RepoPrivate(ctx context.Context, owner, name string) (private bool, ok bool) {
	tok, has := CurrentToken(p.source, p.owners, owner)
	if !has {
		return true, false
	}
	url := fmt.Sprintf("%s/repos/%s/%s", p.apiBase, owner, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return true, false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := p.http.Do(req)
	if err != nil {
		return true, false
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return true, false
	}
	if resp.StatusCode != http.StatusOK {
		return true, false
	}
	var body struct {
		Private bool `json:"private"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return true, false
	}
	return body.Private, true
}
