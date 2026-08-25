---
brief: iso-9001/05
title: Records control and retention, stated once
why: >-
  Every property a records-control clause asks about is already true here and already
  machine-checked — registers are append-only, withdrawal is a tombstone and never a deletion,
  an entry that has ever existed on main and is absent from the working tree is a lint
  failure, duplicate IDs are detected, generated views have exactly one writer and a pull
  request touching them is refused. None of it is written down as a statement, so answering
  "which artifacts are records, who can change them, how would you know if one were altered,
  and how long are they kept" means reading source. And the last of those four has no answer
  at all: no artifact anywhere states a retention period or a disposition rule. Append-only
  forever is an implicit retention rule that nobody wrote down.
wave: 1
depends: ["iso-9001/02"]
unblocks: ["iso-9001/06"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-25 (authored for the iso-9001 board)
sources:
  - "`statusgen/registers.go` — `registerIntegrityProblems`: duplicate-id detection and the tombstone check against history, the two mechanisms that make an alteration visible."
  - "`docs/lifecycle.md` §'STATUS.md — a single-writer generated artifact' — main's CI is the only writer, branches never commit it, and pull-request CI blocks any diff that touches it. The reason given is conflict elimination; the records-control consequence is that a generated view cannot be edited into agreement with a claim."
  - "`docs/registers.md` §'Shared conventions' — append-only, a new entry is a new file, existing entry files are never edited away, 'these logs are the audit trail — a silent edit erases the record of why a decision was made'. Also the slug-ID rule and its explicit 'Do not claim gap detection'."
  - "`docs/evidence-bundle.md` — the export mechanism, its deterministic byte-identical output, its per-file sha256 manifest, its `omitted` array and its exit-3-means-INCOMPLETE contract. The retention statement must describe what an exported bundle is and is not."
  - "`docs/telemetry.md` §Retention — the in-tree precedent for how this repo states a retention position, including stating plainly that a policy is not yet stood up rather than implying one."
  - "`tools/freshness/` — the periodic-re-review mechanism: per-artifact `last-reviewed`, `max-age-days` and `upstreams`, plus optional per-claim anchors. The tool ships; the manifest it reads is the adopter's."
  - "depends iso-9001/02: `docs/streams/FINDINGS.md` states today that contiguity is enforced, which `docs/registers.md` forbids claiming. A records-control statement is the single most expensive document in which to repeat a false enforcement claim, so the source is corrected first."
  - "The standard-side reading: the documented-information clause names five controls — distribution and access, storage and preservation, control of changes, retention, and disposition — and asks that records be protected from unintended alteration. The split worth internalising is that documents are MAINTAINED and records are RETAINED; documents say what to do, records prove it was done. An auditor triages records control in about five minutes: can you find it, is it current, who approved it, and what happens at end of retention."
  - "freshness-checked 2026-08-25 @ 6871a3b (origin/main) — no records-control or retention statement exists; `docs/records-and-retention.md` (planned) is absent and the only page in the tree that discusses retention at all is the telemetry posture."
---

# Brief 05 — records control and retention

## Context

single-point-of-failure: this document becomes the thing people cite instead of reading the
source, which is its purpose and also its risk — a records-control page that drifts from the
machinery is worse than none, because it is believed. The countermeasure is that every claim
in it names the artifact that enforces it, so a reader can check any line in one command, and
the page is registered for periodic re-review rather than left to age.

files:
- `docs/records-and-retention.md` (planned) — the deliverable; one page.
- `docs/registers.md` — a pointer from the register conventions to the new page, so the two
  are found together.

facts:
- **Classify, then state per class.** Records here are not one thing. At minimum: register
  entries (findings, intake), briefs and their Evidence sections, stream READMEs with their
  Verified and Reviewed cells, generated views (`STATUS.md` and the reports), execution
  witnesses, released artifacts and their checksums, and exported evidence bundles. Each has
  a different writer, a different alteration-detection mechanism and a different lifetime.
- **State the retention rule that is already true, and label it as such.** Records live in
  git; git history is retained for the life of the repository; register entries are never
  removed because withdrawal is a tombstone; there is no deletion process and none is
  planned. That is a real retention rule — it has simply never been written down. Do **not**
  invent a period, a review cadence or a disposition process in this brief: inventing a
  policy in a documentation brief is how a document that describes a company nobody works at
  gets written, and it would be a claim this repo cannot honour.
- **Say plainly which questions are the adopter's.** An adopter's own retention obligation —
  a period, a disposition at end of period, a legal hold — is theirs, and this page must say
  so rather than appearing to supply it. The telemetry page's posture is the register to copy:
  state the position that exists, and state plainly where one does not.
- **Name the enforcement per claim.** Every property stated gets the artifact that enforces
  it, so the page is checkable rather than assertive: append-only and tombstone
  (`statusgen/registers.go`), single-writer generated views (the board workflow, which fails a
  pull request whose diff touches them), reference integrity and view drift, and the freshness
  mechanism for periodic re-review.
- **Distinguish detection from prevention, in the same breath as each claim.** Tombstone and
  duplicate-id detection make an alteration *visible* in a pull request; they do not make it
  *impossible*. A history rewrite by someone with the authority to force-push is out of scope
  of every mechanism named here, and the page must say so. An unstated limitation becomes the
  reader's problem.
- **An exported bundle is a copy, not the record.** The record is the tree at a commit; the
  bundle is a deterministic export of a date range, and it says what it could not collect. Do
  not let the page imply the bundle is the archive.
- **Register the page for re-review.** It is a document whose truth depends on code elsewhere,
  which is exactly the class the freshness mechanism exists for. Add it to the manifest the
  freshness tool reads, with the sources it depends on as upstreams.
- **No new check.** This brief adds no control, so it carries no mutation row; its Verify
  table is presence rows plus dereferencing rows against the machinery the page describes, and
  that boundary is stated in Review below.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Write `docs/records-and-retention.md` (planned)** with a per-class table: record class, where it
   lives, who may write it, how an alteration is detected, what enforces that, and how long it
   is kept. One row per class.
2. **State the retention rule as it stands** — retained for the life of the repository,
   register entries never removed, no deletion or disposition process — and label it as a
   description of current practice rather than an approved policy.
3. **State the boundary explicitly**: what makes an alteration *visible* versus what would
   make it *impossible*, and that a history rewrite by a sufficiently privileged actor is
   outside every mechanism the page names.
4. **State what is the adopter's own**: their retention period, their disposition at end of
   period, and any legal-hold obligation. Say it once, plainly, in the register the telemetry
   page uses.
5. **Cross-link from `docs/registers.md`** so a reader arriving at the register conventions
   finds the records statement, and vice versa.
6. **Register the page for periodic re-review** in the freshness manifest, with the source
   files whose changes would invalidate it as upstreams.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/records-and-retention.md` | exit 0 — **DEREFERENCE, inverts**: the page does not exist at authoring (2026-08-25 @ `6871a3b`) |
| 2 | `git grep -nF 'registerIntegrityProblems' -- statusgen/registers.go` | exit 0 — **DEREFERENCE**: the integrity function the page names really exists, so the enforcement claim is not a dangling reference |
| 3 | `git grep -n 'STATUS.md' -- .github/workflows/assay-statusgen.yml` | exit 0 — **DEREFERENCE**: the single-writer property the page attributes to the board workflow is checked against the workflow itself, not asserted |
| 4 | `git grep -n 'contiguity is enforced by statusgen' -- docs/streams/FINDINGS.md` | exit 1, no output — **DEREFERENCE**: iso-9001/02 has landed, so this page is not written on top of a claim the registers page forbids. If this row is green, stop: the dependency did not land |
| 5 | `git grep -cE -e 'Retention' -e 'retained' -- docs/records-and-retention.md` | exit 0; a non-zero count — the retention section exists and is named as such |
| 6 | `git grep -nF 'tombstone' -- docs/records-and-retention.md` | exit 0 — the page names the mechanism that makes a removal visible rather than only asserting append-only |
| 7 | `git grep -cE -e 'visible' -e 'impossible' -- docs/records-and-retention.md` | exit 0; a non-zero count — the detection-versus-prevention boundary is stated in the page, not left to the reader |
| 8 | `git grep -n 'records-and-retention' -- docs/registers.md` | exit 0 — the cross-link exists, so the two pages are found together |
| 9 | `cd tools/freshness && go test ./... -count=1` | exit 0 — **neighbour row (rule 17)**: the new manifest entry parses and the freshness tool's own suite is unaffected by it |
| 10 | `cd statusgen && go run . --root .. --lint` | exit 0 — the tree lints clean with the page in it, which includes the link check resolving every backticked path the page cites; a claim naming a file that does not exist fails here |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter — all four risk answers no). **This table gates the presence of
sections and the resolvability of the artifacts the page names, not the accuracy of its
prose** (`docs/brief-rules.md` rule 8) — row 10's link check catches a named file that does
not exist, and nothing catches a true-sounding sentence about a mechanism that behaves
differently. Reviewer records verdict + date in the stream README table. Reviewer questions
specific to this brief: (1) read each per-class row against the mechanism it names and confirm
the claim, particularly for the classes with no lint behind them; (2) does the page invent any
retention period, cadence or disposition process, rather than describing what is already true?
(3) is the detection-versus-prevention boundary stated per claim rather than once in a
footnote? (4) does it avoid implying that an exported bundle is the archive? (5) is the
adopter's own obligation stated plainly enough that nobody could read this page as supplying
it?
