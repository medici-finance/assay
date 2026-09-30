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
### Non-implementer verifier run — VERIFY: FAIL — 11/12 pass, row 12 fails (check definition) — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Isolated worktree at merged main `b89b3957225e227e69d5b5ec7949344f580d9966` (HEAD == the forge's `commits/main`). Implementing change: squash commit 2083064fd (#1367). `gate: model`, all risk answers `no`. Offline envelope; scratch TMPDIR outside the repo; tree clean after the run. Re-woken because a declared input (the pr-review-desk skill) changed since the 2026-09-27 pass; the brief body is unchanged. Status stays `implemented`.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1` | exit 0 | PASS — exit 0, `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit` | 2026-09-30 | claude-opus-5-5-verifier |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v` | deep set | PASS — exit 0; Test Review Lanes Unknown Tier Gets The Deep Set; lanes correctness, security, fact-check, fail-first | 2026-09-30 | claude-opus-5-5-verifier |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v` | standard path | PASS — exit 0; lanes correctness and security only, zero fail-first. Mutation (contributor row given the fail-first lane): exit 1 — killed, restored | 2026-09-30 | claude-opus-5-5-verifier |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v` | exit 0 | PASS — exit 0. Mutation (extraction seeds confirmed): exit 1, `extraction must start every claim unverified` — killed, restored | 2026-09-30 | claude-opus-5-5-verifier |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v` | exit 0 | PASS — exit 0; 6 claims extracted from the sample body | 2026-09-30 | claude-opus-5-5-verifier |
| 6 | `grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md` | ≥1 line | PASS — exit 0; 4 lines, first at line 63 | 2026-09-30 | claude-opus-5-5-verifier |
| 7 | `grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md` | ≥1 line | PASS — exit 0; 2 lines, line 93 `Base, failing.` | 2026-09-30 | claude-opus-5-5-verifier |
| 8 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | exit 0 | PASS by direct run — exit 0, no output. check:ci: the hermetic network-off witness needs a Linux runner (#1800) | 2026-09-30 | claude-opus-5-5-verifier |
| 9 | `cd tools/skillslint && go run . --root ../..; echo rc=$?` | rc=0 | PASS by direct run — `rc=0`, `HOUSE-VALUES: PASS — 39 markdown file(s) under plugins/`. check:ci as row 8 | 2026-09-30 | claude-opus-5-5-verifier |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v` | exit 0 | PASS — exit 0 | 2026-09-30 | claude-opus-5-5-verifier |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v` | exit 0 | PASS — exit 0; no ledger → unknown → deep set; contributor → standard; broken ledger → unknown → deep set; roster maintainer → standard | 2026-09-30 | claude-opus-5-5-verifier |
| 12 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:04` | exit 0 | FAIL — exit 2 (statusgen v1.0.29), `COULD-NOT-CHECK: … is not in the diff against b89b3957…`. Structurally unrunnable on merged main after a squash merge (#1281). At the implementing commit (base its parent): exit 0, `3 corroborated, 0 disproved, 1 unchecked` (out-of-scope consumer) | 2026-09-30 | claude-opus-5-5-verifier |

RISK-VALUE (trigger fires: the diff touches `tools/desk/internal/deskkit/`; all values reversible):

- RISK-VALUE: DERIVED — laneTable unknown/blessed-once = {strong correctness, security, fact-check, fail-first}; contributor/maintainer = {correctness, security} @ tools/desk/internal/deskkit/reviewlanes.go:119-122 — matches the brief's "Lane set by tier" fact verbatim; row-3 mutant killed.
- RISK-VALUE: DERIVED — out-of-range fallback `laneTable[TierUnknown]` @ tools/desk/internal/deskkit/reviewlanes.go:137 — fails closed to the deepest set, per "every external identity is unknown".
- RISK-VALUE: DERIVED — ExecTier `strong` @ tools/desk/internal/deskkit/reviewlanes.go:80,88,96,105 — the brief fixes correctness at strong tier for the deep set.
- RISK-VALUE: NAMED, NOT DERIVED — `claimMarkerRe` word list @ tools/desk/internal/deskkit/reviewlanes.go:232 — no spec or brief fixes which words mark a claim; a miss only shortens the starting list (every claim starts unverified). Question attached to #1281 (issuecomment-5902864939).

Findings: row 12 is the #1281 check-definition class — re-verifying cannot change it; the row needs restating against the implementing diff, or #1281 needs to land. Note: the lane-selection function has no non-test caller under `tools/`, so tier-keyed depth is enforced by the pr-review-desk procedure, not by the dispatch binary.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
