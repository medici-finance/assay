package identity

// ── The rollout gate (the service contract Task 9, the authorization contract) ─────────
//
// This file is the runtime half of the gate the authorization contract reports missing.
// Before it, the per-user identity path was live wherever the OAuth env vars
// happened to be set — one global switch, uniform across every cell. After
// it, the path is live only where a CELL opted in, and the env vars are a
// precondition for serving what the config already permitted.
//
// Read the gate as a ROLLOUT control, never as a security layer. It bounds
// WHERE the new path is live; it does not substitute for the capability
// middleware, which remains the brief's single-point-of-failure. Its whole
// contribution is that a cell nobody deliberately opted in cannot be reached
// above [Anonymous], no matter what the environment says.
//
// The ruled behaviour of an OFF cell (relayed direction, toolkit 1963,
// fork 2) is LOGIN-GATED WITH NO SIGN-IN ROUTE:
//
//   - the capability ladder middleware stays wrapped around every route, so
//     nothing renders pre-auth on a private cell exactly as before;
//   - the OAuth sign-in route is not served, so no viewer can rise above
//     anonymous;
//   - the pre-capability-model single-persona mode does NOT return. There is no
//     configured "whoever reaches the console IS operator" path to fall back
//     to, and reintroducing one on a sensitive-data identity surface — in a
//     process that must serve an ON cell beside an OFF one — was the second
//     blocker the ruling settled.

// RolloutGate answers, per repo, whether the per-user identity path is live —
// and, once, whether any cell at all opted in.
//
// It is an interface rather than a concrete type so [Guard] and [Auth] keep
// no dependency on the cells.yaml schema: deskd resolves cells to repos and
// hands the answer down. A NIL RolloutGate means the gate could not be
// resolved, and an unresolved gate is OFF — every consumer here treats nil as
// "no cell is enabled" rather than skipping the check.
type RolloutGate interface {
	// EnabledForRepo reports whether the cell owning owner/name has opted in.
	// It must fail CLOSED — false — for a repo no cell owns, for a repo whose
	// cell cannot be resolved, and for the empty repo.
	EnabledForRepo(owner, name string) bool
	// SignInServed reports whether ANY cell has opted in, i.e. whether the
	// OAuth sign-in routes serve at all. A deployment with no opted-in cell
	// serves no sign-in route.
	SignInServed() bool
}

// rolloutEnabledFor is the fail-closed read of a possibly-nil gate for one
// repo. Every capability derivation in this package goes through it, so
// "unwired means off" is stated once.
func rolloutEnabledFor(g RolloutGate, owner, name string) bool {
	if g == nil {
		return false
	}
	return g.EnabledForRepo(owner, name)
}

// rolloutSignInServed is the same fail-closed read for the sign-in routes.
func rolloutSignInServed(g RolloutGate) bool {
	if g == nil {
		return false
	}
	return g.SignInServed()
}
