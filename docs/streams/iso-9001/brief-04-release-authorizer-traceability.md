---
brief: assay:assay:iso-9001:04
title: Record the authorizing human in the release itself
why: >-
  The release clause asks, in terms, for evidence of conformity with acceptance criteria AND
  traceability to the person or persons authorizing release. The pipeline has the person: the
  release job runs in a gated environment where a named human approves, and on the dispatch
  path the actor's login is already interpolated into the annotated tag message. Neither
  reaches the artifact. The release notes name the binaries and the checksums file and no
  human at all, and on the tag-push path there is no actor record anywhere. So the honest
  answer to "who authorized this release" is "read the Actions run history", which is a
  retention question with a different answer in every organisation. One line in the release
  body, and one source-coupling test so it cannot be dropped, closes it.
wave: 1
depends: ["iso-9001/01"]
unblocks: ["iso-9001/06"]
effort: S
exec-tier: strong
exec-tier-why: >-
  The change is small and the file is the highest-consequence one in the repo: a release
  workflow that fails after the tag push burns a version number irrecoverably, and the guard's
  own rule is that a published tag is never moved.
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-25 (authored for the iso-9001 board)
sources:
  - "`.github/workflows/release.yml`, `resolve` job — `ACTOR: ${{ github.actor }}` is already in scope and is emitted into the annotated tag message on the dispatch path (`assay <version> (dispatched by $ACTOR)`); on the push path the message output is emitted empty. The actor exists; it does not reach the release body."
  - "`.github/workflows/release.yml`, `release` job — `environment: release`, whose comment states the intent plainly: with the environment's required human reviewer set, a canonical release pauses for a named human to approve, converting 'any holder of the shared token can cut a release' into 'a person clicked approve'. The approval is real and lives in the run, not in the artifact."
  - "`.github/workflows/release.yml`, the create-release step — the notes body is built by a small python3 helper and names the binaries, the desk-tools image ref and `checksums.txt`. No human appears in it."
  - "`tools/desk/internal/deskkit/version_test.go` — `TestVersionStampedFromReleaseWorkflow` is the in-tree precedent: a test that reads `.github/workflows/release.yml` and reddens if a required stamp is dropped from it. This brief copies that pattern rather than inventing one."
  - "`docs/iso9001-mapping.md` §'8.4 / 8.6 — supply chain and release control' — the statement of what the release chain does and does not establish, including that integrity is sha256-only with no provenance attestation and no signing anywhere in the pipeline."
  - "The standard-side reading: the release clause wants retained evidence of conformity with the acceptance criteria and traceability to the persons authorizing release. Traceability that exists only in a build system's run history is traceability with someone else's retention policy attached."
  - "The tag-format gate in `resolve` is anchored `^v[0-9]+\\.[0-9]+\\.[0-9]+$` against the whole string, which is what makes the tag safe to concatenate unencoded into REST URLs. Any new value written into the release body is NOT covered by that gate and must ride in via `env:`, never a `${{ }}` splice inside `run:`."
  - "freshness-checked 2026-08-25 @ 6871a3b (origin/main) — `git grep -n authorized-by -- .github/workflows/release.yml` returns nothing; the release body carries no authorizer field."
version: 1
id: 2b9ed68c-8429-444b-9bc0-60c778016773
---

# Brief 04 — release-authorizer traceability

## Context

single-point-of-failure: after this brief, one line in the release body is the durable record
of who authorized a release, and the only thing keeping it there is a source-coupling test.
That is thin on purpose. The heavier record — the environment approval — already exists and
is not replaced by this: the line in the body is a *readable copy* of an authorization that
happened elsewhere, and the deliverable must not be worded as though the body line were the
gate. If the body line and the environment approval ever disagree, the environment is the
authority.

files:
- `.github/workflows/release.yml` (implementation home) — the `resolve` job's outputs and the
  create-release step's notes body.
- `tools/desk/internal/deskkit/` — a source-coupling test in the shape of the existing
  release-workflow stamp test, so the field cannot be dropped silently.

facts:
- **The actor is already resolved; do not add a second source for it.** `resolve` has
  `ACTOR: ${{ github.actor }}` in scope and already writes it into the tag message on the
  dispatch path. Emit it as its own job output and consume that. Two independently derived
  copies of the same fact is the defect this repo's derive-or-diff convention exists to
  prevent.
