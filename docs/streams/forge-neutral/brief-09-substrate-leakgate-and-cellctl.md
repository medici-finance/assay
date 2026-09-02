---
brief: forge-neutral/09
title: Substrate — the leak gate's verdict on merge requests, and cellctl's forge-aware new/up
why: >-
  Two pieces of the substrate around the verbs assume GitHub in a way no verb migration
  reaches. The leak gate's verdict is a GitHub commit status, so on a GitLab merge request
  there is no place for it to land and no gate at all — and the pilot found the free-tier
  compensator (a leak sweep in CI) equally absent. `cellctl` requires a GitHub App PEM as a
  mandatory flag and mints installation tokens against a hardcoded host, so a cell cannot be
  stood up for a GitLab-only fleet. A verb that works on both forges inside a cell that only
  boots on one is not portable.
wave: 3
depends: ["forge-neutral/01", "forge-neutral/02"]
unblocks: ["forge-neutral/10"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v1
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolver and the per-forge custody binding cellctl must provision for"
  - "docs/streams/forge-neutral/brief-02-forge-qualified-identity.md — the forge-qualified roster entries cellctl writes into a cell"
  - "docs/streams/forge-gitlab/pilot-report.md §3 row 8 — secret push protection is failed-at-tier AND the free-tier compensator (a leak sweep in CI) was absent; §3 rows 4 and 5 — no pipeline and no ci-config project existed"
  - "freshness-checked 2026-09-02 @ deae247 — cellctl:285,287-288 make --deskd-app-pem and --orgs required on `new`; :134-138 sign an RS256 App JWT; :140,147-150,152 call api.github.com; :293 symlinks the operator's gh config; leaksweep-pattern.yml:1-27 states the strong control half runs privately and posts the `leak-sweep` commit status; no .gitlab-ci.yml exists in the repo"
exec-tier: strong
exec-tier-why: "cellctl provisions and holds role credentials, and the leak gate is a disclosure control; a subtle error in either (a cell that boots with the wrong custody, a gate whose verdict lands nowhere and reads as absent-therefore-fine) survives every functional test (question c)."
gate-why: >-
  Both halves are controls, and both fail in the quiet direction. `cellctl` decides what
  credential material a cell holds and where; getting the GitLab custody shape wrong means a
  cell standing up with a credential nobody scoped. The leak gate's verdict is a merge
  blocker: if the GitLab expression of it is advisory rather than blocking, or if its absence
  is indistinguishable from a pass, a change can land without the gate ever having run. The
  human is confirming the per-forge custody shape cellctl provisions, and that the GitLab
  gate's absence reads as could-not-check — never as clean.
domain: complicated
consumers:
  - "tools/cellctl/cellctl: fixed-here"
  - "docs/cellctl.md: fixed-here (the verb table and the custody hand-steps gain their GitLab shape)"
  - "docs/streams/forge-neutral/leak-gate-shape.md: fixed-here (the per-forge gate design and its three-state contract)"
  - "docs/adopting-assay-gitlab.md: fixed-here (the adopter runbook gains the CI leak-sweep half the pilot found missing)"
  - "the private control-based sweep that posts the verdict: out-of-scope (it is house-side publication infrastructure, absent from this tree by design — this brief specifies the VERDICT SURFACE it must post to on a merge request, not the sweep)"
  - "plugins/assay/skills/install/SKILL.md: follow-up forge-neutral/11 (the install prose names the optional CLI per forge; cellctl's own prerequisites are fixed here)"
---

# Brief 09 — Substrate: leak gate and cellctl

## Context
files:
- `tools/cellctl/cellctl` — `new`, `check`, `deskd` and `up`.
- `docs/cellctl.md` — the verb table and the four custody hand-steps.
- `docs/streams/forge-neutral/leak-gate-shape.md` (planned) — the per-forge verdict surface
  and its three-state contract.
- `docs/adopting-assay-gitlab.md` — the adopter runbook.

single-point-of-failure: for the leak gate the single control is the verdict surface — if
nothing can post a blocking verdict on a merge request, the gate does not exist there. Two
independent layers are required and specified below: the gate's own verdict (blocking where
the forge can express it), and a pipeline-side sweep job that fails the change's own pipeline
— so a change is caught either by the external verdict or by its own CI, on different signals
in different components. For `cellctl` the single control is the custody hand-steps it
deliberately leaves to a human; the second layer is `check`, which independently re-reads each
precondition rather than trusting that `new` performed it.

