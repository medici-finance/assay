---
brief: statusgen/10
title: statusgen graph export — derived-only DOT + JSONL from the existing parse tree, evaluated on real multi-hop questions
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-26 (re-authored clean for the statusgen board)
sources:
  - "A scoping note (§W5) proposing the derived graph export"
  - "A graph-engineering investigation note (2026-07-22) — the read that this stays derived-only, no store/DB"
  - "The findings-register data model (§3) — the schema this export must share entity IDs with; still spec-only as of the freshness check, so this brief conforms to the spec'd IDs rather than to code"
  - "freshness-checked 2026-08-17 — statusgen has exactly four modes (write/check/lint/record; statusgen/main.go run()); no graph emitter, no findings.json emitter"
exec-tier: strong
exec-tier-why: "the export is a third render of one parse tree and must share entity IDs/schema with STATUS.md and the spec'd findings.json; inventing parallel IDs is the defect the coordination note exists to prevent"
why: >-
  Briefs, findings, issues and PRs are already a typed graph statusgen parses every run, but
  multi-hop desk questions ("which briefs are downstream of this finding") are answered today
  by a model re-reading files — slow, and exactly the remembered-state failure class. A
  derived-only export (no new store, no graph DB) makes the latent graph queryable for the
  cost of a render, and the evaluation note tells us whether the multi-hop value is real
  before anyone proposes heavier machinery.
---

# Brief 10 — `statusgen --graph`: derived DOT + JSONL export

## Context
files: `statusgen/` (new `graph.go` + `graph_test.go`, `main.go` mode wiring),
`docs/research/graph-export-evaluation.md` (planned).
facts:
- Derived-only: emitted from the existing parse tree per run — no new store, no cache, no
  graph DB at current corpus size; revisit only if the evaluation proves multi-hop value.
- Nodes: streams, briefs, findings, intake entries, issues (as referenced). Edges: depends/
  unblocks, affects (finding→brief), sources (brief→register entry), issues. Entity IDs are
  the existing typed IDs (`stream/NN`, `F-<slug>`, `I-<slug>`) — the same IDs the findings
  data model assigns findings.json; do not invent a parallel scheme.
- Output: `--graph dot` (Graphviz digraph) and `--graph jsonl` (one node/edge object per
  line, deterministic order — same byte-determinism discipline as the register views).
  Read-only mode: never writes a file, never touches STATUS.md.
- Evaluation: answer 2–3 real multi-hop desk questions in the note; the required one is
  transitive finding-impact closure ("which briefs are downstream of this finding") answered
  on a known case from the findings register, with the expected answer derived by hand
  first, then compared to the export.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md or generated register views — `--graph` must not read or write them.
- Table-driven tests mirroring the existing statusgen test style; no new dependencies (DOT is
  a text format — no Graphviz library, and Verify must not require a local `dot` binary).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Implement `--graph dot|jsonl` in statusgen as a read-only mode over the existing parse
   tree, with the node/edge model and determinism above.
2. Tests: golden-ish table tests for both formats on a fixture tree (node/edge presence,
   deterministic ordering, typed-ID fidelity), plus one test asserting the finding→brief
   `affects` edge direction.
