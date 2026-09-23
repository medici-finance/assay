---
brief: assay:assay:forge-gitlab:12
title: GitLab hardening reads — repohardenguard kinds on the GitLab backend
why: >-
  After forge-gitlab/11 the hardening guard reaches the forge through one enumerated operation
  under a read-only identity — but on a GitLab-resolved project every read kind is a named
  refusal, so the guard can authenticate on GitLab and check nothing. A GitLab desk needs the
  same instrument the GitHub desk has: a checklist of protected-branch, protected-tag, project
  and approval settings compared against live values with the three-state verdict, and an honest
  `not available — <tier>` row for every control Community Edition does not expose. This brief
  adds the GitLab kinds to the same operation, one fixed endpoint each, and the per-forge
  checklist rows that consume them.
wave: 5
depends: ["forge-gitlab/11"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-11 by forge-gitlab authoring session (custody design worker)
sources:
  - "docs/streams/forge-gitlab/brief-11-guard-read-custody.md (assay:assay:forge-gitlab:11) — op 38 `RepoHardeningRead`, the closed kind vocabulary, the `auditor` identity this brief reads under; its GitLab kinds are explicitly deferred here"
  - "docs/streams/forge-gitlab/spec.md §1 (the two disclosed CE degradations and no third without a ruling), §3 (parity per control, even where the mechanism differs), §6 (freeze rule)"
  - "docs/streams/forge-gitlab/edition-matrix.md — rows B1 (protected branches, Free), B2 (identity-level push allowlist, Premium), B3/B4 (approval rules and settings, Premium), B5/B6 (merge-gating project settings, Free), B12 (protected tags, Free role-level), C5 (push rules Premium / secret push protection Ultimate), C6 (custom CI config path, Free)"
  - "docs/streams/forge-gitlab/inventory.md — op 38's row (GitLab column `could-not-check-with-gap — forge-gitlab/12`) that this brief flips to implemented"
  - "tools/desk/cmd/repohardenguard/checklist.go — the row grammar (`Gated`, `Read`, `Field`, `Required` incl. `not available — <why>`) the GitLab rows reuse unchanged"
  - "freshness-checked 2026-09-11 @ 8953d38d — no GitLab hardening read exists on any backend; forge-gitlab/11 is authored, not implemented"
exec-tier: strong
exec-tier-why: "a plausible-but-wrong mapping fail-opens the instrument: a Premium-gated 403 rendered as a value, or a role-level allowlist read as an identity-level one, passes a checklist that the live project does not satisfy (question c); each kind's document shape must be judged against what CE actually returns (question b)."
domain: complicated
tier: free
consumers:
  - "tools/desk/internal/deskkit/forge_gitlab.go + forge_gitlab_test.go: follow-up forge-gitlab/12 (this brief — the GitLab kinds replace the named refusal; goldens per kind incl. the tier-gated 403 → could-not-check case)"
  - "tools/desk/internal/deskkit/forge.go (`HardeningReadKind` vocabulary): follow-up forge-gitlab/12 (this brief — the GitLab kinds are added to the closed set; the GitHub backend refuses them by name, symmetrically)"
  - "tools/desk/cmd/repohardenguard/repohardenguard_test.go: follow-up forge-gitlab/12 (this brief — a GitLab fixture block with a `not available — Premium` row and a tier-403 row)"
  - "docs/streams/forge-gitlab/inventory.md (op 38 GitLab column) + edition-matrix.md (a row per kind): follow-up forge-gitlab/12"
  - "docs/adopting-assay-gitlab.md (the GitLab checklist template + the auditor role's minimum project role per kind): follow-up forge-gitlab/12"
version: 1
id: 7882834e-dcfc-4116-96aa-b211accb9610
---

# Brief 12 — GitLab hardening reads

## Context

forge-gitlab/11 moved `repohardenguard` onto op 38 `RepoHardeningRead(repo, kind)` under the
`auditor` identity and left the GitLab backend refusing every kind by name. This brief supplies the
GitLab kinds. The design constraint is the one every GitLab op in this stream carries: a kind is a
FIXED endpoint literal returning the forge's OWN settings document — never a synthesised
"GitHub-shaped" view — and a checklist is therefore written per forge. The rows compare live values
the forge actually exposes; what CE does not expose is a recorded `not available — <tier>` row, the
guard's existing divergence mechanism, never an approximation.

| Kind | GitLab read (fixed literal) | Tier | Checks it serves |
|---|---|---|---|
| `project` | `GET /projects/:id` | Free | `.visibility`; `.only_allow_merge_if_pipeline_succeeds`, `.only_allow_merge_if_all_discussions_are_resolved` (B5/B6); `.ci_config_path` (C6 — CI definition outside the writable project); `.ci_allow_fork_pipelines_to_run_in_parent_project` |
| `protected-branches` | `GET /projects/:id/protected_branches` | Free (role-level); user/group entries Premium | per branch: `.allow_force_push` = false; `.push_access_levels[].access_level` = 0 (No one) — the CE-level analogue of an empty bypass list; `.merge_access_levels` |
| `protected-tags` | `GET /projects/:id/protected_tags` | Free | `.create_access_levels` for the release-tag pattern (B12) |
| `push-rules` | `GET /projects/:id/push_rule` | **Premium** | on CE the endpoint answers 404/403 → the row is `not available — Premium` (C5); on Premium `.reject_unsigned_commits`, `.prevent_secrets` |
| `approvals` | `GET /projects/:id/approvals` | settings Free to read; enforcement **Premium** | `.reset_approvals_on_push`, `.merge_requests_author_approval` (B4), `.merge_requests_disable_committers_approval` — on CE these are advisory, so the Required cell records the CE meaning |
| `file <path>` | op 22 `ReadFile` (already forge-neutral) | Free | SECURITY/CONTRIBUTING/CODE_OF_CONDUCT presence |

The GitHub backend refuses each GitLab kind by name, exactly as the GitLab backend refuses the
GitHub kinds — the vocabulary is one closed set, the per-forge halves are disjoint, and a
checklist row names a kind its forge serves or gets could-not-check, never the other forge's
document.

files:
- `tools/desk/internal/deskkit/forge.go` — extend `HardeningReadKind` with the five GitLab
  kinds (the validator's closed set grows; still no path argument).
- `tools/desk/internal/deskkit/forge_gitlab.go`, `forge_gitlab_test.go` — implement the five
  kinds; goldens per kind; a `push_rules_premium_gated` golden pinning 403/404 → could-not-check
  carrying a `ForgeAPIError`, never an empty document.
- `tools/desk/internal/deskkit/forge_github.go` — the symmetric named refusal for GitLab kinds.
- `tools/desk/cmd/repohardenguard/repohardenguard_test.go` — a GitLab fixture checklist block
  (`gl/p` project): a Free row, a `not available — Premium` row, a tier-403 row.
- `docs/streams/forge-gitlab/inventory.md`, `edition-matrix.md` — op 38's GitLab column →
  implemented; one matrix row per kind with its docs citation.
- `docs/adopting-assay-gitlab.md` — the GitLab checklist template (rows above, Required cells
  per the profile) and the auditor service account's minimum project role per kind (Reporter
  for `project`/`file`; the protected-branches/tags and approvals reads need at least
  Maintainer on GitLab — dereferenced in Verify row 4, never asserted from memory).
- `changelog/forge-gitlab-12-gitlab-hardening-reads.md` (planned).

single-point-of-failure: the tier-gate mapping — a Premium endpoint on CE must read as
`not available` / could-not-check, never as a value. Backed by two independent layers: the
backend's three-state error classification (a 403/404 arrives as a `ForgeAPIError`, pinned by
golden) and the checklist's own `not available — <tier>` row, which makes NO request at all on a
plan that lacks the feature (the guard's existing `notAvailable()` short-circuit). Different
components, different signals.

facts:
- Freeze rule: each GitLab kind lands with a consuming fixture row in the guard's tests and a
  documented checklist row in the adopter doc.
- The auditor identity is fixed by forge-gitlab/11: a `read_api`-scoped PAT under the brief-03
  custody file `gitlab-auditor.token`. This brief changes no custody; it may RAISE the required
  project ROLE for some kinds (Maintainer), which the adopter doc must state per kind.
- Disclosed CE degradations stay two (spec §1): the identity-level allowlist and enforced
  approvals. The `push-rules` and `approvals` rows express those existing degradations as
  checklist rows; they do not name a third. Secret push protection (Ultimate) is recorded as
  `not available — Ultimate`, the same as the GitHub checklist records a plan-gated feature.
- `.push_access_levels[].access_level == 0` is the CE-expressible form of "no one pushes to
  main"; a user/group entry (Premium) appears as `user_id`/`group_id` fields the row may
  require on Premium and must not require on CE.

## Edition
Minimum GitLab tier: **free**. `project`, `protected-branches`, `protected-tags` and the
`approvals` READ are Free (edition-matrix.md B1, B5, B6, B12; the approvals API page is Free with
Premium-badged rule sections). What degrades on CE is what the profile already discloses: the
allowlist is role-level (B2) and approval settings are advisory (B3/B4) — the checklist records the
CE meaning in the Required cell. `push-rules` is Premium (C5) and is a `not available — Premium`
row on CE; secret push protection is Ultimate and is `not available — Ultimate`. No new degradation.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Extend the kind vocabulary with the five GitLab kinds; implement each on the GitLab backend
   as one fixed endpoint literal returning the raw document; the GitHub backend refuses them by
   name. Goldens per kind + the Premium-gated 403/404 → could-not-check golden.
2. Add the GitLab fixture block to the guard's tests (Free row checked-ok, `not available —
   Premium` row makes no request, tier-403 row could-not-check).
3. Update `inventory.md` (op 38 GitLab column), `edition-matrix.md` (one row per kind, docs
   citation each), and the GitLab adopter doc (checklist template + minimum project role per
   kind, dereferenced against the docs).

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 | check:ci |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | exit 0; output contains `PASS`; the `push_rules_premium_gated` (planned) golden records a 403 classified could-not-check | check:ci +mutation |
| 3 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestGitLabNotAvailableNoRequest -v && go test ./cmd/repohardenguard/ -run TestGitLabTierGateCouldNotCheck -v` | exit 0; output contains `PASS` — a `not available` row issues zero reads; a tier-403 row is could-not-check, never a value (`TestGitLabNotAvailableNoRequest` (planned), `TestGitLabTierGateCouldNotCheck` (planned)) | check +flow |
| 4 | `curl -sS -o /dev/null -w '%{http_code}' -H "PRIVATE-TOKEN: $(cat "$(desktoken auditor --forge gitlab --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')")")" "$GITLAB_API_BASE/projects/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/([^/]+?)(\.git)?$#\1%2F\2#')/protected_branches"` | `200` — the auditor PAT at its documented minimum project role can read the protected-branches document (dereferences the role the adopter doc states) | gate:model +dereference |
| 5 | `grep -c 'not available — Premium' docs/adopting-assay-gitlab.md` | `1` or more — the CE template records the Premium row as a divergence, not as a check | check |
| 6 | `statusgen --root . --consumers` | exit 0 | check |

### Pre-mortem → detection map
| Failure mode of the work | Caught by |
|---|---|
| A Premium endpoint's 403 on CE decoded as an empty document → row reads checked-wrong or ok | row 2 (golden) + row 3 |
| The adopter doc states a project role that cannot actually read the document | row 4 |
| A GitLab kind silently accepted on the GitHub backend (or vice versa) with an empty result | row 2 (`TestForgeNoPassthrough` + coverage) — adequacy of the refusal text is review-only |
| The CE template requires a Premium-only field (`user_id` in the allowlist) and fails every CE project | row 5 + review of the template's Required cells (review-only for adequacy) |

### Dispatch checklist
```
[x] 1. Rows discriminate — rows 2/3 red on a decoded-403; row 4 red on a wrong role claim.
[x] 2. Facts dated — matrix rows cited by id; freshness sha 8953d38d.
[x] 3. Self-contained — kind table with endpoints and tiers is in this file.
[x] 4. Risk answers match files: — the deskkit path trips the risk-path classifier (advisory cross-read), but the change is read-only kinds under the identity 11 fixed: no custody change, no new identity, all four stay no; the implementing PR still carries a Security-Review at the flip gate.
[x] 5. gate-why n/a (gate: model, all no).
[x] 6. Effort honest — five fixed reads + fixtures + docs: M.
[x] 7. consumers: enumerated; row 3 is the flow row.
[x] 8. Pre-mortem run; review-only items named.
```

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

### Implementer offline self-run — 2026-09-15 (worker-desk implementer; NOT verification)

Recorded so the verifier starts warm; the non-implementer run above this line is what
advances the row. Offline (`KUBECONFIG=/dev/null`), no live GitLab or GitHub call; rows that
need a live project are left for the verifier.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `cd tools/desk && go build ./...` + targeted package tests | exit 0 | build exit 0; `go test ./internal/deskkit/` restricted by `-run` to the `TestForgeGitlab`, `TestForgeGithubGolden`, `TestForgeNoPassthrough`, `TestNoForgeCLIShellout` and `TestHardening` families: ok; `go test ./cmd/repohardenguard/`: ok (26 tests, 1 skip). The whole-module `go test ./...` is CI's own run (check:ci) | 2026-09-15 | implementer (self-run) |
| 2 | golden + coverage + no-passthrough | PASS; `push_rules_premium_gated` records a 403 could-not-check | exit 0, PASS ×3; golden `push_rules_premium_gated`: one GET `/api/v4/projects/…/push_rule`, `result: null`, `err: "could-not-check: … (HTTP 403) … record the row as \`not available — Premium\` …"`, `not_found: false`; `push_rules_ce_not_found` is the 404 twin with `not_found: true` | 2026-09-15 | implementer (self-run) |
| 3 | `TestGitLabNotAvailableNoRequest` + `TestGitLabTierGateCouldNotCheck` | PASS; zero reads on a not-available row; tier-403 row could-not-check | exit 0, PASS; the not-available run's call log carries no `read push-rules` and the stub world holds no push-rules document at all (a read would have failed loudly); 403, 404 and the Premium `null` document each report `could-not-check`, exit 6 | 2026-09-15 | implementer (self-run) |
| 4 | live `curl … /protected_branches` under the auditor PAT | `200` | left to the verifier: needs a live project and the provisioned auditor PAT; the adopter doc states Maintainer as the minimum role and says in the same breath that it is stated, not measured | — | — |
| 5 | `grep -c 'not available — Premium' docs/adopting-assay-gitlab.md` | ≥ 1 | `2` (the two push-rules rows of the CE template; the kind table and prose spell the `not available — <tier>` form) | 2026-09-15 | implementer (self-run) |
| 6 | `statusgen --root . --consumers` | exit 0 | exit 0; `statusgen --lint` LINT: PASS | 2026-09-15 | implementer (self-run) |
### Non-implementer verifier run — 2026-09-17 sonnet-5-verifier (verify-desk dispatch) — **VERIFY: PARTIAL (5/6 PASS)** — HELD at `implemented`

Runner ≠ implementer. Own detached temp worktree off `medici-finance/assay` origin/main at `57509073b9b7c989b850c7e5d251f7443ef3794c`.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `go build ./... && go test ./...` | exit 0 | exit 0, every package ok incl. `cmd/repohardenguard` and `internal/deskkit` | 2026-09-17 | sonnet-5-verifier |
| 2 | `TestForgeGitlabGolden`, `TestForgeGitlabCoverage`, `TestForgeNoPassthrough` | exit 0, `push_rules_premium_gated` golden records a classified 403 could-not-check | exit 0 all three. Read the actual golden fixture directly: `"result": null`, `"err": "could-not-check: ... push rules are a Premium feature ..."` — a real classified could-not-check, not a false empty/ok read. Coverage confirms all 46 ops reconciled against inventory.md | 2026-09-17 | sonnet-5-verifier |
| 3 | `TestGitLabNotAvailableNoRequest`, `TestGitLabTierGateCouldNotCheck` | exit 0 | exit 0 both; 4 subtests on the tier-gate test all PASS | 2026-09-17 | sonnet-5-verifier |
| 4 | live GitLab API call under an `auditor` PAT against the pilot project | `200` | **EXPLICITLY UNRUN (could-not-check)** — the live pilot project's slug/API base is deliberately never disclosed in this repo (per `docs/streams/forge-gitlab/pilot-report.md`'s own "Naming" section: only numeric ids are ever given, never a group/project path, so it "carries no private name into a public repo"); `$GITLAB_API_BASE` unset, no gitlab/auditor roster entry available, and minting an auditor token against any repo I could name was correctly refused rather than guessed | 2026-09-17 | sonnet-5-verifier |
| 5 | `grep -c 'not available — Premium' docs/adopting-assay-gitlab.md` | ≥1 | exit 0, `2` | 2026-09-17 | sonnet-5-verifier |
| 6 | `statusgen --root . --consumers` | exit 0 | exit 0 (no local diff to corroborate against, as expected for an unmodified worktree at HEAD) | 2026-09-17 | sonnet-5-verifier |

