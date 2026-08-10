---
brief: methodology/02
title: Evidence enforcement — verified requires a filled Evidence section
wave: 1
depends: ["methodology/01"]
unblocks: ["methodology/07"]
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (initiative-streams step 3)
sources: ["spec §11 adopt-1 (evidence blocks, from Sharper-Flow/Advance)", "[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)", "Task-15 review lesson: verification claims need artifacts"]
---

# Brief 02 — Evidence enforcement at the verified gate

## Context
files: tools/statusgen/{brieffile.go,checks.go} (+tests)
facts:
- brief-v1 files carry an `## Evidence` section with a contract comment; today nothing checks it.
- Enforcement rule: a brief whose stream-README status is `verified` or `done` AND whose file is brief-v1 MUST have a non-empty Evidence section — at least one row of (command, exit code, output/hash, date, runner) beyond the contract comment.
- Legacy briefs (no frontmatter): `grandfathered` in Verified column already covers them — no new requirement.
- AMENDED 2026-07-08 (route-2, from PR #77's /review findings — same file, same wave): this brief ALSO hardens two brieffile.go gaps: (a) a file whose raw content matches the brief-v1 schema marker but whose frontmatter fails to split (e.g. unterminated `---`) is an ERROR, not a legacy exemption — silent false-green otherwise; (b) `risk:` must contain exactly the four canonical keys (regulatory, customer, irreversible, sensitive-data) — a missing question can never fire the human gate.
- Real precedent: Task 15's first verification claim had no artifact and was bounced by review; this brief mechanizes that bounce.
- NOTED 2026-07-09 ([F-10](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-verify-tables-for-prose-deliverables-are-presence-checks-not.md), post-done clarification — no contract change): this check gates the *presence and shape* of Evidence rows, not their truth — a fabricated row passes it. Truth is owned by adversarial re-verification and the review gates (and, structurally, by methodology/16's non-self-writable transitions). Claim discipline for anything citing this mechanism: "consistency-linted", never "machine-verified".

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Extend the brief-01 brief-file parser to capture the Evidence section body (text between `## Evidence` and the next `## `).
2. Add check: README status verified/done + brief-v1 file + Evidence contains no content row → PROBLEM "verified requires filled Evidence".
3. Define "content row": any non-empty line outside HTML comments.
4. TDD with fixtures: verified+empty (fail), verified+filled (pass), done+grandfathered legacy (exempt), todo+empty (pass).
5. Hardening (per amendment): unterminated-frontmatter-with-marker = error; require the four canonical risk keys. TDD both.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run TestEvidence -v` | exit 0; ≥4 subtests PASS |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `statusgen --root . --lint` | exit 0 (all source checks incl. evidence enforcement pass; `--lint` is the branch-side gate — STATUS.md byte-drift is main's job per brief-15, so `--check` necessarily fails on any status-changing branch) |
| 4 | `go test ./tools/statusgen/ -run "TestBriefFileUnterminated|TestBriefFileRiskKeys" -v` | exit 0; both hardening cases covered (row added by the 2026-07-08 amendment) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Implementer run (records the implementation-time result; `verified` still needs an
independent re-run by a non-implementer):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestEvidence -v` | 0 | 5 `TestEvidence*` PASS (verified-empty fail, verified-filled pass, todo-empty pass, legacy-done exempt, content-detection unit) | 2026-07-08 | implementer (Opus 4.8) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | full suite ok; vet clean | 2026-07-08 | implementer (Opus 4.8) |
| 3 | `go run ./tools/statusgen --root . --check` | 0 | clean; brief-12 (done) passes on its filled Evidence section | 2026-07-08 | implementer (Opus 4.8) |

Independent verification (non-implementer re-run, 2026-07-08) — all four Verify items
green on post-#86 main. Item 4's four-canonical-risk-keys hardening landed via PR #86,
so `TestBriefFileRiskKeys` is now a real test (no longer a zero-match false-green):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test ./tools/statusgen/ -run TestEvidence -v` | 0 | 6 `TestEvidence*` PASS | 2026-07-08 | independent (opus) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | 0 | full suite ok; vet clean | 2026-07-08 | independent (opus) |
| 3 | `go run ./tools/statusgen --root . --lint` | 0 | all source checks pass (incl. evidence enforcement); no STATUS.md drift-compare — single-writer per brief-15 (`--check` fails on a status-changing branch by design) | 2026-07-08 | independent (opus) |
| 4 | `go test ./tools/statusgen/ -run "TestBriefFileUnterminated\|TestBriefFileRiskKeys" -v` | 0 | both hardening tests run + PASS | 2026-07-08 | independent (opus) |

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README
table. Human gate is MANDATORY when any risk answer is yes.
