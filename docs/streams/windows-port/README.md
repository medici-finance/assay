---
stream: windows-port
repo: medici-finance/assay
serves: assay
status: active
priority: P2
track: platform
issues: [322]
board: generated
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
harness-specific (measured, `harness-portability` stream, 2026-08-07). **That 2026-08-07
measurement is stale in one respect, found while working brief 01 and ruled on
`medici-finance/assay#322`: "portable" was measured as harness-agnostic argv, not as
`GOOS=windows`-compiling, and today's `main` does not cross-compile for Windows at all —
eight unix-only `syscall` sites (a process-group kill, two `Stat_t` owner checks, five
`flock` copies) block both modules, and `internal/deskkit` is imported by 38 of the 39
desk-tools commands. Brief 00 (new, wave 0) is that source fix, and 01 now depends on it.**
What is *not* portable beyond that is the delivery and glue layer around them: the release
matrix that never emits a Windows binary, the install path that assumes a POSIX filesystem
and `make`, the shell hooks, and the absence of any CI that ever ran on Windows. This stream
ports the delivery layer and proves the result on a real Windows runner; it does not rewrite
the tools, which — once 00 lands — already run.

## End state — what "done" means

A Windows adopter, following `docs/adopting-assay.md`, acquires a **version-pinned,
sha256-verified** `statusgen`/`desk-tools` build for `windows/amd64` (and, cross-compiled,
`windows/arm64`), installs it through a documented Windows install path, and a **Windows CI
leg proves `statusgen --lint` exits 0 and a desk-verb smoke passes on Windows** — so the
"runs on Windows" claim is corroborated by a check, not asserted. Where a surface genuinely
cannot be made native on Windows (a `bash`+`jq` SessionStart hook, say), the gap is
**stated and triaged with a documented workaround**, never silently shipped broken.

**The end state also has a USABILITY half, added by the driver's 2026-09-11 ask** (briefs 06-09).
"Installable" is not the same claim as "installed in three commands", and the first was reached
while the second was not: today an adopter on Windows, using Cursor as the harness and GitLab as
the forge, follows roughly fifteen steps spread across PowerShell, Git-Bash/WSL and manual file
copies. The target shape is the Claude Code marketplace path's equal:

```
1.  powershell -File scripts/bootstrap-windows.ps1 -Tag vX.Y.Z
2.  deskinstall --harness cursor --forge gitlab --repo C:\src\myrepo
3.  invoke `assay:install` inside Cursor
```

No operator-supplied sha256 in (1) — it is resolved from the committed manifest, and the
verify-or-refuse control is unchanged. No manual copy in (2) — Cursor's install mechanism IS file
placement, so the tool does it. No Git-Bash prerequisite anywhere on the GitLab arm. The
fifteen-step path survives as a complete manual appendix; it stops being the only route.

## Scope — the ten units, and what each owns

**Units 0-5 delivered the RUNNABLE end state** (a pinned, verified Windows binary, installed, with
a CI leg proving it). **Units 6-9 deliver the USABLE one** — the same install in three commands
rather than fifteen steps across PowerShell, Git-Bash/WSL and manual file copies. The second half
is the driver's 2026-09-11 ask, and it changes no claim units 0-5 made: it removes the manual
surfaces between an adopter and those claims.


0. **Source build-tag split (brief 00).** `_unix.go` / `_windows.go` pairs for the eight
   unix-only `syscall` sites in `statusgen/` and `tools/desk/`, each Windows variant degrading
   explicitly and visibly rather than silently. Nothing cross-compiles until this lands.
   **Head of the critical path** (`medici-finance/assay#322`).
