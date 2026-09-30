---
brief: assay:assay:windows-port:01
title: Release build matrix — windows/amd64 + windows/arm64 + sha256s
why: >-
  Nothing downstream exists until a Windows binary does: no install path can fetch what the
  release never emits, no CI leg can smoke it, no adopter doc can point at it. Today
  release.yml cross-compiles statusgen and desk-tools for three Unix platforms only. Adding
  the two Windows targets is a mechanical extension of the existing cross-compile loop — Go
  builds them from the same Linux runner — and it is the single move that unblocks the whole
  stream.
wave: 1
depends: ["windows-port/00"]
unblocks: ["windows-port/03", "windows-port/04", "windows-port/05"]
effort: M
gate: human
gate-why: >-
  Amends the release workflow, which mints the artifacts consumers pin by sha256 in
  .assay-versions. A published release asset cannot be un-published once a consumer has
  fetched it, and a .github/workflows/ change cannot be pushed by an agent credential at
  all — it needs a workflow-scoped one, i.e. a human's hands. Both make this a human gate
  regardless of how mechanical the diff looks.
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
decision-trigger: creation
decision-issue: 1148
issues: [322]
schema: brief-v2
authored: 2026-09-01 by windows-port authoring session
sources:
  - "Ian's direction (2026-09-01): add windows/amd64 + windows/arm64 to the statusgen + desk-tools release build WITH sha256s, mirroring the per-platform pinned-artifact contract"
  - "plugins/assay/skills/install/SKILL.md §Scope (lines ~232-243): names the deferred Windows artifacts verbatim — statusgen-windows-amd64.exe, a cross-platform hash-verify, the .exe install path — as a not-yet-authored fast-follow"
  - ".github/workflows/release.yml (build loop ~782-784 statusgen, ~819-833 desk-tools, checksums ~839-849): the exact GOOS/GOARCH list and the single checksums.txt this brief extends"
  - "harness-portability/README.md (measured 2026-08-07): statusgen + tools/** are plain Go argv CLIs, nothing harness-specific — TRUE as a harness claim, but NOT the GOOS=windows claim this brief originally read into it (see #322)"
  - "medici-finance/assay#322 (ruling ratified 2026-09-02): the 'no source change needed' premise is retired — eight unix-only syscall sites block both modules; windows-port/00 fixes the source, this brief keeps its two-file scope and now depends on 00"
  - "freshness-checked 2026-09-01 @ origin/main: release.yml builds darwin-arm64, darwin-amd64, linux-amd64 only; zero GOOS=windows / .exe anywhere in .github/workflows or Makefile"
consumers:
  - ".github/workflows/release.yml: fixed-here (the build + checksum steps gain the two windows targets)"
  - "examples/adopter-scaffold/.assay-versions: fixed-here (illustrative windows pin lines added)"
  - "docs/streams/windows-port/brief-03-install-path.md: follow-up windows-port/03 (the install path selects the statusgen-windows-<arch>.exe asset by name)"
  - "docs/streams/windows-port/brief-04-windows-ci-leg.md: follow-up windows-port/04 (CI smokes the released windows asset)"
  - "docs/adopting-assay.md: follow-up windows-port/05 (the adopter doc names the windows assets)"
version: 1
id: 6a4da201-5caa-420c-9494-fb1b21bd5f1b
---

# Brief 01 — Release build matrix: windows/amd64 + windows/arm64 + sha256s

## Context

files:
- **amend** `.github/workflows/release.yml` — the `Build binaries` step (statusgen) and the
  `Build and package desk-tools binaries` step (desk-tools loop), plus the `Generate checksums`
  step. Add `windows/amd64` and `windows/arm64` to each.
- **amend** `examples/adopter-scaffold/.assay-versions` — add illustrative
  `statusgen-windows-amd64` / `statusgen-windows-arm64` pin lines (placeholder sha256 with a
  comment; the real hash is harvested from a published release, never a local build).

facts:
- **Today's matrix is three Unix targets.** statusgen: `GOOS=darwin GOARCH=arm64`,
  `GOOS=darwin GOARCH=amd64`, `GOOS=linux GOARCH=amd64` (raw binaries named
  `statusgen-<os>-<arch>`, no suffix). desk-tools: the same three, looped over `for platform in
  darwin-arm64 darwin-amd64 linux-amd64`, each tarred to `desk-tools-<platform>.tar.gz`.
- **Windows executables need `.exe`.** The two new statusgen assets are
  `statusgen-windows-amd64.exe` and `statusgen-windows-arm64.exe` — the suffix is part of the
  asset NAME (so `checksums.txt`, the pin file, and the install-path selector all carry it).
  Inside a desk-tools tarball, each `cmd/*` binary is built with a `.exe` suffix
  (`go build -o "$stage/$name.exe"` on the windows legs), then the tarball is
  `desk-tools-windows-amd64.tar.gz` / `desk-tools-windows-arm64.tar.gz`.
- **The build needs no Windows runner.** It is a plain `GOOS=… GOARCH=… go build` cross-compile
  on the existing self-hosted `medici-builder-release` (Linux) runner. Go cross-compiles Windows
  targets natively; nothing about this step touches a Windows host.
- **The source cross-compiles because `windows-port/00` made it — not because it always did.**
  This brief was authored on the premise that "the Go binaries are already portable, so no
  source change is needed." That premise was measured on 2026-08-07 as a HARNESS claim (plain
  argv CLIs), and working this brief disproved it as a `GOOS=windows` claim: eight unix-only
  `syscall` sites — a process-group kill, two `Stat_t` owner checks and five `flock` copies —
  fail to compile in `statusgen/` and `tools/desk/`, and `internal/deskkit` is imported by 38 of
  the 39 desk-tools commands. `medici-finance/assay#322` ruled the fix into its own wave-0 brief
  rather than widening this one, so `depends: ["windows-port/00"]`. Do NOT start this brief
  before 00 has merged: both release build steps run under `set -euo pipefail`, so a failing
  `GOOS=windows` line aborts the whole step and BREAKS today's working three-platform release
  rather than merely failing to add two more.
