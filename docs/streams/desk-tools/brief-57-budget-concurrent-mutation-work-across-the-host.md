---
brief: assay:assay:desk-tools:57
title: Budget concurrent mutation work across the host
why: "Independent verify and review runs each size their mutation workers as if they own the machine, causing avoidable contention and misleading timeouts. A shared lease budget will bound total mutation work while preserving each run's complete mutation set."
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
  - tools/desk/cmd/muhar/main.go
  - freshness-checked 2026-10-24 @ 61eb62a4a (origin/main)
exec-tier: strong
exec-tier-why: "Cross-process locking, cancellation and launcher propagation require concurrency and lifecycle reasoning across components."
domain: complex
consumers:
  - "tools/desk/cmd/muhar/README.md: fixed-here (host lease, queue measurement and recovery contract)"
  - "tools/desk/cmd/cellctl/env.go: fixed-here (shared composition propagates explicit budget inputs)"
  - "tools/desk/cmd/cellctl/launch.go: fixed-here (host launch and dry-run compose explicit budget inputs)"
  - "tools/desk/cmd/cellctl/envkeys.go: fixed-here (both optional inputs classified without defaults)"
  - "docs/cellctl.md: fixed-here (shared directory, total and recovery documented)"
---

# Brief 57 — Budget concurrent mutation work across the host

## Context
files: tools/desk/cmd/muhar/main.go, tools/desk/cmd/muhar/budget.go, tools/desk/cmd/muhar/budget_test.go, tools/desk/cmd/muhar/README.md, tools/desk/cmd/cellctl/env.go, tools/desk/cmd/cellctl/launch.go, tools/desk/cmd/cellctl/envkeys.go, tools/desk/cmd/cellctl/budget_test.go, tools/desk/cmd/cellctl/envkeys_test.go, docs/cellctl.md, changelog/muhar-host-budget.md
facts:
- On 2026-10-24 three independent muhar processes and a full Go review suite overlapped on one 16-CPU host; per-process -j limits did not bound their sum.
- Existing -j 0 caps one process at two workers; it is not a host-wide budget.
- The lease must bound mutation count, not change which mutations run. Every run still executes its baseline, control and complete spec.
- The lock primitive is non-blocking (`deskkit.TryLockExclusive`), so acquisition is per-worker-start with release when that worker completes — no worker ever waits while holding a lease, and every held lease belongs to a worker making independent progress, which is what rules out the classic two-process wedge.
- Containerised runs are covered only when they explicitly share the lease directory and locking semantics; otherwise the configuration is could-not-check, not a claim of host coverage.

layering: Add one small lease owner used by muhar, with pure slot-count policy separated from OS lock effects. cellctl only propagates reviewed configuration; it does not become a second lock manager or scheduler.
design-fit:
  owner: muhar worker admission and cellctl's existing environment propagation
  contract: none — local lease files stay inside the existing tool boundary
  retires: [independent unbounded mutation worker pools on one host]
  weight: "Two opt-in environment values and invalid-config refusals; no new command or daemon."
  why-add: "A shared lease directory and total slot count are the minimum cross-process inputs; per-run flags alone cannot express a machine-wide limit."

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.
- Do not evade a held lease by starting additional workers, using a second identity, or silently selecting a different lease directory.

