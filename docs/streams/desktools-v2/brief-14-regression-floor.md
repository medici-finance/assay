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
authored: 2026-09-29 by the desk, at the driver's direction alongside the #1836 scoping
sources:
  - "the driver's direction of 2026-09-29: a regression test the desk tools pass today and desktools-v2 must be able to pass, with the already-resolved issues integrated"
  - "closed-issue harvest of 2026-09-29 (gh issue list --state closed --search windows|gitlab on this repo): the seed list in deliverable 1; the same query re-run at implementation catches anything resolved since"
  - "#1836 §2 — #1145, #1146 and #1223 confirmed closed; the token-scoping behavior they fixed is part of the floor"
  - "docs/streams/desktools-v2/inventory.md (desktools-v2/01) — the audit that maps each seed issue to its owning package"
exec-tier: strong
exec-tier-why: >-
  (a) each regression test must pin the FIX, not a neighbor — a test that passes both before
  and after the fixing commit is decoration, and telling the two apart is the judgment this
  brief is made of; (b) several seeds are concurrency or transport behaviors (rotate-on-mint
  locking, the gitlab.com dial) where a weak test passes vacuously; (c) the red-at-parent
  confirmation is per-test evidence work a cheap tier cuts corners on exactly where it matters.
domain: complicated
consumers:
  - "tools/desk (NEW internal/regression/ — planned): follow-up desktools-v2/14 (this brief)"
  - "every desktools-v2 brief: the floor this brief lands is what their go test ./... must keep green"
  - "desktools-v2/12 (the platform compatibility suite): sibling — 14 pins today's resolved behavior, 12 extends coverage to Windows/GitLab semantics; the two suites share no cases"
version: 1
id: 4f2e1b7a-9c3d-4e5f-8a6b-1d2c3b4a5967
---

# Brief 14 — regression floor

## Context

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

## Deliverables

1. **Harvest and manifest.** Re-run the closed-issue harvest (`gh issue list --state closed
   --search windows` and `--search gitlab` on this repo, plus the token-scoping closes named
   in #1836), filter to code fixes, and land
   `tools/desk/internal/regression/MANIFEST.md` (planned): one row per seed —
   `| issue | behavior pinned | owning package | test | fixing commit |`. Starter set from the
   2026-09-29 harvest: #322, #643, #656, #687, #697, #708, #719, #727, #757, #772, #773,
   #786, #999, #1007, #1033, #1034, #1056, #1067, #1086, #1145, #1146, #1223.
2. **The tests.** Package `internal/regression` (or the owning package where the behavior
   needs internals): one `TestRegression_Issue<N>_<behavior>` per manifest row, asserting the
   FIXED behavior against fakes/`httptest` — never a live forge, never a network call. Each
   test's doc comment cites the issue and the fixing commit.
3. **Red-at-parent confirmation, recorded.** For every test, run it at the parent of the
   fixing commit and record the failing run (the sha and the failure line) in this brief's
   Evidence table `| issue | test | fixing commit | red-at-parent run |`. A test that passes
   at the parent pins nothing and is reworked or dropped from the manifest with the reason.
4. **The floor rule, where v2 reads it.** A `## Regression floor` section in the desktools-v2
   stream README (prose, outside the generated table): the suite rides the existing
   `go test ./...` in PR CI; a desktools-v2 PR that turns a regression test red may not delete
   or weaken the test — it ports it, and the PR description names the behavior change that
   forced the port.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go test -run 'TestRegression' ./... -v 2>&1 | grep -c '^--- PASS: TestRegression'` | prints a count equal to the manifest's test rows (count it with row 2), every one passing on the merged tree — the tools pass the floor TODAY |
| 2 | `grep -cE '^\| #[0-9]+' tools/desk/internal/regression/MANIFEST.md` | prints the manifest row count; ≥ 20 (the starter set minus any row dropped with its reason recorded in Evidence) |
| 3 | `for n in $(grep -oE '^\| #[0-9]+' tools/desk/internal/regression/MANIFEST.md | tr -d '| #'); do grep -rq "TestRegression_Issue$n" tools/desk --include='*_test.go' || echo "NO TEST: #$n"; done` | prints nothing — every manifest issue has its test |
| 4 | `grep -cE '^\| #[0-9]+.*[0-9a-f]{7,}' docs/streams/desktools-v2/brief-14-regression-floor.md` | prints a count equal to the manifest row count — every Evidence row carries its fixing-commit sha and red-at-parent record |
| 5 | `grep -q '## Regression floor' docs/streams/desktools-v2/README.md` | exit 0 — the floor rule is where a v2 implementer reads it |
| 6 | `cd tools/desk && go test ./... 2>&1 | grep -cE '^(FAIL|---)' ; go test ./... >/dev/null 2>&1; echo rc=$?` | `rc=0` — the whole suite including the floor is green |

## DoD

- All six Verify rows pass on Linux/macOS without a network connection to any forge.
- Every starter-set issue is either a manifest row with a passing test and a red-at-parent
  record, or named in Evidence with the reason it is not (docs-only, decision, or already
  pinned by an existing test — named).
- No production code changes except test helpers a seed genuinely requires, each named in the
  PR description.
