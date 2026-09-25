---
brief: assay:assay:build-less-brittle:04
title: Intake files by error class; a recurring class triggers a design brief, not another point fix
why: >-
  The front door files one issue per symptom ("X refused Y"), and a worker fixes exactly that
  symptom. The same mechanism therefore gets patched where it surfaced, again and again: one
  credential chain went from a missing token to a shim, to a precedence bug, to a wrapper plus
  an identity probe. Filing symptoms against a class record, and switching the class to a
  design brief once it recurs, puts the unit of work at the mechanism instead of the symptom.
wave: 2
depends: ["build-less-brittle/02"]
unblocks: ["build-less-brittle/05", "build-less-brittle/09"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format)"
sources:
  - "docs/streams/build-less-brittle/spec.md §2 D1, §3 rows 1 and 8, §4.1"
  - "tools/desk/cmd/deskfile/deskfile.go (attach verb; new-issue budget)"
  - "statusgen/nextup.go (dispatch reads only `todo` rows) and statusgen/checks.go (`blocked` is a valid status)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: intake-desk SKILL.md has no class, attach-to-class or design trigger step; pr-review-desk's recurrence-promotion routes to a guardrail"
exec-tier: strong
exec-tier-why: "(b) one procedure spans three skills (intake, review, worker dispatch) and must reuse existing verbs, labels and status tokens without inventing any."
domain: complicated
consumers:
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up build-less-brittle/04 (this brief)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md (Recurrence-promotion paragraph): follow-up build-less-brittle/04 (this brief)"
  - "plugins/assay/skills/worker-desk/SKILL.md (Un-briefed issues section): follow-up build-less-brittle/04 (this brief)"
  - "labels error-class, design-owed on each repo in scope: out-of-scope (per-repo provisioning, as the intake skill already requires for needs-decision)"
---

# Brief 04 — Intake files by error class

## Context

files:
- `plugins/assay/skills/intake-desk/SKILL.md`: §"The judgment half", step 1 (CREATE-PLACEHOLDER triage).
- `plugins/assay/skills/pr-review-desk/SKILL.md`: the "Recurrence-promotion" paragraph.
- `plugins/assay/skills/worker-desk/SKILL.md`: §"Un-briefed issues".
- `changelog/build-less-brittle-04.md` (planned)

facts:
- `deskfile attach -R <repo> --to <N> --body-file <f>` exists (`cmdAttach`, deskfile.go:926 at
  f7bde6bfa). It refuses a CLOSED issue (exit 5) and is **not** counted against the new-issue
  budget (deskfile.go:224–226). `deskfile new` is budgeted (default 3 per 24h, deskfile.go:253),
  so creating a class issue is the scarce act and attaching is the cheap one.
- Next-up dispatches only `todo` rows (statusgen/nextup.go:408, 699), and `blocked` is a valid
  status token (statusgen/checks.go:16). A placeholder set to `blocked` therefore leaves the
  dispatch queue without new machinery. Step 1 of the Task verifies that the scanner does not
  overwrite a hand-set status.
- The intake desk already runs at strong tier and already records an impact/risk/effort triple.
  The class decision is added to that same judgement, not a new pass.
- The class issue schema, trigger and kinds are in spec §4.1. Only `confirmed-defect` and
  `false-positive` count, deduped by `incident-group`. That keeps mirrored reports, intended
  refusals and feature requests from triggering design work.
- Line counts at f7bde6bfa: intake-desk 534, pr-review-desk 968, worker-desk 868.
- **Hotspot wiring (build-less-brittle/08, added by the SOTA amendment).** Each attached instance
  also records `module: <S- owner path or cmd/<verb>>`, so the monthly brittle pass can join
  class history to the hotspot ranking without a second read. A class whose module carries a
  `brittle` mark is `design-owed` at its **first** counted instance (the mark already
  supplies the defect history the 3-instance rule waits for), and its deliverable is the
  investigation (09) before any design brief. One line in the trigger text; no new label
  logic — the intake desk reads the mark table in `docs/contracts.md`.

design-fit:
  owner: plugins/assay/skills/intake-desk/SKILL.md (the front door owns triage)
  contract: none — triage procedure has no semantic-owner row
  retires: ["pr-review-desk recurrence-promotion → guardrail route"]
  weight: verbs 0, flags 0, refusals 0, rule-text lines ≤ 0 across the three skills
  why-add: n/a

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- No new verb, flag, label-reading code or status token. Reuse `deskfile attach`, labels, and
  the `blocked` status.
- Each skill edit is net ≤ 0 lines. Offset with incident narrative moved to a findings link, or
  with restatements of the adoption guide.

