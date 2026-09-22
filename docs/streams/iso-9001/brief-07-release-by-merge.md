---
brief: assay:assay:iso-9001:07
title: Release by merge — the human merge is the cut and the authorizer
why: >-
  iso-9001/04 recorded the authorizing human in the release body, but only on the
  workflow_dispatch path; on the tag-push path it emits an explicit "not recorded" because
  the tag pre-exists the run and there is no dispatch actor. Today a release still needs a
  human to TYPE `gh workflow run release.yml -f version=vX.Y.Z` or push a bare `vX.Y.Z` tag,
  and `deskrelease cut` only cuts the umbrella `assay/vX.Y.Z` tag, which does NOT match
  release.yml's `push: tags: ['v*']` trigger and so builds nothing. The maintainer wants the
  whole cut to be something the desk AUTHORS and the maintainer MERGES — no `gh` typing. This
  brief makes the merge of a prepared release PR the cut: a new `release-on-merge.yml` detects
  the merge and creates the tags, and release.yml's tag-push path resolves the authorizer from
  the merged PR's `merged_by.login` — closing iso-9001/04's tag-push gap for exactly this path
  and making the human's merge the recorded ISO 8.6 release authorization.
wave: 2
depends: ["iso-9001/04"]
unblocks: []
effort: M
exec-tier: strong
exec-tier-why: >-
  (b) — correctness is a cross-artifact argument spanning three trust boundaries (a PR marker,
  a GitHub App credential's scope, and release.yml's tag-push gate) against the highest-
  consequence file in the repo: a release workflow that fires on the wrong tag, or a tag
  created without a recorded authorizer, cuts an irreversible release, and the guard's own rule
  is that a published tag is never moved.
gate: human
gate-why: >-
  It changes HOW releases are cut — an irreversible tag-creation pathway — and it depends on
  provisioning a GitHub App identity that can CREATE release tags in CI (a credential-scope
  decision GitHub reserves for a signed-in human). Once promoted and wired, merging a release
  PR cuts an irreversible tag; the human is authorizing both the mechanism and, on every future
  release, the cut itself by merging.
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
design: DR-release-by-merge
decision-trigger: creation
issues: []
schema: brief-v2
authored: 2026-09-22 (authored for the iso-9001 board)
sources:
  - "`.github/workflows/release.yml`, header (the `TWO WAYS IN` / `WHY THE TAG IS CUT INSIDE THIS WORKFLOW` block) — states as repo canon that a tag pushed by a workflow using the default GITHUB_TOKEN does NOT trigger other workflows (GitHub's recursion guard), which is why the tag is cut inside this dispatchable workflow rather than by a separate cut-a-tag workflow that expects this one to fire on the resulting tag."
  - "`.github/workflows/release.yml`, trigger block — `on: push: tags: ['v*']` and `workflow_dispatch`. Only a plain `v*` tag builds; the umbrella `assay/vX.Y.Z` does not match."
  - "`.github/workflows/release.yml`, `resolve` job tag-push path — emits `authorizer` as `not recorded (tag-push path: the tag pre-exists the run, so there is no dispatch actor)`. This is the iso-9001/04 gap this brief closes for the merge-cut path."
  - "`.github/workflows/release.yml`, `Create and push the dispatched tag` step — the dispatch path ALREADY pushes `refs/tags/$TAG` (a plain `vX.Y.Z`) with `GH_TOKEN: ${{ github.token }}` via a per-command `http.extraheader` basic credential. Proof that GITHUB_TOKEN CAN create a `v*` tag here; the recursion guard is why that push does not re-trigger this workflow."
  - "`.github/workflows/release.yml`, `changelog-roll` job — mints the `assay-board-writer` App token via `actions/create-github-app-token` from repo secrets `BOARD_APP_ID` / `BOARD_APP_PRIVATE_KEY` (App perms: contents:write on this repo only) to make a push GITHUB_TOKEN cannot. Proof that an App-installation-token credential is already available in this pipeline; an App-token tag creation is a distinct identity and DOES trigger downstream workflows."
  - "`tools/desk/cmd/deskrelease/cut.go` — `tagPattern` `^(assay|desk-tools|statusgen)/v[0-9]+\\.[0-9]+\\.[0-9]+$` and `mainRef = heads/main`; `deskrelease cut` creates only the namespaced umbrella tag via the GitHub git-data API as the desk App, at origin/main HEAD. It does not cut the plain `vX.Y.Z` that release.yml builds on."
  - "`plugins/assay/scripts/stamp-plugin-version.sh` — the seven files a cut stamps to the umbrella version (`stamp <vX.Y.Z>`), and `paths` prints them for a path-scoped `git add`: `plugins/assay/.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `plugins/assay/paired-versions.yaml`, `plugins/assay/.codex-plugin/plugin.json`, and the three Header files (`plugins/assay/hooks/resident-rules.payload.txt`, `plugins/assay/codex/AGENTS-assay.md`, `plugins/assay/cursor/assay.mdc`)."
  - "`ci/staged-workflows/README.md` — no bot or App in this project holds workflow-push permission and GitHub hard-rejects an App push to `.github/workflows/*`; a prepared workflow lands staged and a maintainer promotes it. The sanctioned way this brief's two workflow files land today."
  - "`docs/streams/desk-supervision/brief-10-workflow-app-wiring.md` — the workflow App (workflows:write) provisioning gate. This brief does NOT depend on it: creating a TAG needs contents:write, not workflows:write, and the staged-copy route lands the workflow FILES today."
  - "GitHub docs, GITHUB_TOKEN + `Triggering a workflow from a workflow` (docs.github.com/actions) — push/tag events authorized by GITHUB_TOKEN do not create a new workflow run; workflow_dispatch/repository_dispatch via GITHUB_TOKEN do (post-2022-09, needs actions:write); a GitHub App installation token or a PAT is a distinct identity whose push/tag DOES trigger downstream workflows."
  - "freshness-checked 2026-09-22 @ origin/main — `git grep -n 'not recorded' .github/workflows/release.yml` returns the tag-push authorizer line; `test -f .github/workflows/release-on-merge.yml` is absent."
