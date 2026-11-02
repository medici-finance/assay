---
brief: assay:assay:graph-execution:33
title: Merge as a human decision node, and a derived risk-input mapping
why: >-
  The shipped implementation pattern lets a desk agent produce the merge commit behind a
  `dispatch` effect, which contradicts the rule that merge is never an agent operation, and
  every check passes because nothing ties an output to the human floor. Its risk-input
  mapping is also asserted rather than derived. This amendment puts merge behind an
  authenticated human act and makes the mapping explainable before admission binds to it.
wave: 0
depends: []
unblocks: ["graph-execution/14", "graph-execution/34"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  The brief changes the human merge floor's encoding in the pattern schema, the pattern lint
  and the instance validator, and it fixes the risk-class to mandatory-gate mapping admission
  will bind to. The human confirms the derived mapping and that no agent role gains a merge
  capability.
decision-trigger: creation
decision-issue: 4511
ruling: https://github.com/medici-finance/assay/issues/4511#issuecomment-6560687132
design: DR-graph-exec-33
issues: [1933]
schema: brief-v2
version: 1
authored: 2026-10-26 by worker-desk (author-brief)
sources:
  - "per the driver's ruling: https://github.com/medici-finance/assay/issues/1933#issuecomment-6419943765"
  - medici-finance/assay#1933 — where the risk-input mapping comes from, and the review finding on the merge node
  - medici-finance/assay#4506 — gap 1 (a `human` owner on decision nodes) is settled here; widening the role-effect table stays unruled
  - docs/enforcement-model.md — "Human floors stand. merge, release and deploy are never an agent operation at any disposition"
  - spec/agentic-admission-v1.md — §2 disposition definitions, §4 step 10 human-floor refusal, §6 disposition-to-risk-class table
  - spec/workflow-instance-v1.md — §8 human decisions are external authenticated acts
  - docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md — the pattern, schema and lint this amends
  - docs/streams/graph-execution/brief-09-instance-contract.md — the instance validator and fixtures this amends
  - 'freshness-checked 2026-10-26 @ b1a7e4233; `merge` is `kind: effect, role: desk` with a `dispatch` effect, and the pattern lint and instance validator both pass it'
exec-tier: strong
exec-tier-why: >-
  Encodes a human floor across two schemas, two validators and fixture digests; a wrong rule
  either lets an agent merge or refuses every shipped instance.
domain: complicated
outcome: none
consumers:
  - "spec/workflow-patterns/implementation-v1.yaml, statusgen/testdata/patterns/good-implementation-v1.yaml, statusgen/testdata/instances/root/spec/workflow-patterns/implementation-v1.yaml: follow-up graph-execution/33"
  - "spec/workflow-pattern-v1.md, schemas/workflow-pattern-v1.json, statusgen/schemas/workflow-pattern-v1.json, statusgen/patterns.go, statusgen/patterns_test.go, statusgen/testdata/patterns: follow-up graph-execution/33"
  - "spec/workflow-instance-v1.md, schemas/workflow-instance-v1.json and its embedded copy, statusgen/instance.go, statusgen/instance_test.go, statusgen/testdata/instances: follow-up graph-execution/33"
  - "docs/lifecycle.md: follow-up graph-execution/33"
  - "tools/desk/internal/deskkit/admission.go (LoadPatternRiskInput, MandatoryGates, humanFloorOperations): follow-up graph-execution/14 (binds the re-pinned pattern at dispatch; no code change here)"
  - "spec/workflow-patterns/research-v1.yaml: out-of-scope (its risk-input and persisting-effect advisories are named below, not changed)"
  - "pattern role-effect table (spec/workflow-pattern-v1.md §7, patternRoleEffectPermissions): out-of-scope (not widened; widening stays unruled)"
---

# Brief 33 — Merge as a human decision node, and a derived risk-input mapping

## Context

files: `spec/workflow-patterns/implementation-v1.yaml`, `statusgen/testdata/patterns/good-implementation-v1.yaml`, `statusgen/testdata/instances/root/spec/workflow-patterns/implementation-v1.yaml`, `spec/workflow-pattern-v1.md`, `schemas/workflow-pattern-v1.json`, `statusgen/schemas/workflow-pattern-v1.json`, `statusgen/patterns.go`, `statusgen/patterns_test.go`, `statusgen/testdata/patterns/bad-merge-behind-dispatch.yaml` (planned), `statusgen/testdata/patterns/bad-human-role-not-decision.yaml` (planned), `statusgen/testdata/patterns/bad-floor-effect-target.yaml` (planned), `spec/workflow-instance-v1.md`, `schemas/workflow-instance-v1.json`, `statusgen/instance.go`, `statusgen/instance_test.go`, `statusgen/testdata/instances/*.yaml`, `statusgen/testdata/instances/human-node-actor.yaml` (planned), `statusgen/testdata/instances/merge-desk-actor.yaml` (planned), `docs/lifecycle.md`, `changelog/graph-execution-33-merge-decision-node.md` (planned) — and the embedded copy `TestInstanceSchemaParity` pins, `statusgen/instance.go`, `statusgen/instance_test.go`, `statusgen/testdata/instances/*.yaml` (every fixture that pins the pattern digest), `statusgen/testdata/instances/human-node-actor.yaml` (planned), `statusgen/testdata/instances/merge-desk-actor.yaml` (planned), `docs/lifecycle.md`, `changelog/graph-execution-33-merge-decision-node.md` (planned).

single-point-of-failure: no verified control confines the merge act to a human today — NONE until
graph-execution/14 binds admission's human-floor refusal at dispatch (/14 depends on this brief and <!-- graph: encoded graph-execution/33→graph-execution/14 -->
owns that layer; it is deferred, not infeasible). What is established about the forge: the `main`
ruleset requires a pull request, an approving review that is not from the last pusher, and the
required status checks (read 2026-10-26: its rule types are `deletion`, `non_fast_forward`,
`required_status_checks`, `pull_request`). None of those restricts which identity performs the merge,
and `.github/workflows/evidence-automerge.yml` already merges evidence PRs as a workflow identity,
kept off implementation PRs only by that workflow's own author guard. Whether any other forge setting
confines the merge act to a human is UNVERIFIED (could-not-check here). This brief adds no control on
the act itself. Its three layers guard the DECLARED shape only: (1) the pattern schema
refuses `role: human` on anything but an effect-free decision node, checked by an independent JSON
Schema validator; (2) the pattern lint refuses any node outputting or targeting a human-floor
artifact unless it is a `human` decision node, whatever its effect is labelled; (3) the instance
validator refuses an actor, a capability or an effect on a `human` node and refuses a desk-role
merge instance. None of the three can tell who merged: a decision record is
`{id, node, outcome, act: {kind, ref}}` (`instanceDecision` in `statusgen/instance.go`), the
validator only checks that a decision names a known node instance, and the instance schema fixes
`act.kind` to `external-authenticated-act` but leaves `act.ref` a free string. An agent that merged
and wrote a well-formed decision record would pass all three.

