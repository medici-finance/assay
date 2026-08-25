---
brief: iso-9001/01
title: Emit the tool-validation evidence pack as a release asset
why: >-
  The release pipeline already proves, on every release, that each refusal gate goes red when
  the error it claims to stop is injected — six mutation runs, each asserting zero survivors,
  each able to fail the release. That is the strongest available answer to the question every
  auditor asks about an automated quality gate: how do you know it works. And the answer is
  thrown away. It lives in a job log that expires, so "show me the record that check X was
  demonstrated to fire, and when" is log archaeology. Nothing has to be built: the demonstration
  already runs. This brief writes it to a file and ships the file.
wave: 0
depends: []
unblocks: ["iso-9001/03", "iso-9001/04", "iso-9001/06"]
effort: S
exec-tier: strong
exec-tier-why: >-
  The whole value of the artifact is whether its control enumeration is honest. An
  under-broad set produces an evidence pack that is confidently silent about the gates it
  omits; an over-broad one names controls nothing demonstrated. Both are worse than no pack.
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-25 (authored for the iso-9001 board)
sources:
  - "`docs/mistake-proofing.md` §3 D1: 'For every control-mode device there MUST exist a demonstration that an injected instance of the error it claims to stop actually reddens it… \"When did this last stop something?\" is a legitimate audit question for any device, and \"never, and we cannot make it\" is a finding.' This brief turns that demonstration into a record."
  - "`docs/mistake-proofing.md` §3 D6: 'a maintained, visible list of what the devices do NOT check… prefer generating them from the checks themselves'. The pack is generated from the mutation specs, never hand-typed — a hand-maintained second copy of a control list is the defect, whatever it contains."
  - "`docs/three-state-instrument-rule.md` §'Auditing a fleet against this rule' — the four-column table (Instrument / what it prints when it cannot see / States / Disposition). It is specified and has no maintained instance; this pack is the first one, scoped to the release-gated refusal paths."
  - "`docs/evidence-bundle.md` — the exit-code contract to copy verbatim in spirit: `3` = exported but INCOMPLETE, with an `omitted` array that is the artifact's own statement of what it is missing, because 'a silently incomplete compliance bundle is a worse outcome than a failed export'."
  - "`.github/workflows/release.yml` — the `test` job already runs six `muhar` sweeps and greps each report for `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate`, failing the release on a survivor. The reports are produced and discarded; this brief captures them."
  - "`tools/desk/cmd/muhar/main.go` — the spec format (`test`, `control`, `mutations[]`, each mutation carrying a human-readable `name` describing the injected error) and the exit contract (0 healthy, 2 HARNESS BROKEN and the run carries no trustworthy verdict, 1 usage/IO)."
  - "The standard-side reading: ISO 9001's monitoring-and-measuring-resources clause bites where monitoring or measuring is used to verify the conformity of products and services to requirements — a gate that can refuse a release is in scope — and asks for retained evidence of fitness for purpose plus action on prior results when a resource is found unfit."
  - "`tools/freshness/` and `tools/bugs-gc/` — the in-tree precedent for a small single-purpose tool as its own Go module under `tools/`, which is the home this brief copies rather than inventing one."
  - "freshness-checked 2026-08-25 @ 6871a3b (origin/main) — no tool-validation artifact exists in the tree, and no release step writes one."
---

# Brief 01 — the tool-validation evidence pack

## Context

single-point-of-failure: after this brief, the assembler is the only thing standing between a
partial mutation run and an evidence pack that reads complete. That is one layer, and the
countermeasure is structural rather than exhortative: the assembler is given the *declared*
control set independently of the reports it finds, so a missing report is a positive
statement in the artifact and a non-zero exit, never an absence nobody can see. The
independent second layer is unchanged and must stay — the six release steps still each fail
the release on a survivor, and this brief must not weaken or replace any of them.

files:
- `tools/toolvalidation/` (implementation home) — a new single-purpose Go module that reads
  the mutation specs plus the captured `muhar` reports and emits the pack in two formats.
