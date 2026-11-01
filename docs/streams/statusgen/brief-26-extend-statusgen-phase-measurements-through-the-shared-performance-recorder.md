---
brief: assay:assay:statusgen:26
title: "Extend statusgen phase measurements through the shared performance recorder"
why: "Statusgen already shares the local recorder but aggregate duration cannot explain discovery, parsing, evaluation, rendering or deskread overhead. Reusing the improved contract makes its costs comparable and identifies safe parse reuse without importing credential or desk internals."
wave: 4
depends: ["desktools-v2/63", "desktools-v2/64"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
outcome: none
version: 3
delivery:
  - {in: "assay#3759", covers: partial, note: "sub-split (a); snapshot reuse, workloads and Verify 2 remain owed"}
id: 34374f41-67cd-492a-9bb3-5d3353cf0559
authored: 2026-10-16 by performance brief authoring
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - "assay:assay:desktools-v2:62 — matched desk workload; this brief supplies the statusgen consumer using the same recorder"
  - docs/streams/desktools-v2/README.md — Performance delivery priority, 2026-10-16
  - docs/library-first.md
  - docs/streams/desk-tools/brief-25-token-memo-and-cache-before-install-id.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - freshness-checked 2026-10-16 @ eba941721 (origin/main); installed reference e70f08e4
exec-tier: strong
exec-tier-why: Cross-consumer compatibility, concurrency and unavailable-state behavior require negative-path and flow proofs.
domain: complicated
consumers:
  - "statusgen/perfrecord.go, main.go, load.go, parse.go and forgeread.go: follow-up statusgen/26"
  - "statusgen/go.mod and go.sum: follow-up statusgen/26"
  - "perfrec/: follow-up desktools-v2/61 (one shared recorder implementation)"
  - "tools/desk/cmd/deskperf/: follow-up statusgen/26 (read actual statusgen records)"
  - "tools/desk/cmd/deskread: out-of-scope (existing online credential boundary is retained; child benefits from desktools-v2/58 and /59)"
  - "statusgen/contexthandoff/: out-of-scope (resolver shipped in assay#3839; preserves independent source enumeration; no context index reuse API is added here)"
  - "qualgen/: out-of-scope (documented consumer candidate; no measured hotpath or new wrapper claimed)"
  - "docs/telemetry.md: fixed-here (documents reuse and workload boundaries)"
  - "statusgen/readmemo.go: fixed-here (split existing disk attempts into file and directory counts)"
  - "statusgen/perfsnapshot_test.go: fixed-here (Verify 2 parity and production-owner mutations)"
  - "statusgen/perfworkload_test.go: fixed-here (synthetic fresh-reference workload driver)"
  - "statusgen/bench/: fixed-here (repeatable entrypoint, inventory and measured report)"
  - "statusgen/testdata/perfworkload/: fixed-here (declared synthetic corpus)"
  - "docs/streams/desktools-v2/cli-migration.json: fixed-here (exact owned test route for the workload launcher)"
  - "tools/desk/internal/clicontract/perfworkload_test.go: fixed-here (owner, neighbor, removed-route and shipping controls)"
---

# Brief 26 — Extend statusgen phase measurements through the shared performance recorder

## Context

files:
- `statusgen/perfrecord.go`
- `statusgen/perfrecord_test.go`
- `statusgen/main.go`
- `statusgen/load.go`
- `statusgen/parse.go`
- `statusgen/forgeread.go`
- `statusgen/go.mod`
- `statusgen/go.sum`
- `statusgen/bench/perfworkload.sh` (planned)
- `statusgen/testdata/perfworkload/` (planned)
- `docs/telemetry.md`
- `changelog/statusgen-performance-phases.md` (planned)

facts (source and date in frontmatter):
- At fresh main eba941721, statusgen/perfrecord.go already calls perfrec.New/Append and CPUMillis; every main exit passes exitProcess. Preserve that coverage and its off-by-default local-only contract.
- docs/library-first.md allows only offline/frozen in-process fact readers; statusgen may not import tools/desk, deskkit, authenticated forgeread adapters or a credential provider.
- Existing telemetry.go has separate counts-only network consent. Local phase measurements do not widen that payload, enable an endpoint or add upload.
- Statusgen has distinct lint, regen, gate-scores, next-up and forge-read modes. A new file/consumer outside a previous read set invalidates completeness; HEAD equality alone does not make a dirty working-tree parse immutable.

layering: one canonical library owner; command/config/effect adapters preserve their existing authority.

single-point-of-failure: an incorrectly reused or incompletely observed input could appear current; independent fresh-reference comparisons and direct lower-boundary negative controls below expose it.

design-fit:
  owner: perfrec and existing domain producers
  contract: versioned local performance record
  retires: [incompatible timing boundaries and unclassified measured paths]
  weight: verbs +0; flags +0; no new service, credential holder or online endpoint
  why-add: Reuse the existing semantic owner and retire duplicated work; proposed measurements are local, optional and bounded.

## Read first

Read the existing implementations listed in Context, the source-pinned performance priority
section of the desktools-v2 README, and docs/library-first.md. Refresh current claims and the
source revision before implementation; dated measurements do not certify current deployment.

## Ground rules

Feature branch and draft PR only; no merge, release, workflow dispatch or live infrastructure
contact. Use isolated temporary homes, fake providers and synthetic repositories. Preserve
existing custody, stop, trust, audit, budget and unavailable-state semantics. Optional
measurement cannot change the operation result. Stop at implemented; independent verification
owns Evidence. All new paths and named tests below are implementation deliverables.

## Interface contract

Use perf-v3 from /63 and the shared context contract from /64 directly, with statusgen-owned fixed mode/phase rosters and the same
wall/CPU baseline semantics. Observe source discovery, read/parse, evaluation, rendering and
existing deskread child time. Online work remains behind the existing verb with no credentials
in statusgen. Input reuse is limited to an invocation-local explicit snapshot and versioned
parser/evaluator result; no persistent semantic cache or second parser is introduced here.
If a required source is unavailable, preserve the existing incomplete/held result.

## Task

1. Adopt the perf-v3 shared recorder API and explicitly selected profile at the current exitProcess boundary, preserving all exits, opt-out behavior and legacy mode mapping. Account for package-init versus Go-entrypoint timing explicitly; do not change historical perf-v1 semantics.
2. Add phases through existing owners for discovery/load, parse, eligibility/coverage evaluation and rendering. Observe actual deskread child starts/elapsed time without opening a forge client in statusgen; the child owns provider/credential counters. Do not sum child and parent inclusive time as elapsed savings.
3. Inventory repeated reads/parses within one invocation in lint, regen, gate-scores and next-up. Reuse the same validated source snapshot/results where those owners already support it; keep the parser/evaluation implementations canonical. A changed membership or file revision between declared snapshots must trigger fresh input, never a stale empty result. Report no-op reuse when the reference workload shows none.
4. Add credential-free synthetic small/large stream workloads for all four offline modes plus a fake deskread process. Record cold/warm p50/p95, CPU, file reads/parses and child counts with sample and source revisions. Compare output/exit/coverage byte-for-byte against the fresh reference evaluator; numerical targets require a declared workload.
5. Prove statusgen keeps zero imports of tools/desk/custody, no new network endpoint and no telemetry files when off. Document how deskperf reads statusgen records and how future context resolver 20 consumes explicit immutable snapshots without weakening independent membership enumeration. Keep module resolution working with GOWORK=off.

## Verify

Named tests are planned deliverables. Each command requires its exact PASS line; a missing
selector is failure. Negative controls must exercise the production owner with the stated
break planted, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation +dereference | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestSharedPhaseRecorderFlow$' ./... > "$out" && grep -F -- '--- PASS: TestSharedPhaseRecorderFlow ' "$out")` | exit 0; named PASS; compiled offline lint/regen/gate-scores/next-up and fake deskread retain output/exit, produce correct shared records and deskperf can read them; copied writer fails parity |
| 2 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestInvocationSnapshotReuseParity$' ./... > "$out" && grep -F -- '--- PASS: TestInvocationSnapshotReuseParity ' "$out")` | exit 0; named PASS; same snapshot shares parsing, while new/deleted/untracked consumers and changed contents match fresh reference results; HEAD-only or prior-readset reuse fails |
| 3 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceKeepsCredentialBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestPerformanceKeepsCredentialBoundary ' "$out")` | exit 0; named PASS; dependency closure contains no desk/custody/authenticated adapter imports, off creates no files and fake deskread stays the sole online route; planted direct credential getter fails |
| 4 | check +dereference | `statusgen --root . --consumers --brief statusgen/26` | exit 0; actual implementation diff corroborates declared routing; out-of-scope sufficiency remains review, and no unavailable check is recorded as a pass. |

## Pre-mortem and detection

- Row 1: compiled offline lint/regen/gate-scores/next-up and fake deskread retain output/exit, produce correct shared records and deskperf can read them; copied writer fails parity.
- Row 2: same snapshot shares parsing, while new/deleted/untracked consumers and changed contents match fresh reference results; HEAD-only or prior-readset reuse fails.
- Row 3: dependency closure contains no desk/custody/authenticated adapter imports, off creates no files and fake deskread stays the sole online route; planted direct credential getter fails.

Scope sufficiency and workload representativeness remain review judgments; publish the
measured environment and limitations. No fixture success is a live permission or fleet speed claim.

## Perf-v3 delivery amendment — 19 October 2026

Consume the [perf-v3 contract](../../performance-attribution-spec.md) directly from perfrec; do not import desk internals or authenticated
adapters. Bind the shared --perf-* / explicit env sources through statusgen's real parser;
keep help/version offline, prior opt-in/schema defaults and no-profile behavior. Current
phase/snapshot work stays in scope. A statusgen invocation has fresh identity, receives
work/session/agent/desk and assigned role through the selected workflow profile, and supplies
its invocation as the deskread child's parent. The child reports its own forge attempts,
timing and points; missing child instrumentation is a coverage gap, never parent zero-cost.

Extend TestSharedPhaseRecorderFlow (row 1) with two qualified work references, override/clear,
child causal links and mixed v1/v2/v3 records. Extend TestPerformanceKeepsCredentialBoundary
(row 3) to prove inherited workflow fields cannot appear in tool profile and malformed optional
context cannot change normal output/exit. Old privacy defaults remain binding outside v3.

## Evidence

<!-- Independent verifier records actual subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->
Post-merge verification of statusgen/26 at merged main 7b0fa99b57c3866c6d12d1cb2cebe0247e12ff09 by a non-implementer verifier: runner assay-verifier-app[bot], model claude-opus-5-5, strong tier, admitted by pre-work attestation medici-finance/assay#4925. Deliveries checked: PR #3759 (merge 44829891e8f7) and its follow-up PR #4541 (merge 5a4a3edda5c8). Installed statusgen v1.0.66 throughout; no locally built binary.

Rows 1-3 (check:ci) ran network-off inside the closed admission bracket: host open-bracket, then statusgen verifyrun --sandbox=container-netns in the v1.0.66 verify-runner image (digest on the Container-Image line) started with --network none, read-only source mounts and no forge credential, then host close-bracket. statusgen verifyrun --bracket-digest on the closed witness reported verdict=match. The closed witness bytes follow unedited.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestSharedPhaseRecorderFlow$' ./... > "$out" && grep -F -- '--- PASS: TestSharedPhaseRecorderFlow ' "$out")` | pass exit=0 sandbox=container-netns | sha256:b9cf22a002bb | 2026-10-30 | assay-verifier-app[bot] @ 7b0fa99b57c3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestInvocationSnapshotReuseParity$' ./... > "$out" && grep -F -- '--- PASS: TestInvocationSnapshotReuseParity ' "$out")` | pass exit=0 sandbox=container-netns | sha256:0058b1f42b83 | 2026-10-30 | assay-verifier-app[bot] @ 7b0fa99b57c3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceKeepsCredentialBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestPerformanceKeepsCredentialBoundary ' "$out")` | pass exit=0 sandbox=container-netns | sha256:b93ca06372ef | 2026-10-30 | assay-verifier-app[bot] @ 7b0fa99b57c3 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:8a4ed420c7b398b85b7db0f4180194627b6e1f6af13556ea76863e4d1edf28e8
Container-Image: sha256:29494cbeca300a29295a2eeed7398500675cb63c7911d660a08c899e8584d175
Verification-Attestation: medici-finance/assay#4925 run=455cd3e05088a0908863bb859d8ddc1d5b90d4096b10483e39493602cd3e58dc source=7b0fa99b57c3866c6d12d1cb2cebe0247e12ff09 brief=docs/streams/statusgen/brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=455cd3e05088a0908863bb859d8ddc1d5b90d4096b10483e39493602cd3e58dc source=7b0fa99b57c3866c6d12d1cb2cebe0247e12ff09 tree=sha256:8a4ed420c7b398b85b7db0f4180194627b6e1f6af13556ea76863e4d1edf28e8 witness=sha256:3f90f2fab37387245bb455638c58083e790a6da83d7623f41526f208fc5d5503 opened=2026-10-30T00:40:11Z closed=2026-10-30T00:47:57Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `statusgen --root . --consumers --brief statusgen/26` | fail exit=2 | sha256:a25a511b904d | 2026-10-30 | assay-verifier-app[bot] @ 7b0fa99b57c3 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4925 run=455cd3e05088a0908863bb859d8ddc1d5b90d4096b10483e39493602cd3e58dc source=7b0fa99b57c3866c6d12d1cb2cebe0247e12ff09 brief=docs/streams/statusgen/brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md model=claude-opus-5-5 tier=strong

