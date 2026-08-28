---
brief: statusgen/12
title: "`homed-in: <owner/repo>` brief field — exclude a brief whose deliverable lives in another repo from THIS board's Next-up, keep its tracking row, carry the target repo"
why: >-
  When a brief's deliverable is moved to a different repository than the board that renders it, the
  brief still renders `todo` with a full Next-up score on the origin board, so it surfaces as a
  top dispatch candidate. A dispatcher then burns a whole slot to DISCOVER the work is not in this
  repo — one wasted run per miss, every cycle, forever, because nothing on the board says the work
  moved. It is a recurring class, not a one-off: every partial de-housing leaves the same footprint.
  `homed-in` turns that silent mis-route into a machine-readable fact — the brief leaves the local
  dispatch pool, keeps its tracking row, and names the repo a cross-repo dispatcher should target.
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-08-27 (authored for the statusgen board)
exec-tier: strong
exec-tier-why: >-
  Cross-artifact reasoning where a subtle error survives the brief's own tests: parse (brieffile.go),
  the Brief row (model.go), the eligibility gate (nextup.go) and the render sites must agree on ONE
  meaning of the field, and the failure mode is exactly the dangerous one — an eligibility exclusion
  that silently hides work from a dispatcher (the same class as the StaleRef broadcast that hid ~596
  briefs). Absence must stay byte-identical to today's board, which is a whole-board invariant, not a
  local check.
sources:
  - "De-housed briefs whose deliverable moved to another repo still render todo + full Next-up score on the origin board and surface as top dispatch candidates; each costs a dispatch slot to discover the mis-route (observed on a live top-of-board fanout)"
  - "The existing eligibility-exclusion precedents this mirrors exactly (claim-awareness, per-stream max-concurrent, drain-before-instrument): eligibility-only, ZERO score-input change (F-09 boundary)"
  - "The optional-KNOWN-key pattern already used for exec-tier / blocked-by / measures (parse as a recognized field, default-inert on absence, PROBLEM on a malformed present value)"
  - "The fail-loud-when-hiding-work discipline: a brief withheld from Next-up must be NAMED on the board, never silently dropped (the StaleRef ~596-brief silent-hide lesson)"
gate-why: n/a (gate is model; all four risk answers are no)
---

# Brief 12 — `homed-in: <owner/repo>` — a re-home pointer that leaves the local dispatch pool but keeps the tracking row

statusgen builds a board per repo root. A brief lives on the board of the repo that holds its
`docs/streams/<stream>/` file — but its DELIVERABLE may have been moved to a different repo (a
de-housing). Today the board has no notion of that split, so the brief scores and ranks as if the
work were local. This brief adds the optional frontmatter field `homed-in: <owner/repo>` that
records the split as a fact the board acts on.

Three required properties (all three, or the field is not doing its job):

1. **Excluded from Next-up eligibility** — a `homed-in` brief is never offered as a dispatch
   candidate on THIS board, so it stops being a dispatch magnet.
2. **Tracking row preserved** — the brief still renders in its stream README table and still counts
   as tracked work. `homed-in` is not `done` (the work is still owed) and not a deletion.
3. **Target repo carried** — the `<owner/repo>` value is rendered on the row and exposed on the
   Next-up view struct, so a cross-repo dispatcher reads the right target instead of discovering it.

## Context

files:
- `statusgen/brieffile.go` — parse `homed-in` as an OPTIONAL KNOWN string key (exactly the
  exec-tier / blocked-by / measures pattern): absent → `""` (normal in-repo brief, the default);
  a wrong TYPE → parse error; wire it into the `BriefFile` struct. In `checkBriefFiles`, validate a
  PRESENT value's shape and worm it into the `Brief` row.
- `statusgen/model.go` — add `HomedIn string` to `Brief` (documented as: `homed-in: <owner/repo>`;
  `""` = local, the default; an eligibility exclusion + display marker, NEVER a Next-up score input,
  F-09 scope note — same wording as ExecTier / Measures).
- `statusgen/nextup.go` — `eligibleBase`: a brief with `HomedIn != ""` is not eligible on this
  board (return false, all statuses). Attribute the exclusion on the `NextUp` view so the held-back
  brief and its target repo are NAMED (mirror `MeasuresGated`/`SerializedUnknown`), not silently
  dropped.
- render site(s) (`emit.go` / the Next-up + stream-table render) — render a `[homed→<owner/repo>]`
  marker on the brief's tracking row wherever the exec-tier `[exec:strong]` marker already renders,
  and a short "N brief(s) homed in another repo" line naming each id + target under the board.
- `statusgen/brieffile_test.go`, `statusgen/nextup_test.go` — tests below.

single-point-of-failure: the eligibility exclusion is the ONE control that keeps a homed-in brief
out of dispatch — and the design's real risk is the exclusion firing too WIDE (hiding a local brief)
or the field being ignored (hiding nothing). The independent second layer against "hides too wide"
is that the exclusion is opt-IN and self-selected (only a brief that WROTE `homed-in` can be held)
AND every held brief is NAMED on the board (a wrongly-excluded brief is visible, not silent) — the
same bounded-loud-self-selected posture drain-before-instrument uses. There is no on-ledger / funds
surface here, so NONE of the core-system layers apply; the "named on the board" property is the
designed second control.

