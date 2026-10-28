---
brief: assay:assay:desktools-v2:76
title: "Move statusgen's git launches to go-git and ratchet git launches down"
why: "statusgen starts a git process for most of its repository reads, and nothing stops the count from growing. The driver's ruling makes go-git the default and allows a git launch only where go-git cannot do the operation, on its own down-only ratchet. desk-tools already has the in-process git layer and its own counter; statusgen has neither."
wave: 1
depends: []
unblocks: []
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
design: DR-forge-library
issues: [2395]
schema: brief-v2
outcome: none
version: 1
id: a27df198-49f5-457c-bb0f-e81289b2a115
authored: 2026-10-25 by desktools-v2 authoring session
sources:
  - "docs/streams/decisions/DR-forge-library.md — point 5 of the driver's ruling (https://github.com/medici-finance/assay/issues/2395#issuecomment-6404972196)"
  - docs/streams/desktools-go-git/README.md — the desk-tools migration, its counter and its gate flip (desktools-go-git/08, todo)
  - tools/desk/scripts/count-git-exec.sh
  - tools/desk/internal/gitcore/
  - "freshness-checked 2026-10-25 @ d6a2ff244 (origin/main)"
exec-tier: strong
exec-tier-why: "Several statusgen git reads feed the verified-flip and corroboration controls. A go-git replacement that walks history or picks merge bases slightly differently returns a plausible answer, and the control then judges the wrong commits; only parity goldens over adversarial histories show the semantics survived."
domain: complicated
gate-why: "sensitive-data is yes because the moved reads include the history and merge-base inputs of the verified-flip provenance control (statusgen/verifyflip.go, SEC-2013-4). The human confirms parity on criss-cross and replace-ref histories, and accepts each git launch kept as one go-git cannot do."
decision-trigger: start
consumers:
  - "tools/go-analysis/pins.env: fixed-here (human-authorized loader alignment to the already admitted BUILD_GO 1.27.1 archives; scanner version, configuration, fingerprint and finding allowances unchanged; reviewed policy-transition admission remains required)"
  - "gitcore/: fixed-here (canonical importable Git owner lifted from tools/desk/internal/gitcore; supported typed reads and native capability boundaries stay with this owner)"
  - "gitcore/gittest/: fixed-here (the canonical test-only fixture owner moves from tools/desk/internal/gittest without duplication)"
  - "tools/desk/: fixed-here (public gitcore/gittest imports and local module replacements; retain desk source guard coverage alongside the lifted owner)"
  - "gitquiet/: out-of-scope (reuse the existing public quiet-fixture implementation; the desk internal adapter remains its consumer)"
  - "statusgen/: fixed-here (link the canonical owner for qualified raw/header, HEAD, discovery/prefix, tree-entry, resolution and parent reads; each native operation keeps its own documented boundary)"
  - "statusgen/verifyflip.go: fixed-here (retain full-history and every-best-base semantics at the existing choke point; do not substitute a different history verdict)"
  - "statusgen/scripts/count-git-launch.sh: fixed-here (source-only offline counter entrypoint used by original Verify row 4)"
  - "statusgen/internal/gitlaunch/: fixed-here (source-only AST inventory, exact per-operation routing and down-only ceiling controls)"
  - "statusgen/git-launches.json: fixed-here (one reasoned operation inventory; distinguish qualified typed defaults, counted native compatibility branches and concrete retained capability/output boundaries)"
  - "docs/streams/desktools-v2/cli-migration.json: fixed-here (only the two exact /76 test routes; preserve the independent statusgen/26 workload route and owner)"
  - "tools/desk/internal/clicontract/gitlaunch_test.go: fixed-here (owner declaration, route removal, shipping and neighboring-entrypoint controls)"
  - "tools/desk/internal/deskkit/citrigger_test.go: fixed-here (actual cross-module read coverage and executing CI route for lifted and retained controls)"
  - "tools/go-analysis/runner.py, scripts/tests/go-analysis.py, scripts/tests/go-analysis-mutations.py: fixed-here (carry the existing desk platform/tag tuples to gitcore without narrowing desk coverage or changing analysis policy)"
  - "Dockerfile, images/assay-harness/Dockerfile, images/verify-runner/Dockerfile: fixed-here (retain sibling module layout in source builds and manifest-only cache preparation)"
  - "statusgen/testdata/verification-handoff/mutate.py: fixed-here (include the lifted owner in the bounded source snapshot without raising its caps)"
  - "docs/research/2026-10-22-git-ai-cache-and-source-control.md: fixed-here (repair only the lifted owner reference; historical citations use immutable pre-lift blobs and completed Verify/Evidence tails stay unchanged)"
  - "docs/streams/desk-tools/README.md: fixed-here (repair only the lifted owner reference; historical citations use immutable pre-lift blobs and completed Verify/Evidence tails stay unchanged)"
  - "docs/streams/desk-tools/brief-26-deskwt-prune-one-walk-and-a-lock.md: fixed-here (repair only the lifted owner reference; historical citations use immutable pre-lift blobs and completed Verify/Evidence tails stay unchanged)"
  - "docs/streams/desktools-go-git/inventory.md: fixed-here (repair only the lifted owner reference; historical citations use immutable pre-lift blobs and completed Verify/Evidence tails stay unchanged)"
  - "docs/streams/desktools-v2/brief-13-platform-gates-and-reconciliation.md: fixed-here (repair only the lifted owner reference; historical citations use immutable pre-lift blobs and completed Verify/Evidence tails stay unchanged)"
  - "docs/streams/desktools-v2/inventory.md: fixed-here (repair only the lifted owner reference; historical citations use immutable pre-lift blobs and completed Verify/Evidence tails stay unchanged)"
  - "docs/streams/forge-neutral/reviewer-write-boundary.md: fixed-here (repair only the lifted owner reference; historical citations use immutable pre-lift blobs and completed Verify/Evidence tails stay unchanged)"
  - "docs/streams/milestone-support/brief-08-normalize-pr-trailers-and-claims-through-the-shared-resolver.md: fixed-here (repair only the lifted owner reference; historical citations use immutable pre-lift blobs and completed Verify/Evidence tails stay unchanged)"
  - "tools/go-analysis/baseline.json: out-of-scope (no identity translation or allowance expansion; exact resolved removals require complete comparable qualification and reviewed policy-transition admission)"
  - "tools/desk/scripts/count-git-exec.sh: out-of-scope (desk-tools' counter and its gate flip belong to desktools-go-git/08; this brief adds statusgen's own ratchet beside it)"
  - "statusgen/evaluation/: out-of-scope (pure evaluation; reads no repository)"
