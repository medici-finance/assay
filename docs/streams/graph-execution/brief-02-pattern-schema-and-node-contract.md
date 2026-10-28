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
version: 2
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
| 10 | check | `o=$(statusgen --root . --consumers --brief graph-execution/02 --delivery 5b384e52c684be71b1447949a9403250640b66a7 --base 060980aa0f1a34035b4ce47e9d4c932c0e829662); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | exit 0; exact `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing` — claims are read from the immutable delivered snapshot of 5b384e52c (#1259) and compared only with its first-parent diff (060980aa0), so the claim stays checkable on merged main instead of degrading to could-not-check; the one UNCHECKED entry is the out-of-scope skill-procedure routing, whose truth is the reviewer's call. Re-baselined 2026-10-16 (was: the live tree, rc=0 only on the authoring branch); moved to the delivery-check contract in `docs/consumer-delivery-checks.md` 2026-10-18 |

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

Non-implementer verification on merged main b0088804294b8b68ad8d06f341e6f0fd9dd2637d, gate: model, all four risk answers no. First table: the `statusgen verifyrun --dry-run` execution witness, landed verbatim except for its header separator line, which was restored on 2026-10-24 (see the Correction entries below); it ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, `unshare --net` available), statusgen built in-container from a clone pinned to this SHA. Second table: the hand run.

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

- The first execution-witness table in the re-run above was transcribed without its header separator line, so a Markdown parser reads the header as a data row and shifts every witness row down by one. The same table is restated below, verbatim and unchanged except for the separator line. The verdict is unchanged. On 2026-10-24 the separator line was also restored in the original table above (see the 2026-10-24 Correction entry), so the restatement below now duplicates it; it is kept as the record of this correction.

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

Execution witness (`statusgen verifyrun --brief docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md --dry-run`, statusgen v1.0.31, darwin host, nothing written): exit 2; 4 of 9 rows witnessed pass, 4 could-not-run, 1 fail.

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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370 (on-behalf-of human:ian) (git-config) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | pass exit=0 | sha256:e86054a2ccc7 | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370 (on-behalf-of human:ian) (git-config) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370 (on-behalf-of human:ian) (git-config) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370 (on-behalf-of human:ian) (git-config) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370 (on-behalf-of human:ian) (git-config) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370 (on-behalf-of human:ian) (git-config) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370 (on-behalf-of human:ian) (git-config) |
| 8 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:817d94d909c5 | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370 (on-behalf-of human:ian) (git-config) |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | fail exit=0 | sha256:63f0a599d650 | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370 (on-behalf-of human:ian) (git-config) |
Verification-Attestation: medici-finance/assay#3144 run=7214283dc8356cc524aca24b83e1177a5f4965477d405becd5a0512bd017ce14 source=1948dc0873705930b202f0e747047df844010d1a brief=docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md model=claude-opus-5-5 tier=any


### Non-implementer verifier re-run — 2026-10-16, merged main 1948dc0873705930b202f0e747047df844010d1a

Verification-Attestation: medici-finance/assay#3144 run=7214283dc8356cc524aca24b83e1177a5f4965477d405becd5a0512bd017ce14 source=1948dc0873705930b202f0e747047df844010d1a brief=docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md model=claude-opus-5-5 tier=any

Admission: deskdispatch --check-verifier exit 0 before any row ran. Expected values were written from the brief text and the tree at this head before reading earlier Evidence. Go rows ran with a throwaway HOME and TMPDIR and the real Go caches. Rows 2, 8 and 10 ran a statusgen built from this commit's own statusgen package, not an installed release. Row 5 needs a python3 that has PyYAML and jsonschema. The host's default python3 has no PyYAML (ModuleNotFoundError, exit 1), so the row ran with the system python 3.9.6 (PyYAML 6.0.3, jsonschema 4.21.1) and Homebrew python 3.14 (PyYAML 6.0.3, jsonschema 4.26.0). This is an environment fact, not a property of the tree.

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | exit 0 | exit 0; ok github.com/medici-finance/assay/statusgen, not cached. A -count=1 -v repeat lists 11 --- PASS lines for TestPatterns tests and 0 FAIL, so the selector matches real tests | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | rc=0 over the two shipped patterns | patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned) / rc=0 | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | exit 0; a reviewer node with push makes the lint exit 1 naming pattern-effect-exceeds-role | exit 0; ok github.com/medici-finance/assay/statusgen; --- PASS: TestPatternsEffectExceedsRoleIsProblem. Independent mutation on a scratch root outside the home: the bad-effect-exceeds-role fixture alone gives rc=1 with [pattern-effect-exceeds-role] node "review" has role "reviewer", which is not permitted to perform effect kind "push". The shipped implementation pattern with its review node's comment effect changed to push (line 44) also gives rc=1 under the same rule | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | exit 0; a review node sharing its input producer's role is refused | exit 0; ok github.com/medici-finance/assay/statusgen; --- PASS: TestPatternsReviewSameRoleIsProblem. Independent mutation: the bad-review-same-role fixture alone gives rc=1 with [pattern-review-same-role]. The shipped implementation pattern with the review role changed to worker (line 37) gives rc=1: review node "review" has role "worker", the same role as "implement", which produced its input "draft-pr" | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | prints ok | exit 0; ok under both interpreters named above (system python 3.9.6 with jsonschema 4.21.1, Homebrew python 3.14 with jsonschema 4.26.0) | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | count >= 1 | exit 0; 1 | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | exit 0 | exit 0; ok github.com/medici-finance/assay/statusgen; --- PASS: TestTopologyValuesMatchSource | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 8 | `statusgen --root . --lint; echo rc=$?` | rc=0 | LINT: PASS / rc=0 | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 10 | `statusgen --consumers --brief graph-execution/02 --root .; echo rc=$?` | rc=0 on the authoring branch; exit 2 on a fully merged main is acceptable, recorded as could-not-check, never as pass | could-not-check: statusgen rc=2, "COULD-NOT-CHECK: assay:assay:graph-execution:02 is not in the diff against 1948dc0873705930b202f0e747047df844010d1a". The shell exits 0 because of the trailing echo. Recorded as could-not-check, as the row's Expect requires. Sanctioned merge-diff recipe in a scratch clone at squash merge 5b384e52c (#1259) with --base 060980aa0f1a (its parent): exit 0; 3 corroborated, 0 disproved, 1 unchecked. The unchecked entry is the out-of-scope desk-skill claim, which that diff leaves unchanged | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

**Execution witness** (statusgen verifyrun, write mode, darwin host, statusgen built from this commit and first on PATH, PyYAML-capable python3 first on PATH): exit 2. The witness table directly above this block is its own output, landed verbatim. Rows 2, 5, 6 and 8 pass. The hashes for rows 2, 5 and 6 (e86054a2ccc7, dc51b8c96c2d, 4355a46b19d3) match earlier darwin witnesses. Rows 1, 3, 4 and 7 are check:ci rows: the witness could not run them because this darwin host has no unshare --net sandbox (#1800, open). Row 10 records fail exit=0 ("exit 0, expected 2"), the check-definition defect below. A first witness attempt with the host's default python3 recorded row 5 as fail exit=1 because PyYAML was missing. That output was this run's own, never landed, and was replaced by the run above.

Findings:

- **No implementation defect.** Rows 1-8 pass by hand, 8 of 9. The substance of row 10 passes by the sanctioned merge-diff recipe. The rows 3 and 4 mutations show the lint refuses a reviewer push and a same-role review on the shipped pattern itself, not only on fixtures.
- **Row 10 is a check-definition blocker, unchanged.** On merged main statusgen exits 2 and the trailing echo makes the shell exit 0. The witness parser takes "exit 2" from the Expect and records a fail, and the Expect forbids recording the exit-2 case as a pass. It clears only when the Verify row is re-authored with a pinned base before the implementation merge. Class: #1281 (open). The re-author tracker #1927 (open) names this brief's row 10 in a comment.
- **Rows 1, 3, 4 and 7, environment:** the darwin witness could not run them (#1800, open). All four pass by hand in this pass.
- **The risk-input design question is still open at #1933**, with no comments and no ruling recorded.

Risk-bearing value enumeration, re-read at 1948dc08:

1. patternRoleEffectPermissions: worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95. Mirrored at spec/workflow-pattern-v1.md:155-158.
2. Implementation risk-input: low = [review]; standard = [review, verify]; elevated = [review, verify]; human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:15-18. Research risk-input: all four = [review-artifact] @ spec/workflow-patterns/research-v1.yaml:13-16.
3. topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:74-77. Source: topology.yaml apps:, from line 236.
4. patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100.
5. patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70.
6. budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31, and research-v1.yaml:26, 37 and 50.

Ranked by irreversibility, none is irreversible. Each is undone by a spec, pattern or lint edit plus a version bump, and a node naming a role mints no token. By consequence: entry 1, the declared single point of failure for permission safety; then entry 2, the gates a risk class makes mandatory; then entries 3 and 4, the vocabulary; then entry 5, the instrument contract; last, entry 6, a retry knob.

RISK-VALUE: DERIVED — patternRoleEffectPermissions worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 — equals Task 2's table exactly. No role holds another role's certifying effect: only the reviewer reviews and only the verifier commits evidence. That is the writer-certifier separation the enforcement model requires. The lint reads this table: a push effect on the shipped reviewer node exits 1 in this pass.
RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — Task 1's example maps elevated and human to security-review and human-signoff gates. Task 3's fixed node list has neither node, so both verdicts collapse onto the standard set and a human-class instance declares no human node. Deriving the value needs a design ruling, open at #1933.
RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:74-77 — equals the four apps: roles in topology.yaml. Row 7 pins the two together and passed.
RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — this is the stream's fixed risk-class vocabulary, from the brief's Context vocabulary fact. The missing-verdict test passed in the row 1 run.
RISK-VALUE: DERIVED — patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70 — Task 4's stated contract. Observed in this pass: rc=0 on the shipped patterns and rc=1 on each bad fixture and mutated copy.
RISK-VALUE: reversible knob, ranked last, no derivation owed — budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

**VERIFY: BLOCKED** — rows 1-8 pass by hand, 8 of 9. Row 10 is could-not-check as written and fails the witness by check definition. It needs a Verify-row re-author (class #1281; tracked at #1927). Status stays implemented.

### Non-implementer verifier re-run — 2026-10-21, merged main 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1

Verification-Attestation: medici-finance/assay#3903 run=3fe3cf01d83d359e6df82f4f3a4ee1022aaf4a6097eaf75edc3b699f3ec7f9a7 source=1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 brief=docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md model=claude-opus-5-5 tier=strong

Admission: deskdispatch --check-verifier exit 0 before any row ran. Every row ran exactly as written in the Verify table at this head. Rows 2, 8 and 10 ran the installed statusgen v1.0.54 on PATH; a supplemental repeat with a statusgen built from this commit's own statusgen package (output kept outside the home) gave the same key lines for all three. Row 5 needs a python3 with PyYAML and jsonschema: the host's default python3 has no PyYAML (ModuleNotFoundError, exit 1), so the row ran with the system python 3.9.6 (PyYAML 6.0.3, jsonschema 4.21.1) first on PATH. This is an environment fact, not a property of the tree. Host: darwin, go 1.27.2, git 2.56.0. KUBECONFIG=/dev/null; no live endpoint contacted.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | exit 0 | exit 0; ok github.com/medici-finance/assay/statusgen. A -count=1 -v repeat lists 11 --- PASS lines for TestPatterns tests and 0 FAIL, so the selector matches real tests | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | rc=0 over the two shipped patterns | patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned) / rc=0 | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | exit 0; a reviewer node with push makes the lint exit 1 naming pattern-effect-exceeds-role | exit 0; ok github.com/medici-finance/assay/statusgen; --- PASS: TestPatternsEffectExceedsRoleIsProblem. Supplemental mutation in a fresh mktemp -d root: the shipped implementation pattern with its review node's comment effect changed to push (line 44) gives rc=1, [pattern-effect-exceeds-role]: node "review" has role "reviewer", which is not permitted to perform effect kind "push" | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | exit 0; a review node sharing its input producer's role is refused | exit 0; ok github.com/medici-finance/assay/statusgen; --- PASS: TestPatternsReviewSameRoleIsProblem. Supplemental mutation in a fresh mktemp -d root: the shipped implementation pattern with the review role changed to worker (line 37) gives rc=1, [pattern-review-same-role]: review node "review" has role "worker", the same role as "implement", which produced its input "draft-pr" | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | prints ok | exit 0; ok (system python 3.9.6, jsonschema 4.21.1). Supplemental discrimination probe: the same schema rejects the implementation pattern with nodes[0].kind set to bogus ('bogus' is not one of ['artifact', 'check', 'decision', 'effect']) | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | count >= 1 | exit 0; 1 | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | exit 0 | exit 0; ok github.com/medici-finance/assay/statusgen; --- PASS: TestTopologyValuesMatchSource | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `statusgen --root . --lint; echo rc=$?` | rc=0 | LINT: PASS / rc=0 (NOTICE lines only, from other streams) | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | `o=$(statusgen --root . --consumers --brief graph-execution/02 --delivery 5b384e52c684be71b1447949a9403250640b66a7 --base 060980aa0f1a34035b4ce47e9d4c932c0e829662); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | exit 0; exact summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing | exit 0; base 060980aa0f1a..5b384e52c684 (immutable delivered snapshot); CORROBORATED spec/README.md, statusgen/main.go, docs/lifecycle.md; UNCHECKED the out-of-scope desk-skill claim; summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing | 2026-10-21 | assay-verifier-app[bot] @ 1f0b54e745d84a9f58d15f5d4bb2f9e84a5b52a1 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Row accounting: 9 of 9 rows pass; 0 fail; 0 could-not-check. Row 10, re-authored to the delivery-check contract since the 2026-10-16 pass, now runs green on merged main. The one UNCHECKED consumer entry is the out-of-scope skill-procedure routing; the reviewer's call per the Expect, and the out-of-scope reason holds (the delivery diff changes none of the three desk skills).

