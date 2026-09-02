---
brief: forge-neutral/08
title: statusgen forge-aware — init's CI scaffold, auto-flip corroboration, honest claim decay
why: >-
  An adopter on GitLab runs `statusgen init`, gets `.github/workflows/assay-statusgen.yml` and
  nothing else, and ends up with a board that has no single writer at all — while `--lint`
  exits 0 with a NOTICE that reads like a transient authentication problem rather than "this
  will never work here". The auto-flip that advances a gate:model brief to done reads reviews
  by shelling `gh` and composes the reviewer login by appending a literal "[bot]", so on any
  other forge it corroborates nothing. Both are the same defect: a check that degrades quietly
  instead of saying what it could not do.
wave: 4
depends: ["forge-neutral/01", "forge-neutral/02", "forge-neutral/07"]
unblocks: ["forge-neutral/10", "forge-neutral/11"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [349]
schema: brief-v1
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "#349 — statusgen init scaffolds a GitHub-only CI half; a GitLab adopter gets a green lint and no board writer"
  - "docs/streams/forge-gitlab/pilot-report.md D-2 — measured on a live GitLab tracking root: ten created paths, none of them a GitLab CI half; the claim-decay NOTICE quoted verbatim"
  - "docs/streams/forge-neutral/brief-02-forge-qualified-identity.md — the corroboration rule per forge that the auto-flip consumes"
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md — the resolution contract statusgen mirrors; statusgen does not import deskkit"
  - "freshness-checked 2026-09-02 @ deae247 — init.go:84 emits `.github/workflows/assay-statusgen.yml` and init.go has zero GitLab references; autoflip.go:518,528 shell `gh pr view` and `gh api …/pulls/<n>/reviews`; autoflip.go:179-185 composes the reviewer login with a literal `[bot]`; claimdecay.go:43 shells `gh pr list` and :95-97 emits the NOTICE while lint still exits 0"
exec-tier: strong
exec-tier-why: "the auto-flip decides whether a brief advances to done on the strength of a review it corroborates; a corroboration rule that is right on one forge and loose on another advances rows nobody approved (questions b and c)."
domain: complicated
consumers:
  - "statusgen/init.go: fixed-here"
  - "statusgen/autoflip.go: fixed-here"
  - "statusgen/claimdecay.go: fixed-here"
  - "docs/adopting-assay.md: fixed-here (its line 12 self-description and the scaffold's closing next-steps text must name whichever CI half was actually written)"
  - "plugins/assay/skills/install/SKILL.md, plugins/assay/skills/adopt/SKILL.md: follow-up forge-neutral/11 (the install prose and the binary-acquisition step are that brief's; this one only makes `init` able to scaffold the right half)"
  - "docs/streams/forge-neutral/brief-01-forge-resolution-contract.md: out-of-scope (statusgen mirrors the resolution ORDER but does not import deskkit; keeping the two in step is a documented contract, not a shared package)"
---

# Brief 08 — statusgen forge-aware

## Context
files:
- `statusgen/init.go` — the scaffold's file list, the workflow template, the next-steps text.
- `statusgen/autoflip.go` — the review read and the reviewer-login composition.
- `statusgen/claimdecay.go` — the dead-claim decay pass and its NOTICE.
- `docs/adopting-assay.md` — the runbook's self-description at line 12.
- `docs/streams/forge-neutral/gitlab-ci-half.md` (planned) — the two-halves board-writer
  design for a GitLab pipeline, so `init`'s template has a specification behind it.

**Why the risk answers are all `no`.** This brief adds a second CI template, makes two reads
forge-aware, and turns a silent degradation into an honest one. It changes no credential and
no trust decision: what a review must be to count is defined in `forge-neutral/02`'s
corroboration rule under that brief's human gate, and this brief consumes it unchanged. The
one place it could go wrong — an auto-flip that advances a row on a review it did not actually
corroborate — is guarded by Verify rows 6 and 7, both negative-path.

single-point-of-failure: the auto-flip's corroboration is the one control between "a reviewer
approved this at this head" and a row reading `done`. Its second, independent layer is the
existing loud-failure rule — anything it cannot corroborate stays `verified` and fails the run
— which trips on a different signal (an unresolvable read) in a different place (the CI job's
exit status) from the comparison itself.

facts:
- `statusgen init` writes ten paths, of which the only CI file is
  `.github/workflows/assay-statusgen.yml` (`statusgen/init.go:76-85`, emitted at `:84`); the
  template is the const at `:274`, GitHub-Actions-shaped at `:285-293`, shelling
  `gh release download` at `:311,:340` and committing as
  `statusgen@users.noreply.github.com` at `:349`. The closing next-steps text names only that
  path (`:468`). `init.go` contains zero GitLab references.
- `--auto-flip-model` reads reviews through `ghModelFlipSource`
  (`statusgen/autoflip.go:369`): `gh pr view <n> --json headRefOid,state,mergedAt` (`:518`)
  and `gh api --paginate repos/<r>/pulls/<n>/reviews` (`:528`, chosen because *"only this
  endpoint returns `commit_id`"*, `:496`). Commit→PR resolution is `gh api` at `:397` and
  `:477`; local history is plain git at `:372`.
- The reviewer login is `scanEffectiveConfig().RoleBots["reviewer"]` plus a literal `"[bot]"`
  (`statusgen/autoflip.go:179-185`). Repo resolution is the stream's `repo:` frontmatter,
  else `ASSAY_HOME_REPO` (`:210-217`).
- Dead-claim decay shells `gh pr list --state all --limit 1000 --json headRefName,state`
  (`statusgen/claimdecay.go:43`) and, on failure, emits
  *"NOTICE: dead-claim decay unavailable — %v. Open branches of already merged/closed PRs may
  still be consuming their stream's dispatch cap; regenerate with `gh` reachable and
  authenticated to release them."* (`:95-97`). Lint still exits 0. The fail direction is
  documented at `:79-89`: a failed read falls back to the full open-branch set, so decay can
  only ever shrink the claim set.
- On GitLab CE approvals do not reset on push, so the at-head property lives in the note body
  — the rule is `docs/streams/forge-neutral/identity.md` (planned) §corroboration, and the
  pilot's steps A9/B4 are the walked instance.
- statusgen's only network code is `const githubAPIBase = "https://api.github.com"`
  (`statusgen/ghfetch.go:3,45`).

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not widen what counts as a corroborated review. If the configured forge's corroboration
  cannot be established, the row stays `verified` and the run fails loudly, exactly as today.
- "Not applicable to this forge" and "failed this run" are different states and must read
  differently. Collapsing them is the defect `#349` is about.

## Task
1. **`init` scaffolds the matching CI half.** `init` resolves the forge for the root it is
   initialising (repo configuration first, then the origin remote's host, mirroring
   `forge-neutral/01`'s order) and writes the corresponding CI half: the existing GitHub
   workflow, or a `.gitlab-ci.yml` carrying the same two halves — lint on a change, regenerate
   and commit the board on the default branch, from a single writer identity. Specify that
   pipeline in `gitlab-ci-half.md` before writing the template. If the forge cannot be
   resolved, `init` writes NO CI half and says so in its output rather than defaulting to
   GitHub.
2. **The next-steps text names what was written.** The closing text must name whichever file
   `init` actually created. A scaffold that tells a GitLab adopter to commit a GitHub workflow
   is advice that cannot be followed (`#349`).
3. **Auto-flip corroboration per forge.** Route the review read through statusgen's own
   forge-aware read path and apply `forge-neutral/02`'s corroboration rule: on GitHub a review
   whose `commit_id` is the head; on GitLab an approval by the accepted reviewer identity plus
   a note by that identity pinning the head SHA. The reviewer login is derived from the
   forge-qualified roster entry, not by appending `[bot]`.
4. **Honest claim decay.** Distinguish three states: decay ran; decay failed this run
   (transient — the current NOTICE wording, which is correct for that case); decay is not
   applicable on this forge because no forge-aware read exists for it. Either route the pass
   through the forge-aware read so it works on both, or emit a distinct message for the
   not-applicable case. Whichever is chosen, the wording must not tell a GitLab adopter to
   authenticate a CLI that will never help.
5. **Runbook self-description.** `docs/adopting-assay.md:12` says the runbook is
   *"GitHub-shaped throughout (Apps, rulesets, `gh`)"*. Amend it for whatever this brief makes
   forge-neutral, and leave the rest of the sentence standing — an overstated correction is
   worse than the original.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd statusgen && go build ./... && go test ./... -count=1` | exit 0 |
| 2 | `cd statusgen && go test ./... -run TestInitScaffoldsGitLabCIHalf -count=1 -v` | exit 0; initialising a root whose configured forge is GitLab writes `.gitlab-ci.yml` and does NOT write `.github/workflows/assay-statusgen.yml` |
| 3 | `cd statusgen && go test ./... -run TestInitScaffoldsGitHubCIHalfUnchanged -count=1 -v` | exit 0; the GitHub path writes exactly the ten paths it writes today — a regression here would break every existing adopter |
| 4 | `cd statusgen && go test ./... -run TestInitUnresolvedForgeWritesNoCIHalf -count=1 -v` | **negative path**: with no configured forge and an unrecognisable remote, NO CI file is written and the output says why; the test fails if the GitHub workflow appears by default |
| 5 | `cd statusgen && go test ./... -run TestInitNextStepsNamesWrittenFile -count=1 -v` | exit 0; the closing text names the file actually created, asserted for both forges |
| 6 | `cd statusgen && go test ./... -run TestAutoFlipGitLabCorroboration -count=1 -v` | exit 0; a GitLab approval by the accepted reviewer WITH a head-pinned note flips the row, and the same approval WITHOUT the head pin does not |
| 7 | `cd statusgen && go test ./... -run TestAutoFlipUncorroboratedStaysVerified -count=1 -v` | **negative path**: on BOTH forges, a review that cannot be corroborated at head leaves the row at `verified` and fails the run loudly. Without this row the brief could ship an auto-flip that advances everything and still pass row 6 |
| 8 | `cd statusgen && go test ./... -run TestAutoFlipReviewerLoginFromRoster -count=1 -v` | exit 0; the reviewer identity comes from the forge-qualified roster entry — the test fails if a literal `[bot]` suffix is still appended |
| 9 | `cd statusgen && go test ./... -run TestClaimDecayThreeStates -count=1 -v` | **negative path**: ran / failed-this-run / not-applicable-on-this-forge produce three distinct messages; the test fails if any two are identical or if not-applicable reuses the "authenticate and regenerate" wording |
| 10 | `cd statusgen && go test ./... -run TestInitGitLabPipelineIsTwoHalves -count=1 -v` | exit 0; the generated `.gitlab-ci.yml` carries both a lint job on a change and a board-regeneration job on the default branch, committing from one writer identity |
| 11 | `statusgen --root . --lint` | `LINT: PASS`, exit 0 |
| 12 | `grep -c 'gitlab' docs/adopting-assay.md` | ≥ 2 — the runbook's self-description reflects what actually became forge-neutral |
| 13 | `statusgen --root . --consumers --brief forge-neutral/08` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| The auto-flip is "made to work on GitLab" by accepting any approval, so rows advance without an at-head verdict | rows 6 (the without-head-pin half) + 7 (the discriminating control across both forges) |
| The reviewer login is still composed by appending `[bot]`, so no GitLab account ever matches and the flip silently never fires | row 8 |
| A GitLab root still gets the GitHub workflow, or gets both | row 2 |
| The GitHub scaffold regresses while the GitLab one is added | row 3 |
| An unresolvable forge quietly defaults to GitHub — the exact shape `forge-neutral/01` forbids | row 4 |
| The `.gitlab-ci.yml` is written but is not actually a single-writer board pipeline, so the adopter still has no board writer | row 10 |
| The next-steps text keeps naming the GitHub workflow, so the adopter follows advice that cannot work | row 5 |
| Not-applicable and failed-this-run stay collapsed, and the lint keeps reading green with a permanently dead pass | row 9 |
| `docs/adopting-assay.md` is over-corrected and now claims forge-neutrality the tools do not have | row 12 is a floor, not a ceiling; the accuracy of the amended sentence is **review-only**, read against what rows 2–10 actually prove |
| The generated pipeline is syntactically valid but never runs on the adopter's instance | **no row** — a live pipeline run belongs to `forge-neutral/10`'s round trip, which is where a real deployment exists |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **model** (from frontmatter; all four risk answers are `no` — see the note in
`## Context`). Reviewer records verdict + date in the stream README table, and confirms that
the amended sentence in `docs/adopting-assay.md` claims no more than rows 2–10 prove.