- `.github/workflows/release.yml` — capture each existing `muhar` report to a file, run the
  assembler, upload the two artifacts alongside the binaries and `checksums.txt`.

facts:
- **Do not re-run the mutations.** The six sweeps already run in the `test` job and are the
  release gate. Re-running them in a second job doubles a long, CPU-bound step and creates
  two verdicts that can disagree. Capture the report the gating step already produces
  (`tee`), and assemble from the captures. Re-establish: read the six `- name: Gate bar —
  mutate …` steps in `.github/workflows/release.yml`.
- **The control enumeration is the whole judgement in this brief.** It must be an
  enumeration the assembler *declares*, not a set inferred from whatever files happen to be
  present, or a missing capture becomes a silently smaller pack. At authoring time the
  declared set is the six specs under `tools/desk/` (`git ls-files 'tools/desk/*mutations*.json'`
  returns six at `6871a3b`) and their six corresponding release steps. Adding a seventh spec
  without adding it to the declared set must be visible, so the assembler compares the
  declared set against the specs it can find on disk and reports a difference in **both**
  directions.
- **A mutation's `name` is the evidence.** Each entry in a spec carries a human-readable name
  describing the injected error ("disarm the GitHub-token arm (a `ghp_` token scans clean)").
  That string is precisely what the auditor question wants — the error that reddens the
  control — and it is already written. The pack reproduces it; it does not paraphrase it.
- **`muhar` exit 2 is not a failure of the gate, it is the absence of a verdict.** Exit 2
  means the harness is broken — a red baseline, or the positive control itself not caught —
  and the run carries no trustworthy per-guard verdict at all. The pack must render that as
  could-not-check for every control in that spec, never as pass and never as fail. This is
  the exact three-state distinction the corpus exists to keep separable, applied to the
  evidence artifact.
- **Two formats, one source.** `.json` is what a consumer parses; `.md` is what a human hands
  to an auditor. Generate both from one in-memory model in one run so they cannot disagree,
  the way the board and its views already do.
- **Scope boundary, and it must be stated in the artifact itself (D6).** This pack covers the
  desk-tools refusal gates that the release mutates. It does NOT cover the lint rule set, the
  workflow-level checks, or brief-authored rule-16 demonstrations, and it carries no re-run
  interval — a demonstration recorded here is a demonstration as at that release, not a
  standing one. Say all of that on the page. A pack that lets a reader believe it covers every
  control in the system would be worse than no pack.
- **This brief adds no compliance claim.** The pack states what was demonstrated and when; it
  does not state that anything conforms to anything. Its header carries the same disclaimer
  register as `docs/evidence-bundle.md`.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- Never cut a tag, and never dispatch the release workflow, to test this. The release path is
  exercised by its `dry_run` input, which builds and checksums and stops before tagging.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Create `tools/toolvalidation/` as its own Go module**, copying the shape of
   `tools/freshness/` (own `go.mod`, one `main.go`, tests beside it). It takes the repo root,
   a directory of captured `muhar` reports, and a release tag; it writes
   `tool-validation-<tag>.md` and `tool-validation-<tag>.json`.
2. **Declare the control set in source as a named list with a one-line rationale per entry**,
   not a glob assembled inline — a list a reviewer can read and argue with. Compare it against
   the specs found on disk and report a difference in both directions (a declared spec with no
   file; a file in no declaration).
3. **Emit one row per mutation**, carrying: the gate it belongs to, the spec path, the
   mutation's `name` (the injected error, verbatim), the verdict (`caught` / `NOT CAUGHT` /
   `could-not-mutate`), the date of the run, and the release tag. Add the four-column
   instrument view from `docs/three-state-instrument-rule.md` as a per-gate summary: what the
   gate prints when it cannot see, how many states it reports, and its disposition.
4. **Three-state the artifact itself.** A spec whose report is missing, unparseable, or whose
   harness exited 2 contributes an `omitted` entry naming the spec and the reason, and the
   assembler exits **3** — *emitted but INCOMPLETE* — rather than 0. Exit 1 stays usage/IO
   error. A pack that quietly covers five gates when six were declared is the failure mode
   this exit code exists to forbid.
