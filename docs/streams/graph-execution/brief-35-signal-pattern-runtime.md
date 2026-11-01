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
unblocks: ["auto-triage/02", "auto-triage/03"]
effort: M
gate: human
gate-why: >-
  Task 10 releases graph-execution/34's activation hold, the rule that a signal pattern is a
  reviewed definition only and MUST NOT be activated. The change adds a refusal and has no caller,
  but lifting that hold is the owner's call, and each activating brief then relies on what it says.
decision-trigger: creation
decision-issue: 5147
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [5144, 5147]
schema: brief-v2
version: 1
id: 31425dd0-3363-46ef-8c59-b73f491dbde3
authored: 2026-11-01 by the-desk (author-brief)
sources:
  - "medici-finance/assay#5144 — the gap report: no brief owns refusing a signal that fails admission, sending a class down its route, or holding on a failed liveness check; it asks for a row showing the runtime refusing with the lint bypassed"
  - "https://github.com/medici-finance/assay/issues/4506#issuecomment-6455243468 — the ruling that containment and confirm-filing are human-owned decision nodes. That a hold must not stand in their way is this brief's reading of it, stated in the ground rules"
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
  - "spec/workflow-patterns/signal-triggered-v1.yaml (planned): out-of-scope (graph-execution/08 authors it and activates nothing, so it carries no edge to this brief; the gate reads whatever pattern a caller names, and refuses or holds when it cannot read one)"
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
pattern runs. Two other layers sit around it, each in a different component on a different signal.
Upstream, 34's schema and pattern lint refuse a pattern that declares a bad predicate, at authoring
time, without seeing any run. Out of band, the roster's role binding still decides whether a forge
write is authorised. Separately, Task 10's spec text requires a human gate on every activating
brief, and auto-triage/02 and /03 carry one today.

**What the gate refuses with no lint run, and nothing beyond this:** a file whose sha256 is not the
expected one, a schema other than `workflow-pattern-v1`, no signal input or more than one, a duplicate key,
a second document, an unknown key in a signal class, its `admission` or the signal input, an empty
`source`, an exit naming no node, a class with no exit or half of an exit pair, an `admission` wider
than the compiled floor, a class with no `admission`, an undeclared or unlisted class, a trigger or
ref its class doesn't admit, and a node off the route (rows 1, 2, 4, 16 and 17).

**What it does not enforce at run time:** `pattern-remedy-set-open` (the remedy constant and the
`refused-paths` floor), `pattern-replay-not-mandatory`, `pattern-effect-exceeds-role`,
`pattern-effect-target-not-owned`, `pattern-containment-set`, `pattern-human-role-not-decision`,
`pattern-judgement-no-default`, `pattern-decision-options`, `pattern-culprit-record`, and the rest of
`pattern-signal-input` (entry-node placement, a non-empty `liveness-check`, exact class coverage,
disconnected nodes, and the record a `mechanical-exit` implies). A pattern with only those faults
passes the gate, so the lint and review are the only layers for them. The pin (Task 1) limits a run
to the revision its caller expected, and that is as far as it goes. The gate's decoder is separate
from the lint's so that one parser bug can't defeat both. The cost is that the two can read one file
differently. Strict decoding (Task 1, row 17) closes the duplicate-key, second-document and unknown-key
cases, within the objects it covers. Row 12 shows the two agree on one fixture, and no row compares them
on every pattern.

**What no layer here catches:** a caller that never calls the gate, or that passes a false value for
any input it takes: trigger, ref, liveness state and source, class, culprit class, node, root,
pattern name or expected digest. Each activating brief binds every one of them to a value the run cannot
choose, and its review checks that. Most come from its platform or reviewed source. Liveness can't: it is
an observation made at run time, so a value fixed in source is not a result, and the gate can't tell the
two apart. A run that can write its root can change the file the gate reads, so the expected digest has to
come from outside any root the run can write, and reading it from a writable checkout doesn't count.
auto-triage/01's `origin` check stands behind trigger and ref for the gate-red class.
Nothing stands behind the other inputs, or behind any input of another class, which is why activation
stays out of this brief.

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
- **No brief owns a liveness result for a signal source.** `git grep -n -i liveness 4de8a95cb -- docs/streams/auto-triage`
  returns only auto-triage/04's watchdog's own liveness. `git grep -n -i -E 'liveness.*signal|signal.*liveness'`
  over the other streams' briefs returns claim, repair and trust-account senses of those words. /08 places each source's
  check outside its stream. A responder running because a gate went red shows that source fired once,
  not that it would report the next red, and no brief argues otherwise, so this brief does not claim
  gate-red's source is live by construction. Until a brief owns a result, a caller following
  Task 10's text has none to pass and the gate holds. A caller passing a constant `live` would get past
  the gate, which can't tell a constant from an observation. Task 10 rules that out in the spec, and the
  activating brief's human gate is what checks it. #5148 tracks the missing owner.
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
  weight: "verbs +1 (`deskdispatch --signal-gate`), flags +11, refusals +8 reason tags, hold reasons +3, rule-text lines +~8 in one spec"
  why-add: "The lint sees no run, so it cannot own this. /01 is brief-level and reads no pattern. /14 is human-gated, has no signal scope and carries seven prerequisite briefs unrelated to signals. The auto-triage stream's provenance predicate covers one class. Folding this into EvaluateAgenticAdmission was considered and rejected: that function decides from an assessment of a work subject, and a signal run's question is about its origin and route. Removal has nothing to offer here: no existing mechanism does this job."

