---
brief: assay:assay:desktools-v2:65
title: "Measure own forge request timing and GraphQL cost"
why: "HTTP counts alone cannot explain forge latency or quota consumption. Existing transport and provider response seams must expose actual attempts, durations and attributable GraphQL cost without issuing monitoring requests or mistaking shared quota deltas for per-request charges."
wave: 3
depends: ["desktools-v2/63"]
unblocks: ["desktools-v2/62", "performance-collection/02", "desktools-v2/68"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v3
version: 3
id: 72fd07fd-5323-4631-8807-fc07c277ca3c
authored: 2026-10-19 by performance attribution authoring
sources:
  - docs/performance-attribution-spec.md
  - docs/streams/desktools-v2/spec.md
  - docs/telemetry.md
  - docs/library-first.md
  - "https://docs.github.com/en/graphql/overview/rate-limits-and-query-limits-for-the-graphql-api#returning-the-point-value-of-a-query (checked 2026-10-19)"
  - "https://docs.gitlab.com/api/graphql/getting_started/ (checked 2026-10-19; complexity is not GitHub quota points)"
  - "freshness-checked 2026-10-19 @ 2e02111ed; perf-v2 implementation 2013b8dc6, no perf-v3 or perfdata module"
exec-tier: strong
exec-tier-why: "Cross-process identity, compatibility and negative-path coverage require design judgment across shared consumers."
domain: complicated
outcome: none
consumers:
  - "tools/desk/cmd/desktoken/: fixed-here (observe the authentication HTTP leaf at its current owner at pickup, including relocation by /58; never read secret material)"
  - "tools/desk/internal/deskkit/forgetransport.go and forge backends: fixed-here"
  - "tools/desk/cmd/deskboard/readcost.go: fixed-here"
  - "tools/desk/internal/credential/: follow-up desktools-v2/62 (planned provider from /58; observe its authentication HTTP at integration)"
  - "perfrec/: follow-up desktools-v2/63"
  - "tools/desk/cmd/deskboard and deskdispatch: follow-up desktools-v2/62"
  - "statusgen/: out-of-scope (child owns provider calls; no authenticated backend import)"
  - "perfdata/adapters/: follow-up performance-collection/02"
  - "tools/desk/internal/forgeservice/: follow-up forge-neutral/45"
---

# Brief 65 — Measure own forge request timing and GraphQL cost

## Context

files:
- `tools/desk/internal/deskkit/forgetransport.go`
- `tools/desk/internal/deskkit/forge_github.go`
- `tools/desk/internal/deskkit/forge_gitlab.go`
- `tools/desk/internal/deskkit/perf.go`
- `tools/desk/internal/deskkit/perfcoverage_test.go`
- `tools/desk/internal/deskkit/forgeobserve.go`, `forgepoints.go`, and `forgeobserve_test.go`
- `tools/desk/cmd/deskboard/readcost.go`
- `tools/desk/cmd/desktoken/` (existing direct authentication HTTP; future credential provider integration belongs to /62)
- `docs/telemetry.md`
- `tools/desk/README.md`
- `changelog/perf-v3-forge-cost.md`

Implementation pickup: authentication HTTP still resides in `cmd/desktoken`;
the future `/58` provider has not replaced it. The supported explicit observer,
public-schema cost qualification, caller gaps, rollback and measured overhead
are recorded in [Own provider observations](../../provider-performance.md).
Shared `/63` and `/64` are consumed as real dependency merges; their acceptance
and this brief's independent Verify evidence remain separate.

facts (source and date in frontmatter):
- The counting transport at 2e02111ed counts attempts but does not time or inspect bodies. deskkit PerfRun observes it as a lower-bound http counter.
- Existing backend/board seams already classify actual reads, 304s and local quota-floor refusals. Reuse them; do not create a second incompatible request counter.
- Provider responses may omit attributable cost. Remaining/used rate gauges are shared and cannot prove one request's point consumption.

layering: perfrec owns typed optional observations; existing CLI, launcher and provider adapters retain effects and authority. No new service or credential boundary.

single-point-of-failure: a producer can mislabel optional context or coverage; independent writer/profile validation and production-flow negative controls expose this without granting operational authority.

design-fit:
  owner: perfrec and existing producer adapters
  contract: docs/performance-attribution-spec.md
  retires: [unattributed new performance samples and implicit unknown-as-zero accounting]
  weight: "verbs +0; profiles +2; shared flags +8 across the track, owned by /64; no new sender, service or credential holder"
  why-add: "Reuse the local recorder and effect seams; three bounded M briefs separate schema, propagation and provider observation."

Risk derivation: this observer adds bounded numeric/classification data only; it must not
read or emit credential values, token response fields, raw URLs or headers, change custody,
or add an authenticated call. The auth negative fixture proves secret-bearing inputs stay
with their existing owner. If implementation needs a disclosure or authority change, revise
the risk/gate before extending scope; the current model gate does not authorize that change.

## Read first

Read docs/performance-attribution-spec.md, the Context sources and the implemented /61.
Refresh current source and active work before pickup; all new paths and named tests below are
deliverables, not existing implementation or acceptance evidence.

At pickup, resolve the actual authentication HTTP leaf: `desktoken` owns it at the source
pin, but /58 may relocate it into the credential provider before this work lands. Observe
that existing leaf at its current owner, using its supported seam; do not recreate the old
leaf, revive withdrawn /33 or require a parser migration. /62 owns production credential
integration and qualification. Update the consumer inventory to the resolved path while
retaining the same secret-negative fixture and no-extra-request contract.

## Ground rules

Feature branch and draft PR only; no merge, release, workflow dispatch or live infrastructure.
Synthetic providers, isolated homes and disposable local processes only. Preserve custody,
claims, budgets, audit, help/version and operation results. Optional collection cannot become
required accounting. Stop at implemented; independent verification owns Evidence.

## Interface contract

Implement docs/performance-attribution-spec.md's own-process request contract. Both forges may share totals with bounded protocol/resource/unit classes. Count retries/304s/errors only at the actual transport leaf; measure dispatch through body EOF/close or terminal error with explicit incomplete coverage. Provider adapters supply bounded actual cost when already observed; estimates require method/version, and unknown/unsupported/failed is never zero. No separate monitoring request, expanded business selection or new credentials. Qualified bounded cost metadata may join the same existing operation; disclose its overhead. Parent processes do not scrape or charge their children's calls.

## Task

1. Reuse the transport leaf and request-meter owner to observe count/outcome and monotonic lifetime through response-body EOF/close/error. Preserve bytes, read/close/cancellation/error behavior without eager read/buffer. Wrapper composition and retry paths count once per actual attempt; cached/local refusals cost zero. Bound samples or use documented mergeable histograms with explicit coverage.
2. At existing GitHub/GitLab response decoding seams, extract attributable GraphQL cost from that operation's existing response path; when supported, add bounded cost metadata to the same operation (GitHub rateLimit.cost, GitLab query complexity in its own unit), never an additional request. Record numeric zero only when measured, count missing observations, and separate protocol/resource/unit and estimated method/version. Shared quota gauges carry window/reset provenance and are never differenced or summed as actual cost. Do not add separate cost/dry-run requests, expand business selections, re-read bodies or acquire new credentials. Qualify supported backend metadata/error semantics without failing the operation for missing telemetry.
3. Feed typed observations to /63's recorder and existing deskperf reader. Keep existing desktoken auth/service HTTP distinct from forge-provider operations, preserve retry/304 distinctions and own-process accounting. Unsupported custom clients and killed/unfinished requests remain coverage gaps. Concurrent request time is work, not additive elapsed time.
4. Qualify through deterministic synthetic transports/provider fixtures, compiled command records and the real report reader. Compare instrumentation-on/off calls, response bytes, exits and cancellation; exercise body errors/early close, retries, zero/missing cost and concurrent unrelated quota usage. Publish actual coverage and on/off overhead; never claim complete fleet coverage from one transport.

## Verify

Named tests below are planned. Each command requires its exact PASS line; a missing test
cannot pass. Plant the stated defect through the production boundary, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ForgeAttemptTimingFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3ForgeAttemptTimingFlow ' "$out")` | exit 0; named PASS. Production backend/transport/record/report counts exact retry,304,error and canceled attempts with body-lifetime timing; local cache/refusal and existing auth HTTP are distinct; planted Authorization headers, token-bearing auth responses and secret URL values must not enter records; planted double-count/timer-at-headers fails. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3GraphQLCostCoverage$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3GraphQLCostCoverage ' "$out")` | exit 0; named PASS. Existing response cost yields measured zero/nonzero while missing/unsupported/failed/estimated remain distinct; unrelated concurrent quota changes cannot become actual cost; raw queries/URLs/bodies never enter records. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ForgeObservationParity$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3ForgeObservationParity ' "$out")` | exit 0; named PASS. On/off has identical provider calls and business selections/results, preserves provider body bytes through observation and the original errors/cancellation/result, with bounded metadata differences documented; parent/child fixture counts each request only in its emitting process and reports uninstrumented child coverage. |
| 4 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/65 --delivery 578a9f39d73ec48bdc07fbdfd3938edb463e7b9b --base 1b00388fb9c5bd17c0817ae3a49751607b0d9cae` | exit 0; changed consumer routing corroborates the actual diff; unavailable is not pass. |

## Pre-mortem and detection

Rows 1–3 plant the counterexamples named in Expect: identity/consent or request-count bugs,
false-zero/compatibility bugs, and changed operational behavior. Row 4 checks shared consumer
routing. Sufficiency of real-world coverage and local disclosure remains reviewer judgment;
fixture success never asserts live adoption or permission.

## Query/cache measurement amendment — 19 October 2026

Observe actual provider sends regardless of whether the caller reached them directly or
through a future query/cache/executor layer. A bounded caller-supplied origin distinguishes
direct, service-passthrough, refresh, poll, repair and unknown; it never grants authority or
triggers a request. Service ingress counts/timing and cache/coalescing outcomes are separate
observations, not provider calls. /63 freezes the record/reducer vocabulary; forge-neutral/39–45
owns the future service adapters. Extend TestPerfV3ForgeAttemptTimingFlow and
TestPerfV3ForgeObservationParity with direct versus mediated synthetic origins, measured-zero
covered hit versus unknown server coverage, shared refresh charged once, and unchanged
business output/call count when diagnostics are off. The planned credential provider belongs
to /58 and its authentication HTTP integration to /62; this brief uses only the existing
authentication HTTP leaf at its current owner and provider transports, so its dependency remains /63.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit and named result. -->

### Independent verification — 2026-10-21 (dispatched verifier, claude-opus-5-5, tier strong)

Subject: merged source 116a2f1494fca6c97209f10883861467f729216d. The home was detached at that
commit and clean before and after every run. Delivery is the two-parent merge 578a9f39d73e
(PR #3583). Its parents' unique merge-base is 331aba8f343c. The Verify section hashes the same
at the original, base, both delivery parents, the delivery and the source.
`deskpathguard rederive` re-derived 4 original rows, 0 author-added, all from base. Every
original row ID, Command, Expect and class below is the original one.
Admission: `deskdispatch --check-verifier` exited 0 before the first row and exited 0 again after
the last, with the same binding.

Verification-Attestation: medici-finance/assay#3927 run=b30084dc56d62755293720a8f2d1df7d478359b54d883a4fa06df6e08bbcb8c6 source=116a2f1494fca6c97209f10883861467f729216d brief=docs/streams/desktools-v2/brief-65-measure-own-forge-request-timing-and-graphql-cost.md model=claude-opus-5-5 tier=strong

Verification-Table: delivery=578a9f39d73ec48bdc07fbdfd3938edb463e7b9b base=331aba8f343c828763a1fae23c4cba26abff7560 original=3d737b4871bf604e698c6c96427bb89c5e794db7

Environment: macOS host, go1.27.2 darwin/arm64, git 2.56.0, pinned desk shims (statusgen
v1.0.54), offline, KUBECONFIG=/dev/null. Fail-first and mutation runs used a scratch clone
(created with --no-hardlinks, dead push URL) in a fresh mktemp -d dir outside the home.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ForgeAttemptTimingFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3ForgeAttemptTimingFlow ' "$out")` | exit 0; named PASS; exact retry/304/error/canceled attempts with body-lifetime timing; secret inputs stay out of records; planted double-count/timer-at-headers fails | pass: exit 0, `--- PASS: TestPerfV3ForgeAttemptTimingFlow (0.01s)`. Fail-first at base 331aba8f: exit 1 (named test absent, "no tests to run"). Mutation: the shipped forgeperf spec's "leaf observation removed" and "nested leaf counted twice" plants are CAUGHT by this test. The test asserts 5 actual sends = 5 reported attempts, 4 bounded outcome classes and full duration/byte coverage. Its record writer refuses the planted Authorization header, secret URL query/path and token-bearing error text | 2026-10-21 | assay-verifier-app[bot] @ 116a2f1494fca6c97209f10883861467f729216d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3GraphQLCostCoverage$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3GraphQLCostCoverage ' "$out")` | exit 0; named PASS; measured zero/nonzero vs missing/unsupported/failed/estimated stay distinct; unrelated quota changes never become cost; no raw query/URL/body in records | pass: exit 0, `--- PASS: TestPerfV3GraphQLCostCoverage (0.01s)`. Fail-first at base: exit 1 (named test absent). Mutation: "unobserved cost becomes zero" and "GitHub JSON-wrapper metadata capture removed" are CAUGHT. The test covers measured zero, missing as unknown, malformed as failed, GitLab complexity in its own unit, custom/unsupported schema not expanded, and an X-RateLimit-Used gauge that is not read as cost | 2026-10-21 | assay-verifier-app[bot] @ 116a2f1494fca6c97209f10883861467f729216d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ForgeObservationParity$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3ForgeObservationParity ' "$out")` | exit 0; named PASS; on/off identical calls and results; body bytes, errors and cancellation preserved; parent/child counted only in the emitting process; uninstrumented child is a coverage gap | pass: exit 0, `--- PASS: TestPerfV3ForgeObservationParity (0.17s)`. Fail-first at base: exit 1 (named test absent). Mutation: "timer ends at headers" is CAUGHT. The test asserts no eager read, the original read/close/cancel errors, auth kept as its own class, one count per parent/child session, and an unobserved child not shown as measured. Its on/off samples assert 2000 = 2000 provider sends and the same business result | 2026-10-21 | assay-verifier-app[bot] @ 116a2f1494fca6c97209f10883861467f729216d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `statusgen --root . --consumers --brief desktools-v2/65` | exit 0; changed consumer routing corroborates the actual diff; unavailable is not pass | could-not-check: exit 2, `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desktools-v2:65 is not in the diff against 116a2f1494fc…, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery.` Counted as not passed. The cause is ordering: this run's marker is not on main yet | 2026-10-21 | assay-verifier-app[bot] @ 116a2f1494fca6c97209f10883861467f729216d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Totals: 4 required rows. Rows 1, 2 and 3 pass, and each discriminates (fail-first red at base,
named plants caught). Row 4 is could-not-check (could-not-run accounting: 1 row).

