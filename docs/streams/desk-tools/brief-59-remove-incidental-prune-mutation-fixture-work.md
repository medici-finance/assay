---
brief: assay:assay:desk-tools:59
title: Remove incidental prune mutation fixture work
why: "The prune mutation sweep repeats expensive performance fixtures for every mutation, including mutations they cannot detect. Move the correctness invariants into focused tests so verification measures the guards rather than fixture construction."
wave: 1
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-24 by maintainer-directed mutation review
sources:
  - medici-finance/assay#4359
  - tools/desk/cmd/deskwt/pruneperf_test.go
  - freshness-checked 2026-10-24 @ 61eb62a4a (origin/main)
domain: complicated
consumers:
  - "tools/desk/cmd/deskwt/prune-mutations.json: fixed-here"
---

# Brief 59 — Remove incidental prune mutation fixture work

## Context
files: tools/desk/cmd/deskwt/prune-mutations.json, tools/desk/cmd/deskwt/pruneperf_test.go, tools/desk/cmd/deskwt/prune_test.go, tools/desk/cmd/deskwt/prunesingleton_test.go, changelog/muhar-prune-fixtures.md
facts:
- On 2026-10-24 the manifest selected 151 top-level tests for the baseline, positive control and each of 55 mutations.
- TestPruneSweepScalesAtN and TestConcurrentPrunesCostOne each construct a 5,000-commit history and 50 worktrees by default; only the ancestor-walk mutation needs the scaling test's one-walk invariant.
- The singleton interval test sleeps 300 ms and can perform six sweeps; runPruneLoop already sweeps once before selecting on stop.
- Keep real Git fixtures and the existing performance tests in their ordinary verification rows. This brief removes incidental work from the mutation lane, not performance coverage.

design-fit:
  owner: deskwt prune tests and the prune mutation manifest
  contract: none — this rearranges existing test coverage without changing the production contract
  retires: [performance fixtures in unrelated mutation runs, the multi-sweep interval sleep]
  weight: "No new verbs, flags, refusals or rule text; one focused test replaces indirect performance-only coverage."
  why-add: "A tiny correctness fixture is required because the 5,000-commit performance fixture is the wrong owner for a mutation-detection invariant."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not weaken or delete the existing scale, concurrency, or real-Git assertions.

## Task
1. Add `TestPruneWalksMainOnce` using a small real-Git fixture. It must prove that one sweep performs exactly one mainline ancestor walk through the existing walk seam, including the shape exercised by the ancestor-walk mutation.
2. Keep `TestPruneSweepScalesAtN` and `TestConcurrentPrunesCostOne` unchanged in their ordinary performance rows. Add `-skip '^(TestPruneSweepScalesAtN|TestConcurrentPrunesCostOne)$'` to the prune mutation manifest so neither runs for unrelated mutations.
3. Rework the singleton interval test as `TestPruneIntervalLockRelease`: use one immediate tick and a pre-closed stop channel, then prove the singleton can be reacquired. Remove the 300 ms sleep and extra sweeps without removing the lock-release assertion.
4. Preserve every existing mutation anchor and the exact 55-caught gate. Run the complete manifest to prove the small walk test catches the ancestor-walk mutation and that no other mutation survives.
5. Change this brief's consumer entry from `follow-up desk-tools/59` to `fixed-here` in the implementation PR. Do not leave a deferred routing claim after the manifest edit lands.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/deskwt -run '^TestPruneWalksMainOnce$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-walk.log" 2>&1 && grep -F -- '--- PASS: TestPruneWalksMainOnce ' "${TMPDIR:-/tmp}/dt59-walk.log"` | exit 0; named PASS; the test reports exactly one mainline walk | check:ci +mutation |
| 2 | `cd tools/desk && go test ./cmd/deskwt -run '^TestPruneIntervalLockRelease$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-interval.log" 2>&1 && grep -F -- '--- PASS: TestPruneIntervalLockRelease ' "${TMPDIR:-/tmp}/dt59-interval.log"` | exit 0; named PASS with one tick and no interval sleep | check:ci |
| 3 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt59-anchors.log"` | exit 0; every manifest anchor still lands | check:ci |
| 4 | `cd tools/desk && report=$(go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json) && printf '%s\n' "$report" && case "$report" in *'Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.'*) ;; *) exit 1;; esac` | exit 0; exact 55 caught, zero survivors and zero unapplied mutations | check +mutation +flow |
| 5 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/59 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; the implementation diff contains the manifest and its routing is corroborated | check +flow +dereference |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->