- **The two paths are different and must not be collapsed.** On the **dispatch** path the
  actor is the person who dispatched the run, and the environment approval names a person too.
  On the **tag-push** path the tag already exists when the workflow starts, so there is no
  dispatch actor for it — the honest value there is the tag's own author, or an explicit
  "not recorded: tag-push path". Do not print the pushing token's identity as though it were
  an authorizer, and do not print an empty field: a blank where an authorizer belongs reads as
  "nobody", which is a stronger claim than the truth.
- **Attacker-shaped text rides in via `env:`.** A login is user-controlled and the tag-format
  gate does not cover it. Pass it as an environment variable into the step and let the python3
  helper read it from the environment the way `RELEASE_TAG` and `DESK_IMAGE_REF` already do.
  Never splice it into a `run:` line.
- **Do not touch the ordering.** The build, the checksum, the tag push and the
  draft-then-publish sequence are load-bearing: a tag that exists without a release burns that
  version number, and assets are immutable so a same-named asset is a hard error rather than a
  clobber. This brief adds a field to a body that is already being constructed; it must not
  move a step.
- **Validate with `dry_run`, never with a tag.** The dispatch input builds and checksums and
  stops before tagging or publishing. A version number spent proving a release-notes edit
  cannot be reclaimed — the guard's own rule is that a published tag is never moved, and the
  recovery is always the next patch version.
- **Why `irreversible: no`.** The deliverable is a reviewable workflow diff on a feature
  branch. The irreversible act is *cutting a release*, which this brief does not do and must
  not do. The distinction is the reason the ground rules below forbid dispatching the
  workflow, and a reviewer should check that no Verify row cuts a tag.
- **Workflow files are a permission surface.** A diff under `.github/workflows/` cannot be
  pushed by a bot installation token that lacks the workflows permission; the push is refused
  outright rather than landing partially. If the write path refuses, that refusal is the
  answer — emit a patch and hand it to a push path that carries the permission. Do not route
  around it with a different tool.
- **This brief does not add signing.** Recording who approved is not the same as establishing
  who built. Integrity here remains sha256-only, with no provenance attestation anywhere in
  the pipeline, and the release notes must not be worded so as to blur the two.

## Ground rules
- NEVER git push / trigger workflows. Feature branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on this branch (generated, single-writer = main CI).
- Never cut a tag and never dispatch the release workflow for a live run; `dry_run` only, and
  only if a human runs it.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. **Emit the authorizer as a `resolve` job output** alongside `tag` and `message`, through
   the same single-line-refusing `emit` helper the job already uses. On the dispatch path it
   is the actor; on the tag-push path it is an explicit not-recorded value, never blank.
2. **Consume it in the create-release step via `env:`** and have the python3 body helper read
   it from the environment, in the same shape as `RELEASE_TAG` and `DESK_IMAGE_REF`.
3. **Add one labelled line to the release notes** naming the authorizing identity and the path
   it came from (dispatch actor, or not recorded on a tag push), plus one sentence stating
   what it does and does not establish: it records who authorized the cut, not who or what
   built the artifact, and the pipeline carries no signature or provenance attestation.
4. **Add the source-coupling test** in the shape of the existing release-workflow stamp test:
   read `.github/workflows/release.yml`, assert the authorizer output is emitted and that the
   create-release step passes it through `env:`. Dropping either must redden the suite.
5. **Record the coverage boundary beside the test** (D6): the body line is a readable copy of
   an authorization that happens in the gated environment; it is not itself a gate, the
   environment is the authority if the two ever disagree, and the tag-push path records no
   authorizer by design.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `git grep -n 'authorized-by' -- .github/workflows/release.yml` | exit 0 — **DEREFERENCE, inverts**: no authorizer field at authoring (2026-08-25 @ `6871a3b`) |
