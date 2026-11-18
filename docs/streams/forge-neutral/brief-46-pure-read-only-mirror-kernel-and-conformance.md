---
brief: assay:assay:forge-neutral:46
title: Pure read-only mirror kernel and conformance
why: "Repeated readers need one reusable answer whose scope, age and completeness cannot be mistaken for a fresh empty queue. A pure kernel establishes these semantics before the durable store exists, without choosing private-data retention or adding another poller."
wave: 2
depends: ["forge-neutral/36"]
unblocks: ["forge-neutral/37", "forge-neutral/50", "forge-neutral/51", "forge-neutral/52"]
split-from: forge-neutral/37
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The kernel's admission and completeness rules will govern private forge observations. The human accepts the demonstrated pure boundary and its limitations; synthetic fixtures do not choose retention, qualify a durable store or authorize exposure."
decision-trigger: spec
decision-issue: 5365
ruling: "https://github.com/medici-finance/assay/issues/5365#issuecomment-6923690851"
issues: []
schema: brief-v2
version: 10
id: cf6f3530-04b7-4c11-b611-e4c02c24a0f4
outcome: none
design: DR-forge-library
authored: 2026-11-03 by statusgen newbrief
sources:
  - docs/consumer-delivery-checks.md
  - docs/streams/forge-neutral/brief-37-scoped-forge-mirror-with-persisted-freshness-coverage-and-polling-repair.md
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md
  - docs/proposals/non-ci-architecture/22-forge-service-and-source-queries.md
  - docs/streams/decisions/DR-forge-library.md
  - docs/go-engineering.md
  - "freshness-checked 2026-11-02 @ public main 7c62923293d0b640ec90afbf318cae2497908f28; read-only mirror acceleration"
exec-tier: strong
exec-tier-why: "Admission mismatches, partial scans and stale watermarks can silently become complete-empty facts; independent lower-layer controls and exact source identity are required."
domain: complicated
consumers:
  - "forgeread/mirror/: follow-up forge-neutral/46"
  - "forgeread/mirror/mirrortest/: follow-up forge-neutral/46"
  - "forgeread/README.md: follow-up forge-neutral/46"
  - "docs/contracts.md: follow-up forge-neutral/46"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up forge-neutral/46 (exact mutation harness test classification only)"
  - "changelog/forge-mirror-kernel.md: follow-up forge-neutral/46"
  - "workflow/store/, trusted mirror ingestion, coalescing and populated reader flow: follow-up forge-neutral/37"
  - "statusgen/: out-of-scope (no import, transport or eligibility change in this contribution)"
---

# Brief 46 — Pure read-only mirror kernel and conformance

## Context

files:
- `forgeread/mirror/`
- `forgeread/README.md`
- `docs/contracts.md`
- `docs/streams/desktools-v2/cli-migration.json`
- `changelog/forge-mirror-kernel.md`

Reuse the existing source-only checker in tools/forge-import-check without copying it.

facts:
- At source `7c62923293d0b640ec90afbf318cae2497908f28`, forgeread/36's module, root offline reader and admitted adapters exist; `forgeread/mirror` and `workflow/store` do not.
- The kernel needs an injected Store and authorizer, not PostgreSQL or graph-domain types. The reference Store is synthetic conformance support, not production persistence or another deployment authority.
- This is a declared contribution split from /37. Every full /37 criterion remains, including real PostgreSQL transactions/restart, authenticated source composition, scoped coalescing, review populations, retention selection, domain mapping and reader cutover.
- The 25 October DR-forge-library selects a future shared credential-owning forgeclient library. This contribution creates no credential-bearing client and does not reinstate the superseded global ban on statusgen's approved library migration.

layering: Stdlib-only mirror semantics take typed observations and explicit time through narrow Store/Admission ports. They perform no HTTP, database, OS/environment, filesystem or process operations. The forgeread root imports no subpackage; no package here imports statusgen or workflow. Rows 1–6 prove the split.

design-fit:
  owner: forgeread/mirror, extracted early from the existing scoped mirror plan
  contract: "scoped mirror identity, coverage and query relation; declare the one semantic owner in docs/contracts.md without taking ownership of credentials, domain eligibility or retention policy"
  retires: []
  weight: "verbs 0; flags 0; production stores 0; one pure package and reusable conformance helper"
  why-add: "The full persisted mirror currently holds these independently useful semantics behind missing durable-store work; this supplies that capability once for the later integration owner."

## Read first and architecture pickup

