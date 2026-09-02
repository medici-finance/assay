---
brief: windows-port/05
title: Adoption-doc delta — the Windows adopter walkthrough
why: >-
  The whole stream is only real to an adopter when the runbook tells them how to do it. Today
  both surfaces stop at Windows, in their own words: `docs/adopting-assay.md` (Prerequisites)
  says "**Windows** is a named fast-follow, not yet in scope — on a Windows host the skill stops
  at that step rather than guessing", and `plugins/assay/skills/install/SKILL.md` §Scope says
  "**Windows is a deferred fast-follow — NOT in this skill's scope yet.**" This brief retires
  both deferrals and replaces them with a real Windows walkthrough — the install path, the pinned release, the CI-proven status, and the
  triaged surfaces that need a documented workaround — so a Windows adopter has a path, not a
  dead end.
wave: 2
depends: ["windows-port/02", "windows-port/03", "windows-port/04"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-09-01 by windows-port authoring session
sources:
  - "Ian's direction (2026-09-01): the Windows adopter walkthrough, mirroring the existing adopting-assay docs' per-profile pattern; end state — a Windows adopter runs the pinned release, CI-proven on Windows"
  - "docs/adopting-assay.md: the runbook has per-SCENARIO sections (green-field / existing-suite / carve-out) but NO per-OS axis; OS handling lives only inside PRIMITIVE: install-statusgen's uname detection and the skill's §Scope Windows-deferred note"
  - "plugins/assay/skills/install/SKILL.md §Scope (lines ~232-243): 'Unix-first (mac/linux). ... Windows is a deferred fast-follow — NOT in this skill's scope yet. The Windows arm (the statusgen-windows-amd64.exe asset, a cross-platform hash-verify, and the .exe install path) is a future, not-yet-authored follow-up. ... when a Windows host is detected, say the acquisition arm is not yet available on this platform and stop, rather than guessing' — the deferral this brief retires; note the SKILL's own wording differs from adopting-assay.md's, and each is quoted from its own file"
  - "docs/adopting-assay.md (Prerequisites, lines ~36-38): '**macOS or Linux.** The statusgen binary acquisition is Unix-first; **Windows** is a named fast-follow, not yet in scope — on a Windows host the skill stops at that step rather than guessing.' — the second deferral this brief retires"
  - "windows-port/01 (release build matrix): the source of the statusgen-windows-<arch>.exe asset names the walkthrough tells the adopter to pin in .assay-versions"
  - "windows-port/02 (portability audit): the documented-workaround rows the doc lifts verbatim"
  - "windows-port/03 (install path): the ruled+built Windows install path the doc walks through"
  - "windows-port/04 (windows CI leg): the green Windows CI the doc cites as the CI-proven evidence"
  - "freshness-checked 2026-09-01 @ origin/main: adopting-assay.md has no Windows walkthrough; the only Windows mention is the deferral"
consumers:
  - "docs/adopting-assay.md: fixed-here (a new Windows walkthrough / per-OS section)"
  - "plugins/assay/skills/install/SKILL.md: fixed-here (the §Scope 'Windows deferred' note is updated to point at the now-real path — the ONE cross-file edit this brief owns)"
---

# Brief 05 — Adoption-doc delta: the Windows adopter walkthrough

## Context

files:
- **amend** `docs/adopting-assay.md` — add the Windows walkthrough. Mirror the existing
  per-scenario structure: the runbook already routes green-field / existing-suite / carve-out;
  the Windows axis is orthogonal (an adopter on Windows follows their scenario AND the Windows
  install section), so the cleanest shape is a **`## Windows adopters` section** the scenario
  sections cross-reference at the install step, not a fourth scenario.
- **amend** `plugins/assay/skills/install/SKILL.md` — update the §Scope "Windows deferred" note to
  point at the now-real path. This is the one cross-file edit this brief owns (the shared-value
  note that install briefs 03 deliberately left for the doc delta).

facts:
- **The doc has no per-OS axis today.** It is per-scenario; OS handling hides inside
  `PRIMITIVE: install-statusgen`'s `uname` detection. The Windows section adds the axis the doc
  lacks, and points the install PRIMITIVE at brief 03's Windows install path.
- **What the walkthrough must contain** (checkable facts an adopter acts on, so this doc gets a
  dereferencing Verify row, not only presence checks): (1) prerequisites (Windows version, Go /
  `gh` if the ruled install fork needs them); (2) the install path from brief 03 — the exact
  command and that it sha256-verifies-or-refuses; (3) the pinned-release model — the adopter pins
  `statusgen-windows-<arch>.exe` in `.assay-versions` (brief 01's asset names); (4) the
  CI-proven status — a pointer to the green Windows CI leg (brief 04) as the corroboration; (5)
  the documented-workaround rows from brief 02 (the `bash`+`jq` SessionStart hook is the likely
  one — state the Git-Bash / manual step honestly rather than implying it just works); (6) the
  native-vs-WSL statement — the claim is native Windows; WSL is a noted fallback, not the path.
- **Do not overclaim.** Where brief 02 marks a surface `documented-workaround` or brief 04 marks
  arm64 native-smoke BLOCKED, the doc says so plainly. An adopter doc that claims full parity
  the stream did not deliver is the failure mode this brief exists to avoid.
- **Regeneration:** if `docs/adopting-assay.md` feeds any generated index or the `adopt` skill's
  routing, note the regen in DoD (the runbook is hand-maintained prose today; confirm no
  generator must run, or run it).

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- Do NOT claim a surface works when brief 02 triaged it `documented-workaround` or brief 04 held
  it BLOCKED — the doc mirrors the delivered state, not the aspiration.
- If anything is unclear or contradicts repo state (e.g. brief 03's fork ruling changed the
  install command): report NEEDS_CONTEXT, don't guess.

## Task
1. Add a `## Windows adopters` section to `docs/adopting-assay.md` covering the six items in
   `facts:`, cross-referenced from the install step of each scenario section.
2. Update `plugins/assay/skills/install/SKILL.md` §Scope: replace its "Windows is a deferred
   fast-follow — NOT in this skill's scope yet" paragraph with a pointer to the now-real Windows
   path (keep the honesty: name any remaining documented-workaround surfaces). Retire the matching
   deferral in `docs/adopting-assay.md`'s Prerequisites ("**Windows** is a named fast-follow, not
   yet in scope …") in the same pass — the two are worded differently and both must go.
3. Lift brief 02's `documented-workaround` rows verbatim into the walkthrough's "known gaps"
   subsection — do not re-summarize them (drift risk).
4. Cite brief 04's Windows CI leg as the "CI-proven on Windows" evidence, with the arm64
   native-smoke caveat stated.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | A Windows section exists: `grep -qiE '^## .*Windows' docs/adopting-assay.md; echo $?` | `0` |
| 2 | It names the pinned windows asset (brief 01): `grep -qE -e 'statusgen-windows-amd64[.]exe' -e 'statusgen-windows-arm64[.]exe' docs/adopting-assay.md; echo $?` | `0` |
| 3 | It documents the verify-or-refuse install control (brief 03): `grep -qiE -e 'sha256' -e 'hash' docs/adopting-assay.md && grep -qiE -e 'refus' -e 'verif' docs/adopting-assay.md; echo $?` | `0` |
| 4 | The install SKILL's §Scope no longer flatly defers Windows — grep the file's OWN deferral sentence, verbatim: `grep -qiF "Windows is a deferred fast-follow" plugins/assay/skills/install/SKILL.md; echo $?` | `1` — the deferral sentence is gone (replaced by the real-path pointer). **This row is non-vacuous by construction: on pre-brief main the same grep returns `0`, because that exact sentence is present at `plugins/assay/skills/install/SKILL.md` §Scope today.** |
| 4a | **Positive control for row 4** — the SKILL now references the Windows path: `grep -qiF 'windows' plugins/assay/skills/install/SKILL.md; echo $?` | `0` (Windows is still mentioned — now as a supported path, not a deferral) |
| 4b | The doc's own deferral sentence is retired too — grep `adopting-assay.md`'s verbatim wording: `grep -qiF "is a named fast-follow, not yet in scope" docs/adopting-assay.md; echo $?` | `1` — gone. Non-vacuous by construction: on pre-brief main this grep returns `0` (the sentence sits in the Prerequisites list). |
| 5 | **Dereferencing — the doc's CI claim resolves to a REAL windows leg** (a wrong-but-well-formed doc that claims CI without one goes red here): `grep -qiE -e 'CI-proven' -e 'windows CI' -e 'windows-latest' docs/adopting-assay.md && grep -rqE 'runs-on: *windows-latest' .github/workflows/; echo $?` | `0` — the doc claims CI proof AND a windows-latest leg actually exists in the workflows |
| 6 | **Dereferencing — a documented-workaround the doc lists is a REAL triaged surface** (not invented): `grep -qiE -e 'git.?bash' -e 'jq' -e 'SessionStart' -e 'hook' docs/adopting-assay.md && grep -qiF 'documented-workaround' docs/streams/windows-port/portability-audit.md; echo $?` | `0` — the doc's known-gap references a surface the audit actually triaged as a workaround |
| 7 | Native-not-WSL is stated: `grep -qiF 'native' docs/adopting-assay.md && grep -qiF 'wsl' docs/adopting-assay.md; echo $?` | `0` — both the native claim and the WSL-fallback note are present |
| 8 | **Consumers routing corroborated by the diff** (run on the implementer's branch): `statusgen --root . --consumers windows-port/05; echo $?` | `0` — both fixed-here edits (adopting-assay.md + the install SKILL §Scope) are proved by the branch diff |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     Rows 5 and 6 are the dereferencing rows: they fail on a doc that CLAIMS Windows CI /
     a triaged workaround the delivered surfaces do not carry. "verified" requires a
     non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|

## Review
Gate: **model** (from frontmatter). All four risk answers are `no` — documentation. The
dereferencing rows (5, 6) are the review's focus: they catch a confident-but-wrong walkthrough
(one claiming Windows CI or a workaround the stream never delivered), which a presence-only grep
could not. The reviewer also confirms the doc does not overclaim past what briefs 02/03/04
delivered — the arm64 caveat and any documented-workaround surfaces are stated, not hidden.
