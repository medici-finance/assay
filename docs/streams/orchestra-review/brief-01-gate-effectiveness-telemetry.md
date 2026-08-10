---
brief: orchestra-review/01
title: Gate-effectiveness telemetry — override rate, catch rate, ceremonial-gate detection
why: >
  Nothing measures whether the gates work. A human gate that reverses App-approved PRs weekly and
  a gate that has never fired in its life are both invisible in every metric we spec'd (#290 covers
  flow and cost, not trust or risk) — so "too heavy to sustain or too ceremonial to catch problems"
  is unanswerable except by feeling. Two observed incidents bound the failure directions: #280 (a
  forged human: token silently suppressing a gate) and #229 (detectors unwirable with the suite
  green). Catch-rate telemetry is the production twin of install-time mutation testing
  (brief-rules 16): mutation proves a gate CAN fire; this proves it DOES.
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
  - "freshness-checked 2026-08-01 @ 3128cd4 (grep docs/ statusgen/ for override-rate/gate-catch coverage: absent; #290 body re-read: its 5 metric families carry no trust/risk category)"
  - "assay-toolkit#307 + docs/research/orchestra-framework-for-assay.md §3.1 (the paper's trust/risk telemetry categories, p.13, are the two families every existing spec lacks)"
  - "assay-toolkit#290 (flow-report scoreboard spec this extends — NOT closed by this brief; if #290 lands first, fold these families into its implementation and retire this brief)"
  - "assay-toolkit#280, #229 (the two bounding incidents: suppressed gate, unwired detector)"
exec-tier: strong
exec-tier-why: "metric definitions are design decisions the facts don't fully pre-specify (a); correctness needs cross-artifact reasoning over audit.jsonl, PR review threads, findings register, and flip records (b)."
---

# Brief 01 — Gate-effectiveness telemetry

