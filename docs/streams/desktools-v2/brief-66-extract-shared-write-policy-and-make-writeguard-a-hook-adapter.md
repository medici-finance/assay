---
brief: assay:assay:desktools-v2:66
title: Extract shared write policy and make writeguard a hook adapter
why: "Write-isolation decisions are trapped in a command package. Extract one reusable evaluator so command hooks and managed adapters share policy without sharing effects or weakening enforcement."
wave: 1
depends: []
unblocks: ["desktools-v2/31", "graph-execution/22"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: Local write-isolation and exemption behavior are security controls; review must confirm policy relocation and all adapters preserve refusals and independent sandbox enforcement.
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 26781d38-3bac-4c4e-8ba2-0688894744fb
authored: 2026-10-18 by Codex planning session
sources:
  - docs/write-policy.md
  - docs/library-first.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - freshness-checked 2026-10-18 @ c3d073572f41e09bedcac05944f9a202c3267353; current writeguard, deskcore and consumer module manifests inspected
exec-tier: strong
exec-tier-why: Cross-module extraction, command classification and security-boundary compatibility require adversarial flow and packaging judgment.
domain: complicated
outcome: none
consumers:
  - "deskcore/writepolicy: follow-up desktools-v2/66"
  - "tools/desk/cmd/writeguard, tools/desk/go.mod, tools/desk/go.sum: follow-up desktools-v2/66"
  - "docs/contracts.md, docs/write-policy-consumers.md, tools/desk/README.md: follow-up desktools-v2/66"
  - "tools/desk/cmd/writeguard CLI parsing: follow-up desktools-v2/31"
  - "loopadmin/adapters/localharness: follow-up graph-execution/22"
  - "managed launch/harness adapters outside the public runner: out-of-scope (separate consumer plans bind the producer version and qualify allocation/sandbox flow)"
  - "statusgen workspace lifecycle: out-of-scope (consumes workspace; only an actual duplicate write-policy evaluator requires migration)"
  - "push, credential, process, scratch and retirement guards: out-of-scope (independent owners and enforcement boundaries)"
---

# Brief 66 — Extract shared write policy and make writeguard a hook adapter

## Context

files: `deskcore/writepolicy/` (planned), `deskcore/archtest/`, `tools/desk/cmd/writeguard/`, `tools/desk/go.mod`, `tools/desk/go.sum`, `docs/write-policy.md`, `docs/write-policy-consumers.md` (planned), `docs/contracts.md`, `tools/desk/README.md`, `changelog/write-policy.md` (planned). Release/CI dependency readers and paths are re-enumerated at pickup.

facts:
- `Config`, `Verdict`, path checking and shell classification currently live in writeguard's `package main`; the types are not an importable consumer API.
- Path resolution reads the filesystem; exemption expiry reads time; main/callout read config, Git and process state. Moving files unchanged would not produce a pure evaluator.
- `deskcore` is the existing pure shared module, while neither tools/desk nor statusgen currently requires it at this source cut; explicit compatible module/release integration is owed.
- Legacy parse/Git failure allows; configured callout failure blocks. The semantic distinction and existing protocol must survive extraction.

layering: Pure `deskcore/writepolicy` receives explicit observations; hook/managed adapters collect effects and map outcomes. Task 2–4 and Verify 1–4 prove the boundary; sandbox enforcement remains independent.
design-fit:
  owner: S-write-policy, migrated from command-local decisions to deskcore/writepolicy only when implementation lands
  contract: S-write-policy in docs/contracts.md; docs/write-policy.md
  retires: [command-local policy implementation, duplicate evaluator sites named in the pickup census]
  weight: package +1 inside existing module; production verbs +0; flags +0; legacy refusals +0; one checked consumer inventory
  why-add: Reuse the existing core module; a narrow evaluator removes duplicated policy without moving observations or authority into libraries
single-point-of-failure: Heuristic command recognition cannot confine arbitrary filesystem effects; actual harness/OS write roots remain independent. Verify 3 and managed qualification test those lower boundaries separately.

## Read first

- `docs/write-policy.md`, `docs/library-first.md`, `docs/contracts.md` and `docs/go-engineering.md`.
- `plugins/assay/references/implementation-pickup.md` and current approved target/transition.
- Existing writeguard implementation/tests, deskcore architecture checks and real consumer manifests.

## Human decision

At pickup, present the source-bound compatibility report and proposed core/adapter interface.
Confirm that every existing denial and exemption condition is preserved, the legacy no-decision
failure path remains explicit, and managed admission does not inherit a legacy fallback.
Options:
1. **Approve the bounded extraction** — preserve legacy behavior and qualify the separate managed profile before activation.
2. **Narrow or hold it** — retain current behavior until the missing boundary or compatibility evidence is supplied.
Default if no answer: none — blocks the undecided security change until answered.

## Ground rules

Own isolated branch and draft PR; no merge, installation, release or workflow trigger.
Offline disposable fixtures only; export KUBECONFIG=/dev/null. Stop at implemented.
No existing control removal/weakening without its recorded authorization; preserve Evidence.

## Task

1. Perform architecture pickup. Census current and planned write-policy consumers and duplicates from fresh source; record revisions, queries, exclusions and each site's migrate/retain/follow-up owner. Migrate bounded actual duplicates here; route larger ones to concrete briefs before claiming closure. Do not relabel allocator users as policy duplicates.
2. Define versioned typed operation/observation/outcome fixtures and implement pure Classify/Evaluate in deskcore/writepolicy. Separate lexical classification from filesystem/ancestor observations. Unknown remains explicit; no environment, filesystem, clock, Git, process, callout, network or credential capability in the transitive core. Preserve sentinel-before-exemption and only-adds-refusals ordering.
3. Convert writeguard into a thin protocol/observation/callout adapter. Retain all hook tool names, JSON/exit/stdout, activation, configuration-source, sentinel and legacy failure behavior. Remove old decision functions when common fixtures pass; command-local wrappers may compose, never reimplement policy. CLI parsing remains /31's work only while that compatibility migration is retained; a separately authorized withdrawal does not retire this evaluator or /22's consumer.
4. Add the tools/desk module requirement/build wiring and qualify a separate external Go consumer with GOWORK=off. Settle compatible deskcore version/profile and release dependency resolution; source replacements alone are insufficient. Update source/consumer contract coverage and CI path triggers through the existing workflow-landing path.
5. Hand back exact API/profile/source versions to managed consumers, update the semantic owner and checked census, and preserve source-bound regression cases plus before/after observation-cost measurements. No claim that publishing source installs a hook.

## Interface contract

`docs/write-policy.md` defines proposed Classify/Evaluate, explicit path/identity/claim/callout observations and allow/deny/unknown. Settle exact Go/schema surface together. Core imports no workspace or desk internals; managed adapters map validated workspace records. Unknown protocol mapping belongs to named profiles. No exported raw credential/config getter.

## Verify

Named tests and paths below are implementation deliverables, not tests claimed to exist now.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +dereference | `(cd deskcore && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyCases$' ./writepolicy > "$out" && rg -q '^--- PASS: TestWritePolicyCases ' "$out")` | exit 0; independent expected decisions/reasons for shared/owned paths, aliases/ancestors, cwd/quotes/heredocs, unresolved expressions, claim expiry/scoping and sentinel order; identical explicit observations give identical results |
| 2 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWriteHookParity$' ./cmd/writeguard > "$out" && rg -q '^--- PASS: TestWriteHookParity ' "$out")` | exit 0; real built hook and direct import use the same production evaluator/fixtures; golden legacy denial/no-decision/activation/callout behavior preserved. Plant skipped deny, exemption-before-sentinel and callout-clears-deny in production; each must redden its case |
| 3 | check:ci +flow +mutation | `(cd deskcore && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyBoundary$' ./archtest > "$out" && rg -q '^--- PASS: TestWritePolicyBoundary ' "$out")` | exit 0; transitive effect/import checks plus explicit-input behavior; planted filesystem/helper, clock and command dependency each caught. No pure-library claim from an allowlist alone |
| 4 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestWritePolicyConsumers$' ./cmd/writeguard > "$out" && rg -q '^--- PASS: TestWritePolicyConsumers ' "$out")` | exit 0; source-bound census covers actual adapters/duplicates; added unregistered evaluator and stale consumer fail; external-module import builds offline from a supplied local module proxy with GOWORK=off and no relative replace |
| 5 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 ./cmd/writeguard/...` | exit 0; inherited regression cases retained or explicitly mapped to equivalent owner tests; POSIX and Windows lexical fixtures; actual native support claimed only with corresponding evidence |

## Definition of Done

All Task and named Verify obligations delivered, changelog and consumer docs updated,
module/release/CI reads accounted for, exact hand-back version recorded. Managed sandbox
flow and release qualification remain owned by their consumer plans; extraction alone
never marks them complete. Record invocation counts/elapsed time for identical fixtures
without inventing a performance target or claiming a heuristic is containment.

## Pre-mortem

A moved package still reads disk/time: Verify 3's planted helper and explicit-input tests.
A hook-shaped unit test never runs the binary: Verify 2 must execute the built hook.
A future caller keeps its own rule: Verify 4's census and separate managed flow proof.
A source replace hides a missing published dependency: Verify 4's external consumer.
A refactor weakens a guard while tests only check allow: deny/order mutations in Verify 2.

## Evidence

No implementation or independent verification claimed. Record actual commands, named
results, fixture/source/profile versions, environment and independent runner at delivery.

## Review

Gate: human. Review the compatibility report, all outcomes, consumer closure and
independent enforcement; no review grants merge, install or activation authority.
