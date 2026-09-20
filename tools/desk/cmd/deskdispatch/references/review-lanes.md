# Review lanes — the per-tier lane set

This is the dispatch reference for tier-keyed review depth: which lanes a
review dispatch runs for a pull request, as a function of the author's
contributor-trust tier. Two readers: the review desk, which selects the lane
set before dispatching, and the reviewers dispatched on the fact-check and
fail-first lanes, whose output contracts are stated here.

The tier vocabulary and its resolution (roster membership first, the
operator-side ledger second, `unknown` as the fail-closed default) live in the
published contributor-trust model — `docs/contributor-trust.md` and the
`deskkit` tier resolver. This document does not restate them; it states what
each tier's review RUNS.

## The lane set by tier

<!-- reviewlanes:begin -->
| tier | lanes |
|---|---|
| unknown | correctness, security, fact-check, fail-first |
| blessed-once | correctness, security, fact-check, fail-first |
| contributor | correctness, security |
| maintainer | correctness, security |
<!-- reviewlanes:end -->

The block above is machine-checked against the lane table the dispatcher
selects from (`deskkit.LanesFor`): a lane set written here that the code does
not select fails the test suite, and so does a lane set the code selects that
this document fails to state. The lane set is the ONLY thing that varies by
tier — the verdict shape, the reviewer identity, the ready flip and the merge
authority are the same at every tier, and no tier merges anything.

Why the deep set sits on the two lowest tiers: the deep lanes are a cost
imposed where claims are least likely to have been verified, not a grant a
tier earns. `unknown` is every identity the ledger does not name — today's bar
— and `blessed-once` is one admitted item from an otherwise-unknown identity,
checked rather than trusted. A recorded standing grant (`contributor`) or
roster membership (`maintainer`) is what returns review to the standard path.
Until an operator configures a ledger this is inert: every external identity
resolves `unknown` and gets the deep set, and roster identities keep the
standard path unchanged.

## The lanes

- **correctness** — the correctness review of the change against the main it
  will merge into. On the deep set it is dispatched at strong tier; on the
  standard set its tier stays risk-keyed as it is today.
- **security** — the security review, unchanged from today's dispatch: its own
  lane on risk-classed pull requests, at strong tier, whatever the author's
  tier is.
- **fact-check** — a claims-versus-diff fact check of the pull-request body.
  Output contract below. Strong tier.
- **fail-first** — a mandatory fail-first reproduction of the change's claimed
  effect. Two required records, below. Strong tier.

## The fact-check lane's output contract

The fact check enumerates every assertion the pull-request body makes and
returns, per claim, exactly one of three states:

- `confirmed` — the diff, or a run, supports the claim.
- `contradicted` — the diff, or a run, disproves the claim.
- `unverified` — it could not be checked from the change alone.

Three rules bind the states:

1. **Every claim gets exactly one state.** A claim listed without a state, or
   with an invented one, violates the contract — the reviewer's output is
   checked for this.
2. **`unverified` is a legitimate, expected outcome.** It is not a failure,
   and it is never rounded to `confirmed`: an instrument that did not look has
   cleared nothing. A body claim nobody could check is recorded as exactly
   that.
3. **A `contradicted` claim is a review finding on its own**, independent of
   whether the diff is correct. The claim, not only the code, is what the
   reviewer answers — an assertion the diff disproves is a finding even when
   the change itself is sound.

The claims-extraction helper (`deskkit.ExtractClaims`) is the sweep the lane
is dispatched with: a starting list of candidate assertions — every bullet and
numbered item, every prose sentence carrying an assertion marker, with
headings, fenced code and table rows skipped — each starting at `unverified`.
The helper is the starting point, not the verdict: extraction quality is the
review gate's judgement, and the reviewer enumerates the body's assertions
beyond the list when the body carries more.

## The fail-first reproduction's two required records

The fail-first lane demonstrates the reported problem BEFORE the change, then
the fix AFTER it. Both records are required — one without the other is an
incomplete lane:

1. **Base, failing.** Run the reported failing case at the merge base of the
   pull request's branch and main, and record it FAILING there: the failing
   assertion, and the commit it ran against.
2. **Head, passing.** Run the same case at the pull request's head and record
   it PASSING.

A change whose reported problem cannot be reproduced at the merge base is
REPORTED as such — "could not reproduce at the base; the fix's premise is
unverified" — never merged around, and never satisfied by a head-only pass: a
reviewer who records only the head passing has recorded half the lane. The
lane runs under the standing rules that govern building and running any change
under review; it introduces no exception to them.

## Selection

The dispatch selection path is one call: `deskkit.ReviewLanesForAuthor(repo,
login, id)` resolves the author's tier (with its provenance) and returns the
lane set from the same table this document's block is checked against — so
the tier a dispatch keys on and the lanes it dispatches cannot disagree. A
caller that resolves the tier by any other route and hand-picks lanes has left
the path this reference describes.