### Verification, 2026-11-11, at merged source 6a9501ba212d2c07c32a5e4fd5f5a2708c949f8d

Verification-Table: delivery=ecee257bd06719468d82522013ff160272e85c3e base=aae4fbf71bca3419f1ac7ece8e83e8e5a651983e original=aae4fbf71bca3419f1ac7ece8e83e8e5a651983e

Independent verifier pass (claude-opus-5-5, strong tier, not the implementer) at merged main 6a9501ba212d2c07c32a5e4fd5f5a2708c949f8d under desk tools v1.0.82. Admission (deskdispatch --check-verifier) returned exit 0 before any row ran; the attestation it bound (medici-finance/assay#6209) is named on both witnesses below. Delivery, re-derived on the home: merge ecee257bd06719468d82522013ff160272e85c3e (#5919) is on HEAD's first-parent chain; its parents 8aedd1b42291 and 99f9a1e82889 have the single merge base aae4fbf71bca. The Verify section hashes identically at that merge base, at the first parent, at the PR head, at the delivery and at this source, so the marker above names the merge base as both base and original. deskpathguard rederive reported 5 rows, 0 author-added. Later commits do touch tools/desk/cmd/deskwt/ and tools/desk/cmd/muhar/ (424 commits follow the delivery), so every row below observes the current source, not the delivery snapshot; pruneperf_test.go is byte-identical between the delivery's first parent and this source, so the two performance tests were left unchanged as the brief requires. Host: darwin/arm64, statusgen v1.0.82, git 2.56.0, KUBECONFIG=/dev/null, no GitHub access variable in the run environment.

Fail-first: no row is risk-bearing. The brief-wide risk block answers no to all four questions, and no Verify row carries a risk-bearing, live, mutating or end-to-end tag, so no fail-first witness is required and none was taken.

Rows 1-3 are check:ci rows. This host has no unshare network sandbox, so they ran in the container admission bracket, selected with --rows 1,2,3: host open, container run, host close, with no edit to the home between the steps. The open step used --module-cache with a new empty directory outside the home; it downloaded the 14 listed module roots against their go.sum and recorded Module-Cache sha256:7f8fa9800956. The container ran the release-pinned runner image ghcr.io/medici-finance/verify-runner:v1.0.82@sha256:43db6da8cd159b2c822af62bb859f1a305637c066ea1a750df16dfd5d7b99563; the v1.0.82 release's verify-runner-image.txt names that exact reference and its sha256 (9348566bea37) matches the release checksums. Its native linux/arm64 entry carries go1.27.3 and statusgen v1.0.82, and it was started with the flags the open step printed: --network none, --read-only, the module cache mounted read-only, GOPROXY=off, GOTOOLCHAIN=local and GOFLAGS=-mod=readonly. It ran as the host uid:gid with all capabilities dropped. The home and its git directory were mounted read-only, the receipt read-only, the only writable bind mount was a new directory for the witness file, and /tmp (holding the Go build cache, HOME and the rows' log files) was a fresh tmpfs. No Verify row was attempted outside the bracket, and no attempt was discarded.

