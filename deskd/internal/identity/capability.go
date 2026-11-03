package identity

import (
	"context"
	"github.com/medici-finance/assay/deskd/internal/humanid"
	"strings"
	"sync"
	"time"
)

// PermissionCacheTTL bounds how long a derived (user, repo) permission is
// trusted before it is re-queried. 120s: short enough that a
// revoked collaborator drops to anonymous within the window (Verify 11), long
// enough that a busy console is not one permission call per request.
const PermissionCacheTTL = 120 * time.Second

// PermissionQuerier answers "what permission does this user hold on this repo?"
// using DESKD'S OWN read identity — the dedicated read App token routed per-org
// by the per-org router — NEVER the viewer's own token. That independence is
// the whole point (Verify 6): a viewer's token scopes or claims cannot change
// their tier, because the query that decides the tier is made with a credential
// the viewer does not control.
//
// It returns the GitHub coarse permission ("none"/"read"/"triage"/"write"/
// "maintain"/"admin"), and ok=false when the answer could not be obtained for
// ANY reason (owner not routed, 404, 401, network). ok=false is the fail-closed
// signal — the Deriver maps it to Anonymous, never to a guessed tier.
type PermissionQuerier interface {
	Permission(ctx context.Context, owner, name, username string) (permission string, ok bool)
}

// Deriver computes a viewer's [Capability] on a specific item's repo, with a
// short per-(user, repo) cache. It is safe for concurrent use.
type Deriver struct {
	querier PermissionQuerier
	now     func() time.Time

	mu    sync.Mutex
	cache map[string]cachedBase
}

// cachedBase is a memoised base tier (anonymous/commenter — never answerer,
// which is login-derived and cheap). The answerer step is applied on every call
// so a change to answer_as takes effect immediately without a cache flush.
type cachedBase struct {
	tier      Capability
	expiresAt time.Time
}

// NewDeriver builds a capability deriver over a permission querier. A nil
// querier makes every non-answerer derivation fail closed to Anonymous, which
// is the correct posture for a deskd that could not build its read router.
func NewDeriver(q PermissionQuerier, now func() time.Time) *Deriver {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Deriver{querier: q, now: now, cache: map[string]cachedBase{}}
}

// Capability derives a viewer's tier on the repo owner/name.
//
//   - A nil session is Anonymous — no viewer, no tier.
//   - Otherwise the base tier comes from the deskd-side permission query
//     (cached ≤120s); none/error → Anonymous, read-or-above → Commenter.
//   - The answerer step is applied last: a viewer whose login equals answerAs is
//     an Answerer (a Commenter first — answerer implies commenter), regardless
//     of the base tier, because answer_as is the cell's authorized-answerer
//     login and the answer path authors with the operator custody credential, not
//     the viewer's repo permission.
//
// answerAs is compared case-insensitively: GitHub logins are case-insensitive.
func (d *Deriver) Capability(ctx context.Context, sess *Session, owner, name, answerAs string) Capability {
	if sess == nil {
		return Anonymous
	}
	base := d.baseTier(ctx, sess.Login, owner, name)
	if humanid.Enabled() && sess.Login != "" && answerAs != "" && strings.EqualFold(sess.Login, answerAs) {
		// Answerer implies commenter. Even if the base query failed closed, an
		// authenticated viewer who IS the configured answerer is an answerer —
		// their authority comes from being answer_as, not from a repo grant.
		return Answerer
	}
	return base
}

// baseTier returns the cached-or-queried base tier for (user, repo).
//
// A DEFINITE answer — a completed permission query, whether it reports read+ or
// a real "none" — is cached for [PermissionCacheTTL]. A fail-closed-on-error
// result (querier absent, or ok=false: owner not routed, 404, 401, network) is
// NOT cached, so it is retried on the next request rather than pinning a viewer
// to anonymous for the whole window on one transient error.
func (d *Deriver) baseTier(ctx context.Context, login, owner, name string) Capability {
	key := strings.ToLower(login) + "\x00" + strings.ToLower(owner) + "/" + strings.ToLower(name)

	d.mu.Lock()
	if c, ok := d.cache[key]; ok && d.now().Before(c.expiresAt) {
		d.mu.Unlock()
		return c.tier
	}
	d.mu.Unlock()

	if d.querier == nil {
		return Anonymous
	}
	perm, ok := d.querier.Permission(ctx, owner, name, login)
	if !ok {
		return Anonymous // fail closed; not cached, so the next request retries
	}
	tier := baseTierForPermission(perm)

	d.mu.Lock()
	d.cache[key] = cachedBase{tier: tier, expiresAt: d.now().Add(PermissionCacheTTL)}
	d.mu.Unlock()
	return tier
}
