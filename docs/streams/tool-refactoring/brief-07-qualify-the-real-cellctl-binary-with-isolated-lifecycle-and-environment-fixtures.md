---
brief: assay:assay:tool-refactoring:07
title: "Qualify the real cellctl binary with isolated lifecycle and environment fixtures"
why: "Some shell suites never exercise the Go product and fixture behavior depends on operator configuration. Real-binary isolated round trips expose lifecycle and environment regressions before release."
wave: 1
depends: ["tool-refactoring/06"]
unblocks: ["tool-refactoring/11"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v2
version: 1
id: e136350e-c8ae-4449-afa6-e90b4d03733d
outcome: none
authored: 2026-11-12 by author-brief session
sources:
  - "docs/streams/tool-refactoring/spec.md — draft scope; not implementation or activation approval"
  - "freshness-checked 2026-11-12 @ 680102a0877755feab9ef85dc4e1d3f52a137a54; source inspection and bounded issue/PR snapshots; not a reproduced production failure"
  - "medici-finance/assay#6185 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6175 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6156 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5823 — related class evidence; resolution not asserted"
  - "medici-finance/assay#6251 — related class evidence; resolution not asserted"
  - "medici-finance/assay#5877 — related class evidence; resolution not asserted"
  - "medici-finance/assay#1631 — related class evidence; resolution not asserted"
  - "desktools-v2/06 — retained owning work; see existing-work.md"
  - "desk-containers/28 — retained owning work; see existing-work.md"
  - "desk-containers/29 — retained owning work; see existing-work.md"
  - "agent-execution/03 — retained owning work; see existing-work.md"
exec-tier: strong
exec-tier-why: "Cross-component class refactoring and negative controls; one local happy-path test cannot prove that every production caller migrated."
domain: complicated
consumers:
  - "tools/desk/cmd/cellctl/lifecycleintegration_test.go: follow-up tool-refactoring/07 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/cellctl/shellfloor_test.go: follow-up tool-refactoring/07 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/cellctl/launch.go: follow-up tool-refactoring/07 (this brief; source implementation is still owed)"
  - "tools/desk/cmd/cellctl/shims.go: follow-up tool-refactoring/07 (this brief; source implementation is still owed)"
  - "tools/cellctl/tests/: follow-up tool-refactoring/07 (this brief; source implementation is still owed)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up tool-refactoring/07 (this brief; source implementation is still owed)"
gate-why: "Changes admission, process/environment isolation or conditional publication of adopter records. Human acceptance confirms that custody and lower-boundary refusals survive the migration; no live cutover is included."
decision-trigger: start
---

# Brief 07 — Qualify the real cellctl binary with isolated lifecycle and environment fixtures

## Context

files: tools/desk/cmd/cellctl/lifecycleintegration_test.go, tools/desk/cmd/cellctl/shellfloor_test.go, tools/desk/cmd/cellctl/launch.go, tools/desk/cmd/cellctl/shims.go, tools/cellctl/tests/, docs/streams/desktools-v2/cli-migration.json, docs/contracts.md, docs/library-first.md, tools/desk/README.md, changelog/tool-refactoring-07.md. Newly named Go/test files are planned deliverables; the existing source paths are the starting seams. No out-of-repository files.

facts (2026-11-12, source `680102a08777`):
- Class: qualify the real cellctl binary with isolated lifecycle and environment fixtures; the issue cases in sources nominate this mechanism, not a claim that every old report still reproduces.
- Existing owner: cellctl integration adapters and existing execution-profile conformance.
- Required reading: [scope](spec.md), [retained work and active PRs](existing-work.md), `docs/library-first.md`, `docs/contracts.md`, and the source paths above. Source inspection precedes any implementation effect.
- All dependency and caller changes are this brief's bounded scope. Retained work: desktools-v2/06, desk-containers/28, desk-containers/29, agent-execution/03. Completed Evidence and dated sources stay unchanged.

layering: keep pure syntax/state/judgment in its named existing library owner and source, process, credential, forge and persistence effects in adapters. Module boundaries are real: desk-tools may import public statusgen subpackages; statusgen cannot import internal deskkit. No new daemon, effect journal, generic bypass or public command is needed.

single-point-of-failure: a caller can bypass the shared decision — the class guard must detect a second planted caller; independent current-state, custody and conditional-write/process checks still refuse at their own effect boundary. Pure-library tests prove no effect authority.

design-fit:
  owner: cellctl integration adapters and existing execution-profile conformance
  contract: cell-op-plan/result; loop-admin-runner-v1; execution-profile-v1
  retires: ["the duplicated projections or inference paths explicitly migrated in Task; inventory and prove deletion before claiming class retirement"]
  weight: "verbs +0; public flags +0; new bypasses +0; duplicate rule bodies decrease after migration; adapter refusal reasons consolidate without dropping independent checks"
  why-add: "Reuse existing owners and profiles. Add only the typed internal seam/tests required below; record actual source and rule-text deltas in delivery."

Sizing: L keeps the real-binary lifecycle round trip and its owned-process cleanup harness together; splitting the oracle inventory away from the child boundary would allow another nominally green suite that never exercises the product. Scope excludes new operator commands and live backends.

## Human decision

The proposed refactor changes a control-bearing path used by automated workers.
The implementation must preserve explicit identity, independent source and permission
checks, and truthful unavailable outcomes. Planning approval cannot activate it.

Options:
1. Approve this bounded migration after the reviewed implementation proves the old controls still refuse unsafe cases.
2. Keep the current path and request a narrower refactor.

Default if no answer: none — blocks until answered.

## Ground rules

- Isolated branch and draft PR; no merge, release, live infrastructure call or control weakening. Stop at implemented.
- At pickup/resume, record Architecture pickup against the current target, contracts, actual APIs and affected work; adapt only within scope. An active material amendment needs its owner checkpoint.
- Refresh issue/PR status and reuse landed repairs before changing code. A scope refusal is a stop.

## Task

Managed workspace creation, attach/finish and retention remain with the shared `workspace` contract/allocator migration. Test the supported current local profile explicitly; no fake allocator can establish managed readiness. Exercise the real allocator adapter only after its owning consumer ships, with a separate truthful profile result.

1. Migrate local up/down/settle callers to brief 06 and exercise the built Go binary through disposable roots, a private session transport and a bounded stub harness. Never link the operator HOME, gh config, keys or real role credential into a fixture.
2. Inventory every shell oracle suite, name its real Go-binary coverage, and either execute it hermetically, port its contract cases into the existing Go runner, or record a reviewed obsolete-case disposition. Incorporate PR 6184 and related repairs if landed; keep no parallel lifecycle implementation as the permanent oracle.
3. Assert environment/profile and clean-up behavior at the actual child boundary with synthetic sentinels; verify that ambient credential aliases do not cross it. Credential-selection changes remain desktools-v2/06; harness/model/backend semantics remain agent-execution/01–03.
4. Bound time and clean up only recorded fixture-owned processes. Repeat up/plan/apply/down/up under Linux and darwin-compatible local fixtures. A forbidden scratch-HOME run is could-not-check and needs a sanctioned runner, never a guard bypass. Real Kubernetes qualification stays with desk-containers/28–29.
5. Update the named API/CLI reference and existing semantic-owner row; add the changelog fragment. Keep consumer routing deferred until delivery actually edits each site. Publish the class census and retired-callsite diff.

## Class guard

Enumerate every product launch path and shell suite in the source inventory. Plant a second child path that inherits a synthetic ambient credential or targets the shell oracle instead of the built Go binary; the harness must detect it and retain fixture-owned cleanup. Preserve the source-pinned regression cases, show a pre-fix red control and retain the planted positive fixture.

## Interface contract

cellctl integration adapters and existing execution-profile conformance owns the stated meaning. Inputs are explicit observations; outputs are typed decisions/records with unavailable states. Internal refactoring does not change a public schema, numeric exit contract, gate policy or custody boundary without its existing owner approval. Source-only success is not released/adopted capability.

## Verify

Row 4 runs only in a clean isolated checkout of the independently resolved implementation delivery D, with its protected delivery binding checked by the existing owner. There HEAD is D and HEAD^ is its first parent. Do not select current integration HEAD as a convenient substitute. If D or its binding cannot be established, record could-not-check. Other rows keep their admitted source and recorded witness identity.

The named tests below are implementation deliverables, not tests claimed to exist today. Each row requires the named `--- PASS:` line as well as exit 0, so a vacuous selector cannot pass. Class controls plant a second defect site outside the reported instance and prove the guard catches it; retain that positive control. Record the pre-fix red and post-fix green.

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && GOWORK=off go test -v ./cmd/cellctl -run '^TestCellLifecycleBinaryRoundTrip$' -count=1` | exit 0; output contains "--- PASS: TestCellLifecycleBinaryRoundTrip"; real binary round trip | check:ci +flow |
| 2 | `cd tools/desk && GOWORK=off go test -v ./cmd/cellctl -run '^TestCellLifecycleEnvironmentClassGuard$' -count=1` | exit 0; output contains "--- PASS: TestCellLifecycleEnvironmentClassGuard"; ambient sentinel and second launch-path plant | check:ci +mutation +neighbour |
| 3 | `cd tools/desk && GOWORK=off go test -v ./cmd/cellctl -run '^TestCellShellFloorCoverage$' -count=1` | exit 0; output contains "--- PASS: TestCellShellFloorCoverage"; shell suite accounting and cleanup | check:ci +dereference |
| 4 | `statusgen --root . --consumers --brief tool-refactoring/07 --delivery "$(git rev-parse HEAD)" --base "$(git rev-parse HEAD^)"` | exit 0; zero disproved and zero unchecked consumer routes for the delivered brief | check:ci +flow |

## Definition of Done

Every Task path has an actual caller or documented retention reason, consumer flows and planted controls pass, obsolete duplicate bodies are removed, and docs/reference plus changelog explain the surviving contract. An independent verifier fills Evidence; this brief does not self-award acceptance. No change to historical Verify or Evidence is included.

## Evidence

<!-- Independent verification appends actual commands, exits, output/digests, date and runner after implementation. Empty at authoring. -->

## Review

Review the named lower-boundary refusals, all migrated callers and module/import constraints, not only the helper. Pre-mortem: a helper can exist without being called (flow row); two copies can agree on an unsafe result (mutation/neighbor row); incompatible records can strand adopters (joined qualification in /11). Scope adequacy and unenumerated consumers remain review obligations.
