---
brief: methodology/04
title: Findings demotion — unresolved finding on in-flight brief is a hard error
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (initiative-streams step 3)
sources: ["spec §11 adopt-3 (scope-change re-entry, from Advance)", "[F-02](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-agent-flow-observability-design-overlaps-frontend-txdetail-b.md) (live example: frontend/07)", "[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)", "[F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) / red-team-2026-07-09.md A4 (desk-ack amendment)"]
---

# Brief 04 — Findings demotion (scope-change re-entry)

## Context
files: tools/statusgen/checks.go (+tests); docs/streams/methodology/README.md conventions if wording needs it
facts:
- Today an unresolved finding only excludes the affected brief from Next-up (StaleRef).
- New rule: if the finding is **desk-acked** and the affected brief's README status is `in-progress` or `implemented`, that is a PROBLEM (exit 1) with message telling the operator to demote the brief to `todo` (re-gate) or resolve the finding. statusgen observes and blocks — it never rewrites READMEs (the observe/actuate split, spec §13).
- `todo`/`blocked` affected briefs stay as today: flagged + excluded, no error.
- Live precedent: [F-02](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-agent-flow-observability-design-overlaps-frontend-txdetail-b.md) currently affects frontend/07 (status todo — would NOT error). A finding filed against a brief mid-implementation is exactly the Advance re-entry case.
- AMENDED 2026-07-09 ([F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md), red-team A4): filing a finding is unverified input — an ungated demotion rule is a denial-of-service on in-flight work (anyone writes a paragraph, a rival brief drops to `todo`). The hard error therefore fires ONLY when the finding carries a desk acknowledgement line `Ack: YYYY-MM-DD <who>` (added to FINDINGS.md's documented format by this brief; the desk's on-record judgment that the finding is real). An unresolved finding WITHOUT an Ack targeting an in-progress/implemented brief keeps today's behavior (flag + Next-up exclusion) plus a visible non-fatal notice `unresolved F-NN awaits desk ack` — never exit 1.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Parse the optional `Ack: YYYY-MM-DD <who>` line in FINDINGS entries (between the Affects and Resolved lines). The format is already documented in FINDINGS.md's header (2026-07-09, PR #95 review follow-up) — implement the parser to match it.
2. In `check()` (after `applyFindings` logic is available — reorder if needed, keeping check-before-emit), add: **acked** unresolved finding targeting a brief with README status in-progress/implemented → PROBLEM `"<stream>/brief-<NN>: unresolved <F-NN> (desk-acked) — demote to todo (re-gate) or resolve the finding"`.
3. Unacked unresolved finding targeting in-progress/implemented → non-fatal notice `"<stream>/brief-<NN>: unresolved <F-NN> awaits desk ack"`; keep existing StaleRef/Next-up exclusion behavior unchanged for all statuses.
4. TDD fixtures: acked finding→in-progress (fail), acked finding→implemented (fail), unacked finding→in-progress (pass + notice), finding→todo (pass, flagged), resolved finding→in-progress (pass).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run TestDemotion -v` | exit 0; ≥5 subtests PASS (incl. both ack states; row amended 2026-07-09 per F-09) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --check` | exit 0 (F-02's target frontend/07 is todo → no violation) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Implementer run (records the implementation-time result; `verified` still needs an
independent re-run by a non-implementer):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestDemotion -v` | 0 | 6 `TestDemotion*` PASS (acked→in-progress fail, acked→implemented fail, unacked→in-progress pass+notice, todo flagged-only, resolved inert, malformed-Ack loud error) | 2026-07-09 | implementer (Fable) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | full suite ok; vet clean | 2026-07-09 | implementer (Fable) |
| 3 | `go run ./tools/statusgen --root . --lint` | 0 | branch-side gate per brief-15 (`--check` compares STATUS.md bytes and is main-only; this branch changes brief-04's status row so drift is by design). [F-02](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-agent-flow-observability-design-overlaps-frontend-txdetail-b.md)'s target frontend/07 is todo → no demotion problem, no notice, as the row predicts. `--check` re-runs on merged main by the verifier. | 2026-07-09 | implementer (Fable) |

Non-implementer re-run on merged main (`f6c3fdb6`) — satisfies the `verified` gate:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestDemotion -v` | 0 | 7 subtests PASS (AckedInProgress, AckedImplemented, UnackedInProgress, TodoFlaggedOnly, ResolvedInProgress, MalformedAck, ResolvedMalformedAckInert) | 2026-07-09 | opus-verifier |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | full suite ok; vet clean | 2026-07-09 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --check` | 0 | STATUS.md byte-matches on merged main ([F-02](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-08-agent-flow-observability-design-overlaps-frontend-txdetail-b.md) → frontend/07 todo → no demotion, as predicted) | 2026-07-09 | opus-verifier |
| 4 | `go run ./tools/statusgen --root . --lint` | 0 | PR-side gate green | 2026-07-09 | opus-verifier |

Review verdict (model:opus, 2026-07-09): the Ack gate is correct across acked / unacked / todo / resolved; the two PR #100 review findings (malformed-Ack double-message; resolved+malformed ordering) are provably fixed and pinned by negative tests. No blocking issues. REVIEW: PASS.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Human gate is MANDATORY when any risk answer is yes.
