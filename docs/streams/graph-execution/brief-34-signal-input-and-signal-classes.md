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
unblocks: ["graph-execution/08", "graph-execution/35"]
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
  - "re-checked 2026-11-01 @ 891f6865d (origin/main): 33's code is on main (b4db4b412). The role enum now includes `human`, and `statusgen/conform.go`'s `schemaKeywords` now also implements `maxItems`, `if`, `then`, `else` and `not`; `oneOf` is still absent. The input item, top-level keys and the zero-hit grep are unchanged. `grep -n 'pattern\\|signal\\|admission' statusgen/eligibility.go` returns zero hits: no runtime reads a pattern, a signal class or an admission predicate"
exec-tier: strong
exec-tier-why: "(a) the exact key shapes and rule boundaries are design choices the facts bound but do not fully fix; (b) one change spans two schema copies, the lint, the spec text, the enforcement registry and its generated copies; (c) a rule that never fires passes every happy-path test"
domain: complicated
outcome: none
consumers:
  - "schemas/workflow-pattern-v1.json, statusgen/schemas/workflow-pattern-v1.json (byte-identical twin): fixed-here"
  - "statusgen/patterns.go, statusgen/patterns_test.go, statusgen/testdata/patterns/: fixed-here"
  - "spec/workflow-pattern-v1.md: fixed-here"
  - "statusgen/enforcementstatus.go: fixed-here"
  - "the generated enforcement tables in plugins/assay/skills/author-brief/SKILL.md and .cursor/skills/author-brief/SKILL.md: follow-up graph-execution/34 (regenerated from the registry, never hand-edited; this change does not edit the skill bundle, so the regeneration lands separately)"
  - "spec/workflow-patterns/signal-triggered-v1.yaml (planned): follow-up graph-execution/08 (the first shipped pattern that uses these keys)"
  - "spec/workflow-patterns/implementation-v1.yaml, spec/workflow-patterns/research-v1.yaml: out-of-scope (unchanged; they use none of the new keys and Verify row 2 proves they still lint clean)"
  - "statusgen/instance.go, schemas/workflow-instance-v1.json: out-of-scope (no instance of a signal-instantiated pattern is produced until a brief activates one; graph-execution/35 produces none; instances of the two shipped patterns use no new key)"
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
`statusgen/testdata/patterns/bad-judgement-class-no-admission.yaml` (planned),
`statusgen/testdata/patterns/bad-class-no-floor.yaml` (planned),
`statusgen/testdata/patterns/bad-class-unlisted.yaml` (planned),
`statusgen/testdata/patterns/bad-artifact-effect-no-admission.yaml` (planned),
`statusgen/testdata/patterns/bad-artifact-effect-no-floor.yaml` (planned),
`statusgen/testdata/patterns/bad-disconnected-effect.yaml` (planned),
`statusgen/testdata/patterns/bad-exit-unreachable.yaml` (planned),
`statusgen/testdata/patterns/bad-containment-default.yaml` (planned),
`plugins/assay/skills/author-brief/SKILL.md` (generated enforcement table only),
`.cursor/skills/author-brief/SKILL.md` (generated enforcement table only),
`changelog/graph-execution-34-signal-classes.md` (planned).

