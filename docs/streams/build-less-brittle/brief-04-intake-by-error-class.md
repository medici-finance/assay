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
version: 3
authored: "2026-09-24 by the build-less-brittle authoring session (read-only; author-brief format)"
sources:
  - "docs/streams/build-less-brittle/spec.md — 2026-09-30 pending scope amendment"
  - "freshness-checked 2026-09-30 @ 8485778515c041fc87966902a14eb9d195492be3: amend unfinished scope; no implementation claim"
  - "docs/streams/build-less-brittle/spec.md §2 D1, §3 rows 1 and 8, §4.1"
  - "tools/desk/cmd/deskfile/deskfile.go (attach verb; new-issue budget)"
  - "statusgen/nextup.go (dispatch reads only `todo` rows) and statusgen/checks.go (`blocked` is a valid status)"
  - "freshness-checked 2026-09-24 @ f7bde6bfa: intake-desk SKILL.md has no class, attach-to-class or design trigger step; pr-review-desk's recurrence-promotion routes to a guardrail"
exec-tier: strong
exec-tier-why: "(b) one procedure spans three skills (intake, review, worker dispatch) and must reuse existing verbs, labels and status tokens without inventing any."
domain: complicated
consumers:
  - "plugins/assay/skills/intake-desk/SKILL.md: fixed-here"
  - "plugins/assay/skills/pr-review-desk/SKILL.md (Recurrence-promotion paragraph): fixed-here"
  - "plugins/assay/skills/worker-desk/SKILL.md (Un-briefed issues section): fixed-here"
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
- Line counts at f7bde6bfa (for scale; the net ≤ 0 rows derive their own base): intake-desk 534, pr-review-desk 968, worker-desk 868.
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

## Record compatibility amendment — 2026-09-28

Use spec §4.1's evidence-reference fields in the existing class attachment block.
Keep the instance table and existing incident-group deduplication as the sole record.
Include a worked case in the edited procedure: two mirrored reports of one failure, a
later success on another revision, then a same-scope recovery check. Only the last
establishes recovery; the counted incident remains one. Legacy attachments remain readable
with unknown fields. Preserve this brief's net-zero skill-line and no-new-tool constraints.

## Work-input amendment — 2026-09-30

Within the existing class attachment/placeholder, carry the mechanism, known scope,
source revisions/evidence references, unresolved questions and next actionable step. Reuse
those fields in the downstream brief instead of requiring the next desk to rediscover the
class. Coalesce repeated reports through the existing incident-group rule; a new observation
can enrich the record without becoming another counted incident or another agent request.
Carry source origin and the existing trust-gate disposition/restrictions with reused text;
quarantined reporter content never becomes downstream instructions by being copied. In the
intake procedure use the labels `mechanism:`, `known-scope:`, `source-revisions:`,
`unresolved-questions:`, `next-action:`, `source-origin:` and `trust-disposition:` within the
existing attachment block. Label the worked case `### Work-input triage example`.

This is a procedure/record refinement within the existing line offsets, not new schema,
scheduler or runtime deduplication code. Execution-attempt coalescing belongs to graph 14
and is not a dependency. Review the worked triage with duplicate versus genuinely new
incidents; both retain evidence and only the latter advances the recurrence count.

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
3. **The trigger** (≤ 8 lines). The class becomes `design-owed` at 3 counted instances or its 2nd
   merged fix, whichever comes first. Label it `design-owed`. Set every symptom placeholder in
   the class to `blocked` with the class issue number in its Notes, except a symptom with
   production-down or security impact: that one stays `todo`, so the worker's two-strikes
   check and the `bleed` reply (spec §10 D-B) still reach it. A `bleed` reply by the driver's
   own login (spec §4.7) naming a parked symptom sets it back to `todo`. The design PR's
   `Closes` line closes the symptoms. `intended-control` instances route to the refusal-text owner as a
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

Rows run from the root of `medici-finance/assay`. The skill text is prose: rows 1–4 and 12
gate presence, rows 5–6 dereference the mechanisms the text names, and rows 7–9 are the net ≤ 0
weight rows. Whether the procedure is followed is measured by the project's close-out (spec §5.4), not here.

| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -cE 'error-class' plugins/assay/skills/intake-desk/SKILL.md` | ≥ `2` |
| 2 | `grep -c -e 'confirmed-defect.*false-positive' -e 'incident-group' plugins/assay/skills/intake-desk/SKILL.md` | ≥ `2` |
| 3 | `grep -c -e '2nd merged fix' -e 'second merged fix' plugins/assay/skills/intake-desk/SKILL.md` | ≥ `1` |
| 4 | `s=$(sed -n '/Recurrence-promotion/,/^$/p' plugins/assay/skills/pr-review-desk/SKILL.md); echo "$s" \| grep -q 'error-class' && ! echo "$s" \| grep -qi 'guardrail-promotion' && echo REDIRECTED` | `REDIRECTED` (the paragraph routes to a class issue and no longer to a guardrail) |
| 5 | `grep -c '^func cmdAttach' tools/desk/cmd/deskfile/deskfile.go` | `1` (the verb the skill names exists) |
| 6 | `grep -c '"blocked": true' statusgen/checks.go` | `1` (the status token the skill uses is valid) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | `NET-OK` |
| 10 | `statusgen --consumers --root . --brief build-less-brittle/04; echo "exit=$?"` | `exit=0` at the PR head (no `consumers:` routing claim is disproved by the diff; the implementer replaces each self-routed entry with `fixed-here` in the same change). Exit 1 names the disproved claim |
| 11 | `grep -c -e 'module:' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'brittle' plugins/assay/skills/intake-desk/SKILL.md` | two counts, each ≥ `1` (the hotspot wiring: instances name their module, and a marked module lowers the trigger to the first instance) |
| 12 | `grep -c -e 'production-down' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'bleed' plugins/assay/skills/intake-desk/SKILL.md` | two counts, each ≥ `1` (the parking carve-out: a production-down or security symptom is never parked, and the driver's `bleed` reply un-parks one) |
| 13 | `f=plugins/assay/skills/intake-desk/SKILL.md; for key in mechanism known-scope source-revisions unresolved-questions next-action source-origin trust-disposition; do grep -qF "$key:" "$f" \|\| exit 1; done; grep -q '^### Work-input triage example$' "$f" && echo WORK-INPUT-FIELDS` | `WORK-INPUT-FIELDS` (presence only; the worked-triage review below checks meaning, retained source trust and recurrence counting) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code,
     output line(s) or hash, date, runner). "verified" requires a NON-implementer. Task step 1's
     scanner finding is recorded here as a row of its own. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -cE 'error-class' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | `2` (≥ 2) | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 2 | `grep -c -e 'confirmed-defect.*false-positive' -e 'incident-group' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | `6` (≥ 2) | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 3 | `grep -c -e '2nd merged fix' -e 'second merged fix' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | `1` (≥ 1) | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 4 | `s=$(sed -n '/Recurrence-promotion/,/^$/p' plugins/assay/skills/pr-review-desk/SKILL.md); echo "$s" \| grep -q 'error-class' && ! echo "$s" \| grep -qi 'guardrail-promotion' && echo REDIRECTED` | pass exit=0 | `REDIRECTED` | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 5 | `grep -c '^func cmdAttach' tools/desk/cmd/deskfile/deskfile.go` | pass exit=0 | `1` | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 6 | `grep -c '"blocked": true' statusgen/checks.go` | pass exit=0 | `1` | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | `NET-OK` — base cb6da0625 (merge-base), tip 7f156368e; 537 → 537 lines | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | `NET-OK` — base cb6da0625, tip 7f156368e; 1023 → 1023 lines | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | `NET-OK` — base cb6da0625, tip 7f156368e; 882 → 882 lines | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 10 | `statusgen --consumers --root . --brief build-less-brittle/04; echo "exit=$?"` | pass exit=0 | 3 CORROBORATED (the three `fixed-here` skill entries), 1 UNCHECKED (the out-of-scope labels entry, unchanged since the merge-base), 0 disproved; `exit=0` | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 11 | `grep -c -e 'module:' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'brittle' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | `2` then `4` (each ≥ 1) | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| 12 | `grep -c -e 'production-down' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'bleed' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | `1` then `1` (each ≥ 1) | 2026-09-30 | assay-worker-app[bot] @ 7f156368ee91 (on-behalf-of human:ian) (implementer, self-run) |
| T1 | `scanloop plan --offline` (Task step 1 as written), then a scratch Go probe of `runScanIssues` over a copied placeholder root (probe file not committed) | pass on an open scan; the label round trip overwrites it (see Output) | `scanloop plan` has no `--offline` flag (`flag provided but not defined: -offline`; the flag exists only on `scanloop run`), so the proof ran the scanner the scan lane calls (`statusgen --scan-issues`) directly: a placeholder hand-set to `status: blocked` with the class number in its body line was byte-identical after an open-issue scan (`status="blocked"`); `statusgen --lint` with it: `LINT: PASS`, exit 0; a close scan retired it to `done/` (`status="done"`); `--- PASS: TestParkProbe (0.08s)` at base 848577851. `planScan` skips an existing placeholder and `planUnblock` strips only `blocked:`/`blockedAt:`. Correction (review round 1): one scanner path DOES overwrite it. An open issue that gains an excluded label is planned `retire-label` (`statusgen/scanissues.go` `planScan`), which writes `status: done` and archives; when the label comes off, `reactivate` writes `status: todo` (`applyCloseOut`); the body line survives. No scanner code changed here: intake step 1 carries a skill-side re-park (a `todo` row whose body still says `Parked` goes back to `blocked`) | 2026-09-30 | assay-worker-app[bot] @ 65c39e74fb19 (on-behalf-of human:ian) (implementer, self-run) |

The reviewer also walks the amendment's worked case through the existing deliverables and
records the source links, gap handling and outcome interpretation in the review. These are
semantic acceptance checks; presence of field names alone does not satisfy them.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main b0088804294b8b68ad8d06f341e6f0fd9dd2637d, gate: model, all four risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim; it ran on the darwin host (no row needs a Linux-only facility), statusgen built from a `--no-hardlinks` clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -cE 'error-class' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 2 | `grep -c -e 'confirmed-defect.*false-positive' -e 'incident-group' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:06e9d52c1720 | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 3 | `grep -c -e '2nd merged fix' -e 'second merged fix' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 4 | `s=$(sed -n '/Recurrence-promotion/,/^$/p' plugins/assay/skills/pr-review-desk/SKILL.md); echo "$s" \| grep -q 'error-class' && ! echo "$s" \| grep -qi 'guardrail-promotion' && echo REDIRECTED` | pass exit=0 | sha256:8cfcff694556 | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 5 | `grep -c '^func cmdAttach' tools/desk/cmd/deskfile/deskfile.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 6 | `grep -c '"blocked": true' statusgen/checks.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 10 | `statusgen --consumers --root . --brief build-less-brittle/04; echo "exit=$?"` | fail exit=0 | sha256:987b7f974921 | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 11 | `grep -c -e 'module:' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'brittle' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:7b90b6c82d45 | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 12 | `grep -c -e 'production-down' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'bleed' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:ad0fadf63cc7 | 2026-09-30 | human:ian @ b0088804294b (git-config) |
| 13 | `f=plugins/assay/skills/intake-desk/SKILL.md; for key in mechanism known-scope source-revisions unresolved-questions next-action source-origin trust-disposition; do grep -qF "$key:" "$f" \|\| exit 1; done; grep -q '^### Work-input triage example$' "$f" && echo WORK-INPUT-FIELDS` | fail exit=1 | sha256:e3b0c44298fc | 2026-09-30 | human:ian @ b0088804294b (git-config) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `grep -cE 'error-class' plugins/assay/skills/intake-desk/SKILL.md` | >= 2 | exit 0; `2` | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 2 | `grep -c -e 'confirmed-defect.*false-positive' -e 'incident-group' plugins/assay/skills/intake-desk/SKILL.md` | >= 2 | exit 0; `6` | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 3 | `grep -c -e '2nd merged fix' -e 'second merged fix' plugins/assay/skills/intake-desk/SKILL.md` | >= 1 | exit 0; `1` | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 4 | Verify row 4 command as written (sed the Recurrence-promotion paragraph of pr-review-desk SKILL.md, grep error-class, negated grep -i guardrail-promotion) | REDIRECTED | exit 0; `REDIRECTED` | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 5 | `grep -c '^func cmdAttach' tools/desk/cmd/deskfile/deskfile.go` | 1 | exit 0; `1` | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 6 | `grep -c '"blocked": true' statusgen/checks.go` | 1 | exit 0; `1` | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 7 | Verify row 7 command as written (intake-desk net line count, impl commit vs its parent) | NET-OK | exit 0; `NET-OK`; impl 23ebf3fc7, base b2c341cfd; 537 to 537 lines | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 8 | Verify row 8 command as written (pr-review-desk net line count) | NET-OK | exit 0; `NET-OK`; impl 23ebf3fc7, base b2c341cfd; 1023 to 1023 lines | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 9 | Verify row 9 command as written (worker-desk net line count) | NET-OK | exit 0; `NET-OK`; impl 23ebf3fc7, base b2c341cfd; 882 to 882 lines | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 10 | `statusgen --consumers --root . --brief build-less-brittle/04; echo "exit=$?"` | exit=0 at the PR head | FAIL AS AUTHORED on merged main: `exit=2`, COULD-NOT-CHECK (brief not in the diff against b0088804294b). Hand procedure (checkout impl commit 23ebf3fc7, a squash commit, with --base its parent b2c341cfd): 3 CORROBORATED fixed-here entries, 1 UNCHECKED out-of-scope labels entry, 0 disproved, exit 0. Substance passes; check-definition failure | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 11 | `grep -c -e 'module:' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'brittle' plugins/assay/skills/intake-desk/SKILL.md` | two counts, each >= 1 | exit 0; `2` then `4` | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 12 | `grep -c -e 'production-down' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'bleed' plugins/assay/skills/intake-desk/SKILL.md` | two counts, each >= 1 | exit 0; `1` then `1` | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 13 | Verify row 13 command as written (seven work-input field labels plus the Work-input triage example heading in intake-desk SKILL.md) | WORK-INPUT-FIELDS | FAIL: exit 1, no output. Per-key grep -cF counts: mechanism: 0, known-scope: 0, source-revisions: 0, unresolved-questions: 0, next-action: 0, source-origin: 0, trust-disposition: 0; heading count 0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — counted-instance threshold = 3 @ plugins/assay/skills/intake-desk/SKILL.md:291 — matches the stream's own constraint, docs/streams/build-less-brittle/spec.md:113 (design-owed at 3 counted instances, distinct incident groups of kind confirmed-defect or false-positive) and brief Task step 3; reversible prose knob.
RISK-VALUE: DERIVED — merged-fix threshold = 2nd @ plugins/assay/skills/intake-desk/SKILL.md:292 — matches spec.md:113-114 (or at its 2nd merged fix, whichever comes first) and brief Task step 3; reversible prose knob.
RISK-VALUE: DERIVED — brittle-module threshold = first counted instance @ plugins/assay/skills/intake-desk/SKILL.md:292 — matches the brief's hotspot-wiring fact (the brittle mark already supplies the defect history the 3-instance rule waits for); reversible prose knob.

Notes:
- FAIL on row 13 (substance): none of the seven work-input labels (`mechanism:` through `trust-disposition:`) and no `### Work-input triage example` heading are in the intake-desk skill, and nothing there carries source origin or trust disposition. The amendment landed in the brief (#1877, #1891) shortly before the implementation (#1876) merged; the implementer's Evidence has no row 13. Bug: #1935. Rows 1-9, 11 and 12 pass by hand and in the witness. Row 10 is also a check-definition failure (exit 2 COULD-NOT-CHECK on merged main; by hand at the implementation commit with its parent as base, 3 corroborated, 0 disproved), tracked with the other row re-authors at #1927. All three RISK-VALUE lines are DERIVED.
- Witness platform: darwin. No row is check:ci and none needs Linux-only facilities, so the OrbStack recipe was not used.
- Grounding: the expectation was written from the brief text and main before reading the implementation diff. Everything from Task steps 2 to 6 and the 2026-09-28 record compatibility amendment is present on main; the 2026-09-30 work-input amendment is not.
- Row 13, FAIL on substance as well as on presence. The work-input amendment asks the attachment block to carry the labels mechanism:, known-scope:, source-revisions:, unresolved-questions:, next-action:, source-origin: and trust-disposition:, and for a worked case headed "### Work-input triage example". Merged main has none of the seven labels and no such heading. The implementation carries related but differently named fields (class: <mechanism> in the title, scope, source-ref, open-questions, next-step), so part of the intent is there. Nothing in the step-1 text carries source origin or the trust-gate disposition with reused text, and nothing says quarantined reporter content must never become downstream instructions. That is a missing deliverable, not a naming difference. Timeline: the amendment landed in the brief via #1877 (17:55) and #1891 (19:00); implementation #1876 squash-merged at 19:02 the same day without it, and the implementer's Evidence has no row 13. It needs a rework follow-up on intake-desk step 1 within the net-zero line budget.
- Row 10, check-definition failure. Run as written on merged main it returns exit=2 COULD-NOT-CHECK, because the Expect is scoped "at the PR head". By hand, at the squash commit with --base set to its parent, it corroborates 3 entries, disproves 0 and exits 0. The witness also reads the Expect cell's "Exit 1 names the disproved claim" as the expected exit code ("exit 0, expected 1"). The row needs a merged-main form, such as the base/tip derivation that rows 7 to 9 use, and an Expect that the witness parses as exit 0. On its own this row would end BLOCKED; row 13's FAIL is the governing verdict.
- statusgen brief --check-verified on a throwaway --no-hardlinks clone with README row 04 flipped to verified: exit 1 (rows 1 to 13 have no execution witness). After appending the verifyrun witness in the same throwaway clone: exit 1, naming "row 10: fail" and "row 13: fail".
- 2026-09-28 record compatibility amendment, semantic walk: the step-1 text has evidence-reference fields (observed-at, source-ref, scope, state, checked-at, recovery-ref). Its worked case covers two mirrored reports under one incident-group as one counted instance, a later success on another revision as another scope rather than recovery, and only a same-scope re-check appending state: recovered, with the count unchanged. A separate occurrence gets a new incident-group and counts. Older blocks with missing fields read as unknown. This part passes.
- Task step 1 scanner probe is the implementer's T1 row, not a Verify row, and this pass did not repeat it. T1 records that scanloop plan has no --offline flag and that the retire-label to reactivate path does rewrite status. The skill answers that with a re-park rule rather than scanner code, which is within the brief's no-new-code constraint.
- The changelog fragment is changelog/intake-error-class.md, not the planned build-less-brittle-04.md name. Its content covers the change, but it does not mention the work-input fields either.