The container step admitted the home (its tree digest equalled the one recorded at the open step) and all three rows passed. The close step appended the Admission-Bracket line. statusgen verifyrun --bracket-digest on the witness then reported verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1. The block below is that file's bytes, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskwt -run '^TestPruneWalksMainOnce$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-walk.log" 2>&1 && grep -F -- '--- PASS: TestPruneWalksMainOnce ' "${TMPDIR:-/tmp}/dt59-walk.log"` | pass exit=0 sandbox=container-netns | sha256:b0a261164dff | 2026-11-11 | assay-verifier-app[bot] @ 6a9501ba212d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskwt -run '^TestPruneIntervalLockRelease$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-interval.log" 2>&1 && grep -F -- '--- PASS: TestPruneIntervalLockRelease ' "${TMPDIR:-/tmp}/dt59-interval.log"` | pass exit=0 sandbox=container-netns | sha256:b4d6cb8bb742 | 2026-11-11 | assay-verifier-app[bot] @ 6a9501ba212d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt59-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:3f318e9ab86c | 2026-11-11 | assay-verifier-app[bot] @ 6a9501ba212d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:3f952f4f1a55e06e88c61d6940b5a5f23b8d0c74ed4043706c34a3d40e5673e9
Container-Image: sha256:43db6da8cd159b2c822af62bb859f1a305637c066ea1a750df16dfd5d7b99563
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6209 run=8dfabf65076ce3226ae6415f86d32f61c0aced8dccddd4edb956e613eb522eac source=6a9501ba212d2c07c32a5e4fd5f5a2708c949f8d brief=docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=8dfabf65076ce3226ae6415f86d32f61c0aced8dccddd4edb956e613eb522eac source=6a9501ba212d2c07c32a5e4fd5f5a2708c949f8d tree=sha256:3f952f4f1a55e06e88c61d6940b5a5f23b8d0c74ed4043706c34a3d40e5673e9 witness=sha256:d759ec8dda22df66475aa8ed696965ac8aa8d2b97322b815e47f6ec33131a2ed opened=2026-11-11T22:42:25Z closed=2026-11-11T22:43:06Z before=receipt after=check-verifier

Rows 4 and 5 are host rows (check, not check:ci). They ran on the unedited home after the bracket closed, with statusgen verifyrun --rows 4,5 --timeout 90m --dry-run, which printed the table and attestation line below (reproduced unchanged; nothing was written to the home, and git status on the home was empty afterwards). Row 4 runs the full prune mutation manifest, which edits and restores source files in the home one mutation at a time; it took about 51 minutes. Row 5 needs two inputs. DELIVERY_HEAD was set to 6a9501ba212d2c07c32a5e4fd5f5a2708c949f8d, the admitted source this pass verifies; the row's own guard requires it to equal the home's HEAD, so the delivery merge itself cannot be named there in a post-merge run. DELIVERY_BASE was set to 8aedd1b42291da9429dd959545e4a167e79ff785, the actual first parent of the delivering merge, which is an ancestor of the source. A supplemental hand capture (no acceptance credit) of the same statusgen command printed "CORROBORATED tools/desk/cmd/deskwt/prune-mutations.json: fixed-here" and "summary: 1 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing"; the same command with --delivery ecee257bd06719468d82522013ff160272e85c3e added reported the same entry and summary for the immutable delivered snapshot.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `cd tools/desk && report=$(go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json) && printf '%s\n' "$report" && case "$report" in *'Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.'*) ;; *) exit 1;; esac` | pass exit=0 | sha256:dca8c9402524 | 2026-11-11 | assay-verifier-app[bot] @ 6a9501ba212d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/59 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:6378c2f1c28a | 2026-11-11 | assay-verifier-app[bot] @ 6a9501ba212d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6209 run=8dfabf65076ce3226ae6415f86d32f61c0aced8dccddd4edb956e613eb522eac source=6a9501ba212d2c07c32a5e4fd5f5a2708c949f8d brief=docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md model=claude-opus-5-5 tier=strong

Observed result per row:
- Row 1: pass exit=0 sandbox=container-netns, output sha256:b0a261164dff. Exit 0 of the trailing grep means the named PASS line for TestPruneWalksMainOnce was present. That test fails unless the counted walk seam (logOriginMain) ran exactly once for a sweep over three real-Git candidates, and unless both merged candidates were removed while the unmerged one stayed.
- Row 2: pass exit=0 sandbox=container-netns, output sha256:b4d6cb8bb742. Exit 0 of the trailing grep means the named PASS line for TestPruneIntervalLockRelease was present. At this source the test closes stop before the loop, passes a one-hour interval, requires exactly one "prune tick " line, requires the merged candidate removed, then requires the singleton to be reacquirable; it contains no sleep.
- Row 3: pass exit=0 sandbox=container-netns, output sha256:3f318e9ab86c. Exit 0 of the trailing grep means the named PASS line for TestSpecAnchorsLand was present.
- Row 4: pass exit=0, output sha256:dca8c9402524. The command exits 1 unless the report contains the exact line "Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.", so exit 0 means all 55 mutations were caught, including the ancestor-walk mutation, with the two performance tests skipped.
- Row 5: pass exit=0, output sha256:6378c2f1c28a. The brief carries no colon-prefixed follow-up marker, and the consumers check reported 1 corroborated, 0 disproved, 0 unchecked.

Risk-bearing values. The enumeration covers the delivery diff 8aedd1b42291..ecee257bd067 (the prune mutation manifest, the two prune test files, the changelog fragment and this brief's consumer line) and the brief's Task list; the lines below were re-read at this source:
- the mutation-lane skip, -skip '^(TestPruneSweepScalesAtN|TestConcurrentPrunesCostOne)$' @ tools/desk/cmd/deskwt/prune-mutations.json:2
- the walk count bound, walks != 1 @ tools/desk/cmd/deskwt/prune_test.go:79
- the removal count, res.removed != 2 @ tools/desk/cmd/deskwt/prune_test.go:82
- the singleton TTL in both tests, singletonTTL: 0 @ tools/desk/cmd/deskwt/prune_test.go:75 and tools/desk/cmd/deskwt/prunesingleton_test.go:226
- the loop interval, time.Hour @ tools/desk/cmd/deskwt/prunesingleton_test.go:226
- the tick count, strings.Count(out, "prune tick ") != 1 @ tools/desk/cmd/deskwt/prunesingleton_test.go:230
- the manifest's mutation count, 55 entries plus one control (56 name fields) in tools/desk/cmd/deskwt/prune-mutations.json, gated by row 4's exact Totals line

All are test or mutation-lane values, reversible by an edit, and the item is marked irreversible: no. Ranked by what breaks if wrong: the skip pattern (a broader pattern would silently drop detecting tests from the mutation lane), then the walk count bound (a looser bound would let the ancestor-walk mutation survive); the removal count, TTL, interval, tick count and mutation count are test knobs and rank last.

RISK-VALUE: DERIVED — mutation-lane skip = '^(TestPruneSweepScalesAtN|TestConcurrentPrunesCostOne)$' @ tools/desk/cmd/deskwt/prune-mutations.json:2 — Task 2 names exactly these two performance tests; the pattern is anchored at both ends and is an alternation of two full names, so it cannot match TestPruneWalksMainOnce or any other selected test. Row 4 shows the narrowed lane still catches all 55 mutations, and both performance tests remain in pruneperf_test.go, unchanged since before the delivery.
RISK-VALUE: DERIVED — walk count bound = walks != 1 @ tools/desk/cmd/deskwt/prune_test.go:79 — the sweep builds its context once and calls the logOriginMain seam once from newSweepCtx (tools/desk/cmd/deskwt/prune.go:680), whose comment states it is the one traversal of origin/main per sweep. With three candidates, the ancestor-walk mutation (manifest line 65) restores one walk per candidate and yields 3, so exactly 1 is the value that tells the two apart; no other selected test counts that seam.

**VERIFY: PASS** — 5 of 5 rows pass with admitted witnesses at merged source 6a9501ba212d. Rows 1-3 passed network-off inside a closed admission bracket (verdict=match), and rows 4 and 5 passed on the host as written. No row is risk-bearing, so no fail-first witness is required. The gate is model, and all risk answers are no.

### Verification, 2026-11-18, at merged source 9c845402c8c00d7c14bda75cd5222fff907fd05d

Independent verifier pass (claude-sonnet-5-5, tier any, not the implementer) at merged main 9c845402c8c00d7c14bda75cd5222fff907fd05d under desk tools v1.0.87, darwin/arm64, KUBECONFIG=/dev/null. Admission (deskdispatch --check-verifier) returned exit 0 before any row ran (attestation medici-finance/assay#6966). Delivery merge ecee257bd06719468d82522013ff160272e85c3e (#5919) is an ancestor of this source. Rows 1-3 ran in the closed container admission bracket (release image ghcr.io/medici-finance/verify-runner:v1.0.87, linux/arm64 manifest sha256:3cfd931fc8fb2277df25eafbc68eccfe84ce7d7b92379b927b8c1841d99854d1, network none, read-only module cache; bracket-digest verdict=match). An earlier container attempt with a missing HOME gave the same rows 1 and 2 result and was superseded by the run below. The block is the closed witness file's bytes, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskwt -run '^TestPruneWalksMainOnce$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-walk.log" 2>&1 && grep -F -- '--- PASS: TestPruneWalksMainOnce ' "${TMPDIR:-/tmp}/dt59-walk.log"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskwt -run '^TestPruneIntervalLockRelease$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-interval.log" 2>&1 && grep -F -- '--- PASS: TestPruneIntervalLockRelease ' "${TMPDIR:-/tmp}/dt59-interval.log"` | fail exit=1 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt59-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:0dcc7103c5af | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:2ded1bc2b15c8ec9dd8a3fa08a545caa03889f4360b13d60c31e628b8a9b2715
Container-Image: sha256:3cfd931fc8fb2277df25eafbc68eccfe84ce7d7b92379b927b8c1841d99854d1
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6966 run=68cffb3baa5deaf33f1d6743743abe0d725ba8ca6a6b6b28e24f9dac19488b20 source=9c845402c8c00d7c14bda75cd5222fff907fd05d brief=docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md model=claude-sonnet-5-5 tier=any
Admission-Bracket: run=68cffb3baa5deaf33f1d6743743abe0d725ba8ca6a6b6b28e24f9dac19488b20 source=9c845402c8c00d7c14bda75cd5222fff907fd05d tree=sha256:2ded1bc2b15c8ec9dd8a3fa08a545caa03889f4360b13d60c31e628b8a9b2715 witness=sha256:7709b6969726b0e45e7f40644aaa133371748278ab180521ef71e21a25e88665 opened=2026-11-18T17:44:05Z closed=2026-11-18T17:48:37Z before=receipt after=check-verifier

