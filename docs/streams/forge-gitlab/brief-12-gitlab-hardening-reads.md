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
- `tools/desk/internal/deskkit/forge.go` — extend `HardeningReadKind` with the five GitLab kinds (the validator's closed set grows; still no path argument).
- `tools/desk/internal/deskkit/forge_gitlab.go`, `tools/desk/internal/deskkit/forge_gitlab_test.go` — implement the five kinds; goldens per kind; a `push_rules_premium_gated` golden pinning 403/404 → could-not-check carrying a `ForgeAPIError`, never an empty document.
- `tools/desk/internal/deskkit/forge_github.go` — the symmetric named refusal for GitLab kinds.
- `tools/desk/cmd/repohardenguard/repohardenguard_test.go` — a GitLab fixture checklist block (`gl/p` project): a Free row, a `not available — Premium` row, a tier-403 row.
- `docs/streams/forge-gitlab/inventory.md`, `docs/streams/forge-gitlab/edition-matrix.md` — op 38's GitLab column → implemented; one matrix row per kind with its docs citation.
- `docs/adopting-assay-gitlab.md` — the GitLab checklist template (rows above, Required cells per the profile) and the auditor service account's minimum project role per kind (Reporter for `project`/`file`; the protected-branches/tags and approvals reads need at least Maintainer on GitLab — dereferenced in Verify row 4, never asserted from memory).
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

### Non-implementer verifier run — VERIFY: BLOCKED — 2/6 pass, 4 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

Runner is not the implementer. Own detached temp worktree cut off merged origin/main at
b3fe2a1c7900f5b2cf9c5da6364fd598a64f609f. Offline (KUBECONFIG=/dev/null), no live GitLab or
GitHub call. Rows 1 and 2 are `check:ci` in this brief's Verify table; on this darwin host the
hermetic `statusgen verifyrun` witness cannot run (needs Linux `unshare --net`), so each is
could-not-check with its direct non-hermetic run recorded. Row 4 needs live GitLab state and row
6's consumers routing corroborates nothing on the fully merged tree.

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | exit 0 | COULD-NOT-CHECK — hermetic-witness-owed-darwin; exit 1; github.com/medici-finance/assay/tools/desk/internal/deskkit ⏎ github.com/medici-finance/assay/tools/desk/cmd/repohardenguard ⏎ internal/loopengine ⏎ TestDrain ⏎ model API overloaded: 529 | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v; echo '--- golden ---'; cat internal/deskkit/testdata/forge_gitlab_golden/push_rules_premium_gated.golden.json` | exit 0; PASS; push_rules_premium_gated golden records a 403 classified could-not-check | COULD-NOT-CHECK — hermetic-witness-owed-darwin; exit 0; --- PASS: TestForgeGitlabGolden ⏎ --- PASS: TestForgeGitlabCoverage ⏎ --- PASS: TestForgeNoPassthrough ⏎ "result": null ⏎ "not_found": false ⏎ push rules are a Premium feature | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestGitLabNotAvailableNoRequest -v && go test ./cmd/repohardenguard/ -run TestGitLabTierGateCouldNotCheck -v` | exit 0; PASS — a not-available row issues zero reads; a tier-403 row is could-not-check, never a value | PASS — no-request-and-cnc; exit 0; --- PASS: TestGitLabNotAvailableNoRequest ⏎ --- PASS: TestGitLabTierGateCouldNotCheck | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | `curl -sS -o /dev/null -w '%{http_code}' -H "PRIVATE-TOKEN: $(cat "$(desktoken auditor --forge gitlab --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')")")" "$GITLAB_API_BASE/projects/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/([^/]+?)(\.git)?$#\1%2F\2#')/protected_branches"` | 200 — the auditor PAT at its documented minimum project role reads the protected-branches document | COULD-NOT-CHECK — no-live-gitlab; exit 3; curl: (3) URL rejected: No host part in the URL ⏎ 000 | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | `grep -c 'not available — Premium' docs/adopting-assay-gitlab.md; grep -n 'not available — Premium' docs/adopting-assay-gitlab.md` | 1 or more | PASS — two-premium-rows; exit 0; 2 ⏎ reject_unsigned_commits \| not available — Premium ⏎ prevent_secrets \| not available — Premium | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | `statusgen --root . --consumers` | exit 0 | COULD-NOT-CHECK — merged-tree; exit 0; consumers: no brief files in the diff against b3fe2a1c7900f5b2cf9c5da6364fd598a64f609f — nothing to corroborate | 2026-09-23 | claude-opus-4-8-verifier |

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

RISK-VALUE: DERIVED — access_level = 0 ("No one") @ tools/desk/internal/deskkit/forge.go:959 (and stated in docs/adopting-assay-gitlab.md lines 529-530, cited to the GitLab Protected Branches API) — GitLab's protected-branches API defines access_level 0 as "No one," 20 Reporter, 30 Developer, 40 Maintainer, 60 Admin; 0 is therefore the correct CE-expressible form of "no bypass to main." Right value, sourced to the forge's own API vocabulary.

RISK-VALUE: N/A for the page-ceiling knobs (gitlabMaxHardeningPage=10 @ tools/desk/internal/deskkit/forge_gitlab.go:2399, gitlabPerPage=100 @ tools/desk/internal/deskkit/forge_gitlab.go:432) — reversible operational bounds that fail closed by design; out of scope for derivation per kit §4 step 3.

Rows 1, 2 are could-not-check (hermetic Linux witness owed); row 4 needs live pilot-project access (human with GitLab access); row 6 corroborates nothing on a merged tree. No defect found in the deliverable.

### Non-implementer verifier run — VERIFY: BLOCKED — 3/6 pass, 3 could-not-check, 0 defect — 2026-09-27 claude-opus-5-5-verifier

Runner is not the implementer (implementing PR #1179, merge commit 4ce477d71). Own detached
temp worktree cut off merged origin/main at b227b40768db08a0a91046899bc1877cf3c6d1ec. Offline
(KUBECONFIG=/dev/null), no live GitLab call. Witness table below is the `statusgen verifyrun`
output (non-dry, clean tree at run start).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestGitLabNotAvailableNoRequest -v && go test ./cmd/repohardenguard/ -run TestGitLabTierGateCouldNotCheck -v` | pass exit=0 | sha256:b62b18610a7b | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 4 | `curl -sS -o /dev/null -w '%{http_code}' -H "PRIVATE-TOKEN: $(cat "$(desktoken auditor --forge gitlab --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')")")" "$GITLAB_API_BASE/projects/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/([^/]+?)(\.git)?$#\1%2F\2#')/protected_branches"` | fail exit=3 | sha256:c3223f8d98ce | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c 'not available — Premium' docs/adopting-assay-gitlab.md` | pass exit=0 | sha256:53c234e5e847 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers` | pass exit=0 | sha256:ec03a67821a6 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |

Per-row notes (direct runs on the same tree, same day, same runner):

- Row 1 — witness could-not-run (check:ci needs the Linux network-off sandbox; host is darwin).
  Direct non-hermetic run: `go build ./...` exit 0; `go test ./...` exit 1, but both packages this
  brief touches are green (cmd/repohardenguard ok, internal/deskkit ok). The three red packages
  are outside this brief and are host artifacts: internal/loopengine (TestDrain and three TestRun_*
  "did not stop within deadline") and cmd/commsloop (TestRunDoesNotBusySpinOnEmptyQueue) are 5s
  wall-clock deadlines that pass when re-run alone (`go test -count=1` → ok, ok); internal/avatar
  TestGolden20px is a byte-exact PNG golden that fails under the host's newer Go toolchain and
  passes under the CI-pinned go1.25.0 (GOTOOLCHAIN=go1.25.0 → ok). Could-not-check, owed to a
  Linux runner on the pinned toolchain.
- Row 2 — witness could-not-run (same sandbox reason). Direct run: exit 0; TestForgeGitlabGolden
  PASS incl. every hardening_read_* golden plus push_rules_premium_gated and push_rules_ce_not_found;
  TestForgeGitlabCoverage "reconciles: 54 operations, all covered"; TestForgeNoPassthrough PASS.
  push_rules_premium_gated golden: one GET of the push_rule route, "result": null, err
  "could-not-check: … permission or tier gate (HTTP 403) … push rules are a Premium feature …",
  "not_found": false. **Mutation (+mutation class):** in the GitLab backend's hardeningGetRaw, a
  mutant returning an empty document `{}` for a tier-gated kind instead of the classified error
  turned TestForgeGitlabGolden RED on push_rules_premium_gated, push_rules_ce_not_found and
  hardening_read_approvals_ce_404; file restored and the tree re-checked clean before the witness
  run. The row discriminates a decoded-403. Could-not-check on the witness; direct run and
  mutation both good.
- Row 3 — PASS. TestGitLabNotAvailableNoRequest PASS; TestGitLabTierGateCouldNotCheck PASS on all
  four subtests (forbidden_403, not_found_404_ce_has_no_route, premium_null_document_is_not_a_value,
  premium_value_is_checked). The same backend mutant left this row green: the guard tests run on a
  stub forge, so row 3 pins the guard layer independently of the backend classification — the two
  layers the brief's single-point-of-failure line names are separately pinned. This also answers
  the Review question: with the not-available short-circuit out of play (the forbidden_403 subtest
  row is a real read, not a not-available row), the 403 still lands as could-not-check.
- Row 4 — could-not-check (environment: no live GitLab project in the offline envelope). The
  witness "fail exit=3" is curl "(3) URL rejected: No host part in the URL" with http_code 000:
  GITLAB_API_BASE is unset and no gitlab-auditor.token custody file exists on this host
  (checked with `desktoken auditor --forge gitlab --no-rotate`, which makes no network contact:
  exit 6, "gitlab token file not found"). Not a defect in the deliverable. **Check-definition
  observation:** the row as authored calls `desktoken auditor --forge gitlab` WITHOUT
  `--no-rotate`, which on a host that does hold the custody file rotates the auditor PAT in place
  against the live instance before the read — a verify row that spends a credential rotation to
  answer a read question. It also derives the project slug from `remote.origin.url`, so it only
  addresses a GitLab project when run from a GitLab-hosted clone. A human re-run should use
  `--no-rotate` and an explicit project path.
- Row 5 — PASS. `2`; the two CE template rows push-rules-signed / push-rules-secrets carry
  Required `not available — Premium`.
- Row 6 — PASS on its literal expectation (exit 0); output "no brief files in the diff against
  b227b40768db… — nothing to corroborate", i.e. vacuous on a fully merged tree.

Since the 2026-09-23 run (merged main b3fe2a1c): no commit touched the hardening-read code path,
its goldens, or the guard's GitLab tests; the adopter doc and the forge files changed only in
unrelated sections (line numbers below re-derived at b227b407).

RISK-VALUE enumeration (kit §4). Risk metadata is all-no and not irreversible, but the deskkit
path trips the risk-path classifier, so the enumeration runs. Literals introduced by #1179 or
named in the Deliverables:

1. tier-gate status set {403, 404} → could-not-check — `fae.Status == http.StatusForbidden || fae.Status == http.StatusNotFound` @ tools/desk/internal/deskkit/forge_gitlab.go:2523 (tier-gated kinds: push-rules, approvals).
2. empty-body refusal `len(raw) == 0` → could-not-check @ tools/desk/internal/deskkit/forge_gitlab.go:2529.
3. CE template Required `[name=main].push_access_levels.0.access_level` = 0 @ docs/adopting-assay-gitlab.md:1075.
4. CE template Required `[name=main].merge_access_levels.0.access_level` = 40 @ docs/adopting-assay-gitlab.md:1076; `[name=v*].create_access_levels.0.access_level` = 40 @ docs/adopting-assay-gitlab.md:1077; `allow_force_push` = false @ docs/adopting-assay-gitlab.md:1074.
5. auditor minimum project role: Reporter (20) for project/file, Maintainer (40) for protected-branches / protected-tags / approvals / push-rules @ docs/adopting-assay-gitlab.md:1018-1023 (and the role table at :142).
6. gitlabMaxHardeningPage = 10 @ tools/desk/internal/deskkit/forge_gitlab.go:2457; gitlabPerPage = 100 @ tools/desk/internal/deskkit/forge_gitlab.go:432.

Ranked by irreversibility: none is irreversible (read-only guard; every value is an edit plus
redeploy). Fail-OPEN exposure ranks first: (1), (3), (2); then (5) (a wrong role fails CLOSED —
a 403 reads could-not-check, never a value); (4) fails closed (a wrong Required value reads
checked-wrong, a false red); (6) last (the walk refuses at the ceiling rather than truncating).

RISK-VALUE: DERIVED — tier-gate {403, 404} @ tools/desk/internal/deskkit/forge_gitlab.go:2523 — a Premium route on a lower tier answers 403 (licensed off / role) or 404 (route absent on CE); both mean "the forge did not show the setting", so both must be could-not-check and neither may become a document. Any other status falls through to mapErr's generic three-state error, which is also never a value. Pinned by the 403 and 404 goldens; the mutation above proves the golden goes red if a 403 is decoded.
RISK-VALUE: DERIVED — access_level = 0 @ docs/adopting-assay-gitlab.md:1075 — GitLab's Protected Branches API defines access_level 0 as "No one" (30 Developer, 40 Maintainer, 60 Admin), so 0 is the CE-expressible form of "nobody pushes to main" — the role-level analogue of an empty bypass list the brief asks for. Side note (fail-closed, not a defect): the same doc records a self-managed read-back observed at 40 when "No one" was requested; on such an instance this row reads checked-wrong, the safe direction.
RISK-VALUE: DERIVED — len(raw) == 0 refusal @ tools/desk/internal/deskkit/forge_gitlab.go:2529 — a 2xx with no body is not a settings document; returning it would let a caller read "empty settings" as a value, so refusing is the only correct mapping.
RISK-VALUE: NAMED, NOT DERIVED — auditor minimum role Maintainer (40) for protected-branches @ docs/adopting-assay-gitlab.md:1020 — the doc itself says it is "a stated minimum, not a measured one"; the measurement is exactly Verify row 4 (a live read under the auditor PAT), which needs a live GitLab project and the provisioned auditor custody. Open question for a human with GitLab access: does a Maintainer-role (and does a Reporter-role) `read_api` PAT get 200 on protected_branches? A wrong answer fails closed (could-not-check), so it is not a fail-open risk.
RISK-VALUE: N/A — page ceilings gitlabMaxHardeningPage = 10 / gitlabPerPage = 100 — reversible operational bounds that fail closed by design (kit §4 step 3); ranked last, no derivation owed.

VERIFY: BLOCKED — rows 3, 5, 6 PASS; rows 1 and 2 could-not-check on the witness (Linux network-off sandbox owed; direct runs green for this brief's packages, row 2 mutation-proven); row 4 could-not-check (live GitLab + auditor custody owed; row definition should gain `--no-rotate`). No defect found in the deliverable. Held at `implemented`.

### Non-implementer verifier run — VERIFY: BLOCKED — 2/6 witness-proven, 0 implementation defects — 2026-09-27 claude-opus-5-5-verifier (verify-desk dispatch)

**Supersede note (2026-09-27, verify-desk):** this run supersedes the batch B (#1783) forge-gitlab/12 receipt and the Evidence block above it. That run executed Verify row 4 unsandboxed (the live-credential class, #1794); this run is network-denied with a trimmed PATH, so row 4 (the only row with a live-credential path) could not run; rows 1-2 are check:ci could-not-run on darwin (#1800). The block above is kept as history.

Runner is not the implementer (implementing PR medici-finance/assay#1179, merge commit
4ce477d7100bc65a57e7fd1a1510bc2f86d22b76). Own detached temp worktree at merged main
e70bc86474f94b6e241d12857a10dcbd8136d556 (equal to the forge's main head at run time, read
with the verifier App token). Witness below written by the pinned statusgen v1.0.27
darwin-arm64 binary (sha256 matches the pin), invoked directly, inside a macOS
network-deny sandbox (`sandbox-exec` profile denying all network) with a minimal PATH
(system dirs plus only the Go toolchain and the pinned statusgen — no credential-minting
verb resolvable), GOTOOLCHAIN=local, KUBECONFIG=/dev/null, GITLAB_API_BASE unset, no GitLab
custody file on the host.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestGitLabNotAvailableNoRequest -v && go test ./cmd/repohardenguard/ -run TestGitLabTierGateCouldNotCheck -v` | pass exit=0 | sha256:5127cc9a8ad5 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `curl -sS -o /dev/null -w '%{http_code}' -H "PRIVATE-TOKEN: $(cat "$(desktoken auditor --forge gitlab --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')")")" "$GITLAB_API_BASE/projects/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/([^/]+?)(\.git)?$#\1%2F\2#')/protected_branches"` | fail exit=3 | sha256:825d9d17f84b | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c 'not available — Premium' docs/adopting-assay-gitlab.md` | pass exit=0 | sha256:53c234e5e847 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers` | pass exit=0 | sha256:1156598a8008 | 2026-09-27 | assay-verifier-app[bot] @ e70bc86474f9 (on-behalf-of human:ian) (forge-identity) |

Per-row notes (real output; supplementary runs are non-hermetic, same merged main, offline):

- Row 1 — check:ci, could-not-run on this darwin host (no `unshare --net`; tracked by
  medici-finance/assay#1800). Supplementary targeted run: `go build ./...` exit 0;
  `go test ./internal/deskkit/ ./cmd/repohardenguard/ -count=1 -timeout 480s` exit 0
  (`ok …/internal/deskkit 58.9s`, `ok …/cmd/repohardenguard 0.36s`). The whole-module
  `go test ./...` is left to the Linux hermetic witness; it was deliberately not run here.
- Row 2 — check:ci, could-not-run on darwin (medici-finance/assay#1800). Supplementary run of
  the exact three commands (each with `-count=1 -timeout 300s`): exit 0; `--- PASS:
  TestForgeGitlabGolden`, `--- PASS: TestForgeGitlabCoverage`, `--- PASS:
  TestForgeNoPassthrough`; 158 PASS lines, 0 FAIL. Golden `push_rules_premium_gated` read
  directly: one GET of `/api/v4/projects/…/push_rule`, `"result": null`, err text
  "could-not-check: … permission or tier gate (HTTP 403) … push rules are a Premium feature …
  record the row as not available — Premium", `"not_found": false`; its 404 twin
  `push_rules_ce_not_found` carries `"not_found": true`, also `result: null`. The row's
  `+mutation` class: the repo's mutation spec (deskkit forge-gitlab mutations file) carries
  a "swallow a tier-gated 403/404 into an empty document" mutant for exactly this path; this
  pass did not execute the mutant, so the mutation leg is unproven here.
- Row 3 — witness pass exit 0 inside the sandbox. Key lines: `--- PASS:
  TestGitLabNotAvailableNoRequest`; `--- PASS: TestGitLabTierGateCouldNotCheck` with subtests
  `forbidden_403`, `not_found_404_ce_has_no_route`, `premium_null_document…` (the null-document-is-not-a-value case),
  `premium_value_is_checked` all PASS. This also answers the Review question's negative path:
  a tier-403 row WITHOUT the `not available` short-circuit is still could-not-check, never a
  value.
- Row 4 — witness `fail exit=3` is environment/check-definition shaped, not a defect: no live
  GitLab instance and no provisioned auditor PAT are available to this verifier. Inside the
  sandbox the row printed `desktoken: command not found`, `cat: : No such file or directory`,
  `curl: (3) URL rejected: No host part in the URL`, `000` — no credential was read, minted or
  rotated and no request left the host. Two check-definition faults the row carries,
  independent of environment: (a) it runs `desktoken auditor` without `--no-rotate`, so on a
  host WITH custody a verify run would rotate a live PAT (medici-finance/assay#1794); (b) its
  `sed -E` uses the lazy `+?` quantifier — macOS system sed rejects it (`RE error:
  repetition-operator operand invalid`, exit 1) and GNU sed accepts it greedily, leaving a
  trailing `.git` in the derived project path; and it derives the GitLab project from this
  checkout's origin, which is the GitHub remote. #1794's proposed explicit-project-path fix
  covers (b). Exact probe owed by a human with GitLab access, auditor PAT at the documented
  Maintainer (40) role: `curl -sS -o /dev/null -w '%{http_code}' -H "PRIVATE-TOKEN: <auditor
  PAT>" "$GITLAB_API_BASE/projects/<group>%2F<project>/protected_branches"` → expect `200`.
- Row 5 — witness pass exit 0; output `2` (lines naming `reject_unsigned_commits` and
  `prevent_secrets` as `not available — Premium` in the CE template).
- Row 6 — witness pass exit 0 but vacuous on a merged tree: `summary: 0 corroborated, 0
  disproved, 5 unchecked` — each consumer entry is "unchanged since the merge-base". It
  proves nothing about the deliverables; not counted as witness-proven.
  - Correction (2026-09-27): the quoted summary is not what the witness recorded. Output hash
    1156598a8008 is the v1.0.27 output at e70bc86474f94b6e241d12857a10dcbd8136d556, where the
    base equals HEAD: the roster echo lines, then `consumers: no brief files in the diff against
    e70bc86474f94b6e241d12857a10dcbd8136d556 — nothing to corroborate`. Reproduced with the
    pinned binary at that commit, network denied: exit 0, same hash 1156598a8008. No consumer
    entry was judged at all. The row stays vacuous and held; the conclusion is unchanged.

Grounding (independent of the rows): all five GitLab kinds are in the closed vocabulary
(tools/desk/internal/deskkit/forge.go, `hardeningKindForge` partition at line 1023); each is
one fixed endpoint literal (tools/desk/internal/deskkit/forge_gitlab.go lines 2443–2450); both
backends refuse the other forge's kinds through one shared by-name refusal; inventory,
edition matrix, the adopter doc's §5a CE template and a changelog fragment all landed with
the implementing PR (the fragment has since been folded into CHANGELOG.md, which carries the
entry at merged main).

Risk-bearing value — enumeration over the implementing diff (merge 4ce477d7: deskkit
forge.go / forge_gitlab.go / forge_github.go, repohardenguard check.go / forge.go /
main.go, and the adopter doc's §5a checklist template):

1. tier-gate status set {403, 404} → could-not-check @ tools/desk/internal/deskkit/forge_gitlab.go:2523
2. 2xx-with-empty-body refused (`len(raw) == 0`) @ tools/desk/internal/deskkit/forge_gitlab.go:2529
3. kind→forge partition (five GitLab kinds → ForgeGitLab, six GitHub kinds → ForgeGitHub) @ tools/desk/internal/deskkit/forge.go:1023
4. endpoint literals `projects/%s`, `…/push_rule`, `…/approvals`, `…/protected_branches`, `…/protected_tags` @ tools/desk/internal/deskkit/forge_gitlab.go:2443–2450
5. main-push-no-one Required = `0` @ docs/adopting-assay-gitlab.md:1075
6. main-merge-maintainers Required = `40` @ docs/adopting-assay-gitlab.md:1076; release-tags Required = `40` @ docs/adopting-assay-gitlab.md:1077
7. auditor minimum project role: Reporter (20) for `project`/`file`, Maintainer (40) for protected-branches / protected-tags / approvals / push-rules @ docs/adopting-assay-gitlab.md:1018–1023
8. template Required cells: visibility `private`, merge-pipeline `true`, merge-threads `true`, main-no-force `false`, fork-pipelines `false`, approvals-reset `true`, approvals-no-author `false`, approvals-no-committer `true`, push-rules rows `not available — Premium`, secret-push-protection `not available — Ultimate` @ docs/adopting-assay-gitlab.md:1069–1083
9. gitlabMaxHardeningPage = 10 @ tools/desk/internal/deskkit/forge_gitlab.go:2457; gitlabPerPage = 100 @ tools/desk/internal/deskkit/forge_gitlab.go:432

Rank: the item is read-only and all risk fields are `no`; nothing here is irreversible — every
entry is fixed by an edit and a redeploy. Top-ranked are the entries whose error fail-OPENS the
instrument (a Premium/absent document reading as a value, or a checklist passing a project
that does not satisfy it): 1, 2, 3, 5, 6, 7. Entry 8 is adopter-editable template content
(reversible, advisory on CE per spec §1). Entry 9 fails CLOSED (a walk past the ceiling
refuses rather than truncates) and ranks last.

RISK-VALUE: DERIVED — tier-gate {403, 404} → could-not-check @ tools/desk/internal/deskkit/forge_gitlab.go:2523 — GitLab answers 403 when the role or licensed tier does not expose a route and 404 when the route does not exist on Community Edition; in both cases no settings document was observed, so the only honest state is could-not-check (result null, ForgeAPIError reachable), which is what both goldens and the guard's tier-gate test show.
RISK-VALUE: DERIVED — empty-body guard `len(raw) == 0` @ tools/desk/internal/deskkit/forge_gitlab.go:2529 — a 2xx with no body is not a settings document; refusing it closes the one remaining path by which "nothing" could be read as "empty settings".
RISK-VALUE: DERIVED — kind→forge partition @ tools/desk/internal/deskkit/forge.go:1023 — the brief requires disjoint per-forge halves of one closed set; the map assigns exactly the kind table's five kinds to GitLab and the six pre-existing kinds to GitHub, and each backend refuses the other half by name with zero requests (golden refusal cases on both backends).
RISK-VALUE: DERIVED — main-push-no-one Required = 0 @ docs/adopting-assay-gitlab.md:1075 — GitLab's Protected Branches API defines access level 0 as "No one" (30 Developer, 40 Maintainer, 60 Admin), so 0 is the CE-expressible form of "nobody pushes to main", as the brief's facts state.
RISK-VALUE: DERIVED — main-merge-maintainers / release-tags Required = 40 @ docs/adopting-assay-gitlab.md:1076–1077 — 40 is GitLab's Maintainer access level, matching the rows' stated intent ("Allowed to merge / create = Maintainers") and the provisioning table's `merge_access_level=40`; the protected-branches golden renders the same 0 / 40 values.
RISK-VALUE: NAMED, NOT DERIVED — auditor minimum role Maintainer (40) for protected-branches / protected-tags / approvals / push-rules, Reporter (20) for project / file @ docs/adopting-assay-gitlab.md:1018–1023 — the adopter doc itself says the Maintainer rows are "a stated minimum, not a measured one" (GitLab's API pages state no minimum role for these GETs); the derivation is the live read-back of Verify row 4, which needs a GitLab instance and a provisioned auditor PAT this verifier does not have. OPEN QUESTION for a human with GitLab access: does a `read_api` auditor PAT at Maintainer (and would one at Reporter/Developer) get `200` on `GET /projects/:id/protected_branches`? A wrong value is reversible and fails closed (403 → could-not-check), but row 4 is the only row that proves it.
RISK-VALUE: N/A — entry 9 (page ceilings 10 × 100) — reversible operational bound that fails closed by design; out of derivation scope per kit §4 step 3.

Other observations: the brief's Context names op 38; the code has renumbered it op 40 (naming
drift only, no behaviour change).

VERIFY: BLOCKED — rows 3 and 5 witness-proven; rows 1 and 2 could-not-run (check:ci, darwin,
medici-finance/assay#1800) with green non-hermetic supplementary runs; row 4 needs a live
GitLab instance plus a provisioned auditor PAT and carries the check-definition faults tracked
by medici-finance/assay#1794; row 6 passes vacuously on a merged tree. No implementation
defect found. Status stays `implemented`.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | could-not-run exit=- — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestGitLabNotAvailableNoRequest -v && go test ./cmd/repohardenguard/ -run TestGitLabTierGateCouldNotCheck -v` | pass exit=0 | sha256:0c12566622bb | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `curl -sS -o /dev/null -w '%{http_code}' -H "PRIVATE-TOKEN: $(cat "$(desktoken auditor --forge gitlab --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')")")" "$GITLAB_API_BASE/projects/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/([^/]+?)(\.git)?$#\1%2F\2#')/protected_branches"` | fail exit=3 | sha256:825d9d17f84b | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c 'not available — Premium' docs/adopting-assay-gitlab.md` | pass exit=0 | sha256:53c234e5e847 | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers` | pass exit=0 | sha256:c53bb5f05568 | 2026-09-28 | assay-verifier-app[bot] @ 25fb2a4b3ea3 (on-behalf-of human:ian) (forge-identity) |

**Re-run (round 2) 2026-09-27 at the #1801 merge tree 25fb2a4b, after #1685 added the ExpectedSHA conditional-write precondition to forge.go / forge_github.go / forge_gitlab.go**
(claude-opus-5-5-verifier, verify-desk dispatch; runner is not the implementer). The witness table
directly above ran at 25fb2a4b3ea34c8b3a5c7acbe540262f8e76309c, a local merge of PR #1801's head
2f1e152f3baa93f0d06d9ccdcde4690c2b9dec16 with main c50a38fc12518a4eec4db37e8dd847d49e79149a (the
forge's main head at run time, read with the verifier App token); rows dated 2026-09-28 (UTC
stamp). Same envelope as the forge-gitlab/11 round-2 re-run: pinned statusgen v1.0.27 invoked
directly, `sandbox-exec` STRICT all-network deny (no loopback), `env -i`, KUBECONFIG=/dev/null,
GOFLAGS=-count=1, a fresh empty GOCACHE, GOPROXY=off, GOTOOLCHAIN=local, PATH limited to the system
dirs plus `go` and the pinned `statusgen` (`desktoken`, `gh`, `glab` do not resolve),
GITLAB_API_BASE unset, no GitLab custody file. Two earlier attempts also set TMPDIR to a scratch
path; they were discarded and re-run without it, with the same exit on every row.

#1685 is on the WRITE path only: `WriteFileInput.ExpectedSHA` plus `expectedSHAPrecondition` in
forge.go (+30) and one call to it in each backend's `WriteFile` (forge_github.go +8,
forge_gitlab.go +5, the latter at line ~3905, after every hardening-read line this brief cites).
The GitLab hardening kinds, their paths, the tier-gate classification, the empty-body guard and
the goldens are untouched. Line cites: forge_gitlab.go:432 / 2399 / 2443 / 2457 / 2523 / 2529 are
unchanged; forge.go moved down by 30, so the kind-to-forge map is now forge.go:1053 (was 1023) and
the forge.go:959 cite is now forge.go:989.

Per row, against the previous witness (at e70bc864):
- Row 1: unchanged, check:ci could-not-run on darwin (medici-finance/assay#1800). Out of witness,
  network denied except loopback: `go build ./...` exit 0; `go test -count=1 -timeout 480s` of
  internal/deskkit and cmd/repohardenguard (plus internal/forgeban and cmd/desktoken) exit 0, all
  `ok`. The whole-module `go test ./...` was NOT run here.
- Row 2: unchanged, check:ci could-not-run (#1800). Out of witness, same loopback envelope,
  uncached: the three named tests `--- PASS`, exit 0, 158 PASS lines and 0 FAIL;
  `push_rules_premium_gated` golden-pinned OK. The mutation leg is still unexecuted.
- Row 3: pass, NEW hash 0c12566622bb, uncached (-count=1, fresh GOCACHE) inside the strict deny:
  `--- PASS: TestGitLabNotAvailableNoRequest` and `TestGitLabTierGateCouldNotCheck` with its four
  subtests (403, CE 404, Premium null document, Premium value) all PASS. The hash differs because
  the uncached output carries fresh timings; the result did not move.
- Row 4: unchanged, `fail exit=3`, same hash 825d9d17f84b. It remains could-not-check: it needs a
  live GitLab instance and a provisioned auditor PAT, and carries the row fixes tracked by
  medici-finance/assay#1794.
- Row 5: unchanged, pass, same hash 53c234e5e847.
- Row 6: pass, new hash c53bb5f05568 (base now c50a38fc), still VACUOUS on a merged tree:
  `summary: 0 corroborated, 0 disproved, 16 unchecked, 1 brief(s) claiming nothing`.

Witness-proven this run: rows 3 and 5. Held: rows 1, 2, 4 and 6. No hardening-read row's result
moved, and no implementation regression from #1685. The risk-bearing enumeration is unchanged
apart from the forge.go line shift above; the RISK-VALUE: NAMED, NOT DERIVED auditor minimum-role
line remains an open question for a human with GitLab access.

VERIFY: BLOCKED (unchanged in substance). Status stays `implemented`.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer verification on merged main b0088804294b8b68ad8d06f341e6f0fd9dd2637d, gate: model, all four risk answers no. First table: the `statusgen verifyrun` execution witness, landed verbatim; it ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, `unshare --net` available), statusgen built in-container from a clone pinned to this SHA. Second table: the hand run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 | sha256:193a4cd4b09d | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | fail exit=1 | sha256:7b35e8abc818 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestGitLabNotAvailableNoRequest -v && go test ./cmd/repohardenguard/ -run TestGitLabTierGateCouldNotCheck -v` | pass exit=0 | sha256:31315e2b569c | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 4 | `curl -sS -o /dev/null -w '%{http_code}' -H "PRIVATE-TOKEN: $(cat "$(desktoken auditor --forge gitlab --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')")")" "$GITLAB_API_BASE/projects/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/([^/]+?)(\.git)?$#\1%2F\2#')/protected_branches"` | fail exit=3 | sha256:e9af73f07960 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c 'not available — Premium' docs/adopting-assay-gitlab.md` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers` | pass exit=0 | sha256:9aa9991922df | 2026-09-30 | assay-verifier-app[bot] @ b0088804294b (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | Verify row 1: cd tools/desk && go build ./... && go test ./... | exit 0 | exit 1 on darwin. Build exit 0; 87 packages ok including cmd/repohardenguard and internal/deskkit (116s); the one red package is internal/avatar TestGolden20px, a byte-exact PNG golden that differs under the host go1.27.1 toolchain (outside this brief). Direct Linux run (golang:1.25-bookworm, go1.25.14, container network none, loopback up): exit 1; deskkit and repohardenguard ok; red packages cmd/deskevidence, cmd/deskgit, cmd/deskwt (git 2.39.5 in the image, #1926 class: merge-tree exit 129, origin resolution, worktree pushurl) and cmd/desksourceguard TestDefaultPlatform (host is linux-arm64, which desk-tools does not publish). All outside this brief. CI corroboration: the public ci workflow at b0088804 (build-test job, step "Build and vet every module", which runs go test ./... for tools/desk) concluded success | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 2 | Verify row 2: the three go test runs (TestForgeGitlabGolden, TestForgeGitlabCoverage, TestForgeNoPassthrough) | exit 0; PASS; push_rules_premium_gated records a 403 classified could-not-check | exit 0 on darwin; --- PASS for all three; 159 PASS lines, 0 FAIL; coverage "reconciles: 55 operations, all covered". Golden push_rules_premium_gated read directly: one GET of the push_rule route, result null, err "could-not-check: … permission or tier gate (HTTP 403) … push rules are a Premium feature … record the row as not available — Premium", not_found false. Same three commands exit 0 in the Linux container with loopback up. Mutation leg executed: a mutant in hardeningGetRaw returning an empty document for a tier-gated 403/404 turned TestForgeGitlabGolden red on push_rules_premium_gated, push_rules_ce_not_found and hardening_read_approvals_ce_404; file restored | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 3 | Verify row 3: go test TestGitLabNotAvailableNoRequest then TestGitLabTierGateCouldNotCheck in cmd/repohardenguard | exit 0; PASS; not-available row issues zero reads; tier-403 row is could-not-check | exit 0 on darwin and in the Linux container; --- PASS TestGitLabNotAvailableNoRequest; --- PASS TestGitLabTierGateCouldNotCheck with subtests forbidden_403, not_found_404_ce_has_no_route, premium_null_document_is_not_a_value, premium_value_is_checked. Under the row 2 mutant this row stayed green (guard runs on a stub forge), so the two layers the SPOF line names are pinned independently | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 4 | Verify row 4: curl of the protected_branches route under the auditor PAT via desktoken auditor --forge gitlab | 200 | could-not-check, exit 3, http_code 000. Run as written with a trimmed PATH (desktoken not resolvable, so nothing could be minted or rotated) and GITLAB_API_BASE unset: "desktoken: command not found", "cat: : No such file or directory", macOS sed "RE error: repetition-operator operand invalid" on the lazy quantifier, "curl: (3) URL rejected: No host part in the URL". No live GitLab instance and no GitLab auditor custody on this host. Check-definition faults independent of environment: the lazy +? quantifier is rejected by BSD sed; the project path is derived from this checkout's origin, which is GitHub; desktoken runs without --no-rotate (#1794) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 5 | Verify row 5: grep -c 'not available — Premium' docs/adopting-assay-gitlab.md | 1 or more | exit 0, output 2 (template rows push-rules-signed and push-rules-secrets) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |
| 6 | Verify row 6: statusgen --root . --consumers (statusgen built from the clone at b0088804) | exit 0 | exit 0; "consumers: no brief files in the diff against b0088804… — nothing to corroborate" (vacuous on a merged tree). Supplementary run on a clone at the implementing merge 4ce477d7 with origin/main at its first parent 39a63769: exit 0, "summary: 0 corroborated, 0 disproved, 5 unchecked" — every entry "unchanged since the merge-base", because the consumers list was written at authoring time, so this gate cannot corroborate this brief on any diff | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ b0088804294b (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — tier-gate {403, 404} → could-not-check @ tools/desk/internal/deskkit/forge_gitlab.go:2547 — on an edition or role that does not expose a Premium route GitLab answers 403 (licensed off or role wall) or 404 (route absent on CE); in neither case was a settings document observed, so could-not-check with the ForgeAPIError reachable is the only honest state; any other status falls to mapErr, also never a value. Pinned by the 403 and 404 goldens; the executed mutant proves the golden turns red if a gated status is decoded into a document.
RISK-VALUE: DERIVED — len(raw) == 0 refusal @ tools/desk/internal/deskkit/forge_gitlab.go:2553 — a 2xx with no body is not a document; returning it would let a caller read nothing as empty settings, so refusal is the only correct mapping.
RISK-VALUE: DERIVED — kind-to-forge partition @ tools/desk/internal/deskkit/forge.go:1085 — the brief requires one closed vocabulary with disjoint per-forge halves; the map assigns exactly the kind table's five kinds to GitLab and the six pre-existing kinds to GitHub, and both backends run the shared by-name refusal before building any request.
RISK-VALUE: DERIVED — main-push-no-one Required = 0 @ docs/adopting-assay-gitlab.md:1075 — GitLab's protected-branches API defines access level 0 as No one (30 Developer, 40 Maintainer, 60 Admin), so 0 is the CE-expressible form of "nobody pushes to main", as the brief's facts state; main-no-force = false is the direct reading of allow_force_push.
RISK-VALUE: DERIVED — main-merge-maintainers and release-tags Required = 40 @ docs/adopting-assay-gitlab.md:1076-1077 — 40 is GitLab's Maintainer access level, matching each row's stated intent; fails closed if wrong.
RISK-VALUE: NAMED, NOT DERIVED — auditor minimum role Maintainer (40) for protected-branches, protected-tags, approvals, push-rules; Reporter (20) for project and file @ docs/adopting-assay-gitlab.md:1018-1023 — the doc itself says the Maintainer rows are "a stated minimum, not a measured one"; the derivation is Verify row 4's live read-back, which needs a GitLab instance and provisioned auditor custody this verifier does not have. A wrong value fails closed (403 → could-not-check); question #1946.
RISK-VALUE: N/A — page ceilings 10 x 100 @ tools/desk/internal/deskkit/forge_gitlab.go:2481 and :432 — reversible operational bound that fails closed; ranked last, no derivation owed per kit §4 step 3.

Notes:
- BLOCKED; no implementation defect found. Rows 3, 5 and 6 pass in the witness and by hand (row 6 vacuously: every entry is unchanged since merge-base). Row 2 passes by hand and in Linux with loopback up, and a deliberate break of the tier gate reddens three goldens; in the witness it fails only on the sandbox loopback limit (#1925). Row 1 fails in the witness on #1925 and the image's git 2.39 (#1926); its only host-toolchain failure is an avatar PNG golden under go1.27, and public CI build-test at this SHA is green. Row 4 could not be checked (needs a live GitLab and auditor custody) and its command needs re-authoring (#1927). A whole-module test run leaves an untracked mailbox directory in the source tree (#1947); it does not affect this brief's rows. `statusgen brief --check-verified` with a hypothetical flip exits 1. One RISK-VALUE line is NAMED, NOT DERIVED (question #1946); five are DERIVED and one is N/A.
- Row 1: fails as authored in every environment available here, for reasons outside this brief: host toolchain PNG golden on darwin; #1925 loopback-down in the witness sandbox; #1926 git 2.39.5 and a linux-arm64 platform assertion in the bookworm container. The packages this brief touches (internal/deskkit, cmd/repohardenguard) are green on darwin and in Linux with loopback up, and the public ci workflow's build-test job (which runs go test ./... over tools/desk) concluded success at b0088804. A passing witness needs a linux-amd64 runner with loopback up inside the sandbox and a current git.
- Row 2: fails in the witness only because of #1925 (loopback down in the unshare sandbox); the same commands pass by hand on darwin and in Linux, and the mutation leg was executed and killed.
- Row 4: could-not-check (no live GitLab, no auditor custody) and a check-definition failure as authored (BSD sed rejects the lazy quantifier; the project path is taken from a GitHub origin; desktoken runs without --no-rotate, #1794). Owed: a human with GitLab access runs the read-back with an explicit project path and a Maintainer-role read_api auditor PAT and records the status code.
- Row 6: passes as authored but proves nothing on a merged tree, and on the implementing diff every consumer entry reports unchanged since the merge-base; the consumer paths were confirmed by reading main directly (grounded expectation above).
- Review question (negative path): with the not-available short-circuit out of play, a 403 still lands as could-not-check (row 3's forbidden_403 subtest is a real read, not a not-available row), and the backend mutant shows the guard layer holds independently of the backend classification.
- Side observation: the whole-module test run in the witness left an untracked tools/desk/cmd/commsloop/mailbox/ directory in the tree after verifyrun (a test writing into the source tree); it did not affect this brief's rows (#1947).
- Naming drift only: the brief's Context says op 38; the code and inventory call it op 40.
- No implementation defect found. No guard or security-control bypass found.

VERIFY: BLOCKED

### Non-implementer verifier re-run: 2026-10-02T22:28:05Z (UTC), assay-verifier-app[bot] (claude-opus-5-5[1m]) (on-behalf-of human:ian), merged main e1d99484ffd91b649ea45e10a1cecf4ba2a4924b

Non-implementer re-run of the Verify table on a detached temporary worktree at merged main e1d99484ffd91b649ea45e10a1cecf4ba2a4924b (worktree clean before and after; nothing edited, committed or pushed). Host: darwin-arm64, go1.27.1, GOPROXY=off, KUBECONFIG=/dev/null, GITLAB_API_BASE unset. Every go command ran under a throwaway HOME outside the checkout, holding only a read-only copy of the roster. PATH was trimmed to the Go toolchain, git and system directories, so no credential-minting verb was resolvable at any point (the #1794 containment). Trigger for the re-run: files the 2026-09-27 outcome record declared as inputs changed on main. gate: model, all four risk answers no.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | Verify row 1: cd tools/desk && go build ./... && go test ./... | exit 0 | exit 0 on the second of two full runs: build exit 0, 96 packages ok, 0 FAIL (95 served from the test cache written by the first run at this same tree, internal/regression re-executed: ok 36.7s). The first full run exited 1 with exactly one red package: internal/regression TestReg786FleetHardening, "fixture suite tools/create-fleet-gitlab_test.sh: signal: killed" at its fixed 60s deadline after 49 ok assertions, on a host at load average about 22; the other 95 packages ok, including internal/deskkit, cmd/repohardenguard and internal/avatar. The same test alone: exit 0, PASS in 25.05s, 49 ok assertions. So the one red is a load-sensitive deadline outside this brief, and every package has a green result at this sha. Change since 2026-09-30: internal/avatar TestGolden20px is now green on this toolchain (#1956 compares decoded pixels), and no untracked directory was left in the tree (#1949) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2: the three go test runs in internal/deskkit (TestForgeGitlabGolden, TestForgeGitlabCoverage, TestForgeNoPassthrough), each with -v | exit 0; output contains PASS; the push_rules_premium_gated golden records a 403 classified could-not-check | exit 0; "--- PASS: TestForgeGitlabGolden", "--- PASS: TestForgeGitlabCoverage", "--- PASS: TestForgeNoPassthrough"; 159 PASS lines, 0 FAIL; "reconciles: 55 operations, all covered"; "golden-pinned operation "push_rules_premium_gated" OK". The golden read directly: one GET of the push_rule route, result null, err "could-not-check: … permission or tier gate (HTTP 403) … record the row as not available — Premium", not_found false. Mutation leg executed on a scratch copy of the module (the tracked file was never edited): a mutant in hardeningGetRaw returning an empty document for a tier-gated 403/404 gave exit 1 with FAIL on push_rules_premium_gated, push_rules_ce_not_found and hardening_read_approvals_ce_404 | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3: go test TestGitLabNotAvailableNoRequest then TestGitLabTierGateCouldNotCheck in cmd/repohardenguard, each with -v | exit 0; output contains PASS; a not-available row issues zero reads; a tier-403 row is could-not-check, never a value | exit 0; "--- PASS: TestGitLabNotAvailableNoRequest"; "--- PASS: TestGitLabTierGateCouldNotCheck" with subtests forbidden_403, not_found_404_ce_has_no_route, premium_null_document_is_not_a_value, premium_value_is_checked all PASS | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4: curl of the protected_branches route with the auditor PAT obtained through desktoken auditor --forge gitlab, against GITLAB_API_BASE | 200 | could-not-check: exit 3, http_code 000. Executed as written with no desktoken resolvable and GITLAB_API_BASE unset, so nothing was minted or rotated and no request left the host: "command not found: desktoken", "cat: : No such file or directory", "curl: (3) URL rejected: No host part in the URL". This host has no live GitLab instance and no GitLab auditor custody, and minting that credential is a human act this verifier does not perform. The row is unchanged since 2026-09-30 and still carries its check-definition faults: desktoken is called without --no-rotate (#1794); the project path is derived from this checkout's origin, which is a GitHub remote (GNU sed yields "medici-finance%2Fassay.git", BSD sed rejects the lazy quantifier with "RE error: repetition-operator operand invalid") | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5: grep -c 'not available — Premium' docs/adopting-assay-gitlab.md | 1 or more | exit 0; output "2" (template rows push-rules-signed and push-rules-secrets, lines 1081-1082) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6: statusgen --root . --consumers (statusgen built from this tree) | exit 0 | exit 0; "consumers: no brief files in the diff against e1d99484ffd91b649ea45e10a1cecf4ba2a4924b — nothing to corroborate" (vacuous on a merged tree, as on 2026-09-30) | 2026-10-02 assay-verifier-app[bot] (on-behalf-of human:ian) |

Execution witness (`statusgen verifyrun --brief … --dry-run`, pinned statusgen v1.0.31 darwin-arm64, same trimmed PATH and throwaway HOME; nothing written to the brief): exit 2, 3 of 6 rows pass.

- row 1: could-not-run (exit=-, sha256:e3b0c44298fc) — check:ci hermetic execution needs a network-off sandbox; the sandbox uses unshare --net, a Linux facility, and this host is darwin (#1800)
- row 2: could-not-run (exit=-, sha256:e3b0c44298fc) — same reason (#1800)
- row 3: pass (exit=0, sha256:83819653abad)
- row 4: fail (exit=3, sha256:ca40ae52e45c) — exit 3, expected 0
- row 5: pass (exit=0, sha256:53c234e5e847)
- row 6: pass (exit=0, sha256:333bfa52f1ad)

Findings:

- Verdict unchanged: BLOCKED, no implementation defect found. By hand rows 1, 2, 3, 5 and 6 pass; row 4 could not be checked.
- #1794 is still open (labels question, raised-by:desk; last comment 2026-10-01 routes it as out-of-scope for a triage pass and states it is not fixed). Neither half of what it asks for has happened: the witness still executes a credential-minting row on a plain run (this run was safe only because the PATH was trimmed), and row 4 has not been re-authored with --no-rotate and an explicit project path. #1946 (auditor minimum roles stated, not measured) is also open.
- The human act still owed for row 4: someone with access to a GitLab instance provisions a read_api auditor PAT at the documented Maintainer role, runs the protected-branches read-back against an explicit project path, and records the status code. No record of that act was found on #1794 or #1946.
- Input changes since the 2026-09-27 record (#1917, #1914, #1851 in forge.go, forge_github.go and forge_gitlab.go; Evidence batches in this brief) moved no hardening-read row: the tier gate, the empty-body refusal, the kind partition and every golden under this brief still pass, and the mutation leg still kills.
- Row 1 moved from red to green by hand on darwin since 2026-09-30: the avatar golden now compares decoded pixels (#1956), and the commsloop test that wrote into the source tree is rooted under a temp directory (#1949). Its remaining weakness is the 60s deadline on the shell fixture in internal/regression, which was killed once under host load and passed alone in 25s; that is outside this brief.
- Rows 1 and 2 are check:ci and stay could-not-run in the witness on darwin (#1800); a witness for them needs a Linux runner (and, per the 2026-09-30 block, loopback up inside the sandbox, #1925).
- Disclosure: a first attempt placed the temp directory inside the checkout; that made several unrelated packages fail (unix socket path length, tests that expect a directory outside any repository, tree walks meeting transient files). That run was discarded as contaminated by the verifier's own setup, the scratch directory was removed, and the runs recorded above used the default temp directory with HOME outside the checkout.

Risk-bearing values (re-read at this sha; the diff scope is the implementing change's hardening-read kinds plus the adopter-doc checklist rows):

RISK-VALUE: DERIVED — tier-gate statuses {403, 404} → could-not-check @ tools/desk/internal/deskkit/forge_gitlab.go:2547 — on an edition or role that does not expose a Premium route GitLab answers 403 (licensed off or role wall) or 404 (route absent on CE); in neither case was a settings document observed, so could-not-check is the only honest state. The executed mutant shows three goldens turn red if a gated status is decoded into a document.
RISK-VALUE: DERIVED — len(raw) == 0 refusal @ tools/desk/internal/deskkit/forge_gitlab.go:2553 — a 2xx with no body is not a document; returning it would let a caller read nothing as empty settings.
RISK-VALUE: DERIVED — kind-to-forge partition @ tools/desk/internal/deskkit/forge.go:1086 — the brief requires one closed vocabulary with disjoint per-forge halves; the map assigns the five kind-table kinds to GitLab and the six earlier kinds to GitHub, and TestForgeNoPassthrough passes.
RISK-VALUE: DERIVED — main-push-no-one Required = 0 @ docs/adopting-assay-gitlab.md:1075 — GitLab's protected-branches API defines access level 0 as No one (30 Developer, 40 Maintainer, 60 Admin), so 0 is the CE-expressible form of "nobody pushes to main".
RISK-VALUE: DERIVED — main-merge-maintainers and release-tags Required = 40 @ docs/adopting-assay-gitlab.md:1076-1077 — 40 is GitLab's Maintainer access level, matching each row's stated intent; a wrong value fails closed.
RISK-VALUE: NAMED, NOT DERIVED — auditor minimum role Maintainer (40) for protected-branches, protected-tags, approvals and push-rules; Reporter (20) for project and file @ docs/adopting-assay-gitlab.md:1018-1023 — the doc itself calls the Maintainer rows "a stated minimum, not a measured one"; the derivation is Verify row 4's live read-back, which needs a GitLab instance and provisioned auditor custody this verifier does not have (#1946). A wrong value fails closed (403 → could-not-check).
RISK-VALUE: N/A — page ceilings gitlabMaxHardeningPage = 10 @ tools/desk/internal/deskkit/forge_gitlab.go:2481 and gitlabPerPage = 100 @ tools/desk/internal/deskkit/forge_gitlab.go:432 — reversible operational bounds that fail closed; ranked last, no derivation owed.

VERIFY: BLOCKED — row 4 (live GitLab read-back under the auditor PAT) could not be checked: it needs a human-provisioned credential and a live instance, and the row still needs the #1794 re-authoring; rows 1, 2, 3, 5, 6 pass by hand, witness 3/6

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-04 claude-opus-5-5-verifier

Non-implementer re-run of the Verify table against merged main fe12ee0c90b9e39e78afc291e5b17111844356cb, on a detached temporary worktree at that sha. Nothing was edited, committed or pushed, and no live GitLab or GitHub endpoint was called by any row. gate: model; all four risk answers are no. Runner: assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian).

Why this re-run: the 2026-10-02 receipt at e1d99484f went stale because tools/desk/internal/deskkit/forge.go, forge_gitlab.go and forge_github.go changed on main (#2046, #2118, #2138). The changes are PullRequest.CrossRepo with its two constants, RepoCommit Parents/Files/FilesComplete, RefComparison Commits/CommitsComplete, GitHub's head/base repo decode, a walked change-comment read for ListCommentsTyped, commit parents and files in the GitHub commit and compare reads, and GitLab CrossRepo derived from MR source and target project ids. None of them touches the hardening-read path: zero diff lines mention hardening, and HardeningReadKind, hardeningKindForge, RepoHardeningRead, hardeningGetRaw, hardeningWalk, refuseHardeningKindForForge, the tier-gated kind map and repohardenguard are byte-unchanged. They do pass through the shared harnesses that rows 2 and 1 run (TestForgeGitlabGolden, TestForgeGitlabCoverage, TestForgeNoPassthrough, the whole-module run), so those rows were re-run in full.

Execution witness: `statusgen verifyrun` on Linux. Container golang:1.25-trixie, already cached and not pulled (golang@sha256:2c4c60ef415fbfa5e90300722293bef36c5e63fae17570ce18f580af933dbd73, linux/arm64, go1.25.14, git 2.47.3). It ran with `--network none`, GOPROXY=off and the host module cache mounted read-only. The clone was pinned to fe12ee0c9, with origin/main also pinned to fe12ee0c9 and every other remote ref deleted. statusgen was built in the container from that clone. The container ran as host uid 501, with a throwaway HOME holding only a copy of roster.env and a safe.directory entry. desktoken was not on PATH and GITLAB_API_BASE was unset. Disclosure: this throwaway, network-off container ran with `--security-opt seccomp=unconfined` so that statusgen's own `unshare --net --map-root-user` sandbox could create its user and network namespaces. The probe exited 0, and the sandbox brought loopback up (the #1925 fix is on main at this sha). Runner cells are reproduced exactly as statusgen derived them.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 | sha256:80409c869ca4 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | pass exit=0 | sha256:118efbcf0809 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestGitLabNotAvailableNoRequest -v && go test ./cmd/repohardenguard/ -run TestGitLabTierGateCouldNotCheck -v` | pass exit=0 | sha256:5486fa9f1ea8 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 4 | `curl -sS -o /dev/null -w '%{http_code}' -H "PRIVATE-TOKEN: $(cat "$(desktoken auditor --forge gitlab --repo "$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+/[^/]+?)(\.git)?$#\1#')")")" "$GITLAB_API_BASE/projects/$(git config --get remote.origin.url \| sed -E 's#.*[:/]([^/]+)/([^/]+?)(\.git)?$#\1%2F\2#')/protected_branches"` | fail exit=3 | sha256:720e2ca544d5 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c 'not available — Premium' docs/adopting-assay-gitlab.md` | pass exit=0 | sha256:53c234e5e847 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers` | pass exit=0 | sha256:3b84b77b4546 | 2026-10-04 | assay-verifier-app[bot] @ fe12ee0c90b9 (on-behalf-of human:ian) (forge-identity) |

`statusgen verifyrun --check` on that witnessed brief (Linux): exit 1, "docs/streams/forge-gitlab/brief-12-gitlab-hardening-reads.md: 4 pass, 2 fail, 0 could-not-run/missing (of 6 Verify rows)". The darwin run in the worktree (statusgen built from fe12ee0c9, trimmed PATH, throwaway HOME) gave `--check` exit 2, "3 pass, 1 fail, 2 could-not-run/missing (of 6 Verify rows)": rows 1 and 2 are check:ci and could not run because darwin has no `unshare --net` (#1800). That darwin witness was discarded, and the brief was restored to its committed content.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | Verify row 1: cd tools/desk && go build ./... && go test ./... | exit 0 | Witness: fail, exit 1. Re-run inside the same sandbox (statusgen netns helper, uid 0 in the user namespace, loopback up): build exit 0; 95 packages ok, including internal/deskkit (163s) and cmd/repohardenguard; 4 red packages, all outside this brief. (a) cmd/desktoken (TestGitLabRotateRefusesBeforeRotatingWhenCustodyDirIsUnwritable, TestGitLabWriteFailureLockout) and cmd/scanloop (TestClassify_UnreadableLocalStateRoutesToJudgment) are permission tests that the sandbox's map-root-user uid 0 bypasses; the same packages run as uid 501 in the same container: ok. (b) cmd/desksourceguard TestDefaultPlatform: "derived platform "linux-arm64" is not one desk-tools publishes", a host-architecture assertion. (c) internal/regression TestReg786FleetHardening: the fixture suite tools/create-fleet-gitlab_test.sh needs jq, which this image does not ship. On darwin the same test alone: exit 0, ok 46.97s (a first solo attempt was killed at its 60s deadline at load average about 29). Host darwin: build exit 0; cmd/repohardenguard ok. A two-package internal/deskkit run timed out at 300s at load average about 40; the test running at the deadline (TestUnionOnlyWidensCalloutFalsePatternTrue) passed alone in 0.03s, and the package is ok in the container. Every package has a green result at this sha, but no single environment here runs the row green exactly as written | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 2 | Verify row 2: the three go test runs in internal/deskkit (TestForgeGitlabGolden, TestForgeGitlabCoverage, TestForgeNoPassthrough), each with -v | exit 0; output contains PASS; the push_rules_premium_gated golden records a 403 classified could-not-check | Witness: pass, exit 0 (first Linux witness pass for this row; #1925 loopback fix). Darwin by hand: exit 0 for each command; 159 PASS lines, 0 FAIL; "reconciles: 55 operations, all covered"; "golden-pinned operation "push_rules_premium_gated" OK". Golden file read directly: one GET of the push_rule route, result null, err "could-not-check: … permission or tier gate (HTTP 403) … push rules are a Premium feature … record the row as `not available — Premium`", not_found false. Mutation leg: the five "GitLab hardening reads" mutants in forge-gitlab-mutations.json were applied through go test -overlay; no tracked file was edited. Results: tier-gate swallow KILLED (push_rules_premium_gated, push_rules_ce_not_found, hardening_read_approvals_ce_404); GitLab serving a GitHub kind KILLED (hardening_read_github_kind_refused, TestHardeningKindRefusedByTheOtherForge); partial list at the page ceiling KILLED (hardening_read_protected_branches_ceiling_refuses); GitHub serving GitLab kinds KILLED (TestForgeGithubGolden/hardening_read_gitlab_kind_refused, TestHardeningKindRefusedByTheOtherForge). The spec's "stop following X-Next-Page" mutant does not compile at this sha ("declared and not used: resp"), so its non-zero exit is a build failure, not a kill. A compiling equivalent (NextPage >= 0) was KILLED (hardening_read_protected_branches_two_pages, _ceiling_refuses) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 3 | Verify row 3: go test TestGitLabNotAvailableNoRequest then TestGitLabTierGateCouldNotCheck in cmd/repohardenguard, each with -v | exit 0; output contains PASS; a not-available row issues zero reads; a tier-403 row is could-not-check, never a value | Witness: pass, exit 0, on Linux and darwin. By hand: exit 0; "--- PASS: TestGitLabNotAvailableNoRequest"; "--- PASS: TestGitLabTierGateCouldNotCheck" with subtests forbidden_403, not_found_404_ce_has_no_route, premium_null_document_is_not_a_value and premium_value_is_checked all PASS | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 4 | Verify row 4: curl of the protected_branches route with the auditor PAT obtained through desktoken auditor --forge gitlab, against GITLAB_API_BASE | 200 | Could not check. Witness on Linux and darwin: fail, exit 3. Run as written with no desktoken resolvable and GITLAB_API_BASE unset, so nothing was minted or rotated and no request left the host; curl rejects a URL with no host part. Neither host has a live GitLab instance or GitLab auditor custody, and provisioning that credential is a human act. The row is unchanged and still carries its check-definition faults (#1794: no --no-rotate; project path derived from a GitHub origin; BSD sed rejects the lazy quantifier) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 5 | Verify row 5: grep -c 'not available — Premium' docs/adopting-assay-gitlab.md | 1 or more | Witness: pass, exit 0. By hand: exit 0, output "2" (template rows push-rules-signed and push-rules-secrets, doc lines 1081-1082) | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |
| 6 | Verify row 6: statusgen --root . --consumers (statusgen built from fe12ee0c9) | exit 0 | Witness: pass, exit 0. By hand: exit 0; "consumers: no brief files in the diff against fe12ee0c90b9e39e78afc291e5b17111844356cb — nothing to corroborate". This pass is vacuous on a merged tree, as on 2026-09-30 and 2026-10-02 | 2026-10-04 assay-verifier-app[bot] (on-behalf-of human:ian) |

Risk-bearing values, re-read at fe12ee0c9. Scope: the hardening-read kinds in the deskkit backends plus the adopter-doc checklist and role rows. The input changes since e1d99484f added no literal on this path.

RISK-VALUE: DERIVED — tier-gate statuses {403, 404} → could-not-check @ tools/desk/internal/deskkit/forge_gitlab.go:2555 — on an edition or role that does not expose a Premium route, GitLab answers 403 (licensed off, or a role wall) or 404 (no such route on CE). In neither case was a settings document observed, so could-not-check is the only honest state. The executed mutant turns three goldens red.
RISK-VALUE: DERIVED — len(raw) == 0 refusal @ tools/desk/internal/deskkit/forge_gitlab.go:2561 — a 2xx with no body is not a document; accepting it would let a caller read nothing as empty settings.
RISK-VALUE: DERIVED — kind-to-forge partition hardeningKindForge @ tools/desk/internal/deskkit/forge.go:1109 — the brief requires one closed vocabulary split into disjoint per-forge halves. The map gives the kind table's five kinds to GitLab and the six earlier kinds to GitHub. Both cross-forge refusal mutants are killed.
RISK-VALUE: DERIVED — main-push-no-one Required = 0 @ docs/adopting-assay-gitlab.md:1075 — GitLab's protected-branches API defines access level 0 as No one (30 Developer, 40 Maintainer, 60 Admin), so 0 is the CE-expressible form of "nobody pushes to main".
RISK-VALUE: DERIVED — main-merge-maintainers and release-tags Required = 40 @ docs/adopting-assay-gitlab.md:1076-1077 — 40 is GitLab's Maintainer access level, which matches each row's stated intent; a wrong value fails closed.
RISK-VALUE: NAMED, NOT DERIVED — auditor minimum role Maintainer (40) for protected-branches, protected-tags, approvals and push-rules; Reporter (20) for project and file @ docs/adopting-assay-gitlab.md:1018-1023 — the doc itself calls the Maintainer rows "a stated minimum, not a measured one". The derivation is Verify row 4's live read-back, which needs a GitLab instance and provisioned auditor custody (#1946, open). A wrong value fails closed (403 → could-not-check).
RISK-VALUE: N/A — page ceilings gitlabMaxHardeningPage = 10 @ tools/desk/internal/deskkit/forge_gitlab.go:2489 and gitlabPerPage = 100 @ tools/desk/internal/deskkit/forge_gitlab.go:432 — reversible operational bounds that fail closed; ranked last, so no derivation is owed.

Findings:

- No implementation defect found. Rows 2, 3, 5 and 6 pass in the Linux witness and by hand. Row 2 passes in a witness for the first time, now that loopback is up in the sandbox (#1925 is closed and the fix is on main). Row 1 fails in the witness on four packages outside this brief, three of them environmental (uid 0 under map-root-user, linux-arm64, no jq in the image). Every package has a green result at this sha somewhere, so row 1 passes by hand only as a composite. Row 4 could not be checked.
- #1794 is still OPEN (labels question, raised-by:desk; last updated 2026-10-01). Row 4 has not been re-authored with --no-rotate and an explicit project path. The witness still executes the credential-minting row on a plain run; this run was safe only because desktoken was absent from PATH in both environments. #1946 (auditor minimum roles stated, not measured) and #1927 (re-authoring tracker) are also open.
- The human act still owed for row 4: someone with access to a GitLab instance provisions a read_api auditor PAT at the documented Maintainer role, runs the protected-branches read-back against an explicit project path, and records the status code.
- Mutation-spec drift: the "stop following X-Next-Page" mutant in forge-gitlab-mutations.json no longer compiles ("declared and not used: resp"). A harness that scores by exit status would count that build failure as a kill. The compiling equivalent was killed by the walk goldens, so coverage is intact, but the spec text needs repair. Filed as medici-finance/assay#2191.
- For a fully green row 1 witness, the remaining environmental gaps are: an image carrying jq; a runner on a published desk-tools platform; and tests guarded against running as uid 0, or a sandbox that does not map to root.

VERIFY: BLOCKED — row 4 (live GitLab read-back under the auditor PAT) could not be checked: it needs a human-provisioned credential and a live instance, and its command still needs the #1794 re-authoring. Linux witness: the tool prints 4/6 pass, 2 fail (row 1 environmental, row 4 blocked), 0 could-not-run; row 6's pass is vacuous on merged main. By hand: rows 2, 3 and 5 pass, row 1 passes as a composite, and row 6 passes only vacuously, so it is held out of the count: 4 of 6 proven. Blocker: medici-finance/assay#1794.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-11 claude-opus-5-5-verifier

Verification-Attestation: medici-finance/assay#2583 run=cd0edf43365dcb22fad69d36e8e916edcab22cfd05a6f003a9a41fdeb75c661d source=e9e48aa37bf9886983e8bee3103aea9815c6e73f brief=docs/streams/forge-gitlab/brief-12-gitlab-hardening-reads.md model=claude-opus-5-5 tier=any

Non-implementer re-run of the Verify table against merged main e9e48aa37bf9886983e8bee3103aea9815c6e73f, on a detached temporary worktree at that sha. The worktree was clean before and after: nothing was edited except this Evidence section, and nothing was committed or pushed. No row called a live GitLab or GitHub endpoint. gate: model; all four risk answers are no. Runner: assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian).

Host: darwin-arm64, go1.27.1, GOPROXY=off, GOTOOLCHAIN=local, KUBECONFIG=/dev/null, GITLAB_API_BASE unset. Every row ran under a throwaway HOME outside the checkout. PATH was trimmed to a scratch bin (statusgen built from this tree), the Go toolchain and the system directories, so no credential-minting verb could be resolved (the #1794 containment). The cell launcher's ambient ASSAY_GO_CACHE_POLICY, ASSAY_SCRATCH_* and DESK_SESSION variables were unset; see the row 1 note.

Why this re-run: the 2026-10-04 receipt at fe12ee0c9 went stale because forge.go, forge_gitlab.go and forge_github.go changed on main (309 commits since; among them #2363, #2423, #2449, #2536 and #2239). Before running any row, the verifier compared the hardening path at fe12ee0c9 and e9e48aa37 byte for byte. These are identical: the GitLab block from gitlabMaxHardeningPage through hardeningWalk (105 lines, which includes the tier-gated kind map, RepoHardeningRead and hardeningGetRaw), the hardeningKindForge map, refuseHardeningKindForForge, the GitHub RepoHardeningRead, and the eleven HardeningReadKind constants. No changed line in those files mentions hardening. Unchanged since fe12ee0c9 as well: cmd/repohardenguard, the adopter doc, and every hardening golden. The literals only moved line numbers.

| # | Command | Expect | Observed (exit + key output line) | Date | Runner |
|---|---------|--------|-----------------------------------|------|--------|
| 1 | Verify row 1: cd tools/desk && go build ./... && go test ./... | exit 0 | As written: exit 1. Build exit 0; 102 packages ok and 3 red. cmd/repohardenguard and cmd/deskdispatch are ok; internal/deskkit is red on one test only. (a) cmd/cellctl TestBashQuoteMatchesBash: "bashQuote("~lead") = "\\~lead", bash says "~lead"". (b) internal/regression TestEvidenceAutomergeRefusal and TestEvidenceAutomergeStep: "unexpected EOF while looking for matching" in the automerge-refusal script, 6 passed and 19 failed. Both come from the trimmed PATH, which resolved macOS bash 3.2.57. With bash 5.3.20 first on PATH: cmd/cellctl ok 46.5s; internal/regression ok 96.0s. (c) internal/deskkit TestGuardStopFlags, subtest "HEARTBEAT exactly at boundary → not stale": "HEARTBEAT stale (age 24h0m0s > 24h0m0s)". The subtest writes the heartbeat 1s inside the 24h window, so it reds whenever the guard reads more than 1s later. It passed alone (ok 1.6s) and failed 8 of 8 under -count=8 at load average 62. That is killswitch code, outside this brief. Every other deskkit test passed in the full run. So the row is red as written, and each red is outside the hardening path: two come from the host shell and one from a timing margin. Every package has a green result at this sha except TestGuardStopFlags, which is green only at low load | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2: the three go test runs in internal/deskkit (TestForgeGitlabGolden, TestForgeGitlabCoverage, TestForgeNoPassthrough), each with -v | exit 0; output contains PASS; the push_rules_premium_gated golden records a 403 classified could-not-check | exit 0 on both runs. 173 PASS lines, 0 FAIL; "--- PASS: TestForgeGitlabGolden", "--- PASS: TestForgeGitlabCoverage", "--- PASS: TestForgeNoPassthrough"; "reconciles: 60 operations, all covered"; "golden-pinned operation "push_rules_premium_gated" OK". Golden file read directly: one GET of the push_rule route, result null, not_found false, err "could-not-check: … permission or tier gate (HTTP 403) … push rules are a Premium feature … record the row as `not available — Premium`". Mutation leg: the spec's control and its five "GitLab hardening reads" mutants, applied through go test -overlay with the spec's own test command; no tracked file was edited. Control killed (27 FAIL). Mutant 39 (swallow a tier-gated 403/404 into an empty document) killed: push_rules_premium_gated and push_rules_ce_not_found FAIL. Mutant 40 (GitLab serves a GitHub kind) killed: hardening_read_github_kind_refused and TestHardeningKindRefusedByTheOtherForge FAIL. Mutant 42 (partial list at the page ceiling) killed: hardening_read_protected_branches_ceiling_refuses FAIL. Mutant 43 (GitHub stops refusing GitLab kinds) killed: github_refuses_every_gitlab_kind FAIL. Mutant 41 as written still does not compile ("declared and not used: resp", assay#2191). Its compiling equivalent was killed: hardening_read_protected_branches_two_pages FAIL. Kills were judged on deskkit and repohardenguard only, because cmd/deskdispatch was red in the baseline under the ambient cache policy (row 1 note) | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3: go test TestGitLabNotAvailableNoRequest then TestGitLabTierGateCouldNotCheck in cmd/repohardenguard, each with -v | exit 0; output contains PASS; a not-available row issues zero reads; a tier-403 row is could-not-check, never a value | exit 0 on both runs. "--- PASS: TestGitLabNotAvailableNoRequest"; "--- PASS: TestGitLabTierGateCouldNotCheck", with subtests forbidden_403, not_found_404_ce_has_no_route, premium_null_document_is_not_a_value and premium_value_is_checked all PASS | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4: curl of the protected_branches route with the auditor PAT obtained through desktoken auditor --forge gitlab, against GITLAB_API_BASE | 200 | Could not check: exit 3, http_code 000. Run as written, after a pre-check that desktoken was unresolvable and GITLAB_API_BASE unset. Nothing was minted or rotated, and no request left the host: "desktoken: command not found", "cat: : No such file or directory", BSD sed "RE error: repetition-operator operand invalid" (twice), "curl: (3) URL rejected: No host part in the URL". This host has no live GitLab instance and no GitLab auditor custody, and provisioning that credential is a human act. The row command is unchanged and still has the #1794 check-definition faults: no --no-rotate, a project path derived from a GitHub origin, and a lazy quantifier that BSD sed rejects | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5: grep -c 'not available — Premium' docs/adopting-assay-gitlab.md | 1 or more | exit 0, output "2" (template rows push-rules-signed and push-rules-secrets, doc lines 1081-1082), on both runs | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6: statusgen --root . --consumers (statusgen built from e9e48aa37) | exit 0 | exit 0 on both runs: "consumers: no brief files in the diff against e9e48aa37bf9886983e8bee3103aea9815c6e73f — nothing to corroborate". The throwaway HOME has no roster, so it printed an advisory roster notice; the command reads only. The pass is vacuous on a merged tree, as on every earlier run | 2026-10-11 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Execution witness (`statusgen verifyrun --brief`, statusgen built from e9e48aa37, same trimmed environment): could not run, exit 2, and no Verify row executed, so no witness table was written. First attempt: "pre-work verifier admission failed; no Verify rows executed: exec: "deskdispatch": executable file not found". Second attempt, with only the deskdispatch shim added to PATH: admission exit 5, "cannot obtain the verifier GitHub App installation token for medici-finance/assay … exec: "desktoken": executable file not found". The witness's own admission needs desktoken to mint the verifier token. With desktoken on PATH, though, the witness would also execute row 4's desktoken auditor --forge gitlab call, the credential-minting row #1794 describes. The verifier stopped there rather than expose that verb or wrap it. Had admission passed, rows 1 and 2 (check:ci) would still have come back could-not-run on this darwin host, which has no unshare --net (#1800). The newest witness on record is therefore the 2026-10-04 Linux witness at fe12ee0c9: 4 pass, 2 fail.

Risk-bearing values, re-read at e9e48aa37. Scope: the hardening-read kinds in the deskkit backends, plus the adopter doc's checklist and role rows. The input changes since fe12ee0c9 added no literal on this path; only the line numbers moved.

RISK-VALUE: DERIVED — tier-gate statuses {403, 404} → could-not-check @ tools/desk/internal/deskkit/forge_gitlab.go:2621 — on an edition or role that does not expose a Premium route, GitLab answers 403 (licensed off, or a role wall) or 404 (no such route on CE). In neither case was a settings document observed, so could-not-check is the only honest state. Mutant 39 turns both push-rule goldens red.
RISK-VALUE: DERIVED — tier-gated kinds {push-rules, approvals} @ tools/desk/internal/deskkit/forge_gitlab.go:2561-2566 — push rules are Premium (edition matrix C5). The approvals route answers 404 on some self-managed CE instances, and its enforcement is Premium (B3/B4). Protected branches, protected tags and project are Free (B1, B12, B5/B6), so a 403 or 404 there correctly stays an ordinary error.
RISK-VALUE: DERIVED — len(raw) == 0 refusal @ tools/desk/internal/deskkit/forge_gitlab.go:2627 — a 2xx with no body is not a document; accepting it would let a caller read nothing as empty settings.
RISK-VALUE: DERIVED — kind-to-forge partition hardeningKindForge @ tools/desk/internal/deskkit/forge.go:1265 — the brief requires one closed vocabulary split into disjoint per-forge halves. The map gives the kind table's five kinds to GitLab and the six earlier kinds to GitHub. Both cross-forge refusal mutants (40, 43) are killed.
RISK-VALUE: DERIVED — main-push-no-one Required = 0 @ docs/adopting-assay-gitlab.md:1075 — GitLab's protected-branches API defines access level 0 as No one (30 Developer, 40 Maintainer, 60 Admin), so 0 is the CE-expressible form of "nobody pushes to main".
RISK-VALUE: DERIVED — main-merge-maintainers and release-tags Required = 40 @ docs/adopting-assay-gitlab.md:1076-1077 — 40 is GitLab's Maintainer access level, which matches each row's stated intent. A wrong value fails closed.
RISK-VALUE: NAMED, NOT DERIVED — auditor minimum role Maintainer (40) for protected-branches, protected-tags, approvals and push-rules, and Reporter (20) for project and file @ docs/adopting-assay-gitlab.md:1018-1023 — the doc itself calls the Maintainer rows "a stated minimum, not a measured one". The derivation is Verify row 4's live read-back, which needs a GitLab instance and provisioned auditor custody (#1946). A wrong value fails closed (403 → could-not-check).
RISK-VALUE: N/A — page ceilings gitlabMaxHardeningPage = 10 @ tools/desk/internal/deskkit/forge_gitlab.go:2555 and gitlabPerPage = 100 @ tools/desk/internal/deskkit/forge_gitlab.go:439 — reversible operational bounds that fail closed; ranked last, so no derivation is owed. Mutant 42 shows the ceiling refuses rather than truncating.

Findings:

- No implementation defect found. The hardening path is byte-identical to the 2026-10-04 receipt. Rows 2, 3 and 5 pass by hand, and every hardening mutant is killed. Row 6 passes only vacuously. Row 1 is red as written, and none of its reds is on this brief's path. Row 4 could not be checked.
- The human act still owed for row 4: someone with access to a GitLab instance provisions a read_api auditor PAT at the documented Maintainer role, runs the protected-branches read-back against an explicit project path, and records the status code. The row also still needs its #1794 re-authoring (--no-rotate, an explicit project path, a portable sed). The current state of #1794, #1946, #1927 and #2191 could not be checked this run: the forge read was refused on this host.
- New: the witness cannot run on darwin without desktoken on PATH, because its own admission mints the verifier token. In this brief that exposes row 4's auditor mint. As long as row 4 calls desktoken without --no-rotate, a hermetic witness here needs either a mint path that admission can reach while row commands cannot, or row 4 re-authored first.
- New, outside this brief: cmd/deskdispatch tests are not hermetic against an ambient ASSAY_GO_CACHE_POLICY. Under a house-cell launch, 10 GitLab claim tests fail with "storage-deferred: deskkit: advisory lock is held by another process", because the child dispatch contends for the shared cell go-cache lock. With the variable unset the package is ok (70.6s alone, 397.6s in row 1).
- New, outside this brief: internal/deskkit TestGuardStopFlags, subtest "HEARTBEAT exactly at boundary → not stale", has a 1s margin, so it fails deterministically under host load (8 of 8 at load average 62).
- cmd/cellctl TestBashQuoteMatchesBash and the internal/regression evidence-automerge tests need bash 4 or later on PATH. Under macOS /bin/bash 3.2 they red; with bash 5.3 they pass.
- Mutation-spec drift persists: mutant 41 in forge-gitlab-mutations.json still fails to compile (assay#2191, filed 2026-10-04). Its compiling equivalent is killed, so coverage is intact.

VERIFY: BLOCKED — row 4 (live GitLab read-back under the auditor PAT) could not be checked: it needs a human-provisioned credential and a live instance, and its command still needs the #1794 re-authoring. The witness could not run: admission needs desktoken, which this run withheld because row 4 would call it. By hand, rows 2, 3 and 5 pass. Row 1 is red as written, and all three reds are outside the brief (host bash 3.2 twice, and a load-sensitive killswitch margin). Row 6 passes vacuously. No implementation defect. Blocker: medici-finance/assay#1794.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-16 claude-opus-5-5 verifier

Verification-Attestation: medici-finance/assay#3136 run=d4af41b96096ee421536b2f2d643323d3182ae8a9670480e0a9acc0f1057bf63 source=1948dc0873705930b202f0e747047df844010d1a brief=docs/streams/forge-gitlab/brief-12-gitlab-hardening-reads.md model=claude-opus-5-5 tier=any

Non-implementer re-run of the Verify table against merged main 1948dc0873705930b202f0e747047df844010d1a, on a detached temporary worktree at that sha. The pre-work admission check passed before any row ran. Nothing was edited except this Evidence section; nothing was committed or pushed. No row called a live GitLab endpoint. gate: model; all four risk answers are no.

Host: darwin-arm64, go1.27.2, git 2.56.0, KUBECONFIG=/dev/null, GITLAB_API_BASE unset; Go rows ran with a throwaway HOME and TMPDIR outside the checkout and the real Go build and module caches passed explicitly.

Why this re-run: the 2026-10-11 receipt at e9e48aa37 is 311 commits behind. Since e9e48aa37 the hardening path changed only in comments: the two changed lines in the deskkit backends that mention hardening are comment text on other page ceilings, the repohardenguard change wraps main in a perf timer and leaves the run function and its tests alone, the adopter-doc changes are outside the checklist template and role rows, and no hardening golden changed. Expected outcomes were written from the brief text before earlier Evidence was read.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | Verify row 1: cd tools/desk && go build ./... && go test ./... | exit 0 | exit 0. Build exit 0; 112 packages ok, 0 FAIL lines, 5 packages with no test files. internal/deskkit ok in 397.9s and cmd/repohardenguard ok in 0.3s on a fully uncached run (that run also set GOFLAGS=-mod=mod; the tree was clean afterwards). A second run exactly as written, without GOFLAGS: exit 0, 112 ok (110 from the test cache). First green run of this row as written on record | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | Verify row 2: the three go test runs in internal/deskkit (TestForgeGitlabGolden, TestForgeGitlabCoverage, TestForgeNoPassthrough), each with -v | exit 0; output contains PASS; the push_rules_premium_gated golden records a 403 classified could-not-check | exit 0. 173 PASS lines, 0 FAIL; "--- PASS: TestForgeGitlabGolden/push_rules_premium_gated" with "golden-pinned operation "push_rules_premium_gated" OK"; "--- PASS: TestForgeGitlabCoverage"; "--- PASS: TestForgeNoPassthrough" and its five subtests. Golden file read directly: one GET of the push_rule route, result null, not_found false, err "could-not-check: … permission or tier gate (HTTP 403) … push rules are a Premium feature … record the row as `not available — Premium`". Every hardening_read golden (project, protected_branches incl. two_pages and ceiling_refuses, protected_tags, push_rules_premium, push_rules_unconfigured_null, push_rules_ce_not_found, approvals, approvals_ce_404, github_kind_refused, unknown_kind) ran and passed | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | Verify row 3: go test TestGitLabNotAvailableNoRequest then TestGitLabTierGateCouldNotCheck in cmd/repohardenguard, each with -v | exit 0; output contains PASS; a not-available row issues zero reads; a tier-403 row is could-not-check, never a value | exit 0. "--- PASS: TestGitLabNotAvailableNoRequest"; "--- PASS: TestGitLabTierGateCouldNotCheck" with subtests forbidden_403, not_found_404_ce_has_no_route, premium_null_document_is_not_a_value and premium_value_is_checked all PASS | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | Verify row 4: curl of the protected_branches route with the auditor PAT obtained through desktoken auditor --forge gitlab, against GITLAB_API_BASE | 200 | Could not check; the command was deliberately held back. Pre-check only: the origin remote is a GitHub URL (no GitLab project to address), GITLAB_API_BASE is unset, and this host holds no GitLab auditor custody. The row needs a live GitLab instance and a human-provisioned read_api auditor PAT, outside the offline envelope. It still calls the credential-minting verb without --no-rotate (medici-finance/assay#1794, open, updated 2026-10-13), so it was not executed. The documented minimum role it would dereference stays unmeasured (medici-finance/assay#1946, open) | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | Verify row 5: grep -c 'not available — Premium' docs/adopting-assay-gitlab.md | 1 or more | exit 0, output "2" (template rows push-rules-signed and push-rules-secrets, doc lines 1109-1110) | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | Verify row 6: statusgen --root . --consumers (statusgen v1.0.45) | exit 0 | exit 0: "consumers: no brief files in the diff against 1948dc0873705930b202f0e747047df844010d1a — nothing to corroborate". The pass is vacuous on a merged tree, as on every earlier run | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

Risk-bearing values, re-read at 1948dc087. Scope: the five GitLab hardening kinds in the deskkit backends (kind constants, endpoint literals, tier-gate classification, page ceilings, kind partition) plus the adopter doc's checklist Required cells and auditor role rows. Ranked: Required cells and tier-gate classification first (a wrong value fail-opens a checklist), endpoint literals and partition next (a wrong value fails closed as a 404 or a refusal), auditor role and page ceilings last (fail closed, reversible by an edit).

RISK-VALUE: DERIVED — main-push-no-one Required = 0 @ docs/adopting-assay-gitlab.md:1103 — GitLab's protected-branches API defines access level 0 as No one (30 Developer, 40 Maintainer, 60 Admin), so 0 is the CE-expressible form of "nobody pushes to main", as the brief's facts state.
RISK-VALUE: DERIVED — main-no-force Required = false @ docs/adopting-assay-gitlab.md:1102 — allow_force_push=false is the brief's per-branch requirement and the B1 control itself.
RISK-VALUE: DERIVED — main-merge-maintainers and release-tags Required = 40 @ docs/adopting-assay-gitlab.md:1104-1105 — 40 is GitLab's Maintainer access level, matching each row's stated intent (B12 for tags); a wrong value fails closed.
RISK-VALUE: DERIVED — tier-gate statuses {403, 404} → could-not-check @ tools/desk/internal/deskkit/forge_gitlab.go:2633 — on an edition or role without the route, GitLab answers 403 (licensed off or a role wall) or 404 (no route on CE); no settings document was observed in either case, so could-not-check is the only honest state. Pinned by push_rules_premium_gated and push_rules_ce_not_found.
RISK-VALUE: DERIVED — len(raw) == 0 refusal @ tools/desk/internal/deskkit/forge_gitlab.go:2639 — a 2xx with no body is not a document; accepting it would let a caller read nothing as empty settings.
RISK-VALUE: DERIVED — endpoint literals "projects/%s", "projects/%s/push_rule", "projects/%s/approvals" @ tools/desk/internal/deskkit/forge_gitlab.go:2553-2555 and "projects/%s/protected_branches", "projects/%s/protected_tags" @ tools/desk/internal/deskkit/forge_gitlab.go:2559-2560 — each matches the brief's kind table one for one (GET /projects/:id, /push_rule, /approvals, /protected_branches, /protected_tags), the GitLab REST routes for those documents; a wrong literal answers 404 and fails closed.
RISK-VALUE: DERIVED — kind-to-forge partition hardeningKindForge @ tools/desk/internal/deskkit/forge.go:1282 — the brief requires one closed vocabulary split into disjoint per-forge halves; the map gives the five GitLab kinds (forge.go:1236-1258) to GitLab and the six earlier kinds to GitHub, and the cross-forge refusal goldens pass.
RISK-VALUE: NAMED, NOT DERIVED — auditor minimum role Maintainer (40) for protected-branches, protected-tags, approvals and push-rules, Reporter (20) for project and file @ docs/adopting-assay-gitlab.md:142 and 1046-1051 — the doc itself says "The Maintainer rows are a stated minimum, not a measured one" (line 1053). The derivation is Verify row 4's live read-back, which needs a GitLab instance and a provisioned auditor credential (medici-finance/assay#1946). A wrong value fails closed (403 → could-not-check).
RISK-VALUE: N/A — page ceilings gitlabMaxHardeningPage = 10 @ tools/desk/internal/deskkit/forge_gitlab.go:2567 and gitlabPerPage = 100 @ tools/desk/internal/deskkit/forge_gitlab.go:440 — reversible operational bounds that refuse rather than truncate (hardening_read_protected_branches_ceiling_refuses passes); ranked last, so no derivation is owed.

Findings:

- No implementation defect found. Rows 1, 2, 3 and 5 pass as written; row 1 is green as written for the first time on record. Row 6 passes only vacuously. Row 4 could not be checked.
- The human act still owed for row 4: someone with access to a GitLab instance provisions a read_api auditor PAT at the documented Maintainer role, runs the protected-branches read-back against an explicit project path, and records the status code. The row also still needs its re-authoring (--no-rotate, an explicit project path, a portable sed): medici-finance/assay#1794 and medici-finance/assay#1927 are open. medici-finance/assay#2191 (mutation-spec drift) is also still open; the mutation leg was not re-run this pass because the hardening path changed only in comments.

VERIFY: BLOCKED — row 4 (live GitLab read-back under the auditor PAT) could not be checked: it needs a human-provisioned credential and a live instance, and its command still needs re-authoring. Rows 1, 2, 3 and 5 pass as written; row 6 passes vacuously. 5 of 6 rows exit 0; 4 of 6 proven. No implementation defect. Blocker: medici-finance/assay#1794.

### Non-implementer verifier re-run — VERIFY: BLOCKED — 2026-10-16 (second pass) claude-opus-5-5 verifier

Verification-Attestation: medici-finance/assay#3183 run=114f5e417145b2eacaf84cc457e02562cdb9050c45f1c165cd5bd083ecb167a3 source=f005cb98440a522cdb23217d440bdca92e5165ce brief=docs/streams/forge-gitlab/brief-12-gitlab-hardening-reads.md model=claude-opus-5-5 tier=any

Non-implementer re-run of the Verify table against merged main f005cb98440a522cdb23217d440bdca92e5165ce, on a detached temporary worktree at that sha. The pre-work admission check passed before any row ran and again after this block was written. Nothing was edited except this Evidence section; nothing was committed or pushed. No row called a live GitLab endpoint. gate: model; all four risk answers are no.

Host: darwin-arm64, go1.27.2, git 2.56.0, KUBECONFIG=/dev/null, GITLAB_API_BASE unset, GIT_CONFIG_GLOBAL unset, GIT_CONFIG_NOSYSTEM=1. Go rows ran with a throwaway HOME and TMPDIR outside the checkout and the real Go build, module and GOPATH caches passed explicitly.

Why this re-run: the earlier 2026-10-16 receipt is at 1948dc087, 34 commits behind. Between 1948dc087 and f005cb98 the only changes on the brief's file set are in the deskkit backends' WriteFile paths (forge_github.go and forge_gitlab.go: the evidence write is split into a shared snapshot helper). The hardening block (endpoint maps, tier-gated kind map, RepoHardeningRead, hardeningGetRaw, hardeningWalk), forge.go, cmd/repohardenguard, the hardening goldens, the adopter doc, inventory.md and edition-matrix.md are unchanged. Expected outcomes were written from the brief text before earlier Evidence was read.

| # | Command | Expect | Observed (exit + key output line) | Date / runner |
|---|---------|--------|-----------------------------------|---------------|
| 1 | Verify row 1: cd tools/desk && go build ./... && go test ./... | exit 0 | Three runs. Run A (uncached, also set GOFLAGS=-mod=mod; tree clean afterwards): exit 0, 112 packages ok, 0 FAIL, internal/deskkit ok 385.4s, cmd/repohardenguard ok 0.6s. Run B exactly as written: exit 1, 111 ok, one red outside this brief: internal/cellcadence "--- FAIL: TestBudgetAndGuard/guard … cadence_test.go:276: context canceled". In isolation with -count=1 that test failed 1 of 5 at load average about 21-31, so it is a timing race in the guard subtest, not on the hardening path. Run C exactly as written: exit 0, 112 ok, 0 FAIL, cellcadence ok 0.977s. internal/deskkit and cmd/repohardenguard were ok in every run | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | Verify row 2: the three go test runs in internal/deskkit (TestForgeGitlabGolden, TestForgeGitlabCoverage, TestForgeNoPassthrough), each with -v | exit 0; output contains PASS; the push_rules_premium_gated golden records a 403 classified could-not-check | exit 0; each of the three runs ends "PASS" and "ok github.com/medici-finance/assay/tools/desk/internal/deskkit"; "--- PASS: TestForgeGitlabGolden/push_rules_premium_gated" with "golden-pinned operation "push_rules_premium_gated" OK". Golden file read directly: one GET of the push_rule route, result null, not_found false, err "could-not-check: GET /projects/medici-finance%2Fassay/push_rule — permission or tier gate (HTTP 403) … push rules are a Premium feature … record the row as `not available — Premium`". Mutation leg (one mutant, through go test -overlay with the mutated file outside the worktree; no tracked file edited): the tier-gated 403/404 branch in hardeningGetRaw changed to return an empty document "{}" with no error. TestForgeGitlabGolden then FAILs on push_rules_premium_gated, push_rules_ce_not_found and hardening_read_approvals_ce_404, so the golden discriminates a decoded 403 | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | Verify row 3: go test TestGitLabNotAvailableNoRequest then TestGitLabTierGateCouldNotCheck in cmd/repohardenguard, each with -v | exit 0; output contains PASS; a not-available row issues zero reads; a tier-403 row is could-not-check, never a value | exit 0. "--- PASS: TestGitLabNotAvailableNoRequest"; "--- PASS: TestGitLabTierGateCouldNotCheck" with subtests forbidden_403, not_found_404_ce_has_no_route, premium_null_document_is_not_a_value and premium_value_is_checked all PASS. The test asserts no "read push-rules" call in the guard's call log and a world with no push-rules document. Under the row 2 mutant this test still passes, as expected: it drives the guard through a stub forge, so it is the second, independent layer the brief's single-point-of-failure line names | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | Verify row 4: curl of the protected_branches route with the auditor PAT obtained through desktoken auditor --forge gitlab, against GITLAB_API_BASE | 200 | Could not check; the command was deliberately held back. Pre-check only: the origin remote is a GitHub URL (no GitLab project to address), GITLAB_API_BASE is unset, and this host holds no GitLab auditor custody. The row needs a live GitLab instance and a human-provisioned read_api auditor PAT, outside the offline envelope, and it still calls the credential-minting verb without --no-rotate (medici-finance/assay#1794, open, updated 2026-10-13). The documented minimum role it would dereference stays unmeasured (medici-finance/assay#1946, open) | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | Verify row 5: grep -c 'not available — Premium' docs/adopting-assay-gitlab.md | 1 or more | exit 0, output "2" (template rows push-rules-signed and push-rules-secrets, doc lines 1109-1110) | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | Verify row 6: statusgen --root . --consumers | exit 0 | exit 0: "consumers: no brief files in the diff against f005cb98440a522cdb23217d440bdca92e5165ce — nothing to corroborate". The pass is vacuous on a merged tree, as on every earlier run | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

Risk-bearing values, re-read at f005cb98. Enumeration scope: the five GitLab hardening kinds in the deskkit backends (kind constants, endpoint literals, tier-gated kind map and status classification, empty-body refusal, page ceilings, kind partition) plus the adopter doc's checklist Required cells and auditor role rows. Ranked: Required cells and tier-gate classification first (a wrong value fail-opens a checklist), endpoint literals and partition next (a wrong value fails closed as a 404 or a refusal), auditor role and page ceilings last (fail closed, reversible by an edit).

RISK-VALUE: DERIVED — main-push-no-one Required = 0 @ docs/adopting-assay-gitlab.md:1103 — GitLab's protected-branches API defines access level 0 as No one (30 Developer, 40 Maintainer, 60 Admin), so 0 is the CE-expressible form of "nobody pushes to main", as the brief's facts state.
RISK-VALUE: DERIVED — main-no-force Required = false @ docs/adopting-assay-gitlab.md:1102 — allow_force_push=false is the brief's per-branch requirement and the B1 control itself.
RISK-VALUE: DERIVED — main-merge-maintainers and release-tags Required = 40 @ docs/adopting-assay-gitlab.md:1104-1105 — 40 is GitLab's Maintainer access level, matching each row's stated intent (B12 for tags); a wrong value fails closed.
RISK-VALUE: DERIVED — tier-gate statuses {403, 404} → could-not-check @ tools/desk/internal/deskkit/forge_gitlab.go:2633 — on an edition or role without the route, GitLab answers 403 (licensed off or a role wall) or 404 (no route on CE); no settings document was observed in either case, so could-not-check is the only honest state. Pinned by push_rules_premium_gated and push_rules_ce_not_found, and the row 2 mutant turns both red.
RISK-VALUE: DERIVED — tier-gated kinds {push-rules, approvals} @ tools/desk/internal/deskkit/forge_gitlab.go:2573 — push rules are Premium (edition matrix C5); the approvals route answers 404 on some self-managed CE instances and its enforcement is Premium (B3/B4). project, protected-branches and protected-tags are Free (B1, B5/B6, B12), so a 403 or 404 there stays an ordinary could-not-check error without a tier note.
RISK-VALUE: DERIVED — len(raw) == 0 refusal @ tools/desk/internal/deskkit/forge_gitlab.go:2639 — a 2xx with no body is not a document; accepting it would let a caller read nothing as empty settings.
RISK-VALUE: DERIVED — endpoint literals "projects/%s", "projects/%s/push_rule", "projects/%s/approvals" @ tools/desk/internal/deskkit/forge_gitlab.go:2553-2555 and "projects/%s/protected_branches", "projects/%s/protected_tags" @ tools/desk/internal/deskkit/forge_gitlab.go:2559-2560 — each matches the brief's kind table one for one (GET /projects/:id, /push_rule, /approvals, /protected_branches, /protected_tags); a wrong literal answers 404 and fails closed.
RISK-VALUE: DERIVED — kind-to-forge partition hardeningKindForge @ tools/desk/internal/deskkit/forge.go:1282 — the brief requires one closed vocabulary split into disjoint per-forge halves; the map gives the five GitLab kinds to GitLab and the six earlier kinds to GitHub, and TestForgeNoPassthrough plus the github_kind_refused golden pass.
RISK-VALUE: NAMED, NOT DERIVED — auditor minimum role Maintainer (40) for protected-branches, protected-tags, approvals and push-rules, Reporter (20) for project and file @ docs/adopting-assay-gitlab.md:142 and 1046-1051 — the doc itself says "The Maintainer rows are a stated minimum, not a measured one" (line 1053). The derivation is Verify row 4's live read-back, which needs a GitLab instance and a provisioned auditor credential (medici-finance/assay#1946). A wrong value fails closed (403 → could-not-check).
RISK-VALUE: N/A — page ceilings gitlabMaxHardeningPage = 10 @ tools/desk/internal/deskkit/forge_gitlab.go:2567 and gitlabPerPage = 100 @ tools/desk/internal/deskkit/forge_gitlab.go:440 — reversible operational bounds that refuse rather than truncate (hardening_read_protected_branches_ceiling_refuses passes); ranked last, so no derivation is owed.

Findings:

- No implementation defect found. Rows 2, 3 and 5 pass as written. Row 1 passed in two of three runs; the one red was a timing race in internal/cellcadence TestBudgetAndGuard/guard, outside this brief. Row 6 passes only vacuously. Row 4 could not be checked.
- New, outside this brief: internal/cellcadence TestBudgetAndGuard/guard fails intermittently with "context canceled" at cadence_test.go:276 (1 of 5 isolated runs at load average about 21-31). No existing issue found by a search for the test name.
- The human act still owed for row 4: someone with access to a GitLab instance provisions a read_api auditor PAT at the documented Maintainer role, runs the protected-branches read-back against an explicit project path, and records the status code. The row also still needs its re-authoring (--no-rotate, an explicit project path, a portable sed): medici-finance/assay#1794 and medici-finance/assay#1927 are open. medici-finance/assay#2191 (mutation-spec drift) is also still open.
- Brief text drift, no effect on any row: the brief names the operation "op 38"; inventory.md records it as op 40 (renumbered because 38 was taken), and the GitLab column of that row reads implemented.

VERIFY: BLOCKED — row 4 (live GitLab read-back under the auditor PAT) could not be checked: it needs a human-provisioned credential and a live instance, and its command still needs re-authoring. Rows 2, 3 and 5 pass as written; row 1 passed in two of three runs (the red was an unrelated timing race); row 6 passes vacuously. 5 of 6 rows exit 0; 4 of 6 proven. No implementation defect. Blocker: medici-finance/assay#1794.

### Independent run 2026-10-24 (claude-opus-5-5, strong) — container-netns admission bracket plus host row

Separate run under a fresh admission (attestation medici-finance/assay#4295). It credits nothing from the runs above. Container rows 1, 2, 3 and 5 ran in one closed admission bracket; row 6 ran on the host because it reads git history; row 4 was not selected (reason below).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go build ./... && go test ./...` | fail exit=1 sandbox=container-netns | sha256:56ae93d600b1 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./internal/deskkit/ -run TestForgeGitlabGolden -v && go test ./internal/deskkit/ -run TestForgeGitlabCoverage -v && go test ./internal/deskkit/ -run TestForgeNoPassthrough -v` | pass exit=0 sandbox=container-netns | sha256:0bd55cd6b562 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/repohardenguard/ -run TestGitLabNotAvailableNoRequest -v && go test ./cmd/repohardenguard/ -run TestGitLabTierGateCouldNotCheck -v` | pass exit=0 sandbox=container-netns | sha256:ec1cad003216 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
| 5 | `grep -c 'not available — Premium' docs/adopting-assay-gitlab.md` | pass exit=0 sandbox=container-netns | sha256:53c234e5e847 | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104
Container-Image: sha256:c2e32c7ff99f67951174e8ff835d928d8999b1bb2ecdc7c63d3775528178640e
Verification-Attestation: medici-finance/assay#4295 run=e4e9d5170f18923aa1d5dbabe11e7384f0098ed7e22f495f69153640c6eea9c9 source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/forge-gitlab/brief-12-gitlab-hardening-reads.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=e4e9d5170f18923aa1d5dbabe11e7384f0098ed7e22f495f69153640c6eea9c9 source=385cb29e5682dcb9a886b298076f722b4823ec1b tree=sha256:98685d6717c495aecd71bc3b8cb782aa9cc7dc0ac1332b029f60eb2c3a1c1104 witness=sha256:cffa6ef21f36e9fdc846a8099550927defce4bf67c3d6ffd4ed1db3a44fdcc89 opened=2026-10-24T17:12:43Z closed=2026-10-24T17:53:06Z before=receipt after=check-verifier

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 6 | `statusgen --root . --consumers` | pass exit=0 | sha256:f61c66ccf12b | 2026-10-24 | assay-verifier-app[bot] @ 385cb29e5682+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4295 run=e4e9d5170f18923aa1d5dbabe11e7384f0098ed7e22f495f69153640c6eea9c9 source=385cb29e5682dcb9a886b298076f722b4823ec1b brief=docs/streams/forge-gitlab/brief-12-gitlab-hardening-reads.md model=claude-opus-5-5 tier=strong

Host and lane. darwin-arm64 host, Docker 29.8.2 (6 CPU), statusgen v1.0.59 as installed. Rows 1, 2, 3 and 5 ran in the pinned verify-runner v1.0.59 image (digest above, linux/amd64 under emulation, go1.27.2) with no network, a read-only root and the checkout mounted read-only. Row 6 ran on the host because it reads git history, which the container mount does not carry.

Row 4 was not selected. It needs a live GitLab instance and an auditor personal access token that a human provisions, and its command mints that token through desktoken. No GitLab API base is configured for this run and the checkout's origin is a GitHub remote. The row's command is still open for re-authoring (medici-finance/assay#1794, re-authoring tracker medici-finance/assay#1927), and the auditor's minimum roles are still stated rather than measured (medici-finance/assay#1946). The row's claim is unproven.

Row 1 is red in this lane, and the red is not in the code this brief changed. A diagnostic re-run of the same command under the same container settings (outside the bracket, no credit) reproduced it. Eight command packages failed. Six of them hit the default 10-minute per-package test timeout: the image runs amd64 under emulation on a host shared with other verifier containers. The other failures are tests that are not hermetic. They resolve the forge from the checkout's origin remote or from the roster, which the container does not have ("cannot resolve which forge serves example-org/agents: no ASSAY_REPO_FORGES entry names it, and the origin remote's host is absent, unreadable, ..."). Some also run statusgen lint and get a refused configuration. None of the failing tests is a GitLab hardening read, and the packages that rows 2 and 3 target pass by name in the same bracket. Row 1 could not be checked in this lane. That is not a fail.

Row 6 passes vacuously: 0 corroborated, 0 disproved, 5 unchecked as unchanged since the merge-base.

Supplemental mutation (no credit): a container run of TestForgeGitlabGolden under a compile overlay. The overlay makes a tier-gated 403 or 404 in the hardening read return an empty JSON document with no error. The test went red in three subtests (push_rules_premium_gated, push_rules_ce_not_found and hardening_read_approvals_ce_404) and exited 1. The unmutated test passes, as row 2 shows. So the golden test catches the tier-gated could-not-check path being turned into a silent empty read.

Risk values at this head:
- Hardening read paths projects/%s, projects/%s/push_rule, projects/%s/approvals, and the list paths projects/%s/protected_branches and projects/%s/protected_tags (forge_gitlab.go:2601-2608): DERIVED. These are GitLab's REST v4 resource paths, and the golden test in row 2 pins them.
- gitlabMaxHardeningPage = 10 (forge_gitlab.go:2615) and gitlabPerPage = 100 (forge_gitlab.go:465): NAMED. These are reversible ceilings that fail closed past the cap, so they rank last.
- Tier-gated kinds {push-rules, approvals} (forge_gitlab.go:2621), a 403 or 404 on them mapped to could-not-check (forge_gitlab.go:2680-2681), and an empty body refused (forge_gitlab.go:2687): DERIVED from GitLab's tier matrix. Rows 2 and 3 exercise them.
- hardeningKindForge partition (forge.go:1360): DERIVED, a closed vocabulary that the coverage test checks.
- Adopter-doc Required cells: main-no-force false, main-push-no-one 0, main-merge-maintainers 40, release-tags 40 (adopting-assay-gitlab.md:1119-1122). DERIVED: GitLab access level 0 is No one and 40 is Maintainer. The push-rules rows are not available — Premium (adopting-assay-gitlab.md:1126-1127), which row 5 counts.
- Auditor roles Reporter (20) and Maintainer (40) (adopting-assay-gitlab.md:142, role table near :1063-1068): NAMED, NOT DERIVED. The doc says so itself at :1070 ("a stated minimum, not a measured one"). Tracked as medici-finance/assay#1946.

Findings: the cmd test packages under tools/desk are not hermetic. They read the ambient origin remote and roster, so row 1 cannot go green in the sanctioned no-network container lane. Under amd64 emulation, the default package timeout is also too short for several packages. Neither finding touches this brief's implementation.

**VERIFY: BLOCKED** — row 4 (live GitLab read-back under a human-provisioned auditor PAT) could not be checked, and its command still needs the medici-finance/assay#1794 re-authoring. In the closed container bracket, rows 2, 3 and 5 pass. Row 1 is red for lane reasons (non-hermetic command tests, and package timeouts under emulation), not for anything in this brief. Row 6 passes vacuously. No implementation defect. Blocker: medici-finance/assay#1794.


## Review
Gate: model (from frontmatter). Reviewer records verdict + date in the stream README table.
Reviewer answers: with the checklist's `not available` short-circuit removed, does the backend's
403 classification alone still keep a Premium row from reading as a value (negative path on the
layered design)?