## Task
1. Add opt-in `MUHAR_LEASE_DIR` and `MUHAR_TOTAL_SLOTS`. When both are set, acquire one OS-backed slot lease per worker as that worker starts — the first lease gates baseline/control preflight — and release each lease when its worker completes; no worker ever waits while holding a lease. A worker takes its lease before any of its own build work (including the schemata setup a worker with index > 0 performs), and once the shared mutation channel is drained, a worker still polling for a lease stops and finishes rather than holding the run open for a lease it no longer needs. The lease count follows the harness's effective worker count (`Harness.Run` clamps it to the mutation count, and to 1 without a workspace), and if that count exceeds `MUHAR_TOTAL_SLOTS`, clamp the effective workers to the total and echo the clamp on stderr rather than waiting forever or refusing. Unset configuration preserves current behaviour; partial, non-positive, or unreadable configuration refuses loudly.
2. Bound the sum of workers across independent processes, including -j 0 and explicit -j N. Waiting for a lease is observable in stderr and distinct from build/test duration. Never shard, skip, or reorder mutations to fit the budget.
3. Use the existing cross-platform exclusive-lock primitives (`deskkit.TryLockExclusive`/`UnlockFile` in `tools/desk/internal/deskkit`) and automatic close-on-exit release. Kill process groups and release every lease on normal return, harness failure, SIGINT and SIGTERM (Unix process groups as today; on Windows the existing top-process-only kill remains, as `process_windows.go` documents). Refuse multiple owners of one slot; stale files alone must not prove a live holder. Document the uncatchable-kill limitation: a SIGKILL or OOM kill releases the lock fds while suite process groups keep running leaseless, so the host total can be exceeded until they exit; inheriting the lease fd into suite children (`ExtraFiles`) is an implementer option to close that gap.
4. Extend cellctl with reviewed per-cell propagation of the two values through BOTH launch environments: the `--kind scrubbed` allowlist (`env.go`) and the non-scrubbed `deskLaunch` composition (`launch.go`, following the `repairAdmissionValue` model, including its dry-run echo). Classify both keys in `envkeys.go`'s registry — `TestEnvKeyRegistryIsComplete` enforces completeness. Document how independent roles share one host lease directory and total, and the explicit limitation for containers without a shared lock filesystem. Do not set a new default for existing cells.
5. Add `TestBudgetBoundsTotal`, `TestBudgetReleases`, and `TestBudgetBadConfig` using two independent processes and a fixture that records peak concurrent work. `TestBudgetBoundsTotal` must also prove that two processes whose combined demand exceeds the total both complete within a stated bound (no wedge) and that `-j N` beyond the total clamps with a stderr echo rather than waiting forever. Add `TestMuharBudgetEnv` proving cellctl propagates only explicitly configured values, in both the scrubbed allowlist and the non-scrubbed launch environment.
6. Document queue-wait measurement and operator recovery. Change every consumer entry (all five) to `fixed-here` in the implementation PR.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestBudget' -count=1 -v > "${TMPDIR:-/tmp}/dt57-budget.log" 2>&1 && grep -F -- '--- PASS: TestBudgetBoundsTotal ' "${TMPDIR:-/tmp}/dt57-budget.log" && grep -F -- '--- PASS: TestBudgetReleases ' "${TMPDIR:-/tmp}/dt57-budget.log" && grep -F -- '--- PASS: TestBudgetBadConfig ' "${TMPDIR:-/tmp}/dt57-budget.log"` | exit 0; all three named PASS lines; two processes never exceed the configured total, two over-demanding processes both complete within the bound, and cancellation releases every slot | check:ci +mutation +flow |
| 2 | `cd tools/desk && go test ./cmd/cellctl -run '^TestMuharBudgetEnv$' -count=1 -v > "${TMPDIR:-/tmp}/dt57-cell.log" 2>&1 && grep -F -- '--- PASS: TestMuharBudgetEnv ' "${TMPDIR:-/tmp}/dt57-cell.log"` | exit 0; named PASS; explicit values propagate in both the scrubbed allowlist and the non-scrubbed launch environment, and unset configuration stays unset | check:ci +flow |
| 3 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt57-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt57-anchors.log"` | exit 0; existing specs and mutation sets remain unchanged | check:ci +neighbour |
| 4 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-57-budget-concurrent-mutation-work-across-the-host.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/57 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 5 corroborated, 0 disproved, 0 unchecked' "$o"` | exit 0; runner, launcher and operator documentation are all in the delivering diff | check +flow +dereference |

## Evidence
<!-- Independent verifier appends exact commands, outputs, source/run identity, date and runner. This authoring PR claims no future implementation test executed. -->
### Verification, 2026-11-10, at merged source 612750a12406f19a96a83e6ea7687d794f772f9f

