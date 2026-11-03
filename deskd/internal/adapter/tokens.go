// tokens.go — reloadable per-org read tokens (the service contract).
//
// # The problem this file exists to fix
//
// [ResolveTokens] (routing.go) has always been read ONCE, at process start:
// cmd/deskd/main.go builds one [RoutingFetcher] from it and hands that single
// instance to [index.Poller] for the life of the process. A GitHub App
// installation token expires after 1 hour, so a laptop deskd that runs past
// an hour is already polling blind unless something restarts it — and
// restarting drops every in-memory per-user session (the service contract), which
// is refused as a design for a hosted deskd (the service contract facts).
//
// # The fix
//
// [TokenSource] is resolved fresh at the top of EVERY poll cycle, never
// cached across cycles, by [ReloadingRouter] — which [index.Poller] drives
// through the [index.CycleRefresher] interface it satisfies structurally.
// [EnvTokenSource] is today's env-var behaviour, unchanged (Verify row 6).
// [FileTokenSource] is the new per-org token-FILE directory contract 28's
// sidecar writes into: this brief ships only the reader.
package adapter

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/medici-finance/assay/deskd/internal/domain"
)

// TokenSource resolves the current per-org read tokens for a set of owners.
// A [ReloadingRouter] calls Resolve at the START of every poll cycle — it is
// never cached across cycles by anything in this package — so an in-process
// credential rotation (a sidecar rewriting a file) is visible on the very
// next cycle with no deskd restart.
//
// Implementations must NEVER log or return a token value anywhere but
// byOwner: [MissingToken.Detail] names a LOCATION (an env var, a file path),
// never a value.
type TokenSource interface {
	Resolve(owners []string) (byOwner map[string]string, missing []MissingToken)
}

// MissingToken names an owner one resolution had no token for, and why.
// Detail is safe to surface verbatim in a could-not-check reason or a startup
// refusal — it names a location, never a value.
type MissingToken struct {
	Owner  string
	Detail string
}

// EnvTokenSource is today's token resolution, expressed as a [TokenSource]:
// the per-org DESKD_GITHUB_TOKEN_<ORG> env var, falling back to the legacy
// DESKD_GITHUB_TOKEN default (see [ResolveTokens], which this wraps
// unchanged). It is the DEFAULT source deskd builds when --tokens-dir is
// absent — Verify row 6 (TestEnvTokenSourceUnchanged, cmd/deskd) pins that it
// resolves EXACTLY as [ResolveTokens] always has, so the laptop `cellctl
// deskd` path keeps working untouched.
type EnvTokenSource struct {
	// Getenv is injected so this is testable without touching the process
	// environment — the same contract [ResolveTokens] has always had.
	Getenv func(string) string
}

// Resolve implements [TokenSource] over [ResolveTokens].
func (s EnvTokenSource) Resolve(owners []string) (map[string]string, []MissingToken) {
	byOwner, missing := ResolveTokens(owners, s.Getenv)
	out := make([]MissingToken, 0, len(missing))
	for _, o := range missing {
		out = append(out, MissingToken{
			Owner:  o,
			Detail: fmt.Sprintf("env: set %s or the default %s", EnvTokenName(o), legacyTokenEnv),
		})
	}
	return byOwner, out
}

// FileTokenSource resolves per-org tokens by reading one file per owner from
// a directory a sidecar shares with deskd (the service contract's tmpfs
// `emptyDir{medium: Memory}` contract; an explicit deployment contract). It is
// the READER half of the reload contract only — WHO writes the files is 28's
// sidecar (a desktoken-based minter holding the read-App PEM); this brief
// ships only this type and a test double.
//
// Resolve re-reads the directory on every call — it caches nothing — so it is
// safe (and required) to call once per poll cycle: a sidecar rotating a file
// between cycles is picked up on the very next one.
type FileTokenSource struct {
	// Dir is the directory holding one file per owner, named by
	// [fileTokenName]. A missing, empty, or unreadable file for a configured
	// owner is reported in missing rather than causing an error return — the
	// caller degrades that ONE org to could-not-check (the read surface rule:
	// never a silent empty inbox), it never blinds the whole cycle.
	Dir string
}

