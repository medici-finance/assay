---
stream: fresh-views
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
board: generated
issues: []
spec: docs/streams/fresh-views/spec.md
---

# fresh-views Stream

**Status: proposed (parked pending approval).** This stream is scaffolded from a `draft`
scoping doc ([spec.md](spec.md)) and ships `status: parked` so it does not fake an approval it
does not have (`stream-source` lint: an *active* stream must cite an `approved` spec; a *parked*
stream may cite a `draft`). Activation — `status: parked → active` and the spec header
`draft → approved` — is a human ruling. Until then the briefs below are authored and tracked
but the stream is out of the active-stream cap and its work is not dispatchable.

Treat every **derived view** — a dispatch plan, a board snapshot, the open-PR set, a
mergeability verdict, a mirrored version string, a reconciled lifecycle cell, a landed sha — as
a **pure function of `main` at a known sha**. Stamp each derived read with the input sha it was
computed against; make a tool **refuse (not guess)** when its input sha is behind the head it is
acting on; and give shared append-only logs a single-writer or union-merge discipline so
concurrent writers cannot serial-conflict. The house rule "refresh, don't remember" exists to
compensate for this gap by hand; this stream makes it a property the code enforces. See
[spec.md](spec.md) for the seam, the evidence, and the design principle.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [input-sha stamp + refuse-when-stale helper for derived reads](brief-01-input-sha-stamp-refuse-when-stale-helper-for-derived-reads.md) | 0 | M | todo | — | — |
| 02 | [read-side loop views re-derive against live main: verifyloop plan + scanloop coalesce](brief-02-read-side-loop-views-re-derive-against-live-main-verifyloop-plan-scanloop-coalesce.md) | 1 | L | todo | — | — |
| 03 | [ready-flip gate re-verifies mergeability on main-advance](brief-03-ready-flip-gate-re-verifies-mergeability-on-main-advance.md) | 1 | M | todo | — | — |
| 04 | [shared append-only log discipline: verify-outcomes.jsonl merge=union + deskevidence post-write sha](brief-04-shared-append-only-log-discipline-verify-outcomes-jsonl-merge-union-deskevidence-post-write-sha.md) | 0 | M | todo | — | — |
| 05 | [mirror-freshness gate: fail the release/CI when a mirrored stamped value drifts from its source](brief-05-mirror-freshness-gate-fail-the-release-ci-when-a-mirrored-stamped-value-drifts-from-its-source.md) | 0 | M | todo | — | — |
| 06 | [reconcile ref-resolution across the brief-v2 id flag-day](brief-06-reconcile-ref-resolution-across-the-brief-v2-id-flag-day.md) | 0 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path

```
fresh-views/01 (input-sha stamp + refuse-when-stale helper)
        │
        ├──▶ fresh-views/02 (read-side loop views: verifyloop plan #334 + scanloop coalesce #228)   ← longest chain
        └──▶ fresh-views/03 (ready-flip mergeability re-check on main-advance #339)

independent (Wave 0, no dependency on the helper):
  fresh-views/04 (shared append-only log discipline: verify-outcomes.jsonl merge=union #882 + deskevidence post-write sha #806)
  fresh-views/05 (mirror-freshness gate for stamped values: brief-v1 header #1192 + codex.md #859)
  fresh-views/06 (reconcile ref-resolution across the brief-v2 id flag-day #1176)
```

**Smallest unblocking move:** build the shared `input-sha stamp + refuse-when-stale` helper
(brief 01). It has no upstream blocker — `tools/desk` already reads `origin/main` and computes
shas — and both read-side applications (02, 03) are the same call against it, so it is the true
head. The tempting-but-wrong first step is to patch `verifyloop` or the ready-flip gate directly
with an ad-hoc re-read: that ships the fix for one tool while the seam stays open in every other,
which is exactly how the eight cited issues accumulated. The Wave 0 briefs other than the helper do
NOT wait on it — they are distinct mechanism families (log concurrency, mirror freshness, ref
resolution) and can proceed in parallel from day one.

## Dependency waves

```
Wave 0: [01] [04] [05] [06]
Wave 1: [02]←01  [03]←01
```

Critical path: `01 → 02`.

## Conventions inherited

- Brief IDs are typed (`fresh-views/NN`); `depends:` / `unblocks:` carry typed IDs only.
- Every fix brief records a `freshness-checked <date> @ <sha>` line in `sources:` (rule 8): the
  cited issue was confirmed OPEN and the cited code confirmed present on `origin/main` before the
  brief was authored.
- The helper (01) is a shared surface; briefs 02 and 03 that consume it enumerate it under
  `consumers:` and carry a flow Verify row (rule 6).
