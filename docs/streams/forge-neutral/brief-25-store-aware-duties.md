---
brief: assay:assay:forge-neutral:25
title: Store-aware duties — the reviewer needs repository read once the cell's store is set, and the boot check names the store
why: >-
  The boot check refuses any role missing repository write, so an operator who narrows the
  reviewer's grant gets a review desk that will not start — whatever store the cell uses.
  Until the duty list is keyed by role and by claim store, the grant cannot be narrowed
  anywhere. This is the step where the tools stop requiring the grant, and where the standing
  facts an operator must not forget — single-host declared not verified; the old store ends in
  a named release — are put in front of them at every boot.
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
  - "the rulings of 2026-09-17 recorded in the spec's §10 — this brief is written on them"
  - "tools/desk/internal/deskkit/preflight.go:782-786 — `requiredDuties`, one list for every role"
  - "tools/desk/internal/deskkit/preflight.go:801-866 — `checkAppScopes`, its GitLab not-applicable arm (802-832) and the stale-grant note (835-838)"
  - "tools/desk/internal/deskkit/roleapp_binding_test.go:64-82 — pins today's single-list behaviour; updated deliberately"
  - "routed here from assay:assay:forge-neutral:23 (the boot check's store line and notices)"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (c): a duty table too permissive boots a role that fails mid-pass, and one too strict stops every adopter at re-pin; neither shows in a happy-path test"
gate-why: >-
  This brief changes which grants the tools REQUIRE of each role identity — the duty half of
  the identity chain. The human confirms the reviewer row is the only one that changes here
  (the other roles are audited after go-live, by ruling), that an unknown role keeps the
  full set, and that the boot wording states each standing fact in terms an operator will
  act on correctly.
decision-trigger: start
domain: complicated
consumers:
  - "tools/desk/internal/deskkit/preflight.go: follow-up forge-neutral/25 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/README.md (preflight table): follow-up forge-neutral/25 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/adopting-assay.md, docs/enforcement-model.md, docs/adopting-assay-gitlab.md: follow-up forge-neutral/29"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up forge-neutral/29"
  - "deletion of the legacy duty row with the forge store: follow-up forge-neutral/32"
  - "duties of the roles other than the reviewer: follow-up forge-neutral/31 (the audit their later narrowing is authored from)"
version: 1
id: 9d0d6fdb-3db6-4e8e-87df-265bcc49ad38
---

# Brief 25 — Store-aware duties

## Context
files:
- `tools/desk/internal/deskkit/preflight.go` — `requiredDuties` becomes `dutiesFor(role, store)`.
- `tools/desk/internal/deskkit/roleapp_binding_test.go` and the preflight suites.
- `tools/desk/cmd/deskpreflight/mutations.json`; `tools/desk/README.md`;
  `changelog/<branch-slug>.md` (planned)

single-point-of-failure: the duty table — behind it, the store resolver (a reviewer booted at
repository read under the legacy resolution is refused again at dispatch, in a different
component) and the forge itself (a write attempted without the grant is refused server-side).

facts:
- The table is the spec's §8. Only the reviewer row varies, and only by store: `file` /
  `service` → repository read; the legacy resolution (release N only) → repository write.
  Unknown role → full set. **Reviewer-only scope, by ruling (spec §10 I):** the other roles
  are audited and narrowed after go-live (brief 31 onward); this brief keys the table by role
  so that is a row change later, and changes no other row now.
- One direction: store → duty → compare to grant. A missing grant never lowers a duty.
- `Check` already carries a `Notice` field for a non-blocking line
  (`preflight.go:226-230`).
- Store line formats are in the spec's §8; this brief prints them verbatim.
- Notices, each printed on every boot, never cached: surplus grant (write held, duty is read);
  `file` store — "single-host declared, not verified"; `file` store with an undetermined
  filesystem — brief 23's notice, surfaced here; legacy resolution — the removal NOTICE from
  brief 21, naming the release.
- There is no tier-dependent notice and no GitLab profile key: the first draft's GitLab Free
  notice was withdrawn with the forge store (spec §9, §10 J).
- GitLab keeps the not-applicable result for the grant comparison; the store line and notices
  are still produced.
- Until briefs 23/24 ship, only the legacy resolution exists, so this brief is
  behaviour-preserving: a reviewer holding write boots clean exactly as today.

## Human decision
The desk tools refuse to start any role whose identity lacks write access to the repository.
The proposal makes that requirement depend on the role and on where the cell keeps its
dispatch claims: once a cell keeps its claims in a folder or with the claim service, the
reviewer needs read access only. For the one release in which the old platform storage still
exists, a reviewer on it still needs write access. Every other role keeps today's requirement
for now; they are reviewed separately after this change is live. The start-up output will name
the claim storage in use and repeat, every time, the standing facts: that a single-computer
declaration is not verified, and that the old storage ends in a named release. The decision is
whether the requirement change and those notices are acceptable.

Options:
1. **Approve as specified.**
2. **Approve, but fail instead of notify while a cell is still on the old storage** — pushes
   every install to switch immediately; every existing install stops at its next upgrade.
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
3. Update the shared-grant binding test deliberately.
4. Mutation entry: `dutiesFor` returns the reviewer's read set for the `verifier` role.
5. README preflight table.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestDutiesForRoleAndStore' -count=1 -timeout 120s -v` | exit 0; one subtest per role × store; unknown role gets the full set; every non-reviewer role is identical across stores |
| 3 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestAppScopesNamesClaimStore' -count=1 -timeout 120s -v` | exit 0; output contains `claim store: file`, `claim store: service`, and `claim store: forge-ref (legacy` (spec V9) |
| 4 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestReviewerReadGrantRefusedUnderLegacyStore' -count=1 -timeout 120s` | exit 0 — checked-failed naming both remedies: restore the grant, or set the cell's claim store (spec V9, negative path) |
| 5 | check:ci | `cd tools/desk && go test ./internal/deskkit/ -run 'TestStandingNoticesFireEveryBoot' -count=1 -timeout 120s -v` | exit 0; each notice appears on two consecutive boots; the removal NOTICE is absent once a store key is set (spec V3, V8) |
| 6 | check:ci +mutation | the `mutations.json` entry named `duties-verifier-gets-reviewer-read-set` | the verifier subtest of row 2 goes RED |
| 7 | check:ci +neighbour | `cd tools/desk && go test ./internal/deskkit/ -run 'TestMultiRoleSharedGrantPassesEveryBoundRole' -count=1 -timeout 120s` | exit 0 — a full shared grant still passes every bound role |
| 8 | check +flow +dereference | `cd tools/desk && go build -o /tmp/deskpreflight-fn25 ./cmd/deskpreflight && /tmp/deskpreflight-fn25 --help`, then run it for the reviewer role against a fixture config home holding a full grant and no store key | `app-scopes-vs-duties` is checked-clean with `claim store: forge-ref (legacy` and the removal NOTICE — an unchanged install boots as before, and is told |
| 9 | check | `(cd statusgen && go build -o /tmp/statusgen-fn25 .) && /tmp/statusgen-fn25 --root . --consumers --brief forge-neutral/25` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| Another role's duties are narrowed by accident | rows 2, 6 |
| An existing adopter's reviewer stops booting at re-pin | rows 7, 8 |
| A notice prints once and is then suppressed | row 5 |
| The store line says `file` while dispatch would refuse | brief 23 row 6 in the other component; row 3 here |
| Notice wording understates or overstates a standing fact | review-only — wording is the human gate's subject |

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
