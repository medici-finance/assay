---
brief: desk-tools/07
title: deskreply — worker-identity PR replies (never the App voice)
wave: 1
depends: ["desk-tools/01"]
unblocks: ["desk-tools/06"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-10 by Fable desk session ([I-23](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-10-desk-tools-zero-prompt-workflow-plumbing-purpose-built-binar.md), scoping.md v2 — critic-found worker-half coverage gap)
sources: ["docs/streams/desk-tools/scoping.md (tool table, TM-2, C-3, C-5, C-10)", "CLAUDE.md 'PR review loop' step 2 (workers MUST reply on findings)", "freshness-checked 2026-07-10 @ b98e1e84"]
why: >-
  The fix→re-review cycle is the loops' highest-volume path, and its worker half was the
  adversarial filter's top coverage gap: every review iteration the worker must reply on
  findings (CLAUDE.md mandates it) and that reply prompts today. Workers must NOT use deskpost
  for it — that would put worker words in the reviewer App's tamper-evident voice — so the worker
  identity needs its own narrow tool.
---

# Brief 07 — deskreply — worker-identity PR replies

## Context
files: create `../assay-toolkit/tools/desk/cmd/deskreply/` (new); uses `../assay-toolkit/tools/desk/internal/deskkit` (brief 01)
facts:
- Identity separation is the POINT of this tool: it posts with the ambient `gh` authentication
  (the `the-org` worker account) and must NEVER touch the App token path — no env read of
  `REVIEWER_APP_ID`/key material, no import of deskpost's mint code. The reviewer voice and the
  worker voice never share a tool (TM-1: the tamper-evident gate depends on the App's voice being
  mintable only via deskpost's audited path).
- Single subcommand: `deskreply <repo> <pr> --body-file F`. Plain issue-comment on the PR.
  No review verb, no verdict flag, no ready — a worker tool cannot emit anything a board or
  desk would read as a reviewer signal (C-7 spirit).
- **C-3:** deskkit bodycheck + 16 KiB cap (git SHAs pass; token shapes refuse); no stdin/inline
  body; no override flag.
- **C-5 (v2):** idempotency key `(repo, pr, headSHA, bodyDigest)`; only `result ∈ {ok,noop}`
  counts; flock; audit after remote success; rate ≤10/hour.
- **C-2/C-10:** verifies before posting: PR is OPEN and the invoking worktree's current branch
  matches the PR's head branch (a worker replies on ITS OWN PR — replying elsewhere is the
  desk's job via deskpost comment); unverifiable → exit 6.
- Repo scope: all three deskkit repos.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement per facts; `deskkit.Guard()` first; audit every path.
2. Tests: PATH-shim fake gh recording argv — assert the ONLY mutating call ever made is
   `pr comment` (never review/ready/create); own-branch precondition failure → exit 5;
   body refusals (oversize, token patterns) → exit 5 with no call; duplicate body at same
   head → noop with no call; kill switch → exit 3.
3. Prove the no-App-path claim mechanically: a test (or build constraint) asserting the
   deskreply binary/package graph does not import deskpost's token package and reads none of
   the App env vars (grep-level check in a test is acceptable and must be present).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/desk/cmd/deskreply/... -count=1` | exit 0; includes every negative test in Tasks 2-3 |
| 2 | `go vet ./tools/desk/...` | exit 0 |
| 3 | `DESK_TOOLS_DISABLED=1 go run ./tools/desk/cmd/deskreply oit 1 --body-file /dev/null; echo $?` | 3 |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
Verifier run (independent, non-implementer — opus-verifier, merged main `46eeaccc`):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/desk/cmd/deskreply/... -count=1` | 0 | `ok .../deskreply` — all Task 2-3 negatives present: not-my-branch/PR-not-open/origin-mismatch/oversize-body/secret-in-body refuse (no call), git-SHA-in-body passes, idempotent-noop, kill-switch, rate-limited-no-call, success-only-mutation-is-pr-comment, `TestNoAppTokenPath` (never the App voice) | 2026-07-11 | opus-verifier |
| 2 | `go vet ./tools/desk/...` | 0 | clean | 2026-07-11 | opus-verifier |
| 3 | `DESK_TOOLS_DISABLED=1 deskreply … ; echo $?` | 3 | compiled binary `BINARY EXIT=3` `refused: desk tools disabled (result=disabled)` (literal `echo $?`=1 is go1.26 go-run exit-masking; compiled binary confirms 3) | 2026-07-11 | opus-verifier |
| 4 | `go run ./tools/statusgen --root . --lint; echo $?` | 0 | exit 0 (advisory NOTICEs only) | 2026-07-11 | opus-verifier |

**VERIFY: PASS** — deskreply posts worker-identity PR replies via a plain `pr comment` (no App token path — `TestNoAppTokenPath`), refuses not-my-branch/PR-not-open/origin-mismatch/oversize/secret-bearing bodies (no call), is idempotent + rate-limited, and honors `DESK_TOOLS_DISABLED`. All mutation surfaces exercised offline (PATH-shim fake gh); no rows UNRUN.

## Review
Gate: model. Reviewer confirms the identity separation holds mechanically (Task 3) and the
own-PR precondition can't be skipped.
