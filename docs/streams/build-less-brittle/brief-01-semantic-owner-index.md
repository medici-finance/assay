---
brief: assay:assay:build-less-brittle:01
title: Semantic-owner index in docs/contracts.md — one meaning, one home
why: >-
  No document says which module owns a meaning, so each fix recomputes it locally. Readiness is
  derived in about seven places, and when two derivations disagree the fix is another
  classification rule inside one of them. A short index naming the ONE owner of each shared
  meaning, and listing today's duplicates, gives every later brief something to fit ("Design
  fit") and gives reviewers a concrete test for "wrong layer".
wave: 0
depends: []
unblocks: ["build-less-brittle/02", "build-less-brittle/07", "build-less-brittle/08", "build-less-brittle/10"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format)"
sources:
  - "docs/streams/build-less-brittle/spec.md §2 D4, §3 row 6, §4.6"
  - "docs/contracts.md (the existing seam-contract pattern this extends; 117 lines at f7bde6bfa)"
  - "docs/streams/graph-execution/spec.md §2 (one eligibility evaluator) and statusgen/eligibility.go"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: docs/contracts.md has no semantic-owner section; no S-<slug> ids exist in the tree"
exec-tier: strong
exec-tier-why: "(b) the seed rows are a cross-artifact sweep: every duplicate implementation of a meaning must be found and cited by path, across tools/desk and statusgen."
domain: complicated
---

# Brief 01 — Semantic-owner index in docs/contracts.md

## Context

files:
- `docs/contracts.md`: add one section, `## Semantic owners — one meaning, one home`.
- `changelog/build-less-brittle-01.md` (planned)

facts:
- `docs/contracts.md` (at f7bde6bfa) defines a **seam contract** as three parts: a versioned
  artifact, a source-side coverage gate, and a consumer-side conformance run. It says a
  contract missing a part "is a convention with a version number". The new section must keep
  that definition honest: a semantic row is an **ownership record**, and it records which of
  the three parts it has, usually none.
- Readiness/eligibility candidates to seed as duplicates (re-verify each path at fresh main):
  `statusgen/eligibility.go` (the declared single evaluator, by its own header
  comment), the `plan` subcommands of `fanoutloop`,
  `verifyloop`, `reviewloop` and `scanloop`, `deskboard` dispatch, the `deskdispatch` phantom
  check, and `desksupervise` reconcile.
- Other meanings with more than one implementation, as of 2026-09-24: claims (`deskclaim-ref`,
  `deskclaim`), the ready flip (`deskflip`, `deskpost ready`), and the risk-lane derivation
  (deskflip terms, `BriefRiskFromBody`, `AuthorsRiskFromBody`). Re-establish each with
  `git grep -l <symbol> refs/remotes/origin/main -- tools/desk statusgen`.
- Decision records use `DR-<slug>` in the DECISIONS register (`spec/registers-v1.md` §7). A row
  cites a DR when one exists, else `none`.
- Ownership versus enforcement: a second **owner** of one meaning is a duplicate. A second
  **enforcement point** at a different trust boundary, failing for a different reason, is a
  legitimate layer (defense in depth). The table keeps them in separate columns.

design-fit:
  owner: docs/contracts.md
  contract: none — this brief creates the index; its own row is `S-semantic-index`
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines 0 (docs outside the ratcheted set)
  why-add: n/a (no ratcheted growth)

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Describe; don't redesign. A row records today's owner and duplicates. Choosing a new owner is a
  design brief's job, not this one's.
- Public tree: mechanisms and public issue numbers only.

## Task

1. Append the section after "What the pattern is not". Open it with three short paragraphs:
   the purpose (one owner per meaning); ownership versus enforcement (see facts); and the
   amendment rule. **Changing a row's owner or meaning amends its decision record in the same
   PR. A local exception in another module is a `design-fit` review finding.**
2. Add one table with columns
   `id | meaning | owner | decision record | duplicates (path: note) | enforcement points | contract parts (artifact/source-gate/consumer-run)`.
3. Seed at least these rows, ids `S-<slug>`: `S-eligibility`, `S-delivery`, `S-identity`
   (credential and identity resolution), `S-claim`, `S-worktree`, `S-decision-acceptance`,
   `S-review-verdict` (verdict validity and the ready flip), `S-publication-scan`,
   `S-status-derivation`, `S-exit-codes`, `S-semantic-index`. For every duplicate, cite a path
   verified at fresh main. Write `none found (<grep command>)` rather than leaving a cell empty.
4. Add a one-line "How a brief cites this" note. The `design-fit:` `contract:` key names an
   `S-` id, or `none — <why>`.
5. Write the changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. This deliverable is prose. Rows 1–3 gate
**presence**; rows 4–5 **dereference** its path claims. Quality (were the right duplicates
found?) is the review gate's.

| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c '^## Semantic owners — one meaning, one home$' docs/contracts.md` | `1` |
| 2 | `sed -n '/^## Semantic owners/,$p' docs/contracts.md \| grep -o -e S-eligibility -e S-delivery -e S-identity -e S-claim -e S-worktree -e S-decision-acceptance -e S-review-verdict -e S-publication-scan -e S-status-derivation -e S-exit-codes -e S-semantic-index \| sort -u \| wc -l \| tr -d ' '` | `11` |
| 3 | `sed -n '/^## Semantic owners/,$p' docs/contracts.md \| grep -ciE 'amends? its decision record'` | ≥ `1` |
| 4 | `set -o pipefail; sed -n '/^## Semantic owners/,$p' docs/contracts.md \| grep -oE -e 'tools/desk/[A-Za-z0-9_./-]+\.go' -e 'statusgen/[A-Za-z0-9_./-]+\.go' \| sort -u \| wc -l \| tr -d ' '` | ≥ `15` (the section cites real source paths, so row 5 cannot pass vacuously) |
| 5 | `out=$(sed -n '/^## Semantic owners/,$p' docs/contracts.md \| grep -oE -e 'tools/desk/[A-Za-z0-9_./-]+\.go' -e 'statusgen/[A-Za-z0-9_./-]+\.go' \| sort -u \| while read p; do test -e "$p" \|\| echo "MISSING $p"; done); test -z "$out" && echo CLEAN \|\| echo "$out"` | `CLEAN` (every cited source path exists at the verified SHA) |
| 6 | `sed -n '/^## Semantic owners/,$p' docs/contracts.md \| grep -E '^[\|] *S-eligibility' \| grep -c 'statusgen/eligibility.go'` | `1` (the declared evaluator is the owner) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer answers: does any seeded row name an owner that is
itself one of several competing implementations, without saying so?
