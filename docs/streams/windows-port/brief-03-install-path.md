---
brief: windows-port/03
title: Windows install path — PowerShell-vs-Go-installer fork, then build
why: >-
  A Windows adopter needs the same thing a Unix adopter gets from `assay:install`: a
  version-pinned, sha256-verified statusgen/desk-tools build placed where the tools can find it,
  with a hash mismatch a hard refusal — not a warning. There is no `desk-install` equivalent on
  Windows today (the Unix path is `sudo make desk-install` into `/opt/desk-tools/bin`). This
  brief decides HOW that install is delivered — a PowerShell script vs a Go-native install
  subcommand — surfacing the fork for a maintainer ruling rather than pre-deciding it, then
  builds the chosen one with the verify-or-refuse control intact.
wave: 1
depends: ["windows-port/01", "windows-port/02"]
unblocks: ["windows-port/05"]
effort: L
gate: human
gate-why: >-
  The PowerShell-script-vs-Go-installer fork is a design commitment, not a risk boolean: it
  fixes the maintenance surface every future Windows adopter and CI leg inherits, and both the
  CI leg (04) and the adoption doc (05) bind to whichever is chosen. Only the maintainer commits
  that fork — the same reservation harness-portability/03 made for the target-set/channel ruling.
  The human is confirming WHICH installer to build and maintain; the four risk answers are all
  no (git-revertible adopter tooling). It is authored L rather than split to M because the
  ruling and its thin, single implementation are one reviewable unit — splitting would strand the
  ruling with no consumer until its build brief lands.
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
decision-trigger: creation
issues: []
schema: brief-v1
authored: 2026-09-01 by windows-port authoring session
sources:
  - "Ian's direction (2026-09-01): the desk-install equivalent for Windows — DECIDE PowerShell script vs a Go installer and SURFACE that fork explicitly (pros/cons, recommendation) rather than pre-deciding silently"
  - "plugins/assay/skills/install/SKILL.md §Scope: the deferred Windows arm names exactly this work — the statusgen-windows-amd64.exe asset, a cross-platform hash-verify, the .exe install path"
  - "docs/adopting-assay.md (PRIMITIVE: install-statusgen ~267, install-desk-tools ~306): the Unix acquire→verify→place flow this mirrors — resolve tag+sha256 from paired-versions.yaml, gh release download, shasum -a 256 compare, REFUSE on mismatch, install -m 0755"
  - "survey (2026-09-01): platform detection is uname-based (plat=$(uname -s|…)-$(uname -m|…)); Windows needs its own detection. Unix install is sudo make desk-install → /opt/desk-tools/bin"
  - "windows-port/01: emits statusgen-windows-<arch>.exe + desk-tools-windows-<arch>.tar.gz (the assets this installs)"
  - "windows-port/02: the portability audit — whether a shell installer even runs, and the config-home (%APPDATA% vs ~/.config) recommendation"
  - "freshness-checked 2026-09-01 @ origin/main: no .ps1 or install.sh exists anywhere in the repo; install is Claude-Code-orchestrated"
consumers:
  - "the chosen installer artifact (scripts/install-windows.ps1 OR a tools/desk install subcommand): fixed-here"
  - "plugins/assay/skills/install/SKILL.md: follow-up windows-port/05 (§Scope's 'Windows deferred' note is lifted once this lands; the doc delta owns that edit)"
  - "docs/adopting-assay.md: follow-up windows-port/05 (the Windows walkthrough documents this install path)"
  - "docs/streams/windows-port/brief-04-windows-ci-leg.md: follow-up windows-port/04 (CI exercises this install path as the smoke's install step, if the fork yields a scriptable installer)"
exec-tier: strong
exec-tier-why: >-
  Correctness depends on a security control (sha256-verify-or-refuse at the acquisition trust
  boundary) that a subtle implementation error would leave silently bypassed — question (c).
---

# Brief 03 — Windows install path: the fork, then the build

## Context

files:
- **create** the chosen installer — EITHER `scripts/install-windows.ps1` (PowerShell path) OR a
  Go-native `install` path in `tools/desk` (e.g. a `deskinstall` cmd / a `statusgen install`
  subcommand). The fork is ruled in `## Human decision` before either is written.
- **do NOT** edit `plugins/assay/skills/install/SKILL.md` §Scope or `docs/adopting-assay.md` here —
  those are brief 05's (the doc delta owns lifting the "Windows deferred" note once this lands).

