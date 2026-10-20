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
version: 2
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
| 12 | `d=; d=$(mktemp -d "${TMPDIR:-/tmp}/ct04-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 2083064fdeb279d886f4b40e5761b3e958961c61 && statusgen --consumers --root "$d" --brief assay:assay:contributor-trust:04 --base cabdc0b8455aa532fcb9267946c0e8da2e93bb5f; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | exit 0; output contains `summary: 3 corroborated, 0 disproved, 1 unchecked`; output does not contain `DISPROVED`; output does not contain `COULD-NOT-CHECK` (the `consumers:` claims are judged against the change that delivered them: a throwaway shared clone, made in owned scratch outside the checkout and removed afterwards, checked out at 2083064fdeb2, the squash that implemented this brief, with the base pinned to its parent cabdc0b8455a, so the diff it reads is exactly the implementing change, never main's later commits and never the runner's own working tree. The three `fixed-here` entries are corroborated by that diff; the one UNCHECKED entry is the `out-of-scope` trusttier.go line, which that diff does not touch by design and which stays the reviewer's call per brief-rule 9, never a pass. Row re-authored 2026-10-14: the earlier unpinned form ran against the runner's own head, where a merged brief is no longer in the diff, so it could only ever answer COULD-NOT-CHECK with exit 2 after the merge; the pinned form gives the same answer before and after the merge) | check |

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

### Non-implementer verifier re-run: 2026-10-02 21:26 (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main 5a108baba705c5f9b458501065fc52a12557c969

Re-run triggered by a changed declared input (the pr-review-desk skill body) since the 2026-09-27 verify-fail. Every row executed by hand on the host (darwin/arm64, go1.27.1, statusgen v1.0.31), offline envelope; the Go test rows ran under a throwaway HOME.

| # | Command | Expect | Observed (exit + key output line) | Date / Runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1` | exit 0; contains `ok` | exit 0; `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit 0.409s` | 2026-10-02 assay-verifier-app[bot] |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v` | exit 0; contains `fact-check`, `fail-first`, `security` | exit 0; `--- PASS: TestReviewLanesUnknownTierGetsTheDeepSet`; logged lanes correctness, security, fact-check, fail-first | 2026-10-02 assay-verifier-app[bot] |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v` | exit 0; no `fail-first` | exit 0; `--- PASS: TestReviewLanesContributorTierKeepsTheStandardPath`; logged lanes correctness and security only, zero occurrences of fail-first | 2026-10-02 assay-verifier-app[bot] |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestClaimStateUnverifiedIsRepresentable` | 2026-10-02 assay-verifier-app[bot] |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestClaimEveryClaimCarriesAState`; `extracted 6 claims from the sample body` | 2026-10-02 assay-verifier-app[bot] |
| 6 | `grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md` | exit 0; at least one line | exit 0; 4 matching lines (63, 70, 82, 101); line 63 reads "unverified — it could not be checked from the change alone." | 2026-10-02 assay-verifier-app[bot] |
| 7 | `grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md` | exit 0; at least one line | exit 0; 2 matching lines (93, 99); line 93 reads "Base, failing. Run the reported failing case at the merge base of the" | 2026-10-02 assay-verifier-app[bot] |
| 8 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | exit 0 | exit 0; no output (direct host execution; the witness's network-off re-execution of this check:ci row needs a Linux runner, see witness below) | 2026-10-02 assay-verifier-app[bot] |
| 9 | `cd tools/skillslint && go run . --root ../..; echo rc=$?` | contains `rc=0` | exit 0; `rc=0`; `HOUSE-VALUES: PASS — 39 markdown file(s) under plugins/`; advisory soft-budget NOTICE lines only (direct host execution; witness as row 8) | 2026-10-02 assay-verifier-app[bot] |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestReviewLanesReferenceMatchesTable` | 2026-10-02 assay-verifier-app[bot] |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestReviewLanesDispatchEndToEnd`; absent ledger gives tier=unknown lanes=[correctness security fact-check fail-first]; roster identity gives tier=maintainer lanes=[correctness security] | 2026-10-02 assay-verifier-app[bot] |
| 12 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:04` | exit 0; no `DISPROVED`; no `COULD-NOT-CHECK`; contains `corroborated` | **FAIL** — exit 2; `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:contributor-trust:04 is not in the diff against 5a108baba705…, so this run carries no evidence about its claims — no entry was corroborated and none was disproved.` | 2026-10-02 assay-verifier-app[bot] |

**Execution witness** (`statusgen verifyrun --brief`, v1.0.31, exit 2): 9 of 12 rows pass (1–7, 10, 11). Rows 8 and 9: could-not-run — the witness re-executes check:ci rows network-off via `unshare --net`, a Linux facility, and this host is darwin; both pass by direct host execution above, and the hermetic re-execution belongs to a Linux runner. Row 12: fail exit=2, expected 0. The witness table was captured and its write to this brief discarded.

**Findings.**

- The verdict is unchanged from 2026-09-27. The only previously failing row, row 12, still fails with the same shape: on merged main the brief is not in the diff, so the command as authored structurally answers COULD-NOT-CHECK and exits 2. statusgen moved v1.0.27 → v1.0.31 between the two runs; the behaviour of this command form on a merged brief did not change. #1281 is still OPEN.
- The input change that woke this re-run does not bear on row 12. Between the two verified heads the pr-review-desk skill body changed (unrelated review-desk edits) and this brief gained its Evidence; the lane table source, its test and the dispatch reference are byte-unchanged. The skill body still carries the tier-keyed paragraph (line 354) and row 9's skill lint stays green over the edited body.
- Supporting evidence for the substantive consumers claim, not a substitute for the row: the same corroboration at the implementing commit 2083064fd against its parent cabdc0b84 exits 0 with `summary: 3 corroborated, 0 disproved, 1 unchecked`; the one unchecked entry is the declared out-of-scope trusttier.go consumer, unchanged by design.
- Row 12 will keep failing on merged main until either the row is re-baselined to a form that can run post-merge (pinning the implementing commit and its base) or #1281 changes what the command does for a merged brief. This is a check-definition blocker, not an implementation defect.
- No mutation probes were re-run in this pass (read-only pass; the 2026-09-27 probes for rows 3 and 4 stand against an unchanged source file).

Risk-bearing values: the source file holding them is byte-unchanged since the 2026-09-27 pass, so the enumeration there stands and was re-read at this head:

RISK-VALUE: DERIVED — laneTable TierUnknown/TierBlessedOnce = {strongTier(LaneCorrectness), LaneSecurity, LaneFactCheck, LaneFailFirst}; TierContributor/TierMaintainer = {LaneCorrectness, LaneSecurity} @ tools/desk/internal/deskkit/reviewlanes.go:119-122 — matches the brief's "Lane set by tier" fact verbatim; only adds review for the two lowest tiers; reversible by edit and redeploy.
RISK-VALUE: DERIVED — out-of-range tier fallback set = laneTable[TierUnknown] @ tools/desk/internal/deskkit/reviewlanes.go:137 — fail-closed to the deepest set, consistent with "with no ledger every external identity is unknown".
RISK-VALUE: NAMED, NOT DERIVED — claimMarkerRe (assertion-marker word list) @ tools/desk/internal/deskkit/reviewlanes.go:232 — no spec or brief fixes which verbs mark a sentence as a claim and no row measures extraction recall; the brief's pre-mortem leaves extraction quality to the review gate. Every extracted claim starts unverified, so a miss narrows the list but never confirms a claim. Reversible. Still an open question, routed not closed.

VERIFY: FAIL — row 12 only (check-definition): `statusgen --consumers --brief` on merged main returns COULD-NOT-CHECK exit 2 because the brief is not in the diff (#1281, open). Rows 1–11 pass on merged main 5a108baba705 by direct execution; rows 8–9 lack a hermetic witness on this host. Status stays implemented.
### Non-implementer verifier re-run: 2026-10-07 (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main 91f04b81ba064394aa121a4940cf11a138b55402

Re-run triggered by changed inputs since the 2026-10-02 verify-fail (six pr-review-desk skill-body commits; statusgen v1.0.31 to v1.0.32 on this host). Every row executed by hand on the host (darwin/arm64, statusgen v1.0.32), offline envelope; the Go test rows ran under a throwaway HOME. The lane table source, its test file and the dispatch reference are byte-unchanged since 5a108baba705.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1` | exit 0; contains `ok` | exit 0; `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit 0.995s` | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v` | exit 0; contains `fact-check`, `fail-first`, `security` | exit 0; `--- PASS: TestReviewLanesUnknownTierGetsTheDeepSet`; logged lanes correctness, security, fact-check, fail-first | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v` | exit 0; no `fail-first` | exit 0; `--- PASS: TestReviewLanesContributorTierKeepsTheStandardPath`; logged lanes correctness and security only; zero occurrences of fail-first in output | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestClaimStateUnverifiedIsRepresentable` | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestClaimEveryClaimCarriesAState`; `extracted 6 claims from the sample body` | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md` | exit 0; at least one line | exit 0; 4 matching lines (63, 70, 82, 101); line 63 reads "unverified — it could not be checked from the change alone." | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md` | exit 0; at least one line | exit 0; 2 matching lines (93, 99); line 93 reads "Base, failing. Run the reported failing case at the merge base of the" | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | exit 0 | exit 0; no output (direct host execution; the witness's network-off re-execution of this check:ci row needs a Linux runner, see witness note) | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/skillslint && go run . --root ../..; echo rc=$?` | contains `rc=0` | exit 0; `rc=0`; `HOUSE-VALUES: PASS — 40 markdown file(s) under plugins/`; SKILLSLINT, HIDDEN-CHARS, GUARDRAILS, ENFORCEMENT-BLOCK, POSIX-TOKEN all PASS; advisory NOTICE lines only (direct host execution; witness as row 8) | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestReviewLanesReferenceMatchesTable` | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestReviewLanesDispatchEndToEnd`; absent ledger gives tier=unknown lanes=[correctness security fact-check fail-first]; contributor row gives [correctness security]; blessed-once row gives the deep set; broken ledger gives tier=unknown deep set; roster identity gives tier=maintainer lanes=[correctness security] | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:04` | exit 0; no `DISPROVED`; no `COULD-NOT-CHECK`; contains `corroborated` | **FAIL** — exit 2 (statusgen v1.0.32, same with the real HOME and a throwaway HOME); `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:contributor-trust:04 is not in the diff against 91f04b81ba064394aa121a4940cf11a138b55402, so this run carries no evidence about its claims — no entry was corroborated and none was disproved.` | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |

**Execution witness** (`statusgen verifyrun --brief`, v1.0.32, exit 2): 9 of 12 rows pass (1–7, 10, 11). Rows 8 and 9: could-not-run — the witness re-executes check:ci rows network-off via `unshare --net`, a Linux facility, and this host is darwin; both pass by direct host execution above, and the hermetic re-execution belongs to a Linux runner. Row 12: fail exit=2, expected 0. The witness table was captured and its write to this brief discarded.

**Findings.**

- The failure still reproduces. Row 12 fails with the identical shape on merged main 91f04b81ba06: the brief is not in the diff against HEAD, so the command as authored structurally answers COULD-NOT-CHECK and exits 2. statusgen moved v1.0.31 to v1.0.32; the only behavioural change visible for this command form is a new remediation hint ("To corroborate a MERGED brief, re-run against the diff that made the claims: check out the branch head and pass its own merge-base"). medici-finance/assay#1281 is still OPEN (read 2026-10-07).
- The changed inputs do not bear on row 12. Six pr-review-desk skill-body commits landed since 5a108baba705 (#2059, #2062, #2126, #2141, #2226, #2231); the tier-keyed paragraph is still present ("Lane depth is tier-keyed for external authors", SKILL.md line 361) and row 9's skill lint stays green over the edited body.
- Supporting evidence for the substantive consumers claim, not a substitute for the row: following the v1.0.32 hint, the same corroboration at the implementing commit 2083064fdeb2 with base its parent cabdc0b8455a exits 0, `summary: 3 corroborated, 0 disproved, 1 unchecked`; the one unchecked entry is the declared out-of-scope trusttier.go consumer, unchanged by design.
- Row 12 will keep failing on merged main until the row is re-baselined to a post-merge-runnable form (pin the implementing commit 2083064fdeb2 and pass base cabdc0b8455a, as the tool's own hint now spells out) or #1281 changes what the command does for a merged brief. Check-definition blocker, not an implementation defect.
- The changelog fragment named in the brief's files list is absent at its path on main because the fragment aggregation commit 36da06d3 cleared changelog/ — the normal fragment lifecycle; no Verify row covers it.
- No mutation probes re-run (read-only pass; the 2026-09-27 probes for rows 3 and 4 stand against a byte-unchanged source file).

Risk-bearing values (risk frontmatter present, all four `no`, irreversible: no — enumeration re-read at this head; reviewlanes.go byte-unchanged since the prior passes):

RISK-VALUE: DERIVED — laneTable TierUnknown/TierBlessedOnce = {strongTier(LaneCorrectness), LaneSecurity, LaneFactCheck, LaneFailFirst}; TierContributor/TierMaintainer = {LaneCorrectness, LaneSecurity} @ tools/desk/internal/deskkit/reviewlanes.go:119-122 — matches the brief's "Lane set by tier" fact verbatim; only adds review for the two lowest tiers; reversible by edit and redeploy.
RISK-VALUE: DERIVED — out-of-range tier fallback set = laneTable[TierUnknown] @ tools/desk/internal/deskkit/reviewlanes.go:137 — fail-closed to the deepest set, consistent with "with no ledger every external identity is unknown".
RISK-VALUE: DERIVED — ExecTier = "strong" @ tools/desk/internal/deskkit/reviewlanes.go:80, :88, :96, :105 — the brief pins correctness at strong tier for the deep set; reversible knob.
RISK-VALUE: NAMED, NOT DERIVED — claimMarkerRe (assertion-marker word list) @ tools/desk/internal/deskkit/reviewlanes.go:232 — no spec or brief fixes which verbs mark a sentence as a claim and no row measures extraction recall; the brief's pre-mortem leaves extraction quality to the review gate. Every extracted claim starts unverified, so a miss narrows the list but never confirms a claim. Reversible. Open question, routed not closed.

VERIFY: FAIL — row 12 only (check-definition): `statusgen --consumers --brief` on merged main 91f04b81ba06 still returns COULD-NOT-CHECK exit 2 because the brief is not in the diff (#1281, open). Rows 1–11 pass on merged main by direct execution; rows 8–9 lack a hermetic witness on this host. Status stays implemented.

Blocker re-confirmed and attached: https://github.com/medici-finance/assay/issues/1281#issuecomment-6037650539. The NAMED, NOT DERIVED value (claimMarkerRe at reviewlanes.go:232) is recorded on that issue.

### Non-implementer verifier re-run: 2026-10-11 (UTC), assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian), merged main bfef0dab238ad5aad468f736d78daa8c4c236dd7

Verification-Attestation: medici-finance/assay#2557 run=f94fe30c1723798e00a3b3ccab29a0c0f6a34742a09da83d215126e90e9570a6 source=bfef0dab238ad5aad468f736d78daa8c4c236dd7 brief=docs/streams/contributor-trust/brief-04-review-depth-by-tier.md model=claude-opus-5-5 tier=any

Re-run on changed inputs since the 2026-10-07 verify-fail at 91f04b81ba06: seven pr-review-desk skill-body commits, one new test in reviewlanes_test.go (TestEveryTierLaneSetIncludesCorrectness, from #2445), and statusgen v1.0.32 to v1.0.37 on this host. reviewlanes.go and the dispatch reference are byte-unchanged since 91f04b81ba06. Admission: `deskdispatch --check-verifier` exit 0 at source bfef0dab238a. Every row ran by hand on the host (darwin/arm64, go1.27.1, GOPROXY=off, statusgen v1.0.37), offline envelope, home clean before and after.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1` | exit 0; contains `ok` | exit 0; `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit 0.324s` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v` | exit 0; contains `fact-check`, `fail-first`, `security` | exit 0; `--- PASS: TestReviewLanesUnknownTierGetsTheDeepSet`; logged lanes correctness, security, fact-check, fail-first | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v` | exit 0; no `fail-first` | exit 0; `--- PASS: TestReviewLanesContributorTierKeepsTheStandardPath`; logged lanes correctness and security only; zero occurrences of fail-first in output. Mutation probe (scratch clone at bfef0dab238a, contributor row given LaneFailFirst): exit 1, `LanesFor(contributor) = [correctness security fail-first], want the standard path [correctness security]`, mutant killed | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestClaimStateUnverifiedIsRepresentable`. Mutation probe (scratch clone, both extraction sites seed ClaimConfirmed): exit 1, `extracted at state confirmed — extraction must start every claim unverified, never confirm it`, mutant killed | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestClaimEveryClaimCarriesAState`; `extracted 6 claims from the sample body` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | `grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md` | exit 0; at least one line | exit 0; 4 matching lines (63, 70, 82, 101); line 63 reads "unverified — it could not be checked from the change alone." | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | `grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md` | exit 0; at least one line | exit 0; 2 matching lines (93, 99); line 93 reads "Base, failing. Run the reported failing case at the merge base of the" | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | exit 0 | exit 0; no output (direct host execution; the witness's network-off re-execution of this check:ci row came back could-not-run on this darwin host, see witness note) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | `cd tools/skillslint && go run . --root ../..; echo rc=$?` | contains `rc=0` | exit 0; `rc=0`; SKILLSLINT PASS (16 skill files), HIDDEN-CHARS, `HOUSE-VALUES: PASS — 42 markdown file(s) under plugins/`, GUARDRAILS, ENFORCEMENT-BLOCK, POSIX-TOKEN all PASS; advisory NOTICE lines only (direct host execution; witness as row 8) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestReviewLanesReferenceMatchesTable` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v` | exit 0; contains `PASS` | exit 0; `--- PASS: TestReviewLanesDispatchEndToEnd`; absent ledger gives tier=unknown lanes=[correctness security fact-check fail-first]; contributor row gives [correctness security]; blessed-once row gives the deep set; broken ledger gives tier=unknown deep set; roster identity gives tier=maintainer lanes=[correctness security] | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 12 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:04` | exit 0; no `DISPROVED`; no `COULD-NOT-CHECK`; contains `corroborated` | **FAIL**: exit 2 (statusgen v1.0.37); `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:contributor-trust:04 is not in the diff against bfef0dab238ad5aad468f736d78daa8c4c236dd7, so this run carries no evidence about its claims — no entry was corroborated and none was disproved.` | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

**Execution witness** (`statusgen verifyrun --brief`, v1.0.37, `--dry-run`, exit 2): 9 of 12 rows pass (1–7, 10, 11), with output hashes for rows 6 and 7 identical to the 2026-09-27 and 2026-10-07 witnesses. Rows 8 and 9: could-not-run, `sandbox=unshare`. The witness re-executes check:ci rows network-off via `unshare --net`, which is Linux-only, and this host is darwin. Both rows pass by direct host execution above; the hermetic re-execution belongs to a Linux runner. Row 12: fail exit=2, expected 0. The dry run printed the witness table without writing it into this brief.

**Findings.**

- The failure still reproduces. Row 12 fails with the same shape on merged main bfef0dab238a: the brief is not in the diff against HEAD, so the command as authored structurally answers COULD-NOT-CHECK and exits 2. statusgen moved v1.0.32 to v1.0.37; this command form still answers the same way for a merged brief, and its remediation hint is unchanged. medici-finance/assay#1281 is still OPEN (read 2026-10-11, last updated 2026-10-09).
- The changed inputs do not affect row 12. The pr-review-desk skill body changed in seven commits since 91f04b81ba06 (#2272, #2431, #2440/#2441, #2366, #2445, #2454, #2474). The tier-keyed paragraph is still present ("Lane depth is tier-keyed for external authors", SKILL.md line 401), and row 9's skill lint stays green over the edited body. The one new test, TestEveryTierLaneSetIncludesCorrectness, is not selected by any row's -run pattern; it ran separately and passed (exit 0, `--- PASS: TestEveryTierLaneSetIncludesCorrectness`).
- Supporting evidence for the consumers claim, which does not replace row 12: in a fresh local clone in the verifier scratchpad (ct-04-impl), the same corroboration at the implementing commit 2083064fdeb2 with base cabdc0b8455a (its parent) exits 0: `summary: 3 corroborated, 0 disproved, 1 unchecked`. The one unchecked entry is the declared out-of-scope trusttier.go consumer, unchanged by design.
- Row 12 will keep failing on merged main until one of two things happens: the row is re-baselined to a form that runs after merge (pin commit 2083064fdeb2 and pass base cabdc0b8455a), or #1281 changes what the command does for a merged brief. Check-definition blocker, not an implementation defect.

Risk-bearing values (risk frontmatter present, all four `no`, irreversible: no; enumeration re-read at this head; reviewlanes.go byte-unchanged since 91f04b81ba06):

RISK-VALUE: DERIVED — laneTable TierUnknown/TierBlessedOnce = {strongTier(LaneCorrectness), LaneSecurity, LaneFactCheck, LaneFailFirst}; TierContributor/TierMaintainer = {LaneCorrectness, LaneSecurity} @ tools/desk/internal/deskkit/reviewlanes.go:119-122 — matches the brief's "Lane set by tier" fact word for word. It only adds review for the two lowest tiers, and an edit plus redeploy reverses it. Both directions killed by mutation this pass (row 3).
RISK-VALUE: DERIVED — out-of-range tier fallback set = laneTable[TierUnknown] @ tools/desk/internal/deskkit/reviewlanes.go:137 — fail-closed to the deepest set, consistent with "with no ledger every external identity is unknown". An empty set would dispatch no review.
RISK-VALUE: DERIVED — ExecTier = "strong" @ tools/desk/internal/deskkit/reviewlanes.go:80, :88, :96, :105 — the brief pins correctness at strong tier for the deep set; reversible knob.
RISK-VALUE: NAMED, NOT DERIVED — claimMarkerRe (assertion-marker word list) @ tools/desk/internal/deskkit/reviewlanes.go:232 — no spec or brief fixes which verbs mark a sentence as a claim, and no row measures extraction recall; the brief's pre-mortem leaves extraction quality to the review gate. Every extracted claim starts unverified (mutation-checked, row 4), so a miss shortens the list but never confirms a claim. Reversible. Open question, routed not closed.

**VERIFY: FAIL** — row 12 only (check-definition): `statusgen --consumers --brief` on merged main bfef0dab238a still returns COULD-NOT-CHECK exit 2 because the brief is not in the diff (#1281, open). Rows 1–11 pass on merged main by direct execution, with the two mutation probes killed. Rows 8–9 have no hermetic witness on this host. Status stays implemented.
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at c690023a864b (rows 1–11 pass, row 12 still COULD-NOT-CHECK exit 2 post-merge)

Verification-Attestation: medici-finance/assay#2926 run=b59e23808fc348b4da0ed28ff17902003657e2cfaa6d7d849762fccba7737f41 source=c690023a864b1c1cfdd55c0a28e19d22e572048d brief=docs/streams/contributor-trust/brief-04-review-depth-by-tier.md model=claude-opus-5-5 tier=strong

Re-run on merged main c690023a864b, 145 commits after the 2026-10-11 verify at bfef0dab238a. Since then the review code, its tests, the dispatch reference and the trust-tier reader have not changed a byte. The one relevant input that moved is the pr-review-desk skill body (+23/-8 lines, from #2548, #2598 and #2795). Its tier-keyed paragraph is still at SKILL.md line 401, and row 9 lints it green. Admission: `deskdispatch --check-verifier` exit 0 at source c690023a864b, run b59e2380…7f41, model claude-opus-5-5, tier strong. Every row ran by hand on the host (darwin/arm64, go1.27.2, statusgen v1.0.42) with KUBECONFIG=/dev/null. The home was clean before and after. Mutation probes and the supporting consumers check ran in separate scratch clones, not in the home. The changelog fragment from the implementing commit 2083064fdeb2 is no longer in changelog/ because the v1.0.16 aggregation folded it into CHANGELOG.md (36da06d35). Its three bullets are present there.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1` | pass exit=0 | `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit 0.271s` | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v` | pass exit=0 | `--- PASS` for Test Review Lanes Unknown Tier Gets The Deep Set; logged unknown-tier lanes: correctness, security, fact-check, fail-first (each of fact-check, fail-first, security present) | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v` | pass exit=0 | `--- PASS` for Test Review Lanes Contributor Tier Keeps The Standard Path; logged lanes correctness and security only; 0 occurrences of fail-first in output. Mutation probe (scratch clone at c690023a864b, contributor row given the deep set): exit 1, `LanesFor(contributor) = [correctness security fact-check fail-first], want the standard path [correctness security]`, mutant killed | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v` | pass exit=0 | `--- PASS` for Test Claim State Unverified Is Representable. Mutation probes (scratch clone): prose extraction seeded at confirmed gives exit 1, `extracted at state confirmed — extraction must start every claim unverified, never confirm it`, killed; state check accepting any value gives exit 1, killed (row 5's test also fails) | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v` | pass exit=0 | `--- PASS` for Test Claim Every Claim Carries A State; `extracted 6 claims from the sample body` | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 6 | `grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md` | pass exit=0 | 4 matching lines (63, 70, 82, 101); line 63 reads "unverified — it could not be checked from the change alone." | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 7 | `grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md` | pass exit=0 | 2 matching lines (93, 99); line 93 reads "Base, failing. Run the reported failing case at the merge base of the" | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 8 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | pass exit=0 | no output (direct host execution) | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 9 | `cd tools/skillslint && go run . --root ../..; echo rc=$?` | pass exit=0 | `rc=0`; SKILLSLINT PASS (17 skill files), `HOUSE-VALUES: PASS — 45 markdown file(s) under plugins/`, GUARDRAILS, ENFORCEMENT-BLOCK, POSIX-TOKEN PASS; only advisory soft-budget NOTICE lines | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v` | pass exit=0 | `--- PASS` for Test Review Lanes Reference Matches Table. Mutation probe (scratch clone, fail-first dropped from the unknown tier): this test, row 2's test and row 11's test all FAIL, exit 1, killed | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v` | pass exit=0 | `--- PASS` for Test Review Lanes Dispatch End To End; absent ledger: tier=unknown provenance=default-absent lanes=[correctness security fact-check fail-first]; contributor row: [correctness security]; blessed-once row: deep set; broken ledger: tier=unknown provenance=default-unreadable, deep set; roster identity: tier=maintainer lanes=[correctness security] | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |
| 12 | `statusgen --root . --consumers --brief assay:assay:contributor-trust:04` | fail exit=2 | statusgen v1.0.42: `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:contributor-trust:04 is not in the diff against c690023a864b1c1cfdd55c0a28e19d22e572048d, so this run carries no evidence about its claims — no entry was corroborated and none was disproved.` | 2026-10-14 | assay-verifier-app[bot] @ c690023a864b (on-behalf-of human:ian) (git-config) |

**Findings.**

- Row 12 fails the same way it did on 10-02, 10-07 and 10-11. The command runs against merged main, and the brief is not in that diff, so for a merged brief it can only answer COULD-NOT-CHECK with exit 2. statusgen moving from v1.0.37 to v1.0.42 did not change this. This is a check-definition blocker, not an implementation defect. It is tracked by medici-finance/assay#1915 (open, updated 2026-10-14), which covers Verify rows that run `statusgen --consumers --brief` after merge without a --base, and by medici-finance/assay#1281 (open), which covers the command's structural limit.
- Supporting evidence that does not replace row 12: in a separate scratch clone at the implementing commit 2083064fdeb2, running the same corroboration with --base cabdc0b8455a (its parent) exits 0 with `summary: 3 corroborated, 0 disproved, 1 unchecked`. The one unchecked entry is the trusttier.go consumer, which the brief declares out of scope and leaves unchanged by design.
- Neighbour tests that no row's -run pattern selects were run separately and pass: Test Every Tier Lane Set Includes Correctness, Test Review Lanes Blessed Once Tier Gets The Deep Set, Test Review Lanes Maintainer Tier Keeps The Standard Path, and Test Review Lanes Reference Path Stays Inside The Module (exit 0).
- Row 12 would pass if it were re-baselined to a form that works after merge: run at commit 2083064fdeb2 with base cabdc0b8455a. Changing the row is a brief-authoring decision, not a verifier's.

Risk-bearing values. The risk frontmatter is present with all four answers `no`, and irreversible is `no`. The trigger still fires because the diff touches tools/desk/internal/deskkit/, a security-trigger path. Enumeration covered the implementing diff 2083064fdeb2 (reviewlanes.go, the dispatch reference, mutations.json) and was re-read at c690023a864b. It found these literals: lane names "correctness", "security", "fact-check" and "fail-first" (lines 71, 78, 86, 94); ExecTier "strong" (lines 80, 88, 96, 105); the laneTable rows (lines 119–122); the out-of-range fallback (line 137); the ClaimState strings (lines 182–184); and claimMarkerRe (line 232). Ranked by irreversibility, every one can be undone with an edit and a redeploy. The lane table and its fallback rank first because they decide how much review an external author gets.

RISK-VALUE: DERIVED — laneTable TierUnknown/TierBlessedOnce = {strongTier(LaneCorrectness), LaneSecurity, LaneFactCheck, LaneFailFirst}; TierContributor/TierMaintainer = {LaneCorrectness, LaneSecurity} @ tools/desk/internal/deskkit/reviewlanes.go:119-122 — this is the brief's "Lane set by tier" fact, one for one. It only adds lanes for the two lowest tiers and widens no permission. Mutations in both directions were killed this run (rows 3 and 10).
RISK-VALUE: DERIVED — out-of-range tier fallback = laneTable[TierUnknown] @ tools/desk/internal/deskkit/reviewlanes.go:137 — this fails closed to the deepest set, matching the brief's "with no ledger every external identity is unknown". The alternative, an empty set, would dispatch no review at all.
RISK-VALUE: DERIVED — ExecTier = "strong" @ tools/desk/internal/deskkit/reviewlanes.go:80, :88, :96, :105 — the brief pins correctness at strong tier for the deep set (line 105), and the security, fact-check and fail-first lanes are strong-tier per the review desk's existing risk-keyed rule. This is a reversible dispatch knob.
RISK-VALUE: DERIVED — ClaimConfirmed/ClaimContradicted/ClaimUnverified = "confirmed"/"contradicted"/"unverified" @ tools/desk/internal/deskkit/reviewlanes.go:182-184 — these are the three-state vocabulary named in the brief's facts. That the states are distinct, and that no claim starts confirmed, was mutation-checked (row 4).
RISK-VALUE: NAMED, NOT DERIVED — claimMarkerRe (the assertion-marker word list) @ tools/desk/internal/deskkit/reviewlanes.go:232 — neither the spec nor the brief fixes which verbs mark a sentence as a claim, and no row measures extraction recall. The brief's pre-mortem leaves extraction quality to the review gate. Every extracted claim starts unverified, so a miss shortens the list but never confirms a claim. This is reversible and stays routed on medici-finance/assay#1916 (open).

**VERIFY: FAIL** — row 12 only (check-definition): on merged main c690023a864b, `statusgen --consumers --brief` returns COULD-NOT-CHECK exit 2 because the brief is not in the diff (medici-finance/assay#1915, medici-finance/assay#1281, both open). Rows 1–11 pass on merged main by direct execution, and all four mutation probes were killed. Status stays implemented.

### Non-implementer verifier run — 2026-10-16 claude-opus-5-5 verifier at 446ceaa76f9f (rows 1–12 pass)

Verification-Attestation: medici-finance/assay#3124 run=96084351840c0c63e4c089182e7c676dae9df7ef6e2bfaa1d66a91d11d77bf41 source=446ceaa76f9fc6abcc6135b98743e00da8aa99be brief=docs/streams/contributor-trust/brief-04-review-depth-by-tier.md model=claude-opus-5-5 tier=any

Run on merged main 446ceaa76f9f, 161 commits after the 2026-10-14 run at c690023a864b. Over that span the review code, its tests, the dispatch reference and the trust-tier reader are byte-identical. Two relevant inputs moved: this brief (row 12 re-authored to the pinned form) and the pr-review-desk skill body (+13/-5 lines), whose tier-keyed paragraph now sits at SKILL.md line 409 and which row 9 lints green. Admission: `deskdispatch --check-verifier` exit 0 at source 446ceaa76f9f, run 96084351…bf41, model claude-opus-5-5, tier any. Expected outcomes were written from the brief text before the implementing diff was read; row 12's expected 3/0/1 split was re-derived from the diff of 2083064fdeb2 against its parent cabdc0b8455a, which touches all three fixed-here paths and leaves trusttier.go alone. Every row ran by hand on the host (darwin/arm64, go1.27.2, statusgen v1.0.45) with KUBECONFIG=/dev/null and a throwaway HOME/TMPDIR outside the home. The home was clean before and after. Mutation probes ran in a separate scratch clone.

| # | Command | Expected | Observed | Runner |
|---|---------|----------|----------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1` | exit 0; output contains ok | exit 0; `ok  github.com/medici-finance/assay/tools/desk/internal/deskkit 0.349s` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v` | exit 0; output contains fact-check, fail-first, security | exit 0; `--- PASS: TestReviewLanesUnknownTierGetsTheDeepSet`; logged unknown-tier lanes correctness, security, fact-check, fail-first (all three required words present) | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v` | exit 0; output does not contain fail-first | exit 0; `--- PASS: TestReviewLanesContributorTierKeepsTheStandardPath`; logged lanes correctness and security only; 0 occurrences of fail-first. Mutation probe (scratch clone, contributor row given the deep set): exit 1, `LanesFor(contributor) = [correctness security fact-check fail-first], want the standard path [correctness security]`, mutant killed | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v` | exit 0; output contains PASS | exit 0; `--- PASS: TestClaimStateUnverifiedIsRepresentable`. Mutation probe (scratch clone, prose extraction seeded at confirmed): exit 1, `extracted at state confirmed — extraction must start every claim unverified, never confirm it`, mutant killed | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v` | exit 0; output contains PASS | exit 0; `--- PASS: TestClaimEveryClaimCarriesAState`; `extracted 6 claims from the sample body` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | `grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md` | exit 0; at least one matching line | exit 0; 4 matching lines (63, 70, 82, 101); line 63 reads "unverified — it could not be checked from the change alone." | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 7 | `grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md` | exit 0; at least one matching line | exit 0; 2 matching lines (93, 99); line 93 reads "Base, failing. Run the reported failing case at the merge base of the" | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 8 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | exit 0 | exit 0; no output | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 9 | `cd tools/skillslint && go run . --root ../..; echo rc=$?` | output contains rc=0 | exit 0; `rc=0`; SKILLSLINT PASS (17 skill files), `HOUSE-VALUES: PASS — 45 markdown file(s) under plugins/`, HIDDEN-CHARS, GUARDRAILS, ENFORCEMENT-BLOCK, AMBIENT-AUTH, POSIX-TOKEN PASS; only advisory soft-budget NOTICE lines | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v` | exit 0; output contains PASS | exit 0; `--- PASS: TestReviewLanesReferenceMatchesTable` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v` | exit 0; output contains PASS | exit 0; `--- PASS: TestReviewLanesDispatchEndToEnd`; absent ledger tier=unknown provenance=default-absent deep set; contributor row [correctness security]; blessed-once row deep set; broken ledger tier=unknown provenance=default-unreadable deep set; roster identity tier=maintainer [correctness security] | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 12 | `d=; d=$(mktemp -d "${TMPDIR:-/tmp}/ct04-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 2083064fdeb279d886f4b40e5761b3e958961c61 && statusgen --consumers --root "$d" --brief assay:assay:contributor-trust:04 --base cabdc0b8455aa532fcb9267946c0e8da2e93bb5f; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | exit 0; output contains summary: 3 corroborated, 0 disproved, 1 unchecked; no DISPROVED line; no unresolved-marker line | exit 0; `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`; CORROBORATED for review-lanes.md, reviewlanes.go and the pr-review-desk SKILL.md; UNCHECKED only for the out-of-scope trusttier.go entry; 0 matches for DISPROVED and 0 for the forbidden marker; scratch clone removed by the command | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

Risk-bearing values. The risk frontmatter is present with all four answers `no`, and irreversible is `no`. The trigger still fires because the implementing diff touches tools/desk/internal/deskkit/, a security-trigger path. Enumeration covered the implementing diff 2083064fdeb2 (reviewlanes.go, the dispatch reference, mutations.json), re-read at 446ceaa76f9f where reviewlanes.go is unchanged. Literals found: lane names "correctness", "security", "fact-check", "fail-first" (reviewlanes.go lines 71, 78, 86, 94); ExecTier "strong" (lines 80, 88, 96, 105); the laneTable rows (lines 119–122); the out-of-range fallback (line 137); the ClaimState strings (lines 182–184); claimMarkerRe (line 232). No numeric constant exists. Every entry is undone by an edit and a redeploy; the lane table and its fallback rank first because they decide how much review an external author gets.

RISK-VALUE: DERIVED — laneTable TierUnknown/TierBlessedOnce = {strongTier(LaneCorrectness), LaneSecurity, LaneFactCheck, LaneFailFirst}; TierContributor/TierMaintainer = {LaneCorrectness, LaneSecurity} @ tools/desk/internal/deskkit/reviewlanes.go:119-122 — one for one with the brief's "Lane set by tier" fact; the standard set equals the pre-change dispatch (correctness and security lanes, pr-review-desk SKILL.md line 255 at cabdc0b8455a), so known contributors are unchanged and only the two lowest tiers gain lanes. Mutation to the contributor row killed this run (row 3).
RISK-VALUE: DERIVED — out-of-range tier fallback = laneTable[TierUnknown] @ tools/desk/internal/deskkit/reviewlanes.go:137 — fails closed to the deepest set, consistent with the brief's "with no ledger every external identity is unknown"; the alternative, an empty set, would dispatch no review at all.
RISK-VALUE: DERIVED — ExecTier = "strong" @ tools/desk/internal/deskkit/reviewlanes.go:80, :88, :96, :105 — the brief pins correctness at strong tier on the deep set (line 105, applied by strongTier); security, fact-check and fail-first are strong-tier per the review desk's existing risk-keyed rule. A reversible dispatch knob.
RISK-VALUE: DERIVED — ClaimConfirmed/ClaimContradicted/ClaimUnverified = "confirmed"/"contradicted"/"unverified" @ tools/desk/internal/deskkit/reviewlanes.go:182-184 — verbatim the three-state vocabulary in the brief's facts; extraction starts every claim unverified, mutation-checked this run (row 4).
RISK-VALUE: NAMED, NOT DERIVED — claimMarkerRe (the assertion-marker word list) @ tools/desk/internal/deskkit/reviewlanes.go:232 — neither the spec nor the brief fixes which verbs mark a sentence as a claim, and no row measures extraction recall; the brief's pre-mortem leaves extraction quality to the review gate. Every extracted claim starts unverified, so a miss shortens the list but never confirms a claim. Reversible; remains routed on medici-finance/assay#1916.

**VERIFY: PASS** — 12/12 rows pass on merged main 446ceaa76f9f by direct execution; row 12 in its re-authored pinned form exits 0 with 3 corroborated, 0 disproved, 1 unchecked (the out-of-scope entry); both mutation probes killed. Gate is model; risk all no.

### Execution witness — 2026-10-19 claude-opus-5-5 verifier at 2f44050f8a70 (12/12 rows pass in a network-off container)

Verification-Attestation: medici-finance/assay#3565 run=88eca35830f1c9bc29460e425ec6cb923a240408acb5db28c2739116408509e3 source=2f44050f8a706d0865e1a95ca3cd8709abe1bd2f brief=docs/streams/contributor-trust/brief-04-review-depth-by-tier.md model=claude-opus-5-5 tier=strong

This is a mechanical execution witness on merged main 2f44050f8a70, not a fresh review, recorded under the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). Runner class: a local docker container, image assay-verify:go1.27 (built FROM golang:1.27.2-trixie, git 2.47), started with `--network none`, GOPROXY=off, GOTOOLCHAIN=local, the engine's default seccomp profile, with statusgen and the desk tools built from the attested tree inside the container; `statusgen verifyrun --sandbox container-netns` ran every Verify row in a throwaway clone at the attested source and wrote the table below. Admission: `deskdispatch --check-verifier` exit 0 at source 2f44050f8a70 before the witness, under the attestation above.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanes' -count=1` | pass exit=0 sandbox=container-netns | sha256:54f60be27166 | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesUnknownTier' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:a424870b6cfb | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesContributorTier' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:e3fb8719f9ee | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimStateUnverifiedIsRepresentable' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:f5fbf7a23417 | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ClaimEveryClaimCarriesAState' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:572de86e2dd2 | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -n 'unverified' tools/desk/cmd/deskdispatch/references/review-lanes.md` | pass exit=0 sandbox=container-netns | sha256:affec666f7bd | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -n 'merge base' tools/desk/cmd/deskdispatch/references/review-lanes.md` | pass exit=0 sandbox=container-netns | sha256:fb5b5d540866 | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && GOWORK=off go build ./... && GOWORK=off go vet ./internal/deskkit/` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 9 | `cd tools/skillslint && go run . --root ../..; echo rc=$?` | pass exit=0 sandbox=container-netns | sha256:6544fd42ba12 | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesReferenceMatchesTable' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:99fd5c396464 | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 11 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'ReviewLanesDispatchEndToEnd' -count=1 -v` | pass exit=0 sandbox=container-netns | sha256:cb66bcd6e7a1 | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |
| 12 | `d=; d=$(mktemp -d "${TMPDIR:-/tmp}/ct04-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 2083064fdeb279d886f4b40e5761b3e958961c61 && statusgen --consumers --root "$d" --brief assay:assay:contributor-trust:04 --base cabdc0b8455aa532fcb9267946c0e8da2e93bb5f; s=$?; test -n "$d" && rm -rf "$d"; exit $s` | pass exit=0 sandbox=container-netns | sha256:cdb3314a95e5 | 2026-10-19 | assay-verifier-app[bot] @ 2f44050f8a70 (on-behalf-of human:ian) (forge-identity) |

The substance is the existing by-hand Evidence block of 2026-10-16 on merged main 446ceaa76f9f above, where all 12 rows passed with 0 fail, both mutation probes were killed and the risk-bearing values were enumerated; this witness adds only the execution record on a network-off runner.

**VERIFY: PASS** — 12/12 rows pass with 0 fail in the network-off container witness at 2f44050f8a70 (sandbox=container-netns), matching the 2026-10-16 by-hand pass; gate is model and every risk answer is no.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
