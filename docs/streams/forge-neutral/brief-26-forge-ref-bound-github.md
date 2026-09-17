---
brief: assay:assay:forge-neutral:26
title: Forge-ref store on GitHub — a ruleset pair that leaves the reviewer only the claim namespace, and the audit rows that watch it
why: >-
  A fleet of separate machines that share nothing but GitHub must keep claims on the forge,
  and there the reviewer keeps repository write. In that one setup the only thing that makes
  'cannot move the head it approves' true is the forge refusing the push. Without a shipped,
  tested ruleset shape each adopter invents their own, and one bypass-list edit quietly
  reopens the gap.
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
  - "tools/desk/cmd/repohardenguard/ — the existing GET-only, three-state hardening reader; this brief adds checklist rows, not a tool"
  - "docs/streams/forge-neutral/brief-19-human-only-surfaces-server-side.md — rulesets are admin-only; applying one is a human act"
  - "docs/streams/forge-neutral/claim-shape.md §3 C4 — a ruleset covering the claim prefix blocks claim release"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): the template must hold against every write route to a branch and under both claim namespaces"
gate-why: >-
  This brief defines the write boundary that stands in for a narrower grant under the forge-
  ref store — an access control on the identity chain. The human confirms the bypass list
  names only writing roles, that the pair is separate from default-branch protection, and
  that the fixture rows were run with the reviewer credential and refused by the forge
  itself.
decision-trigger: start
domain: complicated
consumers:
  - "tools/desk/cmd/repohardenguard (checklist rows): follow-up forge-neutral/26 (this brief; flips to fixed-here when the implementation edits the path)"
  - "tools/desk/cmd/deskflip (refuse a head inside the claim prefix): follow-up forge-neutral/26 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/adopting-assay.md: follow-up forge-neutral/29"
version: 1
id: cae62628-6d19-4b0d-b1e3-5fc00a16ca7b
---

# Brief 26 — Forge-ref store on GitHub: the server-side bound

## Context
files:
- `docs/workflow-templates/rulesets/writers-only-branches.json` (planned),
  `docs/workflow-templates/rulesets/writers-only-tags.json` (planned),
  `docs/workflow-templates/rulesets/README.md` (planned)
- `tools/desk/cmd/repohardenguard/checklist.go` and tests
- `tools/desk/cmd/deskflip/flip.go` and tests
- `changelog/<branch-slug>.md` (planned)

**Scope: the `forge-ref` store only.** Under `file` and `service` the reviewer holds no
repository write and none of this applies; the README says so in its first paragraph.

single-point-of-failure: the ruleset pair — behind it, the hardening audit rows (the pair fails
if mis-edited; the audit fails only if nobody runs it) and the ready flip's refusal of a head
inside the claim prefix.

facts:
- Shape: the spec's §9, GitHub paragraph. Separate from the default-branch ruleset.
- Brief 20 records which write routes the restrict rules cover and whether `refs/dispatch/*`
  is reachable by a ruleset. Read its results first; where one contradicts this brief, STOP.
- The template handles both claim prefixes (spec §10 F): an exclude for
  `refs/heads/dispatch/**`, and a note that `refs/dispatch/*` is outside branch/tag targeting.
- Applying a ruleset is a repository-admin act. No tool applies it.

## Human decision
When several separate computers dispatch work for one GitHub repository, dispatch claims have
to be kept on GitHub, and the reviewing identity then needs write access to record them; the
platform has no narrower permission. The proposal bounds that access with two platform rules:
only the identities meant to write code may create, update or delete any branch or tag, leaving
the reviewing identity able to write only where claims are recorded. The rules ship as a
template that an administrator applies by hand, and a separate audit reports if they are
missing or if the reviewing identity appears in the exception list. Cells on one computer, or
using the claim service, do not need any of this. The decision is whether this is the accepted
boundary for that one setup.

Options:
1. **Approve as specified.**
2. **Approve branches only** — leave tags to release-tag protection.
3. **Reject** — that setup keeps the reviewer's write access unbounded, with a standing notice.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- Fixture rows run against a throwaway repository you created. NEVER apply, edit or test a ruleset on this repository.
- Do not weaken or edit any existing ruleset template or audit row.

## Task
1. The two templates and the README (placeholder bypass ids; who applies; how to check).
2. Audit rows: pair present and active; the three restrict rules present; reviewer App absent
   from bypass (could-not-check when bypass is not visible).
3. `deskflip`: refuse a change whose head branch is inside the claim prefix.
4. Run the fixture rows; quote the forge's refusal text in the PR body.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check:ci | `python3 -c "import json,sys; [json.load(open(p)) for p in sys.argv[1:]]" docs/workflow-templates/rulesets/writers-only-branches.json docs/workflow-templates/rulesets/writers-only-tags.json` | exit 0 |
| 3 | check +flow | **FIXTURE ONLY.** Pair applied, reviewer App not in bypass. Reviewer credential, plain git, no desk tool: `git push origin HEAD:refs/heads/probe-branch` | REFUSED by the forge, quoting the forge's rule-violation text (spec V7) |
| 4 | check +flow | **FIXTURE ONLY.** Same credential, store `forge-ref`: `deskclaim-ref acquire fixture--issue-1 --repo <fixture> --token-file <reviewer token file>` then the same with `release` | exit 0 twice — the claim succeeds under the credential the push was refused for (spec V7) |
| 5 | check | **FIXTURE ONLY.** Same credential: push an update to the head branch of an open change | REFUSED by the forge |
| 6 | check | **FIXTURE ONLY.** A worker-role credential that IS in bypass: `git push origin HEAD:refs/heads/probe-writer` | accepted — the positive control |
| 7 | check:ci | `cd tools/desk && go test ./cmd/repohardenguard/ -run 'TestWritersOnlyPairRows' -count=1 -timeout 120s -v` | exit 0; pair absent → checked-wrong; reviewer in bypass → checked-wrong; bypass not visible → could-not-check |
| 8 | check:ci +mutation | the mutation entry named `hardening-reviewer-in-bypass-reads-ok` | row 7's reviewer-in-bypass subtest goes RED |
| 9 | check:ci | `cd tools/desk && go test ./cmd/deskflip/ -run 'TestFlipRefusesHeadInsideClaimPrefix' -count=1 -timeout 120s` | exit 0 |
| 10 | check +dereference | Open the forge's ruleset availability statement and compare it with the sentence in `docs/workflow-templates/rulesets/README.md` (planned) | the sentence matches the page as read on the Evidence date, or says could-not-check |
| 11 | check | `(cd statusgen && go build -o /tmp/statusgen-fn26 .) && /tmp/statusgen-fn26 --root . --consumers --brief forge-neutral/26` | exit 0 |

Rows 3–6 carry `<fixture>` and `<reviewer token file>` on purpose: they name the runner's own
throwaway repository and credential file, which must not be written into this document.

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| The refusal in row 3 comes from a desk guard, not the forge | row 3 uses plain git and requires the forge's text |
| The rule blocks claim release, leaking claims | row 4 |
| The rule blocks the writers too | row 6 |
| A write route other than git push is not covered | row 5 plus brief 20's results; an uncovered route is stated in the README |
| The reviewer App is added to bypass later | rows 7, 8 |
| Plan availability asserted from memory | row 10 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; an access-control boundary). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between the reviewer credential and a branch head under forge-ref? (The ruleset pair; beneath it the audit rows and the flip-time prefix refusal.)
2. Does any row prove the lower layer with the upper bypassed? (Rows 3 and 5: no desk tool is in the path; the forge refuses.)