facts:
- `cellctl new` requires `--repo --cells-yaml --orgs --deskd-app-pem`
  (`tools/cellctl/cellctl:285,287-288`) and writes `DESKD_APP_PEM`, `DESKD_APP_ID_VAR` and
  `ORGS` into `cell.env` (`:303-305`); it symlinks the operator's `gh` CLI config into the
  cell home (`:293`) and scaffolds a README naming the per-role App ids and the roster keys
  (`:318-326`).
- `cellctl deskd` signs an RS256 App JWT from the PEM (`:134-138`), then calls
  `https://api.github.com/app` (`:140`), `…/app/installations` (`:147-150`) and
  `POST …/app/installations/<id>/access_tokens` (`:152`), exporting
  `DESKD_GITHUB_TOKEN_<ORG>` (`:153-154`). It refuses when the App has no installation on an
  org (`:151`).
- `cellctl check` asserts the App-key symlinks resolve (`:105`), that the `gh` config is
  linked (`:106`) and that the App key is readable (`:109`). `up` stands `deskd` in a tmux
  window under an attended affirmation (`:237-239`) and prints why it did not otherwise
  (`:242-245`). `ls`, `desk` and `down` are not App-coupled.
- The public leak gate is two workflows. `leaksweep-pattern.yml` is the public pattern half;
  its header states the strong control-based sweep *"needs the private withheld-token map …
  so it CANNOT run here. It runs privately, against this repo's PR heads, and posts its
  verdict back as the `leak-sweep` commit status."* (`:1-27`).
  `leaksweep-control.yml:87-91` runs the in-tree disclosure controls.
- On the pilot the compensating free-tier layer was absent too: no `.gitlab-ci.yml`, no
  pipelines, and `secret_push_protection_enabled: false`
  (`docs/streams/forge-gitlab/pilot-report.md` §3 row 8). The remediation tier for the
  compensator is `free` — it needs no licence, only building.
- GitLab's merge-request equivalent of a commit status is a commit status on the MR's head
  pipeline; the blocking form (an external status check) is tier-gated and returned `HTTP 401`
  on the pilot (`pilot-report.md` §3 row 4). So on CE the gate must be expressed as a pipeline
  job that fails, not only as an external verdict.
- `tools/leaksweep` is absent from this tree by design; `deskpreflight` treats its absence as
  `present=false` rather than could-not-check, and says so
  (`tools/desk/cmd/deskpreflight/main.go:701-713`).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not weaken an existing control to make the two forges symmetric. If GitLab CE cannot
  express a blocking external verdict, say so and add the pipeline-side layer — do not make
  the GitHub side advisory to match.
- No token value may be printed, passed in an argv, or written inside a checkout. `cellctl`'s
  existing custody discipline applies unchanged to whatever GitLab path is added.

## Task
1. **Specify the gate first.** Write `leak-gate-shape.md`: for each forge, the verdict surface
   (GitHub commit status; GitLab head-pipeline commit status plus, where the tier allows, an
   external status check), whether it blocks, and — the load-bearing part — the three-state
   contract: gate ran and passed, gate ran and failed, gate could not run. A missing verdict
   must be readable as could-not-check by whatever decides a change is mergeable, never as a
   pass.