facts (b1a7e4233, 2026-10-26):
- Ruling, per the driver's ruling (https://github.com/medici-finance/assay/issues/1933#issuecomment-6419943765):
  merge becomes a `kind: decision` node closed by an external authenticated human act; `human`
  is allowed as an owner only on decision nodes; the pattern and its testdata twin are re-pinned;
  graph-execution/02 stays held until this brief lands. This brief reads "lands" as reaching
  `verified` (implementation merged and its Verify rows evidenced by a non-implementer); the
  hold note sits in brief 02's `## Review` section.
- Today `merge` is `kind: effect, role: desk`, outputs `[merge-commit]`, effect
  `{kind: dispatch, target: merge-commit}`. Every check passes: `dispatch` is in the desk row of the
  §7 role-effect table, and the vocabulary has no `merge` effect kind, so nothing ties the output
  to the human floor. `statusgen patterns --lint --root .` prints `patterns: 2 checked-clean,
  0 checked-failed, 0 could-not-check (2 file(s) scanned)` and exits 0.
- The instance validator reuses the same table (`statusgen/instance.go`, `perm :=
  patternRoleEffectPermissions[n.Role]`), and `statusgen/testdata/instances/valid.yaml` gives the
  merge node a desk actor holding `dispatch`. All 11 instance fixtures that bind the pattern
  (`capability-minted`, `cross-cell`, `duplicate-dependency`, `missing-pattern`,
  `reporter-trust-promoted`, `rewrite-same-id`, `silent-truncation`, `stale-acceptance`,
  `superseding`, `unknown-mandatory-field`, `valid`; `grep -rl` over `statusgen/testdata`) pin
  `digest: sha256:edcf96e1eed65dc4a7a09cd2ed0e55fd8c0dba018a0452b959b7e71ffe13af94`, version 1.
- `risk-input` today: `low: [review]`, `standard/elevated/human: [review, verify]` — named, not
  derived; `elevated` and `human` are indistinguishable from `standard`.
- Admission maps disposition to risk class (`spec/agentic-admission-v1.md` §6): `bounded` →
  `standard`, `supervised-agent` → `elevated`, `human-led`/`discovery-only`/`blocked` → `human`;
  `tools/desk/internal/deskkit/admission.go` loads `risk-input` through `LoadPatternRiskInput`,
  which refuses a gate id that is not a node. The admission layer is not yet bound at dispatch
  (that is graph-execution/14).
- `TestPatternsRoleEffectPermissionsCoversTopologyRoles` pins the §7 table to the topology roles;
  `human` is not a topology role and does not enter that table.

layering: no new component. The pattern schema owns the shape (`human` only on effect-free decision
nodes); the pattern lint owns the cross-field rule (human-floor outputs); the instance validator owns
the actor rule. Each check runs in a different component on a different signal, so a declared-shape
fault in one is caught by the next. None of them sees the merge act. Admission (from
graph-execution/14) is the only planned layer that does, and only for a merge requested through
dispatch or effect admission; the forge's confinement of the act is unverified.

