---
brief: methodology-metrics/28
title: 'Issue metrics — statusgen --issues: standard counts + age/sitting-time + internal-vs-external + by-raising-desk'
why: >-
  GitHub issues are now a first-class part of the work model (the issue-loop desk raises and works
  them), but we measure PRs and briefs, not issues — so we can't see the front door's health: how many
  are open, how long they've been sitting, who raised them, and whether they're our own agents finding
  work or outside humans reporting problems. human:<name> wants standard issue metrics plus three cuts that
  matter operationally: **which desk raised it** (is one loop generating all the churn?),
  **internal-vs-external** (agent-found work vs a human/user reporting a bug), and **how long it has
  been sitting** (the same rot the intake-debt alarm catches, applied to issues).
wave: 3
depends: ["methodology-metrics/16"]
unblocks: ["methodology-metrics/29"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-16 by Opus desk session (human:<name> directive)
sources: ["human:<name> 2026-07-16: 'adjust our metrics to look at github issues. standard metrics around them would be good. ideally measure which desk are they being raised by, internal/external etc, as well as how long they have been sitting there'", "human:<name> 2026-07-16 (follow-up): 'look at type of bug — verify-gate is different than bugs, and high-priority/criticals should be measured separately' → the by-type/severity cut (states vs defects vs critical/high)", "tools/statusgen/dora.go (already runs `gh issue list --state all --label bug --json number,createdAt` for the CFR numerator — this generalizes that fetch to all issues + more fields)", "tools/statusgen/scanissues.go (the existing issue-reading machinery this reuses)", "issue-loop/07 (intake untriaged-age alarm — the age/stale pattern this mirrors for issues)", "methodology-metrics/16 (--dora --series — the time-series/JSON idiom this matches)", "methodology-metrics/27 (weekly DORA artifact — a consumer: gains an issue-metrics section)", "CLAUDE.md bug-tracking rule + escalation labels (question/help wanted/needs-decision — the label taxonomy this breaks down by)", "freshness-checked 2026-07-16 @ 8a916655 (no `statusgen --issues` mode exists; dora.go fetches only number,createdAt for bug issues; authors on the repo today are the-org + *[bot]; no raised-by label convention exists — brief-29 adds it)"]
gate-why: not human-gated — a read-only reporting mode over public GitHub issue metadata; no regulatory/customer/irreversible/sensitive-data surface. Diagnostic, carries the same "not a target / not a scoreboard" banner as --dora.
---

# Brief 28 — Issue metrics (`statusgen --issues`)

## Context
files: `../assay-toolkit/statusgen/` — a new **issues.go** (fetch + compute + render) reusing dora.go's `gh` exec
pattern + scanissues.go's issue reading; `main.go` (the `--issues` flag, `--json`, `--series`,
`--stale-issue-days`); `../assay-toolkit/statusgen/issues_test.go` (planned). Output: human table by default, `--json`
machine form, `--series` weekly buckets — same idiom as `--dora`.

consumers (author-brief rule 6 — the shared value is **the issue-metric definitions + the
`raised-by:<desk>` label the by-desk cut reads**):
- **methodology-metrics/29** (raised-by stamping) — the filing desks stamp `raised-by:<desk>`; this
  brief DEFINES that label + reads it, brief-29 makes the desks emit it. Until 29 lands, the by-desk
  cut shows everything as **`unattributed`** (graceful degradation — itself the signal, like intake
  "untagged").
- **methodology-metrics/27** (weekly DORA artifact) — gains an issue-metrics section built from this
  mode's `--json`. Cross-referenced; the wiring lands with 27's implementation.
- **the retro / exec artifacts** (mm/25) — consume the age + internal/external cuts.

facts (the metric set to compute, from `gh issue list --state all --json number,state,createdAt,closedAt,author,labels,title`
across the repo set — the same repos --dora covers):
- **Standard**: open / closed counts; close rate; **time-to-close** median + p90 (`closedAt −
  createdAt` over closed issues); breakdown **by label**.
- **By TYPE and SEVERITY — not one "bug" lump (human:<name> 2026-07-16):** the label taxonomy carries
  different meanings and MUST be counted in distinct classes, because mixing them hides the signal
  (and, for `verify-gate`, actively distorts the DORA CFR):
  - **Process states, NOT defects** — `verify-gate`, `live-verify`, `needs-decision`: these are
    *work-state* issues (a brief awaiting a human, a live-verify row, a decision fork), not things
    that broke. Count them as their own class and **exclude them from the "bug" totals** (and flag
    that `--dora`'s CFR should use the *defect* count below, not raw `bug`-labelled — an over-count
    the productivity analysis already noted).
  - **Defects** — `bug`: real defects. This is the CFR-relevant count.
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
  **7**), mirroring issue-loop/07's intake-debt alarm. "How long it has been sitting" is this cut.
- **Internal vs external** (author classification, cheap — a team allowlist):
  - **agent** = author is `the-org` or matches `*[bot]`; **human** = anyone else.
  - **internal/team** = author ∈ {`the-org`, `human:<name>`, `*[bot]`} (config: `--team-logins`, default
    that set); **external** = any other login. Today everything is internal; the external bucket is
    how we'll see the first real user-reported issue when it arrives.
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
- NEVER git push / trigger workflows / mutating kubectl beyond the standing branch+draft-PR flow.
- Stop at `implemented` — do not set verified/done.
- NEEDS_CONTEXT over guessing (esp. the team-allowlist default — confirm against current authors).

## Task
1. `../assay-toolkit/statusgen/issues.go` (planned): fetch all issues across the repo set (one `gh issue list` per repo,
   the fields above), compute the four metric groups (standard / age / internal-external / by-desk),
   render a human table (default) + `--json` + `--series` (weekly `createdAt` buckets). Carry the banner.
2. `main.go`: `--issues` flag (+ `--json`/`--series` reuse, `--stale-issue-days` default 7,
   `--team-logins` default `the-org,human:<name>`).
3. The **stale-issue alarm**: emit a NOTICE from the lint/alarm path when an open issue exceeds the
   threshold (gh-guarded; skipped offline). One board line: `issue debt: N open, K over <days>d, oldest #<n> at <age>`.
4. Tests: time-to-close math, age bucketing, author classification (agent/human, internal/external),
   by-desk grouping incl. the `unattributed` fallback, banner presence.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `statusgen --root . --issues` | exit 0; prints open/closed counts, age buckets, internal/external, by-desk |
| 2 | `statusgen --root . --issues \| grep -i 'DIAGNOSTIC'` | ≥1 — the banner is present |
| 3 | `statusgen --root . --issues \| grep -iE -e unattributed -e raised-by` | ≥1 — the by-desk cut renders (unattributed until mm/29) |
| 4 | `statusgen --root . --issues --json \| jq -e '.open,.byDesk,.internal,.external,.ageBuckets,.byType,.defects.critical' >/dev/null` | exit 0 — JSON carries the cuts incl. type/severity |
| 4b | `statusgen --root . --issues \| grep -iE -e 'verify-gate' -e 'critical' -e 'defect'` | ≥1 — states, defects, and critical severity render as distinct classes (verify-gate is NOT counted as a bug) |
| 5 | `go test ./tools/statusgen/ -run 'Issue' -count=1` | exit 0 |
| 6 | `statusgen --root . --lint` | exit 0 (stale-issue alarm is a NOTICE, gh-guarded) |

## Evidence
<!-- filled by a non-implementer at verify time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
