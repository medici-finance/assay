---
brief: assay:assay:desk-tools:40
title: Deliver one review queue observation and offline planning bridge
why: "Review currently joins actions and prs from separate sweeps because the first output omits required information; the planner must consume one complete qualified observation."
wave: 3
depends: ["desk-tools/38", "desk-tools/39"]
unblocks: ["desk-tools/43"]
effort: M
gate: human
decision-issue: 4522
design: DR-desk-tools-40
ruling: "https://github.com/medici-finance/assay/issues/4522#issuecomment-6923695940"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 2
authored: 2026-10-17 by statusgen newbrief
sources:
  - "Planning amendment 2026-10-23; source checked on App-fetched main 9c4307a19a190964db084104ecc1f7bd62cba433; candidate-plus-priority parity and GH-call audit"
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Deliver one review queue observation and offline planning bridge'
id: a635247d-ba2c-4242-8518-860b3d4f30b6
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
gate-why: Review/issue observations can expose scoped operational facts; implemented coverage and authority boundaries require the existing human acceptance.
decision-trigger: spec
consumers:
  - "docs/deskboard.md: fixed-here"
  - "docs/streams/desk-tools/brief-43-migrate-review-callers-with-qualified-versions-and-rollback.md: fixed-here"
  - "tools/desk/cmd/deskboard/main.go: fixed-here"
  - "tools/desk/cmd/reviewloop/plan.go: fixed-here"
  - "docs/contracts.md: fixed-here"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desk-tools/43"
  - "plugins/assay/scripts/pr-monitor.sh: follow-up desk-tools/43"
  - "tools/desk/cmd/deskboard/throughput.go: follow-up desk-tools/43"
---

# Brief 40 — Deliver one review queue observation and offline planning bridge

## Context

files: `tools/desk/cmd/deskboard/` (existing), `tools/desk/cmd/reviewloop/` (existing), `tools/desk/internal/reviewflow/` (planned), `tools/desk/internal/reviewtest/` (planned), `docs/deskboard.md` (planned), `docs/desk-workflow-efficiency-spec.md` (existing), `changelog/desk-tools-40.md` (planned)

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: deskboard collection adapter and reviewloop pure plan adapter over reviewflow. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: deskboard collection adapter and reviewloop pure plan adapter over reviewflow
  contract: review-observation-v1 and approved spec A1–A3
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +1 review-queue, flags +2 at most, refusal call sites +5 (validation only; +0 authority), rule-text +0 duplicate policy; legacy adapters retained temporarily
  why-add: reuse existing contracts/runner/typed reads; a complete observation is required before removing the second read, and a new cache would preserve that duplication

## Implementation pickup — 23 October 2026

Pickup base `3e24567bb`; #4235 (desk-tools/39) and CLI port #3248 are merged.
The existing dispatch claim was acquired before implementation. Command ownership
remains deskboard/reviewloop; the operator contract is [docs/deskboard.md](../../deskboard.md).
Legacy role skill, monitor and throughput migration belongs to existing /43, not a
second /40 plan. No generated board or independent Evidence is advanced here.

Implementation weight qualification: the counter reports flags 584→586 and refusal
call sites 1441→1446. These five calls validate invocation shape; they add no grant
or write authority. Its verb scan remains 71 because it does not inventory the Cobra
command table; the actual CLI adds one verb, covered by the command inventory tests.

## Read first

