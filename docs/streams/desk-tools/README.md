---
stream: desk-tools
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
---

# desk-tools Stream

The planning board for the `tools/desk/` desk-tools suite — the desk binaries
(`deskboard`, `deskpost`, `deskpr`, `deskwt`, `verifyloop`, the loop engine, and the
`deskkit` internal library) that drive the process desks. The desk-tools source lives in
this repo (`tools/desk/`), so its planning lives here too, alongside the code it plans.

Briefs are self-contained `brief-NN-*.md` files; each carries its own scope, rules, task,
and an executable Verify table that runs in an assay checkout (`tools/desk/`, `statusgen/`).
This opening set re-homes the open desk-tools-source planning briefs onto the public board
where their code now lives: the `.assay-versions` binary-channel contract, the drain-engine
batch-fanout consumer, the published-tree residual-identity scrub, the deterministic verdict
runner, and the escape-valve `Decide()` primitive.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Binary channel — publish the `.assay-versions` contract, validate it, stamp desk-tools with its release tag](brief-01-binary-channel-and-pin-contract.md) | 1 | M | implemented | — | — |
| 02 | [Generalize — batch-fanout as the second drain-engine consumer](brief-02-generalize-batch-fanout.md) | 1 | M | implemented | — | — |
| 03 | [Published-tree residual-identity scrub — drive the cold-read to an independent CLEAN](brief-03-published-tree-residual-scrub.md) | 1 | M | implemented | — | — |
| 04 | [Deterministic runner — execute rows, batch, sign, file verdict issues](brief-04-runner-verdict-batching.md) | 1 | M | implemented | — | — |
| 05 | [Escape-valve `Decide()` primitive in deskkit](brief-05-escape-valve-decide.md) | 1 | M | verified | 2026-08-26 opus-4.8[1m]-verifier | — |

## Critical path
None. Each brief is independent and self-contained. The soft ordering their source streams
carried (a version-scheme brief ahead of 01, the drain engine ahead of 02, a set of
risk-path briefs ahead of 03, the verdict payload/row-classes ahead of 04) is satisfied by
work already landed outside this stream, so no typed `depends:` edge remains — see each
brief's Dependencies note.

## Dependency waves
- **Wave 1** — desk-tools/01, /02, /03, /04, /05 (all independent; parallelizable).

## Conventions
- Verify rows must be able to fail: exit status is the assertion, counts are captured and
  gated with `[ … ]`, and `\|` is never used as a regex alternation inside a GFM table cell
  (a basic grep reads the rendered bare pipe as a literal). Use `grep -e A -e B` for
  alternation.
- The desk-tools binaries are consumed as pinned release binaries via `.assay-versions`;
  desk-tools source changes are authored here under `tools/desk/`.