Risk-bearing value enumeration, re-read at 1f0b54e7 (the enumerated files are byte-identical to 1948dc08):

1. patternRoleEffectPermissions: worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95. Mirrored at spec/workflow-pattern-v1.md:155-158.
2. Implementation risk-input: low = [review]; standard = [review, verify]; elevated = [review, verify]; human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:15-18. Research risk-input: all four = [review-artifact] @ spec/workflow-patterns/research-v1.yaml:13-16.
3. topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:74-77.
4. patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100.
5. patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70.
6. budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31, and research-v1.yaml:26, 37 and 50.

Ranked by irreversibility, none is irreversible (frontmatter irreversible: no). Each is undone by a spec, pattern or lint edit plus a version bump, and a node naming a role mints no token. By consequence: entry 1 (the declared single point of failure for permission safety), then entry 2, then entries 3 and 4 (vocabulary), then entry 5 (instrument contract), last entry 6 (a retry knob).

RISK-VALUE: DERIVED — patternRoleEffectPermissions worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 — equals Task 2's table exactly; only the reviewer reviews and only the verifier commits evidence (the writer-certifier separation of the enforcement model), and no role holds a merge effect (the shipped merge node is a desk dispatch, merge staying human). The lint reads this table: a push effect on the shipped reviewer node exits 1 in this pass.
RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — Task 1's example maps elevated and human to security-review and human-signoff gates; Task 3's fixed node list has neither node, so both verdicts collapse onto the standard set and a human-class instance declares no human node. Deriving the value needs a design ruling, still open at #1933 (0 comments, re-read 2026-10-21). It is not a failed obligation: the MUST rule (all four verdicts mapped) holds.
RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:74-77 — equals the four apps: roles in topology.yaml; row 7 pins the two together and passed.
RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — the stream's fixed risk-class vocabulary from the brief's Context vocabulary fact.
RISK-VALUE: DERIVED — patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70 — Task 4's stated contract; observed rc=0 on the shipped patterns and rc=1 on each mutated copy in this pass.
RISK-VALUE: reversible knob, ranked last, no derivation owed — budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