Rows 4 and 5 ran on the host with statusgen verifyrun --rows 4,5 --timeout 90m --dry-run (DELIVERY_HEAD=9c845402c8c00d7c14bda75cd5222fff907fd05d, DELIVERY_BASE=8aedd1b42291da9429dd959545e4a167e79ff785, the first parent of the delivery merge); nothing was written by that run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `cd tools/desk && report=$(go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json) && printf '%s\n' "$report" && case "$report" in *'Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.'*) ;; *) exit 1;; esac` | fail exit=1 | sha256:6b3b961ef6bd | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/59 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:6378c2f1c28a | 2026-11-18 | assay-verifier-app[bot] @ 9c845402c8c0 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6966 run=68cffb3baa5deaf33f1d6743743abe0d725ba8ca6a6b6b28e24f9dac19488b20 source=9c845402c8c00d7c14bda75cd5222fff907fd05d brief=docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md model=claude-sonnet-5-5 tier=any

Diagnosis: environment fault, not an observed deliverable failure. verifyrun gives every admitted row GIT_CONFIG_COUNT=2 (grep.patternType, grep.extendedRegexp). At this source the deskwt worktree-add path refuses command-priority git config ("origin could be changed by worktree-scope or include config: gitcore: remote-urls origin: ... command-priority config (GIT_CONFIG_COUNT, GIT_CONFIG_PARAMETERS or GIT_CONFIG) is set"), so the real-Git fixtures return rc=6 ("add walk-first rc = 6, want 0" at prune_test.go:44). Row 4 baseline was red on pristine source (TestPruneActivityGateHoldsRecentWorktree, "HARNESS BROKEN"), so no mutation ran. Rows 1, 2 and 4 are recorded as could-not-run: the admitted environment prevents their fixtures from running; the delivered work was not observed failing.

