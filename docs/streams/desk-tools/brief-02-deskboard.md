---
brief: desk-tools/02
title: deskboard v2 — read-only cross-repo board in tools/desk (GET-only, tested)
wave: 1
depends: ["desk-tools/01"]
unblocks: ["desk-tools/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [209]
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md), scoping.md)
sources: ["docs/streams/desk-tools/scoping.md", "~/.claude/skills/pr-review-desk/deskboard.go (310 lines, the v1 to absorb)", "freshness-checked 2026-07-10 @ b98e1e84"]
why: >-
  The review/verify/next-job loops hand-poll `gh -R <repo> pr view/list/checks` across three
  repos; the `-R` flag defeats read-only auto-allow, so pure READS prompt constantly (40+ per
  scan cycle measured). One pinned read-only binary replaces all of it with a single allow rule.
---

# Brief 02 — deskboard v2 — read-only cross-repo board

## Context
files: create `../assay-toolkit/tools/desk/cmd/deskboard/` (new); absorb and supersede
`~/.claude/skills/pr-review-desk/deskboard.go` (v1: 310 lines, hardcoded `var repos` with the
same 3-repo set, computes per-PR ACTION {NEEDS-REVIEW, RE-REVIEW, BLOCKED, CHECK, WAIT-CI,
CI-RED, FLIP, READY} by comparing current head vs the head of the desk's latest review, CI
rollup, and the review verdict); uses `../assay-toolkit/tools/desk/internal/deskkit` (brief 01)
facts:
- Constraints implemented here: **C-4** (fixed repo set from deskkit — v1's own list is replaced
  by the deskkit constant), **C-7/C-9** (read-only END TO END, proven by test), **C-10**
  (partial API responses are an error, never a partial board), plus deskkit Guard/audit/version
  from brief 01.
- Read-only guarantee: the tool shells out to `gh` ONLY with read subcommands (`pr list`,
  `pr view`, `api` GET) or calls the REST API with GET. Test strategy (mandatory): a fake `gh`
  shim placed first in PATH during tests records every invocation; tests assert NO invocation
  contains a mutating subcommand (`comment|review|ready|create|edit|merge|close`) and no
  `gh api` call uses `-X POST/PATCH/PUT/DELETE` or `-f/-F` field flags.
