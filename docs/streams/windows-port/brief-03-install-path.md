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

## Review
Gate: **human** (from frontmatter). The human confirms the fork ruling was recorded before the
build, and that the sha256-verify-or-refuse control is the first post-download step and cannot be
bypassed — row 5 (negative path) plus row 5a (fail-first) together prove the refusal is
load-bearing, not decorative. A green happy-path row (4) with no negative-path row is exactly the
one-layer-verified failure this stream forbids on the security surface.
