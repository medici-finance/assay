---
brief: methodology/25
title: gate-why hard lint — flip the missing-gate-why NOTICE to a PROBLEM (phase 3)
wave: 1
depends: ["methodology/24"]
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-09 by opus (author-brief)
sources: ["spec: docs/superpowers/specs/2026-07-09-gate-why-rationale-design.md §2 + rollout", "PR #184 (gate-why mechanism — phase 1)", "methodology/24 (backfill — precondition)"]
---

# Brief 25 — gate-why hard lint (phase 3)

## Context
files:
- `../assay-toolkit/statusgen/brieffile.go` — `checkBriefFiles`, the gate-why block (added in PR #184). Today it calls `notice(...)`; a code comment there marks this exact flip ("PHASE 3 flips this to a hard `add(...)` error once PHASE 2's backfill lands").
- `../assay-toolkit/statusgen/brieffile_test.go` — `TestGateWhyNotice` (table-driven; asserts a risk-gated brief without gate-why produces a NOTICE and NOT a hard problem). Must be inverted to assert a PROBLEM.
- `../oit/.claude/skills/author-brief/SKILL.md` (in-repo) + `~/.claude/skills/author-brief/SKILL.md` (user-level) — both currently say "statusgen NOTICEs a missing gate-why today; it becomes a hard error once the backfill lands." Update the in-repo copy; note the user-level copy in Evidence (do not edit `~/.claude` from a repo brief).

facts:
- The flip is a one-line change: in the gate-why branch of `checkBriefFiles`, replace `notice(...)` with `add(...)` (making a missing gate-why on a risk-gated brief a hard `PROBLEM:`, exit 1). Update the surrounding comment from "NON-FATAL NOTICE / PHASE 3 flips" to "hard error".
- **Precondition (hard):** every risk-gated brief on main already carries a gate-why (methodology/24 complete — its Verify #1 reads 0). If ANY risk-gated brief still lacks gate-why when this merges, main's `--lint` goes red. This is why depends: methodology/24 and why this lands only after the backfill NOTICE count is zero.
- `checkBriefFiles` returns `(problems, notices)`; after the flip the gate-why case appends to `problems`. If that leaves `notice`/`notices` entirely unused in the function, drop the now-dead helper/return plumbing cleanly (keep it if other notices remain — none do today, so verify).

## Ground rules
- **Land only AFTER methodology/24 has merged** and `go run ./tools/statusgen --root . --lint 2>&1 | grep -c gate-why` reads 0 on main. Otherwise this reddens main.
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. In `checkBriefFiles` (`brieffile.go`), change the gate-why check from `notice(...)` to `add(...)` so a `gate: human` OR any-`risk.*:yes` brief with an empty/absent `gate-why` is a hard PROBLEM (exit 1). Reword the comment to state it is now enforced.
2. If `notices`/`notice(...)` are left unused by this change, remove the dead plumbing (the second return value, the helper) and update the caller in `main.go` accordingly — but ONLY if nothing else in the function still emits a notice. Otherwise leave it.
3. Invert `TestGateWhyNotice` → assert a risk-gated brief WITHOUT gate-why now yields a hard problem (via `briefSchemaProblems`), and a brief WITH gate-why yields none. Keep the fixture `brief-36-gatewhy-present.md` (no notice/problem) and use an existing gate-why-less risk-gated fixture (e.g. `brief-30`/`brief-04`) for the positive case.
4. Update the in-repo `author-brief` skill wording from "NOTICEs today; becomes a hard error once the backfill lands" to "a risk-gated brief MUST carry a gate-why (statusgen `--lint` errors otherwise)". Record the identical user-level-skill wording drift in Evidence + file/append an INTAKE follow-up (do not edit `~/.claude`).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test -count=1 ./tools/statusgen/...` | exit 0 (inverted TestGateWhyNotice + all existing tests pass) |
| 2 | `go vet ./tools/statusgen/...` | exit 0; no output |
| 3 | `statusgen --root . --lint` | exit 0 on backfilled main — every risk-gated brief has a gate-why, so the now-hard check finds nothing |
| 4 | add a throwaway risk-gated fixture with no gate-why, then `statusgen --root . --lint` | exit 1; a `PROBLEM:` naming the fixture and "gate-why" (the check is now fatal, not a NOTICE) — remove the fixture after |
| 5 | `grep -rn "NOTICE\|becomes a hard error\|flips this to" tools/statusgen/brieffile.go` | no stale "PHASE 3 flips" / "NON-FATAL NOTICE" comment remains on the gate-why block |

## Evidence
<!-- contract -->
| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test -count=1 ./tools/statusgen/...` | 0 | ok (inverted TestGateWhyProblem + all existing pass) | 2026-07-11 | implementer |
| 2 | `go vet ./tools/statusgen/...` | 0 | no output | 2026-07-11 | implementer |
| 3 | `go run ./tools/statusgen --root . --lint` | 0 | 0 gate-why NOTICEs on backfilled main | 2026-07-11 | implementer |
| 4 | throwaway risk-gated fixture with no gate-why, then `--lint` | 1 | PROBLEM: ... brief methodology/99 is risk-gated but has no gate-why | 2026-07-11 | implementer |
| 5 | `grep -rn "NOTICE\|becomes a hard error\|flips this to\|NON-FATAL" tools/statusgen/brieffile.go` | 0 | no matches (all stale comments removed) | 2026-07-11 | implementer |

User-level author-brief skill drift: `~/.claude/skills/author-brief/SKILL.md` still
says "statusgen currently NOTICEs a missing gate-why on --lint (non-fatal); it
becomes a hard error once the 28-brief backfill lands" — stale; filed as INTAKE
[I-39](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-11-user-level-author-brief-gate-why-wording-drift.md) (`../oit/docs/streams/intake/2026-07-11-user-level-author-brief-gate-why-wording-drift.md`).
Do not edit `~/.claude` from a repo brief.

Independent verifier run (non-implementer — opus-verifier, merged main `0174b912`; the rows above are the implementer's self-report and do not satisfy the non-implementer gate — these re-run them):

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `go test -count=1 ./tools/statusgen/...` | 0 | `ok …statusgen` — inverted `TestGateWhyProblem` + all existing pass | 2026-07-12 | opus-verifier |
| 2 | `go vet ./tools/statusgen/...` | 0 | no output | 2026-07-12 | opus-verifier |
| 3 | `go run ./tools/statusgen --root . --lint` | 0 | exit 0 on backfilled main; 0 gate-why PROBLEMs | 2026-07-12 | opus-verifier |
| 4 | throwaway risk-gated fixture (no gate-why) + `--lint` | 1 | `PROBLEM: … brief methodology/99 is risk-gated but has no gate-why…` — check is now fatal; fixture removed | 2026-07-12 | opus-verifier |
| 5 | `grep -rn "NOTICE\|becomes a hard error\|flips this to\|NON-FATAL" tools/statusgen/brieffile.go` | 1 | no matches — no stale phase-3/NON-FATAL comment remains | 2026-07-12 | opus-verifier |

**Tighten-only (integrity check):** verifier confirmed this ONLY tightens validation — the gate-why check in `brieffile.go checkBriefFiles` flips `notice(...)`→`add(...)` (missing gate-why on a risk-gated brief → hard PROBLEM); the dead `briefNotices` return + its `main.go` call site were dropped. It does **NOT** touch the anti-falsification / register-integrity / non-self-writable logic (`registers.go`, `migrate.go` untouched by PR #385). So the standard **model gate** applies — not the tamper-guard human gate.

**VERIFY: PASS** — a missing gate-why on a risk-gated brief is now a fatal lint PROBLEM (phase 3), backfilled main is clean, and no stale non-fatal comment remains. Integrity logic untouched → model-gated.

## Review
Gate: model (all four risk answers no — internal lint tightening, fully revertible via git). The one real hazard is sequencing: merging before methodology/24's backfill is complete reddens main for every un-backfilled risk-gated brief. Reviewer confirms Verify #3 (exit 0 on backfilled main) AND Verify #4 (the check actually fails a gate-why-less brief now) — presence of the gate AND its teeth.
