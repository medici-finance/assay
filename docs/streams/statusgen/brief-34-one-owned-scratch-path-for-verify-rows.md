---
brief: assay:assay:statusgen:34
title: One owned scratch path for Verify rows
why: "Verify rows still allocate scratch in the checkout or never remove it, and each instance has been fixed one by one. This brief makes one owned scratch idiom the only one a new or changed row may use, has the lint enforce it, and has verifyrun own the row temp directory and catch untracked, non-ignored in-tree writes the lint cannot see (ignored paths are a stated limit)."
wave: 1
depends: []
unblocks: []
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [2789]
schema: brief-v2
outcome: none
version: 1
authored: 2026-10-18 by statusgen newbrief
sources:
  - "medici-finance/assay#2789 — the issue: Verify rows that write scratch into the tree they verify, or leave it behind"
  - "medici-finance/assay#2803 — merged 2026-10-15; corrected seven rows one by one (build-less-brittle 04, 05, 06, 09, 11 row 17, 12, 13)"
  - "https://github.com/medici-finance/assay/issues/2789#issuecomment-6237133837 — design note: make external owned scratch the single authoring path for Verify rows"
  - docs/contracts.md — S-worktree (the worktree owner and the workspace module)
  - docs/workspace-callers.md — the caller census; it does not read docs/
  - spec/cell-workspaces-v1.md — section 8 (effects, owners, the snapshot port) and section 9 (dispositions)
  - docs/verify-row-strength.md — the Verify-row rule catalogue
  - freshness-checked 2026-10-18 @ 83c6988d0 (origin/main)
exec-tier: strong
exec-tier-why: Two independent enforcement layers, a base-relative differential, a runtime filesystem observation and corrections to six other briefs need negative-path proofs at more than one site.
domain: complicated
consumers:
  - "statusgen/verifyrows.go and statusgen/strength.go (two new row rules and the base-relative differential): follow-up statusgen/34"
  - "statusgen/enforcementstatus.go (registry entries for the two new tags): follow-up statusgen/34"
  - "statusgen/verifyrun.go (run-owned row temp directory and the per-row in-tree write check): follow-up statusgen/34"
  - "plugins/assay/skills/author-brief/SKILL.md and .cursor/skills/author-brief/SKILL.md (generated enforcement block, synced by tools/skillslint): follow-up statusgen/34"
  - "docs/brief-rules.md (new rule 50), docs/brief-template.md (shell notes) and docs/verify-row-strength.md (catalogue entries): follow-up statusgen/34"
  - "docs/streams/measured-status/brief-03-commsloop-risk-derivation.md (Verify row 5): follow-up statusgen/34"
  - "docs/streams/build-less-brittle/brief-08-hotspot-metric-and-brittle-mark.md (Verify row 6): follow-up statusgen/34"
  - "docs/streams/build-less-brittle/brief-10-fitness-functions.md (Verify row 7): follow-up statusgen/34"
  - "docs/streams/build-less-brittle/brief-11-regression-test-ratchet.md (Verify rows 3 and 4): follow-up statusgen/34"
  - "docs/streams/derived-board/brief-09-incremental-reconcile.md (Verify row 7): follow-up statusgen/34"
  - "docs/streams/forge-neutral/brief-34-deskread-ci-workflow-token-transport.md (Verify row 16): follow-up statusgen/34"
  - "docs/workspace-callers.md: follow-up statusgen/34 (a row only if the census discovers the new run-scratch site)"
  - "tools/desk/internal/cellscratch/: out-of-scope (its metadata-free snapshots cannot serve rows that run git clone or checkout)"
  - "workspace/ and tools/desk/cmd/deskwt/: out-of-scope (they own worktrees; this brief adds no allocator and no worktree effect)"
---

# Brief 34 — One owned scratch path for Verify rows

## Context

