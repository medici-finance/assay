---
stream: windows-port
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: []
---

# Windows Port Stream

Make an **adopter on Windows able to run the pinned Assay release, CI-proven on Windows.**
Today the shipped toolchain is Unix-first by every measure: the `release` workflow
cross-compiles `statusgen` and `desk-tools` for `darwin-arm64`, `darwin-amd64`, and
`linux-amd64` only; the only documented install path is `sudo make desk-install` into
root-owned `/opt/desk-tools/bin`; the SessionStart hooks are `#!/bin/bash` scripts invoked
as `bash "…"` and depend on `jq`; the push guard is a `#!/bin/sh` shim execing an absolute
POSIX path; and the gap is stated outright in two places, each in its own words —
`docs/adopting-assay.md`'s **Prerequisites** list: **"Windows is a named fast-follow, not yet
in scope — on a Windows host the skill stops at that step rather than guessing"**, and
`plugins/assay/skills/install/SKILL.md` **§Scope**: **"Windows is a deferred fast-follow — NOT
in this skill's scope yet."** This stream is that fast-follow: it turns the stated gap into a
delivered, CI-proven path. (Both sentences are quoted from their own file; `windows-port/05`
retires both.)

The claim is deliberately bounded. The Go binaries themselves are already portable —
`statusgen` and everything under `tools/**` are plain argv CLIs with nothing
harness-specific (measured, `harness-portability` stream, 2026-08-07). What is *not* portable
is the delivery and glue layer around them: the release matrix that never emits a Windows
binary, the install path that assumes a POSIX filesystem and `make`, the shell hooks, and
the absence of any CI that ever ran on Windows. This stream ports the delivery layer and
proves the result on a real Windows runner; it does not rewrite the tools, which already run.

## End state — what "done" means

A Windows adopter, following `docs/adopting-assay.md`, acquires a **version-pinned,
sha256-verified** `statusgen`/`desk-tools` build for `windows/amd64` (and, cross-compiled,
`windows/arm64`), installs it through a documented Windows install path, and a **Windows CI
leg proves `statusgen --lint` exits 0 and a desk-verb smoke passes on Windows** — so the
"runs on Windows" claim is corroborated by a check, not asserted. Where a surface genuinely
cannot be made native on Windows (a `bash`+`jq` SessionStart hook, say), the gap is
**stated and triaged with a documented workaround**, never silently shipped broken.

## Scope — the five units, and what each owns

1. **Release build matrix (brief 01).** Add `windows/amd64` + `windows/arm64` to the
   `statusgen` and `desk-tools` release build, with per-platform `.exe`/tarball assets and
   sha256 lines in `checksums.txt`, mirroring the existing per-platform pinned-artifact
   contract. **Head of the critical path.**
2. **Install path (brief 03).** The `desk-install` equivalent for Windows. Carries a
   **surfaced fork — a PowerShell install script vs a Go-native `install` subcommand** —
   with pros/cons and a recommendation, decided at the human gate rather than pre-decided
   silently, then implemented with the sha256-verify-or-refuse control intact.
3. **Portability audit (brief 02).** Enumerate and triage every shell-assuming surface —
   the `bash`/`jq` hooks, the `#!/bin/sh` push-guard shim, the `sudo make desk-install`
   POSIX install, desk verbs that shell out, path-separator assumptions, `~/.config`
   vs `%APPDATA%` — as a decision table (works / needs-port / documented-workaround /
   out-of-scope).
4. **Windows CI leg (brief 04).** A Windows runner leg proving `statusgen --lint` and a
   desk-verb smoke pass on Windows. The corroboration half of the end state.
5. **Adoption-doc delta (brief 05).** The Windows adopter walkthrough in
   `docs/adopting-assay.md`, mirroring the existing per-scenario runbook pattern, replacing
   the "not yet in scope" stub with a real path.

