---
brief: assay:assay:desktools-v2:14
title: regression floor — the behavior the desk tools pass today, pinned as tests seeded from resolved issues, which desktools-v2 must keep green
why: >-
  desktools-v2 rewrites the forge access layer underneath every verb. The bugs it must not
  reintroduce are already known — they are the resolved issues: deskwt hardcoding /private/tmp
  (#656), deskclaim-ref silently dialing gitlab.com when go-git cannot read origin (#727),
  deskdispatch rejecting a Windows worktree home (#757), the GitLab backend stubbing
  ListOpenIssues (#1033), rotate-on-mint with no lock revoking the live PAT (#1034), the
  review board missing GitLab <slug> authors (#1056), compare needing both base and head
  (#1067), deskfile stamping labels on the merge request that shares the new issue's number
  (#1086), and the token-scoping fixes of #1145/#1146/#1223. Today those fixes exist only as
  diffs; nothing fails if v2 undoes one. This brief pins each resolved behavior as a test that
  passes on main TODAY and rides the existing go test ./... in PR CI, so every v2 change has
  to keep it green — the floor v1 already stands on, made load-bearing.
wave: 2
depends: ["desktools-v2/01"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [1836, 322, 643, 656, 687, 697, 708, 719, 727, 757, 772, 773, 786, 999, 1007, 1033, 1034, 1056, 1067, 1086, 1145, 1146, 1223]
schema: brief-v2
outcome: none
authored: 2026-09-29 by the desk, at the driver's direction alongside the #1836 scoping
sources:
  - "the driver's direction of 2026-09-29: a regression test the desk tools pass today and desktools-v2 must be able to pass, with the already-resolved issues integrated"
  - "closed-issue harvest of 2026-09-29 (gh issue list --state closed --search windows / --search gitlab on this repo): the starter set in deliverable 1; all 22 confirmed closed on 2026-09-30. The same query, with an explicit --limit, is re-run at implementation to catch anything resolved since"
  - "#1836 §2 — #1145, #1146 and #1223 confirmed closed; the token-scoping behavior they fixed is part of the floor (assay:assay:desktools-v2:13 drops them from brief 06's basis and routes that behavior here)"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the package-by-package map of forge-access sites; used to place each test in its owning package. It does not map seed issues — that mapping is this brief's MANIFEST"
exec-tier: strong
exec-tier-why: >-
  (a) each regression test must pin the FIX, not a neighbor — a test that passes both before
  and after the fixing commit is decoration, and telling the two apart is the judgment this
  brief is made of; (b) several seeds are concurrency or transport behaviors (rotate-on-mint
  locking, the gitlab.com dial) where a weak test passes vacuously; (c) the red-at-parent
  confirmation is per-test evidence work a cheap tier cuts corners on exactly where it matters.
domain: complicated
consumers:
  - "tools/desk (NEW internal/regression/ — planned: MANIFEST.md + TestRegressionManifest + the desk-half TestRegression_ tests): follow-up desktools-v2/14 (this brief)"
  - "statusgen (the statusgen-half TestRegression_ tests for #999 and #1007): follow-up desktools-v2/14 (this brief)"
  - "docs/streams/desktools-v2/README.md (the `## Regression floor` section): follow-up desktools-v2/14 (this brief)"
  - "every other desktools-v2 brief — the floor their PRs must keep green through the existing go test ./... in PR CI: out-of-scope (no edit to those briefs; the rule lives in the stream README where their implementers read it)"
  - "desktools-v2/12 (the platform compatibility suite): out-of-scope (sibling brief — 14 pins today's resolved behavior, 12 extends coverage to Windows/GitLab semantics; the two suites share no cases)"
version: 2
id: 4f2e1b7a-9c3d-4e5f-8a6b-1d2c3b4a5967
---

# Brief 14 — regression floor

## Context

files:
- NEW `tools/desk/internal/regression/` (planned) — `MANIFEST.md`, the manifest test, its `testdata/` fixture, and desk-half regression tests.
- `statusgen/` — NEW statusgen-half regression tests (tests only).
- `docs/streams/desktools-v2/README.md` — the `## Regression floor` section.

v2's premise is that the flows have solidified. What has solidified is recorded in the
resolved-issue register: every closed Windows/GitLab/desk-tools bug is a behavior someone
already fixed once. A rewrite that silently un-fixes one is the most expensive defect class
there is — the report comes back as "this worked before". The desk tools pass every one of
these behaviors today; the suite this brief lands proves it and makes any v2 PR that breaks
one go red.

**Scope discipline.** Seeds are issues whose resolution was a CODE fix. Decision issues,
verify-gate trackers and docs-only closes are not seeds. Where a seed's fix is already covered
by a test that genuinely pins it (would fail at the fixing commit's parent), the manifest
points at that test instead of adding a duplicate.

**Two modules.** The seeds span both Go modules in this repo: most are desk-tools fixes
(`tools/desk/...`), while #999 and #1007 are statusgen behaviors (`statusgen/...`, its own
module). The manifest names the owning package per row, and the Verify rows run each half in
its own module. #786 is a shell-hardening fix in the fleet-provisioning script; its row pins
the behavior through a Go test that runs the script against a fake, or it is dropped with that
reason.

## Deliverables

1. **Harvest and manifest.** Re-run the closed-issue harvest (`gh issue list --state closed
   --search windows --limit 500` and the same with `--search gitlab`, on this repo, plus the
   token-scoping closes named in #1836), filter to code fixes, and land
   `tools/desk/internal/regression/MANIFEST.md` (planned) with two tables:
   - the seed table, one row per pinned issue:
     `| #<N> | behavior pinned | owning package | test | fixing commit |` — the package column
     is a repo-relative path beginning `tools/desk/` or `statusgen/`, and the test column names
     either a new `TestRegression_Issue<N>_<behavior>` or an existing test that genuinely pins
     the fix;
   - a `## Dropped` section, one row per starter issue that is not a seed:
     `| #<N> | <reason> |`, the reason never empty.

   Starter set from the 2026-09-29 harvest: #322, #643, #656, #687, #697, #708, #719, #727,
   #757, #772, #773, #786, #999, #1007, #1033, #1034, #1056, #1067, #1086, #1145, #1146,
   #1223. Known drops: **#719** (docs-only close, not a code fix) and **#322** (a cross-compile
   break — its class is held by desktools-v2/13's T1 cross-compile gate, not by a behavior
   test). That leaves 20 expected seed rows.
2. **The manifest test.** `TestRegressionManifest` (planned) in `tools/desk/internal/regression` parses
   MANIFEST.md and fails unless: every starter issue is either a seed row or a `## Dropped` row
   with a non-empty reason; no issue appears in both; every seed row's fixing commit is a
   7–40 character hex sha; and every named test function exists as `func <Name>(` in a
   `*_test.go` file under the row's package directory (resolved from the repo root, so both
   modules are checked). It carries a positive-control fixture under `testdata/` — a manifest
   with one starter issue missing and one seed naming a non-existent test — and asserts the
   parser rejects it with both faults named, so the manifest test cannot pass vacuously.
3. **The tests.** One `TestRegression_Issue<N>_<behavior>` (planned) per seed row that does not point
   at an existing test, in the row's owning package, asserting the FIXED behavior against
   fakes/`httptest` — never a live forge, never a network call. Each test's doc comment cites
   the issue and the fixing commit.
4. **Red-at-parent confirmation, recorded.** For every seed row, run its test at the parent of
   the fixing commit and record the failing run in this brief's Evidence table, one row per
   seed in the form `| #<N> | <test> | <fixing sha> | <parent sha> — <failure line> |`. A test
   that passes at the parent pins nothing and is reworked, or moved to `## Dropped` with the
   reason.
5. **The floor rule, where v2 reads it.** A `## Regression floor` section in the desktools-v2
   stream README (prose, outside the generated table): the suite rides the existing
   `go test ./...` in PR CI; a desktools-v2 PR that turns a regression test red may not delete
   or weaken the test — it ports it, and the PR description names the behavior change that
   forced the port.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|-------|---------|--------|
| 1 | check +dereference +flow | `cd tools/desk && go test -run '^TestRegressionManifest$' -v ./internal/regression/ > "${TMPDIR:-/tmp}/b14-r1.out" 2>&1; grep -F -e '--- PASS: TestRegressionManifest' "${TMPDIR:-/tmp}/b14-r1.out"` | prints the PASS line (exit 0) — every starter issue is a seed row or a reasoned drop, every named test exists in its package, and the positive-control fixture is rejected; exits 1 on today's tree |
| 2 | check | `cd tools/desk && go test -run '^TestRegression_' -v ./... > "${TMPDIR:-/tmp}/b14-r2.out" 2>&1; echo rc=$?; grep -c -F -e '--- PASS: TestRegression_' "${TMPDIR:-/tmp}/b14-r2.out"` | prints `rc=0` then a count ≥ 1 equal to the manifest's `tools/desk/` seed rows whose test is a new `TestRegression_` (planned) — the desk half of the floor passes TODAY |
| 3 | check | `cd statusgen && go test -run '^TestRegression_' -v . > "${TMPDIR:-/tmp}/b14-r3.out" 2>&1; echo rc=$?; grep -c -F -e '--- PASS: TestRegression_' "${TMPDIR:-/tmp}/b14-r3.out"` | prints `rc=0` then a count equal to the manifest's `statusgen/` seed rows whose test is a new `TestRegression_` (planned) (#999 and #1007 unless either points at an existing test) — the statusgen half passes TODAY |
| 4 | check | `m=$(sed '/^## Dropped/,$d' tools/desk/internal/regression/MANIFEST.md \| grep -c -E -e '^[\|] #[0-9]+ [\|]'); e=$(grep -c -E -e '^[\|] #[0-9]+ [\|] Test[A-Za-z0-9_]+ [\|] [0-9a-f]{7,40} [\|] [0-9a-f]{7,40} ' docs/streams/desktools-v2/brief-14-regression-floor.md); echo "manifest=$m evidence=$e"; test "$m" -eq "$e" && test "$m" -ge 18` | prints equal counts and exits 0 — every seed row has its Evidence row carrying the fixing sha and the red-at-parent sha; the ≥ 18 floor allows at most two drops beyond the known #719 and #322, each reasoned in `## Dropped` |
| 5 | check | `grep -q -E -e '^## Regression floor$' docs/streams/desktools-v2/README.md` | exit 0 — the floor rule is where a v2 implementer reads it |
| 6 | check | `statusgen --consumers --root .` | exit 0; no routing claim in this brief is disproved by the diff |

## DoD

- All six Verify rows pass on Linux/macOS without a network connection to any forge. The
  whole `go test ./...` suite, floor included, is PR CI's job and is not re-run here.
- Every starter-set issue is either a manifest seed row with a passing test and a
  red-at-parent record, or a `## Dropped` row with its reason (docs-only, decision, held by
  another gate — named, or already pinned by an existing test — named).
- No production code changes except test helpers a seed genuinely requires, each named in the
  PR description.
