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
wave: 3
depends: ["graph-execution/02", "graph-execution/34", "graph-execution/35"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [4506]
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
  - "docs/streams/graph-execution/spec.md §2.1 (the third, signal-triggered pattern: contain → confirm and de-duplicate → root-cause with a mandatory replay fixture → propose; two exits) and §2.2 (a generated graph must not grant itself permissions)"
  - "graph-execution/02 — the workflow-pattern schema document (planned under spec/), the node contract, the `pattern-effect-exceeds-role` lint, and the evidence/effect vocabulary this pattern instantiates"
  - "topology.yaml (`apps:` role names desk / reviewer / verifier / worker — the only roles a node here may point at) and docs/enforcement-model.md (what a guard is and why a pattern never edits one)"
  - "Replit, trace-to-PR and the human ship/wait/drop decision, https://youtu.be/J8XxVnqUjYE?t=260 and ?t=292 (the propose exit never ships by itself)"
  - "per the maintainer's ruling choosing option A (schema and lint amendment first): https://github.com/medici-finance/assay/issues/4506#issuecomment-6454769886"
  - "per the maintainer's ruling on the role-effect table (no widening; containment and confirm-filing become human-owned decision nodes): https://github.com/medici-finance/assay/issues/4506#issuecomment-6455243468"
  - "graph-execution/34 — the signal input, signal classes, decision options, containment set and the lint rules this pattern instantiates; graph-execution/33 — `human` as an owner of effect-free decision nodes"
  - "freshness-checked 2026-09-16 @ d96fd3ba: `ls spec/workflow-patterns/` — directory absent (02 creates it); `grep -rn 'signal-triggered' spec docs statusgen` returns zero hits"
exec-tier: strong
exec-tier-why: "(a) the containment taxonomy and the outage-vs-regression exit rule are design decisions the facts bound but do not fully pre-specify; (c) a node that is allowed one effect too many survives every happy-path test and is exactly the fault the lint row must catch"
domain: complicated
consumers:
  - "spec/workflow-patterns/ (the pattern bank 02 creates gains a third file): follow-up graph-execution/08 (this brief; flips to fixed-here when the pattern file lands)"
  - "plugins/assay/skills/ (the desk procedures that would DISPATCH into this pattern): out-of-scope (this brief ships the reviewed pattern definition only; wiring a desk to dispatch on a live signal is a later, not-yet-authored brief, which must depend on graph-execution/35, the run-time layer that refuses a run failing a class's admission, runs only a class's route, and holds an instance whose liveness check is could-not-check)"
version: 2
id: 3482df1a-a5e1-4cd5-9c8d-0d423c2541ae
---

# Brief 08 — Signal-triggered pattern

## Context
files: `spec/workflow-patterns/signal-triggered-v1.yaml` (planned), `statusgen/testdata/patterns/signal-triggered/` (planned) — the valid instance plus the mutated copies the negative rows load), `statusgen/patterns_test.go` (planned) — 02's test file, which gains this pattern's cases, `spec/workflow-pattern-v1.md` (planned) — gains one paragraph: how a signal, not an authored item, instantiates a pattern), `docs/dependency-graph-design.md` (one paragraph naming the third pattern), `changelog/graph-execution-08-signal-triggered-pattern.md` (planned)