Supplemental, no acceptance credit:
- Row 4 with the delivery named explicitly, `statusgen --root . --consumers --brief desktools-v2/65 --delivery 578a9f39d73ec48bdc07fbdfd3938edb463e7b9b --base 1b00388fb9c5bd17c0817ae3a49751607b0d9cae`: exit 0, `summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`. The one unchecked entry is statusgen/ (out-of-scope), which is a reviewer judgement. Row 4 has to be re-run as written once this Evidence, with its marker, is on main.
- `muhar -j 1 -spec internal/deskkit/forgeperf-mutations.json` at the source, run from tools/desk: `Harness healthy: baseline GREEN, positive control CAUGHT.` and `Totals: 11 caught, 0 NOT CAUGHT, 0 could-not-mutate.`, exit 0.

Advisory (no verdict weight):
- The 19 October query/cache amendment sits outside the protected Verify table. The named tests use only the direct and poll origins. The verifier found no service-passthrough/refresh/repair origin case, no measured-zero covered hit versus unknown server coverage case, and no shared-refresh-charged-once case in TestPerfV3ForgeAttemptTimingFlow or TestPerfV3ForgeObservationParity.
- The Row 1 Expect also says existing auth HTTP stays distinct and token-bearing auth responses stay out of records. The auth-class separation is asserted in the row 3 test. The desktoken auth fixture (TestAuthInvocationRecord) is outside the Verify rows. The verifier did not run it as a row.

