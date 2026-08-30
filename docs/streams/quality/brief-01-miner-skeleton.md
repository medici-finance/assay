---
brief: quality/01
title: qualgen miner skeleton — go-git extraction, incremental mine, three-state plumbing
why: >-
  A flow board measures delivery (throughput, lead time) but says nothing about whether
  the code being shipped is durable or brittle. This brief builds the foundation of the
  tool that answers that: a repo-agnostic miner that reads a git repository's full
  history into diffable, auditable artifacts. Every later "are we getting better / where
  is it brittle / how do we compare" number reads from these artifacts, so nothing in the
  stream can be computed until this skeleton exists.
wave: 0
depends: []
unblocks: ["quality/02", "quality/03", "quality/04", "quality/06", "quality/16"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-24 by quality-stream authoring session
sources:
  - "docs/streams/quality/spec.md §3 — architecture (single repo-agnostic Go binary, go-git, committed-artifact model, mine/report/pr/check modes, pinned-release distribution)"
  - "docs/streams/quality/spec.md §3.1 — Profile-B no-in-repo-writes + tracking-root rule, incremental extend-never-replace, mine records its horizon"
  - "docs/streams/quality/spec.md §3.2 — three-state instrument invariant (measured / measured-zero / could-not-measure)"
  - "docs/streams/quality/spec.md §4 — M1 aggregation grains this schema must carry (per commit / file / package / PR / stream / author-identity / window)"
exec-tier: strong
exec-tier-why: >-
  (a) the internal commit/diff record schema, and the go-git-vs-shelled-git blame
  boundary, are design decisions the spec deliberately leaves to this brief and the facts
  do not fully pre-specify.
---

# Brief 01 — qualgen miner skeleton: go-git extraction, incremental mine, three-state plumbing

## Context

files:
- NEW `qualgen/go.mod` (planned) — a new Go module, statusgen-sibling. Declares `go-git`
  as its history-access dependency (no cgo, no libgit2). This is the module every later
  `quality/*` brief extends.
- NEW `qualgen/main.go` (planned) — CLI entry. Dispatches the four modes `mine`,
  `report`, `pr <n>`, `check <paths>`. In this brief only `mine` has behavior; the other
  three are recognized, parse their flags, and emit a `not yet implemented` NOTICE
  (mode scaffolding, no behavior).
- NEW `qualgen/measure.go` (planned) (+ `measure_test.go`) (planned) — the first-class three-state
  output type (the interface contract, below). Every metric value the tool ever emits is
  wrapped in this type.
- NEW `qualgen/commit.go` (planned) (+ `commit_test.go`) (planned) — the internal `Commit` record
  type (the commit table).
- NEW `qualgen/diff.go` (planned) (+ `diff_test.go`) (planned) — the internal `FileDiff` / `Hunk` /
  `LineChange` record types (the diff table).
- NEW `qualgen/store.go` (planned) (+ `store_test.go`) (planned) — the single-writer artifact store:
  append-only JSONL writer + reader over the tracking root, plus the mine header/manifest.
- NEW `qualgen/mine.go` (planned) (+ `mine_test.go`) (planned) — the `mine` mode: full + incremental
  extraction, horizon recording.
- NEW `qualgen/report.go` (planned), `qualgen/pr.go` (planned), `qualgen/check.go` (planned) — mode stubs that
  parse flags and NOTICE `not yet implemented`. Filled by later briefs.
- NEW `qualgen/testdata/` (planned) — fixtures built by the test harness (see Task step 6).

facts:
- module: a NEW `qualgen/` Go module (Go generics required for the three-state type; target
  a modern Go toolchain). Do NOT add qualgen code to any existing module.
- library: `go-git` for history access. Shelling to the `git` binary is permitted ONLY where
  go-git is materially slower (blame at scale), decided by benchmark and recorded in this
  brief's PR body; `mine` in this brief needs no blame, so it is pure go-git.
- distribution: pinned release binary via the same version-pin mechanism as statusgen
  (`.assay-versions`). No new pipeline is designed here; the module just builds a binary.
- write rule (spec §3.1): NO in-repo writes to the mined target. All artifacts land under an
  operator-chosen tracking root `--out <dir>`, defaulting to the mined repo only when the
  operator explicitly opts in. Every mode is read-only against the target repo.
- artifact paths (relative to the tracking root, spec §9.4): `docs/quality/mine.json` (the
  header/manifest), `docs/quality/commits.jsonl` (commit table, append-only),
  `docs/quality/diffs.jsonl` (diff table, append-only). A local uncommitted cache MAY hold
  expensive intermediates; none are needed in this brief.
- three-state (spec §3.2): `measured`, `measured-zero`, `could-not-measure` are three
  DISTINCT output states from day one — a failed read, an unreadable blob, a squash-hidden
  parent are `could-not-measure` with a reason, never a silent zero, never conflated with a
  genuine `measured-zero`.

single-point-of-failure: none — this brief is a read-only history miner writing only to an
operator-chosen tracking root; it touches no funds, auth, or ledger surface, so the
core-system defense-in-depth obligation does not apply. The one correctness-critical
invariant (extend-never-replace on incremental runs) is covered by a dedicated Verify row.

## Interface contract (the seam quality/02 and quality/03 consume)

This brief OWNS and freezes these four things; every wave-1 brief depends on them and must
not redefine them:

1. `Measure[T any]` (measure.go): a generic three-state wrapper — `State` is one of
   `Measured` / `MeasuredZero` / `CouldNotMeasure`; `Value T` is meaningful only when
   `Measured`; `Reason string` is required (non-empty) when `CouldNotMeasure`. It
   JSON-marshals to `{"state":"measured|measured-zero|could-not-measure","value":...,"reason":...}`.
   Every number any later brief emits is a `Measure[...]`.
2. `Commit` (commit.go): the commit record — SHA, parent SHAs, author identity (raw
   author line, to be classified by later briefs), committer, timestamps, message, and the
   list of `FileDiff` refs. Marshals one-per-line into `commits.jsonl`.
3. `FileDiff` / `Hunk` / `LineChange` (diff.go): the per-`(commit, file)` diff record —
   path (old/new), change kind, and the ordered `LineChange` list. `LineChange` carries the
   raw op (add/del/context) and a slot for later classification; a blob that cannot be
   line-diffed (binary/unreadable) sets the record's line data to `CouldNotMeasure`.
   Marshals one-per-line into `diffs.jsonl`.
4. `Store` (store.go): the single-writer artifact store — `Append(kind, record)` (append-only
   JSONL, never rewrites prior lines), a reader that streams `commits.jsonl` / `diffs.jsonl`
   back as typed records for aggregation, and the `mine.json` header read/write. quality/02
   and quality/03 READ via this Store and APPEND their aggregates through it; they never
   parse the JSONL by hand.

quality/02 (taxonomy + churn) and quality/03 (hotspots + coupling) both consume 1–4, read
the commit/diff tables via the Store, and append their own aggregates (per
file / package / PR / stream / author-identity / window) as further JSONL through the same
Store — each aggregate value a `Measure[...]`.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch
  + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit a generated view or a `STATUS.md`-class artifact on a branch (single writer =
  main's CI). This brief writes artifacts only under a temp/test tracking root.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Create the `qualgen/` module (`go.mod`) depending on `go-git`. No other runtime deps.
2. Implement `measure.go`: the `Measure[T]` three-state type and its JSON marshal/unmarshal
   per the interface contract. Enforce the invariants (value meaningful only when measured;
   reason required when could-not-measure). This is the FIRST file — everything wraps it.
3. Implement `commit.go` and `diff.go`: the `Commit` and `FileDiff`/`Hunk`/`LineChange`
   record types per the interface contract, with JSONL marshalling. A binary/unreadable blob
   yields a `FileDiff` whose line data is `CouldNotMeasure` with a reason, distinct from a
   text file that changed by zero lines (`MeasuredZero`).
4. Implement `store.go`: the single-writer `Store` — append-only JSONL writer (must not
   rewrite existing lines), typed streaming reader, and `mine.json` header read/write.
   The header records: mined-at, tip SHA, horizon (earliest reachable commit), detected
   discontinuities (shallow-clone floor, rewritten/rename gaps), and per-state coverage
   counts.
5. Implement `mine.go`: walk the target repo with go-git. Full history by default; on a
   repeat run, read the prior `mine.json` and extract ONLY commits reachable that postdate
   the recorded tip — appending, never rewriting (extend-never-replace) — and advance the
   tip/horizon. Every extracted field flows through `Measure[T]`. Wire `mine` into `main.go`
   with `--repo` and `--out` flags.
6. Add the mode scaffolding: `report.go`, `pr.go`, `check.go` recognized by `main.go`,
   parsing flags and emitting a `not yet implemented` NOTICE (exit 0). Add a test harness in
   `qualgen/testdata` (planned)/tests that stands up fixture repos from the stdlib + local `git` binary
   (a repo with a normal text edit, a zero-change re-commit, and a binary blob) for the
   Verify rows below.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd qualgen && go test ./... -run TestMeasureThreeStateRoundTrip` | exit 0; `measured` / `measured-zero` / `could-not-measure` each round-trip through JSONL as a distinct `state` string, and `could-not-measure` requires a non-empty `reason` |
| 3 | `cd qualgen && go test ./... -run TestMineExtractsAllCommits` | exit 0; the mined commit-record count equals `git rev-list --count HEAD` on the fixture — DEREFERENCES extraction against real history |
| 4 | `cd qualgen && go test ./... -run TestMineIncrementalExtends` | exit 0; a second mine over added commits appends records and advances the recorded tip/horizon, and re-mining with no new commits is a no-op (extend-never-replace: prior lines byte-identical) |
| 5 | `cd qualgen && go test ./... -run TestDiffThreeStateDistinguishesZeroFromUnmeasured` | exit 0; a text file changed by zero lines emits `measured-zero`, a binary/unreadable blob emits `could-not-measure` with a reason — the two are never conflated |
| 6 | `TMP=$(mktemp -d); cd qualgen && go run . mine --repo .. --out "$TMP" && grep -Eq "\"tip_sha\": *\"$(git rev-parse HEAD)\"" "$TMP/docs/quality/mine.json" && test -f "$TMP/docs/quality/commits.jsonl"` | exit 0; mine writes header + commit table under the tracking root, and the header's recorded `tip_sha` equals the repo's actual HEAD — DEREFERENCES a real output value, not just presence |
| 7 | `cd qualgen && go run . report --out /tmp/x; go run . pr 1 --out /tmp/x; go run . check qualgen/main.go --out /tmp/x; echo rc=$?` | each mode is recognized, parses flags, prints a `not yet implemented` NOTICE, and exits 0 (mode scaffolding wired, no behavior) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). "verified" in the stream
     README requires this section filled by someone who did NOT implement. -->

### Non-implementer verifier run — VERIFY: PASS — 2026-08-26 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `ea7fea5`
Runner != implementer. Own isolated worktree off `origin/main`, OFFLINE (`KUBECONFIG=/dev/null`). gate: model, all risk no. `qualgen/` module.

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|-----------|------|--------|
| 1 | `cd qualgen && go build ./... && go vet ./...` | exit 0 | exit 0 — clean build + vet | 2026-08-26 | opus-4.8[1m]-verifier |
| 2 | `go test ./... -run TestMeasureThreeStateRoundTrip` | 3 states distinct; could-not-measure needs reason | exit 0 — 7 subtests PASS (measured/measured-zero/could-not-measure distinct; empty-reason rejected) | 2026-08-26 | opus-4.8[1m]-verifier |
| 3 | `go test ./... -run TestMineExtractsAllCommits` | count == git rev-list | exit 0 — PASS | 2026-08-26 | opus-4.8[1m]-verifier |
| 4 | `go test ./... -run TestMineIncrementalExtends` | extend-never-replace | exit 0 — PASS | 2026-08-26 | opus-4.8[1m]-verifier |
| 5 | `go test ./... -run TestDiffThreeStateDistinguishesZeroFromUnmeasured` | exit 0 | exit 0 — PASS | 2026-08-26 | opus-4.8[1m]-verifier |
| 6 | `go run . mine --repo .. --out $TMP` (tip_sha==HEAD; commits.jsonl) | exit 0 | exit 1 IN THE LINKED WORKTREE only (go-git PlainOpen cannot resolve a linked worktree per-worktree HEAD); substance reproduced vs a normal repo of the shape CI checks out: exit 0, tip_sha==HEAD, commits.jsonl written — environment artifact, not a regression | 2026-08-26 | opus-4.8[1m]-verifier |
| 7 | `go run . report/pr/check --out /tmp/x` | each recognized; 'not yet implemented' NOTICE; exit 0 | exit 0 — all three emit the mode-scaffolding NOTICE | 2026-08-26 | opus-4.8[1m]-verifier |

**RISK-VALUE: N/A-equivalent** — enumeration over the qualgen diff found only reversible operational literals (dir/file perms 0o755/0o644, JSONL buffer sizes, short-SHA length 12) and structural identifiers (schema tag, three-state enum strings); no irreversible/authority/financial constant. The only write is to an operator-chosen `--out` root, read-only against the mined target.

Routed edge (not a Verify FAIL): `qualgen mine` cannot mine a target that is itself a linked git worktree (go-git PlainOpen limitation) — harmless to CI (normal checkouts) but a real operator edge; a later brief may want DetectDotGit handling.

## Review
Gate: model (all four risk answers no — a read-only, repo-agnostic history miner writing
only to an operator-chosen tracking root; no funds/auth/ledger surface, no in-repo writes to
the mined target). exec-tier: strong — the internal record schema and the go-git/shell blame
boundary are unspecified design decisions. Reviewer records verdict + date in the stream
README table.
