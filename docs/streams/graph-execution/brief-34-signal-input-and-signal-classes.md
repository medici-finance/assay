---
brief: assay:assay:graph-execution:34
title: Signal input, signal classes and decision options in the workflow-pattern schema
why: >-
  A red main, a firing alert or a dashboard crossing a band has no shape in the pattern
  schema today: an input can only name an authored artifact, a decision cannot list its
  options or carry a recommended default, and there is nowhere to state which runs a signal
  admits or what a culprit record holds. The signal-triggered pattern (graph-execution/08)
  cannot be written until those keys and their lint rules exist. This amendment adds them
  without granting any role a new effect, so containment stays a person's act.
wave: 1
depends: ["graph-execution/33"]
unblocks: ["graph-execution/08"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [4506]
schema: brief-v2
version: 1
id: fa5eb92b-de5d-4724-908b-ca54fadc34e6
authored: 2026-10-27 by worker-desk (author-brief)
sources:
  - "per the maintainer's ruling choosing option A: https://github.com/medici-finance/assay/issues/4506#issuecomment-6454769886"
  - "per the maintainer's ruling on the role-effect table: https://github.com/medici-finance/assay/issues/4506#issuecomment-6455243468"
  - "medici-finance/assay#4506 — the gap report: the eight lint findings a probe of 08's facts produced against the shipped schema, and options A/B/C"
  - "docs/streams/graph-execution/brief-08-signal-triggered-pattern.md — the obligations this schema must be able to express (Context facts, Task 5, Verify rows 3 and 9-12)"
  - "docs/streams/graph-execution/brief-33-merge-decision-node-and-risk-input.md — adds `human` to the node role enum, allowed only on effect-free decision nodes, and the lint rule `pattern-human-role-not-decision`"
  - "spec/workflow-pattern-v1.md §3, §4.1, §4.2, §7 and schemas/workflow-pattern-v1.json — the contract amended here"
  - "statusgen/patterns.go (patternRoleEffectPermissions, checkPatternMustRules, parsePatternDoc) and statusgen/conform.go (schemaKeywords — the closed keyword set the in-tree schema validator implements)"
  - "freshness-checked 2026-10-27 @ 7fcd9d284 (origin/main): the node role enum is still `desk | reviewer | verifier | worker`; an input item still allows only `artifact` and `revision` and requires `artifact`; no top-level key holds signal classes; `grep -rn 'liveness-check\\|signal-classes' schemas statusgen spec` returns zero hits"
exec-tier: strong
exec-tier-why: "(a) the exact key shapes and rule boundaries are design choices the facts bound but do not fully fix; (b) one change spans two schema copies, the lint, the spec text, the enforcement registry and its generated copies; (c) a rule that never fires passes every happy-path test"
domain: complicated
outcome: none
consumers:
  - "schemas/workflow-pattern-v1.json, statusgen/schemas/workflow-pattern-v1.json (byte-identical twin): follow-up graph-execution/34 (this brief; flips to fixed-here when the implementation edits them)"
  - "statusgen/patterns.go, statusgen/patterns_test.go, statusgen/testdata/patterns/: follow-up graph-execution/34 (this brief)"
  - "spec/workflow-pattern-v1.md: follow-up graph-execution/34 (this brief)"
  - "statusgen/enforcementstatus.go and the generated enforcement tables in plugins/assay/skills/author-brief/SKILL.md and .cursor/skills/author-brief/SKILL.md: follow-up graph-execution/34 (this brief; the tables are regenerated, never hand-edited)"
  - "spec/workflow-patterns/signal-triggered-v1.yaml (planned): follow-up graph-execution/08 (the first shipped pattern that uses these keys)"
  - "spec/workflow-patterns/implementation-v1.yaml, spec/workflow-patterns/research-v1.yaml: out-of-scope (unchanged; they use none of the new keys and Verify row 2 proves they still lint clean)"
  - "statusgen/instance.go, schemas/workflow-instance-v1.json: out-of-scope (no instance of a signal-instantiated pattern is produced until a dispatch brief exists; instances of the two shipped patterns use no new key)"
  - "tools/desk/internal/deskkit/admission.go (LoadPatternRiskInput): out-of-scope (decodes only schema, pattern, version, risk-input and node ids with a non-strict decoder, so the new keys are ignored)"
  - "statusgen/coverage.go (loadPatternDocs): out-of-scope (reads node evidence only; no new key changes what it reads)"
  - "pattern role-effect table (spec/workflow-pattern-v1.md §7, patternRoleEffectPermissions): out-of-scope (not widened, per the maintainer's ruling)"
---

# Brief 34 — Signal input, signal classes and decision options in the workflow-pattern schema

## Context

files: `schemas/workflow-pattern-v1.json`, `statusgen/schemas/workflow-pattern-v1.json`,
`statusgen/patterns.go`, `statusgen/patterns_test.go`, `statusgen/enforcementstatus.go`,
`spec/workflow-pattern-v1.md`, `statusgen/testdata/patterns/good-signal-example.yaml` (planned),
`statusgen/testdata/patterns/bad-signal-input-not-entry.yaml` (planned),
`statusgen/testdata/patterns/bad-replay-not-mandatory.yaml` (planned),
`statusgen/testdata/patterns/bad-admission-pull-request.yaml` (planned),
`statusgen/testdata/patterns/bad-remedy-open.yaml` (planned),
`statusgen/testdata/patterns/bad-refused-paths-dropped.yaml` (planned),
`statusgen/testdata/patterns/bad-judgement-no-default.yaml` (planned),
`statusgen/testdata/patterns/bad-withheld-mechanical.yaml` (planned),
`statusgen/testdata/patterns/bad-containment-open.yaml` (planned),
`statusgen/testdata/patterns/bad-options-on-effect.yaml` (planned),
`statusgen/testdata/patterns/bad-exit-effect-no-admission.yaml` (planned),
`statusgen/testdata/patterns/bad-mechanical-no-culprit.yaml` (planned),
`plugins/assay/skills/author-brief/SKILL.md` and `.cursor/skills/author-brief/SKILL.md`
(generated enforcement table only), `changelog/graph-execution-34-signal-classes.md` (planned).

single-point-of-failure: the ONE control for the containment boundary is that a containment
step is a `human` decision node, which carries no effects. Two layers stand behind it, each in a
different component on a different signal: (1) the pattern schema refuses any value outside the
closed containment set, by an items enum that needs no lint (checked by an independent JSON Schema
validator in row 12); (2) the pattern lint refuses a containment list that is not exactly the set,
or sits on a node that is not an effect-free `human` decision (`pattern-containment-set`, plus
33's `pattern-human-role-not-decision`), and it reports even when the schema also refuses. The
out-of-band layer is unchanged from brief 02: the roster's role binding, not the pattern,
authorises a forge write, and `human` binds no identity at all.

facts (7fcd9d284, 2026-10-27):
- **Ruling.** The maintainer chose option A on medici-finance/assay#4506 (a schema and lint
  amendment lands first, then 08 lands on top) and ruled the role-effect table: no widening.
  §7 stays as is (`worker`: `push`, `pr-open`, `comment`). Containment and confirm-filing
  become `human`-owned `kind: decision` nodes that list the containment taxonomy as outcomes,
  and this amendment is its own brief that 08 depends on. Both links are in `sources:`.
- **Relation to graph-execution/33: depend, do not subsume.** 33 already adds `human` to the
  node `role` enum, restricts it to `kind: decision` nodes with no `effects`, and adds the lint
  rule `pattern-human-role-not-decision`. That settles gap 1 of the issue. 33 is authored,
  `gate: human`, carries its own ruling and edits the same schema and lint files. Subsuming it
  would duplicate a human-gated change, and a parallel edit of the role enum would conflict with
  it. This brief therefore depends on 33, uses `role: human` as 33 defines it, and does not touch
  the role enum or the §7 table.
- **Shipped schema (`schemas/workflow-pattern-v1.json`).** Top level and every node, input
  item and evidence item set `additionalProperties: false`. An input item is
  `{artifact, revision?}` with `required: [artifact]`. A node has no field for decision options
  or a default. No top-level key can hold signal classes.
- **The in-tree validator's keyword set is closed.** `statusgen/conform.go`'s `schemaKeywords`
  implements `type`, `const`, `enum`, `required`, `properties`, `additionalProperties`, `items`,
  `minItems`, `minimum`, `pattern` (plus annotations). A coverage test refuses a committed schema
  that uses any other keyword. If 33 extends that set, this brief may use the extension. It never
  adds a keyword without adding it to the validator and that test in the same change. Every rule
  below that needs a keyword outside the set (exactly-one-of, conditional presence, null-valued
  fields) is a lint rule, not a schema rule.
- **The lint already runs MUST rules after a shape failure.** `lintPatternFile` runs the schema
  pass and then `checkPatternMustRules` on whatever decoded, so a fixture that breaks both the
  shape and a MUST rule reports both tags in one run.
- **Rule registration.** Each pattern rule is a `rulePattern…` constant in `statusgen/patterns.go`
  with a row in `statusgen/enforcementstatus.go` (all `StatusFatal`). The generated enforcement
  tables in the two author-brief SKILL.md copies are rebuilt with `make skillslint` after
  `statusgen enforcement-status`; skillslint's byte-diff gate fails a stale copy.

layering: no new component. The schema owns closed value sets and key shapes; the pattern lint
owns every rule that needs the resolved graph or a cross-field condition; the spec text states
both. Runtime behaviour (holding an instance whose liveness check is could-not-check, refusing a
run at admission, enforcing limits) is not built here. It belongs to the eligibility evaluator
(graph-execution/01) and the auto-triage briefs, which read what this schema lets a pattern declare.

design-fit:
  owner: statusgen/patterns.go (pattern contract)
  contract: none — the pattern contract is spec-owned (spec/workflow-pattern-v1.md) with no semantic-owner index row
  retires: []
  weight: "verbs 0, flags 0, refusals +8 lint rules, schema keys +5 (input `signal`; node `options`, `default`, `containment`; top-level `signal-classes`), rule-text lines +~60 in one spec"
  why-add: graph-execution/08's obligations have no place in the closed schema, and widening the §7 role table (the only alternative that needed no new key) was ruled out by the maintainer

## Ground rules

- NEVER git push, trigger workflows or run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented`; you do not set verified or done.
- Do not widen the §7 role-effect table or `patternRoleEffectPermissions`, and do not change the
  node `role` enum (33 owns it).
- Do not author `spec/workflow-patterns/signal-triggered-v1.yaml` (planned); that is graph-execution/08.
- Public tree: every example signal, source and repository in a fixture uses `example-org/*` names.
- If 33 has not reached `implemented` on main when you start, or its shipped shape differs from
  the facts above, report NEEDS_CONTEXT; do not re-implement any of it here.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Signal input.** In `schemas/workflow-pattern-v1.json`, give an input item a second shape:
   add an optional `signal` property, an object with `additionalProperties: false`,
   `required: [source, liveness-check, classes]`, where `source` and `liveness-check` are strings
   and `classes` is an array of strings with `minItems: 1`. Drop `required: [artifact]` from the
   input item. The lint (step 6) now enforces that an item names exactly one of `artifact` or
   `signal`. `liveness-check` states, in words a verifier can check, how a dead source is
   detected. `classes` names the signal classes (step 3) this input admits.
2. **Decision options.** Add three optional node properties:
   - `options`: array of strings, `minItems: 2`. The outcomes a decision node may close with.
   - `default`: string. The recommended next step; it MUST be one of the node's `options` or
     `containment` entries.
   - `containment`: array whose `items` are `enum: [quarantine, rollback, secondary-auth, stop, none-needed]`,
     `minItems: 5`. The closed containment taxonomy, written as a block list.
   `none-needed` is recorded only with the signal reading that justifies it; say so in the spec
   text (step 7).
3. **Signal classes.** Add an optional top-level `signal-classes` array. Each item is an object
   with `additionalProperties: false`, `required: [id, limits]`, and these properties:
   - `id` (string); `exit` (string, a node id: the class's single exit, for a class with no culprit record);
   - `admission`: `{triggers: array of enum [schedule, push], push-refs: array of string}`,
     `required: [triggers]`. Keep the enum: an unknown trigger is a shape violation as well as
     a lint finding;
   - `culprit-record`: `{fields: array of string, default-class: enum [mechanical, judgement], withheld: {class: enum [mechanical, judgement], file: any}}`,
     `required: [fields, default-class, withheld]` (leave `file` untyped if the validator has no
     `null` type; the lint checks it);
   - `mechanical-exit`, `judgement-exit` (strings, node ids); `remedies`, `refused-paths` (arrays of string);
   - `limits`: `{dedupe: const per-culprit, open-ceiling: integer minimum 1, rate: {count: integer minimum 1, window: string}, on-exceed: const escalate}`,
     all four required.
4. **Copy the schema** byte-for-byte to `statusgen/schemas/workflow-pattern-v1.json`
   (`TestPatternsEmbeddedSchemaMatchesCommitted`).
5. **Parse.** Extend `parsePatternDoc` and `patternNode` / `patternDoc` to carry the signal
   input, `options`, `default`, `containment`, and the parsed signal classes. Keep `Inputs` as
   artifact names only, so every existing rule and `statusgen/instance.go` see exactly what they
   see today.
6. **Lint: eight MUST rules** in `statusgen/patterns.go`, each a `rulePattern…` constant, each
   reported as `checked-failed: <path> [<rule>]: …` (exit 1), and each reported even when the
   schema also refuses the same node:
   - `pattern-signal-input`: an input item names exactly one of `artifact` or `signal`; a
     `signal` input sits only on an entry node (no edge points at it); its `source` and
     `liveness-check` are non-empty; every name in `classes` is a declared `signal-classes` id;
     a pattern with a signal input declares at least one class and a pattern declaring classes
     has exactly one signal input; every node id a class names (`exit`, `mechanical-exit`,
     `judgement-exit`) is a node; a class names `exit`, or both `mechanical-exit` and
     `judgement-exit`; and `mechanical-exit` and `culprit-record` imply each other and both imply
     `judgement-exit`, `admission`, `remedies` and `refused-paths` (a class that names a
     `mechanical-exit` carries the whole gate-red record, and a class that carries the culprit
     record names a mechanical exit).
   - `pattern-replay-not-mandatory`: in a pattern with a signal input, at least one node outputs
     the reserved artifact `replay-fixture`, and every such node carries a `kind: command` evidence
     entry with `mandatory: true`. `replay-fixture` is a named constant beside the rules.
   - `pattern-signal-admission-open`: `admission.triggers` is a subset of `schedule`, `push`;
     when `push` is present, `push-refs` is exactly `[main]`; `push` with no `push-refs` is a
     finding. Every class declares `admission` unless its `exit` is a `kind: decision`,
     `role: human` node with no `effects`: a class whose exit (or `mechanical-exit`) is an
     `effect` node and that declares no `admission` is a finding, so an automated effect is never
     reachable from an unscreened run. A run that fails admission is refused wholesale, so the
     class has no lower-confidence path for it; state that in the spec.
   - `pattern-remedy-set-open`: `remedies` is a non-empty subset of the closed constant
     `gofmt`, `stray-file-deletion`, `exact-stale-string-replace`; `refused-paths` contains every
     entry of the gate-configuration floor constant `.github/workflows/`; and the `mechanical-exit`
     node is `kind: effect`. Both constants are named beside the rules, and the spec text names
     them.
   - `pattern-judgement-no-default`: the `judgement-exit` node is `kind: decision`, `role: human`,
     declares `options`, and declares a `default` that is one of them.
   - `pattern-culprit-record`: `fields` is exactly the set `check`, `file`, `line`, `problem`,
     `class`, `origin`; `default-class` is `judgement`; `withheld.class` is `judgement` and
     `withheld.file` is YAML null.
   - `pattern-decision-options`: `options`, `default` and `containment` appear only on
     `kind: decision` nodes; `default` is one of the node's `options` or `containment` entries;
     `options` has no duplicate.
   - `pattern-containment-set`: a node's `containment` is exactly the five values, each once; the
     node is `kind: decision`, `role: human`, with no `effects`; and a pattern with a signal input
     has exactly one such node.
   Register all eight in `statusgen/enforcementstatus.go` as `StatusFatal`, then regenerate the
   enforcement tables with `statusgen enforcement-status` and `make skillslint` (never by hand).
7. **Spec text** in `spec/workflow-pattern-v1.md`: add `signal-classes` to the §3 table
   (OPTIONAL); extend the §4.1 `inputs` row with the `signal` shape and add rows for `options`,
   `default` and `containment`; add a §4.3 "Signal-instantiated patterns" stating each of the
   eight rules in MUST language, the two closed constants and the reserved `replay-fixture`
   artifact, and that a signal pattern's liveness check is a precondition the eligibility
   evaluator holds on (`could-not-check` holds the instance; delivering the checks is out of
   scope); state in §7 that the table is unchanged and that containment is a `human` decision,
   so no role gains an effect kind.
8. **Fixtures and tests.** Add `good-signal-example.yaml`, a neutral minimal signal pattern that
   is NOT graph-execution/08's pattern, shaped like this (claims may be reworded):

   ```yaml
   schema: workflow-pattern-v1
   pattern: signal-example
   version: 1
   risk-input: {low: [review], standard: [review], elevated: [review], human: [review]}
   signal-classes:
     - id: example-alert
       exit: judge
       limits: {dedupe: per-culprit, open-ceiling: 3, rate: {count: 2, window: PT1H}, on-exceed: escalate}
     - id: example-gate-red
       admission: {triggers: [schedule, push], push-refs: [main]}
       culprit-record:
         fields: [check, file, line, problem, class, origin]
         default-class: judgement
         withheld: {class: judgement, file: null}
       mechanical-exit: propose
       remedies: [gofmt, stray-file-deletion, exact-stale-string-replace]
       refused-paths: [.github/workflows/]
       judgement-exit: judge
       limits: {dedupe: per-culprit, open-ceiling: 3, rate: {count: 2, window: PT1H}, on-exceed: escalate}
   nodes:
     - id: intake
       kind: check
       role: worker
       inputs:
         - signal: {source: "example-org/example-app#ci-main", liveness-check: "the source reported within the last hour", classes: [example-alert, example-gate-red]}
       outputs: [signal-record]
       evidence:
         - {kind: command, claim: "the signal, its source and every open item it matches are recorded", mandatory: true}
     - id: contain
       kind: decision
       role: human
       inputs: [{artifact: signal-record}]
       outputs: [containment-record]
       containment:
         - quarantine
         - rollback
         - secondary-auth
         - stop
         - none-needed
     - id: root-cause
       kind: artifact
       role: worker
       inputs: [{artifact: containment-record}]
       outputs: [replay-fixture]
       evidence:
         - {kind: command, claim: "the replay fixture reproduces the failure on the pre-fix revision", mandatory: true}
     - id: propose
       kind: effect
       role: worker
       inputs: [{artifact: replay-fixture}]
       outputs: [branch, draft-pr]
       effects: [{kind: push, target: branch}, {kind: pr-open, target: draft-pr}]
     - id: review
       kind: check
       role: reviewer
       inputs: [{artifact: draft-pr}]
       outputs: [approval]
       evidence:
         - {kind: review, claim: "a non-author reviewer approved the draft PR at its head", mandatory: true}
     - id: judge
       kind: decision
       role: human
       inputs: [{artifact: containment-record}]
       outputs: [disposition]
       options: [open-issue, wait, drop]
       default: open-issue
   edges:
     - {from: intake, to: contain}
     - {from: contain, to: root-cause}
     - {from: root-cause, to: propose}
     - {from: propose, to: review}
     - {from: contain, to: judge}
   join: review
   ```

   Each bad fixture is that file with ONE change: `bad-signal-input-not-entry` (the signal input
   moved to `root-cause`); `bad-replay-not-mandatory` (`mandatory: false` on the replay evidence);
   `bad-admission-pull-request` (`pull_request` added to `triggers`); `bad-remedy-open`
   (`workflow-file-edit` added to `remedies`); `bad-refused-paths-dropped` (`refused-paths: []`);
   `bad-judgement-no-default` (`default` removed from `judge`); `bad-withheld-mechanical`
   (`withheld: {class: mechanical, file: null}`); `bad-containment-open` (`guard-config-edit`
   added to `containment`); `bad-options-on-effect` (`options: [a, b]` added to `propose`);
   `bad-exit-effect-no-admission` (`example-alert`'s `exit` changed from `judge` to `propose`,
   leaving the class with no `admission`); `bad-mechanical-no-culprit` (`culprit-record` removed
   from `example-gate-red`, which still names `mechanical-exit`).
   Tests, each asserting its fixture reports the named rule tag and that the good fixture does not:
   `TestPatternsSignalInput` (planned) (including `bad-mechanical-no-culprit`),
   `TestPatternsReplayMandatory` (planned), `TestPatternsSignalAdmission` (planned) (including
   `bad-exit-effect-no-admission`),
   `TestPatternsRemedySetClosed` (planned) (both remedy fixtures), `TestPatternsJudgementDefault` (planned),
   `TestPatternsCulpritRecord` (planned), `TestPatternsDecisionOptions` (planned), `TestPatternsContainmentSet` (planned); add
   `good-signal-example.yaml` to `TestPatternsGoodFixturesLintClean`.
9. **Changelog.** Add `changelog/graph-execution-34-signal-classes.md` (planned) with `### Added` and one `- ` bullet.

## Verify

Rows run from the repository root on the implementation head, in a POSIX shell. Exit 3 means
could-not-check (setup failed); exit 4 means the expected finding was missing. Neither is a pass.
Fail-first: on the base each bad fixture reports only `[pattern-schema-violation]` (or nothing),
so rows 3-11 exit 4 there.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/good-signal-example.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r")` | exit 0; output contains "1 checked-clean, 0 checked-failed" |
| 2 | check:ci +neighbour | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" patterns --lint --root .)` | exit 0; output contains "0 checked-failed, 0 could-not-check" (the two shipped patterns still lint clean) |
| 3 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-signal-input-not-entry.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-signal-input]' "$T/out" \|\| exit 4; exit $e)` | exit 1 |
| 4 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-replay-not-mandatory.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-replay-not-mandatory]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (the rule graph-execution/08 row 3 relies on) |
| 5 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-admission-pull-request.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-signal-admission-open]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (08 row 9) |
| 6 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-remedy-open.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-remedy-set-open]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (08 row 10, remedy outside the closed set) |
| 7 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-refused-paths-dropped.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-remedy-set-open]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (08 row 10, gate-configuration floor dropped) |
| 8 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-judgement-no-default.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-judgement-no-default]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (08 row 11) |
| 9 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-withheld-mechanical.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-culprit-record]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (08 row 12) |
| 10 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-containment-open.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-containment-set]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (lint layer of the containment boundary) |
| 11 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-options-on-effect.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-decision-options]' "$T/out" \|\| exit 4; exit $e)` | exit 1 |
| 12 | check +mutation | `python3 -c 'import importlib.util as u,sys,os; sys.excepthook=lambda *a: os._exit(3); d="statusgen/testdata/patterns/"; sys.exit(3) if not (u.find_spec("yaml") and u.find_spec("jsonschema") and os.path.isfile(d+"good-signal-example.yaml") and os.path.isfile(d+"bad-containment-open.yaml")) else None; import json,yaml,jsonschema; v=jsonschema.Draft202012Validator(json.load(open("schemas/workflow-pattern-v1.json"))); sys.exit(6) if list(v.iter_errors(yaml.safe_load(open(d+"good-signal-example.yaml")))) else None; e=list(v.iter_errors(yaml.safe_load(open(d+"bad-containment-open.yaml")))); sys.exit(4) if not e else None; sys.exit(1 if any("containment" in list(x.absolute_path) and x.validator=="enum" for x in e) else 7)'` | exit 1: the positive control validates and the bad fixture is refused by the schema alone, on the `containment` items enum, with the lint bypassed. 3 = could-not-check (`yaml`/`jsonschema` absent or a fixture missing); 4 = fixture accepted; 6 = positive control refused; 7 = refused for another reason |
| 13 | check:ci | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go test -count=1 -v -run '^TestPatterns' .) > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; for t in TestPatternsSignalInput TestPatternsReplayMandatory TestPatternsSignalAdmission TestPatternsRemedySetClosed TestPatternsJudgementDefault TestPatternsCulpritRecord TestPatternsDecisionOptions TestPatternsContainmentSet TestPatternsGoodFixturesLintClean TestPatternsRoleEffectPermissionsCoversTopologyRoles TestPatternsEmbeddedSchemaMatchesCommitted; do grep -Fq -- "--- PASS: $t (" "$T/out" \|\| exit 4; done)` | exit 0 (exit 4 = a named test did not run and pass; the role-table test passing shows §7 is unchanged) |
| 14 | check:ci +flow | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" enforcement-status > "$T/out" \|\| exit 3; for r in pattern-signal-input pattern-replay-not-mandatory pattern-signal-admission-open pattern-remedy-set-open pattern-judgement-no-default pattern-culprit-record pattern-decision-options pattern-containment-set; do grep -Fq -- "$r" "$T/out" \|\| exit 4; grep -Fq -- "$r" plugins/assay/skills/author-brief/SKILL.md \|\| exit 5; done; make skillslint)` | exit 0 (every rule is registered, present in the generated table, and the table is in sync) |
| 15 | check | `statusgen --root . --consumers --brief assay:assay:graph-execution:34 --base "$(git merge-base HEAD refs/remotes/origin/main)"` | pre-merge only, on the implementing branch with its merge-base pinned: exit 0, and every `follow-up graph-execution/34` entry the implementation fixes has flipped to `fixed-here` and is corroborated by the diff |
| 16 | check | `statusgen --root . --lint` | exit 0; output contains "LINT: PASS" |
| 17 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-exit-effect-no-admission.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-signal-admission-open]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (a class whose exit is an effect node with no admission is refused: the lower layer, not the happy path) |
| 18 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-mechanical-no-culprit.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-signal-input]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (a class naming a mechanical exit without the culprit record is refused) |

## Pre-mortem

| Failure mode of the work | Caught by |
|---|---|
| A new rule is registered but its check never fires | rows 3-11, each feeding one fixture and asserting the tag |
| A rule fires on the good fixture too, so every signal pattern is refused | row 1 and `TestPatternsGoodFixturesLintClean` (row 13) |
| The containment set is only a lint rule, so a loosened lint lets `guard-config-edit` through | row 12 (schema alone refuses it) |
| The schema is loosened (dropping `required: [artifact]`) and an input with neither key slips by | row 3's rule covers "exactly one of"; `TestPatternsSignalInput` should include that case |
| The amendment widens §7 or the role enum to make a fixture pass | row 13 (`TestPatternsRoleEffectPermissionsCoversTopologyRoles`), and review of the diff to `patternRoleEffectPermissions` |
| The shipped patterns or 33's re-pinned instance fixtures break | row 2; instance fixtures are untouched by this brief (review-only: the diff must not touch `statusgen/testdata/instances/`) |
| The schema twin drifts | row 13 (`TestPatternsEmbeddedSchemaMatchesCommitted`) |
| An effect-exit class ships with no admission, or a mechanical exit with no culprit record | rows 17 and 18 |
| The generated enforcement table is hand-edited or left stale | row 14 (`make skillslint`) |

## Evidence

<!-- appended at implementation time, one row per Verify item, by someone who did not implement -->

## Review

Gate: model (from frontmatter). The reviewer answers:
(1) What single control stands between a signal pattern and a containment action no person
took? A `human` decision node carries no effects (33's schema clause and its lint rule), and the
containment list is closed in the schema and checked again by the lint. Is that acceptable while
no runtime yet reads these keys?
(2) Does a row prove a lower layer catches the fault with the upper one bypassed? Row 12 feeds
the out-of-set containment value to the schema alone; row 10 shows the lint reports it whatever
the schema says.