RISK-VALUE: DERIVED — GitHub same-operation metadata selection = ` _assayPerf:rateLimit{cost}` @ tools/desk/internal/deskkit/forgepoints.go:26 — GitHub's GraphQL rate-limit documentation says the `rateLimit { cost }` field returns the point value of the query it is selected in. Aliasing it as `_assayPerf` keeps it from colliding with business fields. It is added only for the closed compiled query inventory (forgepoints.go:15) and only against `GitHubAPIBase = "https://api.github.com"` @ tools/desk/internal/deskkit/forge.go:51 (gate @ forge_github.go:267). That matches the brief: GitHub rateLimit.cost, same operation, no extra request.
RISK-VALUE: DERIVED — GitLab metadata selection = ` _assayPerf:queryComplexity{score}` @ tools/desk/internal/deskkit/forge_gitlab.go:1375, read key `"score"` @ tools/desk/internal/deskkit/forgepoints.go:74, gated on `GitLabAPIBase = "https://gitlab.com"` @ forge_gitlab.go:65 — GitLab's GraphQL documentation exposes `queryComplexity { score limit }` as the query's complexity. The brief says this is a complexity unit and not GitHub points, and the code records it under its own unit (UnitGitLabComplexity).
RISK-VALUE: DERIVED — negative-cost rejection `n < 0` → failed @ tools/desk/internal/deskkit/forgepoints.go:81, and missing/null metadata → unknown @ forgepoints.go:64 and :77 — provider cost and complexity are non-negative counts. The brief requires that missing is never zero and that malformed is failed. The "unobserved cost becomes zero" plant is caught.
Ranked lower, reversible: the GraphQL path labels `"/graphql"` and `"/api/graphql"` @ tools/desk/internal/deskkit/forgeobserve.go:91; the outcome classes `status / 100` @ forgeobserve.go:118 with 304 split out @ :115; and the test-only overhead sampling (20 cycles × 100 requests). These are classification and measurement knobs, fixable by an edit and a redeploy.