Read /37's entire current Task/Verify/retention decision, `docs/contracts.md`, `docs/library-first.md`, `docs/go-engineering.md`, DR-forge-library and the existing forgeread code. Apply the public implementation-pickup procedure before effects and on resume. Record source/guideline/contract revisions and continue/adapt/hold in the PR. The observed /36 source capability does not silently upgrade its lifecycle status; dispatcher eligibility remains independently enforced.

## Human decision

At implementation design, prepare the demonstrated query/admission and completeness boundary with positive and bypassed-layer evidence. Accepting this pure contribution grants no private-data retention policy, persistent storage, network listener, credential or operational activation. The existing separate retention choice remains unanswered. Options are acceptance of the demonstrated bounded contribution or hold for a named correction; absence of acceptance permits no activation. The executor files the concrete evidence decision at the spec trigger, before acceptance is claimed.

Default if no answer: none — blocks until answered

This decision gates acceptance and activation. Offline implementation and synthetic evidence preparation may proceed; no private repository data is ingested or served.

## Ground rules

- Own isolated worktree and draft PR; no main push, merge, release, workflow trigger or live infrastructure contact. Use synthetic records and disposable offline fixtures; export `KUBECONFIG=/dev/null`.
- Stop at implemented. Tests are implementation evidence; an independent verifier and the existing human gate own acceptance.
- Preserve every /37 obligation and historical Evidence. Do not select retain-history/purge or invent a production default. Permission loss must expose unavailable coverage and a required retention-decision signal.
- No new service, scheduler, wire DTO layer, credential discovery, production Source/Store, cross-process claim, dependency or statusgen link. Missing context holds affected work rather than creating a second authority.

## Task

1. Implement the single mirror-owned identity and observation model. A repository is provider/host/full path, including nested GitLab namespace; a Scope also binds record kind and opaque composition-derived admission ID. Records distinguish issue/pull-request/merge-request, exact head and provider-specific review/approval completeness. Coverage includes never-read/complete/partial/unavailable reason, attempted/complete/observed-up-to/retained-from stamps and revision. RetainedFrom remains optional and unknown here; /37’s durable Store must derive it from actual retained coverage under the selected policy, never from the oldest visible row. Trusted composition must canonicalize GitHub owner/repository case before constructing a Scope; the pure kernel does not resolve provider aliases. Validate foreign records, invalid identity and duplicate records before applying. Copy mutable input/output collections.
2. Define an Admission capability supplied by trusted composition, with ID and current `Authorize(ctx, RepositoryID)`; never deserialize it from client input. `QueryScope` requires a nonempty matching admission ID and current coverage before Store access. Store.Read/Apply independently enforce that same relation and return typed permission refusal, not filtered complete-empty. Revalidate returned scope. A genuinely admitted empty complete snapshot alone is known-empty; stale/partial/failed data never imply current emptiness. Explicit Now/MaxAge govern freshness.
3. Implement deterministic `PlanRefresh` and `PlanScan` with supplied overlap/full-inventory/backoff policy and time. Initial/full scans and overlapping incremental scans preserve uncertainty. A permission-held baseline forces the next recovery plan to a full inventory, independent of the periodic repair interval. Track the last full inventory independently from the last complete incremental scan so successful incremental reads cannot postpone periodic repair forever. One ScanUpdate binds expected version, upserts, coverage and complete marker. Partial/failing scans retain prior records and LastComplete; absence from incremental/partial observations proves nothing. Only complete full inventory can mark not-in-inventory, distinct from closed. Validate temporal regressions. Synthetic not-modified input cannot promote a partial baseline; conditional transport remains /37.
4. Publish narrow Store/Source contract types and reusable conformance assertions against a synthetic reference Store. The source port is a future composition seam, not an adapter added here. Direct Store calls with the library bypassed reject nonmatching/revoked admission and incomplete watermark advance; a permissive Store double proves the library also refuses independently. This is not a claim about PostgreSQL or durable compare-and-swap.
5. On permission loss produce unavailable:permission plus retention-decision-required. Keep this permission-loss hold across subsequent transport, rate-limited, unknown or truncated/non-complete refreshes; only a successful complete full inventory applied under the current matching admission may clear it, with LastComplete and LastInventory both strictly after the held snapshot’s LastAttempted. An incremental scan, stale/replayed complete update or conditional not-modified result cannot clear the hold. Store adapters independently refuse any other clearing transition, including with the planner bypassed. Do not apply retain/purge actions. /37 still owns the selected retention behavior, real SQL constraints, real restart/rebuild, coalesced ingestion, source mapping, graph completeness mapping and populated readers. Document how the kernel is consumed there rather than reimplemented.
6. Add an external imported-package fixture flow that seeds a synthetic scan through production planning/application and queries it through the production API. Keep forgeread root/wire behavior unchanged and no module require/replace. Record lower-layer negative evidence by mutating actual production admission, watermark and partial-absence logic, restoring it and rerunning the named test; a duplicate test implementation is not an oracle.

