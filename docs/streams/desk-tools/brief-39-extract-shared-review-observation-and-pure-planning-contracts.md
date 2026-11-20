---
brief: assay:assay:desk-tools:39
title: Extract shared review observation and pure planning contracts
why: "Queue classification and plan assembly live inside commands, so callers repeat collection and cannot consume one complete observation through a pure shared boundary."
wave: 2
depends: ["desk-tools/37"]
unblocks: ["forge-neutral/39", "desk-tools/40", "desk-tools/41", "graph-execution/27"]
effort: M
gate: human
decision-issue: 4521
design: DR-desk-tools-39
ruling: "https://github.com/medici-finance/assay/issues/4521#issuecomment-6615602021"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 3
authored: 2026-10-17 by statusgen newbrief
sources:
  - "Planning amendment 2026-10-23; source checked on App-fetched main 9c4307a19a190964db084104ecc1f7bd62cba433; candidate-plus-priority parity and GH-call audit"
  - docs/desk-workflow-efficiency-spec.md
  - docs/desk-workflow-efficiency-change-map.md
  - freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d (App-authenticated main fetch)
  - 'desk-tools: Extract shared review observation and pure planning contracts'
id: 5a306961-7f3a-4e54-8c3c-42d0a9ddf2a5
exec-tier: strong
exec-tier-why: Semantic extraction, cost coverage and cross-caller failure behavior require independent contract judgment.
domain: complicated
gate-why: Review/issue observations can expose scoped operational facts; implemented coverage and authority boundaries require the existing human acceptance.
decision-trigger: spec
consumers:
  - "tools/desk/internal/reviewflow/selection.go: fixed-here (remaining candidate-plus-priority amendment)"
  - "tools/desk/internal/reviewtest/selection_test.go: fixed-here (priority/capacity/coverage and mutation proof)"
  - "spec/review-observation-v1.md: fixed-here (Go selection API and compatibility contract)"
  - "docs/contracts.md: fixed-here (existing semantic-owner index)"
  - "tools/desk/internal/deskclient/: follow-up graph-execution/27"
  - "tools/desk/cmd/deskboard/: fixed-here (shared classification facts; observation collection follows in desk-tools/40)"
  - "tools/desk/cmd/reviewloop/: fixed-here (shared plan assembly; observation CLI handoff follows in desk-tools/40)"
  - "tools/desk/cmd/deskdispatch/: follow-up desk-tools/41 (consume the qualified observation during candidate admission)"
---

# Brief 39 — Extract shared review observation and pure planning contracts

## Context

files: `tools/desk/internal/reviewflow/` (planned), `tools/desk/internal/reviewtest/` (planned), `tools/desk/cmd/deskboard/board.go` (existing), `tools/desk/cmd/reviewloop/plan.go` (existing), `spec/review-observation-v1.md` (planned), `schemas/review-observation-v1.json` (planned), `docs/contracts.md` (existing), `changelog/desk-tools-39.md` (planned)

facts: 17 October 2026 at the source pin above. Existing implementation is described in
approved spec §2. All named tests and paths marked planned below are deliverables, not
passing claims. desk-tools/23 remains implemented, /24–25 done, /27 retains its own
verification debt; desktools-v2/09 is verified and /45 implemented. Refresh actual source,
claims and open PRs at pickup. #3248 and #3302 are active CLI ports; #3325 is proposed
supervision guidance and #3305 proposed generic improvement planning. Neither draft is
assumed shipped. The initial blocker is the missing complete observation and attribution,
not the full future runtime.

layering: tools/desk internal reviewflow, extracted from existing cmd classifiers/planner. Contracts and pure computation remain separated from
credentialed collection/effects; adapters consume values through legal module APIs.
No new shipped service, runtime store or credential owner is authorized. The flow and boundary
mutation cases below enforce this placement.

design-fit:
  owner: tools/desk internal reviewflow, extracted from existing cmd classifiers/planner
  contract: review-observation-v1 per approved spec §§3–5; statusgen/evaluation retains eligibility and coverage semantics
  retires: duplicate work only after the applicable caller and equivalence acceptance
  weight: verbs +0, flags +0, refusals +0, rule-text +0; shared contract replaces cmd-local copies
  why-add: reuse existing contracts/runner/typed reads; a complete observation is required before removing the second read, and a new cache would preserve that duplication

## Read first

