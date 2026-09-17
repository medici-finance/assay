---
brief: assay:assay:graph-execution:08
title: Signal-triggered pattern — incident and regression
why: >-
  The implementation and research patterns both start from something a person authored. A
  red main, a firing alert or a dashboard crossing a band starts from a signal, and today
  that has no dispatch shape at all: whoever notices it improvises the containment, the
  de-duplication and the fix, and the improvisation is where guard edits and repeat effects
  creep in. One reviewed pattern with fixed obligations and two exits — a mitigation for a
  human, or a draft PR for review — makes the response reproducible and keeps every effect
  inside what the roles are already allowed to do.
wave: 1
depends: ["graph-execution/02"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
  - "docs/streams/graph-execution/spec.md §2.1 (the third, signal-triggered pattern: contain → confirm and de-duplicate → root-cause with a mandatory replay fixture → propose; two exits) and §2.2 (a generated graph must not grant itself permissions)"
  - "graph-execution/02 — the workflow-pattern schema document (planned under spec/), the node contract, the `pattern-effect-exceeds-role` lint, and the evidence/effect vocabulary this pattern instantiates"
  - "topology.yaml (`apps:` role names desk / reviewer / verifier / worker — the only roles a node here may point at) and docs/enforcement-model.md (what a guard is and why a pattern never edits one)"
  - "Replit, trace-to-PR and the human ship/wait/drop decision, https://youtu.be/J8XxVnqUjYE?t=260 and ?t=292 (the propose exit never ships by itself)"
  - "freshness-checked 2026-09-16 @ d96fd3ba: `ls spec/workflow-patterns/` — directory absent (02 creates it); `grep -rn 'signal-triggered' spec docs statusgen` returns zero hits"
exec-tier: strong
exec-tier-why: "(a) the containment taxonomy and the outage-vs-regression exit rule are design decisions the facts bound but do not fully pre-specify; (c) a node that is allowed one effect too many survives every happy-path test and is exactly the fault the lint row must catch"
domain: complicated
consumers:
  - "spec/workflow-patterns/ (the pattern bank 02 creates gains a third file): follow-up graph-execution/08 (this brief; flips to fixed-here when the pattern file lands)"
  - "plugins/assay/skills/ (the desk procedures that would DISPATCH into this pattern): out-of-scope (this brief ships the reviewed pattern definition only; wiring a desk to dispatch on a live signal is a later brief that first needs the signal-liveness precondition delivered)"
version: 1
id: 3482df1a-a5e1-4cd5-9c8d-0d423c2541ae
---

# Brief 08 — Signal-triggered pattern

## Context
files: `spec/workflow-patterns/signal-triggered-v1.yaml` (planned), `statusgen/testdata/patterns/signal-triggered/` (planned) — the valid instance plus the mutated copies the negative rows load), `statusgen/patterns_test.go` (planned) — 02's test file, which gains this pattern's cases, `spec/workflow-pattern-v1.md` (planned) — gains one paragraph: how a signal, not an authored item, instantiates a pattern), `docs/dependency-graph-design.md` (one paragraph naming the third pattern), `changelog/graph-execution-08-signal-triggered-pattern.md` (planned)

