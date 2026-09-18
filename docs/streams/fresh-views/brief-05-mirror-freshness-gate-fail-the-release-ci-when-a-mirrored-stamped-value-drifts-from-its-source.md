---
brief: assay:assay:fresh-views:05
title: "mirror-freshness gate: fail the release/CI when a mirrored stamped value drifts from its source"
why: >-
  A value copied from a source outside main drifts silently because nothing re-checks the mirror.
  spec/brief-v1.md's header still claims reference implementation v0.22.0 while the released tag is
  v1.0.9 — ~87 releases of drift — because the one-time freshen (#302) never added the release-time
  assertion (#1192). references/codex.md mirrors vendor defaults that no longer match, caught only
  by a manual read (#859). The fix is one mechanism: a mirrored value carries its source + a
  measured-date, and a mechanical gate fails when the mirror disagrees with its source, instead of
  waiting for a human to notice.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1192, 859]
schema: brief-v2
authored: 2026-09-16 by fresh-views scoping session
sources:
  - "docs/streams/fresh-views/spec.md §2 (mirror-freshness corollary: source + measured-date + a gate that fails on drift)"
  - "medici-finance/assay#1192 — spec/brief-v1.md header stale (v0.22.0 vs released v1.0.9); brief-13 Task-4 release-time check never added; no step in release.yml or deskrelease compares the header to the tag"
  - "medici-finance/assay#859 — references/codex.md mirrors vendor defaults that drifted (multi_agent default, multi_agent_v2 flag); carries a 45-day freshness leash — the existing partial instance of this pattern"
  - "spec/brief-v1.md:6 (the mirrored header), .github/workflows/release.yml, tools/desk/cmd/deskrelease/{cut,main,manifest}.go (the release step), plugins/assay/references/codex.md"
  - "freshness-checked 2026-09-16 @ e9fa19d3 (origin/main): #1192/#859 confirmed OPEN; brief-v1.md:6 reads v0.22.0; release.yml/deskrelease carry no header-vs-tag assertion"
exec-tier: strong
exec-tier-why: (b) the gate reasons across the cut tag, the spec header and the manifest; a wrong assertion either never fires (drift returns) or blocks every release
domain: complicated
version: 1
id: 44af73c5-5b7a-4a9a-996d-da7abd07a68b
consumers:
  # Authoring PR: code/spec-path consumers routed to the deferred, self-targeting disposition
  # (rule 6); each flips to fixed-here in the implementation commit that edits the path.
  - "spec/brief-v1.md: follow-up fresh-views/05 (this brief; freshen the header to the current tag as the gate lands — flips to fixed-here when implemented)"
  - "tools/desk/cmd/deskrelease: follow-up fresh-views/05 (this brief; the release step gains the header-vs-tag assertion — flips to fixed-here when implemented)"
  - "plugins/assay/references/codex.md: out-of-scope (the vendor re-measurement in #859 needs a live Codex this brief cannot reach; this brief aligns the mirror-stamp/leash MECHANISM, and routes the data re-measurement to #859's own resolution)"
---

# Brief 05 — mirror-freshness gate for stamped values

## Context
files:
- `tools/desk/cmd/deskrelease/{cut,main,manifest}.go` — the release/cut step: add an assertion that `spec/brief-v1.md`'s `Describes reference implementation:` version equals the tag being cut; fail the cut on mismatch. (Or an equivalent step in `.github/workflows/release.yml` if the cut is gated there — pick the layer that runs on every tag; deskrelease is preferred as it is the code path.)
- `spec/brief-v1.md` (line 6) — the mirrored header value; freshen it to the current tag in the same change so the new gate is green on introduction.
- `plugins/assay/references/codex.md` — READ for the existing 45-day freshness-leash pattern to align the stamp shape to; its vendor re-measurement is #859's own item (out-of-scope here).

facts:
- #1192: `spec/brief-v1.md:6` reads `**Describes reference implementation:** statusgen v0.22.0`; the latest published release is `v1.0.9`. #302 did the one-time freshen but omitted the second half of brief-13 Task 4 — the release-time check. No step in `release.yml` or `deskrelease/{cut,main,manifest}.go` compares the header to the tag (confirmed on main 2026-09-16). Without a mechanical floor the header drifts again.
- #859: `plugins/assay/references/codex.md` mirrors vendor values (`multi_agent` default, `multi_agent_v2.enabled`) that drifted from the vendor reference; the file carries a 45-day freshness leash so it surfaces on its own clock — the EXISTING partial instance of the mirror-freshness pattern. The actual re-measurement against a live Codex is #859's data task, not this brief.
- the pattern (spec §2 corollary): a mirrored value carries (source, measured-date); a gate — a release-time assertion for a value pinned to a release tag, a dated leash for a value pinned to a vendor doc — fails/annunciates when the mirror disagrees with or outlives its source.
- rule 11: this deliverable makes a checkable factual claim ("the header matches the cut tag"). The Verify table must DEREFERENCE — run the gate against a deliberately-mismatched header and confirm it reddens — not merely check the assertion's text is present.
- single point of failure (rule 10): the ONE control is the release-time assertion. It is honest to say NONE stands behind it at cut time for the brief-v1 header (a single mechanical gate on a single value) — a second layer is infeasible without a second independent measurement of "the released version", which the cut tag already IS; the leash pattern (codex.md) is the second, time-based layer for the vendor-doc class. Recorded here rather than manufacturing an assert-spam second layer.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Add the release-time assertion in `deskrelease` (the cut path): read `spec/brief-v1.md`'s `Describes reference implementation:` version and compare it to the tag being cut; fail the cut with a named diagnostic on mismatch.
2. Freshen `spec/brief-v1.md`'s header to the current release tag in the same change, so the gate is green on introduction (do not leave it reading v0.22.0).
3. Add a test that the assertion FAILS on a mismatched header (dereferencing / mutation), and PASSES when the header matches the tag.
4. In the PR body, cite #859 as the sibling instance and state that its vendor re-measurement is routed to #859 (out-of-scope here) — this brief delivers the mechanism, not the codex.md data fix.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskrelease/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/deskrelease/... -run TestHeaderVersionMatchesTag -v` | exit 0; a header version NOT equal to the cut tag makes the assertion exit non-zero with a named diagnostic | check:ci +mutation +flow |
| 3 | `grep -n "Describes reference implementation" ../../spec/brief-v1.md` (from tools/desk) | exit 0; the version equals the current released tag, not `v0.22.0` | check:ci +dereference |
| 4 | `statusgen --consumers --root .` | exit 0 — the diff-aware consumers gate corroborates every routing token against the branch diff | check:ci +dereference |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