- [Approved source](../../desk-workflow-efficiency-spec.md) and [consolidation](../../desk-workflow-efficiency-change-map.md).
- [Delivery waves](README.md#review-efficiency-follow-ons) and existing source files declared above.
- `docs/library-first.md`, `docs/brief-rules.md` and `docs/telemetry.md` for the applicable boundary/consent rules.

## Ground rules

- Preserve exact-head independent verdicts, claims, trust, budgets, audit and consequential rechecks.
- Use fake providers and disposable local Git for acceptance; live activation and collection are separate gates.
- Stop at implemented; independent verification supplies Evidence. Do not rewrite completed brief Evidence.

## Candidate and priority amendment — 2026-10-23

Extend the existing review observation/planning extraction, not its canonical domain meanings.
Define candidate observation and explicit approved policy/evaluator revision, time and capacity
as separate pure inputs, following efficiency spec §2.1 and A11–A12. Return ordered proposals
plus held/unknown/watch and unmapped dispositions; retain exact subject/lane, scope and reasons.
Policy rank is not coalescing-key order. Never drop a candidate absent from a priority mapping
or reconstruct full coverage from shown/capped rows. graph-execution/27 consumes this existing
reviewflow API as one role binding; other domain evaluators stay with their current owners.
Keep collection out of the pure library and preserve the internal/public module boundary.

Add the parity case below inside the already-declared internal/reviewtest test surface.
These are future implementation obligations; existing approval and Evidence are unchanged.

## Task

1. Specify typed review-observation-v1: requested/admitted scope, collection interval and per-input
   provenance/version/freshness/completeness; provider-qualified change identity, full head SHA,
   base SHA when used, body/review/check applicability; all action rows, lanes, rank, holds and
   WAIT/SURFACE/NO-OP. Derive depths from that same uncapped population. It is an interval
   observation, not an atomic provider snapshot.
2. Extract the actual existing classification and pure plan assembly into
   tools/desk/internal/reviewflow. Commands become adapters. Use intended-behavior oracle and
   historical regression fixtures before moving code; no command importing another command, broad
   rewrite or second ready predicate. Share the existing evaluation semantic owner through its legal
   module API; if missing, route extraction through that owner rather than copying it.
3. Pure packages take typed facts, perform no network/filesystem/process/credential reads and return
   explicit partial/stale/schema errors. Operational collection/auth remains with deskkit. The
   tools/desk internal package is not importable from statusgen/forgeread/other modules; eventual
   public serving extraction belongs to forge-neutral/39.
4. Publish contract and consumer compatibility fixtures. Review observations are untrusted advisory
   input, not claim/effect authority. Preserve coalescing, exact-head lanes and existing action
   table; do not merge independent reviewer conclusions.
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
| 1 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewContractFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewContractFlow ' "$o")` | exit 0; named PASS. Existing and extracted production classifiers/planners match independent oracle across heads/body/reviews/checks/lanes/order/holds/uncapped depths. Mutation of one applicability or idle input fails. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewBoundary$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewBoundary ' "$o")` | exit 0; named PASS. Root contract/planner runs with provider effects and credentials forbidden; schema rejects wrong versions, missing scope and incomplete-as-empty. Cross-module import probe cannot consume desk internal packages. |
| 3 | check:ci +dereference | `git grep -qF PlanCandidates HEAD -- spec/review-observation-v1.md && routes=$(statusgen --root . --consumers --brief desk-tools/39 --delivery 790003516c1fb4ea99d0a6d5eb6e67e9123b3c2a --base 4b7f4847f7f3531185fdfb23fbf978ae9551a242) && h=$(mktemp) && trap 'rm -f "$h"' 0 && git cat-file blob c488296801fcdc542108c955d92e85e63d16067a > "$h" && sh "$h" "$routes"` | exit 0; implementation consumer routing is corroborated against the actual diff. Out-of-scope judgments remain review obligations. |
| 4 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPriorityJoin$' ./... > "$o" && grep -F -- '--- PASS: TestReviewPriorityJoin ' "$o")` | exit 0; named PASS. the extracted pure contract matches an independent oracle for identical observation/policy/time/capacity, selected head/lane/order and held/unknown sets. Actual driver integration is accepted downstream in /43 and graph-execution/27; it is not a prerequisite here. Permute identity keys against rank, include unmapped and capped candidates, change policy only and deny provider/process/file reads; a second sorter or any provider call fails. |

## Pre-mortem and detection