Supplemental host probes, NO acceptance credit: with GIT_CONFIG_COUNT=0, go test ./cmd/deskwt -run '^TestPruneWalksMainOnce$' passed (mainline walks=1 for three real-Git candidates) and -run '^TestPruneIntervalLockRelease$' passed. The same failure was reproduced by hand in the container and on the host by setting the row environment's GIT_CONFIG_COUNT=2 variables. The 55-mutation sweep was not re-run with the count cleared, so row 4 has no supplemental result.

Risk-bearing values: the brief-wide risk block answers no to all four questions and no row is risk-bearing; the diff literals (the -skip pattern at tools/desk/cmd/deskwt/prune-mutations.json:2, walks != 1 at tools/desk/cmd/deskwt/prune_test.go:79) are unchanged from the 2026-11-11 pass and were not re-derived because rows 1 and 4 could not reach them.

**VERIFY: BLOCKED** — rows 3 and 5 pass; rows 1, 2 and 4 could-not-run (admitted row environment sets GIT_CONFIG_COUNT, which deskwt refuses); no obligation observed failed. Needs an environment or check-definition repair, then a fresh dispatch.

### Verification, 2026-11-25, at merged source 77ff0f2222a28df926c38ad30fea4a368173712e

Verification-Table: delivery=ecee257bd06719468d82522013ff160272e85c3e base=aae4fbf71bca3419f1ac7ece8e83e8e5a651983e original=aae4fbf71bca3419f1ac7ece8e83e8e5a651983e