---

# Brief 76 — Move statusgen's git launches to go-git and ratchet git launches down

## Context

files:
- `gitcore/`, including the canonical `gitcore/gittest/` fixture owner, lifted from
  `tools/desk/internal/gitcore/` and `tools/desk/internal/gittest/`
- `tools/desk/go.mod`, `tools/desk/go.sum` and the desk Git/fixture import consumers;
  `tools/desk/internal/gitquiet/` continues to use the existing public `gitquiet/` owner
- `gitcore/ackguard_test.go`, `gitcore/claimref_guard_test.go`,
  `gitcore/fetchtags_guard_test.go`, `tools/desk/internal/gitquiet/`,
  `tools/desk/internal/deskkit/ambienttoken_guard_test.go`,
  `tools/desk/internal/deskkit/forge_surface_test.go` and
  `tools/desk/internal/deskkit/citrigger_test.go` (preserve retained and lifted scopes)
- `statusgen/go.mod`, `statusgen/go.sum`, `statusgen/gitcore_read.go`,
  `statusgen/gitbatch.go`, `statusgen/gitinfo.go`, `statusgen/verifyflip.go`,
  `statusgen/registercorroborate.go` and the other Git read call sites
- `statusgen/scripts/count-git-launch.sh`, `statusgen/internal/gitlaunch/`,
  `statusgen/git-launches.json`, `statusgen/gogit-mutations.json` and their named parity/counter controls
- `docs/streams/desktools-v2/cli-migration.json` (the two exact /76 test routes only),
  `tools/desk/internal/clicontract/gitlaunch_test.go`
- `tools/go-analysis/pins.env` (explicitly authorized Go-loader repair using existing Go 1.27.1 archive pins)
- `tools/go-analysis/runner.py`, `scripts/tests/go-analysis.py`,
  `scripts/tests/go-analysis-mutations.py` (retain prior platform/tag coverage for both owners)
- `Dockerfile`, `images/assay-harness/Dockerfile`, `images/verify-runner/Dockerfile`,
  `statusgen/testdata/verification-handoff/mutate.py`
