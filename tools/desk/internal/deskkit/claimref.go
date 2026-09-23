package deskkit

// claimref.go — WHERE a cross-machine dispatch claim lives, defined exactly once.
//
// The dispatch claim is the fleet's mutual-exclusion primitive: a ref in the target repo
// that says "this item is being worked, by someone, somewhere". Two desks on two machines
// cannot see each other's processes, so the ref is the only thing standing between them and
// the same brief. Its failure mode is silent in BOTH directions — a claim nobody can release
// is a slot lost for good, and a claim the reader looks for in the wrong place reads as free
// when it is held — so the namespace is a constant, in one file, that every writer and every
// reader derives from. A second literal spelling of it anywhere in the tree IS the drift this
// file exists to make impossible.
//
// # Why "heads/dispatch" and not "dispatch"
//
// The claim used to live in a namespace of its own directly under `refs/`, outside both
// `heads/` and `tags/`. Creating and reading one is plain git and
// works on any git host; RELEASING one is not. GitLab Community Edition exposes no general
// ref-delete endpoint at any tier — the Branches API deletes a branch and the Tags API
// deletes a tag, and that is the whole surface — so GitLabForge.DeleteRef could only answer
// could-not-check for a ref outside refs/heads, in as many words: such a claim is "NOT
// reported released". A claim that can be taken and never given back degrades the fleet's
// mutual exclusion to a leak.
//
// So the claim moved INTO the one namespace both forges can delete: it is a branch. The
// decision, the live reads it turns on, and the costs it accepts (a claim is visible in the
// branch list, and a push-triggered CI configuration will see it) are recorded in the
// claim-shape decision record of the stream that made it. What matters here is that the
// namespace is the SAME on both forges — two namespaces would be two mutual-exclusion systems, and a
// cross-forge desk pair would collide while each believed it held the lock.
//
// # The shape
//
//	ref path (Forge.DeleteRef argument):  heads/dispatch/<key>
//	fully qualified ref:                  refs/heads/dispatch/<key>
//	branch name (Branches API):           dispatch/<key>
//	claim key:                            <repo>--<stream>--<NN> | <repo>--issue-<NN>
//
// A key occupies exactly ONE path component under the namespace. That is enforced here
// rather than assumed: a key carrying a "/" would nest the claim a level deeper, where the
// reader's single-segment parse would return the wrong key — so it is refused, not
// flattened.

import (
	"fmt"
	"strings"
)

const (
	// ClaimRefNamespace is the namespace a dispatch claim lives in, in the form
	// Forge.DeleteRef takes (no leading "refs/"). Every claim ref path is this plus "/" plus
	// the claim key.
	ClaimRefNamespace = "heads/dispatch"

	// ClaimRefsPrefix is the fully-qualified prefix of every claim ref — what a local
	// `git for-each-ref` and a remote `git ls-remote` listing match on.
	ClaimRefsPrefix = "refs/" + ClaimRefNamespace + "/"

	// ClaimRefsPattern is the glob form of that prefix, for `git ls-remote origin <pattern>`
	// and for prose that has to name the listing.
	ClaimRefsPattern = ClaimRefsPrefix + "*"

	// DispatchClaimActiveRefsPrefix is the namespace dispatch claims are ACTUALLY acquired in
	// today by deskclaim-ref / deskdispatch (`refs/dispatch/`), which DIFFERS from ClaimRefsPrefix
	// (`refs/heads/dispatch/`) that the fleet's Go claim READERS list against — the divergence
	// flagged for a house ruling in issue 708 (cmd/deskclaim-ref/main.go SCOPE NOTE). A reader
	// that must find a live claim WHERE IT ACTUALLY LIVES — rather than where it will live once
	// 708 consolidates the two — reads against THIS prefix. It is deliberately scoped to that
	// reader need (cmd/deskpost's review-lane stamp age-out): the writer/acquire path is untouched,
	// and when 708 lands, both callers of this collapse back onto ClaimRefsPrefix.
	DispatchClaimActiveRefsPrefix = "refs/dispatch/"
)

// ClaimBranchPrefix is the BRANCH-name form of the namespace — the shape a forge's branch
// API, a branch listing, and a repo's push-triggered CI branch filters see. It is DERIVED
// from ClaimRefNamespace rather than spelled a second time, so the branch form and the ref
// form cannot drift apart.
func ClaimBranchPrefix() string {
	return strings.TrimPrefix(ClaimRefNamespace, "heads/") + "/"
}

