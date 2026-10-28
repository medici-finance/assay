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
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1` | exit 0; output contains "ok" |
| 2 | check | `cd tools/desk && go vet ./internal/deskkit/` | exit 0 |
| 3 | check | `cd tools/desk && grep -q 'Derivation:' internal/deskkit/exitcodes.go` | exit 0 (the derivation block is present) |
| 4 | check | `cd tools/desk && go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1 -v 2>&1 \| grep -q -- '--- PASS: TestExitCodeTableMatchesDerivation'` | exit 0 (anchored on the named test's own PASS line, not the bare string "PASS", which a nonexistent `-run` match would also print) |
| 5 | check +mutation | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/mutations.json` | exit 0 — baseline GREEN, positive control CAUGHT, and every mutation CAUGHT, including the new exit-code-convention mutant (only reachable once the `"test"` field names `TestExitCodeTableMatchesDerivation`) |

## Evidence
<!-- appended at implementation time by a non-implementer -->

### Implementer run — derivation recorded; pinning test red-then-green; corpus mutant CAUGHT — 2026-09-27, implementer (worker), branch `feat/measured-status-01` off `refs/remotes/origin/main`

Offline envelope (`KUBECONFIG=/dev/null`); every command run from `tools/desk/`.

| # | Result |
|---|--------|
| 1 | PASS — `go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1` → exit 0, `ok` |
| 2 | PASS — `go vet ./internal/deskkit/` → exit 0 |
| 3 | PASS — `grep -q 'Derivation:' internal/deskkit/exitcodes.go` → exit 0 |
| 4 | PASS — `go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1 -v 2>&1 \| grep -q -- '--- PASS: TestExitCodeTableMatchesDerivation'` → exit 0 |
| 5 | PASS-with-one-could-not-check — `go run ./cmd/muhar -spec internal/deskkit/mutations.json` → exit 0: "Harness healthy: baseline GREEN, positive control CAUGHT."; Totals 61 caught, 0 NOT CAUGHT, 1 could-not-mutate; the new exit-code-convention mutant CAUGHT. The one could-not-mutate entry (`#203 — drop the FORWARD anchor`, `bodycheck.go`) is PRE-EXISTING — its `old` text no longer matches the source after the acronym-matching refactor in #1643, confirmed absent at both the merge-base and current main — and is unrelated to this diff; tracked separately at #1725. It counts as could-not-check for that one entry, not as a pass. |

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

### Verification — VERIFY: FAIL (row 4, check definition) — 2026-10-21