files:
- `statusgen/verifyrows.go`
- `statusgen/verifyrows_test.go`
- `statusgen/strength.go`
- `statusgen/enforcementstatus.go`
- `statusgen/enforcementstatus_test.go`
- `statusgen/verifyrun.go`
- `statusgen/verifyrun_test.go`
- `statusgen/obligationderivation.go`
- `statusgen/gitbatch.go`
- statusgen/scratchrow-mutations.json (planned)
- statusgen/testdata/scratchrows/ (planned)
- `plugins/assay/skills/author-brief/SKILL.md`
- `.cursor/skills/author-brief/SKILL.md`
- `docs/brief-rules.md`
- `docs/brief-template.md`
- `docs/verify-row-strength.md`
- `docs/workspace-callers.md`
- `docs/streams/measured-status/brief-03-commsloop-risk-derivation.md`
- `docs/streams/build-less-brittle/brief-08-hotspot-metric-and-brittle-mark.md`
- `docs/streams/build-less-brittle/brief-10-fitness-functions.md`
- `docs/streams/build-less-brittle/brief-11-regression-test-ratchet.md`
- `docs/streams/derived-board/brief-09-incremental-reconcile.md`
- `docs/streams/forge-neutral/brief-34-deskread-ci-workflow-token-transport.md`
- changelog/statusgen-owned-row-scratch.md (planned)

facts (source and date in frontmatter):
- The defect is a class, not one row. At 83c6988d0 the Verify sections of `docs/streams/*/brief-*.md` hold two shapes. The in-tree shape has one site: measured-status/03 row 5 allocates its clone under the working directory with a mktemp template rooted at `$PWD`. The unowned shape has many: 247 Verify rows call mktemp with no trap and no rm. Among them, six copy or clone a tree and never remove it: build-less-brittle/08 row 6 (a shallow clone), build-less-brittle/10 row 7 and build-less-brittle/11 rows 3 and 4 (`cp -R .` copies), derived-board/09 row 7 (a full clone and a built binary under the temp root) and forge-neutral/34 row 16 (a shared clone and a built binary; it refuses a temp root under the checkout but never removes its directory).
- medici-finance/assay#2803 fixed seven rows one by one. build-less-brittle/11 row 17 now allocates under the temp root and removes its directory. Nothing stops the next row from repeating either shape: no lint rule names in-tree or unowned scratch, and verifyrun runs every row with the caller's own environment and the checkout as its working directory.
- The existing `verify-row-portability` tag (advisory) flags a hardcoded `/tmp` and exempts a `${TMPDIR:-/tmp}` fallback. It says nothing about where a mktemp template is rooted or whether the directory is removed.
- verifyrun runs each row with `bash -o pipefail -c`, the working directory set to the root, and `plan.rowEnv()` as the environment: the caller's own, or `admittedRowEnv` for an admitted verifier run. `treeSHA` already marks a witness `+dirty` when `git status --porcelain` is non-empty, but it reads the tree once, so a row that writes into the tree and the rows after it share one mark, and nothing names the row that wrote.
- The existing scratch and worktree owners do not fit. `cellscratch` serves metadata-free snapshots of an exact commit, and spec section 8 routes only checks that need immutable files rather than a Git checkout to it; the named rows run git clone, checkout and worktree commands. The workspace module and `deskwt` own worktrees. The caller census (`docs/workspace-callers.md`) does not read `docs/`, so Verify rows are not census sites. This brief therefore adds no allocator. The operating system's mktemp under the temp root is the allocator, and the change is about which root and who removes the directory.
- `docs/brief-rules.md` ends at rule 49, so the new rule is 50.
- The idiom below was run on macOS `/bin/bash` 3.2 and on a current bash, both with pipefail. It keeps the row's own exit status (1 stays 1, 0 stays 0, an explicit exit 3 stays 3, and 0 stays 0 when allocation never ran) and removes the directory on every path, including failure.

layering: lint rules in the existing Verify-row lint (verifyrows.go, strength.go), registered in the existing enforcement registry; runtime ownership in the existing verifyrun owner. No new verb, flag, module or service.

single-point-of-failure: the authoring lint (layer 1) is the first control; verifyrun's run-owned temp directory and per-row in-tree write check (layer 2) sit behind it on a different signal (an observed filesystem change at run time, not command text at authoring time), and the existing `+dirty` witness mark (layer 3) sits behind both.

