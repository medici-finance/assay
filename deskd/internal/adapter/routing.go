package adapter

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// THE TRAP, stated at the code so no future reader designs it away: a GitHub
// App mints ONE installation token per org it is installed on — a token minted
// for one org cannot read another org's repos, and no App and no token spans an
// org boundary. The dedicated read App (the service contract) fixes deskd's identity
// and privileges; it does NOT make one token span orgs. So a cell spanning N
// orgs needs N installation tokens EVEN WITH its own App, and something has to
// hold the N tokens and pick the right one per repo. That something is this
// router. 17 (identity) and 18 (per-org routing) are complementary, not
// alternatives. Nothing here may assume "own App ⇒ one token".

// legacyTokenEnv is the single read token deskd has always taken. It is
// retained as the DEFAULT for any org that has no org-specific override, so a
// single-org cell keeps working with exactly its old configuration.
const legacyTokenEnv = "DESKD_GITHUB_TOKEN"

// perOrgTokenPrefix opens the per-org read-token env vars:
// DESKD_GITHUB_TOKEN_<ORG>, e.g. DESKD_GITHUB_TOKEN_EXAMPLE_ORG,
// DESKD_GITHUB_TOKEN_OTHER_ORG. The suffix is the owner uppercased with '-'
// mapped to '_' (GitHub org logins never contain '_', so the mapping is
// injective in practice). See [EnvTokenName].
const perOrgTokenPrefix = "DESKD_GITHUB_TOKEN_"

// EnvTokenName returns the per-org read-token env var name for an owner:
// DESKD_GITHUB_TOKEN_<ORG> with ORG uppercased and '-' replaced by '_'.
//
// This is the ONE place the owner→env-var-name mapping is spelled out; the
// routing itself keys off the real owner string, so this normalisation is used
// only to look the token up, never to route.
func EnvTokenName(owner string) string {
	return perOrgTokenPrefix + strings.ToUpper(strings.ReplaceAll(owner, "-", "_"))
}

// ResolveTokens resolves a read token for every owner a cell's repos name.
//
// For each owner it prefers the org-specific DESKD_GITHUB_TOKEN_<ORG>, falling
// back to the legacy DESKD_GITHUB_TOKEN default. An owner with NEITHER a
// specific token NOR the default is returned in missing — the caller refuses to
// start and names it, because a repo whose owner has no token would otherwise
// poll blind. There is deliberately no fallback to another org's token: reading
// one org's repos with another org's credential is the exact fault the router
// exists to prevent.
//
// getenv is injected so this is testable without touching the process
// environment. It NEVER logs a token; callers must not either.
func ResolveTokens(owners []string, getenv func(string) string) (byOwner map[string]string, missing []string) {
	def := getenv(legacyTokenEnv)
	byOwner = map[string]string{}
	seenMissing := map[string]bool{}
	for _, o := range owners {
		if _, done := byOwner[o]; done {
			continue
		}
		if t := getenv(EnvTokenName(o)); t != "" {
			byOwner[o] = t
			continue
		}
		if def != "" {
			byOwner[o] = def
			continue
		}
		if !seenMissing[o] {
			seenMissing[o] = true
			missing = append(missing, o)
		}
	}
	sort.Strings(missing)
	return byOwner, missing
}

// EscalationFetcher is the per-repo read surface a [RoutingFetcher] dispatches
// to. [Client] satisfies it. The interface exists so the router can hold a
// heterogeneous set of clients, be built from fakes in a table test, and live
// in this package WITHOUT importing internal/index (whose Fetcher interface a
// *RoutingFetcher satisfies structurally — importing it here would be a cycle).
type EscalationFetcher interface {
	FetchEscalations(ctx context.Context, repo Repo) FetchResult
}

// OneFetcher is the single-issue re-derivation [Client] also satisfies. It is
// a separate interface (rather than folded into [EscalationFetcher]) so a
// fake built for the list path alone still satisfies the router without
// having to stub a method it never exercises.
type OneFetcher interface {
	FetchOne(ctx context.Context, repo Repo, number int) (RawIssue, bool, error)
}

// RoutingFetcher holds one read client per org and dispatches each repo's fetch
// to the client for repo.Owner.
//
// It satisfies internal/index.Fetcher structurally, so the poller loop calls it
// through the existing interface with ZERO loop change: the poller already
// passes a Repo that carries its owner, and this router switches on it.
//
// Fail-closed is the load-bearing property: an owner with no mapped client
// returns a could-not-check slice (see [RoutingFetcher.FetchEscalations]),
// never a fetch with a different org's token. A missing mapping degrades one
// slice visibly; it can neither blind the whole poll nor cross-read another org.
type RoutingFetcher struct {
	byOwner map[string]EscalationFetcher
}