VERIFY: FAIL — row 13
### Verification — non-implementer re-verify at merged main 024c87b01aba (2026-09-30)

What moved since the last run: the previous verify (#1935, at main b00888042) was VERIFY: FAIL on row 13 because the work-input amendment's seven fields and the worked-case heading were missing from the intake-desk skill. #1943 (squash 8611850ab) added them with the intake-desk skill held at 537 lines, and row 13 now passes. Row 10 is unchanged: it exits 2 (could-not-check) on merged main as written, and passes when rerun against the implementing diff.

Merged main: 024c87b01aba8f6c7dd7ccd939e647a9b936be09 (origin/main re-fetched at run time; it had not moved). Implementing commits: 23ebf3fc7 (#1876, carries the Brief: trailer) and 8611850ab (#1943, the row-13 follow-up, no Brief: trailer). All rows ran from the repo root of a detached worktree at that sha, offline.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `grep -cE 'error-class' plugins/assay/skills/intake-desk/SKILL.md` | ≥ 2 | exit 0; 2. Discharges Verify row 1. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `grep -c -e 'confirmed-defect.*false-positive' -e 'incident-group' plugins/assay/skills/intake-desk/SKILL.md` | ≥ 2 | exit 0; 6. Discharges Verify row 2. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `grep -c -e '2nd merged fix' -e 'second merged fix' plugins/assay/skills/intake-desk/SKILL.md` | ≥ 1 | exit 0; 1. Discharges Verify row 3. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `s=$(sed -n '/Recurrence-promotion/,/^$/p' plugins/assay/skills/pr-review-desk/SKILL.md); echo "$s" \| grep -q 'error-class' && ! echo "$s" \| grep -qi 'guardrail-promotion' && echo REDIRECTED` | REDIRECTED | exit 0; REDIRECTED. The paragraph now says to attach the finding to the open error-class issue, or to record the class per intake-desk step 1. Discharges Verify row 4. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `grep -c '^func cmdAttach' tools/desk/cmd/deskfile/deskfile.go` | 1 | exit 0; 1. Discharges Verify row 5. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `grep -c '"blocked": true' statusgen/checks.go` | 1 | exit 0; 1. Discharges Verify row 6. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" && echo NET-OK` | NET-OK | exit 0; NET-OK. impl resolved to 23ebf3fc7, base b2c341cfd, 537 lines before and after. The #1943 follow-up (no Brief: trailer, so the row does not select it) is also 537 before and after, which keeps the amendment's net-zero constraint. Discharges Verify row 7. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | NET-OK | exit 0; NET-OK. base b2c341cfd, tip 23ebf3fc7, 1023 lines before and after. Discharges Verify row 8. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | NET-OK | exit 0; NET-OK. base b2c341cfd, tip 23ebf3fc7, 882 lines before and after. Discharges Verify row 9. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10a | `statusgen --consumers --root "$(git rev-parse --show-toplevel)" --brief build-less-brittle/04; echo "exit=$?"` (the command as written, with the root given as an absolute path) | exit=0 at the PR head | exit=2, COULD-NOT-CHECK: "is not in the diff against 024c87b01aba…, so this run carries no evidence about its claims". On merged main this row cannot give its verdict as written, because the brief's Expect applies at the PR head. Row 10b is the corrected form. #1943 does not touch the brief file, so a --consumers run over its diff would be could-not-check too; its only product change, the intake-desk skill, is a fixed-here entry that 10b corroborates. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 10b | `statusgen --consumers --root "$(git rev-parse --show-toplevel)" --brief build-less-brittle/04 --base 23ebf3fc7bf0579329258fd28e9f566732fdb56a~1; echo "exit=$?"` run with HEAD detached at 23ebf3fc7bf0579329258fd28e9f566732fdb56a (squash of #1876), then HEAD returned to 024c87b01aba | exit=0, 0 disproved | exit=0. Summary: 3 corroborated, 0 disproved, 1 unchecked. The three fixed-here skill entries are CORROBORATED. The labels entry (out-of-scope) is UNCHECKED because it is unchanged since the merge-base. This is the implementing diff, which is the form the tool itself recommends for a merged brief. Discharges Verify row 10. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 11 | `grep -c -e 'module:' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'brittle' plugins/assay/skills/intake-desk/SKILL.md` | two counts, each ≥ 1 | exit 0; 2 then 4. Discharges Verify row 11. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 12 | `grep -c -e 'production-down' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'bleed' plugins/assay/skills/intake-desk/SKILL.md` | two counts, each ≥ 1 | exit 0; 1 then 1. Discharges Verify row 12. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 13 | `f=plugins/assay/skills/intake-desk/SKILL.md; for key in mechanism known-scope source-revisions unresolved-questions next-action source-origin trust-disposition; do grep -qF "$key:" "$f" \|\| exit 1; done; grep -q '^### Work-input triage example$' "$f" && echo WORK-INPUT-FIELDS` | WORK-INPUT-FIELDS | exit 0; WORK-INPUT-FIELDS. All seven labels are present in step 1's attachment block, and the heading is at line 331 of the intake-desk skill. This is the row that failed on the previous run; #1943 fixed it. Discharges Verify row 13. | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Beyond the table (semantic acceptance, which is not a Verify row; recorded for traceability). I read the Work-input triage example against the 2026-09-28 and 2026-09-30 amendments. It covers each required case:
- Two mirrored reports at r1 share one incident-group and count as one instance. The mirror only enriches source-revisions:/known-scope: and requests no agent.
- An unblessed quote carries source-origin: and "trust-disposition: quarantined — data only", and never becomes a next-action:.
- Success on r2 is another known-scope:, not recovery. Only a same-scope recheck on r1 appends state: recovered + recovery-ref, and the count is unchanged.
- A separate failure on r3 takes a new incident-group and alone advances the count.

Step 1 also says legacy scope/source-ref/open-questions/next-step read as the renamed fields, with a missing field read as unknown. Task step 1 (the implementer's T1 row) is also consistent with the code: the scanner's retire-label/reactivate path does rewrite status (statusgen/scanissues.go planScan and applyCloseOut), and step 1's "Re-park" sentence handles that without new scanner code.

Execution witness (`statusgen verifyrun --dry-run`, emitted verbatim; not written back to the brief):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -cE 'error-class' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c -e 'confirmed-defect.*false-positive' -e 'incident-group' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:06e9d52c1720 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -c -e '2nd merged fix' -e 'second merged fix' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 4 | `s=$(sed -n '/Recurrence-promotion/,/^$/p' plugins/assay/skills/pr-review-desk/SKILL.md); echo "$s" \| grep -q 'error-class' && ! echo "$s" \| grep -qi 'guardrail-promotion' && echo REDIRECTED` | pass exit=0 | sha256:8cfcff694556 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c '^func cmdAttach' tools/desk/cmd/deskfile/deskfile.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c '"blocked": true' statusgen/checks.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --root . --brief build-less-brittle/04; echo "exit=$?"` | fail exit=0 | sha256:e5b7610fd355 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -c -e 'module:' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'brittle' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:7b90b6c82d45 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c -e 'production-down' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'bleed' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:ad0fadf63cc7 | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |
| 13 | `f=plugins/assay/skills/intake-desk/SKILL.md; for key in mechanism known-scope source-revisions unresolved-questions next-action source-origin trust-disposition; do grep -qF "$key:" "$f" \|\| exit 1; done; grep -q '^### Work-input triage example$' "$f" && echo WORK-INPUT-FIELDS` | pass exit=0 | sha256:efb852f8dd4e | 2026-09-30 | assay-verifier-app[bot] @ 024c87b01aba (on-behalf-of human:ian) (forge-identity) |

Witness note, row 10: the witness says "fail exit=0 — exit 0, expected 1". The witness parser read the Expect cell's "Exit 1 names the disproved claim" as an expected exit code of 1. The recorded exit=0 is the trailing echo's own status. The command's real output was exit=2 could-not-check (see row 10a). This is a witness/Expect-wording artifact, not a disproved claim.

RISK-VALUE: DERIVED — design-owed trigger = 3 counted instances or the 2nd merged fix @ plugins/assay/skills/intake-desk/SKILL.md:281-282. This matches the stream's own stated constraint in spec §4.1 (docs/streams/build-less-brittle/spec.md:113-114) and §3 row 1 (spec.md:82) word for word. That includes the counting rule that only confirmed-defect/false-positive count, deduped by incident-group, so mirrored reports of one failure count once. The derivation is against the spec's stated constraint. The spec states 3 but does not argue from first principles why 3 beats 2 or 4. That gap is acceptable for a reversible knob.
Full enumeration over the #1876 + #1943 skill diffs and the brief's Deliverables:
- class issue opens at the second symptom (first in a brittle-marked module) @ intake-desk SKILL.md:278-279
- review recurrence at three or more times across PRs @ pr-review-desk SKILL.md:530
- brittle-module trigger at the first counted instance @ intake-desk SKILL.md:282
- the new-issue budget (default 3 per 24h, unchanged, lives in deskfile)
- the authoring line budgets (≤12, ≤8, ≤3, net ≤0)

Rank: every entry is a reversible procedure knob in prose. Undoing one takes an edit and a plugin re-release, and none moves funds, identity or published state. The risk metadata is present and all "no", and irreversible: no. The fail-safe trigger does not fire. The entry above is listed as the top-ranked one for completeness.

rows_passed=13 rows_total=13

VERIFY: PASS

## Review
Gate: model (from frontmatter). Rows 7–9 compare each skill at this brief's own change against
the same file just before it (the base is derived in the row; see the README's shared
conventions), so growth or shrinkage by other PRs on main never moves the bar.