- [Approved source](../../desk-workflow-efficiency-spec.md) and [consolidation](../../desk-workflow-efficiency-change-map.md).
- [Delivery waves](README.md#review-efficiency-follow-ons) and existing source files declared above.
- `docs/library-first.md`, `docs/brief-rules.md` and `docs/telemetry.md` for the applicable boundary/consent rules.

## Ground rules

- Preserve exact-head independent verdicts, claims, trust, budgets, audit and consequential rechecks.
- Use fake providers and disposable local Git for acceptance; live activation and collection are separate gates.
- Stop at implemented; independent verification supplies Evidence. Do not rewrite completed brief Evidence.

## Candidate and priority amendment — 2026-10-23

Implement the spec §2.1 inputs through the already-planned ReviewQueue/PlanReview bridge.
One frozen validated observation supplies actions, exact heads, rank inputs, all dispositions
and stage depths. Policy/time/capacity are explicit and revision-bound; the direct-reader
collector is separate from local reselection. Deliver no new daemon, global TTL or policy file.
The shared planner is called by prose and discrete adapters, not copied into their commands.
Register the new test in the existing internal/reviewtest scope. Producer compatibility and
consequential refresh remain as authored; this amendment grants no live consumer cutover.

## Task

1. Deliver ReviewQueue as deskboard review-queue --out FILE using existing typed ReviewQueueSnapshot
   and bounded pools. Extend typed queries only for required missing facts; no raw GraphQL in
   commands. Collect shared facts once per cycle, retaining conditional requests and per-change
   reuse. Depth and all planning rows come from the same population.
2. Deliver PlanReview as reviewloop plan --snapshot FILE with zero network/effects.
   Missing/unsupported/stale/incomplete observations cannot silently become empty queues or
   authorization. Keep legacy --actions/--prs forms during the declared compatibility window; add
   independently captured compatibility fixtures.
3. Expose observation generation/cutoff in reporting so same-cutoff consumers can reuse it without
   pretending a later request is fresh. Provide a compact summary plus complete artifact; no output
   cap may trim the population used for idle/depth.
4. Follow existing CLI contracts and coordinate #3248 before touching its adapter. This is the
   direct-reader bridge, independent of PostgreSQL, forge service and the future tiny desk-review-
   queue command. Document that future spelling as a migration target, not a delivered binary.
5. Update affected command/operator contracts and the declared changelog; regenerate applicable
   documentation with the existing statusgen README generator. No generated global board is branch-
   committed. All planned tests must be wired to actual production paths, not a parallel test
   implementation.

## Verify

Run from the public repository root; each named test is a planned deliverable and its PASS
line is mandatory (zero selected tests is a failure). No authoring-time execution Evidence
is asserted. Each test covers production flow plus the stated wrong-but-plausible mutation.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestQueuePlanBridge$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestQueuePlanBridge ' "$o")` | exit 0; named PASS. Fake provider -> real review-queue command -> saved observation -> offline plan produces exact oracle once; deleting any required head/scope/action input fails rather than yielding idle. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestQueuePartialAndDepth$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestQueuePartialAndDepth ' "$o")` | exit 0; named PASS. Partial/denied/overflow/stale/schema faults refuse complete or idle claims; report depth equals same uncapped population. Legacy actions/prs/plan stdout and exit contracts remain qualified. |
| 3 | check:ci +dereference | `statusgen --root . --consumers --brief desk-tools/40` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |
| 4 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPlanNoFetch$' ./... > "$o" && grep -F -- '--- PASS: TestReviewPlanNoFetch ' "$o")` | exit 0; named PASS. after one real fake-provider collection, repeat pure selection for slot completion and a policy-only reorder with zero additional GH attempts. Source/access/expiry invalidation requests bounded refresh or holds; a hidden actions/prs reread and stale-as-idle mutation each fail. |

## Pre-mortem and detection

| Failure mode | Caught by |
|---|---|
| Adapter produces plausible output while duplicating or omitting work | row 1, independent oracle and actual entrypoint/count instrumentation |
| A partial/unsupported observation or hidden consumer creates a false success | row 2, planted invalid input and explicit refusal/coverage |
| Operational activation or control deletion inferred from source qualification | review-only: review scope and separate existing adoption gates; no live effects in the test harness |

## Evidence

<!-- Independent verifier appends one row per Verify command after implementation. -->

### Independent verification — 2026-11-18, desk tools v1.0.87

Verifier: non-implementer, model claude-opus-5-5, tier strong. Host statusgen v1.0.87, darwin arm64, KUBECONFIG=/dev/null. Source 3a9ae0dec7fb7a9b5b889e64dab0d2412f908fea on main, attestation #6907 (`deskdispatch --check-verifier` admitted this run before any row). This is the item's first independent verification: the Evidence section held only the generated placeholder. The delivering change is #4248, merged on the first-parent chain as c243d2d116fcbb2d3add310c18dd863013eee31f; its first parent 3e24567bbc4e1c53c508e4f74d8ad04d3364d494 is the unique merge-base of its two parents. `deskpathguard rederive` reports 4 original rows, 0 author-added; the parsed Verify table is byte-identical at 3e24567bbc4e, at c243d2d116fc and at this source.

Verification-Table: delivery=c243d2d116fcbb2d3add310c18dd863013eee31f base=3e24567bbc4e1c53c508e4f74d8ad04d3364d494 original=3e24567bbc4e1c53c508e4f74d8ad04d3364d494

