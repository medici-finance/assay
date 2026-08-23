# Brief template (brief-v2)

Copy the fenced block below into `docs/streams/<stream>/brief-<NN>-<slug>.md` and fill
every field. A brief is a **self-contained scope-and-DoD contract**: one agent must be
able to execute it without reading the rest of the plan. Fill every frontmatter field —
an empty `sources: []` or a missing `risk:` answer is a gap a reviewer bounces, not a
shortcut. The `<cell>` and `<repo>` segments of the `brief:` id come from
`docs/streams/graph-repos.yaml` (schema `graph-repos-v1`), the alias registry — the
repo segment is a registry ALIAS, never `<owner>/<name>`. See `brief-rules.md` for the
reason behind each rule.

```markdown
---
brief: <cell>:<repo>:<stream>:<NN>  # hierarchical typed ID, ALWAYS the full form in the file itself;
                                    # cell + repo alias from docs/streams/graph-repos.yaml, stream + NN
                                    # must match the file path and the stream README table row. NEVER a
                                    # prose name. References elsewhere (depends/unblocks/gates.on, the
                                    # `Brief:` PR trailer) also accept the elided forms `<stream>:<NN>`
                                    # and `<repo>:<stream>:<NN>` — each omitted prefix means "same as
                                    # the declaring brief". The brief-v1 `<stream>/<NN>` form is accepted
                                    # on READ for the migration window and rewritten by the migration;
                                    # v2 lint PROBLEMs it in a v2 file.
id: <uuid>                          # minted ONCE at authoring, never reused — the stable key a fact log
                                    # or an executor can reference across renames and re-homes. RESERVED
                                    # (optional under brief-v2; --lint validates shape only) — but mint
                                    # it: a uuid added later is a uuid with no history.
supersedes: []                      # object lineage: the briefs this one splits or re-baselines.
                                    # RESERVED, optional; --lint validates shape only.
version: 1                          # this brief's own revision. Bump on EVERY edit to Task or Verify
                                    # after first dispatch (re-baselines bump it); --lint PROBLEMs a
                                    # Task/Verify diff whose version did not change. Evidence and
                                    # witness rows record the version they ran against — a witness for
                                    # an older version renders `unknown (witness for vN, brief is vM)`,
                                    # never `verified`.
title: <one line>
wave: <int>                         # 0 = no dependencies; N = depends only on briefs in waves < N.
depends: []                         # typed IDs only: ["<stream>:<NN>", ...] (elided or fuller forms).
                                    # NEVER prose arrows ("after the X brief").
unblocks: []                        # typed IDs this brief enables. The inverse of another brief's depends.
effort: S | M | L                   # scheduling + execution tier. S may run inline; M/L plan-then-dispatch to cheap implementers.
gate: model | human                 # DERIVED, not chosen: any risk answer `yes` => human; all four `no` => model.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}   # record all four answers; the gate is their conclusion.
issues: []                          # tracker IDs this brief closes (optional).
schema: brief-v2                    # required on every brief in a v2 tree — an old pinned statusgen
                                    # REFUSES a v2 tree instead of silently ignoring it (fail-closed).
blocked-by: env                     # OPTIONAL, and `env` is the ONLY accepted value (anything else fails --lint).
                                    # Says the blocker is infrastructure/environment — not an agent, not a human
                                    # decision. The Awaiting board files the brief under `Env-blocked` and the
                                    # desk-actionable headline stops counting it. Omit it unless that is literally true:
                                    # the field's whole job is to keep the desk from re-triaging what it cannot move.
gates: []                           # RESERVED, OPTIONAL-but-KNOWN (brief-v2): behavioural/ordering edges —
                                    # parsed, type-checked, lint-validated, NOT yet gating (gating behaviour
                                    # is the graph stream's). Every entry is a MAPPING; `type` and `reason`
                                    # are required, because the point of the field is the machine-attached why:
                                    #   - on: <stream>/<NN>            # any ref from the grammar below
                                    #     type: behavioural-gate | ordering-gate | human-gate | external-env
                                    #     reason: "<the why, machine-attached>"
feathers: []                        # RESERVED, OPTIONAL-but-KNOWN (brief-v2): cross-repo/stream edges —
                                    # same reserved treatment as gates:. A scalar entry defaults to
                                    # type build-dep with no reason; a mapping carries the why:
                                    #   - ref: <repo-alias>:<stream>/<NN>
                                    #     type: build-dep | external-env | ordering-gate
                                    #     reason: "<the why>"
                                    # Ref grammar for on:/ref: (docs/dependency-graph-design.md §3.3):
                                    #   <stream>/<NN>                        in-repo brief (existing form)
                                    #   <repo-alias>:<stream>/<NN>           cross-repo brief
                                    #   <repo-alias>#<NNN>                   cross-repo issue
                                    #   #<NNN>                               in-repo issue
                                    #   <cell>:<repo-alias>:<stream>/<NN>    cross-cell brief (reserved)
                                    # Each omitted prefix segment means "same as the declaring brief";
                                    # aliases resolve through docs/streams/graph-repos.yaml.
authored: <YYYY-MM-DD> by <who/session>
sources: []                         # provenance: scoping doc / finding IDs / intake IDs this derives from. Empty => untraceable.
# parallel-streams: []              # OPTIONAL — declares this brief's work splits into concurrent SHARDS, each scoped to
#                                   # file globs: [{name: engine, files: ["statusgen/**"]}, {name: docs, files: ["docs/**"]}].
#                                   # Absent (the default, and every brief on file today) = ONE worker per brief, unchanged.
#                                   # Presence is a REQUEST: `statusgen shardcheck --brief <path> --root .` decides, and its
#                                   # exit 1 (a named collision) and exit 2 (something it could not analyse) BOTH mean run
#                                   # serially. When to write it, and which collision classes the check does and does not
#                                   # cover, is the author-brief skill's "Intra-brief splits" section — one source, not two.
# consumers: []                     # OPTIONAL — required only when this brief changes a SHARED VALUE (brief-rules.md rule 9).
#                                   # Each item: "<path-or-component>: fixed-here | follow-up <stream>/<NN> | out-of-scope (<why>)".
#                                   # Every entry is CORROBORATED against the branch diff by `statusgen --consumers`, which exits 1
#                                   # on a claim the diff contradicts — so add the Verify row below that runs it, and put the part
#                                   # no diff can settle (out-of-scope judgements, out-of-repo paths) in that row's Expect column.
#                                   # A site naming a SET (glob/braces) claims the whole set; a directory site matches its contents;
#                                   # entries unchanged since the merge-base belong to the branch that wrote them and read UNCHECKED.
# Deliberately NO `status:` key — a frontmatter status would be a cache of GitHub state committed
# to git, and a cache is what the phantom rows were; the lifecycle cell is DERIVED from witnesses
# (brief-rules.md rule 30), never asserted. There is no sidecar status file either, same reason.
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
# OPTIONAL `id` / `target` per row (RESERVED under brief-v2; --lint validates shape only):
# `id` (`v1`, `v2`, …) names the row so a witness points at THE row it ran; `target` names
# the verify substrate when it is not this repo's tree (a sibling repo, a live cluster) —
# the could-not-check-by-design class becomes a field instead of a NOTICE.
# OPTIONAL `Class` column (verdict-lane/02; the row-classes spec lives with that stream's
# design docs in the authoring repo and is staged separately).
# A table may declare each row's CLASS so the verdict lane can route it — add a `Class`
# column right after `#`:  | # | Class | Command | Expect |
#   - check:ci   HERMETIC: tree-only, no environment beyond the checkout. CI re-executes it
#                network-off and refuses the verdict on mismatch. Prefer this for rows that
#                CAN be hermetic (unit tests, a lint over the tree, a signature check).
#   - check      DETERMINISTIC but ENV-BOUND: needs a live PEM, a real queue, a tool on PATH.
#                A runner executes it; CI skips it (its verdict rests on authorship+signature).
#   - gate:model / gate:human  JUDGMENT rows a model / a human reads and decides. gate:human
#                stays on the verify-gate issue pair, outside the transcription lane.
# A table WITHOUT a Class column is legacy: every row is treated as `check`. Nothing is forced.
# SCRIPTED rows: a check:ci / check row may be a reviewed script instead of an inline command —
#   docs/streams/<stream>/verify.d/brief-NN/row-K.sh  (executable, exit 0 = PASS)
# and then the row's Command cell IS that script path. The reviewer who approves the brief
# approves the script. An unknown class, or a scripted row whose script is missing/not +x on a
# non-todo brief, is a `statusgen --lint` PROBLEM.
# For PROSE deliverables (docs, articles) the Verify table is a PRESENCE gate, not a
# quality gate: it checks required elements exist (a file, a section, a token). State that
# honestly here — quality is owned by the human review gate. Posing grep rows as quality DoD is checkmark-DoD.
# DEREFERENCING vs PRESENCE rows (brief-rules.md rule 43) — two different things that both
# look like an ordinary Verify row:
#   - PRESENCE/formatting: `grep -c`, `wc -w`, "section exists" — proves the doc is well-formed.
#     Cannot fail on a wrong-but-well-formed doc: a confident falsehood in the right section, at
#     the right length, passes exactly like the truth would.
#   - DEREFERENCING: fetch a link and check what it actually serves; run a documented command
#     and compare its real output/exit code to the specific claim made in the doc; check a
#     documented ID against the live system it names. CAN fail on a wrong-but-well-formed doc.
# If this brief's deliverable makes a checkable factual claim (a setup/config guide, a market
# or competitive analysis, a spec — anything asserting "X is true" a reader could act on), at
# least ONE Verify row MUST dereference something, not just count it. Triggering evidence: a
# Verify table built entirely of grep-presence rows passed 8/8 on a guide that was factually
# wrong in 4 places, one load-bearing (#19, desk-apps/02); the sibling failure mode is a
# citation link left unresolved (#17, assay-product/02). Skip this only for genuinely
# presence-only content with no factual claim to dereference (a docs reformat, a template
# scaffold). This is a different axis from the row-runner rules (brief-rules.md 25-29): those
# read the command text and are linted; this one is a judgement call and is not.
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
     (command, exit code, output line(s) or hash, date, runner). Witness rows record
     the brief `version:` they ran against.
     The `verified` status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: <model|human> (from frontmatter). Reviewer records verdict + date in the stream