design-fit:
  owner: statusgen/patterns.go (pattern contract) and statusgen/instance.go (instance contract)
  contract: none — the pattern and instance contracts are spec-owned (spec/workflow-pattern-v1.md, spec/workflow-instance-v1.md) with no semantic-owner index row
  retires: [the desk-role merge node, the `dispatch` effect that labels a merge, the named-not-derived risk-input lists]
  weight: "verbs 0, flags 0, refusals +3 lint/validator rules, rule-text lines +~25 across two specs"
  why-add: the human floor has no machine check in either contract today, and no existing rule can hold it; the §7 table cannot carry `human` without widening it, which stays unruled

## Human decision

The shared workflow pattern for implementation work has a merge step currently owned by an
automated coordinator. It will become a decision that only a person closes, by their own
authenticated act. This change declares that rule and checks the declaration; it does not by itself
stop an automated identity from merging. Today's code-hosting rules require an approving review and
passing checks but do not limit who presses merge, and the check that refuses an automated merge at
dispatch comes in a later step. The same change states which
gates each risk class must pass before the final integration check.

Proposed mapping: low risk needs review; standard risk needs review then post-merge verification;
elevated and human risk each need review, the human merge decision and verification. Elevated is
the "more heavily gated" class, so it gains the human merge decision as a named gate. Human-risk
work is separated from elevated work outside the pattern: automated agents may only read on it,
and its plan carries a human review gate.

Options:
1. **Adopt the derived mapping** — low: review; standard: review, verify; elevated: review, merge,
   verify; human: review, merge, verify. The merge step is a person's act for every class either way;
   naming it for elevated and human makes that gate one the admission check must see before the
   integration check runs.
2. **Name the merge step for every class** — low, standard, elevated and human all list review, merge
   (and verify from standard up). Elevated then equals standard, and telling them apart needs a
   separate lane, which is an open question with no ruling yet.

Default if no answer: option 1 after 2026-11-02.

## Ground rules

- NEVER git push, trigger workflows or run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented`; you do not set verified or done.
- Do not widen the §7 role-effect table or `patternRoleEffectPermissions`; `human` never enters it.
- Do not add a `merge` effect kind. A human node carries no effects at all.
- Do not edit brief 02. Its hold (the note in its `## Review` section) releases when this brief
  reaches `verified`, which is not the implementer's act.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Pattern: merge becomes a human decision.** In `spec/workflow-patterns/implementation-v1.yaml`,
   bump `version` to `2` (spec §3: bumped on every node edit after first review; the filename keeps
   its `-v1` schema marker). Reshape the `merge` node to exactly:
   `id: merge`, `kind: decision`, `role: human`, `inputs: [{artifact: approval}]`,
   `outputs: [merge-commit]`, one evidence entry
   `{kind: command, claim: "the forge records the merge of the approved head as an authenticated human act, and no role identity performed it", mandatory: true}`,
   no `effects` key, `outcomes: {wait: [checks-pending, awaiting-human], fail: [conflicting]}`.
   Update the header comment ("desk clears the gate" → a person merges the approved head). Edges,
   `join: verify` and the other four nodes do not change.
2. **Derived risk-input.** Set `risk-input` to the mapping the Human decision selects (default,
   option 1), one flow-style line per class in node order:
   `low: [review]`, `standard: [review, verify]`, `elevated: [review, merge, verify]`,
   `human: [review, merge, verify]`. Write the derivation into `spec/workflow-pattern-v1.md` §5
   as a worked example under the implementation pattern:
   - `low` → `[review]`: the non-author review floor every merged change carries
     (`docs/enforcement-model.md` implementer/reviewer separation; §4.2 rule 3).
   - `standard` → `[review, verify]`: `bounded` maps to `standard` (admission §6); a bounded change
     also owes non-implementer verification of the merged result.
   - `elevated` → `[review, merge, verify]`: `supervised-agent` maps to `elevated`, defined as
     bounded work "with the more heavily gated graph" (admission §2), so elevated MUST name a strict
     superset of standard's gates. The only gate node not already named is `merge`; `implement` is the
     work itself and `close` carries no mandatory evidence.
   - `human` → `[review, merge, verify]`: monotone (⊇ elevated) and containing `merge` by the ruling.
     What separates human from elevated is enforced outside the pattern: the `human-led` disposition
     lets agents only read (admission §2), a human-floor operation forces `human-led` (admission §4
     step 10, `humanFloorOperations` in `admission.go`), and a `gate: human` brief carries a human
     Review gate.
   - The merge floor itself holds for every class by node shape, not by `risk-input`: the node is a
     `human` decision whichever class runs it. The node shape only declares the floor. No verified
     control enforces the act today: the forge ruleset requires a non-author approval and the
     required checks but does not restrict who merges (see the single-point-of-failure line).
     Admission's human-floor refusal enforces it for a merge requested through dispatch once
     graph-execution/14 binds it there and derives the `merge` operation from the node's floor
     artifact (`merge-commit`), not its effect kind; a merge performed out of band with a held forge
     credential is bounded only by that credential's scope (unverified). The §8 decision record
     (`spec/workflow-instance-v1.md`) states that a human act happened. It does not prove who did it.
