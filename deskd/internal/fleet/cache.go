package fleet

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/deskd/internal/adapter"
)

// Cache is the fleet map's read path: one [adapter.Client] per org, built
// from the SAME [adapter.TokenSource] the escalation poller re-resolves every
// cycle (no separate token plumbing), plus an in-memory last-good
// snapshot per repo so a failed live read degrades to STALE rather than
// either a blank lane or a fabricated-fresh one (brief §single-point-of-
// failure).
//
// It is deliberately NOT a background poller — "No new daemon" (brief
// §Delivery). Every [Cache.Snapshot] call does a live bounded read; the
// cache exists only to answer "what did we last actually see" when that
// live read fails, which is what makes the negative path (kill the fetch,
// reload) show stale data instead of an empty or falsely-fresh page.
type Cache struct {
	apiBase string
	base    http.RoundTripper
	// source is resolved on EVERY read via [adapter.CurrentToken], never once
	// at construction (the authorization contract): a rotated token is used on the next
	// Snapshot, and a vanished one fails that repo's live read closed.
	source adapter.TokenSource
	owners []string // the configured owners; any other owner is never resolved

	clientsMu sync.Mutex
	clients   map[string]ownerClient // owner -> client built for the token resolved last
	// pollInterval is the cadence [StaleThreshold] multiplies — named here,
	// not a bare duration, per the brief's design record.
	pollInterval time.Duration
	// claimStaleAfter is the per-claim age threshold [classifyClaim] uses.
	claimStaleAfter time.Duration
	now             func() time.Time

	mu   sync.Mutex
	last map[string]cachedSnapshot // repo ("owner/name") -> last good
}

// ownerClient pairs a client with the token it was built for, so a client is
// reused (keeping its connection pool) only while the source still resolves
// that same token, and rebuilt the moment the token rotates.
type ownerClient struct {
	token  string
	client *adapter.Client
}

type cachedSnapshot struct {
	tokens    []Token
	fetchedAt time.Time
}

// DefaultClaimStaleAfter is the fallback per-claim staleness threshold when
// no lease TTL is configured. It mirrors this repo's own MCP claim lease
// default ([internal/mcp.defaultLeaseTTL] = 15m) scaled by
// [ClaimStaleThresholdMultiplier] — a placeholder pending a real
// dispatch-claim TTL surfaced from cells.yaml; tune it there, not here, if
// the house's actual lease cadence differs.
var DefaultClaimStaleAfter = ClaimStaleThreshold(15 * time.Minute)

// NewCache builds a fleet cache whose read-only clients (identical
// construction to [adapter.NewGitHubRouter], just keeping the concrete
// *[adapter.Client] so [Cache] can reach the two fleet-only methods
// [adapter.Client.FetchOpenPullRequests] / [adapter.Client.FetchDispatchClaims],
// which the narrower [adapter.EscalationFetcher] interface the escalation
// router uses does not expose) are built per org from source, re-resolved on
// every read. owners is the configured owner set; a repo whose owner is not in
// it is never read.
//
// pollInterval is deskd's own poll cadence (same value the escalation
// poller runs on); claimStaleAfter is the per-claim age threshold — pass
// [DefaultClaimStaleAfter] absent a configured one. now is injectable for
// tests.
func NewCache(apiBase string, source adapter.TokenSource, owners []string, base http.RoundTripper, pollInterval, claimStaleAfter time.Duration, now func() time.Time) *Cache {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if claimStaleAfter <= 0 {
		claimStaleAfter = DefaultClaimStaleAfter
	}
	return &Cache{
		apiBase:         apiBase,
		base:            base,
		source:          source,
		owners:          append([]string(nil), owners...),
		clients:         map[string]ownerClient{},
		pollInterval:    pollInterval,
		claimStaleAfter: claimStaleAfter,
		now:             now,
		last:            map[string]cachedSnapshot{},
	}
}