facts:
- **The Unix flow this mirrors, step for step:** resolve `tag` + `sha256` for the detected
  platform from the plugin's shipped `plugins/assay/paired-versions.yaml` (never `latest`) →
  download the release asset for that platform → **compute sha256 and compare; REFUSE on
  mismatch** → place the binary where the tools resolve it. `assay:install`'s rule is verbatim:
  "a hash mismatch is a hard REFUSE, not a warning, so no unverified bytes are ever installed."
- **What is Windows-specific and must be built:** (1) platform detection — the Unix path is
  `uname`-based, which does not exist on native Windows; detect `windows-amd64` /
  `windows-arm64` from the OS/arch (`$env:PROCESSOR_ARCHITECTURE` in PowerShell, or
  `runtime.GOOS`/`GOARCH` in Go). (2) The asset names carry `.exe` (brief 01):
  `statusgen-windows-<arch>.exe`, `desk-tools-windows-<arch>.tar.gz`. (3) The install
  DESTINATION and how the tools find it (a `%APPDATA%`/`%LOCALAPPDATA%` bin dir on `PATH`, vs a
  chosen dir) — take the recommendation from brief 02's config-home row.
- **This is the security-relevant surface of the whole stream.** The install path is the trust
  boundary where an unverified or tampered binary would otherwise execute. The verify-or-refuse
  control is load-bearing and gets a NEGATIVE-PATH Verify row (a byte-flipped binary is REFUSED,
  non-zero exit, binary NOT placed).
- **`gh` availability differs by fork:** the PowerShell path can shell `gh release download` (gh
  runs on Windows) OR fetch via `Invoke-WebRequest` against the release asset URL; the Go path
  can fetch over `net/http` with no external dependency. The fork weighs this.

single-point-of-failure: the sha256-verify-or-refuse check at acquisition — it is the ONE
control standing between a substituted release asset and a Windows adopter running unverified
bytes. Second layer behind it: the pin resolves from the plugin-shipped, version-committed
`paired-versions.yaml` (never `latest`), so the tag+expected-hash the check compares against are
themselves pinned in a reviewed artifact rather than fetched live — the check and the value it
checks against fail for different reasons (a tampered download vs a tampered/renamed pin file),
which is the independence test. NONE is not the answer here: the two layers are real and named.

## Human decision
<!-- gate: human, decision-trigger: creation — filed as a self-contained decision issue.
     Written to be decided from THIS text alone: no links, no repo paths, no brief refs. -->
Windows needs an install path equivalent to the Unix one (a version-pinned, hash-verified
download of the tool binaries, placed where the tools can find them, refusing to install if the
hash does not match). Two ways to deliver it, and the choice fixes what the project maintains
for every future Windows adopter and what the Windows CI exercises. Pick one before it is built.

Options:
1. **A PowerShell install script.** Pros: idiomatic on Windows, no compile step, a Windows admin
   can read and audit it, mirrors how most CLI tools ship a Windows installer, works before any
   tool binary is present (so it can bootstrap the very first install). Cons: a second
   implementation language to maintain alongside the Unix path, PowerShell execution-policy
   friction on locked-down machines, and the hash-verify logic lives in script rather than in
   tested Go.
2. **A Go-native install subcommand** (the tool installs itself / a small installer binary).
   Pros: one language, the hash-verify runs in code already covered by the Go test suite,
   cross-platform by construction (the same subcommand serves Unix too), no shell dependency.
   Cons: a chicken-and-egg for the FIRST install (you need a binary to run the installer, so the
   very first download still needs a tiny bootstrap — a one-line `Invoke-WebRequest` or a
   released installer asset), and it grows the tool's surface.

Recommendation: **Option 2 (Go-native), with a ~5-line PowerShell bootstrap** that fetches only
the installer/first binary. It keeps the security-critical hash-verify in tested Go (one
implementation, one test suite, cross-platform), and confines PowerShell to a trivial,
auditable bootstrap that itself verifies before executing. This also lets the Unix path converge
on the same subcommand over time, retiring `sudo make desk-install`'s POSIX-only shape.

Default if no answer: none — blocks until answered. The installer cannot be built until the fork
is ruled; guessing bakes in exactly the decision this gate exists to make.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task
  instructions.
- Stop at `implemented` — you do not set verified/done.
- Do NOT weaken or omit the sha256-verify-or-refuse control to simplify the installer. It is the
  security floor; a "warn and continue" is a rejected design, and per the security-gate rule,
  removing it is BLOCKED-ON-HUMAN, not a shortcut.