- Subcommands (all emit JSON to stdout; human-readable table with `--table`):
  `deskboard prs` (open PRs, all 3 repos: number, title, draft, headSHA, mergeState, CI rollup),
  `deskboard actions` (v1's ACTION classification, preserved semantics),
  `deskboard reviews <repo> <pr>` (review states + which head each verdict was posted at),
  `deskboard queue` (the awaiting-verification view: gate:human briefs' open verify-gate issues
  via `gh api` GET on the issues endpoint, label `verify-gate`),
  `deskboard diff <repo> <pr>` and `deskboard files <repo> <pr> [path]` (reviewer reads: the PR
  diff and changed-file contents — the largest measured read-prompt class moves to reviewer
  agents otherwise; GET-only like everything else),
  and on EVERY run two banners to stderr: **STALE** (installed sourceSHA vs origin/main's
  tools/desk tree — drift is a per-boot banner per C-1) and **audit-age** (audit file FirstTS —
  a suspicious reset is visible per C-5).
- Anti-stale-quote support (#209): every report (JSON and --table) carries an
  `asOf: <UTC RFC3339>` header field, and `deskboard actions` emits a `MERGED — drop from
  your list` tombstone row for any PR that was open within the trailing hour (from the audit
  file's prior runs) but is now merged/closed — so a desk quoting yesterday's ready-list gets
  contradicted by the next run instead of silently agreeing. Test: fake gh returns a PR as
  merged that a prior audit entry saw open → tombstone row present.
- Security-review gate (#216, methodology/30): `deskboard actions` computes
  **SECURITY-REVIEW-REQUIRED** for a PR that is risk-classed — primary signal: changed files
  (already fetched, GET-only) touch a path trigger (`daml/`,
  `services/ledger-service/internal/auth/`, `services/ledger-service/internal/api/`,
  `k8s/*/identity.yaml`, `k8s/*/canton/`); best-effort secondary: the owning brief's
  frontmatter (`gate: human` / any risk yes) when the PR body or branch names a brief file —
  and no App review at the CURRENT head carries the literal line `Security-Review: pass`.
  FLIP is never emitted while the flag holds. `deskboard reviews` distinguishes
  security-review verdicts (body-marker parse). Test: fake gh, risk-path diff, App APPROVED
  at head, no Security-Review line → action is SECURITY-REVIEW-REQUIRED, not FLIP; with the
  line → FLIP.
- C-10 shape: any gh/API failure or JSON parse error on ANY repo → exit 6 with the failing repo
  named. Never emit a board that silently omits a repo (a partial board reads as "nothing open"
  — the exact partial-read-as-success defect class this repo fixed in ledger-hardening/05).
- v1 stays untouched at its skill path until brief 06 removes the allowlist rule that runs it;
  do NOT edit files under `~/.claude/` in this brief.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Port v1's data model + ACTION classifier into `../assay-toolkit/tools/desk/cmd/deskboard`, replacing its repo
   list with `deskkit`'s (C-4) and wiring `deskkit.Guard()` first, audit line per run, version
   stamp output.
2. Implement the four subcommands above; JSON is the default output (loops consume it), `--table`
   for humans.
3. Implement the fail-closed rule (C-10) exactly as stated in facts — one repo failing fails the
   run with exit 6.
4. Tests: the PATH-shim read-only proof (facts above), classifier table tests ported from v1
   behavior (same inputs → same ACTION), partial-failure → exit 6 test, kill-switch → exit 3 test.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/deskboard/... -count=1` | exit 0; includes the read-only PATH-shim test, the partial-failure exit-6 test, the #209 tombstone test, and the SECURITY-REVIEW-REQUIRED classifier test (#216) |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | `go run ./tools/desk/cmd/deskboard prs 2>&1 \| python3 -m json.tool > /dev/null; echo $?` | 0 (valid JSON against the live repos; run reads only) |
| 4 | `d=$(mktemp -d); go build -o "$d/deskboard" ./tools/desk/cmd/deskboard && DESK_TOOLS_DISABLED=1 "$d/deskboard" prs; echo $?` | 3 (kill switch, exit-3 propagated by the compiled binary — how the tool ships, C-1; `go run` collapses any non-zero child exit to 1 so it cannot surface the code) |
| 5 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

Verifier run (independent, non-implementer — opus-verifier, merged main `2a8cd673`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskboard/... -count=1` | 0 | `ok .../deskboard` — TestReadOnly_PathShim (34 gh invocations enumerated, all read-only), TestActions_PartialFailure_Exit6, TestActions_Tombstone_209 (#209), TestSecurityReviewRequired_216 both subcases (#216) all pass | 2026-07-10 | opus-verifier |
| 2 | `go vet ./tools/desk/...` | 0 | clean | 2026-07-10 | opus-verifier |
| 3 | `go run ./tools/desk/cmd/deskboard prs \| python3 -m json.tool >/dev/null; echo $?` | 0 | valid JSON against live repos; read-only run ok | 2026-07-10 | opus-verifier |
| 4 | compiled binary with `DESK_TOOLS_DISABLED=1` | 3 | refused: "desk tools disabled (result=disabled)" exit 3 — kill switch propagated by compiled binary | 2026-07-10 | opus-verifier |
| 5 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-10 | opus-verifier |

**VERIFY: PASS** — deskboard v2 read-only path-shim, partial-failure exit-6, tombstone (#209), security-review gate (#216), and DESK_TOOLS_DISABLED kill switch all verified.

## Review
Gate: model. Reviewer must confirm the read-only proof test actually enumerates recorded gh
invocations (not just "no error"), and that no code path can emit a partial board.
