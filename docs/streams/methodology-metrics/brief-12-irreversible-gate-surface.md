---
brief: methodology-metrics/12
title: Surface the verified-stage human gate — irreversible briefs get a sign-off issue at implemented (#231)
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [231]
schema: brief-v1
authored: 2026-07-10 by Fable desk session (issue #231 investigation, claims verified against code)
sources: ["issue #231 (verify-desk report; all claims code-verified 2026-07-10: verifyissues.go:222/:381, brieffile.go:439-446)", "ledger-hardening/06 (the orphaned concrete instance)", "#171 verify-gate design (two-touch model, human-not-bot closer)", "freshness-checked 2026-07-10 @ fb9223ce"]
why: >-
  The verify-gate machinery fails inversely to risk: routine gate:human briefs auto-surface a
  sign-off issue at verified, but irreversible briefs — the funds/no-undo class the gates
  exist for — cannot reach verified without the human touch, and no issue opens before
  verified. They orphan silently at implemented (ledger-hardening/06 has sat there since
  2026-07-09 with a model VERIFY: PASS and no surface prompting human:<name>).
---

# Brief 12 — Surface the verified-stage human gate for irreversible briefs

## Context
files: `../assay-toolkit/statusgen/verifyissues.go` (`verifyIssues` eligibility, `closeVerify` state
machine), `../assay-toolkit/statusgen/verifyissues_test.go`; `../oit/.github/workflows/verify-gate-open.yml` /
`verify-gate-close.yml` expected UNCHANGED (statusgen owns the rule — verify, don't assume)
facts:
- Bug (code-verified): `verifyIssues()` emits only rows with `Status == "verified"`
  (verifyissues.go:222); `closeVerify()` refuses any row not `verified` (:381); the
  irreversible lint (brieffile.go:439-446) blocks `verified` without `human:<name>` in
  Reviewed. Net: `gate: human` + `irreversible: yes` briefs orphan at `implemented` with no
  issue — the chicken-and-egg #231 documents.
- Fix shape (#231 option 1, desk-endorsed over the two-issue variant, which is ceremony —
  the second card would open seconds after the first close with identical content):
  - `verifyIssues()` ALSO emits for briefs that are `gate: human` AND `risk.irreversible ==
    yes` AND `Status == "implemented"` AND whose Evidence records a model verify pass —
    match the `**VERIFY: PASS**` marker convention (see ledger-hardening/06 Evidence for the
    live shape); no marker → not eligible (an unverified implemented row must NOT surface).
  - The emitted body is DISTINGUISHED from the ordinary done-close card: states that closing
    advances `implemented → verified → done` in one step; renders the brief's `gate-why`
    verbatim (as the ordinary card does); and — REQUIRED — renders any Evidence rows marked
    UNRUN/deferred prominently under an explicit heading ("closing accepts these as
    deferred, or run them first"): lh/06's live mutating-deposit row is the live example.
    An uninformed rubber-stamp is the failure mode this card must not enable.
  - `closeVerify()` accepts the `implemented` case when (and only when) the same
    eligibility holds, and advances in one step: Verified cell stamped from the recorded
    model verifier + its date (from Evidence), Reviewed cell stamped `human:<closer>` +
    close date — satisfying the irreversible lint in the same write. The existing
    `verified`-state path is unchanged; anything else still refuses with the current error.
- Idempotency/marker conventions, the human-not-bot `ALLOWED_CLOSERS` guard, and the
  single-writer STATUS.md rule all carry over unchanged from #171's design.
- **Trade-off section (human:<name>, 2026-07-10 — the #266 bar, applies to BOTH card types this
  machinery emits):** every gate:human card body carries a **"Why we want it / What it
  limits"** analysis at the level of issue #266's comment (finance/usability consequences,
  the knobs it forces, what's deferrable vs existential) — drafted by the implementer at
  `implemented`-time (a `## Trade-offs` section in the brief or its PR body; the card
  renders it verbatim, like gate-why). A card whose trade-off section is missing renders
  a visible `TRADE-OFFS: not provided` line rather than omitting silently — the human
  should see the gap, not be spared it. Exemplar: issue #266's analysis comment.
- **Prior-state remediation prompt (human:<name>, 2026-07-10 — REQUIRED on every money-path card):**
  the card explicitly asks **"does this fix leave prior corrupted state that needs
  remediation?"** and the trade-off section must answer it (yes+what / no+why). This
  surfaced as a recurring gap across four money-path gates in one day (#283 bad strikes,
  #267 memory-store intents, #293 phantom-capital MM book, #311 already-settled vaults) —
  every one leaves potentially-wrong prior ledger/book state that its Verify table, which
  only checks NEW behavior, does not ask about. A `done` sign-off that certifies "new
  settlements conserve" while a running vault's books are already wrong is a false green.
  The prompt fires for any brief touching funds/settlement/agent-capital state (reuse the
  methodology/31 shared-money-surface trigger); an unanswered prompt renders
  `PRIOR-STATE: unassessed` on the card, blocking a clean sign-off the same way a missing
  trade-off section does.
- Interim workaround (record in #231 when this lands, and it remains valid until then):
  human:<name> may manually flip lh/06 `implemented → verified` with `Reviewed: human:ian`, which
  then auto-mints the normal done-close issue.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state (e.g. the VERIFY: PASS marker has more
  shapes in the wild than lh/06's): report NEEDS_CONTEXT, don't guess.

## Task
1. Extend `verifyIssues()` eligibility + body per facts; the new card's title/labels must
   remain compatible with `verify-gate-open.yml`'s existing parsing (verify by reading the
   workflow, not assuming).
2. Extend `closeVerify()` with the one-step advance per facts; refusal messages for every
   non-eligible shape (implemented without marker; implemented without irreversible;
   anything else non-verified) stay explicit.
3. Tests (`verifyissues_test.go`): irreversible-at-implemented WITH marker → emitted, and
   close advances with both stamps; WITHOUT marker → not emitted, close refuses;
   non-irreversible at implemented → never emitted; existing verified-path cases unchanged;
   UNRUN rows render in the body.
4. Sweep for other orphans: list every current `gate: human` + `irreversible: yes` brief at
   `implemented` in the PR description (lh/06 is known; find any siblings — class-sweep
   rule, methodology/20).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes every case in Task 3 |
| 2 | `statusgen --root . --verify-issues \| grep -c "ledger-hardening/06"` | ≥1 (the orphan surfaces) — if lh/06 was manually advanced meanwhile, substitute the Task-4 sweep's fixture test |
| 3 | `statusgen --root . --verify-issues \| grep -A5 "ledger-hardening/06"` | body shows implemented→verified→done language AND the UNRUN row rendering |
| 4 | `go vet ./tools/statusgen/...` | exit 0 |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

### Non-implementer verifier run — VERIFY: PASS (glm-5.2-verifier, in-repo main `c15dd509`, impl `37b7366b` / PR #700, 2026-07-18)

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok …/statusgen 2.136s`; Task-3 cases present (vg/09 irreversible+implemented+`**VERIFY: PASS**` → emitted ✓; vg/10 no-marker → not emitted ✓; vg/04 empty-Evidence → not emitted ✓; body asserts title / one-step language / UNRUN heading+text ✓; `TestCloseVerifyIrreversibleAdvance` stamps both Verified+Reviewed ✓) |
| 2 | `--verify-issues \| grep -c "ledger-hardening/06"` | 0 | lh/06 already advanced (commit `37b7366b`) → Row-2 substitution clause fires; substitute **assay-product/04** count = 4 (≥1 ✓; README row 42 = `implemented` → eligible); methodology/11 count = 0 (README = `todo` → correctly skipped) |
| 3 | `--verify-issues \| grep -A5 "ledger-hardening/06"` | 0 | lh/06 advanced → live substitute = assay-product/04: irreversible heading ✓, one-step `implemented → verified → done` ✓, gate-why verbatim ✓, `TRADE-OFFS`/`PRIOR-STATE` gaps rendered ✓, before-you-close checklist ✓. UNRUN-row rendering demonstrated by the vg/09 fixture (assay-product/04 has no UNRUN rows) — sanctioned by the Row-2 substitution clause |
| 4 | `go vet ./tools/statusgen/...` | 0 | clean |
| 5 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) |

All 5 rows meet expectations. `gate: model`, all four risks `no` → model flip permitted → `implemented → verified`.

## Review
Gate: model. Reviewer confirms (a) the one-step advance stamps BOTH cells such that the
irreversible lint passes on the resulting tree, (b) an implemented row WITHOUT a recorded
model pass can never surface or close, (c) UNRUN rows are unmissable in the card body,
(d) the workflows genuinely needed no change (or the diff includes them with reason).
