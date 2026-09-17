---
brief: assay:assay:forge-neutral:27
title: Forge-ref store on GitLab — wildcard protection where the tier allows it, and the Free tradeoff provisioned honestly
why: >-
  On GitLab the reviewer account must hold a role that can also push to unprotected branches,
  and only Premium and above can name which accounts may push. With claims kept on GitLab, an
  adopter on Free who is told nothing will assume a separation they do not have — and will not
  learn that moving claims off the forge removes the problem on every tier.
wave: 2
depends: ["forge-neutral/20"]
unblocks: ["forge-neutral/29"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "tools/create-fleet-gitlab.sh — the fleet provisioner; it already prints `failed-at-tier` rows for controls a tier lacks"
  - "docs/adopting-assay-gitlab.md:60-78 (Free-tier table) and :108-121 (role table: reviewer at Developer, PAT scope `api`)"
  - "docs/streams/forge-neutral/brief-05-claim-layer-forge-shape.md — a claim outside `refs/heads` cannot be released on GitLab; the forge-ref store inherits this"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): most-permissive-rule semantics across overlapping wildcard protections decide whether the bound holds, and the answer differs by tier"
gate-why: >-
  This brief changes what the GitLab provisioner protects for role accounts under the forge-
  ref store — access control on the identity chain — and fixes the words an adopter on Free
  reads about a boundary they do not have. The human confirms the protection shape per tier,
  that the Free output neither overstates nor buries the tradeoff, and that it points at the
  file and service stores as the way out.
decision-trigger: start
domain: complicated
consumers:
  - "tools/create-fleet-gitlab.sh: follow-up forge-neutral/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/repohardenguard (GitLab rows): follow-up forge-neutral/27 (this brief; flips to fixed-here when the implementation edits the path)"
  - "the roster's GitLab profile key: follow-up forge-neutral/25 (defines and parses it; this brief only prints the line to add)"
  - "docs/adopting-assay-gitlab.md: follow-up forge-neutral/29"
version: 1
id: 3c2275d2-26c2-4fd6-9161-222e4f1c6b36
---

# Brief 27 — Forge-ref store on GitLab: the bound and the Free tradeoff

## Context
files:
- `tools/create-fleet-gitlab.sh` and its test harness
- `tools/desk/cmd/repohardenguard/` GitLab checklist rows and `gitlab_test.go`
- `docs/streams/forge-neutral/reviewer-write-boundary.md` — row F14 gains its result
- `changelog/<branch-slug>.md` (planned)

**Scope: the `forge-ref` store only.** The provisioner asks (or is told by flag) which claim
store the cell will use; for `file` / `service` it applies none of this and says why.

single-point-of-failure: on Premium/Ultimate, the wildcard protection's allow list — behind it,
the audit rows and (Ultimate) the custom no-push role. On Free with `forge-ref` there is NO
forge-side layer; that is the tradeoff. A second layer on the forge is infeasible there because
the tier has no per-account push rule (measured: allow-list arrays rejected with HTTP 400,
`docs/adopting-assay-gitlab.md:67`). The feasible second layer is not on the forge at all:
use the `file` or `service` store, and the provisioner's Free output says so.

facts:
- Shape per tier: the spec's §9, GitLab paragraph. Premium: protect `*` branches and `*` tags
  with an allow list of the writing roles' accounts; a second rule for `dispatch/*` allowing
  Developers — the most permissive matching rule applies, so the claim prefix stays writable.
- The forge-ref store on GitLab requires the branch-namespace claim (brief 05); a claim under
  `refs/dispatch/*` cannot be released there. The provisioner prints this.
- Brief 20 records whether wildcard protection binds branch creation, whether `refs/dispatch/*`
  can be pushed, and whether an `api`-scope PAT can push over git. Read them first; on a
  contradiction, STOP.
- The provisioner prints the roster profile line for the operator to add. It does not edit
  the roster.

