package main

import "github.com/medici-finance/assay/tools/desk/internal/loopengine"

// inflight.go — the in-flight-claim source for the advisory write-scope overlap warning.
// The overlap universe is the set of items already claimed for the
// same root — the SAME universe the dispatch-claim system tracks — carried as their derived
// write-scopes so `plan` can warn a candidate whose scopes overlap an in-flight one.
//
// OFFLINE. The default source reads the target repo's LOCAL `refs/dispatch/*` refs via
// loopengine.InFlightClaimScopes — never `git ls-remote`. Any failure yields no in-flight
// items: the warning is advisory, so an unreadable claim universe prints no warnings rather
// than failing the plan.

// inFlightSource returns the in-flight claim items (ID + derived write-scopes) for the root.
// A test injects InFlight; the default reads local refs/dispatch claims from Root.
func (f *FanoutLoop) inFlightSource() ([]loopengine.Item, error) {
	if f.InFlight != nil {
		return f.InFlight()
	}
	return loopengine.InFlightClaimScopes(f.Root), nil
}
