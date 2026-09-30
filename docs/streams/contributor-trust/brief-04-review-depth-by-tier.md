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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1` | pass exit=0 | sha256:1d28d7abce26 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v` | pass exit=0 | sha256:e810c326c4dc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v` | pass exit=0 | sha256:8224ef7de39d | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v` | pass exit=0 | sha256:6e93c1cff359 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v` | pass exit=0 | sha256:c179bd682b79 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md` | pass exit=0 | sha256:affec666f7bd | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md` | pass exit=0 | sha256:fb5b5d540866 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/skillslint && go run . --root ../..; echo rc=$?` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v` | pass exit=0 | sha256:211b5cfcb106 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v` | pass exit=0 | sha256:d6b6c1ac458c | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:04` | fail exit=2 | sha256:d76e499494f5 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e+dirty (on-behalf-of human:ian) (forge-identity) |

Verifier observations, 2026-09-27, claude-opus-5-5 (dispatched non-implementer verifier), against merged
main 9585b4b6cc2ea8d35d367fb912e7c8216a765ba3; implementing change: squash commit 2083064fd
(medici-finance/assay#1367). Offline envelope. The witness table above is the per-row record; the key
output line behind each row:

- Row 1: exit 0, `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit`.
- Row 2: exit 0, `--- PASS: TestReviewLanesUnknownTierGetsTheDeepSet`; logged lanes security, fact-check, fail-first.
- Row 3: exit 0, `--- PASS: TestReviewLanesContributorTierKeepsTheStandardPath`; lanes correctness and security only, zero fail-first. Mutation probe (contributor row given LaneFailFirst): exit 1, `LanesFor(contributor) = [correctness security fail-first], want the standard path` — mutant killed, source restored.
- Row 4: exit 0, `--- PASS: TestClaimStateUnverifiedIsRepresentable`. Mutation probe (extraction seeds ClaimConfirmed): exit 1, `extracted at state confirmed — extraction must start every claim unverified, never confirm it` — mutant killed, source restored.
- Row 5: exit 0, `--- PASS: TestClaimEveryClaimCarriesAState` (6 claims extracted from the sample body).
- Row 6: exit 0, 4 matching lines in the dispatch reference, first at line 63.
- Row 7: exit 0, 2 matching lines, line 93 `Base, failing. Run the reported failing case at the merge base`.
- Row 8: the witness's hermetic network-off re-execution needs a Linux runner (this host is darwin); the same command executed directly on the host: exit 0, no output. Hermetic re-execution belongs to CI.
- Row 9: as row 8 for the witness; executed directly on the host: `rc=0`, `HOUSE-VALUES: PASS — 39 markdown file(s) under plugins/`.
- Row 10: exit 0, `--- PASS: TestReviewLanesReferenceMatchesTable`.
- Row 11: exit 0, `--- PASS: TestReviewLanesDispatchEndToEnd`; absent ledger resolves tier=unknown with lanes [correctness security fact-check fail-first]; roster identity resolves tier=maintainer with lanes [correctness security].
- Row 12: exit 2 (statusgen v1.0.27), `COULD-NOT-CHECK: assay:assay:contributor-trust:04 is not in the diff against 9585b4b6…`. This is the known post-merge `--consumers` class tracked at medici-finance/assay#1281: on merged main the brief's own diff is empty, so this command form cannot corroborate. Supporting evidence — the same corroboration re-run at the implementing commit (head 2083064fd, base its parent cabdc0b84): exit 0, `summary: 3 corroborated, 0 disproved, 1 unchecked`; the one unchecked entry is the out-of-scope trusttier.go consumer, unchanged by design.

RISK-VALUE: DERIVED — laneTable TierUnknown/TierBlessedOnce = {strongTier(LaneCorrectness), LaneSecurity, LaneFactCheck, LaneFailFirst}; TierContributor/TierMaintainer = {LaneCorrectness, LaneSecurity} @ tools/desk/internal/deskkit/reviewlanes.go:119-122 — matches the brief's "Lane set by tier" fact verbatim (deep set for the two lowest tiers, today's standard path for known tiers); only adds review for low tiers, reversible by edit + redeploy.
RISK-VALUE: DERIVED — out-of-range tier fallback `set = laneTable[TierUnknown]` @ tools/desk/internal/deskkit/reviewlanes.go:137 — fail-closed to the deepest set, consistent with the brief's "inert until a ledger exists: every external identity is unknown" and the spec's more-scrutiny-on-doubt direction; an empty set would dispatch no review.
RISK-VALUE: DERIVED — ExecTier = "strong" @ tools/desk/internal/deskkit/reviewlanes.go:80, :88, :96, :105 — the brief pins "correctness at strong tier" for the deep set and the security lane is unchanged from existing strong-tier dispatch; reversible knob.
RISK-VALUE: NAMED, NOT DERIVED — claimMarkerRe (assertion-marker word list) @ tools/desk/internal/deskkit/reviewlanes.go:232 — extraction recall is explicitly a review-gate judgement per the brief's pre-mortem; every extracted claim starts unverified, so a miss narrows the starting list but never confirms a claim. Reversible.

**Open question (claimMarkerRe).** The assertion-marker word list is named but not derived: no spec
or brief fixes which verbs mark a body sentence as a claim, and no row measures extraction recall.
Whether the list is sufficient is left to the review gate / a human reader, and is routed, not closed,
by this verification.

VERIFY: FAIL — row 12 only, and stale-shaped (check-definition): the row's command runs corroboration against merged main, where the brief is not in the diff, so it structurally returns COULD-NOT-CHECK exit 2 (the post-merge `--consumers` class, medici-finance/assay#1281). The substantive consumers claim corroborates at the implementing diff (3 corroborated, 0 disproved, 1 out-of-scope unchecked). All other rows pass on merged main; two mutation probes killed.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ e03f4f5c7c41 (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main e03f4f5c7c412560a666d95383bee0444fb6d263, gate: model, all four risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim; it ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, `unshare --net` available), statusgen built in-container from a clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1` | pass exit=0 | sha256:e55f84b1b248 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v` | pass exit=0 | sha256:be40d9f94a9d | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v` | pass exit=0 | sha256:ce24674eef17 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v` | pass exit=0 | sha256:bf8114ae9b9f | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v` | pass exit=0 | sha256:ebe86fe273c1 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 6 | `grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md` | pass exit=0 | sha256:affec666f7bd | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 7 | `grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md` | pass exit=0 | sha256:fb5b5d540866 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 8 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 9 | `cd tools/skillslint && go run . --root ../..; echo rc=$?` | pass exit=0 | sha256:18d2ae7777b5 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v` | pass exit=0 | sha256:53ca3f783f8b | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v` | pass exit=0 | sha256:3f41ae76557b | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |
| 12 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:04` | fail exit=2 | sha256:ba64caaf7b46 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (git-config) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1 | exit 0; ok | exit 0; ok github.com/medici-finance/assay/tools/desk/internal/deskkit 0.620s | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 2 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v | exit 0; fact-check, fail-first, security in output | exit 0; --- PASS: TestReviewLanesUnknownTierGetsTheDeepSet; logged lanes correctness, security, fact-check, fail-first | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 3 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v | exit 0; no fail-first in output | exit 0; --- PASS: TestReviewLanesContributorTierKeepsTheStandardPath; lanes correctness and security only; zero fail-first occurrences. Mutation (contributor row given LaneFailFirst): exit 1, want the standard path [correctness security] — killed, reverted | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 4 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v | exit 0; PASS | exit 0; --- PASS: TestClaimStateUnverifiedIsRepresentable. Mutation (ExtractClaims seeds ClaimConfirmed): exit 1, extracted at state confirmed — killed, reverted | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 5 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v | exit 0; PASS | exit 0; --- PASS: TestClaimEveryClaimCarriesAState | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 6 | grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md | exit 0; at least one line | exit 0; 4 lines (63, 70, 82, 101) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 7 | grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md | exit 0; at least one line | exit 0; 2 lines, 93 Base, failing. Run the reported failing case at the merge base, and 99 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 8 | cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/ | exit 0 | exit 0, no output | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 9 | cd tools/skillslint && go run . --root ../..; echo rc=$? | rc=0 | rc=0; HOUSE-VALUES: PASS — 39 markdown file(s) under plugins/; HIDDEN-CHARS, GUARDRAILS, ENFORCEMENT-BLOCK, POSIX-TOKEN all PASS | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 10 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v | exit 0; PASS | exit 0; --- PASS: TestReviewLanesReferenceMatchesTable | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 11 | cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v | exit 0; PASS | exit 0; --- PASS: TestReviewLanesDispatchEndToEnd | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 12 | statusgen --root . --consumers --brief assay:assay:contributor-trust:04 | exit 0; corroborated; no DISPROVED; no COULD-NOT-CHECK | exit 2; statusgen: --consumers: COULD-NOT-CHECK: assay:assay:contributor-trust:04 is not in the diff against b89b3957225e, so this run carries no evidence about its claims. Hand procedure at the implementing commit 2083064fd with --base its parent cabdc0b84: exit 0, 3 CORROBORATED, 0 disproved, 1 unchecked (trusttier.go, out-of-scope, unchanged by design) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — laneTable TierUnknown = TierBlessedOnce = {strongTier(LaneCorrectness), LaneSecurity, LaneFactCheck, LaneFailFirst}; TierContributor = TierMaintainer = {LaneCorrectness, LaneSecurity} @ tools/desk/internal/deskkit/reviewlanes.go:119-122 — the brief's "Lane set by tier" fact verbatim: the deep set on the two lowest tiers, today's standard path on the known tiers; row 3's mutant confirms the negative control bites.
RISK-VALUE: DERIVED — out-of-range tier fallback set = laneTable[TierUnknown] @ tools/desk/internal/deskkit/reviewlanes.go:137 — fail-closed to the deepest set, the direction the brief states (no ledger means every external identity is unknown and gets the deep lane); the alternative, an empty set, would dispatch no review at all.
RISK-VALUE: DERIVED — extraction seed State: ClaimUnverified @ tools/desk/internal/deskkit/reviewlanes.go:270 and :277 — the brief requires unverified to be representable and never rounded to confirmed; extraction asserts nothing, so the only correct starting state is unverified; row 4's mutant (seed confirmed) is killed.
RISK-VALUE: DERIVED — ClaimConfirmed = "confirmed", ClaimContradicted = "contradicted", ClaimUnverified = "unverified" @ tools/desk/internal/deskkit/reviewlanes.go:182-184 — exactly the three states the brief's fact-check fact names, and CarriesState rejects the zero value.
RISK-VALUE: DERIVED — ExecTier = "strong" @ tools/desk/internal/deskkit/reviewlanes.go:105 (deep-set correctness via strongTier) and :80 (security) — the brief pins correctness at strong tier on the deep set; the security lane is dispatched strong today (pr-review-desk SKILL.md line 328, --kit review --tier strong); :88 and :96 put the two new deep lanes at the same tier, the conservative choice for lanes run on the least-trusted submissions; reversible knob.
RISK-VALUE: NAMED, NOT DERIVED — claimMarkerRe (assertion-marker word list) @ tools/desk/internal/deskkit/reviewlanes.go:232 — no spec or brief fixes which words mark a sentence as a claim and no row measures recall; the brief's pre-mortem assigns extraction quality to the review gate. A miss narrows the starting list but can never confirm a claim, since every extracted claim starts unverified. Routed, not closed.

Notes:
- BLOCKED (check-definition), not a product failure. Rows 1-11 pass; row 12 fails as authored on merged main (post-merge `--consumers` class, #1281) and passes by hand at the implementing commit 2083064fd with `--base cabdc0b84` (exit 0, 3 corroborated, 0 disproved). `statusgen brief --check-verified` with a hypothetical flip exits 1 on row 12 alone. Advancing needs row 12 re-authored to a form that holds on merged main, then a re-verify.
- Rows 1 to 11 pass on merged main by hand (darwin) and in the Linux network-off witness. The earlier could-not-run on rows 8 and 9 (darwin host, no unshare) is resolved: both pass hermetically.
- Row 12 fails as authored and its substance passes by a hand procedure. On merged main the brief is not in the diff, so --consumers answers COULD-NOT-CHECK exit 2 and structurally can never pass after merge (the post-merge --consumers class, #1281). Re-run against the implementing commit 2083064fd with --base its parent cabdc0b84, the same statusgen corroborates all three fixed-here entries, disproves none, and leaves the out-of-scope trusttier.go entry unchecked by design. This is a check-definition failure, so the verdict is BLOCKED, not PASS: row 12 needs re-authoring to a form that is decidable on merged main (for example pinning the implementing commit and its parent via --base), after which the item can be re-verified.
- Mutation probes on rows 3 and 4 were rerun independently in a throwaway clone; both mutants were killed and reverted.
- The changelog fragment named in the brief's files list is no longer on main: the release roll-up 36da06d35 aggregated it into CHANGELOG.md line 1306 and cleared the fragment directory. Present by content, not by path.
- The skill paragraph (SKILL.md line 348) is neutral: it names the resolver and says the tier source is a project-layer value; skillslint HOUSE-VALUES passes.
- Lint emits a risk-files-crossread NOTICE for this brief (all-no risk answers on a security-trigger path). The brief's own facts argue the answers explicitly (adds lanes only for the lowest tiers, no admit/authorize/write predicate changed); the code read agrees: ReviewLanesForAuthor only reads ResolveTier and returns lanes.

VERIFY: BLOCKED

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