## Human decision
On GitLab, the reviewing account has to hold a role that can also push to ordinary branches.
This only matters when a cell keeps its dispatch claims on GitLab itself, which is needed only
when several separate computers dispatch the same repository. For that setup, paid tiers can
restrict pushing to named accounts and the free tier cannot. The proposal: on paid tiers the
setup script protects all branches and tags so only the writing accounts may push; on the free
tier it applies what exists, states plainly that the reviewing account can still push to
merge-request branches, and points to the alternative of keeping claims off GitLab, which
removes the issue on every tier. The decision is whether running that setup on the free tier
with the stated tradeoff is acceptable.

Options:
1. **Approve as specified.**
2. **Refuse platform-stored claims on the free tier** — free-tier cells must use the folder or
   service store; separate machines on the free tier are unsupported.
3. **Approve, without the permissive claim-prefix rule** — paid-tier cells that keep claims on
   GitLab are not offered a bounded reviewer; they keep the standing notice.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- Live rows run against a throwaway group/project you created.
- Do not remove or soften any existing `failed-at-tier` line in the provisioner.

## Task
1. Provisioner: claim-store input; for `forge-ref` on Premium/Ultimate add the wildcard
   protections and the claim-prefix rule; for `file` / `service` apply none and print why.
2. Provisioner, `forge-ref` on Free: `failed-at-tier, remediation: Premium`, then the one-line
   tradeoff, then the pointer to the `file` / `service` stores.
3. Print the roster profile line and the namespace requirement.
4. Audit rows: protections present; reviewer absent from allow lists; could-not-check when the
   reading account cannot see them.
5. Dereference F14; update the spec's row.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check:ci | the provisioner's documented dry-run form with tier free and claim store forge-ref | exit 0; output contains `failed-at-tier, remediation: Premium`, the tradeoff line, and the pointer to the other stores; no request is sent |
| 3 | check:ci | the same dry-run with tier premium | lists protections for `*` branches and `*` tags whose allow list omits the reviewer account, plus the `dispatch/*` rule |
| 4 | check:ci | the same dry-run with claim store file | lists none of the above and prints that the reviewer needs no repository write |
| 5 | check +flow | **FIXTURE, Premium or Ultimate.** Protections applied. Reviewer account's token, plain git: push a new branch; push to an existing merge-request branch | both REFUSED by the forge, quoting its text |
| 6 | check +flow | **Same fixture.** Same token: take and release a claim with the claim tool under `forge-ref` | exit 0 twice |
| 7 | check | **FIXTURE, Free.** The two pushes of row 5 | ACCEPTED — recorded as the measured tradeoff, in words, not as a failure of this brief |
| 8 | check:ci | `cd tools/desk && go test ./cmd/repohardenguard/ -run 'TestGitLabWildcardProtectionRows' -count=1 -timeout 120s -v` | exit 0; reviewer in allow list → checked-wrong; not visible → could-not-check |
| 9 | check +dereference | Open the forge's merge-request approval eligibility documentation and compare with the spec's F14 row | the row quotes the page and its read date, or stays could-not-check |
| 10 | check | `(cd statusgen && go build -o /tmp/statusgen-fn27 .) && /tmp/statusgen-fn27 --root . --consumers --brief forge-neutral/27` | exit 0 |

If no Premium/Ultimate fixture is available, rows 5–6 are **could-not-check** — reported as
such, never inferred from row 3.

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| Wildcard protection does not bind branch creation | row 5 (new-branch push) |
| The permissive claim-prefix rule matches more than the prefix | row 5 runs with that rule applied |
| Free output reads as a pass, or hides the way out | rows 2, 7 |
| Protections are applied to a cell that does not need them | row 4 |
| Rows 5–6 claimed from a dry-run | the could-not-check note; review |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; role-account access control). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between the reviewer account and a merge-request branch under forge-ref? (Paid tiers: the allow list. Free: none on the forge — stated, with the other stores as the remedy.) Is that acceptable?
2. Does any row prove the lower layer with the upper bypassed? (Row 5: plain git, no desk tool. Row 7 proves the absence on Free rather than assuming it.)
