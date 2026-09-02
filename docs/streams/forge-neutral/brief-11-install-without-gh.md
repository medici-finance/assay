---
brief: forge-neutral/11
title: Install without `gh` — binary acquisition, forge-neutral prerequisites, per-forge primitives
why: >-
  The front door is GitHub-only and says so in the first paragraph an adopter reads. The
  install skill has zero GitLab mentions, states "Prerequisite: two GitHub accounts", and its
  very first primitive acquires the pinned binaries with `gh release download` — so an adopter
  on a GitLab-only box with no `gh` on PATH stops before anything else in this stream can
  matter. Release assets are fetchable over plain HTTPS and the pin file already carries the
  sha256 digests, so the CLI was never load-bearing; it was just the tool that happened to be
  there.
wave: 5
depends: ["forge-neutral/01", "forge-neutral/02", "forge-neutral/08"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: brief-v1
authored: 2026-09-02 by forge-neutral authoring session
sources:
  - "driver's direction 2026-09-02 — the install path must be painless on GitLab and must not declare `gh` a requirement"
  - "docs/streams/forge-neutral/brief-08-statusgen-forge-aware.md — `init`'s forge-aware CI scaffold, which this brief's install flow invokes rather than reimplements"
  - "docs/streams/forge-neutral/brief-02-forge-qualified-identity.md — the forge-qualified roster grammar the two-principals prerequisite is stated from"
  - "#349 — the GitHub-only scaffold an adopter currently receives"
  - "freshness-checked 2026-09-02 @ deae247 — plugins/assay/skills/install/SKILL.md has 0 GitLab mentions, says \"Prerequisite: two GitHub accounts\" at :60 and acquires binaries with `gh release download` at :90 and :109; plugins/assay/skills/adopt/SKILL.md:28 lists the eight CORE primitives and :31 binds install-statusgen to `gh release download`; docs/adopting-assay.md:12 declares itself GitHub-shaped throughout; statusgen/init.go has 0 GitLab references"
exec-tier: strong
exec-tier-why: "the deliverable is the supply-chain step that puts a binary on an adopter's machine; a subtle error (a digest compared against a file fetched from the same place that named it, a fallback that installs when verification could not run) survives every functional install (question c)."
gate-why: >-
  Binary acquisition is a supply-chain control: this brief replaces the CLI that currently
  fetches the pinned releases with a plain HTTPS fetch, and the sha256 comparison against the
  pin file becomes the ONLY thing standing between an adopter and an unverified binary. The
  human is confirming that the pin file remains the single source of the expected digest, that
  a digest mismatch or an unavailable digest refuses the install rather than continuing, and
  that no path installs a binary whose digest was not positively verified.
domain: complicated
consumers:
  - "plugins/assay/skills/install/SKILL.md: fixed-here"
  - "plugins/assay/skills/adopt/SKILL.md: fixed-here (the install-statusgen primitive and the per-forge expression of create-labels, the reviewer grant and the main-guard)"
  - "docs/adopting-assay.md: fixed-here (its GitHub-shaped self-description narrows to what is genuinely GitHub-shaped after this brief)"
  - "docs/adopting-assay-gitlab.md: fixed-here (the GitLab runbook stops being a separate dead-end and becomes the per-forge half of one flow)"
  - "statusgen/init.go: out-of-scope (forge-neutral/08 makes `init` scaffold the matching CI half; this brief invokes it and must not reimplement the scaffold)"
  - "tools/cellctl/cellctl: out-of-scope (a cell is not an install; its per-forge prerequisites are forge-neutral/09's)"
---

# Brief 11 — Install without `gh`

## Context
files:
- `plugins/assay/skills/install/SKILL.md` — the turnkey install flow.
- `plugins/assay/skills/adopt/SKILL.md` — the CORE primitives and the install-statusgen note.
- `docs/adopting-assay.md` — the runbook and its self-description.
- `docs/adopting-assay-gitlab.md` — the GitLab profile.

single-point-of-failure: after this brief the sha256 comparison against `.assay-versions` is
the one control between an adopter and an unverified binary — today `gh release download`
provides no verification either, so the control is the same one, but it becomes visible rather
than incidental. Two independent layers are required: the digest comparison at acquisition,
and the post-install proof that already exists in the flow (`statusgen --version` printing the
pinned tag, and `--lint` exiting 0) — a binary that is wrong in a way the digest missed still
fails to identify itself as the pinned tag, on a different signal in a different step.

facts:
- `plugins/assay/skills/install/SKILL.md:60` reads *"Prerequisite: two GitHub accounts"* and
  the file contains zero occurrences of "gitlab".
- Binary acquisition is `gh release download <tag> --repo <umbrella-releases> --pattern …`
  (`plugins/assay/skills/install/SKILL.md:90`) and
  `gh release download "$tag" --pattern "desk-tools-<platform>.tar.gz"`, `shasum -a 256` →
  compare (`:109`). `plugins/assay/skills/adopt/SKILL.md:31` binds the `install-statusgen`
  primitive to `.assay-versions` + `gh release download`.
- The eight CORE primitives are `install-statusgen`, `scaffold-registers`,
  `scaffold-streams`, `add-statusgen-ci`, `install-desk-plugin`, `install-main-guard`,
  `first-board`, `setup-reviewer-app` (`plugins/assay/skills/adopt/SKILL.md:28`).
- `docs/adopting-assay.md:12` states the runbook is *"GitHub-shaped throughout (Apps,
  rulesets, `gh`)"* and points at the GitLab profile as a separate document.