| Failure mode | Caught by |
|---|---|
| Adapter produces plausible output while duplicating or omitting work | row 1, independent oracle and actual entrypoint/count instrumentation |
| A partial/unsupported observation or hidden consumer creates a false success | row 2, planted invalid input and explicit refusal/coverage |
| Operational activation or control deletion inferred from source qualification | review-only: review scope and separate existing adoption gates; no live effects in the test harness |

## Evidence

<!-- Independent verifier appends one row per Verify command after implementation. -->

### Independent re-verification — 2026-11-08, desk tools v1.0.78

Verifier: non-implementer, model claude-opus-5-5, tier strong. Host statusgen v1.0.78, darwin arm64, KUBECONFIG=/dev/null, no forge identity exported, normal HOME. Source 2e1ddcbd72c2dbe2674821bd7eac6c16c6189e0f on main, attestation #5777. This is the item's first independent verification: no earlier verify-outcome record exists for desk-tools/39. The delivering changes are #3570 (8c2f9706c9d5), #3593 (merge 790003516c1f) and #4235 (merge f9cba0a88d39). The risk line carries `sensitive-data: yes`, so every row is risk-bearing. Fail-first ran against base 85fcd48feaf0, the parent of the earliest delivery #3570. That base holds the brief and carries none of the three named tests.

All four rows are check:ci. On this darwin host they ran through the container-netns admission bracket: host `--open-bracket` with `--history-out`, then a `--network none` container on the linux/arm64 manifest of the pinned verify-runner v1.0.75 image (the Container-Image line below) with `--rows 1,2,3 --fail-first --base 85fcd48feaf0 --history-pack`, then host `--close-bracket`. `--bracket-digest` reports `verdict=match`. The table and lines below are verifyrun's witness file, copied byte for byte.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewContractFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewContractFlow ' "$o")` | could-not-run exit=1 sandbox=container-netns — image module cache predates go.sum (github.com/mattn/go-isatty v0.0.24 vs v0.0.20): the row stopped on module resolution before its check ran. Re-run in the verify-runner image of a release whose tools/desk go.mod and go.sum match the verified tree, normally the first release that contains the verified commit | sha256:00be333dd478 | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=85fcd48feaf0 unproven · head=2e1ddcbd72c2 |
| 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewBoundary$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewBoundary ' "$o")` | could-not-run exit=1 sandbox=container-netns — image module cache predates go.sum (github.com/mattn/go-isatty v0.0.24 vs v0.0.20): the row stopped on module resolution before its check ran. Re-run in the verify-runner image of a release whose tools/desk go.mod and go.sum match the verified tree, normally the first release that contains the verified commit | sha256:00be333dd478 | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=85fcd48feaf0 unproven · head=2e1ddcbd72c2 |
| 3 | `statusgen --root . --consumers --brief desk-tools/39` | fail exit=2 sandbox=container-netns | sha256:527dc85a5636 | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) | base=85fcd48feaf0 red rc=2 · head=2e1ddcbd72c2 |
Tree-Digest: sha256:1d843a770e690a7bd01d131ad76d41f0ed001b69b9bdc86fdb9bd443b1d0db82
Container-Image: sha256:576cb2e49407447769b79a32af370a23e1c108d62842b8f1772a809d1d3b7617
Verification-Attestation: medici-finance/assay#5777 run=ce0944129e050dfaa7c4f27a71922fa2952cc7b729e2b0bfaf5d280e1e8b71f6 source=2e1ddcbd72c2dbe2674821bd7eac6c16c6189e0f brief=docs/streams/desk-tools/brief-39-extract-shared-review-observation-and-pure-planning-contracts.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=ce0944129e050dfaa7c4f27a71922fa2952cc7b729e2b0bfaf5d280e1e8b71f6 source=2e1ddcbd72c2dbe2674821bd7eac6c16c6189e0f tree=sha256:1d843a770e690a7bd01d131ad76d41f0ed001b69b9bdc86fdb9bd443b1d0db82 witness=sha256:00bf72ba78c780905c3148b6e933e8102f4f62dd6759317f0efbb3e73fe93db2 opened=2026-11-08T04:40:21Z closed=2026-11-08T04:43:59Z before=receipt after=check-verifier

Row accounting: 4 rows, 0 credited, no acceptance obligation observed failing.