// clientFor returns a read client for owner's CURRENT token. ok=false means
// no token resolves for owner right now (unconfigured owner, missing or empty
// token file): the cached client for that owner is dropped too, so a token
// that has vanished is never reused.
func (c *Cache) clientFor(owner string) (*adapter.Client, bool) {
	tok, ok := adapter.CurrentToken(c.source, c.owners, owner)
	c.clientsMu.Lock()
	defer c.clientsMu.Unlock()
	if !ok {
		delete(c.clients, owner)
		return nil, false
	}
	if cur, has := c.clients[owner]; has && cur.token == tok {
		return cur.client, true
	}
	cl := adapter.NewGitHub(c.apiBase, tok, c.base)
	c.clients[owner] = ownerClient{token: tok, client: cl}
	return cl, true
}

// Snapshot reads every repo's claims + open PRs and returns one [RepoLane]
// per repo, in the order repos was given. Each repo's lane degrades
// independently — one unreachable repo never blanks the rest of the map,
// the same "one repo, one slice" discipline [adapter.Client.FetchEscalations]
// already follows.
func (c *Cache) Snapshot(ctx context.Context, repos []string) []RepoLane {
	lanes := make([]RepoLane, 0, len(repos))
	for _, slug := range repos {
		lanes = append(lanes, c.snapshotOne(ctx, slug))
	}
	return lanes
}

func (c *Cache) snapshotOne(ctx context.Context, slug string) RepoLane {
	now := c.now()
	repo, err := adapter.ParseRepo(slug)
	if err != nil {
		return RepoLane{Repo: slug, OK: false, Reason: "not a valid owner/name repo slug: " + err.Error()}
	}

	client, ok := c.clientFor(repo.Owner)
	if !ok {
		return c.fallback(slug, now, "no read token resolves for owner "+repo.Owner+" right now (see the same per-org token source the escalation poller uses)")
	}

	claims, claimHealth := client.FetchDispatchClaims(ctx, repo)
	prs, prHealth := client.FetchOpenPullRequests(ctx, repo)

	if claimHealth.OK && prHealth.OK {
		tokens := make([]Token, 0, len(claims)+len(prs))
		for _, cl := range claims {
			tokens = append(tokens, classifyClaim(cl, now, c.claimStaleAfter))
		}
		for _, pr := range prs {
			tokens = append(tokens, classifyPR(pr, now))
		}
		sortTokens(tokens)

		c.mu.Lock()
		c.last[slug] = cachedSnapshot{tokens: tokens, fetchedAt: now}
		c.mu.Unlock()

		return RepoLane{Repo: slug, OK: true, Tokens: tokens, FetchedAt: now, Stale: false}
	}

	var reasons []string
	if !claimHealth.OK {
		reasons = append(reasons, "claims: "+claimHealth.Reason)
	}
	if !prHealth.OK {
		reasons = append(reasons, "open PRs: "+prHealth.Reason)
	}
	return c.fallback(slug, now, strings.Join(reasons, "; "))
}

// fallback serves the last good snapshot marked Stale, or reports
// could-not-check when there has never been a good one. This is the
// negative-path behaviour the brief's Verify row 2 pins: a killed fetch
// must never render as fresh, and must never blank a lane that has real
// (if aging) data to show.
func (c *Cache) fallback(slug string, now time.Time, reason string) RepoLane {
	c.mu.Lock()
	cached, ok := c.last[slug]
	c.mu.Unlock()
	if !ok {
		return RepoLane{Repo: slug, OK: false, Reason: reason}
	}

	// Re-derive each token's age (and any stale-claim reclassification)
	// against NOW rather than the moment it was cached — the ref's
	// claimed-at time is still known-good even though this cycle's live
	// read failed, so a claim that crosses the staleness line while the
	// fetch is down still gets flagged.
	drift := now.Sub(cached.fetchedAt)
	tokens := make([]Token, len(cached.tokens))
	for i, t := range cached.tokens {
		t.Age += drift
		if t.Kind == KindClaim && t.Age > c.claimStaleAfter {
			t.State = StaleClaimState
		}
		tokens[i] = t
	}

	return RepoLane{
		Repo: slug, OK: true, Tokens: tokens, FetchedAt: cached.fetchedAt,
		Stale: true, Reason: reason,
	}
}

func sortTokens(tokens []Token) {
	sort.SliceStable(tokens, func(i, j int) bool {
		if tokens[i].Kind != tokens[j].Kind {
			return tokens[i].Kind == KindClaim // claims first, then PRs
		}
		return tokens[i].Key < tokens[j].Key
	})
}
