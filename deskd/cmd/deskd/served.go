// served.go — the mux deskd actually serves, built in ONE function that the
// binary and the route-walk test both call (the authorization contract).
//
// # Why this is a function and not a block inside main()
//
// The route-walk used to read the protected route TABLE (routes.go). The top
// mux main() served was assembled beside it, so a line such as
//
//	top.Handle("/v1/inbox", v1API)
//
// next to the /session mounts compiled, served a read route with no identity
// guard in front of it, and left every test green: the table never saw it.
// A walk that reads a description of the surface cannot see a mount that skips
// the description.
//
// [buildServedMux] is now the only place a pattern is mounted on the served
// mux. main() hands its result to the http.Server unchanged, and the walk in
// served_test.go calls the same function and probes every pattern it mounted
// with an unauthenticated request. Every mount except the few named in
// [unauthenticatedMounts] must answer with the identity wrap's refusal. A
// route that skips the wrap cannot satisfy that, whatever table it appears in.
//
// [servedMux] records each mount as it is made, so the walk enumerates the
// patterns the mux really holds, not a list kept beside it.
//
// TestServedMuxIsTheOnlyMux keeps the function the single choke point. It parses
// the package's non-test files (served_matcher_test.go) and fails the build on:
//
//   - a Handle, HandleFunc, NewServeMux or http.ServeMux (a call or a method
//     value) anywhere but buildServedMux and the recorder Handle/HandleFunc
//     (NewServeMux and http.ServeMux also in newServedMux), and on
//     http.DefaultServeMux anywhere;
//   - a touch of the servedMux's own .mux or .mounts fields anywhere but
//     newServedMux and the recorder methods, so not inside buildServedMux
//     either: only the recorder may reach the inner mux;
//   - a servedMux built any other way (literal, new, var, conversion), a
//     servedMux declared outside this file, an embedded or extra field, a
//     recorder method whose body is not the reviewed one (a mount that skips or
//     filters the record, a ServeHTTP that answers a path itself, a Mounts that
//     returns anything but a plain copy of the record), or a newServedMux whose
//     body is not exactly the one reviewed return;
//   - net/http's Serve, ServeTLS, ListenAndServe(TLS), called or taken as a
//     value, through ANY local name net/http is imported under, and a dot
//     import of it;
//   - an http.Server not built as the one composite literal in the package,
//     whose Handler must be the variable assigned from buildServedMux, and any
//     later assignment to a Handler field;
//   - any other binding of that variable's name (rebinding, shadowing, a
//     parameter or range variable of the same name);
//   - an import of net/http/fcgi, cgi, httptest or pprof, expvar, or
//     golang.org/x/net/http2 (and h2c).
//
// What it does not cover, and has no claim on: a mount reached through
// reflection or unsafe or by aliasing the variable through a pointer; serving
// through a package that is not on the import list above; a handler that
// serves a second route from inside a mounted handler; and control flow the
// AST match cannot name. It is a syntax match with no type information, so it
// matches selectors by name. The walk (TestServedRouteWalk) covers what
// buildServedMux mounts; this check covers where else a route could be added.
package main

import (
	"net/http"

	"github.com/medici-finance/assay/deskd/internal/identity"
	"github.com/medici-finance/assay/deskd/internal/serve"
)

// unauthenticatedMounts is the explicit allow-list of patterns the served mux
// may answer WITHOUT the identity wrap. Each entry needs a one-line reason.
// Everything else buildServedMux mounts must refuse an unauthenticated request
// the way the capability guard does (TestServedRouteWalk). Keep it small:
// adding an entry exempts a route from the identity chain, so it belongs in
// review, never in a drive-by.
var unauthenticatedMounts = map[string]string{
	"/auth/":    "OAuth sign-in and callback: the route a viewer uses to GET the session every gated route checks",
	"/static/":  "embedded CSS/JS for the login page; the web handler mounts only a file server under it (GET /static/), no data",
	"/session":  "reports {login, capability, live} so it can say \"anonymous\"; the answer handler refuses begin without auth",
	"/session/": "begin/end of a live-answer session ride /session; the answer handler fails begin closed without auth",
}

