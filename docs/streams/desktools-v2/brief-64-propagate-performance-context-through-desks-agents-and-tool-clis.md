---
brief: assay:assay:desktools-v2:64
title: "Propagate performance context through desks, agents and tool CLIs"
why: "Inheriting the desk role cannot identify a reviewer subagent, and reusing session work labels misattributes later tasks. Launchers and shared CLI adapters must pass explicit immutable context so every instrumented child reports its own invocation under the correct agent and work."
wave: 3
depends: ["desktools-v2/63"]
unblocks: ["desktools-v2/17", "desktools-v2/62", "statusgen/26", "performance-collection/02", "desktools-v2/68"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Workflow attribution exposes local work linkage; review the implemented field, consent and propagation boundary before acceptance."
decision-trigger: spec
decision-issue: 3537
ruling: "https://github.com/medici-finance/assay/issues/3537#issuecomment-6281511664"
design: DR-perf-v3-prop
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
  - "perfrec/lifecycle.go: fixed-here (public immutable child composition; schema and resolver remain /63)"
  - ".github/workflows/ci.yml: fixed-here (execute the admitted runner module's lifecycle and isolation qualification)"
  - "tools/desk/internal/deskkit/citrigger_test.go: fixed-here (register and qualify that cross-module execution boundary)"
  - "tools/desk/cmd/cellctl and deskdispatch: fixed-here"
  - "tools/desk/internal/deskkit/runtool.go: fixed-here"
  - "tools/desk/internal/cli/ and clicontract/: fixed-here"
  - "tools/desk/cmd/deskboard/, deskread/ and deskgitlab/: fixed-here (existing shared-parser callback plumbing; production qualification stays with existing owners)"
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
- `perfrec/lifecycle.go`
- `.github/workflows/ci.yml`
- `tools/desk/internal/deskkit/citrigger_test.go`
- `tools/desk/cmd/cellctl/`
- `tools/desk/cmd/deskdispatch/`
- `tools/desk/internal/deskkit/runtool.go`
- `tools/desk/internal/deskkit/dispatchrecord.go`
- `tools/desk/internal/cli/`
- `tools/desk/cmd/deskboard/main.go`
- `tools/desk/cmd/deskread/main.go`
- `tools/desk/cmd/deskgitlab/main.go`
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
- `changelog/perf-v3-propagation.md` (planned)

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

Default if no answer: none — blocks until answered

Acceptance waits for concrete implementation evidence.

Decision recorded 2026-10-19: the mapped human selected option 1, accepting the bounded local propagation implementation. The design record transcribes that ruling; independent review and verification remain required.
The ruling is the comment on decision issue #3537 that the brief's `ruling:` key links.

## Ground rules

Feature branch and draft PR only; no merge, release, workflow dispatch or live infrastructure.
Synthetic providers, isolated homes and disposable local processes only. Preserve custody,
claims, budgets, audit, help/version and operation results. Optional collection cannot become
required accounting. Stop at implemented; independent verification owns Evidence.

## Interface contract

Use exactly the field/flag/env table and lifecycle in docs/performance-attribution-spec.md. Mint random session/agent IDs at their owning launch, preserve on explicit resume, mint fresh IDs for replacement/new subagents, and never inherit the parent agent ID as the child. Each tool creates its own invocation ID and passes it only as the child's parentInvocationId. A standing session changes work per launch; shared overhead is unassigned. Desk role and assigned execution role are distinct. Context is descriptive, not custody or dispatch authority.

## Task

1. Pass through only operator-selected telemetry/profile settings; launchers never synthesize ASSAY_TELEMETRY=1 or a tool/workflow profile. Inventory all maintained desk, standalone and runner entrypoints against their actual launch/CLI owners. Add the shared allowlisted flag/env bindings and credential-free perfrec resolver; no second parser or AutomaticEnv. Flags override env, present empty clears, and a flag work-list replaces the env list. Invalid optional metadata is not echoed and cannot change the underlying exit/output. A perf-loop-role override changes attribution only, never DESK_LOOP's operational authority.
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

### Independent verification — 2026-10-20 (dispatched verifier, claude-opus-5-5, tier strong)

Subject: merged source f3724a2d07a8ee8e3ac42f4e87300637e4e3153b (detached home, clean before
and after every run). Delivery is the two-parent merge 4b7f4847f7f3 (PR #3582); the Verify
section is byte-identical at the original, base, delivery and source revisions.
`deskpathguard rederive` re-derived 4 original rows, 0 author-added, all from base; every
original row ID, Command, Expect and class below is the original one.
Admission: `deskdispatch --check-verifier` exit 0 before the first row and exit 0 again
after the last, same binding.

Verification-Attestation: medici-finance/assay#3771 run=4c27a0692c0233e6e2079a4cac3c3046200a59e6f670cee0ec68e8751a2de458 source=f3724a2d07a8ee8e3ac42f4e87300637e4e3153b brief=docs/streams/desktools-v2/brief-64-propagate-performance-context-through-desks-agents-and-tool-clis.md model=claude-opus-5-5 tier=strong

Verification-Table: delivery=4b7f4847f7f3531185fdfb23fbf978ae9551a242 base=b73d90d774fca8aec8974ef410da63ebcb10eece original=3d737b4871bf604e698c6c96427bb89c5e794db7

Environment: macOS host, Go toolchain from the home, pinned desk shims v1.0.52, offline,
KUBECONFIG=/dev/null. Each row was also run by hand inside a local docker container
(image assay-verify:go1.27, `--network none`, only loopback present, home mounted read-only),
with the same exit and named result as on the host.

| # | Command | Expected | Observed | Date / runner |
|---|---|---|---|---|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3LauncherChildFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3LauncherChildFlow ' "$out")` | exit 0; named PASS; planted inherited child invocation/agent ID, stale work and cross-task global context fail | exit 0 — `--- PASS: TestPerfV3LauncherChildFlow (1.13s)` (container: exit 0, 3.90s). Fail-first at base b73d90d7: exit 1 (named test absent). Mutation: all 7 named plants red (inherited agent ID, inherited invocation ID, grandparent parent invocation, stale work, cross-task global context 3 of 3 reps, manufactured child opt-in, attribution overriding the desk loop). Discriminating. | 2026-10-20 assay-verifier-app[bot] @ f3724a2d07a8 (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3CLIContextPrecedence$' ./internal/clicontract/ > "$out" && grep -F -- '--- PASS: TestPerfV3CLIContextPrecedence ' "$out")` | exit 0; named PASS; changing claim identity, token custody or their parser exemptions is a failing control | exit 0 — `--- PASS: TestPerfV3CLIContextPrecedence (6.56s)` (container: exit 0, 6.21s). Fail-first at base: exit 1. Mutation: claim identity (claim target repo from attribution work) and token custody (custody role from attribution role) plants are red. **Parser-exemption control does not discriminate:** flipping the registry state exempt to migrated in docs/streams/desktools-v2/cli-migration.json for deskclaim (line 598), deskclaim-ref (619), desktoken (1387) and deskboot (564), one at a time, leaves this exact command at exit 0 with `--- PASS: TestPerfV3CLIContextPrecedence` every time. The same desktoken flip turns TestCLIInventory, TestCLIResearchReplay and TestCLIRoutingMutations red in the same package, which this row's run filter excludes. Row FAILS its mutation obligation. | 2026-10-20 assay-verifier-app[bot] @ f3724a2d07a8 (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |
| 3 | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RunnerIdentityLifecycle$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3RunnerIdentityLifecycle ' "$out")` | exit 0; named PASS; fresh/resumed/replaced identities, unknown native ID stays a gap, disabled sink cannot block launch | exit 0 — `--- PASS: TestPerfV3RunnerIdentityLifecycle (0.00s)` (container: exit 0). Fail-first at base: exit 1. Mutation: 7 of 7 plants red (no explicit resume, retention past seven UTC days, concealed unknown-mapping gap, replacement inheriting the previous agent, disabled sink blocking launch, diagnostics mutating the launch request, resume keeping stale work). Discriminating. | 2026-10-20 assay-verifier-app[bot] @ f3724a2d07a8 (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |
| 4 | `statusgen --root . --consumers --brief desktools-v2/64` | exit 0; changed consumer routing corroborates the actual diff; unavailable is not pass | exit 2 — `statusgen: --consumers: COULD-NOT-CHECK: assay:assay:desktools-v2:64 is not in the diff against f3724a2d…, and the brief's delivery could not be derived: the committed Evidence has no Verification-Table marker naming the delivery. No routing claim was corroborated and none was disproved.` Same in the container. Could-not-check, counted as unpassed. The cause is ordering: the marker in this Evidence run is not on main yet. Supplemental probes with no acceptance credit: `--delivery 4b7f4847… --base 9d4ae27f…` gave exit 0, 13 corroborated, 0 disproved, 2 unchecked (both out-of-scope entries). The exact row command in a local-only scratch clone with this marker committed (never pushed) gave exit 0 with the same 13/0/2 summary. | 2026-10-20 assay-verifier-app[bot] @ f3724a2d07a8 (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |

Totals: 4 required rows. Rows 1 and 3 pass and discriminate. Row 2 has an exit 0 named PASS
but fails its mutation obligation (parser-exemption control). Row 4 is could-not-check
(could-not-run accounting: 1 row). The supplemental row 4 probes give it no acceptance credit.

Container witness (`statusgen verifyrun --dry-run --sandbox container-netns`, same image,
`--network none`, no credentials mounted): could not run, and the cause is the environment.
The output was `pre-work verifier admission failed; no Verify rows executed: exit status 5 …
cannot mint the verifier App installation token … ("desktoken": executable file not found in
$PATH) … ForgeFor never falls back to an ambient gh-CLI identity`, verifyrun exit 2.
verifyrun's in-process admission has to mint a forge credential over the network. The
credential-free, network-less container cannot do that by design. No workaround was tried.
This is not a code result. The hand runs above are the evidence.

Advisory (no verdict weight):
- An R2 plant that removes the telemetry gate inside the resolver survives row 2. The write
  path has its own `Enabled()` gate, which masks it.
- An R1 plant that makes the tool profile forward workflow linkage inside ContextEnv survives
  row 1. The tool-profile resolver filters it later. Row 1 does not name this plant.
- Row 2's "offline help" clause could not be mutated in any way that can be observed: deskclaim-ref
  handles help before the performance binding, and desktoken, deskclaim and deskboot exit the
  same way before and after a help-shadow plant.
- The named test hardcodes its entrypoint list. No Go code reads docs/performance-entrypoints.json
  (123 rows, consistent with the registry, 0 mismatches).
- The 19 October amendment is outside the protected table. TestPerfV3LauncherChildFlow covers
  consent off and absent protocol admission together. It has no separate protocol-off case,
  no concurrent two-work-item case and no unknown-service-context case.
- Shipped mutation specs: the deskdispatch performance mutations are 18 of 18 caught and the
  runner performance mutations 4 of 4 caught. Supplemental only.

Raw outputs that print the role-context flag by name, roster echo lines and the full test logs
are kept in the verifier's local scratch and not copied here.

RISK-VALUE: DERIVED — mapping retention cutoff = today.AddDate(0, 0, -6) @ loopadmin/runner/performance.go:54 — keeps mappings dated today−6 through today, which is exactly seven UTC days and matches the brief's "maximum seven-UTC-day retention" for memory-only native-ID mappings. A plant extending it to −7 is red under row 3.
RISK-VALUE: DERIVED — handoff dir mode = 0700 @ tools/desk/cmd/deskdispatch/performance_handoff.go:28, group/other refusal mask = 0077 @ performance_handoff.go:32, handoff file mode = 0o600 @ tools/desk/internal/packet/packet.go:747 — owner-only directory and file, refused if any group/other bit is set, as the brief's 0700/0600 private handoff requires. Shipped deskdispatch mutations are red. These values are not covered by a Verify-row command.
RISK-VALUE: DERIVED — handoff expiry = today.AddDate(0, 0, 7) as expiresBeforeUtc @ tools/desk/cmd/deskdispatch/performance_handoff.go:49, and handoff prune cutoff = today.AddDate(0, 0, -6) @ performance_handoff.go:56 — valid for today through today+6 and pruned once older than today−6, so both bounds are seven UTC days as the brief states.
Ranked lower, reversible: perfrec NewID = 16 random bytes (32 hex) @ perfrec/context.go:113. The MaxWork = 8 and MaxWorkBytes = 4096 bounds @ perfrec/context.go:76-77 come from desktools-v2/63, not this diff.

**VERIFY: FAIL** — row 2: TestPerfV3CLIContextPrecedence does not fail when any claim or token-custody entrypoint's parser exemption is flipped in the CLI migration registry. The row's Expect names that as a failing control, so this risk-bearing row does not discriminate. Row 4 is separately could-not-check until this marker lands.

### Row 4 at the Evidence landing commit — 2026-10-20 at a570e840520a

Verification-Attestation: medici-finance/assay#3771 run=4c27a0692c0233e6e2079a4cac3c3046200a59e6f670cee0ec68e8751a2de458 source=f3724a2d07a8ee8e3ac42f4e87300637e4e3153b brief=docs/streams/desktools-v2/brief-64-propagate-performance-context-through-desks-agents-and-tool-clis.md model=claude-opus-5-5 tier=strong

Subject revision: a570e840520af3fe6e01f08fdda7cf07b7328b1a. Its single parent is the attested source f3724a2d07a8, and it changes only this brief's Evidence section: the run above, together with its Verification-Table marker (1 file, 65 insertions).

Verify row 4 was re-run exactly as written in a scratch clone at that commit. The clone was never pushed and has a dead push URL. Its local origin/main is pinned to the subject revision, which is how the brief reads once this commit is on main.

The row ran twice:
- by hand on the host, with the pinned statusgen v1.0.52 shim and KUBECONFIG=/dev/null;
- by hand inside a docker container (image assay-verify:go1.27, `--network none`, only loopback present, clone mounted read-only). The container used statusgen built from the attested source, which is unpinned.

Both runs gave the same exit and summary, and the clone's status was clean afterwards. Rows 1 to 3 are carried forward from the first run at f3724a2d07a8 and labelled that way. a570e840520a changes no code, only this brief.

The statusgen verifyrun container witness was not re-run. In the first run it could not complete, for an environment-only reason: its in-process admission must mint a forge credential, and the credential-free `--network none` container cannot do that by design.

Verification-Table: delivery=4b7f4847f7f3531185fdfb23fbf978ae9551a242 base=b73d90d774fca8aec8974ef410da63ebcb10eece original=3d737b4871bf604e698c6c96427bb89c5e794db7

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3LauncherChildFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3LauncherChildFlow ' "$out")` | pass exit=0 (carried forward from the run at f3724a2d07a8; host and container hand runs; all named plants killed) | sha256:4219e8e52f92 | 2026-10-20 | assay-verifier-app[bot] @ f3724a2d07a8 (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3CLIContextPrecedence$' ./internal/clicontract/ > "$out" && grep -F -- '--- PASS: TestPerfV3CLIContextPrecedence ' "$out")` | exit=0 named PASS, mutation obligation failed (carried forward from the run at f3724a2d07a8): a flipped parser exemption for deskclaim, deskclaim-ref, desktoken or deskboot in the CLI migration registry still passes | sha256:8af2549f2fdb | 2026-10-20 | assay-verifier-app[bot] @ f3724a2d07a8 (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |
| 3 | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RunnerIdentityLifecycle$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3RunnerIdentityLifecycle ' "$out")` | pass exit=0 (carried forward from the run at f3724a2d07a8; host and container hand runs; 7 of 7 plants killed) | sha256:f2e42d58df1a | 2026-10-20 | assay-verifier-app[bot] @ f3724a2d07a8 (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |
| 4 | `statusgen --root . --consumers --brief desktools-v2/64` | pass exit=0 (host); exit=0 in the `--network none` container | sha256:e3fb036a0321 | 2026-10-20 | assay-verifier-app[bot] @ a570e840520a (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |

Output line: `summary: 13 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`, over `base 9d4ae27f2d0dcf429b1ac2e82094464b6ba2a7a3..4b7f4847f7f3531185fdfb23fbf978ae9551a242 (immutable delivered snapshot)`. The container run printed the identical summary line.

Corroborated entries:
- fixed-here: perfrec/lifecycle.go; the CI workflow; deskkit citrigger_test.go and runtool.go; cmd/cellctl and deskdispatch; internal/cli/ and clicontract/; cmd/deskboard/, deskread/ and deskgitlab/; cmd/deskclaim-ref/, deskclaim/, deskboot/ and desktoken/; loopadmin/runner/; plugins/assay/skills/ and references/desk-shell.md
- follow-up: desktools-v2/62 (deskboard and desktoken production observations), statusgen/26, performance-collection/02

Unchecked entries. Both are out of scope; neither is an in-scope entry:
- graph-execution/19 and /16, with forge-neutral/40 (future claim/token-custody replacements)
- qualgen/ and the other standalone maintained entrypoints

Their truth is a reviewer judgement that no diff can settle, so this is an exit 0 with only out-of-scope entries unchecked. Row 4 passes at the landing commit.

The earlier row 4 result at f3724a2d07a8 was exit 2 COULD-NOT-CHECK, because no Verification-Table marker was on main. It is superseded by this run and kept above as history.

Totals: 4 required rows. Rows 1, 3 and 4 pass. Row 2 exits 0 but fails its mutation obligation (the parser-exemption control does not discriminate). Unrun rows: 0.

Raw outputs that print the role-context flag by name, roster echo lines and the full statusgen output are kept in the verifier's local scratch and not copied here.

**VERIFY: FAIL** — row 2: TestPerfV3CLIContextPrecedence does not fail when a claim or token-custody entrypoint's parser exemption is flipped in the CLI migration registry. The row's Expect names that change as a failing control. Row 4 now passes at a570e840520a: exit 0, 13 corroborated, 0 disproved, 2 unchecked, both out of scope.
### Independent verification — 2026-10-27 (dispatched verifier, claude-opus-5-5, tier strong)

Subject: merged main 3a140e11192866bf4c06bae7a318333d576d6f09, in a detached home that was clean before and after every step. Delivery is the two-parent merge 4b7f4847f7f3 (PR #3582), on the first-parent chain of the subject. The Verify section is byte-identical at the original, base, delivery and subject revisions, and it was not amended after delivery, so no acceptance transition applies. deskpathguard rederive found 4 original rows, 0 author-added, all from base. Every row ID, Command, Expect and class below is the original one. Admission: deskdispatch --check-verifier exited 0 before the first row and again before this landing, with the same binding (attestation #4618). The row 2 repair from #3781 is in the subject.

Verification-Table: delivery=4b7f4847f7f3531185fdfb23fbf978ae9551a242 base=b73d90d774fca8aec8974ef410da63ebcb10eece original=3d737b4871bf604e698c6c96427bb89c5e794db7

Rows 1 to 3 are check:ci. The darwin host cannot run the unshare sandbox, so they ran in the container lane inside one closed admission bracket, with no edit to the home between the steps:

1. On the host, statusgen verifyrun --open-bracket (pinned v1.0.63) recorded the home tree digest at 18:44:47Z.
2. The rows ran with statusgen verifyrun --sandbox=container-netns --rows 1,2,3 in the pinned runner image ghcr.io/medici-finance/verify-runner:v1.0.63 (go1.27.2, linux/amd64; digest on the Container-Image line). The container settings were:
   - --network none, --read-only, --cap-drop ALL and no-new-privileges, with the engine's default seccomp profile;
   - read-only bind mounts for the home at its own path, for the repository's git directory (the home is a linked worktree), for a pre-warmed Go module cache and for the admission receipt; the only writable mount was a new directory for the witness file;
   - a tmpfs at /tmp;
   - GOPROXY=off, GOSUMDB=off, GOFLAGS=-mod=readonly, GOTOOLCHAIN=local and KUBECONFIG=/dev/null.
   The step exited 0.
3. On the host, --close-bracket closed the bracket at 18:47:05Z, and --bracket-digest then reported verdict=match.

The module cache is an environment input and does not relax the sandbox. The image's own cache predates the 2026-10-25 tools/desk module refresh: it lacks golang.org/x/sys v0.49.0 and its siblings, so an offline build of rows 1 and 2 against it fails with a module-cache permission error. The cache was therefore filled by go mod download for tools/desk and loopadmin, in a separate networked container from the same image, before the bracket opened. An earlier supplemental probe on an archive copy, against the image's own cache, ran no witness and is not credited.

The block below is the closed witness file, byte for byte:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3LauncherChildFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3LauncherChildFlow ' "$out")` | pass exit=0 sandbox=container-netns | sha256:a603a7e66097 | 2026-10-27 | assay-verifier-app[bot] @ 3a140e111928 (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3CLIContextPrecedence$' ./internal/clicontract/ > "$out" && grep -F -- '--- PASS: TestPerfV3CLIContextPrecedence ' "$out")` | pass exit=0 sandbox=container-netns | sha256:ff51dbd7b3f4 | 2026-10-27 | assay-verifier-app[bot] @ 3a140e111928 (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RunnerIdentityLifecycle$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3RunnerIdentityLifecycle ' "$out")` | pass exit=0 sandbox=container-netns | sha256:6f48fd77ba7d | 2026-10-27 | assay-verifier-app[bot] @ 3a140e111928 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:60739cd7cd47c269cc1c35d0c634ace19ed97562a41e7d8e2a15173dda6010ba
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4618 run=3a9a9acf9834435e24c59571547e2a2e86179330afbd0e171e0c6850471ef3a7 source=3a140e11192866bf4c06bae7a318333d576d6f09 brief=docs/streams/desktools-v2/brief-64-propagate-performance-context-through-desks-agents-and-tool-clis.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=3a9a9acf9834435e24c59571547e2a2e86179330afbd0e171e0c6850471ef3a7 source=3a140e11192866bf4c06bae7a318333d576d6f09 tree=sha256:60739cd7cd47c269cc1c35d0c634ace19ed97562a41e7d8e2a15173dda6010ba witness=sha256:0dff1bccf89c1098953f6e5b927f067f13cca902c26cde255ff6492aa2509a03 opened=2026-10-27T18:44:47Z closed=2026-10-27T18:47:05Z before=receipt after=check-verifier

Row 4 is class check (not check:ci). It reads git history and needs no network sandbox, so it ran on the host as a disjoint selection: statusgen verifyrun --rows 4 --fail-first --base 9d4ae27f2d0d (the first parent of the delivering merge), after the bracket closed. The table below is that run's witness as printed:

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 4 | `statusgen --root . --consumers --brief desktools-v2/64` | pass exit=0 | sha256:7d1ac15c760b | 2026-10-27 | assay-verifier-app[bot] @ 3a140e111928 (on-behalf-of human:ian) (forge-identity) | base=9d4ae27f2d0d red rc=2 · head=3a140e111928 |
Verification-Attestation: medici-finance/assay#4618 run=3a9a9acf9834435e24c59571547e2a2e86179330afbd0e171e0c6850471ef3a7 source=3a140e11192866bf4c06bae7a318333d576d6f09 brief=docs/streams/desktools-v2/brief-64-propagate-performance-context-through-desks-agents-and-tool-clis.md model=claude-opus-5-5 tier=strong

Row 4 output line: summary: 13 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing. The two unchecked entries are out of scope, the same as at a570e840520a: graph-execution/19 and /16 with forge-neutral/40, and qualgen/ and the other standalone entrypoints. At base the row is red (rc=2).

Fail-first for rows 1 to 3 is a supplemental run, not credited. Native --fail-first needs a host checkout at base, and the darwin host cannot sandbox check:ci rows. So each row command ran unchanged, in the same offline container settings, on an archive of base 9d4ae27f2d0d. Every row exited 1 there, because the named test is absent at base.

Mutation plants are also supplemental and not credited. Each plant was applied to an archive copy of the subject outside the home. The row command then ran unchanged, offline, in the same image. Unmutated, each row exits 0.

| Row | Plant | Row exit |
|---|---|---|
| 1 | child mints no agent ID and inherits the parent's (perfrec lifecycle) | 1 |
| 1 | child's parent invocation blanked | 1 |
| 1 | typed child context not propagated by the shared executor (control) | 1 |
| 1 | process-global sticky context across concurrent launches | 1 |
| 1 | child context inherits the parent's work beside its own assignment (ChildContext ranges over p.Work plus work) | **0, survives** |
| 2 | deskboot parser state exempt to migrated in the CLI migration registry | 1 |
| 2 | deskclaim exempt to migrated | 1 |
| 2 | deskclaim-ref exempt to migrated | 1 |
| 2 | desktoken exempt to migrated | 1 |
| 2 | current Go parser selection dropped | 1 |
| 3 | explicit resume never preserved | 1 |
| 3 | native mapping retained past seven UTC days (-6 to -7) | 1 |
| 3 | unknown mapping continuity gap concealed | 1 |
| 3 | runner context boundary skipped (control) | 1 |
| 3 | telemetry-off diagnostics context blocks launch | 1 |

Row 2's parser-exemption control now discriminates. All four exemption flips that survived at f3724a2d07a8 are red at the subject.

Row 1 does not discriminate stale inherited work. The Expect says a planted inherited stale work fails, but the row's parent fixture carries no work, so a child that copies the parent's work into its own assignment still passes. Two supplemental probes confirm the gap is in the fixture:
- with the parent fixture given one work entry, the unmutated subject passes;
- with that fixture and the same plant, the row fails.

A one-line fixture change would close it.

Advisory, not counted: a plant that fails launch when no performance input is attached at all survives row 3. The row's disabled-sink case attaches a telemetry-off context, and that plant is red, so the Expect's claim holds as written.

RISK-VALUE: DERIVED — native mapping retention cutoff = today.AddDate(0, 0, -6) @ loopadmin/runner/performance.go:54. It keeps mappings dated today-6 through today, which is seven UTC days, as the brief's maximum seven-UTC-day retention requires. The -7 plant is red under row 3.
RISK-VALUE: DERIVED — handoff directory mode = 0700 @ tools/desk/cmd/deskdispatch/performance_handoff.go:28, with a group/other refusal mask of 0077 @ performance_handoff.go:32, and handoff file mode = 0o600 @ tools/desk/internal/packet/packet.go:747. These give an owner-only directory and file, and refuse any group or other bit, as the brief's 0700/0600 requires. No Verify row plant exercised them in this pass.
RISK-VALUE: DERIVED — handoff expiry = today.AddDate(0, 0, 7) @ tools/desk/cmd/deskdispatch/performance_handoff.go:49, and prune cutoff = today.AddDate(0, 0, -6) @ performance_handoff.go:56. The handoff is valid through today+6 and pruned once older than today-6, so both are seven UTC days.
RISK-VALUE: NAMED, NOT DERIVED — opaque ID size = 16 random bytes @ perfrec/context.go:114. The spec asks for random IDs in the recorder's existing format and states no width. MaxWork = 8 and MaxWorkBytes = 4096 @ perfrec/context.go:76-77 are owned by desktools-v2/63, not this brief.

Totals: 4 required rows.
- Rows 1 to 3 have a bracket-credited witness, each pass exit=0, sandbox=container-netns. Row 4 has a host witness, pass exit=0.
- Fail-first: every row is red at base.
- Unrun rows: 0.
- Mutation: rows 2 and 3 discriminate every plant run. Row 1 discriminates four of five; the stale inherited work plant survives.

Raw row outputs and plant logs are kept in the verifier's local scratch and not copied here.

**VERIFY: FAIL** — row 1: TestPerfV3LauncherChildFlow still passes when a child inherits its parent's work, the "stale work" plant its Expect names, because the parent fixture carries no work. Rows 2 to 4 pass. Row 2's parser-exemption defect from 2026-10-20 is repaired.

### Fresh re-verify — 2026-10-21 at 5dcdff9fb1ec

The table at the end of this block is the statusgen verifyrun witness, copied back unchanged. It ran inside a local docker container: image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e, git 2.47), `--network none`, sandbox=container-netns, default seccomp, no live infrastructure (KUBECONFIG=/dev/null). Subject: merged source 5dcdff9fb1ec67c0bf80aa60f778da5aef4687b7, detached home, clean before and after every run apart from this Evidence. verifyrun exited 0 with 4 of 4 rows passing. The runner and environment follow the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker).

Verification-Attestation: medici-finance/assay#3809 run=32bc2676b236da52c751d5114ea607f919b37bcd479fd377c0abe22a0f020159 source=5dcdff9fb1ec67c0bf80aa60f778da5aef4687b7 brief=docs/streams/desktools-v2/brief-64-propagate-performance-context-through-desks-agents-and-tool-clis.md model=claude-opus-5-5 tier=strong

Verification-Table: delivery=4b7f4847f7f3531185fdfb23fbf978ae9551a242 base=b73d90d774fca8aec8974ef410da63ebcb10eece original=3d737b4871bf604e698c6c96427bb89c5e794db7

Why this re-run: the previous run at f3724a2d07a8 recorded a fail verdict, with its row 4 follow-up at the landing commit a570e840520a. Row 2's parser-exemption control did not discriminate because the named test hardcoded its entrypoint list. Since then, medici-finance/assay#3781 changed only that test: it now derives the exercised binaries from docs/performance-entrypoints.json joined to the CLI migration registry, and it validates the registry on every exercised row. Separately, the parser flag for the role context was renamed to its loop-role form. That rename touches the CLI helper, its test and the brief's Task text. The Verify table is unchanged. `deskpathguard rederive` re-derived 4 original rows, 0 author-added, all from base. Admission: `deskdispatch --check-verifier` exited 0 before the first row and exited 0 again after the last, with the same binding.

By-hand runs: host macOS, go1.27.2, pinned desk shims v1.0.53, offline, KUBECONFIG=/dev/null. Each ran in a scratch clone at the source with a dead push URL and origin/main pinned to the source.

| # | Command | Expected | Observed | Date / runner |
|---|---|---|---|---|
| 1 | exact row 1 command | exit 0; named PASS; plants fail | exit 0 — `--- PASS: TestPerfV3LauncherChildFlow (1.03s)` (sha256:4befbc4a997f). All plants named by the row are red: inherited agent, child invocation reused as parent, stale work, cross-task global context (3 of 3 reps), manufactured opt-in, attribution overriding the desk loop. | 2026-10-21 assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |
| 2 | exact row 2 command | exit 0; named PASS; claim identity, token custody or parser-exemption changes fail | exit 0 — `--- PASS: TestPerfV3CLIContextPrecedence (11.30s)` (sha256:59d7a1529c52). The claim-identity and token-custody plants are red, and all four parser-exemption plants are red (table below). Discriminating. | 2026-10-21 assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |
| 3 | exact row 3 command | exit 0; named PASS; lifecycle plants fail | exit 0 — `--- PASS: TestPerfV3RunnerIdentityLifecycle (0.00s)` (sha256:f2e42d58df1a). Red: no explicit resume, retention past seven UTC days, concealed unknown-mapping gap, replacement inheriting the previous agent, disabled sink blocking launch, resume keeping stale work. Discriminating. | 2026-10-21 assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |
| 4 | exact row 4 command | exit 0; routing corroborated; unavailable is not pass | exit 0 (sha256:e3fb036a0321) — `summary: 13 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing` over base 9d4ae27f2d0d..4b7f4847f7f3 (immutable delivered snapshot). Both unchecked entries are out of scope: graph-execution/19 and /16 with forge-neutral/40, and qualgen/ with the other standalone entrypoints. | 2026-10-21 assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (claude-opus-5-5 dispatched verifier, hand run) |

Row 2 fail-first, the control that failed last time. Each plant flips one registry state from exempt to migrated in docs/streams/desktools-v2/cli-migration.json, one at a time, and runs the exact row 2 command:

| Plant (parser exemption flipped) | Registry line | Row 2 exit |
|---|---|---|
| deskclaim | 598 | 1 |
| deskclaim-ref | 619 | 1 |
| desktoken | 1387 | 1 |
| deskboot | 564 | 1 |
| baseline (restored) | — | 0 |

Each plant fails inside TestPerfV3CLIContextPrecedence, reporting that the inventory's parser exemption disagrees with the registry. A joint flip of both the registry and the inventory for desktoken is still red, through the registry-validation arm alone. medici-finance/assay#3804 is verified-fixed by medici-finance/assay#3781 at 5dcdff9fb1ec.

Mutation summary (exact row commands; every plant compiles and the clone was clean after each set):
- Row 1, shipped deskkit perfcontext spec: the control and 6 plants are red. 5 are not caught by row 1 alone: present empty flag, nil parser context, CI runner suite, second launcher invocation, untyped grandparent. These target other tests in the spec's wider selection. Row 2 kills present-empty-flag and nil-parser-context.
- Row 3, shipped runner performance spec: the control and 3 of 4 are red. The second-module plant targets TestNoGraphDependency, which is outside this row.
- Shipped deskdispatch handoff spec, run with its own test: the control and 18 of 18 are red. Supplemental only.
- Totals: 4 required rows, all pass and discriminate. Unrun rows: 0.

RISK-VALUE: DERIVED — mapping retention cutoff = today.AddDate(0, 0, -6) @ loopadmin/runner/performance.go:54. It keeps today−6 through today, exactly seven UTC days, and a −7 plant is red under row 3.
RISK-VALUE: DERIVED — handoff dir mode = 0700 @ tools/desk/cmd/deskdispatch/performance_handoff.go:28, group/other refusal mask = 0077 @ performance_handoff.go:32, handoff file mode = 0o600 @ tools/desk/internal/packet/packet.go:747. The handoff is owner-only, as the brief requires. Shipped handoff mutations are red, but no Verify row covers these values.
RISK-VALUE: DERIVED — handoff expiry = today.AddDate(0, 0, 7) @ tools/desk/cmd/deskdispatch/performance_handoff.go:49 and prune cutoff = today.AddDate(0, 0, -6) @ performance_handoff.go:56. Both bounds are seven UTC days.
Ranked lower, reversible: perfrec NewID = 16 random bytes @ perfrec/context.go:114. MaxWork = 8 and MaxWorkBytes = 4096 come from desktools-v2/63. The medici-finance/assay#3781 test oracles are the inventory path, its schema string and the parsed-to-recorder state.

Advisory (no verdict weight):
- A plant that removes the telemetry gate inside the resolver still survives row 2. perfrec's own TestPerfV3ContextAndConsent kills it.
- The 19 October amendment cases (separate protocol-off, concurrent two-work, unknown service context) are outside the protected table and were not re-examined in this pass.

Raw outputs that print the role-context flag by name, roster echo lines and full test logs stay in the verifier's local scratch and are not copied here.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3LauncherChildFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3LauncherChildFlow ' "$out")` | pass exit=0 sandbox=container-netns | sha256:369919d0f0b5 | 2026-10-21 | assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3CLIContextPrecedence$' ./internal/clicontract/ > "$out" && grep -F -- '--- PASS: TestPerfV3CLIContextPrecedence ' "$out")` | pass exit=0 sandbox=container-netns | sha256:d1ef66704cda | 2026-10-21 | assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RunnerIdentityLifecycle$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3RunnerIdentityLifecycle ' "$out")` | pass exit=0 sandbox=container-netns | sha256:f2e42d58df1a | 2026-10-21 | assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --root . --consumers --brief desktools-v2/64` | pass exit=0 sandbox=container-netns | sha256:3c6bfee6a7c4 | 2026-10-21 | assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (forge-identity) |

**VERIFY: PASS** — at 5dcdff9fb1ec all four rows exit 0 with their named result, both in the container-netns verifyrun witness and by hand. Row 2's parser-exemption control now discriminates: deskclaim, deskclaim-ref, desktoken and deskboot each exit 1. Rows 1 and 3 kill their named plants, and row 4 corroborates 13 with 0 disproved.

### Independent second verification attempt — 2026-10-21

Fresh independent gpt-6-astra / strong verifier, admitted run 72193a797b5e9c2281ddbac5bdf82d50a584d66f6becee36cbd287c331c33356, attestation #3818, source 5dcdff9fb1ec67c0bf80aa60f778da5aef4687b7. Repair #3781 is merged at 4395628b9ff5575e2c5d7d62f8ded2902d1c9ee7 and is an ancestor of this source. Expectations were written from the brief and specification before inspecting implementation, tests or prior Evidence. Native host admission exited 0 before and after execution; the full returned bindings were byte-identical. The admitted home remains detached, with no tracked or untracked changes.

Verification-Table: delivery=4b7f4847f7f3531185fdfb23fbf978ae9551a242 base=b73d90d774fca8aec8974ef410da63ebcb10eece original=b73d90d774fca8aec8974ef410da63ebcb10eece

Delivery #3582 was independently read from the forge. Its first parent is 9d4ae27f2d0dcf429b1ac2e82094464b6ba2a7a3; the parents' unique merge-base is the table base above. Explicit re-derivation at that base returned four original rows and zero author-added rows. Complete table bytes agree at divergence, first parent, delivery and observed source (SHA-256 e8b641ef3c071e3dfa78ba41da08987da106bddd1123cbff783d5b9266f2b545).

The installed v1.0.53 statusgen executed `verifyrun --fail-first --base 9d4ae27f2d0dcf429b1ac2e82094464b6ba2a7a3 --dry-run` against the exact admitted home and brief. Exit 2. It returned the following native witness, retained unchanged. No command filter, modified Verify/Expect, substituted binary, installation or confinement relaxation was used.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3LauncherChildFlow$' ./internal/deskkit/ > "$out" && grep -F -- '--- PASS: TestPerfV3LauncherChildFlow ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (forge-identity) | base=9d4ae27f2d0d unproven · head=5dcdff9fb1ec |
| 2 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3CLIContextPrecedence$' ./internal/clicontract/ > "$out" && grep -F -- '--- PASS: TestPerfV3CLIContextPrecedence ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (forge-identity) | base=9d4ae27f2d0d unproven · head=5dcdff9fb1ec |
| 3 | `(cd loopadmin && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestPerfV3RunnerIdentityLifecycle$' ./... > "$out" && grep -F -- '--- PASS: TestPerfV3RunnerIdentityLifecycle ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-21 | assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (forge-identity) | base=9d4ae27f2d0d unproven · head=5dcdff9fb1ec |
| 4 | `statusgen --root . --consumers --brief desktools-v2/64` | pass exit=0 | sha256:7d1ac15c760b | 2026-10-21 | assay-verifier-app[bot] @ 5dcdff9fb1ec (on-behalf-of human:ian) (forge-identity) | base=9d4ae27f2d0d red rc=2 · head=5dcdff9fb1ec |

Verification-Attestation: medici-finance/assay#3818 run=72193a797b5e9c2281ddbac5bdf82d50a584d66f6becee36cbd287c331c33356 source=5dcdff9fb1ec67c0bf80aa60f778da5aef4687b7 brief=docs/streams/desktools-v2/brief-64-propagate-performance-context-through-desks-agents-and-tool-clis.md model=gpt-6-astra tier=strong

Row accounting: 4 required; 1 passed; 0 failed; 3 could-not-run; 0 deferred or author-added. Rows 1–3 were refused before their test commands executed, at both base and head. Their required fail-first and mutation checks therefore remain unproven. No named PASS or mutation result from the earlier verifier is adopted as this run's result.

Row 4 independently re-ran exactly as written: exit 0, `summary: 13 corroborated, 0 disproved, 2 unchecked, 0 brief(s) claiming nothing`. The consumer paths agree with the immutable delivery diff. The two unchecked claims are the declared out-of-scope replacement owners and standalone entrypoints, retained for reviewer judgment. Native fail-first records base exit 2 and head exit 0; that base result is unavailable routing evidence, not a demonstrated implementation failure.

Environment blocker: #1800 is open and tracks this exact Darwin network-sandbox limitation. The current pinned verify-runner v1.0.53 image was inspected locally and is absent. The operator must make the pinned image available and complete its required installed-runner qualification, then a fresh independent verifier must run the exact table with valid admission, fail-first and mutation evidence. A local unqualified image is not a substitute. Full raw logs, pre/post receipts and hashes are retained by the desk.

The earlier FAIL, first-stamp PASS and its three advisories remain preserved as history. Their claims are not promoted or retracted by an unavailable second run. Full risk-value enumeration and derivation, and the separate service-amendment coverage questions, remain incomplete in this attempt. Human done/sign-off stays separate from start decision #3537. The existing Evidence PR's control hold is separate from this runner blocker.

**VERIFY: BLOCKED** — second stamp incomplete: rows 1–3 could not run in the required sandbox; row 4 passed. No lifecycle advancement or second PASS is supported.

## Review

Gate: human. Reject changed legacy meanings, guessed identity/cost, lost coverage or optional measurement that changes an operation.