5. **Wire it into `.github/workflows/release.yml` without weakening the existing gate.** Each
   of the six mutation steps keeps its own `grep` assertion and its own `exit 1`; add only a
   capture of the report to a per-spec file under a run-scoped directory. Then one assembler
   step, then upload the two files with the other release assets. The assembler's exit 3 must
   fail the release step: an incomplete evidence pack does not ship silently beside a
   complete-looking release.
6. **Write the honest header on the generated `.md`.** What it is (a record of demonstrations
   performed at this release), what it is not (an audit opinion, a conformance statement, or
   coverage of any control outside the declared set), and the two things it does not carry: a
   re-run interval, and any rule for what happens to work already verified by a control later
   found blind.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `git grep -n 'tool-validation' -- .github/workflows/release.yml` | exit 0, at least one hit — **DEREFERENCE, inverts**: returns nothing at authoring (2026-08-25 @ `6871a3b`), so a green row proves the release really emits the pack |
| 2 | `test -d tools/toolvalidation` | exit 0 — **DEREFERENCE, inverts**: the module home does not exist at authoring |
| 3 | `git ls-files 'tools/desk/*mutations*.json'` | exit 0; **exactly six paths**, unchanged from authoring — the declared control set matches the specs on disk and this brief added or removed none |
| 4 | `git grep -c 'NOT CAUGHT' -- .github/workflows/release.yml` | exit 0; a count of **at least 6** — **neighbour row (rule 17)**: the six pre-existing release-blocking assertions still stand. Six at authoring; capturing a report must not have replaced one |
| 5 | `cd tools/toolvalidation && go test ./... -count=1` | exit 0 — the assembler's tests pass |
| 6 | `cd tools/toolvalidation && go test ./... -count=1 -run MissingReportIsOmittedAndExitsThree` | exit 0 — **mutation row (rule 16), positive control**: with one declared spec's report removed, the assembler names it in `omitted` and exits 3; with every report present it exits 0. Delete the assertion and this test goes RED |
| 7 | `cd tools/toolvalidation && go test ./... -count=1 -run HarnessBrokenIsNotAPass` | exit 0 — a captured report whose harness exited 2 renders every control in that spec as could-not-check, and is distinguishable in the output from a caught mutation |
| 8 | `cd tools/toolvalidation && go test ./... -count=1 -run DeclaredSetDriftIsReportedBothWays` | exit 0 — a declared spec with no file, and a spec file in no declaration, each produce a distinct report line |
| 9 | `cd tools/toolvalidation && go build -o /tmp/tv . && /tmp/tv -root ../.. -reports testdata/complete -tag v0.0.0-test -out /tmp/tv-out; echo $?` | prints `0`, and `/tmp/tv-out` holds both a `.md` and a `.json` — the two formats are produced in one run from one model |
| 10 | `git grep -cE -e 'audit opinion' -e 'does not' -- tools/toolvalidation/` | exit 0; a non-zero count — the generated header's non-claim wording is in source, not left to the author of the release notes |
| 11 | `cd statusgen && go run . --root .. --lint` | exit 0 — the tree still lints clean, including the link check over every backticked path this brief's files cite |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter — all four risk answers no). The deliverable is a generator and
a workflow capture; nothing here is a compliance claim and nothing is irreversible — no tag is
cut and no release published by this brief. Reviewer records verdict + date in the stream
README table. Reviewer questions specific to this brief: (1) is the control set an explicit,
declared enumeration in source with drift reported in both directions, rather than a glob?
(2) does `muhar` exit 2 render as could-not-check for every control in that spec, with a test?
(3) do all six pre-existing release-blocking assertions survive unchanged — did capturing a
report replace a gate anywhere? (4) does the generated header state the scope boundary, the
absent re-run interval and the absent action-on-prior-results rule, or does it read as
broader coverage than it has?