// fileTokenName is the per-org token-file basename: the owner uppercased with
// '-' replaced by '_' — the SAME suffix [EnvTokenName] uses after its
// DESKD_GITHUB_TOKEN_ prefix, so an operator who knows the env var name can
// spell the file name from it (the filename is derived from the owner).
func fileTokenName(owner string) string {
	return strings.ToUpper(strings.ReplaceAll(owner, "-", "_"))
}

// Resolve implements [TokenSource] over the token-file directory.
func (s FileTokenSource) Resolve(owners []string) (byOwner map[string]string, missing []MissingToken) {
	byOwner = map[string]string{}
	seen := map[string]bool{}
	for _, o := range owners {
		if _, done := byOwner[o]; done {
			continue
		}
		path := filepath.Join(s.Dir, fileTokenName(o))
		raw, err := os.ReadFile(path)
		if err != nil {
			if !seen[o] {
				seen[o] = true
				missing = append(missing, MissingToken{
					Owner:  o,
					Detail: fmt.Sprintf("file %s: %v", path, err),
				})
			}
			continue
		}
		tok := strings.TrimSpace(string(raw))
		if tok == "" {
			if !seen[o] {
				seen[o] = true
				missing = append(missing, MissingToken{
					Owner:  o,
					Detail: fmt.Sprintf("file %s: empty", path),
				})
			}
			continue
		}
		byOwner[o] = tok
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i].Owner < missing[j].Owner })
	return byOwner, missing
}

// MissingTokens reports the configured owners src has no token for right now,
// and nothing else. deskd's startup refusal reads only this. It never sees a
// resolved token map, so start-up holds no token snapshot that a long-lived
// consumer could be built from (the authorization contract).
func MissingTokens(src TokenSource, owners []string) []MissingToken {
	_, missing := src.Resolve(owners)
	return missing
}

// CurrentToken resolves owner's read token from src at the moment of the
// call. Nothing is cached, so a token the sidecar rotated, or removed, since
// the last call is what this call sees. The per-request read-token consumers
// ([PermissionRouter], the fleet map's cache) call it on every read instead
// of holding a start-time copy (the authorization contract). Before #217 those two
// consumers kept the start-time token: once it expired, about an hour after
// start, every read failed closed until deskd restarted.
//
// ok=false is the fail-closed answer. It covers an owner outside configured,
// which is never resolved at all: under the env source the legacy
// DESKD_GITHUB_TOKEN default resolves for ANY owner, and this check stops that
// default reaching an org deskd was not configured for. It also covers a nil
// source, and an owner the source has no token for this call (a missing,
// empty or unreadable token file). The token value is returned only through
// tok and is never logged.
func CurrentToken(src TokenSource, configured []string, owner string) (tok string, ok bool) {
	if src == nil || owner == "" {
		return "", false
	}
	known := false
	for _, o := range configured {
		if o == owner {
			known = true
			break
		}
	}
	if !known {
		return "", false
	}
	byOwner, _ := src.Resolve([]string{owner})
	tok, ok = byOwner[owner]
	if !ok || tok == "" {
		return "", false
	}
	return tok, true
}

// ReloadingRouter is a [Fetcher]-and-[OneFetcher]-satisfying router (it
// structurally matches [index.Fetcher] and [OneFetcher], exactly as
// [RoutingFetcher] does) that re-resolves its per-org clients from a
// [TokenSource] every time [ReloadingRouter.RefreshCycle] runs, rather than
// once at process start.
//
// [index.Poller.PollOnce] calls RefreshCycle — through the [index.CycleRefresher]
// interface this type satisfies structurally — exactly once, at the top of
// every cycle, before dispatching a single repo fetch. A token that rotated
// between cycles is therefore picked up with NO deskd restart and NO dropped
// in-memory per-user session (the service contract).
//
// A Fetcher that does not implement [index.CycleRefresher] (the plain
// [RoutingFetcher] / [NewGitHubRouter]) is untouched by this: PollOnce simply
// does not call anything, and behaves exactly as it always has (Verify row 6).
type ReloadingRouter struct {
	source  TokenSource
	owners  []string
	apiBase string
	base    http.RoundTripper

	mu      sync.RWMutex
	current *RoutingFetcher
	missing map[string]string // owner -> Detail, valid for the CURRENT cycle only
}

