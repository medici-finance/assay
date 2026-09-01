---
brief: harness-portability/13
title: Cursor live-desk-smoke protocol + first run
why: >-
  Brief 12 built the structural Cursor column (binding file, generator verb, generated
  packaging, adopt-flow section) but explicitly scoped the actual proof out as "a separate
  gate:human acceptance step" mirroring the Codex stream's live smoke (HP/07). Without this
  brief nothing in the stream ever runs a full desk loop — dispatch, isolation, a draft PR,
  a review cycle — on a real Cursor install; every "Assay runs natively on Cursor" claim
  stays a documentary assertion, exactly the vacuous-green failure the Codex chain refused
  to accept at HP/07.
wave: 6
depends: ["harness-portability/12"]
unblocks: []
effort: M
gate: human
gate-why: >-
  The acceptance evidence is a live session on a real Cursor install running a FULL desk
  loop (dispatch through to a draft PR and one review cycle), not a single skill
  invocation — an environment (Cursor account/install) only Ian provides or sanctions. The
  human is confirming the loop actually ran, that the degradations observed match brief
  12's ruled matrix, and that the transcript/PR evidence is genuine — the one claim no
  in-repo command can corroborate.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-01 by harness-portability gap-check dispatch (assay-worker-app)
sources: ["brief-12-cursor-third-column.md §\"The one gate:model open item + the gate:human acceptance step\": \"The parity acceptance is a live session on Cursor ... exactly the posture the Codex stream held for its live smoke (HP/07)\" — read in full 2026-09-01, no owning brief exists for that acceptance step", "brief-12's own 11-row Verify table (read in full 2026-09-01): every row is offline/structural (go test, harnessgen --check, mutation tests, freshness) — none executes a live Cursor session or a desk-loop dispatch; row 10 only checks that live-only capability rows STAY FLAGGED, not that they were confirmed", "brief-07-adoption-live-smoke.md (read in full 2026-09-01): the precedent this brief mirrors — protocol doc + adoption-docs runbook + a committed, evidence-bearing run log, gate:human, live-run row explicitly BLOCKED until the human runs it", "docs/adopting-assay.md (read 2026-09-01, ~L773-828): the Cursor section already ends in one unlinked line — \"The one acceptance step is a live smoke run on a Cursor install...\" — this brief is what makes that sentence true", "docs/streams/harness-portability/README.md cross-repo table: \"Live Cursor environment (install) | Head for 12 | Blocks 12's live-confirm rows and its parity smoke; Ian provides\" — the parity-smoke dependency is named but had no owning brief before this one", "freshness-checked 2026-09-01 @ current branch tip: no docs/cursor-smoke-protocol.md, no docs/cursor-smoke-runs/, no PARITY.md/RELEASE-NOTES.md Cursor-smoke entry exist anywhere in the repo (grep + find, clean)"]
consumers: ["docs/adopting-assay.md: fixed-here (the one-line acceptance mention gets a concrete reference)", "docs/cursor-smoke-protocol.md: follow-up (lands with the sequenced tool/method-text de-house — same posture as docs/codex-smoke-protocol.md, per the stream's re-home note)", "the bundle's PARITY.md/RELEASE-NOTES.md: follow-up (same de-house sequencing; version bump + third-harness-smoke record)", "docs/streams/harness-portability/README.md: fixed-here (status row, wave, gate distribution, cross-repo table)"]
exec-tier: strong
exec-tier-why: >-
  (b) cross-artifact correctness — every protocol step must faithfully mirror brief 12's
  ruled degradation matrix (plugins/assay/references/cursor.md) and the two-surface ruling
  (headless-first, IDE secondary); a protocol that drifts from the binding it is supposed
  to prove is exactly the failure class this stream exists to prevent.
---

# Brief 13 — Cursor live-desk-smoke protocol + first run