single-point-of-failure: the ONE control for the containment boundary is that a containment
step is a `human` decision node, which carries no effects. Two layers stand behind it, each in a
different component on a different signal: (1) the pattern schema refuses any value outside the
closed containment set, by an items enum that needs no lint (checked by an independent JSON Schema
validator in row 12); (2) the pattern lint refuses a containment list that is not exactly the set,
or sits on a node that is not an effect-free `human` decision (`pattern-containment-set`, plus
33's `pattern-human-role-not-decision`), and it reports even when the schema also refuses. The
out-of-band layer is unchanged from brief 02: the roster's role binding, not the pattern,
authorises a forge write, and `human` binds no identity at all.
For provenance (which runs may start an instance that reaches an automated effect), the ONE
control is `pattern-signal-admission-open`: once any node in a signal pattern declares
`effects`, whatever its `kind` and whether or not an edge reaches it, every declared class
declares `admission`, with no exemption by exit, by route or by node kind, and no reliance on a
runtime honouring class routes. The `.github/` floor is bound the same way
(`pattern-remedy-set-open`). Behind it, the schema's `triggers` enum refuses an unknown trigger
value without the lint. **No runtime layer stands behind it today.** Nothing yet reads a signal
pattern at run time: graph-execution/01 evaluates brief `gates:`, `depends:` and feathers and
reads no pattern, signal class or admission predicate. The only planned runtime check is
gate-red's admission predicate in auto-triage/01 (`todo`), which covers that one class. graph-execution/35
(`todo`) owns admission refusal for every class, class-route enforcement and the liveness hold;
none of that exists until it lands. Until then, a signal pattern is a reviewed definition only and MUST NOT be
activated (step 7's §4.3 text; graph-execution/08's "Activation is not in this brief").

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
  that uses any other keyword. 33 has since extended that set with `maxItems`, `if`, `then`,
  `else` and `not` (re-check in `sources:`); `oneOf` is still absent. This brief never adds a
  keyword without adding it to the validator and that test in the same change. Every rule below
  that needs exactly-one-of or cross-field logic over the resolved graph is a lint rule, not a
  schema rule, so each rule has one home.
- **Effects are not limited to `kind: effect` nodes.** `spec/workflow-pattern-v1.md` §4.2 rule 2
  (`pattern-effect-target-not-owned` in `statusgen/patterns.go`) lets a node of any kind declare
  effects whose targets are among its own `outputs`. The shipped
  `spec/workflow-patterns/implementation-v1.yaml` does exactly that: `implement` is
  `kind: artifact`, `role: worker`, with `push` and `pr-open` effects. No shipped rule requires a
  node to be reachable along `edges`. Every rule below that keys on effects therefore keys on a
  node's `effects` list, never on its `kind` or on reachability.
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
run at admission, running only a class's route, enforcing limits) is not built here. graph-execution/01 is `done` and reads no pattern. graph-execution/35 (`todo`)
builds admission refusal, route enforcement and the liveness hold for a signal pattern in general;
limits stay with the activating brief. auto-triage/01-03 (`todo`) build gate-red's admission check and its two live exits,
for that class only. The lint rules below are written so that none of them depends on that
missing layer, and step 7's spec text forbids activating a signal pattern before a runtime
enforces admission, routes and liveness.

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
   - `default`: string. The recommended next step; it MUST be one of the node's `options`
     entries. It is a recommendation shown to the person who closes the decision and nothing
     more: it never closes a decision on a timeout, when no one answers, or by any act other
     than the person's own (33's authenticated-act rule stays the control). The containment
     node carries no `default` (step 6, `pattern-containment-set`).
   - `containment`: array whose `items` are `enum: [quarantine, rollback, secondary-auth, stop, none-needed]`,
     `minItems: 5`. The closed containment taxonomy, written as a block list.
   `none-needed` is recorded only with the signal reading that justifies it; say so in the spec
   text (step 7).
