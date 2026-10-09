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
  - "docs/streams/desktools-v2/spec.md §2 Principle 2 — online reads cross the deskread verb boundary and deskkit stays internal; v2 contributes enforcement and proof, not migration"
  - "docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md (forge-neutral/18, in-progress) — OWNS the migration; its Verify row 3 (zero forge-CLI sites in statusgen/) is its completion test. This brief starts where that one ends"
  - "statusgen/forgeread.go — the forgeReader seam: offlineReader is the default, deskreadReader runs the verb once per repo set, every repo lands in exactly one of data / unavailable"
  - "tools/desk/cmd/deskread/main.go — reads authenticate as the session's minted App role via the deskkit resolver; no ambient-credential fallback"
  - "tools/desk/scripts/forge-ban.sh (desktools-v2/02) — the counter whose statusgen half this brief flips to failing"
  - "freshness-checked 2026-09-17 @ 57509073 — 26 exec.Command(\"gh\" sites remain in 15 non-test statusgen files, so forge-neutral/18 is NOT yet at zero and this brief is not yet startable; statusgen/go.mod imports nothing from tools/desk"
consumers:
  - "tools/desk/scripts/forge-ban.sh: follow-up desktools-v2/08 (this brief; the statusgen half becomes failing — flips to fixed-here when the implementation edits the script)"
  - ".github/workflows/forge-surface-control.yml: follow-up desktools-v2/08 (this brief; the advisory step's statusgen half becomes a gate)"
  - "statusgen/forgeread_nogh_test.go and statusgen/reader_boundary_test.go: follow-up desktools-v2/08 (tests only; read migration stays with forge-neutral)"
  - "forgeread/ scan and path-trigger coverage: out-of-scope (forge-neutral/36 owns it and lands it with the extraction)"
exec-tier: strong
exec-tier-why: Cross-module capability checks must tell the permitted offline packages apart from indirect credential or write reach.
domain: complicated
version: 4
id: 0a18147e-5225-4ba4-91ab-b3bcd92bc00d
---

# Brief 08 — hold statusgen at zero

## Context

files:
- `tools/desk/scripts/forge-ban.sh` (planned) — the statusgen half exits non-zero above zero.
- `.github/workflows/forge-surface-control.yml` — that half becomes a gate, not an echo, and
  the path filter gains `statusgen/**` so a statusgen change runs it.
- NEW `statusgen/forgeread_nogh_test.go` (planned) — the no-`gh` scan proof.
- NEW `statusgen/reader_boundary_test.go` (planned) — the capability-boundary test, row 10.
- `changelog/<branch>.md` — the per-PR fragment this repository requires.

single-point-of-failure: after `forge-neutral/18`, the one control keeping `gh` out of
statusgen is review noticing a new shell-out. This brief adds two layers that fail for
different reasons in different components: the counter reddens CI on the SOURCE TEXT of a
re-added `exec.Command("gh"`; the no-`gh` test reddens on the BEHAVIOUR — a scan that still
needs the binary fails when `PATH` does not carry one, whatever the source looks like (a
shell-out reached through a helper or a differently-spelled argv defeats a grep, not the test).

facts:
- PRECONDITION, checked by Verify rows 1 and 8 before anything else: `forge-neutral/18` and
  `forge-neutral/35` have landed (row 8, the board), and no non-test, non-comment statusgen
  line carries the double-quoted literal `"gh"` (row 1, which sees `exec.Command` and
  `exec.CommandContext` launches alike). Row 1 counts 31 lines at this amendment (the earlier
  single-form `exec.Command("gh"` grep found 26 sites at the freshness base), so this brief is
  NOT startable yet. If row 1 or row 8
  fails at pickup, report NEEDS_CONTEXT — do not migrate the remaining sites here; they are the
  sibling brief's deliverable.
- statusgen does not import `deskkit` and must not start to (`statusgen/forgeread.go` header;
  spec §2 Principle 2): `deskkit` stays internal, and `statusgen/go.mod` names nothing under
  `tools/desk` (row 5). If forge-neutral/36 has landed, statusgen may link only that module's
  offline and frozen packages, never an authenticated read adapter (row 10). This brief adds
  no module dependency and no read implementation.
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

## Library-first amendment — 2026-10-08 (ruled: `DR-forge-neutral-36`)

`docs/library-first.md` sets out library-first reuse. The driver ruled on #2395 for offline and
frozen inputs only (`DR-forge-neutral-36`): every online forge read stays on the read verb,
`deskkit` stays internal, and no credential moves into a process that has none today. The
changes below do not depend on that ruling. None of them loosens a row.

- **Precondition is BOTH forge-neutral/18 and /35.** /18 deliberately leaves the
  control-feeding reads to /35, so final zero needs both. Row 1 sees the source text; row 8
  reads the board, because /35's main retirement is a native HTTP client that no `gh` grep can
  observe.
- **Two independent layers on the module boundary.** Row 5, the manifest check, stays. Row 10
  adds `TestReaderCapabilityBoundary`, which walks statusgen's resolved imports. The two fail
  for different reasons in different places: manifest text, and an import graph checked by a
  test.
- **The counter runs on statusgen changes.** Row 9 checks the workflow's path filter, so the
  gate this brief makes is not advisory by omission.
- Scan and path-trigger coverage of the extracted `forgeread/` module is forge-neutral/36's and
  lands with that extraction, not here.

## Task
1. Confirm the precondition (Verify rows 1 and 8). Stop with NEEDS_CONTEXT if either fails.
2. Make `forge-ban.sh` exit non-zero when its `statusgen sites:` count is above zero, leaving
   the desk-tools half advisory. Make the workflow step a gate for that half.
