package main

import (
	"errors"
	"fmt"

	"github.com/medici-finance/assay/tools/desk/internal/comms"
)

// routing.go — commsloop's OWN, INDEPENDENT re-check of the lane ACL.
//
// DEFENSE IN DEPTH (this gateway's single-point-of-failure note): the
// gateway's pre-check pipeline (cmd/commsgw/precheck.go) already checks the
// lane ACL before a message is ever queued. This file is a SECOND, INDEPENDENT
// check of the exact same rule, run again here at the routing boundary — a
// DIFFERENT file, a DIFFERENT process — so that a message which somehow
// entered the accepted-queue WITHOUT clearing the gateway's own check (a
// planted or bypassed message — TestBypass proves this directly by writing
// straight to the accepted-queue, skipping commsgw entirely) is still refused
// here rather than silently routed. Two checks that must independently agree
// is what "the Verify table must include a bypass row proving an
// ACL-violating message... still refuses at the routing boundary (two
// different checks, two different files)" asks for.
//
// This is deliberately NOT a shared function commsgw and commsloop both call:
// a shared function would collapse back to ONE check that happens to run
// twice, which proves nothing about a bypass of the first. The logic is
// small enough (a direct read of comms.ACL.Allow plus the same pair/verb
// split precheck.go's checkLane uses) that duplicating it here is the
// correct, load-bearing choice, not an oversight to refactor away.

// ErrRoutingLaneDenied is commsloop's own lane-ACL refusal, distinct from any
// of commsgw's precheck errors (they live in a different package/binary
// entirely, so there was never a risk of them being the same Go value, but
// the NAME is deliberately its own too).
var ErrRoutingLaneDenied = errors.New("commsloop: lane ACL denies this message at the routing boundary")

// checkLaneAtRoutingBoundary re-validates env against acl, independently of
// whatever commsgw's own precheck already decided.
func checkLaneAtRoutingBoundary(env *comms.Envelope, acl *comms.ACL) error {
	if acl == nil {
		return fmt.Errorf("%w: nil lane ACL (fail closed, never default-allow)", ErrRoutingLaneDenied)
	}
	if env.From.Cell == env.To.Cell {
		if !acl.Allow(env.From.Cell, env.From.Role, env.Verb, env.To.Cell, env.To.Role) {
			return fmt.Errorf("%w: within-cell (%s -> %s) verb %q", ErrRoutingLaneDenied, env.From.Role, env.To.Role, env.Verb)
		}
		return nil
	}
	pairOK := false
	for _, p := range acl.CrossPairs {
		if p.From == env.From.Role && p.To == env.To.Role {
			pairOK = true
			break
		}
	}
	if !pairOK {
		return fmt.Errorf("%w: cross-cell pair (%s -> %s) is not permitted", ErrRoutingLaneDenied, env.From.Role, env.To.Role)
	}
	verbOK := false
	for _, v := range acl.CrossVerbs {
		if v == env.Verb {
			verbOK = true
			break
		}
	}
	if !verbOK {
		return fmt.Errorf("%w: cross-cell verb %q is not in the allow-set", ErrRoutingLaneDenied, env.Verb)
	}
	return nil
}

// isReportClass is the (documented, reviewable) JUDGMENT CALL commsloop makes
// mechanically until the prose router lands: which verbs are pure REPORTS —
// read-only, answered once, no further action or disposition tracking needed
// — versus which need the not-yet-built router's routing decision.
// status/metrics/help-offered are exactly the three ruled cross-cell verbs
// (#1896) that are answered and done; focus-on is deliberately EXCLUDED
// because it carries an ongoing disposition field
// (taken-up/declined/no-answer-yet) a human tracks, and every within-cell verb
// (handoff/notify/ask) is excluded because routing a work item or a question
// to the right role is exactly the judgment the prose router exists to make
// — a report-class shortcut must never quietly stand in for that decision.
func isReportClass(env *comms.Envelope) bool {
	switch env.Verb {
	case "status", "metrics", "help-offered":
		return true
	default:
		return false
	}
}