**VERIFY: PASS** — 9 of 9 rows pass on merged main 1f0b54e7; the risk-input design question stays routed at #1933.

### Corrective re-pass — 2026-10-22, merged main 05dc9fbc88f2b1b23cd78bebc8634c564327eca4

This section supersedes the statements of the 2026-10-21 section named below. It does not edit that section. It answers the review findings F-no-witness and F-ci-red on #3913.

Admission: deskdispatch --check-verifier exit 0 before any row ran, and again after the run. Fresh dispatch; the home is detached at merged main 05dc9fbc88f2, which is the attested source.

Delivered code since the prior pass: git diff --stat 1f0b54e745d8..HEAD over the brief's delivered paths (the two pattern files, the schema document and JSON schema, spec README, the patterns source and tests, its fixtures, the topology derivation, topology.yaml, the main entry point, lifecycle doc and changelog fragment) lists one file: statusgen/main.go, 9 insertions and 1 deletion. That change adds the unrelated `context` subcommand and its name in the unknown-subcommand list. The patterns code, fixtures, schema and pattern files are unchanged.

Tools: statusgen (with its verifyrun subcommand) built from this commit's own statusgen package into a scratch directory outside the home (`go build -o "$SCRATCH/bin/statusgen" .` with `SCRATCH` that directory, go 1.27.2 darwin/arm64), first on PATH for every row. Row 5 ran with a scratch venv python3 (Python 3.15.0, PyYAML 6.0.3, jsonschema 4.26.0) first on PATH. Host: darwin, git 2.56.0. KUBECONFIG=/dev/null; no live endpoint contacted.

