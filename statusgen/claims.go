package main

import (
	"fmt"
	"regexp"
	"strings"
)

// requireClaims turns a failed claim read into a hard failure instead of a
// loudly-degraded board (--require-claims). Package-level so main() can wire
// the flag and tests can set it directly, matching spanOfControl's pattern.
//
// Why the DEFAULT is loud-degraded rather than fail-closed: STATUS.md has a
// single writer (main's CI). Refusing to write leaves the PREVIOUS board on
// main, which reads as current and is a superset in exactly the same way —
// trading a labelled degradation for an unlabelled stale one. So the board
// still renders, but it renders WEARING the degradation. A caller that must
// not dispatch off an unfiltered board (a desk fanning work out) sets
// --require-claims and gets exit 1 with nothing written.
var requireClaims bool

// ClaimSource records whether brief-claim filtering could be established for a
// run, and — when it could not — why.
//
// The zero value is deliberately {Known:false}: a NextUp nobody told about
// claims has NOT been claim-filtered, and must present itself accordingly.
// Defaulting to "known" is the fail-open this type exists to prevent.
type ClaimSource struct {
	Known  bool   // true ONLY when the remote branch list was actually read
	Reason string // why not, when !Known

	// DecayReason records the SECOND thing that can go blind here, independently
	// of the first: the branch list was read, but the change-state read that
	// decays dead claims out of it was not. Empty when the decay ran.
	//
	// It is its own field because the two failures point in OPPOSITE directions
	// and a reader must not confuse them. !Known means the claim set is a
	// SUB-set (nothing was filtered) and the board is an unfiltered SUPERSET of
	// dispatchable briefs — dangerous to dispatch from. DecayReason means the
	// claim set is a SUPER-set (merged/closed corpses still count) and the board
	// is a SUBSET — safe to dispatch from, but silently holding real backlog
	// behind corpses, which is the silent-suppression class this generator
	// exists to prevent. Both are could-not-check; neither is a pass.
	DecayReason string
}

// ClaimView is the claim signal the Next-up capping reasons about: the
// "stream/NN" claim set AND the record of whether that set could actually be
// read. The two travel together because the map alone cannot tell "the remote
// was read and nothing is claimed" from "the remote could not be read" — both
// are an empty map. For a stream that declared `max-concurrent`, those two must
// produce OPPOSITE answers: the first leaves its full budget offerable, the
// second cannot see what is in flight and so must offer nothing.
//
// The zero value is deliberately unknown-and-empty, mirroring ClaimSource: a
// pick run nobody told about claims has not been claim-filtered, and inferring
// "known" from a non-nil map is exactly the fail-open this type exists to
// prevent.
type ClaimView struct {
	Claimed map[string]bool
	Source  ClaimSource
}

// KnownClaims wraps a claim set that WAS successfully read from the remote.
// Use it only where the read is known to have succeeded — everywhere else the
// ClaimView zero value is the honest answer.
func KnownClaims(claimed map[string]bool) ClaimView {
	return ClaimView{Claimed: claimed, Source: ClaimSource{Known: true}}
}

// Notice renders the stderr NOTICE for a degraded run. Empty when claims are
// known.
func (c ClaimSource) Notice(eligible int) string {
	if c.Known {
		return ""
	}
	return fmt.Sprintf("claim filtering UNAVAILABLE — %s. Next-up's %d eligible brief(s) are an UNFILTERED SUPERSET: "+
		"briefs already claimed by an open origin branch are still listed. Do not dispatch from this run; "+
		"use --require-claims to fail instead of degrading", c.reason(), eligible)
}

// Banner renders the in-board degradation notice for STATUS.md's Next-up
// section. Empty when claims are known. It goes in the ARTIFACT, not only on
// stderr: the board is read long after the run that produced it, by readers who
// never see the generator's output.
func (c ClaimSource) Banner() string {
	if c.Known {
		return ""
	}
	return fmt.Sprintf("> **DEGRADED — claim filtering did not run.** %s\n"+
		"> The rows below are an **unfiltered superset**: briefs already claimed by an open `origin` branch are NOT excluded, "+
		"so some may already be in flight. Do not dispatch from this board until a run with a reachable `origin` regenerates it.", c.reason())
}

func (c ClaimSource) reason() string {
	if r := strings.TrimSpace(c.Reason); r != "" {
		return r
	}
	return "the remote branch list was never read"
}

// DecayNotice renders the stderr/--lint could-not-check line for a run whose
// dead-claim decay could not look. Empty when the decay ran.
//
// It says "could-not-check" in its own words rather than wearing the same
// NOTICE: prefix as ordinary advisories, because it is the third state of a
// three-state instrument and must be distinguishable from both a clean run and a
// found defect (docs/three-state-instrument-rule.md).
func (c ClaimSource) DecayNotice() string {
	r := strings.TrimSpace(c.DecayReason)
	if r == "" {
		return ""
	}
	return fmt.Sprintf("could-not-check: claims not decayed — %s. Branches whose PR/merge request has already merged or "+
		"closed are still counted as claims, so the briefs they hold may be silently held back (HeldByStreamCap) and the "+
		"Next-up rows below are a SUBSET, not the full dispatchable set", r)
}

