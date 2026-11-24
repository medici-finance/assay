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
files:
- `docs/distribution.md` — the pin-file spec at its single, settled home: § The `.assay-versions` pin file (extended in place, never duplicated),
- `tools/desk/internal/deskkit/roots.go`, `tools/desk/cmd/deskboard/nextup.go`,
- `tools/desk/README.md`, `.github/workflows/release.yml`.

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

**Row-shape note (2026-09-27, #1546).** Rows 1, 5, 6 and 8 are re-shaped; no Expect changes and
no row's own shell assertion changes. Row 8's check was real and failing: its test read a
release workflow this repository does not carry and skipped on every run (`pass=0`), so it now
reads `.github/workflows/release.yml`, where the desk-tools build step stamps
`deskkit.ReleaseTag`. The other three were check-shape defects the execution witness could not
score, while each row's intended check passed by hand. Row 1's command held a literal backtick
inside a single-backtick span, which the witness truncated (exit 2). The backtick now comes from
`printf '\140'` (octal for a backtick), so the grep pattern matches the same heading byte for byte
and the span holds no backtick. Rows 5, 6 and 8 printed `rc` last, and the witness reads the last
integer on the last line as the `≥` count, so the echo order now puts `pass` last. Each row's
`[ … ]` assertion is unchanged.

**Build step, run once before the code rows**: `make desk-build` — builds every
`tools/desk/cmd/*` unprivileged into `tools/desk/dist/`. Rows that call `tools/desk/dist/deskpins`
will report "no such file or directory" (a failed row, not a placeholder) if this step is skipped.

| # | Command | Expect |
|---|---------|--------|
| 0 | single-spec: `n=$( { grep -rl 'assay-versions. pin file' docs/ README.md tools/desk/README.md 2>/dev/null; } \| xargs -r grep -l -e '^#\{1,3\} .*pin file' \| sort -u \| wc -l \| tr -d ' '); echo "spec-homes=$n"; [ "$n" -eq 1 ]` | exit 0, `spec-homes=1` — one file carries the spec heading; every other mention links to it |
| 1 | `SPEC=docs/distribution.md; BT=$(printf '\140'); test -f "$SPEC" && grep -q "^## The ${BT}.assay-versions${BT} pin file" "$SPEC" && n=$(grep -c -e statusgen -e desk-tools -e daily-harvest -e sha256 -e could-not-check "$SPEC"); echo "n=$n"; [ "$n" -ge 5 ]` | exit 0, `n` ≥ 5 — the spec section exists and names every artifact line and the three states |
| 2 | `cd tools/desk && go test ./... && go vet ./...` | exit 0 |
| 3 | backward-compat against the golden fixture: `mkdir -p /tmp/av && cp tools/desk/internal/deskkit/testdata/assay-versions-live.golden /tmp/av/.assay-versions && tools/desk/dist/deskpins --check --root /tmp/av` | exit 0; checked-clean; the per-platform lines and comment lines parse; no bare `desk-tools` line is required |
| 3a | not-a-duplicate: `mkdir -p /tmp/avd && cp /tmp/av/.assay-versions /tmp/avd/.assay-versions && grep -m1 '^statusgen ' /tmp/avd/.assay-versions > /tmp/dupline && cat /tmp/dupline >> /tmp/avd/.assay-versions && tools/desk/dist/deskpins --check --root /tmp/avd; rc=$?; echo "dup-rc=$rc"; [ "$rc" -ne 0 ]` | exit 0, non-zero `dup-rc` — the identical-tag/sha `statusgen`+`statusgen-linux-amd64` pair stays clean; a real second `statusgen ` line is rejected |
| 4 | fail-closed: `mkdir -p /tmp/av && rm -f /tmp/av/.assay-versions; tools/desk/dist/deskpins --check --root /tmp/av; a=$?; printf 'statusgen only-a-tag\n' > /tmp/av/.assay-versions; tools/desk/dist/deskpins --check --root /tmp/av; b=$?; echo "absent=$a malformed=$b"; [ "$a" -ne 0 ] && [ "$b" -ne 0 ] && [ "$a" -ne "$b" ]` | exit 0 — absent and malformed both non-zero and distinct from each other; never a default |
| 5 | flow: `cd tools/desk && go test ./cmd/deskboard/... -run TestNextup_PinFlowFromConfiguredRoot -v > /tmp/f.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/f.txt); f=$(grep -c '^--- FAIL' /tmp/f.txt); echo "fail=$f rc=$rc pass=$p"; [ "$p" -ge 1 ] && [ "$f" -eq 0 ] && [ "$rc" -eq 0 ]` | exit 0, `pass` ≥ 1, `fail=0` — reading the pin, resolving the binary, and rendering the board still work as one chain |
| 6 | neighbour: `cd tools/desk && go test ./cmd/deskboard/... -run TestStatusgenPin -v > /tmp/n.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/n.txt); f=$(grep -c '^--- FAIL' /tmp/n.txt); echo "fail=$f rc=$rc pass=$p"; [ "$p" -ge 1 ] && [ "$f" -eq 0 ] && [ "$rc" -eq 0 ]` | exit 0, `pass` ≥ 1, `fail=0` — the pre-existing pin reader still parses a good pin and refuses every unreadable one |
| 7 | `grep -q -e 'deskkit.ReleaseTag' -e 'deskkit.Tag' .github/workflows/release.yml && grep -q 'RELEASE_TAG' .github/workflows/release.yml` | exit 0 — the computed tag is stamped, not assigned and discarded |
| 8 | `cd tools/desk && go test ./internal/deskkit/... -run TestVersionStampedFromReleaseWorkflow -v > /tmp/v.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/v.txt); echo "rc=$rc pass=$p"; [ "$p" -ge 1 ] && [ "$rc" -eq 0 ]` | exit 0, `pass` ≥ 1 — the new workflow-assertion test exists and passes |
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

### Verification — 2026-10-16 (assay-verifier-app[bot] @ c3db7cca008e (claude-opus-5-5 verifier))

Verification-Attestation: medici-finance/assay#3162 run=edc01847dd6790e388d18207b988c54d549ea1f84eb67649b85dd51343cdd694 source=c3db7cca008e9dc17b3d0e7462c4a366706f699a brief=docs/streams/desk-tools/brief-01-binary-channel-and-pin-contract.md model=claude-opus-5-5 tier=any

Fresh non-implementer run of every row against merged main c3db7cca008e9dc17b3d0e7462c4a366706f699a,
which carries the row-8 fix 320a8b744 (medici-finance/assay#1819) and the #1546 row re-shape. Admission
`deskdispatch --check-verifier` exit 0 before the first row. Build step `make desk-build` exit 0
(deskpins built into the desk dist directory). Every row ran under bash with `env -i`: a throwaway
HOME and a short TMPDIR outside the worktree, GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_GLOBAL set to an
empty file there, KUBECONFIG=/dev/null, no DESK_/ASSAY_ variables, and the real GOCACHE/GOMODCACHE passed
explicitly. The grep on PATH was BSD grep 2.6.0. Host: darwin arm64, go1.27.2, git 2.56.0, load
average 7 to 49 during the run. Gate is human: this block is Evidence only and the row stays where it is.
No statusgen verifyrun witness was run; every row below is hand-run.

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 0 | the row 0 command exactly as the Verify table writes it | exit 0, spec-homes=1 | exit 0, `spec-homes=1` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 1 | the row 1 command exactly as the Verify table writes it | exit 0, n ≥ 5 | exit 0, `n=16` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | `cd tools/desk && go test ./... && go vet ./...` | exit 0 | exit 0 on the third run (go test 112 ok, 0 FAIL, then go vet exit 0). The first two runs failed, and each failure is classified here. Run 1, in the environment named above: exit 1, 3 packages FAIL (cmd/deskwt, cmd/deskpushguard, internal/deskkit), 404 FAIL lines, every one a git call refused with "update aborted by the reference-transaction hook". Cause, ENVIRONMENT: deskwt's TestMaintenanceHookPolicy (global-path and configured-global subtests) runs `git config --global`. Its fixture swaps HOME but not GIT_CONFIG_GLOBAL, so with GIT_CONFIG_GLOBAL pointing at a file, the test wrote core.hooksPath plus a reference-transaction hook into that one shared file. Every git call in packages running at the same time then ran a hook from a deleted temp dir. Reproduced alone: that one test turns an empty GIT_CONFIG_GLOBAL file into 280 bytes of hook config. With GIT_CONFIG_GLOBAL unset under a throwaway HOME, the same test passes and leaves no global config. That deskwt test is outside this brief. Runs 2 and 3 used GIT_CONFIG_GLOBAL unset with a fresh throwaway HOME and TMPDIR. Run 2: exit 1, 111 ok, 1 FAIL, internal/regression TestEvidenceAutomergeRefusal case G1 ("class guard on the real tree", an UNREDUCED flag on the evidence-automerge poll test). Cause, CODE, outside this brief: that file does call latest_run_per_check at its line 439, and the guard script runs under `set -o pipefail` with a `grep -v … \| grep -q` pipeline, which can return a SIGPIPE status when the reader exits early. Not reproduced in 3 direct reruns of the suite (25/25 each) or 500 isolated pipeline iterations at load ~12; run 2 started at load ~49. Run 3: exit 0, with 111 of the 112 ok packages served from the go test cache filled by run 2 in the same environment and internal/regression re-executed. Every package this brief touches (internal/deskkit, cmd/deskboard, cmd/deskpins with no test files) is ok in runs 2 and 3. | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | the row 3 command exactly as the Verify table writes it | exit 0, checked-clean | exit 0, `checked-clean: 12 artifact line(s), 80 comment line(s)`. The golden fixture has the statusgen, daily-harvest and desk-tools per-platform lines, a desk-tools-source line, and no bare desk-tools line | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3a | the row 3a command exactly as the Verify table writes it | exit 0, non-zero dup-rc | exit 0, `dup-rc=1`, `line 99: duplicate artifact "statusgen" (first seen line 46)`. The identical-tag/sha statusgen and statusgen-linux-amd64 pair stays clean in row 3 | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | the row 4 command exactly as the Verify table writes it | exit 0, absent and malformed non-zero and distinct | exit 0, `absent=2 malformed=3`; the malformed line reports `"statusgen" has 2 field(s) … (missing sha256)` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | the row 5 command exactly as the Verify table writes it | exit 0, pass ≥ 1, fail=0 | exit 0, `fail=0 rc=0 pass=1`, `--- PASS: TestNextup_PinFlowFromConfiguredRoot` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | the row 6 command exactly as the Verify table writes it | exit 0, pass ≥ 1, fail=0 | exit 0, `fail=0 rc=0 pass=1`, `--- PASS: TestStatusgenPin` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 7 | the row 7 command exactly as the Verify table writes it | exit 0 | exit 0. The release workflow's desk-tools build step carries the deskkit.ReleaseTag=${RELEASE_TAG} ldflag at release.yml line 1209 | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 8 | the row 8 command exactly as the Verify table writes it | exit 0, pass ≥ 1 | exit 0, `rc=0 pass=1`, `--- PASS: TestVersionStampedFromReleaseWorkflow`, with no skip. Independent mutation check: in a git-archive copy of this commit outside the home, with the ReleaseTag ldflag deleted from the copy's release.yml, the same test FAILs ("the desk-tools build step's LDFLAGS assignment does not carry -X …deskkit.ReleaseTag=${RELEASE_TAG}"), so the guard now depends on the stamp | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 9 | the row 9 command exactly as the Verify table writes it | exit 0, hardcoded=0 | exit 0, `hardcoded=0` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

Row totals: 11 of 11 pass. Row 8, the 2026-09-25 implementation failure, now passes and depends on
the stamp. Rows 1, 5 and 6 pass as re-shaped.

**Observations for the human gate** (outside the Verify table; re-probed against this commit with the
built deskpins):

- The pin-file section of docs/distribution.md is still about 48 lines. It still gives no worked
  example from a live consumer file, does not name the daily-harvest lines, and does not spell out the
  read / malformed / absent states by name.
- A tab-separated line (statusgen, TAB, tag, TAB, 64-hex) is still checked-clean under deskpins, while
  the reader selects on artifact name plus a literal space. The disagreement fails closed.
- `statusgen daily-harvest/v0.1.0 <64-hex>` is still checked-clean: the namespace prefix is
  shape-checked, not bound to the line's artifact.

**Risk-bearing values.** Enumeration covered the brief's Deliverables and the code they landed at this
commit (tools/desk/internal/deskkit pins.go and version.go, tools/desk/cmd/deskpins main.go, the
desk-tools build step of release.yml, the deskboard pin flow) plus the 320a8b744 fix (version.go,
version_test.go, fixture_skip_test.go, citrigger_test.go). Literals: AssayVersionsFile = ".assay-versions"
@ pins.go:33; UmbrellaArtifact = "assay" @ pins.go:40; artifactTagPattern @ pins.go:70;
sha256Pattern = `^[0-9a-f]{64}$` @ pins.go:77; commitPattern = `^[0-9a-f]{40}$` @ pins.go:78;
selection prefix = artifact + " " @ pins.go:133; umbrella field count = 2 @ pins.go:553; artifact field
count = 3 @ pins.go:566 and 571; SourcePinSuffix = "-source" @ pins.go:237; deskpins exits 0/1/2/3 @
cmd/deskpins/main.go:45-48; ReleaseTag = "" @ version.go:28; devRelease = "dev" @ version.go:36;
IsPinned = SourceSHA != "" && BuiltAt != "" @ version.go:66; ReleaseTag ldflag @ release.yml:1209;
releaseWorkflowPath = .github/workflows/release.yml @ version_test.go:79; deskToolsBuildStepName =
"Build and package desk-tools binaries" @ version_test.go:84; ExitUnverifiable = 6 @ exitcodes.go:55.
Ranking: nothing is marked irreversible. The published-grammar literals rank top (customer: yes; a wrong
one means migrating every consumer). The test-path constants, "dev", IsPinned and the exit code
constants are reversible by an edit and rank last.

- RISK-VALUE: DERIVED — sha256Pattern = `^[0-9a-f]{64}$` @ tools/desk/internal/deskkit/pins.go:77 — a SHA-256 digest is 256 bits, which is 64 hex digits. sha256sum, shasum -a 256 and release checksums.txt print lowercase, so a faithfully copied digest matches and a truncated or case-mangled one is refused.
- RISK-VALUE: DERIVED — commitPattern = `^[0-9a-f]{40}$` @ tools/desk/internal/deskkit/pins.go:78 — a git SHA-1 object id is 160 bits, which is 40 hex digits. Requiring the full id keeps a -source pin immutable and unambiguous, where an abbreviated id or a tag could later resolve to something else.
- RISK-VALUE: DERIVED — selection prefix = artifact + " " @ tools/desk/internal/deskkit/pins.go:133 — "desk-tools" is a string prefix of "desk-tools-linux-amd64", so only a terminator byte after field 1 separates the bare and per-platform lines, and it is the same byte consumer CI greps for (^statusgen followed by a space).
- RISK-VALUE: DERIVED — artifact line field count = 3 @ tools/desk/internal/deskkit/pins.go:566/571, umbrella field count = 2 @ pins.go:553 — the grammar is artifact, tag, sha256. The umbrella line names a composition, not a downloadable asset, so it has no digest to carry.
- RISK-VALUE: NAMED, NOT DERIVED — artifactTagPattern = `^([a-z][a-z0-9-]*/)?v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$` @ tools/desk/internal/deskkit/pins.go:70 — the pattern accepts any lowercase component prefix, including one naming a different artifact. Whether the prefix must match the line's artifact is a grammar choice reserved to the human by gate-why.
- RISK-VALUE: NAMED, NOT DERIVED — UmbrellaArtifact = "assay" with a plain vX.Y.Z tag @ tools/desk/internal/deskkit/pins.go:40 — the brief text says the umbrella is the assay/vX.Y.Z tag, while the spec and code publish `assay <vX.Y.Z>`. The umbrella line's meaning is on the gate-why list for human confirmation.
- RISK-VALUE: NAMED, NOT DERIVED — deskpins exit codes 0 clean / 1 checked-failed / 2 unreadable / 3 malformed @ tools/desk/cmd/deskpins/main.go:45-48 — row 4 proves they are distinct, but the spec section still does not publish them, and the desk's pin reader uses ExitUnverifiable = 6 @ tools/desk/internal/deskkit/exitcodes.go:55 for the same states. Nothing derives why the two differ.

**VERIFY: PASS** — 11/11 rows at c3db7cca008e. Row 8 now passes and depends on the stamp. Row 2 exited 0
on the third run; run 1 failed on an environment interaction (a deskwt test writing global git config
into the shared GIT_CONFIG_GLOBAL file) and run 2 on a suspected pipefail race in the evidence-automerge
class guard, both outside this brief. Gate human: the human sign-off on the published grammar remains open.

### Re-verify 2026-11-23 at 52f341a6e0dc (machine witness, fail-first)

Machine witness, statusgen verifyrun with --fail-first, delivering commit 320a8b744ee8, base 43420f7ecd74. Rows selected with --rows 0,1,2,5,6,7,8,9; rows 3, 3a and 4 were excluded by that --rows filter (see below). Witness bytes as emitted:

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 0 | `n=$( { grep -rl 'assay-versions. pin file' docs/ README.md tools/desk/README.md 2>/dev/null; } \| xargs -r grep -l -e '^#\{1,3\} .*pin file' \| sort -u \| wc -l \| tr -d ' '); echo "spec-homes=$n"; [ "$n" -eq 1 ]` | pass exit=0 | sha256:475548b2fa90 | 2026-11-23 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=43420f7ecd74 green rc=0 · head=52f341a6e0dc · delivering=320a8b744ee8 output=sha256:475548b2fa9078528c967be6d115fbf5092446b8abca0aebedc91e692a97acb1 log=row-000001.log |
| 1 | `SPEC=docs/distribution.md; BT=$(printf '\140'); test -f "$SPEC" && grep -q "^## The ${BT}.assay-versions${BT} pin file" "$SPEC" && n=$(grep -c -e statusgen -e desk-tools -e daily-harvest -e sha256 -e could-not-check "$SPEC"); echo "n=$n"; [ "$n" -ge 5 ]` | pass exit=0 | sha256:180c600340fc | 2026-11-23 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=43420f7ecd74 green rc=0 · head=52f341a6e0dc · delivering=320a8b744ee8 output=sha256:81acd2eb27b6b69f9a6a5a8c9508335e5e032ac98cc8ccb56cf1ab5469d7591a log=row-000002.log |
| 2 | `cd tools/desk && go test ./... && go vet ./...` | fail exit=1 | sha256:8c642281b817 | 2026-11-23 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=43420f7ecd74 red rc=1 · head=52f341a6e0dc · delivering=320a8b744ee8 output=sha256:7b67d87008f170d43e80d2402d9e798f2dbb603321248c36eb01c0ff2eb02532 log=row-000003.log |
| 5 | `cd tools/desk && go test ./cmd/deskboard/... -run TestNextup_PinFlowFromConfiguredRoot -v > /tmp/f.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/f.txt); f=$(grep -c '^--- FAIL' /tmp/f.txt); echo "fail=$f rc=$rc pass=$p"; [ "$p" -ge 1 ] && [ "$f" -eq 0 ] && [ "$rc" -eq 0 ]` | pass exit=0 | sha256:2199037f7365 | 2026-11-23 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=43420f7ecd74 green rc=0 · head=52f341a6e0dc · delivering=320a8b744ee8 output=sha256:2199037f7365e87480888563bb61fe074341b3638b96c745a07d99cd67304da5 log=row-000004.log |
| 6 | `cd tools/desk && go test ./cmd/deskboard/... -run TestStatusgenPin -v > /tmp/n.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/n.txt); f=$(grep -c '^--- FAIL' /tmp/n.txt); echo "fail=$f rc=$rc pass=$p"; [ "$p" -ge 1 ] && [ "$f" -eq 0 ] && [ "$rc" -eq 0 ]` | pass exit=0 | sha256:2199037f7365 | 2026-11-23 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=43420f7ecd74 green rc=0 · head=52f341a6e0dc · delivering=320a8b744ee8 output=sha256:2199037f7365e87480888563bb61fe074341b3638b96c745a07d99cd67304da5 log=row-000005.log |
| 7 | `grep -q -e 'deskkit.ReleaseTag' -e 'deskkit.Tag' .github/workflows/release.yml && grep -q 'RELEASE_TAG' .github/workflows/release.yml` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-23 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=43420f7ecd74 green rc=0 · head=52f341a6e0dc · delivering=320a8b744ee8 output=sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 log=row-000006.log |
| 8 | `cd tools/desk && go test ./internal/deskkit/... -run TestVersionStampedFromReleaseWorkflow -v > /tmp/v.txt 2>&1; rc=$?; p=$(grep -c '^--- PASS' /tmp/v.txt); echo "rc=$rc pass=$p"; [ "$p" -ge 1 ] && [ "$rc" -eq 0 ]` | pass exit=0 | sha256:70d9c12f19f5 | 2026-11-23 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=43420f7ecd74 red rc=1 · head=52f341a6e0dc · delivering=320a8b744ee8 output=sha256:bcc290f4a347f87314529b063fb6e7454c9cb0e25238a01df2f2e11b76bcae82 log=row-000007.log |
| 9 | `n=$(grep -rn 'ConsumerRepo' tools/desk/cmd/deskboard/*.go \| grep -v _test \| grep -c 'hardcoded' \| tr -d ' '); echo "hardcoded=$n"; [ "$n" -eq 0 ]` | pass exit=0 | sha256:5ededa9fefc0 | 2026-11-23 | assay-verifier-app[bot] @ 52f341a6e0dc (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=43420f7ecd74 green rc=0 · head=52f341a6e0dc · delivering=320a8b744ee8 output=sha256:5ededa9fefc00ec36d275c7b42c3c68e9655d7c3d8b3accc94d90fc8d6b917e5 log=row-000008.log |
Verification-Attestation: medici-finance/assay#7502 run=54e407a4b6862de00660c15d668747be37e0fc2116436cfe0f16717e0e01cda1 source=52f341a6e0dc25f7cdc915377f3e9ce6728df746 brief=docs/streams/desk-tools/brief-01-binary-channel-and-pin-contract.md model=claude-opus-5-5 tier=strong

Hand rows (each row run by hand under the verifier Git environment, offline, KUBECONFIG pointed at /dev/null):

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 0 | the row 0 command exactly as the Verify table writes it | spec-homes=1, exit 0 | exit 0, spec-homes=1; agrees with the witness | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 1 | the row 1 command exactly as the Verify table writes it | n of 5 or more, exit 0 | exit 0, n=25; agrees with the witness | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | the row 2 command exactly as the Verify table writes it | go test and go vet green | exit 1 at load average 75 to 83: 116 packages ok, 13 FAIL. Eleven are 10m0s test-binary timeouts (cellctl, deskdispatch, deskevidence, deskpushguard, deskwt, muhar, cellcache, clicontract, deskkit, regression, reviewtest); the other two are timing assertions (writeguard subtest hit its own 45s timeout, cellscratch maintenance-quiet waited 3.08s). go vet did not execute because go test exited first. The brief's own packages cmd/deskboard and cmd/deskpins are ok; internal/deskkit timed out. A failing package set that differs between runs, and is red at base too, reads as host load, not a deliverable defect. Agrees with the witness result (fail exit=1) | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 (supplemental, no acceptance credit) | go test of the internal/deskkit package tree alone | ok | exit 1, 10m0s timeout at load average 113, the running test at the alarm was a performance test (TestNewDeskCommandsAppendPerformance); no assertion failure reported before the timeout | 2026-11-24 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | the row 3 command exactly as the Verify table writes it, with deskpins from make desk-build | exit 0, checked-clean | exit 0, checked-clean: 12 artifact line(s), 80 comment line(s). The fixture has 4 per-platform desk-tools lines, 4 statusgen lines, 4 daily-harvest lines and no bare desk-tools line | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3a | the row 3a command exactly as the Verify table writes it, same binary | exit 0, non-zero dup-rc | exit 0, dup-rc=1; line 99: duplicate artifact statusgen (first seen line 46) | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | the row 4 command exactly as the Verify table writes it, same binary | exit 0; absent and malformed non-zero and distinct | exit 0, absent=2 malformed=3; the malformed line reports statusgen has 2 field(s), missing sha256 | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | the row 5 command exactly as the Verify table writes it | rc 0, one or more PASS, no FAIL | witness reports pass exit=0; the go test of the brief's cmd/deskboard package passed in the row 2 hand run too | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | the row 6 command exactly as the Verify table writes it | rc 0, one or more PASS, no FAIL | witness reports pass exit=0; cmd/deskboard ok in the row 2 hand run | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | the row 7 command exactly as the Verify table writes it | exit 0 | exit 0; agrees with the witness | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | the row 8 command exactly as the Verify table writes it | rc 0, one or more PASS | witness reports pass exit=0 at head and red rc=1 at base | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | the row 9 command exactly as the Verify table writes it | hardcoded=0, exit 0 | exit 0, hardcoded=0; agrees with the witness | 2026-11-23 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Row accounting: 11 Verify rows (0, 1, 2, 3, 3a, 4, 5, 6, 7, 8, 9). Machine witness: 8 rows, 7 pass and 1 fail (row 2). Rows 3, 3a and 4 have hand evidence only; the --rows filter left them out of the witness because the admitted run cannot hold their build output (attempt 1 below).

Attempts:
- Attempt 1, 2026-11-23 23:08:13Z to 23:08:20Z, superseded, no rows executed. Refusal: statusgen verifyrun: pre-work verifier admission failed; no Verify rows executed: exit status 5 ... unattested worktree file: tools/desk/dist/cellctl. Cause: the brief's build step (make desk-build) writes into the gitignored tools/desk/dist directory inside the home, and admission refuses any file the attested tree does not hold. Class: check-definition.
- Attempt 2, 2026-11-23 23:08:55Z to 23:09:05Z, superseded, no rows executed. Refusal: --delivering-commit 6b7b411d11... is refused: neither the base nor the delivering commit 6b7b411d1181 holds the brief, so the Verify rows did not exist on either tree. Cause: the deliverable landed on 2026-08-15 before the brief was re-homed into this repository on 2026-08-26. Class: check-definition (fail-first provenance).
- Attempt 3, 2026-11-23 23:09:41Z to 23:37:39Z, counted, exit 1. This is the witness above, using the row 8 fix (320a8b744ee8) as the delivering commit, the latest delivery that the refusal rule admits.
- Row 2 left a Python bytecode cache under images/verify-runner/__pycache__ in the home (the kubecompat profile-contract test runs a checker script without -B). I removed it before the second admission check. This is a test-hygiene finding: a later admitted run would be refused on it.

Fail-first: row 8 is red at base (rc=1) and passes at head, so it discriminates. Row 2 is red at base and fails at head under load, so it discriminates nothing. Rows 0, 1, 5, 6, 7 and 9 are green at base 43420f7ecd74, so they do not discriminate. Their deliverables landed in August, and the only base the run admits is the parent of the September row 8 fix. Rows 3, 3a and 4 have no base result. No deliverable defect was observed. The gap is in the check definition: no admissible base exists that is both before delivery and holds the brief.

Risk-bearing values (customer: yes; enumerated over the pin parser, the deskpins command, the version stamp and the release workflow):
- RISK-VALUE: DERIVED — sha256Pattern = ^[0-9a-f]{64}$ @ tools/desk/internal/deskkit/pins.go:77 — a SHA-256 digest is 32 bytes, which is 64 lowercase hex characters in the form that sha256sum and the release checksums.txt print. The pin sha256 field is compared against that file.
- RISK-VALUE: DERIVED — commitPattern = ^[0-9a-f]{40}$ @ tools/desk/internal/deskkit/pins.go:78 — a source pin names a git SHA-1 object id, which is 40 lowercase hex characters.
- RISK-VALUE: DERIVED — line prefix = artifact + single space @ tools/desk/internal/deskkit/pins.go:133 (umbrella form at line 432) — the pin-file spec in docs/distribution.md (lines 91 to 138) defines a line as space-separated <artifact> <tag> <sha256>. Matching the prefix with the trailing space stops statusgen from matching statusgen-linux-amd64.
- RISK-VALUE: DERIVED — field count = 3 (fewer than 3 refused at lines 140, 372 and 568; more than 3 refused at line 573) @ tools/desk/internal/deskkit/pins.go — the same spec line has exactly three fields. Row 4 confirms that a 2-field line is reported as malformed.
- RISK-VALUE: NAMED, NOT DERIVED — artifactTagPattern = ^([a-z][a-z0-9-]*/)?v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ @ tools/desk/internal/deskkit/pins.go:70 — the spec does not say whether pre-release tags are excluded or whether the tag prefix must match the artifact. A probe shows that statusgen with a daily-harvest/v0.1.0 tag still checks clean.
- RISK-VALUE: NAMED, NOT DERIVED — umbrella line = 2 fields @ tools/desk/internal/deskkit/pins.go:439 and 555 — the spec section in docs/distribution.md does not document the umbrella line form.
- RISK-VALUE: NAMED, NOT DERIVED — deskpins exits 0 checked-clean, 1 checked-failed, 2 could-not-check, 3 malformed (5 for a refused command line) @ tools/desk/cmd/deskpins/main.go:49 — the spec names no read, absent or malformed states, so it gives nothing to check these exit codes against. Row 4 shows only that absent and malformed are distinct.

Gate observations (gate: human, customer: yes):
- A tab-separated pin line still checks clean (rc 0), and a statusgen line carrying a daily-harvest/v0.1.0 tag still checks clean (rc 0). Both earlier observations still hold.
- docs/distribution.md still does not name daily-harvest in the pin-file section, and its worked example uses a synthetic artifact.

Check-definition amendments needed: rows 3, 3a and 4 should build deskpins outside the tree (for example go build -o into a temporary directory), or the brief needs a sanctioned allowance for build output, so that the rows can run inside an admitted witness. The fail-first base for rows 0, 1, 5, 6, 7 and 9 needs a ruling or rows that discriminate at the admissible base. Environment: row 2 needs a host that is not under load (load average was 54 to 113 throughout).

**VERIFY: FAIL** — check-definition: rows 0, 1, 5, 6, 7, 9 do not discriminate at the only admissible fail-first base, and rows 3, 3a, 4 cannot run in the admitted witness (they pass by hand); environment: row 2 (host-load test timeouts, also red at base). No deliverable defect observed.

## Review
Gate: **human** (from frontmatter; see `gate-why`). The human signs off the published grammar —
field order, the `<artifact>-<platform>` convention, the trailing-space selection rule, and the
umbrella line's meaning — plus the choice that an unreadable pin fails closed rather than
defaulting. A human gate needs a `human:<name>` entry, not a model sign-off.