- Rows 1 and 2: could-not-run in the container. verifyrun's own reason is that the pinned image's module cache predates this source's tools/desk go.sum (go-isatty v0.0.24 against v0.0.20, among 13 mismatches), so `go test` stopped on module resolution before the named test ran. This is the image gap tracked by #4683. Fail-first at base is `unproven` for the same reason.
- Row 3: verifyrun records `fail exit=2` at head and `red rc=2` at base. Exit 2 is COULD-NOT-CHECK; a DISPROVED claim is exit 1. The row ran the image's statusgen (v1.0.75), not the host v1.0.78. The row names no pinned `--delivery`/`--base`, and the brief has no committed Verification-Table marker naming a delivery, so the delivery could not be derived. The refusal branch taken was not captured: the witness records only the exit code and an output digest, so this Evidence does not say whether the default diff could not be taken or did not carry the brief. The instrument did not reach the consumers claims and no claim was observed disproved. This is the post-merge consumers-row definition class tracked by #4560: the row needs a reviewed amendment to a pinned delivery and first-parent base (this item has three deliveries). The base result is the same exit 2, so it shows no discrimination either. No supplemental probe was run.
- Row 4: not run, excluded by the `--rows 1,2,3` filter. A preflight in the same image, offline and with no row credit, showed `go list -deps -test ./...` in tools/desk exiting 1 on modules absent from the image cache (for example github.com/jackc/pgx/v5 v5.12.0). Row 4's `./...` scope needs that full module set, which is the #4683 image gap.

Fail-first: row 1 unproven, row 2 unproven, row 3 red rc=2 (delivery not derived, not a discrimination result), row 4 not run.

Attempts: one attempt, 2026-11-08T04:43:13Z to 2026-11-08T04:43:26Z, rows 1-3, credited as the table above. No attempt was discarded or superseded.

