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
// Membership is MACHINE-DERIVED / STAMPED, never self-declared — that is the whole
// governance point (a stream must not be able to self-declare itself critical).
// The derivation here is PURE and DETERMINISTIC over board-graph facts and stamped
// labels only: no wall clock, no network. The tier is an ORDERING KEY, not the drive
// term and not a metric — it is never exported.
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
//                        explicit state (criticalStampAuthoritiesSet=false): the arm
//                        grants nothing, and any stamp present is reported as
//                        could-not-check (criticalStampNotices), not silently ignored.
//                        Reads only the stamped label, never an intensity term.
//   3. high-unblocks   — blockedCount ≥ highUnblocksThreshold, over the reverse
//                        typed-depends graph (buildRevDeps/blockedCount). The
//                        dependency-edge reciprocity lint (brieffile.go) makes that
//                        count un-gameable: a manufactured one-sided inbound edge is a
//                        --lint PROBLEM, so blockedCount reflects genuine deps only.
//   4. reviewer-finding — this brief remediates an unresolved reviewer finding: the
//                        finding's control: names it (Finding.Control). Machine-
//                        derived: a reviewer files the finding, the brief author
//                        cannot. (An affects:-named brief is StaleRef-excluded from
//                        Next-up by design — see reviewerFindingCritical.)

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
// allowlist. With the placeholder allowlist empty, this is always false — the
// security arm is inert until a human ratifies the authority chain.
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
// unresolved reviewer finding. Machine-derived: a reviewer files the finding, so
// the brief author cannot self-select into the tier. Two linkages are read:
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
//     via control:. TestReviewerFindingArmReachesBoardViaControl pins both halves.
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
