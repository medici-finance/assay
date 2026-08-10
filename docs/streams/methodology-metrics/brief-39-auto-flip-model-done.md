---
brief: methodology-metrics/39
title: Auto-flip verified→done for gate:model briefs from the reviewer-App approval
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [885]
schema: brief-v1
authored: 2026-07-20 by Opus 4.8 authoring session (intake Tier-2, #885)
sources: ["[I-auto-flip-done](https://github.com/example-org/oit/blob/main/docs/streams/intake/2026-07-17-auto-flip-verified-to-done-from-app-approval.md)", "docs/streams/methodology/verify-desk-bottleneck-2026-07-17.md (R2)", "methodology-metrics/12 (the gate:human analogue this mirrors)", "methodology-metrics/15 (human-stamp corroboration — the SHA-recording discipline)", "tools/statusgen/verifyissues.go closeVerify + .github/workflows/verify-gate-close.yml", "#885 (tracking)"]
why: >-
  The verified→done touch for gate:model briefs is pure bookkeeping — commit df7ea6b4 closed 47
  briefs in one commit purely by reading reviewer-app[bot] APPROVED reviews already on
  GitHub. The Reviewed cell is derivable the moment the brief is verified, yet lh/14 waited 7
  days and frontend/01 9 days for this mechanical flip. Machinery half-exists: verify-gate-close.yml
  does exactly this for the gate:human path. Keep the human path unchanged; delete the model-path
  second touch.
---

# Brief 39 — Auto-flip verified→done for gate:model briefs from the App approval

## Context
files:
- `../assay-toolkit/statusgen/verifyissues.go` — `closeVerify` state machine (the gate:human analogue);
  the review-resolution helpers
- `../assay-toolkit/statusgen/corroborate.go` — mm/15's human-stamp corroboration (the pattern for
  recording PR# + head SHA against a stamp — reuse it, don't re-derive)
- `../oit/.github/workflows/verify-gate-close.yml` — the gate:human close workflow (the shape to mirror
  for a main-CI model-path step); a NEW/extended workflow step for the model path
- `../assay-toolkit/statusgen/verifyissues_test.go` / `corroborate_test.go`
- `docs/streams/methodology-metrics/README.md` — one convention line

facts:
- Scope: a **`gate: model`** brief at `verified` whose merge-PR carries an
  `reviewer-app[bot]` **APPROVED** review at the merged head. For such a brief, a
  statusgen/main-CI step resolves that review and, in the SAME commit that records `verified`:
  - stamps the Reviewed cell **recording PR# + head SHA** (per the verification-integrity finding
    that a bare re-transcribed approval can close a brief again — mm/15's discipline is REQUIRED,
    not optional), and
  - flips `verified → done`.
- **The gate:human path is UNCHANGED.** This brief touches ONLY the model path — it deletes the
  separate second human-transcription touch that today lags 7-9 days. verify-gate-close.yml's
  human:<name>-only closer and the whole `gate: human` machinery are untouched.
- **Corroboration is mandatory (mm/15):** the flip requires the App review to exist at the
  recorded head SHA — a re-transcribed or stale approval must not re-close a brief. Reuse mm/15's
  corroboration check; a model brief whose recorded SHA does not match a live App APPROVED review
  does NOT auto-flip (it stays `verified`, surfaced, not silently advanced).
- Overlaps the board-truthfulness intake item (I-65) — this makes `done` mean the same thing it
  did before, just recorded without the manual lag.
- Depends on: nothing (the App reviews and the historian already exist).
- Out of scope: the gate:human path (mm/12 owns it); any change to what "verified" requires (the
  verifier still runs the Verify table — this brief only removes the SEPARATE Reviewed touch).

## Ground rules
- NEVER git push to main / trigger workflows / mutating kubectl. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit STATUS.md on a branch (single writer = main's CI).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. TDD — failing tests first (fake gh/review seam): a verified gate:model brief with an App
   APPROVED review at the merged head → Reviewed stamped with PR#+SHA AND flipped to done in one
   step; the same brief where the recorded SHA does NOT match a live App approval → NOT flipped
   (stays verified, corroboration fails); a gate:human brief → the model path never touches it
   (human machinery unchanged); a verified gate:model brief with no App approval → not flipped.
2. Implement the model-path resolve-and-flip (reusing mm/15 corroborate) in the statusgen/main-CI
   step; add/extend the workflow step (workflow file only — never trigger it). Remove the
   now-redundant separate model-path Reviewed touch.
3. README: one line under conventions — verified→done for gate:model auto-flips from the App
   approval at head (PR#+SHA recorded, mm/15 corroboration required); the gate:human path is
   unchanged.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./tools/statusgen/ -run AutoFlip -v` | exit 0; `TestAutoFlip*` covers the four Task-1 cases (flip, SHA-mismatch-no-flip, human-untouched, no-approval-no-flip) |
| 2 | `go test ./tools/statusgen/ && go vet ./tools/statusgen/` | exit 0 |
| 3 | `yq eval '.' .github/workflows/verify-gate-close.yml > /dev/null; echo $?` | 0 (workflow YAML still parses after the model-path step) |
| 4 | `statusgen --root . --lint; echo $?` | 0 |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — repo-internal Go tooling + main-CI bookkeeping; it
records what an tamper-evident App review already established, with SHA corroboration; the
gate:human path is untouched). Reviewer confirms (a) the flip records PR#+head SHA and refuses on
SHA mismatch, (b) gate:human briefs are never touched by the model path, and records verdict +
date in the stream README table.
