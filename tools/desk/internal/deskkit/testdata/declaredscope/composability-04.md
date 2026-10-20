<!-- Regression excerpt: stream identifiers are synthetic; declaration syntax is retained. -->
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
- **edit** `docs/streams/example-stream/README.md` — one paragraph pointing at the seam.

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
