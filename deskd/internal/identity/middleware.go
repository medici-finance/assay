package identity

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// This file is the SINGLE control the whole the capability model model depends on
// (SPOF-note): one capability derivation wrapped around every data and write
// route. The layers behind it — GitHub's own enforcement on the user-token
// comment path, the internal/comment package-graph separation, and the answer
// path's own answerer-binding gate — are in their own packages. This is the
// layer that refuses first, and it refuses on a deskd-side derived tier, never
// on anything a request supplied.
//
// The web UI renders affordances by tier but is NOT counted here: every gate is
// enforced server-side on the route, before a handler runs.

// RepoResolver resolves an escalation id to the repo that owns it, against the
// TRUSTED index — never from anything in the request. ok=false when the id is
// unknown, which the guard treats as fail-closed.
type RepoResolver interface {
	RepoForItem(id string) (owner, name string, ok bool)
}

// CellPolicy answers the cell-level questions the guard needs: which login may
// answer, and whether the surface is login-gated / anonymous-opt-in. It is
// implemented in the deskd wiring over the cells.yaml schema and the live
// public/private repo verification.
type CellPolicy interface {
	// AnswerAsForRepo returns the answer_as login governing the cell that owns
	// owner/name. "" when unknown (no cell owns it) — which makes an answerer
	// promotion impossible for that repo, the safe default.
	AnswerAsForRepo(owner, name string) string
	// AnonymousReadAllowed reports whether the cell that owns owner/name carries
	// a VERIFIED anonymous-read opt-in (every repo in the cell confirmed public
	// against the GitHub API). Only ever true for a data-READ route.
	AnonymousReadAllowed(owner, name string) bool
	// SurfaceLoginGated reports whether a surface-scoped route (the inbox and
	// other multi-repo feeds) must require an authenticated session because the
	// cell set contains a private repo and is not a verified anonymous-opt-in.
	SurfaceLoginGated() bool
}

// Guard enforces the capability ladder on each route. It holds the session
// store, the tier deriver, the repo resolver, and the cell policy.
type Guard struct {
	Sessions *SessionStore
	Deriver  *Deriver
	Repos    RepoResolver
	Policy   CellPolicy
	// Rollout is the service contract Task 9 gate: on a cell that has not
	// opted in, NO viewer derives above [Anonymous], whatever their GitHub
	// permission and whatever the environment is configured with. A nil
	// Rollout is an unresolved gate and fails CLOSED to OFF — see
	// [RolloutGate].
	Rollout RolloutGate
}

// Route is one protected registration in the deskd HTTP surface. The table of
// these is BOTH the enforcement (each is wrapped by [Guard.wrap]) and the
// contract [ValidateRoutes] / the route-table walk test check: a data or write
// route with Min < Commenter is a hole, refused at build time.
type Route struct {
	// Pattern is the top-mux prefix the route mounts at (e.g. "/answer/").
	Pattern string
	// Min is the minimum tier. A protected route MUST require at least
	// Commenter; the auth, login-page and static routes are NOT in this table.
	Min Capability
	// ItemScoped is true when the repo is derivable from an {id} path segment
	// after Pattern. When false the route is surface-scoped and the
	// private-repo gate ([CellPolicy.SurfaceLoginGated]) applies instead.
	ItemScoped bool
	// AllowAnonymousRead lets a DATA-read route (Min==Commenter, ItemScoped)
	// serve an anonymous viewer when the cell's verified opt-in is on. It is
	// meaningless for a comment/answer route and MUST be false there.
	AllowAnonymousRead bool
	// Authenticated is a route-owned authentication that stands in for a VIEWER
	// SESSION on a surface-scoped route (the service contract P3). It exists for two
	// cases: the MCP surface, whose callers are Apps rather than people, so the
	// per-user OAuth session the capability ladder is built on does not and
	// should not exist for them; and a `/v1` named action carrying a verified
	// signed intent (the authorization contract ruling 3), where the enrolled-key
	// signature is the caller's authentication and no viewer session exists.
	//
	// Before increment 2 that case was handled by mounting /mcp OUTSIDE this
	// table entirely, which is how the route-boundary review found it unauthenticated
	// — a route outside the table is a route no walk can check. This field is
	// the alternative: the route joins the table, ValidateRoutes and the walk
	// cover it, and the guard defers the "who is this" question to the gate that
	// can actually answer it for that caller class.
	//
	// It NEVER weakens an item-scoped route: the per-repo capability derivation
	// below is untouched, and this field is consulted only on the surface branch.
	// A route setting it must not also set AllowAnonymousRead — "authenticated by
	// another gate" and "reachable with no gate" are opposites.
	Authenticated func(r *http.Request) bool
	// Handler is the wrapped handler.
	Handler http.Handler
}

