---
stream: multiroot-fixture
status: active
priority: P1
track: platform
serves: assay
owner: fixture
repo: medici-finance/multiroot-fixture
---

# Multiroot Fixture Stream

The single stream of the synthetic second root (`statusgen/testdata/stubroot/`).
It exists so multi-root statusgen can be exercised against a **populated** root
whose contents are unmistakably its own: the stream name `multiroot-fixture`
appears in no other root, so a board that shows it is provably the stub's board,
and a stub board that shows a root-1 stream is provably bleeding.

Three briefs at three different statuses, so the stub's board has a Next-up pick,
an Awaiting row, and a Done row without borrowing any of them from root 1.

## Briefs

| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Fixture brief at done — exercises the Done/Evidence path](./brief-01-fixture-done.md) | 0 | S | done | 2026-07-25 fixture-verifier | 2026-07-25 fixture-reviewer |
| 02 | [Fixture brief at implemented — exercises the Awaiting queue](./brief-02-fixture-implemented.md) | 0 | S | implemented | — | — |
| 03 | [Fixture brief at todo — exercises Next-up eligibility](./brief-03-fixture-todo.md) | 1 | S | todo | — | — |

## Critical path

```
01 done      02 implemented      03 todo  (independent — no depends between them)
```

Deliberately dependency-free. The fixture's job is per-root ISOLATION, not an
interesting graph, and a chain would gate 03 behind 02 — leaving the stub board's
Next-up section empty and so unable to tell "correctly filtered to this root" from
"this root silently dropped", which is the exact failure multi-root has to rule out.
