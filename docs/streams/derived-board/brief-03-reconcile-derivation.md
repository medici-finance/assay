---
brief: derived-board/03
title: "`statusgen reconcile` — derive lifecycle state from PRs, witnesses, approvals and rulings; brief-v2 parser"
why: >-
  This is the engine that makes the board stop lying: every lifecycle cell is computed
  from a witness the tool actually read, and a cell the tool could not read is printed
  as unknown with the reason, never as a quiet todo. It also lands the brief-v2 parser
  (reserved graph keys, fail-closed on an unpinned old binary), so the fleet takes one
  schema flag-day instead of two.
wave: 1
depends: ["derived-board/01", "derived-board/02"]
unblocks: ["derived-board/04"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-22 by derived-board scoping session
sources:
  - "docs/streams/derived-board/spec.md §2 (derivation table), §4 (online/offline), §5 (brief-v2)"
  - "docs/brief-rules.md rule 30, §Three-state instrument invariant"
  - "statusgen/brieffile.go, statusgen/main.go (verifyrun, the verified→done auto-flip from App approval at head) — the existing derivation pieces this composes"
  - "docs/dependency-graph-design.md §3.3–3.4 — ref grammar + gates:/feathers: shapes to parse and validate"
  - "freshness-checked 2026-08-22 @ f78ea24 — statusgen is tree-only; no GitHub client exists in statusgen/"
exec-tier: strong
exec-tier-why: composes four independent witness sources with precedence + demotion rules; an error here is a board that lies with more authority than before
domain: complicated
consumers:
  - "statusgen/README.md (verbs): fixed-here"
  - "docs/streams/derived-board/spec.md §8 Q2: fixed-here"
---

# Brief 03 — `statusgen reconcile` + brief-v2 parser

## Context
files:
- `statusgen/brieffile.go` — accept `schema: brief-v2`; parse `gates:` / `feathers:` into
  typed structs; parse `id:` (uuid v4 shape, PROBLEM on duplicate ids across the tree),
  `supersedes:` (refs, validated), Verify-row `id`/`target`; validate refs against the §3.3 grammar and `docs/streams/graph-repos.yaml` (planned) — created by brief 01;
  PROBLEM on unknown edge `type`; gating behaviour NOT implemented (reserved).
- `statusgen/lifecycle.go` (new) — the pure derivation: inputs `{briefs, prRecords,
  witnesses, approvals, rulings, issueLabels, lookedAt}` → per-brief `{cell, reason,
  witnessRef}`. No I/O. Precedence and demotion per spec §2.
- `statusgen/ghfetch.go` (new) — the ONLY network code: list PRs (open + merged, paged,
  with bodies and merge SHAs), reviews at head, issue labels. Token from `GITHUB_TOKEN` /
  `--token-file`; read-only endpoints only; every failure returns `lookedAt=false` with
  the HTTP status, never an empty "nothing found".
- `statusgen/main.go` — new verb `reconcile [--root . --repo owner/name --json --offline]`;
  `--lint` gains the offline arm (PR-derived cells `unknown (offline)`).
- `statusgen/testdata/lifecycle/` — fixtures: one per row of the spec §2 table plus
  demotion cases (reopened PR, red witness, dismissed approval) and the offline arm.

facts:
- Cell precedence (highest witnessed wins): `done` > `verified` > `implemented` >
  `in-progress` > `blocked` > `todo`; `unknown` replaces any PR-derived cell when
  `lookedAt=false`. `blocked` overlays `in-progress`/`todo` only.
- `in-progress` = ANY open PR with the trailer (draft included) — spec Q2 settled here.
- Multiple merged PRs with the same trailer: the brief is `implemented` at the LATEST merge
  SHA; the others are listed in the witness. Shards of a `parallel-streams:` brief name the
  parent and are one PR anyway.
- `verified`/`done` derivation is the EXISTING code path (`verifyrun --check`, approval at
  head); this brief calls it, it does not reimplement it.
- Repeatability: `reconcile --json` output is deterministic given the same inputs; the
  fixtures assert byte-equal JSON.
- Rate limits: one list call per repo per run, paged; never per-brief calls.

## Ground rules
- NEVER git push / trigger workflows. Commit on the feature branch only.
- Stop at `implemented`.
- Every negative the tool prints must be paired with evidence it looked (the search ran,
  the page count, the timestamp). A row rendering `todo` with `lookedAt=false` is a bug,
  not a default.

## Task
1. brief-v2 parse + validation in `brieffile.go`; fixtures for each reserved key and for
   a v2 brief under an old `schema:` expectation (PROBLEM names the file and says
   "tree is brief-v2; this statusgen predates it" when the binary is pre-v1 — wording
   lives here even though the refusal ships in v1.0.0).
2. `lifecycle.go` + the full fixture matrix.
3. `ghfetch.go` against the REST API (no GraphQL dependency), with a recorded-response
   test double; no live network in `go test`.
4. `reconcile` verb + `--lint` offline arm + `statusgen/README.md` verb entry.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go test . -run 'Lifecycle' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'BriefV2' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'GHFetch' -count=1 -v \| grep -c '^--- PASS'` | ≥ 14 (7 cells + 3 demotions + offline + 3 v2-parse cases) |
| 2 | `cd statusgen && go run . reconcile --root . --offline --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert all(b['cell']=='unknown' for b in d['briefs'] if b['source']=='pr');print('ok')"` | `ok` — offline never renders a PR-derived cell as todo |
| 3 | `cd statusgen && GITHUB_TOKEN=invalid go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert d['lookedAt']==False and d['reason'].startswith('HTTP');print('ok')"` | `ok` — an auth failure is an `unknown` with the status, not a clean board |
| 4 | `cd statusgen && go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);b=[x for x in d['briefs'] if x['id']=='desk-containers/02'][0];assert b['cell'] in ('implemented','verified','done') and b['witness'].startswith('PR #67');print(b['cell'])"` | prints the cell — DEREFERENCES the real merged PR #67 (needs a read token in env) |
| 5 | `cd statusgen && printf -- '---\nbrief: x/01\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nschema: brief-v2\ngates: [{on: "oit:issue-loop/06", type: ordering-gate, reason: r}]\n---\n' > testdata/tmp-v2.md && go run . --lint --root testdata/v2-smoke; echo rc=$?` | `rc=0` and output contains `gates: 1 edge (reserved, not gating)` — fixture dir prepared by the brief |
| 6 | `cd statusgen && go test . -run 'Demotion' -count=1 -v \| grep -c PASS` | ≥ 3 |
| 7 | `grep -c 'reconcile' statusgen/README.md` | ≥ 1 |
| 8 | `cd statusgen && go vet ./... && ! grep -rn 'graphql' --include=*.go . ` | exit 0 |

## Evidence
<!-- appended at implementation time -->

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
Reviewer questions: (1) find one input combination where the engine prints a negative
without `lookedAt=true` — if you can, the brief is not done; (2) is the precedence table
in `lifecycle.go` identical to spec §2?
