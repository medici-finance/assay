---
brief: assay:assay:graph-execution:02
title: Workflow-pattern schema, node contract, and the implementation and research patterns
why: >-
  Today the shape of a piece of work — review, then merge, then someone else verifies — lives
  in each desk's procedure text and routing code, so it cannot be reviewed as one artifact,
  versioned, or varied per task. Writing the two workflows the fleet actually runs as
  reviewed, machine-checkable pattern files, with every node declaring what it needs, what
  it produces, what evidence it owes and what it may touch, is what makes a workflow
  something a reviewer can approve and a tool can refuse.
wave: 0
depends: []
unblocks: ["graph-execution/03", "graph-execution/04", "graph-execution/05", "graph-execution/08"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
  - "docs/streams/graph-execution/spec.md §2.1 (a pattern bank, selected then adapted), §2.2 (a node is an execution contract; risk class as a declared input; a generated graph must not grant itself permissions)"
  - "spec/README.md and spec/brief-v1.md (the normative-spec home and the document shape a new spec/ file follows)"
  - "topology.yaml `apps:` block (the four role names a node may bind: desk, reviewer, verifier, worker) and its header (compiled derivation, TestTopologyValuesMatchSource)"
  - "docs/enforcement-model.md (the implementer↔reviewer separation the implementation pattern must preserve)"
  - "Daniel Fink, nodes declare what they do / require / may call — https://youtu.be/6V6BAb3qG6U?t=80 ; data-driven interaction testing — https://youtu.be/6V6BAb3qG6U?t=185"
  - "Furong Huang, precomputed workflow bank adapted per task — https://youtu.be/GfmK-v8CARk?t=603"
  - "freshness-checked 2026-09-16 @ d96fd3ba: `ls spec/` lists brief-v1.md, lifecycle-v1.md, registers-v1.md, README.md only; `grep -rln 'workflow-pattern\\|pattern-effect-exceeds-role' statusgen spec docs` hits only docs/streams/graph-execution/README.md — not already satisfied"
exec-tier: strong
exec-tier-why: "(a) the node vocabulary and the effect-permission model are design decisions the facts do not pre-specify, and every later brief in the stream reuses them verbatim; (c) a permission model that lets a pattern name an effect its role is not bound to is exactly the fault the lint exists to refuse, and it survives ordinary tests"
domain: complicated
consumers:
  - "spec/README.md (the spec index table gains the workflow-pattern row): fixed-here (the row is added in this change)"
  - "statusgen/main.go (the `patterns --lint` subcommand): fixed-here (the subcommand is wired in this change)"
  - "docs/lifecycle.md §Review gates (which pattern node a review gate is): fixed-here (the sentence is added in this change)"
  - "plugins/assay/skills/worker-desk/SKILL.md, pr-review-desk/SKILL.md, verify-desk/SKILL.md (the procedures the implementation pattern encodes): out-of-scope (the pattern file describes the existing procedure and changes none of it; a skill that later READS the pattern is graph-execution/05's report to propose)"
version: 1
id: ed7644f5-3b16-4050-955c-2e522e5dc257
---

# Brief 02 — Workflow-pattern schema, node contract, and the implementation and research patterns

## Context
files: `spec/workflow-pattern-v1.md` (planned), `spec/workflow-patterns/implementation-v1.yaml` (planned), `spec/workflow-patterns/research-v1.yaml` (planned), `spec/README.md`, `schemas/workflow-pattern-v1.json` (planned), `statusgen/patterns.go` (planned), `statusgen/patterns_test.go` (planned), `statusgen/testdata/patterns/` (planned; good and bad fixtures), `statusgen/main.go`, `docs/lifecycle.md`, `changelog/graph-execution-02-pattern-schema.md` (planned)
facts:
- `spec/` is the normative, versioned, third-party-implementable home (`spec/README.md`); its documents follow the `brief-v1.md` shape: a `**Status:**` header, numbered sections, MUST/SHOULD language. A JSON schema per document lives in `schemas/` (`brief-v1.json`, `brief-v2.json` today). (Read 2026-09-16 @ d96fd3ba.)
- The roles a node may bind are the four in `topology.yaml` `apps:`: `desk`, `reviewer`, `verifier`, `worker`. The file carries no ids (operator configuration) and states that a listed role is KNOWN, not BOUND — binding is the roster's job. A pattern therefore names roles, never logins or ids.
- `docs/enforcement-model.md` names the one load-bearing separation: the identity that writes a change and the identity that certifies it must differ. The implementation pattern encodes it as two nodes with different roles, and the lint refuses a pattern whose review node shares the implementer's role.
- `statusgen` is offline by default and ships as a pinned binary run from an arbitrary directory, so the pattern lint reads `topology.yaml`'s roles through the compiled derivation (`statusgen/topologyvalues.go`), the same way every other topology consumer does.
- Vocabulary (binding on every later brief, from the stream README): node kinds `artifact | check | decision | effect`; evidence kinds `command | review | witness | observe`; evidence results `pass | fail | missing | error | could-not-check | wrong-revision`; risk-class verdicts `low | standard | elevated | human`.
- Single-point-of-failure note: the ONE control is the `pattern-effect-exceeds-role` lint on the pattern file. Second layer, independent: at instantiation (graph-execution/05's harness) an instance is validated against its pattern — an instance cannot carry an effect its pattern lacks, so an edited instance fails in a different component (the harness) for a different reason (instance ≠ pattern). Third, out-of-band: the roster's role binding is what actually authorises a write on the forge; a node naming `worker` never mints a token, so a wrong pattern cannot itself act.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Public tree: `example-org/*` placeholders everywhere; the pattern files describe roles and artifacts, never a deployment.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **The schema document** `spec/workflow-pattern-v1.md` (planned) (+ `schemas/workflow-pattern-v1.json` (planned)). A pattern is:
   ```yaml
   schema: workflow-pattern-v1
   pattern: implementation          # name
   version: 1                        # bumped on every node/edge/evidence edit after first review
   risk-input:                       # risk-class verdict → mandatory gate nodes
     low:      [review]
     standard: [review, verify]
     elevated: [review, security-review, verify]
     human:    [review, security-review, verify, human-signoff]
   nodes:
     - id: implement
       kind: artifact                # artifact | check | decision | effect
       role: worker                  # a topology.yaml apps: role name
       inputs:  [{artifact: brief, revision: brief.version}]
       outputs: [branch, draft-pr]
       evidence: [{kind: command, claim: "Verify rows pass", mandatory: true}]
       effects:  [{kind: push, target: branch}, {kind: pr-open, target: pr}]
       budget:   {attempts: 2}
       outcomes: {wait: [review-pending], fail: [needs-context, blocked]}
   edges: [{from: implement, to: review}, ...]
   join: verify                      # the node that carries the integration check
   ```
   Normative rules (MUST): every node has exactly one `kind` and one `role`; a node with `effects` is `kind: effect` or declares each effect's `target` among its own `outputs`; a `review` node's role differs from every node that produced its inputs; `join` names a `check` node; `risk-input` maps every one of the four verdicts; nodes sit at durable boundaries only (an artifact, an independent check, a human decision, an external effect) — a step that produces none of these is not a node.
2. **Effect permissions.** A table in the schema doc, `role → permitted effect kinds`, derived from what each `topology.yaml` role does today (`worker`: push, pr-open, comment; `reviewer`: review, comment; `verifier`: evidence-commit, comment; `desk`: file-issue, comment, dispatch). The lint rule `pattern-effect-exceeds-role` refuses a node whose effect kind is not permitted for its role. **A generated instance carries no permission the pattern lacks** — the schema states it and 05's harness enforces it.
3. **The two patterns.** `spec/workflow-patterns/implementation-v1.yaml` (planned): implement (worker, artifact) → review (reviewer, check; `evidence: review`) → merge (desk, effect; gated by `risk-input`) → verify (verifier, check; `evidence: witness`; the join with the integration check) → close (desk, decision). `research-v1.yaml`: collect-sources (worker, artifact) → resolve-gaps (worker, artifact) → synthesise (worker, artifact) → review-artifact (reviewer, check; the join). Each node's `evidence` names the claim in words a verifier can check.
4. **The lint.** `statusgen patterns --lint [--root <root>]` validates every `spec/workflow-patterns/*.yaml` against the JSON schema and the MUST rules; exit 0 clean, 1 on any PROBLEM, 2 could-not-check (unreadable file). Fixtures: a good copy of each pattern; `bad-effect-exceeds-role.yaml` (a reviewer node with `push`); `bad-review-same-role.yaml`; `bad-join-not-check.yaml`; `bad-risk-input-missing-verdict.yaml`. Register the rules so `statusgen enforcement-status` lists them.
5. **Docs.** `spec/README.md` table row; `docs/lifecycle.md` §Review gates gains one sentence naming which pattern node each gate is. `changelog/graph-execution-02-pattern-schema.md` (planned).

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd statusgen && go test -run TestPatterns ./...` | exit 0 |
| 2 | check +flow | `statusgen patterns --lint --root .; echo rc=$?` | `rc=0` over the two shipped patterns |
| 3 | check:ci +mutation | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | exit 0; the fixture with a `reviewer` node declaring `{kind: push}` makes the lint exit 1 naming `pattern-effect-exceeds-role` |
| 4 | check:ci +mutation | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | exit 0; a review node whose role equals its input producer's role is refused — the implementer↔reviewer separation is machine-checked in the pattern |
| 5 | check +dereference | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | prints `ok` — both patterns validate against the schema with an independent validator, not only the Go lint |
| 6 | check +dereference | `grep -c 'workflow-pattern-v1' spec/README.md` | count >= 1 |
| 7 | check:ci +neighbour | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | exit 0 — the role names the lint reads still match `topology.yaml` |
| 8 | check | `statusgen --root . --lint; echo rc=$?` | `rc=0` |
| 10 | check | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | `rc=0` on the authoring branch (every entry routes follow-up or out-of-scope); exit 2 (could-not-check) on a fully merged main is acceptable and must be recorded as such, never as pass |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->
| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | 0 | `ok  	github.com/medici-finance/assay/statusgen` | 2026-09-17 | implementer |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | 0 | `patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned)` / `rc=0` | 2026-09-17 | implementer |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | 0 | `ok  	github.com/medici-finance/assay/statusgen` (fail-first: with `checkPatternMustRules` stubbed to return no violations, this test failed with `want checked-failed, got state=0 violations=[]`) | 2026-09-17 | implementer |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | 0 | `ok  	github.com/medici-finance/assay/statusgen` (fail-first: same stub, failed with `want checked-failed, got state=0 violations=[]`) | 2026-09-17 | implementer |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | 0 | `ok` | 2026-09-17 | implementer |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | 0 | `1` | 2026-09-17 | implementer |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | 0 | `ok  	github.com/medici-finance/assay/statusgen` | 2026-09-17 | implementer |
| 8 | `statusgen --root . --lint; echo rc=$?` | 0 | `LINT: PASS` / `rc=0` | 2026-09-17 | implementer |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | 0 | `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing` / `rc=0` (authoring branch, as expected) | 2026-09-17 | implementer |

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer questions: (1) can any node in either shipped pattern act with an authority the
role it names does not hold today? (2) is every node at a durable boundary, or has a shell
step been promoted to a node?
