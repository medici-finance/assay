---
brief: assay:assay:fresh-views:04
title: "shared append-only log discipline: verify-outcomes.jsonl merge=union + deskevidence post-write sha"
why: >-
  Two shared-log defects, one principle. Concurrent verify-Evidence PRs each append one line to
  verify-outcomes.jsonl and serial-conflict on that single file (#882): approved, clean PRs
  block on a one-line append that never truly conflicts, serializing the whole Evidence lane.
  And deskevidence prints a locally-PREDICTED landed sha that does not exist on main (#806), so
  any downstream keying on it points at a phantom commit. A shared append-only log needs
  union-merge so independent appends don't collide, and a written value downstreams key on must
  be read back from the forge, never predicted.
wave: 0
depends: []
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: [882, 806]
schema: brief-v2
authored: 2026-09-16 by fresh-views scoping session
sources:
  - "docs/streams/fresh-views/spec.md §2 obligation 3 (single-writer / union-merge; read the written value back), §5 Q3 (ordering/dedup)"
  - "medici-finance/assay#882 — verify-outcomes.jsonl serial-conflicts concurrent Evidence PRs; set merge=union (same remedy as the blog index)"
  - "medici-finance/assay#806 — deskevidence prints a landed sha that does not exist on main; report the post-write ref, fail loudly on predicted!=returned"
  - "docs/streams/verify-outcomes.jsonl (the log), .gitattributes (already pins docs/streams/** eol=lf), tools/desk/cmd/deskevidence/github.go + deskevidence.go (the write path)"
  - "freshness-checked 2026-09-16 @ e9fa19d3 (origin/main): #882/#806 confirmed OPEN; .gitattributes has NO merge=union entry for the jsonl; deskevidence present"
exec-tier: strong
exec-tier-why: (b) correctness spans the write path and every downstream that keys on the reported sha; the union-merge choice depends on confirming no consumer needs ordering/dedup
domain: complicated
version: 1
id: 67b7c2f1-7f08-4aa8-a874-233c6a91e3b2
consumers:
  # Authoring PR: routed to the deferred, self-targeting disposition (rule 6); each flips to
  # fixed-here in the implementation commit that edits the path.
  - "docs/streams/verify-outcomes.jsonl: follow-up fresh-views/04 (this brief; confirm no consumer needs line ORDER or uniqueness before union-merge, spec §5 Q3 — flips to fixed-here when implemented)"
  - ".gitattributes: follow-up fresh-views/04 (this brief; adds the merge=union entry — flips to fixed-here when implemented)"
  - "tools/desk/cmd/deskevidence: follow-up fresh-views/04 (this brief; reports the forge-returned commit sha — flips to fixed-here when implemented)"
---

# Brief 04 — shared append-only log discipline

## Context
files:
- `.gitattributes` — add `docs/streams/verify-outcomes.jsonl merge=union` (the existing `docs/streams/** text eol=lf` line stays; a second attribute for the specific file is additive).
- `tools/desk/cmd/deskevidence/github.go` + `tools/desk/cmd/deskevidence/deskevidence.go` — the Evidence-landing write path: after the Contents-API write, report the sha GitHub RETURNED (`commit.sha` in the write response, or a follow-up `GET /repos/{o}/{r}/commits/{branch}`), not a locally predicted one; fail loudly if a predicted and returned sha differ.
- `tools/desk/cmd/deskevidence/deskevidence_test.go` — tests.

facts:
- #882: every verify-Evidence PR appends one line to the shared `docs/streams/verify-outcomes.jsonl`; whichever merges first advances main, and every other open Evidence PR goes CONFLICTING on that one file. The appends are semantically independent (distinct lines), so `merge=union` keeps both with no conflict — the same remedy already used for the shared blog index.
- spec §5 Q3 / #882 caveat: union-merge is safe ONLY if no consumer of the jsonl needs strict line ORDERING or DEDUP, and only if no consumer relies on the file as an ordered/tamper-evident record such that a duplicated or interleaved line could cause it to double-count an outcome. Task step 1 is to confirm this against the file's readers before setting the attribute. Union-merge can also duplicate a line present on both sides — confirm any reader tolerates duplicates or dedups on read.
- #806: `deskevidence` PRINTED `f5a8a22f3a98` while the commit actually on main was `c48d8d427`; the printed sha resolves to no commit reachable from main. Cause: a push race / Contents-API rebase between the sha the verb computed and the commit GitHub created. The verb must report the forge's returned sha, or re-read the ref after the write, and fail loudly when its prediction disagrees.
- single point of failure (rule 10): for #806 the ONE control is "report only the forge-returned sha". The independent second layer is the loud FAIL when predicted != returned — a different signal (a mismatch assertion) than the read itself, so a silently-wrong read still trips the guard rather than propagating a phantom sha.

## Ground rules
- NEVER git push / trigger workflows / run mutating infra commands. Commit only per the task instructions.
- Stop at `implemented` — you do not set verified/done (a different, non-implementing identity does).
- If anything is unclear or contradicts repo state: report NEEDS_CONTEXT, don't guess.

## Task
1. **#882:** confirm no consumer of `verify-outcomes.jsonl` depends on line ordering or uniqueness (grep the tree for readers; record the finding in the PR). Also confirm no consumer treats the file as an ORDERED or TAMPER-EVIDENT record of verification outcomes, and that a duplicated or interleaved line (union-merge's known side effect) cannot cause a consumer to double-count an outcome — this is a recorded decision about an evidence artifact, not a side effect of the conflict-reduction fix. Then add `docs/streams/verify-outcomes.jsonl merge=union` to `.gitattributes`.
2. **#806:** change `deskevidence` to report the commit sha the forge RETURNED for the created commit (Contents-API `commit.sha`, or a post-write `GET .../commits/{branch}`). Assert the returned sha resolves on the branch; if the verb also computed a predicted sha, FAIL LOUDLY (non-zero, named diagnostic) when predicted != returned rather than printing either silently.
3. Tests: (a) a git-level union-merge test — two branches each append a distinct line to the jsonl, a two-parent merge produces no conflict and both lines are present; (b) deskevidence reports the forge-returned sha; (c) deskevidence exits non-zero with a clear message when the predicted and returned shas differ.

## Verify (executable — no prose-only DoD items)
| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `git check-attr merge -- docs/streams/verify-outcomes.jsonl` | exit 0; output ends with `merge: union` | check:ci +dereference |
| 2 | In a scratch repo, branch A and branch B each append one distinct line to the jsonl; `git merge` B into A | exit 0; NO conflict markers; both appended lines present in the merged file | check +flow |
| 3 | `cd tools/desk && go test ./cmd/deskevidence/...` | exit 0 | check:ci |
| 4 | `cd tools/desk && go test ./cmd/deskevidence/... -run TestReportsForgeReturnedSHA -v` | exit 0; the reported sha is the forge's returned value; a predicted!=returned case exits non-zero with a named diagnostic | check:ci +mutation |
| 5 | `statusgen --consumers --root .` | exit 0 — the diff-aware consumers gate corroborates every routing token against the branch diff | check:ci +dereference |

## Evidence
<!-- appended at implementation time by a NON-implementer: one row per Verify item
     (command, exit code, output line(s) or hash, date, runner). -->

## Review
Gate: model (from frontmatter — all four risk answers no). Reviewer records verdict + date in the stream README table.
