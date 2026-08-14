---
brief: vrun/02
title: A brief whose Evidence does not witness every Verify row
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [284]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: the verifyrun POSITIVE CONTROL — the red run that proves --check can fail"]
---

# Fixture — the red run

**This is the proof that `--check` can fail.** A check that has only ever been
observed passing is not known to be a check at all; the stream's shared
conventions require every checker to ship with a captured red run against a
positive control, and this file is that control. `statusgen verifyrun --check`
against it must exit **2**, and the test that asserts so is the one that would
notice if the audit quietly started reporting green.

Its Evidence is deliberately partial, and partial in each of the three distinct
ways a witness set can be incomplete — one per row, so a regression in any one
arm is visible on its own rather than masked by the others:

| Verify row | What the Evidence does | What `--check` must say |
|---|---|---|
| 1 | carries a matching, passing witness | `pass` |
| 2 | carries a witness for a command the row NO LONGER has | `could-not-run` (stale — the row was edited after it ran) |
| 3 | carries nothing at all | `could-not-run` (missing) |
| 4 | carries a matching witness recording a failure | `fail` |

Row 4 is there so the fixture also pins the EXIT-CODE PRECEDENCE: a run holding
both a `fail` and a `could-not-run` exits 2, not 1. The two codes are not
interchangeable — 1 says "the work is wrong", 2 says "the instrument did not
look" — and a fixture with only one of them could not tell you which one wins.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | exit 0 |
| 2 | `test -d statusgen/testdata/verifyrun` | exit 0 (command edited since the witness was taken) |
| 3 | `printf 'ok\n'` | output is `ok` |
| 4 | `false` | exit 0 (records a genuine failure) |

## Evidence
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `true` | pass exit=0 | sha256:e3b0c44298fc | 2026-08-13 | fixture-app[bot] @ 000000000000 |
| 2 | `test -d statusgen` | pass exit=0 | sha256:e3b0c44298fc | 2026-08-13 | fixture-app[bot] @ 000000000000 |
| 4 | `false` | fail exit=1 | sha256:e3b0c44298fc | 2026-08-13 | fixture-app[bot] @ 000000000000 |

## Review
Gate: model.