Non-implementer verification on merged main 48532629922d99cb26077c8baa2ea901af811110 (delivered by 26bf76fc3, #1726). Gate: model; all four risk answers no. Every row ran network-off in the local container (image assay-verify:go1.27-py, go1.27.2 linux/arm64, `--network none`, GOPROXY=off, GOTOOLCHAIN=local, module cache mounted read-only) against a `git archive` of the attested source, so the home tree stayed clean. Rows 1, 2, 3 and 5 have no pipe, so pipefail does not change them. Row 4 was run under the canonical default shell `bash -o pipefail -c` (brief-rules, Shell column default `sh`) and also in plain bash.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1` | exit 0; output contains "ok" | exit 0; "ok github.com/medici-finance/assay/tools/desk/internal/deskkit 0.027s" | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go vet ./internal/deskkit/` | exit 0 | exit 0, no output | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd tools/desk && grep -q 'Derivation:' internal/deskkit/exitcodes.go` | exit 0 (the derivation block is present) | exit 0; the block opens at exitcodes.go line 16 | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1 -v 2>&1 \| grep -q -- '--- PASS: TestExitCodeTableMatchesDerivation'` | exit 0 | FAIL: exit 141 under `bash -o pipefail -c`, 6 of 6 runs. grep -q exits on its first match and go test then takes SIGPIPE writing its trailing PASS/ok lines. Plain bash exits 0. Diagnostic: the -v output does carry "--- PASS: TestExitCodeTableMatchesDerivation (0.00s)", and the `grep -c` form prints 1 on 3 of 3 runs. The test passes; the row as written fails | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/mutations.json` | exit 0; baseline GREEN, positive control CAUGHT, every mutation CAUGHT incl. the exit-code-convention mutant | exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT."; "CAUGHT swap a refusal-class code onto its neighbour (rate-limited takes the constraint-refused code …)"; "Totals: 131 caught, 0 NOT CAUGHT, 0 could-not-mutate." | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Verification-Attestation: medici-finance/assay#3952 run=8e9d310298f303bf4f551d155bb5fad29a8cf407b570f902d35251c06fa9f465 source=48532629922d99cb26077c8baa2ea901af811110 brief=docs/streams/measured-status/brief-01-exit-code-derivation.md model=claude-opus-5-5 tier=any

Risk-bearing values. Risk metadata is present and all four answers are no, including irreversible: no. Enumerated over the 26bf76fc3 diff (exitcodes.go, exitcodes_test.go, mutations.json), re-located at 48532629922d:
- ExitOK = 0 @ tools/desk/internal/deskkit/exitcodes.go:41
- ExitDisabled = 3 @ tools/desk/internal/deskkit/exitcodes.go:44
- ExitRateLimited = 4 @ tools/desk/internal/deskkit/exitcodes.go:48
- ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51
- ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:55
- the test's want literals 0/3/4/5/6 @ tools/desk/internal/deskkit/exitcodes_test.go:90-94 and the reserved set {1, 2} @ exitcodes_test.go:105 (these mirror the table)
- the mutant's replacement literal ExitRateLimited = 5 @ tools/desk/internal/deskkit/mutations.json:531 (test data only)

The diff changes no value; it records a derivation for values already in place. Each one is a reversible caller contract, fixed by an edit and a release. The cost of a wrong value is a caller branching on the wrong integer, and that cost grows with the number of scripts that hard-code the integers. So the four refusal codes rank first and ExitOK ranks last.

RISK-VALUE: DERIVED — ExitDisabled = 3 @ tools/desk/internal/deskkit/exitcodes.go:44 — the lowest value that collides with no shell-reported status. 0 is success. 1 is the general-error status most utilities use. 2 is the status the bash manual documents for a builtin's incorrect usage ("All builtins return an exit status of 2 to indicate incorrect usage"). POSIX XCU 2.8.2 reserves 126, 127 and >128. A tool's first own code is therefore 3.
RISK-VALUE: DERIVED — ExitRateLimited = 4, ExitRefused = 5, ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:48, :51, :55 — one code per caller-distinguishable class (wait and retry the same call / fix input and re-run / fail closed), assigned in handling order from 3. That makes the values 4, 5 and 6, all below the 126 floor. Splitting ExitRefused per reason would add no branch a caller could take differently, because every compiled-in refusal gets the same action. The test pins all four values, and the muhar mutant that swaps two of them is CAUGHT (row 5).

The implementation meets the brief's contract: the Derivation block is present and correct, the pinning test passes, and the corpus catches the new mutant. The defect is in the Verify table. Row 4 pipes go test into `grep -q`, which is the same SIGPIPE-under-pipefail class as #2241 (measured-status/04). Remedy: re-baseline row 4 to the pipefail-safe form, e.g. `… 2>&1 | grep -c -- '--- PASS: TestExitCodeTableMatchesDerivation'` with Expect "prints 1, exit 0". Then re-verify.

**VERIFY: FAIL** — row 4 exits 141 under the canonical `bash -o pipefail -c` shell (6/6 runs); rows 1, 2, 3 and 5 pass (4/5); the code meets the contract; the defect is in the Verify-table check definition (blocker: #4170).
### Re-verification — VERIFY: FAIL (row 4, check definition, unchanged) — 2026-10-27

Non-implementer re-verification on merged main 384d5eda2ca230e7e748750fdaed6ba4d1d50ed8 (work delivered by 26bf76fc31618e542867b7c9758de74285899d46, #1726, a single-parent merge whose parent b326e9202ed8f8c57eb1adcd629f80d6a4850ad0 was passed as the fail-first base). Gate: model; all four risk answers no. Witness below is `statusgen verifyrun --fail-first --base b326e9202ed8f8c57eb1adcd629f80d6a4850ad0` (statusgen v1.0.63) run on the host (darwin/arm64, go1.27.2) from the admitted detached home; every row is class `check` (none is `check:ci`), so no network sandbox applies. The table carries no risk-bearing row tag and the brief-wide risk answers are all no, so the fail-first pass recorded every row not-selected at base. The Verify table at this source is byte-identical to the one the 2026-10-21 run used: the row 4 check definition has not been re-baselined since blocker #4170 was filed.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1` | pass exit=0 | sha256:0ab33ad3e93d | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=384d5eda2ca2 |
| 2 | `cd tools/desk && go vet ./internal/deskkit/` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=384d5eda2ca2 |
| 3 | `cd tools/desk && grep -q 'Derivation:' internal/deskkit/exitcodes.go` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=384d5eda2ca2 |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -run TestExitCodeTableMatchesDerivation -count=1 -v 2>&1 \| grep -q -- '--- PASS: TestExitCodeTableMatchesDerivation'` | fail exit=141 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=384d5eda2ca2 |
| 5 | `cd tools/desk && go run ./cmd/muhar -spec internal/deskkit/mutations.json` | pass exit=0 | sha256:aa9557f366e7 | 2026-10-27 | assay-verifier-app[bot] @ 384d5eda2ca2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=384d5eda2ca2 |
Verification-Attestation: medici-finance/assay#4712 run=170345a8766eb08198ec356a8ffe05681b30a8b38305acd748f04c37e729a761 source=384d5eda2ca230e7e748750fdaed6ba4d1d50ed8 brief=docs/streams/measured-status/brief-01-exit-code-derivation.md model=claude-opus-5-5 tier=strong

Supplemental host observation (no acceptance credit; diagnoses row 4 only): from tools/desk at the same source, the row 4 pipeline under `bash -o pipefail -c` exited 141 on 3 of 3 runs; the same pipeline under plain `bash -c` exited 0; the `grep -c` form under `bash -o pipefail -c` printed 1 and exited 0. So the named test does print its own "--- PASS: TestExitCodeTableMatchesDerivation" line (row 1 also passes), and the row fails because `grep -q` exits on its first match and go test then takes SIGPIPE writing its trailing lines. This is the defect #4170 records.

Risk-bearing values. Risk metadata is present and all four answers are no, including irreversible: no. Enumerated over the delivering diff (exitcodes.go, exitcodes_test.go, mutations.json under tools/desk/internal/deskkit), re-located at 384d5eda2ca2:
- ExitOK = 0 @ tools/desk/internal/deskkit/exitcodes.go:41
- ExitDisabled = 3 @ tools/desk/internal/deskkit/exitcodes.go:44
- ExitRateLimited = 4 @ tools/desk/internal/deskkit/exitcodes.go:48
- ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51
- ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:55
- the test's want literals 0/3/4/5/6 @ tools/desk/internal/deskkit/exitcodes_test.go:90-94, the shell-reserved literals 1 and 2 @ exitcodes_test.go:105, and the contiguous range 3..6 @ exitcodes_test.go:132 (these mirror the table)
- the mutant's replacement literal ExitRateLimited = 5 @ tools/desk/internal/deskkit/mutations.json:531 (test data only)

Since delivery, exitcodes.go gained a ReadClass field, a ReadClassOf helper and a semantic tag; no constant value or line in the const block moved. The delivering diff changes no value; it records a derivation for values already in place. Each is a reversible caller contract, fixed by an edit and a release; a wrong value costs a caller branching on the wrong integer, a cost that grows with every script that hard-codes the integers. The four refusal codes rank first, ExitOK last.

RISK-VALUE: DERIVED — ExitDisabled = 3 @ tools/desk/internal/deskkit/exitcodes.go:44 — the lowest status that collides with no status the shell itself reports: 0 is success, 1 is the general-error status most utilities use, 2 is the status the bash manual documents for a builtin's incorrect usage, and POSIX XCU 2.8.2 reserves 126, 127 and above 128. A tool's first own code is therefore 3.
RISK-VALUE: DERIVED — ExitRateLimited = 4, ExitRefused = 5, ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:48, :51, :55 — one code per caller-distinguishable class (stop / wait and retry the same call / fix the input and re-run / fail closed), assigned contiguously in handling order from 3, all below the 126 floor. A per-reason split of ExitRefused would add no branch a caller could take differently, because every compiled-in refusal gets the same action. Rows 1 and 5 witness the pin and the CAUGHT neighbour-swap mutant at this source.

The implementation meets the brief's contract (rows 1, 2, 3 and 5 pass). The defect is in the Verify table: row 4 as written exits 141 under the canonical `bash -o pipefail -c` row shell. Remedy (unchanged): re-baseline row 4 to a pipefail-safe form, e.g. the `grep -c` form with Expect "prints 1, exit 0", then re-verify.

**VERIFY: FAIL** — row 4 exits 141 under the canonical `bash -o pipefail -c` row shell (statusgen verifyrun witness; 3/3 supplemental runs); rows 1, 2, 3 and 5 pass (4/5); the code meets the contract; the defect is the unchanged Verify-table check definition (blocker: #4170).

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