version: 1
id: 843f8df0-5723-4f26-a653-a1e508def1fa
---

# Brief 07 — Release by merge

## Context

single-point-of-failure: the release-PR marker (`release: vX.Y.Z` title + a `RELEASE: vX.Y.Z`
body line) together with the merger's identity — the merge IS the human gate and the release
authorizer. Behind it, three INDEPENDENT layers, each failing on a different signal in a
different component: (1) the `protect-release-tags` ruleset (server-side, `bypass_actors: []`)
makes an existing tag immutable — creation is allowed but update/deletion/non-fast-forward are
refused at origin, so a tag can never be silently re-pointed; (2) release.yml's tag-push path
REFUSES to build a release for any `v*` tag whose commit is not a merged release PR — so a
stray or hand-cut `vX.Y.Z` tag (whatever identity created it) builds nothing; (3) the existing
`release` environment's required-reviewer approval still gates the publish job — a second human
touchpoint, in a different place, independent of the merge. None of the three is the merge
marker, and none shares its failure signal.

files:
- **new (staged)** `ci/staged-workflows/release-on-merge.yml` — the merge-detect-and-tag
  workflow. Lands under `.github/workflows/` only by a maintainer promote (no App holds
  workflow-push permission; GitHub hard-rejects an App push to `.github/workflows/*`).
- **edit (staged)** `ci/staged-workflows/release.yml` — a staged twin of the LIVE release.yml,
  re-based on it, adding ONLY the tag-push-path authorizer resolution + unmarked-tag refusal in
  the `resolve` job. Additive; no step reordered.
- **edit** `ci/staged-workflows/README.md` — the entries describing both files and their
  one-time maintainer promote.
- **new** `tools/desk/internal/deskkit/release_on_merge_test.go` — a source-coupling test in
  the shape of `release_authorizer_test.go`, reading the STAGED workflow files (the live files
  do not carry the wiring until promotion) and reddening if the release-by-merge wiring is
  dropped, with a mutation control proving every guarded string is caught.
- **specify only (task, not implemented here)** `deskrelease propose vX.Y.Z` — the desk verb
  that opens the release PR. Left as a task because opening a PR is a forge-write flow larger
  than this brief's diff; the interim is the documented worker flow below.

facts:
- **A GITHUB_TOKEN tag push does not trigger release.yml.** release.yml's own header states it
  and GitHub documents it: an event authorized by the workflow's GITHUB_TOKEN does not create a
  new workflow run (the recursion guard). So a `vX.Y.Z` tag created here with GITHUB_TOKEN would
  appear and build NOTHING — the exact silent failure the header warns against. The tag MUST be
  created with a GitHub App installation token (a distinct identity whose tag creation DOES
  trigger release.yml). GITHUB_TOKEN can create the tag (permission-wise: the dispatch path
  already does, and `protect-release-tags` allows creation) — it just cannot make it fire the
  build.
