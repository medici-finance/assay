---
brief: assay:assay:forge-neutral:09
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
design: DR-forge-neutral-09
issues: []
schema: brief-v2
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
  - "docs/streams/forge-neutral/gitlab-ci-half.md: fixed-here (the pipeline-side leak-sweep sweep job forge-neutral/08 templates — task 2, verify row 11)"
  - "docs/adopting-assay-gitlab.md: fixed-here (the adopter runbook gains the CI leak-sweep half the pilot found missing)"
  - "tools/desk/cmd/deskflip: fixed-here (the ready-flip decision reads an ABSENT required leak-gate verdict as could-not-check, never a pass — task 1's three-state contract, verify row 10)"
  - "the private control-based sweep that posts the verdict: out-of-scope (it is house-side publication infrastructure, absent from this tree by design — this brief specifies the VERDICT SURFACE it must post to on a merge request, not the sweep)"
  - "plugins/assay/skills/install/SKILL.md: follow-up forge-neutral/11 (the install prose names the optional CLI per forge; cellctl's own prerequisites are fixed here)"
version: 2
id: a7231cc4-611d-44e3-8a14-ce5a0d9fe3f0
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
| 1 | `bash -n tools/cellctl/testdata/cellctl-shell-oracle.sh` | exit 0 — the script parses |
| 2 | `grep -c '^[\|] ' docs/streams/forge-neutral/leak-gate-shape.md` | ≥ 4 — the per-forge verdict-surface table and the three-state contract table are present as tables |
| 3 | `tools/cellctl/testdata/cellctl-shell-oracle.sh new --help 2>&1 \| grep -c 'deskd-app-pem'` | ≥ 1, and the help text marks it as required on the GitHub path only — read as text, since the flag must still exist |
| 4 | `tools/cellctl/testdata/cellctl-shell-oracle.sh new --forge gitlab --repo example/tracking --cells-yaml /tmp/cells.yaml 2>&1; echo $?` | **negative path**: exits non-zero naming the GitLab custody inputs it needs, and does NOT demand an App PEM; the row fails if the GitHub-only requirement still fires |
| 5 | `tools/cellctl/testdata/cellctl-shell-oracle.sh new --forge github --repo example/tracking --cells-yaml /tmp/cells.yaml --orgs example-org 2>&1; echo $?` | **negative path**: still exits non-zero without `--deskd-app-pem` — the existing requirement survives on the forge where it applies |
| 6 | `tools/cellctl/testdata/cellctl-shell-oracle.sh check --cell` — run against a cell whose configured forge is GitLab (manual: the verifier names the cell) | reports per-precondition ok/MISS for the GitLab preconditions; a GitHub-only precondition appears as MISS or as not-applicable, never as silently absent — read as text |
| 7 | `{ grep -rn -e 'api.github.com' tools/cellctl/testdata/cellctl-shell-oracle.sh \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0` — the host comes from the cell's configured forge endpoint, not a literal. Re-written 2026-10-03 (#1862): every grep stage tolerates only the no-match status, so a missing path or a grep error fails the row instead of passing it. |
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
### Verify run — 2026-09-11, non-implementer dispatched verifier (opus-4.8[1m]-verifier, local) — gate: human, HELD at `implemented`

