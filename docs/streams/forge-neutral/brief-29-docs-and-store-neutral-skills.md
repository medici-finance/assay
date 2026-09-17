---
brief: assay:assay:forge-neutral:29
title: Adopter docs and store-neutral skills — supported topologies, store-aware grants, the GitLab Free tradeoff
why: >-
  The adopter guides tell every operator to give the reviewer repository write and explain at
  length why a narrower grant does not boot; no page says which topologies are supported; and
  four skill bodies name a forge claim namespace or a grant list. Once the tools change, that
  text is the reason adopters keep a grant they no longer need and the reason a skill reads
  wrong on a cell whose claims are not on the forge.
wave: 5
depends: ["forge-neutral/25", "forge-neutral/26", "forge-neutral/27", "forge-neutral/28"]
unblocks: ["forge-neutral/30"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1267]
schema: brief-v2
authored: 2026-09-17 by forge-neutral authoring session (issue 1267)
sources:
  - "#1267 — the problem statement, the driver's direction of 2026-09-17, and the required spec contents"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md — the scoping doc this brief implements; section numbers below refer to it"
  - "docs/adopting-assay.md:327 (App inventory, reviewer row) and :843-898 (`setup-reviewer-app`, *The required duty set*, the retired-recommendation note)"
  - "docs/adopting-assay-gitlab.md:60-78 and :108-121; docs/enforcement-model.md:13-28"
  - "docs/cellctl.md:3-6 and docs/docker.md:152-165 — the only topology statements today (spec §3.2)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md:655-661, pr-shepherd/SKILL.md:27-41, worker-desk/SKILL.md:140 and :346, the-desk/SKILL.md:163-167 — grant list, claim namespaces, `git ls-remote` claim reads"
  - "docs/streams/apps-installer/design.md — per-role manifests fix permissions at creation; the reviewer manifest text is stated here for that stream to consume"
  - "routed here from assay:assay:forge-neutral:22 (skill bodies), assay:assay:forge-neutral:25, assay:assay:forge-neutral:26, assay:assay:forge-neutral:27 and assay:assay:forge-neutral:28 (adopter docs)"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): six documents and four skills must agree with each other and with shipped tool output; well-formed and wrong is the failure this brief exists to avoid"
domain: complicated
consumers:
  - "docs/adopting-assay.md, docs/adopting-assay-gitlab.md, docs/enforcement-model.md: follow-up forge-neutral/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/cellctl.md, docs/docker.md, containers/README.md: follow-up forge-neutral/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "plugins/assay/skills/pr-review-desk, worker-desk, pr-shepherd, the-desk: follow-up forge-neutral/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "the public website's apps and adoption pages: out-of-scope (they mirror the adopter guides and live in the site's own repository; the companion change is tracked there)"
version: 1
id: ea8fee48-5128-402c-bedc-7add5748d825
---

# Brief 29 — Adopter docs and store-neutral skills

## Context
files:
- `docs/adopting-assay.md`, `docs/adopting-assay-gitlab.md`, `docs/enforcement-model.md`
- `docs/cellctl.md`, `docs/docker.md`, `containers/README.md`
- `plugins/assay/skills/pr-review-desk/SKILL.md`, `plugins/assay/skills/worker-desk/SKILL.md`,
  `plugins/assay/skills/pr-shepherd/SKILL.md`, `plugins/assay/skills/the-desk/SKILL.md`
- `changelog/<branch-slug>.md` (planned)

facts:
- Source of truth: the spec's §4 (stores, what each does not give), §6–§7 (guards, drain),
  §8 (duty table), §9 (forge-ref bound; the Free text, verbatim), §11 (order), plus the tool
  output shipped by briefs 21–28. Where a doc and the shipped tool disagree, the tool wins and
  the disagreement is reported.
- **Supported topologies are stated for the first time** (closes spec A7): one host → `file`;
  one host with several containers → `service`, or a claims-only volume where brief 20
  measured it clean; laptop plus cluster, or several hosts that can reach one service →
  `service`, with the placement rule; several machines sharing nothing but the forge →
  `forge-ref`, the only setup in which the reviewer keeps repository write.
- What each store does not give is stated beside it, in the spec's words.
- The narrowed grant is described as **available once the pinned release includes store-aware
  duties**. The grant change is the operator's act and the last step.