All four rows are check:ci, and this darwin host has no `unshare --net`, so all four ran through the container-netns admission bracket: host `--open-bracket` with `--module-cache` and `--history-out`, then a `--network none --read-only` container on the linux/arm64 manifest of the pinned verify-runner v1.0.87 image (the Container-Image line below) with `--fail-first --delivering-commit c243d2d116fc --history-pack`, every input mounted read-only, then host `--close-bracket`. `statusgen verifyrun --bracket-digest` on the closed witness reports `verdict=match` (exit 0). The witness bytes follow unchanged.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestQueuePlanBridge$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestQueuePlanBridge ' "$o")` | pass exit=0 sandbox=container-netns | sha256:13d60e75da71 | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=3e24567bbc4e unproven · head=3a9ae0dec7fb |
| 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestQueuePartialAndDepth$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestQueuePartialAndDepth ' "$o")` | pass exit=0 sandbox=container-netns | sha256:335a419d4ee1 | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=3e24567bbc4e unproven · head=3a9ae0dec7fb |
| 3 | `statusgen --root . --consumers --brief desk-tools/40` | fail exit=2 sandbox=container-netns | sha256:39c6b8099f6e | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=3e24567bbc4e red rc=2 · head=3a9ae0dec7fb · delivering=c243d2d116fc |
| 4 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPlanNoFetch$' ./... > "$o" && grep -F -- '--- PASS: TestReviewPlanNoFetch ' "$o")` | pass exit=0 sandbox=container-netns | sha256:516b477721f2 | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) | base=3e24567bbc4e unproven · head=3a9ae0dec7fb |
Tree-Digest: sha256:f5adc530563f3ac4ef06dbff6db23b96609d310fe9d6e583b49b785858ebf5ce
Container-Image: sha256:3cfd931fc8fb2277df25eafbc68eccfe84ce7d7b92379b927b8c1841d99854d1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6907 run=dfd9ffe2862998ebe018826d4ac7972beb9617b4114b83cc06bd4492db96bafd source=3a9ae0dec7fb7a9b5b889e64dab0d2412f908fea brief=docs/streams/desk-tools/brief-40-deliver-one-review-queue-observation-and-offline-planning-bridge.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=dfd9ffe2862998ebe018826d4ac7972beb9617b4114b83cc06bd4492db96bafd source=3a9ae0dec7fb7a9b5b889e64dab0d2412f908fea tree=sha256:f5adc530563f3ac4ef06dbff6db23b96609d310fe9d6e583b49b785858ebf5ce witness=sha256:b5d138b7c22ecb7b96982d9e68eced6b02d00b428cd56a57e23ebe3008c9591d opened=2026-11-18T15:20:22Z closed=2026-11-18T15:32:30Z before=receipt after=check-verifier

Row accounting: 4 rows. Rows 1, 2 and 4 pass at head; row 3 exits 2 (COULD-NOT-CHECK). No acceptance obligation was observed failing, but no row is fully discharged.

- Rows 1, 2 and 4: `pass exit=0` at head, each with its named `--- PASS:` line. The brief's `risk:` line carries `sensitive-data: yes`, so every row owes a fail-first witness. All three are `unproven` at base 3e24567bbc4e: the base run could not run, so no row was shown to discriminate. The cause is the lane, not the change. `--module-cache` provisions only the modules this source's go.sum names. The base tools/desk go.mod requires modules the cache does not hold, for example golang.org/x/image v0.0.0-20211028202545-6944b10bf410 where the cache holds only v0.47.0. With GOPROXY=off the base build stops on module resolution before the test selector runs.
- Row 3: `fail exit=2` at head and `red rc=2` at base. Exit 2 is COULD-NOT-CHECK; a disproved routing claim would be exit 1. On the host the same authored command (supplemental, below) prints that the brief is not in the diff against this source and that its delivery could not be derived: the committed Evidence has no Verification-Table marker, and no first-parent commit names `Brief: desk-tools/40`. The row names no `--delivery`/`--base`. This is the shared check-definition defect #4560, the same class desk-tools/39 row 3 is amending through #6856.

Fail-first: row 1 unproven, row 2 unproven, row 3 red rc=2 (could-not-check, not a discrimination result), row 4 unproven.

Supplemental host observations, no acceptance credit (2026-11-18, same verifier, run after the bracket closed, not sandboxed; diagnostics, not Evidence rows):

- S1: the authored row 3 command, `statusgen --root . --consumers --brief desk-tools/40`, exits 2. COULD-NOT-CHECK: the brief is not in the diff against 3a9ae0dec7fb, and its delivery could not be derived (no committed Verification-Table marker, no first-parent commit naming the brief, 4594 commits searched).
- S2: the same check pinned, `statusgen --root . --consumers --brief desk-tools/40 --delivery c243d2d116fcbb2d3add310c18dd863013eee31f --base 3e24567bbc4e1c53c508e4f74d8ad04d3364d494`, exits 0 with `summary: 8 corroborated, 0 disproved, 0 unchecked` over the immutable delivered snapshot: five fixed-here routes and three follow-up desk-tools/43 routes. PRODUCER-DELIVERY is COULD-NOT-CHECK by design, and its LIMIT line applies: an omitted consumer is invisible. This is not the authored command, so it earns no credit.
- S3: the bracket's provisioned module cache holds golang.org/x/image only at v0.47.0. The base go.mod's v0.0.0-20211028202545-6944b10bf410 is absent, which accounts for the unproven base runs.

