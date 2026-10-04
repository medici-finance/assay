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
version: 4
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
Row 10 corroborates the `consumers:` claims against the delivering change itself, pinned so it
still has that diff to read after merge (re-authored in version 4, #1977).

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
| 10 | `d=$(mktemp -d "$PWD/.bl04-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 23ebf3fc7bf0 && statusgen --consumers --root "$d" --brief build-less-brittle/04 --base 23ebf3fc7bf0~1; s=$?; rm -rf "$d"; exit $s` | exit 0; output is `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing` (the check runs in a throwaway shared clone, made inside the checkout and removed afterwards, checked out at 23ebf3fc7bf0, the squash that delivered this brief in #1876, with the base pinned to its parent, so the diff it reads is exactly the delivering change: never main's later commits, never the runner's own working tree. The three `fixed-here` skill entries are corroborated by that diff; the one UNCHECKED entry is the `out-of-scope` labels line, which names no path in this repo and stays the reviewer's call per brief-rule 9, never a pass. The later work-input amendment (#1943) edits the intake skill only, a path this row already corroborates, and adds no `consumers:` claim) |
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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -cE 'error-class' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:53c234e5e847 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c -e 'confirmed-defect.*false-positive' -e 'incident-group' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -c -e '2nd merged fix' -e 'second merged fix' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 4 | `s=$(sed -n '/Recurrence-promotion/,/^$/p' plugins/assay/skills/pr-review-desk/SKILL.md); echo "$s" \| grep -q 'error-class' && ! echo "$s" \| grep -qi 'guardrail-promotion' && echo REDIRECTED` | pass exit=0 | sha256:8cfcff694556 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c '^func cmdAttach' tools/desk/cmd/deskfile/deskfile.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c '"blocked": true' statusgen/checks.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 10 | `d=$(mktemp -d "$PWD/.bl04-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 23ebf3fc7bf0 && statusgen --consumers --root "$d" --brief build-less-brittle/04 --base 23ebf3fc7bf0~1; s=$?; rm -rf "$d"; exit $s` | pass exit=0 | sha256:260242d7406f | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -c -e 'module:' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'brittle' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:7b90b6c82d45 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c -e 'production-down' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'bleed' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:ad0fadf63cc7 | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |
| 13 | `f=plugins/assay/skills/intake-desk/SKILL.md; for key in mechanism known-scope source-revisions unresolved-questions next-action source-origin trust-disposition; do grep -qF "$key:" "$f" \|\| exit 1; done; grep -q '^### Work-input triage example$' "$f" && echo WORK-INPUT-FIELDS` | pass exit=0 | sha256:efb852f8dd4e | 2026-10-02 | assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity) |

### Verification — non-implementer re-witness of the amended Verify table at merged main 2a6460e85c78 (2026-10-02)

- Target SHA: 2a6460e85c78 (detached worktree at merged main; clean before the run).
- Runner identity: assay-verifier-app[bot] (on-behalf-of human:ian), non-implementer. Host: darwin. statusgen v1.0.30 as installed on the runner's PATH (not built from the target SHA).
- Gate: model. Risk answers: regulatory no, customer no, irreversible no, sensitive-data no.
- What moved since the last run: #2030 re-authored Verify row 10 (brief version 4) so that it reads the delivering diff from a throwaway shared clone pinned at 23ebf3fc7bf0 with the base pinned to its parent. The earlier form exited 2 (could-not-check) on merged main. Rows 1–9 and 11–13 are unchanged.
- `statusgen verifyrun --dry-run`: tool exit 0, 13 of 13 rows stamped pass. `statusgen verifyrun` (writing form): tool exit 0, 13 of 13 rows stamped pass, witness table appended to the brief's Evidence section in the worktree (left uncommitted). Witness runner cell: assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) (forge-identity).
- Every row was then re-run by hand, each in a fresh `bash -c` at the worktree root, offline. The table below is the hand run.

| # | command (abbreviated) | exit | key observed output | result |
|---|-----------------------|------|---------------------|--------|
| 1 | grep -cE 'error-class' on the intake-desk skill | 0 | `2` (Expect ≥ 2) | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 2 | grep -c 'confirmed-defect.*false-positive' / 'incident-group' on the intake-desk skill | 0 | `6` (Expect ≥ 2) | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 3 | grep -c '2nd merged fix' / 'second merged fix' on the intake-desk skill | 0 | `1` (Expect ≥ 1) | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 4 | sed the Recurrence-promotion paragraph of the pr-review-desk skill; grep error-class; negated grep -i guardrail-promotion | 0 | `REDIRECTED`. The paragraph says to attach the finding to the open error-class issue, or record the class per intake-desk step 1 | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 5 | grep -c '^func cmdAttach' in the deskfile source | 0 | `1` (Expect 1) | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 6 | grep -c '"blocked": true' in statusgen checks.go | 0 | `1` (Expect 1) | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 7 | intake-desk skill net line count, delivering commit vs its parent | 0 | `NET-OK`. impl resolved to 23ebf3fc7bf0 (#1876, the only first-parent commit carrying the Brief trailer outside the streams and changelog paths), base b2c341cfd00e; 537 lines before, 537 after. The base-differs-from-tip guard is live because impl resolved | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 8 | pr-review-desk skill net line count, same derivation | 0 | `NET-OK`. base b2c341cfd00e, tip 23ebf3fc7bf0; 1023 before, 1023 after | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 9 | worker-desk skill net line count, same derivation | 0 | `NET-OK`. base b2c341cfd00e, tip 23ebf3fc7bf0; 882 before, 882 after | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 10 | throwaway shared clone inside the checkout, detached at 23ebf3fc7bf0; statusgen --consumers --brief build-less-brittle/04 --base 23ebf3fc7bf0~1; clone removed; exit carries the tool's status | 0 | `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing`. Base reported as b2c341cfd00e. CORROBORATED: the three fixed-here skill entries. UNCHECKED: the out-of-scope labels entry (names no path in this repo; the reviewer's call, never a pass). The clone directory was gone afterwards and the worktree list was unchanged | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 11 | grep -c 'module:' && grep -c 'brittle' on the intake-desk skill | 0 | `2` then `4` (each ≥ 1) | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 12 | grep -c 'production-down' && grep -c 'bleed' on the intake-desk skill | 0 | `1` then `1` (each ≥ 1) | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |
| 13 | seven work-input labels by grep -qF, plus the '### Work-input triage example' heading, on the intake-desk skill | 0 | `WORK-INPUT-FIELDS`. Per-label counts: mechanism 2, known-scope 3, source-revisions 2, unresolved-questions 1, next-action 2, source-origin 2, trust-disposition 3; heading at line 331 | pass — 2026-10-02 assay-verifier-app[bot] @ 2a6460e85c78 (on-behalf-of human:ian) |

Cross-check of the tool's stamps against real output. The witness marks rows 4–9 and 13 "expect: exit-status only". For each of those the hand run printed the exact Expect token (REDIRECTED, 1, 1, NET-OK three times, WORK-INPUT-FIELDS), and the witness hashes equal the sha256 prefix of that token plus newline (8cfcff694556, 4355a46b19d3, 458c4e39effe, efb852f8dd4e). Rows 1 and 11 hashes (53c234e5e847, 7b90b6c82d45) likewise equal the hash of `2` and of `2`/`4`. No row passes on exit status with output that misses its Expect. No row ends in a trailing echo of its own status, so no recorded exit is the echo's. No row uses a shell-specific path-modifier expansion. No row is marked check:ci and none needs a Linux-only or network-off facility; all thirteen ran on this host. Row 10 is no longer vacuous on merged main: it has the delivering diff to read and its exit is the consumers tool's own.

RISK-VALUE: DERIVED — design-owed trigger = 3 counted instances or the 2nd merged fix @ plugins/assay/skills/intake-desk/SKILL.md:281-282 — matches the stream's stated constraint, docs/streams/build-less-brittle/spec.md:113-114, including the counting rule (only confirmed-defect and false-positive, deduped by incident-group). The spec states 3 without a first-principles argument for 3 over 2 or 4; acceptable for a reversible prose knob.

Enumeration over the #1876 and #1943 skill diffs and the brief's Deliverables: first-counted-instance trigger for a brittle-marked module (intake-desk skill line 282); review recurrence at three or more times across separate PRs (pr-review-desk skill line 531); the new-issue budget (default 3 per 24h, unchanged, lives in deskfile); the authoring line budgets (≤ 12, ≤ 8, ≤ 3, net ≤ 0). All are reversible procedure knobs in prose: an edit and a plugin re-release undoes any of them, and none moves funds, identity or published state. Risk metadata is present and all four answers are no, so the fail-safe trigger does not fire.

VERIFY: PASS

Reasoning: all thirteen rows of the amended table exit 0 on merged main 2a6460e85c78 and each row's real output meets its Expect, both in the `statusgen verifyrun` witness (13 of 13 pass, tool exit 0 in the dry and the writing form) and in an independent hand run. Row 10, the row #2030 re-authored and the one that could not give a verdict on merged main before, now exits 0 with exactly the summary line its Expect names. This is a model verdict on a gate: model brief with no risk flag; the verifier records Evidence and does not set the status.

Row defects and notes (none changes the verdict):
- Rows 7–9 measure only the delivering commit 23ebf3fc7bf0 against its parent. The follow-up #1943 carries no Brief trailer, so the rows do not select it. Checked by hand: the intake-desk skill is 537 lines on merged main as well, so the net ≤ 0 constraint still holds through #1943. The pr-review-desk skill is 1027 lines on merged main against 1023 at the delivering commit; that growth is from other changes, which the rows exclude by design (per the brief's Review section).
- Rows 7–9 resolve the delivering commit through the checkout's remote-tracking main ref, not through HEAD. In this run that ref was at 1f41825862a8, one commit ahead of the target (a board regeneration touching only STATUS.md), and 23ebf3fc7bf0 is an ancestor of both, so the result is the same at the target SHA.
- Row 10 depends on a statusgen binary on the runner's PATH. The witness used the installed v1.0.30, not one built from the target SHA.
- Row 10's one UNCHECKED entry (labels provisioned per repo, out-of-scope) is not evidence either way. Whether those labels exist on each repo in scope was not checked in this pass.
- The witness tool treats rows 4–9 and 13 as exit-status only, because their Expect cells hold no machine-decidable comparison. The hand run closes that gap for this pass; a later witness alone would not.

Could not check: label provisioning on the forge (offline envelope; outside the Verify table). Task step 1's scanner probe (the implementer's T1 row) is not a Verify row and was not repeated. The semantic walk of the worked triage example was done in the 2026-09-30 re-verify and was not repeated here; the intake-desk skill is unchanged in line count since then, but its content was not diffed against that run.

### Non-implementer verifier run — VERIFY: PASS — 2026-10-04 claude-opus-5-5-verifier

- Target: merged main d6662bbc6b4fc64be615baf14a010da126aef16a, in a detached worktree of the repo (the verifier's home worktree), offline (KUBECONFIG=/dev/null). Host: darwin. statusgen v1.0.31 as installed on the runner's PATH (not built from the target SHA).
- Brief frontmatter: gate: model; risk: regulatory no, customer no, irreversible no, sensitive-data no. No Verify row is risk-bearing (all four risk answers are no and no row touches a risk-classed path), so the per-row isolation rule for four or more risk-bearing rows does not apply.
- Grounding: the expected deliverables were written down from the brief text and main before any diff was read: the intake-desk step 1 class decision (attach, kind, incident-group, module:, only confirmed-defect and false-positive count), the trigger (3 counted instances or the 2nd merged fix; first instance in a brittle-marked module), the parking carve-out (production-down/security stay todo, bleed un-parks), the seven work-input labels and the worked-case heading, the pr-review-desk Recurrence-promotion paragraph routed to error-class, the worker-desk design-owed dispatch at strong tier with a brief as deliverable, net ≤ 0 lines per skill, and a changelog fragment. All are present on merged main.
- Delivering changes: #1876 (squash 23ebf3fc7bf0, carries the Brief trailer; the three skills and the changelog fragment) and #1943 (squash 8611850ab, the work-input amendment on the intake-desk skill, no Brief trailer). #2030 re-authored Verify rows 7–10 in the brief only.
- Every row ran by hand, each in a fresh bash -c at the worktree root, then through statusgen verifyrun (writing form, tool exit 0).

| Verify row discharged | Command | Expect | Observed | Result | Date | Runner |
|---|---|---|---|---|---|---|
| 1 | grep -cE 'error-class' on the intake-desk skill | ≥ 2 | exit 0; 2. At the delivering commit's parent b2c341cfd00e the count is 0, so the row is not vacuous | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 2 | grep -c 'confirmed-defect.*false-positive' / 'incident-group' on the intake-desk skill | ≥ 2 | exit 0; 6. At the parent: 0 | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 3 | grep -c '2nd merged fix' / 'second merged fix' on the intake-desk skill | ≥ 1 | exit 0; 1. At the parent: 0 | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 4 | sed the Recurrence-promotion paragraph of the pr-review-desk skill; grep error-class; negated grep -i guardrail-promotion | REDIRECTED | exit 0; REDIRECTED. The paragraph says to attach the finding as an instance to the open error-class issue, or record the class per intake-desk step 1. At the parent the same command exits 1 with no output | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 5 | grep -c '^func cmdAttach' in the deskfile source | 1 | exit 0; 1. Mechanism row: it also prints 1 at the parent, because the verb predates this brief. It proves the named verb exists, not that this brief changed anything; by design per the brief's Verify preamble | pass (mechanism row; vacuous with respect to this brief's own diff, by design) | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 6 | grep -c '"blocked": true' in statusgen checks.go | 1 | exit 0; 1. Mechanism row: also 1 at the parent, same reasoning as row 5 | pass (mechanism row; vacuous with respect to this brief's own diff, by design) | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 7 | intake-desk skill net line count, delivering commit vs its parent (row 7 as written) | NET-OK | exit 0; NET-OK. impl resolved to 23ebf3fc7bf0, base b2c341cfd00e; 537 lines before and 537 after. With no Brief-trailer commit the base-differs-from-tip guard fails the row, so it is not vacuous | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 8 | pr-review-desk skill net line count, same derivation (row 8 as written) | NET-OK | exit 0; NET-OK. base b2c341cfd00e, tip 23ebf3fc7bf0; 1023 before and after | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 9 | worker-desk skill net line count, same derivation (row 9 as written) | NET-OK | exit 0; NET-OK. base b2c341cfd00e, tip 23ebf3fc7bf0; 882 before and after | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 10 | throwaway shared clone inside the checkout, detached at 23ebf3fc7bf0; statusgen --consumers --brief build-less-brittle/04 --base 23ebf3fc7bf0~1; clone removed (row 10 as written) | exit 0; summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing | exit 0; summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing. CORROBORATED: the three fixed-here skill entries. UNCHECKED: the out-of-scope labels entry. Clone directory gone afterwards. The row reads the pinned delivering diff, not the tree under test, so it would give the same answer on a later main; rows 1–4 and 11–13 cover main's current text | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 11 | grep -c 'module:' && grep -c 'brittle' on the intake-desk skill | two counts, each ≥ 1 | exit 0; 2 then 4. At the parent: 0 and 0 | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 12 | grep -c 'production-down' && grep -c 'bleed' on the intake-desk skill | two counts, each ≥ 1 | exit 0; 1 then 1. At the parent: 0 and 0 | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |
| 13 | seven work-input labels by grep -qF plus the '### Work-input triage example' heading on the intake-desk skill | WORK-INPUT-FIELDS | exit 0; WORK-INPUT-FIELDS. Heading at line 335. At #1943's parent all seven labels and the heading are absent, so the row is not vacuous | pass | 2026-10-04 | claude-opus-5-5-verifier (assay-verifier-app[bot]) @ d6662bbc6b4f (on-behalf-of human:ian) |

statusgen verifyrun --check summary (tool exit 0):

docs/streams/build-less-brittle/brief-04-intake-by-error-class.md: 13 pass, 0 fail, 0 could-not-run/missing (of 13 Verify rows)

Execution witness (statusgen verifyrun, writing form, tool exit 0; copied verbatim):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -cE 'error-class' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:53c234e5e847 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c -e 'confirmed-defect.*false-positive' -e 'incident-group' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:06e9d52c1720 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -c -e '2nd merged fix' -e 'second merged fix' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `s=$(sed -n '/Recurrence-promotion/,/^$/p' plugins/assay/skills/pr-review-desk/SKILL.md); echo "$s" \| grep -q 'error-class' && ! echo "$s" \| grep -qi 'guardrail-promotion' && echo REDIRECTED` | pass exit=0 | sha256:8cfcff694556 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c '^func cmdAttach' tools/desk/cmd/deskfile/deskfile.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c '"blocked": true' statusgen/checks.go` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/intake-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/pr-review-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `impl=$(git log --first-parent --format=%H --grep='^Brief: build-less-brittle/04$' refs/remotes/origin/main -- . ':!docs/streams' ':!changelog' \| tail -1); base=${impl:+$impl~1}; base=${base:-$(git merge-base refs/remotes/origin/main HEAD)}; tip=${impl:-HEAD}; test "$(git rev-parse "$base")" != "$(git rev-parse "$tip")" && test "$(git show "$tip:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" -le "$(git show "$base:plugins/assay/skills/worker-desk/SKILL.md" \| wc -l)" && echo NET-OK` | pass exit=0 | sha256:458c4e39effe | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 10 | `d=$(mktemp -d "$PWD/.bl04-consumers.XXXXXX") && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q --detach 23ebf3fc7bf0 && statusgen --consumers --root "$d" --brief build-less-brittle/04 --base 23ebf3fc7bf0~1; s=$?; rm -rf "$d"; exit $s` | pass exit=0 | sha256:260242d7406f | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -c -e 'module:' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'brittle' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:7b90b6c82d45 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 12 | `grep -c -e 'production-down' plugins/assay/skills/intake-desk/SKILL.md && grep -c -e 'bleed' plugins/assay/skills/intake-desk/SKILL.md` | pass exit=0 | sha256:ad0fadf63cc7 | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |
| 13 | `f=plugins/assay/skills/intake-desk/SKILL.md; for key in mechanism known-scope source-revisions unresolved-questions next-action source-origin trust-disposition; do grep -qF "$key:" "$f" \|\| exit 1; done; grep -q '^### Work-input triage example$' "$f" && echo WORK-INPUT-FIELDS` | pass exit=0 | sha256:efb852f8dd4e | 2026-10-04 | assay-verifier-app[bot] @ d6662bbc6b4f+dirty (on-behalf-of human:ian) (forge-identity) |

Witness notes:
- The runner cell reads d6662bbc6b4f+dirty. The only change in the worktree when verifyrun ran was the verifier's untracked scratch directory; no tracked file differed from d6662bbc6, and the skill files the rows read are as merged.
- Hash cross-check: each witness hash equals the sha256 prefix of the hand run's output (rows 1–9 and 11–13 printed their Expect token or counts; row 10's hash 260242d7406f equals the hash of that row's combined stdout and stderr in the hand run). Rows 4–9 and 13 are stamped "expect: exit-status only" by the tool; the hand run shows each printed its exact Expect token.

Semantic read (not a Verify row): intake-desk step 1 on main carries the class decision, the attach block with the 2026-09-28 evidence-reference fields and the seven 2026-09-30 work-input labels, the rule that text held back by the trust gate stays data and never becomes an instruction, the trigger, the parking carve-out, the bleed un-park, the re-park after a scanner reactivate, and the intended-control route to the refusal-text owner. The Work-input triage example covers two mirrored reports as one counted instance, a quarantined quote kept as data and never a next-action, success on another revision as another scope rather than recovery, a same-scope re-check as the only recovery, and a separate failure as the only thing that advances the count. The worker-desk lane routes a design-owed class issue at strong tier with the deliverable its body line names, never code. Task step 5 (worker-desk) has no Verify row of its own; rows 9 and 10 cover its line budget and its consumers claim only.

Risk-bearing value enumeration, over the #1876 and #1943 skill diffs and the brief's Deliverables:
- design-owed counted-instance threshold = 3 @ plugins/assay/skills/intake-desk/SKILL.md:285
- design-owed merged-fix threshold = 2nd @ plugins/assay/skills/intake-desk/SKILL.md:286
- brittle-module threshold = first counted instance @ plugins/assay/skills/intake-desk/SKILL.md:286
- class-issue opening threshold = second symptom @ plugins/assay/skills/intake-desk/SKILL.md:282
- review recurrence threshold = three or more times across separate PRs @ plugins/assay/skills/pr-review-desk/SKILL.md:538
- new-issue budget defaultNewRate = 3 @ tools/desk/cmd/deskfile/deskfile.go:253 (named by the brief, unchanged by it)
- authoring line budgets ≤ 12, ≤ 8, ≤ 3, net ≤ 0 (brief Task text, not shipped values)

Rank: every entry is a reversible procedure knob in skill prose or a pre-existing budget. An edit and a plugin re-release undoes any of them; none moves funds, identity or published state. irreversible: no and every risk answer is no, so the fail-safe trigger does not fire. The top-ranked entries are the two design-owed thresholds:

RISK-VALUE: DERIVED — design-owed counted-instance threshold = 3 @ plugins/assay/skills/intake-desk/SKILL.md:285 — matches the stream's stated constraint, docs/streams/build-less-brittle/spec.md:113–114 (design-owed at 3 counted instances, distinct incident groups of kind confirmed-defect or false-positive) and brief Task step 3. The spec gives no first-principles case for 3 over 2 or 4; acceptable for a reversible prose knob.
RISK-VALUE: DERIVED — design-owed merged-fix threshold = 2nd @ plugins/assay/skills/intake-desk/SKILL.md:286 — matches spec.md:113–114 ("or at its 2nd merged fix, whichever comes first") and brief Task step 3.

rows_passed=13 rows_total=13

VERIFY: PASS

Could not check: label provisioning (error-class, design-owed) on each repo in scope, which is out of scope per the brief's consumers line and needs a live forge read (offline envelope). Task step 1's scanner probe (the implementer's T1 row) is not a Verify row and was not repeated. This is a model verdict on a gate: model brief with no risk flag; the verifier records Evidence and does not set the status.

## Review
Gate: model (from frontmatter). Rows 7–9 compare each skill at this brief's own change against
the same file just before it (the base is derived in the row; see the README's shared
conventions), so growth or shrinkage by other PRs on main never moves the bar.
