---
brief: assay:assay:build-less-brittle:05
title: "Worker two-strikes: the second fix in a class becomes a design note; PR bodies report measured weight"
why: >-
  A worker dispatched on a symptom fixes the symptom, even when the same mechanism was patched
  last week. Each fix then pins itself in place with a fail-first test and, under today's
  defect-class clause, a new guard. Stopping at the second fix in a class, and asking the worker
  for a short design note naming the root invariant, turns repeated patching into one design
  decision. Reporting measured weight in every PR makes additions visible where they are made.
wave: 3
depends: ["build-less-brittle/03", "build-less-brittle/04"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format)"
sources:
  - "docs/streams/build-less-brittle/spec.md §2 D1/D3/D5, §3 rows 3 and 8, §4.3"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md clauses 8 (reuse ladder) and 14 (defect class)"
  - "tools/desk/cmd/deskdispatch/kitparity_test.go:44 (TestDefectClassClauseIsOneWordingAcrossImplementerKits)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: no strike, design-note or Weight section in either implementer kit; clause 14 instructs adding a class guard"
exec-tier: strong
exec-tier-why: "(b) the same clause wording must land byte-identical in two kits under a parity test, and the worker-desk re-dispatch tier rule must agree with it."
domain: complicated
consumers:
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md: follow-up build-less-brittle/05 (this brief)"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md: follow-up build-less-brittle/05 (this brief)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up build-less-brittle/05 (this brief)"
  - "installed deskdispatch binaries (kits are embedded): out-of-scope (reach consumers on the next desk-tools release and pin bump)"
---

# Brief 05 — Worker two-strikes and the measured Weight report

## Context

files:
- `tools/desk/cmd/deskdispatch/references/worker-prompt.md`: clause 8 (scope and reporting), clause 14 (defect class).
- `tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md`: its verbatim copies of both.
- `plugins/assay/skills/worker-desk/SKILL.md`: re-dispatch tier; routing of a returned design note.
- `tools/desk/cmd/deskdispatch/kitparity_test.go`: its required-token list for clause 14.
- `changelog/build-less-brittle-05.md` (planned)

facts:
- The objective kit carries every load-bearing worker clause **verbatim**.
  `TestDefectClassClauseIsOneWordingAcrossImplementerKits` (kitparity_test.go:44) holds clause 14
  identical across the kits, and `TestKitsCarryNoPrivateReferences` (kittext_test.go:109)
  forbids private references in any kit.
- **The guard obligation is itself pinned by a test.** The parity test requires clause 14's body
  in both kits to contain `## Defect class`, `ALLOW-LIST`, `PLANTED SECOND` and `## Fail-first`
  (kitparity_test.go, the `must` list). `ALLOW-LIST` is the add-a-guard obligation this brief
  retires, so the token list changes with it: `ALLOW-LIST` is replaced by `unrepresentable`.
  This is the spec's D3 dynamic in miniature: a test entrenching a patch. Retiring the
  obligation retires the assertion, and the other three tokens stay.
- Clause 8's reuse ladder says "a briefed deliverable is never re-litigated as YAGNI". Strike two
  is the one carve-out.
- Strike detection needs no new tool. The item's `error-class` issue (brief 04) lists attached
  fixes. An item with no class issue: the worker searches the tracker for an open
  `error-class` issue naming the mechanism before coding.
- The Weight line comes from brief 03:
  `cd tools/desk && go test ./internal/weight/ -run TestPrintWeight -count=1 -v -args -rev=<sha>`,
  run once at the merge-base and once at the head, plus `git diff --shortstat <merge-base>...HEAD`.
  In a repository without the counter, the section says `could-not-check (no weight counter)`.
- The bleed exception (spec §10 D-B, **ruled 2026-09-24**): two-strikes is a **hard stop**. A
  `bleed` reply by the driver on the class issue lets a production-down or security fix
  proceed, and nothing else does; the class stays `design-owed`. The kit text carries exactly
  that form.
- Line counts at f7bde6bfa: worker-prompt.md 358, worker-prompt-objective.md 509, worker-desk 868.

design-fit:
  owner: tools/desk/cmd/deskdispatch/references/worker-prompt.md (the implementer contract)
  contract: none — dispatch kits have no semantic-owner row
  retires: ["clause 14's default of adding a class guard (replaced by: remove the hazard, or make it unrepresentable; a guard is weight)", "kitparity_test.go's ALLOW-LIST required token (replaced by unrepresentable)"]
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 in each of the three files
  why-add: n/a

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Kit wording that the parity test pins must change in both kits in the same commit.
- Each file is net ≤ 0 lines. Clause 14's ~45 lines hold worked examples that can be tightened.
  Never drop its "name the class" or "fail-first against a planted second instance" obligations.

