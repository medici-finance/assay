---
brief: assay:assay:derived-board:03
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
schema: brief-v2
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
version: 3
id: c4f45c56-1898-4dcc-aaaa-d10ecda4df98
---

# Brief 03 — `statusgen reconcile` + brief-v2 parser

## Context
files:
- `statusgen/brieffile.go` — accept `schema: brief-v2`; parse the hierarchical `brief:`
  (`<cell>:<repo>:<stream>:<NN>`) and validate cell/repo against `graph-repos.yaml` and
  stream/NN against the path; resolve elided refs; parse `version:` (int ≥ 1) and PROBLEM a
  Task/Verify diff without a bump (needs the merge-base — same plumbing the existing
  Status-transition lint uses); witness rows carry `version`, and the lifecycle fold demotes
  a witness whose version ≠ the brief's to `unknown (witness for vN, brief is vM)`; parse `gates:` / `feathers:` into
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
- `statusgen/lifecycle_test.go` — the fixture matrix: one per row of the spec §2 table
  plus demotion cases (reopened PR, red witness, dismissed approval) and the offline
  arm. (Authored above as `statusgen/testdata/lifecycle/`; the matrix landed inline in
  the test file — text corrected 2026-10-09, the /03 half of #1909.)

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
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd statusgen && ok=0; for f in Lifecycle:20 BriefV2:14 GHFetch:5; do s=${f%%:*}; out=$(go test . -run "$s" -count=1 -v) \|\| { echo "suite $s failed"; exit 1; }; n=$(printf '%s\n' "$out" \| grep -c '^--- PASS'); [ "$n" -ge "${f##*:}" ] && ok=$((ok+1)) \|\| echo "suite $s: $n passes, below ${f##*:}"; done; echo "suites-at-floor=$ok"` | exit 0, output is `suites-at-floor=3` — each suite meets its OWN floor (Lifecycle 20, BriefV2 14, GHFetch 5: the counts at the 2026-10-09 rework), so a missing, shrunken or failing suite fails the row; a red `go test` ends the command non-zero rather than being discarded by a pipe. Re-authored 2026-10-09 (#1787): the original three-command form printed three counts but a witness records only the last pipeline value, so the Lifecycle and BriefV2 suites went unscored (the recorded 5 was GHFetch alone); a single summed count with a floor of 14 could not fail when a whole suite went missing | check |
| 2 | `cd statusgen && go run . reconcile --root . --offline --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert all(b['cell']=='unknown' for b in d['briefs'] if b['source']=='pr');print('ok')"` | `ok` — offline never renders a PR-derived cell as todo | check |
| 3 | `cd statusgen && GITHUB_TOKEN=invalid go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert d['lookedAt']==False and d['reason'].startswith('HTTP');print('ok')"` | `ok` — an auth failure is an `unknown` with the status, not a clean board | check |
| 4 | `cd statusgen && go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);b=[x for x in d['briefs'] if x['id'].endswith(':derived-board:02')][0];assert b['cell'] in ('implemented','verified','done') and b['witness'].startswith('PR #80');print(b['cell'])"` | prints the cell — DEREFERENCES the real merged PR #80, whose body carries `Brief: derived-board/02` (the trailer the engine witnesses; needs a read token in env). Re-anchored from the original `desk-containers/02`/`PR #67`, which the engine correctly returns `todo` for: that brief's deliverable PR lives in another repository and carries no `Brief:` trailer (its board flip went via a separate PR), so this `--repo medici-finance/assay` run has no witness for it. The engine is sound; the old row anchored on a brief whose deliverable it cannot witness from this repo. Re-baselined 2026-09-19 (issue #1305): the `id` field is the hierarchical `assay:assay:derived-board:02` since the derived-board/07 flag-day, so the lookup is now id-shape-tolerant (`endswith(':derived-board:02')` pins stream/NN, tolerating a cell/repo prefix change) — a flat-literal match broke on this anchor a second time. | check |
| 5 | `cd statusgen && printf -- '---\nbrief: x/01\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nschema: brief-v2\ngates: [{on: "rec:ingest/06", type: ordering-gate, reason: r}]\n---\n' > testdata/tmp-v2.md && go run . --lint --root testdata/v2-smoke; echo rc=$?` | `rc=0` and output contains `[eligibility-could-not-check] demo/01: held by rec:ingest/06` — fixture dir prepared by the brief. Re-baselined 2026-09-19 (issue #1305): `gates:` became an actively gating eligibility evaluator (#1251), so the `reserved, not gating` wording can no longer appear — in this fixture the `rec:` alias is unpublished from the tree and the evaluator's honest three-state answer is the could-not-check NOTICE. | check |
| 6 | `cd statusgen && go test . -run 'Demotion' -count=1 -v \| grep -c PASS` | ≥ 3 | check |
| 7 | `grep -c 'reconcile' statusgen/README.md` | ≥ 1 | check |
| 8 | `cd statusgen && go vet ./... && ! grep -rn 'graphql' --include=*.go ghfetch.go reconcile.go lifecycle.go briefv2.go` | exit 0 — the derivation's own network layer uses REST, never GraphQL (the grep is scoped to the files THIS brief introduces; a repo-wide grep additionally matches the pre-existing `trustgate.go` trust-query `gh api graphql`, a security control landed by forward-sync after this brief was authored and out of this brief's scope) | check |
| 9 | `cd statusgen && { go test . -run '^TestFoldWiresAllInputs$' -count=1 -v > "${TMPDIR:-/tmp}/fold9.out" 2>&1 && grep -qF -e '--- PASS: TestFoldWiresAllInputs' "${TMPDIR:-/tmp}/fold9.out"; } \|\| { echo fold-test-failed; exit 1; }; go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert d['lookedAt'] and not d.get('unread'),'a fold read is unread: %s'%d.get('unread');bl=[b for b in d['briefs'] if b['cell']=='blocked' and b['witness'].startswith('linked issue label')];assert bl,'no brief derives blocked from a linked issue label - the fold inputs are not wired (#1787)';print('fold-wired')"` | exit 0, output is `fold-wired` — two halves, both needed. (a) The fixture test `TestFoldWiresAllInputs` drives the PRODUCTION `runReconcile` over a git-backed tree and asserts `done` on both gates (gate:model from the bound reviewer's App approval at the merged head via `decideModelFlip`; gate:human from an anchored Reviewed-cell `human:` stamp), `blocked` from a linked issue label, and no promotion from a red witness. (b) The live read-only run (needs a read token in env, same lane as rows 3/4) reads every fold input with nothing unread, and at least one brief derives `blocked` from a linked issue's label — a cell the unwired engine could not reach. The live tree is NOT asserted to derive `done`: the fold takes `verified` only from the flip owner's rule (`flipLatestPass` — the latest strict bold PASS marker with Date/Runner rows naming the verifier at a sha), and at the 2026-10-09 rework no brief's recorded Evidence meets it (most predate that format), so the live tree derives zero `verified`/`done`; asserting a live `done` would need the fold to accept weaker Evidence than the flip gate does. Re-authored 2026-10-09 from the earlier live `done` count, which counted witnesses the flip owner refuses | check +flow |
| 10 | `T=$(mktemp -d) && (cd tools/desk && go build -o "$T/muhar" ./cmd/muhar) && cd statusgen && "$T/muhar" -spec reconcilefold-mutations.json -j 4 > "$T/out" 2>&1; rc=$?; tail -1 "$T/out"; exit $rc` | exit 0, output is `Totals: 18 caught, 0 NOT CAUGHT, 0 could-not-mutate.` — the mutation harness removes each guard the fold relies on, one at a time, and the fold suites redden for every one: the anchored and relay-stripped human stamp, the held-row and closure-audit refusals, verifier provenance, the version read at the run's sha, approval could-not-check, the at-head and gate checks in `isDone`, the merged-PR overlay requirement and the ListPRs merged check, PR entries filtered from the issues read, issues paging and the page cap, the failed labels read, the `--apply` refusal on unread inputs, and the witnessed-row write. The positive control (the witness map left unwired, the #1787 defect) must also be caught, or the harness reports itself broken and the row fails. Added 2026-10-09 (`+mutation`: this change alters `lifecycle.go`, a control this brief declares) | check +mutation |

## Evidence
### Non-implementer verifier run — VERIFY: FAIL (row 4 — stale Verify-row anchor, not an engine defect); rows 3/4 re-run ONLINE read-only per the desk online-read-only-lane ruling — 2026-09-06 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `5d20ff9`
Runner ≠ implementer. Isolated worktree off origin/main. statusgen built from this worktree's source, not PATH. Envelope: KUBECONFIG=/dev/null; rows 3+4 online read-only (verifier read token) — the sanctioned online lane for read-only forge reads; no mutating/cluster call. `gate: model`, all risk `no`. This supersedes the prior offline "BLOCKED (rows 3/4 could-not-check)" run — rows 3/4 are now RUN.

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | lifecycle/briefv2/ghfetch tests → grep -c '^--- PASS' | ≥14 | exit 0 — 26 (15+6+5) | 2026-09-06 | opus-4.8[1m]-verifier |
| 2 | reconcile --root . --offline --json → all pr-cell unknown | exit 0 | exit 0, ok — no offline PR-derived cell is todo | 2026-09-06 | opus-4.8[1m]-verifier |
| 3 | GITHUB_TOKEN=invalid reconcile --repo medici-finance/assay --json → lookedAt==false, reason HTTP… | exit 0 | exit 0, ok — lookedAt=false, reason "HTTP 401: Bad credentials" (online read; fail-closed) | 2026-09-06 | opus-4.8[1m]-verifier |
| 4 | reconcile --repo medici-finance/assay --json (valid read token) → desk-containers/02 cell in implemented/verified/done, witness PR #67 | rc=0 | **FAIL — rc=1.** Token accepted (lookedAt=true, 139 briefs; 75 briefs derive PR-witnessed implemented/verified/done correctly, incl. derived-board/03 → PR #199). desk-containers/02 → cell=todo, witness="" ("PR search ran; no open or merged PR carries this brief's trailer"). STALE ANCHOR: the deliverable is merged PR #67 whose body carries NO `Brief:` trailer; per the engine's deliberate single-trailer contract (ghfetch.go:98-121, singleBriefTrailer :217) it honestly returns todo; the board-flip lives in PR #74. Live-data drift from the 2026-08-22 fixture — re-anchor row 4 (or re-record PR #67's trailer) | 2026-09-06 | opus-4.8[1m]-verifier |
| 5 | printf v2 fixture && --lint --root testdata/v2-smoke | rc=0 | exit 0 — "gates: 1 edge (reserved, not gating)"; LINT: PASS | 2026-09-06 | opus-4.8[1m]-verifier |
| 6 | go test . -run Demotion → grep -c PASS | ≥3 | exit 0 — 12 | 2026-09-06 | opus-4.8[1m]-verifier |
| 7 | grep -c reconcile statusgen/README.md | ≥1 | exit 0 — 5 | 2026-09-06 | opus-4.8[1m]-verifier |
| 8 | go vet ./... && no 'graphql' in the 4 brief files | exit 0 | exit 0 — vet clean; no graphql in ghfetch/reconcile/lifecycle/briefv2 | 2026-09-06 | opus-4.8[1m]-verifier |

`RISK-VALUE: DERIVED — maxPages = 20 @ statusgen/ghfetch.go:104 (with perPage = 100 @ :103) — a hard cap against a malformed Link-header page loop; 20×100 = 2000 PRs bounds medici-finance/assay (~#468 highest, >4x headroom) so no witness is silently truncated; wrong value truncates but is reversible by edit+redeploy. Fail-closed guard: status != http.StatusOK → lookedAt=false @ ghfetch.go:111 (paired :156) — the literal is the 200 protocol constant, not a tunable; proven by row 3's clean 401→lookedAt=false. version:=1 legacy default @ reconcile.go:133 reversible, ranks last.`
**VERIFY: FAIL — row 4 (stale Verify-row anchor), NOT an engine defect.** The reconcile engine is sound: 75 PR-witnessed cells derive correctly online (incl. this brief's own → PR #199), rows 1-3,5-8 PASS, the fail-closed 401 path works (row 3). Row 4 asserts a specific anchor (desk-containers/02 + PR #67 trailer) that live data no longer satisfies — PR #67 merged without a `Brief:` trailer (its board-flip was PR #74), so the engine correctly returns todo and the row's assertion fails. Per the verifier contract an unmet written expectation is not a pass → stays `implemented`; re-anchor row 4 to a trailer-carrying deliverable (or re-record PR #67's trailer). Counts in CFR (checked-fail on the artifact per the desk ledger ruling). Re-baseline routed.

<!-- appended at implementation time -->

Implemented on `feat/derived-board-03`. New files: `statusgen/lifecycle.go` (pure
derivation), `statusgen/ghfetch.go` (the only network code — REST, no GraphQL),
`statusgen/briefv2.go` (brief-v2 parser/validator), `statusgen/reconcile.go` (the
verb); `brieffile.go` recognizes `schema: brief-v2` and parses its reserved keys;
`main.go` dispatches the `reconcile` verb. Fixture root `statusgen/testdata/v2-smoke/`.

| # | Result | Runner |
|---|--------|--------|
| 1 | PASS — Lifecycle 13 + BriefV2 6 + GHFetch 5 = 24 `^--- PASS` (≥14) | 2026-08-29 opus-4.8[1m] worker |
| 2 | PASS — `reconcile --root . --offline --json` → `ok` (86 briefs, every pr-source cell unknown) | 2026-08-29 opus-4.8[1m] worker |
| 3 | PASS — `GITHUB_TOKEN=invalid … --repo medici-finance/assay` → `ok` (lookedAt=false, `HTTP 401: Bad credentials`) | 2026-08-29 opus-4.8[1m] worker |
| 4 | PASS (online, re-anchored) — `reconcile --repo medici-finance/assay --json` → `derived-board/02` cell `implemented`, witness `PR #80 (merged c93ae91)`; the assert (`cell in (implemented,verified,done) and witness startswith 'PR #80'`) holds. PR #80's body carries `Brief: derived-board/02` — the trailer `ghfetch.ListPRs` witnesses — so the engine dereferences a real merged deliverable. **Re-anchored from the original `desk-containers/02`/`PR #67`**: on `--repo medici-finance/assay` the engine correctly returns `desk-containers/02` → `todo` (reason: "PR search ran; no open or merged PR carries this brief's trailer") because that brief's deliverable PR lives in another repository, whose board flip went via a separate PR and which carries no `Brief:` trailer — nothing in this repo witnesses it. The engine was sound; the old row anchored on a brief whose deliverable it cannot witness from this repo. | 2026-09-06 opus-4.8[1m] worker |
| 5 | PASS — `--lint --root testdata/v2-smoke` → `rc=0`, output contains `gates: 1 edge (reserved, not gating)` | 2026-08-29 opus-4.8[1m] worker |
| 6 | PASS — `go test -run 'Demotion'` → 12 PASS (≥3): reopened-PR, red-witness, dismissed-approval, stale-version demotions | 2026-08-29 opus-4.8[1m] worker |
| 7 | PASS — `grep -c 'reconcile' statusgen/README.md` → 5 (≥1) | 2026-08-29 opus-4.8[1m] worker |
| 8 | PASS — `go vet ./...` clean; grep over the derivation's own files (ghfetch/reconcile/lifecycle/briefv2) finds no `graphql`. Row scoped to the files this brief introduces: the repo-wide form additionally matches the PRE-EXISTING `statusgen/trustgate.go` trust-query (`gh api graphql`), a security control forward-synced after this brief was authored (2026-08-22) and out of this brief's scope — not removed (worker security-gate clause). | 2026-08-29 opus-4.8[1m] worker |

Fail-first (clause 9) — each new guard shown reddening on mutated code, then restored:
- three-state offline invariant: `!LookedAt` cell `"unknown"`→`"todo"` reddens
  `TestLifecycleCellUnknown` (`want unknown, got "todo"`) and `TestLifecycleOffline`.
- ghfetch three-state: a non-200 returning `lookedAt=true, ""` (fail-open) reddens
  `TestGHFetchAuthFailure` (`an auth failure must be lookedAt=false`).
- brief-v2 edge typing: disabling the edge-type check reddens
  `TestBriefV2GatesEdgeValidation` (`bad edge type should be a PROBLEM; got []`).

### Non-implementer verifier run — VERIFY: PASS on offline rows; BLOCKED (offline→online hand-off) — 2026-09-01 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `63a7a8a`

Runner ≠ implementer. Own temp worktree off `origin/main`, offline (`KUBECONFIG=/dev/null`); rows run from inside `statusgen/`. **Held at `implemented`** — the two live-forge rows (3, 4) are could-not-check under the offline envelope and are the online-lane hand-off; the offline half is fully checked-clean.

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | statusgen lifecycle/briefv2/ghfetch tests → `grep -c '^--- PASS'` | 0 | 24 `--- PASS` (≥14) — checked-clean |
| 2 | `go run . reconcile --root . --offline --json` → assert all pr-cell `unknown` | 0 | `ok` — no offline PR-derived cell is `todo`; checked-clean |
| 3 | `GITHUB_TOKEN=invalid go run . reconcile --root . --repo medici-finance/assay --json` → assert `lookedAt==false`, reason `HTTP…` | — | **could-not-check (offline→online hand-off)** — live forge (`api.github.com`). Online-lane marker = this exact command. |
| 4 | `go run . reconcile --root . --repo medici-finance/assay --json` → assert a cell is implemented/verified/done with witness `PR #…` | — | **could-not-check (offline→online hand-off)** — live forge, needs a read token. Online-lane marker = this exact command. |
| 5 | `printf … > testdata/tmp-v2.md && go run . --lint --root testdata/v2-smoke` | 0 | `rc=0`; output contains `gates: 1 edge (reserved, not gating)` — checked-clean |
| 6 | `go test . -run 'Demotion' -count=1 -v \| grep -c PASS` | 0 | 12 (≥3) — checked-clean |
| 7 | `grep -c 'reconcile' statusgen/README.md` | 0 | 5 (≥1) — checked-clean |
| 8 | `go vet ./... && ! grep -rn 'graphql' … ghfetch.go reconcile.go lifecycle.go briefv2.go` | 0 | vet clean; no `graphql` in the 4 brief files — checked-clean |

`RISK-VALUE: DERIVED` — the fail-closed three-state guard `if status != http.StatusOK { return nil, false, httpReason(status, body) }` @ `statusgen/ghfetch.go:111` (paired at `:157`): any non-200 yields `lookedAt=false` + the HTTP status, never an empty "nothing found" — derived from the brief's three-state invariant, proven by `TestGHFetchAuthFailure` (the machinery behind row 3). Secondary hard-bound literal `const maxPages = 20` @ `statusgen/ghfetch.go:104` (page-loop cap).

**VERIFY: BLOCKED (offline→online hand-off)** — all six offline rows (1,2,5,6,7,8) checked-clean by a non-implementer; rows 3 and 4 are live-forge could-not-check and stay for the online/live-forge verify lane (exact commands recorded above). **Status held at `implemented`** — a could-not-check is not a pass; the online lane owns rows 3/4 and the completion flip.
### Non-implementer verifier run — VERIFY: FAIL (row 4 — reconcile engine defect, PR-witness matching broken by the brief-v2 id flag-day) — 2026-09-15 sonnet-5-verifier (verify-desk dispatch), merged main `0bf1166`

Runner ≠ implementer. Isolated worktree off `origin/main` (HEAD == `origin/main` == `0bf1166`).
statusgen built from this worktree's source, not PATH. Envelope: `KUBECONFIG=/dev/null`; rows
3+4 run ONLINE read-only (verifier read token) — the sanctioned online lane for read-only
forge reads used by the two prior verifier runs on this brief; no mutating/cluster call.
`gate: model`, all risk `no`.

| # | command | expected | exit / observed | Date | Runner |
|---|---------|----------|-----------------|------|--------|
| 1 | `go test . -run 'Lifecycle'\|'BriefV2'\|'GHFetch' -count=1 -v \| grep -c '^--- PASS'` (3 calls) | ≥14 | exit 0 — Lifecycle=16, BriefV2=12, GHFetch=5, total=33 | 2026-09-15 | sonnet-5-verifier |
| 2 | `reconcile --root . --offline --json` → all pr-source cells `unknown` | exit 0, `ok` | exit 0, `ok` — no offline PR-derived cell is `todo` | 2026-09-15 | sonnet-5-verifier |
| 3 | `GITHUB_TOKEN=invalid reconcile --repo medici-finance/assay --json` → `lookedAt==False`, reason starts `HTTP` | exit 0, `ok` | exit 0, `ok` — `lookedAt=false`, reason `HTTP 401: Bad credentials` | 2026-09-15 | sonnet-5-verifier |
| 4 | `reconcile --repo medici-finance/assay --json` (valid read token) → `derived-board/02`'s cell in (implemented,verified,done), witness startswith `PR #` | rc=0 | **FAIL — rc=1, `IndexError: list index out of range`.** Token accepted, `lookedAt=true`, 176 briefs read. Root cause: on `bb2079bd` (`flag day: migrate every brief brief-v1 -> brief-v2 (derived-board/07)`, #736, 2026-09-10) every brief's `brief:` id was rewritten hierarchical (`assay:assay:derived-board:02`), but `lifecycle.go`'s `prsByBrief[pr.BriefRef]` (`lifecycle.go:120`) keys by the PR body's literal `Brief:` trailer text verbatim (`ghfetch.go:238-256`, `singleBriefTrailer` — no normalization), and every merged PR in this repo's history (including this brief's OWN deliverable, PR #199, body: `Brief: derived-board/03`, confirmed via a live read) still carries the pre-flag-day flat trailer. Exact-string match against the new hierarchical id therefore never succeeds: **all 176/176 briefs in the current tree derive `cell=todo`, `witness=""`, reason "PR search ran; no open or merged PR carries this brief's trailer"** — including briefs independently confirmed `done`/merged in the README (`derived-board/02`, witness PR #80, merged, body confirmed to carry `Brief: derived-board/02`) and this brief's own PR #199. This is not the prior FAIL's stale-anchor class (2026-09-06, superseded) — the engine itself now returns a false `todo` for the entire PR-witnessed corpus, the exact failure mode ("a board that lies with more authority than before") `exec-tier-why` names. Bug filed: see below. | 2026-09-15 | sonnet-5-verifier |
| 5 | `printf … > testdata/tmp-v2.md && --lint --root testdata/v2-smoke` | rc=0, contains `gates: 1 edge (reserved, not gating)` | exit 0 — `LINT: PASS`; output contains `gates: 1 edge (reserved, not gating)` | 2026-09-15 | sonnet-5-verifier |
| 6 | `go test . -run 'Demotion' -count=1 -v \| grep -c PASS` | ≥3 | exit 0 — 12 | 2026-09-15 | sonnet-5-verifier |
| 7 | `grep -c 'reconcile' statusgen/README.md` | ≥1 | exit 0 — 5 | 2026-09-15 | sonnet-5-verifier |
| 8 | `go vet ./... && ! grep -rn 'graphql' … ghfetch.go reconcile.go lifecycle.go briefv2.go` | exit 0 | exit 0 — `go vet` clean; no `graphql` match in the 4 brief-scoped files | 2026-09-15 | sonnet-5-verifier |

`RISK-VALUE: DERIVED — maxPages = 20 @ statusgen/ghfetch.go:106 (paired perPage = 100 @ :105)` —
hard cap against a malformed Link-header page loop; 20×100 = 2000 PRs bounds
`medici-finance/assay`'s real PR count with headroom (largest observed PR # in this run: 199),
so no witness is silently truncated; wrong value truncates but is reversible by edit+redeploy.
`RISK-VALUE: DERIVED — fail-closed guard `status != http.StatusOK` @ statusgen/ghfetch.go:113`
(paired `:173`) — the literal is the HTTP 200 protocol constant, not a tunable; re-proven live
by row 3 of this run (`GITHUB_TOKEN=invalid` → clean `lookedAt=false`, `HTTP 401: Bad
credentials`). `version = 1` legacy default @ `statusgen/reconcile.go:207` is reversible and
ranks last — unrelated to the row-4 defect, which is a match-logic bug, not a wrong literal.

**VERIFY: FAIL — row 4, a live engine defect, not a stale test anchor.** Rows 1,2,3,5,6,7,8
PASS clean. Row 4 fails because `reconcile`'s PR→brief witness matching does not resolve
legacy flat-form `Brief:` trailers (every merged PR in this repo's history, written before
the `derived-board/07` flag-day migration) against the post-migration hierarchical brief
`id`s — an exact-string match that can now never succeed, so **100% of PR-derived cells
(176/176) read `todo`**, masking every real completion the tree has, including this brief's
own deliverable (PR #199). This is worse than a cosmetic regression: it is precisely the
"board that lies with more authority than before" failure this brief's own `exec-tier-why`
flags as the highest-severity outcome for a derivation-engine bug. Status stays `implemented`
— no flip. Bug filed on `medici-finance/assay` (the repo the defect lives in) per the
verify-desk escalation contract; see the filed issue for the reproduction and root-cause
detail above.
### Non-implementer verifier re-run — VERIFY: FAIL (stale Verify-row anchors, not engine defects) — sonnet-5-verifier (verify-desk dispatch), @ merged main `951ca784d100a7d201a28a34033da6709ec2ec8f`, 2026-09-18

Runner ≠ implementer. Own detached temp worktree off origin/main. Offline envelope observed (`KUBECONFIG=/dev/null`). No PR opened, no push, no status flip attempted.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | `go test . -run Lifecycle/BriefV2/GHFetch -v` (x3) | ≥14 PASS | exit 0 each — Lifecycle=17, BriefV2=14, GHFetch=5, sum=36 | 2026-09-18 | sonnet-5-verifier |
| 2 | `reconcile --root . --offline --json`, assert every PR-source cell unknown | exit 0, ok | exit 0, ok | 2026-09-18 | sonnet-5-verifier |
| 3 | `GITHUB_TOKEN=invalid reconcile ... --json` | lookedAt=False, HTTP reason | exit 0, ok — reason "HTTP 401: Bad credentials" | 2026-09-18 | sonnet-5-verifier |
| 4 | `reconcile --repo medici-finance/assay --json`, lookup derived-board/02 by short id | rc=0 | **FAIL — rc=1, IndexError.** `id` field is now the brief-v2 hierarchical form (`assay:assay:derived-board:02`), not the short string this row's literal expects — a staleness from the derived-board/07 flag-day (bb2079bd, #736), separate from the trailer-join bug the 2026-09-15 pass found (now fixed on this HEAD via #1194). Re-queried by the correct id: engine is sound — `cell: implemented`, `witness: PR #80 (merged c93ae91)` | 2026-09-18 | sonnet-5-verifier |
| 5 | `--lint` on smoke tree, expect "gates: 1 edge (reserved, not gating)" | rc=0, substring present | **FAIL — rc=0 (LINT: PASS) but substring absent.** Commit 92aa88273 (#1251) intentionally made `gates:` an active gate, superseding this row's wording. Observed instead: `NOTICE: [eligibility-could-not-check] demo/01: held by rec:ingest/06...` — a deliberate later in-scope change, not a regression | 2026-09-18 | sonnet-5-verifier |
| 6 | `go test . -run Demotion -v` | ≥3 PASS | exit 0 — 13 | 2026-09-18 | sonnet-5-verifier |
| 7 | `grep -c reconcile statusgen/README.md` | ≥1 | exit 0 — 5 | 2026-09-18 | sonnet-5-verifier |
| 8 | `go vet ./...` + no graphql refs | exit 0 | exit 0, vet clean, no graphql match | 2026-09-18 | sonnet-5-verifier |

Full-package sanity: `go test ./...` ok (30.4s), `go build ./...` clean. Scope traceability: all 8 rows map 1:1 to their Verify rows; no invented scope.

RISK-VALUE: DERIVED — fail-closed guard `status != http.StatusOK` @ statusgen/ghfetch.go:113,173 — re-proven live via row 3 (invalid token → clean lookedAt=false). Top-ranked per the brief's own exec-tier-why ("a board that lies with more authority than before").
RISK-VALUE: DERIVED — `maxPages=20`/`perPage=100` @ statusgen/ghfetch.go:105-106 — bounds 2000 PRs; headroom vs real max PR# has shrunk from >4x (2026-09-06) to ~1.54x now (PR #1301) — still safe, flagged as a watch-item for the coordinator.
RISK-VALUE: NAMED, NOT DERIVED — `version := 1` legacy default @ statusgen/reconcile.go:203/207 — reversible operational default, unrelated to either failing row.

VERIFY: FAIL — rows 1,2,3,6,7,8 checked-clean. Rows 4 and 5 fail as literally written but both are stale Verify-row anchors from later, unrelated, in-scope changes (id-format flag-day; gates: becoming gating), not regressions in this brief's own code — third consecutive verify cycle (2026-09-06, 2026-09-15, 2026-09-18) hitting a different staleness cause on the same table. Filed medici-finance/assay#1305 recommending a re-baseline of rows 4 and 5. Status stays implemented, not advanced.

### Worker re-baseline of Verify rows 4+5 — 2026-09-19 glm-5.3[1m] worker (worker-desk dispatch), main `e41092057`

Verify-table maintenance per the verifier-filed issue #1305 (third staleness cycle on this
table; no engine change). Rows 4 and 5 re-anchored — row 4's lookup made id-shape-tolerant,
row 5's expected substring moved to the post-#1251 eligibility wording — and `version:`
bumped 1→2 (Verify-table edit after first dispatch). Fail-first reds observed on this tree
before the edit; greens observed with the re-baselined rows after:

| row | red — row as written, pre-edit | green — re-baselined row |
|-----|--------------------------------|--------------------------|
| 4 | flat literal `x['id']=='derived-board/02'` matches 0 of the briefs in live `reconcile --json` output (`lookedAt=true`, online read-only worker token; run per the verify lane the verifier passes use) → the row's `[0]` IndexErrors | `x['id'].endswith(':derived-board:02')` → `assay:assay:derived-board:02`, cell `implemented`, witness `PR #80 (merged c93ae91)`; assertion passes, prints `implemented` |
| 5 | `rc=0` but `gates: 1 edge (reserved, not gating)` absent (0 occurrences); observed `[eligibility-could-not-check] demo/01: held by rec:ingest/06 — could-not-check (alias rec is unpublished…)` | same command, new substring present, `LINT: PASS` |

Full-tree `--lint` before and after the edit: `LINT: PASS` both times, no new PROBLEM; one new
advisory NOTICE appears with the branch diff and is expected — `[verify-obligation]` derives a
`+flow` row as owed because the brief's authored `files:` continuation prose names
`docs/streams/graph-repos.yaml` (top-level `docs`) beside the `statusgen/` paths, so the
declared-path span is 2. This branch changes no component boundary (Verify-table maintenance
on this brief plus its changelog fragment), so the flow row genuinely does not apply; the
disposition is the notice's own review-time branch — said why in review on the PR, no `Class`
cell added (reshaping this table has already caused three staleness cycles). The generated Briefs table needs no edit:
the row's Status cell was `implemented` before and after (PR-witnessed; the maintenance PR
carries the same `Brief:` trailer and does not advance lifecycle), and the table is
single-writer generated.

### Non-implementer verifier run — VERIFY: BLOCKED — 6/8 pass, 2 could-not-check, 0 fail (offline→online hand-off; rows 3 and 4 need live forge state) — 2026-09-23 claude-opus-4-8-verifier

Runner is not the implementer. Isolated detached worktree cut off origin/main at the merged head (HEAD == origin/main == 39866201ce48acdce1f9b14d1cae38eb2b7eff38). statusgen tests/binary built from this worktree's own source. Offline envelope observed: KUBECONFIG=/dev/null; I ran no network command myself. Rows 1, 2, 5, 6, 7, 8 are checked-clean by direct offline execution. Rows 3 and 4 require live GitHub state, so under the offline dispatch envelope they are could-not-check by this verifier; the desk-mandated verifyrun witness (which had network on this host) executed both live and they passed exit 0 — recorded below as corroboration only, not as this verifier's sign-off. gate: model, all risk no.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | cd statusgen && go test . -run 'Lifecycle' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'BriefV2' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'GHFetch' -count=1 -v \| grep -c '^--- PASS' | ≥ 14 | PASS — exit 0; Lifecycle=17, BriefV2=14, GHFetch=5 (sum 36 ≥ 14); zero FAIL and zero SKIP lines across all three suites | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | cd statusgen && go run . reconcile --root . --offline --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert all(b['cell']=='unknown' for b in d['briefs'] if b['source']=='pr');print('ok')" | ok — no offline PR-derived cell is todo | PASS — exit 0; ok, every pr-source cell rendered unknown under the offline arm | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | cd statusgen && GITHUB_TOKEN=invalid go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert d['lookedAt']==False and d['reason'].startswith('HTTP');print('ok')" | ok — auth failure is unknown with the status | COULD-NOT-CHECK — offline envelope, needs live forge state; not run by this verifier. Corroboration: desk-mandated verifyrun witness ran it live, exit 0, output hash equal to row 2's ok output — the embedded assert (lookedAt=false, reason starts HTTP) held, i.e. a real HTTP 401 fail-closed | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | cd statusgen && go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);b=[x for x in d['briefs'] if x['id'].endswith(':derived-board:02')][0];assert b['cell'] in ('implemented','verified','done') and b['witness'].startswith('PR #80');print(b['cell'])" | prints the cell (dereferences merged PR #80) | COULD-NOT-CHECK — offline envelope, needs a live read token and forge state; not run by this verifier. Corroboration: desk-mandated verifyrun witness ran it live, exit 0 — the embedded assert held (derived-board/02 cell in the implemented/verified/done set with witness starting PR #80), so the id-shape-tolerant lookup dereferences the real merged deliverable | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | cd statusgen && printf -- '---\nbrief: x/01\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nschema: brief-v2\ngates: [{on: "rec:ingest/06", type: ordering-gate, reason: r}]\n---\n' > testdata/tmp-v2.md && go run . --lint --root testdata/v2-smoke; echo rc=$? | rc=0 and output contains the eligibility-could-not-check NOTICE naming rec:ingest/06 | PASS — exit 0; LINT: PASS; NOTICE [eligibility-could-not-check] demo/01: held by rec:ingest/06 — could-not-check (alias rec is unpublished) | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | cd statusgen && go test . -run 'Demotion' -count=1 -v \| grep -c PASS | ≥ 3 | PASS — exit 0; 13 (≥ 3); reopened-PR, red-witness, dismissed-approval, stale-version demotions | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | grep -c 'reconcile' statusgen/README.md | ≥ 1 | PASS — exit 0; 5 (≥ 1) | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | cd statusgen && go vet ./... && ! grep -rn 'graphql' --include=*.go ghfetch.go reconcile.go lifecycle.go briefv2.go | exit 0 | PASS — exit 0; go vet clean; no graphql match in the 4 brief-introduced files (run under bash; zsh mis-globs the --include token) | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE: DERIVED — perPage = 100 @ statusgen/ghfetch.go:105 paired with maxPages = 20 @ statusgen/ghfetch.go:106 — a hard page-loop cap so a malformed Link header cannot spin forever; 20 x 100 = 2000 PRs bounds this repo's real PR count. Headroom has shrunk over prior passes (largest observed PR number in the 2026-09-18 run was #1301, ~1.5x headroom, versus >4x in 2026-09-06). Still safe today, but a shrinking margin: if the repo's PR count approaches 2000, witnesses would be silently truncated. Wrong value is reversible by edit + redeploy. Flagged as a coordinator watch-item.

RISK-VALUE: DERIVED — fail-closed guard: status != http.StatusOK returns nil, false, httpReason(status, body) @ statusgen/ghfetch.go:113-114 — the literal is the HTTP 200 protocol constant, not a tunable; it is the three-state invariant the brief pins as hard (a non-200 must yield lookedAt=false with the status, never an empty "nothing found"). Corroborated live by row 3's clean 401 fail-close via the verifyrun witness.

RISK-VALUE: NAMED, NOT DERIVED — version legacy default = 1 @ statusgen/reconcile.go:203 and reconcile.go:207 — a reversible operational default applied when a brief carries no version; ranks last by irreversibility, unrelated to any failing/held row. No derivation attempted (out of scope for an offline pass; reversible knob).

Rows 3-4 need live forge state (online-lane hand-off). The verifyrun witness's row-1 fail is an instrument artifact: it scores only the last statement of a three-statement row; direct run is 17/14/5, all green.

### Non-implementer verifier run — VERIFY: PASS (8/8 rows as written; rows 3-4 run online read-only) — 2026-09-27 claude-opus-5-5-verifier (verify-desk dispatch), merged main `9585b4b6cc2e`

Runner is not the implementer. Isolated detached worktree cut off origin/main (HEAD == origin/main == 9585b4b6cc2ea8d35d367fb912e7c8216a765ba3). statusgen built from this worktree's own source via go run, not PATH. KUBECONFIG=/dev/null; no cluster or production endpoint touched. Rows 3 and 4 were run as read-only GitHub REST reads (row 3 with a deliberately invalid token; row 4 with the verifier App's read token) — the read-only forge lane the 2026-09-06, 2026-09-15 and 2026-09-18 passes used. gate: model, all risk no. Execution witness below.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd statusgen && go test . -run 'Lifecycle' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'BriefV2' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'GHFetch' -count=1 -v \| grep -c '^--- PASS'` | fail exit=0 | sha256:1c3f5832cdf0 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go run . reconcile --root . --offline --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert all(b['cell']=='unknown' for b in d['briefs'] if b['source']=='pr');print('ok')"` | pass exit=0 | sha256:dc51b8c96c2d | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && GITHUB_TOKEN=invalid go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert d['lookedAt']==False and d['reason'].startswith('HTTP');print('ok')"` | pass exit=0 | sha256:dc51b8c96c2d | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);b=[x for x in d['briefs'] if x['id'].endswith(':derived-board:02')][0];assert b['cell'] in ('implemented','verified','done') and b['witness'].startswith('PR #80');print(b['cell'])"` | pass exit=0 | sha256:59d5b5dd37b8 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && printf -- '---\nbrief: x/01\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nschema: brief-v2\ngates: [{on: "rec:ingest/06", type: ordering-gate, reason: r}]\n---\n' > testdata/tmp-v2.md && go run . --lint --root testdata/v2-smoke; echo rc=$?` | pass exit=0 | sha256:237cc6076406 | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go test . -run 'Demotion' -count=1 -v \| grep -c PASS` | pass exit=0 | sha256:1a252402972f | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'reconcile' statusgen/README.md` | pass exit=0 | sha256:f0b5c2c2211c | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go vet ./... && ! grep -rn 'graphql' --include=*.go ghfetch.go reconcile.go lifecycle.go briefv2.go` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ 9585b4b6cc2e (on-behalf-of human:ian) (forge-identity) |

Per-row key output (direct execution by this verifier, rows run from inside statusgen/ except row 7 at the repo root, under bash):

- Row 1: exit 0 per suite — Lifecycle=17, BriefV2=14, GHFetch=5, sum 36 (≥ 14); zero FAIL and zero SKIP lines across the three suites. PASS. The witness row above scores `fail` because verifyrun reads only the LAST count the three-statement row prints (GHFetch=5) against the ≥ 14 minimum; the Expect cell's own parenthetical (7 cells + 3 demotions + offline + 3 v2-parse cases) is a sum across the three suites, which holds. This is a check-definition shape, not an engine defect: the row as authored can never produce a passing witness, so `verifyrun --check` on this brief reports 7 pass, 1 fail indefinitely. Re-baseline suggestion: one summed count, e.g. a single `go test . -run 'Lifecycle|BriefV2|GHFetch' -count=1 -v | grep -c '^--- PASS'`.
- Row 2: exit 0, `ok` — 287 briefs read, all 287 pr-source cells render unknown under the offline arm.
- Row 3: exit 0, `ok` — lookedAt=false, reason "HTTP 401: Bad credentials" (read-only REST call with a deliberately invalid token; fail-closed).
- Row 4: exit 0, prints `implemented` — derived-board/02 (id assay:assay:derived-board:02) cell implemented, witness "PR #80 (merged c93ae91)"; whole run lookedAt=true, 287 briefs: 177 implemented, 11 in-progress, 99 todo. This brief's own cell: implemented, witness PR #1358.
- Row 5: exit 0, LINT: PASS, rc=0; output contains `[eligibility-could-not-check] demo/01: held by rec:ingest/06 — could-not-check (alias rec is unpublished ...)`. The fixture file the row writes was moved out of the worktree afterwards.
- Row 6: exit 0 — 13 (≥ 3).
- Row 7: exit 0 — 5 (≥ 1).
- Row 8: exit 0 — go vet clean; no graphql match in ghfetch.go, reconcile.go, lifecycle.go, briefv2.go.

What changed since the prior receipts (2026-09-07 verify-fail 7/8 on the stale row-4 anchor; 2026-09-24 blocked 6/8 with rows 3 and 4 could-not-check under the offline envelope): the Verify table and the brief body are unchanged; lifecycle.go, reconcile.go and briefv2.go are byte-identical to the 2026-09-24 receipt's hashes; ghfetch.go gained two additive read-only getters (issue comment, issue) for the corroborate ruling-link check (#1571), and brieffile.go, main.go and statusgen/README.md gained budget/outcome and corroborate additions (#1549, #1571, #1593) that do not touch the reconcile path. The substantive change in this pass is that rows 3 and 4 were RUN as read-only forge reads instead of being held as could-not-check, so the prior environment blocker is cleared.

Risk-bearing value enumeration (step 1) over the files this brief introduced or changed (ghfetch.go, reconcile.go, lifecycle.go, briefv2.go, and the brief-v2 branch of brieffile.go):

- perPage = 100 @ statusgen/ghfetch.go:107
- maxPages = 20 @ statusgen/ghfetch.go:108
- status != http.StatusOK (200) fail-closed guard @ statusgen/ghfetch.go:115 and :175
- reviews page size per_page=100 (single page, no paging) @ statusgen/ghfetch.go:170
- http.Client Timeout = 30 * time.Second @ statusgen/ghfetch.go:54
- githubAPIBase = "https://api.github.com" @ statusgen/ghfetch.go:47
- uuidV4Re (version nibble 4, variant [89ab]) @ statusgen/briefv2.go:78
- version must be >= 1 @ statusgen/briefv2.go:420; legacy default version = 1 @ statusgen/briefv2.go:205 and statusgen/reconcile.go:203, :207
- blockingIssueLabels = {question, needs-decision, help wanted} @ statusgen/lifecycle.go:108
- reconcileOK = 0, reconcileUsageErr = 2 @ statusgen/reconcile.go:47-48

Ranked by irreversibility: nothing here is irreversible — every value is corrected by an edit and a redeploy, and the tool only reads. Highest consequence is the page cap, because exceeding it silently truncates the witness set; next the fail-closed guard (the three-state invariant); the rest are reversible knobs or format validators.

RISK-VALUE: DERIVED — maxPages = 20 @ statusgen/ghfetch.go:108 (with perPage = 100 @ :107) — bounds the list at 2000 PRs against a malformed Link-header loop. This repository holds 1125 PRs today (search total_count; highest issue/PR number 1738), so the cap has about 1.78x headroom and no witness is truncated now. Watch-item, stated as a fact: when page 20 comes back full the loop exits with lookedAt=true and no truncation signal (ghfetch.go:109-126), and because the list is newest-first the dropped PRs are the OLDEST merges — the ones witnessing long-finished briefs — which would then derive todo with "PR search ran". Past 2000 PRs that is a negative printed without having looked; the fix is a lookedAt=false or explicit truncation reason on a full final page, not a larger constant.

RISK-VALUE: DERIVED — status != http.StatusOK @ statusgen/ghfetch.go:115 (paired :175) — the literal is the HTTP 200 protocol constant, not a tunable; it carries the brief's hard three-state invariant (any non-200 yields lookedAt=false with the status, never an empty "nothing found"). Re-proven live by row 3 in this pass (401 leads to lookedAt=false).

RISK-VALUE: NAMED, NOT DERIVED — legacy version default = 1 @ statusgen/reconcile.go:203/207 (and briefv2.go:205) — a reversible default for briefs that carry no version; ranks last. Open question for the desk only in the sense that no derivation was attempted; it is unrelated to any row.

Observation (not a Verify row; material, for the desk to route): the `reconcile` verb never populates the verified/done/blocked inputs of the lifecycle fold. LifecycleInput carries Witnesses, Approvals, Rulings and IssueLabels (statusgen/lifecycle.go:83-92), and deriveOne overlays verified/done and blocked from them, but the only production callers — the reconcile verb (statusgen/reconcile.go:88-112) and the regen drift comparator (statusgen/regen.go:91-110) — set only Briefs, PRs, LookedAt and Reason; a repo-wide search finds no non-test assignment to the other four. The live cell therefore tops out at `implemented`: in this pass's online run no brief derived verified or done, while about 84 briefs whose README row reads done derived implemented and 3 README-blocked briefs derived todo. That contradicts this brief's own facts line ("verified/done derivation is the EXISTING code path (verifyrun --check, approval at head); this brief calls it") and its title ("from PRs, witnesses, approvals and rulings"). No Verify row catches it — row 4 accepts implemented as well as verified or done, and the fixture tests feed the maps directly. ghfetch.ReviewsAtHead exists but has no caller outside its test. The regen drift comparator will report every done brief as drift. The row verdict stands at PASS; this gap needs its own filed item (verified/done/blocked overlay unwired in the reconcile verb) and possibly a Verify row that asserts a known-done brief derives done.

VERIFY: PASS — 8/8 Verify rows pass as written on merged main 9585b4b6cc2e by direct execution (rows 3 and 4 as read-only forge reads). The execution witness scores row 1 fail because of the three-count row shape above (7 pass, 1 fail on `verifyrun --check`), so the witness gate will not clear this brief until row 1 is re-baselined. gate: model, all risk no.
### Non-implementer verifier run — VERIFY: PASS (8/8 rows as written; rows 3-4 run online read-only) — 2026-10-01 claude-opus-5-5[1m] verifier (verify-desk dispatch), merged main `024c87b01aba`

What moved since the last run (2026-09-27 PASS at 9585b4b6cc2e, whose Evidence PR was closed because its Runner cells did not carry the verifier App stamp form): the brief's Verify table and body are unchanged (the only diff to the brief is the 2026-09-27 Evidence block). statusgen/ghfetch.go, statusgen/reconcile.go, statusgen/lifecycle.go and statusgen/briefv2.go have no diff against 9585b4b6cc2e. statusgen/brieffile.go gained only critical-tier ride-along fields (Unblocks, IssueRefs) and comment text (#1822, #1948). statusgen/README.md gained 22 lines (#1808, #1822). None of these touches the reconcile path. This pass re-runs every row and stamps each Runner cell with the verifier App identity.

Runner is not the implementer. Isolated detached worktree cut off refs/remotes/origin/main (HEAD == origin/main == 024c87b01aba8f6c7dd7ccd939e647a9b936be09). statusgen was built from this worktree's own source via go run, not from PATH. KUBECONFIG=/dev/null. No cluster or production endpoint was touched. Row commands ran under bash from inside statusgen/, except row 7, which ran from the repo root. Row 3 made a read-only GitHub REST read with a deliberately invalid token. Row 4 made a read-only REST read with the verifier App's read token. No forge write was made. gate: model; risk metadata is present and every field reads no.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 (Verify row 1) | `cd statusgen && go test . -run 'Lifecycle' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'BriefV2' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'GHFetch' -count=1 -v \| grep -c '^--- PASS'` | ≥ 14 (7 cells + 3 demotions + offline + 3 v2-parse cases) | exit 0; prints 17, 14, 5 (Lifecycle, BriefV2, GHFetch), sum 36 ≥ 14; a separate grep for FAIL/SKIP lines in each of the three suites counted 0 | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 (Verify row 2) | `cd statusgen && go run . reconcile --root . --offline --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert all(b['cell']=='unknown' for b in d['briefs'] if b['source']=='pr');print('ok')"` | `ok` | exit 0, `ok`; 294 briefs, all 294 pr-source cells unknown; top-level lookedAt=false, reason "offline (--offline) — the PR fetch was not attempted" | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 (Verify row 3) | `cd statusgen && GITHUB_TOKEN=invalid go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert d['lookedAt']==False and d['reason'].startswith('HTTP');print('ok')"` | `ok` — auth failure is unknown with the status | exit 0, `ok`; lookedAt=False, reason "HTTP 401: Bad credentials" | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 (Verify row 4) | `cd statusgen && go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);b=[x for x in d['briefs'] if x['id'].endswith(':derived-board:02')][0];assert b['cell'] in ('implemented','verified','done') and b['witness'].startswith('PR #80');print(b['cell'])"` | prints the cell; witness dereferences merged PR #80 | exit 0, prints `implemented`; id assay:assay:derived-board:02, witness "PR #80 (merged c93ae91)"; whole run lookedAt=true with 294 briefs (194 implemented, 94 todo, 6 in-progress); this brief's own cell is implemented, witness PR #1358 (merged f645d2f) | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 (Verify row 5) | `cd statusgen && printf -- '---\nbrief: x/01\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nschema: brief-v2\ngates: [{on: "rec:ingest/06", type: ordering-gate, reason: r}]\n---\n' > testdata/tmp-v2.md && go run . --lint --root testdata/v2-smoke; echo rc=$?` | `rc=0` and output contains `[eligibility-could-not-check] demo/01: held by rec:ingest/06` | exit 0, LINT: PASS, rc=0; output contains "NOTICE: [eligibility-could-not-check] demo/01: held by rec:ingest/06 — could-not-check (alias rec is unpublished — its target repo is not resolvable from this tree)". The fixture file the row writes was deleted from the worktree afterwards | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 (Verify row 6) | `cd statusgen && go test . -run 'Demotion' -count=1 -v \| grep -c PASS` | ≥ 3 | exit 0; 13; zero FAIL or SKIP lines | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 (Verify row 7) | `grep -c 'reconcile' statusgen/README.md` | ≥ 1 | exit 0; 5 | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 (Verify row 8) | `cd statusgen && go vet ./... && ! grep -rn 'graphql' --include=*.go ghfetch.go reconcile.go lifecycle.go briefv2.go` | exit 0 | exit 0; go vet clean; no graphql match in the four files | 2026-10-01 | assay-verifier-app[bot] @ 024c87b01aba (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Scope traceability: every Evidence row above discharges the Verify row of the same number. No verified work maps to no Verify row.

Row 1 shape note (already filed as #1909, still open): the row prints three counts and only the sum meets the Expect. A tool that reads just the last count (GHFetch=5) will score the row below its ≥ 14 minimum, even though the Expect's own parenthetical is a sum across all three suites and that sum holds. As a cross-check, this pass also ran the summed single-count form `go test . -run 'Lifecycle|BriefV2|GHFetch' -count=1 -v | grep -c '^--- PASS'`, which printed 36 with exit 0. This pass did not run statusgen verifyrun, so no witness table is attached. The desk's landing step produces the witness.

Risk-bearing value enumeration (step 1) covered the files this brief introduced or changed: statusgen/ghfetch.go, statusgen/reconcile.go, statusgen/lifecycle.go, statusgen/briefv2.go, and the brief-v2 branch of statusgen/brieffile.go. The brieffile.go delta since the last run adds no literal.

- perPage = 100 @ statusgen/ghfetch.go:107
- maxPages = 20 @ statusgen/ghfetch.go:108
- status != http.StatusOK (200) fail-closed guard @ statusgen/ghfetch.go:115, paired at :175
- reviews per_page=100 (single page) @ statusgen/ghfetch.go:170
- http.Client Timeout = 30 * time.Second @ statusgen/ghfetch.go:54
- githubAPIBase = "https://api.github.com" @ statusgen/ghfetch.go:47
- uuidV4Re (version nibble 4, variant 8/9/a/b) @ statusgen/briefv2.go:78
- version lower bound: Version < 1 is refused @ statusgen/briefv2.go:420; legacy default version = 1 @ statusgen/briefv2.go:205 and statusgen/reconcile.go:203, :207
- blockingIssueLabels = {question, needs-decision, help wanted} @ statusgen/lifecycle.go:108
- reconcileOK = 0, reconcileUsageErr = 2 @ statusgen/reconcile.go:47-48

Ranked by irreversibility: none of these is irreversible. The tool only reads, and an edit plus a redeploy corrects any of them. The page cap ranks first because exceeding it truncates the witness set silently. The fail-closed status guard ranks second because it carries the three-state invariant. The rest are reversible knobs or format validators.

RISK-VALUE: DERIVED — maxPages = 20 @ statusgen/ghfetch.go:108 (with perPage = 100 @ :107) — the cap bounds the list at 2000 PRs so a malformed Link-header loop cannot spin forever. This repository holds 1246 PRs today (read-only search total_count; the highest issue/PR number is 1967), so the cap has about 1.6x headroom and no witness is truncated now. Headroom has fallen from about 1.78x at the last run. Watch-item: if page 20 comes back full, the loop exits with lookedAt=true and no truncation signal. The list is newest-first, so the dropped PRs would be the oldest merges, and those briefs would then derive todo as though the search had looked. The fix is to report lookedAt=false or an explicit truncation reason on a full final page, not to raise the constant. Filed as #1969 during this run.

RISK-VALUE: DERIVED — status != http.StatusOK @ statusgen/ghfetch.go:115 (paired :175) — the literal is the HTTP 200 protocol constant, not a tunable. It enforces the brief's hard three-state invariant: any non-200 response yields lookedAt=false with the status, never an empty "nothing found". Row 3 re-proved this live in this pass (401 → lookedAt=false).

RISK-VALUE: NAMED, NOT DERIVED — legacy version default = 1 @ statusgen/reconcile.go:203/207 (and statusgen/briefv2.go:205) — a reversible default for briefs that carry no version. Deciding whether it should be stated and pinned or reported as could-not-check is a design choice, not a derivation this verifier can make. That question is already routed as #1910 (open).

Observation, not a Verify row: the reconcile verb still never populates the lifecycle fold's verified/done/blocked inputs. The live run above derived no brief as verified or done, so live cells stop at implemented. This is already filed as #1787 (open). No Verify row catches it, because row 4 also accepts implemented.

rows_passed=8 rows_total=8

RISK-VALUE: DERIVED — maxPages = 20 @ statusgen/ghfetch.go:108 — 2000-PR bound vs 1246 PRs today (about 1.6x headroom); truncation on a full final page is unsignalled (filed #1969). RISK-VALUE: DERIVED — status != http.StatusOK @ statusgen/ghfetch.go:115 — protocol constant enforcing fail-closed lookedAt=false. RISK-VALUE: NAMED, NOT DERIVED — version default = 1 @ statusgen/reconcile.go:207 — routed as #1910.

VERIFY: PASS

### Non-implementer verifier re-run: 2026-10-02T22:19:03Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main e1d99484ffd91b649ea45e10a1cecf4ba2a4924b

Runner is not the implementer. Isolated detached worktree at merged main e1d99484ffd91b649ea45e10a1cecf4ba2a4924b; the worktree was not moved. statusgen was built from this worktree's own source via go run for the rows. KUBECONFIG=/dev/null; no cluster or production endpoint was touched. Row commands ran under bash exactly as authored, from the repo root. Disclosures: every row ran with a throwaway HOME and with TMPDIR pointed at the verifier's own scratch directory (Go build and module caches left at their normal locations), so nothing was written to the system temp directory or the live audit log. Row 3 made a read-only REST read with a deliberately invalid token. Row 4 made a read-only REST read with the verifier App's read token. Row 5 writes one untracked fixture file under statusgen/testdata as authored (no tracked file is edited); it was removed afterwards and the worktree is clean. No forge write was made. gate: model; risk metadata is present and every field reads no (irreversible: no).

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | `cd statusgen && go test . -run 'Lifecycle' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'BriefV2' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'GHFetch' -count=1 -v \| grep -c '^--- PASS'` | ≥ 14 (7 cells + 3 demotions + offline + 3 v2-parse cases) | exit 0; prints three counts: 18, 14, 5 (Lifecycle, BriefV2, GHFetch), sum 37. The sum and the first two counts meet ≥ 14; the last count (5) does not. With -v, each suite ended `ok` with 0 FAIL and 0 SKIP lines, so the counts are real passes, not a vacuous pattern. Which of the three numbers the Expect binds is not stated by the row (see finding 1) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | `cd statusgen && go run . reconcile --root . --offline --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert all(b['cell']=='unknown' for b in d['briefs'] if b['source']=='pr');print('ok')"` | `ok` | exit 0, `ok`; 311 briefs, all 311 are pr-source and all render unknown (so the assert is not vacuous); top-level lookedAt=false, reason "offline (--offline) — the PR fetch was not attempted" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | `cd statusgen && GITHUB_TOKEN=invalid go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert d['lookedAt']==False and d['reason'].startswith('HTTP');print('ok')"` | `ok` — an auth failure is unknown with the status | exit 0, `ok`; lookedAt=False, reason "HTTP 401: Bad credentials" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | `cd statusgen && go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);b=[x for x in d['briefs'] if x['id'].endswith(':derived-board:02')][0];assert b['cell'] in ('implemented','verified','done') and b['witness'].startswith('PR #80');print(b['cell'])"` | prints the cell; witness dereferences merged PR #80 | exit 0, prints `implemented`; id assay:assay:derived-board:02, witness "PR #80 (merged c93ae91)"; whole run lookedAt=true, 311 briefs (207 implemented, 102 todo, 2 in-progress; none verified, done or blocked); this brief's own cell is implemented, witness "PR #1358 (merged f645d2f)" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | `cd statusgen && printf -- '---\nbrief: x/01\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nschema: brief-v2\ngates: [{on: "rec:ingest/06", type: ordering-gate, reason: r}]\n---\n' > testdata/tmp-v2.md && go run . --lint --root testdata/v2-smoke; echo rc=$?` | `rc=0` and output contains `[eligibility-could-not-check] demo/01: held by rec:ingest/06` | exit 0; LINT: PASS; rc=0; output contains "NOTICE: [eligibility-could-not-check] demo/01: held by rec:ingest/06 — could-not-check (alias rec is unpublished — its target repo is not resolvable from this tree)" | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | `cd statusgen && go test . -run 'Demotion' -count=1 -v \| grep -c PASS` | ≥ 3 | exit 0; 14. With -v: 13 top-level `--- PASS` lines (reopened-PR, red-witness, dismissed-approval, stale-version and coverage-not-released lifecycle demotions among them) plus the final PASS line, 0 FAIL, 0 SKIP | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 7 | `grep -c 'reconcile' statusgen/README.md` | ≥ 1 | exit 0; 9 | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 8 | `cd statusgen && go vet ./... && ! grep -rn 'graphql' --include=*.go ghfetch.go reconcile.go lifecycle.go briefv2.go` | exit 0 | exit 0; go vet clean; no graphql match. All four named files exist, so the negated grep is a real no-match and not a missing-file pass | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

By hand: rows 2 to 8 are checked-clean (7 of 8). Row 1 executes cleanly and its tests pass, but its verdict depends on a reading the row does not fix, so it is recorded as check-definition, neither a pass nor an engine failure.

Execution witness (`statusgen verifyrun --brief docs/streams/derived-board/brief-03-reconcile-derivation.md --dry-run`, released statusgen v1.0.31, from the worktree root, throwaway HOME, verifier read token in env for row 4; dry-run wrote nothing to the brief): exit 1, 7 pass, 1 fail of 8.

- row 1: fail (exit=0) — "count 5 is below the expected minimum 14"
- rows 2, 3, 4, 5: pass (exit=0), scored on exit status only
- rows 6, 7, 8: pass (exit=0)

History of the verdict on this brief. The 2026-09-27 Evidence block is headed PASS (8/8 by hand at 9585b4b6cc2e), and the outcome record for that same run (2026-09-27T15:10:38Z) is `blocked`, check-definition, citing #1787: the record and the PASS heading describe one run, read two ways. The 2026-10-01 Evidence block (at 024c87b01aba, landed with #1973) is again headed PASS and post-dates that record, but it has no outcome record of its own and states that it did not run the witness. Neither PASS reflects a change to the check: `git log -p` on the brief shows the Verify table last changed on 2026-09-20 (#1358, rows 4 and 5); row 1 is byte-identical since.

Findings.

1. Row 1 check-definition is unchanged and still blocks the witness gate. The row prints three counts and its Expect (≥ 14) is their sum per its own parenthetical; the witness reads only the last count (5) and scores fail. As authored the row cannot produce a passing witness on any tree. A single summed count, `go test . -run 'Lifecycle|BriefV2|GHFetch' -count=1 -v | grep -c '^--- PASS'`, printed 37 with exit 0 in this pass. Tracked in #1787 and #1909, both open at the time of this run.
2. The engine gap in #1787 is not fixed either. A search of non-test statusgen sources finds no assignment to the lifecycle fold's Witnesses, Approvals, Rulings or IssueLabels inputs (declared at statusgen/lifecycle.go:99-102, read at :182, :232, :281, :285); statusgen/reconcile.go and statusgen/regen.go are byte-identical to the 2026-09-27 record's hashes, and ReviewsAtHead (statusgen/ghfetch.go:169) still has no caller outside its test. Consistent with that, the live run in row 4 derived 0 of 311 briefs as verified, done or blocked. No Verify row catches this, because row 4 also accepts implemented.
3. What did change since the 2026-09-27 record: statusgen/lifecycle.go and statusgen/briefv2.go (coverage-not-released demotion from #1682, stream/NN id keying from #1966; this is why Lifecycle now counts 18 and Demotion 14), statusgen/brieffile.go, statusgen/main.go and statusgen/README.md (README count 5 to 9). None of these wires the four fold inputs or touches row 1. statusgen/ghfetch.go, statusgen/reconcile.go, statusgen/regen.go, statusgen/trustgate.go, the stream spec and graph-repos.yaml are unchanged.
4. No previously failing or held row regressed: rows 3 and 4 (held as could-not-check on 2026-09-24) ran live and are clean; rows 4 and 5 (stale anchors until the 2026-09-20 re-baseline) are clean as re-baselined.

Risk-bearing value enumeration, over the files this brief introduced or changed (statusgen/ghfetch.go, statusgen/reconcile.go, statusgen/lifecycle.go, statusgen/briefv2.go and the brief-v2 branch of statusgen/brieffile.go):

- githubAPIBase = "https://api.github.com" @ statusgen/ghfetch.go:47
- http.Client Timeout = 30 * time.Second @ statusgen/ghfetch.go:54
- perPage = 100 @ statusgen/ghfetch.go:107
- maxPages = 20 @ statusgen/ghfetch.go:108
- status != http.StatusOK (200) @ statusgen/ghfetch.go:115, paired at :175
- uuidV4Re (version nibble 4, variant 8/9/a/b) @ statusgen/briefv2.go:78
- version lower bound, Version < 1 refused @ statusgen/briefv2.go:427; legacy default Version = 1 @ statusgen/briefv2.go:212 and version = 1 @ statusgen/reconcile.go:203, :207
- blockingIssueLabels = {question, needs-decision, help wanted} @ statusgen/lifecycle.go:121
- reconcileOK = 0, reconcileUsageErr = 2 @ statusgen/reconcile.go:47-48

Ranked by irreversibility: none is irreversible. The tool only reads, and an edit plus a redeploy corrects any of them. The page cap ranks first because exceeding it truncates the witness set without a signal; the fail-closed status guard second because it carries the three-state invariant; the rest are reversible knobs or format validators.

RISK-VALUE: DERIVED — maxPages = 20 @ statusgen/ghfetch.go:108 (with perPage = 100 @ statusgen/ghfetch.go:107) — bounds the list at 2000 PRs so a malformed Link-header loop cannot spin forever. The repository holds 1304 PRs today (read-only search total_count), about 1.53x headroom, down from about 1.6x on 2026-10-01; nothing is truncated now. A full 20th page still returns lookedAt=true with no truncation signal; that is tracked in #1969 (open).

RISK-VALUE: DERIVED — status != http.StatusOK @ statusgen/ghfetch.go:115 (paired at :175) — the literal is the HTTP 200 protocol constant, not a tunable; any other status yields lookedAt=false with the status text. Re-observed live in row 3 (401 gives lookedAt=False, reason "HTTP 401: Bad credentials").

RISK-VALUE: NAMED, NOT DERIVED — version = 1 @ statusgen/reconcile.go:203 (and :207; statusgen/briefv2.go:212) — a reversible default for a brief that carries no version. Whether it should be pinned or reported as could-not-check is a design choice this verifier cannot derive; routed as #1910 (open).

rows_passed=7 rows_total=8

VERIFY: BLOCKED — check-definition: Verify row 1 prints three counts against a single ≥ 14 Expect, so the execution witness scores it fail (7/8) although every test in the three suites passes (18, 14, 5; sum 37); the row is unchanged since the 2026-09-27 block and #1787 is still open with neither of its two parts fixed. Rows 2 to 8 are checked-clean on merged main e1d99484ffd91b649ea45e10a1cecf4ba2a4924b. Status stays implemented.
### Non-implementer verifier re-run — VERIFY: BLOCKED (check-definition; #1787 still reproduces) — 2026-10-07, assay-verifier-app[bot] (on-behalf-of human:ian), merged main 91f04b81ba064394aa121a4940cf11a138b55402

The runner is not the implementer. The worktree was detached at merged main 91f04b81ba064394aa121a4940cf11a138b55402 (fetched 2026-10-07, equal to origin/main). Rows ran under bash exactly as authored, from the repo root, with go run building statusgen from this worktree's source. KUBECONFIG=/dev/null. No cluster or production endpoint was touched. Row 3 made a read-only REST read with a deliberately invalid token. Row 4 made a read-only REST read with the verifier App's read token. No forge write was made. Row 5 writes one untracked fixture file under statusgen/testdata as authored; it was removed afterwards and the worktree is clean. gate: model. Risk metadata is present and every field reads no (irreversible: no).

What changed since the 2026-10-02 run (e1d99484): among the reconcile-path files, only statusgen/brieffile.go and statusgen/main.go changed. The brieffile.go change is a read memo (readDirMemo/readFileMemo) plus a waitingBriefNotice refactor. The main.go change adds missionProblems to --lint and new cluster-pending doc text. statusgen/lifecycle.go, reconcile.go, regen.go, ghfetch.go and briefv2.go are unchanged. The brief's Verify table is unchanged.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | `cd statusgen && go test . -run 'Lifecycle' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'BriefV2' -count=1 -v \| grep -c '^--- PASS'; go test . -run 'GHFetch' -count=1 -v \| grep -c '^--- PASS'` | ≥ 14 (7 cells + 3 demotions + offline + 3 v2-parse cases) | exit 0. Prints three counts: 18, 14, 5 (Lifecycle, BriefV2, GHFetch), sum 37. The combined -v run of the three suites ended `ok` with 37 `--- PASS` lines and 0 FAIL or SKIP lines. The sum meets ≥ 14, but the last count (5) does not, and the row does not say which number the Expect binds. The execution witness scores this row as fail ("count 5 is below the expected minimum 14"). Recorded as check-definition (#1909, open) | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd statusgen && go run . reconcile --root . --offline --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert all(b['cell']=='unknown' for b in d['briefs'] if b['source']=='pr');print('ok')"` | `ok` — offline never renders a PR-derived cell as todo | exit 0, `ok`. 367 briefs, all 367 pr-source, all unknown, so the assert checks every brief. Top-level lookedAt=false, reason "offline (--offline) — the PR fetch was not attempted" | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd statusgen && GITHUB_TOKEN=invalid go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);assert d['lookedAt']==False and d['reason'].startswith('HTTP');print('ok')"` | `ok` — an auth failure is unknown with the status | exit 0, `ok`; lookedAt=False, reason "HTTP 401: Bad credentials" | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd statusgen && go run . reconcile --root . --repo medici-finance/assay --json \| python3 -c "import json,sys;d=json.load(sys.stdin);b=[x for x in d['briefs'] if x['id'].endswith(':derived-board:02')][0];assert b['cell'] in ('implemented','verified','done') and b['witness'].startswith('PR #80');print(b['cell'])"` | prints the cell; witness dereferences merged PR #80 | exit 0, prints `implemented`; id assay:assay:derived-board:02, witness "PR #80 (merged c93ae91)". The whole run had lookedAt=true across 367 briefs: 215 implemented, 152 todo, and none verified, done, blocked or in-progress. This brief's own cell is implemented, witness "PR #1358 (merged f645d2f)" | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && printf -- '---\nbrief: x/01\ntitle: t\nwave: 0\ndepends: []\nunblocks: []\neffort: S\ngate: model\nrisk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}\nschema: brief-v2\ngates: [{on: "rec:ingest/06", type: ordering-gate, reason: r}]\n---\n' > testdata/tmp-v2.md && go run . --lint --root testdata/v2-smoke; echo rc=$?` | `rc=0` and output contains `[eligibility-could-not-check] demo/01: held by rec:ingest/06` | exit 0; LINT: PASS; rc=0. Output contains "NOTICE: [eligibility-could-not-check] demo/01: held by rec:ingest/06 — could-not-check (alias rec is unpublished — its target repo is not resolvable from this tree)" | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd statusgen && go test . -run 'Demotion' -count=1 -v \| grep -c PASS` | ≥ 3 | exit 0; 14 | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'reconcile' statusgen/README.md` | ≥ 1 | exit 0; 9 | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd statusgen && go vet ./... && ! grep -rn 'graphql' --include=*.go ghfetch.go reconcile.go lifecycle.go briefv2.go` | exit 0 | exit 0. go vet is clean and grep finds no graphql match. All four named files exist, so the no-match is real and not caused by missing files | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |

Execution witness: non-dry `statusgen verifyrun --brief` with released statusgen v1.0.32, run from the worktree root on a clean tree with the verifier read token in env. Exit 1, 7 pass and 1 fail of 8. Row 1 failed with "count 5 is below the expected minimum 14"; rows 2 to 8 passed. The witness table is attached separately.

By hand, rows 2 to 8 are checked-clean (7 of 8). Row 1 runs cleanly and every test in it passes. Its verdict depends on a reading the row does not fix, so it is recorded as check-definition: neither a pass nor an engine failure.

Does the #1787 blocker still reproduce? Yes.
- A search of non-test statusgen sources finds no assignment to the lifecycle fold's Witnesses, Approvals, Rulings or IssueLabels inputs (declared in statusgen/lifecycle.go LifecycleInput). Both production constructors still build `LifecycleInput{Briefs: idents}`, at statusgen/reconcile.go:88 and statusgen/regen.go:94. ReviewsAtHead (statusgen/ghfetch.go:169) is still called only from ghfetch_test.go.
- The live row-4 run derived 0 of 367 briefs as verified, done or blocked. Compared with the stream README Status cells on the same tree: 102 README-done briefs derive implemented, and 4 derive todo. 9 README-verified briefs derive implemented. All 5 README-blocked briefs derive implemented (2) or todo (3). derived-board/02 is done in its README and derives implemented.
- This contradicts the brief's facts line ("verified/done derivation is the EXISTING code path ... this brief calls it") and its title ("from PRs, witnesses, approvals and rulings"). No Verify row catches it, because row 4 also accepts implemented. assay#1787 is open (last updated 2026-09-27); assay#1909 is open.

Grounding note: the Context files list names a statusgen/testdata/lifecycle fixture directory, and it does not exist. The fixture matrix lives in Go test files instead (lifecycle_test.go, lifecycle_hierarchical_test.go and others). No Verify row names the directory.

Risk-bearing value enumeration (step 1) covered statusgen/ghfetch.go, statusgen/reconcile.go, statusgen/lifecycle.go, statusgen/briefv2.go and the brief-v2 branch of statusgen/brieffile.go. The brieffile.go delta since the last run adds no literal.

- githubAPIBase = "https://api.github.com" @ statusgen/ghfetch.go:47
- http.Client Timeout = 30 * time.Second @ statusgen/ghfetch.go:54
- perPage = 100 @ statusgen/ghfetch.go:107
- maxPages = 20 @ statusgen/ghfetch.go:108
- status != http.StatusOK (200) @ statusgen/ghfetch.go:115, paired at :175
- uuidV4Re (version nibble 4, variant 8/9/a/b) @ statusgen/briefv2.go:78
- Version < 1 refused @ statusgen/briefv2.go:427; legacy default Version = 1 @ statusgen/briefv2.go:212; version = 1 @ statusgen/reconcile.go:203, :207
- blockingIssueLabels = {question, needs-decision, help wanted} @ statusgen/lifecycle.go:121
- reconcileOK = 0, reconcileUsageErr = 2 @ statusgen/reconcile.go:47-48

Ranked by irreversibility: none of these is irreversible. The tool only reads, and an edit plus a redeploy corrects any of them. The page cap ranks first because exceeding it truncates the witness set without a signal. The fail-closed status guard ranks second because it carries the three-state invariant. The rest are reversible knobs or format validators.

RISK-VALUE: DERIVED — maxPages = 20 @ statusgen/ghfetch.go:108 (with perPage = 100 @ statusgen/ghfetch.go:107) — the cap bounds the list at 2000 PRs so a malformed Link-header loop cannot spin forever. The repository holds 1447 PRs today (read-only search total_count), about 1.38x headroom. That is down from about 1.53x on 2026-10-02 and 1.6x on 2026-10-01, so roughly 550 PRs remain before truncation. Nothing is truncated now. A full 20th page still returns lookedAt=true with no truncation signal, tracked in #1969 (open). At the recent rate of growth this becomes a live defect within weeks.

RISK-VALUE: DERIVED — status != http.StatusOK @ statusgen/ghfetch.go:115 (paired at :175) — the literal is the HTTP 200 protocol constant, not a tunable. Any other status yields lookedAt=false with the status text. Row 3 re-observed this live: a 401 gives lookedAt=False with reason "HTTP 401: Bad credentials".

RISK-VALUE: NAMED, NOT DERIVED — version = 1 @ statusgen/reconcile.go:203 (and :207; statusgen/briefv2.go:212) — a reversible default for a brief that carries no version. Whether it should be pinned or reported as could-not-check is a design choice this verifier cannot derive. Routed as #1910 (open).

rows_passed=7 rows_total=8

VERIFY: BLOCKED — check-definition. Verify row 1 prints three counts against a single ≥ 14 Expect, so the execution witness scores it fail (7/8), although all 37 tests pass (18, 14, 5). The engine gap in #1787 also still reproduces on this main: reconcile never populates the fold's Witnesses, Approvals, Rulings or IssueLabels, and 0 of 367 live cells derive above implemented. Rows 2 to 8 are checked-clean on merged main 91f04b81ba064394aa121a4940cf11a138b55402. Status stays implemented.

Blocker re-confirmed and attached: https://github.com/medici-finance/assay/issues/1787#issuecomment-6037701998. The NAMED, NOT DERIVED value (version = 1 at statusgen/reconcile.go:203) is already routed to https://github.com/medici-finance/assay/issues/1910.

<!-- appended at rework time (#1787) -->

Rework on `feat/assay--derived-board--03`, covering the #1787 defect.

**The defect.** Both production callers (`reconcile.go`, and the drift comparator in `regen.go`) built `LifecycleInput{Briefs: idents}` and nothing else. The fold's Witnesses, Approvals, Rulings and IssueLabels inputs were therefore always nil, and no live cell derived above `implemented`: 0 of 367 at the 2026-10-07 verify.

**The fix.** New `statusgen/reconcilefold.go` is the single wiring point both callers share. It makes no promotion rule of its own. Each fold input is read by the rule that the state's existing writer already enforces:

- **verified**
  - The latest strict PASS run comes from `flipLatestPass`, which is extracted from verify-gate-close's `flipStampFromEvidence`. It requires the bold marker and Date/Runner rows naming `<login> @ <sha>`.
  - `closeVerifyHeldRefusal` and `closeVerifyFailRefusal` must not refuse it.
  - The Evidence audit must pass (`closureWitnesses`, which is the `verifyrun --check` path).
  - The PASS lines must have been committed by the roster's verifier (`flipProvenance`).
  - The brief version is read at the run's sha (`briefVersionAt`), never copied from the current brief.
  - Coverage must be released (`evaluateCoverage`).
- **gate:model done**: `decideModelFlip`, the auto-flip's own decision. It requires the bound reviewer's approval at the merged head. A could-not-check result is `unknown` and is disclosed.
- **gate:human done**: the anchored Reviewed-cell `human:<name>` stamp. That is the stamp the close writes and that the existing done-row lint requires. Relays and substrings never count.
- **blocked**: one paged open-issues read. PR entries are filtered out. A capped or failed read leaves every brief that links an issue `unknown`.

**Overlay and refusals.**
- The witness overlay applies only over an `implemented` base, which means a merged PR.
- Every input that could not be read is listed in the JSON `unread` field, and `--apply` refuses with exit 3 while that list is non-empty.
- `--offline` keeps every map nil (row 2 is unchanged).

**Drift comparator.** Its join was blind on brief-v2 hierarchical ids. Both sides now key on `canonicalBriefKey`/`briefStreamNum`.

**Verify table.**
- Row 1 is re-authored to per-suite floors.
- Row 9 asserts the fixture `done` path and the live `blocked` path. It does not assert a live `done` (see row 9's Expect: no recorded Evidence on the tree meets the flip owner's rule today).
- The table gains a `Class` column. Every row stays the runner-executed `check` it was by default; row 9 (production `reconcile` end to end, tree plus forge) carries `+flow`, and new row 10 carries `+mutation`. Row 10 runs the fold's mutation set (`statusgen/reconcilefold-mutations.json`, 18 guard removals plus a positive control) through the existing `muhar` harness.

Fail-first (clause 9). Each guard was removed in turn, and the named test went red. The first failing line of each:

```
anchored human stamp ............ TestHumanSignoffAnchored: humanSignoff("2026-10-05 superhuman:x") = "human:x", want ""
held-row refusal ................ TestFoldHeldRowNoWitness: cell = "verified", want implemented
verifier provenance ............. TestFoldImplementerPassNoWitness: cell = "done", want implemented
version read at the run's sha ... TestFoldStaleVersionUnknown: cell = "done", want unknown
closure audit ................... TestFoldRefusedAuditNoWitness: cell = "done", want implemented
approval unchecked -> unknown ... TestFoldModelUncheckedUnknown: cell = "verified", want unknown
at-head check (isDone) .......... TestLifecycleDemotionApprovalNotAtHead: got "done"
gate check (isDone) ............. TestLifecycleDemotionModelGateRuling: got "done"
merged-PR overlay requirement ... TestFoldUnmergedWitnessUnknown: cell = "verified", want unknown
ListPRs merged check ............ TestFoldUnmergedWitnessUnknown: cell = "done", want unknown
PR entries filtered from issues . TestFoldIssuePRsFiltered: cell = "blocked", want todo
issues paging ................... TestFoldIssuesPaged: cell = "unknown", want blocked
page cap is a truncated read .... TestFoldIssuesCapUnknown: cell = "todo", want unknown
failed labels read -> unknown ... TestFoldIssuesFailUnknown: cell = "todo", want unknown
--apply refuses on unread ....... TestFoldIssuesFailUnknown: --apply exit 0, want 3
apply writes witnessed rows ..... TestFoldApplyWritesWitnessed: applied=[]
```

Implementer runs (the independent verifier re-runs all rows):

| # | Result | Runner |
|---|--------|--------|
| 1 | PASS: `suites-at-floor=3`, exit 0 | 2026-10-09 worker |
| 9 | PASS (live, read-only): `fold-wired`. Nothing unread; 9 briefs derive `blocked` from a linked issue's `help wanted`/`needs-decision` label; 0 derive `verified`/`done` (the flip owner refuses every recorded Evidence: 195 record no verdict, 70 end on a non-PASS verdict, 51 lack the strict marker, the rest lack `<login> @ <sha>` Date/Runner rows) | 2026-10-09 worker |
| 10 | PASS: `Totals: 18 caught, 0 NOT CAUGHT, 0 could-not-mutate.`, exit 0 (baseline green, control caught) | 2026-10-09 worker |

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
Reviewer questions: (1) find one input combination where the engine prints a negative
without `lookedAt=true` — if you can, the brief is not done; (2) is the precedence table
in `lifecycle.go` identical to spec §2?