3. **Schema: `human` on effect-free decision nodes only.** In `schemas/workflow-pattern-v1.json`, add
   `human` to the node `role` enum and an `if {role: const human} then {kind: const decision, and
   effects absent or maxItems 0}` clause. Copy the file byte-for-byte to
   `statusgen/schemas/workflow-pattern-v1.json` (`TestPatternsEmbeddedSchemaMatchesCommitted`).
   In `spec/workflow-pattern-v1.md` §4.1, extend the `role` row: `human` is not a topology role and
   binds no identity; it is permitted only on `kind: decision` nodes with no `effects`, and such a node
   is closed by an external authenticated human act (instance §8). Add §4.2 rules 6 and 7 below. State
   in §7 that the table is unchanged, `human` holds no effect kind, and widening the table is out of scope.
   Replace the §4.1.1 sentence "`implementation`'s `merge` node produces a merge commit" with "is a
   human decision that produces a merge commit".
4. **Pattern lint, two new rules** in `statusgen/patterns.go`, reported as MUST-rule findings
   (`checked-failed: <path> [<rule>]: ...`, exit 1):
   - `pattern-human-floor-output`: a node whose `outputs` names a human-floor artifact — the closed
     list `merge-commit`, `release`, `deployment` — MUST be `kind: decision` with `role: human`; and
     any node with an effect, of any kind (`dispatch` included), whose `target` is a human-floor
     artifact is a finding. The list is a named constant beside the rules, matching the three
     operations `docs/enforcement-model.md` names. It is a deny-list, so the spec text (§4.2 rule 6)
     states its one-to-one correspondence with admission's `humanFloorOperations`
     (`tools/desk/internal/deskkit/admission.go`, `merge`, `release`, `deploy`): `merge-commit` ↔
     `merge`, `release` ↔ `release`, `deployment` ↔ `deploy`. A floor operation added on either side
     needs its artifact added on the other in the same change. The constant's doc comment says the same.
     The lint's output check alone does not see the effect-target branch: the existing
     effect-ownership rule (`pattern-effect-target-not-owned`) skips `kind: effect` nodes, so a
     desk effect node that outputs a non-floor artifact and dispatches at `merge-commit` lints clean
     today.
   - `pattern-human-role-not-decision`: a node with `role: human` MUST be `kind: decision` and MUST
     declare no effects. (The schema refuses the same shape independently; the lint holds it if the
     schema is ever loosened.)
   Both rules report even when the schema also refuses the node, so row 5 holds with the schema
   loosened. `role: human` is accepted by the existing role check only on such a node and only for this
   purpose; it never resolves through `patternRoleEffectPermissions`. New fixtures:
   `statusgen/testdata/patterns/bad-merge-behind-dispatch.yaml` (planned), which is today's version 1
   file verbatim and must fail `pattern-human-floor-output` and not `pattern-effect-exceeds-role`;
   `bad-human-role-not-decision.yaml`, a `role: human, kind: effect` node with a `comment` effect; and
   `statusgen/testdata/patterns/bad-floor-effect-target.yaml` (planned), today's version 1 file with
   only the merge node's output renamed to `merge-request` (and `verify`'s input renamed to match),
   so the node stays `kind: effect, role: desk` with `{kind: dispatch, target: merge-commit}` and
   outputs no floor artifact. It must fail `pattern-human-floor-output` through the effect-target
   branch alone. Tests: `TestPatternsHumanFloorOutput` (planned), covering both the output fixture
   and the effect-target fixture, and `TestPatternsHumanRoleDecision` (planned).
5. **Re-pin the twins.** Copy the new pattern byte-for-byte to
   `statusgen/testdata/patterns/good-implementation-v1.yaml`, and to
   `statusgen/testdata/instances/root/spec/workflow-patterns/implementation-v1.yaml` keeping its
   existing two-line FIXTURE header (so `tail -n +3` of the instance twin equals the spec file).
   Re-pin every instance fixture that binds the pattern: compute the new digest with
   `shasum -a 256` over the instance-root twin (the file the validator resolves), set `version: 2`,
   and replace the old digest in all 11 fixtures named in Context. Re-derive `superseding.yaml`'s
   `supersedes.contract-digest` from `statusgen instance validate --file testdata/instances/valid.yaml
   --root testdata/instances/root --json` (`contract-digest`). The old digest must not remain anywhere.
