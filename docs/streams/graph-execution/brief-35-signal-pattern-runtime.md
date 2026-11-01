---
brief: assay:assay:graph-execution:35
title: Runtime gate for signal patterns - admission refusal, class routes and the liveness hold
why: >-
  A signal pattern can state which runs it admits, which nodes each class runs and how a dead
  source is detected, but nothing reads any of that when a run starts. The lint only checks what
  a pattern declares, so a contributor-triggered run, a run wandering off its class's route or a
  run on a dead source would go unchallenged, and no signal pattern can safely go live. This
  brief builds the one check every activating brief calls before a node runs. It activates
  nothing itself.
wave: 2
depends: ["graph-execution/34"]
unblocks: ["graph-execution/08", "auto-triage/02", "auto-triage/03"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [5144]
schema: brief-v2
version: 1
id: 31425dd0-3363-46ef-8c59-b73f491dbde3
authored: 2026-11-01 by the-desk (author-brief)
sources:
  - "medici-finance/assay#5144 — the gap report: no brief owns refusing a signal that fails admission, sending a class down its route, or holding on a failed liveness check; it asks for a row showing the runtime refusing with the lint bypassed"
  - "docs/streams/graph-execution/brief-34-signal-input-and-signal-classes.md — Task steps 3, 6 and 7: the key shapes, the route definition and the exit selection this gate reads, and the activation hold it discharges"
  - "docs/streams/graph-execution/brief-08-signal-triggered-pattern.md — the first shipped pattern a live activation would read"
  - "docs/streams/auto-triage/brief-02-mechanical-responder.md, docs/streams/auto-triage/brief-03-judgement-responder.md — the live gate-red exits, which own their own limits, dry-run default and human gate"
  - "tools/desk/internal/deskkit/admission.go (LoadPatternRiskInput) — the precedent for a dispatch-side reader of a pattern file that does not rely on the lint"
  - "freshness-checked 2026-11-01 @ 4de8a95cb (origin/main): see the facts below; each names the command it came from"
exec-tier: strong
exec-tier-why: "(a) the verdict order and the fail-closed reading of an unlinted pattern are design decisions the facts bound but do not fully fix; (b) a wrong admit is silent, because every happy-path test still passes"
domain: complicated
outcome: none
consumers:
  - "tools/desk/internal/deskkit/signalgate.go (planned), tools/desk/cmd/deskdispatch/cli.go, tools/desk/cmd/deskdispatch/main.go: follow-up graph-execution/35 (this brief; flips to fixed-here when the implementation edits them)"
  - "spec/workflow-pattern-v1.md: follow-up graph-execution/35 (this brief; §4.3 is 34's text, amended here to name the gate)"
  - "tools/autotriage/ responders (planned): follow-up auto-triage/02 and auto-triage/03 (each carries the edge to this brief and calls this gate before a live effect)"
  - "spec/workflow-patterns/signal-triggered-v1.yaml (planned): out-of-scope (graph-execution/08 authors it; the gate reads whatever pattern a caller names, and refuses or holds when it cannot read one)"
  - "tools/desk/internal/deskkit/graphadmission.go (planned): out-of-scope (graph-execution/14 owns that seam, and its text has no signal scope; a path from that seam to a signal run would be an activating brief, which depends on this one)"
  - "statusgen/eligibility.go: out-of-scope (brief-level eligibility; reads no pattern and gains no signal scope)"
  - "statusgen/instance.go, schemas/workflow-instance-v1.json: out-of-scope (this brief produces no instance; each call decides from its own inputs)"
  - "plugins/assay/skills/: out-of-scope (no desk procedure calls the gate until a brief activates a pattern; Verify row 11 holds this)"
---

# Brief 35 — Runtime gate for signal patterns

## Context

files: `tools/desk/internal/deskkit/signalgate.go` (planned),
`tools/desk/internal/deskkit/signalgate_test.go` (planned),
`tools/desk/internal/deskkit/signalgate-mutations.json` (planned),
`tools/desk/internal/deskkit/testdata/signalgate/` (planned),
`tools/desk/cmd/deskdispatch/cli.go`, `tools/desk/cmd/deskdispatch/main.go`,
`tools/desk/cmd/deskdispatch/signalgate_test.go` (planned),
`spec/workflow-pattern-v1.md`,
`changelog/graph-execution-35-signal-gate.md` (planned).

single-point-of-failure: the ONE control is this gate's verdict, read before a node of a signal
pattern runs. Two layers stand on either side of it, each in a different component on a different
signal. (1) Upstream, 34's schema and pattern lint refuse a pattern that declares a bad predicate,
at authoring time, without seeing any run. (2) This gate refuses a run, at run time, reading the
pattern with its own decoder and its own floor, so a pattern that reached it unlinted is still
refused (Verify row 2 runs the gate with the lint bypassed; row 3 runs the lint with the gate
bypassed). (3) Out of band, the roster's role binding still decides whether a forge write is
authorised, and every activating brief carries its own human gate. **What no layer here catches:**
a caller that never calls the gate, or that passes a false trigger, ref or liveness state. The
gate takes those as inputs. Each activating brief binds them to values its platform supplies and
a model cannot choose, and its review checks that. auto-triage/01's own `origin` check stands
behind this for the gate-red class. Nothing stands behind it for any other class, which is why
activation stays out of this brief.

facts (4de8a95cb, 2026-11-01), each from the command named:
- **No reader exists.** `git grep -n -i -E 'signal-classes|liveness-check' -- '*.go' '*.json' '*.yaml'`
  returns zero hits. `grep -c 'pattern\|signal\|admission' statusgen/eligibility.go` returns 0.
  `ls -d tools/autotriage` fails: it does not exist yet.
- **No existing brief owns this.** `grep -n -i signal` over `brief-01-*.md` and `brief-14-*.md` in
  this stream returns zero hits, so #5144's reading of /01 and /14 stands. `grep -n -i 'signal'` over
  `tools/desk/cmd/deskdispatch/*.go` (non-test files) returns only the review-queue, model-keyed and
  process-signal senses of the word.
- **Only deskkit reads a pattern on the dispatch side.** `grep -ln 'LoadPatternRiskInput\|EvaluateAgenticAdmission'`
  over `tools/desk/cmd/*/*.go` and `tools/desk/internal/*/*.go` lists only `admission.go` and
  `admission_test.go`. `LoadPatternRiskInput` bounds the file size, returns `Unverifiable` on a read
  failure and `Refused` on a bad document, and does its own check "independent of the pattern lint".
- **Exit classes.** `tools/desk/internal/deskkit/exitcodes.go`: `ExitRefused` = 5, `ExitUnverifiable` = 6.
- **Mode shape.** `tools/desk/cmd/deskdispatch/main.go` `perfVerbs` registers flag-shaped modes
  (`--stamp-only`, `--check-verifier`, `--attest-verifier`) beside the `dispatch:*` verbs.
- **File naming.** `spec/workflow-pattern-v1.md` §2: a pattern file is `spec/workflow-patterns/<pattern>-v1.yaml`.
- **34 is not on main yet.** Its keys are specified in 34's Task, not shipped: the facts above show
  no reader or schema key. This brief reads 34's keys as 34 specifies them.

layering: one new rule file in `deskkit`, with one thin CLI mode as its adapter. `signalgate.go` holds
the pure verdict (Task 2-5) and one file read (Task 1); `deskdispatch` parses flags and maps the
verdict to an exit code. `tools/desk/internal/` cannot be imported from another module, and
`tools/autotriage/` is planned as its own package, so activating briefs call the CLI mode. No store,
no scheduler and no forge call: the gate is stateless and is called again for each node, so a source
that dies mid-run holds the next node.

design-fit:
  owner: tools/desk/internal/deskkit (dispatch-side pattern reading, beside admission.go)
  contract: none — the signal-class contract is spec-owned (spec/workflow-pattern-v1.md §4.3) with no semantic-owner index row
  retires: []
  weight: "verbs +1 (`deskdispatch --signal-gate`), flags +9, refusals +7 reason tags, hold reasons +3, rule-text lines +~6 in one spec"
  why-add: "The lint sees no run, so it cannot own this. /01 is brief-level and reads no pattern. /14 is human-gated, has no signal scope and carries seven prerequisite briefs unrelated to signals. The auto-triage stream's provenance predicate covers one class. Folding this into EvaluateAgenticAdmission was considered and rejected: that function decides from an assessment of a work subject, and a signal run's question is about its origin and route. Removal has nothing to offer here: no existing mechanism does this job."

## Ground rules

- NEVER git push, trigger workflows or run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented`; you do not set verified or done.
- **Activate nothing.** No workflow, skill, desk procedure or loop calls the gate in this brief, and no
  live signal is read. Verify row 11 holds this.
- Do not call `statusgen`, its lint or its parser from the gate, and do not add a `statusgen` import
  to `signalgate.go`. Independence from the lint is the point of the layer (Verify row 10).
- No new effect, permission or role. Do not widen §7 of the pattern spec or `humanFloorOperations`.
- No second scheduler and no persisted run state. Effect limits (`dedupe`, `open-ceiling`, `rate`) are
  not in this brief: the activating brief owns them (auto-triage/02 and /03 for gate-red).
- Liveness probes are not in this brief. `liveness-check` is prose a verifier reads; the caller
  runs its own probe and passes the result. An absent result holds.
- Public tree: every source and repository in a fixture uses `example-org/*` names.
- If 34 has not reached `implemented` on main when you start, or its shipped keys differ from the
  Task below, report NEEDS_CONTEXT; do not implement 34's keys here.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Load.** `LoadSignalPattern(path) (SignalPattern, error)` in `signalgate.go` decodes only what the
   gate reads: `schema`, `pattern`, `version`, `signal-classes`, node ids, node `effects`, the node's
   `signal` input, and `edges`. Reuse `admissionMaxPatternLen`. A read failure is `Unverifiable`; a
   bad document is `Refused`. It records the file's sha256 for the verdict record. The loader refuses,
   whatever the lint said, a pattern with:
   - a schema other than `workflow-pattern-v1`, or no signal input, or more than one;
   - an `exit`, `mechanical-exit` or `judgement-exit` that names no node, or a class with no exit;
   - an `admission.triggers` value outside `schedule`, `push`, or `push` whose `push-refs` is anything
     but exactly `[refs/heads/main]`. This floor is compiled in and is 34's rule restated, so a
     pattern that widens its own predicate cannot widen the gate. Its refusal carries
     `admission-floor`; every other load refusal carries `pattern-invalid`.
2. **Admission.** Given a class id and a run origin `{trigger, ref}`, refuse wholesale when the class is
   undeclared or not listed in the signal input's `classes`; when any node is effect-bearing (non-empty
   `effects`, whatever its kind) and the class has no `admission`; when the trigger is not in the
   class's `triggers`; or when the trigger is `push` and the ref is not byte-equal to a `push-refs`
   entry. A bare `main` and `refs/tags/main` are both refused. A refused run has no lower-confidence path.
3. **Exit selection.** A class naming `exit` uses it. A class naming `mechanical-exit` and
   `judgement-exit` selects from the culprit class the caller passes: exactly `mechanical` selects
   `mechanical-exit`; anything else, absent included, selects `judgement-exit` (34 §4.3).
4. **Route.** Compute the run's route over `edges` with visited sets, for the selected exit E and the
   signal-input node S: (nodes reachable from S ∩ nodes that can reach E) ∪ nodes reachable from E.
   This is 34's route narrowed to the exit step 3 selected. A node off it is refused. On 34's
   `good-signal-example.yaml`, `example-alert` at `propose` is refused and at `judge` is on route;
   `example-gate-red` with no culprit class at `root-cause` is refused, and with `mechanical` it is on route.
5. **Liveness.** Take a result `{source, state}`. `live` for the pattern's declared `source` continues.
   `dead` holds. An absent result, any other state, or a result naming another source holds as
   could-not-check. The gate has no cache and accepts no earlier verdict as input.
6. **Verdict.** `EvaluateSignalGate(SignalPattern, SignalRun) SignalVerdict` returns `admit`, `refuse` or
   `hold` with one reason tag from a closed set, evaluated in this order: admission, route, liveness.
   A refusal therefore beats a hold, so a run that fails admission on a dead source is refused and never
   queued for retry. Tags: `pattern-invalid`, `class-unknown`, `admission-missing`, `admission-trigger`,
   `admission-ref`, `admission-floor`, `route-off`; for a hold, `liveness-dead`,
   `liveness-could-not-check`, `pattern-unreadable`. The record holds pattern, version, file digest,
   class, origin, node, selected exit, verdict and reason.
7. **CLI mode.** Add `deskdispatch --signal-gate` to `cli.go`, `perfVerbs` and the executor:
   `--root DIR --pattern NAME --class ID --trigger T --ref REF --node ID [--culprit-class C] [--liveness STATE] [--liveness-source S] [--json]`.
   `NAME` resolves to `<root>/spec/workflow-patterns/<NAME>-v1.yaml`; a name holding a path separator is
   refused, and so is a file whose `pattern:` differs from `NAME`. Exit `ExitOK` on admit,
   `ExitRefused` on refuse, `ExitUnverifiable` on hold. A missing or unreadable pattern is a hold.
8. **Fixtures** in `testdata/signalgate/`: `good-signal.yaml`, the same shape as 34's
   `good-signal-example.yaml`, and these lint-refused variants, one edit each:
   `lint-bad-pull-request.yaml` (`pull_request` added to a class's `triggers`),
   `lint-bad-branch-ref.yaml` (`push-refs: [refs/heads/feature]`),
   `lint-bad-no-admission.yaml` (`admission` removed from `example-alert`),
   `lint-bad-two-inputs.yaml` (a second signal input on `contain`),
   `lint-bad-exit-unknown.yaml` (`example-alert`'s `exit` names no node).
9. **Tests**, each named in Verify. Add `signalgate-mutations.json` with a failing control and one mutant
   per branch of Task 1-6, in the shape of `admission-mutations.json`.
10. **Spec.** In `spec/workflow-pattern-v1.md` §4.3 replace the sentences saying no runtime exists and
    the hold on "a dispatch brief" with: the gate's three checks, its verdict order, that a dead or
    could-not-check source holds, and that a signal pattern MUST NOT be activated except through this
    gate, with the caller binding trigger and ref to values its platform supplies. Keep the statement that
    limits are the activating implementation's. Name no brief in the spec text.
11. **Changelog.** Add `changelog/graph-execution-35-signal-gate.md` (planned) with `### Added` and one `- ` bullet.

## Verify

Rows run from the repository root on the implementation head, in a POSIX shell. Exit 3 means
could-not-check (setup failed); exit 4 means the named test or finding was missing. Neither is a pass.
Fail-first: on the base no named test exists, so rows 1, 2 and 4-8 exit 1 or 4.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateAdmission$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateAdmission (' "$T/out" \|\| exit 4)` | exit 0; on a well-formed pattern, `pull_request`, a `push` to a non-main ref, a bare `main`, `refs/tags/main` and an undeclared class are each refused with their tag, and a positive case (`schedule`; `push` to `refs/heads/main`) admits |
| 2 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateLintBypassed$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateLintBypassed (' "$T/out" \|\| exit 4)` | exit 0; each `lint-bad-*.yaml` fixture, fed straight to the gate with no lint run, is refused: a `pull_request` run against `lint-bad-pull-request.yaml` is refused although the pattern lists that trigger, so removing the compiled floor fails this test |
| 3 | check:ci | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; for f in pull-request:pattern-signal-admission-open branch-ref:pattern-signal-admission-open no-admission:pattern-signal-admission-open two-inputs:pattern-signal-input exit-unknown:pattern-signal-input; do n=${f%%:*}; r=${f#*:}; rm -rf "$T/r"; mkdir -p "$T/r/spec/workflow-patterns"; cp "tools/desk/internal/deskkit/testdata/signalgate/lint-bad-$n.yaml" "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; [ $? -eq 1 ] \|\| exit 4; grep -Fq "[$r]" "$T/out" \|\| exit 4; done)` | exit 0; the lint refuses each fixture on its named rule with the gate bypassed, so rows 2 and 3 prove two layers that each hold alone |
| 4 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateRoute$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateRoute (' "$T/out" \|\| exit 4)` | exit 0; a node off the class's route is refused with `route-off` and a node on it passes; a cycle in `edges` terminates |
| 5 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateExitSelect$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateExitSelect (' "$T/out" \|\| exit 4)` | exit 0; only the exact culprit class `mechanical` reaches `mechanical-exit`; an absent, `judgement` or unknown value is refused there and routes to `judgement-exit` |
| 6 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateLiveness$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateLiveness (' "$T/out" \|\| exit 4)` | exit 0; `dead`, absent, an unknown state and a `live` result naming another source each hold; only `live` for the declared source continues |
| 7 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateOrder$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateOrder (' "$T/out" \|\| exit 4)` | exit 0; a run failing admission on a dead source is refused, not held; an off-route node on a dead source is refused, not held |
| 8 | check:ci +flow | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateCLI$' ./cmd/deskdispatch/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateCLI (' "$T/out" \|\| exit 4)` | exit 0; the mode reads a pattern from a temporary `<root>/spec/workflow-patterns/`, through the real parser and `executeMode`, and exits 0 on admit, 5 on refuse and 6 on hold, including a missing pattern (6), a path-separator name (5) and a `pattern:` mismatch (5) |
| 9 | check:ci +mutation | `cd tools/desk && GOWORK=off go run ./cmd/muhar -spec internal/deskkit/signalgate-mutations.json` | exit 0; baseline passes, the failing control is caught and every mutant is caught, with no survivors |
| 10 | check:ci | `(test -f tools/desk/internal/deskkit/signalgate.go \|\| exit 3; ! grep -n -e 'medici-finance/assay/statusgen' -e 'lintPatternFile' -e 'parsePatternDoc' tools/desk/internal/deskkit/signalgate.go)` | exit 0; the gate imports nothing from the lint it stands behind (3 = file missing) |
| 11 | check:ci | `(test -f tools/desk/internal/deskkit/signalgate.go \|\| exit 3; ! git grep -n -e '--signal-gate' -- .github plugins)` | exit 0; no workflow or shipped skill calls the gate, so this brief activates nothing (3 = file missing) |
| 12 | check:ci +neighbour | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp tools/desk/internal/deskkit/testdata/signalgate/good-signal.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r")` | exit 0; output contains "1 checked-clean, 0 checked-failed": the gate's good fixture is one the lint accepts, so the two read one contract |
| 13 | check:ci +neighbour | `cd tools/desk && GOWORK=off go test -count=1 ./cmd/deskdispatch/ ./internal/deskkit/` | exit 0; the existing dispatch and admission suites still pass |
| 14 | check | `statusgen --root . --consumers --brief assay:assay:graph-execution:35 --base "$(git merge-base HEAD refs/remotes/origin/main)"` | pre-merge only, on the implementing branch with its merge-base pinned: exit 0, and every `follow-up graph-execution/35` entry has flipped to `fixed-here` and is corroborated by the diff |
| 15 | check | `statusgen --root . --lint` | exit 0; output contains "LINT: PASS" |

## Pre-mortem

| Failure mode of the work | Caught by |
|---|---|
| The gate trusts the pattern's predicate, so an unlinted pattern that lists `pull_request` admits a contributor's run | row 2 |
| The gate shares the lint's parser, so one parsing bug defeats both layers | row 10 |
| Trigger is checked and ref is not, so a push to any branch passes | row 1 |
| A branch or tag named `main` satisfies `refs/heads/main` | row 1 |
| Route check accepts both exits of a two-exit class, so an unresolved culprit reaches the mechanical exit | row 5 |
| Could-not-check is read as live, or a result for another source counts | row 6 |
| A dead source holds a run that should be refused, so an inadmissible run waits for retry | row 7 |
| The evaluator is right and the mode maps its verdict to exit 0 | row 8 |
| A test passes because no assertion can fail | row 9 |
| The brief quietly activates a pattern by wiring a caller | row 11 |
| A caller skips the gate or passes a false origin or liveness state | not caught here: each activating brief's review, and auto-triage/01's `origin` check for gate-red. The single-point-of-failure note states the gap |
| A class with no limits owner goes live | not caught here: limits are the activating brief's, and only auto-triage/02 and /03 own them today |

## Evidence

<!-- appended at implementation time, one row per Verify item, by someone who did not implement -->

## Review

Gate: model (from frontmatter). All four risk answers are no: the change adds a refusal, grants no
effect or permission, reads no withheld content, reverts cleanly, and has no caller until a human-gated
brief wires one. `signalgate.go` sits under the `tools/desk/internal/deskkit/` security-path trigger, so
`statusgen --lint` raises `risk-files-crossread` on this brief. The answers stand for the reasons above,
and the reviewer should weigh them. The reviewer answers:
(1) What single control stands between a bad run and a node running? This gate's verdict. Is that
acceptable, given that it trusts its caller for origin and liveness and each activating brief must bind
those?
(2) Does a row prove a lower layer catches the fault with the upper layer bypassed? Row 2 feeds
lint-refused patterns to the gate with no lint; row 3 feeds the same files to the lint with no gate;
row 10 shows the two share no code.
