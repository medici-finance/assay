package main

import (
	"regexp"
	"strings"
)

// drivecritical.go — phase 3: the HARD, NEVER-BURIED critical tier.
//
// The Next-up board sorts lexicographically by (CriticalTier, Total()): a critical
// member ranks ABOVE every score, so NO intensity — surge included — can bury a
// live fire. The critical tier is applied ONLY when a drive is active (nextup.go):
// with no drive there is nothing to bury and the ordinary score already orders the
// board, so a no-drive board stays byte-identical to the pre-drives baseline.
//
// Membership is MACHINE-DERIVED / STAMPED, never self-declared — that is the whole
// governance point (a stream must not be able to self-declare itself critical).
// The derivation here is PURE and DETERMINISTIC over board-graph facts and stamped
// labels only: no wall clock, no network. The tier is an ORDERING KEY, not the drive
// term and not a metric — it is never exported.
//
// The four arms (brief-44's Scoring section):
//
//   1. main-red        — DEFERRED. statusgen is byte-stable and offline; it cannot
//                        poll live GitHub CI, and adding a network/live-CI read would
//                        violate the deterministic-offline invariant. The arm is a
//                        documented SEAM (mainRedCritical, always false) pending a
//                        human ruling on an in-tree, machine-derived main-red marker.
//   2. security/leak   — a STAMPED label whose authority is RATIFIED. The mechanism
//                        (parse + authority check) ships here; the authority set is a
//                        config PLACEHOLDER (criticalStampAuthorities), EMPTY by
//                        default, so the arm is inert until a human ratifies WHO may
//                        stamp. Reads only the stamped label, never an intensity term.
//   3. high-unblocks   — blockedCount ≥ highUnblocksThreshold, over the reverse
//                        typed-depends graph (buildRevDeps/blockedCount). The
//                        dependency-edge reciprocity lint (brieffile.go) makes that
//                        count un-gameable: a manufactured one-sided inbound edge is a
//                        --lint PROBLEM, so blockedCount reflects genuine deps only.
//   4. reviewer-finding — an unresolved reviewer finding names this brief (the
//                        existing Finding.Affects/StaleRef linkage). Machine-derived:
//                        a reviewer files the finding, the brief author cannot.

// highUnblocksThreshold is the blockedCount at/above which a brief is a genuine
// high-unblocks fire (F-09 tunable heuristic, not a truth). 3 mirrors the
// unblocksWeight sizing in nextup.go, where a brief blocking ~3 others already
// out-scores a whole priority tier.
const highUnblocksThreshold = 3

// criticalStampAuthorities is the CONFIG PLACEHOLDER for the stamped security/
// critical arm's authority chain — the set of authorities whose stamp may lift a
// brief into the hard critical tier. It is EMPTY by design: this phase ships the
// MECHANISM, and the HUMAN ratifies WHO is authorized (gate: human). While empty,
// the security arm is INERT — a stamp with no authorized authority grants nothing,
// which is the safe default (a placeholder must never silently authorize anyone).
//
// Ratification wires the real authority here (or, better, threads it from repo
// config so it is not a source edit). Example, deliberately COMMENTED OUT:
//
//	var criticalStampAuthorities = map[string]bool{
//		"security-desk": true,
//	}
var criticalStampAuthorities = map[string]bool{}

// securityCriticalStampRe parses a machine-readable security/critical stamp of the
// shape `critical-security(<authority>)` embedded in a brief's Reviewed/Verified
// cell — following the humanstamp.go precedent (a parseable stamp + an authority
// regex/config). The authority capture is a conservative identifier class so the
// stamp cannot smuggle arbitrary text.
var securityCriticalStampRe = regexp.MustCompile(`critical-security\(([0-9A-Za-z_.:-]+)\)`)

// securityCriticalStamp returns the stamped authority and true iff a well-formed
// security/critical stamp is present on the brief. It reads ONLY the stamped label
// — never an intensity/surge term. Presence alone grants nothing: the authority
// must also be ratified (criticalStampAuthorized).
func securityCriticalStamp(b Brief) (string, bool) {
	for _, cell := range []string{b.Reviewed, b.Verified} {
		if m := securityCriticalStampRe.FindStringSubmatch(cell); m != nil {
			return m[1], true
		}
	}
	return "", false
}

// criticalStampAuthorized reports whether a stamp authority is in the ratified
// allowlist. With the placeholder allowlist empty, this is always false — the
// security arm is inert until a human ratifies the authority chain.
func criticalStampAuthorized(authority string) bool {
	return criticalStampAuthorities[authority]
}

// mainRedCritical is the DEFERRED main-red arm. statusgen is deterministic and
// offline (byte-identical baseline, --lint-clean) and cannot read live GitHub CI,
// so there is no in-tree, machine-derived "main is red" signal to key on. Rather
// than add a network/live-CI read (which would break the offline invariant) or
// invent a governance marker, the arm is deferred: it always reports false and is a
// clearly documented SEAM. When a human ratifies an in-tree, machine-derived
// main-red marker, its check goes here — reading that marker, never the wall clock
// or the network.
func mainRedCritical(_ Brief, _ string) bool {
	return false
}

// reviewerFindingCritical reports whether an UNRESOLVED reviewer finding names this
// brief via its Affects list (`<stream>/<NN>` or `<stream>/brief-<NN>`) — the same
// linkage applyFindings uses. Machine-derived: a reviewer files the finding, so the
// brief author cannot self-select into the tier.
//
// NOTE (documented seam): a brief-SPECIFIC finding also stamps StaleRef, which is a
// hard Next-up EXCLUSION (nextup.go eligible()), so in the current eligibility model
// a finding-named brief is not itself an eligible pick. The linkage is honored here
// so the arm is correct-by-construction and unit-testable, and so it lights up the
// moment a finding-named brief is eligible by any future path. Deliberately NOT
// broadcast from a bare-stream `affects:` entry — that would mark every brief in the
// stream critical, the same over-broad hammer applyFindings' anti-broadcast rule
// forbids.
func reviewerFindingCritical(findings []Finding, streamName, briefNum string) bool {
	id := streamName + "/" + briefNum
	for _, f := range findings {
		if f.Resolved {
			continue
		}
		for _, a := range f.Affects {
			parts := strings.SplitN(a, "/", 2)
			if len(parts) != 2 {
				continue // bare-stream annotation — never a per-brief critical flag
			}
			num := strings.TrimPrefix(parts[1], "brief-")
			if parts[0]+"/"+num == id {
				return true
			}
		}
	}
	return false
}

// criticalTierArm returns the name of the critical-tier arm that qualifies this
// brief, or "" if none. Pure and deterministic over board-graph facts (blockedCount,
// findings) and stamped labels only — no wall clock, no network. Arms are evaluated
// in a fixed order so the attributed arm is stable; membership is what matters for
// the (CriticalTier, score) sort, and any single qualifying arm suffices.
func criticalTierArm(b Brief, streamName string, blockedCount int, findings []Finding) string {
	if mainRedCritical(b, streamName) {
		return "main-red"
	}
	if auth, ok := securityCriticalStamp(b); ok && criticalStampAuthorized(auth) {
		return "security"
	}
	if blockedCount >= highUnblocksThreshold {
		return "high-unblocks"
	}
	if reviewerFindingCritical(findings, streamName, b.Num) {
		return "reviewer-finding"
	}
	return ""
}
