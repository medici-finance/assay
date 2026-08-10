---
brief: desk-hardening/09
title: Coverage & attribution — port bugs/ carrier + expected-visibility drift check
why: >
  Two coverage gaps that both let state drift unnoticed across repos. First, the reviewed-issue-
  close carrier flow (close #N via a PR that adds bugs/<N>.md with evidence, a reviewer judges it,
  the merge closes both, and bugs-gc prunes the carrier) exists ONLY in oit; in
  the other desk-worked repos closes are ad-hoc direct gh issue close with no reviewed resolution
  artifact. Second, no loop watches repo SETTINGS — assay-toolkit sat public for ~3 days before a
  human noticed incidentally. Both are cheap to close: port the carrier machinery, and assert an
  expected-visibility map per repo checked against the API in an existing periodic sweep.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [92, 127]
schema: brief-v1
authored: 2026-07-24 by Opus session (desk-hardening authoring pass)
sources:
  - "freshness-checked 2026-07-24 @ 1768aee (net-new control — verified absent on origin/main)"
  - "assay-toolkit#92 (reviewed-issue-close carrier exists only in oit — port to the other desk-worked repos)"
  - "assay-toolkit#127 (visibility drift: a repo sat public 3 days unnoticed — assert expected visibility per repo)"
exec-tier: strong
exec-tier-why: "spans the multi-repo desk fleet (carrier machinery per repo) and a compiled-in expected-visibility map + drift check whose miss is a security-adjacent exposure."
consumers:
  - "[repos] agent-runtime, medici-examples, reconciler, assay-toolkit: fixed-here (bugs/ carrier machinery + close-PR flow in each CLAUDE.md/AGENTS.md)"
  - "[oit] tools/bugs-gc: cross-ref (vendor per repo OR consume as a shared/pinned release)"
  - "[oit] deskkit / deskboard sweep OR the observability watchdog: fixed-here (expected-visibility map + drift check)"
---

# Brief 09 — Coverage & attribution

## Context
files:
- `[repos]` `bugs/` dir + `bugs/README.md` (planned) + `../oit/.github/workflows/daily-harvest.yml` (planned) in `agent-runtime`, `medici-examples`, `reconciler`, `assay-toolkit`; the close-PR flow recorded in each repo's CLAUDE.md / AGENTS.md
- `[oit]` `tools/bugs-gc/` (source machinery — vendor per repo or make a shared/pinned release)
- `[oit]` deskkit (a compiled-in expected-visibility map, same discipline as `allowedRepos`) + a check in `deskboard` sweep or the observability watchdog exporter
out-of-repo files: none
facts:
- #92 source machinery in oit: `../oit/bugs/README.md`, `tools/bugs-gc/`, `../oit/.github/workflows/daily-harvest.yml`, the "Close-PR flow" in CLAUDE.md; carrier = a PR adding `bugs/<N>.md` (resolution claim + evidence frontmatter + verdict), reviewer judges, merge closes both PR and issue, `bugs-gc` (daily) prunes carriers whose issue is CLOSED
- #92 gap: the desk closes issues in agent-runtime/medici-examples/reconciler/assay-toolkit too, but with no carrier there closes are ad-hoc direct `gh issue close` + a comment — no reviewed resolution artifact, no gc (concrete: agent-runtime#22 closed directly as already-fixed, evidence only in a comment)
- #92 scoping decision to record: whether the lightweight deck/report repos also get carriers or are explicitly exempted to direct-close (they rarely carry work issues)
- #127: assay-toolkit was flipped public ~3 days and only noticed when human:<name> spotted it; the desks watch issues/PRs/CI but nothing watches repo SETTINGS; proposal = a compiled-in expected-visibility map (e.g. canton-k8s: public, everything else: private) + a check in an existing periodic surface comparing against the API, alarming on drift — one GET per repo, trivially cheap
- #127 scope note: when the toolkit DOES go public deliberately (open-core free tier, PR #58), the map entry changing is a reviewed PR arriving with the same hardening pass canton-k8s got, not a settings click; the desk trust gate (oit PR #1070) assumes public repos are KNOWN public — this check makes that assumption safe

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Port the bugs/ carrier (#92)** to at minimum the code repos (`agent-runtime`,
   `medici-examples`, `reconciler`, `assay-toolkit`): a `bugs/` dir + `bugs/README.md` (planned)
   documenting the carrier format + close-PR flow (port oit's), access to a `bugs-gc` equivalent,
   and the close-PR flow recorded in each repo's CLAUDE.md/AGENTS.md.
   **Candidate approaches for bugs-gc:** (a) vendor the tool per repo; (b) make oit's
   `tools/bugs-gc` a shared/pinned release consumed by each repo (ties to the "tooling as an
   assay-toolkit release" direction) — recommend (b). Record the deck/report-repo scoping decision.
2. **Expected-visibility map + drift check (#127).** A compiled-in map of expected visibility per
   repo (same C-4 discipline as `allowedRepos`) + a check in an existing periodic surface
   (`deskboard` sweep or the observability watchdog) that GETs each repo's actual visibility and
   alarms on drift. Changing a map entry is a reviewed PR, never a live settings click.
3. Cross-repo deliverables ride sibling draft PRs referenced from the tracking PR (per the
   cross-repo-pairing rule).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | in each ported repo: `test -f bugs/README.md && grep -ci 'close-PR flow\|bugs-gc\|evidence' bugs/README.md` | exit 0; ≥ 1 |
| 2 | `grep -ci 'Close-PR flow\|bugs/<N>' <each repo CLAUDE.md/AGENTS.md>` | exit 0; ≥ 1 |
| 3 | run the visibility check with the map asserting a repo is private while the API reports public | it ALARMS naming the drifted repo (positive control) |
| 4 | run the visibility check when map and API agree | exit 0 / no alarm |
| 5 | `grep -ci 'expected.visibility\|allowedRepos' <the map source>` | exit 0; ≥ 1 |

## Evidence
<!-- one row per Verify item — filled by a NON-implementer -->

## Review
Gate: model. Reviewer records verdict + date. MUST confirm Verify row 3 actually alarms on
simulated drift — a visibility check that never fires is why the repo sat public for 3 days.