// ValidateRoutes asserts every entry is a real protected route: a minimum tier
// of at least Commenter, and no anonymous-read shortcut on a route that writes.
// This is what the route-table walk test (Verify 9) runs — registering a
// data/write route with Min==Anonymous makes it return an error, exit 1.
func ValidateRoutes(routes []Route) error {
	for _, r := range routes {
		if r.Pattern == "" {
			return fmt.Errorf("a protected route has no pattern")
		}
		if r.Min < Commenter {
			return fmt.Errorf("route %q registers with min tier %q — every data or write route must require at least %q; a route without the capability gate is a hole",
				r.Pattern, r.Min, Commenter)
		}
		if r.AllowAnonymousRead && r.Authenticated != nil {
			return fmt.Errorf("route %q both allows anonymous read and declares its own authentication — those are opposites; pick one", r.Pattern)
		}
		if r.AllowAnonymousRead && r.Min > Commenter {
			return fmt.Errorf("route %q allows anonymous read but requires tier %q — anonymous read is only ever valid for a data-read (commenter) route, never a write",
				r.Pattern, r.Min)
		}
		if r.Handler == nil {
			return fmt.Errorf("route %q has no handler", r.Pattern)
		}
	}
	return nil
}

// Mount validates the route table and returns a mux that serves each route
// through the capability gate. A validation failure is returned, never
// silently mounted — deskd fatals on it, so a hole cannot ship.
func (g *Guard) Mount(routes []Route) (http.Handler, error) {
	if err := ValidateRoutes(routes); err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Pattern, g.wrap(r))
	}
	return mux, nil
}

// Handler returns the capability-gated handler for one route. deskd registers
// it on the top mux; it is [wrap] exported so the same gate the [Mount] path
// applies can be applied route-by-route on a shared mux.
func (g *Guard) Handler(route Route) http.Handler { return g.wrap(route) }

// wrap is the per-request gate.
func (g *Guard) wrap(route Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := g.session(r)

		if !route.ItemScoped {
			// Surface-scoped (inbox, feeds): the private-repo gate. Pre-auth,
			// every data route on a login-gated surface returns 401 and no data
			// — unless the route carries its own authentication for a caller
			// class that has no viewer session (the MCP surface; see
			// [Route.Authenticated]).
			if g.Policy != nil && g.Policy.SurfaceLoginGated() && sess == nil {
				if route.Authenticated == nil || !route.Authenticated(r) {
					denyUnauth(w)
					return
				}
			}
			route.Handler.ServeHTTP(w, r)
			return
		}

		owner, name, ok := g.repoForRequest(route, r)
		if !ok {
			// Repo not resolvable from the trusted index: fail closed. A caller
			// with no session gets 401 (sign in); one with a session gets 404 —
			// an unknown item, not a permission leak.
			if sess == nil {
				denyUnauth(w)
			} else {
				denyNotFound(w)
			}
			return
		}

		// Anonymous read is allowed only for a data-read route whose cell has a
		// verified public opt-in.
		if sess == nil {
			if route.AllowAnonymousRead && g.Policy != nil && g.Policy.AnonymousReadAllowed(owner, name) {
				route.Handler.ServeHTTP(w, r)
				return
			}
			denyUnauth(w)
			return
		}

		cap := g.derive(r, sess, owner, name)
		if cap < route.Min {
			denyForbidden(w, route.Min, cap)
			return
		}
		route.Handler.ServeHTTP(w, r)
	})
}