3. **Signal classes.** Add an optional top-level `signal-classes` array. Each item is an object
   with `additionalProperties: false`, `required: [id, limits]`, and these properties:
   - `id` (string); `exit` (string, a node id: the class's single exit, for a class with no culprit record);
   - `admission`: `{triggers: array of enum [schedule, push], push-refs: array of string}`,
     `required: [triggers]`. Keep the enum: an unknown trigger is a shape violation as well as
     a lint finding. A `push-refs` entry is a fully qualified ref (`refs/heads/main`), never a
     bare branch name, so a tag named `main` can never satisfy it;
   - `culprit-record`: `{fields: array of string, default-class: enum [mechanical, judgement], withheld: {class: enum [mechanical, judgement], file: {type: null}}}`,
     `required: [fields, default-class, withheld]` (`statusgen/conform.go` implements the `null`
     type, so the schema refuses a non-null `file` without the lint; the lint checks it too);
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
   schema also refuses the same node.

   **A class's route** (used by the rules below and stated in the spec, step 7): the route of
   class C is every node that lies on a directed path along `edges` from the signal-input node to
   one of C's exit nodes (`exit`, `mechanical-exit`, `judgement-exit`), plus every node reachable
   along `edges` from one of those exits. An instance of class C runs only the nodes on C's
   route; a node off the route is never run for that instance. No runtime enforces that today
   (graph-execution/01 reads no pattern; see the single-point-of-failure note), so this brief
   defines the route and lints it, and nothing more. No provenance rule below relies on route
   enforcement, on node kind or on reachability. An **effect-bearing node** is any node whose
   `effects` list is non-empty, whatever its `kind` (spec §4.2 rule 2 lets an `artifact`,
   `check` or `decision` node carry effects on its own outputs). Admission and the `.github/`
   floor key on the presence of an effect-bearing node anywhere in the pattern, so they hold even
   if a runtime ran an off-route node, a node no edge reaches, or a non-`effect` node with
   effects.

   - `pattern-signal-input`: an input item names exactly one of `artifact` or `signal`; a
     `signal` input sits only on an entry node (no edge points at it); its `source` and
     `liveness-check` are non-empty; the names in `classes` are exactly the declared
     `signal-classes` ids, each once (a declared class the input does not list is a finding, as
     is a listed name no class declares), so the fail-closed rules below cover every declared
     class; a pattern with a signal input declares at least one class and a pattern declaring
     classes has exactly one signal input; in a pattern with a signal input, every node is
     reachable along `edges` from the signal-input node (no disconnected node, effect-bearing or
     not); every node id a class names (`exit`, `mechanical-exit`, `judgement-exit`) is a node
     (so no class has an empty route); a class names `exit`, or both `mechanical-exit` and
     `judgement-exit`; and `mechanical-exit` and `culprit-record` imply each other and both imply
     `judgement-exit`, `admission`, `remedies` and `refused-paths` (a class that names a
     `mechanical-exit` carries the whole gate-red record, and a class that carries the culprit
     record names a mechanical exit).
   - `pattern-replay-not-mandatory`: in a pattern with a signal input, at least one node outputs
     the reserved artifact `replay-fixture`, and every such node carries a `kind: command` evidence
     entry with `mandatory: true`. `replay-fixture` is a named constant beside the rules.
   - `pattern-signal-admission-open`: `admission.triggers` is a subset of `schedule`, `push`;
     when `push` is present, `push-refs` is exactly `[refs/heads/main]`; `push` with no
     `push-refs` is a finding. **Fail closed on presence:** if any node in the pattern is
     effect-bearing (non-empty `effects`, whatever its `kind`, whether or not an edge reaches
     it), every declared class declares `admission`, whatever its exit and whatever its route.
     There is no exemption for a class whose exit is a human decision: edges carry no condition,
     and a person closing a containment or judgement decision is not a screen on where the signal
     came from. A class may omit `admission` only in a signal pattern in which no node declares
     any effect. A run that fails admission is refused wholesale, so the class has no
     lower-confidence path for it; state that in the spec.
   - `pattern-remedy-set-open`: on a class that names `mechanical-exit`, `remedies` is a
     non-empty subset of the closed constant `gofmt`, `stray-file-deletion`,
     `exact-stale-string-replace`, and the `mechanical-exit` node is `kind: effect`; on any
     class, a `remedies` entry outside that constant is a finding. **Fail closed on presence:**
     if any node in the pattern is effect-bearing, every declared class declares
     `refused-paths` containing every entry of the gate-configuration floor constant `.github/`
     (workflows, composite actions and CODEOWNERS alike), whatever its exit, route or node
     kinds. The floor binds by presence, not by `mechanical-exit` or by route, so a class that
     reaches an effect by its plain `exit`, through a non-`effect` node, or not at all carries it
     too. Both constants are named beside the rules, and the spec text names them and states
     that the floor is a subset of "gate configuration": a runtime allow-list (auto-triage/02)
     may refuse more, never less.
   - `pattern-judgement-no-default`: the `judgement-exit` node is `kind: decision`, `role: human`,
     declares `options`, and declares a `default` that is one of them.
   - `pattern-culprit-record`: `fields` is exactly the set `check`, `file`, `line`, `problem`,
     `class`, `origin`; `default-class` is `judgement`; `withheld.class` is `judgement` and
     `withheld.file` is YAML null.
   - `pattern-decision-options`: `options`, `default` and `containment` appear only on
     `kind: decision` nodes; `default` is one of the node's `options` entries; `options` has no
     duplicate.
   - `pattern-containment-set`: a node's `containment` is exactly the five values, each once; the
     node is `kind: decision`, `role: human`, with no `effects` and no `default` (no outcome of
     the containment step is pre-selected, `none-needed` included); and a pattern with a signal
     input has exactly one such node.
   Register all eight in `statusgen/enforcementstatus.go` as `StatusFatal`, then regenerate the
   enforcement tables with `statusgen enforcement-status` and `make skillslint` (never by hand).
7. **Spec text** in `spec/workflow-pattern-v1.md`: add `signal-classes` to the §3 table
   (OPTIONAL); extend the §4.1 `inputs` row with the `signal` shape and add rows for `options`,
   `default` and `containment`. The `options` row says how it differs from the existing
   `outcomes` key: `outcomes` names a node's non-success dispositions (`wait`, `fail`), while
   `options` lists the choices a person closes a decision with; they are not synonyms. The
   `default` row says `default` is a recommendation only and never closes a decision on a
   timeout, when no one answers, or by any act but the person's own. Add a §4.3
   "Signal-instantiated patterns" stating: the meaning of `exit`, `mechanical-exit` and
   `judgement-exit`, and a class's route as step 6 defines it (an instance of a class runs only
   the nodes on its route); what selects between a class's two exits (the class recorded on the
   culprit record at run time: `mechanical` goes to `mechanical-exit`, anything else, unresolved
   included, goes to `judgement-exit`, per the conservative default); each of the eight rules in
   MUST language; "effect-bearing node" as step 6 defines it (any kind); that admission and the
   `.github/` floor are checked fail-closed on the presence of any effect-bearing node anywhere in
   the pattern, not over a route, a node kind or reachability, so neither depends on route
   enforcement; the two closed constants (the remedy set, and the gate-configuration floor
   `.github/`, stated as a floor that is a subset of gate configuration); `push-refs` as fully
   qualified refs; the reserved `replay-fixture` artifact; and that a signal pattern's liveness
   check is a precondition a runtime MUST hold on (`could-not-check` holds the instance). State
   plainly that no runtime in this version reads a signal pattern, refuses admission, enforces a
   route or holds on liveness, and that a signal pattern MUST NOT be activated (instantiated from
   a live signal) until a dispatch brief that enforces all three has landed; the lint is the only
   layer until then. State in §7 that the table is unchanged and
   that containment is a `human` decision, so no role gains an effect kind.
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
       admission: {triggers: [schedule]}
       refused-paths: [.github/]
       limits: {dedupe: per-culprit, open-ceiling: 3, rate: {count: 2, window: PT1H}, on-exceed: escalate}
     - id: example-gate-red
       admission: {triggers: [schedule, push], push-refs: [refs/heads/main]}
       culprit-record:
         fields: [check, file, line, problem, class, origin]
         default-class: judgement
         withheld: {class: judgement, file: null}
       mechanical-exit: propose
       remedies: [gofmt, stray-file-deletion, exact-stale-string-replace]
       refused-paths: [.github/]
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

   Each bad fixture is that file with exactly the edits listed for it and no others (most are a
   single edit): `bad-signal-input-not-entry` (the signal input moved to `root-cause`);
   `bad-replay-not-mandatory` (`mandatory: false` on the replay evidence);
   `bad-admission-pull-request` (`pull_request` added to `triggers`); `bad-remedy-open`
   (`workflow-file-edit` added to `remedies`); `bad-refused-paths-dropped` (`example-gate-red`'s
   `refused-paths: []`); `bad-judgement-no-default` (`default` removed from `judge`);
   `bad-withheld-mechanical` (`withheld: {class: mechanical, file: null}`); `bad-containment-open`
   (`guard-config-edit` added to `containment`); `bad-options-on-effect` (`options: [a, b]` added
   to `propose`); `bad-exit-effect-no-admission` (two edits: `example-alert`'s `exit` changed
   from `judge` to `propose`, and its `admission` removed); `bad-mechanical-no-culprit`
   (`culprit-record` removed from `example-gate-red`, which still names `mechanical-exit`);
   `bad-judgement-class-no-admission` (one edit: `admission` removed from `example-alert`, whose
   exit stays the human decision `judge` and whose route holds no effect-bearing node, so a rule
   keyed on the class's exit or route would pass it); `bad-class-no-floor` (one edit: `refused-paths` removed from
   `example-alert`, whose exit stays `judge`); `bad-exit-unreachable` (the edge `contain → judge`
   removed, so `judge` is not reachable from the signal input); `bad-containment-default`
   (`default: none-needed` added to `contain`); `bad-class-unlisted` (one edit: `example-alert`
   removed from `intake`'s `classes`, so a declared class is not listed by the input).
   Three fixtures put an effect where a rule keyed on node kind or reachability would miss it:
   `bad-artifact-effect-no-admission` (`example-gate-red` removed, from `signal-classes` and from
   `intake`'s `classes`; `propose` changed to `kind: artifact`, keeping its `push` and `pr-open`
   effects, which §4.2 rule 2 allows on its own outputs; and `admission` removed from
   `example-alert`, so no node is `kind: effect` but one is effect-bearing);
   `bad-artifact-effect-no-floor` (the same first two edits, then `refused-paths` removed from
   `example-alert`); `bad-disconnected-effect` (the edge `root-cause → propose` removed, so
   neither `propose` nor `review` is reachable from the signal input, and `admission` and `refused-paths` both
   removed from `example-alert`).
   Tests, each asserting its fixture reports the named rule tag and that the good fixture does not:
   `TestPatternsSignalInput` (planned) (including `bad-mechanical-no-culprit`,
   `bad-exit-unreachable`, `bad-class-unlisted` and `bad-disconnected-effect`),
   `TestPatternsReplayMandatory` (planned),
   `TestPatternsSignalAdmission` (planned) (including `bad-exit-effect-no-admission`,
   `bad-judgement-class-no-admission`, `bad-artifact-effect-no-admission` and
   `bad-disconnected-effect`),
   `TestPatternsRemedySetClosed` (planned) (`bad-remedy-open`, `bad-refused-paths-dropped`,
   `bad-class-no-floor`, `bad-artifact-effect-no-floor` and `bad-disconnected-effect`),
   `TestPatternsJudgementDefault` (planned),
   `TestPatternsCulpritRecord` (planned), `TestPatternsDecisionOptions` (planned),
   `TestPatternsContainmentSet` (planned) (including `bad-containment-default`); add
   `good-signal-example.yaml` to `TestPatternsGoodFixturesLintClean`.
9. **Changelog.** Add `changelog/graph-execution-34-signal-classes.md` (planned) with `### Added` and one `- ` bullet.

## Verify

Rows run from the repository root on the implementation head, in a POSIX shell. Exit 3 means
could-not-check (setup failed); exit 4 means the expected finding was missing. Neither is a pass.
Fail-first: on the base each bad fixture reports only `[pattern-schema-violation]` (or nothing),
so rows 3-11 and 17-24 exit 4 there.

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
| 17 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-exit-effect-no-admission.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-signal-admission-open]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (a class whose exit is an effect node with no admission is refused) |
| 18 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-mechanical-no-culprit.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-signal-input]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (a class naming a mechanical exit without the culprit record is refused) |
| 19 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-judgement-class-no-admission.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-signal-admission-open]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (a class with no admission whose exit is an effect-free human decision, and whose route holds no effect-bearing node, is still refused because another node declares effects: the rule keys on presence, not on the exit or the route) |
| 20 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-class-no-floor.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-remedy-set-open]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (a class whose exit is a human decision still carries the gate-configuration floor once any node declares effects) |
| 21 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-artifact-effect-no-admission.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-signal-admission-open]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (an effect on a `kind: artifact` node, with no `kind: effect` node in the pattern, still requires admission on every class: the rule keys on a node's `effects`, not its kind) |
| 22 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-artifact-effect-no-floor.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-remedy-set-open]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (the same effect on a `kind: artifact` node still binds the gate-configuration floor on every class) |
| 23 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-disconnected-effect.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-signal-input]' "$T/out" \|\| exit 4; grep -Fq '[pattern-signal-admission-open]' "$T/out" \|\| exit 4; grep -Fq '[pattern-remedy-set-open]' "$T/out" \|\| exit 4; exit $e)` | exit 1, with all three tags: a node no edge reaches is refused, and an effect-bearing node off every path from the signal input still requires admission and the floor on every class (neither rule keys on reachability) |
| 24 | check:ci +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-class-unlisted.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-signal-input]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (a declared class the signal input does not list is refused, so no class escapes the rules that range over the input's classes) |

## Pre-mortem

| Failure mode of the work | Caught by |
|---|---|
| A new rule is registered but its check never fires | rows 3-11, each feeding one fixture and asserting the tag |
| A rule fires on the good fixture too, so every signal pattern is refused | row 1 and `TestPatternsGoodFixturesLintClean` (row 13) |
| The containment set is only a lint rule, so a loosened lint lets `guard-config-edit` through | row 12 (schema alone refuses it) |
| The schema is loosened (dropping `required: [artifact]`) and an input with neither key slips by | row 3's rule covers "exactly one of"; `TestPatternsSignalInput` (planned) should include that case |
| The amendment widens §7 or the role enum to make a fixture pass | row 13 (`TestPatternsRoleEffectPermissionsCoversTopologyRoles`), and review of the diff to `patternRoleEffectPermissions` |
| The shipped patterns or 33's re-pinned instance fixtures break | row 2; instance fixtures are untouched by this brief (review-only: the diff must not touch `statusgen/testdata/instances/`) |
| The schema twin drifts | row 13 (`TestPatternsEmbeddedSchemaMatchesCommitted`) |
| A class with no admission sits in a pattern with an effect-bearing node, whether its exit is that node (row 17) or a human decision whose route holds no effect (row 19) | rows 17 and 19 |
| The admission or floor rule keys on `kind: effect`, so an effect on an `artifact`, `check` or `decision` node (allowed by §4.2 rule 2, and used by the shipped implementation pattern) escapes both | rows 21 and 22 |
| The admission or floor rule keys on reachability, so an effect-bearing node no edge reaches escapes both | row 23 (and `pattern-signal-input` refuses the disconnected node itself) |
| A declared class is left out of the signal input's `classes`, so a rule that ranges over the listed classes skips it | row 24 |
| A mechanical exit ships with no culprit record | row 18 |
| A class whose exit is not a mechanical exit escapes the gate-configuration floor | row 20 |
| A runtime runs a node off a class's route, admits a run that fails admission, or ignores a could-not-check liveness check | not caught here, and no runtime exists to catch it until graph-execution/35 lands (graph-execution/01 reads no pattern; auto-triage/01 covers gate-red admission only). The lint rules do not depend on it (rows 19-23 key on presence), and step 7's spec text forbids activating a signal pattern until a dispatch brief enforces all three |
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
the schema says. For provenance, rows 19 and 21-23 show the admission and floor rules refuse an
un-admitted or unfloored class whatever its exit, whatever kind of node carries the effect, and
whether or not an edge reaches it, so neither rule leans on the exit, the node kind,
reachability or route enforcement. No runtime layer stands behind the lint for provenance today;
is the stated activation hold (step 7) an acceptable substitute until a dispatch brief (graph-execution/35) lands?
