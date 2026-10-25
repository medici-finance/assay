package main

import (
	"net/http"
	"sort"
	"testing"

	"github.com/medici-finance/assay/deskd/internal/contract"
	"github.com/medici-finance/assay/deskd/internal/identity"
)

// stubs builds a route table over inert handlers. The walk asks what the table
// IS, never what its handlers do.
func stubs() []mountedRoute {
	h := http.NotFoundHandler()
	return protectedRoutes(routeHandlers{
		HTMLConsole: h, LocalState: h, Answer: h, Inbox: h, V1: h, Root: h, MCP: h,
		MCPAuthenticated: func(*http.Request) bool { return true },
		V1Authenticated:  func(*http.Request) bool { return true },
	})
}

// productionRouteBaseline is the gate every pattern in protectedRoutes() is
// EXPECTED to carry — hand-maintained beside protectedRoutes() itself, in the
// same package and file a route change edits, so keeping it in sync is a
// same-diff discipline rather than a cross-package one.
//
// This is the baseline TestRouteTableWalkAgainstProduction below walks the
// REAL table against (via identityRoutes(stubs()), not a table synthesized
// from this map) — the pr-review-desk finding on the service contract was that
// internal/identity's own TestRouteTableWalk only ever compared a baseline
// against a table built FROM that same baseline, so it could not observe a
// regression in cmd/deskd/routes.go's actual protectedRoutes(). This does.
func productionRouteBaseline() map[string]identity.RouteBaseline {
	return map[string]identity.RouteBaseline{
		"/item/":        {Min: identity.Commenter, Authenticated: false},
		"/escalations/": {Min: identity.Commenter, Authenticated: false},
		"/answer/":      {Min: identity.Answerer, Authenticated: false},
		"/inbox":        {Min: identity.Commenter, Authenticated: false},
		"/v1/":          {Min: identity.Commenter, Authenticated: true}, // verified signed intent on answer/ack only (the authorization contract ruling 3)
		"/mcp":          {Min: identity.Commenter, Authenticated: true}, // route-owned stand-in for App callers (§P3)
		"/":             {Min: identity.Commenter, Authenticated: false},
	}
}

// TestRouteTableWalkAgainstProduction is the forward guard the PR body and
// the served route contract describe: a FUTURE change to
// cmd/deskd/routes.go's protectedRoutes() that lowers a route's Min tier, or
// adds a route-owned Authenticated bypass beside the session gate, must fail
// a test — not just the algorithm-level walker test in internal/identity.
//
// The first subtest proves the wiring itself: identityRoutes(stubs()) is the
// REAL mounted table (built by protectedRoutes(), the same function main()
// calls), so if a future edit to routes.go weakens a route, THIS subtest
// fails with no mutation needed — the table the walk reads already carries
// the regression. The other two subtests reproduce, on that same real table,
// the two regression shapes the route-boundary review found (a lowered Min, and an
// added Authenticated bypass), confirming AssertNoWeakened catches each one
// here rather than only against internal/identity's hand-mirrored copy.
func TestRouteTableWalkAgainstProduction(t *testing.T) {
	baseline := productionRouteBaseline()

	t.Run("today's real mounted table, unweakened, passes", func(t *testing.T) {
		if err := identity.AssertNoWeakened(identityRoutes(stubs()), baseline); err != nil {
			t.Fatalf("the mounted route table (protectedRoutes()) failed its own baseline walk: %v", err)
		}
	})

	t.Run("a route that lost Min is caught on the real table", func(t *testing.T) {
		mutated := identityRoutes(stubs())
		for i := range mutated {
			if mutated[i].Pattern == "/answer/" {
				mutated[i].Min = identity.Commenter // weakened from Answerer
			}
		}
		if err := identity.AssertNoWeakened(mutated, baseline); err == nil {
			t.Fatal("a route whose Min dropped below baseline was not caught against the real production table")
		}
	})

	t.Run("a route that gained Route.Authenticated is caught on the real table", func(t *testing.T) {
		mutated := identityRoutes(stubs())
		for i := range mutated {
			if mutated[i].Pattern == "/inbox" {
				mutated[i].Authenticated = func(*http.Request) bool { return true } // the audit's exact shape: a bypass beside the guard
			}
		}
		if err := identity.AssertNoWeakened(mutated, baseline); err == nil {
			t.Fatal("a route that gained an Authenticated stand-in outside the baseline was not caught against the real production table")
		}
	})
}

