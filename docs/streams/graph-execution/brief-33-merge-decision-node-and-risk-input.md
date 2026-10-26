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
unblocks: ["graph-execution/14"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  The brief changes the human merge floor's encoding in the pattern schema, the pattern lint
  and the instance validator, and it fixes the risk-class to mandatory-gate mapping admission
  will bind to. The human confirms the derived mapping and that no agent role gains a merge
  capability.
decision-trigger: creation
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

files: `spec/workflow-patterns/implementation-v1.yaml`, `statusgen/testdata/patterns/good-implementation-v1.yaml`,
`statusgen/testdata/instances/root/spec/workflow-patterns/implementation-v1.yaml`, `spec/workflow-pattern-v1.md`,
`schemas/workflow-pattern-v1.json`, `statusgen/schemas/workflow-pattern-v1.json`, `statusgen/patterns.go`,
`statusgen/patterns_test.go`, `statusgen/testdata/patterns/bad-merge-behind-dispatch.yaml` (planned),
`statusgen/testdata/patterns/bad-human-role-not-decision.yaml` (planned), `spec/workflow-instance-v1.md`,
`schemas/workflow-instance-v1.json` and the embedded copy `TestInstanceSchemaParity` pins, `statusgen/instance.go`,
`statusgen/instance_test.go`, `statusgen/testdata/instances/*.yaml` (every fixture that pins the pattern digest),
`statusgen/testdata/instances/human-node-actor.yaml` (planned), `statusgen/testdata/instances/merge-desk-actor.yaml` (planned),
`docs/lifecycle.md`, `changelog/graph-execution-33-merge-decision-node.md` (planned).

single-point-of-failure: the external authenticated human act that closes the `merge` decision —
behind it: (1) the pattern schema refuses `role: human` on anything but an effect-free decision node,
checked by an independent JSON Schema validator; (2) the pattern lint refuses any node outputting a
human-floor artifact unless it is a `human` decision node, whatever its effect is labelled;
(3) the instance validator refuses an actor, a capability or an effect on a `human` node and
refuses a desk-role merge instance; (4) admission's human-floor refusal once /14 binds it at
dispatch; (5) out of band, the forge binding grants no role a merge permission.

facts (b1a7e4233, 2026-10-26):
- Ruling, per the driver's ruling (https://github.com/medici-finance/assay/issues/1933#issuecomment-6419943765):
  merge becomes a `kind: decision` node closed by an external authenticated human act; `human`
  is allowed as an owner only on decision nodes; the pattern and its testdata twin are re-pinned;
  graph-execution/02 stays held until this brief lands.
- Today `merge` is `kind: effect, role: desk`, outputs `[merge-commit]`, effect
  `{kind: dispatch, target: merge-commit}`. Every check passes: `dispatch` is in the desk row of the
  §7 role-effect table, and the vocabulary has no `merge` effect kind, so nothing ties the output
  to the human floor. `statusgen patterns --lint --root .` prints `patterns: 2 checked-clean,
  0 checked-failed, 0 could-not-check (2 file(s) scanned)` and exits 0.
- The instance validator reuses the same table (`statusgen/instance.go`, `perm :=
  patternRoleEffectPermissions[n.Role]`), and `statusgen/testdata/instances/valid.yaml` gives the
  merge node a desk actor holding `dispatch`. All 12 instance fixtures that bind the pattern pin
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
the actor rule. Each check runs in a different component on a different signal, so a fault in one is
caught by the next. Admission and the forge stay the enforcing layers they are today.

design-fit:
  owner: statusgen/patterns.go (pattern contract) and statusgen/instance.go (instance contract)
  contract: none — the pattern and instance contracts are spec-owned (spec/workflow-pattern-v1.md, spec/workflow-instance-v1.md) with no semantic-owner index row
  retires: [the desk-role merge node, the `dispatch` effect that labels a merge, the named-not-derived risk-input lists]
  weight: "verbs 0, flags 0, refusals +3 lint/validator rules, rule-text lines +~25 across two specs"
  why-add: the human floor has no machine check in either contract today, and no existing rule can hold it; the §7 table cannot carry `human` without widening it, which stays unruled

## Human decision

The shared workflow pattern for implementation work has a merge step currently owned by an
automated coordinator. It will become a decision that only a person closes, by their own
authenticated act; no automated role will be able to produce a merge. The same change states which
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

Default if no answer: option 1 after 7 days.

## Ground rules

- NEVER git push, trigger workflows or run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented`; you do not set verified or done.
- Do not widen the §7 role-effect table or `patternRoleEffectPermissions`; `human` never enters it.
- Do not add a `merge` effect kind. A human node carries no effects at all.
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
     `human` decision whichever class runs it. Where the human gate is enforced: the external
     authenticated act (`spec/workflow-instance-v1.md` §8), admission's human-floor refusal (bound at
     dispatch by graph-execution/14), and the forge binding, which grants no role a merge.
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
     operations `docs/enforcement-model.md` names.
   - `pattern-human-role-not-decision`: a node with `role: human` MUST be `kind: decision` and MUST
     declare no effects. (The schema refuses the same shape independently; the lint holds it if the
     schema is ever loosened.)
   Both rules report even when the schema also refuses the node, so row 5 holds with the schema
   loosened. `role: human` is accepted by the existing role check only on such a node and only for this
   purpose; it never resolves through `patternRoleEffectPermissions`. New fixtures:
   `statusgen/testdata/patterns/bad-merge-behind-dispatch.yaml` (planned), which is today's version 1
   file verbatim and must fail `pattern-human-floor-output` and not `pattern-effect-exceeds-role`; and
   `bad-human-role-not-decision.yaml`, a `role: human, kind: effect` node with a `comment` effect.
   Tests: `TestPatternsHumanFloorOutput` (planned), `TestPatternsHumanRoleDecision` (planned).
5. **Re-pin the twins.** Copy the new pattern byte-for-byte to
   `statusgen/testdata/patterns/good-implementation-v1.yaml`, and to
   `statusgen/testdata/instances/root/spec/workflow-patterns/implementation-v1.yaml` keeping its
   existing two-line FIXTURE header (so `tail -n +3` of the instance twin equals the spec file).
   Re-pin every instance fixture that binds the pattern: compute the new digest with
   `shasum -a 256` over the instance-root twin (the file the validator resolves), set `version: 2`,
   and replace the old digest in all 12 fixtures. Re-derive `superseding.yaml`'s
   `supersedes.contract-digest` from `statusgen instance validate --file testdata/instances/valid.yaml
   --root testdata/instances/root --json` (`contract-digest`). The old digest must not remain anywhere.
6. **Instance contract.** In `spec/workflow-instance-v1.md` §5 add rule 9: a node instance whose
   pattern node has `role: human` MUST carry no `actor`, no `effects` entries and no `attempts`; the
   human act is recorded only in `decisions[]` (§8) (`instance-human-node-actor`). In
   `schemas/workflow-instance-v1.json` (and its embedded copy) allow `role: human` with `actor`
   absent and `effects` empty for that role only. In `statusgen/instance.go` add the rule constant,
   skip the §7 capability lookup only for a `human` node, and refuse an actor, capability or effect on
   one. Reshape the merge node in every instance fixture to `role: human`, no actor, `effects: []`, the
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
| 6 | check | `python3 -c 'import importlib.util,sys; sys.exit(3) if not all(importlib.util.find_spec(m) for m in ("yaml","jsonschema")) else None; import json,yaml,jsonschema; s=json.load(open("schemas/workflow-pattern-v1.json")); d=yaml.safe_load(open("statusgen/testdata/patterns/bad-human-role-not-decision.yaml")); v=jsonschema.Draft202012Validator(s); sys.exit(1 if list(v.iter_errors(d)) else 0)'` | exit 1 (the schema alone refuses a non-decision `human` node; exit 3 = could-not-check) |
| 7 | check | `python3 -c 'import importlib.util,sys; sys.exit(3) if not all(importlib.util.find_spec(m) for m in ("yaml","jsonschema")) else None; import json,yaml,jsonschema; jsonschema.validate(yaml.safe_load(open("spec/workflow-patterns/implementation-v1.yaml")), json.load(open("schemas/workflow-pattern-v1.json"))); print("schema-valid")'` | exit 0; output is "schema-valid" |
| 8 | check +flow | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" instance validate --file statusgen/testdata/instances/valid.yaml --root statusgen/testdata/instances/root)` | exit 0; output contains "checked-clean" |
| 9 | check | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" instance validate --file statusgen/testdata/instances/human-node-actor.yaml --root statusgen/testdata/instances/root > "$T/out" 2>&1; e=$?; grep -Fq '[instance-human-node-actor]' "$T/out" \|\| exit 4; exit $e)` | exit 1 |
| 10 | check +flow | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; "$T/sg" instance validate --file statusgen/testdata/instances/merge-desk-actor.yaml --root statusgen/testdata/instances/root > "$T/out" 2>&1; e=$?; grep -Fq '[instance-node-mismatch]' "$T/out" && grep -Fq '[instance-effect-not-declared]' "$T/out" \|\| exit 4; exit $e)` | exit 1 (lower layer: a desk-owned merge instance is refused with no human act present) |
| 11 | check | `(cmp spec/workflow-patterns/implementation-v1.yaml statusgen/testdata/patterns/good-implementation-v1.yaml && tail -n +3 statusgen/testdata/instances/root/spec/workflow-patterns/implementation-v1.yaml \| cmp - spec/workflow-patterns/implementation-v1.yaml && cmp schemas/workflow-pattern-v1.json statusgen/schemas/workflow-pattern-v1.json && ! grep -rFq edcf96e1eed65dc4a7a09cd2ed0e55fd8c0dba018a0452b959b7e71ffe13af94 statusgen/testdata)` | exit 0 |
| 12 | check | `(cd statusgen && GOWORK=off go test -count=1 -run '^(TestPatterns\|TestInstance)' .)` | exit 0; output contains "ok" |
| 13 | check | `(cd tools/desk && GOWORK=off go test -count=1 -run '^TestAgenticAdmissionRiskInputUnion$' ./internal/deskkit/)` | exit 0; output contains "ok" |

## Pre-mortem

- A merge hides behind another label (`comment`, `file-issue`) or a renamed output: rule 4's lint
  keys on the output and on any effect targeting a human-floor artifact, whatever the effect kind.
- A new output name (`merged-head`) escapes the closed list: the list is a named constant with the
  three floor operations; a pattern adding a new floor-shaped output is a pattern review question,
  stated in the spec text.
- The instance validator still resolves `human` through the §7 table and refuses every shipped
  instance, or accepts a human node with an actor: rows 8 and 9 hold both directions.
- A twin or digest is left stale and the instance fixtures fail for `instance-pattern-drift`
  rather than for the intended rule: row 11 checks every twin and the old digest.

## Definition of done

- Rows 1-13 pass as stated (rows 6 and 7 may report could-not-check only where `yaml`/`jsonschema`
  are absent, and the verifier says so).
- No agent role holds an effect targeting `merge-commit`; the §7 table is unchanged.
- graph-execution/02's hold is released by this brief landing (stream README note).

## Evidence

<!-- appended at implementation time, one row per Verify item, by someone who did not implement -->

## Review

Gate: human (frontmatter). The reviewer answers both rule-10 questions in the verdict:
(1) the single control between an agent merge and the damage is the external authenticated human
act closing `merge`, with schema, lint, instance validator, admission and forge binding behind it —
is that acceptable? (2) do rows 4, 5, 6, 9 and 10 prove a lower layer refuses the fault with the
upper layer bypassed? They do by construction: each feeds a fault directly to one validator with no
human act and no other layer present.