**Out of scope:** rewriting the Go tools (already portable); a Windows container image; a
WSL-only path presented as "Windows support" (WSL is Linux — the claim is *native* Windows,
with WSL noted only as a fallback); publishing to any Windows package manager
(winget/Chocolatey) — that is a downstream distribution decision, not this stream.

## Briefs

| # | Brief | Wave | Effort | Gate | Status | Verified | Reviewed |
|---|-------|------|--------|------|--------|----------|----------|
| 01 | [Release build matrix — windows/amd64 + windows/arm64 + sha256s](./brief-01-release-build-matrix.md) | 0 | M | human | todo | — | — |
| 02 | [Portability audit — enumerate + triage the shell-assuming surfaces](./brief-02-portability-audit.md) | 0 | M | model | todo | — | — |
| 03 | [Windows install path — PowerShell-vs-Go-installer fork, then build](./brief-03-install-path.md) | 1 | L | human | todo | — | — |
| 04 | [Windows CI leg — statusgen --lint + a desk-verb smoke on Windows](./brief-04-windows-ci-leg.md) | 1 | M | human | todo | — | — |
| 05 | [Adoption-doc delta — the Windows adopter walkthrough](./brief-05-adoption-doc-delta.md) | 2 | M | model | todo | — | — |

## Critical path

```
[No external-environment head. Unlike the harness-portability stream (whose true head is a
 live Codex environment only Ian can provide), Windows CI runs on GitHub-hosted
 `windows-latest`, free for public repos — so nothing here waits on a procured environment.
 The one open scoping question — a NATIVE windows/arm64 smoke runner — is surfaced INSIDE
 brief 04 and does not gate the wave structure: arm64 ships cross-compiled + checksummed
 regardless; only its native-smoke coverage depends on runner availability.]
                      |
   01 release matrix ──►──┬──► 03 install path (Ian rules the fork) ──►──┐
                          │                                              ├──► 05 adoption doc
   02 portability audit ──┴──► 04 windows CI leg ──────────────────────►─┘
```

**In-stream head: 01 (release build matrix).** Longest chain is `01 → 03 → 05`. 01 is at the
head because **nothing installs, smoke-tests, or is documented until a Windows binary
exists** — 03 has nothing to install, 04 has nothing to smoke, and 05 has no artifact to
point an adopter at, until the release emits `statusgen-windows-*` and
`desk-tools-windows-*`.

**Verified, not assumed — 01 is a REAL head, and it is actually unblocked.** The tempting
wrong belief is that the *portability audit* (02) must precede the build — "you cannot build
Windows binaries until you know what is portable." That is false and was checked against the
code: `statusgen`/`tools/**` are plain Go argv CLIs (`harness-portability` README, measured
2026-08-07), and `release.yml`'s build step is a plain `GOOS=… GOARCH=… go build` cross-compile
loop on the Linux `medici-builder-release` runner. Adding `GOOS=windows GOARCH=amd64` and
`GOOS=windows GOARCH=arm64` is a mechanical extension of that loop; **no Windows runner is
needed for the build**, and the Go tools' own portability is a *different layer* from the shell
glue 02 audits. So 01 has no true blocker hiding upstream of it — it is genuinely first, and
genuinely doable now.

### Tempting-but-wrong first steps

- **Starting at 02 (audit the shell surfaces first).** Feels like the responsible ordering,
  but the audit gates the *install path and the docs*, not the binaries — the binaries
  cross-compile today. Sequencing the whole stream behind the audit delays the head for no
  dependency reason. 02 runs *parallel* to 01 in wave 0.
- **Starting at 03 (write the installer now).** There is nothing to install until 01 emits
  a Windows asset, and the installer's shape depends on the fork Ian rules — writing it
  first bakes in a fork decision the human gate exists to make.
- **Building a Windows CI leg (04) before 01.** A CI leg can `go build` from source on
  Windows and prove `--lint` runs, but the stream's claim is that the *pinned release
  binary* works on Windows; smoking a from-source build proves a weaker thing. 04 smokes the
  released asset, so it follows 01.