// TestServiceWriteRoutesInAllowList is property P2, stated as the brief states
// it: "the set of human-authored write routes registered on cmd/deskd equals
// the allow-list" — {answer, ack, send, session} since the service contract.
//
// It fails in BOTH directions, which is what makes it more than a spot check —
// a route added outside the allow-list fails it, and an allow-listed action
// that quietly stopped being served fails it too. The route-boundary review's
// counter-example (POST /comment/{id}, a free-text human write) would appear
// here as an extra action; that route is now out of the binary's import graph
// entirely (TestServiceExcludesFreeTextCommentPath).
func TestServiceWriteRoutesInAllowList(t *testing.T) {
	got := humanWriteActions(stubs())
	want := append([]contract.Action(nil), contract.NamedActions...)

	sortActions(got)
	sortActions(want)
	if len(got) != len(want) {
		t.Fatalf("the service registers human write actions %v, want exactly the enumerated allow-list %v (the service contract P2)", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("the service registers human write actions %v, want exactly the enumerated allow-list %v — %q is outside it (the service contract P2)", got, want, got[i])
		}
	}
	for _, a := range got {
		if !contract.IsNamedAction(a) {
			t.Fatalf("the service registers %q as a human write, which the contract's closed allow-list does not declare — "+
				"widening the surface is a contract change (internal/contract), never a mount", a)
		}
	}
}

// TestRouteWalkAllUnderMiddleware is the brief's route-walk row: every entry in
// the mounted table registers through the capability gate at at least
// Commenter, and no write route carries the anonymous-read shortcut. It is the
// same check main() fatals on at startup — asserted here so a hole fails the
// BUILD rather than the first boot that happens to be watched.
//
// It reads the TABLE, so it cannot see a pattern mounted beside the table
// (the authorization contract). The walk of the mux deskd actually serves is
// TestServedRouteWalk in served_test.go.
func TestRouteWalkAllUnderMiddleware(t *testing.T) {
	rs := stubs()
	if err := identity.ValidateRoutes(identityRoutes(rs)); err != nil {
		t.Fatalf("the mounted route table does not validate — a data or write route is missing its gate: %v", err)
	}
	for _, r := range rs {
		if r.Kind == kindNamedAction {
			if r.Min < identity.Answerer && r.Pattern == "/answer/" {
				t.Errorf("route %q performs human writes at min tier %q — the answer tree must require %q", r.Pattern, r.Min, identity.Answerer)
			}
			if r.AllowAnonymousRead {
				t.Errorf("route %q performs human writes AND allows anonymous read — a write route may never carry that shortcut", r.Pattern)
			}
		}
	}
}

// TestNoGeneralWriteRouteIsDeclared pins the enumeration Verify row 4 performs
// against a running service: the contract declares no general comment / close /
// react route on any resource, at any version. A general write route added to
// the contract would be caught here before any handler for it could be mounted.
// The admitted actions are exactly the ratified four (the service contract, option
// 1: {answer, ack, send, session}, frozen behind tests); a fifth fails here.
func TestNoGeneralWriteRouteIsDeclared(t *testing.T) {
	for _, r := range contract.Routes {
		if r.Kind != contract.KindNamedAction {
			continue
		}
		switch r.Action {
		case contract.ActionAnswer, contract.ActionAck, contract.ActionSend, contract.ActionSession:
		default:
			t.Errorf("the contract declares a human write route %s %s for action %q — the allow-list is closed at {answer, ack, send, session}", r.Method, r.Path, r.Action)
		}
	}
	for _, r := range contract.Routes {
		for _, general := range []string{"/comment", "/close", "/react"} {
			if len(r.Path) >= len(general) && r.Path[len(r.Path)-len(general):] == general {
				t.Errorf("the contract declares %s %s — there is no general comment/close/react route on any resource (the service contract P2)", r.Method, r.Path)
			}
		}
	}
}

func sortActions(a []contract.Action) {
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
}

// TestAgentSurfaceIsInTheTable is the ordering half of P3. Before increment 2
// /mcp was mounted on the top mux ahead of the capability loop — outside the
// table entirely — which is how the route-boundary review found it authenticating
// nothing. Being IN the table is what makes it walkable at all.
func TestAgentSurfaceIsInTheTable(t *testing.T) {
	rs := stubs()
	var found *mountedRoute
	for i := range rs {
		if rs[i].Pattern == "/mcp" {
			found = &rs[i]
		}
	}
	if found == nil {
		t.Fatal("/mcp is not in the protected route table — a route outside the table is a route no walk can check (the service contract P3)")
	}
	if found.Kind != kindAgent {
		t.Errorf("/mcp is classified %q, want %q", found.Kind, kindAgent)
	}
	if found.AllowAnonymousRead {
		t.Error("/mcp allows anonymous read — the agent surface authenticates its caller")
	}
	if found.Min < identity.Commenter {
		t.Errorf("/mcp registers at min tier %q — every route in this table gates at %q or above", found.Min, identity.Commenter)
	}
}

// TestAgentSurfaceContributesNoHumanWrite: the MCP surface authors as the desk
// App and never as a person (§11), so bringing it into the table must not have
// widened the human-write set. This is the paired control for the P2 walk — a
// walk that counted the agent surface as a human write would be measuring the
// wrong thing.
func TestAgentSurfaceContributesNoHumanWrite(t *testing.T) {
	with := humanWriteActions(stubs())
	var without []mountedRoute
	for _, r := range stubs() {
		if r.Pattern != "/mcp" {
			without = append(without, r)
		}
	}
	if len(with) != len(humanWriteActions(without)) {
		t.Fatalf("the agent surface changed the human-write set (%v) — no MCP tool may author as a person (§11)", with)
	}
}