Verification-Table: delivery=18056ac181bcda0b96f2967950884cb63e64192a base=a7e2f320f8a76a9cee5cef98cecc40b7a432e782 original=a7e2f320f8a76a9cee5cef98cecc40b7a432e782

Independent verifier pass (claude-opus-5-5, strong tier, not the implementer) at merged main 612750a12406f19a96a83e6ea7687d794f772f9f under desk tools v1.0.81. Admission (deskdispatch --check-verifier) returned exit 0 before any row ran; the attestation it bound (medici-finance/assay#5955) is named on both witnesses below. Delivery, re-derived on the home: merge 18056ac181bcda0b96f2967950884cb63e64192a (#5922) is on HEAD's first-parent chain; its parents ecee257bd067 and 4ef4dffade4a have the single merge base a7e2f320f8a7. The Verify section hashes identically at that merge base, at the first parent, at the PR head, at the delivery and at this source, so the marker above names the merge base as both base and original. deskpathguard rederive reported 4 rows, 0 author-added. None of the 14 commits after the delivery changes a path under tools/desk/cmd/muhar/, tools/desk/cmd/cellctl/, docs/cellctl.md or this brief. Host: darwin/arm64, statusgen v1.0.81, git 2.56.0, KUBECONFIG=/dev/null, no GitHub access variable in the run environment.

Fail-first: no row is risk-bearing. The brief-wide risk block answers no to all four questions, and no Verify row carries a risk-bearing, live, mutating or end-to-end tag, so no fail-first witness is required and none was taken.

Rows 1-3 are check:ci rows. This host has no unshare network sandbox, so they ran in the container admission bracket, selected with --rows 1,2,3: host open, container run, host close, with no edit to the home between the steps. The open step used --module-cache with a new empty directory outside the home; it downloaded the 14 listed module roots against their go.sum and recorded Module-Cache sha256:7f8fa9800956. The container ran the release-pinned runner image ghcr.io/medici-finance/verify-runner:v1.0.81@sha256:03a2949dfe483f2645e14b85369c1923f1fb4710c7e5ea37a9b915d0073dde59; the v1.0.81 release's verify-runner-image.txt names that exact reference and its sha256 (bf09d07939b5) matches the release checksums. Its native linux/arm64 entry carries go1.27.3, and it was started with the flags the open step printed: --network none, --read-only, the module cache mounted read-only, GOPROXY=off, GOTOOLCHAIN=local and GOFLAGS=-mod=readonly. It ran as the host uid:gid. The home and its git directory were mounted read-only, the receipt read-only, the only writable bind mount was a new directory for the witness file, and /tmp (holding the Go build cache, HOME and the rows' log files) was a fresh tmpfs. No Verify row was attempted outside the bracket, and no attempt was discarded.

The container step admitted the home (its tree digest equalled the one recorded at the open step) and all three rows passed. The close step appended the Admission-Bracket line. statusgen verifyrun --bracket-digest on the witness then reported verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1. The block below is that file's bytes, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestBudget' -count=1 -v > "${TMPDIR:-/tmp}/dt57-budget.log" 2>&1 && grep -F -- '--- PASS: TestBudgetBoundsTotal ' "${TMPDIR:-/tmp}/dt57-budget.log" && grep -F -- '--- PASS: TestBudgetReleases ' "${TMPDIR:-/tmp}/dt57-budget.log" && grep -F -- '--- PASS: TestBudgetBadConfig ' "${TMPDIR:-/tmp}/dt57-budget.log"` | pass exit=0 sandbox=container-netns | sha256:4854f02aacac | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/cellctl -run '^TestMuharBudgetEnv$' -count=1 -v > "${TMPDIR:-/tmp}/dt57-cell.log" 2>&1 && grep -F -- '--- PASS: TestMuharBudgetEnv ' "${TMPDIR:-/tmp}/dt57-cell.log"` | pass exit=0 sandbox=container-netns | sha256:f20a692f12e1 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt57-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt57-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:11ed77d4bb53 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6
Container-Image: sha256:03a2949dfe483f2645e14b85369c1923f1fb4710c7e5ea37a9b915d0073dde59
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#5955 run=30090ecb45f25241f56d59aefcf17d193bac8d2b409e369dc801dff9e6cfd4ad source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desk-tools/brief-57-budget-concurrent-mutation-work-across-the-host.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=30090ecb45f25241f56d59aefcf17d193bac8d2b409e369dc801dff9e6cfd4ad source=612750a12406f19a96a83e6ea7687d794f772f9f tree=sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6 witness=sha256:fbc8386fdf50f744ac9cfcea7fb4d3d749fdd4f4f391d2805e9c3758cb286755 opened=2026-11-10T05:03:23Z closed=2026-11-10T05:04:40Z before=receipt after=check-verifier

