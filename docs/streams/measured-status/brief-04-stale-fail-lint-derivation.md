---
brief: assay:assay:measured-status:04
title: "statusgen --lint: derive stale-FAIL vs missing-card from commit dates, and route each state to the verify desk instead of nudging a worker to hand-file a sign-off"
why: >-
  A gate:human brief whose last Evidence entry is a VERIFY: FAIL that predates the merge that
  fixed the failing row draws a lint nudge that a worker reads as "hand-file a sign-off issue"
  — but that issue is not the verify-gate card, closing it flips nothing, and the brief keeps
  routing back to dispatch with an empty diff. The lint decides the state from the Evidence
  text alone; it should DERIVE whether the FAIL is stale by comparing the fix commits' dates to
  the Evidence date, and route a stale FAIL to a re-verify, never to a person's signature.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [862]
schema: brief-v2
authored: 2026-09-16 by measured-status scoping session
sources:
  - "#862 — a stale VERIFY: FAIL on a gate:human brief nudges workers to hand-file a sign-off"
  - "statusgen/brieffile.go — the gate:human-at-implemented lint nudge (the `no decision-issue` message)"
  - "statusgen/testdata — the per-state fixture trees the lint tests read"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — the nudge already names `--decision-issues` as the filer (`statusgen/brieffile.go:1549`); what it does NOT do is distinguish a canonical VERIFY: PASS-no-card state from a stale-FAIL state, so a stale FAIL still gets routed to the same file-a-decision-issue nudge instead of a re-verify"
exec-tier: strong
exec-tier-why: changes lint routing logic that steers worker vs verify-desk vs human effort; a mis-derived state sends work down the wrong lane, which is the exact failure the issue reports
domain: complicated
value: med
version: 1
id: 666a7cba-7992-475b-ac9c-e5435b6e4be8
---

# Brief 04 — Derive the stale-FAIL vs missing-card lint state

