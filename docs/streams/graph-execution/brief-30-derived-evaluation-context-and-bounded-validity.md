---
brief: assay:assay:graph-execution:30
title: Derived evaluation context and bounded validity
why: >-
  A cached eligible result can outlive a lease or permission even when no source file
  changes. Runtime consumers need the complete inputs and validity bound of an
  evaluation so display reuse cannot become stale admission.
wave: 3
depends: ["graph-execution/28"]
unblocks: ["graph-execution/14", "graph-execution/19"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: e9baecf8-fffd-47a9-9fe2-6a28c4f3f2dd
authored: 2026-10-14 by worker-desk (author-brief)
sources:
  - docs/proposals/non-ci-architecture/23-runtime-contract-alignment.md
  - docs/streams/graph-execution/work-input-amendment.md — WI-2 applicability
  - statusgen/evaluation/coverage_api.go — DependencyManifest and explicit CoverageInput
  - statusgen/evaluation/identity.go — canonical evaluator Identity
  - freshness-checked 2026-10-14 @ 9a8df91dc; no statusgen/domain or runtime context contract on main
exec-tier: strong
exec-tier-why: Time, dependency coverage and revision identity must agree across pure evaluation, storage and admission.
domain: complicated
consumers:
  - "spec/evaluation-context-v1.md, schemas/evaluation-context-v1.json, statusgen/domain, statusgen/testdata/domain/context: follow-up graph-execution/30"
  - "tools/desk/internal/deskkit/graphadmission.go: follow-up graph-execution/14"
  - "workflow/store and workflow/bindings: follow-up graph-execution/19"
  - "workflow/controller: follow-up graph-execution/21 (through admission /14)"
  - "forgeread source/index caches: out-of-scope (forge-neutral/42–43 own those caches; this contract grants no source access)"
---

# Brief 30 — Derived evaluation context and bounded validity

## Context

files: `spec/evaluation-context-v1.md` (planned), `schemas/evaluation-context-v1.json` (planned), `statusgen/domain/context.go` (planned), `statusgen/domain/context_test.go` (planned), `statusgen/testdata/domain/context/` (planned), `docs/contracts.md`,
`docs/lifecycle.md`, `changelog/graph-execution-30-evaluation-context.md` (planned).

facts (9a8df91dc, 2026-10-14):
- /28 creates the domain records; no domain package exists on this main baseline.
- Existing `evaluation.EvaluateCoverage(in CoverageInput, repo Repo)` takes a `CoverageInput` that carries the revision and dependency manifest;
  WI-2 source/policy/build/environment reuse remains its owner. Do not fork that algorithm.
- `evaluation.EvaluateEligibility` consumes explicit graph facts. It has no live clock,
  credentials or runtime lease authority. The caller must establish current runtime facts.
- /14 rechecks authoritative admission; /19 persists bindings. Those consumers are amended
  with dependency/Verify rows in this authoring change, not implemented here.

layering: extend /28's pure domain package with data and applicability validation. Time is
supplied explicitly; filesystem, clock reads, network, SQL and authorization remain adapters.
There is no new cache service, evaluator or scheduler.

single-point-of-failure: the context validator could omit an input; the admission boundary
independently obtains current lease/access observations and refuses an expired or unavailable
one. A cache hit cannot supply that authority. Row 3 walks both checks with synthetic facts.

design-fit:
  owner: statusgen/domain records; existing eligibility and coverage owners keep their algorithms
  contract: S-verification-domain (planned by /28), extended by evaluation-context-v1
  retires: [implicit assumption that unchanged hashes mean a still-current runtime decision]
  weight: "verbs 0, flags 0, refusals 0; additive pure record validation"
  why-add: n/a; extend the existing domain owner rather than introduce a cache subsystem

## Read first

- [Runtime alignment](../../proposals/non-ci-architecture/23-runtime-contract-alignment.md).
- /28 domain contract and `statusgen/evaluation/coverage_api.go`, `identity.go`.
- /14 admission and /19 store, including their dated context amendments.

## Ground rules

- Isolated branch and draft PR; no deployment, provider contact, merge or credential changes.
- Stop at implemented; independent verification owns later states.
- Synthetic fixtures only. Keep source/coverage algorithms and current human gates.
- No bearer token, credential or raw identity secret in fingerprints or diagnostics.

## Task

1. Define a versioned context using /28 identities: qualified cell/repository/work,
   candidate/acceptance digest, exact source/fact frontiers, evaluator identity,
   query/options, dependency-set digest and coverage, policy/environment versions,
   access-policy epoch, reevaluation generation, observed cutoff and applicable expiry
   bounds. Distinguish immutable derivation identity from a current-use validity check.
   Unknown or incomplete inputs cannot claim a reusable complete context.
2. Add canonical fingerprinting and pure applicability validation to `statusgen/domain`.
   Accept explicit trusted observation time/current context; do not read the clock or
   resolve access yourself. Reject use at/after the earliest required expiry, stale
   epochs/generation, changed membership, missing inputs or mismatched subject/version.
   Caller authorization is a separate mandatory check, never implied by a fingerprint.
3. Supply fixed expected results and schema fixtures. A graph member's removal, a formerly
   absent dependency becoming present, time passing without a new commit, and a policy or
   evaluator update invalidate affected reuse. Incomplete dependency coverage requires
   conservative reevaluation; the implementation must not invent selective applicability.
4. Drive the real /28 Assess and existing evaluator through one fixture flow. Reusing
   computation never retargets an old exact-revision PASS to a new candidate. Compare
   reuse with fresh evaluation, including an authorized immutable hit and an expired
   runtime decision over the same immutable data. No production gate cache is implemented.
5. Update the semantic-owner row, lifecycle explanation, schema and changelog. Document
   /14 and /19 consumption and offline cutoff semantics. Keep environment identity explicit;
   a recorded model/tool digest proves attribution, not reproducibility of nondeterminism.

## Interface contract

Normalized explicit observations -> immutable context/fingerprint plus typed applicability
result -> existing Assess/evaluation. The result is advisory until current admission checks
succeed. Future adapters refuse unsupported mandatory fields and retain invalidation reasons.

## Verify

Named tests and files below are implementation deliverables; no PASS is asserted at authoring.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextInputs$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextInputs " "$out")` | exit 0; named PASS; each declared input changes the fingerprint or applicability; missing membership/coverage is held; mutation: omit evaluator identity |
| 2 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextExpiry$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextExpiry " "$out")` | exit 0; named PASS; unchanged source hashes at and after lease/access expiry are unusable; unavailable time is held; mutation: skip expiry comparison |
| 3 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextAdmissionFlow$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextAdmissionFlow " "$out")` | exit 0; named PASS; real Assess/evaluator flow preserves candidate binding; a synthetic current-authority check refuses an expired grant even with the upper context validator bypassed; mutation: treat a hit as authority |
| 4 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextMembership$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextMembership " "$out")` | exit 0; named PASS; removed and newly discovered dependencies force reevaluation; incomplete coverage never selectively releases; mutation: compare only surviving dependencies |
| 5 | check:ci +dereference +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextSchema$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextSchema " "$out")` | exit 0; named PASS; positive/negative fixtures agree with schema and spec field inventory; no secret material is serialized; mutation: remove a mandatory schema field |

## Definition of Done

All Verify rows discriminate against their mutations; /28 records and the existing evaluator
are reused. Declared docs/schema and changelog match code. No managed runtime is activated.
The contract's adequacy and choice of expiry sources remain independent review judgments.

## Pre-mortem

Missing input (row 1), unchanged hashes hiding expiry (row 2), cache grants authority (row 3),
incomplete membership (row 4), and a schema that omits fields (row 5) have named failing controls.

## Evidence

<!-- Independent verifier records each row, exact subject/environment, command, exit and observed result. -->
### Verification 2026-10-29 — merged main 5729430698d44ffd874bb25275d056f5a5404dc6 (independent verifier)

Subject: medici-finance/assay main at 5729430698d44ffd874bb25275d056f5a5404dc6 (delivered by PR #4785, merge 214bd419e). Environment: all five rows executed by statusgen verifyrun v1.0.65 inside a closed admission bracket, in a `--network none` container from image ghcr.io/medici-finance/verify-runner v1.0.65 (linux/amd64, go1.27.3), with the home mounted read-only; the darwin host cannot provide the unshare sandbox. Witness table below as written by the tool, bracket line included.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextInputs$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextInputs " "$out")` | pass exit=0 sandbox=container-netns | sha256:56de9c5d614b | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextExpiry$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextExpiry " "$out")` | pass exit=0 sandbox=container-netns | sha256:59d39acae129 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextAdmissionFlow$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextAdmissionFlow " "$out")` | pass exit=0 sandbox=container-netns | sha256:07b7a5ba64d6 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextMembership$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextMembership " "$out")` | pass exit=0 sandbox=container-netns | sha256:18efc50ed45b | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextSchema$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextSchema " "$out")` | pass exit=0 sandbox=container-netns | sha256:fad0048954dd | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:14caf0ea56811f25c67d30cc8f736f33e114ece3a6504c05fc4a241f9def8d9f
Container-Image: sha256:1af55e60c9222ad09dab8376b629725135c54912bb3583c13591833db90f2d8f
Verification-Attestation: medici-finance/assay#4820 run=e710fefb1bda78d9050783666668962e41cc19c0c7b9625f7b4248129ad7ed06 source=5729430698d44ffd874bb25275d056f5a5404dc6 brief=docs/streams/graph-execution/brief-30-derived-evaluation-context-and-bounded-validity.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=e710fefb1bda78d9050783666668962e41cc19c0c7b9625f7b4248129ad7ed06 source=5729430698d44ffd874bb25275d056f5a5404dc6 tree=sha256:14caf0ea56811f25c67d30cc8f736f33e114ece3a6504c05fc4a241f9def8d9f witness=sha256:09a8bcfba9e73f5450e1d17747d1e7be01d64269e3155e31efdb999d40af3d87 opened=2026-10-29T17:14:16Z closed=2026-10-29T17:16:03Z before=receipt after=check-verifier

Supplemental host observations (no acceptance credit; diagnostic only, outside the bracket):

| Probe | Command | Observed | Date | Runner |
|---|---|---|---|---|
| S1 mutation controls | `python3 -B -I statusgen/domain/mutate.py` (host go1.27.2 darwin/arm64, scratch copies outside the home) | exit 0; PASS unmutated snapshot; all 11 context mutations KILLED, each failing its named test: context-evaluator and context-currency-check and context-requires-set fail TestContextInputs (row 1); context-expiry, context-time-guard, context-bound-order, context-cached-cutoff and context-currency-key fail TestContextExpiry (row 2); context-authority fails TestContextAdmissionFlow (row 3); context-membership fails TestContextMembership (row 4); context-schema fails TestContextSchema (row 5); the 20 earlier /28 and /31 mutations also KILLED | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (model verifier) (on-behalf-of human:ian) |
| S2 declared docs | read of the merged tree | spec/evaluation-context-v1.md and schemas/evaluation-context-v1.json exist; docs/contracts.md S-verification-domain row and docs/lifecycle.md line 73 cite evaluation-context-v1; the changelog fragment was folded into CHANGELOG.md line 31 by the v1.0.65 aggregation (commit a7438bf77) | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (model verifier) (on-behalf-of human:ian) |

Note on row 3: its named mutation (treat a hit as authority) is planted in the test-local synthetic currentAuthority helper in statusgen/domain/context_test.go, which is what the brief asks for (a synthetic current-authority check); no production admission authority exists in this package.

Risk-bearing values (risk metadata present, irreversible: no; enumeration over the context.go diff):

- RISK-VALUE: DERIVED — expiry comparison `!now.Before(expiry)` (use at the expiry instant = reevaluate) @ statusgen/domain/context.go:246 — Task 2 and spec/evaluation-context-v1.md lines 60-64 require rejection "at/after the earliest required expiry"; an inclusive boundary is the conservative side, and the loop runs over both the cached and the current receipt, so a renewed grant cannot extend the old bound.
- RISK-VALUE: DERIVED — mandatory bound kinds `seen["lease"] && seen["access"]` @ statusgen/domain/context.go:170 (allowed kinds lease, access, evaluation @ :158) — the brief's why names lease and permission expiry as the stale-admission hazard; spec line 38-40 says lease and access are mandatory and no omitted authority implies infinite expiry.
- RISK-VALUE: DERIVED — bound strictly after cutoff `!expiry.After(cutoff)` refuses @ statusgen/domain/context.go:165 and reuse only at/after cutoff `now.Before(cutoff)` holds @ :241 — spec lines 39 and 60 state both; a bound at or before its own observation cutoff could never have been valid.
- RISK-VALUE: DERIVED — currency zero-is-missing `c.AccessEpoch == 0 || c.Generation == 0` @ statusgen/domain/context.go:149 — unknown inputs cannot claim a complete context (Task 1); zero is the Go default for an absent field.
- RISK-VALUE: DERIVED — capability pin `len(c.Requires) != 1` and `"context@1"` @ statusgen/domain/context.go:105,109, schema constant `ContextSchema = "evaluation-context-v1"` @ :14 — versioned contract; future adapters refuse unsupported mandatory fields (Interface contract).
- Reversible format checks ranked last, no derivation needed: evaluator policy digest length 64 @ statusgen/domain/context.go:131, `sha256:` prefix @ :79.

**VERIFY: PASS** — 5/5 rows pass in the container-netns admission bracket (bracket-digest verdict=match); each row's named mutation is killed (S1, supplemental). gate: model, risk all no, irreversible no.

### Verification 2026-10-30 — merged main 011f14965ef2cf173ee6be4ec3d4f319bb55ccae (independent verifier)

Subject: medici-finance/assay main at 011f14965ef2cf173ee6be4ec3d4f319bb55ccae. Environment: all five rows executed by statusgen verifyrun v1.0.67 inside a closed admission bracket, in a `--network none` container from image ghcr.io/medici-finance/verify-runner v1.0.67 (linux/amd64, go1.27.3), with the home mounted read-only; the darwin host cannot provide the unshare sandbox. Witness table below as written by the tool, bracket line included. No discarded or uncredited attempt in this run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextInputs$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextInputs " "$out")` | pass exit=0 sandbox=container-netns | sha256:56de9c5d614b | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextExpiry$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextExpiry " "$out")` | pass exit=0 sandbox=container-netns | sha256:59d39acae129 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextAdmissionFlow$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextAdmissionFlow " "$out")` | pass exit=0 sandbox=container-netns | sha256:49071e6d9634 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextMembership$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextMembership " "$out")` | pass exit=0 sandbox=container-netns | sha256:18efc50ed45b | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextSchema$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextSchema " "$out")` | pass exit=0 sandbox=container-netns | sha256:fad0048954dd | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:8efeda67ba2e48ea5955b3b46beac792fba116f581ab6029ab97c0c654ca294d
Container-Image: sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa
Verification-Attestation: medici-finance/assay#5066 run=a12a7ce7f5bf3dc21225835f4f7e3710e4759ca7a39e6ac8502b3d601ea32655 source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae brief=docs/streams/graph-execution/brief-30-derived-evaluation-context-and-bounded-validity.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=a12a7ce7f5bf3dc21225835f4f7e3710e4759ca7a39e6ac8502b3d601ea32655 source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae tree=sha256:8efeda67ba2e48ea5955b3b46beac792fba116f581ab6029ab97c0c654ca294d witness=sha256:b4a5034f88f8972c275b1937a8298d02b9d58b506e270225447941037dca424e opened=2026-10-30T23:27:49Z closed=2026-10-30T23:28:43Z before=receipt after=check-verifier

Supplemental host observations (no acceptance credit; diagnostic only, outside the bracket):

| Probe | Command | Observed | Date | Runner |
|---|---|---|---|---|
| S1 mutation controls | `python3 -B -I statusgen/domain/mutate.py` (host go1.27.2 darwin/arm64, throwaway HOME and TMPDIR outside the home; the script copies its inputs to scratch) | exit 0; PASS unmutated snapshot; all 11 context mutations KILLED, each failing its named test: context-evaluator and context-currency-check and context-requires-set fail TestContextInputs (row 1); context-expiry, context-time-guard, context-bound-order, context-cached-cutoff and context-currency-key fail TestContextExpiry (row 2); context-authority fails TestContextAdmissionFlow (row 3); context-membership fails TestContextMembership (row 4); context-schema fails TestContextSchema (row 5); the 20 other /28 and /31 mutations also KILLED (31 of 31) | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (model verifier) (on-behalf-of human:ian) |
| S2 declared docs | read of the merged tree | spec/evaluation-context-v1.md, schemas/evaluation-context-v1.json and the context fixture directory exist; docs/contracts.md line 168 and docs/lifecycle.md line 75 carry the contract, lifecycle lines 82 onward and spec lines 75-77 state /14 and /19 consumption; the planned changelog fragment is absent as a file because the release aggregation folded it into CHANGELOG.md line 107 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (model verifier) (on-behalf-of human:ian) |

Note on row 3: its named mutation (treat a hit as authority) is planted in the test-local synthetic currentAuthority helper in statusgen/domain/context_test.go, which is what the brief asks for; the test also drives the real Assess and refuses a moved candidate. This package holds no production admission authority.

Risk-bearing values (risk metadata present, irreversible: no; enumerated over statusgen/domain/context.go and the Deliverables, ranked by what a wrong value would release):

- RISK-VALUE: DERIVED — expiry boundary `!now.Before(expiry)` (use at the expiry instant = reevaluate) @ statusgen/domain/context.go:246 — Task 2 and spec/evaluation-context-v1.md lines 60-64 require rejection at or after the earliest required expiry; the inclusive boundary is the conservative side, and it runs over both receipts, so a renewed grant cannot extend the cached bound.
- RISK-VALUE: DERIVED — mandatory bound kinds `"lease"`, `"access"` @ statusgen/domain/context.go:170 (allowed kinds lease, access, evaluation @ :158) — the brief's why names lease and permission expiry as the stale-admission hazard; spec lines 38-40 make both mandatory and refuse an implied infinite expiry.
- RISK-VALUE: DERIVED — cutoff ordering `!expiry.After(cutoff)` refuses @ statusgen/domain/context.go:165 and `now.Before(cutoff)` holds @ :241 — spec lines 39-40 and 60 state both; a bound at or before its own cutoff was never valid, and use earlier than the observation has no valid window.
- RISK-VALUE: DERIVED — absent currency `c.AccessEpoch == 0 || c.Generation == 0` @ statusgen/domain/context.go:149 — Task 1 forbids unknown inputs claiming a complete context; 0 is the Go value of an absent field, so it cannot be a real epoch or generation.
- RISK-VALUE: DERIVED — capability pin `len(c.Requires) != 1` with `"context@1"` @ statusgen/domain/context.go:105,109 and `ContextSchema = "evaluation-context-v1"` @ :14 — versioned contract; the Interface contract says future adapters refuse unsupported mandatory fields, and one known capability is the whole set this version defines.
- Format checks ranked last, reversible by edit and redeploy, no derivation needed: evaluator policy digest length 64 @ statusgen/domain/context.go:131, `sha256:` prefix @ :79. The test-only timestamps and the mutation script's 30s and 60s timeouts are not production values.

**VERIFY: PASS** — 5/5 rows pass in the container-netns admission bracket (bracket-digest verdict=match); each row's named mutation is killed (S1, supplemental). gate: model, risk all no, irreversible no.

## Review

Gate: model; no live authority or data is changed. Confirm no second evaluator/cache owner.
