---
brief: assay:assay:desktools-v2:01
title: audit & inventory — enumerate every gh shell-out + hardcoded-forge-assumption site (file:line)
why: >-
  Every later brief removes a reach-around: a place a GitHub fact (a subprocess name, a
  remote name, a query shape, a token scope) was written outside the two Forge backends.
  Before any removal, the stream needs one frozen, file:line-accurate inventory of exactly
  which sites those are, across Go, shell, and skills — so no reach-around is missed and no
  migration guesses. This brief produces that inventory and nothing else; it is the
  checklist every migration brief ticks against.
wave: 1
depends: []
unblocks: ["desktools-v2/02", "desktools-v2/03", "desktools-v2/04", "desktools-v2/05", "desktools-v2/06", "desktools-v2/08", "desktools-v2/09", "desktools-v2/10"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-09-16 by desktools-v2 authoring session
sources:
  - "docs/streams/desktools-v2/spec.md §1 (the reach-past table) and §2 (the three design principles) — what this inventory makes file:line-accurate"
  - "tools/desk/internal/deskkit/forge.go — the seam whose bypasses are being counted"
  - "tools/desk/internal/forgeban/allowlist.go — the existing permit-register (const allowedInvocationCeiling now 5) whose rows are a subset of this inventory"
  - "desk audit posted on the stream PR (2026-09-16, origin/main) — the statusgen file:lines and the 5 desk-verb gh exceptions this inventory adopts as its starting point"
  - "statusgen/forgeread.go (header) and docs/streams/forge-neutral/brief-18-statusgen-off-gh-one-read-verb.md — statusgen reaches the seam by RUNNING the `deskread` verb, never by importing deskkit; forge-neutral/18 owns that migration and already carries its own site enumeration"
  - "freshness-checked 2026-09-17 @ 57509073 — `forge_github.go` runs on go-gh with a minted token (not a shelled gh); `forgeban` ceiling is 5 (down from 24); the desk verbs are ~mostly migrated; `grep -rn 'exec.Command(\"gh\"' statusgen --include='*.go'` excluding tests finds 26 sites in 15 files, and statusgen is NOT under forgeban"
exec-tier: any
domain: complicated
version: 1
id: 4041a2e5-62b9-47de-9d42-82e530c571b9
---

# Brief 01 — audit & inventory

## Context

files:
- NEW `docs/streams/desktools-v2/inventory.md` (planned) — the frozen file:line table. This
  is the ONLY deliverable; this brief changes no code.

facts:
- The seam is `tools/desk/internal/deskkit/forge.go` (one `Forge` interface, ~four-dozen
  operations). The two sanctioned backends are `forge_github.go` and `forge_gitlab.go`; a
  GitHub fact appearing OUTSIDE those two files (and their tests) is a reach-around and an
  inventory row.
- Four reach-around SHAPES to enumerate, each a column dimension: (a) a `gh` subprocess
  (`exec.Command("gh"`, a `gh`-shim, or a shell script / skill that shells `gh`);
  (b) a hardcoded remote name (`"origin"`); (c) a hardcoded query shape (a `pullRequest` /
  `mergeRequest` GraphQL block, or a REST path fragment) built outside a backend;
  (d) a token/identity assumption (an inherited `GH_TOKEN`, a `HOME` override, a token
  attached only to a child named `gh`).
- The existing `tools/desk/internal/forgeban/allowlist.go` permit-register already lists the
  surviving forge-CLI *Go* call sites (`const allowedInvocationCeiling` = 5 at
  `e9fa19d3`). Its rows are a SUBSET of this inventory — this brief adds the shell, skills,
  and **statusgen** sites it does not cover, and reconciles against it so no Go site is
  double-counted or dropped.
- **statusgen is the largest remaining `gh` dependency and it is NOT under `forgeban`.** It is
  a separate Go module that does not import `deskkit` — deliberately (`statusgen/forgeread.go`
  header): it reaches the seam by running the `deskread` verb and parsing its JSON. Its
  migration is OWNED by `forge-neutral/18` (in progress), not by this stream. The inventory
  therefore enumerates the statusgen sites from a full sweep
  (`grep -rn 'exec.Command("gh"' statusgen --include='*.go'`, tests excluded — 26 sites in 15
  files at the freshness base; the run's own count is the record, not this number) and routes
  every one of them to `forge-neutral/18`. It marks which are on the `scanloop` path (#628).
  The inventory does NOT propose a client, a library or a port for statusgen.
- The five **desk-verb `gh` exceptions** are a separable, token-custody-gated follow-wave, not
  transport gaps — record them as such, do not route them to the transport migrations:
  `deskadvisory/advisory.go:183` (`gh auth token`, a custody read), `deskdigest/exec.go:47`,
  `deskmerge/exec.go:114` (write-only), `deskdisposition/exec.go:30`,
  `deskpushguard/main.go:409`.
- Search surfaces: `statusgen/**`, `tools/desk/**` (Go + `scripts/`), `tools/cellctl/**`,
  `plugins/assay/` and `.claude/` skill bodies (shell that shells `gh`), and
  `.github/workflows/**`.
- This brief asserts NO count in its own frontmatter or prose — the count is the
  deliverable's, produced by the run, not authored from memory.
- Out of scope: any code change; migrating any site; writing the ban-lint (that is
  `desktools-v2/02`).

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature branch +
  draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` or `docs/streams/FINDINGS.md` on a branch (single writer = main's CI).
- Public repo: use `example-*` placeholders for any org/repo name in prose; no absolute
  machine paths, no private slugs, no session ids.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Write `docs/streams/desktools-v2/inventory.md` (planned): one table keyed by site, with
   columns `# | file:line | tool/skill | reach-around shape (a/b/c/d) | issue | seam op it
   should use (or GAP) | migrating brief`. One row per reach-around site.
2. Enumerate across all four shapes and all search surfaces named in `facts:`. Reconcile the
   Go rows against `tools/desk/internal/forgeban/allowlist.go` explicitly (a `Reconciled:` note under the table
   naming which allowlist rows map to which inventory rows, and any inventory row with no
   allowlist entry).
3. For each row, name the `Forge` op that should replace it, or write `GAP` where no
   enumerated op exists yet (a `GAP` is a signal the interface needs a method — recorded, not
   resolved here).
4. Map each row to the migrating brief (`desktools-v2/03..09`) or `unrouted` if it belongs to
   no brief yet. Route every statusgen `gh` row to `forge-neutral/18` (the owning sibling
   brief; `desktools-v2/08` only holds the zero afterwards); route the five desk-verb `gh`
   exceptions to a `token-custody` follow-wave (not to the transport migrations); route
   access-pattern candidates (N+1 read clusters) to `desktools-v2/09`.
   Add a second table, `## Outward writes`, one row per text-carrying `Forge` write call site
   and per push-path text surface, with the checks it runs today — seeded from
   `docs/streams/desktools-v2/spec.md` §8.2 and re-verified line by line (`desktools-v2/10`
   ticks against it).
5. Record the total site count and the per-shape counts in the PR body (produced by the run,
   not asserted here), broken out desk vs statusgen.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `test -f docs/streams/desktools-v2/inventory.md; echo rc=$?` | `rc=0` (the deliverable exists) |
| 2 | `grep -cE -e 'file:line' -e 'reach-around shape' docs/streams/desktools-v2/inventory.md` | exit 0; count >= 1 (the table declares its shape columns) |
| 3 | `sh -c 'for p in "#1145" "#1146" "#628" "#1019" "#1201" "#884" "#1223"; do grep -qF -- "$p" docs/streams/desktools-v2/inventory.md; rc=$?; if [ "$rc" -ne 0 ]; then echo "MISSING $p"; exit 1; fi; done; echo all-present'` | exit 0; prints `all-present` (each §1 issue is checked SEPARATELY, so one issue repeated on seven lines cannot stand in for the other six; a missing one prints `MISSING <issue>` and exits 1) |
| 4 | `grep -c 'Reconciled:' docs/streams/desktools-v2/inventory.md` | exit 0; count >= 1 (the forgeban reconciliation note is present) |
| 5 | `sh -c 'for f in $(grep -rl "exec.Command(\"gh\"" statusgen --include="*.go"); do case "$f" in *_test.go) continue;; esac; grep -qF -- "$f" docs/streams/desktools-v2/inventory.md; rc=$?; if [ "$rc" -ne 0 ]; then echo "MISSING $f"; exit 1; fi; done; echo all-present'` | exit 0; prints `all-present` (every non-test statusgen file that shells `gh` IN THE TREE AT VERIFY TIME appears in the inventory — the set is derived from the tree, not from a list written here, so no file can be silently omitted) |
| 6 | `bash -c 'for r in $(grep -oE -e "tools/desk/[A-Za-z0-9_./-]+\.go:[0-9]+" -e "statusgen/[A-Za-z0-9_./-]+\.go:[0-9]+" docs/streams/desktools-v2/inventory.md); do f=${r%%:*}; n=${r##*:}; if [ ! -f "$f" ]; then exit 1; fi; if [ "$(wc -l < "$f")" -lt "$n" ]; then exit 1; fi; done; echo ok'` | exit 0; prints `ok` (every cited desk/statusgen file:line resolves to a real line in the tree — the dereferencing check; fails on an invented citation) |
| 7 | `grep -c 'forge-neutral/18' docs/streams/desktools-v2/inventory.md` | exit 0; count >= 1 (the statusgen rows are routed to the sibling brief that owns them, not to a v2 migration) |
| 8 | `grep -c '^## Outward writes' docs/streams/desktools-v2/inventory.md` | exit 0; count >= 1 (the outward-write table `desktools-v2/10` ticks against is present) |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit, key output, date, runner). "verified" requires this filled by
     someone who did NOT implement. -->

### Non-implementer verifier run — VERIFY: PASS — 2026-09-23 opus-5.5-verifier

Run against merged origin/main SHA 50989dbc58f2cd95276dc8fe27314d3a2e15e3f8, from an isolated
worktree cut detached at that head. Confirmed by both a manual run of each row and by
statusgen verifyrun (v1.0.26; all 8 rows pass).

| # | Command | Expected | Observed (exit + key output) | Date | Runner |
|---|---------|----------|------------------------------|-------------|---|
| 1 | test -f docs/streams/desktools-v2/inventory.md; echo rc=$? | rc=0 (deliverable exists) | exit 0; printed rc=0 | 2026-09-23 | opus-5.5-verifier |
| 2 | grep -cE -e 'file:line' -e 'reach-around shape' inventory.md | exit 0; count >= 1 | exit 0; count 11 | 2026-09-23 | opus-5.5-verifier |
| 3 | loop grep -qF each of #1145 #1146 #628 #1019 #1201 #884 #1223 | exit 0; prints all-present | exit 0; printed all-present (no MISSING) | 2026-09-23 | opus-5.5-verifier |
| 4 | grep -c 'Reconciled:' inventory.md | exit 0; count >= 1 | exit 0; count 1 | 2026-09-23 | opus-5.5-verifier |
| 5 | tree-derived: each non-test statusgen gh-shelling file appears in inventory | exit 0; prints all-present | exit 0; printed all-present (15 files, none MISSING) | 2026-09-23 | opus-5.5-verifier |
| 6 | dereference every cited desk/statusgen file:line to a real line | exit 0; prints ok | exit 0; printed ok (no NOFILE/SHORT) | 2026-09-23 | opus-5.5-verifier |
| 7 | grep -c 'forge-neutral/18' inventory.md | exit 0; count >= 1 | exit 0; count 29 | 2026-09-23 | opus-5.5-verifier |
| 8 | grep -c '^## Outward writes' inventory.md | exit 0; count >= 1 | exit 0; count 1 | 2026-09-23 | opus-5.5-verifier |

RISK-VALUE: N/A — enumeration over the diff scope (the inventory commit c128b3e00: a new
markdown inventory doc, a one-line stream-README status flip, and a changelog fragment; no code)
found no literal constant, bound, threshold, tolerance, ratio, timeout, limit, or authority
binding introduced or changed into any code path; the item is a read-only documentation
inventory and is fully reversible by editing markdown, so there is no irreversible act. The
numeric tallies that DO appear in the deliverable (26 sites / 15 statusgen files;
allowedInvocationCeiling = 5) are re-derivable audit observations, not behavior-governing
constants — I re-derived the 26/15 figure from the tree (grep sweep, tests excluded) and it
matches. The brief's own risk block is {regulatory: no, customer: no, irreversible: no,
sensitive-data: no}, gate model, consistent with N/A.

### Execution witness + fresh re-verify — VERIFY: FAIL — 2026-09-27 opus-5.5-verifier

Fresh non-implementer run at merged main 9a557a4c48d1 (statusgen v1.0.27, non-dry witness),
detached worktree, offline. Every row was also run by hand; the hand results match the witness.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `test -f docs/streams/desktools-v2/inventory.md; echo rc=$?` | pass exit=0 | sha256:93ff7811a209 | 2026-09-27 | assay-verifier-app[bot] @ 9a557a4c48d1 (on-behalf-of human:ian) (forge-identity) |
| 2 | `grep -cE -e 'file:line' -e 'reach-around shape' docs/streams/desktools-v2/inventory.md` | pass exit=0 | sha256:25d4f2a86deb | 2026-09-27 | assay-verifier-app[bot] @ 9a557a4c48d1 (on-behalf-of human:ian) (forge-identity) |
| 3 | `sh -c 'for p in "#1145" "#1146" "#628" "#1019" "#1201" "#884" "#1223"; do grep -qF -- "$p" docs/streams/desktools-v2/inventory.md; rc=$?; if [ "$rc" -ne 0 ]; then echo "MISSING $p"; exit 1; fi; done; echo all-present'` | pass exit=0 | sha256:6a62edb7cced | 2026-09-27 | assay-verifier-app[bot] @ 9a557a4c48d1 (on-behalf-of human:ian) (forge-identity) |
| 4 | `grep -c 'Reconciled:' docs/streams/desktools-v2/inventory.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-27 | assay-verifier-app[bot] @ 9a557a4c48d1 (on-behalf-of human:ian) (forge-identity) |
| 5 | `sh -c 'for f in $(grep -rl "exec.Command(\"gh\"" statusgen --include="*.go"); do case "$f" in *_test.go) continue;; esac; grep -qF -- "$f" docs/streams/desktools-v2/inventory.md; rc=$?; if [ "$rc" -ne 0 ]; then echo "MISSING $f"; exit 1; fi; done; echo all-present'` | fail exit=1 | sha256:c8cff280cd6c | 2026-09-27 | assay-verifier-app[bot] @ 9a557a4c48d1 (on-behalf-of human:ian) (forge-identity) |
| 6 | `bash -c 'for r in $(grep -oE -e "tools/desk/[A-Za-z0-9_./-]+\.go:[0-9]+" -e "statusgen/[A-Za-z0-9_./-]+\.go:[0-9]+" docs/streams/desktools-v2/inventory.md); do f=${r%%:*}; n=${r##*:}; if [ ! -f "$f" ]; then exit 1; fi; if [ "$(wc -l < "$f")" -lt "$n" ]; then exit 1; fi; done; echo ok'` | pass exit=0 | sha256:dc51b8c96c2d | 2026-09-27 | assay-verifier-app[bot] @ 9a557a4c48d1 (on-behalf-of human:ian) (forge-identity) |
| 7 | `grep -c 'forge-neutral/18' docs/streams/desktools-v2/inventory.md` | pass exit=0 | sha256:3840bc236ee0 | 2026-09-27 | assay-verifier-app[bot] @ 9a557a4c48d1 (on-behalf-of human:ian) (forge-identity) |
| 8 | `grep -c '^## Outward writes' docs/streams/desktools-v2/inventory.md` | pass exit=0 | sha256:4355a46b19d3 | 2026-09-27 | assay-verifier-app[bot] @ 9a557a4c48d1 (on-behalf-of human:ian) (forge-identity) |

Key observed output per row (hand run, same head):

- Row 1: printed rc=0. Row 2: count 11. Row 3: printed all-present. Row 4: count 1.
- Row 5: printed MISSING statusgen/decisionruling.go, exit 1. That file is new since the
  inventory froze and since the 2026-09-23 PASS: it landed in #1571 (commit 5cb8039a7,
  2026-09-23, not an ancestor of the prior run's head 50989dbc) and carries one gh shell-out at
  statusgen/decisionruling.go line 643, a gh auth token fallback in rulingForgeClient
  (shape (a) + (d), a token-custody read). The tree-derived set is now 16 non-test statusgen
  files / 29 sites, against the inventory's recorded 15 files / 26 sites.
- Row 6: printed ok. Row 7: count 29. Row 8: count 1.

Stale-vs-real (row 5): not stale-shaped. The command is sound and does exactly what its Expect
cell says it does — derive the file set from the tree at verify time so no file can be silently
omitted. The inventory was complete at its own commit (c128b3e00) and at the 2026-09-23 run; it
has since drifted because a new statusgen gh site landed without an inventory row. The
deliverable, not the check, is out of date. The remedy is one inventory row for
statusgen/decisionruling.go line 643 routed to forge-neutral/18 (it is statusgen), noted as a
token-custody read, plus the 26/15 tallies moved to 29/16.

Further drift the Verify table does not catch (observations, not row failures):

- The forgeban ceiling moved: allowedInvocationCeiling = 6 at tools/desk/internal/forgeban/allowlist.go
  line 84 (the inventory's Reconciled: note records 5). The sixth permit is
  internal/deskkit/preflight.go::ambientLoginProbe::gh, added in #1528 (commit 39866201c,
  2026-09-23, after the prior run); it runs gh api user via exec.CommandContext. It has no
  inventory row and is absent from the reconciliation.
- Row 6 checks only that each cited line number exists, not that it still points at the gh
  call. Of the 26 unique statusgen citations, 7 that were gh lines at c128b3e00 no longer are
  (autoflip.go 535/615/656/666, scanissues.go 114/930, trustgate.go 204); 30 of the 38 unique
  tools/desk citations now point at changed line content. The inventory's file:line column
  has line-drifted even where the site set is unchanged.

RISK-VALUE: N/A — enumeration over the implementing diff (c128b3e00: a new markdown
inventory, a one-line stream-README status flip, a changelog fragment; no code) found no
literal constant, bound, threshold, tolerance, ratio, timeout, limit or authority binding
introduced or changed; the item is a reversible documentation inventory with no irreversible
act. The allowedInvocationCeiling value quoted in the brief's facts (5, now 6 at
allowlist.go line 84) is owned by other items' diffs, not this one; it is recorded above as
drift, not as a value this item sets.

VERIFY: FAIL — row 5 (7 of 8 rows pass). The inventory no longer covers the tree: a new
statusgen gh site (decisionruling.go line 643) and a new forgeban permit (preflight.go
ambientLoginProbe, ceiling 5 to 6) landed after the freeze. Status stays implemented.
### Non-implementer verifier run — VERIFY: FAIL — 7/8 rows, row 5 (#1788) — 2026-09-30 claude-opus-5-5-verifier

Runner is not the implementer. Isolated worktree at merged main `b6f8f3c4e86df20681cfb5165f49a9e97659b572` (HEAD == the forge's `commits/main`). `gate: model`, all risk answers `no`. Offline; no check:ci row. Re-confirms the 2026-09-27 FAIL at a newer head: the inventory has not changed since its implementing commit c128b3e00. Status stays `implemented`.

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | `test -f docs/streams/desktools-v2/inventory.md; echo rc=$?` | rc=0 | PASS — `rc=0` | 2026-09-30 | claude-opus-5-5-verifier |
| 2 | row 2 command verbatim (grep count of the file:line and reach-around shape markers) | ≥1 | PASS — 11 | 2026-09-30 | claude-opus-5-5-verifier |
| 3 | row 3 command verbatim (each of the seven issues present) | all-present | PASS — `all-present` | 2026-09-30 | claude-opus-5-5-verifier |
| 4 | `grep -c 'Reconciled:' docs/streams/desktools-v2/inventory.md` | ≥1 | PASS — 1 | 2026-09-30 | claude-opus-5-5-verifier |
| 5 | row 5 command verbatim (tree-derived set of non-test statusgen files that shell gh, each looked up in the inventory) | `all-present` | **FAIL** — exit 1, `MISSING statusgen/decisionruling.go` | 2026-09-30 | claude-opus-5-5-verifier |
| 6 | row 6 command verbatim (dereference every cited file:line) | ok | PASS — `ok` | 2026-09-30 | claude-opus-5-5-verifier |
| 7 | `grep -c 'forge-neutral/18' docs/streams/desktools-v2/inventory.md` | ≥1 | PASS — 29 | 2026-09-30 | claude-opus-5-5-verifier |
| 8 | `grep -c '^## Outward writes' docs/streams/desktools-v2/inventory.md` | 1 | PASS — 1 | 2026-09-30 | claude-opus-5-5-verifier |

RISK-VALUE: N/A — the implementing diff (c128b3e00) is a changelog fragment, a README status line and the inventory markdown; no behaviour-governing literal, nothing irreversible.

Findings: (F1, the FAIL) `statusgen/decisionruling.go` shells `gh auth token` near line 643 (landed after the inventory froze) and has no inventory row — tracked as #1788. (F2) The Reconciled: note records an invocation ceiling of 5; the tree's forgeban allowlist says 6, and the sixth permit (the preflight ambient-login probe) has no row — also #1788. (F3) Row 6 checks that each cited line exists, not that it still points at the gh call; line citations have likely drifted across 36 statusgen commits (not re-counted). Fix in flight: draft PR #1818 (changes requested). Re-verify after it merges.

## Review
Gate: model (all four risk answers no — a read-only inventory document; no code, no
dependency, no capability change). Reviewer records verdict + date in the stream README
table. Row 6 is the dereferencing check: it fails on a wrong-but-well-formed inventory that
cites a file:line the tree does not contain. Rows 3 and 5 test each member separately.