facts:
- Optional-known-key precedent to copy verbatim: exec-tier (`brieffile.go` ~L426), blocked-by
  (~L454), measures (~L469) — parsed independently, defaulted-inert on absence, echoed in a
  semantic PROBLEM when present-but-invalid.
- Eligibility-exclusion precedent: `eligibleBase` in `nextup.go` already excludes on
  stream-status / StaleRef / claim / (via `eligible`) drain-before-instrument. Add the `HomedIn`
  exclusion in `eligibleBase` so it composes with the rest and the `all`-pick loop can attribute it.
- Attribution precedent: `NextUp.MeasuresGated` / `NextUp.SerializedUnknown` — a `[]string` of
  ids the board names; `homed-in` additionally needs the target repo, so carry `map[string]string`
  (id → owner/repo) or a small struct, whichever the render site reads cleanly.
- Shape validation: `<owner>/<repo>` — exactly one `/`, both sides non-empty, no whitespace. Do NOT
  validate against a repo allowlist (statusgen has no such list and must not couple to one).
- Score boundary (F-09): `homed-in` is eligibility-only. It MUST NOT appear in the score formula in
  `nextup.go`. A brief that (hypothetically) survived the gate must score exactly what it scores
  today.
- **Prior art to INTEGRATE with, not duplicate**: statusgen already emits a NOTICE-only board-honesty
  heuristic — `NON-DISPATCHABLE (dehoused)` / `NON-DISPATCHABLE (re-homed)` — that GUESSES a brief's
  work has moved from its stream README banner or body keywords, warns it "inflates the Next-up
  count," but does NOT exclude it from eligibility and carries no target repo. `homed-in` is the
  explicit, author-declared successor: it turns that guess into a fact that actually excludes and
  carries the pointer. Locate that detector and wire the two together — a brief with a valid
  `homed-in` should satisfy the board-honesty check (no phantom NOTICE), because the explicit field
  is now doing precisely what the heuristic was estimating. Do not leave both firing on the same row.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. `brieffile.go`: parse the optional `homed-in` key into `BriefFile.HomedIn`; a non-string value is
   a parse error (`homed-in must be a string`). Default `""`.
2. `model.go`: add `Brief.HomedIn` with the F-09 scope-note doc comment.
3. `checkBriefFiles`: when `HomedIn != ""`, validate the `<owner>/<repo>` shape; a malformed value is
   a hard PROBLEM echoing the bad value (`invalid homed-in %q (want <owner>/<repo>)`). On a valid
   value, worm it into `row.HomedIn` (same place Gate/ExecTier are wired). An invalid value is left
   OFF the row (like value/exec-tier) so a typo cannot silently exclude a brief — it reddens lint
   instead.
4. `nextup.go` `eligibleBase`: `if b.HomedIn != "" { return false }`. In the `all`-pick loop,
   record the excluded brief's id + target on the `NextUp` view (new field) so it is named.
5. render: `[homed→<owner/repo>]` marker on the tracking row + the "homed in another repo" board
   line naming each id and its target.
6. tests (see Verify).

Absent `homed-in` must change nothing: a board built from briefs that none carry the field is
byte-identical to today's (the additive-inert invariant every optional key holds).

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | `go test ./statusgen/` | exit 0; new tests present: absent-is-inert, present-excludes-from-Next-up, present-keeps-tracking-row, malformed-is-PROBLEM, target-repo-carried-on-view |
| 2 | build the binary, run `--lint --root .` on a fixture tree with one `homed-in: owner/repo` brief | exit 0; that brief is NOT in the Next-up picks but IS present in its stream README table render |
| 3 | same fixture, but the brief's value is `homed-in: not-a-repo` | `--lint` exit 1; message contains `invalid homed-in "not-a-repo"` and echoes the file path |
| 4 | build from a tree where NO brief carries `homed-in`; diff its `STATUS.md` render against the pre-change binary's render of the same tree | identical output — the additive-inert invariant (absent field ⇒ byte-identical board) |
| 5 | inspect the `NextUp` value (or the rendered board) for a fixture with `homed-in: acme/widgets` | the excluded brief's id and the string `acme/widgets` are both reachable/rendered — a cross-repo dispatcher can read the target without opening the brief |
| 6 | `git diff --name-only $(git merge-base HEAD origin/main) HEAD -- STATUS.md` | empty — STATUS.md is not committed on the branch (single-writer = main CI) |

## Evidence
<!-- appended at implementation time: one row per Verify item (command, exit code, output line(s)
     or hash, date, runner). "verified" requires this filled by someone who did NOT implement. -->

## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the statusgen README table.

## Release note (why this is inert until a release + re-pin)
This brief lands the SOURCE change in statusgen. A CONSUMER repo that runs a pinned statusgen
release binary gains NO new behaviour until (a) a new statusgen release is cut carrying this change
and (b) the consumer re-pins that release in its `.assay-versions`. Adding a `homed-in:` key to a
brief on a consumer board BEFORE the re-pin is safe — an older statusgen ignores unknown frontmatter
keys — but the Next-up exclusion, the row marker, and the malformed-value lint only take effect once
the consumer is on a release that understands the field. The downstream board sweep that applies the
field is a SEPARATE, later unit of work gated on that release + re-pin.