**VERIFY: BLOCKED** — rows 1–3 pass and discriminate. Row 4 is could-not-check: statusgen cannot derive the delivery until this Verification-Table marker lands on main. After landing, re-run row 4 as written at the landing commit.

### Citation addendum — 2026-10-22 (dispatched verifier, claude-opus-5-5, tier strong)

Append-only. This section re-runs no Verify row and edits no earlier section. It adds the
blocker citation and corrects the remedy stated in the section headed "Independent
verification — 2026-10-21 (dispatched verifier, claude-opus-5-5, tier strong)".

Blocker: row 4 is tracked on #3826 (the row 4 consumer command binds neither delivery nor
base; open). Its defect class is #1915 (open). The decision on whether #3826 proceeds as a
one-off exception is pending on #3832 (open, needs-decision, no ruling recorded).

Correction, superseding the remedy in the section headed "Independent verification —
2026-10-21 (dispatched verifier, claude-opus-5-5, tier strong)": that section's row 4
Observed cell ("The cause is ordering"), its first Supplemental bullet ("Row 4 has to be
re-run as written once this Evidence, with its marker, is on main") and its VERIFY line
("After landing, re-run row 4 as written at the landing commit") name the marker landing
as the whole remedy. It is not. After this Evidence lands, a row 4 re-run as written can exit
0, but that pass cannot support closure. On a branch that closes this brief, lint raises a
PROBLEM on row 4 under the consumer-delivery-range rule. Confirmed from statusgen source at
f1e7cae3aff8: statusgen/strength.go lists ruleConsumerRange in promotedRowRules (tagged
#1915). statusgen/consumerdelivery.go consumerRangeWhy returns "consumer command has no
explicit --base (the default becomes HEAD on merged main)" for any `--consumers` command
without `--base`, which is row 4's command as written. statusgen/verifyrows.go then reports a
promoted rule as a PROBLEM when the branch closes the brief ("an unfailable row cannot be the
basis of a new closure"), and as a NOTICE otherwise. So closure needs both conditions: this
Evidence with its Verification-Table marker on main, and the row 4 definition repair on
#3826 (decision #3832), made through the protected Verify-table amendment path, followed by
a freshly admitted re-run of the repaired row.

Rows, tally and verdict are unchanged: 4 required rows; rows 1, 2 and 3 pass; row 4 is
could-not-check (could-not-run accounting: 1 row).

**VERIFY: BLOCKED** — rows 1–3 pass and discriminate. Row 4 is could-not-check. Blocker: #3826 (class #1915, decision pending on #3832). Closure needs this Evidence's Verification-Table marker on main and the #3826 row 4 definition repair; a re-run as written after landing alone cannot support closure.

### Independent re-verification after acceptance transition — 2026-10-23 (dispatched verifier, claude-opus-5-5, tier strong)

Subject: merged source 4a4b8e4f324834ba75aa2f211c14655dda4d4bfd. The home was detached at that
commit and clean before and after every run. Admission: `deskdispatch --check-verifier` exited 0
before the first row and exited 0 again after the last, with the same binding (attestation
#4239, run 5105533d19d3).

Transition coordinates, checked independently before execution: delivery 578a9f39d73e is the
two-parent merge of PR #3583 on the observed first-parent chain; its parents' unique merge-base
is 331aba8f343c. Original 2dbc1d762d07 is the last commit touching the brief at or before base
and is an ancestor of base. The Verify section hashes identically at original, base, delivery,
the amendment merge's first parent and the amendment branch's merge-base 9e33c4f82cc1; it
differs only from amendment e2b00ebe0f49 (merge of PR #4228) onward. The amendment diff touches
two files: row 4's Command code span and `version:` 2 to 3 in the brief, plus a changelog
fragment. The approval comment on #4230 (issuecomment-6373797887) is by the ratifying human (human:ian), unedited
(created = updated), and its payload names these exact coordinates and rows=4.

Transition checker: `statusgen verifyrun --root <home> --brief <brief> --acceptance-record <descriptor outside the home> --acceptance-forge --dry-run`
printed `acceptance transition: reading approval https://github.com/medici-finance/assay/issues/4230#issuecomment-6373797887`
and then executed the canonical plan: original rows 1–3 unchanged, approved amended row 4. It
did not hold or refuse the transition. `statusgen --root . --lint --acceptance-forge` at the
source: exit 0, `LINT: PASS`.

Environment: macOS host (darwin arm64), go1.27.2, git 2.56.0, installed desk shims and
statusgen v1.0.57, offline apart from the read-only approval lookup, KUBECONFIG=/dev/null.
Container image for the rule-45 bracket: the v1.0.57 release pin
ghcr.io/medici-finance/verify-runner:v1.0.57@sha256:1b0f3a9a32a452b618fcf6761571af35f33239416482b7f748210f7b772566f3
(asset checksum matched the release checksums file; image statusgen reports v1.0.57).

Check:ci rows 1–3, container lane (rule 45): `--open-bracket` exited 0 (opened
2026-10-23T20:16:39Z, tree sha256:d421a14660c5). The `--sandbox=container-netns` step in a
`--network none` container exited 2 before any row:
`statusgen verifyrun: could-not-attribute — no executing identity is available (no GITHUB_ACTOR under GitHub Actions, no git user.name/user.email in this repo).`
No witness was written, so there is nothing to close and no Admission-Bracket line exists.
Blocker: #4208 (the linked-worktree home's gitdir is not in the container, so it has no runner
identity). The host run below records the same rows as could-not-run, because darwin has no
`unshare --net`.

Disclosed attempts, no credit: (1) the first host transition run passed the brief as a path
relative to the home and exited 2 with `protected table: acceptance transition: cannot resolve brief path`
before any row ran; it was re-issued unchanged except for the absolute brief path. (2) The
container-netns step above, refused as quoted. No other run was made or discarded.

Verification-Table: delivery=578a9f39d73ec48bdc07fbdfd3938edb463e7b9b base=331aba8f343c828763a1fae23c4cba26abff7560 original=2dbc1d762d07ac2a2eb7575de2ddd40bf48dc081
Acceptance-Transition: amendment=e2b00ebe0f495037dcfd1483429cfe8ef3c24570 approval=https://github.com/medici-finance/assay/issues/4230#issuecomment-6373797887

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ForgeAttemptTimingFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3ForgeAttemptTimingFlow ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ 4a4b8e4f324834ba75aa2f211c14655dda4d4bfd (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3GraphQLCostCoverage$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3GraphQLCostCoverage ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ 4a4b8e4f324834ba75aa2f211c14655dda4d4bfd (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ForgeObservationParity$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3ForgeObservationParity ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-23 | assay-verifier-app[bot] @ 4a4b8e4f324834ba75aa2f211c14655dda4d4bfd (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --root . --consumers --brief desktools-v2/65 --delivery 578a9f39d73ec48bdc07fbdfd3938edb463e7b9b --base 1b00388fb9c5bd17c0817ae3a49751607b0d9cae` | pass exit=0 | sha256:7aaa72d7a28b | 2026-10-23 | assay-verifier-app[bot] @ 4a4b8e4f324834ba75aa2f211c14655dda4d4bfd (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4239 run=5105533d19d3991d088c4cfa52ee8c345f9b941750d44e28c0f271ce06773c82 source=4a4b8e4f324834ba75aa2f211c14655dda4d4bfd brief=docs/streams/desktools-v2/brief-65-measure-own-forge-request-timing-and-graphql-cost.md model=claude-opus-5-5 tier=strong

Row 4 primary output, inspected by re-running the same command at the source (supplemental
inspection, no separate credit): exit 0, `consumers corroboration — base 1b00388fb9c5bd17c0817ae3a49751607b0d9cae..578a9f39d73ec48bdc07fbdfd3938edb463e7b9b (immutable delivered snapshot), 1 brief(s)`,
`summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`. The one
unchecked entry is statusgen/ (out-of-scope), a reviewer judgement that the row's Expect leaves
to review. The witness Output hash covers verifyrun's own captured stream, so it differs from a
hash of stdout alone.

Totals: 4 required original rows. Row 4 passes (approved amended command). Rows 1, 2 and 3
are could-not-run (could-not-run accounting: 3 rows; blocker #4208 for the container lane,
no `unshare --net` on the darwin host). No obligation was observed failing.

RISK-VALUE: DERIVED — GitHub same-operation metadata selection = ` _assayPerf:rateLimit{cost}` @ tools/desk/internal/deskkit/forgepoints.go:27 — GitHub's GraphQL rate-limit documentation returns the point value of the enclosing query through `rateLimit { cost }`; the brief (Task 2) names exactly this field and forbids a separate request. The `_assayPerf` alias keeps it apart from business fields, and it is appended only to the closed compiled query list in the same file.
RISK-VALUE: DERIVED — GitLab metadata selection = ` _assayPerf:queryComplexity{score}` @ tools/desk/internal/deskkit/forge_gitlab.go:1397, read key `"score"` @ tools/desk/internal/deskkit/forgepoints.go:75 — GitLab's GraphQL `queryComplexity { score }` is the query's complexity in GitLab's own unit; perfrec keeps it as unit gitlab-complexity, never GitHub points, as the brief requires.
RISK-VALUE: DERIVED — negative-cost rejection `n < 0` → failed @ tools/desk/internal/deskkit/forgepoints.go:82, missing or null metadata → unknown @ forgepoints.go:65 and :78 — provider cost and complexity are non-negative counts; the brief requires that missing is never zero and that numeric zero is recorded only when measured.
Ranked lower, reversible: outcome classes `status / 100` @ tools/desk/internal/deskkit/forgeobserve.go:118 with 304 split out @ :115, and the GraphQL deferral `status/100 == 2` @ forgeobserve.go:157. These are classification knobs, fixable by an edit and a redeploy. All entries are reversible; the brief is irreversible: no, gate model, and the lint risk-files cross-read NOTICE on tools/desk/internal/deskkit/ is why this enumeration ran.

**VERIFY: BLOCKED** — row 4 passes under the approved acceptance transition (#4230); rows 1–3 are could-not-run (container lane refused could-not-attribute, blocker #4208; no unshare on the darwin host). No obligation failed.
### Independent re-verification, container lane — 2026-10-27 (dispatched verifier, claude-opus-5-5, tier strong)

Subject: merged source e25479a69d69ad678c15253309fcac6b7b0c86b0. The home was detached at that commit and clean before and after every run. Admission: `deskdispatch --check-verifier` exited 0 before the first row (attestation #4588, run f9ce4d08268e), and the bracket close re-ran it with the same binding. The runner is not the implementer.

Transition coordinates, checked independently before execution: delivery 578a9f39d73e is the two-parent merge of PR #3583 (parents 1b00388fb9c5 and e4c3c2499e2d); the parents' merge-base is 331aba8f343c. Original 2dbc1d762d07 is the last commit touching the brief at or before base. The Verify section hashes identically at original, base, delivery and the amendment's first parent, and changes only at amendment e2b00ebe0f49 (merge of PR #4228), which equals the source. `deskpathguard rederive` re-derived 4 rows, 0 author-added. No Verify row carries a risk-bearing, live, mutating or end-to-end tag and every brief-wide risk answer is no, so no row owes a fail-first witness.

Environment: macOS host (darwin arm64), go1.27.2, git 2.56.0, installed statusgen v1.0.63, KUBECONFIG=/dev/null, offline apart from the read-only approval lookup.

Rows 1–3 (check:ci) ran in the container lane under rule 45, selected with `--rows 1,2,3`. Image: the v1.0.63 release pin ghcr.io/medici-finance/verify-runner:v1.0.63@sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8 (the release asset's checksum matched the release checksums file; the local image's repo digest matched). The container ran linux/amd64 with `--network none`, the home mounted read-only, the host Go module cache mounted read-only, and GOPROXY=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local. `--open-bracket` and `--close-bracket` ran on the host, and `--bracket-digest` reports verdict=match for the bracket below.

Disclosed attempts, no credit: (1) A first bracket (opened 2026-10-27T02:10:51Z) ran rows 1–3 with only the image's own module cache. All three recorded `fail exit=1` with the same output hash. A diagnostic run of the same go test in the same image showed `[setup failed]`: the image's module cache lacks golang.org/x/sys v0.49.0, golang.org/x/term v0.47.0, golang.org/x/net v0.61.0 and golang.org/x/crypto v0.58.0, which tools/desk/go.mod now requires, and the network is off. That witness was never closed and is discarded. (2) Diagnostic container runs outside any bracket (an environment probe, then the single row-1 test without and with the host module cache mounted read-only) located that cause and confirmed the build succeeds with the host module cache. None is credited. The fresh bracket below is the credited run. No other run was made or discarded.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ForgeAttemptTimingFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3ForgeAttemptTimingFlow ' "$out")` | pass exit=0 sandbox=container-netns | sha256:6a9a2eceb731 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3GraphQLCostCoverage$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3GraphQLCostCoverage ' "$out")` | pass exit=0 sandbox=container-netns | sha256:63b642185272 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3ForgeObservationParity$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3ForgeObservationParity ' "$out")` | pass exit=0 sandbox=container-netns | sha256:7b0368417e84 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:d515aab7266fbd4c6aebadc70cda8c12add58886b41248b41fc9b63dc2a03d0d
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4588 run=f9ce4d08268e61f44c2617e3d4a3362cb2ac566b42e440b879cb912e4718993e source=e25479a69d69ad678c15253309fcac6b7b0c86b0 brief=docs/streams/desktools-v2/brief-65-measure-own-forge-request-timing-and-graphql-cost.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=f9ce4d08268e61f44c2617e3d4a3362cb2ac566b42e440b879cb912e4718993e source=e25479a69d69ad678c15253309fcac6b7b0c86b0 tree=sha256:d515aab7266fbd4c6aebadc70cda8c12add58886b41248b41fc9b63dc2a03d0d witness=sha256:842a513f4e56f893ec445f0c3b9219b63b746f52fea6ea701263276a662eaa72 opened=2026-10-27T02:13:41Z closed=2026-10-27T02:15:18Z before=receipt after=check-verifier

Row 4 (check) ran on the host under the approved acceptance transition (#4230), selected with `--rows 4`, with `--acceptance-record` naming the coordinates below and `--acceptance-forge`. verifyrun printed `acceptance transition: reading approval https://github.com/medici-finance/assay/issues/4230#issuecomment-6373797887` and executed the approved amended row 4.

Verification-Table: delivery=578a9f39d73ec48bdc07fbdfd3938edb463e7b9b base=331aba8f343c828763a1fae23c4cba26abff7560 original=2dbc1d762d07ac2a2eb7575de2ddd40bf48dc081
Acceptance-Transition: amendment=e2b00ebe0f495037dcfd1483429cfe8ef3c24570 approval=https://github.com/medici-finance/assay/issues/4230#issuecomment-6373797887

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `statusgen --root . --consumers --brief desktools-v2/65 --delivery 578a9f39d73ec48bdc07fbdfd3938edb463e7b9b --base 1b00388fb9c5bd17c0817ae3a49751607b0d9cae` | pass exit=0 | sha256:77b65f0443a3 | 2026-10-27 | assay-verifier-app[bot] @ e25479a69d69ad678c15253309fcac6b7b0c86b0 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4588 run=f9ce4d08268e61f44c2617e3d4a3362cb2ac566b42e440b879cb912e4718993e source=e25479a69d69ad678c15253309fcac6b7b0c86b0 brief=docs/streams/desktools-v2/brief-65-measure-own-forge-request-timing-and-graphql-cost.md model=claude-opus-5-5 tier=strong

Results: rows 1–3 pass exit 0 in the container lane. Each produced its named `--- PASS:` line, with output hashes 6a9a2eceb731, 63b642185272 and 7b0368417e84. Row 4 passes exit 0 on the host (output hash 77b65f0443a3). A supplemental host run of the same row 4 command (no credit) printed `summary: 8 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`. The one unchecked claim names the statusgen/ tree, which the brief does not claim to change. `statusgen verifyrun --check --acceptance-forge` on this brief with this section committed reads: `4 pass, 0 fail, 0 could-not-run/missing (of 4 Verify rows); 0 author-added with no acceptance credit`.

Risk-bearing values. I enumerated literals over the delivery diff of #3583 (25 files: the deskkit forge observation, points, transport and resolve code, desktoken coverage/observation/gitlab, deskboard read cost, the mutation specs, and the provider-performance, telemetry and changelog docs), then ranked them. The status-class split and the 2xx test in forgeobserve.go are reversible classification knobs and rank last. The top-ranked values are the metadata selections that change what is sent to a provider:

RISK-VALUE: DERIVED — GitHub same-operation metadata selection = ` _assayPerf:rateLimit{cost}` @ tools/desk/internal/deskkit/forgepoints.go:27 — GitHub's GraphQL rate-limit documentation returns the point cost of the enclosing query through `rateLimit { cost }`; the brief (Task 2) names exactly this field and forbids a separate request. The `_assayPerf` alias keeps it apart from business fields. It is appended only to the closed compiled query list (forgepoints.go:15), and only when the base is `GitHubAPIBase = "https://api.github.com"` @ tools/desk/internal/credential/github.go:40 (gate @ tools/desk/internal/deskkit/forge_github.go:268).
RISK-VALUE: DERIVED — GitLab metadata selection = ` _assayPerf:queryComplexity{score}` @ tools/desk/internal/deskkit/forge_gitlab.go:1401, read key `"score"` @ tools/desk/internal/deskkit/forgepoints.go:75, gated on `GitLabAPIBase = "https://gitlab.com"` @ tools/desk/internal/deskkit/forge_gitlab.go:65 — GitLab's GraphQL `queryComplexity { score }` is the query's complexity in GitLab's own unit. The code records it as unit gitlab-complexity and never as GitHub points, as the brief requires.
RISK-VALUE: DERIVED — negative-cost rejection `n < 0` → failed @ tools/desk/internal/deskkit/forgepoints.go:82, and missing or null metadata → unknown @ forgepoints.go:65 and :78 — provider cost and complexity are non-negative counts. The brief requires that missing is never recorded as zero, and that numeric zero is recorded only when measured.

**VERIFY: PASS**

## Review

Gate: model. Reject changed legacy meanings, guessed identity/cost, lost coverage or optional measurement that changes an operation.
