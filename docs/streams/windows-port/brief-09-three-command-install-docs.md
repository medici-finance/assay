---
brief: assay:assay:windows-port:09
title: Three-command Windows install — widen the install skill's scope, collapse the walkthrough, correct the CI skew
why: >-
  Briefs 06, 07 and 08 make a Windows adopter's install three commands. Until the documentation
  says so, the adopter still follows the fifteen-step path, because the fifteen-step path is what
  is written down — in three places that each describe a different arm of it. Two of those places
  are also WRONG at the freshness head, and wrong in the direction that understates what works:
  `plugins/assay/skills/install/SKILL.md` §Scope still frames Windows as an ACQUISITION arm only
  ("The statusgen binary acquisition in step 3 is the only OS-specific arm", :249-251), and
  `docs/adopting-assay.md:978-987` still says the Windows CI leg "lives at
  `ci/staged-workflows/windows-ci-leg.yml`, *not yet* under `.github/workflows/`" — which it does,
  and has since the promotion commit, with `windows-port/04` recorded `done` on this stream's own
  board. A doc that understates a shipped capability is the same defect class as one that
  overstates an unshipped one: the reader cannot act on it.
wave: 4
depends: ["windows-port/06", "windows-port/07", "windows-port/08"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-11 by windows-port authoring session (driver ask, 2026-09-11)
sources:
  - "driver's ask (2026-09-11): widen the install skill's Windows scope from acquisition-only to the whole install; collapse the adopter doc's Windows walkthrough to the three commands with the old path kept as a manual appendix; correct the docs/board skew on the Windows CI leg"
  - "plugins/assay/skills/install/SKILL.md:249-251 — '**Unix-first (mac/linux), with a real native-Windows arm.** The statusgen binary acquisition in step 3 is the only OS-specific arm.' — the acquisition-only framing this brief widens"
  - "plugins/assay/skills/install/SKILL.md:13 and :27 — the same framing repeated in the frontmatter description and the body ('a native-Windows acquisition arm (PowerShell bootstrap + Go-native deskinstall)')"
  - "plugins/assay/skills/install/SKILL.md:254-266 — the two honesty caveats that must SURVIVE the widening: the bash+jq resident-rules workaround, and the BLOCKED native windows/arm64 smoke"
  - "docs/adopting-assay.md:978-987 — '### CI-proven status — staged, pending promotion (not yet a live check)': asserts the leg 'lives at ci/staged-workflows/windows-ci-leg.yml, *not yet* under .github/workflows/'"
  - ".github/workflows/windows-ci-leg.yml — present at the freshness head (jobs windows-smoke:77, windows-bootstrap-smoke:141, arm64-native-smoke:171 held `if: false`); promoted by commit 1ab3e1e5. The doc's claim is stale, in the understating direction"
  - "docs/streams/windows-port/README.md — brief 04 (Windows CI leg) is recorded `done`, verified 2026-09-06, reviewed 2026-09-08: the board and the doc disagree, and the board is right"
  - "docs/adopting-assay.md:927-962 — the current Install-path steps 1-3, including the hand-copied sha256 that windows-port/06 removes"
  - "docs/adopting-assay.md:1085-1098 — the five-step Cursor copy that windows-port/07 replaces with one command"
  - "docs/adopting-assay.md:900-906 — the Windows prerequisites naming Git-Bash/WSL for GitLab fleet provisioning, which windows-port/08 removes"
  - "docs/adopting-assay-gitlab.md:176-201 — the manual token-file link/copy and the 'run it from Git-Bash or WSL, not from PowerShell' line windows-port/08 retires"
  - "docs/adopting-assay.md:702-755 — the create-labels PRIMITIVE, GitHub-only today; windows-port/08 adds the forge-neutral path this doc must then point at"
  - "plugins/assay/references/cursor.md:11-25 — the five manual steps, a second copy of the same instructions; one of the two must become a pointer"
  - "windows-port/05 (done, 2026-09-07) — the brief that authored the Windows walkthrough this one collapses; its own Verify rows are the shape to preserve, not to discard"
  - "freshness-checked 2026-09-11 @ 35316469 (origin/main): install/SKILL.md §Scope still says acquisition-only; adopting-assay.md:978 still says 'staged, pending promotion'; .github/workflows/windows-ci-leg.yml exists"
consumers:
  - "plugins/assay/skills/install/SKILL.md: follow-up windows-port/09 (this brief; flips to fixed-here when the implementation edits the path)"
  - "docs/adopting-assay.md: follow-up windows-port/09 (this brief)"
  - "docs/adopting-assay-gitlab.md: follow-up windows-port/09 (this brief)"
  - "plugins/assay/references/cursor.md: follow-up windows-port/09 (this brief; the five manual steps become a pointer at the one command)"
  - "docs/streams/windows-port/README.md: follow-up windows-port/09 (this brief; the stream's end-state paragraph is restated once the three commands are real)"
  - "plugins/assay/cursor/packaging.md: out-of-scope (the coverage roster is machine-read by harnessgen and by windows-port/07's mode; it is not adopter-facing prose and needs no collapse)"
  - ".github/workflows/windows-ci-leg.yml: out-of-scope (this brief corrects the DOC's description of the workflow; the workflow itself is windows-port/04's and 06's)"
exec-tier: strong
exec-tier-why: >-
  Question (b): correctness is cross-artifact. The same install is described in four documents plus
  a skill body, and this brief must leave exactly one authoritative account with the others
  pointing at it — a per-file-correct edit can still leave two documents that disagree, which is
  the precise defect being fixed. The `sources:` list above records two claims that are already
  wrong in the tree; the risk of authoring a third is the reason this is not `any`.
version: 1
id: 4fe14818-a862-4f36-912e-71dc1e25f5f6
---

# Brief 09 — Three-command Windows install: scope, walkthrough, and the CI skew

## Context

files:
- **edit** `plugins/assay/skills/install/SKILL.md` — §Scope (:247-266), plus the two upstream
  restatements at :13 and :27.
- **edit** `docs/adopting-assay.md` — the **Windows adopters** section (:862-1037), the Cursor
  install steps (:1085-1098), and the `create-labels` PRIMITIVE's forge note (:702-755).
- **edit** `docs/adopting-assay-gitlab.md` — the token-file link/copy step (:176-198) and the
  Git-Bash/WSL prerequisite (:200-201).
- **edit** `plugins/assay/references/cursor.md` — the five manual steps (:11-25) become a pointer.
- **edit** `docs/streams/windows-port/README.md` — the stream's end-state paragraph.
- **create** `changelog/<branch-slug>.md` — this repo enforces a per-PR fragment.
- **do NOT** edit `.github/workflows/windows-ci-leg.yml` — this brief corrects the doc's
  DESCRIPTION of it, not the workflow.

facts:
- **The skill's Windows framing is acquisition-only and must widen.**
  `plugins/assay/skills/install/SKILL.md:249-251`: "**Unix-first (mac/linux), with a real
  native-Windows arm.** The statusgen binary acquisition in step 3 is the only OS-specific arm."
  Once 06, 07 and 08 land, that sentence is false in the understating direction: the harness
  install (07) and the forge provisioning (08) are OS-specific arms too, and both are native. The
  same framing is repeated at :13 (frontmatter description) and :27 — three sites, one claim.
- **Two honesty caveats must SURVIVE the widening, not be swept up in it.**
  `plugins/assay/skills/install/SKILL.md:254-266` records that the harness's session-start resident-rules injection
  needs a documented `bash`+`jq` workaround, and that the **native `windows/arm64` smoke is
  BLOCKED** pending an arm64 runner while the asset still ships cross-compiled and checksummed.
  Neither is retired by 06-08. A widening that quietly drops them replaces an understatement with
  an overstatement, which is worse.
- **The CI-leg claim is stale, and the board already disagrees with it.**
  `docs/adopting-assay.md:978-987` says the Windows leg "lives at
  `ci/staged-workflows/windows-ci-leg.yml`, *not yet* under `.github/workflows/`" and is therefore
  "pending promotion, not a live green check". At the freshness head `.github/workflows/windows-ci-leg.yml`
  exists, carrying `windows-smoke` (:77), `windows-bootstrap-smoke` (:141) and a deliberately
  never-scheduled `arm64-native-smoke` (:171, `if: false`); `windows-port/04` is recorded `done` on
  this stream's board (verified 2026-09-06, reviewed 2026-09-08). Note the file ALSO still exists
  under `ci/staged-workflows/` — the staged copy was not removed at promotion, which is how the
  doc's claim stayed superficially checkable. Establish which is live before writing the
  replacement sentence; do not simply invert the old one.
- **The `arm64-native-smoke` job's `if: false` is a held BLOCKED row, not a broken job.** The
  workflow comment says so explicitly. The corrected doc must not describe it as a passing job,
  and must not describe the leg as fully green either.
- **The fifteen-step path is real and spread over three arms.** Counting the adopter's actual
  actions at the freshness head: clone the repo (:932); run the bootstrap with a hand-copied
  sha256 (:933-944); put `%LOCALAPPDATA%\Assay\bin` on PATH by hand (:909-910); run `deskinstall`
  (:947-956); pin the Windows assets in `.assay-versions` (:963-975); install Git-Bash for the
  hooks and for GitLab provisioning (:900-906); scaffold with `statusgen init`; copy
  `plugins/assay/skills/*` to `.cursor/skills/` (:1087-1090); copy
  `plugins/assay/references/*.md` alongside (:1091-1092); write the `AGENTS.md` bindings
  (:1093-1094); put the desk binaries on PATH again for Cursor (:1095-1096); run
  `tools/create-fleet-gitlab.sh` from Git-Bash or WSL
  (`docs/adopting-assay-gitlab.md:146-163,200-201`); copy each `<prefix>-<role>-bot.token` to
  `gitlab-<role>.token` (:197-198); lock each to an owner-only ACL (:198); export
  `GITLAB_API_BASE` (:203-215). Fifteen, plus the label primitive. The appendix must preserve all
  of them, because they remain the path for anyone not taking the three-command route.
- **The three commands, after 06-08.** (1)
  `powershell -File scripts/bootstrap-windows.ps1 -Tag vX.Y.Z` — sha and PATH resolved by the
  script (`windows-port/06`); (2)
  `deskinstall --harness cursor --forge gitlab --repo C:\src\myrepo` (`windows-port/07`), with the
  pinned-binary install still available as `deskinstall --manifest … --dest …`; (3) invoke
  `assay:install` inside Cursor. GitLab fleet provisioning is `windows-port/08`'s verb, which the
  doc must place relative to those three rather than leaving it implicit.
- **The same instructions exist in two places for Cursor.**
  `plugins/assay/references/cursor.md:11-25` and `docs/adopting-assay.md:1087-1098` are the same
  five steps written twice. The repo's own placement rule is that a second copy is the copy that
  goes stale — this brief leaves ONE authoritative account and makes the other a pointer, rather
  than editing both to agree today.
- **This brief is the deferred consumer of three others, named by their full ids** so the
  back-reference is explicit rather than implied by `depends:`: `assay:assay:windows-port:06`
  (the manifest-driven bootstrap), `assay:assay:windows-port:07` (the Cursor harness mode) and
  `assay:assay:windows-port:08` (the GitLab fleet verb) each route their adopter-facing
  documentation paths — `docs/adopting-assay.md`, `docs/adopting-assay-gitlab.md`,
  `plugins/assay/references/cursor.md`, `plugins/assay/skills/install/SKILL.md` — to
  `follow-up windows-port/09`. Those routings are THIS brief's work item list; a path routed here
  and then not edited here is a stranded deferral, not a completed one.
- **Do not re-describe anything this brief does not read.** Every claim the edited docs make about
  06, 07 and 08 must be checked against those briefs' shipped artifacts, not against their briefs'
  prose — an adopter doc written from a plan rather than from the tree is exactly how
  `adopting-assay.md:978` came to be wrong.

single-point-of-failure: for a docs brief the control is the DEREFERENCING Verify row — a
presence/formatting check cannot fail on a confident falsehood sitting in the right section, which
is precisely the defect at `adopting-assay.md:978` today (a well-formed, correctly-placed,
entirely stale paragraph). The second, independent layer: the repo's link-check in CI fails on a
cited path that does not exist, on a different signal (path resolution) in a different component
than the row that runs the documented command. Rows 5-9 below are the first layer; row 13 is the
second. NONE is not the answer here.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- **Check every claim against the TREE before writing it.** This brief exists because two
  documented claims drifted from the tree; authoring a third is the failure mode. Where a claim
  cannot be checked offline (a live CI run, a Windows runtime), write it as the could-not-check it
  is, with the reason — never round it up.
- **Preserve the two honesty caveats** (the `bash`+`jq` resident-rules workaround, and the BLOCKED
  native `windows/arm64` smoke). Removing an honest caveat to make a section read cleaner is a
  regression, not a collapse.
- **Keep the fifteen-step path complete** in the appendix. It is the manual route and it stays
  usable; a collapse that loses steps is a deletion wearing a rewrite's clothes.
- Do not open a second authoritative account of any instruction. Where two copies exist, one
  becomes a pointer.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Widen `plugins/assay/skills/install/SKILL.md` §Scope** from acquisition-only to the whole install: acquisition
   (06), harness placement (07), and forge provisioning (08), each named with the command that
   performs it. Update the two upstream restatements at :13 and :27 in the same change so all
   three sites say one thing.
2. **Carry both honesty caveats forward** into the widened §Scope, unchanged in substance.
3. **Collapse the adopter walkthrough** in `docs/adopting-assay.md` § Windows adopters to the
   three commands, in order, each with what it does and what proves it worked.
4. **Keep the old path as a clearly-labelled manual appendix** — all fifteen steps, marked as the
   route for an adopter who cannot or does not want to use the three commands, and as the
   reference for what the three commands actually do.
5. **Correct the CI-proven status section** (`:978-987`). Establish from the tree which workflow
   file is live before writing: name the live jobs, state that `arm64-native-smoke` is HELD
   (`if: false`) and not passing, and keep the arm64 asset's ships-cross-compiled-and-checksummed
   fact intact. If the staged copy under `ci/staged-workflows/` is genuinely redundant, say so and
   route its removal — do not remove it as a side effect of a docs edit.
6. **Retire the GitLab Git-Bash/WSL prerequisite** in `docs/adopting-assay.md:900-906` and
   `docs/adopting-assay-gitlab.md:200-201`, and the manual token-file link/copy at
   `adopting-assay-gitlab.md:176-198`, pointing both at `windows-port/08`'s verb. Keep the Git-Bash
   prerequisite that remains genuinely required — the Claude Code SessionStart hooks — and say
   plainly that it is the only one left, and that Cursor does not need it.
7. **Point the `create-labels` PRIMITIVE at the forge-neutral path** from `windows-port/08`,
   keeping the nine `gh label create` lines as the GitHub-native equivalent.
8. **Make one of the two Cursor accounts a pointer.** Choose which is authoritative, say why in the
   PR body, and reduce the other to a link plus a one-line summary.
9. **Restate the stream README's end-state paragraph** so it describes the delivered three-command
   install rather than the acquisition-only end state it was authored against.
10. **Add the changelog fragment** (`changelog/<branch-slug>.md`).

## Verify (executable — no prose-only DoD items)

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | The acquisition-only framing is gone from all three sites: `grep -n 'only OS-specific arm' plugins/assay/skills/install/SKILL.md; echo "rc=$?"` | `rc=1` — no match | `check` |
| 2 | The widened scope names all three arms: `grep -c -e 'deskinstall --harness' -e 'bootstrap-windows.ps1' plugins/assay/skills/install/SKILL.md` | `>= 2`, and a manual read confirms the forge-provisioning arm is named too | `check` |
| 3 | **Both honesty caveats survive**: `grep -c 'windows/arm64' plugins/assay/skills/install/SKILL.md` and `grep -ci 'jq' plugins/assay/skills/install/SKILL.md` | both `>= 1` — the BLOCKED arm64 smoke and the bash+jq workaround are still stated | `check +neighbour` |
| 4 | The stale staged-CI claim is gone: `grep -n 'not yet. under .github/workflows' docs/adopting-assay.md; echo "rc=$?"` | `rc=1` — no match | `check` |
| 5 | **DEREFERENCE the CI claim against the tree** (the row that catches a wrong-but-well-formed paragraph): `test -f .github/workflows/windows-ci-leg.yml && echo LIVE \|\| echo STAGED` then confirm the rewritten section says the same thing | the file state and the prose agree | `check +dereference` |
| 6 | **DEREFERENCE the held arm64 row**: `grep -n 'if: false' .github/workflows/windows-ci-leg.yml` then confirm the doc describes `arm64-native-smoke` as HELD/BLOCKED and not as passing | the job's real state and the prose agree | `check +dereference` |
| 7 | **DEREFERENCE the three commands against the shipped artifacts** — for each of the three commands the walkthrough now prints, run its `--help` (or, for the PowerShell one, read its `param(` block) and confirm every flag the doc shows EXISTS with that spelling: `cd tools/desk && go run ./cmd/deskinstall --help 2>&1 \| grep -c -e '--harness' -e '--forge' -e '--repo'` and `sed -n '/^param(/,/^)/p' scripts/bootstrap-windows.ps1` | every flag in the doc appears in the tool's own surface; no invented flag | `check +dereference` |
| 8 | **DEREFERENCE the GitLab claims**: the doc's GitLab provisioning command exists as a built verb — `cd tools/desk && go build ./cmd/<verb>/ && go run ./cmd/<verb> --help 2>&1 \| head -5` | exit 0; the command named in the doc is the command that exists | `check +dereference` |
| 9 | **The Git-Bash prerequisite that remains is exactly the hooks one**: `grep -n -i 'git-bash' docs/adopting-assay.md docs/adopting-assay-gitlab.md` | every surviving mention is about the Claude Code SessionStart hooks; none is about GitLab fleet provisioning | `check +dereference` |
| 10 | **The manual appendix is complete** — `awk '/Manual appendix/,0' docs/adopting-assay.md \| grep -cE '^[0-9]+\.'` | `>= 15` — no step lost in the collapse | `check` |
| 11 | **The duplicate Cursor account is now a pointer**: `grep -cE '^[0-9]+\. ' plugins/assay/references/cursor.md` compared against the same count in `docs/adopting-assay.md`'s Cursor section | exactly ONE of the two still enumerates the steps; the other links to it | `check +flow` |
| 12 | **The board and the doc agree on brief 04** — `grep -E '^\| 04 ' docs/streams/windows-port/README.md` and the rewritten CI-proven section | both say the Windows CI leg is live/`done`; the skew the brief exists to fix is closed | `check +flow` |
| 13 | Every cited path resolves (the independent second layer): `grep -ohE '\(\.?\.?/?[a-zA-Z0-9_./-]+\.(md\|yml\|ps1\|go)\)' docs/adopting-assay.md \| tr -d '()' \| sort -u \| while read p; do test -e "$p" \|\| test -e "docs/$p" \|\| echo "MISSING: $p"; done` | no `MISSING:` lines | `check:ci` |
| 14 | Consumers routing corroborated by the diff (run on the implementer's branch): `statusgen --root . --consumers windows-port/09; echo $?` | `0` | `check` |
| 15 | Board lint stays clean: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 7 and 8 depend on
     06/07/08 having landed; a verifier running before them records could-not-check
     with that reason rather than a pass. -->

## Review
Gate: **model** (from frontmatter — all four risk answers no). The reviewer's questions: (1) could
any row in this table go red on a confident falsehood in the right section? Rows 5-9 are the
dereferencing rows; rows 1, 4 and 10 are presence gates and cannot. A table without rows 5-9 would
reproduce exactly the defect this brief fixes. (2) Did the collapse drop an honest caveat or a
manual step? Rows 3 and 10 are the regression guards; a green walkthrough with a shrunken appendix
is a deletion, not a collapse.
