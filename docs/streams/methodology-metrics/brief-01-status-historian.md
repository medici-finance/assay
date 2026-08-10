---
brief: methodology-metrics/01
title: Status-transition historian — statusgen logs every brief status change with a timestamp
wave: 0
depends: []
unblocks: ["methodology-metrics/02", "methodology-metrics/03", "methodology-metrics/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by desk (methodology-metrics scoping — the critical-path head)
sources: ["[I-08](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-scada-ooda-industrial-control-lineage-for-the-methodology.md) (SCADA/OODA — the historian)", "docs/streams/methodology/scada-ooda-lineage.md", "2026-07-09 velocity read (lead time uncomputable)"]
---

# Brief 01 — Status-transition historian

## Context
files: `../assay-toolkit/statusgen/` (add a historian: parse + append), `../oit/.github/workflows/status-regen.yml`
(main's single-writer CI is where the append happens), a new append-only log
`docs/streams/.history.jsonl` (or `../assay-toolkit/statusgen/history/` — decide in-brief, keep it git-tracked so
it survives).
facts:
- statusgen today renders **point-in-time** state — a brief's *current* status, never *when* it changed.
  The Verified/Reviewed cells hold free-text dates but there is no machine transition record.
- Without a transition log, **Change Lead Time (implemented→done), `--trend`, and age-based alarm KPIs
  are all uncomputable** — this brief is the substrate the rest of the stream stacks on.
- The log must be **append-only and single-writer** (main's CI), same discipline as STATUS.md — a
  branch must not write it (it would conflict / be forgeable). It records transitions observed *on main*.
- `new Date()` / wall-clock is available in CI (Go `time.Now()`); the append happens in the regen job.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. In statusgen, compute each brief's current `(stream/NN → status)` and **diff against the last recorded
   state in the history log**; for every changed brief, append a record `{ts, brief, from, to, sha}`
   (ISO-8601 ts, the two statuses, the main commit sha). First run seeds the current state (from=`""`).
2. Wire the append into **main's status-regen CI only** (single writer) — a new `statusgen --record`
   (or fold into the regen path) that runs after STATUS.md regenerates and commits the appended log
   with `[skip-status-regen]`. `--lint` on PRs must NOT write it.
3. Make the log the queryable source for downstream briefs: a small Go reader (`history.Load()`) other
   statusgen subcommands import. Keep the format stable + documented in a header comment.
4. Idempotent: re-running with no status changes appends nothing.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 (incl. new historian diff/append tests) |
| 2 | seed a testdata history, flip a brief `implemented→verified`, run `statusgen --record --root <testdata>` | one new JSONL line `{...,"from":"implemented","to":"verified",...}` appended; re-run appends nothing |
| 3 | `statusgen --lint` (real tree) | exit 0; `--lint` does NOT modify the history log |
| 4 | `go vet ./tools/statusgen/` | exit 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `0c9e0bce`; tests re-confirmed at `927d8107`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/` | 0 | `ok` — incl. `TestRecordAppendsTransitionAndIsIdempotent`, `TestRecordFirstRunSeedsEveryBrief`, `TestDiffHistory*`, `TestLoadHistory*`, `TestAppendHistory*` (re-ran green at 927d8107) | 2026-07-09 | opus-verifier |
| 2 | `statusgen --record --root <testdata>` after flipping `hist/01` implemented→verified | 0 | appended exactly one line `{"ts":...,"brief":"hist/01","from":"implemented","to":"verified",...}` (2→3 lines); re-run "history: no status changes — nothing appended" (idempotent) | 2026-07-09 | opus-verifier |
| 3 | `go run ./tools/statusgen --lint` (real tree) | 0 | exit 0; `.history.jsonl` sha256 identical before/after, `git status` clean — `--lint` does NOT write the log | 2026-07-09 | opus-verifier |
| 4 | `go vet ./tools/statusgen/` | 0 | clean | 2026-07-09 | opus-verifier |

**VERIFY: PASS** — status-transition historian appends on `--record` only (single-writer, main CI), idempotent, seeds first run; `--lint` never writes. Re-confirmed green after statusgen changes landed at 927d8107.

## Review
Gate: model (all risk no). Reviewer records verdict + date in the stream README. Confirm the log is
append-only + single-writer (a PR branch cannot forge a transition), mirroring STATUS.md's discipline.