## Task

1. **Verify the parking mechanism first.** Hand-set a scan placeholder's status to `blocked` in a
   scratch copy. Run `scanloop plan --offline` against a captured inbound fixture and confirm the
   status survives regeneration. If the scanner rewrites it, STOP and report NEEDS_CONTEXT with
   the overwriting code path. Do not add scanner code in this brief.
2. **Intake step 1 gains a class decision** (≤ 12 lines). Is the symptom an instance of an open
   `error-class` issue? Yes: `deskfile attach` a block carrying `kind`, `incident-group`,
   optional `introduced-by: <ref> (<same-symptom|shared-mechanism|introduced-by-commit|unconfirmed>)`,
   and a one-line evidence summary. No, and it is a machinery defect: record a `class:` line
   in the triage comment. Open a class issue only when a **second** symptom shares that
   mechanism. That keeps issue volume down: the budget makes `new` the scarce act.
3. **The trigger** (≤ 6 lines). The class becomes `design-owed` at 3 counted instances or its 2nd
   merged fix, whichever comes first. Label it `design-owed`. Set every symptom placeholder in
   the class to `blocked` with the class issue number in its Notes. The design PR's `Closes`
   line closes the symptoms. `intended-control` instances route to the refusal-text owner as a
   UX/wording fix, never as design work. Every instance carries `module:`; a class in a
   `brittle`-marked module is design-owed at its first counted instance and its deliverable
   is the investigation (build-less-brittle/09) first.
4. **Review recurrence** (replace, don't add). A finding raised three or more times across PRs
   is attached to (or opens) an `error-class` issue instead of being filed as a guardrail-
   promotion candidate.
5. **Worker dispatch** (≤ 3 lines). An un-briefed `design-owed` class issue dispatches at strong
   tier, and its deliverable is a brief per `author-brief`, not code.
6. Offset lines, and write the changelog fragment.

## Verify (executable — no prose-only DoD items)

Rows run from the root of `medici-finance/assay`. The skill text is prose: rows 1–4 gate
presence, rows 5–6 dereference the mechanisms the text names, and rows 7–9 are the net ≤ 0
weight rows. Whether the procedure is followed is measured by the project's close-out (spec §5.4), not here.

| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -cE 'error-class' plugins/assay/skills/intake-desk/SKILL.md` | ≥ `2` |
| 2 | `grep -c -e 'confirmed-defect.*false-positive' -e 'incident-group' plugins/assay/skills/intake-desk/SKILL.md` | ≥ `2` |
| 3 | `grep -c -e '2nd merged fix' -e 'second merged fix' plugins/assay/skills/intake-desk/SKILL.md` | ≥ `1` |
| 4 | `s=$(sed -n '/Recurrence-promotion/,/^$/p' plugins/assay/skills/pr-review-desk/SKILL.md); echo "$s" \| grep -q 'error-class' && ! echo "$s" \| grep -qi 'guardrail-promotion' && echo REDIRECTED` | `REDIRECTED` (the paragraph routes to a class issue and no longer to a guardrail) |
| 5 | `grep -c '^func cmdAttach' tools/desk/cmd/deskfile/deskfile.go` | `1` (the verb the skill names exists) |
| 6 | `grep -c '"blocked": true' statusgen/checks.go` | `1` (the status token the skill uses is valid) |
| 7 | `test "$(wc -l < plugins/assay/skills/intake-desk/SKILL.md)" -le 534 && echo NET-OK` | `NET-OK` |
| 8 | `test "$(wc -l < plugins/assay/skills/pr-review-desk/SKILL.md)" -le 968 && echo NET-OK` | `NET-OK` |
| 9 | `test "$(wc -l < plugins/assay/skills/worker-desk/SKILL.md)" -le 868 && echo NET-OK` | `NET-OK` |
| 10 | `statusgen --consumers --root . --brief build-less-brittle/04; echo "exit=$?"` | `exit=0` at the PR head (no `consumers:` routing claim is disproved by the diff; the implementer replaces each self-routed entry with `fixed-here` in the same change). Exit 1 names the disproved claim |
| 11 | `grep -c -e 'module:' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'brittle' plugins/assay/skills/intake-desk/SKILL.md` | two counts, each ≥ `1` (the hotspot wiring: instances name their module, and a marked module lowers the trigger to the first instance) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. Task step 1's
     scanner finding is recorded here as a row of its own. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|

## Review
Gate: model (from frontmatter). Rows 7–9 cap line counts at their f7bde6bfa values. If another
merged PR shrank a skill first, this brief's offset obligation is unchanged; the reviewer checks
the diff's own numstat.