3. Add `TestScanNeverEmptyWithoutForgeBinary` in `statusgen/forgeread_nogh_test.go` (planned): with `PATH`
   set to an empty temp directory, (a) a stub `deskread` placed on it that prints a valid
   envelope yields the stub's issues — the scan needs no `gh`; (b) with no `deskread` either,
   every requested repo comes back in `unavailable` and the data map has length 0.
4. Show the test failing first: run it against a reader that shells `gh` (the pre-migration
   `ghIssueLister` shape) and quote the red line in the PR body under `## Fail-first`.
5. Add `"statusgen/**"` to both path lists of `.github/workflows/forge-surface-control.yml`
   (row 9).
6. Add `TestReaderCapabilityBoundary` in `statusgen/reader_boundary_test.go` (planned) — row 10 —
   and show each planted violation failing first under `## Fail-first`.

## Cobra/Viper integration — preserve the statusgen module boundary

Statusgen receives a dedicated bounded CLI migration owner from desktools-v2/15. This brief still owns holding its forge-CLI count at zero, not its command-tree rewrite. Preserve all existing invocation and machine-output consumers while that owner adopts Cobra/Viper; statusgen continues to call deskread for online reads rather than import deskkit. Source discovery and CLI completion must include statusgen even though it is a separate module. Never treat library imports or a CLI refactor as proof of the existing forge-ban Verify rows.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -d statusgen && { grep -rnF --include='*.go' --exclude='*_test.go' '"gh"' statusgen \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: no non-test, non-comment statusgen line carries the double-quoted literal `"gh"`, so `exec.Command` and `exec.CommandContext` launches are both seen. This is the source-text half of the precondition; it cannot see /35's native HTTP client, which row 8 covers. Any non-zero count means this brief is not startable |
| 2 | `sh tools/desk/scripts/forge-ban.sh; echo rc=$?` | prints `statusgen sites: 0` and `rc=0` on the clean tree |
| 3 | `sh -c 'f=statusgen/zz_banprobe.go; printf "package main\nimport \"os/exec\"\nvar _ = exec.Command(\"gh\", \"api\")\n" > "$f"; sh tools/desk/scripts/forge-ban.sh >/dev/null 2>&1; rc=$?; rm -f "$f"; echo "rc=$rc"; test "$rc" -ne 0'` | exit 0; prints a non-zero `rc=` — the negative-path row: a re-added `gh` shell-out in statusgen makes the counter FAIL, and the probe file is removed whatever the result |
| 4 | `cd statusgen && go test -timeout 5m -run TestScanNeverEmptyWithoutForgeBinary -v .` | output contains the literal line `--- PASS: TestScanNeverEmptyWithoutForgeBinary` (assert on that line, not the exit status — a `-run` selector matching nothing exits 0) |
| 5 | `grep -q 'tools/desk' statusgen/go.mod; test $? -eq 1` | exit 0 (grep found no match — statusgen still imports nothing from desk-tools; the verb boundary held) |
| 6 | `cd statusgen && go test -run '^TestScanNeverEmptyWithoutForgeBinary/gitlab$' -v .` | output must contain the named top-level or subtest `--- PASS:` line (a missing selector is failure); named GitLab-bound scan subtest PASS; brief 08 creates it with ASSAY_REPO_FORGES selecting GitLab (desktools-v2/12 GitLab row) |
| 7 | `cd statusgen && go test -run '^TestScanNeverEmptyWithoutForgeBinary/windows$' -v .` | output must contain the named top-level or subtest `--- PASS:` line (a missing selector is failure); named Windows PATH/PATHEXT subtest PASS; brief 08 creates it using an .exe stub (desktools-v2/12 Windows row) |
| 8 | `grep -cE -e '^[\|] 18 [\|].*[\|] implemented [\|]' -e '^[\|] 18 [\|].*[\|] verified [\|]' -e '^[\|] 18 [\|].*[\|] done [\|]' -e '^[\|] 35 [\|].*[\|] implemented [\|]' -e '^[\|] 35 [\|].*[\|] verified [\|]' -e '^[\|] 35 [\|].*[\|] done [\|]' docs/streams/forge-neutral/README.md` | prints `2`: both forge-neutral/18 and /35 are at least implemented on the board. The board half of the precondition. `0` or `1` means this brief is not startable |
| 9 | `grep -c -F '"statusgen/**"' .github/workflows/forge-surface-control.yml` | prints `2`: the `pull_request` and `push` path lists both carry `statusgen/**`, so a statusgen-only change runs the gate rows 2 and 3 make. Measured at this amendment: `0` |
| 10 | `(cd statusgen && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestReaderCapabilityBoundary$" ./... > "$result" && grep -F -- "--- PASS: TestReaderCapabilityBoundary " "$result")` | exit 0; named PASS. `TestReaderCapabilityBoundary` (planned) resolves statusgen's non-test import graph and fails on any package under the desk-tools module and on any authenticated read-adapter package of the shared module; only that module's offline and frozen packages may appear. **+mutation**: each of three planted violations must fail it — a direct import of a desk-tools package, an indirect import of an adapter package through a statusgen helper, and an adapter import added by a build-tagged file. Row 5 and this row are independent: one reads the manifest, the other the resolved graph |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->

## Review
Gate: model (all four risk answers no — a CI counter made failing for one directory, a path
filter widened to that directory, and two test files: the no-`gh` scan proof and the import
boundary test. No credential, minting or read path changes; read migration belongs to
`forge-neutral/18` and `/35`). Row 3 is the negative-path row for the source-text layer; row 4 is the
behavioural layer and is independent of it. Rows 5 and 10 are the two layers on the module
boundary. Reviewer records verdict + date in the stream
README table.