Attempts: one attempt, 2026-11-18T15:28:50Z to 2026-11-18T15:31:59Z, rows 1-4, credited as the table above. No attempt was discarded or superseded.

Risk-bearing values, enumerated over the production files #4248 changed (tools/desk/cmd/deskboard/reviewobservation.go, main.go, board.go and prfacts.go; tools/desk/cmd/reviewloop/snapshot.go, plan.go and main.go) and ranked by irreversibility. The item is `irreversible: no`, and every entry can be undone by an edit and a redeploy. The highest rank goes to the custody and authority bindings around the sensitive observation. Enumerated: the artifact file mode (os.CreateTemp, 0600), the host qualification `Host: "github.com"` behind the ForgeGitHub-only guard, the scope-equality binding between selection and observation, the frozen-head equality on changed-file reads, the input provenance literals `Version: "deskboard-review-v1"`, `Freshness: "fresh"` and `Complete: true`, the policy prefix `"deskboard-review-v1:"`, the lane names, the selection-shape refusal (capacity and scope required), and the `review-queue` verb entries.
RISK-VALUE: DERIVED — `os.CreateTemp(filepath.Dir(path), ".review-observation-*")` mode 0600 @ tools/desk/cmd/deskboard/reviewobservation.go:153 (stated at :151) — the brief's gate-why says observations expose scoped operational facts, and docs/deskboard.md line 9 makes atomic 0600 the contract. os.CreateTemp creates the file 0600 whatever the umask, so only the owning operator can read it, and the rename installs it without following a final symlink.
RISK-VALUE: DERIVED — `!reflect.DeepEqual(req.Scope, o.Inputs.Requested)` refusal @ tools/desk/cmd/reviewloop/snapshot.go:45 — Task 2 says an incomplete or changed observation must never become an empty queue or an authorization. Exact equality of the requested scope is the only value that refuses both a narrowed and a widened selection; the refusal asks for a refresh.
RISK-VALUE: DERIVED — `pr.HeadSHA != expectedHead[0]` refusal @ tools/desk/cmd/deskboard/board.go:483 — the ground rules keep exact-head verdicts. A changed-file read whose head differs from the frozen queue head would attach facts to the wrong commit, so exact equality is the only safe bound.
RISK-VALUE: DERIVED — `Freshness: "fresh"`, `Complete: true` @ tools/desk/cmd/deskboard/reviewobservation.go:121 — spec/review-observation-v1.md line 71 requires freshness to be positively `fresh`. The values are stamped only after the truncated/quarantined refusal at :115 and the degraded-classification refusal at :125, so a stamped input is complete.
RISK-VALUE: NAMED, NOT DERIVED — `Host: "github.com"` @ tools/desk/cmd/deskboard/reviewobservation.go:113 — admitted only behind `resolution.Kind != deskkit.ForgeGitHub` at :110, and spec/review-observation-v1.md line 36 requires canonical lowercase host identities. This pass did not prove that `deskkit.ForgeKindFor` cannot return ForgeGitHub for a GitHub Enterprise host. The host classifier at tools/desk/internal/deskkit/forgegit.go:156 suggests it cannot, but nothing here shows ForgeKindFor uses it.
Lower-ranked identifiers (`"deskboard-review-v1"` @ reviewobservation.go:121 and :175, lane names `"correctness"`/`"security"` @ :138, the `req.Capacity == nil || len(req.Scope) == 0` refusal @ tools/desk/cmd/reviewloop/snapshot.go:34) are version labels or shape checks that an edit can reverse; they need no derivation.

**VERIFY: BLOCKED** — no acceptance obligation was observed failing. Rows 1, 2 and 4 pass at head, but their required fail-first witnesses are unproven: the bracket's module cache does not provision the base's module set. Row 3's authored consumers check exits 2 (COULD-NOT-CHECK) because the row binds no delivery and base (#4560). A supplemental pinned run corroborates all 8 routes but earns no credit.

## Review

Retain the derived frontmatter gate. Approval of the source spec is recorded; this brief
adds implementation acceptance rather than reopening that decision. Existing adopter
writer/custody/equivalence and collection consent gates remain separate.