design-fit:
  owner: statusgen Verify-row lint and verifyrun
  contract: one documented scratch idiom (brief rule 50), two registered lint tags
  retires: [one-by-one row corrections of in-tree and unowned scratch]
  weight: verbs +0; flags +0; lint tags +2; no new module, allocator or service
  why-add: The class keeps recurring because nothing enforces the safe form; both enforcement points already exist and own this decision.
  reuse-check: statusgen already has a base-relative gate, `--diff-base` in `statusgen/difflint.go`. It re-runs `--lint` on a materialised base worktree, compares the two PROBLEM sets and demotes problems present at the base to NOTICEs; when the base cannot be resolved it demotes nothing. Open work on main, statusgen/27 (todo), plans to promote newly introduced actionable NOTICEs on changed briefs. The implementation PR states which of these it reuses for the per-Command-cell differential and, where it does not use `--diff-base`, why; whichever of this brief and statusgen/27 lands second reconciles with the first rather than adding a second base resolver.

## Read first

Read `statusgen/verifyrows.go` (the rule constants, `rowFindings`, `verifyRowTable`, `verifyCommand` and `unfailableRowAudit`), `statusgen/strength.go`, the `lintRuleRegistry` in `statusgen/enforcementstatus.go`, `branchChangedSet` in `statusgen/obligationderivation.go`, `gitShowObject` in `statusgen/gitbatch.go`, and `runVerifyrun`, `resolveShellPlan`, `runVerifyCommandWith` and `treeSHA` in `statusgen/verifyrun.go`. Read the issue's comments, including the design note in sources. Re-run the census in Verify row 5 against fresh main before starting. If the counts in Context no longer hold, report NEEDS_CONTEXT.

## Ground rules

- Feature branch and draft PR only. Never push to main, merge, release or dispatch a workflow.
- Stop at `implemented`. A different, non-implementing identity owns verification and Evidence.
- Use temporary fixture repositories. No network.
- Do not change the lifecycle cell or any Evidence row of the six corrected briefs. Historical Evidence stays byte-unchanged; only the named Verify rows and each brief's `version:` change.
- If anything is unclear or contradicts repo state, report NEEDS_CONTEXT. Do not guess.

## Interface contract

The sanctioned idiom, which rule 50 states and every new or changed sh row uses when it needs scratch:

    d=; trap 'test -n "$d" && rm -rf "$d"' EXIT; d=$(mktemp -d "${TMPDIR:-/tmp}/<slug>.XXXXXX") && …

The trap is set before allocation, so a failure at any later step still removes the directory, and the row's exit status is preserved. A single temp file uses the same form with `rm -f`. The form works the same by hand and under verifyrun.

Layer 1, lint. These rules apply to sh rows only; cmd and pwsh rows are out of scope.

- `scratch-in-tree` fires when:
  - a mktemp template is rooted at the working directory (`$PWD`, `${PWD}`, `$(pwd)` or a backticked pwd), or is relative, meaning it does not start with `/`, `$` or `~`;
  - `-p` or `--tmpdir` names `.` or a working-directory form; or
  - the row sets `TMPDIR=` to one of those.

  A mktemp with no template and no `-p` uses the temp root and does not fire.
- `scratch-unowned` fires when a mktemp result is not assigned to a variable, or is assigned to one that no `trap … EXIT` (or `trap … 0`) in the same Command cell removes with `rm -rf` or `rm -f`.
- Severity is differential against the merge-base, which is `mergeBaseExact(root, remoteMainRef)`, the value `branchChangedSet` already uses. A row whose Command cell is not byte-present in the base blob of the same brief is added or changed on this branch, and its findings are PROBLEMs. Every other finding is a NOTICE, and `scratch-unowned` NOTICEs are folded into one count line.
- A brief that is new on this branch has no base blob. When the base resolves but the brief's path is absent there (and rename detection, `detectRename` in `statusgen/humanstamp.go`, finds no earlier path), every row of that brief is added, so every finding is a PROBLEM. Any other failure to read a blob at a resolved base is treated the same way: the fail-safe direction is the stricter one, never a downgrade. This is the case the rule exists for, because a newly authored brief is the main way new scratch rows enter the tree.
- Only when the base itself cannot be resolved (`branchChangedSet` returns false, or `mergeBaseExact` returns empty) is every finding a NOTICE that says the differential could not be checked. It is never a silent pass.
- A cell uses one `trap … EXIT`, naming every scratch variable it allocates, because a second `trap … EXIT` in the same cell replaces the first. The lint credits only the last `trap … EXIT` (or `trap … 0`) in the cell. A scratch variable is assigned once: a later reassignment would turn `rm -rf "$d"` into a delete of whatever the variable then holds. The lint does not detect reassignment (see the limits below), so rule 50 states it as an authoring requirement.
- Honest limits: the lint reads command text, so a path reached through a variable (`p=$(pwd); x=$p/out`) is not caught, and a reassigned scratch variable is not caught. Layer 2 exists for the first case.
- Both tags are registered in `lintRuleRegistry` as fatal, with the description stating that they are fatal for added or changed rows and NOTICEs otherwise.