1. **Release build matrix (brief 01).** Add `windows/amd64` + `windows/arm64` to the
   `statusgen` and `desk-tools` release build, with per-platform `.exe`/tarball assets and
   sha256 lines in `checksums.txt`, mirroring the existing per-platform pinned-artifact
   contract. **Head of the delivery layer**, once 00 makes the source build.
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
6. **Manifest-driven bootstrap (brief 06).** `scripts/bootstrap-windows.ps1` stops demanding an
   operator-transcribed sha256 (`-Tag` and `-Sha256` are both `Mandatory=$true` today) and resolves
   tag + digest from the committed `plugins/assay/paired-versions.yaml` instead — the same value CI
   already extracts for itself. The verify-or-refuse is unchanged; the human transcription step is
   what goes. It also writes the user PATH entry the script currently leaves to the adopter.
   **Command 1 of three.**
7. **`deskinstall --harness cursor` (brief 07).** Cursor's install mechanism IS file placement, so
   the five-step manual copy (skills tree, references as a sibling so the `../../references/*.md`
   includes resolve, the `AGENTS.md` bindings) becomes one idempotent command with a `--check` that
   reports drift. **Command 2 of three.** Command 3 is invoking `assay:install` inside Cursor,
   which needs no new work — it is what the first two commands make possible.
8. **Go-native GitLab fleet provisioning (brief 08).** The 1167-line `tools/create-fleet-gitlab.sh`
   is bash + curl + jq, and the adoption docs tell a Windows adopter to run it from Git-Bash or WSL
   because "Native PowerShell cannot run it." A Go desk verb removes that prerequisite: the seven
   role service accounts and their PATs, `gitlab-<role>.token` files under the owner-only Windows
   ACL custody the toolchain already enforces on read, the `GITLAB_API_BASE` export, and a
   forge-neutral `create-labels` path (the GitHub one is nine hand-run `gh label create` lines; the
   GitLab one exists only inside that script). **`gate: human`** — it mints and persists live
   credentials.
9. **Docs and scope delta (brief 09).** The install skill's Windows scope widened from
   acquisition-only to the whole install; the adopter walkthrough collapsed to the three commands
   with the fifteen-step path kept complete as a manual appendix; and the docs/board skew on the
   Windows CI leg corrected — `docs/adopting-assay.md` still calls it "staged, pending promotion"
   while the workflow is live and brief 04 is `done` on this board.

**Out of scope:** rewriting the Go tools (already portable); a Windows container image; a
WSL-only path presented as "Windows support" (WSL is Linux — the claim is *native* Windows,
with WSL noted only as a fallback); publishing to any Windows package manager
(winget/Chocolatey) — that is a downstream distribution decision, not this stream.