// ClaimRefPath renders the claim ref path for key, in the canonical form Forge.DeleteRef
// takes ("heads/dispatch/<key>"), or refuses.
//
// It refuses rather than sanitizes. A caller that hands this a key with a path separator in
// it has a bug in its key derivation, and silently rewriting the key would produce a ref that
// no reader looks for — a claim taken in a place nothing checks is worse than no claim at
// all. The ref it does produce is put through ValidateRefPath, so the one argument that
// reaches a backend is a ref path inside the repo's own namespace and never an API path.
func ClaimRefPath(key string) (string, error) {
	k := strings.TrimSpace(key)
	if k == "" {
		return "", Refused("refusing to build a dispatch claim ref for an empty claim key")
	}
	if strings.Contains(k, "/") {
		return "", Refused(fmt.Sprintf(
			"refusing claim key %q: a claim key occupies ONE path component under %s, so it may not carry "+
				"a path separator (the dispatcher's key grammar is <repo>--<stream>--<NN>; \"/\" is written "+
				"as \"--\")", key, ClaimRefsPrefix))
	}
	return ValidateRefPath(ClaimRefNamespace + "/" + k)
}

// ReviewClaimFamilyRefPrefix renders the fully-qualified ref PREFIX that every review-dispatch
// claim for one PR shares: DispatchClaimActiveRefsPrefix + "<short>--pr-<N>". A review of PR N
// is dispatched under the item key "<short>--pr-<N>", and a RE-dispatch disambiguates with a
// "--<suffix>" (e.g. "--rr3-corr", "--security"), so no single exact key identifies "the review
// claim" — the family is matched by this prefix. ok=false when the repo has no short label or the
// PR number is not positive (the caller's answer for that is ClaimLivenessUnknown, never a
// release).
func ReviewClaimFamilyRefPrefix(repo string, pr int) (string, bool) {
	short := strings.TrimSpace(RepoShortLabel(repo))
	if short == "" || pr <= 0 {
		return "", false
	}
	return fmt.Sprintf("%s%s--pr-%d", DispatchClaimActiveRefsPrefix, short, pr), true
}

// RefInReviewClaimFamily reports whether ref is a member of the review-claim family named by
// familyPrefix: it EQUALS the prefix (the un-suffixed review claim) or begins with the prefix
// plus "--" (a disambiguated re-dispatch). The "--" boundary is load-bearing: without it the
// family for PR 148 ("…--pr-148") would swallow PR 1489's claims ("…--pr-1489…"), since one is a
// string prefix of the other. A claim key carries no "/", so the whole suffix after the PR number
// is either empty or "--<segment>", and this test is exact.
func RefInReviewClaimFamily(familyPrefix, ref string) bool {
	r := strings.TrimSpace(ref)
	return r == familyPrefix || strings.HasPrefix(r, familyPrefix+"--")
}

// ReviewClaimLivenessFromMatchingRefs reduces a MatchingRefs result for a review-claim family to
// a ClaimLiveness. A read error is Unknown (never a release — a family we could not list is not a
// family we saw gone). Otherwise any returned ref that is a true family member (RefInReviewClaimFamily,
// which re-filters the prefix match to the "--" boundary) means the review cycle is still held
// (ClaimHeld); none means it is over (ClaimReleased), and the reviewer's stamp ages out.
func ReviewClaimLivenessFromMatchingRefs(refs []string, familyPrefix string, err error) ClaimLiveness {
	if err != nil {
		return ClaimLivenessUnknown
	}
	for _, r := range refs {
		if RefInReviewClaimFamily(familyPrefix, r) {
			return ClaimHeld
		}
	}
	return ClaimReleased
}

// ClaimKeyFromRef returns the claim key named by ref, and whether ref is a claim ref at all.
// It accepts both spellings a listing can hand it — fully qualified ("refs/heads/dispatch/x")
// and the DeleteRef form ("heads/dispatch/x") — and reports false for anything that is not
// EXACTLY one component under the claim namespace, so a nested or foreign ref is never
// mistaken for a claim.
func ClaimKeyFromRef(ref string) (string, bool) {
	r := strings.TrimPrefix(strings.TrimSpace(ref), "refs/")
	rest, ok := strings.CutPrefix(r, ClaimRefNamespace+"/")
	if !ok || rest == "" || strings.Contains(rest, "/") {
		return "", false
	}
	return rest, true
}
