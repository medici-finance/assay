---
brief: desk-tools/01
title: Binary channel sealed — publish the `.assay-versions` contract, validate it, stamp desk-tools with its release tag
wave: 1
depends: []
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: yes, irreversible: no, sensitive-data: no}
gate-why: >-
  `customer: yes` — this brief publishes an adopter-facing contract: the `.assay-versions`
  grammar becomes the thing every future adopter's CI is written against, and it changes the
  reader that the reference consumer's CI already fails closed on. A published format is cheap
  to get right once and expensive to change afterwards, because every consumer written against
  v1 has to be migrated. The human confirms the published grammar (field order, the
  `<artifact>-<platform>` naming convention, and what the optional umbrella line means) and
  that the fail-closed three-state read is the intended adopter experience rather than a
  defaultable convenience. `irreversible: no` — the contract is a document and a validator,
  both revisable; this brief cuts no tag and publishes no release.
issues: []
schema: brief-v1
authored: 2026-08-02 by Opus session (author-brief); re-homed to the desk-tools board 2026-08-26
sources:
  - "The pinned-release binary channel is the channel the reference consumer actually uses, yet no adopter-facing doc described it and no code validated it: the format existed only as a `strings.Fields` read inside one desk internal, and the release workflow computed the desk-tools tag and never used it."
  - "The pin-file specification section in `docs/distribution.md § The `.assay-versions` pin file` (grammar, rules, per-platform naming, refuse-if-absent) — this brief EXTENDS that single section in place and must never publish a second one."
exec-tier: strong
exec-tier-why: >-
  Correctness depends on cross-component reasoning — the pin file is read by the desk, written
  by a different repo's CI, and must stay backward-compatible with a live consumer while
  gaining an umbrella line; a format change that is right here and wrong there breaks the
  reference consumer's board with a fail-closed exit 6.
why: >-
  The channel the reference consumer actually uses — release binaries pinned by tag + sha256 in
  `.assay-versions` — was the one channel no adopter-facing doc described and no code validated.
  An unpublished, unvalidated contract is not a channel; it is a convention one repo happens to
  follow. This brief publishes the grammar, adds a validator, generalises the reader off its one
  hardcoded consumer, and stamps desk-tools binaries with their release tag so a running binary
  can report which `desk-tools/vX.Y.Z` it is.
---

# Brief 01 — Binary channel sealed: the `.assay-versions` contract, published and validated

## Dependencies
The version-scheme brief this originally depended on (the umbrella `assay/vX.Y.Z` tag over
per-artifact lines) has landed outside this stream, so no typed `depends:` edge remains. This
brief must be able to express the umbrella pin-file line the version scheme defines.

## Context
files: the pin-file spec at its single, settled home — `docs/distribution.md § The
`.assay-versions` pin file` (extended in place, never duplicated),
`tools/desk/internal/deskkit/roots.go`, `tools/desk/cmd/deskboard/nextup.go`,
`tools/desk/README.md`, `.github/workflows/release.yml`.

facts:
- **The current format:** whitespace-separated lines, `<artifact> <tag> <sha256>`, with an
  optional trailing `# …` comment after the third field. The live file uses one on nearly
  every line and is comment-heavy — a validator that chokes on, reorders, or strips whole-line
  `#` comments breaks the file's purpose.
- **Two naming conventions side by side:** a bare `<artifact>` line and `<artifact>-<platform>`
  lines (`statusgen-linux-amd64`, `desk-tools-darwin-arm64`, …). The live file carries
  `statusgen` + 3 per-platform, `daily-harvest` + 3 per-platform, and `desk-tools` per-platform
  ONLY — there is no bare `desk-tools` line. Any rule that assumes a bare line exists for every
  artifact is wrong.
- **`statusgen` and `statusgen-linux-amd64` legitimately carry the identical tag and identical
  sha256.** They are distinct artifact names, not a duplicate line. A duplicate rule must key on
  the artifact **name**, never on the (tag, hash) pair.
- **Selection is a prefix match with a load-bearing trailing space.** The parser is
  `strings.HasPrefix(line, "statusgen ")` and consumer CI uses `grep '^statusgen '`. That
  trailing space is *why* the bare line never matches `statusgen-<platform>` — it is the
  disambiguator, not incidental formatting.
- **Three read states, fail-closed.** Fewer than 3 fields is *malformed*; no matching line is
  *absent*. Both return `Unverifiable` (exit 6). A consumer that cannot read its pin cannot
  claim to be pinned — a default is never an acceptable substitute for an unreadable pin.
- **The reader hardcodes a single consumer**, which is what makes the contract un-adoptable by
  anyone else: publishing the format without removing the hardcoding publishes a contract only
  one repo can satisfy.