- `statusgen/init.go` contains zero GitLab references today; `forge-neutral/08` is what makes
  `init` scaffold the matching CI half, and this brief depends on it for exactly that reason.
- The release assets already carry per-platform sha256 digests in the release's
  `checksums.txt`, and `.assay-versions` pins the tag together with those digests — the pilot
  seeded a tracking root this way (`docs/streams/forge-gitlab/pilot-report.md` §1 step A2).
- The two-principals rule is an invariant, not a GitHub feature: a human identity and an
  automation identity that are not the same principal. Its mechanism differs per forge —
  GitHub Apps, GitLab service accounts — and the pilot provisioned seven seatless bot accounts
  on GitLab Free (`pilot-report.md` §0).
- The install skill's existing posture — REFUSES rather than clobbers an already-adopted repo,
  DRAFT PRs only, escalates every never-autonomous step to a human — is unchanged by this
  brief and must survive it.

## Ground rules
- NEVER git push / trigger workflows / run mutating kubectl. Leave commits per the task
  instructions only.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- **Never install a binary whose digest was not positively verified.** A digest that could not
  be read is a refusal, not a warning. "Verification unavailable, continuing" is the failure
  this brief must not introduce while removing the CLI.
- Do not put `gh` or `glab` invocations into the skill text as the sanctioned path for a
  primitive. Where a primitive needs a forge operation, it goes through a desk verb.
- Do not weaken the refuses-not-clobbers behavior or the draft-PR-only rule.

## Task
1. **Binary acquisition with no CLI.** Replace `gh release download` in both skills with a
   plain HTTPS fetch of the release asset by its pinned tag and platform, followed by a
   sha256 comparison against the digest in `.assay-versions` — the pin file staying the single
   source of the expected value. On mismatch, or when the expected digest cannot be read, the
   step REFUSES and installs nothing. The same step must work identically on a box with no
   `gh` and no `glab`.
2. **Prerequisites stated forge-neutrally.** Rewrite the prerequisite at
   `plugins/assay/skills/install/SKILL.md:60` as the invariant it is: two distinct
   principals — a human identity and an automation identity — with a per-forge mechanism table
   (GitHub Apps; GitLab service accounts) and the forge-qualified roster entry each produces,
   citing `forge-neutral/02`'s grammar rather than restating it.
3. **CORE primitives per forge.** For each of the eight, state whether it is forge-neutral,
   and where it is not, express it per forge through a desk verb — `create-labels`, the
   reviewer grant and `install-main-guard` being the three the driver named. `add-statusgen-ci`
   delegates to `statusgen init`, which `forge-neutral/08` has already made forge-aware; this
   brief must not reimplement the scaffold.
4. **Optional-CLI prose.** State plainly, once, which CLI is optional on which forge and what
   it is optional FOR (convenience reads a human might do), and that no install step requires
   it. Remove every instruction that assumes one is present.
