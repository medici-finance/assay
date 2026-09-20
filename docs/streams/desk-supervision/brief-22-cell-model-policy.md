---
brief: "assay:assay:desk-supervision:22"
title: "Configure provider, model and effort per cell role"
why: "Floating aliases and inherited effort settings change desk behavior without a deliberate cell configuration change. Resolve explicit provider/model/effort assignments at the launch boundary and prohibit Opus 5."
wave: 0
depends: []
unblocks: []
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
gate-why: "An opt-in launcher configuration change with offline negative-path tests; no merge or review authority changes."
issues: []
schema: "brief-v2"
authored: "2026-09-20 by model policy implementation session"
sources: ["docs/cellctl.md", "docs/cellctl-model-policy.md", "direct operator request 2026-09-20"]
consumers: ["tools/cellctl/cellctl: fixed-here", "docs/cellctl.md: fixed-here"]
exec-tier: "strong"
exec-tier-why: "Must keep model aliases, provider credentials, effort and child defaults coherent across two harnesses."
domain: "complicated"
version: 1
id: "9e8796ad-37d2-421b-9a33-4cac0d2808ad"
---

# Brief 22 — Configure provider, model and effort per cell role

## Context

Home: medici-finance/assay. Implement the operator-authorized model mapping independently
of review-rule changes and scheduler work. The existing `cellctl` is the launch boundary;
do not make another scheduler or change a running desk from a prompt.

files:
- tools/cellctl/cellctl
- tools/cellctl/examples/model-policy.json
- tools/cellctl/tests/model-policy.test.py
- tools/cellctl/tests/scrubbed-cell.test.sh
- docs/cellctl.md
- docs/cellctl-model-policy.md
- changelog/cellctl-model-policy.md

facts:
- Legacy model selection is per harness but provider selection is cell-wide or per invocation.
- A floating model alias can advance versions independently of the launcher.
- Claude and Codex have different effort and child-default configuration surfaces.
- Existing provider tier work in PR 1353 overlaps alias environment construction; preserve legacy behavior and coordinate the policy arm when merging that work.

## Read first

[Cell model policy](../../cellctl-model-policy.md), the existing launcher, provider/harness
regression suites, and the official harness references linked in the policy document.

## Interface contract

`CELL_MODEL_POLICY` points to an operator-owned schema-1 JSON file. Explicit role assignments
select provider and tier; each provider tier pins a model ID, effort and supported levels.
Legacy behavior remains when absent. Policy mode refuses unknown/unmapped/denied requests,
unsupported effort, conflicting local allowlists and unsupported launch paths. Prohibit
Opus 5, including explicit child requests; map the example Opus tier to 4.8. Show/check/desk/up
must agree on the selected tuple. The launch record includes a policy hash, never credentials.

Propagate Claude aliases, child default and effort; pass Codex model/effort/child defaults
through CLI overrides. Preserve permission envelopes and credential custody. Codex explicit
child overrides and administrator-managed settings are documented limits, not claimed controls.

Single-point-of-failure: the harness applies local model restrictions — launcher validation
and explicit-request hooks sit above it; provider-side managed restrictions remain operator-owned.
This is configuration correctness, not containment of an agent that can execute a shell.

## Task

1. Implement the opt-in policy in standalone cellctl; keep its single-file release packaging.
2. Refuse invalid whole-cell plans before opening role windows. Preserve legacy shell suites.
3. Add offline request, mapping, effort, settings-conflict, credential and actual exec-argv tests.
4. Update the named operator docs and example; no site generator is involved in these pages.
5. Release/install through the existing procedure. Enable the policy and inspect a parent and
   child transcript before broadening rollout; source completion alone is not live adoption.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow | `python3 tools/cellctl/tests/model-policy.test.py` | named tests pass, including mixed-provider show/up/desk agreement and recording-harness exec arguments |
| 2 | check:ci | `bash tools/cellctl/tests/provider.test.sh` | exit 0; existing provider credential and override behavior preserved without policy |
| 3 | check:ci | `bash tools/cellctl/tests/harness.test.sh` | exit 0; existing Claude/Codex launch behavior preserved without policy |
| 4 | check:ci | `bash tools/cellctl/tests/model-namespace.test.sh && bash tools/cellctl/tests/model-override.test.sh && bash tools/cellctl/tests/cell-set.test.sh` | all three suites exit 0; no legacy pin, override or persistence regression |
| 5 | check:ci | `bash -n tools/cellctl/cellctl` | exit 0 |

Pre-mortem → detection: aliases drift to a prohibited model: direct/child/allowlist negative
cases in row 1. Wrong credential adapter: recorded mixed-provider exec in row 1. Effort
silently defaults: unsupported-effort and exact argument checks in row 1. These are offline
configuration checks; actual provider inference remains a rollout check, never a fabricated PASS.

## Evidence

Implementation-session local tests recorded in the draft PR. Independent verification pending.

## Review

Gate: model. Examine fallback/settings precedence, credential routing and the negative tests.
Do not infer deployment or independent verification from implementer test output.
