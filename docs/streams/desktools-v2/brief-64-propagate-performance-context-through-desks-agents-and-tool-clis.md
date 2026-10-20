---
brief: assay:assay:desktools-v2:64
title: "Propagate performance context through desks, agents and tool CLIs"
why: "Inheriting the desk role cannot identify a reviewer subagent, and reusing session work labels misattributes later tasks. Launchers and shared CLI adapters must pass explicit immutable context so every instrumented child reports its own invocation under the correct agent and work."
wave: 3
depends: ["desktools-v2/63"]
unblocks: ["desktools-v2/17", "desktools-v2/62", "statusgen/26", "performance-collection/02"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Workflow attribution exposes local work linkage; review the implemented field, consent and propagation boundary before acceptance."
decision-trigger: spec
issues: []
schema: brief-v3
version: 2
id: 18b43302-5947-4402-b4f9-c86e6400c616
authored: 2026-10-19 by performance attribution authoring
sources:
  - docs/performance-attribution-spec.md
  - docs/streams/desktools-v2/spec.md
  - docs/telemetry.md
  - docs/library-first.md
  - "freshness-checked 2026-10-19 @ 2e02111ed; perf-v2 implementation 2013b8dc6, no perf-v3 or perfdata module"
exec-tier: strong
exec-tier-why: "Cross-process identity, compatibility and negative-path coverage require design judgment across shared consumers."
domain: complicated
outcome: none
consumers:
  - "tools/desk/cmd/cellctl and deskdispatch: fixed-here"
  - "tools/desk/internal/deskkit/runtool.go: fixed-here"
  - "tools/desk/internal/cli/ and clicontract/: fixed-here"
  - "tools/desk/cmd/deskclaim-ref/, deskclaim/, deskboot/ and desktoken/: fixed-here (current-parser context bindings and custody-preserving inventory; production credential observations remain /62)"
  - "graph-execution/19 and /16; forge-neutral/40: out-of-scope (future claim/token-custody replacements inherit this common binding when implemented; no prerequisite parser migration)"
  - "loopadmin/runner/: fixed-here"
  - "plugins/assay/skills/ and references/desk-shell.md: fixed-here"
  - "tools/desk/cmd/deskboard and desktoken: follow-up desktools-v2/62 (production observations and workload qualification, not the current-parser context binding owned here)"
  - "statusgen/: follow-up statusgen/26"
  - "qualgen/ and other standalone maintained entrypoints: out-of-scope (inventory their integration gaps here; future CLI owners adopt the public perfrec API, with statusgen/26 the qualified standalone producer)"
  - "perfdata/adapters/: follow-up performance-collection/02"
---

# Brief 64 — Propagate performance context through desks, agents and tool CLIs

## Context

files:
- `tools/desk/cmd/cellctl/`
- `tools/desk/cmd/deskdispatch/`
- `tools/desk/internal/deskkit/runtool.go`
- `tools/desk/internal/deskkit/dispatchrecord.go`
- `tools/desk/internal/cli/`
- `tools/desk/internal/clicontract/`
- `tools/desk/cmd/deskclaim-ref/`
- `tools/desk/cmd/deskclaim/`
- `tools/desk/cmd/deskboot/`
- `tools/desk/cmd/desktoken/`
- `docs/streams/desktools-v2/cli-migration.json` (inventory source; parser exemptions stay in force)
- `loopadmin/runner/`
- `plugins/assay/references/desk-shell.md`
- `plugins/assay/skills/worker-desk/SKILL.md`
- `plugins/assay/skills/pr-review-desk/SKILL.md`
- `plugins/assay/skills/verify-desk/SKILL.md`
- `plugins/assay/skills/intake-desk/SKILL.md`
- `plugins/assay/skills/the-desk/SKILL.md`
- `plugins/assay/skills/pr-shepherd/SKILL.md`
- `docs/streams/desktools-v2/cli-contract.md`
- `docs/performance-attribution-spec.md`
- `docs/telemetry.md`
- `tools/desk/README.md`
- `changelog/perf-v3-propagation.md (planned)`

facts (source and date in frontmatter):
- Existing role comes from DESK_LOOP and the transport counters are process-local. Assigned agent work/function is not captured by perf-v2.
- cellctl and deskdispatch already own launch/custody boundaries; loopadmin/runner owns its actual launch protocol. Instrument these owners, not a second launcher or process census.
- The new schema/resolver is /63; current Cobra/Viper adapter has allowlisted configuration and help-first behavior. Standalone modules cannot import desk internals.

layering: perfrec owns typed optional observations; existing CLI, launcher and provider adapters retain effects and authority. No new service or credential boundary.

single-point-of-failure: a producer can mislabel optional context or coverage; independent writer/profile validation and production-flow negative controls expose this without granting operational authority.

design-fit:
  owner: perfrec and existing producer adapters
  contract: docs/performance-attribution-spec.md
  retires: [unattributed new performance samples and implicit unknown-as-zero accounting]
  weight: "verbs +0; profiles +2; shared flags +8 across the track, owned by /64; no new sender, service or credential holder"
  why-add: "Reuse the local recorder and effect seams; three bounded M briefs separate schema, propagation and provider observation."

## Read first

Read docs/performance-attribution-spec.md, the Context sources and the implemented /61.
Refresh current source and active work before pickup; all new paths and named tests below are
deliverables, not existing implementation or acceptance evidence.

## Human decision

Review the implemented identifiable local workflow profile and its demonstrated field and
propagation boundaries. Confirm that opt-out and tool-only use cannot retain work or agent
linkage, and that invalid metadata cannot affect the underlying operation.

Options:
1. **Accept the bounded implementation** — accept the demonstrated local disclosure and isolation; retain separate remote activation controls.
2. **Hold for correction** — identify the field, lifecycle or failure boundary that needs correction.

Default if no answer: none — acceptance waits for concrete implementation evidence.

## Ground rules

Feature branch and draft PR only; no merge, release, workflow dispatch or live infrastructure.
Synthetic providers, isolated homes and disposable local processes only. Preserve custody,
claims, budgets, audit, help/version and operation results. Optional collection cannot become
required accounting. Stop at implemented; independent verification owns Evidence.

## Interface contract

Use exactly the field/flag/env table and lifecycle in docs/performance-attribution-spec.md. Mint random session/agent IDs at their owning launch, preserve on explicit resume, mint fresh IDs for replacement/new subagents, and never inherit the parent agent ID as the child. Each tool creates its own invocation ID and passes it only as the child's parentInvocationId. A standing session changes work per launch; shared overhead is unassigned. Desk role and assigned execution role are distinct. Context is descriptive, not custody or dispatch authority.

## Task

1. Pass through only operator-selected telemetry/profile settings; launchers never synthesize ASSAY_TELEMETRY=1 or a tool/workflow profile. Inventory all maintained desk, standalone and runner entrypoints against their actual launch/CLI owners. Add the shared allowlisted flag/env bindings and credential-free perfrec resolver; no second parser or AutomaticEnv. Flags override env, present empty clears, and a flag work-list replaces the env list. Invalid optional metadata is not echoed and cannot change the underlying exit/output. A perf-desk-role override changes attribution only, never DESK_LOOP's operational authority.
2. Integrate cellctl desk/session launch, deskdispatch agent handoff, supported runner start/resume, shared child process execution and desk skill guidance. Pass fresh child identities, assigned role and chosen work; ordinary tool children retain agent/session but generate fresh invocation IDs. The caller supplies its own invocation as the child's parent. Never overwrite shared global context between concurrent launches. Preserve existing custody, leases, claims and journal/accounting semantics.
3. Record standalone integration gaps and route each to its existing CLI owner; this brief does not port all standalone tools. Implement native-harness-ID to opaque-ID mapping only where an existing harness exposes such identity; retain that private mapping at the launcher boundary, in memory or an existing private state directory owned by the launcher with 0700 directory / 0600 file modes and a maximum seven-UTC-day retention. Never commit, log or publish it. If a mapping needed for resume has expired or is lost, mint fresh opaque IDs and expose a continuity gap; do not infer identity. Unknown boundaries stay unknown and are listed in the inventory. No transcript scraping, recursive monitor or mandatory new service. Standalone adapters may bind the same flags directly to perfrec; statusgen's production phase integration remains /26.
4. Exercise two subjects, brief plus issue, two different assigned roles under one desk, concurrency, retry, explicit resume, replacement, unset/empty/invalid overrides and unknown parent through real parsed binaries and child processes. Prove tool-only profile never emits correlation even when inherited context exists, and telemetry-off creates no measurement files. Document local rollout/opt-out and unavailable coverage. Update generated CLI help through each owner's existing reference workflow and sync generated skill guardrails only if their blocks change.

### Current claim and token-custody adapters

/33 was withdrawn by PR #3409. Its proposed perf-v3 amendment is historical only; this
brief owns the live binding obligation. Explicitly inventory `deskclaim-ref`, `deskclaim`,
`deskboot` and `desktoken` while they remain on their current parsers, as exempted in
cli-migration.json. Bind the common typed resolver through those existing adapters without
porting their parsers, changing custody or reviving /33. Retain help/version, output and
exit behavior; credential observations and workload qualification remain /62 and /65.
The future claim owners graph-execution/19 and /16 and token-custody owner forge-neutral/40
inherit the same binding at replacement pickup; their independent delivery is not a new
prerequisite of this local integration. Record each actual owner in the inventory.

## Verify

Named tests below are planned. Each command requires its exact PASS line; a missing test
cannot pass. Plant the stated defect through the production boundary, not a duplicate checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3LauncherChildFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3LauncherChildFlow ' "$out")` | exit 0; named PASS. Actual launch/parsed binary/child/writer/report flow distinguishes two subjects and child IDs/roles; planted inherited child invocation/agent ID, stale work and cross-task global context fail. |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3CLIContextPrecedence$' ./internal/clicontract/ > "$out" && grep -F -- '--- PASS: TestPerfV3CLIContextPrecedence ' "$out")` | exit 0; named PASS. All declared flags/envs reach the typed resolver; explicit-empty and work-list replacement, invalid metadata, no-profile legacy and tool-only filtering and launcher refusal to create opt-in preserve operation exit/output and offline help. Exercise the actual current-parser claim/token-custody entrypoints named in the inventory; changing claim identity, token custody or their parser exemptions is a failing control. |
| 3 | check:ci +flow +mutation | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RunnerIdentityLifecycle$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3RunnerIdentityLifecycle ' "$out")` | exit 0; named PASS. Actual runner adapter carries fresh/resumed/replaced identities without changing authority/accounting; unknown native ID remains a gap and disabled sink cannot block launch. |
| 4 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/64` | exit 0; changed consumer routing corroborates the actual diff; unavailable is not pass. |

## Pre-mortem and detection

Rows 1–3 plant the counterexamples named in Expect: identity/consent or request-count bugs,
false-zero/compatibility bugs, and changed operational behavior. Row 4 checks shared consumer
routing. Sufficiency of real-world coverage and local disclosure remains reviewer judgment;
fixture success never asserts live adoption or permission.

## Query/cache measurement amendment — 19 October 2026

For service calls, retain immutable per-request context and propagate only bounded opaque
links through a service-owner-admitted protocol/transport option with workflow consent at
both ends. A performance profile alone cannot add headers/context or discover an endpoint.
Do not forward raw work/session/agent locators, credentials or capability material as diagnostic
metadata. Missing/disabled propagation leaves local observations with unavailable linkage.
Extend TestPerfV3LauncherChildFlow with consent off and protocol option off separately,
concurrent requests for two work items, and unknown service context. The service client/handler
protocol and its authorization remain forge-neutral/39–41; this brief only reuses an admitted
interface at existing launch/child seams.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit and named result. -->

## Review

Gate: human. Reject changed legacy meanings, guessed identity/cost, lost coverage or optional measurement that changes an operation.
