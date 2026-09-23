---
brief: assay:assay:contributor-trust:04
title: "Review depth by tier — an unknown author's pull request gets a claims-versus-diff fact check and a fail-first reproduction"
why: >-
  The one thing that was provably wrong in the first unsolicited arrivals here was a body
  claim: "fixed, all tests pass", asserted without being checked, on a pull request whose diff
  did not support it. Review today reads an unknown author's description with the same
  credence as a maintainer's, which is exactly the assumption those arrivals disproved. Making
  depth a function of tier costs a known contributor nothing and puts the cheapest possible
  control — read each claim, check it against the diff, say which ones you could not confirm —
  in front of the submissions where claims are least likely to have been verified.
wave: 1
depends: ["contributor-trust/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-12 by a contributor-trust authoring session
sources:
  - "docs/streams/contributor-trust/spec.md §1 — 'at least one body claim provably false' is the observed defect this lane is designed against, and §3's third consequence (review depth does not vary)."
  - "docs/streams/contributor-trust/brief-02-trust-tiers-and-ledger.md — the tier vocabulary and the per-tier capability table this brief reads; review-lane depth is one of the four capabilities defined there."
  - "plugins/assay/skills/pr-review-desk/SKILL.md — the existing review dispatch: the standing reviewer pool, the correctness and security lanes running in parallel, and the verdict as a real forge review."
  - "freshness-checked 2026-09-12 @ e96b7f6d (origin/main) — review dispatch reads no author property at all; every pull request gets the same lane set regardless of who opened it."
exec-tier: strong
exec-tier-why: >-
  (b) correctness is a cross-artifact argument: which lanes fire for which tier, read against
  the dispatcher's existing lane selection and against the verdict shape the review gate
  already expects.
consumers:
  - "tools/desk/cmd/deskdispatch/references/review-lanes.md: fixed-here (the per-tier lane sets, the fact-check output contract and the fail-first reproduction's two required records land in this reference)"
  - "tools/desk/internal/deskkit/reviewlanes.go: fixed-here (the tier-to-lane table with LanesFor, the dispatch selection path ReviewLanesForAuthor, and the claims-extraction helper land in this file)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: fixed-here (one neutral paragraph: lane depth is tier-keyed, the tier source resolved from the project layer, never inlined)"
  - "tools/desk/internal/deskkit/trusttier.go: out-of-scope (defined by contributor-trust/02; this brief is a consumer of that reader and adds nothing to it)"
version: 1
---

# Brief 04 — Review depth by tier

## Context

files:
- `tools/desk/internal/deskkit/reviewlanes.go` (new) — `LanesFor(tier) []Lane`, pure, table-
  driven from the per-tier capability table, plus the claims-extraction helper the fact-check
  lane is dispatched with.
- `tools/desk/internal/deskkit/reviewlanes_test.go` (new).
- `tools/desk/cmd/deskdispatch/references/review-lanes.md` (new) — the lane set per tier and
  the fact-check lane's output contract, as a dispatch reference.
- `plugins/assay/skills/pr-review-desk/SKILL.md` — one paragraph: lane depth is tier-keyed;
  the tier source is a project-layer value the neutral body resolves, not a value inlined
  here.
- `changelog/contributor-trust-review-depth.md` (new).

facts:
- Lane set by tier. `unknown` and `blessed-once`: correctness at strong tier, the security
  lane, a claims-versus-diff fact check, and a mandatory fail-first reproduction. `contributor`
  and `maintainer`: the standard path, unchanged from today.
- The claims-versus-diff fact check enumerates every assertion the pull-request body makes and
  returns, per claim, one of `confirmed` (the diff or a run supports it), `contradicted` (the
  diff or a run disproves it), or `unverified` (it could not be checked from the change
  alone). Every claim gets exactly one, and `unverified` is a legitimate, expected outcome —
  it is not a failure and must never be rounded to `confirmed`.
- The fail-first reproduction requires the reviewer to demonstrate the reported problem before
  the change: run the failing case at the merge base and record it failing, then at the head
  and record it passing. A change whose problem cannot be reproduced at the base is reported
  as such, not merged around.
- A `contradicted` claim is a review finding on its own, independent of whether the diff is
  correct. The claim, not only the code, is what the reviewer answers.
- Lane selection is the only thing that varies. The verdict shape, the reviewer identity, the
  ready flip and the merge authority are unchanged.
- Risk answers, against the declared paths: `tools/desk/internal/deskkit/` is a security-trigger
  path, and all four answers are nevertheless `no`. The reason is that this brief only ADDS
  lanes for the lowest tiers and changes no predicate that admits, authorizes or writes — the
  tier it reads is produced by `contributor-trust/02`, which carries the human gate for the
  authority question. The failure direction here is a review that is too shallow for a known
  contributor, which the review gate itself catches; it cannot widen what anyone may do.
- This brief is inert until a ledger exists: with no ledger every external identity is
  `unknown`, so external pull requests get the deep lane and internal ones are unaffected.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never build, test or execute an unblessed pull-request head as part of this work, including
  in a fixture. The fail-first reproduction is a lane the reviewer runs under the existing
  blessing rules; this brief defines it and does not run it.

## Task

1. `reviewlanes.go`: the tier-to-lane table and `LanesFor`. Table-driven, so the whole policy
   is readable in one place.
2. The claims-extraction helper and the three-state per-claim result type, with the invariant
   that every extracted claim carries exactly one state and `unverified` is representable.
3. The dispatch reference: the lane set per tier, the fact-check output contract, and the
   fail-first reproduction's two required records (base failing, head passing).
4. One paragraph in the review-desk skill body, written neutrally so it carries no house
   value.
5. The changelog fragment.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1` | exit 0; output contains `ok` | check |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v` | exit 0; output contains `fact-check`; output contains `fail-first`; output contains `security` | check |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v` | exit 0; output does not contain `fail-first` (negative control: a known contributor keeps the standard path) | check +mutation |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v` | exit 0; output contains `PASS` (an unverifiable claim has its own state and is never rounded to confirmed) | check +mutation |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v` | exit 0; output contains `PASS` | check |
| 6 | `grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md` | exit 0; at least one matching line | check |
| 7 | `grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md` | exit 0; at least one matching line (the fail-first reproduction names both records) | check |
| 8 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | exit 0 | check:ci |
| 9 | `cd tools/skillslint && go run . --root ../..; echo rc=$?` | output contains `rc=0` (the skill-body edit keeps the parity and unresolved-house-value checks green) | check:ci +neighbour |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v` | exit 0; output contains `PASS` (the test parses the per-tier lane sets out of the dispatch reference and compares them to `LanesFor`, so a reference document that describes a lane set the dispatcher does not select fails) | check +dereference |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v` | exit 0; output contains `PASS` (tier in, lane set out, through the dispatcher's own selection path rather than through `LanesFor` alone) | check +flow |
| 12 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:04` | exit 0; output does not contain `DISPROVED`; output does not contain `COULD-NOT-CHECK`; output contains `corroborated` (the fully-qualified key is required — the short `<stream>/<NN>` form answers `no brief-v1 file` and exits 2, so it can never corroborate anything) | check |

Pre-mortem to detection map. "Every tier silently gets the deep lane, so the change is a
blanket slowdown nobody asked for" is caught by row 3, the negative control. "A claim the
reviewer could not check is recorded as confirmed, which is the exact failure the lane exists
to prevent" is caught by rows 4 and 5. "The fail-first reproduction is described as 'reproduce
the bug' with no requirement to record the base run, so a reviewer records only the head
passing" is caught by row 7. "The skill-body paragraph inlines a house value into a neutral
body" is caught by row 9. "The fact-check lane enumerates claims badly — it finds three of a
body's seven assertions" — no row; extraction quality is a review-gate judgement, and the
contract only requires that every claim it does extract carries a state.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

### Non-implementer verifier run — VERIFY: BLOCKED — 2026-09-23 opus-5.5-verifier

| # | Command | Expected | Observed (exit + key output) | Date Runner |
|---|---------|----------|------------------------------|-------------|
| 1 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1 | exit 0; output contains ok | exit 0; ok github.com/medici-finance/assay/tools/desk/internal/deskkit | 2026-09-23 opus-5.5-verifier |
| 2 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v | exit 0; output contains fact-check, fail-first, security | exit 0; PASS TestReviewLanesUnknownTierGetsTheDeepSet; log lines show the unknown-tier lanes security, fact-check and fail-first | 2026-09-23 opus-5.5-verifier |
| 3 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v | exit 0; output does NOT contain fail-first (negative control) | exit 0; PASS TestReviewLanesContributorTierKeepsTheStandardPath; no fail-first in output | 2026-09-23 opus-5.5-verifier |
| 4 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v | exit 0; output contains PASS | exit 0; PASS TestClaimStateUnverifiedIsRepresentable | 2026-09-23 opus-5.5-verifier |
| 5 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v | exit 0; output contains PASS | exit 0; PASS TestClaimEveryClaimCarriesAState | 2026-09-23 opus-5.5-verifier |
| 6 | grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md | exit 0; at least one match | exit 0; 4 matching lines (63, 70, 82, 101) | 2026-09-23 opus-5.5-verifier |
| 7 | grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md | exit 0; at least one match (fail-first names both records) | exit 0; 2 matching lines (93 base-failing, 99 cannot-reproduce-at-base) | 2026-09-23 opus-5.5-verifier |
| 8 | cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/ | exit 0 | could-not-check (verifyrun): check:ci hermetic execution requires a network-off sandbox (unshare --net, a Linux facility); this host is darwin. Non-hermetic manual run: exit 0 (build + vet clean). Needs a Linux runner for the sanctioned witness | 2026-09-23 opus-5.5-verifier |
| 9 | cd tools/skillslint && go run . --root ../..; echo rc=$? | output contains rc=0 | could-not-check (verifyrun): same darwin/unshare --net limitation as row 8. Non-hermetic manual run: rc=0, SKILLSLINT PASS (parity + house-values + guardrails + enforcement-block all PASS; word-budget/POSIX notices are advisory, exit unaffected). Needs a Linux runner for the sanctioned witness | 2026-09-23 opus-5.5-verifier |
| 10 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v | exit 0; output contains PASS | exit 0; PASS TestReviewLanesReferenceMatchesTable (reference lane sets parse back to and match LanesFor) | 2026-09-23 opus-5.5-verifier |
| 11 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v | exit 0; output contains PASS | exit 0; PASS TestReviewLanesDispatchEndToEnd; log shows a broken/unreadable ledger still resolving tier=unknown to the deep set (fail-closed) | 2026-09-23 opus-5.5-verifier |
| 12 | statusgen --root . --consumers --brief assay:assay:contributor-trust:04 | exit 0; output does not contain DISPROVED or COULD-NOT-CHECK; output contains corroborated | As written (default base = merged main): exit 2, COULD-NOT-CHECK — the merged brief is not in the diff against merged HEAD, so this run carries no evidence (a post-merge instrument property, not a disproof). Re-run against the base that made the claims (the squash parent of the landing commit): exit 0, "3 corroborated, 0 disproved, 1 unchecked" — the three fixed-here consumers CORROBORATED, the out-of-scope trusttier reader correctly UNCHECKED, nothing DISPROVED | 2026-09-23 opus-5.5-verifier |

RISK-VALUE (kit §4 — enumerate → rank → derive):

Enumeration over the brief's diff scope (reviewlanes.go, reviewlanes_test.go,
review-lanes.md, one SKILL.md paragraph) found NO numeric constant, bound,
threshold, tolerance, ratio, timeout or limit. The only literals introduced are
policy/contract strings and one policy table:
- Lane name strings: correctness @ reviewlanes.go:72, security @ :78, fact-check @ :86, fail-first @ :94.
- ExecTier string "strong" @ reviewlanes.go:80, :88, :96 and set by strongTier @ :105.
- ClaimState strings: confirmed @ reviewlanes.go:182, contradicted @ :183, unverified @ :184.
- The tier-to-lane policy table (the load-bearing binding) @ reviewlanes.go:118-123.
- The fail-closed default (unrecognized/empty tier resolves to the deep set) @ reviewlanes.go:136-138.

