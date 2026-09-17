---
brief: assay:assay:graph-execution:01
title: Eligibility evaluator — gates and feathers become gating, with a reason
why: >-
  A brief can already declare "this is unsafe until X is in force" in its `gates:` field,
  and the tool reads it, validates it, and then ignores it: Next-up offers the brief anyway.
  A declared prerequisite that changes nothing is a promise the board makes and breaks
  silently. Making the declaration decide readiness — and say why — is the first step that
  lets a policy change be one YAML line instead of an edit to a desk's routing code.
wave: 0
depends: []
unblocks: ["graph-execution/05", "graph-execution/07"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by graph-execution authoring session (fable-5.1, author-brief)
sources:
  - "docs/streams/graph-execution/spec.md §1 (the reserved keys are parsed, not gating), §2 (first correctness milestone), §3 (starting points)"
  - "docs/dependency-graph-design.md §3.3 (ref grammar, alias registry, could-not-check past an unpublished alias) and §3.4 (`gates:` entries require type + reason)"
  - "statusgen/briefv2.go (edgeList, validGraphRef, the 'reserved, not gating' NOTICE in checkBriefV2Semantics); statusgen/nextup.go (eligibleBase, depIsSatisfied)"
  - "docs/three-state-instrument-rule.md (could-not-check is a third state, never pass or fail)"
  - "Furong Huang, bank-and-reuse over one fixed topology — https://youtu.be/GfmK-v8CARk?t=603 (the motivation for declarations over routing code)"
  - "freshness-checked 2026-09-16 @ d96fd3ba: `grep -rn -- '--eligibility' statusgen/*.go` returns no hits; `grep -n 'reserved, not gating' statusgen/briefv2.go` returns the two NOTICE sites at lines 446 and 449; eligibleBase gates `todo` on `b.Depends` only — not already satisfied"
exec-tier: strong
exec-tier-why: "(b) correctness is cross-artifact — the evaluator, Next-up, the drive frontier (statusgen/drivefrontier.go mirrors eligibleBase) and the lint must agree on one verdict, or a brief the board hides is offered by the frontier; (c) a wrong could-not-check default silently un-gates every cross-repo hard gate"
domain: complicated
consumers:
  - "statusgen/nextup.go (eligibleBase reads the evaluator): follow-up graph-execution/01 (this brief; flips to fixed-here when the implementation edits the path)"
  - "statusgen/drivefrontier.go (briefFrontierState mirrors Next-up eligibility): follow-up graph-execution/01 (this brief; same verdict source, same change)"
  - "statusgen/briefv2.go (the 'reserved, not gating' NOTICE retires): follow-up graph-execution/01 (this brief)"
  - "docs/dependency-graph-design.md §3.6 (schema note says gating is deferred): follow-up graph-execution/01 (this brief; the section gains the executed semantics)"
  - "plugins/assay/skills/worker-desk/SKILL.md and the-desk/SKILL.md (read Next-up; gain the held/notice vocabulary): out-of-scope (no procedure changes — the board output they read carries the new reason text; a skill wording pass is deferred to graph-execution/05's report)"
version: 1
id: a3481c87-1900-4962-9d9e-94726547bc08
---

# Brief 01 — Eligibility evaluator: gates and feathers become gating, with a reason

## Context
files: `statusgen/eligibility.go` (planned), `statusgen/eligibility_test.go` (planned), `statusgen/nextup.go`, `statusgen/drivefrontier.go`, `statusgen/briefv2.go`, `statusgen/main.go` (the `--eligibility` flag), `statusgen/testdata/eligibility/` (planned; the fixture tree), `docs/dependency-graph-design.md`, `docs/enforcement-model.md`, `docs/lifecycle.md` (§Next-up semantics), `changelog/graph-execution-01-eligibility.md` (planned)
facts:
- `gates:` and `feathers:` are parsed by `edgeList` in `statusgen/briefv2.go` into `[]GraphEdge{Ref, Type, Reason}`; every `gates:` entry requires `on`/`type`/`reason`, a `feathers:` scalar defaults to `type: build-dep`. `validGraphRef` accepts `<stream>/<NN>`, `<alias>:<stream>/<NN>`, `<alias>#<NNN>`, `#<NNN>` and `<cell>:<alias>:<stream>/<NN>`; an alias must exist in `docs/streams/graph-repos.yaml`. (Read 2026-09-16 @ d96fd3ba.)
- `checkBriefV2Semantics` emits `NOTICE: <path>: gates: N edge(s) (reserved, not gating)` — the explicit statement that nothing downstream consumes the edges. This brief retires that NOTICE.
- Next-up eligibility for a `todo` brief-v1/v2 brief is `eligibleBase` in `statusgen/nextup.go`: stream active, no `StaleRef`, no `HomedIn`, not claimed, and every `b.Depends` entry `depIsSatisfied` (target status `done` or `verified`; an unresolvable dep returns false). `gates:`/`feathers:` are not consulted. `statusgen/drivefrontier.go`'s `briefFrontierState` mirrors this rule and must keep agreeing.
- The alias registry (`docs/streams/graph-repos.yaml`) may carry `repo: null, unpublished: true` entries: a ref past such an alias is could-not-check by design (`docs/dependency-graph-design.md` §3.3), never a silent answer. The same holds when the sibling checkout an alias resolves to is absent.
- `statusgen --lint` is offline by default (`--forge` opts in to forge reads, `statusgen/main.go`); the evaluator must produce its verdicts from the tree alone, and report a forge-backed gate (`on: "#<NNN>"`, `type: human-gate`) as could-not-check offline rather than reading it open or closed.
- Single-point-of-failure note: the ONE control is the evaluator's verdict. Second layer, independent: the brief-v2 lint (`checkBriefV2Semantics`) refuses a malformed or unregistered ref before the evaluator ever sees it, so a typo cannot become a silent could-not-check. Third, out-of-band: `--lint` NOTICEs every brief `held` by a could-not-check edge, so a registry or checkout problem that holds work is visible on main's daily lint, not only in a dispatcher's output.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only.
- Public tree: every fixture, test name and example uses `example-org/*` placeholders; nothing names an adopter, a private repo or a private number.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **The evaluator** (`statusgen/eligibility.go` (planned)): `func evaluateEligibility(streams []*Stream, reg *graphRepos, root string) map[string]Eligibility` over every brief. Per brief:
   ```
   {id, verdict: eligible | held | eligible-with-notice,
    holds:   [{ref, type, reason, state}],   // from gates: (and unsatisfied depends:)
    notices: [{ref, type, state}]}           // from feathers:
   state: satisfied | unsatisfied | could-not-check
   ```
   Rules, in order: a `gates:` entry whose target is not `done`/`verified` is `unsatisfied` → `held`; a `gates:` entry that cannot be resolved (unpublished alias, absent sibling checkout, forge-backed target while offline) is `could-not-check` → `held`; a `feathers:` entry that is unsatisfied or could-not-check → `eligible-with-notice` (never held); an unsatisfied `depends:` entry is reported as a hold with `type: build-dep` so one output explains every reason a brief is not offered. `eligible` only when holds is empty. The reason text is the `gates:` entry's own `reason`, verbatim.
2. **Consumers.** `eligibleBase` in `statusgen/nextup.go` and `briefFrontierState` in `statusgen/drivefrontier.go` read the evaluator's verdict for the depends/gates decision instead of walking `b.Depends` themselves; `held` briefs are excluded from Next-up; `eligible-with-notice` briefs are offered and rendered with their notice. No score input changes (F-09 boundary: eligibility only).
3. **CLI.** `statusgen --eligibility [--json] --root <root>`: prints one line per brief (`<id>  <verdict>  <holds…>`), `--json` emits the structure above. Exit 0 on any verdict; exit 2 when the tree cannot be read.
4. **Lint.** Retire the two `(reserved, not gating)` NOTICEs in `statusgen/briefv2.go`. Add `NOTICE: [eligibility-could-not-check] <brief>: held by <ref> — could-not-check (<why>)` so a hold caused by a registry or checkout gap is visible on a full lint. Register the rule so `statusgen enforcement-status` lists it.
5. **The milestone test** (`TestEligibilityDeclarationChangesDispatch` (planned)): a fixture tree under `statusgen/testdata/eligibility/` (planned) with stream `example-a` briefs 01–03 where 02 carries `gates: [{on: example-a/01, type: behavioural-gate, reason: "01 must be in force"}]` and 01 is `todo`. Run Next-up: 02 is `held`. Copy the fixture, change ONLY that one `gates:` line (retarget it to a `done` brief), run Next-up again: 02 is `eligible`. The test asserts the two outputs differ AND that the two runs executed the same binary with no code path keyed on the stream name — the declaration alone changed dispatch.
6. **Docs.** `docs/dependency-graph-design.md` §3.6 (and a new "executed" subsection) states the gating semantics above; `docs/lifecycle.md` §Next-up semantics gains the three verdicts; `docs/enforcement-model.md` gains the evaluator row. `changelog/graph-execution-01-eligibility.md` (planned) with the highlight bullets.

## Verify
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check:ci | `cd statusgen && go test -run 'TestEligibility' ./...` | exit 0 |
| 2 | check:ci +mutation | `cd statusgen && go test -run 'TestEligibilityDeclarationChangesDispatch' -v ./...` | exit 0; the `-v` output shows the first run's verdict for `example-a/02` as `held` and the second run's as `eligible`, with the fixture diff being one `gates:` line |
| 3 | check:ci +mutation | `cd statusgen && go test -run 'TestEligibilityCouldNotCheckHolds' ./...` | exit 0; a `gates:` entry on an `unpublished: true` alias yields `state: could-not-check` and `verdict: held`; the same ref under `feathers:` yields `eligible-with-notice` |
| 4 | check | `statusgen --eligibility --json --root . \| python3 -c 'import json,sys; d=json.load(sys.stdin); print(sorted({b["verdict"] for b in d}))'` | exit 0; the printed set is a subset of `['eligible', 'eligible-with-notice', 'held']` |
| 5 | check +dereference | `statusgen --root . --lint 2>&1 \| grep -c 'reserved, not gating'` | count 0 — the NOTICE this brief retires no longer fires on the tree that carries `gates:`/`feathers:` today |
| 6 | check +flow | `statusgen --next-up --root . > /tmp/ge01-nu.txt; statusgen --eligibility --root . \| awk '$2=="held"{print $1}' > /tmp/ge01-held.txt; grep -c -F -f /tmp/ge01-held.txt /tmp/ge01-nu.txt` | count 0 — no brief the evaluator holds appears on the Next-up board (an empty held list also yields 0 and is acceptable only if row 2 passed) |
| 7 | check:ci +neighbour | `cd statusgen && go test -run TestDriveFrontier ./... && go test -run TestNextUp ./...` | exit 0 — the frontier and Next-up still agree after both read the evaluator |
| 8 | check +dereference | `grep -c 'eligible-with-notice' docs/dependency-graph-design.md docs/lifecycle.md docs/enforcement-model.md` | each file count >= 1 |
| 9 | check | `statusgen --root . --lint; echo rc=$?` | `rc=0` |
| 10 | check | `statusgen --consumers --brief graph-execution/01 --root .; echo rc=$?` | `rc=0` on the authoring branch (every entry routes follow-up or out-of-scope); exit 2 (could-not-check) on a fully merged main is acceptable and must be recorded as such, never as pass |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer question this brief must answer: does any code path in `nextup.go` or
`drivefrontier.go` still decide readiness from a field the evaluator does not read? If yes,
the milestone is not met.