Layer 2, verifyrun, host arm:

- Before the first row, make the root absolute and resolve it with `filepath.EvalSymlinks` (the `--root` value is used as given today, and rows pass `--root .`), create one run-owned temp directory with `os.MkdirTemp`, and resolve that with `filepath.EvalSymlinks`. Containment is a path-segment comparison on the two resolved paths (equal, or the root plus a separator as a prefix), never a bare string prefix and never the `..` prefix test in `relDisplayPath`. If the temp directory is contained in the root, or either resolution fails, the run is refused as could-not-run with the verifyrun could-not-run exit and no row runs. A failed resolution is never read as "not under the root".
- Export the directory as `TMPDIR` in the row environment (`plan.env`, preserving `admittedRowEnv` when the run is admitted), and remove it when the run ends.
- Read untracked, non-ignored paths with `git status --porcelain -z --untracked-files=all` before and after each row. The `--untracked-files=all` form lists every untracked file inside an ordinary untracked directory; the default form collapses such a directory into one entry, so a directory left over from an earlier run would hide every later write into it. The form does not look inside an untracked nested Git repository: it prints one entry ending in `/` for the whole repository, before and after a row alike. A leftover in-tree clone has exactly this shape and would hide every later write into it. So a `??` entry ending in `/` in the before-read makes the row record could-not-run for the in-tree check, with a note naming the entry, never clean. A nested repository the row itself creates is a new path in the after-read and is blamed as below. A row that leaves a path the before-read did not have records `fail`, with a note naming the paths, whatever its own exit status. A path that existed before the row is never blamed on it. If either read fails (for example on a leftover `index.lock`), the row records could-not-run for the check, never clean, and the note says the in-tree check could not be made.
- Honest limit: ignored paths are not read. A row that writes an artifact to an ignored in-tree path (a built binary, a cache directory) is not caught by layer 2, and the `+dirty` mark does not see it either, so such an artifact can survive into the next run. Layer 2 catches untracked, non-ignored in-tree writes only.
- Out of scope: `--in-container`, `--sandbox` re-execution and the fail-first post-row check keep their current behavior. Each states its gap in the run output rather than claiming the check.

Layer 3, the existing `+dirty` witness mark in `treeSHA`, is unchanged. It reads plain `git status --porcelain`, so it omits ignored paths as well.

## Task