## Human decision

Decision-trigger: creation. What is being decided is whether a run-time check may lift the rule that a signal-triggered workflow pattern is a reviewed definition only and must not be activated.

The check refuses a run that fails its class's admission, leaves its class's route or reads an unexpected revision of the pattern, and holds a run whose source is dead or unchecked. It trusts its caller for every input, enforces only a listed subset of the authoring rules, and has no owner yet for a liveness result. A caller following the spec has no result to pass until one exists, so every such run holds. The check can't tell an observed result from a constant.

Why it matters: this rule is the only thing keeping signal patterns from going live, and later activations will rely on what replaces it.

Options:

1. Lift the rule once the check is implemented, for any activating change that carries its own human gate, binds every caller input to a value the run cannot choose and names a liveness source. (Recommended.)
2. Lift it for the gate-red class only. The outage and regression classes stay held until a further ruling.
3. Keep the rule. The check is built, and activation needs a separate ruling later.

Default if no answer: none — blocks until answered.

## Ground rules

- Blocked until #5147 carries a human ruling. Task 10 is written for option 1; a ruling for option 2
  or 3 changes its text, so read the ruling first. The design-decision record `spec/lifecycle-v1.md`
  §4.4 requires doesn't exist yet: only a human ruling can produce one, and no one may write it ahead of that.
- NEVER git push, trigger workflows or run mutating kubectl. Leave commits per the task instructions only.
- Stop at `implemented`; you do not set verified or done.
- **Activate nothing.** No workflow, skill, desk procedure or loop calls the gate in this brief, and no
  live signal is read. Verify row 11 checks this for callers named in the tree.
- Do not call `statusgen`, its lint or its parser from the gate, and do not add a `statusgen` import
  to `signalgate.go`. Independence from the lint is the point of the layer (Verify row 10).
- No new effect, permission or role. Do not widen §7 of the pattern spec or `humanFloorOperations`.
- No second scheduler and no persisted run state. Effect limits (`dedupe`, `open-ceiling`, `rate`) are
  not in this brief: the activating brief owns them (auto-triage/02 and /03 for gate-red).
