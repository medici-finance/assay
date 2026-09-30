---
brief: assay:assay:windows-port:03
title: Windows install path — PowerShell-vs-Go-installer fork, then build
why: >-
  A Windows adopter needs the same thing a Unix adopter gets from `assay:install`: a
  version-pinned, sha256-verified statusgen/desk-tools build placed where the tools can find it,
  with a hash mismatch a hard refusal — not a warning. There is no `desk-install` equivalent on
  Windows today (the Unix path is `sudo make desk-install` into `/opt/desk-tools/bin`). This
  brief decides HOW that install is delivered — a PowerShell script vs a Go-native install
  subcommand — surfacing the fork for a maintainer ruling rather than pre-deciding it, then
  builds the chosen one with the verify-or-refuse control intact.
wave: 2
depends: ["windows-port/01", "windows-port/02"]
unblocks: ["windows-port/05", "windows-port/06", "windows-port/07"]
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
schema: brief-v2
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
version: 1
id: 0625c292-6a5c-421f-88f3-2266baeae926
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
| 8 | **PowerShell bootstrap hash-verify at Windows RUNTIME (decision #508)** — the dedicated `windows-bootstrap-smoke` job (an ONLINE leg, kept separate from the offline `windows-smoke` because the bootstrap downloads a release asset — the sanctioned decision-#508 exception) runs `scripts/bootstrap-windows.ps1` on `windows-latest` via the driver `scripts/windows-bootstrap-hashcheck-smoke.ps1`: a TAMPERED checksum REFUSES (throws on the sha256 mismatch, nothing installed), the pinned checksum INSTALLS, and a check-removed copy installs the tampered asset (non-vacuity). Requires the staged `ci/staged-workflows/windows-ci-leg.yml` PROMOTED into `.github/workflows/` first (maintainer — an App cannot push a workflow file). | a green `windows-bootstrap-smoke` run whose log shows `OK 1/3` / `OK 2/3` / `OK 3/3` then `windows-bootstrap-hashcheck-smoke: PASS`; record the run URL |

## Fail-first
<!-- appended at implementation time: the row-5 test run against the check-disabled installer,
     showing the tampered binary wrongly accepted (the red), and the commit/mutation it ran
     against. Row 5 asserts a GUARD (verify-or-refuse) so this is required, not optional. -->

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" requires a non-implementer. -->

### Non-implementer verifier run — VERIFY: PASS on the mechanical rows; HELD at `implemented` (gate: human) — 2026-09-07 assay-verifier (verify-desk dispatch), merged main `9b1cf06`

Runner ≠ implementer (fresh dispatched verifier, offline `KUBECONFIG=/dev/null`). Ruled fork = Option 2 (Go-native `deskinstall` + a ~5-line PowerShell bootstrap), delivered by commit `11bf3e2`: `tools/desk/cmd/deskinstall/{main.go,install.go,install_test.go}` + `scripts/bootstrap-windows.ps1`. Rows 1–6 + fail-first 5a run offline (the Go tests take a `--platform windows-amd64` override, so no Windows runtime is needed).

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|
| 1 | `ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go \| head -1` | 0 | `tools/desk/cmd/deskinstall/main.go` (Go fork present) | 2026-09-07 | assay-verifier |
| 2 | grep paired-versions pin / never-latest | 0 | `pin=0`, `NO-LATEST` | 2026-09-07 | assay-verifier |
| 3 | grep windows `.exe` + `.tar.gz` asset names | 0 | `row3=0` (both amd64/arm64 asset names present) | 2026-09-07 | assay-verifier |
| 4 | `go test ./cmd/deskinstall/ -run 'TestWindowsInstall.*Verifies' -count=1` | 0 | `--- PASS: TestWindowsInstallVerifiesCorrectHash` | 2026-09-07 | assay-verifier |
| 5 | `go test ./cmd/deskinstall/ -run 'TestWindowsInstall.*RefusesOnHashMismatch' -count=1` | 0 | `--- PASS` — tampered binary REFUSED; neither statusgen.exe nor deskboard.exe placed | 2026-09-07 | assay-verifier |
| 5a | fail-first: stub `verifySHA256` → `return nil`, rerun row-5 test | 1 | `--- FAIL … SECURITY: tampered binary was accepted` (read-only isolated copy); baseline PASSes → guard is load-bearing | 2026-09-07 | assay-verifier |
| 6 | run installer vs local fixture; grep `installed .*v[0-9]+.*sha256` | 0 | `installed statusgen v0.26.0 sha256:67e36fb6…` + `installed desk-tools v0.26.0 sha256:a426ae50…` (offline fixture manifest) | 2026-09-07 | assay-verifier |
| 7 | `statusgen --root . --consumers windows-port/03` | 2 | **could-not-check** — local statusgen v0.27.0 vs brief-pinned v0.26.0 (version-mismatched oracle); aborts on the unrelated local-only FP `decisions/README.md: no frontmatter` before evaluating wp/03. Corroborated by diff: fixed-here `tools/desk/cmd/deskinstall/main.go` created by commit `11bf3e2` | 2026-09-07 | assay-verifier |

**RISK-VALUE: DERIVED** (security/trust-boundary — executable-byte acquisition; the SPOF the brief names):
- sha256-verify-or-refuse control: `reSHA256 = ^[0-9a-f]{64}$` @ `tools/desk/cmd/deskinstall/install.go:61`, exact-inequality refusal `gotHex != wantHex` @ `install.go:109` (`crypto/sha256.Sum256` @ `install.go:106`) — 64-hex = 32-byte digest, the exact form in `plugins/assay/paired-versions.yaml`; any single-byte tamper refused (row 5), `wantHex` public so no timing-safety needed.
- pin source `manifestName = "paired-versions.yaml"` @ `main.go:42` — the expected hash comes from the plugin-shipped, version-committed pin file (`tag: v0.26.0`): the independent second layer (check and pin fail for different reasons).
- never-latest guard `reTag = ^v[0-9]+\.[0-9]+\.[0-9]+` @ `install.go:62` + `tag != m.Tag` @ `install.go:85` — forces a pinned semver, rejects floating refs (row 2 `NO-LATEST`).
Reversible knobs (out of scope): file mode `0o755`, HTTP timeout `120s`.

**VERIFY: PASS** — on the mechanical rows (1–6 + fail-first 5a); row 7 could-not-check (stale oracle). This is `gate: human` — a model cannot sign it off; status stays `implemented`, routed to the human gate. Human-gate confirmation points: (1) the PowerShell-vs-Go fork ruling was recorded before the build (impl matches the recommended Option 2 + bootstrap); (2) the PowerShell bootstrap's own hash-verify (`scripts/bootstrap-windows.ps1:32-33`) is untested PowerShell, exercisable only on a Windows runtime — its negative path was could-not-check here. Read-only, offline; no flip.
**Verify-table RUN 2026-09-10 — opus-4.8[1m]-verifier (non-implementer, verify-desk). NOT a sign-off.** Merged main `8d799c6bc026f675817bc3cfbfa81cad11efe052`, offline (`KUBECONFIG=/dev/null`, go1.26.5, Darwin/arm64). Frontmatter: `gate: human`, `risk {all no}`. Windows install path — the ruled fork is Option 2 (Go-native `deskinstall` + a thin PowerShell bootstrap).

| # | command | exit | observed | discharges |
|---|---------|------|----------|------------|
| 1 | `ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go \| head -1` | 0 | `tools/desk/cmd/deskinstall/main.go` — ruled Go fork present (no monolithic .ps1, correct for Option 2) | Row: artifact exists |
| 2 | grep paired-versions pin / never-latest | 0 | pins from `paired-versions.yaml`; no `latest` | Row: pins from manifest |
| 3 | grep windows `.exe` + `.tar.gz` asset names | 0 | both amd64+arm64 asset names selected | Row: brief-01 asset selection |
| 4 | `go test ./cmd/deskinstall/ -run 'TestWindowsInstall.*Verifies' -count=1` | 0 | PASS TestWindowsInstallVerifiesCorrectHash | Row: positive path installs |
| 5 | `go test ./cmd/deskinstall/ -run 'TestWindowsInstall.*RefusesOnHashMismatch' -count=1` | 0 | PASS — tampered binary REFUSED; neither statusgen.exe nor deskboard.exe placed | Row: negative/security path |
| 5a | fail-first: stub `verifySHA256`→nil in a scratch copy, rerun row 5 | 1 (red as required) | `SECURITY: tampered binary was accepted — install must REFUSE`; unmutated copy PASSes | Row: guard is load-bearing |
| 6 | positive test success-line capture | 0 | success line names version + verified sha256 for both statusgen and desk-tools | Row: success line names version+hash |
| 7 | `statusgen --root . --consumers windows-port/03` | 0 | `no brief files in the diff … nothing to corroborate` — VACUOUS on merged head (branch already merged, empty diff); consumer artifact `tools/desk/cmd/deskinstall/` present in tree (rows 1-6), corroborated by presence not diff | Row: consumer routing (by-presence post-merge) |
| 8 | `windows-bootstrap-smoke` on windows-latest (`scripts/bootstrap-windows.ps1` via `scripts/windows-bootstrap-hashcheck-smoke.ps1`; tampered→REFUSE, pinned→INSTALL) | — | COULD-NOT-CHECK — ONLINE + Windows-runtime row; an offline macOS verifier cannot run GitHub Actions on windows-latest and cannot fetch a run URL. Workflow `.github/workflows/windows-ci-leg.yml` + driver script ARE present; the green run is unverified here | Row 8 (could-not-check, online/Windows) |

Supporting (offline): `go build ./cmd/deskinstall/` 0; `go vet` 0; full `go test ./cmd/deskinstall/ -count=1` 0 (incl TestWindowsInstallRefusesAbsentPin). Bootstrap mirror-control confirmed by INSPECTION (`scripts/bootstrap-windows.ps1`: ValidatePattern 64-hex; `throw REFUSED: sha256 mismatch` on inequality) — but its RUNTIME (PowerShell/Windows) is could-not-check offline.

`RISK-VALUE: DERIVED — verify-or-refuse: gotHex != wantHex @ tools/desk/cmd/deskinstall/install.go:109` (over sha256.Sum256 @ :107) — exact-equality on a 256-bit digest; any single-byte tamper changes it (row 5 REFUSE); runs FIRST post-download, before any placement (two-phase: verify ALL before placing ANY). The ONE control between a substituted asset and unverified bytes.
`RISK-VALUE: DERIVED — reTag = ^v[0-9]+\.[0-9]+\.[0-9]+ @ install.go:62` (+ tag!=m.Tag @ :85) — forces a pinned semver, rejects floating refs; the mechanism behind row 2's no-latest. Second independent layer: expected hash read from version-committed `paired-versions.yaml`, so check and pin fail for different reasons.
`RISK-VALUE: NAMED, NOT DERIVED — the windows asset sha256 trust anchors @ plugins/assay/paired-versions.yaml (statusgen/desk-tools windows amd64+arm64, tag v1.0.3)` — deriving = recompute sha256 over the released windows assets, which needs downloading them (ONLINE) → could-not-check offline. Set by the umbrella re-pin at the merged head, not by the installer diff. Recorded here verbatim as the online-only derivation the human/CI confirms.

**VERDICT: PASS on the mechanical/offline rows (1-6 + fail-first 5a + build/vet/test). Rows 8 could-not-check (online Windows CI leg); row 7 vacuous post-merge (by-presence). gate:human — a model does NOT sign off.** Evidence gathered; status stays at implemented; the flip is the human's.

**Findings (for the human gate):** (1) Implementation matches the recommended Option 2 fork; the hash verify-or-refuse is genuinely load-bearing (fail-first 5a reds when disabled) and two-phase (verify all components before placing any). (2) Confirm before closing: the fork ruling was recorded before the build; and the `windows-bootstrap-smoke` leg is green (`OK 1/3..3/3` + `PASS`) — that green run is the only proof of the PowerShell bootstrap's runtime hash-verify, which no offline verifier can discharge. (3) Consistent with the prior run (2026-09-07 @ 9b1cf06); re-verified at newer head 8d799c6b, same conclusions.
### Verify pass 2026-09-22 (non-implementer, VERIFY: PASS on offline rows — gate:human, verified pending human gate; row 8 online could-not-check)

Runner: `claude-opus-4-8[1m]` (non-implementer). Merged main `6204bb4f1eacc0229f2a86c8e0dce59edabdd22a`. Offline (`KUBECONFIG=/dev/null`). gate: human, risk all-no. Ruled fork = Option 2 (Go-native `deskinstall` + thin PowerShell bootstrap).

| # | Command | Expect | Observed (exit + key line) | Date | Runner |
|---|---------|--------|----------------------------|------|--------|
| 1 | `ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go` | Go fork present | 0 → `tools/desk/cmd/deskinstall/main.go` present, no monolithic .ps1 (correct for Option 2) | 2026-09-22 | opus-4.8-verifier |
| 2 | grep paired-versions pin / never-latest | pinned, no latest | 0 → `pin=0` then `NO-LATEST` | 2026-09-22 | opus-4.8-verifier |
| 3 | grep windows `.exe` + `.tar.gz` asset names | both arch selected | 0 → both amd64/arm64 selected | 2026-09-22 | opus-4.8-verifier |
| 4 | `go test ./cmd/deskinstall/ -run 'TestWindowsInstall.*Verifies' -v` | positive path installs | 0 → `PASS: TestWindowsInstallVerifiesCorrectHash` (success line + ledger path asserted) | 2026-09-22 | opus-4.8-verifier |
| 5 | `go test ...RefusesOnHashMismatch -v` | tampered binary refused, nothing placed | 0 → `PASS: TestWindowsInstallRefusesOnHashMismatch` — neither statusgen.exe nor deskboard.exe placed | 2026-09-22 | opus-4.8-verifier |
| 5a | fail-first: disable `gotHex != wantHex` in worktree copy, rerun row 5, revert | RED when disabled | exit 1 → `install_test.go:169: SECURITY: tampered binary was accepted — install must REFUSE`; unmutated PASSes → guard load-bearing; reverted, tree clean | 2026-09-22 | opus-4.8-verifier |
| 6 | success-line assertion + `go build`/`go vet`/full pkg test | asserted, clean | 0 → success line shape asserted (row 4); build 0, vet 0, full `go test ./cmd/deskinstall/` 0 (incl RefusesAbsentPin) | 2026-09-22 | opus-4.8-verifier |
| 7 | `statusgen --root . --consumers windows-port/03` | exit 0 | 0 → "nothing to corroborate" — VACUOUS post-merge (empty diff, merged brief); consumer `tools/desk/cmd/deskinstall/` present (rows 1-6), corroborated by presence | 2026-09-22 | opus-4.8-verifier |
| 8 | `windows-bootstrap-smoke` on windows-latest | green run, `PASS` | **could-not-check** — ONLINE + Windows-runtime row; offline macOS verifier cannot run GitHub Actions on windows-latest (envelope C3). Workflow + both PS scripts present; runtime hash-verify confirmed by INSPECTION (`if ($got -ne $Sha256){ throw "REFUSED: sha256 mismatch" }` @ bootstrap-windows.ps1:107); RUNTIME could-not-check offline. Wake: a green `windows-bootstrap-smoke` run showing `PASS` with run URL. | 2026-09-22 | opus-4.8-verifier |

Scope traceability: every Evidence row maps 1:1 to its Verify row.

RISK-VALUE: DERIVED — verify-or-refuse `gotHex != wantHex` @ `tools/desk/cmd/deskinstall/install.go:110` (over `sha256.Sum256` @ :108) — exact equality on a 256-bit digest; any single-byte tamper changes it (row 5 REFUSES); runs FIRST post-download, before ANY placement (two-phase: verify all, then place). The ONE control (the brief's named SPOF) between a substituted asset and unverified bytes on disk; proven load-bearing by row 5a.
RISK-VALUE: DERIVED — `reTag = ^v[0-9]+\.[0-9]+\.[0-9]+` @ `install.go:63` + `tag != m.Tag` @ :86 — forces a pinned semver, rejects floating refs (row 2). Second independent layer: expected hash read from the version-committed paired-versions.yaml, so the check and the pin fail for different reasons (tampered download vs tampered pin file) — the brief's independence test.
RISK-VALUE: NAMED, NOT DERIVED — windows asset sha256 trust anchors @ `plugins/assay/paired-versions.yaml` (tag v1.0.6) — deriving = recompute sha256 over the RELEASED windows assets, which requires downloading them (ONLINE) → could-not-check under the offline envelope. Set by the umbrella re-pin at the merged head, not by the installer diff; the online-only derivation CI/the human confirms.

**VERIFY: PASS on offline rows 1-6 + 5a** (mechanical/security path fully green; the hash-verify SPOF is load-bearing). Row 7 exit-0 but vacuous post-merge. Row 8 could-not-check (environment: online GitHub-Actions leg on windows-latest + PowerShell runtime — dischargeable only by CI/a Windows runtime). gate: human → a model records Evidence and does NOT sign off `done`. Advances implemented → verified (first stamp); routed to the human verify-gate, who confirms the row-8 online `windows-bootstrap-smoke` run green before the `done` close.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ dd1582b7a5f3 (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer re-verify on merged main dd1582b7a5f3ee59a550b7b2d73dc187cef451e5. gate: human — Evidence only; the brief stays implemented for the driver's sign-off card. First table: the `statusgen verifyrun` execution witness, landed verbatim. It ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, a full clone pinned to this SHA, statusgen built from the clone's own source). Second table: the hand run on the host (darwin/arm64, go1.27.1).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); test -n "$inst" && echo "$inst"` | pass exit=0 | sha256:4133714c0f01 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `paired-versions.yaml` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:3655ee601176 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `.exe` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:51d2666b0365 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./... -run 'TestWindowsInstall.*Verifies' -count=1` | pass exit=0 | sha256:702a3422c58a | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./... -run 'TestWindowsInstall.*RefusesOnHashMismatch' -count=1` | pass exit=0 | sha256:5a6be4046ff6 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 5a | `git stash` | pass exit=0 | sha256:17f9ea949649 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 6 | `grep -E 'installed .*v[0-9]+[.][0-9]+[.][0-9]+.*sha256'` | fail exit=1 | sha256:e3b0c44298fc | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --consumers windows-port/03; echo $?` | pass exit=0 | sha256:552b46ff01f0 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 8 | `windows-bootstrap-smoke` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:9d245ef98157 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); test -n "$inst" && echo "$inst" | a path printed, exit 0 | checked-clean: exit 0, prints tools/desk/cmd/deskinstall/main.go (Go fork, as ruled on #508); same in the Linux witness (pass exit=0) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 2 | brief row 2 verbatim (grep -qF paired-versions.yaml; grep -qiE latest-word) | pin=0 then NO-LATEST | checked-clean: pin=0, NO-LATEST on host (system grep, one file, positive canary hit) and in the Linux container (GNU grep 3.8, network none) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 3 | brief row 3 verbatim (grep -qE windows .exe names && grep -qE windows .tar.gz names) | 0 | checked-clean: 0 on host and in the Linux container. Matches are at main.go:59-60 (usage text); the actual asset names are resolved from the manifest pin lines at install.go:74-82 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 4 | cd tools/desk && go test ./... -run 'TestWindowsInstall.*Verifies' -count=1 | exit 0; PASS naming the version | checked-clean: exit 0; with -v on the package: "--- PASS: TestWindowsInstallVerifiesCorrectHash" (answers the lint gotest-run-vacuous NOTICE: the test ran, it was not a no-match). Linux witness pass exit=0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 5 | cd tools/desk && go test ./... -run 'TestWindowsInstall.*RefusesOnHashMismatch' -count=1 | exit 0; install FAILED with mismatch, dest path absent | checked-clean: exit 0; -v: "--- PASS: TestWindowsInstallRefusesOnHashMismatch" (test asserts err names "mismatch" and neither statusgen-windows-amd64.exe nor deskboard.exe exists). Linux witness pass exit=0. go vet + full package test also exit 0 | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 5a | fail-first: in a scratch git-archive copy of dd1582b7, perl-edit install.go:110 to "if false && gotHex != wantHex", rerun the row-5 test | row-5 test RED on the check-disabled code | checked by a hand procedure, not the row's literal text (the witness ran only `git stash`): RED as required, exit 1, "install_test.go:169: SECURITY: tampered binary was accepted — install must REFUSE on hash mismatch"; unmutated sibling copy PASSes. The guard is load-bearing | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 6 | run Install against the package's local fixture release (scratch-only harness test calling fixture()+Install with Out=stdout, tag v1.0.29), then grep -E 'installed .*v[0-9]+[.][0-9]+[.][0-9]+.*sha256' | a line naming the pinned version and verified sha256 | checked by a hand procedure, not the row's literal text (the witness ran the bare grep with no input: fail exit=1): grep rc 0; "installed statusgen v1.0.29 sha256:67e36fb63dda0c93c2820c83d70d9bc5cfc67b26eb12f21d31f6d5ee8ba3480f" and "installed desk-tools v1.0.29 sha256:1be8ca2ce32dd253fd0180e52ea448b0a16c9462e62157a6ee962e4ee70bc64c" (fixture digests), then the ledger line | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 7 | statusgen --root . --consumers windows-port/03; echo $? | 0 | checked-clean but vacuous: statusgen built from main's own source (--version dev), exit 0, "consumers: no brief files in the diff against dd1582b7… — nothing to corroborate". Post-merge there is no branch diff; the fixed-here artifact is corroborated by presence (rows 1-6) | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 8 | windows-bootstrap-smoke job (windows-latest; scripts/windows-bootstrap-hashcheck-smoke.ps1 driving scripts/bootstrap-windows.ps1) | a green run whose log shows the OK markers then "windows-bootstrap-hashcheck-smoke: PASS"; record the run URL | could-not-check at the current head: the only run record predates 0e2f4d161 (see Notes). Historical record: Newest run: https://github.com/medici-finance/assay/actions/runs/35143331505 (tag v1.0.11, head 14e51a8146c3, 2026-09-16), job windows-bootstrap-smoke = success. Log: "OK 1/6" … "OK 6/6" then "windows-bootstrap-hashcheck-smoke: PASS". The driver now has 6 assertions, not 3: 2/6 pinned installs, 3/6 tampered digest REFUSED "nothing installed", 5/6 check-removed copy installs the tampered asset (non-vacuity), plus 1/6 disputed -Sha256 and 4/6 absent-platform refusals and their 6/6 non-vacuity. Re-running on current main was could-not-check: it needs a Windows runner and a workflow dispatch, and this verifier may not dispatch | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |

RISK-VALUE: DERIVED — gotHex != wantHex @ tools/desk/cmd/deskinstall/install.go:110 — exact equality of lowercase hex of SHA-256 over the whole downloaded body against the pinned digest. hex.EncodeToString emits lowercase and E2 forces the pin to lowercase, so there is no false mismatch on case. Passing it with substituted bytes requires a SHA-256 second preimage. It runs in phase 1, before the first write (MkdirAll @ :174), so a mismatch leaves nothing placed (row 5). Removing it lets the tampered binary through (row 5a RED). This is the brief's named single point of failure.
RISK-VALUE: DERIVED — Windows trust anchors @ plugins/assay/paired-versions.yaml:42,43,67,68 (v1.0.29: statusgen-windows-amd64.exe 6b8d7eda…da29, statusgen-windows-arm64.exe 5059ccd5…4780, desk-tools-windows-amd64.tar.gz d2b4344a…7032, desk-tools-windows-arm64.tar.gz 0fbb57ac…41bd) — I downloaded the four published v1.0.29 assets read-only (release is non-draft, non-prerelease, published 2026-09-28) and recomputed shasum -a 256 over them. All four equal the pinned values exactly and equal the release's own checksums.txt lines. Scope of the claim: this shows the pin matches the published bytes. It does not attest the release build's provenance, which is outside this brief.
RISK-VALUE: DERIVED — bootstrap $got -ne $Sha256 @ scripts/bootstrap-windows.ps1:108 (with ValidatePattern ^[0-9a-f]{64}$ @ :34 and the manifest-digest check @ :87) — Get-FileHash SHA256, lowercased, compared to the manifest-resolved or re-asserted digest. The temp file is removed and the script throws before any Move-Item into $Dest (:111-112). PowerShell -ne and -match are case-insensitive, but hex case does not change a value, so that is harmless. Runtime proof is the recorded row-8 run: OK 3/6 tamper REFUSED "nothing installed", OK 5/6 check removed so the tamper installs. That run predates the 2026-09-23 ASCII/no-ternary edit (Notes 2), which leaves line 108's logic unchanged.
RISK-VALUE: DERIVED — reSHA256 = ^[0-9a-f]{64}$ @ tools/desk/cmd/deskinstall/install.go:62 — 64 lowercase hex chars = 256 bits = the exact form hex.EncodeToString(sha256.Sum256(...)) produces. It is anchored at both ends, so a truncated, empty or uppercase pin is refused before any download (defence in depth ahead of E1).
RISK-VALUE: DERIVED — reTag = ^v[0-9]+\.[0-9]+\.[0-9]+ @ tools/desk/cmd/deskinstall/install.go:63 + tag != m.Tag @ :86 — forces a semver-shaped tag (never "latest"; row 2) that must equal the component's committed tag. The regex is left-anchored only, so a suffix such as -rc1 is accepted, but the equality check at :86 binds it to the committed value. The tag only selects the URL; integrity is carried by E1/E7. The pin file (reviewed, version-committed) and the download check fail for different reasons, which is the brief's second-layer independence test.
RISK-VALUE: DERIVED — assetURL "https://github.com/%s/releases/download/%s/%s" @ tools/desk/cmd/deskinstall/install.go:100 with release_home: medici-finance/assay @ plugins/assay/paired-versions.yaml:33,:57 (bootstrap hardcodes the same host/repo @ scripts/bootstrap-windows.ps1:104) — this is the repo whose release workflow builds and checksums these assets. A wrong host gives a download error or a digest mismatch, never unverified placement, so the binding is availability and not integrity.
RISK-VALUE: DERIVED — zip-slip guard path.Base + strings.Contains(base, "..") @ tools/desk/cmd/deskinstall/install.go:226-227 — every entry is flattened to its base name and joined under destDir, and non-regular entries are dropped (:223). Traversal out of destDir is therefore impossible, and the ".." test only rejects odd names. It applies only to bytes that already passed E1.

Notes:
- BLOCKED (check-definition), same shape as windows-port/01 on 2026-09-28. The Linux witness passes rows 1, 4, 5 and 7 only: rows 2, 3 and 8 could-not-run (exit 127), row 6 fails (exit 1, a bare grep with no input) and row 5a ran a bare `git stash`, because verifyrun runs only the first code span of each Command cell. Hand rows 5a and 6 ran procedures that are not the rows' literal text. Rows 1, 2, 3, 4, 5 and 7 pass by hand as written. Re-authoring rows 2, 3, 5a, 6 and 8 into runnable commands is tracked as #1890; no row is re-authored in this Evidence. Lint exit 0.
- Row 5a (fail-first): with the compare at install.go:110 mutated in a scratch copy, the security test exits 1 with "SECURITY: tampered binary was accepted".
- Row 8 stays could-not-check until a windows-bootstrap-smoke run at or after 0e2f4d161 exists. The only record is the run of 2026-09-16 (OK 1/6..6/6, PASS); it predates 0e2f4d161 (2026-09-23, PowerShell 5.1 parse fixes; hash-check logic unchanged), and the workflow no longer fires on automation-pushed tags. Proof at the current head needs a maintainer dispatch of the windows CI leg on a Windows runner. The row's Expect (OK 1/3..3/3) is stale; the driver now prints six checks, a superset.
- Decision #508 (Go-native installer plus a minimal PowerShell bootstrap) was ratified in the maintainer's own identity; its required Windows-runtime tamper check is #595, closed as completed.
- #1693 does not fail any row: no row asserts a clean host can obtain deskinstall end to end, and #1693 sets its done-when on windows-port/09's doc. For the driver: the brief's wording says the bootstrap solves the first-install problem, but the first binary chosen is statusgen, so that is not closed end to end.
- Brief tidy-ups for the driver: frontmatter `issues: []` though #508 is the decision issue; lint NOTICEs for an owed dereference row and an owed end-to-end flow row.

VERIFY: BLOCKED

## Review
Gate: **human** (from frontmatter). The human confirms the fork ruling was recorded before the
build, and that the sha256-verify-or-refuse control is the first post-download step and cannot be
bypassed — row 5 (negative path) plus row 5a (fail-first) together prove the refusal is
load-bearing, not decorative. A green happy-path row (4) with no negative-path row is exactly the
one-layer-verified failure this stream forbids on the security surface.
