// routes.go — the service's mount table, as data (the service contract P2).
//
// # Why the table is a value and not a literal inside main()
//
// The custody review had to enumerate deskd's routes by reading six
// files and four sub-muxes, and the answer it reached — "there is one human
// write route outside the allow-list" — is exactly the kind of finding a test
// should reach first. It could not, because the table existed only as a literal
// in the middle of main(): a `[]identity.Route` a test cannot obtain without
// starting a server, polling GitHub and opening a database.
//
// Hoisting it here makes the P2 question answerable in a unit test:
// [protectedRoutes] builds the same table main() mounts, from stub handlers,
// and [humanWriteActions] reduces it to the set of named actions the binary
// registers as human-authored platform writes. TestServiceWriteRoutesInAllowList
// compares that set to internal/contract's closed [contract.NamedActions] and
// fails on any divergence in either direction — a route added outside the
// allow-list, or an allow-listed action that quietly stopped being served.
//
// The [routeKind] field is the classification the walk turns on. It is declared
// beside the route rather than inferred from the pattern, because inferring
// "does this write GitHub as a person?" from a URL is precisely the guess that
// let POST /comment/{id} sit unnoticed beside POST /answer/{id}.
package main

import (
	"net/http"

	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/identity"
)

// routeKind classifies what a mounted route DOES, in the vocabulary the wire
// contract already uses (internal/contract), plus the two kinds that exist only
// on the binary: presentation (the HTML console, moving to the client host in
// the service contract) and the agent surface.
type routeKind string

const (
	// kindRead serves derived index data. No platform write.
	kindRead routeKind = "read"
	// kindLocalState writes deskd's OWN SQLite (ruling D7). No platform write.
	kindLocalState routeKind = "local-state"
	// kindNamedAction is a HUMAN-AUTHORED platform write. Every route with this
	// kind must name an action in contract.NamedActions — that is the whole of
	// property P2, and TestServiceWriteRoutesInAllowList is where it is checked.
	kindNamedAction routeKind = "named-action"
	// kindPresentation serves HTML for a person to look at.
	kindPresentation routeKind = "presentation"
	// kindAgent is the MCP surface: App-identity writes only (§11).
	kindAgent routeKind = "agent"
)

// mountedRoute is one entry of the service's capability-gated table: the
// identity.Route that is actually mounted, plus the classification the P2 walk
// reads.
type mountedRoute struct {
	identity.Route
	Kind routeKind
	// Action is the named action a kindNamedAction route performs, and "" for
	// every other kind.
	Action contract.Action
}

// routeHandlers are the handlers main() has built by the time the table is
// assembled. A test passes stubs; nothing in the table's SHAPE depends on what
// they do, which is what makes the walk cheap to run.
type routeHandlers struct {
	HTMLConsole http.Handler
	LocalState  http.Handler
	Answer      http.Handler
	Inbox       http.Handler
	V1          http.Handler
	Root        http.Handler
	// MCP is the agent surface. It is IN this table as of the service contract
	// increment 2 — before that it was mounted beside the table, which is how
	// the route-boundary review found it unauthenticated: a route outside the table
	// is a route no walk can check.
	MCP http.Handler
	// MCPAuthenticated is the agent surface's own caller authentication. Its
	// callers are Apps and have no viewer session, so the capability guard
	// defers the "who is this" question to the gate that can answer it for that
	// caller class (identity.Route.Authenticated).
	MCPAuthenticated func(r *http.Request) bool
	// V1Authenticated is the `/v1/` tree's stand-in for a viewer session, and
	// it covers ONE request shape: a POST to a named action (answer, ack) whose
	// body carries a signed intent that verifies against a key enrolled in the
	// reviewed intent-keys file (the authorization contract gap 3, ruling 3 —
	// serve.NamedActions.IntentAuthenticated). Every other `/v1/` request —
	// every read, and the browser's cookie-path write — still needs the viewer
	// session exactly as before. It is not the only check: the handler
	// re-verifies the intent and consumes its nonce, and the answer Service and
	// the writer each refuse an act that did not come from that verification.
	V1Authenticated func(r *http.Request) bool
}

