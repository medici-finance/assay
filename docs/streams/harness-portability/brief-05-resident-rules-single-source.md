---
brief: harness-portability/05
title: Resident rules — one source, per-harness delivery generated
why: >-
  The SessionStart hook is how the method arrives in every Claude session; a harness
  with no equivalent simply never receives the resident rules — the method does not
  arrive. Codex's native channel is AGENTS.md. Two hand-maintained copies of the ten
  rules is the drift failure again, so the rules get ONE source and every per-harness
  delivery artifact (Claude hook payload, Codex AGENTS.md fragment) is generated from
  it and byte-compared in CI — divergence becomes impossible, not detected.
wave: 2
depends: ["harness-portability/01", "harness-portability/03"]
unblocks: ["harness-portability/06", "harness-portability/07"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-07 by harness-portability authoring session
sources: ["authoring dispatch (Ian, 2026-08-07): the SessionStart hook is the load-bearing delivery mechanism, not a convenience", "the inject-resident-rules.sh hook + hooks.json: SessionStart is the ONLY hook Assay ships; the ten rules live as a heredoc inside the shell script", "an open finding: shared guardrails restated 4-5 times across desk/loop skills — the single-source cut this brief makes is the same medicine", "harness-portability/01's resident-rules-channel matrix row (which AGENTS.md paths Codex reads, composition, size limits)", "the harness-target ruling (HP/03): ruled target set — which fragments to generate", "the maintainer ruling 2026-08-03 (AGENTS.md is repo-local; adopters get the method through the bundle): the fragment ships IN the bundle for adopters, and never edits this repo's root AGENTS.md", "freshness-checked 2026-08-07 (the rules exist nowhere outside the heredoc)"]
consumers: ["plugins/assay/hooks/inject-resident-rules.sh: fixed-here (becomes a thin emitter of the generated payload; hooks.json untouched)", "Claude sessions of every adopter + this house: fixed-here (byte-identical payload is the acceptance bar — regression row below)", "adopt skill / docs/adopting-assay.md (Codex fragment install step): follow-up harness-portability/06 and harness-portability/07", "root AGENTS.md (this repo): out-of-scope (repo-local by ruling; never a generation target)"]
exec-tier: strong
exec-tier-why: >-
  (c): safety plumbing — the rules ARE the guardrails, and a generation bug that drops
  or reorders a rule ships a weakened method to every session while every simple test
  still passes; byte-equality discipline and the fixture design need care.
---

# Brief 05 — Resident rules: single source, generated delivery

## Context

files:
- **create** `plugins/assay/resident-rules.md` (planned) — the single source (ten rules, versioned)
- **create** `tools/harnessgen` — Go module, first verb: `resident` (emit + `--check`)
- **create** `plugins/assay/codex/AGENTS-assay.md` (planned) — generated Codex fragment (path
  contingent on 03's ruling; adjust with the ruling, not silently)
- **amend** the inject-resident-rules.sh hook — thin emitter of generated content (same
  JSON `systemMessage` contract, byte-identical rules text)
- **amend** the CI workflows — regenerate-and-diff CI check; `go.work`
- **amend** the `plugins/assay` SOURCES coverage roster — entries for the new artifacts

facts:
- Current state: the ten rules live ONLY as a heredoc inside the inject-resident-rules.sh
  hook, JSON-encoded via `jq -Rs '{systemMessage: .}'`. That script/contract stays —
  Claude Code's SessionStart hook expects exactly it.
- **Generation contract**: `harnessgen resident` reads the resident-rules source and emits
  (a) the Claude payload text the hook script carries, (b) the Codex AGENTS.md
  fragment framed for AGENTS.md composition (per 01's `resident-rules-channel` row: the
  fragment must survive being one section among an adopter's existing AGENTS.md
  content). `--check` regenerates to a temp dir and diffs against the committed
  artifacts: any difference → non-zero listing files. CI runs `--check` (the STATUS.md
  single-writer pattern: committed derived artifacts, CI proves they match the source).
- **The Claude payload must survive the refactor byte-identical** in its rules text on
  the first landing (rule-content changes are their own PRs, never smuggled into
  plumbing changes).
- The Codex fragment's install destination (adopter repo AGENTS.md vs `~/.codex/AGENTS.md`)
  follows 01's measurement + 03's ruling; the fragment itself is destination-agnostic
  text. Installing it is the adopt flow's job (06/07), not this brief's.
- Rule 6 of the resident rules (model-tier awareness) references Claude-shaped probes;
  the neutralization of rule TEXT follows brief 04's vocabulary where a harness name is
  load-bearing — coordinate: 04 owns skill bodies, this brief owns the rules text, the
  vocabulary is shared.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the
  task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Never edit this repo's root `AGENTS.md` — it is not a generation target.
- No rule-content edits in this PR beyond mechanical neutralization noted hunk-by-hunk
  in the PR body; content changes are separate PRs.

## Task

1. Extract the heredoc into `plugins/assay/resident-rules.md` (planned) (structured: one `## R<N>`
   section per rule, so per-rule presence is checkable).
2. Build `tools/harnessgen` verb `resident` (+ `--check`), table-driven tests: a
   modified committed artifact → `--check` red naming the file; a source rule deleted →
   both artifacts change; empty/unparseable source → `could-not-check`, non-zero.
3. Generate both artifacts; convert the hook script to carry the generated payload
   (byte-identical rules text; prove via row 3).
4. Wire the CI `--check` on paths `plugins/assay/**` + `tools/harnessgen/**`.
5. Update the SOURCES coverage roster; record in PARITY.md that the rules' home moved.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./... > /tmp/hp05r1.out 2>&1; echo $?` | `0` |
| 2 | `go run ./tools/harnessgen resident --check; echo $?` | `0` — committed artifacts match the source |
| 2a | **Mutation — the check can fail**: `printf '\nDRIFT-PROBE\n' >> plugins/assay/codex/AGENTS-assay.md && go run ./tools/harnessgen resident --check > /tmp/hp05r2a.out 2>&1; echo $?; git checkout -- plugins/assay/codex/AGENTS-assay.md` | non-zero, output names `AGENTS-assay.md`; after the checkout, row 2 passes again — the red was the plant |
| 3 | Hook contract intact: `bash plugins/assay/hooks/inject-resident-rules.sh \| jq -er '.systemMessage' > /tmp/hp05r3.out; echo $?; grep -c '^[0-9]\+\.' /tmp/hp05r3.out` | exit `0`; rule count `10` — valid JSON systemMessage carrying all ten numbered rules |
| 3a | Payload equals source: `for n in 1 2 3 4 5 6 7 8 9 10; do line="$(awk "/^## R$n /{f=1;next} /^## R/{f=0} f" plugins/assay/resident-rules.md 2>/dev/null \| head -1)"; if [ -z "$line" ]; then echo "MISSING R$n (source line empty/unreadable)"; elif ! grep -qF "$line" /tmp/hp05r3.out; then echo "MISSING R$n"; fi; done > /tmp/hp05r3a.out; test ! -s /tmp/hp05r3a.out; echo $?` | `0` — each source rule's first line appears in the emitted payload. **Guarded against a vacuous pass**: an unguarded `grep -qF "$(awk ...)"` silently exits `0` when the source is missing or unparseable — `awk`'s fatal error goes to stderr, the `$(...)` substitution captures an empty string, and `grep -qF ""` trivially matches every line, so all ten MISSING checks are skipped and the row reports a false pass. Reproduced pre-implementation: with the source absent, the unguarded form exits `0`; this guarded form correctly reports all ten `MISSING ... (source line empty/unreadable)` and exits `1` (control: append `R99` probe to the loop → MISSING prints) |
| 4 | Fragment framing: `test -f plugins/assay/codex/AGENTS-assay.md && grep -c '^## ' plugins/assay/codex/AGENTS-assay.md` | `>= 1` and file exists — the fragment is section-framed for AGENTS.md composition (exact heading shape per 01's row; update Expect with the ruling if 03 moves the path, in the same commit) |
| 5 | CI wiring: `grep -rlE 'harnessgen' .github/workflows > /tmp/hp05r5.out; test -s /tmp/hp05r5.out; echo $?` | `0` |
| 6 | **Neighbour row** — `bash plugins/assay/hooks/inject-resident-rules.sh \| jq -e 'keys == ["systemMessage"]'; echo $?` | `0` — the hook's JSON contract (exactly one key) is unchanged; the SessionStart consumer sees the same shape it always did |

## Evidence

<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Row 2a's mutation output pasted, not summarised.
     "verified" requires a non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./...` | 0 | `ok github.com/medici-finance/assay/tools/harnessgen` — table-driven suite: modified-artifact drift names the file, a source rule change drifts BOTH artifacts, empty/unparseable/missing/nine-rule source is could-not-check, and the Claude payload is byte-identical to the pre-refactor heredoc golden | 2026-08-16 | HP/05 implementation session |
| 2 | `go run ./tools/harnessgen resident --check` | 0 | `harnessgen resident --check: clean — committed artifacts match the source` | 2026-08-16 | HP/05 implementation session |
| 2a | mutation: append `DRIFT-PROBE` to the Codex fragment then `resident --check`; restore | 1 | `harnessgen resident --check: DRIFT — committed artifacts differ from the source; run go run ./tools/harnessgen resident and commit:` naming `AGENTS-assay.md`; after regenerating, row 2 returns 0 | 2026-08-16 | HP/05 implementation session |
| 3 | `bash plugins/assay/hooks/inject-resident-rules.sh \| jq -er '.systemMessage'` then `grep -c '^[0-9]\+\.'` | 0 / 10 | valid JSON systemMessage; ten numbered rules present | 2026-08-16 | HP/05 implementation session |
| 3a | guarded per-rule presence loop (each source rule's first line appears in the emitted payload) | 0 | `/tmp/hp05r3a.out` empty — no MISSING lines | 2026-08-16 | HP/05 implementation session |
| 4 | `test -f plugins/assay/codex/AGENTS-assay.md && grep -c '^## '` | 0 / 1 | fragment exists and is section-framed (`## Assay resident operating rules`) for AGENTS.md composition | 2026-08-16 | HP/05 implementation session |
| 5 | `grep -rlE 'harnessgen' .github/workflows` | 0 | the tools workflow — matrix leg `test (tools/harnessgen)` plus an explicit `resident --check` step | 2026-08-16 | HP/05 implementation session |
| 6 | `bash plugins/assay/hooks/inject-resident-rules.sh \| jq -e 'keys == ["systemMessage"]'` | 0 | `true` — the hook's JSON contract is exactly one key, unchanged | 2026-08-16 | HP/05 implementation session |

Evidence rows are implementer-run (this session authored the change); `verified`
requires a non-implementer per the stream README and this brief's gate.

### Non-implementer verifier run — VERIFY: PASS (all 7 rows, no UNRUN) — opus-4.8[1m]-verifier (verify-desk dispatch), independent re-run against merged main, 2026-08-22

Runner ≠ implementer. Isolated worktree; all deliverables in-repo (`tools/harnessgen` + `plugins/assay/` — no dehouse applies). Every row re-executed fresh; implementer Evidence not trusted. Risk line `{regulatory: no, customer: no, irreversible: no, sensitive-data: no}`, `gate: model`.

| # | Command | Exit | Observed | Date | Runner |
|---|---------|------|----------|------|--------|
| 1 | `cd tools/harnessgen && GOFLAGS=-buildvcs=false go test ./...` | 0 | `ok …/tools/harnessgen 0.252s` | 2026-08-22 | opus-4.8[1m]-verifier |
| 2 | `go run ./tools/harnessgen resident --check` | 0 | `clean — committed artifacts match the source` | 2026-08-22 | opus-4.8[1m]-verifier |
| 2a | mutation — append `DRIFT-PROBE` to the Codex fragment, `resident --check`, restore | 1 | `DRIFT — committed artifacts differ …` naming `AGENTS-assay.md`; restored clean; row 2 re-run → 0 | 2026-08-22 | opus-4.8[1m]-verifier |
| 3 | `inject-resident-rules.sh \| jq -er '.systemMessage'`; `grep -c '^[0-9]\+\.'` | 0 | rule count `10` | 2026-08-22 | opus-4.8[1m]-verifier |
| 3a | guarded per-rule presence loop (R1..R10) | 0 | empty — no MISSING lines | 2026-08-22 | opus-4.8[1m]-verifier |
| 4 | `test -f plugins/assay/codex/AGENTS-assay.md && grep -c '^## '` | 0 | `1` — heading `## Assay resident operating rules` | 2026-08-22 | opus-4.8[1m]-verifier |
| 5 | `grep -rlE 'harnessgen' .github/workflows` | 0 | the tools workflow — matrix leg + gated `resident --check` step | 2026-08-22 | opus-4.8[1m]-verifier |
| 6 | `inject-resident-rules.sh \| jq -e 'keys == ["systemMessage"]'` | 0 | `true` — JSON contract exactly one key | 2026-08-22 | opus-4.8[1m]-verifier |

**RISK-VALUE: DERIVED — the ten resident rules' delivered payload is byte-identical (sha256 chain) across the pre-refactor git heredoc, the golden fixture under the generator's testdata, the committed Claude payload text, and the source-generated output** — no rule dropped, reordered, or altered; the refactor changed delivery, not the method text. Proven by sha256 chain, not by a passing test. All other literals (rule-count bound `10`, single-key JSON contract, version string) are reversible contract/framing shapes caught by rows 3/4/6.

**VERIFY: PASS** — all 7 rows run with real observed output matching Expect (incl. inverted mutation row 2a and guarded row 3a); no UNRUN / COULD-NOT-CHECK rows.

## Review

Gate: **model** (from frontmatter). Review priority: the rules-text diff between the
old heredoc and `resident-rules.md` — it should be mechanical extraction; any semantic
delta not flagged hunk-by-hunk in the PR body is the smuggled-content failure this
brief's ground rules forbid.
