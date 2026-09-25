---
brief: assay:assay:harness-portability:16
title: Codex long-context cap — compaction limit on every Codex desk launch, shipped in packaging, linted
why: >-
  OpenAI prices a GPT-5.5 / 5.6 / 6-Astra prompt above 272K input tokens at 2x input and 1.5x
  output for the whole request, not only the part past 272K. A Codex desk window launched by
  `cellctl` sets no compaction limit, so a standing desk that runs for hours can drift over that
  line and double its input cost without any signal. One config key (Codex's
  `model_auto_compact_token_limit`) prevents it. This brief puts that key on every Codex launch,
  ships it in the Codex packaging adopters install, adds a lint that fails if it goes missing,
  and gives standing desks a "compact or reboot" point between ticks. Impact is low but
  recurring, and the fix is a config change.
wave: 7
depends: ["harness-portability/14"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-23 by intake-desk authoring dispatch
sources: ["freshness-checked 2026-09-23 @ e284ba9b8 (origin/main): no `model_auto_compact_token_limit` / `compact` string anywhere under tools/cellctl, tools/desk/cmd/cellctl, tools/harnessgen, tools/harnesslint, plugins/assay/codex, plugins/assay/references/codex.md", "https://developers.openai.com/api/docs/models/gpt-5.5 read 2026-09-23: 'prompts with >272K input tokens are priced at 2x input and 1.5x output for the full session' (cached input not addressed on that page)", "https://developers.openai.com/api/docs/models/gpt-5.6-luna read 2026-09-23: >272K input tokens → 2x input, 1.5x output across the entire request", "https://developers.openai.com/api/docs/pricing read 2026-09-23: separate long-context input / cached-input / output columns for gpt-6-astra, gpt-5.6-*, gpt-5.5, gpt-5.4; no statement about ChatGPT-plan (subscription) applicability", "https://learn.chatgpt.com/docs/config-file/config-reference read 2026-09-23: `model_auto_compact_token_limit` (number) — 'Token threshold that triggers automatic history compaction (unset uses model defaults)'; `model_auto_compact_token_limit_scope` total|body_after_prefix, default total; project-scoped `.codex/config.toml` loads only for a trusted project", "https://github.com/openai/codex/pull/33972 (merged 2026-07-18): bundled GPT-5.6 metadata context_window / max_context_window 372000 → 272000", "https://github.com/openai/codex/issues/32486 (open 2026-09-23): default GPT-5.6 effective context (~353K) extends ~81K tokens past the 272K band; suggests `model_auto_compact_token_limit` as a local workaround", "https://github.com/openai/codex/issues/14456: profile-scoped compaction keys are ignored, top-level values honoured — so the cap is passed as a top-level `-c` override, never through a profile", "tools/desk/cmd/deskdispatch/references/ measured 2026-09-23 @ e284ba9b8: common-clauses 5488 B + one kit 8323–28280 B = 13.8–33.8 KB ≈ 3.5–8.5K tokens at 4 B/token; the brief is passed by path (prompt.go L115), and the largest brief in the repo is 61899 B (≈15.5K tokens)", "plugins/assay/skills/*/SKILL.md measured 2026-09-23: desk-role bodies 34–73 KB (the-desk 34395 … pr-review-desk 72621) read at boot"]
consumers: ["tools/cellctl/cellctl: follow-up harness-portability/16 (this brief; the four codex argv sites + known_cell_env_key + validate_env_key + cell.env scaffold comment)", "tools/desk/cmd/cellctl (launch.go, plan.go, set.go, new.go, model.go): follow-up harness-portability/16 (this brief; the Go port must match the bash oracle byte-for-byte on argv)", "tools/cellctl/tests/harness.test.sh, tools/cellctl/tests/scrubbed-cell.test.sh: follow-up harness-portability/16 (this brief)", "docs/cellctl.md: follow-up harness-portability/16 (this brief; documents the new per-role key)", "tools/harnessgen (codex.go + codex_test.go), plugins/assay/codex/packaging.md, plugins/assay/codex/config-assay.toml: follow-up harness-portability/16 (this brief; new generated artifact)", "tools/harnesslint (main.go, lint.go, lint_test.go, testdata/): follow-up harness-portability/16 (this brief; new codex-config mode)", "components/harness-codex/component.yaml: follow-up harness-portability/16 (this brief; the adapter names ownership of the new fragment)", "docs/adopting-assay.md: follow-up harness-portability/16 (this brief; the Codex install section places the fragment)", "plugins/assay/references/codex.md, plugins/assay/references/standing-note.md, plugins/assay/references/tick-contract.md: follow-up harness-portability/16 (this brief)", "tools/desk/cmd/deskdispatch (dispatch.go, prompt.go + a test): follow-up harness-portability/16 (this brief)", "tools/cellctl/cellctl smoke arm (`codex exec --ephemeral`, ~L2480): out-of-scope (a one-shot, read-only prompt of a few dozen tokens that exits after one turn and cannot approach the threshold)", ".github/workflows/ci.yml: out-of-scope (the harnesslint CI job is harness-portability/15's staged patch; adding the codex-config step to it is a workflow edit the authoring identity cannot write, left for the maintainer who applies that patch)", "docs/streams/harness-portability/README.md: fixed-here (row 16, wave 7 list, note on 16)"]
exec-tier: strong
exec-tier-why: >-
  (b) cross-component correctness: one value has to agree across the bash cellctl oracle, its Go
  port, a generated packaging artifact, a new lint mode and two harness-neutral references. The
  likely failure is one arm left without the cap while every test for the other arms passes.
version: 1
id: 8506492f-80c3-42d6-8c62-2c50d157645e
---

# Brief 16 — Codex long-context cap

## Context
files: tools/cellctl/cellctl, tools/cellctl/tests/harness.test.sh, tools/cellctl/tests/scrubbed-cell.test.sh, tools/desk/cmd/cellctl/{launch.go,plan.go,set.go,new.go,model.go} + tests, docs/cellctl.md, tools/harnessgen/{codex.go,codex_test.go}, plugins/assay/codex/packaging.md, plugins/assay/codex/config-assay.toml (new, generated), tools/harnesslint/{main.go,lint.go,lint_test.go,testdata/}, components/harness-codex/component.yaml, docs/adopting-assay.md, plugins/assay/references/{codex.md,standing-note.md,tick-contract.md}, tools/desk/cmd/deskdispatch/{dispatch.go,prompt.go} + a test, changelog/harness-portability-16-codex-context-cap.md
facts:
- The pricing cliff (dated 2026-09-23, re-check at the `sources:` URLs): above 272K input tokens → 2x input, 1.5x output, for the full request. Confirmed for API-key usage. **Open point:** whether it applies to ChatGPT-plan (subscription) Codex usage is undocumented. **Default: cap anyway.** The cap only moves compaction earlier and costs nothing if the cliff does not apply.
- Codex key: `model_auto_compact_token_limit` (top-level; `-c key=value` on the CLI). Profile-scoped values are ignored upstream, so always pass it top-level. Chosen default **240000** (32K headroom under 272K for one turn's growth plus the compaction pass itself).
- Codex argv sites at e284ba9b8, none carrying the key: bash `tools/cellctl/cellctl` L2070 (scrubbed live), L2320 (scrubbed dry-run plan), L2413 (house/k8s live; already prefixes `"${MODEL_POLICY_ARGS[@]}"`, which are `-c` overrides, L873), L2480 (smoke, out of scope); Go port `tools/desk/cmd/cellctl/launch.go` L123–128, `plan.go` L37–43 (`harnessArgv`), `smoke.go` L58 (out of scope). The non-scrubbed `DRY_RUN=1` line (L2303) prints no argv.
- Per-role key precedent: `CODEX_MODEL_<role>` (role dashes → underscores) with `CODEX_MODEL_default`, resolved at L616–623 and allow-listed by `known_cell_env_key` (L961) / Go `set.go`. The new pair is **`CODEX_COMPACT_LIMIT_<role>` / `CODEX_COMPACT_LIMIT_default`**.
- Packaging today: `harnessgen codex` emits only `plugins/assay/.codex-plugin/plugin.json`, a manifest with no config channel. Codex reads config from `~/.codex/config.toml` or a trusted project's `.codex/config.toml`, so the adopter-side carrier is a **generated TOML fragment** placed there.
- References: `standing-note.md` L75 "carries no compaction rule" refers to a durable memory store's retention, not model-context compaction. Neither it nor `tick-contract.md` says what a standing window does as its context grows. Both are harness-neutral (non-matrix markers), so harness key names belong in `codex.md` only.
- Exposure: dispatched sub-agents start at ≈3.5–8.5K tokens, plus a brief of up to ≈15.5K. The risk is long-running **standing** desk windows: a 34–73 KB skill body at boot plus hours of tool output.
single-point-of-failure: the Codex compaction key itself. Behind it: (1) the launch-time `-c` on every cellctl Codex arm, (2) the shipped config fragment for sessions cellctl did not launch, (3) the standing-note compact-or-reboot point, which works even if Codex ignores the key. harnesslint guards the source value, and harnessgen `--check` guards the derivation.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Public repo: no house values in code, fixtures or prose. Use `example-*` placeholders.

## Task
1. **cellctl (bash oracle).** Resolve `compact_limit` per role: `CODEX_COMPACT_LIMIT_<role>`, else `CODEX_COMPACT_LIMIT_default`, else the value in `$CELL_REPO/plugins/assay/codex/config-assay.toml` if that file exists, else the compiled fallback `240000`. Add `-c model_auto_compact_token_limit=<n>` to the L2070, L2320 and L2413 codex argv, after any `MODEL_POLICY_ARGS`. Print `compact=<n>` on the codex `[dry-run]` and `[launch]` lines. `set` refuses a non-positive-integer value, and `--force` cannot bypass that. A value over 272000 is allowed, but boot prints a `NOTICE` naming the pricing band. Allow-list both keys and add a commented example to the cell.env scaffold. Never touch the claude arm.
2. **cellctl Go port.** Make the same changes in `launch.go`, `plan.go` `harnessArgv`, `set.go`, `new.go` and `model.go`. The bash suites must pass with `CELLCTL=<go binary>`.
3. **harnessgen.** Add a `<!-- assay:codex-config ... -->` block to `packaging.md` carrying `model_auto_compact_token_limit = 240000`. `harnessgen codex` emits `plugins/assay/codex/config-assay.toml` from that block (with a header comment: generated; place at `.codex/config.toml` or merge into `~/.codex/config.toml`). `--check` diffs it the same way it diffs the manifest. Add the fragment to `components/harness-codex/component.yaml` `apply`. Document placement in `docs/adopting-assay.md`'s Codex section.
4. **harnesslint.** Add a new mode, `harnesslint codex-config <bundleDir>`. It exits 1 when `codex/config-assay.toml` is missing, lacks `model_auto_compact_token_limit`, or sets it above 272000, and exits 2 on an unreadable file. Add fixtures `testdata/codex-config/{missing-key,over-threshold,ok}/codex/config-assay.toml` and fail-first tests.
5. **References.** In `standing-note.md`, add a short neutral section, "Context-budget boundary": a standing window whose context nears the harness's compaction threshold writes its standing note at that tick boundary, then compacts or reboots. The note is what survives either. Reword L75 to say the non-rule is about durable-store retention. In `tick-contract.md` §Boundary, add one line: a tick is one-shot and never reaches the boundary, and window mode's compact-or-reboot point is in `standing-note.md`. In `codex.md`, add a "Context budget" paragraph naming the key, the default, the per-role cellctl key and the pricing cliff.
6. **deskdispatch.** Estimate tokens as bytes/4 of the assembled prompt plus the `--brief` file when given. Above `--prompt-token-budget`, which defaults to **32000** (about 1.3x today's worst case), print one `WARNING: assembled prompt ≈<n> tokens exceeds budget <b>` on stderr. The dispatch outcome and exit code stay unchanged. Add no env var.
7. Add a changelog fragment, `changelog/harness-portability-16-codex-context-cap.md` (planned).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `bash tools/cellctl/tests/harness.test.sh` | exit 0. New asserts pass: live codex argv carries `-c model_auto_compact_token_limit=240000`; `CODEX_COMPACT_LIMIT_worker_desk=200000` in cell.env yields `=200000`; the codex `[dry-run]` line shows `compact=`; the claude-arm argv does NOT contain `model_auto_compact_token_limit`; `set` refuses `CODEX_COMPACT_LIMIT_default=abc` even with `--force` | check +mutation |
| 2 | `bash tools/cellctl/tests/scrubbed-cell.test.sh` | exit 0; a codex scrubbed `[plan] argv` line contains `model_auto_compact_token_limit=` | check +neighbour |
| 3 | `cd tools/desk && go build -o /tmp/hp16-cellctl-go ./cmd/cellctl && CELLCTL=/tmp/hp16-cellctl-go bash ../cellctl/tests/harness.test.sh` | exit 0: the Go port passes the same asserts as row 1 | check +flow |
| 4 | `cd tools/desk && go test ./cmd/cellctl/ ./cmd/deskdispatch/` | exit 0 | check:ci |
| 5 | `cd tools/harnessgen && go test ./... && go build -o /tmp/hp16-hg . && cd ../.. && /tmp/hp16-hg codex --check --root "$PWD" && grep -n 'model_auto_compact_token_limit = 240000' plugins/assay/codex/config-assay.toml` | exit 0; the fragment is current and carries the key | check |
| 6 | `cd tools/harnesslint && go test ./... && go build -o /tmp/hp16-hl . && cd ../.. && /tmp/hp16-hl codex-config plugins/assay && ! /tmp/hp16-hl codex-config tools/harnesslint/testdata/codex-config/missing-key && ! /tmp/hp16-hl codex-config tools/harnesslint/testdata/codex-config/over-threshold` | exit 0: green on the real bundle, red (exit 1) on both bad fixtures | check +mutation |
| 7 | `/tmp/hp16-hl bindings plugins/assay/references && /tmp/hp16-hl bodies plugins/assay/skills && ! grep -q model_auto_compact plugins/assay/references/standing-note.md && ! grep -q model_auto_compact plugins/assay/references/tick-contract.md && grep -q model_auto_compact plugins/assay/references/codex.md && grep -q 'Context-budget boundary' plugins/assay/references/standing-note.md` | exit 0: references still neutral, and the key is named only in the Codex binding | check |
| 8 | `cd tools/desk && go test ./cmd/deskdispatch/ -run PromptBudget -v` | exit 0; output names a test that warns on an oversized fixture (prompt plus a >128 KB brief), one that stays silent under budget, and one that shows the exit code and dispatch plan unchanged when the warning fires | check +mutation |
| 9 | Live Codex long-session smoke (needs a Codex install): `codex exec --json -c model_auto_compact_token_limit=60000 --sandbox read-only -m "$CODEX_MODEL" "Print every file under plugins/assay/skills in full, one at a time" > /tmp/hp16-smoke.jsonl; grep -c -i compact /tmp/hp16-smoke.jsonl` | ≥1 compaction event. Evidence records peak input tokens from the token-count events: below 272000 and near the 60000 test limit (the skills tree is ≈106K tokens, so compaction must fire). If the stream carries no compaction marker, record `BLOCKED`/could-not-check with the file path, never a pass | gate:model +dereference |
| 10 | `statusgen --lint --root "$PWD" && statusgen --consumers --brief harness-portability/16 --root "$PWD"` | exit 0; no PROBLEM | check:ci |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" requires this section filled by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). All four risk answers are no. The change is config, lint and
prose. It removes or weakens no security control and publishes nothing. Out of scope: splitting
large skill bodies into on-demand references to cut the boot-time context. That is a separate
skill-length / prompt-audit item and is not tracked here.