6. **Instance contract.** In `spec/workflow-instance-v1.md` §5 add rule 9: a node instance whose
   pattern node has `role: human` MUST carry no `actor`, no `effects` entries and no `attempts`; the
   human act is recorded only in `decisions[]` (§8) (`instance-human-node-actor`). In
   `schemas/workflow-instance-v1.json` (and its embedded copy) allow `role: human` with `actor`
   absent and `effects` empty for that role only. In `statusgen/instance.go` add the rule constant,
   skip the §7 capability lookup only for a `human` node, and refuse an actor, capability or effect on
   one. Two existing code paths assume every node has an actor and must change with it:
   - `instanceNode.Actor` is a value struct (`Actor instanceActor`), and the one-Cell check compares
     `n.Actor.Cell` with `inst.Work.Cell` for every node. Make the actor optional (a pointer, or
     omitted when empty) and skip the one-Cell check for a `human` node, so a human node with no
     actor is not reported as `instance-cross-cell`.
   - `instantiate` refuses a pattern node whose role has no resolved actor ("no resolved actor for
     role"), and `TestInstanceRoundTripIdentity` re-instantiates `valid.yaml`. Skip actor resolution
     for the `human` role there, so the round trip produces a merge node with no actor.
   Reshape the merge node in every instance fixture to `role: human`, no actor, `effects: []`, the
   new evidence claim. Add to `valid.yaml` a decision `dec-example-07-merge` on
   `inst-example-07-a.merge`, outcome `merged`, act `{kind: external-authenticated-act,
   ref: example-org/example-app/pull/42#merge}`. New fixtures: `human-node-actor.yaml` (merge node
   `role: human` with a desk actor, expect `instance-human-node-actor`) and `merge-desk-actor.yaml`
   (today's desk merge node against the new pattern, expect `instance-node-mismatch` and
   `instance-effect-not-declared`). Add both to `TestInstanceFixtures`; add `TestInstanceHumanNodeActor` (planned).
7. **Docs and changelog.** In `docs/lifecycle.md`, where the review and verify nodes are described,
   state that merge is a human decision node between them. Add
   `changelog/graph-execution-33-merge-decision-node.md` (planned) with `### Changed` and one `- ` bullet.

Out of scope, named as follow-ups (none is ruled here):
- The review advisories on the pattern: `review` and `verify` under-declare effects (each declares
  `comment`, not `review` / `evidence-commit`); the review-same-role rule reads only the first
  producer of an input (`findProducerOf`); `research-v1`'s `collect-sources` and `resolve-gaps`
  declare no persisting effect.
- `research-v1`'s `risk-input` maps all four classes to the same list.
- `verify` is both the `join` and a named `risk-input` gate, while §5 says named gates run
  "before join runs"; the wording is left to a spec follow-up.
- Widening the §7 role-effect table (the remaining gaps of medici-finance/assay#4506) stays unruled.
- A validator rule that ties a `risk-input` decision gate to a recorded decision belongs to
  graph-execution/14 (admission binding) and /21 (runtime).

## Verify

Rows run from the repository root on the implementation head, in a POSIX shell. Exit 3 means
could-not-check (setup failed); exit 4 or 5 means the expected finding was missing or a wrong
one appeared. Neither is a pass.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" patterns --lint --root .)` | exit 0; output contains "0 checked-failed, 0 could-not-check" |
| 2 | check | `(f=spec/workflow-patterns/implementation-v1.yaml; n=$(awk '/^  - id: merge$/{p=1;print;next} p&&/^  - id: /{p=0} p' "$f"); [ -n "$n" ] \|\| exit 3; printf '%s\n' "$n" \| grep -Fxq '    kind: decision' && printf '%s\n' "$n" \| grep -Fxq '    role: human' && ! printf '%s\n' "$n" \| grep -Eq '^    effects:' && ! grep -Fq 'target: merge-commit' "$f" && grep -Fxq 'version: 2' "$f")` | exit 0 |
| 3 | check | `(f=spec/workflow-patterns/implementation-v1.yaml; grep -Fxq '  low: [review]' "$f" && grep -Fxq '  standard: [review, verify]' "$f" && grep -Fxq '  elevated: [review, merge, verify]' "$f" && grep -Fxq '  human: [review, merge, verify]' "$f")` | exit 0 (option 1 mapping; if the human decision selects option 2, the reviewer re-states these four lines before implementation) |
| 4 | check | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-merge-behind-dispatch.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-human-floor-output]' "$T/out" \|\| exit 4; grep -Fq '[pattern-effect-exceeds-role]' "$T/out" && exit 5; exit $e)` | exit 1 (fail-first: on b1a7e4233 the same file lints clean, exit 0, so this row cannot pass there) |
| 5 | check | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-human-role-not-decision.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-human-role-not-decision]' "$T/out" \|\| exit 4; exit $e)` | exit 1 |
| 6 | check | `python3 -c 'import importlib.util as u,sys,os; sys.excepthook=lambda *a: os._exit(3); sys.exit(3) if not (u.find_spec("yaml") and u.find_spec("jsonschema") and os.path.isfile("statusgen/testdata/patterns/bad-human-role-not-decision.yaml")) else None; import json,yaml,jsonschema; v=jsonschema.Draft202012Validator(json.load(open("schemas/workflow-pattern-v1.json"))); g=yaml.safe_load(open("spec/workflow-patterns/implementation-v1.yaml")); sys.exit(3) if not any(n.get("role")=="human" and n.get("kind")=="decision" and not n.get("effects") for n in g["nodes"]) else None; sys.exit(6) if list(v.iter_errors(g)) else None; e=list(v.iter_errors(yaml.safe_load(open("statusgen/testdata/patterns/bad-human-role-not-decision.yaml")))); sys.exit(4) if not e else None; sys.exit(5) if any(x.validator=="enum" and list(x.absolute_path)[-1:]==["role"] for x in e) else None; sys.exit(1 if any(list(x.absolute_path)[-1:] in (["kind"],["effects"]) for x in e) else 7)'` | exit 1: the positive control (the shipped `merge` node, `role: human, kind: decision`, no effects) validates, and the bad fixture is refused on its `kind` or `effects`, not on the `role` enum. Exit 3 = could-not-check (`yaml`/`jsonschema` absent, a file missing or unparsable, or no human decision node yet); 4 = fixture accepted; 5 = refused by the role enum; 6 = positive control refused. On 36665f2b0 this row exits 3 (no human decision node; fixture absent), and with both files present but the schema unchanged it exits 6 |
| 7 | check | `python3 -c 'import importlib.util,sys; sys.exit(3) if not all(importlib.util.find_spec(m) for m in ("yaml","jsonschema")) else None; import json,yaml,jsonschema; jsonschema.validate(yaml.safe_load(open("spec/workflow-patterns/implementation-v1.yaml")), json.load(open("schemas/workflow-pattern-v1.json"))); print("schema-valid")'` | exit 0; output is "schema-valid" |
| 8 | check +flow | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" instance validate --file statusgen/testdata/instances/valid.yaml --root statusgen/testdata/instances/root)` | exit 0; output contains "checked-clean" |
| 9 | check | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" instance validate --file statusgen/testdata/instances/human-node-actor.yaml --root statusgen/testdata/instances/root > "$T/out" 2>&1; e=$?; grep -Fq '[instance-human-node-actor]' "$T/out" \|\| exit 4; exit $e)` | exit 1 |
| 10 | check +flow | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" instance validate --file statusgen/testdata/instances/merge-desk-actor.yaml --root statusgen/testdata/instances/root > "$T/out" 2>&1; e=$?; grep -Fq '[instance-node-mismatch]' "$T/out" && grep -Fq '[instance-effect-not-declared]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (lower layer: a desk-owned merge instance is refused with no human act present) |
| 11 | check | `(cmp spec/workflow-patterns/implementation-v1.yaml statusgen/testdata/patterns/good-implementation-v1.yaml && tail -n +3 statusgen/testdata/instances/root/spec/workflow-patterns/implementation-v1.yaml \| cmp - spec/workflow-patterns/implementation-v1.yaml && cmp schemas/workflow-pattern-v1.json statusgen/schemas/workflow-pattern-v1.json && ! grep -rFq edcf96e1eed65dc4a7a09cd2ed0e55fd8c0dba018a0452b959b7e71ffe13af94 statusgen/testdata)` | exit 0 |
| 12 | check | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go test -count=1 -v -run '^(TestPatterns\|TestInstance)' .) > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; for t in TestPatternsHumanFloorOutput TestPatternsHumanRoleDecision TestInstanceHumanNodeActor; do grep -Fq -- "--- PASS: $t (" "$T/out" \|\| exit 4; done)` | exit 0 (exit 4 = a named test did not run and pass) |
| 13 | check | `(cd tools/desk && GOWORK=off go test -count=1 -run '^TestAgenticAdmissionRiskInputUnion$' ./internal/deskkit/)` | exit 0; output contains "ok" |
| 14 | check +mutation | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-floor-effect-target.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; grep -Fq 'target: merge-commit' "$T/r/spec/workflow-patterns/bad-floor-effect-target.yaml" \|\| exit 3; [ "$(grep -Fc 'merge-commit' "$T/r/spec/workflow-patterns/bad-floor-effect-target.yaml")" -eq 1 ] \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-human-floor-output]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (effect-target branch alone: `merge-commit` appears exactly once in the fixture, on the effect target, so no flow- or block-style output names it; exit 3 otherwise. Fail-first: on 36665f2b0 the same file lints clean, `patterns: 1 checked-clean`, exit 0) |

## Pre-mortem

- A merge hides behind another label (`comment`, `file-issue`) or a renamed output: Task 4's lint
  keys on the output (row 4) and on any effect targeting a human-floor artifact, whatever the effect
  kind (row 14).
- An agent performs the merge and writes a well-formed `decisions[]` record: nothing in this brief
  catches it, because the record is not bound to an identity, and the forge ruleset does not
  restrict who merges. Nothing verified catches it until admission's human-floor refusal is bound at
  graph-execution/14, which derives `merge` from the floor artifact and whose Verify row 9 feeds a
  `dispatch`-labelled merge node. A merge done out of band with a held credential stays bounded only
  by that credential's scope.
- A new output name (`merged-head`) escapes the closed list: the list is a named constant with the
  three floor operations, and the spec text ties it one-to-one to admission's `humanFloorOperations`;
  a pattern adding a new floor-shaped output is a pattern review question.
- The instance validator still resolves `human` through the §7 table and refuses every shipped
  instance, or accepts a human node with an actor: rows 8 and 9 hold both directions.
- A twin or digest is left stale and the instance fixtures fail for `instance-pattern-drift`
  rather than for the intended rule: row 11 checks every twin and the old digest.

## Definition of done

- Rows 1-14 pass as stated (rows 6 and 7 may report could-not-check only where `yaml`/`jsonschema`
  are absent, and the verifier says so).
- No agent role holds an effect targeting `merge-commit` (rows 4 and 14); the §7 table is unchanged
  (row 12, `TestPatternsRoleEffectPermissionsCoversTopologyRoles`).

## Evidence

<!-- appended at implementation time, one row per Verify item, by someone who did not implement -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" patterns --lint --root .)` | pass exit=0 | sha256:2ca49b205d93 | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 2 | `(f=spec/workflow-patterns/implementation-v1.yaml; n=$(awk '/^  - id: merge$/{p=1;print;next} p&&/^  - id: /{p=0} p' "$f"); [ -n "$n" ] \|\| exit 3; printf '%s\n' "$n" \| grep -Fxq '    kind: decision' && printf '%s\n' "$n" \| grep -Fxq '    role: human' && ! printf '%s\n' "$n" \| grep -Eq '^    effects:' && ! grep -Fq 'target: merge-commit' "$f" && grep -Fxq 'version: 2' "$f")` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 3 | `(f=spec/workflow-patterns/implementation-v1.yaml; grep -Fxq '  low: [review]' "$f" && grep -Fxq '  standard: [review, verify]' "$f" && grep -Fxq '  elevated: [review, merge, verify]' "$f" && grep -Fxq '  human: [review, merge, verify]' "$f")` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 4 | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-merge-behind-dispatch.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-human-floor-output]' "$T/out" \|\| exit 4; grep -Fq '[pattern-effect-exceeds-role]' "$T/out" && exit 5; exit $e)` | pass exit=1 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 5 | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-human-role-not-decision.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-human-role-not-decision]' "$T/out" \|\| exit 4; exit $e)` | pass exit=1 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 6 | `python3 -c 'import importlib.util as u,sys,os; sys.excepthook=lambda *a: os._exit(3); sys.exit(3) if not (u.find_spec("yaml") and u.find_spec("jsonschema") and os.path.isfile("statusgen/testdata/patterns/bad-human-role-not-decision.yaml")) else None; import json,yaml,jsonschema; v=jsonschema.Draft202012Validator(json.load(open("schemas/workflow-pattern-v1.json"))); g=yaml.safe_load(open("spec/workflow-patterns/implementation-v1.yaml")); sys.exit(3) if not any(n.get("role")=="human" and n.get("kind")=="decision" and not n.get("effects") for n in g["nodes"]) else None; sys.exit(6) if list(v.iter_errors(g)) else None; e=list(v.iter_errors(yaml.safe_load(open("statusgen/testdata/patterns/bad-human-role-not-decision.yaml")))); sys.exit(4) if not e else None; sys.exit(5) if any(x.validator=="enum" and list(x.absolute_path)[-1:]==["role"] for x in e) else None; sys.exit(1 if any(list(x.absolute_path)[-1:] in (["kind"],["effects"]) for x in e) else 7)'` | pass exit=1 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 7 | `python3 -c 'import importlib.util,sys; sys.exit(3) if not all(importlib.util.find_spec(m) for m in ("yaml","jsonschema")) else None; import json,yaml,jsonschema; jsonschema.validate(yaml.safe_load(open("spec/workflow-patterns/implementation-v1.yaml")), json.load(open("schemas/workflow-pattern-v1.json"))); print("schema-valid")'` | pass exit=0 | sha256:6acf9f2299e7 | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 8 | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" instance validate --file statusgen/testdata/instances/valid.yaml --root statusgen/testdata/instances/root)` | pass exit=0 | sha256:7acd29414ae0 | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 9 | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" instance validate --file statusgen/testdata/instances/human-node-actor.yaml --root statusgen/testdata/instances/root > "$T/out" 2>&1; e=$?; grep -Fq '[instance-human-node-actor]' "$T/out" \|\| exit 4; exit $e)` | pass exit=1 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 10 | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" instance validate --file statusgen/testdata/instances/merge-desk-actor.yaml --root statusgen/testdata/instances/root > "$T/out" 2>&1; e=$?; grep -Fq '[instance-node-mismatch]' "$T/out" && grep -Fq '[instance-effect-not-declared]' "$T/out" \|\| exit 4; exit $e)` | pass exit=1 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 11 | `(cmp spec/workflow-patterns/implementation-v1.yaml statusgen/testdata/patterns/good-implementation-v1.yaml && tail -n +3 statusgen/testdata/instances/root/spec/workflow-patterns/implementation-v1.yaml \| cmp - spec/workflow-patterns/implementation-v1.yaml && cmp schemas/workflow-pattern-v1.json statusgen/schemas/workflow-pattern-v1.json && ! grep -rFq edcf96e1eed65dc4a7a09cd2ed0e55fd8c0dba018a0452b959b7e71ffe13af94 statusgen/testdata)` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 12 | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go test -count=1 -v -run '^(TestPatterns\|TestInstance)' .) > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; for t in TestPatternsHumanFloorOutput TestPatternsHumanRoleDecision TestInstanceHumanNodeActor; do grep -Fq -- "--- PASS: $t (" "$T/out" \|\| exit 4; done)` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 13 | `(cd tools/desk && GOWORK=off go test -count=1 -run '^TestAgenticAdmissionRiskInputUnion$' ./internal/deskkit/)` | pass exit=0 | sha256:6fe131a04072 | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
| 14 | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp statusgen/testdata/patterns/bad-floor-effect-target.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; grep -Fq 'target: merge-commit' "$T/r/spec/workflow-patterns/bad-floor-effect-target.yaml" \|\| exit 3; [ "$(grep -Fc 'merge-commit' "$T/r/spec/workflow-patterns/bad-floor-effect-target.yaml")" -eq 1 ] \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; e=$?; grep -Fq '[pattern-human-floor-output]' "$T/out" \|\| exit 4; exit $e)` | pass exit=1 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 65a4deb064cb (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5203 run=fa6a0ba1062f1a6c32673f9b1af126790c8ee746afa690ccd35f14224aeca648 source=65a4deb064cb58ccc387b276e6ddf5dfc9f7aef2 brief=docs/streams/graph-execution/brief-33-merge-decision-node-and-risk-input.md model=claude-opus-5-5 tier=strong

**VERIFY: PASS** — 14 of 14 rows pass at 65a4deb064cb on the darwin-arm64 host, run by statusgen verifyrun v1.0.67 (every row is class check, none check:ci, so no container bracket). Non-implementer: the work was delivered by medici-finance/assay#4897 (worker App author, merge cccead154fe2).

Run notes, so the run can be reproduced: rows 6 and 7 need python3 with yaml and jsonschema. The default python3 on PATH has neither (both rows exit 3 that way), so python3 on PATH was pointed at a Homebrew python 3.14 whose site-packages holds both (jsonschema 4.21.1). The row commands were left unchanged. TMPDIR sat outside the home. Key observed lines (supplemental, reading the same binary and fixtures, no extra acceptance credit): row 1 prints "patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned)". Row 4 reports [pattern-human-floor-output] on both the output and the effect-target branch, with no [pattern-effect-exceeds-role]. Row 5 reports [pattern-human-role-not-decision]. Row 14 reports only [pattern-human-floor-output] through the effect-target branch. Row 8 prints "valid.yaml checked-clean". Row 9 reports [instance-human-node-actor]. Row 10 reports [instance-node-mismatch] (role "desk" against pattern role "human") and [instance-effect-not-declared] (dispatch to merge-commit).

Risk-bearing values, enumerated over the delivered pattern, schema and lint constants:
- RISK-VALUE: DERIVED — patternFloorMerge/patternFloorRelease/patternFloorDeploy = "merge-commit"/"release"/"deployment" @ statusgen/patterns.go:91-93 — each maps one-to-one onto humanFloorOperations = {"merge", "release", "deploy"} @ tools/desk/internal/deskkit/admission.go:416, which are the three operations docs/enforcement-model.md:163 names as human floors.
- RISK-VALUE: DERIVED — risk-input elevated = [review, merge, verify] and human = [review, merge, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — elevated must be a strict superset of standard [review, verify] (admission spec section 2: supervised-agent is "the more heavily gated graph"). merge is the only gate node standard does not already name. human is monotone over elevated. This is option 1, the option the human ruling selected (https://github.com/medici-finance/assay/issues/4511#issuecomment-6560687132).
- RISK-VALUE: DERIVED — node role enum adds "human", constrained by if role=human then kind=decision and effects maxItems 0 @ schemas/workflow-pattern-v1.json:42-43,57 — this is the Task 3 shape, and row 6 shows the schema refuses the bad fixture on kind/effects, not on the role enum.
- Lower rank (reversible, no derivation owed): pattern version: 2 @ spec/workflow-patterns/implementation-v1.yaml:13. Instance pattern digest sha256:d9b81b3d15963b374714fc1cf408e6c867be33f9c0b7ab99a05d99e3e985a57b @ statusgen/testdata/instances/valid.yaml:15 equals shasum -a 256 of the instance-root twin, recomputed here.

Gate: human. This Evidence does not sign off the item. The human confirms the mapping and that no agent role gains a merge capability.

## Review

Gate: human (frontmatter). The reviewer answers both rule-10 questions in the verdict:
(1) no verified control stands between an agent merge and the damage today. The forge ruleset
requires a pull request, a non-last-pusher approving review and the required checks; it does not
restrict which identity performs the merge, and whether anything else on the forge does is
unverified. Schema, lint and instance validator guard only the declared shape (a pattern or instance
that names an agent merge); none of them sees the act, because a decision record carries no verified
identity. Admission's human-floor refusal becomes the first verified layer on a merge requested
through dispatch when graph-execution/14 binds it there and derives the `merge` operation from the
node's floor artifact rather than its effect kind (its Verify row 9 feeds an agent node
`{kind: dispatch, target: merge-commit}` with the upper layers bypassed). Admission sees only
requested operations: a merge performed out of band with a held forge credential is bounded only by
that credential's scope (unverified). /14 depends on this brief, and the <!-- graph: encoded graph-execution/33→graph-execution/14 -->
runtime record of a human act belongs to /14 and /21. Is it acceptable to land the declared-shape
layers while the act itself stays unconfined until /14?
(2) do any rows prove a lower layer catches the fault with the upper layer bypassed? For the
declared shape, yes: rows 4, 5, 6, 9, 10 and 14 each feed one fault straight to one validator with
no other layer present. For the act, no: no row here proves a lower layer catches an agent that
performs a merge and records it as a human decision. That proof is owed by graph-execution/14's
Verify row 9, which runs `TestGraphAdmissionHumanFloorMerge` (planned), not claimed here.
