---
brief: assay:assay:build-less-brittle:14
title: Bind consumer acceptance obligations to immutable delivery provenance before post-merge dispatch
why: >-
  A brief's "consumers were updated" check is authored before the brief merges, so it cannot name
  the commit that will deliver it. After the merge the same check runs against main, finds no
  diff, and cannot pass. Twenty-odd rows across a dozen streams have failed this way, and each
  repair is a hand edit of one row that needs a person's sign-off. Binding the delivering change
  into the acceptance contract at merge time retires that recovery without weakening who may
  change an acceptance row.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-23 by the intake-desk authoring session (draft; routed by the driver's ruling below)
sources:
  - "https://github.com/medici-finance/assay/issues/3832#issuecomment-6366940786"
  - "medici-finance/assay#1915 (class tracker; ruling comment 6366941410 queues this design brief)"
  - "medici-finance/assay#3826 (strike-two design note this brief answers)"
  - "docs/consumer-delivery-checks.md (current derived-delivery path and its stated limit: the base is bound, the delivery is not)"
  - "docs/protected-paths.md §Explicit human-approved acceptance transitions (protected command-correction route)"
  - "docs/contracts.md R-consumer-delivery-range, S-evidence-coverage"
  - "build-less-brittle/05 (two-strikes: the second fix in a class becomes a design note)"
  - "freshness-checked 2026-10-23 @ 0221bf5c1 (origin/main)"
exec-tier: strong
exec-tier-why: "(a) the binding mechanism is not pre-specified; (b) correctness spans the checker, the protected-table evaluator and the verifier's independence; (c) a subtle error here either weakens protected-table authority or lets a wrong delivery earn credit; (d) raised by an error-class trigger (strike two)."
domain: complicated
outcome: none
---

# Brief 14 — Bind consumer acceptance obligations to immutable delivery provenance before post-merge dispatch

## Context
files:
- `docs/streams/build-less-brittle/design-delivery-binding.md` (planned) — the design record this brief delivers

facts:
- Class: a Verify row whose consumers check names no delivering change, so it is satisfiable
  only before the merge and can never pass on merged main. The shape every instance shares is
  "post-delivery obligation, no immutable delivery coordinate in the authored contract"; the
  reported rows (measured-status/02, desk-tools/37, desktools-v2/63, #3826, #4209 and the rest
  of `docs/consumer-delivery-checks.md` §Known #1915 instances) are instances.
- Root invariant (worker design note on #3826): every post-delivery consumer obligation binds
  the actual immutable delivering change and its exact first-parent range, keeps nonzero
  corroboration, and reports unchecked exclusions explicitly. A branch-relative or empty
  documentation diff cannot discharge it.
- Prior fixes (all per-row or per-instance): #1908 pinned build-less-brittle/02 row 9; #2300
  re-derived the stream's rows; the `consumer-delivery-range` lint
  (`statusgen/verifyrows.go:ruleConsumerRange`) flags unpinned rows; `--consumers --delivery
  <sha> --base <first-parent>` evaluates a historical delivery
  (`statusgen/consumerdelivery.go`); a bare `--consumers --brief <id>` on merged main derives
  the pin from the committed `Verification-Table:` marker
  (`statusgen/consumerderived_test.go`).
- Stated limit of the derived path (`docs/consumer-delivery-checks.md`): the base is bound and
  the delivery is not. The marker is written by whoever produced the Evidence, and a marker
  naming a later mainline commit is accepted. The delivery identity is still an independently
  reviewed claim.
- Protected-table authority is fixed input, not a design variable: an original row's Command,
  Expect, class, shell, order and obligations change only by the human-approved
  acceptance-transition route (`docs/protected-paths.md`). The ruling on the driver's
  decision keeps that route for #3826 and #4209 and asks for this design to stop the repeats.
- Overlap checked 2026-10-23: open draft PRs #4139 (validates prospective brief delivery before
  ready-to-merge) and #4213 (the #4209 row repair) touch adjacent seams; neither binds delivery
  into the authored contract. The design must say how it composes with #4139 and must not
  absorb #4213, which stays a protected command correction.
- single-point-of-failure: the delivery identity the checker trusts — the design must name who
  or what establishes it independently of the brief's author and of the checker, and what
  catches a wrong identity when that source is unavailable or wrong.

design-fit:
  owner: statusgen consumer-delivery checker (`statusgen/consumerdelivery.go`) with the protected-table evaluator (`statusgen/protectedclosure.go`)
  contract: S-evidence-coverage
  retires: []   # the design may nominate manual delivery-coordinate recovery for retirement; nothing is removed by this brief
  weight: verbs 0, flags 0, refusals 0, rule-text lines +0 (a design record, not rule text; every rule it proposes lands in a later brief that carries its own design-fit)
  why-add: n/a

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Design only. Do not edit `statusgen/`, `docs/protected-paths.md`, any brief's Verify table,
  Evidence or witnesses, and do not touch #3826 or #4209 rows — those go by the protected
  acceptance-transition route with the driver's approval of the exact payload.
- Do not weaken, bypass or add an override to protected-table authority, independent source
  identification, unavailable-is-not-pass, or the nonzero-corroboration requirement.

## Task
1. **Freshness.** On fresh `origin/main` re-read `docs/consumer-delivery-checks.md`, the
   `consumer-delivery-range` register row, and the derived-delivery tests named above; record
   the commit read. If a merged change already binds delivery into the authored contract,
   stop and report NEEDS_CONTEXT instead of designing.
2. **Instance census.** In the design record, tabulate the class instances still open or
   recently repaired (brief, row, how the delivery coordinate was missing or wrong, who recovered
   it, and by what route). Quote real commits; do not estimate.
3. **Alternatives, compared on one criterion.** Compare at least: (A) status quo — per-row
   pins, the lint and protected command correction; (B) a delivery-binding record written at
   merge by an identity that is neither the brief's author nor the checker, referenced
   symbolically from the row so the Command text carries no SHA; (C) derive the delivery from
   the git graph alone (the first mainline first-parent commit that adds the brief's declared
   files or carries its trailer), and state where trailers are absent or wrong (desk-tools/38
   carries a trailer naming desk-tools/37; measured-status/02 has none); (D) the committed
   `Verification-Table:` marker as the sole source (today's path). Criterion: which option
   keeps the delivery identity independent of both the author and the checker, fails closed when
   its source is unavailable, preserves nonzero corroboration and explicit unchecked
   exclusions, and removes the need for a per-brief protected amendment for briefs authored
   after it lands.
4. **Recommend one design.** State the binding point (when and by whom the coordinate is
   fixed), where it is stored, how a row refers to it, what the checker does when it is missing,
   ambiguous or contradicts the git graph (exit 2 could-not-check, never a pass), how an
   existing already-merged row is handled (it stays on the protected route), and how it composes
   with the open PR on prospective delivery validation named in Context.
5. **Layers.** Name at least two independent controls on the delivery identity (different
   components, different failure signals) and the one behind each, with the single point of
   failure answered honestly. A repeated check is one layer, not two.
6. **Decisions owed.** List every choice that touches protected-table authority or approval
   semantics as a question for the driver with options and a default. File each as a
   `needs-decision` issue through the desk's filing verb before this brief is handed to
   verification; do not resolve them in the record. In the record, write each owed decision
   as one line that carries the word `needs-decision` and its filed issue's full reference
   (`medici-finance/assay#N` or the issue URL), and use that word on no other line: Verify
   row 7 fails any `needs-decision` line without a filed-issue reference.
7. **Implementation briefs, not written here.** List the briefs the recommendation implies
   (title, owning module, depends, effort, risk answers, the Verify row that proves the lower
   layer with the upper one bypassed). Authoring them is a separate PR after the decisions in
   step 6 are settled. State the ordering in the record: Verify rows 4–5 pin today's
   derived-path behaviour, including that a marker naming a later mainline delivery is
   accepted, so this brief is verified before any implementation brief that changes that
   behaviour merges; after that, rows 4–5 going red is the expected effect of the successor,
   not a defect in this record.
8. Cite only public references in the record (no private tracker aliases). Describe long
   identifiers instead of quoting them.
9. Give the record these six level-2 headings, verbatim and each on its own line:
   `## Instance census`, `## Alternatives`, `## Recommended design`, `## Independent layers`,
   `## Decisions owed`, `## Implementation briefs` (Verify row 1 matches whole lines).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `grep -q -x -F '## Instance census' docs/streams/build-less-brittle/design-delivery-binding.md && grep -q -x -F '## Alternatives' docs/streams/build-less-brittle/design-delivery-binding.md && grep -q -x -F '## Recommended design' docs/streams/build-less-brittle/design-delivery-binding.md && grep -q -x -F '## Independent layers' docs/streams/build-less-brittle/design-delivery-binding.md && grep -q -x -F '## Decisions owed' docs/streams/build-less-brittle/design-delivery-binding.md && grep -q -x -F '## Implementation briefs' docs/streams/build-less-brittle/design-delivery-binding.md` | exit 0 — each of the six headings is present as a whole line of its own | check |
| 2 | `grep -c -E '^[0-9a-f]{40}$' docs/streams/build-less-brittle/design-delivery-binding.md` | output is at least 3 (the commits the census cites, one full id per line in a fenced block) | check |
| 3 | `test "$(grep -E '^[0-9a-f]{40}$' docs/streams/build-less-brittle/design-delivery-binding.md \| git cat-file --batch-check \| grep -c ' commit ')" = "$(grep -c -E '^[0-9a-f]{40}$' docs/streams/build-less-brittle/design-delivery-binding.md)"` | exit 0 — every commit the record cites resolves to a commit object in the repository | check +dereference |
| 4 | `cd statusgen && go test . -run 'TestDerivedDeliveryPinsRecordedDeliveryParent$' -count=1 -v \| grep -c -E '^--- PASS: TestDerivedDeliveryPinsRecordedDeliveryParent \('` | output is 1 — the derived-path behaviour the record builds on still holds as described (the anchored pattern counts the top-level result line only, never a subtest's) | check |
| 5 | `cd statusgen && go test . -run 'TestDerivedDeliveryCouldNotCheck$' -count=1 -v \| grep -c -E '^--- PASS: TestDerivedDeliveryCouldNotCheck \('` | output is 1 — the derived path still refuses to pass when its source is unavailable (the anchored pattern counts the top-level result line only, never a subtest's) | check |
| 6 | `statusgen --root . --lint` | exit 0; no PROBLEM line names `build-less-brittle/14` | check |
| 7 | `test "$(grep -c -F 'needs-decision' docs/streams/build-less-brittle/design-delivery-binding.md)" -ge 1 && ! grep -F 'needs-decision' docs/streams/build-less-brittle/design-delivery-binding.md \| grep -q -v -e 'https://github\.com/medici-finance/assay/issues/[0-9]' -e 'medici-finance/assay#[0-9]'` | exit 0 — at least one owed decision is recorded, and every `needs-decision` line names its filed issue by full reference | check |

<!--
Pre-mortem → detection map (author-brief B5)
| Failure mode of the work | Caught by |
|---|---|
| Record is well-formed but cites commits that do not exist or are not the deliveries claimed | row 3 (existence); row 3 cannot check "is the real delivery" — review-only, and the record must say so |
| Record's description of today's derived path is stale or wrong | rows 4–5 |
| An implementation brief that changes the derived path merges before this brief is verified | rows 4–5 go red legitimately; the record states the ordering (Task step 7) and the verifier runs this brief first |
| Design quietly weakens protected-table authority or adds an override | review-only (adequacy); Ground rules forbid it and the design-fit reviewer answers both core-system questions |
| Record resolves a protected-authority choice itself instead of escalating | row 7 (every `needs-decision` line names a filed issue); adequacy review-only |
| Section headings present but empty | review-only; rows 1-2 only prove shape |
-->

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table. The reviewer answers both design-fit questions in the verdict: (1) what single control stands between a wrong delivery identity and a false pass, and is that acceptable; (2) does any Verify row in a downstream brief prove a lower layer catches a wrong identity with the upper layer bypassed.
