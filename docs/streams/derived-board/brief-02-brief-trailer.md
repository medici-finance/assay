---
brief: derived-board/02
title: "`Brief:` trailer — the PR→brief link, required by deskpr create, linted on main"
why: >-
  Deriving a board cell from a PR needs a reliable edge from the PR to the brief. Today
  that edge is convention — branch names, body prose, titles — and a derivation built on
  guesses would be the "confident answer from an instrument that never looked" class the
  house forbids. One trailer line, written at the moment the worker already fills the PR
  body, turns the edge into data.
wave: 0
depends: []
unblocks: ["derived-board/03", "derived-board/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-22 by derived-board scoping session
sources:
  - "docs/streams/derived-board/spec.md §2 (\"the trailer is the only PR→brief edge\"), §8 Q1"
  - "tools/desk/cmd/deskpr/main.go — owns the PR-body write (create/update), draft-only by construction"
  - "freshness-checked 2026-08-22 @ f78ea24 — deskpr has no body-shape validation; statusgen has no PR-body reader"
consumers:
  - "plugins/assay/skills/worker-desk/SKILL.md (task spec: body carries the trailer): follow-up derived-board/05"
  - "plugins/assay/skills/pr-review-desk/SKILL.md (bounce a PR without a trailer): follow-up derived-board/05"
---

# Brief 02 — the `Brief:` trailer

## Context
files:
- `tools/desk/internal/deskkit/trailer.go` (new) — parse/validate `Brief:` and `Issue:`
  trailers from a PR body: grammar, multiplicity rules, a single `ParseTrailers(body)`.
- `tools/desk/cmd/deskpr/main.go` — `create` and `update` refuse (exit 5, message names
  the missing line) unless the body carries exactly one `Brief: <stream>/<NN>` that
  resolves to a brief file under `--root`, OR `Issue: #<N>` for issue-only work.
  `--no-brief` is NOT an escape hatch; there is none.
- `statusgen/prlink.go` (new) — the tree-side half: given a list of (PR number, body,
  merged-sha) records (the fetch belongs to brief 03), classify each as linked /
  unlinked / multi-linked. Pure function, unit-tested with fixtures.
- `docs/desk-tools/deskpr.md` (planned) — document the trailer and the refusal.

facts:
- Trailer grammar: a line `Brief: <stream>/<NN>` (one brief per PR; shards of a
  `parallel-streams:` brief all name the parent) or `Issue: #<N>` (issue-only work).
  Anywhere in the body; first match wins; a second `Brief:` line is a refusal.
- `Closes #N` / `Refs #N` keep their GitHub meaning and are NOT the link — a PR may close
  an issue and deliver a brief.
- deskpr's refusal class is exit 5 (`ExitRefused`), consistent with deskmigrate.
- Existing open PRs without a trailer are not broken by this brief: `deskpr update` on a
  PR whose body lacks one prints the line to add and refuses; the worker edits the body.

## Ground rules
- NEVER git push / trigger workflows. Commit on the feature branch only.
- Stop at `implemented`.
- Do not add a bypass flag, env var, or commit token; a worker-typeable bypass makes the
  edge asserted again (brief-rules rule 30's override paragraph).

## Task
1. Implement `deskkit.ParseTrailers` + table-driven tests (valid, missing, duplicate,
   `Issue:` form, trailer inside a fenced code block = ignored).
2. Wire the check into `deskpr create` / `update` before any network call; resolve the
   brief path under `--root`; exit 5 with the exact missing line on failure.
3. Implement `statusgen` `prlink` classification + fixtures under
   `statusgen/testdata/prlink/`.
4. Docs.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test ./internal/deskkit/ -run Trailer -count=1` | `ok` |
| 2 | `cd tools/desk && go test ./cmd/deskpr/ -count=1` | `ok`, includes a test named `*Refuses*NoTrailer*` |
| 3 | `cd statusgen && go test . -run PRLink -count=1` | `ok` |
| 4 | `printf 'body with no trailer\n' > /tmp/b.md; cd tools/desk && go run ./cmd/deskpr create --title t --body-file /tmp/b.md --root ../..; echo rc=$?` | `rc=5`; stderr contains `Brief: <stream>/<NN>` |
| 5 | `printf 'Brief: derived-board/02\nBrief: derived-board/03\n' > /tmp/b.md; cd tools/desk && go run ./cmd/deskpr create --title t --body-file /tmp/b.md --root ../..; echo rc=$?` | `rc=5`; stderr mentions duplicate |
| 6 | `grep -rn -E -e '--no-brief' -e 'SKIP_TRAILER' -e 'skip-trailer' tools/desk/ \| wc -l` | `0` (no bypass surface) — mutation row: add one and row 2 must fail |
| 7 | `grep -n 'Brief:' docs/desk-tools/deskpr.md` | documented |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
Reviewer question: is there any path through `deskpr` that reaches the network before the
trailer check? Name the line if so.
