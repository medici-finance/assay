---
brief: assay:assay:desk-supervision:06
title: Workpad — one upserted progress comment per PR
why: >-
  Every worker re-dispatched onto a PR starts cold: the previous worker's plan, what it
  verified and what it found lives in a scatter of comments or nowhere, and the outward-write
  budgets exist largely to police that scatter. Symphony's agents keep exactly one marked
  comment per issue and edit it in place — plan, acceptance criteria, environment stamp,
  notes. One upserted workpad per PR gives the next shepherd the prior state, gives the
  reviewer a plan to review against, and replaces N comments with one edit.
wave: 0
depends: []
unblocks: ["desk-supervision/08"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-02 by desk-supervision authoring session
sources:
  - "OpenAI Symphony example WORKFLOW.md (elixir/WORKFLOW.md) — the `## Codex Workpad` marker, find-or-create, ignore resolved comments, environment stamp `<host>:<abs-workdir>@<short-sha>`, acceptance-criteria + validation checklists, no separate done/summary comments — https://github.com/openai/symphony/blob/main/elixir/WORKFLOW.md"
  - "tools/desk/cmd/deskreply — the worker-identity PR comment verb; posts a new comment per call; bodycheck + trust gate + outward-write budget apply"
  - "plugins/assay/skills/pr-shepherd/SKILL.md §1 — 'announce adoption with a short PR comment so the next shepherd sees YOUR claim' (a second comment shape this brief folds into the workpad)"
  - "freshness-checked 2026-09-02 @ 30c9934 — no upsert verb exists; deskreply has no edit path"
consumers:
  - "tools/desk/cmd/deskreply/main.go: fixed-here (`--workpad` upsert path under the worker identity; a plain reply is unchanged)"
  - "tools/desk/internal/deskkit/bodycheck.go: fixed-here (the workpad body passes the same bodycheck; the marker line is exempt from the slug scan by exact match)"
  - "tools/desk/cmd/deskdispatch/references/common-clauses.md: fixed-here (the workpad rule enters the common clauses so every kit carries it)"
  - "plugins/assay/skills/worker-desk/SKILL.md and pr-shepherd/SKILL.md: follow-up desk-supervision/06 (the shepherd's adoption note and the worker's progress notes become workpad edits — skill edits in the implementation PR, after the verb is proven)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md (reviewer reads the workpad's acceptance list): out-of-scope (advisory reading; no reviewer rule changes)"
version: 1
id: f800d926-c99d-4c19-9cda-1759ed81895e
---

# Brief 06 — Workpad

## Context

files:
- `tools/desk/internal/deskkit/workpad.go` (new) — the marker (`<!-- assay:workpad -->`),
  the template sections, `Render(Workpad) string`, `Parse(body) (Workpad, bool)`.
- `tools/desk/internal/deskkit/workpad_test.go` (new).
- `tools/desk/cmd/deskreply/main.go` — `deskreply --workpad --pr N --body-file F`: find
  the newest unresolved comment by the worker identity carrying the marker; edit it in
  place; else create. Never creates a second.
- `tools/desk/cmd/deskdispatch/references/common-clauses.md` — the workpad clause.
- `tools/desk/README.md` — deskreply row update.

single-point-of-failure: the marker match — behind it, the identity filter (only a comment
authored by the worker identity is a candidate, so a look-alike marker in a human's comment
is never edited) and the trust gate that already refuses every verb on an untrusted PR.

facts:
- The template sections, in order: environment stamp line (`<worktree-basename>@<short-sha>`
  — never a machine path, per the public-tree self-containment rule deskpr already
  enforces), `## Plan` (checklist), `## Acceptance criteria` (checklist, mirrors any
  `Validation` / `Test Plan` section of the brief or issue verbatim), `## Validation`
  (commands run + results), `## Notes` (blockers, pushbacks, hand-off).
- Upsert semantics: candidates are comments by the worker identity carrying the marker on
  its own line; resolved/minimised comments are ignored; newest wins; the edit replaces the
  whole body. An edit is one outward write against the same budget as a comment.
- The worker records the workpad comment id in the worktree
  (`git config --worktree assay.workpad <id>`) so a re-dispatched worker finds it without
  a search; the search is the fallback when the config is absent.
- Bodycheck and the public-repo self-containment scan apply to the workpad body exactly
  as to any comment; the marker line is matched exactly and exempt.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. `workpad.go`: marker, template, `Render`/`Parse`, and `Stamp(worktree, sha)`.
2. `deskreply --workpad`: find-or-create under the worker identity via the deskkit forge
   path; record the id worktree-locally; `--dry-run` prints `WORKPAD: would edit #<id>` or
   `WORKPAD: would create`.
3. Common-clauses kit: "keep ONE workpad per PR via `deskreply --workpad`; no separate
   done/summary comments; update it before hand-off and at every blocker".
4. Tests (fixture forge): two upserts ⇒ one comment; a human comment carrying the marker
   is never edited; a resolved worker comment is skipped and a new one created; bodycheck
   refusal on the workpad body exits 5 with nothing posted; the stamp never contains `/`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Workpad' -count=1` | exit 0; output contains `ok` |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskreply/ -run TestWorkpadUpsertIsIdempotent -v -count=1` | exit 0; output contains `--- PASS: TestWorkpadUpsertIsIdempotent` |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskreply/ -run TestWorkpadNeverEditsForeignMarker -v -count=1` | exit 0; output contains `--- PASS: TestWorkpadNeverEditsForeignMarker` |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskreply/ -run TestWorkpadBodycheckRefuses -v -count=1` | exit 0; output contains `--- PASS: TestWorkpadBodycheckRefuses` |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestWorkpadStampHasNoPath -v -count=1` | exit 0; output contains `--- PASS: TestWorkpadStampHasNoPath` |
| 6 | `cd tools/desk && GOWORK=off go build ./cmd/deskreply && ./deskreply --help` | exit 0; output contains `--workpad` |
| 7 | `grep -c 'workpad' tools/desk/cmd/deskdispatch/references/common-clauses.md` | output is `1` or more |
| 8 | `statusgen --root . --consumers --brief desk-supervision/06` | exit 0; output does not contain `DISPROVED` (run on the implementing branch: corroborates the `consumers:` routing against the diff) |

Pre-mortem → detection: "two workers each create a workpad and the PR has two" → row 2;
"a human writes the marker and the bot overwrites their comment" → row 3; "the stamp leaks
an absolute worktree path into a public PR" → row 5; "the verb exists but no kit tells a
worker to use it" → row 7.

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run 'Workpad' -count=1` | 0 | `ok  	github.com/medici-finance/assay/tools/desk/internal/deskkit` | 2026-09-02 | sonnet-5-worker |
| 2 | `cd tools/desk && GOWORK=off go test ./cmd/deskreply/ -run TestWorkpadUpsertIsIdempotent -v -count=1` | 0 | `--- PASS: TestWorkpadUpsertIsIdempotent` | 2026-09-02 | sonnet-5-worker |
| 3 | `cd tools/desk && GOWORK=off go test ./cmd/deskreply/ -run TestWorkpadNeverEditsForeignMarker -v -count=1` | 0 | `--- PASS: TestWorkpadNeverEditsForeignMarker` | 2026-09-02 | sonnet-5-worker |
| 4 | `cd tools/desk && GOWORK=off go test ./cmd/deskreply/ -run TestWorkpadBodycheckRefuses -v -count=1` | 0 | `--- PASS: TestWorkpadBodycheckRefuses` | 2026-09-02 | sonnet-5-worker |
| 5 | `cd tools/desk && GOWORK=off go test ./internal/deskkit/ -run TestWorkpadStampHasNoPath -v -count=1` | 0 | `--- PASS: TestWorkpadStampHasNoPath` | 2026-09-02 | sonnet-5-worker |
| 6 | `cd tools/desk && GOWORK=off go build ./cmd/deskreply && ./deskreply --help` | 0 | help text contains `--workpad` (3 occurrences) | 2026-09-02 | sonnet-5-worker |
| 7 | `grep -c 'workpad' tools/desk/cmd/deskdispatch/references/common-clauses.md` | 0 | `5` | 2026-09-02 | sonnet-5-worker |
| 8 | `statusgen --root . --consumers --brief desk-supervision/06` | see Notes | see Notes | 2026-09-02 | sonnet-5-worker |

Notes on row 8, both runs: BEFORE this Evidence edit landed in the diff, `--consumers`
reported `COULD-NOT-CHECK: desk-supervision/06 is not in the diff` (exit 2) — the brief's
own file was not yet part of the diff for the tool to anchor on. AFTER this edit (which
does put the brief file in the diff), it reports `0 corroborated, 0 disproved, 5
unchecked` (exit 0) — every one of the five `consumers:` entries is `UNCHECKED` with reason
`unchanged since the merge-base`, because this PR never edits the `consumers:` YAML list
itself (`statusgen/consumers.go`'s `corroborateBrief` treats an entry's TEXT, not the
files it names, as what "changed" means; the brief's frontmatter was authored — and
merged to main — with these entries already written, prospectively, before this
implementation existed, so their text necessarily predates this branch even though the
FILES they name (`main.go`, `bodycheck.go`, `common-clauses.md`) genuinely did change in
this branch's diff — confirmed directly: `git diff <merge-base>...HEAD --stat -- <path>`
shows real hunks for all three). Both runs satisfy this row's literal Expect bar (exit 0,
no `DISPROVED`); UNCHECKED is a could-not-check, reported as itself rather than rounded to
"corroborated" — see the PR body for the same finding stated for the reviewer.
### Non-implementer verifier run — 2026-09-11 sonnet-5-verifier (verify-desk dispatch), offline — **VERIFY: PASS — first non-implementer pass**

Pin: `medici-finance/assay` main `86c7d62c8081189147baf37424b602f907b139aa` (git rev-parse == gh api commits/main). Fresh clone. `gate: model`, `risk {all no}`, `irreversible: no`.

| # | Result |
|---|---|
| 1 | PASS — `ok`, 10 subtests including `TestWorkpadStampHasNoPath` |
| 2 | PASS — `TestWorkpadUpsertIsIdempotent` |
| 3 | PASS — `TestWorkpadNeverEditsForeignMarker` |
| 4 | PASS — `TestWorkpadBodycheckRefuses` |
| 5 | PASS — `TestWorkpadStampHasNoPath` |
| 6 | PASS — `--help` contains `--workpad` |
| 7 | PASS — grep count 5 |
| 8 | **FAIL as literally written** — exit 2, `COULD-NOT-CHECK: no brief-v1 file`. Same root cause as `apps-installer/01`/`composability/00`/`desk-supervision/03` (`assay#822`): a later flag-day migration (`bb2079bd`, 2026-09-10, AFTER this brief's Evidence was recorded 2026-09-02) rewrote every brief's `brief:` field to the colon form, breaking the row's literal id string. Re-ran with the corrected colon id: resolves to the benign expected outcome (`COULD-NOT-CHECK: ... is not in the diff` at exit 0), matching the implementer's original reasoning. Independently re-confirmed the underlying substantive claim via `git diff <merge-base>...HEAD --stat` on the three named files — real hunks for all three, exactly as the implementer's Evidence describes. |

**Substance checks (traced actual code and tests, not just names):**
1. `TestWorkpadNeverEditsForeignMarker` — genuine: constructs a comment from a non-worker login carrying the exact marker plus a worker-authored one also carrying it; asserts exactly 1 candidate, the worker's own. Also covers a login-lookalike without the `[bot]` suffix, correctly excluded via `deskkit.SameActor` (not naive string matching).
2. `TestWorkpadStampHasNoPath` — genuine adversarial table (absolute path w/ trailing slash, relative path, Windows-style path, empty string, bare `/`) plus my own additional adversarial cases (path traversal, double/triple slashes, embedded spaces) — all produced clean `basename@shortsha` output with zero path separators via `filepath.Base` + belt-and-suspenders replacement.
3. `TestWorkpadBodycheckRefuses` — genuine: a real token-shaped pattern in the body → exit 5 AND the fake forge recorded zero requests (nothing reached the network layer, not a partial post).
4. `TestWorkpadUpsertIsIdempotent` — genuine: first call creates (editCount 0), second call against the same seam edits (no second create, editCount 1) — one comment, edited.
5. Marker-exemption scope — read `StripWorkpadMarkerLine` and its bodycheck call site directly: the exemption blanks a line only on an EXACT match against the fixed marker constant; appending anything else to that line means it's no longer blanked and stays subject to the normal scan. Narrowly scoped as claimed, cannot be used to smuggle a secret near the marker.

`RISK-VALUE: DERIVED` — the marker string, the identity-match requirement, and the bodycheck-exemption scope are all traced to source and tested against adversarial shapes (a foreign human author, a login-lookalike, path-traversal stamps, same-line-append smuggling), not merely asserted in prose.

**VERIFY: PASS.** Rows 1-7 pass cleanly; row 8 fails only for the same tracked tool-wide gap (`assay#822`) hitting every brief in this fan-out post-migration — the underlying consumers claim is independently re-confirmed correct via direct diff. `gate: model`, `risk: {all no}`, `irreversible: no` — flip-eligible.

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