- **Checksums are one file.** The `Generate checksums` step runs a single `sha256sum` over every
  asset into `checksums.txt`; the two statusgen `.exe` assets and the two desk-tools windows
  tarballs are appended to that list. Consumers pin per-platform by sha256 in `.assay-versions`.
- **Version stamping is unchanged.** statusgen carries `-X main.statusgenVersion=$RELEASE_TAG`;
  desk-tools carries the three `deskkit.{SourceSHA,BuiltAt,ReleaseTag}` stamps. The windows
  builds reuse the SAME `$LDFLAGS` — the stamp is GOOS-independent.
- **arm64 is build-only here.** windows/arm64 is cross-compiled and checksummed like every other
  asset; whether its NATIVE behaviour is ever smoked is brief 04's open question, not this
  brief's — this brief only proves the two windows binaries build and are checksummed.

## Human decision
<!-- gate: human, decision-trigger: creation — filed as a self-contained decision issue.
     Written to be decided from THIS text alone: no links, no repo paths, no brief refs.
     NOTE: the linter's "no decision-issue" NOTICE clears only when a filed issue number is
     recorded as `decision-issue: <NN>` in the frontmatter above — this section is the text to
     file, not the filing itself. -->
The project publishes its command-line tools as version-pinned release downloads: each platform's
file is listed with a hash, and installers refuse anything whose hash does not match. Today three
platforms are published. This asks to add two more (the two Windows processor families), built by
the same cross-compiler on the same machine, listed in the same hash file.

Two properties make it a human call rather than a mechanical one:

1. **A published download cannot be recalled.** Once a release carries the two new files and
   anyone has fetched one, the hash is in the wild; a mistake is corrected by publishing a new
   version, never by withdrawing the old one. Approving this approves that permanence.
2. **Only a human can land it.** The change edits the automation file that decides what runs with
   the project's release credentials, and that file cannot be pushed by an automated identity —
   it needs a credential only a person holds.

The decision is a go/no-go on both, not a choice between designs: **authorize publishing the two
additional Windows downloads under the existing pinned-hash contract, and landing the automation
change by hand.** Answer "go" and the work proceeds; answer "no" and the whole Windows effort
stops here, because nothing downstream exists until the downloads do.

Default if no answer: none — blocks until answered.

## Ground rules
- NEVER git push / trigger workflows / run a release / run mutating infra commands. Commit only
  per the task instructions.
- **Additive-only on `release.yml`.** This brief ADDS two cross-compile targets and their
  checksum lines. It touches NONE of the workflow's security controls — not the `guard` job
  (tag-immutability), not `persist-credentials: false`, not the Go-toolchain sha256 pin, not the
  `sha256sum` checksum step's integrity. `irreversible: yes` is answered on the published-asset
  and workflow-path grounds in `gate-why`, not on any weakening of those controls; the other
  three risk answers are `no` on this basis. Per the security-gate rule, if the change would
  weaken any of those controls, STOP and escalate — it does not.
- Stop at `implemented` — you do not set verified/done, and you do not cut a release.
- Do NOT hand-write a real sha256 into `.assay-versions` — the real hash comes from a published
  release's `checksums.txt`; use a clearly-marked placeholder in the example file.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task

1. In `release.yml`'s statusgen `Build binaries` step, add two lines to the cross-compile block:
   `GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o ../statusgen-windows-amd64.exe .`
   and the `arm64` equivalent.
2. In the desk-tools build step, extend the `for platform in …` list with `windows-amd64` and
   `windows-arm64`, and make the per-`cmd` build suffix the output with `.exe` **on the windows
   legs only** (`ext=""`; `[ "$os" = windows ] && ext=".exe"`; `go build … -o "$stage/$name$ext"`).
   The tarball name follows the existing `desk-tools-$platform.tar.gz` shape.
3. In the `Generate checksums` step, append the four new asset names to the `sha256sum` argument
   list so `checksums.txt` covers all ten assets.
4. Add the two illustrative statusgen windows pin lines to
   `examples/adopter-scaffold/.assay-versions` with a placeholder sha256 and a
   `# harvested from the published release, not a local build` comment.
5. Confirm `windows-port/00` has MERGED and that `statusgen/` and `tools/desk/` therefore
   cross-compile on the branch base — rows 5 and 6 below are that confirmation. This brief still
   changes no Go source: if a unix-only `syscall` site or a `//go:build` constraint 00 did not
   cover still blocks a windows build, that is a gap in 00, not new scope here — report it on
   `medici-finance/assay#322`'s stream and STOP rather than editing `statusgen/` or
   `tools/desk/` from this brief.

## Verify (executable — no prose-only DoD items)