- **The App credential already exists in this pipeline.** release.yml's `changelog-roll` job
  mints the `assay-board-writer` App (`secrets.BOARD_APP_*`, contents:write) to make a push
  GITHUB_TOKEN cannot. `release-on-merge.yml` mints an App token the same way and creates the
  tag through the git-data API — the same mechanism `deskrelease cut` uses as the desk App.
  This job's own GITHUB_TOKEN stays READ-ONLY (contents + pull-requests: read); it never needs
  `actions: write` (which would let it cancel runs and delete logs repo-wide), because it
  creates a tag, it does not dispatch a workflow.
- **Why the tag-push path, not a re-dispatch.** Re-dispatching release.yml via
  `workflow_dispatch` (GITHUB_TOKEN + actions:write) would run the DISPATCH path, which records
  the dispatching identity as the authorizer — not the human who merged — and needs the broad
  `actions: write` grant. The tag-push path with an App token lets release.yml resolve the
  authorizer from the merged PR's `merged_by.login`: the human. This is the design the merge-as-
  authorizer requirement forces.
- **Only the plain `vX.Y.Z` tag builds.** The umbrella `assay/vX.Y.Z` does not match
  `push: tags: ['v*']`, so it is a silent namespace marker (the plugin/paired-versions grammar).
  `release-on-merge.yml` creates BOTH at the same merge commit — umbrella first (silent), plain
  second (fires the build) — so one merge is one atomic cut of both tags at the reviewed commit,
  with no separate `deskrelease cut` to type. `deskrelease cut` is retained UNCHANGED as the
  manual break-glass path; its grammar and guards are not touched.
- **The version bump is what the cut stamps today.** The release PR carries exactly the seven
  files `stamp-plugin-version.sh stamp vX.Y.Z` rewrites (see sources), plus the changelog. This
  is the same stamp the dispatch path performs before tagging; here it is committed in the PR so
  the merge commit already carries the stamped version, and release.yml's existing
  `stamp-plugin-version gate` passes against the tagged tree.
- **Attacker-shaped text rides in via `env:`.** A login and a PR title/body are user-controlled.
  In release.yml's resolve step and in release-on-merge.yml, they ride in via `env:` and are
  parsed by a python3 helper reading the environment — never a `${{ }}` splice inside `run:`,
  exactly as iso-9001/04 requires for the authorizer and the tag-format gate requires for the
  tag. The marker match is anchored (`^release: vX.Y.Z$`, `^RELEASE: vX.Y.Z$`) so a PR that
  merely mentions a version in prose is not mistaken for a release PR.
- **Do not touch release.yml's ordering.** The build, checksum, tag push and
  draft-then-publish sequence are load-bearing. This brief adds resolution logic to the
  `resolve` job's push path and a refusal; it must not move a step in `release`, and it must not
  weaken the tag-format gate, the tag-immutability guard, or the immutable-asset check.
- **This brief adds no signing.** Recording who merged is not establishing who built. Integrity
  remains sha256-only; the release body's authorized-by line (from iso-9001/04) is reused and
  must still not be worded as a provenance or signature claim.
- **Workflow files are a permission surface.** Neither workflow file can be pushed under
  `.github/workflows/` by any App here; both land staged and a maintainer promotes them. This is
  the sanctioned route today (`ci/staged-workflows/README.md`); it is the subject of an open
  retirement decision (desk-supervision/11–12) but stands until that lands.

## Ground rules
- NEVER git push / trigger workflows / cut a tag / dispatch a release. Feature branch + draft
  PR only. No writes under `.github/workflows/` — the deliverable is the STAGED copies.
- Stop at `implemented` — do not set verified/done, never flip the PR ready.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on this branch (generated, single-
  writer = main CI).
