---
brief: assay:assay:composability:04
title: Harness as an exclusively-bound key — adapters as components
why: >-
  Three harness manifests are kept in parallel by hand (a Claude Code plugin manifest, a Codex
  plugin manifest, a generated Cursor rules file), and the skills carry harness-specific shapes
  inline. Making the harness a key that exactly one adapter component provides at a time turns
  "add a harness" into "write one adapter manifest" and "switch harness" into a reconcile engine
  operation, and it gives the harness-portability stream the seam it measured for.
wave: 1
depends: ["composability/00"]
unblocks: ["composability/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-08 by composability authoring session
sources:
  - "docs/streams/composability/component-model.md §9 (assay.harness is exclusively bound; adapters provide it with a flavour; skills and hooks inject it)"
  - "arXiv 2608.25512 §6.2 Service Multiplexing (exclusive binding: several implementations share one interface, at most one bound at a time, switched by unloading one provider and loading another)"
  - "docs/streams/harness-portability/README.md — the measurement that the tools are harness-agnostic argv CLIs and only the delivery shapes are harness-specific"
  - "plugins/assay/.claude-plugin/plugin.json, plugins/assay/.codex-plugin/plugin.json, plugins/assay/cursor/, tools/harnessgen — the three shapes this brief puts behind one key"
  - "composability/00 — the manifests; the role skills inject assay.harness there"
consumers:
  - "plugins/assay/.claude-plugin/plugin.json: fixed-here (owned by the claude-code adapter component; content unchanged)"
  - "plugins/assay/.codex-plugin/plugin.json: fixed-here (owned by the codex adapter component)"
  - "plugins/assay/cursor/*.mdc + tools/harnessgen: fixed-here (the cursor adapter's apply step runs harnessgen)"
  - "plugins/assay/references/{claude-code,cursor,codex}.md: fixed-here (each becomes its adapter's documentation)"
  - "plugins/assay/hooks/hooks.json: fixed-here (owned by the claude-code adapter; hooks component injects assay.harness with flavour claude-code)"
version: 1
id: 6f59838c-f425-439d-a22c-41de87728705
---

# Brief 04 — Harness as an exclusively-bound key: adapters as components

## Context

files:
- **create** `components/harness-claude-code/component.yaml` (planned),
  `components/harness-codex/component.yaml` (planned), `components/harness-cursor/component.yaml` (planned) — each
  `provides: [assay.harness]` with `flavour: <name>`, and `apply` steps that install that
  harness's delivery shape (plugin manifest and hook wiring; codex manifest; `harnessgen`
  output). Their reverses are inside-boundary (files) and trivial.
