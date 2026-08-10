# Three-state instrument invariant

Every desk instrument — any program, script, query, or check whose output a human or
another instrument acts on — reports in **three states, never two**.

## The three states

| State | Meaning | Example output |
|-------|---------|---------------|
| **checked-clean** | The instrument ran to completion, examined the thing it guards, and found no defect. | `PASS`, `exit 0`, `✅`, a green row |
| **checked-failed** | The instrument ran to completion, examined the thing it guards, and found a defect. | `FAIL`, `exit 1`, `❌`, a red row, a PROBLEM line |
| **could-not-check** | The instrument could not examine the thing it guards — it never ran, was refused, timed out, received a truncated response, or found its own prerequisite missing. | `UNKNOWN`, `UNRUN`, `could-not-check`, a distinct non-green/non-red marker |

A two-state instrument (pass/fail) reports `could-not-check` as `pass` or `fail`, and
**both are lies**: "pass" when it never looked is a silent defect; "fail" on a network
blip is noise that trains the desk to ignore the output.

## Sub-rules

### 1. Absence of evidence ≠ evidence of absence

An instrument that receives no data, an empty response, or a `null` result MUST NOT
report "nothing to see." It MUST report `could-not-check` with the reason.

**Example:** a CI rollup that queries 0 checks because a pagination cap truncated the
list before the relevant check → `could-not-check (page cap reached, saw 0/N checks)`.
Currently it reports all green.

### 2. Truncation is a lie

Any limit — `--limit`, page size, node cap, sampling window — that silently drops data
MUST be surfaced. The output MUST name the limit, the count seen, and whether the limit
was reached.

**Example:** a PR monitor that fetches 100 reviews, hits the page cap at exactly 100, and
reports the last review — the next page was never fetched. Output MUST say
`could-not-check (page cap 100 reached; more reviews may exist)`.

### 3. Content-free input ≠ a check

A compliance check whose cheapest satisfying input is empty, or which can pass without
examining the guarded thing, is not a check — it is a green lamp. Every instrument MUST
have a **positive control**: prove it goes RED when the guarded thing is broken, at the
same revision.

**Example:** `grep -c ENC\[ k8s/dev/app/*pager*` with a glob that matches nothing →
exit 1, which an `|| grep -rl sops` fallback catches → exit 0. The row passes because
any SOPS file in the directory satisfies it, not because the target file is encrypted.
That row is not a check.

## Positive-control requirement

Before trusting a clean report from any instrument, prove the instrument FAILS when the
guarded thing is broken:

| Step | Action |
|------|--------|
| 1 | Break the guarded thing — revert the fix, remove the secret, drop the check. |
| 2 | Run the instrument. |
| 3 | Confirm it reports `checked-failed`. |

A green table that was never mutation-tested is the very defect this rule closes.
Any brief that adds a *check* MUST include a mutation-test Verify row
(§ brief-rules.md rule 16; shared-code changes additionally need a neighbour row per rule 17).

## Fleet audit

