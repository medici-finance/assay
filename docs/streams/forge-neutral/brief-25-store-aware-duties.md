---
brief: assay:assay:forge-neutral:25
title: Store-aware duties — the reviewer needs repository write only under the forge-ref store, and the boot check names the store
why: >-
  The boot check refuses any role missing repository write, so an operator who narrows the
  reviewer's grant gets a review desk that will not start — whatever store the cell uses.
  Until the duty list is keyed by role and by claim store, the grant cannot be narrowed
  anywhere. This is the step where the tools stop requiring the grant, and where the standing
  tradeoffs (single-host declared not verified; GitLab Free with forge-stored claims) are put
  in front of the operator at every boot.
wave: 3
depends: ["forge-neutral/21"]
unblocks: ["forge-neutral/29", "forge-neutral/30"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "tools/desk/internal/deskkit/preflight.go:782-786 — `requiredDuties`, one list for every role"
  - "tools/desk/internal/deskkit/preflight.go:801-866 — `checkAppScopes`, its GitLab not-applicable arm (802-832) and the stale-grant note (835-838)"
  - "tools/desk/internal/deskkit/roleapp_binding_test.go:64-82 — pins today's single-list behaviour; updated deliberately"
  - "routed here from assay:assay:forge-neutral:23 (the boot check's store line and notices) and assay:assay:forge-neutral:27 (the GitLab profile key)"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (c): a duty table too permissive boots a role that fails mid-pass, and one too strict stops every adopter at re-pin; neither shows in a happy-path test"
gate-why: >-
  This brief changes which grants the tools REQUIRE of each role identity — the duty half of
  the identity chain. The human confirms the reviewer row is the only one that changes, that
  an unknown role keeps the full set, and that the boot wording states each standing
  tradeoff in terms an operator will act on correctly.
decision-trigger: start
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/preflight.go: follow-up forge-neutral/25 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/internal/deskkit/rosterconfig.go (the GitLab profile key): follow-up forge-neutral/25 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md (preflight table): follow-up forge-neutral/25 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/adopting-assay.md, docs/enforcement-model.md, docs/adopting-assay-gitlab.md: follow-up forge-neutral/29"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up forge-neutral/29"
version: 1
id: 9d0d6fdb-3db6-4e8e-87df-265bcc49ad38
---

# Brief 25 — Store-aware duties

## Context
files:
- `tools/desk/internal/deskkit/preflight.go` — `requiredDuties` becomes `dutiesFor(role, store)`.
- `tools/desk/internal/deskkit/rosterconfig.go` — the per-repo GitLab profile key (spec §10 J).
- `tools/desk/internal/deskkit/roleapp_binding_test.go` and the preflight suites.
- `tools/desk/cmd/deskpreflight/mutations.json`; `tools/desk/README.md`;
  `changelog/<branch-slug>.md` (planned)

single-point-of-failure: the duty table — behind it, the store resolver (a reviewer booted at
repository read under `forge-ref` is refused again at dispatch, in a different component) and
the forge itself (a write attempted without the grant is refused server-side).

facts:
- The table is the spec's §8. Only the reviewer row varies, and only by store: `file` /
  `service` → repository read; `forge-ref` → repository write. Unknown role → full set.
- One direction: store → duty → compare to grant. A missing grant never lowers a duty.
- `Check` already carries a `Notice` field for a non-blocking line
  (`preflight.go:226-230`).
- Store line formats are in the spec's §8; this brief prints them verbatim.
- Notices, each printed on every boot, never cached: surplus grant (write held, duty is read);
  `file` store — "single-host declared, not verified"; `forge-ref` and no fresh clean
  hardening audit — "server-side bound: could-not-check" (spec §10 G); `forge-ref` with
  GitLab profile Free on the reviewer role — the one-line form of the spec's §9 text.
- GitLab keeps the not-applicable result for the grant comparison; the store line and notices
  are still produced.
- Until briefs 23/24 ship, only `forge-ref` resolves, so this brief is behaviour-preserving:
  a reviewer holding write boots clean exactly as today.

## Human decision
The desk tools refuse to start any role whose identity lacks write access to the repository.
The proposal makes that requirement depend on the role and on where the cell keeps its
dispatch claims: every role keeps it except the reviewer, whose requirement drops to read
access unless claims are kept on the code-hosting platform. The start-up output will name the
claim storage in use and repeat, every time, any standing limitation: that a single-computer
declaration is not verified, that the platform-side protection has not been audited, or that
on GitLab's free tier with platform-stored claims the reviewer account can still push. The
decision is whether the requirement change and those notices are acceptable.

Options:
1. **Approve as specified.**
2. **Approve, but fail instead of notify when the platform-side protection is unaudited** —
   stricter; every existing install stops at its next upgrade until the audit is set up.
3. **Reject** — all roles keep the uniform requirement; the grant cannot be narrowed.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- Do not weaken the check for any role other than the reviewer. If another role's test needs changing to pass, STOP and report.

## Task
1. Role-keyed table and `dutiesFor(role, store)`; store from brief 21's resolver.
2. `checkAppScopes` compares against it, appends the store line, sets the notices.
3. GitLab profile key, strictly parsed; needed only for the Free notice.
4. Update the shared-grant binding test deliberately.
5. Mutation entry: `dutiesFor` returns the reviewer's read set for the `verifier` role.
6. README preflight table.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestDutiesForRoleAndStore' -count=1 -timeout 120s -v` | exit 0; one subtest per role × store; unknown role gets the full set |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestAppScopesNamesClaimStore' -count=1 -timeout 120s -v` | exit 0; output contains `claim store: file`, `claim store: service`, `claim store: forge-ref` (spec V9) |
| 4 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestReviewerReadGrantRefusedUnderForgeRef' -count=1 -timeout 120s` | exit 0 — checked-failed naming both remedies: restore the grant, or move the cell's claims off the forge (spec V9, negative path) |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestStandingNoticesFireEveryBoot' -count=1 -timeout 120s -v` | exit 0; each of the four notices appears on two consecutive boots; the Free notice is absent under `file` and under profile premium (spec V3, V8) |
| 6 | check:ci +mutation | the `mutations.json` entry named `duties-verifier-gets-reviewer-read-set` | the verifier subtest of row 2 goes RED |
| 7 | check:ci +neighbour | `cd tools/desk && go test ./internal/deskkit/ -run 'TestMultiRoleSharedGrantPassesEveryBoundRole' -count=1 -timeout 120s` | exit 0 — a full shared grant still passes every bound role |
| 8 | check +flow +dereference | `cd tools/desk && go build -o /tmp/deskpreflight-fn25 ./cmd/deskpreflight && /tmp/deskpreflight-fn25 --help`, then run it for the reviewer role against a fixture config home holding a full grant and no store key | `app-scopes-vs-duties` is checked-clean with `claim store: forge-ref` — an unchanged install boots as before |
| 9 | check | `(cd statusgen && go build -o /tmp/statusgen-fn25 .) && /tmp/statusgen-fn25 --root . --consumers --brief forge-neutral/25` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| Another role's duties are narrowed by accident | rows 2, 6 |
| An existing adopter's reviewer stops booting at re-pin | rows 7, 8 |
| A notice prints once and is then suppressed | row 5 |
| The store line says `file` while dispatch would refuse | brief 23 row 5 in the other component; row 3 here |
| Notice wording understates or overstates a tradeoff | review-only — wording is the human gate's subject |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; the duty set of role identities). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between a too-narrow grant and a desk that fails mid-pass? (The duty table; beneath it the dispatch-time resolver and the forge's own refusal.)
2. Does any row prove a lower layer with the upper bypassed? (Row 4 hands the boot layer a state it must refuse; brief 21 row 6 proves dispatch refuses independently.)