Ranked by irreversibility, every entry is a reversible policy knob (edit +
redeploy); the item's own risk answers are all no and irreversible: no. The
top-ranked (most policy-load-bearing) entries are the tier-to-lane table and its
fail-closed default:

- RISK-VALUE: DERIVED — laneTable[TierContributor] = {correctness, security} @ reviewlanes.go:121 and laneTable[TierMaintainer] = {correctness, security} @ reviewlanes.go:122, versus laneTable[TierUnknown] = laneTable[TierBlessedOnce] = {strong correctness, security, fact-check, fail-first} @ reviewlanes.go:119-120 — the brief facts pin "unknown and blessed-once: ... deep set" and "contributor and maintainer: the standard path, unchanged from today"; the table matches that mapping exactly. Wrong direction is bounded and reversible: giving a low tier the standard path makes review too shallow (caught by the review gate itself), giving a known contributor the deep set only imposes cost — neither widens what anyone may do.
- RISK-VALUE: DERIVED — fail-closed default: an out-of-range/empty tier resolves to laneTable[TierUnknown] (the deep set) @ reviewlanes.go:136-138 — the brief's stated failure-safe direction is MORE scrutiny by default ("with no ledger every external identity is unknown, so external pull requests get the deep lane"); defaulting to the deep set is the correct, safe direction, confirmed at runtime by row 11's broken-ledger case still selecting the deep set.


## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