2. **The pipeline-side layer.** Specify (and, for this repo's own GitLab-facing adopters,
   template into `forge-neutral/08`'s `.gitlab-ci.yml` half) a sweep job that runs in the
   change's own pipeline and fails it. This is the free-tier compensator the pilot found
   missing, and it is the layer that catches a change when the external verdict is
   unavailable.
3. **`cellctl new` becomes forge-aware.** Take the cell's forge explicitly; require the App
   PEM and orgs only on the GitHub path, and on the GitLab path require whatever the per-forge
   custody binding needs instead (the group and the role token store). `--deskd-app-pem` stops
   being unconditionally required. The custody hand-steps stay hand-steps: `new` must not
   acquire credentials on the adopter's behalf on either forge.
4. **`cellctl deskd` and `check` follow.** `deskd` mints per-forge: the App JWT exchange on
   GitHub, the role token store on GitLab, with the hardcoded host replaced by the cell's
   configured forge endpoint. `check` re-reads each precondition for the cell's forge and
   reports per-precondition ok/MISS as it does today — including MISS for a GitHub
   precondition on a GitLab cell, rather than silently skipping it.
5. **Roster entries.** The README `new` scaffolds writes forge-qualified roster entries per
   `forge-neutral/02`'s grammar, so a cell stood up for GitLab does not produce a roster the
   verbs will refuse.
6. **Runbook.** Add the CI leak-sweep half to `docs/adopting-assay-gitlab.md`, cross-linked to
   `leak-gate-shape.md`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `bash -n tools/cellctl/cellctl` | exit 0 — the script parses |
| 2 | `grep -c '^[\|] ' docs/streams/forge-neutral/leak-gate-shape.md` | ≥ 4 — the per-forge verdict-surface table and the three-state contract table are present as tables |
| 3 | `tools/cellctl/cellctl new --help 2>&1 \| grep -c 'deskd-app-pem'` | ≥ 1, and the help text marks it as required on the GitHub path only — read as text, since the flag must still exist |
| 4 | `tools/cellctl/cellctl new --forge gitlab --repo example/tracking --cells-yaml /tmp/cells.yaml 2>&1; echo $?` | **negative path**: exits non-zero naming the GitLab custody inputs it needs, and does NOT demand an App PEM; the row fails if the GitHub-only requirement still fires |
| 5 | `tools/cellctl/cellctl new --forge github --repo example/tracking --cells-yaml /tmp/cells.yaml --orgs example-org 2>&1; echo $?` | **negative path**: still exits non-zero without `--deskd-app-pem` — the existing requirement survives on the forge where it applies |
| 6 | `tools/cellctl/cellctl check --cell` — run against a cell whose configured forge is GitLab (manual: the verifier names the cell) | reports per-precondition ok/MISS for the GitLab preconditions; a GitHub-only precondition appears as MISS or as not-applicable, never as silently absent — read as text |
| 7 | `grep -rn -e 'api.github.com' tools/cellctl/cellctl \| wc -l` | prints `0` — the host comes from the cell's configured forge endpoint, not a literal |
| 8 | `grep -c 'gitlab' docs/cellctl.md` | ≥ 3 — the verb table and the custody hand-steps carry the GitLab shape |
| 9 | `grep -c 'leak' docs/adopting-assay-gitlab.md` | ≥ 1 — the CI leak-sweep half the pilot found missing is in the runbook |
| 10 | `cd tools/desk && go test ./cmd/deskflip/... -run TestMissingLeakGateIsCouldNotCheck -count=1 -v` | **negative path**: a change whose leak-gate verdict is ABSENT is treated as could-not-check by the ready-flip decision, not as a pass; the row fails if absence is silently tolerated |
| 11 | `grep -c 'leaksweep' docs/streams/forge-neutral/gitlab-ci-half.md` | ≥ 1 — the pipeline-side sweep job is part of the GitLab CI half `forge-neutral/08` templates, not a separate thing an adopter must remember |
| 12 | `statusgen --root . --consumers --brief forge-neutral/09` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| On GitLab the leak gate's verdict is absent and absence reads as "no objection", so a change lands ungated | row 10, asserting could-not-check on an ABSENT verdict |
| The GitHub gate is made advisory so the two forges look symmetric | row 10 covers the decision path for both; the Ground rules forbid it and the human gate reads for it |
| Only the external verdict is specified, so on CE (where the blocking form is tier-gated) there is no layer at all | rows 2 + 11 — the pipeline-side job must exist in the templated CI half, not only in prose |
| `cellctl new` stops requiring the App PEM on BOTH forges, so a GitHub cell stands up with no App key | row 5 |
| `cellctl` acquires credentials on the adopter's behalf to make the GitLab path "easier", turning hand-steps into automated custody | **no row** — review-only. The hand-steps are a deliberate design; the Review gate reads the diff for any acquisition the script now performs |
| The hardcoded host survives in one branch of the script | row 7 |
| `check` silently skips preconditions that do not apply, so a half-provisioned cell reads clean | row 6, which requires per-precondition reporting rather than a pass/fail summary |
| A cell stood up for GitLab writes unqualified roster entries the verbs then refuse | row 6 plus `forge-neutral/02`'s parser refusal; the cell fails `check` rather than failing at first write |
| A token value reaches a log, an argv or a checkout | **no row here** — the existing custody discipline is unchanged and its checks are `deskpreflight`'s; the Review gate reads the diff for any new echo, argv or redirect |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). Reviewer records verdict + date in
the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between a withheld-content change and its landing on a GitLab
   merge request, and is that acceptable? (The verdict surface — and alone it is not, which is
   why task 2's pipeline-side job is required rather than optional.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER bypassed? (Row 10
   with the external verdict absent; row 11 with the pipeline job present as the layer that
   still fires.)