Row 4 (check) ran on the host through statusgen verifyrun --rows 4 with no forge token in its environment; the witness above is its own output. The selections are disjoint: rows 1-3 in the container bracket, row 4 on the host.

Row 4 observed result: exit 2, which is statusgen's could-not-check exit for --consumers, not an observed routing failure. Reproduced on a clean tree at the same commit with the base at HEAD, the command prints: "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:statusgen:26 is not in the diff against 7b0fa99b57c3866c6d12d1cb2cebe0247e12ff09, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved." The installed lint flags the same row: "Verify row 4 [consumer-delivery-range] consumer command has no explicit --base (the default becomes HEAD on merged main); post-merge consumer checks must name --brief and pin both --delivery and its first-parent --base." The row as authored cannot discharge its obligation after merge. This is a check-definition defect of the class tracked in #4560. Needed: a reviewed amendment of row 4 that keeps --brief and the corroboration obligation and pins --delivery plus its first-parent --base, then a fresh verifier dispatch. Next actor: the brief author.

Supplemental host probes. They earn no acceptance credit, discharge no Verify row, and are recorded only to locate the blocker:

- S1, observed 2026-10-30 by assay-verifier-app[bot] on the host: `statusgen --root . --consumers --brief statusgen/26 --delivery 5a4a3edda5c801b72e4e40bd17d68bfc6f946550 --base 4264d1027198b8f257a95e1630f07cac358fcd6f` exited 0 with summary 12 corroborated, 0 disproved; the 3 remaining entries are the brief's declared out-of-scope claims, left to review.
- S2, observed 2026-10-30 by assay-verifier-app[bot] on the host: `statusgen --root . --consumers --brief statusgen/26 --delivery 44829891e8f74b1520b64f4180910d96e17be96a --base 205c8b46a6b467f4693fc1d5e1152cf2690d9ab5` exited 0 with summary 6 corroborated, 0 disproved; 2 out-of-scope entries left to review.

