---
brief: statusgen/02
title: 'Issue metrics — statusgen --issues: standard counts + age/sitting-time + internal-vs-external + by-raising-desk'
wave: 1
depends: []
unblocks: ["statusgen/03"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-20 (authored clean for the statusgen board)
sources:
  - "A maintainer directive (2026-07-16): adjust the metrics to look at GitHub issues — standard metrics, plus which desk raised each, internal vs external, and how long each has been sitting"
  - "Follow-up: look at the TYPE of issue — a work-state issue (verify-gate) is different from a defect, and critical/high-priority defects should be measured separately"
  - "statusgen's existing DORA machinery already runs `gh issue list --state all --label bug` for the change-failure-rate numerator; this generalizes that fetch to all issues plus more fields"
  - "statusgen's existing issue-reading machinery this reuses"
  - "The intake untriaged-age alarm — the age/stale pattern this mirrors for issues"
  - "The `--dora --series` time-series/JSON idiom this matches"
  - "The escalation-label taxonomy (question / help wanted / needs-decision) this breaks down by"
why: >-
  GitHub issues are now a first-class part of the work model (an issue-loop desk raises and works
  them), but we measure PRs and briefs, not issues — so we can't see the front door's health: how many
  are open, how long they've been sitting, who raised them, and whether they're our own agents finding
  work or outside humans reporting problems. A maintainer wants standard issue metrics plus three cuts
  that matter operationally: **which desk raised it** (is one loop generating all the churn?),
  **internal-vs-external** (agent-found work vs a human/user reporting a bug), and **how long it has
  been sitting** (the same rot the intake-debt alarm catches, applied to issues).
---

# Brief 02 — Issue metrics (`statusgen --issues`)

## Context
files: `statusgen/` — a new **issues.go** (fetch + compute + render) reusing the `--dora` `gh` exec
pattern + the existing issue-reading helper; `main.go` (the `--issues` flag, `--json`, `--series`,
`--stale-issue-days`); `statusgen/issues_test.go` (planned). Output: human table by default, `--json`
machine form, `--series` weekly buckets — same idiom as `--dora`.

This brief DEFINES and READS the `raised-by:<desk>` label the by-desk cut groups on. A companion
effort (not in this brief) makes the filing desks stamp `raised-by:<desk>`; until they emit it, the
by-desk cut shows everything as **`unattributed`** — graceful degradation that is itself the signal,
like an "untagged" intake item. The self-improvement metric (statusgen/03) extends this mode's
classifier; a weekly DORA/roadmap artifact and the exec/retro reports later read its `--json` output.

facts (the metric set to compute, from `gh issue list --state all --json number,state,createdAt,closedAt,author,labels,title`
across the repo set — the same repos `--dora` covers):
- **Standard**: open / closed counts; close rate; **time-to-close** median + p90 (`closedAt −
  createdAt` over closed issues); breakdown **by label**.
- **By TYPE and SEVERITY — not one "bug" lump:** the label taxonomy carries
  different meanings and MUST be counted in distinct classes, because mixing them hides the signal
  (and, for `verify-gate`, actively distorts the DORA change-failure rate):
  - **Process states, NOT defects** — `verify-gate`, `live-verify`, `needs-decision`: these are
    *work-state* issues (a brief awaiting a human, a live-verify row, a decision fork), not things
    that broke. Count them as their own class and **exclude them from the "bug" totals** (and flag
    that `--dora`'s change-failure rate should use the *defect* count below, not raw `bug`-labelled —
    an over-count the productivity analysis already noted).
  - **Defects** — `bug`: real defects. This is the change-failure-relevant count.
  - **Severity within defects** — a defect ALSO carrying `critical` / `high-priority` (or a title
    flagged `URGENT` / `BLOCKER`, the escalation vocabulary) is broken out **separately**: its own
    count AND its own age/sitting-time, because a critical sitting 3 days is a different alarm than a
    normal bug sitting 3 days. Report `critical/high` distinct from `normal` defects.
  - **Everything else** — `question`, `help wanted`, `other` — reported per label.
  The metric emits: {state-class count} · {defect count, of which critical/high} · {other by label},
  never a single "issues" or "bugs" number that conflates them.
- **Age / sitting-time (OPEN issues)**: age = `now − createdAt`; a distribution in buckets
  **`<1d / 1–3d / 3–7d / >7d`**; the **oldest N** (number + age + title); and a **STALE-ISSUE ALARM** —
  a `--lint`-surfaced NOTICE + a board line when an open issue exceeds `--stale-issue-days` (default
  **7**), mirroring the intake-debt alarm. "How long it has been sitting" is this cut.
- **Internal vs external** (author classification, cheap — a team allowlist):
  - **agent** = author matches the automation account or a `*[bot]` login; **human** = anyone else.
  - **internal/team** = author ∈ the configured team set; **external** = any other login. Today
    everything is internal; the external bucket is how the first real user-reported issue is seen
    when it arrives.
  - Report BOTH axes (agent-vs-human, internal-vs-external) — they answer different questions
    ("are our loops finding the work?" vs "is anyone outside filing?").
- **By raising desk**: group by the **`raised-by:<desk>`** label (`raised-by:verify-desk`,
  `raised-by:issue-loop`, `raised-by:pr-review-desk`, `raised-by:batch-fanout`); no such label →
  **`unattributed`**. This is "which desk are they raised by."
- **The diagnostic banner rides the output** (same as `--dora`): metrics are per-project, for
  continuous improvement, never a target or a cross-team/individual scoreboard.
- Offline discipline: `--issues` needs `gh`; it MUST NOT become a hard dependency of the offline
  `--lint` gate — the stale-issue alarm degrades to "skipped (no gh)" offline, like the other
  gh-adjacent checks.

## Ground rules
- NEVER git push / trigger workflows beyond the standing branch+draft-PR flow.
- Stop at `implemented` — do not set verified/done.
- NEEDS_CONTEXT over guessing (esp. the team-allowlist default — confirm against current authors).

## Task
1. `statusgen/issues.go` (planned): fetch all issues across the repo set (one `gh issue list` per repo,
   the fields above), compute the four metric groups (standard / age / internal-external / by-desk),
   render a human table (default) + `--json` + `--series` (weekly `createdAt` buckets). Carry the banner.
2. `main.go`: `--issues` flag (+ `--json`/`--series` reuse, `--stale-issue-days` default 7,
   `--team-logins` default = the repo's automation account(s)).
3. The **stale-issue alarm**: emit a NOTICE from the lint/alarm path when an open issue exceeds the
   threshold (gh-guarded; skipped offline). One board line: `issue debt: N open, K over <days>d, oldest #<n> at <age>`.
4. Tests: time-to-close math, age bucketing, author classification (agent/human, internal/external),
   by-desk grouping incl. the `unattributed` fallback, banner presence.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --issues` | exit 0; prints open/closed counts, age buckets, internal/external, by-desk |
| 2 | `statusgen --root . --issues \| grep -i 'DIAGNOSTIC'` | ≥1 — the banner is present |
| 3 | `statusgen --root . --issues \| grep -iE -e unattributed -e raised-by` | ≥1 — the by-desk cut renders (unattributed until the desks stamp the label) |
| 4 | `statusgen --root . --issues --json \| jq -e '.open,.byDesk,.internal,.external,.ageBuckets,.byType,.defects.critical' >/dev/null` | exit 0 — JSON carries the cuts incl. type/severity |
| 4b | `statusgen --root . --issues \| grep -iE -e 'verify-gate' -e 'critical' -e 'defect'` | ≥1 — states, defects, and critical severity render as distinct classes (verify-gate is NOT counted as a bug) |
| 5 | `go test ./statusgen/ -run 'Issue' -count=1` | exit 0 |
| 6 | `statusgen --root . --lint` | exit 0 (stale-issue alarm is a NOTICE, gh-guarded) |

## Evidence
<!-- filled by a non-implementer at verify time -->

### Non-implementer verifier run — VERIFY: PASS — 2026-08-26 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `ea7fea5`
Runner != implementer. Own isolated worktree off `origin/main`, OFFLINE (`KUBECONFIG=/dev/null`). gate: model, all risk no. `statusgen/` module; go-test row ran from inside the module dir.

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | `statusgen --root . --issues` | open/closed, age buckets, internal/external, by-desk | exit 0 — open/closed + close-rate + time-to-close median/p90; age buckets; internal-vs-external + agent-vs-human; by-raising-desk all render | 2026-08-26 | opus-4.8[1m]-verifier |
| 2 | `statusgen --root . --issues | grep -i DIAGNOSTIC` | >=1 | exit 0 — the DIAGNOSTIC/Goodhart banner line | 2026-08-26 | opus-4.8[1m]-verifier |
| 3 | `statusgen --root . --issues | grep -iE -e unattributed -e raised-by` | >=1 | exit 0 — multiple raised-by:* lines + the unattributed bucket | 2026-08-26 | opus-4.8[1m]-verifier |
| 4 | `--issues --json | jq -e '.open,.byDesk,.internal,.external,.ageBuckets,.byType,.defects.critical'` | exit 0 | exit 0 — all seven JSON paths present and non-null | 2026-08-26 | opus-4.8[1m]-verifier |
| 5 | `--issues | grep -iE -e verify-gate -e critical -e defect` | >=1 | exit 0 — process-states (verify-gate/live-verify/needs-decision) held distinct from bug/defect totals | 2026-08-26 | opus-4.8[1m]-verifier |
| 6 | `go test -run Issue -count=1` (module dir) | exit 0 | exit 0 — TimeToClose, AgeBucketing, AuthorClassification, ByDeskGrouping, BotDetection, EmptyCorpusDegrades PASS | 2026-08-26 | opus-4.8[1m]-verifier |
| 7 | `statusgen --root . --lint` | exit 0 | exit 0 — LINT: PASS (stale-issue alarm a non-fatal NOTICE) | 2026-08-26 | opus-4.8[1m]-verifier |

**RISK-VALUE: DERIVED** — `defaultStaleIssueDays = 7` @ statusgen/issues.go:53 — reversible alarm threshold, flag-overridable via `--stale-issue-days`, matching the brief default and the existing intake untriaged-age cadence; a wrong value only shifts when a NOTICE fires. Age-bucket boundaries (1/3/7d), oldest-N=5, and the p90 percentile are reversible display knobs, rank last. No irreversible constant in scope.

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