// derive is the ONE capability derivation in this package: the rollout gate
// first, then the deskd-side permission query. Every caller — the route gate,
// the web layer's affordance hint, the surface listing's per-row check — goes
// through here, so the gate cannot be enforced on one path and forgotten on
// another (which is exactly how the surface-feed leak happened on PR #42).
//
// On an OFF cell the answer is [Anonymous] for EVERY viewer, including one
// whose GitHub permission is admin and whose session is perfectly valid. That
// is the ruled meaning of OFF: login-gated, nobody above anonymous, no
// single-persona fallback.
func (g *Guard) derive(r *http.Request, sess *Session, owner, name string) Capability {
	if !rolloutEnabledFor(g.Rollout, owner, name) {
		return Anonymous
	}
	if g.Deriver == nil {
		return Anonymous
	}
	answerAs := ""
	if g.Policy != nil {
		answerAs = g.Policy.AnswerAsForRepo(owner, name)
	}
	return g.Deriver.Capability(r.Context(), sess, owner, name, answerAs)
}

func (g *Guard) session(r *http.Request) *Session {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return nil
	}
	if g.Sessions == nil {
		return nil
	}
	sess, err := g.Sessions.Validate(c.Value)
	if err != nil {
		return nil
	}
	return sess
}

func (g *Guard) repoForRequest(route Route, r *http.Request) (owner, name string, ok bool) {
	id := itemIDFromPath(route.Pattern, r.URL.Path)
	if id == "" || g.Repos == nil {
		return "", "", false
	}
	return g.Repos.RepoForItem(id)
}

// CapabilityFor is the read-only derivation the web layer uses to decide which
// affordances to render (never a control). It mirrors wrap's derivation.
func (g *Guard) CapabilityFor(r *http.Request, owner, name string) (login string, cap Capability) {
	sess := g.session(r)
	if sess == nil {
		return "", Anonymous
	}
	return sess.Login, g.derive(r, sess, owner, name)
}

// SessionFor returns the authenticated session for a request, or nil.
func (g *Guard) SessionFor(r *http.Request) *Session { return g.session(r) }

// ViewerInfo is the read-only capability summary the web layer renders chrome
// and affordances from. It is NEVER a control — the server-side gate on each
// route is; this only decides which buttons a page draws.
type ViewerInfo struct {
	// Authenticated is true when a valid session cookie is present.
	Authenticated bool
	// Login is the authenticated GitHub login, or "" when anonymous.
	Login string
	// Capability is the tier string ("anonymous"/"commenter"/"answerer") on the
	// repo in question. For a surface view (no single repo) it is "" and only
	// Login/Authenticated are meaningful.
	Capability string
	// CanComment / CanAnswer render the comment box and answer affordances. The
	// server still enforces both regardless of what the page drew.
	CanComment bool
	CanAnswer  bool
}

// ViewerFor derives the viewer's capability on owner/name for rendering. It
// mirrors the middleware's derivation exactly, so the affordances a page shows
// match the gate a request will hit.
func (g *Guard) ViewerFor(r *http.Request, owner, name string) ViewerInfo {
	login, cap := g.CapabilityFor(r, owner, name)
	return ViewerInfo{
		Authenticated: login != "",
		Login:         login,
		Capability:    cap.String(),
		CanComment:    cap.AtLeastCommenter(),
		CanAnswer:     cap.IsAnswerer(),
	}
}