- CI consumer targets: `.github/workflows/ci.yml`,
  `.github/workflows/assay-statusgen.yml`, `.github/workflows/git-maintenance-windows.yml`
  and the existing `ci/staged-workflows/assay-statusgen.yml` twin; live activation
  remains the existing maintainer-owned act, not an implicit permission of this brief
- Source-bound mutation maps and acceptance callers whose old `internal/gitcore`
  paths or test-module working directories change with the lift
- `changelog/statusgen-go-git.md` (planned)

The library owns object/revision interpretation. Statusgen's adapters preserve the
ordinary and provenance profiles separately: provenance keeps its original repository
locator stripping and replacement/graft isolation. A positively observed unsupported
profile selects native execution before the operation; an in-process read error remains
uncertainty and never triggers a native retry.

Native retention is operation-specific. The generic blob accessor using full commit
decoding has observed raw-header/path-selector gaps; the selected diff reader has
observed missing-subtree and control-name differences; ancestry/merge-base readers have
observed incomplete-graph uncertainty differences. Full-history path selection and the
every-best-base requirement keep their existing counterexample controls. These findings
do not establish that unrelated operations are impossible. The finite revision-caller
pass reuses the qualified existing primitives; remaining short-token, generic-object,
symbolic-ref, native formatting and public failure-output contracts retain their
original operation. Human acceptance of each retained boundary remains required.