Risk-bearing values, enumerated over the production files of the delivered reviewflow package (#3570, #3593, #4235) and ranked by irreversibility. The item is `irreversible: no`, and every entry can be undone by an edit and a redeploy. Enumerated: the security pass marker, the lane count and lane names, the observation schema and evaluator ids, the selection evaluator id, the capacity lower bound, the unmapped-policy values, the head-unresolved sentinel, the action names, and `MaxBoardAge = 7 * time.Minute` at tools/desk/internal/reviewflow/idlegate.go:63. That last one is an operational cadence knob moved unchanged from tools/desk/cmd/reviewloop/idlegate.go:63 at base; it ranks last and needs no derivation.
RISK-VALUE: DERIVED — `SecurityPassMarker = "Security-Review: pass"` @ tools/desk/internal/reviewflow/reviews.go:16 — byte-identical to the pre-extraction `securityPassMarker` at tools/desk/cmd/deskboard/board.go:62 on base 85fcd48feaf0; Task 4 requires the extraction to preserve the existing action table and exact-head verdicts, so the literal must move unchanged.
RISK-VALUE: DERIVED — `len(v.Lanes) != 2` @ tools/desk/internal/reviewflow/observation.go:286 — spec/review-observation-v1.md line 99 defines correctness and security as two explicit independent lane conclusions, and observation.go:291 admits only those two names, each once.
RISK-VALUE: DERIVED — `ObservationSchema = "review-observation-v1"` @ tools/desk/internal/reviewflow/observation.go:12 and `ObservationEvaluator = "reviewflow-v1"` @ tools/desk/internal/reviewflow/observation.go:13 — equal to the `const` values at schemas/review-observation-v1.json:260 and :323, the published contract this item specifies.
RISK-VALUE: DERIVED — `SelectionEvaluator = "review-selection-v1"` @ tools/desk/internal/reviewflow/selection.go:9 — the evaluator spec/review-observation-v1.md line 161 requires for selection.
RISK-VALUE: DERIVED — `capacity < 0` refusal @ tools/desk/internal/reviewflow/selection.go:94 — spec/review-observation-v1.md line 180 makes zero capacity valid (it preserves every row and source depth), so 0 is the correct lower bound and only negative values are refused.

**VERIFY: BLOCKED** — rows 1, 2 and 4 not run to a verdict: the pinned verify-runner image's module cache predates the tools/desk go.sum (#4683). Row 3's consumers check exited 2 (COULD-NOT-CHECK: the delivery could not be derived, and the refusal branch taken was not captured) and needs a pinned delivery/base amendment (#4560). No acceptance obligation was observed failing.

Acceptance-Changes: 3=Command
Verification-Table: delivery=f9cba0a88d39085831c86717aab04736c562a774 base=d41670bde8b11276639b860b7ea775f6cda8970b original=d41670bde8b11276639b860b7ea775f6cda8970b
Acceptance-Transition: amendment=65c2f44b31a2d5526652a116fe9afb8831c69c59 approval=https://github.com/medici-finance/assay/issues/6856#issuecomment-6989040118

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewContractFlow$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewContractFlow ' "$o")` | pass exit=0 sandbox=unshare | sha256:bdb10a8662d4 | 2026-11-19 | assay-verifier-app[bot] @ 5649360b6fa405a9239de442136db48e21bc1eee (reads could-not-check: a system call from another ABI) (on-behalf-of human:ian) (forge-identity) | base=85fcd48feaf0 red rc=1 · head=5649360b6fa405a9239de442136db48e21bc1eee · delivering=8c2f9706c9d5 output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000001.log |
| 2 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewBoundary$' ./internal/reviewtest/... > "$o" && grep -F -- '--- PASS: TestReviewBoundary ' "$o")` | pass exit=0 sandbox=unshare | sha256:ff5abec2956d | 2026-11-19 | assay-verifier-app[bot] @ 5649360b6fa405a9239de442136db48e21bc1eee (reads could-not-check: a system call from another ABI) (on-behalf-of human:ian) (forge-identity) | base=85fcd48feaf0 red rc=1 · head=5649360b6fa405a9239de442136db48e21bc1eee · delivering=8c2f9706c9d5 output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000002.log |
| 3 | `git grep -qF PlanCandidates HEAD -- spec/review-observation-v1.md && routes=$(statusgen --root . --consumers --brief desk-tools/39 --delivery 790003516c1fb4ea99d0a6d5eb6e67e9123b3c2a --base 4b7f4847f7f3531185fdfb23fbf978ae9551a242) && h=$(mktemp) && trap 'rm -f "$h"' 0 && git cat-file blob c488296801fcdc542108c955d92e85e63d16067a > "$h" && sh "$h" "$routes"` | pass exit=0 sandbox=unshare | sha256:d7526b183d3a | 2026-11-19 | assay-verifier-app[bot] @ 5649360b6fa405a9239de442136db48e21bc1eee (reads could-not-check: a system call from another ABI) (on-behalf-of human:ian) (forge-identity) | base=85fcd48feaf0 red rc=1 · head=5649360b6fa405a9239de442136db48e21bc1eee · delivering=8c2f9706c9d5 output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000003.log |
| 4 | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewPriorityJoin$' ./... > "$o" && grep -F -- '--- PASS: TestReviewPriorityJoin ' "$o")` | pass exit=0 sandbox=unshare | sha256:326ddc22e707 | 2026-11-19 | assay-verifier-app[bot] @ 5649360b6fa405a9239de442136db48e21bc1eee (reads could-not-check: a system call from another ABI) (on-behalf-of human:ian) (forge-identity) | base=85fcd48feaf0 red rc=1 · head=5649360b6fa405a9239de442136db48e21bc1eee · delivering=8c2f9706c9d5 output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000004.log |
Verification-Attestation: medici-finance/assay#7245 run=c4e8d126bbf0268397a492ffa0ac3b08766fe623677fe392bcff7716548f19c2 source=5649360b6fa405a9239de442136db48e21bc1eee brief=docs/streams/desk-tools/brief-39-extract-shared-review-observation-and-pure-planning-contracts.md model=claude-opus-5-5 tier=strong

### Independent re-verification — 2026-11-19, desk tools v1.0.89, Linux host lane

Verifier: non-implementer, model claude-opus-5-5, tier strong. The witness table directly above is the tool-stamped output of attempt 3, the credited attempt. Host: the OrbStack Linux VM assay-verify-amd64-20261021 (Ubuntu 24.04 amd64, go1.27.4, git 2.56.1, desk tools v1.0.89 from /opt/desk-tools/bin), with KUBECONFIG=/dev/null, every inherited GIT_* variable unset, GIT_CONFIG_GLOBAL=/dev/null, GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_COUNT=0 and GIT_TERMINAL_PROMPT=0. Every row ran in the host lane under the unshare network sandbox (sandbox=unshare in each row), not the container-netns bracket. Source 5649360b6fa405a9239de442136db48e21bc1eee on main, detached and clean apart from this Evidence section; attestation #7245. Each Runner cell carries the tool's own annotation "(reads could-not-check: a system call from another ABI)", which is printed as stamped.

An earlier container-lane pass for this item on 2026-11-19 (attestation #7242) was discarded and never landed. No row result from it is used here.

Admission witness before attempt 3: `deskdispatch --check-verifier --root <home> --brief <this brief>` exit 0, binding run=c4e8d126bbf0 source=5649360b6fa4 model=claude-opus-5-5 tier=strong. The same check also exited 0 before attempt 1.

Invocation (all three attempts): `statusgen verifyrun --root <home> --brief <this brief> --acceptance-record <descriptor> --acceptance-forge --fail-first --delivering-commit 8c2f9706c9d5b26b14ad3a8123878563896e27fd --base 85fcd48feaf08c69a0c878939702b30f2b608546 --base-log <fresh dir>`. The descriptor holds the two marker lines printed in the witness: Verification-Table delivery=f9cba0a88d39 base=original=d41670bde8b1, and Acceptance-Transition amendment=65c2f44b31a2 with the approval medici-finance/assay#6856 issuecomment-6989040118. verifyrun read that approval itself (line "acceptance transition: reading approval …6856#issuecomment-6989040118"), accepted the transition and printed Acceptance-Changes: 3=Command. Row 3 therefore ran as the approved acceptance transition. The base and delivering commit are re-derived: #3570 (8c2f9706c9d5) is the earliest of the three deliveries (#3570, #3593 at 790003516c1f, #4235 at f9cba0a88d39), and its first parent is 85fcd48feaf0.

Environment preparation, disclosed (no credit): the VM had no Go module cache. `go mod download` filled it from git-archive copies of the source and of base 85fcd48feaf0 made outside the home, and an offline `go list -deps -test ./...` (GOPROXY=off) in tools/desk then exited 0 for both trees. Before attempt 2 the build cache was warmed in the home's tools/desk with `go test -p 1 -count=1 -run '^$' ./...` (GOPROXY=off, -mod=readonly). The tree was unchanged by it. Both caches live on the VM's disk and survived the VM restart before attempt 3.

Attempts:

- Attempt 1, 2026-11-19T20:44:32Z to 2026-11-19T21:13:07Z, 4 rows, verifyrun exit=2. Discarded (env-fault). Go compiles were killed under the VM's container memory cap, then 2 GiB (memory.max 2147483648): the base log for row 1 read "gitlab.com/gitlab-org/api/client-go: …/compile: signal: killed". It recorded row 1 fail exit=1 with empty output, rows 2 and 3 pass, and row 4 fail exit=137. Its retained witness copy was under the VM's /tmp and did not survive the VM restart. The result lines here are as observed at the time.
- Between attempts 1 and 2, one verifyrun invocation refused on its flags before any row ran ("statusgen verifyrun: --base-log: mkdirat baselog2: file exists", exit 2). No row ran, so it is not an attempt.
- Attempt 2, 2026-11-19T21:29:55Z to 2026-11-19T21:43:38Z, 4 rows, verifyrun exit=1. Discarded (env-fault). Rows 1, 2 and 3 passed, each red at base. Row 4 recorded fail exit=1 because the compile of the tools/desk/internal/deskkit test build in its `./...` scope was killed under the same 2 GiB cap: the base log named the deskkit and cmd/cellctl compiles as killed, and the container's oom_kill counter went from 31 to 34. A direct re-run of row 4's go test at head, with no credit, showed "--- PASS: TestReviewPriorityJoin" with the killed deskkit build as the only cause of exit 1. Its witness copy is kept outside the home. This section was returned to its committed bytes before attempt 3.
- The driver then restarted the VM with memory.max raised to 8589934592 (8 GiB).
- Attempt 3, 2026-11-19T22:31:29Z to 2026-11-19T22:40:41Z, 4 rows, verifyrun exit=0. Credited; this is the witness above. Memory before: memory.max 8589934592, memory.peak 1192030208, oom_kill 0. After: memory.peak 3487895552, oom_kill 0. No process was killed during the attempt, and every base log is empty.

Row accounting (attempt 3): 4 rows, 4 pass, 0 fail, 0 could-not-run.

- Row 1: pass exit=0 (sha256:bdb10a8662d4). Fail-first: red rc=1 at base 85fcd48feaf0, pass at head.
- Row 2: pass exit=0 (sha256:ff5abec2956d). Fail-first: red rc=1 at base, pass at head.
- Row 3: pass exit=0 (sha256:d7526b183d3a), run as the approved acceptance transition. Fail-first: red rc=1 at base, pass at head.
- Row 4: pass exit=0 (sha256:326ddc22e707). Fail-first: red rc=1 at base, pass at head.

Fail-first: all four rows red at base 85fcd48feaf0 and pass at head, with no compile killed at either side. The reds discriminate: at base, none of TestReviewContractFlow, TestReviewBoundary or TestReviewPriorityJoin exists anywhere under tools/desk, and spec/review-observation-v1.md does not contain PlanCandidates (git grep at 85fcd48feaf0 exits 1).

Risk-bearing values, enumerated over the production files of the delivered reviewflow package (#3570, #3593, #4235) and ranked by irreversibility. The item is `irreversible: no`, and every entry can be undone by an edit and a redeploy. Enumerated: the security pass marker, the lane count and lane names, the observation schema and evaluator ids, the selection evaluator id, the capacity lower bound, the board-age bound, the head-unresolved marker, the action names and the unmapped-policy values. Ranked top five, then derived:
RISK-VALUE: DERIVED — `SecurityPassMarker = "Security-Review: pass"` @ tools/desk/internal/reviewflow/reviews.go:16 — byte-identical to the pre-extraction `securityPassMarker` at tools/desk/cmd/deskboard/board.go:62 on base 85fcd48feaf0; Task 4 requires the extraction to preserve the existing action table and exact-head verdicts, so the literal must move unchanged.
RISK-VALUE: DERIVED — `len(v.Lanes) != 2` @ tools/desk/internal/reviewflow/observation.go:286 — spec/review-observation-v1.md line 99 defines correctness and security as two explicit independent lane conclusions, and observation.go:289-291 admits only those two names, each once.
RISK-VALUE: DERIVED — `ObservationSchema = "review-observation-v1"` @ tools/desk/internal/reviewflow/observation.go:12 and `ObservationEvaluator = "reviewflow-v1"` @ tools/desk/internal/reviewflow/observation.go:13 — equal to the `const` values at schemas/review-observation-v1.json:264 and :327, the published contract this item specifies.
RISK-VALUE: DERIVED — `SelectionEvaluator = "review-selection-v1"` @ tools/desk/internal/reviewflow/selection.go:9 — the evaluator spec/review-observation-v1.md line 161 requires for selection.
RISK-VALUE: DERIVED — `capacity < 0` refusal @ tools/desk/internal/reviewflow/selection.go:94 — spec/review-observation-v1.md line 180 makes zero capacity valid (it preserves every row and source depth), so 0 is the correct lower bound and only negative values are refused.
The board-age bound `MaxBoardAge = 7 * time.Minute` @ tools/desk/internal/reviewflow/idlegate.go:63 moved unchanged from base tools/desk/cmd/reviewloop/idlegate.go:63 and is an operational knob, ranked below the five above.

Post-Evidence witness: `deskdispatch --check-verifier --root <home> --brief <this brief>` exit 0 after this Evidence edit, same binding run=c4e8d126bbf0 source=5649360b6fa4 model=claude-opus-5-5 tier=strong.

The brief is gate: human with decision issue #4521, so this Evidence makes no lifecycle flip; the item awaits the human acceptance.

**VERIFY: PASS** — all 4 rows pass at 5649360b6fa4 in the host lane, each red at base 85fcd48feaf0 and pass at head; row 3 ran as the approved acceptance transition (medici-finance/assay#6856).

## Review

Retain the derived frontmatter gate. Approval of the source spec is recorded; this brief
adds implementation acceptance rather than reopening that decision. Existing adopter
writer/custody/equivalence and collection consent gates remain separate.

## Standing-desk consumer routing — 2026-10-18

graph-execution/27 consumes this brief's planned pure policy/projection API from a
tools/desk-local binding. It does not import cmd/main across modules, duplicate the
classifier, change this brief's acceptance or add a process supervisor here. Preserve
the current oracle, authority boundary and planned library home. This is a forward
consumer, not a dependency on future workflow/controller delivery.

## Implementation pickup — remaining amendment, 23 October 2026

Continue at public main `4a4b8e4f3`. The original reviewflow extraction and command
adapters shipped in PR #3570; PR #3593 added the observation contract on that shared
package. Reuse reviewflow/PlanObservation and the existing command adapters.
The remaining merged amendment is the explicit candidate-plus-priority/capacity join and
TestReviewPriorityJoin, not another classifier or a second queue. The source is the merged
planner amendment in PR #4218, with docs/library-first.md, docs/contracts.md and
spec/review-observation-v1.md at this source revision. Claim inspection found no active
owner or open delivery PR for this brief. The driver explicitly requested this implementation
and draft PR; no approval stamp, new ruling or lifecycle advancement is inferred.

The additive internal Go API is PlanCandidates; collection and command/prose adoption remain
/40, /43 and graph-execution/27, with their existing gates. Legacy board/reviewloop ordering
is preserved and checked by the existing regression suite. The original fixed-here entries
for those adapters describe the inherited extraction, not new edits in this follow-up.
No status cell, historical Evidence or human gate is changed. Existing lifecycle reconciliation
holds for the earlier delivery are disclosed rather than bypassed or repaired by a new stamp.

## Delivery reconciliation — 2026-11-03

Source inspection on main `3342a32ab90440ce7f2f6bf8957b83de626d70dd` confirms three
implementation layers are present: classification/planning extraction and command
adapters in #3570 (`8c2f9706c9d5b26b14ad3a8123878563896e27fd`), the observation
contract in #3593 (`790003516`), and candidate-plus-priority/capacity amendment #4235, merged as
`f9cba0a88d39085831c86717aab04736c562a774`. The latter includes selection implementation
`646808813` and qualified tie-breaking regression `e6dde9128`.

| Original obligation | Delivered source to assess independently |
|---|---|
| Task 1: typed observation and full population | #3593: `spec/review-observation-v1.md`, `schemas/review-observation-v1.json`, `tools/desk/internal/reviewflow/observation.go`: `BuildObservation` |
| Task 2: existing classification/planning extraction and command adapters | #3570 (`8c2f9706c9d5b26b14ad3a8123878563896e27fd`): `tools/desk/internal/reviewflow/classify.go`, `actiontable.go`, `coalesce.go`, `idlegate.go`, `humangate.go`, `reviews.go`, `checks.go`, `input.go`; adapters in `tools/desk/cmd/deskboard/board.go` and `tools/desk/cmd/reviewloop/flow.go`; `changelog/desk-tools-39.md` |
| Tasks 2–3: additive observation/planning APIs and legal boundary | #3593: `tools/desk/internal/reviewflow/observation.go`: `PlanObservation`; #4235: `tools/desk/internal/reviewflow/selection.go`: `PlanCandidates`; collection stays outside this owner |
| Task 4: compatibility, authority and intended behavior | `tools/desk/internal/reviewtest/observation_contract_test.go`, `tools/desk/internal/reviewtest/observation_boundary_test.go`, `tools/desk/internal/reviewtest/selection_test.go`: named contract, boundary and priority-join tests |
| Task 5 and candidate amendment | existing source contracts, command adapters and selection amendment; consumer routing and implementation acceptance still require the original checks and review |

The record-specific human ruling is now bound through `DR-desk-tools-39` and
the frontmatter `design:`/`ruling:` fields:
https://github.com/medici-finance/assay/issues/4521#issuecomment-6615602021

No second extraction or classifier is needed. After this metadata lands, the
native `statusgen reconcile` command can assess implemented against the actual
merged deliveries. No generated board row is hand-edited in this change. Do not
substitute consumer routing for delivery qualification.

All four original Verify rows, Evidence and the existing human gate are unchanged. Named tests were inspected as source, not run as
independent verification. Runtime collection/adoption remains with the declared
follow-on briefs, and final human acceptance stays separate.


## Proposed command amendment for Verify row 3

Version 3 binds the routing check to the three implementation ranges above. The
row reads the selection contract from the subject tree, runs the native historical
consumer report on #3593 and executes the pinned Git blob of
`verify.d/39-consumer-routes.sh`. The helper checks added adapter calls at #3570
and #3593 and the four selection sites added at #4235. It also checks the native
report names both implemented adapters and both deferred routes. #5404 recorded
the delivery mapping. #5780 recorded the blocked independent run.

The three #3570 self-routes establish no implementation. At #3593 the report
counts four corroborated declarations, including two follow-up references. At
#4235 the raw report counts six corroborated and two disproved because that diff
does not touch the inherited adapters. Each adapter is assessed at the delivery
where its code changed. Runtime collection, dispatch preparation and caller
adoption remain with /40, /41, /43 and graph-execution/27. The original Expect
continues to reserve out-of-scope judgments for review.

Applying the amended Command requires the exact post-merge human approval for
row 3 and fresh independent verification under the protected-table procedure.