## Interface contract

The concrete small API uses `RepositoryID`, `Scope`, `Admission`, `Observation`, `Coverage`, `Snapshot`, `Result`, `Query`, `RefreshPolicy`, `ScanPlan`, `ScanResult`, `ScanUpdate`, `Source` and `Store`, with `PlanRefresh`, `PlanScan` and `QueryScope`. Freeze exported signatures in the implementation PR before another contribution consumes them. /47 owns provider transport DTOs and imports no mirror package; /37 later maps those DTOs into this model. No generic URL/query/exec/SQL/token accessor is provided. Retained history is distinct from freshness and remains policy-neutral.

## Verify

Named tests below are deliverables, not claimed existing results. Exact PASS output is required; absent/skipped tests cannot pass. All fixtures are offline and synthetic.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelAdmission$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelAdmission ' "$o")` | Allowed A reads its canary; missing, invalid, covered-but-different B and revoked A refuse with no rows/counts. Library refuses before zero Store reads, direct Store bypass refuses independently, and wrong returned scope refuses. Removing either production admission check fails its bypass control. |
| 2 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelCoverage$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelCoverage ' "$o")` | Complete-empty differs from never-read/partial/unavailable/stale. Failed/partial page observations keep prior rows and LastComplete; foreign records and temporal regression refuse. Permission loss signals required retention decision without choosing an action; each later non-complete failure keeps rows withheld until a fresh complete full inventory under the current matching admission, with LastComplete and LastInventory both strictly after the held snapshot’s LastAttempted. The recovery plan forces full inventory; incremental and not-modified results cannot clear the unavailable baseline. Removing the planner hold (`permission-hold-planner`), marking partial complete, advancing watermark or deleting unseen records fails the oracle. |
| 3 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelScanPlanning$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelScanPlanning ' "$o")` | Explicit time/policy yields first/full and overlapping incremental plans, periodic repair and bounded backoff. Complete inventory absence means not-in-inventory, never closed; partial/incremental absence does nothing. Not-modified never promotes partial coverage. Remove overlap or infer closure from absence and the test fails. |
| 4 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorStoreConformance$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorStoreConformance ' "$o")` | Shared conformance exercises reference Store with caller checks bypassed: wrong admission and incomplete watermark advance refuse, authorized complete update succeeds, expected-version mismatch preserves old snapshot; direct Store updates cannot clear a permission-loss hold without a fresh complete full inventory under the current matching admission and LastComplete and LastInventory both strictly after the held snapshot’s LastAttempted. Crafted CompleteScan=true with non-complete coverage, incremental recovery and stale/replayed completeness refuse with the planner bypassed. Removing the independent Store hold (`permission-hold-store`) or making the reference Store permissive fails its bypass control. No SQL/restart acceptance is claimed. |
| 5 | check:ci +flow | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelFlow$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelFlow ' "$o" && GOWORK=off go test -count=1 ./...)` | External package consumer applies synthetic observation then queries exact admitted scope through production API; canary and complete-empty controls pass, unknown/stale remain explicit, caller mutation cannot alter stored data. Existing root/OpenIssues suites pass. |
| 6 | check:ci +mutation +dereference | `GOWORK=off GOPROXY=off GOTOOLCHAIN=local go run tools/forge-import-check/check_env_imports.go forgeread/mirror && (cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelBoundaries$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelBoundaries ' "$o")` | Exit 0; source checker prints 0 and the test emits its named PASS; actual source-only checker rejects os/syscall imports including raw/escaped/build-tagged plants. Implement TestMirrorKernelBoundaries in the module suite to reject HTTP/process/database dependencies, new module requirements and root imports of mirror. Missing package/check execution fails. |
| 7 | check +dereference | `statusgen --root . --consumers --brief forge-neutral/46 --delivery a07f6cc85ad145a5a5b634f2f2a046974f44cf44 --base 2a796ec43b59f4fa4bf97c5ebfedac869beffb00` | The literal pins name the actual /46 mainline delivery and its first parent, fetched from main. Exit 0 corroborates the consumer declarations recorded at that immutable snapshot; unchecked exclusions remain reviewer obligations. This proves path routing, not newer Task/Verify semantics, independent acceptance or activation. |

## Evidence

