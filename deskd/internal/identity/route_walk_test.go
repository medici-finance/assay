package identity

import (
	"net/http"
	"testing"
)

// productionRouteBaseline mirrors cmd/deskd/routes.go's protectedRoutes()
// table as of the service contract — the gate every one of those patterns is
// expected to carry. It is a HAND-MAINTAINED mirror (this package cannot
// import cmd/deskd, a main package); keep it in sync with routes.go when a
// route's Pattern, Min or Authenticated usage changes there.
func productionRouteBaseline() map[string]RouteBaseline {
	return map[string]RouteBaseline{
		"/item/":        {Min: Commenter, Authenticated: false},
		"/escalations/": {Min: Commenter, Authenticated: false},
		"/answer/":      {Min: Answerer, Authenticated: false},
		"/inbox":        {Min: Commenter, Authenticated: false},
		"/v1/":          {Min: Commenter, Authenticated: true}, // verified signed intent on answer/ack only (the authorization contract ruling 3)
		"/mcp":          {Min: Commenter, Authenticated: true}, // route-owned stand-in for App callers (§P3)
		"/":             {Min: Commenter, Authenticated: false},
	}
}

func productionRouteTable(baseline map[string]RouteBaseline) []Route {
	out := make([]Route, 0, len(baseline))
	for pattern, b := range baseline {
		r := Route{Pattern: pattern, Min: b.Min, ItemScoped: pattern == "/item/" || pattern == "/escalations/" || pattern == "/answer/", Handler: http.NotFoundHandler()}
		if b.Authenticated {
			r.Authenticated = func(*http.Request) bool { return true }
		}
		out = append(out, r)
	}
	return out
}

// TestRouteTableWalk is the service contract Verify row 7's second leg: the
// route-table-walk assertion Task 1 calls for. Task 1's own dereference found
// the admission already works with no code change (see route_walk.go's
// package comment), so this proves the WALKER ALGORITHM holds the invariant
// against a hand-mirrored copy of cmd/deskd/routes.go's table — both that
// today's mirrored table (constructed FROM productionRouteBaseline, so this
// subtest is self-consistent by construction) passes it clean, and that each
// of the two regressions the audit cares about is caught once mutated.
//
// This test does NOT observe cmd/deskd/routes.go's real protectedRoutes() —
// this package cannot import a main package — so it cannot by itself catch a
// future regression there. That forward guard is
// cmd/deskd/routes_test.go's TestRouteTableWalkAgainstProduction, which calls
// [AssertNoWeakened] with the real, unmirrored table.
func TestRouteTableWalk(t *testing.T) {
	baseline := productionRouteBaseline()

	t.Run("today's table, unweakened, passes", func(t *testing.T) {
		if err := AssertNoWeakened(productionRouteTable(baseline), baseline); err != nil {
			t.Fatalf("the unmutated production route table failed its own baseline walk: %v", err)
		}
	})

	t.Run("a route that lost Min is caught", func(t *testing.T) {
		mutated := productionRouteTable(baseline)
		for i := range mutated {
			if mutated[i].Pattern == "/answer/" {
				mutated[i].Min = Commenter // weakened from Answerer
			}
		}
		err := AssertNoWeakened(mutated, baseline)
		if err == nil {
			t.Fatal("a route whose Min dropped below baseline was not caught")
		}
	})

	t.Run("a route that gained Route.Authenticated is caught", func(t *testing.T) {
		mutated := productionRouteTable(baseline)
		for i := range mutated {
			if mutated[i].Pattern == "/inbox" {
				mutated[i].Authenticated = func(*http.Request) bool { return true } // the audit's exact shape: a bypass beside the guard
			}
		}
		err := AssertNoWeakened(mutated, baseline)
		if err == nil {
			t.Fatal("a route that gained an Authenticated stand-in outside the baseline was not caught")
		}
	})

	t.Run("a route missing from current is not itself an error (membership is ValidateRoutes' job)", func(t *testing.T) {
		mutated := productionRouteTable(baseline)
		trimmed := mutated[:0]
		for _, r := range mutated {
			if r.Pattern != "/v1/" {
				trimmed = append(trimmed, r)
			}
		}
		if err := AssertNoWeakened(trimmed, baseline); err != nil {
			t.Fatalf("a route's absence from current must not be flagged by this walk: %v", err)
		}
	})
}
