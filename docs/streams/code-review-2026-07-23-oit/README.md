---
stream: code-review-2026-07-23-oit
status: active
priority: P1
track: platform
serves: assay
tiering: implement=cheap verify=strong
---

# Code-review remediation — 2026-07-23 (oit desk-tooling brief, relocated)

This single-brief stream holds **one** brief moved out of `oit`'s
`../oit/docs/streams/code-review-2026-07-23/brief-08-desk-board-loop-post.md` by
`oit:assay-selfcontain/03` ("Move methodology/tool streams → assay-toolkit").
The brief fixes four oit-native desk-tooling bugs (deskboard pagination, ANSI/injection in the
ACTIONABLE lane, loopengine non-atomic reclaim, deskpost order-insensitive security-pass gating)
whose implementation targets `tools/desk/` **in this repo** — that's why it moved here rather than
staying with the rest of `code-review-2026-07-23`, which is oit product-review work and stays in
oit. It lands in its **own** directory, `code-review-2026-07-23-oit/`, deliberately separate from
this repo's pre-existing `code-review-2026-07-23-assay-toolkit/` stream (a different review — the statusgen
anti-falsification / darsync-CI-gate cluster) so the two are never conflated.

**Alias**: this brief is referenced elsewhere (oit's own README/CLAUDE.md, cross-stream links) as
`code-review-2026-07-23/08`. That ID and this stream's `code-review-2026-07-23-oit/brief-08-*`
name the same brief. The alias is **documentary only** — statusgen derives a brief's ID from its
path and has no alias mechanism, so the brief's `brief:` frontmatter is the `-oit` form and the
old ID resolves nowhere in tooling. Old references should repoint to
`assay-toolkit:code-review-2026-07-23-oit/08` (or the full path
`../assay-toolkit/docs/streams/code-review-2026-07-23-oit/brief-08-desk-board-loop-post.md`)
going forward.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 08 | [desk board/loop/post robustness (T2, T4, T5, T6)](./brief-08-desk-board-loop-post.md) | 0 | M | implemented | — | — |

## Gate

`gate: model` — revertible tooling changes, no risk-boolean surface (all `risk:` no in-brief).

## Definition of Done (stream conventions)

- The fix lands in **this repo's `tools/desk/`**. The Verify row in-brief is runnable by a
  non-implementer against this checkout.
- Stop at `implemented`; a non-implementer runs Verify and fills Evidence before `verified`/`done`.