Row 4 is a host row (check, not check:ci). It ran on the unedited home after the bracket closed, with statusgen verifyrun --rows 4 --dry-run, which printed the table and attestation line below (reproduced unchanged; nothing was written to the home). The row needs two inputs. DELIVERY_HEAD was set to 612750a12406f19a96a83e6ea7687d794f772f9f, the admitted source this pass verifies; the row's own guard requires it to equal the home's HEAD, so the delivery merge itself cannot be named there in a post-merge run. DELIVERY_BASE was set to ecee257bd06719468d82522013ff160272e85c3e, the actual first parent of the delivering merge, which is an ancestor of the source. Because no later commit touches the five consumer files, the base..source diff routes them exactly as the delivering diff does. A supplemental hand capture (no acceptance credit) of the same statusgen command printed "summary: 5 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing"; the same command with --delivery 18056ac181bc added reported the immutable delivered snapshot ecee257bd067..18056ac181bc and the same summary, all five entries CORROBORATED. statusgen --lint carries a NOTICE on row 4's definition, the same at this source with or without this Evidence: its consumers command pins a fixed base but no --delivery snapshot. The delivery-pinned capture above shows that gap changes no count here.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-57-budget-concurrent-mutation-work-across-the-host.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/57 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 5 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:95e9eac78778 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5955 run=30090ecb45f25241f56d59aefcf17d193bac8d2b409e369dc801dff9e6cfd4ad source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desk-tools/brief-57-budget-concurrent-mutation-work-across-the-host.md model=claude-opus-5-5 tier=strong

Observed result per row:
- Row 1: pass exit=0 sandbox=container-netns, output sha256:4854f02aacac. The command ends in three grep -F checks, so exit 0 means the PASS lines for TestBudgetBoundsTotal, TestBudgetReleases and TestBudgetBadConfig were all present. TestBudgetBoundsTotal runs two independent muhar child processes (-j 7 and -j 0) against one lease directory with MUHAR_TOTAL_SLOTS=2, requires both to finish within 15 seconds, requires the recorded peak concurrent work to equal 2, requires the -j 7 clamp echo on stderr, and requires a queue-wait line.
- Row 2: pass exit=0 sandbox=container-netns, output sha256:f20a692f12e1. Exit 0 of the trailing grep means the named PASS line for TestMuharBudgetEnv was present.
- Row 3: pass exit=0 sandbox=container-netns, output sha256:11ed77d4bb53. Exit 0 of the trailing grep means the named PASS line for TestSpecAnchorsLand was present.
- Row 4: pass exit=0, output sha256:95e9eac78778. The brief carries no colon-prefixed follow-up marker, and the consumers check reported 5 corroborated, 0 disproved, 0 unchecked.