// NewRoutingFetcher builds a router from an owner→fetcher map. It is the core
// constructor — [NewGitHubRouter] wraps it for the production token path, and
// tests build it directly from recording fakes.
func NewRoutingFetcher(byOwner map[string]EscalationFetcher) *RoutingFetcher {
	m := make(map[string]EscalationFetcher, len(byOwner))
	for owner, f := range byOwner {
		m[owner] = f
	}
	return &RoutingFetcher{byOwner: m}
}

// NewGitHubRouter builds a router with one read-only [Client] per owner from an
// owner→token map (as produced by [ResolveTokens]).
//
// Every token is read-scoped (the service contract) and each org gets its OWN client
// holding ONLY its own org's token — there is no shared client and no default
// client, so one org's credential is structurally unreachable from another
// org's fetch. This adds no write path.
func NewGitHubRouter(apiBase string, tokensByOwner map[string]string, base http.RoundTripper) *RoutingFetcher {
	byOwner := make(map[string]EscalationFetcher, len(tokensByOwner))
	for owner, tok := range tokensByOwner {
		byOwner[owner] = NewGitHub(apiBase, tok, base)
	}
	return NewRoutingFetcher(byOwner)
}

// routingProbe declares the routing-table lookup that failed.
func routingProbe(owner string) domain.Probe {
	return domain.Probe{
		Source: fmt.Sprintf("owner→token routing table (DESKD_GITHUB_TOKEN_<ORG> per org); owner %q has no mapped token", owner),
		Method: domain.MethodLocalTable,
		Window: "the read client mapped for this repo's owner",
		Limit:  0,
	}
}

// FetchEscalations dispatches to the client mapped for repo.Owner.
//
// An unmapped owner returns a could-not-check slice — NOT an empty one, and
// NEVER a fetch with another org's token. This mirrors the degraded-poll
// behaviour a single client already produces on a 404/401 (see
// [Client.FetchEscalations]): the poller leaves the repo's prior rows in place
// and records could-not-check, because an empty result from a source we cannot
// read is BLIND, not idle. The one thing it must not do is fall back to a
// different org's credential: a GitHub App mints one installation token per org,
// so an unmapped org simply has no credential of its own, and borrowing
// another's would either leak a cross-org read or mask a missing install.
func (r *RoutingFetcher) FetchEscalations(ctx context.Context, repo Repo) FetchResult {
	f, ok := r.byOwner[repo.Owner]
	if !ok {
		now := time.Now().UTC()
		p := routingProbe(repo.Owner)
		reason := fmt.Sprintf("no read token is mapped for owner %q — this repo's slice is could-not-check, "+
			"not empty, and is NEVER read with another org's token (a GitHub App mints one installation token per org, "+
			"so an unmapped org has no credential of its own)", repo.Owner)
		return FetchResult{
			Repo:       repo,
			Health:     RepoHealth{Repo: repo.String(), CheckedAt: now, Assessment: domain.Unchecked(p, reason)},
			Candidates: domain.NoCount(p, reason),
			Derived:    domain.NoCount(p, reason),
		}
	}
	return f.FetchEscalations(ctx, repo)
}

// FetchOne dispatches a single-issue re-derivation to the client mapped for
// repo.Owner (the service contract increment 3: the refresh nudge shares the SAME
// per-org routing the poll uses — no new token plumbing). An unmapped owner or
// a fetcher that does not implement [OneFetcher] (a test fake built for the
// list path only) is reported by name rather than silently skipped.
func (r *RoutingFetcher) FetchOne(ctx context.Context, repo Repo, number int) (RawIssue, bool, error) {
	f, ok := r.byOwner[repo.Owner]
	if !ok {
		return RawIssue{}, false, fmt.Errorf("no read token is mapped for owner %q — refresh cannot re-derive %s#%d", repo.Owner, repo, number)
	}
	one, ok := f.(OneFetcher)
	if !ok {
		return RawIssue{}, false, fmt.Errorf("the read client mapped for owner %q does not support a single-issue refresh", repo.Owner)
	}
	return one.FetchOne(ctx, repo, number)
}

// Owners lists the owners the router can route, sorted. It exists so a caller
// (or a test) can assert coverage without reaching into the unexported map. It
// returns owners, never tokens.
func (r *RoutingFetcher) Owners() []string {
	out := make([]string, 0, len(r.byOwner))
	for o := range r.byOwner {
		out = append(out, o)
	}
	sort.Strings(out)
	return out
}
