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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `python3 tools/cellctl/tests/model-policy.test.py` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 2 | `bash tools/cellctl/tests/provider.test.sh` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 3 | `bash tools/cellctl/tests/harness.test.sh` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 4 | `bash tools/cellctl/tests/model-namespace.test.sh && bash tools/cellctl/tests/model-override.test.sh && bash tools/cellctl/tests/cell-set.test.sh` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash -n tools/cellctl/cellctl` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 874d56de38a7 (on-behalf-of human:ian) (forge-identity) |

Verifier notes (2026-09-27, assay-verifier-app[bot], non-implementer, merged main 874d56de38a7; implementing commit ef102f7f7, PR #1388). The witness above is could-not-run for every row because check:ci rows need the Linux network-off sandbox and this host is darwin. The same five commands were also run by hand on the host, in a scrubbed environment (env -i keeping only HOME, PATH, TMPDIR, USER, LANG; KUBECONFIG=/dev/null):

- Row 1: exit 0. "Ran 17 tests ... OK". The named tests include test_desk_show_and_up_agree_on_mixed_providers, test_real_exec_argv_and_env, test_child_mapping_allowlist_and_hooks, test_direct_and_alias_requests, test_ban_cannot_be_removed_by_omitting_deny, test_unsupported_efforts and test_competing_allowlist_refuses_before_launch.
- Row 2: exit 0. 76 ok lines, "provider.test.sh: OK". In the unscrubbed parent agent-session shell the same suite exits 1 with 2 FAILED. The failing checks are "no provider: ANTHROPIC_BASE_URL is NOT exported" and "ANTHROPIC_API_KEY is left alone and no model vars are exported". The recorded launch env showed the caller's own ANTHROPIC_BASE_URL and ANTHROPIC_MODEL values passed through unchanged, because the suite's baseline arm does not unset ambient ANTHROPIC_* variables. That is a test-hermeticity gap from before this brief (both assertions come from earlier provider work), not a regression from it. CI runners do not carry those variables.
- Row 3: exit 0. "harness.test.sh: OK".
- Row 4: exit 0. model-namespace, model-override and cell-set each print OK.
- Row 5: exit 0 with no output.

Risk-bearing value enumeration (diff scope: tools/cellctl/cellctl policy arm, tools/cellctl/examples/model-policy.json, plus the brief Deliverables). Risk metadata is present with every field "no", so no trigger fires. The enumeration is recorded anyway:

1. built-in ban = ["*opus-5*", "*opus5*"] @ tools/cellctl/cellctl:740. This is the brief's only hard prohibition. Reversible.
2. the-desk rule = refuse any model containing "opus" @ tools/cellctl/cellctl:810. Reversible.
3. example deny = "*opus-5" @ tools/cellctl/examples/model-policy.json:4. Commit 647202b63 changed it from "*opus-5*". Reversible.
4. example strong tier = "claude-opus-4-8[1m]" @ tools/cellctl/examples/model-policy.json:44. Reversible.
5. alias map = {fable: top, opus: strong, sonnet: mid, haiku: fast} @ tools/cellctl/cellctl:742. Reversible.
6. effort level sets = claude {low, medium, high, xhigh, max} and codex {minimal, low, medium, high, xhigh} @ tools/cellctl/cellctl:769. The GLM 5.3 and Kimi K3 restriction {low, high, max} is @ tools/cellctl/cellctl:773. Reversible.
7. anthropic base URL = "https://api.anthropic.com" @ tools/cellctl/cellctl:835. Set in policy mode only. Reversible.

Every entry is an operator configuration value. An edit and a new release undo any of them.

RISK-VALUE: DERIVED. banned += ["*opus-5*", "*opus5*"] @ tools/cellctl/cellctl:740. The brief says "Prohibit Opus 5, including explicit child requests". The patterns are matched with fnmatchcase against the lowercased ID with the [1m] suffix removed, so they catch claude-opus-5, its [1m] form, its -5-0 and -5.0 forms, and the compact opus5 spelling. The resolver checks them for tier maps, for direct and alias requests, and for PreModelSwitch and child requests. Row 1's test_ban_cannot_be_removed_by_omitting_deny covers the omitted-deny case.

RISK-VALUE: NAMED, NOT DERIVED. the-desk non-Opus rule @ tools/cellctl/cellctl:810, and the same ban's trailing wildcard at :740. The brief text does not state either rule. Current main's model policy doc says the prohibition ends at the end of the token, so Opus 5.5 (claude-opus-5-5) is allowed and the-desk has a version floor at Opus 5.5. That describes the Go launcher after 647202b63. The bash launcher still matches claude-opus-5-5 against "*opus-5*" and still refuses every Opus model for the-desk. Open question for the desk: are the bash launcher and the doc meant to agree? This does not fail any Verify row.

Observations:
- Task 5 (live release and a parent/child transcript inspection) is a rollout check. It is not in the Verify table, and this offline pass did not perform it.
- The changelog fragment named in the Context file list was folded into CHANGELOG.md by the v1.0.16 changelog roll-up. That is the expected lifecycle, not a missing deliverable.

VERIFY: PASS. All five rows exited 0 on the host in a scrubbed environment. The statusgen witness is could-not-run on darwin, so it needs a Linux-runner witness if the flip requires a machine witness.

## Review

Gate: model. Examine fallback/settings precedence, credential routing and the negative tests.
Do not infer deployment or independent verification from implementer test output.