> **Note (re-home / de-house).** Mirroring HP/07's own note: `docs/cursor-smoke-protocol.md`
> and the bundle's `PARITY.md`/`RELEASE-NOTES.md` are method-text-layer/bundle artifacts
> that land with the sequenced tool/method-text de-house (the same follow-on `tools/harnessgen`
> and the generated `plugins/assay/cursor/` packaging are waiting on, per brief 12's own
> "Tool de-house note"). `docs/adopting-assay.md` and this stream's README are already public
> and are amended directly by this brief. Until the de-house lands, this brief's protocol-doc
> Verify rows run in the tool's source tree, exactly as brief 12's generator Verify rows do.

## Context

files:
- **create** `docs/cursor-smoke-protocol.md` (planned; de-housed — see note above) — the
  scripted live-harness checklist, numbered steps each with a literal action and an
  `Expect:` line, mirroring HP/07's shape and minimum coverage, adapted to Cursor's ruled
  two-surface posture (headless `cursor-agent` primary, in-editor IDE agent secondary) and
  extended with the step this brief exists to add: an actual **full desk loop**, not only
  per-skill invocation.
- **amend** `docs/adopting-assay.md` (~L773–828, the "Running Assay on Cursor" section) —
  replace the current unlinked sentence ("The one acceptance step is a live smoke run on a
  Cursor install...") with a concrete reference to the protocol doc, matching the fuller
  runbook treatment the Codex path already gets in the same file.
- **amend** (planned; de-housed) the bundle's `PARITY.md` / `RELEASE-NOTES.md` — version
  bump recording the Cursor live-desk-smoke as the third-harness parity milestone (mirrors
  HP/07 task 4, one harness later).
- **amend** `docs/streams/harness-portability/README.md` — this brief's status-table row
  (wave 6, depends on 12), the gate-distribution line, the cross-repo dependency table's
  Cursor-environment row, and a "Note on 13" paragraph beside the existing "Note on 07".

out-of-repo files: none

facts:
- **Brief 12 delivers structure, not proof.** It ships the Cursor binding file, the
  `harnessgen cursor` verb, the generated `.mdc` rule, and the adopt-flow section — all
  proven by offline/structural Verify rows (go test, `--check`, mutation tests). It never
  runs a live Cursor session and never exercises a desk-loop dispatch; its own Context
  section calls the live smoke "a separate gate:human acceptance step ... exactly the
  posture the Codex stream held for its live smoke (HP/07)" and defers to it without
  naming an owning brief.
- **"Run a full desk loop" is a strictly bigger claim than HP/07's own smoke steps.**
  HP/07's seven steps (install, resident-rules-present, invoke-each-skill,
  auto-trigger-probe, dispatch-probe, isolation-probe, evidence-probe) prove *skills are
  reachable and degrade correctly* — they never dispatch a worker through to a draft PR and
  a review cycle. This brief's protocol must add that eighth step explicitly, or it inherits
  the same gap it exists to close.
- **The Codex precedent (HP/07) is the shape to mirror, not re-invent**: a scripted
  protocol with per-step `Expect:` lines, a committed run log carrying transcript excerpts
  per step, a `Result: PASS/FAIL/BLOCKED` per step with FAIL routed to a filed issue, and a
  Verify table whose acceptance row stays `BLOCKED (needs live Cursor + Ian)` until the
  human runs it — greening that row from the protocol text alone is the exact
  vacuous-green failure the stream's own conventions forbid.
- **`docs/adopting-assay.md` already carries the landing spot.** The Cursor section exists
  (landed with 12) and already ends in the sentence this brief makes true; no new section
  needs authoring there, only a concrete reference in place of the unlinked mention.

## Human decision

This asks for a live session on a real Cursor install that runs a complete dispatch loop —
not just installing the method and invoking one skill, but dispatching a worker through to
an opened draft pull request and one review cycle — against an actual repository, with the
observed behavior checked against an already-published degradation ruling. The structural
groundwork for a third supported agent harness is finished and passes every check that can
run without that install; the only thing standing between "the structure is built" and "the
claim is true" is this one live run, which needs an account and installed software that only
the project owner can provide or approve, plus that owner's own sign-off that what the
session did and observed matches what was promised.

