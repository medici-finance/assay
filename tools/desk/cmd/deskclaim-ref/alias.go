package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/medici-finance/assay/tools/desk/internal/deskkit"
)

// Per-PR key spellings (medici-finance/assay#3700).
//
// A claim on one PR is keyed `<label>--pr-<N>` (correctness lane) or `<label>--pr-<N>--security`
// (security lane), where <label> is whatever short repo spelling the CALLER uses. Two callers
// that spell the label differently write two refs under refs/dispatch/, so `acquire` on the
// second read the PR as free although it was actively owned, and two sessions drove one PR.
//
// The claim lives in the PR's own repo and a PR number is unique within a repo, so every claim
// with the same <N> and the same lane suffix IS the same claim; no alias table is needed. The
// lanes stay separate (the suffix, not the caller, decides the lane), and keys that are not
// per-PR keys have no alias. This mirrors tools/dispatch-claim.sh in repos that ship it: the two
// tools speak one protocol, so they must agree on what counts as the same claim.

// Scope of "same claim": only the two key shapes above. deskkit's review-claim family helpers
// (ReviewClaimFamilies, ReviewClaimKeyIsSecurity) are a separate, wider grammar for review keys
// (any --<lane> suffix, case-insensitive security); a key with another suffix, or an upper-case
// `PR`, is NOT aliased here and keeps its own claim. Widening this to one shared parser is a
// follow-up; until then the two definitions differ on purpose and this is the narrower one.
var prKeyRe = regexp.MustCompile(`^.+--pr-([0-9]+)(--security)?$`)

// prKeyParts returns the PR number and lane ("" or "--security") of a per-PR key. The number is
// normalised: `pr-05` and `pr-5` are the same PR (an all-zero number reads as "0"). It stays a
// string, so a long number cannot overflow.
func prKeyParts(id string) (num, lane string, ok bool) {
	m := prKeyRe.FindStringSubmatch(id)
	if m == nil {
		return "", "", false
	}
	num = strings.TrimLeft(m[1], "0")
	if num == "" {
		num = "0"
	}
	return num, m[2], true
}

// aliasIDs lists the OTHER claim ids spelling the same PR and lane as id, in sorted order. The
// status is ClaimReadHeld when the claim set was read (even if no alias exists) and
// ClaimReadUnverifiable when it could not be: never "assume none".
func aliasIDs(id string) ([]string, deskkit.ClaimReadStatus) {
	num, lane, ok := prKeyParts(id)
	if !ok {
		return nil, deskkit.ClaimReadHeld
	}
	ids, status := store.List()
	if status == deskkit.ClaimReadUnverifiable {
		return nil, status
	}
	var out []string
	for _, other := range ids {
		if other == id {
			continue
		}
		if n, l, ok := prKeyParts(other); ok && n == num && l == lane {
			out = append(out, other)
		}
	}
	sort.Strings(out)
	return out, deskkit.ClaimReadHeld
}

// claimLive reports whether the claim under id is LIVE: its recorded branch exists on the remote
// (branch-as-claim), or it is inside its state's TTL. live=false with verifiable=true is a free
// or expired claim; verifiable=false is could-not-check.
func claimLive(id string) (ref deskkit.ClaimStoreRecord, live, verifiable bool) {
	ref, status := store.Read(id)
	switch status {
	case deskkit.ClaimReadFree:
		return ref, false, true
	case deskkit.ClaimReadUnverifiable:
		return ref, false, false
	}
	age, ok := ageMinutes(ref.Date)
	if !ok {
		return ref, false, false
	}
	if exists, ok := store.BranchExists(fieldOf(ref.Msg, "branch")); ok && exists {
		return ref, true, true
	}
	ttl := dispatchedTTL()
	if fieldOf(ref.Msg, "state") == "claimed" {
		ttl = claimedTTL()
	}
	return ref, age < ttl, true
}

// checkAliases refuses when another spelling of the same PR and lane is live. phase "pre" runs
// before the create. Phase "post" runs after our create succeeded: two spellings can both pass
// their pre-check before either creates, and no compare-and-swap ties two different refs
// together, so the post phase settles it without any ordering rule. It yields to ANY live alias:
// the claimant removes its own ref and refuses. No clock is consulted (the claim date is the
// client's own clock, so it cannot arbitrate), and a rival that already finished its own post
// phase and returned 0 never looks again, so the later claimant must always be the one to yield.
// The worst case is that two claimants created in the same instant both see each other and both
// refuse; that fails closed, and a retry settles it. stop=false means clear.
//
// A post-phase check that cannot be answered (exit 6) removes the ref just created, best effort:
// a claim whose uniqueness nobody confirmed must not be left standing.
func checkAliases(id, phase string) (rc int, stop bool) {
	rc, stop = checkAliasesOnce(id, phase)
	if stop && rc == exitUnverifiable && phase == "post" {
		store.Remove(id)
	}
	return rc, stop
}

func checkAliasesOnce(id, phase string) (rc int, stop bool) {
	others, status := aliasIDs(id)
	if status == deskkit.ClaimReadUnverifiable {
		errf("unverifiable: could not list dispatch claims to check other spellings of %s%s", id, causeSuffix())
		return exitUnverifiable, true
	}
	for _, other := range others {
		ref, live, verifiable := claimLive(other)
		if !verifiable {
			errf("unverifiable: could not read the claim on %s, another spelling of %s%s", other, id, causeSuffix())
			return exitUnverifiable, true
		}
		if !live {
			continue
		}
		if phase == "post" {
			if outcome, _ := store.Remove(id); outcome != deskkit.ClaimWriteApplied {
				errf("unverifiable: %s is live but could not release %s/%s%s", other, refPrefix, id, causeSuffix())
				return exitUnverifiable, true
			}
		}
		reportHolder(other, ref.Msg, ref.Date)
		logf("  %s is another spelling of the same PR as %s — not dispatching", other, id)
		return exitRefused, true
	}
	return exitOK, false
}

// printAliasHeld is what `show` appends for every live alias of id: a FREE here is not free when
// another spelling of the same PR holds it. exitUnverifiable when the claim set (or one alias) could
// not be read: the verb's own line is already printed, but the answer is not "no alias".
func printAliasHeld(id string) int {
	others, status := aliasIDs(id)
	if status == deskkit.ClaimReadUnverifiable {
		errf("unverifiable: could not list dispatch claims to check other spellings of %s%s", id, causeSuffix())
		return exitUnverifiable
	}
	rc := exitOK
	for _, other := range others {
		ref, live, verifiable := claimLive(other)
		if !verifiable {
			errf("unverifiable: could not read the claim on %s, another spelling of %s%s", other, id, causeSuffix())
			rc = exitUnverifiable
			continue
		}
		if !live {
			continue
		}
		ageStr := "?"
		if age, ok := ageMinutes(ref.Date); ok {
			ageStr = strconv.Itoa(age)
		}
		logf("ALIAS-HELD %s — %s at=%s age=%sm (same PR as %s)", other, ref.Msg, ref.Date, ageStr, id)
	}
	return rc
}
