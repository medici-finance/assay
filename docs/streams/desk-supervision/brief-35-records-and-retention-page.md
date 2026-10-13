---
brief: assay:assay:desk-supervision:35
title: Records and retention page lists every analysis record and what is never recorded
why: >-
  The page that says which records Assay keeps, who writes them and how long they last lists
  seven classes and misses the ones the desk already writes (verify outcomes, the desk audit log,
  the Decide journal, review-finding blocks) and every record the pipeline-analysis briefs add.
  An adopter cannot judge what the tools keep about their work, or what they promise never to
  keep, from a page that omits most of it. This brief makes the page the one index: each record
  class, where it lives, its writer, a link to its schema, its retention, and a plain list of
  what is never recorded.
wave: 2
depends: ["desk-supervision/28", "desk-supervision/29", "desk-supervision/30", "desk-supervision/31", "desk-supervision/32", "desk-supervision/33", "desk-supervision/34"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-06 by an ad-hoc authoring session for the driver (pipeline analysis records)
exec-tier: any
domain: clear
outcome: none
sources:
  - "docs/records-and-retention.md:12-22 @ 1fbf1153f — the Record classes table (seven rows: registers, briefs+Evidence, README cells, generated views, execution witnesses, released artifacts, evidence bundles); no row for verify outcomes, the audit log, the Decide journal or review-finding blocks"
  - "docs/telemetry.md @ 1fbf1153f — counts-only opt-in telemetry contract; §Retention (lines 80-87) does not link the records page"
  - "docs/adopting-assay.md @ 1fbf1153f — no link to docs/records-and-retention.md or docs/telemetry.md anywhere in the file (grep 2026-10-06)"
  - "desk-supervision/27 (separate draft pull request #2287) — adds join keys to verify-outcome records; its fields are documented in verify-wake-v1.md, which this page links rather than restates"
  - "desk-supervision/28 to desk-supervision/34 — the analysis records this page indexes; each brief names its record's location, writer and schema home"
  - "freshness-checked 2026-10-06 @ 1fbf1153f (origin/main)"
consumers:
  - "docs/records-and-retention.md (Record classes table + new never-recorded section): follow-up desk-supervision/35 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/telemetry.md (§Retention gains one link to the records page): follow-up desk-supervision/35 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/adopting-assay.md (one sentence after the §2 Component inventory table): follow-up desk-supervision/35 (this brief; flips to fixed-here when the implementation edits the path)"
  - "the schema homes the page links (verify-wake-v1.md, review-finding-v1.md, decide.md, tools/desk/README.md, spec/registers-v1.md, and the homes briefs 28-34 create): out-of-scope (linked, never edited — they stay the single source for their field lists)"
---

# Brief 35 — Records and retention page lists every analysis record and what is never recorded

## Context
files:
- **edit** `docs/records-and-retention.md` — new rows in the `## Record classes` table; a new
  `## What is never recorded` section; a one-line pointer in the intro that the page indexes
  records and their schema homes hold the field lists.
- **edit** `docs/telemetry.md` — §Retention: one sentence linking `records-and-retention.md`.
- **edit** `docs/adopting-assay.md` — one sentence directly after the §2 Component inventory
  table linking `records-and-retention.md` (what the tools record about the adopter's work).
- **add** `changelog/desk-supervision-35.md` (planned).

facts:
- The existing table (`docs/records-and-retention.md:14-22` @ 1fbf1153f) has the columns
  Class | Where it lives | Who may write it | How an alteration is detected | Enforced by |
  Retention. Keep those columns; the "Where it lives" cell carries the schema-home LINK.
- Existing records the table omits, with their schema homes (all verified to exist @ 1fbf1153f;
  links below are relative to `docs/`):
  - **Verify-outcome records** — one JSON file per outcome under
    `docs/streams/verify-outcomes/<stream>/` (in the repo), written by
    `deskevidence --outcome-record`; layout `docs/streams/verify-outcomes/README.md`, fields
    `docs/streams/desk-supervision/verify-wake-v1.md`. Records are immutable; a correction is a
    new record. desk-supervision/27 adds join-key fields to that same schema doc.
  - **Desk audit log** — `~/.config/assay/audit.jsonl` plus daily rotated segments
    `audit.jsonl.<YYYY-MM-DD>` on the operator's machine, never in the repo (`deskDir()`,
    `tools/desk/internal/deskkit/killswitch.go:65-80`; `auditPath()`,
    `tools/desk/internal/deskkit/audit.go:88-94`). Writer: `deskkit.Log`, called by every desk
    tool. Schema home: `tools/desk/README.md` §"Runtime state (NOT created by this repo)"
    (lines 357-375), which states "Nothing is deleted … no tool removes a segment".
  - **Decide journal** — today `AuditJournal` writes each `DecisionRecord` into the audit log as
    a `decide` verb line with the record flattened into `detail`
    (`tools/desk/internal/deskkit/decide.go:504-519`); the raw context is stored only as a digest
    (`decide.go:223-224`). Contract: `tools/desk/internal/deskkit/decide.md`; assessment spec:
    `spec/decision-assessment-v1.md`. desk-supervision/29 tees it: the structured sidecar is ADDED and the audit
    `decide` line stays as it is, so the row lists both locations and notes that earlier history
    sits only in the audit log.
  - **Review-finding blocks** — a versioned block inside an HTML comment in forge review/reply
    bodies (on the forge, not in the repo), written by `deskpost review` and `deskreply`; schema
    `docs/streams/desk-supervision/review-finding-v1.md`. Retention is the forge's.
- Records the analysis set adds — each sibling brief's `## Context` names the record's location,
  writer and schema home; read them there, do not guess: desk-supervision/28 (dispatch record),
  29 (structured Decide journal + observed outcome), 30 (review round record), 31 (CI check
  history), 32 (human decision record), 33 (intake exit record — the INTAKE register's
  disposition, `spec/registers-v1.md` §5), 34 (worker usage counts — destination adopter-
  configured, never the public repo).
- Not a record, do not list as one: desk-tools/23's local perf record (no `deskperf` command and
  no perf file in `tools/desk` @ 1fbf1153f); statusgen telemetry (no receiver, nothing stored —
  `docs/telemetry.md` §"No receiver yet"; the page already links it).
