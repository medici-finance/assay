---
id: DR-release-by-merge
date: "2026-09-22"
title: "Make the human MERGE of a prepared release PR the release cut and the authorizer, instead of a maintainer typing `gh workflow run release.yml -f version=…` or pushing a bare vX.Y.Z tag"
consequence: major
decided-by: "human:<name>"
alternatives:
  - "Status quo — keep the two manual entry points: a maintainer types `gh workflow run release.yml -f version=vX.Y.Z` (the workflow_dispatch path, which records the dispatch actor as authorizer) or pushes a bare `vX.Y.Z` tag by hand (the tag-push path, which records `not recorded` — the iso-9001/04 second-class path). Ruled against: the cut is a hand-typed command with no reviewable diff, and the tag-push path names no authorizer, so the honest answer to `who authorized this release` is either `read the Actions run history` or `nobody`."
  - "Re-dispatch release.yml from a merge-detector via `workflow_dispatch` (GITHUB_TOKEN + `actions: write`): a workflow on push to main detects the release-PR merge and calls the release workflow's dispatch path. Ruled against: the dispatch path records the DISPATCHING identity as the authorizer, not the human who merged — so it does not close the traceability gap for the human — and it needs the broad `actions: write` grant (which also cancels runs and deletes logs repo-wide). It also bounces one workflow run into another rather than keeping the tag the single source."
  - "Keep `deskrelease cut` cutting only the umbrella `assay/vX.Y.Z` and add a separate `cut-a-plain-tag` step the desk runs: ruled against — a `vX.Y.Z` tag created/pushed with the workflow's GITHUB_TOKEN does NOT trigger release.yml (GitHub's recursion guard; release.yml's own header states this), so it would build nothing; and it reintroduces a hand-run step, which is exactly what the driver asked to remove."
accepted:
  - "A prepared RELEASE PR is the unit of a cut. It carries the version bump `plugins/assay/scripts/stamp-plugin-version.sh stamp vX.Y.Z` writes (the seven stamped files), the aggregated changelog, and a machine-readable marker: the PR title `release: vX.Y.Z` and a `RELEASE: vX.Y.Z` line in the body. The desk authors it (a `deskrelease propose vX.Y.Z` verb, or the documented worker flow); it creates no tag and dispatches no workflow."
  - "The human MERGE is the cut, the human gate, and the authorizer. Under branch protection GitHub records `merged_by.login`; the merge is a reviewable act on a reviewable diff. This is the single control between a candidate version and a live release."
  - "A new `release-on-merge.yml` (on `push: branches: [main]`) detects that the pushed commit is the merge of a release PR (a MERGED PR whose `merge_commit_sha` equals the pushed commit, with the anchored title/body marker) and creates the umbrella `assay/vX.Y.Z` then the plain `vX.Y.Z` tag AT THAT COMMIT — via the git-data API with a GitHub App installation token, because an App token is a distinct identity whose tag creation DOES trigger release.yml, whereas the workflow's GITHUB_TOKEN would not. This job's own GITHUB_TOKEN stays read-only; it never takes `actions: write`."
  - "release.yml's tag-push path resolves the authorizer from the tag's commit → the merged release PR → `merged_by.login`, and REFUSES to build a release for any `v*` tag whose commit is not a merged release PR. This closes iso-9001/04's tag-push traceability gap for the merge-cut path and adds an independent second layer: even a stray or hand-cut `vX.Y.Z` tag, whatever identity created it, builds nothing."
  - "The release-cutter identity is a GitHub App scoped to `contents: write` on this repo only — no `workflows`/`actions`/`administration`/`checks`/`secrets` write, and no ruleset bypass (tag creation is unrestricted by `protect-release-tags`, which forbids only update/deletion). Open sub-decision, deferred to the wiring brief's human gate: whether to provision a DEDICATED release-cutter App (scope-clean, recommended) or reuse the existing `assay-board-writer` App (already a CI secret, but overloading a generated-file-writer identity with release-tag authority). Both are compatible with this record."
  - "`deskrelease cut` is retained UNCHANGED as the manual break-glass path; its tag grammar and guards are not touched. The two workflow files land via the sanctioned staged-copy route (`ci/staged-workflows/`) and a maintainer promote, until desk-supervision/11–12 retire that route in favour of the workflow-App PR path — this record does not depend on that retirement."
---

**Status:** proposed

**This record is NOT approved.** No design-approval ruling has been recorded. It is proposed
for the driver (`human:<name>`) to weigh. While it is proposed, the citing brief (iso-9001/07)
carries status `blocked` (lifecycle-v1.md §2.0 — "MUST NOT be offered" in Next-up), not `todo`
or `implemented`: that reader-checkable board signal, not this sentence, is what holds the brief
until a ruling is recorded here and `decided-by` is confirmed. The staged implementation on the
brief's PR is complete and reviewable; ratifying this record (and provisioning the release-cutter
App, and promoting the two staged workflow files) is the human act that unblocks the move to
`implemented`.

## The problem

Today a release needs a human to TYPE `gh workflow run release.yml -f version=vX.Y.Z` or push a
bare `vX.Y.Z` tag. `deskrelease cut` can only cut the umbrella `assay/vX.Y.Z` tag, which does
not match release.yml's `push: tags: ['v*']` trigger and so builds nothing. The driver wants the
whole cut to be something the desk AUTHORS and the human MERGES — no `gh` typing — with the merge
recorded as the ISO 8.6 release authorization.

## Why an App token is load-bearing

release.yml's header states, as repo canon, that a tag pushed by a workflow using the default
GITHUB_TOKEN does NOT trigger other workflows (GitHub's recursion guard) — which is precisely why
the tag is cut INSIDE the dispatchable release.yml today rather than by a separate cut-a-tag
workflow. A merge-detector that created the `vX.Y.Z` tag with GITHUB_TOKEN would hit the same
guard: the tag would appear and build nothing, silently. A GitHub App installation token is a
distinct identity, so a tag it creates DOES trigger release.yml — and the App credential is
already proven in this pipeline (release.yml's `changelog-roll` job mints the `assay-board-writer`
App to make a push GITHUB_TOKEN cannot). The tag-push path, driven by an App token, is what lets
release.yml resolve the authorizer from the merged PR's `merged_by.login`: the human.

## Defense in depth

The single control is the release-PR marker plus the merger's identity — the merge. Behind it,
three independent layers, each failing on a different signal in a different component: the
`protect-release-tags` ruleset's server-side tag immutability (no bypass actors); release.yml's
refusal to build a release for any `v*` tag whose commit is not a merged release PR; and the
existing gated `release` environment's required-reviewer approval on the publish job. None is the
marker, and none shares its failure signal.
