---
brief: assay:assay:forge-neutral:29
title: Adopter docs and store-neutral skills — supported topologies, the reviewer at repository read, and the removal window
why: >-
  The adopter guides tell every operator to give the reviewer repository write and explain at
  length why a narrower grant does not boot; no page says which topologies are supported; and
  four skill bodies name a forge claim namespace or a grant list. Once the tools change, that
  text is the reason adopters keep a grant they no longer need, the reason an install misses
  the one release in which it must switch, and the reason a skill reads wrong on a cell whose
  claims are not on the forge.
wave: 6
depends: ["forge-neutral/25", "forge-neutral/28"]
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
  - "the rulings of 2026-09-17 recorded in the spec's §10 — this brief is written on them"
  - "docs/adopting-assay.md:327 (App inventory, reviewer row) and :843-898 (`setup-reviewer-app`, *The required duty set*, the retired-recommendation note)"
  - "docs/adopting-assay-gitlab.md:60-78 and :108-121; docs/enforcement-model.md:13-28"
  - "docs/cellctl.md:3-6 and docs/docker.md:152-165 — the only topology statements today (spec §3.2)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md:655-661, pr-shepherd/SKILL.md:27-41, worker-desk/SKILL.md:140 and :346, the-desk/SKILL.md:163-167 — grant list, claim namespaces, `git ls-remote` claim reads"
  - "docs/streams/apps-installer/design.md — per-role manifests fix permissions at creation; the reviewer manifest text is stated here for that stream to consume"
  - "routed here from assay:assay:forge-neutral:22 (skill bodies), assay:assay:forge-neutral:25 and assay:assay:forge-neutral:28 (adopter docs)"
  - "freshness-checked 2026-09-17 @ c67cc371 (origin/main)"
exec-tier: strong
exec-tier-why: "question (b): six documents and four skills must agree with each other and with shipped tool output; well-formed and wrong is the failure this brief exists to avoid"
domain: complicated
consumers:
  - "docs/adopting-assay.md, docs/adopting-assay-gitlab.md, docs/enforcement-model.md: follow-up forge-neutral/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/cellctl.md, docs/docker.md, containers/README.md: follow-up forge-neutral/29 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/UPGRADING.txt and the release-N notes (the removal window announced): follow-up forge-neutral/29 (this brief; flips to fixed-here when the implementation edits the path)"
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
- `docs/UPGRADING.txt`, `docs/release-notes/<release N>.md` (planned)
- `plugins/assay/skills/pr-review-desk/SKILL.md`, `plugins/assay/skills/worker-desk/SKILL.md`,
  `plugins/assay/skills/pr-shepherd/SKILL.md`, `plugins/assay/skills/the-desk/SKILL.md`
- `changelog/<branch-slug>.md` (planned)

facts:
- Source of truth: the spec's §4 (stores, what each does not give), §5–§7 (resolution, guards,
  the two-release migration), §8 (duty table), §9 (what an operator should know during the
  window), §11 (order), plus the tool output shipped by briefs 21–28. Where a doc and the
  shipped tool disagree, the tool wins and the disagreement is reported.
- **Supported topologies are stated for the first time** (closes spec A7): plain host
  processes on one machine → `file`; anything in a container or pod → `service`; a cell that
  spans hosts → `service`, with the placement rule. Several machines that share nothing but the
  forge and cannot reach one service: **unsupported after the removal release** — said in those
  words, with the placement rule as the remedy.
- What each store does not give is stated beside it, in the spec's words.
- **The removal window is announced**: what an unset key means in release N, the NOTICE an
  operator will see, the drain, and that release N+1 refuses an unset key.
- **The GitLab Free tradeoff text is removed, not documented.** The first draft specified an
  adopter-facing paragraph for a reviewer that keeps repository write on GitLab Free; with the
  forge store removed there is no such reviewer. The GitLab guide instead says the reviewer
  needs no repository write once the store key is set, on every tier, and its existing
  "reviewer that can approve but cannot push" degradation row is updated to match.
- The narrowed grant is described as **available once the pinned release includes store-aware
  duties and the cell's store key is set**. The grant change is the operator's act and comes
  after the switch.