**Security-Review precondition (named in the brief's own dispatch checklist item 4): CONFIRMED LANDED.** Implementing PR `medici-finance/assay#1179`, merged 2026-09-16T12:07:07Z. `assay-reviewer-app` posted both a correctness-lane APPROVED review and a separate COMMENTED review titled "Security review — forge-gitlab/12" carrying `Security-Review: pass`.

**RISK-VALUE: DERIVED** — `docs/adopting-assay-gitlab.md`'s CE template enumerates every tier/permission literal (Reporter=20, Maintainer=40, Admin=60; access_level 0/30/40/60; two Premium-gated rows, one Ultimate-gated row). Cross-checked against the actual golden fixture `hardening_read_protected_branches.golden.json`: `push_access_levels[0].access_level=0` ("No one"), `merge_access_levels[0].access_level=40` ("Maintainers") — matches the doc's example rows exactly, no drift. The tier-gate SPOF (a Premium 403 never fail-opening into a value) is backed by two independently-observed layers: the backend's error classification (golden, live) and the checklist's own zero-request short-circuit (`TestGitLabNotAvailableNoRequest`, live) — both real and green.

**VERIFY: PARTIAL** — 5/6 rows PASS; row 4 explicitly unrun for a structural, non-guessable reason (the pilot project is deliberately anonymized in this public repo). No defect found anywhere. Held at `implemented`.

### Non-implementer verifier run — VERIFY: BLOCKED — 2026-09-23 opus-5.5-verifier

Runner is not the implementer. Own detached temp worktree cut off merged origin/main at
b3fe2a1c7900f5b2cf9c5da6364fd598a64f609f. Offline (KUBECONFIG=/dev/null), no live GitLab or
GitHub call.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | cd tools/desk && go build ./... && go test ./... | exit 0 | build exit 0. First go test ./... run exit 1 on one transient flake in internal/loopengine (a network-probing test — "git ls-remote: network unreachable" under the offline envelope); the package passed fresh in isolation (-count=1, exit 0, 6.8s) and two subsequent whole-module go test ./... runs came back exit 0 with no FAIL lines. Deliverable packages internal/deskkit and cmd/repohardenguard green every run. Note: verifyrun's hermetic (network-off) check:ci lane is could-not-run on this darwin host (needs Linux unshare --net) — deferred to a Linux CI runner | 2026-09-23 | opus-5.5-verifier |
| 2 | cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v && ...Coverage -v && ...ForgeNoPassthrough -v | exit 0; PASS; push_rules_premium_gated golden records a 403 classified could-not-check | exit 0 all three, PASS. Every hardening-read subtest OK (project, protected-branches incl. two_pages and ceiling_refuses, protected-tags, push_rules_premium, push_rules_premium_gated, push_rules_ce_not_found, approvals, approvals_ce_404, github_kind_refused, unknown_kind). Read the push_rules_premium_gated golden directly: one GET .../push_rule, result is null, err is a could-not-check ForgeAPIError (HTTP 403, "push rules are a Premium feature ... record the row as not available — Premium"), not_found false — a real classified could-not-check, not a false empty/ok read. Coverage reconciles all ops against inventory | 2026-09-23 | opus-5.5-verifier |
| 3 | cd tools/desk && go test ./cmd/repohardenguard/ -run TestGitLabNotAvailableNoRequest -v && ...TestGitLabTierGateCouldNotCheck -v | exit 0; PASS — a not-available row issues zero reads; a tier-403 row is could-not-check, never a value | exit 0 both, PASS. TestGitLabTierGateCouldNotCheck has four subtests all PASS: forbidden_403, not_found_404_ce_has_no_route, premium_null_document_is_not_a_value, premium_value_is_checked | 2026-09-23 | opus-5.5-verifier |
| 4 | live curl of the protected_branches document under the auditor PAT (gate:model +dereference) | 200 — the auditor PAT at its documented minimum project role reads the protected-branches document | COULD-NOT-CHECK (explicitly unrun). Requires live GitLab state, which the offline envelope forbids (KUBECONFIG=/dev/null; no cluster/production endpoint). GITLAB_API_BASE is unset and there is no gitlab/auditor roster entry, so verifyrun's execution of this row exited 3 (curl could not resolve a base) — an environment absence, not an observed failure of the deliverable. The pilot project is deliberately un-named in this public repo (its report gives numeric ids only), so a model verifier cannot construct the call; it is a human-with-live-access act | 2026-09-23 | opus-5.5-verifier |
| 5 | grep -c 'not available — Premium' docs/adopting-assay-gitlab.md | 1 or more | exit 0, count 2 — the two push-rules rows of the CE template (reject-unsigned-commits and prevent-secrets), each recording the Premium row as a divergence, not a check | 2026-09-23 | opus-5.5-verifier |
| 6 | statusgen --root . --consumers | exit 0 | exit 0. "no brief files in the diff ... nothing to corroborate" (expected for an unmodified worktree at merged HEAD) | 2026-09-23 | opus-5.5-verifier |

RISK-VALUE enumeration (kit §4 — enumerate → rank → derive). The item is read-only and
risk metadata is all-no, but the deskkit path trips the risk-path classifier, so the
enumeration is run. Every literal introduced or changed by the diff (merge #1179):

- tier-gate status set {HTTP 403, HTTP 404} → could-not-check (the diff's error
  classification for push-rules) — the brief's declared single-point-of-failure.
- access_level = 0 ("No one") — the CE-expressible empty-bypass form the checklist Required
  cells pin for protected branches.
- gitlabMaxHardeningPage = 10 — new ceiling on the protected-branches/tags page walk (10
  pages x gitlabPerPage 100 = 1000 entries).
- gitlabPerPage = 100 — pre-existing per-page bound reused by the walk.
- endpoint path string literals (projects/%s, .../protected_branches, .../protected_tags,
  .../push_rule, .../approvals) — not risk-bearing thresholds.

Ranked by irreversibility: none is irreversible (read-only guard; a wrong value is
fixed by an edit and redeploy). The two correctness-critical entries (fail-open exposure)
are the tier-gate mapping and access_level=0; the page ceilings fail CLOSED (a walk still
paginating past the ceiling REFUSES with could-not-check rather than truncating, so a
too-low value can never fail open), so they rank last and need no derivation.

RISK-VALUE: DERIVED — tier-gate {HTTP 403, HTTP 404} → could-not-check @ tools/desk/internal/deskkit/testdata/forge_gitlab_golden/push_rules_premium_gated.golden.json (403) and push_rules_ce_not_found.golden.json (404) — GitLab returns 403 where a Premium route is licensed-off and 404 where the route does not exist on CE; both correctly map to a ForgeAPIError could-not-check (result null), never a value. This is the SPOF; both goldens observed green and the guard's tier-gate test confirms a value is never fabricated.

RISK-VALUE: DERIVED — access_level = 0 ("No one") @ tools/desk/internal/deskkit/forge.go:154 (and stated in docs/adopting-assay-gitlab.md lines 529-530, cited to the GitLab Protected Branches API) — GitLab's protected-branches API defines access_level 0 as "No one," 20 Reporter, 30 Developer, 40 Maintainer, 60 Admin; 0 is therefore the correct CE-expressible form of "no bypass to main." Right value, sourced to the forge's own API vocabulary.

RISK-VALUE: N/A for the page-ceiling knobs (gitlabMaxHardeningPage=10, gitlabPerPage=100 @ tools/desk/internal/deskkit/forge_gitlab.go:93,432) — reversible operational bounds that fail closed by design; out of scope for derivation per kit §4 step 3.

Row 4 needs live pilot-project access (human with GitLab access); no defect found in the deliverable.


## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers: with the checklist's `not available` short-circuit removed, does the backend's
403 classification alone still keep a Premium row from reading as a value (negative path on the
layered design)?