- The App identity that creates release tags is a human provisioning act (a repo-admin decision
  tracked on the PR). Do not assume a specific App is authorized; name the requirement and leave
  the choice to the human.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **`ci/staged-workflows/release-on-merge.yml`** — on `push: branches: [main]`, a single job
   that: (a) resolves the merged PR for the pushed commit via
   `GET /repos/{repo}/commits/{sha}/pulls`, keeping only a MERGED PR whose `merge_commit_sha`
   equals the pushed commit, whose title matches `^release: (vX.Y.Z)$` and whose body carries a
   matching `^RELEASE: vX.Y.Z$` line; (b) exits cleanly (no tag) when the push is not such a
   merge; (c) refuses (exit 1) a release PR with no `merged_by.login`; (d) mints a release-cutter
   App token (`secrets.RELEASE_APP_ID` / `RELEASE_APP_PRIVATE_KEY`) and creates
   `refs/tags/assay/vX.Y.Z` then `refs/tags/vX.Y.Z` at the merge commit via the git-data API,
   treating an already-existing tag as a hard error (never a move). GITHUB_TOKEN read-only.
2. **`ci/staged-workflows/release.yml`** — copy the live file and add, in the `resolve` job's
   tag-push path ONLY: resolve the merged release PR from `github.sha` (via the API, the same
   marker check), emit `authorizer` as `<login> (merged release PR #<n>)`, and REFUSE (exit 1)
   when the tag's commit is not a merged release PR. Add `pull-requests: read` to the `resolve`
   job and `GH_TOKEN`/`REPO`/`SHA` to its step `env:`. Nothing else changes; quote the additions-
   only diff in the PR.
3. **`ci/staged-workflows/README.md`** — add an entry for each file: what it does, that it is
   pending promotion, and the exact maintainer promote command; and the release-cutter App
   provisioning prerequisite (below).
4. **`tools/desk/internal/deskkit/release_on_merge_test.go`** — a source-coupling test reading
   the STAGED files (they are the edit surface; the live files carry no wiring until promotion),
   asserting the release-by-merge wiring is present in each, with a mutation control that removes
   each guarded string in turn and proves every removal is caught. Fail-first evidence in the PR.
5. **Record the coverage boundary beside the test** (D6): the marker + merge is a readable,
   independently re-derived authorization; release.yml's environment approval remains the
   authority if the two ever disagree; the source-coupling test pins PRESENCE of the wiring in
   the staged files, not the live behaviour (which cannot exist until a maintainer promotes).
6. **Board row**: add the iso-9001/07 row to the stream README table, Status `implemented`.

**Release-cutter App — the human prerequisite (repo-admin act, tracked on the PR).** The tag
must be created by a GitHub App installation token, not GITHUB_TOKEN, or release.yml never
fires. The human chooses ONE of:
- **A dedicated release-cutter App** (recommended — scope-clean): contents:write on this repo
  only, no `workflows`/`actions`/`administration`/`checks`/`secrets` write, no ruleset bypass
  (tag creation is unrestricted by `protect-release-tags`). Wire its key as
  `RELEASE_APP_ID` / `RELEASE_APP_PRIVATE_KEY` Actions secrets. This keeps release-cutting a
  distinct identity from board-writing.