1. Add rule 50 to `docs/brief-rules.md`, in the row-runner discipline section after rule 29. It states the idiom, the two tags, the differential severity and the honest limit. It also says that a row needing a Git checkout uses a `git worktree add` and `git worktree remove` pair whose path is under the owned directory. It cites medici-finance/assay#2789. Add the idiom to the shell notes in `docs/brief-template.md`, and add the two tags to `docs/verify-row-strength.md`.
2. Implement both rules in the Verify-row lint, sh rows only, using `verifyCommand` to lift the command. Implement the base-relative differential with `branchChangedSet` (to learn whether the brief changed) and `gitShowObject` (to read the base blob), taking the base from `mergeBaseExact(root, remoteMainRef)`. `mergeBaseExact` is the sanctioned choke point and many production callers use it; `TestMergeBaseChokePoint` forbids running `git merge-base` outside it, so never run `git merge-base` directly and never hand a by-name ref to git. Register both tags in `lintRuleRegistry`, then run `go run ./tools/skillslint --sync` to regenerate the enforcement block in both author-brief skill copies. Never hand-edit that block.
3. Add fixture briefs under statusgen/testdata/scratchrows/ in at least two streams. Each holds an in-tree row, an unowned tree-copy row, a correctly owned row that uses the idiom, a no-template mktemp row with a trap, and a `git worktree add` plus `git worktree remove --force` row with no mktemp, and a cell with two `trap … EXIT` lines where the last does not remove the first variable (`scratch-unowned`). Add `TestScratchRowLintClassifies` (planned), covering the per-row classification at both sites, and `TestScratchRowLintDifferential` (planned), covering the differential: added or changed rows are PROBLEMs, unchanged rows are NOTICEs, a brief absent at a resolved base (new on the branch) gets PROBLEMs for every row, and an unresolvable base gives degraded NOTICEs, never a clean result.
4. Implement layer 2 in `runVerifyrun` for the host arm. Add `TestVerifyrunOwnsRowScratch` (planned), which drives the built binary over a fixture repository and covers:
   - a row that writes into the tree through variable indirection, so it passes the layer-1 lint, records `fail` naming the path;
   - a row that calls mktemp with no template allocates under the run-owned directory, and that directory is gone after the run;
   - a temp root that resolves under the root is refused as could-not-run, including a `TMPDIR` that reaches the root through a symlink outside it, a root given relatively (`--root .`) and a root given through a symlink;
   - a resolution error on either side refuses the run as could-not-run, and a failed before-read or after-read (for example a leftover `index.lock`) records could-not-run, never clean;
   - a path that was untracked before the run is not blamed, but a new file written inside an untracked directory that existed before the run is, and the row records `fail`;
   - an untracked nested Git repository present before the row (a leftover in-tree clone) makes the row record could-not-run for the in-tree check, naming the entry, even when the row writes files inside it; a nested repository the row itself creates is blamed as a new path;
   - an ignored path is not blamed (the stated limit).
5. Write statusgen/scratchrow-mutations.json for the development mutation harness (`tools/desk/cmd/muhar`). It needs a failing control, plus mutants that disable each matcher, invert the differential, skip the in-tree refusal, skip the per-row diff, skip the nested-repository refusal, and drop the `TMPDIR` export. Each mutant must be caught by a named test above.
6. Correct the named rows to the idiom, keeping each row's subject, assertions and Expect unchanged:
   - measured-status/03 row 5: move the clone to the temp root and replace its trailing rm with the trap.
   - build-less-brittle/08 row 6, build-less-brittle/10 row 7 and build-less-brittle/11 rows 3 and 4: add the trap.
   - derived-board/09 row 7: put the built binary inside the owned directory and add the trap.
   - forge-neutral/34 row 16: add the trap for its directory, keeping its existing under-root refusal.

   Bump each brief's `version:` by one and add a short note to each changed row citing this brief. Do not touch any Evidence row or lifecycle cell.
7. Run `TestWorkspaceCallerCensus`. If it discovers the run-scratch site, add a census row with disposition `temporary-compatibility` under the statusgen owner step, with its retirement condition. Add the changelog fragment changelog/statusgen-owned-row-scratch.md. Replace each `follow-up statusgen/34` entry in `consumers:` with `fixed-here` in the same change. Re-pin Verify row 10 from its provisional pre-merge form to the actual full delivery and first-parent pins with exact summary counts, per `docs/consumer-delivery-checks.md`, before requesting post-merge verification.

## Verify