- **`release.yml` computes the tag and drops it.** The build step sets `TAG="$RELEASE_TAG"`
  then builds LDFLAGS from `SourceSHA`/`BuiltAt` only, so `desk-tools --version` reports a commit
  SHA and nothing maps a running binary back to `desk-tools/vX.Y.Z`. `statusgen` does this
  correctly (`-X main.statusgenVersion=$RELEASE_TAG`) — mirror it.
- **Backward compatibility is mandatory and additive.** The live consumer's `.assay-versions`
  is written by another repo and read by CI on every run; existing lines keep parsing, and a
  file with no umbrella line stays valid (a distinct third state — "no umbrella pin", not an
  error, not a default).

## Ground rules
- NEVER git push to `main` / trigger a workflow / cut a tag / run mutating infra commands.
- `.github/workflows/` needs the `workflows` OAuth scope. If your identity's push is rejected
  for scope, that is a STOP: land the non-workflow half, record the workflow half as an unlanded
  deliverable in the PR body, and hand it to an identity that holds the scope. Never re-push
  under a different identity to route around a scope refusal.
- Do not weaken the fail-closed behaviour of the pin reader. Three states only: pin read / pin
  malformed / pin absent — never a default.
- Stop at `implemented` — you do not set verified/done.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Extend `docs/distribution.md § The `.assay-versions` pin file` in place with the exact
   grammar (the optional trailing `# …` comment and the whole-line `#` comments), the artifact
   line names in use (`statusgen`, `desk-tools`, `daily-harvest`, the `<artifact>-<platform>`
   variants, plus the umbrella line), the trailing-space prefix-match selection rule and why it
   is load-bearing, the required fields, the three read states, the fail-closed rule and why,
   and a worked example taken from the live consumer file, not synthesised. Do NOT create a
   second pin-spec document — one published home only.
2. Generalise the desk's `StatusgenPin` reader into a pin reader that returns any artifact's
   line, keeping `StatusgenPin` as a thin wrapper so no caller changes behaviour. Add the
   umbrella line as a readable, optional entry. The trailing-space prefix semantics must
   survive: a lookup for `desk-tools` must not match `desk-tools-linux-amd64`, and vice versa.
3. Remove `deskboard`'s hardcoded single consumer: the root whose `.assay-versions` is read
   comes from configuration, defaulting to the same repo it uses today so current behaviour is
   unchanged.
3a. Check in a golden fixture — a neutralized copy of the live consumer, artifact lines
   byte-identical — at `tools/desk/internal/deskkit/testdata/assay-versions-live.golden`, with a
   header comment naming only a SHA (no repo slug). Every backward-compat assertion runs against
   that file, not a hand-written one-liner: it must carry the comment lines, the per-platform
   naming, the trailing comments, and the no-bare-`desk-tools` case.
4. Add a validator (a `--check-pins` mode or a small tool) that reads a pin file and reports
   checked-clean / checked-failed / could-not-check against the spec. It must reject a missing
   sha256, a malformed tag, a tag outside the namespace grammar, and a duplicate artifact name —
   keying the duplicate rule on the artifact name in field 1 and nothing else.
5. Stamp desk-tools with its release tag in `release.yml`, mirroring statusgen's
   `-X main.statusgenVersion`. Expose it on `--version` alongside SourceSHA/BuiltAt, and add a
   new named test (e.g. `TestVersionStampedFromReleaseWorkflow`) that reads `release.yml`
   and fails if the stamp is removed. `IsPinned()` must keep returning true for exactly the same
   builds it does today.
6. Reconcile any stale prose in `tools/desk/README.md` that still describes a deleted
   frozen-copy layout, and repoint the dead link (`linkcheck` walks `docs/**` only, so it cannot
   see this one — say so in the file).

## Verify (executable — no prose-only DoD items)

**Build step, run once before the code rows**: `make desk-build` — builds every
`tools/desk/cmd/*` unprivileged into `tools/desk/dist/`. Rows that call `tools/desk/dist/deskpins`
will report "no such file or directory" (a failed row, not a placeholder) if this step is skipped.