Risk-bearing values. The enumeration covers the delivery diff ecee257bd067..18056ac181bc (the muhar budget, harness, runner, selection and main sources, the cellctl budget, env, launch and envkeys sources, and the delivered tests) and the brief's Task list; the lines below were re-read at this source:
- the non-positive refusal, total <= 0 @ tools/desk/cmd/muhar/budget.go:35
- the slot lock name, "slot-%d.lock" with mode 0600 @ tools/desk/cmd/muhar/budget.go:47 and tools/desk/cmd/muhar/budget.go:105
- the lease poll interval, 25 * time.Millisecond @ tools/desk/cmd/muhar/budget.go:126
- the interrupt exit code, code = 130 @ tools/desk/cmd/muhar/main.go:334 and return 130 @ tools/desk/cmd/muhar/main.go:489
- both keys classed envMachine in envGroupHarness @ tools/desk/cmd/cellctl/envkeys.go:101 and tools/desk/cmd/cellctl/envkeys.go:103
- test-only values: MUHAR_TOTAL_SLOTS=2 @ tools/desk/cmd/muhar/budget_test.go:117, the 15 * time.Second completion bound @ tools/desk/cmd/muhar/budget_test.go:139, the expected peak=2 and calls=38 @ tools/desk/cmd/muhar/budget_test.go:154, 8 mutations @ tools/desk/cmd/muhar/budget_test.go:106

All are reversible, and the item is marked irreversible: no. Ranked by what breaks if wrong: the slot lock name (a different name in another binary would let two processes lock different files and exceed the total), then the non-positive refusal, then the key classification; the poll interval, exit code and test-only values are operational or test knobs and rank last.

RISK-VALUE: DERIVED — slot lock name = "slot-%d.lock" @ tools/desk/cmd/muhar/budget.go:47 — admission is decided only by an exclusive OS lock on a stable file per slot index 0..total-1, so every process sharing the directory must compute the same names. The same format string is used at validation (line 47) and at acquisition (line 105), the file is never unlinked (the type comment at lines 17-19 says why), and the README states that independent roles on one host must use the same directory and total; TestBudgetBoundsTotal observes a peak of exactly 2 across two processes under a total of 2.
RISK-VALUE: DERIVED — non-positive refusal = total <= 0 @ tools/desk/cmd/muhar/budget.go:35 — the brief requires partial, non-positive or unreadable configuration to refuse loudly. A total of 0 or less would admit no worker and wait forever, so refusing it, together with a missing key or a non-integer value, is exactly the stated contract; the error is printed and muhar exits 1 before any work.

**VERIFY: PASS** — 4 of 4 rows pass with admitted witnesses at merged source 612750a12406. Rows 1-3 passed network-off inside a closed admission bracket (verdict=match), and row 4 passed on the host as written. No row is risk-bearing, so no fail-first witness is required. The gate is model, and all risk answers are no.

### Verification, 2026-11-12, at merged source d10d7fd1f2756b2b82b6d251dbe970b1ec0efb16

Verification-Table: delivery=18056ac181bcda0b96f2967950884cb63e64192a base=a7e2f320f8a76a9cee5cef98cecc40b7a432e782 original=a7e2f320f8a76a9cee5cef98cecc40b7a432e782

