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
version: 2
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
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -c '^--- PASS: TestStaleFailVsMissingCard'` | prints `1`, exit 0 (the test's own `--- PASS:` line is present, so all three fixture-state assertions ran and passed and a renamed or missing test cannot pass vacuously). `grep -c` reads the whole stream, so the row holds under `pipefail`, where the earlier `grep -q` closed the pipe early and `go test` took SIGPIPE (exit 141). Re-baselined 2026-10-16 | check |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | exit 0 (a fixture tree exists per state) | check |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -c 'stale FAIL'` | prints a count >= 1, exit 0 (the stale-FAIL branch's message is exercised); the same `pipefail`-safe `grep -c` form as row 2. Re-baselined 2026-10-16 | check |
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

### Re-verification — VERIFY: FAIL — 2026-10-11 (assay-verifier-app[bot] @ 40b45182b3c0 (claude-opus-5-5) (on-behalf-of human:ian))

Non-implementer re-verification on merged main 40b45182b3c011cef823309c4e1c8d1aecbf6967, gate: model, all four risk answers no. Admitted by the dispatcher's pre-work check (attestation line below). First table: the `statusgen verifyrun` execution witness (statusgen v1.0.39, macOS arm64, go1.27.1, git 2.56.0), landed verbatim. Second table: the hand run in plain bash from the checkout root. Since the 2026-10-07 FAIL, the only statusgen change touching these files is 551a403df (a read memo in brieffile.go); stalefail.go, its test, the three fixture trees, the mutation script and the Verify table are unchanged since 8e20f91571c9. Issue #2241 is still open.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1` | pass exit=0 | sha256:4b111b112a63 | 2026-10-11 | assay-verifier-app[bot] @ 40b45182b3c0 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'PASS'` | fail exit=141 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 40b45182b3c0 (on-behalf-of human:ian) (forge-identity) |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 40b45182b3c0 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'stale FAIL'` | fail exit=141 | sha256:e3b0c44298fc | 2026-10-11 | assay-verifier-app[bot] @ 40b45182b3c0 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && bash testdata/stalefail-mutation.sh` | pass exit=0 | sha256:d67604cc84e8 | 2026-10-11 | assay-verifier-app[bot] @ 40b45182b3c0 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#2622 run=a5e845ec41ac980394212ba19a5a5903069bed5d30afbe11a081d046d7e7e027 source=40b45182b3c011cef823309c4e1c8d1aecbf6967 brief=docs/streams/measured-status/brief-04-stale-fail-lint-derivation.md model=claude-opus-5-5 tier=any

Hand run (each Go row also re-run with GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 as a diagnostic; results identical):

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1` | exit 0; output contains "ok" | exit 0; "ok github.com/medici-finance/assay/statusgen 0.571s". Isolated-git-config rerun: exit 0, "ok … 0.437s" | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'PASS'` | exit 0 | plain bash exit 0; under bash -o pipefail exit 141, 3 of 3 runs (grep -q exits on first match, go test takes SIGPIPE), which matches the witness. Content holds: the verbose log exits 0 with 5 PASS lines; subtests stale-fail, missing-card and current-fail all ran and passed. Isolated-git-config rerun: plain bash exit 0 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | exit 0 | exit 0; the glob matches the stale-fail and missing-card fixture dirs and stalefail-mutation.sh; the current-fail fixture dir is also present | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -q 'stale FAIL'` | exit 0 | plain bash exit 0; under bash -o pipefail exit 141, 3 of 3 runs, same SIGPIPE cause as row 2. Content holds: the verbose log carries "brief check/01 has stale FAIL — newest relevant main commit ed659a6 postdates Evidence 2026-08-01; a non-implementer RE-VERIFY supersedes it". Isolated-git-config rerun: plain bash exit 0 | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | `cd statusgen && bash testdata/stalefail-mutation.sh` | exit 0; planted second route is detected | exit 0; "unclassified waiting-brief route in planted_route_5VTjPL.go:plantedRoute" then "PASS: the planted second route was detected"; tree clean afterwards. Isolated-git-config rerun: exit 0, same PASS line | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Risk-bearing values. Risk metadata is present and all four answers are no, including irreversible: no. Enumerated over the 8e20f91571c9 diff to brieffile.go and stalefail.go, re-located at 40b45182b3c0:
- date.AddDate(0, 0, 1) @ statusgen/stalefail.go:111 — a fix commit counts only if it lands on a later UTC day than the Evidence date
- failHeadingRe = ^#{3,6}[ \t] @ statusgen/stalefail.go:12 — the Evidence run-heading depth
- status set {in-progress, implemented, verified} @ statusgen/stalefail.go:164
- ref order {refs/remotes/origin/main, refs/heads/main} @ statusgen/stalefail.go:81
- is-shallow-repository must equal "false" @ statusgen/stalefail.go:77, otherwise could-not-check

All of these are reversible lint-routing knobs, fixed by an edit and a release.

RISK-VALUE: DERIVED — date.AddDate(0, 0, 1) = 1 day @ statusgen/stalefail.go:111 — Evidence dates are day-granular, so a same-day commit cannot be ordered against them. Counting only next-UTC-day-or-later commits as stale avoids rounding an unknown order to stale. Its one error mode is an over-call for a west-of-UTC same-local-day edit, and that routes to a re-verify, never to a sign-off.

The implementation meets the brief's contract. The defect is in the Verify table: rows 2 and 4 pipe into `grep -q`, which exits on the first match. Under the canonical pipefail witness, go test then takes SIGPIPE and the row exits 141. Remedy (#2241, still open): read the pipe to EOF, for example by dropping -q and redirecting grep to /dev/null, or by using grep -c. Then re-verify.

**VERIFY: FAIL** — rows 2 and 4 exit 141 under the canonical pipefail witness (statusgen verifyrun v1.0.39), unchanged since 2026-10-07; rows 1, 3 and 5 pass; the code meets the contract; the Verify-table defect is tracked in #2241.

### Re-verification — VERIFY: PASS — 2026-10-21

Non-implementer re-verification on merged main 65a6418cd90c921153db509370212b1cdcd4f5e1 against the re-baselined Verify table (rows 2 and 4 now use the pipefail-safe `grep -c` form, re-baselined 2026-10-16). Gate: model; all four risk answers no. Every row was run under `bash -o pipefail` from the checkout root (go1.27.2 darwin/arm64, git 2.56.0), and rows 2 and 4 were run again in plain bash with the same results. stalefail.go, its test, the three fixture trees and the mutation script are unchanged since 8e20f91571c9. The tree was clean after every row.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1` | exit 0; output contains "ok" | exit 0 under pipefail; "ok github.com/medici-finance/assay/statusgen 0.980s" | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -c '^--- PASS: TestStaleFailVsMissingCard'` | prints `1`, exit 0 | prints 1, exit 0 under pipefail (no SIGPIPE); plain bash also prints 1, exit 0. Subtests stale-fail, missing-card and current-fail each show --- PASS | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | exit 0 | exit 0; the glob matches the stale-fail and missing-card fixture dirs and stalefail-mutation.sh; the current-fail fixture dir is also present | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -c 'stale FAIL'` | prints a count >= 1, exit 0 | prints 1, exit 0 under pipefail; plain bash also prints 1, exit 0. The matched line reads "brief check/01 has stale FAIL — newest relevant main commit ed659a6 postdates Evidence 2026-08-01; a non-implementer RE-VERIFY supersedes it" | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd statusgen && bash testdata/stalefail-mutation.sh` | exit 0; planted second route is detected | exit 0 under pipefail; "unclassified waiting-brief route in planted_route_2PxuLq.go:plantedRoute" then "PASS: the planted second route was detected"; tree clean afterwards | 2026-10-21 | assay-verifier-app[bot] @ 65a6418cd90c921153db509370212b1cdcd4f5e1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Verification-Attestation: medici-finance/assay#3882 run=8d6199a753d839b4e786e1525773e5785224231bbddd326cf183fae75c657a94 source=65a6418cd90c921153db509370212b1cdcd4f5e1 brief=docs/streams/measured-status/brief-04-stale-fail-lint-derivation.md model=claude-opus-5-5 tier=strong

Risk-bearing values. Risk metadata is present and all four answers are no, including irreversible: no. Enumerated over the 8e20f91571c9 diff to brieffile.go and stalefail.go, re-located at 65a6418cd90c:
- date.AddDate(0, 0, 1) @ statusgen/stalefail.go:111 — a fix commit counts only if it lands on a later UTC day than the Evidence date
- failHeadingRe = ^#{3,6}[ \t] @ statusgen/stalefail.go:12 — the Evidence run-heading depth
- status set {in-progress, implemented, verified} @ statusgen/stalefail.go:164
- ref order {refs/remotes/origin/main, refs/heads/main} @ statusgen/stalefail.go:81
- is-shallow-repository must equal "false" @ statusgen/stalefail.go:76, otherwise could-not-check

All of these are reversible lint-routing knobs, fixed by an edit and a release.

RISK-VALUE: DERIVED — date.AddDate(0, 0, 1) = 1 day @ statusgen/stalefail.go:111 — Evidence dates are day-granular, so a same-day commit cannot be ordered against them. Counting only commits from the next UTC day or later as stale avoids rounding an unknown order to stale. Its one error mode is an over-call for a same-local-day edit west of UTC, and that routes to a re-verify, never to a sign-off.

The Verify-table defect behind the 2026-10-07 and 2026-10-11 FAILs (rows 2 and 4 exiting 141 under pipefail, #2241) is fixed by the 2026-10-16 re-baseline: both rows now exit 0 under pipefail.

**VERIFY: PASS** — 5/5 rows pass at 65a6418cd90c921153db509370212b1cdcd4f5e1 under pipefail.

### Re-verification — VERIFY: PASS — 2026-10-21 (merged main 471fa0febe6d)

Non-implementer re-verification on merged main 471fa0febe6d4370b973cf2380a0513f6a1d7efd, admitted by the dispatcher's pre-work check (attestation line below). Gate: model; all four risk answers no. Every row was run under `bash -o pipefail` from the checkout root (go1.27.2 darwin/arm64, git 2.56.0). Between 65a6418cd90c and 471fa0febe6d the only statusgen changes are to the dorajson files; stalefail.go, its test, the three fixture trees and the mutation script are unchanged since 8e20f91571c9. The tree was clean after every row.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1` | exit 0; output contains "ok" | exit 0 under pipefail; "ok github.com/medici-finance/assay/statusgen 0.690s" | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -c '^--- PASS: TestStaleFailVsMissingCard'` | prints `1`, exit 0 | prints 1, exit 0 under pipefail; subtests stale-fail, missing-card and current-fail each show --- PASS | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | exit 0 | exit 0; the glob matches the stale-fail and missing-card fixture dirs and stalefail-mutation.sh; the current-fail fixture dir is also present | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -c 'stale FAIL'` | prints a count >= 1, exit 0 | prints 1, exit 0 under pipefail; the matched line reads "brief check/01 has stale FAIL — newest relevant main commit ed659a6 postdates Evidence 2026-08-01; a non-implementer RE-VERIFY supersedes it", distinct from the missing-card line ("sign-off card missing — verify-desk lands the marker") and the current-fail line ("no decision-issue — file one via --decision-issues") | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd statusgen && bash testdata/stalefail-mutation.sh` | exit 0; planted second route is detected | exit 0 under pipefail; "unclassified waiting-brief route in planted_route_CTjOIa.go:plantedRoute" then "PASS: the planted second route was detected"; tree clean afterwards | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Verification-Attestation: medici-finance/assay#3988 run=5086aedb5aacce807de66169de97685930e7ac12331cbcd838d20c682a524752 source=471fa0febe6d4370b973cf2380a0513f6a1d7efd brief=docs/streams/measured-status/brief-04-stale-fail-lint-derivation.md model=claude-opus-5-5 tier=strong

Risk-bearing values. Risk metadata is present and all four answers are no, including irreversible: no. Enumerated over the 8e20f91571c9 diff to brieffile.go and stalefail.go, re-located at 471fa0febe6d:
- date.AddDate(0, 0, 1) @ statusgen/stalefail.go:111 — a fix commit counts only if it lands on a later UTC day than the Evidence date
- failHeadingRe = ^#{3,6}[ \t] @ statusgen/stalefail.go:12 — the Evidence run-heading depth
- status set {in-progress, implemented, verified} @ statusgen/stalefail.go:164
- ref order {refs/remotes/origin/main, refs/heads/main} @ statusgen/stalefail.go:81
- is-shallow-repository must equal "false" @ statusgen/stalefail.go:77, otherwise could-not-check

All of these are reversible lint-routing knobs, fixed by an edit and a release.

RISK-VALUE: DERIVED — date.AddDate(0, 0, 1) = 1 day @ statusgen/stalefail.go:111 — Evidence dates are day-granular, so a same-day commit cannot be ordered against them. Counting only commits from the next UTC day or later as stale avoids rounding an unknown order to stale. Its one error mode is an over-call for a same-local-day edit west of UTC, and that routes to a re-verify, never to a sign-off.

**VERIFY: PASS** — 5/5 rows pass at 471fa0febe6d4370b973cf2380a0513f6a1d7efd under pipefail.

### Re-verification — VERIFY: PASS — 2026-10-22 (merged main 05dc9fbc88f2)

Non-implementer re-verification on merged main 05dc9fbc88f2b1b23cd78bebc8634c564327eca4, admitted by the dispatcher's pre-work check (rc 0 before the rows and again after this section was written, same binding; attestation line below). Gate: model; all four risk answers no. The table below is the write-mode `statusgen verifyrun` witness (v1.0.54), run from the admitted home on a clean tree, exactly as the tool wrote it. No Verify row is classed check:ci, so the network-off container sandbox does not apply to this table. Between 471fa0febe6d and 05dc9fbc88f2 there is no change to stalefail.go, its test, brieffile.go, the three fixture trees or the mutation script.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1` | pass exit=0 | sha256:9b7e7968f3a6 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -c '^--- PASS: TestStaleFailVsMissingCard'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -c 'stale FAIL'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && bash testdata/stalefail-mutation.sh` | pass exit=0 | sha256:2e917c088a3b | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4060 run=f577c76cc64165ec3662c5d54386c92b36153d4d499ecc2e5dff7ef6f6056f3b source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/measured-status/brief-04-stale-fail-lint-derivation.md model=claude-opus-5-5 tier=strong

Supplemental, no credit: a hand run of rows 2 and 4 under `bash -o pipefail` printed 1 for each, and the verbose test log carries three distinct route lines: "has stale FAIL — newest relevant main commit ed659a6 postdates Evidence 2026-08-01; a non-implementer RE-VERIFY supersedes it", "sign-off card missing — verify-desk lands the marker / verify-gate-open files the card" and "is gate:human at implemented but has no decision-issue — file one via --decision-issues".

Risk-bearing values. Risk metadata is present and all four answers are no, including irreversible: no. Enumerated over the 8e20f91571c9 diff to brieffile.go and stalefail.go, re-located at 05dc9fbc88f2:
- date.AddDate(0, 0, 1) @ statusgen/stalefail.go:111 — a fix commit counts only if it lands on a later UTC day than the Evidence date
- failHeadingRe = ^#{3,6}[ \t] @ statusgen/stalefail.go:12 — the Evidence run-heading depth
- status set {in-progress, implemented, verified} @ statusgen/stalefail.go:164
- ref order {refs/remotes/origin/main, refs/heads/main} @ statusgen/stalefail.go:81
- is-shallow-repository must equal "false" @ statusgen/stalefail.go:76, otherwise could-not-check

All of these are reversible lint-routing knobs, fixed by an edit and a release.

RISK-VALUE: DERIVED — date.AddDate(0, 0, 1) = 1 day @ statusgen/stalefail.go:111 — Evidence dates are day-granular, so a same-day commit cannot be ordered against them. Counting only commits from the next UTC day or later as stale avoids rounding an unknown order to stale. Its one error mode is an over-call for a same-local-day edit west of UTC, and that routes to a re-verify, never to a sign-off.

**VERIFY: PASS** — 5/5 rows pass at 05dc9fbc88f2b1b23cd78bebc8634c564327eca4 (statusgen verifyrun write-mode witness).
### Re-verification — VERIFY: PASS — 2026-10-27 (merged main 52719b9592b4)

Non-implementer re-verification on merged main 52719b9592b4443a784d246813117bf4023fb410, admitted by the dispatcher's pre-work check (attestation line below). Gate: model; all four risk answers no. Delivery: #2238, merge 8e20f91571c9 (parent 40d0063ef8a0, resolved from the forge). The table below is the `statusgen verifyrun` witness (v1.0.63, darwin arm64, go1.27.2, git 2.56.0), run from the admitted home on a clean tree, exactly as the tool printed it. No Verify row is classed check:ci, so the network-off container sandbox does not apply. No row carries a risk-bearing, live, mutating or end-to-end tag and no brief-wide risk answer is yes, so no row is risk-bearing and no fail-first witness is owed. Since 05dc9fbc88f2, stalefail.go, its test, the three fixture trees and the mutation script are unchanged; brieffile.go changed in four commits, and its only call into the waiting-brief route (waitingBriefNotice) is unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1` | pass exit=0 | sha256:4fe86a6f9705 | 2026-10-27 | assay-verifier-app[bot] @ 52719b9592b4 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -c '^--- PASS: TestStaleFailVsMissingCard'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-27 | assay-verifier-app[bot] @ 52719b9592b4 (on-behalf-of human:ian) (forge-identity) |
| 3 | `ls statusgen/testdata/*stale*fail* statusgen/testdata/*missing*card* >/dev/null 2>&1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 52719b9592b4 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test . -run TestStaleFailVsMissingCard -count=1 -v 2>&1 \| grep -c 'stale FAIL'` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-27 | assay-verifier-app[bot] @ 52719b9592b4 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && bash testdata/stalefail-mutation.sh` | pass exit=0 | sha256:88eb56247c87 | 2026-10-27 | assay-verifier-app[bot] @ 52719b9592b4 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4566 run=3b0887f1be44e1026b6ec66c69a7109a719ee41c72d300911e4958c21d569dd7 source=52719b9592b4443a784d246813117bf4023fb410 brief=docs/streams/measured-status/brief-04-stale-fail-lint-derivation.md model=claude-opus-5-5 tier=strong

Supplemental, no credit: a hand run of every row under `bash -o pipefail` gave the same exits. Row 1 printed "ok github.com/medici-finance/assay/statusgen 0.434s"; rows 2 and 4 each printed 1; row 3 matched the stale-fail and missing-card fixture dirs and stalefail-mutation.sh; row 5 printed "unclassified waiting-brief route in planted_route_wRLZ2W.go:plantedRoute" then "PASS: the planted second route was detected". The verbose test log carries three distinct route lines, one per subtest (stale-fail, missing-card, current-fail, each --- PASS): "has stale FAIL — newest relevant main commit ed659a6 postdates Evidence 2026-08-01; a non-implementer RE-VERIFY supersedes it", "sign-off card missing — verify-desk lands the marker / verify-gate-open files the card" and "is gate:human at implemented but has no decision-issue — file one via --decision-issues". The tree was clean after every row.

Risk-bearing values. Risk metadata is present and all four answers are no, including irreversible: no. Enumerated over the 8e20f91571c9 diff to brieffile.go and stalefail.go, re-located at 52719b9592b4:
- date.AddDate(0, 0, 1) @ statusgen/stalefail.go:111 — a fix commit counts only if it lands on a later UTC day than the Evidence date
- failHeadingRe = ^#{3,6}[ \t] @ statusgen/stalefail.go:12 — the Evidence run-heading depth
- status set {in-progress, implemented, verified} @ statusgen/stalefail.go:164
- ref order {refs/remotes/origin/main, refs/heads/main} @ statusgen/stalefail.go:81
- is-shallow-repository must equal "false" @ statusgen/stalefail.go:77, otherwise could-not-check

All of these are reversible lint-routing knobs, fixed by an edit and a release.

RISK-VALUE: DERIVED — date.AddDate(0, 0, 1) = 1 day @ statusgen/stalefail.go:111 — Evidence dates are day-granular, so a same-day commit cannot be ordered against them. Counting only commits from the next UTC day or later as stale avoids rounding an unknown order to stale. Its one error mode is an over-call for a same-local-day edit west of UTC, and that routes to a re-verify, never to a sign-off.

**VERIFY: PASS** — 5/5 rows pass at 52719b9592b4443a784d246813117bf4023fb410 (statusgen verifyrun witness).

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
