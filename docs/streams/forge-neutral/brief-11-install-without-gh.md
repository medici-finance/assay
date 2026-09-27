---
brief: assay:assay:forge-neutral:11
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
design: DR-forge-neutral-11
decision-issue: 1554
issues: []
schema: brief-v2
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
  - "plugins/assay/scripts/assay-install.sh: fixed-here (the CLI-free acquisition step, pin-file digest comparison and refusals, and the rehearsal the skill's dry run names — task 1, verify rows 5-8 and 10)"
  - "plugins/assay/scripts/assay-install.test.sh: fixed-here (the hermetic suite that pins every refusal, plus the opt-in real-statusgen rehearsal)"
  - "plugins/assay/paired-versions.yaml: fixed-here (a header comment only — the resolution it describes is now a plain-HTTPS fetch, not `gh release download`)"
  - "statusgen/init.go: out-of-scope (forge-neutral/08 makes `init` scaffold the matching CI half; this brief invokes it and must not reimplement the scaffold)"
  - "tools/cellctl/cellctl: out-of-scope (a cell is not an install; its per-forge prerequisites are forge-neutral/09's)"
version: 1
id: 12da22af-27f5-4953-9fc4-ccbb008def48
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
| 13 | `bash plugins/assay/scripts/assay-install.test.sh` — cases H1–H4 (added by the #1554 ruling: refuse any non-HTTPS URL or redirect during the download) | **negative path**: against a local HTTPS fixture server whose redirect lands on a plain-`http://` server holding the SAME good asset, the acquisition REFUSES (exit 5) and writes nothing to the destination (H1); a `file://` URL is refused (H2); a cross-host HTTPS redirect is followed and the asset still digest-verified (H3); an HTTPS-served asset whose digest does not match the pin is still refused (H4) — HTTPS never substitutes for the pinned sha256. The row fails if H1's binary lands |

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
| A redirect downgrades the download to plain HTTP (or a non-HTTPS URL is accepted), or the HTTPS check is treated as a replacement for the digest (the #1554 ruling's addition) | row 13 — H1/H2 for the downgrade, H4 for "HTTPS is not the integrity check" |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -c 'gh release download' plugins/assay/skills/install/SKILL.md plugins/assay/skills/adopt/SKILL.md \|\| true` | pass exit=0 | sha256:1e06514522cb | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -ci 'gitlab' plugins/assay/skills/install/SKILL.md` | pass exit=0 | sha256:a5331f18877e | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -c 'two GitHub accounts' plugins/assay/skills/install/SKILL.md \|\| true` | pass exit=0 | sha256:9a271f2a916b | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -c '^[\|] ' plugins/assay/skills/install/SKILL.md` | pass exit=0 | sha256:9a92adbc0cee | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 5 | `gh` | pass exit=0 | sha256:5665cc62dabe | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 6 | `on the same box, with the pin file's digest deliberately altered, re-run the acquisition step` | fail exit=2 | sha256:80dd39947a6e | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 7 | `on the same box, with the digest entry for the platform REMOVED from the pin file, re-run the acquisition step` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:2125358ffdab | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 8 | `run the install skill against an already-adopted repo` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:8e39eaae81a7 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 9 | `grep -rn -e 'gh ' -e 'glab ' plugins/assay/skills/install/SKILL.md plugins/assay/skills/adopt/SKILL.md \| grep -v -e optional -e convenience` | fail exit=1 | sha256:e3b0c44298fc | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 10 | `statusgen --version` | pass exit=0 | sha256:ea233f8d0995 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 11 | `grep -c 'adopting-assay-gitlab' docs/adopting-assay.md` | pass exit=0 | sha256:e6c21e8d260f | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 12 | `statusgen --root . --consumers --brief forge-neutral/11` | fail exit=2 | sha256:170dddf43758 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |
| 13 | `bash plugins/assay/scripts/assay-install.test.sh` | fail exit=0 | sha256:fb2930c98ed1 | 2026-09-27 | assay-verifier-app[bot] @ b227b40768db (on-behalf-of human:ian) (forge-identity) |

**Verifier notes — 2026-09-27, non-implementer verifier (opus-5.5), merged main b227b40768db;
implementing change 3439807d08dd (PR #1560).** Every row was also run by hand; the witness
table above is the machine record, and five of its verdicts are artefacts of how it reads a
prose or list-shaped row (rows 5-9, 12, 13 below say which). Key real output per row:

- **Row 1** — `install/SKILL.md:0`, `adopt/SKILL.md:0`. PASS.
- **Row 2** — `46` (need ≥ 3). PASS.
- **Row 3** — `0`. PASS.
- **Row 4** — `14` (need ≥ 2). PASS.
- **Row 5** — the witness row is NOT this row: verifyrun lifted the first code span of the
  prose (a bare `gh`) and ran it, so its `pass` is evidence of nothing. Run by hand instead: a
  shell whose PATH held only a symlink farm (bash, coreutils, git, curl, shasum, openssl — no
  forge CLI); in that same shell `command -v gh; command -v glab` printed nothing (rc=1); then
  `assay-install.sh rehearse` against a fresh repo whose origin is a GitLab host, with a
  statusgen built from merged main (stamped `v0.0.0-verifier`) served as a release from a local
  HTTPS fixture (throwaway certificate via CURL_CA_BUNDLE). Exit 0. Transcript: `fresh` →
  `pinned: statusgen-darwin-arm64 v0.0.0-verifier 275771dd…` → `verified: sha256 275771dd…
  matches the pin — installing` → `statusgen init` → `scaffolded: .gitlab-ci.yml` (no GitHub
  workflow directory created) → `statusgen --version -> v0.0.0-verifier (pinned:
  v0.0.0-verifier)` → `--lint -> exit 0` → `rehearsal PROVEN`. The real target's git status
  stayed clean. PASS, scoped: the release was a local fixture, not the published release (the
  offline envelope forbids the live fetch).
- **Row 6** — same box, pin digest altered in its first six hex characters: exit 5, `REFUSED —
  sha256 MISMATCH for statusgen-darwin-arm64 @ v0.0.0-verifier: pinned 000000dd…, fetched
  275771dd… — nothing installed`; the destination directory was never created. PASS. (Witness
  `fail exit=2` = the prose sentence executed as a shell command.)
- **Row 7** — same box, the platform's pin line deleted: exit 5, `REFUSED — no pin line for
  statusgen-darwin-arm64 … the expected digest could not be read; refusing rather than guessing
  a platform or skipping verification`; destination never created. PASS, and distinct from
  row 6's message. Suite N3-N6/N9-N11 cover the placeholder, truncated, upper-cased, missing
  field, competing-lines, absent-file and other-platform shapes of the same failure.
- **Row 8** — against the rehearsal's own output (streams tree + real pin + .gitlab-ci.yml):
  `classify` exit 5 `REFUSED — this repo is already adopted; nothing to install — refusing
  rather than clobbering the live adoption`; `rehearse` on it exit 5 at step 1 with nothing
  acquired; a sha256 manifest of the adopted repo was byte-identical before and after. PASS.
- **Row 9** — zero hits before the filter and zero after; the pipeline's exit 1 is `grep -v`
  on empty input, i.e. the expected empty list. PASS (witness `fail exit=1` is the list-shaped
  row read as an exit code). A wider scan for backticked gh/glab finds them only in the
  optional-CLI note (install skill lines 65 and 93, adopt skill line 61) plus install skill line
  416 — a Windows limitation note on the GitLab PAT-renewal tool, added later by windows-port/09,
  not an install primitive and outside the row's pattern.
- **Row 10** — the acquired binary's `--version` printed `v0.0.0-verifier`, the tag pinned in
  the rehearsal's .assay-versions. PASS. (The witness ran whatever statusgen was first on PATH,
  v1.0.27 — not the binary this row names.) Suite R3 shows the layer working on its own: a
  digest-verified binary that names a different tag exits 5 `install NOT proven`.
- **Row 11** — `16` and `8`. PASS (the witness ran only the first of the two commands).
- **Row 12** — as written, on merged main: exit 2, the tool's own message that this merged
  brief is not in the diff against main, naming the remedy. Re-run per that remedy at the
  implementing commit with `--base` its parent: exit 0, `3 corroborated, 0 disproved, 6
  unchecked` (the 6 are entries the authored brief already carried — by design). By hand: that
  commit's diff does touch the four fixed-here skill/runbook files (+362/−142) and does not
  touch statusgen/init.go or tools/cellctl/cellctl, so both out-of-scope claims hold. PASS;
  the row is branch-time shaped, not a defect.
- **Row 13** — default suite: exit 0, `40 passed, 0 failed`, including `H1 a redirect to http://
  is REFUSED, nothing written`, H2, H3 and H4. With `--real` (real statusgen built from main):
  exit 0, `42 passed, 0 failed`, including REAL and REAL2. PASS. (Witness `fail exit=0 —
  expected 5` read H1's inner exit code from the Expect cell as the suite's own.) T1/T2 also
  pass: an untrusted certificate is exit 6, and a user curl config that disables verification
  is ignored.

**Risk-bearing value enumeration** (diff scope: plugins/assay/scripts/assay-install.sh, the two
skills, paired-versions.yaml; plus the Deliverables). Literals, ranked by irreversibility — an
unverified binary executed on an adopter's machine cannot be undone by a redeploy:

1. `is_sha256` pattern `^[0-9a-f]{64}$` @ assay-install.sh:95, compared by string equality at :196
2. curl `-q … --proto '=https' --proto-redir '=https'` @ :155, curl exit `1` → refuse (exit 5) @ :159
3. initial-URL scheme allowlist `https://*` @ :149
4. expected-digest source: `pin_fields "$pins" "$asset"` @ :184 — the pin file, and nothing fetched
5. statusgen `tag: v1.0.24` and the per-platform sha256 lines @ paired-versions.yaml:35-44 (the
   values `pin` copies into an adopter's .assay-versions; not changed by this diff)
6. `DEFAULT_BASE_URL="https://github.com"` @ :67, `DEFAULT_RELEASE_HOME="medici-finance/assay"` @ :68
7. tag charset `^[A-Za-z0-9._/+-]+$` @ :118 plus `*..*` refusal @ :119; pin-line field count `3` @ :114
8. exit codes `5` refuse / `6` could-not-check / `2` usage @ :71-73; install mode `0755` @ :206, :218

Entries 6-8 are reversible or fail closed (a wrong home or tag yields a refused or failed fetch,
never an unverified install) and need no derivation.

- `RISK-VALUE: DERIVED — is_sha256 = ^[0-9a-f]{64}$ @ plugins/assay/scripts/assay-install.sh:95 — a sha256 digest is 256 bits = exactly 64 hex characters; sha256sum, shasum -a 256 and openssl dgst -r all print lowercase, as do the release checksums the pins come from, so exact-length lowercase equality is the full digest with no prefix or case-fold leniency; malformed input fails closed (suite N3-N6).`
- `RISK-VALUE: DERIVED — curl --proto '=https' --proto-redir '=https', exit 1 → refuse @ plugins/assay/scripts/assay-install.sh:155,159 — the '=' form sets the allowed-protocol list to exactly https for the transfer and every redirect, which is the #1554 ruling verbatim; curl documents exit 1 as unsupported/disabled protocol, the error a disallowed hop raises; H1 observed it on a real plain-HTTP hop serving the good asset (exit 5, nothing written); -q first means a user curl config cannot re-enable -k (T2).`
- `RISK-VALUE: DERIVED — scheme allowlist https://* @ plugins/assay/scripts/assay-install.sh:149 — refuses every non-https initial URL before curl runs (H2 file://, N13 http://), the ruling's initial-URL half; case-variant schemes are refused too, which fails closed.`
- `RISK-VALUE: DERIVED — expected digest = pin_fields "$pins" @ plugins/assay/scripts/assay-install.sh:184 — the only value compared at :196; the script has no code path that reads checksums.txt or any fetched file as the expected value, and no wget or insecure-TLS fallback, matching DR-forge-neutral-11's accepted ruling that the pin file is the single source.`
- `RISK-VALUE: NAMED, NOT DERIVED — statusgen tag = v1.0.24 and per-platform sha256 (e.g. darwin-arm64 c7dcdc419674799333f4244ed4cdd9146916e766c4ff9c6fe1f9506dc260e1e8) @ plugins/assay/paired-versions.yaml:35-38 — these are the trust root the install's pin step copies into an adopter's .assay-versions, so after this change "the pin file" in practice means "this manifest as shipped in the plugin". Confirming them means comparing with the published v1.0.24 release's checksums.txt, a live fetch the offline envelope forbids. OPEN QUESTION for the human gate: confirm these digests against the published release, and confirm that a manifest in the plugin bundle is an acceptable single source of truth for a digest.`

**Core-system reviewer questions (answered).** (1) The single control is the sha256 equality at
assay-install.sh:196 against the pin-file digest. It is not alone: the post-acquisition proof
(`--version` == pinned tag at :451, `--lint` == 0 at :455) trips on a different signal in a later
step. (2) Yes, one row proves a lower layer catches a fault with the upper layer bypassed. Suite
R3 serves a binary whose digest matches its pin but whose `--version` names a different tag, and
the proof refuses it (exit 5). Rows 6 and 7 prove the digest step refuses in both of its failure
modes.

**Observations (not row failures).** (a) The witness executed prose rows as shell commands,
and for row 5 it ran a bare `gh` on a host that has one installed. The witness's row 5 `pass` is
false: `gh` printed usage and exited 0. That is a verifyrun defect class. (b) docs/adopting-assay.md
still carries `gh` invocations in manual-runbook verify steps (the Scenario 1 draft-PR check,
the reviewer-App installation check, and the upgrade section's checksums materialise line). The
label lines are marked optional convenience. The Verify rows scope task 4 to the skill text, so
whether the runbook needs the same cleanup is the reviewer's call.

VERIFY: PASS — all 13 rows pass on the observed output above. The notes on rows 5-9, 12 and
13 explain each witness verdict that differs. This is Evidence for the human gate, not a sign-off
(gate: human, sensitive-data: yes). One NAMED, NOT DERIVED value is still open.

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
