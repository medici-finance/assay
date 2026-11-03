package identity

import "fmt"

// route_walk.go — the service contract Task-1 safety net.
//
// Task 1 asked one dereferenced question first: does a cookie-bearing browser
// request reach a protected route under --auth=bearer? Reading [Guard.wrap]
// answers it without a line of new product code — [Guard.session] resolves
// the [SessionCookie] unconditionally, with no reference anywhere to
// [AuthMode] or the bearer [Validator]; [AuthMode] governs exactly two things,
// both elsewhere: [CheckBind] (may the process listen at all) and the MCP
// route's own [Route.Authenticated] stand-in (the service contract P3). So a valid
// session cookie is admitted on a bearer bind exactly as it is on a loopback
// one — Task 1 is a no-op, confirmed live by Verify row 2.
//
// [AssertNoWeakened] is the assertion the brief still asked for on the "if
// refused" branch, kept because a no-op answer today is not a promise it
// stays a no-op: it is the walk that, wired to a table, never lets that
// table widen [Route.Authenticated] onto a route that did not carry it, and
// never lets a route's [Route.Min] drop below what it required before. Both
// are exactly the shape the route-boundary review found: a route reachable with
// less than the gate it used to need.
//
// This package cannot import cmd/deskd (a main package), so it cannot walk
// [routes.go]'s real table itself — [TestRouteTableWalk] below only proves
// the WALKER algorithm correct against a hand-mirrored copy. The FORWARD
// GUARD against a future change to the actual mounted table —
// cmd/deskd/routes.go's protectedRoutes() — is
// cmd/deskd/routes_test.go's TestRouteTableWalkAgainstProduction, which calls
// [AssertNoWeakened] with identityRoutes(stubs()): the real table, unmirrored.

// RouteBaseline is what one route's gate is EXPECTED to be — the reference a
// route table is walked against. Keyed by [Route.Pattern] in the caller's
// table.
type RouteBaseline struct {
	Min           Capability
	Authenticated bool // true iff the route carries its own Route.Authenticated
}

// AssertNoWeakened walks current against baseline (by Pattern) and returns an
// error naming the FIRST route that regressed:
//
//   - a route present in baseline whose current Min is LOWER than the
//     baseline's — the gate on that route weakened;
//   - a route baseline recorded with no [Route.Authenticated] stand-in that
//     current now carries one for — a route-owned bypass appeared where the
//     session/capability gate used to be the only way in.
//
// A route in baseline that is absent from current, or a route in current with
// no baseline entry, is NOT an error here — the table's membership (which
// routes exist at all) is [ValidateRoutes]'s job; this walk is only about
// whether an EXISTING route's gate got weaker.
func AssertNoWeakened(current []Route, baseline map[string]RouteBaseline) error {
	byPattern := make(map[string]Route, len(current))
	for _, r := range current {
		byPattern[r.Pattern] = r
	}
	for pattern, want := range baseline {
		got, ok := byPattern[pattern]
		if !ok {
			continue // membership is ValidateRoutes' concern, not this walk's
		}
		if got.Min < want.Min {
			return fmt.Errorf("route %q now requires tier %q, weaker than the baseline %q — a route's gate must never lower", pattern, got.Min, want.Min)
		}
		if !want.Authenticated && got.Authenticated != nil {
			return fmt.Errorf("route %q gained a Route.Authenticated stand-in the baseline did not have — "+
				"a route-owned bypass appeared where the session/capability gate used to be the only way in", pattern)
		}
	}
	return nil
}
