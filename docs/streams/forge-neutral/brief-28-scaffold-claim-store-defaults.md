---
brief: assay:assay:forge-neutral:28
title: Scaffold defaults — a fresh single-host cell gets the file store and the declaration; existing cells are left alone
why: >-
  A new adopter running everything on one machine should never need to give the reviewer
  repository write, but they only get that if the scaffold chooses the file store for them.
  The same scaffold must never touch an existing cell, because an install that is quietly
  switched while another machine still dispatches is the double-dispatch this work exists to
  prevent.
wave: 4
depends: ["forge-neutral/22", "forge-neutral/23"]
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
  - "docs/cellctl.md:21-55 and 117-232 — what `cellctl new` writes for each cell kind"
  - "docs/docker.md:152-165 and containers/README.md:160 — per-desk volumes; no shared volume exists today"
  - "plugins/assay/skills/install/SKILL.md — the turnkey install that writes the roster for a cold adopter"
  - "routed here from assay:assay:forge-neutral:23 (scaffolds that write the keys) and assay:assay:forge-neutral:24 (launchers that export the service variables)"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): three scaffolds (host cell, container cell, turnkey install) must agree with the resolver's preconditions, and none may alter an existing cell"
gate-why: >-
  The scaffold writes the keys that decide where a new cell keeps its claims, including the
  single-host declaration made on the operator's behalf — claim custody. The human confirms
  the declaration is written only for cell kinds that are single-host by construction, is
  shown to the operator when written, and that no code path rewrites an existing cell's
  keys.
decision-trigger: start
domain: complicated
consumers:
  - "tools/cellctl/cellctl: follow-up forge-neutral/28 (this brief; flips to fixed-here when the implementation edits the path)"
  - "plugins/assay/skills/install/SKILL.md: follow-up forge-neutral/28 (this brief; flips to fixed-here when the implementation edits the path)"
  - "containers/ (launch surfaces): follow-up forge-neutral/28 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/cellctl.md, docs/docker.md, containers/README.md: follow-up forge-neutral/29"
version: 1
id: 0bf0c155-c89f-49ef-bcb1-4fb040e2a5db
---

# Brief 28 — Scaffold claim-store defaults

## Context
files:
- `tools/cellctl/cellctl` and `tools/cellctl/tests/` — `new`, `check`, `desk`/`up` exports.
- `plugins/assay/skills/install/SKILL.md` — the roster-writing step.
- `containers/` launch surfaces — the claim-store environment for container desks.
- `changelog/<branch-slug>.md` (planned)

single-point-of-failure: the scaffold's "new cell only" condition — behind it, the resolver's
mixed-store refusal (brief 23: a switched cell beside a live forge-claim dispatcher refuses at
dispatch) and `cellctl check`, which reports the store and declaration it finds without
changing them.

facts:
- Defaults (spec §10 A, H): host and house cells → `ASSAY_CLAIM_STORE=file`,
  `ASSAY_CLAIM_SINGLE_HOST=yes`, claims directory under the cell. Container cells → `service`
  until brief 20's shared-volume row is clean; then a claims-only volume is permitted. The
  volume never holds a working tree, so the per-desk working-volume property stands.
- A cluster-hosted cell → `service`; the scaffold prints the placement rule (spec §4.2).
- The scaffold prints what it wrote and what the declaration means, at creation time.
- An existing cell directory or roster is never modified: `new` already refuses an existing
  cell; this brief adds no migration verb. Moving a cell is the documented drain (spec §7),
  done by the operator.
- `cellctl check` gains a read-only block: resolved store, declaration present, claims
  directory permissions, service reachable — three-state.
- Launchers export `DESK_CLAIM_SERVICE` and `DESK_CLAIM_TOKEN_FILE` from cell configuration
  that is never committed.

## Human decision
When someone sets up a new cell on a single computer, the setup tool would choose to keep that
cell's dispatch claims in a folder on that computer and would record, on the operator's behalf,
that this is the only computer dispatching for its repositories. It shows the operator what it
wrote and what that statement means. With that choice the reviewing identity never needs write
access to the repository. Cells made of several containers or running on a cluster are pointed
at the claim service instead. Existing cells are never changed by the tool. The decision is
whether the setup tool may make that choice and that statement for new single-computer cells.

Options:
1. **Approve as specified.**
2. **Approve, but ask instead of assume** — the setup tool stops and asks the operator to
   confirm the single-computer statement; safer, one more step in a first install.
3. **Reject** — new cells keep claims on the platform unless the operator configures otherwise.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- No code path in this brief may edit an existing cell's `cell.env` or roster. If a test needs one, STOP and report.

## Task
1. `cellctl new`: write the defaults per cell kind; print them with the one-line meaning.
2. `cellctl check`: the read-only claim-store block.
3. Launchers: export the two service variables when configured.
4. Install skill: the same defaults in its roster step, same printed explanation; store-neutral
   wording elsewhere.
5. Container launch surfaces: the service variables; the claims-only volume documented as
   conditional on brief 20's row.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check:ci | `bash tools/cellctl/tests/claim-store-defaults.test.sh` (planned) | exit 0; a new host cell's roster carries `ASSAY_CLAIM_STORE=file` and the declaration; a new container cell's carries `service`; stdout shows the explanation |
| 3 | check:ci | the same test's existing-cell case | `new` against an existing cell refuses and the cell's files are byte-identical before and after |
| 4 | check:ci | the same test's `check` case | `check` reports the store block three-state and modifies nothing |
| 5 | check +flow +dereference | Scaffold a new host cell against a fixture repository with a reviewer grant of repository **read**; boot the review desk; dispatch one review | boot is clean with `claim store: file`; dispatch succeeds; no ref written to the fixture (spec V5 end to end from a fresh scaffold) |
| 6 | check | `grep -n -i -e 'ASSAY_CLAIM_STORE' -e 'single.host' plugins/assay/skills/install/SKILL.md` | hits only inside the roster-writing step |
| 7 | check | `(cd statusgen && go build -o /tmp/statusgen-fn28 .) && /tmp/statusgen-fn28 --root . --consumers --brief forge-neutral/28` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| An existing multi-machine cell is switched to `file` by a re-run of the scaffold | row 3; and brief 23's mixed-store refusal in the other component |
| The declaration is written for a cell kind that is not single-host | row 2's container case |
| The operator never sees what was declared for them | row 2 (stdout) and brief 25's every-boot notice |
| A fresh scaffold does not actually boot a read-only reviewer | row 5 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`; keys that decide where a new cell keeps its claims). Reviewer records
verdict + date in the stream README table.

Reviewer questions for an identity-chain brief, answered in the verdict:
1. What single control stands between the scaffold and an existing cell's store? (The new-cell-only condition; beneath it the mixed-store refusal and the read-only check.)
2. Does any row prove the lower layer with the upper bypassed? (Row 3 aims the scaffold at an existing cell; row 5 proves the default end to end with a credential that cannot write.)