- Do NOT begin building until the `## Human decision` fork is ruled (recorded on the decision
  issue). Report NEEDS_CONTEXT if picked up before the ruling.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. (Human gate) Obtain the fork ruling (PowerShell vs Go-native) recorded on the decision issue.
2. Build the chosen installer to mirror the Unix acquire→verify→place flow: detect
   `windows-<arch>`, resolve tag+sha256 from `paired-versions.yaml`, download
   `statusgen-windows-<arch>.exe` and `desk-tools-windows-<arch>.tar.gz`, **verify sha256 and
   REFUSE on mismatch**, place them on a `PATH`-resolvable dir (per brief 02's config-home
   recommendation).
3. Make the verify-or-refuse the FIRST thing that runs after the download and BEFORE any
   placement or execution — a mismatch leaves nothing installed.
4. Add a test that exercises the refusal: a binary whose bytes do not match the pinned hash is
   REFUSED and NOT placed (the negative-path row below). Show it failing on the unfixed code per
   the fail-first rule (a version of the installer with the check disabled must let the bad
   binary through — capture that red).
5. Emit a clear success line naming the installed version + verified hash, so brief 04's CI smoke
   and brief 05's doc can assert on it.

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | The chosen installer artifact exists (one of the two forks): `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); test -n "$inst" && echo "$inst"` | a path printed (exit 0) — the ruled fork's artifact is present |
| 2 | It resolves the pin from `paired-versions.yaml`, never `latest`: `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); grep -qF 'paired-versions.yaml' "$inst"; echo "pin=$?"; grep -qiE '(^\|[^a-z])latest([^a-z]\|$)' "$inst" && echo USES-LATEST \|\| echo NO-LATEST` | `pin=0` then `NO-LATEST` |
| 3 | It selects the `.exe` / windows tarball asset names from brief 01: `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); grep -qE -e 'statusgen-windows-amd64[.]exe' -e 'statusgen-windows-arm64[.]exe' "$inst" && grep -qE -e 'desk-tools-windows-amd64[.]tar[.]gz' -e 'desk-tools-windows-arm64[.]tar[.]gz' "$inst"; echo $?` | `0` |
| 4 | **Positive path** — a correctly-hashed binary installs (Go fork: `cd tools/desk && go test ./... -run 'TestWindowsInstall.*Verifies' -count=1`; PowerShell fork: a Pester/`-WhatIf` harness fixture with a matching-hash fixture) | exit 0; a `PASS`/installed line naming the version |
| 5 | **NEGATIVE PATH (the security row)** — a byte-flipped binary is REFUSED and NOT placed: `cd tools/desk && go test ./... -run 'TestWindowsInstall.*RefusesOnHashMismatch' -count=1` (or the PowerShell fork's tampered-fixture harness) | exit 0; test asserts the install FAILED with a mismatch message and the destination path does NOT exist |
| 5a | **Fail-first for row 5** — with the verify step disabled, the tampered binary is wrongly accepted: `git stash` (or check out a pre-fix commit / flip the mutation flag) then run the row-5 test; expect it to FAIL (the bad binary installs); `git stash pop` | the row-5 test RED on the unfixed code — pasted under `## Fail-first` in the PR body |
| 6 | The success line names version + verified hash (brief 04/05 assert on it): run the installer against a local fixture release and `grep -E 'installed .*v[0-9]+[.][0-9]+[.][0-9]+.*sha256'` its output | a line naming the pinned version and the verified sha256 |
| 7 | **Consumers routing corroborated by the diff** (run on the implementer's branch): `statusgen --root . --consumers windows-port/03; echo $?` | `0` — the installer artifact (fixed-here) is proved by the branch diff |

## Fail-first
<!-- appended at implementation time: the row-5 test run against the check-disabled installer,
     showing the tampered binary wrongly accepted (the red), and the commit/mutation it ran
     against. Row 5 asserts a GUARD (verify-or-refuse) so this is required, not optional. -->

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" requires a non-implementer. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|

## Review
Gate: **human** (from frontmatter). The human confirms the fork ruling was recorded before the
build, and that the sha256-verify-or-refuse control is the first post-download step and cannot be
bypassed — row 5 (negative path) plus row 5a (fail-first) together prove the refusal is
load-bearing, not decorative. A green happy-path row (4) with no negative-path row is exactly the
one-layer-verified failure this stream forbids on the security surface.
