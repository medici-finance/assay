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

| # | Command | Expect |
|---|---------|--------|
| 1 | `ls components/harness-*/component.yaml \| wc -l` | 3 |
| 2 | `grep -l 'assay.harness' plugins/assay/skills/*/component.yaml \| wc -l` | equals the number of skill directories |
| 3 | `grep -n 'exclusive: true' components/KEYS.md` | one line, for `assay.harness` |
| 4 | `deskmanifest lint --root .` | exit 0 |
| 5 | mutation: mark a second adapter installed in the fixture; `deskmanifest lint --root <fixture>` | exit 1; `assay.harness has 2 ACTIVE providers`; restore |
| 6 | fixture with the codex adapter only: `deskmanifest lint --root <fixture> --activation` | exit 0; hooks component listed `INACTIVE — assay.harness flavour codex ≠ claude-code` |
| 7 | `git diff --stat origin/main -- plugins/assay/.claude-plugin plugins/assay/.codex-plugin plugins/assay/cursor plugins/assay/hooks/hooks.json` | empty (delivery shapes' bytes unchanged) |
| 8 | `cd tools/desk && go test ./cmd/deskmanifest/...` | exit 0 |
| 9 | neighbour: `tools/harnessgen` invoked as the cursor adapter's apply step vs. invoked directly | identical output |
| 10 | `statusgen --lint` | exit 0 |

## Evidence

Pending — the implementer records its run here on reaching `implemented`; an independent
runner records a second run on merged main before `verified`.

## Review

gate: model — pending.