Independent verifier pass (claude-opus-5-5, strong tier, not the implementer) at merged main 77ff0f2222a28df926c38ad30fea4a368173712e under desk tools v1.0.94. Host: darwin/arm64, git 2.56.1, go1.27.4, KUBECONFIG=/dev/null, no GitHub access variable in the run environment. Admission (deskdispatch --check-verifier) returned exit 0 before any row ran and again before this Evidence edit; the attestation it bound (medici-finance/assay#7681) is named on both witnesses below. Delivery, re-derived on the home: merge ecee257bd06719468d82522013ff160272e85c3e (#5919) is on HEAD's first-parent chain. Its parents 8aedd1b42291 and 99f9a1e82889 have the single merge base aae4fbf71bca. The Verify section hashes identically at that merge base, at both parents, at the delivery and at this source. deskpathguard rederive reported 5 rows, 0 author-added. pruneperf_test.go is byte-identical between the delivery's first parent and this source. This pass follows the 2026-11-18 BLOCKED pass. Its cause, the row environment's grep pins (GIT_CONFIG_COUNT=2) being refused by deskwt's git reads, is cleared at this source: gitcore now qualifies exactly that pin set (f71602140e). Rows 1 and 2 below ran with it.

Fail-first: no row is risk-bearing. The brief-wide risk block answers no to all four questions, and no Verify row carries a risk-bearing, live, mutating or end-to-end tag. No fail-first witness is required and none was taken.

Rows 1-3 are check:ci rows. This host has no unshare network sandbox, so they ran in the container admission bracket, selected with --rows 1,2,3: host open, container run, host close, with no edit to the home between the steps. The open step used --module-cache with a new empty directory outside the home (Module-Cache sha256:7f8fa9800956) and --roster-copy. The container ran the release-pinned runner image by digest only, ghcr.io/medici-finance/verify-runner@sha256:79e23ec185b1d5afe40cd92a25d9f7b18fd824f5678bf6c945443753ee6f509a (the v1.0.94 reference in the house image lock), with no platform override. Its native linux/arm64 entry carries go1.27.4, git 2.56.1 and statusgen v1.0.94. It was started with the flags the open step printed: --network none, --read-only, the module cache mounted read-only, GOPROXY=off, GOTOOLCHAIN=local and GOFLAGS=-mod=readonly. It ran as the host uid:gid with all capabilities dropped and no-new-privileges. The home and its git directory were mounted read-only, the receipt read-only, the only writable bind mount was a new directory for the witness file, and /tmp was a fresh tmpfs. The container step admitted the home (tree digest equal to the open step's) and all three rows passed. The close step appended the Admission-Bracket line, and statusgen verifyrun --bracket-digest on the witness then reported verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1. The block below is that file's bytes, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/deskwt -run '^TestPruneWalksMainOnce$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-walk.log" 2>&1 && grep -F -- '--- PASS: TestPruneWalksMainOnce ' "${TMPDIR:-/tmp}/dt59-walk.log"` | pass exit=0 sandbox=container-netns | sha256:da5e21fc94bf | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/deskwt -run '^TestPruneIntervalLockRelease$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-interval.log" 2>&1 && grep -F -- '--- PASS: TestPruneIntervalLockRelease ' "${TMPDIR:-/tmp}/dt59-interval.log"` | pass exit=0 sandbox=container-netns | sha256:b4d6cb8bb742 | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt59-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt59-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:7c8de195f1ef | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:9f9dc8e41b551f84bcb466458b62ba7a4d70436974ad0e17cc8ba433bcd1ccc6
Container-Image: sha256:79e23ec185b1d5afe40cd92a25d9f7b18fd824f5678bf6c945443753ee6f509a
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7681 run=bce2d302dc5fb660a482703633f15dd1bdef556ab27d8d3f6bcee374b133f8a4 source=77ff0f2222a28df926c38ad30fea4a368173712e brief=docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=bce2d302dc5fb660a482703633f15dd1bdef556ab27d8d3f6bcee374b133f8a4 source=77ff0f2222a28df926c38ad30fea4a368173712e tree=sha256:9f9dc8e41b551f84bcb466458b62ba7a4d70436974ad0e17cc8ba433bcd1ccc6 witness=sha256:8508a252d7bdad8f07aa3610b596f1947dfc86737c81a4b817559102fb904fa9 opened=2026-11-25T04:04:31Z closed=2026-11-25T04:05:24Z before=receipt after=check-verifier

Rows 4 and 5 are host rows (check, not check:ci). They ran on the unedited home after the bracket closed, with statusgen verifyrun --rows 4,5 --timeout 90m --dry-run, which printed the table and attestation line below (reproduced unchanged; nothing was written to the home, and git status on the home was empty afterwards). Row 4 ran from 04:05:31Z to 05:13:20Z. Row 5 needs two inputs. DELIVERY_HEAD was set to 77ff0f2222a28df926c38ad30fea4a368173712e, the admitted source, because the row's own guard requires it to equal the home's HEAD. DELIVERY_BASE was set to 8aedd1b42291da9429dd959545e4a167e79ff785, the first parent of the delivering merge.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `cd tools/desk && report=$(go run ./cmd/muhar -spec cmd/deskwt/prune-mutations.json) && printf '%s\n' "$report" && case "$report" in *'Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.'*) ;; *) exit 1;; esac` | fail exit=1 | sha256:a4e1c7b486ac | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/59 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 1 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:6378c2f1c28a | 2026-11-25 | assay-verifier-app[bot] @ 77ff0f2222a2 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7681 run=bce2d302dc5fb660a482703633f15dd1bdef556ab27d8d3f6bcee374b133f8a4 source=77ff0f2222a28df926c38ad30fea4a368173712e brief=docs/streams/desk-tools/brief-59-remove-incidental-prune-mutation-fixture-work.md model=claude-opus-5-5 tier=strong

Observed result per row:
- Row 1: pass exit=0 sandbox=container-netns, output sha256:da5e21fc94bf. Exit 0 of the trailing grep means the named PASS line for TestPruneWalksMainOnce was present. At this source that test fails unless the counted walk seam (logOriginMain) ran exactly once for a sweep over three real-Git candidates.
- Row 2: pass exit=0 sandbox=container-netns, output sha256:b4d6cb8bb742. Exit 0 of the trailing grep means the named PASS line for TestPruneIntervalLockRelease was present. The test closes stop before the loop, passes a one-hour interval, requires exactly one "prune tick " line and then requires the singleton to be reacquirable. It contains no sleep.
- Row 3: pass exit=0 sandbox=container-netns, output sha256:7c8de195f1ef. Exit 0 of the trailing grep means the named PASS line for TestSpecAnchorsLand was present: every manifest anchor still lands.
- Row 4: fail exit=1, output sha256:a4e1c7b486ac. The command exits 1 unless the report contains the exact line "Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate.". The run took 68 minutes, so the harness baseline and control passed and the mutations ran. A red baseline stops muhar within minutes, as on 2026-11-18. The required totals line was absent.
- Row 5: pass exit=0, output sha256:6378c2f1c28a. The brief carries no colon-prefixed follow-up marker, and the consumers check reported 1 corroborated, 0 disproved, 0 unchecked.

Supplemental diagnosis, NO acceptance credit. verifyrun records only the output digest, so the row 4 report was captured from a separate run: the same muhar spec from tools/desk with -j 2 (two isolated copies), under the declared verifier Git environment plus the row's two grep pins, 05:13:55Z to 05:53:15Z, exit 0. It printed "Harness healthy: baseline GREEN, positive control CAUGHT." and "Totals: 54 caught, 1 NOT CAUGHT, 0 could-not-mutate.". The one survivor is the manifest entry "the identity refusals stop binding the reap arm (the shared checkout / the current worktree become reap candidates)" (tools/desk/cmd/deskwt/prune-mutations.json line 101). That mutation removes the !protected term from the reap arm's guard in tools/desk/cmd/deskwt/prune.go (line 1021 at this source). The ancestor-walk mutation was CAUGHT. A second supplemental probe applied only that mutation to a git-archive copy of this source outside the home. With it applied, go test ./cmd/deskwt -run 'TestDeadSession|TestReap' passed every test (exit 0), so no test in that set still detects it. At the 2026-11-11 source, all 55 mutations were caught. Since then the reap path changed in deadsession.go and prune.go (7517ef75a0, 23e8dda672, 292d881942 and 06fee59f4e). judgeReap now asks a dispatch-claim gate before ownership. The test that asserts the shared checkout survives a reaping sweep (deadsession_test.go) is byte-identical across that span. That points to a later change masking the identity guard rather than to this brief's delivered diff; this pass did not isolate which gate now holds the shared checkout. The delivered parts of this brief behave as specified: rows 1-3 and 5 pass, the skip pattern and walk test are intact, and the ancestor-walk mutation is caught.

Risk-bearing values. The enumeration covers the delivery diff 8aedd1b42291..ecee257bd067 (the prune mutation manifest, the two prune test files, the changelog fragment and this brief's consumer line) and the brief's Task list; the lines below were re-read at this source:
- the mutation-lane skip, -skip '^(TestPruneSweepScalesAtN|TestConcurrentPrunesCostOne)$' @ tools/desk/cmd/deskwt/prune-mutations.json:2
- the walk count bound, walks != 1 @ tools/desk/cmd/deskwt/prune_test.go:68
- the removal count, res.removed != 2 @ tools/desk/cmd/deskwt/prune_test.go:71
- the singleton TTL, singletonTTL: 0 @ tools/desk/cmd/deskwt/prune_test.go:64 and tools/desk/cmd/deskwt/prunesingleton_test.go:226
- the loop interval, time.Hour @ tools/desk/cmd/deskwt/prunesingleton_test.go:226
- the tick count, strings.Count(out, "prune tick ") != 1 @ tools/desk/cmd/deskwt/prunesingleton_test.go:230
- the manifest's mutation count, 55 entries plus one control (56 name fields) in tools/desk/cmd/deskwt/prune-mutations.json, gated by row 4's exact Totals line

All are test or mutation-lane values, reversible by an edit, and the item is marked irreversible: no. Ranked by what breaks if wrong: the skip pattern first, because a broader pattern would silently drop detecting tests from the mutation lane. The walk count bound comes next, because a looser bound would let the ancestor-walk mutation survive. The removal count, TTL, interval, tick count and mutation count are test knobs and rank last.

RISK-VALUE: DERIVED — mutation-lane skip = '^(TestPruneSweepScalesAtN|TestConcurrentPrunesCostOne)$' @ tools/desk/cmd/deskwt/prune-mutations.json:2 — Task 2 names exactly these two performance tests. The pattern is anchored at both ends and alternates two full names, so it cannot match TestPruneWalksMainOnce or any reap or dead-session test. The surviving mutation is therefore not caused by the skip: in the archived probe, the full TestDeadSession and TestReap set also passed with the mutation applied.
RISK-VALUE: DERIVED — walk count bound = walks != 1 @ tools/desk/cmd/deskwt/prune_test.go:68 — the sweep calls the logOriginMain seam once from newSweepCtx (tools/desk/cmd/deskwt/prune.go:689). With three candidates, the ancestor-walk mutation restores one walk per candidate and yields 3, so exactly 1 is the value that tells the two apart. The supplemental report shows that mutation CAUGHT.

**VERIFY: FAIL** — row 4: exit 1 at merged source 77ff0f2222a2, the exact "Totals: 55 caught, 0 NOT CAUGHT, 0 could-not-mutate." line is absent. The supplemental capture, with no acceptance credit, shows 54 caught and 1 NOT CAUGHT: the reap-arm identity-refusal mutation survives. Rows 1, 2, 3 and 5 pass with admitted witnesses. The item does not advance. The gate is model, and all risk answers are no.

## Failure map
- The small walk test misses the mutation: row 4 remains red with a surviving ancestor-walk mutation.
- Performance coverage is accidentally removed: the performance files and their ordinary rows remain unchanged; review must reject their deletion.
- The interval test becomes a timing-only check: row 2 must prove lock reacquisition after a real tick.
- The manifest changes but its consumer claim stays deferred: row 5 fails.

## Review
Gate: model. Check that mutation coverage moved rather than disappeared and that the full 55-mutation result is mechanical.
