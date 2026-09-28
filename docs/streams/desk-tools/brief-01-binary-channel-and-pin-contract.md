---
brief: assay:assay:desk-tools:01
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
schema: brief-v2
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
version: 1
id: cc19398b-3c5a-40c2-b66c-8a2cfdc3f85d
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

### Verification — 2026-09-25 (assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian))

Non-implementer run against merged main 893cd6114b0382a1f71e6ef763c6601a5e19d270. Build step
`make desk-build` exit 0 (deskpins built into the desk dist directory); statusgen built from this
tree with `GOWORK=off go build`. Host: darwin arm64, load average 80–104 during the run (other
verifiers running concurrently). Gate is human: this block is Evidence only and the row stays
`implemented`.

**Execution witness** — `statusgen verifyrun --brief <this brief> --root . --dry-run`, rows verbatim
(exit 2 overall: 6 pass, 4 fail, 1 could-not-run):

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 0 | `n=$( { grep -rl 'assay-versions. pin file' docs/ README.md tools/desk/README.md 2>/dev/null; } \| xargs -r grep -l -e '^#\{1,3\} .*pin file' \| sort -u \| wc -l \| tr -d ' '); echo "spec-homes=$n"; [ "$n" -eq 1 ]` | pass exit=0 | sha256:475548b2fa90 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 1 | `SPEC=docs/distribution.md; test -f "$SPEC" && grep -q '^## The` | fail exit=2 | sha256:80dd39947a6e | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./... && go vet ./...` | could-not-run exit=- — timed out after 10m0s — no verdict was produced | sha256:a49dd834fd5c | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 3 | `mkdir -p /tmp/av && cp tools/desk/internal/deskkit/testdata/assay-versions-live.golden /tmp/av/.assay-versions && tools/desk/dist/deskpins --check --root /tmp/av` | pass exit=0 | sha256:1442e5a18b7d | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 3a | `mkdir -p /tmp/avd && cp /tmp/av/.assay-versions /tmp/avd/.assay-versions && grep -m1 '^statusgen ' /tmp/avd/.assay-versions > /tmp/dupline && cat /tmp/dupline >> /tmp/avd/.assay-versions && tools/desk/dist/deskpins --check --root /tmp/avd; rc=$?; echo "dup-rc=$rc"; [ "$rc" -ne 0 ]` | pass exit=0 | sha256:2da5a8d1ef91 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 4 | `mkdir -p /tmp/av && rm -f /tmp/av/.assay-versions; tools/desk/dist/deskpins --check --root /tmp/av; a=$?; printf 'statusgen only-a-tag\n' > /tmp/av/.assay-versions; tools/desk/dist/deskpins --check --root /tmp/av; b=$?; echo "absent=$a malformed=$b"; [ "$a" -ne 0 ] && [ "$b" -ne 0 ] && [ "$a" -ne "$b" ]` | pass exit=0 | sha256:1b9a88a1423a | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test ./cmd/deskboard/... -run TestNextup_PinFlowFromConfiguredRoot -v > /tmp/f.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/f.txt); f=$(grep -c '^--- FAIL' /tmp/f.txt); echo "pass=$p fail=$f rc=$rc"; [ "$p" -ge 1 ] && [ "$f" -eq 0 ] && [ "$rc" -eq 0 ]` | fail exit=0 | sha256:f87fed0a6e3a | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && go test ./cmd/deskboard/... -run TestStatusgenPin -v > /tmp/n.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/n.txt); f=$(grep -c '^--- FAIL' /tmp/n.txt); echo "pass=$p fail=$f rc=$rc"; [ "$p" -ge 1 ] && [ "$f" -eq 0 ] && [ "$rc" -eq 0 ]` | fail exit=0 | sha256:f87fed0a6e3a | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -q -e 'deskkit.ReleaseTag' -e 'deskkit.Tag' .github/workflows/release.yml && grep -q 'RELEASE_TAG' .github/workflows/release.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 8 | `cd tools/desk && go test ./internal/deskkit/... -run TestVersionStampedFromReleaseWorkflow -v > /tmp/v.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/v.txt); echo "pass=$p rc=$rc"; [ "$p" -ge 1 ] && [ "$rc" -eq 0 ]` | fail exit=1 | sha256:fe2a93fcf546 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |
| 9 | `n=$(grep -rn 'ConsumerRepo' tools/desk/cmd/deskboard/*.go \| grep -v _test \| grep -c 'hardcoded' \| tr -d ' '); echo "hardcoded=$n"; [ "$n" -eq 0 ]` | pass exit=0 | sha256:5ededa9fefc0 | 2026-09-25 | assay-verifier-app[bot] @ 893cd6114b03 (on-behalf-of human:ian) (forge-identity) |

**Hand-written rows** — every row where the witness verdict is not the row's verdict, re-run
directly (non-hermetic, same tree, same built binaries):

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 1 | the row 1 command exactly as the Verify table writes it, run under bash | exit 0, n ≥ 5 | exit 0, `n=15`. PASS. The witness ran a truncated command: the row's command holds an inline code span whose own backticks (around the pin-file name) cut the witness's cell parse at the first inner backtick, so it executed only up to `grep -q '^## The` (an unterminated quote, exit 2). Witness-tool defect, not implementation. | 2026-09-25 assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 2 | `cd tools/desk && go test ./... -count=1` then `go vet ./...` | exit 0 | go vet exit 0. go test exit 1: 83 packages ok, 2 FAIL: loopengine `TestDrain` ("max concurrency observed 1; the pool never sustained >1 in flight") and commsloop `TestRunDoesNotBusySpinOnEmptyQueue` ("loopengine.Run did not stop within deadline (possible wedge)"). Isolated re-run of those two, count=3: commsloop 3/3 PASS, TestDrain 1/3 PASS. This matches the open load-induced loopengine timing flake (#612) at load average ~104. Every package this brief touches (internal/deskkit, cmd/deskboard, cmd/deskinstall, cmd/deskversion) is ok. FAIL observed, attributed to host load (#612), not to this brief's code. The witness's own run of this row hit its 10m limit. | 2026-09-25 assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 5 | the row 5 command as written | exit 0, pass ≥ 1, fail=0 | exit 0, `pass=1 fail=0 rc=0`, `--- PASS: TestNextup_PinFlowFromConfiguredRoot`. PASS. The witness's output hash f87fed0a6e3a is the sha256 of exactly that line; its count reader takes the LAST integer on the line (the `rc=0`), not the `pass` count. Witness-tool defect. | 2026-09-25 assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 6 | the row 6 command as written | exit 0, pass ≥ 1, fail=0 | exit 0, `pass=1 fail=0 rc=0`, `--- PASS: TestStatusgenPin`. PASS. Same witness count-reader defect as row 5 (same output hash). | 2026-09-25 assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |
| 8 | the row 8 command as written | exit 0, pass ≥ 1 | exit 1, `pass=0 rc=0`. The test SKIPs: "fixture ../../../../.github/workflows/release-desk.yml not present in this tree". FAIL, real. The test reads a release-desk.yml workflow that does not exist in this repository; the desk-tools stamp lives in release.yml (row 7 green; the ReleaseTag ldflag is at release.yml line 1137). The guard therefore cannot fail if the stamp is removed: release.yml's comment that this test "reddens if this stamp is dropped from release.yml" is false, and the CI-trigger registry in internal/deskkit citrigger_test.go also lists release-desk.yml as the file it reads. Implementation defect (Task 5). | 2026-09-25 assay-verifier-app[bot] @ 893cd6114b03 (claude-opus-5-5[1m]) (on-behalf-of human:ian) |

Row totals with the hand-written re-runs: 9 of 11 pass (0, 1, 3, 3a, 4, 5, 6, 7, 9); row 8 fails on
the implementation; row 2 fails on host load (#612).

**Observations for the human gate** (outside the Verify table; each one was checked directly):

- **Task 1 is thinner than the brief specifies.** The pin-file section of docs/distribution.md is
  45 lines. It carries no worked example taken from a live consumer file, does not name the
  `daily-harvest` lines, gives no grammar for whole-line `#` comments, does not say why the trailing
  space matters beyond one example, and does not spell out the three read states (read / malformed /
  absent). It says only that a missing or malformed pin is fail-closed. Row 1 counts matches across
  the whole file, so it passes either way. The human who confirms "the published grammar" is
  confirming this text.
- **The validator and the reader disagree on separators.** Probe: a tab-separated line
  (`statusgen<TAB>v0.1.0<TAB><64-hex>`) is `checked-clean` under CheckPins, while StatusgenPin on
  the same file returns "no statusgen pin". The reader selects on artifact name plus a literal
  space, and the validator splits on any whitespace. The disagreement fails closed, so it is safe,
  but deskpins passes a file that the desk then refuses.
- **The namespace is shape-checked, not bound to the artifact.** Probe:
  `statusgen daily-harvest/v0.1.0 <64-hex>` is `checked-clean`. See the tag-grammar RISK-VALUE line.
- **Task 3 landed as discovery, not configuration.** deskboard reads the pin from the first
  configured root (in sorted repo order) that carries a pin file, and a present-but-bad pin fails
  closed. The brief asked for a configured root that defaults to the previous repo. Behaviour for the
  previous single-consumer setup is unchanged (row 5).
- deskpins has no test files of its own. Its exit-code mapping is exercised only by rows 3, 3a and 4.

**Risk-bearing values.** Enumeration covered the brief's Deliverables and the code they landed:
tools/desk/internal/deskkit/pins.go, tools/desk/cmd/deskpins/main.go,
tools/desk/internal/deskkit/version.go, the desk-tools build step of release.yml, and
tools/desk/cmd/deskboard nextup.go and main.go. Every literal found:
AssayVersionsFile = ".assay-versions" @ pins.go:33 (pre-existing);
UmbrellaArtifact = "assay" @ pins.go:40; artifactTagPattern @ pins.go:70;
sha256Pattern = `^[0-9a-f]{64}$` @ pins.go:77; commitPattern = `^[0-9a-f]{40}$` @ pins.go:78;
selection prefix = artifact + " " @ pins.go:133; umbrella field count = 2 @ pins.go:553;
artifact field count = 3 (fewer @ pins.go:566, more @ pins.go:571, both malformed);
SourcePinSuffix = "-source" @ pins.go:237; deskpins exits 0/1/2/3 @ cmd/deskpins/main.go:45-48;
devRelease = "dev" @ version.go:36; ReleaseTag = "" @ version.go:28;
IsPinned = SourceSHA != "" && BuiltAt != "" @ version.go:66;
ldflag `-X …/deskkit.ReleaseTag=${RELEASE_TAG}` @ release.yml:1137;
test fixture path "release-desk.yml" @ version_test.go:88 (the row 8 defect);
ExitUnverifiable = 6 @ exitcodes.go:31 (pre-existing).

Ranking: the brief marks nothing irreversible (`irreversible: no`). The top-ranked values are the
published-grammar literals (`customer: yes`), because a wrong one costs a migration of every
consumer written against it. The "dev" stamp, IsPinned, and the internal fixture path are
reversible by an edit and rank last.

- RISK-VALUE: DERIVED — sha256Pattern = `^[0-9a-f]{64}$` @ tools/desk/internal/deskkit/pins.go:77 — SHA-256 is a 256-bit digest, which is 64 hex characters. Lowercase is what `sha256sum`, `shasum -a 256` and the release checksum files print, so a correctly copied digest always matches, and a truncated or uppercase-mangled one is refused.
- RISK-VALUE: DERIVED — commitPattern = `^[0-9a-f]{40}$` @ tools/desk/internal/deskkit/pins.go:78 — a git SHA-1 object id is 160 bits (40 hex). Requiring the full id is what makes a `-source` pin immutable: an abbreviation could become ambiguous, and a tag can be re-pointed.
- RISK-VALUE: DERIVED — selection prefix = artifact + " " @ tools/desk/internal/deskkit/pins.go:133 — `desk-tools` is a string prefix of `desk-tools-linux-amd64`, so only a terminator after field 1 keeps the bare and per-platform lines apart. The same byte is the one consumer CI greps (`^statusgen `). The disagreement noted above (space vs. any whitespace) is a separate gap in the validator, not in this value.
- RISK-VALUE: DERIVED — field counts: artifact line = 3 @ pins.go:566/571, umbrella line = 2 @ pins.go:553 — the grammar is `<artifact> <tag> <sha256>` (three fields). The umbrella names a composition, not a downloadable asset, so there is no digest to carry.
- RISK-VALUE: NAMED, NOT DERIVED — artifactTagPattern = `^([a-z][a-z0-9-]*/)?v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$` @ tools/desk/internal/deskkit/pins.go:70 — Task 4 says the validator must reject "a tag outside the namespace grammar". This pattern accepts ANY lowercase component prefix, including one that names a different artifact (`statusgen daily-harvest/v0.1.0 …` is checked-clean), and it also admits a prefixed tag on the umbrella line. Nothing in the brief or the spec settles whether the prefix must be absent or equal the line's own artifact, and that is a grammar choice this brief's gate-why reserves to the human. **Open question: should a prefixed tag be required to match its line's artifact name (or its artifact family, for per-platform lines)?**
- RISK-VALUE: NAMED, NOT DERIVED — UmbrellaArtifact = "assay" with a plain `vX.Y.Z` umbrella tag @ tools/desk/internal/deskkit/pins.go:40 (tag rule at pins.go:70) — the brief describes the umbrella as the `assay/vX.Y.Z` tag. The spec and code publish `assay <vX.Y.Z>` and cite a 2026-08-15 plain-tag ruling in a code comment. The umbrella line's meaning is on the gate-why list for human confirmation, and a model cannot derive it. **Open question: is the published umbrella form `assay <vX.Y.Z>` (plain tag) the intended contract?**
- RISK-VALUE: NAMED, NOT DERIVED — deskpins exit codes 0 clean / 1 checked-failed / 2 could-not-check / 3 malformed @ tools/desk/cmd/deskpins/main.go:45-48 — row 4 proves they are distinct, and adopter CI will key on them. However, the spec section does not publish them, and the same absent/malformed states exit 6 (ExitUnverifiable) everywhere else in the desk. No derivation says why the validator's codes differ from the reader's. **Open question: are these codes part of the published contract, and should they appear in the spec?**

VERIFY: FAIL — row 8: TestVersionStampedFromReleaseWorkflow SKIPs on an absent release-desk.yml, so
the Task 5 stamp guard is not load-bearing. Row 2 also fails on the known load flake (#612). Rows 1,
5 and 6 are false FAILs from the witness tool and pass when re-run directly.

## Review
Gate: **human** (from frontmatter; see `gate-why`). The human signs off the published grammar —
field order, the `<artifact>-<platform>` convention, the trailing-space selection rule, and the
umbrella line's meaning — plus the choice that an unreadable pin fails closed rather than
defaulting. A human gate needs a `human:<name>` entry, not a model sign-off.