Independent verifier pass (claude-opus-5-5, strong tier, not the implementer) at merged main d10d7fd1f2756b2b82b6d251dbe970b1ec0efb16 under desk tools v1.0.82. Admission (deskdispatch --check-verifier) returned exit 0 before any row ran; the attestation it bound (medici-finance/assay#6241) is named on both witnesses below. Delivery, re-derived on the home: merge 18056ac181bcda0b96f2967950884cb63e64192a (#5922) is on HEAD's first-parent chain; its parents ecee257bd067 and 4ef4dffade4a have the single merge base a7e2f320f8a7. The Verify section hashes identically at that merge base, at both parents, at the delivery and at this source, so the marker above names the merge base as both base and original. deskpathguard rederive reported 4 rows, 0 author-added. Since the earlier pass at 612750a12406, later merges changed muhar's harness, main, runner, selection and process sources, added a state module, rewrote one mutation anchor in the muhar budget mutation set (the runner exit classification it targets moved to resourceSignal), added one unrelated key to the cellctl envkeys registry, and extended the muhar README and docs/cellctl.md; budget.go and both budget tests are unchanged. This pass re-runs every row against that newer source. Host: darwin/arm64, statusgen v1.0.82, git 2.56.0, KUBECONFIG=/dev/null, no GitHub access variable in the run environment.

Fail-first: no row is risk-bearing. The brief-wide risk block answers no to all four questions, and no Verify row carries a risk-bearing, live, mutating or end-to-end tag, so no fail-first witness is required and none was taken.

Rows 1-3 are check:ci rows. This host has no unshare network sandbox, so they ran in the container admission bracket, selected with --rows 1,2,3: host open, container run, host close, with no edit to the home between the steps. The open step used --module-cache with a new empty directory outside the home; it downloaded the 14 listed module roots against their go.sum and recorded Module-Cache sha256:7f8fa9800956. The container ran the release-pinned runner image ghcr.io/medici-finance/verify-runner:v1.0.82@sha256:43db6da8cd159b2c822af62bb859f1a305637c066ea1a750df16dfd5d7b99563; the v1.0.82 release's verify-runner-image.txt names that exact reference and its sha256 (9348566bea37) matches the release checksums. Inside it, statusgen reports v1.0.82, go1.27.3 linux/arm64 and git 2.56.0. It was started with the flags the open step printed: --network none, --read-only, the module cache mounted read-only, GOPROXY=off, GOTOOLCHAIN=local and GOFLAGS=-mod=readonly. It ran as the host uid:gid. The home and its git directory were mounted read-only, the receipt read-only, the only writable bind mount was a new directory for the witness file, and /tmp (holding the Go build cache, HOME and the rows' log files) was a fresh tmpfs. No Verify row was attempted outside the bracket, and no attempt was discarded.

The container step admitted the home (its tree digest equalled the one recorded at the open step) and all three rows passed. The close step appended the Admission-Bracket line. statusgen verifyrun --bracket-digest on the witness then reported verdict=match with span-lines=tree-digest:1,container-image:1,attestation:1. The block below is that file's bytes, unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test ./cmd/muhar -run '^TestBudget' -count=1 -v > "${TMPDIR:-/tmp}/dt57-budget.log" 2>&1 && grep -F -- '--- PASS: TestBudgetBoundsTotal ' "${TMPDIR:-/tmp}/dt57-budget.log" && grep -F -- '--- PASS: TestBudgetReleases ' "${TMPDIR:-/tmp}/dt57-budget.log" && grep -F -- '--- PASS: TestBudgetBadConfig ' "${TMPDIR:-/tmp}/dt57-budget.log"` | pass exit=0 sandbox=container-netns | sha256:d27c9a9fa2bd | 2026-11-12 | assay-verifier-app[bot] @ d10d7fd1f275 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `cd tools/desk && go test ./cmd/cellctl -run '^TestMuharBudgetEnv$' -count=1 -v > "${TMPDIR:-/tmp}/dt57-cell.log" 2>&1 && grep -F -- '--- PASS: TestMuharBudgetEnv ' "${TMPDIR:-/tmp}/dt57-cell.log"` | pass exit=0 sandbox=container-netns | sha256:f20a692f12e1 | 2026-11-12 | assay-verifier-app[bot] @ d10d7fd1f275 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `cd tools/desk && go test ./cmd/muhar -run '^TestSpecAnchorsLand$' -count=1 -v > "${TMPDIR:-/tmp}/dt57-anchors.log" 2>&1 && grep -F -- '--- PASS: TestSpecAnchorsLand ' "${TMPDIR:-/tmp}/dt57-anchors.log"` | pass exit=0 sandbox=container-netns | sha256:72b8db876554 | 2026-11-12 | assay-verifier-app[bot] @ d10d7fd1f275 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:9d575d81778df39a1804c49056099ccef0011403da6d98578900f884a2afaaa1
Container-Image: sha256:43db6da8cd159b2c822af62bb859f1a305637c066ea1a750df16dfd5d7b99563
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6241 run=3229854876d2dcb52fa0f71dfa4cb928d4557c30b03a4ecfbc35f6bcc2218b73 source=d10d7fd1f2756b2b82b6d251dbe970b1ec0efb16 brief=docs/streams/desk-tools/brief-57-budget-concurrent-mutation-work-across-the-host.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=3229854876d2dcb52fa0f71dfa4cb928d4557c30b03a4ecfbc35f6bcc2218b73 source=d10d7fd1f2756b2b82b6d251dbe970b1ec0efb16 tree=sha256:9d575d81778df39a1804c49056099ccef0011403da6d98578900f884a2afaaa1 witness=sha256:b81578cf98606bc946412ab128f6737c77227b010835ba4c3ed4291ae2c4aa6f opened=2026-11-12T00:58:38Z closed=2026-11-12T00:59:39Z before=receipt after=check-verifier