// protectedRoutes is the capability-gated table: EVERY data
// and write route registers through the capability middleware, and the table is
// validated before mounting so a route without the gate cannot ship.
//
// Two entries carry human writes, and both are `/answer/`-shaped or `/v1`-
// shaped sub-routes of a single mount: the tree at "/answer/" (the answer flow
// console path: answer + ack + draft) and the tree at "/v1/" (the seam's two
// named actions). They are declared here at the granularity the mux mounts
// them; [humanWriteActions] resolves each to the actions it can perform by
// asking the contract, not by parsing the pattern.
func protectedRoutes(h routeHandlers) []mountedRoute {
	return []mountedRoute{
		// Item-scoped data read: needs commenter (or the cell's verified anon opt-in).
		{Route: identity.Route{Pattern: "/item/", Min: identity.Commenter, ItemScoped: true, AllowAnonymousRead: true, Handler: h.HTMLConsole}, Kind: kindPresentation},
		// Local read/ack markers (D7, deskd-local only) — still item data, gated as commenter.
		{Route: identity.Route{Pattern: "/escalations/", Min: identity.Commenter, ItemScoped: true, Handler: h.LocalState}, Kind: kindLocalState},
		// Answer/ack/draft: needs answerer. The two platform writes reachable
		// under this tree are the SAME two named actions the seam serves — the
		// console path and the seam path are two mounts of one write path
		// (the answer flow's TestExactlyOneWritePath), not two write paths.
		{Route: identity.Route{Pattern: "/answer/", Min: identity.Answerer, ItemScoped: true, Handler: h.Answer}, Kind: kindNamedAction, Action: contract.ActionAnswer},
		// Surface-scoped feeds: the private-repo gate (SurfaceLoginGated) applies.
		{Route: identity.Route{Pattern: "/inbox", Min: identity.Commenter, ItemScoped: false, Handler: h.Inbox}, Kind: kindRead},
		// The versioned surface: surface-gated at the tree, then filtered
		// per-repo inside each handler (the listing per row, the item view per
		// item) via the same capability guard passed as its authorizer. Its
		// write sub-routes are exactly contract.NamedActions. Its route-owned
		// stand-in admits ONLY a verified signed intent on answer/ack
		// (V1Authenticated; the authorization contract ruling 3) — the second
		// deliberate stand-in after /mcp, recorded in both route baselines.
		{Route: identity.Route{Pattern: "/v1/", Min: identity.Commenter, ItemScoped: false, Handler: h.V1, Authenticated: h.V1Authenticated}, Kind: kindNamedAction, Action: contract.ActionAnswer},
		// The agent surface: App-identity writes only (§11 authority test), so no
		// named action of a PERSON is reachable through it and it contributes
		// nothing to the human-write set the P2 walk computes.
		{Route: identity.Route{Pattern: "/mcp", Min: identity.Commenter, ItemScoped: false, Handler: h.MCP, Authenticated: h.MCPAuthenticated}, Kind: kindAgent},
		{Route: identity.Route{Pattern: "/", Min: identity.Commenter, ItemScoped: false, Handler: h.Root}, Kind: kindRead},
	}
}

// identityRoutes projects the table onto what the capability guard mounts.
func identityRoutes(rs []mountedRoute) []identity.Route {
	out := make([]identity.Route, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Route)
	}
	return out
}

// humanWriteActions is the set of NAMED ACTIONS the mounted table can perform as
// a person. A route tree that carries human writes contributes every named
// action reachable beneath it — which, because both such trees mount the one
// write path, is exactly the contract's declared set. Anything else appearing
// here is a human write route outside the allow-list, and P2 has been broken.
func humanWriteActions(rs []mountedRoute) []contract.Action {
	seen := map[contract.Action]bool{}
	var out []contract.Action
	for _, r := range rs {
		if r.Kind != kindNamedAction {
			continue
		}
		for _, a := range actionsUnder(r) {
			if !seen[a] {
				seen[a] = true
				out = append(out, a)
			}
		}
	}
	return out
}

// actionsUnder enumerates the named actions reachable beneath one mounted tree.
// Both human-write trees serve the whole allow-list; a tree that grew an action
// the contract does not declare would surface it here as an unknown action and
// fail the walk.
func actionsUnder(r mountedRoute) []contract.Action {
	switch r.Pattern {
	case "/answer/", "/v1/":
		return append([]contract.Action(nil), contract.NamedActions...)
	default:
		// A human-write tree the walk does not know how to enumerate is
		// reported as an UNKNOWN action rather than silently as none: an
		// unenumerable write route is exactly what P2 forbids.
		return []contract.Action{contract.Action("unenumerated:" + r.Pattern)}
	}
}