## Task

1. **Clause 8, strike two** (≤ 8 lines, in both kits). Before coding a defect fix, check the
   item's class record. If it already records a merged fix, STOP. Post a design note on the
   class issue with `deskfile attach`, carrying: root invariant; the owner it should live in
   (a semantic-owner row, or `unknown`); what the prior fixes added that a design would retire;
   a proposed design-brief title. Report `NEEDS_CONTEXT: strike two — design note posted`.
   The exception is a driver `bleed` reply on the class issue.
2. **Clause 8, Weight** (≤ 4 lines, both kits). Every PR body carries `## Weight`: the two counter
   lines and the shortstat. A positive delta in any ratcheted dimension also carries
   `why-add:`. The section is a material claim: a wrong line is a review finding.
3. **Clause 14 amended** (both kits, identical). Keep obligations 1 (name the class) and 3
   (fail-first against a planted second instance). Obligation 2 becomes: "Close the class by
   REMOVING the hazardous path or making it unrepresentable (a type, a single choke point).
   Only when removal is infeasible, add a guard, and report it as weight in `## Weight` and as
   a rule-register row."
4. **worker-desk** (≤ 4 lines). A re-dispatch or shepherd pass on a PR whose open finding class
   is at round ≥ 2 runs at strong tier. A worker's `strike two` NEEDS_CONTEXT returns the item
   to intake as `design-owed`. It is not a failure to retry.
5. In `kitparity_test.go`, replace `ALLOW-LIST` with `unrepresentable` in the `must` list, and
   update the file's header comment ("guard the class" becomes "close the class").
6. Offset lines in each file, run the deskdispatch tests, and write the changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. Row 2 is the neighbour/guard row: the parity
test still holds. Row 3 is the mutation row for that guard. Rows 7–9 are net ≤ 0 weight rows. Row 10 checks the pinned token moved.

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskdispatch/ -count=1` | `ok` |
| 2 | `cd tools/desk && go test ./cmd/deskdispatch/ -run TestDefectClassClauseIsOneWordingAcrossImplementerKits -count=1 -v` | `--- PASS` |
| 3 | `cd tools/desk && f=cmd/deskdispatch/references/worker-prompt-objective.md && cp "$f" /tmp/bl05-kit.bak && sed 's/unrepresentable/unrepresentible/' /tmp/bl05-kit.bak > "$f" && go test ./cmd/deskdispatch/ -run TestDefectClassClauseIsOneWordingAcrossImplementerKits -count=1 > /tmp/bl05-mut.out 2>&1; rc=$?; cp /tmp/bl05-kit.bak "$f"; test $rc -ne 0 && echo RED-ON-DRIFT` | `RED-ON-DRIFT` (the parity test catches a one-word drift in the amended clause) |
| 4 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -ciE 'strike two' "$f"; done \| grep -c '^[1-9]'` | `2` |
| 5 | `for f in tools/desk/cmd/deskdispatch/references/worker-prompt.md tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md; do grep -c '## Weight' "$f"; done \| grep -c '^[1-9]'` | `2` |
| 6 | `grep -c 'go test ./internal/weight/ -run TestPrintWeight' tools/desk/cmd/deskdispatch/references/worker-prompt.md && test -f tools/desk/internal/weight/weight_test.go && echo COUNTER-EXISTS` | a count ≥ `1`, then `COUNTER-EXISTS` (dereference: the command the kit tells workers to run exists) |
| 7 | `test "$(wc -l < tools/desk/cmd/deskdispatch/references/worker-prompt.md)" -le 358 && echo NET-OK` | `NET-OK` |
| 8 | `test "$(wc -l < tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md)" -le 509 && echo NET-OK` | `NET-OK` |
| 9 | `test "$(wc -l < plugins/assay/skills/worker-desk/SKILL.md)" -le 868 && echo NET-OK` | `NET-OK` |
| 10 | `f=tools/desk/cmd/deskdispatch/kitparity_test.go; grep -q '"unrepresentable"' "$f" && ! grep -q '"ALLOW-LIST"' "$f" && echo MOVED` | `MOVED` (the pinned token moved with the retired obligation) |
| 11 | `statusgen --consumers --root . --brief build-less-brittle/05; echo "exit=$?"` | `exit=0` at the PR head (no `consumers:` routing claim is disproved by the diff; the implementer replaces each self-routed entry with `fixed-here` in the same change). Exit 1 names the disproved claim |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). The reviewer confirms the clause 14 rewrite keeps the planted-
second-instance fail-first obligation. Losing it would quietly weaken the class discipline this
stream relies on.
