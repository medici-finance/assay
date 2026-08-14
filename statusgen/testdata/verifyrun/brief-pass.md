---
brief: vrun/01
title: A brief whose Verify rows all pass when actually executed
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [284]
schema: brief-v1
authored: 2026-08-13 by fixture
sources: ["fixture: the verifyrun happy path — rows that run, and pass, at the repo root"]
---

# Fixture — every Verify row runs and passes

The positive control for `statusgen verifyrun`. Every row below is hermetic: no
network, no writes, no dependency on a toolchain being installed, and no
dependency on anything outside this repository. It executes at the REPO ROOT
(that is verifyrun's contract), which is why row 2 names its own path from
there — a row that can only pass from inside `statusgen/` would be testing the
fixture's cwd rather than verifyrun's.

Each row exercises one arm of the Expect reader:

- row 1 — bare `exit 0`, nothing else decidable;
- row 2 — an existence guard, still exit-only, but with a real filesystem fact
  behind it, so a fixture that goes missing turns this row red;
- row 3 — the `` output is `X` `` convention;
- row 4 — the `≥ N` count convention.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | exit 0 |
| 2 | `test -f statusgen/testdata/verifyrun/brief-pass.md` | exit 0 (the fixture can see itself from the repo root) |
| 3 | `printf 'rc=0\n'` | output is `rc=0` |
| 4 | `printf 'a\nb\nc\n' \| grep -c .` | exit 0; output ≥ `3` |

## Evidence
<!-- deliberately empty: `verifyrun --dry-run` against this fixture must produce
     a full witness table without depending on one already being here. -->

## Review
Gate: model.
