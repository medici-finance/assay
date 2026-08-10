---
brief: methodology/07
title: Toolkit extraction — statusgen + brief-v1 + conventions as a standalone repo
wave: 2
depends: ["methodology/01", "methodology/02", "methodology/03"]
unblocks: ["methodology/11"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-07-08 by Fable session (initiative-streams step 3)
sources: ["[I-05](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-article-3-writing-specs-that-can-converge-the-initiator-s-cr.md) (adoption prerequisite)", "[I-02](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-08-the-streams-methodology-as-a-medici-service.md)", "spec §10 (statusgen home = deliberate knob)", "../reconciler/docs/convergence-thesis.md (umbrella positioning)"]
gate-why: >-
  Publishes the toolkit as a standalone public repo — a one-way door; once adopters depend
  on it, renaming/unpublishing/relicensing breaks them, so the sign-off confirms
  name+visibility+license are permanent.
---

# Brief 07 — Toolkit extraction (standalone repo)

**CROSS-REPO: creates a NEW sibling repo; also touches `../reconciler` (its step-4
tracking refactor consumes the extracted tool).**

## Context
files: new sibling repo (renamed to `../assay-toolkit` 2026-07-09 per brief-13's Assay decision; was streams-toolkit); this repo's tools/statusgen (source of copy; NOT deleted here)
facts:
- Contents to extract: tools/statusgen (Go, already repo-agnostic), the brief-v1 template + 5 rules (from ~/.claude/skills/author-brief/SKILL.md general core), the FINDINGS/INTAKE/RETRO register conventions, and a README teaching adoption (docs/streams layout, CI workflow example).
- irreversible: yes → gate human, because the repo may be made public (published = cached/indexed forever) and its name is a naming decision. The HUMAN decides: repo name, org, public/private, license (BSL tension noted in canton-dev-fund research — surface it, don't decide it).
- This repo KEEPS its vendored tools/statusgen for now; swapping to consume the extracted one is follow-on work (mirror of reconciler-spinout's pattern), not this brief.
- `../reconciler` becomes the toolkit's second consumer (spec §10) — validate extraction by running the extracted statusgen against a `docs/streams/` scaffold there (read-only trial; do NOT restructure reconciler tracking in this brief).
- AMENDED 2026-07-09 (implementation): the trial scaffold lives at `../assay-toolkit/examples/adopter-scaffold/` (reconciler-flavored content: an identity-reconciler stream with two brief-v1 briefs + FINDINGS/INTAKE + generated STATUS.md) instead of inside `../reconciler`'s working tree — creating files there, even uncommitted, would mutate a repo this brief says to leave untouched. Verify row 4's `<reconciler-scaffold>` placeholder resolves to that path.
- AMENDED 2026-07-09 (implementation): the Go module lives at `statusgen/` (nested module, not repo-root). A root `go.work` exists for root-level `go run ./statusgen` and IDE support, but on go1.26 the bare `./...` pattern at a workspace root matches no packages, so Verify row 2's invocation is `cd ../assay-toolkit/statusgen && go test ./... && go vet ./...` (row updated to match; the CI workflow uses the same form).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task instructions only. Repo creation on GitHub / publishing is the HUMAN's action — prepare everything locally.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Scaffold the sibling repo locally (git init): statusgen module (history-free copy is fine; note provenance in its README), brief-v1 template + rules doc, register conventions doc, adoption README, example CI workflow, LICENSE placeholder flagged for human decision.
2. Make its tests self-sufficient (`go test ./...` green standalone).
3. Trial run against `../reconciler` scaffold per facts.
4. Present name/org/visibility/license options to the human; STOP before any remote creation.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `git -C ../assay-toolkit log --oneline \| wc -l` | ≥1 (local repo exists; adjust path to chosen name) |
| 2 | `cd ../assay-toolkit/statusgen && go test ./... && go vet ./...` | exit 0 (row amended 2026-07-09: nested module layout — see facts) |
| 3 | `test -f ../assay-toolkit/docs/brief-template.md && test -f ../assay-toolkit/README.md && echo ok` | prints `ok` |
| 4 | `cd ../assay-toolkit && go run ./statusgen --root examples/adopter-scaffold --check; echo $?` | 0 (trial scaffold validates; placeholder resolved 2026-07-09 — see facts) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

Implementer run (records the implementation-time result; `verified` still needs an
independent re-run by a non-implementer). Local repo only — NO remote exists; repo
creation/publication is the human's action:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `git -C ../assay-toolkit log --oneline \| wc -l` | 0 | `1` (root commit `6422a5c`, 83 files, 4523 insertions) | 2026-07-09 | implementer (coordinator + 2 dispatched agents) |
| 2 | `cd ../assay-toolkit/statusgen && go test ./... && go vet ./...` | 0 | full suite ok standalone under module `github.com/medici-finance/assay-toolkit/statusgen` (final name post-rename); gofmt clean | 2026-07-09 | implementer |
| 3 | `test -f ../assay-toolkit/docs/brief-template.md && test -f ../assay-toolkit/README.md && echo ok` | 0 | `ok` (plus docs/brief-rules.md, docs/registers.md, docs/lifecycle.md, LICENSE placeholder) | 2026-07-09 | implementer |
| 4 | `cd ../assay-toolkit && go run ./statusgen --root examples/adopter-scaffold --check; echo $?` | 0 | `0` — generated STATUS.md byte-matches; scaffold exercises frontmatter+tiering, brief-v1 files, FINDINGS/INTAKE registers | 2026-07-09 | implementer |

[F-13](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-agent-tool-subagent-dispatch-without-explicit-isolation-can-.md) dispatch check (verify, don't trust): both extraction agents were dispatched
without worktree isolation but at absolute paths into the NEW repo; post-hoc
`git log origin/main..main` on the shared checkout = empty and the toolkit repo held
only uncommitted files until the coordinator's single root commit. No stray commits.

Task 4 (human decisions) presented 2026-07-09 and since ALL DECIDED by human:<name>, same day:
org+repo = medici-finance (remote created by human:<name>; content pushed); visibility =
private for now; license = Apache-2.0 (canonical text committed); name = assay-toolkit
(renamed 2026-07-09 to follow brief-13's Assay decision — GitHub redirects the old
streams-toolkit URL; module path `github.com/medici-finance/assay-toolkit/statusgen`
and all references updated, tests green). The decision record lives in the toolkit
README "Pending decisions" section and ../reconciler/docs/naming.md.

Independent verification (non-implementer opus re-run on merged main 07bcecaa, 2026-07-09).
Run against a **fresh clone** `git clone git@github-jojig:medici-finance/assay-toolkit.git`
(dissolves the prior bounded-review caveat — the review ran the toolkit as a remote consumer
would receive it, not the local working copy). Path substitution recorded: the brief's `../`
does not resolve from this worktree, so the absolute local path is
`/Users/iholsman/jojig/Lending/assay-toolkit` and rows ran against the fresh clone:

| # | Command | Exit | Result | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `git -C <fresh-clone> log --oneline \| wc -l` | 0 | `4` (≥1; root extraction `6422a5c` + 2 decision commits + the streams-toolkit→assay-toolkit rename `e86d85a`) | 2026-07-09 | independent (opus-verifier) |
| 2 | `cd <fresh-clone>/statusgen && go test ./... && go vet ./...` | 0 | full suite ok standalone under module `github.com/medici-finance/assay-toolkit/statusgen`; vet clean | 2026-07-09 | independent (opus-verifier) |
| 3 | `test -f <fresh-clone>/docs/brief-template.md && test -f <fresh-clone>/README.md && echo ok` | 0 | `ok` | 2026-07-09 | independent (opus-verifier) |
| 4 | `cd <fresh-clone> && go run ./statusgen --root examples/adopter-scaffold --check; echo $?` | 0 | `0` — adopter-scaffold STATUS.md byte-matches; the extracted tool validates a fresh adopter's docs/streams end-to-end | 2026-07-09 | independent (opus-verifier) |

All four rows pass on the fresh clone (not the local working copy) — the extraction is
self-sufficient as published to the remote. Verification only; the `gate: human` review
(repo name/visibility/license — already decided by human:<name> per the Task-4 record above) is
separate and unchanged.

## Review
Gate: human (irreversible: yes — external publication + naming/licensing decisions).
Reviewer records verdict + date as `human:<name>` in the stream README table.