Risk-bearing values. Brief risk metadata is present and every field is no; the item is not irreversible. Enumerated anyway over the literals the delivery introduces in statusgen/perfrecord.go and statusgen/bench/perfworkload.sh. The highest-ranked is the phase roster, because records written under a wrong roster stay wrong for deskperf readers. It is still reversible by an edit and a release. The timeouts and sample count are reversible operational knobs, ranked last, with no derivation needed.

RISK-VALUE: DERIVED — perfV3Phases = {"discover","parse","evaluate","render","write","deskread"} @ statusgen/perfrecord.go:257 — matches the brief's Interface contract (discovery/load, parse, evaluation, rendering, and the deskread child) and the "Phase roster" in docs/telemetry.md line 341 element for element.
RISK-VALUE: DERIVED — perfToolName = "statusgen" @ statusgen/perfrecord.go:51 — the record's tool field, a constant rather than argv[0] (comment at statusgen/perfrecord.go:50), so a renamed or wrapped binary still emits records under the tool name the brief names.
RISK-VALUE: NAMED, NOT DERIVED — deskread probe timeout = 15*time.Second @ statusgen/perfrecord.go:432, cmd.WaitDelay = time.Second @ statusgen/perfrecord.go:436, bench SAMPLES default 20 @ statusgen/bench/perfworkload.sh:13, go test -timeout=240s @ statusgen/bench/perfworkload.sh:16 — reversible operational knobs; ranked last and out of scope for derivation by design.

