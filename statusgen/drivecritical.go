package main

import (
	"fmt"
	"regexp"
	"sort"
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
// Membership is DERIVED here, never read from a "this is critical" flag: no field
// lets a brief or stream declare itself critical. The derivation is PURE and
// DETERMINISTIC over board-graph facts, linkage fields and stamped labels only: no
// wall clock, no network. The tier is an ORDERING KEY, not the drive term and not a
// metric — it is never exported.
//
// RESIDUAL (named, not derived — the driver's to accept at merge): the inputs the
// derivation reads are not all authenticated. Four of them are repo text that an
// ordinary reviewed, human-merged PR can write: a brief's own `issues:` list (arm
// 1's fix linkage), a README stamp cell (arm 2 — the authority NAME is checked
// against configuration, but not who wrote the cell), the `depends:`/`unblocks:`
// endpoints of a reciprocated edge (arm 3), and a findings entry's `control:`
// (arm 4 — findingEntry carries no actor). So the brief's "never
// self-declared" holds in the bounded sense that every linkage lands through review
// and a human merge, not as a structural guarantee. Only arm 1's red and arm 2's
// authority SET come from outside the tree (the caller's forge read and roster
// configuration). Arm 3's count reads only reciprocated edges
// (buildReciprocatedRevDeps), so a one-sided edge never counts, but both
// endpoints are PR-writable frontmatter (see arm 3).
//
// The four arms (brief-44's Scoring section):
//
//   1. main-red        — a main-red FIX. statusgen stays offline: whether main is
//                        red is an INJECTED input (--main-health, mainhealth.go) the
//                        caller supplies from its own forge read. A brief qualifies
//                        when main is red AND the brief addresses one of the named
//                        tracking issues (Brief.IssueRefs). Unset input is the
//                        could-not-check state: the arm cannot fire and, while a
//                        drive is active, the board says so — never a silent false.
//   2. security/leak   — a STAMPED label whose authority is RATIFIED. The authority
//                        set is roster configuration (ASSAY_CRITICAL_STAMP_AUTHORITIES,
//                        wired by main into criticalStampAuthorities). UNSET is an
//                        explicit state (criticalStampAuthoritiesSet is false): the arm
//                        grants nothing, and any stamp present is reported as
//                        could-not-check (criticalStampNotices), not silently ignored.
//                        Reads only the stamped label, never an intensity term.
//   3. high-unblocks   — blockedCount ≥ highUnblocksThreshold, over the
//                        RECIPROCATED reverse typed-depends graph
//                        (buildReciprocatedRevDeps): an edge A→B counts only when B
//                        also declares `unblocks: A`. The reciprocity lint
//                        (brieffile.go) reports a one-sided edge at NOTICE tier, so
//                        the lint alone cannot keep a one-sided edge out of the tier
//                        — the graph this arm reads does, by never walking one. It
//                        does NOT stop a change that writes BOTH endpoints (a
//                        brief's own `unblocks:` plus dependents declaring
//                        `depends:` on it, same stream included): both are
//                        PR-writable frontmatter, so such an edge still reaches the
//                        arm. That residual is named for the driver's ratification,
//                        not closed here.
//   4. reviewer-finding — this brief remediates an unresolved reviewer finding: the
//                        finding's control: names it (Finding.Control). The findings
//                        entry is a repo file with no actor field, so the linkage is
//                        only as trustworthy as the review that merges it (see
//                        RESIDUAL above). (An affects:-named brief is StaleRef-excluded
//                        from Next-up by design — see reviewerFindingCritical.)

// highUnblocksThreshold is the blockedCount at/above which a brief is a genuine
// high-unblocks fire (F-09 tunable heuristic, not a truth). 3 mirrors the
// unblocksWeight sizing in nextup.go, where a brief blocking ~3 others already
// out-scores a whole priority tier.
const highUnblocksThreshold = 3

// criticalStampAuthorities is the ratified authority set for the stamped
// security/critical arm — the authorities whose stamp may lift a brief into the
// hard critical tier. It is ROSTER CONFIGURATION, never a compiled-in identity:
// main() wires it from ASSAY_CRITICAL_STAMP_AUTHORITIES (rosterconfig.go) before
// the board is built, and criticalStampAuthoritiesSet records whether that key was
// configured at all. The compiled default is EMPTY and UNSET: a stamp then grants
// nothing, and criticalStampNotices names every stamp that could not be honoured
// and why, so "no authority configured" is a visible state rather than a silent
// "no authority". WHICH authority may stamp is the human's ratification (gate:
// human); this code only reads the answer.
var (
	criticalStampAuthorities    = map[string]bool{}
	criticalStampAuthoritiesSet bool
)

// wireCriticalStampAuthorities installs the ratified authority set from roster
// configuration (ASSAY_CRITICAL_STAMP_AUTHORITIES). Called by main()/runNextUp
// before the board is built, every run, so a prior value cannot leak in. An unset
// key — or a refused configuration — leaves the set EMPTY and UNSET: explicit, and
// reported per stamp by criticalStampNotices.
func wireCriticalStampAuthorities(cfg scanConfig) {
	m := map[string]bool{}
	set := cfg.CriticalStampAuthoritiesSet && len(cfg.Problems) == 0
	if set {
		for _, a := range cfg.CriticalStampAuthorities {
			m[a] = true
		}
	}
	criticalStampAuthorities = m
	criticalStampAuthoritiesSet = set
}

// criticalStampNotices reports every well-formed critical-security stamp on a
// brief in the given streams that grants nothing: the authority set is not
// configured (could-not-check — no ratified set to compare against), or the
// stamp's authority is not in it. Surfaced as --lint NOTICEs so an inert stamp is
// never mistaken for an honoured one. Silent when no brief carries a stamp.
func criticalStampNotices(streams []*Stream) []string {
	var notices []string
	for _, s := range streams {
		for _, b := range s.Briefs {
			auth, ok := securityCriticalStamp(b)
			if !ok {
				continue
			}
			id := s.Name + "/" + b.Num
			switch {
			case !criticalStampAuthoritiesSet:
				notices = append(notices, fmt.Sprintf(
					"%s carries critical-security(%s) but no critical-stamp authority set is configured (ASSAY_CRITICAL_STAMP_AUTHORITIES unset) — "+
						"could-not-check: the stamp grants nothing until a ratified authority set is configured", id, auth))
			case !criticalStampAuthorized(auth):
				notices = append(notices, fmt.Sprintf(
					"%s carries critical-security(%s) but %q is not in the configured critical-stamp authority set — the stamp grants nothing", id, auth, auth))
			}
		}
	}
	sort.Strings(notices)
	return notices
}

// securityCriticalStampRe parses a machine-readable security/critical stamp of the
// shape `critical-security(<authority>)` embedded in a brief's Reviewed/Verified
// cell — following the humanstamp.go precedent (a parseable stamp + an authority
// regex/config). The authority capture is a conservative identifier class so the
// stamp cannot smuggle arbitrary text.
var securityCriticalStampRe = regexp.MustCompile(`critical-security\(([0-9A-Za-z_.:-]+)\)`)

// criticalStampAuthorityRe is the same identifier class, anchored, used to validate
// each configured authority (ASSAY_CRITICAL_STAMP_AUTHORITIES) so the configured set
// can only name what a stamp can carry.
var criticalStampAuthorityRe = regexp.MustCompile(`^[0-9A-Za-z_.:-]+$`)

// criticalStampAuthoritiesEcho renders the effective-config value: "(unset)" is
// distinct from an explicit list, so the echo never presents "no authority
// configured" as "authority set configured empty".
func criticalStampAuthoritiesEcho(auths []string, set bool) string {
	if !set {
		return "(unset — the stamped-security arm grants nothing)"
	}
	return strings.Join(auths, ",")
}

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
// set configured from ASSAY_CRITICAL_STAMP_AUTHORITIES. With the key unset (the
// compiled default) the set is empty and this is always false — the security arm is
// inert until the driver's ratified authority set is configured.
func criticalStampAuthorized(authority string) bool {
	return criticalStampAuthorities[authority]
}

// mainRedCritical is the main-red arm: true iff the injected main-health input
// says main is RED and this brief addresses one of the tracking issues it names
// (Brief.IssueRefs — a placeholder's own issue, or a brief's resolved `issues:`).
// Reads only the injected input and the brief's declared issue linkage — never the
// wall clock or the network. With the input unset (could-not-check) or green it is
// false, and the could-not-check case is reported by nextUp, not swallowed here.
func mainRedCritical(b Brief, _ string) bool {
	if !activeMainHealth.red() {
		return false
	}
	for _, ref := range b.IssueRefs {
		if activeMainHealth.Refs[ref] {
			return true
		}
	}
	return false
}

// reviewerFindingCritical reports whether this brief is the REMEDIATION of an
// unresolved reviewer finding. The finding is a findings-register entry with no
// actor field, so nothing here checks WHO filed it: a PR that adds an entry naming
// a brief in control: lifts that brief, and the only gate is the review and human
// merge of that PR (the RESIDUAL named at the top of this file). Two linkages are
// read:
//
//   - control: — the finding names this brief (`<stream>/<NN>` or
//     `<stream>/brief-<NN>`) as the adaptation that closes it
//     (coder-skills-review/03). This is the REMEDIATION linkage, and it is the one
//     that reaches the board: the remediation brief is ordinary eligible work.
//   - affects: — the finding names this brief as AFFECTED. Honoured for
//     completeness, but such a brief is also stamped StaleRef, which is a hard
//     Next-up exclusion (nextup.go eligibleBase) BY DESIGN: a brief a finding says
//     is wrong must be reconciled before it is handed out, and the critical tier
//     is an ORDERING key over eligible picks, never an eligibility override. So an
//     affects-named brief never reaches the board through this arm — its fix does,
//     via control:. the reviewer-finding-reaches-board-via-control subtest pins both halves.
//
// Deliberately NOT broadcast from a bare-stream entry — that would mark every brief
// in the stream critical, the over-broad hammer applyFindings' anti-broadcast rule
// forbids.
func reviewerFindingCritical(findings []Finding, streamName, briefNum string) bool {
	id := streamName + "/" + briefNum
	names := func(ref string) bool {
		parts := strings.SplitN(strings.TrimSpace(ref), "/", 2)
		if len(parts) != 2 {
			return false // bare-stream annotation — never a per-brief critical flag
		}
		return parts[0]+"/"+strings.TrimPrefix(parts[1], "brief-") == id
	}
	for _, f := range findings {
		if f.Resolved {
			continue
		}
		if names(f.Control) {
			return true
		}
		for _, a := range f.Affects {
			if names(a) {
				return true
			}
		}
	}
	return false
}

// criticalTierArm returns the name of the critical-tier arm that qualifies this
// brief, or "" if none. Pure and deterministic over board-graph facts (the
// reciprocated blockedCount, findings) and stamped labels only — no wall clock, no
// network. Arms are evaluated in a fixed order so the attributed arm is stable;
// membership is what matters for the (CriticalTier, score) sort, and any single
// qualifying arm suffices.
//
// deps is the RECIPROCATED graph, not a count: the high-unblocks arm computes its
// own blockedCount from it, so no caller can hand the arm a count walked over
// one-sided edges (the score's buildRevDeps graph is a different type).
func criticalTierArm(b Brief, streamName string, deps reciprocatedRevDeps, findings []Finding) string {
	if mainRedCritical(b, streamName) {
		return "main-red"
	}
	if auth, ok := securityCriticalStamp(b); ok && criticalStampAuthorized(auth) {
		return "security"
	}
	if deps.count(streamName+"/"+b.Num) >= highUnblocksThreshold {
		return "high-unblocks"
	}
	if reviewerFindingCritical(findings, streamName, b.Num) {
		return "reviewer-finding"
	}
	return ""
}