<!-- Independent verifier records exact source/brief, commands, named PASS and production negative controls. No implementation or acceptance evidence is asserted at authoring. -->

### Independent verification 2026-11-18 — rows 1–6, network-off container bracket

Non-implementer verifier run against merged main `3a9ae0dec7fb7a9b5b889e64dab0d2412f908fea`, rows 1–6 (`check:ci`) executed in the release-pinned verify-runner image under `--network none` inside a closed admission bracket. Witness bytes as produced:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelAdmission$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelAdmission ' "$o")` | pass exit=0 sandbox=container-netns | sha256:2a3423f117c0 | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelCoverage$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelCoverage ' "$o")` | pass exit=0 sandbox=container-netns | sha256:f3bc15921a31 | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelScanPlanning$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelScanPlanning ' "$o")` | pass exit=0 sandbox=container-netns | sha256:89372f1cea3c | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorStoreConformance$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorStoreConformance ' "$o")` | pass exit=0 sandbox=container-netns | sha256:11c6540e25d7 | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelFlow$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelFlow ' "$o" && GOWORK=off go test -count=1 ./...)` | pass exit=0 sandbox=container-netns | sha256:0e6db43fdee9 | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `GOWORK=off GOPROXY=off GOTOOLCHAIN=local go run tools/forge-import-check/check_env_imports.go forgeread/mirror && (cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorKernelBoundaries$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorKernelBoundaries ' "$o")` | pass exit=0 sandbox=container-netns | sha256:b4f25c4affd5 | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:f5adc530563f3ac4ef06dbff6db23b96609d310fe9d6e583b49b785858ebf5ce
Container-Image: sha256:c67baa7a3c8ba3d514128f31340489146525a50c29ebb641a68899c650d354a1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6909 run=d08899e82f9361e4f72027b369bcb43db334017d840156f155302d57abe24cdb source=3a9ae0dec7fb7a9b5b889e64dab0d2412f908fea brief=docs/streams/forge-neutral/brief-46-pure-read-only-mirror-kernel-and-conformance.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=d08899e82f9361e4f72027b369bcb43db334017d840156f155302d57abe24cdb source=3a9ae0dec7fb7a9b5b889e64dab0d2412f908fea tree=sha256:f5adc530563f3ac4ef06dbff6db23b96609d310fe9d6e583b49b785858ebf5ce witness=sha256:9adfecfe4f0d73e69c3b8774328a5729f13921a3aee59570a411030c4ac9387d opened=2026-11-18T15:19:42Z closed=2026-11-18T15:20:59Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 7 | `statusgen --root . --consumers --brief forge-neutral/46 --delivery a07f6cc85ad145a5a5b634f2f2a046974f44cf44 --base 2a796ec43b59f4fa4bf97c5ebfedac869beffb00` | pass exit=0 | sha256:0f7eb3915c21 | 2026-11-18 | assay-verifier-app[bot] @ 3a9ae0dec7fb (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6909 run=d08899e82f9361e4f72027b369bcb43db334017d840156f155302d57abe24cdb source=3a9ae0dec7fb7a9b5b889e64dab0d2412f908fea brief=docs/streams/forge-neutral/brief-46-pure-read-only-mirror-kernel-and-conformance.md model=claude-opus-5-5 tier=strong

Row 7 (class `check`, not `check:ci`) ran on the host with installed statusgen v1.0.87, because it reads git history (delivery `a07f6cc85ad1`, the merge of #5368, and its first parent `2a796ec43b59`). Its primary output: 7 CORROBORATED, 0 disproved, 1 UNCHECKED (`statusgen/: out-of-scope`), with the standing PRODUCER-DELIVERY COULD-NOT-CHECK notice. The Expect cell leaves the unchecked exclusion to the reviewer. A host observation (`git diff --stat 2a796ec43b59..a07f6cc85ad1`) shows 15 changed files and no path under statusgen/.

Rows 1–6 primary output is recorded by digest in the bracketed witness above. All six exited 0 inside the container, and each grep for its named `--- PASS:` line succeeded. Row 5 includes the full `GOWORK=off go test -count=1 ./...` over the forgeread module. Merged main has changed forgeread/mirror since the delivery (later forge-neutral contributions). These rows verify the current merged-main behaviour at the attested source.

**Supplemental probe: no acceptance credit.** This was a mutation matrix in disposable copies of the tracked forgeread snapshot, outside the admitted home. Each of the 21 production-source mutations in forgeread/mirror/mutations.json was applied in its own copy. Then the exact `-run` selectors of rows 1–6 were run offline (`GOWORK=off GOPROXY=off GOTOOLCHAIN=local`). No admitted file was edited.

| Probe | Observation | Date | Runner |
|---|---|---|---|
| Row-claimed mutants vs row 1 selector | query-admission and store-admission both make TestMirrorKernelAdmission fail. | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| Row-claimed mutants vs row 2 selector | permission-hold-planner, partial-complete, partial-watermark, partial-absence and record-regression each make TestMirrorKernelCoverage fail. So do hold-recovery-plan-full, hold-recovery-scan-full and hold-recovery-scan-fresh. | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| Row-claimed mutants vs row 3 selector | missing-overlap, absence-closes, inventory-starvation, partial-not-modified, validator-window and partial-absence each make TestMirrorKernelScanPlanning fail. | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| Row-claimed mutants vs row 4 selector | permission-hold-store, store-admission and store-watermark each make TestMirrorStoreConformance fail. So do the three hold-recovery-store mutants (inventory, state-class and full-class). | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| Row 6 with checker present | Unmutated: the checker prints 0 and TestMirrorKernelBoundaries passes. kernel-http-import: the checker still prints 0 because net/http is outside its os/syscall scope, but the test fails with "forbidden kernel import net/http". | 2026-11-18 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Every mutation the Expect cells of rows 1–4 and 6 name is killed by that row's own selector. Every row is discriminating for its named obligations.

**Risk-bearing values** (the `sensitive-data: yes` trigger). The values below were enumerated over the production files the delivery diff introduced: forgeread/mirror types.go, query.go, coverage.go, planner.go and scan.go, plus mirrortest/store.go, all read at the attested source. The diff introduces no numeric default. Overlap, FullInventoryInterval, FailureBackoff and MaxAge are always caller-supplied. The values found are authority bindings and structural bounds.

- RISK-VALUE: DERIVED — library admission `a.ID() == "" || a.ID() != scope.AdmissionID` @ forgeread/mirror/query.go:20. Task 2 requires a nonempty, exactly matching admission ID before Store access. Exact equality on the opaque, composition-derived ID is the only relation that does not widen a scope. Rows 1 and 4 show that the matching mutant is killed.
- RISK-VALUE: DERIVED — Store admission `a == nil || a.ID() == "" || a.ID() != scope.AdmissionID` @ forgeread/mirror/mirrortest/store.go:26. Task 4 requires the same relation, enforced independently of the library. It matches the library check, adds a nil guard, and its mutant is killed (rows 1 and 4).
- RISK-VALUE: DERIVED — planner hold clearing `s.Coverage.Reason == Permission && (!p.FullInventory || !now.After(s.Coverage.LastAttempted))` @ forgeread/mirror/scan.go:60. Task 5 says recovery needs a full inventory "strictly after the held snapshot's LastAttempted". `After` is strict, so an equal timestamp or a replay cannot clear the hold.
- RISK-VALUE: DERIVED — Store hold clearing requires `CompleteScan`, `State == Complete`, `FullInventory`, `LastComplete.After(old.LastAttempted)` and `LastInventory.After(old.LastAttempted)` @ forgeread/mirror/mirrortest/store.go:66. This is the same strict relation, held independently as Task 5 requires.
- RISK-VALUE: DERIVED — GitHub path segments `len(parts) != 2` (GitLab `len(parts) < 2`) @ forgeread/mirror/types.go:308. A GitHub repository is exactly owner/name, and a GitLab path can carry nested namespaces (Task 1). Ranks below the admission values: an edit and redeploy undo it.
- RISK-VALUE: NAMED, NOT DERIVED — freshness `q.Now.Sub(LastComplete) <= q.MaxAge` @ forgeread/mirror/coverage.go:42, `q.MaxAge <= 0` refused @ forgeread/mirror/query.go:78, repair `now.Sub(c.LastInventory) >= p.FullInventoryInterval` @ forgeread/mirror/planner.go:17. These are reversible operational knobs, and their values come from the policy the caller supplies. The equal-boundary choice is a convention that no spec states.

**VERIFY: PASS** — 7/7 rows pass. Rows 1–6 ran in the closed container-netns bracket (bracket-digest verdict=match). Row 7 ran on the host. Status is not advanced: `gate: human` and `sensitive-data: yes` leave acceptance and activation with the human decision on medici-finance/assay#5365. This Evidence chooses no retention policy and grants no activation.

## Review

Gate human, inherited from /37. Review the actual lower-layer refusal, typed unknown/partial states, dependency closure and declared limits. The full persisted mirror, retention choice, service and deployment remain unqualified by this contribution.