## Context
files:
- `statusgen/brieffile.go` — the gate:human-at-`implemented` lint nudge.
- `statusgen/testdata/` — add a fixture tree per state under the lint's testdata.
- `statusgen/*_test.go` — the lint test that reads the new fixtures.
facts:
- three states the lint must distinguish (per #862): (1) gate:human, last Evidence is a
  canonical VERIFY: PASS, no card -> "sign-off card missing — verify-desk lands the marker";
  (2) gate:human/gate:model, last Evidence is VERIFY: FAIL and the brief's `files:` paths or
  the cited fix issue have commits on main NEWER than the Evidence date -> "stale FAIL — a
  non-implementer RE-VERIFY supersedes it", naming the newest commit; (3) Evidence empty or the
  newest FAIL is current -> the existing nudge stays, retexted to say the decision issue is
  filed by --decision-issues (the tool), not a worker.
- statusgen is pure over the tree plus git-readable history; deriving "newer than the Evidence
  date" uses the same commit-history read attribution.go already performs, degraded three-state
  when history is unavailable (could-not-check, never rounded to "current").
- `statusgen` is its own Go module (`statusgen/go.mod`, no root `go.mod` in this repo); tests
  run from `statusgen/` (`cd statusgen && go test . …`), not from the repo root.
- the existing nudge text (`statusgen/brieffile.go:1549`) already names `--decision-issues` as
  the filer — state (3)'s message needs no retexting on that point; what changes is that state
  (2) (stale FAIL) gets its OWN, differently-worded message naming the newest fix commit and a
  re-verify route, so it is no longer indistinguishable from state (3)'s file-a-decision-issue text.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Split the single nudge into the three states above, deriving state (2) from a commit-date
   comparison (fix commits vs Evidence date), not from the Evidence text alone. Preserve the
   three-state read: an unavailable history is a could-not-check that keeps the conservative
   nudge, never a silent "current".
2. Leave state (3)'s message text as-is — it already names `--decision-issues` (the tool) as
   the filer, not a worker (`statusgen/brieffile.go:1549`); there is nothing to retext there.
   The actual gap is that state (2) currently reuses that same text instead of getting its own
   stale-FAIL message, so give state (2) a distinct message naming the newest commit and
   pointing at a re-verify, and confirm state (3)'s existing text is otherwise untouched.
3. Add one fixture tree per state under the lint's testdata and a test asserting state (2)'s
   new stale-FAIL message (naming the newest commit) is distinct from state (3)'s unchanged
   `--decision-issues` message, and that state (1)'s "sign-off card missing" text is distinct
   from both.
4. **Fail-first (rule 9).** Before landing, run the new test against the pre-change
   `brieffile.go` (single shared message, no date comparison) and confirm it fails to
   distinguish states (2)/(3); then against the fixed code and confirm it passes. Record the
   red-then-green run under `## Evidence`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1` | exit 0; output contains "ok" | check |
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'PASS'` | exit 0 (all three fixture-state assertions ran and passed) | check |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | exit 0 (a fixture tree exists per state) | check |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'stale FAIL'` | exit 0 (the stale-FAIL branch's message is exercised) | check |
| 5 | `cd statusgen && bash testdata/stalefail-mutation.sh` | exit 0; planted second route is detected | check +mutation |

## Evidence
Implementer validation (2026-10-05); independent verification remains pending.

- Fail-first against `40d0063ef`'s unmodified `brieffile.go`: the three-state
  test exited 1, reporting `want "stale FAIL"` and `want "sign-off card missing"`;
  both states still emitted `no decision-issue — file one via --decision-issues`.
  The history-route test also failed for model gates, linked cards, issue-linked
  fixes, missing history/dates, and superseded failures.
- Class guard positive control: a temporary second route in `planted_route.go`
  failed with `unclassified waiting-brief route in planted_route.go:plantedRoute`.
  The plant was removed; the fixed state and class tests then exited 0.

| Verify row | Implementer result |
|---|---|
| 1 | checked-clean: targeted three-state test, `-count=1 -timeout=60s`, exit 0 and `ok` |
| 2 | checked-clean: verbose test log contains `PASS`, grep exit 0 |
| 3 | checked-clean: stale-fail and missing-card fixture directory glob, exit 0; current-fail fixture also present |
| 4 | checked-clean: verbose test log contains `stale FAIL`, grep exit 0 |
| 5 | checked-clean: committed mutation script names the planted second route and exits 0 |

The unchanged fallback is retained for a current FAIL or empty Evidence. Day-only
Evidence compares against later UTC days, because same-day ordering is unknown.
History reads use the local remote-tracking main ref, falling back to local main;
missing or shallow history is reported as could-not-check without network access.
### Independent verification — VERIFY: FAIL — 2026-10-07

Non-implementer verifier, merged main e399dc816b72 (implementing PR #2238, merge 8e20f91571c9). Rows run by hand in plain bash and by the canonical witness (statusgen verifyrun v1.0.32, pipefail). Implementation behaviour is correct; rows 2 and 4 as authored exit 141 under the witness.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1` | exit 0; output contains "ok" | exit 0; "ok github.com/medici-finance/assay/statusgen 1.420s"; witness pass exit=0 | 2026-10-07 | assay-verifier-app[bot] @ e399dc816b72 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'PASS'` | exit 0 | plain bash exit 0; witness (pipefail) fail exit=141, reproduced 3 of 3 under bash -o pipefail: grep -q closes the pipe on first match and go test takes SIGPIPE. Content holds: same pipeline with grep -c exits 0 counting 5 PASS lines; subtests stale-fail, missing-card, current-fail all ran and passed | 2026-10-07 | assay-verifier-app[bot] @ e399dc816b72 (on-behalf-of human:ian) (forge-identity) |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | exit 0 | exit 0; matches stale-fail, missing-card fixture dirs and stalefail-mutation.sh; current-fail fixture also present; witness pass exit=0 | 2026-10-07 | assay-verifier-app[bot] @ e399dc816b72 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'stale FAIL'` | exit 0 | plain bash exit 0; witness (pipefail) fail exit=141, reproduced 3 of 3, same SIGPIPE cause as row 2. Content holds: verbose log carries "brief check/01 has stale FAIL — newest relevant main commit ed659a6 postdates Evidence 2026-08-01; a non-implementer RE-VERIFY supersedes it" | 2026-10-07 | assay-verifier-app[bot] @ e399dc816b72 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && bash testdata/stalefail-mutation.sh` | exit 0; planted second route detected | exit 0; "unclassified waiting-brief route in planted_route_ZIHC6K.go:plantedRoute" then "PASS: the planted second route was detected"; witness pass exit=0; tree clean after | 2026-10-07 | assay-verifier-app[bot] @ e399dc816b72 (on-behalf-of human:ian) (forge-identity) |

Extra checks (verifier's own, not Verify rows):
- TestFailHistoryRoutes + TestFailNoticeClass + TestStaleFailVsMissingCard together: exit 0.
- Non-vacuity mutation: rewording the stale-FAIL message in stalefail.go to "stale failure" turned row 1 red (exit 1, want "stale FAIL"); file restored, tree clean.

Risk-bearing values (risk metadata all no, irreversible: no; enumerated over the 8e20f915 diff to brieffile.go and stalefail.go):
- date.AddDate(0, 0, 1) @ statusgen/stalefail.go:111 — fix commit must land on a later UTC day than the Evidence date
- failHeadingRe = ^#{3,6}[ \t] @ statusgen/stalefail.go:12 — Evidence run-heading depth
- status set {in-progress, implemented, verified} @ statusgen/stalefail.go:164 — carried over unchanged from the old brieffile.go condition
- ref order {refs/remotes/origin/main, refs/heads/main} @ statusgen/stalefail.go:81
- is-shallow-repository must equal "false" @ statusgen/stalefail.go:77, else could-not-check
All are reversible lint routing knobs (edit + release). RISK-VALUE: DERIVED — date.AddDate(0, 0, 1) = 1 day @ statusgen/stalefail.go:111 — Evidence dates are day-granular, so same-day commits cannot be ordered against them. Calling only next-UTC-day-or-later commits stale follows the brief's "never rounded to current/stale on unknown" rule. A west-of-UTC same-local-day edge can over-call stale, but that routes to a re-verify, never to a sign-off (the safe lane).

Remedy for rows 2 and 4 (Verify-table defect, not code): drop -q (for example `... 2>&1 | grep 'PASS' >/dev/null`) or use grep -c, so the pipe is read to EOF; then re-verify.

VERIFY: FAIL — rows 2 and 4 exit 141 under the canonical pipefail witness (SIGPIPE from grep -q); the code meets the contract and rows 1, 3, 5 pass.

Desk cross-check: the brief and statusgen/ are unchanged between e399dc816b72 and current main 91f04b81ba06 (empty git diff), so this verdict holds at current main. Filed: #2241 (rows 2 and 4 SIGPIPE under the pipefail witness).

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
