---
brief: methodology-metrics/10
title: Verification-debt as a first-class alarm — Awaiting-queue depth/ratio on the board
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by Fable session (assay-review-1)
sources: ["docs/assay-review-1/README.md (B-06, U-04)", "STATUS.md 2026-07-09: 24 briefs in Awaiting verification / review vs 8/111 done — the queue is 3× total completions", "verify-desk skill (the queue is its worklist; nothing signals when it grows faster than it drains)", "methodology-metrics/06 (EEMUA-191 overflow-as-alarm — the sibling pattern, applied to Next-up; this applies it to the Awaiting queue)"]
---

# Brief 10 — Verification-debt as a first-class alarm

## Context
files: tools/statusgen/emit.go (+ small check/notice wiring) + tests; docs/streams/methodology-metrics/README.md
facts:
- Merging is not completion (the methodology's own rule), and eligibility gates on deps reaching
  `done`/`verified` — so the Awaiting queue is the throughput valve for every wave behind it. On
  2026-07-09 it held 24 briefs against 8 total done; the one-line verdict of the day-2 review was
  "verification debt is the current critical path." The board renders the queue as rows and says
  nothing about its SIZE, growth, or that it is the constraint.
- SCADA discipline (the stream's lens): a value without an alarm limit isn't monitored, and a
  breach must be an ALARM, not a table the operator may or may not scroll.
- Age percentiles per queue row need the transition historian (mm/01, implemented — data starts at
  its landing). OUT of scope here; mm/03's trend view owns time-series. This brief ships what
  current state supports: depth, composition, ratio, threshold.
- Anti-gaming (stream rule): this is a diagnostic, never a target — the alarm prompts "drain the
  queue / dispatch verifiers," not "stop marking implemented."

## Ground rules
- NEVER push to main / trigger workflows / run mutating kubectl. Feature-branch push + draft PR per
  the [I-12](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-pr-review-loop-agent-run-branch-push-draft-pr-desk-owned-rea.md) loop is the sanctioned flow; leave other commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. The Awaiting-queue heading gains its numbers: `## Awaiting verification / review (N — M at
   implemented, K verified awaiting review)`.
2. A configurable depth threshold (default 10; one constant, documented like batchSize/perStreamCap).
   When N exceeds it OR N exceeds the total done count, emit a `--lint`/run NOTICE (non-fatal):
   `verification debt: N awaiting vs D done — the queue is the constraint; drain before dispatching
   new implementation work (methodology-metrics/10)`.
3. Tests: fixture over/under threshold; heading counts; NOTICE text stable.
4. README: one line under conventions — depth/ratio here, age/trend behind mm/01 via mm/03.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `(rc=0; for t in Await Debt; do out=$(go test ./statusgen/ -count=1 -run "$t" -v 2>&1); tr=$?; { [ $tr -eq 0 ] && printf '%s' "$out" \| grep -q -- '--- PASS'; } \|\| { echo "MISSING-OR-FAIL $t"; rc=1; }; done; exit $rc)` | exit 0, prints nothing — both named test groups EXIST and pass (a `-run` pattern that matches nothing exits 0 with "no tests to run", so existence is asserted from `--- PASS`); over/under-threshold + heading-count subtests PASS. Exit status is captured (`tr=$?`) and asserted BEFORE the `--- PASS` check, so a FAILING test in the group also goes red — the previous pipeline form discarded `go test`'s status and passed on a red suite |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --lint` | exit 0, and on the current tree (queue ≈ 24 > 10) stderr/stdout carries the verification-debt NOTICE |
| 4 | regenerate locally (do NOT commit STATUS.md): | Awaiting heading shows `(N — M at implemented, K verified awaiting review)` matching the table's row count |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `f483c052`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run 'Await\|Debt' -v` | 0 | TestAwaitingHeadingCounts + TestDebtNotice PASS | 2026-07-10 | opus-verifier |
| 2 | `go test ./tools/statusgen/ && go vet` | 0 | ok | 2026-07-10 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --lint` | 0 | NOTICE: `verification debt: 25 awaiting vs 37 done — the queue is the constraint (methodology-metrics/10)` | 2026-07-10 | opus-verifier |
| 4 | regen — Awaiting heading shows counts | 0 | `## Awaiting verification / review (25 — 24 at implemented, 1 verified awaiting review)` | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — verification-debt alarm + Awaiting-heading counts render.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