- The retired-recommendation note (`docs/adopting-assay.md:879-898`) is rewritten, not
  deleted: the earlier narrow recommendation failed because the tooling required the grant;
  the tooling has since changed.
- Skills: no store name, no claim namespace, no `git ls-remote` claim read, no grant list.
  Claims are read with the claim tool's `show` / `list`; the grant in force is read from the
  boot check. A skill body that reads differently per store is the defect.
- The reviewer's manifest permission set — repository read by default, write only for
  `forge-ref` — is stated for the Apps installer stream's manifest flow.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Do not start until `docs/streams/forge-neutral/reviewer-write-boundary.md` reads `**Status:** approved`.** Verify row 1 checks it. A
  `draft` spec means the questions in its §10 are still open; building on a default that is
  later ruled the other way is rework on the identity chain.
- The attribution-not-authorization wording in the enforcement-model document stands.
- State behaviour and boundaries: what each identity can write, and what refuses it. No misuse walkthroughs.

## Task
1. `adopting-assay.md`: topologies and stores; store-aware duty table; both manifest variants;
   rewrite the retired-recommendation note; the forge-ref bound with a link to the templates;
   order of operations with the grant change last.
2. `adopting-assay-gitlab.md`: tier table and reviewer row scoped to `forge-ref`; the §9 text
   verbatim; the profile key; the branch-namespace requirement.
3. `enforcement-model.md`: replace "one list, identically" with the store-aware statement;
   describe the write boundary as a layer beside the identity separation.
4. `cellctl.md`, `docker.md`, `containers/README.md`: the store per cell kind; the keys; the
   drain; the placement rule.
5. Skills: make the four bodies store-neutral. Run the skills linter.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check | `! grep -n -e 'refs/dispatch' -e 'refs/heads/dispatch' -e 'ls-remote' -e 'forge-ref' -e 'ASSAY_CLAIM_STORE' -e 'contents. write' -e 'contents. read' plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/the-desk/SKILL.md` | exit 0 and no line printed — spec V10 |
| 3 | check +dereference | `cd tools/desk && go build -o /tmp/deskpreflight-fn29 ./cmd/deskpreflight`, run it for the reviewer role against a fixture config home for each store, and compare each `claim store:` line with the line quoted in `docs/adopting-assay.md` | each quoted line matches the tool's output character for character |
| 4 | check +dereference | `sed -n '/^> ..On GitLab Free/,/^$/p' docs/streams/forge-neutral/reviewer-write-boundary.md > /tmp/fn29-spec.txt && sed -n '/^> ..On GitLab Free/,/^$/p' docs/adopting-assay-gitlab.md > /tmp/fn29-doc.txt && test -s /tmp/fn29-spec.txt && diff /tmp/fn29-spec.txt /tmp/fn29-doc.txt` | exit 0 — the extract is non-empty and identical |
| 5 | check | `! grep -q -F 'identically to *every* role App' docs/enforcement-model.md` | exit 0 — the superseded statement is gone |
| 6 | check +flow | `grep -n -i -e 'file store' -e 'claim service' -e 'forge-ref' docs/adopting-assay.md` | every store named has, within its own paragraph, the sentence on what it does not give |
| 7 | check +dereference | Follow the `docs/cellctl.md` new-cell steps on a clean machine account | the cell that results reports `claim store: file` at boot — the doc's steps produce what the doc says |
| 8 | check:ci | `cd tools/skillslint && go test ./... -count=1 -timeout 300s` | exit 0 |
| 9 | check | `(cd statusgen && go build -o /tmp/statusgen-fn29 .) && /tmp/statusgen-fn29 --root . --consumers --brief forge-neutral/29` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| A guide quotes output the tool does not print | row 3 |
| The Free tradeoff wording drifts from the approved text | row 4 |
| A skill still implies a store, a namespace or a grant | row 2 |
| A store is recommended without its limitation | row 6 |
| The guide tells operators to narrow before re-pinning | review; the order section is read against the spec's §11 |
| The two adopter guides contradict each other on the reviewer | review-only — no command compares prose across them |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter — all four risk answers no; documentation of behaviour
already shipped and human-gated in briefs 21–28). Reviewer records verdict + date in the
stream README table.
