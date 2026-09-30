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
### Verification — 2026-09-30 (assay-verifier-app[bot] @ e03f4f5c7c41 (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer first verification on merged main e03f4f5c7c412560a666d95383bee0444fb6d263 (implementation: squash 274ece128, #1842). gate: model, all risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim. It ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, a full clone pinned to this SHA, statusgen built from the clone's own source). Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `sed -n '/^### 4.1 Context section/,/^### 4.2/p' spec/brief-v1.md \| grep -c -e 'design-fit:' -e 'why-add'` | pass exit=0 | sha256:1121cfccd591 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' docs/brief-template.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -c -e '^ *owner:' -e '^ *contract:' -e '^ *retires:' -e '^ *weight:' -e '^ *why-add:' plugins/assay/skills/author-brief/SKILL.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -cE '^\[ \] [0-9]+\. ' plugins/assay/skills/author-brief/SKILL.md` | pass exit=0 | sha256:2e6d31a5983a | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 5 | `id=$(grep -oE 'contract: S-[a-z-]+' docs/brief-template.md \| head -1 \| cut -d' ' -f2); test -n "$id" && grep -cE "^[\|] *$id " docs/contracts.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 6 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/02$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/author-brief/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'Is this a design brief raised by an error-class trigger' plugins/assay/skills/author-brief/SKILL.md spec/brief-v1.md \| grep -cE ':[1-9][0-9]*$'` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go test -run TestSharedValueTriggerIsNarrow -count=1 .` | pass exit=0 | sha256:28ee1419f776 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --consumers --root . --brief build-less-brittle/02; echo "exit=$?"` | fail exit=0 | sha256:dfc1536b5189 | 2026-09-30 | assay-verifier-app[bot] @ e03f4f5c7c41 (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | brief Verify row 1: sed -n over the spec/brief-v1.md 4.1 range, piped to grep -c -e design-fit: -e why-add | 2 or more | exit 0, printed 3. The count at the implementing commit's parent is 0, so the row discriminates | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 2 | brief Verify row 2: grep -c for the five indented keys owner/contract/retires/weight/why-add in docs/brief-template.md | 5 or more | exit 0, printed 5 (lines 99-103, the new block). Count at the parent is 0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 3 | brief Verify row 3: the same five-key grep -c over the author-brief SKILL.md | 5 or more | exit 0, printed 5 (lines 179-183, the new template block). Count at the parent is 0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 4 | brief Verify row 4: grep -cE for checklist lines of the form open-bracket space close-bracket N-dot in the author-brief SKILL.md | 9 or fewer, and 1 or more | exit 0, printed 9 (items 1-9, lines 592-605). The parent also has 9, so design-fit was folded into item 8 and not appended | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 5 | brief Verify row 5: extract the first contract: S-slug from docs/brief-template.md, then grep -cE for a table row starting with that id in docs/contracts.md | 1 | exit 0, id=S-eligibility, printed 1 (docs/contracts.md line 165) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 6 | brief Verify row 6: locate the Brief: build-less-brittle/02 first-parent commit and compare the SKILL.md line count at the tip with the count at tip~1 | NET-OK | exit 0, printed NET-OK. impl=274ece128 resolved; the count is 771 at the tip and 772 at 274ece128~1 (net -1) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 7 | brief Verify row 7: grep -c for the question (d) sentence over SKILL.md and spec/brief-v1.md, piped to grep -cE for a nonzero per-file count | 2 | exit 0, printed 2 (SKILL.md line 368, spec/brief-v1.md line 64). Spec count at the parent is 0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 8 | brief Verify row 8: cd statusgen and go test -run TestSharedValueTriggerIsNarrow -count=1 (re-run with -v) | ok | exit 0, "--- PASS: TestSharedValueTriggerIsNarrow", "ok github.com/medici-finance/assay/statusgen". Extra probe: sharedValueTrigger returns empty on every design-fit block in the template, the skill and this brief (throwaway test file, since removed) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 9 | brief Verify row 9 as written on merged main: statusgen --consumers --root . --brief build-less-brittle/02, then echo exit | exit=0 at the PR head | On merged main it printed exit=2: "--consumers: COULD-NOT-CHECK: ... is not in the diff against e03f4f5c7c41" (merged, so there is no diff). This is not a disproval. The shell exit is 0 only because of the trailing echo | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 9a | row 9 at the PR head, as its Expect prescribes: checkout 43c692117f86, then statusgen --consumers --brief build-less-brittle/02 --base 05c937307aa6 (merge-base with the squash parent) | exit=0, no DISPROVED | exit=0. "summary: 3 corroborated, 0 disproved, 1 unchecked". The 3 fixed-here entries (spec, template, skill) are corroborated. The downstream-copies out-of-scope entry is UNCHECKED, which is unchanged since the merge-base (the reviewer's call per brief-rule 9) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |
| 9b | row 9 over the squash diff: checkout 274ece128, then statusgen --consumers --brief build-less-brittle/02 --base 274ece128~1 | exit=0, no DISPROVED | exit=0, same summary: 3 corroborated, 0 disproved, 1 unchecked | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ e03f4f5c7c41 (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — exec-tier question (d) yes -> strong @ spec/brief-v1.md:64 and SKILL.md:368 — the stream spec (docs/streams/build-less-brittle/spec.md lines 113-116) makes a design-owed class ride dispatch "at strong tier" once it reaches 3 counted instances or its 2nd merged fix. The (d) gloss ("enough counted instances or merged fixes to be owed a design") defers to that trigger rather than restating a number, so the tier binding and the spec cannot drift apart.
RISK-VALUE: DERIVED — why-add trigger = any weight delta > 0 @ spec/brief-v1.md:226 — this is copied verbatim from the stream spec section 4.2 grammar ("required when any weight delta is positive"). The template (line 103) and the skill (line 183) state the same threshold, and checklist item 8 (SKILL.md:602) enforces it at dispatch.
RISK-VALUE: DERIVED — design-fit key set = 5 @ spec/brief-v1.md:218 — the keys match the stream spec section 4.2 block exactly, key for key. The hand rows 2 and 3 count exactly those 5 keys in the template and the skill (0 at the parent).
RISK-VALUE: DERIVED — checklist size = 9 @ SKILL.md:605 — the skill's own rule (SKILL.md:610) keeps the list at single digits by folding, not appending. The parent already had 9 items, so folding design-fit into item 8 is the only compliant shape, and it was the one taken.
RISK-VALUE: DERIVED — SKILL.md net line delta = -1 (772 -> 771) @ 274ece128 — the stream spec section 3 row "skill and kit edits are net <= 0 lines" and the brief's weight line ("rule-text lines <= 0") set the bound, and hand row 6 measured it on the implementing commit. The offsets were restatements (see section 2), not operative rules.

Notes:
- BLOCKED (check-definition), not a product failure. The witness passes rows 1-8; row 9 fails as authored (N1), so `statusgen brief --check-verified` exits 1 and the brief is not flipped. Every row meets its stated Expect by hand, row 9 at the PR head and over the squash diff as its Expect prescribes. Advancing needs row 9 re-authored per N1 (routed to worker-desk), then a re-verify.
N1 (row 9 row-definition, not code). Row 9 cannot pass a post-merge witness as authored, for three reasons:
(a) On merged main there is no diff, so the inner statusgen returns COULD-NOT-CHECK exit 2.
(b) The trailing echo makes the shell exit 0 whatever statusgen returns.
(c) verifyrun reads its expected exit from the Expect cell's prose: code spans are stripped, and then "Exit 1 names the disproved claim" matches the exit regex (statusgen/verifyrun.go:349 and :383), so it expects 1.
The substance of the row holds. At the PR head (9a) and over the squash diff (9b), statusgen exits 0 with 3 corroborated and 0 disproved. A witness-fit replacement would pin the base and assert the count, for example: git checkout 274ece128 and run statusgen --consumers --brief build-less-brittle/02 --base 274ece128~1, expecting exit 0 and "3 corroborated, 0 disproved" (with the Expect prose free of any other "exit N" phrase).
N2 (lint on the witness tree, NOTICE only, lint exit 0, no PROBLEM line names this brief):
- gotest-run-vacuous on row 8. This is answered by the hand -v run showing "--- PASS: TestSharedValueTriggerIsNarrow".
- outcome-absent.
- Owed +dereference and +flow Verify-row obligations. Row 5 dereferences the contract id and 9a/9b corroborate across the three artifacts, but no row declares the tokens.
N3 (row 8 relevance). TestSharedValueTriggerIsNarrow does not feed a design-fit block itself. The extra probe (hand row 8) confirms the trigger stays silent on the actual design-fit blocks.
N4 (spec wording, minor). brief-v1 rule 1 says a second owner "is a finding", where the stream spec section 4.2 says "is rejected", and it omits "recorded in the rule register". This reads as a deliberate softening for the public spec. It is not a Verify failure, and it is for the reviewer or desk to weigh.
N5 (consumers out-of-scope entry). The downstream-copies entry stays UNCHECKED by design (adopters re-sync on their own pin bump).
N6 (witness hash drift). Row 8's hash differs between the dry run (3b4ca0716b1b) and the non-dry run (28ee1419f776) because go test prints its elapsed time. Both runs passed.
N7 (hygiene). The dispatched worktree was not written. All runs used throwaway --no-hardlinks clones, and the host hand clone is clean after the run. The witness clone carries only the appended Evidence, and the check-verified copy only its hypothetical README edit.

VERIFY: BLOCKED

## Review
Gate: model (from frontmatter). The reviewer checks that the skill's offsets removed narrative or
restatement, never an operative rule sentence.