| # | Command | Expect |
|---|---------|--------|
| 0 | single-spec: `n=$( { grep -rl 'assay-versions. pin file' docs/ README.md tools/desk/README.md 2>/dev/null; } \| xargs -r grep -l -e '^#\{1,3\} .*pin file' \| sort -u \| wc -l \| tr -d ' '); echo "spec-homes=$n"; [ "$n" -eq 1 ]` | exit 0, `spec-homes=1` — one file carries the spec heading; every other mention links to it |
| 1 | `SPEC=docs/distribution.md; test -f "$SPEC" && grep -q '^## The `.assay-versions` pin file' "$SPEC" && n=$(grep -c -e statusgen -e desk-tools -e daily-harvest -e sha256 -e could-not-check "$SPEC"); echo "n=$n"; [ "$n" -ge 5 ]` | exit 0, `n` ≥ 5 — the spec section exists and names every artifact line and the three states |
| 2 | `cd tools/desk && go test ./... && go vet ./...` | exit 0 |
| 3 | backward-compat against the golden fixture: `mkdir -p /tmp/av && cp tools/desk/internal/deskkit/testdata/assay-versions-live.golden /tmp/av/.assay-versions && tools/desk/dist/deskpins --check --root /tmp/av` | exit 0; checked-clean; the per-platform lines and comment lines parse; no bare `desk-tools` line is required |
| 3a | not-a-duplicate: `mkdir -p /tmp/avd && cp /tmp/av/.assay-versions /tmp/avd/.assay-versions && grep -m1 '^statusgen ' /tmp/avd/.assay-versions > /tmp/dupline && cat /tmp/dupline >> /tmp/avd/.assay-versions && tools/desk/dist/deskpins --check --root /tmp/avd; rc=$?; echo "dup-rc=$rc"; [ "$rc" -ne 0 ]` | exit 0, non-zero `dup-rc` — the identical-tag/sha `statusgen`+`statusgen-linux-amd64` pair stays clean; a real second `statusgen ` line is rejected |
| 4 | fail-closed: `mkdir -p /tmp/av && rm -f /tmp/av/.assay-versions; tools/desk/dist/deskpins --check --root /tmp/av; a=$?; printf 'statusgen only-a-tag\n' > /tmp/av/.assay-versions; tools/desk/dist/deskpins --check --root /tmp/av; b=$?; echo "absent=$a malformed=$b"; [ "$a" -ne 0 ] && [ "$b" -ne 0 ] && [ "$a" -ne "$b" ]` | exit 0 — absent and malformed both non-zero and distinct from each other; never a default |
| 5 | flow: `cd tools/desk && go test ./cmd/deskboard/... -run TestNextup_PinFlowFromConfiguredRoot -v > /tmp/f.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/f.txt); f=$(grep -c '^--- FAIL' /tmp/f.txt); echo "pass=$p fail=$f rc=$rc"; [ "$p" -ge 1 ] && [ "$f" -eq 0 ] && [ "$rc" -eq 0 ]` | exit 0, `pass` ≥ 1, `fail=0` — reading the pin, resolving the binary, and rendering the board still work as one chain |
| 6 | neighbour: `cd tools/desk && go test ./cmd/deskboard/... -run TestStatusgenPin -v > /tmp/n.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/n.txt); f=$(grep -c '^--- FAIL' /tmp/n.txt); echo "pass=$p fail=$f rc=$rc"; [ "$p" -ge 1 ] && [ "$f" -eq 0 ] && [ "$rc" -eq 0 ]` | exit 0, `pass` ≥ 1, `fail=0` — the pre-existing pin reader still parses a good pin and refuses every unreadable one |
| 7 | `grep -q -e 'deskkit.ReleaseTag' -e 'deskkit.Tag' .github/workflows/release.yml && grep -q 'RELEASE_TAG' .github/workflows/release.yml` | exit 0 — the computed tag is stamped, not assigned and discarded |
| 8 | `cd tools/desk && go test ./internal/deskkit/... -run TestVersionStampedFromReleaseWorkflow -v > /tmp/v.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/v.txt); echo "pass=$p rc=$rc"; [ "$p" -ge 1 ] && [ "$rc" -eq 0 ]` | exit 0, `pass` ≥ 1 — the new workflow-assertion test exists and passes |
| 9 | `n=$(grep -rn 'ConsumerRepo' tools/desk/cmd/deskboard/*.go \| grep -v _test \| grep -c 'hardcoded' \| tr -d ' '); echo "hardcoded=$n"; [ "$n" -eq 0 ]` | exit 0, `hardcoded=0` — the single-consumer hardcoding is gone from non-test source |

**On `tools/desk/dist/deskpins`**: the chosen name for the Task 4 validator, built by
`make desk-build` into the desk binary directory. An implementer may pick a different name or a
`deskboard --check-pins` mode but must then update rows 3, 3a and 4 in the same commit.

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item —
     (command, exit code, output line(s) or hash, date, runner).
     "verified" requires this section filled by someone who did NOT implement. -->

## Review
Gate: **human** (from frontmatter; see `gate-why`). The human signs off the published grammar —
field order, the `<artifact>-<platform>` convention, the trailing-space selection rule, and the
umbrella line's meaning — plus the choice that an unreadable pin fails closed rather than
defaulting. A human gate needs a `human:<name>` entry, not a model sign-off.