README table. Human gate is MANDATORY when any risk answer is yes.
```

## Notes on filling it in

- **`brief`/`depends`/`unblocks` are typed IDs** so a rename or renumber does not silently
  break a reference and a script can follow the graph. The file's own `brief:` is always
  the full four-segment form; references may elide leading segments (each omitted prefix
  means "same as the declaring brief").
- **`version` is the brief's own revision counter**, not the schema's: `schema` names the
  contract the file is written against, `version` counts edits to the contract's
  observable surface (Task/Verify) after first dispatch. Witnesses record the version
  they ran against, so a stale witness reads as `unknown`, never as `verified`.
- **`id`/`supersedes`/`gates`/`feathers` are RESERVED keys**: parsed, type-checked, and
  lint-validated under brief-v2, but nothing gates on them yet — that behaviour is the
  graph stream's (see `docs/dependency-graph-design.md`). Reserve cheap now over
  retrofit expensive later; do mint `id` at authoring.
- **The lifecycle cell is never a frontmatter field** — the board derives it from
  witnesses (brief-rules.md rule 30); see the note at the foot of the template block.
- **`wave`** is derivable from `depends` (max dependency wave + 1) — keep them consistent.
- **`gate` is a conclusion, not an opinion**: compute it from the four `risk` answers.
- **`sources`** must name at least one origin (scoping doc, finding `F-NN`, intake `I-NN`),
  or no one can tell why the work exists or whether it is still needed.
- **Verify rows must be runnable by someone who did not do the work** — a row with no
  literal command and no expected exit/output is a hope, not a check.