Row 4 is a host row (check, not check:ci). It ran on the unedited home after the bracket closed, with statusgen verifyrun --rows 4 --dry-run, which printed the table and attestation line below (reproduced unchanged; nothing was written to the home). The row needs two inputs. DELIVERY_HEAD was set to d10d7fd1f2756b2b82b6d251dbe970b1ec0efb16, the admitted source this pass verifies; the row's own guard requires it to equal the home's HEAD, so the delivery merge itself cannot be named there in a post-merge run. DELIVERY_BASE was set to ecee257bd06719468d82522013ff160272e85c3e, the actual first parent of the delivering merge, which is an ancestor of the source. Later commits do touch three of the five consumer files (envkeys.go, the muhar README and docs/cellctl.md), so the base..source diff is wider than the delivering diff. A supplemental hand capture (no acceptance credit) with --delivery 18056ac181bcda0b96f2967950884cb63e64192a added pins the immutable delivered snapshot ecee257bd067..18056ac181bc and printed "summary: 5 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing", all five entries CORROBORATED. So the delivering diff alone corroborates every consumer, and the wider range changes no count. statusgen --lint at this source still carries the NOTICE on row 4's definition that the earlier pass recorded (its consumers command pins a fixed base but no --delivery snapshot), plus an outcome-absent NOTICE on the frontmatter; both are notices, and the delivery-pinned capture above shows the first changes no count here.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `test -n "${DELIVERY_HEAD:-}" && test -n "${DELIVERY_BASE:-}" && test "$(git rev-parse HEAD)" = "$DELIVERY_HEAD" && git merge-base --is-ancestor "$DELIVERY_BASE" "$DELIVERY_HEAD" && ! grep -q -e '[:] follow-up ' docs/streams/desk-tools/brief-57-budget-concurrent-mutation-work-across-the-host.md && o=$(mktemp) && trap 'rm -f "$o"' 0 && statusgen --root . --consumers --brief desk-tools/57 --base "$DELIVERY_BASE" > "$o" 2>&1 && grep -F -e 'summary: 5 corroborated, 0 disproved, 0 unchecked' "$o"` | pass exit=0 | sha256:95e9eac78778 | 2026-11-12 | assay-verifier-app[bot] @ d10d7fd1f275 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6241 run=3229854876d2dcb52fa0f71dfa4cb928d4557c30b03a4ecfbc35f6bcc2218b73 source=d10d7fd1f2756b2b82b6d251dbe970b1ec0efb16 brief=docs/streams/desk-tools/brief-57-budget-concurrent-mutation-work-across-the-host.md model=claude-opus-5-5 tier=strong

Observed result per row:
- Row 1: pass exit=0 sandbox=container-netns, output sha256:d27c9a9fa2bd. The command ends in three grep -F checks, so exit 0 means the PASS lines for TestBudgetBoundsTotal, TestBudgetReleases and TestBudgetBadConfig were all present. TestBudgetBoundsTotal runs two independent muhar child processes (-j 7 and -j 0) against one lease directory with MUHAR_TOTAL_SLOTS=2, requires both to finish within 15 seconds, requires the recorded peak concurrent work to equal 2, and requires the -j 7 clamp echo on stderr.
- Row 2: pass exit=0 sandbox=container-netns, output sha256:f20a692f12e1. Exit 0 of the trailing grep means the named PASS line for TestMuharBudgetEnv was present.
- Row 3: pass exit=0 sandbox=container-netns, output sha256:72b8db876554. Exit 0 of the trailing grep means the named PASS line for TestSpecAnchorsLand was present.
- Row 4: pass exit=0, output sha256:95e9eac78778. The brief carries no colon-prefixed follow-up marker, and the consumers check reported 5 corroborated, 0 disproved, 0 unchecked.