- **Assuming `.exe` handling is free.** The current `statusgen-<platform>` assets are raw
  binaries with no suffix; Windows executables need `.exe`, and the checksum + pin lines
  must carry the suffixed names. This is the one concrete wrinkle at the head (owned by 01),
  not a blocker — but it is not zero.

## Dependency waves

```
Wave 0: [01, 02]                 (independent; 01 = binaries, 02 = shell-surface triage)
Wave 1: [03]←{01,02}, [04]←{01,02}
Wave 2: [05]←{02,03,04}
```

Critical path: `01 → 03 → 05`. 02 runs parallel to 01 in wave 0 and feeds 03, 04, and 05.
04 runs parallel to 03 in wave 1 (both need the Windows binary from 01 and the triage from
02); 05 is the end-state doc and gathers the install path (03), the CI proof (04), and the
triage (02).

## Gate distribution — derived, not spread

**03 is `gate: human`**: the PowerShell-script-vs-Go-installer fork is a design commitment
that shapes the maintenance surface of every future Windows adopter *and* the CI leg (04) and
the adoption doc (05) both bind to whichever is chosen — the maintainer commits that fork,
the same way `harness-portability/03` reserved the target-set/channel ruling to Ian. Its four
risk answers are all `no` (nothing here touches funds, customers, regulators, or an
irreversible surface — everything is git-revertible tooling and docs); the `human` gate is a
**design-commitment** gate, and the `gate-why` says so. All other briefs answer the four risk
questions `no` and gate `model`.

The one **security-relevant** surface lives inside 03 and is *not* a gate question but a
design requirement: the Windows install path MUST sha256-verify the pinned binary before it
runs, and REFUSE on mismatch — mirroring `assay:install`'s "hash mismatch is a hard REFUSE,
not a warning." 03 carries a negative-path Verify row proving the refusal fires on a tampered
binary.

## Cross-repo and out-of-repo dependencies (facts, not `depends:`)

`depends:` arrays are in-repo only. These are sequencing facts the desk carries:

| External | Relationship | Note |
|---|---|---|
| GitHub-hosted `windows-latest` runner | **CI substrate for 04** | Free for public repos; no procurement, no external head |
| A native `windows-11-arm` hosted runner | **Open question for 04** | Gates NATIVE windows/arm64 smoke only; arm64 still ships cross-compiled + checksummed. Surfaced in 04, resolved there, not a stream blocker |
| The `release` environment + `medici-builder-release` runner | **Prereq for 01's release run** | Existing repo-admin acts (the release workflow already documents them); 01 only edits the build loop, it does not run a release |
| `.assay-versions` pin contract (consumers) | **Shared value 01 extends** | 01 adds `statusgen-windows-*` / `desk-tools-windows-*` asset names; 03/04/05 consume them (see 01's `consumers:`) |

## Shared conventions

- **Asset naming is the shared value across the stream.** 01 fixes the Windows asset names
  (`statusgen-windows-amd64.exe`, `statusgen-windows-arm64.exe`,
  `desk-tools-windows-amd64.tar.gz`, `desk-tools-windows-arm64.tar.gz`); 03 (install), 04
  (CI), and 05 (docs) all read those exact names. A rename after 01 lands is a shared-value
  change and re-triggers 03/04/05's consumer rows.
- **`.exe` is not optional and not free.** Every Windows binary asset carries `.exe`; every
  checksum line, pin line, and install-path selection matches the suffixed name.
- **Native, not WSL.** "Runs on Windows" means a native `windows/amd64` process. WSL is
  Linux and is documented only as a fallback, never as the claim.
- **Blocked is a state, not a failure.** A Verify row that needs a runner not yet available
  (e.g. native arm64 smoke) is marked `BLOCKED (needs windows-arm64 runner)` in Evidence with
  the reason — never run vacuously, never greened from the amd64 result.
- **Every absence-assertion grep pairs a positive control**: a zero with no control is not
  evidence.