## Context
files:
- `statusgen/` (new `--gate-telemetry` subcommand, or extension of the #290 flow-report tool if that has landed — check first; on fold-in, amend the Verify rows to its invocation in the same commit, brief-rules 14)
- `statusgen/testdata/gatetelemetry/` (fixture cases: `override-one/`, `zero-fire-untested/`, `zero-fire-tested/`, `missing-audit/`)
- `docs/streams/orchestra-review/metric-definitions.md` (planned) — the metric-definitions table, Task 1's deliverable
facts:
- Data surfaces that already exist, per repo: PR review threads (App verdicts: `assay-reviewer-app[bot]` approvals/CHANGES_REQUESTED, `Security-Review:` verdicts), merge events, `audit.jsonl` (desk tool actions), the FINDINGS register + `bug`-labeled issues (post-merge defect discoveries), ready-flip records (deskpost).
- **Override rate** (trust family): (a) App-approved PRs subsequently human-rejected, reworked, or closed-unmerged; (b) merged PRs later named by a defect finding/bug issue (escaped-defect rate against an APPROVED verdict); (c) human-gate reversals of desk ready-flips.
- **Catch rate** (risk family): per gate class (App review verdict, security review, statusgen lint PROBLEM, corroborate, deskpost refusals, CI red) — fire count over the window, and of fires, how many blocked a real defect vs. noise (proxy: was the PR subsequently amended before merge, or the finding acknowledged). A gate with 0 fires over N windows → flagged **ceremonial-or-untested**, cross-referenced against whether it has a mutation-test row anywhere (brief-rules 16).
- Output shape: dated artifact per window (weekly), same cadence/home #290 specs (metrics-harvest style); three-state per metric — a surface that can't be read (API 403, missing audit file) reports could-not-check, never a zero.
- #290 is OPEN (decision-needed class). This brief does not implement #290's five families and does not close it. Coupling rule: whichever lands second folds into the first's tool/artifact rather than emitting a parallel report.
- Windows where a numerator is legitimately zero are expected (small fleet) — the report distinguishes "0 fires, mutation-tested elsewhere" from "0 fires, never proven able to fire"; only the latter alarms.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write the metric definitions as data at `docs/streams/orchestra-review/metric-definitions.md` (planned)
   (a table: metric, family, numerator, denominator, source surface, three-state semantics) —
   definitions above are the starting point, refine against what the surfaces actually record.
2. Implement collection for the desk-worked repos (compiled-in repo set, same discipline as
   `deskkit.allowedRepos`): read-only over the surfaces in facts; emit the dated artifact.

   **Scope amendment (2026-08-07, PR #349 review finding 8 — brief-rules 14).** Delivered here:
   collection from `audit.jsonl`, the ONE surface in `facts` that has a real producer
   (`tools/desk/internal/deskkit.Log`). It is read in that producer's native `Entry` schema, and
   a line that is not a `deskkit.Entry` reports `could-not-check` rather than an absence of
   events. This closed a live defect, not a hypothetical one: the reader previously expected an
   invented `{event, gate, blockedDefect}` shape no producer emits, so the real desk audit log
   (428 deskpost refusals, 43 ready-flips) rendered as `fires=0 … ceremonial-or-untested` at
   exit 0.

   **Deferred, and NOT delivered by this brief:** the GitHub-sourced collector for
   `pr-verdicts.json` (PR review threads), `defect-findings.json` (FINDINGS register +
   `bug`-labeled issues), and `gates.json`, plus the dated-artifact emitter (output is stdout
   only). Those three surfaces remain hand-authored windows; the Verify table below is
   fixture-only and cannot detect their absence, so it is recorded here instead. Owner: a
   follow-up brief in this stream — `orchestra-review/03`, to be authored. Until it lands, every
   claim this tool makes about the GitHub-sourced metrics is a claim about the computation over
   an assumed input shape, and `docs/streams/orchestra-review/metric-definitions.md`
   § "Collection status" is the standing record of which surfaces are real.
3. Ceremonial-gate detection: enumerate gate classes, count fires per window, cross-reference
   zero-fire gates against mutation-test presence; alarm only on "zero fires AND never
   mutation-tested".
4. If #290's flow-report has landed by pickup time: implement these families inside that tool
   instead of a new one (check first — see coupling rule in facts).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go run . --gate-telemetry --root testdata/gatetelemetry/override-one` | exit 0; stdout contains `override-rate` with numerator `1` naming the fixture's PR `#101` (App-approved-then-human-rejected) |
| 2 | `cd statusgen && go run . --gate-telemetry --root testdata/gatetelemetry/zero-fire-untested` | exit 0; stdout contains `ceremonial-or-untested` (positive control: 0 fires, no mutation-test marker) |
| 3 | `cd statusgen && go run . --gate-telemetry --root testdata/gatetelemetry/zero-fire-tested \| grep -c ceremonial-or-untested; go run . --gate-telemetry --root testdata/gatetelemetry/zero-fire-tested \| grep -c proven-able-to-fire` | first grep = 0; second grep ≥ 1 (mutation-tested zero-fire gate is NOT flagged) |
| 4 | `cd statusgen && go build -o /tmp/gate-telemetry . && /tmp/gate-telemetry --gate-telemetry --root testdata/gatetelemetry/missing-audit; echo "exit=$?"` | stdout contains `could-not-check` for the audit-sourced metrics (not a `0`); `exit=3` (distinct from clean 0, checked-failed 1, and the usage-error 2 the flag package and statusgen's own arg errors already use; built binary because `go run` does not propagate exit codes) |
| 5 | `grep -c 'could-not-check\|ceremonial-or-untested' docs/streams/orchestra-review/metric-definitions.md` | exit 0; ≥ 1 (three-state + ceremonial semantics are in the definitions, not just code) |
| 6 | `cd statusgen && go run . --gate-telemetry --root testdata/gatetelemetry/override-one > /tmp/gt-a.txt && go run . --gate-telemetry --root testdata/gatetelemetry/override-one > /tmp/gt-b.txt && diff /tmp/gt-a.txt /tmp/gt-b.txt` | exit 0 (deterministic; no wall-clock leakage into the window's rows) |

Pinned by this table (implementer keeps these names or amends the rows in the same commit,
brief-rules 14): subcommand `--gate-telemetry`; fixture root `statusgen/testdata/gatetelemetry/`
with cases `override-one` (contains approved-then-rejected PR `#101`), `zero-fire-untested`,
`zero-fire-tested`, `missing-audit`; output tokens `override-rate`, `ceremonial-or-untested`,
`proven-able-to-fire`, `could-not-check`; could-not-check exit code `3` (0 = checked-clean,
1 = checked-failed per `docs/three-state-instrument-rule.md`; 2 is already taken by usage errors —
the flag package's parse-error exit and statusgen's own arg errors). If these families fold
into #290's tool, the invocation changes but the fixtures, tokens, and expectations carry over.

Rows 1–4 gate behavior on fixtures the implementer authors; live-window numbers are unassertable
in CI. Row 5 gates presence of semantics the deliverable itself adds; quality of the metric
*definitions* is the review gate's, not grep's (brief-rules 8).

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model. Reviewer MUST confirm rows 2 and 4 actually fire (a gate-effectiveness instrument
that itself fails open would be the joke the system doesn't need), and that the brief did not
implement a second parallel report if #290's tool already landed.