3. Write `docs/research/graph-export-evaluation.md` (planned): the 2–3 questions, the hand-derived
   expected answers, the export's answers, and a one-paragraph verdict on whether multi-hop
   value justifies any follow-up (default: no further machinery).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go run . --root .. --graph dot \| grep -m1 "digraph"` | exit 0 — DOT output opens a digraph |
| 2 | `cd statusgen && go run . --root .. --graph jsonl \| head -1 \| python3 -m json.tool > /dev/null` | exit 0 — first JSONL line is valid JSON |
| 3 | `cd statusgen && go test -run Graph -v . \| grep -c -e "--- PASS:"` | exit 0 with count ≥ 1 — graph tests exist and pass |
| 4 | `a=$(cd statusgen && go run . --root .. --graph jsonl \| sha256sum); b=$(cd statusgen && go run . --root .. --graph jsonl \| sha256sum); [ "$a" = "$b" ]` | exit 0 — byte-deterministic across runs |
| 5 | `grep -n "downstream" docs/research/graph-export-evaluation.md` | exit 0 — the finding-impact-closure question is answered in the note |
| 6 | `cd statusgen && go run . --root .. --lint` | exit 0 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

**Pre-existing implementation.** The deliverable this brief describes was already built and
merged to `main` before this dispatch: commit `b3244a1` ("statusgen: add --graph derived
export (dot + jsonl)", 2026-08-27), referencing this same work under its prior brief name
`landscape-followups/06` — one day after `9fa1cb8` re-authored it onto this board as
`statusgen/10` (2026-08-26). `statusgen/graph.go`, `statusgen/graph_test.go`, the `--graph`
wiring in `main.go`, and `docs/research/graph-export-evaluation.md` all already exist on
`main` with no gap against this brief's Context/Task. No source changes were needed or made;
this PR only runs the Verify table against current `main` and reconciles the board row.

Runner: worker (session `statusgen-10`), local `go` toolchain, 2026-08-29.

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `cd statusgen && go run . --root .. --graph dot \| grep -m1 "digraph"` | 0 | `digraph assay {` |
| 2 | `cd statusgen && go run . --root .. --graph jsonl \| head -1 \| python3 -m json.tool > /dev/null` | 0 | first JSONL line parses as valid JSON |
| 3 | `cd statusgen && go test -run Graph -v . \| grep -c -e "--- PASS:"` | 0 | `9` (9 graph subtests pass) |
| 4 | two successive `--graph jsonl` runs, sha256-compared | 0 | hashes match — byte-deterministic |
| 5 | `grep -n "downstream" docs/research/graph-export-evaluation.md` | 0 | matches at lines 24 and 33; the required finding-impact-closure question (Q1) is answered in the note |
| 6 | `cd statusgen && go run . --root .. --lint` | 0 | `LINT: PASS` (pre-existing NOTICEs unrelated to this brief) |

### Non-implementer verifier run — VERIFY: PASS — 2026-09-01 opus-4.8[1m]-verifier (verify-desk dispatch), merged main `5927efe`

Runner ≠ implementer (the rows above are the implementer's; this is the independent non-implementer re-run). Own temp worktree off `origin/main`, offline (`KUBECONFIG=/dev/null`); rows run from inside `statusgen/`. Deliverables present: `statusgen/graph.go`, `statusgen/graph_test.go`, `--graph` wiring in `statusgen/main.go`, `docs/research/graph-export-evaluation.md`.

| # | Command | Exit | Result |
|---|---------|------|--------|
| 1 | `go run . --root .. --graph dot \| grep -m1 "digraph"` | 0 | `digraph assay {` |
| 2 | `go run . --root .. --graph jsonl \| head -1 \| python3 -m json.tool >/dev/null` | 0 | first JSONL line parses as valid JSON |
| 3 | `go test -run Graph -v . \| grep -c -- '--- PASS:'` | 0 | 9 (9 graph subtests pass) |
| 4 | two successive `--graph jsonl` runs, sha256-compared | 0 | identical hash `15ccbf6d…f6ba` — byte-deterministic |
| 5 | `grep -n "downstream" docs/research/graph-export-evaluation.md` | 0 | matches L24, L33 — the required finding-impact-closure question (Q1) is answered |
| 6 | `go run . --root .. --lint` | 0 | `LINT: PASS` (only pre-existing NOTICEs) |

`RISK-VALUE: N/A` — read-only export mode; `statusgen/graph.go` writes only to `os.Stdout` (no `os.Create`/`WriteFile`), and all conditionals are structural (empty-string checks @ `statusgen/graph.go:93,110,291`, sort comparators @ `:135,148,151`), not risk-bearing guard constants.

**VERIFY: PASS** — all six Verify rows checked-clean by a non-implementer; my run reproduces the implementer's Evidence exactly. Advancing `implemented → verified`.

## Review
Gate: model. Reviewer records verdict + date in the stream README table.
Reviewer re-derives the finding-impact-closure answer by hand on the chosen case and compares
it to the export's answer — the evaluation is the deliverable that decides whether this line
of work continues, so it must not be graded by its own author.
