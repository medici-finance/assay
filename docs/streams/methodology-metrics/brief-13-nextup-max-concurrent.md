---
brief: methodology-metrics/13
title: Per-stream max-concurrent — Next-up offers at most N in-flight briefs from a serialized stream
wave: 0
depends: []
unblocks: []
effort: S
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [217]
schema: brief-v1
authored: 2026-07-10 by Fable authoring session (issue #217)
sources: ["issue #217", "[F-20](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-daml-hardening-01-05-and-privacy-hardening-03-edit-the-same-.md)", "issue #156", "methodology-metrics/08", "[F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md)"]
why: >-
  Next-up offers daml-hardening/02 AND /03 together (both edit Core.daml/PoolVault.daml)
  though the stream's [F-20](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-daml-hardening-01-05-and-privacy-hardening-03-edit-the-same-.md) resolution mandates template edits serialize — the 2-per-stream
  cap permits exactly the parallel dispatch the stream forbids, on the highest-risk code in
  the repo. The mandate exists only as prose; the board contradicts it.
---

# Brief 13 — Per-stream max-concurrent: Next-up offers at most N in-flight briefs from a serialized stream

## Context

files:
- `../assay-toolkit/statusgen/parse.go` — `frontmatter` struct + `parseStreamREADME()`
- `../assay-toolkit/statusgen/model.go` — `Stream` struct
- `../assay-toolkit/statusgen/nextup.go` — `nextUp()` pick loop (`perStreamCap` applied there)
- `../assay-toolkit/statusgen/checks.go` — stream-frontmatter lint (the tiering empty-field PROBLEM is
  the style precedent)
- `../assay-toolkit/statusgen/{nextup,parse,checks}_test.go` — tests (TDD)
- `../oit/docs/streams/daml-hardening/README.md` — sets the knob

facts:
- Bug: Next-up offers daml-hardening/02 AND /03 together (both score 3000, same
  `Core.daml`/`PoolVault.daml`), though the stream's [F-20](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-daml-hardening-01-05-and-privacy-hardening-03-edit-the-same-.md) resolution (human:<name>, 2026-07-10)
  mandates template edits SERIALIZE through daml-hardening. `perStreamCap = 2`
  (nextup.go const) permits exactly the parallel dispatch the stream forbids.
- Claim-awareness (mm/08, #156): `claimedBriefs()` in `../assay-toolkit/statusgen/claims.go` maps
  open remote branches (`gitinfo.go` `git ls-remote --heads`, fail-open: empty map =
  "no claims known", NEVER "everything claimed") to `"stream/NN"` keys; `eligible()`
  excludes only the claimed brief itself — same-stream siblings stay offerable.
- Knob name: `max-concurrent: N` (stream README frontmatter, optional `*int`, mirror
  the `Tiering *string` present/absent pattern in parse.go/model.go). Chosen over
  `serialize: true`: it overrides the existing numeric per-stream cap rather than
  adding a boolean concept, and composes with in-flight counting.
- Semantics: per-stream offer budget = `max(0, min(perStreamCap, N) - inFlight(s))`
  where `inFlight(s)` = count of claimed `"s.Name/NN"` keys. Cap-at-pick-time alone is
  NOT enough — with 02 claimed, 03 must NOT be offered.
- Boundary ([F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md)/[I-13](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-09-next-up-scoring-v2-dependency-priority-inheritance-ai-native.md)): ZERO score-input changes — this is pick-time capping only.
  `eligible()` and `Eligible` counting unchanged; serialized-out siblings count as
  held-back in the mm/06 overflow line, same as per-stream-capped briefs today.
- Lint: present means `1 <= N <= perStreamCap` (2); out-of-range is a hard
  PROBLEM ("max-concurrent restricts the per-stream cap, never widens it").
- daml-hardening frontmatter today: `stream/status/priority/track/issues` only.
- Explicitly out of scope: rendering the knob in the STATUS roll-up Notes column
  (tiering-style); any batch-fanout / out-of-repo skill edit (interim pick-one
  mitigation is a desk action recorded on #217, obsolete once this merges); docs-site
  (statusgen is internal methodology tooling — prior mm briefs set the precedent).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating kubectl. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing tests first (`superpowers:test-driven-development`):
   a. parse: `max-concurrent: 1` populates `Stream.MaxConcurrent`; absent = nil.
   b. nextUp: stream with `max-concurrent: 1` and two eligible todo briefs = exactly
      one pick; same stream with one brief claimed (in-flight) = ZERO picks from it;
      claimed map empty (fail-open) = one pick; streams without the knob unchanged
      (existing tests stay green).
   c. lint: `max-concurrent: 0` and `max-concurrent: 3` = PROBLEM; `1`/`2` = clean.
2. Implement: `MaxConcurrent *int` in parse.go frontmatter + model.go Stream, wired in
   `parseStreamREADME`; in `nextUp()` replace the flat `perStreamCap` check with the
   per-stream budget above (helper computing `inFlight` from the claimed map);
   range-check in checks.go.
3. Set `max-concurrent: 1` in `../oit/docs/streams/daml-hardening/README.md` frontmatter and
   add one sentence to its coordination section: the [F-20](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-daml-hardening-01-05-and-privacy-hardening-03-edit-the-same-.md) serialization is now
   board-enforced via this knob (#217).
4. Update `nextUp`'s doc comment (it documents #156 today) to document the knob and
   the [F-09](https://github.com/example-org/oit/blob/main/docs/streams/findings/2026-07-09-next-up-scoring-has-no-value-effort-risk-term-and-findings-d.md) no-score-input boundary.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/` | exit 0 |
| 2 | `go test ./tools/statusgen/ -run 'MaxConcurrent' -v` | exit 0; output contains "PASS"; covers one-pick, zero-pick-when-claimed, and fail-open cases |
| 3 | `statusgen --root . --lint` | exit 0 (daml-hardening's `max-concurrent: 1` accepted) |
| 4 | `statusgen --root . && sed -n '/Next up/,/^## /p' STATUS.md \| grep -c 'daml-hardening'` | prints `0` or `1`, never ≥2 (STATUS.md regen local-only, not committed) |
| 5 | `go vet ./tools/statusgen/` | exit 0 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" requires this filled by someone who did NOT implement. -->

## Review
Gate: model (all four risk answers no — repo-internal Go tooling; the DAML briefs it
schedules keep their own human gates). Reviewer records verdict + date in the stream
README table.