// SurfaceAuthorizer returns a per-request predicate that answers, for one
// (owner, name), whether the request's viewer may see that repo's rows on a
// surface (multi-repo) listing. It is the surface-path analogue of [wrap]'s
// item-scoped per-repo gate: the listing enumerates MANY repos in one response,
// so the private-repo gate must be applied per row rather than once per route
// on login-presence alone. It reuses the SAME derivation wrap does —
// [Deriver.Capability] over the deskd-side tier, plus the cell's verified
// anonymous-read opt-in — never a parallel check.
//
// A row is authorized when EITHER its repo carries a verified anonymous-read
// opt-in (public, visible to everyone exactly as its item route serves
// anonymously) OR the viewer is authenticated and derives at least Commenter on
// that repo. Everything else — an unresolvable repo, an unauthenticated viewer
// on a private repo, an authenticated viewer with no read capability — is
// denied, fail-closed. Results are memoised for the request so a feed with many
// rows on one repo makes a single permission query.
func (g *Guard) SurfaceAuthorizer(r *http.Request) func(owner, name string) bool {
	sess := g.session(r)
	memo := map[string]bool{}
	return func(owner, name string) bool {
		if owner == "" || name == "" {
			return false // repo not resolvable from the trusted index: fail closed
		}
		key := owner + "/" + name
		if v, ok := memo[key]; ok {
			return v
		}
		allowed := g.authorizeSurfaceRepo(r, sess, owner, name)
		memo[key] = allowed
		return allowed
	}
}

// authorizeSurfaceRepo is the per-repo decision behind [SurfaceAuthorizer],
// mirroring the item-scoped branch of [wrap]: verified anonymous-read repos are
// visible to everyone; otherwise a live session deriving at least Commenter is
// required. It queries the same deskd-side tier — nothing the request supplied.
func (g *Guard) authorizeSurfaceRepo(r *http.Request, sess *Session, owner, name string) bool {
	if g.Policy != nil && g.Policy.AnonymousReadAllowed(owner, name) {
		return true
	}
	if sess == nil {
		return false
	}
	return g.derive(r, sess, owner, name).AtLeastCommenter()
}

// LocalStateAllowed is the per-item gate for a LOCAL-STATE write reached through
// a surface-scoped mount (the service contract increment 4: POST
// /v1/escalations/{id}/read under the "/v1/" tree). It mirrors [wrap]'s
// item-scoped branch for a route with Min=Commenter and AllowAnonymousRead=false
// — the gate the un-versioned POST /escalations/{id}/read marker is mounted
// behind — so the same write is never looser on the versioned seam:
//
//   - no live session → not authenticated (the caller answers 401), even on a
//     verified anonymous-read repo — unlike [SurfaceAuthorizer], which admits
//     that case for READS;
//   - an unresolvable repo → authenticated but not allowed (fail closed);
//   - otherwise allowed only when the ONE derivation ([Guard.derive]: rollout
//     gate, then the deskd-side permission query) yields at least Commenter.
func (g *Guard) LocalStateAllowed(r *http.Request, owner, name string) (authenticated, allowed bool) {
	sess := g.session(r)
	if sess == nil {
		return false, false
	}
	if owner == "" || name == "" {
		return true, false
	}
	return true, g.derive(r, sess, owner, name).AtLeastCommenter()
}

// SurfaceViewer reports just the authenticated login for a surface (multi-repo)
// view, where no single repo pins a tier.
func (g *Guard) SurfaceViewer(r *http.Request) ViewerInfo {
	sess := g.session(r)
	if sess == nil {
		return ViewerInfo{}
	}
	return ViewerInfo{Authenticated: true, Login: sess.Login}
}

// itemIDFromPath returns the first path segment after prefix. For prefix
// "/answer/" and path "/answer/abc/ack" it returns "abc"; the write path
// resolves the ref from that id against the trusted index, exactly as the
// handlers do — the guard and the handler agree on which item is addressed.
func itemIDFromPath(prefix, path string) string {
	rest := strings.TrimPrefix(path, prefix)
	if rest == path { // prefix did not match
		return ""
	}
	rest = strings.TrimPrefix(rest, "/")
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func denyUnauth(w http.ResponseWriter) {
	writeGate(w, http.StatusUnauthorized, map[string]any{
		"error":    "sign in with GitHub to view this",
		"login_at": "/auth/login",
	})
}

func denyForbidden(w http.ResponseWriter, need, have Capability) {
	writeGate(w, http.StatusForbidden, map[string]any{
		"error": fmt.Sprintf("this action needs %q; you are %q on this repo", need, have),
	})
}

func denyNotFound(w http.ResponseWriter) {
	writeGate(w, http.StatusNotFound, map[string]any{
		"error": "no such item in the index",
	})
}

func writeGate(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
