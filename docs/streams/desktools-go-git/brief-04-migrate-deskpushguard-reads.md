---
brief: assay:assay:desktools-go-git:04
title: migrate deskpushguard detection reads to gitcore (parity + mutation test)
wave: 3
depends: ["desktools-go-git/01", "desktools-go-git/02"]
unblocks: ["desktools-go-git/08"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
authored: 2026-08-21 by desktools-go-git authoring session
sources:
  - "docs/streams/desktools-go-git/inventory.md — op families 14-25 as used by deskpushguard (cat-file, show, log, branch -r, ls-tree, merge-base, rev-list, rev-parse)"
  - "docs/streams/desktools-go-git/brief-02-gitcore-transport-auth.md — the gitcore read helpers"
why: >-
  deskpushguard is a security-detection control: it reads git history to decide whether a
  push introduces foreign/unregistered commits. Its reads map onto gitcore cleanly, but a
  behaviour-preserving swap of a DETECTION control has to prove the detection still FIRES,
  not merely that the happy path is unchanged. It is split from brief 03's read migration
  so its parity + mutation coverage gets its own focused review.
version: 1
id: d178703f-dbb2-49ef-afb4-c9e266da8869
---

# Brief 04 — migrate deskpushguard detection reads (parity + mutation test)

## Context

files:
- `tools/desk/cmd/deskpushguard/foreigncommit.go` — `gitOut` seam: `log --format`, `merge-base`
  (`--is-ancestor`), `rev-list --parents`, `cat-file -e`, `branch -r --contains`,
  `ls-tree -r --name-only`.
- `tools/desk/cmd/deskpushguard/registerid.go` — `cat-file -e`, `show <sha>:<path>`,
  `branch -r`, `diff`, `rev-parse`.
- `tools/desk/cmd/deskpushguard/main.go` — `remote get-url`, `rev-parse` reads.
- `docs/streams/desktools-go-git/inventory.md` (planned) — tick migrated op families.

facts:
- Mapping (go-git object API): `cat-file -e <sha>` -> `Repository.CommitObject` /
  `Object` existence; `show <sha>:<path>` -> `commit.Tree().File(path).Contents()`;
  `log --format=%H|%s|%P` -> `Repository.Log` + commit fields; `branch -r [--contains]`
  -> `Repository.References` filtered + ancestry test; `ls-tree -r --name-only` -> tree
  walk; `merge-base`/`--is-ancestor` -> `commit.MergeBase`/`IsAncestor`;
  `rev-list --parents` -> commit `NumParents`/`ParentHashes`.
- Behaviour-preserving swap. deskpushguard's VERDICT (allow / flag foreign commit) must
  be byte-for-byte the same on every golden fixture. Because it is a detection control, a
  MUTATION test is mandatory (brief-rules rule 16): construct a fixture push that DOES
  introduce a foreign/unregistered commit and confirm the migrated reader still flags it
  RED — a green run alone does not prove the detector still detects.
- Out of scope: any change to deskpushguard's detection LOGIC or thresholds — this is a
  seam swap under the same logic. Loosening or altering a detection control is explicitly
  not in this brief.

## Ground rules
- NEVER git push to main / trigger workflows / run mutating infra commands. Feature
  branch + draft PR only.
- Stop at `implemented` — you do not set verified/done.
- NEVER commit `STATUS.md` on a branch (single writer = main's CI).
- Do NOT change detection logic or thresholds — swap the read seam only.
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. Replace deskpushguard's `gitOut` read seam with `gitcore` object/ref/log reads across
   `foreigncommit.go`, `registerid.go`, `main.go`, keeping verdicts identical.
2. Convert the argv-asserting tests to brief-01 outcome goldens covering: a clean push
   (allowed), a foreign-commit push (flagged), and a register-id read.
3. Add the mandatory MUTATION-TEST Verify row below: break the guarded property (inject a
   foreign commit) and confirm the migrated detector goes RED.
4. Tick the migrated op families in `inventory.md`.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect |
|---|---------|--------|
| 1 | `cd tools/desk && go build ./cmd/deskpushguard/ && go vet ./cmd/deskpushguard/` | exit 0 |
| 2 | `cd tools/desk && go test ./cmd/deskpushguard/` | exit 0; clean-push + foreign-commit + register-id goldens pass |
| 3 | `cd tools/desk && go test ./cmd/deskpushguard/ -run ForeignCommitFlagged` | exit 0; the mutation fixture (a foreign commit injected) is flagged RED by the migrated reader — proving the detector still detects |
| 4 | `cd tools/desk && grep -cE 'exec.Command' cmd/deskpushguard/foreigncommit.go` | exit 0; count 0 (the read seam no longer shells the git binary) |
| 5 | `sh tools/desk/scripts/count-git-exec.sh` | prints `git-exec sites: <N>`; N below the count recorded before this brief |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item. -->
### Non-implementer verifier run — 2026-09-15 sonnet-5-verifier (verify-desk dispatch) — **VERIFY: PASS**

Runner ≠ implementer. Own detached temp worktree off origin/main. Offline (KUBECONFIG=/dev/null).

| # | Command | Expect | Observed | Date | Runner |
|---|---------|--------|----------|------|--------|
| 1 | build + vet the migrated command | exit 0 | exit 0 both | 2026-09-15 | sonnet-5-verifier |
| 2 | full package test suite | exit 0, clean-push/foreign-commit/register-id goldens pass | exit 0, ~35 tests PASS incl. clean-branch, foreign-commit, merge-masquerade, stray-base, register-id-collision goldens | 2026-09-15 | sonnet-5-verifier |
| 3 | mutation test, foreign-commit still detected post-migration | exit 0, flagged RED | exit 0, PASS -- genuine fixture-based proof: builds a real 3-repo on-disk git fixture with the real git binary, calls the migrated production function against it, asserts real detection, not a canned string | 2026-09-15 | sonnet-5-verifier |
| 4 | no exec.Command in the migrated read seam | count 0 | count correctly 0 (grep -c's own exit 1 on zero-matches is standard behavior, not a failure) | 2026-09-15 | sonnet-5-verifier |
| 5 | git-exec site count below pre-brief baseline | lower | confirmed: independently reconstructed the pre-brief baseline from the parent commit (100 sites) vs current (82, with 19 seam-call sites removed by this brief's own commit) -- required reduction holds. Noted a pre-existing, stream-wide off-by-one in the counting script's own exclusion filter that affects before/after equally, not attributable to this brief | 2026-09-15 | sonnet-5-verifier |

RISK-VALUE: DERIVED -- enumerated every read-seam swap (log-range, merge-base/ancestor, ref-containment, cat-file, show, ls-tree, rev-list --parents) at file:line, ranked by irreversibility (a silent failure in the ancestry/range or ref-containment logic ranks highest since that's the actual guarded security property), and derived sound: row 3's mutation test reconstructs the historical laundering bug class with a REAL git repository built via the real git binary and calls the now fully in-process production function against it -- a genuine proof, not a golden-string comparison, corroborated by the rest of the passing suite (row 2) covering merge-commit histories, single-parent-merge masquerades, stray-base cuts, and register-id collisions on real fixtures. Detection logic (thresholds, exclusion rules, scoping) confirmed unchanged by code review -- only the read seam moved. One retained shell-out (a network-liveness probe, ls-remote) is explicitly out of scope, unchanged, bounded to a rare code path.

**VERIFY: PASS** -- all 5 rows checked-clean under their substantive intent (row 4's literal exit code is a grep-with-zero-matches artifact, not a defect).

## Review
Gate: model (all four risk answers no — behaviour-preserving read swap under unchanged
detection logic, with a mandatory mutation test proving the detector still fires).
Reviewer confirms the detection verdicts are identical on the goldens and that the
mutation row goes RED. Reviewer records verdict + date in the stream README table.