| 2 | `git grep -nF 'dispatched by $ACTOR' -- .github/workflows/release.yml` | exit 0 — **DEREFERENCE**: the existing actor interpolation in the tag message still stands; the new output reuses that source rather than adding a second one |
| 3 | `git grep -nF 'environment: release' -- .github/workflows/release.yml` | exit 0 — **DEREFERENCE**: the gated environment that carries the real approval is untouched by this change |
| 4 | `cd tools/desk && go test ./internal/deskkit/ -count=1 -run TestReleaseAuthorizerStampedFromReleaseWorkflow` | exit 0 — the source-coupling test passes against the edited workflow |
| 5 | `cd tools/desk && go test ./internal/deskkit/ -count=1 -run TestVersionStampedFromReleaseWorkflow` | exit 0 — **neighbour row (rule 17)**: the pre-existing test that reads the same workflow file still passes; the edit did not disturb the stamp it guards |
| 6 | `cd tools/desk && go test ./internal/deskkit/ -count=1 -run TestReleaseAuthorizerStampMissingIsCaught` | exit 0 — **mutation row (rule 16), positive control**: with the authorizer line removed from a fixture copy of the workflow, the coupling test reports failure; with it present, it passes |
| 7 | `git grep -cE -e 'signature' -e 'attestation' -- .github/workflows/release.yml` | exit 0; a non-zero count — the note distinguishing "who authorized" from "who built" is in the workflow source, so the release body cannot be read as a provenance claim |
| 8 | `git grep -c 'NOT CAUGHT' -- .github/workflows/release.yml` | exit 0; a count of **at least 6** — **neighbour row**: the six release-blocking mutation assertions are untouched |
| 9 | `cd tools/desk && go test ./internal/deskkit/ -count=1` | exit 0 — the kit's full suite passes |
| 10 | `cd statusgen && go run . --root .. --lint` | exit 0 — the tree still lints clean |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

### Non-implementer verifier run — VERIFY: BLOCKED — 9/10 pass, 1 could-not-check, 0 fail — 2026-09-23 claude-opus-4-8-verifier

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|------|--------|
| 1 | git grep -n 'authorized-by' -- .github/workflows/release.yml | exit 0 — field present (DEREFERENCE inverts the authoring-time absence) | PASS — exit=0; 2 matches at release.yml:1335 (comment) and :1342 (the python3 body emitting the labelled authorized-by line). Field present. | 2026-09-23 | claude-opus-4-8-verifier |
| 2 | git grep -nF 'dispatched by $ACTOR' -- .github/workflows/release.yml | exit 0 — the existing actor interpolation still stands | PASS — exit=0; match at release.yml:211, the tag message reusing the single actor interpolation. Single actor source reused. | 2026-09-23 | claude-opus-4-8-verifier |
| 3 | git grep -nF 'environment: release' -- .github/workflows/release.yml | exit 0 — gated environment untouched | PASS — exit=0; match at release.yml:790 (the gated release environment). Untouched. | 2026-09-23 | claude-opus-4-8-verifier |
| 4 | cd tools/desk && go test ./internal/deskkit/ -count=1 -v -run 'TestReleaseAuthorizer.*StampedFromReleaseWorkflow' | exit 0 — source-coupling test passes | PASS — exit=0; ran with -v: the `--- PASS` line for the authorizer coupling test (0.00s). Test actually executed. (`go test -list 'TestReleaseAuthorizer.*StampedFromReleaseWorkflow'` selects exactly the one test this row names — the authorizer stamped-from-release-workflow coupling test.) | 2026-09-23 | claude-opus-4-8-verifier |
| 5 | cd tools/desk && go test ./internal/deskkit/ -count=1 -v -run 'TestVersionStamped.*FromReleaseWorkflow' | exit 0 — the pre-existing neighbour stamp test still passes | COULD-NOT-CHECK — exit=0 but `--- SKIP`: the named test SKIPPED — "fixture ../../../../.github/workflows/release-desk.yml not present in this tree". The workflow file in this repo is release.yml; there is no release-desk.yml, so this test does not run. `ok deskkit` exit 0 records a skip, not a run — not a pass. Tracked in #1546. (`go test -list 'TestVersionStamped.*FromReleaseWorkflow'` selects exactly the one test this row names — the version-stamped-from-release-workflow neighbour test.) | 2026-09-23 | claude-opus-4-8-verifier |
| 6 | cd tools/desk && go test ./internal/deskkit/ -count=1 -v -run 'TestReleaseAuthorizer.*StampMissingIsCaught' | exit 0 — mutation positive-control passes | PASS — exit=0; ran with -v: the `--- PASS` line for the missing-authorizer positive-control test (0.00s). Test actually executed. (`go test -list 'TestReleaseAuthorizer.*StampMissingIsCaught'` selects exactly the one test this row names — the stamp-missing-is-caught positive-control test.) | 2026-09-23 | claude-opus-4-8-verifier |
| 7 | git grep -cE -e 'signature' -e 'attestation' -- .github/workflows/release.yml | exit 0; non-zero count | PASS — exit=0; count=2. The who-authorized-vs-who-built note is in the workflow source. | 2026-09-23 | claude-opus-4-8-verifier |
| 8 | git grep -c 'NOT CAUGHT' -- .github/workflows/release.yml | exit 0; count at least 6 | PASS — exit=0; count=7 (>=6). The release-blocking mutation assertions are untouched. | 2026-09-23 | claude-opus-4-8-verifier |
| 9 | cd tools/desk && go test ./internal/deskkit/ -count=1 | exit 0 — full kit suite passes | PASS — exit=0; `ok` deskkit (52.99s), no failures. NB: the suite includes the version-stamp test that skips (row 5); a skip does not fail the suite, so this row's exit-0 expectation holds, but the suite does NOT execute that neighbour guard. | 2026-09-23 | claude-opus-4-8-verifier |
| 10 | cd statusgen && go run . --root .. --lint | exit 0 — tree lints clean | PASS — exit=0; final line `LINT: PASS`. Advisory NOTICEs name this brief, both advisory (lint exit 0): (a) `[risk-files-crossread]` prints on the clean merged tree — the brief answers all four risk questions "no" yet declares path `.github/workflows/release.yml`, which the classifier treats as a security-path trigger, so the hand-written risk answers are flagged for review; (b) `[verify-obligation]` prints when the brief is in the branch diff (it evaluates only a brief whose own file the branch changed, so on a clean checkout of the merged SHA only (a) prints) — the declared paths cross top-level directories, so a cross-component +flow Verify row is owed. Neither gates. | 2026-09-23 | claude-opus-4-8-verifier |

