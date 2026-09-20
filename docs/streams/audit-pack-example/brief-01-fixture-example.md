---
brief: assay:assay:audit-pack-example:01
title: Fixture brief for the sdlc/08 audit-pack worked example
why: >-
  `statusgen --export-audit-pack --release <tag>` (sdlc/08) needs a small, self-contained,
  always-on-disk tree to demonstrate the release-keyed pack format without running a real
  release. This brief IS that tree's one real brief file: it cites a fixture requirement so
  the pack has a resolvable requirement->brief chain to report, alongside the deliberately
  unresolvable one in REQ-apfixture-unresolved.
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-01-01 by worker-desk (sdlc/08 fixture, never dispatched)
sources:
  - "sdlc/brief-08-audit-pack-export.md — task item 4: 'a worked
    example committed as a fixture'"
  - "docs/streams/requirements/apfixture-ok.md, apfixture-unresolved.md — the two fixture
    requirements this stream's pack scope names"
  - "docs/release-notes/v0.0.0-fixture.md — the release record naming this brief in scope"
exec-tier: any
domain: clear
version: 1
id: 02a02706-356b-4bfe-ac06-b838f1949ef6
satisfies: ["REQ-apfixture-ok"]
---

# Brief 01 — audit-pack fixture example

## Context

files: none — this brief IS the fixture; nothing it names should ever be implemented.

facts:
- This brief is never dispatched. Its stream is `status: parked` and its Board row is meant
  to stay `todo` forever — `statusgen --export-audit-pack`'s worked example needs a backing
  brief whose chain resolves (a real board row, a real `satisfies:`) without needing a real
  `done` status, real Verified/Reviewed cells, or a real PR, none of which a fixture may
  fabricate (attribution.go's independent-Evidence-row and self-verification checks exist
  precisely to catch a fabricated done/verified row, and this brief must not be the thing
  that trips them for the wrong reason).
- `satisfies: ["REQ-apfixture-ok"]` is the one real thing this file does: it gives
  `docs/streams/requirements/apfixture-ok.md` a citing brief, so the audit-pack worked example
  has a requirement whose backing-brief chain actually resolves, alongside
  `REQ-apfixture-unresolved`'s deliberately-dangling `satisfied-by`.

## Ground rules
- This brief is never implemented, never verified, never done. Do not dispatch it.

## Task

None. This file exists to be cited by `satisfies:`, not to be worked.

## Verify

The pack format this brief demonstrates is verified by sdlc/08's own Verify table
(the sdlc/08 brief in the methodology tracker). This brief's own row is
a structural placeholder only — it is never dispatched, so there is nothing behavioral to
assert here.

| # | Command | Expect |
|---|---------|--------|
| 1 | `true` | exit 0 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). This brief is never reviewed for
real; it stays `todo` on a `parked` stream.