**Also out of scope: the cockpit.** A cockpit (Herdr, Orca, or any other multi-agent supervisor
surface) is an OPTIONAL COMPOSITION layered AFTER install, not a step within it. The three commands
above leave an adopter with a working install and a working desk; what they choose to drive it with
is a separate decision with its own dependencies, and nothing in this stream's end state depends on
one existing. A brief here that reached for a cockpit would be widening the stream, not completing
it.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 00 | [Build-tag split for the unix-only syscall sites in statusgen and desk-tools](brief-00-unix-windows-build-tag-split.md) | 0 | M | implemented | — | — |
| 01 | [Release build matrix — windows/amd64 + windows/arm64 + sha256s](brief-01-release-build-matrix.md) | 1 | M | implemented | — | — |
| 02 | [Portability audit — enumerate + triage the shell-assuming surfaces](brief-02-portability-audit.md) | 0 | M | done | 2026-09-04 opus-4.8[1m]-verifier | 2026-09-04 assay-reviewer-app[bot] (approved PR #413 @ ae22e4fc5f1aac543f4e160cef027f2353a2260f) |
| 03 | [Windows install path — PowerShell-vs-Go-installer fork, then build](brief-03-install-path.md) | 2 | L | implemented | — | — |
| 04 | [Windows CI leg — statusgen --lint + a desk-verb smoke on Windows](brief-04-windows-ci-leg.md) | 2 | M | done | 2026-09-06 host (apply-gated) | 2026-09-08 human:reviewer |
| 05 | [Adoption-doc delta — the Windows adopter walkthrough](brief-05-adoption-doc-delta.md) | 3 | M | done | 2026-09-07 assay-verifier | 2026-09-07 assay-reviewer-app[bot] (approved PR #593 @ 57ac2401e4a807da14aef81d3a288431b7a5f148) |
| 06 | [Manifest-driven bootstrap — resolve tag + sha256 from the committed manifest, and write PATH](brief-06-manifest-driven-bootstrap.md) | 3 | M | implemented | — | — |
| 07 | [deskinstall --harness cursor — place the skills/references tree and write the AGENTS.md bindings](brief-07-deskinstall-harness-cursor.md) | 3 | M | implemented | — | — |
| 08 | [Go-native GitLab fleet provisioning — retire the bash+curl+jq script's Windows dependency](brief-08-go-native-gitlab-fleet-provisioning.md) | 1 | L | todo | — | — |
| 09 | [Three-command Windows install — widen the install skill's scope, collapse the walkthrough, correct the CI skew](brief-09-three-command-install-docs.md) | 4 | M | todo | — | — |
<!-- statusgen:briefs:end -->

Brief 04 implemented via PR #569 (the staged `ci/staged-workflows/windows-ci-leg.yml`) and
PR #583 (its promotion into `.github/workflows/`); the `windows-smoke` job runs green at
d684440 on the LF checkout the repo `.gitattributes` provides (#584/#585). The status flip to
`implemented` was omitted from those PRs and is recorded separately here; verification of the
`gate: human` Verify rows is a separate step.

## Critical path

```
[No external-environment head. Unlike the harness-portability stream (whose true head is a
 live Codex environment only Ian can provide), Windows CI runs on GitHub-hosted
 `windows-latest`, free for public repos — so nothing here waits on a procured environment.
 The one open scoping question — a NATIVE windows/arm64 smoke runner — is surfaced INSIDE
 brief 04 and does not gate the wave structure: arm64 ships cross-compiled + checksummed
 regardless; only its native-smoke coverage depends on runner availability.]
                      |
   00 build-tag split ──►── 01 release matrix ──┬──► 03 install path (Ian rules the fork) ──┐
                                                │                                           ├──► 05 adoption doc
   02 portability audit ────────────────────────┴──► 04 windows CI leg ────────────────────►┘

   ── second half: the three-command install (driver's ask, 2026-09-11) ──

                                       ┌──► 06 manifest-driven bootstrap ──┐
   03 install path ────────────────────┤                                   │
                                       └──► 07 deskinstall --harness cursor ┤──► 09 docs + scope
   00 build-tag split ──┐                                                  │     + CI skew
                        ├──► 08 go-native GitLab fleet (gate: human) ──────┘
   02 portability audit ┘
```

**In-stream head of the second half: 08 (GitLab fleet provisioning).** Not 06 and not 07 — and
this is the head that is easy to get wrong. 06 and 07 are each one artifact with an offline test
suite and a `depends:` on work that has already landed (`03`, `implemented`); either could start
today. 08 is `gate: human` with `decision-trigger: creation`, so its decision issue is filed as the
brief lands and **nothing is implementable until a human rules the PAT-custody fork** — and its
dependency chain reaches all the way back to wave 0 (`00`, `02`). It is also the largest unit (L)
and the only one that mints credentials. A plan that sequenced 06 → 07 → 08 → 09 would put the
longest-lead, human-gated item last and discover the wait at the end; 08 starts its human gate
FIRST, in parallel with 06 and 07, and 09 gathers all three.

**Longest chain of the whole stream: `00 → 01 → 03 → 06|07 → 09`.** 08's own chain
(`00 → 08 → 09`) is shorter in hops but longer in wall-clock, because a human ruling is not a hop.
Track both.

**In-stream head of the first half: 00 (build-tag split).** Longest chain is `00 → 01 → 03 → 05`. 00 is at the
head because **nothing cross-compiles until it lands** — `statusgen` and `tools/desk` both fail
`GOOS=windows go build` on today's `main`, so 01 can add every Windows target it likes and the
release build will only break. 01 follows because **nothing installs, smoke-tests, or is
documented until a Windows binary exists** — 03 has nothing to install, 04 has nothing to
smoke, and 05 has no artifact to point an adopter at, until the release emits
`statusgen-windows-*` and `desk-tools-windows-*`.

**00 is the head because the original head was measured wrong, not because the plan was
re-drawn for taste.** This stream first read 01 as the head on the `harness-portability`
finding (measured 2026-08-07) that the Go tools "cross-compile with no source change." Working
01 disproved it: eight unix-only `syscall` sites block both modules, `internal/deskkit` is
imported by 38 of the 39 desk-tools commands, and 01's own Verify rows 5 and 6 (a real PE32
build of each) were unsatisfiable against `main`. That is `medici-finance/assay#322`; the
ruling (2026-09-02) was to split rather than widen 01 — hence a new wave-0 brief 00, with 01
keeping its two-file scope and gaining `depends: ["windows-port/00"]`.

**02 remains a genuine wave-0 peer.** The audit covers the DELIVERY and GLUE layer (hooks,
install path, config home, shell-outs); 00 covers the Go SOURCE. They touch different files
and neither blocks the other, so they still run in parallel.

### Tempting-but-wrong first steps

- **Starting at 01 (add the Windows targets to the release matrix now).** This was the plan's
  own first belief and it is wrong: both release build steps run under `set -euo pipefail`, so
  the first failing `GOOS=windows` line aborts the whole step — shipping 01 alone would break
  today's working three-platform release rather than merely failing to add two more. 00 first.
- **Starting at 02 (audit the shell surfaces first).** Feels like the responsible ordering,
  but the audit gates the *install path and the docs*, not the binaries. Sequencing the whole
  stream behind the audit delays the head for no dependency reason. 02 runs *parallel* to 00
  in wave 0.
- **Widening 00 into a portability rewrite.** 00 is a compile-target split of existing logic:
  same unix behaviour, same error strings, Windows variants that degrade out loud. Treating it
  as licence to re-architect the lock or the roster trust check turns a wave-0 unblocker into
  an open-ended source brief, which is precisely what the #322 ruling declined.
- **Starting at 03 (write the installer now).** There is nothing to install until 01 emits
  a Windows asset, and the installer's shape depends on the fork Ian rules — writing it
  first bakes in a fork decision the human gate exists to make.
- **Building a Windows CI leg (04) before 01.** A CI leg can `go build` from source on
  Windows and prove `--lint` runs, but the stream's claim is that the *pinned release
  binary* works on Windows; smoking a from-source build proves a weaker thing. 04 smokes the
  released asset, so it follows 01.
- **Starting the second half at 09 (fix the docs now).** Tempting because two documented claims
  are already wrong today — the install skill's acquisition-only scope and the "staged, pending
  promotion" CI-leg paragraph — so a docs pass looks like free value. It is not: 09's whole job is
  to describe the three commands, and two of the three do not exist until 06 and 07 land. Writing
  the walkthrough first produces a document that describes a plan, which is exactly how
  `docs/adopting-assay.md:978` came to be stale in the first place. The two standing errors are
  small enough to ride along with 09 rather than to justify inverting its dependency.
- **Treating 08 as "just a rewrite of a shell script".** It is a rewrite of a shell script that
  mints seven live access tokens and writes them to disk. The custody question on Windows has no
  settled answer (NTFS has no equivalent of the `chmod 0600` the script runs — the repo's own
  `custodyacl.go` says a normal file reads `0666` there), which is why the brief carries a
  `## Human decision` and `sensitive-data: yes`. Sizing it from its line count rather than from
  what it handles is how it gets dispatched to the wrong tier.
- **Assuming `.exe` handling is free.** The current `statusgen-<platform>` assets are raw
  binaries with no suffix; Windows executables need `.exe`, and the checksum + pin lines
  must carry the suffixed names. This is the one concrete wrinkle in the delivery layer
  (owned by 01), not a blocker — but it is not zero.

## Dependency waves

```
Wave 0: [00, 02]                 (independent; 00 = Go source split, 02 = shell-surface triage)
Wave 1: [01]←{00}, [08]←{00,02}
Wave 2: [03]←{01,02}, [04]←{01,02}
Wave 3: [05]←{02,03,04}, [06]←{03}, [07]←{03}
Wave 4: [09]←{06,07,08}
```

Critical path (first half): `00 → 01 → 03 → 05`. 02 runs parallel to 00 in wave 0 and feeds 03, 04,
and 05. 04 runs parallel to 03 in wave 2 (both need the Windows binary from 01 and the triage from
02); 05 is the end-state doc and gathers the install path (03), the CI proof (04), and the
triage (02).

Critical path (second half): `00 → 01 → 03 → 06|07 → 09`, with `00 → 08 → 09` running alongside.
06 and 07 are peers in wave 3 — each edits a different artifact (`scripts/bootstrap-windows.ps1`
and `tools/desk/cmd/deskinstall/`), so they carry no edge between them and can be dispatched
together. 08 sits at wave 1 by dependency but is dispatched EARLY for wall-clock reasons, not wave
reasons: its human gate is the stream's longest lead. 09 is the gathering doc brief and is the only
wave-4 item; it cannot start until all three of its dependencies have landed their artifacts,
because its Verify table dereferences those artifacts' real flag surfaces rather than the briefs'
prose.

## Gate distribution — derived, not spread

**03 is `gate: human`**: the PowerShell-script-vs-Go-installer fork is a design commitment
that shapes the maintenance surface of every future Windows adopter *and* the CI leg (04) and
the adoption doc (05) both bind to whichever is chosen — the maintainer commits that fork,
the same way `harness-portability/03` reserved the target-set/channel ruling to Ian. Its four
risk answers are all `no` (nothing here touches funds, customers, regulators, or an
irreversible surface — everything is git-revertible tooling and docs); the `human` gate is a
**design-commitment** gate, and the `gate-why` says so.

**08 is `gate: human` for a different reason — a risk answer, not a design commitment.** It is the
only brief in the stream that answers a risk question `yes`: `sensitive-data`, because it mints
seven live GitLab personal access tokens and persists each to a file every desk verb then reads.
The gate is derived there, not chosen. What the human confirms is enumerated in its `gate-why` and
decided in its `## Human decision` (`decision-trigger: creation`, so the decision issue is filed as
the brief lands): the PAT custody model on native Windows — NTFS has no equivalent of the
`chmod 0600` the existing shell script runs, and the repo's own `custodyacl.go` records that a
normal file reads `0666` there — and what a partially-completed provisioning run does about
credentials it has already minted.

All other briefs — 00, 01, 02, 04, 05, 06, 07, 09 — answer the four risk questions `no` and gate
`model`.

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
- **A Windows-runtime Verify row is could-not-check for an offline POSIX verifier, never a pass.**
  Observed twice on 03's row 8 (2026-09-07 and 2026-09-10). Briefs in this stream therefore split
  their tables deliberately: logic a POSIX verifier CAN exercise (Go tests, static assertions over
  the script text) separated from rows only `windows-latest` can discharge, each labelled so the
  verifier records the second group as could-not-check with its reason rather than greening it from
  the first.
- **`chmod` is not the Windows answer for credential files.** `os.FileMode`'s permission bits are
  synthetic on NTFS — a normal file reads `0666` — so a `chmod 600` that reports `0600` without
  tightening the DACL is a fake green. The toolchain's owner-only ACL evaluation
  (`tools/desk/internal/deskkit/custodyacl.go`, `custodyowner_windows.go`) is the one home for that
  decision; a brief here CALLS it and never re-derives an ACL opinion of its own.
- **The cockpit is a composition, not a step.** Any supervisor/cockpit surface layered over a
  working install is out of this stream's scope and out of its end state — see **Out of scope**.
