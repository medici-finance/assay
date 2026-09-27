---
brief: assay:assay:measured-status:01
title: "Derive the deskkit exit-code table — record the convention ExitOK/Disabled/RateLimited/Refused/Unverifiable follow, and pin it with a test"
why: >-
  The desk tools' whole error contract is five integers (0/3/4/5/6) that callers, scripts and
  CI branch on, but no derivation is on record for why those values (and not, say, a distinct
  code per refusal class, or the skipped 1/2). A magic constant a verifier can only accept on
  faith is the smallest instance of the "value asserted, not derived" seam; recording the
  convention and pinning it turns five faith-values into checkable ones.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1216]
schema: brief-v2
authored: 2026-09-16 by measured-status scoping session
sources:
  - "#1216 — deskkit.ExitRefused=5 exit-code table, no derivation on record (acp-guardrails/10 verify)"
  - "tools/desk/internal/deskkit/exitcodes.go — the const block (ExitOK=0, ExitDisabled=3, ExitRateLimited=4, ExitRefused=5, ExitUnverifiable=6) and DeskError"
  - "freshness-checked 2026-09-16 @ e9fa19d3 — the table carries per-const doc comments but names no source convention; no test asserts the values against a derivation"
exec-tier: any
domain: clear
version: 1
id: e0e5c40f-6e55-4906-8e90-079dc8649947
---

# Brief 01 — Derive the deskkit exit-code table

## Context
files:
- `tools/desk/internal/deskkit/exitcodes.go` — the exit-code const block and `DeskError`.
- `tools/desk/internal/deskkit/exitcodes_test.go` — add (or extend) the pinning test.
facts:
- current table: `ExitOK=0`, `ExitDisabled=3`, `ExitRateLimited=4`, `ExitRefused=5`, `ExitUnverifiable=6`; `1`/`2` are unused.
- `1`/`2` are conventionally the shell's own general-error and builtin-misuse codes; a CLI that avoids them keeps its own codes distinct from a crash/usage error — this is the convention to state and cite.
- `tools/desk` is its own Go module; tests run from `tools/desk/`.
- this is a derivation-of-record brief: the VALUES do not change, only their derivation gains a written source and a test that fails if a value drifts from it.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add a `// Derivation:` doc block to `exitcodes.go` stating the convention the table
   follows: codes start at `3` to leave `0` for success and `1`/`2` for the shell's own
   general-error / misuse codes, and each subsequent code is one refusal *class* the caller
   must branch on differently (disabled vs rate-limited vs constraint-refused vs
   unverifiable), not one per message. Name why a single `ExitRefused` (not a code per refusal
   reason) is correct: the caller's action is identical for every compiled-in refusal.
2. Add a test `TestExitCodeTableMatchesDerivation` (planned) that asserts each constant equals
   the value the stated convention predicts (0 for OK; the four refusal classes are 3..6
   contiguous; 1 and 2 are never used) and fails if any value drifts.
3. Do not change any constant's value; if the convention and a value genuinely disagree, stop
   and report NEEDS_CONTEXT rather than editing either to force agreement.
4. **Fail-first (rule 9).** Add one entry to `tools/desk/internal/deskkit/mutations.json`
   that flips the new convention (e.g. reorders two adjacent refusal-class values so the
   stated convention and the table disagree), and extend that spec's `"test"` field to
   include `TestExitCodeTableMatchesDerivation` (planned) — the field is a fixed allow-list `-run`
   pattern consumed by `muhar`, and a new test the pattern does not name is invisible to it,
   so the mutant cannot be caught until the field is widened. Confirm the corpus's own guard
   catches it — run `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/mutations.json`
   and check the new mutation reports CAUGHT (mutant applied) and the suite is GREEN again once
   reverted.
   Record the red-then-green run under `## Evidence`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1` | exit 0; output contains "ok" |
| 2 | `cd tools/desk && go vet ./internal/deskkit/` | exit 0 |
| 3 | `cd tools/desk && grep -q 'Derivation:' internal/deskkit/exitcodes.go` | exit 0 (the derivation block is present) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1 -v 2>&1 \| grep -q 'PASS'` | exit 0 (the pinning assertions actually ran and passed) |
| 5 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, and every mutation CAUGHT, including the new exit-code-convention mutant (only reachable once the `"test"` field names `TestExitCodeTableMatchesDerivation` (planned)) |

## Evidence
<!-- appended at implementation time by a non-implementer -->

### Implementer run — derivation recorded; pinning test red-then-green; corpus mutant CAUGHT — 2026-09-27, implementer (worker), branch `feat/measured-status-01` off `refs/remotes/origin/main`

Offline envelope (`KUBECONFIG=/dev/null`); every command run from `tools/desk/`.

| # | Result |
|---|--------|
| 1 | PASS — `go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1` → exit 0, `ok` |
| 2 | PASS — `go vet ./internal/deskkit/` → exit 0 |
| 3 | PASS — `grep -q 'Derivation:' internal/deskkit/exitcodes.go` → exit 0 |
| 4 | PASS — `go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1 -v 2>&1 \| grep -q 'PASS'` → exit 0 |
| 5 | PASS — `go run ./cmd/muhar -spec internal/deskkit/mutations.json` → exit 0: "Harness healthy: baseline GREEN, positive control CAUGHT."; Totals 61 caught, 0 NOT CAUGHT; the new exit-code-convention mutant CAUGHT. One PRE-EXISTING corpus entry (`#203 — drop the FORWARD anchor`, `bodycheck.go`) reports COULD_NOT_MUTATE — its `old` text no longer matches the source after the acronym-matching refactor in #1643; stale on `origin/main` before this change and reported separately. |

**Fail-first (rule 9).** Red-then-green on the new pinning test with the spec's new
mutant planted by hand (`ExitRateLimited = 4` → `= 5` in `exitcodes.go`, then
reverted):

```
--- FAIL: TestExitCodeTableMatchesDerivation (0.00s)
    exitcodes_test.go:97: ExitRateLimited = 5, want 4 (the derivation pins each refusal class to its own code in 3..6)
    exitcodes_test.go:125: code 5 carries two refusal classes (rate-limited and constraint-refused) — one code per class
    exitcodes_test.go:130: the four refusal classes are not distinct codes: map[3:disabled 5:constraint-refused 6:unverifiable]
    exitcodes_test.go:134: code 4 carries no refusal class — the classes must be contiguous 3..6
FAIL
```

Reverted: `ok github.com/medici-finance/assay/tools/desk/internal/deskkit`. The same
swap is the spec's new mutation entry; muhar catches it only because the spec's fixed
`-run` allow-list was widened to name `TestExitCodeTableMatchesDerivation` (Task item
4) — before that widening the mutant would have been invisible to the corpus guard.

This is an IMPLEMENTER run, not a verdict — the frontmatter gate is `model`, and the
flip is the reviewer's / verify-gate's per the Review section below.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
