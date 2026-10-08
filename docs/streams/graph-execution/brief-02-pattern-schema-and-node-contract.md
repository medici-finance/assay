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
unblocks: ["graph-execution/03", "graph-execution/04", "graph-execution/05", "graph-execution/08", "iso-9001/09"]
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
4. **The lint.** `statusgen patterns --lint [--root <root>]` validates every `spec/workflow-patterns/*.yaml` against the JSON schema and the MUST rules; exit 0 clean, 1 on any PROBLEM, 2 could-not-check (unreadable file). Fixtures: a good copy of each pattern; `bad-effect-target-not-owned.yaml` (a non-`effect`-kind node whose effect `target` isn't among its own `outputs`); `bad-effect-exceeds-role.yaml` (a reviewer node with `push`); `bad-review-same-role.yaml`; `bad-join-not-check.yaml`; `bad-risk-input-missing-verdict.yaml`. Register the rules so `statusgen enforcement-status` lists them.
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

### Non-implementer verifier run — VERIFY: BLOCKED — 4/9 pass, 5 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

Runner is not the implementer. Isolated detached worktree cut off `origin/main` at the merged head (HEAD == origin/main == `7e8e79ed43f30fcf43dc2fdbf735606cce2a8323`), offline envelope (`KUBECONFIG=/dev/null`), read-only. `gate: model`, all four risk answers `no`. Rows 1/3/4/7 are `check:ci` rows whose hermetic `statusgen verifyrun` witness could-not-run on darwin (it needs a Linux `unshare --net` network-off sandbox); the non-hermetic direct run passed for each, recorded as supporting-only. Row 10 is the sanctioned post-merge consumers could-not-check.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd statusgen && go test -v -run TestPatterns ./...` | exit 0 | COULD-NOT-CHECK — hermetic witness owed (darwin); statusgen verifyrun reports row 1 could-not-run (check:ci needs a network-off `unshare --net` sandbox, a Linux facility; this host is darwin). Direct non-hermetic run (supporting only): exit 0, 11 subtests all `--- PASS`, no SKIP, `ok github.com/medici-finance/assay/statusgen`. | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | rc=0 over the two shipped patterns | PASS — exit 0: `patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned)` / `rc=0` | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | `cd statusgen && go test -v -run 'TestPatterns.*ExceedsRole' ./...` | exit 0; the reviewer-node-with-push fixture makes the lint exit 1 naming pattern-effect-exceeds-role | COULD-NOT-CHECK — hermetic witness owed (darwin); verifyrun reports row 3 could-not-run (Linux `unshare --net` needed; darwin host). Direct non-hermetic run (supporting only): exit 0, one `--- PASS` line for the effect-exceeds-role test the brief's row 3 names, no SKIP. The regex form selects that same single test (its literal name is a 32+ character run, so it is described here rather than quoted). `go test -list 'TestPatterns.*ExceedsRole'` selects exactly one test, the brief's row 3 test (the literal name is a 32+ character run). | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | `cd statusgen && go test -v -run 'TestPatterns.*ReviewSameRole' ./...` | exit 0; a review node whose role equals its input producer's role is refused (implementer/reviewer separation machine-checked) | COULD-NOT-CHECK — hermetic witness owed (darwin); verifyrun reports row 4 could-not-run (Linux `unshare --net` needed; darwin host). Direct non-hermetic run (supporting only): exit 0, one `--- PASS` line for the review-same-role test the brief's row 4 names, no SKIP. The regex form selects that same single test (its literal name is a 32+ character run, described here rather than quoted). `go test -list 'TestPatterns.*ReviewSameRole'` selects exactly one test, the brief's row 4 test (the literal name is a 32+ character run). | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | prints ok (both patterns validate under an independent validator) | PASS — exit 0: `ok` | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | count >= 1 | PASS — exit 0: `1` | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | `cd statusgen && go test -v -run TestTopologyValuesMatchSource ./...` | exit 0 — the role names the lint reads still match topology.yaml | COULD-NOT-CHECK — hermetic witness owed (darwin); verifyrun reports row 7 could-not-run (Linux `unshare --net` needed; darwin host). Direct non-hermetic run (supporting only): exit 0, `--- PASS: TestTopologyValuesMatchSource`, no SKIP. | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | `statusgen --root . --lint; echo rc=$?` | rc=0 | PASS — exit 0: `LINT: PASS` / `rc=0`; 0 lines beginning PROBLEM; no NOTICE names this brief. | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 (could-not-check) on a fully merged main is acceptable and must be recorded as such, never as pass | COULD-NOT-CHECK — rc=2 on clean merged main: `COULD-NOT-CHECK: assay:assay:graph-execution:02 is not in the diff against 7e8e79ed43f3, so this run carries no evidence about its claims`. Acceptable per this row's own Expect cell; recorded as could-not-check, not pass. | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE: DERIVED — role-to-permitted-effect-kinds table (worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch) @ spec/workflow-pattern-v1.md:111-114 — derived from the four topology.yaml apps: roles (desk, reviewer, verifier, worker; confirmed present in topology.yaml) and each role's actual forge behaviour under the enforcement model (worker implements: pushes / opens PRs / comments; reviewer: reviews / comments; verifier: commits evidence / comments; desk: files issues / comments / dispatches the merge). It matches Task 2's specification exactly. It is the item's declared single point of failure, but reversible: a wrong entry is fixed by a spec+lint edit, and per the brief's own third layer a node naming a role never mints a token, so a wrong permission cannot itself act on the forge. Every effect in both shipped patterns is within its node's role permission (implement/worker: push+pr-open; review/reviewer: comment; merge/desk: dispatch; verify/verifier: comment; close/desk: no effect) — machine-corroborated by rows 2 and 8 (lint clean) and by the pattern role-effect-permissions-covers-topology-roles test.

RISK-VALUE: reversible knobs (ranked last, no derivation required) — budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31 (also research-v1.yaml:26, 37, 50): an attempt/retry count, reversible operational knob. risk-input verdict-to-gate-node maps @ implementation-v1.yaml:14-18 and research-v1.yaml:12-16: policy mappings of which gate nodes each of the four verdicts requires (not numeric literals), reversible.

No implementation defect: rows 1/3/4/7 need a Linux hermetic witness (all subtests pass non-hermetically, no SKIP); row 10 is the sanctioned post-merge could-not-check.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | pass exit=0 | sha256:e86054a2ccc7 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | pass exit=0 | sha256:dc51b8c96c2d | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:6c50dfe73543 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | fail exit=0 | sha256:ee9008be1b51 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier re-run — VERIFY: BLOCKED — 4/9 witnessed pass, 5 could-not-check, 0 real fail — 2026-09-27 claude-opus-5.5-verifier

The witness table directly above is this run's (statusgen verifyrun v1.0.27, write mode, clean tree, HEAD == origin/main == b227b40768db08a0a91046899bc1877cf3c6d1ec). Runner is not the implementer. Offline envelope (KUBECONFIG=/dev/null), read-only. gate: model; all four risk answers no.

Since the 2026-09-23 run: every hashed input (this brief, statusgen/patterns.go, the spec, both pattern files, the JSON schema, spec/README.md, topology.yaml) is byte-identical at this head; only the witness tool moved (v1.0.26 to v1.0.27). The blocker class is unchanged.

Per-row key output (direct runs at this head, same commands as the Verify table):

- Row 1: witness could-not-run (check:ci needs the Linux unshare --net sandbox; host is darwin). Direct host run exit 0, all 11 pattern tests PASS, no SKIP. Supporting Linux network-off run (golang 1.25 container, --network none, read-only module cache, GOPROXY=off): exit 0.
- Row 2: witnessed pass. patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned) / rc=0.
- Row 3: witness could-not-run (same darwin reason). Direct host run exit 0, the effect-exceeds-role test PASS; Linux network-off container run exit 0. Independent mutation: the reviewer-with-push fixture copied alone into a scratch root makes patterns --lint exit 1 naming pattern-effect-exceeds-role; a copy of the SHIPPED implementation pattern with the review node's comment effect changed to push also exits 1 with the same rule.
- Row 4: witness could-not-run (same darwin reason). Direct host run exit 0, the review-same-role test PASS; Linux network-off container run exit 0. Independent mutation: the review-same-role fixture alone exits 1 naming pattern-review-same-role (review node role worker, same as implement).
- Row 5: witnessed pass. ok (independent jsonschema validator over both shipped patterns).
- Row 6: witnessed pass. 1.
- Row 7: witness could-not-run (same darwin reason). Direct host run exit 0, TestTopologyValuesMatchSource PASS; Linux network-off container run exit 0.
- Row 8: witnessed pass. LINT: PASS / rc=0; 0 PROBLEM lines; no NOTICE names this brief.
- Row 10: COULD-NOT-CHECK as written, sanctioned by the row's own Expect. statusgen itself exited 2: COULD-NOT-CHECK: assay:assay:graph-execution:02 is not in the diff against b227b40768db, so this run carries no evidence about its claims. The witness cell reads fail exit=0 because (a) the trailing echo rc=$? makes the shell exit 0 whatever statusgen returns, and (b) the witness expect-parser latched "exit 2" from the merged-main clause of the Expect prose — a check-definition artifact, not an observed failure. Sanctioned recipe (checkout of the squash merge 5b384e52c, --base its parent 060980aa0f1a): exit 0, summary 3 corroborated, 0 disproved, 1 unchecked. The UNCHECKED entry is the out-of-scope desk-skill claim; the merge diff touches none of the worker-desk, pr-review-desk or verify-desk SKILL.md files (its only SKILL.md change is the author-brief enforcement-registry mirror), so the out-of-scope routing holds.

Deliverable grounding at this head: spec/workflow-pattern-v1.md, schemas/workflow-pattern-v1.json, both pattern files, statusgen/patterns.go + patterns_test.go, all seven fixtures (two good, five bad), the patterns subcommand in statusgen/main.go, the docs/lifecycle.md §Review gates sentence, and the spec/README.md row are present; the changelog fragment was aggregated into CHANGELOG.md at the v1.0.13 release; statusgen enforcement-status lists all five pattern-* rules as fatal.

Risk-bearing value enumeration (literals the diff introduces):

1. patternRoleEffectPermissions: worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 (mirrored at spec/workflow-pattern-v1.md:111-114).
2. topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78 (source topology.yaml:237-249).
3. patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100.
4. implementation risk-input: low = [review]; standard = [review, verify]; elevated = [review, verify]; human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:15-18. research risk-input: all four = [review-artifact] @ spec/workflow-patterns/research-v1.yaml:13-16.
5. patterns exit codes: clean = 0, failed = 1, could-not-check = 2, usage = 2 @ statusgen/patterns.go:67-70.
6. budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31 and research-v1.yaml:26, 37, 50.

Ranked by irreversibility: none is irreversible (nothing executes a pattern yet; every value is fixed by a spec/pattern edit and a version bump). Highest consequence first: 1 (the declared single point of failure for permission safety), 4 (which gates a risk class makes mandatory), 2 and 3 (vocabulary the lint pins), 5 (instrument contract), 6 (retry knob, last, no derivation owed).

RISK-VALUE: DERIVED — patternRoleEffectPermissions (reviewer = review, comment) @ statusgen/patterns.go:92-95 — it equals Task 2's table exactly and each role's present forge behaviour: only worker may push or open a PR, reviewer holds no write effect beyond review/comment, verifier's only write is the Evidence commit, desk files, comments and dispatches. No role holds another role's certifying effect, which is the separation docs/enforcement-model.md requires; the table-covers-topology-roles test and the shipped-pattern push mutation above confirm the lint reads this table.

RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78 — equals the four apps: roles in topology.yaml (the compiled-derivation convention; row 7 pins them together).

RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — the fixed risk-class vocabulary this stream's README binds on every later brief; the missing-verdict fixture exits 1.

RISK-VALUE: DERIVED — patterns exit codes 0 / 1 / 2 @ statusgen/patterns.go:67-70 — Task 4's stated contract (0 clean, 1 PROBLEM, 2 could-not-check) and the same three-state numbers as the binary's other lints; observed rc=0 clean and rc=1 on every bad fixture.

RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify] and human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — OPEN QUESTION. Task 1's schema example maps elevated to review + security-review + verify and human to review + security-review + verify + human-signoff; Task 3's fixed node list has no security-review or human-signoff node, so the shipped pattern could not name them and elevated/human collapse to the standard gate set. As shipped, a human-class instance of this pattern declares no human node among its mandatory gates. The brief does not say whether that collapse is intended; deriving it needs a design ruling (add the gate nodes to the pattern, or state that human gating stays outside the pattern). Reversible: no component consumes risk-input yet (graph-execution/05 is todo). Related observation: the lint does not check that risk-input names existing node ids — a copy of the shipped pattern with human = [human-signoff] (no such node) lints clean, rc=0. Not a MUST rule this brief states, so not a failure of this item.

RISK-VALUE: reversible knob (ranked last, no derivation owed) — budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31 (also research-v1.yaml:26, 37, 50).

No implementation defect found. Rows 1/3/4/7 owe the hermetic Linux witness (all pass directly and in a Linux network-off container, which is supporting evidence only, not the certified witness); row 10 is the sanctioned post-merge could-not-check, with the sanctioned recipe corroborating 3 of 4 entries and the fourth verified by diff inspection.

VERIFY: BLOCKED
### Verification — 2026-09-30 (assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main b0088804294b8b68ad8d06f341e6f0fd9dd2637d, gate: model, all four risk answers no. First table: the `statusgen verifyrun --dry-run` execution witness, landed verbatim; it ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, `unshare --net` available), statusgen built in-container from a clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
| 1 | `cd statusgen && go test -run TestPatterns ./...` | pass exit=0 | sha256:8a2302a2bbe5 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | pass exit=0 | sha256:e86054a2ccc7 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | pass exit=0 | sha256:900b07966263 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | pass exit=0 | sha256:b76956e68534 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | fail exit=1 | sha256:520780e9cfcf | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | pass exit=0 | sha256:a5a2b90f5203 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:63e796d12fe1 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | fail exit=0 | sha256:c50d535401a7 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | exit 0 | exit 0, ok github.com/medici-finance/assay/statusgen; -v run shows 11 --- PASS lines (all TestPatterns tests), none failed, none omitted | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | rc=0 over the two shipped patterns | patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned) / rc=0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | exit 0; reviewer node with push makes lint exit 1 naming pattern-effect-exceeds-role | exit 0, --- PASS for the named test. Independent: the reviewer-with-push fixture alone in a scratch root makes patterns --lint exit 1 with [pattern-effect-exceeds-role] node review has role reviewer, not permitted effect kind push; the SHIPPED implementation pattern with its review node's comment effect mutated to push also exits 1 with the same rule | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | exit 0; review node sharing its input producer's role is refused | exit 0, --- PASS for the named test. Independent: the review-same-role fixture alone exits 1 [pattern-review-same-role]; the SHIPPED implementation pattern with review role mutated to worker exits 1 naming implement as producer of draft-pr | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | prints ok | exit 0, ok (jsonschema 4.26.0) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | count >= 1 | exit 0, 1 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | exit 0 | exit 0, --- PASS: TestTopologyValuesMatchSource | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 8 | `statusgen --root . --lint; echo rc=$?` | rc=0 | LINT: PASS / rc=0; 0 PROBLEM lines; NOTICE gotest-run-vacuous names this brief's rows 1, 3, 4, 7 (no PASS-line assertion, unanchored selector) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 on merged main acceptable, recorded as could-not-check | COULD-NOT-CHECK as written: statusgen exited 2, not in the diff against b0088804; shell printed rc=2 and exited 0. Sanctioned recipe (squash merge 5b384e52c checked out, base 060980aa0): exit 0, 3 corroborated, 0 disproved, 1 unchecked; the unchecked entry is the out-of-scope desk-skill claim and the merge diff touches only the author-brief SKILL.md, none of worker-desk, pr-review-desk or verify-desk | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — patternRoleEffectPermissions worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 (mirrored at spec/workflow-pattern-v1.md:111-114) — equals Task 2's table exactly and each role's present forge behaviour; no role holds another role's certifying effect (the enforcement-model separation). The lint demonstrably reads this table: a push effect on the shipped reviewer node exits 1. Declared single point of failure, reversible (spec + lint edit), and a node naming a role mints no token.
RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78 — equals the four topology.yaml apps: roles; row 7 pins the two together.
RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — the stream's fixed risk-class vocabulary; the missing-verdict fixture exits 1.
RISK-VALUE: DERIVED — patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70 — Task 4's contract; observed rc=0 clean, rc=1 on every bad fixture, rc=2 on an unreadable (mode 000) pattern file.
RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 (research all four = [review-artifact] @ research-v1.yaml:13-16) — Task 1's example maps elevated and human to security-review and human-signoff gates, but Task 3's fixed node list has neither node, so both collapse to the standard set and a human-class instance declares no human node. The brief does not say whether the collapse is intended; deriving it needs a design ruling. Unchanged since the 2026-09-27 pass; reversible, no consumer reads risk-input yet.
RISK-VALUE: reversible knob, ranked last, no derivation owed — budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31 and research-v1.yaml:26, 37, 50.

Notes:
- BLOCKED (check-definition), not a product failure. Rows 1-8 pass by hand; the Linux witness passes rows 1-4 and 6-8, and its row 5 fail is environment only (no PyYAML in the image; the darwin witness passes it). Row 10 fails as authored (the trailing echo rc makes the shell exit 0) and passes by the hand procedure, tracked with the other row re-authors at #1927. `statusgen brief --check-verified` with a hypothetical flip exits 1. The NAMED, NOT DERIVED risk-input value is a design question not yet routed; filing is pending.
- Enumeration covered the squash-merge diff 060980aa0..5b384e52c (24 files) as it stands at b0088804; patterns.go, both pattern files, the spec and the schema are unchanged since 5b384e52c. None of the values is irreversible.
- Witness host: Linux container (rows 1, 3, 4, 7 are check:ci and need the network-off sandbox; the darwin witness records them could-not-run). Linux witness: rows 1, 2, 3, 4, 6, 7, 8 pass.
- Linux witness row 5 fail exit=1 is environment only: the golang bookworm image has no PyYAML (ModuleNotFoundError: No module named 'yaml'). The darwin witness at the same SHA records row 5 as pass exit=0, sha256:dc51b8c96c2d, and the hand run printed ok. Rows 2 and 6 hash identically on both hosts (e86054a2ccc7, 4355a46b19d3).
- Row 10 is a check-definition failure: the witness records fail exit=0 on both hosts because the trailing echo rc makes the shell exit 0 whatever statusgen returns, while the witness expect-parser takes exit 2 from the merged-main clause of the Expect prose. The substance passes by the sanctioned hand procedure (3 corroborated, 0 disproved, the fourth entry verified out-of-scope by diff inspection). Fails as authored, passes by hand: BLOCKED, not PASS. Fix is a Verify-row re-author (drop the echo, or state the merged-main recipe in the command).
- Rows 1, 3, 4, 7 also carry a lint NOTICE (gotest-run-vacuous): as authored they cannot tell a real pass from a no-tests-to-run pass. Every named test was confirmed by its --- PASS line by hand and in the container, so no vacuous pass here, but the rows should be re-authored with an anchored selector and a PASS-line assertion.
- statusgen brief --check-verified on a throwaway no-hardlinks clone with the README row flipped to verified: exit 1 — rows 1, 3, 4, 7 could-not-run (the landed darwin witness) and row 10 fail. Even with a Linux witness landed, row 10 as authored keeps the gate closed.
- Row 10 on this main printed exit 2 from statusgen, which its own Expect sanctions as could-not-check; recorded as could-not-check, never as pass.
- Previous passes (2026-09-23, 2026-09-27) found no implementation defect; this pass agrees. The blockers are check-definition (row 10, and the vacuous-selector rows) plus the open risk-input design question, which needs a human ruling.

VERIFY: BLOCKED
### Correction: 2026-09-30, assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian). VERIFY: BLOCKED

- The first execution-witness table in the re-run above was transcribed without its header separator line, so a Markdown parser reads the header as a data row and shifts every witness row down by one. The same table is restated below, verbatim and unchanged except for the separator line. The verdict is unchanged.

**Execution witness** (restated):
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | pass exit=0 | sha256:8a2302a2bbe5 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | pass exit=0 | sha256:e86054a2ccc7 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | pass exit=0 | sha256:900b07966263 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | pass exit=0 | sha256:b76956e68534 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | fail exit=1 | sha256:520780e9cfcf | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | pass exit=0 | sha256:a5a2b90f5203 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:63e796d12fe1 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | fail exit=0 | sha256:c50d535401a7 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |

### Non-implementer verifier re-run: 2026-10-02T22:07:55Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main cf31c32418ba49f93c679913813768542db1c072

Runner is not the implementer. Detached worktree pinned to the merged head above, offline envelope (KUBECONFIG=/dev/null), read-only; gate: model, all four risk answers no. Host: darwin/arm64, go1.27.1, statusgen v1.0.31 (rows 2, 8 and 10 were repeated with a statusgen built from this SHA: identical results). The go test rows ran with the system temp directory and HOME redirected into the runner's own scratch directory. Every mutation ran on a scratch copy outside the tree; no tracked file was edited.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | exit 0 | exit 0, ok github.com/medici-finance/assay/statusgen; a -count=1 -v repeat shows 11 --- PASS lines (every TestPatterns test), no FAIL | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | rc=0 over the two shipped patterns | exit 0, patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned) / rc=0 | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | exit 0; the fixture with a reviewer node declaring push makes the lint exit 1 naming pattern-effect-exceeds-role | exit 0, --- PASS: TestPatternsEffectExceedsRoleIsProblem. Independent mutation on scratch copies: the reviewer-with-push fixture alone gives rc=1, [pattern-effect-exceeds-role]: node "review" has role "reviewer", which is not permitted to perform effect kind "push"; the SHIPPED implementation pattern with the review node's comment effect changed to push gives rc=1 with the same rule | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | exit 0; a review node whose role equals its input producer's role is refused | exit 0, --- PASS: TestPatternsReviewSameRoleIsProblem. Independent mutation on scratch copies: the review-same-role fixture alone gives rc=1, [pattern-review-same-role]: review node "review" has role "worker", the same role as "implement"; the SHIPPED implementation pattern with the review role changed to worker gives rc=1 naming implement as producer of draft-pr | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | prints ok | exit 0, ok (jsonschema 4.26.0, PyYAML 6.0.3) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | count >= 1 | exit 0, 1 | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | exit 0 | exit 0, --- PASS: TestTopologyValuesMatchSource | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `statusgen --root . --lint; echo rc=$?` | rc=0 | exit 0, LINT: PASS / rc=0; 0 PROBLEM lines; NOTICE gotest-run-vacuous still names this brief's rows 1, 3, 4, 7 | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 (could-not-check) on a fully merged main is acceptable and must be recorded as such, never as pass | COULD-NOT-CHECK as written: statusgen printed COULD-NOT-CHECK: assay:assay:graph-execution:02 is not in the diff against cf31c32418ba, rc=2 (the shell itself exits 0 because of the trailing echo). Sanctioned recipe (squash merge 5b384e52c checked out, --base 060980aa0f1a): exit 0, summary: 3 corroborated, 0 disproved, 1 unchecked; the unchecked entry is the out-of-scope desk-skill claim | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (`statusgen verifyrun --brief <this brief> --dry-run`, statusgen v1.0.31, darwin host, nothing written): exit 2; 4 of 9 rows witnessed pass, 4 could-not-run, 1 fail.

- Row 1, 3, 4, 7 (check:ci): could-not-run, exit=-, sha256:e3b0c44298fc. Reason printed: check:ci hermetic execution requires a network-off sandbox, unavailable on this host (unshare --net is a Linux facility, this host is darwin).
- Row 2: pass exit=0, sha256:e86054a2ccc7 (identical to the 2026-09-30 hash).
- Row 5: pass exit=0, sha256:dc51b8c96c2d (identical to the earlier darwin hash).
- Row 6: pass exit=0, sha256:4355a46b19d3 (identical).
- Row 8: pass exit=0, sha256:5993e2a6183d.
- Row 10: fail exit=0, sha256:9de7839b88a2, "exit 0, expected 2".

Findings:

- The verdict has not changed. No implementation defect: rows 1-8 pass by hand and row 10's substance passes by the sanctioned merge-diff recipe. The gate stays closed for the same two reasons as on 2026-09-30.
- Row 10 (check-definition, the durable blocker): as authored it cannot be witnessed as pass on merged main. statusgen exits 2, the trailing echo makes the shell exit 0, and the witness expect-parser takes "exit 2" from the merged-main clause of the Expect prose, so the witness records fail exit=0 on every host. The row's own Expect also forbids recording the exit-2 case as a pass. It clears only by re-authoring the row (drop the echo, or state the merge-diff recipe in the command). The post-merge consumers class is tracked at #1281 (open). The re-author tracker #1927 (open) does not name this brief; no issue specific to this row was found.
- Rows 1, 3, 4, 7 (environment): unchanged on this darwin host, tracked at #1800 (open). The 2026-09-30 Linux network-off witness recorded all four as pass, and the hand runs pass here, so these rows are held by the host, not by the code. They also still carry the gotest-run-vacuous NOTICE (unanchored selector, no PASS-line assertion); each named test was confirmed by its --- PASS line.
- Inputs changed since the last recorded outcome: #1682 (graph-execution/03, the evidence coverage rule and the observe evidence kind) added to statusgen/patterns.go, spec/workflow-pattern-v1.md and schemas/workflow-pattern-v1.json; docs/lifecycle.md, docs/enforcement-model.md and statusgen/main.go also grew. None of it changes a row result: both shipped patterns still lint clean and still validate against the amended schema with the independent validator, and all 11 TestPatterns tests pass. The only effect on this brief's values is a line shift in the spec's permission table (now lines 155-158).
- The risk-input design question is routed at #1933 (open); no ruling is recorded there.

Risk-bearing value enumeration (literals this brief's diff introduced, re-read at this head; the #1682 additions belong to graph-execution/03 and are out of this enumeration):

1. patternRoleEffectPermissions: worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 (mirrored at spec/workflow-pattern-v1.md:155-158).
2. implementation risk-input: low = [review]; standard = [review, verify]; elevated = [review, verify]; human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:15-18. research risk-input: all four = [review-artifact] @ spec/workflow-patterns/research-v1.yaml:13-16.
3. topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78.
4. patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100.
5. patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70.
6. budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31 and research-v1.yaml:26, 37, 50.

Ranked by irreversibility: none is irreversible (each is undone by a spec, pattern or lint edit and a version bump; a node naming a role mints no token). Highest consequence first: 1 (the declared single point of failure for permission safety), 2 (which gates a risk class makes mandatory), 3 and 4 (vocabulary the lint pins), 5 (instrument contract), 6 (retry knob, last, no derivation owed).

RISK-VALUE: DERIVED — patternRoleEffectPermissions worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 — equals Task 2's table exactly and each role's present forge behaviour; no role holds another role's certifying effect, which is the separation the enforcement model requires. The lint demonstrably reads this table: a push effect on the shipped reviewer node exits 1 at this head.
RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — Task 1's example maps elevated and human to security-review and human-signoff gates, but Task 3's fixed node list has neither node, so both collapse onto the standard set and a human-class instance declares no human node. Deriving it needs a design ruling; the question is open at #1933. Unchanged since 2026-09-27.
RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78 — equals the four apps: roles in topology.yaml; row 7 pins the two together and passed.
RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — the stream's fixed risk-class vocabulary (brief Context, vocabulary fact); the missing-verdict test passed.
RISK-VALUE: DERIVED — patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70 — Task 4's stated contract; observed rc=0 on the shipped patterns and rc=1 on each mutated copy in this pass (the rc=2 arm was not exercised by hand in this pass; the could-not-check-on-missing-root test passed).

VERIFY: BLOCKED — rows 1-8 pass by hand (8/9); row 10 is could-not-check as authored and fails the witness by check definition (needs a Verify-row re-author; class #1281); check:ci rows 1, 3, 4, 7 could-not-run in the witness on this darwin host (#1800). Status stays implemented.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Runner: assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), not the implementer. Merged main fe12ee0c90b9e39e78afc291e5b17111844356cb, detached worktree pinned to that head, offline envelope (KUBECONFIG=/dev/null), read-only. gate: model, all four risk answers no. Hand runs: darwin/arm64, go1.27.1, statusgen v1.0.31; rows 2, 8 and 10 repeated with a statusgen built from fe12ee0c9 (identical results). The go test rows ran with GOPROXY=off, -count=1, -timeout 300s, and HOME and the temp directory redirected into the runner's scratch directory. Every mutation ran on a scratch copy outside the tree. No tracked file was edited, and the brief was restored after the darwin verifyrun wrote to it.

Why this re-run: the receipt from 2026-10-02 (blocked at cf31c3241, 8/9 rows, blocker #1281) went stale because three declared inputs changed. None of the changes touches the pattern schema or node contract. (1) The brief: frontmatter `unblocks:` gained iso-9001/09, and the 2026-10-02 Evidence block was added. Task, Verify table, Context and consumers are byte-identical. (2) spec/README.md gained one row for loop-admin-runner-v1.md (#2082, graph-execution/20). The workflow-pattern-v1 row is unchanged. (3) worker-desk SKILL.md changed its board-roots scope flag, the deskack syntax and the comms cutover reading (#2141, #2126, #2120). It has no pattern, node or effect content, and the brief's consumers entry marks it out-of-scope. No deliverable changed between cf31c3241 and fe12ee0c9: the diff over statusgen/patterns.go, statusgen/patterns_test.go, statusgen/testdata/patterns, spec/workflow-patterns, spec/workflow-pattern-v1.md, schemas/workflow-pattern-v1.json, statusgen/topologyvalues.go, topology.yaml and statusgen/main.go is empty.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | exit 0 | exit 0, ok github.com/medici-finance/assay/statusgen. A -v repeat shows 11 --- PASS lines (all TestPatterns tests: RoleEffectPermissionsCoversTopologyRoles, EmbeddedSchemaMatchesCommitted, GoodFixturesLintClean, ShippedPatternsLintClean, EffectExceedsRoleIsProblem, EffectTargetNotOwnedIsProblem, ReviewSameRoleIsProblem, JoinNotCheckIsProblem, RiskInputMissingVerdictIsProblem, LintReportsCouldNotCheckOnMissingRoot, NoVerbIsUsageRefusal) and 0 FAIL | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | rc=0 over the two shipped patterns | exit 0, patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned) / rc=0, with both the pinned v1.0.31 and the fe12ee0c9 build | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | exit 0; a reviewer node declaring push makes the lint exit 1 naming pattern-effect-exceeds-role | exit 0, --- PASS: TestPatternsEffectExceedsRoleIsProblem. Independent mutation on scratch copies: the bad-effect-exceeds-role fixture alone gives rc=1, [pattern-effect-exceeds-role]: node "review" has role "reviewer", which is not permitted to perform effect kind "push". The shipped implementation pattern with line 44's comment effect changed to push also gives rc=1 with the same rule | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | exit 0; a review node sharing its input producer's role is refused | exit 0, --- PASS: TestPatternsReviewSameRoleIsProblem. Independent mutation on scratch copies: the bad-review-same-role fixture alone gives rc=1, [pattern-review-same-role]: review node "review" has role "worker", the same role as "implement". The shipped implementation pattern with line 37's role changed to worker gives rc=1 naming implement as the producer of draft-pr | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | prints ok | exit 0, ok (darwin host; jsonschema 4.26.0, PyYAML 6.0.3). The Linux witness container records fail exit=1 for an environment reason: its python3 has no PyYAML module (ModuleNotFoundError: No module named 'yaml', reproduced separately), and nothing can be installed with the network off. This is not a schema result | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | count >= 1 | exit 0, 1 | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | exit 0 | exit 0, --- PASS: TestTopologyValuesMatchSource | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `statusgen --root . --lint; echo rc=$?` | rc=0 | exit 0, LINT: PASS / rc=0, 0 PROBLEM lines, with both the pinned and the fe12ee0c9 build. The NOTICE gotest-run-vacuous still names this brief's rows 1, 3, 4 and 7, and a NOTICE reports the one-sided depends edge graph-execution/09 to graph-execution/02 | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 (could-not-check) on a fully merged main is acceptable and must be recorded as such, never as pass | COULD-NOT-CHECK as written: statusgen printed COULD-NOT-CHECK: assay:assay:graph-execution:02 is not in the diff against fe12ee0c90b9…, rc=2. The shell exits 0 because of the trailing echo. Sanctioned merge-diff recipe, in a scratch clone at squash merge 5b384e52c (#1259) with --base 060980aa0f1a: exit 0, summary: 3 corroborated, 0 disproved, 1 unchecked. The unchecked entry is the out-of-scope desk-skill claim | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |

**Execution witness** (`statusgen verifyrun --brief`, Linux, network-off). The run used the cached image golang:1.26-bookworm (linux/arm64, go1.26.8, repo digest sha256:a688600ca24f…); no image was pulled. The container settings were: `--network none`, GOPROXY=off, GOSUMDB=off, GOTOOLCHAIN=local, the host module cache mounted read-only, the roster config mounted read-only, and a statusgen cross-built from fe12ee0c9. The working tree was a scratch clone checked out detached at fe12ee0c9, with its origin/main pinned to the same sha. **Disclosure:** the container ran with `--security-opt seccomp=unconfined`. statusgen's own `unshare --net --map-root-user` sandbox for check:ci rows needs it inside Docker. The desk allowed it for this throwaway, network-off container only. The same run on the darwin host could not run rows 1, 3, 4 and 7 (no `unshare --net`, #1800) and gave rows 2, 5, 6 and 8 pass and row 10 fail exit=0.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | pass exit=0 | sha256:cb8004503350 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | pass exit=0 | sha256:e86054a2ccc7 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | pass exit=0 | sha256:b76956e68534 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | pass exit=0 | sha256:a5a2b90f5203 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | fail exit=1 | sha256:520780e9cfcf | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | pass exit=0 | sha256:a5a2b90f5203 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 8 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:b4089ac9f4a6 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | fail exit=0 | sha256:a905d89210a7 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (git-config) |

`statusgen verifyrun --check` on this witness (same container): exit 1, 7 pass, 2 fail, 0 could-not-run/missing (of 9 Verify rows). On the darwin witness the result was exit 2, 4 pass, 1 fail, 4 could-not-run/missing (of 9 Verify rows).

Findings:

- No implementation defect. Rows 1-8 pass by hand (8/9). The substance of row 10 passes by the sanctioned merge-diff recipe. All four check:ci rows (1, 3, 4, 7) now carry a witnessed pass in a network-off Linux sandbox.
- Row 10 (check definition, the durable blocker) is unchanged. As written it cannot be witnessed as pass on merged main: statusgen exits 2, the trailing echo makes the shell exit 0, and the expect-parser takes "exit 2" from the merged-main clause. The row clears only by re-authoring it. The post-merge consumers class is tracked at #1281 (still OPEN at this run). The re-author tracker #1927 (open) still does not name this brief.
- Row 5 in the witness is an environment fault, not a code fault: the cached golang image has no PyYAML, so the import fails before any validation runs. The same command passes on the darwin host. It would clear on any witness host whose python3 carries PyYAML and jsonschema.
- The risk-input design question stays open at #1933, with no ruling recorded.

Risk-bearing value enumeration (literals this brief's diff introduced, re-read at fe12ee0c9; unchanged since cf31c3241):

1. patternRoleEffectPermissions: worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 (mirrored at spec/workflow-pattern-v1.md:155-158).
2. implementation risk-input: low = [review]; standard = [review, verify]; elevated = [review, verify]; human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:15-18. research risk-input: all four = [review-artifact] @ spec/workflow-patterns/research-v1.yaml:13-16.
3. topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78.
4. patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100.
5. patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70.
6. budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31 and research-v1.yaml:26, 37, 50.

Ranked by irreversibility: none is irreversible. Each is undone by a spec, pattern or lint edit plus a version bump, and a node naming a role mints no token. Highest consequence first: 1 (the declared single point of failure for permission safety), then 2 (which gates a risk class makes mandatory), then 3 and 4 (vocabulary the lint pins), then 5 (instrument contract), then 6 (retry knob, ranked last, no derivation owed).

RISK-VALUE: DERIVED — patternRoleEffectPermissions worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 — equals Task 2's table exactly and each role's present forge behaviour; no role holds another role's certifying effect, which is the separation the enforcement model requires. The lint reads this table: a push effect on the shipped reviewer node exits 1 at fe12ee0c9.
RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — Task 1's example maps elevated and human to security-review and human-signoff gates. Task 3's fixed node list has neither node, so both collapse onto the standard set, and a human-class instance declares no human node. Deriving the value needs a design ruling, and the question is open at #1933.
RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78 — equals the four apps: roles in topology.yaml (line 236 onward); row 7 pins the two together and passed.
RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — the stream's fixed risk-class vocabulary (the vocabulary fact in the brief's Context); the missing-verdict test passed.
RISK-VALUE: DERIVED — patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70 — Task 4's stated contract. Observed by hand in this pass: rc=0 on the shipped patterns, rc=1 on each mutated copy and bad fixture, and rc=2 on a root with no spec/workflow-patterns directory.

VERIFY: BLOCKED — rows 1-8 pass by hand (8/9). Row 10 is could-not-check as written and fails the witness by check definition; it needs a Verify-row re-author (class #1281, still open). The Linux network-off witness gives 7/9 pass; its row 5 fail is the container's missing PyYAML, not a code result. Status stays implemented.
### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-07, merged main 36113a1ddbb3492c213520d561edd09e2f7afca8

Runner: assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian). This runner is not the implementer. It used a detached worktree pinned to the merged head. Worktree HEAD, refs/remotes/origin/main and `git ls-remote origin main` all matched at the start. Offline envelope: KUBECONFIG=/dev/null, GOPROXY=off, GOSUMDB=off, GOTOOLCHAIN=local, and HOME, TMPDIR and GOCACHE redirected into the runner's scratch directory. The run was read-only. Host: darwin/arm64, go1.27.1, statusgen v1.0.32 (pinned desk-tools binary). Gate: model. All four risk answers are no, so the risk metadata is present and not absent. Every mutation ran on a scratch copy outside the tree, and no tracked file was edited. `statusgen verifyrun` wrote the brief, and the brief was then restored with `git checkout -- .`.

Why this re-run: 151 commits landed since the 2026-10-04 receipt at fe12ee0c90b9. Two hashed inputs changed:
- statusgen/main.go gained one help-text line for the graph-execution/03 `--coverage` flag. The `patterns` subcommand is unchanged.
- plugins/assay/skills/worker-desk/SKILL.md changed its advisory-comment and disposition wording (#2278). It has no pattern, node or effect content, and this brief's consumers entry marks it out-of-scope.

Every other hashed input is byte-identical to the 2026-10-04 receipt. That covers this brief, statusgen/patterns.go, statusgen/patterns_test.go, both pattern files, spec/workflow-pattern-v1.md, schemas/workflow-pattern-v1.json, spec/README.md, docs/lifecycle.md, docs/enforcement-model.md and statusgen/topologyvalues.go.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | pass exit=0 | ok github.com/medici-finance/assay/statusgen; the streamview package reports no tests to run. A supporting -count=1 -v repeat shows 11 --- PASS lines, every TestPatterns test, and 0 FAIL, so the selector is not vacuous | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | pass exit=0 | patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned) / rc=0 | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | pass exit=0 | ok github.com/medici-finance/assay/statusgen; the -count=1 -v repeat shows one --- PASS line for the named test. Independent mutation on scratch roots: the bad-effect-exceeds-role fixture alone gives rc=1 [pattern-effect-exceeds-role] node review has role reviewer, which is not permitted to perform effect kind push. The SHIPPED implementation pattern with line 44 changed from comment to push also gives rc=1 under the same rule | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | pass exit=0 | ok github.com/medici-finance/assay/statusgen; the -count=1 -v repeat shows one --- PASS line for the named test. Independent mutation on scratch roots: the bad-review-same-role fixture alone gives rc=1 [pattern-review-same-role] review node review has role worker, the same role as implement. The SHIPPED implementation pattern with line 37 changed from reviewer to worker gives rc=1, naming implement as the producer of draft-pr | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | pass exit=0 | ok (jsonschema 4.26.0, PyYAML 6.0.3) | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | pass exit=0 | 1 | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | pass exit=0 | ok github.com/medici-finance/assay/statusgen; the -count=1 -v repeat shows --- PASS: TestTopologyValuesMatchSource | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | LINT: PASS / rc=0; 0 lines begin with PROBLEM. One NOTICE names this brief: the one-sided depends edge from graph-execution/09 to graph-execution/02. No per-row gotest-run-vacuous NOTICE names this brief at this head | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | could-not-check exit=0 (statusgen rc=2) | COULD-NOT-CHECK as written: assay:assay:graph-execution:02 is not in the diff against 36113a1ddbb3, so this run carries no evidence about its claims; rc=2. The shell exits 0 because of the trailing echo. The row's own Expect sanctions this as could-not-check, never as pass. Sanctioned merge-diff recipe, run in a no-hardlinks scratch clone at squash merge 5b384e52c (#1259) with --base 060980aa0f1a: exit 0; summary: 3 corroborated, 0 disproved, 1 unchecked. The unchecked entry is the out-of-scope desk-skill claim. The 24-file merge diff touches only the author-brief SKILL.md, none of worker-desk, pr-review-desk or verify-desk | 2026-10-07 | assay-verifier-app[bot] @ 36113a1ddbb3 (on-behalf-of human:ian) (forge-identity) |

**Execution witness** (`statusgen verifyrun --brief`, write mode, darwin host, statusgen v1.0.32): exit 2. Rows 2, 5, 6 and 8 pass, with hashes e86054a2ccc7, dc51b8c96c2d, 4355a46b19d3 and 53bcbc423275. The first three match every earlier darwin witness. Rows 1, 3, 4 and 7 could-not-run: they are check:ci rows that need the Linux `unshare --net` network-off sandbox, and this host is darwin (#1800, open). Row 10 records fail exit=0 with the reason "exit 0, expected 2". The desk lands the witness table verbatim with its own runner and source stamp.

Findings:

- **No implementation defect.** Rows 1-8 pass by hand (8 of 9). The substance of row 10 passes by the sanctioned merge-diff recipe. The rows 3 and 4 mutations show that the lint refuses a reviewer push and a same-role review on the shipped pattern itself, not only on the fixtures.
- **Row 10 is the durable blocker, a check-definition problem.** It is unchanged since 2026-09-30. As written it cannot be witnessed as pass on merged main:
  - statusgen exits 2, and the trailing echo makes the shell exit 0.
  - The witness expect-parser takes "exit 2" from the merged-main clause, so it records a fail.
  - The row's own Expect forbids recording the exit-2 case as a pass.
  - It clears only when the Verify row is re-authored. The class is tracked at #1281, still OPEN at this run. The re-author tracker #1927 (open) still does not name this brief, and no issue specific to this row was found.
- **Rows 1, 3, 4 and 7 (environment):** the darwin witness could not run them (#1800, open). The 2026-10-04 Linux network-off witness at fe12ee0c9 recorded all four as pass, and nothing they read has changed since then.
- **The risk-input design question stays open at #1933.** That issue has no comments and no recorded ruling.
- **Deliverables are present at this head:**
  - the spec, the JSON schema and both pattern files
  - patterns.go and patterns_test.go
  - all seven fixtures: two good and five bad
  - the patterns subcommand in statusgen/main.go
  - the spec/README.md row and the docs/lifecycle.md §Review gates sentence

  The changelog fragment was aggregated into CHANGELOG.md at v1.0.13 (891a48b1).

Risk-bearing value enumeration. These are the literals this brief's diff introduced, re-read at 36113a1d and byte-unchanged since fe12ee0c9:

1. patternRoleEffectPermissions: worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95. Mirrored at spec/workflow-pattern-v1.md:155-158.
2. Implementation risk-input: low = [review]; standard = [review, verify]; elevated = [review, verify]; human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:15-18. Research risk-input: all four = [review-artifact] @ spec/workflow-patterns/research-v1.yaml:13-16.
3. topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:74-77. The source is topology.yaml apps:, from line 236.
4. patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100.
5. patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70.
6. budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31 and research-v1.yaml:26, 37, 50.

Ranked by irreversibility, none is irreversible. Each is undone by a spec, pattern or lint edit plus a version bump, and a node naming a role mints no token. Highest consequence first:
1. Entry 1, the declared single point of failure for permission safety.
2. Entry 2, which sets the gates a risk class makes mandatory.
3. Entries 3 and 4, the vocabulary the lint pins.
4. Entry 5, the instrument contract.
5. Entry 6, a retry knob. It ranks last and no derivation is owed.

RISK-VALUE: DERIVED — patternRoleEffectPermissions worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 — it equals Task 2's table exactly and matches each role's present forge behaviour. No role holds another role's certifying effect, which is the separation the enforcement model requires. The lint reads this table: at 36113a1d, a push effect on the shipped reviewer node exits 1.
RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — Task 1's example maps elevated and human to security-review and human-signoff gates. Task 3's fixed node list has neither node, so both verdicts collapse onto the standard set and a human-class instance declares no human node. Deriving the value needs a design ruling. The question is open at #1933, with no ruling recorded.
RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:74-77 — equals the four apps: roles in topology.yaml. Row 7 pins the two together, and it passed.
RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — this is the stream's fixed risk-class vocabulary, from the vocabulary fact in the brief's Context. The missing-verdict test passed.
RISK-VALUE: DERIVED — patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70 — Task 4's stated contract. Observed by hand in this pass: rc=0 on the shipped patterns, rc=1 on each mutated copy and bad fixture, and rc=2 on a root with no spec/workflow-patterns directory.
RISK-VALUE: reversible knob, ranked last, no derivation owed — budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

**VERIFY: BLOCKED** — rows 1-8 pass by hand (8 of 9). Row 10 is could-not-check as written and fails the witness by check definition. It needs a Verify-row re-author (class #1281, still open). On this darwin host the witness could not run check:ci rows 1, 3, 4 and 7 (#1800). Status stays implemented.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer questions: (1) can any node in either shipped pattern act with an authority the
role it names does not hold today? (2) is every node at a durable boundary, or has a shell
step been promoted to a node?
