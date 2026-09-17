---
brief: assay:assay:auto-triage:01
title: "Culprit identifier — parse a failing whole-tree gate into check + file:line + mechanical/judgement class"
why: >-
  Every automated response to a red all-stop gate is only as good as the first step:
  knowing WHICH check failed, on WHICH file:line, and whether the fix is mechanical or a
  judgement call. Today a human reads the log and answers those by hand every time
  (#611, #1119, #612). This brief builds the read-only classifier that answers them from
  CI output, and — critically — knows which reds it must never claim to have pinned
  (leak-sweep withholds its detail by design). It is the foundation the responders stand on.
wave: 1
depends: []
unblocks: ["auto-triage/02", "auto-triage/03"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [611, 1119, 612]
schema: brief-v2
authored: 2026-09-16 by the-desk auto-triage authoring session
sources:
  - "docs/streams/auto-triage/spec.md — the four-step response; §5 what the identifier can and cannot see"
  - "medici-finance/assay#611 — gofmt nit in one test file reddening unrelated Verify rows (mechanical class exemplar)"
  - "medici-finance/assay#1119 — gofmt drift tripping unrelated Verify-table rows (mechanical class exemplar)"
  - "medici-finance/assay#612 — load-induced timing flake fails the whole-module test first-attempt (judgement class exemplar)"
  - "freshness-checked 2026-09-16 @ e9fa19d3 (origin/main) — no tools/autotriage/ exists; .github/workflows/ carries ci.yml (build-test/plugin-shell-suites/skillslint), truth-suite.yml, changelog-check.yml, pin-consistency.yml, plugin-drift.yml, assay-statusgen.yml, and leaksweep-pattern.yml (posts the leak-sweep commit status)"
exec-tier: strong
exec-tier-why: >-
  (a) the mechanical-vs-judgement classification is a design decision the facts do not fully
  pre-specify, and (b) correctness depends on parsing heterogeneous CI output formats and on
  the cross-cutting rule that some reds (leak-sweep) must never be classed mechanical.
version: 1
id: 2e97241c-23a2-4269-9e1e-a5eb081680c6
---

# Brief 01 — culprit identifier

## Context

single-point-of-failure: this brief IS the head control — the classifier's output is what
every responder acts on. It is read-only (it files nothing, opens nothing), so a bug here
cannot itself damage `main`; its blast radius is realized only through briefs 02/03, which
carry their own layers (draft-only + human merge, the watchdog). The one property this
brief must get right on its own is the **conservative default**: ambiguous or opaque reds
classify as `judgement`, never `mechanical`.

files:
- `tools/autotriage/` (new Go package) — the parser + classifier and its tests.
- `tools/autotriage/testdata/` (new) — captured CI-output fixtures (gofmt failure, a
  statusgen `PROBLEM` line, a `go test` failure, a leak-sweep red status, a timing-flake
  log), each a real-shaped sample with any withheld/sensitive content redacted to a
  synthetic placeholder.

facts:
- Input: the failing gate's output as CI exposes it — a job log (`gh run view --log`
  shape) for `ci`/`truth-suite`/etc., or a commit-status description for `leak-sweep`.
  The brief parses text it is GIVEN; acquiring it (log fetch vs status read) is the
  responders' job, not this brief's.
- Output schema (the seam 02/03/04 consume): `{check: <workflow/job name>, file: <path>,
  line: <int|null>, problem: <one-line diagnostic>, class: mechanical | judgement,
  origin: {trigger: push | schedule, ref: <branch/sha the run executed against>}}`. This
  schema is the interface contract; changing it later is a shared-value change owed to a
  follow-up brief. `origin` is REQUIRED on every emitted `Culprit` — it is not descriptive
  metadata, it is the field 02/03/04 gate on before they act (see the eligibility bound,
  below), and BOTH sub-fields are gated on: `trigger` alone is not sufficient (see below).
- Mechanical class = deterministic, low-judgement fixes: a gofmt diff (#611/#1119), a
  stray/undeleted file reddening a tree gate, a stale string. Judgement class = a flake
  (#612), a duplicated abstraction (#536), or any red whose remedy needs a human/design
  call.
- **leak-sweep is NEVER mechanical.** Its status carries only "withheld content detected";
  the token and file:line are withheld by design. The classifier emits `check: leak-sweep,
  file: null, class: judgement` and NEVER attempts to reconstruct the culprit from a diff.
- **Eligibility bound (input provenance).** The identifier accepts CI output ONLY from a
  `push`-**to-`main`** or `schedule`-triggered run — never from a `pull_request`-triggered
  run, and never from a `push` run against any ref other than `main`. A `pull_request` run's
  log/diagnostic text is contributor-authored, on a contribution the repository does not
  control, and turning that text into a `file:line` a responder will later EDIT (brief 02) or
  into a `problem` string a responder will PUBLISH (brief 02/03) is exactly the step that
  needs a trust boundary. The same reasoning binds the ref half: a `push` to a non-`main`
  branch can itself be a contributor-controlled push (a branch the repository does not treat
  as protected), so trusting the trigger alone and ignoring which ref it landed on would
  reopen the identical trust gap through a side door — the schema carries `origin.ref`
  specifically so this half of the check has something to gate on, not as unused descriptive
  metadata. A run is eligible only when (`trigger == schedule`) OR (`trigger == push` AND
  `ref` resolves to `main`); every other combination — `pull_request`, any non-push/schedule
  trigger, or a `push` whose `ref` is not `main` — is REFUSED wholesale: no `Culprit` is
  emitted for it at all, not a lower-confidence one — the same all-or-nothing posture the
  leak-sweep rule already applies to opaque content.
- Conservative default: any red the parser cannot resolve to a concrete `file:line` + a
  known-deterministic remedy classifies `judgement`. A false `mechanical` is the expensive
  error (it later opens a wrong draft PR); a false `judgement` merely asks a human.

## Ground rules
- NEVER git push / trigger workflows / run mutating commands. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` / `FINDINGS.md` on a branch.
- This brief is READ-ONLY by construction: it parses and classifies. It MUST NOT file an
  issue, open a PR, or write to any forge. Any such capability belongs to 02/03, behind
  their human gate.
- Fixtures carry NO real withheld/secret content — redact leak-sweep-style samples to a
  clearly-synthetic placeholder.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Create the `tools/autotriage/` Go package with a `Classify(input CIOutput) Culprit`
   surface emitting the output schema in `facts:`.
2. Implement parsers for the readable reds: gofmt diff (names file), `go vet`/`go test`
   failure (names file + line), and a statusgen `PROBLEM` line (`[tag]` + path). Each
   yields `check`, `file`, `line`, `problem`.
3. Implement the classifier: map each parsed culprit to `mechanical` or `judgement` per the
   rules in `facts:`, with the conservative default for anything unresolved.
4. Implement the leak-sweep path: recognise the `leak-sweep` status, emit
   `class: judgement, file: null`, and assert (in code + test) that it can never return
   `mechanical`.
5. Implement the eligibility check as `trigger == schedule OR (trigger == push AND ref ==
   main)`: reject on `pull_request`, any other non-push/schedule trigger, AND on a `push`
   whose `ref` does not resolve to `main` — one predicate covering both halves of `origin`,
   never a trigger-only check.
6. Add `testdata/` fixtures for each class and a test per fixture asserting the exact
   classified output.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/autotriage && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd tools/autotriage && go test ./...` | exit 0; all tests green |
| 3 | `cd tools/autotriage && go test -run TestClassifyGofmtFixture -v` | exit 0; output shows the parsed `check`, the exact `file:line` from the fixture, and `class=mechanical` — dereferences the classifier against a real gofmt-failure fixture (not a presence count) |
| 4 | `cd tools/autotriage && go test -run TestLeakSweepNeverMechanical -v` | exit 0; the leak-sweep fixture classifies `judgement` with `file` null, and the test asserts the classifier returns non-mechanical for it — the negative-path row proving the opaque-red rule holds |
| 5 | `cd tools/autotriage && go test -run TestClassifyFlakeFixture -v` | exit 0; the #612-style timing-flake fixture classifies `judgement` (a wrong-but-plausible classifier that called it `mechanical` fails this row) |
| 6 | `cd tools/autotriage && go test -run TestUnresolvedDefaultsJudgement -v` | exit 0; an unparseable/unknown red defaults to `judgement`, proving the conservative default |
| 7 | `cd tools/autotriage && go test -run TestRefusesPullRequestTriggeredRun -v` | exit 0; a well-formed, otherwise-cleanly-parseable gofmt-diagnostic input whose `origin.trigger` is `pull_request` produces NO `Culprit` at all — proves the identifier refuses on PROVENANCE, independent of how cleanly the content parses (a classifier that trusted content over origin fails this row) |
| 8 | `cd tools/autotriage && go test -run TestRefusesPushToNonMainRef -v` | exit 0; a well-formed, otherwise-cleanly-parseable gofmt-diagnostic input whose `origin.trigger` is `push` but whose `origin.ref` is a non-`main` branch (e.g. a feature branch) produces NO `Culprit` at all — proves the identifier gates on `ref` as well as `trigger`; a classifier that accepts any push regardless of ref fails this row |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit, output line(s)/hash, date, runner). "verified" requires this filled
     by someone who did NOT implement. -->

## Review
Gate: model (read-only classifier; no autonomous write). Reviewer confirms the seam schema
is stable enough for 02/03/04 to build on, that it carries `origin` as a required field with
both `trigger` and `ref` actually consumed, and that Verify rows 4, 6, 7 and 8 (leak-sweep
never mechanical; unresolved defaults to judgement; a `pull_request`-triggered run is refused
wholesale; a `push` run against a non-`main` ref is refused wholesale) actually discriminate —
a classifier that over-claims `mechanical`, that trusts content over provenance, or that gates
on trigger alone while ignoring `ref`, must fail them.
Verdict + date in the stream README table.