- **Reuse the existing `assay-board-writer` App** (`BOARD_APP_*`, already a CI secret, already
  contents:write): fastest, no new provisioning, but overloads a generated-file-writer identity
  with release-tag authority. If chosen, alias `RELEASE_APP_*` to `BOARD_APP_*` (or change the
  staged file's secret names) at promote time.

This choice, and the App install, are the human gate; the staged file names `RELEASE_APP_*`
placeholders so a reviewer sees exactly which secret must exist before promotion.

## The documented worker/desk flow (interim, until `deskrelease propose` lands)

Until the `deskrelease propose vX.Y.Z` verb exists, the release PR is opened by hand and this is
the exact flow (no `gh workflow run`, no bare-tag push):

1. Branch off fresh `origin/main`: `release/vX.Y.Z`.
2. `bash plugins/assay/scripts/stamp-plugin-version.sh stamp vX.Y.Z` — stamps the seven files.
3. `git add $(bash plugins/assay/scripts/stamp-plugin-version.sh paths)` — path-scoped add.
4. Aggregate the changelog highlights for the PR body (the same fragments release.yml's
   `changelog-roll` assembles at release time).
5. Open the PR with title exactly `release: vX.Y.Z` and a body line exactly `RELEASE: vX.Y.Z`.
6. A maintainer reviews and MERGES — the merge is the cut and the recorded authorizer.

`deskrelease propose` (specified, left as a task) automates steps 1–5 as a desk verb: it takes
`vX.Y.Z`, runs `stamp`, path-scoped-adds the stamped files, builds the marker'd PR body, and
opens the PR through the same forge-write path `deskpr create` uses. It creates NO tag and
dispatches NO workflow — the merge does that.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `actionlint ci/staged-workflows/release-on-merge.yml` | exit 0 — the new workflow is valid GitHub Actions YAML |
| 2 | `actionlint ci/staged-workflows/release.yml` | exit 0 — the staged twin is valid |
| 3 | `python3 -c 'import sys,yaml; d=yaml.safe_load(open("ci/staged-workflows/release-on-merge.yml")); assert d["on"]["push"]["branches"]==["main"]; assert d["permissions"]=={"contents":"read","pull-requests":"read"}'` | exit 0 — triggers on push:main and GITHUB_TOKEN is read-only (no actions:write) |
| 4 | `git grep -nF 'create-github-app-token' -- ci/staged-workflows/release-on-merge.yml` | exit 0 — the tag is created with an App token, not GITHUB_TOKEN |
| 5 | `git grep -nF 'RELEASE_APP_ID' -- ci/staged-workflows/release-on-merge.yml` | exit 0 — the release-cutter App secret is named for the reviewer/human |
| 6 | `git grep -nE 'refs/tags/(assay/)?' -- ci/staged-workflows/release-on-merge.yml \| grep -c tags` | exit 0; count ≥ 2 — both the umbrella and plain tags are created |
| 7 | `git grep -nF 'not a merged release PR' -- ci/staged-workflows/release.yml` | exit 0 — the tag-push path refuses a `v*` tag whose commit is not a merged release PR |
| 8 | `git grep -nF 'merged release PR #' -- ci/staged-workflows/release.yml` | exit 0 — the tag-push path emits the authorizer from the merged PR (closing iso-9001/04's tag-push gap) |
| 9 | `git grep -nF 'pull-requests: read' -- ci/staged-workflows/release.yml` | exit 0 — the resolve job can query the PR API |
| 10 | `cd tools/desk && go test ./internal/deskkit/ -count=1 -run TestReleaseByMergeWiringStaged` | exit 0 — the source-coupling test passes against the staged files |
| 11 | `cd tools/desk && go test ./internal/deskkit/ -count=1 -run TestReleaseByMergeWiringMissingIsCaught` | exit 0 — mutation control: every removed guard string is caught |
| 12 | `cd tools/desk && go test ./internal/deskkit/ -count=1 -run TestReleaseAuthorizerStampedFromReleaseWorkflow` | exit 0 — **neighbour row**: iso-9001/04's coupling test over the LIVE release.yml still passes (this brief did not touch the live file) |
| 13 | `git grep -c 'NOT CAUGHT' -- ci/staged-workflows/release.yml` | exit 0; count ≥ 6 — **neighbour row**: the six release-blocking mutation assertions carried into the staged twin are intact |
| 14 | `git diff --stat $(git merge-base refs/remotes/origin/main HEAD)..HEAD -- .github/workflows/` | exit 0; EMPTY — no live workflow file is touched (the deliverable is the staged copies). Base pinned to the merge-base so the row is reproducible as main advances |
| 15 | `cd statusgen && go run . --root .. --lint` | exit 0 — the tree still lints clean |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: human (from frontmatter). This is a core-system change to the supply-chain/release path,
so the reviewer answers BOTH defense-in-depth questions in the verdict: (1) what is the single
control standing between the fault and the damage (a stray tag → a false release), and is it
acceptable? — here the marker+merge, behind the ruleset's tag immutability, release.yml's
refusal of an unmarked tag, and the environment approval; (2) does a Verify row prove a LOWER
layer catches the fault with the marker bypassed? — rows 7/8 prove release.yml refuses a `v*`
tag whose commit is not a merged release PR regardless of who created it. Brief-specific
reviewer questions: (a) does the tag-push authorizer ride in via `env:`, never a `${{ }}`
splice? (b) is the marker match anchored so prose mentioning a version is not mistaken for a
release PR? (c) is this job's GITHUB_TOKEN read-only, with the tag write on the App token? (d)
was any `release` step reordered, or is the release.yml change purely additive to the `resolve`
push path? (e) is the release-cutter App scoped to contents:write with no workflows/actions
write, and is its provisioning left as the human gate? (f) does the source-coupling test read
the STAGED files (so it does not redden the live suite before promotion) and actually redden
when a guard string is dropped?
