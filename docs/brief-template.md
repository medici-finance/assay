# Brief template (brief-v1)

Copy the fenced block below into `docs/streams/<stream>/brief-<NN>-<slug>.md` and fill
every field. A brief is a **self-contained scope-and-DoD contract**: one agent must be
able to execute it without reading the rest of the plan. Fill every frontmatter field —
an empty `sources: []` or a missing `risk:` answer is a gap a reviewer bounces, not a
shortcut. See `brief-rules.md` for the reason behind each rule.

```markdown
---
brief: <stream>/<NN>                # typed ID; must match the stream README table row. NEVER a prose name.
title: <one line>
wave: <int>                         # 0 = no dependencies; N = depends only on briefs in waves < N.
depends: []                         # typed IDs only: ["<stream>/<NN>", ...]. NEVER prose arrows ("after the X brief").
unblocks: []                        # typed IDs this brief enables. The inverse of another brief's depends.
effort: S | M | L                   # scheduling + execution tier. S may run inline; M/L plan-then-dispatch to cheap implementers.
gate: model | human                 # DERIVED, not chosen: any risk answer `yes` => human; all four `no` => model.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}   # record all four answers; the gate is their conclusion.
issues: []                          # tracker IDs this brief closes (optional).
schema: brief-v1                    # marks this file for the validator; files without it are exempt (legacy opt-in).
blocked-by: env                     # OPTIONAL, and `env` is the ONLY accepted value (anything else fails --lint).
                                    # Says the blocker is infrastructure/environment — not an agent, not a human
                                    # decision. The Awaiting board files the brief under `Env-blocked` and the
                                    # desk-actionable headline stops counting it. Omit it unless that is literally true:
                                    # the field's whole job is to keep the desk from re-triaging what it cannot move.
authored: <YYYY-MM-DD> by <who/session>
sources: []                         # provenance: scoping doc / finding IDs / intake IDs this derives from. Empty => untraceable.
# consumers: []                     # OPTIONAL — required only when this brief changes a SHARED VALUE (brief-rules.md rule 9).
#                                   # Each item: "<path-or-component>: fixed-here | follow-up <stream>/<NN> | out-of-scope (<why>)".
#                                   # Every entry is CORROBORATED against the branch diff by `statusgen --consumers`, which exits 1
#                                   # on a claim the diff contradicts — so add the Verify row below that runs it, and put the part
#                                   # no diff can settle (out-of-scope judgements, out-of-repo paths) in that row's Expect column.
#                                   # A site naming a SET (glob/braces) claims the whole set; a directory site matches its contents;
#                                   # entries unchanged since the merge-base belong to the branch that wrote them and read UNCHECKED.
---

# Brief <NN> — <title>

## Context
files: <exact paths the implementer touches — no repo exploration should be needed>
facts: <the 3-5 project facts required to execute — key: value, no narrative>
# If this brief changes a SHARED VALUE (a party/identity, env-var name, config key, a
# field's meaning, a wire/JSON format, a default — anything another component reads),
# fill the consumers: frontmatter field above — grep for every reader and route each one.
# An unlisted consumer is a stranded assumption; an unrouted one is a claim nothing checks.
# If this brief is itself dispatch-shaped (it dispatches work to another agent/worker),
# state its authority envelope (repos/paths/tools/budget it grants, a subset of its own)
# and its declared exclusions (what was deliberately left out of scope) — one line each
# (brief-rules.md rules 22-24).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
<what to build/change — explicit steps. A brief is a contract, not a step-by-step plan;
keep it to scope + intent, not a keystroke script.>

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | <literal command a non-implementer can run> | exit 0; output contains "<literal>" |
# For PROSE deliverables (docs, articles) the Verify table is a PRESENCE gate, not a
# quality gate: it checks required elements exist (a file, a section, a token). State that
# honestly here — quality is owned by the human review gate. Posing grep rows as quality DoD is checkmark-DoD.
# If this brief changes a shared value, add a FLOW-level row: not "the changed site behaves"
# but "the cross-component flow that depends on this value still completes end-to-end."
# It also carries the consumers: corroboration — `statusgen --consumers --brief <stream>/<NN>`,
# expecting exit 0 with the UNCHECKED entries named in the Expect column for the reviewer to weigh.
# Re-run after the merge it exits 2 (COULD-NOT-CHECK): the diff that made the claims true is gone.
# To re-corroborate a merged brief, give it that diff — see brief-rules.md rule 9 for the recipe.
# If this brief adds a CHECK or guard, add a MUTATION-TEST row (brief-rules.md rule 16):
# revert the fix or break the guarded thing, confirm the check goes RED.
# If this brief touches a shared lister/flag/query, add a NEIGHBOUR row (brief-rules.md rule 17):
# one row exercises the adjacent consumer of the shared code path, not the deliverable.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     The `verified` status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: <model|human> (from frontmatter). Reviewer records verdict + date in the stream
README table. Human gate is MANDATORY when any risk answer is yes.
```

## Notes on filling it in

- **`brief`/`depends`/`unblocks` are typed IDs** so a rename or renumber does not silently
  break a reference and a script can follow the graph.
- **`wave`** is derivable from `depends` (max dependency wave + 1) — keep them consistent.
- **`gate` is a conclusion, not an opinion**: compute it from the four `risk` answers.
- **`sources`** must name at least one origin (scoping doc, finding `F-NN`, intake `I-NN`),
  or no one can tell why the work exists or whether it is still needed.
- **Verify rows must be runnable by someone who did not do the work** — a row with no
  literal command and no expected exit/output is a hope, not a check.