facts:
- **Schema and lint come from 02, as amended by 33 and 34.** The file is an instance of `spec/workflow-pattern-v1.md` — node kinds `artifact | check | decision | effect`, evidence kinds `command | review | witness | observe`, effects that point at a `topology.yaml` role and are refused by `pattern-effect-exceeds-role` when they exceed that role's binding. graph-execution/33 adds `role: human`, allowed only on `kind: decision` nodes with no effects. graph-execution/34 adds the `signal` input `{source, liveness-check, classes}`, the decision-node keys `options`, `default` and `containment`, the top-level `signal-classes` block, and the lint rules this brief's rows name. This brief adds no further schema key; if the schema cannot express an obligation below, the gap is reported as NEEDS_CONTEXT against 34, not patched here.
- **Role-effect table: not widened (maintainer's ruling, in `sources:`).** §7 stays as is (`worker`: `push`, `pr-open`, `comment`). No worker node in this pattern carries a containment effect or `file-issue`. Containment and confirm-filing are `human`-owned `kind: decision` nodes that list their outcomes and carry no effects; a person executes the chosen outcome.
- **Four obligations, in order, each a node.** (1) **contain** — a `decision` node, `role: human`, no effects, whose `containment` list is EXACTLY the fixed taxonomy `quarantine | rollback | secondary-auth | stop` plus the recorded outcome `none-needed`, which carries the signal reading that justifies it; a person executes the chosen step. A guard-config edit is never an outcome or an effect of any node in this pattern. (2) **confirm + de-duplicate** — a `check` node, `role: worker`, no effects, recording the signal, its source and every open item it matches; it is followed by a `decision` node `file`, `role: human`, no effects, with `options: [file-new, attach-existing]`, where a person files a new item or attaches to an existing one. (3) **root-cause** — an `artifact` node whose required evidence is a replay fixture that reproduces the regression on the pre-fix revision (`command` evidence, `mandatory: true`); the node outputs the reserved artifact `replay-fixture` (the name `pattern-replay-not-mandatory` looks for); permitted effects: a worktree only. (4) **propose** — an `effect` node producing a draft PR that carries the fixture as its test plus a findings-register entry; never merge, never deploy.
- **Two exits, chosen by the signal class, with no selector node.** The `signal-classes` block declares the three classes this pattern serves (`outage`, `regression`, `gate-red`); a class names its own exit, so the former selector `decision` node is dropped and the intake node's `classes` lists all three. An instance of a class runs only the nodes on that class's route (34 §4.3: the nodes on a path from the signal input to the class's exits, plus the nodes reachable from those exits). `outage` ⇒ `exit: mitigate`, a `kind: decision`, `role: human` node with no effects, `options` and a `default` (a person executes the mitigation); `mitigate` has no edge into root-cause or propose, so outage's route contains no effect-bearing node. `regression` ⇒ `exit: propose`, the draft PR into the ordinary review lane. No runtime enforces routes today (34's single-point-of-failure note), so nothing below relies on one. **Admission on every class.** propose declares effects, so 34's `pattern-signal-admission-open` requires `admission` on all three classes, outage included: the rule fails closed on the presence of any effect-bearing node in the pattern, whatever its kind and whether or not an edge reaches it, and grants no exemption for a human exit. Each declares `{triggers: [schedule, push], push-refs: [refs/heads/main]}`, the gate-red floor. So an outage or regression signal is admitted only from a scheduled run or a push to `main`; a signal from any other source (an alert from a monitoring service with no run behind it) is refused wholesale until a later amendment extends 34's trigger set. **Floor on every class.** propose declares effects, so all three classes declare `refused-paths: [.github/]` (34's floor, which binds every class once any node is effect-bearing, outage included); `outage` and `regression` carry no `remedies`, because it names no `mechanical-exit` and its draft PR is not limited to the mechanical remedy set. Each of the three classes carries `limits` (34 requires them): `outage` and `regression` use the gate-red values `{dedupe: per-culprit, open-ceiling: 3, rate: {count: 2, window: PT1H}, on-exceed: escalate}`. Both exits end at a human; the pattern never removes one.
- **Signal liveness is a precondition, not a deliverable.** A dead signal source makes the pattern report nothing and look healthy. The pattern's first node declares `input: signal {source, liveness-check, classes}`, and 34's spec text states that a runtime MUST hold the instance when the liveness check is `could-not-check`. No runtime does that today: the eligibility evaluator (01) is `done` and reads no pattern, and graph-execution/35, which builds the hold, is `todo`. That is why this pattern is a reviewed definition only and is not activated here. The hold is outside this brief; delivering each source's liveness check is outside this brief and outside this stream.
- **Roles.** confirm / root-cause / propose run as `worker`; contain, file, the outage exit and the judgement exit are `human` decisions with no effects; review of the propose exit is `reviewer`. Every App role a node names (`worker`, `reviewer`) exists in `topology.yaml`'s `apps:` list. `human` is not one of those roles and is never looked up there: `topology.yaml`'s `humans:` list ships empty by design, and its entries — when populated — carry `name`/`login`/`id`, never `role:`. The outage decision's human owner is a fixed node kind this pattern itself declares, not a topology-resolved role.
**Gate-red class, added 2026-10-18 (contract from auto-triage/01–03).** A red whole-tree gate
(a red `main` or a red scheduled check) is one signal class of this pattern. This brief defines
the class's obligations as pattern rules, checked by the same lint. The executable classifier,
the two live responders and their activation are not built here. They remain auto-triage/01
(`gate: model`), /02 and /03 (`gate: human`, design record `DR-auto-triage`), all `todo`, which
implement this contract; where the two differ, the stricter rule applies. The class's obligations:
- **Where each obligation lives (from 34).** The class is a `signal-classes` entry, `id: gate-red`. The admission predicate is its `admission` (`pattern-signal-admission-open`); the culprit record is its `culprit-record` (`pattern-culprit-record`); the mechanical exit is `mechanical-exit: propose` with `remedies` and `refused-paths`, which holds at least the floor `.github/` (`pattern-remedy-set-open`); the judgement exit is `judgement-exit`, a `human` decision with `options` and a `default` (`pattern-judgement-no-default`); the effect limits are its `limits`.
- **Gate-red input admission.** The gate-red class's `admission` admits only a run
  triggered by `schedule`, or by `push` whose ref is `refs/heads/main` (a fully qualified branch
  ref, so a tag named `main` never matches); a `pull_request` run or a
  push to any other ref is refused wholesale (no instance, not a lower-confidence one), because
  its log text is contributor-authored.
- **Culprit record.** The gate-red class's `culprit-record` (34) fixes the record a gate-red
  run produces, which the confirm node records as evidence at run time:
  `{check, file, line, problem, class: mechanical | judgement, origin: {trigger, ref}}`.
  Conservative default: anything not resolved to a concrete `file:line` with a known
  deterministic remedy is `judgement`. A gate that withholds its detail by design (a
  withheld-content leak scan) is always `judgement` with `file: null`, never reconstructed from
  a diff.
- **Mechanical exit = the propose node, narrowed.** For `mechanical`, the class's `remedies`
  limit the propose node's draft PR to the closed remedy set
  `gofmt | stray-file-deletion | exact-stale-string-replace` on the culprit file, checked against
  the canonicalized path; a remedy outside the set, or any path under `.github/workflows/` or
  other gate configuration, routes to the judgement exit. The class's `refused-paths` holds at
  least 34's floor `.github/`, a subset of "gate configuration".
  **This narrows the refused-path rule to a deny-list.** The pattern refuses the named
  gate-configuration paths and accepts every other path. auto-triage/02 instead admits only a
  canonicalized allow-list. That is stricter, and it stays 02's control at run time; this pattern
  rule is the floor any instance must meet, not a replacement for 02's allow-list.
- **Judgement exit = a human decision with a default.** For `judgement`, the exit is a human
  `decision` node that must carry a recommended default next step; a route with no default is
  invalid. Published text derived from CI output is fenced and inert.
- **Effect limits.** One open artifact per culprit (dedupe), an open-artifact ceiling and a
  per-window rate, each failing closed to an escalation signal (consumed by auto-triage/04).
- **Activation is not in this brief.** Running the pattern on a live signal means filing issues and
  opening draft PRs autonomously. That human gate lives in auto-triage/02 and /03 (`gate: human`,
  `DR-auto-triage`), which carry the dry-run default and the live adapters. Every brief that
  activates this pattern depends on graph-execution/35 and passes its gate before a node runs; auto-triage/02 <!-- graph: encoded graph-execution/35→auto-triage/02,auto-triage/03 -->
  and /03 carry that edge. This brief depends on /35 too, so the pattern file lands only once the gate that
  can refuse a run against it exists. For the gate-red class, no other activating brief exists or is
  assumed; the general desk wiring in `consumers` stays a later brief.
- Single-point-of-failure note: the ONE control is that every node able to contain or file is a `human` decision with no effects, refused otherwise by 33's schema clause and `pattern-human-role-not-decision`; `pattern-effect-exceeds-role` still bounds every worker node to §7. Second, independent layer: the containment taxonomy is a closed enumeration checked by 34's schema items enum (an unknown value fails the shape pass) and again by `pattern-containment-set`. Third, out-of-band: the roster's role binding at run time refuses an effect the role is not bound to, in a different component (the desk tools) on a different signal. For provenance, the one control is 34's `pattern-signal-admission-open`, which requires `admission` on all three classes because propose declares effects; it keys on the presence of any effect-bearing node, not on node kind, reachability or a class's route, so it does not rely on a runtime honouring class routes. No runtime layer stands behind it today (no runtime reads a signal pattern; graph-execution/35 builds one and is `todo`; auto-triage/01 will check gate-red admission only), and activation stays out of this brief.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Public tree: every example signal, repo and item in the fixture uses `example-org/*` names.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Author `spec/workflow-patterns/signal-triggered-v1.yaml` (planned) per the facts: the four obligation nodes, the `file` decision, the `outage`, `regression` and `gate-red` entries of `signal-classes` (each with `exit` or its mechanical/judgement pair, `limits`, and `admission` and `refused-paths: [.github/]`, which every class carries because propose declares effects) in place of a selector node, the two exits (the outage exit `mitigate` with no edge into root-cause or propose), the `signal` input with its liveness check, roles per the facts, the containment taxonomy as the contain node's `containment` block list.
2. Fixture directory with the valid instance and four mutated copies: (a) contain node with an added `guard-config-edit` effect, and (a2) contain node with `guard-config-edit` added to its `containment` list; (b) root-cause node with the replay-fixture evidence marked `mandatory: false`; (c) propose node with a `merge` effect. Tests assert the lint reddens each with the rule name.
3. One paragraph in `spec/workflow-pattern-v1.md` (planned) on signal-instantiated patterns and the liveness precondition; one paragraph in `docs/dependency-graph-design.md` naming the third pattern.
4. Changelog fragment.
5. (added 2026-10-18; re-placed onto 34's keys 2026-10-30) Encode the gate-red class per the "Gate-red class" facts as its `signal-classes` entry, as "Where each obligation lives (from 34)" places it: the admission predicate as the class's `admission`, the culprit record as its `culprit-record`, the closed remedy set as its `remedies` with `refused-paths` beside `mechanical-exit: propose`, the default-carrying judgement decision as its `judgement-exit`, and the effect limits as its `limits`. Nothing of the class sits on the `signal` input item or on the propose node. Add mutated copies (d) the gate-red `admission` admitting `pull_request`, (d2) its `push-refs` changed to `[refs/heads/develop]`, (e) the gate-red `remedies` gaining a value outside the closed set, and (e2) its `refused-paths` dropping `.github/`, (f) a judgement decision with no recommended default, (g) a withheld-detail gate classed `mechanical`; tests assert the lint reddens each with the rule name.

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +flow | cd statusgen && go run . patterns --lint --root .. ; echo rc=$? \| tail -1 | `rc=0` — the signal-triggered file passes the same lint 02's two patterns pass |
| 2 | check:ci +mutation | cd statusgen && go test -count=1 -v -run '^TestSignalTriggeredGuardEditIsRefused$' ./... 2>&1 \| grep -F -- '--- PASS: TestSignalTriggeredGuardEditIsRefused ' | exit 0; the mutated copy (a) reddens with `pattern-human-role-not-decision` naming the contain node (a human decision carries no effects), and (a2) reddens with `pattern-containment-set` |
| 3 | check:ci +mutation | cd statusgen && go test -count=1 -v -run '^TestSignalTriggeredFixtureMandatory$' ./... 2>&1 \| grep -F -- '--- PASS: TestSignalTriggeredFixtureMandatory ' | exit 0; mutated copy (b) reddens with `pattern-replay-not-mandatory` — a root-cause node without a mandatory replay fixture is refused |
| 4 | check:ci | cd statusgen && go test -count=1 -v -run '^TestSignalTriggeredNoMergeEffect$' ./... 2>&1 \| grep -F -- '--- PASS: TestSignalTriggeredNoMergeEffect ' | exit 0; mutated copy (c) reddens — the propose exit cannot merge |
| 5 | check +dereference | grep -cE -e '^\s+- quarantine$' -e '^\s+- rollback$' -e '^\s+- secondary-auth$' -e '^\s+- stop$' spec/workflow-patterns/signal-triggered-v1.yaml | count 4 — the taxonomy in the shipped file is exactly the four steps, none added, none dropped |
| 6 | check +dereference | for r in $(grep -oE 'role: [a-z]+' spec/workflow-patterns/signal-triggered-v1.yaml \| sort -u \| cut -d' ' -f2); do [ "$r" = human ] \&\& continue; grep -q "role: $r" topology.yaml \|\| echo "unknown role $r"; done | prints nothing — every non-human role the pattern names exists in `topology.yaml`'s `apps:` list, checked against that file's actual `role: <name>` entries. `human` is skipped, not looked up: `topology.yaml`'s `humans:` list ships empty by design (its header: "SHIPS EMPTY, and that is the fail-closed direction") and its entries, when populated, carry `name`/`login`/`id` keys — never `role:` — so the outage decision's human owner is never resolved against `topology.yaml`; it is the one node owner this pattern fixes structurally rather than by topology lookup |
| 7 | check | statusgen --root . --consumers --brief assay:assay:graph-execution:08 --base "$(git merge-base HEAD refs/remotes/origin/main)"; echo rc=$? \| tail -1 | `rc=0` pre-merge, on the implementing branch with `--base` pinned to its merge-base (a branch-only check; the post-merge run names `--delivery` and its first parent) — the pattern-bank follow-up flipped to fixed-here is corroborated by the diff |
| 8 | check | statusgen --root . --lint; echo rc=$? \| tail -1 | `rc=0` |
| 9 | check:ci +mutation | cd statusgen && go test -count=1 -v -run '^TestSignalTriggeredGateRedAdmission$' ./... 2>&1 \| grep -F -- '--- PASS: TestSignalTriggeredGateRedAdmission ' | exit 0; mutated copies (d) and (d2) redden: a gate-red input that admits a `pull_request` run, or a push to a non-`main` ref, is refused with `pattern-signal-admission-open` (contract from auto-triage/01) |
| 10 | check:ci +mutation | cd statusgen && go test -count=1 -v -run '^TestSignalTriggeredRemedySetClosed$' ./... 2>&1 \| grep -F -- '--- PASS: TestSignalTriggeredRemedySetClosed ' | exit 0; mutated copies (e) and (e2) redden: the gate-red class cannot name a remedy outside the closed set or drop the gate-configuration floor from `refused-paths`, with `pattern-remedy-set-open` (contract from auto-triage/02) |
| 11 | check:ci +mutation | cd statusgen && go test -count=1 -v -run '^TestSignalTriggeredJudgementCarriesDefault$' ./... 2>&1 \| grep -F -- '--- PASS: TestSignalTriggeredJudgementCarriesDefault ' | exit 0; mutated copy (f) reddens: a judgement decision without a recommended default is invalid, with `pattern-judgement-no-default` (contract from auto-triage/03) |
| 12 | check:ci +mutation | cd statusgen && go test -count=1 -v -run '^TestSignalTriggeredWithheldNeverMechanical$' ./... 2>&1 \| grep -F -- '--- PASS: TestSignalTriggeredWithheldNeverMechanical ' | exit 0; mutated copy (g) reddens: a withheld-detail gate can only be `judgement` with `file: null`, with `pattern-culprit-record` (contract from auto-triage/01) |

## Evidence
<!-- appended at implementation time: one row per Verify item — (command, exit code, output line(s) or hash, date, runner). "verified" requires this section filled by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer question this brief must answer: for every App role the shipped pattern file actually
writes as `role: <name>` (never the outage decision's human owner, which row 6 deliberately
skips rather than resolves against `topology.yaml`), does row 6 still fail on a genuinely unknown
role — has the reviewer run it against a copy with a typo'd role to confirm the row can fail,
not merely that it passes on the correct file?
