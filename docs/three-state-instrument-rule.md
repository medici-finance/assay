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
