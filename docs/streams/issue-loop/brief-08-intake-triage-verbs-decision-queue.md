---
brief: issue-loop/08
title: 'Intake triage verbs + decision queue — the four exits from `new`, and `decision-needed` routes into the single needs-decision queue'
wave: 3
depends: ["issue-loop/06"]
unblocks: ["issue-loop/09"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-12 by Fable desk session (I-intake-desk)
sources: ["I-intake-desk (the intake-desk design — this is its verb vocabulary + decision-queue half)", "human:<name> 2026-07-12: 'we need to create an ''intake-desk'' similar to the ''issue-desk'' — as intake requests come in we need to work on them to get them into briefs/issues/flagged as decisions etc etc.'", "issue-loop/06 (needs-decision label + decision-issue template — the SINGLE decision queue this routes into)", "methodology-metrics/11 (gate-queue priority — decision issues join the human queue it orders)", "[I-28](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-loop-monitoring-dashboard-a-wip-website-over-the-standing.md) (loop-monitoring dashboard — the WAITING-ON-HUMAN panel that renders the open decision set)", "CLAUDE.md bug-tracking rule (new bugs → GitHub Issues, label bug)", "tools/statusgen/registers.go + migrate.go (the vocabulary/view being extended)", "freshness-checked 2026-07-12 @ ab92e96e (registers.go:172 vocabulary is `new | watching | scoped → <stream> | rejected — <why>`; no decision-needed value exists)"]
why: >-
  Nothing OWNS the transition out of `disposition: new`, and the worst leak is strategy calls
  that are human:<name>'s: today they sit as ambient `new` indistinguishable from unread ideas. Naming
  the four triage exits — and giving human-blocked entries a first-class `decision-needed`
  route into ONE decision queue — turns "half the front door is untriaged" into a drained
  register plus an explicit humans-blocking list ("very explicitly highlight what is waiting
  on humans and why").
---

# Brief 08 — Intake triage verbs + decision queue

## Context
files: `../assay-toolkit/statusgen/migrate.go` (`intakeEntry` field + parse), `../assay-toolkit/statusgen/registers.go`
(vocabulary line + view rendering) + tests; `docs/streams/issue-loop/README.md` (conventions)
facts:
- **The four exits** (every entry leaves triage as exactly ONE of; a reason is never optional):
  1. `scoped → <stream>` — becomes a brief via the author-brief flow. **Tier gate: triage only
     QUEUES authoring** — brief authoring is design-tier work (author-brief model-tier gate);
     a cheap-tier triage session never authors inline.
  2. `scoped → issue #NN` — operational/bug-shaped work → GitHub issue (label `bug` when
     bug-shaped, per the CLAUDE.md bug-tracking rule); the entry records the issue number.
     The existing parser already handles this generically (`scoped → X` ⇒ ScopedTo="issue #NN")
     — this is a documented convention, not a code change.
  3. `decision-needed` — a call that is human:<name>'s. NEW disposition value; machinery below.
  4. `rejected — <why>` / `watching` — existing vocabulary, explicit reason required.
- **Single decision queue (the design call, per I-intake-desk):** the decision QUEUE is the
  `needs-decision` GitHub-issue set that issue-loop/06 owns — flipping an entry to
  `decision-needed` REQUIRES filing (or already having) a `needs-decision` issue per 06's
  template, recorded in a new optional frontmatter field `decision-issue: <NN>`. The intake
  view renders these entries as a distinct section — but that section is a POINTER into 06's
  queue, never a second queue: one place human:<name> answers decisions (mm/11 orders it; [I-28](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-loop-monitoring-dashboard-a-wip-website-over-the-standing.md)'s
  WAITING-ON-HUMAN panel renders it).
- **statusgen changes**: (a) `intakeEntry` gains `decision-issue` (optional int/string, keep
  YAML key `decision-issue`); (b) `generateIntakeView` renders `decision-needed` entries in a
  distinct "Decision queue — waiting on a human" section at the TOP of the generated INTAKE.md
  (they are the humans-blocking list), each with `Disposition: decision-needed → issue #NN`;
  (c) the view's format line (registers.go:172) gains the new value; (d) a non-fatal NOTICE
  when a `decision-needed` entry lacks `decision-issue:` — advisory, because the entry may be
  flipped moments before the issue is filed, but it must not stay that way.
- View output stays byte-deterministic (entries sorted by id within each section) — the
  single-writer/main-CI discipline is unchanged.
- consumers (rule 6 — this changes a shared vocabulary the register machinery reads):
  issue-loop/07 sensor (keys on literal `new` — a `decision-needed` entry correctly leaves the
  untriaged count; no change needed there), issue-loop/06 (its label/template is the queue —
  its `unblocks` already names this brief, amended at authoring time; 06 was `todo`, no
  demotion), issue-loop/09 (the desk cadence that speaks these verbs), `--migrate` legacy parser
  (migrate.go `parseIntakeDisposition` — extend for symmetry so a round-trip doesn't downgrade
  the value), INTAKE.md view readers (regenerated by main CI on merge).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. statusgen: add `decision-needed` to the disposition vocabulary per facts — field, parse
   (frontmatter + legacy `Disposition:` line symmetry), view rendering with the distinct
   top section, format-line update, missing-`decision-issue` NOTICE.
2. Tests: parse `decision-needed` (+ `decision-issue`); view renders the decision section
   distinctly and deterministically; entry without `decision-issue` NOTICEs; `scoped → issue
   #NN` round-trips; existing dispositions unchanged.
3. README (this stream): a "Triage verbs" conventions block — the four exits verbatim, the
   queue-only tier gate for authoring, and the single-queue rule (decision-needed ⇒
   needs-decision issue via issue-loop/06; the intake section is a pointer, not a queue).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/... -count=1` | exit 0; includes the Task-2 cases |
| 2 | `statusgen --root . --lint` | exit 0 standalone |
| 3 | `go test ./tools/statusgen/ -run 'Decision' -v` | exit 0; decision-section rendering + missing-issue NOTICE subtests PASS |
| 4 | `grep -n "Triage verbs" docs/streams/issue-loop/README.md && grep -n "decision-needed" docs/streams/issue-loop/README.md` | exit 0 (conventions block landed, names the single-queue rule) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Non-implementer verifier run (glm-5.2-verifier, merged main `5dfcb04b`, 2026-07-16). All 4 rows RUN.

| # | Command | Exit | Key output |
|---|---------|------|------------|
| 1 | `go test ./tools/statusgen/... -count=1` | 0 | `ok …/statusgen 2.257s` |
| 2 | `go run ./tools/statusgen --root . --lint` | 0 | advisory NOTICEs only; `verification debt: 31 awaiting vs 100 done` |
| 3 | `go test ./tools/statusgen/ -run 'Decision' -v` | 0 | `TestDecisionNeeded*` all PASS (view, no-decision-section, NOTICE incl. missing-issue, deterministic, parse, legacy-parser symmetry) |
| 4 | `grep -n "Triage verbs" docs/streams/issue-loop/README.md && grep -n "decision-needed" …` | 0 | "Triage verbs (issue-loop/08)" header L126; single-queue rule named L147 |

VERIFY: PASS.

## Review
Gate: model. Reviewer confirms (a) there is ONE decision queue — the view section points at
needs-decision issues and renders no independent state, (b) the view stays byte-deterministic,
(c) the missing-`decision-issue` NOTICE is advisory (exit code unchanged), (d) the tier gate
is stated as queue-only (triage never cheap-authors).
