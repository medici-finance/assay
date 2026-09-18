---
brief: assay:assay:auto-triage:03
title: "Judgement responder — file the bug and route it with a recommended default, for the judgement and opaque classes"
why: >-
  For the judgement class of ambient red (a timing flake #612, a duplicated abstraction
  #536) and for opaque reds whose culprit CI withholds (leak-sweep), there is no
  deterministic fix to draft — but the red still must not sit silent. This brief files the
  bug and ROUTES it with a recommended default, so a human gets a named next-step instead
  of a bare stack trace, without the automation ever guessing a fix or a withheld token.
wave: 2
depends: ["auto-triage/01"]
unblocks: ["auto-triage/04"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
gate-why: >-
  This brief WIDENS the automation's standing authority to file issues and post routing
  recommendations autonomously on a red gate, without a fresh human act. That is an
  authority-boundary decision (cf. desk-tools/17: gate is `human` by authority, all four
  risk answers honestly `no` because filing/routing is reversible at zero cost to `main`).
  The human confirms two things: (1) the responder ROUTES only — it never drafts a fix,
  never merges, never flips ready; and (2) for an opaque red (leak-sweep) the routed body
  NEVER contains a withheld token or reconstructed culprit — on a PUBLIC repo a routing
  message that leaked the detail would be the exact harm leak-sweep exists to prevent.
design: DR-auto-triage
issues: [612, 536]
schema: brief-v2
authored: 2026-09-16 by the-desk auto-triage authoring session
sources:
  - "docs/streams/auto-triage/spec.md — §2 step 4 (judgement → file + route with default); §5 leak-sweep is opaque and always routes; §6 non-goals"
  - "docs/streams/decisions/DR-auto-triage.md — the design record this brief is gated on (PROPOSED)"
  - "medici-finance/assay#612 — load-induced timing flake (judgement: re-run/quarantine, not a code fix)"
  - "medici-finance/assay#536 — duplicated reducer (judgement: a consolidation design call)"
  - "freshness-checked 2026-09-16 @ e9fa19d3 (origin/main) — no tools/autotriage/ router exists; leak-sweep status carries only 'withheld content detected'"
exec-tier: strong
exec-tier-why: >-
  (c) autonomous-action code acting publicly, and (b) the opaque-red path must reason across
  the boundary between what CI exposes and what it withholds — a wrong call there leaks a
  withheld token onto a public issue.
consumers:
  - "tools/autotriage/ (the Classify seam from brief 01): follow-up auto-triage/03 (this brief; consumes 01's output schema, flips to fixed-here when the router implementation reads it)"
version: 1
id: 2b1b6409-6509-4d8e-a91d-9d0fc08f40a2
---

# Brief 03 — judgement responder

## Context

single-point-of-failure: this responder's routing message is the one artifact a human acts
on for a judgement/opaque red — but it takes no fixing action, so its worst failure is a
misleading recommendation (a closed/re-labelled issue), not a bad change to `main`. Three
layers stand behind the single control: the responder NEVER drafts a fix or merges (it only
files + routes), the opaque-red path is content-blind by construction (it routes leak-sweep
by CHECK identity and never reads or echoes the withheld detail), and the forge-write
identity itself holds no merge/review/bypass/workflow authority (credential-layer control,
independent of both).

files:
- `tools/autotriage/` — the judgement router (over brief 01's `Classify`), plus tests.
- `tools/autotriage/testdata/` — judgement (#612 flake, #536 duplication) and leak-sweep
  culprit fixtures.

facts:
- Input: a `Culprit` from brief 01 with `class == judgement`, OR any culprit brief 02 routed
  here (non-mechanical, leak-sweep, path-refused, remedy-refused). An ineligible-origin
  culprit is NOT among these — brief 02 refuses that case wholesale itself and never routes
  it here (routing it here would only relocate the same silent drop, since this router would
  refuse it too). This router keeps its own independent eligibility check below regardless,
  as a defense-in-depth layer for the case where it is reached directly (e.g. a `class ==
  judgement` culprit from brief 01, bypassing 02 entirely) with 01's own guard bypassed or
  buggy.
- **Independent eligibility check.** Same posture as brief 01 and brief 02: this router does
  not trust brief 01's `origin` field merely because it is present — it refuses (files
  nothing) any Culprit whose `origin` is missing, whose `trigger` is not in `{push,
  schedule}`, or whose `trigger` is `push` against a `ref` other than `main`, independent of
  brief 01's own refusal. This is a WHOLESALE refusal, identical in shape to brief 01's and
  brief 02's: it files nothing and produces no record beyond that refusal, which is why brief
  02 does not route the ineligible-origin case here — this router's own posture for that case
  is already the dead end, so routing to it would add a hop without changing the outcome.
- Output: file a bug issue naming the check + `file:line` (when known) + the problem, and
  attach a RECOMMENDED DEFAULT next-step — e.g. a flake → "re-run once; if it re-fails,
  quarantine the test and file a fix brief"; a duplication → "route to a consolidation
  brief"; leak-sweep → "a human reads the private gate log; do not guess from the diff".
- **Opaque-red rule:** for `check == leak-sweep` the routed body carries `file: null`,
  names NO token, and echoes NO file content — it routes by check identity only. The
  withheld detail lives in a private channel a human reads; this responder never reproduces
  it on a public surface.
- **Parsed text is sanitized before it is published, for every routed issue** — not only the
  leak-sweep path. The `problem` diagnostic is truncated to a bounded length, wrapped in a
  fenced code block, and rendered inert (no live markdown links, no directive-shaped
  content) before it enters the issue body: contributor/CI-derived text is DATA quoted from
  a log, never live content in the artifact.
- Every routed issue carries a default — "needs a human" with no recommended action is a
  defect, not a route. The default is a RECOMMENDATION a human overrides, never an action
  the responder takes.
- `--dry-run` prints the issue body + the routing recommendation and writes nothing.
- **Arming mechanism.** Same repository-tracked config surface brief 02 uses (never an
  environment variable or self-settable secret) — the identical config path brief 02's
  refused-path allow-list protects; this router has no file-edit surface of its own to
  refuse over, but never treats its own arming state as something it may set.
- **Dedupe is not an abuse bound.** Independent of the `check`+`file:line` dedupe key, this
  responder enforces an absolute ceiling on its own open filed issues and a per-window rate;
  hitting either fails CLOSED — no further filing, escalate instead (brief 04's path).
- Hard bound: this responder NEVER drafts a fix, opens a non-issue PR, merges, or flips
  ready. It files and routes; that is all.

## Ground rules
- NEVER draft a fix, merge, ready-flip, or self-approve — file + route only; the human acts.
- NEVER echo a withheld/opaque-red token or file content onto any surface (public repo).
- File issues ONLY through the sanctioned desk write path; never a hand-rolled mint; live
  writes only when armed (default `--dry-run`).
- **The forge-write identity this responder posts under holds NO merge authority, NO
  review-submission authority, NO branch-protection-bypass authority, and NO
  workflow-write/workflow-dispatch authority** — a credential-layer control, independent of
  the code-level never-drafts/never-merges guarantees, and it survives a code bug in them.
- Stop at `implemented`. Feature branch + draft PR only. Never commit `STATUS.md`/`FINDINGS.md`.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Build the judgement router over brief 01's `Classify`: on `class == judgement` (or a
   brief-02 route — non-mechanical, leak-sweep, path-refused, remedy-refused; NOT
   ineligible-origin, which brief 02 refuses wholesale itself), independently check `origin`
   eligibility (`trigger == schedule` OR (`trigger == push` AND `ref == main`)), then compose
   the issue body and a recommended-default next-step.
2. Implement the opaque-red (leak-sweep) path: route by check identity, `file: null`, no
   token and no file content in the body; recommend the private-log path for a human.
3. Sanitize parsed CI text (truncate, fence, render inert) in EVERY routed body, not only
   the leak-sweep path.
4. Enforce "every route carries a default" — a route with no recommended next-step is a
   hard error in the router, not an emitted issue.
5. Implement `--dry-run` (print, write nothing) as default; live writes only when armed via
   the repository-tracked config.
6. Implement the open-issue ceiling and per-window rate that fail closed to escalation.
7. Assert in tests that no code path drafts a fix, merges, or flips ready.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/autotriage && go build ./... && go vet ./...` | exit 0 |
| 2 | `cd tools/autotriage && go test ./...` | exit 0; all green |
| 3 | `cd tools/autotriage && go test -run TestJudgementRoutesWithDefault -v` | exit 0; the #612 flake culprit yields an issue body naming the check + `file:line` AND a concrete recommended default (re-run/quarantine) — dereferences the route content, not a presence count |
| 4 | `cd tools/autotriage && go test -run TestNeverDraftsFixNeverMerges -v` | exit 0; asserts NO code path drafts a fix, merges, or flips ready (mutation/negative row) |
| 5 | `cd tools/autotriage && go test -run TestLeakSweepRoutedWithoutLeakingDetail -v` | exit 0; the leak-sweep fixture routes with `file` null and a body containing NO token and NO fixture file content — the public-repo safety row, proven by asserting the withheld synthetic marker is absent from the routed body |
| 6 | `cd tools/autotriage && go test -run TestRouteWithoutDefaultIsError -v` | exit 0; a route constructed with no recommended default is rejected by the router (proves "every route carries a default") |
| 7 | `cd tools/autotriage && go test -run TestRefusesIneligibleOrigin -v` | exit 0; a Culprit whose `origin.trigger` is `pull_request` (or `origin` absent) is refused and files nothing — independent of brief 01's or brief 02's own refusal |
| 8 | `cd tools/autotriage && go test -run TestRefusesPushToNonMainRef -v` | exit 0; a Culprit whose `origin.trigger` is `push` but whose `origin.ref` is a non-`main` branch is refused and files nothing — proving this router's independent check gates on `ref` as well as `trigger`, matching briefs 01 and 02 |
| 9 | `cd tools/autotriage && go test -run TestRoutedBodySanitizesParsedText -v` | exit 0; a NON-leak-sweep judgement culprit's routed body wraps the parsed `problem` text in a fenced block with no live markdown link or directive content passed through — the sanitize rule applies beyond the opaque-red path |
| 10 | `cd tools/autotriage && go test -run TestCeilingFailsClosedToEscalation -v` | exit 0; once the open-issue ceiling or the per-window rate is hit, the router files NOTHING further and instead emits the escalation signal brief 04 observes |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item.
     gate: human — green rows are INPUT to the human review (route-only; no detail leak),
     not a substitute. Status stays `implemented` until the human confirms and
     DR-auto-triage is approved. -->

## Review
Gate: human (autonomous outbound action; authority widening — see gate-why; design:
DR-auto-triage). Reviewer answers both core-control questions: (1) the single control is the
routing message; the layers making it acceptable are route-only (no fix/merge) and
content-blind opaque-red handling — confirm both; (2) row 5 proves the lower, content-blind
layer holds with the happy path bypassed (a leak-sweep red routes with zero withheld detail
on a public surface). Reviewer also confirms rows 7–8 close the input-authorization gap on
BOTH halves of `origin` (trigger and ref) raised on security re-review, that the Input list
no longer names "ineligible origin" as something brief 02 routes here (it dead-ends at 02
instead — confirm this router's own refusal for that case is wholesale, not a filed no-op
disguised as a route target), and that rows 9–10 close the general-sanitize and
abuse-ceiling gaps raised on security review, and that the credential-layer ground rule is
stated. Verdict + date in the stream README table.
