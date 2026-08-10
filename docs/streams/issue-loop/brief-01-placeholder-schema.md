---
brief: issue-loop/01
title: 'Placeholder-brief schema — statusgen treats an issue-<NN> brief as a first-class Next-up row'
wave: 0
depends: []
unblocks: ["issue-loop/02", "issue-loop/03"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md))
sources: ["INTAKE [I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md) (design sketch + open questions)", "tools/statusgen (brief parse + Next-up eligibility)", "methodology-metrics/08 (claim-aware Next-up the placeholders inherit)", "freshness-checked 2026-07-10 @ post-#288 main"]
why: >-
  A placeholder is only useful if Next-up dispatches it like a brief. This brief establishes
  the minimal schema (point-at-issue, no spec duplication) and makes statusgen recognize it —
  proven by a HAND-WRITTEN placeholder flowing through Next-up before brief 02 automates
  creation. It is the head of the critical path: nothing scans-and-emits into a shape that
  doesn't exist yet.
---

# Brief 01 — Placeholder-brief schema

## Context
files: `docs/streams/issue-loop/` (placeholders land here), `../assay-toolkit/statusgen/` (brief parse
+ Next-up), the brief-v1 parser (a placeholder is a REDUCED brief-v1 variant)
facts:
- Placeholder shape: a file `issue-<NN>.md` (or `<repo-slug>-issue-<NN>.md` when not this
  repo) with frontmatter ONLY — `brief: issue-loop/issue-<NN>`, `issue: <NN>`,
  `repo: <owner/name>`, `wave: 0`, `effort:` and `gate:` DERIVED (next fact), `status`,
  plus a one-line body `See issue #<NN> — the issue body is the spec.` No Task/Verify
  duplication: the GitHub issue IS the spec.
- **effort/gate derivation ([I-25](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-issues-as-a-first-class-workstream-inbound-issue-loop-scanne.md) open question, decided here):** effort defaults `M`
  (unknown-until-triaged); gate defaults `model` UNLESS the issue carries a risk label
  (`security`, `funds`, `daml`, or the issue title/labels trip the same daml/auth/funds
  trigger methodology/31 defines) → `human`. The derivation is DOCUMENTED and the scanner
  (brief 02) applies it; a human may override in the placeholder after creation.
- statusgen changes: parse the reduced frontmatter without demanding Task/Verify (a
  placeholder is exempt from the executable-Verify lint — it has none by design; the
  ISSUE holds acceptance); render placeholders as Next-up rows inheriting priority
  (stream P1 + `bug`-label boost when the issue is labeled bug), staleness, and
  claim-awareness (mm/08 — an open branch/PR for the issue excludes it).
- A placeholder is NOT subject to the point-quality/Evidence gates as a normal brief —
  its lifecycle is issue-driven (brief 04 close-out); statusgen must not flag it as an
  unbacked brief.

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Leave commits per task only.
- Stop at `implemented`. NEEDS_CONTEXT over guessing.

## Task
1. Define the placeholder schema (a `schema: placeholder-v1` discriminator in frontmatter);
   statusgen parses it, applies the effort/gate derivation, renders it on Next-up, and
   exempts it from Verify/Evidence lint.
2. Hand-write ONE placeholder for a real open issue (e.g. an unmapped app-layer bug) and
   show it appears in a scratch Next-up regen with correct derived gate.
3. Tests: placeholder parses; derived gate=human on a risk-labelled fixture issue,
   model otherwise; claim-awareness excludes it when a branch exists; a placeholder does
   NOT trip Verify-table or Evidence lint.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes the Task-3 cases |
| 2 | scratch Next-up regen with the hand-written placeholder present | the `issue-<NN>` row appears with its derived gate |
| 3 | `statusgen --root . --lint; echo $?` | 0 (placeholder does not trip brief lint) |

## Evidence
<!-- non-implementer rows. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `444e95a4`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok github.com/medici/statusgen 1.569s` — Task-3 cases in the passing suite | 2026-07-11 | opus-verifier |
| 2 | scratch Next-up regen with the hand-written placeholder present (`--span 50`) | 0 | `issue-loop \| issue-300 — …#300 — see issue (gate:model, effort:M)` appears as a first-class Next-up row (STATUS.md:56); derived gate=model (issue-300 `labels:[bug]`, no risk label), effort defaults M | 2026-07-11 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | advisory NOTICEs only; placeholder `schema: placeholder-v1` (frontmatter + 1-line body) trips no brief/Verify/Evidence lint | 2026-07-11 | opus-verifier |

**VERIFY: PASS** — statusgen renders a hand-written `issue-<NN>` placeholder as a first-class Next-up row with correctly-derived gate, and the placeholder is exempt from brief/Verify/Evidence lint.

## Review
Gate: model. Reviewer confirms the placeholder is exempt from Verify/Evidence lint (not
just silently passing), the gate derivation matches the documented rule, and claim-awareness
applies.