Risk-bearing values. The enumeration covers the delivery diff ecee257bd067..18056ac181bc (the muhar budget, harness, runner, selection and main sources, the cellctl budget, env, launch and envkeys sources, and the delivered tests), the brief's Task list, and the later changes to those paths listed above; the lines below were re-read at this source:
- the non-positive refusal, total <= 0 @ tools/desk/cmd/muhar/budget.go:35
- the slot lock name, "slot-%d.lock" with mode 0600 @ tools/desk/cmd/muhar/budget.go:47 and tools/desk/cmd/muhar/budget.go:105
- the lease poll interval, 25 * time.Millisecond @ tools/desk/cmd/muhar/budget.go:126
- the interrupt exit code, code = 130 @ tools/desk/cmd/muhar/main.go:339 and return 130 @ tools/desk/cmd/muhar/main.go:526
- both keys classed envMachine in envGroupHarness @ tools/desk/cmd/cellctl/envkeys.go:101 and tools/desk/cmd/cellctl/envkeys.go:103
- test-only values: MUHAR_TOTAL_SLOTS=2 @ tools/desk/cmd/muhar/budget_test.go:117, the 15 * time.Second completion bound @ tools/desk/cmd/muhar/budget_test.go:139, the expected peak=2 and calls=38 @ tools/desk/cmd/muhar/budget_test.go:154

All are reversible, and the item is marked irreversible: no. Ranked by what breaks if wrong: the slot lock name (a different name in another binary would let two processes lock different files and exceed the total), then the non-positive refusal, then the key classification; the poll interval, exit code and test-only values are operational or test knobs and rank last.

RISK-VALUE: DERIVED — slot lock name = "slot-%d.lock" @ tools/desk/cmd/muhar/budget.go:47 — admission is decided only by an exclusive OS lock on a stable file per slot index 0..total-1, so every process sharing the directory must compute the same names. The same format string is used at validation (line 47) and at acquisition (line 105), budget.go is unchanged since the delivery, and TestBudgetBoundsTotal observes a peak of exactly 2 across two independent processes under a total of 2.
RISK-VALUE: DERIVED — non-positive refusal = total <= 0 @ tools/desk/cmd/muhar/budget.go:35 — the brief requires partial, non-positive or unreadable configuration to refuse loudly. A total of 0 or less would admit no worker and wait forever, so refusing it, together with a missing key, an empty directory or a non-integer value on the same line, is exactly the stated contract; TestBudgetBadConfig covers 0, -2, a non-integer and each partial pair.

**VERIFY: PASS** — 4 of 4 rows pass with admitted witnesses at merged source d10d7fd1f275. Rows 1-3 passed network-off inside a closed admission bracket (verdict=match), and row 4 passed on the host as written. No row is risk-bearing, so no fail-first witness is required. The gate is model, and all risk answers are no.

## Failure map
- Two runs each exceed the host total: TestBudgetBoundsTotal records the excess and fails.
- Cancellation leaves capacity wedged: TestBudgetReleases fails.
- A bad or partial configuration silently disables budgeting: TestBudgetBadConfig fails.
- cellctl invents a default or drops the explicit setting, or the values reach only the scrubbed allowlist: TestMuharBudgetEnv fails.
- Two over-demanding processes wedge instead of completing: TestBudgetBoundsTotal's completion bound fails.
- The runner works but launcher/docs are omitted: row 4 fails.

## Review
Gate: model. Check lock ownership, process-group cancellation and actual cross-process coverage; per-process -j tests alone are insufficient.