- **edit** `plugins/assay/skills/*/component.yaml` and `plugins/assay/hooks/component.yaml` (planned) —
  `inject.required` gains `assay.harness`; hooks additionally require `flavour: claude-code`
  (the SessionStart/PreToolUse mechanism is Claude Code's).
- **edit** `tools/desk/cmd/deskmanifest/` — `provides` entries MAY carry attributes
  (`flavour`); `inject` entries MAY constrain them (`flavour: claude-code`); exclusive keys
  (declared in `components/KEYS.md` (planned) with `exclusive: true`) MUST have at most one ACTIVE
  provider — a second is a lint PROBLEM.
- **edit** `components/KEYS.md` (planned) — `assay.harness` marked `exclusive: true`.
- **edit** `docs/streams/harness-portability/README.md` — one paragraph pointing at the seam.

facts:
- **Exactly one adapter is ACTIVE.** The record (brief 03, when it lands) or, before that, the
  presence of exactly one adapter's installed shape, selects the binding. The lint refuses two.
- **Skills are not rewritten.** Where a skill body branches on harness today, it keeps the
  branch; the brief only declares the dependency so the branch has a named input. Removing the
  branches is harness-portability's work, not this brief's.
- **Hooks are Claude-Code-only** and say so via the `flavour` constraint; under another adapter
  the hooks component is INACTIVE with a report, which is the honest state (today it is silently
  absent).
- **`harnessgen` becomes an apply step**, not a separately remembered command.
- **No harness-specific content changes.** The three shapes are moved under owners; their
  bytes are unchanged, which row 7 checks.

## Ground rules

- Do not `git push`, trigger workflows, or run mutating infrastructure commands unless
  explicitly instructed. The deliverable is a draft PR opened by the desk verbs.
- Stop at `implemented`. Do not set `verified` or `done`.
- If a skill body turns out to depend on a harness feature not expressible as a `flavour`
  constraint, declare `assay.harness` required without a constraint and raise `NEEDS_CONTEXT`
  naming the feature; do not invent a second key.

## Task

1. Write the three adapter manifests; move ownership of the three delivery shapes to them
   (files stay where they are; the manifests' apply steps name them).
2. Add `assay.harness` to every skill's and the hooks' `inject.required`; add the `flavour`
   constraint on hooks.
3. Extend `deskmanifest lint`: provider attributes, inject constraints, exclusive keys.
4. Mark `assay.harness` exclusive in `components/KEYS.md` (planned).
5. Add tests: two adapters present → lint PROBLEM; hooks with a codex adapter → INACTIVE report;
   one adapter → clean.

## Verify

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `ls components/harness-*/component.yaml \| wc -l` | 3 |
| 2 | check | `grep -l 'assay.harness' plugins/assay/skills/*/component.yaml \| wc -l` | equals the number of skill directories |
| 3 | check | `grep -n 'exclusive: true' components/KEYS.md` | one line, for `assay.harness` |
| 4 | check | `deskmanifest lint --root .` | exit 0 |
| 5 | check +mutation | mutation: mark a second adapter installed in the fixture; `deskmanifest lint --root <fixture>` | exit 1; `assay.harness has 2 ACTIVE providers`; restore |
| 6 | check +flow | fixture with the codex adapter only: `deskmanifest lint --root <fixture> --activation` | exit 0; hooks component listed `INACTIVE — assay.harness flavour codex ≠ claude-code` |
| 7 | check | `git diff --stat origin/main -- plugins/assay/.claude-plugin plugins/assay/.codex-plugin plugins/assay/cursor plugins/assay/hooks/hooks.json` | empty (delivery shapes' bytes unchanged) |
| 8 | check | `cd tools/desk && go test ./cmd/deskmanifest/...` | exit 0 |
| 9 | check +dereference | neighbour: `tools/harnessgen` invoked as the cursor adapter's apply step vs. invoked directly | identical output |
| 10 | check | `statusgen --lint` | exit 0 |

## Evidence

Implemented on branch `feat/composability-04`. All ten Verify rows run locally (offline,
`KUBECONFIG=/dev/null`); `deskmanifest`, `harnessgen`, and `statusgen` built from the
in-tree source.

| # | Result |
|---|--------|
| 1 | PASS — `ls components/harness-*/component.yaml \| wc -l` = 3 |
| 2 | PASS — `grep -l 'assay.harness' plugins/assay/skills/*/component.yaml \| wc -l` = 12; `ls plugins/assay/skills/*/component.yaml \| wc -l` = 12 |
| 3 | PASS — `grep -n 'exclusive: true' components/KEYS.md` — one line, for `assay.harness` |
| 4 | PASS — `deskmanifest lint --root .` exit 0; last line `checked-clean` (26 manifests) |
| 5 | PASS — mutation: `touch AGENTS.md` (marks the codex adapter's evidence marker installed, a second ACTIVE `assay.harness` provider) → `deskmanifest lint --root .` exit 1, `PROBLEM: assay.harness has 2 ACTIVE providers: assay/harness-claude-code, assay/harness-codex`; `rm AGENTS.md` restores exit 0 |
| 6 | PASS (as a synthetic fixture, `TestLint_ActivationFlavourMismatchReportsInactive`, `go test`): a codex-only-adapter fixture + hooks requiring `flavour: claude-code` → `deskmanifest lint --activation` exit 0, report contains `assay/hooks: INACTIVE — assay.harness flavour codex ≠ claude-code`. (This repo's own tree cannot exercise the row directly — it has no codex-only state, by design: exactly one adapter, claude-code, is ACTIVE here per row 4/5.) |
| 7 | PASS — `git diff --stat origin/main -- plugins/assay/.claude-plugin plugins/assay/.codex-plugin plugins/assay/cursor plugins/assay/hooks/hooks.json` empty |
| 8 | PASS — `cd tools/desk && go test ./cmd/deskmanifest/...` exit 0 |
| 9 | PASS — `harnessgen cursor --check --root .` invoked twice (once standing in for "as the cursor adapter's apply step", once "directly") produced byte-identical stdout and exit 0 both times — the apply step names no wrapper, it is the same command |
| 10 | PASS — `statusgen --lint` → `LINT: PASS`, exit 0 (pre-existing NOTICEs only, unrelated to this brief) |

**Design note for review:** `component-model.md` §9 says "the presence of exactly one
adapter's installed shape selects the binding," ahead of the desired-state record (§7,
still planned). This repo itself ships all three harnesses' packaging simultaneously
(`.claude-plugin/`, `.codex-plugin/`, `cursor/assay.mdc` all committed, for adopters to
consume), so "a provider's `component.yaml` exists" cannot be the activation signal for
`assay.harness` without every tree having 3 simultaneous ACTIVE providers — an immediate,
permanent exclusivity PROBLEM in this very repo. To resolve that, this brief adds one
schema attribute beyond the two the brief text names (`flavour`, and the exclusivity
lookup in `components/KEYS.md`): a `provides` entry MAY also carry `evidence`, a
repo-root-relative marker path. A provider is ACTIVE only while its marker exists —
`.claude-plugin/marketplace.json` for claude-code (present here), root `AGENTS.md` for
codex, root `.cursor/rules` for cursor (neither present here, matching
harness-portability/README.md's own naming of those exact paths as each harness's native
install location). This gives exactly one ACTIVE provider in this repo today and a
literal, file-presence answer to "mark a second adapter installed" for the mutation row.
Flagging this explicitly since it is a modeling choice beyond the brief's literal text,
for the reviewer to confirm or redirect.
### Non-implementer verifier run — 2026-09-17 sonnet-5-verifier (verify-desk dispatch) — **VERIFY: PASS**

Runner ≠ implementer. Own detached temp worktree off `medici-finance/assay` origin/main. Deliverable PR #952 (`assay-worker-app[bot]`, own Evidence self-attributed/unverified per house rule) confirmed an ancestor of merged main. Every row independently re-run.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `ls components/harness-*/component.yaml \| wc -l` | 3 | exit 0, `3` | 2026-09-17 | sonnet-5-verifier |
| 2 | `grep -l 'assay.harness' plugins/assay/skills/*/component.yaml \| wc -l` | equals # skill dirs | exit 0, `12`=`12`; manually inspected all 12, each genuinely has `assay.harness` under `inject.required` | 2026-09-17 | sonnet-5-verifier |
| 3 | `grep -n 'exclusive: true' components/KEYS.md` | one line, `assay.harness` | exit 0, one line at :74 | 2026-09-17 | sonnet-5-verifier |
| 4 | `deskmanifest lint --root .` | exit 0 | built from in-tree source, exit 0, "checked-clean: 26 manifest(s)" | 2026-09-17 | sonnet-5-verifier |
| 5 | mutation: mark 2nd adapter installed, lint, restore | exit 1 naming both providers, restore exit 0 | exit 1, `PROBLEM: assay.harness has 2 ACTIVE providers: assay/harness-claude-code, assay/harness-codex` exact match; restored, clean | 2026-09-17 | sonnet-5-verifier |
| 6 | codex-only fixture, `--activation` | exit 0, hooks INACTIVE, flavour mismatch named | built an independent fixture (not copied from the repo's own test) with codex-only provider + hooks requiring `flavour: claude-code`; ran the literal CLI form (not `go test`) — exit 0, "assay/hooks: INACTIVE — assay.harness flavour codex ≠ claude-code" | 2026-09-17 | sonnet-5-verifier |
| 7 | `git diff --stat origin/main -- plugins/assay/.claude-plugin plugins/assay/.codex-plugin plugins/assay/cursor plugins/assay/hooks/hooks.json` | empty | exit 0, empty | 2026-09-17 | sonnet-5-verifier |
| 8 | `go test ./cmd/deskmanifest/...` | exit 0 | exit 0 | 2026-09-17 | sonnet-5-verifier |
| 9 | `harnessgen cursor` idempotence | identical output | built from source, ran twice, identical stdout, tree clean | 2026-09-17 | sonnet-5-verifier |
| 10 | `statusgen --lint` | exit 0 | exit 0, LINT: PASS, only pre-existing unrelated NOTICEs | 2026-09-17 | sonnet-5-verifier |

**RISK-VALUE: DERIVED** — the three activation-evidence marker paths (`.claude-plugin/marketplace.json`, `AGENTS.md`, `.cursor/rules`) that `computeActivation` gates exclusivity on, cross-checked against `docs/streams/harness-portability/README.md:44,51,136-137` independently (not trusted from prose) — match. `assay.harness exclusive: true` in `components/KEYS.md:74` confirmed matched by the actual regex `loadExclusiveKeys` uses (read in `lint.go`, not assumed). `flavour: claude-code` constraint on the hooks component's `inject.required` confirmed in the shipped `component.yaml`. Lowest-ranked: `version: 0.28.0` pin, cosmetic only.

**Existing-Evidence attribution:** the brief's own `## Evidence` is unattributed in body but its landing commit is authored by `assay-worker-app[bot]` (the implementer) — void per house rule; not relied on for any row above, every row independently re-run.

**VERIFY: PASS** — all 10 rows independently reproduced clean. No invented scope — all rows map 1:1 to files actually in the diff.

## Review

gate: model — pending.
