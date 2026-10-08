---
brief: assay:assay:desktools-v2:08
title: hold statusgen at zero — the gh ban fails on statusgen and the scan is proven with no gh present
why: >-
  statusgen's forge reads are being moved off gh by a sibling brief (forge-neutral/18), through
  the deskread verb. Nothing yet stops the next change from adding a gh shell-out back: statusgen
  has never had a row in the forge-CLI permit register, and the v2 counter only counts. This
  brief turns the statusgen half of that counter into a failing check once the sibling reaches
  zero, and proves the property the migration was for — a scan run with no gh on PATH and no
  ambient credential reads a real queue or says could-not-check, never an empty one (#628). It
  migrates nothing itself.
wave: 7
depends: ["desktools-v2/02", "forge-neutral/18", "forge-neutral/35"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-17 by desktools-v2 authoring session (re-scoped from the withdrawn statusgen-migration draft)
sources:
  - "#2111 — Cobra/Viper adoption; CLI compatibility amendment 2026-10-03 (spec §9)"
  - "docs/streams/desktools-v2/spec.md §2 Principle 2 — statusgen reaches the seam across the deskread verb boundary; v2 contributes enforcement and proof, not migration"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md (forge-neutral/18, in-progress) — OWNS the migration; its Verify row 3 (zero forge-CLI sites in statusgen/) is its completion test. This brief starts where that one ends"
  - "statusgen/forgeread.go — the forgeReader seam: offlineReader is the default, deskreadReader runs the verb once per repo set, every repo lands in exactly one of data / unavailable"
  - "tools/desk/cmd/deskread/main.go — reads authenticate as the session's minted App role via the deskkit resolver; no ambient-credential fallback"
  - "tools/desk/scripts/forge-ban.sh (desktools-v2/02) — the counter whose statusgen half this brief flips to failing"
  - "freshness-checked 2026-09-17 @ 57509073 — 26 exec.Command(\"gh\" sites remain in 15 non-test statusgen files, so forge-neutral/18 is NOT yet at zero and this brief is not yet startable; statusgen/go.mod imports nothing from tools/desk"
consumers:
  - "tools/desk/scripts/forge-ban.sh: follow-up desktools-v2/08 (this brief; the statusgen half becomes failing — flips to fixed-here when the implementation edits the script)"
  - ".github/workflows/forge-surface-control.yml: follow-up desktools-v2/08 (this brief; the advisory step's statusgen half becomes a gate)"
  - "statusgen/forgeread_nogh_test.go and statusgen/reader_boundary_test.go: follow-up desktools-v2/08 (tests only; read migration stays with forge-neutral)"
exec-tier: strong
exec-tier-why: Cross-module capability checks must distinguish safe SDK reuse from indirect credential or write reach.
domain: complicated
version: 4
id: 0a18147e-5225-4ba4-91ab-b3bcd92bc00d
---

# Brief 08 — hold statusgen at zero

## Context

files:
- `tools/desk/scripts/forge-ban.sh` (planned) — the statusgen half exits non-zero above zero.
- `.github/workflows/forge-surface-control.yml` — that half becomes a gate, not an echo.
- NEW `statusgen/forgeread_nogh_test.go` (planned) — the no-`gh` scan proof.
- `changelog/<branch>.md` — the per-PR fragment this repository requires.

single-point-of-failure: after `forge-neutral/18`, the one control keeping `gh` out of
statusgen is review noticing a new shell-out. This brief adds two layers that fail for
different reasons in different components: the counter reddens CI on the SOURCE TEXT of a
re-added `exec.Command("gh"`; the no-`gh` test reddens on the BEHAVIOUR — a scan that still
needs the binary fails when `PATH` does not carry one, whatever the source looks like (a
shell-out reached through a helper or a differently-spelled argv defeats a grep, not the test).

facts:
- PRECONDITION, checked by Verify row 1 before anything else: `forge-neutral/18` and `forge-neutral/35` have landed
  and `grep -rn 'exec.Command("gh"' statusgen --include='*.go'` (tests excluded) finds
  nothing. At the freshness base it finds 26 sites, so this brief is NOT startable yet. If row 1
  fails at pickup, report NEEDS_CONTEXT — do not migrate the remaining sites here; they are the
  sibling brief's deliverable.
- statusgen may import the narrow SDK, not deskkit wholesale. The library-first
  contract replaces the old header prohibition. This enforcement brief adds no read implementation.
- The custody property is already `deskread`'s: it resolves its `Forge` through the `deskkit`
  resolver under the session's minted App role and has no ambient fallback. This brief PROVES
  that end to end from statusgen's side; it does not re-implement it.
- The three-state rule is the point of the proof. With no `gh` and no `deskread` reachable,
  `deskreadReader.OpenIssues` must return every repo as `forgeUnavailable` and an EMPTY data
  map — never a map of empty slices, which would read as "looked and found nothing".
- Out of scope: migrating any statusgen read; flipping the desk-tools half of the counter;
  any credential or minting change.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch.
- Public repo: `example-*` placeholders; no absolute machine paths, private slugs, or session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Library-first amendment — 2026-10-08

Read `docs/library-first.md`. The precondition is completion of BOTH forge-neutral/18 and
/35: /18 deliberately leaves control-feeding reads to /35. Do not demand final zero from
/18 alone or drop those reads from the final gate. The SDK from /36 is permitted; importing
custody, role-executor/write adapters or acquiring ambient credentials is not.

Extend existing architecture/forge scans to the extracted module and all invocation forms,
including CommandContext and native HTTP/credential entrypoints in consumer code. Allow
networking only in declared read adapters, never in pure/offline packages. Add
`statusgen/reader_boundary_test.go` (planned) and the named `TestReaderCapabilityBoundary` below;
plant indirect forbidden imports/ambient lookup and prove the scan fails. Pair build hygiene
with offline behavior and fake-provider scope tests; never claim import lint is OS isolation.

## Task
1. Confirm the precondition (Verify row 1). Stop with NEEDS_CONTEXT if it fails.
2. Make `forge-ban.sh` exit non-zero when its `statusgen sites:` count is above zero, leaving
   the desk-tools half advisory. Make the workflow step a gate for that half.
3. Add `TestScanNeverEmptyWithoutForgeBinary` in `statusgen/forgeread_nogh_test.go` (planned): with `PATH`
   set to an empty temp directory, (a) a stub `deskread` placed on it that prints a valid
   envelope yields the stub's issues — the scan needs no `gh`; (b) with no `deskread` either,
   every requested repo comes back in `unavailable` and the data map has length 0.
4. Show the test failing first: run it against a reader that shells `gh` (the pre-migration
   `ghIssueLister` shape) and quote the red line in the PR body under `## Fail-first`.

## Cobra/Viper integration — preserve the statusgen module boundary

Statusgen receives a dedicated bounded CLI migration owner from desktools-v2/15. This brief still owns holding its forge-CLI count at zero, not its command-tree rewrite. Preserve all existing invocation and machine-output consumers while that owner adopts Cobra/Viper; statusgen uses the shared reader API, with the deskread adapter retained where custody requires it. Source discovery and CLI completion must include statusgen even though it is a separate module. Never treat library imports or a CLI refactor as proof of the existing forge-ban Verify rows.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `grep -rlF --include='*.go' --exclude='*_test.go' 'exec.Command("gh"' statusgen; test $? -eq 1` | exit 0 and NO file names printed (the precondition — `forge-neutral/18` and `/35` are complete: grep found nothing, which is its exit 1). Any file name printed means this brief is not startable |
| 2 | `sh tools/desk/scripts/forge-ban.sh; echo rc=$?` | prints `statusgen sites: 0` and `rc=0` on the clean tree |
| 3 | `sh -c 'f=statusgen/zz_banprobe.go; printf "package main\nimport \"os/exec\"\nvar _ = exec.Command(\"gh\", \"api\")\n" > "$f"; sh tools/desk/scripts/forge-ban.sh >/dev/null 2>&1; rc=$?; rm -f "$f"; echo "rc=$rc"; test "$rc" -ne 0'` | exit 0; prints a non-zero `rc=` — the negative-path row: a re-added `gh` shell-out in statusgen makes the counter FAIL, and the probe file is removed whatever the result |
| 4 | `cd statusgen && go test -timeout 5m -run TestScanNeverEmptyWithoutForgeBinary -v .` | output contains the literal line `--- PASS: TestScanNeverEmptyWithoutForgeBinary` (assert on that line, not the exit status — a `-run` selector matching nothing exits 0) |
| 5 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderCapabilityBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestReaderCapabilityBoundary " "$result")` | exit 0; named PASS; narrow SDK allowed, indirect custody/write/ambient acquisition denied; planted indirect violation fails (+mutation) |
| 6 | `cd statusgen && go test -run '^TestScanNeverEmptyWithoutForgeBinary/gitlab$' -v .` | output must contain the named top-level or subtest `--- PASS:` line (a missing selector is failure); named GitLab-bound scan subtest PASS; brief 08 creates it with ASSAY_REPO_FORGES selecting GitLab (desktools-v2/12 GitLab row) |
| 7 | `cd statusgen && go test -run '^TestScanNeverEmptyWithoutForgeBinary/windows$' -v .` | output must contain the named top-level or subtest `--- PASS:` line (a missing selector is failure); named Windows PATH/PATHEXT subtest PASS; brief 08 creates it using an .exe stub (desktools-v2/12 Windows row) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — a CI counter made failing for one directory and one
test file; no credential, minting or read path changes; read migration belongs to
`forge-neutral/18` and `/35`). Row 3 is the negative-path row for the source-text layer; row 4 is the
behavioural layer and is independent of it. Reviewer records verdict + date in the stream
README table.