- Known content exceptions the "never recorded" section must name truthfully rather than hide:
  the audit log `Entry` carries the item `title` (`audit.go:85`) and a free-text `detail`; the
  dispatch stamp's forge labels already expose the model slug as `dispatched-<slug>` beside a
  tier label (`tools/desk/internal/deskkit/modelstamp.go:248-258`); review-finding blocks carry
  the reviewer's finding prose by design (that block is the review itself).
- The page's retention stance (`docs/records-and-retention.md:24-39`) forbids inventing a
  period, cadence or disposition step no mechanism runs. Each new row's Retention cell states
  what the mechanism actually does ("life of the repository", "no tool removes it — the
  operator's machine", "the forge's", or "adopter-configured").
- `statusgen --lint` link-checks every `*.md` under `docs/**` (`statusgen/linkcheck.go:216-235`),
  so a broken relative link on the page already fails CI; Verify row 2 is the independent second
  check, scoped to this page.
- single-point-of-failure: the schema homes stay the only field lists — backed by Verify row 5
  (the page restates no field names) and row 2 (every link resolves).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Existing records.** Add four rows to the `## Record classes` table: verify-outcome records,
   desk audit log, Decide journal, review-finding blocks — each with location, writer, how an
   alteration is detected, what enforces it, retention, and a link to its schema home from
   `facts:`. Name fields only by linking; never copy a field list or field names into the page.
2. **Analysis records.** For each of desk-supervision/28–34, read the merged brief's `## Context`
   and add one row (or, for 29 and 33, update the row the record extends) linking the schema home
   that brief created. Cite the brief ID in the row (e.g. "added by desk-supervision/31") so a
   reader can trace it. Note in the verify-outcome row that desk-supervision/27 extends its schema.
   If a sibling brief has not merged, or names no schema home, stop and report NEEDS_CONTEXT.
3. **Join key.** One short paragraph under the table: records join on the brief ID, the PR
   (`repo#N` + head commit) and `dispatch_ref` as defined by desk-supervision/28; link that
   brief's schema home for the definition rather than restating it.
4. **Never recorded.** Add `## What is never recorded`, scoped to the analysis records, with these
   items: raw PR, issue or comment text and human reply prose (digests only); session
   transcripts, prompts and tool output; per-person latency or token metrics (aggregates per
   decision class, brief or tier only); vendor model names (tier slugs only — and say that the
   existing `dispatched-<slug>` forge label already exposes the model slug); and that no record
   is a target for ranking people or agents. Then name the known exceptions from `facts:` (audit
   log title and detail, review-finding prose, the dispatch label) plainly.
5. **Inbound links.** Add the one-sentence links in `docs/telemetry.md` §Retention and in
   `docs/adopting-assay.md` after the §2 Component inventory table.
6. **Changelog.** Add `changelog/desk-supervision-35.md` (planned) (bullets: the page now indexes every
   analysis record; the never-recorded list).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `statusgen --lint --root .` | exit 0 (link-check over `docs/**` passes) | check:ci |
| 2 | `cd docs && n=0 && for l in $(grep -oE '\]\([^)#]+' records-and-retention.md); do l=${l#??}; case "$l" in http*) continue;; esac; n=$((n+1)); if ! test -e "$l"; then echo "MISSING $l"; exit 1; fi; done; echo "resolved=$n"; test "$n" -ge 20` | exit 0; prints `resolved=` with a value of 20 or more (12 links today + at least 4 existing schema homes + 7 analysis-record homes, deduplication aside) and no `MISSING` line | check +dereference |
| 3 | `cd docs && cp records-and-retention.md "${TMPDIR:-/tmp}/b35-mut.md" && printf '\n[x](streams/no-such-record-home.md)\n' >> "${TMPDIR:-/tmp}/b35-mut.md" && for l in $(grep -oE '\]\([^)#]+' "${TMPDIR:-/tmp}/b35-mut.md"); do l=${l#??}; if ! test -e "$l"; then echo "MISSING $l"; fi; done` | output contains `MISSING streams/no-such-record-home.md` (the row 2 loop goes red on a planted dead link) | check +mutation |
| 4 | `cd docs && for p in streams/verify-outcomes/README.md streams/desk-supervision/verify-wake-v1.md ../tools/desk/README.md ../tools/desk/internal/deskkit/decide.md ../spec/decision-assessment-v1.md streams/desk-supervision/review-finding-v1.md ../spec/registers-v1.md desk-supervision/27 desk-supervision/28 desk-supervision/29 desk-supervision/30 desk-supervision/31 desk-supervision/32 desk-supervision/33 desk-supervision/34; do if ! grep -qF "$p" records-and-retention.md; then echo "ABSENT $p"; exit 1; fi; done; echo all-present` | exit 0; prints `all-present` | check |
| 5 | `! grep -n -e rows_passed -e blocker_kind -e wake_schema -e argsDigest -e bodyDigest -e contextDigest -e evidenceHead -e originHead docs/records-and-retention.md` | exit 0, no output (the page restates no schema field names) | check |
| 6 | `grep -n '^## What is never recorded' docs/records-and-retention.md && for w in transcript per-person 'tier slug' digest ranking 'dispatched-' title; do if ! grep -qiF "$w" docs/records-and-retention.md; then echo "ABSENT $w"; exit 1; fi; done; echo never-list-ok` | exit 0; prints the heading line and `never-list-ok` | check |
| 7 | `grep -qF '](records-and-retention.md' docs/telemetry.md && grep -qF '](records-and-retention.md' docs/adopting-assay.md && echo both-link` | exit 0; prints `both-link` — the adopter-facing pages reach the index, and row 2 proves the index reaches every schema home: the two together walk adoption page → records page → schema home | check +flow |
| 8 | `statusgen --consumers --root .` | exit 0 (every `consumers:` routing is corroborated by the implementation diff) | check:ci |
| 9 | `cmd: grep -n -A8 'func deskDir' tools/desk/internal/deskkit/killswitch.go` then the reviewer reads three new table rows and, for each, opens the code the Writer and Location cells name (for example `tools/desk/internal/deskkit/killswitch.go` `deskDir()` for the audit log path, `tools/desk/cmd/deskevidence/outcomerecord.go` for the verify-outcome writer), and checks each Retention cell against the mechanism it names | verdict records, per row checked, that location, writer and retention match the code; any retention period not backed by a mechanism fails the row | gate:model +dereference |

Pre-mortem → detection map:

| Failure mode | Caught by |
|---|---|
| A schema-home link points at a path that does not exist (typo, a sibling brief renamed its file) | row 2, backed by row 1's link-check; row 3 proves row 2 can go red |
| An existing or new record class is left off the page | row 4 |
| Field lists copied into the page, then drift from the schema home | row 5 |
| "Never recorded" section missing an item, or hides the known exceptions | row 6 |
| Telemetry and adoption pages still do not point at the page | row 7 |
| Writer, location or retention cell is wrong but well-formed (e.g. claims the audit log lives in the repo, or invents a retention period) | row 9 |
| A sibling brief (28–34) not yet merged, so its row would cite a home that does not exist | Task step 2 stop rule + row 2 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
