### Added
- `statusgen --lint` gains `witnessAbsenceGateChecks`: a `verified`/`done` closure THIS
  branch makes with no execution witness (`statusgen verifyrun`) for one or more Verify
  rows is now a hard PROBLEM, merge-base scoped exactly like the existing contradiction
  and UNRUN gates — a pre-existing closure at the merge-base stays the per-stream NOTICE
  it already got.
- `hasVerifyPass` now matches a ratified bold `**VERIFY: (PASS|FAIL)**` regex instead of a
  fixed substring, so a real verifier line carrying prose before its closing `**`
  (`**VERIFY: PASS (4/4 offline-runnable rows)**`) is recognised; `BLOCKED` still never
  matches.
- A `**VERIFY: PASS**` marker is no longer a flip signal on its own when the same Evidence
  entry also reads `HELD` or `could-not-check` on a row that is not genuinely routed —
  enforced in both the model autoflip (`autoflip.go`) and the verify-gate card/`closeVerify`
  (`verifyissues.go`). The routed-row exclusion anchors on `unrun.go`'s own definition
  (`routingKeywordRe` + `routingRefRe`: a routing phrase such as "deferred to" PLUS a
  corroborating reference), not a bare substring "deferred", so negated prose
  ("NOT deferred to anyone, still broken") fails CLOSED instead of silently suppressing the
  contradiction.
- `TestVerifyrunPipelineExit` pins a measured pipe-masked false-clean
  (`<bad cmd> 2>/dev/null | head -c1`) directly, alongside the existing pipefail regression
  coverage.

### Changed
- `spec/lifecycle-v1.md` §2.4's "no execution witness" sentence is now date-bounded to
  closures before statusgen v1.0.13; `plugins/assay/skills/verify-desk/SKILL.md` names
  `statusgen verifyrun` as the run step and the commit-with-Evidence step explicitly.
- `docs/evidence-bundle.md` and `statusgen/README.md` now describe witness *absence* as
  merge-base scoped (grandfathered NOTICE vs. hard PROBLEM for a new post-pin closure),
  matching `spec/lifecycle-v1.md` §2.4 — they previously asserted the flat pre-PR "NOTICE,
  not a block" behaviour unconditionally.
