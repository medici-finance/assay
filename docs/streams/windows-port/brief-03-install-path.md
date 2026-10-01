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
version: 2
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
| 2 | `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); grep -qF 'paired-versions.yaml' "$inst"; p=$?; grep -qiE -e '^latest$' -e '^latest[^a-z]' -e '[^a-z]latest$' -e '[^a-z]latest[^a-z]' "$inst" && l=USES-LATEST \|\| l=NO-LATEST; echo "pin=$p $l"` — it resolves the pin from `paired-versions.yaml`, never `latest` | output is `pin=0 NO-LATEST` — the pin is read from the manifest (`pin=0`) and no floating `latest` ref appears (`NO-LATEST`) |
| 3 | `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); grep -qE -e 'statusgen-windows-amd64[.]exe' -e 'statusgen-windows-arm64[.]exe' "$inst" && grep -qE -e 'desk-tools-windows-amd64[.]tar[.]gz' -e 'desk-tools-windows-arm64[.]tar[.]gz' "$inst"; echo $?` — it selects the `.exe` / windows tarball asset names from brief 01 | output is `0` — both the statusgen `.exe` and the desk-tools windows tarball asset names are present |
| 4 | **Positive path** — a correctly-hashed binary installs (Go fork: `cd tools/desk && go test ./... -run 'TestWindowsInstall.*Verifies' -count=1`; PowerShell fork: a Pester/`-WhatIf` harness fixture with a matching-hash fixture) | exit 0; a `PASS`/installed line naming the version |
| 5 | **NEGATIVE PATH (the security row)** — a byte-flipped binary is REFUSED and NOT placed: `cd tools/desk && go test ./... -run 'TestWindowsInstall.*RefusesOnHashMismatch' -count=1` (or the PowerShell fork's tampered-fixture harness) | exit 0; test asserts the install FAILED with a mismatch message and the destination path does NOT exist |
| 5a | `d=$(mktemp -d) && f=tools/desk/cmd/deskinstall/install.go && test "$(grep -c 'gotHex != wantHex' "$f")" = 1 && sed 's/gotHex != wantHex/false/' "$f" > "$d/install.go" && printf '{"Replace":{"%s":"%s"}}' "$PWD/$f" "$d/install.go" > "$d/o.json" && { (cd tools/desk && go test -overlay "$d/o.json" -run '^TestWindowsInstallRefusesOnHashMismatch$' -count=1 -v ./cmd/deskinstall/ > "$d/out" 2>&1); test $? -ne 0 && grep -qF -e '--- FAIL: TestWindowsInstallRefusesOnHashMismatch' "$d/out" && grep -qF 'SECURITY: tampered binary was accepted' "$d/out" && echo 'FAIL-FIRST: RED'; }` — **Fail-first for row 5**: with the verify step disabled (the sha256 comparison replaced by `false` in a `go test -overlay` copy of `install.go`; the working tree is not touched), the tampered binary is wrongly accepted and the row-5 test goes RED | exit 0, output is `FAIL-FIRST: RED` — the row-5 test FAILS with its `SECURITY: tampered binary was accepted` assertion once the check is disabled (the implementer's red was pasted under `## Fail-first` in the PR body) |
| 6 | `d=$(mktemp -d) && printf '%s\n' 'package main' 'import ("os"; "testing")' 'func TestRow6SuccessLine(t *testing.T) {' 'm, _, _, f := fixture(t, "v0.26.0")' 'if err := Install(Options{ManifestPath: m, DestDir: t.TempDir(), Platform: "windows-amd64", Fetch: f, Out: os.Stdout}); err != nil { t.Fatal(err) }' '}' > "$d/row6_test.go" && printf '{"Replace":{"%s":"%s"}}' "$PWD/tools/desk/cmd/deskinstall/zz_row6_test.go" "$d/row6_test.go" > "$d/o.json" && (cd tools/desk && go test -overlay "$d/o.json" -run '^TestRow6SuccessLine$' -count=1 -v ./cmd/deskinstall/ > "$d/out" 2>&1) && grep -qF -e '--- PASS: TestRow6SuccessLine' "$d/out" && grep -cE 'installed .*v[0-9]+[.][0-9]+[.][0-9]+.*sha256' "$d/out"` — the success line names version + verified hash (brief 04/05 assert on it): the installer runs against the package's local fixture release (a `go test -overlay` file writes its output to stdout; the working tree is not touched) and that output is grepped | ≥ `1` — a line naming the pinned version and the verified sha256 (`installed statusgen v0.26.0 sha256:` followed by the hex digest) |
| 7 | **Consumers routing corroborated by the diff** (run on the implementer's branch): `statusgen --root . --consumers windows-port/03; echo $?` | `0` — the installer artifact (fixed-here) is proved by the branch diff |
| 8 | `id=$(gh run list -R medici-finance/assay --workflow windows-ci-leg.yml --limit 1 --json databaseId --jq '.[0].databaseId'); test -n "$id" \|\| { echo 'could-not-check: no windows-ci-leg run readable'; exit 2; }; sha=$(gh run view "$id" -R medici-finance/assay --json headSha --jq .headSha); job=$(gh run view "$id" -R medici-finance/assay --json jobs --jq '.jobs[] \| select(.name == "windows-bootstrap-smoke") \| .conclusion'); n=$(gh run view "$id" -R medici-finance/assay --log \| grep -cF 'windows-bootstrap-hashcheck-smoke: PASS'); test "${n:-0}" -ge 1 && p=yes \|\| p=no; git merge-base --is-ancestor "$(git log -1 --format=%H -- scripts/bootstrap-windows.ps1 scripts/windows-bootstrap-hashcheck-smoke.ps1)" "$sha" 2>/dev/null && fr=yes \|\| fr=no; echo "run https://github.com/medici-finance/assay/actions/runs/$id head=$sha"; echo "bootstrap-smoke=$job pass=$p fresh=$fr"` — reads the latest `windows-ci-leg` run (the command only reads it; producing the run stays the maintainer dispatch described next). **PowerShell bootstrap hash-verify at Windows RUNTIME (decision #508)** — the dedicated `windows-bootstrap-smoke` job (an ONLINE leg, kept separate from the offline `windows-smoke` because the bootstrap downloads a release asset — the sanctioned decision-#508 exception) runs `scripts/bootstrap-windows.ps1` on `windows-latest` via the driver `scripts/windows-bootstrap-hashcheck-smoke.ps1`: a TAMPERED checksum REFUSES (throws on the sha256 mismatch, nothing installed), the pinned checksum INSTALLS, and a check-removed copy installs the tampered asset (non-vacuity). Requires the staged `ci/staged-workflows/windows-ci-leg.yml` PROMOTED into `.github/workflows/` first (maintainer — an App cannot push a workflow file). | output is `bootstrap-smoke=success pass=yes fresh=yes` — a green `windows-bootstrap-smoke` job whose log carries the driver's closing `windows-bootstrap-hashcheck-smoke: PASS` line (printed only after every one of its `OK n/N` assertions), on a run whose head contains the latest change to the two bootstrap scripts; record the printed run URL |

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
### Verification — 2026-09-30 (assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian)) — Verify table version 2

What moved since the last run (BLOCKED check-definition at dd1582b7a5f3, batch M #1886): Verify rows 2, 3, 5a, 6 and 8 were re-authored by #1904 so they run as written. Nothing else changed: between dd1582b7a5f3 and b7ca79ab798d the only file touched in scope is this brief. The installer, the bootstrap scripts and the pinned manifest are the same. All nine rows now execute as written in the host witness (last time rows 2, 3 and 8 exited 127, row 6 failed on a bare grep and row 5a ran `git stash`). Row 8 now resolves to a real run: that run is green, but it is stale.

Non-implementer re-verify on merged main b7ca79ab798de5f2faa5861818e72fdb38616386. Offline envelope (`KUBECONFIG=/dev/null`) for rows 1-7. Rows 8 and the trust-anchor derivation made read-only `gh` reads with an already-present credential. Host: darwin/arm64, go1.27.1, statusgen v1.0.29. The Linux in-container witness could not run: the docker daemon socket was absent. The hand table comes first, followed by the host `statusgen verifyrun --dry-run` witness exactly as it was emitted. gate: human, so this block records Evidence only and the brief stays at its current status.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); test -n "$inst" && echo "$inst"` | a path printed, exit 0 | Row 1: exit 0, prints tools/desk/cmd/deskinstall/main.go (the Go-native fork ruled on #508). Witness: pass exit=0 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); grep -qF 'paired-versions.yaml' "$inst"; p=$?; grep -qiE -e '^latest$' -e '^latest[^a-z]' -e '[^a-z]latest$' -e '[^a-z]latest[^a-z]' "$inst" && l=USES-LATEST \|\| l=NO-LATEST; echo "pin=$p $l"` | output `pin=0 NO-LATEST` | Row 2: exit 0, output "pin=0 NO-LATEST". My interactive grep is a shell function, so I ran the same two greps again with the system grep binary on the one file: same "pin=0 NO-LATEST". The pin literal is at main.go:42. Witness: pass exit=0 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 3 | `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); grep -qE -e 'statusgen-windows-amd64[.]exe' -e 'statusgen-windows-arm64[.]exe' "$inst" && grep -qE -e 'desk-tools-windows-amd64[.]tar[.]gz' -e 'desk-tools-windows-arm64[.]tar[.]gz' "$inst"; echo $?` | output `0` | Row 3: exit 0, output "0". The system grep binary gives the same result. The matches are the usage text at main.go:59-60. The installer builds the real asset names from the manifest pin lines, and those are at paired-versions.yaml:42-43 and :67-68. Witness: pass exit=0 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./... -run 'TestWindowsInstall.*Verifies' -count=1` | exit 0; PASS naming the version | Row 4, run as authored: exit 0, "ok github.com/medici-finance/assay/tools/desk/cmd/deskinstall", and no FAIL in any package. Run with -v (next row) to show it is not a no-match | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestWindowsInstall.*Verifies' -count=1 -v` | the named test RUNs and PASSes | Row 4 (-v form): exit 0. One test ran and passed: the Windows-install correct-hash test. It asserts both assets are placed, the success line "installed statusgen v0.26.0 sha256:" appears, and the ledger path is printed. There was no SKIP | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./... -run 'TestWindowsInstall.*RefusesOnHashMismatch' -count=1` | exit 0; install FAILED with mismatch, dest absent | Row 5, run as authored: exit 0, "ok github.com/medici-finance/assay/tools/desk/cmd/deskinstall". Witness: pass exit=0 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | `cd tools/desk && go test ./cmd/deskinstall/ -run 'TestWindowsInstall.*RefusesOnHashMismatch' -count=1 -v` | the named test RUNs and PASSes | Row 5 (-v form): exit 0. The hash-mismatch refusal test ran and passed, with no SKIP. The test feeds statusgen bytes with the first byte flipped. It asserts that the error names "mismatch" and that neither statusgen-windows-amd64.exe nor deskboard.exe exists in the destination (install_test.go:141-182) | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5a | `d=$(mktemp -d) && f=tools/desk/cmd/deskinstall/install.go && test "$(grep -c 'gotHex != wantHex' "$f")" = 1 && sed 's/gotHex != wantHex/false/' "$f" > "$d/install.go" && printf '{"Replace":{"%s":"%s"}}' "$PWD/$f" "$d/install.go" > "$d/o.json" && { (cd tools/desk && go test -overlay "$d/o.json" -run '^TestWindowsInstallRefusesOnHashMismatch$' -count=1 -v ./cmd/deskinstall/ > "$d/out" 2>&1); test $? -ne 0 && grep -qF -e '--- FAIL: TestWindowsInstallRefusesOnHashMismatch' "$d/out" && grep -qF 'SECURITY: tampered binary was accepted' "$d/out" && echo 'FAIL-FIRST: RED'; }` | exit 0, output `FAIL-FIRST: RED` | Row 5a: exit 0, output "FAIL-FIRST: RED". The captured test output reads "install_test.go:169: SECURITY: tampered binary was accepted — install must REFUSE on hash mismatch", followed by a FAIL for the package. The system grep counts exactly 1 occurrence of the compare, so the overlay mutated the only check. git status was clean afterwards. Witness: pass exit=0 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | `d=$(mktemp -d) && printf '%s\n' 'package main' 'import ("os"; "testing")' 'func TestRow6SuccessLine(t *testing.T) {' 'm, _, _, f := fixture(t, "v0.26.0")' 'if err := Install(Options{ManifestPath: m, DestDir: t.TempDir(), Platform: "windows-amd64", Fetch: f, Out: os.Stdout}); err != nil { t.Fatal(err) }' '}' > "$d/row6_test.go" && printf '{"Replace":{"%s":"%s"}}' "$PWD/tools/desk/cmd/deskinstall/zz_row6_test.go" "$d/row6_test.go" > "$d/o.json" && (cd tools/desk && go test -overlay "$d/o.json" -run '^TestRow6SuccessLine$' -count=1 -v ./cmd/deskinstall/ > "$d/out" 2>&1) && grep -qF -e '--- PASS: TestRow6SuccessLine' "$d/out" && grep -cE 'installed .*v[0-9]+[.][0-9]+[.][0-9]+.*sha256' "$d/out"` | at least `1` | Row 6: exit 0, output "2". The captured lines are "installed statusgen v0.26.0 sha256:67e36fb63dda0c93c2820c83d70d9bc5cfc67b26eb12f21d31f6d5ee8ba3480f" and "installed desk-tools v0.26.0 sha256:1be8ca2ce32dd253fd0180e52ea448b0a16c9462e62157a6ee962e4ee70bc64c" (fixture digests), then the ledger line. No overlay file was left in the tree. Witness: pass exit=0 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `statusgen --root "$PWD" --consumers windows-port/03; echo $?` | `0` | Row 7, run at the worktree root with an absolute root, because a bare `.` root is write-guarded: exit 0, printing "consumers: no brief files in the diff against b7ca79ab798de5f2faa5861818e72fdb38616386 — nothing to corroborate". It passes the literal Expect, but the check is empty: on merged main there is no branch diff | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `git checkout --detach 11bf3e2a40ed1905c8be12d1fcbc7475787f08d5 && statusgen --root "$PWD" --consumers --base 636b98ba731893de68c7f697f6b432952e142074 --brief windows-port/03; echo $?; git checkout --detach b7ca79ab798de5f2faa5861818e72fdb38616386` | the row's own intent ("run on the implementer's branch"): 0 | Row 7, branch form, at the head of the implementer branch of PR #513 (merge c4fc908d0921): exit 2, "COULD-NOT-CHECK: windows-port/03 is not in the diff against 3486abfb9f1f…". The one implementer commit never touched this brief file, so the instrument has no claims to corroborate on any diff. The worktree was restored to b7ca79ab798d | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 7 | `git show --stat --format=%s 11bf3e2a40ed1905c8be12d1fcbc7475787f08d5` | the fixed-here installer artifact is created by the implementer diff | Row 7, checked by hand with git: exit 0. The implementer commit creates tools/desk/cmd/deskinstall/main.go (+127), install.go (+249), install_test.go (+194) and scripts/bootstrap-windows.ps1 (+36). That corroborates the consumers claim "the chosen installer artifact … fixed-here" from the diff itself | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | `id=$(gh run list -R medici-finance/assay --workflow windows-ci-leg.yml --limit 1 --json databaseId --jq '.[0].databaseId'); test -n "$id" \|\| { echo 'could-not-check: no windows-ci-leg run readable'; exit 2; }; sha=$(gh run view "$id" -R medici-finance/assay --json headSha --jq .headSha); job=$(gh run view "$id" -R medici-finance/assay --json jobs --jq '.jobs[] \| select(.name == "windows-bootstrap-smoke") \| .conclusion'); n=$(gh run view "$id" -R medici-finance/assay --log \| grep -cF 'windows-bootstrap-hashcheck-smoke: PASS'); test "${n:-0}" -ge 1 && p=yes \|\| p=no; git merge-base --is-ancestor "$(git log -1 --format=%H -- scripts/bootstrap-windows.ps1 scripts/windows-bootstrap-hashcheck-smoke.ps1)" "$sha" 2>/dev/null && fr=yes \|\| fr=no; echo "run https://github.com/medici-finance/assay/actions/runs/$id head=$sha"; echo "bootstrap-smoke=$job pass=$p fresh=$fr"` | `bootstrap-smoke=success pass=yes fresh=yes`; record run URL | Row 8: exit 0. Output: "run https://github.com/medici-finance/assay/actions/runs/35143331505 head=14e51a8146c300da9286f96dd7a122cc55e79a0b" and then "bootstrap-smoke=success pass=yes fresh=no". The Expect is NOT met on the freshness term. The newest windows-ci-leg run is from 2026-09-16. The last change to the two bootstrap scripts is 0e2f4d161 (2026-09-23, #1570: ASCII plus removal of the PS7 ternary so the scripts parse on PowerShell 5.1), and that change is not an ancestor of the run head. Diffing the run head against main shows the refusal compare line changed only in its message text. So no Windows-runtime proof exists for the bootstrap as it stands on main. The five newest windows-ci-leg runs are all from 2026-09-16. Witness: fail exit=0 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Host witness (`statusgen verifyrun --brief <brief> --dry-run --root <worktree>`, exit 1). The table below is exactly as emitted:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); test -n "$inst" && echo "$inst"` | pass exit=0 | sha256:4133714c0f01 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (on-behalf-of human:ian) (forge-identity) |
| 2 | `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); grep -qF 'paired-versions.yaml' "$inst"; p=$?; grep -qiE -e '^latest$' -e '^latest[^a-z]' -e '[^a-z]latest$' -e '[^a-z]latest[^a-z]' "$inst" && l=USES-LATEST \|\| l=NO-LATEST; echo "pin=$p $l"` | pass exit=0 | sha256:282e91e5531d | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (on-behalf-of human:ian) (forge-identity) |
| 3 | `inst=$(ls scripts/install-windows.ps1 tools/desk/cmd/deskinstall/main.go 2>/dev/null \| head -1); grep -qE -e 'statusgen-windows-amd64[.]exe' -e 'statusgen-windows-arm64[.]exe' "$inst" && grep -qE -e 'desk-tools-windows-amd64[.]tar[.]gz' -e 'desk-tools-windows-arm64[.]tar[.]gz' "$inst"; echo $?` | pass exit=0 | sha256:9a271f2a916b | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go test ./... -run 'TestWindowsInstall.*Verifies' -count=1` | pass exit=0 | sha256:0de1309eb3cd | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./... -run 'TestWindowsInstall.*RefusesOnHashMismatch' -count=1` | pass exit=0 | sha256:f9f66fdfe3cc | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (on-behalf-of human:ian) (forge-identity) |
| 5a | `d=$(mktemp -d) && f=tools/desk/cmd/deskinstall/install.go && test "$(grep -c 'gotHex != wantHex' "$f")" = 1 && sed 's/gotHex != wantHex/false/' "$f" > "$d/install.go" && printf '{"Replace":{"%s":"%s"}}' "$PWD/$f" "$d/install.go" > "$d/o.json" && { (cd tools/desk && go test -overlay "$d/o.json" -run '^TestWindowsInstallRefusesOnHashMismatch$' -count=1 -v ./cmd/deskinstall/ > "$d/out" 2>&1); test $? -ne 0 && grep -qF -e '--- FAIL: TestWindowsInstallRefusesOnHashMismatch' "$d/out" && grep -qF 'SECURITY: tampered binary was accepted' "$d/out" && echo 'FAIL-FIRST: RED'; }` | pass exit=0 | sha256:ebbc5c15e7cd | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (on-behalf-of human:ian) (forge-identity) |
| 6 | `d=$(mktemp -d) && printf '%s\n' 'package main' 'import ("os"; "testing")' 'func TestRow6SuccessLine(t *testing.T) {' 'm, _, _, f := fixture(t, "v0.26.0")' 'if err := Install(Options{ManifestPath: m, DestDir: t.TempDir(), Platform: "windows-amd64", Fetch: f, Out: os.Stdout}); err != nil { t.Fatal(err) }' '}' > "$d/row6_test.go" && printf '{"Replace":{"%s":"%s"}}' "$PWD/tools/desk/cmd/deskinstall/zz_row6_test.go" "$d/row6_test.go" > "$d/o.json" && (cd tools/desk && go test -overlay "$d/o.json" -run '^TestRow6SuccessLine$' -count=1 -v ./cmd/deskinstall/ > "$d/out" 2>&1) && grep -qF -e '--- PASS: TestRow6SuccessLine' "$d/out" && grep -cE 'installed .*v[0-9]+[.][0-9]+[.][0-9]+.*sha256' "$d/out"` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (on-behalf-of human:ian) (forge-identity) |
| 7 | `statusgen --root . --consumers windows-port/03; echo $?` | pass exit=0 | sha256:314170bcfb32 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (on-behalf-of human:ian) (forge-identity) |
| 8 | `id=$(gh run list -R medici-finance/assay --workflow windows-ci-leg.yml --limit 1 --json databaseId --jq '.[0].databaseId'); test -n "$id" \|\| { echo 'could-not-check: no windows-ci-leg run readable'; exit 2; }; sha=$(gh run view "$id" -R medici-finance/assay --json headSha --jq .headSha); job=$(gh run view "$id" -R medici-finance/assay --json jobs --jq '.jobs[] \| select(.name == "windows-bootstrap-smoke") \| .conclusion'); n=$(gh run view "$id" -R medici-finance/assay --log \| grep -cF 'windows-bootstrap-hashcheck-smoke: PASS'); test "${n:-0}" -ge 1 && p=yes \|\| p=no; git merge-base --is-ancestor "$(git log -1 --format=%H -- scripts/bootstrap-windows.ps1 scripts/windows-bootstrap-hashcheck-smoke.ps1)" "$sha" 2>/dev/null && fr=yes \|\| fr=no; echo "run https://github.com/medici-finance/assay/actions/runs/$id head=$sha"; echo "bootstrap-smoke=$job pass=$p fresh=$fr"` | fail exit=0 | sha256:20d966818832 | 2026-09-30 | assay-verifier-app[bot] @ b7ca79ab798d (on-behalf-of human:ian) (forge-identity) |

The witness's own verdict lines read: row 7 "pass (exit=0) — expect: exit-status only", and row 8 "fail (exit=0) — no output line equals "bootstrap-smoke=success pass=yes fresh=yes"".

Risk-bearing value enumeration. It covers the literals in the implementer diff 11bf3e2a40ed (deskinstall main.go, install.go, bootstrap-windows.ps1), as they stand at b7ca79ab798d, plus the Deliverables' pinned manifest:
- E1 `gotHex != wantHex` @ tools/desk/cmd/deskinstall/install.go:110, over `sha256.Sum256` @ :108. This is the verify-or-refuse compare and ranks first: if it is wrong, unverified bytes are placed and executed on an adopter's machine, and a redeploy does not undo that.
- E2 `reSHA256 = ^[0-9a-f]{64}$` @ install.go:62. This is the pin-shape gate.
- E3 `reTag = ^v[0-9]+\.[0-9]+\.[0-9]+` @ install.go:63, together with `tag != m.Tag` @ :86. This is the never-latest guard.
- E4 `manifestName = "paired-versions.yaml"` @ main.go:42. This is the pin source.
- E5 Windows trust anchors @ plugins/assay/paired-versions.yaml:42,43,67,68 (tag v1.0.29 @ :35/:59).
- E6 `assetURL "https://github.com/%s/releases/download/%s/%s"` @ install.go:100, with `release_home: medici-finance/assay` @ paired-versions.yaml:33,:57. The bootstrap hardcodes the same host @ scripts/bootstrap-windows.ps1:104.
- E7 bootstrap `$got -ne $Sha256` @ scripts/bootstrap-windows.ps1:108, with `ValidatePattern('^[0-9a-f]{64}$')` @ :34.
- E8 zip-slip guard `path.Base` + `strings.Contains(base, "..")` @ install.go:226-227.
- Reversible knobs, which need no derivation: file mode `0o755` @ install.go:174,202,231 and the HTTP `Timeout: 120 * time.Second` @ install.go:248.

RISK-VALUE: DERIVED — gotHex != wantHex @ tools/desk/cmd/deskinstall/install.go:110. It requires exact equality between the lowercase hex SHA-256 of the whole downloaded body and the pinned digest. Getting substituted bytes past it would need a SHA-256 second preimage. It runs in phase 1, before the first write (MkdirAll @ :174), so a mismatch places nothing (row 5). With the compare disabled, the tampered binary is accepted (row 5a RED). This is the single point of failure the brief names.
RISK-VALUE: DERIVED — Windows trust anchors @ plugins/assay/paired-versions.yaml:42,43,67,68 (v1.0.29). I downloaded the four published v1.0.29 Windows assets read-only (release: non-draft, non-prerelease, published 2026-09-28T15:45:45Z) and recomputed `shasum -a 256` over each one. The results match the pinned digests and the release's checksums.txt lines exactly: statusgen-windows-amd64.exe 6b8d7eda38a9920b6bb21f67caebd9da1213d4cd9a5989212e870a35c3b0da29, statusgen-windows-arm64.exe 5059ccd54e2510586c4d4a87b7e4815315e0cfd5cd26c3ef389146d3d69b4780, desk-tools-windows-amd64.tar.gz d2b4344a8805de2efb56b10a241161421c3154f3650f48d9e85a23352cd67032, desk-tools-windows-arm64.tar.gz 0fbb57ac671fc3dc51933e5094001620c8908f1a03ac8292a37a2a314bd341bd. This shows the pin matches the published bytes. It does not attest how the release was built.
RISK-VALUE: DERIVED — $got -ne $Sha256 @ scripts/bootstrap-windows.ps1:108. It compares the Get-FileHash SHA256 of the download with the digest the manifest resolves to. On a mismatch it removes the temp file and throws before any Move-Item into the destination. `ValidatePattern('^[0-9a-f]{64}$')` @ :34 bounds the supplied digest to the 256-bit hex form. Between the last green Windows run (head 14e51a8146c3) and main, this line changed only in its message text: an em dash was replaced with "--". Proof that it works at Windows runtime on the current file is still missing, because row 8 reports fresh=no.
RISK-VALUE: DERIVED — reSHA256 = ^[0-9a-f]{64}$ @ tools/desk/cmd/deskinstall/install.go:62. Sixty-four lowercase hex characters is 256 bits, which is exactly what hex.EncodeToString(sha256.Sum256(...)) produces. The pattern is anchored at both ends, so an empty, truncated or uppercase pin is refused before any download.
RISK-VALUE: DERIVED — reTag = ^v[0-9]+\.[0-9]+\.[0-9]+ @ tools/desk/cmd/deskinstall/install.go:63, with tag != m.Tag @ :86. Together they require a semver-shaped tag that equals the component's committed tag, so "latest" can never be used (row 2). The tag only selects the URL; integrity rests on E1. Because the pin file is reviewed and committed while the download check runs on fetched bytes, the two fail for different reasons, which meets the brief's second-layer independence test.

Scope traceability: every row above names the Verify row it discharges (rows 4, 5 and 7 each have an as-authored line plus a corrective or supporting line). Two pieces of verified work map to no Verify row. The first is the absent-pin refusal test (install_test.go:185), which passed within the package. The second is the ledger-path line that the row-4 test asserts. Both are minor and neither blocks.

Findings:
- Row 7 check-definition: as written, the row can never be non-empty for this brief. The implementer's diff (PR #513, a single commit) did not touch the brief file. On merged main the instrument reports "nothing to corroborate", and at the branch head it reports COULD-NOT-CHECK exit 2. The fixed-here claim is corroborated here by hand, from the implementer commit's stat.
- Row 8 cannot be discharged by a verifier. The windows-ci-leg workflow has had no run since 2026-09-16, and the bootstrap scripts changed on 2026-09-23 (#1570). A maintainer dispatch of windows-ci-leg on a Windows runner at or after 0e2f4d161 is what flips fresh=no to fresh=yes. A verifier may not dispatch workflows.

rows_passed=8 rows_total=9 (rows 1, 2, 3, 4, 5, 5a, 6 and 7 pass; row 7 passes on its literal Expect and on the hand corroboration from git, because the instrument itself had nothing to check. Row 8 does not meet its Expect: fresh=no, and the host witness records it as fail. The last run was green, but no Windows-runtime run exists for the current bootstrap.)
RISK-VALUE: DERIVED — gotHex != wantHex @ tools/desk/cmd/deskinstall/install.go:110 and the four v1.0.29 Windows trust anchors @ plugins/assay/paired-versions.yaml:42,43,67,68 (recomputed, all match); see the lines above for the rest.
VERIFY: BLOCKED

## Review
Gate: **human** (from frontmatter). The human confirms the fork ruling was recorded before the
build, and that the sha256-verify-or-refuse control is the first post-download step and cannot be
bypassed — row 5 (negative path) plus row 5a (fail-first) together prove the refusal is
load-bearing, not decorative. A green happy-path row (4) with no negative-path row is exactly the
one-layer-verified failure this stream forbids on the security surface.