Options:
1. **Run it now** — provide or sanction a Cursor install for a live session in the near
   term (a specific week), attend or delegate attendance, and sign off on the resulting
   run log once it lands.
2. **Defer to a named later date** — the structural work stands as `implemented` with the
   acceptance step explicitly held open; set a target window and revisit then.
3. **Decline for now** — leave the acceptance step open indefinitely; the stream continues
   to state, honestly, that the third-harness claim is unproven pending this step, with no
   target date.

Default if no answer: option 2, revisited in 30 days — the acceptance step stays open and
clearly labeled rather than silently dropped.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Draft PR only.
- Stop at `implemented` — do not set verified/done.
- Do NOT fabricate the live run's evidence. A step with no transcript excerpt is not
  evidence; a run log entry asserting `PASS` with nothing pasted is not a result.
- The live run happens on an environment Ian provides/sanctions — never self-procured.
- Path-specific `git add`; never commit STATUS.md.

## Task

1. Write `docs/cursor-smoke-protocol.md` (planned; de-housed) — numbered steps, each with a
   literal action and an `Expect:` line. Minimum coverage, mirroring HP/07 adapted to
   Cursor's ruled two-surface posture:
   1. Fresh install per the adopt path's Cursor scenario (brief 12 §2c).
   2. Session start on BOTH surfaces → resident rules present without manual pasting
      (probe: headless reads `AGENTS.md` natively; IDE surface reads the generated
      `.cursor/rules/assay.mdc` always-apply rule — ask each session to state rule 3's
      neutral-dispatch wording).
   3. Invoke each of the seven skills by name on the headless `cursor-agent` surface →
      body loads.
   4. Auto-trigger probe on the IDE surface, per brief 12's ruled matrix.
   5. Dispatch probe → `runs`/`degrades`/`refuses` observed matches brief 12's ruled
      matrix exactly (worker-desk **runs** on the IDE surface; headless it **runs-or-
      refuses** on a live-confirm of `git worktree add` permission) — degradation STATED
      by the session, never silent.
   6. Isolation probe → refusal fires where ruled (a headless sandbox without worktree
      permission refuses the fanout rather than working in a shared checkout).
   7. Evidence-discipline probe → a Verify row executed and recorded by the session.
   8. **Full desk loop (the step HP/07's shape does not have and this brief exists to
      add)**: on the headless surface, against a real (non-toy) repository, dispatch one
      small worker-desk item end to end — implementer commits, opens a draft PR — then run
      one `pr-review-desk` cycle against it. `Expect:` a draft PR URL/number exists, carries
      a real diff, and received one reviewer verdict, with the isolation and evidence
      floors from steps 5–7 holding throughout, not re-relaxed for this step.
2. Amend `docs/adopting-assay.md`'s Cursor section: replace the current unlinked
   acceptance-step sentence with a reference to the protocol doc.
3. Amend the bundle's `PARITY.md` / `RELEASE-NOTES.md` (planned; de-housed): version bump
   + record the Cursor live-desk-smoke as the third-harness parity milestone.
4. Coordinate the first live run (Ian, or a session with the environment Ian sanctions);
   commit the run log under `docs/cursor-smoke-runs/<date>-<cursor-version>.md` (planned;
   de-housed, same posture as `docs/codex-smoke-runs/`); file issues for any failed step.