Named tests, the mutation manifest and the fixture tree are planned deliverables. A missing selector is a failure. Every row that needs scratch uses the idiom it enforces.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation +dereference | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg34-r1.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestScratchRowLintClassifies$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestScratchRowLintClassifies ' "$out")` | exit 0; named PASS. At two fixture sites in different streams, the in-tree row gets `scratch-in-tree` and the unowned tree-copy row gets `scratch-unowned`. The owned idiom row, the no-template mktemp row with a trap, and the `git worktree add`/remove row get neither, so the rules separate the shapes and do not fire on every mktemp. |
| 2 | check:ci +mutation | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg34-r2.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestScratchRowLintDifferential$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestScratchRowLintDifferential ' "$out")` | exit 0; named PASS. A row added or changed against the base is a PROBLEM. A byte-identical row from the base is a NOTICE, and `scratch-unowned` NOTICEs fold into one count line. A brief absent at a resolved base (a new brief) gets PROBLEMs for every row. An unresolvable base gives degraded NOTICEs, never a clean result. |
| 3 | check:ci +flow +mutation | `(cd statusgen && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg34-r3.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestVerifyrunOwnsRowScratch$' . > "$out" 2>&1 && grep -F -- '--- PASS: TestVerifyrunOwnsRowScratch ' "$out")` | exit 0; named PASS. The lint is bypassed: a fixture row that writes into the tree through variable indirection passes `--lint`, and verifyrun records it as `fail` naming the path. A no-template mktemp lands under the run-owned directory, which is gone afterwards. A temp root under the checkout is refused as could-not-run, whether it is given directly, through a symlink outside the checkout, or against a root given relatively or through a symlink. A failed status read records could-not-run, never clean. A path untracked before the run is not blamed, but a new file written inside a pre-existing untracked directory records `fail`. A pre-existing untracked nested repository makes the row record could-not-run for the in-tree check, never clean, even when the row writes inside it. An ignored path is not blamed (the stated limit). |
| 4 | check +mutation | `d=; trap 'test -n "$d" && rm -rf "$d"' EXIT; d=$(mktemp -d "${TMPDIR:-/tmp}/sg34-muhar.XXXXXX") && (cd tools/desk && GOWORK=off go build -o "$d/muhar" ./cmd/muhar) && (cd statusgen && "$d/muhar" -spec scratchrow-mutations.json -j 4 > "$d/out" 2>&1); rc=$?; tail -1 "$d/out"; test "$rc" -eq 0 && grep -qE '^Totals: [1-9][0-9]* caught, 0 NOT CAUGHT, 0 could-not-mutate' "$d/out"` | exit 0. The baseline is green, the failing control is caught, and every mutant across both layers (each matcher, the differential, the in-tree refusal, the per-row diff, the nested-repository refusal, the `TMPDIR` export) is caught, with no survivors and nothing that could not be mutated. |
| 5 | check +dereference | `awk '/^## /{v=($0 ~ /^## Verify/)} v && match($0, /mk[t]emp( -[a-z]+)* +"?(\$PWD\|\$\{PWD\}\|\$\(pwd\))/) {n++} END{print n+0}' docs/streams/*/brief-*.md` | output is `0`. At 83c6988d0 it printed `1` (measured-status/03 row 5), under both BSD awk and gawk, so a zero means the correction landed and is not an empty read. The census covers only the literal working-directory template forms; the lint (row 1) covers the rest. |
| 6 | check +dereference | `d=; trap 'test -n "$d" && rm -rf "$d"' EXIT; d=$(mktemp -d "${TMPDIR:-/tmp}/sg34-ev.XXXXXX") && n=0 && for f in docs/streams/measured-status/brief-03-commsloop-risk-derivation.md docs/streams/build-less-brittle/brief-08-hotspot-metric-and-brittle-mark.md docs/streams/build-less-brittle/brief-10-fitness-functions.md docs/streams/build-less-brittle/brief-11-regression-test-ratchet.md docs/streams/derived-board/brief-09-incremental-reconcile.md docs/streams/forge-neutral/brief-34-deskread-ci-workflow-token-transport.md; do git show 83c6988d097d654c45bd660b4ee36480a175d8bb:"$f" \| awk '/^## /{e=($0 ~ /^## Evidence/)} e' > "$d/b" && awk '/^## /{e=($0 ~ /^## Evidence/)} e' "$f" > "$d/h" && head -c "$(($(wc -c < "$d/b")))" "$d/h" \| cmp -s - "$d/b" && n=$((n+1)); done; echo "$n"` | output is `6`. For each corrected brief, its Evidence section at the pinned authoring base is an exact prefix of its current Evidence section, so the history is unchanged and later verifier rows can still be appended. |
| 7 | check | `d=; trap 'test -n "$d" && rm -rf "$d"' EXIT; d=$(mktemp -d "${TMPDIR:-/tmp}/sg34-lint.XXXXXX") && (cd statusgen && GOWORK=off go build -o "$d/sg" .) && "$d/sg" --root . --lint > "$d/out" 2>&1; rc=$?; awk '/scratch-in-tree/{n++} END{print n+0}' "$d/out"; exit $rc` | exit 0; output is `0`. The tree-built lint passes, and no in-tree finding remains anywhere, whether PROBLEM or NOTICE. |
| 8 | check:ci | `out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg34-skl.XXXXXX") && (cd tools/skillslint && go run . --root ../..) > "$out" 2>&1 && grep -F 'ENFORCEMENT-BLOCK: PASS' "$out" && grep -F 'scratch-in-tree' plugins/assay/skills/author-brief/SKILL.md && grep -F 'scratch-unowned' plugins/assay/skills/author-brief/SKILL.md` | exit 0. The ENFORCEMENT-BLOCK PASS line prints, and the generated block in the author-brief skill names both new tags. |
| 9 | check | `(cd workspace && out=; trap 'test -n "$out" && rm -f "$out"' EXIT; out=$(mktemp "${TMPDIR:-/tmp}/sg34-r9.XXXXXX") && GOWORK=off go test -count=1 -v -run '^TestWorkspaceCallerCensus$' ./... > "$out" 2>&1 && grep -F -- '--- PASS: TestWorkspaceCallerCensus ' "$out")` | exit 0; named PASS. The caller census still matches the tree after the verifyrun change, and a renamed or skipped selector fails the row instead of passing with no tests run. |
| 10 | check +dereference | `statusgen --root . --consumers --brief statusgen/34 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | pre-merge only; exit 0. The branch routing dispositions are checked against the pinned branch fork; the implementation diff corroborates the declared routing. The two out-of-scope entries stay review judgments and are not counted as passes. Inherited and out-of-scope routes remain unchecked obligations. At implementation replace this provisional command with the actual full delivery and first-parent pins, plus exact summary counts, before post-merge verification (see [delivery contract](../../consumer-delivery-checks.md)). |

## Pre-mortem and detection

- The lint fires on every mktemp, so authors learn to ignore it. Caught by row 1's negative fixtures at two sites.
- The differential silently passes when the base cannot be read, or downgrades a newly authored brief to NOTICEs. Caught by row 2's unresolvable-base case and its new-brief case.
- A row writes into the tree through a variable the lint cannot see. Caught by row 3 with the lint bypassed.
- verifyrun's temp directory lands inside the checkout (for example `TMPDIR` set to the repo, or a relative or symlinked root defeating a one-sided comparison), so the owner becomes the defect. Caught by row 3's refusal cases.
- A leftover untracked directory hides later in-tree writes from the per-row read, or a failed read passes as clean. Caught by row 3's pre-existing-directory and failed-read cases.
- A leftover untracked nested Git repository (an in-tree clone) hides every later write into it, because `--untracked-files=all` prints it as one entry and does not look inside. Caught by row 3's nested-repository case, which makes the row could-not-run rather than clean.
- A row writes an artifact to an ignored in-tree path, which passes all three layers and can carry into a later run. Not caught: this is a stated limit of layers 2 and 3, not a claim. Review only.
- A second `trap … EXIT` or a reassigned scratch variable defeats the idiom's cleanup. The lint credits only the last EXIT trap; reassignment is not detected. Under verifyrun, layer 2's run-owned directory bounds the leak.
- A guard is present but can never fire. Caught by row 4's mutants.
- The corrections rewrite history or touch another brief's lifecycle. Caught by row 6 and by review of the README diff.
- The new tags drift from the generated skill block. Caught by row 8.
- No row covers whether the idiom suits every future row shape, such as rows that must run inside the checkout. Review only; rule 50 states that a row needing a Git checkout uses a `git worktree add` and `git worktree remove` pair under the owned directory.

## Evidence

<!-- Independent verifier records actual subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

## Review

Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table. The reviewer must reject any of these:
- a lint that passes when the base cannot be read;
- a verifyrun owner that can resolve under the checkout;
- a correction that changes another brief's Evidence or lifecycle;
- a claim that layer 1 alone closes the class;
- a claim that layer 2 or layer 3 catches ignored in-tree paths.