facts:
- **Schema and lint come from 02.** The file is an instance of `spec/workflow-pattern-v1.md` (planned) — node kinds `artifact | check | decision | effect`, evidence kinds `command | review | witness | observe`, effects that point at a `topology.yaml` role and are refused by `pattern-effect-exceeds-role` when they exceed that role's binding. This brief adds no schema key; if the schema cannot express an obligation below, the gap is reported as NEEDS_CONTEXT against 02, not patched here.
- **Four obligations, in order, each a node.** (1) **contain** — an `effect` node whose permitted effects are EXACTLY the fixed taxonomy `quarantine | rollback | secondary-auth | stop`, or the recorded outcome `none needed` carrying the signal reading that justifies it; a guard-config edit is never a permitted effect of any node in this pattern. (2) **confirm + de-duplicate** — a `check` node recording the signal, its source and every open item it matches; permitted effects: file an item or attach to an existing one, nothing else. (3) **root-cause** — an `artifact` node whose required evidence is a replay fixture that reproduces the regression on the pre-fix revision (`command` evidence, `mandatory: true`); permitted effects: a worktree only. (4) **propose** — an `effect` node producing a draft PR that carries the fixture as its test plus a findings-register entry; never merge, never deploy.
- **Two exits, one selector.** A `decision` node after confirm reads the signal class: `outage` ⇒ the exit is a mitigation left for a human to execute (the pattern's last node is a `decision` owned by a human, no automated effect); `regression` ⇒ the exit is the propose node's draft PR into the ordinary review lane. Both exits end at a human; the pattern never removes one.
- **Signal liveness is a precondition, not a deliverable.** A dead signal source makes the pattern report nothing and look healthy. The pattern's first node declares `input: signal {source, liveness-check}` and the eligibility evaluator (01) holds the instance when the liveness check is `could-not-check`; delivering the liveness checks themselves is outside this brief and outside this stream.
- **Roles.** contain / confirm / root-cause / propose run as `worker`; the outage exit is a human `decision`; review of the propose exit is `reviewer`. Every App role a node names (`worker`, `reviewer`) exists in `topology.yaml`'s `apps:` list. `human` is not one of those roles and is never looked up there: `topology.yaml`'s `humans:` list ships empty by design, and its entries — when populated — carry `name`/`login`/`id`, never `role:`. The outage decision's human owner is a fixed node kind this pattern itself declares, not a topology-resolved role.
- Single-point-of-failure note: the ONE control is the `pattern-effect-exceeds-role` lint on the pattern file. Second, independent layer: the pattern's own containment taxonomy is a closed enumeration checked by a separate schema rule (an unknown effect value fails parsing before the role check runs). Third, out-of-band: the roster's role binding at run time refuses an effect the role is not bound to, in a different component (the desk tools) on a different signal.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Public tree: every example signal, repo and item in the fixture uses `example-org/*` names.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Author `spec/workflow-patterns/signal-triggered-v1.yaml` (planned) per the facts: the four obligation nodes, the selector decision node, the two exits, the `signal` input with its liveness check, roles per the facts, every effect enumerated from the closed taxonomy.
2. Fixture directory with the valid instance and three mutated copies: (a) contain node with an added `guard-config-edit` effect; (b) root-cause node with the replay-fixture evidence marked `mandatory: false`; (c) propose node with a `merge` effect. Tests assert the lint reddens each with the rule name.
3. One paragraph in `spec/workflow-pattern-v1.md` (planned) on signal-instantiated patterns and the liveness precondition; one paragraph in `docs/dependency-graph-design.md` naming the third pattern.
4. Changelog fragment.

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +flow | cd statusgen && go run . patterns --lint --root .. ; echo rc=$? | tail -1 | `rc=0` — the signal-triggered file passes the same lint 02's two patterns pass |
| 2 | check:ci +mutation | cd statusgen && go test -run 'TestSignalTriggeredGuardEditIsRefused' ./... | exit 0; the mutated copy (a) reddens with `pattern-effect-exceeds-role` naming the contain node |
| 3 | check:ci +mutation | cd statusgen && go test -run 'TestSignalTriggeredFixtureMandatory' ./... | exit 0; mutated copy (b) reddens — a root-cause node without a mandatory replay fixture is refused |
| 4 | check:ci | cd statusgen && go test -run 'TestSignalTriggeredNoMergeEffect' ./... | exit 0; mutated copy (c) reddens — the propose exit cannot merge |
| 5 | check +dereference | grep -cE -e '^\s+- quarantine$' -e '^\s+- rollback$' -e '^\s+- secondary-auth$' -e '^\s+- stop$' spec/workflow-patterns/signal-triggered-v1.yaml | count 4 — the taxonomy in the shipped file is exactly the four steps, none added, none dropped |
| 6 | check +dereference | for r in $(grep -oE 'role: [a-z]+' spec/workflow-patterns/signal-triggered-v1.yaml \| sort -u \| cut -d' ' -f2); do [ "$r" = human ] \&\& continue; grep -q "role: $r" topology.yaml \|\| echo "unknown role $r"; done | prints nothing — every non-human role the pattern names exists in `topology.yaml`'s `apps:` list, checked against that file's actual `role: <name>` entries. `human` is skipped, not looked up: `topology.yaml`'s `humans:` list ships empty by design (its header: "SHIPS EMPTY, and that is the fail-closed direction") and its entries, when populated, carry `name`/`login`/`id` keys — never `role:` — so the outage decision's human owner is never resolved against `topology.yaml`; it is the one node owner this pattern fixes structurally rather than by topology lookup |
| 7 | check | statusgen --root . --consumers --brief assay:assay:graph-execution:08 --diff-base origin/main; echo rc=$? | tail -1 | `rc=0` on the implementing branch — the pattern-bank follow-up flipped to fixed-here is corroborated by the diff |
| 8 | check | statusgen --root . --lint; echo rc=$? | tail -1 | `rc=0` |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code, output line(s) or hash, date, runner). "verified" requires this section filled by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer question this brief must answer: for every App role the shipped pattern file actually
writes as `role: <name>` (never the outage decision's human owner, which row 6 deliberately
skips rather than resolves against `topology.yaml`), does row 6 still fail on a genuinely unknown
role — has the reviewer run it against a copy with a typo'd role to confirm the row can fail,
not merely that it passes on the correct file?
