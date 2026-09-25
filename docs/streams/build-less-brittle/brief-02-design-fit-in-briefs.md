---
brief: assay:assay:build-less-brittle:02
title: "design-fit: in every new brief — owner, contract, retires, weight, why-add"
why: >-
  A brief today says what to build and how to check it, but never where the change belongs or
  what it replaces. So a symptom brief reliably yields a local patch, and a deletion is never
  written because no brief asks for one. A five-key design-fit block in every new brief's
  Context makes the author answer "which module owns this, what does it retire, and how much
  weight does it add" before any worker starts, and gives the reviewer something to hold the
  diff to.
wave: 1
depends: ["build-less-brittle/01"]
unblocks: ["build-less-brittle/04", "build-less-brittle/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format)"
sources:
  - "docs/streams/build-less-brittle/spec.md §3 rows 2 and 8, §4.2"
  - "spec/brief-v1.md §4.1 (consumers: and layering: precedents), §3.2 exec-tier"
  - "plugins/assay/skills/author-brief/SKILL.md (template, rule 9, dispatch checklist)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no design-fit, retires or why-add key in spec/, docs/brief-template.md or the author-brief skill"
exec-tier: strong
exec-tier-why: "(b) one convention lands consistently in three artifacts (spec, template, skill), downstream copies re-sync on their own pin bump, and the skill must end net ≤ 0 lines."
domain: complicated
consumers:
  - "spec/brief-v1.md §4.1: follow-up build-less-brittle/02 (this brief)"
  - "docs/brief-template.md: follow-up build-less-brittle/02 (this brief)"
  - "plugins/assay/skills/author-brief/SKILL.md: follow-up build-less-brittle/02 (this brief)"
  - "downstream project copies of the author-brief skill (synced bundles, byte-parity twins): out-of-scope (each adopter re-syncs on its own pin bump)"
---

# Brief 02 — design-fit: in every new brief

## Context

files:
- `spec/brief-v1.md`: §4.1 (Context section) gains the `design-fit:` block. §3.2's
  `exec-tier` row gains question (d).
- `docs/brief-template.md`: the Context block gains `design-fit:`.
- `plugins/assay/skills/author-brief/SKILL.md`: the template block, rule 9 (d), and the dispatch
  checklist.
- `changelog/build-less-brittle-02.md` (planned)

facts:
- The block (spec §4.2, verbatim keys): `owner`, `contract`, `retires`, `weight`, `why-add`.
  `contract` names an `S-<slug>` row of `docs/contracts.md` §"Semantic owners" (brief 01), or
  `none — <why>`. `weight` is a signed delta per ratcheted dimension (verbs, flags, refusals,
  rule-text lines). `why-add` is REQUIRED when any delta is positive and `n/a` otherwise.
- It is a Context line, like `consumers:` and `layering:`. It is **not** frontmatter, so the
  JSON schemas and `statusgen conform` are untouched. **No lint is added.** The grammar
  settles first, the same precedent the author-brief skill records for `consumers:`.
- Required on every NEW brief. Legacy briefs are not back-filled.
- Two rules the text must carry (spec §4.2): (1) consolidate meaning, preserve independent
  enforcement; (2) retiring a control at a trust boundary names the layer still refusing the
  threat, with a Verify row proving it with the retired layer absent. Tests pinning a retired
  refusal retire with it.
- New `exec-tier` question (d): "Is this a design brief raised by an error-class trigger?"
  yes → `strong`.
- Line counts at f7bde6bfa (for scale; the net ≤ 0 rows derive their own base): author-brief SKILL.md 768, docs/brief-template.md 218. The
  checklist has 9 items. It must stay at single digits by merging an item, not appending
  (the skill's own rule).

design-fit:
  owner: spec/brief-v1.md §4.1 (the brief body contract)
  contract: none — briefs have no semantic-owner row; the brief spec is its own decision record
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 (author-brief SKILL.md must not grow)
  why-add: n/a

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- The skill edit is **net ≤ 0 lines**. Offset additions by moving incident narrative to a
  findings link or cutting restatements that duplicate `spec/brief-v1.md`. Never cut a rule's
  operative sentence to make room.

## Task

1. `spec/brief-v1.md` §4.1: add a paragraph after the `layering:` paragraph defining
   `design-fit:` (the facts above). Add (d) to the `exec-tier` derivation in §3.2's row text.
2. `docs/brief-template.md`: add the `design-fit:` block to the Context example, with an
   example `contract: S-eligibility`.
3. `plugins/assay/skills/author-brief/SKILL.md`: add the block to the template. Add (d) to
   rule 9. **Merge** checklist items 8 and 9's neighbour so the list stays ≤ 9, with one item
   reading "New component, or any weight delta > 0 → `layering:`/`design-fit:` answered;
   `why-add` names what removal was considered". Offset every added line.
4. Write the changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Rows 1–4 gate presence. Row 5 dereferences the
example contract id. Row 6 is the net ≤ 0 weight row. Row 7 checks question (d) landed in both files. Row 8 is the neighbour row (the
shared-value trigger that reads Context text).

| # | Command | Expect |
|---|---------|--------|
| 1 | `sed -n '/^### 4.1 Context section/,/^### 4.2/p' spec/brief-v1.md \| grep -c -e 'design-fit:' -e 'why-add'` | ≥ `2` |
| 2 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' docs/brief-template.md` | ≥ `5` |
| 3 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' plugins/assay/skills/author-brief/SKILL.md` | ≥ `5` |
| 4 | `grep -cE '^\[ \] [0-9]+\. ' plugins/assay/skills/author-brief/SKILL.md` | ≤ `9` (and ≥ `1`: the checklist still exists) |
| 5 | `id=$(grep -oE 'contract: S-[a-z-]+' docs/brief-template.md \| head -1 \| cut -d' ' -f2); test -n "$id" && grep -cE "^[\|] *$id " docs/contracts.md` | `1` (the template's example cites a row that exists) |
| 6 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/02$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 7 | `grep -c 'Is this a design brief raised by an error-class trigger' plugins/assay/skills/author-brief/SKILL.md spec/brief-v1.md \| grep -cE ':[1-9][0-9]*$'` | `2` (question (d) landed in both the skill and the spec) |
| 8 | `cd statusgen && go test -run TestSharedValueTriggerIsNarrow -count=1 .` | `ok` (neighbour: the consumers trigger still does not fire on ordinary Context lines such as the new block) |
| 9 | `statusgen --consumers --root . --brief build-less-brittle/02; echo "exit=$?"` | `exit=0` at the PR head (no `consumers:` routing claim is disproved by the diff; the implementer replaces each self-routed entry with `fixed-here` in the same change). Exit 1 names the disproved claim |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer checks that the skill's offsets removed narrative or
restatement, never an operative rule sentence.
