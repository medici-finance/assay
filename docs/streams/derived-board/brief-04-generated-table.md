---
brief: derived-board/04
title: generated Briefs table in every stream README + single-writer lint + scheduled reconcile PR
why: >-
  The engine is worthless if the board people read is still the hand-typed table. This
  brief makes the stream README's Briefs table a generated region written only by the
  regen job, makes a hand edit to it a lint PROBLEM, and schedules the reconcile so the
  board reflects a merge within the hour without anyone remembering anything.
wave: 2
depends: ["derived-board/03"]
unblocks: ["derived-board/06"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-22 by derived-board scoping session
sources:
  - "docs/streams/derived-board/spec.md §4 (where derivation runs), §5 (board: generated), §8 Q3"
  - ".github/workflows/assay-statusgen.yml — the existing single-writer regen job (push-to-main, loop guards, assay-board-writer identity)"
  - "docs/brief-rules.md §Derived surfaces — STATUS.md precedent"
  - "freshness-checked 2026-08-22 @ f78ea24 — regen job regenerates STATUS.md only; paths filter excludes STATUS.md to avoid loops"
consumers:
  - ".github/workflows/assay-statusgen.yml: fixed-here (this repo's own workflow; the human pushes workflow files)"
  - "examples/adopter-scaffold/.github/workflows/*: fixed-here"
  - "statusgen init scaffold (the workflow it writes for adopters): fixed-here"
---

# Brief 04 — generated table + single-writer lint + schedule

## Context
files:
- `statusgen/readmetable.go` (new) — render the Briefs table from frontmatter (#, title,
  wave, effort) + `reconcile` cells (Status, Verified, Reviewed + witness link) between
  `<!-- statusgen:briefs:begin -->` / `<!-- statusgen:briefs:end -->`; idempotent rewrite;
  anything outside the markers untouched.
- `statusgen/main.go` — `regen` gains `--readmes`; `--lint`: (a) a README with
  `board: generated` whose table region differs from a fresh offline render of the
  frontmatter columns = PROBLEM "hand edit to a generated table"; (b) the lifecycle
  columns are not compared offline (they are `unknown` offline) — only structure and
  authoring columns are.
- `.github/workflows/assay-statusgen.yml` — regen job runs `reconcile` with
  `GITHUB_TOKEN` (read-only `pull-requests: read`, `issues: read`), then `regen --readmes`;
  a new `schedule:` trigger (hourly) that opens/updates ONE PR `chore(board): reconcile`
  instead of pushing when stream READMEs changed; push-to-main path unchanged for
  `STATUS.md`.
- `statusgen/init.go` — the scaffolded adopter workflow gets the same shape.
- `docs/brief-rules.md` §Derived surfaces — add the README table + the markers.

facts:
- Loop guards stay: the scheduled PR's branch is `board/reconcile`, reused; the regen
  commit message keeps `[skip-status-regen]`; the workflow's `paths:` filter already
  excludes `STATUS.md`.
- `board: generated` in the stream README frontmatter is the opt-in; a README without it
  is rendered as today (hand table) and NOTICEd "ungenerated board" — this is the
  per-stream migration edge brief 06 flips fleet-wide.
- Noise floor (spec Q3): the scheduled run opens a PR only when a cell CHANGES state, not
  when only a witness link/SHA changes; SHA-only changes land on the next push-to-main regen.
- The PR is opened by the existing `assay-board-writer` identity; no new App.

## Ground rules
- NEVER git push / trigger workflows. Workflow-file changes are committed on the branch;
  the human pushes them.
- Stop at `implemented`.
- The generated region is the ONLY thing this code writes in a README.

## Task
1. `readmetable.go` + golden-file tests (render, idempotency, markers-missing error,
   untouched prose outside markers).
2. `regen --readmes`; `--lint` hand-edit PROBLEM with a mutation test (edit one authoring
   cell inside the markers → PROBLEM names stream + row).
3. Workflow: permissions, reconcile step, schedule + PR path; scaffold parity.
4. Flip THIS stream's README to `board: generated` with markers as the first live example.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go test . -run ReadmeTable -count=1` | `ok` |
| 2 | `cd statusgen && go run . regen --readmes --root . --offline && git diff --stat -- docs/streams/derived-board/README.md` | table region rewritten; `git diff` outside the markers is empty (`git diff -U0 -- docs/streams/derived-board/README.md \| grep -v -E '^(@@\|\+\+\+\|---)' \| grep -v -E '^\|' \| wc -l` → `0`) |
| 3 | `cd statusgen && go run . regen --readmes --root . --offline && go run . regen --readmes --root . --offline && git status --porcelain docs/streams \| wc -l` | second run changes nothing beyond the first |
| 4 | `sed -i '' 's/^| 01 | \[brief-v2 spec/| 01 | [EDITED/' docs/streams/derived-board/README.md && cd statusgen && go run . --lint --root ..; echo rc=$?; git checkout -- ../docs/streams/derived-board/README.md` | `rc=1`; output contains `hand edit to a generated table` and `derived-board` |
| 5 | `python3 -c "import yaml;w=yaml.safe_load(open('.github/workflows/assay-statusgen.yml'));assert 'schedule' in w[True] or 'schedule' in w['on'];print('ok')"` | `ok` (YAML parses; schedule trigger present) |
| 6 | `grep -c -E -e 'pull-requests: read' -e 'issues: read' .github/workflows/assay-statusgen.yml` | `2` |
| 7 | `grep -c 'statusgen:briefs:begin' docs/streams/derived-board/README.md` | `1` |
| 8 | `cd statusgen && go run . init --dry-run /tmp/adopter-x \| grep -c 'reconcile'` | ≥ 1 (scaffold parity) |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
Reviewer question: can the scheduled job and the push-to-main job race on the same
README? Describe the interleaving that would produce a lost write, or show why the
branch + single-writer identity prevents it.
