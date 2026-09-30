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
  - "spec/brief-v1.md §4.1: fixed-here"
  - "docs/brief-template.md: fixed-here"
  - "plugins/assay/skills/author-brief/SKILL.md: fixed-here"
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
### Non-implementer verifier run — VERIFY: PASS — 9/9 rows meet their Expect, held (#1805) — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Merged main 0b033c711c9ec30f75cb8c2082a505b06958a423 (cross-checked against the forge's `commits/main`); implementing squash 274ece128 (#1842, PR head 43c692117f86). The deliverable files are byte-identical at the PR head, the squash commit and the main tip. `gate: model`, all risk answers `no`. The `statusgen verifyrun` execution witness was run on Linux (golang:1.25-bookworm, linux/arm64, `--network none`, statusgen built from main's own source, the adopter roster mounted read-only): rows 1–8 pass exit 0, lint exit 0 with 0 PROBLEM lines. Row 9 records `fail exit=0 … expected 1` because verifyrun reads the Expect cell's explanatory prose ("Exit 1 names the disproved claim") as an expected exit, and the row can only be decided at the PR head, which its own Expect says. **Held at implemented on witness row 9 alone**; the verifyrun prose-parse class is routed on #1805. The witness table is summarised here, not landed verbatim, so a parse artifact is not recorded as a disproved claim.

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 1 | row 1 command as written | at least 2 | printed `3`; Linux witness pass exit=0 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 0b033c711c9e |
| 2 | row 2 command as written | at least 5 | printed `5`, all in the new design-fit block of the brief template; witness pass exit=0 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 0b033c711c9e |
| 3 | row 3 command as written | at least 5 | printed `5`, all in the skill's new block; witness pass exit=0 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 0b033c711c9e |
| 4 | row 4 command as written | between 1 and 9 | printed `9`; witness pass exit=0 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 0b033c711c9e |
| 5 | row 5 command as written | `1` | id `S-eligibility`; printed `1` (the semantic-owner row in the contracts table); witness pass exit=0 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 0b033c711c9e |
| 6 | row 6 command as written | NET-OK | printed `NET-OK`; implementing commit found, so the base is its parent; skill 772 lines to 771; witness pass exit=0 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 0b033c711c9e |
| 7 | row 7 command as written | `2` | printed `2` (skill rule 9 and spec §3.2); witness pass exit=0 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 0b033c711c9e |
| 8 | row 8 command as written | ok | `ok` for the statusgen package; a `-v` re-run shows the named test's PASS line, so the run is not vacuous; witness pass exit=0 | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 0b033c711c9e |
| 9 | row 9 command as written, at the PR head 43c692117f86 against its merge-base 05c937307aa6, and at the squash against its parent | exit=0 at the PR head | printed `exit=0`; 3 corroborated, 0 disproved, 1 unchecked, on the host and on Linux. On merged main the same command reports COULD-NOT-CHECK (exit=2), the tool's documented state once the brief is no longer in the diff | 2026-09-30 assay-verifier-app[bot] (claude-opus-5-5) @ 0b033c711c9e |

Risk-bearing values. Risk metadata is present and all four flags are `no`; the diff is markdown only. Enumerated anyway, and every value is reversible by a doc edit:

RISK-VALUE: DERIVED — exec-tier question (d) binds to strong @ spec/brief-v1.md:64 and plugins/assay/skills/author-brief/SKILL.md:368 — the stream spec's own row for this brief names that one new exec-tier question, and its decision D5 runs design work at strong tier.
RISK-VALUE: DERIVED — why-add is required when any weight delta is above 0 @ SKILL.md:183 and spec/brief-v1.md:226 — the brief's facts, and the stream spec making `n/a` legal only at a zero delta.
RISK-VALUE: DERIVED — design-fit key count = 5 @ spec/brief-v1.md:221-227, SKILL.md:179-183, docs/brief-template.md:99-103 — the stream spec names exactly owner, contract, retires, weight and why-add.
RISK-VALUE: DERIVED — complexity-question count = four @ SKILL.md:135, SKILL.md:361, spec/brief-v1.md:64 — three existing questions plus (d).
RISK-VALUE: N/A — checklist size 9, the skill's net line bound and the template's example weight are operational bounds.

Findings: (1) **held on #1805** — witness row 9 is a verifyrun Expect-cell misparse over a row decidable only at the PR head; the row also ends in `; echo "exit=$?"`, so its exit status is always 0. (2) Advisory lint NOTICEs on this brief (row 8 unanchored `-run`, no `outcome:` field, obligation tokens) are authoring shape; none is a PROBLEM. (3) The out-of-scope consumers entry for downstream copies of the skill stays unchecked by the tool; the only other in-repo copy is a weight-counter fixture, not a synced twin. No implementation defect found.

VERIFY: PASS on all nine rows in the context each row specifies; held at implemented until witness row 9 can pass (#1805).

## Review
Gate: model (from frontmatter). The reviewer checks that the skill's offsets removed narrative or
restatement, never an operative rule sentence.
