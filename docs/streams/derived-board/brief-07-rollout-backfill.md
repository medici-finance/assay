---
brief: derived-board/07
title: per-repo rollout — upgrade-assay to v1.0.0, reconcile step in each regen workflow, historical backfill as a drift-report PR; private re-stage of spec + skills
why: >-
  Five repositories carry stream boards and every one of them has phantom rows today.
  The cut is worthless until each repo runs the migration, gets the reconcile step, and
  has its history derived once — with the disagreements between what the table said and
  what GitHub says put in front of a human, not silently overwritten.
wave: 4
depends: ["derived-board/05", "derived-board/06"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-22 by derived-board scoping session
sources:
  - "docs/streams/derived-board/spec.md §7 (backfill as a drift-report PR)"
  - "migrations/0001-v0.13.0-to-v1.0.0-derived-board.md (brief 06) — the exact workflow YAML adopters add"
  - "freshness-checked 2026-08-22 @ f78ea24 — five stream-bearing repos, resolved via docs/streams/graph-repos.yaml"
exec-tier: any
domain: complicated
consumers:
  - "each consumer repo's .assay-versions, regen workflow, brief spec docs and the four desk skills: fixed-here (one PR per repo, listed in Task)"
---

# Brief 07 — rollout + backfill

## Context
files (per consumer repo, one PR each; this repo first):
- `.assay-versions` — re-pinned to v1.0.0 by `upgrade-assay` (sha256 from the published
  release's `checksums.txt`, harvested not typed).
- the repo's statusgen regen workflow — `reconcile` step + read permissions + schedule,
  exactly the YAML from the migration's release note.
- `docs/streams/*/README.md`, `docs/streams/*/brief-*.md` — migrated by the migration op.
- private toolkit only: `docs/brief-rules.md`, `docs/brief-template.md`, the four skills —
  overlay re-stage from the public copies (public is now the newer text).
- `statusgen reconcile --backfill --root . --repo <owner/name> --report` — produces
  `docs/streams/board-drift-<date>.md`: one row per brief where `hand-said ≠ derived`,
  with the witness (or "no PR carries a trailer; last hand edit <sha>"). Committed in the
  same PR so the reviewer sees the drift before the generated table replaces the old one.

facts:
- Historical PRs have no trailer. `--backfill` uses a DECLARED, reviewable fallback for
  history only: a merged PR whose branch name or body contains the brief id in the
  `<stream>/<NN>` or `<stream>-<NN>` form. Every backfilled edge is listed in the report
  with its source; after the rollout the fallback is OFF (trailer only).
- Rows where hand-said `implemented`/`verified`/`done` and no PR is found are NOT demoted
  silently: they render `unknown (no witness — hand-asserted <state> at <sha>)` until a
  human either links the PR (adds the trailer to the merged PR's body via edit, which the
  next reconcile reads) or accepts the demotion in the drift PR.
- Workflow-file pushes in every repo are the human's; each rollout PR is opened by the
  worker with the workflow change included and the human pushes that file.
- Rate limits: one rollout PR at a time per repo; the reconcile step's single list call
  per run is the budget.

## Ground rules
- NEVER git push workflow files / trigger workflows; open the PR with the change and
  name the workflow file in the body for the human.
- Stop at `implemented` per repo; the board itself proves the rest.
- Never resolve a drift row by editing a table; link or accept, in the report.

## Task
1. This repo: `upgrade-assay --to v1.0.0` (dry-run, then apply), workflow step, backfill
   report, PR.
2. Each consumer repo, same sequence, one PR each, serially.
3. Private toolkit additionally: overlay re-stage of the spec docs + four skills.
4. After every repo is merged: remove the historical fallback flag from `--backfill`
   (or leave it refusing with "backfill already ran for this repo" — record which).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -E -e '^statusgen ' -e '^desk-tools-linux-amd64 ' .assay-versions \| awk '{print $2}' \| sort -u` | exactly `v1.0.0` |
| 2 | `statusgen --root . --lint` | exit 0; no `ungenerated board` NOTICE |
| 3 | `grep -l 'board: generated' docs/streams/*/README.md \| wc -l` | equals `ls -d docs/streams/*/ \| wc -l` (every stream) |
| 4 | `ls docs/streams/board-drift-*.md && grep -c '^|' docs/streams/board-drift-*.md` | report exists; row count printed |
| 5 | `statusgen reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);u=[b for b in d['briefs'] if b['cell']=='unknown'];print(len(u));[print(b['id'],b['reason']) for b in u]"` | every `unknown` has a reason naming a hand-asserted state or a missing trailer — none says `offline` in CI |
| 6 | `gh run list -w assay-statusgen.yml -e schedule -L 1 --json conclusion --jq '.[0].conclusion'` | `success` (first scheduled reconcile ran) |
| 7 | `for r in $(python3 -c "import yaml;print(' '.join(v['repo'].split('/')[1] for v in yaml.safe_load(open('docs/streams/graph-repos.yaml'))['repos'].values()))"); do statusgen --root ../$r --lint; echo $r rc=$?; done` | `rc=0` each; list the five in Evidence |
| 8 | in the private toolkit checkout: `diff <(sed -n '1,40p' docs/brief-rules.md) <(curl -fsSL https://raw.githubusercontent.com/medici-finance/assay/main/docs/brief-rules.md \| sed -n '1,40p')` | empty (private re-staged to public text) — DEREFERENCES the public file |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date via the generated board.
Reviewer question: open the drift report and pick three rows at random — does each
witness link resolve to the PR it claims?