- Liveness probes are not in this brief, and no brief owns one yet (#5148). `liveness-check` is prose a
  verifier reads; a caller passes a result only once a brief owns that source's probe, and a constant
  or otherwise unprobed value is not one. An absent result holds, and that hold stands until then.
- **A hold never stops a person.** The ruling makes containment and confirm-filing human-owned decision
  nodes: https://github.com/medici-finance/assay/issues/4506#issuecomment-6455243468
  This brief reads that to mean a hold must not stand in their way, since a source can be dead during
  the very outage they respond to. A refuse or hold stops the calling
  automation's next step. It never blocks a person acting, including at a `human` decision node.
  The gate has no queue, retry or escalation: the activating brief names who sees a held run.
- Public tree: every source and repository in a fixture uses `example-org/*` names.
- If 34 has not reached `implemented` on main when you start, or its shipped keys differ from the
  Task below, report NEEDS_CONTEXT; do not implement 34's keys here.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, do not guess.

## Task

1. **Load and pin.** `LoadSignalPattern(path, expectedSHA256) (SignalPattern, error)` in `signalgate.go`
   decodes only what the gate reads: `schema`, `pattern`, `version`, `signal-classes`, node ids, the
   node's `signal` input, and `edges`. Reuse `admissionMaxPatternLen`. An empty or malformed expected
   digest is `Refused` with `pattern-revision` before any read. A read failure is then `Unverifiable`.
   The file is read once: its bytes are hashed and compared, a difference is `Refused` with
   `pattern-revision`, and only those same bytes are decoded. The gate holds no state, so this is how
   one run stays on one revision: the caller passes the same expected digest for every node, and the
   digest it passes comes from outside any root the run can write. Reading it from a writable checkout
   doesn't count. A bad document is `Refused`. The loader refuses, whatever the lint said, a pattern with:
   - a duplicate key (`yaml.v3` refuses one by default, and row 17 pins that), a second document in the
     file, including an empty one after a trailing `---` (read past the first with a `Decoder`; anything
     but `io.EOF` refuses), or a key outside 34's closed list in a signal class, its `admission` or the
     signal input. Unknown keys elsewhere are ignored: the gate decodes only what it reads, and a list of
     every schema key would tie it to each later schema change, so the schema and lint own those. A key
     34 adds to one of the three objects later is refused here until this list gains it;
   - a schema other than `workflow-pattern-v1`, or no signal input, or more than one;
   - an empty signal-input `source`;
   - an `exit`, `mechanical-exit` or `judgement-exit` that names no node, a class with no exit, or a
     class naming one of `mechanical-exit` and `judgement-exit` without the other, since some culprit
     class would then have nowhere to go;
   - an `admission.triggers` value outside `schedule`, `push`, or `push` whose `push-refs` is anything
     but exactly `[refs/heads/main]`. This floor is compiled in and is 34's rule restated, so a
     pattern that widens its own predicate cannot widen the gate. It covers `admission` only. Its
     refusal carries `admission-floor`; the other refusals in this list carry `pattern-invalid`.
2. **Admission.** Given a class id and a run origin `{trigger, ref}`, refuse wholesale when the class is
   undeclared or not listed in the signal input's `classes`; when the class has no `admission`, whether
   or not any node declares effects; when the trigger is not in the class's `triggers`; or when the
   trigger is `push` and the ref is not byte-equal to a `push-refs` entry. A bare `main` and
   `refs/tags/main` are both refused. A refused run has no lower-confidence path. The no-`admission`
   refusal is stricter than 34's lint, which lets a class omit `admission` in a pattern declaring no
   effects: such a pattern passes the lint and is refused here. The origin has no repository term and
   the ref is read only on `push`, so a fork's run or a scheduled run on another branch is not this
   gate's to refuse. Each activating brief refuses those itself.
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
   `hold` with one reason tag from a closed set, evaluated in this order: revision and load (Task 1),
   admission, route, liveness. A refusal therefore beats a hold, so a run that fails admission on a dead
   source is refused and never queued for retry. Tags: `pattern-revision`, `pattern-invalid`, `class-unknown`, `admission-missing`, `admission-trigger`,
   `admission-ref`, `admission-floor`, `route-off`; for a hold, `liveness-dead`,
   `liveness-could-not-check`, `pattern-unreadable`. The record holds pattern, version, file digest,
   class, origin, node, selected exit, verdict and reason.
7. **CLI mode.** Add `deskdispatch --signal-gate` to `cli.go`, `perfVerbs` and the executor:
   `--root DIR --pattern NAME --class ID --trigger T --ref REF --node ID [--pattern-sha256 HEX] [--culprit-class C] [--liveness STATE] [--liveness-source S] [--json]`.
   `NAME` resolves to `<root>/spec/workflow-patterns/<NAME>-v1.yaml`; a name holding a path separator is
   refused, and so is a file whose `pattern:` differs from `NAME`. Exit `ExitOK` on admit,
   `ExitRefused` on refuse, `ExitUnverifiable` on hold. A missing or unreadable pattern is a hold, once a
   well-formed digest is given. An absent `--pattern-sha256` is a refuse with `pattern-revision`, even if
   the file is missing too. A missing required flag or an unknown flag is a usage error: nonzero, no verdict.
   **Exit 0 is the only proceed.** A caller stops on every other result: refuse, hold, usage error,
   `ExitDisabled`, `ExitRateLimited`, a missing binary or a crash. No path other than an admit may exit 0.
8. **Fixtures** in `testdata/signalgate/`: `good-signal.yaml`, the same shape as 34's
   `good-signal-example.yaml`, and these lint-refused variants, one edit each:
   `lint-bad-pull-request.yaml` (`pull_request` added to a class's `triggers`),
   `lint-bad-branch-ref.yaml` (`push-refs: [refs/heads/feature]`),
   `lint-bad-no-admission.yaml` (`admission` removed from `example-alert`),
   `lint-bad-two-inputs.yaml` (a second signal input on `contain`),
   `lint-bad-exit-unknown.yaml` (`example-alert`'s `exit` names no node). Six more, outside row 3's list:
   `no-effects-no-admission.yaml` (no node declares effects and one class has no `admission`),
   `half-pair.yaml` (`judgement-exit` removed from `example-gate-red`),
   `empty-source.yaml` (the signal input's `source` is empty),
   `duplicate-key.yaml` (`exit` twice in `example-alert`),
   `two-documents.yaml` (a second document after `good-signal.yaml`'s content),
   `unknown-class-key.yaml` (an unknown key in `example-alert`).
9. **Tests**, each named in Verify. Add `signalgate-mutations.json` with a failing control and one mutant
   per branch of Task 1-6, in the shape of `admission-mutations.json`.
10. **Spec.** In `spec/workflow-pattern-v1.md` §4.3 replace the sentences saying no runtime exists and
    the hold on "a dispatch brief" with: the gate's checks and their verdict order; the faults it
    refuses, and that it does not enforce the other lint rules at run time; that a dead or could-not-check
    source holds; and these statements in these words, which row 18 reads: a signal pattern "MUST NOT be
    activated except through the signal gate"; an activating brief "MUST carry its own human gate"; the
    caller binds "trigger, ref, liveness, class, culprit class, node, root, pattern and expected digest"
    each to "a value the run cannot choose", where liveness is "an observation made at run time, never a
    value fixed in source" and the expected digest comes "from outside any root the run can write, and
    reading it from a writable checkout does not count", and the others come from its platform or reviewed
    source; a caller "MUST proceed only on the admit exit"; a hold "never blocks a person"; an activating
    brief "MUST name the source whose result it passes and the brief that owns that result, and a constant
    or otherwise unprobed value is not a liveness result". Keep the statement that limits are the
    activating implementation's. Name no particular brief in the spec text.
11. **Changelog.** Add `changelog/graph-execution-35-signal-gate.md` (planned) with `### Added` and one `- ` bullet.

## Verify

Rows run from the repository root on the implementation head, in a POSIX shell. Exit 3 means
could-not-check (setup failed); exit 4 means the named test or finding was missing. Neither is a pass.
Fail-first: on the base no named test exists, so rows 1, 2, 4-8, 16 and 17 exit 1 or 4; rows 3, 10, 11
and 12 exit 3 on the missing file or fixture; row 18 exits 4 on the missing text.

| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateAdmission$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateAdmission (' "$T/out" \|\| exit 4)` | exit 0; on a well-formed pattern, `pull_request`, a `push` to a non-main ref, a bare `main`, `refs/tags/main` and an undeclared class are each refused with their tag, and a positive case (`schedule`; `push` to `refs/heads/main`) admits |
| 2 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateLintBypassed$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateLintBypassed (' "$T/out" \|\| exit 4)` | exit 0; each `lint-bad-*.yaml` fixture, fed straight to the gate with no lint run, is refused: a `pull_request` run against `lint-bad-pull-request.yaml` is refused although the pattern lists that trigger, so removing the compiled floor fails this test. These five faults are what this row proves, and no others |
| 3 | check:ci | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; for f in pull-request:pattern-signal-admission-open branch-ref:pattern-signal-admission-open no-admission:pattern-signal-admission-open two-inputs:pattern-signal-input exit-unknown:pattern-signal-input; do n=${f%%:*}; r=${f#*:}; rm -rf "$T/r"; mkdir -p "$T/r/spec/workflow-patterns"; cp "tools/desk/internal/deskkit/testdata/signalgate/lint-bad-$n.yaml" "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r" > "$T/out" 2>&1; [ $? -eq 1 ] \|\| exit 4; grep -Fq "[$r]" "$T/out" \|\| exit 4; done)` | exit 0; the lint refuses each fixture on its named rule with the gate bypassed, so for these five faults rows 2 and 3 prove two layers that each hold alone |
| 4 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateRoute$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateRoute (' "$T/out" \|\| exit 4)` | exit 0; a node off the class's route is refused with `route-off` and a node on it passes; a cycle in `edges` terminates |
| 5 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateExitSelect$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateExitSelect (' "$T/out" \|\| exit 4)` | exit 0; only the exact culprit class `mechanical` reaches `mechanical-exit`; an absent, `judgement` or unknown value is refused there and routes to `judgement-exit` |
| 6 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateLiveness$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateLiveness (' "$T/out" \|\| exit 4)` | exit 0; `dead`, absent, an unknown state and a `live` result naming another source each hold; only `live` for the declared source continues |
| 7 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateOrder$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateOrder (' "$T/out" \|\| exit 4)` | exit 0; a run failing admission on a dead source is refused, not held; an off-route node on a dead source is refused, not held |
| 8 | check:ci +flow | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateCLI$' ./cmd/deskdispatch/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateCLI (' "$T/out" \|\| exit 4)` | exit 0; the mode reads a pattern from a temporary `<root>/spec/workflow-patterns/`, through the real parser and `executeMode`, and exits 0 on admit, 5 on refuse and 6 on hold, including a missing pattern (6), a path-separator name (5) and a `pattern:` mismatch (5). Dropping each required flag in turn, or passing an unknown flag, exits nonzero with no verdict, and an absent `--pattern-sha256` exits 5, so only an admit exits 0 |
| 9 | check:ci +mutation | `cd tools/desk && GOWORK=off go run ./cmd/muhar -spec internal/deskkit/signalgate-mutations.json` | exit 0; baseline passes, the failing control is caught and every mutant is caught, with no survivors |
| 10 | check:ci | `(cd tools/desk \|\| exit 3; test -f internal/deskkit/signalgate.go \|\| exit 3; d=$(GOWORK=off go list -deps ./internal/deskkit/) \|\| exit 3; if printf '%s\n' "$d" \| grep -Fxq github.com/medici-finance/assay/statusgen; then exit 1; fi; grep -n -e 'medici-finance/assay/statusgen' internal/deskkit/signalgate.go; e=$?; [ "$e" -eq 1 ] \|\| { [ "$e" -eq 0 ] && exit 1; exit 3; }; grep -ln -e 'lintPatternFile' -e 'parsePatternDoc' internal/deskkit/*.go; e=$?; [ "$e" -eq 1 ] && exit 0; [ "$e" -eq 0 ] && exit 1; exit 3)` | exit 0; the lint's package is not in deskkit's dependencies, the gate's file imports no `statusgen` package, and no deskkit file names the lint's parser. A grep error exits 3, never a pass. A helper elsewhere in deskkit that re-implements the lint under another name is not caught here: review covers that |
| 11 | check | `(test -f tools/desk/internal/deskkit/signalgate.go \|\| exit 3; git grep -n -e '--signal-gate' -e 'EvaluateSignalGate' -e 'LoadSignalPattern' -- . ':!docs' ':!changelog' ':!spec' ':!tools/desk/README.md' ':!tools/desk/internal/deskkit/signalgate*' ':!tools/desk/cmd/deskdispatch'; e=$?; [ "$e" -eq 1 ] && exit 0; [ "$e" -eq 0 ] && exit 1; exit 3)` | pre-merge only, on the implementing branch: exit 0; nothing outside the gate's own files and docs names the mode or its exported functions, across `.github`, `plugins`, `.cursor`, `tools/` and scripts. A grep error exits 3. Two limits: this row goes red by design once an activating brief lands a caller, and it cannot see a workflow that activates a pattern without calling the gate at all |
| 12 | check:ci +neighbour | `(T=$(mktemp -d); (cd statusgen && GOWORK=off go build -o "$T/sg" .) \|\| exit 3; mkdir -p "$T/r/spec/workflow-patterns" && cp tools/desk/internal/deskkit/testdata/signalgate/good-signal.yaml "$T/r/spec/workflow-patterns/" \|\| exit 3; "$T/sg" patterns --lint --root "$T/r")` | exit 0; output contains "1 checked-clean, 0 checked-failed": the gate's good fixture is one the lint accepts, so the two read one contract |
| 13 | check:ci +neighbour | `cd tools/desk && GOWORK=off go test -count=1 ./cmd/deskdispatch/ ./internal/deskkit/` | exit 0; the existing dispatch and admission suites still pass |
| 14 | check | `statusgen --root . --consumers --brief assay:assay:graph-execution:35 --base "$(git merge-base HEAD refs/remotes/origin/main)"` | pre-merge only, on the implementing branch with its merge-base pinned: exit 0, and every `follow-up graph-execution/35` entry has flipped to `fixed-here` and is corroborated by the diff |
| 15 | check | `statusgen --root . --lint` | exit 0; output contains "LINT: PASS" |
| 16 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGatePatternPin$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGatePatternPin (' "$T/out" \|\| exit 4)` | exit 0; an absent, malformed or differing expected digest is refused with `pattern-revision`, an absent one even when the file is missing; a right digest admits; a file changed between two calls with the same expected digest admits the first and refuses the second, so a run cannot change its own route by rewriting the pattern |
| 17 | check:ci +mutation | `(cd tools/desk && T=$(mktemp -d) && GOWORK=off go test -count=1 -v -run '^TestSignalGateUnlintedEdges$' ./internal/deskkit/ > "$T/out" 2>&1; e=$?; [ "$e" -eq 0 ] \|\| exit "$e"; grep -Fq -- '--- PASS: TestSignalGateUnlintedEdges (' "$T/out" \|\| exit 4)` | exit 0; `no-effects-no-admission.yaml` is refused with `admission-missing` for its class with no `admission`, and `half-pair.yaml`, `empty-source.yaml`, `duplicate-key.yaml`, `two-documents.yaml` and `unknown-class-key.yaml` are refused with `pattern-invalid`, so these six shapes fail closed |
| 18 | check:ci | `(f=spec/workflow-pattern-v1.md; test -f "$f" \|\| exit 3; T=$(mktemp) \|\| exit 3; tr '\n' ' ' < "$f" \| tr -s ' ' > "$T" \|\| exit 3; for p in 'MUST NOT be activated except through the signal gate' 'MUST carry its own human gate' 'trigger, ref, liveness, class, culprit class, node, root, pattern and expected digest' 'a value the run cannot choose' 'an observation made at run time, never a value fixed in source' 'from outside any root the run can write, and reading it from a writable checkout does not count' 'MUST proceed only on the admit exit' 'never blocks a person' 'MUST name the source whose result it passes and the brief that owns that result, and a constant or otherwise unprobed value is not a liveness result'; do grep -Fq -- "$p" "$T" \|\| exit 4; done; for p in 'until a dispatch brief' 'no runtime in this version'; do if grep -Fq -- "$p" "$T"; then exit 1; fi; done)` | exit 0; §4.3 carries Task 10's six statements and the binding's words for liveness and the digest, across line wraps, and 34's hold wording is gone |

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
| A run rewrites the pattern in its own root to change its route | row 16, when the expected digest comes from outside that root; each activating brief's review checks where it comes from |
| A class with no `admission`, or half an exit pair, slips past a rule keyed on declared effects | row 17 |
| A duplicate key, a second document or a misspelt class key makes the gate and the lint read different patterns | row 17, for those shapes. Other differentials between the two decoders aren't caught: the note says so |
| A caller reads a usage error, a disabled tool or a missing binary as "not refused" and proceeds | row 8 shows only an admit exits 0; the contract notes on auto-triage/02 and /03 say the same, and no Verify row on those briefs tests it (a Verify change there is the owner's) |
| The spec keeps 34's hold, or lands without the human gate or the liveness owner statement | row 18 |
| A caller skips the gate or passes a false value for any input | not caught here: each activating brief's review, and auto-triage/01's `origin` check for trigger and ref on gate-red. The single-point-of-failure note states the gap |
| An activating brief passes no liveness result | the gate holds (row 6). No brief owns a source today (#5148), so that hold stands |
| An activating brief passes a constant or unprobed `live` | not caught by the gate, which can't tell it from an observation. Row 18 shows the spec forbids it, and the activating brief's human gate and review are what catch it |
| A class with no limits owner goes live | not caught here: limits are the activating brief's, and only auto-triage/02 and /03 own them today |

## Evidence

<!-- appended at implementation time, one row per Verify item, by someone who did not implement -->

## Review

Gate: human (from frontmatter), and #5147 carries the decision. All four risk answers are no: the change
adds a refusal, grants no effect or permission, reads no withheld content and reverts cleanly. The gate is
human because Task 10 lifts 34's activation hold. `signalgate.go` sits under the
`tools/desk/internal/deskkit/` security-path trigger, so `statusgen --lint` raises `risk-files-crossread`
on this brief. The reviewer answers:
(1) What single control stands between a bad run and a node running? This gate's verdict. Is that
acceptable, given that it trusts its caller for trigger, ref, liveness, class, culprit class, node, root,
pattern and expected digest, that each activating brief must bind them, and that the gate can't tell an
observed liveness result from a constant?
(2) Does a row prove a lower layer catches the fault with the upper layer bypassed? For five listed faults,
yes: row 2 feeds lint-refused patterns to the gate with no lint, row 3 feeds the same files to the lint
with no gate, and row 10 shows the two share no code. For the rules the note lists as not enforced at run
time, no: the lint and review are the only layers.