RISK-VALUE (kit §4 — enumerate → rank → derive). The verifier trigger fired: the
diff touches a risk-classed path (.github/workflows/release.yml, a security-path
trigger — see row 10 NOTICE (a)). Enumeration over the diff scope (the `resolve`
job authorizer output + the create-release body helper in
.github/workflows/release.yml, and the source-coupling test in
tools/desk/internal/deskkit/) found no numeric constant/threshold/tolerance/
ratio/timeout/limit introduced or changed; the tag-format gate
`^v[0-9]+\.[0-9]+\.[0-9]+$` and the "at least 6 NOT CAUGHT" count are pre-existing
and not touched by this diff. The values this diff introduces are two
authority-binding string literals:

- RISK-VALUE: DERIVED — authorizer(dispatch-path) = "$ACTOR (dispatch actor)" @ .github/workflows/release.yml:216 — the brief requires reusing the single already-resolved actor source (github.actor, the same one the tag message interpolates) and naming it as the dispatch actor. The literal reuses $ACTOR and labels the path; correct. Reversible (release-body text is editable post-publish; the irreversible act — cutting the release — is out of this diff's scope, brief `irreversible: no`).
- RISK-VALUE: DERIVED — authorizer(tag-push-path) = "not recorded (tag-push path: the tag pre-exists the run, so there is no dispatch actor)" @ .github/workflows/release.yml:239 — the brief requires an explicit not-recorded value on the tag-push path, never blank (a blank reads as "nobody", a stronger claim than the truth) and never the pushing token's identity. The literal is explicit, non-blank, and names no token identity; correct.

Re-run after review: row 5 test skips (fixture points at a missing workflow file) and row 10 NOTICEs recorded. Only row 5 is filed, as #1546. Row 10's `[risk-files-crossread]` NOTICE is NOT filed and stays an open question for the stream owner: are this release-workflow brief's four "no" risk answers right, given it declares .github/workflows/release.yml?

## Review
Gate: model (from frontmatter — all four risk answers no). The `irreversible: no` answer is
the one to check first: it holds only because the deliverable is a workflow diff and no Verify
row cuts a tag or dispatches a live release. Reviewer records verdict + date in the stream
README table. Reviewer questions specific to this brief: (1) does the tag-push path record an
explicit not-recorded value rather than a blank or the pushing token's identity? (2) does the
authorizer ride in via `env:` rather than a `${{ }}` splice inside `run:`? (3) was any step
reordered, or is this purely additive to a body that was already being constructed? (4) does
the release note distinguish "who authorized the cut" from "who built the artifact", and does
it avoid implying signing or attestation that the pipeline does not perform? (5) does the
coupling test actually redden when the field is dropped — not merely pass when it is present?
