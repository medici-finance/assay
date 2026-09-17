---
brief: assay:assay:fresh-views:02
title: "read-side loop views re-derive against live main: verifyloop plan + scanloop coalesce"
why: >-
  Two loop tools act on a stale read of the world. verifyloop plan re-lists briefs as
  dispatchable after they were flipped verified on main (#334), inviting double-verification of
  a scarce role window; scanloop coalesce cannot see another session's or a hand-cut scan PR
  (#228), so duplicate scan carriers pile up and their worktrees leak. Both are the same defect
  — a session-local or pre-flip snapshot acted on as if current — so both are fixed by the same
  move: re-derive the view against live main / the live PR set at act time, stamped via the
  fresh-views/01 helper.
wave: 1
depends: ["fresh-views/01"]
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [334, 228]
schema: brief-v2
authored: 2026-09-16 by fresh-views scoping session
sources:
  - "docs/streams/fresh-views/spec.md §2 (stamp + refuse), §5 Q2 (refuse vs auto-refresh)"
  - "medici-finance/assay#334 — verifyloop plan re-lists verified briefs as dispatchable (cached/pre-flip board snapshot)"
  - "medici-finance/assay#228 — scanloop coalesce is session-local, misses other-session/hand-cut scan PRs; key on a stable marker not the human-facing title prefix"
  - "tools/desk/cmd/verifyloop (plan step), tools/desk/cmd/scanloop/coalesce.go:73 (session-local coalesce), fresh-views/01 (the helper this consumes)"
  - "freshness-checked 2026-09-16 @ e9fa19d3 (origin/main): #334/#228 confirmed OPEN; coalesce.go present; verifyloop plan present"
exec-tier: strong
exec-tier-why: (b) correctness spans two tools + the shared helper and depends on end-to-end freshness of a plan/PR-set view, not a single site
domain: complicated
version: 1
id: 5ce328ad-d5e4-4e90-9d13-5d09d8bbc939
consumers:
  # This is an authoring PR (adds this brief, edits no code), so every code-path consumer is
  # routed to the DEFERRED disposition self-targeting this brief (rule 6); the implementation
  # change flips each to fixed-here in the same commit that edits the path.
  - "tools/desk/internal/freshview: follow-up fresh-views/02 (this brief; calls CheckFresh at each act boundary — flips to fixed-here when the implementation wires it)"
  - "tools/desk/cmd/verifyloop: follow-up fresh-views/02 (this brief; plan re-derives against the fetched head — flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/scanloop/coalesce.go: follow-up fresh-views/02 (this brief; coalesce queries the live open-PR set by a stable marker — flips to fixed-here when the implementation edits the path)"
---

# Brief 02 — read-side loop views re-derive against live main

## Context
files:
- `tools/desk/cmd/verifyloop/*` — the `plan` step: stamp the board read with the fetched head; before emitting the dispatchable list, re-read the head and drop briefs whose lifecycle cell advanced to `verified`/`done`, or refuse with a `StaleError` so the caller re-plans.
- `tools/desk/cmd/scanloop/coalesce.go` (line ~73) — the coalesce probe: query the live open-PR set on the target repo by a STABLE marker, not `this session's state` and not the human-facing title prefix.
- `tools/desk/internal/freshview/*` — READ ONLY: the fresh-views/01 helper (`CheckFresh`, `Stamp`, the two modes).

facts:
- #334: `verifyloop plan` keeps listing briefs as dispatchable after they were flipped `implemented → verified` with Evidence rows on `origin/main`; it reads a cached/pre-flip board snapshot. Fix: the plan is a derived view of the board at a sha — stamp it, and re-check the head before the list is acted on.
- #228 mode 1: coalesce state is session-local; an open scan-carrier PR from another/earlier session is invisible, so a fresh `run` cuts a duplicate. Mode 2: a hand-cut scan PR (`statusgen --scan-issues` + `deskpr create`) escapes coalesce entirely. Footgun: the carrier title prefix is `chore(issue-loop): scan …`; a probe searching `chore(intake): scan` finds nothing. Fix: coalesce keys on a stable, documented marker (a hidden body marker or a label), and detects any OPEN scan PR on the target repo, not just this session's.
- trust boundary (widened surface, not pre-existing): this repo is public and accepts outside pull requests, so a PR's body/title is attacker-controllable. Widening the coalesce probe from session-local state to "any OPEN PR on the target repo" means the marker (and the title-shape fallback) is now a remotely-plantable string — an untrusted-author PR carrying the marker must never be treated as a carrier. The probe MUST constrain matches to carriers authored by the roster-trusted scan identity (the issue-loop/scan bot App bound in `ASSAY_TRUSTED_BOT_SLUGS`); this does not cost the brief its stated goal since #228's hand-cut-carrier case is itself authored by a trusted, roster-bound human login.
- refuse vs refresh (spec §5 Q2): verifyloop `plan` is read-only and cheap to re-derive → REFRESH-in-place is acceptable (re-read + drop the now-verified rows), logging that it did; scanloop coalesce acts (cuts a PR) → it must REFUSE-to-cut when a live OPEN carrier already exists (fold into / defer to it).
- shared value (rule 6): the coalesce marker is a value written by one component (the carrier PR) and read by another (the next `run`'s probe) — enumerate and flow-test it.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **verifyloop plan (#334):** wrap the board read in a `freshview.Stamp` (ref `origin/main`). Before emitting the dispatchable list, re-read the head; if it advanced, re-derive the lifecycle cells and DROP any brief now `verified`/`done`. Log the refresh (stamped sha → current head). Add a table test proving a brief flipped `verified` between the read and the emit does not appear in the plan.
2. **scanloop coalesce (#228):** define a stable carrier marker (a hidden HTML-comment marker in the carrier PR body, e.g. `<!-- scan-carrier: <repo> -->`, plus keying off the documented title shape as a fallback). Change coalesce to query the live OPEN PR set on the TARGET repo for that marker and fold-into/defer-to any match — session-local state becomes an optimization, never the source of truth. A hand-cut carrier carrying the marker is then found. **Author trust gate (required, not optional):** the match predicate is `marker (or title-shape fallback) AND author ∈ trusted set` — a marker- or title-bearing PR from an author outside the roster-trusted scan identity (`ASSAY_TRUSTED_BOT_SLUGS`) MUST be ignored for coalesce purposes (fold-into/defer-to never fires on it), optionally surfaced as a filed anomaly. Never fold scan output into, and never defer to, a carrier whose author is untrusted.
3. Both call `freshview.CheckFresh` at their act boundary (verifyloop: before emit; scanloop: before cut). Do not re-implement the staleness check locally.
4. Document the stable coalesce marker where the carrier is created and where the probe reads it (a one-line comment at each site naming the other).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/verifyloop/...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./cmd/verifyloop/... -run TestPlanDropsVerifiedOnMainAdvance -v` | exit 0; a brief flipped `verified` after the initial read is ABSENT from the emitted dispatchable list | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/scanloop/...` | exit 0 | check:ci |
| 4 | `cd tools/desk && go test ./cmd/scanloop/... -run TestCoalesceFindsForeignCarrier -v` | exit 0; a carrier PR carrying the stable marker but NOT in this session's state is detected and coalesced (proves the cross-session flow) | check +flow |
| 5 | `cd tools/desk && go test ./cmd/scanloop/... -run TestCoalesceIgnoresUntrustedAuthor -v` | exit 0; a carrier PR carrying the stable marker (or the title-shape fallback) authored OUTSIDE the roster-trusted scan identity is NOT coalesced — fold-into/defer-to never fires, a fresh carrier is still cut (proves the trust gate; negative case alongside test 4's positive case) | check:ci +mutation |
| 6 | `cd tools/desk && grep -rn "scan-carrier" cmd/scanloop/ \| wc -l` | exit 0; count ≥ 2 (marker written at the create site and read at the probe site) | check:ci +neighbour |
| 7 | `statusgen --consumers --root .` | exit 0 — the diff-aware consumers gate corroborates every routing token against the branch diff (each `follow-up fresh-views/02` self-reference is satisfied by this brief; flips to `fixed-here` at implementation) | check:ci +dereference |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