// NewReloadingRouter builds a router that starts with no client mapped for
// any owner — every fetch is could-not-check — until the first
// [ReloadingRouter.RefreshCycle] call. [index.Poller.PollOnce] always calls
// RefreshCycle before it dispatches a single repo fetch (including a --once
// run's one and only cycle), so in practice no fetch ever runs against an
// unrefreshed snapshot.
func NewReloadingRouter(apiBase string, source TokenSource, owners []string, base http.RoundTripper) *ReloadingRouter {
	return &ReloadingRouter{
		source:  source,
		owners:  append([]string(nil), owners...),
		apiBase: apiBase,
		base:    base,
		current: NewRoutingFetcher(nil),
		missing: map[string]string{},
	}
}

// RefreshCycle re-resolves every owner's token from the source and rebuilds
// the per-owner client set. It satisfies [index.CycleRefresher] structurally.
//
// It NEVER logs or returns a token value — only [MissingToken.Detail]
// strings, which name a location, not a value — matching the same rule
// [ResolveTokens] has always held.
func (r *ReloadingRouter) RefreshCycle(_ context.Context) {
	byOwner, missing := r.source.Resolve(r.owners)
	next := NewGitHubRouter(r.apiBase, byOwner, r.base)
	missingByOwner := make(map[string]string, len(missing))
	for _, m := range missing {
		missingByOwner[m.Owner] = m.Detail
	}
	r.mu.Lock()
	r.current = next
	r.missing = missingByOwner
	r.mu.Unlock()
}

func (r *ReloadingRouter) snapshot() (*RoutingFetcher, map[string]string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current, r.missing
}

// FetchEscalations dispatches through the MOST RECENTLY refreshed client set.
// An owner this cycle had no token for returns the same could-not-check shape
// [RoutingFetcher.FetchEscalations] returns for an unmapped owner, with the
// source's own detail folded into the reason — never the token value.
func (r *ReloadingRouter) FetchEscalations(ctx context.Context, repo Repo) FetchResult {
	cur, missing := r.snapshot()
	if detail, ok := missing[repo.Owner]; ok {
		return missingTokenResult(repo, detail)
	}
	return cur.FetchEscalations(ctx, repo)
}

// FetchOne dispatches a single-issue re-derivation through the most recently
// refreshed client set, mirroring [RoutingFetcher.FetchOne].
func (r *ReloadingRouter) FetchOne(ctx context.Context, repo Repo, number int) (RawIssue, bool, error) {
	cur, missing := r.snapshot()
	if detail, ok := missing[repo.Owner]; ok {
		return RawIssue{}, false, fmt.Errorf("no read token is mapped for owner %q this cycle (%s) — refresh cannot re-derive %s#%d", repo.Owner, detail, repo, number)
	}
	return cur.FetchOne(ctx, repo, number)
}

// Owners lists the owners this router was built to route, sorted — NOT
// filtered by whether a token currently resolves for them (that varies per
// cycle; a missing owner this cycle is still one this router is configured to
// route, and will resolve again the moment its token reappears).
func (r *ReloadingRouter) Owners() []string {
	out := append([]string(nil), r.owners...)
	sort.Strings(out)
	return out
}

// missingTokenResult mirrors [routingProbe] / [RoutingFetcher.FetchEscalations]'s
// unmapped-owner branch, with the source's own detail folded into the reason
// so an operator reading the inbox sees WHERE to look (a file path, an env
// var name) — never a token value.
func missingTokenResult(repo Repo, detail string) FetchResult {
	now := time.Now().UTC()
	p := routingProbe(repo.Owner)
	reason := fmt.Sprintf("no read token is mapped for owner %q this cycle (%s) — this repo's slice is could-not-check, "+
		"not empty, and is NEVER read with another org's token (a GitHub App mints one installation token per org, "+
		"so an unmapped org has no credential of its own)", repo.Owner, detail)
	return FetchResult{
		Repo:       repo,
		Health:     RepoHealth{Repo: repo.String(), CheckedAt: now, Assessment: domain.Unchecked(p, reason)},
		Candidates: domain.NoCount(p, reason),
		Derived:    domain.NoCount(p, reason),
	}
}