Witness: `statusgen verifyrun --brief docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md` in write mode at the home root, exit 2. The table below is that run's own output, copied unchanged, Runner cells included. `statusgen verifyrun --check` on the written table: exit 2, "5 pass, 0 fail, 4 could-not-run/missing (of 9 Verify rows); 0 author-added with no acceptance credit".

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | pass exit=0 | sha256:e86054a2ccc7 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:1afe40cf07e0 | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
| 10 | `o=$(statusgen --root . --consumers --brief graph-execution/02 --delivery 5b384e52c684be71b1447949a9403250640b66a7 --base 060980aa0f1a34035b4ce47e9d4c932c0e829662); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | pass exit=0 | sha256:d44e0281b75b | 2026-10-22 | assay-verifier-app[bot] @ 05dc9fbc88f2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4017 run=f8a7d689cac3722d3452aa508728be372f218a0bf1ff1570db1c72f5fbf96574 source=05dc9fbc88f2b1b23cd78bebc8634c564327eca4 brief=docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md model=claude-opus-5-5 tier=strong

Linux lane for rows 1, 3, 4 and 7: the local verify image (go1.27-py, linux/arm64, run with --network none and --sandbox container-netns, statusgen and deskdispatch cross-built from this commit) could not produce an admitted witness. verifyrun's pre-work verifier admission runs deskdispatch --check-verifier before any row. Inside the image that check refuses, with "REFUSED — no roster configured" and "deskdispatch activation could not be established". Admission needs the roster and a forge read, and a --network none container has neither. No row ran under that lane, and the four rows stay could-not-run in the witness above. This is the verify-image blocker #4010; it adds this cause to the ones that issue lists.

Supplemental, no acceptance credit: plain `go test -count=1` runs of the four check:ci row selectors in the same image with --network none each exit 0. A -v run of the statusgen package shows 12 --- PASS lines, TestPatternsEffectExceedsRoleIsProblem, TestPatternsReviewSameRoleIsProblem and TestTopologyValuesMatchSource among them, and no FAIL. Host repeats of rows 2, 8 and 10 with the same built statusgen print "patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned)" / rc=0, "LINT: PASS" / rc=0, and "summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing" with exit 0. Row 6's grep prints 1. These lines are diagnostics only: rows 2 and 8 end in echo rc=$?, so the witness checks exit status only, and these repeats show the rc= lines.

Row accounting from the witness: 9 required rows. 5 pass (rows 2, 5, 6, 8, 10). 0 fail. 4 could-not-run (rows 1, 3, 4, 7: check:ci rows need the Linux network-off sandbox, and the only local Linux lane cannot admit the run, #4010).

Superseded statements in the 2026-10-21 section:

- Line 505 (the admission and environment paragraph): rows 2, 8 and 10 ran the installed statusgen v1.0.54. Correction: in this pass every row ran a statusgen built from merged main 05dc9fbc, and row 5 ran with a scratch python3 that has PyYAML.
- Lines 507-517 (the hand-written row table): no verifyrun witness backs that table. Correction: the witness table above replaces it as the record of row results.
- Line 519 (row accounting, "9 of 9 rows pass; 0 fail; 0 could-not-check"): Correction: 5 pass, 0 fail, 4 could-not-run, per the witness.
- Line 539 (the verdict line, PASS, 9 of 9 rows): Correction: BLOCKED, below. The same "9 of 9 rows pass" claim in the PR body and in the 2026-10-21 comment on #1933 is superseded too.

CI on the Evidence PR head: the push-event run 39654124652 failed build-test in the cellcache mutate-gate SIGTERM subtest. That is the known intermittent #3698, and this docs-only change cannot cause it. This pass re-ran no workflow. Re-running the failed job is for the driver only.

Risk-bearing value enumeration, re-read at 05dc9fbc. None of the enumerated files changed since 1f0b54e7.

1. patternRoleEffectPermissions: worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95. Mirrored at spec/workflow-pattern-v1.md:155-158.
2. Implementation risk-input: low = [review]; standard = [review, verify]; elevated = [review, verify]; human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:15-18. Research risk-input: all four = [review-artifact] @ spec/workflow-patterns/research-v1.yaml:13-16.
3. topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:74-77.
4. patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100.
5. patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70.
6. budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

Ranking is unchanged from the 2026-10-21 section. None of these values is irreversible (frontmatter irreversible: no). Entry 6 is a retry knob and ranks last.

RISK-VALUE: DERIVED — patternRoleEffectPermissions worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 — equals Task 2's table exactly. Only the reviewer reviews and only the verifier commits evidence, which is the writer-certifier separation. No role holds a merge effect.
RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — Task 1's example maps these to security-review and human-signoff gates, and the shipped pattern has neither node. Deriving the value needs a design ruling, still open at #1933.
RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:74-77 — equals the four apps: roles in topology.yaml. Row 7 pins the two together; in this pass that is shown only by the supplemental network-off run.
RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — the stream's fixed risk-class vocabulary from the brief's Context.
RISK-VALUE: DERIVED — patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70 — Task 4's stated exit contract.
RISK-VALUE: reversible knob, ranked last, no derivation owed — budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

**VERIFY: BLOCKED** — 5 of 9 rows witnessed passing, 0 fail. Rows 1, 3, 4 and 7 are could-not-run: the check:ci rows need a Linux network-off runner that can complete verifier admission (#4010). Status stays where it is.

### Non-implementer verifier re-run — 2026-10-22, merged main b87d9f502cff0e426a221d93ce826e4f10d47e9f

Run by a fresh, non-implementer verifier (claude-opus-5-5, strong tier) under attestation medici-finance/assay#4155. Admission: `deskdispatch --check-verifier` re-admitted the run at the source above, with the home clean, before the run and again before this landing. The witness below comes from the installed `statusgen verifyrun` (v1.0.56) on the host (darwin). Every row ran against this commit.

- The check:ci rows (1, 3, 4 and 7) are could-not-run again. The host has no network-off sandbox: `unshare --net` is Linux-only. The container admission bracket (brief-rule 45, desktools-v2/71), which would give these rows a sanctioned network-off lane, landed on main on 2026-10-22 in 3989f898e but is in no release yet, and the installed v1.0.56 has no `--open-bracket`. This pass does not use a locally built statusgen in place of the installed instrument. A container run made with a source-built binary earlier in this dispatch is NOT recorded here and claims no credit. Cause: #4010. Rows 1, 3, 4 and 7 need a release that contains 3989f898e.
- Row 5 (`check`, env-bound) needs PyYAML and jsonschema. The verifier's desk launcher swaps HOME, so the interpreter's per-user site-packages became invisible. An earlier invocation in this pass therefore recorded row 5 as `fail exit=1` (`ModuleNotFoundError: No module named 'yaml'`). The cause was the runner environment, not the tree. That table was discarded before landing, and the run below sets PYTHONPATH to the same interpreter's site-packages. The row 5 output hash (dc51b8c96c2d) equals the earlier passing witnesses of this row.
- The `echo rc=$?` rows were read directly at this commit, because their exit cell cannot fail. Row 2 printed `patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned)`, then `rc=0`. Row 8 printed `LINT: PASS`, then `rc=0`; it emitted NOTICE lines only and no PROBLEM. Row 10 read base 060980aa0 against the immutable delivered snapshot 5b384e52c (#1259). It CORROBORATED spec/README.md, statusgen/main.go and docs/lifecycle.md §Review gates, left the desk-skill entry UNCHECKED, and printed the exact `summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing` with s=0.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ b87d9f502cff (on-behalf-of human:ian) (forge-identity) |
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | pass exit=0 | sha256:e86054a2ccc7 | 2026-10-22 | assay-verifier-app[bot] @ b87d9f502cff (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ b87d9f502cff (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ b87d9f502cff (on-behalf-of human:ian) (forge-identity) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-22 | assay-verifier-app[bot] @ b87d9f502cff (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-22 | assay-verifier-app[bot] @ b87d9f502cff (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-22 | assay-verifier-app[bot] @ b87d9f502cff (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:c816b5c89433 | 2026-10-22 | assay-verifier-app[bot] @ b87d9f502cff (on-behalf-of human:ian) (forge-identity) |
| 10 | `o=$(statusgen --root . --consumers --brief graph-execution/02 --delivery 5b384e52c684be71b1447949a9403250640b66a7 --base 060980aa0f1a34035b4ce47e9d4c932c0e829662); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | pass exit=0 | sha256:57f9f3c09f54 | 2026-10-22 | assay-verifier-app[bot] @ b87d9f502cff (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4155 run=01602a76c346503338e0fa4bb43609ea0593f3429385ba102add252b90f3d149 source=b87d9f502cff0e426a221d93ce826e4f10d47e9f brief=docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md model=claude-opus-5-5 tier=strong

Risk-value enumeration, re-read at b87d9f502cff:

1. patternRoleEffectPermissions: worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95.
2. Implementation risk-input: elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18.
3. topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:74-77.
4. patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100.
5. patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70.
6. budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

The ranking is unchanged from the 2026-10-21 section. None of these values is irreversible (frontmatter irreversible: no). Entry 6 is a retry knob, so it ranks last.

RISK-VALUE: DERIVED — patternRoleEffectPermissions worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:92-95 — these are the effects each topology.yaml apps: role performs today (spec/workflow-pattern-v1.md §7).
RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — Task 1's example maps these to the same pair as standard. The mapping ruling is still open at #1933, unchanged by this pass.
RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:74-77 — equals the four apps: roles in topology.yaml. Row 7 pins the two together; in this pass row 7 is could-not-run, so the pin is unwitnessed at this commit.
RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — the stream's fixed risk-class vocabulary from the brief's Context.
RISK-VALUE: DERIVED — patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70 — Task 4's stated exit contract.
RISK-VALUE: reversible knob, ranked last, no derivation owed — budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

**VERIFY: BLOCKED** — 5 of 9 rows witnessed passing (2, 5, 6, 8, 10) and 0 fail. Rows 1, 3, 4 and 7 are could-not-run because the check:ci rows have no admitted network-off lane in the installed tools (#4010). The admission-bracket lane awaits a release that contains 3989f898e. Status stays where it is.

### Non-implementer verifier re-run — 2026-10-24, merged main 385cb29e5682dcb9a886b298076f722b4823ec1b — container-netns admission bracket

Prior attempts: the two 2026-10-22 passes (sources 05dc9fbc and b87d9f50) closed at VERIFY: BLOCKED with rows 1, 3, 4 and 7 could-not-run for want of an admitted network-off lane (#4010); this run is a fresh dispatch under its own admission and credits nothing from them.

Run by a fresh, non-implementer verifier (claude-opus-5-5, strong tier) under attestation medici-finance/assay#4304. Admission: deskdispatch --check-verifier exit 0 at the home root before any row, with the home clean and detached at the attested source above; the open and close bracket steps each re-ran the check. Installed host tools: statusgen v1.0.59; git 2.56.0; host darwin. Container lane: release-pinned runner ghcr.io/medici-finance/verify-runner:v1.0.59@sha256:c2e32c7ff99f67951174e8ff835d928d8999b1bb2ecdc7c63d3775528178640e (image Id sha256:756b81ba91a8627b30524316a3baabed654a0369f9331a31e77546ead69c19ad, linux/amd64), run with --network none, a read-only root filesystem, all capabilities dropped and the home mounted read-only at a fixed container path; the rows ran under the image's own released statusgen. No locally built binary, gitdir mount or identity injection was used. KUBECONFIG=/dev/null; no live endpoint was contacted.

Rows 1, 3, 4 and 7 (check:ci) ran in ONE closed admission bracket, opened 2026-10-24T17:34:17Z and closed 2026-10-24T17:42:31Z. The table below is the closed witness file's bytes, unchanged. statusgen verifyrun --bracket-digest on that file printed bracket=L10 header=L1 verdict=match span-lines=tree-digest:1,container-image:1,attestation:1 (exit 0).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | pass exit=0 sandbox=container-netns | sha256:051655273936 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | pass exit=0 sandbox=container-netns | sha256:b4a860f0cc6d | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | pass exit=0 sandbox=container-netns | sha256:47f816e40eb4 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | pass exit=0 sandbox=container-netns | sha256:a7adfdc293f9 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104
Container-Image: sha256:c2e32c7ff99f67951174e8ff835d928d8999b1bb2ecdc7c63d3775528178640e
Verification-Attestation: medici-finance/assay#4304 run=58880a5cb8e83dcf0024929427bdb73a5ccb7a3d3ae4aec5f121a7d26a4f862f source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=58880a5cb8e83dcf0024929427bdb73a5ccb7a3d3ae4aec5f121a7d26a4f862f source=385cb29e5682dcb9a886b298076f722b4823ec1b tree=sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104 witness=sha256:4a15d113e59e8afb30e7e8a926eb1733cfc98fdbcebeb4880bbba262e7f79cc6 opened=2026-10-24T17:34:17Z closed=2026-10-24T17:42:31Z before=receipt after=check-verifier

Rows 2, 5, 6, 8 and 10 ran on the host with the installed statusgen verifyrun --rows 2,5,6,8,10 --dry-run, while the home was still clean and before the bracket closed; the table below is that run's own output, unchanged. Row 5 ran with a python3 3.14 that has PyYAML 6.0.3 and jsonschema first on PATH (the default interpreter lacks PyYAML; that is a runner-environment fact, not a tree result).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | pass exit=0 | sha256:e86054a2ccc7 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:b0218a0e1b5a | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 10 | `o=$(statusgen --root . --consumers --brief graph-execution/02 --delivery 5b384e52c684be71b1447949a9403250640b66a7 --base 060980aa0f1a34035b4ce47e9d4c932c0e829662); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | pass exit=0 | sha256:57f9f3c09f54 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4304 run=58880a5cb8e83dcf0024929427bdb73a5ccb7a3d3ae4aec5f121a7d26a4f862f source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md model=claude-opus-5-5 tier=strong

Rows 2 and 8 end in echo rc=$?, so the witness checks exit status only; the lines they print were read directly at this commit, as diagnostics with no separate credit. Row 2 printed "patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned)" then rc=0. Row 8 printed LINT: PASS then rc=0, with NOTICE lines only and no PROBLEM line. Row 10 corroborated spec/README.md, statusgen/main.go and docs/lifecycle.md §Review gates against the delivered snapshot 5b384e52c and its first-parent diff, left the desk-skill entry UNCHECKED (out-of-scope, the reviewer's call), and printed the exact "summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing" with s=0. Row 6's grep printed 1.

Supplemental, no acceptance credit: the installed statusgen patterns --lint, run over scratch roots outside the home that each hold one shipped fixture, exits 1 on bad-effect-exceeds-role.yaml naming [pattern-effect-exceeds-role] (node "review" has role "reviewer", which is not permitted to perform effect kind "push"), and exits 1 on bad-review-same-role.yaml naming [pattern-review-same-role] (review node "review" has role "worker", the same role as "implement"). This matches the second clause of the Expect cells of rows 3 and 4.

The lint reports the [gotest-run-vacuous] NOTICE on rows 1, 3, 4 and 7 (go test -run carries no named PASS assertion, so a missing test would also exit 0). Supplemental, no acceptance credit: a verbose network-off run in the same pinned image with the home mounted read-only, go test -count=1 -v -run "TestPatterns|TestTopologyValuesMatchSource" ./... in statusgen, exited 0 with 12 --- PASS lines and no FAIL: TestPatternsRoleEffectPermissionsCoversTopologyRoles, TestPatternsEmbeddedSchemaMatchesCommitted, TestPatternsGoodFixturesLintClean, TestPatternsShippedPatternsLintClean, TestPatternsEffectExceedsRoleIsProblem, TestPatternsEffectTargetNotOwnedIsProblem, TestPatternsReviewSameRoleIsProblem, TestPatternsJoinNotCheckIsProblem, TestPatternsRiskInputMissingVerdictIsProblem, TestPatternsLintReportsCouldNotCheckOnMissingRoot, TestPatternsNoVerbIsUsageRefusal and TestTopologyValuesMatchSource. So each named test exists and passes at this commit; the selectors are not vacuous here. Adding the named PASS assertion to these rows is a Verify-row re-author, out of this pass's scope.

Delivered code since the previous pass (b87d9f50): git diff --stat over the brief's delivered paths lists only spec/README.md (two proposed milestone and stream spec rows and a section about them; the workflow-pattern-v1 row is unchanged) and statusgen/main.go (lint notice reporting, --all and --json lint flags, register-link reporting). Neither touches the patterns code, fixtures, schema, pattern files or topology derivation.

Row accounting: 9 required rows. 9 pass (rows 1, 3, 4 and 7 in the container bracket; rows 2, 5, 6, 8 and 10 on the host). 0 fail. 0 could-not-run.

Risk-bearing value enumeration, re-read at 385cb29e5682 (frontmatter irreversible: no; all values reversible by edit and redeploy):

1. patternRoleEffectPermissions: worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:91-96. Mirrored at spec/workflow-pattern-v1.md:155-158.
2. Implementation risk-input: low = [review]; standard = [review, verify]; elevated = [review, verify]; human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:15-18. Research risk-input: all four = [review-artifact] @ spec/workflow-patterns/research-v1.yaml:13-16.
3. topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78.
4. patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100.
5. patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70.
6. budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

Ranking: entry 1 first (a wrong permission lets a pattern grant a role an effect it does not hold), then 2 (gate nodes per risk class), then 3, 4 and 5; entry 6 is a retry knob and ranks last.

RISK-VALUE: DERIVED — patternRoleEffectPermissions worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:91-96 — equals Task 2's table exactly. Only the reviewer reviews and only the verifier commits evidence, which is the writer-certifier separation; no role holds a merge effect.
RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — Task 1's example maps these to security-review and human-signoff gates, and the shipped pattern has neither node. Deriving the value needs a design ruling, still open at #1933.
RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78 — equals the four apps: roles in topology.yaml (lines 237-249). Row 7, witnessed in the bracket above, pins the two together at this commit.
RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — the stream's fixed risk-class vocabulary from the brief's Context.
RISK-VALUE: DERIVED — patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70 — Task 4's stated exit contract (0 clean, 1 on any PROBLEM, 2 when it cannot check).
RISK-VALUE: reversible knob, ranked last, no derivation owed — budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

**VERIFY: PASS** — 9 of 9 rows witnessed passing on merged main 385cb29e5682, 0 fail, 0 could-not-run; the check:ci rows 1, 3, 4 and 7 ran in a closed container-netns admission bracket. The risk-input design question stays routed at #1933.

#### Correction — 2026-10-24, assay-verifier-app[bot] (on-behalf-of human:ian)

Four shape-only edits, three to earlier entries and one to this entry, made so the flip's Evidence scan can key them. The driver ruled the edits allowed (https://github.com/medici-finance/assay/pull/4344#issuecomment-6389131147), with the tool fix to follow. No recorded result, count, digest or verdict changed. Each original span and its replacement:

1. 2026-09-30 entry, first execution-witness table. Original: the header row `| # | Command | Result | Output | Date | Runner |` was followed directly by row 1, with no separator line. Replacement: the separator line `|---|---------|--------|--------|------|--------|` was inserted after the header. The entry's preamble and its 2026-09-30 Correction entry now say so.
2. 2026-10-02 entry, execution witness line. Original: statusgen verifyrun --brief &lt;this brief&gt; --dry-run (in inline code; the angle brackets are written here as character references so this entry opens no element). Replacement: `statusgen verifyrun --brief docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md --dry-run`. The original entry recorded only the placeholder, so the replacement names the brief the placeholder stood for; it is not a record of the argument spelling that run used.
3. 2026-10-22 entry, Tools line. Original: go build -o &lt;scratch&gt;/bin/statusgen . (in inline code, angle brackets written as character references). Replacement: `go build -o "$SCRATCH/bin/statusgen" .` with `SCRATCH` that directory.
4. This 2026-10-24 entry, the RISK-VALUE line for patternsExitClean (the second-to-last RISK-VALUE line before the verdict), which is outside the admission bracket's span (the bracket's witness digest is unchanged). Original, then replacement, quoted exactly in the block below.

```text
original:    Task 4's stated exit contract (0 clean, 1 on any PROBLEM, 2 could-not-check).
replacement: Task 4's stated exit contract (0 clean, 1 on any PROBLEM, 2 when it cannot check).
```

The 2026-10-24 verdict above stands as recorded.

### Non-implementer verifier re-run — 2026-10-25, merged main 4aeecf71626f485c4938dec3165e887a2f74e20d — container-netns admission bracket and fail-first witness for row 10

Why this re-run: the 2026-10-24 pass above recorded all nine rows passing, but its row 10 witness carries no Base cell. Row 10 is risk-bearing under the closure gate's row tagging, so the lint refuses the implemented-to-verified flip until a fail-first witness shows the row reds on a tree without the change. This run witnesses every row afresh at the new merged main, adds that fail-first witness for row 10, and credits nothing from earlier passes.

Run by a fresh, non-implementer verifier (claude-opus-5-5, strong tier) under attestation medici-finance/assay#4432. Admission: deskdispatch --check-verifier exit 0 at the home root before any row, with the home clean and detached at the attested source above; the open and close bracket steps each re-ran the check. Installed host tools: statusgen v1.0.60; git 2.56.0; host darwin. Container lane: release-pinned runner ghcr.io/medici-finance/verify-runner:v1.0.60@sha256:4cb3f129e71ff220b62995c587fd4716e2c73c81267bc583df3f544f6fdd5ae3 (image Id sha256:056712f4c990696f651762d77f372ca9dbf6351b3cda32a5b5313f534eea239e, linux/amd64), on the local Docker engine with --network none, the engine's default seccomp profile, a read-only root filesystem, all capabilities dropped and the home mounted read-only at its own path; the rows ran under the image's own released statusgen v1.0.60 and Go 1.27.2. No locally built binary, gitdir mount or identity injection was used. KUBECONFIG=/dev/null; no live endpoint was contacted.

Rows 1, 3, 4 and 7 (check:ci) ran in ONE closed admission bracket, opened 2026-10-25T18:44:42Z and closed 2026-10-25T18:47:08Z. The table below is the closed witness file's bytes, unchanged. statusgen verifyrun --bracket-digest on that file printed bracket=L10 header=L1 verdict=match span-lines=tree-digest:1,container-image:1,attestation:1 (exit 0).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test -run TestPatterns ./...` | pass exit=0 sandbox=container-netns | sha256:66167cbfab62 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && go test -run TestPatternsEffectExceedsRoleIsProblem ./...` | pass exit=0 sandbox=container-netns | sha256:dbb3bdb63f85 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go test -run TestPatternsReviewSameRoleIsProblem ./...` | pass exit=0 sandbox=container-netns | sha256:457ba58440e7 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
| 7 | `cd statusgen && go test -run TestTopologyValuesMatchSource ./...` | pass exit=0 sandbox=container-netns | sha256:c2dd3763368e | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:a07e75f04765056587c4d1b5228d8647eb4d6d748c59de45455e49d7f64f4e9b
Container-Image: sha256:4cb3f129e71ff220b62995c587fd4716e2c73c81267bc583df3f544f6fdd5ae3
Verification-Attestation: medici-finance/assay#4432 run=30bc0a6848988039eb299f544a658106e986cc5d81c2dd82a5ad1b5234bdccd6 source=4aeecf71626f485c4938dec3165e887a2f74e20d brief=docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=30bc0a6848988039eb299f544a658106e986cc5d81c2dd82a5ad1b5234bdccd6 source=4aeecf71626f485c4938dec3165e887a2f74e20d tree=sha256:a07e75f04765056587c4d1b5228d8647eb4d6d748c59de45455e49d7f64f4e9b witness=sha256:9398658f2ba569191795946ad6ecac1ff9896829e72a03bb9606cfa41658f94e opened=2026-10-25T18:44:42Z closed=2026-10-25T18:47:08Z before=receipt after=check-verifier

Rows 2, 5, 6, 8 and 10 ran on the host with the installed statusgen verifyrun --rows 2,5,6,8,10 --fail-first --base 060980aa0f1a34035b4ce47e9d4c932c0e829662 --dry-run, with the home clean and before the bracket opened; the table below is that run's own output, unchanged. The base is the parent of the delivering commit 5b384e52c (the squash merge of #1259, a single-parent commit on main's first-parent line); it is an ancestor of the merge-base and it holds this brief. Only row 10 is risk-bearing under the gate's row tagging, so only row 10 ran at base, and rows 2, 5, 6 and 8 carry not-selected Base cells. Row 5 ran with the system python3 3.9.6, whose PyYAML 6.0.3 and jsonschema 4.21.1 live in the operator's user site-packages and were passed through PYTHONPATH, because the desk tool wrapper changes HOME and so hides the user site; that is a runner-environment fact, not a tree result. An earlier attempt of the same host run without PYTHONPATH failed row 5 (exit 1, no yaml module); it is superseded by the table below and credits nothing.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 2 | `statusgen patterns --lint --root .; echo rc=$?` | pass exit=0 | sha256:e86054a2ccc7 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=4aeecf71626f |
| 5 | `python3 -c 'import json,yaml;s=json.load(open("schemas/workflow-pattern-v1.json"));import jsonschema;[jsonschema.validate(yaml.safe_load(open(p)),s) for p in ["spec/workflow-patterns/implementation-v1.yaml","spec/workflow-patterns/research-v1.yaml"]];print("ok")'` | pass exit=0 | sha256:dc51b8c96c2d | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=4aeecf71626f |
| 6 | `grep -c 'workflow-pattern-v1' spec/README.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=4aeecf71626f |
| 8 | `statusgen --root . --lint; echo rc=$?` | pass exit=0 | sha256:80a3d251dc7e | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=4aeecf71626f |
| 10 | `o=$(statusgen --root . --consumers --brief graph-execution/02 --delivery 5b384e52c684be71b1447949a9403250640b66a7 --base 060980aa0f1a34035b4ce47e9d4c932c0e829662); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | pass exit=0 | sha256:57f9f3c09f54 | 2026-10-25 | assay-verifier-app[bot] @ 4aeecf71626f (on-behalf-of human:ian) (forge-identity) | base=060980aa0f1a red rc=1 · head=4aeecf71626f |
Verification-Attestation: medici-finance/assay#4432 run=30bc0a6848988039eb299f544a658106e986cc5d81c2dd82a5ad1b5234bdccd6 source=4aeecf71626f485c4938dec3165e887a2f74e20d brief=docs/streams/graph-execution/brief-02-pattern-schema-and-node-contract.md model=claude-opus-5-5 tier=strong

Fail-first, row 10: base=060980aa0f1a red rc=1, head=4aeecf71626f pass. Diagnostic, no separate credit: on a detached checkout of the base, the row's statusgen --consumers call prints "`COULD-NOT-CHECK: delivery must be an ancestor of the checkout's HEAD`" and exits 2, so the row's own exit-status guard exits 1. The red therefore shows that the row cannot pass on a tree that lacks the delivering commit; it does not show a claim disproved at base. That is the discrimination this row's delivery-check form provides, recorded here as observed.

Rows 2 and 8 end in echo rc=$?, so the witness checks exit status only; the lines they print were read directly at this commit, as diagnostics with no separate credit. Row 2 printed "patterns: 2 checked-clean, 0 checked-failed, 0 could-not-check (2 file(s) scanned)" then rc=0. Row 8 printed LINT: PASS then rc=0, with NOTICE lines only and no PROBLEM line. Row 10 at head corroborated spec/README.md, statusgen/main.go and docs/lifecycle.md §Review gates against the immutable delivered snapshot 5b384e52c, diffed from the base above, left the desk-skill entry UNCHECKED (out-of-scope, the reviewer's call), and printed the exact "summary: 3 corroborated, 0 disproved, 1 unchecked, 0 brief(s) claiming nothing" with s=0. Row 6's grep printed 1.

Supplemental, no acceptance credit: the installed statusgen patterns --lint, run over scratch roots outside the home that each hold one shipped fixture, exits 1 on bad-effect-exceeds-role.yaml naming [pattern-effect-exceeds-role] (node "review" has role "reviewer", which is not permitted to perform effect kind "push"), and exits 1 on bad-review-same-role.yaml naming [pattern-review-same-role] (review node "review" has role "worker", the same role as "implement"). This matches the second clause of the Expect cells of rows 3 and 4.

The lint reports the [gotest-run-vacuous] NOTICE on rows 1, 3, 4 and 7. Supplemental, no acceptance credit: a verbose network-off run in the same pinned image with the home mounted read-only, go test -count=1 -v -run "TestPatterns|TestTopologyValuesMatchSource" ./... in statusgen, exited 0 with 12 --- PASS lines and no FAIL: TestPatternsRoleEffectPermissionsCoversTopologyRoles, TestPatternsEmbeddedSchemaMatchesCommitted, TestPatternsGoodFixturesLintClean, TestPatternsShippedPatternsLintClean, TestPatternsEffectExceedsRoleIsProblem, TestPatternsEffectTargetNotOwnedIsProblem, TestPatternsReviewSameRoleIsProblem, TestPatternsJoinNotCheckIsProblem, TestPatternsRiskInputMissingVerdictIsProblem, TestPatternsLintReportsCouldNotCheckOnMissingRoot, TestPatternsNoVerbIsUsageRefusal and TestTopologyValuesMatchSource. Each named test exists and passes at this commit, so the selectors are not vacuous here.

Delivered code since the previous pass (385cb29e): git diff --stat over the brief's delivered paths lists only spec/README.md, one added spec-index row for another document; the workflow-pattern-v1 row is unchanged. Nothing in the patterns code, fixtures, schema, pattern files or topology derivation changed.

Row accounting: 9 required rows. 9 pass (rows 1, 3, 4 and 7 in the container bracket; rows 2, 5, 6, 8 and 10 on the host, row 10 with its fail-first Base cell). 0 fail. 0 could-not-run.

Risk-bearing value enumeration, re-read at 4aeecf71626f (frontmatter irreversible: no; all values reversible by edit and redeploy):

1. patternRoleEffectPermissions: worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:91-96. Mirrored at spec/workflow-pattern-v1.md:155-158.
2. Implementation risk-input: low = [review]; standard = [review, verify]; elevated = [review, verify]; human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:15-18. Research risk-input: all four = [review-artifact] @ spec/workflow-patterns/research-v1.yaml:13-16.
3. topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78.
4. patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100.
5. patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70.
6. budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

Ranking: entry 1 first (a wrong permission lets a pattern grant a role an effect it does not hold), then 2 (gate nodes per risk class), then 3, 4 and 5; entry 6 is a retry knob and ranks last.

RISK-VALUE: DERIVED — patternRoleEffectPermissions worker = push, pr-open, comment; reviewer = review, comment; verifier = evidence-commit, comment; desk = file-issue, comment, dispatch @ statusgen/patterns.go:91-96 — equals Task 2's table exactly. Only the reviewer reviews and only the verifier commits evidence, which is the writer-certifier separation; no role holds a merge effect.
RISK-VALUE: NAMED, NOT DERIVED — implementation risk-input elevated = [review, verify], human = [review, verify] @ spec/workflow-patterns/implementation-v1.yaml:17-18 — Task 1's example maps these to security-review and human-signoff gates, and the shipped pattern has neither node. Deriving the value needs a design ruling, still open at #1933.
RISK-VALUE: DERIVED — topologyAppRoles = desk, reviewer, verifier, worker @ statusgen/topologyvalues.go:73-78 — equals the four apps: roles in topology.yaml (lines 237-249). Row 7, witnessed in the bracket above, pins the two together at this commit.
RISK-VALUE: DERIVED — patternRiskVerdicts = low, standard, elevated, human @ statusgen/patterns.go:100 — the stream's fixed risk-class vocabulary from the brief's Context.
RISK-VALUE: DERIVED — patternsExitClean = 0, patternsExitFailed = 1, patternsExitCouldNot = 2, patternsExitUsageError = 2 @ statusgen/patterns.go:67-70 — Task 4's stated exit contract (0 clean, 1 on any PROBLEM, 2 when it cannot check).
RISK-VALUE: reversible knob, ranked last, no derivation owed — budget.attempts = 2 @ spec/workflow-patterns/implementation-v1.yaml:31.

**VERIFY: PASS** — 9 of 9 rows witnessed passing on merged main 4aeecf71626f, 0 fail, 0 could-not-run; the check:ci rows 1, 3, 4 and 7 ran in a closed container-netns admission bracket, and risk-bearing row 10 carries a fail-first witness, red at base 060980aa0f1a and passing at head. The risk-input design question stays routed at #1933.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer questions: (1) can any node in either shipped pattern act with an authority the
role it names does not hold today? (2) is every node at a durable boundary, or has a shell
step been promoted to a node?

Hold, per the driver's ruling:
https://github.com/medici-finance/assay/issues/1933#issuecomment-6419943765
This brief's `done` flip is held until graph-execution/33 reaches `verified` (its implementation
merged and its Verify rows evidenced by a non-implementer). /33 reshapes this brief's `merge` node
into a `human` decision and re-pins the pattern; the hold releases on that status alone.
This note is not machine-read: `statusgen --auto-flip-model` decides a `gate: model` flip from
Evidence and does not read `## Review`. The desk watching the board holds the flip by hand until
/33 is `verified`; a machine-read form of the hold is a follow-up.
