---
brief: orchestra-review/02
title: Docs cold-boot reconstruction drill — scheduled detection for the docs-vs-reality drift class
why: >
  "Could a new manager reconstruct the workflow from documentation?" is the ORCHESTRA paper's one
  operational test we demonstrably fail: boot lists drifted both directions (#258), topology lives
  in 5+ parallel hand-maintained tables (#276), an 11-doc drift bundle (#279) — every instance
  found INCIDENTALLY. One human and one machine hold the working knowledge (#291); until the docs
  pass a cold boot, the bus factor of the process is 1 regardless of what the runbooks claim.
  A repeatable drill converts this drift class from incidental discovery to scheduled detection.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-01 by Fable session (ORCHESTRA-review authoring pass, issue #307)
sources:
  - "freshness-checked 2026-08-01 @ 3128cd4 (grep docs/ for cold-boot/reconstruction-drill: absent; #291 body re-read: its dry-run drills credential/infra rebuild, not process/docs reconstruction)"
  - "assay-toolkit#307 + docs/research/orchestra-framework-for-assay.md §3.2 (the paper's reconstruction test, p.12)"
  - "assay-toolkit#258, #276, #279 (the drift class this detects); #291 (the infra-layer complement — NOT closed by this brief)"
---

# Brief 02 — Docs cold-boot reconstruction drill

## Context
files:
- `docs/` (new: drill protocol doc + first-run report; suggested `docs/drills/`)
facts:
- Drill = a fresh agent session with ONLY the target repos' checked-in docs — no memory files, no
  session context, no human coaching — attempting, in order: (a) name the desks/roles and how each
  boots; (b) reconstruct the pipeline topology (who hands to whom, via what durable state); (c) walk
  one item end-to-end on paper: file → brief → implement → review → verify → close, naming every
  gate and who may flip it; (d) name the identities/Apps involved and which gates each may satisfy.
- Scoring: each step ends checked-clean (docs sufficed), checked-failed (docs wrong/contradictory —
  cite doc vs. reality), or could-not-check (docs silent; tribal knowledge required). Every
  checked-failed / could-not-check becomes a filed issue (dedupe against #258/#276/#279 and
  open drift issues first — the drill will rediscover known drift; link, don't refile).
- Isolation: the drill agent is read-only over the repos and writes nothing but its report
  (dispatch per the isolation boilerplate; it must not "fix" what it finds).
- The drill agent's tier: cheap is fine and arguably better — a strong model papers over doc gaps
  with priors, which is exactly the signal being measured. Protocol should pin this choice.
- Cadence: protocol doc proposes one (monthly-ish, or after any topology-changing merge); the
  standing schedule itself is a #285-class decision — record the proposal, don't self-authorize
  a cron.
- Report filename convention (pinned for Verify): `docs/drills/cold-boot-run-<NN>.md` (first run
  = `cold-boot-run-01.md`). The protocol MUST require every checked-failed / could-not-check row
  to carry its issue link or dedupe link (`#NNN`) on the same line — Verify row 5's mechanical
  check depends on that formatting.
- Complement boundary: #291's deliverable (2) drills rebuild-from-zero of credentials/infra on a
  new machine; this drill assumes a working machine and tests whether the PROCESS is reconstructable
  from docs. Different layers; a finding belonging to the other layer gets routed to #291, not
  absorbed here.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- ONE sanctioned remote mutation: Task 3's issue filing. File only after deduping against open
  drift issues (#258/#276/#279 and neighbors — the #156/#157 duplicate-filing class is the
  hazard); when an existing issue covers the finding, link it in the report instead of refiling.
  The DRILL AGENT itself files nothing — it only reports; the operator (you) files.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write the drill protocol doc: the four reconstruction steps, three-state scoring, read-only
   isolation requirements, drill-agent tier pin, issue-filing/dedupe rules, cadence proposal.
2. Run the drill once (dispatch the cold agent; you are the operator, not the drill subject) against
   the desk-worked doc surface (this repo + the oit desk skills — the drill READS oit, it does not
   modify it).
3. Land the first-run report beside the protocol: per-step three-state results with citations,
   and the list of issues filed (or dedupe links to existing drift issues).
4. Seeded-defect control: before the run, plant one deliberate false claim in a THROWAWAY COPY of
   one doc given to the drill agent (never in the repo); confirm the drill surfaces it. Records the
   drill can actually detect drift (brief-rules 16 spirit — an instrument proven able to fire).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/drills/cold-boot-protocol.md && grep -c 'checked-clean\|checked-failed\|could-not-check' docs/drills/cold-boot-protocol.md` | exit 0; ≥ 3 (three-state scoring is in the protocol) |
| 2 | `grep -ci 'read-only\|must not fix' docs/drills/cold-boot-protocol.md` | exit 0; ≥ 1 |
| 3 | `grep -c 'checked-clean\|checked-failed\|could-not-check' docs/drills/cold-boot-run-*.md` | exit 0; ≥ 1 (the glob resolves — report exists with three-state rows; a clean pass is legal, a missing report is not) |
| 4 | `grep -i 'seeded' docs/drills/cold-boot-run-*.md \| grep -ci 'detected'` | exit 0; ≥ 1 (report names the seeded-defect control and marks it detected — positive control) |
| 5 | `ls docs/drills/cold-boot-run-*.md >/dev/null && ! grep -h -e checked-failed -e could-not-check docs/drills/cold-boot-run-*.md \| grep -vE '#[0-9]+'` | exit 0 (report exists AND every checked-failed / could-not-check row carries an issue or dedupe link `#NNN` on the same line — formatting mandated by the protocol, see facts; the `ls` guard keeps the row red while the deliverable is absent) |
| 6 | `for f in docs/drills/cold-boot-run-*.md; do grep -qF 'Harness preload:' "$f" \|\| exit 1; done` | exit 0 (every run report records the harness-preload caveat field — a bare denial no longer satisfies the protocol) |
| 7 | `for f in docs/drills/cold-boot-run-*.md; do grep -qF 'Seed artifact:' "$f" \|\| exit 1; done; for p in $(sed -n 's/.*Seed artifact:.*\(docs\/drills\/seeds\/[A-Za-z0-9._-]*\).*/\1/p' docs/drills/cold-boot-run-*.md); do test -f "$p" \|\| exit 1; done` | exit 0 (every run report records a seed artifact, AND any committed seed path it names actually exists — this is what stops a run claiming a positive control it never planted) |
| 8 | `for f in docs/drills/cold-boot-run-*.md; do grep -qF -e 'Seed mode:** replace' -e 'Seed mode:** supplement' "$f" \|\| exit 1; done` | exit 0 (seed mode pinned to the two legal tokens, so supplement-vs-replace runs are comparable) |

Rows 1-4 gate presence and are passed by construction once the docs exist (they are greps
over documents the author just wrote) — disclosed, not hidden. Row 5 is a line-scoped
mechanical proxy for link-coverage. Rows 6-8 are the falsifiable ones: each one goes red on
a report that omits the field, and row 7 goes red on a report that names a seed file that
was never committed. Every row 6-8 was checked against a negative control that must fail
(see Evidence). Whether the reconstruction judgments are honest is still the review gate's
(brief-rules 8).

Note on running rows 1-5: their `\|` sequences are markdown-escaped pipes. Rows 1, 3 and 4
must be run with the backslash intact — `grep -c 'checked-clean\|checked-failed\|...'` is
BRE alternation, and stripping the backslash turns it into a literal pipe that matches
nothing and exits 1. Rows 6-8 avoid alternation and pipes entirely for this reason.

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->
<!-- Implementer's own run below, for reference only; a non-implementer must independently re-run
     this table before `verified`. -->

| # | Result (implementer run, 2026-08-01) |
|---|---|
| 1 | `grep -c ...` on docs/drills/cold-boot-protocol.md → `7`, exit 0. PASS |
| 2 | `grep -ci 'read-only\|must not fix'` → `3`, exit 0. PASS |
| 3 | `grep -c ...` on `docs/drills/cold-boot-run-*.md` → `13`, exit 0. PASS |
| 4 | `grep -i seeded ... \| grep -ci detected` → `3`, exit 0. PASS |
| 5 | `ls ... && ! grep -h -e checked-failed -e could-not-check ... \| grep -vE '#[0-9]+'` → no unlinked rows found, exit 0. PASS |
| 6 | exit 0. PASS. Negative control: a run report with no `Harness preload:` line → exit 1 |
| 7 | exit 0. PASS. Negative controls (run in a scratch tree, not this repo): report with no `Seed artifact:` line → exit 1; report naming a seeds path whose file does not exist → exit 1; same report once that file is created → exit 0 |
| 8 | exit 0. PASS. Negative control: a report with `Seed mode:** whatever` → exit 1 |

All 8 rows pass. `cd statusgen && go run . --root .. --lint` also exit 0 (pre-existing NOTICEs
only, unrelated to this brief).

## Review
Gate: model. Reviewer MUST confirm:
- the seeded control was planted per the protocol's pinned mode, and that the run report's
  `Seed artifact:` line either names a committed seed the reviewer can read, or says
  `unrecoverable` (legal only for run 01, which predates the requirement);
- the run report records a `Harness preload:` caveat rather than a denial, and that the steps it
  caveats are the ones the preload would plausibly answer;
- rows 6-8 fail on a report missing the field — do not accept them on a green run alone.