Accounting: 4 rows total. 3 passed (rows 1, 2, 3). 0 failed obligations. 1 could-not-check (row 4: exit 2, COULD-NOT-CHECK, check-definition, #4560). No could-not-check result is counted as a pass.

**VERIFY: BLOCKED** — rows 1-3 pass in the admitted container bracket; row 4 exit 2 is could-not-check (consumers row lacks a --delivery/--base pin on merged main; check-definition, #4560). No acceptance obligation was observed to fail.

Re-verification of statusgen/26 at merged main 2a3b4be189484335a8592a80d619011d350a74c7 by a non-implementer verifier: runner assay-verifier-app[bot], model claude-opus-5-5, strong tier, admitted by pre-work attestation medici-finance/assay#5099. Installed statusgen v1.0.67 throughout; no locally built binary.

Rows 1-3 (check:ci) ran network-off inside the closed admission bracket: host open-bracket, then statusgen verifyrun --sandbox=container-netns in the v1.0.67 verify-runner image (release reference checked against checksums.txt; digest on the Container-Image line) started with --network none, a read-only source mount and no forge credential, then host close-bracket. statusgen verifyrun --bracket-digest on the closed witness reported verdict=match. The closed witness bytes follow unedited.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestSharedPhaseRecorderFlow$' ./... > "$out" && grep -F -- '--- PASS: TestSharedPhaseRecorderFlow ' "$out")` | pass exit=0 sandbox=container-netns | sha256:5f1dafdba11d | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestInvocationSnapshotReuseParity$' ./... > "$out" && grep -F -- '--- PASS: TestInvocationSnapshotReuseParity ' "$out")` | pass exit=0 sandbox=container-netns | sha256:e6c07eafcb3f | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerformanceKeepsCredentialBoundary$' ./... > "$out" && grep -F -- '--- PASS: TestPerformanceKeepsCredentialBoundary ' "$out")` | pass exit=0 sandbox=container-netns | sha256:adcd754edba1 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:c09fa1c5ee2cb408a1192399fb9f6dd4c108843f9a2400453ef776287339bae5
Container-Image: sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa
Verification-Attestation: medici-finance/assay#5099 run=f692700c5286303453331846bd8a695651b6c4f3be520e3f226d57e9cb96856f source=2a3b4be189484335a8592a80d619011d350a74c7 brief=docs/streams/statusgen/brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=f692700c5286303453331846bd8a695651b6c4f3be520e3f226d57e9cb96856f source=2a3b4be189484335a8592a80d619011d350a74c7 tree=sha256:c09fa1c5ee2cb408a1192399fb9f6dd4c108843f9a2400453ef776287339bae5 witness=sha256:267ff3b7590d068040c2f3610c86273fac48371f4f82861fdfb1f90b8e497e68 opened=2026-10-31T22:45:38Z closed=2026-10-31T22:51:27Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `statusgen --root . --consumers --brief statusgen/26` | fail exit=2 | sha256:5bd3fafe3496 | 2026-10-31 | assay-verifier-app[bot] @ 2a3b4be18948 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5099 run=f692700c5286303453331846bd8a695651b6c4f3be520e3f226d57e9cb96856f source=2a3b4be189484335a8592a80d619011d350a74c7 brief=docs/streams/statusgen/brief-26-extend-statusgen-phase-measurements-through-the-shared-performance-recorder.md model=claude-opus-5-5 tier=strong

Row 4 (check) ran on the host through statusgen verifyrun --rows 4 with no forge token in its environment, on a clean home at the attested source; the witness above is its own output. The selections are disjoint: rows 1-3 in the container bracket, row 4 on the host.

Row 4 observed result: exit 2, which is statusgen's could-not-check exit for --consumers, not an observed routing failure. The command prints: "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:statusgen:26 is not in the diff against 2a3b4be189484335a8592a80d619011d350a74c7, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved." The installed lint flags the same row: "Verify row 4 [consumer-delivery-range] consumer command has no explicit --base (the default becomes HEAD on merged main); post-merge consumer checks must name --brief and pin both". The row is unchanged since the earlier verification above, so the same check-definition defect stands (class tracked in #4560). Needed: a reviewed amendment of row 4 that keeps --brief and the corroboration obligation and pins --delivery plus its first-parent --base, then a fresh verifier dispatch. Next actor: the brief author.

Discarded attempt, disclosed. An earlier host run of row 4 in this same dispatch reported exit 0, witness sha256:2a66cfa49e5a, and it is discarded with no acceptance credit. It ran with this run's uncommitted Evidence edit in the home, which put the brief itself in the working-tree diff. A plain re-run in that dirty state printed "summary: 0 corroborated, 0 disproved, 15 unchecked", so the zero exit corroborated nothing and did not meet the row's Expect. The home was restored to the attested source and row 4 re-run clean, giving the witness above. The defect this exposes: an Evidence edit alone can turn this row's exit 2 into exit 0 with nothing corroborated, so any witness of this row taken after the Evidence edit reads as a pass. Rows 1-3 ran before that edit and the bracket closed before it.

Supplemental host probes. They earn no acceptance credit, discharge no Verify row, and are recorded only to locate the blocker:

- S1, observed 2026-10-31 by assay-verifier-app[bot] on the host, clean home: `statusgen --root . --consumers --brief statusgen/26 --delivery 5a4a3edda5c801b72e4e40bd17d68bfc6f946550 --base 4264d1027198b8f257a95e1630f07cac358fcd6f` exited 0 with summary 12 corroborated, 0 disproved, 3 unchecked.
- S2, observed 2026-10-31 by assay-verifier-app[bot] on the host, clean home: `statusgen --root . --consumers --brief statusgen/26 --delivery 44829891e8f74b1520b64f4180910d96e17be96a --base 205c8b46a6b467f4693fc1d5e1152cf2690d9ab5` exited 0 with summary 6 corroborated, 0 disproved, 2 unchecked.

Risk-bearing values. Brief risk metadata is present and every field is no; the item is not irreversible. Enumerated anyway over the literals the delivery introduces in statusgen/perfrecord.go and statusgen/bench/perfworkload.sh. Those two files have not changed since the earlier verification above. The highest-ranked is the phase roster, because records written under a wrong roster stay wrong for deskperf readers; it is still reversible by an edit and a release. The timeouts and sample count are reversible operational knobs, ranked last, with no derivation needed.

RISK-VALUE: DERIVED — perfV3Phases = {"discover","parse","evaluate","render","write","deskread"} @ statusgen/perfrecord.go:257 — covers the brief's Interface contract (discovery/load, parse, evaluation, rendering, and the deskread child), plus the write phase, and is the roster docs/telemetry.md documents.
RISK-VALUE: DERIVED — perfToolName = "statusgen" @ statusgen/perfrecord.go:51 — the record's tool field, a constant rather than argv[0], so a renamed or wrapped binary still emits records under the tool name the brief names.
RISK-VALUE: NAMED, NOT DERIVED — deskread probe timeout = 15*time.Second @ statusgen/perfrecord.go:432, cmd.WaitDelay = time.Second @ statusgen/perfrecord.go:436, bench SAMPLES default 20 @ statusgen/bench/perfworkload.sh:13, go test -timeout=240s @ statusgen/bench/perfworkload.sh:16 — reversible operational knobs; ranked last and out of scope for derivation by design.

Accounting: 4 rows total. 3 passed (rows 1, 2, 3). 0 failed obligations. 1 could-not-check (row 4: exit 2, COULD-NOT-CHECK, check-definition, #4560). No could-not-check result is counted as a pass.

**VERIFY: BLOCKED** — rows 1-3 pass in the admitted container bracket at 2a3b4be18948; row 4 exit 2 is could-not-check (consumers row lacks a --delivery/--base pin on merged main; check-definition, #4560). No acceptance obligation was observed to fail.

## Review

Gate: model; all risk answers are no. Reject false-zero, stale-hit, silently changed legacy semantics or unmeasured savings claims.