> **2026-09-27:** Verify rows re-authored for witness executability (#1805); no semantic change — rows 2, 5 and 8 now open with their runnable command as the first code span; no status change.

| # | Command | Expect |
|---|---------|--------|
| 1 | statusgen windows targets present in the build step: `grep -cE -e 'GOOS=windows GOARCH=amd64 go build' -e 'GOOS=windows GOARCH=arm64 go build' .github/workflows/release.yml` | `>= 2` (both arch lines) |
| 2 | `grep -c 'statusgen-windows-amd64[.]exe' .github/workflows/release.yml && grep -c 'statusgen-windows-arm64[.]exe' .github/workflows/release.yml` — the two statusgen assets carry the `.exe` suffix (`&&`, so a zero count on the first asset fails the row rather than being masked by the second) | each `>= 1` |
| 3 | desk-tools loop includes both windows platforms: `grep -oE -e 'windows-amd64' -e 'windows-arm64' .github/workflows/release.yml \| sort -u \| wc -l` | `2` |
| 4 | Checksums step covers all four new assets: `for a in statusgen-windows-amd64.exe statusgen-windows-arm64.exe desk-tools-windows-amd64.tar.gz desk-tools-windows-arm64.tar.gz; do grep -qF "$a" .github/workflows/release.yml \|\| { echo "MISSING $a"; exit 1; }; done; echo OK` | `OK` |
| 4a | **Positive control for row 4** — a bogus asset name is absent: `grep -qF 'statusgen-windows-mips.exe' .github/workflows/release.yml; echo $?` | `1` |
| 5 | `cd statusgen && GOOS=windows GOARCH=amd64 go build -o /tmp/wp01-sg-amd64.exe . && GOOS=windows GOARCH=arm64 go build -o /tmp/wp01-sg-arm64.exe . && file /tmp/wp01-sg-amd64.exe` — **Dereferencing — the windows binaries actually cross-compile** (proves `windows-port/00`'s split really landed on this branch's base, not just that YAML mentions the targets) | exit 0; `file` output contains `PE32` / `MS Windows` (a real Windows PE executable was produced) |
| 6 | **Dereferencing — desk-tools cross-compiles for windows too**: `cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /tmp/wp01-dt2.exe ./cmd/deskpost && file /tmp/wp01-dt2.exe` | exit 0; `file` output contains `PE32`/`MS Windows` (a representative desk verb cross-compiles to windows) |
| 7 | Example pin file gained the two windows lines: `grep -cE -e '^statusgen-windows-amd64 ' -e '^statusgen-windows-arm64 ' examples/adopter-scaffold/.assay-versions` | `2` |
| 8 | `d=$(mktemp -d) && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q 744fcfc621ad && test "$(git -C "$d" diff --name-only 744fcfc621ad^ 744fcfc621ad -- .github/workflows/release.yml examples/adopter-scaffold/.assay-versions \| grep -c .)" -eq 2 && statusgen --root "$d" --consumers --brief windows-port/01 --base 2f7ea5c64105^; rc=$?; rm -rf "$d"; exit $rc` — **Consumers routing corroborated by the diff**, pinned so it is not vacuous on merged main: a throwaway local clone is checked out at the implementing commit `744fcfc621ad` (which changes both `fixed-here` paths — asserted first), and `--consumers` judges the claims against the diff from before the brief wrote them (`2f7ea5c64105^`, the parent of the stream's authoring commit) to that commit. The implementing commit does not itself touch the brief file, so a `--base 744fcfc621ad^` run cannot select it; offline, local clone only | exit 0 (`0` DISPROVED) — every `consumers:` claim this brief makes is proved by the branch's own diff (release.yml + the example pin changed here; the follow-up edges reference briefs that cite 01) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|

### Non-implementer verifier run — 2026-09-12 sonnet-5-verifier (verify-desk dispatch), FIRST verify pass — **VERIFY: FAIL**

Runner ≠ implementer. Own temp worktree off origin/main, `KUBECONFIG=/dev/null`.

| # | Command | Expect | Observed | Date / Runner |
|---|---------|--------|----------|---------------|
| 1 | grep for GOOS=windows GOARCH={amd64,arm64} go build in release.yml | >=2 | exit 0, count=4 | 2026-09-12 sonnet-5-verifier |
| 2 | grep -c 'statusgen-windows-(amd64|arm64)[.]exe' release.yml | each >=1 | exit 0, 3 and 3 | 2026-09-12 sonnet-5-verifier |
| 3 | grep -oE windows-(amd64|arm64) release.yml, unique count | 2 | exit 0, 2 | 2026-09-12 sonnet-5-verifier |
| 4 | loop over 4 asset names, grep -qF each | OK | exit 0, OK | 2026-09-12 sonnet-5-verifier |
| 4a | positive control, bogus statusgen-windows-mips.exe | 1 | exit 1 (absent, as expected) | 2026-09-12 sonnet-5-verifier |
| 5 | cd statusgen && GOOS=windows GOARCH=amd64/arm64 go build | exit 0, PE32/MS Windows | exit 0, PE32+ executable (console) x86-64, for MS Windows | 2026-09-12 sonnet-5-verifier |
| 6 | cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskpost | exit 0, PE32/MS Windows | exit 0, PE32+ executable (console) x86-64, for MS Windows | 2026-09-12 sonnet-5-verifier |
| 7 | grep -cE for statusgen-windows-(amd64,arm64) in examples/adopter-scaffold/.assay-versions | 2 | **exit 0, count = 0 — FAIL.** Case-insensitive grep for windows on the whole file also returns nothing; the file has zero mention of Windows | 2026-09-12 sonnet-5-verifier |
| 8 | statusgen --root . --consumers windows-port/01 | exit 0 | exit 0, but message is "no brief files in the diff — nothing to corroborate" (expected on a post-merge worktree with no local diff; meaningful only on the implementer's own branch) | 2026-09-12 sonnet-5-verifier |

**Root cause of the row-7 failure — a real regression, not an unimplemented task.** This brief's own diff correctly added exactly the two illustrative pin lines row 7 checks for (statusgen-windows-amd64/-arm64, placeholder all-zero sha256, marked as harvested-from-release placeholders), and every other part of this brief's own diff is intact (rows 1-6, 4a all pass). A later, unrelated commit wholesale-rewrote examples/adopter-scaffold/.assay-versions to bump every pin to a newer umbrella version, and that rewrite silently dropped the two windows-port lines instead of carrying them forward — confirmed via git log -p, that commit is the last one to touch the file. The fix is a follow-up commit re-adding the two lines at the current pin version; a worker fix is already in flight for this (per the-desk relay).

`RISK-VALUE: DERIVED` — placeholder-sha256 = 64 zero-hex-digits @ examples/adopter-scaffold/.assay-versions (as merged in this brief's own PR, currently absent from main — see row 7 FAIL) — correct by construction: an all-zero digest cannot collide with any real release hash, and the brief's own ground rules require a clearly-marked placeholder here, never a real hash. This is illustrative documentation only (never read by an install/verify path), so it carries no operational risk on its own; the actual irreversible act this brief's `gate: human`/`irreversible: yes` answers is the human-only publish of the two new release assets under the existing pinned-hash contract, which this diff does not itself perform.

**VERIFY: FAIL — row 7.** Rows 1-6, 4a all PASS. Row 8 is only meaningful run on the implementer's own branch (recording as could-not-meaningfully-check on a post-merge worktree, not a real pass or fail). Per frontmatter `gate: human`, `irreversible: yes`: this verifier does not sign off and status does not change regardless of the FAIL. Status stays `implemented`; re-run row 7 once the pin-restoration fix lands.
### Verify pass 2026-09-22 (non-implementer, VERIFY: PASS — gate:human irreversible, held at implemented, routes to human gate #322)

Runner: `claude-opus-4-8[1m]` (non-implementer). Merged main `6204bb4f1eacc0229f2a86c8e0dce59edabdd22a`. Offline (`KUBECONFIG=/dev/null`; build/grep only, no release run). gate: human, risk {irreversible: yes}.

| # | Command | Expect | Observed (exit + key line) | Date | Runner |
|---|---------|--------|----------------------------|------|--------|
| 1 | grep -cE `GOOS=windows GOARCH={amd64,arm64} go build` release.yml | ≥2 | 0 → count=4 | 2026-09-22 | opus-4.8-verifier |
| 2 | grep -c `statusgen-windows-{amd64,arm64}.exe` release.yml | each ≥1 | 0 → 3 and 3 | 2026-09-22 | opus-4.8-verifier |
| 3 | grep -oE `windows-{amd64,arm64}` \| sort -u \| wc -l | 2 | 0 → 2 | 2026-09-22 | opus-4.8-verifier |
| 4 | loop grep -qF over 4 windows asset names | OK | 0 → OK; (4a positive control: bogus `-mips.exe` absent → exit 1 as expected) | 2026-09-22 | opus-4.8-verifier |
| 5 | `cd statusgen && GOOS=windows GOARCH=amd64/arm64 go build; file` | exit 0, PE32/MS Windows | 0 → amd64 `PE32+ (console) x86-64, MS Windows`; arm64 `PE32+ (console) Aarch64, MS Windows` | 2026-09-22 | opus-4.8-verifier |
| 6 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build ./cmd/deskpost; file` | exit 0, PE32/MS Windows | 0 → `PE32+ (console) x86-64, MS Windows` | 2026-09-22 | opus-4.8-verifier |
| 7 | grep -cE `^statusgen-windows-{amd64,arm64} ` examples/adopter-scaffold/.assay-versions | 2 | 0 → count=2 (2026-09-12 regression healed by #974, lines 23-24 present at pin v0.28.0) | 2026-09-22 | opus-4.8-verifier |
| 8 | `statusgen --root . --consumers windows-port/01` | exit 0 | 0 → "no brief files in the diff against 6204bb4f — nothing to corroborate" (expected post-merge; meaningful only on the implementer branch) | 2026-09-22 | opus-4.8-verifier |

Scope traceability: every Evidence row maps 1:1 to its Verify row. Checksum step (`sha256sum … > checksums.txt`) lists all four windows assets; asset-name spelling identical across build/checksum/upload/pin surfaces (rows 2,4,7). Rows 5/6 produce real Windows PE executables, proving windows-port/00's build-tag split landed on this base.

RISK-VALUE: DERIVED — placeholder-sha256 = 64×'e' @ `examples/adopter-scaffold/.assay-versions:23` (and 64×'f' @ :24) — a single hex digit repeated 64× is a recognizable FIXTURE placeholder matching the file's house convention + its explicit header ("The sha256 digests below are FIXTURE placeholders, not real release digests"); never read by any install/verify path, so it bears no operational risk and cannot be a real release digest.
RISK-VALUE: N/A (for the true irreversible act) — the irreversible act is the human-only PUBLISH of the two Windows release assets under the pinned-hash contract; this diff performs no transfer/spend/publish and writes no real hash literal (the real digest is harvested from a published checksums.txt at release time).

**VERIFY: PASS** — all 8 rows clean; rows 5/6 build real Windows PE executables; row 7 regression healed by #974. gate: human + irreversible: yes → a model records Evidence and does NOT sign off, and does NOT flip the verified cell. Status LEFT at `implemented`. Routed to the human publish-gate (decision issue #322): the go/no-go authorizes publishing the two Windows downloads under the existing pinned-hash contract AND landing the release.yml change by human hands (agent credentials lack workflow scope). On "go" the human lands; on "no" the windows-port stream stops.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -cE -e 'GOOS=windows GOARCH=amd64 go build' -e 'GOOS=windows GOARCH=arm64 go build' .github/workflows/release.yml` | pass exit=0 | sha256:7de1555df0c2 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 2 | `.exe` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:78190cea3904 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -oE -e 'windows-amd64' -e 'windows-arm64' .github/workflows/release.yml \| sort -u \| wc -l` | pass exit=0 | sha256:52dc20cec7d8 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 4 | `for a in statusgen-windows-amd64.exe statusgen-windows-arm64.exe desk-tools-windows-amd64.tar.gz desk-tools-windows-arm64.tar.gz; do grep -qF "$a" .github/workflows/release.yml \|\| { echo "MISSING $a"; exit 1; }; done; echo OK` | pass exit=0 | sha256:a12b7cb43c9d | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 4a | `grep -qF 'statusgen-windows-mips.exe' .github/workflows/release.yml; echo $?` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 5 | `windows-port/00` | could-not-run exit=127 — the shell could not execute the command (exit 127) | sha256:058f0d65478b | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /tmp/wp01-dt2.exe ./cmd/deskpost && file /tmp/wp01-dt2.exe` | pass exit=0 | sha256:3e9d54d9e4f4 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -cE -e '^statusgen-windows-amd64 ' -e '^statusgen-windows-arm64 ' examples/adopter-scaffold/.assay-versions` | pass exit=0 | sha256:53c234e5e847 | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |
| 8 | `statusgen --root . --consumers windows-port/01; echo $?` | pass exit=0 | sha256:ecb46196e63e | 2026-09-28 | assay-verifier-app[bot] @ c50a38fc1251 (on-behalf-of human:ian) (forge-identity) |

#### Witness notes — 2026-09-28 claude-opus-5-5 (verify-desk dispatch, non-implementer), third verify pass

Merged main c50a38fc12518a4eec4db37e8dd847d49e79149a (the witness stamp). Implementing commit 744fcfc621ad5801925d400aeebfa0558581768b (PR #464); pin-line restoration commit 6424f6be2 (PR #974). Witness run with the pinned statusgen v1.0.27 binary (sha256 matches the darwin-arm64 pin), not a wrapper, under a network-denied sandbox profile `(version 1)(allow default)(deny network*)` (no loopback allowance: no row needs a listener), with GOFLAGS=-count=1, GOPROXY=off, GOTOOLCHAIN=local and a fresh empty GOCACHE outside the checkout. No Verify row's first code span mints a credential, calls a forge, or mutates live state. Host is darwin: no row in this table needs a Windows host (every build row is a cross-compile), and neither deliverable is a CRLF file. No row is a check:ci row. No row writes into the source tree (rows 5 and 6 write their executables to the system temp dir).

Per-row key output (decoded from the witness hash, or re-run by hand under the same sandbox where the witness could not execute the intended command):
- Row 1: output `4` (Expect >= 2). Witness hash decodes to that output.
- Row 2: witness could-not-run, exit 127. The first code span in the Command cell is the prose token `.exe` ("carry `.exe`:"), not the grep pair, so the witness executed `.exe`. Check-definition defect, not an implementation defect. Hand re-run of the two intended greps: exit 0, output `3` and `3` (Expect each >= 1).
- Row 3: output `2` (Expect 2). Witness hash decodes to that output.
- Row 4: output `OK`. Witness hash decodes to that output.
- Row 4a: `; echo $?` tail, so the witness exit=0 says nothing by itself; output hash 4355a46b19d3 decodes to `1` (Expect 1). The bogus asset name is absent.
- Row 5: witness could-not-run, exit 127. The first code span is the prose token `windows-port/00` ("proves `windows-port/00`'s split …"), not the build chain. Check-definition defect. Hand re-run of the intended chain: exit 0; amd64 `PE32+ executable (console) x86-64, for MS Windows`; arm64 `PE32+ executable (console) Aarch64, for MS Windows`.
- Row 6: exit 0; `PE32+ executable (console) x86-64, for MS Windows` (witness hash decodes to that output).
- Row 7: output `2` (Expect 2). Lines 23 and 24 of the example pin file carry the two windows lines at v0.28.0.
- Row 8: vacuous on merged main. The witness ran on a clean tree at the merged head, so there is no branch diff to corroborate. A hand re-run reports `0 corroborated, 0 disproved, 5 unchecked`, exit 0. The row only means something on the implementer's pre-merge branch.

Beyond the table (read-only, verifier App token only): the build, checksum (release.yml lines 1173-1174, 1183-1184) and upload (lines 1381, 1385) lists spell the four windows asset names identically. The installer manifest (plugins/assay/paired-versions.yaml lines 42-43, 67-68) pins those same names. Its four v1.0.24 windows digests (a1555865524a…, c284b2b262fa…, 004d8359557d…, c803325bbb8f…) equal the lines in that published release's checksums.txt. The windows assets have been published on every release since v0.26.0 (2026-09-05). The latest, v1.0.27, carries all six windows assets (statusgen, qualgen and desk-tools, two archs each).

Risk-bearing value enumeration (diff scope: 744fcfc62 and 6424f6be2 over release.yml and the example pin file, plus the Deliverables):
- `GOOS/GOARCH = windows/amd64` @ .github/workflows/release.yml:1074, and the platform token `windows-amd64` @ :1144
- `GOOS/GOARCH = windows/arm64` @ .github/workflows/release.yml:1075, and the platform token `windows-arm64` @ :1144
- `asset name = statusgen-windows-amd64.exe` / `statusgen-windows-arm64.exe` @ .github/workflows/release.yml:1074-1075, 1173-1174, 1381
- `asset name = desk-tools-windows-amd64.tar.gz` / `desk-tools-windows-arm64.tar.gz` @ .github/workflows/release.yml:1183-1184, 1385 (built from the :1144 loop)
- `ext = ".exe"` (windows legs only) @ .github/workflows/release.yml:1148
- `checksum algorithm = sha256sum` @ .github/workflows/release.yml:1169 (unchanged by this diff; the diff appends four entries)
- `placeholder digest = 64 x 'e'` @ examples/adopter-scaffold/.assay-versions:23; `64 x 'f'` @ :24
- `pin tag = v0.28.0` @ examples/adopter-scaffold/.assay-versions:23-24

Rank: (1) the published asset names and (2) the windows/arm64 target are irreversible. Once published and fetched, a name or binary can only be superseded by a new release, never recalled, and the assets are already published. (3) The windows/amd64 target, the `.exe` suffix and the checksum coverage are irreversible for the same reason, but they can be derived. (4) The placeholder digests and the example pin tag are reversible documentation and are ranked last.

RISK-VALUE: DERIVED — asset names `statusgen-windows-amd64.exe` / `statusgen-windows-arm64.exe` @ .github/workflows/release.yml:1074-1075 — each is the existing `statusgen-<GOOS>-<GOARCH>` shape plus `.exe`, which Windows needs to run a file as a program. The same spelling appears in the checksum list (:1173-1174), the upload list (:1381) and the installer manifest (paired-versions.yaml:42-43). Those manifest digests equal the published v1.0.24 checksums.txt, so every producer and consumer names the same bytes.
RISK-VALUE: DERIVED — `GOOS/GOARCH = windows/amd64` @ .github/workflows/release.yml:1074 — this is Go's canonical port name (`go tool dist list` lists windows/386, windows/amd64, windows/arm64). The hand re-run of rows 5 and 6 produced a real x86-64 PE32+ executable.
RISK-VALUE: DERIVED — `ext = ".exe"` @ .github/workflows/release.yml:1148 — the guard `[ "$os" = windows ]` gives the suffix only to the windows legs, so the three Unix tarball layouts stay unchanged. The tarball names keep the `desk-tools-$platform.tar.gz` shape (:1183-1184) that paired-versions.yaml:67-68 pins.
RISK-VALUE: NAMED, NOT DERIVED — `GOOS/GOARCH = windows/arm64` @ .github/workflows/release.yml:1075 (and `windows-arm64` @ :1144) — the build is proven: row 5's hand re-run produced a real Aarch64 PE32+ executable. What is not derived is whether the published windows/arm64 binaries behave correctly on a native Windows-on-ARM host. That needs a windows/arm64 host or runner, and this darwin verifier has neither; the brief itself scopes arm64 as build-only. OPEN QUESTION FOR THE HUMAN: do you accept that published windows/arm64 statusgen and desk-tools assets, which have shipped on every release since v0.26.0, are verified only as cross-compiled, never run natively?
RISK-VALUE: DERIVED — placeholder digests `64 x 'e'` @ examples/adopter-scaffold/.assay-versions:23 and `64 x 'f'` @ :24 — the file header (line 17) marks every digest as a FIXTURE placeholder. The installer's real manifest is plugins/assay/paired-versions.yaml, not this example. A repeated single hex digit is clearly not a harvested digest, which is what the brief's ground rule asks for.

Observations (not failures of this brief):
- Rows 2 and 5 are check-definition defects. Each Command cell opens with a prose code span before the command, so the execution witness can never prove them. The fix is to re-author those two Verify cells so the command is the first code span. That is a brief edit, not a code change.
- Row 8 is vacuous on merged main, as in the two earlier passes.
- The earlier 2026-09-22 pass named #322 as the decision issue, but #322 is the cross-compile defect issue. The go/no-go decisions are #452 (ratified 2026-09-06) and #1148 ("go", 2026-09-15), both closed by the human, and the brief's frontmatter still records no `decision-issue:`.
- The example pin file's darwin lines (21-22) carry digests that look real even though the line-17 header calls every digest a fixture. These lines are outside this brief's diff.

**VERIFY: BLOCKED** — check-definition. The implementation is correct on every row: rows 1, 3, 4, 4a, 6 and 7 were proved by the witness, and rows 2 and 5 pass on a hand re-run. The execution witness still could not run rows 2 and 5, and row 8 is vacuous on merged main. gate: human, irreversible: yes. A model does not sign this off, and the status stays `implemented`. The windows/arm64 NAMED, NOT DERIVED value above is carried as the human's open question.
### Verification — 2026-09-30 (assay-verifier-app[bot] @ dd1582b7a5f3 (claude-opus-5-5) (on-behalf-of human:ian)) — 2026-09-30 claude-opus-5-5-verifier

Non-implementer re-verify on merged main dd1582b7a5f3ee59a550b7b2d73dc187cef451e5. gate: human — Evidence only; the brief stays implemented for the driver's sign-off card. First table: the `statusgen verifyrun` execution witness, landed verbatim. It ran on Linux (golang:1.25-bookworm pinned by digest, `--network none`, a full clone pinned to this SHA, statusgen built from the clone's own source). Second table: the hand run on the host (darwin/arm64, go1.27.1).

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `grep -cE -e 'GOOS=windows GOARCH=amd64 go build' -e 'GOOS=windows GOARCH=arm64 go build' .github/workflows/release.yml` | pass exit=0 | sha256:7de1555df0c2 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -c 'statusgen-windows-amd64[.]exe' .github/workflows/release.yml && grep -c 'statusgen-windows-arm64[.]exe' .github/workflows/release.yml` | pass exit=0 | sha256:9422b95a5e93 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 3 | `grep -oE -e 'windows-amd64' -e 'windows-arm64' .github/workflows/release.yml \| sort -u \| wc -l` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 4 | `for a in statusgen-windows-amd64.exe statusgen-windows-arm64.exe desk-tools-windows-amd64.tar.gz desk-tools-windows-arm64.tar.gz; do grep -qF "$a" .github/workflows/release.yml \|\| { echo "MISSING $a"; exit 1; }; done; echo OK` | pass exit=0 | sha256:a12b7cb43c9d | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 4a | `grep -qF 'statusgen-windows-mips.exe' .github/workflows/release.yml; echo $?` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd statusgen && GOOS=windows GOARCH=amd64 go build -o /tmp/wp01-sg-amd64.exe . && GOOS=windows GOARCH=arm64 go build -o /tmp/wp01-sg-arm64.exe . && file /tmp/wp01-sg-amd64.exe` | pass exit=0 | sha256:d8193eb02688 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /tmp/wp01-dt2.exe ./cmd/deskpost && file /tmp/wp01-dt2.exe` | pass exit=0 | sha256:c631241e0b69 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -cE -e '^statusgen-windows-amd64 ' -e '^statusgen-windows-arm64 ' examples/adopter-scaffold/.assay-versions` | pass exit=0 | sha256:53c234e5e847 | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |
| 8 | `d=$(mktemp -d) && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q 744fcfc621ad && test "$(git -C "$d" diff --name-only 744fcfc621ad^ 744fcfc621ad -- .github/workflows/release.yml examples/adopter-scaffold/.assay-versions \| grep -c .)" -eq 2 && statusgen --root "$d" --consumers --brief windows-port/01 --base 2f7ea5c64105^; rc=$?; rm -rf "$d"; exit $rc` | pass exit=0 | sha256:c4f24a35780f | 2026-09-30 | assay-verifier-app[bot] @ dd1582b7a5f3 (on-behalf-of human:ian) (forge-identity) |

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | grep -cE -e 'GOOS=windows GOARCH=amd64 go build' -e 'GOOS=windows GOARCH=arm64 go build' .github/workflows/release.yml | >= 2 | exit 0, output 4 (statusgen lines 1074-1075 and qualgen lines 1098-1099 both match). checked-clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 2 | grep -c 'statusgen-windows-amd64[.]exe' .github/workflows/release.yml && grep -c 'statusgen-windows-arm64[.]exe' .github/workflows/release.yml | each >= 1 | exit 0, output 3 and 3 (build 1074/1075, checksum 1173/1174, upload 1381). checked-clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 3 | grep -oE -e 'windows-amd64' -e 'windows-arm64' .github/workflows/release.yml \| sort -u \| wc -l | 2 | exit 0 (pipefail), output 2. The desk-tools loop at release.yml:1144 lists windows-amd64 and windows-arm64. checked-clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 4 | for a in statusgen-windows-amd64.exe statusgen-windows-arm64.exe desk-tools-windows-amd64.tar.gz desk-tools-windows-arm64.tar.gz; do grep -qF "$a" .github/workflows/release.yml \|\| { echo "MISSING $a"; exit 1; }; done; echo OK | OK | exit 0, output OK. The Generate checksums step (release.yml:1169-1184) lists all four names. checked-clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 4a | grep -qF 'statusgen-windows-mips.exe' .github/workflows/release.yml; echo $? | 1 | output 1 (the bogus name is absent, so row 4's grep can fail). checked-clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 5 | cd statusgen && GOOS=windows GOARCH=amd64 go build -o /tmp/wp01-sg-amd64.exe . && GOOS=windows GOARCH=arm64 go build -o /tmp/wp01-sg-arm64.exe . && file /tmp/wp01-sg-amd64.exe | exit 0; PE32 / MS Windows | exit 0, "PE32+ executable (console) x86-64, for MS Windows". The arm64 build gives "PE32+ executable (console) Aarch64, for MS Windows". checked-clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 6 | cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /tmp/wp01-dt2.exe ./cmd/deskpost && file /tmp/wp01-dt2.exe | exit 0; PE32 / MS Windows | exit 0, "PE32+ executable (console) x86-64, for MS Windows". checked-clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 7 | grep -cE -e '^statusgen-windows-amd64 ' -e '^statusgen-windows-arm64 ' examples/adopter-scaffold/.assay-versions | 2 | exit 0, output 2 (lines 23-24, v0.28.0, fixture digests 64 x 'e' and 64 x 'f'). checked-clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| 8 | d=$(mktemp -d) && git clone -q --shared --no-checkout . "$d" && git -C "$d" checkout -q 744fcfc621ad && test "$(git -C "$d" diff --name-only 744fcfc621ad^ 744fcfc621ad -- .github/workflows/release.yml examples/adopter-scaffold/.assay-versions \| grep -c .)" -eq 2 && statusgen --root "$d" --consumers --brief windows-port/01 --base 2f7ea5c64105^; rc=$?; rm -rf "$d"; exit $rc | exit 0, 0 DISPROVED | exit 0. Output: "summary: 5 corroborated, 0 disproved, 0 unchecked". All 5 consumers entries are CORROBORATED: release.yml and .assay-versions are fixed-here, and briefs 03 and 04 plus docs/adopting-assay.md are follow-ups. The row is not vacuous: the implementing commit touches both fixed-here paths (the test is -eq 2), and the base resolves to 031334633fe5. checked-clean | 2026-09-30 | assay-verifier-app[bot] (claude-opus-5-5) @ dd1582b7a5f3 (on-behalf-of human:ian) |
| R1 | (beyond the table; read-only, verifier App token) gh release view v1.0.29 -R medici-finance/assay and gh api repos/medici-finance/assay/releases/tags/v1.0.29 | the windows assets are published and checksummed | Tag v1.0.29 (Latest, published 2026-09-28T15:45:45Z, not a draft or prerelease; the tag ref is an annotated tag object e8914d5085e7) has 18 assets. The windows assets are statusgen-windows-amd64.exe, statusgen-windows-arm64.exe, qualgen-windows-amd64.exe, qualgen-windows-arm64.exe, desk-tools-windows-amd64.tar.gz and desk-tools-windows-arm64.tar.gz. checksums.txt has 15 lines, including all 6 windows names. checked-clean | 2026-09-30 | claude-opus-5-5-verifier |
| R2 | (beyond the table) gh release download v1.0.29 of checksums.txt and the 6 windows assets into scratch; grep windows checksums.txt \| shasum -a 256 -c | all OK | exit 0. All 6 windows assets report OK: each downloaded file's sha256 matches checksums.txt, and each also equals the API's server-side `digest` field. statusgen-windows-amd64.exe 6b8d7eda38a9…, statusgen-windows-arm64.exe 5059ccd54e25…, desk-tools-windows-amd64.tar.gz d2b4344a8805…, desk-tools-windows-arm64.tar.gz 0fbb57ac671f…, qualgen-windows-amd64.exe a38621eb8a33…, qualgen-windows-arm64.exe fde3a3bca68d…. file(1): the amd64 .exe files are PE32+ x86-64 and the arm64 .exe files are PE32+ Aarch64, all "for MS Windows". Each desk-tools windows tarball has 67 *.exe entries plus hooks/pre-push (for example ./deskpost.exe, PE32+ x86-64). checked-clean | 2026-09-30 | claude-opus-5-5-verifier |
| R3 | (beyond the table) grep windows plugins/assay/paired-versions.yaml | the installer manifest pins the published digests | Lines 42-43 pin statusgen-windows-{amd64,arm64}.exe at v1.0.29, and lines 67-68 pin desk-tools-windows-{amd64,arm64}.tar.gz at v1.0.29. All four full 64-hex digests equal the v1.0.29 checksums.txt lines exactly. checked-clean | 2026-09-30 | claude-opus-5-5-verifier |
| R4 | (beyond the table) gh api releases/tags/v1.0.28 and v1.0.27, windows asset names | same 6 windows assets | Both releases carry the same 6 windows asset names. checked-clean | 2026-09-30 | claude-opus-5-5-verifier |

RISK-VALUE: DERIVED — asset names statusgen-windows-amd64.exe / statusgen-windows-arm64.exe @ .github/workflows/release.yml:1074-1075 — each is the existing statusgen-<GOOS>-<GOARCH> shape plus ".exe", which Windows needs to run a file as a program. The same spelling appears at every step: the checksum list (:1173-1174), the explicit upload list (:1381), the installer manifest (paired-versions.yaml:42-43) and the published v1.0.29 release. The downloaded bytes' sha256 equals checksums.txt, the API digest and the manifest pin, so every producer and consumer names the same bytes.
RISK-VALUE: DERIVED — asset names desk-tools-windows-amd64.tar.gz / desk-tools-windows-arm64.tar.gz @ .github/workflows/release.yml:1183-1184 — these keep the existing desk-tools-$platform.tar.gz shape (:1160) with platform taken from the :1144 loop. The same names are in the upload list (:1385), paired-versions.yaml:67-68 and the published v1.0.29 release, and the sha256 matches on all four surfaces.
RISK-VALUE: DERIVED — GOOS/GOARCH = windows/amd64 @ .github/workflows/release.yml:1074 — this is Go's canonical port name (`go tool dist list` lists windows/386, windows/amd64 and windows/arm64). Row 5 produces an x86-64 PE32+ executable, and so does the published asset.
RISK-VALUE: DERIVED — GOOS/GOARCH = windows/arm64 @ .github/workflows/release.yml:1075 — this is Go's canonical port name for Windows on ARM64 (same `go tool dist list`). Row 5's arm64 build and the published statusgen/qualgen arm64 assets are PE32+ Aarch64 executables, "for MS Windows". The literal is right for what the brief scopes: arm64 is build-only here. Native execution is a separate scope question; see Notes N2.
RISK-VALUE: DERIVED — ext = ".exe" @ .github/workflows/release.yml:1148 — the guard [ "$os" = windows ] (with os="${platform%-*}" at :1145) gives the suffix only to the two windows legs, so the three Unix tarball layouts are unchanged. The published windows tarballs contain 67 *.exe binaries plus hooks/pre-push.
RISK-VALUE: DERIVED — placeholder digests 64 x 'e' / 64 x 'f' @ examples/adopter-scaffold/.assay-versions:23-24 — the file header at :17 says every digest is a FIXTURE placeholder. A single hex digit repeated 64 times cannot be a harvested digest, which is what the brief's ground rule asks for. The installer's real manifest is plugins/assay/paired-versions.yaml, not this example file.

Notes:
- All 9 Verify rows meet their Expect. The Linux witness passed 9/9, exit 0; every output hash was recomputed in the same container. The image carries no file(1), so Debian bookworm file 5.44 was mounted read-only for rows 5 and 6.
- The rows 2 and 5 check-definition defect from the 2026-09-28 pass is gone: after the 2026-09-27 re-authoring the witness runs every row as written. Row 8 now reports 5 corroborated, 0 disproved, 0 unchecked.
- Published release v1.0.29 (read-only): 6 windows assets (statusgen, qualgen, desk-tools; amd64 and arm64). All 6 were downloaded; each passes `shasum -c` against checksums.txt and equals the API digest, and plugins/assay/paired-versions.yaml lines 42-43 and 67-68 pin exactly those digests.
- For the driver: both go/no-go decision issues (#452 "ratified", #1148 "go") were closed by a human account acting as itself. The frontmatter records no `decision-issue:`, so lint prints a NOTICE; adding `decision-issue: 1148` clears it (metadata only).
- For the driver: windows/arm64 is build-only per the brief. The arm64 assets are proven built, named and sha256-matched, but none has run on a native Windows-on-ARM host (the native smoke job is held with `if: false`).

VERIFY: PASS

## Review
Gate: **human** (from frontmatter, risk-derived: `irreversible: yes`) — this edits
`.github/workflows/release.yml`, a security-classified path, and a published release asset
cannot be un-published once a consumer has fetched it; regulatory / customer / sensitive-data
are `no` (a cross-compile loop and a checksum list, otherwise git-revertible CI config). The reviewer
confirms rows 5 and 6 (the binaries genuinely cross-compile — the dereferencing rows), and that
the four new asset names appear identically in the build, checksum, and (illustrative) pin
surfaces so brief 03/04/05 read a stable name.

Rows 5 and 6 are also this brief's check on its dependency: they are copies of
`windows-port/00`'s rows 3 and 4, kept here so that a regression in the build-tag split shows up
as a RED on the brief that would ship a broken release, not only on the brief that fixed the
source. The reviewer confirms this brief's diff still touches exactly its two declared files —
adding a `statusgen/` or `tools/desk/` source edit here is out of scope and belongs to 00.