// servedMux is the top-level mux deskd serves. It wraps http.ServeMux so that
// every pattern mounted on it is also recorded, which is what lets the
// route-walk enumerate the mux's real contents. Handle and HandleFunc both go
// through the recorder, so either way of mounting a route is walked.
type servedMux struct {
	mux    *http.ServeMux
	mounts []string
}

func newServedMux() *servedMux { return &servedMux{mux: http.NewServeMux()} }

// Handle mounts h at pattern and records the pattern for the walk.
func (m *servedMux) Handle(pattern string, h http.Handler) {
	m.mux.Handle(pattern, h)
	m.mounts = append(m.mounts, pattern)
}

// HandleFunc mounts f at pattern and records the pattern for the walk.
func (m *servedMux) HandleFunc(pattern string, f func(http.ResponseWriter, *http.Request)) {
	m.Handle(pattern, http.HandlerFunc(f))
}

// ServeHTTP dispatches to the wrapped mux.
func (m *servedMux) ServeHTTP(w http.ResponseWriter, r *http.Request) { m.mux.ServeHTTP(w, r) }

// Mounts returns every pattern mounted so far, in mount order.
func (m *servedMux) Mounts() []string { return append([]string(nil), m.mounts...) }

// servedDeps is what buildServedMux mounts. main() builds the real handlers; the
// route-walk test passes sentinels that record being reached, plus a real
// identity.Guard, so the walk exercises the actual wrap.
type servedDeps struct {
	// Routes are the handlers of the capability-gated table (routes.go).
	Routes routeHandlers
	// Auth serves the OAuth sign-in routes under /auth/.
	Auth http.Handler
	// Static serves the login page's embedded assets under /static/.
	Static http.Handler
	// Session serves /session and /session/ (the answer handlers).
	Session http.Handler
	// Guard is the capability middleware every table route is wrapped in.
	Guard *identity.Guard
	// Origin is the request-origin floor (serve.OriginGuard) in front of every
	// table route and /session: a cross-site or rebound browser request is
	// refused before the capability gate or any handler sees it. Its zero value
	// admits loopback host names only.
	Origin serve.OriginGuard
}

// buildServedMux builds the mux deskd serves. It validates the protected route
// table first, and returns the error rather than mounting a table with a hole
// in it; main() fatals on that error.
//
// Mount only here. A pattern that is not an [unauthenticatedMounts] entry must
// go through d.Guard, which the loop at the end does for the whole table.
func buildServedMux(d servedDeps) (*servedMux, error) {
	table := protectedRoutes(d.Routes)
	routes := identityRoutes(table)
	if err := identity.ValidateRoutes(routes); err != nil {
		return nil, err
	}

	top := newServedMux()
	// Anonymous-reachable, data-free routes: sign-in and static assets.
	top.Handle("/auth/", d.Auth)
	top.Handle("/static/", d.Static)
	// P3: /mcp is not mounted here, ahead of the gate. It is an entry in the
	// protected table (routes.go), so it registers through the capability
	// middleware like every other data route AND carries its own caller
	// authentication inside the handler.
	// /session reports {login, capability, live}; begin/end ride it. Reachable
	// anonymously so it can report "anonymous"; begin fails closed without auth.
	// Its begin/end are form posts, so the origin floor fronts it too.
	top.Handle("/session", d.Origin.Wrap(d.Session))
	top.Handle("/session/", d.Origin.Wrap(d.Session))
	// Every data/write route through the origin floor, then the capability
	// gate. The agent surface additionally requires a JSON body: its writes
	// are App acts, and a text/plain body is the no-preflight cross-site shape.
	for _, mr := range table {
		gated := d.Guard.Handler(mr.Route)
		if mr.Kind == kindAgent {
			top.Handle(mr.Pattern, d.Origin.WrapJSON(gated))
			continue
		}
		top.Handle(mr.Pattern, d.Origin.Wrap(gated))
	}
	return top, nil
}