// DecayBanner renders the in-board could-not-check notice for STATUS.md's Next-up
// section. Empty when the decay ran. It goes in the ARTIFACT, not only on stderr:
// a warning printed by a generator nobody watches, while the board it wrote reads
// clean, is a TWO-state instrument — which is exactly how a forge whose decay
// could never run stayed invisible for six days.
func (c ClaimSource) DecayBanner() string {
	r := strings.TrimSpace(c.DecayReason)
	if r == "" {
		return ""
	}
	return fmt.Sprintf("> **COULD-NOT-CHECK — dead-claim decay did not run.** %s\n"+
		"> Open branches whose PR/merge request has already **merged or closed** are still counted as claims, so they keep "+
		"consuming their stream's dispatch cap. The rows below are a **subset**: briefs held behind those dead claims are "+
		"missing from this board, not absent from the backlog.", r)
}

// resolveClaims builds the "stream/NN" claim set from open origin branches.
//
// It returns the ClaimSource alongside, so the caller can never confuse "read
// the remote, found no claims" with "could not read the remote" — the two
// produce an identical empty map, and treating them alike is what let a 3s
// timeout silently widen the board.
func resolveClaims(root string, streams []*Stream) (map[string]bool, ClaimSource) {
	branches, err := listRemoteBranches(root)
	if err != nil {
		return map[string]bool{}, ClaimSource{Reason: err.Error()}
	}
	// Decay dead claims: a branch whose PR/merge request has already merged or
	// closed is not an in-flight claim — drop it before it consumes its stream's
	// dispatch cap. A failed change-state read leaves the full open-branch set
	// (see decayDeadClaims), so this only ever shrinks the claim set, never drops
	// a live claim — and hands back the reason, which travels on the ClaimSource
	// so the run and the board both wear the could-not-check.
	branches, decayReason := decayDeadClaims(root, branches)
	claimed := claimedBriefs(streams, branches)
	// Placeholder claim-awareness: an open fix/issue-<NN>
	// branch excludes that issue's placeholder from Next-up.
	for k := range claimedPlaceholders(streams, branches) {
		claimed[k] = true
	}
	return claimed, ClaimSource{Known: true, DecayReason: decayReason}
}

// This repo's brief-branch conventions (CLAUDE.md "one brief = one branch = one PR"),
// verified against `git branch -r` at authoring time:
//
//	fix|feature|feat/<stream>-<NN>[-slug]  e.g. fix/ledger-hardening-06-idempotency
//	docs/<stream>-<NN>[-slug]              e.g. docs/frontend-14-auth-e2e-closeout
//	chore/<stream>-<NN>[-slug]
//	<stream>/brief-<NN>[-slug]             e.g. ledger-hardening/brief-05, frontend/brief-04
//
// A branch matching neither shape returns ok=false and is ignored — it must never be
// mistaken for a claim on some unrelated brief.
var (
	branchPrefixNumRe  = regexp.MustCompile(`^(?:fix|feature|feat|docs|chore)/([a-z][a-z0-9]*(?:-[a-z0-9]+)*)-(\d{2}[a-z]?)(?:-.+)?$`)
	branchSlashBriefRe = regexp.MustCompile(`^([a-z][a-z0-9]*(?:-[a-z0-9]+)*)/brief-(\d{2}[a-z]?)(?:-.+)?$`)
)

// parseBranchClaim extracts a (streamToken, briefNum) pair from a branch name.
// streamToken is the raw token captured from the branch — it may be an abbreviation
// of the real stream name (e.g. "privacy" for "privacy-hardening") and must be
// resolved via resolveStreamToken before use as a claim key.
func parseBranchClaim(branch string) (streamToken, num string, ok bool) {
	for _, re := range []*regexp.Regexp{branchPrefixNumRe, branchSlashBriefRe} {
		if m := re.FindStringSubmatch(branch); m != nil {
			return m[1], m[2], true
		}
	}
	return "", "", false
}

// resolveStreamToken maps a parsed stream token to a known stream name: an exact
// match first, else a unique hyphen-boundary prefix match (branch names sometimes
// abbreviate the stream, e.g. "privacy" for "privacy-hardening"). Returns ok=false
// on no match or an ambiguous prefix match — a claim must never attach to the
// wrong stream.
func resolveStreamToken(streams []*Stream, token string) (name string, ok bool) {
	for _, s := range streams {
		if s.Name == token {
			return s.Name, true
		}
	}
	match, count := "", 0
	for _, s := range streams {
		if strings.HasPrefix(s.Name, token+"-") {
			match = s.Name
			count++
		}
	}
	if count == 1 {
		return match, true
	}
	return "", false
}

// claimedBriefs maps open remote branches to "stream/NN" claim keys, for every
// branch that matches a known brief-branch convention AND resolves to a known
// stream. Branches that don't match, or resolve ambiguously, are silently
// skipped — never hide an unrelated brief.
func claimedBriefs(streams []*Stream, branches []string) map[string]bool {
	claimed := map[string]bool{}
	for _, b := range branches {
		token, num, ok := parseBranchClaim(b)
		if !ok {
			continue
		}
		name, ok := resolveStreamToken(streams, token)
		if !ok {
			continue
		}
		claimed[name+"/"+num] = true
	}
	return claimed
}