5. **One flow, two profiles.** `docs/adopting-assay.md:12`'s self-description narrows to what
   is genuinely still GitHub-shaped after this brief, and `docs/adopting-assay-gitlab.md`
   stops reading as a separate dead-end: the two become the per-forge halves of one install
   flow, cross-linked in both directions.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -c 'gh release download' plugins/assay/skills/install/SKILL.md plugins/assay/skills/adopt/SKILL.md \|\| true` | prints `0` for both files — the CLI acquisition step is gone, not merely supplemented |
| 2 | `grep -ci 'gitlab' plugins/assay/skills/install/SKILL.md` | ≥ 3 — the prerequisite table, the per-forge primitives and the optional-CLI note all mention it |
| 3 | `grep -c 'two GitHub accounts' plugins/assay/skills/install/SKILL.md \|\| true` | prints `0` — the prerequisite is stated as two principals with a per-forge mechanism, not as two accounts on one forge |
| 4 | `grep -c '^[\|] ' plugins/assay/skills/install/SKILL.md` | ≥ 2 — the per-forge prerequisite table and the primitives table are tables, not prose |
| 5 | **GitLab-only dry run**: on a box (or container) with **no `gh` and no `glab` on `PATH`**, against a GitLab-remote repo, run the install skill's flow in its dry/rehearsal mode end to end; capture the transcript | exit 0; the transcript shows the binary acquired and sha256-verified, `statusgen init` scaffolding `.gitlab-ci.yml`, and no step attempting a CLI. `command -v gh; command -v glab` in the same shell must both print nothing — recorded in the transcript |
| 6 | on the same box, with the pin file's digest deliberately altered, re-run the acquisition step | **negative path**: the step REFUSES, installs nothing, and names the mismatch. The row fails if the binary lands or if the mismatch is reported as a warning |
| 7 | on the same box, with the digest entry for the platform REMOVED from the pin file, re-run the acquisition step | **negative path**: the step REFUSES because the expected digest could not be read. This is the distinct failure from row 6, and it is the one a "verification unavailable, continuing" fallback would pass |
| 8 | run the install skill against an already-adopted repo | **negative path**: it REFUSES rather than clobbering, exactly as today — the pre-existing property survives the rewrite |
| 9 | `grep -rn -e 'gh ' -e 'glab ' plugins/assay/skills/install/SKILL.md plugins/assay/skills/adopt/SKILL.md \| grep -v -e optional -e convenience` | every remaining hit is inside the optional-CLI note; a CLI invocation presented as the sanctioned path for a primitive is a FAIL — read as a list |
| 10 | `statusgen --version` after the dry run's acquisition step | prints the tag pinned in `.assay-versions` — the independent second layer: a binary that is wrong in a way the digest missed cannot name itself correctly |
| 11 | `grep -c 'adopting-assay-gitlab' docs/adopting-assay.md` and `grep -c 'adopting-assay.md' docs/adopting-assay-gitlab.md` | ≥ 1 each — the two profiles cross-link in both directions |
| 12 | `statusgen --root . --consumers --brief forge-neutral/11` | exit 0 — every `consumers:` routing claim is corroborated against this branch's own diff |

## Pre-mortem → detection map

| Failure mode of the work | Caught by |
|---|---|
| The CLI is removed from the prose but a step still needs it, and nobody notices because the author's box has it | row 5, whose `command -v` output must show both CLIs absent in the very shell the flow ran in |
| A digest mismatch is downgraded to a warning so the install "just works" | row 6 |
| A missing digest entry is treated as "nothing to compare, proceed" — the subtler half of row 6 | row 7, deliberately a separate row for exactly this reason |
| The install verifies against a digest fetched from the same place as the binary, so the comparison proves nothing | **review-only** — the pin file must be the source, which the Review gate reads in the diff; row 6 fails only when the pin file is the source, which is why it is stated that way |
| The refuses-not-clobbers or draft-PR-only posture is lost in the rewrite | row 8 |
| The scaffold is reimplemented in the skill instead of delegating to `statusgen init`, so the two drift | row 5's transcript must show `init` doing the scaffolding; `consumers:` routes `init` out-of-scope and row 12 corroborates it |
| `gh` reappears as the "recommended" way to do a primitive | row 9 |
| The prerequisite is genericised into vagueness — "two identities" with no per-forge mechanism an adopter can act on | row 4 requires a mechanism TABLE; whether it is actionable is **review-only** |
| The GitHub install regresses while the GitLab path is added | rows 1, 8 and 10 all run on the existing flow too; a GitHub install that stops working fails 10 |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter — `sensitive-data: yes`). Reviewer records verdict + date in
the stream README table.

Core-system reviewer questions, answered in the verdict:
1. What single control stands between an adopter and an unverified binary? (The sha256
   comparison against the pin file.) Is it acceptable alone? (No — `statusgen --version`
   against the pinned tag and `--lint` exiting 0 are the second layer, tripping on a different
   signal at a different step.)
2. Does any Verify row prove a LOWER layer catches the fault with the UPPER bypassed? (Row 10
   runs after acquisition and fails on a binary that is not the pinned tag, whatever the digest
   step concluded; rows 6 and 7 prove the digest step itself refuses in both of its distinct
   failure modes.)
