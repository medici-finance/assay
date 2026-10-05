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
### Non-implementer verifier run — 2026-10-06 claude-opus-5-5[1m]

Verified SHA: `fc5afe37f9e48681b39007ef694ac73fc4ef562e` (merged main; implementing commit 8e20f9157, #2238). Detached clean checkout, offline (KUBECONFIG=/dev/null, GOPROXY=off), go1.27.1 darwin/arm64.

**Grounding (written from the brief text before reading the implementer's tests or Evidence).** Expected: (1) gate:human, last Evidence a canonical VERIFY: PASS, no card gives a "sign-off card missing" notice routed to verify-desk; (2) gate:human or gate:model, last Evidence a VERIFY: FAIL, with a commit on main touching the `files:` paths or citing the fix issue that is newer than the Evidence date, gives a distinct "stale FAIL" notice naming the newest commit and a re-verify route; (3) empty Evidence or a current FAIL keeps the existing "no decision-issue — file one via --decision-issues" text unchanged; unavailable history is could-not-check and keeps the conservative nudge. One fixture tree per state; a test named TestStaleFailVsMissingCard asserting the three messages are distinct; a mutation script that detects a planted second route. The merged code and fixtures match this: the single routing point is waitingBriefNotice in statusgen/stalefail.go, called from statusgen/brieffile.go, with fixture trees stale-fail, missing-card and current-fail under statusgen/testdata.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1` | pass exit=0 | `ok github.com/medici-finance/assay/statusgen 0.626s`. With -v, 3 subtests ran and passed (stale-fail, missing-card, current-fail). Note: an exit 0 with "ok" alone would also be printed for a nonexistent test name ("[no tests to run]"); this check confirmed the test exists and asserts | 2026-10-06 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (claude-opus-5-5[1m]) |
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'PASS'` | fail exit=141 as written under the witness harness (pipefail); exit 0 in a plain shell | Reproduced 5 of 5 times with bash -o pipefail: exit 141, because grep -q exits on its first match and the go command then gets SIGPIPE writing its trailing `ok` line. Plain shell: exit 0. The test itself passes (log ends `--- PASS: TestStaleFailVsMissingCard`, `PASS`, `ok`). The row is also weak as a check: grep 'PASS' matches the "PASS" printed for a run with no matching tests, and any single passing subtest | 2026-10-06 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (claude-opus-5-5[1m]) |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | pass exit=0 | Glob resolves to statusgen/testdata/stale-fail, statusgen/testdata/missing-card and statusgen/testdata/stalefail-mutation.sh. The third fixture tree, statusgen/testdata/current-fail, is also present (the glob does not check it) | 2026-10-06 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (claude-opus-5-5[1m]) |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'stale FAIL'` | fail exit=141 as written under the witness harness (pipefail); exit 0 in a plain shell | Same SIGPIPE mechanism as row 2, 5 of 5 times with pipefail. The same pipeline with `grep 'stale FAIL' >/dev/null` instead of `grep -q` exits 0 under pipefail. The observed line is a real notice: `brief check/01 has stale FAIL — newest relevant main commit ed659a6 postdates Evidence 2026-08-01; a non-implementer RE-VERIFY supersedes it`. The grep alone would also match the failure text `want "stale FAIL"`, so only row 1's exit 0 rules out a red run | 2026-10-06 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (claude-opus-5-5[1m]) |
| 5 | `cd statusgen && bash testdata/stalefail-mutation.sh` | pass exit=0 | `unclassified waiting-brief route in planted_route_Dx0X7k.go:plantedRoute` then `PASS: the planted second route was detected`; the planted file was removed and the tree stayed clean | 2026-10-06 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (claude-opus-5-5[1m]) |

Supplementary checks (diagnostic only; they are not Verify rows):

- **Independent fail-first.** TestStaleFailVsMissingCard and the three fixture trees were copied onto the parent of the implementing commit (40d0063ef, unmodified brieffile.go). Result: exit 1. stale-fail failed with `want "stale FAIL"` and `newest fix commit missing`; missing-card failed with `want "sign-off card missing"`; current-fail passed (its text is unchanged). The test tells the old single route apart from the new three-state routing.
- **Own probes on a built binary** (`statusgen --lint` on copies of the stale-fail fixture):
  - With no git history, it emits the unchanged `--decision-issues` nudge plus `could-not-check stale FAIL — Evidence date or main history unavailable`. It is not rounded to current.
  - With gate:model and later path commits, it emits `stale FAIL` naming the newest commit (5661806) over an older one.
  - With the only repair commit on the same UTC day as the Evidence date, it keeps the fallback nudge.
- **Related suites.** TestFailNoticeClass, TestFailHistoryRoutes (12 cases), TestFailRunDate (4), TestFailMergeLanding (4) and the decision-issue tests all pass with `-count=1 -timeout=300s`.

Execution witness (`statusgen verifyrun`, v1.0.32, non-dry, clean tree):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1` | pass exit=0 | sha256:2c9b6014928d | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'PASS'` | fail exit=141 | sha256:e3b0c44298fc | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'stale FAIL'` | fail exit=141 | sha256:e3b0c44298fc | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && bash testdata/stalefail-mutation.sh` | pass exit=0 | sha256:65ac3123cdf5 | 2026-10-05 | assay-verifier-app[bot] @ fc5afe37f9e4 (on-behalf-of human:ian) (forge-identity) |

**Risk-bearing values.** The brief is gate:model and every risk field is `no`. Enumeration covered the diff of 8e20f9157 (statusgen/stalefail.go, statusgen/brieffile.go, statusgen/testdata/stalefail-mutation.sh):

| Literal | Location |
|---|---|
| staleness threshold `date.AddDate(0, 0, 1)` (one UTC day) | stalefail.go:111 |
| history refs `"refs/remotes/origin/main"`, `"refs/heads/main"`, in that order | stalefail.go:81 |
| `"--first-parent"` | stalefail.go:95 |
| shallow-repository guard `"false"` | stalefail.go:76–77 |
| run-heading depth `` `^#{3,6}[ \t]` `` | stalefail.go:12 |
| Evidence date shape `` `\b\d{4}-\d{2}-\d{2}\b` `` | stalefail.go:13 |
| fix-issue verbs regex | stalefail.go:14 |
| waiting statuses `"in-progress"`/`"implemented"`/`"verified"` | stalefail.go:164 |
| mutation-run `-timeout=60s` | stalefail-mutation.sh:14 |

All of these are reversible: a wrong value misroutes a lint NOTICE and is fixed by an edit and a release, and none certifies a pass. The threshold and the ref binding rank highest because they decide the routing.

- RISK-VALUE: DERIVED — staleness threshold = `date.AddDate(0, 0, 1)` @ statusgen/stalefail.go:111. Evidence dates are day-granular with no time of day, so a commit on the same day cannot be ordered against the failed run. "Commit on a later UTC day" is the only ordering the Evidence text can prove. The verifyrun witness stamps UTC dates (this run stamped 2026-10-05 while local time was 2026-10-06), so the UTC reading matches witness-dated Evidence. Residual: a hand-written local date from a runner west of UTC can make a fix that landed before the run read as stale. That error routes to a re-verify (cheap and self-correcting), never to a sign-off.
- RISK-VALUE: DERIVED — main history ref = `"refs/remotes/origin/main"` then `"refs/heads/main"` @ statusgen/stalefail.go:81. The brief requires "commits on main" and a pure, offline read. HEAD would let an unmerged branch supersede a FAIL (TestFailMergeLanding/unmerged and TestFailHistoryRoutes/head-only check that it does not). When neither ref exists the result is could-not-check, as the brief's three-state rule requires.

**Verdict detail.** The implemented behaviour matches the brief on every point checked: three distinct routes, the newest commit named, could-not-check kept, state (3)'s text unchanged, and the mutation guard live. Rows 2 and 4 fail as written: under the repo's own execution witness (pipefail) they exit 141 every time, because `grep -q` closes the pipe early. They exit 0 only in a plain shell. These are Verify-table authoring defects, not implementation defects. The same mechanism appears in measured-status/03 row 2. Before the witness can read green, both rows need a re-baseline that is safe under pipefail (for example `grep 'PASS' >/dev/null` in place of `grep -q 'PASS'`), ideally anchored to `--- PASS: TestStaleFailVsMissingCard`. Status stays `implemented`.

Filed as #2241 (Verify-row re-authoring, rows 2 and 4). Status stays `implemented`, with no flip.

VERIFY: FAIL

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
