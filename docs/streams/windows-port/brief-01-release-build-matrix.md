---
brief: windows-port/01
title: Release build matrix — windows/amd64 + windows/arm64 + sha256s
why: >-
  Nothing downstream exists until a Windows binary does: no install path can fetch what the
  release never emits, no CI leg can smoke it, no adopter doc can point at it. Today
  release.yml cross-compiles statusgen and desk-tools for three Unix platforms only. Adding
  the two Windows targets is a mechanical extension of the existing cross-compile loop — Go
  builds them from the same Linux runner — and it is the single move that unblocks the whole
  stream.
wave: 0
depends: []
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
issues: []
schema: brief-v1
authored: 2026-09-01 by windows-port authoring session
sources:
  - "Ian's direction (2026-09-01): add windows/amd64 + windows/arm64 to the statusgen + desk-tools release build WITH sha256s, mirroring the per-platform pinned-artifact contract"
  - "plugins/assay/skills/install/SKILL.md §Scope (lines ~232-243): names the deferred Windows artifacts verbatim — statusgen-windows-amd64.exe, a cross-platform hash-verify, the .exe install path — as a not-yet-authored fast-follow"
  - ".github/workflows/release.yml (build loop ~782-784 statusgen, ~819-833 desk-tools, checksums ~839-849): the exact GOOS/GOARCH list and the single checksums.txt this brief extends"
  - "harness-portability/README.md (measured 2026-08-07): statusgen + tools/** are plain Go argv CLIs, nothing harness-specific — so they cross-compile to windows with no source change"
  - "freshness-checked 2026-09-01 @ origin/main: release.yml builds darwin-arm64, darwin-amd64, linux-amd64 only; zero GOOS=windows / .exe anywhere in .github/workflows or Makefile"
consumers:
  - ".github/workflows/release.yml: fixed-here (the build + checksum steps gain the two windows targets)"
  - "examples/adopter-scaffold/.assay-versions: fixed-here (illustrative windows pin lines added)"
  - "docs/streams/windows-port/brief-03-install-path.md: follow-up windows-port/03 (the install path selects the statusgen-windows-<arch>.exe asset by name)"
  - "docs/streams/windows-port/brief-04-windows-ci-leg.md: follow-up windows-port/04 (CI smokes the released windows asset)"
  - "docs/adopting-assay.md: follow-up windows-port/05 (the adopter doc names the windows assets)"
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
- **Checksums are one file.** The `Generate checksums` step runs a single `sha256sum` over every
  asset into `checksums.txt`; the two statusgen `.exe` assets and the two desk-tools windows
  tarballs are appended to that list. Consumers pin per-platform by sha256 in `.assay-versions`.
- **Version stamping is unchanged.** statusgen carries `-X main.statusgenVersion=$RELEASE_TAG`;
  desk-tools carries the three `deskkit.{SourceSHA,BuiltAt,ReleaseTag}` stamps. The windows
  builds reuse the SAME `$LDFLAGS` — the stamp is GOOS-independent.
- **arm64 is build-only here.** windows/arm64 is cross-compiled and checksummed like every other
  asset; whether its NATIVE behaviour is ever smoked is brief 04's open question, not this
  brief's — this brief only proves the two windows binaries build and are checksummed.

## Ground rules
- NEVER git push / trigger workflows / run a release / run mutating infra commands. Commit only
  per the task instructions.
- **Additive-only on `release.yml`.** This brief ADDS two cross-compile targets and their
  checksum lines. It touches NONE of the workflow's security controls — not the `guard` job
  (tag-immutability), not `persist-credentials: false`, not the Go-toolchain sha256 pin, not the
  `sha256sum` checksum step's integrity. The four risk answers are `no` on that basis. Per the
  security-gate rule, if the change would weaken any of those, STOP and escalate — it does not.
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
5. Confirm no source change is needed in `statusgen/` or `tools/desk/` — the Go builds
   cross-compile as-is (rule: report if any `//go:build` constraint or a `syscall`/unix-only
   import blocks a windows build).

## Verify (executable — no prose-only DoD items)

| # | Command | Expect |
|---|---------|--------|
| 1 | statusgen windows targets present in the build step: `grep -cE -e 'GOOS=windows GOARCH=amd64 go build' -e 'GOOS=windows GOARCH=arm64 go build' .github/workflows/release.yml` | `>= 2` (both arch lines) |
| 2 | The two statusgen assets carry `.exe`: `grep -c 'statusgen-windows-amd64[.]exe' .github/workflows/release.yml; grep -c 'statusgen-windows-arm64[.]exe' .github/workflows/release.yml` | each `>= 1` |
| 3 | desk-tools loop includes both windows platforms: `grep -oE -e 'windows-amd64' -e 'windows-arm64' .github/workflows/release.yml \| sort -u \| wc -l` | `2` |
| 4 | Checksums step covers all four new assets: `for a in statusgen-windows-amd64.exe statusgen-windows-arm64.exe desk-tools-windows-amd64.tar.gz desk-tools-windows-arm64.tar.gz; do grep -qF "$a" .github/workflows/release.yml \|\| { echo "MISSING $a"; exit 1; }; done; echo OK` | `OK` |
| 4a | **Positive control for row 4** — a bogus asset name is absent: `grep -qF 'statusgen-windows-mips.exe' .github/workflows/release.yml; echo $?` | `1` |
| 5 | **Dereferencing — the windows binaries actually cross-compile** (proves the "no source change needed" claim, not just that YAML mentions them): `cd statusgen && GOOS=windows GOARCH=amd64 go build -o /tmp/wp01-sg-amd64.exe . && GOOS=windows GOARCH=arm64 go build -o /tmp/wp01-sg-arm64.exe . && file /tmp/wp01-sg-amd64.exe` | exit 0; `file` output contains `PE32` / `MS Windows` (a real Windows PE executable was produced) |
| 6 | **Dereferencing — desk-tools cross-compiles for windows too**: `cd tools/desk && GOOS=windows GOARCH=amd64 go build -o /tmp/wp01-dt2.exe ./cmd/deskpost && file /tmp/wp01-dt2.exe` | exit 0; `file` output contains `PE32`/`MS Windows` (a representative desk verb cross-compiles to windows) |
| 7 | Example pin file gained the two windows lines: `grep -cE -e '^statusgen-windows-amd64 ' -e '^statusgen-windows-arm64 ' examples/adopter-scaffold/.assay-versions` | `2` |
| 8 | **Consumers routing corroborated by the diff** (run on the implementer's branch): `statusgen --root . --consumers windows-port/01; echo $?` | `0` — every `consumers:` claim this brief makes is proved by the branch's own diff (release.yml + the example pin changed here; the follow-up edges reference briefs that cite 01) |

## Evidence
<!-- appended at implementation time: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" status in the stream README requires this section filled
     by someone who did NOT implement. -->

| # | Command | Exit | Output | Date | Runner |
|---|---------|------|--------|------|--------|

## Review
Gate: **model** (from frontmatter). All four risk answers are `no` — this edits a
cross-compile loop and a checksum list; everything is git-revertible CI config. The reviewer
confirms rows 5 and 6 (the binaries genuinely cross-compile — the dereferencing rows), and that
the four new asset names appear identically in the build, checksum, and (illustrative) pin
surfaces so brief 03/04/05 read a stable name.