Target: merged `origin/main` @ `fc9001a7ab48ee9c859dd7e52f7543dec5f86c50` (two-protocol confirmed; landing commit `39ec71f0`, PR #821). Offline (`KUBECONFIG=/dev/null`) in an isolated worktree; runner ≠ implementer. gate: human + sensitive-data — table RUN for Evidence; no model sign-off.

| # | Command | Exit | Key observed output | Result |
|---|---------|------|---------------------|--------|
| 1 | `bash -n tools/cellctl/cellctl` | 0 | script parses | PASS |
| 2 | `grep -c '^[|] ' docs/streams/forge-neutral/leak-gate-shape.md` | 0 | `8` (verdict-surface + three-state tables both present) | PASS |
| 3 | `cellctl new --help \| grep -c 'deskd-app-pem'` | — | `2`; help: "`--deskd-app-pem` REQUIRED on the github path ONLY … gitlab: NO App PEM and NO --orgs" | PASS |
| 4 (neg) | `cellctl new --forge gitlab --repo … --cells-yaml …` | 3 | "on the gitlab path --group is required … the role token store (`gitlab-<role>.token`) is a custody hand step, NOT an App PEM" — names GitLab custody inputs, does NOT demand an App PEM | PASS |
| 5 (neg) | `cellctl new --forge github … --orgs example-org` (no pem) | 3 | "on the github path --orgs and --deskd-app-pem are required" — requirement survives on GitHub | PASS |
| 6 | `cellctl check --cell` on a scaffolded GitLab cell (`glcell`, forge=gitlab) | 1 | per-precondition ok/MISS for GitLab (group ok, token store ok, deskd token MISS); GitHub-only preconditions reported `n/a … not applicable on a gitlab cell` — never silently absent | PASS |
| 7 | `grep -rn 'api.github.com' tools/cellctl/cellctl \| wc -l` | — | `0` — host from the configured forge endpoint | PASS |
| 8 | `grep -c 'gitlab' docs/cellctl.md` | — | `19` (≥3) | PASS |
| 9 | `grep -c 'leak' docs/adopting-assay-gitlab.md` | — | `8` (≥1) | PASS |
| 10 (neg, crux) | `cd tools/desk && go test ./cmd/deskflip/... -run TestMissingLeakGateIsCouldNotCheck -count=1 -v` | 0 | PASS — absent `leak-sweep` required verdict → `deskkit.ExitUnverifiable`, no mutation, required-checks endpoint actually read; "an absent required verdict is could-not-check, never a pass". Companion `TestPresentLeakGateFlips` confirms a present verdict still flips | PASS |
| 11 | `grep -c 'leaksweep' docs/streams/forge-neutral/gitlab-ci-half.md` | — | `4` (≥1) | PASS |
| 12 | `statusgen --root . --consumers --brief forge-neutral/09` | — | COULD-NOT-CHECK — `no brief-v1 file` (statusgen v1.0.6 brief-v2 gap). Manual: landing commit `39ec71f0` touches every consumers path (`tools/cellctl/cellctl`+239, `docs/cellctl.md`, `leak-gate-shape.md` new, `docs/adopting-assay-gitlab.md`, `gitlab-ci-half.md`, `tools/desk/cmd/deskflip/{flip.go,deskflip_test.go}`) | COULD-NOT-CHECK |

**Risk-bearing value (sensitive-data: yes — ENUMERATE → RANK → DERIVE):**
- Leak-gate verdict states: three-state — ran-and-passed (clear) / ran-and-failed (blocked) / could-not-run (absent → could-not-check); could-not-check maps to `deskkit.ExitUnverifiable = 6` (`tools/desk/internal/deskkit/exitcodes.go:31`).
- Per-forge custody inputs: GitHub → `--deskd-app-pem` + `--orgs` (App mints per-org installation tokens); GitLab → `--group` + role token store `gitlab-<role>.token` (hand-provisioned, never minted).
- `RISK-VALUE: DERIVED (top-ranked) — absent leak-sweep verdict ⇒ could-not-check ⇒ ExitUnverifiable(6) ⇒ ready-flip refuses (no mutation).` A missing verdict silently rounded to a pass would let withheld content land on a public MR (irreversible disclosure). Derived + proven by row 10 + `flip.go` + `leak-gate-shape.md §three-state`. Custody inputs rank below (mis-provision caught by `check` before first write). No `NAMED, NOT DERIVED` value open.

**Sensitive-data defense (gate: human) — two independent layers:** `deskflip` does not trust the green rollup alone — it reads the branch-protection required-status-check set and cross-checks that every required context (incl. `leak-sweep`) actually reported at the head; an absent required verdict returns `ExitUnverifiable` and blocks the flip, distinct from both pass and fail. Layer 2: the pipeline-side sweep job fails the change's own CI — on GitLab CE, where the blocking external check is tier-gated, the pipeline job IS the merge blocker. Single control (verdict surface) is explicitly not sufficient, hence the mandatory pipeline layer; row 10 proves the flip catches the fault with the external verdict absent.

**Scope-traceability:** all work maps to a Verify row (or review-only pre-mortem lines); `flip.go` +45 = the row-10 could-not-check decision, `deskflip_test.go` +57 = its test. Custody discipline preserved: no token minted on the GitLab path (hand-provisioned store, mode-0600 read; "rotation is a hand step"); no credential acquisition added.

**VERDICT: PASS** on rows 1–11; row 12 COULD-NOT-CHECK (statusgen v1.0.6 brief-v2 gap, consumers hand-corroborated) — **HELD at `implemented` (human sign-off owed via the verify-gate).** The human confirms the per-forge custody shape and that absence reads as could-not-check (both proven above).

### Re-verify run — 2026-09-27, non-implementer dispatched verifier (opus-5.5[1m]-verifier, local) — gate: human, HELD at `implemented`

Target: merged main @ `d034d39fe1c8f0a0a93ba1177f208c48b068a1d0`, confirmed against the forge with the verifier App token. Implementing commit `39ec71f0` (PR #821); the Verify rows were re-pointed at the relocated shell oracle by `b227b407` (PR #1739), which is why this re-verify ran. Witness: the pinned statusgen v1.0.27 binary (sha256 matches the pin) invoked directly, not through a shim, inside a network-denied sandbox (profile: deny network, loopback-only allowance for the row-10 httptest server), with GOFLAGS=-count=1, GOPROXY=off, KUBECONFIG=/dev/null and CELLS_ROOT pointed at a scratch directory outside the checkout. Runner is not the implementer. gate: human + sensitive-data, so the table is RUN for Evidence and no model signs off.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `bash -n tools/cellctl/testdata/cellctl-shell-oracle.sh` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c '^[\|] ' docs/streams/forge-neutral/leak-gate-shape.md` | pass exit=0 | sha256:aa67a169b0bb | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `tools/cellctl/testdata/cellctl-shell-oracle.sh new --help 2>&1 \| grep -c 'deskd-app-pem'` | pass exit=0 | sha256:53c234e5e847 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `tools/cellctl/testdata/cellctl-shell-oracle.sh new --forge gitlab --repo example/tracking --cells-yaml /tmp/cells.yaml 2>&1; echo $?` | pass exit=0 | sha256:0376abaaac03 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `tools/cellctl/testdata/cellctl-shell-oracle.sh new --forge github --repo example/tracking --cells-yaml /tmp/cells.yaml --orgs example-org 2>&1; echo $?` | pass exit=0 | sha256:9900f39c8601 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `tools/cellctl/testdata/cellctl-shell-oracle.sh check --cell` | fail exit=3 | sha256:4f2432e67465 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -rn -e 'api.github.com' tools/cellctl/testdata/cellctl-shell-oracle.sh \| wc -l` | fail exit=1 | sha256:4eff2db4bada | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c 'gitlab' docs/cellctl.md` | pass exit=0 | sha256:537879630753 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c 'leak' docs/adopting-assay-gitlab.md` | pass exit=0 | sha256:25d4f2a86deb | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskflip/... -run TestMissingLeakGateIsCouldNotCheck -count=1 -v` | pass exit=0 | sha256:82988fead221 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -c 'leaksweep' docs/streams/forge-neutral/gitlab-ci-half.md` | pass exit=0 | sha256:7de1555df0c2 | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --root . --consumers --brief forge-neutral/09` | fail exit=2 | sha256:02cac87dac8a | 2026-09-27 | assay-verifier-app[bot] @ d034d39fe1c8 (on-behalf-of human:ian) (forge-identity) |

**Per-row notes.** These are real outputs. Where a row's output is deterministic, its witness hash was decoded by re-running the row and hashing the output.
- Row 1: exit 0, no output (the hash is sha256 of empty). PASS.
- Row 2: `8` (≥4). PASS.
- Row 3: `2`. The help text reads "--deskd-app-pem is REQUIRED on the github path ONLY" and "gitlab: … NO App PEM and NO --orgs". PASS (text read).
- Row 4 (neg): the witness `pass exit=0` comes from the `; echo $?` tail, so it is vacuous as an exit signal. The output hash decodes to the oracle's refusal "cellctl: new: on the gitlab path --group is required (the GitLab group this cell reads); the role token store (gitlab-<role>.token) is a custody hand step, NOT an App PEM", followed by `3`. It names the GitLab custody input and does not demand an App PEM. Behaviour PASS; the witness verdict is vacuous (check-definition).
- Row 5 (neg): same shape. The hash decodes to "cellctl: new: on the github path --orgs and --deskd-app-pem are required (the App mints per-org installation tokens)", followed by `3`. Behaviour PASS; the witness verdict is vacuous (check-definition).
- Row 6: witness `fail exit=3`. The command cell as written carries no cell name, so the oracle answers "no cell '--cell' under <scratch cells root> (cell.env missing)" (hash decoded). This is check-definition, not a defect. I then ran it by hand as the Expect directs. I scaffolded a GitLab cell `glcell` in the scratch cells root (`new glcell --forge gitlab --repo <scratch git repo> --cells-yaml <scratch file> --group example-group`, exit 0) and ran `check glcell`: exit 1, with per-precondition rows `ok GitLab group`, `ok GitLab role token store`, `MISS GitLab cell token` (none placed), `MISS GitLab fetch transport reachable` (network denied), `n/a GitHub App PEM — not applicable on a gitlab cell` and `n/a gh config link — not applicable on a gitlab cell`. With a placeholder token file and a stray `DESKD_APP_PEM` added to the scratch cell.env, the rows become `ok GitLab cell token …` and `MISS stray GitHub App PEM on a gitlab cell`. GitHub-only preconditions show as n/a or MISS and never go missing. Behaviour PASS (manual).
- Row 7: witness `fail exit=1`. The output hash decodes to `0` (padded by wc -l). The exit 1 is grep's no-match status, which the witness shell's pipefail carries through. The expected output `0` is what printed. Behaviour PASS; check-definition (the row needs a form that does not fail under pipefail).
- Row 8: `20` (≥3). PASS.
- Row 9: `11` (≥1). PASS.
- Row 10 (neg, crux): exit 0 inside the sandbox, `--- PASS`, with the refusal line "the rollup at aaaaaaaa is green but … requires 1 status check(s) that did not report on this head at all (leak-sweep) — an absent required verdict is could-not-check, never a pass". Targeted package tests (`-run '^(…)$' -timeout 300s`) also pass the companion present-verdict test. Mutation probe, run in this worktree and then restored: making `missingRequiredChecks` return nil turns the row-10 test red with "absent leak-gate verdict rc = 0, want 6 (could-not-check, never a pass)". Row 10 does test the behaviour. PASS.
- Row 11: `4` (≥1). PASS as written; see open question Q3.
- Row 12: witness `fail exit=2`. The hash decodes to "statusgen: --consumers: COULD-NOT-CHECK: assay:assay:forge-neutral:09 is not in the diff against d034d39…", because merged main carries no diff for the brief. COULD-NOT-CHECK, tracked by #1281. As a supplementary check, I checked out `39ec71f0` and ran the command with `--base` set to its parent: 2 CORROBORATED (gitlab-ci-half.md and tools/desk/cmd/deskflip), 0 disproved, 6 UNCHECKED ("this branch did not make this claim", because those consumers lines predate the landing PR). By hand, the landing commit's stat touches every fixed-here site: the cellctl shell script (+239, now at the relocated oracle path), docs/cellctl.md, leak-gate-shape.md (new), gitlab-ci-half.md (new), docs/adopting-assay-gitlab.md, and deskflip flip.go and deskflip_test.go.

**Supplementary check (not a Verify row): the shipped Go launcher.** I re-ran rows 3, 4, 5 and 7 and the row-6 check against a local build of tools/desk/cmd/cellctl. The help count is `2`. The gitlab-path and github-path refusals match the oracle word for word, both exit 3. The `check glcell` rows match the oracle's. Non-test Go sources carry 0 api.github.com literals. The oracle and the port agree.

**Risk-bearing value (sensitive-data: yes — ENUMERATE → RANK → DERIVE).** I enumerated over the landing diff `39ec71f0` (the cellctl shell script, deskflip flip.go, leak-gate-shape.md, gitlab-ci-half.md, docs/cellctl.md, docs/adopting-assay-gitlab.md) plus the Deliverables. The oracle relocation `b227b407` changed no value.
1. `ExitUnverifiable = 6` @ tools/desk/internal/deskkit/exitcodes.go:55, reached via `deskkit.Unverifiable` at tools/desk/cmd/deskflip/flip.go:381. Also the empty-context skip `key != ""` @ flip.go:2031.
2. The required context `leak-sweep` @ docs/streams/forge-neutral/leak-gate-shape.md:33. The flip reads the required set live (flip.go:377) and carries no literal of its own.
3. `allow_failure: false` @ docs/streams/forge-neutral/gitlab-ci-half.md:48.
4. `leaksweep run --tree "$CI_PROJECT_DIR" --engines legacy`, with no token map, @ gitlab-ci-half.md:44. Rules `merge_request_event` and default branch @ gitlab-ci-half.md:38-39.
5. The token file `gitlab-deskd.token` and its "0600" check label @ tools/cellctl/testdata/cellctl-shell-oracle.sh:360, :1577, :3271.
6. `CELL_FORGE` default `github` @ oracle:350.
7. `GITHUB_HOST` default `github.com`, used as `https://api.$GITHUB_HOST` @ oracle:351, :354, :3201.
8. GitLab endpoint default `https://gitlab.com/api/v4` @ oracle:357, :3257.
9. App JWT window `iat now-30`, `exp now+540` @ oracle:1740. The window predates this item; the diff changed only its host.
10. `port=8787` and `DESKD_ADDR 127.0.0.1:8787` @ oracle:3141, :307; `die` exit 3 @ oracle:289.

Rank: 1–4 are top, because a wrong value lets withheld content land ungated and a disclosure cannot be undone. 5 is next (custody of a live credential). 6–9 are mid: a wrong endpoint or forge fails loudly at mint or check and is reversed by a cell.env edit. 10 is last (operational knobs, no derivation needed).

- RISK-VALUE: DERIVED — ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:55 (applied at flip.go:381) — an absent required verdict needs a third state distinct from pass and fail. 6 is the repo's single could-not-verify exit, so deskflip refuses without mutation. The row-10 mutation probe shows the test depends on it. The `key != ""` skip at flip.go:2031 drops only an empty context name, which no forge can require.
- RISK-VALUE: DERIVED — allow_failure = false @ docs/streams/forge-neutral/gitlab-ci-half.md:48 — GitLab treats a job with allow_failure true as a warning that does not fail the pipeline. false is the only setting under which a tripped sweep turns the pipeline red, and a red pipeline is the CE blocking layer.
- RISK-VALUE: NAMED, NOT DERIVED — required context `leak-sweep` @ docs/streams/forge-neutral/leak-gate-shape.md:33 — the flip's could-not-check fires only if main's live branch protection lists `leak-sweep` as required. That is forge configuration, not tree content, and this offline pass did not read it. Open question Q1.
- RISK-VALUE: NAMED, NOT DERIVED — `--engines legacy` with no token map @ docs/streams/forge-neutral/gitlab-ci-half.md:44 — what the pattern engine detects without a token map, and so whether a green job means anything, cannot be derived here. The leaksweep tool is absent from this tree by design and is not in the desk-tools release. Open question Q2.
- RISK-VALUE: NAMED, NOT DERIVED — token-file mode "0600" @ tools/cellctl/testdata/cellctl-shell-oracle.sh:1577 — the check row's label states 0600, but its predicate tests readability only. A mode-0644 placeholder token reported `ok GitLab cell token (0600, …)` on both the oracle and the Go port. Open question Q4.
- RISK-VALUE: DERIVED — CELL_FORGE default = github @ oracle:350 — a cell scaffolded before forge support is GitHub by construction, and `new` always writes CELL_FORGE=gitlab for a GitLab cell (oracle:3263), so the default reaches only legacy cells.
- RISK-VALUE: DERIVED — GitLab endpoint = https://gitlab.com/api/v4 @ oracle:357, :3257 — this is GitLab.com's REST v4 base. Self-managed instances override it with --gitlab-api-base.
- RISK-VALUE: DERIVED — GitHub endpoint = https://api.$GITHUB_HOST, GITHUB_HOST default github.com, @ oracle:351, :354 — correct for github.com. Observation: on GitHub Enterprise Server the REST base is https://<host>/api/v3, not https://api.<host>. The script comment says GITHUB_HOST points a GHES cell elsewhere; that holds only through a direct FORGE_API_BASE override. The fix is a cell.env edit, and the mint fails loudly until then.
- RISK-VALUE: DERIVED — App JWT iat = now-30, exp = now+540 @ oracle:1740 — GitHub rejects an App JWT whose lifetime exceeds 10 minutes. The 570 s total stays under 600 s and keeps a 30 s backdate for clock skew.

**Open questions for the human gate (the NAMED, NOT DERIVED values above, verbatim):**
- Q1. Does main's live branch protection list `leak-sweep` as a required status check? deskflip's could-not-check (row 10) depends on it.
- Q2. With no token map, what does `leaksweep run --engines legacy` catch in an adopter's pipeline, and where does an adopter get the leaksweep binary? It is absent from this tree and from the desk-tools release, so the templated job fails closed (command not found) instead of sweeping.
- Q3. Row 11's Expect says the job is part of the GitLab CI half that the statusgen-init CI brief templates, "not a separate thing an adopter must remember", and gitlab-ci-half.md says it is "templated in alongside the rest of the CI half". At merged main, the GitLab template in `statusgen init` (statusgen/init.go:599, jobs statusgen-lint and statusgen-regen) has no leaksweep job, and runbook §4a tells the adopter to "Add the leak-sweep job to the shared `.gitlab-ci.yml` you commit". The grep passes, but the scaffold does not include the pipeline-side layer. GitLab CE has no per-job required setting, so the doc's claim that an absent leaksweep job reads as a missing required step rests on the job living in a protected ci-config project plus "pipelines must succeed". I could not derive that offline. The live GitLab conformance row for the leak gate is still NOT YET RUN (#1553).
- Q4. Should `check` assert mode 0600 on the GitLab token file, as the scrubbed kind already does for PEMs, instead of testing readability only?

**Sensitive-data defense (the Review's two questions).** (1) The single control on a GitLab MR: on CE it is the pipeline-side sweep job, which per Q2 and Q3 exists as a spec and a runbook hand step, not in the scaffold. On GitHub it is the required `leak-sweep` status plus deskflip's absence check. (2) A lower layer with the upper bypassed: row 10 proves the flip refuses when the external verdict is absent. No row proves the pipeline job fires, because no GitLab pipeline can be reached offline (#1553).

VERIFY: PASS — at merged main, behaviour on rows 1–11 matches every Expect: rows 4, 5 and 7 by decoded witness output, row 6 by the manual scratch-cell run. Row 12 is COULD-NOT-CHECK (#1281). The witness-held rows 4, 5, 6, 7 and 12 fail on how the check is written or on the tool, not on the code. HELD at `implemented` — gate: human, with Q1–Q4 open for the human.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `bash -n tools/cellctl/testdata/cellctl-shell-oracle.sh` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c '^[\|] ' docs/streams/forge-neutral/leak-gate-shape.md` | pass exit=0 | sha256:aa67a169b0bb | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 3 | `tools/cellctl/testdata/cellctl-shell-oracle.sh new --help 2>&1 \| grep -c 'deskd-app-pem'` | pass exit=0 | sha256:53c234e5e847 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 4 | `tools/cellctl/testdata/cellctl-shell-oracle.sh new --forge gitlab --repo example/tracking --cells-yaml /tmp/cells.yaml 2>&1; echo $?` | pass exit=0 | sha256:0376abaaac03 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 5 | `tools/cellctl/testdata/cellctl-shell-oracle.sh new --forge github --repo example/tracking --cells-yaml /tmp/cells.yaml --orgs example-org 2>&1; echo $?` | pass exit=0 | sha256:9900f39c8601 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 6 | `tools/cellctl/testdata/cellctl-shell-oracle.sh check --cell` | fail exit=3 | sha256:53c24ff9fa84 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -rn -e 'api.github.com' tools/cellctl/testdata/cellctl-shell-oracle.sh \| wc -l` | fail exit=1 | sha256:4eff2db4bada | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c 'gitlab' docs/cellctl.md` | pass exit=0 | sha256:537879630753 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -c 'leak' docs/adopting-assay-gitlab.md` | pass exit=0 | sha256:25d4f2a86deb | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 10 | `cd tools/desk && go test ./cmd/deskflip/... -run TestMissingLeakGateIsCouldNotCheck -count=1 -v` | pass exit=0 | sha256:cbb60bfdc10f | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -c 'leaksweep' docs/streams/forge-neutral/gitlab-ci-half.md` | pass exit=0 | sha256:7de1555df0c2 | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --root . --consumers --brief forge-neutral/09` | pass exit=0 | sha256:98f580cffebf | 2026-09-28 | assay-verifier-app[bot] @ e1afb99aca99 (on-behalf-of human:ian) (forge-identity) |

**Re-run 2026-09-28 at batch tree e1afb99a after main changed `tools/cellctl/testdata/cellctl-shell-oracle.sh` and `tools/desk/cmd/deskflip/flip.go`** — the witness table directly above. Tree: e1afb99aca990bbdb14ef44eb65fceb0610b18e8 = merged main 02a2f75fb532af2bf7bc6284e3f5f4c6e9337f48 (read with the verifier App token at run time; main had not moved) plus this batch's Evidence commit, clean stamp. Non-implementer dispatched verifier (claude-opus-5-5, local). Envelope: pinned statusgen v1.0.27 called directly (sha256 matches the darwin-arm64 pin); `sandbox-exec` network-denied with loopback-only allowance; `env -i` with the real HOME; PATH = system dirs plus a scratch dir holding only go and statusgen; `KUBECONFIG=/dev/null`, `GOFLAGS=-count=1`, a fresh empty GOCACHE, `GOPROXY=off` over a pre-populated module cache, `GOTOOLCHAIN=local` (go1.27.1); CELLS_ROOT a scratch directory outside the checkout. The oracle rows ran under `/bin/bash` 3.2.57 (the `bash` first on that PATH). An earlier attempt stamped `+dirty` (a module-cache pre-fetch had rewritten `tools/desk/go.sum`); that file and the brief were restored and the table above is the clean second run.

Drift since the 2026-09-27 run (d034d39f): 31d2ad5366266c16c33b38d8741ca868c6e58944 (PR #1650, issue #1631) changes only the oracle's `gen_shims` — the ambient gh credential now reaches a shimmed verb as `CELLCTL_GH_AMBIENT` and becomes `GH_TOKEN` only inside a cell `gh` wrapper, exported, never in an argv. `gen_shims` runs on the boot path, not in `new --help`, the two `new` refusals, `check`, or the host grep, and adds no `api.github.com` literal. It narrows credential exposure and does not touch this brief's custody inputs. a5bacb1237f8207051c7ff164cdde08a09dd565b (PR #1727) changes one line of flip.go, the model-floor stamp predicate in `checkModelFloor`; the leak-gate path (`missingRequiredChecks`, the `Unverifiable` return at flip.go:381-382, the `key != ""` skip at :2031) is byte-identical. Neither change touches this brief's behaviour.

Per row, vs the previous run:
- Rows 1, 2, 3, 4, 5, 7, 8, 9, 11: output hash identical to the previous run. Decoded again: row 2 `8`; row 3 `2`, help text still says `--deskd-app-pem` is REQUIRED on the github path ONLY and gitlab takes NO App PEM and NO --orgs; row 4 the gitlab refusal naming `--group` and the role token store, then `3`; row 5 the github refusal requiring `--orgs` and `--deskd-app-pem`, then `3`; row 7 `0` (exit 1 is grep no-match under pipefail); row 8 `20`; row 9 `11`; row 11 `4`. Rows 4 and 5 remain vacuous as witness exits (`; echo $?` tail) and row 7 remains a witness fail on a correct `0` — check-definition, unchanged.
- Row 6: hash changed only because the refusal names this run's scratch cells root ("no cell '--cell' under <scratch> (cell.env missing)", exit 3). The command cell's first code span carries no cell name, so the witness cannot run the intended check (#1805). Run by hand on a scratch GitLab cell scaffolded by the oracle: `ok GitLab group`, `ok GitLab role token store`, `MISS GitLab cell token`, `n/a GitHub App PEM` and `n/a gh config link` (not applicable on a gitlab cell); with a placeholder token and a stray `DESKD_APP_PEM` the rows become `ok GitLab cell token` and `MISS stray GitHub App PEM on a gitlab cell`. Same as before. The placeholder was mode 0644 and still read `ok … (0600, …)`, so open question Q4 stands.
- Row 10: pass exit 0; the hash differs run to run (timings and temp paths in `-v` output). The refusal line is unchanged: "requires 1 status check(s) that did not report on this head at all (leak-sweep) — an absent required verdict is could-not-check, never a pass". Targeted package tests (the row-10 test and its present-verdict companion, `-count=1 -timeout 300s`) pass. Mutation probe repeated at this tree: `missingRequiredChecks` returning nil turns row 10 red with "absent leak-gate verdict rc = 0, want 6"; flip.go restored with a path-specific checkout.
- Row 12: now `pass exit=0` where it was `fail exit=2`. The pass is vacuous: this batch's own Evidence commit puts the brief in the diff against main, so the check runs, and reports 0 corroborated, 0 disproved, 8 unchecked ("this branch did not make this claim"). Nothing is corroborated on merged main (#1281).
- No row is classed `check:ci`, so none records could-not-run on this darwin host (#1800 not engaged). The whole tools/desk module was not run; only the deskflip package tests above.

Risk values: unchanged. The oracle line numbers cited in the 2026-09-27 enumeration moved by +39 below the `gen_shims` hunk (token check label :1577 → :1616, JWT window :1740 → :1779, `port=8787` :3141 → :3180, GitHub endpoint :3201 → :3240, GitLab endpoint :3257 → :3296, `CELL_FORGE=gitlab` :3263 → :3302, token file :3271 → :3310); every literal is the same. Q1–Q4 stay open for the human.

VERIFY: PASS — at batch tree e1afb99a, rows 1–11 behave as their Expect says (rows 4, 5 and 7 by decoded output, row 6 by the manual scratch-cell run); row 12 is vacuous on merged main (#1281). Witness-held rows: 4, 5, 6, 7, 12. HELD at `implemented` — gate: human.

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