Every desk instrument enumerated, assessed against the three-state invariant, with a
disposition. Instruments already dispatched (#25, #34) are listed for completeness
but marked `dispatched` rather than re-covered.

### Board and dispatch instruments

| Instrument | Repo | What it prints when it cannot see | States | Disposition |
|-----------|------|----------------------------------|--------|-------------|
| `deskboard` ACTION column | oit | "must act" based on `lastCommit < lastReview` — never checks a worker exists or PR is still open | 2 (action / no-action) | `follow-up: #35` (ORPHANED vs BLOCKED) |
| Next-up board ("eligible") | oit | prints briefs whose `depends:` are satisfied without checking a worker is alive for in-flight items sharing the stream | 2 | `out-of-scope` (scheduling, not an instrument — the desk dispatcher owns the "is this available" check) |
| Intake untriaged-age alarm | assay-toolkit | prints age of oldest untriaged intake entry — never checks the entry still exists or is still triaged to the same state | 2 (alarm / silent) | `follow-up: #30` (the dominant-defect-class issue — silent-staleness is a two-state instance) |
| `why`-field lint | assay-toolkit | checks presence of `why:` in brief frontmatter — never checks the text is non-trivial or actually answers "why" | 2 (present / absent) | `out-of-scope` — rule 8 already states "quality is the human gate" for prose; the lint correctly gates presence only, and a content-quality gate is the reviewer's job, not a lint's |
| Register anti-falsification guard | assay-toolkit | detects file DELETION and in-place field-gutting (per PR #223: `resolved: no → yes`, `affects` narrowed/emptied, `ack` emptied — compared against merge-base with main). Emits a distinct degraded NOTICE when main is unreachable and the base falls back to HEAD (textbook `could-not-check`). | 3 (checked-clean / checked-failed / could-not-check) | `fixed-upstream` — PR #223 (merged 2026-07-31). The fleet's first fully three-state-compliant instrument. |
| STATUS.md single-writer check | oit | asserts `STATUS.md` was regenerated from current sources — does not verify the sources themselves are honest | 2 (regenerated / stale) | `out-of-scope` (anti-falsification is the register guard's job, not the single-writer check's) |

### CI and deploy instruments

| Instrument | Repo | What it prints when it cannot see | States | Disposition |
|-----------|------|----------------------------------|--------|-------------|
| `deskpost` CI rollup | oit | reads page 1 of N check runs; if the relevant check is on page 2, it reports "all green" rather than "I didn't see it" | 2 (green / red) | `follow-up: #33` (expected-checks per repo — deskpost must know which checks to expect before reporting "all green") |
| `medici-deploy` Job success condition | oit | exits 0 when the Job's own script completes — never checks the deploy actually landed on-cluster (RESULT: UPLOADED in logs) | 2 (exit-0 / exit-!0) | `follow-up: #30` (the deploy Job's two-state exit code is an instance of the dominant defect class) |
| `statusgen` DAR-drift PROBLEM | assay-toolkit | compares the deployed DAR version against the pinned release — but the comparison can't run if the deploy Job's log is unreachable, and the absence renders as "no PROBLEM" | 2 (PROBLEM / silent) | `out-of-scope` (the DAR-drift check is correct when it runs; the deploy Job's own three-state gap is `follow-up: #30`) |
| `statusgen` alarm flood notice | assay-toolkit | counts open findings; prints NOTICE at >7. Does not distinguish new-from-existing or check that each alarm is still actionable | 2 (notice / silent) | `out-of-scope` (advisory; the register-drain loop — `follow-up: desk-hardening/07` — handles staleness) |

### Runtime and debug instruments

| Instrument | Repo | What it prints when it cannot see | States | Disposition |
|-----------|------|----------------------------------|--------|-------------|
| PR monitor snapshot | oit | fetches per-repo PR lists via `gh`; on any single-repo failure, overwrites the prior good snapshot with an empty/partial one — no `MONITOR-WARN` | 2 (data / empty) | `follow-up: #36` (per-repo exit-code capture; carry prior snapshot forward on failure) |
| Debug-pod script exit codes | oit | `probe-canton.sh`, `verify-user.sh`, `check-reconciler.sh` all exit 0 on success — a network timeout, a `kubectl exec` failure, or a missing pod all exit non-zero, indistinguishable from a failing probe | 2 (exit-0 / exit-!0) | `follow-up: #30` (two-state exit codes are an instance of the dominant defect class) |
| `check-alert-placeholders.sh` | oit | greps for `DO-NOT-MERGE-UNSET-WEBHOOK` markers — never checks the actual secret values are functional (a placeholder that looks real passes) | 2 (marker-present / marker-absent) | `out-of-scope` (the marker is a positive-control mechanism; #1465 covers the functional-probe gap) |
| `writeguard` (F-34 isolation backstop) | oit | blocks Bash/Edit/Write targeting the shared checkout — false-positive blocks for paths under sanctioned prefixes (session scratchpads, worktrees) are `checked-failed` reported for a condition the guard never established | 2 (allow / block) | `follow-up: #30` — the two-state output class (no distinct "I may have blocked incorrectly" signal) is an instance of the dominant defect class |

### Dispatched (not re-covered)

These two issues have concrete, single-instance fixes dispatched as their own PRs.
This stream scopes the systemic remedy only; the instances are listed for completeness.

| Instrument | Issue | Disposition |
|-----------|-------|-------------|
| `deskboard` checks-passed vs no-checks-ran (the page-cap half of #33) | #33 | `dispatched` — #25 (concrete fix for the deskboard-specific half; the deskpost-rollup half stays in the audit above as `follow-up: #33`) |
| Mutation harness silently fails to mutate — reports "gate does not fire" | #34 | `dispatched` — #34 (concrete fix in its own PR) |

## Application to Verify tables

The three-state invariant applies to every Verify row. A row that cannot run (missing
dependency, no cluster access, wrong platform) MUST be recorded as explicitly **UNRUN**
with the reason, never silently skipped or assumed-pass. The `verify-desk` skill already
states this; this rule promotes it from the verify loop to every instrument.

A Verify table for a brief that adds a *check* MUST include a mutation-test row
(§ brief-rules.md rule 16). A brief that touches a *shared lister/flag/query* MUST
include a neighbour row (§ brief-rules.md rule 17).

1. **A mutation-test row** — revert the fix or break the guarded thing, confirm the
   check goes RED.
2. **A neighbour row** — for a change touching a shared lister/flag/query, one row
   exercises the *adjacent consumer*, not the deliverable.
