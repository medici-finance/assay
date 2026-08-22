---
brief: derived-board/01
title: brief-v2 spec — derived lifecycle cells, generated table, reserved graph keys; public re-stage of brief-rules + template
why: >-
  The board's lifecycle cells are hand-asserted prose that a second actor must remember
  to edit, so merged work reads as todo (desk-containers/02, 2026-08-22; dozens swept by
  hand on 2026-08-21). The spec that defines what a cell may claim has to change before
  any tool can derive it — and the public copy of that spec is already 139+40 lines behind
  the private one, so adopters are reading a different contract than the house runs.
wave: 0
depends: []
unblocks: ["derived-board/03", "derived-board/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-22 by derived-board scoping session
sources:
  - "docs/streams/derived-board/spec.md §2, §3, §5 — the derivation table, what stays hand-written, brief-v2 contents"
  - "docs/brief-rules.md rule 30 (Derived status cells) — the principle this brief generalizes"
  - "private dependency-graph design note §3.3, §3.4, §3.6 — the keys and ref grammar reserved under brief-v2, and the open v1-keys-vs-v2 question this resolves (re-staged publicly by this brief)"
  - "freshness-checked 2026-08-22 @ f78ea24 — public docs/brief-rules.md is 629 lines vs 768 private; docs/brief-template.md 98 vs 138"
exec-tier: strong
exec-tier-why: spec text that every downstream implementer and lint derives from; rule wording must be exact and must not contradict rules 30/31/36
consumers:
  - "assay-toolkit docs/brief-rules.md: follow-up derived-board/07 (private re-stage lands with the rollout)"
  - "assay-toolkit docs/brief-template.md: follow-up derived-board/07"
  - "plugins/assay/skills/author-brief/SKILL.md (template block): follow-up derived-board/05"
---

# Brief 01 — brief-v2 spec + public re-stage of the brief contract

## Context
files:
- `docs/brief-rules.md` — re-staged from the private copy FIRST (overlay: bring the public
  file up to the private text, preserving any public-only edits), THEN amended: rule 30
  generalized to all five lifecycle cells + `unknown`; new rule "the `Brief:` trailer is the
  only PR→brief edge"; new rule "a generated table is single-writer"; §"Derived surfaces"
  lists the stream README Briefs table.
- `docs/brief-template.md` — re-staged the same way, then: `schema: brief-v2`; reserved
  keys `gates:` / `feathers:` documented as OPTIONAL-but-KNOWN with the ref grammar; a
  note that there is deliberately no `status:` key.
- `docs/streams/graph-repos.yaml` (planned) — schema `graph-repos-v1`, `cell: assay`, the
  alias registry (`at`, `oit`, `rec`, `ac`, `mp`) per the graph design §3.3.
- `docs/dependency-graph-design.md` (planned) — overlay re-stage of the private dependency-graph
  design note, scrubbed for public self-containment, so the reserved keys have a public
  definition to point at.
- `docs/streams/derived-board/spec.md` — update §8 if a question is settled here.

facts:
- The private copies are the newer text; the public tree is the release home. Re-stage is
  an overlay, never a mirror, and public content must be self-contained (no private issue
  numbers, internal slugs, withheld paths) — scrub while overlaying.
- brief-v2 frontmatter = brief-v1 + `schema: brief-v2` + the reserved keys. No field is
  removed. `status:` is explicitly NOT a field (spec §3).
- The lifecycle vocabulary is exactly: `todo`, `in-progress`, `implemented`, `verified`,
  `done`, `blocked`, `unknown`. `unknown` carries a reason in parentheses on the board.
- Rule 30's three-state table stays; this brief adds the rows for `in-progress` /
  `implemented` (witness = PR with trailer) and the `unknown` arm.
- Public `statusgen --lint` today accepts only `schema: brief-v1`; a v2 brief in the tree
  would PROBLEM until brief 03 lands the parser. Therefore this brief changes the SPEC
  DOCS and registry only — no brief in the tree is flipped to v2 here.

## Ground rules
- NEVER git push / trigger workflows. Commit on the feature branch only.
- Stop at `implemented` — you do not set verified/done.
- If the private and public spec texts conflict in a way the overlay rule does not
  resolve, report NEEDS_CONTEXT with both excerpts; do not pick one silently.

## Task
1. Overlay-re-stage `docs/brief-rules.md` and `docs/brief-template.md` from the private
   copies; scrub for public self-containment.
2. Amend `brief-rules.md`: generalize rule 30 (table per spec §2, including `unknown`);
   add the trailer rule and the generated-table single-writer rule as numbered rules in
   §"Derived status cells"; add the stream README table to §"Derived surfaces".
3. Amend `brief-template.md`: `schema: brief-v2`; document `gates:` / `feathers:` and the
   ref grammar verbatim from the graph design §3.3–3.4 (reserved: parsed and validated,
   not yet gating); add the "no `status:` key" note with the one-line reason.
4. Add the alias registry `docs/streams/graph-repos.yaml` (planned).
5. Record in spec §8 that Q5 (feathering table) is deferred to the graph stream.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c '^schema: brief-v2' docs/brief-template.md` | `1` |
| 2 | `grep -c -E -e '^[\|] `todo`' -e '^[\|] `in-progress`' -e '^[\|] `implemented`' -e '^[\|] `verified`' -e '^[\|] `done`' -e '^[\|] `blocked`' -e '^[\|] `unknown`' docs/brief-rules.md` | `7` (one derivation row per lifecycle cell) |
| 3 | `grep -n -E 'Brief: <stream>/<NN>' docs/brief-rules.md` | at least one match inside §"Derived status cells" |
| 4 | `grep -n -E '^(gates|feathers):' docs/brief-template.md` | both keys present |
| 5 | `grep -c 'status:' docs/brief-template.md` | matches only the "no `status:` key" note (inspect: exactly 1 line, and it is the note) |
| 6 | `python3 -c "import yaml,sys;d=yaml.safe_load(open('docs/streams/graph-repos.yaml'));assert d['schema']=='graph-repos-v1' and d['cell'] and d['repos'];print('ok')"` | `ok` |
| 7 | `diff <(sed -n '1,20p' docs/brief-rules.md) <(git show origin/main:docs/brief-rules.md \| sed -n '1,20p'); wc -l docs/brief-rules.md` | line count ≥ 768 (re-stage brought the private text across) |
| 8 | `statusgen --root . --lint` | exit 0 — docs-only change, no brief flipped to v2 yet |
| 9 | `! grep -rn -E -e 'assay-toolkit#[0-9]+' -e 'jojig-dao/' docs/brief-rules.md docs/brief-template.md docs/dependency-graph-design.md` | exit 0 (no private issue refs leaked by the overlay) |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer question: does the generalized rule 30 contradict rule 31 (re-baseline obligation) or
rule 36 (Evidence record) anywhere? Quote the lines if so.
