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
// Defaulting to "known" is the fail-open this type exists to prevent
// (assay-toolkit#305).
type ClaimSource struct {
	Known  bool   // true ONLY when the remote branch list was actually read
	Reason string // why not, when !Known
}

// Notice renders the stderr NOTICE for a degraded run. Empty when claims are
// known.
func (c ClaimSource) Notice(eligible int) string {
	if c.Known {
		return ""
	}
	return fmt.Sprintf("claim filtering UNAVAILABLE — %s. Next-up's %d eligible brief(s) are an UNFILTERED SUPERSET: "+
		"briefs already claimed by an open origin branch are still listed. Do not dispatch from this run; "+
		"use --require-claims to fail instead of degrading (assay-toolkit#305)", c.reason(), eligible)
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
		"so some may already be in flight. Do not dispatch from this board until a run with a reachable `origin` regenerates it "+
		"(assay-toolkit#305).", c.reason())
}

func (c ClaimSource) reason() string {
	if r := strings.TrimSpace(c.Reason); r != "" {
		return r
	}
	return "the remote branch list was never read"
}

// resolveClaims builds the "stream/NN" claim set from open origin branches.
//
// It returns the ClaimSource alongside, so the caller can never confuse "read
// the remote, found no claims" with "could not read the remote" — the two
// produce an identical empty map, and treating them alike is what let a 3s
// timeout silently widen the board (assay-toolkit#305).
func resolveClaims(root string, streams []*Stream) (map[string]bool, ClaimSource) {
	branches, err := listRemoteBranches(root)
	if err != nil {
		return map[string]bool{}, ClaimSource{Reason: err.Error()}
	}
	claimed := claimedBriefs(streams, branches)
	// Placeholder claim-awareness (issue-loop/01): an open fix/issue-<NN>
	// branch excludes that issue's placeholder from Next-up (mm/08).
	for k := range claimedPlaceholders(streams, branches) {
		claimed[k] = true
	}
	return claimed, ClaimSource{Known: true}
}

// This repo's brief-branch conventions (CLAUDE.md "one brief = one branch = one PR"),
// verified against `git branch -r` at authoring time (issue #156):
//
//	fix|feature|feat/<stream>-<NN>[-slug]  e.g. fix/ledger-hardening-06-idempotency
//	docs/<stream>-<NN>[-slug]              e.g. docs/frontend-14-auth-e2e-closeout
//	chore/<stream>-<NN>[-slug]
//	<stream>/brief-<NN>[-slug]             e.g. methodology/brief-05, methodology-metrics/brief-04
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
