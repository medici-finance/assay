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

**Example:** `grep -c ENC\[ config/secrets/*.env` with a glob that matches nothing →
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

`mistake-proofing.md` promotes this positive-control clause from single instruments to
every device in the system (its **D1**), and adds the classification — timing × mode ×
bypass — that says where a device sits and how strong it actually is.

## Auditing a fleet against this rule

The class is closed by **coverage, not by fixing everything at once**. Enumerate every
instrument, record what each one does when it cannot see, and give each a disposition.
An instrument with a recorded disposition is closed even where the fix is deferred; an
instrument nobody looked at is the open risk.

One row per instrument, four columns:

| Column | What goes in it |
|--------|-----------------|
| **Instrument** | The program, script, query, or check — named precisely enough to run. |
| **What it prints when it cannot see** | The *literal* output. Not a paraphrase: the exact string, and the exit code beside it. This is the column that does the work — writing the real string is usually the moment the two-state ones become obvious. |
| **States** | 2 or 3. Three requires a distinct output for could-not-check; a NOTICE on stderr while the artifact still reads clean is **two**. |
| **Disposition** | `fixed-here` · `fixed-upstream` · `follow-up: <ref>` · `out-of-scope <why>`. |

Three findings recur often enough to look for them by name:

1. **An absent data source becomes an empty result set.** A "not found, return nothing"
   branch feeds a renderer written to describe emptiness affirmatively, and
   *"I could not read my input"* prints as *"I read it and it was clean."*
2. **The warning and the exit code disagree.** A tool writes a real problem to stderr and
   exits 0. Every caller checking `$?` — which is every caller — reads checked-clean.
   Suppressing the warning with a quiet flag hides it completely.
3. **A cap is reached but not reported.** A page limit, node cap, or result limit is hit,
   and the count printed is a floor presented as a total.

An instrument that reports a floor must say so in the same breath as the number.

### The disposition is a claim, and claims get checked

`fixed-upstream` means *someone else's merged change closes this*. Verify that the change
is merged, not merely open — an open pull request fixes nothing, and an audit row that
records an intention as an outcome is itself an instrument reporting a pass without
looking. Record which it is.

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