- The retired-recommendation note (`docs/adopting-assay.md:879-898`) is rewritten, not
  deleted: the earlier narrow recommendation failed because the tooling required the grant;
  the tooling has since changed.
- Skills: no store name, no claim namespace, no `git ls-remote` claim read, no grant list.
  Claims are read with the claim tool's `show` / `list`; the grant in force is read from the
  boot check. A skill body that reads differently per store is the defect.
- The reviewer's manifest permission set — repository read — is stated for the Apps installer
  stream's manifest flow.

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
1. `adopting-assay.md`: topologies and stores; store-aware duty table; the reviewer manifest;
   rewrite the retired-recommendation note; the removal window; order of operations with the
   grant change after the switch.
2. `adopting-assay-gitlab.md`: reviewer row and the degradation row updated; no tradeoff
   paragraph added.
3. `enforcement-model.md`: replace "one list, identically" with the store-aware statement;
   describe the write boundary as a layer beside the identity separation.
4. `cellctl.md`, `docker.md`, `containers/README.md`: the store per cell kind; the keys; the
   drain; the placement rule; containers always `service`.
5. `UPGRADING.txt` and the release-N notes: the removal window.
6. Skills: make the four bodies store-neutral. Run the skills linter.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check | `grep -c -F '**Status:** approved' docs/streams/forge-neutral/reviewer-write-boundary.md` | `1` — the spec this brief implements is approved; `0` means STOP, do not start |
| 2 | check | `! grep -n -e 'refs/dispatch' -e 'refs/heads/dispatch' -e 'ls-remote' -e 'forge-ref' -e 'ASSAY_CLAIM_STORE' -e 'contents. write' -e 'contents. read' plugins/assay/skills/pr-review-desk/SKILL.md plugins/assay/skills/worker-desk/SKILL.md plugins/assay/skills/pr-shepherd/SKILL.md plugins/assay/skills/the-desk/SKILL.md` | exit 0 and no line printed — spec V10 |
| 3 | check +dereference | `cd tools/desk && go build -o /tmp/deskpreflight-fn29 ./cmd/deskpreflight`, run it for the reviewer role against a fixture config home for each store and for an unset key, and compare each `claim store:` line and the removal NOTICE with the lines quoted in `docs/adopting-assay.md` | each quoted line matches the tool's output character for character |
| 4 | check | `! grep -n -i -e 'On GitLab Free' -e 'can also push to' docs/adopting-assay-gitlab.md docs/adopting-assay.md` | exit 0 and no line printed — the withdrawn tradeoff text was not added |
| 5 | check | `! grep -q -F 'identically to *every* role App' docs/enforcement-model.md` | exit 0 — the superseded statement is gone |
| 6 | check +flow | `grep -n -i -e 'file store' -e 'claim service' docs/adopting-assay.md` | every store named has, within its own paragraph, the sentence on what it does not give |
| 7 | check +dereference | Follow the `docs/cellctl.md` new-cell steps on a clean machine account | the cell that results reports `claim store: file` at boot — the doc's steps produce what the doc says |
| 8 | check | `grep -n -i -e 'unset' -e 'ASSAY_CLAIM_STORE' docs/UPGRADING.txt` | the removal window is present: what unset means in N, and the refusal in N+1 |
| 9 | check:ci | `cd tools/skillslint && go test ./... -count=1 -timeout 300s` | exit 0 |
| 10 | check | `(cd statusgen && go build -o /tmp/statusgen-fn29 .) && /tmp/statusgen-fn29 --root . --consumers --brief forge-neutral/29` | exit 0 |

## Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| A guide quotes output the tool does not print | row 3 |
| The withdrawn GitLab Free paragraph is documented anyway | row 4 |
| A skill still implies a store, a namespace or a grant | row 2 |
| A store is recommended without its limitation | row 6 |
| An install never learns the old store ends, and stops at N+1 without warning | row 8, and brief 21's every-boot NOTICE in the other component |
| The guide tells operators to narrow before switching the store | review; the order section is read against the spec's §11 |
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
