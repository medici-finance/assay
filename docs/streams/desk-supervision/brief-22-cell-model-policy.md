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

### Non-implementer verifier run — VERIFY: PASS — 2026-09-23 opus-5.5-verifier

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | python3 tools/cellctl/tests/model-policy.test.py | named tests pass, incl. mixed-provider show/up/desk agreement and recording-harness exec args | exit 0; "Ran 17 tests ... OK"; each test line ends "... ok" incl. test_desk_show_and_up_agree_on_mixed_providers and test_real_exec_argv_and_env (no SKIP) | 2026-09-23 | opus-5.5-verifier |
| 2 | bash tools/cellctl/tests/provider.test.sh | exit 0; existing provider credential and override behavior preserved without policy | exit 0 in a clean env: "provider.test.sh: OK". Ambient run showed 2 FAILED (the "no provider → BASE_URL/model vars NOT exported" assertions) caused by ANTHROPIC_* vars inherited from the launching cell shell leaking into the test — a verifier-harness artifact, not a merged-main regression (see Findings) | 2026-09-23 | opus-5.5-verifier |
| 3 | bash tools/cellctl/tests/harness.test.sh | exit 0; existing Claude/Codex launch behavior preserved without policy | exit 0; "harness.test.sh: OK" (up threads --harness codex to every role window; invalid CELL_HARNESS refused) | 2026-09-23 | opus-5.5-verifier |
| 4 | bash tools/cellctl/tests/model-namespace.test.sh && bash tools/cellctl/tests/model-override.test.sh && bash tools/cellctl/tests/cell-set.test.sh | all three suites exit 0; no legacy pin/override/persistence regression | exit 0 chained; "model-namespace.test.sh: OK", "model-override.test.sh: OK", "cell-set.test.sh: OK" | 2026-09-23 | opus-5.5-verifier |
| 5 | bash -n tools/cellctl/cellctl | exit 0 | exit 0; no syntax errors | 2026-09-23 | opus-5.5-verifier |

Hermetic-witness note (all rows are class check:ci): `statusgen verifyrun` could-not-run every row —
the network-off sandbox uses `unshare --net`, a Linux facility unavailable on this darwin host. The
formal network-off re-execution of the check:ci rows is could-not-check here and must be produced on
a Linux CI runner; the rows above were executed directly (the model-policy suite is offline by design
— local Git fixtures + recording stubs, per the policy doc). The verifyrun witness rows are left
uncommitted in the worktree brief file for the desk.

RISK-VALUE (trigger: the diff pins a hard house standing constraint — "Prohibit Opus 5"; gate:model.
irreversible:no, so every literal below is reversible by editing the JSON/source and re-launching):

- RISK-VALUE: DERIVED — opus5_ban = ["*opus-5*", "*opus5*"] @ tools/cellctl/cellctl:734 — the bash
  oracle appends these to the policy `deny` list; base() strips a trailing [1m] before the
  case-insensitive glob, so claude-opus-5, its [1m]/gateway suffixes and the concatenated Opus5
  spelling are all denied. This satisfies the brief's contract "prohibit Opus 5, including context
  suffixes and explicit child requests." (Over-broad vs the CURRENT doc — the trailing `*` also
  matches opus-5-5, which the later version-floor work says must be allowed. See Findings.)
- RISK-VALUE: DERIVED — example_opus_alias = claude-opus-4-8[1m] @ tools/cellctl/examples/model-policy.json:44 — the strong tier (the `opus` alias) is pinned to 4.8, exactly the brief's "map the example Opus tier to 4.8", and it is an exact ID, not a floating alias.
- RISK-VALUE: N/A for the remaining enumerated literals — they are reversible operational config, not
  irreversible acts: example anthropic top = claude-fable-5-1 (roles/the-desk avoids Opus by design),
  GLM 5.3 / Kimi K3 effort set = {low, high, max} @ tools/cellctl/cellctl:767, native base URL
  https://api.anthropic.com @ tools/cellctl/cellctl:829 (policy path only, provider==anthropic),
  schema pin = 1. Each is edit-and-redeploy reversible; none is an irreversible transfer/spend/publish.


## Review

Gate: model. Examine fallback/settings precedence, credential routing and the negative tests.
Do not infer deployment or independent verification from implementer test output.
