---
brief: assay:assay:windows-port:06
title: Manifest-driven bootstrap — resolve tag + sha256 from the committed manifest, and write PATH
why: >-
  Step 1 of a Windows install is meant to be one command. Today it is a command the adopter cannot
  run without first opening a release page, copying a 64-hex digest by hand, and pasting it as a
  mandatory parameter — `-Tag` and `-Sha256` are both `Mandatory=$true`
  (`scripts/bootstrap-windows.ps1:22-23`) — and then hand-editing the user PATH, because the script
  places a binary into `%LOCALAPPDATA%\Assay\bin` and never puts that directory on PATH
  (`bootstrap-windows.ps1:34-36`). Both are transcription surfaces: a digest typed by a human is a
  digest that can be typed wrong, and the correct answer is already committed in the repo the
  adopter just cloned. Resolving tag + sha from that manifest removes the transcription step
  WITHOUT weakening the control — the bootstrap still verifies-or-refuses, it just stops asking a
  human to supply the value it is verifying against.
wave: 3
depends: ["windows-port/03"]
unblocks: ["windows-port/09"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-11 by windows-port authoring session (driver ask, 2026-09-11)
sources:
  - "driver's ask (2026-09-11): a Windows adopter should install in three commands, the first being `powershell -File scripts/bootstrap-windows.ps1 -Tag vX.Y.Z` with no operator-supplied sha and PATH written"
  - "scripts/bootstrap-windows.ps1:22-23 — `[Parameter(Mandatory=$true)][string]$Tag` and `[Parameter(Mandatory=$true)][ValidatePattern('^[0-9a-f]{64}$')][string]$Sha256`: BOTH mandatory today"
  - "scripts/bootstrap-windows.ps1:32-33 — the verify-or-refuse control this brief must preserve verbatim in effect (`Get-FileHash` then `throw \"REFUSED: sha256 mismatch …\"`)"
  - "scripts/bootstrap-windows.ps1:34-36 — New-Item/Move-Item/Write-Host: the script creates $Dest and places the asset, and writes NO PATH entry"
  - "plugins/assay/paired-versions.yaml:37-41 — the committed manifest already carries `windows-amd64: statusgen-windows-amd64.exe <tag> <sha256>` under `statusgen.platforms`, the exact value the operator is asked to transcribe"
  - "plugins/assay/scripts/check-paired-versions.sh:17-21 — checks B (SINGLE TAG) and C (HASH SHAPE, 64 lowercase hex) already guard that manifest in CI: the second, independent layer behind the download verify"
  - ".github/workflows/windows-ci-leg.yml:156-163 — the `windows-bootstrap-smoke` job ALREADY extracts tag+sha from paired-versions.yaml in bash before calling the bootstrap; this brief moves that same resolution into the script so the adopter gets what CI already has"
  - "scripts/windows-bootstrap-hashcheck-smoke.ps1 — the existing three-assertion Windows-runtime driver (tamper→refuse, pinned→install, check-removed→installs) this brief extends rather than duplicates"
  - "tools/desk/cmd/deskinstall/install.go:188-194 (`place`) — deskinstall writes the asset under its ASSET name (`statusgen-windows-amd64.exe`), not `statusgen.exe`; the bootstrap does the same at :35, so `statusgen` is not invocable by bare name after either step"
  - "docs/adopting-assay.md:932-946 — the current walkthrough's steps 1-2, which tell the adopter to copy the sha256 from the release's paired-versions.yaml / checksums.txt by hand"
  - "freshness-checked 2026-09-11 @ 35316469 (origin/main): bootstrap-windows.ps1 is 36 lines, both params still mandatory, no PATH write, no manifest read"
consumers:
  - "scripts/bootstrap-windows.ps1: follow-up windows-port/06 (this brief; flips to fixed-here when the implementation edits the path)"
  - "scripts/windows-bootstrap-hashcheck-smoke.ps1: follow-up windows-port/06 (this brief; the negative-path assertions land with the implementation)"
  - ".github/workflows/windows-ci-leg.yml: follow-up windows-port/06 (this brief; the `windows-bootstrap-smoke` step stops pre-extracting tag+sha once the script resolves them)"
  - "docs/adopting-assay.md: follow-up windows-port/09 (the walkthrough collapse owns the doc edit, not this brief)"
  - "plugins/assay/skills/install/SKILL.md: follow-up windows-port/09 (§Scope's acquisition-only wording is 09's edit)"
  - "plugins/assay/paired-versions.yaml: out-of-scope (this brief READS the manifest; it never re-pins it — a re-pin is the release chain's act, guarded by check-paired-versions.sh)"
exec-tier: strong
exec-tier-why: >-
  Question (c): correctness depends on a security control at an acquisition trust boundary
  (sha256-verify-or-refuse), and the change moves the value the control compares against from an
  operator argument to a parsed file — a subtle parse error (wrong column, silent empty match,
  a fall-through when the platform line is absent) would leave the check comparing against
  nothing and survive a happy-path test.
version: 1
id: 23362df7-69a4-4278-83e7-a5f5ed192927
---

# Brief 06 — Manifest-driven bootstrap

## Context

files:
- **edit** `scripts/bootstrap-windows.ps1` — `-Sha256` stops being a mandatory parameter; the
  script resolves `tag` + `sha256` for the detected `windows-<arch>` from the committed manifest,
  keeps the verify-or-refuse, and writes the user PATH entry.
- **edit** `scripts/windows-bootstrap-hashcheck-smoke.ps1` — add the manifest negative-path
  assertions (tampered manifest, absent platform line) alongside the three it already runs.
- **edit** `.github/workflows/windows-ci-leg.yml` — the `windows-bootstrap-smoke` step no longer
  pre-extracts tag+sha in bash (lines 156-162); it calls the driver, which calls the script that
  resolves them. **NOTE:** an App credential cannot push a workflow file — stage the workflow edit
  in the PR and name it for a maintainer, exactly as `windows-port/04` did.
- **create** `changelog/<branch-slug>.md` — this repo enforces a per-PR fragment
  (`changelog/README.md` exists).
- **do NOT** edit `docs/adopting-assay.md` or `plugins/assay/skills/install/SKILL.md` — the
  walkthrough collapse is `windows-port/09`'s.

facts:
- **Both parameters are mandatory today.** `scripts/bootstrap-windows.ps1:22-23`:
  `[Parameter(Mandatory=$true)][string]$Tag` and
  `[Parameter(Mandatory=$true)][ValidatePattern('^[0-9a-f]{64}$')][string]$Sha256`. The target
  shape is `-Tag vX.Y.Z` alone, with `-Sha256` retained as an OPTIONAL override that, when
  supplied, must MATCH the manifest or refuse (a mismatch between two pinned sources is a defect,
  never a precedence question).
- **The answer is already in the repo the adopter just cloned.** `plugins/assay/paired-versions.yaml`
  carries, under `statusgen.platforms`, the line
  `windows-amd64: statusgen-windows-amd64.exe v1.0.6 235e87d0c36e22f9798b7f9441fda8ec902a0cc2b5adf3f340ec5cf29f33916a`
  (lines 37-41 at the freshness head), plus the `windows-arm64` twin and a sibling `desk-tools`
  block. Grammar: `<artifact> <tag> <sha256>`, three whitespace-separated fields.
- **CI already does this exact resolution, in bash.** `.github/workflows/windows-ci-leg.yml:156-162`
  greps the `windows-amd64:` line out of `paired-versions.yaml` and `awk`s fields 3 and 4 into
  `tag` / `sha` before invoking the driver. The adopter is asked to do by hand what the runner
  already does mechanically — that asymmetry is the whole defect.
- **The control to preserve, verbatim in effect:** `bootstrap-windows.ps1:32-33` computes
  `Get-FileHash -Algorithm SHA256` and, on inequality, `Remove-Item`s the download and
  `throw`s `"REFUSED: sha256 mismatch for $asset (got $got, pinned $Sha256) — no unverified bytes
  installed"`. It runs BEFORE `New-Item`/`Move-Item` (:34-35). That ordering is load-bearing and
  must not move.
- **The second, independent layer already exists and is named:**
  `plugins/assay/scripts/check-paired-versions.sh` asserts, offline and in CI, check B (every
  pinned tag is the SAME tag) and check C (every sha256 is exactly 64 lowercase hex) over that
  manifest (its header, lines 17-21). The download check and the manifest check fail for different
  reasons in different components — that is the independence test, not two copies of one assert.
- **PATH is never written.** `bootstrap-windows.ps1:34-36` creates `$Dest`
  (`%LOCALAPPDATA%\Assay\bin` by default), moves the asset in, and prints. Nothing touches
  `[Environment]::SetEnvironmentVariable('Path', …, 'User')`. `docs/adopting-assay.md:909-910`
  records the consequence in prose ("A `deskinstall` that succeeds in one session does not update
  already-open shells") — the doc describes the gap rather than the tool closing it.
- **The asset lands under its ASSET name, not its command name.** Both the bootstrap
  (`:35`, `Move-Item … (Join-Path $Dest $asset)`) and `deskinstall`
  (`tools/desk/cmd/deskinstall/install.go:192-193`, `filepath.Join(destDir, p.asset)`) write
  `statusgen-windows-amd64.exe`. With `$Dest` on PATH, `statusgen --version` still does not
  resolve — only `statusgen-windows-amd64 --version` does. The three-command target requires a
  `statusgen.exe` the adopter can actually invoke.
- **Windows-runtime rows cannot be discharged by an offline non-Windows verifier.** This is
  observed, not theoretical: `windows-port/03`'s row 8 came back could-not-check for exactly this
  reason on two separate verifier runs. Split the Verify table accordingly — parse/refusal logic
  that a POSIX verifier can exercise, and a clearly-labelled Windows-runtime row that CI owns.

single-point-of-failure: the sha256-verify-or-refuse at `bootstrap-windows.ps1:32-33` — the ONE
control between a substituted release asset and a Windows adopter executing unverified bytes.
Two independent layers behind it, and this brief must not collapse them into one: (1) the expected
digest comes from `plugins/assay/paired-versions.yaml`, a version-committed artifact changed only
by a reviewed re-pin, never fetched live alongside the asset; (2) that manifest is itself guarded
by `plugins/assay/scripts/check-paired-versions.sh` checks B and C, which run in CI, in a
different component, on a different signal (shape and tag-coherence of the pin file) than the
download comparison. A tampered manifest therefore has to defeat a reviewed diff AND a CI shape
check before it ever reaches the download comparison. NONE is not the answer here.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- **Do NOT weaken the verify-or-refuse to simplify the resolution.** Moving the expected digest
  from an argument to a parsed file must not introduce ANY path where the comparison is skipped:
  an unreadable manifest, an absent `windows-<arch>` line, a malformed pin, or a tag that does not
  match the requested `-Tag` are all REFUSALS with a named reason, never a fall-through to
  "download and hope". Per the security-gate rule, removing or softening the check is
  `BLOCKED-ON-HUMAN`, not a shortcut.
- Do NOT hand-edit `plugins/assay/paired-versions.yaml`. This brief reads it.
- An App credential cannot push a `.github/workflows/**` file. Stage the workflow edit and name it
  for a maintainer in the PR body rather than attempting the push.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **Resolve the pin in the script.** Add a manifest read to `scripts/bootstrap-windows.ps1`:
   locate `plugins/assay/paired-versions.yaml` relative to `$PSScriptRoot` (the script already
   assumes it lives in a clone — `docs/adopting-assay.md:932` says so outright), find the
   `statusgen.platforms` entry for the detected `windows-<arch>`, and split its value into
   `<artifact> <tag> <sha256>`.
2. **Make `-Sha256` optional and `-Tag` sufficient.** `-Tag` selects which pinned release is being
   installed; the sha comes from the manifest line for that tag. Keep `-Sha256` as an optional
   override whose ONLY legal use is to re-assert the manifest value: supplied-and-equal proceeds,
   supplied-and-different REFUSES naming both values. Keep the existing
   `ValidatePattern('^[0-9a-f]{64}$')` on it.
3. **Refuse, with a named reason, on every resolution failure.** Manifest file absent or
   unreadable; no `statusgen.platforms` block; no line for the detected platform; a pin line that
   does not split into exactly three fields; a sha that is not 64 lowercase hex; a manifest tag
   that differs from the requested `-Tag`. Each is a distinct, greppable refusal message and a
   non-zero exit. Nothing is downloaded before resolution succeeds, and nothing is placed before
   the hash matches — the two-phase ordering at `:32-35` is preserved.
4. **Write the user PATH entry, idempotently.** After a successful place, add `$Dest` to the USER
   `Path` (`[Environment]::SetEnvironmentVariable('Path', …, 'User')`) only when it is not already
   a segment of it — compare segment-wise, not by substring, so `…\Assay\bin2` never suppresses
   `…\Assay\bin`. Print what was written, and print the standing caveat that already-open shells
   do not pick it up.
5. **Make the installed binary invocable by name.** Place (or additionally place) the verified
   bytes as `statusgen.exe` in `$Dest`, so that with PATH written `statusgen --version` resolves.
   Keep the asset-named copy if removing it would break `deskinstall`'s or CI's expectations —
   verify which before choosing, and say which you chose in the PR body.
6. **Extend the existing Windows-runtime driver, do not write a second one.** Add to
   `scripts/windows-bootstrap-hashcheck-smoke.ps1` the manifest negative paths: a TAMPERED manifest
   digest (one hex character flipped) REFUSES and installs nothing; an ABSENT platform line REFUSES
   and installs nothing. Keep its existing non-vacuity assertion shape — a check-removed copy must
   still let the bad input through — so a green refusal cannot be vacuous.
7. **Simplify the CI step to prove the script does the work.** In
   `.github/workflows/windows-ci-leg.yml`, drop the bash pre-extraction at lines 156-162 and pass
   only the tag; the step passing is then evidence that the SCRIPT resolved the sha, because
   nothing else did.
8. **Add the changelog fragment** (`changelog/<branch-slug>.md`), one or more highlight bullets.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `grep -n 'Mandatory=$true' scripts/bootstrap-windows.ps1 \| grep -c 'Sha256'` | `0` — `-Sha256` is no longer a mandatory parameter | `check` |
| 2 | `grep -c 'Mandatory=$true' scripts/bootstrap-windows.ps1` | `1` — exactly one mandatory parameter remains (`-Tag`) | `check` |
| 3 | `grep -qF 'paired-versions.yaml' scripts/bootstrap-windows.ps1; echo $?` | `0` — the script names the committed manifest it resolves from | `check` |
| 4 | The verify-or-refuse survives and still precedes placement: `awk '/Get-FileHash/{h=NR} /REFUSED: sha256 mismatch/{r=NR} /Move-Item/{m=NR} END{print (h>0 && r>h && m>r) ? "ORDER-OK" : "ORDER-BROKEN"}' scripts/bootstrap-windows.ps1` | `ORDER-OK` | `check` |
| 5 | No floating ref is introduced: `grep -niE '(^\|[^a-z])latest([^a-z]\|$)' scripts/bootstrap-windows.ps1 && echo USES-LATEST \|\| echo NO-LATEST` | `NO-LATEST` | `check` |
| 6 | A user PATH write exists and is segment-guarded: `grep -qF "SetEnvironmentVariable" scripts/bootstrap-windows.ps1 && grep -qiE "'User'\|\"User\"" scripts/bootstrap-windows.ps1; echo $?` | `0` | `check` |
| 7 | Every resolution failure has its own refusal message (not one catch-all): `grep -coE 'throw \"REFUSED' scripts/bootstrap-windows.ps1` | `>= 5` — one per failure class named in Task 3 plus the download mismatch | `check` |
| 8 | **Positive path, Windows runtime** — `windows-bootstrap-smoke` on `windows-latest` runs `scripts/windows-bootstrap-hashcheck-smoke.ps1 -Tag <pinned>` with NO `-RealSha256`, and the untampered run installs | a green `windows-bootstrap-smoke` whose log shows the bootstrap printing the sha it resolved from the manifest, matching the manifest's own line; record the run URL | `check:ci` |
| 9 | **NEGATIVE PATH — tampered manifest (the security row)** — the driver flips one hex character of the `windows-amd64` digest in a scratch copy of `paired-versions.yaml` and runs the bootstrap against it | the bootstrap REFUSES with a `sha256 mismatch` message, exits non-zero, and `$Dest` contains no `statusgen*.exe` | `check:ci +mutation` |
| 10 | **NEGATIVE PATH — absent platform line** — the driver removes the `windows-amd64:` line from a scratch copy and runs the bootstrap | the bootstrap REFUSES naming the missing platform, exits non-zero, nothing downloaded, nothing placed | `check:ci +mutation` |
| 11 | **NON-VACUITY for rows 9-10** — the same two tampered inputs, run against a copy of the bootstrap with the resolution-refusal lines removed, DO proceed | the check-removed copy installs / attempts the download, proving rows 9-10 redden because of the guard and not an unrelated failure | `check:ci +mutation` |
| 12 | **Fail-first for rows 9-11** — capture the row-9 assertion RED against the pre-change script (which has no manifest resolution to tamper with, so the driver's new assertion cannot pass) or against a mutation of the new script with the refusal stubbed out | the new assertion observed FAILING, pasted under `## Fail-first` in the PR body with the commit/mutation it ran against | `check` |
| 13 | **`-Sha256` override disagreement refuses** — `pwsh -File scripts/bootstrap-windows.ps1 -Tag <pinned> -Sha256 <64 hex that is not the manifest's>` | non-zero exit, a refusal naming BOTH the supplied and the manifest digest; nothing downloaded | `check:ci` |
| 14 | **The binary is invocable by name after step 1** — on the Windows runner, after the bootstrap, `statusgen --version` (bare name, PATH-resolved in a fresh shell) | prints the pinned tag; exit 0 | `check:ci +dereference` |
| 15 | **Dereference the pin against the manifest** (catches a wrong-but-well-formed script): `line=$(grep -E '^[[:space:]]*windows-amd64:[[:space:]]+statusgen-windows-amd64\.exe' plugins/assay/paired-versions.yaml \| head -1); echo "$line" \| awk '{print $3, $4}'` then confirm the row-8 log printed the SAME tag and digest | the two agree exactly | `gate:model +dereference` |
| 16 | Consumers routing corroborated by the diff (run on the implementer's branch): `statusgen --root . --consumers windows-port/06; echo $?` | `0` | `check` |
| 17 | Board lint stays clean: `statusgen --root . --lint` | `0` PROBLEMs | `check:ci` |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). Rows 8-14 are
     Windows-runtime / online rows — an offline POSIX verifier records them as
     could-not-check with the reason, never greened from the static rows. -->

## Review
Gate: **model** (from frontmatter — all four risk answers no). The reviewer's two questions on this
surface: (1) is there any input — unreadable manifest, absent platform line, malformed pin, tag
disagreement — on which the script downloads or places WITHOUT a digest comparison? Rows 7, 9, 10
and 13 exist to make that answerable from evidence rather than from reading. (2) Do rows 9-11 prove
the refusal is caused by the guard, or only that something failed? Row 11 is the non-vacuity
control; a row-9 green with no row-11 has verified nothing.