The first canonical Go-analysis acquisition stopped at the Go 1.25 loader / Go 1.27
source requirement boundary, recorded in [the /76 analysis blocker](https://github.com/medici-finance/assay/issues/4655#issuecomment-6455905095).
The human subsequently authorized runtime alignment; acquisition with the pinned Go
1.27.1 runtime has succeeded. Policy admission, complete comparable scans and exact
resolved-allowance removals remain separate qualification work. This implementation
note claims neither a canonical scan result nor permission to translate baseline keys.

Desktools-v2/68 and PR #4541's independently owned changes remain excluded. The only
shared-registry amendment is /76's two exact source-only test routes.

single-point-of-failure: none — this brief adds no credential, identity or forge access; it
changes how local repository objects are read. The control it can weaken is the verified-flip
provenance check, whose single input is the history walk and merge-base result; parity goldens
(rows 2 and 3) and the existing `TestMergeBaseChokePoint` are the layers behind it, and any site
without demonstrated parity stays on git.

facts (d6a2ff244, 2026-10-25):
- statusgen non-test code, by `count-git-exec.sh`'s two patterns run over `statusgen/`: 87 direct
  `exec.Command("git", …)` spawns and 11 calls through the named seam helpers, 98 in all
  (re-measured at `e07782de2`: 90 direct and 11 seam, 101 in all; the three new spawns are in
  `statusgen/verifyrun.go`). statusgen
  also has its own helpers (`historyGit`, `coverageGit`, `gitShowObject`, `gitMergeBaseOut`) that
  those patterns do not name; the ratchet here counts spawns, not helper calls.
- desk-tools: `git-exec sites: 204 (direct spawns: 54, seam call sites: 150)` (unchanged at
  `e07782de2`). Its counter is
  advisory until desktools-go-git/08 flips it.
- `tools/desk/go.mod` pins `github.com/go-git/go-git/v5 v5.19.3`; `statusgen/go.mod` has no go-git.
- `gitcore` is internal to the desk module and imports `internal/gitquiet` and `internal/gittest`;
  47 non-test desk Go files import it (`grep -rlE 'internal/gitcore(/[^"]*)?"' tools/desk
  --include='*.go' | grep -v '_test\.go$' | wc -l`, re-measured at `e07782de2`). statusgen
  cannot import it where it is.
- `statusgen/verifyflip.go` at `e07782de2`: `flipJudgeHistory` (line 947) walks
  `rev-list --full-history <c>..HEAD -- <base>` at line 980; `flipMergeBases` (line 1115) runs
  `merge-base --all` with `--no-replace-objects` (line 1119), held by `TestMergeBaseChokePoint`
  (`statusgen/registercorroborate_test.go:826`, SEC-2013-4).
- The criss-cross merge fix now on main added `flipSideBroughtNothing`
  (line 1088), which judges a merge against EVERY best base `flipMergeBases` returns: a go-git
  merge-base that returns a single base, or a different set, changes that verdict. The
  every-best-base parity in Task 4 and row 3 is therefore load-bearing. Line numbers move;
  re-read these functions at pickup.

design-fit:
  owner: gitcore (the existing in-process git layer, made importable)
  contract: "none — local object reads only; no identity, custody or forge contract changes"
  retires: [statusgen git process launches where go-git can do the operation]
  weight: "verbs 0, flags 0; one module lifted, one ratchet"
  why-add: "Reuses desk-tools' existing go-git layer instead of a second one in statusgen."

## Read first

`docs/streams/desktools-go-git/README.md` and its brief 01 (inventory and golden harness),
`gitcore/` and `gitcore/gittest/` (the lifted canonical owners), and the verified-flip code named in facts.

## Human decision

Options:
1. **Accept the migration** — accept the parity evidence for every moved site and the list of
   sites kept on git with the reason go-git cannot do each.
2. **Hold or narrow** — name a moved site whose parity is not shown, and keep it on git.

Default if no answer: none — blocks until answered.

## Ground rules

Feature branch and draft PR only; synthetic repositories only. Never `go test ./...`; run
named tests with a timeout. Do not touch desk-tools' counter or its gate (desktools-go-git/08).
A site whose go-git result differs on any golden stays on git and is listed. No change to what
the verified-flip control decides, only to how it reads.

## Interface contract

statusgen reads repository objects through the lifted `gitcore` API. Each kept git launch is
named in one allowlist with the operation and the reason go-git cannot do it. The ratchet counts
non-test spawn lines in `statusgen/` and fails on any count above the recorded ceiling; the
ceiling only goes down.

## Task

1. Lift `gitcore` into an importable module beside `gitquiet/`, carrying or replacing its
   internal imports; desk-tools keeps building with `GOWORK=off`.
2. Inventory statusgen's spawns: each is moved, kept (go-git cannot), or deferred with an owner.
3. Move the moved sites, file by file, each with a parity golden against the git result.
4. For `flipJudgeHistory` and `flipMergeBases`: build goldens over criss-cross merges, a
   replace ref, a path renamed and re-added, and a merge that drops the path. Move only if every
   golden matches; otherwise keep on git and list them.
5. Add the down-only ratchet and its allowlist; record the ceiling as the post-migration count.

## Verify

Named tests are planned deliverables; each needs its exact PASS line.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -timeout 300s -v -run '^TestGoGitReadParity$' . > "$out" && grep -F -- '--- PASS: TestGoGitReadParity ' "$out")` | exit 0; named PASS; every moved site returns byte-equal results to the git command it replaced over the synthetic repositories; one planted off-by-one in a moved walk fails |
| 2 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -timeout 300s -v -run '^TestFlipHistoryParity$' . > "$out" && grep -F -- '--- PASS: TestFlipHistoryParity ' "$out")` | exit 0; named PASS; `flipJudgeHistory`'s commit set matches `rev-list --full-history` on every golden history, including a merge that drops the path; a walk without full-history semantics fails |
| 3 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -timeout 300s -v -run '^TestMergeBaseChokePoint$' . > "$out" && grep -F -- '--- PASS: TestMergeBaseChokePoint ' "$out")` | exit 0; named PASS; the existing SEC-2013-4 guard still holds, and if `flipMergeBases` moved, its criss-cross and replace-ref goldens return every best base and ignore replace refs; a single-base result fails |
| 4 | check +mutation | `sh statusgen/scripts/count-git-launch.sh` | exit 0 on the clean tree, printing the count at or under the recorded ceiling; a planted `exec.Command("git", …)` in a new non-test statusgen file makes it exit non-zero |
| 5 | check:ci | `(cd tools/desk && GOWORK=off go build ./...)` | exit 0; desk-tools builds against the lifted `gitcore` |
| 6 | check +dereference | `statusgen --root . --consumers --brief desktools-v2/76 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | pre-merge, on the implementation branch: exit 0; the declared routing corroborates the actual diff; out-of-scope judgments remain UNCHECKED and reviewer-owned; a could-not-check is never recorded as a pass |

## Pre-mortem and detection

- A moved read differs in a way only an unusual history shows: rows 1 and 2.
- Merge-base selection changes on a criss-cross history or honours a replace ref: row 3.
- A new git launch lands after the migration: row 4.
- The lift breaks desk-tools: row 5.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit, named result and date. No implementation is claimed by this authoring change. -->

## Review

Gate: human. Confirm each kept launch is one go-git cannot do, and that rows 2 and 3 cover the
histories the verified-flip control actually sees.