5. Add this brief's row + supporting README edits (status table, gate distribution,
   cross-repo table, "Note on 13") to `docs/streams/harness-portability/README.md`.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/cursor-smoke-protocol.md; echo $?` | `0` — run in the de-housed tool source tree per the re-home note (same posture as HP/07 row 1) |
| 2 | Every step has an Expect, and the full-desk-loop step is present: `s=$(grep -cE '^#### Step [0-9]+' docs/cursor-smoke-protocol.md); e=$(grep -cE '^Expect:' docs/cursor-smoke-protocol.md); echo "$s $e"; test "$s" -ge 8 -a "$s" -eq "$e"; echo $?` | `0` — at least the eight minimum steps (HP/07's seven plus the full-desk-loop step), step-count equals Expect-count |
| 3 | **Dereferencing (rule 11) — the desk-loop step is real, not vacuous**: `grep -A3 -E '^#### Step 8' docs/cursor-smoke-protocol.md \| grep -qiE 'draft PR' && grep -A3 -E '^#### Step 8' docs/cursor-smoke-protocol.md \| grep -qiE 'pr-review-desk\|review'; echo $?` | `0` — step 8's own Expect line names a draft PR AND a review cycle, not just "invoked a skill"; a protocol that only re-tests per-skill invocation (HP/07's shape unmodified) fails this row |
| 3a | **Positive control for row 3**: `grep -qF 'draft-PR-no-such-token' docs/cursor-smoke-protocol.md; echo $?` | `1` |
| 4 | `docs/adopting-assay.md` no longer carries the bare unlinked mention: `grep -qF 'cursor-smoke-protocol.md' docs/adopting-assay.md; echo $?` | `0` |
| 4a | **Positive control for row 4**: `grep -qF 'cursor-smoke-protocol-no-such-file.md' docs/adopting-assay.md; echo $?` | `1` |
| 5 | Version bumped past what `origin/main` already carried when this brief started: `base=$(git show origin/main:plugins/assay/.claude-plugin/plugin.json \| jq -r .version); cur=$(jq -r .version plugins/assay/.claude-plugin/plugin.json); test "$cur" != "$base"; echo $?` | `0` |
| 6 | The CURRENT version's own RELEASE-NOTES section names Cursor's live smoke: `V=$(jq -r .version plugins/assay/.claude-plugin/plugin.json); awk -v v="$V" '/^## v/{p = ($0 ~ ("v" v))} p' plugins/assay/RELEASE-NOTES.md \| grep -qiE 'cursor.*smoke\|smoke.*cursor'; echo $?` | `0` — scoped to the section headed by the CURRENT version, not a whole-file grep (mirrors HP/07 row 6's guard against 12's own version-bump entry passing this vacuously) |
| 7 | `docs/streams/harness-portability/README.md` carries this brief's row + the updated gate-distribution line: `grep -qF '13' docs/streams/harness-portability/README.md && grep -qE '03,? 07,? and 13 are.*gate: human\|03.*07.*13.*human' docs/streams/harness-portability/README.md; echo $?` | `0` |
| 8 | **BLOCKED (needs live Cursor + Ian)** — the first run log exists and is complete: `ls docs/cursor-smoke-runs/ && f=$(ls docs/cursor-smoke-runs/ \| head -1) && grep -cE -e '^Result: PASS' -e '^Result: FAIL' -e '^Result: BLOCKED' "docs/cursor-smoke-runs/$f"` | count equals the protocol's step count, every step carries a `Result:`, and step 8 (the full desk loop) carries a real draft-PR URL/number in its transcript excerpt. Until the environment exists this row is BLOCKED — it is this brief's acceptance row and is never greened from the protocol text alone |

## Evidence

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s), date, runner). Row 8 is the acceptance evidence:
     the run log path, the Cursor version, who ran it, and the desk-loop's draft-PR
     reference. BLOCKED until the live environment exists.
     "verified" requires a non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|

## Review

Gate: **human** (from frontmatter). The human signs the run log: a real Cursor install, the
listed version, transcript excerpts present per step, a genuine draft-PR reference for the
full-desk-loop step, degradations observed matching brief 12's ruled matrix, failures routed
to issues. Rows 1–7 green with row 8 BLOCKED means this brief is NOT done — that state is
"ready for the live run," and saying otherwise is the vacuous-green failure this stream's
bar forbids (the same reading HP/07's own Review section states for its row 7).
